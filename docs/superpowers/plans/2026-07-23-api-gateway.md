# API Gateway & Permissions Implementation Plan (sub-project 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the platform's wire surface per the approved spec (`docs/superpowers/specs/2026-07-23-api-gateway-design.md`): protojson WebSocket protocol, four-role authorization, invite-token identity, the `vtt` CLI (ADR-008), the four carry-forward fixes, and a three-client live exit scenario.

**Architecture:** `internal/identity` (participants table, invite tokens — own SQLite handle, not event-sourced) and `internal/gateway` (frame codec, single authz table, WebSocket server fanning out pre-marshaled bytes) over `internal/campaign`; `cmd/vtt` cobra shell with ultra-thin commands. Contract grows commands/frames (imperative names) mirroring events (past-tense names). Store notification decouples from Append (explicit `Notify`, per-subscriber sequence dedupe) so subscribers always observe State ≥ any event they've seen.

**Tech Stack:** Go 1.26; `github.com/coder/websocket` (maintained; pinned exact at implementation, version recorded); `github.com/spf13/cobra` (pinned exact; viper deferred until a config file exists — noted in ADR-008); existing pinned contract pipeline.

## Global Constraints

- Repo `/Users/patriklager/dev/vtt-platform`; branch `feat/api-gateway` from `main`.
- Review-before-commit flow (Patrik-approved); controller commits `CLAUDE_REVIEW_DONE=1` after task review. Controller stages post-report.
- **Task 1 gate caveat:** `check:drift` HEAD-relative → red pre-commit on regenerated files is EXPECTED; Task 1 verifies with vet/tests/breaking pre-commit; controller runs full `task check` post-commit.
- Contract changes ADDITIVE ONLY; `check:breaking` green throughout.
- Naming convention (binding): commands are imperative (`CreateScene`), events past-tense (`SceneCreated`) — never confuse the two.
- Vocabulary discipline in all new code (semgrep gate now scans test files too).
- New deps pinned EXACT and recorded: coder/websocket, spf13/cobra. Nothing else new.
- `contract-spike/` frozen. All 9 existing gates stay green; arch-lint config grows per §9 of the spec.

---

### Task 1: Contract — commands, frames, ownership, participant identity + toolgen map/list support

**Files:**
- Modify: `contract/vtt/v1/events.proto` (Actor + Envelope fields), `contract/vtt/v1/commands.proto` (commands + frames)
- Modify: `tools/toolgen/main.go` (map/list/Struct schema support + 6 manifest entries), `contract/testdata/expected_tools.json`
- Modify: `contract/roundtrip_test.go`, `contract/events.test.ts`; Create: `contract/testdata/client_command.json`, `contract/testdata/server_frame_result.json`
- Modify: `contract/gen/**` (regenerated)

**Interfaces:**
- Produces: `Actor.ControllerId`, `Envelope.ParticipantId`, `vttv1.ClientCommand` (oneof tags 10–16), `vttv1.ServerFrame{oneof: result|event}`, `vttv1.CommandResult`, six command messages `CreateScene/AddActor/PlaceToken/StartSession/EndSession/RetractEvents`; TS schemas for all. Tasks 4–7 depend on these exact names.

- [ ] **Step 1: Branch** — `git checkout -b feat/api-gateway main`

- [ ] **Step 2: events.proto additions** — `Actor` gains `string controller_id = 7;` (comment: participant who may act as this actor; empty = DM/agent only). `Envelope` gains `string participant_id = 6;` (comment: who caused this event; stamped by the gateway).

- [ ] **Step 3: commands.proto additions** (after MoveTokenResponse):

```proto
// Commands are imperative; the events they become are past-tense.
message CreateScene {
  string scene_id = 1;
  string name = 2;
  int32 grid_width = 3;
  int32 grid_height = 4;
}

message AddActor {
  Actor actor = 1;
}

message PlaceToken {
  string token_id = 1;
  string scene_id = 2;
  string actor_id = 3;
  GridPosition position = 4;
}

message StartSession {
  string name = 1;
}

message EndSession {}

message RetractEvents {
  int64 from_sequence = 1;
  int64 to_sequence = 2;
  string reason = 3;
}

message ClientCommand {
  string request_id = 1;
  oneof command {
    MoveTokenRequest move_token = 10;
    CreateScene create_scene = 11;
    AddActor add_actor = 12;
    PlaceToken place_token = 13;
    StartSession start_session = 14;
    EndSession end_session = 15;
    RetractEvents retract_events = 16;
  }
}

message CommandResult {
  string request_id = 1;
  bool ok = 2;
  string error = 3;
  int64 sequence = 4;
}

// The server->client frame; the oneof key is the frame discriminator.
message ServerFrame {
  oneof frame {
    CommandResult result = 1;
    Envelope event = 2;
  }
}
```

- [ ] **Step 4: toolgen map/list/Struct support** (the sub-project-2 carry-forward comes due: `AddActor` embeds `Actor`, which has two map fields and a Struct). In `schemaFor`, REPLACE the `IsList/IsMap` panic guard with real handling, checked BEFORE the Kind switch:

```go
	if f.IsMap() {
		props[name] = map[string]any{
			"type":                 "object",
			"additionalProperties": valueSchema(f.MapValue()),
		}
		if !isOptional(f) {
			required = append(required, name)
		}
		continue
	}
	if f.IsList() {
		props[name] = map[string]any{"type": "array", "items": valueSchema(f)}
		if !isOptional(f) {
			required = append(required, name)
		}
		continue
	}
```

with a new helper `valueSchema(f protoreflect.FieldDescriptor) map[string]any` handling scalar kinds + MessageKind recursion, and a special case in the MessageKind path: if the message's full name is `google.protobuf.Struct`, emit `{"type": "object"}` (open object — the contract never inspects module data). Refactor the existing per-field switch to use `valueSchema` so scalar logic exists once. Keep the panic default for genuinely unhandled kinds.

- [ ] **Step 5: manifest +6.** Add entries (message, tool name, description, descriptor) for: `vtt.v1.CreateScene`→`create_scene`, `vtt.v1.AddActor`→`add_actor`, `vtt.v1.PlaceToken`→`place_token`, `vtt.v1.StartSession`→`start_session`, `vtt.v1.EndSession`→`end_session`, `vtt.v1.RetractEvents`→`retract_events` (descriptions one sentence each, DM-voiced, e.g. "Create a new scene with a grid." / "Retract a range of events from the record with a stated reason."). NOTE: the manifest-completeness test keys on the `"Request"` suffix and will NOT auto-cover these; extend `TestManifestCoversAllCommandMessages` to also require manifest entries for every message that appears as a `ClientCommand` oneof variant (walk `ClientCommand`'s descriptor fields — that IS the command registry now).

- [ ] **Step 6: regenerate + golden.** `task generate:contract`; rewrite `contract/testdata/expected_tools.json` for 7 tools (write it by intent from the schemas: `add_actor`'s inputSchema must show `attributes`/`resources` as objects with additionalProperties and `moduleData` as a bare object; `controller_id` appears as optional?—NO: `controller_id` is a plain proto3 field, not `optional`-annotated, so it lands in `required` — that is correct and deliberate for tool calls: the agent should always say who controls an actor it creates, empty string allowed). Run `go test ./tools/toolgen/` until golden matches intent.

- [ ] **Step 7: fixtures + round-trip extensions.** `client_command.json` (a MoveToken command with requestId + `reason` present) and `server_frame_result.json` (a ServerFrame carrying a CommandResult with `"sequence": "12"` — int64-as-string). Add Go round-trip funcs + TS cases for both (same pattern as existing). Update the two existing fixtures? NO — `actor.json`/envelope fixtures stay untouched (new fields are optional-absent; round-trip unaffected — that's additive evolution working).

- [ ] **Step 8: pre-commit verify** — vet/go test/bun test/`task check:breaking` green; `git status` shows only the expected files. (Drift red pre-commit expected; controller runs full check post-commit.)

- [ ] **Step 9: Commit point** — `feat: contract commands, frames, ownership + toolgen map/list support`

---

### Task 2: Store notify decoupling (carry-forward fix, touches committed sub-project-2 code)

**Files:**
- Modify: `internal/store/store.go`, `internal/store/subscribe.go`, `internal/store/subscribe_test.go`, `internal/campaign/campaign.go`
- Modify: `internal/campaign/campaign_test.go` (if any test depended on notify timing)

**Interfaces:**
- Produces: `(*Store).Notify(env *vttv1.Envelope)` (explicit, post-apply); `Append` NO LONGER notifies; per-subscriber sequence dedupe (`lastSeq`) making Notify idempotent per subscriber and closing the persist-vs-subscribe race. Campaign calls `Notify` after live apply (Append) and after rebuild (Undo). Gateway (Task 5) relies on: an observer that receives event N and then calls `State()` always sees state ≥ N.

- [ ] **Step 1: Failing test first** — in `subscribe_test.go`, add `TestNotifyAfterApplyOrdering_NoDuplicateOnRace`: append via raw store (no notify), Subscribe(0, 8) — catch-up delivers the event; then call `s.Notify(sameEnv)` — assert NOTHING further arrives (dedupe by sequence). And `TestAppendDoesNotNotify`: subscribe first, append, assert no delivery until explicit `Notify`. Run — FAIL (Notify undefined; Append still notifies).

- [ ] **Step 2: Implement.** `subscriber` gains `lastSeq int64`. Subscribe sets `lastSeq` to the last catch-up sequence during the locked preload. `notifyLocked` skips `env.Sequence <= sub.lastSeq`, else delivers and updates `lastSeq`. Remove the notify call from `Append`; add:

```go
// Notify fans env out to subscribers. Callers invoke it AFTER the event's
// effects are observable (campaign: after live apply) so a subscriber that
// sees event N can always read state >= N. Idempotent per subscriber via
// sequence dedupe, which also closes the subscribe-between-persist-and-notify
// race.
func (s *Store) Notify(env *vttv1.Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifyLocked(env)
}
```

- [ ] **Step 3: Campaign call sites.** `Append`: after successful live `engine.Apply`, call `c.log.Notify(env)`. `Undo`: after successful `rebuildLocked`, call `c.log.Notify(marker)`. (Poison paths: no notify — a poisoned campaign must not advertise the event.)

- [ ] **Step 4: Update existing tests** that assumed Append-notifies (subscribe tests calling raw `s.Append` then expecting delivery: insert `s.Notify(env)` where the OLD semantic was intended, or assert the new one where the test's purpose was ordering). Campaign-level subscriber tests should pass UNCHANGED (campaign now notifies internally) — if one fails, that is a real regression: stop and investigate.

- [ ] **Step 5: Full verify** — `go test ./internal/... -race -count=1`; property test counts UNCHANGED (notify timing is invisible to it); `task check` green.

- [ ] **Step 6: Commit point** — `fix: decouple store notification from append; notify after live apply`

---

### Task 3: internal/identity — participants, invites, revocation

**Files:**
- Create: `internal/identity/identity.go`, `internal/identity/identity_test.go`

**Interfaces:**
- Produces: `identity.Open(path string) (*DB, error)` (own SQLite handle on the campaign file; creates `participants` table); `Role` type with `RoleDM/RoleAgent/RolePlayer/RoleSpectator` + `ParseRole`; `(*DB).CreateInvite(name string, role Role, controls []string) (token string, id string, err error)` (32 random bytes, base64url; stores SHA-256 hash; token returned ONCE); `(*DB).Verify(token string) (*Participant, error)` (constant-time hash compare via `crypto/subtle`; revoked/unknown → error); `(*DB).Revoke(id string) error`; `Participant{ID, Name string, Role Role, Controls []string}`; `(*DB).Close()`.
- Schema: `participants(id TEXT PRIMARY KEY, display_name TEXT, role TEXT, controls TEXT /*JSON array*/, token_hash BLOB UNIQUE, revoked INTEGER DEFAULT 0)`.

- [ ] **Step 1: Failing tests** — behavioral list, every case: create→verify round-trip returns matching Participant; token is shown once and NOT recoverable from the DB (assert stored value ≠ token); wrong token rejected; revoked token rejected after Revoke; ParseRole accepts exactly the four roles, rejects others; two invites → distinct tokens/ids; Verify is constant-time-shaped (assert implementation uses subtle.ConstantTimeCompare — white-box grep is acceptable here via a code comment contract, the behavioral test just proves correctness); identity DB coexists with a store.Store handle on the SAME file (open both, both work — one focused test).
- [ ] **Step 2: Implement** (hash lookup: SELECT by token_hash is NOT constant-time-comparable — instead SELECT candidate row by hash equality is fine since the hash is not secret-dependent-timing-sensitive... DESIGN NOTE, binding: compute `h := sha256(token)`, query `WHERE token_hash = ?` — the DB index comparison is on the HASH, which is safe (attacker cannot learn the token from hash-equality timing); still wrap the final confirmation in `subtle.ConstantTimeCompare(h, row.hash)` for defense in depth and to make the contract explicit).
- [ ] **Step 3: Verify + `task check` (arch-lint: add `identity: { in: internal/identity }` with `mayDependOn: [identity]` self-only — no contract dep needed) — green.** `go test ./internal/identity/ -race`.
- [ ] **Step 4: Commit point** — `feat: identity — participants, hashed invite tokens, revocation`

---

### Task 4: internal/gateway — codec, authz table, command→event conversion (pure core)

**Files:**
- Create: `internal/gateway/authz.go`, `internal/gateway/convert.go`, `internal/gateway/codec.go` + `_test.go` for each

**Interfaces:**
- Produces (consumed by Task 5): `Authorize(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error` — the ONE authz function backed by a table `var commandRoles = map[string]map[identity.Role]bool` keyed by oneof field name, plus the player-ownership special case (MoveToken: look up token→actor→ControllerId == p.ID); `ToEvent(cmd *vttv1.ClientCommand, p *identity.Participant) (*vttv1.Envelope, error)` — builds the past-tense event envelope (EventId = new random ULID-ish id, ParticipantId = p.ID, ActorRole = string(p.Role), OccurredAt = now) EXCEPT RetractEvents which returns a sentinel (`ErrIsRetraction` + parsed range) because campaign.Undo owns marker construction; `EncodeFrame(*vttv1.ServerFrame) ([]byte, error)` / `DecodeCommand([]byte) (*vttv1.ClientCommand, error)` via protojson.

**Authz table (verbatim, the heart of §4):**

```go
// commandRoles is THE authorization policy (spec §4). Player MoveToken has an
// additional ownership check in Authorize; everything not listed is denied.
var commandRoles = map[string]map[identity.Role]bool{
	"move_token":     {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"create_scene":   {identity.RoleDM: true, identity.RoleAgent: true},
	"add_actor":      {identity.RoleDM: true, identity.RoleAgent: true},
	"place_token":    {identity.RoleDM: true, identity.RoleAgent: true},
	"start_session":  {identity.RoleDM: true, identity.RoleAgent: true},
	"end_session":    {identity.RoleDM: true, identity.RoleAgent: true},
	"retract_events": {identity.RoleDM: true, identity.RoleAgent: true},
}
```

- [ ] **Step 1: Failing tests.** Authz: table-driven over EVERY command × all four roles (7×4 = 28 cells, generated from the table itself plus explicit expected outcomes written out — do NOT derive expectations from the same map under test; write the 28 expectations literally). Player-ownership: player moves own-controlled token OK; other's token denied; token of controllerless actor denied; unknown token denied (authz layer, before campaign validation). Convert: each command produces the right event variant with ParticipantId/ActorRole stamped and EventId non-empty/unique; RetractEvents returns the sentinel; unknown/empty oneof errors. Codec: ClientCommand JSON round-trip; ServerFrame with each oneof arm; malformed JSON errors cleanly.
- [ ] **Step 2: Implement.** Arch-lint: `gateway: { in: internal/gateway, mayDependOn: [contract, campaign, engine, identity, gateway] }` (engine for State in ownership check).
- [ ] **Step 3: Verify + task check green.**
- [ ] **Step 4: Commit point** — `feat: gateway core — authz table, command conversion, frame codec`

---

### Task 5: internal/gateway — WebSocket server

**Files:**
- Create: `internal/gateway/server.go`, `internal/gateway/server_test.go`
- Modify: `go.mod` (+`github.com/coder/websocket` pinned exact — record the version)

**Interfaces:**
- Produces: `gateway.New(c *campaign.Campaign, ids *identity.DB) *Server`; `(*Server).Handler() http.Handler` (routes `/ws`, `/healthz`). Connection flow (decided): both parameters in the URL — `/ws?token=<invite>&after=<seq>` — so authentication AND catch-up position are settled before the upgrade; `ids.Verify` runs BEFORE `websocket.Accept` (bad/revoked token → plain HTTP 401, no upgrade); then catch-up + live via `campaign.Subscribe(after, 256)`. Every outbound envelope is marshaled ONCE per event to bytes and fanned to all sockets (the immutability carry-forward). Inbound: DecodeCommand → Authorize (against `c.State()`) → ToEvent → `c.Append` (or `c.Undo` for the retraction sentinel) → CommandResult with sequence; authz/validation failures → CommandResult ok=false with error, connection STAYS open; malformed frame → close that connection only.
- `gatewayBuffer = 256` named constant with the overflow-closes-socket comment.

- [ ] **Step 1: Failing tests** (real server, `httptest.NewServer`, real coder/websocket clients; behavioral list): healthz 200; connect with bad/revoked token → HTTP 401 before upgrade... (design: verify BEFORE `websocket.Accept`); valid connect + `after=0` → receives full history then live; two clients both receive a DM's accepted command as an Envelope frame; player ownership denial arrives as ok=false Result AND no event broadcast; spectator command → ok=false; agent RetractEvents → works, marker broadcast to all; sequence in CommandResult matches the broadcast envelope's; State-visibility guarantee: on receiving event N a client calling... (clients can't call State — SKIP, that guarantee is Task 2's and tested there); malformed JSON closes only that client, others live.
- [ ] **Step 2: Implement** (one goroutine per connection reading commands; one per connection writing from its subscribe channel + a per-conn result queue — keep it simple: writes serialized through a per-conn mutex or a single writer goroutine fed by a channel; document the choice).
- [ ] **Step 3: `go test ./internal/gateway/ -race` + task check green.**
- [ ] **Step 4: Commit point** — `feat: gateway WebSocket server — auth, catch-up, fan-out, live commands`

---

### Task 6: cmd/vtt — the binary (ADR-008)

**Files:**
- Create: `cmd/vtt/main.go`, `cmd/vtt/serve.go`, `cmd/vtt/invite.go`, `cmd/vtt/revoke.go`, `cmd/vtt/version.go` + `cmd/vtt/cli_test.go`
- Create: `docs/adr/008-vtt-cli-shell.md`
- Modify: `go.mod` (+cobra pinned), `.go-arch-lint.yml` (`cmd` component)

**Interfaces:**
- Produces: `vtt serve --campaign <file> --addr <addr>`; `vtt invite --campaign <file> --name <n> --role <r> [--controls id,id]` (prints token ONCE + participant id); `vtt revoke --campaign <file> --id <participant-id>`; `vtt version`. Every RunE ≤30 lines, delegating to internal packages (ckeletin pattern; enforce by eyeball + a comment, not tooling, this round).
- ADR-008 content: the §2.3 decision verbatim from the spec (pattern adopted: cobra + ultra-thin commands; viper deferred until a config file exists; updateable framework layer rejected — dueling sources of truth), Status Accepted, referencing Peiman's ckeletin-go as origin.

- [ ] **Step 1: Failing test** — `cli_test.go` drives the cobra root command in-process (no exec): `version` prints something version-shaped; `invite` on a temp campaign prints a token and `Verify` accepts it; `revoke` then makes `Verify` reject it; `serve` is NOT integration-tested here (Task 7 does it end-to-end) beyond flag validation (missing --campaign errors).
- [ ] **Step 2: Implement + ADR-008.** Arch-lint: `cmd: { in: cmd/**, mayDependOn: [gateway, identity, campaign, contract, cmd] }`; nothing gains `cmd` in its deps.
- [ ] **Step 3: task check green** (semgrep scans cmd/? — vocabulary rule's paths.include is `internal/` only; extend `.semgrep/vocabulary.yml` include to `[internal/, cmd/]` in this task).
- [ ] **Step 4: Commit point** — `feat: vtt CLI — serve, invite, revoke, version (ADR-008)`

---

### Task 7: Exit scenario — three clients over live WebSockets

**Files:**
- Create: `internal/gateway/scenario_test.go`

**Interfaces:**
- Consumes everything. Produces spec §10's exit criteria, executable — sub-project 4's harness seed one layer up.

- [ ] **Step 1: Write the scenario test** (behavioral, every step asserted): boot a real server on a temp campaign (in-process: gateway.New + httptest; invites minted via identity directly): DM token, player token controlling `act-lera` only, agent token, spectator token. DM: StartSession, CreateScene, AddActor×2 (`act-ursus` controllerless, `act-lera` controlled by the player participant), PlaceToken×2. Player: moves lera's token OK; moves ursus's token → ok=false AND no broadcast (all clients assert absence). Agent: moves ursus's token OK; retracts that move (RetractEvents) → all four clients receive the marker. Spectator: attempts StartSession → ok=false. DM: EndSession. All clients disconnect; player reconnects with `after=0` → full catch-up equals what a fresh subscriber sees; final assertion: the sequence of event frames received live by the DM client equals the catch-up sequence (order + ids), and every envelope carries the correct `participant_id` for who caused it.
- [ ] **Step 2: Run + full verify** — `go test ./internal/gateway/ -race -count=1`; full `task check`; `go test ./... -race -count=1` once.
- [ ] **Step 3: Commit point** — `test: three-role live WebSocket exit scenario`

---

## After this plan

Sub-project 3 complete: merge via finishing-a-development-branch. Carry-forwards for sub-project 4 (simulation harness): the scenario test is the harness's template — harness = scripted participants driving this exact wire API from a CLI (`vtt client` shape decided there); MCP gateway (sub-project 6) consumes the SAME tools.json the contract now generates for all seven commands.
