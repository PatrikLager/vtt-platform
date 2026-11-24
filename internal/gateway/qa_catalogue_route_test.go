package gateway_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qacrBaseRuleset = "../../rulesets/tavern-brawl"

type qacrForm struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Properties  []string `json:"properties"`
}

type qacrEntry struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Form        string         `json:"form"`
	Values      map[string]int `json:"values"`
}

// Keep the forms out of key and name order, and each form's keys out of the
// ruleset's property order and byte order: an ordered fixture cannot tell
// declared order from a sort (VTT-1004).
var qacrForms = []qacrForm{
	{Key: "zulu", Name: "Ledger", Description: "The form listed first.", Properties: []string{"footing", "brawn", "grit"}},
	{Key: "alpha", Name: "Satchel", Description: "The form listed second.", Properties: []string{"grit", "footing"}},
	{Key: "mike", Name: "Crate", Description: "The form listed last.", Properties: []string{"brawn"}},
}

type qacrEntryFile struct {
	file string
	body string
}

// Keep the file names out of id and name order, and a child before what it
// extends: otherwise read order passes for VTT-1005's sort.
var qacrEntryFiles = []qacrEntryFile{
	{"a.json", `{"id":"india","name":"Lantern","description":"A base holding the top of the range.","form":"mike","values":{"brawn":2147483647}}`},
	{"b.json", `{"id":"delta","name":"Mallet","description":"Extends bravo and sets footing alone.","form":"zulu","values":{"footing":9},"extends":"bravo"}`},
	{"c.json", `{"id":"bravo","name":"Quill","description":"Extends echo, setting brawn to zero and grit below it.","form":"zulu","values":{"brawn":0,"grit":-3},"extends":"echo"}`},
	{"d.json", `{"id":"hotel","name":"Basket","description":"Extends india and sets nothing.","form":"mike","values":{},"extends":"india"}`},
	{"e.json", `{"id":"echo","name":"Kettle","description":"A base setting every key of its form.","form":"zulu","values":{"footing":2,"brawn":5,"grit":7}}`},
	{"f.json", `{"id":"charlie","name":"Anvil","description":"A base of the second form.","form":"alpha","values":{"grit":1,"footing":11}}`},
	{"g.json", `{"id":"foxtrot","name":"Zither","description":"A base setting one key, at the bottom of the range.","form":"zulu","values":{"brawn":-2147483648}}`},
}

// VTT-1005, VTT-1002.
var qacrCatalogue = []qacrEntry{
	{ID: "bravo", Name: "Quill", Description: "Extends echo, setting brawn to zero and grit below it.", Form: "zulu", Values: map[string]int{"footing": 2, "brawn": 0, "grit": -3}},
	{ID: "charlie", Name: "Anvil", Description: "A base of the second form.", Form: "alpha", Values: map[string]int{"grit": 1, "footing": 11}},
	{ID: "delta", Name: "Mallet", Description: "Extends bravo and sets footing alone.", Form: "zulu", Values: map[string]int{"footing": 9, "brawn": 0, "grit": -3}},
	{ID: "echo", Name: "Kettle", Description: "A base setting every key of its form.", Form: "zulu", Values: map[string]int{"footing": 2, "brawn": 5, "grit": 7}},
	{ID: "foxtrot", Name: "Zither", Description: "A base setting one key, at the bottom of the range.", Form: "zulu", Values: map[string]int{"brawn": -2147483648}},
	{ID: "hotel", Name: "Basket", Description: "Extends india and sets nothing.", Form: "mike", Values: map[string]int{"brawn": 2147483647}},
	{ID: "india", Name: "Lantern", Description: "A base holding the top of the range.", Form: "mike", Values: map[string]int{"brawn": 2147483647}},
}

func qacrCopyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func qacrRulesetDir(t *testing.T, forms json.RawMessage, files []qacrEntryFile) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ruleset")
	qacrCopyDir(t, qacrBaseRuleset, dir)
	if forms != nil {
		manifest := filepath.Join(dir, "ruleset.json")
		raw, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		fields["forms"] = forms
		out, err := json.MarshalIndent(fields, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifest, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(files) > 0 {
		cat := filepath.Join(dir, "catalogue")
		if err := os.MkdirAll(cat, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(cat, f.file), []byte(f.body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir
}

func qacrLoad(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load(%s): %v", dir, err)
	}
	return rs
}

func qacrFormsJSON(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(qacrForms)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type qacrTable struct {
	url    string
	tokens map[string]string
	ids    *identity.DB
}

func qacrEventID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// Keep the player a spectator granted an actor: CreateInvite refuses player
// (VTT-911).
func qacrServe(t *testing.T, rs *rules.Ruleset) *qacrTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	tokens := map[string]string{}
	mint := func(role, name string, r identity.Role) string {
		tok, id, err := ids.CreateInvite(name, r)
		if err != nil {
			t.Fatalf("CreateInvite %s: %v", name, err)
		}
		tokens[role] = tok
		return id
	}
	dmID := mint("dm", "Host", identity.RoleDM)
	mint("agent", "Helper", identity.RoleAgent)
	playerID := mint("player", "Mover", identity.RoleSpectator)
	mint("spectator", "Watcher", identity.RoleSpectator)

	stamp := func(env *vttv1.Envelope) *vttv1.Envelope {
		env.EventId = qacrEventID(t)
		env.OccurredAt = timestamppb.Now()
		env.ParticipantId = dmID
		env.ActorRole = string(identity.RoleDM)
		return env
	}
	if _, err := c.AppendBatch([]*vttv1.Envelope{
		stamp(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: &vttv1.Actor{ActorId: "qacr-piece", Name: "Piece"},
		}}}),
		stamp(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: "qacr-piece", ParticipantId: playerID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}}),
	}); err != nil {
		t.Fatalf("grant the player an actor: %v", err)
	}

	srv := gateway.New(c, ids)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		hs.Close()
		_ = c.Close()
		_ = ids.Close()
	})
	tb := &qacrTable{url: hs.URL, tokens: tokens, ids: ids}
	tb.requireRoles(t)
	return tb
}

var qacrRoles = []string{"dm", "agent", "player", "spectator"}

func qacrGet(t *testing.T, url, auth string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

// Keep this check: a refused token, or a player read as a spectator, leaves
// every per-role assertion testing fewer roles than it names.
func (tb *qacrTable) requireRoles(t *testing.T) {
	t.Helper()
	for _, role := range qacrRoles {
		status, body := qacrGet(t, tb.url+"/api/me", "Bearer "+tb.tokens[role])
		if status != http.StatusOK {
			t.Fatalf("/api/me for %s: status %d, body %s", role, status, body)
		}
		var me struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(body, &me); err != nil {
			t.Fatalf("/api/me for %s: %v (%s)", role, err, body)
		}
		if me.Role != role {
			t.Fatalf("/api/me for the %s fixture says role %q", role, me.Role)
		}
	}
}

func (tb *qacrTable) readRuleset(t *testing.T, role string) map[string]json.RawMessage {
	t.Helper()
	status, body := qacrGet(t, tb.url+"/api/ruleset", "Bearer "+tb.tokens[role])
	if status != http.StatusOK {
		t.Fatalf("/api/ruleset for %s: status %d, body %s", role, status, body)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("/api/ruleset for %s: %v (%s)", role, err, body)
	}
	return fields
}

func qacrDecodeForms(t *testing.T, role string, fields map[string]json.RawMessage) []qacrForm {
	t.Helper()
	raw, ok := fields["forms"]
	if !ok {
		t.Fatalf("%s: /api/ruleset has no forms field", role)
	}
	var got []qacrForm
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s: forms %s: %v", role, raw, err)
	}
	return got
}

func qacrDecodeCatalogue(t *testing.T, role string, fields map[string]json.RawMessage) []qacrEntry {
	t.Helper()
	raw, ok := fields["catalogue"]
	if !ok {
		t.Fatalf("%s: /api/ruleset has no catalogue field", role)
	}
	var got []qacrEntry
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s: catalogue %s: %v", role, raw, err)
	}
	return got
}

func qacrRequireEmptyList(t *testing.T, role, field string, fields map[string]json.RawMessage) {
	t.Helper()
	raw, ok := fields[field]
	if !ok {
		t.Errorf("%s: /api/ruleset has no %s field; want []", role, field)
		return
	}
	if got := string(bytes.TrimSpace(raw)); got != "[]" {
		t.Errorf("%s: %s = %s; want []", role, field, got)
	}
}

func qacrRequireForms(t *testing.T, role string, got []qacrForm) {
	t.Helper()
	if len(got) != len(qacrForms) {
		t.Fatalf("%s: %d forms %+v; want %d %+v", role, len(got), got, len(qacrForms), qacrForms)
	}
	for i, want := range qacrForms {
		g := got[i]
		if g.Key != want.Key || g.Name != want.Name || g.Description != want.Description {
			t.Errorf("%s: form %d = %+v; want %+v", role, i, g, want)
		}
		if !slices.Equal(g.Properties, want.Properties) {
			t.Errorf("%s: form %s properties %v; want %v in declared order", role, want.Key, g.Properties, want.Properties)
		}
	}
}

func qacrRequireCatalogue(t *testing.T, role string, got []qacrEntry) {
	t.Helper()
	if len(got) != len(qacrCatalogue) {
		t.Fatalf("%s: %d entries %+v; want %d", role, len(got), got, len(qacrCatalogue))
	}
	for i, want := range qacrCatalogue {
		g := got[i]
		if g.ID != want.ID {
			t.Errorf("%s: entry %d has id %q; want %q (sorted by id)", role, i, g.ID, want.ID)
			continue
		}
		if g.Name != want.Name || g.Description != want.Description || g.Form != want.Form {
			t.Errorf("%s: entry %s = name %q description %q form %q; want %q %q %q",
				role, want.ID, g.Name, g.Description, g.Form, want.Name, want.Description, want.Form)
		}
		if !maps.Equal(g.Values, want.Values) {
			t.Errorf("%s: entry %s values %v; want %v", role, want.ID, g.Values, want.Values)
		}
	}
}

func TestQACRFixtureLoadsAsWritten(t *testing.T) {
	rs := qacrLoad(t, qacrRulesetDir(t, qacrFormsJSON(t), qacrEntryFiles))
	if len(rs.Forms) != len(qacrForms) {
		t.Fatalf("loaded %d forms; want %d", len(rs.Forms), len(qacrForms))
	}
	if len(rs.Catalogue) != len(qacrCatalogue) {
		t.Fatalf("loaded %d entries; want %d", len(rs.Catalogue), len(qacrCatalogue))
	}
	keys := []string{}
	names := []string{}
	for _, f := range qacrForms {
		keys = append(keys, f.Key)
		names = append(names, f.Name)
	}
	if slices.IsSorted(keys) || slices.IsSorted(names) {
		t.Fatal("forms are declared in key or name order")
	}
	files := []string{}
	for _, f := range qacrEntryFiles {
		var e qacrEntry
		if err := json.Unmarshal([]byte(f.body), &e); err != nil {
			t.Fatal(err)
		}
		files = append(files, e.ID)
	}
	if slices.IsSorted(files) {
		t.Fatal("catalogue files are written in id order")
	}
	entryNames := []string{}
	for _, e := range qacrCatalogue {
		entryNames = append(entryNames, e.Name)
	}
	if slices.IsSorted(entryNames) {
		t.Fatal("entry names sort the same as ids")
	}
}

func TestQACREveryRoleReadsTheFormsInDeclaredOrder(t *testing.T) {
	tb := qacrServe(t, qacrLoad(t, qacrRulesetDir(t, qacrFormsJSON(t), qacrEntryFiles)))
	for _, role := range qacrRoles {
		// VTT-1004: each form with its key, name, description and property keys, in
		// declared order, to every role.
		qacrRequireForms(t, role, qacrDecodeForms(t, role, tb.readRuleset(t, role)))
	}
}

func TestQACREveryRoleReadsTheCatalogueSortedByIDResolvedThroughExtends(t *testing.T) {
	tb := qacrServe(t, qacrLoad(t, qacrRulesetDir(t, qacrFormsJSON(t), qacrEntryFiles)))
	for _, role := range qacrRoles {
		// VTT-1005, VTT-1002: each entry with id, name, description, form and values after
		// what it extends, sorted by id.
		qacrRequireCatalogue(t, role, qacrDecodeCatalogue(t, role, tb.readRuleset(t, role)))
	}
}

func TestQACRTheCatalogueOrderHoldsOnEveryRead(t *testing.T) {
	tb := qacrServe(t, qacrLoad(t, qacrRulesetDir(t, qacrFormsJSON(t), qacrEntryFiles)))
	want := []string{}
	for _, e := range qacrCatalogue {
		want = append(want, e.ID)
	}
	for _, role := range qacrRoles {
		for range 25 {
			got := []string{}
			for _, e := range qacrDecodeCatalogue(t, role, tb.readRuleset(t, role)) {
				got = append(got, e.ID)
			}
			// VTT-1005: sorted by id on every request, not as a map happens to iterate.
			if !slices.Equal(got, want) {
				t.Fatalf("%s: catalogue ids %v; want %v", role, got, want)
			}
		}
	}
}

func TestQACRAnExtendedValueOfZeroIsServedAsZero(t *testing.T) {
	tb := qacrServe(t, qacrLoad(t, qacrRulesetDir(t, qacrFormsJSON(t), qacrEntryFiles)))
	for _, role := range qacrRoles {
		for _, e := range qacrDecodeCatalogue(t, role, tb.readRuleset(t, role)) {
			if e.ID != "bravo" && e.ID != "delta" {
				continue
			}
			// VTT-1002, VTT-1005: an entry's own zero over a value it extends is served, not
			// dropped or inherited.
			if v, ok := e.Values["brawn"]; !ok || v != 0 {
				t.Errorf("%s: entry %s brawn = %d (present %v); want 0 present", role, e.ID, v, ok)
			}
		}
	}
}

func TestQACRNoFormsAndNoCatalogueAreEmptyListsForEveryRole(t *testing.T) {
	cases := []struct {
		name  string
		serve bool
		forms json.RawMessage
	}{
		{"no ruleset served", false, nil},
		{"a ruleset declaring neither", true, nil},
		{"forms written null", true, json.RawMessage(`null`)},
		{"forms written empty", true, json.RawMessage(`[]`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rs *rules.Ruleset
			if tc.serve {
				rs = qacrLoad(t, qacrRulesetDir(t, tc.forms, nil))
			}
			tb := qacrServe(t, rs)
			for _, role := range qacrRoles {
				fields := tb.readRuleset(t, role)
				// VTT-1004: [] and never null when none is declared or no ruleset is served.
				qacrRequireEmptyList(t, role, "forms", fields)
				// VTT-1005: [] and never null when the ruleset holds none or no ruleset is
				// served.
				qacrRequireEmptyList(t, role, "catalogue", fields)
			}
		})
	}
}

func TestQACRFormsWithoutACatalogueServeTheFormsAndAnEmptyCatalogue(t *testing.T) {
	tb := qacrServe(t, qacrLoad(t, qacrRulesetDir(t, qacrFormsJSON(t), nil)))
	for _, role := range qacrRoles {
		fields := tb.readRuleset(t, role)
		// VTT-1004: the declared forms are served though the catalogue is empty.
		qacrRequireForms(t, role, qacrDecodeForms(t, role, fields))
		// VTT-1005: [] when the ruleset holds no entry.
		qacrRequireEmptyList(t, role, "catalogue", fields)
	}
}

func TestQACRARequestWithoutAnAcceptedTokenGetsNoCatalogue(t *testing.T) {
	tb := qacrServe(t, qacrLoad(t, qacrRulesetDir(t, qacrFormsJSON(t), qacrEntryFiles)))
	revokedTok, revokedID, err := tb.ids.CreateInvite("Leaver", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := qacrGet(t, tb.url+"/api/ruleset", "Bearer "+revokedTok); status != http.StatusOK {
		t.Fatalf("the token to revoke is refused before it is revoked: %d", status)
	}
	if err := tb.ids.Revoke(revokedID); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"no header":        "",
		"no bearer prefix": tb.tokens["dm"],
		"another scheme":   "Token " + tb.tokens["dm"],
		"an unknown token": "Bearer qacr-not-a-token",
		"a revoked token":  "Bearer " + revokedTok,
		"an empty bearer":  "Bearer ",
	}
	for name, auth := range cases {
		status, body := qacrGet(t, tb.url+"/api/ruleset", auth)
		// VTT-581: a request without an accepted token gets 401 and no catalogue.
		if status != http.StatusUnauthorized {
			t.Errorf("%s: status %d; want 401", name, status)
		}
		if got := strings.TrimSpace(string(body)); got != "gateway: unauthorized" {
			t.Errorf("%s: body %q; want %q", name, got, "gateway: unauthorized")
		}
		for _, e := range qacrCatalogue {
			if bytes.Contains(body, []byte(e.Name)) || bytes.Contains(body, []byte(e.ID)) {
				t.Errorf("%s: body names entry %s", name, e.ID)
			}
		}
		for _, f := range qacrForms {
			if bytes.Contains(body, []byte(f.Name)) {
				t.Errorf("%s: body names form %s", name, f.Key)
			}
		}
	}
}

func TestQACRTheWireNamesEachFieldExactly(t *testing.T) {
	tb := qacrServe(t, qacrLoad(t, qacrRulesetDir(t, qacrFormsJSON(t), qacrEntryFiles)))
	for _, role := range qacrRoles {
		fields := tb.readRuleset(t, role)
		var forms, entries []map[string]json.RawMessage
		if err := json.Unmarshal(fields["forms"], &forms); err != nil {
			t.Fatalf("%s: forms: %v", role, err)
		}
		if err := json.Unmarshal(fields["catalogue"], &entries); err != nil {
			t.Fatalf("%s: catalogue: %v", role, err)
		}
		if len(forms) == 0 || len(entries) == 0 {
			t.Fatalf("%s: %d forms, %d entries", role, len(forms), len(entries))
		}
		// VTT-1004: each form's key, name, description and property keys, under those
		// names exactly.
		for _, f := range forms {
			for _, k := range []string{"key", "name", "description", "properties"} {
				if _, ok := f[k]; !ok {
					t.Errorf("%s: a form has no %q field: %v", role, k, slices.Collect(maps.Keys(f)))
				}
			}
		}
		// VTT-1005: each entry's id, name, description, form and values, under those names
		// exactly.
		for _, e := range entries {
			for _, k := range []string{"id", "name", "description", "form", "values"} {
				if _, ok := e[k]; !ok {
					t.Errorf("%s: an entry has no %q field: %v", role, k, slices.Collect(maps.Keys(e)))
				}
			}
		}
	}
}
