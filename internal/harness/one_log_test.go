package harness_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

type oneLogSend struct {
	switchTo *string
	entry    *vttv1.Envelope
}

func oneLogSwitch(actorID string) oneLogSend { return oneLogSend{switchTo: &actorID} }

func oneLogEntry(seq int64, text string) oneLogSend {
	return oneLogSend{entry: &vttv1.Envelope{Sequence: seq,
		Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}}
}

func (o oneLogSend) to(c *fakeConn) {
	if o.switchTo != nil {
		c.trySwitch(*o.switchTo)
		return
	}
	c.trySend(o.entry)
}

func oneLogWorld(perStep ...[]oneLogSend) map[string]*fakeConn {
	world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}
	n := int64(0)
	world["dm"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		n++
		env := sceneCreatedEnv(fmt.Sprintf("ev-%d", n))
		if n == 1 {
			env = sessionStartedEnv("ev-1")
		} else {
			env.GetSceneCreated().SceneId = fmt.Sprintf("scn-%d", n)
		}
		env.Sequence = n
		world["dm"].trySend(env)
		for _, o := range perStep[n-1] {
			o.to(world["player"])
		}
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: n}, nil
	}
	return world
}

type oneLogDial struct {
	world  map[string]*fakeConn
	redial *fakeConn
	named  []string
}

func (d *oneLogDial) dial(name string, after int64, log string) (harness.Conn, error) {
	d.named = append(d.named, fmt.Sprintf("%s after=%d log=%q", name, after, log))
	if name == "player" && d.world["player"].isClosed() {
		return d.redial, nil
	}
	return d.world[name], nil
}

func oneLogReconnect(t *testing.T, after int64, perStep [][]oneLogSend, replay ...oneLogSend) (harness.StepResult, []string) {
	t.Helper()
	var got harness.StepResult
	var named []string
	synctest.Test(t, func(t *testing.T) {
		d := &oneLogDial{world: oneLogWorld(perStep...), redial: newFakeConn("player-redialled")}
		for _, o := range replay {
			o.to(d.redial)
		}
		sc := logSeatScenario(len(perStep), harness.Step{By: "player", Reconnect: &harness.ReconnectSpec{AfterSequence: after}})
		rep, err := harness.RunScenario(context.Background(), sc, d.dial, nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		for i, s := range rep.Steps[:len(perStep)] {
			if !s.Pass {
				t.Fatalf("step %d before the reconnect: %s, want pass", i, s.Detail)
			}
		}
		got, named = rep.Steps[len(perStep)], d.named
	})
	return got, named
}

var heldA = [][]oneLogSend{
	{oneLogSwitch("a"), oneLogEntry(1, "a1"), oneLogEntry(2, "a2")},
	{oneLogEntry(3, "a3")},
}

// VTT-411
func TestASeatThatAnnouncesASwitchAndKeepsTheOldLogFailsTheStep(t *testing.T) {
	run := func(t *testing.T, second []oneLogSend) []harness.StepResult {
		t.Helper()
		var steps []harness.StepResult
		synctest.Test(t, func(t *testing.T) {
			world := oneLogWorld([]oneLogSend{oneLogSwitch("a"), oneLogEntry(1, "a1"), oneLogEntry(2, "a2")}, second)
			rep, err := harness.RunScenario(context.Background(), logSeatScenario(2), fixedDialer(world), nil, io.Discard)
			if err != nil {
				t.Fatalf("RunScenario: %v", err)
			}
			steps = rep.Steps
		})
		return steps
	}
	if s := run(t, []oneLogSend{oneLogSwitch("b"), oneLogEntry(1, "b1")}); !s[0].Pass || !s[1].Pass {
		t.Fatalf("a switch to b followed by b's first entry: %+v, want both steps passed", s)
	}
	s := run(t, []oneLogSend{oneLogSwitch("b"), oneLogEntry(3, "a3")})
	if !s[0].Pass || s[1].Pass || !strings.Contains(s[1].Detail, "sequence 3 straight after a switch to b") {
		t.Fatalf("a switch to b followed by a's next entry: %+v, want the second step failed for it", s)
	}
}

// VTT-413
func TestAReconnectOfAHeldLogIsComparedAfterTheCursor(t *testing.T) {
	if s, _ := oneLogReconnect(t, 2, heldA, oneLogSwitch("a"), oneLogEntry(3, "a3")); !s.Pass {
		t.Errorf("a redial of a sending its entry after the cursor: %s, want pass", s.Detail)
	}
	if s, _ := oneLogReconnect(t, 2, heldA, oneLogSwitch("a"), oneLogEntry(1, "a1"), oneLogEntry(2, "a2"), oneLogEntry(3, "a3")); s.Pass {
		t.Error("a redial of a replaying it from its first entry passed")
	}
	retold := [][]oneLogSend{heldA[0], {oneLogSwitch("a"), oneLogEntry(3, "a3")}}
	if s, _ := oneLogReconnect(t, -1, retold, oneLogSwitch("a"), oneLogEntry(1, "a1"), oneLogEntry(2, "a2"), oneLogEntry(3, "a3")); !s.Pass {
		t.Errorf("a redial below the first entry of a log told twice, replaying all of it: %s, want pass", s.Detail)
	}
}

// VTT-412
func TestAReconnectToldAnotherLogFails(t *testing.T) {
	s, _ := oneLogReconnect(t, 2, heldA, oneLogSwitch("b"), oneLogEntry(3, "a3"))
	if s.Pass || !strings.Contains(s.Detail, "told b, not a") {
		t.Fatalf("a redial of a told b: %+v, want it failed for being told another log", s)
	}
}

// VTT-413
func TestAReconnectComparesWholeEntries(t *testing.T) {
	s, _ := oneLogReconnect(t, 2, heldA, oneLogSwitch("a"), oneLogEntry(3, "a3, reworded"))
	if s.Pass || !strings.Contains(s.Detail, "compared whole") {
		t.Fatalf("a redial whose entry differs only in its payload: %+v, want it failed", s)
	}
}

// VTT-413
func TestAReconnectOfASeatOnNobodyExpectsNothing(t *testing.T) {
	steps := [][]oneLogSend{heldA[0], {oneLogSwitch("")}}
	if s, _ := oneLogReconnect(t, 2, steps, oneLogSwitch("")); !s.Pass {
		t.Errorf("a redial of a seat on nobody sending nothing: %s, want pass", s.Detail)
	}
	if s, _ := oneLogReconnect(t, 2, steps, oneLogSwitch(""), oneLogEntry(1, "a1")); s.Pass {
		t.Error("a redial of a seat on nobody sending an entry passed")
	}
}

// VTT-414
func TestAReconnectNamesTheLogTheSeatHolds(t *testing.T) {
	_, named := oneLogReconnect(t, 2, heldA, oneLogSwitch("a"), oneLogEntry(3, "a3"))
	if want := `player after=2 log="a"`; named[len(named)-1] != want {
		t.Errorf("the redial of a seat holding a was %s, want %s", named[len(named)-1], want)
	}
	_, named = oneLogReconnect(t, 2, [][]oneLogSend{heldA[0], {oneLogSwitch("")}}, oneLogSwitch(""))
	if want := `player after=2 log=""`; named[len(named)-1] != want {
		t.Errorf("the redial of a seat on nobody was %s, want %s", named[len(named)-1], want)
	}
}

// VTT-415
func TestACommandThatAppendsNothingPasses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		world := oneLogWorld(heldA[0])
		world["player"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
			world["player"].trySwitch("b")
			world["player"].trySend(oneLogEntry(1, "b1").entry)
			return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true}, nil
		}
		sc := logSeatScenario(1, harness.Step{By: "player", Command: []byte(`{"setViewpoint":{"actorId":"b"}}`),
			Expect: &harness.Expect{OK: true}})
		start := time.Now()
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if s := rep.Steps[1]; !s.Pass || time.Since(start) >= time.Second {
			t.Fatalf("a set_viewpoint step: %+v after %s, want it passed well inside a second", s, time.Since(start))
		}
	})
}
