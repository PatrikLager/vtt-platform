package campaign_test

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	_ "modernc.org/sqlite"
)

const (
	qaWatchDeadline = 3 * time.Second
	qaWatchQuiet    = 75 * time.Millisecond
	qaWatchParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaWatchNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

var qaWatchEventN atomic.Int64

func qaWatchEnv(t *testing.T, p proto.Message) *vttv1.Envelope {
	t.Helper()
	e := &vttv1.Envelope{
		EventId:    fmt.Sprintf("qa-watch-%d", qaWatchEventN.Add(1)),
		OccurredAt: timestamppb.Now(),
	}
	switch m := p.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: m}
	case *vttv1.SessionEnded:
		e.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: m}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: m}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: m}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: m}
	case *vttv1.TokenRemoved:
		e.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: m}
	case *vttv1.NarrationAdded:
		e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: m}
	case *vttv1.NoteUpserted:
		e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: m}
	case *vttv1.NoteDeleted:
		e.Payload = &vttv1.Envelope_NoteDeleted{NoteDeleted: m}
	case *vttv1.ResourceChanged:
		e.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: m}
	case *vttv1.ConditionApplied:
		e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: m}
	case *vttv1.ActorControlGranted:
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: m}
	case *vttv1.AdventureLoaded:
		e.Payload = &vttv1.Envelope_AdventureLoaded{AdventureLoaded: m}
	default:
		t.Fatalf("qaWatchEnv: no arm for %T", p)
	}
	return e
}

func qaWatchOpen(t *testing.T) *campaign.Campaign {
	t.Helper()
	c, err := campaign.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func qaWatchEnvs(t *testing.T, ps ...proto.Message) []*vttv1.Envelope {
	t.Helper()
	envs := make([]*vttv1.Envelope, len(ps))
	for i, p := range ps {
		envs[i] = qaWatchEnv(t, p)
	}
	return envs
}

func qaWatchSend(c *campaign.Campaign, envs []*vttv1.Envelope) (int64, error) {
	if len(envs) == 1 {
		return c.Append(envs[0])
	}
	return c.AppendBatch(envs)
}

func qaWatchAppend(t *testing.T, c *campaign.Campaign, ps ...proto.Message) (int64, []*vttv1.Envelope) {
	t.Helper()
	envs := qaWatchEnvs(t, ps...)
	seq, err := qaWatchSend(c, envs)
	if err != nil {
		t.Fatalf("append %T...: %v", ps[0], err)
	}
	return seq, envs
}

func qaWatchActor(id string, kind vttv1.ActorKind) *vttv1.ActorAdded {
	return &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId:   id,
		Name:      strings.ToUpper(id[:1]) + id[1:],
		Kind:      kind,
		Resources: map[string]*vttv1.Resource{"focus": {Current: 5, Max: 5}},
	}}
}

func qaWatchToken(actor, scene string, x, y int32) *vttv1.TokenPlaced {
	return &vttv1.TokenPlaced{
		TokenId:  "t-" + actor,
		SceneId:  scene,
		ActorId:  actor,
		Position: &vttv1.GridPosition{X: x, Y: y},
	}
}

func qaWatchHall() *vttv1.SceneCreated {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 9; x++ {
		for y := int32(0); y < 3; y++ {
			kind := "floor"
			if x == 4 {
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return &vttv1.SceneCreated{SceneId: "hall", Name: "hall", GridWidth: 9, GridHeight: 3, Tiles: tiles}
}

func qaWatchTableSteps() [][]proto.Message {
	return [][]proto.Message{
		{&vttv1.SessionStarted{Name: "first"}},
		{qaWatchHall()},
		{&vttv1.SceneCreated{SceneId: "vault", Name: "vault", GridWidth: 3, GridHeight: 3}},
		{qaWatchActor("hero", qaWatchParty)},
		{qaWatchToken("hero", "hall", 1, 1)},
		{qaWatchActor("scout", qaWatchParty)},
		{qaWatchToken("scout", "hall", 7, 1)},
		{qaWatchActor("mage", qaWatchParty)},
		{qaWatchActor("goblin", qaWatchNonParty)},
		{qaWatchToken("goblin", "hall", 2, 1)},
		{qaWatchActor("rogue", qaWatchParty)},
		{&vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-rogue", Kind: qaWatchNonParty}},
	}
}

func qaWatchTable(t *testing.T) *campaign.Campaign {
	t.Helper()
	c := qaWatchOpen(t)
	for _, step := range qaWatchTableSteps() {
		qaWatchAppend(t, c, step...)
	}
	qaWatchRequireFixture(t, c)
	return c
}

func qaWatchLog(t *testing.T, c *campaign.Campaign, id string) []campaign.CharacterEntry {
	t.Helper()
	log, err := c.CharacterLog(id)
	if err != nil {
		t.Fatalf("CharacterLog(%s): %v", id, err)
	}
	return log
}

func qaWatchFold(t *testing.T, log []campaign.CharacterEntry) *engine.State {
	t.Helper()
	envs := make([]*vttv1.Envelope, len(log))
	for i, e := range log {
		envs[i] = e.Env
	}
	st, err := campaign.FoldPrefix(envs)
	if err != nil {
		t.Fatalf("fold a character log: %v", err)
	}
	return st
}

func qaWatchThrough(log []campaign.CharacterEntry, worldSeq int64) []campaign.CharacterEntry {
	end := -1
	for i, e := range log {
		if e.WorldSeq == worldSeq {
			end = i
		}
	}
	return log[:end+1]
}

func qaWatchAt(log []campaign.CharacterEntry, worldSeq int64) []campaign.CharacterEntry {
	var out []campaign.CharacterEntry
	for _, e := range log {
		if e.WorldSeq == worldSeq {
			out = append(out, e)
		}
	}
	return out
}

func qaWatchActive(t *testing.T, c *campaign.Campaign) []string {
	t.Helper()
	actors := c.Actors()
	if actors == nil {
		t.Fatalf("Actors is nil")
	}
	var ids []string
	for id, a := range actors {
		if engine.IsPartyMember(a) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func qaWatchHoldsTokenOf(st *engine.State, actor string) bool {
	for _, tok := range st.Tokens {
		if tok.ActorID == actor {
			return true
		}
	}
	return false
}

func qaWatchRequireFixture(t *testing.T, c *campaign.Campaign) {
	t.Helper()
	active := qaWatchActive(t, c)
	if strings.Join(active, ",") != "hero,mage,scout" {
		t.Fatalf("fixture: active logs are %v, want hero, mage, scout", active)
	}
	hero := qaWatchFold(t, qaWatchLog(t, c, "hero"))
	scout := qaWatchFold(t, qaWatchLog(t, c, "scout"))
	mage := qaWatchFold(t, qaWatchLog(t, c, "mage"))
	if qaWatchHoldsTokenOf(hero, "scout") || qaWatchHoldsTokenOf(scout, "hero") {
		t.Fatalf("fixture: hero and scout must not see each other")
	}
	if !qaWatchHoldsTokenOf(hero, "goblin") || qaWatchHoldsTokenOf(scout, "goblin") {
		t.Fatalf("fixture: goblin must be seen by hero alone")
	}
	if len(mage.Tokens) != 0 {
		t.Fatalf("fixture: mage must see no token, holds %v", mage.Tokens)
	}
	if len(qaWatchLog(t, c, "rogue")) == 0 {
		t.Fatalf("fixture: rogue must hold a paused log")
	}
}

func qaWatchWoken(wake <-chan struct{}, within time.Duration) bool {
	select {
	case _, ok := <-wake:
		return ok
	case <-time.After(within):
		return false
	}
}

func qaWatchDrain(wake <-chan struct{}) {
	for {
		select {
		case _, ok := <-wake:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

func qaWatchWatch(t *testing.T, c *campaign.Campaign) <-chan struct{} {
	t.Helper()
	wake, stop, err := c.WatchAppends()
	if err != nil {
		t.Fatalf("WatchAppends: %v", err)
	}
	t.Cleanup(stop)
	return wake
}

func qaWatchBare(env *vttv1.Envelope) *vttv1.Envelope {
	out := proto.Clone(env).(*vttv1.Envelope)
	out.EventId, out.Sequence, out.OccurredAt = "", 0, nil
	out.SessionId, out.ActorRole, out.ParticipantId = "", "", ""
	return out
}

func qaWatchNarrations(log []campaign.CharacterEntry, text string) []campaign.CharacterEntry {
	var out []campaign.CharacterEntry
	for _, e := range log {
		if e.Env.GetNarrationAdded() != nil && e.Env.GetNarrationAdded().GetText() == text {
			out = append(out, e)
		}
	}
	return out
}

// VTT-353
func TestQAWatchEverySuccessfulAppendWakesEveryWatch(t *testing.T) {
	c := qaWatchOpen(t)
	undrained := qaWatchWatch(t, c)
	early := []<-chan struct{}{qaWatchWatch(t, c), qaWatchWatch(t, c), qaWatchWatch(t, c)}
	steps := [][]proto.Message{
		{&vttv1.SceneCreated{SceneId: "vault", Name: "vault", GridWidth: 3, GridHeight: 3}},
		{&vttv1.SessionStarted{Name: "first"}},
		{qaWatchActor("hero", qaWatchParty), qaWatchToken("hero", "vault", 1, 1)},
		{&vttv1.NarrationAdded{Text: "the vault is cold"}},
		{qaWatchActor("scout", qaWatchParty)},
		{&vttv1.NarrationAdded{Text: "a"}, &vttv1.NarrationAdded{Text: "b"}},
	}
	var late []<-chan struct{}
	for i, step := range steps {
		if i == 2 {
			late = append(late, qaWatchWatch(t, c))
		}
		envs := qaWatchEnvs(t, step...)
		done := make(chan error, 1)
		go func() {
			_, err := qaWatchSend(c, envs)
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("step %d: %v", i, err)
			}
		case <-time.After(qaWatchDeadline):
			t.Fatalf("step %d: the append did not return with an undrained watch", i)
		}
		for j, w := range append(append([]<-chan struct{}{}, early...), late...) {
			if !qaWatchWoken(w, qaWatchDeadline) {
				t.Errorf("step %d (%T): watch %d was not woken", i, step[0], j)
			}
		}
	}
	if !qaWatchWoken(undrained, qaWatchDeadline) {
		t.Errorf("the undrained watch holds no wake after %d appends", len(steps))
	}
}

// VTT-354
func TestQAWatchRefusedAppendWakesNoWatch(t *testing.T) {
	c := qaWatchTable(t)
	watches := []<-chan struct{}{qaWatchWatch(t, c), qaWatchWatch(t, c)}
	qaWatchAppend(t, c, &vttv1.NarrationAdded{Text: "before"})
	for j, w := range watches {
		if !qaWatchWoken(w, qaWatchDeadline) {
			t.Fatalf("fixture: watch %d not live before the refusal", j)
		}
		qaWatchDrain(w)
	}
	refusals := [][]*vttv1.Envelope{
		{qaWatchEnv(t, &vttv1.TokenPlaced{
			TokenId: "t-astray", SceneId: "nowhere", ActorId: "hero", Position: &vttv1.GridPosition{},
		})},
		{
			qaWatchEnv(t, &vttv1.NarrationAdded{Text: "half a batch"}),
			qaWatchEnv(t, qaWatchToken("nobody", "hall", 0, 0)),
		},
		{qaWatchEnv(t, qaWatchActor("hero", qaWatchParty))},
	}
	for i, envs := range refusals {
		_, err := qaWatchSend(c, envs)
		if err == nil {
			t.Fatalf("refusal %d: fixture: the append must be refused", i)
		}
		for j, w := range watches {
			if qaWatchWoken(w, qaWatchQuiet) {
				t.Errorf("refusal %d: watch %d was woken by a refused append (%v)", i, j, err)
			}
		}
	}
	qaWatchAppend(t, c, &vttv1.NarrationAdded{Text: "after"})
	for j, w := range watches {
		if !qaWatchWoken(w, qaWatchDeadline) {
			t.Errorf("watch %d not woken by the success after the refusals", j)
		}
	}
}

// VTT-354
func TestQAWatchFailedWriteWakesNoWatch(t *testing.T) {
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	for _, step := range qaWatchTableSteps() {
		qaWatchAppend(t, c, step...)
	}
	db, err := sql.Open("sqlite", campaign.LogPath(dir)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	watches := []<-chan struct{}{qaWatchWatch(t, c), qaWatchWatch(t, c)}
	for _, table := range []string{"events", "character_events"} {
		qaWatchAppend(t, c, &vttv1.NarrationAdded{Text: "live before " + table})
		for j, w := range watches {
			if !qaWatchWoken(w, qaWatchDeadline) {
				t.Fatalf("%s: fixture: watch %d not live", table, j)
			}
			qaWatchDrain(w)
		}
		trigger := "qa_watch_fail_" + table
		if _, err := db.Exec("CREATE TRIGGER " + trigger + " BEFORE INSERT ON " + table +
			" BEGIN SELECT RAISE(ABORT, 'qa watch'); END"); err != nil {
			t.Fatalf("create trigger: %v", err)
		}
		_, appendErr := c.Append(qaWatchEnv(t, &vttv1.NarrationAdded{Text: "fails on " + table}))
		if _, err := db.Exec("DROP TRIGGER " + trigger); err != nil {
			t.Fatalf("drop trigger: %v", err)
		}
		if appendErr == nil {
			t.Fatalf("%s: fixture: the write must fail", table)
		}
		for j, w := range watches {
			if qaWatchWoken(w, qaWatchQuiet) {
				t.Errorf("%s: watch %d was woken by a failed append (%v)", table, j, appendErr)
			}
		}
	}
	qaWatchAppend(t, c, &vttv1.NarrationAdded{Text: "after"})
	for j, w := range watches {
		if !qaWatchWoken(w, qaWatchDeadline) {
			t.Errorf("watch %d not woken by the success after the failures", j)
		}
	}
}

type qaWatchAck struct {
	log    string
	round  int
	waking bool
}

// VTT-353 VTT-362
func TestQAWatchAWakeWaitsWhenEachRoundsFirstEntryArrives(t *testing.T) {
	c := qaWatchTable(t)
	active := qaWatchActive(t, c)
	acks := make(chan qaWatchAck, 64)
	done := make(chan struct{})
	var wg sync.WaitGroup
	for _, id := range active {
		log := qaWatchLog(t, c, id)
		events, unsub, _, err := c.SubscribeCharacter(id, log[len(log)-1].Env.GetSequence(), 64, 10*time.Second)
		if err != nil {
			t.Fatalf("SubscribeCharacter(%s): %v", id, err)
		}
		t.Cleanup(unsub)
		wake := qaWatchWatch(t, c)
		wg.Add(1)
		go func() {
			defer wg.Done()
			awaiting, waking := true, false
			for {
				var env *vttv1.Envelope
				var ok bool
				select {
				case env, ok = <-events:
					if !ok {
						return
					}
				case <-done:
					return
				}
				if awaiting {
					select {
					case <-wake:
						waking = true
					default:
						waking = false
					}
					awaiting = false
				}
				var round int
				text := env.GetNarrationAdded().GetText()
				if _, err := fmt.Sscanf(text, "round %d end", &round); err == nil {
					acks <- qaWatchAck{log: id, round: round, waking: waking}
					awaiting = true
				}
			}
		}()
	}
	t.Cleanup(func() { close(done); wg.Wait() })
	const rounds = 300
	late := 0
	for r := 0; r < rounds; r++ {
		end := &vttv1.NarrationAdded{Text: fmt.Sprintf("round %d end", r)}
		if r%2 == 0 {
			qaWatchAppend(t, c, end)
		} else {
			qaWatchAppend(t, c, &vttv1.NarrationAdded{Text: fmt.Sprintf("round %d start", r)}, end)
		}
		for range active {
			select {
			case a := <-acks:
				if a.round != r {
					t.Fatalf("round %d: %s acknowledged round %d", r, a.log, a.round)
				}
				if !a.waking {
					late++
					t.Errorf("round %d: %s's first entry arrived before the wake", r, a.log)
				}
			case <-time.After(qaWatchDeadline):
				t.Fatalf("round %d: an active log did not deliver the narration", r)
			}
		}
	}
	t.Logf("%d of %d deliveries came before their wake", late, rounds*len(active))
}

// VTT-353
func TestQAWatchWokenWatcherFindsTheAppendApplied(t *testing.T) {
	c := qaWatchTable(t)
	wake := qaWatchWatch(t, c)
	for _, id := range []string{"bard", "lurker", "warden"} {
		seen := make(chan bool, 1)
		go func() {
			if !qaWatchWoken(wake, qaWatchDeadline) {
				seen <- false
				return
			}
			_, inActors := c.Actors()[id]
			st := c.State()
			seen <- inActors && st != nil && st.Actors[id] != nil
		}()
		qaWatchAppend(t, c, qaWatchActor(id, qaWatchParty))
		select {
		case ok := <-seen:
			if !ok {
				t.Errorf("%s: woken watcher did not find the append in Actors and State", id)
			}
		case <-time.After(2 * qaWatchDeadline):
			t.Fatalf("%s: the watcher never reported", id)
		}
	}
}

// VTT-356
func TestQAWatchCloseEndsEveryWatch(t *testing.T) {
	c, err := campaign.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	var stops []func()
	watch := func() <-chan struct{} {
		w, stop, err := c.WatchAppends()
		if err != nil {
			t.Fatalf("WatchAppends: %v", err)
		}
		stops = append(stops, stop)
		return w
	}
	pending := watch()
	drained := watch()
	watch()
	stops[len(stops)-1]()
	for _, step := range qaWatchTableSteps()[:5] {
		qaWatchAppend(t, c, step...)
	}
	qaWatchDrain(drained)
	fresh := watch()
	blocked := watch()
	released := make(chan bool, 1)
	go func() {
		for {
			_, ok := <-blocked
			if !ok {
				released <- true
				return
			}
		}
	}()
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for name, w := range map[string]<-chan struct{}{"pending": pending, "drained": drained, "fresh": fresh} {
		ended := false
		deadline := time.After(qaWatchDeadline)
	recv:
		for {
			select {
			case _, ok := <-w:
				if !ok {
					ended = true
					break recv
				}
			case <-deadline:
				break recv
			}
		}
		if !ended {
			t.Errorf("%s watch did not end when the campaign closed", name)
		}
	}
	select {
	case <-released:
	case <-time.After(qaWatchDeadline):
		t.Errorf("a receiver blocked on its wake was not released by Close")
	}
	for _, stop := range stops {
		stop()
	}
}

// VTT-356
func TestQAWatchWatchesComeAndGoDuringAppends(t *testing.T) {
	c, err := campaign.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	const watchers, appends = 4, 120
	var wg sync.WaitGroup
	quit := make(chan struct{})
	for g := 0; g < watchers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				w, stop, err := c.WatchAppends()
				if err != nil {
					return
				}
				select {
				case _, ok := <-w:
					stop()
					if !ok {
						return
					}
				case <-quit:
					_, ok := <-w
					for ok {
						_, ok = <-w
					}
					stop()
					return
				}
			}
		}()
	}
	for i := 0; i < appends; i++ {
		qaWatchAppend(t, c, &vttv1.SceneCreated{SceneId: fmt.Sprintf("room-%d", i), Name: "room", GridWidth: 2, GridHeight: 2})
	}
	close(quit)
	closed := make(chan error, 1)
	go func() { closed <- c.Close() }()
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(qaWatchDeadline):
		t.Fatalf("a watch live at Close did not end")
	}
	if err := <-closed; err != nil {
		t.Errorf("Close: %v", err)
	}
}

func qaWatchRequireEveryMemberIntroduced(t *testing.T, c *campaign.Campaign, when string) {
	t.Helper()
	party := qaWatchActive(t, c)
	for _, id := range party {
		st := qaWatchFold(t, qaWatchLog(t, c, id))
		for _, m := range party {
			a, ok := st.Actors[m]
			switch {
			case !ok:
				t.Errorf("%s: %s's log does not introduce party member %s", when, id, m)
			case !engine.IsPartyMember(a):
				t.Errorf("%s: %s's log holds %s, but not as a party member", when, id, m)
			}
		}
	}
}

// VTT-361
func TestQAWatchEveryLogIntroducesEveryPartyMember(t *testing.T) {
	c := qaWatchTable(t)
	qaWatchRequireEveryMemberIntroduced(t, c, "after the table")
	later := []struct {
		name string
		ps   []proto.Message
	}{
		{"bard joins with no token", []proto.Message{qaWatchActor("bard", qaWatchParty)}},
		{"lurker waits in the vault", []proto.Message{
			qaWatchActor("lurker", qaWatchNonParty), qaWatchToken("lurker", "vault", 1, 1),
		}},
		{"lurker, seen by none, joins", []proto.Message{
			&vttv1.ActorControlGranted{ActorId: "lurker", ParticipantId: "p-l", Kind: qaWatchParty},
		}},
		{"goblin, seen by hero alone, joins", []proto.Message{
			&vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "p-g", Kind: qaWatchParty},
		}},
		{"rogue resumes after members joined", []proto.Message{
			&vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-rogue", Kind: qaWatchParty},
		}},
		{"a narration", []proto.Message{&vttv1.NarrationAdded{Text: "quiet"}}},
	}
	for _, step := range later {
		qaWatchAppend(t, c, step.ps...)
		qaWatchRequireEveryMemberIntroduced(t, c, step.name)
	}
	if got := strings.Join(qaWatchActive(t, c), ","); got != "bard,goblin,hero,lurker,mage,rogue,scout" {
		t.Fatalf("fixture: party is %s", got)
	}
}

// VTT-361
func TestQAWatchEveryLogIntroducesEveryPartyMemberAtEveryStep(t *testing.T) {
	c := qaWatchOpen(t)
	for i, step := range qaWatchTableSteps() {
		qaWatchAppend(t, c, step...)
		qaWatchRequireEveryMemberIntroduced(t, c, fmt.Sprintf("table step %d", i))
	}
}

// VTT-362 VTT-316 VTT-371
func TestQAWatchNarrationReachesEveryLog(t *testing.T) {
	c := qaWatchTable(t)
	seenByHero, _ := qaWatchAppend(t, c, &vttv1.ResourceChanged{ActorId: "goblin", Resource: "focus", Delta: -1, NewValue: 4})
	cases := []struct {
		name string
		ps   []proto.Message
		text string
	}{
		{"unanchored", []proto.Message{&vttv1.NarrationAdded{Text: "the hall trembles"}}, "the hall trembles"},
		{"anchored on what hero alone saw", []proto.Message{&vttv1.NarrationAdded{
			Text: "the goblin flinches", AnchorFromSeq: seenByHero, AnchorToSeq: seenByHero,
		}}, "the goblin flinches"},
		{"inside a batch", []proto.Message{
			&vttv1.NoteUpserted{Key: "aside", Title: "Aside", Text: "x", Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET},
			&vttv1.NarrationAdded{Text: "dust falls"},
		}, "dust falls"},
	}
	for _, tc := range cases {
		_, envs := qaWatchAppend(t, c, tc.ps...)
		world := envs[len(envs)-1]
		for _, id := range qaWatchActive(t, c) {
			got := qaWatchNarrations(qaWatchLog(t, c, id), tc.text)
			if len(got) != 1 {
				t.Errorf("%s: %s's log holds %d copies of the narration, want 1", tc.name, id, len(got))
				continue
			}
			if got[0].Env.GetEventId() != world.GetEventId() {
				t.Errorf("%s: %s's narration has event_id %q, want %q",
					tc.name, id, got[0].Env.GetEventId(), world.GetEventId())
			}
		}
		if got := qaWatchNarrations(qaWatchLog(t, c, "rogue"), tc.text); len(got) != 0 {
			t.Errorf("%s: the paused rogue log was written the narration", tc.name)
		}
	}
}

// VTT-363 VTT-316 VTT-371
func TestQAWatchSessionStartAndEndReachEveryLog(t *testing.T) {
	c := qaWatchTable(t)
	endSeq, ended := qaWatchAppend(t, c, &vttv1.SessionEnded{})
	startSeq, started := qaWatchAppend(t, c, &vttv1.SessionStarted{Name: "second"})
	sessions := c.State().Sessions
	open := sessions[len(sessions)-1]
	for _, id := range qaWatchActive(t, c) {
		log := qaWatchLog(t, c, id)
		var gotEnd, gotStart *vttv1.Envelope
		for _, e := range qaWatchAt(log, endSeq) {
			if e.Env.GetSessionEnded() != nil {
				gotEnd = e.Env
			}
		}
		for _, e := range qaWatchAt(log, startSeq) {
			if e.Env.GetSessionStarted() != nil {
				gotStart = e.Env
			}
		}
		if gotEnd == nil {
			t.Errorf("%s's log was not written the session's end", id)
		} else if gotEnd.GetEventId() != ended[0].GetEventId() {
			t.Errorf("%s's SessionEnded has event_id %q, want %q", id, gotEnd.GetEventId(), ended[0].GetEventId())
		}
		if gotStart == nil {
			t.Errorf("%s's log was not written the session's start", id)
			continue
		}
		if gotStart.GetSessionStarted().GetName() != "second" {
			t.Errorf("%s's SessionStarted names %q", id, gotStart.GetSessionStarted().GetName())
		}
		if gotStart.GetEventId() != started[0].GetEventId() || gotStart.GetSessionId() != open.ID {
			t.Errorf("%s's SessionStarted carries event %q session %q, want %q %q", id,
				gotStart.GetEventId(), gotStart.GetSessionId(), started[0].GetEventId(), open.ID)
		}
		st := qaWatchFold(t, log)
		last := st.Sessions[len(st.Sessions)-1]
		if last.Name != "second" || last.EndSeq != 0 {
			t.Errorf("%s's log folds to last session %+v, want %q open", id, last, "second")
		}
	}
	rogue := qaWatchLog(t, c, "rogue")
	if len(qaWatchAt(rogue, endSeq))+len(qaWatchAt(rogue, startSeq)) != 0 {
		t.Errorf("the paused rogue log was written the session")
	}
}

// VTT-364 VTT-316 VTT-371
func TestQAWatchPublicNoteReachesEveryLog(t *testing.T) {
	c := qaWatchTable(t)
	versions := []*vttv1.NoteUpserted{
		{Key: "map", Title: "Map", Text: "east wing", Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC},
		{Key: "map", Title: "Map", Text: "west wing", Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC},
	}
	for _, v := range versions {
		seq, envs := qaWatchAppend(t, c, v)
		for _, id := range qaWatchActive(t, c) {
			log := qaWatchLog(t, c, id)
			var got *vttv1.Envelope
			for _, e := range qaWatchAt(log, seq) {
				if e.Env.GetNoteUpserted() != nil {
					got = e.Env
				}
			}
			if got == nil {
				t.Errorf("%q: %s's log was not written the public note", v.GetText(), id)
				continue
			}
			if !proto.Equal(got.GetNoteUpserted(), v) || got.GetEventId() != envs[0].GetEventId() {
				t.Errorf("%q: %s's log was written %v", v.GetText(), id, got)
			}
			note, ok := qaWatchFold(t, log).Notes["map"]
			if !ok || note.Text != v.GetText() {
				t.Errorf("%q: %s's log folds to note %+v", v.GetText(), id, note)
			}
		}
		if len(qaWatchAt(qaWatchLog(t, c, "rogue"), seq)) != 0 {
			t.Errorf("%q: the paused rogue log was written the note", v.GetText())
		}
	}
}

func qaWatchOwnEvent(t *testing.T, c *campaign.Campaign, id, when string, p proto.Message) {
	t.Helper()
	seq, envs := qaWatchAppend(t, c, p)
	for _, e := range qaWatchAt(qaWatchLog(t, c, id), seq) {
		if e.Env.GetEventId() == envs[0].GetEventId() && proto.Equal(qaWatchBare(e.Env), qaWatchBare(envs[0])) {
			return
		}
	}
	t.Errorf("%s: %s's own log was not written %T about itself", when, id, p)
}

// VTT-365 VTT-371
func TestQAWatchOwnCharacterCountsAsSeen(t *testing.T) {
	c := qaWatchTable(t)
	scoutSeq, _ := qaWatchAppend(t, c, &vttv1.ResourceChanged{ActorId: "scout", Resource: "focus", Delta: -1, NewValue: 4})
	for _, id := range []string{"hero", "mage"} {
		if len(qaWatchAt(qaWatchLog(t, c, id), scoutSeq)) != 0 {
			t.Fatalf("fixture: %s's log is written an unseen member's status", id)
		}
	}
	qaWatchOwnEvent(t, c, "mage", "no token", &vttv1.ResourceChanged{
		ActorId: "mage", Resource: "focus", Delta: -2, NewValue: 3,
	})
	qaWatchOwnEvent(t, c, "mage", "no token", &vttv1.ConditionApplied{ActorId: "mage", ConditionId: "marked"})
	if r := qaWatchFold(t, qaWatchLog(t, c, "mage")).Actors["mage"].GetResources()["focus"]; r.GetCurrent() != 3 {
		t.Errorf("mage's log folds its own focus to %v, want 3", r)
	}
	qaWatchOwnEvent(t, c, "hero", "token on a seen square", &vttv1.ResourceChanged{
		ActorId: "hero", Resource: "focus", Delta: -1, NewValue: 4,
	})
	qaWatchAppend(t, c, &vttv1.TokenRemoved{TokenId: "t-hero"})
	qaWatchOwnEvent(t, c, "hero", "token removed", &vttv1.ResourceChanged{
		ActorId: "hero", Resource: "focus", Delta: -1, NewValue: 3,
	})
	qaWatchOwnEvent(t, c, "hero", "token removed", &vttv1.ConditionApplied{ActorId: "hero", ConditionId: "marked"})
	qaWatchAppend(t, c, qaWatchActor("bard", qaWatchParty))
	qaWatchOwnEvent(t, c, "bard", "log opened with no token", &vttv1.ResourceChanged{
		ActorId: "bard", Resource: "focus", Delta: -4, NewValue: 1,
	})
}

func qaWatchHoldsAdventureLoad(log []campaign.CharacterEntry) bool {
	for _, e := range log {
		if e.Env.GetAdventureLoaded() != nil {
			return true
		}
	}
	return false
}

// VTT-219
func TestQAWatchAdventureLoadReachesNoLog(t *testing.T) {
	c := qaWatchTable(t)
	qaWatchAppend(t, c, &vttv1.AdventureLoaded{AdventureId: "crypt", Name: "Crypt"})
	qaWatchAppend(t, c,
		&vttv1.AdventureLoaded{AdventureId: "tower", Name: "Tower"},
		&vttv1.SceneCreated{SceneId: "tower", Name: "tower", GridWidth: 2, GridHeight: 2},
		qaWatchActor("bard", qaWatchParty),
		&vttv1.NarrationAdded{Text: "the tower opens"},
	)
	active := qaWatchActive(t, c)
	if strings.Join(active, ",") != "bard,hero,mage,scout" {
		t.Fatalf("fixture: active logs are %v", active)
	}
	for _, id := range append(active, "rogue") {
		log := qaWatchLog(t, c, id)
		if qaWatchHoldsAdventureLoad(log) {
			t.Errorf("%s's log was written an adventure load", id)
		}
		if id != "rogue" && len(qaWatchNarrations(log, "the tower opens")) != 1 {
			t.Errorf("fixture: %s's log missed the batch's narration", id)
		}
	}
}

// VTT-239
func TestQAWatchNoteUnpublishedAndDeletedAreOneFrame(t *testing.T) {
	c := qaWatchTable(t)
	public := func(text string) *vttv1.NoteUpserted {
		return &vttv1.NoteUpserted{Key: "door", Title: "Door", Text: text, Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC}
	}
	qaWatchAppend(t, c, public("it is red"))
	secretSeq, _ := qaWatchAppend(t, c, &vttv1.NoteUpserted{
		Key: "door", Title: "Door", Text: "it is a trap", Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
	})
	qaWatchAppend(t, c, public("it is blue"))
	deleteSeq, _ := qaWatchAppend(t, c, &vttv1.NoteDeleted{Key: "door"})
	for _, id := range qaWatchActive(t, c) {
		log := qaWatchLog(t, c, id)
		unpublished, deleted := qaWatchAt(log, secretSeq), qaWatchAt(log, deleteSeq)
		if len(unpublished) == 0 || len(deleted) == 0 {
			t.Errorf("%s: %d frames for the unpublishing, %d for the deletion; want both written",
				id, len(unpublished), len(deleted))
			continue
		}
		if len(unpublished) != len(deleted) {
			t.Errorf("%s: %d frames for the unpublishing, %d for the deletion", id, len(unpublished), len(deleted))
			continue
		}
		for i := range deleted {
			if !proto.Equal(qaWatchBare(unpublished[i].Env), qaWatchBare(deleted[i].Env)) {
				t.Errorf("%s frame %d: unpublishing wrote %v, deletion wrote %v",
					id, i, unpublished[i].Env, deleted[i].Env)
			}
		}
		for name, seq := range map[string]int64{"unpublishing": secretSeq, "deletion": deleteSeq} {
			if _, ok := qaWatchFold(t, qaWatchThrough(log, seq)).Notes["door"]; ok {
				t.Errorf("%s: the note is still in the log after its %s", id, name)
			}
		}
	}
}
