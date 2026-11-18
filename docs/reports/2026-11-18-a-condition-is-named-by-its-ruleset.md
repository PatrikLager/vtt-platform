# A condition is named in the ruleset's words wherever a window shows it — report

Ticket: `docs/superpowers/specs/2026-11-18-a-condition-is-named-by-its-ruleset-design.md`.
Plan: `docs/superpowers/plans/2026-11-18-a-condition-is-named-by-its-ruleset.md`.
Branch `feat/a-condition-is-named-by-its-ruleset` from main `b424e4c`.
Commits: `82952f9` (Task 0: ticket, plan, register), `52801e4` (Task 1, the
namer, the dots and the lines), `f0d561b` (Task 2, the console),
`2df13c0` (Task 3, the window and the real wire; the last code commit) and
`49f9c0e` (the comment ledger). Every code reference below is to
`2df13c0`.

## Done looks like

1. [x] **A dot names its condition.** `namer` (`client/src/view/naming.ts`)
   takes the window's condition list; `Namer.condition` gives the name the
   list gives an id, else the id, an empty name read as none (the owner's
   OD-1 A), by `nameIn`, the lookup that names an ability; and
   `Namer.conditionTitle` that name followed by a line break and the
   description when it is non-empty. `tokensOnScene`
   (`client/src/view/grid.ts`) gives each `ConditionDot` a `label` from it,
   and the board titles the dot by it.
   - `client/test/naming.test.ts`'s "a condition is named by the list's
     name, else by its id, and by its id undeclared or with no list" and "a
     condition's title is its name, a line break and its description as
     written, else its name alone"; `client/test/grid.test.ts`'s "a dot
     carries its condition's title beside its id, in applied order";
     `client/test/spectator-view.test.ts`'s "a dot is titled by its
     condition's name and description from the window's list, else by its
     id".
   - Independently: `client/test/qa-condition-names.test.ts`.
   - On `b424e4c` a dot is titled by its id, and these tests fail.
2. [x] **The Story and the Events panel name it.** `describe`'s
   `conditionApplied` and `conditionRemoved` lines
   (`client/src/view/spectator.ts`) name the condition by
   `Namer.condition`; the Story's `tell` and the Events panel's
   `renderTicker` both read `describe`.
   - `client/test/spectator-view.test.ts`'s "the Story and the Events panel
     name a condition gained and lost by the window's list, else by its
     id"; independently, `client/test/qa-condition-names.test.ts`.
   - The tests that read a condition by its id with no list stay green
     unchanged: `client/test/qa-story-mechanical.test.ts` reads
     `gained prone`, `client/test/qa-companion-first-sight.test.ts`
     `gained blessed`.
   - On `b424e4c` a line names the condition by its id, and these tests
     fail.
3. [x] **The DM console names both.** `renderDMConsole`
   (`client/src/view/dm.ts`) labels each "Remove condition" button by the
   actor's name in the fold, else its id, `: `, `Namer.condition` of the
   condition and ` ✕`, and still sends `removeCondition` with the ids.
   - `client/test/dm-view.test.ts`'s "a removal button names the actor by
     its name in the fold and the condition by the window's list", "a
     removal button names an actor with an empty name, or one the fold
     lacks, by its id", "with no list a removal button names the condition
     by its id" and "a removal button named by the list still sends the
     actor's and the condition's ids"; independently,
     `client/test/qa-console-condition-names.test.ts`.
   - On `b424e4c` a button reads the actor's id and the condition's id, and
     these tests fail.
4. [x] **The newest answer names it.** `paint` (`client/src/app.ts`) hands
   `layout.conditions`, the newest `/api/ruleset` read's list, to the board
   and the lines as `ViewExtras.conditions` and to the console as
   `DMDeps.conditions`.
   - `client/test/app.test.ts`'s "a player's window names a condition on
     its dot and in its Story by its newest ruleset answer", whose older
     read answers after the newest and changes neither, and "a DM's window
     names a condition on its dot, its Story, its Events panel and its
     removal button by its answer"; independently,
     `client/test/qa-condition-window.test.ts`.
   - On the real wire, under the owner's OD-2 A:
     `client/e2e/conditions.spec.ts`'s "a DM's window names a condition a
     hit applies on its dot, its Story, its Events panel and its removal
     button" reads the name and description from
     `rulesets/dnd45e-minimal/conditions/dazed.json` and finds them on the
     DM's page after a Staggering Blow certain to hit, then removes the
     condition by its button.
5. [x] **The gate.** `task check` is green on `49f9c0e`, the ledger commit
   on top of `2df13c0`, and `task e2e` is green on the same commit,
   `client/e2e/conditions.spec.ts` among its specs; the embedded bundle
   under `cmd/vtt/webdist` was rebuilt in `2df13c0` from that commit's
   client. `task check` did not measure the client's mutation: its stored
   report measured `09c55a9` (SPEC-023).

## What the rules became

- A dot's title: VTT-862.
- A line in the Story and the Events panel: VTT-863.
- The console's removal button, and the ids it sends: VTT-864.
- The newest read's list names them: VTT-865.

VTT-475 and VTT-835 stand as worded; their evidence gained this ticket's
QA tests of the actor in a line and of the dots' count and order.

Refused at the sort: merging VTT-865 into VTT-845, which stands for the
panel; and a row for the removal group's buttons, one per condition held,
which is today's behaviour, held by `dm-view.test.ts` and by no row, and
not changed here.

Every row from VTT-862 to VTT-865 cites tests; none is open.

## The deliberate breaks

Each was made by an edit, run, and restored, the file's hash checked
afterwards; each turned its test red.

- **Task 1:** the dot titled by its id again; the description first, and
  joined by a space; the empty-description filter dropped; `??` in place
  of `||`, so an empty name stood; any list entry's description taken;
  each line naming the condition by its id; a description's line breaks
  made spaces; after the review, `??` in the shared lookup `nameIn`.
- **Task 2:** the actor named by the namer's GM chain; the id fallback
  dropped for an empty name; an actor the state lacks read unguarded; the
  condition's name sent in place of its id; the condition named by its id.
- **Task 3:** every answer's list taken as it arrives; the console handed
  no list; the board handed none; the client reading the condition's name
  under another key, with the bundle rebuilt, which turned the e2e spec
  red, the bundle then rebuilt back byte-identical; after the review, the
  window keeping an earlier list when a newer answer gives none.

The break script's guard stopped none of these.

## Rule 9: how MapTool does this

In MapTool's core a token state is identified by its name
(`AbstractTokenOverlay`), so the word shown is the name. The owner's 4.5e
framework under `~/dev/RPTool/rule-framework` gives a condition an `id` and
a `label` (`schema/condition.schema.ts`), and its scripts print the label,
else the id, when a condition is applied
(`library/mtscript/public/lib/RE_applyCondition.mts`) and in the
remove-condition picker (`library/mtscript/public/removeCondition.mts`).
Borrowed: the ruleset's name, else the id, wherever a condition is shown.

## The plan's gaps and the owner's answers

- **G1** (the button's sending of the ids in no rule) and **G3** (how an
  undeclared condition reaches a window): closed by the ticket's amendment
  before Task 0's commit.
- **G2** (an empty name): answered by OD-1 A; the ticket's item 1 and rule
  1 were amended to say so.
- **G4** (tests and records the ticket does not name): touched as the plan
  listed them.
- **G5** (a DM's or an agent's window reads once): VTT-865's older-answer
  clause is reachable in a player's window only; the plan's note under its
  table says so.
- **G6** (a long description in a dot's title, and touch platforms that
  draw no title): unchanged and unmeasured.

The owner's answers, 2026-11-18: every place names a condition, the
console's actor as its ability rows name an actor; a dot's hover is the
name, a line break and the description; OD-1 A; OD-2 A.

## The existing tests that changed

- `client/test/dm-view.test.ts`'s "a condition on an actor gets a removal
  button that sends removeCondition" clicks "A: dazed ✕", its fixture's
  actor being named, where it clicked "a1: dazed ✕", and now asserts the
  ids it sends beside the command's case.

## Phase 4a adjudications

No QA agent found a behaviour defect. One line per finding.

- Task 1 (`client/test/qa-condition-names.test.ts`), spec gap: SPEC-022
  says a condition is shown as the entry names it — goes into SPEC-022.
- Task 1, spec gap: SPEC-022's naming names no condition — goes into
  SPEC-022.
- Task 1, out of scope: the board versus the namer handed the list — the
  window's, Task 3.
- Task 1, unreachable: a condition id twice in the list, or twice on an
  actor — the ruleset's conditions are a map keyed by id, and the fold
  refuses a duplicate condition on an actor.
- Task 2 (`client/test/qa-console-condition-names.test.ts`), ambiguity: the
  label read as the button's trimmed text — the button's text is the label
  alone.
- Task 2, unreachable: an actor holding one condition twice — the fold
  refuses a duplicate condition on an actor.
- Task 2, spec gap: SPEC-022 has no sentence on the removal button's label
  — goes into SPEC-022.
- Task 3 (`client/test/qa-condition-window.test.ts`), spec gap: VTT-865's
  older-answer clause means something in a player's window only — G5.
- Task 3, ambiguity: the actor's words in a line — VTT-475 and VTT-863's
  tests hold them.
- Task 3, today's behaviour: a player's window reads at its first
  `watchableChanged` (VTT-753); no row here.

## Phase 4b

A reviewer read each task's diff before its commit; Task 0's was read
again after its fixes.

- **Task 0:** the e2e design's actors holding `flurry_uses`,
  which `add_actor` refuses (VTT-775); OD-1 A in no row or test; "newest
  answer" meaning two things across the rows; then VTT-775 restated too
  broadly, a count in the ticket, and the plan's draft table reading as
  binding.
- **Task 1:** nothing to fix but the condition lookup copying the
  ability's, now the shared `nameIn`.
- **Task 2:** a QA test cited for VTT-864 that no break of the row turns
  red. The re-keyed `BooleanLiteral` entry in
  `tools/ts-mutation-equivalents.txt` names `condition`, not
  `conditionTitle` as the plan's D6 says, since the console reads only
  `condition` and `resource` from its namer; the reviewer checked it with
  the equivalents tool's own entry check and by flipping `true` in a copy,
  the client suite staying green.
- **Task 3:** QA tests cited for VTT-445 and VTT-845, which no break of
  those rows turns red; QA's tests of a newest answer with no list
  answering a read the window ignores, so they passed on `f0d561b`
  (restructured, seen red). The reviewer ran the e2e spec on fresh tables
  in a copy of the tree, green, and red against a bundle from `f0d561b`'s
  `app.ts`, and checked its determinism: a hit on every roll, no
  `bloodied` or `dying`, the target in reach, both actors accepted, and
  the target named "Dummy" in every place.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The plan's D11: Tasks 1 and 2 change nothing the running window draws | Task 2's commit names the actor on a running DM console's removal buttons at once | `paint` already hands the console the fold; only the condition's half waited for Task 3's list |
| The plan's D6: the re-keyed entry's reason names `condition` and `conditionTitle` | It names `condition` beside `resource`, and no `conditionTitle` | The console reads only `condition` and `resource` from its namer |

## What could not be established

- **A long description in a dot's title, and a title on touch
  platforms** (G6). No browser was asked.
- **No gate typechecks `client/e2e`.** `client/tsconfig.json` includes
  `src`, `test` and the generated contract, not `e2e`; the e2e specs, this
  one among them, are run by Playwright and checked by no compiler step.
- **The client's stored mutation report** measured `09c55a9`; this ticket
  changed `client/src/view/naming.ts`, `grid.ts`, `spectator.ts`, `dm.ts`
  and `client/src/app.ts` after it, so `task check` judges nothing of it
  until a full Stryker run, which is the owner's to ask for (SPEC-023).

## What was deliberately left out

- The agent's tools, which answer condition ids as data and `get_ruleset`
  their names.
- A condition's dot colour or icon.
- Any other id the DM console shows.
- The panel's own naming of a condition (`sheetFor`, VTT-842), left as it
  was (the plan's D8).
