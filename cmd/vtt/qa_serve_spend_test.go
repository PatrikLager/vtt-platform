package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

var (
	qaSFBinMu     sync.Mutex
	qaSFBinByTest = map[string]string{}
)

// qaSFBinary builds this package's vtt binary into the test's temp dir,
// once per test.
func qaSFBinary(t *testing.T) string {
	t.Helper()
	qaSFBinMu.Lock()
	defer qaSFBinMu.Unlock()
	if p, ok := qaSFBinByTest[t.Name()]; ok {
		return p
	}
	p := buildVTTBinary(t)
	qaSFBinByTest[t.Name()] = p
	return p
}

func qaSFAbs(t *testing.T, rel string) string {
	t.Helper()
	p, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func qaSFLoad(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load ruleset %s: %v", dir, err)
	}
	return rs
}

func qaSFCopyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o600)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func qaSFEditJSON(t *testing.T, path string, edit func(map[string]any)) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// qaSFVariantDir writes tavern-brawl with a resource "anger" declared after
// stamina, "rage-punch" spending anger and "bottle-smash" spending stamina
// beside chair-swing, and returns its directory.
func qaSFVariantDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	qaSFCopyDir(t, qaSFAbs(t, "../../rulesets/tavern-brawl"), dir)
	qaSFEditJSON(t, filepath.Join(dir, "ruleset.json"), func(doc map[string]any) {
		props, _ := doc["properties"].([]any)
		doc["properties"] = append(props, map[string]any{
			"key": "anger", "type": "resource", "name": "Anger",
			"description": "QA fixture resource spent by rage-punch.", "thresholds": []any{},
		})
	})
	swing, err := os.ReadFile(filepath.Join(dir, "abilities", "chair-swing.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []struct{ id, resource string }{{"rage-punch", "anger"}, {"bottle-smash", "stamina"}} {
		path := filepath.Join(dir, "abilities", a.id+".json")
		if err := os.WriteFile(path, swing, 0o600); err != nil {
			t.Fatal(err)
		}
		qaSFEditJSON(t, path, func(ab map[string]any) {
			ab["id"] = a.id
			ab["name"] = a.id
			ab["usage"] = map[string]any{"limited": map[string]any{"resource": a.resource, "cost": 1}}
		})
	}
	return dir
}

type qaSFSheet struct {
	id        string
	abilities []string
	resources []string
}

// qaSFCampaign writes a campaign holding a SessionStarted and one ActorAdded
// per sheet, in the order given, appended straight to the log so no
// ruleset reads them, and returns its directory, closed.
func qaSFCampaign(t *testing.T, sheets []qaSFSheet) string {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	envs := []*vttv1.Envelope{{
		EventId: "qa-sf-session", SessionId: "qa-sf-session", OccurredAt: timestamppb.Now(),
		Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "qa"}},
	}}
	for i, s := range sheets {
		res := map[string]*vttv1.Resource{}
		for _, k := range s.resources {
			res[k] = &vttv1.Resource{Current: 1, Max: 2}
		}
		envs = append(envs, &vttv1.Envelope{
			EventId: "qa-sf-actor-" + strconv.Itoa(i), SessionId: "qa-sf-session", OccurredAt: timestamppb.Now(),
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
				ActorId: s.id, Name: s.id, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
				Attributes: map[string]int32{"brawn": 2}, Resources: res, AbilityIds: s.abilities,
			}}},
		})
	}
	if _, err := c.AppendBatch(envs); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

// qaSFFirstFault answers a sheet's first fault: keys, ability ids, then the
// spend rule (VTT-1065, VTT-688, VTT-777).
func qaSFFirstFault(rs *rules.Ruleset, s qaSFSheet) *rules.SheetFault {
	if f := rs.SheetKeyFault([]string{"brawn"}, s.resources); f != nil {
		return f
	}
	if f := rs.SheetAbilityFault(s.abilities); f != nil {
		return f
	}
	return rs.SheetSpendFault(s.abilities, s.resources)
}

// qaSFServeRefused runs vtt serve and answers its combined output, failing
// unless it exits non-zero.
func qaSFServeRefused(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, qaSFBinary(t), append([]string{"serve"}, args...)...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("vtt serve %q did not exit (it served): %s", args, out)
	}
	if err == nil {
		t.Fatalf("vtt serve %q exited 0: %s", args, out)
	}
	t.Logf("vtt serve refused: %s", out)
	return string(out)
}

// qaSFServeStarts runs vtt serve until it says it is listening, then stops it.
func qaSFServeStarts(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, qaSFBinary(t), append([]string{"serve"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		if strings.Contains(sc.Text(), "vtt serve: listening on") {
			return
		}
	}
	t.Fatalf("vtt serve %q never listened; stderr: %s", args, stderr.String())
}

// TestQASFServeRefusesACampaignBreakingTheSpendRule pins VTT-777: vtt serve
// with a ruleset refuses a campaign any of whose actors names a limited
// ability without its resource or holds a resource some ability spends
// that no named ability spends, naming each such actor and its first fault
// (in byte order of ids); VTT-1065 and VTT-688 held: a key fault and an
// unheld ability id stay an actor's first fault.
func TestQASFServeRefusesACampaignBreakingTheSpendRule(t *testing.T) {
	rsDir := qaSFAbs(t, "../../rulesets/tavern-brawl")
	rs := qaSFLoad(t, rsDir)
	sheets := []qaSFSheet{
		{id: "f-badkey", abilities: []string{"chair-swing"}, resources: []string{"zest"}},
		{id: "d-orphan", abilities: []string{"fists"}, resources: []string{"stamina"}},
		{id: "a-fits", abilities: []string{"chair-swing", "fists"}, resources: []string{"drink", "stamina"}},
		{id: "e-unheld", abilities: []string{"chair-swing", "no-such-ability"}},
		{id: "c-drink-only", resources: []string{"drink"}},
		{id: "b-no-stamina", abilities: []string{"chair-swing"}, resources: []string{"drink"}},
	}
	camp := qaSFCampaign(t, sheets)

	want := map[string]struct{ field, key string }{
		"b-no-stamina": {"ability_ids", "chair-swing"},
		"d-orphan":     {"resources", "stamina"},
		"e-unheld":     {"ability_ids", "no-such-ability"},
		"f-badkey":     {"resources", "zest"},
	}
	var parts []string
	for _, id := range []string{"a-fits", "b-no-stamina", "c-drink-only", "d-orphan", "e-unheld", "f-badkey"} {
		var s qaSFSheet
		for _, x := range sheets {
			if x.id == id {
				s = x
			}
		}
		f := qaSFFirstFault(rs, s)
		w, misfit := want[id]
		if !misfit {
			if f != nil {
				t.Fatalf("fixture %q faults: %+v", id, *f)
			}
			continue
		}
		if f == nil || f.Field != w.field || f.Key != w.key {
			t.Fatalf("fixture %q first fault %+v, want %s %q", id, f, w.field, w.key)
		}
		parts = append(parts, fmt.Sprintf("actor %q: %s: %s", id, f.Field, f.Reason))
	}
	wantMsg := "vtt serve: ruleset " + rsDir + ": the campaign's actors do not fit it: " + strings.Join(parts, "; ")

	out := qaSFServeRefused(t, "--campaign", camp, "--addr", "127.0.0.1:0", "--ruleset", rsDir)
	if !strings.Contains(out, wantMsg) {
		t.Fatalf("vtt serve output\n got: %s\nwant it to contain: %s", out, wantMsg)
	}
	for _, fit := range []string{`"a-fits"`, `"c-drink-only"`} {
		if strings.Contains(out, fit) {
			t.Errorf("output names fitting actor %s: %s", fit, out)
		}
	}
}

// TestQASFServeOrderAndSharedResource pins the order inside an actor's
// first fault at vtt serve (VTT-777): ability ids before resources, each in
// byte order; and a shared resource fitting while any spender is named.
func TestQASFServeOrderAndSharedResource(t *testing.T) {
	rsDir := qaSFVariantDir(t)
	rs := qaSFLoad(t, rsDir)
	sheets := []qaSFSheet{
		{id: "k-shared", abilities: []string{"bottle-smash"}, resources: []string{"stamina"}},
		{id: "j-both", abilities: []string{"chair-swing"}, resources: []string{"anger"}},
		{id: "i-orphans", abilities: []string{"fists"}, resources: []string{"stamina", "anger"}},
		{id: "h-two-limited", abilities: []string{"rage-punch", "chair-swing"}},
	}
	camp := qaSFCampaign(t, sheets)
	parts := []string{}
	for _, c := range []struct {
		s          qaSFSheet
		field, key string
	}{
		{sheets[3], "ability_ids", "chair-swing"},
		{sheets[2], "resources", "anger"},
		{sheets[1], "ability_ids", "chair-swing"},
	} {
		f := rs.SheetSpendFault(c.s.abilities, c.s.resources)
		if f == nil || f.Field != c.field || f.Key != c.key {
			t.Fatalf("fixture %q fault %+v, want %s %q", c.s.id, f, c.field, c.key)
		}
		parts = append(parts, fmt.Sprintf("actor %q: %s: %s", c.s.id, f.Field, f.Reason))
	}
	wantTail := "the campaign's actors do not fit it: " + strings.Join(parts, "; ")
	out := qaSFServeRefused(t, "--campaign", camp, "--addr", "127.0.0.1:0", "--ruleset", rsDir)
	if !strings.Contains(out, wantTail) {
		t.Fatalf("vtt serve output\n got: %s\nwant it to contain: %s", out, wantTail)
	}
	if strings.Contains(out, `"k-shared"`) {
		t.Errorf("output names the shared-resource actor: %s", out)
	}

	fitting := qaSFCampaign(t, []qaSFSheet{sheets[0], {id: "a-fits", abilities: []string{"rage-punch", "chair-swing"}, resources: []string{"anger", "stamina", "drink"}}})
	qaSFServeStarts(t, "--campaign", fitting, "--addr", "127.0.0.1:0", "--ruleset", rsDir)
}

// TestQASFServeWithNoRulesetRefusesNeitherHalf pins VTT-778's vtt serve
// half, against a campaign the ruleset test above shows refused.
func TestQASFServeWithNoRulesetRefusesNeitherHalf(t *testing.T) {
	camp := qaSFCampaign(t, []qaSFSheet{
		{id: "b-no-stamina", abilities: []string{"chair-swing"}, resources: []string{"drink"}},
		{id: "d-orphan", abilities: []string{"fists"}, resources: []string{"stamina"}},
	})
	qaSFServeRefused(t, "--campaign", camp, "--addr", "127.0.0.1:0", "--ruleset", qaSFAbs(t, "../../rulesets/tavern-brawl"))
	qaSFServeStarts(t, "--campaign", camp, "--addr", "127.0.0.1:0")
}

func qaSFFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// TestQASFAddActorToolSaysWhatTheSpendRuleRefuses pins VTT-786 through the
// tool list `vtt mcp` serves: add_actor's schema says that, with a ruleset
// served, an ability spending a resource the actor does not hold, and a
// resource some ability spends that no named ability spends, refuse the
// command.
func TestQASFAddActorToolSaysWhatTheSpendRuleRefuses(t *testing.T) {
	camp := qaSFCampaign(t, nil)
	ids, err := identity.Open(campaign.LogPath(camp))
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := ids.CreateInvite("qa-agent", identity.RoleAgent)
	if err != nil {
		t.Fatal(err)
	}
	_ = ids.Close()

	addr := qaSFFreePort(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	serve := exec.CommandContext(ctx, qaSFBinary(t), "serve", "--campaign", camp, "--addr", addr)
	serveOut, err := serve.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = serve.Process.Kill()
		_ = serve.Wait()
	}()
	sc := bufio.NewScanner(serveOut)
	listening := false
	for sc.Scan() {
		if strings.Contains(sc.Text(), "listening on") {
			listening = true
			break
		}
	}
	if !listening {
		t.Fatal("vtt serve never listened")
	}
	go func() { _, _ = io.Copy(io.Discard, serveOut) }()

	mcp := exec.CommandContext(ctx, qaSFBinary(t), "mcp", "--server", "ws://"+addr+"/ws", "--token", token)
	var mcpErr bytes.Buffer
	mcp.Stderr = &mcpErr
	in, err := mcp.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := mcp.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := mcp.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = in.Close()
		_ = mcp.Process.Kill()
		_ = mcp.Wait()
	}()
	for _, msg := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"qa","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	} {
		if _, err := io.WriteString(in, msg+"\n"); err != nil {
			t.Fatalf("write to vtt mcp: %v (stderr %s)", err, mcpErr.String())
		}
	}
	rd := bufio.NewReader(out)
	var list struct {
		ID     int `json:"id"`
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties struct {
						Actor struct {
							Properties map[string]struct {
								Description string `json:"description"`
							} `json:"properties"`
						} `json:"actor"`
					} `json:"properties"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	for list.ID != 2 {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			t.Fatalf("read from vtt mcp: %v (stderr %s)", err, mcpErr.String())
		}
		list.ID = 0
		if err := json.Unmarshal(line, &list); err != nil {
			t.Fatalf("decode %s: %v", line, err)
		}
	}
	var abilityDoc, resourceDoc string
	found := false
	for _, tool := range list.Result.Tools {
		if tool.Name == "add_actor" {
			found = true
			abilityDoc = tool.InputSchema.Properties.Actor.Properties["abilityIds"].Description
			resourceDoc = tool.InputSchema.Properties.Actor.Properties["resources"].Description
		}
	}
	if !found {
		t.Fatal("vtt mcp lists no add_actor")
	}
	for _, w := range []string{"With a ruleset served, an ability that spends a resource the actor does not hold refuses the whole command"} {
		if !strings.Contains(abilityDoc, w) {
			t.Errorf("abilityIds description does not say %q:\n%s", w, abilityDoc)
		}
	}
	for _, w := range []string{"with a ruleset served, so does a resource some ability of the ruleset spends that no ability the actor names spends"} {
		if !strings.Contains(resourceDoc, w) {
			t.Errorf("resources description does not say %q:\n%s", w, resourceDoc)
		}
	}
	if !strings.Contains(resourceDoc, "refuses the whole command") {
		t.Errorf("resources description does not say the command is refused:\n%s", resourceDoc)
	}
}
