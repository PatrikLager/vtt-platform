# Plan: the DM whispers to a character, and a character to the DM

- **Ticket:**
  `docs/superpowers/specs/2026-11-01-whispering-to-and-from-characters-design.md`.
- **Verdict:** passes, with gaps G1 to G12 (below).
  - Verified 2026-11-01 by an agent that did not write the ticket,
    read-only against main `73bc88f` and the uncommitted ticket.
  - Probes ran in copies of the tree outside the repository. P4's and P5's
    prototype is a scratch edit of the two `.proto` files, their
    regeneration, `engine.Apply`'s arm and `perceive`'s arm; nothing in the
    repository was edited.
  - `task check`, `task e2e`, Stryker and gremlins were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-11-01-whispering-to-and-from-characters.md`.
- **Base:** main `73bc88f`.
- **Branch:** `feat/whispering-to-and-from-characters`.
- **Owner's rulings taken in:**
  - 2026-10-22, B-2: a whisper is addressed to a CHARACTER and is shown only
    in that character's log ("Bara till Tobbe, det var han som hörde det").
    A whisper to a PLAYER is shown in every character that player holds.
    When a player whispers to the DM it is the character who whispers
    ("Tobbe viskar"), shown only in that character's log and to the DM. The
    server sends a whisper only to its recipients, never to everyone with a
    filter in the client.
  - 2026-11-01, "/w och /dm i Say-rutan": the DM writes `/w Tobbe text` in
    the console's Say box, with quotes round a name that has a space; a
    player's name in place of a character's goes to every character that
    player controls. A player writes `/dm text` in the panel's Say box,
    whispered from the character the window watches. Same pattern as `/ooc`
    (`readLine`). The agent gets a tool of its own.
  - 2026-11-01, "Alla som ser genom Tobbe": a whisper to Tobbe lands in
    Tobbe's character log like everything he perceives, so his controllers
    and anyone perched on him see it. No second delivery path.
  - Never ruled, and a non-goal: one player whispering to another.
  - Standing rulings that bind:
    - narration reaches every active character log (VTT-362);
    - a table has one DM in practice, the agent or a human (2026-10-15);
    - a field a player can write text into joins SPEC-018's table, and every
      other field outside it stays unbounded on purpose (2026-10-09);
    - a refused command keeps what was typed (SPEC-022);
    - delete the old solution first (2026-09-04);
    - a near-theoretical edge case with a small consequence gets no code, no
      test and no debt entry (2026-10-21);
    - the client's full mutation run is the owner's to ask for (2026-10-26).
- **Signed off:** 2026-11-01, with Q1 answered (a), always as the DM,
  and Q2 answered one ticket, "Ja, kör som ett ärende". D15 was told to the
  owner, not asked. G1 was fixed in the ticket before Task 0's commit.
- **The ids, allocated at Task 0:** W1 to W10 are VTT-582 to VTT-591, C1 to
  C6 are VTT-592 to VTT-597, and N529 is VTT-598. VTT-529 is withdrawn.
  Under Q1(a) there is no W11. Task 0's review reworded W1, W2, W8, W10,
  C2, C4 and C5 before their commit; the rows below carry the new words.

## The verdict, check by check

1. **Every path resolves. By command** (P1).
   - Every file the ticket names exists, and every symbol stands where it
     says: `AddNarration`, `NarrationAdded` and `NoteVisibility` in the
     contract; `Projection.classify` and its "addressed to the table";
     `readLine`; `commandRoles`, `mayVoice` and its "ask the DM for one";
     `voice`; `describe`; VTT-362; the five specifications; and MapTool's
     `WhisperMacro`, `ToGMMacro` and `TextMessage`'s `WHISPER` channel.
   - **D-2 and D-3 resolve to nothing** in the alpha findings, which hold
     D-4 alone (G1).
2. **"Done" is an observation.** Items 1 to 8 name tests or a tool that do
   not exist. `grep -rli whisper contract/vtt internal client/src` finds
   nothing (P2), so each fails today. Item 9 is the gate.
3. **Each rule is breakable. A reading.** Each of the six rules has an
   observation that goes red; see "Requirements" and the breaks. Two are
   thin as worded (G4, G5). The "done" items carry four rules the rules
   section omits (G6).
4. **The scope matches the claim. By command, then a reading.**
   - A prototype of the two oneof arms alone (P4) turns these red:
     `toolgen`'s manifest test; the gateway's role-table and conversion
     tests; every test that builds `mcp.New`, in `internal/mcp` and
     `cmd/vtt`; the projection's completeness test; five client tests; and
     the client typecheck.
   - So the MCP tool is required (G8), and the command arm cannot land
     without its client surface (D14).
   - Files the ticket does not name: G2.
5. **No recorded decision is contradicted silently. A reading.**
   - These stay true: VTT-362, VTT-505 and SPEC-016's "narration is
     addressed to the table", since a whisper is another payload; and
     VTT-503, since no whisper field is written by the server (D6).
   - B-1, "text is the DM's tool", agrees.
   - SPEC-018's closed series is not reopened: the whisper's text joins by
     SPEC-018's own first criterion.
   - SPEC-013's "Three checks for every role" becomes four.
   - **VTT-529 becomes false as worded** (G3).
6. **The records it moves. By command, then a reading.** The section is
   present, and its five paths resolve. The reading: SPEC-021's consequences
   on narration a paused or dropped log misses stay true (D16). Nothing else
   becomes false.

## The gaps this plan inherits

- **G1. D-2 and D-3 cite nothing.** The finding is B-2. Every record this
  plan writes cites B-2.
- **G2. Files the ticket does not name.**
  - `client/src/fold.ts` (the arm SPEC-018 mirrors);
  - `client/src/style.css` (`.whisper`);
  - `client/src/app.ts` (the panel's `notify`, G9);
  - `contract/testdata/expected_tools.json`;
    `contract/testdata/add_whisper_command.json` and
    `whisper_added_envelope.json`, new; `contract/roundtrip_test.go`;
  - `internal/gateway/authz_test.go` (`commandFor`, the table's rows) and
    `convert_test.go`;
  - `internal/mcp/tools_test.go` (`wantCommandToolNames`) and
    `tools_internal_test.go` (`theCommandTools`);
  - `internal/campaign/campaign_test.go` (`cenv` panics on a payload it has
    no case for);
  - `client/test/command-surface.test.ts`,
    `client/test/qa-what-a-window-shows.test.ts` (a
    `Record<PayloadCase, Envelope>` the typecheck refuses, P4),
    `client/test/spectator-view.test.ts` (the describe-completeness tables);
  - `cmd/vtt/webdist`; `tools/comment-ceilings.txt` (D18).
  - Not changed: `internal/gateway/server.go`, since `add_whisper` is one
    envelope through `ToEvent`; and `voice` in `convert.go` (D6).
- **G3. VTT-529 becomes false as worded.** It sends "a line whose first word
  is not `/ooc` in character"; a `/w` line is neither. It is withdrawn and
  succeeded by N529 at Task 0, and its 16 citing lines move to N529 (D21,
  P12). VTT-500 is read and kept: it was split from VTT-501's `/ooc` in one
  plan, so its Send is the line in character, and a `/dm` line still goes
  from the party member the panel acts as.
- **G4. "A whisper addressed to a player reaches every party member that
  player controls" names no layer and no moment.** The contract carries
  actor ids alone (D2, D7), so the console holds this rule, as the console
  knows the table when the line is sent (C2). The agent names each party
  member, and its tool says so (D13).
- **G5. "A Say line that begins with a whisper command is never said aloud"
  names no box, no command and no case.** Pinned by D10, D11 and D12: both
  boxes, `/w` and `/dm`, any case.
- **G6. Rules the "done" items imply and the rules section omits:** the
  Story's line (item 7, C6); the fold's refusal of an actor it has not folded
  (item 4, W5); the narrowing (W3); the nameless party member (W7); and that
  a DM's whisper names party members alone (W6). P5: a whisper to a creature
  folds and reaches no character log, so it would vanish without a word.
- **G7. The four unestablished points.** The first is Q1. The other three are
  settled from the code: D7, D4 and D8.
- **G8. The MCP tool is required.** `mcp.New` refuses to start ("oneof
  fields with no tools.json entry [add_whisper]", P4).
- **G9. The panel has no way to tell a player anything.** `renderPlayerPanel`
  takes no `notify`. D12 adds one.
- **G10. Every touched file sits within 0.9 of its comment ceiling, most
  within 0.1** (P8). `narration_test.go` and `every_log_told_test.go` are at
  0.0, so the new tests go in new files. Eight client files are held at 100%
  line coverage (P9).
- **G11. Item 7 needs four people, and the e2e fixture mints three** (P11).
  D19 admits the fourth through the share link.
- **G12. "No command or event in the contract is addressed to one
  character"** is a scope claim its named search does not bound. It was read
  against `classify`, and it is true: every arm forwards to every viewer,
  withholds from every viewer, or decides by a note's visibility or by
  sight.

## Measurements this plan stands on

All on 2026-11-01 at `73bc88f`, in copies outside the repository.

| # | Probe | Result |
|---|---|---|
| P1 | `test -e` on every path the ticket names; `grep -c` for every symbol | all resolve; `grep -n "D-2\b\|D-3\b"` in the findings prints nothing |
| P2 | `grep -rli whisper contract/vtt internal client/src` | nothing |
| P3 | Baseline, untouched copy: `go test` of `contract`, `tools/toolgen`, `internal/{gateway,mcp,perceive,engine,campaign,eventgen}`, `cmd/vtt`; `env -u CLAUDECODE bun test client/test contract`; `bunx tsc --noEmit -p client/tsconfig.json` | all pass; 1872 client tests pass; typecheck clean |
| P4 | Prototype: `AddWhisper add_whisper = 36` and `WhisperAdded whisper_added = 35`, each `{ text = 1; repeated to_actor_ids = 2; from_actor_id = 3; as = 4 }`; `go tool buf generate`; toolgen; the P3 runs | Go red: `TestManifestCoversAllCommandMessages` ("command message vtt.v1.AddWhisper has no toolgen manifest entry"); `TestEveryClientCommandHasRoleCells/add_whisper` ("commandFor: unknown command name"); `TestEveryClientCommandConverts/add_whisper`; every `mcp.New` test in `internal/mcp` and `cmd/vtt` ("oneof fields with no tools.json entry [add_whisper]"); `TestEveryEnvelopePayloadArmHasAnExplicitRuling` ("whisper_added (field 35) has no explicit ruling"). Green: `contract`, `engine`, `campaign`, `eventgen`. Client red: "describe renders a real label for every event kind, not the fallback", "no Story line is a payload's name, an actor or token id, or someone joining", "qaWS: VTT-476 VTT-475 no line of any case is a payload's name or an id", "every command the contract defines has a declared human surface", "every command with a human surface has a builder in commands.ts". Typecheck: `describe` lacks an ending return; `qa-what-a-window-shows.test.ts`'s record lacks `whisperAdded` |
| P5 | Prototype on P4: `engine.Apply` arm (text 1 to 8192, every named actor folded); `sightView.eyes`; `classify` arm (forward when an eye is `from_actor_id` or among `to_actor_ids`); in `Event`, the forwarded copy's `to_actor_ids` cut to the eyes. Scratch campaign test: party members tobbe, lera and brann, brann with no token; the DM to tobbe; the DM to brann and tobbe; tobbe to the DM, issued as a player; party member `late` added after; a whisper to a creature | tobbe's log: three whispers, each naming tobbe alone; brann's: one, naming brann alone; lera's and late's: none; the creature's whisper folds in the world and reaches no log; each log folds at every prefix; each forwarded whisper keeps the world event's id and carries no participant id or role. The fold refuses an unknown actor and an empty text. Break 1, the cut removed: tobbe's and brann's read "brann,tobbe". Break 2, `classify` forwards every whisper: lera and brann are written all three, and lera's copy of the DM's whisper names nobody |
| P6 | P5's code; `go test ./internal/perceive/ ./internal/engine/ ./internal/campaign/ ./internal/harness/ -count=1` | all pass |
| P7 | `grep -rn 'Eyes{' --include='*.go' internal cmd \| grep -v _test.go` | `eyesOf` (`Shoulder`), `opening` (no eyes), `canSee` (`Shoulder`) and the sight gate in `handleCommand` (`Shoulder`); `Eyes{Controller` appears in eight `internal/perceive` test files and nowhere else |
| P8 | `measure` and `share_of` from `tools/check-comments.py` against `tools/comment-ceilings.txt` | `apply.go` 32.74 (32.8); `projection.go` 17.83 (17.9); `authz.go` 12.46 (12.6); `convert.go` 7.64 (7.7); `player.ts` 46.43 (46.5); `commands.ts` 40.58 (41.1); `dm.ts` 35.84 (36.7); `view/player.ts` 21.05 (21.1); `spectator.ts` 40.19 (40.4); `fold.ts` 32.69 (32.7); `authz_test.go` 34.45 (34.7); `convert_test.go` 13.08 (13.2); `mcp/tools_test.go` 26.60 (26.7); `mcp/tools_internal_test.go` 23.57 (23.8); `roster_test.go` 7.01 (7.1); `narration_test.go` and `every_log_told_test.go` 0.00 (0.0) |
| P9 | `tools/coverage-thresholds.txt`, `tools/ts-coverage-thresholds.txt` | `internal/engine` 97.5, `perceive` 94.5, `campaign` 94.7, `gateway` 93.5, `mcp` 87.5, `tools/toolgen` 85.0; `app.ts`, `commands.ts`, `fold.ts`, `player.ts`, `view/dm.ts`, `view/feed.ts`, `view/player.ts`, `view/spectator.ts` 100 |
| P10 | `~/dev/RPTool/maptool` at `f4b7fef6c` | see Rule 9 |
| P11 | `client/e2e/setup.ts` | mints `DM` (dm), `Lera` (player), `Watcher` (spectator), one table per spec file |
| P12 | `git grep -c VTT-529 -- client internal cmd docs/specifications` | 16 lines in 8 files: `client/e2e/refusals.spec.ts` 1, `client/src/view/dm.ts` 1, `client/test/app.test.ts` 1, `dm-view.test.ts` 2, `qa-console-say.test.ts` 5, `qa-refused-line.test.ts` 3, `qa-speaking.test.ts` 2, `spectator-view.test.ts` 1; and SPEC-022 2 |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone (D18).
  - 3: additive. A new message and arm at the next free number of each
    oneof: `add_whisper = 36`, `whisper_added = 35`. The gaps at 11, 16 and
    17 stay unused (SPEC-007). Regenerate with `task generate:contract`;
    generated code is committed. `check:breaking` does not enforce this.
  - 4: one fold. A whisper appends through `campaign.Append`, and its arm in
    `engine.Apply` changes no state.
  - 5: no game-system vocabulary.
  - 8: cite names, never `file:line` or `.superpowers/`; cite B-2, not D-2
    or D-3.
  - 9: answered below.
  - 10: a comment under `internal/`, `cmd/` or `client/src` is a warning or
    a pointer. History and argument go to the report.
- **The comment ceilings** (P8). Each file is measured with `share_of`
  before its commit. A test cites its rule on a citation-only line, which
  counts in no share. New tests go in new files where a file is at 0.0.
- **`check:new-prose`** holds added lines of `.go` and `.ts` files to an
  85-column band and the SHORT rule, test files included.
- **`go-arch-lint`.** `internal/perceive` imports nothing of the module but
  itself, `contract`, `engine` and `sight`, its tests included; a campaign
  test may import `perceive`, a perceive test may not import `campaign`.
- **Coverage floors** (P9). Every new branch has a test that reaches it; the
  eight client files stay at 100.
- **The Go mutation gate runs in `task check`.** Each condition added has a
  test that goes red when it is negated.
- **Nobody starts Stryker** (D20).
- **A negative assertion needs its positive.** "Lera's log holds no whisper"
  comes with a narration both logs hold; "refused" comes with an accepted
  whisper at the same table; "nothing appended" reads the head before and
  after.
- **Compare envelopes with `proto.Equal`.** A character log's opening,
  resume and projection-made entries carry no `EventId`; a forwarded entry
  shares the world event's id.
- **A client test reads a file outside `client/` only if
  `tools/ts-mutation-inputs.py` hashes it:** `contract/testdata/` or
  `contract/gen/`, never `cmd/vtt/tools.json`.
  `tools/ts_mutation_inputs_test.py` catches a miss only inside
  `task check`.
- **A new function or test goes after a closing brace** (`check:doc-owner`).
- **A test the register cites has a literal name.**
- `bun test` is measured with `env -u CLAUDECODE`.
- **Before every commit:**
  - `python3 tools/check-new-prose.py main`;
  - `python3 tools/check-requirements-chain.py .`;
  - `python3 tools/check-comments.py main`, after staging;
  - for a client commit, `bunx tsc --noEmit -p client/tsconfig.json`, and
    `task build:client` with `cmd/vtt/webdist` committed if it changed;
  - for a client commit, `python3 tools/ts_mutation_inputs_test.py`.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo a break. Use the inverse edit, checked
  with grep, and assert before a break that its broken text is not already
  in the file.
- **The review gate refuses untracked files.** Park scratch files outside the
  tree with `cp` and `cmp`.
- **Hash the tree before and after any reviewer.**
- **The pre-push hook takes minutes.** Give it 600 000 ms, send its whole
  output to a file, and check the machine's load first.

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c` (P10):

- **`/w` (`WhisperMacro`, alias of `whisper`).** The first word is the
  recipient, or a quoted run (`StringUtil.getFirstWord`, `splitNextWord`).
  The name is matched against the connected players by
  `StringUtil.findMatch`: the first whose name STARTS WITH the typed text,
  without case. A player who is not connected is refused
  (`msg.error.playerNotConnected`), and so is oneself (`whisper.toSelf`,
  "Talking to yourself again?"). The recipient is shown "{0} whispers: {1}"
  (`whisper.string`), with the sender's chosen identity
  (`getFrame().getCommandPanel().getIdentity()`), and the sender "You
  whisper to {0}: {1}" (`whisper.you.string`).
- **`/gm` (`ToGMMacro`, alias `togm`).** Sent on the `GM` channel under the
  PLAYER's name: "{0} says to the GMs:" (`togm.saysToGM`), and the sender
  sees "You say to the GMs: {0}" (`togm.self`).
- **`/reply` (`WhisperReplyMacro`, alias `rep`)** whispers to
  `MapTool.getLastWhisperer()`.
- **Distribution.** `MapTool.addServerMessage` receives every message on
  every client and drops a `WHISPER` whose target is not this player's
  name, and a `GM` message for a non-GM. `TextMessage.Channel` has `ALL`,
  `SAY`, `GM`, `ME`, `GROUP`, `WHISPER`, `GMME`, `NOTME`, `NOTGM` and
  `NOTGMME`.

Borrowed: the command word `/w`, the quoted name, matching without case, a
line that names its sender and its recipient, and a whisper to the DM from
the Say box.

Not borrowed:
- **The distribution.** Here the server writes a whisper only into the logs
  it is addressed to (B-2; D3).
- **The prefix match.** "To" would reach whichever of Tobbe and Tom came
  first: a private line to the wrong person. Here a name must equal one
  (D11).
- **The connected-only refusal.** A log keeps what it is written, so a
  whisper to a character whose window is closed is read when it opens.
- **`/gm` under the player's name.** Here the character whispers ("Tobbe
  viskar").
- **A sender's echo.** The DM reads the world log, and a whisperer reads
  their own character's log, so each sees their own whisper with no second
  frame.
- **`/reply`, and the aliases `/whisper` and `/togm`.** The owner named two
  commands (D10). A preference.
- **The whisperer's chosen identity:** Q1.

## Design questions for the owner

1. **Q1. Does the DM's whisper speak under the console's "in character
   as…" name when one is given?** The ticket asks this.
   - **Answered (a), 2026-11-01.**
   - Recommended: **(a) always as the DM.** The line reads "The DM whispers
     to Tobbe: …".
     - `say-as` keeps its name across lines (VTT-521, "Ja, behåll namnet").
       Under (b), a rules aside whispered minutes after the DM voiced a
       goblin goes out as "Goblin whispers to Tobbe". The player is misled
       about who is speaking, which (a) can never do.
     - An NPC's words still fit in the text: `/w Tobbe The goblin hisses:
       run`.
     - Nothing to add. No `as` field, no rule refusing a player's `as`, no
       row in SPEC-018, no field doc for the agent.
   - Alternative (b): **as `say-as` holds it, the DM when blank**, as
     MapTool and the console's own line do (VTT-498).
     - Cost: `as = 4` on both messages. `mayWhisper` refuses a player's
       whisper that carries an `as`, which keeps VTT-503 true. SPEC-018
       gains a "what holds it" row: the DM's and the agent's alone, so it
       stays unbounded under SPEC-018's own rule. Add a field doc, W11
       ("the DM's and the agent's whisper is appended with the `as` the
       command carried"), C2 reads `say-as`, and C6 puts `as` in place of
       "The DM".
     - About fifteen lines and two tests more.
2. **Q2. One ticket and one branch, or split?** The ticket asks this.
   - **Answered one, 2026-11-01.**
   - Recommended: **one.**
     - P4: the command arm cannot land without its tool, its role cells,
       its conversion, the MCP name lists, the client builder and a human
       surface. The surface test refuses an arm without a builder and a
       reachable control.
     - `readLine` is the one reader for both Say boxes (SPEC-022). Growing
       it changes both boxes in one commit (D14).
     - The event alone (Task 1) can land first, and does, on this branch.
     - Ticket 13 touched as many components and ran as one.
   - Alternative: **two tickets**, (A) the event, the command, the tool and
     the console's `/w`, and (B) the panel's `/dm` and the browser test.
     - Cost: (A) ships a player rule no player can reach from a window, and
       (A)'s `readLine` either leaves `/dm` said aloud in the panel or
       changes the panel anyway. Item 7's browser test belongs to (B), so
       (A) has no browser test of its own.

**Noted for the owner, not asked.** A whisper stays in the character log it
was written into (D15). So whoever watches through that character later
reads it too: a spectator who perches on Tobbe next week, or a player the DM
hands Tobbe to. That is "Alla som ser genom Tobbe" with time in it. It
follows from B-2 and SPEC-021, and it is the same for everything else
Tobbe perceived.

## Decisions

The words "Q1(a)" mark what depends on Q1's recommended answer. Under (b),
Q1's own entry says what changes.

1. **D1. The command is `AddWhisper`, arm `add_whisper = 36`; the event is
   `WhisperAdded`, payload `whisper_added = 35`.** Each has `string text =
   1; repeated string to_actor_ids = 2; string from_actor_id = 3;`, and
   `as = 4` only under Q1(b).
   - Additive, at each oneof's next free number (rule 3, SPEC-007).
   - Command imperative, event past tense (CLAUDE.md rule 3). Named as its
     sibling: a whisper is a story entry the log keeps, as `AddNarration`
     and `NarrationAdded` are, and the agent finds `add_whisper` beside
     `add_narration`. Rejected: `Whisper` and `Whispered`, which break that
     pairing.
   - No anchors (D17).
   - Each message's comment is a pointer. `WhisperAdded`'s warns that a
     log is written only the whispers it is addressed to, and only its own
     character among `to_actor_ids` (SPEC-016).
2. **D2. One event, addressed by actor ids; which field is set says the
   direction.** The DM's and the agent's whisper names `to_actor_ids`. A
   party member's whisper to the DM names `from_actor_id`.
   - Forced by: character logs are named by actor id (SPEC-021), and B-2
     addresses a whisper to a character and has a player's come from the
     character. `perceive` knows no participant and no role (VTT-311), so
     a participant id in the event could not be routed.
   - Rejected: two events, which double every arm in both folds, `classify`
     and `describe`. Rejected: a `to_dm` bool. protojson drops a false bool,
     and a dropped flag would show a player's whisper as the DM's.
   - The speaker of a party member's whisper is `from_actor_id`, named by
     each window's namer (D9). Nothing in the event is written by the
     server, so `voice` does not change and VTT-503 stays true.
3. **D3. The routing is `classify`'s, by the log's eyes.** `classify`
   forwards a `WhisperAdded` when the look's eyes include its
   `from_actor_id` or one of its `to_actor_ids`, and withholds it
   otherwise. `sightView` gains `eyes`, the actors `eyes` answered, set in
   `look`.
   - Forced by: "Alla som ser genom Tobbe", meaning one delivery path, the
     log. Forced also by `TestEveryEnvelopePayloadArmHasAnExplicitRuling`.
     The look's `sees` cannot stand in for the eyes, since it holds every
     actor in sight.
   - **A `Controller` seat.** No production caller sets `Eyes.Controller`
     (P7). Every character log is decided through its own actor as a
     `Shoulder` (`eyesOf`). The rule needs no case for it: under a
     `Controller` the eyes are every actor the participant controls, and a
     whisper to any of them is forwarded. A perceive test pins both kinds,
     since the completeness test runs under `Controller`.
   - A party member with no token still hears: `eyes` answers a party
     member as a shoulder whether or not it stands on the board (P5, brann).
   - P5 measured it, and break 2 shows the projection is the only thing in
     the way.
4. **D4. Each log is written only its own character among `to_actor_ids`**
   (the ticket's third unestablished point).
   - The forwarded copy's `to_actor_ids` keeps only the look's eyes. That
     takes about six lines in `Event` after `forwardable`, on the clone.
   - Forced by SPEC-016's rule that a log is written of an event only what
     its eyes perceive, and by B-2's "det var han som hörde det". That
     Brann heard the same whisper is the DM's knowledge, not Tobbe's.
   - The ids that reach a log are party members, on every log's roster
     already, so the ticket's worry, "neither may learn of a character
     outside the whisper", cannot arise. VTT-505's reason, a creature's id
     in a log that never saw it, cannot arise either (W6).
   - Rejected: the whole list. A spectator perched on Tobbe would read that
     Brann was whispered the same. Narrowing costs one test.
   - P5 measured it; break 1 goes red.
5. **D5. The fold refuses a whisper whose text is empty or over 8192 bytes,
   and one naming an actor it has not folded.** This applies to
   `engine.Apply` and `client/src/fold.ts`, and the arm changes no state.
   - Forced by SPEC-018. A player writes the text, so it meets the first
     criterion and joins the table, beside `NarrationAdded.text`, at
     `maxTextBytes`, not empty.
   - `from_actor_id` is written by a player too. It stays out of the table
     because `engine.Apply` requires it to name a folded actor: the second
     table's first row, which `to_actor_ids` joins as well.
   - The fold does not ask whether an actor is a party member. That is
     `mayWhisper`'s (D6), and a validator never sits in the fold (SPEC-013).
   - P5: every character log folds its narrowed whisper at every prefix.
     Each log holds its own character, introduced at its opening.
6. **D6. Who may whisper, and what.** `commandRoles` gives `add_whisper` to
   the dm, the agent and the player, and no cell to the spectator.
   `playerRules` holds `unrestricted`. `mayWhisper`, beside `mayVoice`, runs
   in `Authorize` for every role:
   - **dm and agent:** refused when `from_actor_id` is set (as VTT-499
     refuses an actor id from them), when `to_actor_ids` is empty, or when
     any id is not an actor of the snapshot that `engine.IsPartyMember`
     accepts.
   - **player:** refused when `to_actor_ids` is not empty ("a player cannot
     whisper to a character"). Refused with one text when `from_actor_id`
     is not `ownPartyMember`, whether absent, not theirs or not a party
     member, since a varying refusal tells which ids exist (VTT-506's
     reason). Then refused when that party member has no name (D8).
   - Forced by SPEC-013's own precedent. Whom a line speaks as is decided in
     `Authorize` for every role (`mayVoice`), not in a validator. The
     whisper's check is the same kind.
   - A refusal appends nothing (VTT-162). The handler is the generic one:
     `ToEvent` gains an arm carrying the three fields as they came, and
     `server.go` does not change.
7. **D7. The console turns a player's name into party member ids when the
   line is sent** (the ticket's second unestablished point). The command
   and the event carry actor ids alone.
   - Forced by D2: the event must name actors anyway. The console already
     holds the world and the people (`known`), and the agent names actor ids
     too, so the server needs no participant-to-character path.
   - **A grant in between.** A grant that lands after the console painted
     and before the append is missed: the whisper goes to the characters the
     console showed. That takes the one DM (2026-10-15) granting a character
     to the same player, from a second window, while sending them a whisper.
     The consequence is one character missing a whisper, and the DM's own
     Story names whom it reached. Near-theoretical with a small
     consequence: no code, no test, no debt entry (2026-10-21).
   - Rejected: a `to_participant_ids` field that the gateway expands from
     the snapshot. That is a second addressing path through the contract,
     the gateway, the tool and SPEC-013, for that window alone.
8. **D8. A party member with no name cannot whisper to the DM** (the
   ticket's fourth unestablished point). It is refused with `mayVoice`'s
   words, "this party member has no name; ask the DM for one, or speak out
   of character".
   - Forced by VTT-475: a window names no actor by its id. The DM's window
     would show the whisperer as "a creature" (`UNNAMED`), so the DM could
     not tell who whispered. "Tobbe viskar" names the character.
   - The same rule as a line in character (VTT-495), and the player can
     still ask for a name out of character.
9. **D9. What each window's Story shows.** `describe` words it, and
   `renderFeed` draws it with the class `whisper` where it draws every other
   entry `mechanical`. `style.css` gives `.whisper` its look. `feed.ts` does
   not change: it makes every payload but narration a beat, so a whisper is
   one, and joins an anchored narration's beat only when that narration's
   range covers it, as any event does. The Events panel, which is the DM's
   alone, shows the same words.

   | Window | DM to Tobbe | DM to Kim's Tobbe and Brann | Tobbe to the DM |
   |---|---|---|---|
   | The DM's and the agent's (world log) | The DM whispers to Tobbe: … | The DM whispers to Brann, Tobbe: … | Tobbe whispers to the DM: … |
   | Whoever watches through Tobbe | The DM whispers to Tobbe: … | The DM whispers to Tobbe: … (D4) | Tobbe whispers to the DM: … |
   | Whoever watches through Brann | nothing | The DM whispers to Brann: … | nothing |
   | Whoever watches through Lera | nothing | nothing | nothing |

   - Names come from the window's namer, `names.actor`, so a whisper names
     a party member as the window names it everywhere else (VTT-475,
     VTT-476). They are joined by ", " in the order the event holds them:
     the console's actor-id order (D11).
   - "The DM" stands for the agent too: a table has one DM.
   - The words are proposed; C6 pins their parts and the class.
10. **D10. `readLine` grows to read the whole Say line.** It returns one of
    `{ kind: "say", text }`, `{ kind: "ooc", text }`,
    `{ kind: "whisper", name, text }` and `{ kind: "toDM", text }`, each
    `text` trimmed.
    - One reader for both boxes, because SPEC-022 has both boxes read a line
      "in the console and in a player's panel alike". Asked for by the
      ruling, "same pattern as /ooc".
    - `/w` and `/dm` are matched as the first word **in any case**. Forced
      by the consequence: `/W` typed with caps lock would be said aloud to
      every character, the leak this ticket exists to stop. `/ooc` stays
      exact (VTT-501's tests): its mistake costs a mark, not a secret.
    - The name after `/w` is the next word, or the run between a pair of
      double quotes. An unclosed quote takes the rest of the line as the
      name and leaves no text, which D11 then reports. `/w` alone reads as
      no name and no text.
    - `/wx` and `say /w hi` are said aloud, as `/oocx` is.
11. **D11. The console's `/w`.** Send and Enter go through the Say's
    `sending`, keyed `add-narration`, so a whisper takes the Say's lock,
    keeps what was typed when refused, and clears `say-text` alone when
    accepted (VTT-517, VTT-533, VTT-535, VTT-521). `whisperTargets`, a pure
    function exported from `dm.ts`, resolves the name:
    1. A party member of the console's state whose `name` equals it,
       compared without case: that one. When two or more do, the DM is told
       that two party members have that name, and nothing is sent.
    2. Otherwise a person the console knows (`known`: the roster and
       presence, by participant id, as the grant select knows them) whose
       name equals it without case: that participant's party members, those
       whose `controllerIds` hold them, by actor id. When there are none,
       the DM is told that person controls no party member.
    3. Otherwise the DM is told nobody at the table has that name.
    - A character comes before a player, because B-2 addresses a whisper to
      a character, and the ruling has a player's name stand "in place of" a
      character's.
    - Equal, not a prefix (Rule 9). Without case, because no two
      participants who are not revoked hold names equal without case
      (VTT-542, VTT-543), so a player's name finds one participant.
    - Two party members sharing a name is refused, not guessed. Guessing
      sends a private line to the wrong player, which is not a small
      consequence.
    - The DM is also told, and nothing is sent, for a `/w` with no name,
      for one with no text, and for `/dm`, which in the console says
      whisper to a character with `/w`.
    - Every one of these sends nothing, takes no lock and keeps `say-text`
      as typed. The DM is told by `DMDeps.notify`. The words are the
      implementer's; C3 pins that each is told.
    - Q1(a): `say-as` is not read for a whisper.
12. **D12. The panel's `/dm`.** `submit` sends an `addWhisper` whose
    `from_actor_id` is the party member the panel acts as, the same `actorId`
    `speakAs` uses, which follows the viewpoint (VTT-408). The builder's
    signature is the implementer's.
    - The panel's lock `saying`, keep-on-refusal (VTT-518) and
      empty-on-accept (VTT-520) hold, as for any line.
    - `/w …` sends nothing and tells the player to whisper to the DM with
      `/dm`. An empty `/dm` is told too.
    - The panel tells through a new optional last parameter `notify` of
      `renderPlayerPanel`, which `app.ts` sets as it sets the console's,
      to the toast. Optional, so the 24 existing calls in the tests keep
      compiling.
    - A player who controls no party member draws no Say at all (the
      panel's "You do not control an actor yet."), so has no `/dm`.
13. **D13. The tool, `add_whisper`.**
    - `requiredOverride` is `["text", "toActorIds"]`. The derived list would
      force `fromActorId`, which an agent would then fabricate and be
      refused, the trap `add_narration`'s override closes.
    - The description says the whisper is written into the world log and
      into the log of each party member named, and no other. It says that
      to whisper to a player you name every party member they control. And
      it says a `whisperAdded` in `get_events_since` naming `fromActorId` is
      that party member whispering to the DM and to you, answered with
      `add_whisper`, never `add_narration`, which every character hears.
    - Field docs: `toActorIds` are party members' ids; `fromActorId` is
      "Never set: a player's; refused from the DM and the agent"; `text` is
      at most 8192 bytes of UTF-8, not empty.
    - The MCP server's instructions do not change, because the tool is the
      one place an agent reads this.
14. **D14. The command arm lands in one commit with every table that names
    each command** (P4): the manifest, the golden, the role row,
    `mayWhisper`, `ToEvent`, the MCP name lists, the builder, the surface
    row, `readLine` and both boxes.
    - The surface row is `addWhisper: { surface: "dm-console", action:
      "add-narration" }`, the Say's Send, which is where a human whispers.
      Its comment says `/w` and `/dm` share that Send.
    - The event lands first, alone, in Task 1. The payload arm needs no
      command table.
15. **D15. A whisper stays in its log.** A connection that switches to a log
    is sent it from its first entry (SPEC-015), so whoever watches through
    Tobbe later reads Tobbe's whispers.
    - Forced by "Alla som ser genom Tobbe" and SPEC-021. No code. Noted for
      the owner above.
16. **D16. A log opened after a whisper never holds it, and no paused log
    is ever addressed.**
    - `opening` carries the open session, the party and the public notes
      (VTT-317): no narration and no whisper (P5, `late`).
    - Every party member's log is written (VTT-316), and an addressee is a
      party member at the snapshot (D6), so a recipient's log is open, or
      opens at that event.
    - SPEC-021's consequences on narration stay true. No code; QA is told.
17. **D17. No anchors on a whisper.** A preference: the console sends none,
    and an anchor would need `reanchor`'s rewrite per log (VTT-322).
18. **D18. `python3 tools/check-comments.py --write-ledger` runs once, after
    the last code commit**, and only if `check-comments.py main` finds a
    file more than 1.0 under its ceiling.
19. **D19. One browser beat, E1, in a new file
    `client/e2e/whispering.spec.ts`.**
    - Four people (G11). The fixture's DM, Lera playing `Lera<i>`, and
      Watcher promoted to play `Tobbe<i>`, as `speaking.spec.ts` promotes
      them. A guest joins through the share link, as `booting.spec.ts`'s
      does, and perches on `Tobbe<i>`.
    - Rerunnable: names and actor ids carry `test.info().repeatEachIndex`;
      it opens no session, which a whisper does not need, as
      `booting.spec.ts` opens none; `test.afterAll` closes the door. A rerun
      leaves Watcher holding the earlier run's Tobbe too, and a player's
      window watches its first party member by id (SPEC-015's default), so
      Watcher's window switches to `Tobbe<i>` by its chip before it types.
20. **D20. No Stryker run.** `client/src` changes, so the client's stored
    mutation report goes out of date. At Task 6 the owner is told which
    commit it measured and the tickets since, and that
    `task check:ts-mutation:full` is his to ask for.
21. **D21. VTT-529's citations move to N529 in Task 2**, the commit that makes
    VTT-529 false (P12): the 16 lines in 8 client files, `dm.ts`'s pointer
    comment among them. SPEC-022's two move at Task 5. A test that holds
    N529 and named VTT-529 keeps its assertions; only its id line changes.

## Requirements

Allocated at Task 0 with `requirement-id`, each `**OPEN — no test yet**`
until its tests land. The W, C, N and K labels, and the test labels H, F,
S, X, T, A, M, L, V, B and E, are this plan's.

The server, the folds and the tool:

- **W1.** "A whisper whose `to_actor_ids` name party members, and which names
  no `from_actor_id`, is written into the world log and into the character log
  of each party member it names, and into no other character log." H1, A1, E1.
  Red under K1.
- **W2.** "A whisper whose `from_actor_id` names a party member, and which
  names no `to_actor_ids`, is written into the world log and into that party
  member's character log, and into no other character log." H2, A3, E1. Red
  under K2.
- **W3.** "Of the party members a whisper names in `to_actor_ids`, the copy
  written into a character log names only that log's own character." H1, H3.
  Red under K3.
- **W4.** "Both folds refuse a `WhisperAdded` whose text is empty or longer
  than 8192 bytes of UTF-8, and accept one of exactly 8192." F1, F3. Red
  under K4, K6.
- **W5.** "Both folds refuse a `WhisperAdded` that names an actor they have
  not folded." F2, F3. Red under K5, K7.
- **W6.** "A DM's or an agent's whisper is refused, and nothing is appended,
  when it names no actor to whisper to, names one that is not a party
  member, or names an actor it whispers from." A2. Red under K12, K13.
- **W7.** "A player's whisper is refused, and nothing is appended, unless it
  names no actor to whisper to and names as its whisperer a party member
  that player controls whose name is not empty." A3. Red under K14, K15,
  K16.
- **W8.** "A player's whisper whose `from_actor_id` is empty, names no actor,
  is not theirs, or is not a party member is refused with one text." A3. Red
  under K25.
- **W9.** "Only a dm, an agent or a player may whisper." The `add_whisper`
  rows of `TestAuthorizeTableAllCommandsAllRoles`, A3's accepted whisper,
  A4. Red under K11.
- **W10.** "The `add_whisper` tool says a whisper is written into the world
  log and into the log of each party member it names and no other, that a
  whisper to a player names every party member that player controls, and that
  a party member's whisper to the DM is answered with `add_whisper`, never
  `add_narration`." T1. Red under K17.

The client:

- **C1.** "A Say line whose first word is `/w`, in any case, reads as a
  whisper to the next word, or to the words between a pair of double quotes
  after it, with the rest of the line, trimmed, as its text; one whose first
  word is `/dm`, in any case, reads as a whisper to the DM with the rest of
  the line, trimmed, as its text." L1. Red under K18.
- **C2.** "The DM console's Send, or Enter in its `say-text`, sends a `/w`
  line that has a name and a text as one whisper to the party member whose
  name equals the line's name without regard to case, or, when no party
  member's does, to every party member controlled by the participant whose
  name equals it without regard to case, in actor-id order, as the console
  knows them when the line is sent." V1, V2. Red under K19, K20.
- **C3.** "The DM console sends nothing, tells the DM why, and leaves
  `say-text` as typed, for a `/w` line whose name no party member and no
  participant has, that two party members have, or whose participant
  controls no party member, for a `/w` line with no name or no text, and
  for a `/dm` line." V3. Red under K21, K22.
- **C4.** "The player panel's Send, or Enter in its `say-text`, sends a `/dm`
  line with text after it as one whisper to the DM from the party member the
  panel acts as." S3, E1. Red under K23.
- **C5.** "The player panel sends nothing for a `/w` line or a `/dm` line with
  nothing after it, tells the player why, and leaves `say-text` as typed." S4,
  B1. Red under K24.
- **C6.** "The Story and the Events panel draw a whisper that names
  `to_actor_ids` as `The DM whispers to `, those party members' names as the
  window names actors, joined by `, `, then `: ` and its text; and one that
  names `from_actor_id` as that party member's name, ` whispers to the DM: `
  and its text. The Story draws either with the class `whisper`." S1, S2,
  E1. Red under K8, K9, K10.
- **N529**, succeeding VTT-529: "The DM console's Send, or Enter in its
  `say-text`, sends a line whose first word is not exactly `/ooc` and is
  neither `/w` nor `/dm` in any case, in character, with what `say-as`
  holds, trimmed, as `as`, and no `as` when it is blank." VTT-529's tests,
  and V1's `/wx` case.

The browser:

- **E1** is evidence for W1, W2, C4 and C6, not a row.

Withdrawn at Task 0, in the register's form, with the evidence
`**READING — verify-ticket check 5 of 2026-11-01; withdrawn, held by
nothing**`: VTT-529, "False as worded once a console line whose first word is
`/w` is sent as a whisper", succeeded by N529.

Kept, wording still true, evidence added:
- **VTT-272** (no envelope a player or spectator is sent carries a
  participant id or a role): H1.
- **VTT-503** (the event carries every field of the command it also has):
  A5.
- **VTT-162** (a refused command appends nothing): A2, A3.
- **VTT-476** (no Story line is a payload's name): the describe tables gain
  `whisperAdded`.
- **VTT-517, VTT-518, VTT-520, VTT-533, VTT-535**: V4, S3, S4, which send a
  whisper through the same paths.

Read and kept as they stand: VTT-362, VTT-505, VTT-495, VTT-499, VTT-500,
VTT-501, VTT-506, VTT-530, VTT-531, VTT-317, VTT-316. None says anything a
whisper makes false. VTT-362 and VTT-505 speak of `NarrationAdded` alone.

The sort, and what it rejected:
- "A whisper to a player reaches every party member that player controls":
  C2, the console's (G4). The server has no player to expand.
- "A spectator does not whisper": W9.
- "Never said aloud": C1 with C3 and C5, both boxes (G5).
- The grant in between: no row (D7). A late log: no row; W1's "no other
  character log" covers it with H1's case. A later watcher: no row (D15).
- The exact words of the console's and the panel's notices: how, not what.
  C3 and C5 pin that each is told.

| Ticket item | Observed by |
|---|---|
| 1 | H1, A1 |
| 2 | V2 (the console), E1 |
| 3 | H2, A3, A4 |
| 4 | F1, F3 |
| 5 | L1, V1, V2, V3 |
| 6 | L1, S3, S4, B1 |
| 7 | S1, S2, E1 |
| 8 | T1 |
| 9 | Task 6 |

## The existing tests that change

- `TestAuthorizeTableAllCommandsAllRoles`: `commandFor` builds the DM's
  shape, `to_actor_ids` naming one party member. Its four `add_whisper`
  rows are dm true, agent true, player false, with a comment as
  remove_actor's row has (a player's whisper names no actor to whisper to;
  A3 holds its accepted shape), and spectator false. The count check
  `len(authzCases) != 96` "(24 commands x 4 roles)" becomes 100 (25 x 4).
- `wantCommandToolNames` and `theCommandTools` gain `add_whisper`.
- `TestToolsMatchGolden` reads the regenerated golden.
- `cenv` in `internal/campaign/campaign_test.go` gains a `WhisperAdded` case.
- `COMMAND_SURFACE` gains `addWhisper` (D14).
- The describe-completeness tables in `client/test/spectator-view.test.ts`
  and `qa-what-a-window-shows.test.ts` gain a `whisperAdded` envelope.
- `client/test/player.test.ts`'s `readLine` cases now assert the `kind`
  field, `say` or `ooc`, where they assert `ooc`.
- Every test citing VTT-529 cites N529 (D21).
- Two names become false once `/w` and `/dm` exist, and are renamed in
  Task 2: `player.test.ts`'s "any other Say line is in character, trimmed,
  its slashes kept" and `qa-panel-say.test.ts`'s "a first word that is not
  exactly /ooc stays in character", each to say a first word that is not
  /ooc, /w or /dm. VTT-501's evidence cell, which cites the first, moves
  with it.
- No other existing test changes. P4 ran every completeness test the arms
  reach, and P6 ran `perceive`, `engine`, `campaign` and `harness` under
  P5's code.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Sign-off is given, with Q1 and Q2 answered.
  - Allocate W1 to W10, C1 to C6 and N529 with `requirement-id`, and W11
    too under Q1(b).
  - Withdraw VTT-529 in the register's form, naming N529.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean; the rows exist; the chain passes.

### Task 1: the event, its folds, its projection and its line (W1 to W5, C6; D1 to D5, D9, D16)

- **Files:**
  - `contract/vtt/v1/events.proto` (`WhisperAdded`, `whisper_added = 35`);
    `task generate:contract`;
    `contract/testdata/whisper_added_envelope.json`, new;
    `contract/roundtrip_test.go`;
  - `internal/engine/apply.go` (the arm), `apply_test.go`,
    `apply_boundary_test.go`;
  - `internal/perceive/projection.go` (`sightView.eyes`, `look`,
    `classify`'s arm, the cut in `Event`);
    `internal/perceive/whisper_internal_test.go`, new;
  - `internal/campaign/campaign_test.go` (`cenv`);
    `internal/campaign/whisper_test.go`, new, package `campaign_test`;
  - `client/src/fold.ts` (the arm), `client/test/fold-rejections.test.ts`;
  - `client/src/view/spectator.ts` (`describe`, `renderFeed`),
    `client/src/style.css`; `client/test/spectator-view.test.ts`,
    `client/test/qa-what-a-window-shows.test.ts`;
  - `cmd/vtt/webdist`;
  - `docs/requirements.md`: W1 to W5, C6.
- **New tests**, each with a literal name and its id line:
  - **H1** (W1, W3, VTT-272), `whisper_test.go`,
    `TestAWhisperReachesTheWorldAndTheLogsItNamesAlone`.
    - Party members tobbe, lera and brann; brann has no token.
    - The DM whispers to tobbe, then to brann and tobbe. A narration
      follows: the positive control every log holds.
    - The world log holds both whispers as appended. tobbe's log holds
      both, each `proto.Equal` to the world's envelope with its sequence,
      participant id and role cleared and `to_actor_ids` cut to tobbe.
      brann's holds the second, cut to brann. lera's holds neither.
    - A party member `late`, added afterwards, holds neither.
    - Every log folds at every prefix.
  - **H2** (W2), `TestAPartyMembersWhisperReachesTheWorldAndItsOwnLogAlone`:
    tobbe whispers to the DM, issued as a player. The world and tobbe's log
    hold it; lera's and brann's do not; the narration control follows.
  - **H3** (W3), `whisper_internal_test.go`,
    `TestAWhisperIsForwardedThroughEitherKindOfEyes`:
    - under a `Shoulder` and under a `Controller`, a whisper naming an eye
      is forwarded with only the eyes among `to_actor_ids`;
    - one naming none of them is withheld;
    - one whose `from_actor_id` is the eye is forwarded.
  - **F1** (W4), `apply_boundary_test.go`,
    `TestAWhisperAtTheTextCapIsAccepted` and, in `apply_test.go`,
    `TestAWhisperWhoseTextIsEmptyOrOverTheBoundIsRefused`: 0 and 8193
    bytes refused, 8192 accepted.
  - **F2** (W5), `apply_test.go`, `TestAWhisperNamingAnActorNotFoldedIsRefused`:
    an unknown id in `to_actor_ids`, an empty one, and an unknown
    `from_actor_id`, each refused; a known one accepted.
  - **F3** (W4, W5), `fold-rejections.test.ts`: "a whisper with no text is
    rejected", "a whisper longer than 8192 bytes is rejected", "a whisper of
    exactly 8192 bytes folds", "a whisper naming an actor the fold does not
    hold is rejected" (from and to).
  - **S1** (C6), `spectator-view.test.ts`, "the Story draws the DM's whisper
    naming whom it is to, with the class whisper": to one party member and
    to two, named as the window names them.
  - **S2** (C6), "the Story and the Events panel draw a party member's
    whisper to the DM by its name".
  - **X1**, `contract/roundtrip_test.go`,
    `TestWhisperAddedEnvelopeRoundTrip`, against the fixture.
- **Order:**
  1. The payload arm and `task generate:contract`.
  2. The tests, red where the code is not there yet.
  3. The two folds, `classify` and the cut, `describe` and `renderFeed`.
  4. The breaks.
- **True when:**
  - `go test ./contract/ ./internal/engine/ ./internal/perceive/
    ./internal/campaign/ ./internal/harness/` passes;
  - `env -u CLAUDECODE bun test client/test contract` passes; the typecheck
    is green; `fold.ts` and `spectator.ts` are at 100;
  - `share_of` holds every file at or under its ceiling;
  - `task generate:contract` run again changes nothing.
- **Breaks**, each undone by its inverse edit:
  - **K1:** `classify` forwards every whisper. H1 red (P5 break 2).
  - **K2:** the arm ignores `from_actor_id`. H2 red.
  - **K3:** the cut is removed. H1 and H3 red (P5 break 1).
  - **K4:** `engine.Apply` takes 8193 bytes. F1 red.
  - **K5:** `engine.Apply` skips the actor check. F2 red.
  - **K6:** `fold.ts` takes an empty text. F3 red.
  - **K7:** `fold.ts` skips the actor check. F3 red.
  - **K8:** `describe` leaves out the recipients. S1 red.
  - **K9:** `renderFeed` draws a whisper `mechanical`. S1 red.
  - **K10:** `describe` names the DM for a party member's whisper. S2 red.

### Task 2: the command, its rules, the tool and both Say boxes (W6 to W10, C1 to C5, N529; D6 to D8, D10 to D14)

- **Files:**
  - `contract/vtt/v1/commands.proto` (`AddWhisper`, `add_whisper = 36`);
    `task generate:contract`, which writes `contract/gen/` and
    `cmd/vtt/tools.json`; `contract/testdata/expected_tools.json`;
    `contract/testdata/add_whisper_command.json`, new;
    `contract/roundtrip_test.go`;
  - `tools/toolgen/main.go` (the entry), `main_test.go`;
  - `internal/gateway/authz.go` (`commandRoles`, `playerRules`,
    `commandName`, `mayWhisper`, the call in `Authorize`), `convert.go`
    (`ToEvent`'s arm); `authz_test.go`, `convert_test.go`;
    `internal/gateway/whisper_test.go`, new, package `gateway_test`, on
    `newGWFixture`;
  - `internal/mcp/tools_test.go`, `tools_internal_test.go`;
  - `client/src/commands.ts` (`addWhisper`), `client/src/player.ts`
    (`readLine`), `client/src/view/dm.ts` (`whisperTargets`, the Say),
    `client/src/view/player.ts` (`submit`, `notify`), `client/src/app.ts`
    (passes `notify`);
  - `client/test/commands.test.ts`, `player.test.ts`, `dm-view.test.ts`,
    `spectator-view.test.ts`, `app.test.ts`, `command-surface.test.ts`;
  - VTT-529's citing lines (D21, P12);
  - `cmd/vtt/webdist`;
  - `docs/requirements.md`: W6 to W10, C1 to C5, N529.
- **New tests**, each with a literal name and its id line:
  - **T1** (W10), `main_test.go`,
    `TestAddWhisperSaysWhomItReachesAndHowToAnswer`: the description holds
    "and no other", "every party member they control" and "never
    add_narration"; `required` is `text` and `toActorIds`.
  - **X2**, `TestAddWhisperCommandRoundTrip`, against the fixture.
  - **A1** (W1), `whisper_test.go`, `TestTheDMWhispersToAPartyMemberOverTheWire`:
    - a player holding tobbe and a player holding lera, both connected and
      drained;
    - the DM sends `add_whisper` to tobbe: ok, with a sequence; the world
      event carries the DM's participant id and role, the text and tobbe;
    - tobbe's player is sent it, and lera's player is not, shown by a
      narration both are sent next.
  - **A2** (W6, VTT-162), `TestADMsWhisperToNoPartyMemberIsRefused`:
    - the DM and the agent each name no actor, a creature, an unknown id,
      and tobbe with `from_actor_id` set;
    - each is refused, and the head does not move;
    - the control: the same DM's whisper to tobbe is ok.
  - **A3** (W2, W7, W8, VTT-162),
    `TestAPlayerWhispersToTheDMFromTheirOwnPartyMemberAlone`:
    - from their own: ok, appended naming `from_actor_id`;
    - from an empty id, from another player's party member, from a
      creature and from an unknown id: refused, with one text, and the head
      unmoved;
    - with `to_actor_ids` set: refused;
    - from a nameless party member of theirs: refused, naming "no name".
  - **A4** (W9), `TestASpectatorCannotWhisper`, over the wire, with the head
    before and after; and the table's rows.
  - **A5** (VTT-503), `convert_test.go`,
    `TestToEventAddWhisperProducesWhisperAdded`.
  - **M1**, `commands.test.ts`, "addWhisper matches the committed
    add_whisper fixture", read from `contract/testdata/`.
  - **L1** (C1), `player.test.ts`, "a Say line whose first word is /w or
    /dm, in any case, is a whisper":
    - `/w Tobbe hi there`; `/W tobbe hi`; `/w "Two Words" hi`;
      `/w "Unclosed hi`; `/w`; `/dm help`; `/DM help`;
    - `/wx hi` and `say /w hi` said aloud; the `/ooc` cases unchanged.
  - **V1** (C2, N529), `dm-view.test.ts`, "the console whispers /w Name to
    the party member of that name, whatever its case, and says /wx aloud".
  - **V2** (C2), "a participant's name, whatever its case, whispers to every
    party member they control, by id, and a party member's name wins over a
    participant's", with a `/w kim` case for Kim.
  - **V3** (C3), "an unknown name, a shared name, a player with no party
    member, no name, no text and /dm each send nothing, tell the DM and keep
    say-text".
  - **V4** (VTT-535, VTT-517, VTT-533), "a whisper takes the Say's lock,
    keeps its line when refused and clears it when accepted".
  - **S3** (C4, VTT-518, VTT-520), `spectator-view.test.ts`, "a player's
    /dm whispers to the DM from the party member the panel acts as, and
    keeps the line until accepted".
  - **S4** (C5), "a player's /w line, or a /dm line with nothing after it,
    sends nothing, tells them why and keeps the line".
  - **B1** (C5), `app.test.ts`, "a player's /w line is told in the toast".
- **Order:**
  1. The command arm and `task generate:contract`, then every table P4
     names, until `go test ./tools/toolgen/ ./contract/
     ./internal/gateway/ ./internal/mcp/ ./cmd/vtt/` and
     `bun test client/test` pass.
  2. The tests, red where the code is not there yet.
  3. `mayWhisper`, `ToEvent`'s arm, the tool's text, `readLine`,
     `whisperTargets`, both Say boxes, `notify`.
  4. The breaks.
- **True when:**
  - those Go packages pass; `go vet` is clean;
  - `env -u CLAUDECODE bun test client/test contract` passes; the typecheck
    is green; `commands.ts`, `player.ts`, `view/dm.ts`, `view/player.ts`
    and `app.ts` are at 100;
  - `share_of` holds every file at or under its ceiling;
  - `git grep -n "VTT-529" -- client internal cmd` prints nothing;
  - `task generate:contract` run again changes nothing.
- **Breaks:**
  - **K11:** `commandRoles` gives the spectator a cell. The table and A4
    red.
  - **K12:** `mayWhisper` lets the DM name a creature. A2 red.
  - **K13:** `mayWhisper` lets the DM set `from_actor_id`. A2 red.
  - **K14:** `mayWhisper` lets a player set `to_actor_ids`. A3 red.
  - **K15:** `mayWhisper` accepts a party member that is not the player's.
    A3 red.
  - **K16:** the name check is dropped. A3's nameless case red.
  - **K17:** the description loses "never add_narration". T1 red.
  - **K18:** `readLine` matches `/w` only in lower case. L1 red.
  - **K19:** `whisperTargets` tries participants before party members. V2
    red.
  - **K20:** a participant's party members unsorted, in `Object.values`
    order. V2 red, with fixture actors inserted out of id order.
  - **K21:** a shared name whispers to the first. V3 red.
  - **K22:** a client-side refusal clears `say-text`. V3 red.
  - **K23:** the panel's `/dm` whispers from the first party member, not the
    one it acts as. S3 red.
  - **K24:** the panel says a `/w` line aloud. S4 red.
  - **K25:** an absent `from_actor_id` is refused with a text of its own,
    naming the id. A3's one-text case red.

### Task 3: the browser (items 2, 3, 6, 7; D19)

- **Files:** `client/e2e/whispering.spec.ts`, new.
- `i` is `test.info().repeatEachIndex`.
- **E1** (W1, W2, C4, C6), `the DM whispers to Tobbe and Tobbe answers, and
  only the DM and whoever watches through Tobbe read either`.
  - With `asDM`, and no session: add party members `Tobbe<i>` and
    `Lera<i>`; promote Watcher to player; grant `Lera<i>` to Lera and
    `Tobbe<i>` to Watcher; open the door. Watcher's page opens after these,
    and switches to `Tobbe<i>` by its chip (D19).
  - A guest context joins by the link as `Eye<i>` and perches on
    `Tobbe<i>`.
  - The DM's console: `/w Tobbe<i> an arrow from the dark`, Enter. Tobbe's
    and Eye's Stories show "The DM whispers to Tobbe<i>: an arrow from the
    dark". The DM narrates "the wind turns"; Lera's Story shows that line
    and not the whisper.
  - The DM's console: `/w Watcher a second voice`, Enter. Tobbe's and Eye's
    Stories show "The DM whispers to Tobbe<i>: a second voice", and Lera's
    does not (ticket item 2; D4 narrows a rerun's earlier Tobbes away).
  - Tobbe's panel: `/dm I saw it`, Enter. The DM's and Eye's Stories show
    "Tobbe<i> whispers to the DM: I saw it". Lera's does not, after a
    second narration she is shown.
  - The frames Tobbe's page sends hold an `addWhisper` naming
    `fromActorId`, and no `addNarration` for that line.
  - A screenshot of each of the four windows after the answer.
- `test.afterAll` closes the door.
- **Order:** `task build:client`; check the machine's load; `task e2e`
  whole; then `bunx playwright test whispering.spec.ts --repeat-each=2`.
- **True when:** every spec passes, and the rerun passes twice.

### Task 4: the report

- **Do:** write `docs/reports/2026-11-01-whispering-to-and-from-characters.md`
  and give it its reading review.
- **It covers:**
  - each "Done looks like" item with its observation;
  - K1 to K25 with what went red;
  - Rule 9;
  - G1 to G12, and Q1's and Q2's answers;
  - P4 and P5, said once;
  - Phase 4a's adjudications under their own heading.
  - Leave room for the gate's figures.
- **True when:** it answers the ticket's nine items in their numbering, each
  `[x]` naming its observation.

### Task 5: the specifications

With the `specification` skill, from the report:
- **SPEC-016:**
  - "How each payload is ruled": a `WhisperAdded` is forwarded when the
    log's eyes include its `from_actor_id` or one of its `to_actor_ids`,
    else withheld;
  - "What `Event` answers": the forwarded copy's `to_actor_ids` keeps only
    the eyes;
  - "What a look sees": the look's `eyes`;
  - Consequences: a whisper reaches only the logs it names, each naming its
    own character alone;
  - Status: the new tests; Requirements: W1 to W3.
- **SPEC-018:**
  - the table: `WhisperAdded.text`, 8192, not empty; "Twenty-four" and
    "these twenty-four" become twenty-five;
  - the first criterion's sentence on player cells takes `add_whisper` in;
  - the second table's first row takes `WhisperAdded.to_actor_ids` and
    `from_actor_id`;
  - the arm lists in Status, both folds; Requirements: W4, W5.
- **SPEC-013:**
  - the table's row;
  - "Three checks for every role", "All three run", "those three checks"
    and "the three checks for every role" become four, with `mayWhisper`'s
    bullets;
  - the player half's `unrestricted` sentence names `add_whisper`;
  - Status: `whisper_test.go`; Requirements: W6 to W9.
- **SPEC-007:**
  - a section "Whom a whisper is addressed to": the two fields, which is
    set says the direction, actor ids alone, no anchor;
  - the numbers 36 and 35;
  - Status: X1, X2, T1; Requirements: W10.
- **SPEC-022:**
  - "Saying something": `readLine`'s four kinds; `/w` and `/dm` in any case;
    the console's resolution and its notices; the panel's `/dm` and `/w`
    and its `notify`;
  - "The Story's lines": the whisper's line and the class `whisper`;
  - Status and Requirements: VTT-529 out; N529 and C1 to C6 in.
- **SPEC-021** is read and not changed (D16).

No sentence on the grant in between (D7).

**True when:** each changed sentence has been run against the code by someone
other than its writer; every id in each Requirements section resolves;
`git grep -n "VTT-529" docs/specifications` prints nothing.

### Task 6: the ledger, the gate, whole, and the commits

- **Do:**
  - Run `python3 tools/check-comments.py main`. If it finds a file more than
    1.0 under its ceiling, run `--write-ledger` once and commit the ledger
    (D18).
  - Check free space and the machine's load.
  - Run `task check` whole on the last code commit. Launch it with
    `start_new_session=True`, send all output to a file, and require the
    gate's own completion line and `check:ts-mutation`'s own line.
  - Write the figures into the report; commit the report, then the
    specifications.
  - Tell the owner D20's reminder.
- **True when:** the gate is green, `check:comments`, `check:new-prose`,
  `check:requirements-chain`, `check:coverage`, `check:mutation` and
  `check:drift` included; `check:ts-mutation` printed that the client's
  mutation was not measured, naming its commit and this ticket; the log
  holds no Stryker line.
- Push and merge on the owner's word.

## For Phase 4a

Hand the QA agent:
- W1 to W10, C1 to C6, N529 and the kept rows' texts;
- SPEC-016's, SPEC-018's, SPEC-013's, SPEC-007's and SPEC-022's changed
  paragraphs;
- the surfaces, as signatures and names, never the source:
  - the wire: `{"addWhisper":{"text":"…","toActorIds":["…"]}}` and
    `{"addWhisper":{"text":"…","fromActorId":"…"}}` in a `ClientCommand`,
    its `CommandResult`, and `whisperAdded` in an `Envelope`;
  - `go doc ./internal/campaign`: `Open`, `Append`, `CharacterLog`,
    `CharacterEntry`, `FoldPrefix`;
  - `gateway.New(c, ids).Handler()`, `/ws`; `newGWFixture` and its fields,
    and `sendCommand`, in `internal/gateway/server_test.go`;
  - `readLine`, `addWhisper`, `whisperTargets`, `renderDMConsole` and
    `DMDeps`, `renderPlayerPanel` and its `notify`, `describe`;
  - the `add_whisper` entry of `contract/gen/tools/tools.json`.

Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- its files: `internal/campaign/qa_whisper_test.go`, package `campaign_test`,
  which may import `perceive` and `engine` and not `gateway`;
  `internal/gateway/qa_whisper_test.go`, package `gateway_test`; and
  `client/test/qa-whisper.test.ts`. Each is held to the default comment
  ceiling, 25.0. `go-arch-lint` runs in the hook and refuses an import across
  layers;
- a client test reads a file outside `client/` only from `contract/testdata/`
  or `contract/gen/`, never `cmd/vtt/tools.json`;
- a character log's opening, resume and projection-made entries carry no
  `EventId`; a forwarded whisper shares the world event's id; compare with
  `proto.Equal`;
- each log names only its own character among `to_actor_ids` (D4);
- a log opened after a whisper never holds it, a recipient's log is never
  paused (D16), and whoever watches through a character later reads its
  whispers (D15). None of these is a finding;
- a grant between the console's paint and the append is known and left open
  by the owner's rule on near-theoretical cases (D7). A test of it is not a
  finding;
- a nameless party member cannot whisper to the DM (D8);
- `/w` and `/dm` match in any case, `/ooc` only exactly (D10);
- "refused" comes with an accepted whisper at the same table; "nothing
  appended" reads the head before and after; "not sent" comes with a line the
  same connection is sent;
- give each test a literal name and its rule's id;
- measure with `env -u CLAUDECODE bun test <file>`;
- the tree is hashed before and after it.

## For Phase 4b

Hand the reviewer the diff with `git diff HEAD` and tell it:
- look at whether each scope claim ("only", "never", "every", "no other")
  names the search that bounds it;
- check each comment under `internal/`, `cmd/` and `client/src` is a warning
  or a pointer (rule 10), and each citation a name (rule 8);
- check the proto comments on `AddWhisper` and `WhisperAdded` are true of
  `mayWhisper` and `classify` as written.
