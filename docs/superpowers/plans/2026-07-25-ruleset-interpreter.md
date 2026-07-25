# Ruleset Loader & Rules Interpreter Implementation Plan (sub-project 5a)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver 5a per the approved spec (`docs/superpowers/specs/2026-07-25-ruleset-interpreter-design.md`): the generic rules vocabulary in the contract, engine folds, AppendBatch, the ruleset format + interpreter, the toy ruleset + conformance suite (the P4 proof), and full wiring through gateway/harness/MCP.

**Architecture:** `internal/rules` (loader with embedded format schemas, closed expression grammar, pure Resolve) between the gateway and campaign.AppendBatch; engine grows three generic folds; every game word lives in `rulesets/*/` JSON only.

**Tech Stack:** existing. JSON Schema validation for the ruleset format: prefer stdlib-only strict decoding + hand validation if a maintained Go JSON-Schema validator would add a heavy dep — DECIDED AT TASK 4 with the controller's sign-off if a new dep is wanted (santhosh-tekuri/jsonschema is the known candidate; pin exact if adopted).

## Global Constraints

- Branch `feat/ruleset-interpreter` from `main`. Adapted review-before-commit flow; reviewers READ-ONLY everywhere; injections in throwaway rsync copies.
- ADR-009 binding: stub-first behavioral RED; injection proofs for after-the-fact tests; boundary behavior only.
- **Vocabulary ban is the design:** internal/rules speaks resources/abilities/conditions/defenses generically; "hp"/"ac"/"bloodied"/"goblin" appear ONLY under `rulesets/`. semgrep scans internal/ + cmd/ as configured; `rulesets/` is content data (not scanned — correct).
- Contract additive only; Task 1 drift-caveat protocol (base gates pre-commit, full check post-commit).
- arch-lint: `rules: { in: internal/rules, mayDependOn: [engine, contract, rules] }`; gateway gains rules; NOTHING else may import rules. Bite-proof.
- AppendBatch (Task 3) is data-integrity-core work: opus-depth review, atomicity kill-injection mandatory.
- Property/soak/scenario/library pinned counts must remain byte-identical through Tasks 1–3 (no behavior change until wiring); Task 6 ADDS a library scenario (its own new pins).

---

### Task 1: Contract — rules vocabulary

**Files:** `contract/vtt/v1/events.proto` (+4 messages, oneof tags 18–21), `contract/vtt/v1/commands.proto` (UseAbility, RemoveCondition + ClientCommand oneof tags 17–18), toolgen manifest +2 (use_ability: "Use one of the loaded ruleset's abilities as an actor against explicit targets."; remove_condition: "Remove a named condition from an actor (DM-ended durations)."), golden update, fixtures (`ability_used_envelope.json` with rolls + string-int64s; `use_ability_command.json`), both round-trip suites, gen regenerated.

```proto
message AbilityUsed {
  string actor_id = 1;
  string ability_id = 2;
  repeated string target_ids = 3;
  message Roll {
    string expression = 1;
    repeated int32 results = 2;
    int32 total = 3;
  }
  repeated Roll rolls = 4;
  string outcome_summary = 5;
}
message ResourceChanged {
  string actor_id = 1;
  string resource = 2;
  int32 delta = 3;
  int32 new_value = 4;
  string reason = 5;
}
message ConditionApplied {
  string actor_id = 1;
  string condition_id = 2;
  string source = 3;
}
message ConditionRemoved {
  string actor_id = 1;
  string condition_id = 2;
  string reason = 3;
}
message UseAbility {
  string actor_id = 1;
  string ability_id = 2;
  repeated string target_ids = 3;
}
message RemoveCondition {
  string actor_id = 1;
  string condition_id = 2;
}
```
NOTE: AbilityUsed.rolls is `repeated` MESSAGE — toolgen's IsList path finally gets a real production field; the golden must show the array/items schema (the previously-unexercised branch becomes load-bearing — extend the toolgen tests accordingly). Evidence protocol: P3-Task-1 (drift red pre-commit expected).

### Task 2: Engine folds (generic)

**Files:** `internal/engine/{state.go,apply.go,apply_test.go}`.
State gains `Conditions map[string][]ActorCondition` (`ActorCondition{ID, Source string; AppliedSeq int64}`); Snapshot deep-copies it. Apply cases: ResourceChanged → resources map arithmetic on the actor's entry (create entry if resource unknown-but-actor-exists? NO — reject: resource must exist on the actor OR delta creates it? BINDING: the entry must already exist on the actor (statblocks declare resources at add_actor time); unknown → rejection, state unchanged); clamp current at 0 and at max when max > 0; `new_value` in the event must EQUAL the post-clamp computed value (validation: mismatch → rejection — the interpreter computes it, the engine verifies it; keeps the log's testimony honest). ConditionApplied → append if not already present (duplicate id+actor → rejection); ConditionRemoved → remove (absent → rejection). AbilityUsed → deliberate no-op (testimony; tested). All TDD behavioral-RED with the established rejection-leaves-state-unchanged pattern.

### Task 3: AppendBatch (DATA-INTEGRITY CORE — opus review)

**Files:** `internal/store/store.go` (+AppendBatch: one tx, contiguous seqs stamped, notify after commit in order, all-or-nothing; sequences reset on every failure path), `internal/campaign/campaign.go` (+AppendBatch: same lock as Append; session-stamp each; snapshot-fold the WHOLE batch (clone snapshot, apply sequentially — any failure rejects all); persist; live-apply all; notify all; poison on post-persist failure), tests incl.: atomicity kill-injection (throwaway: fail store mid-batch → ZERO events persisted, campaign usable), batch-then-undo (retract the batch's range — existing machinery), subscriber sees the batch contiguously, property/soak counts byte-identical (no callers yet).

### Task 4: internal/rules — format, loader, expressions

**Files:** `internal/rules/{schema/*.json (embedded),format.go,load.go,expr.go}` + tests.
Expression grammar (CLOSED, verbatim in expr.go's doc):
```
expr    := term (('+'|'-') term)*
term    := factor (('*'|'/') factor)*
factor  := INT | DICE | ref | func | '(' expr ')'
DICE    := INT 'd' INT
ref     := '@'IDENT (attribute) | '#'IDENT (resource current)
func    := ('floor'|'max'|'min'|'half') '(' expr (',' expr)* ')'
```
Parse at LOAD for every expression; unknown @/# idents vs the manifest's declared names → load error naming file+field. Eval takes (attrs, resources, Roller); division floors; half(x)=floor(x/2). Exhaustive table tests + `go test -fuzz` pass (bounded corpus run, findings triaged). Load: strict JSON decoding + schema validation (dep decision per Tech Stack — if hand-rolled, the format schemas still ship as JSON Schema DOCUMENTS for external authors, validated in tests against the fixtures). Cross-ref validation per spec §5.

### Task 5: Resolve + toy ruleset + conformance suite

**Files:** `internal/rules/{resolve.go,resolve_test.go}`, `internal/rules/conformance/conformance.go` (+_test), `rulesets/tavern-brawl/*`.
Resolve per spec §5 (pure given Roller): validations (actor/targets/stats/usage/range-Chebyshev) → clean errors; attack roll per target vs defense value from target's open maps; hit/miss/effect outcome lists → ordered events; usage spend as leading ResourceChanged; thresholds evaluated post-changes (apply_condition when expr true and not present; remove_when_false); returns []*Envelope payload protos (campaign stamps the rest). Golden scenarios fixed-seed: exact expected batch per ability (hit, miss, threshold-crossing, usage-exhausted rejection, out-of-range rejection). Toy ruleset per spec §8 exercising EVERY format feature. Conformance suite: runs any dir (schema+crossref+every-ability-resolves+goldens present); wired into task check for rulesets/*.

### Task 6: Wiring — serve/gateway/harness/MCP

**Files:** `cmd/vtt/serve.go` (+--ruleset flag → rules.Load → gateway), `internal/gateway/{authz.go,server.go,convert.go}` (+use_ability/remove_condition rows: dm/agent any; player only self-controlled actor — ownership on ACTOR id; handler: Resolve→AppendBatch, result carries first seq; RemoveCondition → single Append via existing path), `internal/harness/{scenario.go,engine.go}` (+`useAbility` step type — command steps already generic via ClientCommand? CHECK: scenario command steps wrap the ClientCommand oneof — the new commands work through the EXISTING generic step; only the denial/broadcast multi-event expectations need batch-awareness: ok-steps may now produce MULTIPLE events — engine matcher extended: expect.events_min or match-first-sequence semantics; BINDING: ok-step for use_ability asserts result ok + ALL batch events observed (first seq from result, contiguous run)), new library scenario `scenarios/toy-brawl.json` (serve with --ruleset in the runner for this scenario — library runner gains per-scenario ruleset config… BINDING: scenario file gains optional top-level `"ruleset": "tavern-brawl"`; self-contained boot loads it), MCP `get_ruleset_guide` tool + e2e extension (tool list 12; use_ability through MCP against toy ruleset; guide served; no-ruleset → clean error). authz table test grows to the new command count × 4 roles literal cells.

### Task 7: Workflow-level final review (Patrik's standing preference) + fix wave → merge gate.
Merge-gate bundle: umbrella spec "rule module"→"ruleset" terminology amendment; harness spec gains the useAbility/batch-aware step + ruleset field; gateway spec authz table +2 rows; MCP spec tool count.

## After this plan
5b: `rulesets/dnd45e-minimal` as pure data + module-level goldens + the demo rematch (Claude runs a REAL fight). Adventure format: future, with the world layer.
