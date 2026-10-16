package store_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

var qaSubEventCounter atomic.Int64

func qaSubOpen(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "qa-sub.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func qaSubEnv(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{
		NarrationAdded: &vttv1.NarrationAdded{Text: text},
	}}
}

func qaSubWorld(text string) *vttv1.Envelope {
	e := qaSubEnv(text)
	e.EventId = fmt.Sprintf("qa-sub-evt-%d", qaSubEventCounter.Add(1))
	return e
}

type qaSubEnt struct {
	actor string
	cause int
	text  string
}

type qaSubBatch struct {
	worlds  []*vttv1.Envelope
	entries []store.Entry
}

func qaSubAppend(t *testing.T, s *store.Store, worldTexts []string, ents ...qaSubEnt) qaSubBatch {
	t.Helper()
	var b qaSubBatch
	for _, w := range worldTexts {
		b.worlds = append(b.worlds, qaSubWorld(w))
	}
	for _, e := range ents {
		b.entries = append(b.entries, store.Entry{ActorID: e.actor, Cause: e.cause, Env: qaSubEnv(e.text)})
	}
	if _, err := s.AppendBatch(b.worlds, b.entries); err != nil {
		t.Fatalf("append batch: %v", err)
	}
	return b
}

func qaSubNotifyAll(s *store.Store, b qaSubBatch) {
	for _, w := range b.worlds {
		s.Notify(w)
	}
	for _, e := range b.entries {
		s.NotifyCharacter(e.ActorID, e.Env)
	}
}

func qaSubKey(e *vttv1.Envelope) string {
	return fmt.Sprintf("%d:%s", e.GetSequence(), e.GetNarrationAdded().GetText())
}

func qaSubEntryKeys(actor string, from, to int) []string {
	var out []string
	for i := from; i <= to; i++ {
		out = append(out, fmt.Sprintf("%d:%s#%d", i, actor, i))
	}
	return out
}

func qaSubWorldKeys(from, to int) []string {
	var out []string
	for i := from; i <= to; i++ {
		out = append(out, fmt.Sprintf("%d:W%d", i, i))
	}
	return out
}

// qaSubDrain must run inside a synctest bubble.
func qaSubDrain(ch <-chan *vttv1.Envelope) (got []string, closed bool) {
	for {
		synctest.Wait()
		select {
		case e, ok := <-ch:
			if !ok {
				return got, true
			}
			got = append(got, qaSubKey(e))
		default:
			return got, false
		}
	}
}

func qaSubExpect(t *testing.T, name string, ch <-chan *vttv1.Envelope, want []string) {
	t.Helper()
	got, closed := qaSubDrain(ch)
	if closed {
		t.Errorf("%s: channel closed, want open", name)
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func qaSubSubscribe(t *testing.T, s *store.Store, actor string, cursor int64, buffer int, budget time.Duration) (<-chan *vttv1.Envelope, int64) {
	t.Helper()
	ch, unsub, head, err := s.SubscribeCharacter(actor, cursor, buffer, budget)
	if err != nil {
		t.Fatalf("subscribe %s after %d: %v", actor, cursor, err)
	}
	t.Cleanup(unsub)
	return ch, head
}

func qaSubIsPrefix(got, want []string) bool {
	return len(got) <= len(want) && slices.Equal(got, want[:len(got)])
}

// VTT-329, VTT-331.
func TestQASubDeliversItsLogFromItsOwnCursorThenLive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		qaSubAppend(t, s, []string{"W1", "W2", "W3", "W4"})
		qaSubAppend(t, s, []string{"W5", "W6"},
			qaSubEnt{"qa-a", 0, "qa-a#1"}, qaSubEnt{"qa-b", 0, "qa-b#1"},
			qaSubEnt{"qa-b", 1, "qa-b#2"}, qaSubEnt{"qa-a", 1, "qa-a#2"})
		qaSubAppend(t, s, []string{"W7"},
			qaSubEnt{"qa-a", -1, "qa-a#3"}, qaSubEnt{"qa-b", 0, "qa-b#3"}, qaSubEnt{"qa-b", 0, "qa-b#4"})
		qaSubAppend(t, s, []string{"W8", "W9"},
			qaSubEnt{"qa-a", 1, "qa-a#4"}, qaSubEnt{"qa-a", 0, "qa-a#5"},
			qaSubEnt{"qa-b", 0, "qa-b#5"}, qaSubEnt{"qa-b", 0, "qa-b#6"}, qaSubEnt{"qa-b", 1, "qa-b#7"})

		ch, head := qaSubSubscribe(t, s, "qa-a", 2, 2, 0)
		if head != 5 {
			t.Errorf("catch-up head = %d, want 5", head)
		}
		qaSubExpect(t, "history", ch, qaSubEntryKeys("qa-a", 3, 5))

		b := qaSubAppend(t, s, []string{"W10"}, qaSubEnt{"qa-b", 0, "qa-b#8"}, qaSubEnt{"qa-a", 0, "qa-a#6"})
		qaSubNotifyAll(s, b)
		qaSubExpect(t, "first live", ch, qaSubEntryKeys("qa-a", 6, 6))

		b = qaSubAppend(t, s, []string{"W11"}, qaSubEnt{"qa-a", 0, "qa-a#7"}, qaSubEnt{"qa-a", -1, "qa-a#8"})
		qaSubNotifyAll(s, b)
		qaSubExpect(t, "second live", ch, qaSubEntryKeys("qa-a", 7, 8))

		whole, wholeHead := qaSubSubscribe(t, s, "qa-a", 0, 1, 0)
		if wholeHead != 8 {
			t.Errorf("catch-up head from 0 = %d, want 8", wholeHead)
		}
		qaSubExpect(t, "whole log", whole, qaSubEntryKeys("qa-a", 1, 8))
		qaSubExpect(t, "first subscription afterwards", ch, nil)
	})
}

// VTT-329. Keep the cursor past the log's end and below the world's sequences.
func TestQASubCursorIsTheLogsOwnSequence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		qaSubAppend(t, s, []string{"W1", "W2", "W3", "W4", "W5", "W6", "W7", "W8", "W9", "W10"})
		var ents []qaSubEnt
		for i := 1; i <= 5; i++ {
			ents = append(ents, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", i)})
		}
		qaSubAppend(t, s, []string{"W11"}, ents...)

		ch, head := qaSubSubscribe(t, s, "qa-a", 7, 4, 0)
		if head != 7 {
			t.Errorf("catch-up head = %d, want the cursor 7", head)
		}
		qaSubExpect(t, "history", ch, nil)

		for i := 6; i <= 8; i++ {
			b := qaSubAppend(t, s, []string{fmt.Sprintf("W%d", 6+i)}, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", i)})
			qaSubNotifyAll(s, b)
		}
		qaSubExpect(t, "live", ch, qaSubEntryKeys("qa-a", 8, 8))
	})
}

// VTT-331.
func TestQASubCatchUpHeadIsTheLastQueuedOrTheCursor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		qaSubAppend(t, s, []string{"W1", "W2", "W3", "W4", "W5", "W6"})
		var ents []qaSubEnt
		for i := 1; i <= 10; i++ {
			ents = append(ents, qaSubEnt{"qa-b", 0, fmt.Sprintf("qa-b#%d", i)})
		}
		for i := 1; i <= 3; i++ {
			ents = append(ents, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", i)})
		}
		qaSubAppend(t, s, []string{"W7", "W8", "W9", "W10", "W11", "W12", "W13", "W14", "W15", "W16", "W17", "W18", "W19", "W20"}, ents...)

		cases := []struct {
			actor     string
			cursor    int64
			head      int64
			from, to  int
			delivered bool
		}{
			{"qa-a", 0, 3, 1, 3, true},
			{"qa-a", 1, 3, 2, 3, true},
			{"qa-a", 3, 3, 0, 0, false},
			{"qa-a", 9, 9, 0, 0, false},
			{"qa-b", 4, 10, 5, 10, true},
			{"qa-ghost", 0, 0, 0, 0, false},
			{"qa-ghost", 5, 5, 0, 0, false},
		}
		for _, c := range cases {
			ch, head := qaSubSubscribe(t, s, c.actor, c.cursor, 1, 0)
			name := fmt.Sprintf("%s after %d", c.actor, c.cursor)
			if head != c.head {
				t.Errorf("%s: catch-up head = %d, want %d", name, head, c.head)
			}
			var want []string
			if c.delivered {
				want = qaSubEntryKeys(c.actor, c.from, c.to)
			}
			qaSubExpect(t, name, ch, want)
		}
	})
}

// VTT-331.
func TestQASubNegativeCursorHeadIsTheCursorOrTheLastQueued(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		qaSubAppend(t, s, []string{"W1"}, qaSubEnt{"qa-a", 0, "qa-a#1"}, qaSubEnt{"qa-a", 0, "qa-a#2"})

		ch, head := qaSubSubscribe(t, s, "qa-ghost", -1, 1, 0)
		if head != -1 {
			t.Errorf("empty log after -1: catch-up head = %d, want -1", head)
		}
		qaSubExpect(t, "empty log after -1", ch, nil)

		chA, headA := qaSubSubscribe(t, s, "qa-a", -1, 1, 0)
		if headA != 2 {
			t.Errorf("log of 2 after -1: catch-up head = %d, want 2", headA)
		}
		qaSubExpect(t, "log of 2 after -1", chA, qaSubEntryKeys("qa-a", 1, 2))
	})
}

// VTT-329, VTT-331.
func TestQASubWaitsForALogWithNoEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		qaSubAppend(t, s, []string{"W1", "W2"}, qaSubEnt{"qa-b", 0, "qa-b#1"})

		ch, head := qaSubSubscribe(t, s, "qa-a", 0, 1, 0)
		if head != 0 {
			t.Errorf("catch-up head = %d, want the cursor 0", head)
		}
		qaSubExpect(t, "before its first entry", ch, nil)

		b := qaSubAppend(t, s, []string{"W3"}, qaSubEnt{"qa-b", 0, "qa-b#2"})
		qaSubNotifyAll(s, b)
		qaSubExpect(t, "after another log grew", ch, nil)

		b = qaSubAppend(t, s, []string{"W4", "W5"}, qaSubEnt{"qa-a", 1, "qa-a#1"}, qaSubEnt{"qa-a", 0, "qa-a#2"})
		qaSubNotifyAll(s, b)
		qaSubExpect(t, "first entries", ch, qaSubEntryKeys("qa-a", 1, 2))
	})
}

// VTT-330. Keep a leak from each log ahead of some receiver's last sequence,
// or a dedupe hides the leak.
func TestQASubCarriesNoOtherLogAndNoWorldEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		world, unsubW, worldHead, err := s.SubscribeWithNoProgressTimeout(0, 64, 0)
		if err != nil {
			t.Fatalf("world subscribe: %v", err)
		}
		t.Cleanup(unsubW)
		if worldHead != 0 {
			t.Fatalf("world head = %d, want 0", worldHead)
		}
		subA, _ := qaSubSubscribe(t, s, "qa-a", 0, 64, 0)
		subB, _ := qaSubSubscribe(t, s, "qa-b", 0, 64, 0)
		subLong, _ := qaSubSubscribe(t, s, "qa-a-long", 0, 64, 0)

		b1 := qaSubAppend(t, s, []string{"W1"},
			qaSubEnt{"qa-b", 0, "qa-b#1"}, qaSubEnt{"qa-b", 0, "qa-b#2"}, qaSubEnt{"qa-b", 0, "qa-b#3"})
		for _, e := range b1.entries {
			s.NotifyCharacter(e.ActorID, e.Env)
		}
		s.Notify(b1.worlds[0])
		qaSubExpect(t, "world after B ran ahead", world, qaSubWorldKeys(1, 1))
		qaSubExpect(t, "A after B ran ahead", subA, nil)
		qaSubExpect(t, "B", subB, qaSubEntryKeys("qa-b", 1, 3))
		qaSubExpect(t, "A-long after B ran ahead", subLong, nil)

		b2 := qaSubAppend(t, s, []string{"W2", "W3", "W4", "W5"}, qaSubEnt{"qa-a", 3, "qa-a#1"})
		for _, w := range b2.worlds {
			s.Notify(w)
		}
		s.NotifyCharacter("qa-a", b2.entries[0].Env)
		qaSubExpect(t, "world", world, qaSubWorldKeys(2, 5))
		qaSubExpect(t, "A after the world ran ahead", subA, qaSubEntryKeys("qa-a", 1, 1))
		qaSubExpect(t, "B after the world ran ahead", subB, nil)
		qaSubExpect(t, "A-long after the world ran ahead", subLong, nil)

		subA2, headA2 := qaSubSubscribe(t, s, "qa-a", 1, 64, 0)
		if headA2 != 1 {
			t.Errorf("second A head = %d, want 1", headA2)
		}

		b3 := qaSubAppend(t, s, []string{"W6"},
			qaSubEnt{"qa-a", 0, "qa-a#2"}, qaSubEnt{"qa-a", -1, "qa-a#3"}, qaSubEnt{"qa-b", 0, "qa-b#4"},
			qaSubEnt{"qa-a-long", 0, "qa-a-long#1"})
		s.NotifyCharacter("qa-a", b3.entries[0].Env)
		s.NotifyCharacter("qa-a-long", b3.entries[3].Env)
		s.Notify(b3.worlds[0])
		s.NotifyCharacter("qa-b", b3.entries[2].Env)
		s.NotifyCharacter("qa-a", b3.entries[1].Env)
		qaSubExpect(t, "A", subA, qaSubEntryKeys("qa-a", 2, 3))
		qaSubExpect(t, "second A", subA2, qaSubEntryKeys("qa-a", 2, 3))
		qaSubExpect(t, "B last", subB, qaSubEntryKeys("qa-b", 4, 4))
		qaSubExpect(t, "A-long", subLong, qaSubEntryKeys("qa-a-long", 1, 1))
		qaSubExpect(t, "world last", world, qaSubWorldKeys(6, 6))
	})
}

// VTT-329.
func TestQASubStaleAndRepeatedNotifiesArriveOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		var ents []qaSubEnt
		for i := 1; i <= 5; i++ {
			ents = append(ents, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", i)})
		}
		persisted := qaSubAppend(t, s, []string{"W1"}, ents...)

		ch, head := qaSubSubscribe(t, s, "qa-a", 2, 8, 0)
		if head != 5 {
			t.Errorf("catch-up head = %d, want 5", head)
		}
		for _, e := range persisted.entries {
			s.NotifyCharacter("qa-a", e.Env)
		}
		b := qaSubAppend(t, s, []string{"W2"}, qaSubEnt{"qa-a", 0, "qa-a#6"})
		s.NotifyCharacter("qa-a", b.entries[0].Env)
		s.NotifyCharacter("qa-a", b.entries[0].Env)
		qaSubExpect(t, "after stale and repeated notifies", ch, qaSubEntryKeys("qa-a", 3, 6))
	})
}

// VTT-329.
func TestQASubLongHistoryThroughAOneSlotBuffer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		const n = 200
		for batch := 0; batch < 10; batch++ {
			var ents []qaSubEnt
			for i := 1; i <= n/10; i++ {
				seq := batch*(n/10) + i
				ents = append(ents, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", seq)})
			}
			qaSubAppend(t, s, []string{fmt.Sprintf("W%d", batch+1)}, ents...)
		}
		ch, head := qaSubSubscribe(t, s, "qa-a", 0, 1, 0)
		if head != n {
			t.Errorf("catch-up head = %d, want %d", head, n)
		}
		b := qaSubAppend(t, s, []string{"W11"}, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", n+1)})
		qaSubNotifyAll(s, b)
		qaSubExpect(t, "history then live", ch, qaSubEntryKeys("qa-a", 1, n+1))
	})
}

// VTT-329, VTT-331.
func TestQASubRacingAppendsDeliverEachEntryOnce(t *testing.T) {
	for round := 0; round < 4; round++ {
		qaSubRaceRound(t, round)
	}
}

type qaSubRacer struct {
	startAt   int
	cursor    int64
	fromHead  bool
	buffer    int
	readDelay bool
}

func qaSubRaceRound(t *testing.T, round int) {
	t.Helper()
	const n = 120
	const actor, other = "qa-a", "qa-b"
	s := qaSubOpen(t)
	racers := []qaSubRacer{
		{startAt: 0, cursor: 0, buffer: 1},
		{startAt: 0, fromHead: true, buffer: 2},
		{startAt: 1, cursor: 50, buffer: 1},
		{startAt: 10, cursor: 5, buffer: 1},
		{startAt: 30, fromHead: true, buffer: 1},
		{startAt: 31, fromHead: true, buffer: 4},
		{startAt: 60, cursor: 0, buffer: 1, readDelay: true},
		{startAt: 90, fromHead: true, buffer: 1},
		{startAt: 119, cursor: 100, buffer: 2},
	}
	var wg sync.WaitGroup
	runRacer := func(idx int, r qaSubRacer) {
		defer wg.Done()
		before, err := s.ReadCharacterAfter(actor, 0)
		if err != nil {
			t.Errorf("round %d racer %d: read: %v", round, idx, err)
			return
		}
		cursor := r.cursor
		if r.fromHead {
			cursor = int64(len(before))
		}
		ch, unsub, head, err := s.SubscribeCharacter(actor, cursor, r.buffer, 0)
		if err != nil {
			t.Errorf("round %d racer %d: subscribe: %v", round, idx, err)
			return
		}
		defer unsub()
		after, err := s.ReadCharacterAfter(actor, 0)
		if err != nil {
			t.Errorf("round %d racer %d: read: %v", round, idx, err)
			return
		}
		lo := max(cursor, int64(len(before)))
		hi := max(cursor, int64(len(after)))
		if head < lo || head > hi {
			t.Errorf("round %d racer %d: cursor %d, catch-up head %d, want within [%d, %d]", round, idx, cursor, head, lo, hi)
		}
		want := qaSubEntryKeys(actor, int(cursor)+1, n)
		var got []string
		deadline := time.After(30 * time.Second)
		for len(got) < len(want) {
			select {
			case e, ok := <-ch:
				if !ok {
					t.Errorf("round %d racer %d: channel closed after %d entries", round, idx, len(got))
					return
				}
				got = append(got, qaSubKey(e))
				if r.readDelay && len(got)%7 == 0 {
					time.Sleep(time.Millisecond)
				}
			case <-deadline:
				t.Errorf("round %d racer %d: timed out after %v", round, idx, got)
				return
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("round %d racer %d: cursor %d: got %v, want %v", round, idx, cursor, got, want)
		}
		select {
		case e, ok := <-ch:
			if ok {
				t.Errorf("round %d racer %d: extra entry %s", round, idx, qaSubKey(e))
			}
		case <-time.After(20 * time.Millisecond):
		}
	}

	started := make(map[int]bool)
	launch := func(progress int) {
		for idx, r := range racers {
			if r.startAt == progress && !started[idx] {
				started[idx] = true
				wg.Add(1)
				go runRacer(idx, r)
			}
		}
	}
	progress := make(chan int, n)
	appendDone := make(chan struct{})
	go func() {
		defer close(appendDone)
		defer close(progress)
		for i := 1; i <= n; i++ {
			w := qaSubWorld(fmt.Sprintf("W%d", i))
			ea := qaSubEnv(fmt.Sprintf("%s#%d", actor, i))
			eb := qaSubEnv(fmt.Sprintf("%s#%d", other, i))
			if _, err := s.AppendBatch([]*vttv1.Envelope{w}, []store.Entry{
				{ActorID: other, Cause: 0, Env: eb},
				{ActorID: actor, Cause: 0, Env: ea},
			}); err != nil {
				t.Errorf("round %d: append %d: %v", round, i, err)
				return
			}
			s.Notify(w)
			s.NotifyCharacter(other, eb)
			s.NotifyCharacter(actor, ea)
			progress <- i
		}
	}()
	launch(0)
	for p := range progress {
		launch(p)
	}
	<-appendDone
	wg.Wait()
	for idx := range racers {
		if !started[idx] {
			t.Errorf("round %d: racer %d never started", round, idx)
		}
	}
}

// VTT-336.
func TestQASubStalledSubscriptionIsClosedAfterItsBudget(t *testing.T) {
	const budget = 2 * time.Second
	appendThree := func(t *testing.T, s *store.Store) {
		t.Helper()
		b := qaSubAppend(t, s, []string{"W1"},
			qaSubEnt{"qa-a", 0, "qa-a#1"}, qaSubEnt{"qa-a", 0, "qa-a#2"}, qaSubEnt{"qa-a", 0, "qa-a#3"})
		qaSubNotifyAll(s, b)
	}
	t.Run("live entries waiting, open just before the budget", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s := qaSubOpen(t)
			ch, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, budget)
			appendThree(t, s)
			time.Sleep(budget - time.Millisecond)
			qaSubExpect(t, "just before the budget", ch, qaSubEntryKeys("qa-a", 1, 3))
		})
	})
	t.Run("live entries waiting, closed at the budget", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s := qaSubOpen(t)
			ch, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, budget)
			appendThree(t, s)
			time.Sleep(budget + time.Millisecond)
			got, closed := qaSubDrain(ch)
			if !closed {
				t.Errorf("not closed after the budget; read %v", got)
			}
			if !qaSubIsPrefix(got, qaSubEntryKeys("qa-a", 1, 3)) || len(got) == 3 {
				t.Errorf("read %v before the close, want a strict in-order prefix of entries 1..3", got)
			}
		})
	})
	t.Run("history waiting, closed at the budget", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s := qaSubOpen(t)
			appendThree(t, s)
			ch, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, budget)
			time.Sleep(budget + time.Millisecond)
			got, closed := qaSubDrain(ch)
			if !closed {
				t.Errorf("not closed after the budget; read %v", got)
			}
		})
	})
	t.Run("a reader that keeps accepting within the budget is never closed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s := qaSubOpen(t)
			ch, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, budget)
			var ents []qaSubEnt
			for i := 1; i <= 6; i++ {
				ents = append(ents, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", i)})
			}
			qaSubNotifyAll(s, qaSubAppend(t, s, []string{"W1"}, ents...))
			var got []string
			for range 6 {
				time.Sleep(budget - time.Millisecond)
				synctest.Wait()
				select {
				case e, ok := <-ch:
					if !ok {
						t.Fatalf("closed after %v although each entry was accepted within the budget", got)
					}
					got = append(got, qaSubKey(e))
				default:
					t.Fatalf("nothing to read after %v", got)
				}
			}
			if !slices.Equal(got, qaSubEntryKeys("qa-a", 1, 6)) {
				t.Errorf("got %v", got)
			}
			qaSubExpect(t, "afterwards", ch, nil)
		})
	})
	t.Run("an idle subscription with nothing waiting is not closed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s := qaSubOpen(t)
			ch, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, budget)
			time.Sleep(10 * budget)
			qaSubNotifyAll(s, qaSubAppend(t, s, []string{"W1"}, qaSubEnt{"qa-a", 0, "qa-a#1"}))
			qaSubExpect(t, "after idling", ch, qaSubEntryKeys("qa-a", 1, 1))
			time.Sleep(10 * budget)
			qaSubNotifyAll(s, qaSubAppend(t, s, []string{"W2"}, qaSubEnt{"qa-a", 0, "qa-a#2"}))
			qaSubExpect(t, "after idling again", ch, qaSubEntryKeys("qa-a", 2, 2))
		})
	})
	for _, given := range []time.Duration{0, -time.Second} {
		t.Run(fmt.Sprintf("budget %v is the default, open just before it", given), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := qaSubOpen(t)
				ch, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, given)
				appendThree(t, s)
				time.Sleep(store.SubscriberNoProgressTimeout - time.Millisecond)
				qaSubExpect(t, "just before the default", ch, qaSubEntryKeys("qa-a", 1, 3))
			})
		})
		t.Run(fmt.Sprintf("budget %v is the default, closed at it", given), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := qaSubOpen(t)
				ch, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, given)
				appendThree(t, s)
				time.Sleep(store.SubscriberNoProgressTimeout + time.Millisecond)
				got, closed := qaSubDrain(ch)
				if !closed {
					t.Errorf("not closed after the default budget; read %v", got)
				}
			})
		})
	}
}

// VTT-337.
func TestQASubStalledSubscriptionDelaysAndClosesNoOther(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		const budget = time.Second
		stalled, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, budget)
		reader, _ := qaSubSubscribe(t, s, "qa-a", 0, 1, budget)
		late, _ := qaSubSubscribe(t, s, "qa-a", 2, 1, budget)
		other, _ := qaSubSubscribe(t, s, "qa-b", 0, 1, budget)
		world, unsubW, _, err := s.SubscribeWithNoProgressTimeout(0, 1, budget)
		if err != nil {
			t.Fatalf("world subscribe: %v", err)
		}
		t.Cleanup(unsubW)

		appendRange := func(from, to int) {
			for i := from; i <= to; i++ {
				b := qaSubAppend(t, s, []string{fmt.Sprintf("W%d", i)},
					qaSubEnt{"qa-b", 0, fmt.Sprintf("qa-b#%d", i)}, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", i)})
				qaSubNotifyAll(s, b)
			}
		}
		start := time.Now()
		appendRange(1, 30)
		if d := time.Since(start); d != 0 {
			t.Errorf("appending and notifying took %v of fake time with a stalled subscriber", d)
		}
		qaSubExpect(t, "reader", reader, qaSubEntryKeys("qa-a", 1, 30))
		qaSubExpect(t, "late reader", late, qaSubEntryKeys("qa-a", 3, 30))
		qaSubExpect(t, "other log", other, qaSubEntryKeys("qa-b", 1, 30))
		qaSubExpect(t, "world", world, qaSubWorldKeys(1, 30))
		if d := time.Since(start); d != 0 {
			t.Errorf("the others were delivered after %v of fake time", d)
		}

		time.Sleep(budget + time.Millisecond)
		got, closed := qaSubDrain(stalled)
		if !closed {
			t.Errorf("stalled subscription not closed after its budget; read %v", got)
		}
		qaSubExpect(t, "reader after the drop", reader, nil)
		qaSubExpect(t, "late reader after the drop", late, nil)
		qaSubExpect(t, "other log after the drop", other, nil)
		qaSubExpect(t, "world after the drop", world, nil)

		appendRange(31, 35)
		qaSubExpect(t, "reader keeps receiving", reader, qaSubEntryKeys("qa-a", 31, 35))
		qaSubExpect(t, "late reader keeps receiving", late, qaSubEntryKeys("qa-a", 31, 35))
		qaSubExpect(t, "other log keeps receiving", other, qaSubEntryKeys("qa-b", 31, 35))
		qaSubExpect(t, "world keeps receiving", world, qaSubWorldKeys(31, 35))
	})
}

// VTT-335, the store's half.
func TestQASubClosingTheStoreEndsEveryCharacterSubscription(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, err := store.Open(filepath.Join(t.TempDir(), "qa-sub-close.db"))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		var ents []qaSubEnt
		for i := 1; i <= 5; i++ {
			ents = append(ents, qaSubEnt{"qa-a", 0, fmt.Sprintf("qa-a#%d", i)})
		}
		ents = append(ents, qaSubEnt{"qa-b", 0, "qa-b#1"}, qaSubEnt{"qa-b", 0, "qa-b#2"})
		qaSubAppend(t, s, []string{"W1"}, ents...)

		subs := map[string]<-chan *vttv1.Envelope{}
		var unsubs []func()
		add := func(name, actor string, cursor int64, buffer int) {
			ch, unsub, _, err := s.SubscribeCharacter(actor, cursor, buffer, 0)
			if err != nil {
				t.Fatalf("%s: subscribe: %v", name, err)
			}
			subs[name] = ch
			unsubs = append(unsubs, unsub)
		}
		add("stalled with history waiting", "qa-a", 0, 1)
		add("caught up", "qa-a", 5, 4)
		add("other log", "qa-b", 0, 4)
		add("empty log", "qa-ghost", 0, 4)
		_, unsubGone, _, err := s.SubscribeCharacter("qa-a", 0, 8, 0)
		if err != nil {
			t.Fatalf("gone: subscribe: %v", err)
		}
		unsubGone()
		qaSubExpect(t, "other log before close", subs["other log"], qaSubEntryKeys("qa-b", 1, 2))

		start := time.Now()
		if err := s.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
		if d := time.Since(start); d != 0 {
			t.Errorf("close took %v of fake time", d)
		}
		for name, ch := range subs {
			got, closed := qaSubDrain(ch)
			if !closed {
				t.Errorf("%s: channel still open after the store closed; read %v", name, got)
			}
		}
		if d := time.Since(start); d != 0 {
			t.Errorf("subscriptions ended after %v of fake time", d)
		}
		for _, u := range unsubs {
			u()
		}
	})
}

// The store's contract after Close (TestOperationsFailAfterClose).
func TestQASubSubscribingToAClosedStoreEndsAtOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, err := store.Open(filepath.Join(t.TempDir(), "qa-sub-closed.db"))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		qaSubAppend(t, s, []string{"W1"}, qaSubEnt{"qa-a", 0, "qa-a#1"})
		if err := s.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		ch, unsub, _, err := s.SubscribeCharacter("qa-a", 0, 4, 0)
		if err != nil {
			return
		}
		defer unsub()
		got, closed := qaSubDrain(ch)
		if !closed {
			t.Errorf("a subscription to a closed store is open; read %v", got)
		}
	})
}

// VTT-332, the store's precondition.
func TestQASubAFailedAppendsEntryReachesNoSubscriber(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := qaSubOpen(t)
		first := qaSubAppend(t, s, []string{"W1"})
		low, _ := qaSubSubscribe(t, s, "qa-a", -1, 4, 0)
		zero, _ := qaSubSubscribe(t, s, "qa-a", 0, 4, 0)

		dup := qaSubEnv("W-dup")
		dup.EventId = first.worlds[0].GetEventId()
		failed := qaSubEnv("qa-a#failed")
		if _, err := s.AppendBatch([]*vttv1.Envelope{dup}, []store.Entry{{ActorID: "qa-a", Cause: 0, Env: failed}}); err == nil {
			t.Fatalf("append of a duplicate event id succeeded")
		}
		if failed.GetSequence() != 0 {
			t.Fatalf("failed entry stamped %d", failed.GetSequence())
		}
		s.NotifyCharacter("qa-a", failed)
		qaSubExpect(t, "after -1", low, nil)
		qaSubExpect(t, "after 0", zero, nil)

		qaSubNotifyAll(s, qaSubAppend(t, s, []string{"W2"}, qaSubEnt{"qa-a", 0, "qa-a#1"}))
		qaSubExpect(t, "after -1, then a real entry", low, qaSubEntryKeys("qa-a", 1, 1))
		qaSubExpect(t, "after 0, then a real entry", zero, qaSubEntryKeys("qa-a", 1, 1))
	})
}
