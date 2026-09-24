# SPEC-007: The wire contract between server and client

## Status

Accepted. The schemas are authored as protobuf in `contract/vtt/v1/`, package
`vtt.v1`, as a buf module rooted at `contract/`. Generated output is committed
under `contract/gen/go` and `contract/gen/ts`, and the MCP tool definitions
under `contract/gen/tools`.

Held by `contract/contract_test.go`, `contract/roundtrip_test.go`,
`contract/events.test.ts` and the fixtures in `contract/testdata/`. Gated by
`task check:drift` and `task check:breaking`, both inside `task check`.

Both generator plugins are local, run through `buf.gen.yaml`, and pinned
outside it: `protoc-gen-go` by go.mod's tool directive, `protoc-gen-es` by
package.json and bun.lock. Three field numbers are absent from their oneofs — 11 and 16 from
`ClientCommand.command`, 17 from `Envelope.payload` — and no `reserved`
statement marks them.

THIS RECORD IS WHERE THE WIRE CONVENTIONS LIVE. `contract/README.md` points
here, and the proto files keep only what warns whoever edits the line. ADR-007
is frozen and holds the road to the decision — the three formats compared, the
scorecard, and why protobuf was chosen.

## Principles served

vtt-platform has no blueprint. This section is owed and cannot be written
honestly until one exists — naming principles here would invent them from a
single record, which is the failure a blueprint is written to prevent.

## How it works

**Protobuf is the single authored source of truth.** `.proto` files under
`contract/` are authored; Go, TypeScript and the MCP tool definitions are
generated from them and committed. A consumer never authors a type that
crosses the boundary. The files sit under `contract/vtt/v1/` because buf's
`PACKAGE_DIRECTORY_MATCH` lint rule requires the package path in the directory
path.

### The wire conventions a consumer must know

**int64 serializes as a JSON string.** `Envelope.sequence` is `"42"`, not `42`.
Generated consumers handle it; hand-rolled ones must.

**The event envelope has no `type` field.** The `oneof payload` inlines one key
per variant — `{"tokenMoved": {...}}`. Generated consumers get compiler-checked
discrimination: a Go type switch, a TypeScript `case`/`value` pair. Hand-rolled
consumers switch on presence of one key among N. `contract/testdata/` holds the
executable form.

**`Actor.module_data` is an opaque `google.protobuf.Struct`.** The contract
never inspects it. Rule modules own its shape and validate it with their own
content schemas. Go access walks `structpb.Value`.

**proto3 `optional` is the optionality annotation.** A field marked `optional`
is an optional parameter in the derived MCP tool; an unannotated field is
required. proto3 cannot express caller-facing optionality on its own, so this
annotation is what `tools/toolgen` reads.

### The ordering contract

Frames are delivered in one order per connection, and events are ordered among
THEMSELVES by sequence.

A `CommandResult` is NOT ordered against the `Envelope`s that same command
produced. The result is enqueued by the command loop and the events by the
broadcast pump — two independent producers. Either can arrive first, and which
one wins varies with load.

SO A CLIENT MUST NOT CORRELATE BY ARRIVAL POSITION. Correlate a result by its
`request_id` and events by their sequence; a result's `sequence` field names the
first event that command produced.

The one exception is `catch_up_head`, positional by definition: always the first
frame on a connection. `frameQueue` in `internal/gateway/server_test.go` is the
shape a correct reader takes — demultiplex the kinds, never discard the one you
were not currently asking for.

### What the connection opens with

`CatchUpHead` is sent once, first, carrying the highest sequence already queued
as this connection's catch-up backlog. A client wanting a point-in-time snapshot
reads until it has seen `head_sequence`; one wanting a live tail ignores the
frame. A `head_sequence` of 0 means the log was empty at subscribe time.

`PresenceSnapshot` follows immediately, sent unconditionally INCLUDING WHEN
EMPTY, so a joining client never infers who is online from silence.

### What appends to the log, and what does not

The log only goes forward. Nothing retracts, and nothing rewrites what is
already in it.

These commands APPEND NOTHING: `set_viewpoint`, `set_join_door`,
`rotate_join_link`, `promote_participant`. A viewpoint is a view preference, a
door is operational state, and a role lives in the participants table — none is
a fact about the campaign, and a replay must not reconstruct them.

`PresenceChanged` and `PresenceSnapshot` are FRAMES, never Envelopes. Who
happened to be online is not campaign history.

`DISCONNECTED` is emitted when a participant's LAST connection goes, not their
first: invite tokens are reusable, so one participant may hold two.

### What a command refuses

`grant_actor_control` REQUIRES `kind`, and a grant that omits it is refused
rather than defaulted. proto3 cannot mark a field required, so an omitted enum
arrives as UNSPECIFIED — indistinguishable from a caller that deliberately said
nothing — and either default is wrong in a way that matters.

`revoke_actor_control` requires both `actor_id` and `participant_id`.

`set_join_door` carries `admit_limit`: how many people THIS OPENING may admit.
A non-positive value means `identity.DefaultAdmitLimit`, because absent and 0
arrive identically under protojson and "admit nobody" is undebuggable rather
than dangerous. On a close it is stored, and `GET /api/join-link` reports it as
the limit; a shut door admits nobody whatever it says.

`promote_participant` accepts ONLY "player" or "spectator". A shared join link
mints spectators, and letting promotion reach "dm" or "agent" would make that
link a path to full authority in two steps.

`set_join_door`, `grant_actor_control` and `promote_participant` are DM and
agent only. `revoke_actor_control` is open to a player as well, who may revoke
only control they themselves hold (`authorizeSelfRevoke`).

### The commands that yield a batch

`remove_actor` and `load_map` each produce an ordered batch accepted or
rejected atomically, never a single event.

`remove_actor` emits one `TokenRemoved` per token of that actor, in token-id
order, then the `ActorRemoved`. This is a correctness requirement rather than a
convenience: both folds refuse a token whose actor they do not know, so an
`ActorRemoved` that left this actor's tokens standing would leave a world whose
own introductions no longer fold.

`load_map` emits one `SceneCreated` carrying resolved terrain and objects, then
one `TokenPlaced` per declared placement. A map never creates actors; a
placement's `actor_id` must already exist in campaign state or the whole batch
is rejected.

### Evolution

Additive changes only: new fields with new numbers, new messages, new oneof
variants. Renames, deletions, and number or type changes are breaking.

A retired field number is never reused. NOTHING ENFORCES THIS TODAY:
`ClientCommand.command` has gaps at 11 and 16 and `Envelope.payload` at 17, and
neither file carries a `reserved` statement, so `buf` cannot object if a future
addition takes one of them. What the gaps were is not recorded in the schema.

`task check:breaking` runs `buf breaking` against `main` using FILE rules. It is
triggered by `contract/RELEASED`: absent, it reports what `buf breaking` would
have objected to and exits 0; present, it enforces. Creating that file is a
deliberate commit of its own — it is the moment additive-only turns on for good.

`task check:drift` regenerates and requires an empty diff. A committed `gen/` is
what makes that gate real.

### The losing formats

`contract-spike/` holds the three prototypes ADR-007 compared. It is retained as
evidence, excluded from coverage gates, and never imported by production code.
Its Taskfile targets — `generate:proto`, `generate:jsonschema`,
`generate:openapi` — exist to reproduce the evidence and stay outside
`task check`.

JSON Schema is present too, as toolgen's OUTPUT format, because MCP
`inputSchema` is JSON Schema. Protobuf is the single AUTHORED source of truth,
not the only format in the architecture.

## Consequences

**A consumer that reads this record needs nothing else.** int64-as-string, the
oneof key and `Struct` on module data are the three things a hand-rolled
consumer gets wrong, and they are here rather than spread across a README, a
comment and a decision record.

**The drift gate reads every diff as real**, because both plugin versions are
pinned by lockfiles: a regenerate differs only when the schema or a pinned
version changes.

**`check:breaking` reports rather than enforces today**, because
`contract/RELEASED` does not exist. Every breaking change until that file is
created is permitted, and the gate's output is advice.

**The proto files still carry doc comments, and that is deliberate.** What
remains there warns whoever edits the line — why an enum is not a bool, why a
validation cannot live in `ToEvent`, why `TokenHidden` is not `RemoveToken`.
Those readers are not reading this record.

## Requirements

None allocated for this record; no ticket carries them yet.
