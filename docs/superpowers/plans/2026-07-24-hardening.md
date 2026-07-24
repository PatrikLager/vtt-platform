# Hardening Pass Implementation Plan (sub-project 3.5)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Execute the hardening spec (`docs/superpowers/specs/2026-07-24-hardening-design.md`): session_id stamping, machine mutation audit + gateway hand-injection, and known-gap closure — the first plan fully under ADR-009.

**Architecture:** No new packages. Campaign gains session-id stamping under its existing lock; store gains a one-line Notify guard; cmd/vtt's serve composition extracts to a testable helper; everything else is tests and audit evidence.

**Tech Stack:** existing; plus `gremlins` (installed at `$(go env GOPATH)/bin/gremlins`, validated on this repo).

## Global Constraints

- Branch `feat/hardening` from `main`. Review-before-commit flow; controller stages post-report and commits with `CLAUDE_REVIEW_DONE=1`.
- **ADR-009 is binding on every task:** tests first with behavioral RED wherever a stub can compile; fault-injection transcripts for after-the-fact assertions; boundary behavior only. Reviewers reject tasks lacking the evidence.
- Re-review dispatches restate: reviews are READ-ONLY — no stash, no checkout, no tree mutation; fault injection happens in throwaway copies only (rsync to scratch, never the repo tree).
- No contract changes at all in this plan (`check:breaking`/`check:drift` must stay trivially green).
- Property-count re-pin, if needed, is a DELIBERATE documented step, never a silent update.

---

### Task 1: session_id stamping (TDD, behavioral RED)

**Files:**
- Modify: `internal/campaign/campaign.go` (stamp logic; Undo loses `sessionID` param)
- Modify: all Undo callers (mechanical); `internal/gateway/server.go` (drop the `""` arg)
- Create: `internal/campaign/session_stamp_test.go`
- Modify: `internal/campaign/property_test.go` ONLY if counts shift (see Step 4)

**Interfaces:**
- Produces: `campaign.Append` stamps `env.SessionId` under `c.mu` before validation/persist: SessionStarted → fresh generated session id (crypto/rand hex, `sess-` prefix); all other events + Undo markers → currently-open session's id; no open session → left as-is (caller-supplied or empty). Incoming NON-EMPTY session_id on non-SessionStarted events is OVERWRITTEN by the open session's id (campaign is authoritative). `Undo(from, to int64, reason, eventID, actorRole, participantID string)` — sessionID param GONE.

- [ ] **Step 1: Behavioral RED.** Write `session_stamp_test.go` (package campaign_test) against CURRENT code — these tests compile and FAIL behaviorally (the strongest RED): (a) append SessionStarted with empty session_id → returned/broadcast envelope has non-empty `sess-`-prefixed id AND `State().Sessions[0].ID` equals it; (b) subsequent SceneCreated/TokenMoved envelopes carry that same session id; (c) after SessionEnded + new SessionStarted, later events carry the SECOND id; (d) event appended with NO open session keeps its incoming session_id verbatim; (e) a non-SessionStarted event submitted with a WRONG non-empty session_id gets overwritten to the open session's; (f) Undo marker carries the open session's id. Run: `go test ./internal/campaign/ -run SessionStamp` — capture the behavioral failures (empty ids where non-empty expected).
- [ ] **Step 2: Implement** in `campaign.Append` (inside the lock, before snapshot-validation so validation sees the final envelope) + marker construction in `Undo`; drop Undo's sessionID param; update all callers (grep `.Undo(` — 13 sites at last count, now passing one fewer arg).
- [ ] **Step 3: GREEN** + full `go test ./internal/... -race -count=1`.
- [ ] **Step 4: Property counts.** Run the property test; if counts shifted, STOP and reason: the generator submits session ids explicitly — stamping overwrite may alter statesEqual inputs. If a shift is legitimate (ids now campaign-generated), re-pin counts in one commented change documenting why. If anything else shifted, that is a regression — investigate.
- [ ] **Step 5: Gateway scenario expectations** — scenario asserts session ids? If it asserted empty/echoed values anywhere, update to assert the STAMPED behavior (non-empty, consistent across a session's events). `task check` green.
- [ ] **Step 6: Commit point** — `feat: campaign stamps session_id under lock (merge-gate decision)`

---

### Task 2: Known-gap closure (tests + two one-line guards)

**Files:**
- Create/modify: `internal/campaign/undo_test.go` (partial-overlap case), `contract/testdata/server_frame_error.json` + both round-trip suites, `tools/toolgen/main_test.go` (ListValue IsList test), `internal/store/{subscribe.go,subscribe_test.go}` (Notify seq-0 guard + test), `cmd/vtt/serve.go` + `cmd/vtt/serve_compose.go` + `cmd/vtt/serve_e2e_test.go`

**Steps (each gap = RED-first where code changes; fault-injection where test-only):**
- [ ] **(a) Partial-overlap retraction:** test first: retract [n,n], then attempt [n-1,n+1] → rejected (already-retracted overlap). Passes against current code? Then per ADR-009 rule 3 it needs INJECTION PROOF: temporarily weaken the `already[seq]` check (skip the map consult), confirm the new test FAILS, restore, green. Transcripts captured.
- [ ] **(b) Non-empty error fixture:** `server_frame_error.json` (ServerFrame→CommandResult with ok=false, error="actor not found", no sequence key — protojson drops zero int64? sequence:"0" is dropped as zero value — omit it). Round-trip in Go + TS suites.
- [ ] **(c) toolgen IsList:** unit test calling the schema builder on `(&structpb.ListValue{}).ProtoReflect().Descriptor()` — asserts the repeated `values` field yields `{"type":"array","items":{...}}` and the Struct special-case fires for nested Struct values. No contract change.
- [ ] **(d) Notify seq-0 guard:** RED first — test calls `s.Notify(&vttv1.Envelope{Sequence: 0, ...})` expecting a panic-free NO-OP + (new behavior) a returned error or silent drop? DESIGN (binding): Notify silently ignores sequence==0 with a doc-comment (public-method hardening; no error return — signature stability). Test: subscriber receives nothing; then a normal stamped envelope still delivers. RED against current code = the zero-seq envelope IS delivered (assert absence fails). Implement guard, GREEN.
- [ ] **(e) serve composition e2e:** extract `composeServer(campaignPath, addr) (*http.Server, func() error, error)` into `serve_compose.go` (RunE calls it, stays ≤30 lines). E2E test: compose on a temp campaign + random port, start in goroutine, healthz 200, mint an invite via identity directly, WS connect + StartSession command round-trip, graceful Shutdown, assert clean exit. RED: the test is written against the NOT-YET-EXTRACTED helper (compile RED acceptable here — new symbol) but its assertions must be proven by one injection after green (e.g. break the identity wiring in the helper, connect fails with 401, restore).
- [ ] **Commit point** — `test: close ledgered gaps; Notify guard; serve composition e2e`

---

### Task 3: Mutation audit — machine (4 packages) + hand (gateway)

**Files:**
- Create: `docs/superpowers/reports/2026-07-24-mutation-audit.md` (the evidence artifact, committed)
- Create/modify: any test files the audit demands (each addition RED-proven by its killing mutant — the mutant IS the injection)
- Modify: `Taskfile.yml` (optional `audit:mutation` convenience target, NOT in `check` — mutation runs are minutes-long, not gate material)

**Steps:**
- [ ] **Step 1: Baseline runs.** `gremlins unleash ./internal/engine/`, then store, campaign, identity (one at a time; campaign's property test may make runs slow — if a package exceeds ~15 min, rerun with `--tags` or exclude the property test via gremlins config if supported, DOCUMENT any exclusion). Record per-package: killed/survived/not-covered.
- [ ] **Step 2: Survivor triage.** Every survivor: (i) write the test that kills it (the mutant is a ready-made injection proof — run gremlins again or hand-apply the mutation to confirm the new test fails under it), or (ii) document as accepted equivalent mutant with reasoning (e.g. defensive branch provably unreachable). No third category.
- [ ] **Step 3: Gateway hand-injection list** — ten assertions, each: name the injection (one-line code break), apply in a THROWAWAY rsync copy, run the specific test, confirm the targeted failure message, delete copy. The ten: (1) authz table cell flip (player gains create_scene) → 28-cell test fails; (2) ownership check disabled → denial test fails; (3) Verify bypassed (always-ok) → 401 tests fail; (4) catch-up skips first event → history-order test fails; (5) overflow force-close disabled → internal regression test fails via its tightened Fatalf; (6) backfill dropped → From-assertion fails; (7) attribution hardcoded → ActorRole assertion fails; (8) marker broadcast suppressed → retract-to-all test fails; (9) malformed-frame handling closes ALL conns → isolation test fails; (10) result sequence zeroed → sequence-match test fails. Transcript per item in the report.
- [ ] **Step 4: Re-run everything** — full `task check`, `go test ./... -race -count=1`, final mutation re-run on any package that gained tests (survivors now killed). Report finalized with the numbers.
- [ ] **Commit point** — `test: mutation audit — survivors killed or adjudicated; gateway injection proofs`

---

### Task 4: Final review + merge prep

- [ ] Standard final whole-branch review (fable): audit-report spot-checks (re-run one gremlins package, re-run two hand injections), spec-conformance sweep, Minors triage.
- [ ] Merge gate to Patrik with the audit numbers as the headline.

## After this plan

Sub-project 4 (simulation harness) begins with a proven-toothy net. Mutation audit becomes a periodic practice (post-merge of each sub-project, not per-commit — documented in the report's closing section).
