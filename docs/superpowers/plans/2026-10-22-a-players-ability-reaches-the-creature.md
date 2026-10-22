# Plan: a player's ability reaches the creature they aim it at

- **Ticket:**
  `docs/superpowers/specs/2026-10-22-a-players-ability-reaches-the-creature-design.md`.
- **Verdict:** passes with gaps (G1-G5, below). Verified 2026-10-22 by an agent
  that did not write the ticket. It was read-only against HEAD `41ccead`.
  Probes ran out of the tree: a scratch bun test that imports the tree's
  modules, a `vtt` built into the scratchpad serving a scratch campaign, and a
  `git archive HEAD` copy for `buf generate`, toolgen and `vite build`.
  `task check`, `task e2e` and mutation runs were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-22-a-players-ability-reaches-the-creature.md`.
- **Base:** main `41ccead`.
- **Branch:** `fix/a-players-ability-reaches-the-creature`.
- **Signed off:** 2026-10-22 by the owner ("Ja, kör"), with one change: the
  e2e reads what happened from the Story, not the ticker (D4). D8 is kept
  ("Ja, ta med det"). The question rule holds ("Ja, samma regel").

## The gaps this plan inherits

- **G1. Item 3's file list was short, and nothing reads the text.** The ticket
  now names the files.
  - Two files are missing from the list:
    - `contract/testdata/expected_tools.json`, which is edited by hand and read
      by `TestToolsMatchGolden`;
    - `cmd/vtt/tools.json`, which `generate:contract`'s `cp` writes outside
      `contract/gen/`.
  - `TestToolsMatchGolden` holds the bytes, whatever they say.
  - Fix: D6 adds a test of the description's wording.
- **G2. Item 4's named test does not observe "a token id included".**
  - `TestResolveUnknownTarget`'s state holds no token.
  - A `Resolve` that mapped a token id to its actor would leave it green.
  - Fix: D7 adds the test.
- **G3. Item 2 cannot be met as the fixture stands.**
  - The fixture's player character `act-lera` is made through the console with
    no attributes.
  - Through the panel today she is refused `unknown target actor` like any
    player, because `Resolve` reads the targets before the caster's
    attributes. Once the fix sends her target's actor, her Longsword Strike is
    refused for her missing `str` (P2, which aimed with an actor id).
  - Fix: D4 has the e2e hand the player the adventure's Human Fighter. That
    grant moves the player's default viewpoint (P3), so the test runs last.
- **G4. The contract's own examples use token ids for these fields.**
  - `contract/testdata/use_ability_command.json` and `ability_used_envelope.json`
    carry `actorId: "tok-ursus"` and `targetIds: ["tok-goblin-2",
    "tok-goblin-3"]`. `tok-ursus` is a token in `client_command.json`,
    `envelope.json` and `move_token_request.json`.
  - `client/test/commands.test.ts` builds the same command and compares values.
  - Fix: D8 renames them. The owner kept it at sign-off, and the ticket's list
    now names the files.
- **G5. The DM claim was narrower than the ticket said.** The ticket now says
  so. A DM is refused "the
  same way" only when aiming as an actor granted to them (SPEC-022: a DM's chips
  list the actors granted them). This has no effect on the plan.

## Established here, not in the ticket

- **E1. A comment-only proto change does not move the bundle (P6).** So Task 1's
  commit carries no `cmd/vtt/webdist` change, and Task 4's does.

## Measurements this plan stands on

Every probe ran outside the repository, at `41ccead`.

| # | Probe | Result |
|---|---|---|
| P1 | A scratch bun test renders `renderPlayerPanel` with token `t2` of actor `a2`, clicks "Bran" and expects `targetIds: ["a2"]` | red: received `["t2"]` |
| P2 | Scratch `vtt serve` on `dnd45e-minimal` and `adventures/`, driven over the wire through `table.spec.ts`'s sequence: start session, load goblin-ambush, add `act-lera` as a party member, grant her to the player, place `tok-lera` at 0,0, then the player moves her to 6,5. Then Lera's Longsword Strike | refused: `rules: resolve: actor "act-lera" missing attribute "str" required by ability "longsword-strike"'s resolution roll` |
| P3 | The DM grants `act-fighter` to the same player as a party member, and the player opens a new connection with `after=0` and no `log` | `WatchableChanged` lists `act-fighter` "Human Fighter", then `act-lera` "Lera"; `ViewpointChanged` names `act-fighter`; the fold holds `tok-cutter`→`act-cutter` "Goblin Cutter", hp 8/8, and `tok-archer`, `tok-fighter` and `tok-lera` |
| P4 | That player: `useAbility{act-fighter, longsword-strike, ["tok-cutter"]}`, then the same with `["act-cutter"]` | the first is refused with `rules: resolve: unknown target actor "tok-cutter"`. The second is ok, and the player's log is sent `AbilityUsed`, here a miss (`14 vs 15`) |
| P5 | The actor-id strike repeated until it hits | `AbilityUsed` (hit, 16 vs 15), `ResourceChanged act-cutter hp → 1` and `ConditionApplied`, all sent to the player |
| P6 | Copy of the tree: a comment added on both `target_ids`, then `buf lint` and `buf generate` | four files change: `commands.pb.go`, `events.pb.go` (the struct is realigned), `commands_pb.ts` and `events_pb.ts`. `vite build` output is byte-identical to the committed `cmd/vtt/webdist` |
| P7 | Copy: a `fieldDocs` entry for `use_ability`'s `targetIds`, toolgen rerun and the `cp` made | `TestToolsMatchGolden` red, with a one-line diff (the `description`) until `expected_tools.json` follows; `TestRunOutputMatchesCommittedToolsJSON` green |
| P8 | `check-comments.py`'s own `measure` | `internal/rules/resolve_test.go` 109 of 878 lines (12.41%, ceiling 12.5): a ten-line test may add at most two comment lines besides its citation line. `client/src/view/player.ts` 68 of 247 (27.53%, ceiling 27.6) |
| P9 | grep | one `useAbility(` call site. The client test's name is cited nowhere. `use_ability`'s description occurs in `tools/toolgen/main.go`, both `tools.json` files and `expected_tools.json`. No `client/e2e` spec uses an ability. No adventure places two tokens for one actor |
| P10 | `go test ./internal/rules -run 'TestResolveUnknownTarget$'` | green today |

Not measured: the browser e2e (verifier constraint), and the Go break K4 (by
reading `Resolve` only).

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no floor, ceiling or exemption moves. `tools/comment-ceilings.txt` is not
    touched.
  - 3: additive only, which here means comments and a description. No field,
    number or name changes. `check:breaking` only reports pre-release, so do not
    take its silence as a guard.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: no comment line is added under `client/src`, and `player.ts` changes
    only code (P8). Test files carry `// VTT-NNN` citation lines, which the
    comment gate sets aside.
- **The comment budget in `resolve_test.go`:** at most two comment lines besides
  the citation line (P8). Run `python3 tools/check-comments.py main` after
  staging.
- **Requirement ids** come from `requirement-id` after sign-off, never by hand
  (SPEC-008). A test file that cites an id needs the row to exist first.
- **Tier 1 runs at pre-commit:** `bun test client/test contract contract-spike`
  and `task client:typecheck`. The source and its tests go in one commit.
- **bun is quiet under `CLAUDECODE`.** Run it as `env -u CLAUDECODE bun test …`.
- **The bundle.**
  - The commit hook does not catch a stale bundle. Run `task build:client`
    before Task 4's commit.
  - `task check:drift` reads `git status --porcelain` over `contract/gen`,
    `cmd/vtt/tools.json` and `cmd/vtt/webdist`, so it passes only after the
    commit. Pre-push runs it.
  - `task e2e` builds `vtt` from the working tree, with whatever webdist is
    there. A client break reaches the e2e only after `task build:client`.
- **TS mutation.** Any change under `client/src` or `client/test` makes
  `check:ts-mutation` re-mutate the client. `view/player.ts` has no adjudicated
  mutants.
- **`review-record` refuses untracked files.** Commit the ticket, this plan and
  the findings in Task 0.
- **Mechanics.**
  - A new test goes after a closing brace.
  - Stage and commit in separate calls.
  - Undo a break with its inverse edit and check with grep, never with
    `git checkout --`.

## Rule 9: how MapTool does this

Checked in `~/dev/RPTool`:

- **What MapTool does.**
  - In `rule-framework/library/mtscript/public/useAbility.mts` the caster is
    `currentToken()`. In its MANUAL fallback (an area power resolves its own
    targets) the targets are `getSelected("json")`, which returns the selected
    tokens' ids.
  - Each target's stats are read off the token: `getProperty("HP", tid)`.
- **Why that works for MapTool.** `Token` in
  `maptool/src/main/java/net/rptools/maptool/model/Token.java` holds the
  properties, in `propertyMapCI` through `getProperty`. A macro's token
  argument is resolved by `FunctionUtil.getTokenFromParam`, through
  `FindTokenFunctions.findToken` and `Zone.resolveToken`, by GUID or name. A
  MapTool token IS the creature, so aiming at the token and aiming at the
  creature are the same act.
- **What we borrow.** You point at a creature through the piece on the board:
  MapTool's selection on the map becomes our one button per token in range. We
  also measure range between pieces, as MapTool's Zone geometry does and as
  `Resolve` does with tokens.
- **What does not fit.** Carrying the token's id into the rules. In our split
  the stats live on the Actor, and the Token is only where that actor stands on
  a scene. So the token-to-actor step (`Token.ActorID`) happens at the click.
- **Checked and rejected.** Making the server accept token ids, MapTool-style.
  It would give `target_ids` two meanings, against SPEC-016 and SPEC-018,
  which already judge by actor, and against ticket item 4.
- **Checked and not acted on.** An actor with two tokens on one scene. MapTool
  cannot have this, since each token is its own creature. The engine allows it,
  and `Resolve`'s `findToken` measures from the lowest token id, while the
  panel offers each token in range. No shipped adventure, fixture or e2e
  places two tokens for one actor (P9). Under the owner's 2026-10-21 ruling on
  nearly theoretical edge cases, it gets no code, no test and no debt entry.

## Decisions

1. **D1. The fix is at the click.** The target button sends the token's actor:
   `send(useAbility(actorId, armed.id, [t.ActorID]))`.
   - Forced by the ticket's first rule, and by `Resolve`, SPEC-016 and SPEC-018
     all keying on actors (Rule 9 above).
   - `t.ActorID` always names an actor the fold holds: `fold.ts`'s
     `tokenPlaced` arm refuses a token for an unknown actor.
2. **D2. `player.ts` gains no comment.** Forced by rule 10 and P8.
3. **D3. The client test keeps its fixture and changes its expectation.**
   - The test is renamed to "a target button sends the ability at that token's
     actor, then disarms".
   - The fixture stays token `t2`, actor `a2`, and the test expects
     `targetIds: ["a2"]`.
   - Forced by item 1. The ids must stay distinct, or K6 survives (F1). The
     rename is free: the name is cited nowhere (P9).
4. **D4. The e2e is the last test of `client/e2e/table.spec.ts`.**
   - **Name:** "a player's ability reaches the creature they aim it at", under
     a `// VTT-NNN` line for R1. It takes `{ page, browser }`.
   - **Setup.** The DM grants the adventure's Human Fighter (`act-fighter`) to
     the player as a party member, through the console's grant row. The fighter
     stands at 0,0 and the Goblin Cutter (`act-cutter`, hp 8) at 1,0. Longsword
     Strike has reach 1.
   - **Why last.** It must come after "the story feed and ticker record what
     happened" and "a player sees their own panel and can move their token".
     Both open as the player and expect Lera's log, and after the grant a fresh
     player connection opens on `act-fighter` (P3; SPEC-015's
     `defaultViewpoint`).
   - **The steps:**
     - A second page opens as the DM and grants `act-fighter` using the same
       selectors as the Lera grant: `.control-actor[data-actor="act-fighter"]`,
       `.grant-target` = `ids.player`, `.grant-kind` =
       `ACTOR_KIND_PARTY_MEMBER`, `button.grant`. Then, after the same 400 ms
       wait the Lera grant uses, `expectNoRefusal` runs.
     - The player's page opens and clicks the "Human Fighter" chip in `.player`.
       The gateway applies nothing for the actor already shown (SPEC-022), so
       this only makes the test independent of the default.
     - It waits for `.player button.chip.sel` to read "Human Fighter".
     - It clicks the "Longsword Strike" chip, then the `[data-action="use-ability"]`
       button labelled "Goblin Cutter".
   - **What it observes.**
     - Before the player's page opens, it listens with `page.on("websocket",
       …)` and the socket's `framesent`, and keeps every sent frame that parses
       as a `useAbility` command (`Wire` sends
       `JSON.stringify(toJson(ClientCommandSchema, …))`).
     - It reads the Story, never the ticker: ruling B-3 in
       `docs/superpowers/notes/2026-10-22-alpha-one-findings.md` gives the
       ticker to the DM alone, and the third ticket in its order of work
       removes it from a player's window.
     - It waits for a `.feed .mechanical` line "act-fighter used
       longsword-strike" OR `.toast` (`locator.or(...).first()`). Then it runs
       `expectNoRefusal(page, "use ability")`, so today's tree fails with the
       refusal's own text.
     - Only then, after the `.or()` wait and `expectNoRefusal`, so that
       today's tree fails with the refusal's text, it requires exactly one kept
       frame, with `actorId` `act-fighter` and `targetIds` `["act-cutter"]`.
       This is what tells the creature from the token (K6) and from the caster
       (K7): `describe` writes no target, and a miss writes no hp line.
     - The line appears exactly once.
     - Every `.feed .mechanical` line matching `^act-cutter hp → ` shows a
       value below 8. A hit writes one (P5), and a miss writes none (P4),
       because the dice are random.
     - Screenshot `08-player-ability-resolved`.
   - **Forced by** G3, P2-P5 and item 2.
   - **Rejected:** a new spec file with its own server. It would start a session
     and load the adventure again for one test. That is a preference, named as
     one.
5. **D5. Both proto fields get a comment.** No field, number or name changes
   (rule 3). SPEC-007 keeps proto doc comments as warnings to whoever edits.
   - `UseAbility.target_ids`: `// Actor ids, never token ids: rules.Resolve looks
     each one up among the folded actors and refuses one it does not find
     (SPEC-018).`
   - `AbilityUsed.target_ids`: `// Actor ids, as UseAbility.target_ids names
     them: the projection forwards this event by the actors it names (SPEC-016).`
   - `check:drift` is the comments' only gate: toolgen does not read them.
6. **D6. The tool's description comes from `fieldDocs`.**
   - Added on `"vtt.v1.UseAbility"`, key `targetIds`, with `requiredOverride`
     left nil. All three fields are genuinely required.
   - Proposed text: "The ACTOR ids of the creatures the ability is aimed at,
     never a token's id: a token is only where an actor stands on a scene, and
     an id that names no actor is refused. Each target must have a token in the
     caster's scene within the ability's range."
   - Each clause is `Resolve`'s: the unknown-target refusal, and the
     active-scene range check.
   - `TestUseAbilitySaysItsTargetsAreActors` in `tools/toolgen/main_test.go`
     requires "ACTOR ids", "never a token's id" and "names no actor is refused".
     It mirrors `TestMoveTokenReasonSaysOnlyTheDMAndTheAgentReadIt`.
   - `expected_tools.json` gains the one line by hand (P7).
   - Forced by item 3 and G1.
7. **D7. A new Go test, `TestResolveRefusesATokenIDAsATarget`.** It goes in
   `internal/rules/resolve_test.go`, after `TestResolveUnknownTarget`.
   - The state holds actors `a` (`brawn` 3) and `b` (`guard` 10), and tokens
     `ta` at 0,0 and `tb` at 1,0 on `s1`.
   - It calls `useAbility("a", "jab", "tb")` and expects the error `unknown
     target actor "tb"` and no events.
   - At most two comment lines (P8). Forced by G2.
8. **D8. The contract's examples name actors.**
   - The changes:
     - In `use_ability_command.json` and `ability_used_envelope.json`,
       `tok-ursus` becomes `act-ursus`, and `tok-goblin-2`/`-3` become
       `act-goblin-2`/`-3`, the `outcomeSummary` included.
     - `client/test/commands.test.ts` follows in "useAbility matches the
       committed fixture, targets included". In "every constructed command
       carries a request id", `useAbility("a1", "ab", ["t2"])` becomes `["a2"]`.
   - `attack_rolled.json` is left as it is: nothing in production writes an
     `AttackRolled` (SPEC-018), and the ticket's rule names only these two
     fields.
   - Forced by G4, and kept by the owner at sign-off.
9. **D9. One commit per task.** Task 1 carries no webdist change (E1). Task 4
   carries the rebuilt `cmd/vtt/webdist`.
10. **D10. The recipe's labels are `spec silent` and `test data missing`.**
    - `spec silent`: nothing said which id, so the client test pinned the
      token's.
    - `test data missing`: no e2e or scenario drove a browser-sent ability.
11. **D11. `Specifications this moves: None.` stands.** It is a reading. SPEC-016
    and SPEC-018 already say actors. SPEC-022's panel paragraph does not
    describe target buttons, and the requirement row is the record.

## Requirements

Two rows, allocated by `requirement-id` at Task 0: R1 is VTT-436 and R2 is
VTT-437. They are written as `**OPEN — no test yet**` and filled by the tasks
below.

- **R1.** The player panel aims an ability at the actor the clicked token shows,
  never at the token.
  - Evidence: `client/test/spectator-view.test.ts#a target button sends the
    ability at that token's actor, then disarms` and
    `client/e2e/table.spec.ts#a player's ability reaches the creature they aim
    it at`.
- **R2.** `UseAbility.target_ids` and `AbilityUsed.target_ids` name actors.
  - Evidence: `internal/rules/resolve_test.go#TestResolveRefusesATokenIDAsATarget`
    and `tools/toolgen/main_test.go#TestUseAbilitySaysItsTargetsAreActors`,
    and for `AbilityUsed` the existing
    `internal/rules/resolve_v2_test.go#TestResolveV2WardShiftMultiTarget`,
    which requires `AbilityUsed.TargetIds` to be the actors `b` and `c` while
    their tokens are `tb` and `tc`.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off on this plan.
  - Commit the ticket, this plan and the findings they cite,
    `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`.
  - Allocate R1 and R2 with `requirement-id`, and add both rows to
    `docs/requirements.md` with OPEN evidence.
- **True when:** `git status` is clean and the two rows exist.

### Task 1: the contract and the tool say actors (R2, item 3)

- **Files:**
  - `contract/vtt/v1/commands.proto`, `contract/vtt/v1/events.proto` (D5);
  - `tools/toolgen/main.go`, `tools/toolgen/main_test.go`,
    `contract/testdata/expected_tools.json` (D6);
  - generated by `task generate:contract`: `contract/gen/go/vtt/v1/{commands,events}.pb.go`,
    `contract/gen/ts/vtt/v1/{commands,events}_pb.ts`, `contract/gen/tools/tools.json`
    and `cmd/vtt/tools.json`;
  - `docs/requirements.md` (half of R2's evidence).
- **Order:**
  - Write the test and see it red (no description).
  - Add the `fieldDocs` entry and the proto comments.
  - Run `task generate:contract`.
  - Edit `expected_tools.json`.
  - Then `go test ./tools/toolgen/ ./contract/... ./cmd/vtt/` and
    `env -u CLAUDECODE bun test client/test contract contract-spike`.
- **True when:**
  - Toolgen is green.
  - `git status` lists exactly the files above. `cmd/vtt/webdist` is not among
    them (E1).
- **Breaks:**
  - **K1:** delete the `targetIds` entry and regenerate. The new test and
    `TestToolsMatchGolden` go red.
  - **K2:** make the description say "token ids" and update the golden to match.
    Only the new test goes red, which shows it reads the content, not the bytes.

### Task 2: the server's refusal of a token id is observed (R2, item 4)

- **Files:** `internal/rules/resolve_test.go` (D7);
  `internal/rules/resolve_v2_test.go`, where a `// VTT-437` line goes above
  `TestResolveV2WardShiftMultiTarget`, since `check:requirements-chain` requires
  an evidence file to carry the row's id; and `docs/requirements.md`.
- **True when:** the new test and `TestResolveUnknownTarget` are both green, and
  `check-comments.py main` passes after staging.
- **Breaks:**
  - **K4:** in `Resolve`'s target loop, when `st.Actors[tid]` misses, take
    `st.Tokens[tid]`'s `ActorID` instead. The new test goes red and
    `TestResolveUnknownTarget` stays green. Record that in the report as the
    reason the new test exists.

### Task 3: the contract's examples name actors (D8)

- **Files:** `contract/testdata/use_ability_command.json`,
  `contract/testdata/ability_used_envelope.json` and
  `client/test/commands.test.ts`.
- **True when:**
  - `TestUseAbilityCommandRoundTrip`, `TestAbilityUsedEnvelopeRoundTrip`,
    `contract/events.test.ts` and `client/test/commands.test.ts` are all green.
  - `grep -n 'tok-' contract/testdata/use_ability_command.json
    contract/testdata/ability_used_envelope.json` prints nothing.
- **Breaks:**
  - **K5:** change the fixture's `targetIds` back without touching
    `commands.test.ts`. The client fixture test goes red.

### Task 4: the panel aims at the actor (R1, items 1 and 2)

- **Files:**
  - `client/test/spectator-view.test.ts` (D3) and `client/e2e/table.spec.ts`
    (D4);
  - `client/src/view/player.ts` (D1);
  - `cmd/vtt/webdist/` from `task build:client`;
  - `docs/requirements.md`.
- **Order:** tests before code.
  1. Edit the client test. It goes red with received `["t2"]` (P1).
  2. Append the e2e and run `task e2e` on the unchanged webdist. It goes red
     with `use ability: it was refused — refused: rules: resolve: unknown
     target actor "tok-cutter"`. This is item 2's "fails on today's tree".
     Check the machine load first: the e2e deadlines fail under load.
  3. Make the one-line fix.
  4. Run `task build:client`.
  5. Both tests green, and `task client:typecheck` clean.
- **True when:**
  - Both tests are green.
  - `git status` lists the files above and `cmd/vtt/webdist/assets/index.js`.
- **Breaks:** each is applied, then `task build:client` for the e2e, then
  reverted and rebuilt:
  - **K6:** put back `[t.ID]`, which is today's line. The client test goes red
    (`["t2"]`), and the e2e goes red with the refusal text above.
  - **K7:** send the caster, `[actorId]`. The client test goes red (`["a1"]`),
    and the e2e goes red on the sent frame's `targetIds` (`["act-fighter"]`).
- **Fixture check:**
  - **F1:** give the test's token and actor the same id, then apply K6. The test
    stays green, which shows the distinct ids are what make it see the defect.
    Undo it.

### Task 5: the recipe (item 5)

- **File:** `docs/verification-debt.md`, a new dated entry above "Open debt", in
  the shape of the 2026-10-04 entry:
  - **The defect:** the target button sent the token's id, and `Resolve` reads
    actors.
  - **Recipe:** in `client/src/view/player.ts`, in `renderPlayerPanel`'s target
    button, replace `[t.ActorID]` with `[t.ID]`, then run `task build:client`.
  - **What should have caught it:** the client test of the target button,
    which instead pinned it, because neither the contract nor the tool said
    which id. And no e2e had a player use an ability.
  - **Labels:** D10.
  - **Closed by:** R1's two tests, which went red on K6 in Task 4.
- **True when:** the recipe, applied by hand, turns VTT-436's two tests red
  (it is K6), and its inverse edit leaves `git diff` on `client/src` empty.
  It also rebuilds the bundle, so the inverse is followed by
  `task build:client` and leaves `cmd/vtt/webdist` clean as well. Neither
  `check:new-prose` nor `check:doc-owner` reads a Markdown file, so neither is
  Task 5's check.

### Task 6: the gate, whole

- **Do:** run `task check` whole on Task 5's commit, then `task e2e` on the same
  commit.
- **True when:** both are green. `check:drift` is green, with every regenerated
  file committed, and `check:requirements-chain` resolves R1 and R2.

### Task 7: the report and the specification step

- **Do:**
  - Write the implementation report under `docs/reports/`. It covers each "Done
    looks like" item with its observation, the breaks K1-K7 and F1 with what
    went red, and Phase 4a's adjudications under their own heading.
  - The specification step: `None.` (D11).

## For Phase 4a

Hand the QA agent:

- the two rules;
- `go doc ./internal/rules Resolve`;
- the exported `renderPlayerPanel` and `useAbility` signatures;
- the `use_ability` entry of `contract/gen/tools/tools.json`;
- this plan's measurements P2-P5, so its tests can drive a statted caster.

Tell it that `go-arch-lint` holds toolgen to `contract` alone, so a test there
reads files rather than importing `internal/`.

## Forward coupling

The e2e finds its target by the label "Goblin Cutter". That label is the actor's
name, which a player is sent today (P3). Finding A-6 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`, the fifth ticket in
its order of work, will stop sending a player a non-party actor's name. When it lands, this locator changes
with it. The test's other observations survive that change:

- the `AbilityUsed` line names ids only;
- a missing hp row passes the hp check.

The e2e reads the Story's mechanical lines, which `describe` in
`client/src/view/spectator.ts` writes. A ticket that rewords those lines (finding
A-2 there) changes the e2e's expected text with them.
