package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

const (
	qaSDSAna    = "qasds-ana"
	qaSDSBo     = "qasds-bo"
	qaSDSCy     = "qasds-cy"
	qaSDSToken  = "tok-qasds"
	qaSDSPrompt = 10 * time.Second
)

type qaSDSStep struct {
	frame *vttv1.ServerFrame
	pause time.Duration
}

type qaSDSFake struct {
	srv     *httptest.Server
	mu      sync.Mutex
	queries []url.Values
	written int
	stop    chan struct{}
	wg      sync.WaitGroup
}

func qaSDSServe(t *testing.T, steps []qaSDSStep) *qaSDSFake {
	t.Helper()
	f := &qaSDSFake{stop: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		f.wg.Add(1)
		defer f.wg.Done()
		f.mu.Lock()
		f.queries = append(f.queries, r.URL.Query())
		f.mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
		defer cancel()
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		gone := make(chan struct{})
		go func() {
			defer close(gone)
			for {
				if _, _, err := c.Read(ctx); err != nil {
					return
				}
			}
		}()
		for _, s := range steps {
			if s.pause > 0 {
				select {
				case <-time.After(s.pause):
				case <-gone:
					return
				case <-f.stop:
					return
				case <-ctx.Done():
					return
				}
			}
			b, err := protojson.Marshal(s.frame)
			if err != nil {
				return
			}
			if err := c.Write(ctx, websocket.MessageText, b); err != nil {
				return
			}
			f.mu.Lock()
			f.written++
			f.mu.Unlock()
		}
		select {
		case <-gone:
		case <-f.stop:
		case <-ctx.Done():
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(func() {
		close(f.stop)
		f.srv.CloseClientConnections()
		waited := make(chan struct{})
		go func() { f.wg.Wait(); close(waited) }()
		select {
		case <-waited:
		case <-time.After(5 * time.Second):
			t.Errorf("fake gateway handler still running after the test")
		}
		f.srv.Close()
	})
	return f
}

func (f *qaSDSFake) url() string {
	return "ws" + strings.TrimPrefix(f.srv.URL, "http") + "/ws"
}

func (f *qaSDSFake) framesWritten() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.written
}

func (f *qaSDSFake) handshakes() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]url.Values(nil), f.queries...)
}

func qaSDSHead(n int64) qaSDSStep {
	return qaSDSStep{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_CatchUpHead{CatchUpHead: &vttv1.CatchUpHead{HeadSequence: n}}}}
}

func qaSDSPresence() qaSDSStep {
	return qaSDSStep{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_PresenceSnapshot{PresenceSnapshot: &vttv1.PresenceSnapshot{
		Present: []*vttv1.PresenceChanged{{ParticipantId: "qasds-p1", DisplayName: "Pia", State: vttv1.PresenceState_PRESENCE_STATE_CONNECTED}},
	}}}}
}

func qaSDSPresenceChanged(id string) qaSDSStep {
	return qaSDSStep{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_PresenceChanged{PresenceChanged: &vttv1.PresenceChanged{
		ParticipantId: id, DisplayName: "Someone", State: vttv1.PresenceState_PRESENCE_STATE_CONNECTED,
	}}}}
}

func qaSDSWatchable(ids ...string) qaSDSStep {
	names := map[string]string{qaSDSAna: "Ana", qaSDSBo: "Bo", qaSDSCy: "Cy"}
	w := &vttv1.WatchableChanged{}
	for _, id := range ids {
		w.Actors = append(w.Actors, &vttv1.WatchableActor{ActorId: id, Name: names[id]})
	}
	return qaSDSStep{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_WatchableChanged{WatchableChanged: w}}}
}

func qaSDSViewpoint(actorID string) qaSDSStep {
	return qaSDSStep{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_ViewpointChanged{ViewpointChanged: &vttv1.ViewpointChanged{ActorId: actorID}}}}
}

func qaSDSPaused(s qaSDSStep, d time.Duration) qaSDSStep {
	s.pause = d
	return s
}

func qaSDSNote(prefix string, seq int64) *vttv1.Envelope {
	return &vttv1.Envelope{
		Sequence: seq,
		Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key:        fmt.Sprintf("%s-key-%d", prefix, seq),
			Title:      fmt.Sprintf("%s-title-%d", prefix, seq),
			Text:       fmt.Sprintf("%s-text-%d", prefix, seq),
			Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		}},
	}
}

func qaSDSLog(actorID string, from, to int64) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for i := from; i <= to; i++ {
		out = append(out, qaSDSNote(actorID, i))
	}
	return out
}

func qaSDSEntries(es []*vttv1.Envelope) []qaSDSStep {
	var out []qaSDSStep
	for _, e := range es {
		out = append(out, qaSDSStep{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: e}}})
	}
	return out
}

func qaSDSSeq(parts ...[]qaSDSStep) []qaSDSStep {
	var out []qaSDSStep
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func qaSDSOne(s ...qaSDSStep) []qaSDSStep { return s }

func qaSDSPlayerOpening(head int64, actorID string) []qaSDSStep {
	return qaSDSOne(qaSDSHead(head), qaSDSPresence(), qaSDSWatchable(qaSDSAna, qaSDSBo, qaSDSCy), qaSDSViewpoint(actorID))
}

func qaSDSWant(t *testing.T, es []*vttv1.Envelope) map[string]any {
	t.Helper()
	st, err := harness.Fold(es)
	if err != nil {
		t.Fatalf("fixture does not fold: %v", err)
	}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal fold: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal fold: %v", err)
	}
	return m
}

type qaSDSRun struct {
	out     string
	err     error
	elapsed time.Duration
	fake    *qaSDSFake
}

func qaSDSDump(t *testing.T, steps []qaSDSStep) qaSDSRun {
	t.Helper()
	f := qaSDSServe(t, steps)
	start := time.Now()
	out, err := runCLI(t, "state", "dump", "--server", f.url(), "--token", qaSDSToken)
	return qaSDSRun{out: out, err: err, elapsed: time.Since(start), fake: f}
}

func qaSDSDecode(t *testing.T, out string) map[string]any {
	t.Helper()
	i := strings.Index(out, "{")
	if i < 0 {
		t.Fatalf("no JSON object in output:\n%s", out)
	}
	var m map[string]any
	if err := json.NewDecoder(strings.NewReader(out[i:])).Decode(&m); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	return m
}

func qaSDSAssertPrinted(t *testing.T, r qaSDSRun, wantEntries []*vttv1.Envelope, wantHead float64, wantLog *string) {
	t.Helper()
	if r.err != nil {
		t.Fatalf("dump refused: %v\n%s", r.err, r.out)
	}
	if r.elapsed >= qaSDSPrompt {
		t.Errorf("dump took %v, want under %v", r.elapsed, qaSDSPrompt)
	}
	got := qaSDSDecode(t, r.out)
	head, ok := got["headSequence"]
	if !ok {
		t.Errorf("output has no headSequence:\n%s", r.out)
	} else if head != wantHead {
		t.Errorf("headSequence = %v, want %v", head, wantHead)
	}
	logVal, hasLog := got["log"]
	switch {
	case wantLog == nil && hasLog:
		t.Errorf("world dump carries log = %v, want no log key", logVal)
	case wantLog != nil && !hasLog:
		t.Errorf("output has no log key, want %q:\n%s", *wantLog, r.out)
	case wantLog != nil && logVal != *wantLog:
		t.Errorf("log = %v, want %q", logVal, *wantLog)
	}
	delete(got, "headSequence")
	delete(got, "log")
	want := qaSDSWant(t, wantEntries)
	if !reflect.DeepEqual(got, want) {
		gb, _ := json.Marshal(got)
		wb, _ := json.Marshal(want)
		t.Errorf("printed fold differs from the fold of the held log\n got: %s\nwant: %s", gb, wb)
	}
}

func qaSDSAssertRefused(t *testing.T, r qaSDSRun, switchAt int, held, movedTo string) {
	t.Helper()
	if r.err == nil {
		t.Fatalf("dump printed state, want a refusal:\n%s", r.out)
	}
	if r.elapsed >= qaSDSPrompt {
		t.Errorf("refusal took %v, want under %v", r.elapsed, qaSDSPrompt)
	}
	if n := r.fake.framesWritten(); n < switchAt+1 {
		t.Errorf("fake wrote %d frames, the switch is frame %d: the refusal came before the switch", n, switchAt+1)
	}
	for _, marker := range []string{"headSequence", "-text-", "Notes"} {
		if strings.Contains(r.out, marker) {
			t.Errorf("refusal output carries state (%q):\n%s", marker, r.out)
		}
	}
	msg := r.err.Error() + "\n" + r.out
	if !strings.Contains(msg, held) {
		t.Errorf("refusal does not name the held log %q: %v\n%s", held, r.err, r.out)
	}
	if !strings.Contains(msg, movedTo) {
		t.Errorf("refusal does not name where the seat moved, %q: %v\n%s", movedTo, r.err, r.out)
	}
}

func qaSDSIndexOfSwitch(steps []qaSDSStep) int {
	seen := 0
	for i, s := range steps {
		if s.frame.GetViewpointChanged() != nil {
			seen++
			if seen == 2 {
				return i
			}
		}
	}
	return -1
}

func qaSDSStr(s string) *string { return &s }

// VTT-423 VTT-424
func TestQASDSSwitchBeforeHeadRefuses(t *testing.T) {
	cases := []struct {
		name  string
		steps []qaSDSStep
		twin  []qaSDSStep
		head  int64
	}{
		{
			name: "new log longer than the head",
			head: 3,
			steps: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 2)),
				qaSDSOne(qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 5))),
			twin: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))),
		},
		{
			name: "new log shorter than the head",
			head: 5,
			steps: qaSDSSeq(qaSDSPlayerOpening(5, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 2)),
				qaSDSOne(qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 2))),
			twin: qaSDSSeq(qaSDSPlayerOpening(5, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 5))),
		},
		{
			name: "switch before the first entry",
			head: 3,
			steps: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna),
				qaSDSOne(qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 4))),
			twin: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))),
		},
		{
			name: "switch away and back before the head",
			head: 3,
			steps: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 1)),
				qaSDSOne(qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 1)),
				qaSDSOne(qaSDSViewpoint(qaSDSAna)), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))),
			twin: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := harness.Fold(append(qaSDSLog(qaSDSAna, 1, 5), qaSDSLog(qaSDSBo, 1, 5)...)); err != nil {
				t.Fatalf("fixture: two logs must fold together so a mixed fold is printable: %v", err)
			}
			twin := qaSDSDump(t, tc.twin)
			qaSDSAssertPrinted(t, twin, qaSDSLog(qaSDSAna, 1, tc.head), float64(tc.head), qaSDSStr(qaSDSAna))
			if !strings.Contains(twin.out, "-text-") {
				t.Fatalf("twin printed no notes, so their absence below proves nothing:\n%s", twin.out)
			}
			r := qaSDSDump(t, tc.steps)
			qaSDSAssertRefused(t, r, qaSDSIndexOfSwitch(tc.steps), qaSDSAna, qaSDSBo)
		})
	}
}

// VTT-423 VTT-424
func TestQASDSSwitchToNobodyBeforeHeadRefuses(t *testing.T) {
	twin := qaSDSDump(t, qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))))
	qaSDSAssertPrinted(t, twin, qaSDSLog(qaSDSAna, 1, 3), 3, qaSDSStr(qaSDSAna))
	steps := qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 1)),
		qaSDSOne(qaSDSWatchable(qaSDSBo, qaSDSCy), qaSDSViewpoint("")))
	r := qaSDSDump(t, steps)
	qaSDSAssertRefused(t, r, qaSDSIndexOfSwitch(steps), qaSDSAna, "nobody")
}

// VTT-425 VTT-426 VTT-434
func TestQASDSSwitchAfterHeadPrintsHeldLog(t *testing.T) {
	cases := []struct {
		name     string
		steps    []qaSDSStep
		twin     []qaSDSStep
		want     []*vttv1.Envelope
		wantHead float64
	}{
		{
			name: "switch at the head",
			steps: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3)),
				qaSDSOne(qaSDSWatchable(qaSDSBo, qaSDSCy), qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 5))),
			twin:     qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))),
			want:     qaSDSLog(qaSDSAna, 1, 3),
			wantHead: 3,
		},
		{
			name: "switch after an entry past the head",
			steps: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 4)),
				qaSDSOne(qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 5))),
			twin:     qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 4))),
			want:     qaSDSLog(qaSDSAna, 1, 4),
			wantHead: 4,
		},
		{
			name: "switch to nobody after the head",
			steps: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3)),
				qaSDSOne(qaSDSViewpoint(""))),
			twin:     qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))),
			want:     qaSDSLog(qaSDSAna, 1, 3),
			wantHead: 3,
		},
		{
			name: "two switches after the head",
			steps: qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3)),
				qaSDSOne(qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 2)),
				qaSDSOne(qaSDSViewpoint(qaSDSCy)), qaSDSEntries(qaSDSLog(qaSDSCy, 1, 6))),
			twin:     qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))),
			want:     qaSDSLog(qaSDSAna, 1, 3),
			wantHead: 3,
		},
		{
			name: "held log empty at head zero",
			steps: qaSDSSeq(qaSDSPlayerOpening(0, qaSDSAna),
				qaSDSOne(qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 3))),
			twin:     qaSDSPlayerOpening(0, qaSDSAna),
			want:     nil,
			wantHead: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if reflect.DeepEqual(qaSDSWant(t, tc.want), qaSDSWant(t, append(append([]*vttv1.Envelope(nil), tc.want...), qaSDSLog(qaSDSBo, 1, 2)...))) {
				t.Fatalf("fixture: the held fold equals a fold that took the new log's entries")
			}
			twin := qaSDSDump(t, tc.twin)
			qaSDSAssertPrinted(t, twin, tc.want, tc.wantHead, qaSDSStr(qaSDSAna))
			r := qaSDSDump(t, tc.steps)
			if n := r.fake.framesWritten(); n < qaSDSIndexOfSwitch(tc.steps)+1 {
				t.Fatalf("fake wrote %d frames, never reaching the switch", n)
			}
			qaSDSAssertPrinted(t, r, tc.want, tc.wantHead, qaSDSStr(qaSDSAna))
		})
	}
}

// VTT-425
func TestQASDSSwitchAfterHeadStopsReadingAThrottledNewLog(t *testing.T) {
	twin := qaSDSDump(t, qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3))))
	qaSDSAssertPrinted(t, twin, qaSDSLog(qaSDSAna, 1, 3), 3, qaSDSStr(qaSDSAna))
	var trickle []qaSDSStep
	for _, s := range qaSDSEntries(qaSDSLog(qaSDSBo, 1, 100)) {
		trickle = append(trickle, qaSDSPaused(s, 100*time.Millisecond))
	}
	steps := qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 3)),
		qaSDSOne(qaSDSViewpoint(qaSDSBo)), trickle)
	r := qaSDSDump(t, steps)
	if r.elapsed >= 5*time.Second {
		t.Errorf("dump read on for %v after the switch; the new log trickles for 10s", r.elapsed)
	}
	qaSDSAssertPrinted(t, r, qaSDSLog(qaSDSAna, 1, 3), 3, qaSDSStr(qaSDSAna))
}

// VTT-425 VTT-426
func TestQASDSNobodyOpeningThenSwitchPrintsNobody(t *testing.T) {
	twin := qaSDSDump(t, qaSDSPlayerOpening(0, ""))
	qaSDSAssertPrinted(t, twin, nil, 0, qaSDSStr(""))
	steps := qaSDSSeq(qaSDSPlayerOpening(0, ""), qaSDSOne(qaSDSViewpoint(qaSDSBo)), qaSDSEntries(qaSDSLog(qaSDSBo, 1, 4)))
	r := qaSDSDump(t, steps)
	if n := r.fake.framesWritten(); n < 5 {
		t.Fatalf("fake wrote %d frames, never reaching the switch", n)
	}
	qaSDSAssertPrinted(t, r, nil, 0, qaSDSStr(""))
}

// VTT-426
func TestQASDSWorldDumpCarriesNoLog(t *testing.T) {
	world := qaSDSLog("qasds-world", 1, 3)
	r := qaSDSDump(t, qaSDSSeq(qaSDSOne(qaSDSHead(3), qaSDSPresence()), qaSDSEntries(world)))
	qaSDSAssertPrinted(t, r, world, 3, nil)
	got := qaSDSDecode(t, r.out)
	want := qaSDSWant(t, world)
	want["headSequence"] = float64(3)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("world dump is not the fold's fields plus headSequence:\n%s", r.out)
	}
	player := qaSDSDump(t, qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(world)))
	qaSDSAssertPrinted(t, player, world, 3, qaSDSStr(qaSDSAna))
	pgot := qaSDSDecode(t, player.out)
	want["log"] = qaSDSAna
	if !reflect.DeepEqual(pgot, want) {
		t.Errorf("player dump is not the world's JSON plus log:\n%s", player.out)
	}
}

// VTT-432
func TestQASDSDialNamesNoLog(t *testing.T) {
	for _, steps := range [][]qaSDSStep{
		qaSDSSeq(qaSDSOne(qaSDSHead(1), qaSDSPresence()), qaSDSEntries(qaSDSLog("qasds-world", 1, 1))),
		qaSDSSeq(qaSDSPlayerOpening(1, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 1))),
	} {
		r := qaSDSDump(t, steps)
		if r.err != nil {
			t.Fatalf("dump refused: %v\n%s", r.err, r.out)
		}
		hs := r.fake.handshakes()
		if len(hs) != 1 {
			t.Fatalf("handshakes = %d, want 1", len(hs))
		}
		q := hs[0]
		if q.Get("token") != qaSDSToken {
			t.Errorf("token = %q, want %q", q.Get("token"), qaSDSToken)
		}
		if !q.Has("after") || q.Get("after") != "0" {
			t.Errorf("after = %q (present %v), want 0", q.Get("after"), q.Has("after"))
		}
		if q.Has("log") {
			t.Errorf("dial names log %q, want none: a dump holds no log when it dials", q.Get("log"))
		}
	}
}

// VTT-433 VTT-434
func TestQASDSFramesTakenByKind(t *testing.T) {
	plain := qaSDSSeq(qaSDSPlayerOpening(3, qaSDSAna), qaSDSEntries(qaSDSLog(qaSDSAna, 1, 4)))
	twin := qaSDSDump(t, plain)
	qaSDSAssertPrinted(t, twin, qaSDSLog(qaSDSAna, 1, 4), 4, qaSDSStr(qaSDSAna))
	a := qaSDSEntries(qaSDSLog(qaSDSAna, 1, 4))
	mixed := qaSDSSeq(
		qaSDSOne(qaSDSHead(3), qaSDSPresence(), qaSDSPresenceChanged("qasds-p2"), qaSDSWatchable(qaSDSAna, qaSDSBo),
			qaSDSPresenceChanged("qasds-p3"), qaSDSViewpoint(qaSDSAna), qaSDSPresenceChanged("qasds-p4")),
		a[:1],
		qaSDSOne(qaSDSWatchable(qaSDSAna, qaSDSBo, qaSDSCy)),
		a[1:3],
		qaSDSOne(qaSDSPresenceChanged("qasds-p5"), qaSDSWatchable(qaSDSAna, qaSDSCy)),
		a[3:],
	)
	r := qaSDSDump(t, mixed)
	qaSDSAssertPrinted(t, r, qaSDSLog(qaSDSAna, 1, 4), 4, qaSDSStr(qaSDSAna))
}
