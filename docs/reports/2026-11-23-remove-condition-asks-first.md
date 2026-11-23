# The console's Remove condition asks the DM first — report

Ticket: `docs/superpowers/specs/2026-11-23-remove-condition-asks-first-design.md`.
Plan: `docs/superpowers/plans/2026-11-23-remove-condition-asks-first.md`.
Branch `fix/remove-condition-asks-first` from `3ebb59c`, the last commit of
the unmerged `fix/delete-note-asks-first`. Commits: `1168445` (Task 0:
ticket, plan, register) and `162a389` (Task 1: the question, the lock, its
tests, the browser test's answer, the register's cells and the rebuilt
bundle; the last code commit). Every code reference below is to `162a389`.
SPEC-022 moves next, written from this report.

## Done looks like

1. [x] **Remove condition asks first.** Each button of the "Remove condition"
   group in `renderDMConsole` (`client/src/view/dm.ts`) presses through
   `sending`, keyed `remove-condition:<actor>:<condition>`. It asks `Remove
   <condition> from <actor>? Nothing in the console puts it back.` through
   `DMDeps.confirm`, the two names the button's label is built from; a no
   returns nothing, a yes returns `remove_condition` for the actor's and the
   condition's ids with `clears: []`.
   - `client/test/dm-view.test.ts`'s "a removal button asks first, naming
     the actor and the condition as it reads them, and a no sends nothing"
     (VTT-964), and "a condition on an actor gets a removal button that sends
     removeCondition" and "a removal button named by the list still sends the
     actor's and the condition's ids", which now assert one question and one
     send (VTT-965).
   - Independently: `client/test/qa-remove-condition-asks-first.test.ts`.
   - On `1168445` each failed: the first asked nothing and sent on a no, the
     other two asked nothing.
2. [x] **An unanswered removal holds its button back.**
   - "a removal button pressed again while its command is unanswered asks
     nothing and sends nothing, holds back no other, and sends once answered"
     (VTT-966, VTT-541), and the walk "a control whose command is unanswered
     sends nothing when pressed again, each row its own", which `FORMS` and
     the walk's own list now take a removal button of each actor through.
   - "a removal button whose send rejects lets it go, asks no repaint, and
     reports the error, which goes no further" (VTT-968).
   - On `1168445` each failed: a held press sent again, and each rejected
     send reached the window as an `error` event (G1).
   - The drawing walk (VTT-967) and the refusal walk (VTT-969) take the
     removal button in too; both were green on `1168445`.
3. [x] **The browser test still removes a condition.**
   `client/e2e/conditions.spec.ts`'s "a DM's window names a condition a hit
   applies on its dot, its Story, its Events panel and its removal button"
   answers the question through `page.once("dialog", …)`, still sees `Dummy
   lost …` in the feed and the dot gone, and asserts the question's words;
   green in `task e2e` on `162a389`.
4. [x] **The gate.** `task check` and `task e2e` are green on `162a389`.
   `cmd/vtt/webdist` is rebuilt in that commit, and `check:drift`, a step of
   `task check`, finds no difference. `check:comments` is clean, no ledger row
   moved, and D6's ledger step did not run. The Go mutation gate measures no
   client code, and `check:ts-mutation` judged no client report: the stored
   one measured `d1ff728` and is out of date (SPEC-023).

## What the rule became

The ticket's first rule became VTT-964, its second VTT-966 and its third
VTT-968; item 1's yes became VTT-965, and the sort added VTT-967 and VTT-969
(the plan's D1). As the register words them:

- **VTT-964.** A button of the console's "Remove condition" group, pressed
  while no `remove_condition` it sent is unanswered, asks the DM first, in a
  message holding the actor and the condition as the button reads them, and
  sends nothing when the DM declines.
- **VTT-965.** A button of the console's "Remove condition" group the DM
  accepts sends one `remove_condition` naming the actor's id and the
  condition's id.
- **VTT-966.** While a `remove_condition` a button of the console's "Remove
  condition" group sent is unanswered, pressing that button again sends
  nothing.
- **VTT-967.** A press of a button of the console's "Remove condition" group
  that sends a `remove_condition` changes nothing the console draws until
  that command is answered.
- **VTT-968.** A `remove_condition` a button of the console's "Remove
  condition" group sent, whose promise rejects, clears nothing, asks for no
  repaint, frees that button, and is reported with console.error.
- **VTT-969.** When the server refuses a `remove_condition` a button of the
  console's "Remove condition" group sent, the console asks for no repaint of
  its own.

VTT-523, VTT-524, VTT-526, VTT-538 and VTT-864 stand as worded (the plan's
D9). Refused by the sort: the question's words as a row (the tests pin
them); that a held press asks nothing (a consequence of where the question
sits); that a removal holds back no other button (VTT-541 states it, and the
held-press test joins its evidence); the agent's MCP `remove_condition`
asking nothing (a non-goal); that the question names what the label read at
its paint (how, not what); and ids holding `:` sharing a lock (an almost
theoretical edge case). One more the sort refused on a reason that did not
hold: see the deviations.

## Rule 9

MapTool sets and clears a token's state from its popup menu, one checkbox
per state (`ChangeStateAction` in `TokenPopupMenu`), and clears them all
with "Clear State" (`ClearStateAction`), asking nothing; a search of
`i18n.properties` for a confirmation naming a state finds none. Its answer
does not fit: its menu sets a cleared state again with one press, and the
console has no control that applies a condition. Borrowed: nothing.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as verified | Its problem statement corrected before Task 0's commit | The verifier's notes: a DM's window draws the player panel too, so `use_ability` is sent from a DM's window as well (N1); and a rejected send is caught by happy-dom in the client's tests, so "caught by nothing" holds of the console alone (N2) |
| The sort refused a row "an accepted removal clears no field", on the reason that VTT-533 and the acceptance walk hold it | No row holds it for the fields of the forms in `FORMS`, which the acceptance walk types; VTT-522 and VTT-534 hold it for `say-as` and `say-text`. The QA file's test of it cites the plan's D5 alone | Task 1's reading review: VTT-533 binds only the fields that fed a command, and no field feeds a removal button, so under `clears: ["actor-json"]` VTT-533 would still be true as worded. The acceptance walk turns red under that break. A search of the register's rows for `repaint`, `typed` and `clears` finds none binding the fields of the forms in `FORMS` for a command they did not feed; VTT-522 and VTT-534 bind `say-as` and `say-text` after any repaint |

## History cut from the code (rule 10)

None: the removal button carried no comment before.

## The deliberate breaks

Each applied, seen red, and restored from a saved copy compared byte for byte:
the question dropped (the no test, both press tests, the held-press test);
asked in a wrapper outside `sending` (the held-press test); one key for the
group, a key by the condition alone and the key emptied (the held-press test
and the lock walk); a key by the actor alone (the held-press test); sent
outside `sending` (the held-press, rejection and lock-walk tests); the actor,
or the condition, by its id in the question (the no test); a repaint at the
press (the drawing and refusal walks and the rejection test); `note-key`
cleared on a yes (the acceptance walk); the ids swapped in the command (both
press tests and the held-press test); yes and no reversed (every test that
presses).

## Phase 4a adjudications

- No defects. Its tests in `client/test/qa-remove-condition-asks-first.test.ts`
  land: names holding quotes, commas, markup and ampersands, read literally;
  each fallback to an id; no list; an actor named `""`; a decline followed by
  a yes; the ids on the wire; a held button at its own paint and a later one;
  two conditions on one actor and one condition on two actors, each held while
  the others are pressed; every other console button pressed with a removal
  held, and every other control's unanswered command beside a removal button;
  what is drawn until the answer and the repaint after an accepted one; a
  refused, an accepted and a rejected removal.
- QA raised readings of the rows, each adjudicated here; no row changed.
  - VTT-864's "the actor's name in the fold, else its id" for an actor named
    `""`: its test "a removal button names an actor with an empty name, or one
    the fold lacks, by its id" pins the id.
  - A condition held for an actor id the fold lacks: the fold refuses a
    `conditionApplied` for an unknown actor and drops an actor's conditions
    with it at `actorRemoved`, and the same test pins the label by its id.
  - What VTT-968's console.error reports: `sending` hands it the rejection;
    dm-view's rejection test asserts the error itself.
  - The one repaint after an accepted removal: the plan's D1 names it a
    consequence of `sending`, held by no row, as for Delete note.
  - That a held press asks nothing: refused as a row (a consequence of where
    the question sits); the held-press tests assert it.

## Task 1's reading review

- It asked for the QA test "an accepted removal clears no field typed
  anywhere in the console" to leave VTT-533's evidence, and the QA comments'
  VTT-533 and VTT-517 to go, since neither row binds a command no field feeds
  (the second deviation). It asked the QA comments that pointed to "the
  plan's" decisions to name the plan by its path, and a comment claiming a
  check its test does not make to lose that clause. Each is done.

## The plan's gaps

- **G1.** happy-dom catches a rejected promise a click listener returns,
  passes it to `console.error` and dispatches an `error` event at the window;
  so the rejection test also counts the window's `error` events, which the
  base dispatched and the change does not.
- **G2.** The ticket's open points: the words are OD-1 A; the naming is D2;
  the lock is D3, forced by VTT-541; how the rows take the removal in is D9.
- **G3.** `tools/ts-mutation-equivalents.txt` (read, not edited), the
  comment ceilings (binding, no row moved), the QA file and this report,
  beside the ticket's list.
- **G4.** "Once that command is answered it sends again" held before the
  change, which had no lock; the held-press test asserts it after the
  release, so a lock never freed turns it red.
- **G5.** Item 3 held before the change; its evidence is `task e2e` on
  `162a389`.

## What the next full mutation run should find

A survivor of a kind already adjudicated, not answered ahead of a run that
names it (the plan's P5): the `[]` of the removal button's `clears` filled,
as for the take chip's `clears: [],`. The key emptied is killed by the lock
walk, which presses a second actor's button while the first is held. The
equivalents file is not edited (the plan's D7): its console lock-key block's
header is in the past tense, the change adds no literal key, and the new
template key opens with a non-empty prefix. The client's stored mutation
report measured `d1ff728` and is out of date (SPEC-023).

## What could not be established

Nothing was left open.

## What was deliberately left out

- A row that an accepted command clears no field of another form, for the
  fields of the forms in `FORMS`: the acceptance walk holds it and no row
  does (the second deviation); VTT-522 and VTT-534 hold it for `say-as` and
  `say-text`. It goes to the owner as a question with this report.
- The agent's removal over MCP; taking an ability and revoking control, which
  do not ask either.
- The client's full mutation run, the owner's to ask for.
