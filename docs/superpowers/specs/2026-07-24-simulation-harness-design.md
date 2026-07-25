# Simulation Harness — Design Spec (sub-project 4)

**Date:** 2026-07-24
**Status:** Approved design (brainstorming output)
**Parent:** Platform spec §4.2/§5 item 4; pillars P1 (the harness is its proof);
ADR-009 (binding); gateway exit scenario (internal/gateway/scenario_test.go)
as the template.

## 1. Purpose

Scripted participants playing full sessions through the REAL WebSocket API,
as a CLI. The harness is: (a) the platform's standing dress rehearsal —
whole campaigns run headlessly before the table ever does; (b) the permanent,
machine-checked proof of P1; (c) the debug toolkit (`events tail`,
`state dump`); (d) the blueprint the future TS client and the LLM DM's
practice loop both follow.

## 2. Decisions (locked in brainstorming)

1. **Scenarios are declarative JSON files** — participants, ordered steps
   with expectations, final-state probes. LLM-authorable by construction
   (commands are protojson `ClientCommand` shapes). No YAML.
2. **Four modes ship now** (one wire-client core): `vtt client run`,
   `vtt client soak`, `vtt events tail`, `vtt state dump`.

## 3. Package architecture — P1 made mechanical

- `internal/harness`: wire-only client core (dial, protojson frames,
  request_id↔result correlation, event-stream consumption, reconnect);
  scenario engine (load/validate/execute); local folder reusing
  `engine.Apply` over received envelopes for client-side derived state.
- **arch-lint (the point):** `harness → {contract, engine}` ONLY.
  `store`, `campaign`, `gateway`, `identity` are forbidden imports — CI
  proves forever that the harness can act only through the wire.
  (Reusing `engine.Apply` is not a leak: the fold is the published
  derivation algorithm the TS client will reimplement; it consumes only
  contract types.)
- `cmd/vtt`: thin subcommands. Self-contained boot glue (composeServer +
  invite minting) lives in cmd — the harness core receives only ws URLs and
  tokens, never server objects, even when the server is in-process.

## 4. Scenario format (v1 vocabulary — deliberately small)

```json
{
  "name": "three-role-exit",
  "participants": [
    {"name": "dm", "role": "dm"},
    {"name": "lera", "role": "player", "controls": ["act-lera"]},
    {"name": "gm-bot", "role": "agent"},
    {"name": "watcher", "role": "spectator"}
  ],
  "steps": [
    {"by": "dm", "command": {"startSession": {"name": "s1"}}, "expect": {"ok": true}},
    {"by": "lera", "command": {"moveToken": {"tokenId": "tok-ursus", "to": {"x": 1, "y": 1}}},
     "expect": {"deniedContaining": "not authorized"}},
    {"by": "lera", "reconnect": {"afterSequence": 0}}
  ],
  "probes": [
    {"tokenAt": {"tokenId": "tok-lera", "x": 5, "y": 8}},
    {"sessionCount": {"open": 0, "total": 1}},
    {"actorExists": {"actorId": "act-ursus"}}
  ]
}
```

- `command` bodies are protojson of the contract's `ClientCommand` oneof —
  parsed with the contract types, no bespoke command grammar. ONE
  pre-dispatch substitution feature exists (added in build, documented here):
  `{{id:<name>}}` resolves to that participant's server-assigned id
  (engine-side, opaque string substitution — needed because `controller_id`
  requires real ids the author cannot know). Self-contained runs supply ids
  automatically; live mode reads them from `tokens.json`'s additive optional
  `"ids": {"<name>": "<participant-id>"}` map (`vtt invite` prints the id).
- `expect`: `{"ok": true}` or `{"deniedContaining": "<substring>"}` (denied
  steps also assert NO event broadcast reaches any participant).
- `reconnect` step: drop and re-dial that participant with `after=<seq>`;
  the runner asserts catch-up equality against the events it saw live
  (the exit scenario's signature check).
- `probes` run against the CLIENT-SIDE folded state at scenario end:
  `tokenAt`, `sessionCount`, `actorExists` — v1 complete list; growth is a
  spec amendment.
- Validation errors name the step index and field; unknown fields are
  errors (strict decoding).

## 5. Modes

- **`vtt client run <scenario.json>`** — default SELF-CONTAINED: boot temp
  server via composeServer, mint invites per participant, run, teardown;
  `--server ws://…` + `--tokens tokens.json` runs the same scenario against
  a live server. Output: human step log; `--json` machine report (per-step
  and per-probe pass/fail); exit code 0/1 for CI.
- **`vtt client soak --seed S --events M`** — the wire-level keystone:
  seeded `math/rand` generator issues only AUTHORIZED commands from
  role-appropriate participants (mix modeled on the campaign property
  test's); at checkpoints and at end, the incremental client-side fold must
  DEEP-EQUAL a fresh catch-up fold on a second connection. Same seed →
  identical command sequence (deterministic; pinned by test).
- **`vtt events tail --server --token [--after N]`** — stream envelopes as
  protojson lines until interrupted.
- **`vtt state dump --server --token`** — catch-up from 0, fold, print
  state as JSON with a top-level `headSequence` (highest folded sequence;
  the caller's staleness check — the dump is a point-in-time snapshot),
  exit. This is the contract sub-project 6's LLM tools inherit.
- **Fresh-campaign requirement (documented post-build, fail-loud):** `run`
  and `soak` require a campaign with no pre-existing events; catch-up
  backlog aborts with a clear error. Relative-sequence scenario references
  (running against live history) are a planned format-v2 extension.

## 6. Committed scenario library

`scenarios/` at repo root: v1 ships (1) `three-role-exit.json` — the
gateway exit scenario ported to data (all denials, retraction, reconnect
equality); (2) `denials.json` — every authz-table deny cell exercised once;
(3) `smoke.json` — minimal session for quick checks. A Go test in
`cmd/vtt` (NOT internal/harness — the runner needs the composeServer boot
glue that the P1 rule forbids harness from importing; corrected at build)
executes every library scenario self-contained — the library runs inside
`task check` on every commit.

## 7. ADR-009 posture

Runner built behavioral-RED (fixture scenarios with known outcomes against
a stubbed wire); scenario-library tests are after-the-fact → injection
proofs (break a gateway behavior in a throwaway copy, watch the
corresponding library scenario fail); soak determinism pinned; post-merge
mutation audit extends to `internal/harness` ONCE the suite's fixed-sleep
cost is reduced (fake-clock carry-forward — full mutation runs are
impractical at ~70s/run; the branch's seven injection proofs stand as the
interim teeth evidence; the `audit:mutation` target already lists harness
with this caveat).

## 8. Exit criteria

- `three-role-exit.json` green via self-contained run AND against a live
  `vtt serve` process (two different invocations, same scenario file).
- Soak: `--seed 1 --events 500` green including fold-equality checkpoints.
- `events tail` and `state dump` verified against a live server in the e2e.
- arch-lint proves harness wire-only; all gates green; property counts
  untouched.

## 9. Non-goals (YAGNI)

No LLM calls (sub-project 6 consumes scenarios/soak). No TS. No latency/
packet-loss simulation. No chaos beyond the `reconnect` step. No
multi-campaign. No probe-vocabulary generality (three probes, listed).
No parallel scenario execution.

## 10. Open questions (deferred, with owners)

- Whether soak's generator should include AUTHORIZED-BUT-REJECTED attempts
  (e.g. valid-role commands that fail campaign validation) as a deliberate
  mix component — decided during implementation, documented in the plan.
- tokens.json format for live-mode runs — decided at plan time (trivial:
  name→token map).

## 11. Amendment (2026-07-25, sub-project 5a merge)

Scenario format gains:

- **`"ruleset": "<id>"`** — optional top-level field. Self-contained
  (`run`/`soak` without `--server`) boots and loads `rulesets/<id>`. Live
  mode does NOT consult it — the operator starts the server itself with
  `vtt serve --ruleset <dir>`.
- **`use_ability` ok-steps are batch-aware:** `{"expect": {"ok": true}}`
  against a `useAbility` command asserts the result is ok AND that ALL of
  the ability's batch events are observed as a contiguous run starting at
  the result's first sequence (5a's `AppendBatch` contract).
- **Two new probe kinds:** `resourceAt` and `hasCondition` — growth of the
  v1 probe list per §4's "growth is a spec amendment" clause.
