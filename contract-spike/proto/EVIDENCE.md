# Protobuf contract prototype — evidence

Spike for the shared VTT event contract (Task 3 of 6, `spike/contract-format`). Schema:
`contract/v1/events.proto`. Generated with `buf generate` via remote plugins (no local
`protoc`). Round-tripped against the shared fixtures in `../fixtures/` in both Go and
TypeScript.

## Codegen quality (Go/TS)

Both outputs generated cleanly from `buf generate` using remote plugins
(`buf.build/protocolbuffers/go`, `buf.build/bufbuild/es`) — no local `protoc` install
needed.

- `gen/go/contract/v1/events.pb.go` — 805 lines, idiomatic generated Go: one struct per
  message, getters, `protoreflect` descriptor plumbing, `oneof` rendered as an interface
  (`isEnvelope_Payload`) with a wrapper struct per variant. Field names become
  `UpperCamelCase` Go fields (`TokenId`, `SceneId`) from `snake_case` proto fields —
  standard, no surprises.
- `gen/ts/contract/v1/events_pb.ts` — 310 lines. `protoc-gen-es` v2 generates a
  *schema-descriptor* style API (`TokenMovedSchema`, `create(Schema)`, `fromJson`/`toJson`
  free functions from `@bufbuild/protobuf`) rather than classes with methods — a
  deliberate v2 design choice (tree-shakeable, no class boilerplate), but it means the
  test file has to import a *schema* object per message and pass it to generic
  `fromJson`/`toJson` functions, which loses type inference across a heterogeneous list
  (hence the `as any` casts already present in the brief's own `events.test.ts`).
- **Friction:** the default `buf.build/bufbuild/es` output target is `js+dts`
  (`events_pb.js` + `events_pb.d.ts`), *not* a single `.ts` file, even though the brief
  (and most buf/connect docs examples) assume a `.ts` file exists at
  `gen/ts/contract/v1/events_pb.ts`. Fixed forward by adding `opt: target=ts` to the
  `bufbuild/es` plugin entry in `buf.gen.yaml`, which produces the single `.ts` file the
  brief and `events.test.ts`'s import path expect. Import-path resolution would actually
  have worked either way (Bun/TS resolve an extension-less specifier to the `.js`/`.d.ts`
  pair just as well as to a `.ts` file) — this is a config-surface friction, not a
  functional break, but worth flagging: the plugin's *default* target diverges from the
  natural expectation of "buf generate → `.ts` file."

## JSON mapping frictions

All four fixture round-trip tests (`TokenMoved`, `AttackRolled`, `Actor`,
`MoveTokenRequest`) pass byte-for-byte-semantically (via `reflect.DeepEqual` on decoded
maps) in both Go (`protojson`) and TypeScript (`fromJson`/`toJson`) with **zero** schema
changes needed — the proto field names map to the fixtures' `camelCase` JSON keys
automatically (protojson's default `json_name` derivation from `snake_case` is exactly
`camelCase`, matching the fixtures as authored).

The one real friction, surfaced by the **envelope probe** (not the round-trip fixtures,
none of which contain an `int64` field): `int64 sequence` on `Envelope` serializes to
**JSON string** `"42"`, not a JSON number, under protojson (see wire-shape output below).
This is protojson spec behavior (int64/uint64/fixed64/sfixed64 all serialize as strings to
avoid precision loss in JS `number`), not a bug — but it means any hand-rolled
event-sequence consumer that expects `sequence` to be a JSON number will break, and it is
a real authoring/consumer surprise if the team is not deliberately aware of protojson's
64-bit-integer convention.

## Envelope/oneof wire shape

Probe: marshaled a populated `Envelope{Payload: &Envelope_TokenMoved{...}}` with
`protojson.MarshalOptions{Indent: "  "}` (throwaway `go run` snippet, not committed —
brief allowed either `/tmp` or an inline snippet; deleted after capturing output below).

```json
{
  "eventId":  "evt-001",
  "sequence":  "42",
  "occurredAt":  "2026-07-23T12:00:00Z",
  "sessionId":  "sess-abc",
  "actorRole":  "gm",
  "tokenMoved":  {
    "tokenId":  "tok-ursus",
    "sceneId":  "scn-goblin-warrens",
    "from":  {
      "x":  3,
      "y":  7
    },
    "to":  {
      "x":  5,
      "y":  8
    }
  }
}
```

Confirms the design-relevant finding: proto's `oneof` serializes as the **variant field
name inlined directly** into the envelope object (`"tokenMoved": {...}`) — there is no
generic `"type"`/`"payload"` discriminator envelope shape the way our spec sketch assumed.
A consumer has to test for the *presence of one of N possible keys*
(`tokenMoved` / `attackRolled` / …) rather than switch on a single `type: string` field.
That's workable (Go's generated `oneof` interface + a type switch, TS's generated
`case` field — see below — both do this for you), but it is a divergence from a
hand-authored discriminated-union JSON contract that any non-generated consumer (a
different language, a hand-rolled parser) has to replicate by hand. Note also:
`protoc-gen-es` additionally emits a `case: "tokenMoved"` and `value: {...}` pair on the
TS message type (a convenience discriminator on top of the raw wire shape) — the *wire*
JSON itself has no such field; that's purely a TS-side ergonomic addition from the
generated code, not something visible cross-language on the wire.

## Extensibility (maps + Struct)

`Actor` — the deliberately open-ended message (`map<string, int32> attributes`,
`map<string, Resource> resources`, `google.protobuf.Struct module_data`) — round-trips
cleanly in both Go and TS with no special-casing needed in the test code:

- `map<string, int32>` / `map<string, Resource>` generate straightforward
  `map[string]int32` / `map[string]*Resource` in Go and `{ [key: string]: number }` /
  `{ [key: string]: Resource }`-shaped records in TS; JSON keys pass through unchanged.
- `google.protobuf.Struct module_data` (holding `{"rageStance": "BerserkerFury",
  "feralRejuvenation": true}` — a string and a bool, i.e. genuinely arbitrary
  module-defined shape) round-trips through `structpb`/`Struct` machinery with no schema
  changes on our side — this is the intended escape hatch for the "arbitrary module blob"
  requirement, and it works as advertised. Cost: `Struct`/`Value` are their own nested
  wrapper types (Go: `*structpb.Struct` wrapping `map[string]*structpb.Value`, each
  `Value` a `oneof` of number/string/bool/struct/list/null) — any code that wants to *use*
  `module_data` programmatically (not just pass it through) has to walk that `Value`
  oneof, unlike a plain `map[string]any` in a JSON-native format. It buys type-safety
  everywhere else in the schema at the cost of losing it exactly where you deliberately
  asked for openness.

## Tool-derivation cost (lines of custom code, expressiveness gaps)

`toolgen/main.go` (49 lines total) walks `MoveTokenRequest`'s `protoreflect` descriptor to
emit an MCP-style tool-definition JSON Schema matching `expected_tool.json` exactly (test
passes, see TDD evidence below). Breaking down the actual custom logic (excluding
package/import/`main()`/comment boilerplate):

- `schemaFor()` (descriptor walk + type-kind switch, recursive for nested messages): **21
  lines**
- `buildTool()` (wraps `schemaFor()` in the MCP tool envelope): **8 lines**
- **~29 lines of custom code** to go from a compiled proto descriptor to an MCP
  `inputSchema`, for a message with 2 fields (one nested).

Expressiveness gap (as flagged by the brief, confirmed): **proto3 has no concept of
optional vs. required scalar fields** — every field is implicitly present in the
descriptor (message-typed fields are technically nullable at the Go/TS level, but the
descriptor gives no `required: false` signal you could reflect on). `toolgen` therefore
marks *every* field required unconditionally
(`required = append(required, name)` runs for every field, no branch). This happens to
match `expected_tool.json` here because both fields of `MoveTokenRequest` genuinely are
required — but it is not a real inference, it's an artifact of proto3 not being able to
express the distinction at all. `proto3_optional`/`optional` keyword support exists for
scalar presence-tracking but wasn't used in this schema, and even where used, the
generated descriptor's "field has presence tracking" is a different, weaker signal than
"caller-facing optional." Any real MCP tool with genuinely optional parameters would need
either (a) hand-authored overrides layered on top of `toolgen`'s output, or (b) proto
field options / a custom extension to carry an explicit required/optional flag — both
represent additional custom code beyond the 29 lines above.

**TDD evidence (`toolgen_test.go`):**

Before implementation — `go test ./contract-spike/proto/toolgen/`:
```
# github.com/PatrikLager/vtt-platform/contract-spike/proto/toolgen [github.com/PatrikLager/vtt-platform/contract-spike/proto/toolgen.test]
contract-spike/proto/toolgen/toolgen_test.go:11:9: undefined: buildTool
FAIL	github.com/PatrikLager/vtt-platform/contract-spike/proto/toolgen [build failed]
FAIL
```

After implementing `main.go` — `go test ./contract-spike/proto/toolgen/ -v`:
```
=== RUN   TestToolgenMatchesExpectedTool
--- PASS: TestToolgenMatchesExpectedTool (0.00s)
PASS
ok  	github.com/PatrikLager/vtt-platform/contract-spike/proto/toolgen	0.429s
```

## Breaking-change tooling (buf output)

**Friction, fixed forward:** the brief's exact invocation —
`buf breaking --against '../../.git#branch=main,subdir=contract-spike/proto'` — fails in
this repo, because nothing under `contract-spike/` has ever been committed to `main`
(Task 1/2's fixtures and this task's proto files live only on `spike/contract-format`,
and per this task's explicit instructions nothing is committed even there). Output:

```
Failure: Module "path: "contract-spike/proto"" had no .proto files
```

Fixed forward: `buf breaking --against` accepts any valid buf input source, not just git
refs — a plain local directory works too. Snapshotted the pre-rename `buf.yaml` +
`contract/v1/events.proto` (with `scene_id`, before the probe rename) into a scratch
directory, applied the same rename the brief specifies
(`sed -i '' 's/string scene_id = 2;/string scene_ref = 2;/'` on the working copy), and ran
`buf breaking --against <scratch-dir-with-pre-rename-schema>`:

```
contract/v1/events.proto:17:3:Field "2" with name "scene_ref" on message "TokenMoved" changed option "json_name" from "sceneId" to "sceneRef".
contract/v1/events.proto:17:10:Field "2" on message "TokenMoved" changed name from "scene_id" to "scene_ref".
```

`buf breaking` exits non-zero (exit code 100) and correctly flags **two** distinct
findings for a single rename: the Go/proto field-name change itself, and the derived
`json_name` change (which is the one that actually matters for wire compatibility with
JSON consumers — a consumer reading `sceneId` off the wire breaks). This is exactly the
regression-catching behavior a spec-format needs; caught deterministically with zero
hand-written detection logic, in contrast to the ~29 lines of custom code toolgen needed
for the tool-derivation probe. The field was reverted immediately after
(`sed` back to `scene_id`, since nothing is tracked in git yet so `git checkout --`
per the brief's literal instruction isn't applicable — confirmed via `git status` showing
the file as untracked both before and after) and `buf lint` + `go test` re-verified clean.

## Drift-gate fit (regenerate + `git diff --exit-code gen/`)

**Caveat on method:** the brief's literal drift-gate check
(`git diff --exit-code gen/`) assumes `gen/` is already committed, so a regenerate that
produces different bytes shows up as a diff. Since this task's `gen/` is deliberately left
**uncommitted** (per this task's explicit "do not commit" instruction), `git diff` against
an untracked path is always empty regardless of whether regeneration actually changed
anything — it would give a false-green signal. Substituted a direct determinism check:
SHA-1 of every generated file, before and after a full `buf generate` rerun.

```
=== before ===
ba59cc4fe47ff12e706473b65bfc71d3dee383fe  gen/ts/contract/v1/events_pb.ts
c48caa5f010f41e83ee39fa11c624f0bb5df2bc6  gen/go/contract/v1/events.pb.go
=== after regenerate ===
ba59cc4fe47ff12e706473b65bfc71d3dee383fe  gen/ts/contract/v1/events_pb.ts
c48caa5f010f41e83ee39fa11c624f0bb5df2bc6  gen/go/contract/v1/events.pb.go
```

Identical bytes both times (verified twice: once via `buf generate` directly, once via
`task generate:proto`). `buf generate`'s remote-plugin output is deterministic given a
fixed schema + fixed plugin versions, so the drift-gate pattern (regenerate in CI, fail if
`git diff --exit-code gen/` is non-empty once `gen/` is actually committed) is sound in
principle here — it just could not be exercised end-to-end in *this* task because of the
no-commit constraint. Note the gate's soundness still depends on the remote plugins
resolving to the *same* pinned version across CI runs; `buf.gen.yaml` here pins plugins by
name only (`buf.build/protocolbuffers/go`, `buf.build/bufbuild/es`), not by version/digest
— a real drift gate would want a pinned plugin revision (buf supports `revision:` on
remote plugin entries) so "no diff" isn't accidentally passing because CI happened to run
before upstream published a new plugin version.

## Versions used

| Tool / package | Version |
|---|---|
| `go` | go1.26.4 darwin/arm64 |
| `buf` (installed via `go install github.com/bufbuild/buf/cmd/buf@latest`) | 1.72.0 |
| `bun` | 1.3.10 |
| `task` (go-task) | 3.45.4 |
| `google.golang.org/protobuf` (Go module, from `go.mod`) | v1.36.11 |
| `protoc-gen-go` (remote plugin, materialized version per generated file header) | v1.36.11 |
| `protoc-gen-es` (remote plugin, materialized version per generated file header) | v2.13.0 |
| `@bufbuild/protobuf` (npm/bun package) | 2.13.0 |

**Note on `buf` PATH:** `buf` is not installed to a directory on `PATH` by default —
`go install` places it at `$(go env GOPATH)/bin/buf`
(`/Users/patriklager/go/bin/buf` in this environment), which is not on `PATH` in this
shell. Every `buf` invocation in this spike (`buf lint`, `buf generate`, `buf breaking`,
`task generate:proto`) was run with
`export PATH="$(go env GOPATH)/bin:$PATH"` prefixed first. `task check` itself does not
invoke `buf` (only `generate:proto` does), so `task check` passes without this PATH
adjustment; anyone running `task generate:proto` will need `$(go env GOPATH)/bin` on
`PATH` or to invoke `buf` by full path.
