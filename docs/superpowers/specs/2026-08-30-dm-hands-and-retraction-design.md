# Hands on the board, and a seat that survives an undo

**Sub-project 12.** Three of twenty-two commands have no human sender, and one
undo can freeze a player permanently. Both are gaps the last two arcs recorded
on their way out rather than defects anyone discovered later.

---

## 1. Why this, and why now

maps-as-geometry merged with a gap stated in its own merge commit (`f94c529`):

> **The web client cannot send `load_map`, `open_door` or `close_door`.** It
> RENDERS terrain and OpenDoors faithfully; only the MCP agent seat can change
> them. A human DM at the browser cannot open a door. Patrik's call pending.

That call is made: **the console tracks the tool surface.** Anything the agent
can send, a human can send. The three missing commands are the gap to close, and
the arrangement that let the gap open — nothing anywhere says a command has no
human sender — is the thing worth fixing, because command twenty-three will
arrive the same way.

The second half is a defect the visibility arc measured and filed rather than
fixed, at `internal/gateway/project.go:419-431`:

> Folding one projected stream with and without a retraction of the revealing
> event: the player does forget the goblin — and if any LATER event about it was
> forwarded, their fold then fails on the dangling reference ("moved unknown
> token") where the DM's identical retraction folds cleanly [...] `campaign.Undo`
> dry-runs the would-be fold before persisting, but it dry-runs the LOG, so a
> retraction safe for every seat that receives the log can still be unsafe for
> one that receives a projection of it.

The end state is a permanently frozen player: `session.ts`'s `ingest` re-folds
the whole log on every event, so once the log cannot fold, every subsequent
event fails the same way and the board never recovers. Session one will involve
an undo. This arc closes it first.

**Why now.** Both arcs that produced these gaps are merged and green. Session one
is the next walk, and it is a poor test of the platform if the DM must ask a
language model to open every door and a single undo can end a player's evening.

---

## 2. Non-goals

**A general in-browser map editor.** `load_map` picks an existing map by id. The
authoring API is `docs/map-format.md` and stays a file format.

**Objects for the standard pack.** The eleven-tile, zero-object gap is real
(`f94c529`) and is content work, not this.

**Touch targets and small-screen layout.** Deferred by decision at session zero;
the laptop is the primary target.

**Changing what a retraction means.** Retraction stays a range over sequence
numbers, retracted for every seat. Only the delivery of one to a projected seat
changes.

**A per-viewer pre-flight on undo.** Considered and rejected in §5.2.

---

## 3. Parity: the console tracks the tool surface

### 3.1 What is missing, exactly

`cmd/vtt/tools.json` publishes twenty-two commands. `client/src/commands.ts`
exports a builder for nineteen of them. The three with no human sender are
`open_door`, `close_door` and `load_map` — precisely the three `f94c529`
recorded, three arcs ago.

A trap for whoever writes the controls: `view/dm.ts` already renders buttons
labelled **"Open the door"** and **"Close the door"**, carrying the action
strings `open-door` and `close-door`. Those are the **join door**
(`set_join_door`) — the admissions door for seating people at the table — and
have nothing to do with a door in a wall. The new controls must not reuse either
label or either action string.

### 3.2 The invariant, not the count

Closing three commands by hand leaves the same gap open for the twenty-third. So
the client declares, in one place, which surface issues each `ClientCommand`
case:

    dm-console | player-panel | board | spectator | join-flow | not-user-issued

A test walks the generated `ClientCommand` oneof and fails when a case is absent
from that table. A new contract arm therefore cannot land until someone has
decided where a human reaches it.

**`not-user-issued` requires a stated reason, and the table rejects a bare
entry** — the same discipline `tools/mutation-equivalents.txt` opens with, for
the same cause. An escape hatch that costs nothing to use is how the gap this
section closes would quietly reopen: the next command declares itself unreachable
and no one ever reads the line.

For cases declared `dm-console` or `player-panel`, the same test renders the
real panel and asserts a control exists carrying that command's declared action
string, using the `action` parameter `dm.ts`'s `button()` already takes. The
table cannot drift from the UI, because the test looks at the UI.

**This asserts a relationship and never a number.** No count of commands appears
in any assertion or comment; a count rots by addition, which is the failure this
whole section exists to prevent.

---

## 4. The two controls

### 4.1 A door is armed, then clicked

`OpenDoor` and `CloseDoor` carry `{scene_id, at}` — a scene and a grid square,
both of which the board already knows. The natural control is therefore the
board itself, and the board is already clickable: `app.ts` builds a move command
from a click for every acting role, DM included.

So a click needs disarming before it can mean something else. **A toggle arms
door mode; while armed, a board click works the door under it instead of moving
a token.** This is not a new idea in this client — `view/player.ts` already says
of ability targeting:

> An armed ability means the click is aimed, not a move; targeting is handled by
> the target buttons so a stray board click cannot fire it.

Same shape, same reason. Whether the click opens or closes is read from folded
state (`OpenDoors`), so one control serves both commands and the DM never picks
the wrong verb. A click on a square that is not a door is refused visibly and
does **not** fall through to a move: a mode that silently does the other thing
is worse than a mode that does nothing.

**Players get this control too, and that is not scope creep.** `authz.go`
authorizes `open_door` and `close_door` for `RolePlayer`, gated by
`mayWorkDoor`: a player needs a token they control within Chebyshev distance 1
of the door square. The wire has permitted this since maps-as-geometry and no
human has ever been able to reach it. The client can evaluate the same predicate
from folded state, so a player's door control is **offered only where it would
be granted** rather than offered everywhere and refused — the standard this
client already holds for the perch control, whose comment refuses "an affordance
whose every use is a refusal".

Unlike the join door, these commands produce real events (`DoorOpened`,
`DoorClosed`, both already folded by `fold.ts`), so the board repaints from the
event stream. None of `dm.ts`'s manual-refresh machinery for no-event commands
applies.

### 4.2 `load_map` is a picker

`LoadMap` carries only `{map_id}`. `GET /api/maps` already exists, is open to
every role, and returns each map's id, name, grid dimensions and pack — enough
to render an informative picker with no server work at all. It is fetched once
and handed to the console exactly as `adventures` already is.

The control sits beside Load adventure, because that is what it is: a second way
to bring a place into the campaign. `LoadMap` produces an ordered batch —
`SceneCreated` plus one `TokenPlaced` per declared placement — and is rejected
atomically if a placement names an actor that does not exist. The picker
therefore reports the failure as one message rather than implying a partial load.

---

## 5. A projected seat survives a retraction

### 5.1 The defect

A goblin is revealed to a player at sequence 41 — their own move brought it into
view, so the projection synthesized an introduction stamped 41. At sequence 50
the goblin moves, still visible, and that event is forwarded at its own sequence.
The DM retracts 41.

Every fold expands `EventsRetracted` into a set of sequence numbers and skips by
number. The player's fold therefore skips the introduction and then meets the
sequence-50 move referring to a token it has never seen: **"moved unknown
token"**. The DM's identical retraction folds cleanly, because the goblin reached
the DM at its own sequence and reached the player at the revealing one.

`campaign.Undo` dry-runs the would-be fold before persisting, and it dry-runs
**the log**. A retraction safe for every seat that receives the log can be unsafe
for one that receives a projection of it.

`project_test.go` covers retraction against the projection generally —
`TestASceneRetractedOutOfTheWorldIsForgottenSILENTLY` is the clean case. It does
not cover a retraction whose range removes an introduction while a later event
about the introduced thing survives.

### 5.2 Why not a per-viewer pre-flight

The obvious fix — dry-run every connected viewer's projection before persisting
a retraction, and refuse if any fails — is rejected.

It makes a command's outcome depend on **who happens to be connected**. The same
undo succeeds at one moment and fails at the next for a reason the DM cannot see
and cannot act on, and the DM's own view gives no hint of it. A gate whose answer
moves with the roster is worse than the defect: the defect is at least
reproducible.

It is also unnecessary, because of §5.3's observation.

### 5.3 A fresh catch-up is always coherent

The projection is a pure function of `(log-so-far, viewer)` — visibility spec
§4.1. Re-deriving a projected seat from the retracted log therefore always
produces a foldable stream: the goblin is simply never introduced, and if a later
event about it remains visible, catch-up synthesizes a fresh introduction at
*that* sequence. The dangling reference exists only for a seat that already
folded the pre-retraction stream.

**So a projected seat does not fold a retraction. It starts over.** When a seat
carries a projector — `seat.projected(role)`, which is false for exactly the two
roles that receive the log unfiltered and true for every other role including one
this build has never heard of — and the event is `EventsRetracted`, the gateway
sends that seat a restart instead: empty everything held, replay from the
beginning. The fail-closed direction is inherited rather than re-decided: an
unknown role is projected, so it restarts. The client already has this machinery and already means exactly this
by it — `wire.ts`'s `restart()` dials from zero and fires the restart handlers;
`session.ts` answers with `empty()`, which drops the log entirely rather than
rolling back, "because the whole of it is about to arrive again".

**The DM's seat is untouched.** The identity projection folds retractions
correctly today, `feed.ts` renders the range and `undo.ts` skips by number.
Nothing about that path changes.

### 5.4 Restart is an explicit frame, not a close

`ServerFrame` gains a sixth arm carrying a restart instruction. It must not be
expressed as a connection close: a close is indistinguishable from a network
drop, so the client's reconnect path would resume at `seenSeq - 1` and re-take
its poisoned log — precisely the wrong recovery. An explicit frame says the one
thing that is true: what you hold is void, take it again from the start.

The cost is a full re-projection of the affected seats on every retraction.
Retraction is rare and deliberate; the alternative is a frozen player.

---

## 6. Contract additions

Additive only, per ADR-007.

- One new `ServerFrame` oneof arm for the restart instruction, at the next free
  field number. The five existing arms (`result`, `event`, `catch_up_head`,
  `presence_snapshot`, `presence_changed`) are unchanged.

Nothing else. `OpenDoor`, `CloseDoor` and `LoadMap` already exist on the wire and
are already authorized; this arc gives them senders, not definitions.

---

## 7. Testing

**The parity invariant is itself the test** for §3, and it must fail for the
right reason: a case removed from the table reds it, and a control whose action
string is changed reds it. Both proven by injection rather than asserted.

**The door control** is pinned at the boundary the client actually has: given a
folded state and an armed toggle, a click on a door square produces the correct
command for the door's current state, a click on a plain square produces none,
and a player with no adjacent controlled token is offered no control at all. That
last one is the `mayWorkDoor` mirror, and it is tested as a mirror — the same
scene driven through both the Go predicate and the client's, agreeing.

**The retraction restart** is the keystone case, and it is written RED first
against the current tree, where it fails as "moved unknown token". The shape:
reveal at 41, move at 50, retract 41. Two halves, because the freeze is a
client-side fold error reached through a server-side decision: the Go test
asserts the projected seat is sent a restart rather than the marker, and the
TypeScript test asserts that a `Session` handed the re-derived stream empties,
folds cleanly, reports no error and shows no goblin.

**Existing coverage that must keep passing unchanged:** `session.test.ts`'s torn-
batch reconnect, `project_test.go`'s retraction suite, and the visibility
keystone at every prefix.

---

## 8. What could go wrong

**A restart storm.** A DM undoing several times in a row restarts every projected
seat each time. Bounded by how fast a human presses undo, and each restart is one
projection of a log that is getting shorter. Worth watching in session one, not
worth pre-optimising.

**A player's door control flickering.** The affordance depends on adjacency, so
it appears and disappears as the token moves. That is honest — the permission
really does change — but if it reads as a bug during the walk, the answer is to
show the control disabled with the reason rather than to hide it.

**Arming left on.** A DM who arms doors, walks away and comes back will move
nothing and open things instead. The mode must be visible on the board itself,
not only in the console that armed it.

**The stale-prose risk this arc runs itself.** `project.go` carries a comment
block declaring the torn-batch hazard UNRESOLVED. It is resolved —
`seat.pastResume` filters a projected seat's delivery to `sequence > resume`,
`wire.reconnect()` rolls back to `seenSeq - 1` before re-dialling, and
`session.test.ts:406` pins the whole recovery. That comment cost this spec's
first draft an entire section proposing a fix for a solved problem. It is
corrected as part of this arc, along with its citation of `session.ts:158-160`,
which now points at a presence function; the append is at `:215`.

---

## 9. Exit criteria

1. Every `ClientCommand` case is declared in the surface table, and removing an
   entry or renaming a control's action string reds the gate.
2. A human DM opens and closes a door in a wall from the browser, with no agent
   involved.
3. A player opens an adjacent door from the browser, and is offered no door
   control where `mayWorkDoor` would refuse them.
4. A human DM loads a standalone map from the browser, chosen from a list they
   did not have to type.
5. The reveal-move-retract sequence leaves a projected seat with a correct,
   foldable board and no error, where the same sequence on the current tree
   freezes it.
6. `task check` green, both mutation gates included.
7. A cold reader given only this spec and `docs/map-format.md` can open a door
   from the browser without asking how.
