package harness_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

var (
	qaOLDM        = harness.Participant{Name: "dm", Role: "dm"}
	qaOLSpectator = harness.Participant{Name: "sp", Role: "spectator"}
	qaOLPlayer    = harness.Participant{Name: "pl", Role: "player"}
)

type qaOLReaction struct {
	by    string
	seq   int64
	sends map[string][]harness.Delivery
}

type qaOLDial struct {
	name  string
	after int64
	log   string
}

type qaOLTable struct {
	mu        sync.Mutex
	opens     map[string][]harness.Delivery
	redials   map[string][][]harness.Delivery
	reactions []qaOLReaction
	next      int
	dialed    map[string]int
	dials     []qaOLDial
	live      map[string]*qaOLConn
	all       []*qaOLConn
	faults    []string
}

type qaOLConn struct {
	table  *qaOLTable
	name   string
	events chan harness.Delivery
	mu     sync.Mutex
	closed bool
}

type qaOLCase struct {
	name    string
	parts   []harness.Participant
	opens   map[string][]harness.Delivery
	steps   []harness.Step
	reacts  []qaOLReaction
	redials map[string][][]harness.Delivery
	want    []bool
}

func qaOLCopy(d harness.Delivery) harness.Delivery {
	var out harness.Delivery
	if d.Envelope != nil {
		out.Envelope = proto.Clone(d.Envelope).(*vttv1.Envelope)
	}
	if d.Viewpoint != nil {
		out.Viewpoint = proto.Clone(d.Viewpoint).(*vttv1.ViewpointChanged)
	}
	return out
}

func (tb *qaOLTable) fault(format string, args ...any) {
	tb.faults = append(tb.faults, fmt.Sprintf(format, args...))
}

func (tb *qaOLTable) dial(name string, after int64, log string) (harness.Conn, error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	n := tb.dialed[name]
	tb.dialed[name] = n + 1
	tb.dials = append(tb.dials, qaOLDial{name: name, after: after, log: log})
	var sends []harness.Delivery
	switch {
	case n == 0:
		sends = tb.opens[name]
	case n-1 < len(tb.redials[name]):
		sends = tb.redials[name][n-1]
	default:
		tb.fault("unscripted redial %d of %s", n, name)
	}
	c := &qaOLConn{table: tb, name: name, events: make(chan harness.Delivery, 256)}
	c.push(sends)
	tb.live[name] = c
	tb.all = append(tb.all, c)
	return c, nil
}

func (tb *qaOLTable) redialLogs(name string) []string {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	var logs []string
	seen := false
	for _, d := range tb.dials {
		if d.name != name {
			continue
		}
		if seen {
			logs = append(logs, d.log)
		}
		seen = true
	}
	return logs
}

func (tb *qaOLTable) closeAll() {
	tb.mu.Lock()
	conns := slices.Clone(tb.all)
	tb.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func (c *qaOLConn) push(ds []harness.Delivery) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		c.table.fault("delivery to %s after its connection closed", c.name)
		return
	}
	for _, d := range ds {
		select {
		case c.events <- qaOLCopy(d):
		default:
			c.table.fault("delivery buffer of %s full", c.name)
		}
	}
}

func (c *qaOLConn) SendCommand(_ context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	tb := c.table
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if tb.next >= len(tb.reactions) {
		tb.fault("unscripted command from %s: %v", c.name, cmd)
		return nil, errors.New("qaOL: unscripted command")
	}
	r := tb.reactions[tb.next]
	tb.next++
	if r.by != c.name || tb.live[c.name] != c {
		tb.fault("command %d came from %s, scripted for %s", tb.next-1, c.name, r.by)
	}
	for name, ds := range r.sends {
		conn, ok := tb.live[name]
		if !ok {
			tb.fault("delivery to %s, which was never dialed", name)
			continue
		}
		conn.push(ds)
	}
	return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: r.seq}, nil
}

func (c *qaOLConn) Events() <-chan harness.Delivery { return c.events }

func (c *qaOLConn) CloseErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("qaOL: connection closed")
	}
	return nil
}

func (c *qaOLConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.events)
	}
	return nil
}

func qaOLView(actor string) harness.Delivery {
	return harness.Delivery{Viewpoint: &vttv1.ViewpointChanged{ActorId: actor}}
}

func qaOLEntry(seq int64, text string) harness.Delivery {
	return harness.Delivery{Envelope: &vttv1.Envelope{
		Sequence: seq,
		Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key:        "seen",
			Title:      "seen",
			Text:       text,
			Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		}},
	}}
}

func qaOLLog(log string, from, to int64) []harness.Delivery {
	var ds []harness.Delivery
	for s := from; s <= to; s++ {
		ds = append(ds, qaOLEntry(s, fmt.Sprintf("%s%d", log, s)))
	}
	return ds
}

func qaOLSwitch(actor string, from, to int64) []harness.Delivery {
	return append([]harness.Delivery{qaOLView(actor)}, qaOLLog(actor, from, to)...)
}

func qaOLWorld(seq int64) harness.Delivery {
	return harness.Delivery{Envelope: &vttv1.Envelope{
		EventId:  fmt.Sprintf("world-%d", seq),
		Sequence: seq,
		Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key:        fmt.Sprintf("w%d", seq),
			Title:      "world",
			Text:       fmt.Sprintf("world %d", seq),
			Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		}},
	}}
}

func qaOLStamped(seq int64, text, eventID, sessionID string) harness.Delivery {
	d := qaOLEntry(seq, text)
	d.Envelope.EventId = eventID
	d.Envelope.SessionId = sessionID
	return d
}

func qaOLJoin(parts ...[]harness.Delivery) []harness.Delivery {
	var out []harness.Delivery
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func qaOLCommand(by, command string) harness.Step {
	return harness.Step{By: by, Command: json.RawMessage(command), Expect: &harness.Expect{OK: true}}
}

func qaOLPerch(by, actor string) harness.Step {
	return qaOLCommand(by, fmt.Sprintf(`{"setViewpoint":{"actorId":%q}}`, actor))
}

func qaOLNarrate() harness.Step {
	return qaOLCommand("dm", `{"addNarration":{"text":"the door creaks"}}`)
}

func qaOLGrant(actor string) harness.Step {
	return qaOLCommand("dm", fmt.Sprintf(
		`{"grantActorControl":{"actorId":%q,"participantId":"{{id:pl}}","kind":"ACTOR_KIND_PARTY_MEMBER"}}`, actor))
}

func qaOLRevoke(actor string) harness.Step {
	return qaOLCommand("dm", fmt.Sprintf(
		`{"revokeActorControl":{"actorId":%q,"participantId":"{{id:pl}}"}}`, actor))
}

func qaOLReconnect(by string, after int64) harness.Step {
	return harness.Step{By: by, Reconnect: &harness.ReconnectSpec{AfterSequence: after}}
}

func qaOLQuiet(by string, sends map[string][]harness.Delivery) qaOLReaction {
	return qaOLReaction{by: by, sends: sends}
}

func qaOLAppend(seq int64, others map[string][]harness.Delivery) qaOLReaction {
	sends := map[string][]harness.Delivery{"dm": {qaOLWorld(seq)}}
	for name, ds := range others {
		sends[name] = ds
	}
	return qaOLReaction{by: "dm", seq: seq, sends: sends}
}

func qaOLOpens(parts []harness.Participant) map[string][]harness.Delivery {
	opens := map[string][]harness.Delivery{}
	for _, p := range parts {
		if p.Role == "player" || p.Role == "spectator" {
			opens[p.Name] = []harness.Delivery{qaOLView("")}
		}
	}
	return opens
}

func qaOLPlay(t *testing.T, c qaOLCase) (*harness.Report, *qaOLTable, string) {
	t.Helper()
	opens := c.opens
	if opens == nil {
		opens = qaOLOpens(c.parts)
	}
	tb := &qaOLTable{
		opens:     opens,
		redials:   c.redials,
		reactions: c.reacts,
		dialed:    map[string]int{},
		live:      map[string]*qaOLConn{},
	}
	ids := map[string]string{}
	for _, p := range c.parts {
		ids[p.Name] = "pid-" + p.Name
	}
	sc := &harness.Scenario{Name: c.name, Participants: c.parts, Steps: c.steps}
	var rep *harness.Report
	var out bytes.Buffer
	var runErr error
	synctest.Test(t, func(t *testing.T) {
		rep, runErr = harness.RunScenario(context.Background(), sc, tb.dial, ids, &out)
		tb.closeAll()
	})
	if runErr != nil {
		t.Fatalf("RunScenario: %v\n%s", runErr, out.String())
	}
	if len(tb.faults) > 0 {
		t.Fatalf("fake table faults: %v\n%s", tb.faults, out.String())
	}
	if tb.next != len(c.reacts) {
		t.Fatalf("%d of %d scripted commands were sent\n%s", tb.next, len(c.reacts), out.String())
	}
	return rep, tb, out.String()
}

func qaOLCheck(t *testing.T, c qaOLCase) {
	t.Helper()
	rep, _, out := qaOLPlay(t, c)
	if len(rep.Steps) != len(c.want) {
		t.Fatalf("%d step results, want %d\n%s", len(rep.Steps), len(c.want), out)
	}
	for i, want := range c.want {
		got := rep.Steps[i]
		t.Logf("step %d (%s by %s): pass = %v; detail: %s", i, got.Kind, got.By, got.Pass, got.Detail)
		if got.Pass != want {
			t.Errorf("step %d (%s by %s): pass = %v, want %v; detail: %s", i, got.Kind, got.By, got.Pass, want, got.Detail)
		}
	}
	if t.Failed() {
		t.Logf("report:\n%s", out)
	}
}

func qaOLRunCases(t *testing.T, cases []qaOLCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { qaOLCheck(t, c) })
	}
}

func qaOLWithDM(parts ...harness.Participant) []harness.Participant {
	return append([]harness.Participant{qaOLDM}, parts...)
}

// VTT-415 VTT-381
func TestQAOLAnAcceptedCommandNamingNoSequenceNeedsNoWorldEvent(t *testing.T) {
	perch := func(seq int64) qaOLReaction {
		return qaOLReaction{by: "sp", seq: seq, sends: map[string][]harness.Delivery{"sp": qaOLSwitch("A", 1, 2)}}
	}
	qaOLRunCases(t, []qaOLCase{
		{
			name:   "a spectator's switch naming no sequence passes beside a world seat that saw nothing",
			parts:  qaOLWithDM(qaOLSpectator),
			steps:  []harness.Step{qaOLPerch("sp", "A")},
			reacts: []qaOLReaction{perch(0)},
			want:   []bool{true},
		},
		{
			name:   "the same switch naming a sequence no world seat observes fails",
			parts:  qaOLWithDM(qaOLSpectator),
			steps:  []harness.Step{qaOLPerch("sp", "A")},
			reacts: []qaOLReaction{perch(3)},
			want:   []bool{false},
		},
		{
			name:   "a world seat's own command naming no sequence passes",
			parts:  qaOLWithDM(qaOLSpectator, harness.Participant{Name: "ag", Role: "agent"}),
			steps:  []harness.Step{qaOLCommand("dm", `{"rotateJoinLink":{}}`)},
			reacts: []qaOLReaction{qaOLQuiet("dm", nil)},
			want:   []bool{true},
		},
		{
			name:   "the same world seat's command naming a sequence nobody observes fails",
			parts:  qaOLWithDM(qaOLSpectator, harness.Participant{Name: "ag", Role: "agent"}),
			steps:  []harness.Step{qaOLCommand("dm", `{"rotateJoinLink":{}}`)},
			reacts: []qaOLReaction{{by: "dm", seq: 2}},
			want:   []bool{false},
		},
		{
			name:   "a switch naming no sequence passes in a scenario with no world seat",
			parts:  []harness.Participant{qaOLSpectator, qaOLPlayer},
			steps:  []harness.Step{qaOLPerch("sp", "A")},
			reacts: []qaOLReaction{perch(0)},
			want:   []bool{true},
		},
	})
}

// VTT-411
func TestQAOLAFirstEntryAfterASwitchToAnotherLogMustBeSequenceOne(t *testing.T) {
	sp := func(ds []harness.Delivery) map[string][]harness.Delivery {
		return map[string][]harness.Delivery{"sp": ds}
	}
	pl := func(ds []harness.Delivery) map[string][]harness.Delivery {
		return map[string][]harness.Delivery{"pl": ds}
	}
	qaOLRunCases(t, []qaOLCase{
		{
			name:   "from nobody onto a log whose first entry is 3 fails",
			parts:  qaOLWithDM(qaOLSpectator),
			steps:  []harness.Step{qaOLPerch("sp", "A")},
			reacts: []qaOLReaction{qaOLQuiet("sp", sp(qaOLSwitch("A", 3, 4)))},
			want:   []bool{false},
		},
		{
			name:   "from nobody onto a log from its first entry passes",
			parts:  qaOLWithDM(qaOLSpectator),
			steps:  []harness.Step{qaOLPerch("sp", "A")},
			reacts: []qaOLReaction{qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2)))},
			want:   []bool{true},
		},
		{
			name:  "a revocation moving a player to another log at entry 2 fails the DM's step",
			parts: qaOLWithDM(qaOLPlayer),
			steps: []harness.Step{qaOLGrant("A"), qaOLGrant("B"), qaOLRevoke("A")},
			reacts: []qaOLReaction{
				qaOLAppend(1, pl(qaOLSwitch("A", 1, 3))),
				qaOLAppend(2, nil),
				qaOLAppend(3, pl(qaOLSwitch("B", 2, 3))),
			},
			want: []bool{true, true, false},
		},
		{
			name:  "a revocation moving a player to another log from its first entry passes",
			parts: qaOLWithDM(qaOLPlayer),
			steps: []harness.Step{qaOLGrant("A"), qaOLGrant("B"), qaOLRevoke("A")},
			reacts: []qaOLReaction{
				qaOLAppend(1, pl(qaOLSwitch("A", 1, 3))),
				qaOLAppend(2, nil),
				qaOLAppend(3, pl(qaOLSwitch("B", 1, 3))),
			},
			want: []bool{true, true, true},
		},
		{
			name:  "back onto a log held before, continuing its old run, fails",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLPerch("sp", "A")},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3))),
				qaOLQuiet("sp", sp(qaOLSwitch("B", 1, 2))),
				qaOLQuiet("sp", sp(qaOLSwitch("A", 4, 4))),
			},
			want: []bool{true, true, false},
		},
		{
			name:  "back onto a log held before, from its first entry, passes",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLPerch("sp", "A")},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3))),
				qaOLQuiet("sp", sp(qaOLSwitch("B", 1, 2))),
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 4))),
			},
			want: []bool{true, true, true},
		},
		{
			name:  "a switch naming the log the seat holds keeps its run",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "A")},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3))),
				qaOLQuiet("sp", sp(qaOLSwitch("A", 4, 4))),
			},
			want: []bool{true, true},
		},
		{
			name:  "a first entry of 3 drained a step after its switch fails that later step",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLNarrate()},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2))),
				qaOLQuiet("sp", sp([]harness.Delivery{qaOLView("B")})),
				qaOLAppend(1, sp(qaOLLog("B", 3, 3))),
			},
			want: []bool{true, true, false},
		},
		{
			name:  "a first entry of 1 drained a step after its switch passes",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLNarrate()},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2))),
				qaOLQuiet("sp", sp([]harness.Delivery{qaOLView("B")})),
				qaOLAppend(1, sp(qaOLLog("B", 1, 1))),
			},
			want: []bool{true, true, true},
		},
		{
			name:  "the second of two switches in one drain, at entry 3, fails",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B")},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2))),
				qaOLQuiet("sp", sp(qaOLJoin(qaOLSwitch("B", 1, 2), qaOLSwitch("A", 3, 3)))),
			},
			want: []bool{true, false},
		},
		{
			name:  "two switches in one drain, each from its first entry, pass",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B")},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2))),
				qaOLQuiet("sp", sp(qaOLJoin(qaOLSwitch("B", 1, 2), qaOLSwitch("A", 1, 1)))),
			},
			want: []bool{true, true},
		},
	})
}

// VTT-412
func TestQAOLAReconnectFailsWhenTheRedialIsToldAnotherLog(t *testing.T) {
	sp := func(ds []harness.Delivery) map[string][]harness.Delivery {
		return map[string][]harness.Delivery{"sp": ds}
	}
	heldA := []qaOLReaction{qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3)))}
	heldBThenA := []qaOLReaction{
		qaOLQuiet("sp", sp(qaOLSwitch("B", 1, 2))),
		qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2))),
	}
	qaOLRunCases(t, []qaOLCase{
		{
			name:    "told another log followed by an entry equal to the one sent live fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldA,
			redials: map[string][][]harness.Delivery{"sp": {qaOLJoin([]harness.Delivery{qaOLView("B")}, qaOLLog("A", 3, 3))}},
			want:    []bool{true, false},
		},
		{
			name:    "told another log from its first entry fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldA,
			redials: map[string][][]harness.Delivery{"sp": {qaOLSwitch("B", 1, 2)}},
			want:    []bool{true, false},
		},
		{
			name:    "told the log it holds, followed by the entries after the cursor, passes",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldA,
			redials: map[string][][]harness.Delivery{"sp": {qaOLSwitch("A", 3, 3)}},
			want:    []bool{true, true},
		},
		{
			name:    "told the log it held before its last switch fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "B"), qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldBThenA,
			redials: map[string][][]harness.Delivery{"sp": {{qaOLView("B")}}},
			want:    []bool{true, true, false},
		},
		{
			name:    "told the log its last switch named passes",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "B"), qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldBThenA,
			redials: map[string][][]harness.Delivery{"sp": {{qaOLView("A")}}},
			want:    []bool{true, true, true},
		},
		{
			name:    "a first-step redial told another log than the opening named fails",
			parts:   qaOLWithDM(qaOLSpectator),
			opens:   map[string][]harness.Delivery{"sp": {qaOLView("A")}},
			steps:   []harness.Step{qaOLReconnect("sp", 0)},
			redials: map[string][][]harness.Delivery{"sp": {{qaOLView("B")}}},
			want:    []bool{false},
		},
		{
			name:    "a first-step redial told the log the opening named passes",
			parts:   qaOLWithDM(qaOLSpectator),
			opens:   map[string][]harness.Delivery{"sp": {qaOLView("A")}},
			steps:   []harness.Step{qaOLReconnect("sp", 0)},
			redials: map[string][][]harness.Delivery{"sp": {{qaOLView("A")}}},
			want:    []bool{true},
		},
		{
			name:    "a redial after a step told another log than the opening named fails",
			parts:   qaOLWithDM(qaOLSpectator),
			opens:   map[string][]harness.Delivery{"sp": {qaOLView("A")}},
			steps:   []harness.Step{qaOLNarrate(), qaOLReconnect("sp", 0)},
			reacts:  []qaOLReaction{qaOLAppend(1, nil)},
			redials: map[string][][]harness.Delivery{"sp": {{qaOLView("B")}}},
			want:    []bool{true, false},
		},
	})
}

// VTT-413
func TestQAOLAReconnectReplayMustEqualTheCurrentRunAfterTheCursor(t *testing.T) {
	sp := func(ds []harness.Delivery) map[string][]harness.Delivery {
		return map[string][]harness.Delivery{"sp": ds}
	}
	heldA := []qaOLReaction{qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 4)))}
	replayA := func(ds ...harness.Delivery) map[string][][]harness.Delivery {
		return map[string][][]harness.Delivery{"sp": {qaOLJoin([]harness.Delivery{qaOLView("A")}, ds)}}
	}
	switched := []qaOLReaction{
		qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3))),
		qaOLQuiet("sp", sp(qaOLSwitch("B", 1, 3))),
	}
	restarted := []qaOLReaction{
		qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3))),
		qaOLAppend(1, sp(qaOLLog("R", 1, 3))),
	}
	twice := func(second []harness.Delivery) qaOLCase {
		return qaOLCase{
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 3), qaOLNarrate(), qaOLReconnect("sp", 3)},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3))),
				qaOLAppend(1, sp(qaOLLog("A", 4, 5))),
			},
			redials: map[string][][]harness.Delivery{"sp": {{qaOLView("A")}, qaOLJoin([]harness.Delivery{qaOLView("A")}, second)}},
		}
	}
	afterTwiceOK := twice(qaOLLog("A", 4, 5))
	afterTwiceOK.name = "a second reconnect whose replay equals the entries sent live since the first passes"
	afterTwiceOK.want = []bool{true, true, true, true}
	afterTwiceBad := twice(qaOLJoin(qaOLLog("A", 4, 4), []harness.Delivery{qaOLEntry(5, "A5 elsewhere")}))
	afterTwiceBad.name = "a second reconnect whose replay differs in one payload fails"
	afterTwiceBad.want = []bool{true, true, true, false}
	qaOLRunCases(t, []qaOLCase{
		{
			name:    "a replay equal to the entries after the cursor passes",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldA,
			redials: replayA(qaOLLog("A", 3, 4)...),
			want:    []bool{true, true},
		},
		{
			name:    "a replay whose last entry differs only in its payload fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldA,
			redials: replayA(qaOLEntry(3, "A3"), qaOLEntry(4, "A4 elsewhere")),
			want:    []bool{true, false},
		},
		{
			name:    "a replay missing the last entry fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldA,
			redials: replayA(qaOLLog("A", 3, 3)...),
			want:    []bool{true, false},
		},
		{
			name:    "a replay carrying an entry never sent live fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldA,
			redials: replayA(qaOLLog("A", 3, 5)...),
			want:    []bool{true, false},
		},
		{
			name:    "after a switch, a replay equal to the new log's entries after the cursor passes",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLReconnect("sp", 1)},
			reacts:  switched,
			redials: map[string][][]harness.Delivery{"sp": {qaOLSwitch("B", 2, 3)}},
			want:    []bool{true, true, true},
		},
		{
			name:   "after a switch, a replay carrying the old log's entry at the same sequence fails",
			parts:  qaOLWithDM(qaOLSpectator),
			steps:  []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLReconnect("sp", 1)},
			reacts: switched,
			redials: map[string][][]harness.Delivery{"sp": {qaOLJoin(
				[]harness.Delivery{qaOLView("B")}, qaOLLog("A", 2, 2), qaOLLog("B", 3, 3))}},
			want: []bool{true, true, false},
		},
		{
			name:    "after a run restarted at entry 1, a replay equal to the new run after the cursor passes",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLNarrate(), qaOLReconnect("sp", 1)},
			reacts:  restarted,
			redials: replayA(qaOLLog("R", 2, 3)...),
			want:    []bool{true, true, true},
		},
		{
			name:    "after a run restarted at entry 1, a replay of the old run after the cursor fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLNarrate(), qaOLReconnect("sp", 1)},
			reacts:  restarted,
			redials: replayA(qaOLLog("A", 2, 3)...),
			want:    []bool{true, true, false},
		},
		afterTwiceOK,
		afterTwiceBad,
		{
			name:    "a replay crossing an earlier reconnect equal to the run after the cursor passes",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 1), qaOLReconnect("sp", 1)},
			reacts:  []qaOLReaction{qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3)))},
			redials: map[string][][]harness.Delivery{"sp": {qaOLSwitch("A", 2, 3), qaOLSwitch("A", 2, 3)}},
			want:    []bool{true, true, true},
		},
		{
			name:   "a replay crossing an earlier reconnect differing in one payload fails",
			parts:  qaOLWithDM(qaOLSpectator),
			steps:  []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 1), qaOLReconnect("sp", 1)},
			reacts: []qaOLReaction{qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3)))},
			redials: map[string][][]harness.Delivery{"sp": {
				qaOLSwitch("A", 2, 3),
				qaOLJoin(qaOLSwitch("A", 2, 2), []harness.Delivery{qaOLEntry(3, "A3 elsewhere")}),
			}},
			want: []bool{true, true, false},
		},
		{
			name:    "a replay whose entry differs only in its session id fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2)},
			reacts:  heldA,
			redials: replayA(qaOLEntry(3, "A3"), qaOLStamped(4, "A4", "", "session-1")),
			want:    []bool{true, false},
		},
		{
			name:  "after a switch, a replay of entries sharing event ids with the left log passes",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLReconnect("sp", 1)},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLJoin(qaOLSwitch("A", 1, 1),
					[]harness.Delivery{qaOLStamped(2, "A2", "world-7", "s"), qaOLStamped(3, "A3", "world-7", "s")}))),
				qaOLQuiet("sp", sp(qaOLJoin(qaOLSwitch("B", 1, 1),
					[]harness.Delivery{qaOLStamped(2, "B2", "world-7", "s"), qaOLStamped(3, "B3", "world-7", "s")}))),
			},
			redials: map[string][][]harness.Delivery{"sp": {{
				qaOLView("B"), qaOLStamped(2, "B2", "world-7", "s"), qaOLStamped(3, "B3", "world-7", "s"),
			}}},
			want: []bool{true, true, true},
		},
		{
			name:  "after a switch, a replay of the left log's entry sharing its event id and sequence fails",
			parts: qaOLWithDM(qaOLSpectator),
			steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLReconnect("sp", 1)},
			reacts: []qaOLReaction{
				qaOLQuiet("sp", sp(qaOLJoin(qaOLSwitch("A", 1, 1),
					[]harness.Delivery{qaOLStamped(2, "A2", "world-7", "s"), qaOLStamped(3, "A3", "world-7", "s")}))),
				qaOLQuiet("sp", sp(qaOLJoin(qaOLSwitch("B", 1, 1),
					[]harness.Delivery{qaOLStamped(2, "B2", "world-7", "s"), qaOLStamped(3, "B3", "world-7", "s")}))),
			},
			redials: map[string][][]harness.Delivery{"sp": {{
				qaOLView("B"), qaOLStamped(2, "B2", "world-7", "s"), qaOLStamped(3, "A3", "world-7", "s"),
			}}},
			want: []bool{true, true, false},
		},
	})
}

// VTT-413
func TestQAOLAReconnectReplayMustEqualTheWholeRunWhenNoActorsLogIsHeld(t *testing.T) {
	sp := func(ds []harness.Delivery) map[string][]harness.Delivery {
		return map[string][]harness.Delivery{"sp": ds}
	}
	leftA := []qaOLReaction{
		qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 3))),
		qaOLQuiet("sp", sp([]harness.Delivery{qaOLView("")})),
	}
	untold := map[string][]harness.Delivery{"pl": nil}
	toldNothing := []qaOLReaction{qaOLAppend(1, map[string][]harness.Delivery{"pl": qaOLLog("X", 1, 3)})}
	qaOLRunCases(t, []qaOLCase{
		{
			name:    "on nobody, a replay of no entry passes",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLReconnect("sp", 2)},
			redials: map[string][][]harness.Delivery{"sp": {{qaOLView("")}}},
			want:    []bool{true},
		},
		{
			name:    "on nobody, a replay carrying an entry fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLReconnect("sp", 2)},
			redials: map[string][][]harness.Delivery{"sp": {qaOLSwitch("", 1, 1)}},
			want:    []bool{false},
		},
		{
			name:    "moved to nobody, a replay of no entry passes",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", ""), qaOLReconnect("sp", 1)},
			reacts:  leftA,
			redials: map[string][][]harness.Delivery{"sp": {{qaOLView("")}}},
			want:    []bool{true, true, true},
		},
		{
			name:    "moved to nobody, a replay of the left log's entries after the cursor fails",
			parts:   qaOLWithDM(qaOLSpectator),
			steps:   []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", ""), qaOLReconnect("sp", 1)},
			reacts:  leftA,
			redials: map[string][][]harness.Delivery{"sp": {qaOLJoin([]harness.Delivery{qaOLView("")}, qaOLLog("A", 2, 3))}},
			want:    []bool{true, true, false},
		},
		{
			name:    "told no log, a replay of the whole run, cursor ignored, passes",
			parts:   qaOLWithDM(qaOLPlayer),
			opens:   untold,
			steps:   []harness.Step{qaOLNarrate(), qaOLReconnect("pl", 2)},
			reacts:  toldNothing,
			redials: map[string][][]harness.Delivery{"pl": {qaOLLog("X", 1, 3)}},
			want:    []bool{true, true},
		},
		{
			name:    "told no log, a replay of only the entries after the cursor fails",
			parts:   qaOLWithDM(qaOLPlayer),
			opens:   untold,
			steps:   []harness.Step{qaOLNarrate(), qaOLReconnect("pl", 2)},
			reacts:  toldNothing,
			redials: map[string][][]harness.Delivery{"pl": {qaOLLog("X", 3, 3)}},
			want:    []bool{true, false},
		},
	})
}

// VTT-414
func TestQAOLAReconnectNamesTheLogTheSeatHolds(t *testing.T) {
	sp := func(ds []harness.Delivery) map[string][]harness.Delivery {
		return map[string][]harness.Delivery{"sp": ds}
	}
	pl := func(ds []harness.Delivery) map[string][]harness.Delivery {
		return map[string][]harness.Delivery{"pl": ds}
	}
	cases := []struct {
		qaOLCase
		seat string
		logs []string
	}{
		{
			qaOLCase: qaOLCase{
				name:  "a spectator perching from nobody to A, B, A and nobody",
				parts: qaOLWithDM(qaOLSpectator),
				steps: []harness.Step{
					qaOLReconnect("sp", 0),
					qaOLPerch("sp", "A"), qaOLReconnect("sp", 2),
					qaOLPerch("sp", "B"), qaOLReconnect("sp", 1), qaOLReconnect("sp", 1),
					qaOLPerch("sp", "A"), qaOLReconnect("sp", 2),
					qaOLPerch("sp", ""), qaOLReconnect("sp", 0),
				},
				reacts: []qaOLReaction{
					qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2))),
					qaOLQuiet("sp", sp(qaOLSwitch("B", 1, 1))),
					qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2))),
					qaOLQuiet("sp", sp([]harness.Delivery{qaOLView("")})),
				},
				redials: map[string][][]harness.Delivery{"sp": {
					{qaOLView("")}, {qaOLView("A")}, {qaOLView("B")}, {qaOLView("B")}, {qaOLView("A")}, {qaOLView("")},
				}},
			},
			seat: "sp",
			logs: []string{"", "A", "B", "B", "A", ""},
		},
		{
			qaOLCase: qaOLCase{
				name:  "an accepted switch no ViewpointChanged follows leaves the held log named",
				parts: qaOLWithDM(qaOLSpectator),
				steps: []harness.Step{qaOLPerch("sp", "A"), qaOLPerch("sp", "B"), qaOLReconnect("sp", 1)},
				reacts: []qaOLReaction{
					qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 1))),
					qaOLQuiet("sp", nil),
				},
				redials: map[string][][]harness.Delivery{"sp": {{qaOLView("A")}}},
			},
			seat: "sp",
			logs: []string{"A"},
		},
		{
			qaOLCase: qaOLCase{
				name:  "a player moved by the DM's grant and revocation",
				parts: qaOLWithDM(qaOLPlayer),
				steps: []harness.Step{qaOLGrant("A"), qaOLReconnect("pl", 2), qaOLRevoke("A"), qaOLReconnect("pl", 0)},
				reacts: []qaOLReaction{
					qaOLAppend(1, pl(qaOLSwitch("A", 1, 2))),
					qaOLAppend(2, pl([]harness.Delivery{qaOLView("")})),
				},
				redials: map[string][][]harness.Delivery{"pl": {{qaOLView("A")}, {qaOLView("")}}},
			},
			seat: "pl",
			logs: []string{"A", ""},
		},
		{
			qaOLCase: qaOLCase{
				name:   "a redial told another log leaves the seat holding the log it was told",
				parts:  qaOLWithDM(qaOLSpectator),
				steps:  []harness.Step{qaOLPerch("sp", "A"), qaOLReconnect("sp", 2), qaOLReconnect("sp", 1)},
				reacts: []qaOLReaction{qaOLQuiet("sp", sp(qaOLSwitch("A", 1, 2)))},
				redials: map[string][][]harness.Delivery{"sp": {
					qaOLSwitch("B", 1, 1), {qaOLView("B")},
				}},
			},
			seat: "sp",
			logs: []string{"A", "B"},
		},
		{
			qaOLCase: qaOLCase{
				name:  "a redial from nobody told a log leaves the seat holding that log",
				parts: qaOLWithDM(qaOLSpectator),
				steps: []harness.Step{qaOLReconnect("sp", 0), qaOLReconnect("sp", 0)},
				redials: map[string][][]harness.Delivery{"sp": {
					{qaOLView("A")}, {qaOLView("A")},
				}},
			},
			seat: "sp",
			logs: []string{"", "A"},
		},
		{
			qaOLCase: qaOLCase{
				name:    "a reconnect as the first step names the log the opening's ViewpointChanged named",
				parts:   qaOLWithDM(qaOLSpectator),
				opens:   map[string][]harness.Delivery{"sp": {qaOLView("A")}},
				steps:   []harness.Step{qaOLReconnect("sp", 0)},
				redials: map[string][][]harness.Delivery{"sp": {{qaOLView("A")}}},
			},
			seat: "sp",
			logs: []string{"A"},
		},
		{
			qaOLCase: qaOLCase{
				name:    "a reconnect after a step names the log the opening's ViewpointChanged named",
				parts:   qaOLWithDM(qaOLSpectator),
				opens:   map[string][]harness.Delivery{"sp": {qaOLView("A")}},
				steps:   []harness.Step{qaOLNarrate(), qaOLReconnect("sp", 0)},
				reacts:  []qaOLReaction{qaOLAppend(1, nil)},
				redials: map[string][][]harness.Delivery{"sp": {{qaOLView("A")}}},
			},
			seat: "sp",
			logs: []string{"A"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep, tb, out := qaOLPlay(t, c.qaOLCase)
			if got := tb.redialLogs(c.seat); !slices.Equal(got, c.logs) {
				t.Errorf("redials of %s named logs %q, want %q", c.seat, got, c.logs)
			}
			for _, s := range rep.Steps {
				if !s.Pass {
					t.Logf("step %d (%s by %s) failed: %s", s.Index, s.Kind, s.By, s.Detail)
				}
			}
			if t.Failed() {
				t.Logf("report:\n%s", out)
			}
		})
	}
}
