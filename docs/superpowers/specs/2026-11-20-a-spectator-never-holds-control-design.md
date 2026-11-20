# A participant is a player exactly while they control an actor

## The problem

**A role and a character are kept apart, in stores that do not follow each
other.** A participant's role is identity's: `identity.CreateInvite` in
`internal/identity/identity.go` stores it at the mint (`vtt invite --role`
in `cmd/vtt/invite.go` takes dm, agent, player or spectator; a join mints a
spectator, `handleJoin` in `internal/gateway/join.go`), and
`promote_participant` changes it (`handlePromotion` in
`internal/gateway/server.go`, `identity.SetRole`). Control is the log's: an
`ActorControlGranted` or an `ActorControlRevoked`, folded by
`internal/engine/apply.go` (`controlTarget`), which cannot read a role
(VTT-039). Every decision the gateway makes reads the stored role,
`identity.Participant.Role`: `Authorize` and `commandRoles` in
`internal/gateway/authz.go`, `MayWatch`, `defaultViewpoint` and `watchable`
in `internal/gateway/viewpoint.go`, the seat in `internal/gateway/seat.go`,
`handleMe` in `internal/gateway/metadata.go` (`GET /api/me`), the
participant list (`GET /api/participants`) and the ruleset read
(`GET /api/ruleset`). So the two drift apart both ways:

- **A spectator who holds control.** `grant_actor_control` is checked for
  its form alone (`validateGrantActorControl` in
  `internal/gateway/grant_validate.go`), so a grant to a spectator is
  appended; `handlePromotion` makes a controlling player a spectator and
  revokes nothing. A grant to a participant id identity never minted, or
  has revoked, is appended too: `identity.Lookup` is never called on the
  grantee.
- **A player who holds no actor.** `vtt invite --role player` and a
  promotion to player make one before any grant, and a revocation of a
  player's last actor (`revoke_actor_control`, a player's own revocation,
  `remove_actor`) leaves one. The system serves that state on purpose:
  `MayWatch` and `defaultViewpoint` treat a player who controls no party
  member as a spectator, `watchable` tells them so through
  `controls_no_party_member` (VTT-454), and the window perches them with
  "You play no character; the DM can give you one." (`mayPerch` in
  `client/src/app.ts`, the perch in `client/src/view/spectator.ts`, the
  owner's ruling B-4 of 2026-10-22).

The console works the stored role. Its roster rows (`renderDMConsole` in
`client/src/view/dm.ts`) offer "Make player" for a spectator and "Make
spectator" for a player, whatever they hold, without asking. The MCP tool
texts (`tools/toolgen/main.go`) say a promotion to player is what lets a
participant "be given a character and act", and that a promotion "writes no
event".

**The owner's rulings.** 2026-11-15: "det är aldrig tillåtet, en åskådare ska
aldrig kunna spela, den måste först bli promoted till spelare". 2026-11-20:
"det ska inte kunna finnas en spelare utan karaktär, du kan endast vara en
spelare, om du har en karaktär, om du inte har en, så är du en åskådare";
asked how, the log decides: a participant who is not the DM or the agent is
a player while they control at least one actor, of any kind, and a spectator
otherwise, so a grant makes a spectator a player and the last revocation
makes a player a spectator; the console's "Make spectator" stays, takes
every actor the player controls in one command, and asks first ("Make Lera
a spectator? They no longer control Ash and Gobo."); a grant naming an id
identity does not hold, or holds revoked, is refused ("There is no such
participant."); one ticket; offered a blueprint sentence in which a
player none of whose characters is a party member still follows one of
their choosing: "Igen. Har du kontroll över en varelse eller karaktär så är
du en spelare". 2026-11-21, asked what such a player watches through: the
creature becomes a party member, so every actor a player controls is a
party member, a grant that would leave a player holding one that is not is
refused, and no player perches; the refusals read "Robin can only be given
a party member." and "Lera controls Ash: revoke it from them before you
make it a non-party actor."; "Make spectator" names the actors in the order
of their names.

**What MapTool does.** MapTool's roles are `PLAYER` and `GM`
(`Player.Role` in `net.rptools.maptool.model.player.Player`), and no
watching role. Ownership is a token's list of player names
(`Token.isOwner`), and `Players.setRole` changes the player database and
touches no token. There is nothing to borrow: MapTool's role is a
permission level stored apart from ownership, which is the split this
ticket removes.

**What this is not.** A change to the DM's or the agent's role, which stays
identity's and is never derived. A change to what a spectator may watch or
issue (VTT-139, SPEC-015). A change to who may grant or revoke (the DM, the
agent, and a player revoking themselves). A migration: nobody uses the
product (the owner's ruling of 2026-09-04). A change to removal beyond what
the role's derivation makes of it.

## Done looks like

1. **A grant makes a spectator a player.** After a `grant_actor_control`
   naming a participant who controls no actor, the server answers them as a
   player: `GET /api/me` says `player`, and `Authorize` accepts a player's
   command on that actor from them. A gateway test that fails on today's
   tree.
2. **The last revocation makes a player a spectator.** After the revocation
   of a player's last actor, by the DM, by the agent, by the player
   themselves, or by `remove_actor`, `GET /api/me` says `spectator` and a
   player's command is refused. A gateway test that fails on today's tree.
3. **No participant who controls nothing is a player.** A participant
   minted or promoted as a player who controls no actor is answered as a
   spectator by `GET /api/me`, by `GET /api/participants` and by the seat. A
   gateway test that fails on today's tree.
4. **No spectator controls an actor.** Every answer the server gives about a
   participant who is not the DM or the agent and controls an actor says
   `player`. A gateway test that fails on today's tree.
5. **A grant to nobody is refused.** A `grant_actor_control` naming a
   participant id identity does not hold, or holds revoked, is refused with
   "There is no such participant.", and nothing is appended. A gateway test
   that fails on today's tree.
6. **"Make spectator" takes every actor.** The console's "Make spectator" on
   a player asks the DM first, naming the actors they control, sends nothing
   if the DM declines, and on a yes leaves the log with one
   `ActorControlRevoked` per actor the player controlled and the participant
   a spectator. A client test and a gateway test that fail on today's tree.
7. **The console offers no "Make player".** No roster row offers a
   promotion to player; a grant is how a spectator becomes one. A client
   test that fails on today's tree.
8. **A window follows its role.** A window whose participant is granted an
   actor becomes a player's window, and one whose last actor is revoked
   becomes a spectator's perch, without a reload; no player's window
   perches, and the line "You play no character; the DM can give you one."
   is gone. A client test that fails on today's tree.
9. **The tool texts say so.** The MCP tool descriptions of
   `grant_actor_control`, `revoke_actor_control` and `promote_participant`
   say that a grant makes a spectator a player, that the last revocation
   makes a player a spectator, that a player's actor is a party member, and
   what is left of promotion. The
   tool-definition golden (`contract/testdata/expected_tools.json`) changes
   with them.
10. **No record models the old split.** No test, requirement row,
    specification or blueprint sentence states a player who controls no
    actor, a player who controls an actor that is not a party member, or a
    spectator who controls one, as a state the system holds. A
    reading, by the ticket's verifier and by each task's review.
11. **The gate.** `task check` is green, the embedded bundle under
    `cmd/vtt/webdist` is rebuilt from the changed client, and `task e2e` is
    green.
12. **A player's actor is a party member.** A `grant_actor_control` naming
    a participant who is not the DM or the agent with a kind other than
    party member is refused, and so is one making an actor such a
    participant controls a non-party actor; nothing is appended. A gateway
    test that fails on today's tree.

## Rules this puts on the system

- A participant who is not the DM or the agent is a player while the log
  holds them among the controllers of at least one actor, and a spectator
  otherwise, in every answer the server gives about them and every decision
  it makes for them.
- Every actor a participant who is not the DM or the agent controls is a
  party member; a grant naming such a participant with a kind other than
  party member, or stating a kind other than party member of an actor such
  a participant controls, is refused and appends nothing.
- A grant naming a participant id identity does not hold, or holds revoked,
  is refused and appends nothing.
- Making a player a spectator revokes their control of every actor they
  control, as events of the log, in one command.
- The console asks before making a player a spectator, naming the actors
  they control, and sends nothing on a no.
- The console offers no promotion to player.

Rows this contradicts, for the plan to withdraw or succeed: VTT-036 (a
promotion appends no event), VTT-025 and VTT-029 (what a promotion may do
and changes), and every row whose wording or evidence holds a player who
controls no actor or a spectator who controls one, VTT-454, VTT-455,
VTT-605 to VTT-607 and VTT-731 among them.

## What it touches

In this order:

1. **The gateway and identity.** Where a participant's role is resolved for
   a decision or an answer (`internal/gateway/authz.go`, `viewpoint.go`,
   `seat.go`, `metadata.go`, `server.go`, and the other files reading
   `identity.Participant.Role`), the grant's grantee check,
   `handlePromotion`, and what identity stores and accepts as a role
   (`internal/identity/identity.go`, `cmd/vtt/invite.go`), with their tests.
2. **The tool texts and the contract's comments.** `tools/toolgen/main.go`,
   the comments in `contract/vtt/v1/commands.proto` and `events.proto`,
   regenerated into `contract/gen`, `cmd/vtt/tools.json` and
   `contract/testdata/expected_tools.json`. No field or command is removed
   (rule 3).
3. **The client.** `client/src/app.ts` (when a window re-reads its role,
   `mayPerch`), `client/src/view/spectator.ts` (the player's perch line),
   `client/src/view/dm.ts` (the roster row's buttons), their tests under
   `client/test/` and `client/e2e/`, and `cmd/vtt/webdist`.
4. **The records.** `docs/requirements.md`, and `docs/blueprint.md` §16,
   whose sentence "A spectator, and a player who controls no character,
   follows a party member" names the state this removes.

This crosses the gateway, identity, the contract's comments and the client;
the owner asked for one ticket on 2026-11-20, before the role became
derived, and again at the plan's sign-off on 2026-11-21.

## Specifications this moves

- docs/specifications/007-the-wire-contract.md
- docs/specifications/009-identity-and-joining.md
- docs/specifications/011-the-connection.md
- docs/specifications/012-the-read-surface.md
- docs/specifications/013-authorization.md
- docs/specifications/015-the-seat-and-the-perch.md
- docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **Where the derivation lives.** Whether the gateway derives the role from
  the folded state at each decision, or keeps identity's column in step
  with the log at each grant and revocation, and what identity's column
  then holds for a participant who is not the DM or the agent, is the
  plan's. The owner's answer "the log decides" rules out a second store that can
  disagree.
- **What becomes of `promote_participant` and `vtt invite --role player`.**
  The command stays in the contract (rule 3); whether a promotion to player
  is refused, and whether the invite still takes `player`, is the plan's.
- **How a window learns its role changed.** Today a promotion reaches the
  window through a `PresenceChanged` that starts a `GET /api/me` read
  (VTT-712); a grant or a revocation starts none.
- **`controls_no_party_member`.** With every actor a player controls a
  party member, the flag is false for every player and true for every
  spectator; it stays in the contract (rule 3), and what the window reads it
  for is the plan's.
- **A grant racing a revocation.** Whether commands from two connections
  are serialised against each other was not established. With the role
  derived from the log, an interleaving leaves a state the rule allows,
  which the plan should confirm.
