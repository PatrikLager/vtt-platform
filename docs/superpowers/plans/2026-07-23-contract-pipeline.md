# Production Contract Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the production contract & codegen pipeline decided by ADR-007: a buf module under `contract/`, pinned end-to-end toolchain, committed generated Go/TS/LLM-tool artifacts, drift + breaking gates folded into `task check`, and the wire-convention documentation.

**Architecture:** `contract/` is a buf v2 module holding `vtt.v1` proto schemas (seeded from the spike's proven shapes). Generation runs through **local, lockfile-pinned plugins** (`go tool protoc-gen-go` pinned in go.mod; `protoc-gen-es` pinned in bun.lock) — a deliberate upgrade over the ADR's remote-plugin pinning that fulfills its gate-soundness goal more strongly and works offline. `tools/toolgen` derives MCP tool JSON from proto descriptors with proto3 `optional` as the optionality annotation. All generated output is committed; `check:drift` regenerates and diffs; `check:breaking` runs `buf breaking` against main.

**Tech Stack:** Go 1.26, buf v1.72.0 (via go.mod `tool` directive), protoc-gen-go v1.36.11 (go.mod tool), @bufbuild/protoc-gen-es 2.13.0 + @bufbuild/protobuf (bun), Task, bun test.

## Global Constraints

- Repo: `/Users/patriklager/dev/vtt-platform`; work on a new branch `feat/contract-pipeline` from `main`.
- Go module path: `github.com/PatrikLager/vtt-platform`.
- **Review-before-commit flow (Patrik-approved):** implementers do NOT commit; the task reviewer reviews the staged working-tree diff; the controller commits with `CLAUDE_REVIEW_DONE=1` after approval. Commit steps below mark the intended commit point and message.
- Generated code is committed, never gitignored (`contract/gen/**` including `tools.json`).
- `task check` must pass at the end of every task.
- Pinned versions are LAW (from spike evidence): buf `v1.72.0`, protoc-gen-go `v1.36.11`, `@bufbuild/protoc-gen-es@2.13.0`, `@bufbuild/protobuf` stays lockfile-pinned. No `@latest` anywhere in the production pipeline.
- `contract-spike/` is frozen evidence: read-only, never imported by production code, its Taskfile targets untouched.
- No server/engine code (YAGNI — that's sub-project 2). Only `contract/`, `tools/toolgen/`, `Taskfile.yml`, `go.mod`/`go.sum`, `package.json`/`bun.lock`.
- Package naming: `vtt.v1`, files under `contract/vtt/v1/` — required by buf's `PACKAGE_DIRECTORY_MATCH` lint rule (the ADR's literal `contract/v1/` path would force disabling a lint rule in the constitution repo; this refinement is documented in Task 2 and flagged for ADR-007 cross-reference in Task 5's README).

---

### Task 1: Toolchain pinning + housekeeping

**Files:**
- Modify: `go.mod` (go directive; tool directives), `go.sum`
- Modify: `package.json`, `bun.lock` (add `@bufbuild/protoc-gen-es`)
- Modify: `Taskfile.yml` (simplify `check` guards)

**Interfaces:**
- Consumes: existing repo state on `main`.
- Produces: `go tool buf` (v1.72.0) and `go tool protoc-gen-go` (v1.36.11) invocable from anywhere in the repo; `bunx protoc-gen-es` resolving to 2.13.0; a bare 3-command `check` task. Task 2 depends on all three.

- [ ] **Step 1: Create the branch**

```bash
cd /Users/patriklager/dev/vtt-platform && git checkout -b feat/contract-pipeline main
```

- [ ] **Step 2: Relax the Go version floor** — edit `go.mod` line 3: `go 1.26.4` → `go 1.26` (carried-forward final-review note; the exact-patch pin was a `go mod init` artifact).

- [ ] **Step 3: Pin the toolchain**

```bash
go get -tool github.com/bufbuild/buf/cmd/buf@v1.72.0
go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
bun add -d @bufbuild/protoc-gen-es@2.13.0
```

- [ ] **Step 4: Verify the pins**

Run: `go tool buf --version` — expected `1.72.0`.
Run: `bunx protoc-gen-es --version` — expected `2.13.0` (protoc plugins print version with this flag; if the flag is unsupported, verify via `bun pm ls | grep protoc-gen-es` instead and note it).

- [ ] **Step 5: Simplify the `check` guards** — Go packages and TS test files now exist permanently (`contract-spike/`), so the empty-repo guards are vestigial and carry a documented false-green path. Replace the three guarded cmds in `Taskfile.yml` `check:` with:

```yaml
  check:
    desc: Single quality gateway — every task must leave this green
    cmds:
      - go vet ./...
      - go test ./...
      - bun test
```

- [ ] **Step 6: Run `task check`** — expected green (spike suites: 5 Go tests, 9 TS assertions).

- [ ] **Step 7: Commit point** (controller, after review)

```
chore: pin contract toolchain (buf, protoc-gen-go, protoc-gen-es), relax go floor
```

---

### Task 2: Contract module, v1 schemas, generation

**Files:**
- Create: `contract/buf.yaml`, `contract/buf.gen.yaml`
- Create: `contract/vtt/v1/events.proto`, `contract/vtt/v1/commands.proto`
- Create: `contract/gen/go/vtt/v1/*.pb.go`, `contract/gen/ts/vtt/v1/*_pb.ts` (generated, committed)
- Modify: `Taskfile.yml` (add `generate:contract`)

**Interfaces:**
- Consumes: pinned toolchain from Task 1.
- Produces: Go package `vttv1` at `github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1`; TS module `contract/gen/ts/vtt/v1/{events_pb,commands_pb}.ts` exporting `EnvelopeSchema`, `TokenMovedSchema`, `AttackRolledSchema`, `ActorSchema`, `MoveTokenRequestSchema`, `MoveTokenResponseSchema`; Taskfile target `generate:contract`. Tasks 3–5 depend on these exact names.

- [ ] **Step 1: Write buf configs**

`contract/buf.yaml`:
```yaml
version: v2
lint:
  use: [STANDARD]
breaking:
  use: [FILE]
```

`contract/buf.gen.yaml` — local plugins, lockfile-pinned (deliberate ADR-007 refinement: stronger than `revision:` pinning, offline, and closes the drift-gate pinning gap the evidence flagged):
```yaml
version: v2
plugins:
  - local: [go, tool, protoc-gen-go]
    out: gen/go
    opt: paths=source_relative
  - local: [bunx, protoc-gen-es]
    out: gen/ts
    opt: target=ts
```

- [ ] **Step 2: Write the schemas** — seeded from the spike's proven shapes; changes from the spike: `vtt.v1` package, split events/commands files, `MoveTokenResponse` added, `optional string reason` added to `MoveTokenRequest` (the ADR's optionality-annotation mechanism, exercised from day one).

`contract/vtt/v1/events.proto`:
```proto
syntax = "proto3";

package vtt.v1;

import "google/protobuf/struct.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1;vttv1";

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

// Engine-generic actor: named stats and pools are open maps; module_data is an
// opaque module-owned blob the contract never inspects (see README: Struct rules).
message Actor {
  string actor_id = 1;
  string name = 2;
  string module_id = 3;
  map<string, int32> attributes = 4;
  map<string, Resource> resources = 5;
  google.protobuf.Struct module_data = 6;
}

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
```

`contract/vtt/v1/commands.proto`:
```proto
syntax = "proto3";

package vtt.v1;

import "vtt/v1/events.proto";

option go_package = "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1;vttv1";

message MoveTokenRequest {
  string token_id = 1;
  GridPosition to = 2;
  // Optional DM/agent annotation shown in the log. proto3 `optional` is the
  // contract's optionality annotation: toolgen omits such fields from `required`.
  optional string reason = 3;
}

message MoveTokenResponse {
  bool ok = 1;
  string error = 2;
  TokenMoved event = 3;
}
```

- [ ] **Step 3: Add the Taskfile target**

```yaml
  generate:contract:
    dir: contract
    cmds:
      - go tool buf lint
      - go tool buf generate
```

- [ ] **Step 4: Generate**

Run: `task generate:contract`
Expected: exit 0; `contract/gen/go/vtt/v1/{events,commands}.pb.go` and `contract/gen/ts/vtt/v1/{events_pb,commands_pb}.ts` exist with standard generated headers naming protoc-gen-go v1.36.11 / protoc-gen-es v2.13.0. Then `go mod tidy` if the generated code changed requirements (it should not — protobuf runtime already required).

- [ ] **Step 5: Run `task check`** — expected green (generated code compiles under `go vet`).

- [ ] **Step 6: Commit point** (controller, after review)

```
feat: production contract module vtt.v1 with pinned local-plugin generation
```

---

### Task 3: Production round-trip tests + testdata

**Files:**
- Create: `contract/testdata/token_moved.json`, `attack_rolled.json`, `actor.json`, `move_token_request.json` — byte-for-byte copies of `contract-spike/fixtures/` (production takes ownership; the spike stays frozen)
- Create: `contract/testdata/envelope.json` (new — encodes the wire conventions)
- Create: `contract/roundtrip_test.go`, `contract/events.test.ts`

**Interfaces:**
- Consumes: `vttv1` Go package and TS schemas from Task 2.
- Produces: the production contract test suite; `contract/testdata/` goldens that Task 4 extends and the README (Task 5) references.

- [ ] **Step 1: Copy the four fixtures**

```bash
mkdir -p contract/testdata
cp contract-spike/fixtures/token_moved.json contract-spike/fixtures/attack_rolled.json contract-spike/fixtures/actor.json contract-spike/fixtures/move_token_request.json contract/testdata/
```

- [ ] **Step 2: Write `contract/testdata/envelope.json`** — this file IS the wire-convention spec in executable form: `sequence` as a JSON string (protojson int64), the oneof as an inlined `tokenMoved` key (no `type`/`payload` discriminator):

```json
{
  "eventId": "01J9ZK7M2N3P4Q5R6S7T8U9V0W",
  "sequence": "42",
  "occurredAt": "2026-07-23T19:04:05Z",
  "sessionId": "sess-happy-dragon",
  "actorRole": "agent",
  "tokenMoved": {
    "tokenId": "tok-ursus",
    "sceneId": "scn-goblin-warrens",
    "from": { "x": 3, "y": 7 },
    "to": { "x": 5, "y": 8 }
  }
}
```

- [ ] **Step 3: Write the Go test**

`contract/roundtrip_test.go`:
```go
package contract_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func roundTrip(t *testing.T, fixture string, msg proto.Message) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + fixture)
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

func TestTokenMovedRoundTrip(t *testing.T)  { roundTrip(t, "token_moved.json", &vttv1.TokenMoved{}) }
func TestAttackRolledRoundTrip(t *testing.T) { roundTrip(t, "attack_rolled.json", &vttv1.AttackRolled{}) }
func TestActorRoundTrip(t *testing.T)        { roundTrip(t, "actor.json", &vttv1.Actor{}) }
func TestMoveTokenRequestRoundTrip(t *testing.T) {
	roundTrip(t, "move_token_request.json", &vttv1.MoveTokenRequest{})
}
func TestEnvelopeRoundTrip(t *testing.T) { roundTrip(t, "envelope.json", &vttv1.Envelope{}) }

func TestEnvelopePayloadIsCompilerDiscriminated(t *testing.T) {
	raw, _ := os.ReadFile("testdata/envelope.json")
	var env vttv1.Envelope
	if err := protojson.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	switch p := env.Payload.(type) {
	case *vttv1.Envelope_TokenMoved:
		if p.TokenMoved.TokenId != "tok-ursus" {
			t.Fatalf("wrong token: %s", p.TokenMoved.TokenId)
		}
	default:
		t.Fatalf("expected TokenMoved payload, got %T", p)
	}
}
```

- [ ] **Step 4: Run Go tests**

Run: `go test ./contract/`
Expected: 6/6 PASS. (These are transcription-style tests over already-generated types — the assertion of value is the envelope conventions; a fixture/proto mismatch fails loudly here.)

- [ ] **Step 5: Write the TS test**

`contract/events.test.ts`:
```ts
import { test, expect } from "bun:test";
import { fromJson, toJson } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  TokenMovedSchema,
  AttackRolledSchema,
  ActorSchema,
} from "./gen/ts/vtt/v1/events_pb";
import { MoveTokenRequestSchema } from "./gen/ts/vtt/v1/commands_pb";

const cases = [
  ["token_moved.json", TokenMovedSchema],
  ["attack_rolled.json", AttackRolledSchema],
  ["actor.json", ActorSchema],
  ["move_token_request.json", MoveTokenRequestSchema],
  ["envelope.json", EnvelopeSchema],
] as const;

for (const [fixture, schema] of cases) {
  test(`${fixture} round-trips`, async () => {
    const raw = await Bun.file(
      new URL(`./testdata/${fixture}`, import.meta.url),
    ).json();
    const msg = fromJson(schema as any, raw);
    expect(toJson(schema as any, msg)).toEqual(raw);
  });
}

test("envelope payload is a discriminated case/value pair", async () => {
  const raw = await Bun.file(
    new URL("./testdata/envelope.json", import.meta.url),
  ).json();
  const env = fromJson(EnvelopeSchema, raw);
  expect(env.payload.case).toBe("tokenMoved");
  if (env.payload.case === "tokenMoved") {
    expect(env.payload.value.tokenId).toBe("tok-ursus");
  }
});
```

- [ ] **Step 6: Run TS tests**

Run: `bun test contract/`
Expected: 6/6 PASS. Then `task check` — expected green (spike + contract suites).

- [ ] **Step 7: Commit point** (controller, after review)

```
test: production contract round-trip suite with envelope wire-convention goldens
```

---

### Task 4: toolgen promotion (MCP tool definitions from descriptors)

**Files:**
- Create: `tools/toolgen/main.go`, `tools/toolgen/main_test.go`
- Create: `contract/testdata/expected_tools.json` (golden)
- Create: `contract/gen/tools/tools.json` (generated, committed)
- Modify: `Taskfile.yml` (`generate:contract` gains the toolgen step)

**Interfaces:**
- Consumes: `vttv1` package from Task 2 (`MoveTokenRequest` descriptor; `File_vtt_v1_commands_proto` file descriptor).
- Produces: `contract/gen/tools/tools.json` — the MCP tool definitions the future gateway (sub-project 6) serves; `go run ./tools/toolgen -o <path>` CLI contract.

- [ ] **Step 1: Write the golden** — `contract/testdata/expected_tools.json`. Note `reason` present in properties but ABSENT from `required` (the proto3 `optional` annotation doing its job):

```json
[
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
        },
        "reason": { "type": "string" }
      },
      "required": ["tokenId", "to"]
    }
  }
]
```

- [ ] **Step 2: Write the failing tests**

`tools/toolgen/main_test.go`:
```go
package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func TestToolsMatchGolden(t *testing.T) {
	raw, err := os.ReadFile("../../contract/testdata/expected_tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(buildTools())
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(gotJSON, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("tools mismatch:\nwant %v\ngot  %v", want, got)
	}
}

// Every command message (name ending in "Request") must have a manifest entry —
// forgetting one means the LLM silently loses a capability.
func TestManifestCoversAllCommandMessages(t *testing.T) {
	msgs := vttv1.File_vtt_v1_commands_proto.Messages()
	for i := 0; i < msgs.Len(); i++ {
		name := string(msgs.Get(i).FullName())
		if !strings.HasSuffix(name, "Request") {
			continue
		}
		found := false
		for _, spec := range manifest {
			if spec.message == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("command message %s has no toolgen manifest entry", name)
		}
	}
}
```

Run: `go test ./tools/toolgen/`
Expected: FAIL — `undefined: buildTools`, `undefined: manifest`.

- [ ] **Step 3: Implement**

`tools/toolgen/main.go`:
```go
// Command toolgen derives MCP tool definitions from the contract's protobuf
// descriptors. proto3 `optional` fields (synthetic oneofs) are omitted from
// each tool's `required` list — this is the contract's optionality annotation
// (ADR-007). Output is committed at contract/gen/tools/tools.json and covered
// by the drift gate.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"google.golang.org/protobuf/reflect/protoreflect"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

type toolSpec struct {
	message     string
	name        string
	description string
	descriptor  protoreflect.MessageDescriptor
}

var manifest = []toolSpec{
	{
		message:     "vtt.v1.MoveTokenRequest",
		name:        "move_token",
		description: "Move a token to a new grid position on its scene.",
		descriptor:  (&vttv1.MoveTokenRequest{}).ProtoReflect().Descriptor(),
	},
}

func isOptional(f protoreflect.FieldDescriptor) bool {
	oo := f.ContainingOneof()
	return oo != nil && oo.IsSynthetic()
}

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
		case protoreflect.BoolKind:
			props[name] = map[string]any{"type": "boolean"}
		case protoreflect.Int32Kind, protoreflect.Int64Kind:
			props[name] = map[string]any{"type": "integer"}
		case protoreflect.MessageKind:
			props[name] = schemaFor(f.Message())
		default:
			panic(fmt.Sprintf("toolgen: unhandled kind %v on %s", f.Kind(), f.FullName()))
		}
		if !isOptional(f) {
			required = append(required, name)
		}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func buildTools() []map[string]any {
	var tools []map[string]any
	for _, spec := range manifest {
		tools = append(tools, map[string]any{
			"name":        spec.name,
			"description": spec.description,
			"inputSchema": schemaFor(spec.descriptor),
		})
	}
	return tools
}

func main() {
	out := flag.String("o", "", "output path (default stdout)")
	flag.Parse()
	data, err := json.MarshalIndent(buildTools(), "", "  ")
	if err != nil {
		panic(err)
	}
	data = append(data, '\n')
	if *out == "" {
		fmt.Print(string(data))
		return
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		panic(err)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./tools/toolgen/`
Expected: 2/2 PASS. (If the golden mismatches on `required` ordering or the `reason` field, the failure output names it — fix the golden only if toolgen is right, never the reverse without understanding why.)

- [ ] **Step 5: Wire generation + emit committed output**

`Taskfile.yml` — `generate:contract` becomes:
```yaml
  generate:contract:
    dir: contract
    cmds:
      - go tool buf lint
      - go tool buf generate
      - mkdir -p gen/tools
      - go run ../tools/toolgen -o gen/tools/tools.json
```

Run: `task generate:contract` then verify `contract/gen/tools/tools.json` exists and contains the `move_token` tool (`grep -q '"name": "move_token"' contract/gen/tools/tools.json`). Do NOT literal-diff the file against the golden — Go's `json.Marshal` alphabetizes keys, so byte order differs from the golden's; semantic equality is already enforced by `TestToolsMatchGolden`, and file↔generator binding is enforced by `check:drift` (Task 5).

- [ ] **Step 6: Run `task check`** — expected green.

- [ ] **Step 7: Commit point** (controller, after review)

```
feat: promote toolgen — MCP tool definitions from contract descriptors with optionality
```

---

### Task 5: Drift + breaking gates, wire-convention README

**Files:**
- Modify: `Taskfile.yml` (add `check:drift`, `check:breaking`; fold both into `check`)
- Create: `contract/README.md`

**Interfaces:**
- Consumes: `generate:contract` from Task 4.
- Produces: the enforced pipeline — `task check` now fails on generated-code drift and (once `contract/` is on main) on breaking schema changes.

- [ ] **Step 1: Add the gates**

```yaml
  check:drift:
    desc: Regenerate contract artifacts and fail if committed output drifted
    cmds:
      - task: generate:contract
      - git diff --exit-code -- contract/gen

  check:breaking:
    desc: Fail on breaking changes to contract/ vs main
    cmds:
      - |
        if git ls-tree -d main:contract >/dev/null 2>&1; then
          cd contract && go tool buf breaking --against '../.git#branch=main,subdir=contract'
        else
          echo "check:breaking: contract/ not on main yet; gate activates after first merge"
        fi
```

And `check` becomes:
```yaml
  check:
    desc: Single quality gateway — every task must leave this green
    cmds:
      - go vet ./...
      - go test ./...
      - bun test
      - task: check:drift
      - task: check:breaking
```

- [ ] **Step 2: Prove the drift gate bites** — induce drift, watch it fail, restore. Appending to a generated file cannot demonstrate this: `check:drift` regenerates before diffing, so a hand-edit under `contract/gen/**` is overwritten before the comparison ever runs and the gate reports exit 0 regardless — the gate's semantic is committed-vs-regenerated output, not "the generated tree hasn't been poked." Drift has to be induced at the source (the `.proto`) instead:

```bash
# Temporarily add a field to MoveTokenRequest in contract/vtt/v1/commands.proto:
#   optional string test_drift_field = 4;
task check:drift; echo "exit: $?"
git checkout -- contract/vtt/v1/commands.proto
task check:drift; echo "exit: $?"
```
Expected: first `task check:drift` exits non-zero with `contract/gen` files (regenerated from the tampered schema) showing up in the diff; after `git checkout` restores the proto, the second `task check:drift` regenerates from the committed schema and exits 0. Paste both outcomes into the task report. (The breaking gate cannot bite until `contract/` exists on main — the skip branch will run; verify its message appears and note that post-merge activation was the spike's own evidence-verified behavior.)

- [ ] **Step 3: Write `contract/README.md`** — the wire-convention doc ADR-007 mandates. Full content:

```markdown
# contract/ — the platform's wire constitution

`vtt.v1` protobuf schemas are the single authored source of truth for every
boundary in the system (ADR-007). Generated artifacts are COMMITTED:
`gen/go` (server), `gen/ts` (client), `gen/tools/tools.json` (MCP tool
definitions). Regenerate with `task generate:contract`; CI-equivalent gates:
`task check:drift` (regenerate + diff must be empty) and `task check:breaking`
(`buf breaking` vs main).

## Wire conventions every consumer must know

1. **int64 serializes as a JSON string.** `Envelope.sequence` is `"42"`, not
   `42`, in JSON (protojson convention). Generated consumers handle this;
   hand-rolled ones must.
2. **The event envelope has no `type` field.** The `oneof payload` inlines one
   key per variant: `{"tokenMoved": {...}}` or `{"attackRolled": {...}}`.
   Generated consumers get compiler-checked discrimination (Go type switch;
   TS `payload.case`/`payload.value`); hand-rolled consumers switch on
   presence-of-key. `contract/testdata/envelope.json` is the executable spec.
3. **`Actor.module_data` is an opaque `google.protobuf.Struct`.** The contract
   never inspects it; rule modules own its shape and validate it with their
   own (JSON Schema) content schemas. Go access walks `structpb.Value`; a
   shared helper will live in the engine (sub-project 2), not here.
4. **Optionality annotation:** proto3 `optional` on a field means "optional
   parameter" in derived MCP tools; unannotated fields are required. See
   `tools/toolgen`.

## Evolution rules

Additive changes only: new fields (new numbers), new messages, new oneof
variants. Renames, deletions, number or type changes are breaking —
`task check:breaking` enforces this against main (buf FILE rules). The event
log is forever; the gate is what keeps old campaigns readable.

## Layout note

Files live under `contract/vtt/v1/` (package `vtt.v1`) rather than ADR-007's
literal `contract/v1/` — buf's PACKAGE_DIRECTORY_MATCH lint rule requires the
package path in the directory path, and disabling lint rules in this module
was judged worse than the path refinement.
```

- [ ] **Step 4: Run `task check`** — expected green end-to-end (vet, both test suites, drift regenerate+diff clean, breaking skip-branch message).

- [ ] **Step 5: Commit point** (controller, after review)

```
feat: drift and breaking gates in task check; contract wire-convention README
```

---

## After this plan

Sub-project 1 (contract & codegen pipeline) is complete. Merge via finishing-a-development-branch; the breaking gate arms itself on the merge commit. Next: sub-project 2 (event core), whose plan should consume `vttv1.Envelope` as the log record type and revisit the `Struct`-walking helper named in the README.
