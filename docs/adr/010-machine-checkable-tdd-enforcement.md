# ADR-010: Machine-checkable enforcement for ADR-009

**Status:** Accepted (Patrik, 2026-07-28)

**Context:** ADR-009 §5 is titled *"Enforcement is procedural, not
honor-system"*, and lists three mechanisms: every implementation brief carries
the TDD mandate, every task reviewer verifies RED/GREEN or fault-injection
evidence, and a task without that evidence is Needs-fixes by definition.

Every one of those is a human procedure. Measured against
[ckeletin-go](https://github.com/peiman/ckeletin-go) — the standard this repo
adopted, whose claims are *"every rule is machine-checkable"* and *"no honor
system"* — ADR-009 was the one major rule with nothing checking it. ADR-004 has
go-arch-lint, ADR-007 has buf breaking and the drift gate. ADR-009 had prose.

It had already eroded, twice, in ways that were recorded at the time:

1. **The at-cap boundary gap.** A P11 review found it, classified it "low", and
   ledgered it as a carry-forward: *"multibyte/at-cap boundary tests"*. One of
   six instances was fixed as a merge-gate MUST-FIX; that fix's own comment
   then asserted the siblings were *"already covered implicitly via their own
   accept tests"*, which was false. It stayed false for two sub-projects.
   A mutation audit found all five survivors in 36 seconds.
2. **The fault-injection protocol.** ADR-009 §3 requires injections; the ban on
   running them against the real tree broke twice, both incidents ledgered, and
   the response both times was more prose ("a standing template line").

Principle 9 predicts exactly this: rules that aren't enforced erode. The
coverage gate (merged b1306f3) closed the "are the lines even run" half.
Coverage cannot answer the half ADR-009 actually cares about — whether the
tests *assert* anything. A suite with no assertions reaches 100% coverage.

**Decision:**

1. **Mutation testing is the machine-checkable form of ADR-009 §3.** gremlins
   attempts every mutation it can make and reports which survive; that is
   exhaustive fault injection, done by a tool rather than by whoever remembered
   to do it. `task check:mutation` is the gate.

2. **The standard is zero UNADJUDICATED survivors**, not an efficacy
   percentage. Efficacy is a ratio, so enough newly-killed mutants mask a new
   survivor: `internal/engine` at 49 killed / 2 lived is 96.08%, and 100 killed
   / 3 lived is 97.1% — a real new gap passing a 96.0 floor. Every survivor
   must be named in `tools/mutation-equivalents.txt` with a stated reason, or
   the gate fails. This restores the 2026-07-24 audit's own wording.

   Deliberately not gremlins' built-in `--threshold-efficacy`, despite it being
   the toolchain-native option and despite E5's lesson that hand-rolled
   enforcement is where this project's defects concentrate. The precision was
   judged worth ~90 lines of parsing (Patrik, 2026-07-28). That judgement was
   made when the gate was still planned for the merge gate; it stands under
   decision 5's placement inside `task check`, since the argument was never
   about cost — a masked survivor is the exact failure this gate exists to
   prevent.

3. **A stale adjudication is also a failure.** An entry whose mutant no longer
   survives must be removed, because it silently pre-approves a future survivor
   at that location.

4. **An equivalence claim requires a stated observable.** A bare entry is a
   parse error. This is not ceremony: four mutants were adjudicated equivalent
   on 2026-07-27 and **two were wrong**, reasoning from JSON `omitempty` when
   the observable was `reflect.DeepEqual` on decoded structs, where a nil map
   and an empty map differ. A two-line fixture killed both. A wrong
   "equivalent" verdict is worse than an unadjudicated survivor — it converts a
   real gap into a permanently excused one, in writing.

5. **It runs inside `task check`.** Patrik's ruling, 2026-07-28: *"Time is
   not an issue, quality is. So do not count seconds to achieve quality."*

   An earlier draft of this ADR placed the gate at the merge gate instead, and
   justified it purely on runtime — ~110s against check's ~205s. That reasoning
   is void, and its consequences were larger than the placement: it required a
   tracked `.githooks/` directory, a `pre-merge-commit` hook, a `pre-commit`
   companion (because pre-merge-commit alone is bypassable — see below), a
   `check:hooks` task to verify `core.hooksPath`, a `setup:hooks` task to set
   it, and a `VTT_MUTATION_SKIP` escape hatch. All of that existed to make a
   MERGE-TIME gate unskippable. A gate that runs on every check needs none of
   it, and it was deleted.

   Worth keeping from that detour, because it will apply to any future
   merge-time hook here: **`pre-merge-commit` alone is bypassable, and git
   hands you the bypass unprompted.** When it fails, git stops with the merge
   staged and prints *"Not committing merge; use 'git commit' to complete the
   merge."* Following that instruction runs an ordinary commit, for which git
   does not run `pre-merge-commit`, and the refused merge lands. Verified —
   merge blocked with HEAD unmoved, then `git commit --no-edit` exit 0 and HEAD
   moves. Any merge-time hook needs a `pre-commit` companion guarded on
   `MERGE_HEAD`.

6. **`internal/harness` is the one exclusion, on DESIGN grounds.** Its fixed
   sleeps cost ~70s *per mutant* — gremlins reruns the package suite once per
   mutant — so a run is hours. Under the ruling above that is not a reason to
   skip it for speed; it is a reason the ledgered fake-clock work stops being a
   nice-to-have and becomes the thing blocking a package from being verifiable
   at all. When it lands, harness joins the gated set. `cmd/vtt` is unmeasured
   for the same reason (~40s per test run) and carries the same obligation.

   **AMENDED 2026-07-29 — the precondition is met, the conclusion was wrong.**
   The fake-clock work landed: the harness tests now run inside
   `testing/synctest` bubbles, where the clock advances only once every
   goroutine is durably blocked. The package suite went 51s → 0.7s and a full
   mutation run is **3m38s**, so the runtime argument above is void.
   `internal/harness` still does NOT join the gated set, for a different and
   more uncomfortable reason: that run reports **29 survivors and 32 uncovered
   mutants** (220 total, at `check:mutation`'s own timeout coefficient of 30).
   Re-measure before acting on the split — the Lived/Not-covered boundary has
   been seen to shift by one between runs and trees; the total does not.

   **Progress, 2026-07-30: 29 -> 7 survivors**, 180 killed, efficacy 84.5% ->
   96.3%, across four passes. Three findings outlast the count:

   - **A loose bound is not a test.** The checkpoint test asserted
     `Checkpoints >= 2`; the mutant produced 115 instead of 3 and passed. A
     soak doing 40x the intended verification work, reporting success.
   - **Timing-only mutations survive buffer assertions.** The progress-print
     mutant changes only WHEN a line is emitted; the end-of-run sweep leaves
     the final buffer byte-identical. It had to be caught by sampling the log
     mid-run from inside the fake's send.
   - **Rejection tests do not pin acceptance.** `validateProbe`'s counters
     survived because the only probe test loaded an AMBIGUOUS probe and
     asserted it errors — which a mutated counter also does. A validator that
     rejects everything passes every rejection test.

   **`waitFor(name, 0, timeout)` does NOT return immediately** — it scans for
   an envelope with `Sequence == 0`, and sequences start at 1, so it spins to
   the deadline and returns false. That kills the easy assumption that
   soak.go's `lastAcceptedSeq > 0` / `waitForSeq > 0` guards are equivalent
   because "waiting for 0 is free". They are NOT free.

   They are nevertheless **probably equivalent, for a different reason**:
   `planDeniedAttempt` returns ok=false while `m.tokenIDs` is empty, and
   tokens are only appended when a `placeToken` is ACCEPTED — so a denial is
   structurally impossible until at least one action has been accepted, and
   `lastAcceptedSeq` is provably >= 1 wherever those guards are evaluated. The
   reasoning is a REACHABILITY argument, which is more fragile than the
   value-identity arguments elsewhere in mutation-equivalents.txt: it breaks
   the day a denial-able action exists that does not require a token. Record
   that dependency with any entry.

   **Adjudication is blocked on gating, not the other way round.**
   check-mutation.py computes stale entries as `set(equivalents) - claimed`,
   and only packages in PACKAGES are ever claimed — so an equivalents entry
   for `internal/harness` while harness is excluded is reported as stale and
   REDS the gate. The remaining survivors must therefore be killed or
   adjudicated and harness added to PACKAGES in one change.

   Remaining 7: `engine.go:737` (batch mismatch detail — needs a
   one-participant-short batch fixture), `soak.go:271` (2) and `soak.go:401`
   (the reachability argument above), `soak.go:611` (`time.Sleep(5ms)` poll
   interval), `soak.go:691` (`canPlaceToken` eligibility), `soak.go:736` (RNG
   coin flip).

   A caution earned twice on this branch: the first test written for the
   progress-print mutant did NOT kill it, because the mutation changes only
   WHEN a line is emitted and the end-of-run sweep makes the final buffer
   identical. Verify every new test red-against-the-mutant in a throwaway
   copy. "The test passes" has never been evidence that it tests anything. The sleeps were never the only thing hiding this — they were the
   reason nobody had looked. Gating the package is now a matter of killing or
   adjudicating those 29, tracked as its own task; the exclusion is a recorded
   test-coverage debt, not a performance concession.

   Worth noting for `cmd/vtt`, which this ADR bundles under the same excuse:
   the "~40s per test run" above has never held up. It measured 32s earlier in
   this effort and **21.3s** now (`go test -count=1 -p 1 ./cmd/vtt/...`). Do
   not inherit either number without re-measuring — that applies to the 32s
   this sentence used to quote, too.

   Every other package is gated. The list was ONCE the five that happened to
   have been measured while this gate was written, which silently dropped
   `internal/campaign` — a package the older `audit:mutation` task did cover,
   and which owns the fold driver, undo, and the poison contract. Gating it
   found three real survivors in `c.head`'s batch arithmetic within a minute.
   A gate scoped to what its author touched is not a gate.

7. **A timed-out mutant counts as a detection, and a majority of timeouts
   fails the run** (Patrik, 2026-07-28). gremlins abandons a mutant whose
   suite runs too long; in this codebase those are overwhelmingly mutations
   that hang a socket or a wait, which the suite did notice.

   **They are not reliably detections, and the record should say so.** A
   review disproved the general claim with a working case: a lenient assertion
   over a value whose upper bound the mutation removes (`if d > ceiling { d =
   ceiling }`, then sleep) is reported TIMED OUT, yet applying that mutation
   by hand the suite PASSES. The mutant genuinely survives and this gate scores
   it detected. So every timed-out mutant is PRINTED with its location — they
   are not failures, but they are the set the run did not measure, and they
   must not vanish between gremlins' output and the gate's report.

   The cap guards the inverse, which was observed here first: two
   goroutine-waiting tests took `internal/mcp` from 57 killed / 8 lived / 0
   timed out to 6 / 0 / 58, while gremlins still printed "Test efficacy:
   100.00%" — it computes killed/(killed+lived) and ignores timeouts entirely.
   Measured proportions sit two orders apart (mcp broken 91%, gateway 18%, mcp
   fixed 0%), so a MAJORITY fails as an unusable measurement. Exactly 50% is
   accepted, pinned by test rather than left to the reader.

8. **gremlins is pinned** as a `go tool` dependency at v0.6.0, the same
   mechanism `buf` and `protoc-gen-go` use. A gate cannot depend on
   `go install ...@latest`: an unpinned tool means the gate's behavior is not
   reproducible, and its output format is the thing being parsed.

**Consequences:**

`task check` gains roughly 20 minutes (the eight gated packages, of which
`internal/mcp` alone is ~857s) and a new class of failure: a change that adds
behavior without tests now fails the gate rather than reaching review. That is
the point, and per the ruling above the runtime is not an argument against it.

**This does not make ADR-009 fully machine-checked, and it should not be
claimed as such.** §1 (no impl-then-test) and §2 (behavioral RED) are about the
*order* work happened in, which no artifact in the finished tree records — a
mutation-clean package looks identical whether its tests were written first or
last. §3 (fault-injection evidence) is what mutation replaces, and §4 (tests
pin boundaries, not internals) remains a reviewer judgment. The honest
statement is that the *outcome* ADR-009 §3 exists to guarantee is now enforced;
the *process* rules are still procedural.

**References:** `tools/check-mutation.py`, `tools/mutation-equivalents.txt`,
`tools/check_mutation_test.py` (19 boundary tests — the gate is itself gated),
`docs/superpowers/reports/2026-07-27-enforcement-layer.md`.
