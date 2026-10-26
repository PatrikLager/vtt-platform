# Plan: a player's line speaks as the character they play, and /ooc as the player

- **Ticket:**
  `docs/superpowers/specs/2026-10-26-a-player-speaks-as-their-character-design.md`.
- **Verdict:** passes, with gaps G1 to G8 (below).
  - Verified 2026-10-26 by an agent that did not write the ticket, read-only
    against HEAD `e33376f`.
  - Probes ran in copies of HEAD outside the tree, under
    `~/.claude/projects/-Users-patriklager-dev-vtt-platform/scratch-t7/`, none
    committed: a gateway that refuses every player narration, a client with
    no speaker box, and a contract with the fields D1 adds, regenerated.
  - `task check`, `task e2e` and the mutation gates were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-26-a-player-speaks-as-their-character.md`.
- **Base:** main `e33376f`.
- **Branch:** `fix/a-player-speaks-as-their-character`.
- **Owner's rulings taken in, all 2026-10-26:**
  - a player's narration speaks as the character they play, and the
    free-text "in character as" box goes for a player;
  - out of character is `/ooc` typed in the text box, as MapTool's `/ooc`:
    not a toggle, not a second box;
  - "Ja för DM", "Ja, som DM": the DM and the agent keep a free speaker
    name, or none;
  - "Namn: (( text ))": an out-of-character line shows the name the player
    joined with, then the text inside `(( ))`, as MapTool's
    `MessageUtil.getFormattedOoc`; the server sets that name;
  - "Alltid som karaktären": a player's line without `/ooc` always speaks as
    the chosen character, so a player cannot send a nameless line that
    looks like the DM's narration;
  - "Ett ärende, servern först": one ticket, one branch; the contract and the
    server's refusal first, then the client.
- **Signed off:** 2026-10-26, "Ja, kör", with the answers to the plan's
  questions:
  - Q1, "Ja": the DM and the agent may send a line out of character, under
    the name the server writes;
  - Q2, "Nej, de använder as": the server refuses an `actor_id` from the DM
    or the agent.

## The gaps this plan inherits

- **G1. VTT-260 stops being true, and the ticket does not say so.** Its row
  reads "The event a one-envelope command becomes carries every field the
  command gave". SPEC-013 says the same of `ToEvent`. Items 5 and 6 have the
  server write `as` "whatever name the client sent", and `actor_id` has no
  field in the event (D1). D4 withdraws the row, and C8 succeeds it.
- **G2. Rules 1 and 2, as worded, publish a creature's hidden name.** A
  player may control an actor that is not a party member. The tool's own
  `grant_actor_control` text describes that step at the end of a creature's
  run, and `controls` does not ask the kind. A line spoken as such an actor
  would carry its world name to every character log, since `classify`
  forwards narration whatever the eyes see. That is the name `shownAs`
  keeps out of every log (VTT-465, SPEC-016). D2 narrows the rule to a
  party member the player controls, `ownPartyMember`, the predicate
  `judgedBy` and `MayWatch` already use. C1 states the narrowed rule.
- **G3. A party member can have no name.** The DM console's "Add actor"
  sends with an id alone ("an actor needs an id", in `renderDMConsole`), and
  `Actor.name` may be empty (SPEC-018). Stamped, the line would have an
  empty `as` and be drawn as narration, which the ruling "Alltid som
  karaktären" forbids. D2 refuses that line. An adventure's actor must be
  named (`internal/adventure/load.go`).
- **G4. "What it touches" misses files the change reaches (P1, P2, P5):**
  - `internal/gateway/server.go`: `handleCommand` calls the stamp (D3);
  - `scenarios/story-table.json` and `scenarios/goldens/story-table/stream.json`.
    The scenario's player narrates with no actor, so
    `TestScenarioLibraryRunsSelfContained` and
    `TestScenarioGoldenStreamsHaveNotDrifted` go red;
  - `internal/gateway/authz_test.go`'s table, three tests in
    `qa_issuer_test.go`, and `server_test.go`'s
    `TestAPromotionBitesWithoutReconnecting`;
  - `contract/testdata/expected_tools.json` (`TestToolsMatchGolden`) and
    `tools/toolgen/main_test.go`'s
    `TestAddNarrationFieldDocsNameOptionalFieldsAgainstFabrication`;
  - six tests in `client/test/spectator-view.test.ts`, not one (P2);
  - `client/src/player.ts` (D7) and `client/src/style.css` (D9).
- **G5. A DM can draw the player panel.** `canAct` admits a DM, and
  `grant_actor_control` takes any participant. So the DM's window, not its
  console, can hold a Say form. The ruling "goes for a player" keeps the
  "as" box there (D7).
- **G6. The MCP server can sit as a player** (`TestMCPSeatedAsAPlayerFollowsItsSetViewpoint`).
  `add_narration`'s one description serves every seat. Its `as` text, "omit
  to speak as yourself", becomes false for a player (D10).
- **G7. Rule 5 depends on Q1.** If the DM and the agent may send a line out
  of character, the server writes that line's `as` too. Rule 5 then holds
  only "unless the line is out of character" (C4).
- **G8. Item 8 needs a second player, and the e2e fixture mints one.**
  `startFixtures` mints `DM`, `Lera` and `Watcher` (`client/e2e/setup.ts`).
  D11 promotes the spectator in a spec file of its own.

## Measurements this plan stands on

All at `e33376f`, outside the repository.

| # | Probe | Result |
|---|---|---|
| P1 | A copy whose `playerRules["add_narration"]` refuses every player; `go test -count=1 -p 1 ./internal/gateway/... ./internal/mcp/... ./cmd/vtt/...` | 7 red, each on the probe's message: `TestAuthorizeTableAllCommandsAllRoles` (its `add_narration/player` cell), `TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause`, `TestQAIssACatchUpIsSentNoIssuerOrCause`, `TestQAIssTheDMAndTheAgentAreSentEachEventAsLogged`, `TestAPromotionBitesWithoutReconnecting`, `TestScenarioLibraryRunsSelfContained`, `TestScenarioGoldenStreamsHaveNotDrifted`. `internal/mcp` green. The first is HEAD's acceptance of a player's nameless line (items 3 and 4's "today") |
| P2 | A copy whose player panel draws no "as" box and sends no `as`; `env -u CLAUDECODE bun test client/test contract contract-spike` | 21 of 1554 red: 15 in `qa-say-boxes.test.ts`; in `spectator-view.test.ts` both "what a player typed in Say survives a repaint…", "Send empties both Say boxes for the paints after it", "an in-character speaker is carried, and a blank one is omitted", "both say-something boxes carry a placeholder" and "the panel's structure carries the classes the stylesheet targets". HEAD's copy: 1554 pass, 0 fail |
| P3 | A copy with D1's three fields; `go tool buf lint`, `go tool buf generate`, toolgen | 8 files change: the two protos, `commands.pb.go`, `events.pb.go`, `commands_pb.ts`, `events_pb.ts`, both `tools.json`. Only `AddNarration` and `NarrationAdded` change in generated code; `tools.json` gains `actorId` and `outOfCharacter`, with no description. `go build ./...` passes, `contract` tests pass, `bunx tsc --noEmit -p client/tsconfig.json` exits 0, the client suite is 1554 green. Red: `TestToolsMatchGolden` alone |
| P4 | Comment shares, by `check-comments.py`'s own `measure` | the budget table under Constraints |
| P5 | Callers, by grep | `Authorize`: `authorize` in `server.go`. `ToEvent`: `handleCommand`, and 34 test calls in 4 files. NarrationAdded's non-test Go readers: `engine.Apply`, `classify` and `forwardable` (`perceive`), `reanchor` (`campaign`), `Compile` (`adventure`), `toEnvelopeDump` (`conformance`), `eventgen`. Client: `fold.ts`, `buildFeed`, `renderFeed`, `describe`, `addNarration`'s one caller in `client/src/view/player.ts`. Player narrations in `scenarios/`: one, `story-table.json`. The e2e narrates from the DM's token only |
| P6 | Reading | `reanchor` rewrites the anchors in place and `forwardable` clones the whole envelope, so `as` and the new mark reach every character log as appended. `Compile` writes a `NarrationAdded` with `text` alone, and `narrationAddedDump` holds `text` alone, so no conformance golden changes |
| P7 | Reading `internal/gateway/join.go` and `identity.CreateInvite` | `usableDisplayName` caps a join's name at 64 runes, at most 256 bytes. `CreateInvite`, which `vtt invite --name` calls, checks the role and nothing of the name |
| P8 | `~/dev/RPTool/maptool` at `f4b7fef6c` | Rule 9, below |

Not measured: `task check`, `task e2e`, both mutation gates, and the server
change itself, which was probed only as the blanket refusal in P1.

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no ceiling, floor or exemption rises.
    - `tools/comment-ceilings.txt` rows only fall, by
      `python3 tools/check-comments.py --write-ledger`. Run it once per task,
      last, after staging.
    - Neither adjudication file gains a row for code this plan writes.
      Keep byte-identical: `abs` in `authz.go` (one Go row); `client/src/player.ts`'s
      sort comparators; `spectator.ts`'s two `renderPerch` comparisons;
      `feed.ts`'s fifteen rows. Add no line whose code repeats one of theirs,
      or a row's `copies` moves.
  - 3: the contract is additive. `check:breaking` only reports until
    `contract/RELEASED` exists, so the rule binds by itself. Regenerate with
    `task generate:contract`.
  - 4: the fold is untouched (D6).
  - 5: no game-system vocabulary; `.semgrep/vocabulary.yml` was read, and
    "character" is not in it.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `internal/`, `cmd/` or `client/src` is a warning, a
    pointer or an exported symbol's one-line doc.
- **The comment budget (P4).** Every file this plan edits sits within 0.1 of
  its ceiling. Shares are compared unrounded. Run `python3
  tools/check-comments.py main` after staging.

  | File | HEAD | Ceiling | What fits |
  |---|---|---|---|
  | `internal/gateway/server.go` | 202 of 919, 21.98% | 22.0 | one comment line needs at least three code lines with it |
  | `internal/gateway/authz.go` | 32 of 245, 13.06% | 13.1 | a new function with a one-line doc, and six code lines with it |
  | `internal/gateway/convert.go` | 10 of 129, 7.75% | 7.8 | a new function with a one-line doc and twelve code lines with it, no other comment |
  | `client/src/view/player.ts` | 55 of 249, 22.09% | 22.1 | one comment line per four code lines |
  | `client/src/view/spectator.ts` | 259 of 634, 40.85% | 40.9 | code lines only, or one comment line with two code lines |
  | `client/src/view/feed.ts` | 31 of 86, 36.05% | 36.1 | no added comment line without two code lines; reword `Narration.as`'s doc in place (D9) |
  | `client/src/commands.ts` | 165 of 393, 41.98% | 42.0 | a builder with a one-line doc |
  | `client/src/player.ts` | 38 of 79, 48.10% | 48.2 | a function with a one-line doc; the share then falls more than 1.0, so `--write-ledger` must lower the row |
  | `internal/gateway/authz_test.go` | 413 of 1191, 34.68% | 34.7 | two code lines with the first added comment line |
  | `internal/gateway/convert_test.go` | 59 of 444, 13.29% | 13.3 | seven code lines with the first added comment line |
  | `internal/gateway/qa_issuer_test.go` | 4 of 464, 0.86% | 0.9 | no added comment line |
  | `internal/gateway/server_test.go` | 616 of 2578, 23.89% | 23.9 | three code lines with the first added comment line |
  | `cmd/vtt/scenario_goldens_test.go` | 59 of 230, 25.65% | 25.7 | three code lines with the first added comment line |

  `tools/toolgen` is outside `check-comments.py`'s roots.
- **Coverage.** Every client file this plan edits is gated at 100.00 in
  `tools/ts-coverage-thresholds.txt`, and no new client file is planned. No
  hook runs `check:ts-coverage`. Run `python3 tools/check-ts-coverage.py
  tools/ts-coverage-thresholds.txt` by hand.
- **Write no guard that no test can reach.** For example, the stamp runs
  only on what `Authorize` accepted, so it needs no nil check for an absent
  actor (D3).
- **Requirement ids** come from
  `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/requirement-id`
  after sign-off, never by hand (SPEC-008).
  - A test that cites an id needs the row first.
  - An e2e test named as evidence carries the id in a `// VTT-NNN` line.
  - A row's text is never edited. A withdrawn row reads `WITHDRAWN
    <date>: <its text>. False as worded <why>. Succeeded by VTT-NNN.`, as
    VTT-035 and VTT-141 do.
  - A test renamed means the evidence cell naming it changes.
    `check-requirements-chain.py` reads literal test names in quotes.
- **Tier 1 runs at pre-commit.** `check:comments`, `check:ts-coverage`,
  `check:new-prose` and `check:requirements-chain` run only in `task check`,
  so run each by hand before a commit.
- **bun is quiet under `CLAUDECODE`.** Run `env -u CLAUDECODE bun test
  client/test contract contract-spike`.
- **`check-new-prose.py` holds test files too:** the 85-column band and the
  SHORT rule. Run `python3 tools/check-new-prose.py main` before each commit.
- **A NodeList needs `Array.from`;** `client/tsconfig.json` has no
  `DOM.Iterable`. `exactOptionalPropertyTypes` is on, so a builder spreads
  an optional field in, as `addNarration` does, and never assigns
  `undefined`.
- **The bundle.** Each commit that changes `client/src` or `contract/gen/ts`
  carries `cmd/vtt/webdist`, rebuilt by `task build:client`: the bundle embeds
  the contract's file descriptors. `check:drift` passes only after the commit.
  The e2e builds its binary from the working tree and embeds whatever
  `cmd/vtt/webdist` holds; it never rebuilds the bundle.
- **The scenario golden is edited by hand.** `cmd/vtt/scenario_goldens_test.go`
  has no `-update` flag, by ADR-009.
- **One browser run at a time.** The e2e and any probe share a state file.
- **Pushing** is on the owner's word. The pre-push hook takes about three
  minutes. Check `os.getloadavg()` first, write the whole output to a file,
  and never bypass the hook.
- **`review-record` refuses untracked files.** Commit the ticket and this
  plan in Task 0.
- **Mechanics.** Stage and commit in separate calls. Undo a break with its
  inverse edit and check it with grep, never `git checkout --`. Hash the
  tree before and after any agent that reads it.

## Rule 9: how MapTool does this

Read in `~/dev/RPTool/maptool` at `f4b7fef6c`. The ticket's paragraph holds.

- **What MapTool does.**
  - A line that does not open with `/` is a say: `commitCommand` in
    `CommandPanel` prefixes `/s `, and so does `MacroManager.executeMacro`.
  - `MessageUtil.getFormattedSay` writes `identity + ":"` and the text. The
    identity is `CommandPanel.getIdentity()`, the impersonated token's name.
    With nothing impersonated, `TokenIdentity.getIdentity()` answers
    `MapTool.getPlayer().getName()`. So a player's line in MapTool always
    carries a name.
  - Impersonation is the token's popup menu (`AbstractTokenPopupMenu.ImpersonateAction`,
    which commits `/im <token id>` and only for a token the player owns) or
    `/im`, and it is kept until changed.
  - Under restricted impersonation, `ImpersonateMacro.canImpersonate` gives
    the GM anything, a bare name included, and a player only a token
    `isOwner` of theirs. It runs in the client.
  - `OOCMacro` (name `ooc`) writes `getFormattedOoc`: `<div class='ooc'>`, the
    player's own name, `": "`, then `(( msg ))`. The client sets that name.
    `OOCMacro` has no GM check.
  - The command word is the first word, `splitNextWord`. It is looked up in
    `MACROS`, a case-sensitive `HashMap`, so `/OOC` is not `/ooc`.
  - `ClientHandshake` refuses a second connection under a connected
    player's name ("That player name is already connected"), because
    ownership is by name.
- **What we borrow.**
  - The speaker is the character the player plays, kept until changed: here
    the viewpoint the panel acts as (D7).
  - `/ooc` as the first word, exactly, and `name: (( text ))` with a class of
    its own (D7, D9).
  - The GM's free name, from `canImpersonate` (rulings "Ja för DM" and "Ja,
    som DM").
- **What we do not borrow.**
  - The trust. MapTool checks impersonation in the client and lets the
    client write the out-of-character name. Here the server refuses the line
    (D2) and writes both names (D3), the shape CLAUDE.md rule 9 gives for
    MapTool's distribution model.
  - The player's own name as the fallback speaker. The ruling "Alltid som
    karaktären" makes an in-character line speak as a character or not at
    all (D2).
  - Unique names at connect. Ownership here is by participant id, so two
    players named alike confuse a reader and nothing else. That stays
    outside this ticket (the ticket's second open point).

## Design questions for the owner

1. **Q1. May the DM and the agent send a line out of character?** The
   rulings give them a free speaker. They do not say whether the server's
   `(( ))` line is theirs too.
   - Recommended: **yes**, under their own participant name, written by the
     server as for a player.
     - The field then means one thing for every seat, and `add_narration`
       has one description for all of them (G6).
     - MapTool's `/ooc` has no GM check.
     - Rule 5 then reads "unless the line is out of character" (C4, G7).
   - If no: an out-of-character line from the DM or the agent is refused.
     C4 keeps the ticket's wording, and `TestAuthorizeTableAllCommandsAllRoles`
     needs a player-only command for its player cell (D13).
2. **Q2. May the DM and the agent name an actor (`actor_id`)?**
   - Recommended: **no**. The server refuses it, and they voice anyone with
     `as`, as today.
     - The stamp writes `Actor.name`. Of a creature, that is the name no
       character log holds (VTT-465). An agent naming a goblin by id would
       publish that name to every log without typing it.
     - `as` makes the DM type exactly what the table reads ("Ja för DM").
   - If yes: the server writes the named actor's world name for any actor
     the world holds, and refuses one it does not hold. D2's check becomes
     `playerRules`' entry, and Authorize gains nothing for every role. C5
     goes.

Not needed for this plan: whether two participants may share a display name.
MapTool's answer is above.

## Decisions

1. **D1. The contract gains three fields, and the speaker's name stays in
   `as`.**
   - The fields:
     - `AddNarration.actor_id = 5`, the actor a line speaks as;
     - `AddNarration.out_of_character = 6`;
     - `NarrationAdded.out_of_character = 5`.
   - None is proto3 `optional`, like the fields beside them. `add_narration`'s
     `requiredOverride: ["text"]` keeps the new ones optional in the tool.
   - Forced by:
     - ADR-007 and CLAUDE.md rule 3: new numbers, nothing renamed;
     - items 3 to 6: the command names the actor and the mark, and the event
       carries the mark and the name;
     - P3: only the two messages change in generated code;
     - `as`, which both folds already bound at 256 bytes (VTT-266) and
       `renderFeed` and `describe` already draw. The server's name needs no
       new free-text field, so SPEC-018 does not move (D6);
     - protojson omitting a false bool. Every `NarrationAdded` already in a
       log then reads as not out of character, which it is. NoteVisibility
       is an enum because there absence had to mean the safe value; here
       absence means the old one;
     - the name `actor_id`, as `UseAbility` and `RemoveCondition` name
       theirs.
   - **The event carries no actor id.**
     - `classify` forwards narration to every log whatever it sees. A
       creature's id in a log that never saw the creature tells the player
       the id exists, the ground of VTT-472.
     - Every reader would need a namer for it.
2. **D2. One speaker check, run by `Authorize` for every role on
   `add_narration`.** It sits as `MayWatch` does for `set_viewpoint`.
   `playerRules`' entry stays `unrestricted`, "whose rule has already
   applied" (SPEC-013). The function's name is the implementer's. In order:
   - out of character and naming an actor: refused, for every role. A line
     is in character or out of it, and the house refuses a contradiction
     rather than picking (`grant_actor_control`'s `kind`);
   - out of character: accepted for a player, and for the DM and the agent
     under Q1's answer;
   - the DM or the agent naming an actor: refused (Q2);
   - the DM or the agent otherwise: accepted, `as` as given;
   - a player: refused unless `ownPartyMember(st.Actors[actor_id], p.ID)`,
     with one text whether the actor is absent, not theirs, or not a party
     member, as `MayWatch` answers every named refusal with one string
     (VTT-347). `ownPartyMember` is nil-safe: `IsPartyMember` reads
     `GetKind()`;
   - then a player's party member whose `Name` is empty: refused, with a
     text that says to ask the DM for a name or to use `/ooc`. Ownership is
     already checked, so the text tells nothing new.
   - Every refusal wraps `ErrUnauthorized`.
   - Forced by G2 and G3, the rulings, the `set_viewpoint` precedent, and
     Q2: a refusal that binds the DM cannot live in the player half, which
     `Authorize` never reaches for a DM.
3. **D3. The server writes the speaker after `ToEvent`, in `handleCommand`.**
   - A pure function in `convert.go` (name the implementer's), called right
     after the `TokenMoved` backfill and before `campaign.Append`, with the
     snapshot `authorize` returned.
   - `ToEvent` copies `as` and `out_of_character` as the command gave them.
     Then the function:
     - sets `as` to `p.Name` when the line is out of character;
     - sets `as` to `st.Actors[actor_id].Name` for a player's line in
       character;
     - leaves the DM's and the agent's in-character `as` alone.
   - Forced by:
     - the ticket: `ToEvent` has the participant and not the state;
     - `handleCommand` holding both, the backfill's own reason (VTT-160);
     - P5: a third parameter on `ToEvent` changes 34 test calls in 4 files;
     - SPEC-013's principle: the stamp reads the snapshot `Authorize` judged.
   - `p` is re-resolved before every command (SPEC-009), so the name is the
     current one.
   - Keep `server.go`'s budget: the call takes at least three code lines for
     one comment line (P4).
4. **D4. VTT-260 is withdrawn, and C8 succeeds it.** Forced by G1: items 5
   and 6, and `actor_id` with no field in the event.
5. **D5. The name written is `Actor.name`, never a token's.**
   - Forced by the ticket's rule 2, and by an actor having no token or
     several.
   - A player's chips already name a party member by it:
     `WatchableActor.Name` is `a.GetName()` in `viewpoint.go`.
   - Recorded, not fixed: a party member whose token a player's window
     names otherwise (VTT-475's chain) speaks under its actor's name.
6. **D6. No fold changes, and SPEC-018 does not move.**
   - A bool needs no bound, and `as` keeps its 256.
   - Read in SPEC-018: `NarrationAdded.as` stays in its bounded table,
     since text a non-DM role chose still reaches it, now the player's
     display name through `/ooc`, and its 256 bytes already bound that.
     `AddNarration.actor_id` never reaches the log.
   - A name longer than 256 bytes: a join cannot make one (P7). A `vtt
     invite --name` can. That participant's `/ooc` line is then refused by
     the fold, cleanly, since a fold refusal appends nothing (SPEC-013, step
     9). That is near-theoretical with a small consequence, so it gets no
     code, no test and no debt entry, by the owner's ruling of 2026-10-21.
     The report records it.
7. **D7. The panel: one box for a player; the "as" box kept for the DM and
   the agent.**
   - Forced by the ruling ("goes for a player") and by "Ja för DM", with G5.
   - A player's Send, or Enter, reads the line through a pure function in
     `client/src/player.ts`. That file's header puts the panel's rules
     there, as tested pure functions.
     - First word exactly `/ooc`: out of character, with the rest of the
       line, trimmed, as its text.
     - Anything else: in character, as the panel's `actorId`. That is the
       same `ui.selectedActorId || mine[0]!.actorId` the abilities and moves
       act as, which `paint` keeps on the viewpoint (VTT-408).
     - `/ooc` with nothing after it sends nothing and leaves the box as
       typed, as an empty line does today.
     - Any other leading `/` stays text, as today.
   - The DM's and the agent's panel sends `as` from its box, as today. It
     reads `/ooc` too under Q1's "yes".
   - `PlayerUIState.sayAs` stays, for that panel.
   - The exact lowercase word is borrowed from MapTool (Rule 9), a
     preference within it.
8. **D8. Two new builders in `client/src/commands.ts`, a player's line and
   an out-of-character line.** `addNarration(text, as?, anchor?)` stays for
   the DM's panel. A preference: it leaves `commands.test.ts`'s three calls
   and the DM's path alone.
9. **D9. The Story and the Events panel draw an out-of-character line as
   `<as>: (( <text> ))`.**
   - `Narration` in `feed.ts` gains `ooc`.
   - `renderFeed` gives the line its own class, `ooc`, with the `speaker`
     span. `describe` writes the same words.
   - `Narration.as`'s doc, "empty for table talk", stops being true. Reword
     it in place, since `feed.ts` has no room for a new comment line (P4).
   - Forced by the ruling "Namn: (( text ))" and by `getFormattedOoc`'s
     `<div class='ooc'>`. The `.ooc` style in `style.css` is a preference.
10. **D10. `add_narration`'s description and field docs say what each seat
    sends.**
    - A player names `actorId`, a party member they control, or sets
      `outOfCharacter`. The server writes `as` for both.
    - The DM and the agent voice anyone with `as`. Q1 and Q2 decide the
      rest.
    - Forced by G6. `TestAddNarrationFieldDocsNameOptionalFieldsAgainstFabrication`'s
      "speak as yourself" changes with it. `expected_tools.json` is
      regenerated from the new output.
11. **D11. The browser test goes in a new spec file, `client/e2e/speaking.spec.ts`.**
    - Forced by `startFixtures`: one campaign per spec file, so the
      spectator promoted to a second player changes nothing in
      `table.spec.ts` or `typing.spec.ts`.
    - The first player, `Lera`, plays `act-fighter`, "Human Fighter". The two
      names differ, so a stamp that wrote the participant's name in
      character would show.
12. **D12. `story-table`'s player speaks as `act-hero`.**
    - The step gains `"actorId": "act-hero"`.
    - `stream.json` gains `"as": "Lera"` on that line, edited by hand.
    - `state.json` does not change, since narration folds to nothing.
13. **D13. The Go tests P1 turned red keep what each tests.**
    - `TestAuthorizeTableAllCommandsAllRoles`: `addNarrationCmd` becomes an
      out-of-character line, valid for every role under Q1's "yes".
    - The three `qa_issuer_test.go` tests: the player speaks as their own
      party member. The line then names a character, not the player, so "no
      issuer" stays what is tested.
    - `TestAPromotionBitesWithoutReconnecting`: the promoted player controls
      nothing, so the line is out of character.
14. **D14. The tasks run contract, server, client, browser,** by "Ett
    ärende, servern först". Between Task 2's commit and Task 3's, the
    branch's client sends player lines the server refuses. That is
    acceptable on a branch merged whole.

## Requirements

Each candidate names the observation that would go red if it broke. Task 0
allocated them after sign-off, under Q1's "Ja" and Q2's "Nej": C1 to C5 are
VTT-495 to VTT-499, C6 is split in two, VTT-500 for the box and the speaker and
VTT-501 for `/ooc`, since they are two obligations, and C7 to C9 are VTT-502 to
VTT-504. VTT-500 reads "A player's player panel draws no box for a speaker,
and its Send speaks as the party member the panel acts as."; VTT-501 "A line
the player panel sends whose first word is `/ooc` goes out of character, with
the rest of the line as its text.", any role's panel, by Q1.

- **C1.** "A player's `add_narration` is refused unless it is out of
  character and names no actor, or is not out of character and names a
  party member that player controls whose name is not empty." A1 to A4. Red
  under K1, K2, K3, K9.
- **C2.** "A player's narration that is not out of character is appended
  with `as` set to the name of the actor it names, whatever `as` the command
  carried." S1. Red under K4.
- **C3.** "A narration out of character is appended marked out of
  character, with `as` set to the display name of the participant who sent
  it, whatever `as` the command carried." S2, S3. Red under K5, K6, K19.
- **C4.** "The DM's and the agent's narration that is not out of character
  is appended with the `as` the command carried, empty included." S4. Red
  under K7. Under Q1's "no", the clause "that is not out of character" goes.
- **C5** (Q2's "no"). "A DM's or an agent's `add_narration` that names an
  actor is refused." A5. Red under K8.
- **C6**, allocated as two rows. VTT-500, "A player's player panel draws no
  box for a speaker, and its Send speaks as the party member the panel acts
  as.": V1, W1, red under K10 to K12. VTT-501, "A line the player panel sends
  whose first word is `/ooc` goes out of character, with the rest of the line
  as its text.": V2, V3, W1, red under K13 and K14. VTT-500 keeps its "and":
  the box and Send's speaker are one panel's behaviour, and V1 holds both.
- **C7.** "The Story, and the Events panel where a window draws one, draw a
  narration out of character as its `as`, `: `, and its text between `(( `
  and ` ))`." F1, F2, W1. Red under K15, K16. The Events panel is the DM's and
  the agent's alone (VTT-445).
- **C8**, VTT-260's successor. "The event a one-envelope command becomes
  carries every field of the command that the event also has, a grant's kind
  and a move's reason included, except a narration's `as` when it is a
  player's or out of character, which the server writes." VTT-260's evidence
  stays, with S1 and S2 added. Red under K17.
- **C9.** "A DM's or an agent's player panel draws the in-character box, and
  its Send carries what the box holds as `as`." V4. Red under K18.

The sort, and what it rejected:
- Rule 2, "every viewer sees…", is C2 plus VTT-362 (narration reaches every
  active log) and `forwardable`'s whole clone (P6). It gets no id of its
  own; W1 sees all of it.
- "A player cannot send a nameless line" follows from C1 and C2.
- `/ooc`'s case and word boundary are inside VTT-501's "first word".
- An empty `/ooc` sending nothing joins the existing "empty or
  whitespace-only text sends nothing", which has no row.
- The 256-byte invite name, and an empty one: near-theoretical (D6).
- Duplicate display names: not this ticket.

Withdrawn: VTT-260, succeeded by C8.

Kept, wording still true:
- VTT-491 and VTT-492, which are about any box;
- VTT-493: "the player panel's Say boxes" is one box for a player and two
  for a DM's panel, and each keeps what was typed;
- VTT-266;
- VTT-272: an out-of-character line carries a display name, never a
  participant id or a role.

The evidence cells of VTT-491 and VTT-493 change as their tests are renamed
(Task 3).

| Ticket item | Observed by |
|---|---|
| 1 | V1, red under K10 and K11 |
| 2 | V2, red under K13 |
| 3 | A1 |
| 4 | A2's `as`-set and `as`-empty cases |
| 5 | S1 |
| 6 | S2, S3, F1, F2 |
| 7 | S4 |
| 8 | W1, red on Task 1's commit |

## The existing tests that change

- **Go (P1):** `TestAuthorizeTableAllCommandsAllRoles`, the three
  `qa_issuer_test.go` tests, `TestAPromotionBitesWithoutReconnecting`, both
  scenario tests, through `story-table` (D12, D13).
- **toolgen:** `TestAddNarrationFieldDocsNameOptionalFieldsAgainstFabrication`
  and `TestToolsMatchGolden`, through `expected_tools.json` (D10).
- **`convert_test.go`:** `TestToEventAddNarrationProducesNarrationAdded`
  keeps the DM's verbatim copy and gains `OutOfCharacter`.
- **Client (P2):** the 15 `qa-say-boxes.test.ts` tests and the six
  `spectator-view.test.ts` tests.
  - Each rewritten for a player keeps its point with the one box.
  - Those about the "as" box move to a DM's panel, which keeps it.
  - "an in-character speaker is carried, and a blank one is omitted"
    becomes V4.
  - The rewrite may rename tests. Then the evidence cells of VTT-491 and
    VTT-493 change in the same commit.
- **e2e:** `typing.spec.ts`'s "what a player types in Say survives a repaint,
  focus and all" finds the box by its placeholder, and so does
  `client/test/qa-window-number.test.ts`. Move both to
  `[data-field="say-text"]` if the placeholder changes.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off, with answers to Q1 and Q2.
  - Allocate C1 to C9, less those an answer removes, with `requirement-id`,
    each row `**OPEN — no test yet**`.
  - Mark VTT-260 WITHDRAWN, naming C8's id as its successor. The tests that
    cite it move to C8's id in Task 2, with the code that makes it false.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean, the rows exist, and `python3
  tools/check-requirements-chain.py` passes.

### Task 1: the contract (D1)

- **Files:**
  - `contract/vtt/v1/commands.proto` and `events.proto`, each new field with
    a doc comment saying who writes it;
  - what `task generate:contract` regenerates (P3);
  - `cmd/vtt/webdist/`, rebuilt by `task build:client`, since
    `contract/gen/ts` changes;
  - `contract/testdata/expected_tools.json`, the regenerated `tools.json`;
  - a new `contract/testdata/narration_out_of_character_envelope.json` and
    `add_narration_command.json`, with round trips in `roundtrip_test.go`
    and `events.test.ts`, as `describe_token_command.json` was added.
- **True when:**
  - `go build ./...`, the Go and client suites and `task client:typecheck`
    are green;
  - `task check:drift` passes after the commit;
  - nothing else changes behaviour. The e2e's "today" is taken on this
    commit (Task 4).

### Task 2: the server's refusal and its stamp (items 3 to 7)

- **Files:**
  - `internal/gateway/authz.go`: D2, and `Authorize`'s doc comment, "the two
    checks for every role", reworded in place to "the checks for every role";
  - `convert.go`: D3's function, and `ToEvent` copying `OutOfCharacter`;
  - `server.go`: the call (D3);
  - `tools/toolgen/main.go`: D10, then `task generate:contract`;
  - `tools/toolgen/main_test.go` and `contract/testdata/expected_tools.json`;
  - `scenarios/story-table.json` and `scenarios/goldens/story-table/stream.json`
    (D12);
  - `internal/gateway/authz_test.go`, `convert_test.go`, `qa_issuer_test.go`,
    `server_test.go` (D13), and a new `internal/gateway/narration_test.go`
    (name the implementer's);
  - `tools/comment-ceilings.txt`, by `--write-ledger`;
  - `docs/requirements.md`: C1 to C5's and C8's evidence;
  - every test that cites VTT-260 cites VTT-503 instead, in
    `convert_test.go`, `server_test.go`, `qa_token_texts_test.go`,
    `qa_describe_token_test.go`, `qa_move_reason_test.go` and
    `cmd/vtt/scenario_goldens_test.go`, so no test holds a withdrawn row.
- **New tests:**
  - **A1** (item 3): `Authorize` for a player on a world with two party
    members, one theirs. In character as the other: refused. As theirs:
    accepted.
  - **A2** (items 4, G2): the player names no actor, once with `as` `DM` and
    once with `as` empty: both refused. Then a non-party actor the player
    controls: refused, with the same text as an absent id.
  - **A3** (G3): the player's own party member with an empty name: refused.
  - **A4:** out of character with no actor: accepted for a player who
    controls nothing. Out of character naming an actor, the player's own
    named party member included: refused for every role.
  - **A5** (C5): the DM and the agent naming an actor: refused.
  - **S1** (item 5): over a live server, a player's line as their party
    member, with `as` `Someone Else`, appends a `NarrationAdded` whose `as`
    is the actor's name. Read it on the DM's connection.
  - **S2** (item 6): a player who joined as `Ada` sends `/ooc`'s command,
    with `as` `Lera`. The appended event is marked out of character and its
    `as` is `Ada`. Then the DM and the agent each send one with `as` `Lera`:
    each is appended under its own participant name (VTT-497 is any role's).
  - **S3:** another player's connection receives S2's line with the mark and
    `Ada`, so the projection keeps both (P6).
  - **S4** (item 7): the DM's and the agent's lines with `as` `Goblin chief`
    and with `as` empty are appended with that `as`.
  - The stamp's function gets unit tests beside S1 to S4 for its three arms.
- **Order:**
  1. Write A1 to A5 and S1 to S4.
  2. Run them on Task 1's tree: A1's refusal, A2, A3, A4's refusal, A5, S1,
     S2 and S3 are red, and S4 and the acceptances are green.
  3. Write D2, D3 and D10, then the test and scenario edits.
  4. Make the breaks.
  5. Run `go test ./internal/gateway/ ./cmd/vtt/ ./tools/toolgen/`, the
     client suite, `check-new-prose.py` and `check-comments.py main` after
     staging. Run `--write-ledger` last.
- **True when:**
  - all green, with every probe-red test of P1 green by D12 and D13;
  - `grep -n '"add_narration": unrestricted' internal/gateway/authz.go`
    still prints its line, since D2's check runs in `Authorize`.
- **Breaks**, each undone by its inverse edit:
  - **K1:** `Authorize` skips the speaker check. A1, A2 red.
  - **K2:** `controls` in place of `ownPartyMember`. A2's creature case red.
  - **K3:** the empty-name refusal removed. A3 red.
  - **K4:** the in-character stamp removed. S1 red.
  - **K5:** the out-of-character stamp writes the command's `as`. S2 red.
  - **K6:** `ToEvent` drops `OutOfCharacter`. S2, S3 red.
  - **K7:** the in-character stamp runs for every role. S4 red.
  - **K8:** a DM's `actor_id` accepted. A5 red.
  - **K9:** out of character naming an actor accepted. A4 red.
  - **K19:** the out-of-character stamp runs for a player only. S2's DM and
    agent cases red.
  - **K17:** `ToEvent` drops the command's `anchor_to_seq` for a narration.
    `TestToEventAddNarrationProducesNarrationAdded`, C8's evidence, red.
  - Apply by hand the mutants gremlins will make on the new Go code: each
    condition negated and each comparison flipped. Each must turn a test
    red. A survivor is fixed in the code or the tests, never adjudicated.

### Task 3: the client (items 1, 2, 6)

- **Files:**
  - `client/src/player.ts`: D7's pure function;
  - `client/src/commands.ts`: D8;
  - `client/src/view/player.ts`: D7;
  - `client/src/view/feed.ts`: D9's `ooc` and the reworded doc;
  - `client/src/view/spectator.ts`: `renderFeed` and `describe`;
  - `client/src/style.css`: `.ooc`;
  - `client/test/spectator-view.test.ts`, `qa-say-boxes.test.ts`,
    `player.test.ts` (the pure function), `commands.test.ts`,
    `feed.test.ts`;
  - `client/e2e/typing.spec.ts`'s and `client/test/qa-window-number.test.ts`'s
    locators, if the placeholder changes;
  - `cmd/vtt/webdist/`;
  - `tools/comment-ceilings.txt`, by `--write-ledger`: `client/src/player.ts`'s
    row will fall more than 1.0;
  - `docs/requirements.md`: VTT-500's, VTT-501's, VTT-502's and VTT-504's
    evidence, and VTT-491's and
    VTT-493's cells for renamed tests.
- **New tests:**
  - **V1** (item 1): a player controlling `a1` and `a2`, the panel acting as
    `a2`. The panel has no `say-as` box. `I draw my sword`, then Send: one
    narration, `actorId` `a2`, not out of character, no `as`.
  - **V2** (item 2): `/ooc where is the pizza`, then Send: out of character,
    text `where is the pizza`, no `actorId`.
  - **V3:** `/ooc` alone sends nothing. `/oocx hi` and `/OOC hi` are sent in
    character as typed. Enter sends as Send does. A DM's panel's `/ooc brb`,
    with `Goblin` in the in-character box, is sent out of character
    carrying `as` `Goblin`, as VTT-504 says; the server writes its own
    (VTT-497). VTT-501 is any role's.
  - **V4** (C9): a DM's panel, with an actor granted to the DM, draws
    `say-as`, and Send carries its trimmed text as `as`.
  - **F1** (item 6): `renderSpectator` with an out-of-character
    `NarrationAdded` whose `as` is `Ada`. The line's class is `ooc`, and its
    `textContent` is `Ada: (( where is the pizza ))`.
  - **F2:** the Events panel's row for it reads the same.
- **Order:**
  1. Write V1 to V4, F1 and F2. All are red on Task 2's tree but V4.
  2. Rewrite P2's 21 tests (above).
  3. Make D7 to D9.
  4. Make the breaks.
  5. `task build:client`; the client suite; `task client:typecheck`;
     `check-ts-coverage.py`; `check-new-prose.py`; `check-comments.py main`
     after staging. Run `--write-ledger` last.
- **True when:** all green, and `grep -rn 'in character as' client/src/view/player.ts`
  prints only the DM's panel's line.
- **Breaks:**
  - **K10:** the player's panel draws the "as" box again. V1 red.
  - **K11:** Send sends no `actorId`. V1 red.
  - **K12:** the line speaks as `ui.selectedActorId` with no fallback. A V1
    variant with `selectedActorId` empty red.
  - **K13:** `/ooc` is not stripped. V2 red.
  - **K14:** a prefix test in place of the first word. V3's `/oocx` red.
  - **K15:** `renderFeed` draws an out-of-character line as speech. F1 red.
  - **K16:** `describe` ignores the mark. F2 red.
  - **K18:** the "as" box removed for every role. V4 red.
  - Before the gate, apply by hand the mutants Stryker will make on the new
    lines: the `/ooc` literal, the trims, the empty-text guard and each
    conditional. Each must turn a test red.

### Task 4: the browser test (item 8)

- **Files:** `client/e2e/speaking.spec.ts`, new (D11); `docs/requirements.md`,
  VTT-496's, VTT-497's, VTT-500's, VTT-501's and VTT-502's evidence.
- **W1:**
  - the DM, over a raw socket:
    - starts a session and loads `goblin-ambush`;
    - grants `act-fighter` to `Lera` as a party member;
    - promotes `Watcher` to player;
    - adds a party member for `Watcher` and grants it;
  - `Watcher`'s page is opened after the promotion, since the client reads
    the role at boot;
  - `Lera`'s page sends `I draw my sword`; `Watcher`'s Story reads `Human
    Fighter: I draw my sword`;
  - `Lera` sends `/ooc brb`; `Watcher`'s Story reads `Lera: (( brb ))`;
  - it carries VTT-496, VTT-497, VTT-500, VTT-501 and VTT-502, and closes
    its session in `test.afterAll`, as `client/e2e/setup.ts` asks of every
    spec that opens one.
- **Order:**
  1. Write W1. Run it in a copy of Task 1's commit made with `git archive`,
     with the spec copied in. It is red: the first line reads `I draw my
     sword` as narration. That is item 8's "today".
  2. Then run `task e2e` on the tree, after Task 3's bundle. W1 is green,
     and so is every other spec.
- **True when:** W1 was seen red on Task 1's commit and is green on the
  tree, with every other spec green.
- **Break:** rebuild the bundle with K11. W1 red.

### Task 5: the report

- **Do:** write `docs/reports/2026-10-26-a-player-speaks-as-their-character.md`
  and give it its reading review. It covers:
  - each "Done looks like" item with its observation;
  - K1 to K19, with what went red;
  - Rule 9;
  - G1 to G8, Q1 and Q2, and D6's measured edge;
  - Phase 4a's adjudications, under their own heading.

  Leave room for the gate's figures.
- **True when:** it answers the ticket's eight items in the ticket's
  numbering, each `[x]` naming its observation.

### Task 6: the specifications

With the `specification` skill, from the report:
- **SPEC-013:**
  - the speaker check, for every role, in the table's cell and "The rules";
    its "Two checks for every role" paragraph becomes three;
  - the stamp, as a step of the command path after the backfill;
  - VTT-260's exception, now C8;
  - in Consequences, a player speaks only as a party member they control,
    or out of character.
- **SPEC-007:**
  - the three fields and who writes each;
  - why the event carries no actor id;
  - that an old `NarrationAdded` reads as not out of character.
- **SPEC-016:** an out-of-character line reaches every active character log
  naming the participant who sent it, by display name and never by id, by
  the owner's ruling "Namn: (( text ))".
- **SPEC-022:**
  - the panel's box per role;
  - `/ooc`;
  - the Story's and the Events panel's out-of-character line;
  - VTT-493's text for one box.
- Each spec's Requirements list gains its ids. SPEC-013's swaps VTT-260 for
  VTT-503: no specification names a withdrawn id.

**True when:** each new sentence has been run against the code by someone
other than its writer.

### Task 7: the gate, whole, and the commits

- **Do:**
  - Run `task check` whole on Task 4's commit, the last that changes code.
    Launch it with `start_new_session=True`, check free disk first, and
    require the gate's own completion line.
  - Then run `task e2e` once on the same commit.
  - Write both runs' figures into the report.
  - Commit the report, then the specifications.
- **True when** both are green, including:
  - `check:mutation` and `check:ts-mutation`, with no new adjudication.
    Stryker runs in Docker here, about an hour;
  - `check:ts-coverage` and `check:comments`;
  - `check:drift`;
  - `check:requirements-chain`.

## For Phase 4a

Hand the QA agent:
- the allocated rules' texts;
- `go doc ./internal/gateway`, and the client signatures:
  `renderPlayerPanel(st, me, abilities, ui, send, rerender, switcher?)`,
  `renderSpectator(root, st, log, status, extras?)`, `describe(e, names)`
  and the new builders;
- that the stamp is unexported and runs only inside `handleCommand`, so it
  is reached through a live server;
- that a player who controls a party member always watches one (VTT-340),
  and that a player may still control a creature (G2).

Tell it:
- `ownPartyMember` and `controls` differ exactly on a controlled creature;
- run `env -u CLAUDECODE bun test …`;
- never start a browser run beside another one.
