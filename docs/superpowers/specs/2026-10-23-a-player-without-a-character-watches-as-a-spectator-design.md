# A player who controls no party member watches as a spectator does

## The problem

A player who controls no party member watches nobody and may choose nobody
else. `MayWatch` in `internal/gateway/viewpoint.go` accepts, for
`identity.RolePlayer`, only a party member whose controllers include the
player, and refuses the empty id ("a player watches one of their own party
members"). So `watchable` names no one for such a player, `defaultViewpoint`
gives them nobody, and a `set_viewpoint` naming any party member, or nobody,
is refused. In the browser, `paint` in `startSession` (`client/src/app.ts`)
offers the perch only where `mayPerch` holds, which is the role `spectator`;
a player is drawn `renderPlayerPanel` (`client/src/view/player.ts`), which for
an empty set reads "You do not control an actor yet." and offers nothing. So a
player whose character the DM has revoked, or who has joined and not yet been
granted one, sees no board and has nothing to do, while a
spectator at the same table may watch any party member. The owner ruled on
2026-10-22 (ruling B-4 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`) that "a player whose
character the DM has taken watches as a spectator does": the role stays
`player`, and what changes is whom they may watch while they control no party
member.

Three things hang on the present rule and must not follow the change. The move
gate in `handleCommand` (`internal/gateway/server.go`) judges a player's move
by `seat.judgedBy`, which answers the viewpoint the seat was told while
`MayWatch` accepts it. Today that is never a party member the player does not
control, so a player who controls no party member has no eyes and moves no
token (VTT-370), a token of a `NON_PARTY` actor they still control included
(`TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken` in
`internal/gateway/qa_move_gate_test.go`). A player who controls a party member
watches only their own (VTT-343, VTT-346). And the refusal of a named actor is
one string whether it exists, is a party member, or is someone else's
(VTT-347).

## Done looks like

1. In `internal/gateway`, a test shows that `MayWatch` accepts, for a player
   who controls no party member, every party member and the empty id, and
   refuses a non-party or absent actor with the same string it gives a
   spectator; it fails on today's tree. A second shows that `watchable` names
   every party member, each with its name, for such a player; it fails today.
2. A gateway test over a live connection: a player who controls no party
   member is sent a `WatchableChanged` naming every party member, sends
   `set_viewpoint` naming one, is answered ok, and is sent a
   `ViewpointChanged` naming it followed by that party member's log; a
   connection whose `log` names it opens on it, so a reload keeps the
   shoulder (VTT-442, carried to such a player). It fails today at the
   refusal.
3. Still refused, and still green: a player who controls a party member is
   refused another player's party member and nobody
   (`TestAPlayerMayWatchOnlyThePartyMembersTheyControl`,
   `TestAPlayerMayNotWatchNobody` in `internal/gateway/viewpoint_test.go`), and
   a refusal of a named actor still gives one string
   (`TestAPerchRefusalDoesNotSayWhetherTheActorExists`).
4. A player who controls no party member moves no token while their
   connection shows another player's party member, a `NON_PARTY` actor's token
   they control included: a gateway test perches such a player and then has
   every move refused as unseen. `TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken`
   (`qa_move_gate_test.go`), whose `log=pc-c` case expects such a connection to
   open on nobody, changes to expect `pc-c` and keeps every refusal; and
   `TestQAMGLosingTheOnlyPartyMemberLeavesNoTokenMovable` changes to expect the
   player to stay on `pc-b` after the revocation, with every move of the pet's
   token still refused.
5. A player watching another player's party member who is then granted a
   party member of their own moves to their first party member by id, and is
   sent a `WatchableChanged` naming only theirs: a gateway test, which fails
   today at the perch.
6. A player watching their only party member when the DM revokes it keeps
   watching that party member (the owner's answer of 2026-10-23, "Stay on
   Tobbe"), and is sent a `WatchableChanged` that says they control no party
   member, before any entry of the revocation, even at a table where that
   party member is the only one and so whom they may watch does not change: a
   gateway test, which fails today, where the revocation moves them to nobody.
7. `WatchableChanged` carries whether the participant controls no party
   member (the owner's answer of 2026-10-23, "Field on the list": an additive
   field), true for a player who controls none and false for one who controls
   any; a change in it alone is enough for the set to be sent again. Gateway
   tests, and the contract's round trip, that fail today.
8. The existing gateway tests whose expectation is today's rule change to
   the new one and keep what they were written to hold. A search
   (`grep -n 'func Test' internal/gateway/*_test.go | grep -iE
   'revok|nobody'`, read one by one) and a prototype of the first three rules
   that ran the whole Go tree (23 tests red, all in `internal/gateway`) find,
   besides the two in item 4:
   `TestQACreatureSetViewpointNeedsAPartyMemberTheyControl`
   (`qa_creature_test.go`);
   `TestQARejudgePromotedSpectatorOnAnothersPartyMemberWatchesNobody`
   (`qa_promotion_rejudge_test.go`);
   `TestQAQueuedPromotedSpectatorOnAnothersPartyMemberMovesToNobody`
   (`qa_promotion_queued_test.go`);
   `TestQAOpeningAPromotedSpectatorOnAnothersPartyMemberEndsWatchingNobody`
   (`qa_promotion_opening_test.go`);
   `TestQAWatchableARevocationChangesOnlyTheRevokedPlayersSet`
   (`qa_watchable_test.go`); `TestNoEntryCausedByTheLossIsSent` and three more
   in `seat_internal_test.go`; and the "control revoked" case of
   `TestAConnectionStopsWatchingAnActorItMayNoLongerWatch` (`watch_test.go`).
   The plan names all 23. A promoted spectator who controls no party member is
   now a player who may stay where they perched, so the promotion tests that
   hold "a seat the new role may not watch moves before any entry" (VTT-387)
   must give the promoted player a party member of their own to keep holding
   it; and the tests that hold VTT-348 by revoking a player's only party
   member must give the player a second one of their own, since the
   revocation no longer takes the right to watch away.
9. In the browser, a window whose client last read the role `player` and was
   last sent a `WatchableChanged` that says they control no party member,
   except while it shows the reason it cannot fold its log, offers what a
   spectator is offered: the perch ("Whose eyes"), a button for each party
   member that set names and "No shoulder", and not the player panel's chips
   or abilities; a click on a button sends
   `set_viewpoint` naming it, and a board click sends nothing (the owner's
   ruling "Bara knappar", 2026-10-20). Tests in `client/test/app.test.ts`
   that fail on today's tree.
10. A player who controls a party member keeps the player panel: "a player
    is offered exactly the party members the set names" and "a player
    switches by a button and by a click on the token" in
    `client/test/app.test.ts` stay green.
11. In `client/e2e/handover.spec.ts`, a browser test: the DM revokes a
    player's only character with the console's Revoke button, and the player
    then chooses another party member's shoulder on the perch and is shown
    that log. It fails on today's bundle. "a DM hands a second character
    over, and the player keeps both across a reconnect", whose beat 2 expects
    a player holding nothing to read "You do not control an actor yet.",
    asserts the perch there instead and stays green.
12. `set_viewpoint`'s tool description in `tools/toolgen/main.go`, and
    `SetViewpoint`'s comment in `contract/vtt/v1/commands.proto`, no longer
    say a player may watch only a party member they control or that only a
    spectator may ask for nobody; `check:drift` stays green with the
    regenerated `tools.json`, and `TestToolsMatchGolden` with the hand-kept
    `contract/testdata/expected_tools.json`.

## Rules this puts on the system

- A player who controls no party member may watch any party member, or
  nobody.
- A player who controls a party member may watch only the party members they
  control, and not nobody.
- A player's move is judged only by the eyes of a party member they control;
  a player who controls none moves no token, whatever their connection shows.
- A player's or a spectator's connection is told whether its participant
  controls no party member, with whom it may watch.
- The browser offers a player who controls no party member the perch a
  spectator is offered, and no player panel.

Existing rules and records this makes false as worded, each withdrawn and
succeeded, never edited: VTT-343 ("A player's `set_viewpoint` is accepted
only for a party member they control"), VTT-346 ("A player's `set_viewpoint`
naming no actor is refused"), VTT-351 ("A player's move is judged by the eyes
of the character their connection shows or, …"), VTT-402 ("A player is
offered a switch button for exactly the party members the last
`WatchableChanged` names", while such a player is also offered "No
shoulder") and VTT-406 (a player's board click on a listed party member's
token sends `set_viewpoint`, while such a player's sends nothing). And
SPEC-021's consequence "A non-party creature left in a player's control is
still theirs to move: … the move is judged by the eyes of the character the
player watches (SPEC-015)" stops holding for a player who controls no party
member: read literally, it lets the creature move wherever another player's
character sees, and the refusal of an occupied square then names its
obstruction through that character's eyes.

## What it touches

In this order:

1. `internal/gateway/viewpoint.go` (`MayWatch`, `defaultViewpoint`,
   `watchable`) and `internal/gateway/seat.go` (`judgedBy`), with tests in
   `internal/gateway/viewpoint_test.go`, `internal/gateway/watchable_test.go`,
   `internal/gateway/watch_test.go`, `internal/gateway/qa_move_gate_test.go`,
   `internal/gateway/seat_internal_test.go`,
   `internal/gateway/qa_creature_test.go`, the test files item 8 names, and
   `internal/gateway/authz_test.go`, `internal/gateway/creature_test.go` and
   `internal/gateway/qa_seat_test.go`, which cite the rows this withdraws.
2. `contract/vtt/v1/commands.proto` (`SetViewpoint`'s comment, and
   `WatchableChanged`'s new field), the gateway code that fills the field,
   `tools/toolgen/main.go` (`set_viewpoint`'s description), and what
   `task generate:contract` regenerates from them: `contract/gen/`,
   `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`; and
   `contract/testdata/expected_tools.json`, edited by hand to match, which
   `TestToolsMatchGolden` in `tools/toolgen/main_test.go` holds. The bundle
   `cmd/vtt/webdist/assets/index.js` is rebuilt in the same commit, since
   regenerating the TypeScript changes it.
3. `client/src/app.ts` (`mayPerch`, `paint`), `client/src/wire.ts` and
   `client/src/session.ts`; the comments that say only a spectator has the
   perch, in `client/src/view/spectator.ts` and `client/src/style.css`;
   `client/src/view/player.ts`, whose pointers cite the rows this withdraws;
   tests in `client/test/app.test.ts`, `client/test/spectator-view.test.ts`,
   `client/test/player.test.ts` and `client/test/qa-switch.test.ts`;
   `client/e2e/handover.spec.ts`; the bundle.
4. `docs/requirements.md`.

This crosses three components (gateway, contract and its generated tools, the
browser client) and names more than ten files. Asked whether to split it, the
owner answered on 2026-10-23 "One ticket, server first": one branch, its tasks
in the order above.

## Specifications this moves

docs/specifications/015-the-seat-and-the-perch.md
docs/specifications/013-authorization.md
docs/specifications/022-a-client-holds-one-log.md
docs/specifications/007-the-wire-contract.md
docs/specifications/021-each-party-member-keeps-its-own-log.md

## What could not be established

- **The console's Revoke button under the browser suite.** Item 11 uses the
  DM console's per-holder Revoke button, which no test in `client/e2e/` uses
  yet. Whether it works there has not been run.
