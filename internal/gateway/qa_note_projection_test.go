package gateway_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Keep every key a string no other frame field contains, and every title and
// text of a note that is not public containing qaNPHush: qaNPScan's substring
// test over a frame's JSON is the leak detector.

const (
	qaNPKeyA   = "qanp-key-amber"
	qaNPKeyB   = "qanp-key-birch"
	qaNPKeyC   = "qanp-key-cedar"
	qaNPScene  = "qanp-scene"
	qaNPHero   = "qanp-hero"
	qaNPGoblin = "qanp-goblin"
	qaNPPlayer = "qanp-player-eyes"
	qaNPBlind  = "qanp-player-blind"
	qaNPHush   = "hush"
)

var qaNPOccurredAt = timestamppb.New(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))

type qaNPSeat struct {
	name      string
	viewer    gateway.Viewer
	projected bool
	pr        *gateway.Projector
	world     *engine.State
	fold      *engine.State
}

type qaNPTable struct {
	t          testing.TB
	seq        int64
	ref        *engine.State
	seats      []*qaNPSeat
	everPublic map[string]bool
	allKeys    []string
	stats      *qaNPStats
}

// Keep requireEach called on every sequence run, or a run that never made a
// note secret passes.
type qaNPStats struct {
	forwardedUpserts, deletesOnDeletion, deletesOnHiding, otherFrames int
}

func (st *qaNPStats) requireEach(t *testing.T) {
	t.Helper()
	t.Logf("frames to projected seats: %+v", *st)
	if st.forwardedUpserts == 0 || st.deletesOnDeletion == 0 || st.deletesOnHiding == 0 {
		t.Errorf("a transition was never exercised: %+v", *st)
	}
}

func qaNPViewers() []gateway.Viewer {
	return []gateway.Viewer{
		{ParticipantID: qaNPPlayer, Role: identity.RolePlayer},
		{ParticipantID: qaNPBlind, Role: identity.RolePlayer},
		{ParticipantID: "qanp-spec-perched", Role: identity.RoleSpectator, Viewpoint: qaNPHero},
		{ParticipantID: "qanp-spec-bare", Role: identity.RoleSpectator},
		{ParticipantID: "qanp-spec-monster", Role: identity.RoleSpectator, Viewpoint: qaNPGoblin},
		{ParticipantID: "qanp-dm", Role: identity.RoleDM},
		{ParticipantID: "qanp-agent", Role: identity.RoleAgent},
	}
}

func qaNPNewTable(tb testing.TB, viewers []gateway.Viewer) *qaNPTable {
	tb.Helper()
	table := &qaNPTable{
		t:          tb,
		ref:        engine.NewState(),
		everPublic: map[string]bool{},
		allKeys:    []string{qaNPKeyA, qaNPKeyB, qaNPKeyC},
		stats:      &qaNPStats{},
	}
	for _, v := range viewers {
		table.seats = append(table.seats, &qaNPSeat{
			name:      fmt.Sprintf("%s/%s/%q", v.Role, v.ParticipantID, v.Viewpoint),
			viewer:    v,
			projected: v.Role == identity.RolePlayer || v.Role == identity.RoleSpectator,
			pr:        gateway.NewProjector(v),
			world:     engine.NewState(),
			fold:      engine.NewState(),
		})
	}
	return table
}

func (tb *qaNPTable) prelude() {
	tb.t.Helper()
	tb.must(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: qaNPScene, Name: "Glade", GridWidth: 5, GridHeight: 5}}})
	tb.must(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: qaNPHero, Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}}})
	tb.must(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: qaNPHero, ParticipantId: qaNPPlayer, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}})
	tb.must(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "qanp-tok-hero", SceneId: qaNPScene, ActorId: qaNPHero, Position: &vttv1.GridPosition{X: 1, Y: 1}}}})
	tb.must(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: qaNPGoblin, Name: "Goblin", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}}})
	tb.must(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "qanp-tok-goblin", SceneId: qaNPScene, ActorId: qaNPGoblin, Position: &vttv1.GridPosition{X: 3, Y: 3}}}})
}

func qaNPUpsert(key string, vis vttv1.NoteVisibility) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{Key: key, Visibility: vis}}}
}

func qaNPBareDelete(key string, seq int64) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, Payload: &vttv1.Envelope_NoteDeleted{NoteDeleted: &vttv1.NoteDeleted{Key: key}}}
}

func qaNPDelete(key string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NoteDeleted{NoteDeleted: &vttv1.NoteDeleted{Key: key}}}
}

func (tb *qaNPTable) must(env *vttv1.Envelope) map[string][]*vttv1.Envelope {
	tb.t.Helper()
	out, err := tb.feed(env)
	if err != nil {
		tb.t.Fatalf("world refused %v: %v", env, err)
	}
	return out
}

// feed hands each seat the event as SPEC-015's receive does.
func (tb *qaNPTable) feed(env *vttv1.Envelope) (map[string][]*vttv1.Envelope, error) {
	tb.t.Helper()
	next := tb.seq + 1
	env.Sequence = next
	env.EventId = fmt.Sprintf("qanp-ev-%d", next)
	env.ParticipantId = "qanp-dm"
	env.ActorRole = string(identity.RoleDM)
	env.OccurredAt = qaNPOccurredAt
	if up := env.GetNoteUpserted(); up != nil {
		if up.Visibility == vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
			up.Title = fmt.Sprintf("open-%d-title", next)
			up.Text = fmt.Sprintf("open-%d-text", next)
		} else {
			up.Title = fmt.Sprintf("%s-%d-title", qaNPHush, next)
			up.Text = fmt.Sprintf("%s-%d-text", qaNPHush, next)
		}
	}
	if err := engine.Apply(tb.ref, env); err != nil {
		return nil, err
	}
	tb.seq = next
	if up := env.GetNoteUpserted(); up != nil && up.Visibility == vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
		tb.everPublic[up.Key] = true
	}
	out := map[string][]*vttv1.Envelope{}
	for _, s := range tb.seats {
		if err := engine.Apply(s.world, env); err != nil {
			tb.t.Fatalf("seat %s world refused a log event %v: %v", s.name, env, err)
		}
		envBefore := proto.Clone(env)
		notesBefore := qaNPCopyNotes(s.world.Notes)
		frames := s.pr.Project(env, s.world)
		if !proto.Equal(envBefore, env) {
			tb.t.Errorf("seq %d seat %s: Project wrote to the event", next, s.name)
		}
		if !reflect.DeepEqual(notesBefore, s.world.Notes) {
			tb.t.Errorf("seq %d seat %s: Project wrote to the state's notes", next, s.name)
		}
		out[s.name] = frames
		if !s.projected {
			if len(frames) != 1 || frames[0] != env {
				tb.t.Errorf("seq %d seat %s: want the event itself, unchanged, got %d frames %v", next, s.name, len(frames), frames)
			}
			continue
		}
		tb.checkFrames(s, env, frames)
		tb.checkFold(s)
	}
	return out, nil
}

func (tb *qaNPTable) checkFrames(s *qaNPSeat, env *vttv1.Envelope, frames []*vttv1.Envelope) {
	tb.t.Helper()
	for i, f := range frames {
		switch {
		case f.GetNoteUpserted() != nil:
			tb.stats.forwardedUpserts++
		case f.GetNoteDeleted() != nil && env.GetNoteDeleted() != nil:
			tb.stats.deletesOnDeletion++
		case f.GetNoteDeleted() != nil:
			tb.stats.deletesOnHiding++
		default:
			tb.stats.otherFrames++
		}
		if f.GetSequence() != env.GetSequence() {
			tb.t.Errorf("seq %d seat %s frame %d: carries sequence %d", env.GetSequence(), s.name, i, f.GetSequence())
		}
		qaNPScan(tb.t, fmt.Sprintf("seq %d seat %s frame %d", env.GetSequence(), s.name, i), f, tb.neverPublic())
		if del := f.GetNoteDeleted(); del != nil {
			if _, held := s.fold.Notes[del.GetKey()]; !held {
				tb.t.Errorf("seq %d seat %s: sent a NoteDeleted of %q its fold does not hold", env.GetSequence(), s.name, del.GetKey())
			}
			if f == env || !proto.Equal(f, qaNPBareDelete(del.GetKey(), env.GetSequence())) {
				tb.t.Errorf("seq %d seat %s: a NoteDeleted that is not the bare frame: %v", env.GetSequence(), s.name, f)
			}
		}
		if up := f.GetNoteUpserted(); up != nil && up.GetVisibility() != vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
			tb.t.Errorf("seq %d seat %s: sent a NoteUpserted with visibility %v", env.GetSequence(), s.name, up.GetVisibility())
		}
		if err := engine.Apply(s.fold, f); err != nil {
			tb.t.Errorf("seq %d seat %s: its fold refuses frame %d %v: %v", env.GetSequence(), s.name, i, f, err)
		}
	}
}

func (tb *qaNPTable) neverPublic() []string {
	var out []string
	for _, k := range tb.allKeys {
		if !tb.everPublic[k] {
			out = append(out, k)
		}
	}
	return out
}

func qaNPScan(tb testing.TB, where string, f *vttv1.Envelope, forbidden []string) {
	tb.Helper()
	raw, err := protojson.Marshal(f)
	if err != nil {
		tb.Fatalf("%s: marshal: %v", where, err)
	}
	js := string(raw)
	if strings.Contains(js, qaNPHush) {
		tb.Errorf("%s: a frame carries the title or text of a note that is not public: %s", where, js)
	}
	for _, k := range forbidden {
		if strings.Contains(js, k) {
			tb.Errorf("%s: a frame names %q, a key never recorded public: %s", where, k, js)
		}
	}
}

type qaNPNote struct {
	title, text string
}

func qaNPPublicNotes(st *engine.State) map[string]qaNPNote {
	out := map[string]qaNPNote{}
	for k, n := range st.Notes {
		if n.Visibility == vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
			out[k] = qaNPNote{n.Title, n.Text}
		}
	}
	return out
}

func qaNPHeldNotes(st *engine.State) map[string]qaNPNote {
	out := map[string]qaNPNote{}
	for k, n := range st.Notes {
		out[k] = qaNPNote{n.Title, n.Text}
	}
	return out
}

func qaNPCopyNotes(m map[string]engine.Note) map[string]engine.Note {
	if m == nil {
		return nil
	}
	out := make(map[string]engine.Note, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (tb *qaNPTable) checkFold(s *qaNPSeat) {
	tb.t.Helper()
	want := qaNPPublicNotes(tb.ref)
	got := qaNPHeldNotes(s.fold)
	if !reflect.DeepEqual(want, got) {
		tb.t.Errorf("after seq %d seat %s: fold holds notes %v, the world's public notes are %v", tb.seq, s.name, got, want)
	}
	for k, n := range s.fold.Notes {
		if n.Visibility != vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
			tb.t.Errorf("after seq %d seat %s: fold holds %q with visibility %v", tb.seq, s.name, k, n.Visibility)
		}
	}
}

func (tb *qaNPTable) seat(role identity.Role, id string) *qaNPSeat {
	for _, s := range tb.seats {
		if s.viewer.Role == role && s.viewer.ParticipantID == id {
			return s
		}
	}
	tb.t.Fatalf("no seat %s/%s", role, id)
	return nil
}

func (tb *qaNPTable) projectedSeats() []*qaNPSeat {
	var out []*qaNPSeat
	for _, s := range tb.seats {
		if s.projected {
			out = append(out, s)
		}
	}
	return out
}

var qaNPNonPublic = []vttv1.NoteVisibility{
	vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
	vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED,
	vttv1.NoteVisibility(3),
	vttv1.NoteVisibility(-1),
}

// VTT-233
func TestQANoteProjectionAnUnspecifiedNoteIsSentToNoPlayerOrSpectator(t *testing.T) {
	tb := qaNPNewTable(t, qaNPViewers())
	tb.prelude()
	for i := 0; i < 2; i++ {
		env := qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED)
		out := tb.must(env)
		for _, s := range tb.projectedSeats() {
			if len(out[s.name]) != 0 {
				t.Errorf("upsert %d: seat %s was sent %v for a note recorded without a visibility", i, s.name, out[s.name])
			}
			if len(s.fold.Notes) != 0 {
				t.Errorf("upsert %d: seat %s fold holds %v", i, s.name, s.fold.Notes)
			}
		}
		dm := tb.seat(identity.RoleDM, "qanp-dm")
		if len(out[dm.name]) != 1 || out[dm.name][0] != env {
			t.Errorf("upsert %d: the DM was not sent the event itself: %v", i, out[dm.name])
		}
	}
}

// VTT-234
func TestQANoteProjectionAPublicNoteReachesEveryPlayerAndSpectator(t *testing.T) {
	tb := qaNPNewTable(t, qaNPViewers())
	tb.prelude()
	for i := 0; i < 2; i++ {
		env := qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC)
		out := tb.must(env)
		for _, s := range tb.projectedSeats() {
			got := out[s.name]
			if len(got) != 1 || !proto.Equal(got[0], forwardedOf(env)) {
				t.Errorf("upsert %d: seat %s was sent %v, want the event alone, less its issuer", i, s.name, got)
				continue
			}
			n, ok := s.fold.Notes[qaNPKeyA]
			if !ok || n.Title != env.GetNoteUpserted().GetTitle() || n.Text != env.GetNoteUpserted().GetText() {
				t.Errorf("upsert %d: seat %s fold holds %v", i, s.name, s.fold.Notes)
			}
		}
	}
}

// VTT-235
func TestQANoteProjectionANoteNeverPublicIsNamedInNoFrame(t *testing.T) {
	for _, vis := range qaNPNonPublic {
		t.Run(vis.String(), func(t *testing.T) {
			tb := qaNPNewTable(t, qaNPViewers())
			tb.prelude()
			seqs := []*vttv1.Envelope{
				qaNPUpsert(qaNPKeyA, vis),
				qaNPUpsert(qaNPKeyA, vis),
				{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{TokenId: "qanp-tok-goblin", SceneId: qaNPScene, From: &vttv1.GridPosition{X: 3, Y: 3}, To: &vttv1.GridPosition{X: 2, Y: 3}}}},
				qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET),
				qaNPDelete(qaNPKeyA),
			}
			for _, env := range seqs {
				out := tb.must(env)
				if env.GetTokenMoved() != nil {
					continue
				}
				for _, s := range tb.projectedSeats() {
					if len(out[s.name]) != 0 {
						t.Errorf("seq %d: seat %s was sent %v for a note never public", env.GetSequence(), s.name, out[s.name])
					}
				}
			}
		})
	}
}

// VTT-236 VTT-221
func TestQANoteProjectionANoteThatStopsBeingPublicLeavesEveryFoldThatHeldIt(t *testing.T) {
	for _, vis := range qaNPNonPublic {
		t.Run(vis.String(), func(t *testing.T) {
			tb := qaNPNewTable(t, qaNPViewers())
			tb.prelude()
			tb.must(qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
			tb.must(qaNPUpsert(qaNPKeyB, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
			env := qaNPUpsert(qaNPKeyA, vis)
			out := tb.must(env)
			for _, s := range tb.projectedSeats() {
				got := out[s.name]
				if want := qaNPBareDelete(qaNPKeyA, env.GetSequence()); len(got) != 1 || !proto.Equal(got[0], want) {
					t.Errorf("seat %s was sent %v, want only %v", s.name, got, want)
					continue
				}
				if _, held := s.fold.Notes[qaNPKeyA]; held {
					t.Errorf("seat %s still holds %q", s.name, qaNPKeyA)
				}
				if _, held := s.fold.Notes[qaNPKeyB]; !held {
					t.Errorf("seat %s lost %q, which is still public", s.name, qaNPKeyB)
				}
			}
			out = tb.must(qaNPUpsert(qaNPKeyA, vis))
			for _, s := range tb.projectedSeats() {
				if len(out[s.name]) != 0 {
					t.Errorf("update while not public: seat %s was sent %v", s.name, out[s.name])
				}
			}
			env = qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC)
			out = tb.must(env)
			for _, s := range tb.projectedSeats() {
				if len(out[s.name]) != 1 || !proto.Equal(out[s.name][0], forwardedOf(env)) {
					t.Errorf("made public again: seat %s was sent %v", s.name, out[s.name])
				}
			}
		})
	}
}

// VTT-237
func TestQANoteProjectionADeletedPublicNoteLeavesEveryFoldThatHeldIt(t *testing.T) {
	tb := qaNPNewTable(t, qaNPViewers())
	tb.prelude()
	tb.must(qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	tb.must(qaNPUpsert(qaNPKeyB, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	env := qaNPDelete(qaNPKeyA)
	out := tb.must(env)
	for _, s := range tb.projectedSeats() {
		got := out[s.name]
		if want := qaNPBareDelete(qaNPKeyA, env.GetSequence()); len(got) != 1 || got[0] == env || !proto.Equal(got[0], want) {
			t.Errorf("seat %s was sent %v, want only %v", s.name, got, want)
		}
		if _, held := s.fold.Notes[qaNPKeyA]; held {
			t.Errorf("seat %s still holds %q", s.name, qaNPKeyA)
		}
		if _, held := s.fold.Notes[qaNPKeyB]; !held {
			t.Errorf("seat %s lost %q", s.name, qaNPKeyB)
		}
	}
}

// VTT-238
func TestQANoteProjectionADeletionReachesOnlyASeatWhoseFoldHoldsTheNote(t *testing.T) {
	cases := map[string][]*vttv1.Envelope{
		"secret then deleted": {
			qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET),
		},
		"public, made secret, then deleted": {
			qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC),
			qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET),
		},
		"public, deleted, recreated secret, then deleted": {
			qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC),
			qaNPDelete(qaNPKeyA),
			qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED),
		},
	}
	for name, pre := range cases {
		t.Run(name, func(t *testing.T) {
			tb := qaNPNewTable(t, qaNPViewers())
			tb.prelude()
			for _, env := range pre {
				tb.must(env)
			}
			out := tb.must(qaNPDelete(qaNPKeyA))
			for _, s := range tb.projectedSeats() {
				if len(out[s.name]) != 0 {
					t.Errorf("seat %s holds no %q and was sent %v", s.name, qaNPKeyA, out[s.name])
				}
			}
		})
	}
}

// VTT-176
func TestQANoteProjectionTheDMAndTheAgentAreSentEveryNoteEventUnchanged(t *testing.T) {
	tb := qaNPNewTable(t, qaNPViewers())
	tb.prelude()
	events := []*vttv1.Envelope{qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET)}
	for _, vis := range append([]vttv1.NoteVisibility{vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC}, qaNPNonPublic...) {
		events = append(events, qaNPUpsert(qaNPKeyA, vis))
	}
	events = append(events, qaNPDelete(qaNPKeyA))
	for _, env := range events {
		out := tb.must(env)
		for _, s := range []*qaNPSeat{tb.seat(identity.RoleDM, "qanp-dm"), tb.seat(identity.RoleAgent, "qanp-agent")} {
			if len(out[s.name]) != 1 || out[s.name][0] != env {
				t.Errorf("seq %d: seat %s was sent %v, want the event itself", env.GetSequence(), s.name, out[s.name])
			}
		}
	}
}

// SPEC-016: "It answers any role but those and `identity.RolePlayer` and
// `identity.RoleSpectator` with nothing".
func TestQANoteProjectionAnUnknownRoleIsSentNoNote(t *testing.T) {
	roles := []identity.Role{"gm", "", "DM", "Player"}
	world := engine.NewState()
	var prs []*gateway.Projector
	for _, r := range roles {
		prs = append(prs, gateway.NewProjector(gateway.Viewer{ParticipantID: "qanp-odd", Role: r}))
	}
	for i, env := range []*vttv1.Envelope{
		qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET),
		qaNPUpsert(qaNPKeyB, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC),
		qaNPDelete(qaNPKeyA),
	} {
		env.Sequence = int64(i + 1)
		if up := env.GetNoteUpserted(); up != nil {
			up.Title, up.Text = "t", "x"
		}
		if err := engine.Apply(world, env); err != nil {
			t.Fatal(err)
		}
		for j, pr := range prs {
			if got := pr.Project(env, world); len(got) != 0 {
				t.Errorf("role %q was sent %v", roles[j], got)
			}
		}
	}
}

// VTT-239
func TestQANoteProjectionAPlayerCannotTellADeletionFromANoteMadeSecret(t *testing.T) {
	run := func(last *vttv1.Envelope) map[string][]*vttv1.Envelope {
		tb := qaNPNewTable(t, qaNPViewers())
		tb.prelude()
		tb.must(qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
		return tb.must(last)
	}
	hidden := run(qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET))
	deleted := run(qaNPDelete(qaNPKeyA))
	for name, h := range hidden {
		d := deleted[name]
		if !strings.HasPrefix(name, string(identity.RolePlayer)) && !strings.HasPrefix(name, string(identity.RoleSpectator)) {
			continue
		}
		if len(h) != 1 || len(d) != 1 {
			t.Errorf("seat %s: made secret sent %v, deleted sent %v", name, h, d)
			continue
		}
		if !proto.Equal(h[0], d[0]) || !proto.Equal(h[0], qaNPBareDelete(qaNPKeyA, h[0].GetSequence())) {
			t.Errorf("seat %s can tell them apart:\n made secret: %v\n deleted:     %v", name, h[0], d[0])
		}
	}
}

type qaNPOp struct {
	key string
	vis vttv1.NoteVisibility
	del bool
}

func (o qaNPOp) env() *vttv1.Envelope {
	if o.del {
		return qaNPDelete(o.key)
	}
	return qaNPUpsert(o.key, o.vis)
}

func (o qaNPOp) String() string {
	if o.del {
		return "del(" + o.key + ")"
	}
	return fmt.Sprintf("%v(%s)", o.vis, o.key)
}

func qaNPAlphabet(keys []string, vis []vttv1.NoteVisibility) []qaNPOp {
	var out []qaNPOp
	for _, k := range keys {
		for _, v := range vis {
			out = append(out, qaNPOp{key: k, vis: v})
		}
		out = append(out, qaNPOp{key: k, del: true})
	}
	return out
}

func qaNPRunOps(t *testing.T, viewers []gateway.Viewer, ops []qaNPOp, stats *qaNPStats) {
	t.Helper()
	tb := qaNPNewTable(t, viewers)
	tb.stats = stats
	tb.prelude()
	for _, o := range ops {
		_, _ = tb.feed(o.env())
	}
}

func qaNPExhaust(t *testing.T, alphabet []qaNPOp, length int, viewers []gateway.Viewer) {
	t.Helper()
	stats := &qaNPStats{}
	idx := make([]int, length)
	ops := make([]qaNPOp, length)
	count := 0
	for {
		for i, j := range idx {
			ops[i] = alphabet[j]
		}
		failedBefore := t.Failed()
		qaNPRunOps(t, viewers, ops, stats)
		count++
		if t.Failed() && !failedBefore {
			t.Fatalf("first failing sequence: %v", ops)
		}
		i := length - 1
		for ; i >= 0; i-- {
			idx[i]++
			if idx[i] < len(alphabet) {
				break
			}
			idx[i] = 0
		}
		if i < 0 {
			break
		}
	}
	t.Logf("%d sequences of length %d over %d ops", count, length, len(alphabet))
	stats.requireEach(t)
}

// VTT-233 VTT-234 VTT-235 VTT-236 VTT-237 VTT-238 VTT-221 VTT-222 VTT-224
func TestQANoteProjectionEverySequenceOnTwoKeysKeepsEachFoldToThePublicNotes(t *testing.T) {
	alphabet := qaNPAlphabet([]string{qaNPKeyA, qaNPKeyB}, []vttv1.NoteVisibility{
		vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
		vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED,
	})
	qaNPExhaust(t, alphabet, 4, qaNPViewers())
}

// VTT-233 VTT-235 VTT-236 VTT-237 VTT-238
func TestQANoteProjectionEverySequenceOnOneKeyKeepsEachFoldToThePublicNotes(t *testing.T) {
	alphabet := qaNPAlphabet([]string{qaNPKeyA}, []vttv1.NoteVisibility{
		vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
		vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED,
		vttv1.NoteVisibility(3),
	})
	qaNPExhaust(t, alphabet, 5, qaNPViewers())
}

// VTT-234 VTT-235 VTT-236 VTT-237 VTT-238
func TestQANoteProjectionRandomLogsMixingSightAndNotesKeepEachFoldToThePublicNotes(t *testing.T) {
	notes := qaNPAlphabet([]string{qaNPKeyA, qaNPKeyB, qaNPKeyC}, []vttv1.NoteVisibility{
		vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
		vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED,
	})
	stats := &qaNPStats{}
	moves, grants := 0, 0
	for seed := uint64(1); seed <= 60; seed++ {
		r := rand.New(rand.NewPCG(seed, 0x51de))
		tb := qaNPNewTable(t, qaNPViewers())
		tb.stats = stats
		tb.prelude()
		pos := map[string]*vttv1.GridPosition{
			"qanp-tok-hero":   {X: 1, Y: 1},
			"qanp-tok-goblin": {X: 3, Y: 3},
		}
		granted := false
		failedBefore := t.Failed()
		var trail []string
		for step := 0; step < 40; step++ {
			var env *vttv1.Envelope
			switch r.IntN(6) {
			case 0: // a token move changes what the eyes see
				tok := "qanp-tok-hero"
				if r.IntN(2) == 0 {
					tok = "qanp-tok-goblin"
				}
				to := &vttv1.GridPosition{X: r.Int32N(5), Y: r.Int32N(5)}
				env = &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{TokenId: tok, SceneId: qaNPScene, From: pos[tok], To: to}}}
				if _, err := tb.feed(env); err == nil {
					pos[tok] = to
					moves++
				}
			case 1: // control changes who has eyes
				if granted {
					env = &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: qaNPHero, ParticipantId: qaNPBlind}}}
				} else {
					env = &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: qaNPHero, ParticipantId: qaNPBlind, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}}
				}
				if _, err := tb.feed(env); err == nil {
					granted = !granted
					grants++
				}
			default:
				env = notes[r.IntN(len(notes))].env()
				_, _ = tb.feed(env)
			}
			trail = append(trail, fmt.Sprintf("%d:%v", env.GetSequence(), env.GetPayload()))
		}
		if t.Failed() && !failedBefore {
			t.Fatalf("seed %d failed; trail:\n%s", seed, strings.Join(trail, "\n"))
		}
	}
	t.Logf("%d moves and %d control changes accepted", moves, grants)
	stats.requireEach(t)
	if moves == 0 || grants == 0 || stats.otherFrames == 0 {
		t.Errorf("sight never changed alongside the notes: %d moves, %d control changes, %+v", moves, grants, *stats)
	}
}

type qaNPWire struct {
	t          *testing.T
	c          *campaign.Campaign
	ids        *identity.DB
	hs         *httptest.Server
	seq        int64
	everPublic map[string]bool
	keys       []string
}

func qaNPNewWire(t *testing.T) *qaNPWire {
	t.Helper()
	dir := t.TempDir()
	ids, err := identity.Open(filepath.Join(dir, "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := campaign.Open(filepath.Join(dir, "campaign"))
	if err != nil {
		t.Fatal(err)
	}
	w := &qaNPWire{t: t, c: c, ids: ids, everPublic: map[string]bool{}}
	w.hs = httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() {
		w.hs.Close()
		_ = c.Close()
		_ = ids.Close()
	})
	return w
}

func (w *qaNPWire) invite(role identity.Role) string {
	w.t.Helper()
	token, _, err := w.ids.CreateInvite("qanp-"+string(role), role)
	if err != nil {
		w.t.Fatal(err)
	}
	return token
}

func (w *qaNPWire) append(env *vttv1.Envelope) int64 {
	w.t.Helper()
	env.EventId = fmt.Sprintf("qanp-wire-%d", w.seq+1)
	env.ParticipantId = "qanp-dm"
	env.ActorRole = string(identity.RoleDM)
	env.OccurredAt = qaNPOccurredAt
	if up := env.GetNoteUpserted(); up != nil {
		word := "open"
		if up.Visibility != vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
			word = qaNPHush
		} else {
			w.everPublic[up.Key] = true
		}
		up.Title, up.Text = fmt.Sprintf("%s-w%d-title", word, w.seq+1), fmt.Sprintf("%s-w%d-text", word, w.seq+1)
		w.keys = append(w.keys, up.Key)
	}
	s, err := w.c.Append(env)
	if err != nil {
		w.t.Fatalf("append %v: %v", env, err)
	}
	w.seq = s
	return s
}

func (w *qaNPWire) marker() int64 {
	return w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: fmt.Sprintf("marker after %d", w.seq)}}})
}

func (w *qaNPWire) neverPublic() []string {
	var out []string
	for _, k := range w.keys {
		if !w.everPublic[k] {
			out = append(out, k)
		}
	}
	return out
}

type qaNPClient struct {
	t        *testing.T
	w        *qaNPWire
	conn     *websocket.Conn
	frames   chan *vttv1.ServerFrame
	fold     *engine.State
	after    int64
	perchSeq []*vttv1.Envelope
	maxSeq   int64
}

func qaNPDial(ctx context.Context, w *qaNPWire, token string, after int64, fold *engine.State) *qaNPClient {
	t := w.t
	t.Helper()
	url := fmt.Sprintf("ws%s/ws?token=%s&after=%d", strings.TrimPrefix(w.hs.URL, "http"), token, after)
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	conn.SetReadLimit(1 << 22)
	c := &qaNPClient{t: t, w: w, conn: conn, frames: make(chan *vttv1.ServerFrame, 1024), fold: fold, after: after, maxSeq: after}
	go func() {
		defer close(c.frames)
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if err := protojson.Unmarshal(raw, f); err != nil {
				return
			}
			c.frames <- f
		}
	}()
	return c
}

func (c *qaNPClient) until(ctx context.Context, what string, done func(*vttv1.ServerFrame) bool) {
	c.t.Helper()
	for {
		select {
		case <-ctx.Done():
			c.t.Fatalf("waiting for %s: %v", what, ctx.Err())
		case f, ok := <-c.frames:
			if !ok {
				c.t.Fatalf("waiting for %s: connection closed", what)
			}
			if ev := f.GetEvent(); ev != nil {
				c.event(ev)
			}
			if done(f) {
				return
			}
		}
	}
}

func (c *qaNPClient) event(ev *vttv1.Envelope) {
	c.t.Helper()
	where := fmt.Sprintf("wire seq %d", ev.GetSequence())
	qaNPScan(c.t, where, ev, c.w.neverPublic())
	if ev.GetSequence() == 0 {
		c.perchSeq = append(c.perchSeq, ev)
	} else if ev.GetSequence() <= c.after {
		c.t.Errorf("%s: at or below the resume cursor %d", where, c.after)
	}
	if ev.GetSequence() > c.maxSeq {
		c.maxSeq = ev.GetSequence()
	}
	if del := ev.GetNoteDeleted(); del != nil {
		if _, held := c.fold.Notes[del.GetKey()]; !held {
			c.t.Errorf("%s: a NoteDeleted of %q the fold does not hold", where, del.GetKey())
		}
		if !proto.Equal(ev, qaNPBareDelete(del.GetKey(), ev.GetSequence())) {
			c.t.Errorf("%s: a NoteDeleted that is not the bare frame: %v", where, ev)
		}
	}
	if err := engine.Apply(c.fold, ev); err != nil {
		c.t.Errorf("%s: the fold refuses %v: %v", where, ev, err)
	}
}

func (c *qaNPClient) catchUp(ctx context.Context) {
	c.t.Helper()
	var head int64 = -1
	c.until(ctx, "catch-up", func(f *vttv1.ServerFrame) bool {
		if h := f.GetCatchUpHead(); h != nil {
			head = h.GetHeadSequence()
		}
		return head >= 0 && c.maxSeq >= head
	})
}

func (c *qaNPClient) upTo(ctx context.Context, seq int64) {
	c.t.Helper()
	c.until(ctx, fmt.Sprintf("seq %d", seq), func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetSequence() == seq
	})
}

func (c *qaNPClient) send(ctx context.Context, cmd *vttv1.ClientCommand) {
	c.t.Helper()
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		c.t.Fatal(err)
	}
	if err := c.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

func (c *qaNPClient) perch(ctx context.Context, req, actorID string, done func(*vttv1.ServerFrame) bool) {
	c.t.Helper()
	c.send(ctx, &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_SetViewpoint{SetViewpoint: &vttv1.SetViewpoint{ActorId: actorID}}})
	gotResult, gotView := false, false
	c.until(ctx, "perch "+req, func(f *vttv1.ServerFrame) bool {
		if r := f.GetResult(); r != nil && r.GetRequestId() == req {
			if !r.GetOk() {
				c.t.Fatalf("perch %s refused: %s", req, r.GetError())
			}
			gotResult = true
		}
		if done(f) {
			gotView = true
		}
		return gotResult && gotView
	})
}

func (c *qaNPClient) checkNotes(stage string) {
	c.t.Helper()
	want := qaNPPublicNotes(c.w.c.State())
	if got := qaNPHeldNotes(c.fold); !reflect.DeepEqual(want, got) {
		c.t.Errorf("%s: the fold holds %v, the public notes are %v", stage, got, want)
	}
	for _, ev := range c.perchSeq {
		if ev.GetNoteUpserted() != nil || ev.GetNoteDeleted() != nil {
			c.t.Errorf("%s: a perch sent a note frame %v", stage, ev)
		}
	}
}

func qaNPSceneSeen(scene string, lit bool) func(*vttv1.ServerFrame) bool {
	return func(f *vttv1.ServerFrame) bool {
		ev := f.GetEvent()
		ss := ev.GetSceneSeen()
		return ev.GetSequence() == 0 && ss != nil && ss.GetSceneId() == scene && (len(ss.GetVisible()) > 0) == lit
	}
}

// SPEC-016, What `transitions` sends: "A perch changes no note, so it sends
// no note frame."
func TestQANoteProjectionAPerchChangesNothingAboutNotes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	w := qaNPNewWire(t)
	token := w.invite(identity.RoleSpectator)
	for _, sc := range []string{"qanp-s1", "qanp-s2"} {
		w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: sc, Name: sc, GridWidth: 4, GridHeight: 4}}})
	}
	for i, hero := range []string{"qanp-hero-a", "qanp-hero-b"} {
		w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: hero, Name: hero, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}}})
		w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: hero, ParticipantId: "qanp-p-" + hero, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}})
		w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "qanp-tok-" + hero, SceneId: fmt.Sprintf("qanp-s%d", i+1), ActorId: hero, Position: &vttv1.GridPosition{X: 1, Y: 1}}}})
	}
	w.append(qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	w.append(qaNPUpsert(qaNPKeyB, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET))
	w.append(qaNPUpsert(qaNPKeyC, vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED))
	w.append(qaNPUpsert("qanp-key-dogwood", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	w.append(qaNPUpsert("qanp-key-dogwood", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET))
	w.append(qaNPUpsert("qanp-key-elm", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	w.append(qaNPDelete("qanp-key-elm"))

	cl := qaNPDial(ctx, w, token, 0, engine.NewState())
	defer cl.conn.CloseNow()
	cl.catchUp(ctx)
	cl.checkNotes("after catch-up")
	if _, ok := cl.fold.Notes[qaNPKeyA]; !ok {
		t.Fatalf("the unperched spectator was not sent the public note; fold %v", cl.fold.Notes)
	}

	cl.perch(ctx, "perch-a", "qanp-hero-a", qaNPSceneSeen("qanp-s1", true))
	cl.checkNotes("perched on hero a")
	cl.perch(ctx, "perch-b", "qanp-hero-b", qaNPSceneSeen("qanp-s2", true))
	cl.checkNotes("perched on hero b")
	cl.perch(ctx, "perch-none", "", qaNPSceneSeen("qanp-s2", false))
	cl.checkNotes("perched on nobody")
	cl.perch(ctx, "perch-a2", "qanp-hero-a", qaNPSceneSeen("qanp-s1", true))
	cl.checkNotes("perched on hero a again")
	if len(cl.perchSeq) == 0 {
		t.Fatal("no perch frame was observed; the perch test is vacuous")
	}

	for _, env := range []*vttv1.Envelope{
		qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET),
		qaNPUpsert("qanp-key-fir", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC),
		qaNPUpsert("qanp-key-gum", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET),
		qaNPDelete("qanp-key-gum"),
		qaNPUpsert(qaNPKeyB, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC),
	} {
		at := w.append(env)
		cl.upTo(ctx, w.marker())
		cl.checkNotes(fmt.Sprintf("perched, after seq %d", at))
	}
	cl.perch(ctx, "perch-b2", "qanp-hero-b", qaNPSceneSeen("qanp-s2", true))
	cl.checkNotes("perched on hero b after the notes moved")
}

// VTT-236 VTT-237 VTT-233
func TestQANoteProjectionAReconnectingPlayerIsSentWhatChangedAboutNotesWhileAway(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	w := qaNPNewWire(t)
	token := w.invite(identity.RolePlayer)
	w.append(qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	w.append(qaNPUpsert(qaNPKeyB, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	w.append(qaNPUpsert(qaNPKeyC, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET))
	w.append(qaNPUpsert("qanp-key-dogwood", vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED))

	fold := engine.NewState()
	first := qaNPDial(ctx, w, token, 0, fold)
	first.catchUp(ctx)
	first.checkNotes("first connection")
	if len(fold.Notes) != 2 {
		t.Fatalf("first connection holds %v, want the two public notes", fold.Notes)
	}
	cursor := first.maxSeq
	first.conn.CloseNow()

	w.append(qaNPUpsert(qaNPKeyA, vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED))
	w.append(qaNPDelete(qaNPKeyB))
	w.append(qaNPUpsert(qaNPKeyC, vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	w.append(qaNPUpsert("qanp-key-elm", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	w.append(qaNPUpsert("qanp-key-elm", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET))
	w.append(qaNPUpsert("qanp-key-fir", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET))

	second := qaNPDial(ctx, w, token, cursor, fold)
	defer second.conn.CloseNow()
	second.catchUp(ctx)
	second.upTo(ctx, w.marker())
	second.checkNotes("after reconnecting")
	if _, ok := fold.Notes[qaNPKeyC]; !ok || len(fold.Notes) != 1 {
		t.Errorf("after reconnecting the fold holds %v, want only %q", fold.Notes, qaNPKeyC)
	}
}
