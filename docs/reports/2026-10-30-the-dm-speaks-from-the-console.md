# The DM speaks from the console, with /ooc named where they type: the change

**Ticket:** `docs/superpowers/specs/2026-10-30-the-dm-speaks-from-the-console-design.md`.
An agent that did not write it verified it and passed it with gaps G1 to
G7; the ticket's writer corrected G1's MapTool path and G6's wording before
Task 0's commit.
**Plan:** `docs/superpowers/plans/2026-10-30-the-dm-speaks-from-the-console.md`,
written by that verifier from probes outside the tree, and amended by the
owner's answers to its four questions and by Task 0's review.
**Where it came from:** the owner's request of 2026-10-30, "ta DM:ens
platshållare för /ooc som nästa ärende, borde vi inte ta DM konsolen
samtidigt?". `docs/reports/2026-10-29-a-refused-command-keeps-what-was-typed.md`
had left out, under "What was deliberately left out", that the DM's
console has no Say form and a DM who controls no actor has none in the panel
either.
**The owner's rulings, all 2026-10-30:**
- Where the DM speaks: "I konsolen, alltid".
- The layout: "Ett ärende, en gren".
- On the plan's questions: Q1 "Först, över Session", Q2 and Q3 "Ja, båda",
  Q4 "Inget". The plan signed off: "Ja, kör".
- At Task 3, shown E2's screenshot with the console's Say row cut short at
  1280 px: "Ja, låt textrutan fylla raden".

**Last code commit:** `b70af46`, on `9e33202`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `9b19264` | Task 0: the ticket, the plan, VTT-527 to VTT-537, VTT-504 and VTT-519 withdrawn |
| `c7989b5` | Task 1: the console's Say in `client/src/view/dm.ts`, `client/test/support/say.ts`, the tests, QA's `client/test/qa-console-say.test.ts`, VTT-538 and VTT-539 |
| `5da493a` | Task 2: `renderPlayerPanel` draws Say for a player only, the DM's Say tests moved to the console, `sayConsole`, QA's `client/test/qa-panel-say.test.ts`, VTT-540 |
| `b70af46` | Task 3: `client/e2e/refusals.spec.ts`, the Say row's widths in `client/src/style.css`, `say-text` without `wide`, the bundle |
| `fe713da` | VTT-539 withdrawn, succeeded by VTT-541; the three tests that held it cite VTT-541, and QA's header names its ids without spanning VTT-539 |
| `08917d7` | E1's comment in `client/e2e/refusals.spec.ts` says the next test's grant outlives the session it starts, not the one it ends; comment only |

## What the code did on `9e33202`

- `renderDMConsole` in `client/src/view/dm.ts` drew no Say: the file named
  neither `addNarration` nor `speakOutOfCharacter`, nor a `say-as` or
  `say-text` box.
- The DM's Say was the player panel's. `renderPlayerPanel` in
  `client/src/view/player.ts`, which `client/src/app.ts` draws for any role
  whose window does not perch, drew `say-as` and `say-text` for a DM or an
  agent (`speaksFreely`), the line box's placeholder reading "say or
  narrate", and `say-text` alone for a player, reading "say something, or
  /ooc to step out of character". For a role that controlled no actor it drew
  "You do not control an actor yet." and returned before the Say. So a DM
  narrated from the browser only through a character granted to them.
- `client/test/command-surface.test.ts` tagged `addNarration`'s control
  `dm-console`, under a comment saying it was drawn in the panel.

## The plan's labels, and the tests that carry them

| Label | Test | File |
|---|---|---|
| S1 | "the console draws its Say first, whether or not anybody controls an actor" | `client/test/dm-view.test.ts` |
| S2 | "the console's line box names /ooc, and its speaker box says something else" | the same |
| S3 | "the console's Send speaks in character as say-as holds it, trimmed, or as nobody when it is blank" | the same |
| S4 | "a console line whose first word is /ooc goes out of character, with the rest as its text" | the same |
| S5 | "a blank console line, or /ooc with nothing after it, sends nothing and holds nothing back" | the same |
| S6 | "Enter in the console's say-text sends the line, and another key does not" | the same |
| S7 | "a refused console line leaves say-as and say-text as typed, in character or out" | the same |
| S8 | "an accepted console line empties say-text, keeps say-as, and the next line speaks as it" | the same |
| S9 | "while a console line is unanswered, Send and Enter send nothing, and another control still sends" | the same |
| S10 | "a press of the console's Send or Enter that sends a line changes nothing drawn until its answer" | the same |
| S11 | "a console line whose send rejects is reported, clears nothing, and frees Send and Enter" | the same |
| S12 | "what was typed in the console's say-text survives a render, before a line and after one" | the same |
| A1 | "a DM who controls no actor speaks from the console, and an accepted line empties its box" | `client/test/app.test.ts` |
| A2 | "a DM's and an agent's page holds one line box, the console's, and a player's the panel's" | the same |
| U1 | "a DM's or an agent's panel draws no Say, and a player's draws its line box and Send" | `client/test/spectator-view.test.ts` |
| U2 | "a player's line box names /ooc in its placeholder" | the same |
| E1 | "a DM's accepted line empties say-text and keeps say-as for the next line" | `client/e2e/refusals.spec.ts` |
| E2 | "a DM granted a character draws its chip, and the console's Say alone" | the same |

E1 keeps the name of the test it rewrites.

## Done looks like, answered

1. [x] S1 draws the console over an empty world and over a world whose two
   actors are held, one by `p-dm` and one by `p-ana`. Each time its first
   group is headed "Say something" and holds `say-as` and `say-text`, both
   inputs, and a Send (`add-narration`). S2: `say-text`'s placeholder
   contains `/ooc`, and `say-as`'s is not empty and differs from it.
   `client/test/qa-console-say.test.ts`'s "a DM who controls no actor is
   drawn say-as, say-text and Send", "a DM who controls an actor is drawn
   say-as, say-text and Send", "the Say is first in the console, headed Say
   something" and "say-text's placeholder names /ooc" hold the same. On
   `9b19264`, before the code, S1 and S2 were red for want of the boxes.
2. [x] S3 types `  Grimble ` into `say-as` and `Hello` into `say-text` and
   presses Send: one `addNarration`, text `Hello`, `as` `Grimble`, not out of
   character, no actor id. A `say-as` of spaces sends `Hold.` with no `as`.
   S4 sends `/ooc brb` out of character with the text `brb`, and `/oocx hi`
   and `say /ooc now` in character as typed. S5, with the answer held, types
   an empty line, spaces, `/ooc` and `/ooc   `, each pressed by Send and by
   Enter: nothing is sent, and `Hello` typed after them sends. S6: Enter in
   `say-text` sends the line; the keys `a` and `Shift` do not. QA's "Send
   says Hello in character as Grimble", "/ooc brb with a speaker goes out of
   character as brb", "a blank line sends nothing and takes no lock" and
   "/ooc with nothing after it sends nothing" hold the same.
3. [x] For the console's Say:
   - refused: S7, for `Hello` and for `/ooc brb`, with `Grimble` in
     `say-as`, answered `ok: false`: one line sent, no repaint asked, and the
     next render's `say-as` reads `Grimble` and `say-text` the line;
   - accepted: S8 holds the answer and types `Hello, and` into `say-text`
     after Send. No repaint is asked before the release and one after it;
     the next render's `say-as` reads `Grimble` and `say-text` `""`; `Bye`
     then carries `as` `Grimble`;
   - the lock: S9 presses Send then Send, Send then Enter, and Enter then
     Enter, the answer held: one line each time, and a console drawn afresh
     sends none either. Start session still sends meanwhile. After a refusal
     frees the lock, a press sends `Bye`;
   - the press: S10, by Send and by Enter, the answer held: one line sent,
     no repaint asked, both live boxes as typed, and the next render's
     `outerHTML` and every box's `.value` as before the press;
   - a rejected send: in S11 Send's send rejects, then Enter sends again and
     rejects too. `console.error` gets each error, no repaint is asked, and
     the next render's boxes read as typed.

   S12: what was typed in `say-text` survives a render, and after an
   accepted line what was typed since survives one.
   `client/test/qa-console-say.test.ts` holds each of these again under its
   own names, in its groups "what an answer leaves typed", "one press, one
   line", "a press that sends draws nothing new before its answer" and "a
   send whose promise rejects".
4. [x] U1 draws the panel for a DM and for an agent, each controlling `a1`:
   the chip `Lera` is drawn, and no `say-as`, no `say-text`, no
   `add-narration` and no heading "Say something". A player's panel draws
   `say-text` and Send. U2: a player's `say-text` placeholder contains
   `/ooc`. The existing "a player's Send speaks as the party member the panel
   acts as, and the panel draws no speaker box" holds a player's missing
   speaker box. QA's "a DM's panel, controlling an actor, draws no Say" and
   "an agent's panel, controlling an actor, draws no Say" also read the
   panel's text and attribute values for anything about saying, and its
   other tests do the same with a switcher and after a repaint. On `c7989b5`
   U1 was red at the DM's panel; U2 was green, as G5 said.
5. [x] A1 boots the app on a DM's page with no session open: the panel
   reads "You do not control an actor yet." and the page holds one
   `say-text`, inside `.dm`. `Hello` typed there and Enter: the socket has
   one frame, an `addNarration` with the text `Hello`. Answered `ok`, the
   page holds one `say-text`, inside `.dm`, reading `""`. A2 draws the page
   for a DM, an agent and a player, each panel showing the chip `Lera`: the
   one `say-text` is the console's for the DM and the agent and the panel's
   for the player, and only the player's `.player` holds an `add-narration`.
6. [x] E1: the DM, granted nothing, sees the panel's "You do not control an
   actor yet." and one `say-text` on the page, none in `.player`. In the
   console the DM types `Grimble` and `Hello` and presses Enter: the Story
   shows `Grimble: Hello`, `say-text` reads `""` and `say-as` `Grimble`.
   `Bye`, sent by the console's Send, shows as `Grimble: Bye`. E2: through
   `asDM` the DM starts a session, loads `goblin-ambush` and is granted
   `act-cutter` as `ACTOR_KIND_NON_PARTY`. The panel shows the chip "Goblin
   Cutter" and holds no `say-text`, `say-as` or `add-narration`, and none of
   its text or attribute values matches the pattern QA's VTT-540 tests use;
   the page's one `say-text` is the console's. `task e2e` ran whole four
   times across Task 3's changes, 32 passed and 1 skipped each time, the
   skipped one `client/e2e/handover.spec.ts`'s `test.skip` "a network drop
   while the table plays on ends in a Reconnect".
   `client/e2e/speaking.spec.ts`'s "another player's Story names a line by
   its speaker, in character and out of it" passed each time. E2's last
   screenshot, `client/e2e/.artifacts/r2-a-dms-panel-draws-no-say.png`,
   which git ignores, shows the panel with the Goblin Cutter chip, its
   abilities and "Click the board to move.", and no Say; the console's first
   group, "Say something", shows both placeholders whole; the Story holds
   `Grimble: Hello` and `Grimble: Bye` from E1. The fourth run ended at
   13:55:58. One edit followed before `b70af46` (13:57:01): it swapped the
   two comment lines above E1, putting the id line last for
   `check-new-prose.py`'s SHORT rule. No line was added, and no code or
   assertion changed.

## What the rules became

- "The DM's and the agent's console draws a Say form whatever they control,
  and its line box's placeholder names `/ooc`": VTT-527, and VTT-528, which
  also holds a player's line box.
- "The console's Send speaks with what its speaker box holds as `as`, or
  none when it is blank, and a line whose first word is `/ooc` goes out of
  character": VTT-529 and VTT-530, and VTT-531 for a blank line or a bare
  `/ooc`. VTT-501, which the rule cites, stays the panel's, a player's now.
- "The player panel draws a Say form for a player only. This withdraws
  VTT-504": VTT-532. VTT-504 is withdrawn, dated 2026-10-30, and succeeded
  by VTT-527, VTT-529 and VTT-532.
- "The console's Say keeps what VTT-518, VTT-520, VTT-521, VTT-522, VTT-523,
  VTT-524 and VTT-526 hold for the panel's; those rows … move with it": the
  plan's D11 keeps a row that stays true under its id and words, and
  withdraws and succeeds one the change makes false, so no committed row was
  reworded to follow the Say. Its G2 says what each row became, and Task 0's
  review asked this report to reconcile them with the rule.
  - VTT-517, "a command the server refuses clears no DM console field that
    fed it", holds of the console's Say as worded, and gained its tests.
  - VTT-518 and VTT-520 name the player panel's `say-text` and stay true, of
    a player's panel.
  - VTT-521 and VTT-522's first half name "a DM's or an agent's `say-as`",
    by role and field. By Q2's answer the console's box keeps the name, so
    both stay true as worded; their evidence moved with the tests.
  - VTT-523, VTT-524 and VTT-526 list their controls and stay true. The
    console's Say got its own: VTT-535, VTT-536 and VTT-537.
  - VTT-522's second half names the panel's `say-text`; the console's got
    VTT-534.
  - VTT-519 said an accepted command leaves each console field that fed it
    as with nothing typed, the notes form's key, title and text excepted.
    The console's `say-as` keeps its text after an accepted line, so VTT-519
    is withdrawn, dated 2026-10-30, and succeeded by VTT-533, which adds that
    exception.

Three rows came after sign-off, each at a QA agent's request, and a fourth
succeeds one of them; the plan lists none of them:
- **VTT-538**, from Task 1's QA: when the server refuses a command one of
  the nine console controls that clear a field, or the console's Send or
  Enter in `say-text`, sent, the console asks for no repaint of its own.
- **VTT-539**, from Task 1's QA: while a command one console control sent is
  unanswered, every console control that does not share its lock still
  sends; Send and Enter in the console's `say-text` share one.

  Both were first worded of every console control, and Task 1's review
  found both false as worded: the door, the join link and the role controls
  hand every answer, a refusal included, to `refreshSharing`, which re-reads
  the link and the roster and repaints; and Send and Enter share one lock.
  Both were reworded in place before `c7989b5`. It is the class of
  `2026-10-29-a-refused-command-keeps-what-was-typed`'s VTT-525: a row
  widened from a SPEC-022 sentence that is scoped to `sending`.

  VTT-539 was withdrawn on 2026-10-30, by `fe713da`, after SPEC-022's
  reading review found it false as worded still: Arm doors and the guide
  send no command, and a press a control's own check refuses sends nothing.
  **VTT-541** succeeds it: a command a DM console control sent holds back,
  while it is unanswered, no control but that one, Send and Enter in the
  console's `say-text` counting as one control. It carries VTT-539's
  evidence, and the three tests that cited VTT-539 cite it.
- **VTT-540**, from Task 2's QA, the owner's "Inget" (Q4): a DM's or an
  agent's player panel draws no line box, no speaker box, no Send for a line
  (`add-narration`), and no line pointing to the console's Say. Its scope was
  checked against QA's tests for a DM and an agent, with and without a
  switcher, and after a repaint.

The sort refused, under the plan's "Requirements": the page holding one line
box, which follows from VTT-527 and VTT-532; the browser test, an
observation; `speaking.spec.ts`, a guard; where the Say sits, its heading,
names and classes; the agent's window, which VTT-527 covers; a DM who
controls an actor keeping the panel's chips and abilities, existing
behaviour; the server, unchanged; the comment in
`client/test/command-surface.test.ts`; an out-of-character line from the
console carrying `say-as`, a preference (D9) the server overwrites
(VTT-497); the console's `say-as` after a repaint, VTT-522 as worded; and a
refused console line keeping both boxes, VTT-517 as worded.

## Phase 4a

One QA agent for each task that delivered behaviour, Tasks 1 and 2. Each was
given the requirements, the ticket, SPEC-022 and the surface as signatures
and names, never the source. Task 0's records and Task 3's browser tests,
stylesheet and bundle, and the `wide` class dropped from `say-text`, had no
behaviour of their own to derive.

| Task | Tests, as committed | Failures |
|---|---|---|
| 1 | 31 in `client/test/qa-console-say.test.ts` | 0 |
| 2 | 33 in `client/test/qa-panel-say.test.ts` | 0 |

Neither found a behaviour defect. Task 1's QA's injections each went red
where aimed, but for two that stayed green and were right to: VTT-521 holds
for a refused line too, and a refusal draws nothing new. Task 2's QA made
eighteen injections, each red where aimed.

### QA adjudications

"SPEC-022 says it" means SPEC-022 as of the commit that follows this report.

Task 1's QA numbered its spec gaps G1 to G6. It left its three requirements
needing an ID unnumbered. The first is taken with G3, and the other two are
7 and 8 here.
- QA finding G1: spec gap — the agent's half of VTT-527 cannot be reached
  through `DMDeps`, which carries no role; as built, `isDM` in
  `client/src/app.ts` draws the console for the DM and the agent, and A1 and
  A2 observe the composed app — no change.
- QA finding G2: spec gap — item 5 needs `client/src/app.ts`, which QA was
  not handed; A1 holds it — no change.
- QA finding G3: spec gap, with its first requirement needing an ID —
  whether a line out of character from the console carries `say-as`; it
  does, and the plan's sort refused a row for it as a preference (D9), since
  the server writes that line's speaker (VTT-497) — QA's "/ooc from the
  console carries what say-as holds" cites D9 and no id.
- QA finding G4: spec gap — Enter in `say-as`, and Shift with Enter, are
  unspecified; under "What could not be established" — no change.
- QA finding G5: spec gap — whether the line's text is trimmed; `readLine`
  trims it, as SPEC-022 says under VTT-501 — no change.
- QA finding G6: spec gap — the console's live `say-text` keeps the line
  after `ok` until the repaint, where the panel empties its live box; as
  built for every console form, since `sending` clears the draft and asks
  for `DMDeps.rerender`, which `app.ts` sets to `paint` — SPEC-022 says it.
- QA finding 7: requirement needing an ID — "a refused line asks for no
  repaint", which SPEC-022 says of the controls that send through
  `sending` — VTT-538 allocated, and reworded before `c7989b5` (above).
- QA finding 8: requirement needing an ID — another console control sends
  while the Say is unanswered — VTT-539 allocated, reworded before
  `c7989b5`, and withdrawn by `fe713da`, succeeded by VTT-541 (above).

Task 2's QA numbered its spec gaps 1 to 6. Its four requirements needing an
ID are 7 to 10 here, in its order.
- QA finding 1: spec gap — SPEC-022 still says a DM's and an agent's panel
  draws `say-as` (VTT-504); withdrawn — SPEC-022 says the console draws it.
- QA finding 2: spec gap — VTT-522's "a DM's or an agent's `say-as`" names
  no surface; true as worded, since the console's box keeps the name (Q2) —
  no change.
- QA finding 3: spec gap — which actor a player's line speaks as when
  `selectedActorId` is `""`; existing behaviour, their first, which
  spectator-view's "with no party member chosen, a player's Send speaks as
  their first" holds under VTT-500 — no change.
- QA finding 4: spec gap — a player with no actor and no switcher; the app
  hands every player's panel a switcher, and a player whose set says they
  control no party member perches
  (`2026-10-23-a-player-without-a-character-watches-as-a-spectator`;
  `mayPerch` in `client/src/app.ts`), out of scope — no change.
- QA finding 5: spec gap — when a repaint is asked for is said in SPEC-022
  and in neither VTT-518 nor VTT-520, the panel's rows; as built — SPEC-022
  says it.
- QA finding 6: spec gap — VTT-524 read strictly, that a repaint while a
  line is unanswered draws the same markup; as built — no change.
- QA finding 7: requirement needing an ID — the owner's "Inget": no trace
  of Say in a DM's or an agent's panel, not even a pointer — VTT-540
  allocated.
- QA finding 8: requirement needing an ID — a DM who controls an actor
  keeps the panel for choosing it; refused by the plan's sort as existing
  behaviour, which SPEC-022 records — QA's two chips tests cite VTT-438.
- QA finding 9: requirement needing an ID — text typed while a line is
  unanswered is emptied at `ok`; VTT-520's wording covers it — no id.
- QA finding 10: requirement needing an ID — a line in which `readLine`
  finds nothing takes no lock; SPEC-022's paragraph under VTT-523 says it —
  no id.

After Phase 4a the QA files changed by review findings:
- Task 1's: its header names the ids it holds, and one test was renamed,
  now "say-text shows what was typed across repaints and a refusal".
- Task 2's: the two chips tests cite VTT-438 instead of the ticket's words.

## Phase 4b

One reviewer per task, by command, with probes outside the tree. One pass
each.

| Task | Findings |
|---|---|
| 0 | 1 Must, 3 Should, 8 Nits |
| 1 | 2 Must, 7 Nits |
| 2 | 1 Should, 6 Nits |
| 3 | 2 Should, 3 Nits |

Every Must and every Should was taken. Of the Nits, Task 1's sixth was for
the specification, which lists VTT-538 and VTT-541, VTT-539's successor, and
its seventh for this report (below). Of Task 2's, two changed nothing: the
2026-10-25 debt entry's closing paragraph names `sayAs`, a dated record of
what closed it; and the agent's half of each test draws the same role-blind
console. A
third changed half: QA's citations of VTT-501 and VTT-518 stand, since
SPEC-022 puts a blank line under VTT-501 and the absent repaint under
VTT-517 and VTT-518, and QA's chips tests cite VTT-438.

The findings that changed the work:
- **Task 0.** The Must: two tickets were named by a number, which resolves
  to nothing in the repository; they are named by date and slug. The Shoulds:
  two "red under" claims credited a row with a break that only reddened a
  test; text the amendments left behind contradicted the records; D8 left
  `.say .as` in the stylesheet with no element to carry it, which Task 2
  deleted. Among the Nits: the lock and rejected-send tests press both Send
  and Enter; the moved tests' new citations were named; and the thin margin
  of `view/dm.ts`'s comment share above its band was said.
- **Task 1.** The Musts: VTT-538 and VTT-539 as first worded (above). The
  Nits: the group's citation line gained VTT-528 and VTT-533; VTT-528's
  panel half waited for Task 2's U2; VTT-529 gained QA's "Enter in say-text
  says Hello in character as Grimble"; QA's header and a test name; and the
  two Say forms between Tasks 1 and 2 (below). The reviewer redid all twelve
  breaks, and made two more in a scratch copy: `say-as` sent untrimmed
  turned S3, QA's trim tests and spectator-view's "an in-character speaker
  is carried, and a blank one is omitted" red, and the Say group drawn last
  turned S1
  and QA's "the Say is first in the console, headed Say something" red.
- **Task 2.** The Should: the renamed "a player's /ooc sends no speaker"
  holds VTT-501, not VTT-500, since breaks of both halves of VTT-500 left
  it green; it cites VTT-501 and its register entry moved there. Nits
  taken: a stale "Q1" in `client/test/qa-speaking.test.ts`'s header; the
  role check's citation names VTT-532 and VTT-540; "qaSay: a blank Send sends
  nothing and leaves both boxes as typed" cites VTT-531; the chips tests
  cite VTT-438. The reviewer found the role check after the chips,
  abilities, targets, move hint and doors, and `app.ts` the panel's only
  caller, with four roles; the role check removed turned 14 tests red and
  the check that admits the agent 4; the role check moved above the chips,
  12; `forgetTheSay` made a no-op, 6; six randomised runs of the seven Say
  files passed.
- **Task 3.** The Shoulds: with `flex` sizing the console's `say-text`, its
  `wide` class sized nothing, though a test and D8 said it did, so the box
  drops it and the test says its class is `""`; and D8's widths no longer
  hold, which this report records. The Nits: `min-width` kept the box from
  shrinking in a narrow console, so it is `flex: 1 1 240px`; E1 runs before
  E2, whose grant outlives the session it starts, which the file's
  `afterAll` ends, and a comment says so (since `08917d7`; on `b70af46` it
  said "the session it ends");
  VTT-521 says Send while E1 pressed Enter twice, so `Bye` goes by Send. In
  a scratch copy the console clearing `say-as` too turned E1 red at its
  `say-as` read, and the panel's role check removed turned E2 red at the
  panel's line box count; a rebuild there matched the bundle byte for byte.

### Between Task 1's commit and Task 2's

From `c7989b5` to `5da493a` a DM or an agent who controls an actor was drawn
two Say forms, the console's and the panel's, sharing their `data-field`
names (the plan's D12). Task 1's review measured that a repaint then hands
the focus to the panel's `say-text`, first in tree order, so what is typed
splits between the two forms. Nothing was pushed between the two commits,
and `cmd/vtt/webdist` was not rebuilt: its build before `b70af46` is
`128e4f3`. No test committed at `c7989b5` failed on that window. Written in
Task 2, A2 and three of the moved tests of
`client/test/qa-say-boxes.test.ts`, which count a window's boxes or hide the
console, were red on `c7989b5`'s code and are green on `5da493a`.

## The deliberate breaks

Each was made by an exact edit, confirmed by grep, run, undone by the
inverse edit, and checked by grep and by the file's hash. Task 1's ran over
the seventeen test files of the plan's P4 and P5, and Task 2's over the
whole client suite.

| Break | What it did | Red |
|---|---|---|
| K1 | the Say group drawn only when the world holds an actor | S1, A1, and S2 to S12 and the three field-list tests, which draw an empty world |
| K2 | `/ooc` taken out of the console's placeholder | S2 |
| K3 | `as` never passed | S3, S8 |
| K4 | `readLine`'s `ooc` ignored | S4 |
| K5 | the empty-text return removed | S5 |
| K6 | `clears: ["say-text", "say-as"]` | S8 |
| K7 | `clears: []` | S8, S12, A1 |
| K8 | Enter given its own lock key | S9, at Send then Enter: two lines |
| K11 | Enter's key check removed | S6 |
| K12 | the press deletes `draft["say-text"]` | S7, S10, S11 |
| K13 | `say-text` drawn without reading `draft` | S12, S7, S11 |
| K14 | the Say sends through `d.send` outside `sending` | S9, S11, and S12 through S11's second unhandled rejection surfacing during it |
| K9 | the panel's role check removed | U1, A2, and seven tests of `client/test/qa-say-boxes.test.ts`, whose box lookup found the panel's form first; run before QA's file existed; at `b70af46` QA's five VTT-540 tests go red too |
| K10 | the role check admits the agent | U1 and A2, at the agent; at `b70af46` QA's two agent tests too |
| — | a player's placeholder without `/ooc` | U2 alone |
| — | K6 redone in Task 2, once the DM's tests had moved | 16 tests, the moved ones and Task 1's QA's among them |
| — | K3 redone the same way | 23 tests, the moved ones among them |

Every test a break was predicted to redden went red. K5's first undo
failed: the inverse of a pure deletion had no anchor. The line was put back
at once by an insertion anchored on the next line, and checked by grep and
hash; K5's result is from a rerun, which agreed with the first.

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c`:
- **One box, always there, for every client.** `MapToolFrame` builds one
  `CommandPanel` (`src/main/java/net/rptools/maptool/client/ui/commandpanel/`),
  whoever the player is. The GM speaks from the same box as everyone, with
  nothing to be granted first.
- **The speaker stays.** `globalIdentity`, a `TokenIdentity`, is kept until
  `setGlobalIdentity` sets another or `clearGlobalIdentity` clears it.
- **`/ooc` is typed into that box.** `OOCMacro` is a slash command of the
  same panel.
- **The box empties at send.** `commitCommand()` adds the line to
  `commandHistory` unless it repeats the last, and sets `commandTextArea` to
  `""`.

Borrowed: one Say the DM always has (VTT-527), a speaker that stays
(VTT-521), and `/ooc` in the same box, named where it is typed (VTT-528).
Not borrowed: the box that empties at send, by the rulings of 2026-10-29; a
player's free speaker, by those of 2026-10-26 (VTT-500); the command
history.

## The plan's gaps and questions

- **G1.** The ticket's MapTool path resolved from `~/dev/RPTool`, not from
  inside `maptool`. Corrected in the ticket before Task 0's commit.
- **G2.** The fourth rule missed VTT-519 and needed "move" to mean
  something the register allows; answered under "What the rules became".
- **G3.** Files the ticket did not name:
  `client/test/qa-refused-command.test.ts` and `client/src/view/dm.ts` cited
  VTT-519, and now cite VTT-533; the register's evidence cells moved with the
  tests; `client/test/support/say.ts` is new. `client/src/app.ts` did not
  change (D4), and `tools/comment-ceilings.txt` has not (below).
- **G4.** Deleting the DM's Say would raise `view/player.ts`'s comment share
  above its ceiling; D6 kept it under (below).
- **G5.** Item 4's player half, item 2's blank half and item 6's
  `speaking.spec.ts` held on `9e33202`, so they guard the change rather than
  fail before it. S5 pairs its blank lines with one that sends.
- **G6.** The ticket said the mutation report was out of date since
  `9e33202`; it measured `db81c37` and has been out of date since
  `2026-10-29-a-refused-command-keeps-what-was-typed`'s first `client/src`
  commit. Reworded in the ticket.
- **G7.** The agent's window gains the Say, since `isDM` draws the console
  for both roles; A2 observes it. Whether a person opens an agent's window,
  and whether a DM's panel with no Say reads as complete, are below, under
  what could not be established.

- **Q1**, where in the console the Say sits: "Först, över Session". It is
  the console's first group, under "DM console".
- **Q2**, whether the console's boxes keep the names `say-as`, `say-text`
  and `add-narration`: "Ja, båda". They do, so VTT-521 and VTT-522 stay true
  as worded.
- **Q3**, where the console's Say keeps its text across a repaint: "Ja,
  båda". In `draft`, and it sends through `sending` with `clears:
  ["say-text"]`.
- **Q4**, whether a DM who controls an actor keeps any trace of Say in the
  panel: "Inget". None, not even a pointer to the console: VTT-540.

## The comment shares

`python3 tools/check-comments.py`'s `measure`, on `b70af46`:
- `client/src/view/player.ts`: 52 comment lines of 247, 21.05 against its
  ceiling of 21.3. D6 made two prose comments citation-only lines,
  `// VTT-522` and `// VTT-524`, and deleted the VTT-500 comment and
  `// --- speaking ---`; the role check carries `// VTT-532 VTT-540`. The
  plan's scratch draft measured 21.14, and the two comments rewritten as
  prose 22.40, which the gate refuses.
- `client/src/view/dm.ts`: 265 of 723, 36.65 against 37.5, above the band's
  lower edge of 36.5. The Say group carries one citation line.

Both are within 1.0 of their ceilings, so D15's `--write-ledger` has nothing
to lower in these two files.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| D8: `say-text` takes the console's `wide` class, and `say-as` the console's default width | `client/src/style.css` gives `.dm input[data-field="say-as"]` 200 px and `.dm input[data-field="say-text"]` `flex: 1 1 240px`; `say-text` carries no class, and dm-view's "the console's styling hooks are the ones the stylesheet targets" says its class is `""` | E2's first screenshot at 1280 px cut both placeholders short. The owner, shown it, ruled "Ja, låt textrutan fylla raden". `say-as` at 180 px still cut "(optional"; with `flex` sizing the box, `wide` sized nothing (Task 3's review); `min-width` kept it from shrinking in a narrow console |
| The plan's requirements: VTT-527 to VTT-537 | VTT-538, VTT-539 and VTT-540 as well, VTT-539 since succeeded by VTT-541 | Task 1's QA found two sentences of SPEC-022 its tests leant on held by no row, and Task 2's QA the owner's "Inget". VTT-538 and VTT-539 were reworded before their commit, and VTT-539 withdrawn by `fe713da` (above) |
| The plan's "For Phase 4a": one QA file, `client/test/qa-console-say.test.ts` | Two QA agents, one per task; Task 2's wrote `client/test/qa-panel-say.test.ts` | Phase 4a runs once for each task that delivers behaviour, and the panel's change existed only at Task 2 |
| D10: `client/test/support/say.ts` exports `forgetTheSay()` | It also exports `sayConsole(st, send, rerender)`, which `forgetTheSay` uses and four test files draw the console with | `client/test/spectator-view.test.ts`, `qa-say-boxes.test.ts`, `qa-speaking.test.ts` and `qa-refused-line.test.ts` each needed a console with only its send and its repaint wired; one helper rather than four copies of a `DMDeps` stub |
| VTT-500 keeps "a player's /ooc sends no speaker, whatever the panel's state holds", renamed | The renamed "a player's /ooc sends no speaker" cites VTT-501, which holds it in the register | With `sayAs` gone it observes an out-of-character line, and breaks of both halves of VTT-500 left it green (Task 2's review) |
| spectator-view's "what a DM typed in Say survives a repaint, with the as box focused" loses its comment | It lost the assertion under the comment too | `sayTable` finds the boxes by `data-field`, so asserting their `data-field` compared the lookup with itself |
| The moved tests keep their bodies | Some end with `await settle()` | The console's lock lives in module state, shared by every test in one bun process, so each such test lets its answer settle before it ends |
| E1: `Hello` by Enter, then `Bye`; Task 3's first E1 sent both by Enter | `Bye` goes by the console's Send; E1 first asserts the panel's "You do not control an actor yet.", one `say-text` on the page and none in `.player` | VTT-521 says Send, which E1 had not pressed (Task 3's review). The first assertions observe that the DM speaks with no character granted |
| E2 cites VTT-532 | It cites VTT-532 and VTT-540, and reads the panel's text and attributes against QA's pattern | VTT-540 was allocated after the plan |
| S1, S9 and A1 as the plan words them | S1 also reads the first group's heading; S9 also presses a console drawn afresh while the line is held; A1 first reads the panel's "You do not control an actor yet." | They observe D2's place, the lock across a render, and item 5's "controls no actor" |
| Task 1's register list | VTT-521 also gained S8, and VTT-529 QA's "Enter in say-text says Hello in character as Grimble" | S8's id line cites VTT-521; Task 1's review asked for the second |

## What could not be established

- Whether a person opens an agent's window (the ticket, G7). A2 shows that
  an agent's page draws the console's Say and the panel none.
- Whether a DM's panel with no Say reads as complete. E2's screenshot was
  shown to the owner; his recorded answer is about the console's Say row.
- No browser test holds an answer, so the console's lock (VTT-535) and its
  press that draws nothing (VTT-536) are held by unit tests alone.
- Enter in `say-as`, and Shift with Enter in either `say-text`: in each view
  only `say-text` has a keydown handler, and it tests `key === "Enter"`
  alone, so by reading Enter in `say-as` sends nothing and Shift with Enter
  sends as Enter does. No test presses either, and no ruling was asked (QA
  finding G4).
- `playwright.config.ts` names no browser project, so the e2e ran in
  Playwright's default browser alone, as
  `2026-10-29-a-refused-command-keeps-what-was-typed`'s did.
- The client's mutation on the new code in `dm.ts` and `player.ts`: Stryker
  was not run (D16, the owner's rule of 2026-10-26). The stored report
  measured `db81c37`; what the gate's step printed is under Figures.

## What was deliberately left out, and where it went

- The server: unchanged. `mayVoice` in `internal/gateway/authz.go` already
  lets a DM and an agent name any speaker and speak out of character
  (SPEC-013).
- MapTool's command history, recalling a sent line: not taken up, and no
  ticket written; it would be one of its own.
- Enter in `say-as`, and Shift with Enter: no code; under "What could not be
  established".
- The closing paragraph of `docs/verification-debt.md`'s entry "2026-10-25 —
  a repaint took the focus from the box the DM was typing in" names `sayAs`.
  It is a dated record of what closed that entry, and stays.
- The console draws the same Say for the agent as for the DM; nothing marks
  the agent's apart (Task 2's review).
- The client's full mutation run is the owner's to ask for (SPEC-023); under
  "What could not be established".
- SPEC-022: the commit that follows.

## Figures

From the `task check` on `08917d7`, a comment's correction in
`client/e2e/refusals.spec.ts` that follows the last code commit `b70af46` and
`fe713da`'s withdrawal of VTT-539, launched with this report, its Figures not
yet written, and SPEC-022's change in the tree, uncommitted; it exited with
code 0. No Go file differs from `9e33202`.

- Go coverage: 21 packages at or above their floors; the gate notes 13 that
  clear theirs by more than 1.0pp. `internal/gateway` measured 94.4% against
  its 93.5% floor, and `internal/perceive` 95.7% against its 94.5% floor.
- Client coverage: 24 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors.
  - All 15 reused the verdict of their last passing run, nothing in their
    dependency closure differing.
  - The reused verdicts carry 7 timed-out mutants, counted as killed and
    not measured: 4 in `internal/sight`, 1 each in `internal/mcp`,
    `internal/perceive` and `internal/rules`.
  - `tools/mutation-equivalents.txt` is unchanged since `a8bb0ef`, on `main`
    before this ticket.
- Client mutation: not run, by the owner's rule of 2026-10-26 (SPEC-023). The
  log holds no `task: [check:ts-mutation:full]` line, the form a step's start
  takes, and no line naming Stryker. `check:ts-mutation` printed "THE CLIENT'S
  MUTATION WAS NOT MEASURED", that the stored report is out of date and
  measured `db81c37aa307`, and the three tickets added since, this one among
  them. `tools/ts-mutation-equivalents.txt` is unchanged on this branch.
  `entry_faults`, which the gate runs only when it judges a current report,
  run by hand on this tree: no fault over its 63 entries.
- Race: both runs pass.
- Requirements chain: 541 rows, 323 test files, 17 specifications.
- Comments: 339 files, clean, with 2 comment lines added against `main`,
  both in `client/src/view/dm.ts`. Its two notices are the ones the gate
  gave on `2aa7fc3`: `cmd/vtt/events_tail_truncation_test.go` at 22.83
  against 22.6, and `internal/gateway/export_test.go` at 49.18 against
  45.9.
- New prose, which reads Go and TypeScript only: 1809 added lines across 13
  files, clean.
- Disk: 32.7 GiB free just before the launch, and 32.7 GiB by the gate's own
  line when the Go mutation step began.
- Duration: about 970 s, from the launch at 14:52:09 to the log's last write
  at 15:08:19 on 2026-10-30.

`task e2e`, once, against the same tree after the gate: 32 passed and 1
skipped, in 34.2 s. The skipped one is "a network drop while the table plays
on ends in a Reconnect", a `test.skip` in `client/e2e/handover.spec.ts`.
