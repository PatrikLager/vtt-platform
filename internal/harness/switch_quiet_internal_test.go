package harness

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

const switchQuietWindow = 100 * time.Millisecond

func sendAroundASwitch(ch chan<- Delivery, done chan<- struct{}) {
	defer close(done)
	ch <- Delivery{Envelope: env(1)}
	time.Sleep(80 * time.Millisecond)
	ch <- Delivery{Viewpoint: &vttv1.ViewpointChanged{ActorId: "a2"}}
	time.Sleep(70 * time.Millisecond)
	ch <- Delivery{Envelope: env(2)}
}

func TestASwitchDoesNotExtendDrainToSequencesQuietWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ch, done := make(chan Delivery, 4), make(chan struct{})
		go sendAroundASwitch(ch, done)
		start := time.Now()
		got, reached := drainToSequence(ch, 1, switchQuietWindow, time.Second, nil)
		if took := time.Since(start); !reached || len(got) != 1 || took != switchQuietWindow {
			t.Fatalf("drained %d entries, reached=%t, after %s; want 1 at %s", len(got), reached, took, switchQuietWindow)
		}
		<-done
	})
}

func TestASwitchDoesNotExtendDrainQuietIntosWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		conn, done := newStubConn(), make(chan struct{})
		go sendAroundASwitch(conn.events, done)
		history := map[string][]Delivery{}
		var mu sync.Mutex
		start := time.Now()
		ended := drainQuietInto("p", conn, history, &mu, switchQuietWindow)
		if took := time.Since(start); ended || len(entriesOf(history["p"])) != 1 || took != switchQuietWindow {
			t.Fatalf("drained %d entries, ended=%t, after %s; want 1 at %s", len(entriesOf(history["p"])), ended, took, switchQuietWindow)
		}
		<-done
	})
}
