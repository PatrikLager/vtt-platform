# Plan: the DM gives a character an ability and takes one away, in play

- **Ticket:** `docs/superpowers/specs/2026-11-14-a-sheets-abilities-change-in-play-design.md`
  (blueprint principles 2, 3, 7, 12, 13, 16, 17, 26 and 27).
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-14 by an agent that did not write the ticket,
    read-only against `12bda75` on `feat/a-sheets-abilities-change-in-play`
    in the worktree `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and gremlins were not run. Probe P1 ran through
    `go test -overlay`, its test file kept outside the tree.
- **To be committed as:** `docs/superpowers/plans/2026-11-14-a-sheets-abilities-change-in-play.md`.
- **Base:** `12bda75`. **Branch:** `feat/a-sheets-abilities-change-in-play`.
- **Owner's rulings taken in:**
  - 2026-11-14: giving and taking an ability is one ticket, not split; the
    command, the MCP tool and the DM console's controls are all in it; the
    player is told both when an ability is given and when one is taken.
  - 2026-11-13: giving or taking an ability on a character already in a
    campaign is its own ticket, after "a sheet offers only what it names".
    A player reads through `/api/ruleset` only the abilities named by the
    sheets of the actors they control, the DM and the agent all. A Story
    names an ability its window cannot read by its id (that plan's OD-5,
    A; OD-5 below narrows it, and says so). A repeated id on a sheet is
    accepted, and the list has no fold bound (that plan's OD-4).
  - 2026-11-08: the DM owns the fit of campaign data; no automatic defaults.
  - 2026-11-06: sight decides what a character log holds of a companion's
    sheet.
  - 2026-11-05: a character has only the powers its sheet names, and only
    those show in its window.
  - 2026-11-02: plain words are for the board engine's refusals a player
    meets from the panel. A give's and a take's refusals reach only the DM
    and the agent, so they keep the fold's and the gateway's form (D2, D3).
  - 2026-10-21: a near-theoretical edge with a small consequence gets no
    code, no test and no debt entry (D8).
  - 2026-10-09: a field outside SPEC-018's first table is unbounded on
    purpose (OD-6).
  - 2026-09-23: a fact has one home (D3, D8).
  - 2026-09-04: nobody uses the product; no migration. Principle 5 still
    binds every log already written: none holds the new events.
  - Standing: no counts or measured figures in anything written down.
- **Signed off:** by the owner, 2026-11-14: every recommendation of OD-1
  to OD-7.
- **Ids:** VTT-713 to VTT-736, allocated at Task 0 (under
  "Requirements"); the `R` labels are this plan's.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol it names is where it says: `Actor.ability_ids` and
   `Envelope` in `contract/vtt/v1/events.proto`; `ClientCommand` and its
   control, condition and removal arms in `commands.proto`; `Apply` and its
   `ActorAdded` arm; `sameIncarnation`, `recall`, `Resume`, `classify`,
   `statusFrames` and `belief` in `projection.go`; `abilitiesReadBy`;
   `offered`; `watchKey`, `readRuleset` and `onPartyMembersLeft` in
   `app.ts`; `SheetAbilityFault` and `Offers`; `commandName`,
   `commandRoles` and `playerRules`; `handleCommand`; the refusal
   `does not name ability` in `handleUseAbility`; every specification
   under "Specifications this moves"; and MapTool's
   `Token.macroPropertiesMap`, `Token.Update.saveMacro` and `deleteMacro`,
   `updateTokenProperty`, `ServerCommandClientImpl`,
   `ServerMessageHandler`, `ClientMessageHandler`,
   `Zone.tokenMacroChanged`, `SelectionPanel`, `MacroButtonPopupMenu`,
   `MacroFunctions` and `Token.toDto` (P7). The Problem's claims about
   today hold: `ActorRemoved` takes the actor's conditions with it, and
   `engine.Apply` refuses an `ActorAdded` that names a controller.
2. **"Done" is an observation. By command for items 1, 2 and 7, by
   reading for the others.** Each item names what will be true and the
   test that shows it; no named test exists yet, so none could be run to
   fail.
   - Items 1 and 2: no command, event or tool gives or takes an ability
     (P3), and a use of an ability the sheet does not name is refused for
     every role today (P2).
   - Item 7: a list changed with no event makes the actor's own log, and
     a log that held it whole out of sight, be sent a bare `ActorRemoved`
     and a fresh `ActorAdded` of it at the next unrelated event (P1). That
     is what the ticket's "would read as another character" predicts.
   - Item 5: `watchKey` keys on the watchable set's ids and its flag, so a
     change to a controlled actor's list issues no read (reading).
   - Item 6: the window's only toast about its party members is
     `onPartyMembersLeft`'s (reading).
   - Items 3, 4 and 8 to 11 name tests and gates that do not exist yet or
     cannot hold today (reading).
3. **Each rule is breakable. By reading.** Each rule line maps to a row R1
   to R23 below, each with what turns it red.
4. **The scope matches the claim. By command, then reading.** The callers
   (P4): `sameIncarnation` from `recall` and `Resume`; `statusFrames` from
   `correct` and `Resume`; `beliefOf` from `recall` and `Resume`;
   `watchable` from `openSeat` and `seat.resolve`; `ToEvent` from
   `handleCommand`; `SheetAbilityFault` from `loadActors`, `sheetsFit` and
   `validateAddActorSheet`; `rules.Offers` from `handleUseAbility` and
   `abilitiesReadBy`; `watchKey` and `rereadIfStale` from `app.ts` alone;
   `describe` from `renderFeed` and `renderTicker`; `renderDMConsole` from
   `app.ts`. The work reaches files the ticket does not name, and one it
   names must not change: G1, G2.
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-025's consequence "No command changes which keys a sheet holds
     or which abilities it names" is overturned for abilities; the ticket
     names SPEC-025 and its Problem states the change.
   - SPEC-016 defines `sameIncarnation` over the fields only `ActorAdded`
     writes, the ability ids among them, and VTT-707 and VTT-708 name
     "ability ids" among what re-introduces an actor. A give writes the
     list, so by SPEC-016's own definition the list leaves
     `sameIncarnation` (D5), which the previous plan's D4 already recorded
     as this ticket's. The ticket names SPEC-016 but not the rows: G4.
   - `add_actor`'s schema says `use_ability refuses this actor any ability
     not named here` (VTT-703), which a give makes false: G4.
   - VTT-706 says each fold holds an actor's list "as its `ActorAdded` gave
     them", which a give makes false read as a standing rule: G4.
   - The owner's OD-5 of 2026-11-13 (an ability a window cannot read is
     drawn by its id) is narrowed by OD-5 below for an ability the window
     read once; the plan says so there.
   - SPEC-015 says `watchable` names each party member with its name;
     OD-3's recommendation adds the controlled ones' ability ids: G5.
6. **The records the work moves are named. By command, then reading.**
   Every path under "Specifications this moves" resolves; none is `New:`.
   SPEC-015 moves under OD-3's recommendation and is not listed (G5).
   SPEC-021 is listed, and by reading no sentence of it describes a
   sheet's list or the comparison `Resume` makes; it moves at most in the
   tests it names (G5).

**Split: not offered.** The owner ruled on 2026-11-14 that it is one
ticket. The tasks are ordered so each commit passes its hooks.

## The gaps this plan inherits

- **G1. `playerRules` is named under "What it touches" and must not
  change.** `TestEveryPlayerCommandHasARule` refuses a rule for a command
  no player may issue, and both commands are the DM's and the agent's.
  Task 3 adds no player rule.
- **G2. Files the work reaches that the ticket does not name.**
  - The tests that walk a oneof and fail on a new arm (P5):
    `internal/perceive/projection_internal_test.go`
    (`TestEveryEnvelopePayloadArmHasAnExplicitRuling`,
    `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable`);
    `internal/gateway/authz_test.go` (`commandFor`,
    `TestEveryClientCommandHasRoleCells`); `convert_test.go`
    (`TestEveryClientCommandConverts`); `internal/mcp`'s registration and
    fuzz walks; `client/test/command-surface.test.ts` (`COMMAND_SURFACE`,
    whose every row's control must render).
  - `client/src/view/spectator.ts` is not "if": `describe` has no
    `default`, so a new payload fails `client:typecheck` until it is
    worded (VTT-476), and the Story draws every entry of a window's log
    but a re-introduction run (VTT-662). OD-4 is the narrower question.
  - `contract/roundtrip_test.go` and new `contract/testdata` fixtures,
    which the client's tests read as hashed inputs (run
    `ts_mutation_inputs_test` before each commit that adds one).
  - `client/src/metadata.ts`'s `Ability`, which the console now takes;
    the file itself is unchanged.
  - Under OD-3's recommendation: `internal/gateway/viewpoint.go`
    (`watchable`), `client/src/session.ts`'s set handler, and the
    `server_frame_watchable_changed*.json` fixtures and the tests that
    hold a set.
  - `internal/eventgen/model.go`, if D11 is kept.
  - `scenarios/toy-brawl.json` and `scenarios/goldens/toy-brawl` (D10).
- **G3. The rules list omits one rule item 4 states:** with no ruleset
  served, a give is read against none. R7 holds it.
- **G4. Register rows and texts that become false.** VTT-703 and the
  `add_actor` schema sentence it pins (P9), in Task 3; VTT-706, in Task 1;
  VTT-707, VTT-708 and, under OD-2, VTT-699, in Task 2. Each is reworded or
  withdrawn and succeeded in the task that makes it false, so every row is
  true at every commit.
- **G5. SPEC-015 moves under OD-3's recommendation and is not listed;
  SPEC-021 is listed and has nothing to move but its pins.** Task 7 moves
  SPEC-015 and reads SPEC-021 against the tree.
- **G6. The alpha campaign.** `~/dev/vtt-alpha/adventures/two-rooms`
  declares `dnd45e-minimal`, and none of its actor files names an ability
  (P8). OD-7.

## Measurements this plan stands on

- **P1.** A test placed in `internal/perceive` by `go test -overlay`, its
  file in the session's scratchpad: a world with a scene and the party
  members `a1` and `a2`, each naming `x`, each granted and each with a
  token in sight of the other; `a2`'s log holds `a1` whole. `a1`'s token
  is removed, so `a2` no longer sees it; then the world's `a1` is given
  `y` by assignment, as no event can yet; then a `NarrationAdded`. On
  `12bda75` both `a1`'s own log and `a2`'s are sent `ActorRemoved(a1)`,
  `ActorAdded(a1)` and the narration. `git status` after it shows only the
  ticket.
- **P2.** On `12bda75`, `go test -run` passes
  `TestAUseTheCastersSheetDoesNotNameIsRefusedForEveryRole` and
  `TestQAUseEachRoleIsRefusedAnAbilityTheSheetDoesNotName`, and
  `go test ./internal/perceive/` passes.
- **P3.** `git grep -n -i -E
  'give_?ability|take_?ability|AbilityGiven|AbilityTaken' -- contract/vtt
  internal cmd client/src tools` prints nothing.
- **P4.** The callers in check 4, by `git grep -n -F` over `'*.go'
  ':!*_test.go' ':!contract/gen' ':!contract-spike'` and over `client/src`.
- **P5.** `git grep -n -E 'func TestEvery|Oneofs\(\)\.ByName' --
  '*_test.go'` prints the walkers in G2; `COMMAND_SURFACE` asserts that the
  generated oneof and its table name the same set.
- **P6.** By reading `internal/gateway/seat.go` and `server.go`: `resolve`
  recomputes `watchable(s.p, actors)` at every judgment, `recheck` runs it
  at every wake an append raises, and `tell` queues the set whenever it is
  not `proto.Equal` to the last one queued, before any entry of the append
  that changed it (SPEC-015, SPEC-007).
- **P7.** MapTool at RPTool `maptool` `f4b7fef6c`: `Token.Update` holds
  `saveMacro` and `deleteMacro`, and `Token.updateProperty` applies them
  one macro at a time; `MacroFunctions` and `MacroButtonPopupMenu` call
  `updateTokenProperty` with them; `ServerMessageHandler` answers
  `UPDATE_TOKEN_PROPERTY_MSG` by updating its own copy and
  `sendToClients(id, msg)`, every client but the sender; `SelectionPanel`
  redraws on `TokenMacroChanged`.
- **P8.** `~/dev/vtt-alpha/adventures/two-rooms/adventure.json` names
  `dnd45e-minimal`, and no file under its `actors/` has `ability_ids`
  (read with Python).
- **P9.** `TestAddActorSaysAnActorUsesOnlyTheAbilitiesItNames` and
  `TestQAUseTheServedAddActorSchemaSaysAnActorUsesOnlyWhatItsAbilityIdsName`
  pin the fragment `not named here` in `add_actor`'s `abilityIds` doc.
- **P10.** Two projection tests pin what D5 changes:
  `TestACompanionNamingOnlyAbilitiesIsBroughtWholeAtItsFirstSight` asserts
  the list-only companion is withdrawn and introduced again at first sight;
  `TestACompanionAddedAgainWithAnotherListOfItsLengthIsSeenWithIt`'s case
  `fists, rally` re-adds the same ids in another order and asserts the
  log's order with `sameStrings` and `proto.Equal`
  (`sheetHeldAsTheWorldHoldsIt`). No test in `qa_actor_abilities_shown_test.go`
  asserts a withdrawal.
- **P11.** `scenarios/toy-brawl.json` serves `tavern-brawl`, whose
  abilities are `chair-swing`, `fists`, `sober-up` and `splash-of-water`,
  and both its actors name every one.
- **P12.** `.semgrep/vocabulary.yml` bans none of `give`, `take` or
  `ability`.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no gate moves: a test whose assertion changes
  changes because a decision here changed the rule, and the task says
  which; a seed in `characterLogSeeds` may be replaced, as its comment
  directs, and no guard's threshold moves); 3 (new messages and new arms,
  nothing renumbered: `ClientCommand.command` takes 37 and 38,
  `Envelope.payload` 36 and 37, `WatchableActor` field 3, never the gaps
  at 11, 16 and 17; `check:breaking` reports, it does not enforce); 4
  (`engine.Apply` is the one fold: the new arms are in it, and the
  projection's correction frames fold through it); 5 (`give`, `take`,
  `ability` pass semgrep, P12; `hp` stays out of Go tests); 8 and 10
  (names, not line numbers; a comment is a warning or a pointer, and one
  this change makes false is rewritten: `sameIncarnation`'s and
  `actorBelief`'s docs, `TestEveryClientCommandConverts`'s list where it
  speaks of commands); 9 (answered below).
- **Principle 4.** The fold judges each give and take against the live
  state; the gateway's validator judges what only it can, against the
  served ruleset.
- **Principle 6.** The console's offer is an aid; the server's refusal is
  what stands between a participant and a change.
- **Principle 34.** Every tool description says what the server does at
  each commit that changes it (D9).
- **ADR-009.** A golden is derived by hand first: `state.json` is edited by
  hand before `stream.json` is recorded (`scenarios/goldens/README.md`).
- **The project's mechanical gates**, as
  `docs/superpowers/plans/2026-11-05-a-ruleset-names-its-properties.md`
  lists them under "Constraints that bind every task", and the Go mutation
  gate over `tools/check-mutation.py`'s `PACKAGES`, which hold
  `internal/engine`, `internal/gateway`, `internal/perceive`,
  `internal/campaign`, `internal/rules` and `internal/mcp`. A client change
  is committed with its `cmd/vtt/webdist`, rebuilt with
  `task build:client`; client tests are measured with `env -u CLAUDECODE`;
  `tools/check-new-prose` and `python3 tools/check-comments.py` run before
  each commit; stage and commit in separate calls.

## Rule 9: how MapTool does this

The ticket records MapTool's answer, and P7 confirms it at RPTool `maptool`
`f4b7fef6c`:
- A token holds its own macros. One is added or removed at a time, by a
  client, through `updateTokenProperty` with `saveMacro` or `deleteMacro`;
  the server updates its copy and relays the message to every other
  client, and each redraws the selection panel on `TokenMacroChanged`.
  Nothing records the change and nobody is told.
- **Borrowed:** the granularity, one ability given or taken on one
  character at a time (OD-1); and a redraw on the change signal the client
  is already sent, rather than a reload (OD-3).
- **Not borrowed:** the client's own judgment of who may change a macro
  (here the server's role table and fold judge, principle 7); the absence
  of a record (each change is an event, principles 2 and 3); every client
  sent every token's macros (a log is sent a change only while its eyes see
  the actor, principles 16 and 17); the copy on the token (the sheet names
  the ruleset's entry, principle 27); and the silence (the player is told,
  principle 12 and the owner's ruling of 2026-11-14).

## Decisions

1. **D1. The contract, under OD-1's recommendation.**
   - `GiveAbility { string actor_id = 1; string ability_id = 2; }` and
     `TakeAbility { string actor_id = 1; string ability_id = 2; }`, arms
     `give_ability = 37` and `take_ability = 38` of
     `ClientCommand.command`.
   - `AbilityGiven { string actor_id = 1; string ability_id = 2; }` and
     `AbilityTaken { string actor_id = 1; string ability_id = 2; }`, arms
     `ability_given = 36` and `ability_taken = 37` of `Envelope.payload`.
   - Each message's comment is a pointer to SPEC-025, the arms' to SPEC-007.
   - **Forced by** rule 3 (imperative commands, past-tense events, new
     numbers above every number in use, never a gap); the owner's words
     "give" and "take", since "grant" and "revoke" already name control.
     A message for each act rather than one with a flag is a preference:
     each reads as the act it records.
   - Task 1 adds the events, Task 3 the commands, so each commit's
     oneof walkers pass with the code that answers them.
2. **D2. The fold.** `engine.Apply` and `client/src/fold.ts` each gain an
   arm per event, validating before they change anything:
   - `AbilityGiven`: refused when its ability id is empty
     (`engine: ability_given requires an ability id`), then for an actor
     the state does not hold (`engine: ability_given names unknown actor
     "<actor>"`), then for an id the actor's `ability_ids` already names
     (`engine: actor "<actor>" already names ability "<id>"`); otherwise
     the id is appended at the end of the list.
   - `AbilityTaken`: refused for an actor the state does not hold
     (`engine: ability_taken names unknown actor "<actor>"`), then for an id
     the list does not name (`engine: actor "<actor>" does not name ability
     "<id>"`); otherwise every occurrence of the id leaves the list, the
     others keeping their order.
   - Each arm writes a fresh slice, as the `ActorControlRevoked` arm does,
     and touches no other field of the actor and nothing else of the state.
   - The client fold's refusals are `FoldError`s in its own words, as its
     condition arms are; what both folds refuse is the same.
   - **Forced by** principle 4 (the fold judges against the whole record
     before it, so a give the gateway's snapshot judged stale is refused
     here rather than appended twice); item 3; item 2's "afterwards the
     sheet does not name it", with the owner's 2026-11-13 acceptance of a
     repeated id from `ActorAdded`; and the condition arms' shape, which
     this mirrors. Appending at the end is a preference: it is where the
     record puts the newest.
3. **D3. The gateway.**
   - `commandName` gains `give_ability` and `take_ability`; `commandRoles`
     gives each to the DM and the agent; `playerRules` gains nothing (G1).
   - `ToEvent` converts each to its event, copying both fields.
   - `validateGiveAbility(cmd *vttv1.GiveAbility, rs *rules.Ruleset) error`
     in a new `internal/gateway/ability_validate.go`, beside
     `add_actor_validate.go`: with a ruleset served, it asks
     `rs.SheetAbilityFault([]string{id})` and refuses with
     `gateway: give_ability: actor "<actor>": ability_ids: <reason>`, the
     shape `validateAddActorSheet` gives; with none it reads nothing.
     `handleCommand` calls it after `validateAddActorWith`.
   - A take is read against no ruleset: a sheet served with one names only
     ids it holds (SPEC-025), and the fold refuses an id the sheet does not
     name.
   - **Forced by** SPEC-013 (validators after `Authorize`, for every role,
     before anything is written, never in `ToEvent` or the fold, which
     knows no ruleset); SPEC-025 and the 2026-09-23 ruling (a sheet is read
     against its ruleset through `SheetAbilityFault`); item 4's "naming
     the ability, as `add_actor` names one". That the ruleset's refusal
     comes before the fold's unknown actor is a preference: the DM and the
     agent are sent the world, so no order is an oracle.
4. **D4. What a character log is sent of a give or a take.** `classify`
   gains one arm for both payloads: `passIf(now.party[id] && saw(id))`,
   the `ResourceChanged` arm's test. `forwardable` keeps both fields: an
   event names its actor and its ability and no cause.
   - So the changed party member's own log is sent it, as is a log that
     saw it before the event, save a log whose decision starts afresh at
     that event, which D5's correction brings to the change instead; a log
     that did not see it is sent nothing; and no log is sent a creature's,
     a log whose eyes see the creature included, since `shownWith` holds a
     creature without its list.
   - **Forced by** item 7, principle 17 (a status change reaches a log
     only while its eyes see the actor), and SPEC-016's rule that a
     payload added to the contract needs an arm in `classify`.
5. **D5. The list leaves `sameIncarnation` and joins the belief.**
   - `sameIncarnation` stops comparing `ability_ids`; its doc stays true,
     since a give now writes the list.
   - `actorBelief` gains the list as the viewer's fold holds it;
     `beliefOf` copies it.
   - `statusFrames`, after its conditions and before its controllers,
     sends one `AbilityTaken` per id the belief holds and the actor does
     not, in the belief's order, each id once, then one `AbilityGiven` per
     id the actor holds and the belief does not, in the actor's order,
     each once, comparing the lists as sets (OD-2's recommendation). Each
     frame carries the event's sequence and nothing else of an envelope,
     as its other frames do.
   - `Resume` is unchanged in code: it asks `sameIncarnation` and then
     `statusFrames`, so a fold that holds each actor in sight with the
     world's ids resumes, and one that lags them starts afresh and is
     corrected.
   - So a log never reads a give or a take as a removal and an addition:
     `recall` marks no actor `gone` or `reshaped` for its list, and a log
     that did not see the change is brought to it by takes and gives at
     its next sight. A party member held unseen whose sheet holds a list
     and no other value is brought to it by gives at its first sight,
     rather than withdrawn and introduced again, and one with other values
     is withdrawn and introduced whole as today.
   - **Forced by** item 7 and P1; SPEC-016's definition of
     `sameIncarnation` and of the belief ("what an event can change of an
     actor"); the previous plan's D4, which recorded this move as this
     ticket's. Placing the frames after the conditions is a preference.
6. **D6. The Story's words, under OD-4's recommendation.** `describe`
   words `abilityGiven` as "<actor> can now use <ability>" and
   `abilityTaken` as "<actor> can no longer use <ability>", the actor and
   the ability as the window's `Namer` names them. **Forced by**
   `describe`'s missing `default` (VTT-476) and principle 13; the words
   are a preference, matching the toast's (D8).
7. **D7. The console's controls.** `renderDMConsole` gains, after "Remove
   condition" and before "Who controls what", an "Abilities" group drawn
   whenever the world holds an actor: one row per actor of the world, by
   id (`data-actor`), with its name, else its id; then one chip per
   distinct id its sheet names, in the sheet's order, labelled the
   ability's display name, else its id, and "✕" (`data-action`
   `take-ability`), sending `take_ability`; then, when the console's list
   holds an ability the sheet does not name, a `<select>` of those, in the
   list's order, with a blank first option, remembered in `draft` across
   re-renders as the grant row's is, and a "Give" button (`data-action`
   `give-ability`) sending `give_ability`, or, with nothing chosen,
   notifying "Choose an ability to give first." Both go through `sending`,
   keyed per actor and per ability. `DMDeps` gains
   `abilities: readonly Ability[]`, which `app.ts` hands it from its
   newest `/api/ruleset` answer, every ability for the DM and the agent.
   - With no ruleset served the list is empty, so the console offers no
     give: a use resolves nothing then (VTT-132), and the agent can still
     give any id.
   - **Forced by** item 10 and `COMMAND_SURFACE`, whose rows need a control
     that renders; the placement beside "Remove condition", the per-actor
     row and the words are preferences.
8. **D8. How the player is told and the window reads again, under OD-3's
   recommendation.**
   - `WatchableActor` gains `repeated string ability_ids = 3`. `watchable`
     fills it, with a copy of the actor's list, for each party member it
     names that `controls` names the participant in, and leaves it empty
     for any other. A spectator's set and the set of a player who
     controls none carry no ids.
   - Since `tell` sends the set whenever it differs (P6), a give or a take
     on a party member a player controls reaches every connection of
     theirs at the judgment after its append, before its entry, whichever
     character the connection watches; a redial after one is sent the new
     set, and the client's kept set tells it what changed meanwhile.
   - `Session` gains `onSheetChanged(fn)`: when a set arrives and neither
     it nor the previous one says the participant controls none, for each
     actor in both, in the new set's order, the ids the new set holds that
     the previous did not, in its order, and those the previous held that
     the new does not, in its order; handed before the set is replaced,
     as `onPartyMembersLeft`'s are. A page's first set, an equal set, a
     renamed one and a set beside a flag that is true hand over nothing.
   - `app.ts`'s `watchKey`, for a player's window, keys on each watchable
     actor's id and its ids, beside the flag, so such a set issues a read.
   - In a window whose role the page last read is `player`, each change
     handed over is kept until the read it issued is answered, or fails,
     and then shown as the toast: per actor, in the set's order, "<Name>
     can now use <A>." for the given ids and "<Name> can no longer use
     <B>." for the taken ones, several joined by " and ", the sentences by
     a space, the name the set gives, else "An unnamed character"; each
     ability named as OD-5 says. `sheetNews`, a pure function in
     `client/src/view/naming.ts`, writes the words.
   - A give and a take of one id between two judgments leave the set equal
     and tell nobody; near-theoretical, no code (the 2026-10-21 ruling). A
     controlled creature's change neither tells nor re-reads: the panel
     never acts as a creature (SPEC-022), the same near-theoretical case
     the previous plan's D8 left.
   - **Forced by** principle 12 ("when they happen") and the owner's ruling
     that the player is told: a player watching one of their characters is
     sent nothing of another's log, and the window has no other signal
     that is safe on replay (`Wire` ignores `CatchUpHead`, and a switch
     sends the new log from its first entry, so a toast on a folded event
     would repeat at every load). Principle 16 holds: a player reads their
     own party members' lists already, through `/api/ruleset`'s union and
     each one's own log. The words are a preference.
9. **D9. What the system says about itself (principle 34).**
   - `give_ability`, at Task 3: "Give an actor one ability of the served
     ruleset (get_ruleset lists them): afterwards its sheet names the
     ability and use_ability resolves it for that actor. Only its ability
     list changes: its tokens, controllers, conditions and resources stay
     as they are, so use this, not remove_actor and add_actor, to change
     what a character can do. Refused, appending nothing: an actor the
     campaign does not hold, an ability its sheet already names, and, with
     a ruleset served, an id the ruleset does not hold. DM/agent only."
   - `take_ability`, at Task 3: "Take one ability from an actor's sheet:
     afterwards its sheet does not name it and use_ability refuses that
     actor the ability, whoever asks. Only its ability list changes.
     Refused, appending nothing: an actor the campaign does not hold, and
     an ability its sheet does not name. DM/agent only."
   - Both, at Task 5, gain: "Each player who controls the actor while it
     is a party member is told."
   - Both keep the derived required list, `actorId` and `abilityId`, each
     genuinely required; field docs: `actorId` "The id of the actor whose
     sheet changes."; `abilityId` "The ability's id in the served ruleset."
   - `add_actor`'s `abilityIds`, at Task 3: "Optional. The abilities this
     actor has, each by its id in the served ruleset (get_ruleset lists
     them). use_ability refuses this actor any ability its sheet does not
     name, so an actor that names none can use none until give_ability
     gives it one; give_ability and take_ability change the list in play.
     With a ruleset served, an id the ruleset does not hold refuses the
     whole command."
   - `get_ruleset`'s description and both rulesets' guides stay as they
     are: each is true after this ticket.
   - **Forced by** principle 34, P9 and each commit's truth.
10. **D10. The goldens.** `scenarios/toy-brawl.json` gains, before its
    `endSession`: the DM takes `sober-up` from `act-patron`; the patron's
    give and the spectator's take, each `deniedContaining` `not
    authorized`; the DM's give of `no-such-move`, denied naming it; the
    DM's take of `sober-up` again, denied as not named; the DM's give of
    `sober-up`, ok. `act-patron` then names `chair-swing`, `fists`,
    `splash-of-water`, `sober-up`, which the hand-derived `state.json`
    holds. **Forced by** item 8 and P11 (every tavern-brawl ability is
    named, so a give needs a take first); the scenario chosen because it
    serves a ruleset and has a golden is a preference.
11. **D11. Property histories carry gives and takes.** `internal/eventgen`
    gains `giveAbility` and `takeAbility` draws, each drawing the refusal
    its fold makes (`MustFail`: a give of an id already named, a take of
    one not named), so `TestEveryCharacterLogFoldsAtEveryPrefix` and the
    character-log oracle cover the new arms over random histories. A
    seed whose walk no longer meets the guards is replaced, as the seeds'
    comment directs. **A preference**, with the reason that SPEC-021's
    oracle is the projection's only cross-history check.
12. **D12. SPEC-018, under OD-6's recommendation.** Neither fold bounds
    an ability id's length. The second table gains `AbilityGiven.ability_id`
    ("with a ruleset served, `give_ability` refuses an id the ruleset does
    not hold, and `internal/rules/load.go` bounds an ability id at
    `maxIDBytes`; with none, the WebSocket frame alone, for `give_ability`,
    a command of the DM and the agent; both folds refuse an empty one"),
    and its first row gains `AbilityGiven.actor_id`, `AbilityTaken.actor_id`
    and `AbilityTaken.ability_id`, each of which the fold refuses when it
    names nothing it has folded.

## Open decisions for the owner

- **OD-1. One event per ability, or one carrying the new list.
  Recommended: A.**
  - **A: `AbilityGiven` and `AbilityTaken`, each naming one actor and one
    ability (D1).** The fold judges each against the live state, so two
    gives sent at once both land or the second is refused, never lost
    (principle 4); the record says what happened, "given X" (principle 3);
    the projection corrects with the pair, as it corrects conditions; the
    toast, the Story and the console each read the act from the event.
    It is MapTool's granularity (P7).
  - **B: one event carrying the sheet's whole new list.** Its correction is
    one frame and a log's order always equals the world's. Cost: the
    gateway writes the list from its snapshot, so two gives sent at once
    can each write a list without the other's id, and the fold cannot tell
    (principle 4); the Story and the message must diff the list against
    the window's fold to say what changed.
- **OD-2. Whether a character log holds the list in the world's order.
  Recommended: A.**
  - **A: the same ids, compared as a set.** A log that did not see a change
    is corrected by takes and gives, and may then hold the ids in another
    order than the world, as SPEC-016 already lets it hold conditions and
    controllers; nothing reads the order (the panel offers in the ruleset
    list's order, `Offers` and the read ask membership). A take and a give
    back of one id while the log did not see it sends nothing at sight
    ("A change undone while the viewer did not see the actor sends
    nothing"). Cost: P10's reorder case and `sheetHeldAsTheWorldHoldsIt`
    compare the list as a set, and VTT-699's and VTT-707's wording says
    so.
  - **B: the world's order.** A correction that cannot reach it by takes
    and gives appended at the end either withdraws and re-introduces the
    actor, which item 7 forbids for a give or a take, or takes and gives
    again ids the world kept, which the Story then draws as changes that
    never happened.
- **OD-3. How the player is told, and how their window reads again.
  Recommended: A.**
  - **A: the watchable set carries the controlled party members' ids
    (D8).** Told at once, whichever character the window watches, after a
    redial too; the same change issues the read; one contract field,
    `watchable` and the client's set handler. SPEC-007 and SPEC-015 move.
  - **B: through the log alone.** The Story line (OD-4) is the message,
    and the key reads the ids the window's fold holds for the actors the
    player controls. No contract field. Cost: a player watching another
    of their characters learns of it only by switching to the one changed,
    which the alpha table, where one player plays more than one
    character, meets at once;
    principle 12's "when they happen" holds only for the watched one.
  - **C: a new `ServerFrame` arm sent to the controllers' connections.**
    The same effect as A, with a frame of its own and a second producer
    beside `tell` in the pump.
- **OD-4. Whether the Story draws a line for a give and a take.
  Recommended: A.**
  - **A: in every window whose log holds the event**, as for every entry
    (VTT-662): the DM's Story and Events panel, the log of the character
    changed and of each that saw it, and a correction's frames where a log
    is brought to the change at sight (D6). No new rule.
  - **B: none in a player's or a spectator's Story**: a new exclusion in
    `buildFeed`, and the toast alone tells the player.
  - **C: none in any Story**: the same exclusion for the DM too, who then
    reads it only in the Events panel.
- **OD-5. What a window calls an ability it no longer reads.
  Recommended: B.**
  - **A: its id**, as the 2026-11-13 OD-5 ruled for what a window cannot
    read. No code. After the read a take issues, the player's Story draws
    "Lera can no longer use longsword-strike", and her earlier uses of it
    by id, beside a toast that named it (principle 13).
  - **B: by every ruleset list the window has read since it loaded.**
    `app.ts` keeps each ability's newest display name from every answer,
    and hands the Story's `Namer` and `sheetNews` that list, while the
    panel keeps the newest answer alone. An ability the window never read
    is still drawn by its id, as A; a reload forgets what an earlier
    answer named. This narrows the 2026-11-13 OD-5 for abilities the
    window was once allowed to read.
  - **C: the event carries the ability's display name**, written by the
    gateway from the served ruleset, as `AbilityUsed.outcome_summary`
    does. Named after a reload too. A contract field the fold never
    reads, and the previous plan's OD-5 C left a name in an event to step 1
    item 3.
- **OD-6. SPEC-018's bound on the new ability id. Recommended: A.**
  - **A: no length bound; an empty id refused (D12).** The give is the DM's
    and the agent's, so neither of SPEC-018's tests is met, and the list it
    adds to is unbounded on purpose (the 2026-11-13 OD-4, the 2026-10-09
    ruling).
  - **B: 1 to 128 bytes in both folds**, as `AbilityUsed.ability_id` is.
    It joins the first table without meeting either test, and a sheet
    could then hold, from an `ActorAdded` with no ruleset, an id no give
    could write back.
- **OD-7. The alpha campaign (G6). Recommended: both, outside this
  ticket.** The running campaign seeded from `two-rooms` is brought up to
  date by `give_ability`, through the agent, once this lands (principle
  26: the DM's); the adventure's actor files gain `ability_ids` before it
  seeds another campaign. Neither is in the repository.

## Requirements

Sorted and allocated at Task 0, 2026-11-14, after the owner's sign-off. The
plan's tasks keep the `R` labels; the register carries the ids. Every row is
new work and reads `**OPEN — no test yet**` until its task cites its tests.
The sort split R8, which held two rules, into R8a and R8b.

| R | Id | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|---|
| R1 | VTT-713 | 1 | A `give_ability` from the DM or the agent naming an actor the campaign holds and an ability its sheet does not name, with a ruleset served one it holds, appends one `AbilityGiven`, after which the actor's `ability_ids` end with that id | nothing appended, or the list unchanged |
| R2 | VTT-714 | 2 | A `take_ability` from the DM or the agent naming an actor and an ability its sheet names appends one `AbilityTaken`, after which the sheet names no occurrence of it and keeps its other ids in their order | an occurrence kept, another id moved or dropped |
| R3 | VTT-715 | 3 | Both folds change nothing of an actor at an `AbilityGiven` or an `AbilityTaken` but its `ability_ids`, and nothing else of the state | another field differs |
| R4 | VTT-716 | 4 | A `give_ability` or a `take_ability` from a player or a spectator is refused as not authorized and appends nothing | appended |
| R5 | VTT-717 | 4 | Both folds refuse an `AbilityGiven` or an `AbilityTaken` naming an actor they do not hold, so the command appends nothing | appended |
| R6 | VTT-718 | 4 | With a ruleset served, a `give_ability` of an id the ruleset does not hold is refused, naming the actor and the id, and appends nothing | appended |
| R7 | VTT-719 | 4 | With no ruleset served, a `give_ability` is read against none | refused for the id |
| R8a | VTT-720 | 4 | Both folds refuse an `AbilityGiven` of an id the sheet names | appended |
| R8b | VTT-721 | 4 | Both folds refuse an `AbilityTaken` of an id the sheet does not name | appended |
| R9 | VTT-722 | OD-6 | Both folds refuse an `AbilityGiven` whose ability id is empty | appended |
| R10 | VTT-723 | 7 | A character log is sent an `AbilityGiven` or an `AbilityTaken` of a party member, less its issuer, exactly when its eyes saw it before the event, and never one of an actor that is not a party member | sent out of sight, or a creature's |
| R11 | VTT-724 | 7 | A character log that holds a party member whose ids differ, as a set, from the world's is sent at the first event its eyes see it one `AbilityTaken` per id it holds that the world does not and one `AbilityGiven` per id the world holds that it does not | the list left stale |
| R12 | VTT-725 | 7 | No character log is sent an `ActorRemoved` or an `ActorAdded` of an actor because its ability ids differ from the world's | a withdrawal or an introduction sent (P1) |
| R13 | VTT-726 | 7 | `Resume` resumes a log whose fold holds each actor in sight with the world's ability ids, in any order, and starts afresh from one whose fold, for an actor in sight, lacks an id the world's names or names one the world's does not | starts afresh, or resumes a lagging log |
| R14 | VTT-727 | 8 | The client's fold of an `AbilityGiven` and an `AbilityTaken` agrees with `engine.Apply`'s, refusals included, as `vtt state dump` prints it | a dump differs |
| R15 | VTT-728 | 9 | The `give_ability` and `take_ability` tools say they are the DM's and the agent's, what each refuses, and that nothing else about the actor changes | a sentence missing |
| R16 | VTT-729 | D9 | The `add_actor` schema says `use_ability` refuses an actor any ability its sheet does not name, so one naming none can use none until given one, and that `give_ability` and `take_ability` change the list (succeeds VTT-703) | "not named here" |
| R17 | VTT-730 | 10 | The DM console offers, for each actor, a give of each ability of its list the sheet does not name and a take of each id the sheet names, and sends `give_ability` or `take_ability` | an offer missing, or a named ability offered to give |
| R18 | VTT-731 | 5 | A `WatchableChanged` carries, for each party member the participant controls, the ids its sheet names, and none for any other, and is sent again when they change | a spectator sent ids, or no new set |
| R19 | VTT-732 | 5 | A player's window reads `/api/ruleset` again when a `WatchableChanged` changes the ids of a party member it names | the old list kept |
| R20 | VTT-733 | 6 | A player's window shows a toast naming the character and each ability given and taken when a set changes a controlled party member's ids, and none for a page's first set, an equal set, or a set beside one saying the participant controls none | no toast, or a false one |
| R21 | VTT-734 | 6 | The toast and the Story name an ability by the display name any ruleset list the window has read since it loaded gives it, else by its id (OD-5, B) | an id where a list read gave a name |
| R22 | VTT-735 | 9 | The `give_ability` and `take_ability` tools say each player who controls the actor while it is a party member is told | the sentence missing |
| R23 | VTT-736 | 6 | The Story words an `abilityGiven` "<actor> can now use <ability>" and an `abilityTaken` "<actor> can no longer use <ability>" | other words, or the payload's name |

**Rejected at the sort**, each with what already holds it:
- A use of a given ability resolves, and a use of a taken one is refused:
  VTT-690 and VTT-137 hold, through `rules.Offers` on the snapshot.
- `/api/ruleset` answers a player the new set at the next read: VTT-691,
  decided per request.
- The panel stops offering a taken ability, and offers a given one once
  the list holds it: VTT-695, filtering by the sheet in the fold.
- A give or a take's frames carry the event's sequence: SPEC-016's rows
  for every frame.
- A controlled creature's change tells nobody and issues no read: D8,
  near-theoretical.

**Rows a task withdraws or rewords**, each in the task that makes it false:
- VTT-706 ("as its `ActorAdded` gave them"), in Task 1: reworded or
  succeeded to "until an `AbilityGiven` or an `AbilityTaken` changes them".
- VTT-707 and VTT-708 ("or ability ids"), and VTT-699 under OD-2 (the
  ids, as a set), in Task 2, each succeeded by a row that says what D5
  makes true; the tests citing them cite the row each holds.
- VTT-703, in Task 3, succeeded by R16.
- VTT-696 keeps its row; R19 adds the ids beside it.

## The existing tests and data that change

By search; each task reruns its own packages first and names what turned red.
- **The oneof walkers** (P5): each gains its case in the task that adds the
  arm it walks (Task 1 for the events, Task 3 for the commands).
- **P10's tests** (Task 2): the first is re-pointed to the correction (no
  `ActorRemoved`, the list held at first sight); the reorder case and
  `sheetHeldAsTheWorldHoldsIt` compare the list as a set, under OD-2.
- **P9's tests** (Task 3): their fragment becomes R16's words.
- **The tool golden** (Tasks 3 and 5): `contract/testdata/expected_tools.json`,
  `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`.
- **The sets a test holds** (Task 5): the `server_frame_watchable_changed*`
  fixtures, and each Go and client test that compares a player's set whose
  party members name abilities, as Task 5's search finds them.
- **The data** (Task 4): `scenarios/toy-brawl.json` and
  `scenarios/goldens/toy-brawl`.
- **Unchanged:** `rules.Resolve` and its tests; `SheetAbilityFault` and
  `Offers`; `abilitiesReadBy`; `introduce`, `shownWith` and `unseenAs`;
  every ruleset's guide; `get_ruleset`'s description.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** the owner answers OD-1 to OD-7; run the `requirements` sort over
  R1 to R23; allocate ids; commit the ticket, this plan and
  `docs/requirements.md`.
- **True when:** `git status` is clean and the requirements chain passes.

### Task 1: the events, in both folds and at the log's door (R3, R5, R8a, R8b, R9, R10, R14's fold half, R23; D1, D2, D4, D6)

- **Files:** `contract/vtt/v1/events.proto` (`AbilityGiven`,
  `AbilityTaken`, `Envelope.payload` 36 and 37); what
  `task generate:contract` writes under `contract/gen`;
  `contract/roundtrip_test.go` and `contract/testdata` fixtures for both
  envelopes; `internal/engine/apply.go` (both arms) and a new
  `internal/engine` test; `client/src/fold.ts` (both arms) and a client
  fold test; `internal/perceive/projection.go` (`classify`'s arm);
  `projection_internal_test.go`'s forwardable list; new `internal/perceive`
  tests for D4: forwarded to the changed party member's own log and to a
  log that saw it, withheld from one that did not, withheld for a
  creature a log sees; `client/src/view/spectator.ts` (`describe`) and the
  Story's every-kind tests; `docs/requirements.md` (VTT-706); a rebuilt
  `cmd/vtt/webdist`.
- **True when:** each fold appends a given id, removes every occurrence of a
  taken one, refuses in D2's order and words, and leaves every other field
  as it was, compared field by field; `TestEveryEnvelopePayloadArmHasAnExplicitRuling`
  passes; `client:typecheck` passes; `git diff main -- contract/vtt` adds
  `AbilityGiven`, `AbilityTaken` and their arms and changes nothing else;
  `task check:breaking` reports nothing breaking.
- **Deliberate breaks:** remove the first occurrence alone (the repeat
  test turns red in that fold); drop the duplicate refusal (the refusal
  test turns red); drop `now.party` from the arm (the creature test turns
  red); drop `saw` (the out-of-sight test turns red).
- **The state here:** no command writes either event; nothing at the table
  changes.

### Task 2: the projection (R11, R12, R13; D5, D11; OD-2; item 7)

- **Files:** `internal/perceive/projection.go` (`sameIncarnation`,
  `actorBelief`, `beliefOf`, `statusFrames`, and the docs that name the
  fields); new `internal/perceive` tests: a log that saw a party member
  given an ability holds it and is sent no `ActorRemoved` or `ActorAdded`
  of it then or at later events; a log that held it whole out of sight is
  sent nothing at the give, nothing at later events out of sight, and at
  its next sight the takes and gives that bring it to the world's set; a
  take and a give back while unseen send nothing at sight; a list-only
  party member held unseen is brought to its list by gives at first sight;
  a creature's give reaches no log and a creature that comes into sight is
  introduced without its list; `Resume` resumes from a fold holding the
  world's ids in another order and starts afresh from one that lags them;
  P10's tests re-pointed; `internal/campaign`: a campaign reopened after a
  give resumes each log rather than starting over; `internal/eventgen`
  (D11) and its model test; `docs/requirements.md` (VTT-699, VTT-707,
  VTT-708).
- **True when:** the new tests pass; every other `internal/perceive` and
  `internal/campaign` test passes unchanged;
  `TestEveryCharacterLogFoldsAtEveryPrefix` and the oracle pass with the
  new draws.
- **Deliberate breaks:** put the list back in `sameIncarnation` (the
  out-of-sight and `Resume` tests turn red, as P1's frames); drop the list
  from `statusFrames` (the sight correction and the lagging-`Resume` tests
  turn red); compare the list in order (the take-and-give-back and
  reordered-`Resume` tests turn red).

### Task 3: the commands, the tools and the console (R1, R2, R4, R6, R7, R15, R16, R17; D3, D7, D9; items 1 to 4, 9, 10)

- **Files:** `contract/vtt/v1/commands.proto` (`GiveAbility`,
  `TakeAbility`, `ClientCommand.command` 37 and 38), regenerated;
  `contract/roundtrip_test.go` and command fixtures;
  `internal/gateway/authz.go` (`commandName`, `commandRoles`) and
  `authz_test.go` (`commandFor`); `convert.go` (`ToEvent`);
  `ability_validate.go` (new); `server.go` (`handleCommand`); new
  `internal/gateway` tests through the WebSocket: a give then a use
  resolves (red on `12bda75`); a take then a use is refused for the DM, the
  agent and a player; the state before and after compared field by field;
  each refusal of item 4 with the log's length unchanged; with no ruleset,
  a give of any id accepted; `tools/toolgen/main.go` (both entries, and
  `add_actor`'s `abilityIds`) and `main_test.go`; the regenerated tool
  copies and `expected_tools.json`; P9's tests;
  `cmd/vtt/mcp_ruleset_e2e_test.go` (through the tools, a give then a use
  resolves and a take then a use is a tool error); `internal/mcp` (both
  tools registered, carrying their arguments); `client/src/commands.ts`
  (`giveAbility`, `takeAbility`) and its test; `client/src/view/dm.ts`
  (D7) and `client/src/app.ts` (the console's `abilities`);
  `client/test/command-surface.test.ts` (both rows) and a console test
  driving a give and a take; `docs/requirements.md` (VTT-703); a rebuilt
  `cmd/vtt/webdist`.
- **True when:** the new tests pass; `TestEveryClientCommandHasRoleCells`,
  `TestEveryPlayerCommandHasARule`, `TestEveryClientCommandConverts` and the
  MCP walks pass; `client:typecheck` passes; every tool text is true at
  this commit.
- **Deliberate breaks:** give a player a cell in `commandRoles` (the role
  tests turn red); drop the validator's call (the ruleset test turns red);
  offer a named ability in the console's select (the console test turns
  red).

### Task 4: the goldens (R14; D10; item 8)

- **Files:** `scenarios/toy-brawl.json`; `scenarios/goldens/toy-brawl`:
  `state.json` edited by hand first, then `stream.json` recorded, in which
  the new events appear and every sequence after them moves, its rolls
  unchanged.
- **True when:** `internal/harness.TestFoldGoldenCorpus`,
  `cmd/vtt.TestScenarioGoldenStreamsHaveNotDrifted`,
  `client/test/fold-parity.test.ts` and the corpus half of
  `TestEveryCharacterLogFoldsAtEveryPrefix` pass.
- **Deliberate break:** make the client fold insert a given id in sorted
  order (fold parity turns red).

### Task 5: the window (R18 to R22; D8; OD-3, OD-5; items 5 and 6)

- **Files:** `contract/vtt/v1/commands.proto` (`WatchableActor` field 3),
  regenerated, and a fixture of a set carrying ids;
  `internal/gateway/viewpoint.go` (`watchable`) and new `internal/gateway`
  tests: a player's set carries their party members' ids; a spectator's
  and a non-controlling player's carry none; a give to a controlled party
  member sends that player's seat a new set before its entry, whichever
  log it watches; a give to another player's party member sends this
  player's seat none; `client/src/session.ts` (`onSheetChanged`);
  `client/src/app.ts` (`watchKey`, the toast, the names kept under OD-5);
  `client/src/view/naming.ts` (`sheetNews`); client tests: the session's
  hand-over, first, equal, renamed and flag cases; the window reads again on
  changed ids; a player's panel offers a given ability after the read and
  stops offering a taken one; the toast's words for a give, a take, both,
  an unnamed character, a take whose ability the answer no longer holds,
  and a player watching another of their characters; no toast in a DM's or
  a spectator's window; `tools/toolgen/main.go` (D9's sentence),
  regenerated, `expected_tools.json` and `main_test.go`; the sets a test
  holds; a rebuilt `cmd/vtt/webdist`.
- **True when:** the new tests pass, Go and client; every VTT-605,
  VTT-606, VTT-696 and VTT-697 test passes unchanged; `client:typecheck`
  passes.
- **Deliberate breaks:** fill ids for every party member (the spectator
  test turns red); drop the ids from `watchKey` (the read test turns red);
  hand over changes beside a true flag (the flag case turns red); name
  from the newest answer alone (the take's toast names the id and turns
  red).

### Task 6: the gate

- **Do:** `python3 tools/check-comments.py main` and `--write-ledger`
  once, last, if a ceiling can come down; check free space and the load;
  run `task check` whole on the last code commit, detached under `/bin/sh`,
  output to a file, requiring its completion line; run `task e2e`. Tell
  the owner the client's stored mutation report is out of date from Task 1
  and which commit it measured (SPEC-023); a full Stryker run is his to
  ask for.
- **True when:** `task check` and `task e2e` are green (item 11).

### Task 7: the report, then the specifications

- **The report:** `docs/reports/2026-11-14-a-sheets-abilities-change-in-play.md`
  with the `implementation-report` skill, and its reading review: each
  "Done looks like" item with its observation; R1 to R23 and what turned
  each red; Rule 9; G1 to G6 and each OD's answer; Phase 4a's
  adjudications under their own heading; the gate.
- **The specifications**, with the `specification` skill, from the report:
  - **SPEC-007:** the commands, the events and their arms; the tools; under
    OD-3, `WatchableActor`'s field.
  - **SPEC-013:** the role table's rows; `validateGiveAbility` in the
    command path; the fold's refusals reach the issuer.
  - **SPEC-016:** `classify`'s arm; `sameIncarnation` without the list; the
    belief and the correction's frames; what a list-only party member is
    brought by; the order, under OD-2; the Consequences on what re-introduces
    an actor.
  - **SPEC-018 (D12):** the rows OD-6 decides.
  - **SPEC-021:** read against the tree; its pins, if Task 2's campaign
    test is cited there.
  - **SPEC-022:** the fold's arms; the Story's words; the console's group;
    the re-read on ids; the toast; the naming under OD-5.
  - **SPEC-025:** a sheet's list changes by a give and a take;
    `SheetAbilityFault`'s new caller; the Consequence on what no command
    changes, rewritten.
  - **SPEC-012:** "a grant, a revocation or a change of role" gains a give
    and a take.
  - **SPEC-015 (G5):** `watchable` carries the controlled party members'
    ids, under OD-3.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and the surfaces
as signatures and names, never the source: `go doc ./internal/engine`
(`Apply`), `go doc ./internal/perceive` (`New`, `Resume`, `Event`), `go doc
./internal/gateway` (`Authorize`, `ToEvent`), the `give_ability`,
`take_ability` and `add_actor` entries in `contract/gen/tools/tools.json`,
the `WatchableChanged` shape from SPEC-007, and the signatures of
`giveAbility`, `takeAbility`, `Session.onSheetChanged`, `sheetNews` and
`renderDMConsole`'s `DMDeps`. Tell it: never run `task check`, `task e2e`,
Stryker or gremlins; its files are new `qa_` files; semgrep bans game words
in Go tests too (`hp` among them); go-arch-lint holds the layer imports, and
`internal/perceive`'s tests import only its own package, `contract`,
`engine` and `sight`; the tree is hashed before and after it. Known and not
findings: a log may hold a list in another order than the world (OD-2); a
give and a take of one id between two judgments tell nobody, and a
controlled creature's change tells nobody (D8); no fold bounds an ability
id's length (OD-6); a reload forgets the names earlier answers gave (OD-5);
the fold's refusal words reach the DM and the agent as they are.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "never", "whichever") names the search that bounds it, P4
and P5 above all; check each comment under `internal/`, `cmd/` and
`client/src` is a warning or a pointer; check the tool descriptions against
what the server refuses and tells at that commit; check that `git diff main
-- contract/vtt` adds `GiveAbility`, `TakeAbility`, `AbilityGiven`,
`AbilityTaken`, their arms and, under OD-3, `WatchableActor`'s field 3, and
nothing else.
