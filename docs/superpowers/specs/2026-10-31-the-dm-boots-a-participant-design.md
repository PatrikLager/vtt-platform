# The DM removes a participant from the console

## The problem

**A participant can be revoked only from the command line, by an id the
console never shows.** `identity.Revoke` in `internal/identity/identity.go`
flips a participant's `revoked` flag, and its one caller outside tests is
`vtt revoke --campaign <dir> --id <participant id>` in `cmd/vtt/revoke.go`. No
`ClientCommand` in `contract/vtt/v1/commands.proto` revokes a participant:
`RevokeActorControl` takes a character from its holder, and
`PromoteParticipant` changes a role. The DM console's "Who may do what" group
(`renderDMConsole` in `client/src/view/dm.ts`) draws one row per participant
with their name, their role, and a "Make player" or "Make spectator" button;
the participant id is only the row's `data-participant` attribute, which
nothing draws.

**So a participant who lost their credential stays at the table for good.**
Somebody whose browser lost its credential joins again through the share link
and becomes a new participant, under a new name since the old one is held
(`ErrNameTaken`, SPEC-009). The old participant stays in the roster, and stays
a controller of every character they held: control is an
`ActorControlGranted` in the log and survives disconnection. The console names
each holder from the roster or from presence (`known` in `renderDMConsole`);
once a holder is revoked by `vtt revoke` the roster no longer lists them, and
presence stops listing them once their connections close, at their next
command or the next append; from then the character row shows that holder's
raw id, marked away. `vtt revoke` touches identity
only; no `ActorControlRevoked` follows it.

MapTool's Boot Player (`BOOT_CONNECTED_PLAYER` in `AppActions`) is offered to
the GM, refuses the GM's own name, asks "Are you sure you want to boot {0}?",
and disconnects a player who is connected, whose client then says "You have
been booted from the server." The player can connect again, because there the
name is the identity. The owner's rulings of 2026-10-31 go further: a boot
here revokes the participant permanently, connected or not, and also revokes
their control of every character.

## Done looks like

1. The DM console's "Who may do what" row of a player or a spectator offers a
   control that, after the DM confirms, revokes that participant. Afterwards
   `identity.Lookup` answers `ErrInvalidToken` for them, `GET
   /api/participants` no longer lists them, and a join through the share link
   under their name is admitted. A client test and a gateway test name this,
   and both fail on today's tree, where no such control or command exists.
2. Cancelling the confirmation sends nothing and revokes nobody.
3. Every open connection of the removed participant is closed at once, not at
   their next command or the next append, and a connection opened afterwards
   with their credential is refused.
4. Every character the removed participant controlled no longer lists them
   among its controllers, through `ActorControlRevoked` in the log; any other
   controller of the same character keeps it.
5. A participant who is not connected is removed the same way.
6. A removal is refused, and nothing is revoked or appended, when its target
   is a dm, an agent, the issuer, unknown or already revoked, and when its
   issuer is a player or a spectator.
7. `vtt revoke` still works as it does today.
8. `task check` exits 0 on the branch.

## Rules this puts on the system

- A dm can remove a player or a spectator, revoking them as `vtt revoke`
  does.
- Removing a participant also revokes their control of every character.
- A removed participant's open connections close at once.
- A removal of a dm, an agent, oneself, or a participant who is unknown or
  already revoked is refused and changes nothing.

## What it touches

In this order, because each layer is what the next one calls:

1. `contract/vtt/v1/commands.proto` and the generated code under
   `contract/gen/`: a new command, additive (ADR-007, CLAUDE.md rule 3).
2. `internal/gateway/authz.go` (`commandRoles`) and `internal/gateway/server.go`
   (the command's handler, beside `handlePromotion`), with
   `internal/gateway/presence.go`'s `rejudge` to reach open connections.
3. `tools/toolgen/main.go` and `cmd/vtt/tools.json`, if the agent is given the
   command as an MCP tool (as `promote_participant` is).
4. `client/src/commands.ts` and `client/src/view/dm.ts`, with their tests and
   a beat in `client/e2e/joining.spec.ts`.
5. `docs/requirements.md`: the rules above.

Four components. Whether to split them is asked at sign-off.

## Specifications this moves

docs/specifications/009-identity-and-joining.md
docs/specifications/013-authorization.md
docs/specifications/007-the-wire-contract.md

## What could not be established

- Whether the agent may remove a participant. `promote_participant` is ruled
  dm-and-agent in `commandRoles` and is an MCP tool; the owner's ruling names
  the DM's console. The plan proposes, the owner decides.
- What the removed participant's own window shows when its connection is
  closed with "gateway: credential no longer valid". `client/src/app.ts` was
  not read for that close; MapTool says "You have been booted from the
  server."
- Whether the identity write and the log append can be made one step. Identity
  is not event-sourced (SPEC-009) and lives in its own tables, so a failure
  between the two writes leaves one done. The order is the plan's.
- Whether a dm removing a participant who is in the middle of a command can
  leave that command half-applied. Not read.
