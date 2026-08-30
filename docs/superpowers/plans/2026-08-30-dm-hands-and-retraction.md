# Hands on the Board, and a Seat That Survives an Undo — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give a human at the browser the three commands only the agent could send, make "a command with no human sender" a gate failure rather than an oversight, and stop one undo from permanently freezing a player.

**Architecture:** Three of twenty-two `ClientCommand` arms have no builder in `client/src/commands.ts`. A declared surface table plus a test that walks the generated oneof turns that from an oversight into a build failure; the two controls then fill the gap (a board-click mode for doors, a picker for maps). Separately, a projected seat cannot fold a retraction that removes an introduction it was given at the revealing sequence — so the gateway stops sending it the marker and sends a restart instead, and the client re-derives from a fresh catch-up, which is coherent by construction.

**Tech Stack:** Go 1.26 (`internal/gateway`, `internal/campaign`), TypeScript + Bun (`client/`), protobuf via buf (`contract/`), Task for gates.

**Spec:** `docs/superpowers/specs/2026-08-30-dm-hands-and-retraction-design.md`

## Global Constraints

- **Airtight TDD (ADR-009).** Tests first, RED before the solution exists, behavioral RED over compile-failure RED wherever a stub can compile. After-the-fact tests need fault-injection proof per load-bearing assertion.
- **`task check` is the single gateway.** Never weaken a gate to pass it. `task check:fast` is an inner-loop convenience and never satisfies the gate.
- **Contract evolution is additive only** (ADR-007). Generated code is committed; regenerate with `task generate:contract`. `check:breaking` enforces.
- **One fold.** `engine.Apply` via `campaign.foldEvents` is the only code that changes game state.
- **No game-system vocabulary in platform code** (pillar P2/P4, semgrep enforces).
- **Review before commit.** Nothing lands unreviewed; the dev-cycle hook enforces it. Commit with `CLAUDE_REVIEW_DONE=1` after the task's review.
- **State the invariant, not the count.** No assertion or comment may encode "twenty-two commands". Assert the relationship.
- **Rebuild the embedded bundle** after any `client/src` change: `task client:build` refreshes `cmd/vtt/webdist`, and the commit hook will NOT catch a stale one.

---

### Task 1: Every command has a sender, and it stays that way

**Files:**
- Create: `client/test/command-surface.test.ts`
- Modify: `client/src/commands.ts` (append three builders after `setJoinDoor`, ~line 310)

**Interfaces:**
- Produces: `openDoor(sceneId: string, at: Point): ClientCommand`, `closeDoor(sceneId: string, at: Point): ClientCommand`, `loadMap(mapId: string): ClientCommand`. `Point` is the existing `{x: number; y: number}` already imported by `moveToken`.
- Produces: `COMMAND_SURFACE`, exported from the test file, mapping every `ClientCommand` oneof case name to `{surface, action?, why?}`.

- [ ] **Step 1: Write the failing test**

Create `client/test/command-surface.test.ts`:

```ts
import { test, expect } from "bun:test";
import { ClientCommandSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import * as commands from "../src/commands";

/**
 * WHERE A HUMAN REACHES EACH COMMAND.
 *
 * maps-as-geometry shipped with three commands (open_door, close_door,
 * load_map) that only the MCP agent could send: the web client had no builder
 * and no control, and nothing anywhere said so. Three arcs later it was still
 * true. This table is what says so.
 *
 * A count would rot by addition — the failure this table exists to prevent —
 * so nothing here asserts how many commands there are. It asserts that the
 * generated oneof and this table name the same set.
 */
type Surface =
  | "dm-console"
  | "player-panel"
  | "board"
  | "spectator"
  | "join-flow"
  | "not-user-issued";

export const COMMAND_SURFACE: Record<
  string,
  {
    surface: Surface;
    /** The control's data-action, for the panels. Task 4 fills these in and
     *  asserts each one renders; declared here so the shape does not change
     *  under a later task. */
    action?: string;
    /** Required when surface is "not-user-issued", and only then. */
    why?: string;
  }
> = {
  startSession: { surface: "dm-console" },
  endSession: { surface: "dm-console" },
  createScene: { surface: "dm-console" },
  placeToken: { surface: "dm-console" },
  addActor: { surface: "dm-console" },
  loadAdventure: { surface: "dm-console" },
  loadMap: { surface: "dm-console" },
  upsertNote: { surface: "dm-console" },
  deleteNote: { surface: "dm-console" },
  addNarration: { surface: "dm-console" },
  removeCondition: { surface: "dm-console" },
  retractEvents: { surface: "dm-console" },
  grantActorControl: { surface: "dm-console" },
  revokeActorControl: { surface: "dm-console" },
  promoteParticipant: { surface: "dm-console" },
  setJoinDoor: { surface: "join-flow" },
  rotateJoinLink: { surface: "join-flow" },
  moveToken: { surface: "board" },
  openDoor: { surface: "board" },
  closeDoor: { surface: "board" },
  useAbility: { surface: "player-panel" },
  setViewpoint: { surface: "spectator" },
};

/** The oneof's case names, read from the generated descriptor. */
function commandCases(): string[] {
  const oneof = ClientCommandSchema.oneofs.find((o) => o.name === "command");
  if (!oneof) throw new Error("ClientCommand has no `command` oneof");
  return oneof.fields.map((f) => f.localName);
}

test("every command the contract defines has a declared human surface", () => {
  const declared = new Set(Object.keys(COMMAND_SURFACE));
  const missing = commandCases().filter((c) => !declared.has(c));
  expect(missing).toEqual([]);
});

test("the table declares nothing the contract does not define", () => {
  const defined = new Set(commandCases());
  const stale = Object.keys(COMMAND_SURFACE).filter((c) => !defined.has(c));
  expect(stale).toEqual([]);
});

test("a command nobody can issue must say why, so the hatch costs something", () => {
  const bare = Object.entries(COMMAND_SURFACE)
    .filter(([, v]) => v.surface === "not-user-issued" && !v.why?.trim())
    .map(([k]) => k);
  expect(bare).toEqual([]);
});

test("every command with a human surface has a builder in commands.ts", () => {
  // The builder's name is the case name: commands.ts has named them that way
  // since it was written, and this test is what keeps that true.
  const withoutBuilder = commandCases()
    .filter((c) => COMMAND_SURFACE[c]?.surface !== "not-user-issued")
    .filter((c) => typeof (commands as Record<string, unknown>)[c] !== "function");
  expect(withoutBuilder).toEqual([]);
});
```

- [ ] **Step 2: Run it to verify it fails for the right reason**

Run: `bun test client/test/command-surface.test.ts`
Expected: FAIL on "every command with a human surface has a builder" with `["openDoor", "closeDoor", "loadMap"]`. The first three tests pass — the table already declares all cases, which is what makes this a behavioral RED rather than a compile failure.

- [ ] **Step 3: Add the three builders**

Append to `client/src/commands.ts`, after `setJoinDoor`:

```ts
/**
 * OpenDoor / CloseDoor work a door in a WALL, and are not the join door.
 *
 * view/dm.ts renders buttons labelled "Open the door" and "Close the door"
 * already, and they are setJoinDoor — the admissions door for seating people
 * at the table. Two different doors, one word. These carry a scene and a
 * square; that one carries a policy.
 */
export function openDoor(sceneId: string, at: Point): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "openDoor", value: { sceneId, at } },
  });
}

export function closeDoor(sceneId: string, at: Point): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "closeDoor", value: { sceneId, at } },
  });
}

/**
 * LoadMap brings one standalone map into the campaign as a whole ordered
 * batch — a SceneCreated plus one TokenPlaced per declared placement — and is
 * rejected atomically if a placement names an actor that does not exist yet.
 * So a refusal means nothing loaded, and the caller may say so plainly.
 */
export function loadMap(mapId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "loadMap", value: { mapId } },
  });
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bun test client/test/command-surface.test.ts`
Expected: 4 pass.

- [ ] **Step 5: Prove each assertion is load-bearing**

For each of the four tests, break the thing it names and confirm only that test reds:
1. Delete the `loadMap` entry from `COMMAND_SURFACE` → test 1 fails.
2. Add `bogusCommand: { surface: "board" }` → test 2 fails.
3. Change `loadMap` to `{ surface: "not-user-issued" }` → test 3 fails (no `why`).
4. Rename the exported `openDoor` to `openDoorX` → test 4 fails.

Restore after each. Record the four results in the commit message.

- [ ] **Step 6: Run the wider suite and commit**

```bash
bun test client/test
task check:fast
git add client/test/command-surface.test.ts client/src/commands.ts
CLAUDE_REVIEW_DONE=1 git commit
```

---

### Task 2: A map is picked, not typed

**Files:**
- Modify: `client/src/view/dm.ts` (add `maps` to `DMDeps` beside `adventures` at ~line 194; render a group after the adventures block at ~line 381)
- Modify: `client/src/app.ts` (retain the maps already fetched at ~line 591; pass into `renderDMConsole` at ~line 466)
- Test: `client/test/dm-view.test.ts`

**Interfaces:**
- Consumes: `loadMap(mapId)` from Task 1; `MapMeta` from `client/src/metadata.ts` (`{id, name, gridWidth, gridHeight, pack?}`), and `fetchMaps`, both of which already exist.
- Produces: `DMDeps.maps: MapMeta[]`.

- [ ] **Step 1: Write the failing test**

Add to `client/test/dm-view.test.ts`, extending its `harness` opts with `maps?: MapMeta[]` passed through to `renderDMConsole`:

```ts
test("a DM loads a map by picking it, and the button says which one", () => {
  const h = harness(newState(), [], {
    maps: [
      { id: "cellar", name: "The Cellar", gridWidth: 10, gridHeight: 9 },
      { id: "wood", name: "", gridWidth: 40, gridHeight: 40 },
    ],
  });
  const load = h.node.querySelector<HTMLButtonElement>('[data-action="load-map"]');
  expect(load).not.toBeNull();
  expect(load!.textContent).toContain("The Cellar");
  load!.click();
  expect(h.sent).toHaveLength(1);
  expect(h.sent[0].command.case).toBe("loadMap");
  expect((h.sent[0].command.value as { mapId: string }).mapId).toBe("cellar");
});

test("a map with no name is offered under its id rather than a blank button", () => {
  const h = harness(newState(), [], {
    maps: [{ id: "wood", name: "", gridWidth: 40, gridHeight: 40 }],
  });
  const buttons = [...h.node.querySelectorAll<HTMLButtonElement>('[data-action="load-map"]')];
  expect(buttons.map((b) => b.textContent)).toEqual(["Load wood"]);
});

test("no maps configured means no Maps group at all, not an empty one", () => {
  const h = harness(newState(), [], { maps: [] });
  expect(h.node.querySelector('[data-action="load-map"]')).toBeNull();
  expect(h.node.textContent).not.toContain("Maps");
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bun test client/test/dm-view.test.ts`
Expected: FAIL — no element matches `[data-action="load-map"]`.

- [ ] **Step 3: Render the group**

In `client/src/view/dm.ts`, add to `DMDeps` beside `adventures`:

```ts
  /**
   * Every standalone map the server loaded at boot (GET /api/maps), already
   * fetched by app.ts for pack loading. Empty renders no group: a Maps
   * heading with nothing under it tells a DM their maps failed to load, which
   * is a different and false story from "this table has none configured".
   */
  maps: MapMeta[];
```

and, immediately after the adventures block:

```ts
  // --- maps ---
  if (d.maps.length > 0) {
    const row: HTMLElement[] = [];
    for (const m of d.maps) {
      row.push(button(`Load ${m.name || m.id}`, () => void d.send(loadMap(m.id)), "load-map"));
    }
    wrap.appendChild(group("Maps", ...row));
  }
```

- [ ] **Step 4: Wire app.ts**

At `client/src/app.ts` ~line 591 the maps are already fetched and handed to `loadMapPacks`. Retain them in a `let maps: MapMeta[] = []` beside `adventures`, assign in the same `.then`, and pass `maps` into `renderDMConsole`.

- [ ] **Step 5: Run tests, rebuild the bundle, commit**

```bash
bun test client/test && task client:typecheck && task client:build
git add client/src/view/dm.ts client/src/app.ts client/test/dm-view.test.ts cmd/vtt/webdist
CLAUDE_REVIEW_DONE=1 git commit
```

---

### Task 3: What a click means when doors are armed — the predicate

**Files:**
- Create: `client/src/view/doors.ts`
- Test: `client/test/doors.test.ts`

**Interfaces:**
- Produces: `doorCommandFor(st: State, me: Me, armed: boolean, cell: Point): ClientCommand | null` and `mayWorkDoor(st: State, me: Me, cell: Point): boolean`.
- Later tasks call `doorCommandFor` from `app.ts`'s `onCell` and `mayWorkDoor` to decide whether to offer the toggle at all.

A separate module rather than more of `view/player.ts`: this predicate is shared by the DM console and the player panel, and `player.ts` is already the player's own surface.

- [ ] **Step 1: Write the failing test**

Create `client/test/doors.test.ts`. The fixture and the first two cases, which
fix the idiom for the rest:

```ts
import { test, expect } from "bun:test";
import { newState, type State } from "../src/state";
import { doorCommandFor, mayWorkDoor } from "../src/view/doors";
import type { Me } from "../src/metadata";

/**
 * THE CLIENT HALF OF A RULE THAT LIVES IN TWO PLACES.
 * internal/gateway/authz.go's mayWorkDoor is the enforcing copy; this one
 * decides whether to OFFER the control. They must agree on the geometry, so
 * cases 7 and 8 below use the same offsets as its Go table test.
 */
function scene(opts: { open?: boolean } = {}): State {
  const st = newState();
  st.Scenes["scn"] = {
    SceneId: "scn", Name: "Cellar", GridWidth: 10, GridHeight: 9,
    Tiles: { "3,3": "door" },
    OpenDoors: opts.open ? { "3,3": true } : {},
  } as State["Scenes"][string];
  return st;
}

const dm: Me = { participantId: "p-dm", role: "dm" } as Me;

test("armed, a closed door opens", () => {
  const cmd = doorCommandFor(scene(), dm, true, { x: 3, y: 3 });
  expect(cmd?.command.case).toBe("openDoor");
  expect((cmd!.command.value as { at: { x: number; y: number } }).at).toEqual({ x: 3, y: 3 });
});

test("armed, an open door closes — one control, both verbs", () => {
  const cmd = doorCommandFor(scene({ open: true }), dm, true, { x: 3, y: 3 });
  expect(cmd?.command.case).toBe("closeDoor");
});
```

The remaining cases, same shape:

```
1. armed, cell is a closed door           -> openDoor(sceneId, cell)
2. armed, cell is an open door            -> closeDoor(sceneId, cell)
3. armed, cell is floor                   -> null
4. armed, cell is wall                    -> null
5. NOT armed, cell is a closed door       -> null   (a move is not this module's business)
6. player, no controlled token in scene   -> mayWorkDoor false, doorCommandFor null
7. player, controlled token at distance 1 diagonally -> mayWorkDoor true
8. player, controlled token at distance 2 -> mayWorkDoor false
9. DM with no token anywhere              -> mayWorkDoor true
```

Case 7 pins Chebyshev rather than Manhattan distance; case 9 pins "a non-player
passes unconditionally". Both mirror `internal/gateway/authz.go`.

- [ ] **Step 2: Run it to verify it fails**

Run: `bun test client/test/doors.test.ts`
Expected: FAIL — module not found. Then add a stub returning `null`/`false` and re-run so the failures are behavioral, per ADR-009.

- [ ] **Step 3: Implement**

`doorCommandFor` returns `null` unless `armed`; reads the door's current state from the folded scene's `OpenDoors` keyed as `fold.ts`'s `doorKey(x, y)` does; refuses when `!mayWorkDoor`. `mayWorkDoor` returns `true` for any non-player role, else scans `st.Tokens` for a token in the same scene whose actor `me` controls with `Math.max(|dx|, |dy|) <= 1`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `bun test client/test/doors.test.ts` — 9 pass.

- [ ] **Step 5: Cross-language mirror check**

Add a case to the existing Go table test for `mayWorkDoor` in `internal/gateway/authz_test.go` using the same scene and offsets as TS case 7 and 8, so both languages are pinned on the same numbers. If the Go test already covers the diagonal, cite it in the TS header instead of adding a duplicate.

- [ ] **Step 6: Commit**

```bash
bun test client/test && task check:fast
git add client/src/view/doors.ts client/test/doors.test.ts internal/gateway/authz_test.go
CLAUDE_REVIEW_DONE=1 git commit
```

---

### Task 4: Arming the board, and the invariant grows a second half

**Files:**
- Modify: `client/src/view/player.ts` (`PlayerUIState` gains `doorsArmed: boolean`)
- Modify: `client/src/view/dm.ts` (the toggle, action `arm-doors`)
- Modify: `client/src/view/player.ts` (the same toggle for a player, offered only when `mayWorkDoor` is true anywhere in the current scene)
- Modify: `client/src/app.ts` (`onCell` tries `doorCommandFor` before `moveCommandFor`)
- Modify: `client/src/view/spectator.ts` (armed indication on the board itself)
- Test: `client/test/dm-view.test.ts`, `client/test/player.test.ts`, `client/test/app.test.ts`, `client/test/command-surface.test.ts`

**Interfaces:**
- Consumes: `doorCommandFor`, `mayWorkDoor` (Task 3); `openDoor`, `closeDoor` (Task 1).

- [ ] **Step 1: Write the failing tests**

In `app.test.ts`: with doors armed, a click on a door square sends `openDoor` and **no** `moveToken`; with doors disarmed, the same click sends `moveToken`; with doors armed, a click on a plain square sends nothing at all.

In `dm-view.test.ts`: a `[data-action="arm-doors"]` control exists and toggling it flips `ui.doorsArmed`.

In `player.test.ts`: a player with an adjacent controlled token is offered the toggle; a player with none is not.

In `command-surface.test.ts`, extend the file with the control-level half:

```ts
test("every command surfaced in a panel has a control a person can reach", () => {
  // Renders the real panels and looks for the real controls, so this table
  // cannot drift from the UI: the assertion reads the DOM, not the table.
  const unreachable = Object.entries(COMMAND_SURFACE)
    .filter(([, v]) => v.surface === "dm-console" || v.surface === "player-panel")
    .filter(([name]) => !controlExistsFor(name))
    .map(([name]) => name);
  expect(unreachable).toEqual([]);
});
```

`controlExistsFor` renders the DM console and player panel against a fixture rich enough to show every group, and looks for `[data-action]` matching that command's declared action. Declare the action string in `COMMAND_SURFACE` alongside the surface in this task.

- [ ] **Step 2: Run to verify they fail**

Run: `bun test client/test`
Expected: FAIL on each new assertion; the control-level test names every console command lacking a `[data-action]`, which is the working list for this task.

- [ ] **Step 3: Implement**

`onCell` becomes: `const cmd = doorCommandFor(...) ?? (ui.doorsArmed ? null : moveCommandFor(...)); if (cmd) act(cmd);` — armed means a non-door click does nothing rather than falling through to a move.

The board shows the armed state itself (a class on the canvas container plus a legible label), not only the console that armed it: §8 of the spec records the DM who arms doors, walks away, and comes back moving nothing.

Give every existing console control its `action` string where one is missing, since the new test demands one per declared command.

- [ ] **Step 4: Run tests, typecheck, rebuild, commit**

```bash
bun test client/test && task client:typecheck && task client:build
git add client/src client/test cmd/vtt/webdist
CLAUDE_REVIEW_DONE=1 git commit
```

---

### Task 5: The restart frame enters the contract

**Files:**
- Modify: `contract/vtt/v1/commands.proto` (`ServerFrame`, ~line 320)
- Regenerate: `contract/gen/**` via `task generate:contract`
- Test: `contract` suite (`bun test contract`)

**Interfaces:**
- Produces: `ServerFrame.restart` at field number **6**, message `Restart {}`.

- [ ] **Step 1: Write the failing test**

In the `contract/` suite, assert the frame round-trips and that its oneof case name is `restart`, mirroring however `catch_up_head` is already asserted there.

- [ ] **Step 2: Run to verify it fails**

Expected: FAIL — no such field on the generated type.

- [ ] **Step 3: Add the arm**

```proto
// Restart tells one connection that everything it holds is void and it must
// take the log again from the beginning.
//
// SENT ONLY TO A PROJECTED SEAT, AND ONLY FOR A RETRACTION. A projected seat
// receives an introduction stamped with the sequence of the event that
// revealed it (visibility spec §4.2), so a retraction covering that sequence
// deletes the introduction while any later event about the introduced thing
// survives at its own sequence — and the seat's fold then fails on the
// dangling reference where the DM's identical retraction folds cleanly
// (internal/gateway/project.go). Re-deriving from a fresh catch-up is
// coherent by construction, because the projection is a pure function of
// (log-so-far, viewer): the thing is simply never introduced, or is
// introduced again at the later sequence that still reveals it.
//
// NOT A CONNECTION CLOSE, deliberately. A close is indistinguishable from a
// network drop, so the client's reconnect path would resume at seenSeq-1 and
// re-take the very log this frame exists to discard.
message Restart {}

message ServerFrame {
  oneof frame {
    CommandResult result = 1;
    Envelope event = 2;
    CatchUpHead catch_up_head = 3;
    PresenceSnapshot presence_snapshot = 4;
    PresenceChanged presence_changed = 5;
    Restart restart = 6;
  }
}
```

- [ ] **Step 4: Regenerate and verify the gates**

```bash
task generate:contract
task check:drift
task check:breaking
bun test contract
```

Expected: all pass. `check:breaking` must be green — adding a oneof arm at a fresh field number is additive.

- [ ] **Step 5: Commit**

```bash
git add contract
CLAUDE_REVIEW_DONE=1 git commit
```

---

### Task 6: A retraction restarts a projected seat instead of reaching it

**Files:**
- Modify: `internal/gateway/seat.go` (a predicate beside `receive`)
- Modify: `internal/gateway/server.go` (the LIVE pump arm, ~line 715)
- Test: `internal/gateway/server_visibility_test.go`, `internal/gateway/seat_internal_test.go`

**Interfaces:**
- Produces: `func (s *seat) restartsOn(env *vttv1.Envelope) bool` — true when this seat carries a projector and `env` is an `EventsRetracted`.

- [ ] **Step 1: Write the failing test**

In `server_visibility_test.go`, drive the spec's §5.1 sequence over real WebSockets: reveal a goblin to a player by moving their token, move the goblin while still visible, then have the DM `retract_events` over the revealing sequence. Assert the player's connection receives a **Restart** frame and no `EventsRetracted`, and that the DM's connection receives the `EventsRetracted` exactly as it does today.

Add a unit test in `seat_internal_test.go`: `restartsOn` is true for a projected seat given an `EventsRetracted`, false for the same seat given any other payload, and false for an unprojected seat given an `EventsRetracted`.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/gateway/ -run 'Retract|Restart' -count=1 -v`
Expected: FAIL — the player receives the marker.

- [ ] **Step 3: Implement**

`restartsOn` on the seat, because the seat is what knows whether it is projected. In `serve`'s live arm, before `deliver(s.encodeFrame, sub.receive(env), ...)`: if `sub.restartsOn(env)`, send the restart frame and skip the delivery.

**Only the live arm.** The backlog arm at ~line 617 must not change: a catch-up is already a fresh derivation over the retracted log, so a retraction inside the backlog needs no restart, and sending one there would loop a reconnecting client forever. Say that in the code comment.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gateway/ -count=1`

- [ ] **Step 5: Prove the loop guard**

Fault injection: move the restart send into `deliver`'s path so it also fires during catch-up, and confirm a reconnecting player loops. Restore, and record the observation in the commit message — this is the failure the "live arm only" comment exists to prevent, and a comment nobody tested is the class this repo keeps finding.

- [ ] **Step 6: Commit**

```bash
task check:fast
git add internal/gateway
CLAUDE_REVIEW_DONE=1 git commit
```

---

### Task 7: The client empties and starts over

**Files:**
- Modify: `client/src/wire.ts` (`handleFrame`'s switch, ~line 448)
- Test: `client/test/wire.test.ts`, `client/test/session.test.ts`

**Interfaces:**
- Consumes: the `restart` frame arm (Task 5).

- [ ] **Step 1: Write the failing test**

In `session.test.ts`, against a fake server that sends a few events, then a `restart` frame, then replays a different (retracted) log from sequence 1: assert the `Session` ends with exactly the replayed log, no fold error, and no duplicate — and that its cursor was reset, by asserting the second connection's `after` query parameter is `0`.

Model it on the existing torn-batch test at `session.test.ts:406`, which already builds a fake server that honours `after`.

- [ ] **Step 2: Run to verify it fails**

Expected: FAIL — the frame falls through the switch, the client keeps its log, and the replay duplicates it into a fold error.

- [ ] **Step 3: Implement**

Add a `case "restart"` to `handleFrame` that does what `restart()` does: fire the restart handlers (which `session.ts` answers with `empty()`), reset both cursors, and re-dial from zero. Reuse `restart()` rather than re-implementing its body, so the two paths cannot drift.

- [ ] **Step 4: Run tests, typecheck, rebuild, commit**

```bash
bun test client/test && task client:typecheck && task client:build
git add client/src/wire.ts client/test cmd/vtt/webdist
CLAUDE_REVIEW_DONE=1 git commit
```

---

### Task 8: The keystone — the sequence that used to freeze a player

**Files:**
- Modify: `internal/gateway/server_visibility_test.go` (end-to-end)
- Modify: `client/test/fold-parity.test.ts` or the projection-parity suite, whichever already owns cross-language projection cases

- [ ] **Step 1: Write the end-to-end assertion**

Reveal at 41, move at 50, retract 41, over real WebSockets: the player's client, driven through the real `Session`, ends with a board that folds cleanly, reports no error, and shows no goblin. This is the spec's exit criterion 5 and it is the one test that proves the whole arc rather than one seam of it.

- [ ] **Step 2: Verify it fails on the pre-Task-6 tree**

`git stash` Tasks 6 and 7, run, and confirm the failure is "moved unknown token". Restore. Record the observed failure text in the commit message — an end-to-end test that has never been seen to fail proves nothing.

- [ ] **Step 3: Run the full gate**

```bash
task check
```

Expected: green, both mutation gates included. Budget ~32 minutes for `check:mutation` if any gated package's dependency closure changed, and up to an hour for `check:ts-mutation` if anything under `client/src` changed — which it has, so the container will run.

- [ ] **Step 4: Adjudicate any survivors**

New client code will produce mutants. Every surviving mutant is either killed by a new test or adjudicated in `tools/ts-mutation-equivalents.txt` with a stated reason. Never adjudicate to get the gate green.

- [ ] **Step 5: Commit**

```bash
git add -A
CLAUDE_REVIEW_DONE=1 git commit
```

---

### Task 9: Correct the prose this arc proved wrong

**Files:**
- Modify: `internal/gateway/project.go` (the UNRESOLVED block, ~lines 433-456; the `session.ts:158-160` citation, ~line 449)

- [ ] **Step 1: Replace the UNRESOLVED block**

It declares the torn-batch hazard unresolved and hands it to "whoever wires the pump". It is resolved, and the comment cost this arc's spec an entire section proposing a fix for a solved problem. Replace it with what is true and where the proof lives: `seat.pastResume` filters a projected seat's delivery to `sequence > resume`; `wire.reconnect()` rolls back to `seenSeq - 1` before re-dialling, so the client drops the partial batch and re-takes that whole sequence; `client/test/session.test.ts`'s torn-batch test pins the recovery end to end.

- [ ] **Step 2: Fix the rotted citation**

`session.ts:158-160` now points at a presence function. The append is `this.log.push(e)` in `ingest`. Cite the function, not the line — a line number is what rotted.

- [ ] **Step 3: Sweep for the same rot in this file**

Every other file:line reference in `project.go` — open each and confirm it still names what the sentence says. Fix or re-anchor any that have moved. This is the cheapest moment to do it, because the file is already open and the arc has already paid for one of them.

- [ ] **Step 4: Commit**

```bash
task check:fast
git add internal/gateway/project.go
CLAUDE_REVIEW_DONE=1 git commit
```

---

## Merge gate

Before the merge call: `task check` green from cold; the spec's nine exit criteria walked one at a time with the result recorded; exit criterion 7 (the cold read) run as a real cold read — one agent given only the spec and `docs/map-format.md`, forbidden from reading the client, asked to open a door from the browser. What it cannot do is the doc gap.
