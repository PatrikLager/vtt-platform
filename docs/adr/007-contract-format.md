# ADR-007: Contract schema format

**Status:** Accepted (signed off by Patrik, 2026-07-23)
**Context:** Spec §4.1 defers the contract format to comparative prototyping.
Three prototypes were built against a shared benchmark slice
(contract-spike/fixtures/): protobuf (buf), JSON Schema 2020-12 (quicktype/ajv),
OpenAPI 3.1 (oapi-codegen/openapi-typescript). Evidence:
contract-spike/{proto,jsonschema,openapi}/EVIDENCE.md.

The contract feeds three consumers (Go server, thin TS browser client, LLM/MCP
tool definitions), the primary wire surface is a WebSocket event stream plus
HTTP, rule modules need open/extensible content, CI needs a regenerate-and-diff
drift gate, and breaking-change discipline matters because the schema is the
system's constitution (spec §4.1–§4.6).

## Scorecard

Scores 1 (poor) – 5 (excellent), justified by linked evidence, not vibes.

| Criterion                                   | protobuf | JSON Schema | OpenAPI |
|---------------------------------------------|----------|-------------|---------|
| Go codegen quality                          | 5        | 3           | 3       |
| TS codegen quality                          | 4        | 4           | 2       |
| LLM tool derivation (custom-code cost)      | 2        | 5           | 3       |
| Event-envelope / stream fit                 | 4        | 3           | 1       |
| Extensible module content (open maps/blobs) | 3        | 5           | 3       |
| Breaking-change tooling                     | 5        | 2           | 2       |
| Drift-gate simplicity                       | 4        | 5           | 2       |
| Edit→generate DX loop                       | 4        | 4           | 2       |
| Ecosystem longevity / stability             | 4        | 3           | 4       |

### Score justifications (traceable to EVIDENCE.md)

**Go codegen quality — proto 5, JSON Schema 3, OpenAPI 3.**
Proto: 805 lines of idiomatic generated Go, one struct per message, `oneof`
rendered as a real interface with per-variant wrappers, standard
`snake_case`→`UpperCamelCase` naming, zero Go-side frictions
(proto/EVIDENCE.md, Codegen quality). JSON Schema: compiles and round-trips
cleanly, but nested `$defs` type names are unstable without `title` discipline
(`GridPosition`→`From`/`To`, `DieRoll`→`RollElement`, `Resource`→`ActorSchema`
— reproduced from the brief's own schema), the schema's openness signal
"silently vanishes in Go" (open in TS, structurally closed in Go, no warning),
and quicktype prints a confusing "cannot infer" non-error on `moduleData`
(jsonschema/EVIDENCE.md, Codegen quality + JSON mapping frictions). OpenAPI:
post-fix output is idiomatic 58-line Go, but the default `-generate types`
silently prunes path-unreachable schemas — the brief's exact command produced
code that did not compile against the brief's own test until `skip-prune` was
added, and inline shapes become anonymous inline structs
(openapi/EVIDENCE.md, Codegen quality).

**TS codegen quality — proto 4, JSON Schema 4, OpenAPI 2.**
Proto: full runtime serialization (`fromJson`/`toJson`) plus a generated
`case`/`value` discriminator pair on unions, but protoc-gen-es v2's
schema-descriptor API "loses type inference across a heterogeneous list (hence
the `as any` casts)" and the default output target is `js+dts`, not the
expected `.ts` (proto/EVIDENCE.md, Codegen quality). JSON Schema: clean,
directly importable `export interface` declarations whose index signatures
faithfully preserve schema openness, but zero runtime code ships
(`--just-types` — validation is bring-your-own ajv) and the same nested naming
drift appears here too (jsonschema/EVIDENCE.md, Codegen quality). OpenAPI: "no
flat top-level type exports at all" — every consumer must write
`components["schemas"]["TokenMoved"]` or hand-roll aliases (fix flag
`--root-types` probed, not default), and the open `moduleData` field generates
`Record<string, never>` — "empty object, statically enforced," the opposite of
the spec's intent, silently (openapi/EVIDENCE.md, Codegen quality + JSON
mapping frictions).

**LLM tool derivation — proto 2, JSON Schema 5, OpenAPI 3.**
Proto: ~29 lines of `protoreflect` descriptor-walking custom code, and — the
decisive gap — "proto3 has no concept of optional vs. required scalar fields,"
so toolgen marks every field required unconditionally; real optional tool
parameters need hand-authored overrides or custom field options, i.e. more
custom code (proto/EVIDENCE.md, Tool-derivation cost). JSON Schema: ~10 lines
— read, strip three metadata keys, wrap; "tool derivation degrades from
generation to embedding" because the authored format *is* the MCP
`inputSchema` format, and the `required` array carries author intent through
verbatim at zero cost. Caveat honestly noted in the evidence: the near-zero
cost "only holds cleanly for ref-free schemas" — `$defs` usage forces a
resolver/inliner (jsonschema/EVIDENCE.md, Tool-derivation cost). OpenAPI: same
embedding argument applies in principle (3.1 schemas are JSON Schema 2020-12),
but no toolgen was built (evidence is thin here — scored conservatively), and
the probe confirmed the `$ref` resolver is "the median case, not the
exception" for a consolidated spec document (openapi/EVIDENCE.md,
Tool-derivation cost).

**Event-envelope / stream fit — proto 4, JSON Schema 3, OpenAPI 1.**
This criterion carries the most weight: the WS event stream is the platform's
primary wire surface. Proto: `oneof` is first-class and generates real
discrimination in both languages (Go interface + type switch, TS
`case`/`value` pair). Two real costs: the wire shape inlines the variant key
(`"tokenMoved": {...}`) rather than a generic `type`/`payload` discriminator,
so non-generated consumers must test for presence of one of N keys; and
`int64 sequence` serializes as JSON string `"42"` under protojson
(proto/EVIDENCE.md, Envelope/oneof wire shape + JSON mapping frictions). JSON
Schema: ajv validates a `oneOf`+discriminator envelope correctly at runtime,
but quicktype "does not generate a discriminated union from `oneOf`" — both
languages get one merged all-optional type where reading the wrong branch's
field still type-checks; the evidence calls this "a genuine regression from
the proto prototype's finding," leaving hand-written discrimination "with no
compiler backing" (jsonschema/EVIDENCE.md, Envelope/stream fit). OpenAPI: "no
primitive … for a client holding open an Envelope-multiplexed WebSocket";
`webhooks` models the wrong direction and transport; the envelope "has nowhere
to live as a modeled concept"; the honest fix is a second spec document
(AsyncAPI) — "two spec documents, two generators, two drift gates"
(openapi/EVIDENCE.md, Envelope/stream fit).

**Extensible module content — proto 3, JSON Schema 5, OpenAPI 3.**
Proto: maps and `google.protobuf.Struct` round-trip cleanly with no schema
changes — the escape hatch "works as advertised" — but programmatic use of
`module_data` must walk the nested `Value`-oneof wrapper: type safety is lost
"exactly where you deliberately asked for openness," plus wrapper overhead
(proto/EVIDENCE.md, Extensibility). JSON Schema: the format's "main structural
advantage" — `additionalProperties` maps to native `map[string]int64` /
index-signature types directly and bare-`object` `moduleData` becomes plain
`map[string]interface{}` / `unknown`, "because the wire format and the schema
format are the same format — there is no analogue to `Struct`/`Value` needed"
(jsonschema/EVIDENCE.md, Extensibility). OpenAPI: identical primitives (3.1
schemas are JSON Schema) and Go output is genuinely open, but the default TS
output turns the open blob into `Record<string, never>` — silently closed —
requiring the `--empty-objects-unknown` flag to correct
(openapi/EVIDENCE.md, JSON mapping frictions + Extensibility).

**Breaking-change tooling — proto 5, JSON Schema 2, OpenAPI 2.**
Proto: `buf breaking` caught the probe rename with two precise field-level
findings including the derived `json_name` change ("the one that actually
matters for wire compatibility"), exits non-zero unconditionally, "with zero
hand-written detection logic" (proto/EVIDENCE.md, Breaking-change tooling).
JSON Schema: the only plausible off-the-shelf tool, `json-schema-diff` v1.0.0,
does not support draft 2020-12 at all (draft-07 only — our schemas as authored
fail outright), and even on downgraded copies its output is a "set-theoretic
whole-schema diff … materially weaker as a human-readable diagnostic"; the
fallback is human review plus the drift gate (jsonschema/EVIDENCE.md,
Breaking-change tooling). OpenAPI: `oasdiff` gives buf-quality field-level
messages for reachable schemas, but exits 0 by default on breaking changes (a
CI-wiring trap requiring `--fail-on ERR`), and is structurally blind to
path-unreachable schemas — "the specific set of schemas this project would
most need breaking-change protection for," i.e. the event stream
(openapi/EVIDENCE.md, Breaking-change tooling).

**Drift-gate simplicity — proto 4, JSON Schema 5, OpenAPI 2.**
All three regenerated byte-identically (SHA-verified). Proto: sound, but
remote plugins are "pinned by name only … a real drift gate would want a
pinned plugin revision" (proto/EVIDENCE.md, Drift-gate fit). JSON Schema:
"more reproducible by construction" — quicktype is an exact-version lockfile
devDependency, so the upstream-registry failure mode "doesn't exist here"
(jsonschema/EVIDENCE.md, Drift-gate fit). OpenAPI: the Go generator runs via
`go run …@latest`, re-resolving on every invocation with "no lockfile-style
pin at all" — "strictly weaker" than proto's name-pinning, the most exposed of
the three (openapi/EVIDENCE.md, Drift-gate fit).

**Edit→generate DX loop — proto 4, JSON Schema 4, OpenAPI 2.**
No EVIDENCE.md section maps 1:1 to this criterion; it is synthesized from the
codegen and versions sections — scored conservatively. Proto: one `buf
generate` with remote plugins ("no local `protoc` install needed"), `buf lint`
built in; frictions were the `js+dts` default target and buf living off-PATH
in `$(go env GOPATH)/bin` (proto/EVIDENCE.md, Codegen quality + Versions).
JSON Schema: one quicktype invocation per language, "no local schema server,
no `$ref` resolver, no plugin registry"; frictions were the `title` authoring
discipline and the misleading non-fatal warning (jsonschema/EVIDENCE.md,
Codegen quality). OpenAPI: two generators from two ecosystems, and *both*
shipped defaults a real consumer must override (Go: silent pruning that broke
compilation; TS: no root types, closed open-objects) — "both overrides exist
as one documented flag away," but the loop starts broken
(openapi/EVIDENCE.md, Codegen quality).

**Ecosystem longevity / stability — proto 4, JSON Schema 3, OpenAPI 4.**
Evidence here is thin for all three (version tables plus indirect signals) —
scored conservatively. Proto: buf 1.72.0 / protobuf-go v1.36.11 /
protoc-gen-es v2.13.0, all current and actively maintained, with the mature
`buf breaking` as a proxy for toolchain seriousness; docked one point because
protoc-gen-es v2's deliberate API redesign (classes → schema descriptors) is
recent churn we directly paid for (proto/EVIDENCE.md, Codegen quality +
Versions). JSON Schema: quicktype 26.0.0 and ajv 8.20.0 are active, but the
breaking-change corner is effectively abandoned — `json-schema-diff` v1.0.0,
"last published years ago," pre-2020-12 (jsonschema/EVIDENCE.md,
Breaking-change tooling + Versions). OpenAPI: oapi-codegen v2.8.0,
openapi-typescript 7.13.0, oasdiff v1.25.1 — all current and active; the
format's weaknesses here are structural, not longevity-related
(openapi/EVIDENCE.md, Versions).

## Decision

**Protobuf (buf), authored as `.proto` under `contract/`, is the adopted
contract format.**

OpenAPI is eliminated first, and cleanly: the primary wire surface of this
platform is an event stream, and OpenAPI has no way to model it — the envelope
"has nowhere to live," its premier breaking-change tool structurally cannot
see the event schemas, and the honest fix is maintaining a second spec format
(AsyncAPI) with a second toolchain and second drift gate. It scores last on
nearly every criterion that is not inherited from JSON Schema.

Between protobuf and JSON Schema the unweighted column sums are a near-tie
(35 vs 34; OpenAPI 22), so the totals do not decide — the spec's priorities
do. The three
findings that decided it:

1. **The event stream is the primary wire surface, and only protobuf's codegen
   protects it.** Proto `oneof` produced compiler-backed discrimination in
   both Go and TS; quicktype's `oneOf` output collapsed to a merged
   all-optional type in both languages — the evidence's own words: "a genuine
   regression," hand-written discrimination "with no compiler backing." The
   thin client and the Go engine both live on this envelope; this is where
   type safety pays most.
2. **The schema is the constitution, and only protobuf has enforceable
   breaking-change discipline.** `buf breaking` is precise, field-level,
   wire-aware (`json_name`), and gate-by-default with zero custom code. JSON
   Schema's only off-the-shelf tool cannot parse the draft we author in; the
   fallback is human review — an honor-system rule, exactly the kind that
   erodes. A constitution needs a court, not a suggestion box.
3. **Protobuf's losses are cheaply mitigable; JSON Schema's are not.** Where
   JSON Schema wins (tool derivation 5 vs 2, extensibility 5 vs 3, drift-gate
   pinning 5 vs 4), the protobuf-side gap closes with bounded, one-time work:
   a toolgen that already exists at ~29 lines plus an explicit optionality
   annotation, a small `Struct`-walking helper, a one-line `revision:` pin.
   Where protobuf wins (envelope, breaking-change), the JSON-Schema-side gap
   has no cheap fix: recovering compile-time union safety means replacing or
   wrapping the generator, and a 2020-12-aware breaking-change differ is a
   project, not a patch.

**Accepted known costs** (all from proto/EVIDENCE.md, to be mitigated in the
production pipeline, not wished away):

- **The required/optional gap in tool derivation.** proto3 cannot express
  caller-facing optionality, so MCP tool definitions with genuinely optional
  parameters need an explicit annotation mechanism (custom field options or
  equivalent) layered into toolgen — accepted as bounded custom code.
- **protojson's int64-as-string convention.** 64-bit integer fields (e.g.
  `Envelope.sequence`) serialize as JSON strings; every hand-rolled consumer
  must know this. Accepted as a documented wire convention.
- **The oneof wire shape.** No generic `type`/`payload` discriminator on the
  wire; non-generated consumers must switch on presence-of-one-of-N keys.
  Accepted as a documented envelope convention (generated consumers get it
  for free).
- **`Struct`/`Value` wrapper cost on module blobs.** Programmatic access to
  `module_data` walks a `Value` oneof rather than a native map. Accepted;
  a small shared helper amortizes it.
- **Remote plugin pinning and toolchain PATH.** `buf.gen.yaml` must pin plugin
  revisions for the drift gate to be sound, buf itself must be
  version-pinned, and `$(go env GOPATH)/bin` must be on PATH in dev and CI.
- **protoc-gen-es v2 ergonomics.** Schema-descriptor-style TS loses inference
  over heterogeneous message lists; accepted, since the client consumes typed
  envelopes (where the generated `case`/`value` pair helps), not generic
  message lists.

Note the losers do not vanish from the architecture: JSON Schema remains
present as toolgen's *output* format (MCP `inputSchema` is JSON Schema), and
plain HTTP endpoint shapes are still described by the same `.proto` messages.
What is decided here is the single authored source of truth.

## Consequences

The production contract & codegen pipeline plan (sub-project 1, plan 2) must
build, concretely:

- **Schema layout under `contract/`:** buf module rooted at `contract/` —
  `contract/buf.yaml` (lint + `breaking` config), `contract/buf.gen.yaml`,
  schemas in `contract/v1/*.proto` (starting from the spike's `events.proto`
  shape: envelope with `oneof` payload, commands, actors, module-content
  messages), generated output **committed** under `contract/gen/go/` and
  `contract/gen/ts/` (a committed `gen/` is what makes the drift gate real —
  the spike could only SHA-check because nothing was committed).
- **Generator toolchain, pinned end-to-end:** buf installed at a pinned
  version (`go install github.com/bufbuild/buf/cmd/buf@v1.72.0`, not
  `@latest`), remote plugins `buf.build/protocolbuffers/go` and
  `buf.build/bufbuild/es` pinned by `revision:` in `buf.gen.yaml` (the spike
  pinned by name only — flagged in evidence as a gate-soundness hole), and
  `opt: target=ts` on the es plugin so a single `.ts` file is emitted.
  `@bufbuild/protobuf` stays lockfile-pinned in `package.json`/`bun.lock`.
- **Taskfile targets:** `generate:contract` (`buf lint` + `buf generate`),
  `check:drift` (regenerate, then `git diff --exit-code contract/gen/`),
  `check:breaking` (`buf breaking --against '.git#branch=main,subdir=contract'`
  — the spike's failure of this exact form was only because nothing was on
  `main` yet; once `contract/` is committed it works as designed). Both checks
  fold into `task check` per the Section 8 single-gateway rule.
- **toolgen promoted to production:** move the spike's descriptor-walking
  generator to a real package (e.g. `tools/toolgen`), extend it with the
  explicit optionality annotation (custom field option) decided above, emit
  MCP tool JSON for every command message, and golden-file-test each emitted
  tool the way the spike tested `expected_tool.json`. Generated tool JSON is
  committed and covered by the same drift gate.
- **Wire-convention documentation** (in `contract/README.md`): int64-as-JSON-
  string, the oneof envelope key convention, and `Struct` usage rules for
  `module_data` — the three consumer surprises the evidence caught, written
  down where every consumer author will see them.
- **CI:** drift gate and breaking gate as required checks; buf version and
  PATH setup baked into the CI environment.

## Fate of the losing prototypes

contract-spike/ is retained as decision evidence, excluded from coverage
gates, and never imported by production code. The spike-only Taskfile targets
(`generate:proto`, `generate:jsonschema`, `generate:openapi`) remain solely to
reproduce the evidence and stay outside `task check`.
