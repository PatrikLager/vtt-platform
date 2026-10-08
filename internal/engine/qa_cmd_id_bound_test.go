package engine_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const qaCmdBound = 128

type qaCmdCase struct {
	label string
	id    string
}

func qaCmdAccepted() []qaCmdCase {
	return []qaCmdCase{
		{"one byte", "a"},
		{"127 ascii", strings.Repeat("a", 127)},
		{"128 ascii", "S" + strings.Repeat("a", 126) + "E"},
		{"64 two-byte runes", strings.Repeat("é", 64)},
		{"42 three-byte runes and two ascii", strings.Repeat("€", 42) + "ab"},
		{"32 four-byte runes", strings.Repeat("😀", 32)},
	}
}

func qaCmdRefused() []qaCmdCase {
	return []qaCmdCase{
		{"129 ascii", strings.Repeat("a", 129)},
		{"1000 ascii", strings.Repeat("a", 1000)},
		{"65 two-byte runes", strings.Repeat("é", 65)},
		{"one ascii then 64 two-byte runes", "a" + strings.Repeat("é", 64)},
		{"43 three-byte runes", strings.Repeat("€", 43)},
		{"32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a"},
	}
}

var (
	qaCmdParticipantField = regexp.MustCompile(`(?i)participant[ _.]?id`)
	qaCmdModuleField      = regexp.MustCompile(`(?i)module[ _.]?id`)
	qaCmdResourceField    = regexp.MustCompile(`(?i)resource`)
	qaCmdAttributeField   = regexp.MustCompile(`(?i)attribute`)
	qaCmdObjectField      = regexp.MustCompile(`(?i)object(s?\[\d+\])?[ _.]?id`)
	qaCmdSessionField     = regexp.MustCompile(`(?i)session[ _.]?id`)
	qaCmdSceneField       = regexp.MustCompile(`(?i)scene[ _.]?id`)
	qaCmdZero             = regexp.MustCompile(`\b0\b`)
	qaCmdMissingPart      = regexp.MustCompile(`^engine: \S+ requires a participant id$`)
)

func qaCmdScene(id, name string, objectIDs ...string) *vttv1.Envelope {
	objs := make([]*vttv1.SceneObject, 0, len(objectIDs))
	var col, row int32
	for _, oid := range objectIDs {
		objs = append(objs, &vttv1.SceneObject{
			ObjectId: oid, Kind: "crate", At: &vttv1.GridPosition{X: col, Y: row},
			Width: 1, Height: 1, Art: "qa-crate",
		})
		col++
		if col == 8 {
			col, row = 0, row+1
		}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: id, Name: name, GridWidth: 8, GridHeight: 8, Objects: objs,
	}}}
}

func qaCmdActor(id string, edits ...func(*vttv1.Actor)) *vttv1.Envelope {
	a := &vttv1.Actor{ActorId: id, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}
	for _, edit := range edits {
		edit(a)
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func qaCmdModule(v string) func(*vttv1.Actor) {
	return func(a *vttv1.Actor) { a.ModuleId = v }
}

func qaCmdResources(keys ...string) func(*vttv1.Actor) {
	return func(a *vttv1.Actor) {
		a.Resources = map[string]*vttv1.Resource{}
		for _, k := range keys {
			a.Resources[k] = &vttv1.Resource{Current: 1, Max: 2}
		}
	}
}

func qaCmdAttributes(keys ...string) func(*vttv1.Actor) {
	return func(a *vttv1.Actor) {
		a.Attributes = map[string]int32{}
		for _, k := range keys {
			a.Attributes[k] = 3
		}
	}
}

func qaCmdNamed(name string) func(*vttv1.Actor) {
	return func(a *vttv1.Actor) { a.Name = name }
}

func qaCmdControlled(a *vttv1.Actor) {
	a.ControllerId = "qa-participant"
	a.ControllerIds = []string{"qa-participant"}
}

func qaCmdGrant(actorID, participantID string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actorID, ParticipantId: participantID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		},
	}}
}

func qaCmdRevoke(actorID, participantID string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{
		ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: actorID, ParticipantId: participantID},
	}}
}

func qaCmdSession(sessionID, name string) *vttv1.Envelope {
	return &vttv1.Envelope{SessionId: sessionID, Payload: &vttv1.Envelope_SessionStarted{
		SessionStarted: &vttv1.SessionStarted{Name: name},
	}}
}

func qaCmdPlace(tokenID, sceneID, actorID string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tokenID, SceneId: sceneID, ActorId: actorID, Position: &vttv1.GridPosition{X: 1, Y: 1},
	}}}
}

func qaCmdMove(tokenID, sceneID string, to *vttv1.GridPosition, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tokenID, SceneId: sceneID, From: &vttv1.GridPosition{X: 1, Y: 1}, To: to, Reason: reason,
	}}}
}

func qaCmdTo() *vttv1.GridPosition { return &vttv1.GridPosition{X: 2, Y: 1} }

func qaCmdApply(t *testing.T, st *engine.State, seq int64, env *vttv1.Envelope) {
	t.Helper()
	env.EventId = "qa-cmd-" + strconv.FormatInt(seq, 10)
	env.Sequence = seq
	if err := engine.Apply(st, env); err != nil {
		t.Fatalf("event %d refused: %v", seq, err)
	}
}

func qaCmdFold(t *testing.T, envs ...*vttv1.Envelope) *engine.State {
	t.Helper()
	st := engine.NewState()
	for i, env := range envs {
		qaCmdApply(t, st, int64(i+1), env)
	}
	return st
}

func qaCmdRefusal(t *testing.T, setup func() []*vttv1.Envelope, env *vttv1.Envelope) string {
	t.Helper()
	var before []*vttv1.Envelope
	if setup != nil {
		before = setup()
	}
	st := qaCmdFold(t, before...)
	env.EventId = "qa-cmd-last"
	env.Sequence = int64(len(before) + 1)
	err := engine.Apply(st, env)
	if err == nil {
		t.Fatalf("the fold accepted an event the test expects it to refuse")
	}
	return err.Error()
}

func qaCmdNamesFieldBoundLength(t *testing.T, msg string, field *regexp.Regexp, length int, values ...string) {
	t.Helper()
	rest := msg
	for _, v := range values {
		if v != "" {
			rest = strings.ReplaceAll(rest, v, "<v>")
		}
	}
	t.Logf("refusal: %s", rest)
	if !field.MatchString(rest) {
		t.Errorf("refusal %q does not name the field %s", rest, field)
	}
	if !strings.Contains(rest, strconv.Itoa(qaCmdBound)) {
		t.Errorf("refusal %q does not name the bound %d", rest, qaCmdBound)
	}
	if length == 0 {
		if !qaCmdZero.MatchString(rest) {
			t.Errorf("refusal %q does not name the length 0", rest)
		}
	} else if !strings.Contains(rest, strconv.Itoa(length)) {
		t.Errorf("refusal %q does not name the length %d", rest, length)
	}
}

func qaCmdActorSetup() []*vttv1.Envelope { return []*vttv1.Envelope{qaCmdActor("qa-a")} }

func qaCmdBoardSetup() []*vttv1.Envelope {
	return []*vttv1.Envelope{qaCmdScene("qa-s", "qa"), qaCmdActor("qa-a"), qaCmdPlace("qa-t", "qa-s", "qa-a")}
}

// VTT-289
func TestQACmdTheGoFoldAcceptsAControlParticipantIDUpToTheBound(t *testing.T) {
	for _, c := range qaCmdAccepted() {
		t.Run(c.label, func(t *testing.T) {
			if len(c.id) > qaCmdBound {
				t.Fatalf("fixture is %d bytes", len(c.id))
			}
			st := qaCmdFold(t, qaCmdActor("qa-a"), qaCmdGrant("qa-a", "qa-other"), qaCmdGrant("qa-a", c.id))
			if !slices.Contains(st.Actors["qa-a"].GetControllerIds(), c.id) {
				t.Fatalf("granted %d-byte participant id is not held whole", len(c.id))
			}
			qaCmdApply(t, st, 4, qaCmdRevoke("qa-a", c.id))
			got := st.Actors["qa-a"].GetControllerIds()
			if slices.Contains(got, c.id) || !slices.Contains(got, "qa-other") {
				t.Fatalf("revoking the %d-byte participant id left controllers %d long", len(c.id), len(got))
			}
		})
	}
}

// VTT-289
func TestQACmdTheGoFoldRefusesAControlParticipantIDOverTheBound(t *testing.T) {
	events := []struct {
		name string
		make func(string) *vttv1.Envelope
	}{
		{"granted", func(p string) *vttv1.Envelope { return qaCmdGrant("qa-a", p) }},
		{"revoked", func(p string) *vttv1.Envelope { return qaCmdRevoke("qa-a", p) }},
	}
	for _, ev := range events {
		for _, c := range qaCmdRefused() {
			t.Run(ev.name+"/"+c.label, func(t *testing.T) {
				msg := qaCmdRefusal(t, qaCmdActorSetup, ev.make(c.id))
				qaCmdNamesFieldBoundLength(t, msg, qaCmdParticipantField, len(c.id), c.id)
			})
		}
	}
}

// VTT-290
func TestQACmdTheGoFoldAcceptsAModuleIDAndKeysUpToTheBound(t *testing.T) {
	for _, c := range qaCmdAccepted() {
		t.Run(c.label, func(t *testing.T) {
			st := qaCmdFold(t, qaCmdActor("qa-a", qaCmdModule(c.id),
				qaCmdResources("focus", c.id), qaCmdAttributes(c.id, "vim")))
			a := st.Actors["qa-a"]
			if a.GetModuleId() != c.id {
				t.Errorf("module id of %d bytes not held whole", len(c.id))
			}
			if _, ok := a.GetResources()[c.id]; !ok || len(a.GetResources()) != 2 {
				t.Errorf("resource name of %d bytes not held whole", len(c.id))
			}
			if _, ok := a.GetAttributes()[c.id]; !ok || len(a.GetAttributes()) != 2 {
				t.Errorf("attribute name of %d bytes not held whole", len(c.id))
			}
		})
	}
}

// VTT-290
func TestQACmdTheGoFoldAcceptsAnActorWithNoModuleID(t *testing.T) {
	st := qaCmdFold(t, qaCmdActor("qa-a", qaCmdModule(""), qaCmdResources("focus"), qaCmdAttributes("vim")))
	if _, ok := st.Actors["qa-a"]; !ok {
		t.Fatalf("actor with no module id not held")
	}
}

// VTT-290
func TestQACmdTheGoFoldRefusesAModuleIDOverTheBound(t *testing.T) {
	for _, c := range qaCmdRefused() {
		t.Run(c.label, func(t *testing.T) {
			msg := qaCmdRefusal(t, nil, qaCmdActor("qa-a", qaCmdModule(c.id)))
			qaCmdNamesFieldBoundLength(t, msg, qaCmdModuleField, len(c.id), c.id)
		})
	}
}

// VTT-290
func TestQACmdTheGoFoldRefusesAResourceOrAttributeNameOverTheBound(t *testing.T) {
	kinds := []struct {
		name  string
		field *regexp.Regexp
		edit  func(...string) func(*vttv1.Actor)
	}{
		{"resource", qaCmdResourceField, qaCmdResources},
		{"attribute", qaCmdAttributeField, qaCmdAttributes},
	}
	for _, k := range kinds {
		for _, c := range qaCmdRefused() {
			t.Run(k.name+"/"+c.label, func(t *testing.T) {
				msg := qaCmdRefusal(t, nil, qaCmdActor("qa-a", k.edit("qa-short", c.id, "qa-other")))
				qaCmdNamesFieldBoundLength(t, msg, k.field, len(c.id), c.id)
			})
		}
	}
}

// VTT-290
func TestQACmdTheGoFoldNamesTheLongestKeysLengthInBytes(t *testing.T) {
	ascii129, twoByte200, threeByte150 := strings.Repeat("a", 129), strings.Repeat("é", 100), strings.Repeat("€", 50)
	for _, k := range []struct {
		name string
		edit func(...string) func(*vttv1.Actor)
	}{{"resource", qaCmdResources}, {"attribute", qaCmdAttributes}} {
		t.Run(k.name, func(t *testing.T) {
			msg := qaCmdRefusal(t, nil, qaCmdActor("qa-a", k.edit(ascii129, twoByte200, threeByte150)))
			for _, v := range []string{ascii129, twoByte200, threeByte150} {
				msg = strings.ReplaceAll(msg, v, "<v>")
			}
			t.Logf("refusal: %s", msg)
			if !strings.Contains(msg, "200") {
				t.Errorf("refusal %q does not name the longest key's 200 bytes", msg)
			}
			for _, other := range []string{"129", "150"} {
				if strings.Contains(msg, other) {
					t.Errorf("refusal %q names %s, not only the longest key's length", msg, other)
				}
			}
		})
	}
}

// VTT-297
func TestQACmdTheGoFoldRefusesAnEmptyResourceOrAttributeName(t *testing.T) {
	for _, k := range []struct {
		name  string
		field *regexp.Regexp
		edit  func(...string) func(*vttv1.Actor)
	}{{"resource", qaCmdResourceField, qaCmdResources}, {"attribute", qaCmdAttributeField, qaCmdAttributes}} {
		t.Run(k.name, func(t *testing.T) {
			msg := qaCmdRefusal(t, nil, qaCmdActor("qa-a", k.edit("qa-short", "", "qa-other")))
			qaCmdNamesFieldBoundLength(t, msg, k.field, 0)
		})
	}
}

func qaCmdObjectIDs(sc engine.Scene) []string {
	ids := make([]string, 0, len(sc.Objects))
	for _, o := range sc.Objects {
		ids = append(ids, o.ObjectID)
	}
	return ids
}

// VTT-291 VTT-292
func TestQACmdTheGoFoldAcceptsDistinctObjectIDsUpToTheBound(t *testing.T) {
	for _, c := range qaCmdAccepted() {
		t.Run(c.label, func(t *testing.T) {
			ids := []string{"qa-1", c.id, "qa-3"}
			st := qaCmdFold(t, qaCmdScene("qa-s", "qa", ids...))
			if got := qaCmdObjectIDs(st.Scenes["qa-s"]); !slices.Equal(got, ids) {
				t.Fatalf("scene holds %d objects, not the three sent whole", len(got))
			}
		})
	}
	t.Run("three ids at the bound differing in the last byte", func(t *testing.T) {
		stem := strings.Repeat("é", 63) + "x"
		ids := []string{stem + "a", stem + "b", stem + "c"}
		st := qaCmdFold(t, qaCmdScene("qa-s", "qa", ids...))
		if got := qaCmdObjectIDs(st.Scenes["qa-s"]); !slices.Equal(got, ids) {
			t.Fatalf("scene holds %d objects, not the three sent whole", len(got))
		}
	})
}

// VTT-291
func TestQACmdTheGoFoldRefusesAnObjectIDOverTheBound(t *testing.T) {
	for _, c := range qaCmdRefused() {
		t.Run(c.label, func(t *testing.T) {
			msg := qaCmdRefusal(t, nil, qaCmdScene("qa-s", "qa", "qa-1", c.id, "qa-3"))
			qaCmdNamesFieldBoundLength(t, msg, qaCmdObjectField, len(c.id), c.id)
		})
	}
}

// VTT-291
func TestQACmdTheGoFoldMeasuresObjectIDsInOrder(t *testing.T) {
	first, second := strings.Repeat("a", 129), strings.Repeat("é", 100)
	msg := qaCmdRefusal(t, nil, qaCmdScene("qa-s", "qa", "qa-1", first, second))
	msg = strings.ReplaceAll(strings.ReplaceAll(msg, first, "<v>"), second, "<v>")
	t.Logf("refusal: %s", msg)
	if !strings.Contains(msg, "129") || strings.Contains(msg, "200") {
		t.Fatalf("refusal %q does not name the first long object's 129 bytes alone", msg)
	}
}

// VTT-292
func TestQACmdTheGoFoldRefusesAnEmptyObjectID(t *testing.T) {
	msg := qaCmdRefusal(t, nil, qaCmdScene("qa-s", "qa", "qa-1", "", "qa-3"))
	qaCmdNamesFieldBoundLength(t, msg, qaCmdObjectField, 0)
}

// VTT-292
func TestQACmdTheGoFoldRefusesARepeatedObjectID(t *testing.T) {
	edge := strings.Repeat("😀", 32)
	cases := []struct {
		label, scene, repeat string
		ids                  []string
	}{
		{"adjacent", "qa-s", "qa-2", []string{"qa-1", "qa-2", "qa-2"}},
		{"apart", "qa-s", "qa-2", []string{"qa-1", "qa-2", "qa-3", "qa-2"}},
		{"first and last", "qa-other-scene", "qa-1", []string{"qa-1", "qa-2", "qa-3", "qa-1"}},
		{"at the bound", "qa-s", edge, []string{edge, "qa-2", edge}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			msg := qaCmdRefusal(t, nil, qaCmdScene(c.scene, "qa", c.ids...))
			want := `engine: object "` + c.repeat + `" appears twice in scene "` + c.scene + `"`
			if msg != want {
				t.Fatalf("refusal\n got %q\nwant %q", msg, want)
			}
		})
	}
}

// VTT-293
func TestQACmdTheGoFoldBoundsASessionStartedsSessionID(t *testing.T) {
	for _, c := range qaCmdAccepted() {
		t.Run("accepts/"+c.label, func(t *testing.T) {
			st := qaCmdFold(t, qaCmdSession(c.id, "qa"))
			if len(st.Sessions) != 1 || st.Sessions[0].ID != c.id {
				t.Fatalf("session id of %d bytes not held whole", len(c.id))
			}
		})
	}
	for _, c := range qaCmdRefused() {
		t.Run("refuses/"+c.label, func(t *testing.T) {
			msg := qaCmdRefusal(t, nil, qaCmdSession(c.id, "qa"))
			qaCmdNamesFieldBoundLength(t, msg, qaCmdSessionField, len(c.id), c.id)
		})
	}
}

// VTT-293
func TestQACmdTheGoFoldBoundsATokenMovedsSceneID(t *testing.T) {
	for _, c := range qaCmdAccepted() {
		t.Run("accepts/"+c.label, func(t *testing.T) {
			st := qaCmdFold(t, qaCmdScene(c.id, "qa"), qaCmdActor("qa-a"), qaCmdPlace("qa-t", c.id, "qa-a"),
				qaCmdMove("qa-t", c.id, qaCmdTo(), ""))
			if tok := st.Tokens["qa-t"]; tok.X != 2 || tok.Y != 1 {
				t.Fatalf("token not moved: at %d,%d", tok.X, tok.Y)
			}
		})
	}
	for _, c := range qaCmdRefused() {
		t.Run("refuses/"+c.label, func(t *testing.T) {
			msg := qaCmdRefusal(t, qaCmdBoardSetup, qaCmdMove("qa-t", c.id, qaCmdTo(), ""))
			qaCmdNamesFieldBoundLength(t, msg, qaCmdSceneField, len(c.id), c.id)
		})
	}
}

// VTT-297
func TestQACmdTheGoFoldRefusesAnEmptySessionIDOrMoveSceneID(t *testing.T) {
	t.Run("session started", func(t *testing.T) {
		qaCmdNamesFieldBoundLength(t, qaCmdRefusal(t, nil, qaCmdSession("", "qa")), qaCmdSessionField, 0)
	})
	t.Run("token moved", func(t *testing.T) {
		msg := qaCmdRefusal(t, qaCmdBoardSetup, qaCmdMove("qa-t", "", qaCmdTo(), ""))
		qaCmdNamesFieldBoundLength(t, msg, qaCmdSceneField, 0)
	})
}

type qaCmdOrderCase struct {
	label       string
	setup       func() []*vttv1.Envelope
	alone, both func() *vttv1.Envelope
}

func qaCmdRunOrder(t *testing.T, cases []qaCmdOrderCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			want := qaCmdRefusal(t, c.setup, c.alone())
			got := qaCmdRefusal(t, c.setup, c.both())
			if got != want {
				t.Fatalf("refusal\n got %q\nwant %q (the earlier fault's)", got, want)
			}
		})
	}
}

// VTT-289
func TestQACmdTheGoFoldChecksAParticipantIDBeforeTheActor(t *testing.T) {
	long := strings.Repeat("é", 65)
	var cases []qaCmdOrderCase
	for name, ev := range map[string]func(string, string) *vttv1.Envelope{
		"granted": qaCmdGrant, "revoked": qaCmdRevoke,
	} {
		cases = append(cases,
			qaCmdOrderCase{name + " long participant id before an unknown actor", qaCmdActorSetup,
				func() *vttv1.Envelope { return ev("qa-a", long) },
				func() *vttv1.Envelope { return ev("qa-nobody", long) }},
			qaCmdOrderCase{name + " missing participant id before an unknown actor", qaCmdActorSetup,
				func() *vttv1.Envelope { return ev("qa-a", "") },
				func() *vttv1.Envelope { return ev("qa-nobody", "") }},
		)
		t.Run(name+" missing participant id is refused as missing", func(t *testing.T) {
			msg := qaCmdRefusal(t, qaCmdActorSetup, ev("qa-a", ""))
			if !qaCmdMissingPart.MatchString(msg) {
				t.Fatalf("refusal %q, want engine: <event> requires a participant id", msg)
			}
		})
	}
	qaCmdRunOrder(t, cases)
}

// VTT-290 VTT-297
func TestQACmdTheGoFoldChecksAnActorsModuleAndKeysAfterItsName(t *testing.T) {
	long, longName := strings.Repeat("a", 129), strings.Repeat("a", 257)
	actor := func(id string, edits ...func(*vttv1.Actor)) func() *vttv1.Envelope {
		return func() *vttv1.Envelope { return qaCmdActor(id, edits...) }
	}
	qaCmdRunOrder(t, []qaCmdOrderCase{
		{"actor id before a long module id", nil, actor(long), actor(long, qaCmdModule(long))},
		{"duplicate actor before a long module id", qaCmdActorSetup, actor("qa-a"), actor("qa-a", qaCmdModule(long))},
		{"declared controller before a long module id", nil, actor("qa-b", qaCmdControlled),
			actor("qa-b", qaCmdControlled, qaCmdModule(long))},
		{"name before a long module id", nil, actor("qa-b", qaCmdNamed(longName)),
			actor("qa-b", qaCmdNamed(longName), qaCmdModule(long))},
		{"name before an empty resource name", nil, actor("qa-b", qaCmdNamed(longName)),
			actor("qa-b", qaCmdNamed(longName), qaCmdResources(""))},
		{"long module id before a long resource name", nil, actor("qa-b", qaCmdModule(long)),
			actor("qa-b", qaCmdModule(long), qaCmdResources(long))},
		{"long module id before an empty attribute name", nil, actor("qa-b", qaCmdModule(long)),
			actor("qa-b", qaCmdModule(long), qaCmdAttributes(""))},
		{"long resource name before a long attribute name", nil, actor("qa-b", qaCmdResources(long)),
			actor("qa-b", qaCmdResources(long), qaCmdAttributes(long))},
		{"empty resource name before a long attribute name", nil, actor("qa-b", qaCmdResources("")),
			actor("qa-b", qaCmdResources(""), qaCmdAttributes(long))},
	})
}

// VTT-291 VTT-292
func TestQACmdTheGoFoldChecksObjectIDsAfterTheScenesOwnFaults(t *testing.T) {
	long, longName := strings.Repeat("€", 43), strings.Repeat("a", 257)
	scene := func(id, name string, objs ...string) func() *vttv1.Envelope {
		return func() *vttv1.Envelope { return qaCmdScene(id, name, objs...) }
	}
	sceneSetup := func() []*vttv1.Envelope { return []*vttv1.Envelope{qaCmdScene("qa-s", "qa")} }
	qaCmdRunOrder(t, []qaCmdOrderCase{
		{"scene id before a long object id", nil, scene(long, "qa"), scene(long, "qa", "qa-1", long)},
		{"scene id before a repeated object id", nil, scene(long, "qa"), scene(long, "qa", "qa-1", "qa-1")},
		{"duplicate scene before an empty object id", sceneSetup, scene("qa-s", "qa"), scene("qa-s", "qa", "qa-1", "")},
		{"duplicate scene before a repeated object id", sceneSetup, scene("qa-s", "qa"),
			scene("qa-s", "qa", "qa-1", "qa-1")},
		{"name before a long object id", nil, scene("qa-s", longName), scene("qa-s", longName, "qa-1", long)},
		{"name before a repeated object id", nil, scene("qa-s", longName), scene("qa-s", longName, "qa-1", "qa-1")},
		{"a long object id before a later repeat", nil, scene("qa-s", "qa", "qa-1", long),
			scene("qa-s", "qa", "qa-1", long, "qa-1")},
		{"an empty object id before a later repeat", nil, scene("qa-s", "qa", "qa-1", ""),
			scene("qa-s", "qa", "qa-1", "", "qa-1")},
	})
}

// VTT-293 VTT-297
func TestQACmdTheGoFoldChecksASessionIDLast(t *testing.T) {
	long, longName := strings.Repeat("a", 129), strings.Repeat("a", 257)
	session := func(id, name string) func() *vttv1.Envelope {
		return func() *vttv1.Envelope { return qaCmdSession(id, name) }
	}
	open := func() []*vttv1.Envelope { return []*vttv1.Envelope{qaCmdSession("qa-first", "qa")} }
	qaCmdRunOrder(t, []qaCmdOrderCase{
		{"open session before a long session id", open, session("qa-second", "qa"), session(long, "qa")},
		{"open session before an empty session id", open, session("qa-second", "qa"), session("", "qa")},
		{"name before a long session id", nil, session("qa-s", longName), session(long, longName)},
		{"name before an empty session id", nil, session("qa-s", longName), session("", longName)},
	})
}

// VTT-293 VTT-297
func TestQACmdTheGoFoldChecksAMovesSceneIDLast(t *testing.T) {
	long, longReason := strings.Repeat("😀", 33), strings.Repeat("a", 257)
	move := func(tok, scene string, to *vttv1.GridPosition, reason string) func() *vttv1.Envelope {
		return func() *vttv1.Envelope { return qaCmdMove(tok, scene, to, reason) }
	}
	var cases []qaCmdOrderCase
	for _, bad := range []string{long, ""} {
		cases = append(cases,
			qaCmdOrderCase{"unknown token before scene id " + strconv.Itoa(len(bad)), qaCmdBoardSetup,
				move("qa-nope", "qa-s", qaCmdTo(), ""), move("qa-nope", bad, qaCmdTo(), "")},
			qaCmdOrderCase{"no destination before scene id " + strconv.Itoa(len(bad)), qaCmdBoardSetup,
				move("qa-t", "qa-s", nil, ""), move("qa-t", bad, nil, "")},
			qaCmdOrderCase{"long reason before scene id " + strconv.Itoa(len(bad)), qaCmdBoardSetup,
				move("qa-t", "qa-s", qaCmdTo(), longReason), move("qa-t", bad, qaCmdTo(), longReason)},
		)
	}
	qaCmdRunOrder(t, cases)
}

func qaCmdToolDescription(t *testing.T, field string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "gen", "tools", "tools.json"))
	if err != nil {
		t.Fatalf("read tools.json: %v", err)
	}
	var tools []struct {
		Name        string `json:"name"`
		InputSchema struct {
			Properties map[string]struct {
				Properties map[string]struct {
					Description string `json:"description"`
				} `json:"properties"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatalf("decode tools.json: %v", err)
	}
	for _, tl := range tools {
		if tl.Name == "add_actor" {
			return tl.InputSchema.Properties["actor"].Properties[field].Description
		}
	}
	t.Fatalf("no add_actor tool in tools.json")
	return ""
}

var (
	qaCmdStatedBound = regexp.MustCompile(`At most (\d+) bytes of UTF-8`)
	qaCmdNoEmptyName = regexp.MustCompile(`no empty name`)
)

// VTT-296
func TestQACmdAddActorStatesTheBoundTheGoFoldEnforces(t *testing.T) {
	cases := []struct {
		field string
		edit  func(string) func(*vttv1.Actor)
		keys  bool
	}{
		{"moduleId", qaCmdModule, false},
		{"resources", func(v string) func(*vttv1.Actor) { return qaCmdResources("qa-short", v) }, true},
		{"attributes", func(v string) func(*vttv1.Actor) { return qaCmdAttributes("qa-short", v) }, true},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			desc := qaCmdToolDescription(t, c.field)
			m := qaCmdStatedBound.FindStringSubmatch(desc)
			if m == nil {
				t.Fatalf("add_actor's %s states no bound: %q", c.field, desc)
			}
			n, err := strconv.Atoi(m[1])
			if err != nil || n != qaCmdBound {
				t.Fatalf("add_actor's %s states %q bytes, want %d", c.field, m[1], qaCmdBound)
			}
			for _, v := range []string{strings.Repeat("a", n), strings.Repeat("é", n/2)} {
				if err := engine.Apply(engine.NewState(), qaCmdActor("qa-a", c.edit(v))); err != nil {
					t.Fatalf("the fold refuses the stated %d bytes: %v", n, err)
				}
				if engine.Apply(engine.NewState(), qaCmdActor("qa-a", c.edit(v+"a"))) == nil {
					t.Fatalf("the fold accepts one byte over the stated %d", n)
				}
			}
			if c.keys {
				if !qaCmdNoEmptyName.MatchString(desc) {
					t.Fatalf("add_actor's %s does not state that a name may not be empty: %q", c.field, desc)
				}
				if engine.Apply(engine.NewState(), qaCmdActor("qa-a", c.edit(""))) == nil {
					t.Fatalf("the fold accepts the empty %s name the tool rules out", c.field)
				}
			}
		})
	}
}
