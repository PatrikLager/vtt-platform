# Plan: a scenario invites a participant as the server does, names a person, and makes a player only by a grant

- **Ticket:** `docs/superpowers/specs/2026-11-22-a-scenario-participant-becomes-a-player-only-by-a-grant-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-22 by an agent that did not write the ticket, against
    `1c3887c` on `fix/a-scenario-participant-becomes-a-player-only-by-a-grant`
    in the worktree `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e` and Stryker were not run, nor the whole `./cmd/vtt` package.
    Every run that changed anything (M2 to M7) ran in a copy of the tree
    made by `git archive HEAD` outside the repository; the worktree's
    `git status` was the same before and after. `go clean -cache` ran at
    the end. The rest are searches and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-22-a-scenario-participant-becomes-a-player-only-by-a-grant.md`.
- **Base:** `1c3887c`. **Branch:** `fix/a-scenario-participant-becomes-a-player-only-by-a-grant`.
- **Owner's rulings taken in (2026-11-22):**
  - The changes are one ticket: participants named for a person, the
    scenario format refusing the role `player`, and the would-be player of
    `adventure-night.json` (and, found at investigation, of `denials.json`)
    granted an actor before it acts as a player.
  - In every scenario file the participant named `player` becomes `Anna`
    and the one named `spectator` becomes `Kalle`, spelled so; their
    `{{id:...}}` placeholders and golden ids follow (`p-Anna`, `p-Kalle`).
  - The soak roster's `player1` becomes `Anna` and `player2` becomes `Lisa`.
  - Standing: no counts or measured figures in anything written down; one
    source per fact, pointed at; a code comment is a warning or a pointer;
    the client's full mutation run on the owner's word; push and merge on
    the owner's word; `go clean -cache` before and after every gate.
- **Signed off:** by the owner, 2026-11-22: OD-1 = A (Kalle's refusals pinned
  too), OD-2 = B (`player` alone), and the plan "Godkänd".
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off: R1 =
  VTT-941, R2 = VTT-942, R3 = VTT-943, R4 = VTT-944.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol is where it says: `Participant`, `parseScenario` and
   `LoadScenario` in `internal/harness/scenario.go`; `CreateInvite` and its
   refusal text in `internal/identity/identity.go`, VTT-911 in the register
   with that text; `mintInvites` and its rewrite in `cmd/vtt/harness_boot.go`;
   `SoakParticipants`, `soakPlayer1` and `soakPlayer2` in
   `internal/harness/soak.go`; `characterSeats` in `internal/harness/engine.go`;
   `TestScenarioGoldenStreamsHaveNotDrifted`; the named tests and the report
   `docs/reports/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md`;
   `Player.Role` in MapTool's `Player.java`. **One path does not resolve:**
   `projections/` under `scenarios/goldens/<name>/`. Each golden directory
   holds `stream.json` and `state.json` and nothing else, and no test reads a
   projection from there (G1).
2. **"Done" is an observation. By command where it could be.**
   - Item 1's and item 2's tests are still to be written; M2 shows today's
     tree loads and mints the scenario files' `player` participants without
     complaint, so each will fail today.
   - Items 3 and 4: both `git grep` commands print lines on today's tree.
   - Item 4's `TestScenarioGoldenStreamsHaveNotDrifted` is green on the
     changed copy once the goldens are re-derived (M6).
   - Item 5: M2 shows the would-be player's refusals today carry
     `role "spectator" may not issue ...`, so a step pinned to the player's
     text fails on today's files (D8).
   - Item 6 is the gate.
3. **Each rule is breakable. By reading.** Each of the ticket's rules has an
   observation that fails (the "Red under" column below). The first rule's
   first half, "declares each participant with a role the server mints", is
   held at the mint and not at load: a role other than `player` that the
   server does not mint is refused by `CreateInvite` (VTT-911) once
   `mintInvites` stops rewriting (R2). OD-2 asks whether load should refuse
   it too.
4. **The scope matches the claim. By command, then reading.**
   - Readers of the soak's names, by `git grep player1|player2` over the
     whole tree: `soak.go`, and in `internal/harness` the tests
     `soak_test.go`, `soak_log_seat_test.go`, `qa_log_seat_test.go`,
     `qa_soak_log_test.go` and `qa_world_seat_test.go`; no Go file outside
     `internal/harness`, no TypeScript, no JSON, no golden, no specification
     and no register row; in `docs/`, old reports and plans, which are
     history and are not edited. The ticket names `qa_log_seat_test.go` and
     `qa_soak_log_test.go` alone (G2).
   - `qa_world_seat_test.go`'s soak fake judges "player" from the role the
     roster declares, and its soak tests turn red once the roster declares
     spectators (M4, D5). The ticket does not foresee it (G2).
   - Readers of the scenario participants' names: `cmd/vtt/state_dump_switch_test.go`
     reads `a-player-switches-and-reconnects.json`'s `player` token, which the
     ticket does not name (G2); the `{{id:...}}` placeholders inside the
     files; the golden ids `p-<name>`; comments in `cmd/vtt/library_test.go`
     (D13).
   - Of the client tests the ticket names, only `fold-parity.test.ts`
     reads `scenarios/goldens/`; the others name it in comments alone.
     No client test pins a participant id or name (G1, M7).
   - The goldens of `door-watch`, `shared-control`, `smoke` and `toy-brawl`
     do not change (M6).
   - The goldens are among the client mutation run's hashed inputs
     (`tools/ts-mutation-inputs.py`, SPEC-023), so the stored client report
     goes out of date with the first commit that changes one (G3).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-009's "`mintInvites` mints a scenario's player as a spectator" is
     overturned; the ticket names SPEC-009 and states the rewrite in its
     problem.
   - `docs/superpowers/plans/2026-11-20-a-spectator-never-holds-control.md`,
     Task 5, chose to translate a scenario's `player` in `mintInvites`, and its
     report left `adventure-night.json` out. A plan is not a decision record;
     the owner's ruling of 2026-11-22 overturns the choice, said here so the
     next reader finds both.
   - ADR-009 and `scenarios/goldens/README.md` ("Why there is no `-update`
     flag") bind how the goldens are re-derived (D9). VTT-380, VTT-911 and
     VTT-912 stand as worded.
6. **The records the work moves are named, and reachable. By command, then
   reading.** The section is present, its one path resolves, nothing is
   `New:`. A search of `docs/specifications/` for `scenario`, `soak`,
   `mintInvites`, `characterSeats` and `participants` finds SPEC-009 (the
   sentence above, which moves), SPEC-022's scenario-engine and soak
   paragraphs (they name no role and no participant, and `characterSeats`
   does not change: they stay), SPEC-019 (`mintInvites` opens identity
   alone: stays), SPEC-023 (the goldens are hashed: a consequence, no change)
   and SPEC-007 and SPEC-013 (cite scenario tests by name: stay). No
   specification describes the scenario format; the ticket's rules land in
   SPEC-009 (D15).

## The gaps this plan inherits

- **G1. Paths the ticket names that hold nothing to change.**
  `scenarios/goldens/<name>/projections/` does not exist. `client/test/fold-unit.test.ts`,
  `client/test/removal-batch-parity.test.ts` and `client/test/visibility.test.ts`
  read no golden. `internal/harness/qa_one_log_test.go` builds its
  participant in code, which the ticket's own non-goals exclude; it does not
  change (D4).
- **G2. Files the ticket does not name:**
  - `internal/harness/soak_test.go`, `internal/harness/soak_log_seat_test.go`
    and `internal/harness/qa_world_seat_test.go` read the soak's names;
    `qa_world_seat_test.go`'s fake also judges the role (D5).
  - `cmd/vtt/state_dump_switch_test.go` reads the scenario participant
    `player`'s token.
  - `cmd/vtt/harness_boot_test.go` gets R2's test; `cmd/vtt/scenario_goldens_test.go`
    and `cmd/vtt/library_test.go` get R4's citation lines;
    `docs/requirements.md` gets the rows.
- **G3. The client's stored mutation report** goes out of date with Task 1's
  golden commit (SPEC-023). `task check` names that and passes; a full run
  is the owner's to ask for.
- **G4. The non-goals and item 3 disagree as worded.** The non-goals exclude
  "a change to a participant whose name is not a role's (`fighter`,
  `brawler`, ...)", and item 3 requires that no scenario declare a
  `player`, which changes those participants' role. Read here as: their
  names do not change; their role becomes `spectator` (D3).
- **G5. The ticket's "What could not be established"** travels, each traced:
  the first point is M2 and D8, the second M5, the third M3 and D4, the
  fourth the scope search above and M4.

## Measurements this plan stands on

- **M1. The names.** Under `strings.EqualFold`, the rule `CreateInvite`
  and `JoinAdmits` hold a name to (SPEC-009), `Anna` and `Kalle` equal no
  other participant of any scenario that holds them, and `Anna` and `Lisa`
  equal neither `dm` nor `agent`. `Anna` and `shared-control.json`'s `ana`
  differ and never share a scenario. M6 minted every scenario and the soak
  under the new names.
- **M2. The refusal texts, by command.** A probe test, kept outside the
  tree, ran `denials.json` and `adventure-night.json` through
  `bootSelfContained` and `RunScenario` with every refused step's
  expectation set to a string no refusal holds, so each step reported the
  text it received.
  - Today, the participant named `player` is refused `load_map`,
    `add_actor`, `place_token`, `start_session`, `end_session`, `move_token`
    and `load_adventure` with `gateway: not authorized: role "spectator" may
    not issue "<command>"`: a spectator's refusal, as the ticket says.
  - With the participant renamed `Anna`, declared `spectator`, and granted
    an actor by an accepted step first (D6, D7): the same commands but the
    move are refused with `gateway: not authorized: role "player" may not
    issue "<command>"`, and the move of `tok-npc` with `gateway: not
    authorized: token "tok-npc" is not controlled by participant "<id>"`,
    whose id differs every run. Every one still contains `not authorized`.
  - The DM's kindless grant and seeded `add_actor` in `denials.json` are
    refused as today, and `Kalle`'s refusals carry `role "spectator"`.
- **M3. `characterSeats`' `player` arm is reached.** With the arm removed
  by `go test -overlay` in the copy, `internal/harness`'s tests turn red in
  `engine_test.go`, `late_leak_test.go`, `one_log_test.go`,
  `qa_log_seat_test.go` and `qa_one_log_test.go`, which build a `player`
  participant in code, and in the soak's test files, which read
  `SoakParticipants`. Once the roster declares spectators, the soak reaches
  the `spectator` arm, and the code-built tests still reach `player`.
- **M4. The whole change, applied in the copy:** every scenario's names and
  roles, the grants and the pinned texts, `mintInvites` passing the role
  unchanged, `parseScenario` refusing `player`, the soak roster, its readers
  renamed, and `state_dump_switch_test.go`'s token key.
  - `go test ./internal/harness/` was red only in
    `TestQAWSSoakPassesWhenNothingLeaks`,
    `TestQAWSSoakFailsWhenOnlyOneWorldSeatHearsADenial` and
    `TestQAWSSoakDoesNotReadCharacterSeats`: the fake behind them, in
    `qaWSSoak.eventFor`, holds a participant to the player rules only when
    the roster declares them `player`, so it stopped refusing the soak's
    deliberate denials. Judging every participant who is neither DM nor
    agent by those rules made the package green.
  - `go vet` on both packages was clean.
- **M5. The dice in `adventure-night`.** `TestScenarioGoldenStreamsHaveNotDrifted`
  masks `results`, `total`, `outcomeSummary`, `delta`, `newValue` and
  `outcome` on both sides (`maskDice`), so the rolls never decide it. The
  added grant is one `ActorControlGranted` straight after the adventure's
  load, and every later event's sequence moves past the grant's. The state reads no
  roll: `act-fighter` starts at full hit points (`adventures/goblin-ambush/actors/act-fighter.json`),
  the rally's `ResourceChanged` carries `newValue` at that maximum in the
  committed stream and in a fresh capture alike, and the fold checks
  `newValue` against what it computes (`engine.Apply`).
- **M6. The goldens, re-recorded in the copy** with a throwaway test calling
  `captureNormalizedStream`, then folded with `harness.Fold` and the fold
  gate's `dumpJSON`:
  - `door-watch`, `shared-control`, `smoke` and `toy-brawl`: stream and
    state unchanged (their renamed participants send no accepted command).
  - `session-zero`, `story-table`, `three-role-exit`: stream and state equal
    to the committed ones with `p-player` replaced by `p-Anna`, and nothing
    else.
  - `adventure-night`: the stream gains the grant; the state gains
    `controller_id` and `controller_ids` of `p-Anna` on `act-fighter`, and
    `headSequence` follows the added grant.
  - `denials`: the stream gains an `ActorAdded` of `act-lera` and its grant,
    both in `sess-1`; the state gains `"act-lera": {"actor_id": "act-lera",
    "name": "Lera", "controller_id": "p-Anna", "controller_ids": ["p-Anna"],
    "kind": 1}`, and `headSequence` becomes the grant's sequence.
  - With those goldens in place: `go test ./internal/harness/` green
    (`TestFoldGoldenCorpus` and `TestTheCorpusNeverConfersControlAtCreationNorGrantsInSilence`
    among it); in `cmd/vtt`, `TestScenarioGoldenStreamsHaveNotDrifted`,
    `TestScenarioLibraryRunsSelfContained`, `TestEveryScenarioFileIsActuallyRun`,
    `TestEveryMapTheCorpusNamesIsInstalledAndUsed`,
    `TestAPlayersDumpNamesItsLogAgainstARealGateway`,
    `TestThreeRoleExitScenarioOverLiveServeSubprocess` and
    `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts` green,
    the soak's pinned counts unchanged; `client/test/fold-parity.test.ts`
    green under `env -u CLAUDECODE bun test`.
- **M7. A capitalised name changes nothing a reader assumes.** The golden
  normalisation replaces each minted id with `"p-" + name` byte for byte
  (`captureNormalizedStream`); `resolveParticipantIDPlaceholders` substitutes
  `{{id:<name>}}` byte for byte, case kept; golden directories are named by
  scenario, not participant. `sort.Strings` orders bytewise, so a report line
  listing several names now puts `Anna`, `Kalle` and `Lisa` before `agent`
  and `dm`; no test pins that order (M4, M6). Among character seats the order
  is the old one (`Anna` before `Kalle`, as `player` before `spectator`;
  `Anna` before `Lisa`), and the soak draws through `soakPlayerNames`' fixed
  order.
- **M8. The comment ledger.** `tools/check-comments.py --report` (run with
  `PYTHONDONTWRITEBYTECODE=1`):
  - `internal/harness/scenario.go` and `internal/harness/engine.go` sit at
    their ceilings; `cmd/vtt/harness_boot.go`, `internal/harness/soak.go`,
    `cmd/vtt/library_test.go`, `cmd/vtt/client_soak.go` and
    `cmd/vtt/harness_boot_test.go` just under theirs. The touched test files
    in `internal/harness` and `cmd/vtt/state_dump_switch_test.go` have a
    ceiling of nothing: a citation line of ids alone, which counts in no
    share, is the only comment they may gain.
  - In the copy, removing `mintInvites`' rewrite took `harness_boot.go`'s
    share above its ceiling, since more code lines went than comment lines:
    a notice, not a refusal, while the change adds no comment line there.
  - `Participant`'s doc in `scenario.go`, and the doc of
    `TestThreeRoleExitScenarioOverLiveServeSubprocess` and the file head of
    `library_test.go`, are blocks over the bound: check:comments refuses a
    line added to any of them unless the block ends within the bound.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened. A test that reads a
  renamed participant reads the new name and asserts what it asserted; the
  soak fake of D5 refuses what it refused; a pinned refusal text is
  narrower than `not authorized`, never wider.
- **P2. Rules 3, 4 and 5.** No contract change, no fold change, no
  game-system word: the names are people's, the actors the scenarios' own.
- **P3. Rules 8 and 10.** Names, never line numbers. A comment under
  `internal/` or `cmd/` is a warning or a pointer; a comment the change makes
  false is corrected in the commit that makes it false, within M8's ceilings
  and the block bound; no added comment line carries a banned term.
- **P4. Rule 9, answered.** MapTool's `Player.Role` is `PLAYER` or `GM`,
  chosen at connection, and ownership is held apart from it; here the role
  is derived from control (SPEC-009), so its model does not fit. MapTool has
  no scripted run of several connections to compare the scenario format
  with. Nothing borrowed; checked and rejected, as the ticket says.
- **P5. The owner's rulings above.** The names as ruled; the server,
  `CreateInvite`, the roles and what a grant does unchanged; no scenario
  file renamed; Go tests that build a participant in code unchanged.
- **P6. ADR-009 and `scenarios/goldens/README.md`.** No `-update` flag: a
  golden's `state.json` is derived by hand first, its `stream.json` recorded
  after, and both gates judge the pair.
- **P7. The mechanical gates.** `python3 tools/check-new-prose.py main` and
  `PYTHONDONTWRITEBYTECODE=1 python3 tools/check-comments.py main` before
  each commit; stage and commit in separate calls; bun under
  `env -u CLAUDECODE`; commands started with `/bin/sh`, never zsh;
  `go clean -cache` before and after a gate. No `client/src` change, so no
  webdist rebuild.

## Decisions

1. **D1. The library first, the refusal last.** Task 1 converts the library
   and the roster, Task 2 the two scenarios, Task 3 the load-time refusal and
   the mint, the reverse of the ticket's "What it touches".
   - **Forced by** every commit passing its hooks: a refusal at load before
     the files change reds every scenario but `smoke.json`, and the mint
     passing `player` through reds the same ones and the soak.
2. **D2. The names as ruled, case kept.** `player` → `Anna` and `spectator`
   → `Kalle` in `name`, `by` and `{{id:...}}`; `player1` → `Anna` and
   `player2` → `Lisa` in `soak.go`'s constants and every reader.
   - **Forced by** the owner's rulings, and by M1 and M7: no name collides
     and nothing lowercases.
3. **D3. Every `"role": "player"` becomes `"role": "spectator"`,** whatever
   the participant's name (`fighter`, `watcher`, `latecomer`, `brawler`,
   `patron`, `ana`, `bo`, `cy`, `dee` keep their names).
   - **Forced by** item 3, read with G4.
4. **D4. `characterSeats` keeps its `player` arm, and its doc stays.**
   - **Forced by** the non-goal on Go tests that build a participant in
     code, and M3: those tests reach the arm, and a stored `player` row
     still parses (VTT-912). VTT-380 stands as worded.
5. **D5. `qa_world_seat_test.go`'s soak fake judges a participant as
   `roleIn` does:** `eventFor` holds every participant who is neither `dm`
   nor `agent` to the player rules, and stamps `player` on what one of them
   is accepted, `spectator` otherwise.
   - **Forced by** M4, P1 (each of its tests refuses and accepts what it
     did) and SPEC-009 (a stored role of neither dm nor agent decides
     nothing). `soak_test.go`'s fake keeps its own role table, keys renamed:
     it reads no role from `SoakParticipants`, and it stayed green (M4).
6. **D6. `denials.json` grants Anna a new party member.** After the DM's
   `place_token` of `tok-npc` and before Anna's first refused step: an
   accepted `add_actor` of `act-lera`, named `Lera`, of kind
   `ACTOR_KIND_PARTY_MEMBER`, then an accepted `grant_actor_control` of it to
   `{{id:Anna}}` stating that kind. The DM's refused kindless grant of
   `act-npc` stays as it is.
   - **Forced by** her `move_token` of `tok-npc` standing for a player's
     refusal of a token she does not control: granting `act-npc` would make
     it hers. A party member because a player's character is one
     (blueprint 16); `Lera` is the corpus's party member already.
7. **D7. `adventure-night.json` grants Anna `act-fighter`** by an accepted
   `grant_actor_control` stating `ACTOR_KIND_PARTY_MEMBER`, straight after
   the DM's `load_adventure` and before her `load_adventure`.
   - **Forced by** the adventure being the scenario's only source of actors;
     `act-fighter` is already a party member, so the grant changes nothing
     in the state but its controllers (M6).
8. **D8. Anna's refusals pin the player's text.** `role "player" may not
   issue "<command>"` for each role-table refusal, and `is not controlled by
   participant` for the move, stopping before the per-run id.
   - **Forced by** item 5's "a check that fails on today's files" and M2:
     today each of those steps receives the spectator's text, so the
     scenario itself goes red, through `TestScenarioLibraryRunsSelfContained`,
     whenever its sender is not a player.
9. **D9. The goldens follow P6.**
   - `session-zero`, `story-table`, `three-role-exit`: `p-player` →
     `p-Anna` in `state.json`, then in `stream.json`; M6 shows that is what
     a recording gives.
   - `denials`, `adventure-night`: `state.json` derived by hand with M6's
     additions, then `stream.json` recorded with a throwaway test outside the
     tree (`go test -overlay`, a file calling `captureNormalizedStream` and
     writing the golden), never committed. The adventure's fresh rolls are
     testimony; the drift gate masks them and the state reads none (M5).
   - The other goldens and `scenarios/goldens/README.md` are untouched; no
     README sentence becomes false.
10. **D10. The format refuses `player` alone** (OD-2): `parseScenario`
    returns, before any step is decoded, an error naming the participant,
    saying a participant becomes a player when given a character and to
    declare them a spectator.
    - **Forced by** the ruling's wording and item 1.
11. **D11. `mintInvites` hands `CreateInvite` `identity.Role(p.Role)`,** and
    its inline comment goes with the rewrite. It gains no comment line but
    R2's citation line.
    - **Forced by** item 2 and M8's notice on `harness_boot.go`.
12. **D12. The fourth rule's sibling, the names, is held by a test, not by
    the one-time `git grep` of item 4.** R3's test walks `scenarios/*.json`
    and `harness.SoakParticipants()` and refuses a name that, lowercased and
    stripped of trailing digits, is `player` or `spectator`; it fails on an
    empty walk.
    - **Forced by** the rule being one the system keeps, and by the soak's
      own former names, `player1` and `player2`, which an equality check
      would let through.
13. **D13. Comments follow the code** (P3, M8), in the commit that changes
    it:
    - `library_test.go`'s file-head paragraph naming three-role-exit's
      `{{id:player}}` placeholder is deleted whole, which adds no line;
      `TestThreeRoleExitScenarioOverLiveServeSubprocess`'s doc is rewritten
      within the bound, as a pointer to what the test runs and to
      `RunScenario`'s `{{id:<name>}}`.
    - `client_soak.go`'s "fixed four-role roster" becomes "fixed roster"
      (its block is within the bound).
    - `Participant`'s doc and `SoakParticipants`' doc stay: they stay true.
    - `cmd/vtt/client_e2e_test.go`'s block naming `{{id:player}}` describes
      an `add_actor` carrying a controller, false since control moved to a
      grant and before this work; it is not this work's.
    - The comments in `engine.go` (`markUnprovableSilence`) and
      `engine_test.go` that count `denials.json`'s trailing denials stay
      true: the added steps are accepted and come before the run.
14. **D14. Kalle's refusals pin the spectator's text,** `role "spectator"
    may not issue "<command>"`, in `denials.json` and `adventure-night.json`.
    - **Forced by** the owner's answer to OD-1 (A).
15. **D15. The specification last, from the report.** SPEC-009's
    Consequences: the `mintInvites` sentence is replaced by the ticket's
    rules in words, with R1 to R4; its Status gains `internal/harness/scenario.go`
    and `cmd/vtt/harness_boot.go` among what implements it, and the new
    tests' files among what pins it; "Principles served" gains 34 if the
    `specification` skill's reading agrees (a file an LLM authors that says
    `player` where the server mints a spectator is a false description); its
    Requirements gain R1 to R4.
16. **D16. No `task e2e`, no webdist.** Nothing under `client/` changes but
    what the client tests read, and fold-parity is in `task check`.

## Open decisions for the owner

- **OD-1. Pin Kalle's refusals too** (D14).
  - **A (recommended).** In `denials.json` and `adventure-night.json`,
    each of Kalle's refused steps pins `role "spectator" may not issue
    "<command>"`. The pair then states its contrast, the same command
    refused to a player and to a spectator in each one's words, and
    Kalle's steps go red if the mint ever made Kalle anything else (R2,
    end to end). Cost: those steps' expectation strings; no golden moves,
    since a refused step writes nothing.
  - **B.** Leave them `not authorized`, as the ticket asks no more.
- **OD-2. What the format refuses at load** (D10).
  - **B (recommended).** `player` alone, as ruled. Any other role the
    server does not mint already fails at the mint, naming the participant
    (`CreateInvite`, VTT-911, reached unchanged through R2). Cost: none.
  - **A.** Every role `CreateInvite` would refuse, the empty one included.
    Cost: one more check and test, and a file with an empty role refused at
    load rather than at the mint.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill; the register's
words bind. Every new row reads `**OPEN — no test yet**` until its task
cites its tests.

| R | Item | Rule (as drafted; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | `harness.LoadScenario` refuses a scenario that declares a participant whose role is `player`, with an error that names the participant and says to declare them a spectator, and loads a scenario whose participants are dm, agent and spectator. | the check dropped; the error not naming the participant; `spectator` refused, alone or beside `player` |
| R2 | 2 | `mintInvites`, which `vtt client run` and `vtt client soak` boot through, mints each participant at the role the scenario declares, unchanged: a participant declared `player` fails to mint with `identity.CreateInvite`'s refusal, and every participant minted holds the role declared. | the rewrite restored; any role rewritten to another |
| R3 | 4 | No participant of a scenario file under `scenarios/`, and none of `harness.SoakParticipants`, has a name that, ignoring case and trailing digits, is `player` or `spectator`. | a participant named `player`, `Spectator` or `player2`, in a file or in the roster |
| R4 | 5 | In `scenarios/denials.json` and `scenarios/adventure-night.json`, each refused step that stands for a player's refusal pins the text the gateway gives a player, and is sent after an accepted `grant_actor_control` gives its sender an actor. | the grant dropped, moved after the first such step, or given to another participant; the state or stream golden not holding the grant |

**Candidates the sort may refuse:**
- **A row that `SoakParticipants` declares no `player`.** Refused: held by
  R2, through `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts`,
  since the soak boots through `mintInvites`.
- **A row that every scenario file declares only roles the server mints.**
  Refused: R1 at load and R2 at the mint, run over the library by
  `TestScenarioLibraryRunsSelfContained`.
- **A row naming Anna, Kalle and Lisa.** Refused: the names are data; the
  rule is R3.
- **A row per scenario of R4.** Refused: one rule; the two files are its
  cases.
- **A corpus check refusing a player's step pinned to `not authorized`
  alone.** Refused: the server's own text in the step is the observation; a
  check over expectation strings would be a second source. That a pin is
  never loosened back is the reading review's.
- **A row that the goldens hold the new ids.** Refused: the drift and fold
  gates hold the goldens whatever the names.
- **A row that Kalle's refusals pin the spectator's text** (OD-1 A).
  Refused: the steps are themselves the observation, and what they hold end
  to end, that the mint makes Kalle a spectator, is R2.

**The sort at Task 0** kept R1 to R4 as drafted, each with its "Red under"
observation, and refused the candidates above.

**Rows that become false:** none. Checked and left standing: **VTT-380**
(a code-built `player` participant is still a character seat, D4),
**VTT-911**, **VTT-912**, **VTT-358**, and **VTT-426**, whose test reads the
renamed token and keeps its name.

## The existing tests that change

- **Renamed reads of the soak's names (Task 1):** `internal/harness/soak_test.go`
  (its fake's role-table keys and `ids` keys, and every report string it
  asserts), `internal/harness/soak_log_seat_test.go`,
  `internal/harness/qa_log_seat_test.go`, `internal/harness/qa_soak_log_test.go`,
  `internal/harness/qa_world_seat_test.go`; ids built from a name follow it
  (`pid-player1` → `pid-Anna`).
- **A fake that judges as the server does (Task 1, D5):**
  `qa_world_seat_test.go`'s `qaWSSoak.eventFor`, behind
  `TestQAWSSoakPassesWhenNothingLeaks`,
  `TestQAWSSoakFailsWhenOnlyOneWorldSeatHearsADenial` and
  `TestQAWSSoakDoesNotReadCharacterSeats`.
- **A renamed token (Task 1):** `cmd/vtt/state_dump_switch_test.go`'s
  `TestAPlayersDumpNamesItsLogAgainstARealGateway`.
- **Unchanged, by the non-goal and D4:** `internal/harness/engine_test.go`,
  `late_leak_test.go`, `mutation_boundary_test.go`, `one_log_test.go`,
  `qa_one_log_test.go`, `corpus_actor_kind_test.go`,
  `internal/gateway/scenario_test.go`, and every client test.

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R4 and the
  candidates, as OD-1 and OD-2 leave them; allocate ids with
  `requirement-id`; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: the library and the roster declare and name people (R3; D2 to D5, D9, D12, D13; items 3 and 4)

- **Files:** every `scenarios/*.json` but `smoke.json` (names, `by`,
  placeholders, roles; `adventure-night.json` and `denials.json` renamed
  only, their grants are Task 2's); `scenarios/goldens/{session-zero,story-table,three-role-exit}/`
  (`state.json` first, then `stream.json`); `internal/harness/soak.go`
  (`SoakParticipants`, `soakPlayer1`, `soakPlayer2`); the test files under
  "The existing tests that change"; `internal/harness/scenario_test.go`
  (R3's test); `cmd/vtt/library_test.go` and `cmd/vtt/client_soak.go` (D13);
  `docs/requirements.md` (R3's cell).
- **Tests first, seen red:** R3's test, on today's names.
- **True when:** R3's test passes and failed before; both of item 3's and
  item 4's `git grep` commands print nothing; `go test ./internal/harness/`
  passes; in `cmd/vtt`, M6's named tests pass; `client/test/fold-parity.test.ts`
  passes; P7's checks pass.
- **Deliberate breaks:**
  - Name a participant `Player` in one scenario file: R3's test turns red.
  - Set `soakPlayer2` back to `player2`: R3's test turns red.
  - Leave `p-player` in `session-zero`'s `state.json`: `TestFoldGoldenCorpus`
    and fold-parity turn red; in its `stream.json`: the drift gate turns red.
  - Rename `a-player-switches-and-reconnects.json`'s participant but not
    its `{{id:...}}`: `TestScenarioLibraryRunsSelfContained` turns red.
  - Restore `eventFor`'s stored-role judgment: D5's tests turn red.
- **Phase 4a:** applies, narrowly: R3's walker, and D5's fake against
  `roleIn`.
- **The state here:** no scenario file or roster says `player`, and no
  participant is named for a role; the mint still rewrites, now nothing.

### Task 2: the two scenarios test a player (R4; D6 to D9; item 5)

- **Files:** `scenarios/denials.json`, `scenarios/adventure-night.json`;
  `scenarios/goldens/denials/` and `scenarios/goldens/adventure-night/`
  (`state.json` by hand, then `stream.json` recorded, D9);
  `cmd/vtt/library_test.go` and `cmd/vtt/scenario_goldens_test.go` (R4's
  citation lines); `docs/requirements.md` (R4's cell).
- **Tests first, seen red:** pin Anna's steps (D8, and Kalle's under
  OD-1 A) before adding either grant: `TestScenarioLibraryRunsSelfContained`
  reports each of Anna's steps receiving `role "spectator"`.
- **True when:** the library, both golden gates and fold-parity pass with
  the grants; P7's checks pass.
- **Deliberate breaks:**
  - Drop `denials.json`'s grant: Anna's steps turn red (M2).
  - Move it after Anna's first refused step: that step turns red.
  - Grant `act-lera` to Kalle instead: Anna's steps turn red.
  - Drop `adventure-night.json`'s grant: Anna's step turns red (M2).
  - Leave `act-fighter`'s controller fields out of the hand-derived
    `state.json`: `TestFoldGoldenCorpus` and fold-parity turn red.
  - Commit `denials`' old `stream.json`: the drift gate turns red.
- **Phase 4a:** applies: does each step standing for a player's refusal
  come from a player, and could any pass from a spectator.
- **The state here:** the library's player refusals are a player's.

### Task 3: the format refuses a player, and the mint passes the role unchanged (R1, R2; D10, D11; items 1 and 2)

- **Files:** `internal/harness/scenario.go` (`parseScenario`, and R1's
  citation line beside the check); `internal/harness/scenario_test.go`;
  `cmd/vtt/harness_boot.go` (`mintInvites`); `cmd/vtt/harness_boot_test.go`;
  `docs/requirements.md` (R1's and R2's cells).
- **Tests first, seen red:**
  - `LoadScenario` on a file declaring `{"name": "Anna", "role": "player"}`
    fails naming `Anna` and saying `spectator`; one declaring dm, agent and
    spectator loads.
  - `mintInvites` on a directory `campaign.EnsureDir` made, given a scenario
    built in code with a `player` participant, returns `CreateInvite`'s
    refusal; given dm, agent and spectator participants, each minted id's
    stored role, read back through `identity`, is the one declared.
- **True when:** the new tests pass and failed before; `go test
  ./internal/harness/` and M6's `cmd/vtt` tests pass; P7's checks pass.
- **Deliberate breaks:**
  - Drop the check in `parseScenario`: R1's refusal case turns red.
  - Refuse `spectator` as well: R1's loading case, and the library, turn
    red.
  - Leave the participant's name out of the error: R1's name assertion
    turns red.
  - Restore `mintInvites`' rewrite: R2's refusal case turns red.
  - Rewrite `agent` to `dm` in `mintInvites`: R2's stored-role case turns
    red.
- **Phase 4a:** applies: the format and the mint, by their signatures.
- **The state here:** a scenario cannot declare a player, and the harness
  mints what a scenario declares.

### Task 4: the gate (item 6)

- **Do:** `PYTHONDONTWRITEBYTECODE=1 python3 tools/check-comments.py main`,
  and `--write-ledger` once, last, where a share has fallen more than the
  band under its ceiling (D13's deletions, M8); `go clean -cache`; check
  free space and the load (`os.getloadavg()`); run `task check` whole on the
  last code commit, detached under `/bin/sh`, output to a file, requiring
  its completion line; `go clean -cache`. Tell the owner the client's stored
  mutation report is out of date from Task 1's golden commit and which
  commit it measured (SPEC-023, G3).
- **True when:** `task check` is green.

### Task 5: the report, then the specification

- **The report:** `docs/reports/2026-11-22-a-scenario-participant-becomes-a-player-only-by-a-grant.md`,
  with the `implementation-report` skill and its reading review: each "Done
  looks like" item with its observation; the rows and what turned each red;
  Rule 9 as P4 answers it; G1 to G5, OD-1's and OD-2's answers; Phase 4a's
  adjudications under their own heading; the gate.
- **SPEC-009,** with the `specification` skill, from the report, as D15
  says.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the specification;
  push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, SPEC-009 as drafted, the scenario files and
goldens as data, and the surfaces as signatures, never the source:
`go doc ./internal/harness` (`LoadScenario`, `Participant`, `Scenario`,
`SoakParticipants`, `RunScenario`, `Dialer`), and from `cmd/vtt`, a package
`main`, the signatures of `mintInvites`, `bootSelfContained` and
`dialerFor`, and `identity.CreateInvite` and `identity.Open`. Tell it: never
run `task check`, `task e2e` or Stryker, nor the whole `./cmd/vtt` package;
its files are new `qa_*_test.go` files; `go-arch-lint` holds `internal/harness`
to importing no `cmd` code; semgrep refuses game-system words in platform
code; the tree is hashed before and after it. Known and not findings: a
code-built `player` participant still runs (D4); an unknown role fails at
the mint, not at load (OD-2's answer); report lines order names bytewise
(M7).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "no other") names the search that bounds it; check every
renamed read asserts what it asserted, and that no pinned refusal was
widened (rule 2); read D5's fake against `roleIn`; read each hand-derived
`state.json` against M6's additions; check each comment under `internal/`
and `cmd/` is a warning or a pointer, within M8's ceilings and the block
bound, and that `library_test.go` and `client_soak.go` no longer say what
D13 removes.
