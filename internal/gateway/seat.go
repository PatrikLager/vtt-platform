package gateway

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// Answer false for the DM and the agent alone: every other role is sent one
// character log, never the world (SPEC-015, VTT-366).
func watchesALog(r identity.Role) bool {
	switch r {
	case identity.RoleDM, identity.RoleAgent:
		return false
	}
	return true
}

var errWatchEnded = errors.New("gateway: the campaign ended this seat's watch")

// Touch a seat from the pump alone once serve has queued its head, told aside:
// frames must leave in the order the seat changed (SPEC-011, SPEC-015).
type seat struct {
	c          *campaign.Campaign
	p          *identity.Participant
	buffer     int
	noProgress time.Duration

	events <-chan *vttv1.Envelope
	unsub  func()

	// Keep wake nil for a seat sent the world: only a log seat switches.
	wake      <-chan struct{}
	stopWake  func()
	shown     string
	judgedAs  identity.Role
	watchable *vttv1.WatchableChanged
	// Read told from any goroutine; watch stores it before a frame names it (VTT-453).
	told atomic.Pointer[string]
}

// openSeat subscribes p's connection: the world for the DM and the agent, else
// the log named when p may watch it, or p's default (SPEC-015, VTT-339).
func openSeat(c *campaign.Campaign, p *identity.Participant, named string, after int64,
	buffer int, noProgress time.Duration) (*seat, int64, error) {
	s := &seat{c: c, p: p, buffer: buffer, noProgress: noProgress, judgedAs: p.Role}
	if !watchesALog(p.Role) {
		events, unsub, head, err := c.SubscribeWithNoProgressTimeout(after, buffer, noProgress)
		if err != nil {
			return nil, 0, err
		}
		s.events, s.unsub = events, unsub
		return s, head, nil
	}
	wake, stop, err := c.WatchAppends()
	if err != nil {
		return nil, 0, err
	}
	s.wake, s.stopWake = wake, stop
	actors := c.Actors()
	s.watchable = watchable(p, actors)
	target, cursor := defaultViewpoint(p, actors), int64(0)
	if named != "" && MayWatch(p, named, actors) == nil {
		// Honour the cursor in the log it names alone (VTT-349).
		target, cursor = named, after
	}
	head, err := s.watch(target, cursor)
	if err != nil {
		s.close()
		return nil, 0, err
	}
	return s, head, nil
}

// watch moves the seat onto actorID's log after cursor, and keeps moving while a
// waiting wake resolves elsewhere, so the head it returns is the log it shows
// (VTT-384).
func (s *seat) watch(actorID string, cursor int64) (int64, error) {
	for {
		s.drop()
		shown := actorID
		s.shown, s.judgedAs = shown, s.p.Role
		s.told.Store(&shown)
		var head int64
		if actorID != "" {
			events, unsub, h, err := s.c.SubscribeCharacter(actorID, cursor, s.buffer, s.noProgress)
			if err != nil {
				return 0, err
			}
			s.events, s.unsub, head = events, unsub, h
		}
		select {
		case _, ok := <-s.wake:
			if !ok {
				return 0, errWatchEnded
			}
			next := s.resolve(s.c.Actors())
			if next == actorID {
				return head, nil
			}
			actorID, cursor = next, 0
		default:
			return head, nil
		}
	}
}

// resolve keeps what the seat shows while p may watch it, else p's default
// (VTT-340).
func (s *seat) resolve(actors map[string]*vttv1.Actor) string {
	s.watchable = watchable(s.p, actors)
	if s.shown != "" && MayWatch(s.p, s.shown, actors) == nil {
		return s.shown
	}
	return defaultViewpoint(s.p, actors)
}

// recheck reports whether the actors after a wake moved the seat (VTT-348).
func (s *seat) recheck() (bool, error) {
	s.judgedAs = s.p.Role
	next := s.resolve(s.c.Actors())
	if next == s.shown {
		return false, nil
	}
	_, err := s.watch(next, 0)
	return true, err
}

// rejudge rechecks a seat judged by a role its participant lost (VTT-387).
func (s *seat) rejudge() (bool, error) {
	if s.p.Role == s.judgedAs {
		return false, nil
	}
	return s.recheck()
}

// admit decides an entry the pump has taken: a waiting wake is read first, since
// its append raised it before any entry it caused, and an entry of a log the
// seat has just left is not sent (VTT-348, VTT-355).
func (s *seat) admit() (send, moved bool, err error) {
	select {
	case _, ok := <-s.wake:
		if !ok {
			return false, false, errWatchEnded
		}
		moved, err := s.recheck()
		return !moved && err == nil, moved, err
	default:
		return true, false, nil
	}
}

// Nil the channel with the subscription: a dropped subscription's channel
// closes, and the pump reads a closed one as the store's drop.
func (s *seat) drop() {
	if s.unsub != nil {
		s.unsub()
	}
	s.events, s.unsub = nil, nil
}

func (s *seat) close() {
	s.drop()
	if s.stopWake != nil {
		s.stopWake()
	}
}

// Keep one slot, latest wins, and never block set: a queue's depth is the
// client's to choose, and a blocking hand-off stalls the table (SPEC-015,
// docs/reports/2026-08-18-visibility.md). Guard the slot with mu and nothing
// else: the pump orders frames, not this lock.
type perchBox struct {
	mu       sync.Mutex
	shoulder string
	full     bool
	// Keep wake at capacity 1: unbuffered, set's send is dropped while the pump is
	// busy and the shoulder waits for the next hop.
	wake chan struct{}
}

func newPerchBox() *perchBox { return &perchBox{wake: make(chan struct{}, 1)} }

// Never block here: the command goroutine must not wait on the pump (SPEC-015).
func (b *perchBox) set(actorID string) {
	b.mu.Lock()
	b.shoulder, b.full = actorID, true
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Report ok=false once the slot is empty: a second wake-up must not re-apply a
// shoulder (TestARapidHopIsCoalescedToTheShoulderItEndedOn).
func (b *perchBox) take() (actorID string, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	actorID, ok = b.shoulder, b.full
	b.full = false
	return actorID, ok
}

// judgedBy is the told one while it is p's party member, else p's default (VTT-453).
func (s *seat) judgedBy(p *identity.Participant, actors map[string]*vttv1.Actor) string {
	viewpoint := ""
	if told := s.told.Load(); told != nil {
		viewpoint = *told
	}
	if !ownPartyMember(actors[viewpoint], p.ID) {
		return defaultViewpoint(p, actors)
	}
	return viewpoint
}

// Ask perceive afresh per question: canSee must record nothing a seat was
// shown (SPEC-015).
func canSee(viewpoint string, st *engine.State, sceneID string, at *vttv1.GridPosition) bool {
	return perceive.CanSee(perceive.Eyes{Shoulder: viewpoint}, st, sceneID, at)
}
