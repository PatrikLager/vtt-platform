package campaign_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/eventgen"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// oracleSeat is a decision through one character's eyes that remembers what it
// gave, kept apart from the fan-out's own memory of the log.
type oracleSeat struct {
	eyes     perceive.Eyes
	sent     []*vttv1.Envelope
	decision *perceive.Projection
}

func (s *oracleSeat) project(t *testing.T, env *vttv1.Envelope, st *engine.State) []*vttv1.Envelope {
	t.Helper()
	shown, err := campaign.FoldPrefix(s.sent)
	if err != nil {
		t.Fatalf("the oracle's own frames do not fold: %v", err)
	}
	out := s.decision.Event(s.eyes, env, shown, st)
	s.sent = append(s.sent, out...)
	return out
}

func readEnvelopes(t *testing.T, path string) []*vttv1.Envelope {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var msgs []json.RawMessage
	if err := json.Unmarshal(raw, &msgs); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := make([]*vttv1.Envelope, 0, len(msgs))
	for i, m := range msgs {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(m, env); err != nil {
			t.Fatalf("%s[%d]: %v", path, i, err)
		}
		out = append(out, env)
	}
	return out
}

// logOracle appends events and, after each event, compares every party
// member's new log entries with what a fresh decision through its eyes, fed the
// world event by event and its own frames, gives.
type logOracle struct {
	t        *testing.T
	c        *campaign.Campaign
	landed   []*vttv1.Envelope
	ids      map[string]bool
	member   map[string]bool
	seats    map[string]*oracleSeat
	compared map[string]int
	resumed  map[string]bool
}

func newLogOracle(t *testing.T) *logOracle {
	t.Helper()
	c, err := campaign.Open(filepath.Join(t.TempDir(), "campaign"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return &logOracle{t: t, c: c, ids: map[string]bool{}, member: map[string]bool{},
		seats: map[string]*oracleSeat{}, compared: map[string]int{}, resumed: map[string]bool{}}
}

func (o *logOracle) log(id string) []campaign.CharacterEntry {
	o.t.Helper()
	entries, err := o.c.CharacterLog(id)
	if err != nil {
		o.t.Fatal(err)
	}
	return entries
}

// appendEvents appends envs as one batch and checks the logs event by event;
// it reports whether the batch landed and the first sequence it was given.
func (o *logOracle) appendEvents(envs []*vttv1.Envelope) (int64, bool) {
	o.t.Helper()
	for _, env := range envs {
		if a := env.GetActorAdded().GetActor(); a != nil {
			o.ids[a.GetActorId()] = true
		}
	}
	held := map[string]int{}
	for id := range o.ids {
		held[id] = len(o.log(id))
	}
	first, err := o.c.AppendBatch(envs)
	if err != nil {
		for id, n := range held {
			if got := len(o.log(id)); got != n {
				o.t.Fatalf("a refused batch grew %s's log from %d to %d", id, n, got)
			}
		}
		return 0, false
	}
	logs := map[string][]campaign.CharacterEntry{}
	at := map[string]int{}
	for id := range o.ids {
		logs[id] = o.log(id)
		at[id] = held[id]
	}
	for k, env := range envs {
		o.landed = append(o.landed, env)
		st, err := campaign.FoldPrefix(o.landed)
		if err != nil {
			o.t.Fatal(err)
		}
		for id := range o.ids {
			o.event(id, first+int64(k), env, st, logs[id], at)
		}
	}
	for id := range o.ids {
		if at[id] != len(logs[id]) {
			o.t.Fatalf("batch at %d: %s's log holds %d entries no event of it accounts for", first, id, len(logs[id])-at[id])
		}
	}
	return first, true
}

// event checks id's entries for the world event seq, from entries[at[id]].
func (o *logOracle) event(id string, seq int64, env *vttv1.Envelope, st *engine.State, entries []campaign.CharacterEntry, at map[string]int) {
	o.t.Helper()
	a, ok := st.Actors[id]
	if !ok || !engine.IsPartyMember(a) {
		o.member[id] = false
		return
	}
	if !o.member[id] {
		// Seed only here: seeded anywhere else, the seat forgets what the log's
		// decision remembers (VTT-319).
		lead := at[id]
		for lead < len(entries) && entries[lead].WorldSeq == 0 {
			switch entries[lead].Env.GetPayload().(type) {
			case *vttv1.Envelope_SceneCreated, *vttv1.Envelope_SceneSeen,
				*vttv1.Envelope_TokenPlaced, *vttv1.Envelope_TokenMoved,
				*vttv1.Envelope_DoorOpened, *vttv1.Envelope_DoorClosed,
				*vttv1.Envelope_NarrationAdded:
				o.t.Fatalf("world event %d: %s's opening or resume holds a board frame %v", seq, id, entries[lead].Env)
			}
			lead++
		}
		if o.seats[id] != nil {
			o.resumed[id] = true
		}
		sent := make([]*vttv1.Envelope, 0, lead)
		for _, e := range entries[:lead] {
			sent = append(sent, e.Env)
		}
		o.seats[id] = &oracleSeat{eyes: perceive.Eyes{Shoulder: id}, sent: sent,
			decision: perceive.New(perceive.Eyes{Shoulder: id}, nil)}
		at[id] = lead
		o.member[id] = true
	}
	frames := o.seats[id].project(o.t, env, st)
	if at[id]+len(frames) > len(entries) {
		o.t.Fatalf("world event %d: %s's log holds %d more entries, the perched spectator is sent %d: %v",
			seq, id, len(entries)-at[id], len(frames), frames)
	}
	for i, frame := range frames {
		e := entries[at[id]+i]
		if !sameFrame(e.Env, frame) || e.WorldSeq != seq {
			o.t.Fatalf("world event %d: %s's entry %d is %v (world %d), the oracle's frame %v",
				seq, id, at[id]+i+1, e.Env, e.WorldSeq, frame)
		}
	}
	at[id] += len(frames)
	if len(frames) > 0 {
		o.compared[id]++
	}
}

// sameFrame compares a log entry with the oracle's frame, apart from the
// sequence each carries and a narration's anchor range (VTT-319, VTT-322).
func sameFrame(entry, frame *vttv1.Envelope) bool {
	a, b := proto.Clone(entry).(*vttv1.Envelope), proto.Clone(frame).(*vttv1.Envelope)
	a.Sequence, b.Sequence = 0, 0
	for _, e := range []*vttv1.Envelope{a, b} {
		if n := e.GetNarrationAdded(); n != nil {
			n.AnchorFromSeq, n.AnchorToSeq = 0, 0
		}
	}
	return proto.Equal(a, b)
}

// logsCompared is how many non-empty logs were compared at an event with
// frames, failing the test for one that never was.
func (o *logOracle) logsCompared() int {
	o.t.Helper()
	n := 0
	for id := range o.ids {
		if len(o.log(id)) == 0 {
			continue
		}
		if o.compared[id] == 0 {
			o.t.Errorf("%s's log was never compared at an event with frames", id)
		}
		n++
	}
	return n
}

// inChunks appends envs in batches of one to four events.
func (o *logOracle) inChunks(envs []*vttv1.Envelope) {
	o.t.Helper()
	for i, size := 0, 1; i < len(envs); i, size = i+size, size%4+1 {
		end := min(i+size, len(envs))
		if _, ok := o.appendEvents(envs[i:end]); !ok {
			o.t.Fatalf("events %d to %d were refused", i+1, end)
		}
	}
}

var oracleID int

func oracleEnv(payload proto.Message) *vttv1.Envelope {
	oracleID++
	return cenv(fmt.Sprintf("oracle-%d", oracleID), payload)
}

// pausingHistory pauses the hero by a grant and resumes it, then removes the
// scout and adds another party member under its id.
func pausingHistory() []*vttv1.Envelope {
	at := func(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 7; x++ {
		tiles[fmt.Sprintf("%d,0", x)] = &vttv1.TileRef{Kind: "wall"}
		tiles[fmt.Sprintf("%d,2", x)] = &vttv1.TileRef{Kind: "wall"}
	}
	tiles["3,1"] = &vttv1.TileRef{Kind: "door"}
	var out []*vttv1.Envelope
	for _, p := range []proto.Message{
		&vttv1.SessionStarted{Name: "one"},
		&vttv1.SceneCreated{SceneId: "s", Name: "Hall", GridWidth: 7, GridHeight: 3, Tiles: tiles},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "hero", Kind: party}},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "scout", Kind: party}},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "goblin", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero", Position: at(1, 1)},
		&vttv1.TokenPlaced{TokenId: "t-scout", SceneId: "s", ActorId: "scout", Position: at(2, 1)},
		&vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s", ActorId: "goblin", Position: at(5, 1)},
		&vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		&vttv1.NarrationAdded{Text: "while possessed"},
		&vttv1.SessionEnded{},
		&vttv1.SessionStarted{Name: "two"},
		&vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", From: at(5, 1), To: at(2, 1)},
		&vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s", From: at(1, 1), To: at(0, 1)},
		&vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1", Kind: party},
		&vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"},
		&vttv1.TokenRemoved{TokenId: "t-scout"},
		&vttv1.ActorRemoved{ActorId: "scout"},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Second scout", Kind: party}},
		&vttv1.TokenPlaced{TokenId: "t-scout-2", SceneId: "s", ActorId: "scout", Position: at(1, 1)},
		&vttv1.NarrationAdded{Text: "the scout returns"},
	} {
		out = append(out, oracleEnv(p))
	}
	return out
}

// VTT-319
func TestACharacterLogIsWhatItsCharactersEyesAreGiven(t *testing.T) {
	ran, seedLogs, corpusLogs := 0, 0, 0
	for seed := int64(1); seed <= 6; seed++ {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			ran++
			o := newLogOracle(t)
			rng := rand.New(rand.NewSource(seed))
			m := eventgen.New()
			for i := 0; i < 200; i++ {
				a := m.Step(rng, i)
				if a.Env == nil {
					continue
				}
				seq, ok := o.appendEvents([]*vttv1.Envelope{a.Env})
				if ok == a.MustFail {
					t.Fatalf("action %d (%s): landed %v, the model expected %v", i, a.Kind, ok, !a.MustFail)
				}
				if ok {
					m.Accepted(a, seq)
				}
			}
			seedLogs += o.logsCompared()
		})
	}
	streams, err := filepath.Glob(filepath.Join("..", "..", "scenarios", "goldens", "*", "stream.json"))
	if err != nil || len(streams) == 0 {
		t.Fatalf("no corpus streams (%v)", err)
	}
	for _, path := range streams {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			ran++
			o := newLogOracle(t)
			envs := readEnvelopes(t, path)
			for _, env := range envs {
				env.Sequence = 0
			}
			o.inChunks(envs)
			corpusLogs += o.logsCompared()
		})
	}
	t.Run("a pause and a resume, in batches", func(t *testing.T) {
		ran++
		o := newLogOracle(t)
		o.inChunks(pausingHistory())
		o.logsCompared()
		if !o.resumed["hero"] || !o.resumed["scout"] {
			t.Fatalf("the history resumed %v, want the hero and the scout", o.resumed)
		}
	})
	// Judge the sources only on a whole, passing run: a filtered one proves less.
	if ran == 6+len(streams)+1 && !t.Failed() && (seedLogs < 2 || corpusLogs < 2) {
		t.Errorf("too few logs compared: %d from the seeds, %d from the corpus, want 2 of each", seedLogs, corpusLogs)
	}
}
