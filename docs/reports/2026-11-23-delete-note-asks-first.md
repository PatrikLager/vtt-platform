# The console's Delete note asks the DM first — report

Ticket: `docs/superpowers/specs/2026-11-23-delete-note-asks-first-design.md`.
Plan: `docs/superpowers/plans/2026-11-23-delete-note-asks-first.md`.
Branch `fix/delete-note-asks-first` from main `d1ff728`. Commits: `e972949`
(Task 0: ticket, plan, register) and `8170606` (Task 1: the question, the
lock, its tests, the lock-key reasons, the register's cells and the rebuilt
bundle; the last code commit). Every code
reference below is to `8170606`. SPEC-022 moves next, written from this
report.

## Done looks like

1. [x] **Delete note asks first.** Delete note's press in
   `client/src/view/dm.ts` sends through `sending`, keyed `delete-note`. After
   the empty check it asks `Delete the note "<note>"? Its title and text go
   with it.` through `DMDeps.confirm`, the note shown by
   `d.st.Notes[key]?.Title || key` for the trimmed key; a no returns nothing,
   a yes returns `delete_note` for the trimmed key with `clears: []`, so
   `note-key` keeps what was typed.
   - `client/test/dm-view.test.ts`'s "Delete note asks first, naming the note
     by its title, else by its key, and a no sends nothing" (VTT-956), "a
     Delete note the DM declines keeps the key field as typed, at the next
     paint too" (VTT-958) and "deleting a note trims its key", which now
     asserts one question (VTT-957).
   - Independently: `client/test/qa-delete-note-asks-first.test.ts`.
   - On `e972949` each failed: the first two asked nothing and sent on a no,
     the third asked nothing.
2. [x] **A blank key still asks nothing.** "each guard refuses with its own
   exact wording" and "a whitespace-only id is refused everywhere one is
   required" in `dm-view.test.ts` assert that no question is asked (VTT-959),
   the second newly, with the notice, for its Delete note arm; the QA file's
   blank-key tests likewise.
3. [x] **An unanswered deletion holds Delete note back.**
   - "a Delete note pressed again while its command is unanswered asks
     nothing and sends nothing, and sends once answered" (VTT-960), and the
     walk "a control whose command is unanswered sends nothing when pressed
     again, each row its own", which `FORMS` now takes Delete note through.
   - "a Delete note whose send rejects lets it go, clears nothing, asks no
     repaint, and reports the error" (VTT-962).
   - On `e972949` each failed: a held press sent again, and the rejected
     send reached no `console.error` and surfaced as an unhandled error, which
     also failed a neighbouring test, "the guide button shows the guide it
     fetched".
   - The drawing walk (VTT-961) and the refusal walk (VTT-963) take Delete
     note in too; both were green on `e972949`.
4. [x] **The gate.** `task check` and `task e2e` are green on `8170606`.
   `cmd/vtt/webdist` is rebuilt in that commit, and `check:drift`, a step of
   `task check`, finds no difference. `check:comments` is clean, and no
   ledger row moved. The Go mutation gate measures no client code, and
   `check:ts-mutation` judged no client report: the stored one measured
   `d1ff728` and is out of date (SPEC-023).

## What the rule became

After the sort at Task 0, the ticket's first rule became VTT-956, VTT-957
and VTT-958, its second VTT-960 and its third VTT-962; the sort added VTT-959
(item 2) and VTT-961 and VTT-963 (the plan's D1):

- **VTT-956.** Delete note, pressed with a key that is not blank while no
  `delete_note` it sent is unanswered, asks first, in a message holding the
  title the console's state holds for the note of the trimmed key, or that
  key when it holds no title for it, and sends nothing on a no.
- **VTT-957.** A Delete note the DM accepts sends one `delete_note` naming
  the trimmed key.
- **VTT-958.** A Delete note the DM declines leaves `note-key` as typed, at
  the next paint too.
- **VTT-959.** Delete note, pressed with `note-key` empty or white space
  while no `delete_note` it sent is unanswered, shows `name the note to
  delete`, asks nothing and sends nothing.
- **VTT-960.** While a `delete_note` it sent is unanswered, pressing Delete
  note again sends nothing.
- **VTT-961.** A press of Delete note that sends changes nothing the console
  draws until that command is answered.
- **VTT-962.** A `delete_note` it sent whose promise rejects clears nothing,
  asks for no repaint, frees Delete note, and is reported with
  console.error.
- **VTT-963.** When the server refuses a `delete_note` it sent, the console
  asks for no repaint of its own.

The lock clause, whose absence withdrew VTT-946 and VTT-949, was drafted into
VTT-956 and VTT-959 from the start. VTT-523, VTT-524, VTT-526 and VTT-538,
which list the controls `sending` holds, stand as worded (the plan's D9): they
never listed every such control (a row's Give and an ability's take chip send
through `sending` and are in none of the four, the plan's N2), and succeeding
them would have moved every citation of them. Refused by the sort: the
question's words as a row (the tests pin them); that a held press asks nothing
(a consequence of where the question sits); the agent's MCP `delete_note`
asking nothing (a non-goal); that an accepted Delete note keeps `note-key`
(VTT-533 states it); the lookup by exact key (how, not what); that Delete note
holds back no other control (VTT-541 states it); and a title of white space
alone or holding quotes shown as written (an almost theoretical edge case).

## Rule 9

MapTool has no campaign notes: a search of the keys of `~/dev/RPTool/maptool`'s
`i18n.properties` for `note` finds notes only on a token's edit dialog and in
campaign-export warnings (the plan's check 1). Its named, user-written objects
that ask before they are deleted, naming the object, include a lookup table
(`Delete table "{0}"?`, `LookupTablePanel.confirm.delete`) and a macro (`You
are about to delete the {0} macro.`, `confirm.macro.delete`), behind no
preference: `LookupTablePanel` and `MacroButtonPopupMenu` call
`MapTool.confirm` directly, and `AppPreferences` holds a delete warning only
for tokens and drawings (`tokensWarnWhenDeleted`, `drawingsWarnWhenDeleted`).
Borrowed: asking before a named piece of writing is deleted, naming it by
what the user sees it called, every time. Not borrowed: the macro's "Do you
wish to continue?", which the console's other questions do without.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as verified | Its problem statement corrected before Task 0's commit | The verifier found it said a removed condition is put back with a press or two; the console has no control that applies a condition, and no command applies one but `use_ability` |
| D7: no header or other word of `tools/ts-mutation-equivalents.txt` changes but the count clause | The header of the console's lock-key block also put in the past tense: "each call whose key was a string literal on 526885b", and "the two calls whose key was a template literal there" | Task 1's reading review: with `"delete-note"` a further literal key and no entry for it, the present tense said every literal key's mutant was adjudicated; the template-key sentence was already false in the present tense at the base. No key changed |

## History cut from the code (rule 10)

None: Delete note carried no comment before.

## The deliberate breaks

Each applied, seen red, and restored from a saved copy compared byte for byte:
the question dropped (the VTT-956, VTT-957 and VTT-958 tests red); asked
before the empty check (both guard tests); asked outside the press `sending`
runs (the held-press test); the key always shown, `??` for `||`, the untrimmed
field looked up, and the lookup's `?.` dropped (the VTT-956 test); the draft
cleared on a no (the VTT-958 test); the untrimmed key sent (the VTT-957 test);
sent outside `sending` (the held-press and rejection tests and the lock walk);
`note-key` cleared on an accepted answer (the walk "an accepted command clears
the fields of its own form and of no other"); Save's lock key used (the lock
and drawing walks); a repaint asked for at the press (the drawing and refusal
walks and the rejection test); sent outside `sending` with a repaint at every
answer and a rejection reported (the refusal and lock walks and the held-press
test); the untrimmed field checked for empty (the whitespace test); the draft
cleared at the press (the drawing walk, and the QA file's "a Delete note that
sends changes nothing drawn until it is answered", once it compared a fresh
paint).

## Phase 4a adjudications

- No defects. Its tests in `client/test/qa-delete-note-asks-first.test.ts`
  land: titles holding quotes and punctuation, an empty title, a key no note
  holds, a padded key, a note stored under a padded key, keys named like
  `Object` members, a decline followed by a yes, a blank press while held, a
  press at a later paint while held, Save and Delete note each held while the
  other is pressed, a note removed or retitled after the paint, a refused and
  a rejected send after a yes, what is drawn until the answer, and the repaint
  after an accepted one.
- QA raised one reading, adjudicated here. It read "the title the console's
  state holds" as open between the state at paint and at press. The console
  reads its `st` at the press, and the QA file's "a note removed from the
  state after the paint is asked by its key" and "a note retitled in the state
  after the paint is asked by its new title" pin that reading. No row changed.

## Task 1's reading review, on the QA file

- It asked for the QA file's comments that cited the QA dispatch brief,
  which is in no record, to be changed. The test of the repaint after an
  accepted Delete note now points to the plan's D1, which names that repaint
  a consequence of `sending`; the test that an unanswered Delete note
  does not hold Save back cites VTT-541, as does the test of the reverse, and
  VTT-541's row cites both. The review also asked the drawing test to compare
  a fresh paint, so that a press clearing a draft before its answer turns it
  red; it does, and was seen red.

## The plan's gaps

- **G1.** The whitespace test's Delete note arm did not check the notice or
  that nothing is asked; it does now.
- **G2 and G4.** The ticket's open points: the naming is the title, else the
  key (OD-1 A, as the Notes panel heads a note); the words are OD-2 A; how the
  rows take Delete note in is D9.
- **G3.** `tools/ts-mutation-equivalents.txt`, the QA file and this report,
  beside the ticket's list. `tools/comment-ceilings.txt` binds and no row
  moved: with the one warning line, `check:comments` is clean, and D6's
  ledger step did not run.
- **G5.** "Once that command is answered it sends again" held before the
  change, which had no lock; the held-press test asserts it after the release,
  so a lock never freed would turn it red.

## What the next full mutation run should find

Survivors of kinds already adjudicated, not answered ahead of a run that
names them (the plan's P5): the key `"delete-note"` emptied, as for the entries
on the console's other literal lock keys, and the `[]` of Delete note's
`clears` filled, as for the take chip's `clears: [],`. The client's stored
mutation report measured `d1ff728` and is out of date from `8170606`
(SPEC-023).

## What could not be established

Nothing was left open.

## What was deliberately left out

- An e2e test: no spec presses Delete note (a `git grep` of `client/e2e` for
  `delete-note`, `deleteNote` and `"Delete"`; `handover.spec.ts` types into
  `note-key` and presses Save), and the `window.confirm` wiring is the one
  Remove actor, Remove token, Boot, Make spectator and New link share, Boot,
  Make spectator and New link driven through a real dialog.
- The agent's deletion over MCP; Remove condition, taking an ability and
  revoking control, which do not ask either. Remove condition goes to the
  owner as a question with this report: a removed condition cannot be put
  back from the console.
- Telling apart notes of one title, which read alike in the question; the
  key typed settles which is deleted.
- The client's full mutation run, the owner's to ask for.
