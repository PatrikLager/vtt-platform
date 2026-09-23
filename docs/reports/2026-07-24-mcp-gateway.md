# Implementation report — MCP gateway (sub-project 6)

Plan: `docs/superpowers/plans/2026-07-24-mcp-gateway.md`
Spec: `docs/superpowers/specs/2026-07-24-mcp-gateway-design.md`
Territory: `/Users/patriklager/dev/vtt-platform/internal/mcp`, plus `/Users/patriklager/dev/vtt-platform/tools/toolgen` and `/Users/patriklager/dev/vtt-platform/cmd/vtt/mcp.go`.

Everything below was verified by command against the working tree at `48e3fe2` on 2026-09-19. The tree was not modified; the bite proof in section 3 was run in a throwaway rsync copy in the scratchpad, which has been deleted.

---

## 1. Vad som byggdes

`vtt mcp` — an MCP server over stdio that seats a language model at the table as the *agent* participant. It is a wire client: it dials the gateway's WebSocket through `internal/harness` exactly as a human client does, holds one connection for the process lifetime, and adds zero privilege — every tool call becomes an ordinary `ClientCommand` stamped with the agent's participant id and judged by the gateway's own authz table. The arc shipped the server core with generic command dispatch built from the committed tool manifest, two read tools (`get_state`, `get_events_since`), the `cmd/vtt` wiring with token resolution, and an end-to-end test that plays `scenarios/smoke.json` through the tools and compares the result against an independent wire observation. Acceptance was not a test but a live demo: Patrik watching Claude DM a short session through Claude Code.

---

## 2. Hur det fungerar i dag

**One reflective handler, 22 command tools.** `mcp.New` (`internal/mcp/server.go:145`) parses `Config.ToolsJSON` into manifest entries, then calls `buildDispatch` (`internal/mcp/tools.go:55`), which walks `vttv1.ClientCommand`'s `command` oneof descriptor and matches each field's proto name against the manifest's tool names. The result is `map[string]protoreflect.FieldDescriptor`. `New` then loops the entries and registers each one with the *same* handler closure, `s.handlerFor(fd)` (`server.go:460`). That handler does four things regardless of which command it is: turn arguments into the command (`commandFromArgs`), `SendCommand`, `protojson.Marshal` the result, set `IsError: !result.GetOk()`.

There is no per-command switch in the package. Measured: `grep -n switch internal/mcp/*.go` (non-test) returns exactly two switches, both unrelated — `door_tools.go:73` on URL scheme and `door_tools.go:169` on HTTP status code.

`buildDispatch`'s agreement check is symmetric and both directions are `mcp.New` errors: a tools.json name with no oneof field, and a oneof field with no tools.json entry. Four internal tests cover it (`tools_internal_test.go:88–148`), including one that asserts both directions are reported at once.

**The six hand-registered tools.** `New` calls four registration functions after the manifest loop:

| tool | file | behaviour |
|---|---|---|
| `get_state` | `read_tools.go:145` | folds the server's own accumulated history through `harness.Fold`, marshals it in `vtt state dump`'s Go-JSON shape, adds `headSequence` and `wireConnected` as camelCase siblings |
| `get_events_since` | `read_tools.go:215` | paginates the same history strictly after `afterSequence`; default limit 50, values above 200 clamped; each envelope re-encoded as protojson (so its own `sequence` is a *string*) |
| `get_ruleset_guide` | `guide_tool.go:46` | returns `Config.RulesetGuide` verbatim, or a tool-level `isError` "mcp: no ruleset loaded" |
| `get_adventure_guide` | `adventure_guide_tool.go:88` | returns `Config.AdventureGuides[adventureId]` verbatim; distinct errors for "none configured" and "unknown id" |
| `get_join_link` | `door_tools.go:122` | HTTP GET `/api/join-link` on the origin derived from the WS URL, bearer token in the header |
| `get_participants` | `door_tools.go:126` | HTTP GET `/api/participants`, same path |

Only the first two belong to this arc; the other four arrived in later sub-projects (ruleset-interpreter, adventure-format, #45). All six are named in one exported slice, `GoRegisteredToolNames` (`read_tools.go:104`), which exists so the total has exactly one place to be maintained.

Live total: **28** — 22 manifest tools + 6 Go-registered. Verified by `go test ./internal/mcp/...` (green, 1.6 s), where `TestListToolsReturnsEveryCommandAndReadTool` asserts the registered set equals `len(wantCommandToolNames) + len(GoRegisteredToolNames)` and names every member.

**How `contract/gen/tools/tools.json` is produced and copied.**

1. `tools/toolgen/main.go` holds a hand-written `manifest []toolSpec` — 22 entries, each naming a proto message, a tool name, an LLM-facing description, and optional per-message schema overrides. `buildTools` derives each tool's JSON Schema from the compiled descriptors; proto3 `optional` (synthetic oneofs) decides the `required` list, per ADR-007.
2. `task generate:contract` (`Taskfile.yml:1135`) runs `buf lint`, `buf generate`, then `go run ../tools/toolgen -o gen/tools/tools.json`, then one `cp gen/tools/tools.json ../cmd/vtt/tools.json`.
3. `cmd/vtt/tools.json` is embedded with `//go:embed` (`cmd/vtt/mcp.go:45`) — `go:embed` cannot cross package directories, and a runtime file read would break single-binary distribution. `internal/mcp` never touches the filesystem; `Config.ToolsJSON` takes raw bytes.
4. `task check:drift` (`Taskfile.yml:1057`) re-runs `generate:contract` and fails if `git status --porcelain -- contract/gen cmd/vtt/tools.json cmd/vtt/webdist` is non-empty. The `cp` line is what binds the copy to the gate; without it the copy would be silently repaired instead of the divergence being seen.

Measured: `contract/gen/tools/tools.json` and `cmd/vtt/tools.json` are byte-identical, 22 entries each.

**Session lifecycle.** `Run` (`server.go:215`) dials once, installs the client, starts `pump` on a derived context, then serves MCP. The shutdown defers are ordered so LIFO cancels the pump *before* closing its client. `pump` drains `client.Events()` continuously — `harness.Client` tears itself down if its buffer overflows — and calls `recordEvent`, the one mutex-guarded hook that dedupes (strictly-greater sequence), appends to `history`, and advances `lastSeq`. On stream close it hands off to `redial`, which retries `harness.Dial` with `after=lastSeq` under capped exponential backoff (100 ms → 2 s, no jitter). Tool calls made while disconnected return a clean MCP error, never a crash.

**cmd wiring.** `vtt mcp --server ws://… [--token …] [--ruleset …] [--adventures-dir …]`, served over `&mcpsdk.StdioTransport{}`. `resolveMCPToken` (`cmd/vtt/mcp.go:150`): flag wins, then `VTT_TOKEN`, else a clear error naming both. `context.Canceled` from the SDK is swallowed so Ctrl-C exits 0. SDK pinned at `github.com/modelcontextprotocol/go-sdk v1.6.1` since the first commit of the arc and never bumped since; the transport type the plan asked to have resolved and recorded is `mcpsdk.Transport`.

**One thing the code relies on that is worth stating plainly.** The SDK's low-level `Server.AddTool` does **not** validate arguments against the declared input schema — its own doc comment says "Unmarshaling the arguments and validating them against the input schema are the caller's responsibility." So every `inputSchema` in this package, including `get_events_since`'s `"required": ["afterSequence"]` and toolgen's `required` lists, is advertisement to the model, not a gate. `commandFromArgs` and the two strict `encoding/json` decoders are the only enforcement on this side, and the gateway is the enforcement on the other. This is why `commandFromArgs` is correctly described as a trust boundary.

---

## 3. Besluten och varför

**1. Reflective dispatch instead of a per-command switch.** *Rejected:* one handler per command, or one `switch` over the oneof — the shape `internal/gateway` uses. *Reason:* a new campaign command must cost zero per-command MCP code. The manifest is generated from the contract, the oneof descriptor is read at startup, and the two are matched by name. Today three other places in the repo transcribe the same contract by hand: `internal/gateway/authz.go`'s `commandName` (22 `case` arms returning string literals), `internal/gateway/authz.go`'s `commandRoles` map (22 string keys), and `client/src/commands.ts` (one builder per command). A fourth, `internal/gateway/convert.go`, transcribes 14 of them. All four must be edited when a command is added; `internal/mcp` must not. That asymmetry is the repo's own evidence that the transcriptions are avoidable — the same `protoreflect` walk works in both places, and `internal/gateway`'s own `TestEveryClientCommandHasRoleCells` already uses it to *check* the table it declines to *derive*.

The cost is real and worth naming: reflective dispatch means the compiler cannot tell you a command is unhandled. The replacement for compiler help is three guards — `toolgen.TestManifestCoversAllCommandMessages` (contract → manifest), `TestEveryManifestCommandIsRegisteredAsATool` (manifest → registered tool, reading the generated manifest), and `buildDispatch`'s symmetric startup check (configured tools.json ↔ the `vttv1` build actually linked in).

**2. `ok=false` becomes `isError: true` with the CommandResult JSON as content.** *Rejected:* returning a Go error, which the SDK surfaces as a protocol-level dispatch failure. *Reason:* a refused command is a normal conversational event for an LLM — it needs the structured failure to read and correct, not a broken call. The same reasoning is applied a second time in `handleGetEventsSince`, where a bad argument returns a tool-level `isError` rather than an error.

**3. `get_state` folds the server's own accumulated history, never a second connection.** *Rejected:* opening a second wire connection, or a fresh `after=0` catch-up per call. *Reason:* two connections can observe two different prefixes of the log, and the whole value of `headSequence` is that the model can reason about what it has seen. `historySnapshot` (`server.go:360`) is the one read path both read tools use, so they always see the same stream as each other and as `pump`.

**4. `.go-arch-lint.yml` structurally forbids campaign, store, identity and gateway.** The rule is one line: `mcp: { mayDependOn: [harness, engine, contract, mcp] }` (`.go-arch-lint.yml:213`), with the SDK free as vendor. *Rejected:* the honour-system convention that `internal/harness` had been running on.

*Verified to bite.* In a throwaway rsync copy: a production file importing `internal/gateway` produces `Component mcp shouldn't depend on …/internal/gateway`; a `_test.go` file importing `campaign`, `identity` and `store` produces three notices. `go-arch-lint check` on the real tree is green. So the rule holds for test files too, which is the half that usually leaks.

*What it buys.* "The MCP server bypasses the authz table" is not merely false, it is unstateable — there is no path from this package to `Authorize`, to `identity`, or to the store. Every privilege question about the agent seat reduces to a question about the gateway, which has its own tests. The project memory records this as the finding that collapsed a planned ~40-line escalation audit into one test.

*What it costs*, in four places I can point at:
- `get_state` must reproduce `cmd/vtt/state_dump.go`'s `writeDump` marshaling by hand (`marshalStateWithHead`, `read_tools.go:282`) because `internal/mcp` cannot import `cmd/vtt` — the dependency runs the other way. Two copies of one output shape, held in step only by the e2e's equality assertion.
- `--ruleset` and `--adventures-dir` cannot be read by this package at all. `cmd/vtt` loads them at boot and hands over already-resolved *text* (`Config.RulesetGuide`, `Config.AdventureGuides`). The consequence is a design one: the guide is a **startup snapshot**, not a live read from the running server. Server-authoritative guide delivery would need a new wire command and is deferred.
- `get_join_link` and `get_participants` go over HTTP to the gateway's own routes rather than calling gateway code, and re-derive the HTTP origin from the WS URL.
- `FuzzToolArgumentsBecomeTheCommandTheyName` has to state the authorization invariant on the *producing* side and explain in prose why it matters, because it cannot import `internal/gateway` to assert it directly.

**5. No jitter in the redial backoff.** *Rejected:* jittered backoff. *Reason:* the reconnect target is one gateway process behind one URL, not a fleet — there is no thundering herd to spread. Ledgered as a candidate refinement.

**6. `history` is unbounded.** *Rejected:* a ring buffer or retention policy. *Reason:* the target scale is one table-top campaign's events for one `vtt mcp` process lifetime. This is the decision most likely to need revisiting, and the comment says so.

**7. `tools.json` is delivered as bytes through `Config`, embedded by `cmd/vtt`.** *Rejected:* `internal/mcp` reading the file at runtime. *Reason:* single-binary distribution, plus it keeps this package free of filesystem access entirely. The `cp` + `check:drift` pair is what stops the embedded copy going stale.

---

## 4. Vad som visade sig fel

**The counts. All of them, repeatedly, and this is the arc's dominant defect.** The plan promised "nine tools (seven generic from tools.json + two read tools)", and on 2026-07-24 that was exact — the manifest held 7 entries, including `create_scene` and `retract_events`, both of which have since left the platform. By 2026-08-25 the server served 28 and README still said nine. The spec's §4 said nine; its own §10 amendment said TWELVE; a walkthrough note said 24; `tools_test.go` carried a lead sentence saying 16 over a running tally that ran 7→9→12→13→15→16→17 and stopped, above a list of 22. Every one of those numbers was correct on the day it was written.

Two corrections, the first of which was itself wrong:
- `73a5300` (2026-08-25) pinned README's count to the live registry with a test that builds the expected phrase from the real totals. Its own review then found that the commit message had **fabricated a stale number inside the sentence denouncing stale numbers** — it claimed a backlog entry recorded 24 and was "ALSO already wrong"; no such entry exists, and the 24 that does exist was correct when written. The same review found three more stale counts in the very file being fixed.
- `21ef370` (the next day, Patrik's ruling) deleted the count assertion entirely: state the invariant, not the count. The README now describes the *kinds* of tool; spec §10 withdraws both its numbers. And the review corrected the author's account again — he had written that the manifest→registered link was unheld and "nothing went red", having injected the fault, seen his own new test fail, and never checked what else failed. Three tests fail, two of them pre-existing. *Proving a test can fail is not proving it adds coverage.*
- `ddf2d96` (2026-08-26) swept the rest, under a rule worth preserving: a number describing the system's *current shape* is replaced by the relationship; a number recording a *change* or a *dated observation* stays. It also found that `read_tools.go` had been citing spec §4 for a sentence the spec never contained ("Both registered in the same tool table" is the *plan's* Task 2) — a misquote that survived a rewrite of §4 itself in the accuracy pass.

**The three (five) copies of the tool list — checked, and they agree today.** I measured every copy:

| copy | kind | count | guard |
|---|---|---|---|
| `tools/toolgen/main.go` `manifest` | hand-written | 22 | `TestManifestCoversAllCommandMessages` (contract → manifest) |
| `contract/gen/tools/tools.json` | generated | 22 | `check:drift` |
| `cmd/vtt/tools.json` | `cp` of the above | 22 | `check:drift` (byte-identical — verified) |
| `contract/testdata/expected_tools.json` | hand-written golden | 22 | `TestToolsMatchGolden` only — **not generated and not in `check:drift`'s pathspec** (recorded in `tools/check-no-create-scene.py:116`) |
| `internal/mcp/tools_test.go` `wantCommandToolNames` | hand-written | 22 | itself |

All five carry the same 22 names in the same order; `expected_tools.json` is byte-identical to the generated file. `internal/gateway`'s `commandRoles` also carries the same 22.

The one that "silently stopped growing before" is `wantCommandToolNames`, and the instance is recorded in its own comment: `load_adventure` reached the oneof and never reached the list, and nothing went red. That is why `TestEveryManifestCommandIsRegisteredAsATool` reads the *generated* manifest instead. The hand-written list keeps its place for the opposite direction only — a command *removed* from the contract vanishes from the manifest too, so only a hand-written set can notice it is gone.

**The fuzz target: the premise in the caller's brief is wrong, and I can show where it came from.** The project memory (`apparatus-work-is-paused.md`, item 2) records a 143-line fuzz target that was written, reviewed and dropped, whose property `authorized implies the table names it` was a tautology — `Authorize` returns nil only when `roles[p.Role]` is true, which implies `len(commandRoles[name]) > 0`, which is exactly what `HasRoleCellsForTest` reports — and which never reached its assertion: 562,043 executions, 206 interesting inputs, zero decoded, because random mutation does not synthesize valid protojson for a 22-arm oneof.

That target was **not written here.** It was aimed at `DecodeCommand`, which lives in `internal/gateway/codec.go:31`. `fcac0ac` (2026-08-27), the commit that records the drop, touches exactly one file: `internal/gateway/codec_test.go`, +12/−1. What survived is a 12-line `cmd != nil` assertion — the malformed-JSON test had discarded the command into `_`, so the failure path could have returned `&cmd` alongside its error and the whole suite would have passed.

The reason it was aimed there was itself a false claim about *this* package: that MCP is a WebSocket client, so a model's bytes arrive at `DecodeCommand`. They do not. `commandFromArgs` unmarshals the model's arguments into the *sub-message* and builds the `ClientCommand` by reflection from a valid descriptor; `internal/harness` then marshals it. `DecodeCommand` only ever sees our own encoder's output. **The MCP argument decoder is the only place in the platform a model controls bytes** — and that is what eventually produced the real target here, `ee2913e` (2026-09-14), which split `commandFromArgs` out of `handlerFor` precisely so it could be fuzzed without a live client and a connected wire.

**The measurements that did hold.** Two claims in `server.go` reproduce:
- `commandFromArgs`'s "Review ran all 22 arms against 30 hostile seeds — 660 pairs, zero violations." I counted 30 seeds and 22 arms; `go test -run Fuzz… -v` runs `seed#0` through `seed#659` — exactly 660 — all PASS.
- The depth table at `server.go:427–430`. Re-run today through `add_actor` (the deepest arm, the only command reaching `google.protobuf.Struct`):

  | | comment | re-run 2026-09-19 |
  |---|---|---|
  | depth 1000 | 6 KB, 1 ms, accepted | 5.9 KB, 3 ms, accepted |
  | depth 9000 | 54 KB, 16 ms, accepted | 52.8 KB, 19 ms, accepted |
  | depth 20000 | 120 KB, 12 ms, refused, max recursion depth | 117.3 KB, 13 ms, `proto: exceeded max recursion depth` |
  | depth 200000 | 1.2 MB, 11 ms, refused | 1.17 MB, 13 ms, `proto: exceeded max recursion depth` |

  The two structural claims underneath it also hold: `contract/vtt/v1/events.proto` imports only `struct.proto` and `timestamp.proto`, `commands.proto` imports only `events.proto`, and `google.protobuf.Any` appears nowhere in the contract — which is what keeps the decoder off the path CVE-2024-24786 made an infinite loop.

**Escaped defects, found by review inside the arc.** `1e13730` (Task 1 review): the shutdown defers were registered in the wrong order, so close-before-cancel allowed one spurious redial attempt on the stdio-EOF path; and the package doc claimed a direct `internal/engine` import that did not yet exist. `363f9d0` (workflow-level final review, 13 confirmed findings, the last commit of the arc):
- **Critical:** the invite token leaked to stderr. Fixed at source in `harness.Dial` so all consumers inherit the redaction.
- `add_actor`'s "fabrication trap": none of `Actor`'s fields are proto3 `optional`, so the derived `required` list forced every one of them and an LLM caller could not tell "must supply" from "happens to be non-optional on the wire" — it fabricated a plausible `moduleId`/`controllerId` instead of sending nothing. Fixed by adding `requiredOverride`/`fieldDocs` to toolgen.
- The server instructions said int64 fields always serialize as JSON strings. `get_state`'s body is the exception and the instructions did not say so. Now they do, and `TestServerInstructionsScopeTheInt64AsStringRuleToNotGetState` holds it.
- `get_events_since` accepted unknown keys and leaked `encoding/json`'s Go struct field path (`getEventsSinceArgs.afterSequence`) to the caller. Now `DisallowUnknownFields` plus `describeArgsDecodeError`, naming the JSON argument the caller actually sent.
- `wireConnected` added to both read tools' output: without it a frozen snapshot was indistinguishable from live state.
- A TOCTOU in `redial`: `harness.Dial`'s ctx bounds only the handshake, so a dial succeeding in the same instant the context is cancelled would be installed by `setClient` with nothing left running to close it. `harnessDial` became a package var purely so an internal test can reproduce that race deterministically.

**A plan constraint the arc broke, in its own last commit.** The plan's Global Constraints say "tools.json is consumed AS COMMITTED (no regeneration in this branch; contract untouched — drift/breaking trivially green)." `363f9d0` regenerates it: it touches `tools/toolgen/main.go`, `contract/gen/tools/tools.json`, `cmd/vtt/tools.json` and `contract/testdata/expected_tools.json`. The `add_actor` fix could not be made anywhere else. Nothing records the constraint as waived.

---

## 5. Spår

**Range: `1039555..363f9d0` — six consecutive commits, no branch, no merge.**

The chain the brief suggested does not apply here, and I verified why rather than assuming it.

1. `git log --oneline --reverse -- internal/mcp/` gives the first commit touching the territory: `eb6d213`.
2. `git log --merges --ancestry-path eb6d213..main --format='%h %ad %s'` piped to `tail -5` returns the *next* sub-projects' merges (`83be590` ruleset-interpreter, 2026-07-25, and later) — there is no merge whose second parent reaches `eb6d213`. The arc has no merge commit of its own.
3. `git log --format='%h %p %s'` from `363f9d0` shows a strictly single-parent chain back through `021c5b9 → 3cd9052 → 1e13730 → eb6d213 → 1039555 → a91b8b0`. `a91b8b0` is the previous sub-project's spec amendment, so `1039555` ("docs: MCP gateway spec and plan (sub-project 6)") is the arc's first commit; the commit after `363f9d0` is `0434d5e`, "docs: ruleset interpreter spec and plan (sub-project 5a)", which opens the next arc.

So: **work landed straight on `main`**, as the brief anticipated. Six commits, 2026-07-24 21:14 to 2026-07-25 00:05 — under three hours wall-clock.

| commit | date | plan step |
|---|---|---|
| `1039555` | 07-24 21:14 | spec + plan |
| `eb6d213` | 07-24 21:45 | Task 1 — server core, generic dispatch, arch-lint rule, SDK pin |
| `1e13730` | 07-24 21:47 | Task 1 review one-liners |
| `3cd9052` | 07-24 22:09 | Task 2 — read tools |
| `021c5b9` | 07-24 22:48 | Task 3 — `cmd/vtt/mcp.go`, e2e, README, embedded tools.json |
| `363f9d0` | 07-25 00:05 | Task 4 — workflow-level final review, 13 findings |

What is *not* in the range but belongs to the territory's later history, and is cited above because the arc's claims were corrected there: `73a5300`, `21ef370`, `ddf2d96` (the count corrections, 2026-08-25/26), `fcac0ac` (the dropped gateway fuzz target, 2026-08-27), `ee2913e` (the real MCP fuzz target, 2026-09-14), and `7bf4ff6` (the door tools, 2026-08-11).

Sources used beyond git: the plan and spec above, `docs/adr/007-contract-format.md` and `009-airtight-tdd-protocol.md` (referenced, not quoted), `README.md`, `Taskfile.yml`, `.go-arch-lint.yml`, and the project memory at `~/.claude/projects/-Users-patriklager-dev-vtt-platform/memory/`. `docs/verification-debt.md` (uncommitted, sub-project 14) contains no MCP entries — checked. There are no task-report files for this arc in the repo; the briefs the comments cite (`task-1-brief.md`, `task-2-brief.md`, `task-3-brief.md`) do not exist in the tree, so every claim attributed to them is unverifiable at source and was checked against the code instead.

---

## 6. Kommentarsblock denna rapport friar

Measured: `internal/mcp` production files carry **439 comment lines against 731 code lines**, with **14 blocks of ten or more consecutive comment lines** — reproducing the brief's numbers exactly. Below, each block, and for each the sentences that must **stay**. The brief's warning is correct: not one block is pure history. Every one wraps a rule in a story, and deleting the block wholesale would delete the rule.

**1. `internal/mcp/server.go:1–14` — package doc: what this package is and what it may import.**
*Frees:* the parenthetical dating `internal/engine` to the read tools ("a direct import as of the read tools, read_tools.go — … not just reached transitively through harness").
*Stays:* this package is a wire client and adds **zero privilege**; every tool call becomes a `ClientCommand` the gateway judges with its ordinary authz table, stamped with the agent's participant id. It may import only `vttv1`, `internal/harness`, `internal/engine`, the MCP SDK and stdlib — never `gateway`, `campaign`, `identity` or `store`. `.go-arch-lint.yml` enforces it, **test files included**.

**2. `server.go:84–108` — `Config`: why the guides arrive as text.**
*Frees:* "ruleset-interpreter Task 6", "adventure-format Task 4", "server-authoritative guide delivery is a later-plan candidate — see the task report".
*Stays:* `ToolsJSON` is raw bytes; `Server` never reads the filesystem. `RulesetGuide` and `AdventureGuides` are **already-loaded text**, never a directory path and never a loaded `Ruleset` struct, because this package may not import `internal/rules` or `internal/adventure` and no wire command exists to fetch a guide instead. `cmd/vtt` validates at boot, so a bad `--ruleset` fails loud immediately.

**3. `server.go:128–137` — the `history` field: the single source, and its bound.**
*Frees:* "Unbounded for v1"; "retention/bounding policy is a ledgered future concern, not this task's (see the task report)".
*Stays:* `history` is ascending by sequence with no duplicates; it is the one source both read tools read, via `historySnapshot`, **never a second connection**. It is unbounded, so a long-lived process grows with the log — the assumption being made is one table-top campaign per process lifetime.

**4. `server.go:284–294` — the TOCTOU guard in `redial`.**
*Frees:* "final review Fix 6c".
*Stays:* `harnessDial`'s context bounds only the handshake — the returned `Client`'s lifetime is independent of it. `Run`'s shutdown defer already read `currentClient()` as nil (that is why `redial` is running), so a client installed after cancellation would never be closed. Close the orphan here and bail out.

**5. `server.go:368–398` — `commandFromArgs`: the trust boundary, and the invariant that matters.**
*Frees:* "NOT OF THE WHOLE MCP SURFACE, which a first version of this said"; "SPLIT OUT SO IT CAN BE FUZZED. Inside handlerFor it needed a live client…"; "Review ran all 22 arms against 30 hostile seeds — 660 pairs, zero violations."
*Stays:* this is the trust boundary of the **command** surface only — the read tools and `get_adventure_guide` decode their own arguments with `encoding/json` and never pass through here, each with its own strict decoder. The invariant is not "does not panic": **the command returned must be the one the tool named**, because `internal/gateway` decides authorization by reading which oneof arm is populated, so arguments that could select a different arm would be judged by another command's rule. Why it cannot fail as written: protojson is handed a message of `fd`'s own type and holds no reference to `ClientCommand`, so it cannot address another arm; `Set` on a oneof member replaces the wrapper whole.

**6. `server.go:409–419` — absent arguments, and what protojson does with `null`.**
*Frees:* "Every existing call site in this package passes an empty map … so the old suite never exercised this line — TestAToolCalledWithNoArgumentsGetsAnEmptyObject does."
*Stays:* an absent argument object is an **empty** one, not an error — a tool whose fields are all optional is legitimately called with nothing, and a real MCP client may omit `arguments` entirely. A `null` for any field that is not a `google.protobuf.Value` is discarded and decoding continues, so `{"tokenId":null}` is accepted and yields an empty `token_id`; validating empty ids is the gateway's job, not this seam's.

**7. `server.go:423–440` — what bounds a hostile argument blob.**
*Frees:* nothing outright — the dated measurement table is a change-record under the repo's own `ddf2d96` rule, and it reproduces (section 4). Keep it with its date.
*Stays:* what keeps this bounded is the **contract**, and no gate asserts either half. There is no `google.protobuf.Any` anywhere in the contract — only `struct.proto` and `timestamp.proto` are imported — which is what keeps the decoder off the path CVE-2024-24786 made an infinite loop. Nesting is bounded by protowire's recursion limit, decremented per message, so a hostile blob is refused in milliseconds rather than overflowing a stack, which Go does not recover from. **Add an `Any` to a command arm, or a deeper recursive type, and both guarantees change materially.**

**8. `server.go:450–459` — `handlerFor`: the one generic handler.**
*Frees:* "THE ARGUMENT HALF LIVES IN commandFromArgs since 2026-09-14 … This function is what remains: the parts that genuinely need a session."
*Stays:* `fd` identifies which oneof field this tool name maps to; everything else is identical regardless of which command it is. **There is deliberately no per-command switch.**

**9. `internal/mcp/tools.go:35–54` — `buildDispatch`: the name→arm match and the symmetric check.**
*Frees:* "self-review requirement, task-1-brief.md"; the worked example tracing `move_token` back through toolgen's manifest.
*Stays:* tool names in tools.json are the oneof field's proto names, and this is the **only** place a tool name resolves to a command shape. Both directions of disagreement — a tools.json name with no oneof field, a oneof field with no tools.json entry — are startup errors; they must agree exactly. And the division of labour: toolgen's own `TestManifestCoversAllCommandMessages` guarantees contract → manifest; *this* check catches drift between the tools.json the package was **configured** with and the `vttv1` build it is **running against**, e.g. a stale embedded copy.

**10. `internal/mcp/read_tools.go:20–29` — the pagination limits.**
*Frees:* "Binding decisions (this task's own, beyond what the brief pinned explicitly — flagged for reviewer sign-off)".
*Stays:* an omitted or non-positive `limit` defaults to 50 — 0 is "unset", not "return nothing". Anything above 200 is **silently clamped, not rejected**: a pagination cap is a server-side resource bound, not a caller-input failure worth surfacing to the LLM as a broken call. Both halves are promised to the model in the tool description and asserted by `TestGetEventsSinceDefaultLimitAppliesWhenOmitted` and `TestGetEventsSinceLimitClampedToMaximum`.

**11. `read_tools.go:132–144` — `handleGetState`: the dump contract, verbatim and duplicated.**
*Frees:* '"The dump contract verbatim" (task-2-brief.md) means exactly this marshaling approach'.
*Stays:* folds this server's own accumulated history via `harness.Fold`, never a second connection. Returns the **same JSON shape `vtt state dump` prints** — the folded `*engine.State` marshaled through `encoding/json`, its exported fields' own Go/json-tag names verbatim (Actor uses its protobuf snake_case tags), **not** re-shaped to protojson camelCase — with `headSequence` added as a sibling top-level key. Deliberately duplicated rather than shared, because `internal/mcp` cannot import `cmd/vtt`: cmd depends on mcp, never the reverse.

**12. `internal/mcp/door_tools.go:16–36` — why the two door read tools exist and why they use HTTP.**
*Frees:* "Before them the door was advertised and undrivable, which is the failure T6 exists to catch, one layer up"; the `#45` issue number.
*Stays:* `rotate_join_link` deliberately returns no secret — **a secret must not travel the frame channel every participant reads**. **No new authority:** both routes are already gated to dm/agent by the gateway (`metadata.go`'s `joinLinkRoles` and `participantRoles`, both verified to exist), and the MCP server presents the same token, so the gateway makes the decision it already makes for the DM console. **HTTP rather than the WebSocket, because that is where the answers live:** the roster and the link are identity state, not campaign events, and neither has a frame; inventing one that carried a shared secret is exactly what `rotate_join_link` declines to do.

**13. `door_tools.go:52–66` — `httpOriginFrom`.**
*Frees:* nothing. This block is rule end to end.
*Stays:* the HTTP origin is **derived** from the WebSocket URL rather than configured, so the two cannot drift. **The query is dropped**, because the URL the CLI passes carries the token and carrying it into every metadata request would put a credential in the request line, where it reaches logs and proxies; it travels in the `Authorization` header instead. An untranslatable scheme is an **error, not a passthrough** — answering `http://…` for an `ftp://` input would send the token somewhere nobody meant.

**14. `door_tools.go:132–144` — `readMetadata`.**
*Frees:* the aside comparing a second shape to "the drift the tools.json golden exists to prevent, reintroduced by hand" — keep the rule, the analogy can go.
*Stays:* the body is returned **verbatim and not re-shaped**: the gateway already decided what these routes say and the DM console renders exactly this, so a second shape here is a second contract to keep in step. A refusal is a **tool error, not content**, so the agent sees a failure instead of narrating an empty roster to its user. 401 and 403 are reported **distinctly from a transport failure**, because they mean different things to whoever is debugging: the wrong token, versus a token whose role this route does not admit. (The `json.Valid` check just below carries its own rule and is outside this block: a 200 carrying non-JSON means a proxy rewrote the body, and handing that to an agent as "the roster" is how a tool reports confident nonsense.)

**One near-miss worth naming.** `read_tools.go:95–103`, the doc comment on `GoRegisteredToolNames`, is nine lines — just under the threshold — and carries the single most load-bearing rule in the package's maintenance story: *there is ONE place to update, because the count of MCP tools once lived at four sites that each carried their own copy, and a single rename shipped a stale one three times.* It should not be freed by this report.

---

## Caveats

- No task-report or brief files for this arc exist in the tree. Claims in comments attributed to `task-1-brief.md`, `task-2-brief.md`, `task-3-brief.md` and "the task report" are unverifiable at source; I checked each against the code and said so where it mattered.
- The arc's five code commits have empty bodies apart from the attribution trailer; `363f9d0`'s three-line body is the only prose the arc's own commits carry about its review. The detailed reasoning in section 4 about the counts comes from commits *after* the arc (`73a5300`, `21ef370`, `ddf2d96`), which are the ones that wrote it down.
- I did not run `task check` in full (the mutation and drift gates are minutes-to-hours). I ran `go test ./internal/mcp/...` (green), the fuzz seed corpus (660/660 pass), `go-arch-lint check` (green), the arch-lint bite proof in a throwaway copy (bites, test files included), and the protojson depth measurement (reproduces).
- The repo was left untouched: `git status --short` still shows the same 17 uncommitted files and HEAD is still `48e3fe2`.