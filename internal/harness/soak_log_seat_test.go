package harness_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"testing/synctest"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

type soakLogs struct {
	actor   map[string]string
	views   map[*fakeConn]*soakLogView
	dialsOf map[string]int
	named   []string

	freshRenames   bool
	noLogOther     bool
	freshOverflow  int
	freshToldOther bool
	liveSwitchAt   int64
	freshCorrupt   bool
	freshEndsAt    int
	liveCorruptAt  int64
}

type soakLogView struct {
	next   int64
	live   bool
	rename bool
}

func newSoakLogs() *soakLogs {
	return &soakLogs{
		actor:   map[string]string{"player1": "pc-player1", "player2": "pc-player2"},
		views:   map[*fakeConn]*soakLogView{},
		dialsOf: map[string]int{},
	}
}

func (l *soakLogs) dial(w *soakWorld, c *fakeConn, name string, after int64, log string) bool {
	actor, ok := l.actor[name]
	if !ok {
		return false
	}
	l.dialsOf[name]++
	fresh := l.dialsOf[name] > 1
	tuned := fresh && name == "player1"
	if tuned {
		l.named = append(l.named, log)
	}
	opened := actor
	if tuned && ((log == "" && l.noLogOther) || (log != "" && l.freshToldOther)) {
		opened = "pc-other"
	}
	push := func(d harness.Delivery) {
		select {
		case c.events <- d:
		default:
			panic("soakLogs: a dial's preload overflowed its buffer")
		}
	}
	push(harness.Delivery{Viewpoint: &vttv1.ViewpointChanged{ActorId: opened}})
	v := &soakLogView{live: !fresh, rename: tuned && l.freshRenames}
	l.views[c] = v
	if tuned && log == "" && l.noLogOther {
		return true
	}
	cursor := after
	if log == "" {
		cursor = 0
	}
	cut := 0
	if tuned && l.freshOverflow > 0 {
		cut, l.freshOverflow = l.freshOverflow, 0
	}
	sent := 0
	for _, env := range w.history {
		v.next++
		if v.next <= cursor {
			continue
		}
		if cut > 0 && sent >= cut {
			c.closeErr = harness.ErrEventsOverflow
			close(c.events)
			c.closed = true
			return true
		}
		if tuned && l.freshEndsAt > 0 && sent >= l.freshEndsAt {
			close(c.events)
			c.closed = true
			return true
		}
		e := l.entry(v, env)
		if tuned && l.freshCorrupt && sent == 0 {
			e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{TokenId: "no-such-token"}}
		}
		push(harness.Delivery{Envelope: e})
		sent++
	}
	return true
}

func (l *soakLogs) deliver(c *fakeConn, env *vttv1.Envelope) bool {
	v, ok := l.views[c]
	if !ok {
		return false
	}
	v.next++
	if v.live && c.name == "player1" && v.next == l.liveSwitchAt {
		c.trySwitch("pc-other")
	}
	e := l.entry(v, env)
	if v.live && c.name == "player1" && v.next == l.liveCorruptAt {
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{TokenId: "no-such-token"}}
	}
	c.trySend(e)
	return true
}

func (l *soakLogs) entry(v *soakLogView, env *vttv1.Envelope) *vttv1.Envelope {
	e := proto.Clone(env).(*vttv1.Envelope)
	e.Sequence, e.EventId = v.next, ""
	if s := e.GetSessionStarted(); s != nil && v.rename {
		s.Name = "renamed"
	}
	return e
}

func runLogSoak(t *testing.T, tune func(*soakLogs)) (*soakLogs, *harness.SoakReport, string) {
	t.Helper()
	var logs *soakLogs
	var rep *harness.SoakReport
	var out bytes.Buffer
	synctest.Test(t, func(t *testing.T) {
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		logs = newSoakLogs()
		tune(logs)
		w.logs = logs
		var err error
		rep, err = harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: 5, Events: 120, CheckEvery: 50, IDs: ids}, w.dial, &out)
		if err != nil {
			t.Fatalf("RunSoak: %v", err)
		}
	})
	return logs, rep, out.String()
}

// VTT-416
func TestASoakCheckpointFailsALogSeatWhoseFreshDialFoldsDifferently(t *testing.T) {
	if _, rep, out := runLogSoak(t, func(*soakLogs) {}); !rep.Pass {
		t.Fatalf("a soak whose players' fresh dials replay their logs failed:\n%s", out)
	}
	_, rep, out := runLogSoak(t, func(l *soakLogs) { l.freshRenames = true })
	if rep.Pass || !strings.Contains(out, "FAIL: player1's fold of pc-player1's log") {
		t.Fatalf("a fresh dial of player1 whose session differs in its name only did not fail:\n%s", out)
	}
	if strings.Contains(out, "FAIL: player2") {
		t.Fatalf("player2, whose fresh dials replay its log, was blamed:\n%s", out)
	}
}

// VTT-428
func TestASoakCheckpointDialsALogSeatNamingTheLogItHolds(t *testing.T) {
	logs, rep, out := runLogSoak(t, func(l *soakLogs) { l.noLogOther = true })
	if !rep.Pass {
		t.Fatalf("a soak whose dials naming no log open another log failed:\n%s", out)
	}
	for _, named := range logs.named {
		if named != "pc-player1" {
			t.Fatalf("player1's fresh dials named %q, want each to name pc-player1", logs.named)
		}
	}
	if len(logs.named) == 0 {
		t.Fatal("no checkpoint dialled player1")
	}
}

// VTT-416
func TestALogSeatsFreshDialThatOverflowsResumesTheSameLog(t *testing.T) {
	logs, rep, out := runLogSoak(t, func(l *soakLogs) { l.freshOverflow = 3 })
	if logs.freshOverflow != 0 {
		t.Fatal("fixture: no fresh dial of player1 overflowed")
	}
	if !rep.Pass {
		t.Fatalf("a fresh dial of player1 that overflowed and resumed failed:\n%s", out)
	}
}

// VTT-429
func TestASoakCheckpointFailsAFreshDialToldAnotherLog(t *testing.T) {
	_, rep, out := runLogSoak(t, func(l *soakLogs) { l.freshToldOther = true })
	if rep.Pass || !strings.Contains(out, "FAIL: player1's fresh dial was told pc-other, not pc-player1") {
		t.Fatalf("a fresh dial of player1 told another log did not fail:\n%s", out)
	}
}

// VTT-411
func TestASoakCheckpointFailsALogSeatThatKeepsItsOldLogAfterASwitch(t *testing.T) {
	_, rep, out := runLogSoak(t, func(l *soakLogs) { l.liveSwitchAt = 5 })
	const want = "FAIL: player1's stream breaks its log: sequence 5 straight after a switch to pc-other"
	if rep.Pass || strings.Count(out, want) != 1 {
		t.Fatalf("player1's stream, told pc-other and kept on its old log, was not failed once (%q):\n%s", want, out)
	}
	if strings.Count(out, "FAIL: player1") != 1 || strings.Contains(out, "FAIL: player2") {
		t.Fatalf("the break was blamed more than once, or on player2:\n%s", out)
	}
}

// VTT-431
func TestASoakCheckpointFailsAFreshDialThatDoesNotFold(t *testing.T) {
	_, rep, out := runLogSoak(t, func(l *soakLogs) { l.freshCorrupt = true })
	if rep.Pass || !strings.Contains(out, "FAIL: player1's fresh dial of pc-player1's log did not fold") {
		t.Fatalf("a fresh dial of player1 that does not fold did not fail:\n%s", out)
	}
}

// VTT-430
func TestASoakCheckpointFailsAFreshDialThatEndsShortOfItsHead(t *testing.T) {
	_, rep, out := runLogSoak(t, func(l *soakLogs) { l.freshEndsAt = 2 })
	if rep.Pass || !strings.Contains(out, "FAIL: player1's fresh dial never reached sequence") {
		t.Fatalf("a fresh dial of player1 that ends short of its head did not fail:\n%s", out)
	}
}

// VTT-431
func TestASoakCheckpointFailsALogSeatWhoseLiveRunDoesNotFold(t *testing.T) {
	_, rep, out := runLogSoak(t, func(l *soakLogs) { l.liveCorruptAt = 3 })
	if rep.Pass || !strings.Contains(out, "FAIL: player1's fold of pc-player1's log failed") {
		t.Fatalf("player1's live run, which does not fold, did not fail:\n%s", out)
	}
}
