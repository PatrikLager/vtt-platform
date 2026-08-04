# What check:mutation gates, and what it does not

`tools/check-mutation.py`'s `PACKAGES` list is the gated set. This file records
what is OUTSIDE it, with measured numbers, so the gate's narrowness is a
published figure rather than an impression — the same job
`tools/ts-mutation-scope.md` does for the TypeScript side.

The rule it follows is the one `check-mutation.py` states about itself:

> This list was ONCE the five packages that happened to have been measured
> while writing the gate — which silently dropped `internal/campaign` [...]
> Gating it immediately found three real survivors in c.head's batch
> arithmetic. **A gate scoped to what its author touched is not a gate.**

## The defect this file was written during: unresolvable packages

**A `package main` in a directory not named `main` cannot be measured by this
gate at all, and fails SILENTLY as a perfect score.**

gremlins picks a mutant's test target by walking up from the mutated file for a
directory whose NAME equals the declared package name. `cmd/vtt` declares
`package main` and no ancestor is named `main`, so gremlins falls back to the
bare module path and runs `go test github.com/PatrikLager/vtt-platform`. That
does not resolve, `go test` exits 1, and **exit 1 is exactly what gremlins
scores as KILLED**. Every mutant becomes a false kill in ~11ms.

This was found on 2026-08-04 by review, inside a change that was ADDING
`cmd/vtt` to `PACKAGES` on the strength of "91 mutants, 91 killed, 0 lived,
~9.4s". Two things make it worth recording at length:

- **`tools/toolgen` had the same shape and had been in the gated set all
  along.** One of the gated packages had never been measured. Correctly
  measured it is genuinely 9 killed / 0 lived — the right answer, arrived at
  by luck.
- **Sampling cannot detect it.** The author hand-applied a KILLED mutant and
  watched the suite go red, which felt like verification and was not: a
  constant "everything dies" verdict passes that check too. What exposes it is
  the inverse — apply a mutant that is provably EQUIVALENT and see whether the
  tool still says KILLED. `cmd/vtt/state_dump.go:153` (`>` → `>=`, reassigning
  the same value when equal) is killable by no test, and gremlins called it
  killed.

`check-mutation.py` now refuses to run against such a package
(`unresolvable_packages`), with boundary tests in `check_mutation_test.py`.
The guard is structural rather than sampled, for the reason above.

## Gated

Whatever is in `PACKAGES`. The count is deliberately not restated here — it
drifted within an hour of this file being written.

## Not gated

| package | survivors | not covered | runtime | why |
|---|---|---|---|---|
| `internal/rules` | **59** | **40** | 7m47s | backlog, no reason on record |
| `internal/adventure` | **14** | **4** | 1m06s | backlog, no reason on record |
| `internal/harness` | 2 | 32 | <4m | **blocked, argued** |
| `cmd/vtt` | 0 of 77 evaluated | 7 | **~32m+** | **unresolvable (above)** |
| `tools/toolgen` | 0 | — | ~1s | **unresolvable (above)** |

`internal/rules` and `internal/adventure` are outside the gate with **no
recorded reason anywhere**, not excluded on an argument but never added. That
`internal/adventure/conformance` was gated while `internal/adventure` was not
is the signature of a list assembled from what happened to be measured.

### `internal/rules/conformance` — worked down and gated 2026-08-04

10 survivors → **0 unadjudicated** (58 killed, 2 adjudicated equivalent).

**Six of the ten lived behind one hollow test.** `TestRunSmokeFailure` asserted
only that the error mentioned the failing ability `"big-move"` — and the
fixture produces such an error by two entirely different routes. With the
fixture actor's resource max silently falling back to 1000 instead of the
declared 1, the ability became affordable, the smoke pass SUCCEEDED, and Run
failed later with `has no golden scenario` — still naming big-move, still
green. The P4 proof harness's own proof was unpinned by a substring match.
The test now pins the reason: `insufficient`, and `have 1, need 5`, where 1 is
the value only `default_max_expr` can produce.

Two needed new fixtures, both recording behaviour that had never been
exercised:

- `minimal-v2-zero-default-max` — a `default_max_expr` evaluating to exactly 0,
  which must fall back rather than build an actor that can afford nothing.
- `minimal-v2-rolls-exhausted` — a golden recording NO rolls, so the roller
  exhausts at exactly `i == len(steps)`. Off by one the other way it indexes an
  empty slice and panics, blaming the platform for an authoring mistake.

Two are genuinely equivalent and are adjudicated in
`tools/mutation-equivalents.txt`: the `MaxInt32` clamp assigning a value it
already holds, and the golden scene name — the latter equivalent *only* because
`goldenFile` has a single `scene` field stamped onto every token, so the
caster/target mismatch the same-scene filter exists to catch is inexpressible.
An empty scene id is emphatically NOT interchangeable with a name in general
(`resolve.go:677` disables the filter on ""), and that entry is written to
expire the moment `goldenActor` gains a per-actor scene.

`internal/harness` is excluded on an argument, and that argument lives in
`check-mutation.py`'s header where the list is — not duplicated here. Short
version: its last real survivor guards a RACE, and `testing/synctest`'s fake
clock makes the guard's removal detectable only ~60% of the time. Read the
header; it has been rewritten twice and each version was true when written.

`cmd/vtt` is NOT an unrecorded omission — ADR-010:96-97 excludes it explicitly
("unmeasured for the same reason (~40s per test run) and carries the same
obligation"), revisited at ADR-010:156-160. Its numbers above come from a
manual run in a worktree with the directory renamed so resolution works: 85 of
100 mutants in **32 minutes**, of which 75 killed, 2 timed out, 7 not covered,
and **no genuine survivors among the 77 evaluated**. So its tests do look
strong; the cost is the problem, not the quality.

**`internal/rules` is the one that matters most.** It is the rules interpreter
— `compile`, `expr`, `resolve`, `load`, `schema`, `format`, `crypto_roller`,
4,224 lines — which is what ADR-002 ("rules as declarative data") rests on. Its
40 NOT COVERED mutants are consistent with its 89.0% coverage floor and break
no gate, but "10% of the rules interpreter is unreached by any test" is the
legible form of that number, and a coverage percentage is exactly what ADR-010
says cannot answer the question mutation answers.

## NOT COVERED is measured here but not enforced anywhere

**83** mutants across the table above sit in code no test reaches (40 + 4 +
32 + 7 + 0). `check-mutation.py` has no regex for `NOT COVERED` and no check on
it, so the gate passes with them present.

That is a deliberate open question, not an oversight to fix in passing:
`check:coverage` already enforces per-package line floors, so making NOT
COVERED fatal here would either duplicate that gate or contradict it, and it
would need the gated packages measured first — any one of them carrying a
not-covered mutant would turn `task check` red the moment it landed.

Worth noting the inconsistency while it stands: the gate prints every TIMED OUT
mutant on the stated principle that *"there is no excuse for the gate to know
less than its own input"*, then silently discards NOT COVERED, which gremlins
names in the same output. Same category of unmeasured mutant, opposite
treatment.

## Some of the gated packages' "kills" are timeouts, not evaluated detections

Two gate runs on 2026-08-04 over IDENTICAL code: `internal/gateway` 5 then 6,
`internal/campaign` 2 both times, `internal/mcp` 1 then 3. Do not quote a
single run's figure as the count — the variance is the finding, and it is the
same cache-driven timeout effect described at the foot of this file, where
`testExecutionTime = coverage_elapsed × coefficient` makes every mutant's
deadline depend on whether the baseline run was cached.

`check-mutation.py` counts a timeout as a detection
(Patrik's ruling, 2026-07-28) and prints each with "counted as killed, NOT
measured" — loud rather than hidden, but nothing carries the standing total,
and a number nobody totals is a number nobody watches. Same category as the 11
recorded in `tools/ts-mutation-scope.md`.

The gate's own advice on each line is the right fix — make the test that blocks
under the mutant fail fast — rather than adjudicating them or raising the
timeout.

## The order, and why gating is the LAST step

Adding a package to `PACKAGES` before its survivors are at zero turns
`task check` red. So the sequence is always: measure, work to zero
unadjudicated survivors, THEN gate. Never gate first, and never adjudicate a
survivor equivalent merely to get the gate green —
`tools/mutation-equivalents.txt` opens with what that costs, having had two of
four adjudications turn out wrong on 2026-07-27.

Smallest first. `internal/rules/conformance` (10) is done; next
`internal/adventure` (14) → `internal/rules` (59).

## Reproducing any number here

    /usr/bin/time -p go tool gremlins unleash ./internal/rules/ \
        --workers 1 --timeout-coefficient 30

`--workers 1` matters: it is what `check-mutation.py` uses, and higher worker
counts produce CPU-contention false timeouts. `--dry-run` counts mutants
without executing tests, which is the cheap way to size a package first.

**gremlins re-runs the whole package suite once per mutant**
(`executor.go:193-241`), as ADR-010:92-93 states. Coverage-guidance only skips
NOT COVERED mutants without executing anything. So mutants × suite-time is the
correct model for projecting cost — `internal/mcp`'s ~857s for 64 mutants is
that arithmetic from the other direction. An early draft of this file claimed
the opposite, on the strength of a `cmd/vtt` run that appeared 270× faster than
projected; the projection was right and the run was the broken one described at
the top.

**Two measurement traps, both of which have produced wrong numbers here:**

1. **gremlins' self-reported duration is not wall clock.** It excludes the
   coverage-gathering phase — 6.37s reported against 35.64s measured. Every
   runtime in this file comes from `/usr/bin/time real`.
2. **Go's test-result cache on the coverage baseline changes everything.**
   gremlins runs its baseline without `-count=1`, so a warm cache turns a 27s
   baseline into 0.17s. That is not just a faster run: `testExecutionTime =
   coverage_elapsed × coefficient` (`executor.go:101`), so a cached baseline
   shrinks every mutant's timeout from ~13m42s to ~4.8s. That is the likeliest
   explanation for the "58 timeouts, then 1, then 58 again across three runs of
   identical code" that `check-mutation.py` currently attributes to
   compilation. Compare like with like, and state the cache state.
