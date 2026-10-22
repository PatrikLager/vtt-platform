# A player's ability reaches the creature they aimed it at: the change

**Ticket:** `docs/superpowers/specs/2026-10-22-a-players-ability-reaches-the-creature-design.md`,
verified by an agent that did not write it (passes with gaps G1-G5).
**Plan:** `docs/superpowers/plans/2026-10-22-a-players-ability-reaches-the-creature.md`,
written by that verifier and amended at sign-off and by the reading review of
`985d4c2`, the commit that carried both.
**Where it came from:** finding S-1 of the alpha test of 2026-10-22,
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`, the first of the
nine tickets in the order the owner agreed ("ok, ordningen stämmer, kör").
**The owner's rulings, 2026-10-22:** at sign-off "Ja, kör", with the e2e
reading the Story rather than the ticker, which ruling B-3 of the same note
gives to the DM alone; the contract's examples renamed ("Ja, ta med det");
the question rule holds, every review finding fixed and each task committed
without asking unless it is a design question ("Ja, samma regel").

**Last code commit:** `80007aa`, on `41ccead`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `985d4c2` | The ticket, the plan, the alpha's findings note, VTT-436 and VTT-437 |
| `6e05bac` | Task 1: the contract's two `target_ids` comments, `use_ability`'s `targetIds` description, the regenerated files and the golden, `TestUseAbilitySaysItsTargetsAreActors`, and VTT-437's evidence |
| `5c083e0` | Task 2: `TestResolveRefusesATokenIDAsATarget`, and VTT-437 on `TestResolveV2WardShiftMultiTarget` |
| `0b62fa0` | Task 3: the contract's ability examples name actors |
| `80007aa` | Task 4: the panel aims at the actor; the client test, the e2e, QA's tests, the bundle, VTT-438 to VTT-441, the alpha note's A-11 and more of A-2 |
| `38e837c` | Task 5: the recipe in `docs/verification-debt.md` |

## What the code did on `41ccead`

The player panel's target button, in `renderPlayerPanel`
(`client/src/view/player.ts`), sent `useAbility(actorId, armed.id, [t.ID])`:
the id of the token clicked. `rules.Resolve` (`internal/rules/resolve.go`)
looks every `target_ids` entry up among the folded actors and refuses one it
does not find, so every ability aimed through the panel was refused with
`unknown target actor`; every token the shipped adventures place has an id of
its own. The test of that button expected the token's id, the contract's
`UseAbility.target_ids` and `AbilityUsed.target_ids` carried no comment,
`use_ability`'s tool definition gave `targetIds` no description, and the
contract's examples put token ids in both fields. No browser e2e had a player
use an ability.

## Done looks like, answered

1. [x] "a target button sends the ability at that token's actor, then
   disarms" in `client/test/spectator-view.test.ts` keeps a token `t2` showing
   actor `a2` and expects `targetIds: ["a2"]`. It was red on `41ccead`'s
   source, receiving `"t2"`.
2. [~] "a player's ability reaches the creature they aim it at", the last test
   of `client/e2e/table.spec.ts`, has the DM grant the adventure's Human Fighter
   to the player, who strikes the Goblin Cutter beside the fighter through the
   panel against the shipped binary. It requires no refusal, exactly one sent
   `use_ability` naming `act-fighter`, `longsword-strike` and `["act-cutter"]`,
   the Story's line for the ability once, and no hp line for the cutter at 8 or
   above. It was red on `41ccead`'s bundle with `use ability: it was refused —
   refused: rules: resolve: unknown target actor "tok-cutter"`, the alpha's own
   refusal. The ticket's "the creature's hp falls or the miss is recorded" is
   held as the Story's line for the ability, which `describe` writes for a hit
   and a miss alike, and that hp check. The dice are not seeded, so the e2e does
   not tell a hit from a miss; the sent frame and the refusal check hold the
   rule.
3. [x] `UseAbility.target_ids` and `AbilityUsed.target_ids` say they are actor
   ids; `use_ability`'s `targetIds` says the same in
   `contract/gen/tools/tools.json`, `cmd/vtt/tools.json` and the golden, and
   `TestUseAbilitySaysItsTargetsAreActors` reads the words. `check:drift`
   passed inside `task check`, after `generate:contract` and `build:client`
   had regenerated what it compares.
4. [x] `TestResolveUnknownTarget` is green, and
   `TestResolveRefusesATokenIDAsATarget` shows a token's id refused while that
   token and its actor both stand on the scene, where the actor's id passes the
   target lookup: the ticket's named test holds no token, so it could not see a
   `Resolve` that turned a token's id into its actor (G2).
5. [x] The recipe is in `docs/verification-debt.md`, dated 2026-10-22, and
   applied by hand it reddens the client test, the e2e and ten of QA's tests.

`task check`, whole, exited 0 on `38e837c` in 7905 s, launched
detached; no floor was lowered and no ceiling raised. `task e2e` on the same
commit: 18 passed and 1 skipped, in 25 s. The skipped one, "a network drop
while the table plays on ends in a Reconnect", is a `test.skip` in
`client/e2e/handover.spec.ts`.

## What the rules became

- **VTT-436**, "The player panel aims an ability at the actor the clicked
  token shows, never at the token": the client test, the e2e, and eight of
  QA's tests (one target, several, a companion, the caster itself, a DM, an
  agent, a token whose id is another actor's, two tokens of one actor).
- **VTT-437**, "`UseAbility.target_ids` and `AbilityUsed.target_ids` name
  actors": `TestResolveRefusesATokenIDAsATarget`,
  `TestResolveV2WardShiftMultiTarget`, which already required
  `AbilityUsed.TargetIds` to be the actors while their tokens differ, and
  `TestUseAbilitySaysItsTargetsAreActors`.
- **VTT-438 to VTT-441**, the panel's existing rules that QA cited by quote,
  which had no row in the register:
  one button per ability for the actor the panel acts as (VTT-438); target
  buttons for tokens on the acting scene at a Chebyshev distance of at most the
  range, none without a token (VTT-439); a target labelled with its actor's
  name when it has one (VTT-440); a click sends one `use_ability` as the
  selected actor, or the first the chips list, and disarms (VTT-441).

The sort over QA's candidates refused four: a multi-target ability aimed at
several targets from the panel (no such rule; finding A-11 in the alpha note);
that an actor chip's click sends no `use_ability` (what a chip's click sends is
SPEC-022's); that the panel is drawn only for a participant who controls an
actor (the fourth ticket of the alpha's order changes what a player without a
character sees); and that "disarmed" is the empty string (an implementation
value; VTT-441 states the observable).

## Phase 4a

Tasks 1, 2, 3 and 5 deliver no behaviour (contract comments and a tool
description, a test, test data, a recipe), so QA ran for Task 4 alone. One QA
agent was given the two rules and a description of the panel's surface, the
contract's `commands.proto` and SPEC-022 whole, the client's export
signatures and interfaces without bodies (extracted by a script, since the
project records `go doc` for Go only), and the DOM support file, and was not
given the source, the existing tests, the plan or the diff. It wrote 16 tests in
`client/test/qa-ability-target.test.ts`, all passing, with a token's id
different from its actor's in every fixture. Its 26 injections into its own
fixtures and expectations all behaved; source breaks against its file are
below.

### QA adjudications

- QA finding 1: spec ambiguity — whether "within the ability's range"
  includes the bound — amended: VTT-439 says "at most the ability's range",
  which `Resolve`'s refusal of a distance over the range matches.
- QA finding 2: spec gap — whether one command may carry several targets of a
  multi-target ability — no change here; recorded as finding A-11 of the alpha
  note.
- QA finding 3: spec gap — how a target button differs from an actor chip with
  the same label — no change; no rule says how a target's label differs from
  a chip's, and QA's tests find a target as a button naming the actor whose
  click sends a `use_ability`. Its helper comment now warns of it.
- QA finding 4: spec gap — what value "disarmed" is — no change; VTT-441
  states the observable.
- QA finding 5: spec gap — what the panel shows a participant who controls no
  actor — no change; the alpha's fourth ticket.
- QA finding 6: spec gap — `AbilityUsed.target_ids` is not visible from the
  panel — no change; held by `TestResolveV2WardShiftMultiTarget`.
- QA finding 7: spec gap — affordability of a resource-cost ability — no
  change; outside this ticket, and VTT-438 cites the existing "an unaffordable
  ability is shown but DISABLED, not hidden".
- Sentences needing an id: four, allocated as VTT-438 to VTT-441 after the
  sort above. QA's quote citations, which named a file outside the tree, were
  replaced with those ids; its pass-through test of `useAbility` now points at
  `UseAbility.target_ids`, since it cannot tell one kind of id from another.

## Phase 4b

One reviewer per task, by command, on copies of the tree.

- Task 0 (`985d4c2`): two passes. The first found no Must. Shoulds: G3's
  stated reason was false (`Resolve` reads the targets before the caster's
  attributes); the planned e2e could not tell which actor was hit, since
  `describe` names no target and a miss writes no hp line (it now checks the
  sent frame); R2's `AbilityUsed` half had no test; the ticket and plan cited
  the alpha's findings in a file outside the tree (now the note in
  `docs/superpowers/notes/`); Task 5's "true when" named two gates that read no
  Markdown. The second found one Must: the note's S-2 gave a cause for the
  lost token that the client's own code does not support, now "not
  established".
- Task 1 (`6e05bac`): one pass, no Must or Should.
- Task 2 (`5c083e0`): one pass, no Must or Should; its nit gave the target the
  pool a hit changes, so the refusal is about the token's id alone.
- Task 3 (`0b62fa0`): one pass, nothing.
- Task 4 (`80007aa`): two passes. The first found no Must and four Shoulds,
  all in the register and the note: VTT-441 said the panel acts as the first
  actor the player controls by id, false on a player's path, where `app.ts`
  passes the watchable set as the chips and the panel acts as their first;
  VTT-441's evidence did not see the selection ignored; VTT-440 left out the
  label's fallback to the token's id for an actor with no name (now claimed
  only for a named actor); the note's A-2 addendum put `narrationAdded` in the
  Story, where only the ticker shows it. The second found nothing.
- Task 5 (`38e837c`): two passes. The first found one Must: the recipe's
  `[t.ActorID]` also matched the label's `st.Actors[t.ActorID]`, so applying
  it as written put back a different defect; it now quotes the whole call.
  The second found one Must: the entry said only SPEC-018 had said which id,
  where SPEC-016 had too, both written after the test.

## The verifier's gaps, G1-G5

- G1: item 3's file list missed the golden and `cmd/vtt/tools.json`, and no
  test read the description; `TestUseAbilitySaysItsTargetsAreActors`.
- G2: item 4's named test holds no token; `TestResolveRefusesATokenIDAsATarget`.
- G3: the e2e fixture's Lera has no stats, so the e2e uses the adventure's
  Human Fighter and runs last.
- G4: the contract's examples used token ids; Task 3, kept by the owner.
- G5: the ticket's DM claim holds for a DM aiming as an actor granted to them;
  the ticket says so.

## The deliberate breaks

Each applied on the working tree and undone by its inverse edit or its saved
text, compared with the original, except the last row's, which the Task 4
reviewer applied to a copy; the client breaks marked bundle were built
into `cmd/vtt/webdist` and run through the whole of `client/e2e/table.spec.ts`,
whose last test depends on the earlier ones loading the adventure.

The scripted rows after F1 name what went red in
`client/test/qa-ability-target.test.ts` and `client/test/spectator-view.test.ts`,
the two files those breaks ran against; across the whole bun tier the range and
scene breaks also red existing tests in `client/test/player.test.ts`. The last
row's list is the whole tier's.

| Break | Red |
|---|---|
| K1: `targetIds`'s description removed, the files regenerated | `TestUseAbilitySaysItsTargetsAreActors`, `TestToolsMatchGolden`, `TestRunOutputMatchesCommittedToolsJSON` |
| K2: the description says "token ids", the golden updated to match | `TestUseAbilitySaysItsTargetsAreActors` alone |
| K4: `Resolve` falls back to `st.Tokens[tid].ActorID` when the actor lookup misses | `TestResolveRefusesATokenIDAsATarget` alone; `TestResolveUnknownTarget` green. Rewriting the target to its actor instead also reds it, by the empty roller's panic, and with the new test skipped the package passes under both |
| K5: the example's `targetIds` back to token ids, the client test unchanged | "useAbility matches the committed fixture, targets included" |
| K6: `[t.ID]` back (bundle) | the client test (`"t2"`), the e2e (the refusal), 10 of QA's 16 |
| K7: the caster as the target, `[actorId]` (bundle) | the client test (`"a1"`), the e2e (the sent frame's `targetIds`), 9 of QA's 16 |
| F1: the client test's token given its actor's id, with K6 | green: the different ids are what let the test see the defect |
| only the first ability drawn | QA's "one button per ability the ruleset lists", and an existing chip test |
| the range exclusive at its bound | QA's range test, with most of QA's file |
| tokens on any scene | QA's range test |
| the label as the token's id | QA's label test, with most of QA's file, which finds a target by its actor's name, and the client test |
| no disarm after the click | QA's disarm test and the client test |
| the last actor by default | QA's fallback test |
| `const actorId = mine[0]!.actorId;` in `renderPlayerPanel`, the selection ignored (the Task 4 reviewer's, on a copy) | QA's "a token showing another actor the player controls is aimed at that actor", and the existing "a selection naming an actor that no longer exists renders nothing further", and "a player's armed ability stays armed, and a switch disarms it" in `client/test/app.test.ts` |

## Rule 9: how MapTool does this

`useAbility.mts` in `~/dev/RPTool/rule-framework/library/mtscript/public/`
acts as `currentToken()` and, in its MANUAL fallback, takes its targets from
`getSelected("json")`, the selected tokens' ids, and reads each target's
stats from the token (`getProperty("HP", tid)`). That works because a MapTool
`Token` holds the creature's properties: aiming at the token and at the
creature are one act. The panel borrows the act of pointing through the piece
on the board; the step from token to actor (`Token.ActorID`) happens at the
click, since here the stats live on the actor and a token is only where it
stands. A server accepting token ids was considered and rejected: it would
give `target_ids` two meanings.

## Settled since the ticket

- The ticket's first open question: a change to a generated TypeScript
  comment alone does not move `cmd/vtt/webdist/` (the plan's E1), and
  `6e05bac`, which made one, carries no change there.
- Its second was not a question: the sight check on a target is finding A-10,
  under "What was deliberately left out" below.
- `Specifications this moves: None.` stands, by the plan's D11, a reading.

## Figures

From the one `task check` on `38e837c`:

- Go coverage: 21 packages at or above their floors, `internal/rules` 91.0%
  (floor 89.0), `tools/toolgen` 86.8% (85.0); the gate notes 11 that clear
  theirs by more than 1.0pp.
- Client coverage: 22 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors. Thirteen were
  re-mutated, each with the regenerated `contract/gen/go/vtt/v1` in its
  dependency closure, `internal/identity` through its tests; `internal/artlib`
  and `internal/campaigncfg`, which have it in neither, reused their last
  passing verdicts. 38 survivors, all adjudicated, 16 of them in
  `internal/rules`; 7 timed out and counted as killed, not measured.
- Client mutation: in its container, Stryker's own "Done in 52 minutes and 44
  seconds": 2957 mutants, 2882 killed, 9 timed out and counted as killed, 66
  survivors all adjudicated equivalent. `client/src/view/player.ts`: 241
  killed, none survived.
- Race: both runs pass.
- Requirements chain: 441 rows, 284 test files, 16 specifications.
- Comments: 319 files, 0 comment lines added against `main`, clean. The two
  `// VTT-437` lines are citation lines, which the gate counts as none, and
  the client's tests lie outside its scope of `internal/`, `cmd/` and
  `client/src`. Its two notices are the ones `a1ca051` had:
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6 and
  `internal/gateway/export_test.go` at 49.18 against 45.9.
  `tools/comment-ceilings.txt` is unchanged.
- Disk: 39.3 GiB free when `check:mutation` began, above its 16 GiB floor.
- Duration: 7905 s, from the log's first write at 13:44:20 to its last, the
  `TASK_EXIT=0` line, at 15:56:05,
  against about 3200 s for the last green run, on `a1ca051`, where the client
  mutation took 31 minutes and one Go package was re-mutated.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| The e2e reads the ticker | It reads the Story | The owner, at sign-off: ruling B-3 gives the ticker to the DM alone, and the alpha's third ticket removes it from a player's window |
| The e2e sees the outcome in the Story | It also requires the sent frame's `targetIds` | The Task 0 review: `describe` names no target and a miss writes no hp line, so K7 would have passed the Story alone |
| Two requirement rows | Six | QA cited four of the panel's existing rules by quote, and a test must cite a durable name |
| The alpha's findings outside the tree | `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`, committed with the ticket | The Task 0 review: the ticket and plan cited a file only one machine has (rule 8) |
| The recipe "replace `[t.ActorID]` with `[t.ID]`", as the plan's Task 5 wrote | The whole call quoted | The Task 5 review: the plan's text matches the label too |
| QA's file committed as written | Its citations replaced with ids, one test renamed, one comment rewritten | Its quote citations named a file outside the tree; "never the token's id" was false of an actor with no name; its helper comment described rather than warned |

## What could not be established

- How a target button should be labelled for an actor with no name: the panel
  falls back to the token's id, VTT-440 claims only the named case, and the
  alpha's fifth ticket changes what a player is told of a creature.

## What was deliberately left out, and where it went

- The server does not check that a player's character sees an ability's
  target (finding A-10): the alpha's fifth ticket.
- A multi-target ability is aimed at one target from the panel (A-11): a later
  conversation on kinds of effect, by the owner's ruling of 2026-10-22,
  "Deferred: kinds of effect" in the alpha note.
- `attack_rolled.json` keeps token ids in its actor fields: nothing in
  production writes an `AttackRolled` (SPEC-018).
- `use_ability`'s `actorId` has no description; the ticket's rule names only
  `target_ids`.
- Bare payload names in the Story and the ticker (more of A-2): the alpha's
  fifth ticket, which carries A-2.
