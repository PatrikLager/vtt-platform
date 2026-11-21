# Plan: a participant is a player exactly while they control an actor

- **Ticket:** `docs/superpowers/specs/2026-11-20-a-spectator-never-holds-control-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-20 by an agent that did not write the ticket, against
    `c99715b` on `feat/a-spectator-never-holds-control` in the worktree
    `vtt-platform-visibility`, with the ticket not yet committed.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and `task build:client` were not run. M1 to M8 ran in
    copies of the tree outside the repository, each deleted afterwards;
    `tools/check-comments.py --report` ran in the worktree with
    `PYTHONDONTWRITEBYTECODE=1`, its `git status` the same before and after;
    the rest are searches and readings.
  - An earlier plan for an earlier shape of the ticket (before the role became
    derived) was read as leads only. Nothing here rests on it unmeasured.
- **To be committed as:** `docs/superpowers/plans/2026-11-20-a-spectator-never-holds-control.md`.
- **Base:** `c99715b`. **Branch:** `feat/a-spectator-never-holds-control`.
- **Owner's rulings taken in:**
  - 2026-11-15: a spectator must never be able to play.
  - 2026-11-20: there is no player without a character. THE LOG DECIDES: a
    participant who is not the DM or the agent is a player while the log holds
    them among the controllers of at least one actor, of any kind, and a
    spectator otherwise; a grant makes a spectator a player, the last
    revocation (by the DM, the agent, the player, `remove_actor`, removal)
    makes a player a spectator. Chosen over keeping identity's role column in
    step with the log ("two stores that can drift").
  - 2026-11-20: the console's "Make spectator" stays, takes every actor the
    player controls in one command (one `ActorControlRevoked` per actor, as
    `handleRemoval` builds them) and asks first: "Make Lera a spectator? They
    no longer control Ash and Gobo." The console offers no "Make player".
  - 2026-11-20: a grant naming a participant id identity does not hold, or
    holds revoked, is refused, appends nothing, and says "There is no such
    participant."
  - 2026-11-20: tests that give a participant control only to show that the
    ROLE decides are kept where they still mean something.
  - 2026-11-20: one ticket (asked before the role became derived); OD-6 asks
    again.
  - Standing: nobody uses the product, so no migration and no phased removal
    (2026-09-04); an almost theoretical edge case with a small consequence
    gets no code, no test and no debt (2026-10-21); refusals in plain words,
    one server text for every reader, a name in place of an id (2026-11-02);
    "equivalent", for a client mutant, means no observation at all in the DOM;
    no counts or measured figures in anything written down; a comment under
    `internal/`, `cmd/` or `client/src` is a warning or a pointer; the
    client's full mutation run on the owner's word; push and merge on the
    owner's word.
- **Signed off:** 2026-11-21 by the owner ("Ja, ett ärende, kör"), with the
  answers under "The owner's answers".
- **Ids:** allocated at Task 0 (the line "Allocated at Task 0").

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol is where it says: `CreateInvite`, `SetRole` and `Lookup`
   in `internal/identity/identity.go`; `newInviteCmd`'s `--role` in
   `cmd/vtt/invite.go`; `handleJoin` in `internal/gateway/join.go`;
   `handlePromotion` in `internal/gateway/server.go`; `controlTarget` in
   `internal/engine/apply.go`, which names no role (VTT-039's tests are in
   `internal/engine/role_test.go` and `qa_role_test.go`); `commandRoles` and
   `Authorize` in `authz.go`; `MayWatch`, `defaultViewpoint` and `watchable`
   in `viewpoint.go`; `seat.go`; `handleMe` and `handleParticipants` in
   `metadata.go`; `validateGrantActorControl` in `grant_validate.go`;
   `mayPerch` in `client/src/app.ts`; the perch's line in
   `client/src/view/spectator.ts`; `renderDMConsole`, "Make player" and "Make
   spectator" in `client/src/view/dm.ts`; "be given a character and act" and
   "writes no event" in `tools/toolgen/main.go`; the tool JSON files,
   `cmd/vtt/webdist`, `contract/gen`, both protos; rows VTT-025, VTT-029,
   VTT-036, VTT-454, VTT-455, VTT-605 to VTT-607 and VTT-731; the blueprint
   §16 sentence verbatim; the specifications it names. In `~/dev/RPTool/maptool`:
   `Player.Role` (`PLAYER`, `GM`) in `model/player/Player.java`,
   `Token.isOwner(String playerId)`, and `Players.setRole`, which writes the
   player database and touches no token.
2. **"Done" is an observation. By command, then reading; no test is named,
   so none could be run red by name.** M1's green base holds the tests that
   contradict items 1 to 7 today: `TestMetadataMeIdentifiesTheCaller` answers
   a minted player who controls nothing `player` (items 3, 1);
   `TestQACNASpectatorWhoControlsAPartyMemberIsToldSo` builds a spectator who
   controls (item 4); `TestDMGrantsControlOverTheWire` grants to an id
   nobody minted (item 5); `TestAPromotionAppendsNoEvent` (item 6); "a
   spectator can be promoted, and a player is not offered promotion again" in
   `dm-view.test.ts` (item 7). Item 8 is false today on the server
   (`announcePromotion`'s one caller is `handlePromotion`, check 4, so no
   frame follows a grant or a revocation) and cannot be made red in the
   client (G2). Item 9
   is a golden, item 10 a reading (this plan's lists are its output), item 11
   the gate.
3. **Each rule is breakable. By reading.** Each rule has an observation that
   fails (the "Red under" column). Two reservations: rule 1's "every
   decision" is held per surface (R1 to R5) and by Phase 4b's search for
   readers of the role (D2), since no single test reaches every reader; and
   the instant a racing command reads a stale snapshot has no observation a
   test can force (D10).
4. **The scope matches the claim. By command, then reading.**
   - Readers of `.Role` outside test files, by `git grep -n '\.Role\b'`:
     `authz.go` (`Authorize`, `mayVoice`, `mayWhisper`, `mayWorkDoor`,
     `errUndecided`), `viewpoint.go` (`MayWatch`, `defaultViewpoint`),
     `seat.go` (`watchesALog`, `judgedAs`), `metadata.go` (`handleMe`,
     `handleParticipants`, `abilitiesReadBy`, and the dm-and-agent route
     gates `adventureGuideRoles`, `rulesetGuideRoles`, `joinLinkRoles` and
     `participantRoles`), `server.go` (the player's sight gate, the stamps of
     `handleRemoveActor` and `handleRemoval`, and `handlePromotion`'s and
     `handleRemoval`'s refusal of a dm or agent target), `convert.go` (`ToEvent`'s stamp, `voice`), `ruleset.go`,
     `ability_change.go`, `adventure.go` and `map.go` (stamps),
     `internal/harness/engine.go` (`characterSeats`, which reads the
     scenario's declared role, not identity's), and the mints of
     `newInviteCmd` and `mintInvites` in `cmd/vtt`. `internal/mcp` reads none: it dials as the agent, and only
     `getParticipantsDescription` in `door_tools.go` names the roles.
   - Callers: `identity.SetRole` by `handlePromotion` alone;
     `presenceRegistry.rejudge` by `handlePromotion` and `handleRemoval`;
     `seat.rejudge` by `onEntry` alone; `announcePromotion` by
     `handlePromotion` alone; `CreateInvite` outside tests by `handleJoin`,
     `newInviteCmd` and `mintInvites`; `promoteParticipant` (client) by the
     roster rows alone.
   - The work reaches files and records the ticket does not name (G1), and
     its tests reach far past the files it lists (M2, M3, M6, M8), as the
     ticket says it would.
5. **No recorded decision is contradicted silently. By reading.**
   - Overturned and named by the ticket: VTT-036 and the "a promotion
     appends nothing" sentences of SPEC-007, SPEC-009 and SPEC-015; VTT-025
     and VTT-029; the owner's ruling B-4 of 2026-10-22 as far as its perch
     line goes (OD-4); the 2026-11-15 ruling's "promoted to player first",
     superseded by the 2026-11-20 ruling the ticket quotes.
   - Overturned and not named: SPEC-009's "Identity is not event-sourced.
     Participants, their roles ..." (a role that is neither dm nor agent
     becomes the log's), SPEC-015's role re-judge (`judgedAs`, `rejudge`,
     VTT-387) and its promotion bullet, SPEC-011's `rejudge` paragraph naming
     a promotion among its callers, and the `PresenceChanged` comment in
     `commands.proto` ("or ... being PROMOTED"). Said here so the next reader
     finds both.
   - Holding: VTT-039 (the derivation lives in the gateway; the fold reads no
     role); SPEC-013's "`Authorize` performs no I/O" (it reads the snapshot it
     is handed); SPEC-009's "A role never travels in a presence frame" (D8's
     frame carries none); the owner's OD-3 of 2026-11-14 (a set carries ability
     ids for every party member the participant controls: true as worded,
     with no spectator left to be an example of it); the ruling of 2026-09-06
     that the console may grant to anyone on the roster or in presence (the
     grant select is unchanged). Blueprint 7 (a change of control is a change
     of role, applied at the next action), 8 (admission stays identity's; the
     derived role only judges someone already admitted) and 22 hold.
6. **The records the work moves are named, and reachable. By command, then
   reading.** The section is present, its paths resolve, nothing is `New:`.
   A search of `docs/specifications/` for `promot`, `SetRole`,
   `announcePromotion`, `controls no party member`, `play no character` and
   `participants table` finds the ones it names and SPEC-021, whose hit
   ("controls no party member moves it nowhere", VTT-370) is read again in
   Task 9, VTT-370 being withdrawn (D25). A reading finds one more the work moves: SPEC-017, whose
   `handleRemoveActor` gains D8's announcement (G1).

## The gaps this plan inherits

- **G1. Files and records the ticket does not name:**
  - `internal/gateway/convert.go` (`ToEvent`'s stamp and `voice`, reached
    through the participant `authorize` hands on, D2), and the stamps in
    `ruleset.go`; `presence.go` is read, not changed.
  - `internal/mcp/door_tools.go`: `getParticipantsDescription`.
  - `cmd/vtt/harness_boot.go` (`mintInvites`), `cmd/vtt/cli_test.go`,
    `internal/gateway/scenario_test.go`, `client/e2e/setup.ts`, under OD-1 A.
  - `client/src/commands.ts` (`promoteParticipant`), `client/src/session.ts`
    (`onPresence`'s doc), and the client tests of M8.
  - `client/e2e/booting.spec.ts`, `speaking.spec.ts`, `whispering.spec.ts`,
    `joining.spec.ts` and `handover.spec.ts` (D20).
  - `internal/gateway/convert_test.go`: `TestEveryClientCommandConverts`'s
    reason for `promote_participant`.
  - `tools/mutation-equivalents.txt`: the `viewpoint.go` entry on
    `watchable`'s sort (M13); `tools/comment-ceilings.txt` (M12).
  - SPEC-017 (check 6).
- **G2. Item 8's "a client test that fails on today's tree"** cannot be
  written under OD-3 A: the window already re-reads `/api/me` on a presence
  frame naming it (VTT-712), so it needs no new code to follow its role.
  Item 8 is held by R18's gateway test and by `joining.spec.ts`'s grant (D20),
  both red today. Item 8's "no player's window perches" (D14) is red today in
  a client test: a player's window sent a set saying it controls no party
  member perches (R29).
- **G3. Item 3's "minted or promoted as a player"** names routes OD-1 A
  and OD-2 A close: the mint is refused (R23) and the promotion is refused
  (R11). What is left of item 3, a participant who controls nothing answered
  as a spectator, is held by R1, R2 and R5, and `roleIn`'s own test holds a
  participant identity holds as a player (an old row) judged by the log.
- **G4. The ticket's "What could not be established"** travels: the first
  is D1, D2 and OD-1; the second OD-2 and OD-1; the third OD-3; the fourth
  OD-4; the fifth M9 and D10.
- **G5. Tests that stay green while their subject changes.** Under the
  derivation a player literal that controls nothing is judged a spectator,
  and a spectator literal that controls is judged a player; a test asserting
  a refusal both roles share stays green for the other role's reason, and the
  rule it was written for loses its kill. No red run finds these; M7 names
  them and Task 3 rewrites them.
- **G6. VTT-289's wire half for a grant becomes unreachable:** a minted id
  is far under the fold's bound (M14), so no grant over the wire names one at
  or over it. The fold's bound stays held by `internal/engine` and
  `client/test/fold-rejections.test.ts`; the revoke halves of the wire tests
  stay (D17).
- **G7. Blueprint §16 is the owner's sentence.** He rewrote the proposal
  (D21).

## Measurements this plan stands on

- **M1. The base is green.** In copies of the tree at `c99715b`: `go test
  -count=1 ./internal/... ./cmd/... ./tools/...` passed whole; `env -u
  CLAUDECODE bun test client/test`, with the worktree's `node_modules`
  linked and `campaigns/`, `rulesets/`, `adventures/`, `scenarios/` copied,
  passed whole.
- **M2. The role derived, and nothing else.** In a copy where a `roleIn`
  answered dm and agent from identity and every other role from the actors,
  read by `Authorize`, `MayWatch`, `defaultViewpoint`, by `handleCommand`
  after `authorize`, and by `handleMe`, `handleParticipants` and
  `abilitiesReadBy`, with identity and promotion as they are, `go test` over
  `internal/gateway`, `internal/harness`, `internal/mcp`, `internal/identity`
  and `cmd/vtt` failed in `internal/gateway` alone, these:
  - a promotion that changes what a seat may watch with nothing appended:
    `seat_internal_test.go`'s `TestAPromotionAsTheSeatOpensSendsNoEntryOfTheShoulder`,
    `TestAPromotionAsTheSeatOpensMovesItOntoTheirPartyMember`,
    `TestAnEntryTakenAfterAPromotionIsJudgedByTheNewRole`,
    `TestARoleReadAfterAdmitStillJudgesTheEntry`; `watch_test.go`'s
    `TestAPromotionMovesASpectatorOffAShoulderThePlayerMayNotWatch`,
    `TestAPromotedSpectatorMovesOntoThePartyMemberTheyHold`,
    `TestAPromotionMovesEveryConnectionOfTheParticipant`;
    `watchable_test.go`'s `TestAPromotionTellsTheSeatTheNewRolesSet`;
    `qa_watchable_test.go`'s `TestQAWatchableAPromotionChangesTheSet`;
    `qa_promotion_rejudge_test.go`'s
    `TestQARejudgePromotedSpectatorOnAnothersPartyMemberMovesToTheirOwn`,
    `TestQARejudgePromotedSpectatorHoldingAPartyMemberWatchesItFromItsFirstEntry`,
    `TestQARejudgeBothConnectionsOfAPromotedSpectatorAreRejudged`;
    `qa_promotion_queued_test.go`'s
    `TestQAQueuedPromotedSpectatorOnAnothersPartyMemberMovesToTheirOwn`,
    `TestQAQueuedPromotedSpectatorMovesToItsOwnFirstPartyMember`;
    `qa_player_perch_test.go`'s
    `TestQAPPPromotingASpectatorToAPlayerWithoutAPartyMemberKeepsThePerch`;
    `server_test.go`'s `TestAPromotionBitesWithoutReconnecting`;
  - a spectator who controls: `qa_controls_none_test.go`'s
    `TestQACNASpectatorWhoControlsAPartyMemberIsToldSo`,
    `TestQACNASpectatorsControlChangeSendsTheSetAgain`,
    `TestQACNMakingAPlayerASpectatorKeepsSayingTheyControlOne`;
    `watchable_abilities_test.go`'s
    `TestASpectatorWhoControlsAPartyMemberIsSentItsAbilityIds`;
    `qa_ruleset_by_role_test.go`'s
    `TestQAReadASpectatorReadsNoAbilityEvenWhereItsIdControlsAnActor`;
    `qa_describe_token_test.go`'s `TestQADTSpectatorIsRefused`;
    `ability_change_authz_test.go`'s
    `TestOnlyTheDMAndTheAgentMayGiveOrTakeAnAbility`; `authz_test.go`'s
    `TestAuthorizeTableAllCommandsAllRoles` (its spectator column, whose
    participant is the fixture's `p-1`, owner of `a1`); `qa_whisper_test.go`'s
    `TestQAWhAuthorizeAdmitsAWhisperFromTheDMTheAgentAndAPlayerAlone` (its
    spectator rows use the player's id; its empty-role row failed too,
    because that copy derived every role but dm and agent, and stopped
    failing in M3 once the derivation was confined to player and spectator,
    D1);
  - a player who controls nothing: `metadata_test.go`'s
    `TestMetadataMeIdentifiesTheCaller`,
    `TestMeSaysWhoYouAreAndNeverWhatYouControl`,
    `TestParticipantsNamesEveryoneAndTheirRole`; `narration_test.go`'s
    `TestALineOutOfCharacterSpeaksAsNoActor`; `qa_door_square_test.go`'s
    `TestQADSAPlayerWhoControlsNothingGetsOneTextAtEverySquare`.
- **M3. The rest of the server.** On top of M2, with the derivation confined
  to player and spectator: a promotion to any role but spectator refused; a
  promotion to spectator appending one revocation per actor through a helper
  `handleRemoval` shares, with no `SetRole` and no `rejudge` signal; the
  grantee step (M5); the announcement (M4); `seat.rejudge`, `judgedAs` and
  `onEntry`'s post-admit step removed. The same packages failed in
  `internal/gateway` alone, every test M2 names, M5's, and these:
  `watch_test.go`'s `TestAPlayerMadeASpectatorKeepsWatchingTheirPartyMember`
  and `TestAPromotedSpectatorKeepsWatchingAPartyMemberTheyHold`;
  `server_test.go`'s `TestAPromotionAppendsNoEvent`,
  `TestDMPromotesASpectatorOverTheWire`, `TestPromotionCannotUNMAKEADMOrAgent`
  (its control step promotes a spectator to player),
  `TestAPromotionIsAnnouncedToThePromotedPersonThemselves` and
  `TestAPromotionIsNotAnnouncedToARevokedWatcher`; `qa_joining_test.go`'s
  `TestQAAPromotionLeavesTheLogHeadWhereItWas`; every test of
  `qa_promotion_opening_test.go`; every test of
  `qa_promotion_revoked_test.go` but
  `TestQAPromoRevokedPromotingARevokedParticipantSendsThemNothing` and
  `TestQAPromoRevokedARevokedWatcherIsToldOfNoArrivalAndNoDeparture`;
  `qa_promotion_queued_test.go`'s
  `TestQAQueuedPromotedSpectatorOnItsOwnPartyMemberKeepsStreaming`; every
  test of `qa_promotion_rejudge_test.go` but
  `TestQARejudgeASpectatorPromotedToSpectatorKeepsItsPerch`;
  `qa_watchable_test.go`'s
  `TestQAWatchableAChangeThatLeavesTheSetAloneSendsNone` (its last step
  demotes a player holding party members, which now changes the set) and
  `TestQAWatchableTheDMAndTheAgentAreSentNoSet` (its step promotes to
  player). `TestQAPPDemotingAPlayerToSpectatorKeepsThePerch` and
  `TestQAQueuedDemotedPlayerKeepsStreamingItsLog` passed.
- **M4. The announcement alone.** In a pristine copy where an accepted grant
  and revocation call `announcePromotion` for the participant they name, and
  `handleRemoveActor` calls it for each controller of the removed actor,
  `internal/gateway`'s tests all passed.
- **M5. The grantee step alone.** In a pristine copy where a grant whose
  grantee `identity.Lookup` answers `ErrInvalidToken` is refused with "There
  is no such participant." after `validateForm`, `internal/gateway` failed
  these and `cmd/vtt`, `internal/harness` and `internal/mcp` passed:
  `TestDMGrantsControlOverTheWire` (grants to `p-second`),
  `TestPlayerOwnershipDenialNoBroadcast` (`dmSeedOtherToken` grants to
  `someone-else`),
  `TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing`,
  `TestQACmdControlOverTheBoundIsRefusedOnTheWireWithTheFoldsText` and
  `TestQACmdControlAtTheBoundIsRecordedWhole`.
- **M6. Identity refusing to mint a player, alone.** In a pristine copy whose
  `CreateInvite` refused `RolePlayer`, `go test` over every package failed in
  `internal/gateway`, `internal/identity` and `cmd/vtt`, and in no other. The
  failing tests are the ones whose fixtures mint a player, bounded by `git
  grep -n 'RolePlayer' -- '*_test.go'` (the shared fixtures among them:
  `newGWFixture`, `newMetaFixture`, `qacnTable.invite`, `qaMGTable.invite`,
  the `qa_writer_hold_test.go` mints), `"--role", "player"` in
  `cmd/vtt/cli_test.go`, and `mintInvites` minting each scenario's
  `"role": "player"` and the soak's players (`TestScenarioLibraryRunsSelfContained`,
  `TestScenarioGoldenStreamsHaveNotDrifted`,
  `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts` among
  them). `client/e2e/setup.ts` mints "Lera" as a player (`task e2e` not
  run).
- **M7. Tests green under the derivation whose subject changed.** In a copy
  of M2's tree whose `roleIn` printed a line whenever identity's role and the
  log's disagreed, `go test -v ./internal/gateway/`, each line attributed to
  the test running:
  - `Authorize`, `MayWatch` and `watchable` handed a player literal no actor
    counts among its controllers, so the spectator's table refuses what the
    player's rule was meant to: `authz_test.go`'s
    `TestAuthorizeNonControllerStillDeniedWhenActorIsShared`,
    `TestAuthorizePlayerControllerlessActorTokenDenied`,
    `TestAuthorizePlayerOtherTokenDenied`,
    `TestAuthorizePlayerRemoveConditionOtherActorDenied`,
    `TestAuthorizePlayerRevokeSelfOnAnActorTheyDoNotControlDenied`,
    `TestAuthorizePlayerUnknownTokenDenied`,
    `TestAuthorizePlayerUseAbilityControllerlessActorDenied`,
    `TestAuthorizePlayerUseAbilityOtherActorDenied`,
    `TestAuthorizePlayerUseAbilityUnknownActorDenied` and
    `TestAuthorizeUnknownCommandDeniedForEveryRole`; `qa_remove_roles_test.go`'s
    `TestQARemoveParticipantOnlyDmAndAgentMayIssueIt` and
    `TestQARemoveParticipantPlayerRefusedEvenForSelf`; `qa_token_texts_test.go`'s
    `TestQATTAPlayersAndASpectatorsDescribeAreRefused`; `viewpoint_test.go`'s
    `TestAPlayerWhoControlsNoPartyMemberMayWatchAnyPartyMemberOrNobody`;
    `seat_internal_test.go`'s
    `TestWatchableNamesEveryPartyMemberForAPlayerWhoControlsNone`.
    `TestAuthorizeEmptyParticipantMatchesNothing` is among them and keeps its
    meaning: the empty-id guard in `controls` is now what keeps `roleIn` from
    answering player, so its removal still turns the test red.
  - Wire tests asserting about a player who controls nothing, Ivo
    (`newGWFixture`'s `otherPlayerToken`) or a minted player never granted:
    `qa_controls_none_test.go`'s
    `TestQACNAPlayerWhoControlsNothingIsToldTheyControlNone`;
    `qa_whisper_test.go`'s `TestQAWhAPlayerWhoControlsNothingCannotWhisper`;
    `watchable_test.go`'s
    `TestAPlayerWhoControlsNoPartyMemberPerchesAndReopensOnIt` and the "a
    player who controls nothing" case of
    `TestTheSetSaysWhetherTheParticipantControlsNoPartyMember`;
    `watchable_abilities_test.go`'s
    `TestTheSetsOfASpectatorAndAPlayerWhoControlNoPartyMemberCarryNoAbilityIds`.
    The other wire tests the instrument named judged a minted player at the
    instant before their grant, which OD-1 A mints as the spectator they
    are.
  - A spectator who controls, beyond M2's and M3's: `viewpoint_test.go`'s
    `TestAnActorWithNoDeclaredKindIsNoShoulderHoweverManyHoldIt`;
    `authz_test.go`'s `TestOnlyTheDMAndTheAgentMayDescribeAToken`;
    `qa_player_perch_test.go`'s `TestQAPPTheSpectatorsPerchIsUnchanged`;
    `qa_watchable_test.go`'s
    `TestQAWatchableASpectatorsOpeningNamesEveryPartyMember`.
- **M8. The console and the perch.** In a copy of the client where a
  spectator's roster row draws no "Make player", "Make spectator" asks
  `d.confirm` with the owner's sentence before it sends, and the perch draws
  no line, `bun test client/test` failed these and nothing else:
  `dm-view.test.ts`'s "a spectator can be promoted, and a player is not
  offered promotion again" and "the roster names each person, their role, and
  what the button will do"; `command-surface.test.ts`'s "every command
  surfaced in a panel has a control a person can reach" (its fixture's roster
  holds one spectator, and the row it finds is `promote-`); `app.test.ts`'s
  "a DM's roster learns about somebody who joins after the console loaded"
  (it finds `promote-p-new`) and "a player who controls no party member is
  told so on the perch, and a spectator is not"; `spectator-view.test.ts`'s
  "the perch says you play no character when told to, and nothing when not";
  `qa-refusal-words.test.ts`'s "the perch says the window plays no character
  only when told to", "the perch tells a player they play none, not a
  spectator", "a page's first set tells nothing, though it controls none" and
  "a set saying the player controls none names none as controlled" (each of
  the last of these assert the line beside what they are about). No test failed for
  the question: no test makes a player who controls an actor a spectator.
- **M9. Commands are serialised per connection, not across connections.**
  `serve`'s read loop answers one command before it reads the next;
  `handleCommand` takes `campaign.State()`, a snapshot, and judges outside
  the campaign's lock; `Campaign.Append` and `AppendBatch` hold `c.mu` for
  the fold and the write alone; `Server` holds no lock but `mapsMu`.
- **M10. The fold.** `engine.Apply`'s `ActorControlRevoked` arm removes only
  the named participant and is a no-op for one not in the set; it refuses an
  unknown actor (`controlTarget`), which refuses a whole batch. Only
  `ActorControlGranted`, `ActorControlRevoked` and `ActorRemoved` change an
  actor's controllers (`ActorAdded` with a controller is refused), and only
  grants, revocations, `remove_actor`, removal and, after this work, a
  promotion append them (`git grep` for those payloads outside test
  files: `convert.go`, `server.go`, `internal/eventgen`'s generator, which
  builds test logs, and `internal/perceive`'s character-log introductions,
  which never reach the world log).
- **M11. The order of a revocation's frames.** The batch's wake is taken by
  the revoked participant's pump, which queues the new set; the command's
  goroutine then resolves `revoked()` (an identity read per present
  participant) and queues the `PresenceChanged`; the window changes its role
  only after the `/api/me` read that frame starts. So the set reaches a
  window before its new role, except when the command's goroutine overtakes
  the pump and the HTTP answer then overtakes a frame already on the socket.
- **M12. The comment ceilings.** `tools/check-comments.py --report` puts
  `server.go`, `seat.go`, `authz.go`, `viewpoint.go`, `metadata.go`,
  `convert.go`, `presence.go`, `identity.go`, `invite.go`, `harness_boot.go`,
  `door_tools.go`, `app.ts`, `session.ts`, `commands.ts`, `view/dm.ts` and
  `view/spectator.ts` each at its ceiling in `tools/comment-ceilings.txt` or
  just under it. A comment line fits only beside added code or in place of a
  removed one; removing comment lines may leave a file under its ceiling by
  more than the band, refused until `--write-ledger` runs, once, last (Task
  8).
- **M13. One adjudication sits on this code.** `tools/mutation-equivalents.txt`
  holds `viewpoint.go`'s CONDITIONALS_BOUNDARY entry on `watchable`'s
  `sort.Slice(set.Actors, ...)` line; its code must stand byte for byte, as
  often as `copies` says. None in `tools/ts-mutation-equivalents.txt` names
  the roster rows, the perch's line or `mayPerch`.
- **M14. A minted id is short.** `CreateInvite` mints an id as the hex of
  random bytes, a fixed form far under the fold's participant-id bound, and
  its `INSERT INTO participants` is the only one outside test files
  (SPEC-009).
- **M15. Rule 9, by command.** See check 1: MapTool's role is a permission
  level kept apart from ownership. Nothing to borrow; its split is the one
  this removes.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened. A test is deleted only
  where what it observes can no longer happen, with its row withdrawn or its
  cell updated in the same commit and the reason in the report; a test whose
  fixture changes asserts what it asserted, or more (M7's rewrites keep the
  rule they were written for reachable).
- **P2. Rule 3.** No field or command leaves the contract:
  `promote_participant` and its `role` stay; comments change, and `task
  generate:contract` regenerates. No contract field is added (OD-3 A).
- **P3. Rule 4 and VTT-039.** No fold change, and no role reaches
  `internal/engine`.
- **P4. SPEC-013's boundaries.** `Authorize` performs no I/O; a validator
  refuses a form, not an issuer; the grantee's identity read is a handler
  step (D7).
- **P5. Rules 8 and 10.** Names, never line numbers. A comment under
  `internal/`, `cmd/` or `client/src`, test files included, is a warning or a
  pointer; a comment the change makes false is corrected or removed in the
  commit that makes it false (D22); M12 binds what may be added.
- **P6. Rule 9,** answered by the ticket and checked (M15).
- **P7. SPEC-020 and SPEC-023.** M13's code stands. The client's stored
  mutation report goes out of date with the first client commit; `task
  check` names that and passes; a full run is the owner's to ask for. A Go
  mutation survivor on this work's code is killed, never adjudicated to pass.
- **P8. The mechanical gates.** A commit that changes `client/src` carries
  `cmd/vtt/webdist` rebuilt by `task build:client`; bun under `env -u
  CLAUDECODE`; `task client:typecheck`; `python3 tools/check-new-prose.py`
  and `python3 tools/check-comments.py main` before each commit; stage and
  commit in separate calls; a client test reads no file outside
  `contract/gen` and `contract/testdata`; `Array.from` over a `NodeList`;
  commands started with `/bin/sh`, never zsh; the semgrep vocabulary rule
  ("hp"). Each task gets Phase 4a's QA and one reviewer before its commit.

## Decisions

1. **D1. The derivation.** `roleIn(p, actors)` in `internal/gateway/authz.go`,
   beside `controls`: for a participant identity holds as a player or a
   spectator, `player` when an actor of `actors` counts `p.ID` among its
   `controller_ids` (`controls`), else `spectator`; every other role as
   identity holds it. A copy of the participant carrying that role is what
   the decisions judge.
   - **Forced by** the ruling (any actor, of any kind); VTT-039 (not in the
     fold); P4 (it reads the actors it is handed); `controls` reading
     `controller_ids` and refusing the empty id (SPEC-013, VTT-148); and M2,
     where deriving every role but dm and agent let an empty role through
     `Authorize`: a role identity would never hand is left as it is, so it
     still fails closed in the table.
2. **D2. Every decision derives from the actors it judges; nothing is
   cached.** `Authorize` derives from `st.Actors`; `MayWatch` and
   `defaultViewpoint` from the actors they are handed, so `watchable`,
   `openSeat`, `resolve`, `onPerch` and `judgedBy` judge the log through
   them; `authorize` derives from the snapshot it takes and hands that
   participant to `handleCommand` for every step after it (the player's
   sight and target gates, `voice`, the batch handlers, `ToEvent`'s stamp);
   `handleMe` and `handleParticipants` derive from `campaign.Actors()`;
   `abilitiesReadBy` asks no role of a participant who is neither the DM nor
   the agent and reads the abilities of the actors they control, which is
   none for a spectator.
   - **Forced by** the ruling ("two stores that can drift"): the folded state
     is already the one cache of the controllers, and a map kept in step at
     each append is a second store with an ordering of its own against the
     pump. Deriving inside `Authorize` and `MayWatch`, not only at their
     callers, is forced by item 10: a unit test handing them a role literal
     is then judged by the log too, so no test can state a spectator who
     controls (G5). `watchesALog` asks dm-or-agent only and stays.
3. **D3. `promote_participant` is the "Make spectator" command (OD-2 A).**
   `authorizePromotionTarget` accepts `spectator` alone; `player` is refused
   with OD-5's words, any other role with its present refusal, corrected to
   name spectator as the one role. `handlePromotion` keeps its lookup and its
   dm-or-agent refusal (VTT-026), builds one `ActorControlRevoked` per actor
   the target controls in the snapshot `authorize` took, appends them as one
   batch when there are any and nothing otherwise, stamps them with the
   issuer, and returns the batch's first sequence. It calls neither `SetRole`
   nor `presenceRegistry.rejudge`; it announces (D8) when it appended.
   - **Forced by** the ruling (one command, as `handleRemoval` builds them),
     rule 3 (the command stays) and OD-2's answer.
4. **D4. One helper builds the revocations.** `controlRevocations(st,
   targetID, issuer)` returns, for each actor whose controllers include the
   target, in actor-id order, an `ActorControlRevoked` naming the target,
   stamped with a fresh event id, the issuer's id and role and one time.
   `handleRemoval` and `handlePromotion` call it; removal behaves as before
   (VTT-559 to VTT-563).
   - **Forced by** the ruling ("as `handleRemoval` builds them") and the
     owner's rule of 2026-09-23 (one source).
5. **D5. `identity.SetRole` is deleted,** with its tests, in the task that
   removes its one caller. VTT-029, VTT-030, VTT-037 and VTT-066 are
   withdrawn there.
   - **Forced by** check 4 (its one caller is `handlePromotion`) and the
     owner's rule of 2026-09-04.
6. **D6. The seat's role re-judge goes.** `seat.judgedAs`, `seat.rejudge`,
   `onEntry`'s post-admit `rejudge` step and `handlePromotion`'s `rejudge`
   signal are removed, and so is `pumpState.revoked`'s `ps.sub.p = now`.
   Kept: the credential re-resolution itself (it closes a revoked
   participant's connection, VTT-032, VTT-352, VTT-376), `presenceRegistry.rejudge`
   and the pump's `rejudge` arm (removal's signal, VTT-558, VTT-566), the
   pump's opening `recheck`, and `admit`'s read of a waiting wake.
   - **Forced by** D2 and D5: a role now changes only when an actor's
     controllers change, which only an append does (M10), and every append
     raises the wake `admit` reads before it sends an entry (VTT-348); and
     no command changes a participant's row but `Revoke`, so a successful
     lookup returns the participant the seat already holds. Code with
     nothing left to judge has no observation, so its mutants would survive
     (P7). VTT-387 and VTT-388 are withdrawn.
7. **D7. The grantee step.** In `handleCommand`, after `validateForm` and
   before the dispatch, a `grant_actor_control` asks `s.ids.Lookup` of its
   participant id: `ErrInvalidToken` refuses with R7's text, appending
   nothing; any other error refuses with its own text, as
   `handlePromotion`'s and `handleRemoval`'s lookups do; otherwise the grant
   goes on to `ToEvent` and `campaign.Append` as today. A grant to the DM or
   the agent stays accepted, save D25's refusal of a kind other than party
   member of an actor a player controls.
   - **Forced by** P4 (not in `Authorize`, not in a validator, not in the
     fold); after `authorize`, so only the DM and the agent reach an identity
     read and a player learns nothing of which ids exist; after
     `validateForm`, so a grant wrong in its kind is told about its kind
     (VTT-151 and `scenarios/denials.json` keep their texts); on `ToEvent`'s
     path, so SPEC-013's "every other command reaches `ToEvent`" and
     `TestEveryClientCommandConverts` stay true.
8. **D8. A control change re-announces whom it names (OD-3 A).**
   `announcePromotion` becomes `reannounce` and is called after an accepted
   grant or revocation for the participant it names, after an accepted
   `remove_actor` for each controller the snapshot's actor lists, and after a
   promotion that appended, for its target; removal announces nothing new
   (its target's connections close). Each call is made whether or not the
   participant's role changed.
   - **Forced by** item 8 and the window's existing path: a presence frame
     naming the window re-reads `/api/me` and then the ruleset (VTT-712), and
     any presence frame re-reads the DM's roster (`refreshSharing`), so one
     frame moves both windows. Unconditional because a before-and-after
     comparison across a racing append (M9) can miss a change and leave a
     window in the wrong role until a reload; the cost of a spare frame is
     one `/api/me` read. M4: nothing breaks. The frame is sent from the
     command's goroutine, as a promotion's is today, so a wedged connection
     delays the command's answer by `presenceSendBudget` (SPEC-011).
9. **D9. The toast's race (M11) gets no code, no test and no debt.** When the
   window reads its new role before the set arrives, VTT-606 withholds "You
   no longer play ...". Almost theoretical (M11) and small (the perch still
   follows); the owner's rule of 2026-10-21. One line in the report.
10. **D10. Interleavings (M9) get no code, no test and no debt.** Every
    answer is a function of one snapshot, and in every folded state a
    participant is a player exactly while an actor lists them, so no
    interleaving produces a state the role's rule forbids (D25's rule has the
    one named last below); it can only make a
    decision read an older snapshot, as every player rule already does
    (SPEC-013). Named, for Phase 4a's "known":
    - a grant judged as its grantee is removed can land after the removal's
      batch, leaving control with a revoked id: shown by id in "Who controls
      what", issuing nothing, undone by Revoke;
    - "Make spectator" racing a grant leaves the participant a player with
      the new actor; racing a `remove_actor`, its batch names an unknown
      actor and the fold refuses it whole, leaving them a player as they
      were;
    - a command judged just before the participant's last revocation lands
      is accepted as a player's;
    - the DM's grant of a kind other than party member of an actor, judged
      before a grant of that actor to a player lands, appends after it and
      leaves the player holding an actor that is not a party member, which
      D25's rule forbids: it takes two of the DM's or the agent's connections
      granting one actor at once; the player then fails closed as D25 says,
      and a further grant mends it;
    - a grant of an actor landing between a `remove_actor`'s snapshot and its
      batch leaves that grantee unannounced (D8 reads the controllers from the
      snapshot, the only place they still are), so their window keeps its
      role until a reload: two of the DM's or the agent's connections at once,
      found by Task 1's review.
11. **D11. The console (rule 4, rule 5).** A spectator's roster row offers
    Boot alone. A player's row offers "Make spectator", which asks
    `d.confirm` with R14's sentence, built from `d.st`: the actors whose
    `controllerIds` include the participant, each shown by its name or by its
    id when it has none, sorted by what it is shown by, ties broken on id
    (the owner's answer of 2026-11-21), joined as OD-5 answers, the
    participant by the roster's name, as Boot's question does. A no sends nothing; a yes
    sends `promoteParticipant(participantId)` and reads the roster again
    once answered. A roster row reading `player` whose participant `d.st`
    lists among no actor's controllers (a roster a frame behind the log) gets
    no code: theoretical, and a sentence naming nobody is its whole
    consequence.
    - **Forced by** the ruling's words and the owner's order; the id for an
      unnamed actor because "Who controls what"
      shows the DM an unnamed actor by its id; the re-read after the answer
      by Boot's precedent (VTT-571), the result being what a re-read must
      follow.
12. **D12. `promoteParticipant(participantId)`** names `spectator`, the one
    role the server accepts; its doc and `commands.test.ts`'s builder test
    follow.
    - **Forced by** D3: a parameter with one value is a choice nobody can
      make.
13. **D13. The window needs no new code to follow its role.** `mayPerch`, the
    presence handler and `rereadIfStale` stay; the comments that say a
    promotion re-announces (`onPresence`'s block in `app.ts`,
    `Session.onPresence`'s doc, `DMDeps.roster`'s and `DMDeps.send`'s docs,
    the "Who may do what" block in `dm.ts`) are corrected to the control
    change.
    - **Forced by** D8 and P5.
14. **D14. No player perches** (the owner's answer to OD-4, and D25). A
    window perches exactly while the role the page last read is
    `spectator`: `mayPerch` loses its player half, and
    `PerchControl.playsNone`, `app.ts`'s `playsNone: isPlayer`, the line
    "You play no character; the DM can give you one." and VTT-607 go. A
    player's window always draws the player panel.
15. **D15. The tool texts and the proto comments** say R20 to R22 in their
    own words: `PromoteParticipant`'s comment (no "NO EVENT", no "ToEvent
    allowlist ... for that reason"), `GrantActorControl`'s, and
    `PresenceChanged`'s ("or ... being PROMOTED" becomes a control change);
    `getParticipantsDescription` in `internal/mcp/door_tools.go` says a player
    is a participant who controls an actor and that `promote_participant`
    makes one a spectator. `task generate:contract` regenerates
    `contract/gen`, `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`;
    `contract/testdata/expected_tools.json` follows.
    - **Forced by** item 9 and SPEC-007 (a proto comment warns whoever edits
      the line).
16. **D16. Identity (OD-1 A).** `CreateInvite` mints dm, agent and spectator
    and refuses player with OD-5's words; `ParseRole` still reads dm, agent,
    player and spectator, so a stored `player` in an old campaign file is
    judged by the log like any spectator (no migration); `Participant.Role`'s doc says the gateway
    answers player and spectator from the log (SPEC-009); `vtt invite`'s
    `--role` help names dm, agent and spectator. `mintInvites` mints a
    scenario's `player` as a spectator, with a pointer to SPEC-009: a
    scenario's role names the seat `characterSeats` plays and its grant step
    makes the participant a player, so the scenario files keep their words.
    `client/e2e/setup.ts` mints "Lera" as a spectator. Every test mint of
    M6's search is rewritten to a spectator.
    - **Forced by** OD-1's answer, the owner's rule of 2026-09-04, and M6
      (the search bounds the rewrite; no assertion changes, because D2
      already judges a granted spectator a player).
17. **D17. The participant-id bound's wire tests keep their revoke halves.**
    `TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing` and
    `TestQACmdControlOverTheBoundIsRefusedOnTheWireWithTheFoldsText`: their
    grant cases expect R7's text and nothing appended, their revoke cases the
    fold's text as today; `TestQACmdControlAtTheBoundIsRecordedWhole` keeps
    its revoke half and loses its grant half (M14, G6).
    - **Forced by** M5, M14 and P1.
18. **D18. M7's tests are rewritten to say what they meant (G5).** A player
    literal handed to `Authorize`, `MayWatch` or `watchable` controls an
    actor of its own in the state, a party member (D25); a spectator literal
    or fixture controls nothing; a wire test about a player who controls
    nothing, or none of whose actors is a party member, loses that case, the
    state being unreachable (D25), or becomes one about a spectator where its
    observation survives (the "a player who controls nothing"
    case of `TestTheSetSaysWhetherTheParticipantControlsNoPartyMember`,
    beside "a spectator who controls nothing"). The instrument (M7) is run
    again before Task 3's commit: no test may judge a player literal who
    controls nothing but `roleIn`'s own test and
    `TestAuthorizeEmptyParticipantMatchesNothing`.
    - **Forced by** item 10, P1 and the owner's ruling on the role-decides
      tests: each still shows the cell or answer it was written for.
19. **D19. The register.** The rows under "Requirements" at Task 0; the
    withdrawals in the tasks named there; every standing row's cell follows
    its tests in the task that moves them.
20. **D20. The real wire.**
    - `joining.spec.ts`: beats 4 and 5 become one: the DM grants Robin a
      character from "Who controls what", the roster's role cell for Robin
      then reads `player`, and the guest's window draws the player panel on
      the connection it joined on (items 1 and 8, R2, R18).
    - `booting.spec.ts`, `speaking.spec.ts` and `whispering.spec.ts` drop
      their `promoteParticipant` steps; the grants they already send make the
      players.
    - `handover.spec.ts`: "the DM takes a player's only character back ..."
      keeps its toasts and its perch, loses the line (D14), and takes the
      creature back in D25's order; a new
      test there, "the DM makes a player a spectator, and they perch": Lera
      holds Ash and Bram, the DM's "Make spectator" is confirmed through
      Playwright's dialog handler, the dialog's message is R14's sentence,
      "Who controls what" names no holder for either, Lera's window perches
      and the roster's role cell reads `spectator` (items 2, 6).
    - **Forced by** items 6, 8 and 11; the e2e is where a window following
      its role without a reload is observable on the real wire (G2).
21. **D21. Blueprint §16** (G7): "A spectator follows a party member of their
    choosing in the same way." replaces "A spectator, and a player who
    controls no character, follows a party member of their choosing in the
    same way."
    - **Forced by** the owner's answer of 2026-11-20, "Igen. Har du kontroll
      över en varelse eller karaktär så är du en spelare", and D25.
22. **D22. Comments and reasons follow the code** (P5), in the commit that
    makes them false: `handleCommand`'s "Answer these three without ToEvent:
    they append nothing" over the promotion; `handlePromotion`'s comments;
    `onEntry`'s doc (VTT-387); `pumpState.revoked`'s doc ("judging by their
    current role"); `seat.recheck`'s and the removed `rejudge`'s; `onWake`'s
    "after an append or a promotion"; `authorizePromotionTarget`'s refusal
    text; `identity.go`'s `SetRole`, `Participant` and `CreateInvite` docs;
    `invite.go`'s flag help; `handleRemoval`'s order comment where the helper
    moves it; `convert_test.go`'s reason; D13's client comments; the test
    comments that say a promotion appends nothing or re-judges a seat. Each
    new line is a warning or a pointer, by requirement id where a ceiling
    binds (M12).
23. **D23. Every client commit carries its bundle** (P8): Task 4.
24. **D24. The specifications and the blueprint last,** from the report
    (Task 9).

## Open decisions for the owner

- **OD-1. What identity's role column holds for a participant who is neither
  the DM nor the agent** (and `CreateInvite`, `ParseRole`, `vtt invite --role
  player`).
  - **A (recommended).** One value, `spectator`, the role every participant
    has before a grant and the one a join already mints. `CreateInvite`
    refuses `player` (OD-5's words); `ParseRole` still reads dm, agent,
    player and spectator, so an old file's `player` row is judged by the log;
    `vtt invite --role` takes
    dm, agent or spectator. Nothing in the store then says what the log
    contradicts, and a gateway reader that forgot to derive would fail closed
    (a player read as a spectator) rather than open. Cost: every test mint
    of a player becomes a spectator (M6's search, mechanical, in a task of
    its own), the harness mints a scenario's player as a spectator (D16), and
    the e2e mints Lera so.
  - **B.** Identity keeps minting and storing whichever of dm, agent, player
    and spectator it is given, and nothing reads player or spectator from it. No mint changes; the
    column then says `player` of participants the log holds as spectators,
    and `spectator` of players, which is the store that disagrees the
    ruling turned down, only unread.
- **OD-2. What `promote_participant` becomes** (rule 3 keeps it).
  - **A (recommended).** It is "Make spectator": to `spectator`, it revokes
    every actor the target controls, one `ActorControlRevoked` each, as one
    batch (D3, D4), and appends nothing for one who controls none; to any
    other role it is refused. One command for the console's one button, no
    contract change.
  - **B.** A new additive command does "Make spectator", and
    `promote_participant` is refused for every role. A second command, its
    tool, its `commandRoles` row, its client builder and its command-surface
    row, beside one that can only refuse.
- **OD-3. How a window learns its role changed.**
  - **A (recommended).** The gateway re-announces, as a promotion does
    today, every participant a grant, a revocation, a `remove_actor` or an
    appending promotion names (D8). The window's existing presence path
    re-reads `/api/me` and the ruleset, and the DM's console re-reads the
    roster by the same frame. No contract change, no client logic change
    (G2).
  - **B.** An additive field on `WatchableChanged` carries the derived role,
    ordered with the set by the pump, and the window takes its role from it.
    The DM's console is sent no set, so its roster still needs A's frame or a
    second derivation of its own; the window then reads its role from two
    sources.
- **OD-4. The perch's line, "You play no character; the DM can give you
  one."** Once a player is a participant who controls an actor, a perching
  player is one whose actors are none of them party members (a creature
  handed over, or the middle of the tool's "grant it again as non-party,
  then revoke" step).
  - **A (recommended).** The line goes: a player who controls only actors
    that are not party members perches as a spectator does, with no line.
    The state the line was written for no longer exists, and to such a
    player it would say "no character" of the creature they hold.
    `PerchControl.playsNone`, `app.ts`'s `playsNone: isPlayer` and VTT-607
    go; the VTT-605 and VTT-606 tests lose their assertion of it.
  - **B.** The line stays for that player in new words, for instance "None
    of your characters is in the party; the DM can give you one."
  - **C.** The line stays as it is.
- **OD-5. The words.**
  - **A (recommended).** A promotion to player:
    `A participant becomes a player when given a character.` (in
    `authorizePromotionTarget`, before any lookup, so it names nobody, as
    `There is no such participant.` names no id). `vtt invite --role
    player`: `identity: a participant becomes a player when given a
    character; invite them as a spectator`.
  - **B.** A promotion to player names the target:
    `Robin becomes a player when given a character.`, decided in
    `handlePromotion` after the lookup, so an unknown id is told
    `There is no such participant.` first.
  - **And the joiner of "Make spectator"'s list** past two actors:
    **A (recommended)** ` and ` between each, as VTT-605's toast joins the
    names a player no longer plays ("Gobo and Ash and a3"); **B** commas
    and a last ` and ` ("Gobo, Ash and a3"). Either reads the owner's own
    sentence for two.
- **OD-6. One ticket, or two.**
  - **A (recommended).** One. A split after the server tasks would leave a
    console whose "Make player" the server refuses and whose "Make spectator"
    asks nothing while it takes characters; the tasks below keep each commit
    green and say what holds between them.
  - **B.** Two: the server, identity and tool texts (Tasks 1 to 3, 5, 6);
    then the console, the perch and the real wire (Tasks 4, 7).

## The owner's answers, 2026-11-20 and 2026-11-21

- **OD-1:** A. **OD-2:** A. **OD-3:** A. **OD-5:** A, both halves. **OD-6:**
  A, one ticket.
- **OD-4:** none of A, B or C: "En spelare med en varelse under dess
  kontroll är en spelare". Offered D21's proposed sentence, he answered
  "Igen. Har du kontroll över en varelse eller karaktär så är du en
  spelare", and, asked on 2026-11-21 what a player who holds only a creature
  that is not a party member watches through, he chose that the creature is a party
  member: D25. So no player perches (D14), and §16 loses the player (D21).

The decisions the answers add, continuing the list under "Decisions":

25. **D25. Every actor a player controls is a party member.** In the grantee
    step (D7), after the lookup: a grant whose grantee is neither the DM nor
    the agent and whose kind is not `ACTOR_KIND_PARTY_MEMBER` is refused
    (R25, R27's words); a grant stating a kind other than party member of an
    actor whose controllers include a participant identity holds as neither
    the DM nor the agent is refused (R26), whoever it names. Both append
    nothing. The texts are disjoint by the grantee: an id identity does not
    hold, or holds revoked, is told R7's text whatever its kind; a grantee
    who is neither the DM nor the agent, R27's; the DM or the agent, R28's.
    `roleIn` counts any actor, by the ruling's "of any kind"; with D25 no
    reachable state tells that from counting party members alone, so that
    half is held by reading (Phase 4b), not by a test. The DM takes a creature back by revoking the player
    first and then granting it to themselves as non-party, or by
    `remove_actor`; the tool texts say so (D15, Task 6). With it a player
    always controls a party member, so `MayWatch` judges every player as
    watching their own (`ownOnly` for every player, no `controlsAPartyMember`
    branch), `defaultViewpoint` answers a player's first party member, and
    `controls_no_party_member` is false for every player and true for every
    spectator (it stays in the contract, rule 3). A player left without a
    party member by D10's interleavings or an old campaign file watches
    nobody, which fails closed: no code, no test, no debt (the owner's rule
    of 2026-10-21).
    - **Forced by** the owner's answer, the ruling's "of any kind", and
      SPEC-021 (only a party member keeps a character log to watch through).
26. **D26. The words of D25's refusals** (proposed by me, chosen by the
    owner on 2026-11-21): R27 `Robin can only be given a party member.`; R28 `Lera
    controls Ash: revoke it from them before you make it a non-party actor.`,
    those controllers who are neither the DM nor the agent, by name, in the
    order the actor's `controller_ids` holds them, joined as OD-5 answers (`Lera and Ivo control Ash: ...`), an actor by its name or by
    its id when it has none.
    - **Forced by** the 2026-11-02 ruling (a name instead of an id) and
      OD-5's joiner.

**Amendments to the tasks.**
- **Task 2** carries D25 and D26 as well (R25 to R28): tests first, seen
  red: a DM's and an agent's non-party grant to a spectator and to a player
  refused with R27's text, appending nothing; a party-member grant to a
  spectator appended; the DM's non-party grant to themselves of an actor
  Lera controls refused with R28's text, and of one no participant but the
  DM controls appended; a party-member grant to the player, the DM and the
  agent appended. Named by the review of Task 0, and re-searched before the
  code: `scenarios/door-watch.json` and `scenarios/shared-control.json`
  with their goldens, `creature_test.go`, `qa_creature_test.go`
  (`TestQACreatureANonPartyGrantEndsTheRun` among them), `qa_move_gate_test.go`'s
  `TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken` and
  `qa_ruleset_by_role_test.go`'s `TestQAReadANonPartyActorAddedAsSuchAndGrantedCounts`:
  each is rewritten so the player's actor is a party member where its
  subject survives, or deleted with its row's cell where its subject was a
  player holding a non-party actor, the reason in the report. Before the code, search every test, scenario, soak
  plan and e2e spec that grants `ACTOR_KIND_NON_PARTY` to a participant who
  is neither the DM nor the agent, or of an actor one controls, and rewrite
  each to the take-back order (revoke, then grant to the DM as non-party),
  keeping what it asserts. Deliberate breaks: drop the grantee-kind test (a
  non-party grant to a player appended, R25 red); drop the controllers test
  (R26 red); name the actor by id when it has a name (R28 red).
- **Task 3**'s `MayWatch` and `defaultViewpoint` follow D25. The rows
  VTT-340, VTT-370, VTT-452, VTT-453 and VTT-456 are withdrawn there,
  succeeded by R33, by D25 (R25, R26), by D25, by R34 and by R35.
- **Task 4** follows D14: the rows VTT-457, VTT-458, VTT-459, VTT-460,
  VTT-461, VTT-607 and VTT-696 are withdrawn there, succeeded by R30, R29,
  R29, R31, R29, nothing (the line goes) and R32.
- **Task 6**'s tool texts say the take-back order (D25), and the
  `grant_actor_control` kind description stops telling the DM to grant a
  player's creature back to them as non-party; VTT-389, which holds that
  text, is withdrawn there, succeeded by R20 and R21. The `set_viewpoint`
  tool text ("a spectator, and a player who controls no party member, may
  watch any PARTY MEMBER") and `commands.proto`'s `WatchableChanged` field
  comment ("a client draws a player the perch by true") and
  `PresenceChanged`'s "roles live in identity" follow too (D15).
- **Task 3** also changes Task 1's tests: `qa_reannounce_test.go`'s
  promotions to `player` (refused by R11) become "Make spectator" or go, and
  VTT-906's cell gains the appending promotion's test (its promotion clause
  has none until then). **Task 5** rewrites the player mints in
  `reannounce_test.go`, `qa_reannounce_test.go` and the qaPR harness they
  share.
- **Task 9**: SPEC-009 and SPEC-011 name `announcePromotion`, which is now
  `reannounce`; SPEC-011 says a `remove_actor` re-announces once per
  controller, each fan-out bounded by `presenceSendBudget`; SPEC-013's
  command path gains the re-announcement after the append; SPEC-017 gains
  `handleRemoveActor`'s announcements.
- **Every task**: a pointer to a row the task withdraws (`git grep` of the
  id across `internal/`, `cmd/`, `client/`, `contract/vtt` and `tools/`) is
  re-pointed to its successor in the commit that withdraws it (D22); the
  chain gate does not see a pointer to a withdrawn row.
- **Task 9**: SPEC-015 and SPEC-022 lose the player who controls no party
  member, and SPEC-021 is checked for it.

**Requirement candidates added by the answers:**

| R | From | Rule (as drafted; the register's row binds) | Red under |
|---|---|---|---|
| R25 | D25; item 12 | A `grant_actor_control` naming a participant who is neither the DM nor the agent, stating a kind other than party member, is refused and appends nothing. | a non-party grant to a spectator or a player appended (today) |
| R26 | D25; item 12 | A `grant_actor_control` stating non-party of an actor whose controllers include a participant who is neither the DM nor the agent is refused and appends nothing. | the DM's non-party grant to themselves of a player's actor appended (today) |
| R27 | D26 | R25's refusal reads `<name> can only be given a party member.`, naming the grantee by display name. | the id echoed; other words |
| R28 | D26 | R26's refusal reads `<names> control(s) <actor>: revoke it from them before you make it a non-party actor.`, the controllers by display name joined as OD-5 answers, the actor by its name or by its id when it has none. | an id where a name is held; another joiner |
| R29 | D14 | A browser window perches exactly while the role the page last read is `spectator`, whatever the last `WatchableChanged` says. | a player's window with a set saying it controls no party member perching (today) |
| R30 | VTT-457 | A browser window whose participant's role the client last read as `player` offers a switch button for exactly the party members the last `WatchableChanged` names, and none before the client is sent one, except while it shows the reason it cannot fold its log. | a button missing; a button before any set |
| R31 | VTT-460 | A board click in a browser window whose participant's role the client last read as `player`, on a cell holding a token of a party member the last `WatchableChanged` names other than the one they watch, sends `set_viewpoint` naming one of those party members. | a move sent instead |
| R32 | VTT-696 | A player's window reads `/api/ruleset` again when a `WatchableChanged` changes the set of actors it may watch. | no read after the set changes |
| R33 | VTT-340 | Whenever a player's connection watches no actor the player may watch, it watches their first party member by id. | it watches nobody; another party member |
| R34 | VTT-453 | At the command's snapshot, a player's move is judged by the eyes of the party member their connection shows while they control it, otherwise by their first party member by id. | another party member's eyes |
| R35 | VTT-456 | An event that ends a participant's control of their last actor does not move a connection of theirs off the party member it watches, when that one is still a party member after the event. | the connection moved to nobody |

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill, as the answers
leave them; the register's words are the ones that bind. Every new row reads
`**OPEN — no test yet**` until its task cites its tests.

| R | From | Rule (as drafted; the register's row binds) | Red under |
|---|---|---|---|
| R1 | rule 1; items 1 to 4 | `GET /api/me` answers `player` for a participant who is neither the DM nor the agent while an actor counts them among its controllers, and `spectator` otherwise. | a grant leaves it `spectator` (today); the last revocation, by the DM, the agent, the player or `remove_actor`, leaves it `player`; a non-party actor not counted; the DM or the agent answered `player` |
| R2 | rule 1; items 3, 4 | `GET /api/participants` answers each participant who is neither the DM nor the agent as `player` while an actor counts them among its controllers, and as `spectator` otherwise. | identity's column listed (today); one row derived and another not |
| R3 | rule 1; items 1, 2 | A command of a participant who is neither the DM nor the agent is judged by the player's cells and rules while an actor in the snapshot it is judged against counts them among its controllers, and by the spectator's otherwise. | a granted spectator's move refused by the table (today); an out-of-character line accepted after their last revocation; `Authorize` handed a spectator literal who controls refusing a player's command; a non-party actor not counted |
| R4 | rule 1 | Every event a command of such a participant appends carries `player` as its `actor_role`. | a granted spectator's move or ability use stamped `spectator` |
| R5 | rule 1; items 3, 4 | What a connection of such a participant may watch, watches when it watches nothing it may, and is told it may watch, is judged as a player's while an actor counts them among its controllers, and as a spectator's otherwise. | a spectator perched on another participant's party member, granted one of their own, stays where it is, and its set still names every party member |
| R6 | rule 2; item 5 | A `grant_actor_control` naming a participant id identity does not hold, or holds revoked, is refused and appends nothing. | an id never minted appended (today); a removed participant's id appended |
| R7 | words | That refusal reads `There is no such participant.`, for an id never minted and a revoked one alike. | the id echoed; identity's error text; two texts |
| R8 | rule 3; item 6 | A `promote_participant` to `spectator` appends one `ActorControlRevoked` naming the participant for each actor whose controllers include them, in actor-id order, as one batch, and no other event. | nothing appended (today); a non-party actor left; another order; two appends; an event for one who controls nothing |
| R9 | rule 3 | Every event a promotion appends carries the participant id and the role of whoever issued it. | the target stamped; an agent's promotion stamped as the DM's |
| R10 | rule 3 | A promotion's result carries the first sequence of the batch it appended, and none when it appended nothing. | no sequence after a batch; the last sequence; a sequence with nothing appended |
| R11 | OD-2; items 1, 3 | A `promote_participant` naming any role but `spectator` is refused and appends nothing. | a promotion to player accepted (today); one to dm accepted |
| R12 | OD-5 | A `promote_participant` naming `player` is refused with OD-5's text. | other words; a `gateway:` prefix; an id |
| R13 | rule 4; item 6 | "Make spectator" on a player's roster row asks the DM first, in a message holding the player's display name and the name of each actor the console's state lists them among the controllers of, and sends nothing when the DM declines. | no question (today); a send after a no; an actor missing; another participant's actor named |
| R14 | rule 4 | That message reads `Make <name> a spectator? They no longer control <actors>.`, the actors in actor-id order, each by its name or by its id when it has none, joined as OD-5 answers. | another order; an unnamed actor blank; another joiner; other words |
| R15 | item 6 | A confirmed "Make spectator" sends one `promote_participant` naming that player and `spectator`. | nothing sent; two sends; `player` named |
| R16 | item 6 | A confirmed "Make spectator" asks for the roster to be read again when its promotion is answered, and not when it is sent. | read before the answer; never read |
| R17 | rule 5; item 7 | No row of the console's "Who may do what" offers a promotion to player. | "Make player" on a spectator's row (today); any `promote-` control |
| R18 | item 8; OD-3 | After an accepted `grant_actor_control` or `revoke_actor_control`, an accepted `remove_actor`, and an accepted `promote_participant` that appended, every connection whose participant is not revoked is sent a `PresenceChanged` naming each participant the command named as a controller (the grantee, the one revoked, each controller of the removed actor, the promoted) while that participant is connected, their own connections included. | no frame after a grant (today); the grantee's own connection skipped; a revoked watcher sent it; a removed actor's controller not named |
| R19 | OD-3 | A participant so named who has no connection is announced to nobody. | a `CONNECTED` frame for someone absent |
| R20 | item 9 | The `grant_actor_control` tool says a grant makes a spectator a player, and that a grant naming a participant the table does not hold, or has removed, is refused. | either sentence missing |
| R21 | item 9 | The `revoke_actor_control` tool says a player whose last actor is revoked is a spectator. | the sentence missing |
| R22 | item 9 | The `promote_participant` tool says it makes a player a spectator by revoking their control of every actor they control, one revocation per actor, that `role` accepts only `spectator`, and that a participant becomes a player when granted an actor. | "writes no event" left; "promote them to player" left |
| R23 | OD-1 A | `identity.CreateInvite` mints dm, agent and spectator, and refuses every other role, `player` included, minting nothing. | a player minted; a row inserted before the refusal |
| R24 | VTT-066 | `ParseRole` accepts exactly dm, agent, player and spectator, and refuses any other. | a fifth role accepted; one of the four refused |

**Allocated at Task 0, 2026-11-21:** R1 = VTT-889, R2 = VTT-890, R3 = VTT-891, R4 = VTT-892, R5 = VTT-893, R6 = VTT-894, R7 = VTT-895, R8 = VTT-896, R9 = VTT-897, R10 = VTT-898, R11 = VTT-899, R12 = VTT-900, R13 = VTT-901, R14 = VTT-902, R15 = VTT-903, R16 = VTT-904, R17 = VTT-905, R18 = VTT-906, R19 = VTT-907, R20 = VTT-908, R21 = VTT-909, R22 = VTT-910, R23 = VTT-911, R24 = VTT-912, R25 = VTT-913, R26 = VTT-914, R27 = VTT-915, R28 = VTT-916, R29 = VTT-917, R30 = VTT-918, R31 = VTT-919, R32 = VTT-920, R33 = VTT-921, R34 = VTT-922, R35 = VTT-923. Added in Task 1, from Phase 4a: VTT-924, a refused grant, revocation or `remove_actor` sends no `PresenceChanged`. Added in Task 2: VTT-925, the form check of a grant, succeeding VTT-151, whose "a grant stating any kind ... is accepted" D25 made false; SPEC-013's `## Requirements` list follows in Task 9. Withdrawn in Task 2 on its review: VTT-914 and VTT-916, whose words counted a controller identity holds revoked, succeeded by VTT-926 (R26) and VTT-927 (R28); added VTT-928, D7's order (identity judged only after `Authorize` and the form check). Moved to Task 3 on that review: the tests that build a player holding a non-party actor by appending events directly, which stay green past the wire — `qa_move_gate_test.go`'s non-party and perch tests (`TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken` among them), `qa_whisper_test.go`'s `TestQAWhADMOrAgentWhisperNamingNoPartyMemberIsRefusedAndAppendsNothing` and `TestQAWhAPlayersWhisperFromAnythingButTheirOwnPartyMemberIsRefusedWithOneText`, `qa_ruleset_by_role_test.go`'s `TestQAReadANonPartyActorAddedAsSuchAndGrantedCounts`, `TestQAReadAPlayerReadsTheUnionOverTheActorsTheyControlInIdOrder` and `TestQAReadAGrantAndARevocationReachTheNextReadOnTheSameConnection`; Task 3 rewrites each so the player's actor is a party member, or deletes it where its subject is unreachable (D18, D25). Added in Task 4 on its review: VTT-930, the console reads the roster again when its grant, revocation or removal of an actor is answered (an offline participant's role moves with no presence frame). Recorded in Task 3: VTT-067 and VTT-068 withdrawn too (each held only by a `SetRole` test), VTT-067 succeeded by VTT-929 (`identity.Revoke` reports a participant that does not exist).

**Candidates the sort may refuse:**
- **A row that `/api/ruleset` follows the derived role.** Refused: VTT-691
  and VTT-692 as worded hold it once `abilitiesReadBy` asks no role of a
  participant who is neither the DM nor the agent (D2); Task 3's test that a
  granted spectator reads its actor's abilities cites VTT-691.
- **A row that a granted spectator's connection moves onto their party
  member.** Refused: R33 (VTT-921) as worded, once R5 makes them a player; the test
  cites both.
- **A row that a promotion of a participant who controls nothing appends
  nothing.** Refused: R8's observation with no actor.
- **A row for the window following its role.** Refused: VTT-712 holds the
  window's half, R18 the server's (G2).
- **A row that the perch draws no line** (D14). Refused: an absence of
  deleted code.
- **The races and the toast's order.** Refused: D9, D10.
- **A row that "Make spectator" on a row whose participant controls nothing
  in the console's state asks nothing.** Refused: D11, theoretical.

**Withdrawn:**
- **In Task 3:** VTT-025 ("A promotion names spectator alone. Succeeded by
  R11."); VTT-029 ("No promotion writes to identity: it revokes control in
  the log (R8), and `identity.SetRole` is gone."); VTT-030 ("No promotion
  writes to identity (R8), and nothing sets revoked back (SPEC-009)."); VTT-031
  ("A role follows control at the next command. Succeeded by R3."); VTT-036
  ("A promotion to spectator appends its revocations. Succeeded by R8 and
  R11."); VTT-037 ("No promotion writes a role; its revocations name the
  promoted participant alone (R8)."); VTT-047 and VTT-309 ("Succeeded by
  R18."); VTT-310 ("Succeeded by R19."); VTT-066 ("`SetRole` is gone.
  Succeeded by R24, and by R23 under OD-1 A."); VTT-387 ("A role changes only
  with control, at an append, whose wake is read before any entry. Succeeded
  by VTT-348."); VTT-388 ("Succeeded by VTT-923 and VTT-921."); and, by
  D25, VTT-340 (R33), VTT-370 ("No player controls no party member (D25)."),
  VTT-452 ("No player controls no party member (D25)."), VTT-453 (R34) and
  VTT-456 (R35).
- **In Task 4 (D14):** VTT-457 (R30), VTT-458, VTT-459 and VTT-461 (R29),
  VTT-460 (R31), VTT-607 ("No perch draws a line of its own.") and VTT-696
  (R32).
- **In Task 6:** VTT-389 (R20, R21).

**Rows checked and left standing,** their cells following the tests that
move: VTT-026, VTT-027, VTT-028, VTT-032, VTT-112, VTT-113, VTT-139, VTT-140,
VTT-142, VTT-147, VTT-148, VTT-161, VTT-162, VTT-181, VTT-289,
VTT-344, VTT-348, VTT-352, VTT-368, VTT-399, VTT-400, VTT-401,
VTT-450, VTT-454, VTT-455, VTT-463, VTT-495,
VTT-499, VTT-503, VTT-543, VTT-544, VTT-559 to VTT-571, VTT-590, VTT-605,
VTT-606, VTT-609, VTT-691, VTT-692, VTT-712, VTT-716, VTT-731, VTT-752.

## The existing tests that change

By M2 to M8, re-measured before each commit.

- **Task 1:** none (M4).
- **Task 2:** M5's tests (D17); `TestDMGrantsControlOverTheWire` grants to a
  second minted participant and asserts their id second;
  `dmSeedOtherToken` grants to a minted participant.
- **Task 3:**
  - Deleted, their premise a role that changes with nothing appended, their
    observations held where named: `seat_internal_test.go`'s
    `TestAPromotionAsTheSeatOpensSendsNoEntryOfTheShoulder`,
    `TestAnEntryTakenAfterAPromotionIsJudgedByTheNewRole`,
    `TestARoleReadAfterAdmitStillJudgesTheEntry` (VTT-348);
    `qa_promotion_opening_test.go` and `qa_promotion_queued_test.go`'s
    promoted-spectator tests (R33's grant tests, VTT-340's until Task 3);
    `qa_promotion_rejudge_test.go`'s tests but
    `TestQARejudgeASpectatorPromotedToSpectatorKeepsItsPerch`;
    `server_test.go`'s `TestAPromotionAppendsNoEvent` and
    `qa_promotion_revoked_test.go`'s `TestQAPromoRevokedAPromotionAppendsNoEvent`
    (R8, R11); `qa_joining_test.go`'s
    `TestQAAPromotionLeavesTheLogHeadWhereItWas`; `qa_controls_none_test.go`'s
    `TestQACNASpectatorWhoControlsAPartyMemberIsToldSo` (VTT-454 held by
    `TestQACNAPlayerWhoControlsAPartyMemberIsToldSo`);
    `qa_ruleset_by_role_test.go`'s
    `TestQAReadASpectatorReadsNoAbilityEvenWhereItsIdControlsAnActor` (VTT-692
    held by `TestASpectatorReadsNoAbility`).
  - Rewritten as a grant, keeping what they observe:
    `TestAPromotionAsTheSeatOpensMovesItOntoTheirPartyMember`,
    `TestAPromotionMovesASpectatorOffAShoulderThePlayerMayNotWatch`,
    `TestAPromotedSpectatorMovesOntoThePartyMemberTheyHold`,
    `TestAPromotionMovesEveryConnectionOfTheParticipant`,
    `TestAPromotedSpectatorKeepsWatchingAPartyMemberTheyHold`,
    `TestAPromotionTellsTheSeatTheNewRolesSet`, the "a spectator made a
    player" case of `TestQAWatchableAPromotionChangesTheSet`,
    `TestQACNASpectatorsControlChangeSendsTheSetAgain`,
    `TestASpectatorWhoControlsAPartyMemberIsSentItsAbilityIds`,
    `TestAPromotionBitesWithoutReconnecting`
    (R3), `TestQAWatchableTheDMAndTheAgentAreSentNoSet`'s step, and
    `qa_promotion_revoked_test.go`'s announcement tests, whose trigger
    becomes a grant (VTT-032, R18, R19).
  - Rewritten as "Make spectator", expecting its batch:
    `TestDMPromotesASpectatorOverTheWire` (R8 to R10);
    `TestPromotionCannotUNMAKEADMOrAgent`'s control step;
    `TestAPlayerMadeASpectatorKeepsWatchingTheirPartyMember`,
    `TestQARejudgePlayerPromotedToSpectatorKeepsItsPartyMemberLog`'s
    observation (sent the revocation's entry, not moved: R35);
    `TestQACNMakingAPlayerASpectatorKeepsSayingTheyControlOne` becoming
    `TestQACNMakingAPlayerASpectatorSaysTheyControlNone`; the "a player made a
    spectator" case of `TestQAWatchableAPromotionChangesTheSet`;
    `TestQAWatchableAChangeThatLeavesTheSetAloneSendsNone` loses its last
    step.
  - Rewritten for the derivation (M2, M7, D18): `metadata_test.go`'s
    fixture grants its player an actor; `TestALineOutOfCharacterSpeaksAsNoActor`'s
    player speaker controls one; `TestQADSAPlayerWhoControlsNothingGetsOneTextAtEverySquare`'s
    Ivo controls an actor with no token; `TestAuthorizeTableAllCommandsAllRoles`'s
    spectator column judges a participant who controls nothing, and so do
    the spectator cases of `TestOnlyTheDMAndTheAgentMayGiveOrTakeAnAbility`,
    `TestQADTSpectatorIsRefused`, `TestQAWhAuthorizeAdmitsAWhisperFromTheDMTheAgentAndAPlayerAlone`,
    `TestOnlyTheDMAndTheAgentMayDescribeAToken`,
    `TestAnActorWithNoDeclaredKindIsNoShoulderHoweverManyHoldIt`,
    `TestQAPPTheSpectatorsPerchIsUnchanged` and
    `TestQAWatchableASpectatorsOpeningNamesEveryPartyMember`; M7's player
    literals gain an actor; M7's wire tests as D18 says.
  - Kept, green under M3, their citation of VTT-388 moved:
    `TestQAQueuedDemotedPlayerKeepsStreamingItsLog` and
    to R35, and `TestQAPPDemotingAPlayerToSpectatorKeepsThePerch` deleted
    with `TestQAPPPromotingASpectatorToAPlayerWithoutAPartyMemberKeepsThePerch`,
    their premise a perched player (D14, D25), R35's test holding what a
    demoted participant watches;
    `TestQARejudgeASpectatorPromotedToSpectatorKeepsItsPerch` to R8.
  - `convert_test.go`'s `promote_participant` reason; `identity_test.go`'s
    `SetRole` tests deleted with it (VTT-029, VTT-030, VTT-037, VTT-066's
    half).
- **Task 4:** M8's tests: its `dm-view.test.ts` roster tests become R13
  to R17's; `command-surface.test.ts`'s `promoteParticipant` row reads
  `demote-` and its fixture's roster holds a player who controls `a1`;
  `app.test.ts`'s roster-learns test finds `boot-p-new`; under D14 the
  perch-line tests are deleted with VTT-607 and the `qa-refusal-words`
  tests that assert the line beside their own subject lose that assertion;
  `commands.test.ts`'s builder test (D12).
- **Task 5:** M6's search: every test mint of a player becomes a spectator;
  `cmd/vtt/cli_test.go`'s `--role player` invites become `spectator` and one
  asserts R23's refusal; `internal/gateway/scenario_test.go`'s mint.
- **Task 6:** `tools/toolgen/main_test.go`'s description tests; the golden.
- **Task 7:** D20's specs.

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R35 and the
  candidates, as the answers leave them; allocate ids with `requirement-id`;
  commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: a control change re-announces whom it names (R18, R19 for grants, revocations and removed actors; D8)

- **Files:** `internal/gateway/server.go` (`announcePromotion` renamed
  `reannounce`; its calls after a grant's and a revocation's append and in
  `handleRemoveActor`); a new `internal/gateway/reannounce_test.go`;
  `docs/requirements.md`.
- **Tests first, seen red:** after a DM's grant to a connected spectator,
  their own connection, the DM's and another player's are each sent a
  `PresenceChanged` naming them, and a revoked watcher is not; the same after
  the agent's revocation, a player's own revocation, and a `remove_actor` of
  an actor two participants control (both named); a grant to a participant
  with no connection sends no such frame to anyone (R19).
- **True when:** the new tests pass and failed before; `go test
  ./internal/gateway/ ./internal/mcp/ ./cmd/vtt/` passes; `go vet`, lint and
  `check:comments` pass.
- **Deliberate breaks:**
  - Announce only the grant: the revocation and `remove_actor` cases turn
    red.
  - Announce `remove_actor`'s first controller alone: its two-controller
    case turns red.
  - Hand `reannounce` an empty deny set: the revoked watcher's case turns
    red.
- **The state here:** every control change tells the windows to read again;
  nothing reads differently yet.

### Task 2: a grant names a participant the table holds (R6, R7; D7, D17)

- **Files:** `internal/gateway/server.go` (the grantee step); a new
  `internal/gateway/grantee_test.go`; M5's tests; `docs/requirements.md`
  (R6's and R7's cells; VTT-147's, VTT-161's, VTT-162's, VTT-289's and
  VTT-503's).
- **Tests first, seen red:** a DM's and an agent's grant naming an id nobody
  minted, and one naming a removed participant, are each refused with R7's
  text and append nothing, the actor's controllers unchanged; a grant to the
  player, the DM and the agent is appended; a kindless grant to an unknown id
  is told about its kind; a player's grant naming an unknown id is refused
  with the table's text, as every player's grant is.
- **True when:** the new tests pass and failed before; the packages Task 1
  names pass.
- **Deliberate breaks:**
  - Drop the `ErrInvalidToken` arm: the unknown and removed cases turn red.
  - Echo the id in the text: R7's test turns red.
  - Move the step before `validateForm`: the kindless case turns red.
  - Move it before `authorize`: a player's grant to an unknown id answers
    R7's text, and the case asserting the table's refusal turns red.
- **The state here:** no grant gives control to someone the table does not
  hold.

### Task 3: the role is the log's (R1 to R5, R8 to R12, R18 for promotions; D1 to D6, D18, D22; withdrawals)

- **Files:** `internal/gateway/authz.go` (`roleIn`, `Authorize`,
  `authorizePromotionTarget`); `viewpoint.go` (`MayWatch`,
  `defaultViewpoint`, with M13's line untouched); `server.go` (`authorize`
  handing on the derived participant, `handleCommand`, `handlePromotion`,
  `controlRevocations`, `handleRemoval` through it, `onEntry`,
  `pumpState.revoked`, the dispatch comment); `seat.go` (`judgedAs`,
  `rejudge`); `metadata.go` (`handleMe`, `handleParticipants`,
  `abilitiesReadBy`); `internal/identity/identity.go` (`SetRole` deleted,
  the docs) and `internal/identity/identity_test.go` (its `SetRole` tests);
  new `internal/gateway/role_test.go` and
  `internal/gateway/make_spectator_test.go`; the Task 3 tests above;
  `docs/requirements.md`.
- **Tests first, seen red:**
  - `roleIn`: a participant identity holds as a spectator is a player while
    an actor lists them, a spectator otherwise; one held as
    a player who controls nothing is a spectator; the DM, the agent and an
    unknown role are what identity says.
  - Over the wire, Robin minted a spectator: granted a party member, her
    `/api/me` and the roster say `player` (R1, R2), her move of its token
    is judged by the player's rules and its event says `player` (R3, R4);
    granted a party member while perched on Lera's, her connection moves
    onto her own and its set names hers alone (R5, R33); she reads its
    abilities (VTT-691). Revoked her last actor, by the DM, the agent,
    herself, and by `remove_actor`, each in its own case: `/api/me` and the
    roster say `spectator`, and her out-of-character line is refused (R1 to
    R3).
  - "Make spectator": Lera, controlling two party members and a third she
    shares with Ivo, ids out of name order, is made a
    spectator by the DM: the log gains exactly her revocations, in actor-id
    order, each naming her and stamped with the DM (R8, R9); the result
    carries the batch's first sequence (R10); Ivo still controls the shared
    one; her connection is sent a `PresenceChanged` naming her (R18) and the
    set of one who controls none, and keeps watching what it watched
    (R35). The same by the agent, stamped with the agent. A spectator
    made a spectator: nothing appended, no sequence (R8, R10). A promotion to
    player and to dm: refused, nothing appended, the player's in OD-5's
    words (R11, R12).
- **True when:** the new tests pass and failed before; removal's tests
  (VTT-557 to VTT-566) pass unchanged; M7's instrument, run in a copy, names
  no unit test handing `Authorize`, `MayWatch` or `watchable` a player
  literal who controls nothing but `roleIn`'s own test and
  `TestAuthorizeEmptyParticipantMatchesNothing` (D18), and every wire test it names
  judges a minted player only before their grant; `go test ./internal/...
  ./cmd/... ./tools/...` passes; `task check:requirements-chain` passes.
- **Deliberate breaks:**
  - Read identity's role in `handleMe`: R1's grant case turns red.
  - Derive in `authorize` but not in `Authorize`: `roleIn`'s `Authorize`
    case (a spectator literal who controls, judged a player) turns red.
  - Hand `handleCommand` identity's participant after `authorize`: R4's
    stamp turns red.
  - Drop `authorizeTokenOwnership`'s controller check:
    `TestAuthorizePlayerOtherTokenDenied`, its player now holding an actor of
    their own, turns red (D18).
  - Name the issuer as the revoked, or stamp the target: R8's and R9's tests
    turn red.
  - Accept a promotion to player: R11's test turns red.
  - Return no sequence after a batch: R10's test turns red.
- **The state here:** the server answers every participant by the log; the
  console still offers "Make player", which the server now refuses, and
  "Make spectator" takes characters without asking (Task 4).

### Task 4: the console and the perch (R13 to R17; D11 to D14; OD-4; VTT-607)

- **Files:** `client/src/view/dm.ts` (the roster rows; `DMDeps.roster`'s,
  `DMDeps.send`'s and the "Who may do what" comments); `client/src/commands.ts`
  (`promoteParticipant`); `client/src/app.ts` (the presence block's
  comments; `mayPerch`'s player half and `playsNone`, D14);
  `client/src/view/spectator.ts` (D14: the line and `PerchControl.playsNone`);
  `client/src/session.ts` (`onPresence`'s doc); M8's tests;
  `client/test/dm-view.test.ts`; `docs/requirements.md`; a rebuilt
  `cmd/vtt/webdist`.
- **Tests first, seen red:** a roster holding a DM, an agent, Lera (a player
  controlling `a1` "Gobo", `a2` "Ash" and an unnamed `a3`, so the ids run
  against the names) and Zoe (a spectator), with another participant
  controlling `a4`: Zoe's row offers Boot and nothing else, no row offers a
  `promote-` control (R17); Lera's "Make spectator" asks
  "Make Lera a spectator? They no longer control Ash and Gobo and a3." under
  OD-5 A's joiner (R13, R14); a no sends nothing; a yes sends one promotion
  of Lera to spectator (R15) and reads the roster again only once it is
  answered (R16); a player's window sent a set saying it controls no party
  member draws the player panel and no perch, and a spectator's window
  perches whatever its set says (R29); the tests that held VTT-457, VTT-460
  and VTT-696 re-cited to R30, R31 and R32.
- **True when:** the new tests pass and failed before; the whole client
  suite, `task client:typecheck` and P8's checks pass.
- **Deliberate breaks:**
  - Draw "Make player" again: R17's test turns red.
  - Send before asking: the no case turns red.
  - Sort the actors by id: R14's test turns red.
  - Restore `mayPerch`'s player half: R29's test turns red.
  - Name an unnamed actor by its empty name: R14's test turns red.
  - Name `a4`'s holder's actor too: R13's test turns red.
  - Refresh beside the send: R16's test turns red.
- **The state here:** the console makes no player but by a grant and asks
  before it takes characters.

### Task 5: identity mints no player (R23, R24; D16; OD-1 A)

- **Files:** `internal/identity/identity.go` (`CreateInvite`,
  `Participant`'s doc); `internal/identity/identity_test.go`;
  `cmd/vtt/invite.go` (the help); `cmd/vtt/harness_boot.go` (`mintInvites`);
  `client/e2e/setup.ts`; M6's test mints; `docs/requirements.md`.
- **Tests first, seen red:** `CreateInvite` with `player` returns an error
  and `List` holds no new row (R23); `vtt invite --role player` exits
  non-zero, prints no token and prints OD-5's words.
- **True when:** `go test ./internal/... ./cmd/... ./tools/...` passes with
  no assertion changed but the mint's role (M6); the scenario goldens and
  the soak's pinned counts are unchanged.
- **Deliberate breaks:**
  - Insert the row before the refusal: R23's "no new row" turns red.
  - Translate no scenario player in `mintInvites`:
    `TestScenarioLibraryRunsSelfContained` turns red.
- **The state here:** no store says what the log contradicts.

### Task 6: the tool texts (R20 to R22; D15)

- **Files:** `tools/toolgen/main.go`, `tools/toolgen/main_test.go`;
  `contract/vtt/v1/commands.proto` (D15's comments); `contract/gen`,
  `contract/gen/tools/tools.json`, `cmd/vtt/tools.json` by `task
  generate:contract`; `contract/testdata/expected_tools.json`;
  `internal/mcp/door_tools.go`; `docs/requirements.md`.
- **Tests first, seen red:** each description holds R20's, R21's and R22's
  sentences, and `promote_participant`'s holds neither "writes no event" nor
  "promote them to \"player\"".
- **True when:** `go test ./tools/toolgen/ ./internal/mcp/ ./contract/...`,
  `task check:drift` and the client tests that read the tool JSON pass.
- **Deliberate break:** restore "so it writes no event": R22's test turns
  red.

### Task 7: the real wire (D20; items 6, 8)

- **Files:** `client/e2e/joining.spec.ts`, `booting.spec.ts`,
  `speaking.spec.ts`, `whispering.spec.ts`, `handover.spec.ts`;
  `docs/requirements.md` (the e2e in R2's, R13's, R14's and R18's cells).
- **True when:** `task e2e` passes, these specs among it.
- **Deliberate breaks:**
  - Skip `reannounce` after a grant: `joining.spec.ts` finds no player panel
    on the guest's connection.
  - Send "Make spectator" without asking: the new handover test sees no
    dialog.
- **The state here:** the whole table, through a browser and a server.

### Task 8: the gate (item 11)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger` once,
  last, if a ceiling must come down (M12); check free space (the Go mutation
  gate grows `GOCACHE`) and the load (`os.getloadavg()`); run `task check`
  whole on the last code commit, detached under `/bin/sh`, output to a file,
  requiring its completion line; run `task e2e`. Tell the owner the client's
  stored mutation report is out of date from Task 4 and which commit it
  measured (SPEC-023).
- **True when:** `task check` and `task e2e` are green; the embedded bundle
  is the last client commit's.

### Task 9: the report, then the specifications and the blueprint

- **The report:** `docs/reports/2026-11-20-a-spectator-never-holds-control.md`,
  with the `implementation-report` skill and its reading review: each "Done
  looks like" item with its observation; the rows and what turned each red;
  Rule 9; G1 to G7 and the answers to OD-1 to OD-6; D9 and D10 in a line
  each; every deleted test with its reason; Phase 4a's adjudications under
  their own heading; the gate.
- **The specifications,** with the `specification` skill, from the report,
  each changed sentence run against the code by someone other than its
  writer, their `## Requirements` lists gaining the new ids and losing the
  withdrawn:
  - SPEC-007: what appends nothing (no longer `promote_participant`); a
    promotion's batch; `promote_participant` accepts spectator alone; what a
    grant refuses.
  - SPEC-009: identity holds the DM's and the agent's roles, and the log a
    player's (OD-1's column); "Promotion is a command" rewritten as "Make
    spectator"; the grantee; the re-announcement; `SetRole` gone; the
    re-resolution paragraph without "judges by the current role".
  - SPEC-011: `reannounce` and its callers; `rejudge`'s callers, removal
    alone.
  - SPEC-012: `handleMe`, the roster and `abilitiesReadBy` by the log.
  - SPEC-013: the derivation in `Authorize`; the grantee step in the command
    path; the promotion's bound and its batch in the dispatch paragraph;
    "Promotion's bounds" in "What this record does not decide".
  - SPEC-015: `MayWatch` and the default by the log; the promotion bullet,
    `judgedAs` and `rejudge` out; the ability ids' "whatever the
    participant's role" without its spectator example; "A promotion appends
    nothing ... raises no wake" out; the credential paragraph.
  - SPEC-017: `handleRemoveActor` re-announces each controller.
  - SPEC-022: the roster rows; the perch per OD-4; "until a promotion makes
    it one" and the presence frame's reason.
  - Blueprint §16: the owner's sentence (D21).
- **True when:** the report, the specifications and the blueprint are
  committed; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and `go doc
./internal/gateway` and `go doc ./internal/identity`, never the source; the
client's surfaces as signatures (`renderDMConsole`, `DMDeps`, `PerchControl`,
`promoteParticipant`, `boot` with its fetch and socket seams as
`app.test.ts` uses them). Tell it: never run `task check`, `task e2e` or
Stryker; its files are new `qa_*` or `qa-*` test files; go-arch-lint holds
the layer imports and semgrep refuses game-system words; client tests under
`env -u CLAUDECODE bun test`, reading no file outside `contract/gen` and
`contract/testdata`; fixtures with more than one actor, a shared one, a
non-party one only the DM holds, and the one asked about not first by id; a participant minted
as a spectator and granted is the player (OD-1 A); the tree is hashed before
and after it. Known and not findings: D10's interleavings; D9's toast; a
roster a frame behind the log (D11); the at-bound grant unreachable over the
wire (G6); a stored `player` row from an old file (no migration); a
`PresenceChanged` after a grant that changed nobody's role (D8, by design).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: run `git grep -n '\.Role\b'
-- internal/gateway ':!*_test.go'` and check that every reader of the role of
a participant who may be a player or a spectator reads it through `roleIn`,
from the snapshot its decision judges (D2); check `handleRemoval` behaves as
before through `controlRevocations`; check no test, row or specification left
in the tree states a spectator who controls, a player who controls nothing,
or a player who controls an actor that is not a party member, as a state the
system holds (item 10, D18, D25), and that each test M7
names still reaches the rule it was written for; check each deleted or
rewritten test against "The existing tests that change" and its row's cell
(rule 2); check each scope claim ("every", "only", "no other") names the
search that bounds it; check each comment under `internal/`, `cmd/` and
`client/src` is a warning or a pointer and none still says a promotion
appends nothing, changes a role, or re-judges a seat; check M13's line stands.
