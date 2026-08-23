# Actor Kind Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An actor carries its own kind, and the visibility exception "always known" keys on that kind rather than on whether anyone happens to control it.

**Architecture:** One additive contract field on `Actor`, folded into `engine.State` in both languages, read by the projection at its two existing call sites. No new dependency: kind is already in state by the time the projection asks.

**Tech Stack:** protobuf (vtt.v1), Go (`internal/engine`, `internal/gateway`), TypeScript (`client/src/fold.ts`).

## Global Constraints

- Governing design: **spec §5.1** in `docs/superpowers/specs/2026-08-18-visibility-design.md` (amended 2026-08-23). Read it before Task 1; it carries the ruling, the two cases that decide it, and the migration rule.
- ADR-007: contract evolution is **additive only**, and `check:breaking` enforces. Its green IS evidence here: `Actor` predates `feat/visibility` — it exists at the merge-base `c40a450` with fields 1-8 unchanged — so `buf breaking` has a real baseline in `main` to diff field 9 against. (An earlier draft of this plan said the opposite. `check:breaking` is only blind to messages *introduced on the branch*, which `SceneSeen` and `TokenHidden` are and `Actor` is not.)
- ADR-009 airtight TDD: tests first, behavioural RED, fault-injection proof per load-bearing assertion.
- CLAUDE.md rule 4: `engine.Apply` stays the only writer of game state.
- CLAUDE.md rule 5: no game-system vocabulary. The semgrep ban list is system mechanics (`healing_surge`, `bloodied`, `saving_throw`, `hit_points`, `fortitude`); PC/NPC is a table role and passes. Verify against `.semgrep/vocabulary.yml` rather than trusting this line.
- Go's `engine.Apply` and `client/src/fold.ts` are **strict mirrors**.
- `task check` is the single gateway. Never weaken a gate to pass it.

---

## Task 1: Actor carries its kind, and the roster keys on it

**Files:**
- Modify: `contract/vtt/v1/events.proto`, `internal/engine/apply.go`, `internal/engine/state.go`, `client/src/fold.ts`, `client/src/state.ts`, `internal/gateway/project.go`, `internal/gateway/viewpoint.go`
- Test: `internal/gateway/project_test.go`, `internal/gateway/viewpoint_internal_test.go`, `internal/engine/*_test.go`, `client/test/`

**Interfaces:**
- Produces: `Actor.kind` on the wire; `engine.Actor.Kind` in Go; `Actor.Kind` in TS state.
- Consumes: nothing new. `look()` and `MayPerch` already hold the `engine.State` they need.

- [ ] **Step 1: Write the failing test — the leak, stated as a test**

**READ THIS BEFORE THE CODE BELOW.** The snippets are a SKETCH of the two
behaviours, not transcription. Of the helpers they read as if they call, only
`twoRooms` exists — as `func twoRooms() *engine.State` (`project_test.go:103`),
**taking no `*testing.T`**. `grantControl`, `projectAll`, `playerViewer` and
`actorIDsIn` do not exist anywhere in the repo and you are writing them. The
real building blocks to compose them from:

- grant control: `mustApply(st, seq, &vttv1.ActorControlGranted{...})`, the
  pattern existing tests already use.
- a viewer: `gateway.Viewer{ParticipantID: "...", Role: identity.RolePlayer}`
  constructed inline. There IS a `player()` helper at `project_test.go:119`,
  but it is hard-wired to `"p-1"` and takes no argument, so it will not serve
  a test that needs a named participant.
- projecting a whole state: the nearest existing thing is
  `projectWholeLog(t, g keystoneGolden, v gateway.Viewer)`
  (`keystone_test.go:1056`), which takes a golden fixture rather than a plain
  `*engine.State` — so it is a model, not a match.

The load-bearing behaviour. A monster behind a closed door, granted to a DM or
agent participant, must NOT reach a player's roster:

```go
func TestAnNPCHeldByTheDMIsNotPublishedToThePartysRoster(t *testing.T) {
	// The whole-branch review's finding I1, as a fixture the keystone
	// structurally cannot provide: its oracle transcribes the same predicate
	// (keystone_test.go:241, :258, :269), so both sides of the equation agree
	// while both are wrong.
	st := twoRooms() // hero at (1,1); goblin archer at (5,1) behind a SHUT door
	// ... grant the goblin to a DM participant via mustApply/ActorControlGranted
	// ... project the whole state for a player viewer
	// EXPECT: the goblin archer is absent from the roster.
}
```

And its control, which must keep passing — the exception itself:

```go
func TestAPartyMemberIsKnownEvenWhenHeldByTheDM(t *testing.T) {
	// A player's character run by the DM while its player is offline is STILL
	// a party member (spec §5.1). A rule keyed on the CONTROLLER's role drops
	// them from every roster; this is the test that would catch that, and it
	// is why kind belongs to the actor rather than to whoever holds it.
	// EXPECT: the absent player's character IS on the roster.
}
```

Write both against real helpers before Step 2, or Step 2 measures a compile
error rather than the behavioural RED ADR-009 requires.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/gateway/ -run TestAnNPCHeldByTheDM -v`
Expected: FAIL — the goblin IS in the roster today.
**Verify the RED by seeing the test NAME in the output**, not by a non-zero exit: a `-run` filter that matches nothing prints `ok` and is indistinguishable from a pass. This trap is in this repo's ledger.

- [ ] **Step 3: Add the contract field**

`Actor` gains a kind. Choose the next free field number by reading the message, not by assuming. An enum is preferable to a bool — `PARTY_MEMBER` vs the rest — because the third case (a neutral, a familiar, a summon) is foreseeable and a bool would have to be renamed to admit it. Default value must be the UNSPECIFIED zero so absence is expressible and the migration rule below can see it.

Regenerate with `task generate:contract` (generated code is committed) and confirm `check:drift` stays green.

- [ ] **Step 4: Fold it, in both languages, with the migration rule**

`internal/engine/apply.go` and `client/src/fold.ts` gain the field in their `ActorAdded` arms, as strict mirrors. Spec §5.1's migration rule is the load-bearing part:

> absent + has a controller → party member; absent + no controller → not.

Write a test for an `ActorAdded` with no kind and a controller (must be treated as a party member — this is history already written) and one with no kind and no controller (must not be).

- [ ] **Step 5: Both call sites read kind**

Three readers of the same predicate, and they are in two files — check each at its real location rather than assuming they sit together:

- the roster loop, `internal/gateway/project.go:351-359`, currently `if len(a.GetControllerIds()) > 0`
- `eyes()`, **also `internal/gateway/project.go`** (:365-392, player check at :372, spectator check at :386) — NOT in `viewpoint.go`
- `MayPerch`, `internal/gateway/viewpoint.go:62`

**One rule, all three**: fixing only the roster leaves an agent-held monster perchable, which is the half `viewpoint.go:22-31` already flags for adjudication. Note that comment mentions the roster only as PRECEDENT for leaving perch alone — it does not flag the roster itself as a defect, so update it to say what now governs both.

- [ ] **Step 6: Fix the keystone's oracle, which shares the bug**

`internal/gateway/keystone_test.go`'s `oracleEyes` transcribes the same predicate. It must key on kind too — otherwise the keystone keeps agreeing with a projection that has changed. This is the transcription risk the arc documented, and this is the one place it bit.

- [ ] **Step 7: Run the gates**

Run: `go test ./... && bun test client/test && task client:typecheck && task check:drift && task check:breaking`
Expected: PASS. Do NOT run `check:mutation` or `check:ts-mutation` without confirming disk headroom first — this repo has recorded that gate filling the disk and then reporting false numbers while exiting 0. If lines shift in a file with entries in `tools/mutation-equivalents.txt`, re-key headers AND body references, measured after the last edit and byte-compared.

- [ ] **Step 8: Commit**

```bash
git add contract internal/engine internal/gateway client/src client/test docs
git commit -m "An actor knows what it is, so control cannot promote a monster"
```

---

## Out of scope, deliberately

- **Testimony outliving sight** (review finding I2): a legitimately glimpsed NPC still streams its conditions and damage forever, because `pr.actors` never forgets. Separate rule, separate decision, recorded in spec §5.1's closing paragraph so this task is not mistaken for closing it.
- **Objects never fogged** (finding I3): `planFog` keys on terrain-`Explored` while objects draw unconditionally.
- **What a player may read off a token they can see** — the sheet-"areas" model, per-ruleset, with a DM override and a knowledge-check reveal. Patrik's framing, 2026-08-22; its own arc.
