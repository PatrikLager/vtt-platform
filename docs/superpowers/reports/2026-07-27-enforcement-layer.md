# Enforcement layer — ADR-009 evidence report

**Date:** 2026-07-27
**Scope:** Plan 14 (E1 coverage raise, E2 `check:coverage` gate)
**Status:** evidence complete; zero survivors outstanding
**Revision:** corrected 2026-07-27 after a second review — see "Correction" below
**Revision 2:** after a fourth review, the hand-rolled profile merge was replaced
with `go tool covdata merge` (see "Shape" at the end)

## Why this report exists

Every test in this change set is written over already-built code. ADR-009 §3
requires fault-injection proof per load-bearing assertion for after-the-fact
tests, and §5 makes a task without it "Needs-fixes by definition."

The first version of this work shipped without a report. A review caught that
— and the omission was not incidental: this is the change set whose entire
purpose is to end honor-system ADR-009 compliance, and it was itself relying
on the honor system. It also caught a concrete consequence (finding #3 below),
which is exactly what the protocol exists to surface.

Two forms of evidence appear here. Where a package's suite runs fast enough,
**exhaustive mutation** (gremlins) is used — strictly stronger than hand-picked
injection, since it tries every mutation the tool can make rather than the ones
the author thought of. Where the suite is too slow for that (`cmd/vtt` at ~40s
per run, the same class as `internal/harness`), **targeted injection** against
the named assertion is used instead.

All injections were performed in throwaway copies of the tree, never by
editing and reverting the real tree — the standing rule after two ledgered
incidents.

A note on how, because the first attempt was wrong: `git ls-files | tar` copies
only TRACKED files, so a throwaway built that way silently omits new
uncommitted work and any result from it is meaningless. One M1 injection was
run that way and produced a misleading pass before the mistake was caught. Use
`rsync -a --exclude .git --exclude node_modules ./ <dir>/` while the change set
is uncommitted.

---

## Exhaustive evidence (mutation)

| Package | Killed | Lived | Efficacy | Runtime |
|---|---|---|---|---|
| `internal/identity` | 17 | 0 | 100% | 16s |
| `internal/store` | 31 | 0 | 100% | 33s |
| `internal/engine` | 49 | 2 | 96.08% | 36s |
| `tools/toolgen` | 9 | 0 | 100% | 3s |
| `internal/adventure/conformance` | **23** | **0** | **100%** | 19s |

### Correction (2026-07-27, second review)

The first version of this report adjudicated **four** survivors as equivalent.
Two of them were not, and the error mattered: an "equivalent" verdict says no
test can ever kill a mutant, which forecloses writing one. A wrong equivalence
call is worse than an unadjudicated survivor.

**`conformance.go:319:23` and `:325:22`** — `len(a.Attributes) > 0` and
`len(a.Resources) > 0` → `>=`, creating an empty non-nil map instead of leaving
it nil. I reasoned from `json:"...,omitempty"` (`conformance.go:249-250`): nil
and empty marshal identically, so no observable difference. **The premise was
true and irrelevant.** JSON is not the observable — `checkCompiledBatchGolden`
compares with `reflect.DeepEqual` on *decoded structs* (`conformance.go:171`),
and `reflect.DeepEqual(map(nil), map{})` is `false`.

They survived only because all three fixtures gave every actor both maps. The
fix is a two-line fixture: `testdata/adventures/ok/actors/wraith.json`, an
actor with neither, plus its hand-derived golden entry. Both mutants now die —
the package went 21/2 to **23/0, 100% efficacy**.

### Survivors, adjudicated

Two remain, both in `internal/engine`, both genuinely equivalent: the mutated
form assigns a scalar equal to the value already held, so no test can
distinguish it.

1. **`internal/engine/apply.go:141:15`** — `computed < 0` → `<=`. When
   `computed` is 0 the branch assigns 0.
2. **`internal/engine/apply.go:144:30`** — `computed > int64(res.Max)` → `>=`.
   When `computed == Max` the branch assigns `Max`.
   (The sibling `res.Max > 0` on the same line IS killed — `apply_test.go:576`
   pins "max == 0 means unlimited".)

Recorded in `apply_boundary_test.go:26-30` so nobody later "fixes" them with a
test that cannot fail. The distinction against the two corrected above: these
assign an *equal* value; those assigned a *different* one (empty non-nil map ≠
nil).

### What mutation found that review had not

Five `CONDITIONALS_BOUNDARY` mutants survived in `internal/engine` before this
work — every one an at-cap boundary (`apply.go:174`, `:189`, `:202`, `:205`,
`:208`). All five are now killed by `apply_boundary_test.go`; the reviewer
independently re-injected all five and confirmed each FAILS with the file
present and SURVIVES with it removed.

`:189` (`AnchorFromSeq > AnchorToSeq` → `>=`) is the substantive one: it
rejects narration anchored to a single event, the ordinary "this describes
*that* event" case.

Provenance matters here. P11 Task 2's review classified this gap "low", P11
Task 4 ledgered it as carry-forward *"multibyte/at-cap boundary tests"*, one
instance was fixed as a merge-gate MUST-FIX, and that fix's own doc comment
then asserted the siblings *"already follow implicitly via their own accept
tests"* — false, and unchecked for two sub-projects. The comment is corrected
at `apply_test.go:539-549`.

---

## Targeted evidence (suites too slow for exhaustive mutation)

### `internal/mcp/redial_internal_test.go`

**Assertion:** redial's backoff select wakes on `ctx.Done()` rather than
sleeping to the next loop iteration.

**Injection:** deleted `case <-ctx.Done(): return false` from `server.go:304-305`.

| | Result |
|---|---|
| Baseline | PASS (0.10s) |
| Injected | **FAIL** — *"redial took 102.378958ms to notice cancellation (limit 50ms, one backoff is 100ms)"* |

**This assertion previously did not discriminate.** The original test asserted
only "returns false", which is true on both paths: with the arm deleted, the
top-of-loop `ctx.Err()` guard returns false as soon as the 100ms backoff
elapses. The test passed against the code it claimed to cover. Elapsed time is
the only discriminator, and the test now asserts it from both sides — under
half a backoff period, and not before cancellation fires (which would mean the
entry guard caught an already-cancelled context and the backoff path was never
exercised at all).

### `internal/mcp/tools_internal_test.go`

**Assertion:** `buildDispatch` reports oneof fields with no `tools.json` entry
— the direction that catches the contract gaining a command the embedded
manifest never learned about, leaving it silently unreachable.

**Injection:** deleted the `missingTool` accumulation loop in `tools.go`.

| | Result |
|---|---|
| Baseline | PASS |
| Injected | **FAIL** — *"error must report both directions, got: ... oneof fields with no tools.json entry []"* |

### `cmd/vtt/dialer_test.go`

**Assertion:** `--server` without `--tokens` is rejected rather than falling
through to self-contained mode. Load-bearing because the fall-through is
silent: an operator running a scenario against what they believe is their live
table would get a throwaway server and a green result.

**Injection:** replaced the `serverURL == "" || tokensPath == ""` guard in
`client_run.go` with `if false`.

| | Result |
|---|---|
| Baseline | PASS |
| Injected | **FAIL** — the error became *"read tokens file: open /tmp/tokens.json: no such file"* instead of the flags-go-together message |

### `internal/adventure/conformance/dump_test.go`

Covered exhaustively above (23/0, after the fixture correction). Two additional claims were verified
directly rather than by injection, because they are claims about the *world*
rather than about a branch:

- `DumpCompiledBatch` really was at 0.0% coverage — measured with the new file
  hidden; the package moves 80.6% → 89.2% with it present.
- Dump output and the committed golden really do differ textually while
  matching semantically (map key ordering: golden stores `vim, vigor, brace`,
  `json.MarshalIndent` emits alphabetical).

### `tools/check_coverage_test.py`

The gate script's own tests are themselves regression evidence: two of them
(`test_misspelled_threshold_key_is_fatal`,
`test_measured_package_without_a_floor_is_fatal`) reproduce defects a review
demonstrated in the first version, where the gate returned **exit 0 for a state
it should have rejected**. Both now exit 1. 19 tests, all passing.

### `tools/check-coverage.py` and its Taskfile wiring

Later reviews found four further ways the gate could pass when it should fail —
three in the second round, the fourth (a floor editable below the minimum) in
the third. All are now pinned by `check_coverage_test.py` (19 tests) and one
by direct injection:

| Defect | Evidence |
|---|---|
| A misspelled/stale threshold key silently dropped that package's floor to 85 | `test_misspelled_threshold_key_is_fatal` |
| A measured package with no floor entry sat outside the expectation set, so nothing noticed if it stopped being measured | `test_measured_package_without_a_floor_is_fatal` |
| A missing or empty thresholds file dropped **all thirteen** floors to 85, silently | `test_missing_thresholds_file_is_fatal`, `test_empty_thresholds_file_is_fatal` |
| A package with NO `_test.go` files emitted no covdata at all under the new pipeline, so it appeared in neither `measured` nor `thresholds` and NEITHER set-equality direction could see it — an untested package shipped with the gate reporting success. Introduced by the covdata rewrite; the older `-coverprofile` listed such a package at 0%. | `test_untested_package_is_fatal`, `test_missing_expected_file_is_fatal`, plus an end-to-end injection: adding an untested `internal/newpkg` now fails `task check:coverage` (exit 201, package named) |
| A floor could be edited BELOW the 85% minimum — `internal/engine 40.0` passed at 97.7% measured, exit 0. The standard the script cites was the one number nothing checked, and this was the single quiet edit that could weaken any floor. Made worse by the missing-floor fatal, whose message quotes a package's sub-85 measurement and tells you to add a line. | `test_floor_below_the_minimum_is_fatal`, `test_floor_exactly_at_the_minimum_is_allowed` |

And the gate-of-the-gate itself: `check:coverage` invoked the test suite as
`python3 ... | tail -3`, whose pipeline exit status is `tail`'s — always 0,
with no `pipefail` — so under `set -e` **a failing gate-test did not abort**.
Injection proof, with a deliberately failing test added to the suite:

| | Result |
|---|---|
| Before (piped) | test failure printed, execution continued, `task check:coverage` exit **0** |
| After (`run_quiet`) | exit **201**, diagnostics printed, gate never reached, no coverage table emitted |

---

## Known gaps

- **`internal/harness`** is excluded from exhaustive mutation: its fixed
  sleeps cost ~70s *per mutant*, so a full run is hours. This is the ledgered
  fake-clock dependency, unchanged by this work.
- **`internal/campaign`, `internal/gateway`, `internal/rules`** were not
  re-audited here — this change set added no tests to them. Their last audit
  is `2026-07-24-mutation-audit.md`, which predates the fold gaining six event
  types. Re-auditing them is E3's business, not this report's.
- **`cmd/vtt` and `internal/mcp`** have targeted rather than exhaustive
  evidence, for runtime reasons stated above.
- **`task check` is not reliably green.** It failed 1 of 2 consecutive full
  runs during review — `TestEventsTailBinaryExitsCleanlyOnSIGINT`, *"subprocess
  produced no output within 5s"* — the ledgered contention flake, now running
  inside `check:coverage`. "Exits 0" is currently a probabilistic statement
  about this suite, not a property of it, and any claim to the contrary in this
  change set should be read with that caveat. The `run_quiet` wrapper did its
  job: the diagnostics were printed rather than swallowed.

## Standing consequence for E3

Per-package mutation runtime is **not** uniform — 3s to 36s for the five
packages measured here, against `internal/harness`'s hours. The Taskfile's
justification for keeping `audit:mutation` out of `check` ("minutes-long") is
true of the aggregate and false of the parts. E3 should gate the fast packages
and record the harness exception with its fake-clock dependency, rather than
leaving all five on a remembered cadence — which is how the at-cap gap above
survived two sub-projects.

---

## Shape (fourth review, acted on)

Asked for a judgment rather than more findings, the reviewer gave one worth
more than the findings were: **converged on correctness, not on shape.**

Across five rounds there were **9 Must-fixes**, and **8 of them were in the
enforcement layer, not in the eight Go test files this change set exists to
add.** Those had one defect total (round-1's non-discriminating redial test).
(An earlier revision said "9 of 10" — an arithmetic error, corrected here.) The machinery built to measure quality produced nearly every
quality problem — including three consecutive rounds of "passes when it should
fail" in code written to prevent exactly that.

The cause was concrete: ~200 lines of Python hand-rolling a coverage-profile
merge — block parser, count-summing union, covermode agreement check — almost
all of it in service of ONE requirement, that cmd/vtt reads 79.5% without
subprocess coverage and 85.2% with it. That difference is 25 statements in
three files.

`go tool covdata merge` does the merge natively. The pipeline is now:

```
go test -cover <pkgs> -args -test.gocoverdir=$D_UNIT     # in-process
VTT_SUBPROCESS_COVERDIR=$D_SUB go test ./cmd/vtt/...     # subprocess
go tool covdata merge -i=$D_UNIT,$D_SUB -o=$D_MERGED     # native
go tool covdata textfmt -i=$D_MERGED -o=$PROFILE
```

Verified equivalent BEFORE deleting anything: the native pipeline reproduces
all thirteen package numbers exactly, cmd/vtt's 85.2% included. The deleted
code is precisely where the round-2 and round-3 parsing findings lived. What
remains in Python is the part Go does not do — the ratchet: tally per package,
compare to floors, enforce set equality in both directions, and enforce the 85%
minimum on the floors themselves.

`go tool covdata percent` was considered and rejected for two reasons, and the
weaker one was recorded first. Its output is not reliably one-package-per-line
— a zero-statement package emits its name with no percentage and the next
package's line runs on behind it — but that is only awkward, and a parser could
skip lines without `coverage:`.

The decisive reason is that **`percent` prints one decimal place, and rounding
UP can lift a package over its floor.** A package genuinely at 88.988% prints
as `89.0%` and would PASS a floor of 89.0 that it actually fails. That is a
pass-when-it-should-fail baked into the output format itself, unrecoverable by
any parser. The textfmt profile is the only interface carrying the statement
counts the ratchet needs, which settles the follow-on question: `parse_profile`
stays.

The general lesson, worth more than this script: **the layer that kept
producing enforcement bugs was the layer the toolchain already implemented.**

And a second one, from what the rewrite broke. Equivalence was verified before
deleting anything — the native pipeline reproduced all thirteen package numbers
exactly — and that verification **could not have caught the regression it
introduced**, because it compared both pipelines on a tree where every package
already had tests. `go test -cover -args -test.gocoverdir` emits nothing for a
package with no test binary, where `-coverprofile` listed it at 0%. Checking a
rewrite on the happy path proves the happy path. The cases worth testing are
the ones the OLD code handled and the new one might not.
