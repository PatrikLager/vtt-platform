# Plan: the Story draws what happens in the game, and no line that is mechanical

- **Ticket:** `docs/superpowers/specs/2026-11-16-the-story-writes-nothing-from-the-sheet-design.md`,
  as amended on 2026-11-16 for the control lines, and again for the board's,
  the scene's, the note's and the adventure's lines and for the door.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-16 by an agent that did not write the ticket,
    read-only against `92cb3e3` on `feat/the-story-writes-nothing-from-the-sheet`
    in the worktree `vtt-platform-visibility`, with the ticket untracked,
    for the sheet's and control's kinds.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e` and Stryker were not run. P1 and P2 ran in a copy of the
    tree in the session's scratchpad, never in the worktree.
  - **Amended the same day by the ticket's writer,** after the owner's
    rulings on the other mechanical lines, the door and a token's going out
    of sight. In the ticket it touched the problem, items 3 to 6, the rules,
    the MapTool paragraph and "What could not be established". In this plan
    it touched the rulings, the ids, the sort's refusals, the kinds, checks
    1 to 3, G1 and G7 to G9, P7 to P9, Rule 9, D1, D5, D6 and D9 to D11, R3
    and R10 to R14, "The existing tests that change", Task 1's breaks, Task
    3 and the notes for Phase 4a and 4b. A reviewer read the amendment
    before Task 0's commit, and its findings are fixed here; Phase 4b's
    reviewer reads it again with Task 1's diff.
- **To be committed as:** `docs/superpowers/plans/2026-11-16-the-story-writes-nothing-from-the-sheet.md`.
- **Base:** `92cb3e3`. **Branch:** `feat/the-story-writes-nothing-from-the-sheet`.
- **Owner's rulings taken in:**
  - 2026-11-16: the Story writes nothing from a sheet (resources,
    abilities, attributes); an ability's use stays; a condition gained or
    lost stays; the Events panel, the DM's and the agent's, is the
    technical log and is not the subject.
  - 2026-11-16: "kontrollraderna borde också flyttas till loggen. Det är
    mekanisk information": the Story draws no grant and no taking back of
    control; the Events panel keeps both.
  - 2026-11-16: the other mechanical lines go to the log: a token's
    placing, move, removal and description, a scene, a change of view, a
    note and an adventure's loading. A session's start and end, an actor
    coming into sight, joining the party and leaving, and a roll stay.
  - 2026-11-16: a door stays, as an event of the game ("tobbe öppnade
    dörren eller liknande"). Naming who opened it is a ticket of its own;
    here the line names no square, and a door the projection corrects at a
    sight draws none.
  - 2026-11-16 ("Ja"): a token's going out of sight leaves the Story too
    (D11).
  - 2026-11-16: a player's panel that holds the character's sheet is the
    next ticket; no view of the sheet is part of this one.
  - Overturned by the rulings above: OD-4 of 2026-11-14 (the Story draws a give
    and a take in every window, a correction's included), and OD-4 of the
    plan `docs/superpowers/plans/2026-11-15-an-ability-is-given-with-its-value.md`,
    signed off 2026-11-15 (the Story words a resource added and removed,
    and the player's log shows them there) (G3).
  - 2026-10-21: a near-theoretical edge with a small consequence gets no
    code, no test and no debt entry.
  - 2026-09-04: nobody uses the product; no migration. Nothing written to
    any log changes: this is the client's drawing alone.
  - Standing: no counts or measured figures in anything written down; push
    and merge on the owner's word.
- **Signed off:** 2026-11-16. OD-1 A ("ok"), OD-2 A ("låt event panelen
  använda dom"), OD-3 as amended below (the ability button's title), OD-4
  no message ("ingen notis").
- **Ids:** allocated at Task 0. The `R` labels are this plan's: R1
  VTT-809, R2 VTT-810, R3 VTT-822, R4 VTT-812, R5 VTT-813, R6 VTT-814, R7
  VTT-815, R8 VTT-816, R9 VTT-817, R10 VTT-818, R11 VTT-819, R12 VTT-820,
  R13 VTT-821, R14 VTT-823. VTT-811 was allocated for R3 first, named
  `tokenHidden` among the kinds the Story shows, and is withdrawn in Task
  0's commit by the owner's ruling on a token going out of sight.
- **The sort's refusals:** the Events panel's row for each left-out entry
  (VTT-447, held, not added); the player's toast on a give and a take
  (VTT-733, held); no message for a grant (an absence, which nothing could
  turn red); a payload added to the contract failing the client's typecheck
  until the table rules on it (D4, how the rules are held, not a rule); a
  run at a companion's first sight drawing no line (G8, which R2, R3 and
  R10 already decide); the fold, the namer and the Events panel handed the whole log
  (SPEC-022, held by VTT-447 and VTT-448); nothing on the wire changing
  (scenery).

**The left-out kinds**, named once and referred to by that name below:
`resourceChanged`, `abilityGiven`, `abilityTaken`, `resourceAdded` and
`resourceRemoved` (the sheet's); `actorControlGranted` and
`actorControlRevoked` (control's); `tokenPlaced`, `tokenMoved`,
`tokenHidden`, `tokenRemoved`, `tokenDescribed`, `sceneCreated`,
`sceneSeen`, `noteUpserted`, `noteDeleted` and `adventureLoaded` (the
board's and the table's). **The kept kinds:** `sessionStarted`,
`sessionEnded`, `actorAdded`, `actorRemoved`, `conditionApplied`,
`conditionRemoved`, `abilityUsed`, `attackRolled`, `narrationAdded`,
`whisperAdded`, and `doorOpened` and `doorClosed` when the entry carries an
event id.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists:
   `client/src/view/spectator.ts`, `client/src/view/feed.ts`,
   `client/src/view/naming.ts`, `client/src/app.ts`,
   `internal/perceive/projection.go`, every test file it lists,
   `client/e2e/table.spec.ts`, `cmd/vtt/webdist`, `docs/requirements.md`
   and SPEC-022. Every symbol is where it says: `renderSpectator`,
   `renderFeed`, `describe` and `renderTicker` in `spectator.ts`;
   `buildFeed` and `reintroductions` in `feed.ts`; `sheetNews` in
   `naming.ts`; `onPartyMembersLeft` and the "You no longer play" toast in
   `app.ts`; `statusFrames` in `projection.go`, which writes the sheet's and
   control's kinds with no event id (`resourceFrames`, the ability loops,
   `grant`, the revoke loop), and `doorTransitions` beside it, which writes
   a `doorOpened` or `doorClosed` with no event id; `transitions` and
   `hideTokensOf` there and `resume` in `internal/campaign/fanout.go`, the
   three writers of a `tokenHidden` (`git grep 'Envelope_TokenHidden{'`
   over `internal`, `cmd` and `tools`, tests aside), none with an event
   id; `OpenDoor` and
   `DoorOpened` in `contract/vtt/v1`, a scene and a square each; rows
   VTT-263, VTT-447, VTT-622, VTT-623, VTT-662, VTT-733, VTT-736, VTT-756
   and VTT-790; MapTool's `TokenPropertyFunctions`, `EditTokenDialog` and
   `ZoneRenderer` (P3, P8).
2. **"Done" is an observation. By command for items 1, 2, 3, 4 and 7, by
   reading for the others.** Items 1, 2 and 7 were written as probe tests
   (P2): each fails on `92cb3e3` and passes once `buildFeed` leaves the
   sheet's and control's kinds out. Items 3 and 4 fail on `92cb3e3` by P7,
   where tests that expect each kind's Story line turn red under the
   amended filter. Item 5 holds today by VTT-447's and VTT-263's tests.
   Items 6 and 8 name tests that pass today and must keep passing. Item 9 is
   the gate.
3. **Each rule is breakable. By reading.** The four rules map to R1, R2,
   R10, R11, R12 and R14 below; each turns red when one kind is drawn again, and
   each kind is exercised with an event id and with none.
4. **The scope matches the claim. By command, then reading.** The callers
   (P4): `buildFeed` from `renderSpectator` alone; `describe` from
   `renderFeed` and `renderTicker`; `renderSpectator` from `paint` in
   `client/src/app.ts`. The code change is `buildFeed` and `renderFeed`, and
   one pointer in `client/src/view/grid.ts` (`label`'s doc cites VTT-622
   and VTT-623, which are withdrawn). The tests it reaches are wider than
   the ticket's list (P1, P7, G1), and so are the rows (G2). The ticket says
   "among them" and leaves the sort to the plan, so this is a gap, not a
   return.
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-022's Story paragraphs and Consequences, OD-4 of 2026-11-14,
     VTT-662 and VTT-263 are named by the ticket and overturned by it.
   - OD-4 of the 2026-11-15 plan is overturned too (G3), and the ticket now
     names it.
   - Blueprint principle 12: a taking back is still told by the toast; a
     grant was told by a Story line in a log that held it and is now told
     by nothing but the perch and the panel offering the character (OD-4).
   - Blueprint principle 17: a wound still reaches the player, as data on
     the chip, and its cause still does not. Not contradicted.
   - VTT-662's run, and SPEC-022's consequence that a correction's sheet
     frames end a run, are kept by D2.
6. **The records the work moves are named. By command, then reading.** The
   section is present and its one path resolves; nothing is `New:`. By
   search, SPEC-022 is the only specification that says what the Story
   draws (`grep -n Story docs/specifications/*.md` finds no other);
   SPEC-016 describes `statusFrames` and `doorTransitions`, which do not
   change; SPEC-007 names `feed.ts` only for how a narration is grouped,
   which does not change.

**Split: not offered.** Every left-out kind leaves the Story by one table
in one function, and the tests that change for each share helpers
(`expectTold`, `OPENING_STORY`, `storyRows`).

## The gaps this plan inherits

- **G1. Tests the ticket does not name.** P1, P7 and P9 name every test
  that turns red; the ticket names the files "among them".
  `client/test/naming.test.ts` changes in a citation alone (VTT-622,
  VTT-623); `client/test/qa-ability-change.test.ts`,
  `client/test/qa-resource-change.test.ts`, and "qaWS: a player's Story
  words each case of the table" and "qaWS: a DM's Story words by the GM
  chain" in `client/test/qa-what-a-window-shows.test.ts` stay green, but a
  test's name or citation in each becomes false. `client/test/qa-give-value.test.ts`
  cites VTT-622 and VTT-623 in comments. The sort is below.
- **G2. Rows the ticket does not name.** VTT-734 and VTT-759 stay true (the
  Story still names an ability, in its use line) but their Story evidence
  reads give lines. VTT-475's and VTT-484's evidence includes tests that
  read a token's placing in the Story. VTT-393, VTT-445, VTT-446 and
  VTT-447's evidence includes tests that count a grant, a placing or a move
  among the Story's rows. VTT-733, VTT-695, VTT-732 and VTT-758's evidence
  includes tests that also read a give line in the Story. VTT-475's and
  VTT-476's evidence includes tests whose names say the Story words a
  left-out kind. Each stands; each named test changes ("Requirements"
  below).
- **G3. A second decision overturned:** OD-4 of the 2026-11-15 plan, signed
  off by the owner on 2026-11-15, took "A: the Story words each new event
  (D9); the player's message names abilities only … the player's log,
  forwarded the resource event, shows it in the Story". After this work a
  player's window shows a resource that a give brought only as a chip.
- **G4. No row says a narration is never dropped.** Item 7 holds today for
  a run (`buildFeed`'s "Never drop a narration" path) by tests no row
  cites. R4 adds the row. After this work it holds for every narration
  anchored to moves alone, the common case.
- **G5. How control's leaving meets `restates`.** D2 settles it: `restates`
  keeps `actorControlGranted`, because a run's conditions sit behind its
  grants.
- **G6. The ticket's "What could not be established" travels:** a grant has
  no message for the player (OD-4); a party member with no token shows its
  values nowhere in a player's window; an ability's use line names no cost
  and no damage, so a wound reaches the window as the chip and the DM's
  Events panel alone.
- **G7. Who opened a door.** The door line names no one; the opener is a
  ticket of its own, by the owner's ruling.
- **G8. A run at a companion's first sight now draws no line.** A party
  member held unseen whose sheet holds attributes or module data is
  introduced again at a character log's first sight; the run is left out
  (VTT-662, R3), its grant is left out (R2) and the token's arrival is a
  `tokenPlaced`, left out too (R10), so the Story draws no line for it, and
  the board draws its token. One whose sheet holds neither is corrected
  rather than introduced again: it draws no line of its sheet or its
  control (item 2), but each condition it holds is a bare
  `conditionApplied` and is drawn ("<actor> gained <condition>"), as it is
  today. A creature coming into sight still draws its `actorAdded` line.
- **G9. The end-to-end suite.** `client/e2e/table.spec.ts` "the story feed
  and ticker record what happened" expects a player's Story to hold "Lera is
  at <x>,<y>". Other specs expect a perched window's Story to hold some
  mechanical line (`handover.spec.ts`, `table.spec.ts`); by reading, each
  such log holds a kept kind (an `actorAdded`), and `task e2e` in Task 2 is
  the observation.

## Measurements this plan stands on

- **P1. Which tests the sheet's and control's kinds turn red.** On
  2026-11-16, in a copy of the tree at `92cb3e3` in the session's
  scratchpad (the client suite green there first), `buildFeed` was made to
  drop every entry of those kinds after `reintroductions`, and
  `env -u CLAUDECODE bun test test` was run from the copy's `client/`.
  Superseded in its list by P7, which measured the whole amended filter;
  P1's reading of what each test asserts is the basis of "The existing
  tests that change".
- **P2. Items 1, 2 and 7 fail today.** In the same copy, a probe test file
  drew, through `renderSpectator`, a log holding each of the sheet's and
  control's kinds with an event id and with none, in a window with and
  without the Events panel; a log in which a party member added with no
  sheet is corrected by a bare `resourceAdded` and a bare `abilityGiven`
  before its token arrives; and a narration anchored only to a
  `resourceChanged`. On `92cb3e3` each fails ("Ash hp → 4", "Ash now has hp
  at 5", "Ash can now use Slash" drawn; the narration shares the change's
  beat). With P1's filter each passes, the Events panel drawing a row for
  each entry.
- **P3. MapTool (rule 9), a sheet and control.** In `~/dev/RPTool/maptool`:
  neither `TokenPropertyFunctions.java` nor `EditTokenDialog.java` calls
  `addMessage` or `addLocalMessage`. Every file that changes a token's
  owners posts no chat for it; `ZoneRenderer`'s one message there is
  `TextMessage.gm` "Token.dropped.byPlayer", to the GM alone, when a player
  drops a token.
- **P4. Callers,** by `grep -n` over `client/src`: as in check 4.
- **P5. A withdrawn id is cited nowhere in code.** `git grep` for VTT-449,
  VTT-697, VTT-713, VTT-714, VTT-728, VTT-739, VTT-740, VTT-745 and VTT-757
  under `client`, `internal`, `cmd` and `tools` finds nothing. The house
  practice is that a test citing a withdrawn row cites its successor.
- **P6. Comment ceilings.** `tools/comment-ceilings.txt` holds
  `client/src/view/feed.ts` and `client/src/view/spectator.ts`; added code
  lowers a share, added comment lines raise it.
- **P7. Which tests the amended filter turns red.** On 2026-11-16, in a
  fresh copy of the tree at `92cb3e3` in the session's scratchpad
  (`client`, `contract`, `campaigns`, `scenarios`, the root `package.json`
  and the worktree's `node_modules`; the client suite green there first),
  `buildFeed` was made to drop every entry of a left-out kind and every
  `doorOpened` and `doorClosed` with no event id after `reintroductions`,
  and `renderFeed` to word a door as "a door opened" or "a door closed".
  What turned red, by file:
  - `app.test.ts`: "no role's window shows a log sequence number", "a
    player's and a spectator's window draws no Events panel, and its Story
    shows what it was sent", "a page whose /api/me answers no role draws no
    Events panel, and its Story shows what it was sent", "a DM's and an
    agent's window draws a row in the Events panel for each entry it was
    sent, newest first" and "the ruleset's properties reach the chip, the
    Story and the cost".
  - `feed.test.ts`: every test whose fixture's mechanical entries are
    `moved` (`tokenMoved`): "entries are chronological", the anchoring
    tests, "overlapping anchors resolve last-writer-wins", "entries are
    ordered ascending even when the log arrives shuffled", "an anchor whose
    range is empty does not capture the next event after it" and "an unset
    payload alongside real events leaves the real ones intact".
  - `qa-companion-first-sight.test.ts`: the run's unit tests through
    `expectTold`, and the window tests "<role>: the Story draws the token's
    arrival, not the run", "<role>: a session's window does not tell the run
    at a first sight" and "<role>: entries arriving one at a time end with
    the run untold".
  - `qa-events-panel.test.ts`: "qaEP: <role>: the ticker and the feed label
    a move with its reason", "qaEP: <role>: the Story labels a move with its
    reason", "qaEP: <role>: the window shows no log sequence number", "qaEP:
    <role>: the Story still shows every entry of the log, more than 40
    included", "qaEP: <role>: the Story shows every entry of the log while
    the panel shows 40", and the tests on a role read late or never, which
    count `SETUP`'s entries among the Story's rows.
  - `qa-move-reason.test.ts`: every test of the describe "renderSpectator:
    the feed and the ticker".
  - `qa-older-answer.test.ts`: every test of the describe "two
    /api/ruleset reads in flight in a player's window", and "an older read's
    answer arriving after the newest does not change how the Story names a
    resource".
  - `qa-property-names.test.ts`: "the Story line of a declared resource's
    change has its whole form", "the Story line of an undeclared resource's
    change names its key", "with no properties the Story line names a
    resource by its key", "the Story line names the actor by token name,
    party name, or not" and "a number property does not name a Story line of
    the same key".
  - `qa-sheet-news.test.ts`: "VTT-732 VTT-733 VTT-734 VTT-695 a give and a
    take re-read the ruleset, then tell the player and change the panel".
  - `qa-what-a-window-shows.test.ts`: "qaWS: VTT-475 VTT-477 a player's
    Story shows the window's naming" and "qaWS: VTT-484 VTT-477 a DM's Story
    shows the GM chain".
  - `sheet-news.test.ts`: "a player is told what a give and a take changed
    once the read it issued answers, a taken ability named by an earlier
    answer" and "an older read's answer arriving after the newest names an
    ability in the Story at once, and leaves the message standing".
  - `spectator-view.test.ts`: "the feed and the ticker show a move's
    reason", "a window not asked for the Events panel draws none, and its
    Story still shows the log" and "at a companion's first sight the Story
    shows it coming into view and neither its leaving nor its joining".
  - Nothing else turned red. `dm-view.test.ts`, `fold-rejections.test.ts`,
    `fold-unit.test.ts`, `join.test.ts`, `player.test.ts`,
    `qa-describe-token.test.ts` and `qa-token-texts.test.ts` hold a left-out
    kind's words and stayed green: they read `describe`, the Events panel or
    the fold.
- **P8. MapTool (rule 9), the board and the door.** By `grep -rl` for
  `addMessage` and `addLocalMessage` over its sources, crossed with the
  files that move a token: MapTool posts no chat line of its own when a
  token moves. What reaches the chat there is what an event macro a
  campaign runs on the move returns (`EventMacroUtil`), a connection's
  coming and going (`ClientMessageHandler`), and, near a token's arrival,
  `ZoneRenderer`'s GM line "Token.dropped.byPlayer" and the paster's own
  error ("Token.error.unableToPaste", in `ZoneRenderer` and `AppActions`).
  MapTool's core has no door: `ZoneRenderer` names one only in a comment on
  a stamp that blocks vision, and `DungeonDraftImporter` imports doors as
  topology.
- **P9. What a token's going out of sight turns red.** On 2026-11-16, in
  the same copy with P7's filter and the tests rewritten for it,
  `tokenHidden` was left out too. "qaWS: VTT-475 VTT-477 a player's Story
  shows the window's naming" and "qaWS: VTT-484 VTT-477 a DM's Story shows
  the GM chain", which the rewrite had pointed at "<token> went out of
  sight", turned red, and so did D5's walk; nothing else did.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no assertion is weakened to pass: an assertion on
  a Story line of a left-out kind becomes an assertion that the line is
  absent, or moves to the Events panel or `describe`, and a count of Story
  rows becomes the count of entries the Story draws, with the left-out
  line asserted absent); 4 (the fold is untouched: `buildFeed` is a view);
  5 (no game word in code or tests under `internal/` and `cmd/`; this work
  is in `client/` alone); 8 and 10 (names, never line numbers; a comment is
  a warning or a pointer, and every comment this change makes false is
  rewritten: `feed.ts`'s header, the "Never drop a narration" comment,
  `reintroductions`' doc, and the comments in `feed.test.ts` that call a
  move the beat a narration groups); 9 (answered below).
- **Blueprint principles 12, 13 and 17**, read above; the change shows the
  table nothing it did not show before.
- **SPEC-022's** "the fold, the namer and the Events panel are handed the
  whole log" stays true: only the Story's entries are filtered.
- **SPEC-020, SPEC-023.** `tools/ts-mutation-equivalents.txt` adjudicates
  mutants in `feed.ts` and `spectator.ts` by the code they sit on; no
  adjudicated line is edited by D1 or D9. The client's stored mutation
  report goes out of date with the first client commit; `task check` names
  that and passes; a full run is the owner's to ask for.
- **The project's mechanical gates:** a client change is committed with its
  `cmd/vtt/webdist`, rebuilt with `task build:client`; client tests are run
  with `env -u CLAUDECODE`; `task client:typecheck`; `tools/check-new-prose.py`
  and `python3 tools/check-comments.py` run before each commit; stage and
  commit in separate calls.

## Rule 9: how MapTool does this

P3 and P8, beside the ticket's paragraph:
- A token's property changes, by macro or by dialog, post nothing to the
  chat; the chat holds what is said and what a macro prints.
- A token's ownership change posts nothing to the chat, and a token's move
  posts no line of its own: what reaches the chat on a move is what an
  event macro the campaign runs returns (`EventMacroUtil`). Near a token's
  arrival `ZoneRenderer` tells the GM alone that a player dropped a token,
  and the paster sees its own error.
- A door is a macro's, and what it prints reaches the chat under the token
  the player speaks as.
- **Borrowed:** a sheet's change, a change of who controls a character and a
  token's move are not chat lines, and what is mechanical goes to the GM's
  view. The Story keeps what a use of an ability prints, and a door as an
  event.
- **Not borrowed, and not needed here:** MapTool's distribution; nothing in
  this work changes what a connection is sent.

## Decisions

1. **D1. Where the Story leaves them out: `buildFeed`, after
   `reintroductions`.** `buildFeed` drops each entry of a left-out kind, and
   each `doorOpened` and `doorClosed` with no event id, from the log it
   groups. `describe`, `renderTicker` and `renderSpectator` do not change.
   - **Forced by** `spectator.ts`'s header, which warns that every
     decision in the view lives in `grid.ts` and `feed.ts`, tested without a
     browser; and by the narration rule: `buildFeed`'s "Never drop a
     narration whose range names no entry kept here" path already draws a
     narration anchored only to left-out entries in a beat of its own, at
     its own place (item 7, P2), with no new code. Filtering in
     `renderFeed` would leave the narration in the left-out entry's beat
     and draw an empty beat for every beat of left-out entries alone.
2. **D2. Runs are found in the whole log, before the left-out kinds
   leave.** `reintroductions` and `restates` do not change, and `restates`
   keeps `actorControlGranted`.
   - A run's extent is defined over the log's entries (VTT-662: "directly
     follows", "directly behind it"), in the order `introduce` writes
     (SPEC-016): grants, then conditions. Dropping grants from `restates`
     would end a run at its add and draw the conditions that restate it
     ("Ash gained blessed" at a first sight); keeping it costs nothing,
     since a grant is left out whether or not it is in a run.
   - Filtering before finding runs would also change which entries end a
     run: a correction's sheet frames end one today (SPEC-022,
     Consequences), so the conditions behind them are drawn. Keeping that
     is the smaller change and the ruling does not ask for another.
   - **Forced by** VTT-662's run definition and SPEC-016's order.
     `qa-companion-first-sight.test.ts`'s "a bare grant after a resource
     change ends the run and is told", rewritten and renamed "a resource
     change ends the run, and the bare condition behind it is told", is its
     guard.
3. **D3. The Events panel and `describe` do not change.** `renderTicker` is
   handed the whole log as today; `describe` keeps every word, a door's
   square and a move's reason included, so item 5 holds by construction and
   VTT-447 and VTT-448 stand.
   - **Forced by** the ticket's item 5 and the rulings.
4. **D4. The list's form: a table over every case of `Envelope.payload`**
   (OD-1 A), true for the kinds the Story draws and false for the left-out
   kinds, typed so that a payload added to the contract fails
   `task client:typecheck` until somebody decides whether the Story draws
   it. A payload whose case is unset is drawn as today ("something
   happened"); the table holds the cases the contract names.
5. **D5. The new checks.** In `client/test/feed.test.ts`, units of
   `buildFeed`: one walk over every case of `Envelope.payload` (as
   `spectator-view.test.ts`'s "describe renders a real label for every
   event kind, not the fallback" walks them), each with an event id and
   with none, keeps every kept kind and a door with an event id, and
   leaves every left-out kind and a door with none out ("buildFeed keeps
   each kind the Story draws and leaves out the rest, an entry with an
   event id and one with none"); a narration anchored only to left-out
   entries is its own beat; a run followed by a correction's sheet frames
   and a bare condition draws the condition. In `client/test/app.test.ts`, a
   booted window of each role (`/api/me` answering `dm`, `agent`, `player`,
   `spectator`) is sent, with an event id, every left-out kind but
   `tokenRemoved` and `tokenHidden`, and a session's start and end, an add,
   a condition gained and lost, a use, a roll, a whisper and both doors;
   then, bare, every left-out kind and both doors. Its Story draws the kept
   entries with an event id and nothing else, and the DM's and the agent's
   Events panel draws a row for each in `describe`'s words (items 1, 3, 5
   and 6). In
   `client/test/spectator-view.test.ts`, a character log in which a party
   member held unseen with no sheet is corrected by bare `resourceAdded` and
   `abilityGiven` frames, in `statusFrames`' order (resources, conditions,
   abilities, grants), before its token arrives, draws no "now has", no
   "can now use" and no control line (item 2); a door opened and closed
   with an event id draws "a door opened" and "a door closed" and no
   square, a bare one draws nothing, and the Events panel draws each with
   its square (item 4). Item 6's other kept lines, an actor coming into
   sight, joining the party and leaving, and a narration, are held by the
   walk and by the existing tests that draw them, which stay green.
   - **Forced by** the ticket's items; the walk because a per-kind table in
     the code is otherwise held only for the kinds a test names.
6. **D6. How the existing tests change** is "The existing tests that
   change" below: a Story assertion on a left-out kind becomes its absence,
   or moves to the Events panel or `describe`; a Story assertion on an
   ability's display name moves to that ability's use line; a Story
   assertion that reads a token's name in a placing moves to a kept line
   that names the token or the actor (`abilityUsed`, `conditionApplied`); a fixture whose moves stand for "a mechanical
   entry" takes a kept kind instead, its claim unchanged; a test's name that
   says a left-out kind is told or drawn in the Story is renamed, and the
   register's evidence follows the rename.
   - **Forced by** rule 2, and by G2's rows, which stay true and keep their
     evidence.
7. **D7. The register,** under OD-2 and OD-3: rows made false are
   withdrawn in Task 1, the commit that makes them false, each with its
   successor; tests citing a withdrawn id cite its successor (P5).
8. **D8. SPEC-022 last,** after the report, from the report, per the
   project's order.
9. **D9. The Story's words for a door are `renderFeed`'s.** `renderFeed`
   draws a `doorOpened` as "a door opened" and a `doorClosed` as "a door
   closed", and every other entry with `describe`; `describe` keeps the
   square for the Events panel (D3).
   - **Forced by** the ruling (the Story's door is an event, not a square)
     and D3 (the Events panel unchanged). A function beside `describe` that
     `renderFeed` calls keeps one switch of words per surface.
10. **D10. A door with no event id is the projection's belief, and leaves
    the Story.** In a character log an entry with no event id is the
    projection's own, or an opening's or a resume's entry (SPEC-016,
    SPEC-021); `doorTransitions` (`internal/perceive/projection.go`) writes
    one when a door it sees differs from what the log believed, and a
    forwarded door carries its event id. The world log's entries all carry
    one: `Store.AppendBatch` refuses an envelope whose event id is empty
    ("store: envelope event_id must not be empty").
    - **Forced by** the ruling: a door the character discovers open did not
      open in its sight, and the board draws it open.
11. **D11. A token going out of sight leaves the Story.** `tokenHidden` is
    a left-out kind.
    - **Forced by** the owner's ruling, and by its writers (check 1): no
      command writes one, and `resume` writes one for every token a resumed
      log holds before the projection places what the character sees
      (VTT-318). With the placing left out, the Story would draw "<token>
      went out of sight" for tokens still on the board, and never their
      return, which is a `tokenPlaced`.

## The owner's answers

- **OD-1. The list's form: A,** a table over every case of
  `Envelope.payload` (D4).
- **OD-2. VTT-736 and VTT-790: A,** each withdrawn and succeeded by a row
  that says the Events panel words them so (R5, R6).
- **OD-3. VTT-756: succeeded by the ability button's title** (R9). The
  owner ruled that a player looks at the panel, not at the token on the
  board, and that the sheet's place is the player's panel (the next
  ticket); the ability button's title names the resource the ability spends
  by the ruleset's name today (`renderPlayerPanel`, `names.resource`), so
  the row holds there. Once the Story names no resource, VTT-756 is true of
  nothing; VTT-753 holds the panel's abilities and not the window's
  properties, so without a successor the properties half is held by no row.
- **OD-4. A grant's message: none.**

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill; the
register's words are the ones that bind. Every new row reads
`**OPEN — no test yet**` until Task 1 cites its tests.

| R | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|
| R1 | 1, 2 | The Story draws no line for a `resourceChanged`, an `abilityGiven`, an `abilityTaken`, a `resourceAdded` or a `resourceRemoved` in the log its window holds, whatever the participant's role, whether the entry carries an event id or none | one of them drawn |
| R2 | 1 | The Story draws no line for an `actorControlGranted` or an `actorControlRevoked` in the log its window holds, whatever the participant's role, whether the entry carries an event id or none | either drawn |
| R3 | 6 | The Story shows every kept kind's entry of the log its window holds, a door's only while it carries an event id, whatever the participant's role, except while the window shows the reason it cannot fold its log, and except each `actorRemoved` with no event id that the `actorAdded` of the same actor directly follows, that `actorAdded`, and that actor's `actorControlGranted` and `conditionApplied` entries with no event id directly behind it, the runs found in the whole log (succeeds VTT-662) | a kept kind left out, a run told, or a correction's condition behind sheet frames left out |
| R4 | 7 | A narration anchored only to entries the Story leaves out is drawn, in a beat of its own at its own place in the log | dropped, or drawn in a left-out entry's beat |
| R5 | 5, OD-2 | The Events panel words an `AbilityGiven` as "<actor> can now use <ability>" and an `AbilityTaken` as "<actor> can no longer use <ability>" (succeeds VTT-736) | other words |
| R6 | 5, OD-2 | The Events panel words a `ResourceAdded` as "<actor> now has <resource> at <current>" and a `ResourceRemoved` as "<actor> no longer has <resource>" (succeeds VTT-790) | other words |
| R7 | 5 | A browser window names a resource its ruleset declares as a resource property by that property's display name on a resource chip's title, on the Events panel's line of a change to that resource and on an ability's cost (succeeds VTT-622) | a key shown |
| R8 | 5 | A browser window names a resource its ruleset does not declare as a resource property, or any resource while it has no ruleset, by its key on a resource chip's title and on the Events panel's line of a change to it (succeeds VTT-623) | a display name shown |
| R9 | OD-3 | An answer to an older `/api/ruleset` read arriving after the answer to a player's window's newest read leaves unchanged the name by which the window's player panel titles an ability with the resource it spends (succeeds VTT-756) | the older answer's name on the button |
| R10 | 3 | The Story draws no line for a `tokenPlaced`, a `tokenMoved`, a `tokenRemoved`, a `tokenDescribed`, a `sceneCreated`, a `sceneSeen`, a `noteUpserted`, a `noteDeleted` or an `adventureLoaded` in the log its window holds, whatever the participant's role, whether the entry carries an event id or none | one of them drawn |
| R11 | 4 | The Story draws a `doorOpened` that carries an event id as "a door opened" and a `doorClosed` that carries one as "a door closed" | a square named, or other words |
| R12 | 4 | The Story draws no line for a `doorOpened` or a `doorClosed` with no event id | either drawn |
| R13 | 5 | The Events panel labels a move with its reason when its frame carries one (succeeds VTT-263) | the reason missing |
| R14 | 3 | The Story draws no line for a `tokenHidden` in the log its window holds, whatever the participant's role | one drawn |

**Rows withdrawn, in Task 1, each with its successor:** VTT-662 (R3),
VTT-736 (R5), VTT-790 (R6), VTT-622 (R7), VTT-623 (R8), VTT-756 (R9),
VTT-263 (R13). Each reads "WITHDRAWN 2026-11-16: <its words>. False as
worded once the Story draws no line for …. Succeeded by …", its evidence
`**READING — <the review>; withdrawn, held by nothing**`, as the register's
recent withdrawals do. VTT-811 is withdrawn the same way in Task 0's
commit, succeeded by VTT-822 (R3) and VTT-823 (R14).

**Rows checked and left standing,** their evidence cells updated where a
named test is renamed: VTT-393, VTT-445, VTT-446, VTT-447, VTT-448,
VTT-475, VTT-476, VTT-477, VTT-484, VTT-502, VTT-597, VTT-695, VTT-732,
VTT-733, VTT-734, VTT-752, VTT-753, VTT-758 and VTT-759. VTT-734 and
VTT-759 stay true through the ability's use line, which `names.ability`
names as `describe` does. VTT-475 and VTT-484 stay true through the kept
lines that name a token or an actor.

## The existing tests that change

By P1, P7 and by reading every test that names a left-out kind's words;
each is re-run by Task 1 before and after.

**Read the Story, and change:**
- `client/test/app.test.ts`:
  - "the ruleset's properties reach the chip, the Story and the cost":
    drops its Story assertion and its name's "the Story"; the Events
    panel's half is `spectator-view.test.ts`'s "the Events panel names a
    declared resource by its display name". Cites R7.
  - "no role's window shows a log sequence number", "a player's and a
    spectator's window draws no Events panel, and its Story shows what it
    was sent" and "a page whose /api/me answers no role draws no Events
    panel, and its Story shows what it was sent": the Story's row count
    leaves out every left-out entry, and "control of Lera given" and each
    left-out line of the fixture are asserted absent. Cite R3 for VTT-662.
  - "a DM's and an agent's window draws a row in the Events panel for each
    entry it was sent, newest first": compares the panel with `describe`
    of each entry sent, newest first, not with the Story's rows reversed.
- `client/test/feed.test.ts`: the `moved` fixture becomes a kept kind (a
  roll, `attackRolled`), every test's claim unchanged, and the comments that
  call it a move follow.
- `client/test/qa-companion-first-sight.test.ts`: `expectTold` and
  `expectedSequences` treat an entry of a left-out kind as left out;
  `OPENING_STORY` loses its scene, "control of … given", placing and view
  lines, and `BASE_STORY` is `OPENING_STORY`, since the step, the view and
  Ash's arrival draw none; `BASE_EVERY_LINE` keeps every line of the base
  log for the Events panel's expectations, built on a new
  `OPENING_EVERY_LINE`; each `count(…, "control of Ash given")`
  expectation on the Story becomes 0. "a narration between the removal and
  the add: the run is told", "a narration anchored to the run alone is
  still drawn" and "a narration anchored over the run and the arrival is
  drawn with it", renamed "… is drawn in a beat of its own", read the
  Story without the left-out lines; the last two are R4's evidence. "<role>: the Story draws the token's
  arrival, not the run" becomes "draws neither the run nor the arrival"
  (G8), the arrival's line asserted absent and the token asserted on the
  board. Renamed, since their names say a grant is told: "the opening's
  introduction and grant of a companion are told", "a grant behind the add
  that carries an event id is told", "another actor's grant behind the add
  is told, and what follows it", "two companions' grants behind both adds
  are told". "a bare grant after a resource change ends the run and is
  told": both are left out and the condition behind them is told; it is
  D2's guard, renamed "a resource change ends the run, and the bare
  condition behind it is told". Citations of VTT-662 become R3's.
- `client/test/qa-events-panel.test.ts`: each expectation counting
  `SETUP`'s entries among the Story's rows counts those the Story draws;
  "the Story still shows every entry of the log, more than 40 included" and
  "the Story shows every entry of the log while the panel shows 40" are
  renamed to say every entry it draws, and their fixture past 40 rows, and
  that of "neither the ticker's rows nor the Story's show a log sequence
  number", is a kept kind (`gains`, conditions Bram gains) in place of
  moves, so the Story still draws more than 40 rows. "the ticker and the feed label a
  move with its reason" keeps the ticker's half and asserts the move absent
  from the Story (R13); "the Story labels a move with its reason" (a
  player's and a spectator's window, which draw no panel) asserts the move
  absent from the Story. The panel's own expectations do not change.
- `client/test/qa-move-reason.test.ts`, the describe "renderSpectator: the
  feed and the ticker": each test keeps its ticker half and asserts the
  move absent from the Story; the markup and entity tests read the ticker's
  row; renamed for the Events panel, citing R13.
- `client/test/qa-older-answer.test.ts`: the `Table` gains a use of an
  ability; `story(actor, ability)` reads that use line ("<actor> used
  <ability>"), so each test of the describe "two /api/ruleset reads in
  flight in a player's window" asserts the ability's name in the Story
  through a use after its give. "an older read's answer arriving after the
  newest does not change how the Story names a resource" reads the ability
  button's title in the player panel (R9), and is renamed for it.
- `client/test/qa-property-names.test.ts`: `qaPNStoryOf` becomes a helper
  that draws a DM's window with the Events panel, asserts the Story draws
  no line for the change, and returns the panel's row, checked against
  `describe`. "the Story line of a declared resource's change has its whole
  form", "the Story line of an undeclared resource's change names its
  key", "with no properties the Story line names a resource by its key"
  and "a number property does not name a Story line of the same key" are
  renamed for the Events panel and cite R7 or R8. "the Story line names the
  actor by token name, party name, or not" (VTT-475, a player's window,
  which draws no panel) reads a condition gained, a line the Story keeps.
- `client/test/qa-sheet-news.test.ts`, "VTT-732 VTT-733 VTT-734 VTT-695 a
  give and a take re-read the ruleset, then tell the player and change the
  panel": each `storyText` expectation of a give or a take line becomes its
  absence (R1); the toast expectations do not change.
- `client/test/qa-what-a-window-shows.test.ts`: "qaWS: VTT-475 VTT-477 a
  player's Story shows the window's naming" and "qaWS: VTT-484 VTT-477 a
  DM's Story shows the GM chain" read the token's name in the use line
  alone, and assert neither its placing nor its going out of sight drawn.
- `client/test/sheet-news.test.ts`: "a player is told what a give and a
  take changed once the read it issued answers, a taken ability named by
  an earlier answer": its `.feed` expectation becomes the line's absence.
  "an older read's answer arriving after the newest names an ability in
  the Story at once, and leaves the message standing": a use of the given
  ability is sent after the give, and the Story's expectations read the
  use line.
- `client/test/spectator-view.test.ts`: "at a companion's first sight the
  Story shows it coming into view and neither its leaving nor its joining"
  draws no line for the scene, the first add's grant or the arrival (G8),
  words the door as an event, asserts the token on the board, and is
  renamed; "a window not asked
  for the Events panel draws none, and its Story still shows the log"
  counts the entries the Story draws; "the feed and the ticker show a
  move's reason" keeps the ticker's half and asserts the move absent from
  the Story, citing R13.
- `client/e2e/table.spec.ts`, "the story feed and ticker record what
  happened": the player's Story expects a kept line of Lera's log in place
  of "Lera is at <x>,<y>", and the placing line asserted absent.

**Stay green, but a name or a citation becomes false, and changes:**
- `client/test/qa-ability-change.test.ts`, "VTT-736 the Story words an
  abilityGiven …" and "VTT-736 the Story words an abilityTaken …": they
  read `describe`; renamed for the Events panel, citing R5. The id in each
  name is a citation too, so the new name carries R5's id, not VTT-736.
- `client/test/qa-resource-change.test.ts`, the describe "the Story's line
  of a resource event" and its comment: they read `describe`; renamed for
  the Events panel, citing R6.
- `client/test/spectator-view.test.ts`: "describe renders a real label for
  every event kind, not the fallback" cites R8, R5 and R6 for VTT-623,
  VTT-736 and VTT-790; "a resource chip, its Story line and an ability's
  cost name the resource by its display name" reads `describe` and is
  renamed for the Events panel line (R7).
- `client/test/qa-what-a-window-shows.test.ts`: "qaWS: a player's Story
  words each case of the table" and "qaWS: a DM's Story words by the GM
  chain" read `describe`; renamed for `describe`.
- Every test P7 found green that holds a left-out kind's words under a name
  saying "Story" or "feed", found by `git grep` in Task 1, is renamed the
  same way.

**Cite a withdrawn id in a comment, and change in that citation alone:**
`client/test/naming.test.ts` ("a resource is named by the ruleset's display
name, else its key", VTT-622 and VTT-623 to R7 and R8);
`client/test/qa-give-value.test.ts` (VTT-622 and VTT-623 to R7 and R8);
and every other citation of a withdrawn id above a test whose assertions
stay, each to its successor, found by the `git grep` Task 1 names.

**Read `describe`, the Events panel or `sheetNews`, and stay as they are:**
`client/test/naming.test.ts`'s assertions (the namer and `sheetNews`); the
toast expectations of `sheet-news.test.ts`, `qa-sheet-news.test.ts` and
`qa-older-answer.test.ts`; `spectator-view.test.ts`'s Events panel tests
("the ticker shows the newest event first", "each ticker row is its event's
description alone, newest first", "the Events panel names a declared
resource by its display name"); `qa-events-panel.test.ts`'s panel
expectations; every fold test; the other `client/e2e/` specs, whose Story
expectations are the use line, a narration, a whisper and a non-empty
Story (G9).

## Tasks

### Task 0: the records, the ids

- **Do:** run the `requirements` sort over R1 to R14; allocate ids with
  `requirement-id`; withdraw VTT-811 (under "Ids"); commit the ticket,
  this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: the Story leaves out the left-out kinds (R1 to R14; D1 to D11; items 1 to 8)

- **Files:** `client/src/view/feed.ts` (`buildFeed`, the table under D4,
  the header comment, the "Never drop a narration" comment and
  `reintroductions`' doc rewritten as warnings and pointers);
  `client/src/view/spectator.ts` (`renderFeed`'s door words, D9);
  `client/src/view/grid.ts` (`label`'s doc points at R7 and R8);
  `client/test/feed.test.ts`, `client/test/app.test.ts` and
  `client/test/spectator-view.test.ts` (D5's new tests, written first and
  seen red on the base); every test "The existing tests that change"
  names; `client/e2e/table.spec.ts`; `docs/requirements.md` (the
  withdrawals, the new rows' evidence, the renamed tests in standing rows'
  evidence); a rebuilt `cmd/vtt/webdist`.
- **True when:** D5's tests pass and failed before the change to
  `feed.ts` and `spectator.ts`; `env -u CLAUDECODE bun test client/test`
  passes; `task client:typecheck` passes; `git diff --stat main --
  client/src` lists `client/src/view/feed.ts`,
  `client/src/view/spectator.ts` and `client/src/view/grid.ts` alone, the
  last a comment; `git grep -nE
  'VTT-(263|622|623|662|736|756|790|811)([^0-9]|$)' -- client internal cmd tools`
  prints nothing (not `\b`, which `git grep -E` does not read as a
  boundary, so a pattern with it prints nothing on today's tree too);
  `task check:requirements-chain` passes; `task build:client` has run
  after the last change to `client/src` and its output is in the commit;
  `tools/check-new-prose.py` and `python3 tools/check-comments.py` pass.
- **Deliberate breaks:**
  - Mark `abilityTaken` drawn: D5's walk and the booted window test turn
    red (R1). Do the same for
    `actorControlRevoked` (R2) and `tokenMoved` (R10).
  - Mark `tokenHidden` drawn: the walk, the booted window test and the two
    qaWS tests P9 names turn red (R14).
  - Mark `conditionApplied` left out: the walk and item 6's window test
    turn red (R3).
  - Keep a door with no event id: D5's door tests turn red (R12). Leave a
    door with an event id out: they turn red too (R3, R11).
  - Word the Story's door with `describe`: the door window test turns red
    (R11).
  - Filter the left-out kinds before `reintroductions`: "a resource change
    ends the run, and the bare condition behind it is told" and D5's
    correction test turn red (D2).
  - Remove `actorControlGranted` from `restates`: the first-sight tests
    expecting no "Ash gained blessed" turn red (D2).
  - Drop a narration whose anchor finds no kept entry: item 7's test and
    "a narration anchored to the run alone is still drawn" turn red (R4).
  - Hand `renderTicker` the Story's entries in place of the log: item 5's
    booted window test turns red (D3).
  - Leave `resourceAdded` drawn: item 2's first-sight test turns red.
  - Name a resource by its key in `describe`'s `resourceChanged` arm: the
    re-pointed `qa-property-names.test.ts` tests turn red (R7). Name an
    undeclared resource by anything but its key: R8's tests turn red.
  - Change `describe`'s words for an `abilityGiven`, then for a
    `resourceAdded`: R5's, then R6's tests turn red.
  - Let an older answer set the window's properties: R9's test turns red.
  - Drop a move's reason in `describe`: the re-pointed move-reason tests
    turn red (R13).
  - Name an ability in `abilityLine` from the newest answer alone: the
    rewritten `qa-older-answer.test.ts` and `sheet-news.test.ts` Story
    expectations turn red (VTT-734, VTT-759).
- **The state here:** every role's Story draws no line of a left-out kind
  and no door the projection corrected, and words a door as an event; the
  Events panel, the toasts, the chips and the panel's costs are as before;
  nothing on the wire or in any log changes.

### Task 2: the gate (item 9)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger` once,
  last, if a ceiling can come down; check free space and the load
  (`os.getloadavg()`); run `task check` whole on the last code commit,
  detached under `/bin/sh`, output to a file, requiring its completion
  line; run `task e2e`. Tell the owner the client's stored mutation report
  is out of date from Task 1 and which commit it measured (SPEC-023); a
  full Stryker run is his to ask for.
- **True when:** `task check` and `task e2e` are green.

### Task 3: the report, then the specification

- **The report:** `docs/reports/2026-11-16-the-story-writes-nothing-from-the-sheet.md`
  with the `implementation-report` skill, and its reading review: each
  "Done looks like" item with its observation; R1 to R14 and what turned
  each red; Rule 9; G1 to G9 and each OD's answer; Phase 4a's
  adjudications under their own heading; the gate.
- **SPEC-022**, with the `specification` skill, from the report:
  - **Status:** the Story shows the kept kinds and leaves out the left-out
    kinds, a corrected door and the runs (R1 to R4, R10 to R12 and R14 for
    VTT-662); the Events panel words a give, a take, a resource added and
    removed, and a move's reason (R5, R6, R13 for VTT-736, VTT-790,
    VTT-263); an older answer and the ability button (R9 for VTT-756); the
    resource names (R7, R8 for VTT-622, VTT-623).
  - **What the window shows:** `buildFeed` leaves out the left-out kinds
    and a door with no event id after the runs, found in the whole log
    (D2); "So the Story of a window that holds the world log leaves nothing
    out" is rewritten; a narration anchored only to left-out entries is its
    own beat (R4); "nor its grants' … lines" follows; a run at a
    companion's first sight draws no line (G8).
  - **The answers paragraph:** "the window's properties, by which the Story
    names a resource" becomes the ability button's title (R9).
  - **How the window names what it shows:** "The Story's and the Events
    panel's line of a `resourceChanged`" becomes the Events panel's.
  - **The Story's lines:** the left-out kinds are the Events panel's words,
    by the owner's rulings of 2026-11-16, overturning OD-4 of 2026-11-14
    and of 2026-11-15; an ability's use, a condition, a roll and a door as
    an event stay the Story's.
  - **Consequences:** "the window's Story shows each of them" (the Events
    panel bullet); "An `abilityGiven` … is no part of a run … they and the
    conditions and grants behind them are drawn" (the conditions are; the
    left-out kinds are not); the bullet on a correction's `resourceAdded`
    and `abilityGiven` frames, replaced by what a first sight draws; the
    creature made a party member "still draws 'control of <actor> given'";
    a payload added to the contract needs a decision whether the Story draws
    it before the client typechecks (D4).
  - **Requirements:** the new ids in, the withdrawn ones out.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specification; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, SPEC-022 as drafted, and the surfaces as
signatures and names, never the source: `buildFeed(whole: Envelope[]):
FeedEntry[]` and `FeedEntry`; `renderSpectator(root, st, log, status,
extras)` and `ViewExtras`'s `dm`, `ticker`, `abilities` and `properties`;
`describe(e, names)`; `renderPlayerPanel`'s signature; `boot` from
`client/src/app.ts` with its fetch and socket seams as the existing
`app.test.ts` uses them. Tell it: never run `task check`, `task e2e` or
Stryker; its files are new `client/test/qa-*.test.ts` files; run them with
`env -u CLAUDECODE bun test`; the client fold refuses an `abilityGiven` of
an id the sheet names, a `resourceAdded` of a held resource and an actor
added with a controller, so a fixture is built as the gateway writes a log;
the tree is hashed before and after it. Known and not findings: a grant
tells a player nothing but the perch and the panel (OD-4); a party member
with no token shows its values nowhere in a player's window; the use line
names no cost or damage; a run's extent is found in the whole log (D2), so
a correction's conditions behind sheet frames are drawn; the door line
names no opener (G7); a run at a companion's first sight draws no line
(G8); a token going out of sight draws no line (D11).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "no other") names the search that bounds it; check that
`git diff --stat main -- client/src` is `feed.ts`, `spectator.ts` and a
comment in `grid.ts` alone, and that `renderTicker` is still handed the
whole log; check each comment under `client/src` is a warning or a
pointer; check every renamed test's new name in the register's evidence,
and that no test cites a withdrawn id; check every assertion this work
changed for being weakened rather than moved (rule 2); and read this plan's
amendment (P7 to P9, D9 to D11, R3, R10 to R14) against the code, since no
second agent verified it.
