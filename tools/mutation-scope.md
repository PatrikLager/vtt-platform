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

## The second route to the same lie: a symlink the copy drops

Found 2026-08-05 while testing whether `--integration` could gate `cmd/vtt`.

gremlins copies the module before mutating it, and the copy DROPS symlinks.
`copyPath` (`workdir.go:142`) switches on `mode.IsDir()` and `mode.IsRegular()`; a symlink is
neither, so it falls through the switch and is skipped **with no error**. The
path is simply absent from the copy. Any test that reads it fails there under
every mutant, exiting 1 — and exit 1 is what gremlins scores as KILLED
(`getTestFailedStatus`, `executor.go:257`). Perfect efficacy, nothing
detected.

`scenarios/testdata/dnd45e-minimal-adventures/goblin-ambush` is the repo's only
TRACKED symlink — the working tree carries 39, but the other 38 are under
`node_modules`, which the guard does not walk. It exists because `serve --adventures-dir ./adventures` cannot boot —
the two committed adventures declare different rulesets
(`docs/superpowers/plans/2026-07-26-client.md:718`) — so it presents a
single-ruleset view of one adventure without duplicating it. `cmd/vtt`'s
scenario tests boot a server against that directory.

**`--integration` does not rescue this; it is how the route was found.**
Integration mode ignores the broken package resolution entirely — `cmd.Dir` is
the copied module root and the target is `./...` (`executor.go:200`, `:236`) —
so it looked like the fix. It is not, because its oracle is *the whole suite's
exit code*, and the suite cannot pass in a copy with a dropped symlink. Measured
on `internal/engine/apply.go`, which has two survivors already adjudicated
equivalent: **48 killed, 0 lived** — both equivalents reported killed. The
timing said the same thing independently: 8.7s per mutant against 29s for a real
`go test ./...`.

Note what this implies beyond the symlink. Integration mode's verdict is
contaminated by ANY failure anywhere in `./...`, so a single flaky test in an
unrelated package silently converts the gate to a constant. That is a worse
foundation than per-package mode even once every symlink is gone.

`dropped_symlinks()` guards it, with `ALLOWED_SYMLINKS` carrying a reason per
entry that must name the packages the symlink makes unmeasurable. Five fault
injections, each run against the WHOLE suite with a green baseline either side:

| injection | fires |
|---|---|
| walk only `filenames`, missing symlinked DIRECTORIES | 5 tests |
| `ALLOWED_SYMLINKS` emptied | `..._real_tree_carries_no_unrecorded_symlink`, `..._allowlist_is_not_vacuous` |
| reason requirement removed | `..._entry_without_a_reason_is_fatal` |
| `run()` no longer consults the guard | `..._gate_fails_and_names_the_symlink...` |
| prune `UNWALKED_DIRS` BEFORE checking | `..._symlink_NAMED_like_a_vendored_dir...` |

**A methodology trap worth keeping, because it silently faked two of those
numbers first.** macOS system Python sets `sys.pycache_prefix`, so bytecode is
cached in `~/Library/Caches/com.apple.python/...`, NOT in a `tools/__pycache__`
you can see. Staleness is judged on (mtime, size). An injection that moves a
line without changing the file's SIZE, restored by `cp` within the same second,
leaves that cache valid — so the suite runs the INJECTED bytecode against
restored source, and the result is unrelated to the file on disk. It presents as
a test failing while the source is provably correct. Clear the cache, or run
`python3 -B`, before trusting any fault-injection result on these tools.

That is the same shape as the "run the suite, not the test" note further down:
both are ways an injection reports a number about something other than the code
being reviewed.

## Gated

Whatever is in `PACKAGES`. The count is deliberately not restated here — it
drifted within an hour of this file being written.

**A gated parent excludes its gated children.** `gremlins unleash
./internal/rules/` RECURSES into subdirectories and reports those mutants
relative to the package it was given — `conformance/conformance.go:207:11`
rather than the child's own path. With both parent and child in `PACKAGES`
that is wrong twice: the same mutant is measured under two different keys, so
an adjudication written for the child does not match the parent's report and
an already-excused survivor is reported as unadjudicated; and it costs the
runtime twice. `internal/adventure`'s run carried **23** of its child's
mutants, `internal/rules`' carried **60**.

`gremlins_args` now passes `--exclude-files` for any gated child, verified
against the real tool (23 → 0, with the parent's own 75 untouched). Only GATED
children are excluded: a subdirectory not separately in `PACKAGES` is measured
by its parent or nowhere, and dropping it would trade a visible gap for an
invisible one.

Found 2026-08-04 while measuring `internal/rules` — two survivors turned up in
a file that is not in `internal/rules`. The gate had been double-measuring
`internal/adventure/conformance` since the moment its parent was gated, and was
green and honest the whole time, because that child happens to have no
survivors to double-report. Nothing was ever going to point at it.
`internal/adventure` ⊃ `internal/adventure/conformance` was the only such pair
when the guard was written, and the second one — `internal/rules` ⊃
`internal/rules/conformance` — arrived a day later, which is why the guard is
the thing that finds them rather than this sentence. Enumerate the pairs from
`PACKAGES` rather than trusting a count written here.

**It had already contaminated a published number.** This table said
`internal/rules` had **59** survivors. Ten of those were
`conformance/conformance.go` — the already-gated child's, already adjudicated.
Its own count is **49**, confirmed from both measurement runs (59−10 and 51−2,
which agree). The NOT COVERED 40 was clean. So the defect this section
describes had quietly overstated the last remaining backlog by 20%, in the file
whose stated job is measured figures rather than impressions.

**The rule that follows, stated once so it is not rediscovered:** a parent's
published figure INCLUDES any UNGATED child's mutants, attributed to the parent.
Only gated children are excluded, because an ungated subdirectory is measured by
its parent or nowhere. There is no ungated child in the tree today, but when
there is, that is what its parent's number means.

## Not gated

| package | survivors | not covered | runtime | why |
|---|---|---|---|---|
| `internal/harness` | 2 | 32 | <4m | **blocked, argued** |
| `cmd/vtt` | **never measured** | unknown | unknown | **unresolvable + symlink** |
| `tools/toolgen` | 0 | — | ~1s | **unresolvable (above)** |

`cmd/vtt`'s row held "0 of 77 evaluated" until 2026-08-05. It was retracted, not
recomputed: every run of it so far has been a constant-KILLED oracle, by one of
two independent routes (below). `tools/toolgen`'s 0 stands — its tests pass in a
symlink-free copy, which is what makes its renamed-worktree measurement clean
and `cmd/vtt`'s not.

**No package remains outside the gate on no argument.** All three that were in
that state on 2026-08-04 — `internal/rules/conformance`, `internal/adventure`
and `internal/rules` — have been worked to zero unadjudicated survivors and
gated (below). What is left out is left out for a stated reason: `harness`
because its last survivor is killed only ~60% of the time and a probabilistic
gate is a flaky one; `tools/toolgen` because gremlins cannot resolve `package
main` in a mismatched directory and scores every mutant a false kill; `cmd/vtt`
for that reason AND, independently, because its scenario tests read a symlink
the workdir copy drops (below) — fixing either one alone leaves the other
producing the same constant verdict.

That `internal/adventure/conformance` was gated while `internal/adventure` was
not, for as long as it was, remains the signature of how the list was
originally assembled — from whatever happened to have been measured.

### `internal/adventure` — worked down and gated 2026-08-04

14 survivors + 4 NOT COVERED → **0 unadjudicated, 0 unreached** (96 killed, 1
adjudicated equivalent). It is a validator, so almost every survivor was an
inclusive limit nobody had tested ON.

**Seven boundaries fell to one fixture.** `testdata/at-every-boundary` sits
exactly on all of them at once and is entirely legal: 8192-byte narration and
note text, a 128-byte key, a 256-byte title, a 1×1 grid, and a resource with
`max: 0` (unlimited) and a non-zero current. Loosen any one comparison by a
character and it stops loading. A fixture one byte *under* each limit would
load either way and pin nothing — which is how all seven came to be unpinned.
(The eighth boundary mutant, `p.Y >= GridHeight`, is killed by the y-fixture
fix below. The fixture's placement at (0,0) is a deliberate redundant pin, not
one of the seven: `p.X < 0` and `p.Y < 0` were already killed by
`testdata/valid/scenes/gate.json`.)

**A trap worth recording, because the answer is the opposite of the neighbour
entry.** `compile.go:35` carries five `ARITHMETIC_BASE` mutants on a slice
capacity hint, and `tools/mutation-equivalents.txt` already holds four
adjudications saying capacity hints are unobservable. Those are **maps**. A
negative *slice* capacity **panics**, and the campaign entry says so in its own
caveat. Four of the five are killable and now killed by
`TestCompileHandlesLopsidedAdventureShapes`, which builds adventures where one
term dominates.

The trailing `+1` looked equivalent — it floors at 0, so it never panics — and
was adjudicated as such. **That was wrong, and review caught it.** A slice
differs from a map in TWO ways, not one: a negative cap panics, *and a slice's
capacity is a readable part of its value*. `Compile` returns the slice, so
`cap(got) != len(got)` distinguishes them (14 vs 24 on a 12-scene adventure).
The adjudication's reason — "nothing in this package reads `cap()`" — was a
claim about today's callers, not about whether an observable exists, which is
exactly the "true, and irrelevant" shape `mutation-equivalents.txt`'s header
warns about. The entry is gone and the test asserts `cap == len`, which also
pins what the expression is for: one exactly-sized allocation.

So adjudicating by analogy would have excused four real gaps, and adjudicating
the fifth on a survey of current callers excused a fifth.

**An asymmetry between two sibling fixtures.** `placement-x-out-of-bounds` used
`x=10` on a width-10 grid — exactly on the boundary, pinning `>=`.
`placement-y-out-of-bounds` used `y=-1`, pinning the *lower* bound instead. So
between the pair, `p.Y >= GridHeight` was never tested at all. The y fixture now
mirrors x; the lower bound is pinned by `at-every-boundary`'s (0,0) placement.

**The 4 NOT COVERED were four `must not be empty` rules with no fixture** —
empty placement `token_id` and `actor_id`, empty note `key` and `text`. The
catalogue had `note-key-too-long` but never `note-key-empty`. Since
`check-mutation.py` does not fail on NOT COVERED, nothing in the gate was ever
going to point at them.

**Known gap, deliberately left:** with x and y both now pinned at the inclusive
upper bound, no fixture asserts that a NEGATIVE placement coordinate is
rejected. No gremlins mutator can produce `p.Y < -5`, so the gate will never
report it — the same "nothing was going to point at them" argument as the
NOT COVERED four. Recorded here rather than rediscovered.

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
obligation"), revisited at ADR-010:156-160.

**RETRACTED 2026-08-05: "75 killed, no genuine survivors among the 77
evaluated, so its tests do look strong" was a THIRD instance of the same
artifact.** That figure came from a manual run in a worktree with the directory
renamed so resolution works — which fixed the resolution and left a second,
independent constant-KILLED oracle in place. `cmd/vtt`'s scenario tests boot a
server against `scenarios/testdata/dnd45e-minimal-adventures`, whose only entry
is a SYMLINK, and gremlins' workdir copy drops symlinks silently
(`copyPath`, `workdir.go:142`, switches on `IsDir()`/`IsRegular()`; a symlink is neither and
falls through with no error). In the copy that directory is empty, `vtt serve`
reports "adventures dir ... contains no adventures", and the suite fails under
every mutant. Reproduced directly: `go test ./cmd/vtt/` in a symlink-free copy
fails in **23.7s**, matching the renamed-worktree run's ~22s per mutant.

Nothing is currently known about the strength of `cmd/vtt`'s tests. The honest
statement is that it has never been measured — three times now.

`dropped_symlinks()` guards this, structurally, the same way
`unresolvable_packages()` guards the first route. Its allowlist entry for this
symlink names the consequence, so gating `cmd/vtt` is refused in writing rather
than discovered again.

### `internal/rules` — worked down and gated 2026-08-05

49 survivors → **16, all adjudicated equivalent**, out of 553 mutants (38 not
covered, 1 timed out). The interpreter ADR-002 rests on is now gated. The killed
count is the one figure here NOT worth quoting: two runs a day apart gave 496
and 498, because a mutant that times out in one run is evaluated in the next.
The survivor count did not move.

The sixteen reduce to THREE shapes: assigns-a-value-already-held (9); the
distinguishing input is unreachable (5 — two by short-circuit, one by parity,
one by an earlier return, one because the slice is seeded before the check);
the call has no observable effect (2 — a map capacity hint, and a call with an
empty slice).

Five of the sixteen have a NOT-equivalent sibling on their own line, and each of
those five names it, so nobody generalises from the verdict to the line — that
distinction nearly cost four real gaps in `internal/adventure`. The other eleven
name nothing because there is nothing to name; two point at a related mutant
elsewhere (`expr.go:923` at `:1002`, `compile.go:58` at the contrasting SLICE
hint in `internal/adventure`). Absence of a sibling note is not evidence that
the line is wholly equivalent — read the line.

**Two GAME RULES had no test at all.** `resolve.go:294`'s `hit := total >=
vsTotal` — every hit case cleared the defence outright and every miss fell
short, so equality, the one input where `>=` and `>` disagree, was never
exercised; the mutant turns every tie in the game into a miss. And
`chebyshevDistance`'s `dy` arm: seventeen of eighteen range tests placed both
tokens on y=0, so negating a positive dy makes a target two squares north read
as distance −2 and reach becomes unlimited along one axis.

**The dice bounds are checked in THREE places** and a bare `NdM` literal reaches
only one. The lexer folds `0d6` into a single token (`:861`/`:864`); the parser
re-checks for separate nodes (`:1036`/`:1065`); a third handles a fused
`d`+digits token after a separate count (`:1080`). `1d1`, `1d(1)` and `(1)d1`
take three different code paths and only the first had a test. A fourth check,
in `Eval`, sees the RUNTIME value and is reachable only through `1d(@faces)`.

**`int32Checked`'s range was only ever tested from outside.** Every existing
case drove a value past the bound; a delta of exactly ±2147483647/8 — legal,
and the widest a resource change can be — would have been refused.

**Errors that mislead rather than fail:** `refDisplay` picks `@` vs `#` and
whether to show the scope (swapped, it names `#caster.vim` for `@caster.vim`);
`describeTok` swapped reports end-of-input as a token and vice versa; and
`compile.go:227` suggests the FIX — `write the "0 - 3" idiom` — where the
inverted mutant advises `"0 - -3"`, invalid in a grammar with no unary minus.

Worked in slices, for the record. The first was expr.go's dice bounds, depth
guards and identifier charset — eleven killed, each fault-injection proven.

**A method note worth keeping.** An early injection run reported all eight dice
mutants surviving, and that was an artifact: the injections ran against ONE
test (`go test -run ...`) while a mutant survives only if the WHOLE SUITE
passes under it. Four of the eight were already dead to a `TestParseDiceBounds`
900 lines up the same file. Run the suite, not the test — a narrow injection
overstates the gap.

**The equivalences this section used to hold in escrow now live in
`tools/mutation-equivalents.txt`**, where the gate reads them. They were staged
here first because an entry with no matching survivor is a STALE ENTRY and fails
the gate, so an adjudication cannot exist before its package is in `PACKAGES` —
the two have to land in one change, and they did. Do not re-add adjudication
text here: one of them would be edited and the other would not.

**`internal/rules` is the one that matters most.** It is the rules interpreter
— `compile`, `expr`, `resolve`, `load`, `schema`, `format`, `crypto_roller`,
4,224 lines — which is what ADR-002 ("rules as declarative data") rests on. Its
38 NOT COVERED mutants are consistent with its 89.0% coverage floor and break
no gate, but "10% of the rules interpreter is unreached by any test" is the
legible form of that number, and a coverage percentage is exactly what ADR-010
says cannot answer the question mutation answers.

## NOT COVERED is measured here but not enforced anywhere

**39** mutants across the table above sit in code no test reaches (32 + 7 + 0),
and a further **38** sit inside a GATED package, `internal/rules` — gating a
package drives its survivors to zero, not its unreached code.
`check-mutation.py` has no regex for `NOT COVERED` and no check on it, so the
gate passes with all 77 present.

(This total has been wrong once already, by carrying rows that had since been
gated. Re-add from the table rather than editing the sum.)

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
`internal/campaign` 2 both times, `internal/mcp` 1 then 3. `internal/rules`
joined them on gating with 1 (`INCREMENT_DECREMENT at expr.go:796:8`), so its
gate line prints a TIMED OUT count too; that is expected, not a regression. Do
not quote a
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

Smallest first. `internal/rules/conformance` (10) and `internal/adventure`
(14) are done; `internal/rules` (49) is what remains.

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
