# A scenario invites a participant as the server does, names a person, and makes a player only by a grant — report

Ticket: `docs/superpowers/specs/2026-11-22-a-scenario-participant-becomes-a-player-only-by-a-grant-design.md`.
Plan: `docs/superpowers/plans/2026-11-22-a-scenario-participant-becomes-a-player-only-by-a-grant.md`.
Branch `fix/a-scenario-participant-becomes-a-player-only-by-a-grant` from main
`1c3887c`. Commits: `2fb3778` (Task 0: ticket, plan, register), `285b79e`
(Task 1, the library and the soak roster), `adf65b5` (Task 2, denials and
adventure-night), `b271809` (Task 3, the format and the mint; the last code
commit) and `4bd6b5f` (the comment ledger). Every code reference below is to
`b271809`.

## Done looks like

1. [x] **A scenario declaring a player is refused when loaded.**
   `parseScenario` (`internal/harness/scenario.go`) returns, before any step
   is decoded, `scenario participant "<name>": a participant becomes a
   player when given a character; declare them a spectator` for a
   participant whose role is `player`.
   - `internal/harness/scenario_test.go`'s
     `TestLoadScenarioRefusesAParticipantDeclaredPlayer` and
     `TestLoadScenarioLoadsDMAgentAndSpectatorParticipants`.
   - Independently: `internal/harness/qa_scenario_role_test.go`'s
     `TestQAScenarioRolePlayerRefusedAnywhere`,
     `TestQAScenarioRoleTwoPlayersRefused`,
     `TestQAScenarioRoleDMAgentSpectatorLoadAsWritten` and
     `TestQAScenarioRolePlayerNamedOthersAreNotRefused`.
   - On `adf65b5` the refusal test failed: `LoadScenario` accepted the
     participant.
2. [x] **The harness mints what the scenario declares.** `mintInvites`
   (`cmd/vtt/harness_boot.go`) hands `identity.CreateInvite` the role
   `identity.Role(p.Role)`; the rewrite of `player` to `spectator` is gone.
   - `cmd/vtt/harness_boot_test.go`'s
     `TestMintInvitesRefusesAParticipantDeclaredPlayer` and
     `TestMintInvitesMintsEachParticipantAtTheRoleItDeclares`, which reads
     each minted role back through `identity`'s `Verify`.
   - Independently: `cmd/vtt/qa_mint_role_test.go`'s
     `TestQAMintEachRoleAtExactlyThatRole`,
     `TestQAMintPlayerRefusedWithCreateInvitesText`,
     `TestQAMintPlayerFirstMintsNothingAsSpectator` and
     `TestQAMintUnknownRoleIsNotRewritten`.
   - On `adf65b5` the refusal test failed: `mintInvites` returned no error.
3. [x] **Every scenario and the soak roster declare a role the server
   mints.** `git grep -n -E '"role": *"player"' -- scenarios` prints
   nothing; `SoakParticipants` (`internal/harness/soak.go`) declares Anna and
   Lisa `spectator`. Every scenario participant formerly declared
   `player` is declared `spectator`, names other than `player` and
   `spectator` unchanged.
   - `TestScenarioLibraryRunsSelfContained`,
     `TestEveryScenarioFileIsActuallyRun` and
     `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts` in
     `cmd/vtt`, the soak's pinned counts unchanged; `internal/harness`'s soak
     tests. Since Task 3, a file declaring `player` cannot load (item 1).
4. [x] **No participant is named for a role a grant changes.** `git grep -n
   -E '"name": *"(player|spectator)"' -- scenarios` prints nothing. The
   participant named `player` is `Anna` and the one named `spectator` is
   `Kalle`, in `name`, `by` and `{{id:...}}`; the soak roster's `player1`
   and `player2` are `Anna` and `Lisa` (the owner's rulings, 2026-11-22).
   - `internal/harness/scenario_test.go`'s
     `TestNoScenarioOrSoakParticipantIsNamedForARole`, which refuses a name
     that, ignoring case and trailing digits, is `player` or `spectator`;
     red on `2fb3778` for each such participant. Independently:
     `internal/harness/qa_scenario_names_test.go`'s
     `TestQAScenarioNamesAreNotRoles` and `TestQASoakNamesAreNotRoles`.
   - The goldens of session-zero, story-table and three-role-exit carry
     `p-Anna` where they carried `p-player`;
     `TestScenarioGoldenStreamsHaveNotDrifted` and `TestFoldGoldenCorpus`
     pass, and `client/test/fold-parity.test.ts`.
5. [x] **The two scenarios test a player.** `scenarios/denials.json` adds a
   party member, `act-lera` (`Lera`), and grants it to Anna after the DM
   places `tok-npc`; `scenarios/adventure-night.json` grants Anna
   `act-fighter` straight after the DM loads the adventure. Each of Anna's
   refused steps expects the player's text, `not authorized: role "player"
   may not issue "<command>"`, and her move of `tok-npc` `not authorized:
   token "tok-npc" is not controlled by participant`; each of Kalle's
   expects the spectator's (OD-1 A). The goldens hold the grants.
   - `TestScenarioLibraryRunsSelfContained`: with the pins in place and the
     grants not yet added, each of Anna's refused steps failed, receiving
     the spectator's text; with the grants, every step passes.
   - `TestScenarioGoldenStreamsHaveNotDrifted` goes red when a grant is
     dropped or given to another participant; `TestFoldGoldenCorpus` holds
     the hand-derived states.
6. [x] **The gate.** `task check` is green on `4bd6b5f`, the ledger commit
   after the last code commit. Its Go mutation gate measures neither
   `internal/harness` nor `cmd/vtt` (`PACKAGES` in `tools/check-mutation.py`),
   so it says nothing about this change; the mutants it counts as killed
   without measuring, because they timed out, are in `internal/perceive`,
   `internal/sight` and `internal/mcp`. It judges no client mutation report:
   the stored one measured `b90fc1a` and is out of date (SPEC-023). `task
   e2e` was not run, and no `cmd/vtt/webdist` rebuild was needed: nothing
   under `client/src` changed (D16).

## What the rules became

- **A scenario declares each participant with a role the server mints, and
  one declaring `player` is refused when loaded.** VTT-941, for the load;
  the first half, for any other role, is held at the mint through VTT-942
  (OD-2 B).
- **The harness mints each participant the role the scenario declares,
  unchanged.** VTT-942.
- **A scenario names no participant for a role a grant or a revocation
  changes.** VTT-943, over the scenario files and `SoakParticipants`.
- **A scenario step standing for a player's refusal is sent by a participant
  holding an actor.** VTT-944, for `denials.json` and `adventure-night.json`,
  with the pinned player's text.

Refused by the sort: a row that `SoakParticipants` declares no `player`
(held by VTT-942 through the soak's boot); a row that every file declares
only minted roles (VTT-941 and VTT-942 over the library run); a row naming
Anna, Kalle and Lisa (data); a row per scenario of VTT-944; a check over
expectation strings refusing a player's step pinned to `not authorized`
alone (a second source; a loosened pin is the reading review's); a row that
the goldens hold the new ids (the drift and fold gates); a row that Kalle's
refusals pin the spectator's text (the steps are the observation, VTT-942
what they hold end to end).

`TestFoldGoldenCorpus` was cited for VTT-944 during Task 2 and withdrawn
before the commit: it reads the goldens alone, so it cannot fail under the
row as worded.

## Rule 9

MapTool's `Player.Role` (`maptool/src/main/java/net/rptools/maptool/model/player/Player.java`)
is `PLAYER` or `GM`, chosen when a client connects, with token ownership held
apart. Here the role is derived from control (SPEC-009); its model does not
fit, and it has no scripted run of several connections to compare with.
Nothing borrowed.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| D13 named the comments the renames make false | `soak.go`'s package comment ("the platform's fixed four-role roster (dm, two players, agent)") and `planAddActor`'s doc ("to player1 then player2") were rewritten too | Both became false with the roster; `planAddActor`'s block was over the bound, so it was shrunk whole to a warning and a pointer (`validateAddActor`, `m.pendingGrant`), dropping its paragraph on how add_actor came to refuse a controller |
| D13: `client_e2e_test.go`'s block naming `{{id:player}}` "is not this work's" | `TestClientRunSelfContainedRunsCommittedThreeRoleExitScenario`'s doc rewritten as a pointer | The rename made a second part of it false, and P3 corrects a comment in the commit that makes it false |
| D5 changed one soak fake, `qaWSSoak.eventFor` | `qaLSSoak.handle` and `qaSLSGateway.append` stamp `player` on an accepted command from a participant who controls an actor | With the roster declared `spectator` they stamped `spectator` on accepted commands, which the gateway never writes; no assertion reads the stamp, before or after |
| M8: the touched harness test files have a ceiling of nothing | The ledger lowered `scenario_test.go`, `harness_boot_test.go`, `client_e2e_test.go`, `scenario.go` and `soak.go` besides `library_test.go` | `scenario_test.go`'s ceiling was not nothing, and it and `harness_boot_test.go` fell with test code added beside citation lines alone; `client_e2e_test.go`, `scenario.go` and `soak.go` fell with the rewritten and shrunk blocks |
| D13 listed the comments the change makes false and left `LoadScenario`'s doc out | `LoadScenario`'s doc shrunk to a pointer naming VTT-941 | It listed what load checks as "structural only" and left out the role refusal; its block was over the bound |
| M6: the re-recorded goldens differ by the grants and sequences | `adventure-night/stream.json`'s recorded diff is mostly order | The committed file listed `sceneCreated`'s tiles row by row and a fresh capture sorts them; the drift gate re-marshals with sorted keys and cannot see it. Order aside: the grant, `abilityUsed`'s and `resourceChanged`'s sequences shifted past it, and fresh masked dice with `newValue` unchanged. Kept as recorded (ADR-009) |

## The deliberate breaks

Each was applied, seen red, and undone by the inverse edit or from a saved
copy compared byte for byte.

- **VTT-943** (Task 1): a goblin-fight participant named `Player`, and
  `soakPlayer2` back to `player2`, each turned
  `TestNoScenarioOrSoakParticipantIsNamedForARole` red. `p-player` back in
  session-zero's `state.json` turned `TestFoldGoldenCorpus` and fold parity
  red, and in its `stream.json` `TestScenarioGoldenStreamsHaveNotDrifted`;
  `{{id:player}}` left in `a-player-switches-and-reconnects.json` turned
  `TestScenarioLibraryRunsSelfContained` red; `qaWSSoak.eventFor` judging by
  the stored role turned its soak tests red.
- **VTT-944** (Task 2): `denials.json`'s grant dropped, moved after Anna's
  first refused step, or given to Kalle, and `adventure-night.json`'s grant
  dropped, each turned Anna's steps in `TestScenarioLibraryRunsSelfContained`
  red; `act-fighter`'s controller fields left out of the hand-derived state
  turned `TestFoldGoldenCorpus` and fold parity red; `denials`' old
  `stream.json` turned the drift gate red.
- **VTT-941** (Task 3): the check dropped turned the load refusal tests red;
  `spectator` refused as well turned the loading test and VTT-943's walker
  red; the participant's name left out of the error turned the name
  assertion red.
- **VTT-942** (Task 3): the rewrite restored turned the mint refusal tests
  red; `agent` rewritten to `dm` turned the stored-role test red.

## Phase 4a adjudications

- **Task 1** (VTT-943): no findings. Kept `TestQAScenarioNamesAreNotRoles`
  and `TestQASoakNamesAreNotRoles`. Dropped a test of the names Anna, Kalle
  and Lisa (data; the sort refused a row for them), a test of the declared
  roles (VTT-941's, held by the library run), a test that every scenario
  loads and its placeholders resolve (held by
  `TestScenarioLibraryRunsSelfContained` and
  `TestEveryScenarioFileIsActuallyRun`) and a test running injected faults
  (a permanent test of the test).
- **Task 2** (VTT-944): no findings. Dropped
  `TestQAScenarioPlayerRefusalsFollowTheGrantAndSayPlayer` (the check over
  expectation strings the sort refused), `TestQAScenarioPlayerCommittedRunsGreen`
  (the library run's own path), a fault-injection test (a test of the
  scenarios, which are tests) and a golden check (beyond the row's words;
  held by the fold and drift gates).
- **Task 3** (VTT-941, VTT-942): no findings. Kept the QA tests named
  under items 1 and 2; dropped a probe that logged and asserted nothing.
  It observed that load accepts `Player`, `PLAYER` and ` player`: the row
  says the role `player`, and such a role fails at the mint as an unknown
  one (`identity: unknown role "Player"`, naming the participant), as
  Task 3's Phase 4b review confirmed by probe. Near-theoretical, small consequence:
  no code.

## The plan's gaps

- **G1.** `scenarios/goldens/<name>/projections/` does not exist; of the
  client tests the ticket names, only `fold-parity.test.ts` reads the
  goldens. Nothing to change there.
- **G2.** The files the ticket did not name changed as the plan said:
  `soak_test.go`, `soak_log_seat_test.go`, `qa_world_seat_test.go` and
  `state_dump_switch_test.go`, plus the comment files above.
- **G3.** The client's stored mutation report measured `b90fc1a` and was out
  of date before this ticket; the goldens are among its hashed inputs
  (SPEC-023). A full run is the owner's to ask for.
- **G4.** Participants not named for a role kept their names; their role
  became `spectator` (D3).
- **G5.** The ticket's open points: a player's refusal texts (M2, pinned in
  Task 2); the adventure's dice (the drift gate masks them and the state
  reads none); `characterSeats`' `player` arm stays, reached by tests that
  build a participant in code (D4); the soak's names are read only in
  `internal/harness`.

## What could not be established

Nothing was left open. The ticket's open points are answered under G5. What
remains are decisions rather than questions: a grant moved after Anna's first
refused step writes no event, so the library run sees it and the drift gate
does not; and a pin loosened back to `not authorized` still matches the
player's text, so no test sees it — the plan's sort left that to the reading
review. The soak fakes' `ActorRole` stamp is read by no assertion, so a fake
that stamps it wrongly fails nothing; it models the gateway and decides no
outcome.

## What was deliberately left out

- Go tests that build a participant in code and name it `player`
  (`internal/harness/engine_test.go` and others, `internal/gateway/scenario_test.go`),
  and `characterSeats`' `player` arm: the ticket's non-goal and D4.
- Renaming a scenario file: a file's name says what it tests.
- Refusing at load every role the server does not mint: OD-2 B.
