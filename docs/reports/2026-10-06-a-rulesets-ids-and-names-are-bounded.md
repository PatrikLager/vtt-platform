# A ruleset's ids and names are bounded: the change

**Ticket:** `docs/superpowers/specs/2026-10-06-a-rulesets-ids-and-names-are-bounded-design.md`,
revised by its writer after verification and after sign-off (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-06-a-rulesets-ids-and-names-are-bounded.md`,
verified by `verify-ticket` (passes with gaps).
**The owner's rulings:** on 2026-10-06, a split of the id-like strings the id
report raised into two tickets by source, this one first; at sign-off the same
day, a bound of 128 bytes through the fold's own `maxIDBytes` with a pinned
copy in `internal/rules` (Q1(a)), a resolution's branch labels in scope
(Q2(b)), and the rest as the plan proposed: no fold bound on
`ConditionApplied.source` (Q3(a)), an empty condition or ability id refused
(Q4(b)), no bound of its own on `ResourceChanged.resource` (Q5), the threshold
refusal naming its position (Q6(b)), the schemas stating the bound (Q7), rows
A to F, one code commit and the report (Q8). On the review: every finding
fixed. On the report's review: every finding fixed, and D2's texts raised here
rather than as a ticket.
**Last code commit:** `53acb19`, on `fcd367c`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline fcd367c..53acb19

    53acb19 A ruleset's ids and names are bounded

`git diff --stat fcd367c..53acb19`: 30 files changed, 3292 insertions(+), 82
deletions(-).

The gate, `task check` whole, after `53acb19`, once: it exited 0 with no step
failed. Its check steps' own verdict lines read `check:comments` clean over
273 files, `check:requirements-chain` 288 rows, 233 test files and 13
specifications, `check:doc-owner` 80 files, `check:new-prose` 2077 added lines
clean, `check:coverage` 20 packages at or above their floors
(`internal/engine` 99.0 %, `internal/rules` 91.0 %, `internal/gateway` 94.8 %
and `internal/campaign` 89.3 %), `check:no-pack`, `check:no-retraction` and
`check:no-create-scene` clean, `task lint` 0 issues, `check:breaking`
reporting pre-release with no objection, `check:mutation` 14 packages with
zero unadjudicated survivors (nine mutated afresh, `internal/identity`,
`internal/artlib`, `internal/campaigncfg`, `internal/mapdef` and
`internal/store` reusing their verdicts; six mutants timed out in
`internal/sight`, `internal/rules` and `internal/mcp` and counted as killed),
and `check:ts-mutation`, re-mutating since the client's sources and tests
changed, 2928 mutants, 2832 killed, 67 survivors all adjudicated equivalent,
zero unadjudicated, 29 timed out and counted as killed, over 55 minutes.
`check:mutation` began with 38.4 GiB free; `task check:drift` exited 0 after
the commit.

## Done looks like, answered

1. `[x]` `rules.Load` refuses a ruleset that declares an ability id, a
   condition id, an attribute, defense or resource name, or a resolution's
   branch label longer than 128 bytes, naming the file and the field, and
   loads one whose every such string is exactly 128:
   `internal/rules/load_test.go#TestARulesetWhoseIDOrNameExceedsTheBoundIsRefused`,
   one subtest per kind, red on `fcd367c`'s source (`Load = <nil>`), with
   `internal/rules/id_bound_internal_test.go` set aside because it does not
   build there; and `#TestARulesetWhoseIDsAndNamesAreExactlyTheBoundLoads`,
   whose at-bound half K7 to K12 hold. QA's
   `internal/rules/qa_ruleset_bound_test.go`.
2. `[x]` `engine.Apply` refuses a `ConditionApplied` whose condition id, and
   an `AbilityUsed` whose ability id, is empty or longer than 128 bytes, and
   accepts one of exactly 128:
   `internal/engine/apply_test.go#TestAConditionWhoseIDExceedsTheBoundIsRefused`,
   `#TestAnAbilityWhoseIDExceedsTheBoundIsRefused`,
   `#TestAnEmptyConditionOrAbilityIDIsRefused` and
   `#TestAConditionIDIsMeasuredBeforeItsActorIsLookedUp`, red on `fcd367c`
   (`err = <nil>`, or the unknown actor's text before the id's); and
   `internal/engine/apply_boundary_test.go#TestConditionAndAbilityIDsAtCapAreAccepted`,
   held by K18, K19 and K24. QA's `internal/engine/qa_ruleset_bound_test.go`.
3. `[x]` `client/src/fold.ts` refuses the same events at the same byte counts:
   `client/test/fold-rejections.test.ts#a condition id over 128 UTF-8 bytes is
   rejected`, `#an ability id longer than 128 bytes is rejected`, `#an empty
   condition id is rejected`, `#an empty ability id is rejected` and `#a
   condition id is measured before its actor is looked up`, each red on
   `fcd367c`; and `#condition and ability ids of exactly 128 bytes are
   ACCEPTED`, held by K28 and K29. QA's
   `client/test/qa-ruleset-bound.test.ts`.
4. `[x]` The debt entry's recipe is refused at `rules.Load`, naming the file
   and the field, and a `use_ability` refusal read off a real connection
   carries no ruleset text over the bound:
   `internal/rules/load_test.go#TestARulesetNameOfSeventyThousandBytesIsRefusedAtLoad`,
   red on `fcd367c`'s source;
   `internal/rules/resolve_test.go#TestAThresholdRefusalNamesItsPositionNotItsExpression`,
   red there with a 70,105-byte refusal; and
   `internal/gateway/ruleset_test.go#TestAThresholdRefusalReachesTheIssuerWithoutItsExpression`,
   red there with `websocket: message too big: read limited at 32769 bytes`.
   The three close the entry, whose `**Closed by**` line names them.
5. `[x]` Every shipped ruleset and every ruleset fixture meant to load still
   loads, every golden still folds, and `task check` whole is green:
   `internal/rules/conformance/conformance_test.go#TestConformanceOverRulesetsGlob`
   loads both shipped rulesets,
   `internal/adventure/conformance/conformance_test.go#TestConformanceOverAdventuresGlob`
   every adventure on them,
   `internal/harness/fold_golden_test.go#TestFoldGoldenCorpus` folds every
   golden stream through the Go fold, and
   `client/test/fold-parity.test.ts#fold parity: <name>` folds them through
   `fold.ts`, three of them holding `abilityUsed` or `conditionApplied`. Every
   ruleset fixture meant to load is loaded by the tests that use it. The gate
   paragraph above records the gate, in which all of them ran.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A ruleset's ability ids, condition ids, attribute, defense and resource names and its resolutions' branch labels are at most a fixed number of UTF-8 bytes; a ruleset carrying a longer one is refused at load. | VTT-283, and VTT-284 for the branch labels |
| Both folds refuse a `ConditionApplied` whose condition id, or an `AbilityUsed` whose ability id, is longer than that bound, or empty. | VTT-285, and VTT-286 for the empty ids |
| A refused `use_ability` carries no ruleset text longer than that bound. | VTT-288, that the threshold refusal names its position; the other refusals quote only bounded names, held by D8's reading and no test; VTT-132 gains the over-the-wire test |

VTT-287, that the schemas state the bound, came from Q7.

## The sort

Six rows accepted as D12 proposed (A to F, now VTT-283 to VTT-288), and ten
candidates refused, as D12 lists them; none was reversed.

## Phase 4a: QA adjudications

QA on opus, given VTT-283 to VTT-288, VTT-132, VTT-161 and VTT-162, SPEC-018
whole, SPEC-016's paragraphs on `source`, `reason` and introductions, both
debt entries, the text of `AbilityUsed`, `ConditionApplied`,
`ConditionRemoved`, `ResourceChanged`, `UseAbility` and `RemoveCondition`, the
four schemas, `go doc -all` of `rules`, `engine`, `gateway` and `campaign`,
`fold.ts`'s and `state.ts`'s export lines, and
`internal/rules/testdata/valid/` and `rulesets/tavern-brawl/` as format
examples. Its brief named `go-arch-lint check` and the imports each file may
use. It wrote `internal/rules/qa_ruleset_bound_test.go`,
`internal/engine/qa_ruleset_bound_test.go`,
`internal/gateway/qa_ruleset_bound_test.go`,
`internal/campaign/qa_ruleset_bound_test.go` and
`client/test/qa-ruleset-bound.test.ts`: 17 Go tests holding 114 subtests, and
34 TS tests, none failing (QA's own report counted 27 Go tests); thirty
injections into its own files, all red; every file outside its five hashed the
same before and after.

- The threshold's position is unstated as 0- or 1-based: QA accepted either
  and required the text to change with the position; the implementer's test
  pins the observed 0. No change; it goes to the last section.
- "Naming the file" is unstated as base name or path: QA required the base
  name; the refusal carries the path opened, as the other loaders' do. No
  change.
- VTT-284 has no at-bound clause: the at-bound load holds a 128-byte label,
  and its test cites VTT-284. No change.
- A branch label an outcome names is not bounded on its own: it must match a
  label its resolution declares. No change.
- A `use_ability` refusal can echo the command's own ability id at frame
  length: SPEC-018 bounds a command's text by the frame, and D8 says so. No
  change.
- The bounds' copies: the at-bound Resolve-then-fold test reds when the
  engine's bound falls below the loader's, not when it rises; the debt entry
  stays open. No change.
- To drive `Resolve` past a loader that refuses over-long ids, three of QA's
  gateway tests change a loaded ruleset's exported `Compiled` and
  `Conditions`, as the id arc's QA changed a held `mapdef.Map`. No change.
- Requirements QA asked an id for: the refusal's form, the two arms' order,
  `campaign.Append`'s "with that text", `campaign.Open` refusing a log and
  `checkLen`'s TS wording. The form and the condition arm's order are D12's
  sixth and fifth refusals; the ability arm's "for its ability id alone" and
  the rest are SPEC-018's own sentences, as in the id arc. None dispensed.
- Three of QA's gateway tests cited VTT-161 for a `Resolve` or no-ruleset
  refusal; VTT-161 was struck from them on the review's S1, leaving VTT-132
  beside their other citations (Phase 4b).

## Phase 4b: the review

One reviewer on opus, given `git diff HEAD`, the untracked files, the plan,
the draft commit message and the break script. Nothing that breaks at the
table. Its findings, each fixed by the owner's ruling: `check:comments` red on
three test files fallen more than a point under their ceilings, against the
plan's "No ledger row changes"; the over-the-wire threshold test citing
VTT-161, whose refusals are the role table's, a player rule's or the fold's,
and not observing that the connection takes a next command, now VTT-132's with
a next command sent, and three of QA's gateway tests with the same citation,
VTT-161 struck from each; the commit message's test-first paragraph claiming
reds on a tree where `internal/rules`' tests do not build; SPEC-018's Status
without the QA files; a `source`'s phase that can also be `effect`; the debt
entry's every-mirror-green sentence, which the at-bound Resolve-then-fold test
now qualifies; "by index" for fields that have none; and K13's expected reds
naming the parent test alone. It confirmed D13's key moves and ran K1 to K36
in its own clone.

## The breaks

The commit's message carries them, grouped where breaks share a shape, with
the checks that spoke; each was run in a scratch clone holding the commit's
files and restored from its saved text, checked by hash. Thirty-six, the
plan's K1 to K36, each asserting its expected reds by name; every one went
red where the table said. K36's expected reds include the over-the-wire
threshold test, which the plan did not have.

## Rule 9: how MapTool does this

Answered in the plan, from `~/dev/RPTool/maptool` at `f4b7fef6c`: a campaign's
vocabulary, its token property names and token states, is declared once and
checked for emptiness and uniqueness, never for length
(`TokenPropertiesManagementPanel.parseTokenProperties`,
`TokenStatesController`), and every use is checked against the declaration by
membership for a token state (`TokenStateFunction`); a token property's use is
not checked, since `setProperty` stores any name (`TokenPropertyFunctions`,
`Token.setProperty`). Its frames carry any length
(`AbstractConnection.readMessage`). Borrowed: the declaration is the authority
and a use is checked against it by membership, which is what makes a bound on
the declaration bound what `Resolve` emits and the actor keys the adventure
loader admits; `add_actor`'s keys are checked against nothing and are the
second ticket's (D7). Not borrowed: the absence of a length bound, since the
server here reads at most 32768 bytes a frame. Checked and rejected: trimming
and case-folding a declared name, which would rename an author's id without
telling them.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| the ticket as written before verification | its writer corrected the problem paragraph (`rules.Load` validates by hand, and the schemas are documents), Done item 4 (a threshold's expression and a refusal read off a connection), and "What it touches"; after sign-off, it added the branch labels and the empty ids | the verification's four defects, and Q2(b) and Q4(b) |
| D10: "No gateway test" | `TestAThresholdRefusalReachesTheIssuerWithoutItsExpression` in `internal/gateway/ruleset_test.go`, with K36 expecting it red | the revised Done item 4 asks for a refusal read off a real connection, which the debt entry's own closing condition names; the existing VTT-132 test reads a `Resolve` refusal carrying no ruleset text, and the new one is red on `fcd367c` |
| D11: "No existing row changes" | VTT-132 gains the over-the-wire test | the review's S1: the refusal is `Resolve`'s, which VTT-132 states and VTT-161 does not |
| D14: "No ledger row changes", from P10's probe | eight rows lowered and five added by `--write-ledger` | the final tree's tests added more lines than the probe's, and `check:comments` refuses a share more than a point under its ceiling (the review's M1) |
| D2 and Task 12: the report raises D2's texts as a ticket | raised to the owner with this report; no ticket written | a ticket is the owner's to ask for, as the id report's raised items became tickets on the owner's word, and the owner ruled so on this report's review |
| Task 11: the breaks after C1 | the breaks before C1, in a clone of the files C1 then committed, run twice: by the reviewer and on the final tree | the process puts the break before the commit, whose message carries the result; the clone was built from the modified and untracked files the commit then held, and its `git diff HEAD --stat` matched the tree's |

## What could not be established

- **The threshold's position is 0-based**, as observed; VTT-288 does not say,
  and its tests pin the observed text.
- **A loader refusal names the file by the path opened**, absolute in the
  tests, as the map and adventure loaders do; the rows say "naming the file".
- **The branch label an outcome names** is not bounded on its own; it must
  match a label its resolution declares, which VTT-284 bounds.
- **A `use_ability` refusal still echoes the command's own ability id, actor
  id and target ids**, bounded by the frame rather than the ruleset (D8).
- **`ConditionApplied.source`** has no fold bound (Q3(a)); its only producers
  outside tests, `Resolve` and the projection, compose it from bounded parts
  or send none.
- **`ResourceChanged.resource`** has no fold bound of its own (Q5). The fold
  requires it to name one of the actor's resources, and an actor's keys, which
  `add_actor` writes and the projection's corrections re-send, are unbounded
  until the second ticket (D7). From `Resolve` it is a declared name of at
  most 128 bytes (VTT-283).
- **The loader's order against the checks that quote a name is unobserved**
  (the plan's Gap 6, D12's eighth refusal). `checkManifestBounds` runs before
  the name loops, so no load refusal quotes more than 128 bytes of a name;
  moving its call after them leaves `go test ./internal/rules/...` green,
  since no test combines an over-long name with a second fault.
- **Nothing ties the copies of a bound to the engine's constant** but each
  side's tests, and the at-bound Resolve-then-fold test reds only when the
  engine's id bound falls below `internal/rules`' copy; the debt entry stays
  open.

## What was deliberately left out, and where it went

- A ruleset's display names, its expressions, its atom ids, param names and
  graph keys, and the manifest's own id and name (D2): nowhere yet; raised to
  the owner with this report.
- The command- and file-sourced ids: the second ticket the owner asked for.
  Its plan should bound an actor's `resources` and `attributes` keys at
  `maxIDBytes` too, since the adventure loader admits only declared names as
  keys (D7).
- No proto changed; `check:breaking` names nothing.
