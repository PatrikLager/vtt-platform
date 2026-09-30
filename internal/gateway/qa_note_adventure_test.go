package gateway_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaNALoadedKey = "ravine-trail-warning"

type qaNASeat struct {
	viewer gateway.Viewer
	pr     *gateway.Projector
	fold   *engine.State
	frames []*vttv1.Envelope
}

type qaNATable struct {
	log   []*vttv1.Envelope
	world *engine.State
	seats map[string]*qaNASeat
}

func qaNACompile(tb testing.TB, advID, rulesetID string) []*vttv1.Envelope {
	tb.Helper()
	rs, err := rules.Load(filepath.Join("../../rulesets", rulesetID))
	if err != nil {
		tb.Fatalf("load ruleset: %v", err)
	}
	adv, err := adventure.Load(filepath.Join("../../adventures", advID), rs)
	if err != nil {
		tb.Fatalf("load adventure: %v", err)
	}
	envs, _, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		tb.Fatalf("compile: %v", err)
	}
	return envs
}

func qaNAFileNote(tb testing.TB, advID, key string) (title, text string) {
	tb.Helper()
	files, _ := filepath.Glob(filepath.Join("../../adventures", advID, "notes", "*.json"))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			tb.Fatalf("read %s: %v", f, err)
		}
		var ns []struct{ Key, Title, Text string }
		if err := json.Unmarshal(raw, &ns); err != nil {
			tb.Fatalf("decode %s: %v", f, err)
		}
		for _, n := range ns {
			if n.Key == key {
				return n.Title, n.Text
			}
		}
	}
	tb.Fatalf("no note %q in %s's notes files", key, advID)
	return "", ""
}

func qaNAGrant(actorID, participantID string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actorID, ParticipantId: participantID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}}}
}

func qaNANote(key string, vis vttv1.NoteVisibility) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: key, Title: "DM's " + key, Text: "the watcher leaves at dusk", Visibility: vis,
	}}}
}

func qaNAFarHall() []*vttv1.Envelope {
	return []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: "far-hall", Name: "Far Hall", GridWidth: 6, GridHeight: 6}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-far", Name: "Far Walker", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "tok-far", SceneId: "far-hall", ActorId: "act-far", Position: &vttv1.GridPosition{X: 1, Y: 1}}}},
		qaNAGrant("act-far", "p-far"),
	}
}

func qaNAViewers() map[string]gateway.Viewer {
	return map[string]gateway.Viewer{
		"dm":              {ParticipantID: "dm-1", Role: identity.RoleDM},
		"agent":           {ParticipantID: "agent-1", Role: identity.RoleAgent},
		"player-fighter":  {ParticipantID: "p-fighter", Role: identity.RolePlayer},
		"player-far":      {ParticipantID: "p-far", Role: identity.RolePlayer},
		"player-nobody":   {ParticipantID: "p-nobody", Role: identity.RolePlayer},
		"spectator-free":  {ParticipantID: "s-free", Role: identity.RoleSpectator},
		"spectator-perch": {ParticipantID: "s-perch", Role: identity.RoleSpectator, Viewpoint: "act-fighter"},
	}
}

func qaNAIsTable(name string) bool {
	return strings.HasPrefix(name, "player") || strings.HasPrefix(name, "spectator")
}

func qaNARun(tb testing.TB, parts ...[]*vttv1.Envelope) *qaNATable {
	tb.Helper()
	t := &qaNATable{world: engine.NewState(), seats: map[string]*qaNASeat{}}
	for name, v := range qaNAViewers() {
		t.seats[name] = &qaNASeat{viewer: v, pr: gateway.NewProjector(v), fold: engine.NewState()}
	}
	for _, part := range parts {
		for _, e := range part {
			env := proto.Clone(e).(*vttv1.Envelope)
			env.Sequence = int64(len(t.log) + 1)
			env.EventId = fmt.Sprintf("ev-%d", env.Sequence)
			env.ActorRole = string(identity.RoleDM)
			env.ParticipantId = "dm-1"
			if err := engine.Apply(t.world, env); err != nil {
				tb.Fatalf("world fold of seq %d: %v", env.Sequence, err)
			}
			t.log = append(t.log, env)
			for name, s := range t.seats {
				for _, f := range s.pr.Project(env, t.world) {
					if err := engine.Apply(s.fold, f); err != nil {
						tb.Fatalf("%s's fold of a frame for seq %d: %v", name, env.Sequence, err)
					}
					s.frames = append(s.frames, f)
				}
			}
		}
	}
	return t
}

func qaNAGoblinTable(tb testing.TB, after ...*vttv1.Envelope) *qaNATable {
	tb.Helper()
	load := qaNACompile(tb, "goblin-ambush", "dnd45e-minimal")
	return qaNARun(tb, qaNAFarHall(), load, []*vttv1.Envelope{qaNAGrant("act-fighter", "p-fighter")}, after)
}

func qaNAAssertHoldsPublic(tb testing.TB, who string, st *engine.State, key, title, text string) {
	tb.Helper()
	n, ok := st.Notes[key]
	if !ok {
		tb.Errorf("%s's fold does not hold %q; it holds %d notes", who, key, len(st.Notes))
		return
	}
	if n.Visibility != vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
		tb.Errorf("%s's fold holds %q as %v, want NOTE_VISIBILITY_PUBLIC", who, key, n.Visibility)
	}
	if n.Title != title || n.Text != text {
		tb.Errorf("%s's fold holds %q as (%q, %q), the notes file says (%q, %q)", who, key, n.Title, n.Text, title, text)
	}
}

// VTT-240 VTT-234
func TestQANoteAdventureEveryPlayerAndSpectatorSeatHoldsTheLoadedNote(t *testing.T) {
	title, text := qaNAFileNote(t, "goblin-ambush", qaNALoadedKey)
	table := qaNAGoblinTable(t)
	if _, ok := table.seats["player-fighter"].fold.Tokens["tok-fighter"]; !ok {
		t.Fatalf("fixture: the fighter's player does not see its own token")
	}
	far := table.seats["player-far"].fold
	if _, ok := far.Tokens["tok-far"]; !ok {
		t.Fatalf("fixture: the far player does not see its own token")
	}
	if _, ok := far.Tokens["tok-fighter"]; ok {
		t.Fatalf("fixture: the far player sees into the ravine, so it proves nothing about sight")
	}
	for name, s := range table.seats {
		if !qaNAIsTable(name) {
			continue
		}
		qaNAAssertHoldsPublic(t, name, s.fold, qaNALoadedKey, title, text)
		sent := 0
		for _, f := range s.frames {
			if n := f.GetNoteUpserted(); n != nil && n.GetKey() == qaNALoadedKey {
				sent++
				if n.GetVisibility() != vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
					t.Errorf("%s was sent %q with visibility %v", name, qaNALoadedKey, n.GetVisibility())
				}
			}
		}
		if sent != 1 {
			t.Errorf("%s was sent %d NoteUpserted for %q, want 1", name, sent, qaNALoadedKey)
		}
	}
}

// VTT-240 VTT-234
func TestQANoteAdventureEveryNoteOfTheCellarAdventureReachesAPlayerWithNoCharacter(t *testing.T) {
	load := qaNACompile(t, "cellar-rats", "tavern-brawl")
	var keys []string
	for _, e := range load {
		if n := e.GetNoteUpserted(); n != nil {
			keys = append(keys, n.GetKey())
		}
	}
	if len(keys) == 0 {
		t.Fatalf("cellar-rats compiled no note")
	}
	table := qaNARun(t, load)
	for _, name := range []string{"player-nobody", "spectator-free"} {
		for _, k := range keys {
			title, text := qaNAFileNote(t, "cellar-rats", k)
			qaNAAssertHoldsPublic(t, name, table.seats[name].fold, k, title, text)
		}
	}
}

// VTT-240 VTT-176 VTT-228
func TestQANoteAdventureTheDMAndTheAgentAreSentTheLoadedNoteUnchangedAndPublic(t *testing.T) {
	title, text := qaNAFileNote(t, "goblin-ambush", qaNALoadedKey)
	table := qaNAGoblinTable(t)
	qaNAAssertHoldsPublic(t, "the world", table.world, qaNALoadedKey, title, text)
	for _, name := range []string{"dm", "agent"} {
		s := table.seats[name]
		if len(s.frames) != len(table.log) {
			t.Fatalf("%s was sent %d frames for %d events", name, len(s.frames), len(table.log))
		}
		for i := range table.log {
			if !proto.Equal(s.frames[i], table.log[i]) {
				t.Errorf("%s's frame %d differs from the event: %v vs %v", name, i, s.frames[i], table.log[i])
			}
		}
		qaNAAssertHoldsPublic(t, name, s.fold, qaNALoadedKey, title, text)
	}
}

// VTT-240 VTT-233 VTT-235
func TestQANoteAdventureADMSecretBesideTheLoadedNoteStillReachesNoPlayer(t *testing.T) {
	title, text := qaNAFileNote(t, "goblin-ambush", qaNALoadedKey)
	table := qaNAGoblinTable(t,
		qaNANote("watcher-leaves", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET),
		qaNANote("dm-scratch", vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED),
	)
	for name, s := range table.seats {
		if !qaNAIsTable(name) {
			if len(s.fold.Notes) != 3 {
				t.Errorf("%s's fold holds %d notes, want 3", name, len(s.fold.Notes))
			}
			continue
		}
		qaNAAssertHoldsPublic(t, name, s.fold, qaNALoadedKey, title, text)
		if len(s.fold.Notes) != 1 {
			t.Errorf("%s's fold holds %d notes, want only %q", name, len(s.fold.Notes), qaNALoadedKey)
		}
		for i, f := range s.frames {
			raw, err := protojson.Marshal(f)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			for _, secret := range []string{"watcher-leaves", "dm-scratch", "the watcher leaves"} {
				if strings.Contains(string(raw), secret) {
					t.Errorf("%s's frame %d names %q: %s", name, i, secret, raw)
				}
			}
		}
	}
}

// VTT-240 VTT-236
func TestQANoteAdventureTheLoadedNoteMadeSecretLeavesEveryPlayerFold(t *testing.T) {
	table := qaNAGoblinTable(t, qaNANote(qaNALoadedKey, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET))
	for name, s := range table.seats {
		_, held := s.fold.Notes[qaNALoadedKey]
		if qaNAIsTable(name) && held {
			t.Errorf("%s still holds %q after it was made secret", name, qaNALoadedKey)
		}
		if qaNAIsTable(name) {
			deleted := 0
			for _, f := range s.frames {
				if f.GetNoteDeleted().GetKey() == qaNALoadedKey {
					deleted++
				}
			}
			if deleted != 1 {
				t.Errorf("%s was sent %d NoteDeleted for %q, want 1", name, deleted, qaNALoadedKey)
			}
		}
		if !qaNAIsTable(name) && !held {
			t.Errorf("%s lost %q", name, qaNALoadedKey)
		}
	}
}
