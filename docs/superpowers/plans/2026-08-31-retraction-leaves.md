# The Log Only Goes Forward — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove undo from the platform entirely, add the two forward corrections it was covering, and require improvised scenes to declare their terrain.

**Architecture:** Retraction is removed in dependency order — fixtures first, so nothing exercises it; then each consumer; then the contract last, which is also what proves the new pre-release breaking gate works. Two commands replace what undo covered, on the rule that removal means "no longer part of the world going forward" and never "this never was". `create_scene` gains `mapdef`'s completeness rule so the improvised path obeys the same standard as the authored one.

**Tech Stack:** Go 1.26 (`internal/campaign`, `internal/engine`, `internal/gateway`, `internal/harness`, `internal/mcp`), TypeScript + Bun (`client/`), protobuf via buf (`contract/`), Task for gates.

**Spec:** `docs/superpowers/specs/2026-08-30-retraction-leaves-design.md`

## Global Constraints

- **Airtight TDD (ADR-009).** Tests first, RED observed and recorded before the change exists. Behavioral RED over compile-failure RED wherever a stub can compile. For a REMOVAL, the RED is inverted: the test that proves the thing is gone must fail while it is still present.
- **`task check` is the single gateway.** Never weaken a gate to pass it. `task check:fast` is an inner-loop convenience and never satisfies the gate. It does NOT run `check:ts-coverage`, `check:breaking`, `check:drift`, or either mutation gate.
- **Citations in comments name things, never line numbers** (Patrik, 2026-08-30): a function, a test, a constant, a named arm; `[anchor:kebab-name]` for a genuinely nameless target. Mutation-adjudication coordinates are the sole exception — `file:line:col` is a mutant's identity.
- **One fold.** `engine.Apply` stays the only code that changes game state.
- **No game-system vocabulary in platform code** (semgrep enforces).
- **Adding or removing lines in a gated package moves mutation adjudication keys.** The twelve gated Go packages are `PACKAGES` in `tools/check-mutation.py`; `client/src` is Stryker's glob. After any change there, run the relevant mutation gate and re-key from ITS OUTPUT — never by counting lines. Re-key, never delete: the reasons are the expensive part.
- **A new file under `client/src` needs a floor in `tools/ts-coverage-thresholds.txt`** in the same change, or `check:ts-coverage` reds.
- **Rebuild the embedded bundle** after any `client/src` change: `task build:client`, and stage `cmd/vtt/webdist`. The commit hook will not catch a stale one.
- **Goldens are never machine-regenerated.** `cmd/vtt/scenario_goldens_test.go` has no `-update` flag by deliberate decision. `state.json` is hand-derived from the scenario definition; `stream.json` is recorded from the server; their agreement is evidence only because neither was produced from the other.

---

### Task 1: The breaking gate learns when it applies

**Files:**
- Modify: `Taskfile.yml` (the `check:breaking` task)
- Modify: `docs/adr/007-contract-format.md`
- Test: `tools/check_breaking_marker_test.sh` (create)

**Interfaces:**
- Produces: the contract `contract/RELEASED` — absent means pre-release (report, do not fail), present means released (fail as today). Every later task depends on this existing, because Task 7 deletes messages from the proto and cannot pass otherwise.

- [ ] **Step 1: Write the failing test**

Create `tools/check_breaking_marker_test.sh`, executable, asserting both directions against a temporary marker:

```bash
#!/usr/bin/env bash
# The breaking gate must report-without-failing before release and fail after.
# Patrik's amendment (ADR-007, 2026-08-30): additive-only binds from the first
# release others can run, not before.
set -euo pipefail
cd "$(dirname "$0")/.."

fail() { echo "check_breaking_marker_test: $1" >&2; exit 1; }

[ -f contract/RELEASED ] && fail "contract/RELEASED exists; this test manages it and will not clobber yours"

# Pre-release: must exit 0 even when buf would object.
if ! task check:breaking >/tmp/cbm-pre.log 2>&1; then
  fail "pre-release run must not fail; see /tmp/cbm-pre.log"
fi
grep -q "pre-release" /tmp/cbm-pre.log || fail "pre-release run must SAY it is reporting rather than enforcing"

# Released: must enforce. Restore the marker's absence whatever happens.
trap 'rm -f contract/RELEASED' EXIT
: > contract/RELEASED
task check:breaking >/tmp/cbm-post.log 2>&1 || true
grep -q "pre-release" /tmp/cbm-post.log && fail "released run must not claim to be pre-release"

echo "ok  check:breaking honours contract/RELEASED in both directions"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tools/check_breaking_marker_test.sh`
Expected: FAIL on the pre-release grep — today's gate prints no such line, because it has no notion of release.

- [ ] **Step 3: Teach the gate the marker**

In `Taskfile.yml`'s `check:breaking`, wrap the existing `buf breaking` invocation. Keep every line of the existing `REF` resolution and its comment — that comment records a real incident (the gate hard-failing on the repo's first pull request) and must not be lost. Add around the invocation:

```bash
if [ -f RELEASED ]; then
  go tool buf breaking --against "../.git#ref=$REF,subdir=contract"
else
  echo "check:breaking: pre-release (contract/RELEASED absent) — REPORTING, not enforcing."
  echo "  ADR-007's additive-only rule binds from the first release others can run."
  echo "  Anything buf objects to below is real and will fail once that file exists."
  go tool buf breaking --against "../.git#ref=$REF,subdir=contract" || true
fi
```

Note the working directory: the existing code does `cd contract` first, so the test is `-f RELEASED`, not `-f contract/RELEASED`.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tools/check_breaking_marker_test.sh` — expect the ok line.
Run: `task check:breaking` — expect exit 0 and the pre-release banner.

- [ ] **Step 5: Amend ADR-007**

Add a dated amendment paragraph in the file's existing style — §5 of ADR-010 shows the house form, `**AMENDED <date> — <what was wrong>**`. It must say: additive-only binds from the first release others can run; before that a breaking change is permitted with a stated reason; the gate encodes the same trigger via `contract/RELEASED`; and creating that file is a deliberate commit that turns the rule on for good. Do not delete the original decision text.

- [ ] **Step 6: Verify and stop**

Run: `task check:fast`. Leave the work in the working tree; the controller reviews and commits.

---

### Task 2: No scenario retracts

**Files:**
- Modify: `scenarios/denials.json`, `scenarios/three-role-exit.json`
- Modify: `scenarios/goldens/denials/{state.json,stream.json}`, `scenarios/goldens/three-role-exit/{state.json,stream.json}`

**Interfaces:**
- Consumes: nothing.
- Produces: a corpus in which no scenario issues `retractEvents`, so every later task can remove machinery without a fixture exercising it.

- [ ] **Step 1: Read what the two scenarios use retraction FOR**

`denials.json` uses it twice as an **authorization** test — a player and a spectator each attempt `retractEvents` and expect `deniedContaining: "not authorized"`. Those steps do not test retraction; they test that the authz table refuses a command to a role. When the command ceases to exist they lose their subject entirely and are deleted, not rewritten — there is no "denied" for a command the contract does not define.

`three-role-exit.json` retracts a `TokenMoved` as part of its narrative and then reconnects. Its retraction step is deleted; the reconnect and every assertion after it stay, and their expected sequences shift.

Write down, before editing, which assertions in each file depend on a sequence number that will move.

- [ ] **Step 2: Edit the two scenario definitions**

Delete the retraction steps. In `denials.json` delete the two `retractEvents` steps entirely. In `three-role-exit.json` delete its retraction step and renumber any assertion that names a sequence.

- [ ] **Step 3: Re-derive `state.json` BY HAND for both**

Do not run the server and copy its output. `cmd/vtt/scenario_goldens_test.go` states the rule and the reason: there is deliberately no `-update` flag, because a regenerate-on-demand switch is how a golden stops being a claim anyone checked. `state.json` is derived from the scenario DEFINITION by reading it and working out the resulting state.

For each of the two, read the edited scenario, work out the end state, and write it. Then say in your report what changed relative to the old `state.json` and why — a diff you cannot explain is a golden that stopped asserting.

- [ ] **Step 4: Re-record `stream.json` for both**

This half IS a recording of the server. Run the scenario and capture the normalized stream, then verify it against the hand-derived `state.json` from Step 3. Their agreement is the evidence; if they disagree, one of them is wrong and you must say which before changing either.

- [ ] **Step 5: Run the golden gates**

```bash
go test ./cmd/vtt/ -run 'TestScenario' -count=1
go test ./internal/harness/ -run 'TestFoldGoldenCorpus' -count=1
```
Expected: PASS. Report both.

- [ ] **Step 6: Verify and stop**

Run `task check:fast`. Do not commit.

---

### Task 3: The client forgets how to retract

**Files:**
- Delete: `client/src/undo.ts`, and its test
- Modify: `client/src/fold.ts` (the two-pass `fold`), `client/src/view/feed.ts`, `client/src/view/dm.ts`, `client/src/commands.ts`
- Modify: `client/test/command-surface.test.ts` (the `COMMAND_SURFACE` table), `client/test/fold-*.test.ts`, `client/test/dm-view.test.ts`, `client/test/feed.test.ts`
- Modify: `tools/ts-coverage-thresholds.txt` (remove the `undo.ts` row)

**Interfaces:**
- Produces: `fold(envelopes: Envelope[]): State` — single pass, same signature.

- [ ] **Step 1: Write the failing test**

Add to `client/test/command-surface.test.ts`:

```ts
test("no command builder can retract, because the platform cannot", () => {
  // Patrik, 2026-08-30: retraction leaves the platform. A retraction's purpose
  // is to make something not have happened, and it cannot do that — the player
  // read the log and knows what it said. This asserts the ABSENCE, so it is
  // written before the removal and must fail now.
  const retractors = Object.keys(commands).filter((k) => /retract/i.test(k));
  expect(retractors).toEqual([]);
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bun test client/test/command-surface.test.ts`
Expected: FAIL with `["retractEvents"]`.

- [ ] **Step 3: Collapse the fold to one pass**

`client/src/fold.ts`'s `fold` currently walks the log twice — pass 1 builds a `retracted` set from `eventsRetracted` markers, pass 2 applies everything not in it. Delete pass 1, delete the `retracted` set and its `continue`, delete the marker's own skip. Rewrite the function's doc comment: it currently explains the two passes and cites `internal/harness/fold.go`'s matching shape, and both halves stop being true.

- [ ] **Step 4: Remove the rest of the client's retraction surface**

`undo.ts` and its test are deleted outright. Remove its row from `tools/ts-coverage-thresholds.txt` or `check:ts-coverage` fails on a floor for a file that does not exist. Remove `retractEvents` from `commands.ts` and from `COMMAND_SURFACE`. Remove the Undo controls from `view/dm.ts` — both the single-event and range buttons, including the `retract-events` action strings and the confirmation dialog. Remove the retraction rendering from `view/feed.ts`.

- [ ] **Step 5: Run the tests**

```bash
bun test client/test
bunx tsc --noEmit -p client/tsconfig.json
task check:ts-coverage
task build:client
```
All must pass; `check:ts-coverage` is the one `check:fast` cannot see.

- [ ] **Step 6: The mutation gate, because `client/src` moved**

Deleting a file and collapsing a function shifts every Stryker adjudication below it in the files you touched. Run the container gate and re-key from its output:

```bash
task check:ts-mutation
python3 tools/check-ts-mutation.py tools/ts-mutation-equivalents.txt
```

This takes about an hour and the tree must not change while it runs. If it reports `ADJUDICATION MOVED`, re-key those entries from the gate's own coordinates — never by counting lines — and never delete one. If an entry's mutant is gone because the code is gone, remove that entry and say so explicitly in your report.

- [ ] **Step 7: Verify and stop**

Report the gate verdict. Do not commit.

---

### Task 4: The harness forgets how to retract

**Files:**
- Modify: `internal/harness/fold.go` (the two-pass `Fold`), `internal/harness/engine.go` (retraction steps)
- Modify: the harness tests that drive retraction

**Interfaces:**
- Produces: `harness.Fold` — single pass, same signature.

- [ ] **Step 1: Write the failing test**

Add a test asserting the absence, in the package's existing style: given a log containing no retraction machinery, `Fold` applies every envelope in order and no code path exists that skips by sequence. Concretely, assert that the package exports no retraction helper and that `Fold`'s result for a fixed log equals applying every event — the test must fail while the two-pass exists, so make it assert on behaviour that differs, not on names alone.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/harness/ -run Fold -count=1 -v`. Record the failure.

- [ ] **Step 3: Collapse `Fold` and remove the scenario steps**

Single pass. Remove `engine.go`'s retraction step handling and the `runReconnectStep` accounting that exists for it. Correct `fold.go`'s doc comment, which describes the two passes and is cited BY `client/src/fold.ts` — Task 3 removed that citation, so check it agrees.

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/harness/... -count=1
go test ./internal/harness/... -race -count=1
```

- [ ] **Step 5: Verify and stop**

`internal/harness` is NOT one of the twelve gated mutation packages — confirm that against `PACKAGES` in `tools/check-mutation.py` rather than trusting this line. Run `task check:fast`. Do not commit.

---

### Task 5: The campaign forgets how to retract

**Files:**
- Modify: `internal/campaign/campaign.go` (`foldEvents`, `retractedSet`, `Undo`), `internal/campaign/foldprefix.go`
- Modify: the campaign tests that drive undo

**Interfaces:**
- Produces: `foldEvents(events []*vttv1.Envelope) (*engine.State, error)` — one parameter, not two.

- [ ] **Step 1: Write the failing test**

The behavioural RED: a log containing an `EventsRetracted` marker must now fold it like any other event rather than skipping it, and `Undo` must not exist. Write the test that a fold of a log applies every envelope, and assert `campaign` exposes no undo entry point.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/campaign/ -count=1 -v`. Record the failure.

- [ ] **Step 3: Remove undo and collapse the fold**

Delete `Undo` and `retractedSet`. `foldEvents` currently reads:

```go
func foldEvents(events []*vttv1.Envelope, retracted map[int64]bool) (*engine.State, error) {
	st := engine.NewState()
	for _, env := range events {
		if retracted[env.Sequence] {
			continue
		}
		if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
			continue
		}
		if err := engine.Apply(st, env); err != nil {
```

Both `continue` blocks go, and so does the second parameter:

```go
func foldEvents(events []*vttv1.Envelope) (*engine.State, error) {
	st := engine.NewState()
	for _, env := range events {
		if err := engine.Apply(st, env); err != nil {
``` `foldprefix.go` exists specifically to answer retraction being retroactive, and its doc comment says so; with retraction gone, `FoldPrefix` collapses into the same single pass. Read that file's comment before deleting anything: it names a test that pins the two-pass behaviour, and that test goes with it.

Update `foldEvents`'s own doc comment, which currently says it is shared by `rebuildLocked` and "Undo's dry-run viability check". After this task it has one caller.

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/campaign/... -count=1
go test ./internal/... -count=1
```

- [ ] **Step 5: The Go mutation gate, because a gated package moved**

`internal/campaign` is in `PACKAGES`. Run `task check:mutation`. Note that if any earlier task changed `contract/gen/go`, every package's fingerprint changes and the whole set re-measures — about 50 minutes. Otherwise it re-measures campaign alone. Re-key any `ADJUDICATION MOVED` from the gate's output; delete an entry only when its mutant is gone with the code, and say which.

- [ ] **Step 6: Verify and stop**

Report the gate verdict. Do not commit.

---

### Task 6: The gateway and the agent forget how to retract

**Files:**
- Modify: `internal/gateway/server.go` (the `retract_events` handler), `internal/gateway/authz.go` (its `commandRoles` row and any switch arm), `internal/gateway/convert.go` if it names the command
- Modify: `tools/toolgen/main.go` and regenerate `cmd/vtt/tools.json` + `contract/gen/tools/tools.json`
- Modify: the gateway tests that drive retraction, including `authzCases`

**Interfaces:**
- Produces: an agent tool surface with no retraction tool.

- [ ] **Step 1: Write the failing test**

Assert the tool surface has no retraction entry, in the style `tools/toolgen`'s tests already use — and assert the invariant rather than a count, per this repo's standing rule: no tool whose name matches `retract` exists.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./tools/toolgen/ ./internal/gateway/ -count=1 -v`. Record the failure.

- [ ] **Step 3: Remove the handler, the authz row and the tool**

Delete the handler, its `commandRoles` row, its `authzCases` cells, and the toolgen entry. Regenerate with `task generate:contract` and stage what it produces.

- [ ] **Step 4: Run the tests and the drift gate**

```bash
go test ./internal/gateway/... ./tools/... -count=1
task check:drift
```

- [ ] **Step 5: The Go mutation gate**

`internal/gateway` is gated and you have changed it. Run `task check:mutation` and re-key from its output as in Task 5.

- [ ] **Step 6: Verify and stop**

Do not commit.

---

### Task 7: The contract forgets how to retract

**Files:**
- Modify: `contract/vtt/v1/commands.proto` (delete `RetractEvents` and its oneof arm), `contract/vtt/v1/events.proto` (delete `EventsRetracted` and its oneof arm)
- Regenerate: `contract/gen/**` via `task generate:contract`
- Modify: `contract/` tests and any fixture under `contract/testdata` naming retraction

**Interfaces:**
- Consumes: Task 1's `contract/RELEASED` marker, without which this task cannot pass `check:breaking`.

- [ ] **Step 1: Write the failing test**

In the `contract` suite, assert no message or oneof arm matching `retract` exists in the generated descriptors — a check over `ClientCommandSchema` and `EnvelopeSchema`'s oneof case names, in the style `command-surface.test.ts` uses for the client.

- [ ] **Step 2: Run it to verify it fails**

Run: `bun test contract`. Expected: FAIL naming `retractEvents` and `eventsRetracted`.

- [ ] **Step 3: Delete from the proto and regenerate**

Delete both messages and both oneof arms. Do NOT add `reserved` for the freed field numbers: this is a pre-release removal and the numbers are free to reuse; reserving them would preserve a compatibility claim the amendment says does not yet apply. Run `task generate:contract`.

- [ ] **Step 4: Confirm the gate reports rather than fails**

Run: `task check:breaking`
Expected: exit 0, with the pre-release banner AND buf's objections printed beneath it. Paste both. This is the task that proves Task 1's gate does what it claims — a gate that reports nothing here would mean the marker had simply switched the check off.

- [ ] **Step 5: Run everything that reads the contract**

```bash
go build ./... && go test ./... -count=1
bun test contract client/test
task check:drift
```

- [ ] **Step 6: Verify and stop**

Do not commit.

---

### Task 8: `remove_token` takes a piece off the board

**Files:**
- Modify: `contract/vtt/v1/commands.proto` (`RemoveToken`), `contract/vtt/v1/events.proto` (`TokenRemoved`), regenerate
- Modify: `internal/engine/apply.go` (the fold arm), `internal/gateway/{server.go,authz.go,convert.go}`, `tools/toolgen/main.go`
- Modify: `client/src/commands.ts`, `client/src/fold.ts`, `client/test/command-surface.test.ts`, `client/test/commands.test.ts`
- Modify: `contract/roundtrip_test.go` and a new `contract/testdata` fixture

**Interfaces:**
- Produces: `removeToken(tokenId: string): ClientCommand` in `client/src/commands.ts`; `TokenRemoved{token_id}` on the wire.

- [ ] **Step 1: Read the precedent before writing anything**

`place_token` → `TokenPlaced` is the same shape one direction over. Read its command message, its event, its fold arm in `engine.Apply` AND in `client/src/fold.ts`, its authz row, its toolgen entry, and its round-trip fixture. This task is that set, for removal. Do not invent a shape; mirror the one that exists.

`TokenRemoved` is NOT `TokenHidden`. `TokenHidden` is projection-only and means "you cannot see it"; this means the piece is not there. Say so in the new message's doc comment, because the next reader will ask.

- [ ] **Step 2: Write the failing tests**

Three: the engine fold arm removes the token from state and errors on an unknown token id; the client fold arm does the same; and the wire shape round-trips, asserted exactly via `toJson` as `commands.test.ts`'s `openDoor`/`closeDoor`/`loadMap` tests do — those three are the idiom, and their own comment explains why they assert the exact object rather than a subset.

- [ ] **Step 3: Run them to verify they fail**

Record each failure.

- [ ] **Step 4: Implement**

Contract, regenerate, both fold arms, gateway handler, authz row in `commandRoles` and a cell in `authzCases`, toolgen entry, client builder, `COMMAND_SURFACE` entry with its surface and action.

- [ ] **Step 5: Run the tests and the gates**

```bash
go test ./... -count=1
bun test client/test contract
task check:drift && task check:breaking && task check:ts-coverage
task build:client
```

- [ ] **Step 6: Verify and stop**

Do not commit. The mutation gates run in Task 11.

---

### Task 9: `remove_actor` cascades, atomically

**Files:**
- Modify: `contract/vtt/v1/commands.proto` (`RemoveActor`), `contract/vtt/v1/events.proto` (`ActorRemoved`), regenerate
- Modify: `internal/engine/apply.go`, `internal/gateway/{server.go,authz.go}`, `tools/toolgen/main.go`
- Modify: `client/src/{commands.ts,fold.ts}`, `client/test/{command-surface,commands}.test.ts`
- Modify: `contract/roundtrip_test.go` and a fixture

**Interfaces:**
- Consumes: `TokenRemoved` from Task 8 — the cascade emits it.
- Produces: `removeActor(actorId: string): ClientCommand`; `ActorRemoved{actor_id}`.

- [ ] **Step 1: Understand why the cascade is not optional**

`engine.Apply` and `client/src/fold.ts` both reject a token whose actor is unknown, in almost the same words. An `ActorRemoved` that left tokens behind would produce a log that no longer folds. So the command produces an ordered batch: a `TokenRemoved` for each of that actor's tokens, then `ActorRemoved`. `load_map` is the precedent for a command that yields an ordered batch accepted or rejected atomically — read `LoadMap`'s doc comment and its handler.

Control grants need no event: `controller_ids` is a field on the actor, so removing the actor removes them with it.

- [ ] **Step 2: Write the failing tests**

Four: the batch is emitted in order (tokens then actor); the log still folds after it; a partial application is impossible — if any part is rejected, nothing is appended; and an actor with no tokens yields a batch of one.

- [ ] **Step 3: Run them to verify they fail**

Record each failure.

- [ ] **Step 4: Implement**

Mirror `load_map`'s atomic-batch handling. The fold arm for `ActorRemoved` removes the actor; it must not need to remove tokens, because the batch already did — and if it did, the two folds would disagree about whether a token can outlive its actor.

- [ ] **Step 5: Run the tests and gates**

As Task 8's Step 5.

- [ ] **Step 6: Verify and stop**

Do not commit.

---

### Task 10: A room must declare its squares

**Files:**
- Modify: `internal/gateway/create_scene_validate.go` (`validateCreateSceneTerrain`)
- Modify: all 8 scenario definitions under `scenarios/` that call `createScene`
- Modify: all 8 golden directories under `scenarios/goldens/`, both files each, plus `session-zero/projections/`

**Interfaces:**
- Consumes: nothing.
- Produces: `create_scene` refuses a grid with any undeclared square.

- [ ] **Step 1: Write the failing tests**

Two, on the boundary: a scene whose tiles cover every square of `grid_width × grid_height` is accepted; a scene one square short is refused, and the refusal names a missing square so the DM can fix it. `mapdef`'s rule is the one being mirrored — `format.go` says that if tiles are declared they must hold an entry for every square, and `docs/map-format.md` says there is no implicit fallback anywhere.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/gateway/ -run CreateScene -count=1 -v`.

- [ ] **Step 3: Implement the rule**

Extend `validateCreateSceneTerrain`. It already validates tile KINDS against `mapdef`'s exported vocabulary rather than a restated literal — follow that instinct for completeness too, and do not hand-roll a second copy of a rule `mapdef` already owns.

- [ ] **Step 4: Give every scenario its terrain**

This is the bulk of the sub-project. Eight scenario definitions call `createScene`; each now needs a complete tile map for its declared grid. Keep grids small — a scenario's grid exists to exercise the platform, not to be a level.

- [ ] **Step 5: Re-derive all 8 `state.json` BY HAND**

Every `SceneCreated` in the corpus now carries terrain, so every golden state changes. `state.json` is hand-derived from the definition and there is deliberately no `-update` flag, for the reason `cmd/vtt/scenario_goldens_test.go` gives: a regenerate-on-demand switch is how a golden stops being a claim anyone checked.

Work one scenario at a time. For each, report what changed and why. A diff you cannot explain is a golden that stopped asserting.

- [ ] **Step 6: Re-record all 8 `stream.json`, and `session-zero/projections/`**

Recorded from the server, then checked against the hand-derived state. Their agreement is the evidence; if any pair disagrees, say which is wrong before changing either.

- [ ] **Step 7: Run the golden gates**

```bash
go test ./cmd/vtt/ -run 'TestScenario' -count=1
go test ./internal/harness/ -run 'TestFoldGoldenCorpus' -count=1
```

- [ ] **Step 8: Verify and stop**

Do not commit.

---

### Task 11: The citation convention, and the whole gate

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Write the convention into `CLAUDE.md`**

Add it to the non-negotiable rules, in that file's voice, with the reason attached — the reason is what makes it survive:

> Cite a **name** — a function, a test, a constant, a named arm. For a target with no name, place an `[anchor:kebab-name]` at it and cite that string. A bare `file.go:123` in prose is out. Mutation-adjudication coordinates are the sole exception: `file:line:col` is a mutant's identity, generated by the gate.
>
> A stale line number fails SILENTLY — it still looks valid while pointing at the wrong code. A deleted anchor fails LOUDLY, because grep returns nothing. A wrong answer becomes no answer.

State that converting the repo's existing bare citations is not part of this rule's arrival, and that a gate enforcing it (every citation resolves to exactly one anchor, no duplicates, no orphans) is available but unbuilt.

- [ ] **Step 2: Prove the removal is complete WITH A GATE, not a search**

The spec's exit criterion 1 says "proven by a gate rather than a search", and it
means it: a grep somebody ran once proves nothing about tomorrow. Add a check to
`task check:invariants`' neighbourhood — a small script in `tools/`, wired into
the Taskfile beside the other `check:` tasks, that fails when any `retract`
identifier survives:

```bash
#!/usr/bin/env bash
# Retraction left the platform (Patrik, 2026-08-30). A retraction's purpose is
# to make something not have happened, and it cannot do that. This gate is what
# stops it coming back one helper at a time.
set -euo pipefail
cd "$(dirname "$0")/.."
hits=$(grep -rin 'retract' internal/ client/src/ cmd/ contract/ scenarios/ tools/   --include='*.go' --include='*.ts' --include='*.proto' --include='*.json' --include='*.py'   | grep -v '/gen/' | grep -v webdist | grep -v 'check-no-retraction' || true)
if [ -n "$hits" ]; then
  echo "check:no-retraction: retraction is not part of this platform; found:" >&2
  echo "$hits" >&2
  exit 1
fi
echo "check:no-retraction: clean"
```

Write it, wire it into `Taskfile.yml` as its own `check:` task and into `check`
itself, and run it. If it reports hits, each one is either a real leftover or a
deliberate historical reference — decide which, out loud, for every hit, and
narrow the gate rather than the truth.

- [ ] **Step 3: Run the whole gate, from cold**

```bash
go clean -cache   # the mutation gate refuses below 16 GiB free and a full run costs ~7
task check
```

Budget: up to ~50 minutes for `check:mutation` (every package re-measures, because `contract/gen/go` changed) and about an hour for `check:ts-mutation`. The tree must not change while either runs.

- [ ] **Step 4: Adjudicate what the gates find**

Every surviving mutant is killed with a test or adjudicated with a stated observable. Never adjudicate to make a gate green. If you believe a mutant is equivalent, try to falsify your own claim first by constructing a state that distinguishes it — three equivalence claims were examined in the previous sub-project and all three were wrong.

- [ ] **Step 5: Verify and stop**

Report the full gate output. Do not commit.

---

## Merge gate

`task check` green from cold; the spec's eight exit criteria walked one at a time with the result recorded; and the `contract/RELEASED` marker deliberately left absent, with its meaning stated in the merge commit so the day it appears is a decision rather than an accident.
