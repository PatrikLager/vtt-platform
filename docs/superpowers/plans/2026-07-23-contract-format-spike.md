# Contract Format Spike Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Decide the platform's contract schema format (protobuf vs JSON Schema vs OpenAPI) by building three working prototypes against one shared benchmark slice, and record the decision as ADR-007 with Patrik's sign-off.

**Architecture:** A `contract-spike/` directory holds shared JSON fixtures (the benchmark slice of the game domain) plus one prototype per candidate format. Each prototype generates Go types and TypeScript types from its schema source and proves them with round-trip tests against the shared fixtures, then derives an LLM tool definition. Evidence from all three feeds a scorecard in ADR-007. The production pipeline is a separate follow-up plan once the format is chosen.

**Tech Stack:** Go (root `go.mod`), Bun + TypeScript (root `package.json`), Task (taskfile.dev), buf + protoc-gen-go + protobuf-es (protobuf prototype), quicktype + ajv (JSON Schema prototype), oapi-codegen + openapi-typescript (OpenAPI prototype).

## Global Constraints

- Repo: `/Users/patriklager/dev/vtt-platform` — all paths below are relative to this root.
- Go module path: `github.com/PatrikLager/vtt-platform` (renamed together with the project later; see spec §9).
- Generated code is **committed**, never gitignored — "regenerate + diff" is the future drift gate (spec §8).
- `task check` must pass at the end of every task (spec §8: single quality gateway).
- Commits are gated by Patrik's dev-cycle hook: after the task's review passes, run the commit prefixed `CLAUDE_REVIEW_DONE=1` exactly as shown in the commit steps.
- Spike code lives only under `contract-spike/` and `docs/adr/` — no `internal/`, no server code yet (YAGNI; that's later sub-projects).
- Tool versions: use latest stable (no pins) except where a command shows an explicit `@latest`; record actual versions used in ADR-007's evidence section.

---

### Task 1: Repo scaffolding + founding ADRs

**Files:**
- Create: `go.mod`, `package.json`, `Taskfile.yml`, `.gitignore`
- Create: `docs/adr/001-api-first-headless-core.md` … `docs/adr/006-foundations-first-build-order.md`
- Test: `task check` runs green (trivially — no code yet)

**Interfaces:**
- Consumes: nothing (first task).
- Produces: root `go.mod` (module `github.com/PatrikLager/vtt-platform`), root `package.json` with bun test wiring, `Taskfile.yml` with `check` task. All later tasks add commands to `Taskfile.yml` and depend on these files existing.

- [ ] **Step 1: Initialize Go module, bun package, gitignore**

```bash
cd /Users/patriklager/dev/vtt-platform
go mod init github.com/PatrikLager/vtt-platform
bun init -y   # creates package.json; delete the generated index.ts if present
rm -f index.ts
```

`.gitignore`:
```
node_modules/
coverage/
*.out
.DS_Store
```

- [ ] **Step 2: Write Taskfile.yml**

```yaml
version: '3'

tasks:
  check:
    desc: Single quality gateway — every task must leave this green
    cmds:
      - go vet ./...
      - go test ./...
      - bun test
```

Note: with no Go packages or TS tests yet, `go vet ./...`/`go test ./...` print "no packages" warnings but exit 0, and `bun test` exits 0 with no tests. That is the expected green state for this task.

- [ ] **Step 3: Write the six founding ADRs**

Each file follows this shape (shown in full for 001; write 002–006 with the analogous content drawn from spec §3, §4, §5 — the "Decision" paragraphs below are given for each):

`docs/adr/001-api-first-headless-core.md`:
```markdown
# ADR-001: API-first, headless core

**Status:** Accepted (founding decision, 2026-07-23)
**Context:** Foundry VTT's automation lives inside its browser client; there is no
sanctioned remote API, which makes an LLM participant a hack. Our defining feature
is an LLM as a first-class participant up to full DM.
**Decision:** Every game action is a structured, permission-checked API operation
with a subscribeable event stream. The human UI is just another API client. The
LLM gateway is derived from the API, never a side channel.
**Consequences:** The API surface must be complete before any UI exists; the
simulation harness (ADR-006) is the first consumer.
```

Decision paragraphs for the remaining five:
- `002-rules-as-declarative-data.md`: Game logic lives in rule-module data interpreted by one engine. No game-system concept appears in engine code (machine-enforced by a semgrep vocabulary ban). Rules are testable, diffable, and LLM-legible/authorable as data.
- `003-event-sourced-state.md`: The append-only game log is the source of truth; state is derived, never mutated in place. Only the event-application package writes state. Yields undo, replay, audit, and the LLM context feed. *(CORRECTED 2026-09-01: ADR-003 no longer says "undo" — see its 2026-09-01 amendment. Undo left the platform in sub-project 13 on Patrik's ruling of 2026-08-30; replay, audit and the LLM context feed still fall out of the log exactly as quoted. This line is a QUOTATION of the ADR made on 2026-07-23, kept so the spike's inputs stay readable as they were.)*
- `004-engine-module-boundary.md`: The engine knows actors, scenes, turns, effects, dice — never healing surges. Proven permanently by a toy second module passing the same conformance suite as D&D 4.5e with zero engine changes.
- `005-two-layer-stack.md`: Go server (auditable core, single-binary deploy) + thin TypeScript browser client, bridged by types generated from a single schema source. Client renders state and submits intents; no rules execution client-side.
- `006-foundations-first-build-order.md`: Platform built horizontally before first play; the simulation harness is built alongside the foundations (not after) so every API is exercised by scripted play from the start. Exit criteria per spec §5.

- [ ] **Step 4: Run the gateway**

Run: `task check`
Expected: exits 0 (no-package warnings acceptable).

- [ ] **Step 5: Commit** (after task review passes)

```bash
git add -A
CLAUDE_REVIEW_DONE=1 git commit -m "chore: scaffold repo, quality gateway, founding ADRs 001-006

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: Benchmark fixtures — the shared slice all prototypes must express

**Files:**
- Create: `contract-spike/fixtures/token_moved.json`, `attack_rolled.json`, `actor.json`, `move_token_request.json`, `expected_tool.json`
- Create: `contract-spike/fixtures/fixtures_test.go`
- Create: `contract-spike/README.md`

**Interfaces:**
- Consumes: root `go.mod` from Task 1.
- Produces: the five fixture files, loaded by every prototype's round-trip tests via relative path `../fixtures/<name>.json` (Go) and `contract-spike/fixtures/<name>.json` (TS from repo root). Field names are camelCase — chosen to match protobuf's protojson mapping so all three formats can share fixtures.

- [ ] **Step 1: Write the fixtures**

`token_moved.json` — simple nested event payload:
```json
{
  "tokenId": "tok-ursus",
  "sceneId": "scn-goblin-warrens",
  "from": { "x": 3, "y": 7 },
  "to": { "x": 5, "y": 8 }
}
```

`attack_rolled.json` — repeated/nested structures:
```json
{
  "attackerId": "tok-ursus",
  "targetId": "tok-goblin-3",
  "expression": "1d20+9",
  "rolls": [ { "die": 20, "result": 14 } ],
  "modifiers": [
    { "source": "STR", "value": 5 },
    { "source": "proficiency", "value": 3 },
    { "source": "combatAdvantage", "value": 2 }
  ],
  "total": 24,
  "versus": "AC",
  "outcome": "hit"
}
```

`actor.json` — the extensibility probe (open maps + arbitrary module blob):
```json
{
  "actorId": "act-ursus",
  "name": "Ursus",
  "moduleId": "dnd45e",
  "attributes": { "strength": 21, "constitution": 16 },
  "resources": {
    "hp": { "current": 88, "max": 102 },
    "surges": { "current": 9, "max": 11 }
  },
  "moduleData": { "rageStance": "BerserkerFury", "feralRejuvenation": true }
}
```

`move_token_request.json` — the API-operation probe:
```json
{
  "tokenId": "tok-ursus",
  "to": { "x": 5, "y": 8 }
}
```

`expected_tool.json` — target shape every prototype must be able to produce for the LLM gateway (MCP tool definition):
```json
{
  "name": "move_token",
  "description": "Move a token to a new grid position on its scene.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "tokenId": { "type": "string" },
      "to": {
        "type": "object",
        "properties": {
          "x": { "type": "integer" },
          "y": { "type": "integer" }
        },
        "required": ["x", "y"]
      }
    },
    "required": ["tokenId", "to"]
  }
}
```

- [ ] **Step 2: Write the failing test** — fixtures are valid JSON and non-empty

`contract-spike/fixtures/fixtures_test.go`:
```go
package fixtures_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFixturesAreValidJSON(t *testing.T) {
	names := []string{
		"token_moved.json", "attack_rolled.json", "actor.json",
		"move_token_request.json", "expected_tool.json",
	}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("%s: invalid JSON: %v", name, err)
		}
		if len(v) == 0 {
			t.Fatalf("%s: empty object", name)
		}
	}
}
```

- [ ] **Step 3: Run test to verify it passes** (fixtures already written in Step 1, so this validates them; if any fixture has a typo the test fails)

Run: `go test ./contract-spike/fixtures/`
Expected: PASS

- [ ] **Step 4: Write `contract-spike/README.md`** — three sentences: what the spike is, that fixtures are the shared benchmark, that each prototype directory is throwaway evidence for ADR-007 (link it).

- [ ] **Step 5: Run `task check`** — expected green.

- [ ] **Step 6: Commit** (after review)

```bash
git add contract-spike/
CLAUDE_REVIEW_DONE=1 git commit -m "feat: add shared benchmark fixtures for contract format spike

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: Protobuf prototype

**Files:**
- Create: `contract-spike/proto/buf.yaml`, `contract-spike/proto/buf.gen.yaml`
- Create: `contract-spike/proto/contract/v1/events.proto`
- Create: `contract-spike/proto/gen/` (generated Go + TS, committed)
- Create: `contract-spike/proto/roundtrip_test.go`, `contract-spike/proto/events.test.ts`
- Create: `contract-spike/proto/toolgen/main.go`, `contract-spike/proto/toolgen/toolgen_test.go`
- Create: `contract-spike/proto/EVIDENCE.md`
- Modify: `Taskfile.yml` (add `generate:proto`), `package.json` (add `@bufbuild/protobuf`)

**Interfaces:**
- Consumes: fixtures from Task 2 (paths above); Taskfile from Task 1.
- Produces: `EVIDENCE.md` with recorded observations (consumed by Task 6's scorecard); generated package `contractv1` (spike-internal only).

- [ ] **Step 1: Install buf and write config**

```bash
go install github.com/bufbuild/buf/cmd/buf@latest
bun add @bufbuild/protobuf
```

`contract-spike/proto/buf.yaml`:
```yaml
version: v2
lint:
  use: [STANDARD]
breaking:
  use: [FILE]
```

`contract-spike/proto/buf.gen.yaml` (remote plugins — no local protoc needed):
```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/bufbuild/es
    out: gen/ts
```

- [ ] **Step 2: Write the schema**

`contract-spike/proto/contract/v1/events.proto`:
```proto
syntax = "proto3";

package contract.v1;

import "google/protobuf/struct.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/PatrikLager/vtt-platform/contract-spike/proto/gen/go/contract/v1;contractv1";

message GridPosition {
  int32 x = 1;
  int32 y = 2;
}

message TokenMoved {
  string token_id = 1;
  string scene_id = 2;
  GridPosition from = 3;
  GridPosition to = 4;
}

message DieRoll {
  int32 die = 1;
  int32 result = 2;
}

message Modifier {
  string source = 1;
  int32 value = 2;
}

message AttackRolled {
  string attacker_id = 1;
  string target_id = 2;
  string expression = 3;
  repeated DieRoll rolls = 4;
  repeated Modifier modifiers = 5;
  int32 total = 6;
  string versus = 7;
  string outcome = 8;
}

message Resource {
  int32 current = 1;
  int32 max = 2;
}

// Extensibility probe: open maps + arbitrary module blob (google.protobuf.Struct).
message Actor {
  string actor_id = 1;
  string name = 2;
  string module_id = 3;
  map<string, int32> attributes = 4;
  map<string, Resource> resources = 5;
  google.protobuf.Struct module_data = 6;
}

// Event-envelope probe: how does proto's oneof shape the wire JSON?
message Envelope {
  string event_id = 1;
  int64 sequence = 2;
  google.protobuf.Timestamp occurred_at = 3;
  string session_id = 4;
  string actor_role = 5;
  oneof payload {
    TokenMoved token_moved = 10;
    AttackRolled attack_rolled = 11;
  }
}

message MoveTokenRequest {
  string token_id = 1;
  GridPosition to = 2;
}
```

- [ ] **Step 3: Generate and vendor the output**

```bash
cd contract-spike/proto && buf lint && buf generate && cd ../..
go get google.golang.org/protobuf@latest
go mod tidy
```

Expected: `gen/go/contract/v1/events.pb.go` and `gen/ts/contract/v1/events_pb.ts` exist.

- [ ] **Step 4: Write the failing Go round-trip test**

`contract-spike/proto/roundtrip_test.go`:
```go
package proto_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	contractv1 "github.com/PatrikLager/vtt-platform/contract-spike/proto/gen/go/contract/v1"
)

// roundTrip unmarshals a fixture into msg via protojson, marshals it back,
// and compares semantically (protojson output ordering is not stable text).
func roundTrip(t *testing.T, fixture string, msg proto.Message) {
	t.Helper()
	raw, err := os.ReadFile("../fixtures/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(raw, msg); err != nil {
		t.Fatalf("unmarshal %s: %v", fixture, err)
	}
	out, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s round-trip mismatch:\nwant %v\ngot  %v", fixture, want, got)
	}
}

func TestTokenMovedRoundTrip(t *testing.T)  { roundTrip(t, "token_moved.json", &contractv1.TokenMoved{}) }
func TestAttackRolledRoundTrip(t *testing.T) { roundTrip(t, "attack_rolled.json", &contractv1.AttackRolled{}) }
func TestActorRoundTrip(t *testing.T)        { roundTrip(t, "actor.json", &contractv1.Actor{}) }
func TestMoveTokenRequestRoundTrip(t *testing.T) {
	roundTrip(t, "move_token_request.json", &contractv1.MoveTokenRequest{})
}
```

- [ ] **Step 5: Run Go tests**

Run: `go test ./contract-spike/proto/`
Expected: PASS. If a fixture field mismatches the proto JSON mapping, the test names the field — fix the proto, regenerate, rerun. Record any friction (e.g., int64 `sequence` serializing as a JSON string in protojson) in `EVIDENCE.md`.

- [ ] **Step 6: Write the TS round-trip test**

`contract-spike/proto/events.test.ts`:
```ts
import { test, expect } from "bun:test";
import { fromJson, toJson } from "@bufbuild/protobuf";
import {
  TokenMovedSchema,
  AttackRolledSchema,
  ActorSchema,
  MoveTokenRequestSchema,
} from "./gen/ts/contract/v1/events_pb";

const cases = [
  ["token_moved.json", TokenMovedSchema],
  ["attack_rolled.json", AttackRolledSchema],
  ["actor.json", ActorSchema],
  ["move_token_request.json", MoveTokenRequestSchema],
] as const;

for (const [fixture, schema] of cases) {
  test(`${fixture} round-trips`, async () => {
    const raw = await Bun.file(
      new URL(`../fixtures/${fixture}`, import.meta.url),
    ).json();
    const msg = fromJson(schema as any, raw);
    expect(toJson(schema as any, msg)).toEqual(raw);
  });
}
```

Run: `bun test contract-spike/proto/`
Expected: PASS.

- [ ] **Step 7: Write the LLM tool-definition generator (the custom-code-cost probe)**

Failing test first — `contract-spike/proto/toolgen/toolgen_test.go`:
```go
package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestToolgenMatchesExpectedTool(t *testing.T) {
	got := buildTool()
	raw, err := os.ReadFile("../../fixtures/expected_tool.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(got)
	var gotMap map[string]any
	_ = json.Unmarshal(gotJSON, &gotMap)
	if !reflect.DeepEqual(want, gotMap) {
		t.Fatalf("tool mismatch:\nwant %v\ngot  %v", want, gotMap)
	}
}
```

Run: `go test ./contract-spike/proto/toolgen/` — expected: FAIL (`buildTool` undefined).

Implementation — `contract-spike/proto/toolgen/main.go` (walks the message descriptor via protoreflect; every line here is evidence of what proto→JSON-Schema costs):
```go
// Command toolgen derives an MCP tool definition for MoveTokenRequest from
// its protobuf descriptor. Exists to measure the custom-code cost of the
// proto -> LLM-tool path; see ../EVIDENCE.md.
package main

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	contractv1 "github.com/PatrikLager/vtt-platform/contract-spike/proto/gen/go/contract/v1"
)

func schemaFor(md protoreflect.MessageDescriptor) map[string]any {
	props := map[string]any{}
	var required []any
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		name := f.JSONName()
		switch f.Kind() {
		case protoreflect.StringKind:
			props[name] = map[string]any{"type": "string"}
		case protoreflect.Int32Kind, protoreflect.Int64Kind:
			props[name] = map[string]any{"type": "integer"}
		case protoreflect.MessageKind:
			props[name] = schemaFor(f.Message())
		default:
			panic(fmt.Sprintf("toolgen: unhandled kind %v", f.Kind()))
		}
		required = append(required, name)
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func buildTool() map[string]any {
	md := (&contractv1.MoveTokenRequest{}).ProtoReflect().Descriptor()
	return map[string]any{
		"name":        "move_token",
		"description": "Move a token to a new grid position on its scene.",
		"inputSchema": schemaFor(md),
	}
}

func main() {
	out, _ := json.MarshalIndent(buildTool(), "", "  ")
	fmt.Println(string(out))
}
```

Note: proto3 cannot express "required" — `toolgen` marks every field required, which happens to match `expected_tool.json` for this message. Record this expressiveness gap in `EVIDENCE.md`; it is scorecard-relevant (LLM tools want required/optional distinctions).

Run: `go test ./contract-spike/proto/toolgen/` — expected: PASS.

- [ ] **Step 8: Probe the envelope and breaking-change detection; write EVIDENCE.md**

Envelope probe (no test — recorded output):
```bash
cat > /tmp/env_probe.go  # or a Go snippet in EVIDENCE.md run via `go run`
```
Marshal an `Envelope{Payload: &contractv1.Envelope_TokenMoved{...}}` with protojson and paste the resulting JSON into `EVIDENCE.md` — the point is to show the oneof wire shape (`"tokenMoved": {...}` instead of our spec-sketch `"type"/"payload"`).

Breaking-change probe:
```bash
cd contract-spike/proto
sed -i '' 's/string scene_id = 2;/string scene_ref = 2;/' contract/v1/events.proto
buf breaking --against '../../.git#branch=main,subdir=contract-spike/proto'
git checkout -- contract/v1/events.proto
```
Expected: buf reports the field rename as a breaking change. Paste the exact output into `EVIDENCE.md`.

`contract-spike/proto/EVIDENCE.md` sections (fill each from the runs above): *Codegen quality (Go/TS)* · *JSON mapping frictions* · *Envelope/oneof wire shape* · *Extensibility (maps + Struct)* · *Tool-derivation cost (lines of custom code, expressiveness gaps)* · *Breaking-change tooling (buf output)* · *Drift-gate fit (regenerate + `git diff --exit-code gen/`)* · *Versions used*.

- [ ] **Step 9: Wire Taskfile + gateway**

Add to `Taskfile.yml`:
```yaml
  generate:proto:
    dir: contract-spike/proto
    cmds:
      - buf lint
      - buf generate
```

Run: `task check` — expected green (Go + TS proto tests included automatically).

- [ ] **Step 10: Commit** (after review)

```bash
git add -A
CLAUDE_REVIEW_DONE=1 git commit -m "feat: protobuf contract prototype with round-trip tests and evidence

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: JSON Schema prototype

**Files:**
- Create: `contract-spike/jsonschema/schemas/token_moved.schema.json`, `attack_rolled.schema.json`, `actor.schema.json`, `move_token_request.schema.json`
- Create: `contract-spike/jsonschema/gen/go/types.go` (generated, committed), `contract-spike/jsonschema/gen/ts/types.ts` (generated, committed)
- Create: `contract-spike/jsonschema/roundtrip_test.go`, `contract-spike/jsonschema/validate.test.ts`
- Create: `contract-spike/jsonschema/toolgen/main.go`, `contract-spike/jsonschema/toolgen/toolgen_test.go`
- Create: `contract-spike/jsonschema/EVIDENCE.md`
- Modify: `Taskfile.yml` (add `generate:jsonschema`), `package.json` (add `ajv`)

**Interfaces:**
- Consumes: fixtures from Task 2.
- Produces: `EVIDENCE.md` for Task 6. Generated Go types are package `jsgen`, TS types exported from `gen/ts/types.ts`.

- [ ] **Step 1: Write the schemas** (draft 2020-12; one file shown in full — the other three follow the same pattern, mirroring the proto shapes from Task 3 exactly: `attack_rolled` with `rolls`/`modifiers` arrays of objects, `actor` with `additionalProperties` maps for `attributes`/`resources` and a free-form `moduleData` object, `move_token_request` with `tokenId` + `to`)

`contract-spike/jsonschema/schemas/token_moved.schema.json`:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://vtt-platform.dev/schemas/token_moved.schema.json",
  "title": "TokenMoved",
  "type": "object",
  "properties": {
    "tokenId": { "type": "string" },
    "sceneId": { "type": "string" },
    "from": { "$ref": "#/$defs/GridPosition" },
    "to": { "$ref": "#/$defs/GridPosition" }
  },
  "required": ["tokenId", "sceneId", "from", "to"],
  "additionalProperties": false,
  "$defs": {
    "GridPosition": {
      "type": "object",
      "properties": {
        "x": { "type": "integer" },
        "y": { "type": "integer" }
      },
      "required": ["x", "y"],
      "additionalProperties": false
    }
  }
}
```

`contract-spike/jsonschema/schemas/move_token_request.schema.json` — written **ref-free** on purpose, so Step 5's toolgen can embed it without a `$ref` resolver:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://vtt-platform.dev/schemas/move_token_request.schema.json",
  "title": "MoveTokenRequest",
  "type": "object",
  "properties": {
    "tokenId": { "type": "string" },
    "to": {
      "type": "object",
      "properties": {
        "x": { "type": "integer" },
        "y": { "type": "integer" }
      },
      "required": ["x", "y"]
    }
  },
  "required": ["tokenId", "to"]
}
```

Extensibility probe in `actor.schema.json` — the two open maps:
```json
"attributes": { "type": "object", "additionalProperties": { "type": "integer" } },
"resources": { "type": "object", "additionalProperties": { "$ref": "#/$defs/Resource" } },
"moduleData": { "type": "object" }
```

Correctness of the two pattern-following schemas (`attack_rolled`, `actor`) is pinned by the fixtures: Step 3's round-trip tests and Step 4's ajv validation fail on any field mismatch.

- [ ] **Step 2: Generate Go and TS types with quicktype**

```bash
bun add -d quicktype
bunx quicktype --src-lang schema --lang go --package jsgen \
  -o contract-spike/jsonschema/gen/go/types.go contract-spike/jsonschema/schemas/*.schema.json
bunx quicktype --src-lang schema --lang typescript --just-types \
  -o contract-spike/jsonschema/gen/ts/types.ts contract-spike/jsonschema/schemas/*.schema.json
```

Inspect both outputs and note quality observations in `EVIDENCE.md` (naming, pointer-vs-value choices in Go, how the open maps came out — this is primary scorecard evidence).

- [ ] **Step 3: Write the failing Go round-trip test**

`contract-spike/jsonschema/roundtrip_test.go` — same semantic-compare pattern as Task 3 but with `encoding/json` and the generated `jsgen` types:
```go
package jsonschema_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	jsgen "github.com/PatrikLager/vtt-platform/contract-spike/jsonschema/gen/go"
)

func roundTrip[T any](t *testing.T, fixture string) {
	t.Helper()
	raw, err := os.ReadFile("../fixtures/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	var typed T
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatalf("unmarshal %s: %v", fixture, err)
	}
	out, err := json.Marshal(typed)
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]any
	_ = json.Unmarshal(raw, &want)
	_ = json.Unmarshal(out, &got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s mismatch:\nwant %v\ngot  %v", fixture, want, got)
	}
}

func TestTokenMoved(t *testing.T)      { roundTrip[jsgen.TokenMoved](t, "token_moved.json") }
func TestAttackRolled(t *testing.T)    { roundTrip[jsgen.AttackRolled](t, "attack_rolled.json") }
func TestActor(t *testing.T)           { roundTrip[jsgen.Actor](t, "actor.json") }
func TestMoveTokenRequest(t *testing.T) { roundTrip[jsgen.MoveTokenRequest](t, "move_token_request.json") }
```

(Adjust type names to what quicktype actually emitted; if they differ from schema titles, record that as codegen friction.)

Run: `go test ./contract-spike/jsonschema/` — expected: PASS after any generation fixes; record fixes made.

- [ ] **Step 4: Write the TS validation test (ajv) — validation is JSON Schema's native superpower; probe it**

```bash
bun add ajv
```

`contract-spike/jsonschema/validate.test.ts`:
```ts
import { test, expect } from "bun:test";
import Ajv2020 from "ajv/dist/2020";
import type { TokenMoved } from "./gen/ts/types";

const ajv = new Ajv2020();

const cases = [
  ["token_moved", "token_moved.json"],
  ["attack_rolled", "attack_rolled.json"],
  ["actor", "actor.json"],
  ["move_token_request", "move_token_request.json"],
] as const;

for (const [schemaName, fixture] of cases) {
  test(`${fixture} validates against ${schemaName}.schema.json`, async () => {
    const schema = await Bun.file(
      new URL(`./schemas/${schemaName}.schema.json`, import.meta.url),
    ).json();
    const data = await Bun.file(
      new URL(`../fixtures/${fixture}`, import.meta.url),
    ).json();
    const validate = ajv.compile(schema);
    expect(validate(data)).toBe(true);
  });
}

test("generated TS types are consumable", async () => {
  const data = (await Bun.file(
    new URL("../fixtures/token_moved.json", import.meta.url),
  ).json()) as TokenMoved;
  expect(data.tokenId).toBe("tok-ursus");
});
```

Run: `bun test contract-spike/jsonschema/` — expected: PASS.

- [ ] **Step 5: Write the tool-definition generator** — same test harness as Task 3's Step 7 (copy `toolgen_test.go`, fixture path `../../fixtures/expected_tool.json`). Implementation is the punchline of this format:

`contract-spike/jsonschema/toolgen/main.go`:
```go
// Command toolgen derives the MCP tool definition for MoveTokenRequest.
// With JSON Schema as the source format this is embedding, not generation —
// the schema IS the inputSchema. Compare ../proto/toolgen/main.go.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func buildTool() map[string]any {
	raw, err := os.ReadFile("../schemas/move_token_request.schema.json")
	if err != nil {
		panic(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		panic(err)
	}
	// Strip metadata keys irrelevant to a tool inputSchema.
	for _, k := range []string{"$schema", "$id", "title"} {
		delete(schema, k)
	}
	return map[string]any{
		"name":        "move_token",
		"description": "Move a token to a new grid position on its scene.",
		"inputSchema": schema,
	}
}

func main() {
	out, _ := json.MarshalIndent(buildTool(), "", "  ")
	fmt.Println(string(out))
}
```

Note: the test needs the working directory trick — read the schema relative to the test file (`../schemas/...` works because `go test` runs in the package dir). If `move_token_request.schema.json` uses `$defs`/`$ref`, either inline `GridPosition` in that schema or add a ~10-line ref-inliner — record whichever was needed in `EVIDENCE.md` (it is the honest cost of this path).

Run: `go test ./contract-spike/jsonschema/toolgen/` — expected: PASS.

- [ ] **Step 6: Probe breaking-change detection; write EVIDENCE.md**

There is no buf-equivalent for JSON Schema. Probe what exists and record findings:
```bash
bunx json-schema-diff --help   # if the package resolves, diff a renamed-field variant
```
If no usable tool: document the fallback (schema changes reviewed by humans + the CI drift gate catching generated-type changes) and score accordingly. Fill `EVIDENCE.md` with the same section headings as Task 3.

- [ ] **Step 7: Wire Taskfile** — add `generate:jsonschema` running the two quicktype commands from Step 2. Run `task check` — expected green.

- [ ] **Step 8: Commit** (after review)

```bash
git add -A
CLAUDE_REVIEW_DONE=1 git commit -m "feat: JSON Schema contract prototype with round-trip tests and evidence

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: OpenAPI prototype (deliberately smaller — the event-stream-fit probe)

**Files:**
- Create: `contract-spike/openapi/openapi.yaml`
- Create: `contract-spike/openapi/gen/go/types.gen.go` (generated, committed), `contract-spike/openapi/gen/ts/types.ts` (generated, committed)
- Create: `contract-spike/openapi/roundtrip_test.go`
- Create: `contract-spike/openapi/EVIDENCE.md`
- Modify: `Taskfile.yml` (add `generate:openapi`)

**Interfaces:**
- Consumes: fixtures from Task 2.
- Produces: `EVIDENCE.md` for Task 6. Generated Go types are package `oagen`.

- [ ] **Step 1: Write the spec** — OpenAPI 3.1 (JSON Schema dialect), modeling the one command as an HTTP operation and the schemas as components:

`contract-spike/openapi/openapi.yaml`:
```yaml
openapi: 3.1.0
info:
  title: vtt-platform contract spike
  version: 0.0.1
paths:
  /sessions/{sessionId}/tokens/{tokenId}/move:
    post:
      operationId: moveToken
      parameters:
        - name: sessionId
          in: path
          required: true
          schema: { type: string }
        - name: tokenId
          in: path
          required: true
          schema: { type: string }
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/MoveTokenRequest' }
      responses:
        '200':
          description: Token moved
          content:
            application/json:
              schema: { $ref: '#/components/schemas/TokenMoved' }
components:
  schemas:
    GridPosition:
      type: object
      properties:
        x: { type: integer }
        y: { type: integer }
      required: [x, y]
    MoveTokenRequest:
      type: object
      properties:
        tokenId: { type: string }
        to: { $ref: '#/components/schemas/GridPosition' }
      required: [tokenId, to]
    TokenMoved:
      type: object
      properties:
        tokenId: { type: string }
        sceneId: { type: string }
        from: { $ref: '#/components/schemas/GridPosition' }
        to: { $ref: '#/components/schemas/GridPosition' }
      required: [tokenId, sceneId, from, to]
    AttackRolled:
      type: object
      properties:
        attackerId: { type: string }
        targetId: { type: string }
        expression: { type: string }
        rolls:
          type: array
          items:
            type: object
            properties:
              die: { type: integer }
              result: { type: integer }
            required: [die, result]
        modifiers:
          type: array
          items:
            type: object
            properties:
              source: { type: string }
              value: { type: integer }
            required: [source, value]
        total: { type: integer }
        versus: { type: string }
        outcome: { type: string }
      required: [attackerId, targetId, expression, rolls, modifiers, total, versus, outcome]
    Actor:
      type: object
      properties:
        actorId: { type: string }
        name: { type: string }
        moduleId: { type: string }
        attributes:
          type: object
          additionalProperties: { type: integer }
        resources:
          type: object
          additionalProperties:
            type: object
            properties:
              current: { type: integer }
              max: { type: integer }
            required: [current, max]
        moduleData: { type: object }
      required: [actorId, name, moduleId, attributes, resources]
```

- [ ] **Step 2: Generate Go and TS**

```bash
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
  -generate types -package oagen \
  -o contract-spike/openapi/gen/go/types.gen.go contract-spike/openapi/openapi.yaml
bun add -d openapi-typescript
bunx openapi-typescript contract-spike/openapi/openapi.yaml \
  -o contract-spike/openapi/gen/ts/types.ts
go mod tidy
```

- [ ] **Step 3: Write the Go round-trip test**

`contract-spike/openapi/roundtrip_test.go`:
```go
package openapi_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	oagen "github.com/PatrikLager/vtt-platform/contract-spike/openapi/gen/go"
)

func roundTrip[T any](t *testing.T, fixture string) {
	t.Helper()
	raw, err := os.ReadFile("../fixtures/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	var typed T
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatalf("unmarshal %s: %v", fixture, err)
	}
	out, err := json.Marshal(typed)
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]any
	_ = json.Unmarshal(raw, &want)
	_ = json.Unmarshal(out, &got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s mismatch:\nwant %v\ngot  %v", fixture, want, got)
	}
}

func TestTokenMoved(t *testing.T)       { roundTrip[oagen.TokenMoved](t, "token_moved.json") }
func TestAttackRolled(t *testing.T)     { roundTrip[oagen.AttackRolled](t, "attack_rolled.json") }
func TestActor(t *testing.T)            { roundTrip[oagen.Actor](t, "actor.json") }
func TestMoveTokenRequest(t *testing.T) { roundTrip[oagen.MoveTokenRequest](t, "move_token_request.json") }
```

(Adjust type names to oapi-codegen's actual output; record deviations as codegen friction.)

Run: `go test ./contract-spike/openapi/`
Expected: PASS; record generation quality notes in `EVIDENCE.md`.

- [ ] **Step 4: Probe event-stream fit and tooling; write EVIDENCE.md**

No code — this format's core weakness must be documented from the artifacts:
- Where would `Envelope` (the WebSocket event stream) live? OpenAPI 3.1 models HTTP request/response; note the `webhooks` section and its awkwardness for a bidirectional WebSocket stream; note AsyncAPI exists as the event-native sibling and would mean maintaining TWO spec documents (OpenAPI for HTTP + AsyncAPI for events) — record this as the headline finding.
- Tool derivation: components schemas are JSON Schema in 3.1, so the Task 4 embedding approach applies — but note the extraction step (pulling a schema out of the spec document and resolving `$ref`s).
- Breaking-change tooling: note `oasdiff` exists for OpenAPI; run `go run github.com/oasdiff/oasdiff@latest breaking --help` and record whether it is usable.

- [ ] **Step 5: Wire Taskfile** (`generate:openapi` with the two Step 2 commands), run `task check` — expected green.

- [ ] **Step 6: Commit** (after review)

```bash
git add -A
CLAUDE_REVIEW_DONE=1 git commit -m "feat: OpenAPI contract prototype with round-trip tests and evidence

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 6: Scorecard, ADR-007, decision sign-off

**Files:**
- Create: `docs/adr/007-contract-format.md`
- Modify: `contract-spike/README.md` (link the ADR as the spike's outcome)

**Interfaces:**
- Consumes: the three `EVIDENCE.md` files from Tasks 3–5.
- Produces: the accepted contract-format decision that the follow-up pipeline plan builds on.

- [ ] **Step 1: Write ADR-007 from the evidence** — full skeleton (fill every cell from the three EVIDENCE.md files; no cell may say TBD):

```markdown
# ADR-007: Contract schema format

**Status:** Proposed (awaiting Patrik sign-off)
**Context:** Spec §4.1 defers the contract format to comparative prototyping.
Three prototypes were built against a shared benchmark slice
(contract-spike/fixtures/): protobuf (buf), JSON Schema 2020-12 (quicktype/ajv),
OpenAPI 3.1 (oapi-codegen/openapi-typescript). Evidence:
contract-spike/{proto,jsonschema,openapi}/EVIDENCE.md.

## Scorecard

Scores 1 (poor) – 5 (excellent), justified by linked evidence, not vibes.

| Criterion                                   | protobuf | JSON Schema | OpenAPI |
|---------------------------------------------|----------|-------------|---------|
| Go codegen quality                          |          |             |         |
| TS codegen quality                          |          |             |         |
| LLM tool derivation (custom-code cost)      |          |             |         |
| Event-envelope / stream fit                 |          |             |         |
| Extensible module content (open maps/blobs) |          |             |         |
| Breaking-change tooling                     |          |             |         |
| Drift-gate simplicity                       |          |             |         |
| Edit→generate DX loop                       |          |             |         |
| Ecosystem longevity / stability             |          |             |         |

## Decision

<the winning format, with the 2–3 findings that decided it, and what we accept
as its known costs — e.g., if protobuf: the required/optional gap in tool
derivation; if JSON Schema: absent breaking-change tooling, mitigated by the
drift gate + schema review>

## Consequences

<what the production pipeline plan (sub-project 1, plan 2) must build:
generator toolchain, Taskfile targets, CI drift gate shape, schema layout
under contract/ — concretely for the chosen format>

## Fate of the losing prototypes

contract-spike/ is retained as decision evidence, excluded from coverage
gates, and never imported by production code.
```

- [ ] **Step 2: Self-check the ADR** — every scorecard cell filled and traceable to an EVIDENCE.md line; Decision names known costs, not just strengths; Consequences concrete enough to seed the next plan.

- [ ] **Step 3: Present to Patrik for sign-off** — schemas are decided together (spec §6); this is the gate. Present the scorecard and recommendation; on approval flip **Status** to `Accepted (2026-MM-DD)`. If Patrik overrides the recommendation, record his rationale in the Decision section — the ADR documents the decision made, not the one proposed.

- [ ] **Step 4: Run `task check`** — expected green.

- [ ] **Step 5: Commit** (after review + sign-off)

```bash
git add docs/adr/007-contract-format.md contract-spike/README.md
CLAUDE_REVIEW_DONE=1 git commit -m "docs: ADR-007 contract format decision from spike evidence

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## After this plan

Write the follow-up plan (`docs/superpowers/plans/YYYY-MM-DD-contract-pipeline.md`) for the production contract & codegen pipeline — generators wired into `contract/`, versioning discipline, and the CI drift gate — every step concrete now that ADR-007 names the format. That plan completes sub-project 1.
