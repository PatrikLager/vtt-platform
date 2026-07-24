# ADR-009: Airtight TDD as binding development protocol

**Status:** Accepted (Patrik, 2026-07-24)
**Context:** The platform's tests are primarily a REGRESSION NET for the
post-prototype phase: once real play exposes design misjudgments, changes will
ripple in unforeseen ways across the event core, gateway, and rule modules.
The net only works if (a) every test has been proven able to fail, and (b)
tests pin behavior at boundaries, so intended changes don't cry wolf.
Process history: TDD was brief-level policy through sub-projects 1–3 and
eroded exactly once (gateway server task went impl-then-test; that same task
was also where the final review found two Important lifecycle bugs).
Principle 9: unenforced rules erode.

**Decision — binding for every implementation task from sub-project 4 on:**

1. **No impl-then-test, ever.** Tests are written and run RED before the
   implementation exists — even when interfaces are fully pre-specified.
2. **Behavioral RED where feasible.** Where a package/function already
   compiles (or can be stubbed to compile returning zero values), the RED
   must show the ASSERTIONS failing — not merely a missing-symbol compile
   error. Compile-failure RED is acceptable only for brand-new packages, and
   reviewers treat it as weak evidence.
3. **After-the-fact tests require fault-injection proof.** Keystone,
   scenario, and characterization tests (written over already-built code)
   have no natural red phase; each load-bearing assertion must be proven
   able to fail by temporarily re-breaking the code (injection → observe the
   specific failure → restore → green), with both transcripts in the task
   report.
4. **Tests pin boundary behavior, not internals.** Wire frames, authz
   outcomes, state derivation, log round-trips, gate exit codes — never
   private structure that an intended refactor would legitimately change.
   A reviewer finding an internals-coupled test reports it as a defect.
5. **Enforcement is procedural, not honor-system:** every implementation
   brief carries the TDD mandate; every task reviewer verifies RED/GREEN (or
   fault-injection) evidence and re-derives it when in doubt; a task without
   the evidence is Needs-fixes by definition. These rules live in CLAUDE.md
   so every agent session inherits them.

**Consequences:** Slightly slower first-drafts; materially stronger net.
Bug-fix waves already follow this shape (failing regression test showing the
bug's symptom precedes every fix) — this ADR makes the same rigor universal.
