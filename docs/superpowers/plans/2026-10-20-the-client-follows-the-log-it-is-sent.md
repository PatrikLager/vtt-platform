# Plan: a player or a spectator chooses whose log the browser shows, and the browser follows it

- **Ticket:**
  `docs/superpowers/specs/2026-10-20-the-client-follows-the-log-it-is-sent-design.md`.
- **Verdict:** passes with gaps (G1-G11, below).
  - Verified 2026-10-20, read-only, against HEAD `0db3d18`, by an agent that
    did not write the ticket.
  - Its probes ran on a copy of the tree made with `git archive HEAD` and
    wrote nothing into the repository.
  - They used `go tool buf generate`, `go test`, `golangci-lint`, `bun test`
    and `vite build`.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-20-the-client-follows-the-log-it-is-sent.md`.
- **Base:** main `0db3d18`.
- **Branch:** `feat/the-client-follows-the-log-it-is-sent`.
- **Signed off:** 2026-10-20 ("ja kör"), D1-D19 as written.
  - Q1 and Q2 were answered on 2026-10-20 as the verifier recommended (D12,
    D13):
    - Q1, "ok, vi behåller det": a run still ends by a non-party grant and a
      revocation (VTT-389), and a creature left in a player's control is not
      offered;
    - Q2, "Bara knappar": a spectator switches by its buttons alone.
  - The standing rule holds: review findings are fixed and each task is
    committed without asking, except a design question. Push and merge happen
    on the owner's word.

## The gaps this plan inherits

- **G1. Item 3 does not cover the ordinary redial.** That is the gateway
  opening on the held log after the cursor, where the client must keep what
  it folded. A client that reset at every `ViewpointChanged` would pass items
  1-3. Task 3 adds the test (VTT-396).
- **G2. Item 2 does not cover a redial after a switch.** There, `after` is the
  new log's last sequence, not the old one's. Task 3 adds the test (VTT-398).
- **G3. Item 5 leaves out a promotion.** It names a grant, a revocation, a
  kind change and a removal. A promotion changes the set too.
  - Measured on the probe: a spectator promoted to a player who controls
    nothing is sent the empty set, then `ViewpointChanged` "".
  - Task 2's test includes it (VTT-400).
- **G4. SPEC-016 is moved and not named.** One clause of its consequences
  becomes false: "a spectator can be offered a shoulder that is no longer a
  party member, which `MayWatch` refuses (SPEC-015)". Task 9 moves it.
- **G5. Two comments become false and item 8's grep cannot see either:**
  - `SetViewpoint`'s comment in `contract/vtt/v1/commands.proto`, which says
    "perches a spectator" and "the client re-sends it on reconnect", and
    carries a "Task 6 of the visibility arc" history line. Task 1 rewrites it.
  - `setViewpoint`'s comment in `client/src/commands.ts`, which says "a perch
    does not survive a reconnect and the client is what re-sends it". Task 5
    rewrites it.
- **G6. Item 8's grep counts `look()` as a deleted name.** It is not:
  `(*Projection).look` lives in `internal/perceive/projection.go`. Rewritten
  comments point at SPEC-016, so the grep's pattern stays as the ticket wrote
  it.
- **G7. Item 8's grep does not reach the machinery's other words.** Task 5
  adds a second search:
  `grep -rnE 'sequence 0|sequence-0|SEQUENCE 0|SEQUENCE ZERO|perch frame|perch-shaped|perchShaped|perchGranted|torn batch|seenSeq|high-water|HIGH-WATER|rollback|onRollback|restart\(|re-sends? (it|the shoulder)|re-perch' client/src`.
  Today it prints 77 lines (measured): 50 in `wire.ts`, 18 in `app.ts`, 8 in
  `session.ts` and 1 in `commands.ts`. After Task 5 it prints nothing.
- **G8. The contract change alone moves the bundle.**
  `cmd/vtt/webdist/assets/index.js` changes, because the generated TS embeds
  the file descriptor (measured: `vite build` on the probe copy). So Task 1
  rebuilds webdist too, not only the client tasks.
- **G9. Files the work moves that the ticket does not name:**
  - the three Go test readers in D5;
  - `client/test/qa-window-number.test.ts`, whose spectator test finds the
    "Quill" shoulder in the fold;
  - `tools/comment-ceilings.txt`;
  - `tools/ts-mutation-equivalents.txt`;
  - `client/src/view/doors.ts` (D13).
- **G10. The ticket's "What could not be established", settled here:**
  - **Names (#1).** A party member is on every roster, and one added again as
    another is introduced at once (SPEC-016). The set carries its own names
    either way (D1).
  - **Order (#2).** D4.
  - **A player holding only non-party actors (#3).** D12 and Q1.
  - **Timing (#4).** Not established. The figure on record is 4501 s on
    `21648b0` (`docs/reports/2026-10-19-the-window-shows-no-log-number.md`).
- **G11. Whether a spectator's board click switches.** The ticket's rule "and
  a spectator to each one and to nobody" does not say. This plan reads it as
  buttons only (D13, Q2).

## Constraints that bind every task

**CLAUDE.md's rules.** They are named here and stated where they live:
- **2.** No floor, ceiling, threshold, exemption or adjudication is moved to
  get a green run. A ceiling row only goes down, by `--write-ledger`.
- **3.** The contract change is additive: a new message, a new message type
  for its entries, and a new `ServerFrame` arm. `check:breaking` only reports
  before release (`contract/RELEASED` is absent), so the rule binds by being
  the rule.
- **4.** Nothing here applies an event. The gateway reads
  `Campaign.Actors()`, and the client's fold is the TS mirror it already has.
- **5.** No game-system vocabulary. "Party member" is already the platform's
  word (`engine.IsPartyMember`).
- **8.** Cite names. Never `file:line`, never a `.superpowers/` path.
- **9.** Answered below.
- **10.** A comment added under `client/src` or `internal/` is a warning, a
  pointer, or an exported symbol's one-line doc. Test files carry citation
  lines.

**The owner's rulings.** They are quoted in the ticket and in
`docs/specifications/015-the-seat-and-the-perch.md`:
- 2026-10-15: one active character at a time, switched by clicking its token.
  No viewpoint means no events.
- 2026-10-16: the client resets at `ViewpointChanged`.
- 2026-10-17: "Första karaktären". The DM and the agent keep the world. A
  reconnect naming `log=` opens on that log directly.
- 2026-10-19: "Bara fönstret". Every sequence stays on the wire.
- 2026-10-20: "Två, klienten först" and "Servern skickar listan".

**What follows from the rulings, by this plan:**
- No Go reader changes. `internal/harness/client.go`'s `readLoop` skips an arm
  it does not know. `internal/harness`, `internal/mcp` and `contract` stayed
  green on the probe with the frame in place.
- `cmd/vtt` also stayed green, with one exception.
  `TestQAHoldADirectoryInviteCreatesHoldsNoEventsTable` failed at a load near
  16, and passed when run alone.
- What the harness, `vtt mcp` and `vtt state dump` do at a switch is the next
  ticket's, together with the open debt entry "The harness cannot see a log
  seat switched on its old subscription".

**Tier 1 runs at pre-commit.**
- It runs `bun test client/test contract contract-spike`,
  `task client:typecheck`, lint, vet, the Go tier-1 tests, arch, vocabulary,
  doc-owner, secrets and the review gate.
- The typecheck includes `client/test`. A changed signature therefore changes
  every test call in the same commit.
- Map a `NodeList` through `Array.from`.

**bun is quiet under `CLAUDECODE`.** Run it as `env -u CLAUDECODE bun test …`
from the repository root.

**The bundle.**
- The commit hook does not catch a stale bundle. Run `task build:client`
  before each commit that changes `client/src` or `contract/gen/ts` (G8).
- Comment-only changes do not reach the bundle. Check that with
  `git status --short cmd/vtt/webdist`.
- `task check:drift` reads `git status --porcelain`, so it can pass only after
  the commit. Pre-push runs it too.
- `task e2e` builds `vtt` from the working tree with whatever webdist is
  there.

**The comment gate (SPEC-010, `tools/check-comments.py`).**
- The gate refuses:
  - an added comment line carrying a banned term: a date, "measured", "used
    to", "previously", "spec §", "task N", "review found";
  - a block over six lines to which a line is added. Rewriting one line inside
    a long block therefore means rewriting the block to six lines or fewer;
  - a comment line added to a file above its row.
- `client/src/view/spectator.ts` is above its row today: 49.8% against 49.6
  (measured, `--report`). Task 4 must bring it under before it adds a comment
  line.
- A file more than 1.0 point under its row needs
  `python3 tools/check-comments.py --write-ledger`, which only lowers rows.
- Run `python3 tools/check-comments.py main` and the same against HEAD after
  staging.
- `internal/gateway/qa_promotion_rejudge_test.go`'s row is 0.0. Its edit adds
  no comment line.

**Tests that register rows cite keep their names.** They change bodies only:
- VTT-393's evidence:
  - `client/test/app.test.ts`'s "no role's window shows a log sequence
    number";
  - `client/test/spectator-view.test.ts`'s "each ticker row is its event's
    description alone, newest first";
  - `client/e2e/table.spec.ts`'s "the story feed and ticker record what
    happened";
  - the four `client/test/qa-window-number.test.ts` tests the row names.
- VTT-230's and VTT-263's `spectator-view.test.ts` tests.
- Every gateway test whose file gains a helper change (D5).

**The TS mutation gate.**
- `check:ts-mutation` re-mutates the whole client in its container on any
  change under `client/src` or `client/test`. It runs only in `task check`.
- An entry in `tools/ts-mutation-equivalents.txt` whose code is gone is stale,
  and fails the gate (VTT-301).
- Entries name code, never lines (SPEC-020).

**The Go mutation gate.**
- `check:mutation` re-mutates `internal/gateway`.
- Write no guard that no test can reach. In particular, `watchable` takes no
  `id != ""` check: `engine.Apply` refuses an `ActorAdded` with an empty id,
  so the live actors never hold that key.

**Coverage.**
- These client files have floors of 100.00 in
  `tools/ts-coverage-thresholds.txt`:
  - `client/src/app.ts`, `commands.ts`, `player.ts`, `session.ts` and
    `wire.ts`;
  - `client/src/view/player.ts` and `view/spectator.ts`.
- `internal/gateway` has a floor of 93.5 in `tools/coverage-thresholds.txt`.

**Lint.** `gocyclo`'s `min-complexity` is 30. On the probe, `pump` measured 19
and the new queuing method 11, and `golangci-lint run ./internal/gateway/`
found 0 issues.

**Scheduling.** Gateway tests that depend on goroutine order are stressed with
`go test -race -count=20 -cpu 1,2`. Read the load with `os.getloadavg()`
first.

**`review-record` refuses untracked files.** Commit the ticket and this plan in
Task 0, and park scratch files outside the tree.

**Mechanics.**
- A new test goes after a closing brace.
- Stage and commit in separate calls.
- Undo a break with the inverse edit and check it with grep, never with
  `git checkout --`.
- Hash the tree before and after each QA agent.
- Never stash while a reviewer reads.

## Rule 9: how MapTool does this

Checked in `~/dev/RPTool/maptool/src/main/java`. The ticket's answer holds,
with three additions.

- **How a view is chosen.** `ZoneViewModel.makePlayerView(role, selected)`
  takes the selected tokens that have sight and that `AppUtil.playerOwns`. That
  is every token for a GM, and every token when strict token management is
  off. With none selected, it takes `Zone.getOwnedTokensWithSight(player)`
  when the server uses individual views, and `Zone.getPlayerTokensWithSight()`
  (the `PC` tokens with sight) otherwise.
- **How the client learns of a change of owners: it is not told.**
  - `ZoneRenderer.renderZone` calls `ZoneViewModel.update` on every paint.
  - Its `updatePlayerView` recomputes the view from the current selection and
    the current owners.
  - A selected token the player no longer owns drops out of the view
    (`removeIf`). With none left, the view falls back to the owned or the `PC`
    tokens.
  - This works because the client holds every token with its owner list, so a
    token update is all it needs.
- **How a click chooses.**
  - `PointerTool.mousePressed` replaces the selection with a pressed token
    that is not yet selected, if `SelectionModel.isSelectable` allows it
    (visible, or owned).
  - It leaves the selection alone for a token that is already selected.
  - It starts a box selection on empty ground. A move is a drag.

**What we take:**
- Pressing one's own token is what moves the view. This is the shape the
  2026-10-15 ruling borrows.
- Pressing the one already chosen changes nothing (D13).

**What we leave:**
- The union of views and multi-select. Each connection has one viewpoint
  (SPEC-015).
- The drag. Here a click moves (D13).
- A client that works out whom it may look through from the whole campaign.
  That is the distribution model this platform refuses (CLAUDE.md rule 9;
  `internal/gateway/seat.go`).

`WatchableChanged` has no MapTool equivalent for exactly that reason. A client
that holds one character log cannot work the set out for itself.

## Decisions

1. **D1. The frame.**

   ```proto
   message WatchableChanged {
     repeated WatchableActor actors = 1;
   }
   message WatchableActor {
     string actor_id = 1;
     string name = 2;
   }
   ```

   It is a new `ServerFrame` arm: `WatchableChanged watchable_changed = 7`.
   - **The name is a preference.** CLAUDE.md rule 3's imperative and
     past-tense forms bind commands and events, and a frame is neither. Its
     precedent is `ViewpointChanged`: a frame sent at the opening and again at
     each change, carrying the whole present value. `PresenceChanged` is a
     delta, and `PresenceSnapshot` is sent once. "Watch" is the gateway's own
     word (`MayWatch`, `watch`, `watchesALog`).
   - **7 is forced.** `ServerFrame`'s oneof holds 1-6, with no gap and no
     `reserved` range. SPEC-007 lists the contract's gaps: 11 and 16 in
     `ClientCommand`, and 17 in `Envelope`.
   - **`name` is forced.**
     - A spectator watching nobody holds no fold (VTT-341), so it has no
       roster to label a button from (item 6).
     - The gateway reads the name from the same `Campaign.Actors()` snapshot
       that decided the set.
     - An empty name is labelled by the id (VTT-404), as `label` in
       `client/src/view/spectator.ts` does today.
   - **The empty set** goes out as `{"watchableChanged":{}}`, because
     protojson omits an empty repeated field. The arm's presence is the
     frame, as `ViewpointChanged`'s is. Task 1 pins both fixtures.
   - The proto comment is at most three lines and points at SPEC-007 and
     SPEC-015.

2. **D2. Where the set is judged.** `watchable(p, actors)` in
   `internal/gateway/viewpoint.go` gives every id that `MayWatch` accepts
   among `actors`, sorted by id, each with its name.
   - The seat records the set at each judgment, from that judgment's role and
     actors: in `openSeat`, and in `seat.resolve`.
   - **Forced.** `resolve` is what `watch`'s settle loop and `recheck` call, and
     `recheck` is reached from `onWake`, `admit` and `rejudge`. Every path that
     takes a wake, or reads a new role, therefore records the set.
   - **Measured on the probe.** A grant (of a party member the watched
     character does not see), a revocation, a kind change, a removal and a
     promotion each produced a new set. A narration produced none.
   - `defaultViewpoint` is left as it is (scope).

3. **D3. When the pump sends the set.**
   - It sends the set when it differs from the last one queued, comparing
     the frames with `proto.Equal`. The first one is always sent.
   - It does so at the opening, and in `pumpState.moved` for every judgment.
   - A world seat (`seat.wake == nil`) sends none.
   - **Sending only on a difference is a preference.** Every append wakes
     every log seat, so sending on every judgment would add a frame per
     append per connection.
   - The new test holds it: no set follows a narration.

4. **D4. Order: the set goes before the `ViewpointChanged` the same judgment
   causes**, at the opening and at every move.
   - **Forced.** SPEC-007 and SPEC-015 say a `ViewpointChanged` is followed by
     the new log from its first entry, and VTT-342's tests read it that way.
   - **Measured on the probe.** With the set after the `ViewpointChanged`,
     three `seat_internal_test.go` tests that check `len(out)` after the
     announcement fail, even with D5's readers in place. With the set before
     it, none do.
   - The set is queued through the same helper as `ViewpointChanged`.
     `sendViewpoint` is generalised to take the frame, so the encode-failure
     arm stays one arm.

5. **D5. Three Go test readers learn to skip the frame.** They are
   `watchConn.next` in `internal/gateway/watch_test.go`, `frameOf` in
   `internal/gateway/seat_internal_test.go`, and `qaRejudgeIsSideFrame` in
   `internal/gateway/qa_promotion_rejudge_test.go`.
   - **Measured on the probe.** The frame in place and the readers unchanged
     gave 27 failing gateway tests. With the three readers changed, 0 failed.
   - The new tests read frames in order through a reader of their own, so that
     they see the set.

6. **D6. The client holds one log.**
   - **What `Wire` holds:**
     - `log`: null for a connection never told a viewpoint, which is the world
       for a DM; "" for nobody; otherwise the actor id;
     - one cursor, `lastSeq`.
   - **At a `ViewpointChanged` naming another log than `log`**, `Wire` does,
     in order:
     1. sets `log`;
     2. sets the cursor to 0;
     3. raises the reset;
     4. hands the actor to `Session`.
   - **At an entry of sequence 1 while the cursor is above 0**, `Wire` sets
     the cursor to 0 and raises the reset before it delivers the entry.
     Otherwise a redial after a same-log restart cut short at entry 3 of N
     would ask for `after=N`, and entries 4..N would never be folded.
   - The cursor never goes down otherwise. "an out-of-order replay never
     walks the resume cursor backwards" stays.
   - **Forced** by the ticket's rule and by SPEC-021: a log numbers from 1,
     with no gaps (VTT-321).
   - **Why sequence 1, and not "at or below the cursor".** A duplicate or an
     out-of-order entry must fail the fold honestly (`Session.ingest`'s "hold
     the last good state"). Resetting on one would leave a board folded from
     the middle of a log.

7. **D7. What goes and what stays.**
   - **Deleted:** `Wire.restart`, `Session.restart`, `Wire.onRollback`,
     `Session.rollback`, `seenSeq`, `connect`'s `onOpen` hook, and these
     parts of `client/src/app.ts`:
     - `perchShaped` and `perchGranted`;
     - `redial`'s restart branch;
     - the `viewpoint` that the result set;
     - `iMayPerch`, inlined into `paint`.
   - **Kept, with new meaning:**
     - `Wire.onRestart` stays as the reset channel. The wire raises it itself
       (D6).
     - `Session.empty` stays as its handler.
   - **Forced.**
     - No sequence-0 envelope reaches a connection: `enqueueAll` ignores one
       (SPEC-021).
     - Every entry has its own sequence (VTT-321).
     - A reconnect naming `log=` opens after its cursor while it may still
       watch that log (VTT-339, VTT-349).
   - **What each redial now does:**
     - A perched spectator resumes like everyone else, with nothing re-sent.
     - A spectator promoted to a player is re-judged by the gateway, which
       announces any move.

8. **D8. The redial.**
   - The URL is `?token=…&after=<lastSeq>`, plus `&log=<id>` when `log` is a
     non-empty id. The id is URI-encoded, as the token is.
   - Every role redials through `session.reconnect()`.
   - `connect()` takes no cursor and dials from the one `Wire` holds, so a
     first connect is `after=0` with no `log`.
   - A DM's or an agent's `after` is its last world sequence.
   - **Forced** by the ticket's rule and by SPEC-015's consequence "a player's
     or a spectator's reconnect names the log its cursor was read from".

9. **D9. A reset clears the fold failure the window shows.**
   - The failure moves into `Session`, set by `ingest` and cleared by `empty`,
     and `app.ts` draws it from there.
   - **Forced** by the observations of items 1 and 3: the window shows the log
     the gateway sends. Otherwise `app.ts`'s `failure` sticks for the rest of
     the page, through every later log.
   - VTT-393's exception still holds for a log that cannot fold.

10. **D10. The window shows as watched the actor the last `ViewpointChanged`
    named** (`Session.viewpoint`). That covers the spectator's "Perched on"
    and the marked button and chip.
    - Neither the click nor the result decides it. `perchOn` becomes a send
      that shows a refusal verbatim and clears the toast on success.
    - **Forced.**
      - SPEC-015: "a client learns of a move by its `ViewpointChanged`", and
        the gateway moves seats unasked.
      - The 2026-10-19 report's QA observation: a spectator opened on a party
        member read "Perched on: nobody".

11. **D11. The set in the client.**
    - `Wire` gets a `watchableChanged` arm.
    - `Session` holds the last set (empty until the first) and notifies a
      change.
    - The set is connection state: a reset keeps it, and each new set replaces
      it, a redial's opening one included.

12. **D12. A player's panel.**
    - **The chips.** When `app.ts` hands `renderPlayerPanel` a switcher, the
      chips are the set's party members, in the set's order (by id),
      labelled per VTT-404.
      - The watched one is marked `chip sel`.
      - A click on any chip sends `set_viewpoint` naming it, as the
        spectator's buttons do. On the watched one the gateway answers
        nothing (VTT-375).
    - **`ui.selectedActorId`.** For a player, it is the watched actor. `app.ts`
      writes it when a `ViewpointChanged` names another actor, and clears an
      armed ability then. No click writes it. The panel's abilities and
      `moveCommandFor` therefore act as the watched character (VTT-408).
    - **The DM and the agent** get no switcher. Their picker and
      `ui.selectedActorId` are as today.
    - **Forced** by the rulings of 2026-10-15 and 2026-10-20, and by VTT-351:
      the move gate judges by the watched character's eyes.
    - **A controlled non-party creature is not offered** (Q1, accepted).
      A player watching nobody acts as nobody (VTT-370).

13. **D13. A player's board click**, in order:
    1. With doors armed, a door command or nothing, as today.
    2. On a cell of the board's scene holding a token of a party member the
       set names, other than the watched one: `set_viewpoint` naming it. If
       several, the lowest id.
    3. On a cell holding only the watched one's token: nothing. This is
       MapTool's precedent, and a preference.
    4. Otherwise, a move of the watched one's token.

    The cell check is a pure function beside `moveCommandFor` in
    `client/src/view/player.ts`. It uses `currentSceneId`, exported from
    `client/src/view/doors.ts` rather than copied a fourth time (a
    preference).

    A spectator's board takes no click, as today (Q2, accepted).

14. **D14. A spectator's shoulders.**
    - `PerchControl` gains the set. `renderPerch` lists the set's party
      members, sorted by label and then by id, with its comparator unchanged.
      This keeps both `spectator.ts` entries in
      `tools/ts-mutation-equivalents.txt` keyed to code that stays.
    - Each shoulder is labelled per VTT-404.
    - "No party members yet." shows for an empty set. "No shoulder" is always
      there.
    - "Perched on:" names the watched one by the set's name, by its id when
      the set does not name it, and as "nobody" for "".
    - The fold is no longer read for shoulders.

15. **D15. Test fixtures open the way the gateway does.**
    - **For a player or a spectator**, a list then a `ViewpointChanged` come
      before any entry. This applies to:
      - `seedTable`, `tableAs` and `playerTableWithDoor` in
        `client/test/app.test.ts`;
      - "clicking the board moves the player's token";
      - "the ruleset's abilities reach the player panel, and nothing else
        does";
      - `openWith` in `client/test/qa-window-number.test.ts`.
    - **DM fixtures** are unchanged.
    - **Forced** by D12 (a player acts as the watched one) and by VTT-368.

16. **D16. Comments.**
    - Each block a task touches ends at six lines or fewer, as a warning or a
      pointer to SPEC-007, SPEC-015 or SPEC-016.
    - The long blocks about the replay cursor, `restart`, `perchShaped` and
      `redial` are deleted with their code.
    - **Shares expected to fall:**
      - `client/src/wire.ts`, `session.ts`, `app.ts` and `commands.ts`;
      - `client/src/fold.ts`;
      - `client/src/view/spectator.ts`, `view/dm.ts` and `view/player.ts`.

      Each row is lowered by `--write-ledger` in the task that lowers it.
    - `spectator.ts` must be under 49.6 before Task 4 adds a comment line
      there.

17. **D17. Mutation adjudications.**
    - **In `tools/ts-mutation-equivalents.txt`:**
      - `wire.ts`'s `env.sequence >= this.seenSeq` entry is deleted with its
        line.
      - `wire.ts`'s `env.sequence >= this.lastSeq` entry keeps its key, if the
        line stays as it is. Its reason is rewritten, because `reconnect` no
        longer derives from `seenSeq`.
      - `wire.ts`'s `default:` entry stays.
      - `spectator.ts`'s two comparator entries keep their keys. Their reasons
        are rewritten to the set's entries, and the "equal id and equal label
        render identically" argument carries over.
    - **New survivors in Go or TS** are killed by a test where one can see
      them, and only otherwise adjudicated (SPEC-020).

18. **D18. Task order** follows what each task depends on:
    - 1 before 2: the Go types.
    - 1 before 4: the TS types.
    - 3 before 4: a switch the UI sends must not break the window, and Task
      4's tests drive switches through Task 3's reset.
    - 3 depends on nothing in 1 and 2, because `ViewpointChanged` has been on
      the wire since `db608e9`. It follows them so that the contract and its
      gateway land first. That is a preference.
    - 5 sweeps what 3 and 4 leave.
    - 6 observes 4.
    - 7 is on the last code commit, which is Task 6.

19. **D19. Phase 4a** runs one QA agent on each of Tasks 2, 3 and 4. Each has
    observable behaviour and was written by people who expect its tests to
    pass. Tasks 1, 5 and 6 have none: they cover a schema checked by
    round-trips, comments, and the e2e that is itself the observation.

## Requirements

The rules below are sorted per the `requirements` skill. Ids are allocated
after sign-off by the dispenser (`requirement-id`), and each row starts
`**OPEN — no test yet**`. Each rule carries:
- what it says;
- the observation that goes red when it is broken, and the planned test that
  holds it;
- the break that shows it.

**VTT-394.** "When a `ViewpointChanged` names another party member than the
one whose log the client holds, or nobody while it holds one, the client
discards what it folded and folds what follows."
- *Observation:* item 1, for a player and for a spectator. The test is
  `client/test/app.test.ts`'s "a switch to a log that opens a session and a
  scene the first held shows the second log's board". It was measured red
  today, at "Cannot derive state from the log: session already open at
  sequence 1". The switch to nobody is `app.test.ts`'s "a spectator who
  switches to nobody is shown no board", which replaces the deleted "a
  spectator who hopped OFF a shoulder still starts over".
- *Break:* C1, which reddens both.

**VTT-395.** "When an entry of sequence 1 arrives while the client holds
entries of its log, the client discards what it folded and folds from that
entry."
- *Observation:* item 3's same-log half. The test is `app.test.ts`'s "a redial
  the gateway opens on another log, or on the held log from its first entry,
  shows that log folded from its first entry".
- *Break:* C3.

**VTT-396** (G1). "A `ViewpointChanged` naming the log the client holds leaves
what it folded in place."
- *Observation:* `app.test.ts`'s "a redial the gateway opens on the held log
  after its cursor keeps what was folded". The scene from entry 2 is still
  drawn after entry N+1.
- *Break:* C2.

**VTT-397.** "A redial names in `log` the actor whose log the client holds, and
names no `log` when the client holds the world or watches nobody."
- *Observation:* item 2. The test is `client/test/wire.test.ts`'s "a player's
  redial names the log it holds and the last sequence it folded". It was
  measured red today, with `log` null. The nobody half is `wire.test.ts`'s "a
  redial while watching nobody names no log": the gateway reads `log=` as no
  `log` (`openSeat`), so only the URL can show it.
- *Breaks:* C4 and C9.

**VTT-398.** "A redial's `after` is the highest sequence the client has been
sent of the log it holds since it last discarded what it folded."
- *Observation:* item 2, plus `wire.test.ts`'s "a DM's redial names its last
  world sequence and no log", (G2) "a redial after a switch resumes the new
  log, not the old one", and "a redial after the held log started again
  resumes from where the new start reached". It was measured red today, with
  `after` N-1.
- *Breaks:* C5, C6 and C10.

**VTT-399.** "A player's or a spectator's connection is sent, at its opening, a
`WatchableChanged` naming the party members `MayWatch` accepts for its
participant, each with its name, even when there are none."
- *Observation:* `internal/gateway/watchable_test.go`'s
  `TestAConnectionIsToldWhomItMayWatchBeforeItsOpeningViewpoint`.
- *Breaks:* B1, B4 and B7.

**VTT-400.** "A player's or a spectator's connection is sent a new
`WatchableChanged` whenever the party members `MayWatch` accepts for its
participant change."
- *Observation:* `TestTheSetIsSentAgainWhenItChanges`,
  `TestASpectatorIsToldOfEveryPartyMember` and
  `TestAPromotionTellsTheSeatTheNewRolesSet`. Together they cover a grant (one
  the watched character does not see included), a revocation, a kind change,
  a removal and a promotion (G3).
- *Break:* B2.

**VTT-401.** "A DM's or an agent's connection is sent no `WatchableChanged`."
- *Observation:* `TestTheDMAndTheAgentAreToldNoSet`.
- *Break:* B3.

**VTT-402.** "A player is offered a switch button for exactly the party
members the last `WatchableChanged` names."
- *Observation:* item 6. The test is `app.test.ts`'s "a player is offered
  exactly the party members the set names". The set names one the fold shows
  no control of, and the fold shows control of one the set does not name.
- *Break:* E1.

**VTT-403.** "A spectator is offered a switch button for exactly the party
members the last `WatchableChanged` names, and one for nobody."
- *Observation:* item 6. The test is `client/test/spectator-view.test.ts`'s "a
  spectator watching nobody is offered every party member the set names, and
  nobody". The `app.test.ts` half boots a spectator. Both were red today by
  reading: "No party members yet.".
- *Break:* E5.

**VTT-404.** "A switch button is labelled by the name the last
`WatchableChanged` gives its party member, or by its id when that name is
empty."
- *Observation:* the VTT-402 and VTT-403 tests. A name the fold does not hold
  appears on the button.
- *Break:* E6.

**VTT-405.** "A click on a switch button sends `set_viewpoint` naming its party
member, or naming nobody for the spectator's "No shoulder"."
- *Observation:* item 4's button half. The test is `app.test.ts`'s "a player
  switches by a button and by a click on the token". "'no shoulder' reaches
  the wire, and is not quietly skipped" stays.
- *Break:* E2.

**VTT-406.** "With doors not armed, a player's click on a cell holding a token
of a party member the last `WatchableChanged` names, other than the one they
watch, sends `set_viewpoint` naming one of those party members."
- *Observation:* item 4's token half, in the same test. It was red today by
  reading: the click sends `moveToken`.
- *Break:* E3.

**VTT-407** (D10). "The client shows as watched the actor the last
`ViewpointChanged` named, whatever a command's result said."
- *Observation:* `app.test.ts`'s "the window follows the ViewpointChanged, not
  the command's result". An ok result with no `ViewpointChanged` leaves
  "Perched on: nobody". A `ViewpointChanged` with no command moves it. "the
  indicator follows the SERVER's answer, never the click" keeps its name, with
  its body rewritten to the frame.
- *Break:* C7.

**VTT-408** (D12). "A player's board click that moves a token moves the token of
the character they watch."
- *Observation:* `app.test.ts`'s "a player's move moves the character they
  watch". The player is watching the second of two party members.
- *Break:* E9.

**The sort refused ten candidates:**

- **"A switch or the set appends nothing."** Not new: a frame is not an
  `Envelope` (SPEC-007), and `set_viewpoint` appends nothing (VTT-184).
- **"The set comes before the `ViewpointChanged` the same change causes."**
  That is how, not what (D4).
- **"The set is sent only when it differs."** How (D3). Its test asserts it
  anyway.
- **"A redial re-sends no `set_viewpoint`."** Not a rule: one re-sent naming
  the same log sends nothing back (VTT-375).
- **"A click on the watched character's token sends nothing."** A UI choice
  (D13), held by its test in `client/test/player.test.ts`.
- **"A reset clears the fold failure."** It is part of the VTT-394 and VTT-395
  observations: the window shows the new log.
- **"A spectator's board click sends nothing."** Existing behaviour that this
  work leaves alone, held by "a spectator's clicks on the board send nothing"
  (Q2).
- **"No comment names a deleted symbol."** A process rule, held by item 8's
  grep and G7's search (Task 5).
- **"`task check` is green."** Process (item 9).
- **"The set carries each party member's name."** It is inside VTT-399.

**Rows withdrawn: none.** The search:
- The live rows whose evidence is under `client/` are VTT-228, 229, 230, 263,
  264, 266, 274, 277, 278, 285, 286, 289-293, 297 and 393: the fold's bounds,
  the notes, the DM console, the move reason and the window. None states a
  rollback, a restart, a high-water mark, a perch frame or a sequence-0
  frame.
- The gateway rows about the opening and the switch stay true under D4. These
  are VTT-341, VTT-342, VTT-367, VTT-368, VTT-374, VTT-375 and VTT-385. The set
  is not an entry, and it precedes the `ViewpointChanged`.
- The grep for "perch", "restart", "rollback", "sequence 0", "seenSeq",
  "redial", "reconnect", "resum" and "shoulder" found only rows already
  withdrawn (VTT-178, 179, 180, 183, 185, 186, 187, 226, 246) and rows that
  stay true.

## Tasks

Each code task follows this pattern:

1. Write the tests and see each fail for its stated reason.
2. Implement, and see them pass.
3. Apply each break, see the named test go red and every other new test stay
   green, then restore it by the inverse edit and check the restore with grep.
4. Stage, then run `python3 tools/check-comments.py main` and the same against
   HEAD.
5. Rebuild the bundle where the constraints say to.

Then come the reading review, the review record and the commit, and after the
commit `task check:drift`. Each task is its own commit through the review
gate.

### Task 0: sign-off and ids

- **Files:**
  - `docs/requirements.md`: VTT-394 to VTT-408's rows, from the dispenser;
  - the ticket and this plan;
  - all three committed together.
- **True when:**
  - The owner has signed off D1-D19, and answered Q1 and Q2.
  - VTT-394 to VTT-408 are allocated and OPEN, and replace the labels the
    plan was signed with. Their text is the register's; R6-R11 and R13 name
    `WatchableChanged` where the signed text said "the set", so that each row
    stands alone.
  - `task check:requirements-chain` is green.

### Task 1: the contract carries whom a connection may watch (G5, G8)

- **Files:**
  - `contract/vtt/v1/commands.proto`:
    - `WatchableChanged` and `WatchableActor`, and the `ServerFrame` arm at 7
      (D1);
    - `SetViewpoint`'s comment, rewritten to at most four lines: it names the
      log a player's or a spectator's connection is sent; it appends nothing;
      an empty `actor_id` is nobody, the spectator's alone; and it points at
      SPEC-007 and SPEC-015.
  - `contract/vtt/v1/events.proto`: `Actor.kind`'s comment, where "MayPerch"
    becomes `MayWatch` (item 8).
  - `contract/gen/go/vtt/v1/commands.pb.go`, `events.pb.go`,
    `contract/gen/ts/vtt/v1/commands_pb.ts` and `events_pb.ts`, by
    `task generate:contract`.
  - `contract/testdata/server_frame_watchable_changed.json` (two actors, one
    with an empty name) and `server_frame_watchable_changed_none.json`
    (`{"watchableChanged":{}}`).
  - `contract/roundtrip_test.go`:
    - `TestServerFrameWatchableChangedRoundTrip` and
      `TestServerFrameWatchableChangedToNoneRoundTrip`;
    - `TestAnEmptyWatchableSetStillSerializesAsItsFrame`, in the shape of
      `TestEmptyPresenceSnapshotStillSerializesAsASnapshotFrame`.
  - `contract/events.test.ts`: two rows in `cases`, and the comment that names
    `project_internal_test.go` rewritten to point at SPEC-016 (item 8).
  - `cmd/vtt/webdist/assets/index.js`, by `task build:client` (G8).
- **True when:**
  - `go tool buf lint` is clean.
  - `go test -count=1 ./contract/ ./internal/gateway/ ./internal/harness/ ./internal/mcp/`
    is green. The gateway sends no new frame yet.
  - `env -u CLAUDECODE bun test client/test contract contract-spike` and
    `task client:typecheck` are green.
  - `task check:breaking` reports no breaking change.
  - `git diff --stat` names only these files.
  - After the commit, `task check:drift` is green.
- **Breaks:**
  - Rename `actorId` to `actor` in the two-actor fixture. Both round-trips go
    red: the Go unmarshal refuses an unknown field, and the TS `fromJson`
    throws.
  - Drop the empty fixture's arm (`{}`). The "none" round-trips and the empty
    serialisation test go red.
- **Phase 4a:** none (D19).

### Task 2: the gateway sends whom a connection may watch (item 5; VTT-399 to VTT-401; G3)

- **Files:**
  - `internal/gateway/viewpoint.go`: `watchable` (D2), with no empty-id guard.
  - `internal/gateway/seat.go`: the set recorded in `openSeat` and
    `seat.resolve` (D2), with no comment line added. The file is at its row,
    16.2.
  - `internal/gateway/server.go`: `pumpState` records the last set queued.
    Its queuing method is called before `announce` at the opening and inside
    `moved` (D3, D4), and `sendViewpoint` is generalised to queue either frame
    (D4).
  - `internal/gateway/watch_test.go`, `seat_internal_test.go` and
    `qa_promotion_rejudge_test.go`: the three readers (D5), each one
    condition, with no comment line added.
  - `internal/gateway/watchable_test.go`, new, package `gateway_test`. It
    reuses `newGWFixture`, `f.watch`, `w.command`, `appendEvents`,
    `partyMemberOf`, `participantID`, `logOf` and `requireLog`, plus one
    reader that returns the next frame of any kind but presence. It carries
    citation lines only.
  - `tools/comment-ceilings.txt`, by `--write-ledger` (the new file's row, and
    any row lowered).
  - `docs/requirements.md`: VTT-399 to VTT-401's evidence.
- **The tests**, each in this order: the head, then the set, then the
  `ViewpointChanged`, then entries.
  - **`TestAConnectionIsToldWhomItMayWatchBeforeItsOpeningViewpoint`:**
    - the fixture's player is sent `[a1 "Hero"]`, then `ViewpointChanged` a1;
    - the spectator is sent every party member, including one another player
      controls, then `ViewpointChanged` "";
    - the other player, who controls nothing, is sent `{}` (the empty set),
      then `ViewpointChanged` "".
  - **`TestTheSetIsSentAgainWhenItChanges`**, on the player:
    1. grant `a2`, which has no token and so is unseen by a1: `[a1, a2]`, and
       no `ViewpointChanged`;
    2. a narration: no set;
    3. revoke `a1`: `[a2]`, then `ViewpointChanged` a2, then a2's log from
       its first entry;
    4. grant `a2` `ACTOR_KIND_NON_PARTY`: `{}`, then `ViewpointChanged` "";
    5. grant `a3`: `[a3]`, then `ViewpointChanged` a3;
    6. remove `a3`: `{}`, then `ViewpointChanged` "".
  - **`TestASpectatorIsToldOfEveryPartyMember`:**
    1. a new party member is added to the set;
    2. a kind change takes one out;
    3. the watched one leaving the party is sent `{}`, or the rest, then
       `ViewpointChanged` "" (VTT-386).
  - **`TestAPromotionTellsTheSeatTheNewRolesSet`:** a spectator opened on a1
    is promoted to a player who controls nothing, and is sent `{}`, then
    `ViewpointChanged` "" (G3; probe-measured).
  - **`TestTheDMAndTheAgentAreToldNoSet`:** across an append that changes the
    set, the DM's and the agent's frames hold no `watchableChanged`.
- **Breaks**, one per assertion. Each is red at its own assertion, with the
  others green:
  - **B1.** The queuing method returns without queuing: red at the opening.
  - **B2.** The set is recorded in `openSeat` only, not in `seat.resolve`: red
    at the grant.
  - **B3.** Drop the world-seat guard: red in the DM test.
  - **B4.** `watchable` takes `engine.IsPartyMember` alone, not `MayWatch`:
    red at the player's opening, which then names the other player's party
    member.
  - **B5.** Queue the set after `announce`: red at the opening order and at
    the revocation's order, and three `seat_internal_test.go` tests go red too
    (probe-measured).
  - **B6.** Drop the `proto.Equal` check: red at "a narration: no set".
  - **B7.** Name each actor by its id: red at "Hero".
- **True when:**
  - `go test -count=1 ./internal/gateway/ ./internal/harness/ ./internal/mcp/ ./cmd/vtt/`
    is green. If a `cmd/vtt` deadline test fails under load, rerun it alone
    and check the load.
  - `go test -race -count=20 -cpu 1,2 -run 'Watchable|WhomItMayWatch|TheSetIs|NewRolesSet|ToldNoSet' ./internal/gateway/`
    is green, at a load you have read first.
  - `golangci-lint run ./internal/gateway/` finds nothing.
  - `go-arch-lint` is clean.
  - `go doc ./internal/gateway` is unchanged, because no exported symbol is
    added.
- **Phase 4a:** one QA agent (below). Its file is
  `internal/gateway/qa_watchable_test.go`.

### Task 3: the client holds one log (items 1-3; VTT-394 to VTT-398, VTT-407; G1, G2)

- **Files:**
  - `client/src/wire.ts`:
    - `log`, one cursor, the `viewpointChanged` arm and the reset (D6);
    - `connect()` with no cursor, and `reconnect()` without the rollback (D8);
    - the deletions (D7);
    - the header and `pending`'s comment rewritten to at most six lines each.
  - `client/src/session.ts`: the deletions (D7); `empty` as the reset's
    handler, which also clears the failure (D9); `viewpoint` and `failure`
    exposed.
  - `client/src/app.ts`:
    - `redial` becomes `session.reconnect()`;
    - the deletions (D7);
    - the failure is drawn from the session (D9);
    - the perch's `current` is the session's viewpoint (D10);
    - `mayPerch`'s comment becomes a pointer to SPEC-015.
  - `client/test/wire.test.ts`, `session.test.ts` and `app.test.ts`, as below.
  - `cmd/vtt/webdist/assets/index.js`, by `task build:client`.
  - `tools/comment-ceilings.txt`, by `--write-ledger` (`wire.ts`, `session.ts`
    and `app.ts` fall).
  - `tools/ts-mutation-equivalents.txt` (D17: the `wire.ts` entries).
  - `docs/requirements.md`: VTT-394 to VTT-398's and VTT-407's evidence.
- **New tests:**
  - **`wire.test.ts`:**
    - "a player's redial names the log it holds and the last sequence it
      folded" (item 2);
    - "a DM's redial names its last world sequence and no log";
    - "a redial after a switch resumes the new log, not the old one". The
      wire holds a1 to 5, then a2 to 2, so the expected query is `log=a2`,
      `after=2`;
    - "a reconnect whose dial fails leaves the cursor and the log";
    - "a redial while watching nobody names no log" (VTT-397);
    - "a redial after the held log started again resumes from where the new
      start reached": the wire holds a1 to 5, is sent a1's entries 1 to 3
      again, and redials with `log=a1`, `after=3` (VTT-398).
  - **`session.test.ts`:**
    - "a ViewpointChanged naming another log empties what was folded";
    - "a ViewpointChanged naming the log held keeps it";
    - "an entry of sequence 1 starts the held log again";
    - "a reset clears the fold failure".
  - **`app.test.ts`:**
    - item 1's test, for a player and for a spectator. The probe's shape: a1's
      log, then `ViewpointChanged` a2 and a log that opens a session and
      introduces scene s1 again. It was measured red today;
    - item 3's test, with both halves: another log, and the held log from
      entry 1;
    - G1's test: the held log after its cursor;
    - "a spectator who switches to nobody is shown no board" (VTT-394);
    - VTT-407's test.
- **Tests that change:**
  - **In `wire.test.ts`:**
    - every `connect(0n)` becomes `connect()`;
    - "a reconnect resumes one sequence below the last one seen, and says so"
      is replaced by item 2's test;
    - "a reconnect before anything was ever seen still resumes from zero"
      loses `onRollback`;
    - "an out-of-order replay never walks the resume cursor backwards" now
      expects `after=5`;
    - "a frame from the socket a reconnect abandoned is never folded" loses
      its rollback assertions;
    - "reconnecting repeatedly does not walk the cursor backwards" now expects
      `after=9` each time;
    - "a close answers the commands sent on ITS socket, and no others" keeps
      its body, with its comments brought up to date;
    - "a frame from the socket a restart abandoned is never folded", "a
      restart before anything was ever connected dials from zero rather than
      throwing" and "a restart whose dial fails leaves the wire exactly as it
      was" are deleted with `restart()`.
  - **In `session.test.ts`:**
    - "reconnect redials and resumes from the last folded sequence" now
      expects `after` "4";
    - "a reconnect after a torn batch takes that sequence again, and folds it
      exactly once" and "a redial that drops nothing does not repaint" are
      deleted with the rollback;
    - the three `restart` tests are deleted, and the new reset tests replace
      them.
  - **In `app.test.ts`:**
    - "clicking Reconnect redials from the last folded sequence" now expects
      `after=7`;
    - "the indicator follows the SERVER's answer, never the click" keeps its
      name, and is rewritten to the frame;
    - "an accepted perch moves the indicator" becomes "a ViewpointChanged
      moves the indicator";
    - "a spectator's reconnect starts the log over, and re-sends the
      shoulder" becomes "a spectator's reconnect names the log it watches and
      re-sends nothing";
    - "a player's reconnect still resumes, and sends no perch" is folded into
      item 3's test;
    - these seven are deleted: "a redial whose abandoned socket closes late
      still ends up perch-shaped" (its late close is `wire.test.ts`'s "a close
      answers…"), "perch frames that outran a result the drop swallowed still
      start the next redial over", "a spectator who has never perched resumes
      like anybody else", "a REFUSED perch does not shape the log, so that
      watcher still resumes", "a spectator who hopped OFF a shoulder still
      starts over", "hopping off a shoulder nobody ever took leaves the log
      unshaped" and "a spectator PROMOTED to player still starts over, because
      their log is perch-shaped".
- **Fixtures that send sequence 1 twice.** After the reset lands, run the
  whole client suite. A fixture that delivers a second entry of sequence 1 on
  one connection now raises the reset, and a gateway never sends one without a
  restart. Renumber any such fixture, and say which in the report.
  `wire.test.ts`'s two-connect tests are the place to look first.
- **Breaks:**
  - **C1.** Skip the reset at a `ViewpointChanged` naming another viewpoint:
    item 1's test goes red at "Cannot derive state", and "a spectator who
    switches to nobody is shown no board" goes red at the old board.
  - **C2.** Reset at every `ViewpointChanged`: G1's test goes red.
  - **C3.** At an entry of sequence 1, skip raising the reset but keep the
    cursor's zeroing: item 3's same-log half goes red.
  - **C4.** Leave `log` out of the URL: item 2's test goes red.
  - **C5.** `after` = cursor − 1: item 2's test goes red.
  - **C6.** Leave the cursor alone at a switch: G2's test goes red.
  - **C7.** Set the indicator from the ok result: VTT-407's test goes red.
  - **C8.** Keep the failure across a reset: "a reset clears the fold failure"
    goes red.
  - **C9.** Send `log=` for nobody: "a redial while watching nobody names no
    log" goes red.
  - **C10.** Leave the cursor alone at a sequence-1 reset: "a redial after the
    held log started again resumes from where the new start reached" goes
    red, asking for `after=5`.
- **True when:**
  - The client suite and `task client:typecheck` are green.
  - Item 8's grep and G7's search print nothing for `wire.ts`, `session.ts`
    and `app.ts`.
  - `git status --short cmd/vtt/webdist` names `assets/index.js`.
  - The comment gate is clean against main and against HEAD.
  - After the commit, `task check:drift` is green.
- **Phase 4a:** one QA agent. Its file is `client/test/qa-one-log.test.ts`.

### Task 4: a player and a spectator switch (items 4 and 6; VTT-402 to VTT-406, VTT-408)

- **Files:**
  - `client/src/wire.ts` and `client/src/session.ts`: the `watchableChanged`
    arm, and the set held across a reset (D11).
  - `client/src/app.ts`:
    - the player's switcher, `ui.selectedActorId` from the viewpoint (D12);
    - `onCell` for a player (D13);
    - the perch handed the set (D14).
  - `client/src/view/player.ts`: the switcher chips (D12), and the cell check
    beside `moveCommandFor` (D13).
  - `client/src/view/doors.ts`: `currentSceneId` exported (D13).
  - `client/src/view/spectator.ts`: `renderPerch` from the set (D14), with its
    block comment deleted. That brings the file under 49.6 (D16). Then
    `ViewExtras.perch`'s doc, which names `MayPerch`, and `PerchControl`'s,
    which says `app.ts` moves the indicator only on an ok result and is false
    from Task 3 on (D10), become pointers to SPEC-015. They wait for this task
    because the file is above its row until the block goes.
  - **Tests:**
    - `client/test/app.test.ts`: VTT-402, VTT-405, VTT-406 and VTT-408's
      tests, and D15's fixtures;
    - `client/test/spectator-view.test.ts`: the perch tests read a set, not a
      fold. "the perch lists the party, and nobody else" becomes "the perch
      lists exactly the party members it is handed". "a shoulder the roster no
      longer holds reads as its id, not as nobody" becomes "a shoulder the set
      does not name reads as its id, not as nobody". "a table with no party
      yet says so rather than rendering an empty panel" is given an empty set.
      VTT-403 and VTT-404's tests are new;
    - `client/test/player.test.ts`: the cell check's tests, including "a click
      on the watched character's token sends nothing" and "two listed party
      members on one cell switch to the lowest id";
    - `client/test/session.test.ts`: "the set survives a switch", and "a new
      set replaces the last";
    - `client/test/qa-window-number.test.ts`: `openWith` delivers a set for a
      log seat. "spectator: perching on another shoulder, and the new log that
      follows, show no sequence of either log" finds "Quill" in it. Every test
      keeps its name.
  - `cmd/vtt/webdist/assets/index.js`.
  - `tools/comment-ceilings.txt` and `tools/ts-mutation-equivalents.txt` (D17:
    the `spectator.ts` reasons).
  - `docs/requirements.md`: VTT-402 to VTT-406's and VTT-408's evidence.
- **Breaks:**
  - **E1.** The player's chips come from `controlledActors`: VTT-402 goes red.
  - **E2.** A chip click writes `ui.selectedActorId` instead of sending: VTT-405
    goes red.
  - **E3.** The cell check is skipped, so the click moves: VTT-406 goes red.
  - **E4.** A click on the watched one's token moves: `player.test.ts`'s
    "sends nothing" goes red.
  - **E5.** The spectator's shoulders come from the fold: VTT-403 goes red.
  - **E6.** Label by id: VTT-404 goes red.
  - **E7.** Drop "No shoulder": "'no shoulder' reaches the wire, and is not
    quietly skipped" goes red.
  - **E8.** A reset empties the set: "the set survives a switch" goes red. That
    is a new `session.test.ts` test.
  - **E9.** A player's mover is the first controlled actor: VTT-408 goes red.
- **True when:**
  - The client suite and the typecheck are green.
  - `spectator.ts` is at or under its row.
  - The comment gate is clean.
  - After the commit, `task check:drift` is green.
- **Phase 4a:** one QA agent. Its file is `client/test/qa-switch.test.ts`.

### Task 5: the comment sweep (item 8; G5-G7)

- **Files:**
  - `client/src/commands.ts`: `setViewpoint`'s comment, at most six lines,
    with "re-sends" gone;
  - `client/src/fold.ts`: three blocks, each at most six lines, pointing at
    SPEC-016;
  - `client/src/view/dm.ts`: the block naming `MayPerch`, at most six lines;
  - comments in `client/test/app.test.ts`, `session.test.ts`,
    `spectator-view.test.ts`, `dm-view.test.ts`, `commands.test.ts`,
    `removal-batch-parity.test.ts` and `fold-unit.test.ts`;
  - `tools/comment-ceilings.txt`.
- **True when:**
  - item 8's grep prints nothing;
  - G7's search prints nothing;
  - `git status --short cmd/vtt/webdist` prints nothing, because comments do
    not reach the bundle;
  - the comment gate is clean, with lowered rows only.
- **Break:** put one stale pointer back. The grep prints it.
- **Phase 4a:** none.

### Task 6: the e2e (item 7)

- **Files:**
  - `client/e2e/table.spec.ts`. A new test, "a spectator chooses a shoulder
    and is shown that log", goes after "a DM starts a session and loads an
    adventure":
    - the spectator is offered "Human Fighter" (`act-fighter`, `party_member`
      in `adventures/goblin-ambush/actors/act-fighter.json`) and "Lera";
    - they choose Human Fighter;
    - "Perched on: Human Fighter" shows;
    - `.tick` rows exist;
    - "Cannot derive state" never shows;
    - screenshot `04b-spectator-on-a-shoulder`.
  - `client/e2e/handover.spec.ts`. After beat 3:
    1. click "Bram";
    2. Bram's chip is `chip sel` and Ash's is not, and Bram's token is on the
       board;
    3. click "Ash", and the chips swap back;
    4. no "Cannot derive state" at any point;
    5. screenshots `h6b-player-switches-to-bram` and `h6c-player-back-on-ash`.
- **True when:**
  - `task e2e` passes every test, `handover.spec.ts`'s skipped one aside.
  - No `vtt serve` or browser is left running, and nothing listens on
    8900-8910.
- **Break:**
  1. Copy webdist aside.
  2. Write Task 3's commit's bundle with
     `git show <task-3-commit>:cmd/vtt/webdist/assets/index.js`.
  3. Run `bunx playwright test client/e2e/table.spec.ts client/e2e/handover.spec.ts`.
     Both new beats go red.
  4. Rebuild, and check that `diff -r` against the copy prints nothing.
- **Phase 4a:** none.

This is the last code commit.

### Task 7: `task check`, whole, on Task 6's commit (item 9)

- **Before launching:**
  - Run `docker info`. `check:ts-mutation` re-mutates the whole client.
  - Read the load.
  - The figure on record is 4501 s (G10).
- **Launch:** with `start_new_session=True`, and read the gate's own completion
  lines.
- **True when:**
  - it exits 0;
  - no floor is lowered, and no ceiling is raised.
- **After it:** the records change no `.go` or `.ts` file. Before each record
  commit, run `task check:requirements-chain`, `task check:new-prose` and the
  comment gate against main and HEAD.

### Task 8: the implementation report

- **File:** `docs/reports/2026-10-20-the-client-follows-the-log-it-is-sent.md`,
  written by the `implementation-report` skill against the ticket's ten
  items, each with its observation.
- **It carries:**
  - G1-G11;
  - the rulings, and Q1's and Q2's answers;
  - D4's and D5's measurements;
  - each break and what it reddened;
  - Phase 4a's adjudications, one line each;
  - the rows lowered;
  - Task 7's figures, at their commit.

### Task 9: the specification step (item 10; G4)

- **Files:**
  - **`docs/specifications/007-the-wire-contract.md`.** Under "What the
    connection opens with", a paragraph on `WatchableChanged`:
    - what it carries;
    - that it comes before the opening `ViewpointChanged` and again at each
      change, before any `ViewpointChanged` that change causes;
    - that the empty set is `{"watchableChanged":{}}`;
    - that it is a frame and never an `Envelope`;
    - that the DM and the agent are sent none.

    Then what a client does: it holds one log; it resets at a
    `ViewpointChanged` naming another party member or nobody, and at an entry
    of sequence 1 while it holds entries; it keeps the log at one naming the
    log it holds; and its redial names `log` and `after` (`client/src/wire.ts`).
    In "A sequence is for correlating…", "resumes and rolls back" becomes
    "resumes". The Status line gains the tests.
  - **`docs/specifications/011-the-connection.md`.** The frames the pump
    queues, and the producer sentence, gain `WatchableChanged`. The Status
    line names the generalised helper.
  - **`docs/specifications/015-the-seat-and-the-perch.md`.**
    - "The open" and "What moves a seat" say when a seat is sent the set: at
      each judgment where it differs, before that judgment's
      `ViewpointChanged`.
    - The Status line gains `watchable` and the queuing method.
    - Consequences gain "a client learns whom it may watch by
      `WatchableChanged`".
  - **`docs/specifications/016-the-projection.md`.** The consequence clause in
    G4 becomes: whom a connection may watch is `WatchableChanged`'s (SPEC-015),
    not a roster's.
  - Each Requirements list gains its rows.
- **True when:**
  - each passage passes the `specification` skill's catches list, read against
    the committed code;
  - `task check:requirements-chain` is green.

## For Phase 4a

One QA agent for each of Tasks 2, 3 and 4, before that task's commit.

**For every agent:**
- Hash the tree before and after it runs.
- It writes only its own file, with the prefix named, and cites the rows.
- Its tests are committed with the task. Its adjudications go in the report.

**Task 2's agent** (`qa_watchable_test.go`, prefix `qaWatchable`):
- **It receives:**
  - VTT-399 to VTT-401's text;
  - the rulings of 2026-10-17 and 2026-10-20;
  - SPEC-007's "What the connection opens with";
  - SPEC-015 whole;
  - `go doc ./internal/gateway`;
  - `internal/gateway/watch_test.go`'s and `server_test.go`'s fixtures
    (`newGWFixture`, `f.watch`, `w.command`, `appendEvents`, `partyMemberOf`).
- **It is withheld:** `watchable_test.go`, this plan, the verdict, and the
  bodies of `seat.go`, `viewpoint.go` and `server.go`.
- **Its brief names:**
  - `go-arch-lint`;
  - `-cpu 1`;
  - that a frame is read by kind, never by position;
  - that the set comes before the `ViewpointChanged`, so a reader that wants
    order must see both kinds.

**Task 3's agent** (`client/test/qa-one-log.test.ts`):
- **It receives:**
  - VTT-394 to VTT-398's and VTT-407's text;
  - SPEC-007's "The ordering contract" and "What the connection opens with";
  - SPEC-015's consequences;
  - SPEC-021's "What a character log is";
  - the exported signatures of `client/src/wire.ts` and `session.ts`;
  - `client/src/app.ts`'s `boot`;
  - `client/test/support/dom.ts`;
  - `client/test/app.test.ts` as it is at main, for its `FakeSocket` harness.
- **It is withheld:** the new tests, this plan, the verdict, and the source
  bodies.
- **Its brief says:**
  - run bun as `env -u CLAUDECODE bun test`;
  - map a `NodeList` through `Array.from`;
  - a log numbers from 1, and a world log does too.

**Task 4's agent** (`client/test/qa-switch.test.ts`):
- **It receives:**
  - VTT-402 to VTT-406's and VTT-408's text;
  - the rulings of 2026-10-15 and 2026-10-20;
  - Q1's and Q2's answers;
  - the exported signatures of `client/src/view/player.ts`,
    `view/spectator.ts` and `app.ts`'s `boot`;
  - the same harness files as Task 3's agent.
- **It is withheld:** the same as Task 3's agent.
