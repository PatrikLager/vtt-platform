# Verification debt, and recipes for defects that escaped

Two things live here that were previously written in code comments, where
nobody acts on them.

**Verification debt** is a coverage gap someone already knows about. Written as
a comment it explains one test; written here it is a claim on future work.

**A recipe** is how to put an escaped defect back. Recipes are cheap at the
moment of escape — you have just fixed it, so you have the exact edit — and
expensive afterwards, because the tree moves. Write one when a defect escapes;
do not backfill old ones.

Every entry names the gate that should have caught it and why it did not. The
labels are deliberately few:

| Label | Meaning |
|---|---|
| `spec silent` | no requirement determines the behaviour, so no test can pin it |
| `test data missing` | the requirement is clear; no fixture reaches the state |
| `test asserts nothing` | a fixture reaches it; the assertion cannot fail |
| `outside the tool` | no mutation operator expresses the defect |
| `dispatch too narrow` | the checker was given a filtered view of the spec |
| `requirement unnamed` | the rule exists but nothing could cite it durably |

Without that label an entry cannot be used in hindsight — it says a defect
escaped but not what to change so the next one does not.

---

*The three entries dated 2026-09-16 and 2026-09-17 describe `internal/perceive`,
which was rolled back uncommitted on 2026-09-20 to be redone under the process;
the package and the tests they name are not in the tree. They stay as the record
of their period.*

## 2026-09-16 — a refused `Look` also dropped the roster

**What escaped.** `Look` refuses sight to anything but a party member. Expressed
as an early return, that refusal also skipped the roster loop below it, which is
a different rule. Survived two review rounds; caught by a reviewer reading the
code, not by any test.

**Labels:** `test data missing`, `outside the tool`.

**Recipe.** In `perceive.Look`, change the refusal body from `eyes = nil` to
`return v`.

**Which gate should have caught it.** None could, and the first version of this
paragraph got the measurement wrong in the direction that mattered — corrected
2026-09-17 after a reviewer re-ran it. What is actually true:

- `if true ||` reds the oracle at FOUR seats, all on `ActorControlGranted`:
  `door-watch/act-latecomer`, `door-watch/act-watcher`, `shared-control/act-scout`,
  `shared-control/act-warden`.
- `if false &&` — the roster off for everyone — reds **nothing** except
  `TestARefusedLookStillFillsTheRoster`. Not one of the eleven seats notices.

So the two are COMPLEMENTARY, each holding one direction, and neither is
redundant:

- **roster too permissive** (`if true ||`, every actor named) — held by the
  ORACLE's four seats. `TestARefusedLookStillFillsTheRoster` passes under it.
- **roster empty** (`if false &&`) — held ONLY by that test. All eleven seats
  pass under it.

The corpus is blind to the WITHHOLDING direction by construction: `classify_test.go`
builds `Shown` by folding the seat's own recorded stream, which already carries
the synthesized introductions, so a missing `Lit.actors` entry is masked by
`Shown.Actors`. It is not blind to the permissive one — an actor the roster
should not name has no `Shown` entry to hide behind, which is what those four
`ActorControlGranted` seats catch. Deleting them to "save" duplication would
remove the only thing that reds when the roster over-shares, which is finding 14
and the reason the roster rule (the actor roster is projected too) exists.

The defect itself is additionally a control-flow restructure, which no operator
mutation expresses — so the mutation gate was never going to reach it either.

**What the wrong version would have cost.** It told the next reader that the
corpus defends the roster loop. Delete the test on that basis and a roster that
goes empty for everyone ships green: every character's log stops naming their
own party, and under the rule that a character log is the record of what that
character experienced, that is permanent for the characters it touched.

**Closed by** `TestARefusedLookStillFillsTheRoster`, pinning the rule that a refusal
of sight is not a refusal of the roster, which reds on the recipe above. The probe is `ActorControlGranted` with `Shown` empty
of the granted actor: a `knows()` arm carrying no position, so its verdict is a
direct read of the roster, which is otherwise unobservable.

**Fixture check.** Putting `a-hero` into `Shown` makes the same test pass with
the defect present — the empty `Shown` is load-bearing, not decoration.

---

## 2026-09-16 — the QA dispatch hid the requirement it was testing

**What escaped.** The first run of the independent QA stage reported that the
spec does not determine which component keeps the roster promise. It does:
visibility spec §5, *"The actor roster is projected too."* That section was not
in the four-section list the dispatch handed over, because the list was built by
reading headings and §5 is titled "Contract additions".

**Label:** `dispatch too narrow`.

**Recipe.** Dispatch the QA stage with a section list rather than the whole
spec, and choose the sections by heading.

**Closed by** the change to `qa-prompt.md` that forbids section lists: a
filtered view inherits the dispatcher's blind spot, which is the one thing the
stage exists not to inherit.

**Closed 2026-09-17, labelled `requirement unnamed`.** One rule from the same
finding lived in a code comment with no home in any spec: introduction has
exactly one code path. It is now a rule in the visibility ticket, citing the
per-character-logs rule that a character log is the record of what that
character experienced. Pinned by
`TestQAActorAddedIsWithheldEvenForAPartyMember`. What made it debt rather than
a tidy-up: the comment was inherited from `internal/gateway/project.go`, which
Task 12 of `docs/superpowers/plans/2026-09-14-per-character-logs.md` deletes, and it survived the port only because the port was verbatim.

---

## 2026-09-17 — the mutation gate reaches neither line the roster defect lived on

**What it is.** `internal/perceive` was gated the day it was written, and its
first run reports `Killed: 4, Lived: 0, Test efficacy 100.00%` over a file of
roughly five hundred lines — the exact count moved three times the same day, so
it pins nothing.
Nine `if` statements; five generate no mutant, and two of those five are the
sight refusal (`!ok || !engine.IsPartyMember(a)`) and the roster loop's
`if engine.IsPartyMember(a)`. A mutant appears where an ENABLED mutator
family's operator token appears — gremlins defaults five families to true, only
two of them comparisons, so "comparisons only" is not the rule. The roster line
holds no operator any mutator rewrites; the refusal holds a `||`, rewritten
only by `INVERT_LOGICAL`, which is off by default. The if-with-init is not the
cause — adding one comparison to that same statement produces mutants at it.

**Label:** `outside the tool`.

**Why it is recorded rather than fixed — with one correction.** A first version
of this entry said "there is nothing to fix in the gate; the mutator set is what
it is." That is false: the set is a FLAG. gremlins ships `INVERT_LOGICAL`
(`&&`/`||`), off by default, and with it enabled a mutant lands on the `||` of
the sight refusal and the suite KILLS it. What remains true is that the gate as
CONFIGURED does not reach those two lines, and that nobody should read `100.00%`
on this package as coverage of its security decision. Enabling the flag is an
open decision costing one new survivor at the `knows()` chain's first `&&`.

**What actually holds those two lines:** `TestLookRefusesEyesToAnythingButA-
PartyMember` and `TestARefusedLookStillFillsTheRoster`, both proved red by HAND
injection. That is not belt-and-braces; it is the only measurement those lines
have. Deleting either removes the whole defence and no gate will say so.

Published with numbers in `tools/mutation-scope.md`, per that file's job.

---

## 2026-09-23 — the shut-door refusal has no test that can see its write

**What it is.** `JoinAdmits` in `internal/identity` reads the door once and
refuses by one guard of four terms: the door not open, an empty stored secret,
a secret that does not match, a spent budget. The rule ADR-011 rests on is that
a refusal writes nothing, because identity shares its SQLite file with the
event log and a write on a refusal path takes the write lock on the file the
table is appending events to. The mismatch and spent-budget terms are held by
`TestAWrongSecretRefusesWithoutTouchingTheDatabase` and
`TestASpentBudgetRefusesWithoutTouchingTheDatabase`, which arm a database fault
so the `UPDATE` itself fails if it is reached; the empty-secret term by
`TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath`. The door term has no
such test.

**Labels:** `test asserts nothing`, `outside the tool`.

**Recipe.** In `JoinAdmits`, remove the shut-door term from the guard that
refuses before the `UPDATE`. Measured 2026-09-23 on the tree with that edit
applied: the identity and gateway suites stay green.

**Why the two tests that aim at it cannot see it.** `TestAClosedDoorSpendsNothing`
reads the admission counter afterwards. That proves the write had no EFFECT,
because the `UPDATE`'s own condition refuses the increment anyway, not that it
was never ATTEMPTED, which is the property. `TestARefusedJoinWritesNothingAtAll`
counts rows in the door's table before and after, with no row present at all,
so `JoinAdmits` returns at `sql.ErrNoRows` before the guard the recipe edits;
its path never reaches the line.

**Why the mutation gate cannot reach it.** It rewrites operators, and the defect
is a whole term disappearing from a chain of refusals.

**What closes it.** The instrument the other two refusals already use: arm the
database fault so the statement fails if it is reached, with a shut door in
place of a wrong secret. Not closed here; the joining-a-table arc's ticket is
where it is closed or recorded as open.

---

## Open debt

**The oracle corpus contains no non-party viewpoint.** `MayPerch` produced every
seat in it, so any rule that only diverges for a non-party or not-yet-existing
viewpoint is invisible to all eleven. `TestARefusedLookStillFillsTheRoster`
covers the one such rule known today; the corpus limit itself is unchanged, and
the next rule of that shape will need its own hand-built fixture.
