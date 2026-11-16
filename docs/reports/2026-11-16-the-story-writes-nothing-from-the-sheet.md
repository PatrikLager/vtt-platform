# The Story draws what happens in the game, and no line that is mechanical — report

Ticket: `docs/superpowers/specs/2026-11-16-the-story-writes-nothing-from-the-sheet-design.md`.
Plan: `docs/superpowers/plans/2026-11-16-the-story-writes-nothing-from-the-sheet.md`.
Branch `feat/the-story-writes-nothing-from-the-sheet` from main `92cb3e3`.
Commits: `7833775` (Task 0: ticket, plan, register), `73761c8` (Task 1, the
Story's filter, its tests and the bundle; the last code commit) and `d22e2af`
(the comment ledger). Every code reference below is to `73761c8`.

## Done looks like

1. [x] **The Story draws no sheet or control entry.** `buildFeed`
   (`client/src/view/feed.ts`) keeps an entry only when `told` answers yes,
   and `told` reads `STORY`, which answers no for `resourceChanged`,
   `abilityGiven`, `abilityTaken`, `resourceAdded`, `resourceRemoved`,
   `actorControlGranted` and `actorControlRevoked`.
   - Every kind, with an event id and with none:
     `client/test/feed.test.ts`'s "buildFeed keeps each kind the Story draws
     and leaves out the rest, an entry with an event id and one with none",
     a walk over every case of `Envelope.payload`.
   - Every role's booted window: `client/test/app.test.ts`'s "every role's
     Story draws what happens in the game and no mechanical entry, and the
     DM's and the agent's Events panel draws each".
   - Independently: `client/test/qa-story-mechanical.test.ts`'s "<id>:
     <window>, <variant>: the Story draws no line for a <kind>", for each
     window and each of the variants with and without event ids.
   - Each fails on `92cb3e3`: run against that commit's `feed.ts`,
     `spectator.ts` and `grid.ts`, the walk and the booted window test are
     red.
2. [x] **A first sight draws none either.**
   `client/test/spectator-view.test.ts`'s "a party member corrected at its
   first sight draws no line of its sheet or its control, and the Events
   panel draws each" builds the correction in `statusFrames`' order;
   independently, "<window>: a party member corrected at its first sight
   draws no "now has", "can now use" or "is at" line". Red on `92cb3e3`.
3. [x] **The Story draws no board, scene, note or adventure entry,** nor a
   token's going out of sight (the owner's ruling after the plan's sign-off;
   VTT-823). `STORY` answers no for `tokenPlaced`, `tokenMoved`,
   `tokenHidden`, `tokenRemoved`, `tokenDescribed`, `sceneCreated`,
   `sceneSeen`, `noteUpserted`, `noteDeleted` and `adventureLoaded`. Held by
   the walk and the booted window test above, by
   `client/test/qa-move-reason.test.ts`'s "the Story draws no line for a
   move with its reason", by `client/test/qa-what-a-window-shows.test.ts`'s
   "qaWS: VTT-475 VTT-477 a player's Story shows the window's naming" and
   "qaWS: VTT-484 VTT-477 a DM's Story shows the GM chain" for `tokenHidden`,
   and end to end by
   `client/e2e/table.spec.ts`'s "the story feed and ticker record what
   happened", which now expects "Lera is in the party" and no "is at".
4. [x] **A door is an event, not a square.** `tell`
   (`client/src/view/spectator.ts`) words a door as "a door opened" or "a
   door closed"; `told` keeps a door only with an event id.
   `client/test/spectator-view.test.ts`'s "the Story draws a door as an
   event with no square, and none the log corrected, and the Events panel
   draws each with its square"; independently, "<window>: a door opened and
   closed with an event id is drawn as an event, naming no square" and
   "<window>: a door opened and closed with no event id draws no Story
   line". Red on `92cb3e3`.
5. [x] **The Events panel is unchanged.** `renderSpectator` hands
   `renderTicker` the whole log, and `describe` is untouched. The test that
   passes on `92cb3e3` and after: `client/test/qa-story-mechanical.test.ts`'s
   "<window>, <variant>: the Events panel draws one row per entry, newest
   first, each as describe words it". Beside it, the booted window test
   compares the panel with `describe`'s words for every entry, and
   `client/test/spectator-view.test.ts`'s "the Events panel shows a move's
   reason, and the Story draws no move" and
   `client/test/qa-events-panel.test.ts`'s panel tests hold it; these also
   assert the Story's absences, so they are red on `92cb3e3` (Deviations).
6. [x] **What stays in the Story.** The tests that pass on `92cb3e3` and
   after: `client/test/qa-story-mechanical.test.ts`'s "<window>: an
   ability's use, a roll and a whisper read as today" and "<window>: a
   removal that carries an event id is drawn, and the same actor's add
   behind it". Beside them, the walk keeps every kept kind; the booted window
   test draws a session's start and end, an add, a condition gained and
   lost, a use, a roll, a whisper and both doors;
   `client/test/qa-companion-first-sight.test.ts`'s "a narration between the
   removal and the add: the run is told" draws a removal and a narration;
   and "<window>, <variant>: the Story holds exactly the game's lines, in log
   order". These assert absences too, so they are red on `92cb3e3`.
7. [x] **A narration is never dropped.** `client/test/feed.test.ts`'s "a
   narration anchored only to an entry the Story leaves out is its own beat,
   at its own place" (red on `92cb3e3`, where it shared the change's beat);
   `client/test/qa-companion-first-sight.test.ts`'s "a narration anchored to
   the run alone is still drawn" and "a narration anchored over the run and
   the arrival is drawn in a beat of its own".
8. [x] **The player is still told.** `sheetNews` is untouched; the toast
   expectations of `client/test/qa-sheet-news.test.ts`,
   `client/test/sheet-news.test.ts` and `client/test/qa-older-answer.test.ts`
   did not change, and pass.
9. [x] **The gate.** `task check`, whole, on `d22e2af` with a clean tree:
   green. `task e2e` on `d22e2af`: green; `client/e2e/handover.spec.ts`'s
   "a network drop while the table plays on ends in a Reconnect" is skipped
   by its own `test.skip`, which this branch does not touch.
   `cmd/vtt/webdist` was rebuilt with `task build:client` in `73761c8`,
   after the last change to `client/src`.

## What the rules became

- The Story draws no line for an entry that changes what a sheet holds:
  VTT-809.
- The Story draws no line for a grant or a taking back of control: VTT-810.
- The Story draws no line for a token's placing, move, removal or
  description, a scene, a change of view, a note or an adventure's loading:
  VTT-818; nor for a token going out of sight: VTT-823.
- The Story draws a door with an event id without its square, and none with
  no event id: VTT-819, VTT-820.

Beneath them: the kept kinds and the run (VTT-822, succeeding VTT-662), a
narration's own beat (VTT-812), and the rows that moved a Story half to the
Events panel or to the ability button: VTT-813 (VTT-736), VTT-814 (VTT-790),
VTT-815 (VTT-622), VTT-816 (VTT-623), VTT-817 (VTT-756), VTT-821 (VTT-263).
VTT-811 was allocated for the kept kinds with `tokenHidden` among them and
withdrawn in Task 0's commit, before any test cited it.

The sort's refusals stand as the plan records them: the Events panel's row
for each entry (VTT-447, held), the toast (VTT-733, held), no message for a
grant, the typecheck forcing a ruling for a new payload (how the rules are
held), a run at a first sight (decided by VTT-810, VTT-818 and VTT-822).

## The deliberate breaks

Run one at a time against the whole client suite in a copy of the tree whose
client files were byte for byte the worktree's (`cmp`), each reverted by its
inverse edit and the copy compared again afterwards; their results are in
the task's notes and in `73761c8`'s commit message. The two breaks for
Phase 4b's fixes were run in the worktree.
- `abilityTaken`, `actorControlRevoked`, `tokenMoved`, `tokenHidden` and
  `resourceAdded` marked drawn: the walk and the booted window test spoke
  for each; for `tokenHidden` the qaWS tests named under item 3 too, for
  `resourceAdded`
  the corrected party member's test.
- `conditionApplied` left out: the walk, the booted window test and the run
  tests spoke.
- A bare door kept, a door with an event id left out, `STORY.doorOpened`
  false, the Story's door worded by `describe`: the door tests spoke.
- The left-out kinds filtered before `reintroductions`: "a resource change
  ends the run, and the bare condition behind it is told" and "a
  correction's sheet frame ends a run, and the bare condition behind it is
  told" spoke (D2). `restates` without grants: the first-sight tests spoke.
  `restates` taking a grant with an event id: "a grant behind the add that
  carries an event id ends the run" spoke.
- A narration whose anchor finds no kept entry dropped: the narration-beat
  tests spoke (VTT-812).
- `renderTicker` handed the Story's entries: the Events panel tests spoke.
- `describe` naming a declared resource by its key, an undeclared one by
  anything else, a give and a resource added reworded, a move's reason
  dropped: VTT-815's, VTT-816's, VTT-813's, VTT-814's and VTT-821's tests
  spoke.
- An older ruleset answer setting the window's properties: VTT-817's test
  spoke. The namer handed the newest answer's abilities alone: the
  older-answer tests spoke (VTT-734, VTT-759).
- `sceneSeen` removed from `STORY`: the typecheck failed (TS2741), as D4
  intends.

## Rule 9: how MapTool does this

As the ticket and the plan's P3 and P8 record: MapTool posts nothing to the
chat for a property's change or an owner's change, and no chat line of its
own for a move (an event macro's output reaches the chat through
`EventMacroUtil`); its core has no door. Borrowed: a sheet's change, a change
of control and a move are not chat lines, and what is mechanical goes to the
GM's view. Not borrowed: its distribution.

## The plan's gaps and the owner's answers

G1, G2: every test and row the plan named changed as it said, and two more
were renamed (Deviations). G3: OD-4 of 2026-11-15 is overturned; a resource a
give brings shows in a player's window as a chip alone. G4: VTT-812 is the
row. G5: `restates` keeps grants (D2). G6, G7: travel to "What could not be
established". G8: a run at a companion's first sight draws no line; a
corrected party member's bare conditions are still drawn. G9: item 9
below. OD-1 A (a table over every payload case), OD-2 A (VTT-736 and VTT-790
succeeded as Events panel rows), OD-3 (VTT-756 succeeded by the ability
button's title), OD-4 (no message for a grant).

## Phase 4a adjudications

One QA agent, handed the ticket, the rows, SPEC-022 as it stood and the
exported surface, wrote `client/test/qa-story-mechanical.test.ts`; every
test passed, and its injections into its own inputs each turned tests red.
- QA finding 1: spec gap — VTT-822's exception for a window that cannot
  fold its log is decided in `app.ts`, outside the surface handed over — no
  change: held by `client/test/qa-companion-first-sight.test.ts`'s "%s: a
  log that cannot fold shows its reason and no Story".
- QA finding 2: spec gap — VTT-815's chip title and ability cost, and
  VTT-817's reads, are outside that surface — no change: held by the tests
  the register cites for them.
- QA finding 3: spec ambiguity — VTT-812's "at its own place": read as the
  narration's own place in the log, as the row says — no change.
- QA finding 4: spec gap — the Story's order is stated by no document QA
  was handed — no change: not this ticket's subject.

## Phase 4b

Task 0's ticket and plan had two reading reviews before their commit; the
second followed the owner's ruling on `tokenHidden`. Task 1 had one review
and a narrow second one of its fixes:
- A test that could no longer fail on its name ("a grant behind the add that
  carries an event id ends the run"): a bare condition now stands behind the
  grant.
- `cmd/vtt/webdist` rebuilt.
- `STORY`'s door entries were never read: `told` now reads them.
- `buildFeed`'s comments that this change made false, and the doc comments
  on `told`, `tell` and `reintroductions`, rewritten as pointers. The reason
  removed from beside the code: overlapping anchors resolve last-writer-wins
  because a union-find over ranges is not worth it for two DMs anchoring
  overlapping narration; the later narration owns the grouping
  (`client/test/feed.test.ts`'s "overlapping anchors resolve
  last-writer-wins").
- Two renames (Deviations); "the ruleset's properties reach the chip and
  the cost, and the Story draws no resource change" now also asserts the
  Story holds no "→", and "a narration anchored over the run and the arrival
  is drawn in a beat of its own" asserts its beat is found before reading
  it.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Items 5 and 6: client tests that pass on today's tree and after | The independent tests named under each pass on `92cb3e3`; the walk, the booted window test and the existing tests named beside them are red there | Those tests assert what the Story leaves out as well as what it keeps and the Events panel draws, so the base tree, which draws every entry, fails their absences. |
| Phase 4a handed SPEC-022 as drafted | QA was handed the ticket, the rows, SPEC-022 as it stood and the exported surface | SPEC-022 is updated after the report, from it (D8), so no draft existed; QA was told that where SPEC-022 says the Story draws a kind the ticket leaves out, the ticket and the rows win. |
| The breaks in the worktree | In a copy whose client files were the worktree's byte for byte, before the copy's files were moved in | The worktree held the uncommitted ticket and plan under review at the time; the reviewer read the copy's code. |
| Task 1's "True when": `check-comments.py` passes | `73761c8` was committed with one finding (`feed.ts`'s share under its ceiling); the ledger came down in `d22e2af` | A lowered ceiling cannot be raised again, so the ledger is written once, after the last code commit, as Task 2 says. |
| The renames the plan lists | Also `spectator-view.test.ts`'s "… and its Story still draws the game's lines" and `qa-window-number.test.ts`'s "describe, the label the Events panel shows for an event, …" | Their old names said the Story shows the log or `describe`'s label, which is no longer true; the register's evidence follows. |

## What could not be established

- **Who opened a door.** The line names no one; a ticket of its own.
- **How a player learns of a grant.** By the window offering the character;
  no message, by the owner's ruling.
- **Where a value is still seen.** A party member's resource values only on
  its disc's chips; one with no token shows them nowhere. The player panel
  as the character sheet is the next ticket.
- **What an ability's use leaves unsaid.** Its cost and damage reach the
  window as data alone.
- **The client's mutation.** `task check` says it was not measured: the
  stored report measured `184d917`, and this ticket and the one before it
  were added since (SPEC-023). A full run is the owner's to ask for.
- **A near-duplicate test.** "a bare condition behind a grant with an event
  id is told" now differs from "a grant behind the add that carries an event
  id ends the run" only by the token's arrival; kept, since deleting a test
  drops coverage nobody named.

## What was deliberately left out

- The player panel as the character sheet: the next ticket, by the owner's
  ruling of 2026-11-16.
- Naming a door's opener: a ticket of its own (contract, gateway,
  projection, MCP and client).
- SPEC-022: updated after this report, from it.
- A full client Stryker run: the owner's to ask for.
