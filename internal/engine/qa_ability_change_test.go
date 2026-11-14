package engine_test

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const qaAbWorld = `[
  {"eventId": "w1", "sequence": "1", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sessionStarted": {"name": "Night"}},
  {"eventId": "w2", "sequence": "2", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sceneCreated": {"sceneId": "scn-1", "name": "Hall", "gridWidth": 3, "gridHeight": 1,
      "tiles": {"0,0": {"kind": "floor", "material": "stone"}, "1,0": {"kind": "door", "material": "oak"},
        "2,0": {"kind": "floor", "material": "stone"}}}},
  {"eventId": "w3", "sequence": "3", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-a", "name": "Aster", "moduleId": "mod-x",
      "attributes": {"might": 3, "wit": 2}, "resources": {"stamina": {"current": 4, "max": 6}},
      "moduleData": {"tone": "calm"}, "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["mu", "kappa", "mu", "alpha", "mu"]}}},
  {"eventId": "w4", "sequence": "4", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-b", "name": "Bram", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["kappa", "mu"]}}},
  {"eventId": "w5", "sequence": "5", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-c", "name": "Crag", "kind": "ACTOR_KIND_NON_PARTY",
      "abilityIds": ["claw"]}}},
  {"eventId": "w6", "sequence": "6", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-d", "name": "Dain", "kind": "ACTOR_KIND_PARTY_MEMBER"}}},
  {"eventId": "w7", "sequence": "7", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-r", "name": "Rook", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["mu"]}}},
  {"eventId": "w8", "sequence": "8", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorRemoved": {"actorId": "act-r"}},
  {"eventId": "w9", "sequence": "9", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorControlGranted": {"actorId": "act-a", "participantId": "p-a", "kind": "ACTOR_KIND_PARTY_MEMBER"}},
  {"eventId": "w10", "sequence": "10", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorControlGranted": {"actorId": "act-a", "participantId": "p-b", "kind": "ACTOR_KIND_PARTY_MEMBER"}},
  {"eventId": "w11", "sequence": "11", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "tokenPlaced": {"tokenId": "tok-a", "sceneId": "scn-1", "actorId": "act-a", "position": {"x": 0, "y": 0},
      "name": "the tall one"}},
  {"eventId": "w12", "sequence": "12", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "tokenPlaced": {"tokenId": "tok-c", "sceneId": "scn-1", "actorId": "act-c", "position": {"x": 2, "y": 0}}},
  {"eventId": "w13", "sequence": "13", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "conditionApplied": {"actorId": "act-a", "conditionId": "dazed", "source": "trap"}},
  {"eventId": "w14", "sequence": "14", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "resourceChanged": {"actorId": "act-a", "resource": "stamina", "delta": -1, "newValue": 3}},
  {"eventId": "w15", "sequence": "15", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "doorOpened": {"sceneId": "scn-1", "at": {"x": 1, "y": 0}}},
  {"eventId": "w16", "sequence": "16", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "noteUpserted": {"key": "lore", "title": "Lore", "text": "Old", "visibility": "NOTE_VISIBILITY_PUBLIC"}}
]`

const qaAbDumpLog = `[
  {"eventId": "d1", "sequence": "1", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sessionStarted": {"name": "Night"}},
  {"eventId": "d2", "sequence": "2", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-a", "name": "Aster", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["mu", "kappa", "mu", "alpha", "mu"]}}},
  {"eventId": "d3", "sequence": "3", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-b", "name": "Bram", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["kappa"]}}},
  {"eventId": "d4", "sequence": "4", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-c", "name": "Crag", "kind": "ACTOR_KIND_NON_PARTY",
      "abilityIds": ["claw", "claw"]}}},
  {"eventId": "d5", "sequence": "5", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorControlGranted": {"actorId": "act-a", "participantId": "p-a", "kind": "ACTOR_KIND_PARTY_MEMBER"}},
  {"eventId": "d6", "sequence": "6", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityGiven": {"actorId": "act-a", "abilityId": "zeta"}},
  {"eventId": "d7", "sequence": "7", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityTaken": {"actorId": "act-a", "abilityId": "mu"}},
  {"eventId": "d8", "sequence": "8", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityGiven": {"actorId": "act-a", "abilityId": "mu"}},
  {"eventId": "d9", "sequence": "9", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityGiven": {"actorId": "act-b", "abilityId": "mu"}},
  {"eventId": "d10", "sequence": "10", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityTaken": {"actorId": "act-b", "abilityId": "kappa"}},
  {"eventId": "d11", "sequence": "11", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityTaken": {"actorId": "act-b", "abilityId": "mu"}},
  {"eventId": "d12", "sequence": "12", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityTaken": {"actorId": "act-c", "abilityId": "claw"}},
  {"eventId": "d13", "sequence": "13", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityGiven": {"actorId": "act-c", "abilityId": "bite"}}
]`

const qaAbDumpWant = `{
  "Actors": {
    "act-a": {
      "actor_id": "act-a",
      "name": "Aster",
      "controller_id": "p-a",
      "controller_ids": [
        "p-a"
      ],
      "kind": 1,
      "ability_ids": [
        "kappa",
        "alpha",
        "zeta",
        "mu"
      ]
    },
    "act-b": {
      "actor_id": "act-b",
      "name": "Bram",
      "kind": 1
    },
    "act-c": {
      "actor_id": "act-c",
      "name": "Crag",
      "kind": 2,
      "ability_ids": [
        "bite"
      ]
    }
  },
  "Conditions": {},
  "Notes": {},
  "Scenes": {},
  "Sessions": [
    {
      "ID": "sess-1",
      "Name": "Night",
      "StartSeq": 1,
      "EndSeq": 0
    }
  ],
  "Tokens": {},
  "headSequence": 13
}`

func qaAbParse(t *testing.T, raw string) []*vttv1.Envelope {
	t.Helper()
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("log: %v", err)
	}
	out := make([]*vttv1.Envelope, 0, len(items))
	for i, it := range items {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(it, env); err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		out = append(out, env)
	}
	return out
}

func qaAbFold(t *testing.T, log []*vttv1.Envelope) *engine.State {
	t.Helper()
	st := engine.NewState()
	for i, env := range log {
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("entry %d refused: %v", i, err)
		}
	}
	return st
}

type qaAbDumpShape struct {
	Actors       map[string]*vttv1.Actor
	Conditions   map[string][]engine.ActorCondition
	Notes        map[string]engine.Note
	Scenes       map[string]engine.Scene
	Sessions     []engine.Session
	Tokens       map[string]engine.Token
	HeadSequence int64 `json:"headSequence"`
}

func qaAbDump(t *testing.T, st *engine.State, head int64) string {
	t.Helper()
	b, err := json.MarshalIndent(qaAbDumpShape{
		Actors: st.Actors, Conditions: st.Conditions, Notes: st.Notes,
		Scenes: st.Scenes, Sessions: st.Sessions, Tokens: st.Tokens,
		HeadSequence: head,
	}, "", "  ")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	return string(b)
}

func qaAbRest(t *testing.T, st *engine.State) string {
	t.Helper()
	b, err := json.Marshal([]any{st.Conditions, st.Notes, st.Scenes, st.Sessions, st.Tokens})
	if err != nil {
		t.Fatalf("rest: %v", err)
	}
	return string(b)
}

func qaAbGive(seq int64, actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId: "x", Sequence: seq, SessionId: "sess-1", ActorRole: "dm", ParticipantId: "p-dm",
		Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{ActorId: actor, AbilityId: ability}},
	}
}

func qaAbTake(seq int64, actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId: "x", Sequence: seq, SessionId: "sess-1", ActorRole: "dm", ParticipantId: "p-dm",
		Payload: &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{ActorId: actor, AbilityId: ability}},
	}
}

func qaAbOnlyTheList(t *testing.T, before, after *engine.State, target string, want []string) {
	t.Helper()
	if got := after.Actors[target].GetAbilityIds(); !slices.Equal(got, want) {
		t.Fatalf("ability ids of %s = %q, want %q", target, got, want)
	}
	if len(after.Actors) != len(before.Actors) {
		t.Fatalf("actors: %d after, %d before", len(after.Actors), len(before.Actors))
	}
	for id, was := range before.Actors {
		exp := proto.Clone(was).(*vttv1.Actor)
		if id == target {
			exp.AbilityIds = want
		}
		if !proto.Equal(after.Actors[id], exp) {
			t.Errorf("actor %s = %v, want %v", id, after.Actors[id], exp)
		}
	}
	if a, b := qaAbRest(t, after), qaAbRest(t, before); a != b {
		t.Errorf("the rest of the state changed:\n got %s\nwant %s", a, b)
	}
}

// VTT-715
func TestQAAbilityGivenChangesOnlyTheActorsList(t *testing.T) {
	world := qaAbParse(t, qaAbWorld)
	before := qaAbFold(t, world)
	st := qaAbFold(t, world)
	if err := engine.Apply(st, qaAbGive(17, "act-a", "zeta")); err != nil {
		t.Fatalf("give refused: %v", err)
	}
	qaAbOnlyTheList(t, before, st, "act-a", []string{"mu", "kappa", "mu", "alpha", "mu", "zeta"})
}

// VTT-715
func TestQAAbilityGivenAppendsAtTheEndNotInOrder(t *testing.T) {
	world := qaAbParse(t, qaAbWorld)
	st := qaAbFold(t, world)
	for i, id := range []string{"zeta", "beta", "claw"} {
		if err := engine.Apply(st, qaAbGive(int64(17+i), "act-a", id)); err != nil {
			t.Fatalf("give %s refused: %v", id, err)
		}
	}
	want := []string{"mu", "kappa", "mu", "alpha", "mu", "zeta", "beta", "claw"}
	if got := st.Actors["act-a"].GetAbilityIds(); !slices.Equal(got, want) {
		t.Fatalf("ability ids = %q, want %q", got, want)
	}
	if got := st.Actors["act-c"].GetAbilityIds(); !slices.Equal(got, []string{"claw"}) {
		t.Fatalf("another actor's list changed: %q", got)
	}
}

// VTT-715
func TestQAAbilityTakenChangesOnlyTheActorsList(t *testing.T) {
	world := qaAbParse(t, qaAbWorld)
	before := qaAbFold(t, world)
	st := qaAbFold(t, world)
	if err := engine.Apply(st, qaAbTake(17, "act-a", "mu")); err != nil {
		t.Fatalf("take refused: %v", err)
	}
	qaAbOnlyTheList(t, before, st, "act-a", []string{"kappa", "alpha"})
}

// VTT-715
func TestQAAbilityTakenOfASingleIdKeepsTheOthersInOrder(t *testing.T) {
	world := qaAbParse(t, qaAbWorld)
	before := qaAbFold(t, world)
	st := qaAbFold(t, world)
	if err := engine.Apply(st, qaAbTake(17, "act-a", "kappa")); err != nil {
		t.Fatalf("take refused: %v", err)
	}
	qaAbOnlyTheList(t, before, st, "act-a", []string{"mu", "mu", "alpha", "mu"})
}

// VTT-715
func TestQAAbilityChangeOfACreatureChangesOnlyItsList(t *testing.T) {
	world := qaAbParse(t, qaAbWorld)
	before := qaAbFold(t, world)
	st := qaAbFold(t, world)
	if err := engine.Apply(st, qaAbGive(17, "act-c", "mu")); err != nil {
		t.Fatalf("give refused: %v", err)
	}
	qaAbOnlyTheList(t, before, st, "act-c", []string{"claw", "mu"})
	mid := qaAbFold(t, world)
	if err := engine.Apply(mid, qaAbGive(17, "act-c", "mu")); err != nil {
		t.Fatalf("give refused: %v", err)
	}
	if err := engine.Apply(st, qaAbTake(18, "act-c", "claw")); err != nil {
		t.Fatalf("take refused: %v", err)
	}
	qaAbOnlyTheList(t, mid, st, "act-c", []string{"mu"})
}

// VTT-715
func TestQAAbilityChangeLeavesAnEarlierListAsItWas(t *testing.T) {
	world := qaAbParse(t, qaAbWorld)
	st := qaAbFold(t, world)
	steps := []*vttv1.Envelope{
		qaAbTake(17, "act-a", "mu"), qaAbGive(18, "act-a", "zeta"), qaAbGive(19, "act-a", "beta"),
		qaAbGive(20, "act-a", "omega"), qaAbTake(21, "act-a", "kappa"), qaAbGive(22, "act-a", "mu"),
	}
	for _, env := range steps {
		held := st.Actors["act-a"].AbilityIds
		whole := slices.Clone(held[:cap(held)])
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("%v refused: %v", env.GetPayload(), err)
		}
		if !slices.Equal(held[:cap(held)], whole) {
			t.Fatalf("%v wrote into the old list: %q, was %q", env.GetPayload(), held[:cap(held)], whole)
		}
	}
	if got, want := st.Actors["act-a"].AbilityIds, []string{"alpha", "zeta", "beta", "omega", "mu"}; !slices.Equal(got, want) {
		t.Fatalf("ability ids = %q, want %q", got, want)
	}
}

type qaAbRefusal struct {
	id, name string
	prefix   []*vttv1.Envelope
	env      *vttv1.Envelope
	want     string
}

func qaAbRefusals() []qaAbRefusal {
	return []qaAbRefusal{
		{"VTT-722", "give of an empty id", nil, qaAbGive(17, "act-a", ""),
			"engine: ability_given requires an ability id"},
		{"VTT-722", "give of an empty id to an unknown actor", nil, qaAbGive(17, "ghost", ""),
			"engine: ability_given requires an ability id"},
		{"VTT-722", "give of an empty id to no actor", nil, qaAbGive(17, "", ""),
			"engine: ability_given requires an ability id"},
		{"VTT-717", "give to an unknown actor", nil, qaAbGive(17, "ghost", "zeta"),
			`engine: ability_given names unknown actor "ghost"`},
		{"VTT-717", "give to an unknown actor of an id others name", nil, qaAbGive(17, "ghost", "mu"),
			`engine: ability_given names unknown actor "ghost"`},
		{"VTT-717", "give to a removed actor", nil, qaAbGive(17, "act-r", "zeta"),
			`engine: ability_given names unknown actor "act-r"`},
		{"VTT-717", "give to no actor", nil, qaAbGive(17, "", "zeta"),
			`engine: ability_given names unknown actor ""`},
		{"VTT-717", "take from an unknown actor", nil, qaAbTake(17, "ghost", "mu"),
			`engine: ability_taken names unknown actor "ghost"`},
		{"VTT-717", "take from a removed actor of an id it named", nil, qaAbTake(17, "act-r", "mu"),
			`engine: ability_taken names unknown actor "act-r"`},
		{"VTT-717", "take of an empty id from an unknown actor", nil, qaAbTake(17, "ghost", ""),
			`engine: ability_taken names unknown actor "ghost"`},
		{"VTT-720", "give of an id the sheet repeats", nil, qaAbGive(17, "act-a", "mu"),
			`engine: actor "act-a" already names ability "mu"`},
		{"VTT-720", "give of an id the sheet names once", nil, qaAbGive(17, "act-a", "alpha"),
			`engine: actor "act-a" already names ability "alpha"`},
		{"VTT-720", "give of the first id of another sheet", nil, qaAbGive(17, "act-b", "kappa"),
			`engine: actor "act-b" already names ability "kappa"`},
		{"VTT-720", "give of an id the sheet was given", []*vttv1.Envelope{qaAbGive(17, "act-b", "zeta")},
			qaAbGive(18, "act-b", "zeta"), `engine: actor "act-b" already names ability "zeta"`},
		{"VTT-721", "take of an id only another sheet names", nil, qaAbTake(17, "act-a", "claw"),
			`engine: actor "act-a" does not name ability "claw"`},
		{"VTT-721", "take of an empty id", nil, qaAbTake(17, "act-a", ""),
			`engine: actor "act-a" does not name ability ""`},
		{"VTT-721", "take from a creature of an id it lacks", nil, qaAbTake(17, "act-c", "mu"),
			`engine: actor "act-c" does not name ability "mu"`},
		{"VTT-721", "take from a sheet with no ids", nil, qaAbTake(17, "act-d", "mu"),
			`engine: actor "act-d" does not name ability "mu"`},
		{"VTT-721", "take again of an id already taken", []*vttv1.Envelope{qaAbTake(17, "act-a", "mu")},
			qaAbTake(18, "act-a", "mu"), `engine: actor "act-a" does not name ability "mu"`},
	}
}

// VTT-717, VTT-720, VTT-721, VTT-722
func TestQAAbilityChangeRefusalsLeaveTheStateAsItWas(t *testing.T) {
	world := qaAbParse(t, qaAbWorld)
	for _, c := range qaAbRefusals() {
		t.Run(c.id+" "+c.name, func(t *testing.T) {
			log := append(slices.Clone(world), c.prefix...)
			before := qaAbFold(t, log)
			st := qaAbFold(t, log)
			err := engine.Apply(st, c.env)
			if err == nil {
				t.Fatalf("accepted; want %q", c.want)
			}
			if err.Error() != c.want {
				t.Errorf("error = %q, want %q", err.Error(), c.want)
			}
			if a, b := qaAbDump(t, st, 0), qaAbDump(t, before, 0); a != b {
				t.Errorf("a refusal changed the state:\n got %s\nwant %s", a, b)
			}
			for id, was := range before.Actors {
				if !proto.Equal(st.Actors[id], was) {
					t.Errorf("a refusal changed actor %s: %v, was %v", id, st.Actors[id], was)
				}
			}
		})
	}
}

// VTT-727: the measure, against contract/testdata/named_token_dump.json
func TestQAAbilityDumpMeasureMatchesTheRecordedDump(t *testing.T) {
	rawLog, err := os.ReadFile("../../contract/testdata/named_token_log.json")
	if err != nil {
		t.Fatal(err)
	}
	rawDump, err := os.ReadFile("../../contract/testdata/named_token_dump.json")
	if err != nil {
		t.Fatal(err)
	}
	log := qaAbParse(t, string(rawLog))
	got := qaAbDump(t, qaAbFold(t, log), log[len(log)-1].GetSequence())
	if want := strings.TrimSpace(string(rawDump)); got != want {
		t.Fatalf("the measure is not the dump's format:\n got %s\nwant %s", got, want)
	}
}

// VTT-727
func TestQAAbilityChangeDumpIsTheAgreedOne(t *testing.T) {
	log := qaAbParse(t, qaAbDumpLog)
	got := qaAbDump(t, qaAbFold(t, log), log[len(log)-1].GetSequence())
	if got != qaAbDumpWant {
		t.Fatalf("dump:\n got %s\nwant %s", got, qaAbDumpWant)
	}
}

// VTT-727
func TestQAAbilityChangeAcceptsWhatTheClientAccepts(t *testing.T) {
	world := qaAbParse(t, qaAbWorld)
	cases := []struct {
		actor string
		envs  []*vttv1.Envelope
		want  []string
	}{
		{"act-a", []*vttv1.Envelope{qaAbGive(17, "act-a", "claw")}, []string{"mu", "kappa", "mu", "alpha", "mu", "claw"}},
		{"act-d", []*vttv1.Envelope{qaAbGive(17, "act-d", "mu")}, []string{"mu"}},
		{"act-b", []*vttv1.Envelope{qaAbTake(17, "act-b", "mu"), qaAbGive(18, "act-b", "mu")}, []string{"kappa", "mu"}},
	}
	for _, c := range cases {
		st := qaAbFold(t, append(slices.Clone(world), c.envs...))
		if got := st.Actors[c.actor].GetAbilityIds(); !slices.Equal(got, c.want) {
			t.Errorf("ability ids of %s = %q, want %q", c.actor, got, c.want)
		}
	}
}
