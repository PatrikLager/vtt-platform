# Implementation report — the simulation harness (`internal/harness`)

**Plan:** `docs/superpowers/plans/2026-07-24-simulation-harness.md` (sub-project 4)
**Spec:** `docs/superpowers/specs/2026-07-24-simulation-harness-design.md`
**Territory:** `internal/harness/{client.go, engine.go, fold.go, scenario.go, soak.go}` — 1,417 comment lines against 1,843 code lines, 50 comment blocks of ten or more lines (both figures reproduced exactly; the test files are not in scope and add another 1,826 comment lines).
**Written:** 2026-09-19, read-only against a tree at `48e3fe2` with sub-project 14's 17 files uncommitted and untouched.

---

## 1. Vad som byggdes

A headless WebSocket client and two things driven through it: a JSON scenario runner and a seeded soak. `harness.Client` dials the gateway's `/ws` with an invite token and a catch-up cursor, demuxes `ServerFrame` results from broadcast events on one reader goroutine, and correlates results to callers by `request_id`; `harness.RunScenario` executes a declarative file of participants, ordered steps and final-state probes against that client and reports per-step and per-probe verdicts; `harness.RunSoak` generates a long mixed command sequence from a seed and proves, repeatedly, that state folded incrementally from one participant's live stream deep-equals state folded from a fresh second connection's catch-up. The package was built to be structurally incapable of cheating: `.go-arch-lint.yml` gives it `mayDependOn: [contract, engine, harness]`, with no `excludeFiles` entry, so neither its production code nor its tests can reach `store`, `campaign`, `gateway` or `identity` — which is what makes it the permanent machine-checked proof of pillar P1, that a client can drive a live table using nothing but the wire constitution.

---

## 2. Hur det fungerar i dag

Verified against the tree, by command, on 2026-09-19.

**It is not a test package.** Despite the name, `internal/harness` ships in the binary. Its production consumers are `cmd/vtt/client_run.go`, `cmd/vtt/client_soak.go`, `cmd/vtt/events_tail.go`, `cmd/vtt/state_dump.go`, `cmd/vtt/harness_boot.go` and `internal/mcp/{server.go,read_tools.go}` — so `vtt client run`, `vtt client soak`, `vtt events tail`, `vtt state dump` and `vtt mcp` are all this package. `internal/mcp`'s own package comment calls itself "internal/harness's second consumer" and inherits the same arch rule (`mcp: { mayDependOn: [harness, engine, contract, mcp] }`).

**The P1 rule still bites.** I re-ran the plan's Task 1 Step 4 bite-proof in a throwaway copy: adding `import _ ".../internal/campaign"` to a file in `internal/harness` makes `go-arch-lint check` report `Component harness shouldn't depend on .../internal/campaign`, naming the file. Clean tree: `OK - No warnings found`. There is no `excludeFiles` entry for harness, so the rule covers `_test.go` too.

**The client.** `Dial` puts `token` and `after` in the query string, raises the read limit to 200 KiB (`readLimit`), and starts `readLoop`. That loop is the only `conn.Read` caller for the connection's lifetime. Results go to the `SendCommand` caller waiting on a per-request channel; events go to a 256-slot buffer (`eventBuffer`, the same number as the gateway's `gatewayBuffer` — confirmed, `internal/gateway/server.go` has `const gatewayBuffer = 256`); a full buffer tears the connection down with `ErrEventsOverflow` rather than blocking the reader. A `ServerFrame` arm this build does not recognise is *ignored*; an empty oneof is fatal. `CatchUpHead` blocks for the server's announced backlog head, and checks the already-announced case in its own `select` before racing ctx and teardown. `CloseErr` reports why the stream ended — and its own comment warns that non-nil does not mean failure, because a clean close also surfaces as a read error.

**The scenario engine.** `LoadScenario` decodes strictly (`DisallowUnknownFields`) and re-decodes each step individually so an error names the step index; validation is structural only — `Command` stays opaque bytes until execution. `RunScenario` dials every participant at `after=0`, refuses to run against a campaign that replays anything (`errFreshCampaignRequired`), resolves every `{{id:<name>}}` placeholder once by byte substitution, then runs every step — a failing step does not abort the run. An accepted step must be observed, matching by `Sequence`, by every *unprojected* participant (dm, agent); players and spectators are drained but never required to observe, because since the visibility arc their stream is a projection and zero envelopes is a correct answer. `use_ability`, `load_adventure` and `load_map` are routed to `observeBatchOnAll` because their result carries only the batch's first sequence. A reconnect step closes the connection, redials at `afterSequence`, and asserts the replay equals, in `event_id` order, the subset of that participant's own live history above the cursor. Probes evaluate against `Fold` of participant 0's observations; six kinds exist (`tokenAt`, `sessionCount`, `actorExists`, `resourceAt`, `hasCondition`, `noteAt`).

**The soak.** Fixed roster dm + player1 + player2 + agent. `pickBucket` maps one `rand.Float64()` to load map 5%, add actor 10%, place 15%, move-own 60%, session churn 5%, deliberate authz-denied 5%. A per-participant goroutine drains each connection continuously into `soakHistories` — that structure is simultaneously the thing that keeps the four long-lived sockets from overflowing, the checkpoint's "incremental" side, and the evidence base for every denial assertion. Every `CheckEvery` accepted actions, `runSoakCheckpoint` folds the observer's history and compares it against a fresh `after=0` connection drained *to a known head* — with a resume loop across the overflow disconnects the client is designed to produce.

**`harness.Fold`.** Eleven lines: `engine.NewState()`, then `engine.Apply` per envelope in order, skipping only `engine.ErrUnknownVariant`. It is reached from `vtt state dump` (`cmd/vtt/state_dump.go`) and from the MCP `get_state` tool (`internal/mcp/read_tools.go`'s `handleGetState`, which folds the MCP server's own accumulated history, never a second connection).

**Verified by running it:**
- `go test ./internal/harness/ -count=1` → ok, 1.5 s. `./cmd/vtt` → ok, 120 s.
- Coverage of `internal/harness`: **89.4 %** against the 85.0 floor in `tools/coverage-thresholds.txt`.
- `vtt client run scenarios/three-role-exit.json` (self-contained) → all 14 steps and all 5 probes pass, exit 0. `scenarios/smoke.json` likewise.
- `vtt client soak --seed 1 --events 500 --json` → `Accepted 478, Denied 22, Checkpoints 5`, `Pass true` — exactly the numbers pinned in `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts`, including the 2026-08-31 re-baseline from 480/20.
- The arc's exit criteria still have tests: `TestThreeRoleExitScenarioOverLiveServeSubprocess` (the live `vtt serve` leg), `TestScenarioLibraryRunsSelfContained` and `TestEveryScenarioFileIsActuallyRun` (the library runs inside `task check`), `TestFoldGoldenCorpus` (the fold against hand-derived goldens).

**What the package is *not* covered by.** `internal/harness` is the one package excluded from `check:mutation` on an argument rather than an oversight. `tools/mutation-scope.md` gives it "2 survivors, 32 not covered, <4 m runtime, **blocked, argued**", and ADR-010 records the exclusion reason changing three times: first runtime (~70 s per mutant — void since `9c651b6` put the suite inside `testing/synctest`, 51 s → 0.7 s), then the survivor count (29 → 2 across five passes), and now **flaky detection** — the last real survivor guards a race, and synctest's fake clock erases the interleaving it defends against, so it is killed only ~60 % of runs. `cmd/vtt`, which is where most of the harness's end-to-end proof lives, has *never* been measured at all: `package main` in a directory named `vtt` resolves to the bare module path and gremlins scores every mutant a false kill.

---

## 3. Besluten och varför

### 3.1 The hardest one: asserting a negative over a network

A denied step must prove two things — that the command came back `ok=false` with the right message, and that **no event reached anybody**. The first is a read. The second is a negative over a network, and there is no wire signal for "nothing is coming."

**First design (shipped in `8aea462`): a bounded quiet window.** `denialAbsenceWindow = 300ms`. Wait; if nothing arrives, the denial passes. The constant's own comment carries the argument for why the shape is safe: a fixed window "can only ever false-PASS an implementation that eventually (but slowly) broadcasts past this window — it can never flake-FAIL a correct one, since a connection that truly never broadcasts stays silent no matter how long the window is." That argument is sound and it is also *exactly where the hole is*: it assumes the connection could have spoken.

**What was rejected, and why.** Making the window tunable was rejected outright and the comment says so — a CLI flag is a path to accidentally shortening it below the safety margin. Extending the window was rejected on cost: on 2026-07-28 that one 300 ms wait accounted for **64 s of `internal/harness`'s 93 s and 28 s of `cmd/vtt`'s 42 s**, measured by dropping it to 10 ms, and it was the reason both packages sat outside `check:mutation`.

**Second design (`4995765`, `7cb0f7d`, `545d718`): prove it by ordering instead of by waiting.** A denied step now passes *provisionally* and its claim is settled against the next accepted step's own event. Per-connection delivery is sequence-ordered and the gateway broadcasts only from the store subscription, so anything a denied command wrongly produced carries a **lower** sequence and must already have arrived by the time the witness lands. Finding none is a proof, not a timeout: no wait, and no dependence on how fast the server is. `leakedSince` (scenario) and `soakHistories.leakedBelow` (soak) are that proof. The bounded window survives only for a *trailing* run of denials, which has no witness — paid once per scenario rather than once per denial.

The rejected alternative there was the obvious one, and it is documented as a mistake that was made: the *first* version of `late_leak_test.go` claimed the harness could miss a leaked broadcast entirely, and justified the redesign on correctness. That claim was wrong — see §4.

**Two corrections the ordering proof needed.** `545d718` turned the single pending slot into a **queue**: denials run back to back in five of the six committed scenarios (`denials.json` has fourteen), and one slot was simply overwritten by the next denial, so the earlier claim was never settled. And because consecutive denials produce no events, nothing orders them relative to each other — so `markLeaked` blames the **earliest** outstanding denial and states the range rather than implying a precision the proof does not have.

**Third design step (`a8c3d3d`, "Stop the harness proving negatives against connections that could not speak"): a live-socket precondition.** This is the part the prompt asks for and it is the centre of the arc. `Conn` grew a `CloseErr()` method — deliberately on the *interface*, so a future fake cannot silently no-op it — and `engine.go`'s comment on it records that **four assertions were returning wrong verdicts and passing**:

> a batch truncated by a teardown counted as complete; a scenario denial "proved" by the silence of a participant that could no longer hear; a soak denial proved the same way through histories that had stopped growing; and a reconnect-at-head certifying "nothing replayed" against a socket that could not replay. All four PASSED.

Every one is the same defect: `case env, ok := <-conn.Events()` where the `!ok` arm — the channel is *closed* — was handled identically to the timeout arm. A dead socket is quiet, a correct server is quiet, and the code could not tell them apart, so the dead socket satisfied the assertion. The rule the comment then states is the arc's single most important sentence:

> a bounded wait proves a negative only against a LIVE connection. If silence is your evidence, ask this first.

The fix is `streamEndReason`/`errStreamEnded` plus four asymmetries that are each argued in place:
- `recvWithin` returns a third value separating "the window elapsed on a live stream" from "the stream was torn down under the read", because reporting the second as "the batch ended" turns a truncated observation into a passing assertion.
- `runCommandStep` reports *both* facts — "unobservable, stream ended for X" and "not observed by Y" — never one instead of the other.
- `markUnprovableSilence` fails **every** outstanding denial, unlike `markLeaked` which fails only the first. The asymmetry is the point: a leak provably came from exactly one denied step and which one is unknowable, so blaming the first is the honest summary; an unprovable silence rests every outstanding denial on the *same* dead participant, so all of them are equally unproven. Marking one would leave the other thirteen in `denials.json` printing `pass=true` on evidence that does not exist.
- `runReconnectStep` tracks `streamEnd` separately from `timedOut`, because setting the timeout flag on a channel close "reported 'before timing out' for a redial whose stream was torn down in milliseconds, naming an `observeTimeout` that never elapsed."

The related note `every-assertion-negative-proves-nothing` in project memory (found 2026-08-26 on `internal/gateway/keepalive.go`) is the general form: a defensive mechanism whose interesting cases are all "and then nothing bad happens" can have its verdict negated and stay green. `a-green-sample-is-not-evidence-of-absence` is the measurement twin: a clean sample of a 1-in-5 event has a 17 % chance and proves nothing.

### 3.2 Reuse `engine.Apply`, do not reimplement it — and take the rule-4 hit

**Decision:** `harness.Fold` applies `engine.Apply` over received envelopes to derive client-side state. **Rejected:** a bespoke client-side state model, and (later) an arch-lint `excludeFiles` exemption letting the harness reach `campaign.FoldPrefix`.

This is the point where CLAUDE.md rule 4 — *"`engine.Apply` (via `campaign.foldEvents`) is the only code that changes game state. Never add a second event-application loop."* — collides with the P1 rule, and the collision is real, not apparent. **Established by enumeration:** `engine.Apply` is called from exactly two places in production Go code, `internal/campaign/campaign.go`'s `foldEvents` and `internal/harness/fold.go`'s `Fold`. The two loops are line-for-line identical apart from the error prefix (`campaign: corrupt log at seq %d` vs `harness: fold: corrupt event at sequence %d`) and the fact that campaign logs a warning on the unknown-variant skip while harness is silent. By the letter of rule 4 this is a violation.

**What the rule actually protects**, read off the place that cites it most carefully, `internal/campaign/foldprefix.go`: `FoldPrefix` exists *only* so that the gateway does not grow its own loop. Its comment is explicit that retraction's departure removed the original correctness reason for its shape and that "Rule 4 is what is left holding this function up, and it holds it alone." So the rule protects a single *implementation of what an event means* — it exists so that a second component cannot drift on semantics. `harness.Fold` does not re-implement semantics; it calls the same `engine.Apply`. The loop around it is duplicated; the meaning is not.

**Is it an exception or a violation?** It is a **documented, arch-enforced exception**, and the exception is argued in three places that agree with each other: the spec's §3 ("Reusing `engine.Apply` is not a leak: the fold is the published derivation algorithm the TS client will reimplement; it consumes only contract types"), `fold.go`'s own comment ("this is deliberately not a second implementation of the event-core's semantics"), and — most usefully, because it is the most recent and was written by someone deciding whether to copy the precedent — `internal/perceive/oracle_test.go`, which says:

> THE PRECEDENT IS harness.Fold, NOT A RE-READING OF RULE 4. An earlier version of this comment argued that rule 4 only forbids a second WRITER, which is not what it says and is contradicted by `campaign/foldprefix.go` […] `internal/harness/fold.go` is a standing third loop, and it is tolerated for a reason that applies here exactly: the arch rules bar harness from importing campaign, so the published derivation has to consume "only contract types plus engine.Apply".

That passage also records that an arch `excludeFiles` exemption was *proposed* for `perceive` and rejected on this precedent, with the reason "a gate exemption nobody needs is worse than none."

The exception is not free, and the cost is visible in the tree: `perceive`'s copy deliberately **does not** skip `ErrUnknownVariant`, because what is forward-compatibility for a client would be a silent hole in an oracle. Two loops with the same body and different tolerance for the same error is exactly the drift rule 4 exists to prevent — it has simply been made deliberate and written down instead of prevented.

### 3.3 The remaining decisions

| Decision | Rejected alternative | Reason |
|---|---|---|
| Re-implement protojson framing rather than import `internal/gateway`'s `codec.go` | reuse the gateway codec | the duplication *is* the proof: a harness that shares the server's encoder proves nothing about the wire. Arch-lint makes it unfalsifiable. |
| `eventBuffer = 256`, overflow tears the client down | block the reader goroutine | a blocked reader also stalls result delivery for every in-flight `SendCommand`. This side cannot tell the server to slow down, so the documented recovery is the caller's: re-dial with a fresh cursor. |
| `readLimit = 200 KiB` | leave coder/websocket's 32768 default | a real `SceneCreated` does not fit. **Re-measured today and it reproduces exactly:** `adventures/goblin-ambush/scenes/ravine.json` is 32×32 = 1024 squares and its committed `scene_created` golden is 44,547 bytes compact — well past 32768. 200 KiB matches `internal/gateway/server_internal_test.go`'s existing `bigPaddingName` precedent rather than inventing a second number, and is explicitly *not* derived from any ceiling: a bigger map needs the same fix again. |
| An unknown `ServerFrame` arm is ignored | tear the connection down | it used to be fatal. The 2026-08-06 contract task added presence arms; under the old behaviour the first presence broadcast would have killed every soak, scenario and `vtt client run` at connect time, with an error naming the wrong cause. ADR-007 makes the contract additive precisely so old readers survive new frames. |
| `projectedSeats` names the **projected** roles | mirror `gateway.projected`, which names the unprojected ones | deliberate opposite defaults. The gateway must fail *closed* — an unrecognised role gets nothing. A test harness must fail *loud* — an unrecognised role is treated as an ordinary observer, so a scenario that introduces one fails its steps instead of quietly asserting nothing. |
| Projected seats are drained, not matched — and drained **after** the required seats | drain in parallel | measured requirement. The drain is bounded by a 300 ms quiet window and the required read by a 2 s timeout, so a late broadcast — the exact shape `TestDeniedCommandLeakFailsTheDeniedStep` injects — reaches the required seat and would have been missed by a projected one running beside it. |
| `Fold` skips `ErrUnknownVariant`, fails on everything else | fail on all | the same forward-compatibility the server's own replay gives an unrecognised variant. |
| Scenario boot glue lives in `cmd/vtt`, hands over only strings | let the harness compose a server in self-contained mode | self-contained mode must not become a back door that hands the harness a server object just because the process owns both ends. `harness_boot.go` returns a ws URL, a name→token map and a name→id map — never the `*http.Server`, `*campaign.Campaign` or `*identity.DB`. |
| `statesEqual` duplicated from `internal/campaign/scenario_test.go` | share it | harness may not import campaign even in test code. `proto.Equal` per actor, `reflect.DeepEqual` for the plain maps. |
| Soak maps drawn from a pool **in order** | draw at random | choosing between interchangeable things would spend an rng draw and buy nothing — and the determinism obligation is over the whole draw sequence. The same reasoning is why `planLoadMap` consumes exactly one `pickDMOrAgent` draw, the same as the `planCreateScene` it replaced, so the `create_scene` → `load_map` swap did not move seed 1's pinned counts. |
| Fresh catch-up drains to a **known head** | drain to silence | see §4. |

---

## 4. Vad som visade sig fel

### 4.1 The two claims the prompt asked me to check

**Claim A — "`leakedSince` can be neutered in one specific way with the suite still green."** **True, with an important qualification.** I ran fourteen hand-built mutations against a throwaway copy.

Every mutation that stops `leakedSince` *detecting* a leak is killed by `internal/harness`'s own suite: returning `nil` outright, scanning nothing (`envs[len(envs):]`), a comparison that is never true, a comparison that is always true, `<=` instead of `<`, and dropping the `min(...)` bound-guard (which panics — pinned by `TestDenialFollowedByReconnectDoesNotPanic`). Mutating the call site to `sr.Pass && sr.firstSeq > 0`, or removing `markLeaked`'s `Pass = false`, is also killed.

What survives is the `lens` parameter. Replacing `envs[min(lens[name], len(envs)):]` with plain `envs` — **ignoring the before-snapshot entirely** — leaves `internal/harness` completely green. So does `!=` in place of `<`. Both are over-reporting mutants, and the reason nothing in the package notices is a **degenerate fixture**: every denial-leak fixture in `late_leak_test.go` puts the denial as step 0 of an empty log, so there is no earlier history for the offset to exclude. Neither `TestDeniedThenCleanAcceptPasses` nor `TestConsecutiveDenialsBlameTheEarliestOutstandingDenial` ever runs a denial *after* an accepted step.

Both are caught — by `cmd/vtt`. Under the `lens`-ignoring mutant, `TestClientRunSelfContainedRunsCommittedThreeRoleExitScenario`, `TestScenarioLibraryRunsSelfContained`, `TestThreeRoleExitScenarioOverLiveServeSubprocess` and `TestScenarioGoldenStreamsHaveNotDrifted` all fail, because `scenarios/three-role-exit.json` has its denials at steps 8 and 10 with seven accepted events already in every history. That is the honest shape of the finding: **`leakedSince`'s snapshot offset is dead to the gated package and alive only in the ungated one** — and `cmd/vtt` is the package `tools/mutation-scope.md` records as "never measured", unmeasurable while `package main` sits in a directory named `vtt`.

One mutant survives **both** packages: settling against `pending[len(pending)-1].lens` instead of `pending[0].lens` — i.e. re-introducing exactly the defect `545d718` fixed. It survives because in the scenario path it is an *equivalent* mutant: `history` is only ever appended to by `observeOnAll`, `observeBatchOnAll`, `drainAllForSilence` and `runReconnectStep`, none of which run between two consecutive denial steps, so the earliest and latest snapshots are always identical. In the **soak** it is not equivalent — the per-participant drain goroutines run continuously, so a leak from denial A is folded into histories before denial B's snapshot is taken and becomes invisible to it. That is pinned, in the soak only, by `TestRunSoakCatchesALeakFromANonFinalDenial` (seed 22). So the scenario engine's `pending[0]` is defended by nothing of its own; it is correct by a property of the current control flow, and the moment the scenario engine grows a background drain it becomes the soak's bug with no test to say so.

**Claim B — "a probe whose kind no arm recognises returns Pass."** **False as stated, and one character away from true.** `evaluateProbe`'s `default` arm returns `ProbeResult{Index: idx, Kind: "unknown", Detail: "probe has none of …"}` — `Pass` is the zero value, `false`. Verified by execution: an empty `Probe{}` returns `Pass=false`, and `RunScenario` then sets `rep.Pass = false`. It has always been that way; `git log -S` shows the arm arriving in `8aea462` and never changing its verdict. The same is true of `RunScenario`'s `Kind: "unknown"` step arm.

What *is* true, and is what the reader was probably looking at: **both arms are uncovered.** The coverage profile shows zero hits on `engine.go:1207-1208` (the probe default) and `engine.go:342-343` (the step default). `validateProbe` and `validateStep` make them unreachable from a loaded scenario file, so only a caller constructing a `Scenario` in Go can hit them. And because `Pass` is the zero value rather than an explicit `Pass: false`, adding `Pass: true` to **both** arms leaves `internal/harness` green — I made that edit and ran it. So the failure mode the claim describes is real and undefended; it just is not the code's current behaviour.

### 4.2 Claims this arc later had to retract

**The correctness argument for the ordering proof was wrong; the cost argument was right.** The first version of `late_leak_test.go` (`4995765`) claimed the harness could miss a denied command's broadcast entirely. `7cb0f7d` retracted it two days later and left the retraction in the file: the test that "proved" it delayed the leaked broadcast until *after* the next accepted step's broadcast, which is out-of-order delivery — impossible, since the gateway broadcasts only from the sequence-ordered store subscription, and it would have defeated an ordering-based proof too. The test contradicted the design it was arguing for. What was actually broken was **attribution**: the denied step was reported passing and the innocent next step was blamed with "event not observed", sending an operator after the wrong command. The redesign went ahead on cost and clarity.

**"Every `Events()` consumer accounts for a closed channel" was claimed while it was false.** `engine.go`'s own comment says so: "an earlier version of this comment claimed the conversion was complete while `soakHistories.start`, the evidence base for `RunSoak`'s own denial assertion, still had the hole."

**The soak keystone raised a false alarm about itself, twice.** On 2026-08-04 CI printed `[checkpoint 5] FAIL: incremental fold (480 events) != fresh catch-up fold (257 events)` — which reads as the event-sourcing design's central claim failing. It was not. 257 = `eventBuffer`(256) + 1 in flight: the fresh catch-up connection had been torn down for overflow and the checkpoint folded the truncated history as if complete. Fixed by `84c9b66` (`drainFreshCatchUp` resumes from the last sequence *actually received*) and `cf1df63` (drain to a known head rather than to 300 ms of silence — a 300 ms gap mid-replay is ordinary on a loaded runner). Both comments now say the same thing: a false alarm on the keystone is worse than a flake, because it sends someone hunting a fold divergence that does not exist.

**`vtt state dump` was a coin flip.** `CatchUpHead` originally put the announced-head channel, `ctx.Done()` and `readerDone` in one `select`. Once the head is announced the connection is free to end and the ctx free to expire, so two or three arms are ready at once — and Go picks uniformly at random. A caller was roughly as likely to be told "connection ended before the server announced its catch-up head" as to be told the head it already held. Fail-closed, so it never truncated a dump; but the error was a lie (`3a81cdd`, `9e3e689`).

**A failing soak said `"Pass":false` and nothing else.** `SoakReport.Report` "exists because it was documented before it was real": `errSoakFailed`'s comment in `cmd/vtt/client_soak.go` promised that `--json`'s Report body names the failing action, while `--json` sent the progress writer to `io.Discard` and the struct had no such field. Every FAIL line naming the cause was thrown away at the one moment it mattered (`5891dcb`).

**`leakedSince` could panic on an operator's scenario file.** `runReconnectStep` *rebuilds* history rather than appending, so it can be shorter than a pending denial's snapshot. The soak twin `leakedBelow` always had the `min` guard; the engine copy did not, and settling that denial panicked with a slice-out-of-range instead of reporting a failed scenario.

**A batch command was missing from `isBatchCommand` and the corpus could not show it.** `load_map` belongs in the set because `mapdef.Compile` emits `1 + len(placements)` envelopes appended as one batch — but all the committed maps declare zero placements, which makes every committed `loadMap` step the one-envelope case `observeOnAll` happens to handle. Two measurements in that comment have since drifted: it says "all ten `scenarios/maps/*.json` declare none" and there are now **eleven** (`door-hall.json` arrived in `53bb71f`); the substantive claim still holds — I checked all eleven and every one declares zero placements, while `campaigns/example/maps/cellar.json` declares one. The `Conn.CloseErr` comment's enumeration has drifted the same way: it lists eight consumers and does not name `drainQuietInto`, which arrived later and does handle the closed channel. Both claims survive their own stale counts, which is the good case; the point is that both counts are the kind that go stale silently.

**Two spec paragraphs were false and are now corrected in place.** §4's example scenario carried `"controls": ["act-lera"]` — a key that never conferred anything (no `ActorControlGranted` was ever emitted from it) and that `DisallowUnknownFields` would have rejected, so a scenario copied off the spec page would not have loaded. And the plan's Task 2 signature for `Fold` specified two-pass retraction semantics; `92f1284` made it single-pass and `59542e1` deleted the message, so Task 2's fold-parity test and Task 4's injection proof (ii) describe machinery that no longer exists. The plan carries an AMENDED banner saying so.

**The mutation-exclusion reason was wrong twice before it was right.** ADR-010 first excluded harness for runtime (~70 s per mutant); `9c651b6` made that void. It then excluded it on 29 survivors; five passes took that to 2. The standing reason is the third: the last survivor guards a race, and synctest's fake clock — the thing that made mutation testing feasible at all — makes the race guard's removal detectable only ~60 % of the time. Gating on a ~60 % kill installs a flaky gate.

### 4.3 The defect class the report itself is aimed at

Eleven comments across the five production files cite `task-1-brief.md`, `task-2-brief.md`, `task-5-brief.md` or "P6 Task 4 report" as the **binding** source for semantics — the denial-absence window, the 300 ms constant, the scenario engine's contract, the soak's mix and roster. Those files are in `.superpowers/`, which is gitignored. On this machine they resolve — to a *different plan's* briefs: `.superpowers/sdd/task-2-brief.md` today is the ruleset-interpreter's engine-folds brief (Conditions, ResourceChanged) and `task-5-brief.md` is that plan's Resolve/conformance brief. No sub-project-4 task report survives anywhere in the tree. This is the exact shape CLAUDE.md rule 8 names as out ("a bare `task-3-brief.md` does not say which plan's"), and it fails in the worst way: the citation still looks valid while resolving to the wrong document, on the one machine where it resolves at all. Every one of those eleven citations should become a citation of the spec section or the test name that actually holds the rule.

Nothing about `internal/harness` appears in `docs/verification-debt.md` (uncommitted, sub-project 14). The two gaps found above — the dead `lens` offset in the gated package, and the two uncovered `"unknown"` arms whose verdict is a zero value — are exactly what that file is for, under the labels `test data missing` and `test asserts nothing`.

---

## 5. Spår

**Range: `2743417..e13f468`, with `c218279` and `a91b8b0` as the arc's merge-gate tail. There is no merge commit and no branch.** The plan's Global Constraints say "Branch `feat/simulation-harness` from `main`"; that is not what happened. The whole arc landed straight on main in a single 4½-hour run on 2026-07-24.

| commit | time | what |
|---|---|---|
| `2743417` | 13:06 | docs: simulation harness design spec |
| `6680836` | 13:29 | docs: implementation plan |
| `121be93` | 14:06 | Task 1 — wire client (`client.go`, `client_test.go`, the arch rule) |
| `8aea462` | 14:40 | Task 2 — `scenario.go`, `engine.go`, `fold.go` + testdata |
| `55b8d36` | 15:30 | Task 3 — `client_run.go`, `events_tail.go`, `state_dump.go`, `harness_boot.go` |
| `7990095` | 16:32 | Task 4 — `scenarios/{three-role-exit,denials,smoke}.json`, `library_test.go` |
| `e13f468` | 17:34 | Task 5 — `soak.go`, `client_soak.go` |
| `c218279` | 19:33 | workflow-review fix wave, 23 findings — harness gets `errFreshCampaignRequired` and the ambiguous-`expect` rejections |
| `a91b8b0` | — | docs: six spec amendments from the merge gate |

**How I established it.** The chain from the prompt gives the front end and then stops. `git log --oneline --follow --reverse -- internal/harness/` names `121be93` as the first commit. `git log --merges --oneline --ancestry-path 121be93..main | tail -1` returns merges, but the earliest of them belongs to sub-project 5a three weeks later — none of them is this arc's. `git rev-list --parents -n 1 121be93` returns a single parent, so `121be93` is not the tip of anything; and `git log --format='%h %ad %p | %s'` across the window shows every commit from `2743417` to `a91b8b0` with exactly one parent. So the arc is a run of **nine consecutive commits with no merge at all**, and the plan's branch constraint was not followed. I bounded the front at the spec commit by walking back from `121be93` until the subject stopped being about this sub-project, and the back at `1039555` ("docs: MCP gateway spec and plan"), the first commit belonging to the next arc. `c218279` and `a91b8b0` sit between the last task commit and that boundary and are the arc's own review wave, so they are in.

**What has happened to the territory since.** 44 commits have touched `internal/harness` between `121be93` and today. The arc as landed was roughly 1,400 lines of production code across four files; today's five files are 1,843 lines of code and 1,417 of comment. The heaviest later arcs are visible in the comment mass itself: the live-socket work of 2026-08-02…08-06 (`cf1df63`, `3a81cdd`, `9e3e689`, `84c9b66`, `a8c3d3d`), the visibility projection (`c245a6d`, `0d30285`), the actor-control model change (`18b7212`, `f1af602`, `8f130c8`, `499bbec`), retraction's removal (`92f1284`, `133e896`, `59542e1`) and `create_scene`'s (`e110e9b`, `f740b2c`, `0b9d6b7`).

---

## 6. Kommentarsblock denna rapport friar

50 blocks of ten or more consecutive comment lines, all in the five production files. Line ranges are as of `48e3fe2`. **Not one block is pure history** — every one wraps a rule in a story, so each entry gives both: what this report frees, and the invariant that must stay at that line for a change to be correct.

### `internal/harness/client.go`

| lines | what the report frees | what must stay |
|---|---|---|
| 1–13 | why protojson framing is re-implemented here instead of reused from the gateway's `codec.go` — "a deliberate, documented duplication, not an oversight" | **This package may import only `vttv1`, `internal/engine`, `coder/websocket` and stdlib.** Never gateway, campaign, identity or store. Arch-lint enforces it, including in tests. |
| 32–42 | the comparison with the gateway's own overflow reaction, and the recovery narrative | Buffer is **256**, matching the gateway's. On overflow the client tears *itself* down rather than blocking the reader — a blocked reader also stalls result delivery. The caller's only recovery is `Dial` again with a fresh `after`. |
| 45–60 | the maps-as-geometry story, the `bigPaddingName` precedent, the goblin-ambush measurement | `Dial` must raise the read limit or a real `SceneCreated` silently and permanently desyncs the connection. **200 KiB is not derived from any ceiling in the wire contract** — a map larger than this needs the same fix again. |
| 187–208 | the coder/websocket issue link and quotation | `cmd` **must not be shared across concurrent `SendCommand` calls** — `RequestId` is mutated in place. Cancelling ctx mid-write tears down the **whole** connection, not just that call; there is no per-call recovery. |
| 314–326 | that this arm used to be fatal, and what the presence arms would have done | **An unrecognised `ServerFrame` arm is ignored, never fatal.** ADR-007's additivity depends on it; `client/src/wire.ts` does the same. An *empty* oneof stays fatal. |
| 364–380 | the CI failure that conflated a dead socket with a slow reader | **Non-nil does not mean failure** — a clean close also reports non-nil. The only question worth asking is `errors.Is(CloseErr(), ErrEventsOverflow)`. |
| 450–472 | the uniform-random `select` bug that made `vtt state dump` a coin flip | An **announced head outranks** both ctx expiry and connection end, so it is checked in its own `select` first. 0 means the log was empty. The head is the catch-up/live boundary the wire does not otherwise express. |

### `internal/harness/engine.go`

| lines | what the report frees | what must stay |
|---|---|---|
| 27–59 | the four wrong verdicts, the earlier over-claim, the enumeration of consumers | **`CloseErr` is on the interface so a fake cannot silently no-op it.** A bounded wait proves a negative **only against a live connection** — if silence is your evidence, ask this first. |
| 92–105 | that this resolution used to live in test-side helpers | `{{id:<name>}}` is the library's **one** templating convention, resolved in one place so every caller gets it. |
| 111–125 | the P6 fix-round provenance | Substitution is a **plain byte scan, never a protojson parse**. A name missing from `ids` is a hard error naming step and name, raised before anything is dialed — never a literal placeholder dispatched as command data. |
| 150–161 | the "planned format extension" aside | Every step, probe and reconnect assertion reasons in **absolute sequence numbers from a fresh campaign**. A non-fresh campaign must fail loudly here, not shift every later comparison. |
| 166–176 | the cross-reference to `denialAbsenceWindow`'s reasoning | Silence proves freshness **only from a connection that could have spoken** — the returned error separates "heard nothing on a live stream" from "the stream was already gone". |
| 232–266 | the P6 Task 4 fix-round provenance for `ids` | The three binding semantics: a denial asserts `ok=false` + substring + **no broadcast to any participant**; an accepted step asserts `ok=true` + observation by every **unprojected** participant; a reconnect asserts catch-up equals the live subset above the cursor in `event_id` order. Probes fold participant 0's observations. A non-nil error means a **framework** failure only. |
| 476–485 | the description of why this replaced a timeout | **The ordering proof.** Per-connection delivery is sequence-ordered and the gateway broadcasts only from the store subscription, so anything a denied command produced carries a **lower** sequence and must already have arrived. The `lens` offset is what makes this a claim about the *suffix* — ignoring it turns every earlier event into a false leak (measured). |
| 541–554 | the 64 s/93 s and 28 s/42 s cost figures (void — the suite is now 1.5 s) | Absence is **not** proven at the denied step. The verdict is deferred to `RunScenario` and settled against the next accepted step's event, because proving it by waiting both costs the window and **blames the wrong step**. |
| 566–575 | which arc made which command batch-aware | `use_ability`, `load_adventure` and `load_map` results carry only the batch's **first** sequence; every other accepted command produces exactly one event. |
| 608–628 | the `load_map` miss, the corpus blindness, the (now stale) "all ten maps" count | **The set is deliberately not derived from anything** — there is no wire-visible mark on a batch-producing command, so this list and `internal/gateway`'s dispatch are two hand-kept halves of one fact. |
| 798–818 | that this is the visibility arc's doing rather than a relaxation, and the measurement behind the ordering | A projected seat is **not required to observe anything** (zero is correct). It must still be **drained**, because `leakedSince` and the reconnect comparison both read `history`. The drain runs **after** the required seats, never beside them. |
| 845–882 | the contract provenance and the "nothing to push back" argument | A batch is a **contiguous run from `firstSeq`**, ended by the quiet window or by a contiguity break. Every observed event is recorded into `history` regardless of outcome. Correctness rests on `RunScenario`'s serial execution — exactly one command in flight. |
| 1054–1063 | "This used to return the same `(nil, false)` for both" | A window elapsing on an **open** stream and a channel **closing** are opposite facts. Reporting the second as "the batch ended" turns a truncated observation into a passing assertion. |
| 1212–1236 | the comparison with `gateway.projected` and the reason for the opposite default | This names the **projected** roles (`player`, `spectator`), so an unrecognised or empty role is treated as an ordinary observer and **fails loudly**. Role strings, not `identity.Role` — harness may not import identity. |

### `internal/harness/fold.go`

| lines | what the report frees | what must stay |
|---|---|---|
| 11–28 | single-pass since `92f1284`, and retraction's departure | `Fold` is the **published derivation algorithm**, consuming only contract types plus `engine.Apply`. `ErrUnknownVariant` is skipped; anything else is fatal — the same forward-compatibility the server's own replay gives. |

### `internal/harness/scenario.go`

| lines | what the report frees | what must stay |
|---|---|---|
| 15–28 | the "planned format extension" aside | Sequence reasoning is **absolute, from a fresh campaign**; `RunScenario` enforces it at runtime rather than letting it misbehave silently. |
| 43–53 | that it mirrors `Ruleset`'s resolution | Repo-root-relative, and **already the directory itself** — no further joining. Empty means no `load_adventure`. |
| 55–71 | that `--maps-dir` was deleted by create-scene-leaves Task 5 | **Naming the directory is a request to install its contents** into the campaign, not to point the server at them. The field exists so `vtt client run` keeps working on a scenario file outside this repository. |
| 75–95 | the deleted `controls` key, and the `add_actor`-with-controller route that is gone | A participant **is** a connection at a role. Control comes from a `grant_actor_control` step and **only** from there. An `add_actor` step must carry a `kind` or the server refuses it. |
| 131–141 | which arc added which probe kind | **Exactly six kinds**, one per probe. Probes assert *structural* state, never exact dice or wire bytes. Growth is a spec amendment. |
| 187–197 | why there is no `Present`-style absent flag | `Key` **must be present**. `TitleIs` and `TextContains` are independently checked **only when non-empty**; a probe setting neither asserts presence alone. |
| 204–217 | the `task-2-brief.md` citation | Unknown JSON fields are **errors**; every step is decoded individually so a failure **names the step index**. Validation is **structural only** — `Command`'s protojson is parsed at execution time, so a malformed command is that step's failure, not a load error. |

### `internal/harness/soak.go`

| lines | what the report frees | what must stay |
|---|---|---|
| 1–15 | the "keystone" framing and the comparison with the reconnect scenarios | At every checkpoint an **incremental** fold of one participant's live stream must **deep-equal** a **fresh** catch-up fold on a brand-new second connection. |
| 36–55 | that `IDs` is additive beyond the brief, and the precedent chain | `IDs` may be **nil**: the run completes with degraded mix coverage, never a crash. These are real server-assigned participant ids this package cannot invent. |
| 93–104 | that the field was documented before it existed, and the CI run that printed `"Pass":false` and nothing else | The action log is carried on a **failing** run only, because `--json` is the mode CI uses and it otherwise discards every FAIL line. |
| 150–177 | retraction's 10 %, the `create_scene`→`load_map` swap, and why the freed share went to move-own | The bands: **5 / 10 / 15 / 60 / 5 / 5**, summing to 100. `pickBucket` is a **pure function of `r`** — every draw must consult nothing but the rng stream and accumulated model state, never wall-clock or map-iteration order. |
| 217–227 | "since the visibility projection landed" | DM and agent can be waited on **by sequence**; the two players receive a projection and must be **settled**, not waited on, before a denial snapshot — or their legitimately-late envelopes read as a leak. |
| 350–362 | that `waitFor`'s comment named this race and the settle did not guard it | **`SendCommand` returning means the server accepted the command, not that any broadcast reached a participant's `Events()`.** Settle only after `waitAllCaughtUp`. |
| 405–414 | why this was extracted rather than inlined (gocyclo — "a quality gate is not weakened to fit a change") | **Three outcomes, not two.** Grew = leak. Ended = not evidence of anything. Quiet on a live stream = proof. |
| 473–491 | the 480-vs-257 false alarm and its exact printed line | Drain to the **observer's own highest sequence**, not to a silence. The quiet window survives only as the signal that the tail *after* the target has settled. |
| 547–571 | the 2026-08-04 incident and its diagnosis | The resume cursor is the **last sequence actually received**, never the target — re-dialling from 0 double-counts and from ahead loses envelopes silently. This covers **client-side** overflow only. Attempts are bounded, and one clock bounds the whole catch-up, not each attempt. |
| 692–709 | the fix-round note about the third purpose being missing from an earlier version | The drain begins **at dial time** and keeps every long-lived connection from overflowing. It is simultaneously the checkpoint's incremental side and the **only** evidence base for every denial assertion. |
| 768–777 | that this replaced `grewSince`'s unconditional sleep | Same ordering proof as `leakedSince`: a leak carries a lower sequence and must arrive before the witness. **No wait, and it holds however slow the server is.** |
| 862–874 | the `propModel`/`doUndo` record and the `eventgen` comparison | **`apply` is called only after the wire accepted the command** (`result.Ok`), so the model never assumes success ahead of the server's answer. |
| 955–971 | "one draw is not always one bucket, as of 2026-08-24" | **Exactly one `rng.Float64()` picks the bucket**, then bucket-dependent draws. An unmet precondition falls through to `addActor` (always valid) to guarantee forward progress. An owed grant is issued first and consumes **no** rng. |
| 1016–1039 | that this was a wire-size decision until `create_scene` left | **30.** `planMoveOwn`'s destinations must draw from exactly this range, or move-own becomes an unintended second source of denial. Exported because `cmd/vtt` writes the pool files and this package may not. |
| 1042–1056 | the 5 %-band arithmetic and which two tests measure the mix | The mix test **asserts** the pool did not run dry, so shrinking this reds there rather than quietly distorting the ratio. A longer run degrades into `addActor`, never into a denial. |
| 1059–1074 | the "install, then load" provenance | A live `--server` soak needs these maps installed **at `SoakMapGridSize` square and all floor**. The pool is drawn in order because choosing between interchangeable things would spend an rng draw and buy nothing. |
| 1099–1112 | "assigning is now the next step, not this one", and the cost being the point | The **first two** `addActor` draws go to player1 then player2 in that priority order; every later one stays uncontrolled. |
| 1130–1142 | that `add_actor` began refusing a kindless actor on 2026-08-24, and the `actorKind`-not-`kind` naming aside | **Every generated actor states a kind**, and the value follows the recorded intention — `PARTY_MEMBER` for an actor this run means to hand to a player, `NON_PARTY` otherwise — so both values appear in every run of any length. |
| 1167–1177 | "a soak issuing the refused shape would spend its whole run failing on its own setup" | The grant states `PARTY_MEMBER`. It takes **no rng** and clears the slot **before** returning, so the same seed produces the same sequence and no grant is ever issued twice. |
| 1250–1262 | that grid bounds became enforceable in maps-as-geometry Task 6 | Destinations are bounded to the scene's **own** `SoakMapGridSize`. Every square of every pool map is **floor**, so bounds are the only way a move could be refused — and a well-formed client never sends a destination its scene cannot contain. |
| 1308–1320 | the comparison with `waitAllCaughtUp` as "the sharper instrument" | A projected seat **cannot** be waited on by sequence — there is no envelope to name and zero is a correct answer — so a bounded quiet is the only signal left. Returns nothing on purpose: a seat that never goes quiet is busy, not broken. |

---

## Notes on method and limits

Every claim in §2 and §4 was verified by running a command; nothing was taken from a comment without checking it. Mutation work was done on two `cp -R` copies under the session scratchpad — **the repository was never modified**; `git status` shows the same 17 files at the end as at the start.

Two things I could not establish. First, the arc's ADR-009 injection proofs (Task 4's three, Task 5's one) are cited in the plan but no transcript survives: `.superpowers/sdd/` holds no sub-project-4 report, and the `p4-*` files there belong to the api-gateway arc. I can confirm the proofs' *targets* still have tests, not that the proofs were run. Second, I did not run `check:mutation` over `internal/harness` myself — it is not in the gate, and `tools/mutation-scope.md`'s "2 survivors / 32 not covered" is from a 2026-07-30 run against a tree that has moved a long way since. The hand-built mutations in §4.1 are not a substitute for that run; they are a targeted probe of one function.
