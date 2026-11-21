# A participant is a player exactly while they control an actor — report

Ticket:
`docs/superpowers/specs/2026-11-20-a-spectator-never-holds-control-design.md`.
Plan: `docs/superpowers/plans/2026-11-20-a-spectator-never-holds-control.md`.
Branch `feat/a-spectator-never-holds-control` from main `c99715b`. Commits:
`be756c9` (Task 0: ticket, plan, register), `42bd59e` (Task 1, a control change
re-announces whom it names), `b0b1715` (Task 2, the grantee step), `9518c94`
(Task 3, the role is the log's), `39504b1` (Task 4, the console and the perch),
`dd68d20` (Task 5, identity mints no player), `fd213c9` (Task 6, the tool
texts), `d354ce2` (Task 7, the real wire; the plan's last task), `d840679` (the
comment ledger) and `9a86ccc` (a QA test made to pin Task 6's `place_token`
text, after the gate's first run). Every code reference below is to `9a86ccc`,
the last commit to change code; its only change to code since `d354ce2` is that
test's expected text.

## Done looks like

1. [x] **A grant makes a spectator a player.** `roleIn`
   (`internal/gateway/authz.go`) judges a participant identity holds as a
   player or a spectator by the actors of the snapshot a decision judges: a
   player while an actor counts them among its `controller_ids`, a spectator
   otherwise; `Authorize` derives it first, and `authorize` hands the derived
   participant to every step of `handleCommand` after it.
   - `internal/gateway/make_spectator_test.go`'s
     `TestAGrantMakesASpectatorAPlayerOverTheWire`: Robin, minted a spectator,
     is answered `spectator` by `GET /api/me` and by the DM's roster and is
     refused a line as Ash; after the DM's grant of Ash both answer `player`,
     her line as Ash is accepted, and its event carries her id, `player` as its
     `actor_role` and Ash's name.
   - Independently: `internal/gateway/qa_role_from_log_test.go`'s
     `TestQARLTheRoleAnsweredFollowsControlThroughGrantsAndRevocations` and
     `TestQARLASpectatorGivenACharacterIssuesAPlayersCommandsUntilTheLastIsGone`.
   - The unit half: `internal/gateway/role_internal_test.go`'s
     `TestAuthorizeJudgesASpectatorWhoControlsAnActorAsAPlayer`.
   - On the real wire: `client/e2e/joining.spec.ts`\'s "a stranger with one
     link joins, watches, is given a character, and plays" (the roster's role
     cell for Robin reads `player` after the grant).
   - On `c99715b` `GET /api/me` answered identity's stored role, which no
     grant changed (the plan's M2).
2. [x] **The last revocation makes a player a spectator.**
       `make_spectator_test.go`'s
       `TestTheLastRevocationMakesAPlayerASpectator`, a case each for the DM,
       the agent, the player herself and `remove_actor`: after the first of her
       revocations Robin is a `player` by `/api/me` and the roster and speaks
       out of character; after the last, both answer `spectator` and her
       out-of-character line is refused with the spectator's table text.
       Independently: `qa_role_from_log_test.go`'s
       `TestQARLEveryWayOfEndingControlMakesASpectatorAtTheLastActor`. On
       `c99715b` a revocation left identity's role, and so every answer, as it
       was.
3. [~] **No participant who controls nothing is a player.**
   - **What changed, and why.** The item's two routes to such a participant,
     "minted or promoted as a player", are closed rather than answered (the
     owner's A to OD-1 and OD-2; the plan's G3). A mint as a player is refused:
     `internal/identity/identity_test.go`'s
     `TestCreateInviteRefusesAPlayerAndEveryOtherRoleAndMintsNothing` and
     `cmd/vtt/cli_test.go`'s
     `TestInviteAsAPlayerExitsNonZeroWithTheRefusalAndPrintsNoToken` (VTT-911).
     A promotion to player is refused: `make_spectator_test.go`'s
     `TestAPromotionToAnyRoleButSpectatorIsRefusedAndAppendsNothing` (VTT-899,
     VTT-900).
   - **What is left of the item.** A participant identity holds as a player
     from a campaign file written before this ticket: `ParseRole` still reads
     `player`, and nothing migrates it. `qa_role_from_log_test.go`'s
     `TestQARLAParticipantStoredAsAPlayerIsASpectatorUntilGivenACharacter`
     writes such a row by SQL: `GET /api/me`, and `GET /api/participants` asked
     by the DM and by the agent, answer it `spectator`, and its
     out-of-character line is refused, until a grant.
   - **The seat** judges through `roleIn` in `MayWatch` and `defaultViewpoint`:
     `role_internal_test.go`'s
     `TestRoleInJudgesAPlayerOrASpectatorByTheActorsAndNoOtherRole` judges its
     "a player who controls nothing" a spectator;
     `internal/gateway/seat_internal_test.go`'s
     `TestWatchableNamesEveryPartyMemberForAParticipantWhoControlsNothing` and
     `internal/gateway/watchable_test.go`'s
     `TestAParticipantWhoControlsNothingPerchesAndReopensOnIt` hold a
     participant who controls nothing to the spectator's set and perch. Over
     the wire,
     `TestQARLAParticipantStoredAsAPlayerIsASpectatorUntilGivenACharacter`
     holds what a seat of a stored player row may watch: before the grant and
     after the revocation, its `set_viewpoint` on a party member the row does
     not control is accepted and answered by the perch on it. No test observes
     the viewpoint such a seat opens on (see "What could not be established").
   - On `c99715b`, `internal/gateway/metadata_test.go`'s
     `TestMetadataMeIdentifiesTheCaller` answered a minted player who
     controlled nothing `player` (M2).
4. [x] **No spectator controls an actor.** `Authorize`, `MayWatch` and
       `defaultViewpoint` derive inside themselves, so even a unit test that
       hands them a spectator literal who controls an actor is judged a player:
       `role_internal_test.go`'s
       `TestAuthorizeJudgesASpectatorWhoControlsAnActorAsAPlayer` and
       `TestMayWatchJudgesASpectatorWhoControlsAnActorAsAPlayer`.
   - Over the wire: `TestAGrantMakesASpectatorAPlayerOverTheWire` (the two
     reads, the command and its stamp); `make_spectator_test.go`'s
     `TestAGrantMovesAPerchedSpectatorOntoTheirOwnPartyMember` (the seat:
     perched on Lera's Ash and granted Bram, Robin's connection is told Bram
     alone and moved onto it); `qa_role_from_log_test.go`'s
     `TestQARLWhatAConnectionWatchesFollowsTheControlItsParticipantHolds`.
   - "Every answer" is held per surface by these and by a search:
     `git grep -n '\.Role\b' -- internal/gateway ':!*_test.go'`, run while
     writing this report. Every reader it finds reads a participant `roleIn`
     returned (`Authorize`, `MayWatch`, `defaultViewpoint`, `authorize`'s
     result and the handlers it is handed to, `handleMe`,
     `handleParticipants`), or asks only whether one is the DM or the agent
     (`runsTheTable`, `watchesALog`, the route gates, the targets of
     `handlePromotion` and `handleRemoval`), or is `roleIn` itself.
   - On `c99715b`, `qa_controls_none_test.go`'s
     `TestQACNASpectatorWhoControlsAPartyMemberIsToldSo` built a spectator who
     controls a party member and asserted what it was told.
5. [x] **A grant to nobody is refused.** `granteeRefusal`
   (`internal/gateway/grantee.go`), called by `handleCommand` after
   `authorize` and `validateForm` and before anything is appended, asks
   `identity.Lookup` of the grantee.
   - `internal/gateway/grantee_test.go`'s
     `TestAGrantNamingAParticipantTheTableDoesNotHoldIsRefused`: the DM's and
     the agent's grants naming an id nobody minted, and a revoked participant's
     id, are each refused with `There is no such participant.`, and the next
     narration lands at the next sequence.
   - Independently: `internal/gateway/qa_grantee_test.go`'s
     `TestQAGRAGrantNamingAParticipantNeverMintedIsRefusedPlainly`,
     `TestQAGRAGrantNamingARemovedParticipantIsRefusedWithTheSameText` and
     `TestQAGRAGrantNamingAnEmptyParticipantIdIsRefusedPlainly`; identity's
     other errors: `internal/gateway/grantee_internal_test.go`'s
     `TestAGrantIsRefusedWhenIdentityCannotReadAParticipant`.
   - On `c99715b`, `server_test.go`'s `TestDMGrantsControlOverTheWire`
     granted to an id nobody minted and was appended (M5).
6. [x] **"Make spectator" takes every actor.**
   - The console: `client/test/dm-view.test.ts`\'s "Make spectator asks first,
     naming the player and what they control by name, else id, in that order,
     and a no sends nothing" and "a confirmed Make spectator sends one
     promotion of its row's player to spectator, and reads the roster again
     once answered"; independently, `client/test/qa-console-perch.test.ts`'s
     VTT-901 to VTT-904 tests.
   - The gateway: `make_spectator_test.go`'s
     `TestMakeSpectatorRevokesEveryActorThePlayerControlsInActorIDOrder`, by
     the DM and by the agent: the log gains one `ActorControlRevoked` per actor
     Lera controls, in actor-id order, each naming her and stamped with the
     issuer, and nothing else; the result carries the batch's first sequence;
     `/api/me` and the roster then answer her `spectator`, and Ivo, who shares
     one of those actors, stays a `player`. Also `server_test.go`'s
     `TestDMMakesAPlayerASpectatorOverTheWire` and, independently,
     `qa_role_from_log_test.go`'s
     `TestQARLMakeSpectatorRevokesEachActorTheParticipantControlsAsOneBatch`.
   - On the real wire: `client/e2e/handover.spec.ts`\'s "the DM makes a player
     a spectator, and they perch". The dialog reads "Make Lera a spectator?
     They no longer control Ash and Bram.", the DM's window sends exactly one
     promotion naming Lera and `spectator`, "Who controls what" names no holder
     for either actor, and the roster's role cell reads `spectator`.
   - The question names the actors in the order of their names, not of their
     ids (the owner's answer of 2026-11-21; Deviations).
   - On `c99715b` the row sent its promotion without asking, and
     `TestAPromotionAppendsNoEvent` held that a promotion appended nothing.
7. [x] **The console offers no "Make player".** `renderDMConsole`
       (`client/src/view/dm.ts`) gives a spectator's row Boot alone and a
       player's row "Make spectator" and Boot. `dm-view.test.ts`\'s "a
       spectator's row offers Boot and nothing else, and no row offers a
       promotion" and "everyone is shown, a player can be made a spectator, and
       nobody is offered promotion to player"; independently,
       `qa-console-perch.test.ts`\'s "VTT-905: no control anywhere in the
       console is a promotion to player". On `c99715b`, `dm-view.test.ts`'s "a
       spectator can be promoted, and a player is not offered promotion again"
       asserted "Make player" on a spectator's row (M8).
8. [~] **A window follows its role.**
   - **What changed, and why.** The window needed no new code to re-read its
     role (the plan's G2 and D13): a `PresenceChanged` naming the window
     already started a `GET /api/me` read and then a ruleset read (VTT-712).
     So the following half is not held by a client test that fails on
     `c99715b`; it is held by the server's re-announcement and by the real
     wire, both of which fail there.
   - **The server.** `reannounce` (`internal/gateway/server.go`, once
     `announcePromotion`), called through `reannounceControlChange` after an
     accepted grant or revocation, in `handleRemoveActor` for each controller
     of the removed actor, and after a promotion that appended (VTT-906,
     VTT-907, VTT-924): `internal/gateway/reannounce_test.go`'s
     `TestAControlChangeIsAnnouncedToEveryUnrevokedConnectionNamingWhomItNames`,
     `TestARemovedActorIsAnnouncedNamingEachOfItsControllers`,
     `TestAControlChangeOfAParticipantWithNoConnectionIsAnnouncedToNobody` and
     `TestARevocationTheFoldRefusesAnnouncesNothing`; independently,
     `qa_reannounce_test.go`.
   - **The window, on that frame.** `client/test/app.test.ts`\'s "a granted
     spectator's own screen catches up without reconnecting",
     `client/test/qa-console-perch.test.ts`\'s "VTT-917: the window follows the
     role last read: spectator, then player, then spectator" and
     `client/test/qa-window-reread.test.ts`\'s "a spectator's window re-read as
     a player at its re-announcement reads the ruleset again".
   - **Without a reload, on the real wire.** `joining.spec.ts`\'s "a stranger
     with one link joins, watches, is given a character, and plays": the
     guest's window draws the player panel after the grant, on the socket it
     joined on. `handover.spec.ts`\'s "the DM makes a player a spectator, and
     they perch": Lera's window leaves the player panel for the perch, on her
     one socket both ways. Task 7's break that skipped the re-announcement
     after a grant turned the joining test red.
   - **No player's window perches** (`mayPerch` in `client/src/app.ts` answers
     spectator alone): `app.test.ts`\'s "a player's window sent a set saying it
     controls no party member draws the player panel and no perch" and
     `qa-console-perch.test.ts`\'s "VTT-917: a player's window never perches,
     the flag false or true", the first red on `c99715b`, where such a window
     perched.
   - **The line is gone.** `git grep 'You play no character' -- client/src
     cmd/vtt/webdist` finds nothing; `qa-console-perch.test.ts`\'s "VTT-917: a
     player's last party member taken back leaves the panel, no perch and no
     old line" asserts it absent.
9. [x] **The tool texts say so.** `tools/toolgen/main.go`.
   - `tools/toolgen/main_test.go`'s
     `TestGrantActorControlSaysAGrantMakesAPlayerOfAPartyMemberOnly` (a grant
     makes a spectator a player; a participant who is neither the DM nor the
     agent is granted only a party member; a grant naming a participant the
     table does not hold is refused; VTT-908).
   - `TestGrantActorControlSaysACreatureAPlayerRunsIsAPartyMember` (the
     take-back order in `kind`'s description; VTT-931).
   - `TestRevokeActorControlSaysItEndsAPlayersRunOfACreature` (the last
     revocation makes a spectator; VTT-909).
   - `TestPromoteParticipantSaysItMakesAPlayerASpectator` (what is left of
     promotion: one revocation per actor, `spectator` the one role, a
     participant becomes a player when granted an actor; VTT-910).
   - The golden `contract/testdata/expected_tools.json`, `cmd/vtt/tools.json`
     and `contract/gen` changed with them in `fd213c9`;
     `main_test.go`'s `TestToolsMatchGolden` and `task check:drift` hold them.
   - The proto comments of `GrantActorControl`, `PromoteParticipant`,
     `PresenceChanged`, `SetViewpoint`, `WatchableChanged` and `ActorKind`,
     and `getParticipantsDescription` (`internal/mcp/door_tools.go`), say the
     same.
10. [ ] **No record models the old split.** Not met on this tree, and the
    item stands: the tests and the register are done, save the title of
    `handover.spec.ts`'s second test (below), and the specifications and the
    blueprint are not.
    - **Tests.** D18's instrument, re-run before Task 3's commit, names no
      unit test handing `Authorize`, `MayWatch` or `watchable` a player
      literal who controls nothing, but `roleIn`'s own test and
      `TestAuthorizeEmptyParticipantMatchesNothing`, which assert that such a
      literal is judged a spectator or matches nothing. Task 5 then minted
      every test participant as a spectator. Each task's Phase 4b read for
      the old split; the tests whose subject was the old split are deleted
      (below).
    - **The register.** The rows below are withdrawn; VTT-454, VTT-455,
      VTT-605, VTT-606 and VTT-731 were read and stand as worded.
    - **Not yet.** SPEC-015 still says a promotion re-judges seats and that a
      player who controls no party member moves nothing; SPEC-022 still
      describes a player's perch and its line; `docs/blueprint.md` §16 still
      reads "A spectator, and a player who controls no character, follows a
      party member of their choosing in the same way." These follow from this
      report in their own commit (the plan's D24 and Task 9).
    - **Left in the tree** (see "What was deliberately left out"):
      `internal/testdb/testdb.go`'s header example, the title of
      `handover.spec.ts`'s second test, and `scenarios/adventure-night.json`'s
      ungranted "player".
11. [x] **The gate.** `task check` and `task e2e`.
    - The embedded bundle under `cmd/vtt/webdist` is `39504b1`'s build, the
      only commit on the branch to change `client/src`; `check:drift`, a step
      of `task check`, rebuilds it and fails on any difference.
    - A first run of the gate went red at `qa_describe_token_test.go`'s
      `TestQADTPlaceTokenToolDocsTheFourTexts`, which pinned the
      `place_token` name text Task 6 changed; `9a86ccc` makes it assert the
      shipped text (Deviations).
    - The client's stored mutation report measured `c99715b`; this ticket
      changed the client after it, so `task check` judges none of it and
      names that commit (SPEC-023).
    - `task check` is green on `9a86ccc` from a clean tree, the Go mutation
      gate with no survivor left unadjudicated; `task e2e` is green on
      `9a86ccc` from a clean tree, `handover.spec.ts`'s network-drop test
      skipped as it is on `main` (`bafaa27`).
12. [x] **A player's actor is a party member.** `granteeRefusal`, after the
    grantee's lookup.
    - `grantee_test.go`'s `TestANonPartyGrantToAPlayerOrASpectatorIsRefused`:
      the DM's and the agent's non-party grant to two spectators who control
      nothing, of a non-party actor and of a party member, is refused with
      `<name> can only be given a party member.` and appends nothing
      (VTT-913, VTT-915); `qa_grantee_test.go`'s
      `TestQAGRANonPartyGrantToAPlayerOrASpectatorNamesThem` holds the same
      refusal for grantees who already control a party member.
    - `grantee_test.go`'s
      `TestANonPartyGrantOfAnActorAPlayerControlsIsRefused`: the DM's non-party
      grant to themselves of an actor Lera controls is refused with `Lera
      controls Ash: revoke it from them before you make it a non-party actor.`
      and appends nothing (VTT-926, VTT-927);
      `TestANonPartyGrantOfAnActorNoPlayerControlsIsAppended` shows the same
      grant appended where no such participant controls the actor.
    - Independently, `qa_grantee_test.go`: the joiner across several
      players, the DM or the agent as a co-controller not named, a removed
      controller neither named nor counted, an unnamed actor named by its id,
      and the take-back order
      (`TestQAGRTheDMRevokesAPlayersControlAndThenMakesTheActorNonParty`).
    - On `c99715b` both grants were appended (`validateGrantActorControl`
      checked the form alone).

## What the rules became

- **A participant who is neither the DM nor the agent is a player while the log
  holds them among the controllers of an actor, and a spectator otherwise, in
  every answer and decision:** VTT-889 (`GET /api/me`), VTT-890 (`GET
  /api/participants`), VTT-891 (a command), VTT-892 (its stamp), VTT-893 (what
  a connection may watch); VTT-921, VTT-922 and VTT-923 succeed the watching
  rows D25 made false. "Every decision" is held per surface by those rows and
  by reading (Phase 4b's search of the readers of `.Role`, D2). "Of any kind"
  is held by reading alone: with D25 no state reached over the wire, save D10's
  interleavings and a campaign file written before this ticket, tells
  `roleIn`'s count of any actor from a count of party members.
- **Every actor such a participant controls is a party member:** VTT-913
  (the grantee's kind) and VTT-926 (the actor's other controllers), their
  words VTT-915 and VTT-927; VTT-928 (identity is judged only after
  `Authorize` and the form check); VTT-925 (the form check, succeeding
  VTT-151). VTT-914 and VTT-916 were allocated and withdrawn (Deviations).
- **A grant naming a participant id identity does not hold, or holds
  revoked, is refused and appends nothing:** VTT-894; its words VTT-895.
- **Making a player a spectator revokes every actor they control, in one
  command:** VTT-896; the stamps VTT-897; the result's sequence VTT-898; any
  other role refused VTT-899, a promotion to player in OD-5's words VTT-900;
  its re-announcement is in VTT-906.
- **The console asks first, naming the actors, and sends nothing on a no:**
  VTT-901; the question's words VTT-902; the send VTT-903; the roster read
  again once the promotion is answered, VTT-904.
- **The console offers no promotion to player:** VTT-905.

Rows the plan set beside the ticket's rules: VTT-906 and VTT-907 (the
re-announcement, item 8); VTT-908 to VTT-910 (the tool texts, item 9);
VTT-911 and VTT-912 (identity, OD-1); VTT-917 to VTT-920 (the window under
D14). Added during the work: VTT-924 to VTT-931 (Deviations).

Refused by the sort, as the plan records it:

- A row that `/api/ruleset` follows the derived role: VTT-691 and VTT-692 as
  worded hold it once `abilitiesReadBy` asks no role of a participant who is
  neither the DM nor the agent;
  `make_spectator_test.go`'s `TestASpectatorGrantedAPartyMemberReadsItsAbilities`
  cites VTT-691.
- A row that a granted spectator's connection moves onto their party member:
  VTT-921 as worded, once VTT-893 makes them a player.
- A row that a promotion of a participant who controls nothing appends
  nothing: VTT-896's observation with no actor.
- A row for the window following its role: VTT-712 holds the window's half,
  VTT-906 the server's.
- A row that the perch draws no line: an absence of deleted code.
- The races and the toast's order: D9 and D10.
- A row that "Make spectator" on a row whose participant controls nothing in
  the console's state asks nothing: theoretical (D11).

Withdrawn on this branch, each with its successor where it has one:
VTT-025 (VTT-899); VTT-029 and VTT-030 (no promotion writes to identity);
VTT-031 (VTT-891); VTT-036 (VTT-896, VTT-899); VTT-037 (a promotion's
revocations name the promoted alone); VTT-047 (VTT-906); VTT-066 (VTT-911,
VTT-912); VTT-067 (VTT-929); VTT-068 (`SetRole` is gone); VTT-151
(VTT-925); VTT-309 (VTT-906); VTT-310 (VTT-907); VTT-340 (VTT-921); VTT-370
(VTT-913, VTT-926); VTT-387 (VTT-348); VTT-388 (VTT-923, VTT-921); VTT-389
(VTT-908, VTT-931); VTT-452 (no player controls no party member); VTT-453
(VTT-922); VTT-456 (VTT-923); VTT-457 (VTT-918); VTT-458, VTT-459 and
VTT-461 (VTT-917); VTT-460 (VTT-919); VTT-607 (the line goes); VTT-696
(VTT-920); VTT-914 (VTT-926); VTT-916 (VTT-927).

## The deliberate breaks

Each was made by an edit, run, and restored; each turned its tests red.

- **Task 1** (`reannounce_test.go`, `qa_reannounce_test.go`,
  `reannounce_internal_test.go`): the grant announces nothing; the
  revocation announces nothing; `remove_actor` announces its first
  controller alone; `reannounce` denies nobody (red in
  `qa_promotion_revoked_test.go`'s deny-set test and in
  `TestAReannouncementIsNeverQueuedForARevokedWatcher`); a participant with
  no connection announced anyway; a control change announced before its
  append (`TestARevocationTheFoldRefusesAnnouncesNothing`).
- **Task 2** (`grantee_test.go`, `qa_grantee_test.go`,
  `grantee_internal_test.go`): an unknown grantee falls through; the refusal
  echoes the id; the grantee judged before the validators; judged before
  `authorize`; any kind may go to a player; a player's actor may be made
  non-party; one verb for many controllers; the DM named among the holders;
  a removed holder counted; the actor named by its id; an unnamed actor
  named blank; an unreadable grantee passes; an unreadable holder skipped.
- **Task 3** (`role_internal_test.go`, `make_spectator_test.go`,
  `qa_role_from_log_test.go` and the rewritten tests): `handleMe` reads
  identity's role; `Authorize` judges identity's role; `handleCommand`
  stamps identity's role; a promotion to player accepted, first in
  `handlePromotion` and then in the bound; the refusal loses its words;
  "Make spectator" returns no sequence; "Make spectator" not re-announced; a
  player may watch any party member; `controlRevocations` unsorted (see
  "What could not be established").
- **Task 4** (`dm-view.test.ts`, `app.test.ts`, `qa-console-perch.test.ts`):
  "Make player" drawn again; the send before the question; the actors sorted
  by id; the roster re-read beside the send; a player perches again; a
  grant, a revocation and a removal of an actor that read nothing; the
  re-read before the answer.
- **Task 5** (`identity_test.go`, `cli_test.go`, the scenario library):
  identity mints a player; a scenario's player minted as a player
  (`TestScenarioLibraryRunsSelfContained`).
- **Made in scratch copies of the tree by the agents that wrote or reviewed
  a task, and reported red there:** Task 3's controller check in
  `authorizeTokenOwnership` dropped (`TestAuthorizePlayerOtherTokenDenied`
  and its siblings) and the target stamped or named as the issuer (the
  "Make spectator" and removal tests); Task 4's unnamed actor shown by its
  empty name and another player's actor named in the question
  (`dm-view.test.ts`'s ask test); Task 5's row inserted before the refusal
  (`TestCreateInviteRefusesAPlayerAndEveryOtherRoleAndMintsNothing`).
- **Task 6** (`tools/toolgen/main_test.go`): the grant says nothing of making
  a player; the promotion "writes no event" again; the old take-back order.
- **Task 7** (the e2e, against the rebuilt binary): no re-announcement after
  a grant (`joining.spec.ts`); "Make spectator" sent without asking
  (`handover.spec.ts`).

## Rule 9: how MapTool does this

MapTool's roles are `PLAYER` and `GM` (`Player.Role` in
`net.rptools.maptool.model.player.Player`), with no watching role. Ownership
is a token's list of player names (`Token.isOwner`), and `Players.setRole`
changes the player database and touches no token. Nothing was borrowed:
MapTool's role is a permission level stored apart from ownership, which is
the split this ticket removes.

## The plan's gaps and the owner's answers

- **The answers.** OD-1 A (identity mints dm, agent and spectator; a stored
  `player` is judged by the log). OD-2 A (`promote_participant` is "Make
  spectator"). OD-3 A (the gateway re-announces). OD-5 A, both halves (the
  words, and ` and ` as the joiner). OD-6 A (one ticket). OD-4: none of A, B
  or C; the owner's answers of 2026-11-20 and 2026-11-21 became D25 (every
  actor a player controls is a party member), D14 (no player perches) and
  D21 (§16 loses the player).
- **G1, files the ticket did not name:** as the plan lists them, and beyond
  them `internal/gateway/grantee.go`, `README.md`'s invite example,
  `cmd/vtt/joinlink.go`'s comment, `client/src/metadata.ts`,
  `client/src/view/player.ts` and `client/src/wire.ts` (pointers to the
  withdrawn rows), `scenarios/door-watch.json` and
  `scenarios/shared-control.json` with their goldens, and the test files of
  M2, M3, M6 and M7 and those D25 reached.
- **G2, item 8:** item 8 above.
- **G3, item 3:** item 3 above.
- **G4, the ticket's open questions:** the derivation lives in the gateway, at
  each decision, from the snapshot it judges (D1, D2); `promote_participant` is
  "Make spectator" and `vtt invite` refuses `player` (OD-1, OD-2); a window
  learns its role from the re-announcement (OD-3, D8); over the wire, save
  D10's interleavings and a campaign file written before this ticket,
  `controls_no_party_member` is false for every player and true for every
  spectator, and the window no longer reads it for the perch or the ruleset key
  (D14, Deviations); a grant racing a revocation is D10.
- **G5, tests green for the other role's reason:** D18 (item 10). The
  instrument's re-run also found `qa_note_refusal_test.go`'s player subtest,
  `qa_plain_refusal_test.go`'s refused-before-the-gate move and
  `grantee_test.go`'s `TestAPlayersGrantIsRefusedByTheTableWhoeverItNames`, each
  now giving its player a party member of their own.
- **G6, the participant-id bound:** a minted id is far under the fold's bound,
  so no grant over the wire can name one at or over it;
  `qa_cmd_id_bound_test.go`'s
  `TestQACmdControlOverTheBoundIsRefusedOnTheWireAndAppendsNothing` expects
  VTT-895's text for its grant cases and the fold's for its revoke cases; the
  fold's bound stays held by `internal/engine` and
  `client/test/fold-rejections.test.ts` (D17).
- **G7, blueprint §16:** the owner's sentence (D21), in the specifications'
  commit.
- **D9, the toast's race:** when a window reads its new role before its new
  set arrives, VTT-606 withholds "You no longer play ..."; almost
  theoretical and small, so no code, no test and no debt (the owner's rule of
  2026-10-21).
- **D10, interleavings across connections:** every answer is a function of
  one snapshot, so a racing command can only read an older one, as every
  player rule already does; the cases that leave a state worth naming, among
  them a grant landing between a `remove_actor`'s snapshot and its batch,
  which leaves that grantee unannounced, are listed in the plan and get no
  code, no test and no debt.

## The deleted tests

Each was deleted because what it observes can no longer happen, its row
withdrawn or its cell updated in the same commit (CLAUDE.md rule 2).

- **`identity.SetRole` is gone** (Task 3, D5):
  `internal/identity/identity_test.go`'s
  `TestSetRolePromotesTheNamedParticipant`, `TestSetRoleLeavesEVERYONEElseAlone`,
  `TestSetRoleRejectsARoleThatIsNotOneOfTheFour`,
  `TestSetRoleOnSomeoneWhoDoesNotExistIsAnError`, `TestSetRoleToTheSameRoleIsFine`,
  `TestSetRoleDoesNotDisturbTheCredential`,
  `TestSetRoleOnARevokedParticipantStaysRevoked` and
  `TestLookupReflectsAPromotionImmediately` (VTT-029, VTT-030, VTT-037,
  VTT-066, VTT-067, VTT-068); the `SetRole` assertions of
  `TestTheDoorRefusesWhenTheDatabaseIsUnusable` and
  `TestTheIdentityStoreReportsFailuresRatherThanPretending` (VTT-070 stays).
- **A role that changes with nothing appended** (Task 3, D6):
  `seat_internal_test.go`'s
  `TestAPromotionAsTheSeatOpensSendsNoEntryOfTheShoulder`,
  `TestAnEntryTakenAfterAPromotionIsJudgedByTheNewRole` and
  `TestARoleReadAfterAdmitStillJudgesTheEntry` (VTT-387; VTT-348's tests hold
  the entry); `qa_promotion_rejudge_test.go`'s
  `TestQARejudgePromotedSpectatorOnAnothersPartyMemberMovesToTheirOwn`,
  `TestQARejudgePromotedSpectatorHoldingAPartyMemberWatchesItFromItsFirstEntry`,
  `TestQARejudgeBothConnectionsOfAPromotedSpectatorAreRejudged`,
  `TestQARejudgeSpectatorPromotedToPlayerKeepsAPerchItMayStillWatch` and
  `TestQARejudgePromotedSpectatorWhoControlsNoneKeepsItsPerch`; the whole of
  `qa_promotion_opening_test.go`
  (`TestQAOpeningAPromotedSpectatorOnAnothersPartyMemberEndsWatchingTheirOwn`,
  `TestQAOpeningAPromotedSpectatorHoldingAPartyMemberWatchesItFromItsFirstEntry`);
  `qa_promotion_queued_test.go`'s
  `TestQAQueuedPromotedSpectatorOnAnothersPartyMemberMovesToTheirOwn`,
  `TestQAQueuedPromotedSpectatorMovesToItsOwnFirstPartyMember` and
  `TestQAQueuedPromotedSpectatorOnItsOwnPartyMemberKeepsStreaming` (the grant
  tests of VTT-921 hold the move); `qa_watchable_test.go`'s "made a player who
  holds nothing" case.
- **A promotion appends nothing** (Task 3, VTT-036): `server_test.go`'s
  `TestAPromotionAppendsNoEvent`; `qa_promotion_revoked_test.go`'s
  `TestQAPromoRevokedAPromotionAppendsNoEvent`; the whole of
  `qa_joining_test.go`: `TestQAAPromotionLeavesTheLogHeadWhereItWas`, and
  `TestQATheHeadProbeSeesAnAppend`, whose one job was to make that test's
  silence evidence (its ledger row removed).
- **No path makes anyone a DM** (Task 3): `qa_ruleset_by_role_test.go`'s
  `TestQAReadAPlayerPromotedToDMReadsEveryAbilityAtTheNextRead` (VTT-693's
  cell).
- **A spectator who controls** (Task 3, D2): `qa_controls_none_test.go`'s
  `TestQACNASpectatorWhoControlsAPartyMemberIsToldSo` (VTT-454 held by
  `TestQACNAPlayerWhoControlsAPartyMemberIsToldSo`);
  `qa_ruleset_by_role_test.go`'s
  `TestQAReadASpectatorReadsNoAbilityEvenWhereItsIdControlsAnActor` (VTT-692
  held by `TestASpectatorReadsNoAbility`).
- **A player who controls nothing** (Task 3, D18): `viewpoint_test.go`'s
  `TestAPlayerWhoControlsNoPartyMemberMayWatchAnyPartyMemberOrNobody`
  (VTT-452); `qa_whisper_test.go`'s
  `TestQAWhAPlayerWhoControlsNothingCannotWhisper` (the spectator's refusal is
  `TestQAWhASpectatorCannotWhisperAtAll`); `qa_controls_none_test.go`'s
  `TestQACNAPlayerWhoControlsNothingIsToldTheyControlNone` (the spectator's
  case holds it); `watchable_test.go`'s
  `TestTheSetSaysWhetherTheParticipantControlsNoPartyMember`'s never-granted
  player; `watchable_abilities_test.go`'s
  `TestTheSetOfASpectatorCarriesNoAbilityIds`'s never-granted player.
- **A player holding an actor that is not a party member** (Tasks 2 and 3,
  D25): `qa_promotion_rejudge_test.go`'s
  `TestQARejudgeControllingOnlyANonPartyActorWatchesNobody` (VTT-340's cell);
  `qa_creature_test.go`'s `TestQACreatureATakeBackEndsTheRunAndPausesTheLog`'s
  VTT-350 move case and its "stays after revoking it" case; `watch_test.go`'s
  `TestAPlayerWithNoPartyMemberMovesNothing` (VTT-370);
  `qa_controls_none_test.go`'s
  `TestQACNAPlayerWhoControlsOnlyANonPartyActorIsToldTheyControlNone` and
  `TestQACNANonPartyGrantLeavesAPlayerControllingNone`;
  `qa_move_gate_test.go`'s
  `TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken`;
  `qa_ruleset_by_role_test.go`'s
  `TestQAReadANonPartyActorAddedAsSuchAndGrantedCounts`;
  `qa_ability_sight_test.go`'s
  `TestQAASAPlayerWhoControlsNoPartyMemberHasNoEyesToAimBy`;
  `qa_plain_refusal_test.go`'s
  `TestQAPRPlainAPlayerWhoRunsOnlyACreatureIsToldTheyCannotSee`;
  `watchable_test.go`'s
  `TestTheSetSaysWhetherTheParticipantControlsNoPartyMember`'s "a player who
  controls only a non-party actor" case.
- **A perched player** (Tasks 3 and 4, D14): `qa_move_gate_test.go`'s
  `TestQAMGAPerchedPlayerMovesNoToken`; `qa_player_perch_test.go`'s
  `TestQAPPAPlayerWithoutAPartyMemberWatchesAnyPartyMemberOrNobody`,
  `TestQAPPAPlayerWithoutAPartyMemberMovesNothingWhateverTheyWatch`,
  `TestQAPPPromotingASpectatorToAPlayerWithoutAPartyMemberKeepsThePerch` and
  `TestQAPPDemotingAPlayerToSpectatorKeepsThePerch` (VTT-923's tests hold
  what a demoted participant watches); in the client,
  `app.test.ts`'s "a perched player's board click sends nothing, on a listed
  party member's token or on an empty square" (VTT-461),
  `qa-door-mode.test.ts`'s "VTT-461: a perching player's click on a closed
  door, an open door or a plain square sends nothing", and
  `qa-perched-player.test.ts`'s "qaNP: No shoulder in a player's perch sends
  set_viewpoint naming nobody" (VTT-459), "qaNP: a shoulder in a player's
  perch sends set_viewpoint naming it, the watched one and one the log has
  not shown included", "qaNP: a perched player's perch shows as watched the
  actor each ViewpointChanged names, not a command's result" and "qaNP: a
  perched player whose set names no party member is offered only nobody, and
  no player panel" (VTT-458).
- **The perch's line** (Task 4, VTT-607): `spectator-view.test.ts`'s "the
  perch says you play no character when told to, and nothing when not";
  `app.test.ts`'s "a player who controls no party member is told so on the
  perch, and a spectator is not"; `qa-refusal-words.test.ts`'s "the perch
  says the window plays no character only when told to". The
  `qa-refusal-words` tests that asserted the line beside their own subject
  lost that assertion alone.
- **The flag left the ruleset key** (Task 4): `qa-window-reread.test.ts`'s
  "a set whose flag alone changes issues one new read".
- **A grant at the participant-id bound** (Task 2, D17, G6):
  `qa_cmd_id_bound_test.go`'s `TestQACmdControlAtTheBoundIsRecordedWhole`'s
  grant half, and `server_test.go`'s
  `TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing`'s
  accepted grant at the cap, whose `grant-cap` case now expects `There is no
  such participant.`
- **A minted player never granted** (Task 5): the player case of
  `qa_promotion_revoked_test.go`'s
  `TestQAPromoRevokedARevokedWatcherIsNotSentTheGrant`'s role loop, which
  equalled its spectator case once identity minted no player; VTT-032 for a
  granted player is held by `TestRevokingRemovesSomebodyWhoIsStillConnected`.

## The tests rewritten under a new name

Each in this list keeps what it observed, reached by a grant or by "Make
spectator" instead of a promotion, or by a participant who controls nothing
instead of a player who controls nothing; the tests whose assertion was
inverted follow it, under their own heading:

- `seat_internal_test.go`: `TestAPromotionAsTheSeatOpensMovesItOntoTheirPartyMember`
  → `TestAGrantAsTheSeatOpensMovesItOntoTheirPartyMember`;
  `TestWatchableNamesEveryPartyMemberForAPlayerWhoControlsNone` →
  `TestWatchableNamesEveryPartyMemberForAParticipantWhoControlsNothing`.
- `watch_test.go`: `TestAPromotedSpectatorWatchesTheirFirstPartyMember` →
  `TestAGrantedSpectatorWatchesTheirFirstPartyMember`;
  `TestAPromotionMovesASpectatorOffAShoulderThePlayerMayNotWatch` →
  `TestAGrantMovesASpectatorOffAShoulderThePlayerMayNotWatch`;
  `TestAPromotedSpectatorMovesOntoThePartyMemberTheyHold` →
  `TestAGrantOfAnActorAlreadyAddedMovesASpectatorOntoIt`;
  `TestAPromotionMovesEveryConnectionOfTheParticipant` →
  `TestAGrantMovesEveryConnectionOfTheParticipant`;
  `TestAPromotedSpectatorKeepsWatchingAPartyMemberTheyHold` →
  `TestAGrantedSpectatorKeepsWatchingThePartyMemberTheyAreGiven`.
- `watchable_test.go`: `TestAPromotionTellsTheSeatTheNewRolesSet` →
  `TestAGrantTellsTheSeatTheNewRolesSet`;
  `TestAPlayerWhoControlsNoPartyMemberPerchesAndReopensOnIt` →
  `TestAParticipantWhoControlsNothingPerchesAndReopensOnIt`;
  `TestAPerchedPlayerGrantedAPartyMemberMovesToTheFirst` →
  `TestAPerchedSpectatorGrantedAPartyMemberMovesToTheFirst`.
- `watchable_abilities_test.go`:
  `TestASpectatorWhoControlsAPartyMemberIsSentItsAbilityIds` →
  `TestASpectatorGrantedAPartyMemberIsSentItsAbilityIds`;
  `TestTheSetsOfASpectatorAndAPlayerWhoControlNoPartyMemberCarryNoAbilityIds` →
  `TestTheSetOfASpectatorCarriesNoAbilityIds`.
- `server_test.go`: `TestDMPromotesASpectatorOverTheWire` →
  `TestDMMakesAPlayerASpectatorOverTheWire`;
  `TestAPromotionBitesWithoutReconnecting` → `TestAGrantBitesWithoutReconnecting`.
- `authz_test.go`: `TestPromotionMayOnlyTargetPlayerOrSpectator` →
  `TestPromotionMayOnlyMakeASpectator`.
- `qa_watchable_test.go`: `TestQAWatchableAPromotionChangesTheSet` →
  `TestQAWatchableAGrantAndAMakeSpectatorChangeTheSet`.
- `qa_controls_none_test.go`:
  `TestQACNGrantingTheOnlyPartyMemberToAPlayerWhoControlsNoneSendsTheSetAgain`
  →
  `TestQACNGrantingTheOnlyPartyMemberToAParticipantWhoControlsNoneSendsTheSetAgain`;
  `TestQACNARegrantMakingTheirNonPartyActorAPartyMemberSaysTheyControlOne` →
  `TestQACNAPartyMemberGrantOfANonPartyActorSaysTheyControlOne`.
- `qa_promotion_rejudge_test.go`:
  `TestQARejudgePlayerPromotedToSpectatorKeepsItsPartyMemberLog` →
  `TestQARejudgeAPlayerMadeASpectatorKeepsItsPartyMemberLog`.
- `qa_promotion_revoked_test.go`: each announcement test's trigger is a
  grant:
  - `TestQAPromoRevokedARevokedWatcherIsNotSentThePromotion` →
    `TestQAPromoRevokedARevokedWatcherIsNotSentTheGrant`;
  - `TestQAPromoRevokedNoDeviceOfARevokedParticipantIsSentThePromotion` →
    `TestQAPromoRevokedNoDeviceOfARevokedParticipantIsSentTheGrant`;
  - `TestQAPromoRevokedARevocationBetweenTwoPromotionsWithholdsOnlyTheSecond` →
    `TestQAPromoRevokedARevocationBetweenTwoControlChangesWithholdsOnlyTheSecond`;
  - `TestQAPromoRevokedTheNextEventClosesARevokedWatcherWhoWasSentNoPromotion` →
    `TestQAPromoRevokedTheNextEventClosesARevokedWatcherWhoWasSentNoGrant`;
  - `TestQAPromoRevokedThePromotedParticipantIsToldOnTheirOwnConnection` →
    `TestQAPromoRevokedTheGrantedParticipantIsToldOnTheirOwnConnection`;
  - `TestQAPromoRevokedEveryUnrevokedConnectionIsSentThePromotion` →
    `TestQAPromoRevokedEveryUnrevokedConnectionIsSentTheGrant`;
  - `TestQAPromoRevokedPromotingAnAbsentParticipantAnnouncesNothing` →
    `TestQAPromoRevokedGrantingAnAbsentParticipantAnnouncesNothing`.
- `qa_player_perch_test.go`:
  `TestQAPPAPlayerWithoutAPartyMemberOpensOnTheLogItNames` →
  `TestQAPPAParticipantWhoControlsNothingOpensOnTheLogItNames`;
  `TestQAPPAPlayerWithoutAPartyMemberIsToldEveryPartyMember` →
  `TestQAPPAParticipantWhoControlsNothingIsToldEveryPartyMember`;
  `TestQAPPAWatchedActorThatLeavesThePartyMovesThePlayerToNobody` →
  `...MovesTheWatcherToNobody`;
  `TestQAPPAPlayerWatchingAnotherPartyMemberStaysAsItChangesHands` →
  `TestQAPPASpectatorWatchingAPartyMemberStaysAsItChangesHands`.
- `qa_seat_test.go`: `TestQASeatAPlayerWithNoPartyMemberWatchesNobodyUntilGranted`
  → `TestQASeatAParticipantWhoControlsNothingWatchesNobodyUntilGranted`.
- `qa_door_square_test.go`:
  `TestQADSAPlayerWhoControlsNothingGetsOneTextAtEverySquare` →
  `TestQADSAPlayerWhosePartyMemberHasNoTokenGetsOneTextAtEverySquare`.
- `qa_whisper_test.go`:
  `TestQAWhAPartyMemberRegrantedAsACreatureNeitherHearsNorWhispers` →
  `TestQAWhAPartyMemberTakenBackAsACreatureNeitherHearsNorWhispers`.
- `qa_creature_test.go`: `TestQACreatureANonPartyGrantEndsTheRun` →
  `TestQACreatureATakeBackEndsTheRunAndPausesTheLog`.
- `qa_cmd_id_bound_test.go`:
  `TestQACmdControlOverTheBoundIsRefusedOnTheWireWithTheFoldsText` →
  `...OnTheWireAndAppendsNothing`.
- `qa_role_from_log_test.go` (QA's, renamed in Task 5):
  `TestQARLAParticipantMintedAsAPlayerIsASpectatorUntilGivenACharacter` →
  `TestQARLAParticipantStoredAsAPlayerIsASpectatorUntilGivenACharacter`.
- The client: `joining.spec.ts`'s "... is promoted, and plays" → "... is
  given a character, and plays"; `app.test.ts`'s "a promoted spectator's own
  screen catches up without reconnecting" → "a granted spectator's ...", and
  "a player's window reads the ruleset again when its watchable set or flag
  changes, and only then" → "... when its watchable set changes, and only
  then"; `commands.test.ts`'s "promotion names the participant and the role"
  → "promotion names the participant and spectator, the one role it may
  name"; `qa-window-reread.test.ts`'s promotion test → "a spectator's window
  re-read as a player at its re-announcement reads the ruleset again", and
  "a perched player granted a character is offered its abilities once the
  re-read answers" → "a player whose set names no party member, granted a
  character, is offered its abilities once the re-read answers";
  `qa-door-mode.test.ts`'s "VTT-884 VTT-460: a player's click on an open
  door square holding a listed party member's token switches to them, as
  the same click does where no door is" → the same name citing VTT-919.

Beyond the renames, every test the plan's "The existing tests that change"
names was rewritten as it says: player literals given a party member of
their own, spectator literals controlling nothing, non-party actors held by
the DM or taken back in D25's order, and every test mint of a player made a
spectator (Task 5), with the assertions that read identity's stored role
now expecting `spectator`.

## The tests whose assertion was inverted

Each now asserts the opposite of what it asserted on `c99715b`, by the
decision named:

- `qa_controls_none_test.go`'s
  `TestQACNMakingAPlayerASpectatorKeepsSayingTheyControlOne` →
  `TestQACNMakingAPlayerASpectatorSaysTheyControlNone`: "Make spectator"
  takes every actor (D3).
- `dm-view.test.ts`'s "a spectator can be promoted, and a player is not
  offered promotion again" → "everyone is shown, a player can be made a
  spectator, and nobody is offered promotion to player": there is no "Make
  player" (D3, item 7).
- No player perches (D14), in `app.test.ts`: "a player whose set says they
  control no party member is offered the perch, and no player panel" → "a
  player's window sent a set saying it controls no party member draws the
  player panel and no perch", and "the set's flag moves a player between the
  perch and the player panel" → "the set's flag leaves a player's window on
  the player panel".
- No player perches (D14), in `qa-perched-player.test.ts`:
  - "qaNP: a player whose set says they control no party member draws the
    perch with exactly the set and nobody, and no player panel" → "qaNP: a
    player whose set says they control no party member draws the player
    panel with exactly the set, and no perch";
  - "qaNP: a perched player's board click sends nothing, where a listed
    player's same click switches or moves" → "qaNP: a player whose set says
    they control no party member switches or moves by a board click as a
    listed player does";
  - "qaNP: a set sent before /api/me answers player still draws the perch
    once it does" → "qaNP: a set saying no party member, sent before /api/me
    answers player, draws the player panel with exactly the set once it
    does";
  - "qaNP: a spectator re-read as player keeps the perch while the set says
    no party member, and gets the panel once it does not" → "qaNP: a
    spectator re-read as player leaves the perch for the player panel at the
    re-read, whatever its set says";
  - "qaNP: after a log that cannot fold, a discard keeps the set, so the
    player is drawn the perch again" → "qaNP: after a log that cannot fold, a
    discard keeps the set, so the player is drawn the player panel with it
    again";
  - "qaNP: each set's flag moves a player between the perch and the player
    panel" → "qaNP: each set's flag leaves a player on the player panel,
    offered exactly that set".
- `qa-refusal-words.test.ts`'s "the perch tells a player they play none, not
  a spectator" → "a spectator's perch offers the set whether or not it says
  they control none": the perch's line goes (D14, VTT-607).

## Phase 4a's adjudications

- Task 1, finding 1: spec ambiguity — VTT-906 says neither "exactly one" nor
  "nobody else named"; QA asserted exactly one per named participant and no
  other presence frame, D8's reading — nothing changed; QA's tests kept.
- Task 1, finding 2: needs an id — a refused grant, revocation or
  `remove_actor` sends no `PresenceChanged` — VTT-924 allocated, QA's
  refusal tests re-cited to it.
- Task 1, finding 3: spec gap — a revoked named participant still
  connected, the moment "connected" is judged, the frame's order against the
  result — near-theoretical, or SPEC-011's "do not rely on order" — nothing
  changed.
- Task 1, finding 4: spec gap — a spectator revoking themselves — refused
  before any announcement, a spectator issuing `set_viewpoint` alone
  (VTT-139) — nothing changed.
- Task 1, note: `TestQARARevokedWatcherIsNotSentTheReannouncement` is weak
  evidence, an append closing the revoked watcher's connection first — the
  deny set is held by `TestQAPromoRevokedARevokedWatcherIsNotSentTheGrant`
  and, after the review, `TestAReannouncementIsNeverQueuedForARevokedWatcher`
  — nothing changed in QA's test.
- Task 2, finding 1: spec ambiguity — VTT-914 does not say "whom identity
  still holds" — nothing changed at Phase 4a; Phase 4b withdrew VTT-914 and
  VTT-916 for the same gap, succeeded by VTT-926 and VTT-927.
- Task 2, finding 2: spec ambiguity — VTT-895's and VTT-913's antecedents,
  read literally, overlap the form check's and the table's refusals — D7's
  order decides, stated after the review as VTT-928 — nothing changed in
  code.
- Task 2, finding 3: spec ambiguity — VTT-147's "may grant control of an
  actor a player controls" — still true for a party-member grant; VTT-913
  narrows the kind — nothing changed.
- Task 2, finding 4: spec gaps — the kindless grant's text, an empty id,
  identity before the fold's unknown-actor refusal, "the DM" as identity's
  role — VTT-925's form check, an id identity does not hold, D7's order, and
  identity's role respectively — nothing changed.
- Task 2, needs an id: a player's grant gets the table's text whatever it
  names, and the form check comes first — held by D7's order and the
  existing authorization rows — no new row.
- Task 3, finding 1: "of any kind" unreachable over the wire after D25 —
  known (D25) — nothing changed.
- Task 3, finding 2: no re-announcement for a promotion that appended
  nothing — VTT-906 says "that appended" — nothing changed.
- Task 3, finding 3: the universal claim of rule 1 — held by reading (Phase
  4b) — nothing changed.
- Task 3, finding 4: a poisoned `/api/me` unreachable through the surface —
  nothing changed.
- Task 3, finding 5: VTT-922 needs a scene — held by the existing seat and
  move tests — nothing changed.
- Task 3, finding 6: a revoked watcher with an open connection cannot be
  built over the wire — held by `reannounce_internal_test.go` — nothing
  changed.
- Task 3, needs an id: a shared actor's other controllers keep it — the
  fold's revoke arm, a candidate the sort refused — no new row.
- Task 3, needs an id: "Make spectator"'s batch against VTT-923 — the
  batch's last event ends the last actor's control, so VTT-923 covers it —
  no new row.
- Task 4, findings 1 and 2: the question's tie order and string comparison
  unobservable or unstated — UTF-16 order, as the console's other sorts —
  nothing changed.
- Task 4, finding 3: is a refused promotion "answered" — yes, the roster is
  read again — nothing changed.
- Task 4, finding 4: a declined question reads nothing — consistent with
  VTT-904 — nothing changed.
- Task 4, finding 5: a set whose flag alone changes no longer reads the
  ruleset — by design, the flag having left the key (Deviations) — nothing
  changed.
- Task 4, finding 6: VTT-901's "display name" is the roster's name, the
  console's only name for a row — nothing changed.
- Task 4, finding 7: SPEC-022 still describes the player's perch — the
  specifications' commit — nothing changed in code.
- Task 4, needs an id: the positive row controls, the absent line, the panel
  side of VTT-917 — held by VTT-901 to VTT-905, VTT-917, and the refused
  candidate for the line — no new rows.
- Task 4, defect outside the ticket: a failed presence-triggered `/api/me`
  re-read surfaces as an unhandled promise rejection; the window keeps the
  role it last read, as SPEC-022 says — left out, for the owner's backlog.
- Task 5, finding 1: the refusal of a player comes before a held name's —
  recorded, no row.
- Task 5, finding 2: the words of other refused roles are not promised —
  nothing changed.
- Task 5, finding 3: a refused `vtt invite` still creates the campaign file,
  with no participant row — the store's opening, older than this ticket —
  nothing changed.
- Task 5, finding 4: SPEC-009 still names `SetRole` — the specifications'
  commit; `identity.Participant`'s doc reworded to say the gateway judges no
  player or spectator by `Role`.
- Tasks 0, 6 and 7 had no Phase 4a: Task 0 is records, and see Deviations
  for Tasks 6 and 7.

## Phase 4b

- **Task 0:** the review named the scenarios and tests D25 reaches
  (`scenarios/door-watch.json`, `scenarios/shared-control.json`,
  `creature_test.go`, `qa_creature_test.go` and others), which the plan's
  Task 2 amendment took in.
- **Task 1:** no Must beyond `gocyclo` on `handleCommand`, met by
  `reannounceControlChange`. No test held VTT-924's revocation clause;
  `TestARevocationTheFoldRefusesAnnouncesNothing` was added. The wire tests
  caught a broken deny set only sometimes; the internal
  `TestAReannouncementIsNeverQueuedForARevokedWatcher` was added. A grant
  landing between a `remove_actor`'s snapshot and its batch goes unannounced;
  it joined D10.
- **Task 2:** VTT-914 and VTT-916 counted a revoked controller; withdrawn,
  succeeded by VTT-926 and VTT-927. The state-level tests building a player
  with a non-party actor by appending events directly moved to Task 3.
  Identity's other errors were untested; `grantee_internal_test.go` was
  added. D7's order became VTT-928.
- **Task 3:** D3's placement: the refusal of a promotion to player was
  first put in `handlePromotion`, on a belief that lint (`staticcheck`'s
  ST1005) refused a capitalised error ending in a full stop from
  `Authorize`; the review moved it to `authorizePromotionTarget`, as
  planned, through `plainRefusal`, which already carries plain-words
  refusals out of `Authorize` and passes lint, and
  `TestPromotionMayOnlyTargetPlayerOrSpectator` became
  `TestPromotionMayOnlyMakeASpectator`; `convert_test.go`'s reason for
  `promote_participant` rewritten; the serve loop's comment re-pointed to
  VTT-032; `TestPromotionCannotUNMAKEADMOrAgent`'s header rewritten and a
  dead identity-role check dropped.
- **Task 4:** `DMDeps.send`'s doc and the comment above `act` said role
  commands append no event, and were rewritten; the question's tie on id
  could not be observed, and the shown names now sort as they read; the
  roster did not follow an offline participant's grant or revocation
  (VTT-930, Deviations).
- **Task 5:** no Must. `mintInvites`' comment says only a grant makes a
  player (SPEC-009); `client/e2e/setup.ts`'s comments rewritten;
  `qa_art_root_test.go`'s `TestQAArtRootArtIsAnsweredToEveryRole`'s player case
  restored as a spectator granted an actor; `scenarios/adventure-night.json`
  found (left out).
- **Task 6:** no Must. VTT-931 allocated; `ActorKind`'s pointer is VTT-913;
  `SetViewpoint`'s comment says what a player may not ask for;
  `cmd/vtt/joinlink.go`'s "the DM promotes from there" rewritten.
- **Task 7:** no Must. The "no reload" claim is held by a socket count;
  `joining.spec.ts`'s "spec §3.1" citation became VTT-889 and VTT-890; the
  new handover test asserts that Lera held both actors first.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The verifier's OD-4 A: the perch's line goes, and a player holding only actors that are not party members perches as a spectator does | D25: a grant naming a participant who is neither the DM nor the agent with another kind, or making such a participant's actor non-party, is refused; D14: no player perches; item 12 and VTT-913 to VTT-923 follow; VTT-340, VTT-370, VTT-452, VTT-453, VTT-456 to VTT-461 and VTT-696 withdrawn | The owner answered none of A, B or C: a player with a creature is a player, and, asked what such a player watches through, chose that the creature is a party member (2026-11-20 and 2026-11-21) |
| R14: the question names the actors in actor-id order | In the order of what each is shown by, its name or else its id (VTT-902) | The owner's answer of 2026-11-21 |
| D11: ties in that order broken on id | `renderDMConsole` sorts the shown names as they read; VTT-902 keeps "ties broken on id" | Task 4's review: two actors shown by the same text read the same either way, so the tie arm had no observation and its mutants could not be killed |
| D25's refusals in words to be chosen | `<name> can only be given a party member.` and `<names> control(s) <actor>: revoke it from them before you make it a non-party actor.` (D26) | Proposed with D25 and chosen by the owner on 2026-11-21 |
| D7: VTT-151 and `scenarios/denials.json` keep their texts | VTT-151 withdrawn, succeeded by VTT-925 (the form check); `scenarios/denials.json` unchanged | D25 made VTT-151's "a grant stating any kind ... is accepted" false; its tests hold the form check alone |
| VTT-914 and VTT-916 as allocated at Task 0 | Withdrawn at Task 2, succeeded by VTT-926 and VTT-927 | Their words counted a controller identity holds revoked, whom the code neither counts nor names |
| VTT-067 and VTT-068 standing | Withdrawn at Task 3; VTT-067 succeeded by VTT-929 for `identity.Revoke` | VTT-067's sentence named `SetRole`, which is gone; its `Revoke` half, held by `identity_failure_test.go`'s `TestRevokeUnknownParticipantErrors`, moved to VTT-929. VTT-068 was held only by a `SetRole` test, deleted with `SetRole` |
| VTT-389 succeeded by VTT-908 and VTT-909 | Succeeded by VTT-908 and VTT-931 | Task 6's review: VTT-909 names the revoke tool while the test of the grant's `kind` description cited it, so the take-back sentence there got its own row |
| No rows beyond VTT-889 to VTT-923 | VTT-924 (Task 1, Phase 4a), VTT-925 to VTT-928 (Task 2 and its review), VTT-929 (Task 3), VTT-930 (Task 4's review), VTT-931 (Task 6's review) | Each is a rule the work met that no allocated row stated: a refusal's silence, the form check, the revoked controller, D7's order, `Revoke`'s missing participant, the roster re-read, the grant's kind text |
| D8: `reannounce` called in `handleCommand` after a grant's and a revocation's append | Called through `reannounceControlChange` | `gocyclo` refused `handleCommand`'s complexity with the calls inline (Task 1's review) |
| D7: the grantee step in `server.go` beside `handlePromotion` | `granteeRefusal` in its own file, `internal/gateway/grantee.go`, called from `handleCommand` with one branch | The same `gocyclo` bound on `handleCommand`, and the step reads identity for several refusals |
| D13: the window needs no new code to follow its role | `watchKey` (`client/src/app.ts`) no longer holds the set's `controlsNoPartyMember`; "a set whose flag alone changes issues one new read" deleted; the VTT-712 promotion test rewritten as a re-read at the re-announcement | Under D25 a player's flag is always false, so a key holding it changed for no player's window; VTT-920, which succeeds VTT-696, names the set's actors alone |
| D8: the DM's roster follows a role by the presence frame | The console also reads the roster again when its grant, revocation or removal of an actor is answered (`Sending.rereads` in `client/src/view/dm.ts`, VTT-930) | A participant with no connection is announced to nobody (VTT-907), so their role cell kept its old role until a reload (Task 4's review) |
| Task 2's amendment: the scenarios D25 reaches rewritten in the take-back order or with a party-member grant | `scenarios/door-watch.json` and `scenarios/shared-control.json` grant their non-party actor to the DM, and drop the player participant that held it; the goldens' participant id and controller ids follow | A party-member grant would give that player a character log of an actor out of sight and change what door-watch is about |
| Task 2 rewrites every test D25 breaks | The state-level tests that build a player holding a non-party actor by appending events directly moved to Task 3 | They stay green past the wire, so Task 2's red run did not find them; Task 2's review did |
| P8: every task gets Phase 4a's QA | None for Task 6 or Task 7 | Task 6 changes prose (tool descriptions and proto comments) pinned by toolgen's description tests, with no behaviour to derive; Task 7's e2e are themselves the observation of items 6 and 8, written against the rows |
| Task 6 true when `go test ./tools/toolgen/ ./internal/mcp/ ./contract/...` passes | Task 6 changed `place_token`'s name text, which `qa_describe_token_test.go#TestQADTPlaceTokenToolDocsTheFourTexts` pinned; the gate's first run went red there, and `9a86ccc` makes the test assert the shipped text | That test is in `internal/gateway`, which Task 6's checks did not run and which tier 1 (`task test:unit`, the pre-commit hook's tests) leaves out |

## What could not be established

- **"Of any kind"** in rule 1: `roleIn` counts any actor, but under D25 no
  state reached over the wire, save D10's interleavings and a campaign file
  written before this ticket, tells that from a count of party members, so no
  test holds it; it is held by reading.
- **"Every decision"** in rule 1 is held per surface and by the search in
  item 4; no single test reaches every reader of the role.
- **A seat opened by a stored player row:**
  `TestQARLAParticipantStoredAsAPlayerIsASpectatorUntilGivenACharacter`
  holds the reads, the commands and what such a seat may watch, and
  `roleIn`'s own test the judgement, but no test observes the viewpoint such
  a seat opens on (`defaultViewpoint`).
- **`controlRevocations`' actor-id order:** Task 3's break that dropped the
  sort turned its test red by the map's iteration order, so a run can miss
  it; recorded, not strengthened.
- **Task 5's deleted subtests:** its commit message speaks of ungranted
  "player" subtests, in the plural, that duplicated their spectator case.
  Its diff shows the player case of
  `TestQAPromoRevokedARevokedWatcherIsNotSentTheGrant`'s role loop deleted,
  and `TestQAArtRootArtIsAnsweredToEveryRole`'s player case, deleted and
  then restored on review as a spectator granted an actor; the commit
  message's plural counts the deletion before the review.
- **Template-literal tests:** `qa-console-perch.test.ts`'s role loop for
  VTT-917 and its flag loop for VTT-919 are named with template literals,
  which the requirements chain cannot cite, so those rows do not name them.
- **`handover.spec.ts`'s second test** asserts a toast whose arrival rests
  on D9's race: the set before the role. Whether a loaded machine can
  withhold it was not measured.
- **A grant racing a revocation** (the ticket's last open question):
  commands are serialised per connection, not across connections (M9); D10
  names what an interleaving leaves, and no test forces one.
- **The client's mutation run:** the stored report measured `c99715b`, so
  nothing this ticket changed in `client/src` has been mutated; a full run is
  the owner's to ask for (SPEC-023).

## What was deliberately left out

- **The ticket's "What this is not":** the DM's and the agent's role, still
  identity's; what a spectator may watch or issue (VTT-139, SPEC-015); who
  may grant or revoke; a migration (an old file's stored `player` is judged
  by the log); removal beyond what the derivation makes of it.
- **The specifications and blueprint §16:** SPEC-007, SPEC-009, SPEC-011,
  SPEC-012, SPEC-013, SPEC-015, SPEC-017 and SPEC-022, SPEC-021 checked,
  and §16's sentence (D21). Destination: their own commit, written from
  this report (the plan's Task 9).
- **`scenarios/adventure-night.json`:** its "player" participant is never
  granted, so it is a spectator for the whole run and its first step repeats
  its second; a grant would move its goldens. Destination: the owner's
  backlog, raised with this report.
- **A failed presence-triggered `/api/me` re-read** surfaces as an unhandled
  promise rejection (`onPresence`'s read in `client/src/app.ts` has no
  catch); older than this ticket, found by Task 4's QA. Destination: the
  owner's backlog, raised with this report.
- **`internal/testdb/testdb.go`'s header example** ("a promotion the DM
  believes happened") is stale; touching its over-bound block fails
  `check:comments`. Destination: the owner's backlog, raised with this
  report.
- **`handover.spec.ts`'s second test** keeps its title, "the DM takes a
  player's only character back, and the player chooses another shoulder",
  though the participant is a spectator once it is taken back; renaming it
  moves its citations across several rows' cells. Kept as a fixture label.
- **`internal/mcp`'s history comments** ("could promote a participant only
  by supplying an id") stay: still true, and in over-bound blocks.
- **`Session.controlsNoPartyMember`** stays: tests read it, and the flag
  still decides which party members VTT-605's toast counts as controlled.
- **By decision, no code, no test and no debt:** D9's toast race; D10's
  interleavings; a roster a frame behind the log (D11); a `PresenceChanged`
  after a grant that changed nobody's role (D8, by design); a player left
  without a party member by an interleaving or an old file, who watches
  nobody and so fails closed (D25).
- **The client's full mutation run:** on the owner's word (SPEC-023).
