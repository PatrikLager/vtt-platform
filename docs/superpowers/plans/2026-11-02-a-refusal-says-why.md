# Plan: a refusal tells the player why, in plain words

- **Ticket:** `docs/superpowers/specs/2026-11-02-a-refusal-says-why-design.md`,
  as narrowed on 2026-11-02 to the board engine.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-02 by an agent that did not write the ticket,
    read-only against main `8f933f8` and the uncommitted ticket, first as
    written and again after its narrowing. Check 1 was re-run on the
    narrowed ticket's paths.
  - Probes ran in a copy of the tree outside the repository. P5 and P6 are
    prototypes of the server half and the client half in that copy, measured
    and then undone; nothing in the repository was edited.
  - `task check`, `task e2e`, Stryker and gremlins were not run.
- **To be committed as:** `docs/superpowers/plans/2026-11-02-a-refusal-says-why.md`.
- **Base:** main `8f933f8`.
- **Branch:** `fix/a-refusal-says-why`.
- **Owner's rulings taken in:**
  - 2026-11-02, "Servern, en text för alla": the server writes the words
    where it refuses, one text for every reader, the agent's included. No
    client-side table, no second contract field. The window shows the text
    with no `refused: ` and no package before it. The preview chosen for a
    move: `You cannot move there: a wall.`
  - 2026-11-02, "Toast + rad i perch-vyn", kept after the probe ("Ja,
    beslutet står"): the window says "You no longer play Lera." when it
    happens, and a player's perch says they play no character and the DM can
    give them one; a spectator's does not.
  - 2026-11-02, the narrowing: sight, movement, joining a table, and a
    character granted or taken back are the board engine's; the rules
    engine, which runs a game's rules on the board, is separate and must
    take any game system. This ticket is the board engine's. Every refusal of
    `rules.Resolve` keeps its words until how the rules engine consumes a
    ruleset has been investigated on its own track: `ruleset.json` lists
    attributes, defenses and resources as bare keys (`str`, `ac`,
    `flurry_uses`), so plain words there would still be ids.
    - **So the preview's second text, `Goblin Archer is out of range of
      Longbow: 6 squares away, range 5.`, is no longer in scope.** It is a
      refusal of `Resolve`.
  - Standing rulings that bind:
    - an unseen square is refused with one text (VTT-156); an unseen target
      is answered as an id naming no actor (VTT-472);
    - "Stay on Tobbe" (VTT-456);
    - a refused command keeps what was typed (SPEC-022);
    - delete the old solution first (2026-09-04);
    - a near-theoretical edge case with a small consequence gets no code, no
      test, no debt entry and no spec sentence (2026-10-21);
    - one source for every fact (2026-09-23);
    - the client's full mutation run is the owner's to ask for (2026-10-26).
- **Signed off:** 2026-11-02. Q2: "Ett ärende". The words of D3, D11 and D12:
  "Godkänn orden". G10 was fixed in the ticket before Task 0: its first rule
  now names the move gate's refusal. Q1 was withdrawn by the narrowing; its
  number is not reused, nor are D2, D6, the requirement labels P4 to P8, G1,
  G3, G7 and G8.
- **Ids:** P1 = VTT-601, P2 = VTT-602, P3 = VTT-603, C1 = VTT-604,
  C2 = VTT-605, C3 = VTT-606, C4 = VTT-607.

## The verdict, check by check

1. **Every path resolves. By command** (P1). Re-run on the narrowed ticket:
   every one of its 19 paths, `rulesets/dnd45e-minimal/ruleset.json` among
   them, and every symbol it names is found (`session.ts`'s `set` and
   `feed.ts`'s `buildFeed` included).
2. **"Done" is an observation. By command, then a reading.** Items 1 to 6
   name tests that do not exist, and the behaviour each pins is absent
   today: `git grep -i "no longer play\|play no character\|You cannot move
   there" -- client/src internal cmd` prints nothing, and
   `client/test/app.test.ts` asserts `refused: not authorized`. The probe
   logs show cases 1 and 2 as the ticket tells them. Item 7 is now
   consistent with the narrowed scope; item 8 is the gate.
3. **Each rule is breakable. A reading.** All four are. "The set they
   control" is not the set's `actors` (G4).
4. **The scope matches the claim. By command, then a reading.** P5 and P6
   turn red the tests listed under "The existing tests that change". Files
   the ticket does not name: G5.
5. **No recorded decision is contradicted silently. A reading.** VTT-156,
   VTT-472, VTT-506, VTT-589, VTT-456, VTT-340 and VTT-404 stay true.
   SPEC-013's "every refusal `Authorize` returns wraps `ErrUnauthorized`"
   stays true (D4). With `Resolve` out of scope, no creature is named by any
   new text, so K-1 is not in play.
6. **The records it moves. By command, then a reading.** The section names
   SPEC-013 and SPEC-022, and both resolve. `git grep -n "cannot move
   there\|refused: \|not authorized: \|has no name" docs/specifications`
   finds those two alone. SPEC-012 no longer moves: it states
   `handleUseAbility`'s refusals without quoting them, and they keep their
   words.

## The gaps this plan inherits

- **G1, G3, G7, G8: settled by the narrowing.** The creature's GM name
  (G1), item 8's conflict with an outcome-time refusal (G3) and the
  vocabulary rule for ruleset tests (G8) concerned `Resolve` alone. Item 5
  now says "the DM revokes a player's characters one after the other",
  which is what E1 does (G7).
- **G2. The panel's refusals, bounded.** D1 enumerates every refusal the
  panel's moving, speaking and whispering can meet, and which are reached in
  ordinary play, by a race, or near-theoretically, with the search behind
  each.
- **G4. The set a player controls is not `WatchableChanged.actors`.** When
  the set says they control none, `actors` is every party member, and a
  spectator's `actors` is every party member whatever they control
  (`watchable` and `MayWatch` in `internal/gateway/viewpoint.go`). D9, D10.
- **G5. Files the ticket does not name.**
  - `client/test/session.test.ts`, `client/test/app.test.ts`,
    `client/test/spectator-view.test.ts`;
  - `client/e2e/handover.spec.ts` (E1) and `client/e2e/refusals.spec.ts` (it
    matches `^refused: `);
  - `internal/gateway/plain_refusal_test.go`, new;
  - `cmd/vtt/webdist`; `tools/comment-ceilings.txt`.
  - Not changed, though the brief asked: `cmd/vtt` prints no `refused: `
    (P4), and no MCP tool description quotes a text that changes (P4).
- **G6. Four negative assertions go vacuous.** `client/test/app.test.ts`
  asserts `not.toContain("refused")` in four tests. Without the prefix they
  pass whatever the window shows. Each is replaced by the positive it stood
  for ("The existing tests that change").
- **G9. The comment ceilings are tight** (P8): every touched file is within
  0.1 of its ceiling, and `narration_test.go`, `creature_test.go` and
  `qa_creature_test.go` are at 0.0. New tests go in new files.
- **G10. Settled before Task 0:** the ticket's first rule names the move
  gate's refusal, so `authorizeTokenOwnership`'s refusal, which names a token
  id and a participant id and keeps its words by item 7, is outside it.

## Measurements this plan stands on

All on 2026-11-02 at `8f933f8`, in a copy outside the repository.

| # | Probe | Result |
|---|---|---|
| P1 | `test -e` on the narrowed ticket's 19 paths; `grep -c` on each symbol | all resolve |
| P3 | Baseline, untouched copy: `go test ./internal/gateway/ ./internal/rules/... ./internal/harness/ -count=1`; `env -u CLAUDECODE bun test client/test contract`; `bunx tsc --noEmit -p client/tsconfig.json` | all pass (8 min 53 s); 1939 client tests pass; typecheck clean |
| P4 | `git grep` for each text that changes, across `*.go`, `*.ts`, `*.json`, `scenarios/`, `docs/specifications`, `docs/requirements.md`; `git grep -n 'refused: ' -- 'cmd/*.go' 'internal/*.go' ':!*_test.go'`; the same texts in `cmd/vtt/tools.json`, `contract/gen/tools`, `tools/toolgen`, `internal/mcp/server.go`, `contract/testdata/expected_tools.json` | the pins listed in D3's table; no Go code prints `refused: `; no tool description quotes a text that changes; no register row quotes one |
| P5 | Prototype of the server half (D3's T1 to T4, D4), with `Resolve` changed too before the narrowing; `go build`, `go vet`; P3's Go runs; `go test ./cmd/vtt/ -run 'TestScenarioLibraryRunsSelfContained\|TestMCP\|Ruleset'` | builds and vets clean. Of the board engine's texts: 25 gateway tests red, every one through the five pins under D3; `scenarios/session-zero.json` green; every nameless test green (they pin substrings). The run's red in `internal/rules`, the conformance goldens and `goblin-fight.json` came from the `Resolve` half, which the narrowing drops |
| P6 | Prototype of the client half (D7 to D12); typecheck; `env -u CLAUDECODE bun test client/test contract` | typecheck clean; 1935 pass, 4 fail: "a refused command is reported verbatim rather than silently doing nothing", "the indicator follows the SERVER's answer, never the click", "an accepted perch CLEARS the refusal the last one left up", "a refused Start session leaves the name typed beside the refusal" |
| P7 | On P6, `Session` over a real socket: case 1 (last party member revoked), case 2 (one of two), two at once with a rename and an equal set before, a first grant after none | told `[Lera]`, `[Ama]`, `[Ama Stone, Bo, a3]` once, and nothing |
| P8 | `measure` and `share_of` from `tools/check-comments.py` against `tools/comment-ceilings.txt` | `server.go` 21.31 (21.4); `authz.go` 11.80 (11.9); `viewpoint.go` 9.46 (9.5); `server_test.go` 23.89 (23.9); `qa_move_gate_test.go` 1.45 (1.5); `watch_test.go` 1.59 (1.6); `seat_internal_test.go` 2.72 (2.8); `whisper_test.go` 1.34 (1.4); `qa_speaking_test.go` 5.54 (5.6); `server_visibility_test.go` 13.38 (13.4); `app.ts` 47.64 (47.7); `session.ts` 34.92 (35.0); `view/spectator.ts` 39.81 (39.9); `view/player.ts` 20.55 (20.6); `narration_test.go`, `creature_test.go`, `qa_creature_test.go` 0.00 (0.0) |
| P9 | `tools/coverage-thresholds.txt`, `tools/ts-coverage-thresholds.txt` | `internal/gateway` 93.5; `app.ts`, `session.ts`, `view/spectator.ts`, `view/player.ts` 100 |
| P10 | `~/dev/RPTool/maptool` at `f4b7fef6c` | see Rule 9 |
| P12 | `client/e2e/setup.ts`, `client/e2e/handover.spec.ts` | mints `DM`, `Lera` (player), `Watcher`; the handover characters are `Ash` (`act-hand-a`) and `Bram` (`act-hand-b`); its second test revokes Bram and then Ash |
| P14 | `git grep -n "errors.Is(.*ErrUnauthorized" -- '*.go' ':!*_test.go'`; `git grep -n "res.error" -- client/src` | nothing; `act` and `switchTo` alone |

P2, P11 and P13 measured `Resolve`, the creature's names and the vocabulary
rule, and fell with the narrowing; their numbers are not reused.

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone (D14).
  - 3: the contract does not change. No field is added (the owner's ruling).
  - 4: one fold. Nothing here touches `engine.Apply`.
  - 5: no game-system vocabulary, tests included (`.semgrep/vocabulary.yml`
    covers every Go file under `internal/` and `cmd/`).
  - 8: cite names; never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `internal/`, `cmd/` or `client/src` is a warning or
    a pointer. The words a refusal says are pointed to (SPEC-013, SPEC-022),
    not explained beside the code.
- **Nothing under `internal/rules`, `rulesets/` or `scenarios/` changes**
  (item 7). `git diff main -- internal/rules rulesets scenarios` prints
  nothing at every commit.
- **The comment ceilings** (P8). Each file is measured with `share_of`
  before its commit. A test cites its rule on a citation-only line.
- **`check:new-prose`** holds added comment lines of `.go` and `.ts`
  files to an 85-column band, tests included.
- **`go-arch-lint`.** A `gateway_test` test may import `gateway`, `engine`,
  `identity`, `campaign` and the contract as its neighbours do.
- **Coverage floors** (P9). The four client files stay at 100.
- **The Go mutation gate runs in `task check`.** Each new condition has a
  test that goes red when it is negated. No adjudicated code line is edited
  (`tools/mutation-equivalents.txt`, SPEC-020): its `authz.go` and
  `viewpoint.go` entries sit on lines this plan does not touch.
- **Nobody starts Stryker** (D15).
- **A negative assertion needs its positive.** "No toast" comes with a set
  that does tell at the same table.
- **A client test reads a file outside `client/` only if
  `tools/ts-mutation-inputs.py` hashes it**: `contract/testdata/` or
  `contract/gen/`, never `cmd/vtt/tools.json`.
- **A new function or test goes after a closing brace** (`check:doc-owner`).
- **A test the register cites has a literal name.**
- `bun test` is measured with `env -u CLAUDECODE`.
- **Before every commit:** `python3 tools/check-new-prose.py main`;
  `python3 tools/check-requirements-chain.py .`; `python3
  tools/check-comments.py main`, after staging; for a client commit,
  `bunx tsc --noEmit -p client/tsconfig.json`, `task build:client` with
  `cmd/vtt/webdist` committed if it changed, and `python3
  tools/ts_mutation_inputs_test.py`.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo a break: the inverse edit, checked with
  grep, after asserting the broken text is not already in the file.
- **The review gate refuses untracked files.** Park scratch files outside the
  tree with `cp` and `cmp`.
- **Hash the tree before and after any reviewer.**
- **The pre-push hook takes minutes.** 600 000 ms, its whole output to a
  file, the machine's load checked first.

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c` (P10):

- **A catalog of sentences.** Every sentence a user reads is a key in
  `i18n.properties`, plain and in the second person: `slash.mustBeGM` "You
  must be a GM to use "/{0}".", `msg.error.gmRequired` "Only GMs can do
  that.", `msg.error.playerNotConnected` "Player "{0}" is not connected.".
- **A refusal names a token by its name.** `ImpersonateMacro` refuses with
  `impersonate.mustOwn`, "Only {0}'s owners can impersonate or run macros on
  it.", where `{0}` is `token.getName()`, the players' name. It is shown
  with `MapTool.addLocalMessage`, to the one user.
- **A blocked move is not refused.** `AbstractAStarWalker` routes around
  what blocks, and the walker writes no message.
- **No message for a change of ownership** was found in the catalog.

Borrowed: the second person; the sentence written where the refusal is
decided.

Not borrowed:
- **The catalog file.** The owner ruled the words are written where the
  server refuses, one text for every reader. A catalog is a second home for
  each sentence.
- **Refusals decided in the client.** MapTool's client holds the whole
  campaign and refuses locally; here the server decides and the client shows
  `CommandResult.error`.
- **Routing around a wall.** A move here is one square, and the owner's
  preview refuses it in words.

## Design questions for the owner

Q1 was withdrawn by the narrowing (2026-11-02).

2. **Q2. One ticket or split?** The ticket asks this, across its two
   components, the gateway and the client.
   - Recommended: **one.**
     - The owner ruled A-3 and A-7 together, as item 8 of the alpha order.
     - Both halves are small: four strings and one type in the gateway; two
       lines in `app.ts`, one hook in `Session` and one line in the perch.
     - Item 4 (no prefix) shows the server's words: shipped alone, it would
       show today's `gateway: cannot move there — a wall` bare for a time.
     - One report, one browser beat, one gate run.
   - Alternative: **two tickets**, A-3 (Task 1 and the prefix of Task 2) and
     A-7 (the toast, the perch line, E1).
     - Cost: a second report, review and gate for about forty lines, and
       `app.ts` and `app.test.ts` touched twice in a row.

**Noted for the owner, not asked.**
- **The preview's `Goblin Archer is out of range of Longbow: 6 squares
  away, range 5.` is not delivered here.** It is a refusal of `Resolve`,
  which the narrowing leaves as it is.
- **The DM's toast loses `refused: ` as well** (D7). The DM's other
  refusals keep their words, which begin with their package (`gateway:`,
  `engine:`, `rules:`), so they still read as refusals.
- **An unnamed party member is "an unnamed character" in the toast** (D11).

## Decisions

1. **D1. What the panel can meet, bounded.** The panel sends
   `moveToken`, `useAbility`, `setViewpoint`, `speakAs` and
   `speakOutOfCharacter`, and `addWhisper` (`client/src/view/player.ts`),
   and the board's armed doors send `openDoor` and `closeDoor`
   (`doorCommandFor`). Every refusal a move, a line and a whisper can meet
   was read in `handleCommand`, `Authorize`, `playerRules`, and
   `engine.Apply`'s arms for those commands:
   - **In ordinary play:** the move gate's two (T1, T2), since a player can
     click any square of the board, dark ones included; a nameless
     character's line and whisper (T3, T4), since the console's Add actor
     needs no name.
   - **By a race alone:** a revocation landing between paint and click
     (`authorizeTokenOwnership`, `mayVoice`'s and `mayWhisper`'s
     not-theirs, `MayWatch`). The panel repaints on every frame, so the
     window is one round trip wide. They keep their words (item 7):
     rewording `authorizeTokenOwnership` breaks the soak's and the
     scenarios' "not authorized" (`RunSoak`'s denied bucket,
     `scenarios/denials.json`, `scenarios/three-role-exit.json`), and no
     reader meets them in ordinary play, the agent included, since it is not
     judged by player rules (G10).
   - **Not ordinary:** a text over 8192 bytes, refused by the fold (a paste
     of eight kilobytes into a chat line).
   - **Not the board engine's words:** using an ability. `Resolve`'s
     refusals keep theirs (the narrowing), and so does the sight gate's
     answer for an unseen target, which VTT-472 makes `Resolve`'s own answer
     for an id naming no actor (`rules.UnknownTarget`).
     `authorizeActorOwnership` is a race, as above.
   - Doors are outside the ruling's actions, and `doorCommandFor` asks the
     same `mayWorkDoor` geometry first.
2. D2 was withdrawn by the narrowing.
3. **D3. The words.** Each `.` closes the sentence.

   | # | Old | New | Made in | Pinned today by |
   |---|---|---|---|---|
   | T1 | `gateway: cannot move there — you cannot see that square` | `You cannot move there: you cannot see that square.` | `handleCommand` | `qaMGUnseen` (`qa_move_gate_test.go`), `unseenSquare` (`watch_test.go`, used by `creature_test.go`, `qa_creature_test.go`), `unseen` in `TestAMoveRightAfterARevocationIsNotJudgedByTheLostCharacter`; SPEC-013; `scenarios/session-zero.json`'s `deniedContaining` still matches |
   | T2 | `gateway: cannot move there — ` + `describeBlockage(why)` | `You cannot move there: ` + `describeBlockage(why)` + `.` | `handleCommand` | `qaMGBlocked` and its crate line; `want` in `TestAPlayerCannotWalkOntoBlockingSceneryAndTheRefusalNamesIt`; SPEC-013 |
   | T3 | `gateway: not authorized: this party member has no name; ask the DM for one, or speak out of character` | `This character has no name. Ask the DM for one, or speak out of character with /ooc.` | `mayVoice` | substrings only: "DM" and "out of character" in `TestQASpkNamedRefusalsShareOneText`; `errors.Is` in `narration_test.go` and `TestQASpkPlayerRefusalsAndAcceptances` |
   | T4 | `gateway: not authorized: this party member has no name; ask the DM for one` | `This character has no name. Ask the DM for one with /ooc, then whisper.` | `mayWhisper` | substring "no name" in `whisper_test.go`; no test asks `errors.Is` of it |
   | T13 | `refused: ` + error, in `act` | the error | `client/src/app.ts` | four tests (P6); `refusals.spec.ts`; SPEC-022 |
   | T14 | `refused: ` + error, in `switchTo` | the error | `client/src/app.ts` | as T13 |

   - T1 and T2 are forced by the owner's preview. T3 and T4 are this plan's
     words, in its form and the second person. The owner may reword any at
     sign-off; each test pins the words.
   - T1 names no terrain: `TestAPlayerCannotProbeTheDarkWithMoveCommands`
     forbids "wall", "door", "crate", "scenery", "in the way" and "grid".
     It keeps "you cannot see that square", so `session-zero.json` keeps its
     substring (P5).
   - T2 keeps `describeBlockage` as it is, so "a closed door", "something (a
     boulder) is in the way" and "outside the grid" follow the colon.
   - T5 to T12 were withdrawn by the narrowing; their numbers are not
     reused.
4. **D4. T3 and T4 keep `errors.Is(err, ErrUnauthorized)`.** A small type in
   `authz.go`, `plainRefusal`, whose `Error()` is the plain text and whose
   `Unwrap()` returns `ErrUnauthorized`.
   - No production code asks `errors.Is` of `ErrUnauthorized` (P14); the
     tests do (`mustRefuse` in `narration_test.go`, `qa_speaking_test.go`,
     `qa_whisper_test.go`, `authz_test.go`), and so does SPEC-013's
     "Every refusal `Authorize` returns wraps `ErrUnauthorized`", which stays
     true.
   - Every other `Authorize` refusal keeps `gateway: not authorized: …`
     (item 7; the soak and the scenarios pin "not authorized").
5. **D5. The move gate's texts stay in `handleCommand`.** Two literals, and
   `describeBlockage` unchanged: `server_internal_test.go`'s table of its
   rewrites stays as it is.
6. D6 was withdrawn by the narrowing.
7. **D7. `act` and `switchTo` show the error with nothing before it, in
   every window.** They are one function each, for the console, the panel,
   the board and the perch; the ticket's rule says "the window". They are
   the only readers of `CommandResult.error` in `client/src` (P14).
   - Every refusal the narrowing leaves keeps its words, a DM's and an
     ability's alike (`gateway: …`, `engine: …`, `rules: …`), so a player
     who uses an ability still reads `rules: resolve: …`, now with no
     `refused: ` before it.
   - `client/e2e/refusals.spec.ts`'s first test matched `^refused: ` to tell
     the server's refusal from the console's own guard. It matches the
     server's own text instead, `^engine: token placed in unknown scene`.
   - Item 4's test is the existing "a refused command is reported verbatim
     rather than silently doing nothing", asserting the toast equals
     `not authorized`: red today.
8. **D8. G6's four negatives.** Each is replaced by what it stood for, or
   removed where the test already holds that positive ("The existing tests
   that change").
9. **D9. `Session` computes who left; `app.ts` words it.** `Session` gains
   `onPartyMembersLeft(fn)`. In its `wire.onWatchable` handler, before it
   replaces the set and before its change handlers run, it calls `fn` with
   the party members the previous set named as controlled and the new one
   does not, by actor id, in the previous set's order, when there are any. A
   set names party members as controlled when it does not say
   `controls_no_party_member`; otherwise it names none.
   - Forced by G4 and one source: `Session` already holds the previous set at
     the one moment it is replaced; a copy of it in `app.ts` would be a
     second.
   - By id, so a rename sends nothing (P7).
   - A page load cannot tell: a `Session` starts with an empty set (P7). A
     redial cannot tell falsely: a reset keeps the set (VTT-457), and the
     new connection's first set equals it unless something changed while it
     was away, which is then true.
   - The toast is set before the change handlers paint, so one paint shows
     it.
10. **D10. Only a player's window is told.** `app.ts`'s handler sets the
    toast only while the role `/api/me` last gave is `player`.
    - Forced by G4: a player demoted to spectator keeps their control
      (`handlePromotion` revokes nothing), and a spectator's set names every
      party member, so its `actors` is not what they control.
    - A spectator holding control promoted back to player: `handlePromotion`
      signals the pump (`rejudge`) and sends the presence frame
      (`announcePromotion`) separately, so the new set races the client's
      `/api/me` read that the presence frame starts. If the read wins, the
      spectator's set (every party member) is compared with the player's
      (their own), and the toast names party members they never played. It
      needs a player demoted while holding party members and promoted back,
      and one wrong toast is its whole consequence. Near-theoretical with a
      small consequence: no code, no test.
11. **D11. The toast's words** (the ticket's second unestablished point).
    `You no longer play ` + the names, in the previous set's order, joined
    by ` and ` + `.`; an empty name is `an unnamed character`.
    - "You no longer play Lera." is the owner's. Two at once happen when two
      revocations land in one wake of the pump (the agent revokes twice in a
      row). P7: `Ama Stone and Bo and an unnamed character`.
    - Not the id, though the chip shows one (VTT-404): a player's window
      names no actor by its id outside the chips and the perch (SPEC-022,
      VTT-475). A preference in its words.
12. **D12. The perch line.** `PerchControl` gains `playsNone?: boolean`;
    `renderPerch` draws `You play no character; the DM can give you one.` in
    a `p` of class `plays-none` under "Whose eyes" when it is set. `app.ts`
    sets it when the perching window's role is `player`, which by `mayPerch`
    means the set says they control none. No style: a preference.
    - It survives a reload: it is drawn from the role and the set, not from
      an event.
13. **D13. One browser beat, E1, in `client/e2e/handover.spec.ts`'s second
    test.** It already revokes Bram and then Ash from Lera and reloads her
    window (P12). E1 asserts the toast after each, the perch line after
    Ash's, and after the reload the line and no toast.
14. **D14. `python3 tools/check-comments.py --write-ledger` runs once, after
    the last code commit,** and only if `check-comments.py main` finds a file
    more than 1.0 under its ceiling.
15. **D15. No Stryker run.** `client/src` changes, so the stored mutation
    report goes out of date. At Task 6 the owner is told which commit it
    measured and the tickets since, and that `task
    check:ts-mutation:full` is his to ask for.

The ticket's first unestablished point, the unseen square's words, is T1.

## Requirements

Allocated at Task 0 with `requirement-id`, each `**OPEN — no test yet**`
until its tests land. The P, C and K labels, and the test labels M, N, U, S,
V and E, are this plan's.

The server:

The P labels here are requirements; the probes under "Measurements this
plan stands on" are a separate series.

- **P1.** "A player's `move_token` of a token whose actor they control, to a
  square the eyes judging their moves do not see, is refused with `You
  cannot move there: you cannot see that square.`"
  `TestQAMGAnUnseenSquareIsRefusedWithOneStringWhateverStandsOnIt`,
  `TestAMoveRightAfterARevocationIsNotJudgedByTheLostCharacter`, with T1.
  Red under K2.
- **P2.** "A player's `move_token` of a token whose actor they control, to a
  square the eyes judging their moves see and `engine.State.Blocked` blocks,
  is refused with `You cannot move there: `, the reason as
  `describeBlockage` words it, and `.`." M1. Red under K1.
- **P3.** "A player's `add_narration` in character, and a player's
  `add_whisper` that names no actor to whisper to, from a party member of
  theirs whose name is empty, is refused with a text that begins `This
  character has no name.` and names no package, and the refusal wraps
  `ErrUnauthorized`." N1. Red under K3, K4.

The client:

- **C1.** "A browser window shows a refused command's error as its toast,
  with nothing before it." U1, U2. Red under K5, K6.
- **C2.** "A browser window whose role the client last read as `player`,
  sent a `WatchableChanged` that names as controlled no longer some party
  members the previous set named so, shows the toast `You no longer play `,
  their names in the previous set's order joined by ` and `, each `an
  unnamed character` when its name is empty, and `.`; a set names party
  members as controlled only when it does not say the participant controls
  none." S1, U3, E1. Red under K7, K8, K11.
- **C3.** "No other `WatchableChanged` sets that toast: not the first a page
  is sent, not one naming the same party members under other names, and
  none in a window whose role the client last read as not `player`." S1,
  U4, U5, E1. Red under K9, K10.
- **C4.** "The perch of a window whose role the client last read as `player`
  reads `You play no character; the DM can give you one.`, and a
  spectator's perch does not." V1, U6, E1. Red under K12.

Kept, wording still true:
- **VTT-156**, **VTT-472**, **VTT-506**, **VTT-589**: their rules do not
  change; T1 is still one string, and the unseen target keeps `Resolve`'s
  answer.
- **VTT-495**, **VTT-588**: a nameless party member is still refused.
- **VTT-456**, **VTT-340**, **VTT-404**, **VTT-457**, **VTT-458**,
  **VTT-407**, **VTT-517**, **VTT-157**, **VTT-453**: read and kept; their
  tests pass, those of VTT-407, VTT-517, VTT-157 and VTT-453 with a pin
  rewritten, and none renamed.
- **VTT-132**, **VTT-288**, and every row on `Resolve`'s refusals: their
  words are kept (item 7).

The sort, and what it rejected:
- "The move gate's refusal of a player's move, for a square they cannot see
  or one that blocks, and the refusal of a line or a whisper from a party
  member with no name, name no package, no id and no participant id": P1 to
  P3, each a text a test reads.
- "a player is told which party member left": C2 and C3.
- The promotion race of D10: no row.
- The exact wording of each text is in the rows, since the words are the
  feature.

| Ticket item | Observed by |
|---|---|
| 1 | M1 |
| 2 | the VTT-156 tests with T1 |
| 3 | N1 |
| 4 | U1, U2 |
| 5 | S1, U3, E1 |
| 6 | V1, U6, E1 |
| 7 | every test outside D3's pins passing unedited; `git diff main -- internal/rules rulesets scenarios` empty |
| 8 | Task 6 |

## The existing tests that change

- **The move gate's five pins** (P5): `qaMGUnseen` and `qaMGBlocked` in
  `qa_move_gate_test.go`, its crate line gaining `.`; `unseenSquare` in
  `watch_test.go`; `unseen` in `seat_internal_test.go`; `want` in
  `server_test.go`'s `TestAPlayerCannotWalkOntoBlockingSceneryAndTheRefusalNamesIt`.
  These turn 25 tests green again.
- **`client/test/app.test.ts`**, four that assert the prefix (P6):
  - "a refused command is reported verbatim rather than silently doing
    nothing": the toast `toBe("not authorized")` (U1);
  - "the indicator follows the SERVER's answer, never the click" and "an
    accepted perch CLEARS the refusal the last one left up": `not a party
    member` exactly (U2); the second test's comment loses its quotation of
    the prefix;
  - "a refused Start session leaves the name typed beside the refusal": `a
    session is already open` exactly.
- **G6's four**, in the same file:
  - "before anything connects the page says connecting, with no toast":
    `not.toContain("refused")` becomes `.toast` is null;
  - "a command that succeeds clears the toast rather than leaving one up"
    and the accepted-perch test: the line goes, since each asserts `.toast`
    is null beside it, and the accepted-perch test shows its refusal first;
  - "a warning on an ok=true result reaches the DM's toast": the line goes;
    the test asserts the toast's whole text two lines above.
  - The comment in the warning test quoting `` `refused: ${res.error}` ``
    is history; it goes.
- **`client/e2e/refusals.spec.ts`**'s first test (D7).
- No other existing test changes: P5 and P6 ran every Go package and client
  test the change reaches.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Sign-off is given, with Q2 answered and the words of D3, D11 and D12
    accepted or reworded.
  - Allocate P1 to P3 and C1 to C4 with `requirement-id`.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean; the rows exist; the chain passes.

### Task 1: the gateway's words (P1 to P3; D3, D4, D5; items 1, 2, 3)

- **Files:** `internal/gateway/server.go` (`handleCommand`),
  `internal/gateway/authz.go` (`mayVoice`, `mayWhisper`, `plainRefusal`);
  the five pins; `internal/gateway/plain_refusal_test.go`, new, package
  `gateway_test`; `docs/requirements.md`.
- **New tests**, each with a literal name and its id line:
  - **M1** (P2), `TestAPlayersMoveOntoWhatTheySeeIsRefusedInPlainWords`: a
    seen wall, a seen closed door and seen scenery, each refused with its
    exact text; the control, a seen floor square, accepted.
  - **N1** (P3), `TestANamelessCharacterIsToldInPlainWordsWhyItCannotSpeakOrWhisper`:
    a line in character and a whisper to the DM from the player's nameless
    party member, each refused over the wire with T3's or T4's exact text,
    and through `Authorize` with `errors.Is(err, gateway.ErrUnauthorized)`;
    the control, the same from their named party member, accepted.
- **Order:** the tests, red; the texts and `plainRefusal`; the pins; the
  breaks.
- **True when:** `go test ./internal/gateway/ ./cmd/vtt/` passes, the
  scenario library included; `go vet` is clean; `share_of` holds every file
  at or under its ceiling; `git diff main -- internal/rules rulesets
  scenarios` prints nothing.
- **Breaks**, each undone by its inverse edit:
  - **K1:** the blocked text keeps `gateway: cannot move there — `. M1 red.
  - **K2:** T1 loses its `.`.
    `TestQAMGAnUnseenSquareIsRefusedWithOneStringWhateverStandsOnIt` and
    `TestQAMGASeenBlockedSquareIsRefusedWithTheObstructionNamed` red;
    `TestAPlayerCannotProbeTheDarkWithMoveCommands` stays green, since it
    compares the answers with one another.
  - **K3:** `plainRefusal.Unwrap` returns nil. N1 and
    `TestAPlayerCannotSpeakAsAPartyMemberWithNoName` red.
  - **K4:** `mayWhisper`'s nameless refusal goes back to
    `fmt.Errorf("%w: …", ErrUnauthorized)`. N1 red.

### Task 2: the window (C1 to C4; D7 to D12; items 4, 5, 6)

- **Files:** `client/src/app.ts` (`act`, `switchTo`, the left hook, the
  perch's flag), `client/src/session.ts` (`onPartyMembersLeft`),
  `client/src/view/spectator.ts` (`PerchControl`, `renderPerch`);
  `client/test/app.test.ts`, `session.test.ts`, `spectator-view.test.ts`;
  `client/e2e/refusals.spec.ts`; `cmd/vtt/webdist`; `docs/requirements.md`.
- **New and rewritten tests:**
  - **U1** (C1), "a refused command is reported verbatim rather than silently
    doing nothing", rewritten: red today.
  - **U2** (C1), the two perch-refusal tests, rewritten.
  - **S1** (C2, C3), `session.test.ts`, "a set that stops naming a party
    member as controlled tells who left, and a first, an equal or a renamed
    set tells nothing": case 1, case 2, two at once, a rename, a first set, a
    grant after none, over `gatewayServing`.
  - **U3** (C2), `app.test.ts`, "a player is told You no longer play and the
    names of the party members that left, and an unnamed one as an unnamed
    character": case 1 (the perch follows), case 2, two at once.
  - **U4** (C3), "a spectator's window is told nothing when a party member
    leaves the set", a spectator whose set says they control one; the
    control, a player at the same table told.
  - **U5** (C3), "a fresh page and a redial sent the same set show no
    toast": the control, a set after the redial that drops one, told.
  - **U6** (C4), "a player who controls no party member is told so on the
    perch, and a spectator is not".
  - **V1** (C4), `spectator-view.test.ts`, "the perch says you play no
    character when told to, and nothing when not".
- **Order:** the tests, red; the code; G6's four and the rewrites; the
  breaks; `task build:client`.
- **True when:** `env -u CLAUDECODE bun test client/test contract` passes;
  the typecheck is green; the four files are at 100; `share_of` holds;
  `python3 tools/ts_mutation_inputs_test.py` passes; `git grep -n
  'refused: ' -- client/src` prints nothing.
- **Breaks:**
  - **K5:** `act` keeps the prefix. U1 red.
  - **K6:** `switchTo` keeps the prefix. U2 red.
  - **K7:** `Session` ignores `controls_no_party_member`. S1's case 1 and
    U3 red.
  - **K8:** the names joined by `, `. U3 red.
  - **K9:** compared by name, not id. S1's rename red.
  - **K10:** the role check removed. U4 red.
  - **K11:** the unnamed fallback removed. U3 red.
  - **K12:** the perch line drawn for every perch. U6 and V1 red.

### Task 3: the browser (item 5; D13)

- **Files:** `client/e2e/handover.spec.ts`.
- **E1** (C2, C3, C4), in "the DM takes a player's only character back, and
  the player chooses another shoulder":
  - after Bram's revoke, Lera's `.toast` reads "You no longer play Bram.";
  - after Ash's, "You no longer play Ash.", and `.perch .plays-none` reads
    "You play no character; the DM can give you one.";
  - after the reload, the line is there and there is no `.toast`;
  - a screenshot after Ash's revoke.
- **Order:** `task build:client`; check the machine's load; `task e2e` whole,
  `refusals.spec.ts` included; then `bunx playwright test handover.spec.ts`
  again.
- **True when:** every spec passes, twice for `handover.spec.ts`.

### Task 4: the report

- **Do:** write `docs/reports/2026-11-02-a-refusal-says-why.md` and give it
  its reading review.
- **It covers:** each "Done looks like" item with its observation; K1 to
  K12 with what went red; Rule 9; the gaps and Q2's
  answer; the narrowing and what it left to the rules engine's track; P5
  and P6, said once; Phase 4a's adjudications under their own heading. Room
  for the gate's figures.
- **True when:** it answers the ticket's eight items in their numbering.

### Task 5: the specifications

With the `specification` skill, from the report:
- **SPEC-013:** "The player's move gate": T1 and T2's words. The
  `ErrUnauthorized` paragraph: the nameless refusals are `plainRefusal`,
  which unwraps to it; the bullets for `mayVoice` and `mayWhisper` name the
  words. Status: `plain_refusal_test.go`; Requirements: P1 to P3.
- **SPEC-022:** `act` shows a refusal as the toast of its error, nothing
  before it; a paragraph on the toast when a set stops naming a party member
  as controlled, from `Session.onPartyMembersLeft`; the perch's line for a
  player; Status and Requirements: C1 to C4.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer; every id in each Requirements section
  resolves; `git grep -n "cannot move there —\|refused: " docs/specifications`
  prints nothing.

### Task 6: the ledger, the gate, whole, and the commits

- **Do:**
  - `python3 tools/check-comments.py main`; `--write-ledger` once if a file
    is more than 1.0 under its ceiling (D14).
  - Check free space and the load. Run `task check` whole on the last code
    commit, with `start_new_session=True`, all output to a file; require the
    gate's own completion line and `check:ts-mutation`'s own line.
  - Write the figures into the report; commit the report, then the
    specifications.
  - Tell the owner D15's reminder.
- **True when:** the gate is green; `check:ts-mutation` printed that the
  client's mutation was not measured, naming its commit and this ticket; the
  log holds no Stryker line.
- Push and merge on the owner's word.

## For Phase 4a

Hand the QA agent:
- P1 to P3, C1 to C4, and the kept rows' texts;
- SPEC-013's and SPEC-022's changed paragraphs;
- the surfaces as signatures and names, never the source:
  - the wire: a `ClientCommand` with `moveToken`, `addNarration`,
    `addWhisper`, `setViewpoint`, and its `CommandResult`; a `ServerFrame`
    with `watchableChanged`;
  - `go doc ./internal/gateway`: `Authorize`, `ErrUnauthorized`, `New`;
  - `newGWFixture`, `sendCommand`, `readResult` in `internal/gateway`'s
    tests;
  - `Session` and `onPartyMembersLeft`, `boot`, `renderSpectator` and
    `PerchControl`.

Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- its files: `internal/gateway/qa_plain_refusal_test.go`, package
  `gateway_test`, held to the default ceiling, 25.0; and
  `client/test/qa-refusal-words.test.ts`. `go-arch-lint` runs in the hook
  and refuses an import across layers;
- `semgrep` bans game words in every Go file under `internal/` and `cmd/`,
  tests included;
- a client test reads a file outside `client/` only from `contract/testdata/`
  or `contract/gen/`, never `cmd/vtt/tools.json`;
- every refusal outside T1 to T4 keeps its words: `Resolve`'s, the sight
  gate's unseen target, and the race refusals D1 lists. None of these is a
  finding;
- the promotion race of D10 is known and left open by the owner's rule on
  near-theoretical cases. A test of it is not a finding;
- "no toast" comes with a set that tells at the same table;
- give each test a literal name and its rule's id;
- measure with `env -u CLAUDECODE bun test <file>`;
- the tree is hashed before and after it.

## For Phase 4b

Hand the reviewer the diff with `git diff HEAD` and tell it:
- look at whether each scope claim ("only", "never", "every", "no other")
  names the search that bounds it;
- check each comment under `internal/`, `cmd/` and `client/src` is a warning
  or a pointer (rule 10), and each citation a name (rule 8);
- check every new text against D3's table and the register's rows, word for
  word, and that no refusal outside D3 changed: `git diff main --
  internal/rules rulesets scenarios` prints nothing.
