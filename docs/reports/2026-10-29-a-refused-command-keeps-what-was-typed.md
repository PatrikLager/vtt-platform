# A refused command keeps what was typed, and the DM's speaker stays until changed: the change

**Ticket:** `docs/superpowers/specs/2026-10-29-a-refused-command-keeps-what-was-typed-design.md`.
An agent that did not write it verified it and passed it with gaps G1 to
G8; the ticket's writer corrected G1's MapTool commit and G6's citation
before Task 0's commit.
**Plan:** `docs/superpowers/plans/2026-10-29-a-refused-command-keeps-what-was-typed.md`,
written by that verifier from probes outside the tree, and amended by the
owner's answers to its three questions and by Task 0's review.
**Where it came from:** the four points
`docs/reports/2026-10-25-a-repaint-keeps-what-is-being-typed.md` left out,
under "What was deliberately left out", three raised to the owner and one
sent to `docs/verification-debt.md`; he took them up on 2026-10-29 with
"kör ticket 6".
**The owner's rulings, all 2026-10-29:**
- On the four points:
  - a console select or a focused button losing the focus at a repaint:
    "Nej, låt VTT-492 stå", out of scope;
  - the DM's "in character as" box: "Ja, behåll namnet", only the text
    empties at Send;
  - refused text: "Ja, Say och alla formulär", to the option "Töm bara vid
    ok, överallt där konsolen eller panelen tömmer i dag. Vid vägran står
    texten kvar bredvid felmeddelandet.";
  - the false el() comments: "Rätta alla sju, behåll disable".
- The layout: "Ett ärende, en gren", the comments first.
- On the plan's questions: Q1 "Töm ändå", Q2 "Nej, osynligt lås", Q3 "Ja,
  med de sju". The plan signed off: "Ja, kör".
- At Task 2's review, on a send whose promise rejects: "console.error(e)".

**Last code commit:** `128e4f3`, on `83d9661`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `cb08077` | Task 0: the ticket, the plan, VTT-517 to VTT-523, VTT-493 withdrawn |
| `90e7aeb` | Task 1: the eight comments corrected, the debt entry closed |
| `6d2d281` | Task 2: the console's `sending` and `DMDeps.rerender`, `client/test/support/answers.ts`, the tests, QA's `client/test/qa-refused-command.test.ts`, VTT-524 |
| `a37b2c9` | Task 3: the panel's `submit` and `PlayerUIState.saying`, the tests, QA's `client/test/qa-refused-line.test.ts`, VTT-525 |
| `128e4f3` | Task 4: `client/e2e/refusals.spec.ts`, the waits in `speaking.spec.ts` and `handover.spec.ts`, `typing.spec.ts`'s citation, the bundle |
| `9503aa9` | VTT-525 withdrawn, succeeded by VTT-526; the four rejected-send tests cite VTT-526, and QA's header names its ids |

## What the code did on `83d9661`

- `renderDMConsole` in `client/src/view/dm.ts`: each button that sends a
  command and clears a field called `d.send(...)` and then `clearDraft(...)`
  in the same turn, before any answer. These were Start session, Add actor,
  Remove actor, Add from JSON, Place token, Remove token, a row's Describe,
  Save note, which also set its live visibility select to `""`, and a row's
  Grant. A refused command's text was gone, and dm-view's "a REFUSED start
  still clears the name — the text is gone with the command" pinned it.
- `submit` in `renderPlayerPanel` (`client/src/view/player.ts`) sent, then
  emptied `say-text`, `ui.sayAs` and `ui.sayText` and repainted. Its `send`
  was typed `(c: ClientCommand) => void`, so the panel could not read an
  answer, and a DM's speaker box emptied at every Send.
- Nothing kept a second press from sending: Start session pressed twice sent
  two commands (the plan's P5). Send pressed twice sent one, only because the
  first press had emptied the box.
- Eight comments said that el()'s `text !== undefined` guard forced true
  writes the word "undefined" in a real browser, or leaks text into a
  container.

## The plan's labels, and the tests that carry them

| Label | Test | File |
|---|---|---|
| T1 | "a refused Start session keeps the name as typed, whatever the refusal" | `client/test/dm-view.test.ts` |
| T2 | "a refused command leaves every field of its form as typed" | the same |
| T3 | "a note's key, title and text stay as typed, refused or accepted" | the same |
| T4 | "an accepted Start session empties the name, and asks for one repaint after the answer" | the same |
| T5 | "an accepted command clears the fields of its own form and of no other" | the same |
| T6 | "a control whose command is unanswered sends nothing when pressed again, each row its own" | the same |
| T7 | "a press the console's own guard refuses holds nothing back" and "a send that rejects lets its control go, clears nothing, and reports the error" | the same |
| A1 | "a refused Start session leaves the name typed beside the refusal" | `client/test/app.test.ts` |
| A2 | "an accepted Start session that brings no event empties the name" | the same |
| A3 | "a DM's accepted line keeps the speaker's name in say-as and empties say-text" | the same |
| U1, U2 | "a line the server refuses stays in say-text at the next paint, in character or out" | `client/test/spectator-view.test.ts` |
| U3 | "a line the server accepts empties say-text, with one repaint asked for after the answer" | the same |
| U4 | "a DM's or an agent's say-as keeps its name after an accepted line, and the next line speaks as it" and "a DM's or an agent's refused line leaves say-as and say-text as typed" | the same |
| U5 | "Send or Enter while a line is unanswered sends nothing, until the answer frees them" | the same |
| U6 | "a blank line takes no lock: the line typed after it sends" | the same |
| E1 | "a refused Place leaves its five boxes as typed, beside the refusal" | `client/e2e/refusals.spec.ts` |
| E2 | "a DM's accepted line empties say-text and keeps say-as for the next line" | the same |

Three of the implementers' new tests carry no label of the plan's:
- dm-view's "a press that sends changes nothing the next render draws until
  its answer" and spectator-view's "a press of Send or Enter that sends a
  line draws nothing new until its answer", for VTT-524;
- spectator-view's "a line whose send rejects frees Send, keeps say-text,
  and reports the error", which with T7's second test holds VTT-526.

## Done looks like, answered

1. [x] T1 types `Night One` into `session-name` through an `input` event and
   clicks Start session, with a `send` that answers `ok: false`, once with "a
   session is already open" and once with "wire: not connected". After the
   answer the next render's `session-name` reads `Night One`, one command was
   sent, and no repaint was asked for. In the composed app, A1's socket
   answers `ok: false`: the box reads `Night One` and the toast reads
   `refused: a session is already open`. "a REFUSED start still clears the
   name — the text is gone with the command" is gone.
2. [x] T4, with a `send` that answers `ok: true`: no repaint at the press, one
   after the answer, and the next render's `session-name` reads `""`. A2's
   socket answers `ok: true` with no event: the box reads `""`, with no toast.
3. [x] T2 goes through Start session and the eight other forms, Add actor,
   Remove actor, Add from JSON, Place token, Remove token, a row's Describe,
   Save note and a row's Grant, with selects chosen through `change`. After a
   refused answer the next render reads every field each form fed as typed.
   T5 types every form's fields, presses one form, answers `ok`, and reads
   the next render. The pressed form's cleared fields read `""`, the blank
   option, or for a Describe box the token's present text; every other field,
   another form's or the same form's that its command does not clear, reads
   as typed. T3: Save note's key, title and text read as typed after either
   answer. QA's tests in `client/test/qa-refused-command.test.ts` cover each
   control, refused and accepted, with templated names.
4. [x] U1 and U2, one test over the two lines: a player's `I draw my sword`
   and `/ooc where is the pizza`, each sent with a `send` that answers `ok:
   false`, read in `say-text` at the next paint. U3, over the same two lines
   answered `ok: true`: `say-text` still holds the line right after the
   press, with no repaint asked for; after the answer it reads `""`, with one
   repaint asked for, and again at the next paint.
5. [x] U4's first test, for a DM and an agent: `Grimble` in `say-as`, `Hello`
   in `say-text`, Send, answered `ok: true`. The next paint shows `say-as`
   holding `Grimble` and `say-text` empty, and `Bye`, sent without touching
   `say-as`, carries `as` `Grimble`. A3, in the composed app: a DM types the
   two, presses Enter, and the socket answers `ok: true` with no event;
   `say-as` reads `Grimble` and `say-text` reads `""`.
6. [x] U5 presses Send then Send, Send then Enter, and Enter then Enter, with
   the answer held: one command each time. After a refused answer frees the
   panel, a press sends the next line. For the console, T6 presses each of
   the nine controls, and a second row's Describe and Grant, with the answers
   held: each sends once, a second press of any sends nothing, and neither
   does a press in a console drawn afresh; after the release every control
   sends again. The second rows send while the first rows are held, so each
   row has its own lock.
7. [x] E1: the DM fills Place token with `tok-x`, `nowhere`, `nobody`, `2` and
   `3`, reads the five boxes before the click, and clicks Place. The toast
   begins `refused: `, and the five boxes hold what was typed. E2: through
   `asDM`, the DM starts a session, loads `goblin-ambush` and is granted its
   `act-cutter` as `ACTOR_KIND_NON_PARTY`. The DM types `Grimble` in
   `say-as` and `Hello` in `say-text` and presses Enter: the Story shows
   `Grimble: Hello`, `say-text` reads `""` and `say-as` `Grimble`; `Bye`
   then shows as `Grimble: Bye`. `task e2e` ran whole twice,
   31 passed and 1 skipped each time, the skipped one `handover.spec.ts`'s
   `test.skip`. The last screenshot,
   `client/e2e/.artifacts/r1-refused-place-keeps-what-was-typed.png`, which
   git ignores, shows `tok-x`, `nowhere`, `nobody`, `2` and `3` in the Place
   token row and the toast `refused: engine: token placed in unknown scene
   "nowhere"` below it.
8. [x] At `128e4f3`, `git grep -n -E 'word "undefined"|browser stringifies|harness-blind|both leak|literal word|renders the word' -- client/src client/test/dm-view.test.ts tools/ts-mutation-equivalents.txt`
   prints nothing and exits 1. `git grep -c 'Stryker disable next-line
   ConditionalExpression' -- client/src` counts one in each of
   `client/src/view/dm.ts`, `client/src/view/player.ts` and
   `client/src/view/spectator.ts`, each followed, after its comment lines, by
   `if (text !== undefined) n.textContent = text;`.
   - The reading, Task 1's review (Phase 4b): the two paragraphs of
     `tools/ts-mutation-equivalents.txt` say forced true leaves a new
     element empty in Chromium 143 and in happy-dom 20.11.1, and forced false
     is not equivalent. The three el() comments carry a warning, the two test
     comments in `client/test/dm-view.test.ts` say what each test holds, and
     `client/src/view/join.ts`'s header keeps only its reason for not using
     an el() helper.
   - The entry "A dm-view comment says el()'s guard forced true leaks text"
     of `docs/verification-debt.md` stands above "Open debt", under "2026-10-25
     — a dm-view comment said el()'s guard forced true leaks text", with a
     **Closed by** paragraph.

## What the rules became

- "A box or field … keeps what was typed until the server accepts that
  command; a refusal, or no connection, leaves it as typed. This withdraws
  VTT-493": VTT-517 for the console and VTT-518 for the panel's `say-text`,
  and VTT-522 for what a repaint shows. VTT-493 is withdrawn, dated
  2026-10-29, and succeeded by VTT-518, VTT-520, VTT-521 and VTT-522. No
  connection earned no row: `Wire.send` answers `ok: false`, which the client
  cannot tell from a refusal, and T1 runs with "wire: not connected".
- "A command the server accepts clears what was typed in the fields that fed
  it …, except the speaker box": VTT-519 for the console, the notes form's
  key, title and text excepted as the ticket's item 3 has them, and VTT-520
  for `say-text`.
- "A DM's or an agent's speaker box keeps what it holds after Send, until it
  is changed": VTT-521, and VTT-522's half for `say-as`.
- "One press … sends one command, however often it is pressed before the
  answer": VTT-523, which names the nine controls, and Send or Enter in
  `say-text`.

Two rows came after sign-off, at QA's request; the plan lists neither:
- **VTT-524**, the owner's "Nej, osynligt lås" (Q2): a press that sends a
  command changes nothing the console or the panel draws until that command
  is answered. Asked for by Task 2's QA.
- **VTT-525**, the owner's "console.error(e)": a command the console or the
  panel sent whose promise rejects clears nothing, asks for no repaint, frees
  the control that sent it, and is reported with `console.error`. Asked for
  by Task 3's QA. Withdrawn on 2026-10-30, by `9503aa9`, after SPEC-022's
  reading review found it false as worded: End session, Delete note and the
  panel's ability button do not catch a rejected send. **VTT-526** succeeds
  it, naming the nine console controls and the panel's Send and Enter in
  `say-text`.

The sort refused, under the plan's "Requirements": no connection as a rule
of its own; the refusal's toast, which is `act`'s and older than this
ticket; the corrected comments, which the ticket says add no rule; how the
repaint and the lock work (D6, D8); Save note's key, title and text,
covered by VTT-517 and VTT-519's exception; a DM with no actor and no Say
form, older and out of scope; and MapTool's history. C8 was not allocated,
by Q1's answer.

## Phase 4a

One QA agent per task that delivered behaviour, Tasks 2 and 3. Each was
given the requirements, the ticket, SPEC-022 and the surface as signatures
and names, never the source. Task 0's records, Task 1's comments and Task
4's tests and bundle had no behaviour of their own to derive. Task 2's QA
read the interface declarations in
`client/src/session.ts` and `client/src/metadata.ts`, types its handoff named
without their fields, and said so.

| Task | Tests, as committed | Failures |
|---|---|---|
| 2 | 79 in `client/test/qa-refused-command.test.ts` | 0 |
| 3 | 30 in `client/test/qa-refused-line.test.ts` | 0 |

Task 2's QA made fourteen injections and Task 3's more than thirty, each red
where it aimed. Neither found a behaviour defect. The tree, less the QA
file, hashed the same before and after each.

### QA adjudications

"SPEC-022 says it" means SPEC-022 as of the commit that follows this report.

Task 2's QA, in its own numbering:
- QA finding 1: spec ambiguity — the handoff was wrong, not the code: the
  console's selects carry classes, not a `data-field`, and the kind is
  `ACTOR_KIND_NON_PARTY`, not `ACTOR_KIND_MONSTER` — nothing changed; QA's
  tests use the classes.
- QA finding 2: requirement needing an ID — the owner's Q2 answer, that
  nothing is drawn differently while a command is unanswered, had no row —
  VTT-524 allocated, and QA's two comments that cited the ruling name it.
- QA finding 3: spec gap — SPEC-022 does not say what repaints the console
  after an accepted answer — `DMDeps.rerender`, by the plan's D6 — SPEC-022
  says it.
- QA finding 4: spec gap — whether VTT-523's lock is per control or per row
  for Describe and Grant — per row, by D8's keys — SPEC-022 says it.
- QA finding 5: spec gap — what an accepted Remove actor or Remove token
  leaves in the rest of its form — as typed, as before this ticket, and T5
  reads it — SPEC-022 says it.
- QA finding 6: spec gap — what a send whose promise rejects does — fields
  kept and control freed, then taken to the owner at Task 2's review, whose
  "console.error(e)" became VTT-525, now VTT-526 — SPEC-022 says it.
- QA finding 7: spec gap — a press the lock blocks gives no notice and asks
  for no repaint — consistent with VTT-524, no row — SPEC-022 says it.
- QA finding 8: spec ambiguity — SPEC-022 said Describe clears its row and
  Send empties both Say boxes at the press, against the ticket — the ticket
  stands — SPEC-022 says the server's acceptance clears them.

Task 3's QA, in its own numbering:
- QA finding 1: requirement needing an ID — the owner's ruling on a rejected
  send: frees the control, asks for no repaint, `console.error` — VTT-525
  allocated, held by the console's, the panel's and QA's rejected-send tests,
  which cite its successor VTT-526 since `9503aa9`.
- QA finding 2: spec gap — the panel asks for a repaint after an accepted
  line — as built — SPEC-022 says it.
- QA finding 3: spec gap — the panel asks for none after a refusal — as
  built — SPEC-022 says it.
- QA finding 4: spec gap — what "frees the form" means — the next Send or
  Enter sends — SPEC-022 says when the lock ends.
- QA finding 5: spec gap — a refusal is not logged — as built: it goes to
  the toast, and only a rejection to `console.error` — SPEC-022 says it.
- QA finding 6: spec gap — SPEC-022 still states VTT-493 — withdrawn —
  SPEC-022 drops it.

After Phase 4a the QA files changed by review findings:
- Task 2's: none beyond naming VTT-524.
- Task 3's: a test that cited VTT-501 asserts `outOfCharacter`; a
  rejected-send test cites VTT-525, now VTT-526; the header names the ids
  it holds.

## Phase 4b

One reviewer per task, by command, with probes outside the tree.

| Task | Pass 1 | Pass 2 |
|---|---|---|
| 0 | 3 Must, 7 Should, 14 Nits | 4 Should, 9 Nits |
| 1 | 4 Should, 5 Nits | — |
| 2 | 1 Must, 3 Should, 5 Nits | — |
| 3 | 3 Should, 5 Nits | — |
| 4 | 3 Nits | — |

Every Must and every Should was taken. Of the Nits, two of Task 1's and one
of Task 3's were for this report, which carries them: the D2 and D5 rows
under Deviations, and VTT-524 and VTT-525 with the ruling each carried, under
the rules. One of Task 4's is for the specification's review. Two of Task
2's changed nothing:
- a `send` that throws instead of returning a promise keeps its control
  held. The reviewer found no path in the app to it; no code (below, under
  what was left out);
- a pointer added to `DMDeps.send`'s doc came back out: that block is over
  six lines, and `check:comments` refuses a block of more than six lines to
  which the change adds one.

The findings that changed the work:
- **Task 0.** The Musts: VTT-517 and VTT-518 said a refused field holds what
  it held at the send, false when typed into before the answer; VTT-523 named
  every console control though only nine are locked, and missed Enter after
  Enter; the ticket's item 8 asked the comments to carry the measurement,
  which the plan's D2 and D3 forbid, and was rewritten, its grep gaining
  `renders the word`. Among the Shoulds: VTT-522 was false for the accepted
  line's own repaint; item 3 said "empty" where a select returns to the blank
  option and a Describe box to the token's text; the rule still said VTT-493
  moves rather than is withdrawn; D2 now says the directive hides forced
  false. At the second pass: VTT-522's new half was cited on no test that sees
  it; the lock test pressed three of the nine controls; the ticket's fourth
  rule still covered Delete note; SPEC-022's id range hid VTT-493. The rows
  were reworded in place before the commit.
- **Task 1.** Stryker stores only the directive's first line as the reason,
  for the forced false mutant too, so that line now reads "only forced true
  is equivalent". Forced false, made by hand in each view, turned 28, 89 and
  125 tests red, in `dm.ts`, `player.ts` and `spectator.ts`. The debt
  heading took the entry's recorded date and its lead in the past tense; the
  closing paragraph stopped crediting both test comments with a browser
  claim; `join.ts` named a "shared" el(), which is three file-private copies.
- **Task 2.** The Must: VTT-523's and VTT-524's evidence listed console tests
  while the panel still emptied and repainted at the press; both stayed OPEN
  until Task 3. The Shoulds: the rejected send was caught silently, which the
  owner turned into "console.error(e)"; VTT-524's one cited test saw one
  field, so the console's test over every form was written; VTT-524's
  wording was false of a press that sends nothing, and was reworded before
  the commit.
- **Task 3.** "qaSay: a blank Send leaves the boxes as its sending decides"
  still encoded VTT-493's emptying, and became "qaSay: a blank Send sends
  nothing and leaves both boxes as typed", with a press that sends. A test's
  name hedged on what it asserts, and became "qaSay: an accepted Enter in the
  text box keeps the 'as' box and empties the text box". U3 did not observe
  "after the answer"; it now reads the box and the repaint count right after
  the press. The reviewer ran seventeen mutants of `submit`, and probed the
  composed app: a socket's close frees the lock; a party switch while a line
  is unanswered speaks as the party member chosen at the press; the second
  paint of an accepted line is needed, since K12 reds A3, and keeps the
  focus.
- **Task 4.** In a scratch copy, clearing the console's fields at the press
  turned E1 red at the first box after the toast, and clearing `say-as` at an
  accepted answer turned E2 red; a rebuild there matched the committed bundle
  byte for byte. Two comments of `refusals.spec.ts` were made exact. The
  third Nit, that its pointer "SPEC-022 is the record of what a refused or an
  accepted command leaves typed" holds only once SPEC-022 is updated, is for
  the specification's review.

## The deliberate breaks

Each was made by an exact edit, confirmed by grep, run, undone by the
inverse edit, checked by grep again, and compared with `cmp` against the
saved file: identical every time. Task 2's ran over the fifteen test files
of the plan's P7, and Task 3's over the whole client suite, each before that
task's QA file existed; the rows of later breaks say when they ran. At
`128e4f3` QA's files redden as well: K9 reddens U5 and six tests of
`client/test/qa-refused-line.test.ts`.

| Break | What it did | Red |
|---|---|---|
| K1 | the console's `ok` check removed, so drafts clear on any answer | T1, T2, A1 |
| K2 | `clearDraft` before `d.send` | T1, T2, A1, and T7's rejected-send test |
| K3 | `d.rerender()` removed | T4, A2 |
| K4 | the accepted branch clears nothing | T4, T5, A2, and 14 existing tests that read a cleared draft |
| K5 | the panel empties `say-text` on any answer | U1 and U2, and U4's refused test |
| K6 | the panel does not empty `say-text` on `ok` | U3, U4's accepted test, A3, and 9 updated tests; after Task 3's review, "qaSay: a second Send sends the text typed since the first's answer, as the same speaker" too |
| K7 | `ui.sayAs = ""` put back at the press | both U4 tests, A3, the panel's VTT-524 test, and 6 updated tests |
| K8 | the console's lock check removed | T6 alone |
| K9 | the panel's lock check removed | U5 alone |
| K10 | the console's lock never freed | T6, at the press after the release; 36 tests over the run, as held locks leaked into later tests |
| K11 | the console's lock taken before its guards | T7's guard test; 38 tests over the run, for the same reason |
| K12 | no `rerender` after the panel's `ok` | U3, A3, "qaSay: Enter's repaint leaves the focus in the new text box", and "Send narrates the trimmed text, and an accepted line clears the box and repaints" |
| — | `console.error(e)` replaced by `void e` in the console | T7's rejected-send test |
| — | the same in the panel | "a line whose send rejects frees Send, keeps say-text, and reports the error" alone |
| — | the console clears its drafts at the press, at Task 2's review, over `dm-view.test.ts` | the console's VTT-524 test, T1, T2 and T7's rejected-send test, once the VTT-524 test read the fields' values; on `outerHTML` alone it stayed green. Over P7, A1 too |
| — | the panel empties `say-text` and `ui.sayText` at the press | the panel's VTT-524 test, U1 and U2, U4's refused test, the panel's rejected-send test |
| — | the panel empties only `ui.sayText` at the press | the panel's VTT-524 test, at the next paint's values |
| — | the panel's lock never freed | U5's release half, the panel's rejected-send test, U4's accepted test, "only /ooc as the line's first word steps out of character", and the second-Send test of K6 |
| — | the panel's lock taken before its blank check | U6, "only /ooc as the line's first word steps out of character", "sending narration requires text, and clears the box once it is accepted" |
| — | a repaint at the panel's press, at Task 3's review | five tests, U3 among them once it read the box at the press |
| — | Task 2's first handler of a rejection removed, before the review replaced it | T7's rejected-send test, "Unhandled error between tests" |

Every test a break was predicted to redden went red.

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c`, `CommandPanel.java`:
- **The chat box empties at send.** `commitCommand()` trims the line, adds it
  to `commandHistory` unless it repeats the last, calls
  `commitCommand(command, null)` and sets `commandTextArea` to `""`.
- **Its local refusals do the same.** When `commitCommand(String,
  MapToolMacroContext)` drops the line for `CHEATER_PATTERN` or for its
  `</div>` count, the box still empties, and the up and down arrow keys
  recall the line from the history. No server refuses a chat line there, so
  MapTool has no answer for a refused one.
- **A second Enter is harmless**, because the box is already empty and an
  empty line is dropped.
- **The speaker is `globalIdentity`**, a `TokenIdentity`, kept until
  `setGlobalIdentity` sets another or `clearGlobalIdentity` clears it.

Borrowed: the speaker that stays (VTT-521). Not borrowed: the box that
empties at send, because here a server can refuse a line, and by the owner's
ruling the text stays. The history is not taken up. MapTool's dialogs were
not examined.

The project's own precedent for a press while an answer is awaited is the
join form: `JoinViewState.busy` in `client/src/view/join.ts` draws the Join
button disabled and labelled "Joining…", and `submit` in `runJoin` in
`client/src/app.ts` returns early while it is set. Q2 asked whether to draw
the console and the panel that way; the owner answered "Nej, osynligt lås".

## The plan's gaps and questions

- **G1.** The ticket's MapTool commit, `63862e4`, is the HEAD of
  `~/dev/RPTool`, which tracks no file of `maptool`; `maptool` is its own
  repository, at `f4b7fef6c`. Corrected in the ticket before Task 0's commit.
- **G2.** An eighth place carried the false claim: `client/src/view/join.ts`'s
  header, hidden from the ticket's first grep by a line break. Q3.
- **G3.** VTT-493's subject was both Say boxes, so a speaker box that keeps
  its text made it false for `say-as` too. Withdrawn at Task 0 and succeeded
  by VTT-518, VTT-520, VTT-521 and VTT-522.
- **G4.** Item 6, item 2 and the accepted halves of items 3 and 4 held on
  `83d9661`, so they guard the change rather than fail before it. Start
  session pressed twice sent two commands, which no item observed: T6 does.
- **G5.** The ticket named three of the eight test files that build a panel
  and one of the seven that build a `DMDeps`. Tasks 2 and 3 changed the rest,
  each `send` answering and each `DMDeps` given `rerender`.
- **G6.** "Two further mutants, el()'s" begins a sentence, not a paragraph.
  Corrected in the ticket.
- **G7.** Only Chromium is installed for Playwright, so Firefox and WebKit
  were not run (below).
- **G8.** The directive also hides the guard forced false, which is not
  equivalent: el() would write no text at all, so its headings and labels,
  and the panel's chips, would read blank. The plan had it by reading;
  Task 1's review measured it (Phase 4b), and each directive's first line
  says only forced true is equivalent.

- **Q1**, something typed into a box after Send and before the answer: "Töm
  ändå". An accepted answer empties it as well; C8 was not allocated. QA's
  templated tests "typed into while unanswered, an accepted answer still …"
  and "… a refusal keeps the latest …" hold both halves for each console
  control, and "an accepted answer empties say-text typed into while
  waiting" and "a refused answer keeps what was typed while waiting" for the
  panel.
- **Q2**, whether the screen shows an unanswered command: "Nej, osynligt
  lås". A further press of the control sends nothing, and nothing is drawn
  differently: VTT-524.
- **Q3**, whether `join.ts`'s header is corrected with the seven: "Ja, med de
  sju". Task 1 cut its false parenthetical.

## The measurement the comments no longer carry

On 2026-10-29, at `83d9661`, outside the tree. The owner's probe, which
widened the ticket's point about the comments, was rerun by the plan's
verifier as he had run it: copied into `client/`, run with `env -u
CLAUDECODE bun`, and removed.
- Chromium through Playwright, HeadlessChrome 143.0.7499.4: a `div` holding a
  `span`, given `textContent = undefined`, reads `""` with 0 child nodes; an
  empty `div` given `null` reads `""`.
- happy-dom 20.11.1, which the client's tests run on: the `div` holding a
  `span` reads `""` with 0 child nodes; a new `div` given `undefined` reads
  `""`; an empty `div` given `null` reads `""` with 0 child nodes, run by Task
  0's second reviewer. `click()` on a disabled button delivers no click,
  which Q2's alternative would have relied on.

`textContent` is a nullable `DOMString`, so `undefined` reaches it as `null`
and replaces the element's children with nothing. el() is handed a new,
empty element, so forced true changes nothing there; on an element with
children it would remove them, which is what each el() comment now warns
of. `tools/ts-mutation-equivalents.txt` carries the result and cites the
plan's P3 and P4.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The plan's requirements: C1 to C7, VTT-517 to VTT-523 | VTT-524 and VTT-525 as well, VTT-525 since succeeded by VTT-526 | Task 2's QA found the owner's Q2 answer held by no row, and Task 3's QA his ruling on a rejected send |
| D8: a lock is freed when the send's promise settles; nothing said what else a rejection does | A rejection clears nothing, asks for no repaint, frees its control and goes to `console.error`, in the console and the panel (VTT-526, which succeeds VTT-525) | The implementer first caught it silently, as `app.ts` catches `session.reconnect()`. Task 2's reviewer ruled that unsound: a rejection from `act` comes from a throw in its paint or in writing the command, so a silent catch hides a bug. The owner chose "console.error(e)" |
| T7, one test; the plan said its rejected half holds on Task 1's commit | Two tests, and the rejected half was red there | `d.send(...)` left a rejection unhandled, and bun fails a test on one: a `process.on("unhandledRejection")` listener was never called, and `reportError` in a `.catch` failed it too. The guard test runs over the eight guarded forms, since Describe has none |
| Tasks 2 and 3: the plan's T, A and U tests | Also the two VTT-524 tests and the panel's rejected-send test | VTT-524 and VTT-525 needed observations. The two VTT-524 tests and the panel's rejected-send test were red on the commit before their code |
| The console's VTT-524 test compares the next render | It compares the next render's `outerHTML` and each field's value | Its first version compared `outerHTML` alone and stayed green when the drafts were cleared at the press: `outerHTML` carries an input's `value` attribute, not the `.value` that typing sets |
| D8: a press the console's own guard refuses takes no lock | Guards return `void d.notify(...)`, and a press is typed `() => Sending \| undefined` | The test harness's `notify` returned the array's new length, which reached `sending` as a command; 22 dm-view tests went red on the first run |
| G5: the six `DMDeps` files' `send` already resolve a result | `client/test/qa-describe-token.test.ts`'s answered `{}`, and now answers `ok()` | An answer with no `ok` clears nothing, and "qaDT: sending clears that row's four drafts and leaves another row's" went red |
| The plan's list of existing tests that change | Also "a saved note carries the visibility chosen" | It pressed Save in two consoles in one turn, and the second press found `upsert-note` held; it now awaits each answer |
| The plan renamed three tests | Seven more were renamed, listed below | Each old name said the press empties a box, which the accepted answer now does, or that `say-as` empties. No old name is cited in the register, a specification or `client/` |
| D12: `speaking.spec.ts`, and any other spec that refills a box before its command's answer | `client/e2e/handover.spec.ts` too, covered by D12's "any other spec", not named in it | Its first test's loop refilled Add actor's and Place token's boxes after two fixed 300 ms pauses, not after the answer. It now waits for `actor-id` and `token-id` to read `""` |
| The plan's "For Phase 4a": QA's tests go in `client/test/qa-refused-command.test.ts` | Task 3's QA wrote `client/test/qa-refused-line.test.ts` | Task 2's QA file holds the console; the panel's QA was briefed with its own file, so the two never collide |
| E1 reads the boxes after the click | It reads them before the click as well | A fill that a repaint lost leaves x or y empty, which sends 0 and is refused too; E1 must fail there rather than pass |
| D2: the el() reason at most two lines | Three: the directive's line and two more | The first line is the reason Stryker stores for both mutants, so it says only forced true is equivalent; the other two say that holds only while `n` is empty, that the directive hides forced false, and when to drop it |
| D5: the **Closed by** paragraph names Task 1's commit | It names the plan's Task 1 | The paragraph was written inside that commit, before its hash existed. The commit is `90e7aeb` |

The seven renames beyond the plan's, each old name first:
- in `client/test/spectator-view.test.ts`, "sending narration requires text,
  and clears the box afterwards" to "sending narration requires text, and
  clears the box once it is accepted", and "Send narrates the trimmed text,
  clears the box and repaints" to "Send narrates the trimmed text, and an
  accepted line clears the box and repaints";
- in `client/test/qa-say-boxes.test.ts`, "qaSay: after Send both boxes are
  empty at the paints that follow" to "qaSay: after an accepted Send the text
  box is empty and the 'as' box kept at the paints that follow"; "qaSay: a
  second Send sends only what was typed since the first" to "qaSay: a second
  Send sends the text typed since the first's answer, as the same speaker";
  "qaSay: a blank Send leaves the boxes as its sending decides" to "qaSay: a
  blank Send sends nothing and leaves both boxes as typed"; and "qaSay: Enter
  in the text box leaves the boxes as its sending decides" to "qaSay: an
  accepted Enter in the text box keeps the 'as' box and empties the text
  box";
- in `client/test/qa-speaking.test.ts`, the template
  `` qaSpk: a ${me.role} Send empties both boxes `` to
  `` qaSpk: a ${me.role} Send keeps the as box and empties the text box ``.

## What could not be established

- Firefox and WebKit (G7): `playwright.config.ts` names no browser project
  and only Chromium is installed, so the e2e ran Chromium alone, and what
  `textContent = undefined` does in the other two was not measured.
- How long the gap between a send and its answer is, and how often a box is
  typed into during it. Under "Töm ändå", an accepted answer empties what was
  typed there.
- No browser test holds an answer. E1 would pass a client that cleared at
  the press and refilled at a refusal; that interval is VTT-524's, held by
  unit tests alone (Task 4's review). Playwright's `page.routeWebSocket`
  could hold one; it was not tried.
- A result the gateway fails to encode is not written (the plan's P9, by
  reading), so the control that sent it stays held until the socket closes.
  Not observed.
- A `rerender` that throws after an accepted answer is not passed to
  `console.error`: VTT-526 covers a send whose promise rejects, and the throw
  comes after it resolved, so its rejection is unhandled. It needs a paint
  that throws at the second of the two paints an accepted answer makes and
  not at the first. No ruling was asked for and no code written.
- The e2e runs: after the second, two comments of
  `client/e2e/refusals.spec.ts` were reworded at Task 4's review; the run
  after the gate, under Figures, is the first on the reworded file, and it
  passed. The reviewer's copy of the file from before those edits differs
  from `128e4f3`'s in those two comments alone.
- The client's mutation on the new code in `dm.ts` and `player.ts`: Stryker
  was not run, by the owner's rule of 2026-10-26. `python3
  tools/ts-mutation-report.py check` on `128e4f3` prints that the stored
  report is out of date and measured `db81c37`, and names this ticket among
  those since.

## What was deliberately left out, and where it went

- A console select or a focused button still loses the focus at a repaint,
  as VTT-492 requires, by the owner's "Nej, låt VTT-492 stå".
- A `send` that throws instead of returning a promise keeps its control held:
  the console's key stays in `unanswered`, and the panel's
  `PlayerUIState.saying` stays set. `submit` also takes its lock before it
  builds the command, so a builder that threw would do the same. Task 2's
  reviewer found no path in the app to the first, and Task 3's called the
  second near-theoretical. Near-theoretical with a small consequence, so no
  code, no test and no debt entry, by the owner's ruling of 2026-10-21 on
  such cases.
- The console's controls that clear nothing today, End session, Revoke, the
  join door, rotating the join link, promotion, Delete note, a condition's ✕,
  and map and adventure loading, are unchanged, as the ticket says. A
  rejected send from one of them is unhandled, as before, and so is one from
  the panel's ability button, which ignores its answer: `git grep -n
  '\.catch(' -- client/src/view/dm.ts client/src/view/player.ts` prints
  nothing, and only `sending` and `submit` hand `.then` a rejection handler.
- The DM's browser console has no Say form, and a DM who controls no actor
  has none in the panel either, so the speaker rule is seen only on a DM's
  panel that has one.
  `docs/reports/2026-10-26-a-player-speaks-as-their-character.md` raised it
  to the owner under "What was deliberately left out"; it is not this
  ticket.
- MapTool's command history, recalling a sent line with the arrow keys, is
  not taken up. No ticket was written; it would be one of its own.
- `DMDeps.send`'s doc says its promise is load-bearing for the controls that
  read state back, and does not name the nine forms that now wait for it. A
  pointer added at Task 2's review came back out (Phase 4b). SPEC-022, from
  the commit that follows, records that the nine controls send through
  `sending` and wait for the answer.
- The client's full mutation run is the owner's to ask for (SPEC-023); the
  tickets since `db81c37` are the two `check` names.
- SPEC-022: the commit that follows this report.

## Figures

From the `task check` on `2aa7fc3`, the ledger's commit, which follows the
last code commit `128e4f3` and `9503aa9`'s withdrawal of VTT-525, launched
with this report, its Figures not yet written, and SPEC-022's change in the
tree, uncommitted; it exited with code 0. No Go file differs from `83d9661`.

- Go coverage: 21 packages at or above their floors; the gate notes 13 that
  clear theirs by more than 1.0pp. `internal/gateway` measured 94.1%
  against its 93.5% floor, and `internal/perceive` 95.7% against its 94.5%
  floor, as on `1268bee`.
- Client coverage: 24 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors.
  - All 15 reused the verdict of their last passing run, nothing in their
    dependency closure differing.
  - The reused verdicts carry 7 timed-out mutants, counted as killed and
    not measured: 4 in `internal/sight`, 1 each in `internal/mcp`,
    `internal/perceive` and `internal/rules`.
  - `tools/mutation-equivalents.txt` is unchanged since `a8bb0ef`, on
    `main` before this ticket.
- Client mutation: not run, by the owner's rule of 2026-10-26 (SPEC-023). The
  log holds no `[check:ts-mutation:full]` line and no line naming Stryker.
  `check:ts-mutation` printed "THE CLIENT'S MUTATION WAS NOT MEASURED", that
  the stored report is out of date and measured `db81c37aa307`, and the two
  tickets added since: the 2026-10-26 ticket on the client's mutation gate
  and this one. `tools/ts-mutation-equivalents.txt` changed only in Task 1's
  two comment paragraphs. `entry_faults`, which the gate runs only when it
  judges a current report, run by hand on this tree: no fault over its 63
  entries.
- Race: both runs pass.
- Requirements chain: 526 rows, 321 test files, 17 specifications.
- Comments: 339 files, clean, with 21 comment lines added against `main`.
  Its two notices are the ones the gate gave on `1268bee`:
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6, and
  `internal/gateway/export_test.go` at 49.18 against 45.9.
- New prose, which reads Go and TypeScript only: 2148 added lines across 27
  files, clean.
- Disk: 33.5 GiB free just before the launch, and 33.4 GiB by the gate's own
  line when the Go mutation step began.
- Duration: about 959 s, from the launch at 09:42:21 to the log's last write
  at 09:58:20 on 2026-10-30.

`task e2e`, once, against the same tree after the gate: 31 passed and 1
skipped, in 33.1 s. The skipped one is "a network drop while the table plays
on ends in a Reconnect", a `test.skip` in `client/e2e/handover.spec.ts`.
