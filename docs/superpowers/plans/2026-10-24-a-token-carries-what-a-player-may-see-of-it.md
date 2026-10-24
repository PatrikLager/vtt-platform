# Plan: a token carries what a player may see of a creature, and a player is sent no creature's name or statblock

- **Ticket:**
  `docs/superpowers/specs/2026-10-24-a-token-carries-what-a-player-may-see-of-it-design.md`,
  from rulings A-6, K-1 and B-5, with A-2 and A-10, of
  `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`, the fifth ticket
  in that note's order of work. It also fixes the unstable test the owner
  folded into it ("ta ticket 5 och rätta det instabila testet i den").
- **Verdict:** passes, with one gap (H1).
  - Verified 2026-10-24 by an agent that did not write the ticket, read-only
    against HEAD `1e24277`.
  - The first verification returned the ticket on three findings:
    - F1: item 2 bounded four new fields against SPEC-018's admission rule,
      the owner's ruling of 2026-10-09;
    - F2: SPEC-021, which cites VTT-313 in prose, was not named;
    - F3: the new command met `COMMAND_SURFACE` in
      `client/test/command-surface.test.ts`, which the ticket did not address.
  - The writer revised the ticket, and the owner answered F3 ("Ja, en
    kontroll i konsolen") and the outcome line (below). The second
    verification checked those three findings alone, and all three are fixed.
  - Probes ran outside the tree:
    - a server and client prototype, in a detached worktree at HEAD;
    - a clean worktree at HEAD beside it, for baselines;
    - scratch `perceive` tests.

    Both worktrees were removed afterwards.
  - `task check`, `task e2e` and the mutation gates were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md`.
- **Base:** main `1e24277`.
- **Branch:** `fix/a-token-carries-what-a-player-may-see-of-it`.
- **Owner's rulings taken in:**
  - **A-6, 2026-10-22, "servern ska inte skicka det".** The server sends a
    player no non-party actor's name and no statblock.
  - **K-1, 2026-10-22.** Hovering over or right-clicking a token shows what
    the character perceives of it.
  - **B-5, 2026-10-22, "Låna MapTools uppdelning rakt av".** That
    information lives on the token: a name and notes players see, and a GM
    name and GM notes only the GM sees.
  - **B-1, 2026-10-22.** Event data follows sight strictly: a player whose
    character does not see something is sent nothing of it, "not even that
    it happened".
  - **2026-10-24:**
    - "Inget hp; utfallsraden står": no creature status reaches a player, and
      `outcome_summary` is still sent whole;
    - "Ja, ett eget kommando": a command rewrites a token's four fields;
    - "Ja, som i dag": a left click still moves (A-5);
    - "En ticket, servern först": one branch, the tasks in "What it touches"
      order;
    - "Ja, en kontroll i konsolen": the DM console offers the rewrite;
    - the outcome line: "man visar att det har blivit träff, men inte 'varför'
      … bara longsword strike hit (dice: 17) typ. vad man slog, men inte att
      det var för att goblin hade 15 i ac".
  - **Earlier rulings that stand:**
    - 2026-10-09: a field outside SPEC-018's table is unbounded on purpose;
    - 2026-10-19: a creature a player runs is a plain party member;
    - 2026-09-23: one source;
    - 2026-09-04: nobody uses the product, so nothing is migrated.
- **Signed off:** 2026-10-24, by the owner ("Ja, kör").

## The gap this plan inherits

- **H1. Three behaviours are observable only in a real browser, and no
  verifier ran one.**
  - The hover half of item 9 is CSS `:hover` (D15), which happy-dom does not
    evaluate.
  - The card must never intercept a left click (`pointer-events: none`).
  - The console's rewrite control has never been clicked in a browser.
  - Each first shows when Task 7 runs `task e2e`, which carries a hover beat
    and a right-click beat (W1).

## Established here, not in the ticket

- **E1. A kind change in sight needs the token hidden and placed again
  around the re-introduction (P3).** This closes the ticket's third open
  point.
  - In both directions the frames are: `TokenHidden` of each of the actor's
    tokens on the board, a bare `ActorRemoved`, `ActorAdded`, the grants, the
    conditions, then `TokenPlaced`. The forwarded `ActorControlGranted`
    follows.
  - Both folds take the forwarded grant, which repeats a controller the
    introduction already granted: their idempotent return comes after the
    kind write.
  - Every frame folded in the harness, which fails on a refusal.
- **E2. A reshaping kind change out of sight must wait until the actor is
  seen (P2).**
  - Re-introducing at once breaks
    `TestAKindChangedOutOfSightIsCorrectedOnSight`. That test holds VTT-247
    ("kind is status") and B-1: a charm on an unseen goblin sends the player
    nothing.
  - Holding the actor as last seen, and re-introducing it when it is seen,
    keeps that test green (D8).
- **E3. A creature added again under another name looks the same to a
  character log.**
  - The log holds a creature's id, module id and kind, so only another
    module id tells two creatures apart. No name leaks either way.
  - Four existing tests use a renamed or re-attributed non-party actor to
    show a re-introduction, and change for that reason (Task 5).
- **E4. `classify` keeps its signature.**
  - `projection_internal_test.go` calls `classify(env, now)`, so the party
    membership `ResourceChanged` needs rides on the look (D9).
  - `look` already walks `engine.IsPartyMember` for its actors.
- **E5. What the command-surface guard needs (P9).** Three of its seven
  tests, read against the code:
  - a declared surface for the new case: red today;
  - a builder in `commands.ts` named by the case, `describeToken`;
  - for a `dm-console` row with an `action`, a control with that
    `data-action` rendered by `renderDMConsole`.

  The guard's fixture `richState` holds token `t1`, so a per-token control
  renders there.
- **E6. Today's bundle refuses an unknown key.**
  - `docs/superpowers/plans/2026-10-23-a-player-without-a-character-watches-as-a-spectator.md`
    measured `Wire.handleFrame`'s `fromJson` throwing on a key its schema
    lacks (its P7 and E3).
  - The first commit that can put `name` or `outcomes` on the wire must carry
    a bundle built from the regenerated `contract/gen/ts`. That is Task 2.
- **E7. The line that draws a disc's initial carries two mutation
  adjudications (P8).**
  - In `tools/ts-mutation-equivalents.txt`, `client/src/view/grid.ts
    StringLiteral offset=39` and `offset=51` sit on
    `initial: ([...(name || tok.ActorID || "?")][0] ?? "?").toUpperCase(),`.
  - Task 7 rewrites that line, so both rows leave with it (D20). The new line
    must hold no fallback a test cannot reach.
- **E8. The vocabulary gate reads Go test files.**
  - `.semgrep/vocabulary.yml` scans `internal/` and `cmd/`, tests included,
    and refuses `\bhp\b` among other words.
  - New Go tests name resources `vigor` or `pool`, as the existing ones do.
  - The client and the e2e suite are outside its scope.
- **E9. Structured outcomes change two recorded streams, and no ruleset
  golden (P10).**
  - `scenarios/goldens/adventure-night`, `door-watch` and `toy-brawl` hold an
    `abilityUsed` in `stream.json`.
  - Adventure-night's is `rally`, an ability with no resolution, so it gains
    no outcome (D12) and its stream does not change. The streams of
    `door-watch` and `toy-brawl` do.
  - `TestScenarioGoldenStreamsHaveNotDrifted` masks the keys in
    `diceFields` (`results`, `total`, `outcomeSummary`, `delta`, `newValue`,
    `outcome`), so a per-target field named `total` is masked like
    `Roll.total`.
  - `internal/rules/conformance` compares `AbilityUsed` field by field, so
    `rulesets/*/goldens` stay untouched.
- **E10. Two `perceive` tests this plan edits cite withdrawn rows.**
  `TestAPlayerIsIntroducedToATokenlessActorItIsGranted` cites VTT-241 and
  `TestAPlayersOwnTokenlessCharacterIsHeardOf` cites VTT-246. Both were
  withdrawn on 2026-10-17. Task 5 re-points them while editing them.
- **E11. SPEC-012 and SPEC-015 stay true.**
  - The sight gate sits in `handleCommand` before dispatch, so SPEC-012's
    "Authorization ran in handleCommand before either handler" holds.
  - The gate calls `perceive.SeesActor` itself, so SPEC-015's "`canSee` …
    the player's move gate in `handleCommand` is its one production caller"
    holds.
  - SPEC-015 lists `seat.go`'s functions and gains none.
- **E12. Rows false as worded are the four the ticket names: VTT-247,
  VTT-248, VTT-313 and VTT-440.** A search of `docs/requirements.md` for
  introduction, resources, sight, titles, initials, labels and Story wording
  found no other.
  - VTT-243, VTT-244, VTT-252, VTT-263, VTT-393, VTT-404, VTT-436, VTT-439,
    VTT-441 and VTT-447 stay true as worded. Their tests change wording or
    fixtures and keep their claims.
  - Citations to re-point, counted by grep at `1e24277`:

    | Id | Test lines | Source pointers |
    |---|---|---|
    | VTT-247 | 3, in 2 files | none |
    | VTT-248 | 6, in 3 files | none |
    | VTT-313 | 6, in 5 files | 2, both in `projection.go` (`recall`'s comment, `sameIncarnation`'s doc) |
    | VTT-440 | 1 | none |

## Measurements this plan stands on

All at `1e24277`, outside the repository.

| # | Probe | Result |
|---|---|---|
| P1 | Untouched worktree: the 16 tests of P2, and `env -u CLAUDECODE bun test client/test contract` | the 16 pass; 1403 pass, 0 fail |
| P2 | Server prototype, whole `go test ./...`. Contract fields and arms; engine; `ToEvent` and `commandRoles`; the projection as D7-D10; the sight gate as D11; `describe_token` in the tool manifest and both `tools.json` | 16 top-level tests red: 8 in `internal/perceive`, 5 in `internal/campaign`, 3 in `internal/gateway`. Every other package ok, `cmd/vtt` and `internal/harness` included. Listed under "The existing tests that change". With the projection re-introducing a reshaped actor at once instead (option A), 17: `TestAKindChangedOutOfSightIsCorrectedOnSight` too |
| P2b | P2 before the tool manifest and the `internal/mcp` lists knew the command | 14 MCP tests in `cmd/vtt`, `TestBuildDispatchAcceptsExactAgreement` and `TestListToolsReturnsEveryCommandAndReadTool` in `internal/mcp`, and `TestToolsMatchGolden` also red: `oneof fields with no tools.json entry [describe_token]` |
| P3 | Scratch `perceive` tests over the P2 prototype, `Eyes{Shoulder}` | a creature is held as id, module, kind and conditions; its token as name and notes. Twelve events in sight send no `ActorAdded` or `ActorRemoved`. Kind changes in sight: E1's frames, folded. A GM-only rewrite sends nothing; a name rewrite in sight sends one `TokenDescribed` without GM fields; an unseen rewrite nothing, and the token comes back with its present name |
| P4 | The TS `Token` fields required, `bunx tsc --noEmit -p client` | 34 errors in 8 test files: 33 bare token literals, and one where `log.map(describeEvent)` meets the new signature |
| P5 | A light client prototype: naming, `describe`, disc, target labels, the fold, and a `COMMAND_SURFACE` row written `not-user-issued` before the owner's answer | with the disc's `title` dropped: 45 of 1403 red. With `title` set to the naming: 28 red, listed below. The 17 extra find the disc by `[title="Lera"]` |
| P6 | Comment shares by `check-comments.py`'s own `measure`, HEAD and the prototype | the budget table below |
| P7 | Item 1's test on HEAD, `-count=100` | failed at the rename's `t.Fatal`: `database is locked (5) (SQLITE_BUSY)` |
| P8 | Adjudication rows whose `code:` line is in a file this plan edits | the rows that pin the plan (Constraints) |
| P9 | `command-surface.test.ts` with the regenerated contract | "every command the contract defines has a declared human surface" and "every command with a human surface has a builder in commands.ts" red on `describeToken` |
| P10 | `AbilityUsed` in the corpora | 3 scenario streams hold one; the conformance runner compares fields by name |

Not measured:
- `task e2e`, the mutation runs and `task check`;
- the loaders, `eventgen`, `Resolve`'s outcomes and the console control, none
  prototyped;
- the sight gate's own new tests. P2 shows only that no existing test goes red
  under it.

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no ceiling, floor or exemption rises.
    - `tools/comment-ceilings.txt` rows only fall, by
      `check-comments.py --write-ledger`, when a file ends more than 1.0 under
      its ceiling.
    - Neither mutation adjudication file gains a row for code this plan
      writes.
  - 3: every contract change is additive:
    - four fields on `PlaceToken` and `TokenPlaced`;
    - one new arm in each oneof;
    - one new message and one new field on `AbilityUsed`.

    `check:breaking` only reports pre-release, so nothing but this rule
    guards it.
  - 4: no second event-application loop. `TokenDescribed` gets an arm in
    `engine.Apply` and its mirror in `client/src/fold.ts`, nothing else.
  - 5: no game-system word in platform code. New Go tests name resources
    `vigor` or `pool` (E8). The new fields are `name`, `notes`, `gm_name`,
    `gm_notes`, `outcomes`, `result` and `total`.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment is a warning, a pointer or an exported symbol's one-line
    doc, with no date, hash or task beside the code.
- **The comment budget (P6).** Shares are compared unrounded. Run
  `python3 tools/check-comments.py main` after staging, since a line moved
  between files shows only then. Every file this plan edits sits within 0.1
  of its ceiling at HEAD, so an added comment line is paid for by added code
  lines. By the measured headroom that is from under 1 to about 11 code lines
  per comment line, depending on the file: `state.ts` needs under one, and `convert.go`
  about ten.

  | File | HEAD | Ceiling | Prototype |
  |---|---|---|---|
  | `internal/perceive/projection.go` | 149 of 822, 18.13% | 18.2 | 155 of 876, 17.69% |
  | `internal/gateway/server.go` | 200 of 908, 22.03% | 22.1 | 200 of 919, 21.76% |
  | `internal/gateway/convert.go` | 10 of 117, 8.55% | 8.6 | 10 of 129, 7.75% |
  | `internal/gateway/authz.go` | 32 of 242, 13.22% | 13.3 | 32 of 245, 13.06% |
  | `internal/engine/apply.go` | 221 of 665, 33.23% | 33.3 | 221 of 675, 32.74% |
  | `internal/engine/state.go` | 90 of 210, 42.86% | 42.9 | 90 of 214, 42.06% |
  | `internal/rules/resolve.go` | 247 of 651, 37.94% | 38.0 | 249 of 656, 37.96% |
  | `client/src/fold.ts` | 218 of 651, 33.49% | 33.5 | 218 of 666, 32.73% |
  | `client/src/state.ts` | 147 of 236, 62.29% | 62.3 | 147 of 240, 61.25% |
  | `client/src/view/spectator.ts` | 258 of 571, 45.18% | 45.2 | 258 of 595, 43.36% |
  | `client/src/view/grid.ts` | 58 of 127, 45.67% | 45.7 | 58 of 129, 44.96% |
  | `client/src/view/player.ts` | 54 of 233, 23.18% | 23.2 | 54 of 234, 23.08% |
  | `client/src/view/dm.ts` | 266 of 654, 40.67% | 40.7 | not prototyped |
  | `client/src/commands.ts` | 164 of 380, 43.16% | 43.2 | not prototyped |
  | `client/src/app.ts` | 262 of 530, 49.43% | 49.5 | not prototyped |

  `state.ts` and `spectator.ts` finished more than 1.0 under in the
  prototype. Each such row is lowered by `--write-ledger` in its task's
  commit. `client/src/view/naming.ts` is new and gets its row from
  `--write-ledger`.
- **The adjudications (P8).** SPEC-020 keys a row by the code it sits on.
  - Keep byte-identical:
    - the four `projection.go` rows' lines: `objectInSight`'s two footprint
      loops, `sortedSceneIDsUnion`'s `make`, and `recall`'s
      `if lit := trueKeys(sc.Visible); len(lit) > 0 {`;
    - `resolve.go`'s six (`if len(ability.Effects) > 0 {` among them);
    - `apply.go`'s four;
    - `authz.go`'s one;
    - `fold.ts`'s, `case "attackRolled":` and its `default:` among them;
    - `dm.ts`'s four, `input`'s `if (cls) i.className = cls;` and
      `target.value = draft[field] ?? "";` among them;
    - `grid.ts`'s five on `tokensOnScene`'s
      `return discs.sort((a, b) => (a.tokenId < b.tokenId ? -1 : a.tokenId > b.tokenId ? 1 : 0));`;
    - `spectator.ts`'s two in `renderPerch`, on its two sort comparisons;
    - `internal/adventure/load.go`'s one, on
      `attrOrDefSet := make(map[string]bool, len(rs.Attributes)+len(rs.Defenses))`.
  - The two `grid.ts` rows on the initial line leave with it (E7, D20).
    First apply each mutant by hand to the old line and see it survive, since
    deleting a row that still holds a live mutant loses it silently. Then
    delete the rows in the same commit as the line.
  - Run both adjudication self-tests before the gate.
- **Requirement ids** come from `requirement-id` after sign-off, never by hand
  (SPEC-008).
  - A test or spec that cites an id needs the row first.
  - An e2e test named as evidence carries the id in a `// VTT-NNN` line.
  - A renamed test's register cells change in the same commit, or
    `check:requirements-chain` refuses the stale name.
  - No test cites a withdrawn id once its successor exists. The last task to
    touch a file re-points it (E12).
- **Tier 1 runs at pre-commit:**
  - `go test` tier 1;
  - `bun test client/test contract contract-spike`;
  - `task client:typecheck`;
  - lint, vet, arch, vocabulary, secrets.
- **bun is quiet under `CLAUDECODE`.** Run `env -u CLAUDECODE bun test …`.
- **The bundle.**
  - Each commit that changes `contract/gen/ts` or `client/src` carries
    `cmd/vtt/webdist`, rebuilt by `task build:client` (E6). The commit hook
    does not catch a stale bundle.
  - `check:drift` passes only after the commit. Pre-push runs it.
- **Property and pump-order tests.** Task 3 reshuffles every `eventgen` seed
  (D18). Run `internal/campaign`'s property and oracle tests with
  `-count=20`, and the `perceive` and gateway tests a task edits with
  `-count=3 -cpu 1,2`, before each commit.
- **Pushing.** The pre-push hook takes about three minutes. Check the load
  with `os.getloadavg()` first, and never bypass the hook.
- **`review-record` refuses untracked files.** Commit the ticket and this plan
  in Task 0.
- **Mechanics.**
  - A new test goes after a closing brace.
  - Stage and commit in separate calls.
  - Undo a break with its inverse edit and check with grep, never
    `git checkout --`.
  - Hash the tree before and after any agent that reads it.

## Rule 9: how MapTool does this

Checked read-only in `~/dev/RPTool/maptool`, at `f4b7fef6c`.

- **What MapTool does.**
  - `Token` carries `name` (default `""`), `notes`, `notesType`, `gmName`,
    `gmNotes` and `gmNotesType`, besides `label`.
  - `PointerTool.createHoverNote` builds the note as follows:
    - the name, followed for the GM by ` (gmName)` when the GM name is set
      and differs;
    - then the notes;
    - then, for the GM, a "GM Notes" heading and the GM notes.

    It shows the note for a marker token on a left-button release.
  - A token's stat sheet on hover drops `gmOnly` properties for a player.
  - `ZoneRenderer` labels a token by its name, and in the GM view appends
    ` (gmName)`.
  - `EditTokenDialog.commit` refuses an empty name
    (`msg.error.emptyTokenName`) and writes the notes and GM notes together.
    The whole token then goes to the server (`putToken`).
- **What we borrow.**
  - The four fields and their split (B-5).
  - The card's order: name, notes, then the GM's lines (D15).
  - A rewrite that states every field at once, as the dialog commits (D2).
- **What we do not borrow.**
  - The distribution, which rule 9 always excludes. Every MapTool client
    receives `gmName` and `gmNotes` and hides them on the client. Here no
    character log is sent them (D7, D10).
  - The empty-name refusal. An unnamed creature is a fixed wording here
    (D14), and a placement need not name its token.
  - "name (gmName)" as the DM's label. The ticket rules that the DM's window
    names a creature by its GM name, else its actor's name (item 10). The DM's
    card shows both names (D15).
  - The marker-only, click-shown note. Here every token has a card, on hover
    and on right-click (K-1).

## Design questions for the owner

None open. The two questions the first verification raised are answered:
"Ja, en kontroll i konsolen" (D16), and the outcome line (D12, D13).

## Decisions

1. **D1. The contract's names and numbers.**
   - `PlaceToken` and `TokenPlaced` gain `string name = 5`, `notes = 6`,
     `gm_name = 7`, `gm_notes = 8`.
   - `DescribeToken`, at `ClientCommand` oneof field 34, carries
     `token_id = 1`, `name = 2`, `notes = 3`, `gm_name = 4`, `gm_notes = 5`.
     `TokenDescribed`, at `Envelope` oneof field 34, carries the same fields.
   - `AbilityUsed` gains
     `message Outcome { string target_id = 1; string result = 2; int32 total = 3; }`
     and `repeated Outcome outcomes = 6`.
   - Forced by:
     - rule 3: commands imperative, events past tense, additive only;
     - 34 being the next free arm in both oneofs, after the 33 the removals
       took. `ClientCommand`'s holes at 11 and 16 and `Envelope`'s at 17 are
       retired numbers, and a reused
       number would collide old logs onto a new type (ADR-007).
   - `gm_name` and `gm_notes` on both events carry the warning the other
     withheld fields carry: never forward them to a player or spectator
     (SPEC-016).
   - `Outcome.total`'s name is also forced by E9: `maskDice` masks `total`.
2. **D2. The rewrite replaces all four fields.**
   - An empty string in `DescribeToken` clears that field.
   - The tool keeps its derived `required` list, all five fields. Its
     description says to send the present value of any field kept.
   - Forced by:
     - protojson's omission of `""`: an absent field and an emptied one are
       the same bytes, so a partial update cannot tell "keep" from "clear"
       (the reasoning `JoinDoor`'s comment gives);
     - MapTool's dialog, which commits the fields together (rule 9);
     - the event then being the token's whole description, so neither fold
       nor `ToEvent` needs the snapshot's token.
3. **D3. `place_token`'s tool keeps the four fields optional.**
   - A `requiredOverride` of `tokenId`, `sceneId`, `actorId` and `position`
     keeps today's required list, with a `fieldDocs` line per new field.
   - Forced by the fabrication trap `add_actor`'s override answers: a field
     that is non-optional on the wire is otherwise required in the schema,
     and an LLM fabricates a value.
4. **D4. The four fields are unbounded in bytes (ticket item 2, SPEC-018).**
   - No fold or loader checks their length. Neither does
     `TokenDescribed.token_id`, which `engine.Apply` refuses unless it names a
     folded token.
   - SPEC-018's second table gains rows for:
     - the four fields on both events, held by the WebSocket frame for
       `place_token` and `describe_token` and by nothing in a file;
     - `TokenDescribed.token_id`, held by the lookup;
     - `AbilityUsed.Outcome.target_id`, held by `Resolve`'s lookup;
     - `Outcome.result`, held by `internal/rules/load.go`'s branch-label
       bound.
5. **D5. `engine.Token` gains `Name`, `Notes`, `GMName` and `GMNotes`, each
   `json:",omitempty"`.**
   - `foldToDumpJSON` emits each only when non-empty, after `Y`, in that
     order.
   - Forced by `writeDump` marshalling `engine.State`: without `omitempty`,
     every token in every `scenarios/goldens/*/state.json` gains four keys,
     and `fold-parity.test.ts` reads them all. `Explored` and `Visible` are
     the precedent.
6. **D6. The TS `Token` marks the four fields optional.**
   - The fold sets all four on every `tokenPlaced` and `tokenDescribed`,
     mirroring Go's zero values. Readers default `?? ""`.
   - Forced by P4: required fields break 33 bare literals in 8 test files for
     no behaviour, the case `Scene`'s optional fields already record.
7. **D7. What a character log is sent of an actor is one function,
   `shownAs`.**
   - A party member, whole.
   - Any other actor, `shownWith(a, false)`: its id, module id, kind,
     `controller_id` and `controller_ids`.
   - `introduce` clones `shownAs(a)` and then clears its controllers as
     today. `correct` and `Resume` compare against `shownAs` of the world's
     actor.
   - Forced by item 4. The comparisons must use the same function, or a held
     creature differs from the world at every event and is re-introduced each
     time (K14).
8. **D8. `recall` marks a held actor `gone` or `reshaped`.**
   - **`gone`** when it differs from the world's actor shown under the held
     actor's own kind, `shownWith(w, engine.IsPartyMember(held))`: the world
     added it again as another.
   - **`reshaped`**, a new map, when it is not gone and differs from
     `shownAs(w)`: a kind change altered what the log may hold.
   - `transitions` withdraws an actor that is `gone`, or `reshaped` and in
     `now.sees`. A new `hideTokensOf` first sends a `TokenHidden` for each of
     its tokens the viewer's board holds, sorted, and forgets them, so the
     token walk places them again (E1).
   - `correct`'s re-introduction path hides tokens the same way.
   - `recall` records each held token's actor (`tokenActor`) and text
     (`tokenText`) from the viewer's fold.
   - Forced by:
     - E1: both folds refuse a bare `ActorRemoved` while a token of the actor
       stands;
     - E2: a reshaping change unseen waits for sight (B-1, VTT-247).
   - So a party member made non-party while unseen keeps its name in that log
     until it is seen again. A creature made a party member unseen is held
     stripped until seen, while `WatchableChanged` names it to every
     connection that may watch it (SPEC-015).
9. **D9. Party membership after the event is on the look.**
   - `sightView` gains `party`, filled where `look` already asks
     `engine.IsPartyMember`.
   - `classify`'s `ResourceChanged` arm forwards only when `now.party` holds
     the actor and the viewer saw it.
   - Forced by E4 and item 5.
10. **D10. `TokenDescribed` gets an arm in `classify`.**
    - It forwards when:
      - the viewer's board held the token before the event;
      - the look holds it after;
      - and its name or notes differ from `tokenText`.
    - `forwardable` clears its `gm_name` and `gm_notes`.
    - `transitions`' `TokenPlaced` carries the world token's `Name` and
      `Notes`, never its GM fields.
    - Forced by item 6 and by `TestEveryEnvelopePayloadArmHasAnExplicitRuling`.
11. **D11. The sight gate for a player's `use_ability` sits in
    `handleCommand`, right after the move gate.**
    - For each target, in order, it asks `perceive.SeesActor(Eyes{Shoulder:
      sub.judgedBy(p, st.Actors)}, st, tid)`.
    - On the first `false` it refuses with `rules.UnknownTarget(tid).Error()`.
    - `SeesActor` is new in `projection.go`: `New(eyes, nil).look(st).sees`,
      recording nothing, as `CanSee` does.
    - `rules.UnknownTarget` is new and exported, and `Resolve`'s target loop
      calls it, so the refusal's wording has one home.
    - Forced by item 8's "exactly the one an id naming no actor gets":
      - The gate refuses every target it does not see, an absent id
        included, before `Resolve` runs. So an unseen id and an absent one get
        the same answer whatever else the command carries.
      - A gate that refused only existing unseen actors would answer a
        command with a bogus ability id differently for the two ids. That is
        an oracle, and K19 holds it shut.
    - The DM and the agent are not gated (item 8).
    - Consequences:
      - E11.
      - A self-cast by a party member other than the one judging the moves,
        and out of its sight, is refused, as a move of that member's token
        would be. Both are judged by `judgedBy`.
      - A player who controls only a non-party creature has no eyes, so every
        target is refused.
12. **D12. The window reads an ability's result and roll from structured
    `outcomes`, not from `outcome_summary`.**
    - `Resolve` appends `{target_id: tid, result: word, total: total32}` for
      each target of an ability with a resolution, beside the summary part it
      already builds from the same variables. An ability with none appends
      nothing.
    - Weighed:
      - **Parse `outcome_summary`.** No contract change and no golden. But it
        is a second home for `Resolve`'s format string. Display names are
        ruleset free text that no loader bounds or escapes (SPEC-018), so
        `"; "` and `" on "` inside a name make the split ambiguous. A parse
        that misses has only the raw text to fall back on, and the raw text
        carries the defense and the target's id.
      - **Structured fields (chosen).** One home: `Resolve` writes both the
        summary and the outcomes. The defense is not in the field, so no
        client path can show it. The cost is two `Resolve` tests and two
        re-recorded streams (E9), with ruleset goldens untouched.
    - `outcome_summary` stays whole on the wire ("utfallsraden står").
13. **D13. `describe(e, names)` words every payload case.** The switch over
    `p.case` ends in an exhaustive `never` check, so a new arm fails
    typecheck. `names` gives `token(id)`, `actor(id)` and `ability(id)`.

    | Case | Line |
    |---|---|
    | `actorAdded`, party member | `<actor> is in the party` |
    | `actorAdded`, any other | `<actor> comes into sight` |
    | `tokenPlaced` | `<token> is at <x>,<y>` |
    | `tokenMoved` | `<token> moved to <x>,<y>`, with `because` for a reason, as today |
    | `tokenRemoved` | `<token> taken off the board` |
    | `tokenHidden` | `<token> went out of sight` |
    | `tokenDescribed` | `<token> described` |
    | `actorRemoved` | `<actor> left` |
    | `conditionApplied`, `conditionRemoved` | `<actor> gained <condition id>`, `<actor> lost <condition id>` |
    | `resourceChanged` | `<actor> <resource> → <value>` |
    | `abilityUsed` | below |
    | `actorControlGranted`, `actorControlRevoked` | `control of <actor> given`, `control of <actor> taken back` |
    | `doorOpened`, `doorClosed` | `a door opened at <x>,<y>`, `a door closed at <x>,<y>` |
    | `sceneSeen` | `the view changed` |
    | `narrationAdded` | `<as>: <text>`, or `<text>` with no speaker |
    | the seven others | as today: `sessionStarted`, `sessionEnded`, `sceneCreated`, `noteUpserted`, `noteDeleted`, `adventureLoaded`, `attackRolled` |
    | no payload | `something happened` |

    - **The ability line.** `<actor> used <ability>`, then, for each outcome
      in order, ` on <actor(target)>: <result>, rolled <total>`, joined by
      `;`. With no outcomes it reads ` on <targets>`, joined by `, `.
      - Example: `Human Fighter used Longsword Strike on a hunched goblin:
        hit, rolled 17`.
      - It never reads `outcome_summary` (item 11).
      - It shows the resolution roll alone. The owner's "bara longsword
        strike hit (dice: 17)" names the roll the branch compares, and
        damage dice tell hp ("Inget hp").
    - **Abilities and conditions.** `<ability>` is the display name from the
      `/api/ruleset` the page already loads for every role, else the ability
      id. A condition keeps its id, as today: item 10's "never an id" binds
      actor and token ids, and item 11's test checks those.
14. **D14. One naming module, `client/src/view/naming.ts`, serves the
    initial, the title, the card, the target buttons and the Story.**
    - **A window that is not a DM's or an agent's:** the token's `Name`, else
      its actor's name when the actor is a party member, else `UNNAMED`.
    - **A DM's or an agent's window:** the token's `GMName`, else its actor's
      name, else `UNNAMED`. The agent counts as the DM, as for the console
      and the Events panel (SPEC-022).
    - **An actor** is named by the first of its tokens in id order whose
      field is set, else as above.
    - **A token or actor the fold no longer holds** (after `tokenHidden`,
      `tokenRemoved` or `actorRemoved`) is named from the newest
      `tokenPlaced`, `tokenDescribed` or `actorAdded` entry of the log the
      window holds. So a Story line keeps naming what the window was told.
    - **The fixed wording:** `UNNAMED = "a creature"`. The initial is the
      naming's first character uppercased, and `?` for `UNNAMED`. The disc
      keeps a native `title`, set to the naming.
    - Forced by:
      - item 10;
      - P5: dropping `title` turns 17 more tests red. Those tests find a disc
        by its title and keep their meaning when the title is the naming;
      - E7: the initial's line drops its unreachable fallbacks.
    - `renderPlayerPanel` picks the chain by `me.role`. The panel is also
      drawn for the DM and the agent.
15. **D15. The card is part of each disc's DOM.**
    - `.token-card` holds, each only when non-empty:
      - `.card-name`, the naming;
      - in a DM's or an agent's window, `.card-shown`: "players see: " and
        the token's name or `UNNAMED`;
      - `.card-notes`;
      - in a DM's or an agent's window, `.card-gm-notes`.
    - CSS shows it on `.token:hover` and `.token.pinned`, with
      `pointer-events: none`, so a left click still reaches the board's click
      handler and moves as today (A-5).
    - A `contextmenu` on a disc calls `preventDefault` and hands
      `onCardPin(tokenId)` to the page. A second right-click on the pinned
      disc unpins it.
    - `app.ts` keeps the pinned id across paints, so an event's repaint keeps
      the card. `ViewExtras` gains `dm`, `abilities`, `pinnedCard` and
      `onCardPin`.
    - Forced by:
      - item 9;
      - H1: hover in CSS survives a repaint, which a `mouseenter` flag does
        not;
      - the DM's card showing both names, which MapTool's "name (gmName)"
        does.
16. **D16. The console's rewrite control.**
    - A "Describe tokens" group in `renderDMConsole` draws one row per token
      of the world, sorted by id. Each row has:
      - a label, the token's id, as the place-token form names tokens. The
        naming module arrives only in Task 7, and the console is the DM's;
      - inputs `describe-name:<id>` and `describe-gm-name:<id>`;
      - textareas `describe-notes:<id>` and `describe-gm-notes:<id>`;
      - a "Describe" button with `data-action="describe-token"`.
    - Each field shows its draft. From Task 3, which gives the TS `Token` its
      four fields (D6), a field with no draft shows the token's present value.
      That is set after `input` returns, so `input`'s adjudicated lines stay
      as they are.
    - The button sends `describeToken(id, name, notes, gmName, gmNotes)` as
      typed, and clears that row's four drafts.
    - `COMMAND_SURFACE` declares `describeToken: { surface: "dm-console",
      action: "describe-token" }`.
    - The place-token form keeps its fields (the ticket).
    - Forced by "Ja, en kontroll i konsolen" and E5.
17. **D17. The ability's display name reaches the view from
    `/api/ruleset`.** `app.ts` already keeps `rs.abilities` for every role.
    - Forced by item 11's "shows the ability".
    - Rejected: an `ability_name` field on `AbilityUsed`. It would copy what
      the read surface already serves.
18. **D18. `eventgen` names its placements and rewrites them.**
    - `placeToken` sets a name and a GM name derived from the token id, with
      no rng draw.
    - A `describeToken` action takes the band `0.48 ≤ r < 0.52` from
      `moveToken`'s, while a token exists, with one `Intn`.
    - Forced by the ticket's "for the folds' parity": the oracle and property
      tests then fold the new event over random logs.
    - The new band reshuffles every seed's log, hence the `-count=20` runs in
      Constraints.
19. **D19. `adventures/goblin-ambush` names its two creatures.**
    - `tok-cutter`: name "a hunched goblin", GM name "Goblin Cutter".
    - `tok-archer`: name "a goblin with a bow", GM name "Goblin Archer",
      notes "It keeps to the rocks above the trail."
    - `tok-fighter` stays as it is.
    - No player-facing text contains an actor name, which W1 relies on.
    - `goldens/compiled-batch.json` is edited by hand and then verified by the
      adventure conformance runner. The comment in
      `internal/adventure/conformance/dump_test.go` states that discipline:
      derive a golden by hand first.
20. **D20. The initial's line is rewritten as
    `initial: name === UNNAMED ? "?" : [...name][0]!.toUpperCase()`.**
    - The naming never returns `""`, so no fallback literal remains.
    - The two adjudication rows on the old line leave in Task 7's commit (E7).
21. **D21. The existing tests that change keep what they were written to hold
    ("The existing tests that change", below).** A test whose name becomes
    false is renamed, and its register cells change with it.
22. **D22. Withdrawals at Task 0, re-pointing per task.**
    - VTT-247, VTT-248, VTT-313 and VTT-440 are marked withdrawn when their
      successors are allocated.
    - Each citation moves to its successor in the task that edits its file,
      `projection.go`'s two VTT-313 pointers in Task 5.
    - After Task 7 nothing under `internal/`, `cmd/`, `client/` or `tools/`
      cites a withdrawn id. SPEC-016 and SPEC-021 still cite VTT-247,
      VTT-248 and VTT-313 until Task 10 rewrites them.

## Requirements

Allocated by `requirement-id` at Task 0, after sign-off, and written
`**OPEN — no test yet**`.
- R1 to R16 were allocated first, as VTT-462 to VTT-477, in that order.
- The reading review of Task 0 found six of them joining separate rules. The
  rules split from them are R17 to R23, seven, since R2 gave two; they were
  allocated next, as VTT-478 to VTT-484, in that order.
- Nine of the first sixteen were reworded before their first commit, while
  nothing cited them.

The sort ran over the ticket's eleven rules and its four withdrawals. Each row
below gives the observation that would go red if it broke.

- **Withdrawn**, each marked
  `WITHDRAWN 2026-10-24: <text>. <why>. Succeeded by VTT-NNN.`, with evidence
  `**READING — verify-ticket check 5 of 2026-10-24; withdrawn, held by nothing**`:
  - VTT-247, false once a creature's status in a character log holds no
    resources. Succeeded by R10.
  - VTT-248, false for the same reason. Succeeded by R8 and R4.
  - VTT-313, false once a log holds of a creature only its module id among
    `ActorAdded`'s fields. Succeeded by R9.
  - VTT-440, false once a target button is labelled by the window's naming:
    the token's name first, and `a creature` for an unnamed creature.
    Succeeded by R14.
- **R1.** "A character log is sent no token's GM name or GM notes."
  - Evidence: N3, N4 and Gw5.
  - Red under K12 and K17.
- **R17.** "A token in a character log's sight arrives in it with its name and
  notes."
  - Evidence: N3.
  - Red under K12b.
- **R2.** "A `describe_token` from a player or a spectator is refused by
  authorization."
  - Evidence: A1.
  - Red under K6.
- **R18.** "A `describe_token` from the DM or the agent replaces its token's
  name, notes, GM name and GM notes, each with the value it carries, an empty
  one included."
  - Evidence: En2, Fo2 and A2b.
  - Red under K4b and K5b.
- **R19.** "A `describe_token` naming no token is refused and appends nothing."
  - Evidence: En3, Fo3 and A2a.
  - Red under K4c.
- **R3.** "The DM console offers, for each token the world holds, a control
  that sends `describe_token` with the four fields as typed."
  - Evidence: Cc1 and Cc2.
  - Red under Kc1 and Kc2.
- **R4.** "A character log is introduced to an actor that is not a party
  member with its id, module id and kind, and its controllers and conditions
  behind it, and with no name, attributes, resources or module data."
  - Evidence: N1 and Gw5.
  - Red under K10.
- **R5.** "A character log is sent no `ResourceChanged` of an actor that is
  not a party member after the event."
  - Evidence: N2.
  - Red under K11.
- **R6.** "A character log is sent a token's rewrite only when the token was on
  its board before the event and is in its sight after it, and the name or
  notes differ from what the log holds."
  - Evidence: N4.
  - Red under K13.
- **R7.** "An actor in a character log's sight whose kind change alters what
  the log may be sent of it is withdrawn and introduced afresh at that event,
  its tokens on the log's board hidden before and placed again after."
  - Evidence: N6 and N7.
  - Red under K15.
- **R20.** "An actor whose kind changed while out of a character log's sight,
  so that what the log may be sent of it differs from what the log holds, is
  withdrawn and introduced afresh when it next comes into sight."
  - Evidence: N8, both directions, and
    `TestAKindChangedOutOfSightIsCorrectedOnSight`.
  - Red under K16 and K16b.
  - Until then the log holds it as R10 says.
- **R8 (succeeds VTT-248).** "An actor that comes into a viewer's sight, by an
  event or by a perch, is brought in its fold to its present conditions,
  controllers and kind, and a party member to its present resources too."
  - Evidence: VTT-248's tests, re-pointed.
  - Red under the breaks those tests already catch.
- **R9 (succeeds VTT-313).** "An actor the world adds again after a removal is
  introduced afresh to a viewer that sees it when the viewer no longer holds
  the id, or holds one that differs from what the viewer may be sent of the
  new actor, and is corrected otherwise. For a party member that is its name,
  module id, attributes, module data, resource names or maxima; for any other
  actor, its module id."
  - Evidence: VTT-313's tests as Task 5 edits them, and N5.
  - Red under K14.
- **R10 (succeeds VTT-247).** "A player's or spectator's fold holds each actor
  it does not see at the conditions, controllers and kind it had after the
  last event at which the viewer saw it, or at its introduction, and, when it
  was a party member then, at its resources then too."
  - Evidence: VTT-247's tests, re-pointed.
  - Red under the breaks those tests already catch, and under K14.
- **R11.** "A player's `use_ability` naming a target that the eyes judging the
  player's moves do not see is refused with the answer a target id naming no
  actor gets, and appends nothing."
  - Evidence: Gw1, Gw2 and Gw3.
  - Red under K18, K19 and K22.
- **R21.** "A DM's or an agent's `use_ability` is not judged by sight."
  - Evidence: Gw4.
  - Red under K20.
- **R12.** "Each of an `AbilityUsed`'s `outcomes`, one per target of an ability
  with a resolution, carries the target, the branch it resolved and the
  resolution roll's total, and nothing of what the roll was compared with."
  - Evidence: Ru1 and Ru2.
  - Red under K21.
  - `outcome_summary` still carries the compared total ("utfallsraden
    står"), and so do the `rolls` beside the outcomes when the compared
    expression rolls dice; every shipped one is a bare `@target.…` reference.
    R16 keeps the window from showing it.
- **R13.** "A browser window shows, for a token on its board, a card with the
  token's name and notes on hover and while a right-click pins it."
  - Evidence: C4, C5 and W1.
  - Red under Kc6b and Kc7. Kc7b checks C5's `preventDefault`, which holds no
    row.
- **R22.** "A token's card shows its GM name and GM notes in a DM's or an
  agent's browser window, and in no other."
  - Evidence: C4.
  - Red under Kc6 and Kc6c.
- **R14 (succeeds VTT-440).** "In a browser window that is not a DM's or an
  agent's, a token's initial, title and card, the player panel's target
  buttons and the Story name an actor or a token by the token's name, else a
  party member's actor name, else `a creature`, and never by an id."
  - Evidence: C1, C3, C9, C10 and the edited `qa-ability-target` tests.
  - Red under Kc4.
  - Out of its scope, as today: the perch's switch buttons, which name a
    party member by its id when its name is empty (VTT-404), and the player
    panel's own chips.
- **R23.** "In a DM's or an agent's browser window, a token's initial, title
  and card, the player panel's target buttons and the Story name an actor or
  a token by the token's GM name, else the actor's name, else `a creature`."
  - Evidence: C2.
  - Red under Kc5.
- **R15.** "No Story line is a payload's name."
  - Evidence: C7 and both edited `table.spec.ts` tests.
  - Red under Kc8.
- **R16.** "An ability's Story line names the ability, each target as the
  window names it, each outcome's result and roll total, and never the total
  the roll was compared with."
  - Evidence: C8 and W1.
  - Red under Kc9.
- **The sort refused:**
  - "A left click still moves" (A-5): VTT-408 and VTT-460 already hold it.
    C6 joins their evidence.
  - The unstable test puts no rule on the system (the ticket).
  - "The tools describe the four fields": `TestToolsMatchGolden`,
    `check:drift` and T1-T3 hold the text, as item 12 of
    `docs/superpowers/plans/2026-10-23-a-player-without-a-character-watches-as-a-spectator.md`
    did.
  - The conditions a log is sent of a creature: VTT-243 and VTT-249, as
    worded.
  - "No Story line in a window that is not a DM's carries an id": R14 covers
    the Story.
  - The console's prefill of a field with no draft: a convenience of D16,
    held by Cc4 and Kc3, and no rule.

| Ticket item | Observed by |
|---|---|
| 1 | `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody` at `-count=1000`, and the probe in Task 1's commit |
| 2 | Rt1, Rt2 (round trips), En1-En3 (engine), Fo1-Fo3 (fold), L1-L4 (loaders), the dump parity fixture Du |
| 3 | A1, A2a, A2b, En2, En3, Fo2, Fo3, Cc1-Cc4, Cb, the three surface guards green |
| 4 | N1, Gw5 |
| 5 | N2 |
| 6 | N3, N4 |
| 7 | N5, N6, N7, N8 |
| 8 | Gw1, Gw2, Gw3, Gw4 |
| 9 | C4, C5, C6, W1 |
| 10 | C1, C2, C3, C9, C10 |
| 11 | C7, C8, W1, and "the story feed and ticker record what happened" as edited |
| 12 | the conformance runner on the hand-edited golden; W1 |
| 13 | T1-T3, `TestToolsMatchGolden`, `check:drift` |
| 14 | the edits below, all green |

## The existing tests that change

From P2 and P5. Each keeps what it was written to hold.

**`internal/perceive` (8, Task 5):**
- `history_test.go`, `TestAnActorAddedAgainIsIntroducedAfreshOnlyWhenItIsAnother`:
  - "reuse-renamed" now expects only `TokenPlaced t-gob2`: a creature added
    again under another name is corrected (E3);
  - "reuse-module-id" expects `ActorAdded gob` with no name;
  - a new case renames a party member and expects the re-introduction with
    its new name.
- `history_test.go`, `TestACorrectionFollowsTheOrderTheViewerHolds`: the
  "controller-restatement" fixture keeps the kind change a grant can carry.
  - Gob is added with no kind, granted kindless until its last grant, and
    that last grant is `NON_PARTY`: a kind change that alters nothing the log
    holds.
  - It expects `ActorControlGranted p1 ACTOR_KIND_NON_PARTY` then
    `TokenPlaced t-gob3`, the viewer's order as before.
- `history_test.go`,
  `TestAnActorAddedAgainUnchangedWithItsConditionsReachesTheViewerAsTheWorldHoldsIt`:
  compares the viewer's gob with the world's gob as a log is sent it (id,
  module, kind). The conditions half is unchanged.
- `roster_test.go`, `TestAPlayerIsIntroducedToATokenlessActorItIsGranted`:
  the non-party familiar arrives with its controller and without its pool.
  The VTT-241 citation moves to R4.
- `roster_test.go`, `TestARemovalBatchProjectsToTheBytesBothFoldsRead`:
  - the second goblin is added with another module id;
  - `contract/testdata/removal_batch_projected_stream.json` is regenerated
    with no names in it;
  - the before-and-after assertions read module ids.
- `status_test.go`, `TestAPlayersOwnTokenlessCharacterIsHeardOf`: the
  familiar becomes a party member, which is what "character" names, so its
  pool change is heard as before. The VTT-246 citation moves to VTT-365.
- `status_test.go`, `TestAnIdReusedAfterAnUnseenRemovalFoldsAgain`: the
  second rogue gets another module id, and the held actor is compared with
  it as a log is sent it.
- `unseen_test.go`, `TestAnActorAddedAgainWithAnotherAttributeIsIntroducedAfresh`:
  the scout becomes a party member, whose attributes a log holds.
- `viewer_test.go`'s `envelope` helper gains a `TokenDescribed` arm. It panics
  on a payload it lacks.

**`internal/campaign` (5, Task 5).** A creature's resource change reaches no
log, and a renamed creature looks the same.
- `character_log_test.go`, `TestALogPausesWhileItsActorIsNotAPartyMember`,
  case "an id reused while paused": the reused goblin gets another module id,
  and "holds the old goblin" reads it.
- `qa_character_log_test.go`, `TestQALogACharacterLogContinuesAfterReopen`:
  the control's change on the seen `x` is a condition, which a log is still
  sent (B-1).
- `qa_pause_resume_test.go`, `TestQAPauseADemotedActorsLogReceivesNothing`,
  case "paused by a non-party grant": the probe change on the demoted `eye`
  is a condition.
- `qa_pause_resume_test.go`, `TestQAPauseAReusedIdIsWithdrawnBeforeItsNewActorIsIntroduced`:
  the new visitor gets another module id, and the fixture's check reads it.
- `qa_pause_resume_test.go`, `TestQAPauseStatusChangedDuringThePauseStillFoldsAfterTheResume`:
  the status changed during the pause is a condition on `visitor`.

**`internal/gateway` (3, Tasks 2 and 5):**
- `authz_test.go`, `TestEveryClientCommandHasRoleCells`: its `commandFor`
  helper gains `describe_token` (Task 2).
- `qa_issuer_test.go`, `TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause`
  and `TestQAIssACatchUpIsSentNoIssuerOrCause`: the scenario's targets
  become party members, so their changes still reach the player stripped of
  cause, and the "proves nothing" guard stays (Task 5).

**Mechanical (Task 2):**
- `internal/mcp/tools_internal_test.go`'s `theCommandTools` and
  `internal/mcp/tools_test.go`'s `wantCommandToolNames` gain
  `describe_token`.
- `contract/testdata/expected_tools.json` is edited by hand until
  `TestToolsMatchGolden` passes.
- The 14 MCP tests of `cmd/vtt` go green with the regenerated `tools.json`
  (P2b).

**Scenario streams (Task 6):** `scenarios/goldens/door-watch` and
`toy-brawl`, `stream.json` (E9). `adventure-night`'s does not change.
- Re-record each after reading `TestScenarioGoldenStreamsHaveNotDrifted`'s
  diff. The only change is an `outcomes` array on its `abilityUsed`.
- `state.json` is re-derived by hand: `AbilityUsed` folds to nothing, so it
  does not change.

**Client (27 in Task 7, and `fold-unit.test.ts`'s one in Task 3; P5):**

| File | Tests that change |
|---|---|
| `client/test/spectator-view.test.ts` (13) | renamed from "a disc is titled by name, falling back to the actor id" to "… falling back to `a creature`, never an id"; "a named actor's disc is titled by the NAME, not the id" (a party member's name); "a position-less placement reads as the origin, not as undefined" (D13's lines); "a target button sends the ability at that token's actor, then disarms" (found by the token's name); renamed from "a target with no actor name is labelled by its token id" to "… is labelled `a creature`, never by its id"; "a window not asked for the Events panel draws none, and its Story still shows the log"; "an actor with neither name nor id still reads as something" (`a creature`); "an envelope with no payload at all reads as a generic event" (`something happened`); "an event kind describe() does not handle degrades to its case name", which becomes C7; "describe renders a real label for every event kind it handles, not the fallback"; "each ticker row is its event's description alone, newest first"; "the board draws a disc per token, with its initial"; "the feed and the ticker show a move's reason" |
| `client/test/qa-move-reason.test.ts` (8) | the five `renderSpectator: the feed and the ticker > …` and three `describe: a move's label and its reason > …` tests: the token is named by the naming; the reason is unchanged |
| `client/test/qa-ability-target.test.ts` (3) | renamed from "qaAT: a named actor's target button is labelled by its name, not its token's id" to "… by the window's naming, the token's name first"; "qaAT: several targets in range — each button aims at its own token's actor" and "qaAT: two tokens of one actor give two buttons, each aimed at that actor": the fixtures name their tokens |
| `client/test/grid.test.ts` (2) | "a disc is labelled with the actor's initial" (the naming's initial); "a token whose actor is gone still draws, with an empty name" (`a creature`, initial `?`) |
| `client/test/app.test.ts` (1) | "a DM's and an agent's window draws a row in the Events panel for each entry it was sent, newest first": D13's lines |

Also changing:
- `client/test/command-surface.test.ts`: `COMMAND_SURFACE` gains the
  `describeToken` row (Task 2).
- `client/test/removal-batch-parity.test.ts`: its two goblin name assertions
  read module ids (Task 5, with the fixture).

**e2e (Task 7):** both are in `client/e2e/table.spec.ts`.
- "the story feed and ticker record what happened":
  `/^tok-lera placed at \d+,\d+$/` becomes `/^Lera is at \d+,\d+$/`.
- "a player's ability reaches the creature they aim it at": W1.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off on this plan.
  - Allocate R1-R16 with `requirement-id`, then, after Task 0's reading
    review split six of them, R17-R23 (Requirements).
  - Mark VTT-247, VTT-248, VTT-313 and VTT-440 withdrawn, as the Requirements
    section says.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:**
  - `git status` is clean;
  - the twenty-three rows exist;
  - `python3 tools/check-requirements-chain.py` passes;
  - the four rows read `WITHDRAWN 2026-10-24`.

### Task 1: the unstable test (item 1)

- **Files:** `internal/gateway/server_test.go`.
- **Do:**
  - In `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody`, open
    the handle with
    `sql.Open("sqlite", campaign.LogPath(f.path)+"?_pragma=busy_timeout(5000)")`.
  - Then a probe, outside the tree, with two handles on one SQLite file. One
    holds a read transaction across the other's
    `ALTER TABLE … RENAME`:
    - with the pragma, the rename waits for the read and succeeds;
    - without it, the rename fails at once with `SQLITE_BUSY`.

    Its output goes in the commit message.
- **True when:**
  - `go test -count=1000 -run
    '^TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody$'
    ./internal/gateway/` passes;
  - the probe shows both answers.
- **Break K1:** the probe without the pragma is the break. A green sample of
  the test is not evidence that the race is gone, so the mechanism is
  isolated rather than
  counted.

### Task 2: the contract, the tools, and every guard that walks the oneofs (items 2, 3 and 13, contract half; item 3's console)

A new oneof arm turns red every test that walks the oneofs, so this commit
carries what each of them asks. Pre-commit runs tier 1, and pre-push tiers 2
and 3, where `test:unit` leaves `internal/gateway`, `internal/mcp` and
`cmd/vtt`:
- at pre-commit, `TestEveryEnvelopePayloadArmHasAnExplicitRuling`
  (`projection_internal_test.go`), `TestToolsMatchGolden`
  (`tools/toolgen`) and the three command-surface guards (P9, E5);
- at pre-push, `TestEveryClientCommandConverts` (`convert_test.go`),
  `TestEveryClientCommandHasRoleCells` and its `commandFor` helper
  (`authz_test.go`), the MCP tool lists and fuzz seeds (`internal/mcp`), and
  the 14 MCP tests of `cmd/vtt` (P2b).

The surface guards are why the console's control comes here, ahead of the
ticket's order. The owner ruled "En ticket, servern först", and this is a
departure from it that the gates force, not one chosen.

- **Files:**
  - `contract/vtt/v1/commands.proto` and `contract/vtt/v1/events.proto` (D1);
  - what `task generate:contract` writes: `contract/gen/`,
    `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`;
  - `tools/toolgen/main.go` (D2, D3; `use_ability`'s description);
  - `contract/testdata/expected_tools.json`, by hand;
  - new fixtures `contract/testdata/token_placed_with_text_envelope.json`,
    `token_described_envelope.json`, `describe_token_command.json` and
    `ability_used_with_outcomes_envelope.json`, with rows in
    `contract/roundtrip_test.go` and `contract/events.test.ts`;
  - `internal/gateway/convert.go`: `place_token`'s four fields and
    `describe_token`'s conversion;
  - `internal/gateway/authz.go`: the `describe_token` row, DM and agent, and
    its `commandName` arm;
  - `internal/gateway/authz_test.go`'s `commandFor`;
  - `internal/perceive/projection.go`: a `TokenDescribed` arm in `classify`
    that withholds, which leaks nothing until Task 5 gives it D10's rule;
  - the `internal/mcp` lists;
  - `client/src/commands.ts` (`describeToken`, with a one-line doc);
  - `client/src/view/dm.ts` (D16);
  - `client/test/command-surface.test.ts` (the row, with its `action`);
  - `client/test/dm-view.test.ts` and `client/test/commands.test.ts`;
  - `cmd/vtt/webdist/` (E6);
  - `docs/requirements.md`.
- **The tool texts:**
  - **`place_token`'s field docs:**
    - `name`: "Optional; what every player and spectator whose character
      sees this token is told it is."
    - `notes`: "Optional; what they are told of it beside its name."
    - `gmName` and `gmNotes`: "Optional; the DM's and the agent's alone: no
      player or spectator is ever sent it."
  - **`describe_token`'s description:** "Rewrite a placed token's name,
    notes, GM name and GM notes, all four at once: a field sent empty is
    cleared, so send the present value of any you keep. Players and
    spectators whose character sees the token are sent its new name and
    notes; the GM name and GM notes reach only the DM and the agent. DM/agent
    only."
  - **`use_ability`'s `targetIds` doc** gains: "A player's every target must
    be an actor the character judging their moves sees; any other is refused
    exactly as an id that names no actor is."
- **New tests:**
  - **T1**, `tools/toolgen/main_test.go`,
    `TestPlaceTokenOffersTheFourTextsAsOptional`: the schema's `required`
    list is the four of today, and each new field is documented with who is
    sent it.
  - **T2**, `TestDescribeTokenStatesAllFourAtOnce`: the tool exists, its
    `required` list is all five fields, and its description says an empty
    field is cleared.
  - **T3**, `TestUseAbilitySaysAPlayersTargetMustBeSeen`.
  - **Rt1, Rt2**: the four fixtures round trip in Go and TS. They are red
    today, since the keys are unknown.
  - **A1**, `authz_test.go`, `TestOnlyTheDMAndTheAgentMayDescribeAToken`: a
    player and a spectator are refused with `ErrUnauthorized`, and the DM and
    the agent pass `Authorize`.
  - **Convert**, `convert_test.go`: `place_token`'s four fields and
    `describe_token`'s five reach the event.
  - **Cc1**: the console draws a row per token of the world, by id.
  - **Cc2**: "Describe" sends `describe_token` naming that token, with the
    four fields as typed, emptied ones included.
  - **Cc3**: a typed field survives a repaint until sent, and only its own
    row's draft is cleared.
  - **Cb**: `describeToken` builds the command with all four fields.
- **Order:**
  1. Write T1-T3, the round trips, A1, Convert and Cc1-Cb: red.
  2. Change the protos, then run `task generate:contract`.
  3. Edit `expected_tools.json` by hand until `TestToolsMatchGolden` passes.
  4. Make the conversion, the role row, the withheld arm, the `internal/mcp`
     lists, the builder, the console control and the `COMMAND_SURFACE` row.
  5. Run `task build:client`.
  6. Run every Go test, `env -u CLAUDECODE bun test client/test contract` and
     `task client:typecheck`. Under P2/P2b, nothing else reds on these
     changes. The console's command is refused by the fold until Task 3 gives
     `engine.Apply` its arm.
- **True when:**
  - all of the above are green, the three surface guards among them;
  - the four `dm.ts` adjudicated lines are byte-identical to main's;
  - `git status` lists the files above, `cmd/vtt/webdist/assets/index.js`
    among them.
- **Breaks:**
  - **K2:** drop `place_token`'s `requiredOverride`. T1 goes red.
  - **K3:** drop the `use_ability` sentence. T3 goes red.
  - **K6:** `commandRoles` admits the player. A1 goes red.
  - **Kc1:** the console group draws the first token only. Cc1 goes red.
  - **Kc2:** the builder drops `gmNotes`. Cc2 and Cb go red.

### Task 3: the engine, the client fold, the generator (items 2 and 3, the folds)

- **Files:**
  - `internal/engine/state.go` and `apply.go` (D5);
  - `client/src/fold.ts` and `state.ts` (D6);
  - `client/src/view/dm.ts`, the console's prefill (D16), which needs D6's
    fields;
  - `client/test/fold-unit.test.ts`'s "a token whose id names a prototype
    member is placed and moved like any other": the expected token carries
    the four empty fields (D6). Its `toEqual` goes red with Task 3's fold,
    not Task 7's;
  - `internal/eventgen/model.go` (D18);
  - a new `contract/testdata/named_token_dump.json`;
  - the tests below;
  - `cmd/vtt/webdist/`;
  - `docs/requirements.md`.
- **New tests:**
  - **En1-En3**, `internal/engine/apply_test.go`:
    - `TestATokenPlacedKeepsItsFourTexts`;
    - `TestATokenDescribedReplacesAllFourTexts`, emptied ones included;
    - `TestADescriptionOfNoTokenIsRefusedAndChangesNothing`: the error is
      `engine: described unknown token "t"`, worded as `moved` and `removed`
      are.
  - **Fo1-Fo3**, `client/test/fold-unit.test.ts`: the same three, mirroring the
    refusal as `unknown token "t" described`.
  - **Du**, the dump parity: `named_token_dump.json` is the dump of a world
    with one named token. `cmd/vtt`'s `TestTheDumpOfANamedTokenCarriesItsTexts`
    compares it with `writeDump`'s, and a `fold-unit.test.ts` row compares it
    with `foldToDumpJSON`'s.
  - **A2a**, a live gateway test in `server_test.go`,
    `TestADescribeOfNoTokenIsRefusedAndAppendsNothing`: the head is
    unchanged.
  - **A2b**, the same file,
    `TestADMsDescribeAppendsOneTokenDescribedCarryingEveryField`: the DM's
    `describe_token` of a held token appends one `TokenDescribed` carrying
    every field given. Red after Task 2: the fold has no arm.
  - **Cc4**, `client/test/dm-view.test.ts`: a console field with no draft
    shows the token's present text.
  - **Gen**, `internal/eventgen/model_test.go`: a generated `describeToken`
    folds, and a placement's texts are its id's.
- **True when:**
  - `go test ./...` and every client test are green;
  - the property and oracle tests pass at `-count=20`;
  - the four `dm.ts` adjudicated lines are byte-identical to main's;
  - the comment check after staging.
- **Breaks:**
  - **K4:** `apply.go` drops `Name`. En1 goes red.
  - **K4b:** `apply.go`'s `TokenDescribed` arm keeps a field sent empty. En2
    goes red.
  - **K4c:** `apply.go`'s `TokenDescribed` arm stores a token it does not
    hold instead of refusing. En3 and A2a go red.
  - **K5:** `fold.ts` drops `gmNotes`. Fo1 goes red.
  - **K5b:** `fold.ts`'s `tokenDescribed` arm keeps a field sent empty. Fo2
    goes red.
  - **K7:** `foldToDumpJSON` omits `Notes`. Du goes red in TS.
  - **Kc3:** the prefill ignores the token. Cc4 goes red.

### Task 4: the loaders and the shipped adventure (items 2 and 12, files half)

- **Files:**
  - `internal/adventure/format.go`, `load.go` and `compile.go`;
  - `internal/mapdef/format.go`, `load.go` and `compile.go`.

    Each format gains `Name, Notes, GMName, GMNotes` on `Placement`, and each
    loader's `placementJSON` the same fields in the same order, with tags
    `name`, `notes`, `gm_name`, `gm_notes`. The conversions
    `mapdef.Placement(p)` and `Placement(p)` stay conversions.
  - their tests and testdata: a named placement in
    `internal/adventure/testdata/valid` and in a `mapdef` fixture;
  - `docs/map-format.md` §6: the four optional keys, who is sent which, and
    that none is bounded (SPEC-018);
  - `adventures/goblin-ambush/scenes/ravine.json` and
    `goldens/compiled-batch.json`, by hand (D19);
  - `docs/requirements.md`.
- **New tests:**
  - **L1, L2**, adventure: `TestAPlacementCarriesItsFourTextsIntoTokenPlaced`
    (load and compile), and a placement with them loads. Red today, since
    `decodeStrict` refuses the keys.
  - **L3, L4**: the same for `mapdef`.
- **True when:**
  - all loader tests, the adventure conformance runner and `cmd/vtt` are
    green;
  - `grep -c '"gm_name"' adventures/goblin-ambush/scenes/ravine.json` prints
    2.
- **Breaks:**
  - **K8:** the adventure compile drops `gm_name`. L1 goes red.
  - **K9:** the map loader drops `notes`. L3 goes red.

### Task 5: what a character log is sent (items 4-7)

- **Files:**
  - `internal/perceive/projection.go` (D7-D10; the `TokenDescribed` arm Task
    2 left withholding gets D10's rule; `projection.go`'s VTT-313 pointers
    re-pointed in place);
  - `internal/perceive/` tests, the eight edited and N1-N8;
  - `internal/campaign/` tests, the five edited;
  - `internal/gateway/qa_issuer_test.go`;
  - `internal/gateway/qa_creature_test.go` (VTT-313, twice) and
    `internal/perceive/qa_testimony_eyes_test.go` (VTT-247), re-pointed;
  - a new `internal/gateway/token_text_test.go` for Gw5;
  - `contract/testdata/removal_batch_projected_stream.json` and
    `client/test/removal-batch-parity.test.ts`;
  - `docs/requirements.md`.
- **New tests**, each after a closing brace, with `Eyes{Shoulder}` and
  resources named `vigor` (E8):
  - **N1**, `roster_test.go`,
    `TestACreatureIsIntroducedWithItsIdModuleKindAndConditionsAlone`.
    - The creature has a name, attributes, a resource, module data, a
      condition and a controller.
    - The `ActorAdded` carries id, module and kind. The grant and the
      condition follow.
    - A party member in the same look is introduced whole.
  - **N2**, `status_test.go`, `TestACreaturesResourceChangeReachesNoLog`:
    seen or not, it is withheld. A party member's in sight is forwarded.
  - **N3**, `board_test.go`, `TestATokenArrivesWithItsNameAndNotesAlone`.
  - **N4**, `board_test.go`,
    `TestATokensRewriteReachesALogOnlyInSightAndOnlyWhenItsTextChanged`, P3's
    shape:
    - a GM-only rewrite sends nothing;
    - a name rewrite sends one `TokenDescribed` with no GM fields;
    - an unseen rewrite sends nothing, and the token returns with its present
      name.
  - **N5**, `history_test.go`, `TestACreatureInSightIsNotIntroducedAgain`:
    twelve events send no `ActorRemoved` or `ActorAdded` of it. This is item
    7's first case, green today and a guard of D7.
  - **N6**, `status_test.go`,
    `TestACreatureAGrantMakesAPartyMemberInSightArrivesWhole`: exactly E1's
    frames, folded, then a next event sends nothing of it.
  - **N7**, `status_test.go`,
    `TestAPartyMemberAGrantMakesACreatureInSightIsHeldStripped`: the same
    shape the other way.
  - **N8**, `status_test.go`,
    `TestAReshapingKindChangeOutOfSightWaitsForSight`, in both directions.
    Its actors carry no resource and each has a controller the log holds, so
    the kind can travel in a grant and `correct`'s own fallback, which
    re-introduces whenever the resource sets differ, cannot stand in for the
    re-introduction K16b removes.
    - A creature made a party member while unseen: no frame while unseen, and
      on sight the re-introduction, whole.
    - A party member made a creature while unseen: no frame while unseen, and
      on sight the re-introduction, stripped.
  - **Gw5**, `token_text_test.go`,
    `TestAPlayerIsSentNoFrameNamingACreatureItSees`, over a live connection.
    - A player whose character sees a creature is sent no frame carrying the
      creature's actor name, any of its resource keys, or the token's GM name.
    - The DM's stream carries all three.
- **Order:**
  1. Write N1-N8 and Gw5: N1-N4 and N6-N8 red, as P2 and P3 measured.
  2. Make D7-D10.
  3. Run `go test ./...`: the sixteen of P2 go red, less
     `TestEveryClientCommandHasRoleCells` (Task 2), and the new ones green.
  4. Edit them as listed.
  5. Regenerate the removal-batch fixture from the test's own stream, read
     the diff, and edit the parity test.
  6. Re-point VTT-247, VTT-248 and VTT-313 under `internal/`.
- **Between this commit and Task 7's,** the embedded bundle draws a
  creature's disc with its actor id as the initial, since the name is gone.
  The branch is not merged before Task 8.
- **True when:**
  - `go test ./...` and every client test are green;
  - the edited pump-order tests pass at `-count=3 -cpu 1,2`;
  - the four `projection.go` adjudication lines are byte-identical to main's.
- **Breaks:**
  - **K10:** `introduce` clones the actor whole. N1 and Gw5 go red.
  - **K11:** `classify` forwards every `ResourceChanged` the viewer saw. N2
    goes red.
  - **K12:** `TokenPlaced` carries `GmName`. N3 and Gw5 go red.
  - **K12b:** `transitions`' `TokenPlaced` drops `Notes`. N3 goes red.
  - **K13:** `TokenDescribed` is forwarded whenever held and in sight. N4
    goes red.
  - **K14:** `recall` compares with the world's actor whole. N5 goes red, a
    re-introduction at every event.
  - **K15:** `hideTokensOf` is removed. N6 and N7 go red at the fold
    (`viewer_test.go`'s Fatal).
  - **K16:** a reshaped actor is withdrawn whether seen or not. N8 and
    `TestAKindChangedOutOfSightIsCorrectedOnSight` go red. P2 measured the
    second.
  - **K16b:** a reshaped actor that comes into sight is corrected instead of
    re-introduced. N8 goes red in both directions.
  - **K17:** `forwardable` keeps `GmNotes`. N4 goes red.

### Task 6: a player's ability, and what it records (items 8 and 11, server half)

- **Files:**
  - `internal/gateway/server.go` (D11);
  - `internal/perceive/projection.go` (`SeesActor`);
  - `internal/rules/resolve.go` (`UnknownTarget`; outcomes, D12);
  - gateway, `perceive` and `rules` tests;
  - the two scenario streams that change (E9);
  - `docs/requirements.md`.
- **New tests:**
  - **Gw1**, a live test in `server_visibility_test.go`,
    `TestAPlayersAbilityAtATargetBehindAWallIsRefusedAsAnUnknownTarget`.
    - The target is in range behind a wall, and the answer is the string a
      target id naming no actor gets, with the id put in.
    - The head is unchanged.
    - Red today: it resolves.
  - **Gw2**, the same file,
    `TestAnUnseenTargetAndAnAbsentOneAreAnsweredAlikeWhateverTheCommand`. For
    each pair, unseen `X` against absent `Y`, the answers are equal once `X`
    is replaced by `Y`:
    - a valid ability;
    - a bogus ability id;
    - `[X, X]` against `[Y, Y]`;
    - more targets than the ability allows.
  - **Gw3**, the same file, `TestAPlayersAbilityAtASeenTargetInRangeResolves`:
    resolved as today.
  - **Gw4**, `TestTheDMsAndTheAgentsAbilitiesAreNotJudgedBySight`: each
    resolves behind the wall.
  - **S1**, `perceive`,
    `TestSeesActorAnswersWhatALookSeesAndRecordsNothing`.
  - **Ru1**, `internal/rules/resolve_test.go`,
    `TestAnAbilityUsedCarriesEachTargetsResultAndRollTotalOnly`. Two targets,
    one hit and one miss:
    - each outcome carries its target, its branch and the resolution total;
    - the compared total appears in no outcome.
  - **Ru2**, `TestAnAbilityWithNoResolutionCarriesNoOutcome`.
- **Order:**
  1. Write the tests: Gw1, Gw2, Gw4 (with the gate placed wrongly) and Ru1 red.
  2. Make D11 and D12.
  3. Run `cmd/vtt`: the two streams differ by `outcomes` alone. Re-record
     them and re-derive `state.json`, which is unchanged.
- **True when:**
  - `go test ./...` is green;
  - `grep -rn 'unknown target actor' --include='*.go' internal | grep -v
    _test.go` prints `UnknownTarget`'s line alone.
- **Breaks:**
  - **K18:** the gate is removed. Gw1 goes red.
  - **K19:** the gate refuses only actors the snapshot holds and does not
    see. Gw2's bogus-ability pair goes red.
  - **K20:** the gate also runs for the DM. Gw4 goes red.
  - **K21:** `Outcome.total` takes the compared total. Ru1 goes red.
  - **K22:** the gate answers its own literal. Gw1 goes red.

### Task 7: what a browser window shows (items 9-12, browser half)

- **Files:**
  - `client/src/view/naming.ts`, new (D14);
  - `client/src/view/grid.ts` (`tokensOnScene` takes the role; D20);
  - `client/src/view/spectator.ts` (`describe`, `renderDisc`, the card,
    `renderFeed`, `renderTicker`, `ViewExtras`; D13, D15);
  - `client/src/view/player.ts` (labels);
  - `client/src/app.ts` (`dm`, `abilities`, the pinned card);
  - `client/src/style.css` (the card);
  - the 27 client tests above for Task 7, and C1-C10;
  - `client/e2e/table.spec.ts`;
  - `tools/ts-mutation-equivalents.txt` (the two rows leave, E7);
  - `cmd/vtt/webdist/`;
  - `docs/requirements.md`.
- **New unit tests:**
  - **C1**: the non-DM chain: token name, then a party member's name, then
    `a creature`, never an id.
  - **C2**: the DM chain: GM name, then the actor's name, then `a creature`.
  - **C3**: the initial and the title: `?` and `a creature` for an unnamed
    creature, and never an id.
  - **C4**: the card per role. A player's holds the name and notes and no GM
    text; a DM's adds "players see" and the GM notes.
  - **C5**: a `contextmenu` on a disc is prevented and calls `onCardPin` with
    its id. A disc whose id is `pinnedCard` is drawn `.pinned`, across a
    repaint.
  - **C6**: a click on a disc still reaches `onCell`.
  - **C7**: over every `Envelope` payload case, built by reflection from
    `EnvelopeSchema`'s oneof, for a non-DM namer. No line equals its case
    name, contains a fixture actor or token id, or matches `/joined/`.
  - **C8**: the ability line. The `AbilityUsed` has `outcome_summary`
    `"Longsword Strike on act-x: hit (17 vs 15)"` and an outcome with total
    17.
    - The line is `… used Longsword Strike on a hunched goblin: hit, rolled
      17`.
    - It contains neither `15` nor `act-x`.
  - **C9**: a token the fold no longer holds is named by the newest entry
    that named it.
  - **C10**: the target buttons are labelled by the naming, never an id.
- **The e2e:**
  - "the story feed and ticker record what happened": `/^Lera is at
    \d+,\d+$/`.
  - **W1**, in "a player's ability reaches the creature they aim it at":
    - The player's page records every frame it receives.
    - It aims at "a hunched goblin".
    - The Story's line matches `/^Human Fighter used Longsword Strike on a
      hunched goblin: (hit|miss), rolled \d+$/`, and no row contains `" vs "`.
    - No received frame contains "Goblin Cutter", or a `resourceChanged` or a
      `resources` key of `act-cutter`.
    - Hovering `tok-cutter` shows a `.token-card` reading "a hunched goblin".
    - A right-click keeps it shown after the pointer leaves.
    - It carries R13's and R16's ids.
- **Order:**
  1. Write the e2e changes. Run `task e2e` on Task 6's commit, whose bundle
     Task 3 last rebuilt: both go red, and everything else stays green. This
     is item 12's "fails on today's bundle".
  2. Write C1-C10: red.
  3. Make D13-D15, D17 and D20, then the 27 edits.
  4. Delete the two `grid.ts` rows after the by-hand check (Constraints).
  5. Run `task build:client`, every client test, `task client:typecheck`,
     `task e2e`, and the comment check after staging.
- **True when:**
  - all of the above are green, H1 observed (W1);
  - `grep -rn -E 'VTT-(247|248|313|440)([^0-9]|$)' internal cmd client tools`
    prints nothing.
- **Breaks:**
  - **Kc4:** the naming falls back to the actor id. C1, C3 and C10 go red.
  - **Kc5:** the DM chain puts the token's name first. C2 goes red.
  - **Kc6:** the card shows GM notes to a player. C4 goes red.
  - **Kc6b:** the card omits the notes. C4 goes red.
  - **Kc6c:** the DM's card omits the GM notes. C4 goes red.
  - **Kc7:** a right-click does not pin the card. C5 goes red.
  - **Kc7b:** the `contextmenu` is not prevented. C5 goes red.
  - **Kc8:** `describe`'s `tokenHidden` arm returns `p.case`. C7 goes red.
    The exhaustive `never` check leaves the default unreachable, so a break
    there could not go red.
  - **Kc9:** the ability line reads `outcome_summary`. C8 goes red at `15`.
  - The card's `pointer-events: none` has no happy-dom break (H1). W1's
    hover and the existing e2e moves by a board click observe it.

### Task 8: the gate, whole

- **Do:** run `task check` whole on Task 7's commit, the last that changes
  code, then `task e2e` on the same commit.
- **True when:** both are green, including:
  - `check:drift`;
  - `check:breaking`, which reports nothing breaking;
  - both mutation gates, with no new adjudication on code this plan wrote;
  - `check:requirements-chain`, which resolves R1-R23.

### Task 9: the report

- **Do:** write the implementation report under `docs/reports/`, with its own
  reading review and its own commit. It covers:
  - each "Done looks like" item with its observation (the table above);
  - every K and Kc break, with what went red;
  - D12's weighing;
  - E1's frames;
  - H1's first browser run;
  - Phase 4a's adjudications, under their own heading.
- **True when:**
  - the report answers the ticket's fourteen items in the ticket's
    numbering, each `[x]` naming its observation;
  - it has had its reading review;
  - it is committed alone, after Task 8's gate.

### Task 10: the specifications

- **Do,** with the `specification` skill, last and from the report:
  - **SPEC-016:**
    - `shownAs` and what an introduction carries;
    - `gone` and `reshaped`, `hideTokensOf`, `look`'s `party`;
    - the `ResourceChanged` and `TokenDescribed` rulings, `forwardable`'s GM
      fields, the token's texts in `TokenPlaced`, `SeesActor`;
    - the consequences: "kind is status", the two occasions of a bare
      `ActorRemoved` (now preceded by `TokenHidden` in sight), and an actor
      added again;
    - the Requirements list, with successors in place of the withdrawn rows.
  - **SPEC-007:** the four fields, the new arms, `AbilityUsed.outcomes`, and
    that `outcome_summary` stays whole.
  - **SPEC-013:** `describe_token`'s row, the sight gate in the command path
    and its refusal, and that a player who controls only a creature names no
    target.
  - **SPEC-018:** D4's second-table rows.
  - **SPEC-014:** a placement's four texts.
  - **SPEC-021:** the reused-id sentence's citation of VTT-313 moves to R9;
    for a creature, only another module id re-introduces.
  - **SPEC-022:** the naming, the card, the Story's lines and the ability
    line, and the console's rewrite control.
  - Cite the owner's rulings A-6, K-1, B-5, B-1 and those of 2026-10-24.
- **True when:**
  - `python3 tools/check-requirements-chain.py` passes on its commit, run by
    hand;
  - each new sentence has been run against the code by someone other than
    its writer, Phase 4b's reading included.

## For Phase 4a

Hand the QA agent:
- R1-R23, with VTT-243, VTT-249, VTT-252, VTT-365, VTT-404, VTT-408 and
  VTT-460;
- `go doc ./internal/perceive` and `go doc ./internal/rules`, with
  `SeesActor` and `UnknownTarget`;
- `TokenPlaced`'s, `TokenDescribed`'s and `AbilityUsed`'s fields;
- that the browser's role comes from `/api/me`.

Tell it:
- **What a log holds.**
  - A creature in a character log is its id, module and kind.
  - Two creatures under one id differ only by module.
  - A reshaping kind change waits for sight.
- **Character logs.** A character log's own entries carry no `EventId`, and
  forwarded ones share the world's ids, so compare entries with `proto.Equal`.
- **Imports.** `go-arch-lint` holds the layer imports, and `perceive` imports
  only `contract`, `engine` and `sight`.
- **The vocabulary gate** reads Go tests: no `hp` (E8).
- **Pump-order tests.** Run them at `-cpu 1,2`.
- **The browser.**
  - happy-dom evaluates no `:hover` (H1).
  - bun runs every test file in one process, so a boot test clears both
    storages in `beforeEach`.
  - A failing `toBeNull()` takes 20-30 s to print.

## Forward coupling

- **The defense and the ids on the wire.** `outcome_summary` still carries
  the target's id and the compared total ("utfallsraden står"), and every
  frame carries author ids. Item 10 keeps both out of the window. Keeping
  them out of the frames is the ticket's open point, not this plan's.
- **The monster check (K-1, "later").** A check that reveals more on a
  success would be a `describe_token` the DM issues, or a per-character text.
  A token holds one name for every player.
- **B-2 (whispering)** is the next ticket. The card and the Story's naming
  are the window's, so a whisper's line names actors through the same module.
- **A-11 (multi-target from the browser)** is unchanged. C8's line already
  words several outcomes.
- **A creature a player controls as `NON_PARTY`** reaches their window
  stripped, as any creature does. The 2026-10-19 ruling makes a creature a
  player runs a party member, and `grant_actor_control`'s description says
  so (VTT-389).
