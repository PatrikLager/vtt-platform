# OpenAPI contract prototype — evidence

Spike for the shared VTT event contract (Task 5 of 6, `spike/contract-format`) — deliberately
the smallest of the three prototypes. Spec: `openapi.yaml` (OpenAPI 3.1, JSON Schema 2020-12
dialect), modeling the one command (`moveToken`) as an HTTP operation and the four shared
shapes (`GridPosition`, `MoveTokenRequest`, `TokenMoved`, `AttackRolled`, `Actor`) as
`components/schemas`. Generated with `oapi-codegen` (Go) and `openapi-typescript` (TS).
Round-tripped against the shared fixtures in `../fixtures/` in Go only — the brief specifies
no TS test for this prototype (unlike Task 4's `validate.test.ts`). This prototype's headline
job is documenting OpenAPI's event-stream weakness, not maximal coverage — see
**Envelope/stream fit** below.

**TDD-ish sequencing note:** `roundtrip_test.go` was written and run *before* generation, per
this task's workflow (not a brief requirement — this is toolgen, not application logic, so
there is no TDD mandate; the sequence is recorded for transparency, not as a claimed
methodology). Before `gen/go/` existed, `go test ./contract-spike/openapi/...` genuinely
failed:
```
contract-spike/openapi/roundtrip_test.go:9:2: no required module provides package github.com/PatrikLager/vtt-platform/contract-spike/openapi/gen/go; to add it:
	go get github.com/PatrikLager/vtt-platform/contract-spike/openapi/gen/go
FAIL	github.com/PatrikLager/vtt-platform/contract-spike/openapi [setup failed]
```
After generation (see Codegen quality below), all four tests passed with **zero** type-name
adaptation needed — `oapi-codegen`'s type names for our schemas landed exactly on
`TokenMoved`/`AttackRolled`/`Actor`/`MoveTokenRequest`, matching the brief's
`roundtrip_test.go` verbatim. The generation-shape friction that *did* require a fix-forward
is `-generate`'s default pruning, not naming — see below.

## Codegen quality (Go/TS)

- **Friction, fixed forward — default schema pruning.** The brief's literal Step 2 command
  (`-generate types -package oagen -o ... openapi.yaml`) produces a `types.gen.go` with
  **only two** of the four schemas: `GridPosition` and `MoveTokenRequest` (plus the
  request-body alias). `AttackRolled` and `Actor` are silently dropped — they're declared
  under `components/schemas` but never `$ref`'d from the one `moveToken` path, and
  `oapi-codegen`'s default behavior is to prune any schema unreachable from a path/operation
  (confirmed via `oapi-codegen --help`: `skip-prune` is a valid value inside the same
  comma-separated `-generate` list, not a separate flag — its existence as a name implies
  pruning is on by default unless you opt out). This is a real, silent-by-default divergence
  from what the brief's own round-trip test needs (`TestAttackRolled` and `TestActor` both
  reference `oagen.AttackRolled` / `oagen.Actor`, neither of which exist without the fix) —
  running the brief's exact command produces code that does not compile against the brief's
  own test file. **Fixed forward**: `-generate types,skip-prune` (both `generate:openapi`'s
  Taskfile entry and every invocation in this evidence use the corrected flag). Root cause
  confirmed by diffing the two outputs — with the flag, `types.gen.go` grows from 27 to 58
  lines and gains exactly `Actor` and `AttackRolled`, byte-identical otherwise.
- `gen/go/types.gen.go` — 58 lines total for four schemas. One struct per schema, idiomatic
  generated Go, `UpperCamelCase` fields from the spec's `camelCase` JSON properties
  (`TokenId`, `SceneId` — via a `json:"tokenId"` tag per field, not name inference the way
  proto's `snake_case`→`json_name` derivation worked; here the wire key is preserved
  verbatim in the tag since the schema property names already are the wire keys). Inline
  `properties`-without-`$ref` shapes (the `rolls`/`modifiers` array items in `AttackRolled`,
  the `resources` value shape in `Actor`) become **anonymous inline Go structs**
  (`Modifiers []struct{ Source string; Value int } `json:"modifiers"``) rather than named
  types — because the spec author (this brief) inlined those shapes instead of extracting
  them to `components/schemas` the way `GridPosition` was. No `$ref` was used for those, so
  no separate named type exists to generate.
- `gen/ts/types.ts` — 116 lines. **No flat top-level type exports at all** by default — every
  schema is nested inside one big `export interface components { schemas: { TokenMoved: {
  ... }, ... } }`, referenced internally via `components["schemas"]["TokenMoved"]`. This is a
  structurally different generation model from both sibling prototypes: `quicktype`
  (Task 4) and `protoc-gen-es` (Task 3) both emit a type/class *named* `TokenMoved` directly
  importable as `TokenMoved`; `openapi-typescript`'s default output requires every consumer
  to write `components["schemas"]["TokenMoved"]` (or hand-roll their own
  `type TokenMoved = components["schemas"]["TokenMoved"];` aliases) — there is no test file
  in this prototype to force that adaptation into the open, but any future TS consumer would
  hit it immediately. **Confirmed fix exists**: `--root-types` (probed, not applied to the
  committed file — the brief's exact Step 2 command has no such flag) emits exactly
  `export type SchemaTokenMoved = components['schemas']['TokenMoved'];` per schema
  (prefixed `Schema` by default; `--root-types-no-schema-prefix` drops the prefix, untested
  here). This is the TS-side mirror of the Go-side pruning friction above: both generators
  ship a default that a real consumer of this spec would almost certainly want to override,
  and both overrides exist as one documented flag away.
- **No fatal errors, no warnings, from either generator** for the four schemas as authored
  (unlike Task 4's non-fatal `quicktype` stdout warning on the free-form `moduleData` field —
  see the `Actor.moduleData` finding under JSON mapping frictions below, which is a *silent*
  divergence here, not a warned one).

## JSON mapping frictions

The four fixture round-trip tests (`TokenMoved`, `AttackRolled`, `Actor`,
`MoveTokenRequest`) pass byte-for-byte-semantically (`reflect.DeepEqual` on decoded maps) in
Go with **zero** schema changes needed, same as both sibling prototypes — OpenAPI 3.1's
schema property names are, like JSON Schema's, exactly the wire keys (authored
`camelCase` throughout, matching the fixtures verbatim; no proto-style `snake_case`
derivation step exists here either).

- **Real finding — `Actor.moduleData` open-object handling diverges by language, silently.**
  The spec declares `moduleData: { type: object }` (bare, no `properties`, no
  `additionalProperties` — same deliberately-arbitrary-blob intent as Task 3's
  `google.protobuf.Struct` and Task 4's bare-`object` schema). The two generators disagree on
  what "arbitrary object" means:
  - Go (`oapi-codegen`): `ModuleData *map[string]interface{} \`json:"moduleData,omitempty"\`` —
    genuinely open, matches the fixture's `{"rageStance": "BerserkerFury",
    "feralRejuvenation": true}` (string + bool, heterogeneous) with no special handling; the
    `TestActor` round-trip passes cleanly through this field.
  - TypeScript (`openapi-typescript`, default flags): `moduleData?: Record<string, never>` —
    a type that permits **zero** properties. `Record<string, never>` is not "unknown shape,"
    it is "empty object, statically enforced" — the *opposite* of the Go type's openness, from
    the identical spec input, with **no warning printed either way**. Confirmed root cause and
    fix via `--empty-objects-unknown` (probed against a scratch copy): flips the output to
    `moduleData?: Record<string, unknown>`, which is the openness-preserving equivalent of the
    Go type. This is a sharper version of Task 4's `additionalProperties` finding (there,
    TS stayed open and Go stayed closed with no fix flag available; here, TS's *default* is
    closed while Go's default is open, and TS's fix is one flag away) — the direction of the
    asymmetry flipped, but the underlying lesson repeats across two of the three formats: a
    "deliberately open" field needs per-generator, per-language verification, because
    "arbitrary JSON object" is not a value every codegen tool agrees means "no constraints."
  - No TS test exists in this prototype to have caught this at generation time (the brief
    specifies no TS round-trip/validate test here, unlike Task 4's `ajv` pass) — this finding
    surfaced purely from reading the generated output, which is itself worth noting: with no
    consuming test, a silently-wrong TS type for an open field would ship undetected.
- **Integers:** every `"type": "integer"` field mapped to Go `int` and TS `number`, no
  precision-loss surprises in the fixtures used (small ints only) — same non-finding as
  Task 4's JSON Schema prototype, and for the same reason: JSON Schema's `integer` keyword
  (which OpenAPI 3.1 inherits verbatim, being JSON-Schema-dialect-compatible) has no
  wire-format string-encoding convention, unlike protojson's `int64`-as-string rule from
  Task 3.

## Envelope/stream fit

**This is the headline finding for this prototype**, per the brief. OpenAPI models one thing:
HTTP request/response. There is no primitive in OpenAPI 3.1 for "a client holds open a
`Envelope`-multiplexed WebSocket connection and receives an ordered stream of typed events" —
neither the base spec nor either code generator has any concept of a persistent connection,
message sequencing, or push-without-a-preceding-request.

- **The closest thing OpenAPI 3.1 has is `webhooks` — and it models the wrong direction and
  the wrong transport.** `webhooks` (added in 3.1, promoted from Swagger's separate
  "callbacks" idea) describes HTTP requests **the server sends to a URL the client
  registered out-of-band** — i.e., the server acting as an HTTP *client* making a POST to
  *your* endpoint, which responds. That is a fundamentally different shape from "the client
  holds one long-lived WebSocket connection and the server pushes frames down it": there is
  no callback URL in our system, no separate HTTP request per event, and no response the
  event-receiver is expected to return. Confirmed empirically by authoring a scratch
  `webhooks` section (not committed — probe only) modeling the two event types:
  ```yaml
  webhooks:
    tokenMovedEvent:
      post:
        operationId: onTokenMoved
        requestBody:
          required: true
          content:
            application/json:
              schema: { $ref: '#/components/schemas/TokenMoved' }
        responses:
          '200': { description: Event received }
    attackRolledEvent:
      post:
        operationId: onAttackRolled
        requestBody: { ... AttackRolled ... }
        responses: { '200': { description: Event received } }
  ```
  Running both generators against this probe confirms `webhooks` gets **identical structural
  treatment to `paths`** in both tools — `oapi-codegen` emits the exact same
  `OnTokenMovedJSONRequestBody`/`OnAttackRolledJSONRequestBody` alias shape it emits for real
  path operations' request bodies; `openapi-typescript` emits a `webhooks` interface with the
  same per-verb (`get?/put?/post/...`) PathItem shape as the `paths` interface, referencing
  the same `operations["onTokenMoved"]` machinery paths use. Neither tool treats `webhooks` as
  anything other than "more HTTP operations, direction relabeled." There is no session
  concept, no way to say "these N event shapes are multiplexed over one connection," no
  sequence/ordering field, and no discriminated-union primitive to tie a `type` tag to a
  `payload` shape the way proto's `oneof` (Task 3) or even a hand-rolled JSON Schema `oneOf`
  (Task 4) can — an `Envelope{eventId, sequence, occurredAt, sessionId, actorRole, payload}`
  wrapper simply has nowhere to live as a *modeled* concept; at best you list every event's
  request-body schema separately under `webhooks` (as the probe does) and lose the
  envelope's shared fields entirely, or hand-write the envelope as one more
  `components/schemas` object with a manually-maintained `oneOf` — which is exactly the
  workaround Task 4 already had to reach for, except OpenAPI adds no path/operation framing
  value on top of it the way it does for the one real HTTP command this spec has.
- **`oasdiff breaking` cannot see event-schema changes at all, which is the direct, testable
  consequence of the above.** Because `AttackRolled` and `Actor` are not reachable from any
  path or webhook operation in the committed spec, they are invisible to the primary
  breaking-change tool for this format (see Breaking-change tooling below for the full
  probe) — a rename inside `AttackRolled` produces **zero** findings from `oasdiff breaking`,
  while the identical class of rename inside `TokenMoved` (which *is* reachable, as the
  `moveToken` response schema) is correctly flagged. Since the event stream is exactly the
  set of schemas OpenAPI has no path to hang off of, the format's premier breaking-change
  tool structurally cannot protect the part of the contract this project cares about most —
  not a hypothetical concern, but the literal, reproduced behavior of the tool against this
  spec.
- **The headline recommendation, per the brief: this would require maintaining TWO spec
  documents.** OpenAPI 3.1 has an event-native sibling, **AsyncAPI**, purpose-built for
  message-driven/WebSocket/pub-sub APIs — it has first-class `channels`, `messages`, and
  (in AsyncAPI 3.x) explicit `oneOf`-message-per-channel support that models exactly the
  `Envelope` shape this project needs. But AsyncAPI is a **separate specification format**
  with its own document, its own `components/schemas` section (which could `$ref` the same
  underlying JSON Schema definitions as the OpenAPI doc, in principle, but not without a
  second toolchain: a second codegen invocation, a second breaking-change checker, a second
  place authors must remember to update in lockstep with the first). A team choosing OpenAPI
  for `moveToken`-style HTTP commands and needing to also describe the `Envelope` stream is
  choosing to maintain **two spec documents, two generators, two drift gates** — versus
  proto3's single `.proto` file covering both the RPC-shaped command and the `oneof`-typed
  stream event in one schema (Task 3), or JSON Schema's single `schemas/*.schema.json` set
  covering both via a hand-rolled `oneOf` envelope (Task 4). This is the structural,
  irreducible cost this prototype's win condition — being small on purpose — exists to
  surface.

## Extensibility

`Actor` — the deliberately open-ended schema (`attributes: additionalProperties: {type:
integer}`, `resources: additionalProperties: {object}`, `moduleData: {type: object}`) —
round-trips cleanly in Go with no special-casing (`TestActor` passes with zero schema
changes), confirming `additionalProperties` and bare-`object` both work as JSON-Schema-dialect
primitives inside OpenAPI 3.1 exactly as they do in standalone JSON Schema (Task 4) — OpenAPI
3.1's schema objects **are** JSON Schema, so this is expected, not a new finding in itself.

- `additionalProperties: {type: integer}` / `additionalProperties: {object}` generate
  `map[string]int` / `map[string]struct{ Current int; Max int }` in Go — same
  native-map treatment as Task 4's JSON Schema prototype, and structurally lighter than
  Task 3's `map<string, Resource>` + separate `google.protobuf.Struct` combination (no
  wrapper `Value`-oneof type to walk).
- **The openness/closedness asymmetry that Task 4 found for `additionalProperties`** (open in
  TS, structurally closed in Go) does **not** reproduce for `attributes`/`resources` here —
  both are explicit `additionalProperties` schemas (not the bare-`object` shape), and both
  generators treat them as genuine open maps in both languages. The asymmetry this prototype
  finds instead is on `moduleData` specifically — the bare-`{type: object}` field, no
  `additionalProperties` keyword at all — see JSON mapping frictions above. The lesson is
  narrower than Task 4's: it's not "OpenAPI/JSON-Schema `additionalProperties` is
  cross-language-fragile," it's specifically "a schema author relying on *bare* `object`
  (omitting `additionalProperties` entirely) to mean 'arbitrary' gets silently different
  answers per generator/language; a schema author who writes `additionalProperties: true`
  (or the corresponding typed map) explicitly does not."

## Tool-derivation cost

Per the brief, **no code was written for this section** — `components/schemas` in a 3.1
document already *is* JSON Schema (3.1 adopted the 2020-12 dialect directly, unlike 3.0's
JSON-Schema-inspired-but-incompatible subset), so the Task 4 finding ("when the target format
an MCP/LLM-tool caller wants is the same format the contract is authored in, tool derivation
degrades from generation to embedding") applies here too, in principle — with one added step
Task 4 did not have to take.

- **The extraction step, confirmed empirically.** Pulling `MoveTokenRequest` directly out of
  `openapi.yaml`'s `components.schemas` (via a one-line `python3 -c "yaml.safe_load(...)"`
  probe, no committed code) yields:
  ```json
  {
    "type": "object",
    "properties": {
      "tokenId": { "type": "string" },
      "to": { "$ref": "#/components/schemas/GridPosition" }
    },
    "required": ["tokenId", "to"]
  }
  ```
  Unlike Task 4's `move_token_request.schema.json` (deliberately authored ref-free, per that
  task's brief, specifically so the embedding step could stay a straight read with no
  resolver), this OpenAPI spec's `MoveTokenRequest` **does** use an internal `$ref` for the
  nested `GridPosition` shape — because `GridPosition` is shared with `TokenMoved` and
  `AttackRolled`'s sibling shapes and it would be poor spec-authoring practice to duplicate
  it inline three times inside one `openapi.yaml`. The extracted node above is **not**
  directly usable as an MCP `inputSchema` on its own: `#/components/schemas/GridPosition` is
  meaningless without the rest of the document alongside it, whereas `expected_tool.json`'s
  own `inputSchema` inlines the `x`/`y` shape directly with no `$ref` at all. A real
  extraction step for this format therefore needs a `$ref` resolver (walk the document,
  substitute `$ref` targets in place) before the result is a portable, standalone JSON
  Schema object an MCP caller can use — the exact ~10-line resolver Task 4 flagged as
  "would have been needed had `attack_rolled.schema.json`'s refs been the toolgen target
  instead" is *not* optional here; it is the median case for a spec document, not the
  exception, because a single OpenAPI document consolidating multiple operations'/schemas'
  shared shapes is exactly the scenario that produces internal `$ref`s. This is the concrete
  form of the "extraction step" the brief calls out: OpenAPI's own economy-of-repetition
  (one document, `$ref`-shared subshapes) is in direct tension with the "just embed the
  schema" simplicity Task 4 got to claim for a set of already-flat, single-purpose schema
  files.
- **No toolgen code or test was added under `contract-spike/openapi/`** — the brief's Step 4
  explicitly scopes this section to documentation from existing artifacts, not a new
  `toolgen/` package (unlike Tasks 3 and 4, which both built and TDD'd a `toolgen/main.go`).
  This omission is itself evidence for the "deliberately smaller" framing of this prototype.

## Breaking-change tooling

`oasdiff` (per the brief) is the closest OpenAPI-native analogue to `buf breaking` / the
`json-schema-diff` probe from the sibling prototypes. Ran the brief's exact command:

```
$ go run github.com/oasdiff/oasdiff@latest breaking --help
Display breaking changes between base and revision specs.
Base and revision can be a path to a file, a URL, a git ref (e.g. main:openapi.yaml), or '-' to read standard input.
...
Usage:
  oasdiff breaking base revision [flags]
Flags:
  ...
  -o, --fail-on string           exit with return code 1 when output includes errors with this level or higher: ERR or WARN
  ...
```
Usable — installs and runs cleanly via `go run ...@latest` with no local `protoc`/plugin
registry equivalent needed (same zero-local-install pattern as `buf`/`quicktype` in the
sibling prototypes' probes).

- **Confirmed it correctly flags a rename on a schema reachable from a path.** Renamed
  `TokenMoved.sceneId` → `sceneRef` in a scratch copy of `openapi.yaml` (`TokenMoved` is the
  `moveToken` response schema, i.e. path-reachable) and diffed against the original:
  ```
  $ oasdiff breaking openapi-base.yaml openapi-after.yaml
  1 changes: 1 error, 0 warning, 0 info
  error	[response-required-property-removed] at openapi-after.yaml
  	in API POST /sessions/{sessionId}/tokens/{tokenId}/move
  		removed the required property `sceneId` from the response with the `200` status
  ```
  Correctly caught, with a precise field-level message (closer to `buf breaking`'s
  field-level output than `json-schema-diff`'s whole-schema value-space diff from Task 4).
- **Real finding — `oasdiff breaking` exits 0 on this breaking change by default.** The run
  above reported an `error`-level finding but the shell's exit code was **0**. Confirmed via
  `--help`: `--fail-on` (`-o`) is what controls the exit code, and it defaults to empty — i.e.
  reporting is unconditional but **failing the process is opt-in**, unlike `buf breaking`
  (exits 100 unconditionally on any breaking finding, Task 3) or `json-schema-diff`
  (exits 1 unconditionally, Task 4). Re-ran with `--fail-on ERR`:
  ```
  $ oasdiff breaking --fail-on ERR openapi-base.yaml openapi-after.yaml
  1 changes: 1 error, 0 warning, 0 info
  error	[response-required-property-removed] ...
  exit status 1
  ```
  Non-zero, as expected, once the flag is supplied. This is a real CI-wiring trap: a team
  copying the brief's bare `oasdiff breaking base revision` invocation into a CI step (no
  `--fail-on`) would see the breaking-change text printed in logs but the pipeline would
  stay green — the tool's default behavior is report-only, not gate-by-default, and nothing
  in the base command signals that.
- **Confirmed — unreachable schemas are invisible to `oasdiff breaking`, tying directly to
  the Envelope/stream fit finding above.** Repeated the same rename probe against
  `AttackRolled.attackerId` (a schema declared under `components/schemas` but not `$ref`'d
  from any path in the committed spec):
  ```
  $ oasdiff breaking --fail-on ERR openapi-base.yaml openapi-after2.yaml
  No breaking changes to report, but the specs are different.
  Run 'oasdiff diff' to see structural differences.
  ```
  Exit 0, zero findings, despite an identical class of required-field rename to the one
  caught above. `oasdiff breaking` walks paths/operations, not the full `components/schemas`
  set — a schema with no inbound path/operation reference is outside its scan surface
  entirely. Since the event-stream schemas (`AttackRolled`, and `Actor` if used as an event
  payload) are exactly the schemas this spec has no path for (per the Envelope/stream fit
  finding), this is not a hypothetical edge case: it is the specific set of schemas this
  project would most need breaking-change protection for, and the tool's default scan cannot
  reach them without first inventing a path/webhook wrapper for every event type purely to
  give `oasdiff` something to walk.

## Drift-gate fit

**Same caveat on method as both sibling prototypes:** the literal drift-gate check (`git diff
--exit-code gen/`) assumes `gen/` is already committed; since this task's `gen/` is
deliberately left uncommitted (per this task's "do not commit" instruction), `git diff`
against an untracked path is always empty regardless of whether regeneration changed
anything — false-green. Substituted the same direct check as Tasks 3 and 4: SHA-1 of every
generated file, before and after a full `task generate:openapi` rerun.

```
=== before ===
1597ca7d94486f93c1da96b2fa127af18a281268  contract-spike/openapi/gen/go/types.gen.go
ddf1c6c663d130f62a3516406a14535fedcbe662  contract-spike/openapi/gen/ts/types.ts
=== after `task generate:openapi` ===
1597ca7d94486f93c1da96b2fa127af18a281268  contract-spike/openapi/gen/go/types.gen.go
ddf1c6c663d130f62a3516406a14535fedcbe662  contract-spike/openapi/gen/ts/types.ts
```

Identical bytes. Both generators are deterministic given a fixed spec + fixed generator
version, so the drift-gate pattern (regenerate in CI, fail if `git diff --exit-code gen/` is
non-empty once `gen/` is actually committed) is sound in principle here too.

- **Reproducibility caveat, same shape as Task 3's `buf` finding, worse in degree.** Both
  Step 2 commands resolve their generator via `@latest`
  (`go run .../oapi-codegen/v2/cmd/oapi-codegen@latest`) or an un-pinned `bunx` invocation of
  a `package.json`-declared version (`openapi-typescript@^7.13.0` — the `^` allows minor/patch
  drift on a bare `bun install` elsewhere, though the committed `bun.lock` pins the exact
  resolved version for this checkout). The **Go side is the more exposed one**: `go run
  pkg@latest` re-resolves to whatever the latest published `oapi-codegen` release is on
  *every single invocation*, with no lockfile-style pin at all (confirmed: `go mod tidy`
  after generation left `go.mod`/`go.sum` completely untouched — `go run pkg@latest` does not
  add the tool as a module dependency the way `go install`/an explicit `require` would, so
  there is no artifact in this repo recording which `oapi-codegen` version produced the
  committed `types.gen.go`, beyond the version string oapi-codegen prints into its own
  generated file header comment). This is a strictly weaker reproducibility guarantee than
  Task 3's `buf.gen.yaml` (which at least pins plugins by *name*, even if not by
  digest/revision) — here there is no config file for the Go generator step at all, only the
  literal `go run ...@latest` command baked into `Taskfile.yml`. A drift gate built on top of
  this command is sound *today*, but "no diff" on a future CI run could mean either "nothing
  changed" or "a new `oapi-codegen@latest` release happened to produce byte-identical output
  for this particular spec" — indistinguishable from the gate's perspective, and more likely
  to silently start producing *different* output on some future run than either sibling
  prototype's generator invocation.

## Versions used

| Tool / package | Version |
|---|---|
| `go` | go1.26.4 darwin/arm64 |
| `bun` | 1.3.10 |
| `task` (go-task) | 3.45.4 |
| `oapi-codegen` (via `go run .../v2/cmd/oapi-codegen@latest`, version printed in generated file header) | v2.8.0 |
| `openapi-typescript` (npm/bun devDependency, from `bun add -d openapi-typescript`) | 7.13.0 |
| `oasdiff` (probed via `go run github.com/oasdiff/oasdiff@latest`, not a project dependency) | v1.25.1 |
| OpenAPI spec dialect used | 3.1.0 (JSON Schema 2020-12) |

**Note on `go run ...@latest` and `go.mod`:** per this task's constraints, neither
`oapi-codegen` nor `oasdiff` was added as a module dependency — `go run pkg@latest` builds
and runs the tool from its own module graph without touching the current module's
`go.mod`/`go.sum`. Confirmed via `git status go.mod go.sum` immediately after `go mod tidy`
following generation: no changes. This matches the brief's expectation that `go run` would
"require nothing" of `go.mod` — it did not.
