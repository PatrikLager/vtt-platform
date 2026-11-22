# The console's Remove asks the DM first — report

Ticket: `docs/superpowers/specs/2026-11-22-remove-asks-first-design.md`.
Plan: `docs/superpowers/plans/2026-11-22-remove-asks-first.md`.
Branch `fix/remove-asks-first` from main `009db12`. Commits: `4b09970` (Task
0: ticket, plan, register) and `9e59968` (Task 1, the question, its tests and
the rebuilt bundle; the last code commit). Every code reference below is to
`9e59968`. SPEC-022 moves next, written from this report.

## Done looks like

1. [x] **Remove asks first.** Remove's press in `client/src/view/dm.ts`,
   inside the press `sending` runs and after the empty check, asks
   `Remove <actor>? Nothing brings it back, and its tokens leave the board.`
   through `DMDeps.confirm`, the actor shown by `d.st.Actors[id]?.name || id`
   for the trimmed id; a no returns nothing, a yes returns `remove_actor` as
   before.
   - `client/test/dm-view.test.ts`'s "Remove asks first, naming the actor by
     its name, else by its id, and a no sends nothing" (VTT-950), "a Remove
     the DM declines keeps the actor field as typed, at the next paint too"
     (VTT-948) and "removing an actor sends its trimmed id and clears the
     field", which now asserts one question (VTT-947).
   - Independently: `client/test/qa-remove-asks-first.test.ts`.
   - On `4b09970` the VTT-950 and VTT-948 tests failed (nothing asked, a
     send on a no), and the existing Remove test failed on its new question
     count.
2. [x] **An empty field still asks nothing.** "each guard refuses with its own
   exact wording" and "a whitespace-only id is refused everywhere one is
   required" in `dm-view.test.ts` now assert that no question is asked
   (VTT-951); the QA file's blank-field tests likewise.
3. [x] **No false reason is left.** The comment above Remove in `dm.ts` is a
   warning and a citation line; the paragraph above the Remove test in
   `dm-view.test.ts` is gone. A search of `client/`, `docs/specifications/`,
   `README.md` and `docs/blueprint.md` (`git grep -i`) finds none of "exactly
   one control asks first", "every seat at this table can see" and "NO
   CONFIRMATION".
4. [x] **The gate.** `task check` and `task e2e` are green on `9e59968`.
   `cmd/vtt/webdist` is rebuilt in that commit; `check:drift`, a step of
   `task check`, rebuilds it and finds no difference. The Go mutation gate
   measures no client code, and `check:ts-mutation` judged no client report:
   the stored one measured `b90fc1a` and is out of date (SPEC-023).
   `check:comments` is clean, `dm.ts` within its ceiling's band, so no ledger
   row moved.

## What the rule became

The ticket's one rule, "the console's Remove asks the DM first, in a message
holding the actor's name, and sends nothing when the DM declines", became,
after the sort at Task 0 and the withdrawals in Task 1:

- **VTT-950.** Remove, pressed with an actor id that is not blank while no
  `remove_actor` it sent is unanswered, asks first, in a message holding the
  name the console's state holds for that actor, or the id when it holds
  none, and sends nothing on a no.
- **VTT-947.** A Remove the DM accepts sends one `remove_actor` naming the
  trimmed id.
- **VTT-948.** A Remove the DM declines leaves the actor field as typed, at
  the next paint too.
- **VTT-951.** Remove, pressed with the field empty or white space while no
  `remove_actor` it sent is unanswered, shows `name the actor to remove`,
  asks nothing and sends nothing.

**VTT-946 and VTT-949 are withdrawn.** Allocated at Task 0 without the
clause "while no `remove_actor` it sent is unanswered", both were false for a
press the console's lock stops: `sending` returns before the press runs, so
such a press asks nothing and shows nothing. The implementer found it for
VTT-946; Task 1's reading review found it for VTT-949, with a probe that
held a send and pressed Remove again with a blank field. They are succeeded
by VTT-950 and VTT-951. The plan, landed in `4b09970`, still lists R1 as
VTT-946 and R5 as VTT-949.

Refused by the sort: the question's exact words as a row (VTT-569 and
VTT-901 pin a name; VTT-902 does pin Make spectator's words, but the sort
kept Remove's words in its tests, VTT-950 holding the name); a row
that a press the lock stops asks nothing (a consequence of where the
question sits; `dm-view.test.ts`'s "a Remove pressed again while its command
is unanswered asks nothing and sends nothing" holds it, citing VTT-523 for
"sends nothing"); the agent's MCP `remove_actor` and `remove-token` asking
nothing (non-goals); the lookup by exact id (how, not what); the opening
comment's rule (a comment).

## Rule 9

MapTool asks before deleting tokens, by default: `msg.confirm.deleteToken`,
behind the preference `tokensWarnWhenDeleted`, default on, which the dialog
lets the user switch off. Borrowed: asking before an irreversible removal.
Not borrowed: the switch, which the owner's "always" rules out.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Task 0's rows R1 and R5 as VTT-946 and VTT-949 | Both withdrawn in Task 1, succeeded by VTT-950 and VTT-951 | False as worded for a press the console's lock stops (above) |
| Task 1: one test for R1, R2 and R4 | Separate tests for VTT-950 and VTT-948 | Each row cites the test that holds it alone |
| The ticket as verified | Its problem statement corrected before Task 0's commit | The verifier found slips in it: SPEC-017 says an actor added again under the same id is controlled by nobody, and a connected player is shown `You no longer play <name>` (VTT-605) |

## History cut from the code (rule 10)

Above Remove in `dm.ts`, removed: "Takes an actor out of the world for good,
and every token it has on the board with it (retraction-leaves Task 9, spec
§5.2) — the DM's reach for the one command in this arc that emits a batch.
Shares actorId with Add rather than a field of its own: naming an actor to
remove needs nothing else, which is exactly how Remove sits beside Place one
group down." and "NO CONFIRMATION, per this file's own opening rule: exactly
one control asks first, and it is the one whose damage lands OUTSIDE the
room. A removal lands on the log every seat at this table can see." The
second was false: New link, Boot and Make spectator each ask, and only the DM
and the agent are sent a removal (SPEC-017).

## The deliberate breaks

Each applied, seen red, and restored from a saved copy compared byte for
byte: the question dropped (the VTT-950, VTT-948 and existing Remove tests
red); asked before the empty check (both guard tests red); asked outside the
press `sending` runs (the held-press test red); always the id (`Remove a1?`);
`??` for `||` (`Remove ?`); the untrimmed id (`Remove   a1 ?`); the field
cleared on a no (the VTT-948 test red).

## Phase 4a adjudications

No findings. Its tests in `client/test/qa-remove-asks-first.test.ts` land.
The held-press test's assertion that a second press asks nothing carried
VTT-950, which does not state it: that assertion's citation is dropped (the
test stays in VTT-950's row through its press after the answer), and its
"sends nothing" assertion cites VTT-523. QA could not show its tests fail on
the old code without editing it; with the question dropped from `dm.ts` (and
restored), the tests that need a question went red and the blank-field tests
stayed green. Task 1's reading review asked for robustness fixes in the file (the button
found by `data-action`, the notice read from `notify` alone, a `finally` in
the empty-field test), made before the commit.

## The plan's gaps

- **G1.** The guard tests did not check that nothing is asked; they now do.
- **G2.** A nameless actor is shown by its id, as the console shows every
  actor name (`grep -n '\.name ||' client/src/view/dm.ts`).
- **G3.** `docs/requirements.md`, `tools/comment-ceilings.txt` and the
  adjudication keyed on Remove's line in `tools/ts-mutation-equivalents.txt`:
  the line is byte-identical, so the adjudication holds, and no ceiling
  moved.
- **The ticket's open points.** SPEC-017 states no console control and does
  not move; the words are the example the owner chose, naming the actor alone
  (OD-1 A).

## What could not be established

Nothing was left open.

## What was deliberately left out

- An e2e test: no spec presses Remove, and the `window.confirm` wiring is the
  one Boot, Make spectator and New link share, each already driven through a
  real dialog (D6).
- The agent's removal over MCP and "Remove" for a token: the ticket's
  non-goals. A token's Remove still asks nothing, where MapTool asks before
  deleting a token (the plan's P4); raised with the owner.
- The client's full mutation run: the stored report measured `b90fc1a` and
  is out of date; a run is the owner's to ask for.
