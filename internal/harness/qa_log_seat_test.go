package harness_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

var errQALSClosed = errors.New("qaLS: connection closed by the client")

type qaLSHandler interface {
	handle(name string, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error)
}

type qaLSConn struct {
	name     string
	srv      qaLSHandler
	mu       sync.Mutex
	ch       chan *vttv1.Envelope
	closed   bool
	closeErr error
}

func qaLSNewConn(name string, srv qaLSHandler) *qaLSConn {
	return &qaLSConn{name: name, srv: srv, ch: make(chan *vttv1.Envelope, 16384)}
}

func (c *qaLSConn) SendCommand(_ context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	if c.isClosed() {
		return nil, errQALSClosed
	}
	return c.srv.handle(c.name, cmd)
}

func (c *qaLSConn) Events() <-chan *vttv1.Envelope { return c.ch }

func (c *qaLSConn) CloseErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeErr
}

func (c *qaLSConn) Close() error {
	c.end(errQALSClosed)
	return nil
}

func (c *qaLSConn) end(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.closeErr = err
		close(c.ch)
	}
}

func (c *qaLSConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *qaLSConn) push(env *vttv1.Envelope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.ch <- proto.Clone(env).(*vttv1.Envelope):
	default:
		c.closed = true
		c.closeErr = harness.ErrEventsOverflow
		close(c.ch)
	}
}

func qaLSIsLogRole(role string) bool { return role == "player" || role == "spectator" }

func qaLSNote(key string) *vttv1.Envelope_NoteUpserted {
	return &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: key, Title: key, Text: "text of " + key, Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
	}}
}

func qaLSWorld(seq int64) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:  fmt.Sprintf("qals-world-%d", seq),
		Sequence: seq,
		Payload:  qaLSNote(fmt.Sprintf("world-%d", seq)),
	}
}

func qaLSEntry(log string, seq int64) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:  fmt.Sprintf("qals-%s-%d", log, seq),
		Sequence: seq,
		Payload:  qaLSNote(fmt.Sprintf("%s-entry-%d", log, seq)),
	}
}

func qaLSLog(log string, seqs ...int64) []*vttv1.Envelope {
	out := make([]*vttv1.Envelope, 0, len(seqs))
	for _, s := range seqs {
		out = append(out, qaLSEntry(log, s))
	}
	return out
}

type qaLSAct struct {
	deny  bool
	world []int64
	to    map[string][]*vttv1.Envelope
	skip  []string
	drop  []string
}

type qaLSScript struct {
	mu          sync.Mutex
	roles       map[string]string
	acts        []qaLSAct
	next        int
	conns       map[string]*qaLSConn
	all         []*qaLSConn
	dials       map[string]int
	replays     map[string][][]*vttv1.Envelope
	worldLog    []*vttv1.Envelope
	worldReplay map[string][][]*vttv1.Envelope
	dialAfter   map[string][]int64
	openAtDial  []string
}

func (s *qaLSScript) handle(name string, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.next >= len(s.acts) {
		return nil, fmt.Errorf("qaLS: script has no act for command %d by %s", s.next, name)
	}
	a := s.acts[s.next]
	s.next++
	for _, n := range a.drop {
		if c := s.conns[n]; c != nil {
			c.end(errors.New("qaLS: the server ended the connection"))
		}
	}
	if a.deny {
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "qaLS denied: nope"}, nil
	}
	envs := make([]*vttv1.Envelope, 0, len(a.world))
	for _, seq := range a.world {
		envs = append(envs, qaLSWorld(seq))
	}
	s.worldLog = append(s.worldLog, envs...)
	for n, c := range s.conns {
		if qaLSIsLogRole(s.roles[n]) || slices.Contains(a.skip, n) {
			continue
		}
		for _, e := range envs {
			c.push(e)
		}
	}
	for n, list := range a.to {
		if c := s.conns[n]; c != nil {
			for _, e := range list {
				c.push(e)
			}
		}
	}
	return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: a.world[0]}, nil
}

func (s *qaLSScript) dial(name string, after int64) (harness.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	role, ok := s.roles[name]
	if !ok {
		return nil, fmt.Errorf("qaLS: unknown participant %q", name)
	}
	n := s.dials[name]
	s.dials[name] = n + 1
	s.dialAfter[name] = append(s.dialAfter[name], after)
	c := qaLSNewConn(name, s)
	if n > 0 {
		switch {
		case qaLSIsLogRole(role):
			if n-1 >= len(s.replays[name]) {
				return nil, fmt.Errorf("qaLS: no replay scripted for redial %d of %s", n, name)
			}
			for _, e := range s.replays[name][n-1] {
				c.push(e)
			}
		case n-1 < len(s.worldReplay[name]):
			for _, e := range s.worldReplay[name][n-1] {
				c.push(e)
			}
		default:
			for _, e := range s.worldLog {
				if e.GetSequence() > after {
					c.push(e)
				}
			}
		}
	}
	if old := s.conns[name]; old != nil && !old.isClosed() {
		s.openAtDial = append(s.openAtDial, name)
	}
	s.conns[name] = c
	s.all = append(s.all, c)
	return c, nil
}

func (s *qaLSScript) closeAll() {
	s.mu.Lock()
	all := slices.Clone(s.all)
	s.mu.Unlock()
	for _, c := range all {
		c.end(errQALSClosed)
	}
}

type qaLSBuilder struct {
	parts       []harness.Participant
	steps       []harness.Step
	acts        []qaLSAct
	replays     map[string][][]*vttv1.Envelope
	worldReplay map[string][][]*vttv1.Envelope
	nextWorld   int64
	worldByStep int
	script      *qaLSScript
}

func qaLSScenario(parts ...harness.Participant) *qaLSBuilder {
	return &qaLSBuilder{
		parts:       parts,
		replays:     map[string][][]*vttv1.Envelope{},
		worldReplay: map[string][][]*vttv1.Envelope{},
		nextWorld:   1,
		worldByStep: 1,
	}
}

func qaLSP(name, role string) harness.Participant { return harness.Participant{Name: name, Role: role} }

func (b *qaLSBuilder) accept(by string, to map[string][]*vttv1.Envelope) *qaLSBuilder {
	seqs := make([]int64, 0, b.worldByStep)
	for range b.worldByStep {
		seqs = append(seqs, b.nextWorld)
		b.nextWorld++
	}
	return b.acceptAt(by, seqs, to)
}

func (b *qaLSBuilder) acceptAt(by string, seqs []int64, to map[string][]*vttv1.Envelope) *qaLSBuilder {
	b.steps = append(b.steps, harness.Step{
		By:      by,
		Command: json.RawMessage(`{"addNarration":{"text":"qaLS step"}}`),
		Expect:  &harness.Expect{OK: true},
	})
	b.acts = append(b.acts, qaLSAct{world: seqs, to: to})
	if last := seqs[len(seqs)-1]; last >= b.nextWorld {
		b.nextWorld = last + 1
	}
	return b
}

func (b *qaLSBuilder) deny(by string) *qaLSBuilder {
	b.steps = append(b.steps, harness.Step{
		By:      by,
		Command: json.RawMessage(`{"addNarration":{"text":"qaLS denied step"}}`),
		Expect:  &harness.Expect{DeniedContaining: "nope"},
	})
	b.acts = append(b.acts, qaLSAct{deny: true})
	return b
}

func (b *qaLSBuilder) reconnect(by string, after int64, replay []*vttv1.Envelope) *qaLSBuilder {
	b.steps = append(b.steps, harness.Step{By: by, Reconnect: &harness.ReconnectSpec{AfterSequence: after}})
	b.replays[by] = append(b.replays[by], replay)
	return b
}

func (b *qaLSBuilder) reconnectWorld(by string, after int64, replay []*vttv1.Envelope) *qaLSBuilder {
	b.steps = append(b.steps, harness.Step{By: by, Reconnect: &harness.ReconnectSpec{AfterSequence: after}})
	if replay != nil {
		b.worldReplay[by] = append(b.worldReplay[by], replay)
	}
	return b
}

func (b *qaLSBuilder) run(t *testing.T) *harness.Report {
	t.Helper()
	s := &qaLSScript{
		roles:       map[string]string{},
		acts:        b.acts,
		conns:       map[string]*qaLSConn{},
		dials:       map[string]int{},
		replays:     b.replays,
		worldReplay: b.worldReplay,
		dialAfter:   map[string][]int64{},
	}
	for _, p := range b.parts {
		s.roles[p.Name] = p.Role
	}
	b.script = s
	var rep *harness.Report
	var out bytes.Buffer
	synctest.Test(t, func(t *testing.T) {
		sc := &harness.Scenario{Name: "qaLS-" + t.Name(), Participants: b.parts, Steps: b.steps}
		r, err := harness.RunScenario(t.Context(), sc, s.dial, nil, &out)
		s.closeAll()
		if err != nil {
			t.Fatalf("RunScenario framework error: %v\nreport:\n%s", err, out.String())
		}
		rep = r
	})
	if rep == nil {
		t.FailNow()
	}
	if s.next != len(s.acts) {
		t.Fatalf("fixture: %d scripted commands, the harness issued %d", len(s.acts), s.next)
	}
	t.Logf("scenario report:\n%s", out.String())
	return rep
}

func qaLSWantSteps(t *testing.T, rep *harness.Report, want ...bool) {
	t.Helper()
	if len(rep.Steps) != len(want) {
		t.Fatalf("Report has %d steps, want %d: %+v", len(rep.Steps), len(want), rep.Steps)
	}
	all := true
	for i, w := range want {
		st := rep.Steps[i]
		if st.Pass != w {
			t.Errorf("step %d (%s by %s): Pass = %v, want %v; Detail %q", i, st.Kind, st.By, st.Pass, w, st.Detail)
		}
		if !st.Pass && st.Detail == "" {
			t.Errorf("step %d failed with an empty Detail", i)
		}
		if !st.Pass {
			t.Logf("step %d Detail: %s", i, st.Detail)
		}
		all = all && w
	}
	if rep.Pass != all {
		t.Errorf("Report.Pass = %v, want %v", rep.Pass, all)
	}
}

func qaLSTo(name string, envs ...*vttv1.Envelope) map[string][]*vttv1.Envelope {
	return map[string][]*vttv1.Envelope{name: envs}
}

func qaLSMustFold(t *testing.T, envs []*vttv1.Envelope) {
	t.Helper()
	if _, err := harness.Fold(envs); err != nil {
		t.Fatalf("fixture does not fold: %v", err)
	}
}

func TestQALSFixtureShapesFold(t *testing.T) {
	qaLSMustFold(t, []*vttv1.Envelope{qaLSWorld(1), qaLSWorld(2), qaLSWorld(5)})
	qaLSMustFold(t, append(qaLSLog("a", 1, 2, 2, 3), qaLSLog("b", 1, 1)...))
}

// VTT-359
func TestQALSRunThatContinuesAcrossStepsPasses(t *testing.T) {
	rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player")).
		accept("dm", nil).
		accept("dm", qaLSTo("p", qaLSLog("a", 1, 2, 3)...)).
		accept("dm", nil).
		accept("dm", qaLSTo("p", qaLSLog("a", 4)...)).
		accept("dm", qaLSTo("p", qaLSLog("a", 5, 6)...)).
		run(t)
	qaLSWantSteps(t, rep, true, true, true, true, true)
}

// VTT-359
func TestQALSRunRestartingAtOneIsASwitchAndPasses(t *testing.T) {
	for _, role := range []string{"player", "spectator"} {
		t.Run(role, func(t *testing.T) {
			rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", role)).
				accept("dm", qaLSTo("p", qaLSLog("a", 1, 2)...)).
				accept("dm", qaLSTo("p", qaLSLog("a", 3)...)).
				accept("dm", qaLSTo("p", qaLSLog("b", 1)...)).
				accept("dm", qaLSTo("p", qaLSLog("b", 2, 3)...)).
				accept("dm", qaLSTo("p", append(qaLSLog("c", 1), qaLSLog("d", 1, 2)...)...)).
				run(t)
			qaLSWantSteps(t, rep, true, true, true, true, true)
		})
	}
}

type qaLSBreakCase struct {
	name  string
	steps [][]*vttv1.Envelope
	want  []bool
}

func qaLSBreakCases() []qaLSBreakCase {
	a := func(seqs ...int64) []*vttv1.Envelope { return qaLSLog("a", seqs...) }
	b := func(seqs ...int64) []*vttv1.Envelope { return qaLSLog("b", seqs...) }
	return []qaLSBreakCase{
		{"skip", [][]*vttv1.Envelope{a(1), a(2), a(4), a(5)}, []bool{true, true, false, true}},
		{"repeat", [][]*vttv1.Envelope{a(1), a(2), a(2), a(3)}, []bool{true, true, false, true}},
		{"backwards", [][]*vttv1.Envelope{a(1, 2, 3), a(4), a(2), a(3)}, []bool{true, true, false, true}},
		{"opens at 2", [][]*vttv1.Envelope{a(2), a(3), a(4)}, []bool{false, true, true}},
		{"opens at 0", [][]*vttv1.Envelope{a(0), a(1), a(2)}, []bool{false, true, true}},
		{"opens at 2 after an empty step", [][]*vttv1.Envelope{nil, a(2), a(3)}, []bool{true, false, true}},
		{"skip inside one step's entries", [][]*vttv1.Envelope{a(1), a(2, 4, 5), a(6)}, []bool{true, false, true}},
		{"skip after an empty step", [][]*vttv1.Envelope{a(1), nil, a(3)}, []bool{true, true, false}},
		{"two breaks blamed separately", [][]*vttv1.Envelope{a(1), a(3), a(4), a(6), a(7)}, []bool{true, false, true, false, true}},
		{"switch then skip", [][]*vttv1.Envelope{a(1, 2), b(1), b(3)}, []bool{true, true, false}},
		{"continuing the old log after a switch", [][]*vttv1.Envelope{a(1, 2, 3), b(1), a(4)}, []bool{true, true, false}},
	}
}

// VTT-359
func TestQALSBrokenRunFailsTheStepThatDrainedIt(t *testing.T) {
	for _, role := range []string{"player", "spectator"} {
		for _, tc := range qaLSBreakCases() {
			t.Run(role+"/"+tc.name, func(t *testing.T) {
				bld := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", role))
				for _, envs := range tc.steps {
					var to map[string][]*vttv1.Envelope
					if envs != nil {
						to = qaLSTo("p", envs...)
					}
					bld.accept("dm", to)
				}
				qaLSWantSteps(t, bld.run(t), tc.want...)
			})
		}
	}
}

// VTT-359
func TestQALSBreakOnOneSeatDoesNotBlameStepsOfAnother(t *testing.T) {
	rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player"), qaLSP("s", "spectator")).
		accept("dm", map[string][]*vttv1.Envelope{"p": qaLSLog("a", 1), "s": qaLSLog("x", 1)}).
		accept("dm", map[string][]*vttv1.Envelope{"p": qaLSLog("a", 2), "s": qaLSLog("x", 3)}).
		accept("dm", map[string][]*vttv1.Envelope{"p": qaLSLog("a", 3), "s": qaLSLog("x", 4)}).
		accept("dm", map[string][]*vttv1.Envelope{"p": qaLSLog("a", 5), "s": qaLSLog("x", 5)}).
		run(t)
	qaLSWantSteps(t, rep, true, false, true, false)
}

// VTT-359
func TestQALSCharacterSeatSentNothingPasses(t *testing.T) {
	rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player"), qaLSP("s", "spectator")).
		accept("dm", nil).
		accept("dm", nil).
		accept("dm", nil).
		run(t)
	qaLSWantSteps(t, rep, true, true, true)
}

// VTT-359
func TestQALSCharacterSeatNeedNotObserveTheStepEvent(t *testing.T) {
	rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player"), qaLSP("s", "spectator")).
		accept("dm", map[string][]*vttv1.Envelope{"s": qaLSLog("x", 1, 2, 3)}).
		accept("dm", map[string][]*vttv1.Envelope{"p": qaLSLog("a", 1)}).
		accept("dm", map[string][]*vttv1.Envelope{"s": qaLSLog("x", 4)}).
		accept("dm", map[string][]*vttv1.Envelope{"p": qaLSLog("a", 2)}).
		run(t)
	qaLSWantSteps(t, rep, true, true, true, true)
}

// VTT-359 VTT-380
func TestQALSWorldSeatsAreNotHeldToTheLogRun(t *testing.T) {
	rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("ag", "agent"), qaLSP("e", ""), qaLSP("r", "referee"), qaLSP("p", "player")).
		acceptAt("dm", []int64{2}, qaLSTo("p", qaLSLog("a", 1)...)).
		acceptAt("dm", []int64{5}, qaLSTo("p", qaLSLog("a", 2)...)).
		acceptAt("dm", []int64{9}, qaLSTo("p", qaLSLog("a", 3)...)).
		run(t)
	qaLSWantSteps(t, rep, true, true, true)
}

// VTT-381
func TestQALSWorldSeatMissingTheStepEventFails(t *testing.T) {
	bld := qaLSScenario(qaLSP("dm", "dm"), qaLSP("ag", "agent"), qaLSP("p", "player")).
		accept("dm", nil).
		accept("dm", qaLSTo("p", qaLSLog("a", 1)...))
	bld.acts[1].skip = []string{"ag"}
	qaLSWantSteps(t, bld.run(t), true, false)
}

func qaLSSwitchedSeat(role string) *qaLSBuilder {
	return qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", role)).
		accept("dm", nil).
		accept("dm", qaLSTo("p", qaLSLog("a", 1)...)).
		accept("dm", qaLSTo("p", qaLSLog("a", 2, 3)...)).
		accept("dm", qaLSTo("p", qaLSLog("b", 1)...)).
		accept("dm", qaLSTo("p", qaLSLog("b", 2)...))
}

// VTT-360
func TestQALSReconnectAfterASwitchComparesWithTheCurrentLogOnly(t *testing.T) {
	for _, role := range []string{"player"} {
		for _, after := range []int64{1, 5, 9} {
			t.Run(fmt.Sprintf("%s/after=%d", role, after), func(t *testing.T) {
				rep := qaLSSwitchedSeat(role).
					reconnect("p", after, qaLSLog("b", 1, 2)).
					accept("dm", qaLSTo("p", qaLSLog("b", 3)...)).
					run(t)
				qaLSWantSteps(t, rep, true, true, true, true, true, true, true)
			})
		}
	}
}

// VTT-360
func TestQALSReconnectAfterASwitchInsideOneStep(t *testing.T) {
	rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player")).
		accept("dm", nil).
		accept("dm", qaLSTo("p", qaLSLog("a", 1, 2)...)).
		accept("dm", qaLSTo("p", append(qaLSLog("a", 3), qaLSLog("b", 1, 2)...)...)).
		reconnect("p", 2, qaLSLog("b", 1, 2)).
		run(t)
	qaLSWantSteps(t, rep, true, true, true, true)
}

func TestQALSReconnectClosesThenRedialsWithTheCursor(t *testing.T) {
	bld := qaLSSwitchedSeat("player").reconnect("p", 4, qaLSLog("b", 1, 2))
	qaLSWantSteps(t, bld.run(t), true, true, true, true, true, true)
	if got := bld.script.dialAfter["p"]; !slices.Equal(got, []int64{0, 4}) {
		t.Errorf("p was dialled with cursors %v, want [0 4]", got)
	}
	if len(bld.script.openAtDial) != 0 {
		t.Errorf("redialled while the old connection was still open: %v", bld.script.openAtDial)
	}
}

// VTT-360
func TestQALSReconnectReplayThatDiffersFails(t *testing.T) {
	b1, b2 := qaLSEntry("b", 1), qaLSEntry("b", 2)
	idSwapped1, idSwapped2 := proto.Clone(b1).(*vttv1.Envelope), proto.Clone(b2).(*vttv1.Envelope)
	idSwapped1.EventId, idSwapped2.EventId = b2.GetEventId(), b1.GetEventId()
	renamed := proto.Clone(b1).(*vttv1.Envelope)
	renamed.EventId = "qals-b-1-other"
	secondAtOne := proto.Clone(b2).(*vttv1.Envelope)
	secondAtOne.Sequence = 1
	cases := []struct {
		name   string
		replay []*vttv1.Envelope
	}{
		{"the log before the switch", qaLSLog("a", 1, 2, 3)},
		{"the whole live history", append(qaLSLog("a", 1, 2, 3), b1, b2)},
		{"missing the last entry", []*vttv1.Envelope{b1}},
		{"an entry never seen live", []*vttv1.Envelope{b1, b2, qaLSEntry("b", 3)}},
		{"nothing", nil},
		{"a different event id", []*vttv1.Envelope{renamed, b2}},
		{"event ids swapped", []*vttv1.Envelope{idSwapped1, idSwapped2}},
		{"a different sequence", []*vttv1.Envelope{b1, secondAtOne}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := qaLSSwitchedSeat("player").reconnect("p", 5, tc.replay).run(t)
			qaLSWantSteps(t, rep, true, true, true, true, true, false)
		})
	}
}

// VTT-360
func TestQALSReconnectComparesByEventIDAndSequenceOnly(t *testing.T) {
	b1, b2 := qaLSEntry("b", 1), qaLSEntry("b", 2)
	b1.Payload, b2.Payload = qaLSNote("b-reworded-1"), qaLSNote("b-reworded-2")
	rep := qaLSSwitchedSeat("player").reconnect("p", 5, []*vttv1.Envelope{b1, b2}).run(t)
	qaLSWantSteps(t, rep, true, true, true, true, true, true)
}

// VTT-360
func TestQALSReconnectWithoutASwitchIgnoresTheCursor(t *testing.T) {
	for _, after := range []int64{0, 2, 6} {
		t.Run(fmt.Sprintf("after=%d", after), func(t *testing.T) {
			rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player")).
				accept("dm", nil).
				accept("dm", qaLSTo("p", qaLSLog("a", 1, 2)...)).
				accept("dm", qaLSTo("p", qaLSLog("a", 3)...)).
				reconnect("p", after, qaLSLog("a", 1, 2, 3)).
				accept("dm", qaLSTo("p", qaLSLog("a", 4)...)).
				run(t)
			qaLSWantSteps(t, rep, true, true, true, true, true)
		})
	}
}

// VTT-360
func TestQALSReconnectTwiceAfterASwitch(t *testing.T) {
	rep := qaLSSwitchedSeat("player").
		reconnect("p", 5, qaLSLog("b", 1, 2)).
		accept("dm", qaLSTo("p", qaLSLog("b", 3)...)).
		reconnect("p", 3, qaLSLog("b", 1, 2, 3)).
		accept("dm", qaLSTo("p", qaLSLog("b", 4)...)).
		run(t)
	qaLSWantSteps(t, rep, true, true, true, true, true, true, true, true, true)
}

// VTT-360
func TestQALSReconnectOfASeatSentNothing(t *testing.T) {
	base := func() *qaLSBuilder {
		return qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player")).
			accept("dm", nil).
			accept("dm", nil)
	}
	t.Run("empty replay passes", func(t *testing.T) {
		qaLSWantSteps(t, base().reconnect("p", 3, nil).run(t), true, true, true)
	})
	t.Run("a replay of entries never seen live fails", func(t *testing.T) {
		qaLSWantSteps(t, base().reconnect("p", 3, qaLSLog("a", 1)).run(t), true, true, false)
	})
}

// VTT-382
func TestQALSWorldSeatReconnectComparesAfterTheCursor(t *testing.T) {
	base := func() *qaLSBuilder {
		return qaLSScenario(qaLSP("dm", "dm"), qaLSP("ag", "agent"), qaLSP("p", "player")).
			accept("dm", qaLSTo("p", qaLSLog("a", 1)...)).
			accept("dm", qaLSTo("p", qaLSLog("a", 2)...)).
			accept("dm", qaLSTo("p", qaLSLog("a", 3)...))
	}
	t.Run("replay after the cursor passes", func(t *testing.T) {
		rep := base().reconnectWorld("ag", 1, nil).accept("dm", qaLSTo("p", qaLSLog("a", 4)...)).run(t)
		qaLSWantSteps(t, rep, true, true, true, true, true)
	})
	t.Run("replay from the first entry fails", func(t *testing.T) {
		whole := []*vttv1.Envelope{qaLSWorld(1), qaLSWorld(2), qaLSWorld(3)}
		rep := base().reconnectWorld("ag", 1, whole).run(t)
		qaLSWantSteps(t, rep, true, true, true, false)
	})
}

// VTT-358 VTT-380
func TestQALSDeniedStepWithoutAWorldSeatIsUnprovable(t *testing.T) {
	t.Run("only character seats", func(t *testing.T) {
		rep := qaLSScenario(qaLSP("p", "player"), qaLSP("s", "spectator")).
			acceptAt("p", []int64{1}, qaLSTo("p", qaLSLog("a", 1)...)).
			deny("p").
			run(t)
		qaLSWantSteps(t, rep, true, false)
	})
	t.Run("a dm seat makes it provable", func(t *testing.T) {
		rep := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player")).deny("p").run(t)
		qaLSWantSteps(t, rep, true)
	})
	t.Run("the only world seat's connection has ended", func(t *testing.T) {
		bld := qaLSScenario(qaLSP("dm", "dm"), qaLSP("p", "player")).
			accept("dm", qaLSTo("p", qaLSLog("a", 1)...)).
			deny("p")
		bld.acts[1].drop = []string{"dm"}
		qaLSWantSteps(t, bld.run(t), true, false)
	})
	t.Run("an empty role is a world seat", func(t *testing.T) {
		rep := qaLSScenario(qaLSP("w", ""), qaLSP("p", "player")).deny("p").run(t)
		qaLSWantSteps(t, rep, true)
	})
}

type qaLSLines struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *qaLSLines) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *qaLSLines) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return bytes.Count(w.buf.Bytes(), []byte("\n"))
}

func (w *qaLSLines) lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := strings.TrimRight(w.buf.String(), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

type qaLSToken struct {
	actor, scene string
	x, y         int32
}

type qaLSMark struct {
	acceptedBefore int
	lines          int
}

type qaLSSoak struct {
	mu            sync.Mutex
	roles         map[string]string
	pids          map[string]string
	world         []*vttv1.Envelope
	logs          map[string][]*vttv1.Envelope
	silent        map[string]bool
	live          map[string]*qaLSConn
	conns         []*qaLSConn
	tokens        map[string]*qaLSToken
	actors        map[string]map[string]bool
	out           *qaLSLines
	breakName     string
	breakFrom     int
	shifted       bool
	breakLine     int
	breakAccepted int
	accepted      int
	marks         []qaLSMark
	denials       []string
	dials         []string
	session       string
	sessions      int
}

func qaLSNewSoak() *qaLSSoak {
	s := &qaLSSoak{
		roles: map[string]string{}, pids: map[string]string{}, logs: map[string][]*vttv1.Envelope{},
		silent: map[string]bool{}, live: map[string]*qaLSConn{}, tokens: map[string]*qaLSToken{},
		actors: map[string]map[string]bool{}, out: &qaLSLines{}, breakLine: -1,
	}
	for _, p := range harness.SoakParticipants() {
		s.roles[p.Name] = p.Role
		s.pids[p.Name] = "qals-pid-" + p.Name
	}
	return s
}

func (s *qaLSSoak) ids() map[string]string {
	out := make(map[string]string, len(s.pids))
	for k, v := range s.pids {
		out[k] = v
	}
	return out
}

func qaLSOutOfGrid(p *vttv1.GridPosition) bool {
	return p.GetX() < 0 || p.GetY() < 0 || p.GetX() >= harness.SoakMapGridSize || p.GetY() >= harness.SoakMapGridSize
}

func (s *qaLSSoak) plan(role, pid string, cmd *vttv1.ClientCommand) ([]*vttv1.Envelope, error) {
	if qaLSIsLogRole(role) {
		mv := cmd.GetMoveToken()
		if mv == nil {
			return nil, fmt.Errorf("not authorized: a %s may only move a token it controls (%T)", role, cmd.GetCommand())
		}
		tok := s.tokens[mv.GetTokenId()]
		if tok == nil {
			return nil, fmt.Errorf("unknown token %q", mv.GetTokenId())
		}
		if !s.actors[tok.actor][pid] {
			return nil, fmt.Errorf("not authorized: %s does not control %s", pid, tok.actor)
		}
		if qaLSOutOfGrid(mv.GetTo()) {
			return nil, fmt.Errorf("blocked: (%d,%d) is outside the grid", mv.GetTo().GetX(), mv.GetTo().GetY())
		}
	}
	one := func(e *vttv1.Envelope) ([]*vttv1.Envelope, error) { return []*vttv1.Envelope{e}, nil }
	switch c := cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_MoveToken:
		tok := s.tokens[c.MoveToken.GetTokenId()]
		if tok == nil {
			return nil, fmt.Errorf("unknown token %q", c.MoveToken.GetTokenId())
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
			TokenId: c.MoveToken.GetTokenId(), SceneId: tok.scene,
			From: &vttv1.GridPosition{X: tok.x, Y: tok.y}, To: c.MoveToken.GetTo(), Reason: c.MoveToken.GetReason(),
		}}})
	case *vttv1.ClientCommand_LoadMap:
		id := c.LoadMap.GetMapId()
		if !slices.Contains(harness.SoakMapIDs(), id) {
			return nil, fmt.Errorf("map %q is not installed", id)
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: id, Name: id, GridWidth: harness.SoakMapGridSize, GridHeight: harness.SoakMapGridSize,
		}}})
	case *vttv1.ClientCommand_AddActor:
		if c.AddActor.GetActor().GetKind() == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
			return nil, errors.New("add_actor: kind is required")
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: proto.Clone(c.AddActor.GetActor()).(*vttv1.Actor),
		}}})
	case *vttv1.ClientCommand_GrantActorControl:
		g := c.GrantActorControl
		if g.GetKind() == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
			return nil, errors.New("grant_actor_control: kind is required")
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: g.GetActorId(), ParticipantId: g.GetParticipantId(), Kind: g.GetKind(),
		}}})
	case *vttv1.ClientCommand_RevokeActorControl:
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
			ActorId: c.RevokeActorControl.GetActorId(), ParticipantId: c.RevokeActorControl.GetParticipantId(),
		}}})
	case *vttv1.ClientCommand_PlaceToken:
		p := c.PlaceToken
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: p.GetTokenId(), SceneId: p.GetSceneId(), ActorId: p.GetActorId(), Position: p.GetPosition(),
		}}})
	case *vttv1.ClientCommand_StartSession:
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: c.StartSession.GetName()}}})
	case *vttv1.ClientCommand_EndSession:
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}})
	case *vttv1.ClientCommand_RemoveToken:
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: c.RemoveToken.GetTokenId()}}})
	case *vttv1.ClientCommand_RemoveActor:
		id := c.RemoveActor.GetActorId()
		var toks []string
		for tid, tok := range s.tokens {
			if tok.actor == id {
				toks = append(toks, tid)
			}
		}
		sort.Strings(toks)
		var out []*vttv1.Envelope
		for _, tid := range toks {
			out = append(out, &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: tid}}})
		}
		out = append(out, &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: id}}})
		return out, nil
	case *vttv1.ClientCommand_AddNarration:
		n := c.AddNarration
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
			Text: n.GetText(), As: n.GetAs(), AnchorFromSeq: n.GetAnchorFromSeq(), AnchorToSeq: n.GetAnchorToSeq(),
		}}})
	default:
		return nil, fmt.Errorf("qaLS fake: unsupported command %T", c)
	}
}

func (s *qaLSSoak) track(e *vttv1.Envelope) {
	switch p := e.GetPayload().(type) {
	case *vttv1.Envelope_ActorAdded:
		set := map[string]bool{}
		for _, id := range p.ActorAdded.GetActor().GetControllerIds() {
			set[id] = true
		}
		s.actors[p.ActorAdded.GetActor().GetActorId()] = set
	case *vttv1.Envelope_ActorControlGranted:
		if s.actors[p.ActorControlGranted.GetActorId()] == nil {
			s.actors[p.ActorControlGranted.GetActorId()] = map[string]bool{}
		}
		s.actors[p.ActorControlGranted.GetActorId()][p.ActorControlGranted.GetParticipantId()] = true
	case *vttv1.Envelope_ActorControlRevoked:
		delete(s.actors[p.ActorControlRevoked.GetActorId()], p.ActorControlRevoked.GetParticipantId())
	case *vttv1.Envelope_ActorRemoved:
		delete(s.actors, p.ActorRemoved.GetActorId())
	case *vttv1.Envelope_TokenPlaced:
		tp := p.TokenPlaced
		s.tokens[tp.GetTokenId()] = &qaLSToken{actor: tp.GetActorId(), scene: tp.GetSceneId(), x: tp.GetPosition().GetX(), y: tp.GetPosition().GetY()}
	case *vttv1.Envelope_TokenMoved:
		if tok := s.tokens[p.TokenMoved.GetTokenId()]; tok != nil {
			tok.x, tok.y = p.TokenMoved.GetTo().GetX(), p.TokenMoved.GetTo().GetY()
		}
	case *vttv1.Envelope_TokenRemoved:
		delete(s.tokens, p.TokenRemoved.GetTokenId())
	}
}

func qaLSIsSession(e *vttv1.Envelope) bool {
	switch e.GetPayload().(type) {
	case *vttv1.Envelope_SessionStarted, *vttv1.Envelope_SessionEnded:
		return true
	}
	return false
}

func (s *qaLSSoak) handle(name string, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marks = append(s.marks, qaLSMark{acceptedBefore: s.accepted, lines: s.out.count()})
	role, pid := s.roles[name], s.pids[name]
	deny := func(err error) (*vttv1.CommandResult, error) {
		s.denials = append(s.denials, fmt.Sprintf("%s(%s): %v", name, role, err))
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: err.Error()}, nil
	}
	envs, err := s.plan(role, pid, cmd)
	if err != nil {
		return deny(err)
	}
	base := int64(len(s.world)) + 1
	session, sessions := s.session, s.sessions
	for i, e := range envs {
		e.EventId = fmt.Sprintf("qals-soak-world-%d", base+int64(i))
		e.Sequence = base + int64(i)
		e.ActorRole = role
		e.ParticipantId = pid
		if _, ok := e.GetPayload().(*vttv1.Envelope_SessionStarted); ok {
			sessions++
			session = fmt.Sprintf("qals-session-%d", sessions)
		}
		e.SessionId = session
		if _, ok := e.GetPayload().(*vttv1.Envelope_SessionEnded); ok {
			session = ""
		}
	}
	if _, err := harness.Fold(append(slices.Clone(s.world), envs...)); err != nil {
		return deny(fmt.Errorf("rejected: %w", err))
	}
	for _, e := range envs {
		s.track(e)
	}
	s.session, s.sessions = session, sessions
	s.world = append(s.world, envs...)
	s.accepted++
	for _, c := range s.conns {
		if !qaLSIsLogRole(s.roles[c.name]) {
			for _, e := range envs {
				c.push(e)
			}
		}
	}
	for _, p := range harness.SoakParticipants() {
		if !qaLSIsLogRole(p.Role) || s.silent[p.Name] {
			continue
		}
		for _, e := range envs {
			if qaLSIsSession(e) {
				continue
			}
			n := int64(len(s.logs[p.Name]) + 1)
			entry := proto.Clone(e).(*vttv1.Envelope)
			entry.EventId = fmt.Sprintf("qals-soak-%s-%d", p.Name, n)
			entry.Sequence = n
			entry.ActorRole, entry.ParticipantId = "", ""
			s.logs[p.Name] = append(s.logs[p.Name], entry)
			for _, c := range s.conns {
				if c.name != p.Name {
					continue
				}
				if c == s.live[p.Name] && p.Name == s.breakName && s.breakFrom > 0 && s.accepted >= s.breakFrom {
					if !s.shifted {
						s.shifted, s.breakLine, s.breakAccepted = true, s.out.count(), s.accepted
					}
					bad := proto.Clone(entry).(*vttv1.Envelope)
					bad.Sequence = n + 1
					c.push(bad)
					continue
				}
				c.push(entry)
			}
		}
	}
	return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: base}, nil
}

func (s *qaLSSoak) dial(name string, after int64) (harness.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	role, ok := s.roles[name]
	if !ok {
		return nil, fmt.Errorf("qaLS: unknown soak participant %q", name)
	}
	s.dials = append(s.dials, fmt.Sprintf("%s after=%d accepted=%d lines=%d", name, after, s.accepted, s.out.count()))
	c := qaLSNewConn(name, s)
	if qaLSIsLogRole(role) {
		if !s.silent[name] {
			for _, e := range s.logs[name] {
				c.push(e)
			}
		}
	} else {
		for _, e := range s.world {
			if e.GetSequence() > after {
				c.push(e)
			}
		}
	}
	if s.live[name] == nil {
		s.live[name] = c
	}
	s.conns = append(s.conns, c)
	return c, nil
}

func (s *qaLSSoak) closeAll() {
	s.mu.Lock()
	all := slices.Clone(s.conns)
	s.mu.Unlock()
	for _, c := range all {
		c.end(errQALSClosed)
	}
}

const (
	qaLSSoakSeed       = 11
	qaLSSoakEvents     = 160
	qaLSSoakCheckEvery = 25
)

func qaLSRunSoak(t *testing.T, s *qaLSSoak) *harness.SoakReport {
	t.Helper()
	cfg := harness.SoakConfig{Seed: qaLSSoakSeed, Events: qaLSSoakEvents, CheckEvery: qaLSSoakCheckEvery, IDs: s.ids()}
	var rep *harness.SoakReport
	synctest.Test(t, func(t *testing.T) {
		r, err := harness.RunSoak(t.Context(), cfg, s.dial, s.out)
		s.closeAll()
		if err != nil {
			t.Fatalf("RunSoak framework error: %v\ndenials: %v\nreport tail:\n%s", err, s.denials, qaLSTail(s.out.lines(), 30))
		}
		rep = r
	})
	if rep == nil {
		t.FailNow()
	}
	return rep
}

func qaLSTail(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func qaLSInserted(clean, broken []string) []int {
	n, m := len(clean), len(broken)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if clean[i] == broken[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []int
	i, j := 0, 0
	for j < m {
		switch {
		case i < n && clean[i] == broken[j]:
			i++
			j++
		case i < n && lcs[i+1][j] >= lcs[i][j+1]:
			i++
		default:
			out = append(out, j)
			j++
		}
	}
	return out
}

func qaLSAssertCleanSoak(t *testing.T, s *qaLSSoak, rep *harness.SoakReport) {
	t.Helper()
	if s.accepted <= qaLSSoakCheckEvery {
		t.Fatalf("fixture: only %d accepted commands, no periodic checkpoint can run", s.accepted)
	}
	if !rep.Pass {
		t.Errorf("SoakReport.Pass = false on a clean run; denials %v\nreport tail:\n%s", s.denials, qaLSTail(s.out.lines(), 40))
	}
}

// VTT-359
func TestQALSCleanSoakWithCharacterLogsPasses(t *testing.T) {
	s := qaLSNewSoak()
	rep := qaLSRunSoak(t, s)
	qaLSAssertCleanSoak(t, s, rep)
	for _, p := range []string{"player1", "player2"} {
		if len(s.logs[p]) == 0 || len(s.logs[p]) == len(s.world) {
			t.Errorf("fixture: %s's log has %d entries against a world of %d; it must be sent a log that is not the world", p, len(s.logs[p]), len(s.world))
		}
	}
	t.Logf("accepted %d, denied %d, world %d, player1 log %d, checkpoints %d, dials %v", s.accepted, len(s.denials), len(s.world), len(s.logs["player1"]), rep.Checkpoints, s.dials)
}

// VTT-359
func TestQALSCleanSoakWithSilentPlayersPasses(t *testing.T) {
	s := qaLSNewSoak()
	s.silent["player1"], s.silent["player2"] = true, true
	rep := qaLSRunSoak(t, s)
	qaLSAssertCleanSoak(t, s, rep)
}

var qaLSCheckpointNumbers = regexp.MustCompile(`checkpoint (\d+)\b`)

func qaLSSoakBreak(t *testing.T, breakFrom func(accepted int) int) {
	t.Helper()
	clean := qaLSNewSoak()
	cleanRep := qaLSRunSoak(t, clean)
	qaLSAssertCleanSoak(t, clean, cleanRep)
	from := breakFrom(clean.accepted)

	s := qaLSNewSoak()
	s.breakName, s.breakFrom = "player1", from
	rep := qaLSRunSoak(t, s)
	if !s.shifted {
		t.Fatalf("fixture: the break from accepted %d was never delivered (accepted %d)", from, s.accepted)
	}
	if rep.Pass {
		t.Errorf("SoakReport.Pass = true with player1's stream broken at accepted command %d", s.breakAccepted)
	}
	drainer := ((s.breakAccepted + qaLSSoakCheckEvery - 1) / qaLSSoakCheckEvery) * qaLSSoakCheckEvery
	drainerNumber := drainer / qaLSSoakCheckEvery
	if drainer > s.accepted {
		drainerNumber = s.accepted/qaLSSoakCheckEvery + 1
	}
	windowEnd := s.out.count()
	for _, mk := range s.marks {
		if mk.acceptedBefore >= drainer {
			windowEnd = mk.lines
			break
		}
	}
	lines := s.out.lines()
	inserted := qaLSInserted(clean.out.lines(), lines)
	var named []int
	for _, i := range inserted {
		t.Logf("line %d only in the broken run: %s", i, lines[i])
		if strings.Contains(lines[i], "player2") {
			t.Errorf("line %d blames player2, whose stream was clean: %s", i, lines[i])
		}
		if strings.Contains(lines[i], "player1") {
			named = append(named, i)
		}
	}
	t.Logf("break delivered at accepted %d (report line %d); draining checkpoint after accepted %d ends at line %d; dials %v",
		s.breakAccepted, s.breakLine, drainer, windowEnd, s.dials)
	if len(named) == 0 {
		t.Fatalf("no report line names player1")
	}
	inWindow := 0
	for _, i := range named {
		switch {
		case i < s.breakLine:
			t.Errorf("line %d names player1 before the break was delivered (line %d): %s", i, s.breakLine, lines[i])
		case i < windowEnd:
			inWindow++
			got := qaLSCheckpointNumbers.FindAllStringSubmatch(lines[i], -1)
			if len(got) != 1 || got[0][1] != strconv.Itoa(drainerNumber) {
				t.Errorf("line %d names checkpoints %v, want only checkpoint %d, the one that drained the break: %s", i, got, drainerNumber, lines[i])
			}
		default:
			t.Errorf("line %d names player1 after the draining checkpoint (ends at line %d): %s", i, windowEnd, lines[i])
		}
	}
	if inWindow == 0 {
		t.Errorf("no line naming player1 was written by the checkpoint that drained the break")
	}
	own := false
	for j := s.breakLine; j < windowEnd && j < len(lines); j++ {
		if slices.Contains(inserted, j) {
			continue
		}
		for _, m := range qaLSCheckpointNumbers.FindAllStringSubmatch(lines[j], -1) {
			own = own || m[1] == strconv.Itoa(drainerNumber)
		}
	}
	if !own {
		t.Errorf("fixture: the harness wrote no line of its own naming checkpoint %d between the break and the next action", drainerNumber)
	}
}

// VTT-359 VTT-383
func TestQALSSoakPlayerStreamBrokenMidRunFailsItsCheckpoint(t *testing.T) {
	qaLSSoakBreak(t, func(int) int { return qaLSSoakCheckEvery + 3 })
}

// VTT-359 VTT-383
func TestQALSSoakPlayerStreamBrokenInTheLastStretchFailsTheFinalCheckpoint(t *testing.T) {
	qaLSSoakBreak(t, func(accepted int) int {
		last := (accepted / qaLSSoakCheckEvery) * qaLSSoakCheckEvery
		if last == accepted {
			last -= qaLSSoakCheckEvery
		}
		return last + 1
	})
}
