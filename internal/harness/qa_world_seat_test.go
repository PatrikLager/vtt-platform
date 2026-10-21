package harness_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"unicode"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

type qaWSConn struct {
	hub      *qaWSHub
	name     string
	ch       chan harness.Delivery
	closed   bool
	closeErr error
}

func (c *qaWSConn) SendCommand(_ context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	return c.hub.handle(c, cmd)
}

func (c *qaWSConn) Events() <-chan harness.Delivery { return c.ch }

func (c *qaWSConn) CloseErr() error {
	c.hub.mu.Lock()
	defer c.hub.mu.Unlock()
	return c.closeErr
}

func (c *qaWSConn) Close() error {
	c.hub.mu.Lock()
	defer c.hub.mu.Unlock()
	c.hub.endLocked(c, errors.New("qa: closed by the client"))
	return nil
}

type qaWSHub struct {
	mu       sync.Mutex
	roles    map[string]string
	conns    []*qaWSConn
	log      []*vttv1.Envelope
	head     int64
	overflow bool
	strays   int
	serve    func(c *qaWSConn, cmd *vttv1.ClientCommand) *vttv1.CommandResult
}

func (h *qaWSHub) dial(name string, after int64, _ string) (harness.Conn, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.roles[name]; !ok {
		return nil, fmt.Errorf("qa: no participant %q", name)
	}
	c := &qaWSConn{hub: h, name: name, ch: make(chan harness.Delivery, 1<<14)}
	h.conns = append(h.conns, c)
	for _, env := range h.log {
		if env.GetSequence() > after {
			h.deliverLocked(c, env)
		}
	}
	return c, nil
}

func (h *qaWSHub) handle(c *qaWSConn, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.closed {
		return nil, errors.New("qa: send on a closed connection")
	}
	res := h.serve(c, cmd)
	res.RequestId = cmd.GetRequestId()
	return res, nil
}

func (h *qaWSHub) deliverLocked(c *qaWSConn, env *vttv1.Envelope) {
	if c.closed {
		return
	}
	select {
	case c.ch <- harness.Delivery{Envelope: env}:
	default:
		h.overflow = true
	}
}

func (h *qaWSHub) endLocked(c *qaWSConn, why error) {
	if c.closed {
		return
	}
	c.closed = true
	c.closeErr = why
	close(c.ch)
}

func (h *qaWSHub) liveLocked() []*qaWSConn {
	var out []*qaWSConn
	for _, c := range h.conns {
		if !c.closed {
			out = append(out, c)
		}
	}
	return out
}

func (h *qaWSHub) sendLocked(to []string, seq int64) {
	for _, c := range h.liveLocked() {
		if slices.Contains(to, c.name) {
			h.strays++
			h.deliverLocked(c, &vttv1.Envelope{
				EventId:  fmt.Sprintf("qa-stray-%s-%d", c.name, seq),
				Sequence: seq,
				Payload: &vttv1.Envelope_NarrationAdded{
					NarrationAdded: &vttv1.NarrationAdded{Text: "qa stray"}},
			})
		}
	}
}

func qaWSIsCharacter(role string) bool { return role == "player" || role == "spectator" }

type qaWSWhen int

const (
	qaWSAtNextCommand qaWSWhen = iota
	qaWSAfterDelay
)

type qaWSDrop struct {
	to   []string
	seq  int64
	when qaWSWhen
}

type qaWSAct struct {
	consume int64
	drops   []qaWSDrop
	end     []string
}

type qaWSTable struct {
	qaWSHub
	denials map[string]qaWSAct
	own     map[string]int64
	queued  []qaWSDrop
}

func qaWSNewTable(parts []harness.Participant, denials map[string]qaWSAct) *qaWSTable {
	t := &qaWSTable{denials: denials, own: map[string]int64{}}
	t.roles = map[string]string{}
	for _, p := range parts {
		t.roles[p.Name] = p.Role
	}
	t.serve = t.serveStep
	return t
}

func (t *qaWSTable) serveStep(c *qaWSConn, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	for _, d := range t.queued {
		t.sendLocked(d.to, d.seq)
	}
	t.queued = nil
	tag := cmd.GetAddNarration().GetText()
	if act, denied := t.denials[tag]; denied {
		t.head += act.consume
		for _, d := range act.drops {
			switch d.when {
			case qaWSAtNextCommand:
				t.queued = append(t.queued, d)
			case qaWSAfterDelay:
				time.AfterFunc(time.Millisecond, func() {
					t.mu.Lock()
					defer t.mu.Unlock()
					t.sendLocked(d.to, d.seq)
				})
			}
		}
		for _, lc := range t.liveLocked() {
			if slices.Contains(act.end, lc.name) {
				t.endLocked(lc, errors.New("qa: the server ended the stream"))
			}
		}
		return &vttv1.CommandResult{Error: "qa-refused: " + tag}
	}
	t.head++
	world := &vttv1.Envelope{
		EventId:  fmt.Sprintf("qa-world-%d", t.head),
		Sequence: t.head,
		Payload: &vttv1.Envelope_NarrationAdded{
			NarrationAdded: &vttv1.NarrationAdded{Text: tag}},
	}
	t.log = append(t.log, world)
	for _, lc := range t.liveLocked() {
		if !qaWSIsCharacter(t.roles[lc.name]) {
			t.deliverLocked(lc, world)
			continue
		}
		t.own[lc.name]++
		t.deliverLocked(lc, &vttv1.Envelope{
			EventId:  fmt.Sprintf("qa-own-%s-%d", lc.name, t.own[lc.name]),
			Sequence: t.own[lc.name],
			Payload: &vttv1.Envelope_NarrationAdded{
				NarrationAdded: &vttv1.NarrationAdded{Text: tag}},
		})
	}
	return &vttv1.CommandResult{Ok: true, Sequence: t.head}
}

func qaWSAccept(by, tag string) harness.Step {
	return harness.Step{By: by, Command: qaWSNarration(tag), Expect: &harness.Expect{OK: true}}
}

func qaWSRefuse(by, tag string) harness.Step {
	return harness.Step{
		By:      by,
		Command: qaWSNarration(tag),
		Expect:  &harness.Expect{DeniedContaining: "qa-refused"},
	}
}

func qaWSNarration(tag string) json.RawMessage {
	b, err := json.Marshal(map[string]any{"addNarration": map[string]string{"text": tag}})
	if err != nil {
		panic(err)
	}
	return b
}

func qaWSParts(nameRole ...string) []harness.Participant {
	var out []harness.Participant
	for i := 0; i+1 < len(nameRole); i += 2 {
		out = append(out, harness.Participant{Name: nameRole[i], Role: nameRole[i+1]})
	}
	return out
}

func qaWSRun(t *testing.T, parts []harness.Participant, steps []harness.Step,
	denials map[string]qaWSAct) (*harness.Report, *qaWSTable) {
	t.Helper()
	tab := qaWSNewTable(parts, denials)
	sc := &harness.Scenario{Name: "qa-world-seat", Participants: parts, Steps: steps}
	rep, err := harness.RunScenario(context.Background(), sc, tab.dial, nil, io.Discard)
	if err != nil {
		t.Fatalf("RunScenario framework error: %v", err)
	}
	if tab.overflow {
		t.Fatal("a fake connection's buffer overflowed")
	}
	if len(rep.Steps) != len(steps) {
		t.Fatalf("got %d step results for %d steps", len(rep.Steps), len(steps))
	}
	return rep, tab
}

func qaWSWantStrays(t *testing.T, tab *qaWSTable, want int) {
	t.Helper()
	if tab.strays != want {
		t.Fatalf("the fake delivered %d stray envelopes, want %d", tab.strays, want)
	}
}

func qaWSDump(rep *harness.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pass=%v\n", rep.Pass)
	for _, s := range rep.Steps {
		fmt.Fprintf(&b, "  step %d by=%s kind=%s pass=%v detail=%q\n",
			s.Index, s.By, s.Kind, s.Pass, s.Detail)
	}
	return b.String()
}

func qaWSWantFail(t *testing.T, rep *harness.Report, idx int, wants ...string) {
	t.Helper()
	if rep.Pass {
		t.Fatalf("report passed, want step %d to fail\n%s", idx, qaWSDump(rep))
	}
	s := rep.Steps[idx]
	if s.Pass {
		t.Fatalf("step %d passed, want it to fail\n%s", idx, qaWSDump(rep))
	}
	for _, w := range wants {
		if !strings.Contains(strings.ToLower(s.Detail), strings.ToLower(w)) {
			t.Errorf("step %d detail %q does not contain %q", idx, s.Detail, w)
		}
	}
}

func qaWSWantPass(t *testing.T, rep *harness.Report, idxs ...int) {
	t.Helper()
	for _, i := range idxs {
		if !rep.Steps[i].Pass {
			t.Errorf("step %d failed, want it to pass\n%s", i, qaWSDump(rep))
		}
	}
}

var qaWSTable4 = qaWSParts(
	"alpha", "dm", "bravo", "agent", "carol", "player", "dave", "spectator")

func qaWSOrderingSteps() []harness.Step {
	return []harness.Step{
		qaWSAccept("alpha", "a1"),
		qaWSAccept("alpha", "a2"),
		qaWSAccept("bravo", "a3"),
		qaWSRefuse("carol", "d1"),
		qaWSAccept("alpha", "a4"),
	}
}

func TestQAWSALeakToOneWorldSeatFailsTheDeniedStepAndNamesOnlyThatSeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rep, tab := qaWSRun(t, qaWSTable4, qaWSOrderingSteps(), map[string]qaWSAct{
			"d1": {consume: 1, drops: []qaWSDrop{{to: []string{"bravo"}, seq: 4}}},
		})
		qaWSWantStrays(t, tab, 1)
		qaWSWantFail(t, rep, 3,
			"unexpected broadcast observed by", "bravo", "after a denied command")
		if strings.Contains(rep.Steps[3].Detail, "alpha") {
			t.Errorf("detail %q names alpha, which received nothing", rep.Steps[3].Detail)
		}
		qaWSWantPass(t, rep, 0, 1, 2)
	})
}

func TestQAWSALeakToEveryWorldSeatNamesEachOfThem(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rep, tab := qaWSRun(t, qaWSTable4, qaWSOrderingSteps(), map[string]qaWSAct{
			"d1": {consume: 1, drops: []qaWSDrop{{to: []string{"alpha", "bravo"}, seq: 4}}},
		})
		qaWSWantStrays(t, tab, 2)
		qaWSWantFail(t, rep, 3,
			"unexpected broadcast observed by", "alpha", "bravo", "after a denied command")
	})
}

// VTT-359
func TestQAWSCharacterSeatEnvelopesBelowTheWitnessAreNotRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rep, tab := qaWSRun(t, qaWSTable4, qaWSOrderingSteps(), map[string]qaWSAct{
			"d1": {drops: []qaWSDrop{
				{to: []string{"carol"}, seq: 2},
				{to: []string{"dave"}, seq: 1},
			}},
		})
		qaWSWantStrays(t, tab, 2)
		qaWSWantPass(t, rep, 3)
		qaWSWantFail(t, rep, 4, "carol's stream breaks its log", "dave's stream breaks its log")
		if strings.Contains(rep.Steps[4].Detail, "unexpected broadcast") {
			t.Fatalf("a character seat's stray was read as a leak\n%s", qaWSDump(rep))
		}
	})
}

// VTT-380
func TestQAWSEveryRoleButPlayerAndSpectatorIsAWorldSeat(t *testing.T) {
	for _, role := range []string{"", "custom", "agent", "dm"} {
		t.Run(fmt.Sprintf("role=%q", role), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				parts := qaWSParts("alpha", "dm", "echo", role, "carol", "player")
				steps := []harness.Step{
					qaWSAccept("alpha", "a1"),
					qaWSAccept("alpha", "a2"),
					qaWSAccept("alpha", "a3"),
					qaWSRefuse("carol", "d1"),
					qaWSAccept("alpha", "a4"),
				}
				rep, tab := qaWSRun(t, parts, steps, map[string]qaWSAct{
					"d1": {consume: 1, drops: []qaWSDrop{{to: []string{"echo"}, seq: 4}}},
				})
				qaWSWantStrays(t, tab, 1)
				qaWSWantFail(t, rep, 3, "unexpected broadcast observed by", "echo")
			})
		})
	}
}

func TestQAWSOnlyTheEarliestOutstandingDeniedStepFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		steps := []harness.Step{
			qaWSAccept("alpha", "a1"),
			qaWSAccept("alpha", "a2"),
			qaWSRefuse("carol", "d1"),
			qaWSAccept("alpha", "a3"),
			qaWSRefuse("carol", "d2"),
			qaWSRefuse("carol", "d3"),
			qaWSAccept("bravo", "a4"),
		}
		rep, tab := qaWSRun(t, qaWSTable4, steps, map[string]qaWSAct{
			"d1": {},
			"d2": {},
			"d3": {consume: 1, drops: []qaWSDrop{{to: []string{"alpha"}, seq: 4}}},
		})
		qaWSWantStrays(t, tab, 1)
		qaWSWantFail(t, rep, 4, "unexpected broadcast observed by", "alpha")
		qaWSWantPass(t, rep, 2)
	})
}

func qaWSNoWorldSteps() []harness.Step {
	return []harness.Step{
		qaWSAccept("carol", "a1"),
		qaWSAccept("carol", "a2"),
		qaWSRefuse("carol", "d1"),
		qaWSAccept("carol", "a3"),
	}
}

// VTT-358
func TestQAWSDeniedStepWithNoWorldSeatIsUnprovable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parts := qaWSParts("carol", "player", "dave", "spectator")
		rep, _ := qaWSRun(t, parts, qaWSNoWorldSteps(), map[string]qaWSAct{"d1": {}})
		qaWSWantFail(t, rep, 2, "unprovable", "no world seat")
	})
}

// VTT-358
func TestQAWSTrailingDeniedStepWithNoWorldSeatIsUnprovable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parts := qaWSParts("carol", "player", "dave", "spectator")
		rep, _ := qaWSRun(t, parts, qaWSNoWorldSteps()[:3], map[string]qaWSAct{"d1": {}})
		qaWSWantFail(t, rep, 2, "unprovable", "no world seat")
	})
}

func TestQAWSTheSameScenarioWithAWorldSeatAddedPasses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parts := qaWSParts("carol", "player", "dave", "spectator", "alpha", "dm")
		rep, _ := qaWSRun(t, parts, qaWSNoWorldSteps(), map[string]qaWSAct{"d1": {}})
		if !rep.Pass {
			t.Fatalf("want pass\n%s", qaWSDump(rep))
		}
	})
}

func qaWSTrailingSteps() []harness.Step {
	return []harness.Step{
		qaWSAccept("alpha", "a1"),
		qaWSAccept("bravo", "a2"),
		qaWSRefuse("carol", "d1"),
		qaWSRefuse("dave", "d2"),
	}
}

func TestQAWSTrailingDeniedStepsPassInSilence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rep, _ := qaWSRun(t, qaWSTable4, qaWSTrailingSteps(), map[string]qaWSAct{
			"d1": {}, "d2": {},
		})
		if !rep.Pass {
			t.Fatalf("want pass\n%s", qaWSDump(rep))
		}
	})
}

func TestQAWSTrailingDeniedStepFailsWhenAnyParticipantHearsAnything(t *testing.T) {
	for _, who := range []string{"dave", "carol", "bravo"} {
		t.Run(who, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				rep, tab := qaWSRun(t, qaWSTable4, qaWSTrailingSteps()[:3], map[string]qaWSAct{
					"d1": {drops: []qaWSDrop{{to: []string{who}, seq: 3, when: qaWSAfterDelay}}},
				})
				qaWSWantStrays(t, tab, 1)
				qaWSWantFail(t, rep, 2)
			})
		})
	}
}

func TestQAWSTrailingDeniedStepFailsAsUnprovableWhenAStreamHasEnded(t *testing.T) {
	for _, who := range []string{"carol", "bravo"} {
		t.Run(who, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				rep, _ := qaWSRun(t, qaWSTable4, qaWSTrailingSteps()[:3], map[string]qaWSAct{
					"d1": {end: []string{who}},
				})
				qaWSWantFail(t, rep, 2, "unprovable")
			})
		})
	}
}

type qaWSSoak struct {
	qaWSHub
	ids     map[string]string
	state   *engine.State
	session string
	leakTo  []string
	armed   bool
	queued  []qaWSDrop
}

func qaWSNewSoak(leakTo ...string) *qaWSSoak {
	s := &qaWSSoak{ids: map[string]string{}, state: engine.NewState(), leakTo: leakTo}
	s.roles = map[string]string{}
	for _, p := range harness.SoakParticipants() {
		s.roles[p.Name] = p.Role
		s.ids[p.Name] = "qa-id-" + p.Name
	}
	s.armed = len(leakTo) > 0
	s.serve = s.serveAction
	return s
}

func (s *qaWSSoak) controls(name, tokenID string) bool {
	tok, ok := s.state.Tokens[tokenID]
	if !ok {
		return false
	}
	return slices.Contains(s.state.Actors[tok.ActorID].GetControllerIds(), s.ids[name])
}

func (s *qaWSSoak) eventFor(c *qaWSConn, cmd *vttv1.ClientCommand) (*vttv1.Envelope, string) {
	env := &vttv1.Envelope{ActorRole: s.roles[c.name], ParticipantId: s.ids[c.name]}
	player := s.roles[c.name] == "player"
	switch x := cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_MoveToken:
		m := x.MoveToken
		if player && !s.controls(c.name, m.GetTokenId()) {
			return nil, "qa: not authorized"
		}
		tok := s.state.Tokens[m.GetTokenId()]
		env.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
			TokenId: m.GetTokenId(), SceneId: tok.SceneID, Reason: m.GetReason(),
			From: &vttv1.GridPosition{X: tok.X, Y: tok.Y}, To: m.GetTo()}}
		return env, ""
	case nil:
		return nil, "qa: empty command"
	}
	if player {
		return nil, "qa: not authorized"
	}
	switch x := cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_LoadMap:
		id := x.LoadMap.GetMapId()
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: id, Name: id,
			GridWidth: harness.SoakMapGridSize, GridHeight: harness.SoakMapGridSize}}
	case *vttv1.ClientCommand_AddActor:
		env.Payload = &vttv1.Envelope_ActorAdded{
			ActorAdded: &vttv1.ActorAdded{Actor: x.AddActor.GetActor()}}
	case *vttv1.ClientCommand_GrantActorControl:
		g := x.GrantActorControl
		env.Payload = &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{
				ActorId: g.GetActorId(), ParticipantId: g.GetParticipantId(), Kind: g.GetKind()}}
	case *vttv1.ClientCommand_PlaceToken:
		p := x.PlaceToken
		env.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: p.GetTokenId(), SceneId: p.GetSceneId(),
			ActorId: p.GetActorId(), Position: p.GetPosition()}}
	case *vttv1.ClientCommand_StartSession:
		s.session = fmt.Sprintf("qa-session-%d", s.head+1)
		env.Payload = &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: x.StartSession.GetName()}}
	case *vttv1.ClientCommand_EndSession:
		env.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}
	default:
		return nil, "qa: unsupported command"
	}
	return env, ""
}

func (s *qaWSSoak) serveAction(c *qaWSConn, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	for _, d := range s.queued {
		s.sendLocked(d.to, d.seq)
	}
	s.queued = nil
	env, refusal := s.eventFor(c, cmd)
	if refusal != "" {
		if s.armed && refusal == "qa: not authorized" {
			s.armed = false
			s.head++
			seq := s.head
			s.queued = append(s.queued, qaWSDrop{to: s.leakTo, seq: seq})
		}
		return &vttv1.CommandResult{Error: refusal}
	}
	env.SessionId = s.session
	env.Sequence = s.head + 1
	env.EventId = fmt.Sprintf("qa-world-%d", env.Sequence)
	if err := engine.Apply(s.state, env); err != nil {
		return &vttv1.CommandResult{Error: "qa: " + err.Error()}
	}
	s.head++
	s.log = append(s.log, env)
	for _, lc := range s.liveLocked() {
		s.deliverLocked(lc, env)
	}
	return &vttv1.CommandResult{Ok: true, Sequence: env.Sequence}
}

func qaWSRunSoak(t *testing.T, s *qaWSSoak) (*harness.SoakReport, string) {
	t.Helper()
	var out strings.Builder
	cfg := harness.SoakConfig{Seed: 7, Events: 80, CheckEvery: 25, IDs: s.ids}
	rep, err := harness.RunSoak(context.Background(), cfg, s.dial, &out)
	if err != nil {
		t.Fatalf("RunSoak framework error: %v", err)
	}
	if s.overflow {
		t.Fatal("a fake connection's buffer overflowed")
	}
	if rep.Counts["deniedAttempt"] == 0 {
		t.Fatalf("the seed drew no denied attempt, so nothing was proved\n%s", out.String())
	}
	return rep, out.String()
}

func TestQAWSSoakPassesWhenNothingLeaks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rep, progress := qaWSRunSoak(t, qaWSNewSoak())
		if !rep.Pass {
			t.Fatalf("want pass\n%s", progress)
		}
	})
}

func qaWSFailLines(progress string) [][]string {
	var out [][]string
	for _, line := range strings.Split(progress, "\n") {
		if strings.Contains(line, "FAIL") {
			out = append(out, strings.FieldsFunc(line, func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsDigit(r)
			}))
		}
	}
	return out
}

func TestQAWSSoakFailsWhenOnlyOneWorldSeatHearsADenial(t *testing.T) {
	for seat, other := range map[string]string{"agent": "dm", "dm": "agent"} {
		t.Run(seat, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := qaWSNewSoak(seat)
				rep, progress := qaWSRunSoak(t, s)
				if s.strays != 1 {
					t.Fatalf("the leak reached %d seats, want 1", s.strays)
				}
				if rep.Pass {
					t.Fatalf("soak passed with a leak to %s\n%s", seat, progress)
				}
				lines := qaWSFailLines(progress)
				named := false
				for _, words := range lines {
					named = named || slices.Contains(words, seat)
					if slices.Contains(words, other) {
						t.Errorf("FAIL line %v names %s, which heard nothing", words, other)
					}
				}
				if !named {
					t.Errorf("no FAIL line names %s\n%s", seat, progress)
				}
			})
		})
	}
}

func TestQAWSSoakDoesNotReadCharacterSeats(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaWSNewSoak("player1", "player2")
		rep, progress := qaWSRunSoak(t, s)
		if s.strays != 2 {
			t.Fatalf("the leak reached %d seats, want 2", s.strays)
		}
		if !rep.Pass {
			t.Fatalf("want pass\n%s", progress)
		}
	})
}
