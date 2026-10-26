# The client's full mutation run happens when the owner asks, and `task check` says when it last did

## The problem

**`task check` starts an hour of Stryker whenever the client has changed.**
`check:` in `Taskfile.yml` runs `check:ts-mutation`, whose last command hashes
the report's inputs with `tools/ts-mutation-inputs.py` and compares the hash
with `reports/mutation/ts-inputs.sha256`. When the two differ, or there is no
`reports/mutation/mutation.json`, it produces the report: on macOS 26 by
`task check:ts-mutation:docker`, elsewhere by `npx stryker run`. Then it
stamps the hash, and on every path it judges the report with
`tools/check-ts-mutation.py`. Nothing in the step can leave the run out. A
run mutates `client/src` but `client/src/main.ts`, whatever changed: on
`db81c37` that was 3276 mutants, and Stryker's own "Done in 86 minutes and 38
seconds" (the Figures of
`docs/reports/2026-10-26-a-player-speaks-as-their-character.md`).

The owner's rule of 2026-10-26: "vi kör bara Full Stryker, när jag
efterfrågar det. Du påminner mig och räknar upp alla ändringar (tickets) som
gjorts sedan senaste Stryker körningen. Och jag avgör om jag vill att Stryker
ska köras." Asked what `task check` does when the stored report is out of
date, the owner answered "Går igenom med varning": it passes, and warns.

**Nothing records which tree the stored report measured.** The stamp holds
the input hash and nothing else. `mutation.json`'s top-level keys are
`files`, `schemaVersion`, `thresholds`, `testFiles`, `projectRoot`, `config`
and `framework`; none names a commit. So the tickets since the last run
cannot be listed from what is stored.

**The hash misses inputs the client imports from outside `client/`.**
`TREES` in `tools/ts-mutation-inputs.py` is `client/src` and `client/test`,
and `FILES` is `stryker.conf.json`, `package.json`, `bun.lock`, `bun.lockb`
and `client/tsconfig.json`. Twelve files under `client/src` import
`contract/gen/ts/vtt/v1/commands_pb.ts` or `events_pb.ts`, and
`client/test/dm-view.test.ts` and `client/test/join-url-format.test.ts`
import `contract/testdata/join_url_format.json`. A change to any of those
three files leaves the hash where it was, so a report made before it reads
as current. The client's tests also read files outside `client/` by a path
written in them: `client/test/fold-parity.test.ts` reads
`scenarios/goldens`; `commands.test.ts`, `fold-unit.test.ts` and
`removal-batch-parity.test.ts` read `contract/testdata`;
`qa-id-bound.test.ts` and `qa-cmd-id-bound.test.ts` read
`contract/gen/tools/tools.json`; and `art-assets.test.ts` reads
`campaigns/example/art` and `campaigns/example/maps/cellar.json`. A change to
any of them leaves the hash where it was too.

**Four texts say `task check` runs the client's mutation gate.**
`check:ts-mutation`'s description says "IN `task check`, AND NOT SCOPED", and
the banner it prints says "now the gate runs it". `CLAUDE.md` says both
mutation gates run "when somebody types `task check`". SPEC-020's Status says
`check:mutation` and `check:ts-mutation` "run inside `task check`". The
description and the banner also say why the gate was made to run itself:
from 2026-08-11 to 2026-08-25 it tested zero mutants while everyone had a
reason to think something else ran it, and on 2026-08-25 it found 125
survivors (`tools/ts-mutation-scope.md`). The owner's rule overturns that
decision.

**How MapTool does this (rule 9).** It does not: this is the project's own
gate tooling, and no Gradle file under `~/dev/RPTool/maptool` names a
mutation tool (`grep -rli 'pitest\|stryker'` over them finds none).

## Done looks like

1. Run with the client changed since the stored report was made,
   `check:ts-mutation` starts neither Stryker nor its container, exits 0, and
   prints that the client's mutation was not measured, the commit the stored
   report measured, and every ticket added under `docs/superpowers/specs/`
   since that commit. Today it starts the container, or Stryker.
2. Run with no stored report, `check:ts-mutation` starts neither, exits 0, and
   prints that there is no report. Today it starts the container, or Stryker.
3. Run with a current report, `check:ts-mutation` judges it against
   `tools/ts-mutation-equivalents.txt`, and a survivor no entry claims fails
   it. This holds today and must still hold.
4. A task that is not a step of `task check` runs Stryker over the
   `mutate` list in `stryker.conf.json`, unchanged, and judges the report;
   and a `check:ts-mutation` run right after it calls the report current and
   names the commit it measured. Today `check:ts-mutation:docker` runs
   Stryker and judges, and records nothing.
5. A change to `contract/gen/ts/vtt/v1/events_pb.ts`, one to
   `contract/testdata/join_url_format.json`, and one to
   `scenarios/goldens/adventure-night/state.json`, each moves the report's
   input hash. Today none does.
6. `CLAUDE.md`, `Taskfile.yml`'s descriptions and banners, and
   `docs/specifications/` hold no sentence saying `task check` runs Stryker.

## Rules this puts on the system

- `task check` never starts Stryker.
- With the stored report out of date, `task check`'s client mutation step
  passes, and says the client's mutation was not measured, which commit the
  report measured, and which tickets were added since.
- With no stored report, the step passes and says there is none.
- With the stored report current, the step judges it as it does today.
- The full run records which commit's tree it measured.
- A change to any tracked file a client source or test imports, or that a
  client test reads by a path written in it, moves the report's input hash
  (the owner's answer "Ja", 2026-10-26).

## What it touches

1. `Taskfile.yml`: `check:ts-mutation`, its description and its banner;
   `check:ts-mutation:docker`, or a new task beside it, for the run on
   request.
2. `tools/ts-mutation-inputs.py` and `tools/ts_mutation_inputs_test.py`.
3. Wherever the step's decision is made testable, a script under `tools/` and
   its test, and `_report_is_current` in `tools/check_ts_mutation_qa_test.py`
   if the stamp's format changes.
4. `CLAUDE.md`'s "How this project is built and checked".
5. `docs/specifications/020-a-mutation-adjudication-names-the-code-it-excuses.md`,
   and a new specification for the decision (the owner's answer "Ny
   SPEC-023", 2026-10-26).
6. The other texts that say who runs the gate: `main()`'s message in
   `tools/check-ts-mutation.py`, a recipe in `docs/verification-debt.md`,
   and the header of `.github/workflows/ci.yml`, which names
   `check:ts-mutation:docker`.

It does not touch:
- `mutate`, or any threshold or adjudication. The full run mutates what it
  mutates today.
- The Go gate, `check:mutation`, which stays a step of `task check`.
- A run per changed file. Measured 2026-10-26 over the client's import
  graph and `mutation.json`'s 3276 mutants, for a change to
  `client/src/view/player.ts` alone: rerunning every file reached by a test
  that reaches the changed file, which keeps today's guarantee, reruns all of
  them; the narrower rule of the file, what it imports and what imports it
  still reruns 45 per cent. The implementation report keeps the method.
- CI. The owner, 2026-10-26: "vi kör inte CI på github längre".
- The reminder itself, which is the session's, not the tree's.

## Specifications this moves

docs/specifications/020-a-mutation-adjudication-names-the-code-it-excuses.md
New: when the client's full mutation run happens, what its stamp records,
and what `task check` says of the stored report.

## What could not be established

- What the stored report made on `db81c37` records as its commit once the
  stamp carries one: its hash matches today's tree, and nothing stored says
  which commit made it.
- Which tickets count as "since" when the report's commit is not an ancestor
  of the branch being checked.
- Whether a sentence outside the four texts named, in `tools/` or
  `docs/reports/`, states that `task check` runs Stryker as a present fact.
  A report is never revised to match later work, so one there would stand.
