package conformance_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"unicode"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/adventure/conformance"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Keep every content word out of this file (rule 5): each key is read from
// the shipped files, reached from the scenario below.
const qasrScenario = "adventure-night"

// Derive this sha256 of a qasrD21Rendering by hand from D21 of
// docs/superpowers/plans/2026-11-25-a-sheet-refers-to-the-catalogue.md,
// never from the files it is held against.
const qasrD21Digest = "c710270c5d6c35adf243201bb5710a3ca85cf1b33c513cfa48cf39a5a853d8f2"

type qasrWorld struct {
	root, rulesetsRoot, rulesetDir, advDir, goldensDir string
}

func qasrRepo() string { return filepath.Join("..", "..", "..") }

func qasrShipped(t *testing.T) qasrWorld {
	t.Helper()
	root := qasrRepo()
	var sc struct {
		Steps []struct {
			Command struct {
				LoadAdventure *struct {
					AdventureID string `json:"adventureId"`
				} `json:"loadAdventure"`
			} `json:"command"`
		} `json:"steps"`
	}
	qasrReadJSON(t, filepath.Join(root, "scenarios", qasrScenario+".json"), &sc)
	advID := ""
	for _, s := range sc.Steps {
		if s.Command.LoadAdventure != nil {
			advID = s.Command.LoadAdventure.AdventureID
			break
		}
	}
	if advID == "" {
		t.Fatalf("scenario %s loads no adventure", qasrScenario)
	}
	w := qasrWorld{
		root:         root,
		rulesetsRoot: filepath.Join(root, "rulesets"),
		advDir:       filepath.Join(root, "adventures", advID),
		goldensDir:   filepath.Join(root, "scenarios", "goldens", qasrScenario),
	}
	w.rulesetDir = filepath.Join(w.rulesetsRoot, qasrDeclaredRuleset(t, w.advDir))
	return w
}

func qasrDeclaredRuleset(t *testing.T, advDir string) string {
	t.Helper()
	var a struct {
		Ruleset string `json:"ruleset"`
	}
	qasrReadJSON(t, filepath.Join(advDir, "adventure.json"), &a)
	if a.Ruleset == "" {
		t.Fatalf("%s declares no ruleset", advDir)
	}
	return a.Ruleset
}

func qasrCopy(t *testing.T) qasrWorld {
	t.Helper()
	src := qasrShipped(t)
	dst := t.TempDir()
	w := qasrWorld{
		root:         dst,
		rulesetsRoot: filepath.Join(dst, "rulesets"),
		rulesetDir:   filepath.Join(dst, "rulesets", filepath.Base(src.rulesetDir)),
		advDir:       filepath.Join(dst, "adventures", filepath.Base(src.advDir)),
		goldensDir:   filepath.Join(dst, "goldens"),
	}
	for from, to := range map[string]string{src.rulesetDir: w.rulesetDir, src.advDir: w.advDir, src.goldensDir: w.goldensDir} {
		if err := os.CopyFS(to, os.DirFS(from)); err != nil {
			t.Fatalf("copy %s: %v", from, err)
		}
	}
	return w
}

func qasrReadJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func qasrEditJSON(t *testing.T, path string, edit func(v any) any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out, err := json.MarshalIndent(edit(v), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(out, qasrReindent(t, b)) {
		t.Fatalf("%s: the edit changed nothing", path)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

func qasrReindent(t *testing.T, b []byte) []byte {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func qasrReplace(t *testing.T, path, old, repl string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if n := strings.Count(s, old); n != 1 {
		t.Fatalf("%s holds %q %d times, want once", path, old, n)
	}
	if repl != "" && strings.Contains(s, repl) {
		t.Fatalf("%s already holds %q", path, repl)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(s, old, repl, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func qasrObj(v any, keys ...string) map[string]any {
	for _, k := range keys {
		v = v.(map[string]any)[k]
	}
	return v.(map[string]any)
}

func qasrLoadRuleset(t *testing.T, w qasrWorld) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(w.rulesetDir)
	if err != nil {
		t.Fatalf("rules.Load(%s): %v", w.rulesetDir, err)
	}
	return rs
}

func qasrLoad(t *testing.T, w qasrWorld) (*rules.Ruleset, *adventure.Adventure) {
	t.Helper()
	rs := qasrLoadRuleset(t, w)
	adv, err := adventure.Load(w.advDir, rs)
	if err != nil {
		t.Fatalf("adventure.Load(%s): %v", w.advDir, err)
	}
	return rs, adv
}

func qasrManifest(t *testing.T, w qasrWorld) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(w.rulesetDir, "ruleset.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func qasrSorted(s []string) []string {
	c := slices.Clone(s)
	sort.Strings(c)
	return c
}

func qasrPlaceFaults(rs *rules.Ruleset, manifest []byte) []string {
	var faults []string
	var file struct {
		Places []struct {
			Key, Name, Description string
			Forms, States          []string
			Copies                 bool
		} `json:"places"`
	}
	if err := json.Unmarshal(manifest, &file); err != nil {
		return []string{err.Error()}
	}
	if len(rs.Places) != 3 {
		faults = append(faults, fmt.Sprintf("the ruleset declares %d places, D21 three", len(rs.Places)))
	}
	if len(file.Places) != len(rs.Places) {
		faults = append(faults, fmt.Sprintf("loaded %d places, the file writes %d", len(rs.Places), len(file.Places)))
	}
	for i := range min(len(file.Places), len(rs.Places)) {
		f, p := file.Places[i], rs.Places[i]
		if f.Key != p.Key || f.Name != p.Name || f.Description != p.Description || f.Copies != p.Copies ||
			!slices.Equal(f.Forms, p.Forms) || !slices.Equal(f.States, p.States) {
			faults = append(faults, fmt.Sprintf("place %d loads as %+v, the file writes %+v", i, p, f))
		}
	}
	formKeys := make([]string, 0, len(rs.Forms))
	for _, f := range rs.Forms {
		formKeys = append(formKeys, f.Key)
	}
	if len(formKeys) != 2 {
		faults = append(faults, fmt.Sprintf("the ruleset declares forms %v, D21's places hold two", formKeys))
	}
	shape := map[string][]string{}
	for _, p := range rs.Places {
		if len(p.States) != 0 {
			faults = append(faults, fmt.Sprintf("place %q names states %v, D21 none", p.Key, p.States))
		}
		k := fmt.Sprintf("%d forms, copies %t", len(p.Forms), p.Copies)
		shape[k] = append(shape[k], strings.Join(qasrSorted(p.Forms), ","))
	}
	one, once, both := shape["1 forms, copies true"], shape["1 forms, copies false"], shape["2 forms, copies true"]
	switch {
	case len(one) != 1 || len(once) != 1 || len(both) != 1 || len(shape) != 3:
		faults = append(faults, fmt.Sprintf("the places' shapes are %v, D21 one of each of a form with copies, "+
			"a form once and both forms with copies", shape))
	case one[0] == once[0]:
		faults = append(faults, fmt.Sprintf("both single-form places hold %q, D21 one form each", one[0]))
	case both[0] != strings.Join(qasrSorted(formKeys), ","):
		faults = append(faults, fmt.Sprintf("the two-form place holds %q, D21 both forms %v", both[0], formKeys))
	}
	return faults
}

func qasrSectionFaults(rs *rules.Ruleset) []string {
	var faults []string
	type at struct{ page, section int }
	var found []at
	for i, pg := range rs.Sheet {
		for j, s := range pg.Sections {
			if len(s.Places) > 0 {
				found = append(found, at{i, j})
			}
		}
	}
	if len(found) != 1 {
		return []string{fmt.Sprintf("%d sections show places, D21 one", len(found))}
	}
	pg := rs.Sheet[found[0].page]
	s := pg.Sections[found[0].section]
	if found[0].section != len(pg.Sections)-1 {
		faults = append(faults, fmt.Sprintf("section %q stands at %d of page %q's %d sections, D21 after them",
			s.Name, found[0].section, pg.Name, len(pg.Sections)))
	}
	if found[0].section == 0 {
		faults = append(faults, fmt.Sprintf("section %q is the first of page %q, D21 after its existing ones", s.Name, pg.Name))
	}
	keys := make([]string, 0, len(rs.Places))
	for _, p := range rs.Places {
		keys = append(keys, p.Key)
	}
	if !slices.Equal(qasrSorted(s.Places), qasrSorted(keys)) {
		faults = append(faults, fmt.Sprintf("section %q shows %v, the ruleset declares %v", s.Name, s.Places, keys))
	}
	return faults
}

func qasrWordFaults(rs *rules.Ruleset) []string {
	var faults []string
	for _, p := range rs.Places {
		for field, text := range map[string]string{"name": p.Name, "description": p.Description} {
			if strings.TrimSpace(text) == "" {
				faults = append(faults, fmt.Sprintf("place %q has no %s", p.Key, field))
			}
			if i := strings.IndexFunc(text, unicode.IsDigit); i >= 0 {
				faults = append(faults, fmt.Sprintf("place %q's %s holds a digit at byte %d: %q", p.Key, field, i, text))
			}
		}
	}
	if len(rs.Places) == 0 {
		faults = append(faults, "no place to read")
	}
	return faults
}

type qasrRawRef map[string]string

func qasrActorFiles(t *testing.T, advDir string) map[string]map[string]qasrRawRef {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(advDir, "actors", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no actor files under %s: %v", advDir, err)
	}
	out := map[string]map[string]qasrRawRef{}
	for _, p := range paths {
		var a struct {
			ActorID    string                `json:"actor_id"`
			References map[string]qasrRawRef `json:"references"`
		}
		qasrReadJSON(t, p, &a)
		out[a.ActorID] = a.References
	}
	return out
}

func qasrToWire(refs map[string]adventure.Reference) map[string]*vttv1.Reference {
	out := make(map[string]*vttv1.Reference, len(refs))
	for id, r := range refs {
		out[id] = &vttv1.Reference{EntryId: r.EntryID, Place: r.Place, State: r.State}
	}
	return out
}

func qasrSameRefs(got map[string]*vttv1.Reference, file map[string]qasrRawRef) bool {
	if len(got) != len(file) {
		return false
	}
	for id, f := range file {
		g, ok := got[id]
		if !ok || len(f) > 3 {
			return false
		}
		want := &vttv1.Reference{EntryId: f["entry_id"], Place: f["place"], State: f["state"]}
		if !proto.Equal(g, want) {
			return false
		}
	}
	return true
}

// VTT-1042 VTT-1043 VTT-1072
func qasrReferenceFaults(rs *rules.Ruleset, adv *adventure.Adventure, files map[string]map[string]qasrRawRef) []string {
	var faults []string
	if len(adv.Actors) != len(files) {
		faults = append(faults, fmt.Sprintf("loaded %d actors, %d files", len(adv.Actors), len(files)))
	}
	for _, a := range adv.Actors {
		wire := qasrToWire(a.References)
		if !qasrSameRefs(wire, files[a.ID]) {
			faults = append(faults, fmt.Sprintf("actor %q loads references %v, its file writes %v", a.ID, a.References, files[a.ID]))
		}
		if len(a.References) == 0 {
			faults = append(faults, fmt.Sprintf("actor %q holds no reference, D21 gives each some", a.ID))
		}
		seen := map[string]bool{}
		for _, id := range slices.Sorted(maps.Keys(a.References)) {
			r := a.References[id]
			if id != r.EntryID {
				faults = append(faults, fmt.Sprintf("actor %q: reference %q names entry %q, D21 its entry's id", a.ID, id, r.EntryID))
			}
			if r.State != "" {
				faults = append(faults, fmt.Sprintf("actor %q: reference %q holds state %q, D21 none", a.ID, id, r.State))
			}
			e, pl := rs.Catalogue[r.EntryID], rs.Place(r.Place)
			switch {
			case e == nil:
				faults = append(faults, fmt.Sprintf("actor %q: reference %q names entry %q, not in the catalogue", a.ID, id, r.EntryID))
			case pl == nil:
				faults = append(faults, fmt.Sprintf("actor %q: reference %q names place %q, not declared", a.ID, id, r.Place))
			case !slices.Contains(pl.Forms, e.Form):
				faults = append(faults, fmt.Sprintf("actor %q: reference %q puts form %q in place %q, which holds %v",
					a.ID, id, e.Form, r.Place, pl.Forms))
			case !pl.Copies && seen[r.Place+"\x00"+r.EntryID]:
				faults = append(faults, fmt.Sprintf("actor %q: reference %q holds entry %q twice in place %q", a.ID, id, r.EntryID, r.Place))
			}
			seen[r.Place+"\x00"+r.EntryID] = true
			if f := rs.ReferenceFault(id, wire[id]); f != nil {
				faults = append(faults, fmt.Sprintf("actor %q: ReferenceFault: %+v", a.ID, *f))
			}
		}
		if f := rs.SheetReferenceFault(wire); f != nil {
			faults = append(faults, fmt.Sprintf("actor %q: SheetReferenceFault: %+v", a.ID, *f))
		}
	}
	return faults
}

// Keep this rendering's form, or re-derive qasrD21Digest by hand.
func qasrD21Rendering(rs *rules.Ruleset, adv *adventure.Adventure) string {
	var lines []string
	for _, p := range rs.Places {
		lines = append(lines, fmt.Sprintf("place %s forms=%s copies=%t states=%s",
			p.Key, strings.Join(qasrSorted(p.Forms), ","), p.Copies, strings.Join(p.States, ",")))
	}
	for _, pg := range rs.Sheet {
		for _, s := range pg.Sections {
			if len(s.Places) > 0 {
				lines = append(lines, fmt.Sprintf("section %s/%s places=%s", pg.Name, s.Name, strings.Join(qasrSorted(s.Places), ",")))
			}
		}
	}
	for _, a := range adv.Actors {
		for id, r := range a.References {
			lines = append(lines, fmt.Sprintf("ref %s %s %s=%s state=%s", a.ID, r.Place, id, r.EntryID, r.State))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func qasrD21Faults(rs *rules.Ruleset, adv *adventure.Adventure) []string {
	r := qasrD21Rendering(rs, adv)
	sum := sha256.Sum256([]byte(r))
	if got := hex.EncodeToString(sum[:]); got != qasrD21Digest {
		return []string{fmt.Sprintf("the content renders as\n%s\nwhose digest %s is not D21's %s", r, got, qasrD21Digest)}
	}
	return nil
}

func qasrGoldenFaults(t *testing.T, w qasrWorld, adv *adventure.Adventure) []string {
	t.Helper()
	var faults []string
	files := qasrActorFiles(t, w.advDir)
	var golden []struct {
		Type       string `json:"type"`
		ActorAdded *struct {
			ActorID    string                `json:"actor_id"`
			References map[string]qasrRawRef `json:"references"`
		} `json:"actor_added"`
	}
	qasrReadJSON(t, filepath.Join(w.advDir, "goldens", "compiled-batch.json"), &golden)
	inGolden := map[string]bool{}
	for _, e := range golden {
		if e.ActorAdded == nil {
			continue
		}
		inGolden[e.ActorAdded.ActorID] = true
		if !reflect.DeepEqual(e.ActorAdded.References, files[e.ActorAdded.ActorID]) {
			faults = append(faults, fmt.Sprintf("the golden gives actor %q references %v, its file %v",
				e.ActorAdded.ActorID, e.ActorAdded.References, files[e.ActorAdded.ActorID]))
		}
	}
	envs, _, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		return append(faults, fmt.Sprintf("Compile: %v", err))
	}
	compiled := 0
	for _, env := range envs {
		a := env.GetActorAdded().GetActor()
		if a == nil {
			continue
		}
		compiled++
		if !inGolden[a.GetActorId()] {
			faults = append(faults, fmt.Sprintf("the golden holds no actor_added for %q", a.GetActorId()))
		}
		if !qasrSameRefs(a.GetReferences(), files[a.GetActorId()]) {
			faults = append(faults, fmt.Sprintf("Compile gives actor %q references %v, its file %v",
				a.GetActorId(), a.GetReferences(), files[a.GetActorId()]))
		}
	}
	if compiled != len(files) {
		faults = append(faults, fmt.Sprintf("Compile adds %d actors, %d files", compiled, len(files)))
	}
	return faults
}

func qasrStream(t *testing.T, dir string) []*vttv1.Envelope {
	t.Helper()
	var raw []json.RawMessage
	qasrReadJSON(t, filepath.Join(dir, "stream.json"), &raw)
	envs := make([]*vttv1.Envelope, 0, len(raw))
	for i, r := range raw {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(r, env); err != nil {
			t.Fatalf("stream.json[%d]: %v", i, err)
		}
		envs = append(envs, env)
	}
	return envs
}

func qasrPayload(env *vttv1.Envelope) *vttv1.Envelope {
	c := proto.Clone(env).(*vttv1.Envelope)
	c.EventId, c.Sequence, c.OccurredAt, c.SessionId, c.ActorRole, c.ParticipantId = "", 0, nil, "", "", ""
	return c
}

func qasrNightFaults(t *testing.T, w qasrWorld, adv *adventure.Adventure) []string {
	t.Helper()
	var faults []string
	stream := qasrStream(t, w.goldensDir)
	st := engine.NewState()
	for _, env := range stream {
		if err := engine.Apply(st, env); err != nil && !errors.Is(err, engine.ErrUnknownVariant) {
			faults = append(faults, fmt.Sprintf("Apply(sequence %d): %v", env.GetSequence(), err))
		}
	}
	gotJSON, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(gotJSON, &got); err != nil {
		t.Fatal(err)
	}
	qasrReadJSON(t, filepath.Join(w.goldensDir, "state.json"), &want)
	if head, last := want["headSequence"], float64(stream[len(stream)-1].GetSequence()); head != last {
		faults = append(faults, fmt.Sprintf("state.json's headSequence is %v, the stream ends at %v", head, last))
	}
	delete(want, "headSequence")
	for _, k := range slices.Sorted(maps.Keys(want)) {
		if !reflect.DeepEqual(got[k], want[k]) {
			faults = append(faults, fmt.Sprintf("the fold's %s differs from state.json's", k))
		}
	}
	if len(got) != len(want) {
		faults = append(faults, fmt.Sprintf("the fold writes %d fields, state.json %d", len(got), len(want)))
	}
	envs, _, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		return append(faults, fmt.Sprintf("Compile: %v", err))
	}
	if len(stream) < len(envs) {
		return append(faults, fmt.Sprintf("the stream holds %d events, Compile gives %d", len(stream), len(envs)))
	}
	for i, env := range envs {
		if !proto.Equal(qasrPayload(stream[i]), env) {
			faults = append(faults, fmt.Sprintf("stream event %d is %v, Compile gives %v", i, qasrPayload(stream[i]), env))
		}
	}
	files := qasrActorFiles(t, w.advDir)
	for id, refs := range files {
		a := st.Actors[id]
		if a == nil || !qasrSameRefs(a.GetReferences(), refs) {
			faults = append(faults, fmt.Sprintf("the fold gives actor %q references %v, its file %v", id, a.GetReferences(), refs))
		}
	}
	return faults
}

func qasrStatblocks(guide string) ([]*vttv1.Actor, error) {
	var out []*vttv1.Actor
	parts := strings.Split(guide, "```json\n")
	for _, p := range parts[1:] {
		end := strings.Index(p, "```")
		if end < 0 {
			return nil, errors.New("an unclosed json block")
		}
		a := &vttv1.Actor{}
		if protojson.Unmarshal([]byte(p[:end]), a) != nil || a.GetActorId() == "" {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// VTT-1059
func qasrStatblockFaults(guide string, adv *adventure.Adventure) []string {
	var faults []string
	blocks, err := qasrStatblocks(guide)
	if err != nil {
		return []string{err.Error()}
	}
	byID := map[string]*vttv1.Actor{}
	for _, b := range blocks {
		if byID[b.GetActorId()] != nil {
			faults = append(faults, fmt.Sprintf("two statblocks describe %q", b.GetActorId()))
		}
		byID[b.GetActorId()] = b
	}
	actors := map[string]bool{}
	for _, a := range adv.Actors {
		actors[a.ID] = true
		b := byID[a.ID]
		if b == nil {
			faults = append(faults, fmt.Sprintf("no statblock describes actor %q", a.ID))
			continue
		}
		want := qasrToWire(a.References)
		same := len(b.GetReferences()) == len(want)
		for id, r := range want {
			same = same && proto.Equal(b.GetReferences()[id], r)
		}
		if !same {
			faults = append(faults, fmt.Sprintf("the statblock of %q names %v, the actor holds %v", a.ID, b.GetReferences(), want))
		}
	}
	for id := range byID {
		if !actors[id] {
			faults = append(faults, fmt.Sprintf("a statblock describes %q, which the adventure does not hold", id))
		}
	}
	return faults
}

func qasrGuideTextFaults(guide string, rs *rules.Ruleset) []string {
	var faults []string
	for _, p := range rs.Places {
		if !strings.Contains(guide, "`"+p.Key+"`") {
			faults = append(faults, fmt.Sprintf("the guide never names place `%s`", p.Key))
		}
	}
	for _, s := range []string{"**Adding.**", "**Changing.**", "**Removing.**",
		"`add_reference`", "`change_reference`", "`remove_reference`", "No player is sent a message"} {
		if !strings.Contains(guide, s) {
			faults = append(faults, fmt.Sprintf("the guide does not say %q", s))
		}
	}
	refused := false
	for _, para := range strings.Split(guide, "\n\n") {
		flat := strings.Join(strings.Fields(para), " ")
		if strings.Contains(flat, "a form lists") && strings.Contains(flat, "refuses") {
			refused = true
		}
	}
	if !refused {
		faults = append(faults, "no paragraph of the guide says that a key a form lists is refused")
	}
	return faults
}

func qasrGuide(t *testing.T, w qasrWorld) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(w.rulesetDir, "guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func qasrNone(t *testing.T, faults []string) {
	t.Helper()
	for _, f := range faults {
		t.Error(f)
	}
}

func qasrSome(t *testing.T, faults []string, want string) {
	t.Helper()
	for _, f := range faults {
		if strings.Contains(f, want) {
			t.Logf("caught: %s", f)
			return
		}
	}
	t.Errorf("no fault holds %q; faults: %q", want, faults)
}

func qasrLoadRefused(t *testing.T, err error, want ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("loaded; want a refusal holding %q", want)
	}
	for _, s := range want {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("refusal %q does not hold %q", err, s)
		}
	}
	t.Logf("refused: %v", err)
}

func qasrPlaceAt(v any, i int) map[string]any {
	return qasrObj(v)["places"].([]any)[i].(map[string]any)
}

func qasrPlacesSection(v any) (page map[string]any, at int) {
	for _, pg := range qasrObj(v)["sheet"].([]any) {
		for j, s := range pg.(map[string]any)["sections"].([]any) {
			if _, ok := s.(map[string]any)["places"]; ok {
				return pg.(map[string]any), j
			}
		}
	}
	panic("no section shows places")
}

func TestQASRTheRulesetDeclaresD21sThreePlaces(t *testing.T) {
	w := qasrShipped(t)
	qasrNone(t, qasrPlaceFaults(qasrLoadRuleset(t, w), qasrManifest(t, w)))

	faulted := func(t *testing.T, edit func(v any) any, want string) {
		t.Helper()
		c := qasrCopy(t)
		qasrEditJSON(t, filepath.Join(c.rulesetDir, "ruleset.json"), edit)
		qasrSome(t, qasrPlaceFaults(qasrLoadRuleset(t, c), qasrManifest(t, c)), want)
	}
	t.Run("a place's copies dropped", func(t *testing.T) {
		faulted(t, func(v any) any {
			for i := range 3 {
				if p := qasrPlaceAt(v, i); len(p["forms"].([]any)) == 1 && p["copies"] == true {
					delete(p, "copies")
				}
			}
			return v
		}, "the places' shapes are")
	})
	t.Run("a place given a state", func(t *testing.T) {
		faulted(t, func(v any) any { qasrPlaceAt(v, 0)["states"] = []any{"qasr-state"}; return v }, "names states")
	})
	t.Run("the two-form place loses a form", func(t *testing.T) {
		faulted(t, func(v any) any {
			for i := range 3 {
				if p := qasrPlaceAt(v, i); len(p["forms"].([]any)) == 2 {
					p["forms"] = p["forms"].([]any)[:1]
				}
			}
			return v
		}, "the places' shapes are")
	})
	t.Run("both single-form places hold one form", func(t *testing.T) {
		faulted(t, func(v any) any {
			var single []map[string]any
			for i := range 3 {
				if p := qasrPlaceAt(v, i); len(p["forms"].([]any)) == 1 {
					single = append(single, p)
				}
			}
			single[1]["forms"] = slices.Clone(single[0]["forms"].([]any))
			return v
		}, "both single-form places hold")
	})
	t.Run("a fourth place shown", func(t *testing.T) {
		faulted(t, func(v any) any {
			m := qasrObj(v)
			extra := maps.Clone(qasrPlaceAt(v, 0))
			extra["key"] = "qasr-place"
			m["places"] = append(m["places"].([]any), extra)
			pg, j := qasrPlacesSection(v)
			s := pg["sections"].([]any)[j].(map[string]any)
			s["places"] = append(s["places"].([]any), "qasr-place")
			return v
		}, "declares 4 places")
	})
}

func TestQASRTheSheetShowsThePlacesInOneSectionAfterItsPage(t *testing.T) {
	w := qasrShipped(t)
	qasrNone(t, qasrSectionFaults(qasrLoadRuleset(t, w)))

	faulted := func(t *testing.T, edit func(v any) any, want string) {
		t.Helper()
		c := qasrCopy(t)
		qasrEditJSON(t, filepath.Join(c.rulesetDir, "ruleset.json"), edit)
		qasrSome(t, qasrSectionFaults(qasrLoadRuleset(t, c)), want)
	}
	t.Run("the section moved first on its page", func(t *testing.T) {
		faulted(t, func(v any) any {
			pg, j := qasrPlacesSection(v)
			ss := pg["sections"].([]any)
			s := ss[j]
			pg["sections"] = append([]any{s}, slices.Delete(slices.Clone(ss), j, j+1)...)
			return v
		}, "D21 after them")
	})
	t.Run("the section split in two", func(t *testing.T) {
		faulted(t, func(v any) any {
			pg, j := qasrPlacesSection(v)
			ss := pg["sections"].([]any)
			s := ss[j].(map[string]any)
			ps := s["places"].([]any)
			second := map[string]any{"name": "qasr-second", "places": ps[1:]}
			s["places"] = ps[:1]
			pg["sections"] = append(ss, second)
			return v
		}, "2 sections show places")
	})
	t.Run("a place no section shows is refused by rules.Load (VTT-1020)", func(t *testing.T) {
		c := qasrCopy(t)
		var dropped string
		qasrEditJSON(t, filepath.Join(c.rulesetDir, "ruleset.json"), func(v any) any {
			pg, j := qasrPlacesSection(v)
			s := pg["sections"].([]any)[j].(map[string]any)
			ps := s["places"].([]any)
			dropped = ps[len(ps)-1].(string)
			s["places"] = ps[:len(ps)-1]
			return v
		})
		_, err := rules.Load(c.rulesetDir)
		qasrLoadRefused(t, err, "ruleset.json", dropped)
	})
}

func TestQASREveryPlaceIsNamedAndDescribedInWords(t *testing.T) {
	w := qasrShipped(t)
	qasrNone(t, qasrWordFaults(qasrLoadRuleset(t, w)))

	for name, text := range map[string]string{"an ASCII digit": "7", "an Arabic-Indic digit": "٣"} {
		t.Run("a description holding "+name, func(t *testing.T) {
			c := qasrCopy(t)
			qasrEditJSON(t, filepath.Join(c.rulesetDir, "ruleset.json"), func(v any) any {
				p := qasrPlaceAt(v, 1)
				p["description"] = p["description"].(string) + " " + text
				return v
			})
			qasrSome(t, qasrWordFaults(qasrLoadRuleset(t, c)), "holds a digit")
		})
	}
	t.Run("a name holding a digit", func(t *testing.T) {
		c := qasrCopy(t)
		qasrEditJSON(t, filepath.Join(c.rulesetDir, "ruleset.json"), func(v any) any {
			p := qasrPlaceAt(v, 2)
			p["name"] = p["name"].(string) + " 2"
			return v
		})
		qasrSome(t, qasrWordFaults(qasrLoadRuleset(t, c)), "'s name holds a digit")
	})
	for _, field := range []string{"name", "description"} {
		t.Run("an empty "+field+" is refused by rules.Load (VTT-1016)", func(t *testing.T) {
			c := qasrCopy(t)
			qasrEditJSON(t, filepath.Join(c.rulesetDir, "ruleset.json"), func(v any) any {
				qasrPlaceAt(v, 0)[field] = ""
				return v
			})
			_, err := rules.Load(c.rulesetDir)
			qasrLoadRefused(t, err, "ruleset.json", "places[0]."+field)
		})
	}
}

func qasrActorFile(t *testing.T, w qasrWorld, pick func(refs map[string]any) bool) string {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(w.advDir, "actors", "*.json"))
	for _, p := range paths {
		var a map[string]any
		qasrReadJSON(t, p, &a)
		if refs, ok := a["references"].(map[string]any); ok && pick(refs) {
			return p
		}
	}
	t.Fatal("no actor file fits the fault")
	return ""
}

func TestQASRTheAdventureLoadsAndEveryReferenceFits(t *testing.T) {
	w := qasrShipped(t)
	rs, adv := qasrLoad(t, w)
	qasrNone(t, qasrReferenceFaults(rs, adv, qasrActorFiles(t, w.advDir)))

	anyActor := func(map[string]any) bool { return true }
	firstRef := func(refs map[string]any) (string, map[string]any) {
		ids := make([]string, 0, len(refs))
		for id := range refs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return ids[0], refs[ids[0]].(map[string]any)
	}
	t.Run("a reference id that is not its entry's", func(t *testing.T) {
		c := qasrCopy(t)
		path := qasrActorFile(t, c, anyActor)
		qasrEditJSON(t, path, func(v any) any {
			refs := qasrObj(v, "references")
			id, r := firstRef(refs)
			delete(refs, id)
			refs["qasr-"+id] = r
			return v
		})
		rs, adv := qasrLoad(t, c)
		qasrSome(t, qasrReferenceFaults(rs, adv, qasrActorFiles(t, c.advDir)), "D21 its entry's id")
	})
	t.Run("a reference given a state is refused by adventure.Load (VTT-1043)", func(t *testing.T) {
		c := qasrCopy(t)
		path := qasrActorFile(t, c, anyActor)
		var id string
		qasrEditJSON(t, path, func(v any) any {
			var r map[string]any
			id, r = firstRef(qasrObj(v, "references"))
			r["state"] = "qasr-state"
			return v
		})
		_, err := adventure.Load(c.advDir, qasrLoadRuleset(t, c))
		qasrLoadRefused(t, err, filepath.Base(path), "references", id)
	})
	t.Run("an entry the catalogue does not hold is refused by adventure.Load", func(t *testing.T) {
		c := qasrCopy(t)
		path := qasrActorFile(t, c, anyActor)
		qasrEditJSON(t, path, func(v any) any {
			_, r := firstRef(qasrObj(v, "references"))
			r["entry_id"] = "qasr-missing"
			return v
		})
		_, err := adventure.Load(c.advDir, qasrLoadRuleset(t, c))
		qasrLoadRefused(t, err, filepath.Base(path), "references", "qasr-missing")
	})
	t.Run("a form put in a place that does not hold it is refused by adventure.Load", func(t *testing.T) {
		c := qasrCopy(t)
		rs := qasrLoadRuleset(t, c)
		refID, to := "", ""
		path := qasrActorFile(t, c, func(refs map[string]any) bool {
			for _, id := range slices.Sorted(maps.Keys(refs)) {
				e := rs.Catalogue[refs[id].(map[string]any)["entry_id"].(string)]
				for _, p := range rs.Places {
					if !slices.Contains(p.Forms, e.Form) {
						refID, to = id, p.Key
						return true
					}
				}
			}
			return false
		})
		qasrEditJSON(t, path, func(v any) any {
			qasrObj(v, "references", refID)["place"] = to
			return v
		})
		_, err := adventure.Load(c.advDir, rs)
		qasrLoadRefused(t, err, filepath.Base(path), "references", refID)
	})
	t.Run("a second reference to one entry in a place holding it once is refused (VTT-1072)", func(t *testing.T) {
		c := qasrCopy(t)
		rs := qasrLoadRuleset(t, c)
		refID := ""
		path := qasrActorFile(t, c, func(refs map[string]any) bool {
			for id, r := range refs {
				if p := rs.Place(r.(map[string]any)["place"].(string)); p != nil && !p.Copies {
					refID = id
					return true
				}
			}
			return false
		})
		qasrEditJSON(t, path, func(v any) any {
			refs := qasrObj(v, "references")
			refs["zz-qasr-"+refID] = maps.Clone(refs[refID].(map[string]any))
			return v
		})
		_, err := adventure.Load(c.advDir, rs)
		qasrLoadRefused(t, err, filepath.Base(path), "references", "zz-qasr-"+refID)
	})
	t.Run("an actor holding no reference", func(t *testing.T) {
		c := qasrCopy(t)
		path := qasrActorFile(t, c, anyActor)
		qasrEditJSON(t, path, func(v any) any { delete(qasrObj(v), "references"); return v })
		rs, adv := qasrLoad(t, c)
		qasrSome(t, qasrReferenceFaults(rs, adv, qasrActorFiles(t, c.advDir)), "holds no reference")
	})
}

func TestQASRTheContentIsD21sAsWritten(t *testing.T) {
	w := qasrShipped(t)
	rs, adv := qasrLoad(t, w)
	qasrNone(t, qasrD21Faults(rs, adv))
	t.Logf("rendering:\n%s", qasrD21Rendering(rs, adv))

	faulted := func(t *testing.T, edit func(c qasrWorld)) {
		t.Helper()
		c := qasrCopy(t)
		edit(c)
		rs, adv := qasrLoad(t, c)
		qasrSome(t, qasrD21Faults(rs, adv), "is not D21's")
	}
	t.Run("an actor's two copies-holding references swap places", func(t *testing.T) {
		faulted(t, func(c qasrWorld) {
			rs := qasrLoadRuleset(t, c)
			path := qasrActorFile(t, c, func(refs map[string]any) bool {
				n := 0
				for _, r := range refs {
					if p := rs.Place(r.(map[string]any)["place"].(string)); p != nil && p.Copies {
						n++
					}
				}
				return n == 2
			})
			qasrEditJSON(t, path, func(v any) any {
				var held []map[string]any
				for _, r := range qasrObj(v, "references") {
					if rs.Place(r.(map[string]any)["place"].(string)).Copies {
						held = append(held, r.(map[string]any))
					}
				}
				held[0]["place"], held[1]["place"] = held[1]["place"], held[0]["place"]
				return v
			})
		})
	})
	t.Run("a reference dropped", func(t *testing.T) {
		faulted(t, func(c qasrWorld) {
			path := qasrActorFile(t, c, func(refs map[string]any) bool { return len(refs) == 2 })
			qasrEditJSON(t, path, func(v any) any {
				refs := qasrObj(v, "references")
				for id := range refs {
					delete(refs, id)
					break
				}
				return v
			})
		})
	})
	t.Run("the section renamed", func(t *testing.T) {
		faulted(t, func(c qasrWorld) {
			qasrEditJSON(t, filepath.Join(c.rulesetDir, "ruleset.json"), func(v any) any {
				pg, j := qasrPlacesSection(v)
				pg["sections"].([]any)[j].(map[string]any)["name"] = "qasr-section"
				return v
			})
		})
	})
}

func TestQASRTheCompiledGoldenNamesEveryActorsReferences(t *testing.T) {
	w := qasrShipped(t)
	if err := conformance.Run(w.advDir, w.rulesetsRoot); err != nil {
		t.Fatalf("conformance.Run: %v", err)
	}
	_, adv := qasrLoad(t, w)
	qasrNone(t, qasrGoldenFaults(t, w, adv))

	golden := func(c qasrWorld) string { return filepath.Join(c.advDir, "goldens", "compiled-batch.json") }
	t.Run("control: every edited file rewritten unchanged still conforms", func(t *testing.T) {
		c := qasrCopy(t)
		paths, _ := filepath.Glob(filepath.Join(c.advDir, "actors", "*.json"))
		for _, p := range append(paths, golden(c), filepath.Join(c.rulesetDir, "ruleset.json")) {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, qasrReindent(t, b), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := conformance.Run(c.advDir, c.rulesetsRoot); err != nil {
			t.Fatalf("conformance.Run over the rewritten copy: %v", err)
		}
		_, adv := qasrLoad(t, c)
		qasrNone(t, qasrGoldenFaults(t, c, adv))
		qasrNone(t, qasrNightFaults(t, c, adv))
	})
	t.Run("the golden drops an actor's reference", func(t *testing.T) {
		c := qasrCopy(t)
		qasrEditJSON(t, golden(c), func(v any) any {
			for _, e := range v.([]any) {
				if aa, ok := e.(map[string]any)["actor_added"].(map[string]any); ok {
					refs := aa["references"].(map[string]any)
					for id := range refs {
						delete(refs, id)
						break
					}
					break
				}
			}
			return v
		})
		if err := conformance.Run(c.advDir, c.rulesetsRoot); err == nil {
			t.Error("conformance.Run passed a golden missing a reference")
		} else {
			t.Logf("Run refused: %.200s", err)
		}
		_, adv := qasrLoad(t, c)
		qasrSome(t, qasrGoldenFaults(t, c, adv), "the golden gives actor")
	})
	t.Run("an actor file moves a reference the golden does not", func(t *testing.T) {
		c := qasrCopy(t)
		rs := qasrLoadRuleset(t, c)
		path := qasrActorFile(t, c, func(refs map[string]any) bool { return len(refs) == 3 })
		qasrEditJSON(t, path, func(v any) any {
			for _, r := range qasrObj(v, "references") {
				r := r.(map[string]any)
				e := rs.Catalogue[r["entry_id"].(string)]
				for _, p := range rs.Places {
					if p.Key != r["place"] && p.Copies && slices.Contains(p.Forms, e.Form) {
						r["place"] = p.Key
						return v
					}
				}
			}
			return v
		})
		if err := conformance.Run(c.advDir, c.rulesetsRoot); err == nil {
			t.Error("conformance.Run passed a golden that no longer matches the actor file")
		} else {
			t.Logf("Run refused: %.200s", err)
		}
		_, adv := qasrLoad(t, c)
		qasrSome(t, qasrGoldenFaults(t, c, adv), "the golden gives actor")
	})
}

func TestQASRTheScenarioGoldensAgreeWithCompileAndTheFold(t *testing.T) {
	w := qasrShipped(t)
	_, adv := qasrLoad(t, w)
	qasrNone(t, qasrNightFaults(t, w, adv))

	firstActorAdded := func(v any) map[string]any {
		for _, e := range v.([]any) {
			if aa, ok := e.(map[string]any)["actorAdded"].(map[string]any); ok {
				return qasrObj(aa, "actor", "references")
			}
		}
		panic("no actorAdded")
	}
	t.Run("the stream moves a reference the state does not", func(t *testing.T) {
		c := qasrCopy(t)
		qasrEditJSON(t, filepath.Join(c.goldensDir, "stream.json"), func(v any) any {
			for _, r := range firstActorAdded(v) {
				r.(map[string]any)["entryId"] = "qasr-entry"
				break
			}
			return v
		})
		f := qasrNightFaults(t, c, adv)
		qasrSome(t, f, "the fold's Actors differs from state.json's")
		qasrSome(t, f, "Compile gives")
	})
	t.Run("the state drops a reference the stream holds", func(t *testing.T) {
		c := qasrCopy(t)
		qasrEditJSON(t, filepath.Join(c.goldensDir, "state.json"), func(v any) any {
			for _, a := range qasrObj(v, "Actors") {
				refs := a.(map[string]any)["references"].(map[string]any)
				for id := range refs {
					delete(refs, id)
					break
				}
				break
			}
			return v
		})
		qasrSome(t, qasrNightFaults(t, c, adv), "the fold's Actors differs from state.json's")
	})
	t.Run("the actor file moves a reference both goldens keep", func(t *testing.T) {
		c := qasrCopy(t)
		path := qasrActorFile(t, c, func(refs map[string]any) bool { return len(refs) == 2 })
		qasrEditJSON(t, path, func(v any) any {
			refs := qasrObj(v, "references")
			for id := range refs {
				delete(refs, id)
				break
			}
			return v
		})
		_, adv := qasrLoad(t, c)
		f := qasrNightFaults(t, c, adv)
		qasrSome(t, f, "Compile gives")
		qasrSome(t, f, "the fold gives actor")
	})
}

// VTT-1059
func TestQASRTheGuidesStatblocksNameTheActorsReferences(t *testing.T) {
	w := qasrShipped(t)
	_, adv := qasrLoad(t, w)
	guide := qasrGuide(t, w)
	blocks, err := qasrStatblocks(guide)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != len(adv.Actors) {
		t.Errorf("the guide holds %d statblocks, the adventure %d actors", len(blocks), len(adv.Actors))
	}
	qasrNone(t, qasrStatblockFaults(guide, adv))

	rs := qasrLoadRuleset(t, w)
	editBlock := func(t *testing.T, id string, edit func(block string) string) qasrWorld {
		t.Helper()
		c := qasrCopy(t)
		g := filepath.Join(c.rulesetDir, "guide.md")
		s := qasrGuide(t, c)
		anchor := `"actorId": "` + id + `",` + "\n" + `  "kind"`
		if strings.Count(s, anchor) != 1 {
			t.Fatalf("the guide opens a statblock of %q %d times, want once", id, strings.Count(s, anchor))
		}
		start := strings.Index(s, anchor)
		end := start + strings.Index(s[start:], "```")
		block := s[start:end]
		edited := edit(block)
		if edited == block {
			t.Fatal("the edit changed nothing")
		}
		if err := os.WriteFile(g, []byte(s[:start]+edited+s[end:]), 0o600); err != nil {
			t.Fatal(err)
		}
		return c
	}
	for _, a := range adv.Actors {
		ids := slices.Sorted(maps.Keys(a.References))
		r := a.References[ids[0]]
		written := fmt.Sprintf(`{"entryId": %q, "place": %q}`, r.EntryID, r.Place)
		other := ""
		for _, p := range rs.Places {
			if p.Key != r.Place {
				other = p.Key
			}
		}
		t.Run("the statblock of "+a.ID+" moves a reference", func(t *testing.T) {
			c := editBlock(t, a.ID, func(block string) string {
				if strings.Count(block, written) != 1 {
					t.Fatalf("the statblock of %q does not write %s once", a.ID, written)
				}
				return strings.Replace(block, written, fmt.Sprintf(`{"entryId": %q, "place": %q}`, r.EntryID, other), 1)
			})
			qasrSome(t, qasrStatblockFaults(qasrGuide(t, c), adv), "the statblock of")
		})
		t.Run("the statblock of "+a.ID+" replaces a reference", func(t *testing.T) {
			c := editBlock(t, a.ID, func(block string) string {
				return strings.Replace(block, fmt.Sprintf(`%q: %s`, ids[0], written), `"qasr-extra": {"entryId": "qasr", "place": "qasr"}`, 1)
			})
			qasrSome(t, qasrStatblockFaults(qasrGuide(t, c), adv), "the statblock of")
		})
		t.Run("the statblock of "+a.ID+" no longer decodes as an actor", func(t *testing.T) {
			c := editBlock(t, a.ID, func(block string) string {
				return strings.Replace(block, `"actorId"`, `"actorID"`, 1)
			})
			qasrSome(t, qasrStatblockFaults(qasrGuide(t, c), adv), "no statblock describes actor")
		})
	}
}

func TestQASRTheGuideSaysHowToAddChangeAndRemoveAReference(t *testing.T) {
	w := qasrShipped(t)
	rs := qasrLoadRuleset(t, w)
	qasrNone(t, qasrGuideTextFaults(qasrGuide(t, w), rs))

	faulted := func(t *testing.T, old, repl, want string) {
		t.Helper()
		c := qasrCopy(t)
		qasrReplace(t, filepath.Join(c.rulesetDir, "guide.md"), old, repl)
		qasrSome(t, qasrGuideTextFaults(qasrGuide(t, c), rs), want)
	}
	t.Run("the no-message sentence dropped", func(t *testing.T) {
		faulted(t, "No player is sent a message", "The table is sent a message", "No player is sent a message")
	})
	t.Run("removing left unsaid", func(t *testing.T) {
		faulted(t, "**Removing.**", "**Taking.**", "**Removing.**")
	})
	t.Run("a place the guide never names", func(t *testing.T) {
		c := qasrCopy(t)
		qasrEditJSON(t, filepath.Join(c.rulesetDir, "ruleset.json"), func(v any) any {
			p := qasrPlaceAt(v, 1)
			old := p["key"]
			p["key"] = "qasr-place"
			pg, j := qasrPlacesSection(v)
			ps := pg["sections"].([]any)[j].(map[string]any)["places"].([]any)
			for i, k := range ps {
				if k == old {
					ps[i] = "qasr-place"
				}
			}
			return v
		})
		qasrSome(t, qasrGuideTextFaults(qasrGuide(t, w), qasrLoadRuleset(t, c)), "never names place `qasr-place`")
	})
	t.Run("the form-listed refusal left unsaid", func(t *testing.T) {
		faulted(t, "`add_actor` refuses a creature whose", "`add_actor` takes a creature whose", "a key a form lists is refused")
	})
	for _, f := range rs.Forms {
		key := f.Properties[0]
		t.Run("an actor file whose attributes hold "+f.Key+"'s first key does not load, as the guide says", func(t *testing.T) {
			c := qasrCopy(t)
			path := qasrActorFile(t, c, func(map[string]any) bool { return true })
			qasrEditJSON(t, path, func(v any) any {
				qasrObj(v, "attributes")[key] = json.Number("1")
				return v
			})
			_, err := adventure.Load(c.advDir, qasrLoadRuleset(t, c))
			qasrLoadRefused(t, err, filepath.Base(path), "attributes", key)
		})
	}
}
