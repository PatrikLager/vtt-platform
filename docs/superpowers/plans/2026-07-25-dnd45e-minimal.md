# dnd45e-minimal Implementation Plan (sub-project 5b)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver 5b per the approved spec (`docs/superpowers/specs/2026-07-25-dnd45e-minimal-design.md`): Patrik's 4.5e house system, demo-minimal slice, as pure data passing the untouched conformance suite, plus the goblin-fight wire scenario. Merge gate = the live demo rematch.

**Architecture:** content only — `rulesets/dnd45e-minimal/**` + `scenarios/goblin-fight.json`. ZERO platform changes: any need for a code change is a BLOCKED escalation, never a workaround.

**Tech Stack:** existing. Conformance suite (`internal/rules/conformance`, run via the `rulesets/` glob test) is the gate; harness scenario library runs the wire scenario inside `task check`.

## Global Constraints

- Branch `feat/dnd45e-minimal` from main. Adapted review-before-commit flow; reviewers READ-ONLY; injections in throwaway rsync copies only.
- Diff scope: `rulesets/dnd45e-minimal/**`, `scenarios/goblin-fight.json` ONLY (plus plan/spec docs already on main). Platform code, schemas, contract, Taskfile: FROZEN.
- Format v1 rules (frozen at 5a): IDENT charset for attribute/defense/resource names; ability/condition ids kebab-case; NO unary minus (`0 - x`); NO comparison operators (non-zero truthiness, `max(0, …)` idiom); NO dice in threshold `when`/`default_max_expr`; dice bounds 1..100 × 1..1000; int32 value contract; duplicate target ids rejected.
- ADR-009 in data form: goldens derived FROM the format contract by hand first, then verified by the conformance run — a golden adjusted to match observed output without a derivation note in the report is impl-then-test and forbidden.
- Conformance enforces one golden per ability (7 abilities → ≥7 goldens; spec lists 13).
- Existing pinned counts (property/soak/scenario library) byte-identical; goblin-fight.json adds its own new pins.

---

### Task 1: The ruleset — content + goldens

**Files:** Create `rulesets/dnd45e-minimal/{ruleset.json, abilities/*.json (7), conditions/*.json (3), guide.md, goldens/*.json (13)}`.

**Read first:** the 5b spec §3–§4 (exact expressions, verbatim); `rulesets/tavern-brawl/**` (format by example); `internal/rules/conformance/conformance.go` (fixture-statblock generation rules + tableRoller stepping + golden JSON format — the authoritative reference for deriving expected batches).

**Content (spec §4 verbatim — expressions are binding):** attributes `str, dex, con, max_hp`; defenses `ac, fort, ref, will`; resources `hp` (thresholds: bloodied `max(0, @max_hp - #hp * 2 + 1)` remove_when_false true; dying `max(0, 1 - #hp)` remove_when_false true) and `flurry_uses`. Seven abilities with the spec's exact rolls/deltas: goblin-scimitar, goblin-shortbow, longsword-strike, crossbow-shot, hunters-flurry (limited flurry_uses cost 1, max_targets 2, miss `0 - half(1d8 + @dex)`), staggering-blow (vs fort, + apply_condition dazed), rally (non-attack, effect `1d6 + @con`). Conditions bloodied/dying/dazed with 4.5e descriptions.

**guide.md** — reference statblocks (attribute/resource maps the DM copies into add_actor; document the max_hp-duplication rule and that only encounter-power users need flurry_uses):
- Goblin cutter: attrs {str 1, dex 3, con 1, max_hp 8}, defenses {ac 15, fort 12, ref 14, will 11}, resources {hp 8/8}. Abilities: goblin-scimitar.
- Goblin archer: attrs {str 0, dex 4, con 1, max_hp 6}, defenses {ac 14, fort 11, ref 15, will 11}, resources {hp 6/6}. Abilities: goblin-shortbow, goblin-scimitar.
- Human fighter: attrs {str 4, dex 2, con 3, max_hp 28}, defenses {ac 17, fort 15, ref 13, will 12}, resources {hp 28/28, flurry_uses 1/1}. Abilities: longsword-strike, crossbow-shot, hunters-flurry, staggering-blow, rally.
Plus: turn-flow suggestions, threshold behavior (automatic), manual condition removal (`remove_condition`), the demo runbook appendix (serve --ruleset, invite agent, .mcp.json with --ruleset arg, suggested opening prompt).

**Goldens (13, hand-derived):** one hit + one miss for each attack ability where distinct behavior exists; specifically: goblin-scimitar-hit, goblin-shortbow-hit, longsword-strike-hit, longsword-strike-miss, crossbow-shot-hit, hunters-flurry-two-targets (ordering pin), hunters-flurry-usage-exhausted (rejection), staggering-blow-dazed (condition outcome), rally-heals (positive change), bloodied-crossing (downward), bloodied-removed (rally upward crossing), dying-crossing (hp to 0), out-of-range rejection. Derive each expected batch BY HAND from the spec's expressions + conformance's fixture/roller rules; record the derivation arithmetic in the report.

- [ ] **Step 1 (RED):** author ruleset.json + abilities + conditions + guide.md; run `go test ./internal/rules/conformance/ -run TestConformanceOverRulesetsGlob -count=1` — MUST FAIL demanding per-ability goldens (the suite's enforcement is the RED).
- [ ] **Step 2:** hand-derive and write the 13 goldens; rerun — GREEN. Any derivation/runtime mismatch: STOP, investigate, document which side was wrong before touching the golden.
- [ ] **Step 3:** full `go test ./... -race -count=1` + `task check` green; `git diff --stat` scope check. **Commit point** — `feat: rulesets/dnd45e-minimal — the 4.5e house system as pure data`

### Task 2: The wire scenario

**Files:** Create `scenarios/goblin-fight.json`.

**Read first:** `scenarios/toy-brawl.json` (the pattern: ruleset field, statblock setup, use_ability ok-steps, probes, denial rows); Task 1's guide.md statblocks (the scenario's add_actor payloads MUST match the guide's reference statblocks exactly — the scenario is the guide's executable proof).

Scenario: top-level `"ruleset": "dnd45e-minimal"`; session + scene; fighter (player-controlled) + goblin cutter + goblin archer (DM) with guide-exact statblocks; tokens placed in range; longsword-strike hit path (crypto dice → assert invariants: result ok, batch events observed, resourceAt decreased? NOTE: crypto dice mean the exact hp value is unknowable — use the probe forms that exist; if resourceAt only supports exact values, probe a field that IS deterministic, e.g. hasCondition after staggering-blow is NOT deterministic either (hit vs miss)… BINDING: the scenario asserts what IS deterministic over crypto dice: command ok + batch contiguity (the harness ok-step), rally's positive change on a fresh full-hp fighter clamps at max (hp stays 28 — deterministic!), remove_condition denial/ok paths, and authz denials (player driving goblin cutter → denied; spectator use_ability → denied). Structure the fight beats as ok-steps without value probes where dice make values nondeterministic; document this reasoning in a scenario comment field if the format has one, else in the report.
- [ ] **Step 1 (RED):** write the scenario; the library glob test picks it up automatically — run `go test ./cmd/vtt/ -run TestScenarioLibrary -count=1` with a deliberately-wrong expectation first (e.g. rally hp 27) to prove the runner actually evaluates it; then correct to 28.
- [ ] **Step 2:** full `task check` green ×2 (scenario runs with crypto dice — two passes guard against dice-dependent flake in the assertions chosen). **Commit point** — `feat: goblin-fight scenario — the guide's statblocks, executable`

### Task 3: Workflow-level final review (Patrik's standing preference) + fix wave → DEMO GATE

Lenses scaled to a data-only diff: (1) golden-arithmetic verifier (re-derive every golden independently); (2) 4.5e-fidelity + guide honesty (guide vs ruleset vs scenario consistency; runbook correctness); (3) format-compliance + P4 (zero platform changes; conformance untouched; YAGNI). Adversarial verify panel per finding. Fix wave. Then STOP: present the demo runbook to Patrik — the live rematch is the merge gate. Merge bundle after acceptance: memory update; v2-grammar carry-forwards (max-reference, comparison operators) ledgered.
