# Simulation Harness Implementation Plan (sub-project 4)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the harness per the approved spec (`docs/superpowers/specs/2026-07-24-simulation-harness-design.md`): wire-only client core, JSON scenario engine, the four CLI modes, the committed scenario library, and the wire-level soak — with the arch-lint rule that makes the harness the permanent P1 proof.

**Architecture:** `internal/harness` (wire client + scenario engine + local folder; imports ONLY contract + engine + coder/websocket) and thin `cmd/vtt` subcommands carrying the self-contained boot glue. The harness deliberately does NOT import gateway's codec — it speaks protojson over contract types directly, which independently documents the wire protocol.

**Tech Stack:** existing (coder/websocket already pinned; no new deps).

> **AMENDED 2026-08-31 — every retraction step in this plan describes machinery
> that no longer exists.** Sub-project 13
> (`docs/superpowers/specs/2026-08-30-retraction-leaves-design.md`, Patrik's
> ruling of 2026-08-30) removed retraction from the platform. In this plan that
> falsifies: Task 2's `Fold` signature comment ("SAME two-pass semantics
> (retractedSet from markers)") and its Step 2 fold-parity test; Task 4's Step 1
> description of `three-role-exit.json` and its Step 3 injection proof (ii); and
> Task 5's soak action mix. Corrections are inline at each. The rest of the plan
> — the P1 arch-lint rule, the wire client, the scenario format, the CLI modes,
> the committed library, the soak's fold-equality checkpoint — is intact and
> still describes what runs.

## Global Constraints

- Branch `feat/simulation-harness` from `main`. Review-before-commit flow; controller stages post-report, commits `CLAUDE_REVIEW_DONE=1`. Reviewers READ-ONLY (restated in every dispatch, including re-reviews); injections in throwaway rsync copies only.
- **ADR-009 binding:** new-package tasks scaffold a compiling stub FIRST so RED is behavioral (methods return `errors.New("unimplemented")`), not compile-failure. After-the-fact tests (scenario library) carry injection proofs. Boundary behavior only.
- **The P1 rule is the deliverable:** `.go-arch-lint.yml` gains `harness: { in: internal/harness, mayDependOn: [contract, engine, harness] }`. Any harness import of store/campaign/gateway/identity is a build-breaking defect, including in test files (no excludeFiles for harness — its tests must be wire-only too; in-test fake servers are written with coder/websocket + contract types directly).
- No contract changes; drift/breaking trivially green. Vocabulary gate covers internal/ + cmd/ (scenario JSON under `scenarios/` is content data, not code — not scanned, correct).
- Resolved spec §10 questions (binding): soak mix includes a **5% deliberate authz-denied component** (player attempts another's token) asserting denial + no-broadcast over the wire; `tokens.json` format is `{"participants": {"<name>": "<token>"}}`.

---

### Task 1: internal/harness — wire client core

**Files:**
- Create: `internal/harness/client.go`, `internal/harness/client_test.go`
- Modify: `.go-arch-lint.yml` (harness component, the P1 rule)

**Interfaces (Tasks 2–5 depend on these exact signatures):**
```go
type Client struct{ … }
func Dial(ctx context.Context, wsURL, token string, after int64) (*Client, error)
// SendCommand assigns a fresh request_id, sends, and blocks until the matching
// CommandResult arrives (events keep flowing to Events() meanwhile).
func (c *Client) SendCommand(ctx context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error)
func (c *Client) Events() <-chan *vttv1.Envelope   // closed on disconnect
func (c *Client) Close() error
```
Internals: one reader goroutine demuxes ServerFrame (result → per-request channel by request_id; event → Events buffer 256, overflow = client-side error state). protojson via contract types ONLY (no gateway import — deliberate, documented in the package comment).

- [ ] **Step 1: Compiling stub** — types + methods returning `errors.New("harness: unimplemented")`; arch-lint component added; `task check` green (stub compiles, no tests yet).
- [ ] **Step 2: Behavioral RED.** `client_test.go` (package harness_test) with an in-test FAKE wire server (httptest + coder/websocket + contract types only — scripted: canned ServerFrames per received ClientCommand). Cases: dial with token+after lands in URL query; SendCommand correlates by request_id even when the fake interleaves an event between send and result; two concurrent SendCommands correlate correctly; events stream in order on Events(); server close → Events() closes and SendCommand errors; malformed frame from server → client errors loudly. Run — all fail with "unimplemented" (behavioral RED). Capture.
- [ ] **Step 3: Implement → GREEN** + `-race` on the package.
- [ ] **Step 4: `task check`** (arch gate now enforcing the P1 rule — verify with a bite-proof: temp file importing campaign in a throwaway copy → arch-lint fails naming it; transcript to report).
- [ ] **Step 5: Commit point** — `feat: harness wire client — the P1-proof core`

---

### Task 2: Scenario format + engine + local folder

**Files:**
- Create: `internal/harness/scenario.go` (types + strict loading/validation), `internal/harness/engine.go` (execution), `internal/harness/fold.go` (local folder), + `_test.go` each

**Interfaces:**
```go
type Scenario struct {
  Name         string        `json:"name"`
  Participants []Participant `json:"participants"`
  Steps        []Step        `json:"steps"`
  Probes       []Probe       `json:"probes"`
}
type Participant struct{ Name, Role string; Controls []string }
type Step struct {           // exactly ONE of Command/Reconnect set
  By        string
  Command   json.RawMessage  // protojson ClientCommand (parsed via protojson into vttv1.ClientCommand)
  Expect    *Expect          // required with Command
  Reconnect *ReconnectSpec   // {afterSequence}
}
type Expect struct{ OK bool `json:"ok"`; DeniedContaining string `json:"deniedContaining"` }
type Probe struct{ TokenAt *TokenAtProbe; SessionCount *SessionCountProbe; ActorExists *ActorExistsProbe }
func LoadScenario(path string) (*Scenario, error)  // strict: DisallowUnknownFields; errors name step index + field
type Conn interface {        // satisfied by *Client; fake in engine tests
  SendCommand(ctx context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error)
  Events() <-chan *vttv1.Envelope
  Close() error
}
type Dialer func(name string, after int64) (Conn, error)  // engine redials via this (reconnect steps)
func RunScenario(ctx context.Context, sc *Scenario, dial Dialer, report io.Writer) (*Report, error)
type Report struct{ Steps []StepResult; Probes []ProbeResult; Pass bool }
func Fold(events []*vttv1.Envelope) (*engine.State, error)  // engine.NewState + engine.Apply, skip markers/retracted like the server's fold — SAME two-pass semantics (retractedSet from markers), documented as the published derivation algorithm
```
*CORRECTED 2026-08-31.* `harness.Fold` is SINGLE-pass as of `92f1284`, and its
role as the published derivation algorithm is unchanged. It applies
`engine.Apply` to each envelope once, in order, and skips only
`engine.ErrUnknownVariant` — the forward-compatibility behaviour the server's
own replay gives an unrecognised variant. There is no retracted set to collect,
because nothing skips by sequence number anywhere in the platform, and `59542e1`
then deleted the marker message itself. Step 2's fold-parity test and its
injection proof went with it; the parity property is now carried by the soak's
fold-equality checkpoint and by `internal/harness`'s own suite.
Engine semantics (binding): denied steps assert result ok=false + error contains substring + NO event broadcast observed by ANY participant within a 300ms absence window; ok steps assert result ok + the produced event (matching sequence) observed by ALL participants; `reconnect` closes that participant's Conn, redials via Dialer with `afterSequence`, and asserts the catch-up events equal (event_id order) what that participant saw live before; probes evaluate against `Fold(all events observed by participant 0)`.

- [ ] **Step 1: Compiling stubs → behavioral RED.** Test fixtures in `internal/harness/testdata/`: `valid_minimal.json`, `unknown_field.json`, `missing_expect.json`, `both_command_and_reconnect.json` + engine tests against a scripted FAKE Conn/Dialer (canned results + broadcast events): ok-step pass; ok-step FAIL when fake omits the broadcast; denial pass; denial FAIL when fake broadcasts anyway; reconnect equality pass/fail; probe pass/fail per probe kind; strict-load errors name step index. RED captured (stubs unimplemented), implement, GREEN.
- [ ] **Step 2: Fold parity test** — feed the same envelope sequence (incl. a retraction marker) to `Fold` and compare against expectations derived from the event-core's documented semantics (token reverts). This is after-the-fact w.r.t. engine.Apply → ONE injection proof: in a throwaway, make Fold skip the retractedSet pass, watch the parity test fail.
- [ ] **Step 3: `-race` + task check. Commit point** — `feat: harness scenario engine, strict format, client-side fold`

---

### Task 3: CLI wiring — run / tail / dump

**Files:**
- Create: `cmd/vtt/client_run.go`, `cmd/vtt/events_tail.go`, `cmd/vtt/state_dump.go`, `cmd/vtt/harness_boot.go` (self-contained glue: composeServer + minting → returns URLs+tokens), `cmd/vtt/client_e2e_test.go`

**Interfaces:** `vtt client run <scenario> [--server ws://… --tokens tokens.json] [--json]` (exit 0/1); `vtt events tail --server --token [--after N]`; `vtt state dump --server --token`. All RunE ≤30 lines; boot glue hands the harness ONLY strings.

- [ ] **Step 1: Behavioral RED** — e2e test drives the cobra root in-process: run a minimal inline scenario self-contained (boot glue is new code → stub first, tests fail behaviorally); tokens.json live-mode path tested against a composeServer instance started by the TEST (the test is in cmd package — allowed to compose); `--json` report shape asserted; exit codes asserted. tail/dump: against the same live instance — tail emits N protojson lines then interrupt; dump prints state JSON with the expected token position.
- [ ] **Step 2: Implement → GREEN; `task check`. Commit point** — `feat: vtt client run / events tail / state dump`

---

### Task 4: Scenario library + injection proofs

**Files:**
- Create: `scenarios/three-role-exit.json`, `scenarios/denials.json`, `scenarios/smoke.json`, `internal/harness/library_test.go` (executes every `scenarios/*.json` self-contained via a cmd-free path? NO — library_test needs the boot glue which lives in cmd; put the library runner in `cmd/vtt/library_test.go` instead — cmd may compose; harness stays wire-only)

- [ ] **Step 1: Port the exit scenario to data** — `three-role-exit.json` reproduces internal/gateway/scenario_test.go's flow faithfully: session, scene, two actors (one player-controlled), placements, moves, player denial (with no-broadcast), agent move + retraction (marker to all), spectator denial, end, player reconnect-equality. *(CORRECTED 2026-08-31: the retraction leg is gone. `three-role-exit.json` contains zero occurrences of the word, and so does `denials.json`; both keep every other leg. Step 3's injection proof (ii), "retraction broadcast suppressed", has no operation left to suppress.)* `denials.json`: every DENY cell of the authz table exactly once (player×6 non-move commands + player-other's-token + spectator×7). `smoke.json`: session-scene-actor-place-move-end.
- [ ] **Step 2: Library runner test** (cmd/vtt/library_test.go): globs `scenarios/*.json`, runs each self-contained, asserts Report.Pass — the library now executes inside `task check` forever.
- [ ] **Step 2b: Live-`vtt serve` leg (spec §8 literal):** one test builds the real binary (`go build -o <tmp>/vtt ./cmd/vtt`), starts `vtt serve` as a SUBPROCESS on a temp campaign + random port, mints invites via identity (test-side), runs `three-role-exit.json` against it in live mode (`--server`/`--tokens`), asserts Pass, then kills the process (no graceful-shutdown path exists yet — Process.Kill is the documented teardown; the connection-drain carry-forward owns improving this).
- [ ] **Step 3: ADR-009 injection proofs (after-the-fact tests):** THREE, in throwaway copies: (i) authz table cell flip (player gains create_scene) → `denials.json` run FAILS naming the step; (ii) retraction broadcast suppressed → `three-role-exit.json` FAILS at the marker-to-all step; (iii) catch-up order corrupted (skip first event in gateway serve) → reconnect-equality step FAILS. Transcripts to report.
- [ ] **Step 4: task check green. Commit point** — `feat: committed scenario library, proven able to fail`

---

### Task 5: Soak mode

**Files:**
- Create: `internal/harness/soak.go`, `internal/harness/soak_test.go`, `cmd/vtt/client_soak.go` (+e2e case in client_e2e_test.go)

**Interfaces:** `func RunSoak(ctx, cfg SoakConfig, dial Dialer, report io.Writer) (*SoakReport, error)`; `SoakConfig{Seed int64; Events int; CheckEvery int}` (default CheckEvery 100). Generator: seeded `math/rand`; participants dm + 2 players + agent; action mix modeled on the campaign property test (create scene 5%, add actor 10% — dm/agent issue lifecycle; place 15%; move-own 50%; session churn 5%; retraction 10% by agent choosing a safe TokenMoved seq from its OWN observed events; **5% deliberate authz-denied attempts** (player→other's token) asserting ok=false + no broadcast). *(CORRECTED 2026-08-31: the retraction bucket left in `92f1284` and its 10% went to move-own, so the shipped mix is create scene 5%, add actor 10%, place 15%, **move-own 60%**, session churn 5%, deliberate authz-denied 5%. `pickBucket` in `internal/harness/soak.go` is the table, and its comment records the same reassignment. The soak's scenes are also all-floor now rather than bare grids, because `create_scene` requires complete terrain since `e110e9b`.)* Model tracks controlled actors so players only move their own. Checkpoint every `CheckEvery` accepted events + at end: incremental client-side fold DEEP-EQUALS (statesEqual semantics: proto.Equal for actors) a fresh catch-up fold on a NEW second connection. `vtt client soak --seed S --events M [--server/--tokens]`.

- [ ] **Step 1: Stub → behavioral RED** (fake Conn: generator determinism test — same seed twice → byte-identical command sequence; mix-ratio sanity over 1000 draws; denied-attempt bookkeeping). Implement → GREEN.
- [ ] **Step 2: The real soak e2e** (cmd test): self-contained `--seed 1 --events 500` → Pass, with the checkpoint fold-equality exercised (report shows >0 checkpoints). Record the seed-1 accepted/denied counts in the report AND pin them in the test (deterministic).
- [ ] **Step 3: Injection proof** for the fold-equality (after-the-fact): throwaway copy, corrupt the catch-up fold path (skip one event), soak checkpoint FAILS. Transcript.
- [ ] **Step 4: Full `go test ./... -race -count=1`, task check. Commit point** — `feat: wire-level soak — rebuild==live over the WebSocket`

---

### Task 6: Final review + merge gate

- [ ] Final whole-branch review — mode per Patrik's standing preference: OFFER workflow-level (he opted in for hardening; confirm for this branch at the gate) or single-reviewer fable. Include: post-merge mutation audit run over `internal/harness` (cadence policy), ledger Minors triage, spec-conformance sweep.
- [ ] Merge gate to Patrik with library/soak results as the headline.

## After this plan

Sub-project 5 (module loader & rules interpreter) returns to the game itself. The harness carry-forward for sub-project 6: the LLM's practice loop = MCP tools + this scenario format; `scenarios/` becomes its curriculum.
