# Event Core Implementation Plan (sub-project 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the event core per the approved spec (`docs/superpowers/specs/2026-07-23-event-core-design.md`): append-only SQLite log, single-fold projection, subscriptions, compensating-marker undo, six additive contract events, and the semgrep/go-arch-lint guards — all green in `task check`.

**Architecture:** Three packages — `internal/store` (SQLite log + subscriptions, knows no game state), `internal/engine` (State + the single pure `Apply` fold, no I/O), `internal/campaign` (composition: Open=replay, Append=validate→persist→apply→notify, Undo=marker+rebuild). One fold function used by replay and live path makes divergence structurally impossible; a property test enforces it besides.

**Tech Stack:** Go 1.26, `modernc.org/sqlite` v1.54.0 (pure Go, pinned), existing contract pipeline (buf + pinned local plugins), semgrep + go-arch-lint (both already installed on this machine), Task, bun test (contract TS suite only — no new TS).

> **AMENDED 2026-08-30 — RETRACTION LEFT THE PLATFORM, SO EVERY UNDO/RETRACTION
> STEP BELOW DESCRIBES MACHINERY THAT NO LONGER EXISTS.** Patrik's ruling of
> 2026-08-30: a retraction exists to make something not have happened, and it
> cannot — the person already read the log. Sub-project 13
> (`docs/superpowers/specs/2026-08-30-retraction-leaves-design.md`) removed it in
> full: `RetractEvents` and `EventsRetracted` left the contract in `59542e1`,
> `campaign.Undo` and `retractedSet` in `133e896`, the gateway handler and its
> authorization row in `5396338`, `client/src/undo.ts` and `fold.ts`'s second
> pass in `d3e2f28`, and the harness's matching two-pass in `92f1284`.
> Affected here: the Goal's "compensating-marker undo", the Architecture line's
> `Undo=marker+rebuild`, Task 1's `EventsRetracted` message, envelope arm and
> `contract/testdata/retraction_envelope.json` fixture, Task 4's
> `TestEventsRetractedIsNoOpInline` arm of the fold, Task 5's whole `Undo` half —
> the API, `retractedSet`, the two-pass `rebuildLocked`, the no-nesting and
> already-retracted rules and `internal/campaign/undo_test.go` — and Task 7's
> retraction interleaving in both the exit scenario and the keystone property.
> The single fold, the store's sequence authority, the subscription contract, the
> poison rules and both enforcement gates all shipped and are live.
> This plan is left as the record of what was built at the time; it is not a
> description of the tree. `tools/check-no-retraction.py` is the gate that keeps
> it from coming back.

## Global Constraints

- Repo `/Users/patriklager/dev/vtt-platform`; branch `feat/event-core` from `main`.
- **Review-before-commit flow (Patrik-approved):** implementers do NOT commit; controller commits with `CLAUDE_REVIEW_DONE=1` after task review. Commit points below are markers.
- **Task 1 gate caveat (carry-forward):** `check:drift` is HEAD-relative, so it runs RED on Task 1's uncommitted regenerated files — that is correct behavior, not failure. Task 1's pre-commit verification is: `go vet` + `go test` + `bun test` + `task check:breaking` green, plus `git status` showing exactly the expected gen changes. The controller runs full `task check` immediately POST-COMMIT as the real gate. All other tasks: full `task check` green pre-commit as usual.
- Contract evolution is ADDITIVE ONLY (new messages, new oneof tags 12–17); `check:breaking` must stay green throughout.
- `internal/` code must never contain game-system vocabulary (the Task 6 semgrep list); write all code in Tasks 2–5 with this in mind — generic names only (scenes, actors, tokens, sessions, resources).
- New dependency pins: `modernc.org/sqlite v1.54.0` (exact; its transitive modernc deps land in go.sum — sanctioned). No other new deps.
- `contract-spike/` frozen; `tools/toolgen` untouched (no new commands — events are records, not LLM tools).
- Store rule: subscribers can never block appends (bounded buffer; overflow closes that subscriber).
- Engine rule: `AttackRolled` applies as a deliberate no-op (testimony, not state — rules meaning is sub-project 5).

---

### Task 1: Contract lifecycle events (additive evolution through the armed gates)

**Files:**
- Modify: `contract/vtt/v1/events.proto` (6 new messages + 6 oneof variants)
- Modify: `contract/gen/**` (regenerated), `contract/roundtrip_test.go`, `contract/events.test.ts`
- Create: `contract/testdata/scene_envelope.json`, `contract/testdata/retraction_envelope.json`

**Interfaces:**
- Produces: `vttv1.SceneCreated`, `ActorAdded`, `TokenPlaced`, `SessionStarted`, `SessionEnded`, `EventsRetracted` + envelope variants `Envelope_SceneCreated` … `Envelope_EventsRetracted`; TS `SceneCreatedSchema` etc. Tasks 2–7 depend on these exact names.

- [ ] **Step 1: Branch** — `git checkout -b feat/event-core main`

- [ ] **Step 2: Extend the schema.** Append to `contract/vtt/v1/events.proto` (after `Resource`, before `Envelope`):

```proto
message SceneCreated {
  string scene_id = 1;
  string name = 2;
  int32 grid_width = 3;
  int32 grid_height = 4;
}

message ActorAdded {
  Actor actor = 1;
}

message TokenPlaced {
  string token_id = 1;
  string scene_id = 2;
  string actor_id = 3;
  GridPosition position = 4;
}

message SessionStarted {
  string name = 1;
}

message SessionEnded {}

// Compensating undo marker (spec §6). Retraction events cannot themselves
// be retracted. Sequences follow wire convention 1 (int64 → JSON string).
message EventsRetracted {
  int64 from_sequence = 1;
  int64 to_sequence = 2;
  string reason = 3;
}
```

And extend the `Envelope.payload` oneof:

```proto
    SceneCreated scene_created = 12;
    ActorAdded actor_added = 13;
    TokenPlaced token_placed = 14;
    SessionStarted session_started = 15;
    SessionEnded session_ended = 16;
    EventsRetracted events_retracted = 17;
```

- [ ] **Step 3: Regenerate** — `task generate:contract`. Expected: gen files change; `task check:breaking` exits 0 (additive). Run `go vet ./... && go test ./... && bun test` — green.

- [ ] **Step 4: New envelope fixtures.** `contract/testdata/scene_envelope.json`:

```json
{
  "eventId": "01J9ZK7M2N3P4Q5R6S7T8U9V10",
  "sequence": "7",
  "occurredAt": "2026-07-23T20:00:00Z",
  "sessionId": "sess-happy-dragon",
  "actorRole": "agent",
  "sceneCreated": {
    "sceneId": "scn-goblin-warrens",
    "name": "Goblin Warrens",
    "gridWidth": 30,
    "gridHeight": 20
  }
}
```

`contract/testdata/retraction_envelope.json`:

```json
{
  "eventId": "01J9ZK7M2N3P4Q5R6S7T8U9V11",
  "sequence": "43",
  "occurredAt": "2026-07-23T21:30:00Z",
  "sessionId": "sess-happy-dragon",
  "actorRole": "dm",
  "eventsRetracted": {
    "fromSequence": "40",
    "toSequence": "42",
    "reason": "DM ruled the attack invalid"
  }
}
```

- [ ] **Step 5: Extend both round-trip suites.** In `contract/roundtrip_test.go` add:

```go
func TestSceneEnvelopeRoundTrip(t *testing.T) { roundTrip(t, "scene_envelope.json", &vttv1.Envelope{}) }
func TestRetractionEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "retraction_envelope.json", &vttv1.Envelope{})
}
```

In `contract/events.test.ts` add to `cases`:

```ts
  ["scene_envelope.json", EnvelopeSchema],
  ["retraction_envelope.json", EnvelopeSchema],
```

Run: `go test ./contract/ && bun test contract/` — expected 8/8 Go, 8/8 TS.

- [ ] **Step 6: Pre-commit verification (per the Task 1 gate caveat)** — `go vet ./...`, `go test ./...`, `bun test`, `task check:breaking` all green; `git status --porcelain` shows only `contract/vtt/v1/events.proto`, `contract/gen/**`, the two fixtures, and the two test files. (`task check:drift` will be red until commit — expected.)

- [ ] **Step 7: Commit point** (controller; then controller runs full `task check` post-commit — must be green)

```
feat: add lifecycle + retraction events to contract (additive, tags 12-17)
```

---

### Task 2: internal/store — the append-only log

**Files:**
- Create: `internal/store/store.go`, `internal/store/store_test.go`
- Modify: `go.mod`/`go.sum` (`modernc.org/sqlite v1.54.0`)

**Interfaces:**
- Produces: `store.Open(path) (*Store, error)`, `(*Store).Append(env *vttv1.Envelope) (int64, error)`, `(*Store).ReadAfter(afterSeq int64) ([]*vttv1.Envelope, error)`, `(*Store).Close() error`. Task 3 adds Subscribe to the same struct; Task 5 consumes all of it.

- [ ] **Step 1: Add the dependency** — `go get modernc.org/sqlite@v1.54.0`

- [ ] **Step 2: Write the failing tests** — `internal/store/store_test.go`:

```go
package store_test

import (
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

func newEnv(id string) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:   id,
		SessionId: "sess-1",
		ActorRole: "dm",
		Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "test"},
		},
	}
}

func openTemp(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "campaign.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAppendAssignsSequentialSequence(t *testing.T) {
	s := openTemp(t)
	for want := int64(1); want <= 3; want++ {
		env := newEnv(string(rune('a' + want)))
		got, err := s.Append(env)
		if err != nil {
			t.Fatal(err)
		}
		if got != want || env.Sequence != want {
			t.Fatalf("seq: got %d (env %d), want %d", got, env.Sequence, want)
		}
	}
}

func TestAppendRejectsNonZeroSequence(t *testing.T) {
	s := openTemp(t)
	env := newEnv("x")
	env.Sequence = 9
	if _, err := s.Append(env); err == nil {
		t.Fatal("want error for non-zero sequence")
	}
}

func TestAppendRejectsEmptyAndDuplicateEventID(t *testing.T) {
	s := openTemp(t)
	if _, err := s.Append(newEnv("")); err == nil {
		t.Fatal("want error for empty event_id")
	}
	if _, err := s.Append(newEnv("dup")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(newEnv("dup")); err == nil {
		t.Fatal("want error for duplicate event_id")
	}
}

func TestReadAfterAndPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first := newEnv("e1")
	if _, err := s.Append(first); err != nil {
		t.Fatal(err)
	}
	second := newEnv("e2")
	if _, err := s.Append(second); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, err := s2.ReadAfter(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !proto.Equal(got[0], second) {
		t.Fatalf("ReadAfter(1): got %v", got)
	}
	all, _ := s2.ReadAfter(0)
	if len(all) != 2 {
		t.Fatalf("ReadAfter(0): got %d events", len(all))
	}
}
```

Run: `go test ./internal/store/` — expected FAIL (package does not exist).

- [ ] **Step 3: Implement** — `internal/store/store.go`:

```go
// Package store is the append-only event log. It persists protobuf-binary
// vtt.v1 Envelopes in SQLite, assigns the authoritative sequence, and knows
// nothing about game state (spec §4).
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

const schema = `
CREATE TABLE IF NOT EXISTS events (
  seq         INTEGER PRIMARY KEY,
  event_id    TEXT NOT NULL UNIQUE,
  session_id  TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  payload     BLOB NOT NULL
);`

type Store struct {
	mu sync.Mutex
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: init schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Append stamps the next sequence into env, persists it, and returns the
// sequence. Callers must submit sequence=0; event_id must be non-empty and
// unique (idempotency guard).
func (s *Store) Append(env *vttv1.Envelope) (int64, error) {
	if env.Sequence != 0 {
		return 0, errors.New("store: envelope sequence must be 0 on append")
	}
	if env.EventId == "" {
		return 0, errors.New("store: envelope event_id must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var next int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM events`).Scan(&next); err != nil {
		return 0, err
	}
	env.Sequence = next
	blob, err := proto.Marshal(env)
	if err != nil {
		env.Sequence = 0
		return 0, fmt.Errorf("store: marshal: %w", err)
	}
	occurredAt := ""
	if ts := env.OccurredAt; ts != nil {
		occurredAt = ts.AsTime().UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if _, err := tx.Exec(
		`INSERT INTO events (seq, event_id, session_id, occurred_at, payload) VALUES (?, ?, ?, ?, ?)`,
		next, env.EventId, env.SessionId, occurredAt, blob,
	); err != nil {
		env.Sequence = 0
		return 0, fmt.Errorf("store: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		env.Sequence = 0
		return 0, err
	}
	return next, nil
}

// ReadAfter returns all events with seq > afterSeq, in order.
func (s *Store) ReadAfter(afterSeq int64) ([]*vttv1.Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAfterLocked(afterSeq)
}

func (s *Store) readAfterLocked(afterSeq int64) ([]*vttv1.Envelope, error) {
	rows, err := s.db.Query(`SELECT payload FROM events WHERE seq > ? ORDER BY seq`, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*vttv1.Envelope
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		env := &vttv1.Envelope{}
		if err := proto.Unmarshal(blob, env); err != nil {
			return nil, fmt.Errorf("store: corrupt event payload: %w", err)
		}
		out = append(out, env)
	}
	return out, rows.Err()
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}
```

- [ ] **Step 4: Run tests** — `go test ./internal/store/` — expected PASS (4/4). Then `task check` — green.

- [ ] **Step 5: Commit point** — `feat: append-only SQLite event store with authoritative sequencing`

---

### Task 3: internal/store — subscriptions

**Files:**
- Create: `internal/store/subscribe.go`, `internal/store/subscribe_test.go`
- Modify: `internal/store/store.go` (Append gains a notify call; Close closes subscribers)

**Interfaces:**
- Produces: `(*Store).Subscribe(afterSeq int64, buffer int) (<-chan *vttv1.Envelope, func(), error)` — catch-up (events > afterSeq) then live; the returned func unsubscribes. Overflowed subscribers get their channel CLOSED (spec §4: log is the recovery path).

- [ ] **Step 1: Write the failing tests** — `internal/store/subscribe_test.go`:

```go
package store_test

import (
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func recv(t *testing.T, ch <-chan *vttv1.Envelope) *vttv1.Envelope {
	t.Helper()
	select {
	case env, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		return env
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
		return nil
	}
}

func TestSubscribeCatchUpThenLive(t *testing.T) {
	s := openTemp(t)
	s.Append(newEnv("e1"))
	s.Append(newEnv("e2"))

	ch, cancel, err := s.Subscribe(1, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if got := recv(t, ch); got.EventId != "e2" {
		t.Fatalf("catch-up: got %s, want e2", got.EventId)
	}
	s.Append(newEnv("e3"))
	if got := recv(t, ch); got.EventId != "e3" {
		t.Fatalf("live: got %s, want e3", got.EventId)
	}
}

func TestSubscribeOverflowClosesThatSubscriberOnly(t *testing.T) {
	s := openTemp(t)
	small, cancelSmall, _ := s.Subscribe(0, 1)
	defer cancelSmall()
	big, cancelBig, _ := s.Subscribe(0, 16)
	defer cancelBig()

	for i := 0; i < 4; i++ {
		s.Append(newEnv(string(rune('a' + i))))
	}
	// small (cap 1, never drained) must end CLOSED; drain to find closure.
	deadline := time.After(2 * time.Second)
	closed := false
	for !closed {
		select {
		case _, ok := <-small:
			if !ok {
				closed = true
			}
		case <-deadline:
			t.Fatal("small subscriber never closed on overflow")
		}
	}
	// big subscriber unaffected: sees all 4.
	for i := 0; i < 4; i++ {
		recv(t, big)
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	s := openTemp(t)
	ch, cancel, _ := s.Subscribe(0, 4)
	cancel()
	s.Append(newEnv("after"))
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("received event after unsubscribe")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("channel not closed after unsubscribe")
	}
}
```

Run: `go test ./internal/store/ -run Subscribe` — expected FAIL (`Subscribe` undefined).

- [ ] **Step 2: Implement** — `internal/store/subscribe.go`:

```go
package store

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

type subscriber struct {
	ch     chan *vttv1.Envelope
	closed bool
}

// Subscribe delivers every event with seq > afterSeq: history first (loaded
// atomically under the store lock, so no gap or duplicate versus live
// appends), then live events. buffer bounds the channel beyond the catch-up
// batch; a subscriber that falls behind has its channel closed — the log is
// the recovery path, and no subscriber may block appends.
func (s *Store) Subscribe(afterSeq int64, buffer int) (<-chan *vttv1.Envelope, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	history, err := s.readAfterLocked(afterSeq)
	if err != nil {
		return nil, nil, err
	}
	sub := &subscriber{ch: make(chan *vttv1.Envelope, len(history)+buffer)}
	for _, env := range history {
		sub.ch <- env // fits by construction
	}
	s.subs = append(s.subs, sub)

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.dropLocked(sub)
	}
	return sub.ch, cancel, nil
}

// notifyLocked delivers env to all live subscribers; callers hold s.mu.
func (s *Store) notifyLocked(env *vttv1.Envelope) {
	for _, sub := range s.subs {
		if sub.closed {
			continue
		}
		select {
		case sub.ch <- env:
		default: // overflow: close, drop
			s.dropLocked(sub)
		}
	}
	// compact dropped subscribers
	live := s.subs[:0]
	for _, sub := range s.subs {
		if !sub.closed {
			live = append(live, sub)
		}
	}
	s.subs = live
}

func (s *Store) dropLocked(sub *subscriber) {
	if !sub.closed {
		sub.closed = true
		close(sub.ch)
	}
}
```

And in `store.go`: add `subs []*subscriber` to the `Store` struct; in `Append`, after `tx.Commit()` succeeds, call `s.notifyLocked(env)`; in `Close`, before `db.Close()`, drop all subscribers (`for _, sub := range s.subs { s.dropLocked(sub) }; s.subs = nil`).

- [ ] **Step 3: Run tests** — `go test ./internal/store/` — expected PASS (7/7: 4 prior + 3 new). Then `go test ./internal/store/ -race` once (subscriptions are concurrency code). Then `task check` — green.

- [ ] **Step 4: Commit point** — `feat: store subscriptions with atomic catch-up and overflow-close`

---

### Task 4: internal/engine — State and the single fold

**Files:**
- Create: `internal/engine/state.go`, `internal/engine/apply.go`, `internal/engine/apply_test.go`

**Interfaces:**
- Produces: `engine.NewState() *State`; `engine.Apply(st *State, env *vttv1.Envelope) error` (the ONLY state mutator in the codebase — arch-lint guards this by package); `(*State).Snapshot() *State` (deep copy); `engine.ErrUnknownVariant`. Types: `State{Scenes map[string]Scene, Actors map[string]*vttv1.Actor, Tokens map[string]Token, Sessions []Session}`, `Scene{ID, Name string; GridWidth, GridHeight int32}`, `Token{ID, SceneID, ActorID string; X, Y int32}`, `Session{ID, Name string; StartSeq, EndSeq int64}` (EndSeq 0 = open). Task 5 and 7 consume all of this.

- [ ] **Step 1: Write the failing tests** — `internal/engine/apply_test.go`. Table-driven; a helper builds envelopes. Cover, at minimum (each its own test function, complete in the file):

```go
package engine_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func env(seq int64, payload any) *vttv1.Envelope {
	e := &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1"}
	switch p := payload.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: p}
	case *vttv1.SessionEnded:
		e.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: p}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.TokenMoved:
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: p}
	case *vttv1.AttackRolled:
		e.Payload = &vttv1.Envelope_AttackRolled{AttackRolled: p}
	case *vttv1.EventsRetracted:
		e.Payload = &vttv1.Envelope_EventsRetracted{EventsRetracted: p}
	}
	return e
}

// seedScene applies SessionStarted + SceneCreated and returns the state.
func seedScene(t *testing.T) *engine.State {
	t.Helper()
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10})))
	return st
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
```

Test functions (write each fully):
- `TestSceneActorTokenLifecycle` — seed; add actor (`ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"}}`); place token (`TokenPlaced{TokenId: "t1", SceneId: "scn", ActorId: "a1", Position: &vttv1.GridPosition{X: 3, Y: 7}}`); move it (`TokenMoved{TokenId: "t1", SceneId: "scn", From: ..., To: &vttv1.GridPosition{X: 5, Y: 8}}`); assert `st.Tokens["t1"].X == 5 && Y == 8`, scene and actor present, session open.
- `TestApplyRejections` — duplicate scene id; TokenPlaced into unknown scene; TokenPlaced with unknown actor; TokenMoved for unknown token; duplicate actor id; duplicate token id; SessionEnded with no open session; second SessionStarted while one is open. Each returns a non-nil error AND leaves state unchanged (compare against a pre-call `Snapshot()` via `reflect.DeepEqual` on exported fields — assert on `len(st.Tokens)` etc.).
- `TestAttackRolledIsDeliberateNoOp` — apply an `AttackRolled` on a seeded state; assert state deep-equals its prior snapshot.
- `TestEventsRetractedIsNoOpInline` — apply returns nil, state unchanged (rebuild happens in campaign).
- `TestUnknownVariantError` — `engine.Apply(st, &vttv1.Envelope{EventId: "u", Sequence: 9})` (nil payload) returns `engine.ErrUnknownVariant` (use `errors.Is`).
- `TestSnapshotIsDeepCopy` — snapshot; mutate the ORIGINAL (move token, add attribute to an actor's map via `st.Actors["a1"].Attributes["x"]=1`); assert snapshot unchanged (its actor map lacks "x", its token still at old position).

Run: `go test ./internal/engine/` — expected FAIL (package does not exist).

- [ ] **Step 2: Implement state.go**

```go
// Package engine holds the derived game state and the single fold that
// advances it. Apply is the only state mutator in the codebase (spec §3);
// the package does no I/O and imports only the contract.
package engine

import (
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

type Scene struct {
	ID, Name               string
	GridWidth, GridHeight  int32
}

type Token struct {
	ID, SceneID, ActorID string
	X, Y                 int32
}

type Session struct {
	ID, Name          string
	StartSeq, EndSeq  int64 // EndSeq 0 = open
}

type State struct {
	Scenes   map[string]Scene
	Actors   map[string]*vttv1.Actor
	Tokens   map[string]Token
	Sessions []Session
}

func NewState() *State {
	return &State{
		Scenes: map[string]Scene{},
		Actors: map[string]*vttv1.Actor{},
		Tokens: map[string]Token{},
	}
}

// Snapshot returns a deep copy; readers never alias live state (spec §5).
func (st *State) Snapshot() *State {
	out := NewState()
	for k, v := range st.Scenes {
		out.Scenes[k] = v
	}
	for k, v := range st.Actors {
		out.Actors[k] = proto.Clone(v).(*vttv1.Actor)
	}
	for k, v := range st.Tokens {
		out.Tokens[k] = v
	}
	out.Sessions = append([]Session(nil), st.Sessions...)
	return out
}

// openSession returns the index of the open session, or -1.
func (st *State) openSession() int {
	for i := range st.Sessions {
		if st.Sessions[i].EndSeq == 0 {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 3: Implement apply.go**

```go
package engine

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

var ErrUnknownVariant = errors.New("engine: unknown event variant")

// Apply advances st by one event. It validates BEFORE mutating: any error
// return leaves st unchanged. AttackRolled and EventsRetracted are deliberate
// no-ops here (spec §5).
func Apply(st *State, env *vttv1.Envelope) error {
	switch p := env.Payload.(type) {
	case *vttv1.Envelope_SessionStarted:
		if st.openSession() >= 0 {
			return fmt.Errorf("engine: session already open")
		}
		st.Sessions = append(st.Sessions, Session{
			ID: env.SessionId, Name: p.SessionStarted.Name, StartSeq: env.Sequence,
		})
		return nil

	case *vttv1.Envelope_SessionEnded:
		i := st.openSession()
		if i < 0 {
			return fmt.Errorf("engine: no open session to end")
		}
		st.Sessions[i].EndSeq = env.Sequence
		return nil

	case *vttv1.Envelope_SceneCreated:
		sc := p.SceneCreated
		if _, dup := st.Scenes[sc.SceneId]; dup {
			return fmt.Errorf("engine: scene %q already exists", sc.SceneId)
		}
		st.Scenes[sc.SceneId] = Scene{
			ID: sc.SceneId, Name: sc.Name,
			GridWidth: sc.GridWidth, GridHeight: sc.GridHeight,
		}
		return nil

	case *vttv1.Envelope_ActorAdded:
		a := p.ActorAdded.Actor
		if a == nil || a.ActorId == "" {
			return fmt.Errorf("engine: actor_added requires an actor with an id")
		}
		if _, dup := st.Actors[a.ActorId]; dup {
			return fmt.Errorf("engine: actor %q already exists", a.ActorId)
		}
		st.Actors[a.ActorId] = proto.Clone(a).(*vttv1.Actor)
		return nil

	case *vttv1.Envelope_TokenPlaced:
		tp := p.TokenPlaced
		if _, dup := st.Tokens[tp.TokenId]; dup {
			return fmt.Errorf("engine: token %q already exists", tp.TokenId)
		}
		if _, ok := st.Scenes[tp.SceneId]; !ok {
			return fmt.Errorf("engine: token placed in unknown scene %q", tp.SceneId)
		}
		if _, ok := st.Actors[tp.ActorId]; !ok {
			return fmt.Errorf("engine: token placed for unknown actor %q", tp.ActorId)
		}
		if tp.Position == nil {
			return fmt.Errorf("engine: token placed without position")
		}
		st.Tokens[tp.TokenId] = Token{
			ID: tp.TokenId, SceneID: tp.SceneId, ActorID: tp.ActorId,
			X: tp.Position.X, Y: tp.Position.Y,
		}
		return nil

	case *vttv1.Envelope_TokenMoved:
		tm := p.TokenMoved
		tok, ok := st.Tokens[tm.TokenId]
		if !ok {
			return fmt.Errorf("engine: moved unknown token %q", tm.TokenId)
		}
		if tm.To == nil {
			return fmt.Errorf("engine: token move without destination")
		}
		tok.X, tok.Y = tm.To.X, tm.To.Y
		st.Tokens[tm.TokenId] = tok
		return nil

	case *vttv1.Envelope_AttackRolled:
		return nil // testimony, not state — rules meaning arrives in sub-project 5

	case *vttv1.Envelope_EventsRetracted:
		return nil // handled by campaign rebuild, not in-line

	default:
		return fmt.Errorf("%w: %T", ErrUnknownVariant, env.Payload)
	}
}
```

- [ ] **Step 4: Run tests** — `go test ./internal/engine/` — expected PASS. Then `task check` — green.

- [ ] **Step 5: Commit point** — `feat: engine state and the single Apply fold`

---

### Task 5: internal/campaign — composition, replay, undo

**Files:**
- Create: `internal/campaign/campaign.go`, `internal/campaign/campaign_test.go`, `internal/campaign/undo_test.go`

**Interfaces:**
- Produces: `campaign.Open(path) (*Campaign, error)`; `(*Campaign).Append(env) (int64, error)` (validate→persist→apply; rejects `EventsRetracted` — use Undo); `(*Campaign).Undo(from, to int64, reason, eventID, sessionID string) error`; `(*Campaign).State() *engine.State` (snapshot); `(*Campaign).Subscribe(afterSeq int64, buffer int)` (delegates to store); `(*Campaign).Close() error`. Task 7 consumes all of it.

- [ ] **Step 1: Write the failing tests.** `campaign_test.go` covers: open→append lifecycle events→state correct; append validation failure (TokenMoved for unknown token) persists NOTHING (ReadAfter length unchanged); append of an `EventsRetracted` envelope rejected with "use Undo"; close→reopen→state deep-equals; subscriber sees appended events. `undo_test.go` covers: retract a move → token back at prior position; retracting a range containing an `EventsRetracted` rejected (no-nesting); retracting an already-retracted range rejected; out-of-range rejected; subscriber receives the EventsRetracted event itself. Write complete test code following Task 4's helper style (build envelopes with `EventId` set — campaign requires it; e.g. helper `func cenv(id string, payload any) *vttv1.Envelope`).

Run: `go test ./internal/campaign/` — expected FAIL (package does not exist).

- [ ] **Step 2: Implement** — `internal/campaign/campaign.go`:

```go
// Package campaign composes the store and the engine: replay on open, the
// validate→persist→apply append path, and compensating-marker undo (spec §3,
// §6, §7). It is the only package that imports both store and engine.
package campaign

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

type Campaign struct {
	mu    sync.Mutex
	log   *store.Store
	state *engine.State
}

func Open(path string) (*Campaign, error) {
	s, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	c := &Campaign{log: s}
	if err := c.rebuildLocked(); err != nil {
		s.Close()
		return nil, err
	}
	return c, nil
}

// rebuildLocked derives state from the full log: pass 1 collects retracted
// ranges, pass 2 folds the non-retracted events. Unknown variants in an
// existing log are skipped with a warning (forward compatibility, spec §7);
// any other apply error is a corrupt log and fails loudly.
func (c *Campaign) rebuildLocked() error {
	events, err := c.log.ReadAfter(0)
	if err != nil {
		return err
	}
	retracted := retractedSet(events)
	st := engine.NewState()
	for _, env := range events {
		if retracted[env.Sequence] {
			continue
		}
		if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
			continue
		}
		if err := engine.Apply(st, env); err != nil {
			if errors.Is(err, engine.ErrUnknownVariant) {
				slog.Warn("campaign: skipping unknown event variant during replay",
					"sequence", env.Sequence, "event_id", env.EventId)
				continue
			}
			return fmt.Errorf("campaign: corrupt log at seq %d: %w", env.Sequence, err)
		}
	}
	c.state = st
	return nil
}

func retractedSet(events []*vttv1.Envelope) map[int64]bool {
	out := map[int64]bool{}
	for _, env := range events {
		if r, ok := env.Payload.(*vttv1.Envelope_EventsRetracted); ok {
			for seq := r.EventsRetracted.FromSequence; seq <= r.EventsRetracted.ToSequence; seq++ {
				out[seq] = true
			}
		}
	}
	return out
}

// Append validates against the projection, persists (the commit point), then
// advances the live projection. Any validation error writes nothing (spec §7).
func (c *Campaign) Append(env *vttv1.Envelope) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
		return 0, errors.New("campaign: EventsRetracted must be appended via Undo")
	}
	if env.Payload == nil {
		return 0, engine.ErrUnknownVariant
	}
	// Validate on a snapshot so a rejection cannot half-mutate live state.
	if err := engine.Apply(c.state.Snapshot(), env); err != nil {
		return 0, err
	}
	seq, err := c.log.Append(env) // persists AND notifies subscribers
	if err != nil {
		return 0, err
	}
	if err := engine.Apply(c.state, env); err != nil {
		// Snapshot-validated, so this is unreachable; fail loudly if not.
		return 0, fmt.Errorf("campaign: live apply diverged from validation: %w", err)
	}
	return seq, nil
}

// Undo appends an EventsRetracted marker and rebuilds the projection.
// The range must exist, contain no retraction markers (no nesting), and not
// overlap an already-retracted span (spec §6).
func (c *Campaign) Undo(from, to int64, reason string, eventID, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if from < 1 || to < from {
		return fmt.Errorf("campaign: invalid retraction range [%d,%d]", from, to)
	}
	events, err := c.log.ReadAfter(0)
	if err != nil {
		return err
	}
	var maxSeq int64
	if n := len(events); n > 0 {
		maxSeq = events[n-1].Sequence
	}
	if to > maxSeq {
		return fmt.Errorf("campaign: retraction range end %d beyond log head %d", to, maxSeq)
	}
	already := retractedSet(events)
	for _, env := range events {
		if env.Sequence < from || env.Sequence > to {
			continue
		}
		if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
			return fmt.Errorf("campaign: cannot retract a retraction (seq %d)", env.Sequence)
		}
		if already[env.Sequence] {
			return fmt.Errorf("campaign: seq %d is already retracted", env.Sequence)
		}
	}
	marker := &vttv1.Envelope{
		EventId:   eventID,
		SessionId: sessionID,
		ActorRole: "dm",
		Payload: &vttv1.Envelope_EventsRetracted{
			EventsRetracted: &vttv1.EventsRetracted{
				FromSequence: from, ToSequence: to, Reason: reason,
			},
		},
	}
	if _, err := c.log.Append(marker); err != nil {
		return err
	}
	return c.rebuildLocked()
}

func (c *Campaign) State() *engine.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state.Snapshot()
}

func (c *Campaign) Subscribe(afterSeq int64, buffer int) (<-chan *vttv1.Envelope, func(), error) {
	return c.log.Subscribe(afterSeq, buffer)
}

func (c *Campaign) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.log.Close()
}
```

- [ ] **Step 3: Run tests** — `go test ./internal/campaign/` — expected PASS. Then `go test ./... && task check` — green.

- [ ] **Step 4: Commit point** — `feat: campaign composition — replay, validated append, compensating undo`

---

### Task 6: Enforcement — semgrep vocabulary ban + go-arch-lint layers

**Files:**
- Create: `.semgrep/vocabulary.yml`, `.go-arch-lint.yml`
- Modify: `Taskfile.yml` (`check:vocabulary`, `check:arch`, folded into `check`)

**Interfaces:**
- Produces: two new gates in `task check`. Both tools are already installed on this machine (`semgrep` 
  at ~/Library/Python/3.9/bin, `go-arch-lint` at ~/go/bin) — verify with `--version`/`version` first and record versions in the report; if either is missing, BLOCKED (do not improvise substitutes).

- [ ] **Step 1: Write `.semgrep/vocabulary.yml`** — pillar P2/P4 made mechanical (spec §3):

```yaml
rules:
  - id: no-game-system-vocabulary-in-engine
    message: >
      Game-system vocabulary is forbidden in platform code (pillars P2/P4,
      ADR-002/004). Rules concepts live in rule-module DATA, never engine code.
    severity: ERROR
    languages: [go]
    paths:
      include: [internal/]
    patterns:
      - pattern-regex: (?i)(healing_?surge|daily_?power|encounter_?power|at_?will_?power|fortitude|bloodied|saving_?throw|action_?point|hit_?points)
```

- [ ] **Step 2: Write `.go-arch-lint.yml`** (spec §3 layer rules):

```yaml
version: 3
workdir: .
allow:
  depOnAnyVendor: true
components:
  contract: { in: contract/gen/go/** }
  store:    { in: internal/store }
  engine:   { in: internal/engine }
  campaign: { in: internal/campaign }
  toolgen:  { in: tools/toolgen }
deps:
  store:    { mayDependOn: [contract] }
  engine:   { mayDependOn: [contract] }
  campaign: { mayDependOn: [contract, store, engine] }
  toolgen:  { mayDependOn: [contract] }
```

- [ ] **Step 3: Wire into Taskfile** — add:

```yaml
  check:vocabulary:
    desc: Fail on game-system vocabulary in platform code (P2/P4)
    cmds:
      - semgrep scan --config .semgrep/vocabulary.yml --error --quiet internal/

  check:arch:
    desc: Enforce package layering (store/engine/campaign)
    cmds:
      - go-arch-lint check
```

and append both to `check:` cmds (after `check:breaking`).

- [ ] **Step 4: Prove both gates bite, then restore.** (a) Add `const healingSurge = 1` to a temp file in `internal/engine/`, run `task check:vocabulary` → expect non-zero with the file named; delete the temp file, rerun → green. (b) Add a temp file in `internal/engine/` importing `github.com/PatrikLager/vtt-platform/internal/store` (`var _ = store.Open`), run `task check:arch` → expect non-zero naming the illegal dep; delete, rerun → green. Both transcripts go in the report verbatim.

- [ ] **Step 5: Run full `task check`** — green (now seven gates). Record semgrep and go-arch-lint versions in the report.

- [ ] **Step 6: Commit point** — `feat: vocabulary and architecture gates join task check`

---

### Task 7: Keystone property test + exit scenario

**Files:**
- Create: `internal/campaign/property_test.go`, `internal/campaign/scenario_test.go`

**Interfaces:**
- Consumes: everything. Produces: the spec §9 exit criteria, executable.

- [ ] **Step 1: Write `scenario_test.go`** — the spec's exit scenario, exactly (seed of sub-project 4's harness): open campaign in t.TempDir → subscribe → SessionStarted → SceneCreated → two ActorAdded → two TokenPlaced → three TokenMoved → Undo the middle move (assert token position reverts to the pre-retracted-move position and subscriber received the EventsRetracted event) → SessionEnded → Close → reopen with `campaign.Open(same path)` → assert `reflect.DeepEqual`-style equality of `State()` against the pre-close snapshot (compare via `google.golang.org/protobuf/proto.Equal` for actors and plain equality elsewhere — write a `statesEqual(a, b *engine.State) bool` helper, complete in the file). Write the full test.

- [ ] **Step 2: Write `property_test.go`** — the keystone (spec §9): deterministic `math/rand` (seed 1). Generator loop, N=400 iterations: at each step pick a random VALID action given current model (create scene ~5%, add actor ~10%, place token ~15% when scene+actor exist, move token ~55% when tokens exist, undo a random valid non-marker un-retracted range ~10% when possible, start/end session ~5% keeping exactly-one-open invariant simple: start if none open, end if open and rand<0.05). After every 50 events: `live := c.State()`; reopen a SECOND handle? No — single-writer: instead call the rebuild path by `c2, _ := campaign.Open(path)`? SQLite file locked by open handle — use `c.Close()` + reopen + compare + continue with reopened campaign:

```go
snapshot := c.State()
c.Close()
c, err = campaign.Open(path)
// assert statesEqual(snapshot, c.State())
```

This IS the rebuild==live proof (live projection vs full-replay reconstruction). Write the complete generator + assertions; on failure print the failing action index and seed.

- [ ] **Step 3: Run** — `go test ./internal/campaign/ -run 'Scenario|Property' -v` — PASS; then full `task check` — green; then `go test ./... -race` once (final concurrency sanity).

- [ ] **Step 4: Commit point** — `test: keystone rebuild-equals-live property and exit scenario`

---

## After this plan

Sub-project 2 complete: merge via finishing-a-development-branch. Carry-forwards for sub-project 3 (API gateway & permissions): campaign's Append/Subscribe are the surface the gateway wraps; the sync-vs-async Append question (spec §11) gets decided against real WebSocket flow; the `vtt` CLI binary shell (ckeletin scaffold candidacy) is decided there too.
