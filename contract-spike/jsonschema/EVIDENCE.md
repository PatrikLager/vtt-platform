# JSON Schema contract prototype — evidence

Spike for the shared VTT event contract (Task 4 of 6, `spike/contract-format`). Schemas:
`schemas/*.schema.json` (draft 2020-12). Generated with `quicktype` (no `$ref` resolver
of our own needed — quicktype resolves `$defs`/`$ref` internally). Round-tripped against
the shared fixtures in `../fixtures/` in both Go and TypeScript, plus a native-to-the-format
validation probe with `ajv`.

## Codegen quality (Go/TS)

Both outputs generated cleanly from `bunx quicktype --src-lang schema ...` against all four
`schemas/*.schema.json` files in one invocation per target language — no local schema
server, no `$ref` resolver, no plugin registry (unlike proto's `buf generate` against
remote plugins).

- `gen/go/types.go` — 115 lines. One struct per schema (`TokenMoved`, `AttackRolled`,
  `Actor`, `MoveTokenRequest`) plus a small `Marshal`/`Unmarshal` helper pair per type.
  Field names become Go `UpperCamelCase` from the fixtures' `camelCase` JSON keys
  (`TokenID`, `SceneID`) — quicktype capitalizes trailing/embedded acronym-looking suffixes
  (`Id` → `ID`), a minor but real naming transform to be aware of (`tokenId` →
  `TokenID`, not `TokenId`).
- `gen/ts/types.ts` — 58 lines. Plain `export interface` declarations, no runtime code at
  all (`--just-types`) — no parse/validate/serialize helpers ship with the TS output,
  unlike the Go output's generated `Marshal`/`Unmarshal` functions. Anyone wanting
  runtime validation on the TS side has to bring their own (that's exactly what Step 4's
  `ajv` probe is for — see below).
- **Friction — `$defs` naming drift:** none of our four schemas' `$defs` entries
  (`GridPosition`, `DieRoll`, `Modifier`, `Resource`) carry a `title`. quicktype does not
  use the `$defs` **key** as the emitted type name unless the definition has an explicit
  `title`; instead it derives a name from usage context. Observed renames:
  `GridPosition` → `From` (in `TokenMoved`, named after the first property that uses it)
  and → `To` (in `MoveTokenRequest`'s inline duplicate shape, named after its property);
  `DieRoll` → `RollElement` (property `rolls` + array-item convention `<Prop>Element`);
  `Modifier` → `ModifierElement`; `Resource` → `ActorSchema` (named after the *file*
  `actor.schema.json`, not the property `resources`, since it's used inside an
  `additionalProperties` map rather than a plain property). **Confirmed root cause and
  fix**: added `"title": "DieRoll"` / `"title": "Modifier"` to a scratch copy of
  `attack_rolled.schema.json`'s `$defs` and regenerated — quicktype honored the titles
  exactly (`Modifier`, `DieRoll` came out named correctly). This is a real authoring
  discipline the brief's own `token_moved.schema.json` example does not follow (its
  `GridPosition` `$defs` entry has no `title` either), so the drift reproduces from the
  brief's own given schema, not just the two we authored.
- **No adaptation needed for the brief's test code**, despite the drift above: the
  brief's `roundtrip_test.go` and `validate.test.ts` only reference the four **top-level**
  type names (`TokenMoved`, `AttackRolled`, `Actor`, `MoveTokenRequest`), which are all
  driven by each schema's `title` (which we did set) and matched exactly — zero renames
  needed in either test file. The drift is real but invisible to consumers who only touch
  top-level types; it becomes visible the moment code needs to name a nested shape
  directly (e.g. a hand-written function taking a `DieRoll` parameter would have to import
  `RollElement` instead — a surprising, unstable name tied to *where* the shape is first
  used, not what it *is*).
- **Non-fatal warning, both runs:** `quicktype --lang go` prints (to stdout, not stderr,
  interestingly — visible even though the file still writes out and exit code is 0):
  ```
  Issue in line 63: quicktype cannot infer this type because there is no data about it in the input.
  63: 	ModuleData map[string]interface{} `json:"moduleData"`
  ```
  for `Actor.moduleData` (schema: bare `{"type": "object"}`, no `properties` — the
  deliberately free-form module blob). The fallback (`map[string]interface{}` in Go,
  `{ [key: string]: unknown }` in TS) is exactly the right shape for "arbitrary object,"
  so this is an FYI-level message, not a real problem — but it is worth flagging because
  the tool's own wording ("cannot infer") reads like an error at first glance.

## JSON mapping frictions

All four fixture round-trip tests (`TokenMoved`, `AttackRolled`, `Actor`,
`MoveTokenRequest`) pass byte-for-byte-semantically (`reflect.DeepEqual` on decoded maps)
in Go with **zero** schema changes needed, and all four fixtures independently validate
against their schemas via `ajv` in TypeScript — because our schema property names were
authored as `camelCase` directly (matching the fixtures verbatim), there is no
`snake_case`-derivation step the way protojson has for proto fields; JSON Schema has no
opinion on field-name casing at all, so "the schema property name is exactly the wire
key" — simpler than proto's derivation, but also means a JSON Schema author who *doesn't*
match the wire format by hand gets no help and no error until validation time.

- **Openness signal survives to TypeScript, silently vanishes in Go.**
  `move_token_request.schema.json` is (per the brief, deliberately) written without
  `"additionalProperties": false`. quicktype's TS output reflects that: both
  `MoveTokenRequest` and its nested `To` interface get an index signature
  (`[property: string]: unknown;`) added, meaning the TS compiler treats extra properties
  on that type as legal. The Go output has **no equivalent** — Go structs are always
  "closed" at the type level (`encoding/json.Unmarshal` silently drops unknown fields
  into nothing, `Marshal` never emits fields that aren't struct fields) — so the exact
  same schema produces a type that is structurally *open* in TypeScript and structurally
  *closed* (silently, not enforced, just incapable of representing extra fields either
  way) in Go. A consumer reading the Go type alone would not know the schema permits
  additional properties; a consumer reading the TS type would. This is a cross-language
  fidelity gap that has no analogue in the proto prototype (proto3 messages are
  unconditionally "extra fields ignored" in both languages, so there was nothing to lose
  in translation there).
- **Integers:** every `"type": "integer"` field in our schemas (all of them) mapped to Go
  `int64` and TS `number` with no precision-loss surprises in the fixtures used (small
  ints only) — unlike proto3's `int64`, JSON Schema's `integer` keyword has no wire-format
  string-encoding convention, so there was no equivalent to protojson's
  int64-as-JSON-string finding from the Task 3 evidence. (Not tested with a value near
  `Number.MAX_SAFE_INTEGER`; JSON Schema alone gives no signal either way for that case —
  it would be a project-level convention to establish, not something the format enforces.)

## Envelope/stream fit

JSON Schema has no first-class tagged-union primitive equivalent to proto3's `oneof`.
The idiomatic approximation is `oneOf` with a discriminator field (`"type": {"const":
"..."}` per branch) — probed with a throwaway `envelope.schema.json` (not committed;
scratch file, deleted after capturing output below, matching the brief's allowance for
throwaway probes) modeling the same `Envelope{eventId, sequence, occurredAt, sessionId,
actorRole, oneOf[{type:"tokenMoved", payload:TokenMoved-shape}, {type:"attackRolled",
payload:AttackRolled-shape}]}` shape as Task 3's proto `Envelope`.

- **`ajv` validates the union correctly** — a sample envelope with `type: "tokenMoved"`
  and a matching `payload` validates; flipping `type` to `"attackRolled"` while leaving
  the `tokenMoved`-shaped `payload` correctly fails (`oneOf` reports "must match exactly
  one schema in oneOf" plus the two branch-level mismatches). Validation-time correctness
  of the discriminated union is not in question.
- **Real finding — quicktype does not generate a discriminated union from `oneOf`.**
  Both the TS and Go outputs collapse the two `oneOf` branches into **one merged type**
  with every field from every branch present and marked optional/pointer:
  ```ts
  export interface Payload {
      sceneId?:    string;
      tokenId?:    string;
      attackerId?: string;
      targetId?:   string;
      [property: string]: unknown;
  }
  export type Type = "tokenMoved" | "attackRolled";
  ```
  ```go
  type Payload struct {
      SceneID    *string `json:"sceneId,omitempty"`
      TokenID    *string `json:"tokenId,omitempty"`
      AttackerID *string `json:"attackerId,omitempty"`
      TargetID   *string `json:"targetId,omitempty"`
  }
  type Type string
  const ( AttackRolled Type = "attackRolled"; TokenMoved Type = "tokenMoved" )
  ```
  The discriminator itself becomes a clean enum (`Type`/`"tokenMoved" | "attackRolled"`),
  but nothing ties `Type == "tokenMoved"` to "only `tokenId`/`sceneId` are populated" at
  the type level in either language — every payload field is optional/nilable everywhere,
  and reading `payload.attackerId` when `type === "tokenMoved"` type-checks in TypeScript
  and compiles in Go despite being semantically wrong. This is a genuine regression from
  the proto prototype's finding: proto3's `oneof` generated a real Go interface
  (`isEnvelope_Payload`) with a type switch and a TS `case`/`value` discriminated pair —
  both giving compile-time-ish guidance. quicktype's `oneOf` handling gives up that
  guarantee entirely; a JSON Schema + quicktype envelope would need hand-written
  discrimination logic (a switch on `type` with manual casts/assertions) with **no**
  compiler backing that the cast is safe, in either language.
- **Not part of committed schemas.** No `envelope.schema.json` is added under
  `schemas/` — the brief's Task 4 deliverable list only calls for the four fixture-backed
  schemas, and this probe exists purely to fill this evidence section the same way Task 3
  probed `Envelope` with a throwaway `go run` snippet. Scratch files were written to the
  session scratchpad, not the repo.

## Extensibility

`actor.schema.json` — the deliberately open-ended schema
(`"attributes": {"additionalProperties": {"type": "integer"}}`,
`"resources": {"additionalProperties": {"$ref": "#/$defs/Resource"}}`,
`"moduleData": {"type": "object"}`) — round-trips cleanly in both Go and TS with no
special-casing in the test code, and validates cleanly under `ajv`:

- `additionalProperties: {"type": "integer"}` / `additionalProperties: {"$ref":
  "#/$defs/Resource"}` generate `map[string]int64` / `map[string]ActorSchema` in Go and
  `{ [key: string]: number }` / `{ [key: string]: ActorSchema }`-shaped index signatures
  in TS — JSON keys pass through unchanged, no wrapper types, no special decode step.
  This is noticeably lighter-weight than proto's `map<string, Resource>` +
  `google.protobuf.Struct` combination: JSON Schema's `additionalProperties` maps to a
  native language map/record type directly, whereas proto's escape hatch
  (`google.protobuf.Struct`) required its own nested `Value`-oneof wrapper type to walk
  programmatically (see Task 3 evidence). Here, `moduleData` (holding
  `{"rageStance": "BerserkerFury", "feralRejuvenation": true}` — genuinely
  heterogeneous: a string and a bool) becomes plain `map[string]interface{}` /
  `{ [key: string]: unknown }` — usable with a single type assertion/cast per field, no
  intermediate wrapper type to unwrap first. **This is the format's main structural
  advantage over proto3 for the "arbitrary module blob" requirement**: JSON Schema's
  `"type": "object"` with no `properties` restriction *is* "arbitrary JSON," natively,
  because the wire format and the schema format are the same format — there is no
  analogue to `Struct`/`Value` needed because there is nothing to bridge.
  Cost/trade-off: total loss of type safety on that field in both languages (`interface{}`
  / `unknown` — same as proto's `Struct` cost, just without the extra wrapper-walking
  step), and (as with proto's `Struct`) `ajv` cannot validate *inside* `moduleData` at
  all — an author who wants to constrain it (e.g. per-`moduleId` shape) would need a
  separate `if`/`then` conditional schema keyed on `moduleId`, not attempted here.

## Tool-derivation cost (lines of custom code, expressiveness gaps)

`toolgen/main.go` (35 lines total, including comment/package/import/`main()`
boilerplate) reads `move_token_request.schema.json` off disk and drops three metadata
keys (`$schema`, `$id`, `title`) before wrapping it as the `inputSchema` of an MCP tool
definition — this passes `TestToolgenMatchesExpectedTool` byte-for-byte against
`../../fixtures/expected_tool.json` (TDD evidence below) with **no descriptor walk, no
type-kind switch, no recursion**, because the schema *already is* JSON Schema — the same
format an MCP `inputSchema` expects.

- Actual custom logic (excluding boilerplate): **file read + `json.Unmarshal` + a
  3-key `delete()` loop + a literal map** — roughly **10 lines**, versus proto's
  **~29 lines** of `protoreflect`-descriptor-walking `schemaFor()` needed to derive the
  equivalent JSON Schema *from* a compiled proto descriptor (Task 3 evidence). This is
  the headline structural finding for this format: **when the target format an
  MCP/LLM-tool caller wants (JSON Schema) is the same format the contract is authored in,
  "tool derivation" degrades from generation to embedding.** No descriptor reflection API
  is needed at all.
- **Why `move_token_request.schema.json` needed no ref-inliner.** The brief specifies it
  as deliberately ref-free (the `to` field's `GridPosition` shape is written inline, not
  via `$ref: "#/$defs/GridPosition"`) precisely so this step could stay a straight
  read-and-strip. Confirmed by inspection: the schema as authored has no `$defs`/`$ref`
  at all, so no ~10-line ref-inliner (as the brief flags as a possible necessity) was
  needed. Had `attack_rolled.schema.json` (which *does* use `$defs`/`$ref` for `DieRoll`/
  `Modifier`) been the toolgen target instead, an inliner (or a real `$ref` resolver)
  would have been mandatory — the ref-free authoring choice is a real, deliberate
  cost/simplicity trade-off, not a coincidence: **JSON Schema's "embedding, not
  generation" advantage only holds cleanly for ref-free schemas**; the moment `$defs` are
  used, a tool-definition consumer needs either a resolver or an inlining step, at which
  point the LOC advantage over proto's descriptor walk narrows (still likely smaller, but
  no longer near-zero).
- **Expressiveness parity (unlike proto3):** JSON Schema *does* have first-class
  optional-vs-required (the schema's own `required` array), so — unlike proto's
  `toolgen`, which had to mark every field required unconditionally because proto3 has no
  such signal — a JSON-Schema-sourced `inputSchema` carries a real, author-intended
  required/optional distinction straight through with **zero extra code**: the
  `required` array is just copied verbatim, because it's already sitting there in the
  schema we're deriving the tool from (in this case `["tokenId", "to"]`, matching
  `expected_tool.json` exactly). This is the one place JSON Schema is strictly *more*
  capable than proto3 for this specific probe, at no extra implementation cost.

**TDD evidence (`toolgen_test.go`):**

Before implementation — `go test ./contract-spike/jsonschema/toolgen/ -v`:
```
# github.com/PatrikLager/vtt-platform/contract-spike/jsonschema/toolgen [github.com/PatrikLager/vtt-platform/contract-spike/jsonschema/toolgen.test]
contract-spike/jsonschema/toolgen/toolgen_test.go:11:9: undefined: buildTool
FAIL	github.com/PatrikLager/vtt-platform/contract-spike/jsonschema/toolgen [build failed]
FAIL
```

After implementing `main.go` — `go test ./contract-spike/jsonschema/toolgen/ -v`:
```
=== RUN   TestToolgenMatchesExpectedTool
--- PASS: TestToolgenMatchesExpectedTool (0.00s)
PASS
ok  	github.com/PatrikLager/vtt-platform/contract-spike/jsonschema/toolgen	0.348s
```

## Breaking-change tooling

There is no `buf`-equivalent purpose-built for JSON Schema with the same install
footprint. Probed the brief's suggested `json-schema-diff`:

```
$ bunx json-schema-diff --help
Usage: json-schema-diff [options] <sourceSchemaFile> <destinationSchemaFile>

Finds differences between two json schema files
...
The files must be valid according to Json Schema draft-07.
...
```

The package resolves via `bunx` with no install step needed in `package.json` (matches
the brief's usage — a probe, not a project dependency; nothing added to
`package.json`/`bun.lock` from this step, confirmed via `git status` before/after).

- **Friction, fixed forward:** running it against our actual `token_moved.schema.json`
  (declared `"$schema": "https://json-schema.org/draft/2020-12/schema"`) fails outright:
  ```
  Error: no schema with key or ref "https://json-schema.org/draft/2020-12/schema"
      at Ajv.validate (.../node_modules/ajv/dist/core.js:148:23)
  ```
  `json-schema-diff` (v1.0.0, last published years ago per its own `--help` text)
  bundles an Ajv instance that only knows the draft-07 meta-schema — it does not
  recognize 2020-12 at all, so it cannot be pointed at our schemas as authored without
  first downgrading their `$schema` declaration. This is a real, current-state gap: the
  brief mandates draft 2020-12 for authoring (correctly — it's the current spec), but the
  one plausible off-the-shelf breaking-change tool for JSON Schema does not support that
  draft.
- **Fixed forward to confirm the tool otherwise works:** copied `token_moved.schema.json`
  to a scratch `before`/`after` pair, changed `$schema` to
  `http://json-schema.org/draft-07/schema#` on both, renamed `sceneId` → `sceneRef` in
  `after` (mirroring Task 3's exact rename probe), and re-ran:
  ```
  $ bunx json-schema-diff before.schema.json after.schema.json
  Breaking changes found between the two schemas.

  Values described by the following schema were added:
  { "type": "object", ... "properties": { ..., "sceneRef": { "type": "string" } }, "required": [ "tokenId", "sceneRef", "from", "to" ] }

  Values described by the following schema were removed:
  { "type": "object", ... "properties": { ..., "sceneId": { "type": "string" } }, "required": [ "tokenId", "sceneId", "from", "to" ] }
  ```
  Exit code 1 (correctly non-zero). The tool *does* work, and does correctly flag the
  rename as breaking — but note the shape of its output versus `buf breaking`'s: it is a
  **set-theoretic whole-schema diff** ("here is the JSON-value-space that was added, here
  is what was removed"), not a **field-level diff** ("field `sceneId` renamed to
  `sceneRef`"). For a two-field rename the added/removed value-space schemas are large and
  a human has to infer "this looks like a rename" themselves — there is no equivalent to
  `buf breaking`'s precise `Field "2" ... changed name from "scene_id" to "scene_ref"`
  message. Usable as a CI gate (non-zero exit on any breaking change), materially weaker
  as a human-readable diagnostic.
- **Fallback, if this tool were rejected:** the brief's suggested fallback — schema
  changes reviewed by humans, plus the CI drift gate (regenerate + diff `gen/`) catching
  any resulting generated-type change — remains available regardless, and is the only
  option for the discriminated-union / `oneOf` shapes this format leans on for envelopes,
  since `json-schema-diff`'s value-space-diff approach was only exercised here against a
  flat object rename, not a `oneOf` change.

## Drift-gate fit

**Same caveat on method as the proto prototype:** the literal drift-gate check
(`git diff --exit-code gen/`) assumes `gen/` is committed; since this task's `gen/` is
deliberately uncommitted (per the "do not commit" instruction), `git diff` against an
untracked path is always empty regardless of whether regeneration changed anything —
false-green. Substituted the same direct check as Task 3: SHA-1 of every generated file,
before and after a full `task generate:jsonschema` rerun.

```
=== before ===
d7befbc99ce44cb5a7f9e354cded423e38eb38bb  contract-spike/jsonschema/gen/go/types.go
d04a7814c071fc423694971572ad3913716a6944  contract-spike/jsonschema/gen/ts/types.ts
=== after `task generate:jsonschema` ===
d7befbc99ce44cb5a7f9e354cded423e38eb38bb  contract-spike/jsonschema/gen/go/types.go
d04a7814c071fc423694971572ad3913716a6944  contract-spike/jsonschema/gen/ts/types.ts
```

Identical bytes. `quicktype`'s output is deterministic given a fixed set of input schema
files + fixed quicktype version, so the drift-gate pattern (regenerate in CI, fail if
`git diff --exit-code gen/` is non-empty once `gen/` is actually committed) is sound in
principle here too. One difference worth flagging versus the proto prototype's gate:
quicktype is a **local npm devDependency** pinned in `package.json`/`bun.lock`
(`quicktype@26.0.0`, exact version resolved and lockfile-pinned by `bun add -d
quicktype`), not a remote-plugin-resolved tool the way `buf generate`'s
`buf.build/protocolbuffers/go` / `buf.build/bufbuild/es` plugins are (Task 3 flagged
those as name-only-pinned, versioned only by whatever the remote registry currently
serves). A JSON Schema drift gate is therefore **more reproducible by construction**: the
generator version itself is lockfile-pinned, so "no diff" can't silently start passing
because an upstream plugin registry served a newer generator version between CI runs —
that whole failure mode doesn't exist here.

## Versions used

| Tool / package | Version |
|---|---|
| `go` | go1.26.4 darwin/arm64 |
| `bun` | 1.3.10 |
| `task` (go-task) | 3.45.4 |
| `quicktype` (npm/bun devDependency, from `bun add -d quicktype`) | 26.0.0 |
| `ajv` (npm/bun dependency, from `bun add ajv`) | 8.20.0 |
| `json-schema-diff` (probed via `bunx`, not a project dependency) | 1.0.0 |
| JSON Schema draft used for authored schemas | 2020-12 |

**Note on `json-schema-diff`'s draft support:** as documented above, `json-schema-diff`
v1.0.0 only validates against draft-07 — the probe in the Breaking-change tooling section
above used draft-07-declared scratch copies of our schemas specifically to exercise the
tool; the actual committed schemas under `schemas/` remain draft 2020-12 throughout, per
the brief's explicit instruction, and were never modified for this probe.
