# SPEC-009: Identity lives beside the log, not in it

## Status

Accepted. Implemented by `internal/identity/identity.go`,
`internal/gateway/join.go`, `internal/gateway/authz.go`,
`internal/gateway/server.go`, `internal/gateway/presence.go`,
`internal/gateway/metadata.go`, `cmd/vtt/invite.go`, `client/src/join.ts`,
`client/src/app.ts`, `client/src/view/join.ts` and `client/src/view/dm.ts`;
pinned by the checks the rows under Requirements name, in
`internal/identity/identity_test.go`, `internal/identity/qa_names_test.go`,
`internal/identity/qa_joining_internal_test.go`,
`internal/identity/fault_internal_test.go`,
`internal/identity/identity_failure_test.go`, `internal/gateway/join_test.go`,
`internal/gateway/qa_names_test.go`, `cmd/vtt/cli_test.go`,
`cmd/vtt/qa_invite_names_test.go`, `client/test/join.test.ts`,
`client/test/app.test.ts`, `client/test/qa-join-names.test.ts`,
`client/e2e/joining.spec.ts`,
`internal/gateway/authz_test.go`, `internal/gateway/server_test.go`,
`internal/gateway/qa_promotion_revoked_test.go`,
`internal/gateway/qa_seat_test.go`, `internal/gateway/seat_internal_test.go`,
`internal/gateway/qa_promotion_rejudge_test.go`,
`internal/gateway/qa_promotion_opening_test.go`,
`internal/gateway/qa_promotion_queued_test.go`,
`internal/gateway/remove_participant_test.go`,
`internal/gateway/qa_removal_test.go`,
`internal/gateway/qa_remove_roles_test.go`, `client/test/dm-view.test.ts`,
`client/test/qa-boot.test.ts`, `client/test/qa-survivor-rules.test.ts`,
`client/e2e/booting.spec.ts`, `internal/engine/role_test.go` and
`internal/engine/qa_role_test.go`.

## Principles served

Principles 7, 8, 9, 10 and 22 of `docs/blueprint.md`:

- **7.** Authentication happens once and authorization continuously: the role
  is looked up again before every command.
- **8.** The door and revocation live in the identity store, so replaying a
  campaign can neither reopen a door nor bring a revoked participant back.
- **9.** A join refused at the door, or for a name, writes nothing.
- **10.** A name another participant who is not revoked holds is refused at
  the door.
- **22.** A participant who reconnects with their credential resolves to the
  same id, and keeps their name, their role and their characters.

## How it works

**Identity is not event-sourced.** Participants, their roles, their credential
hashes and the campaign's join door live in two SQLite tables, `participants`
and `join_access`, in the campaign file the event log also uses;
`identity.Open` takes its own handle on that file, independent of `store.Open`.
`engine.State` names no role and `internal/engine` reads none (`grep -rn Role
internal/engine/` prints nothing). An envelope carries `actor_role`, the
issuer's role at the moment of the command, stamped by the gateway (`ToEvent`
in `internal/gateway/convert.go` and the batch handlers beside it) and by
`internal/eventgen`; nothing outside a test file in `internal/engine`,
`internal/campaign` or `internal/store` reads it (`grep -rn ActorRole` over
those three directories, test files aside). No message reachable from the
Envelope's payload oneof has a field whose name contains `role`; whether a
string field ever carries a role as its value is not decidable from the
contract and is not claimed. The door is a row in `join_access` and revocation
is an update to `participants` through `identity.Revoke`, so replaying a
campaign can neither reopen a door nor bring a revoked participant back.

**Authentication happens once; authorization happens continuously.**
`handleWS` verifies the token against the plain HTTP request, before
`websocket.Accept`; a token `identity.Verify` refuses gets a 401 and the
connection is never upgraded. From then on the participant is re-resolved
through `identity.Lookup`: in the read loop before every command; on the pump,
through `pumpState.revoked`, when it starts, on every append wake and every
perch and every `rejudge` signal it takes, before and after `admit` decides
each envelope it takes, and after every switch (SPEC-015); and through
`revoked` before every presence announcement: an arrival or a departure in
`announcePresence` and `announceDeparture`, and a promotion in
`announcePromotion`. An announcement skips a revoked participant's connections
and leaves them open; their next command, or the pump's next wake, `rejudge`
signal, perch or envelope, closes them, so a log seat's connection is closed
at the next append whether or not it sends that connection an entry (VTT-352).
A lookup that answers `identity.ErrInvalidToken` ends the connection with a
policy-violation close, from the read loop and from the pump alike (VTT-376).
A successful lookup on the pump replaces the seat's participant, so the pump
judges by the current role (VTT-340, SPEC-015), as the read loop does for a
command (VTT-031). Any other lookup error is operational: `answerCommand`
refuses that one command with `gateway: identity unavailable` and the
connection stays; the delivery and presence checks match `ErrInvalidToken`
alone and let an operational failure through. The `/api/*` routes verify the
bearer token on every request in `authed`, and answer an unknown and a revoked
token identically. `identity.Verify` finds the row by equality on the
credential's SHA-256 hash, which is not secret to anyone without the
credential, and confirms the match with `subtle.ConstantTimeCompare`. A stored
role that does not parse is refused by `Verify`, `Lookup` and `List`, never
skipped or defaulted.

**One shared link admits people; the DM promotes them afterwards.** `POST
/join` is unauthenticated. It reads at most `maxJoinBody` bytes, trims the
display name of surrounding white space before anything judges or stores it,
refuses a name `usableDisplayName` rejects with its own 400 before the door is
consulted, and asks `identity.JoinAdmits` with the secret the body carries and
the trimmed name. `JoinAdmits` answers `identity.ErrNameTaken` for a name a
participant who is not revoked holds, and `handleJoin` answers that with a 400
of its own, whose body says the name is taken. Every other refusal `JoinAdmits`
gives, and every other error it returns, is answered with one 403 and one body.
A body that does not decode within `maxJoinBody` is refused as malformed with a
400 of its own. An admission mints a
participant at `identity.RoleSpectator` through `CreateInvite`, and the
credential it returns is the only copy: the table holds its SHA-256 hash.
A participant who reconnects with that credential resolves to
the same id, so they keep their name, their role and, through the log, their
characters. The DM reads the secret, the door and the budget together from `GET
/api/join-link`, open to the roles in `joinLinkRoles`; the secret is readable
before the door is opened. `JoinSecret` reads before it writes and `JoinBudget`
never writes, so once the secret exists the console's poll takes no write lock
on the file the log appends to; `JoinOpen` answers false when the database
cannot be read. `set_join_door` reaches `identity.SetJoinOpen` and is refused when it
says neither open nor closed; `rotate_join_link` reaches
`identity.RotateJoinSecret` and its result carries no secret.

**Every refusal the one read can decide is answered before anything is
written.** `JoinAdmits` reads the row once, compares the secret with
`subtle.ConstantTimeCompare` inside `internal/identity`, and answers a shut
door, an empty stored secret, a wrong secret and a spent budget with a refusal
before any statement writes, and the refusing path runs that one `SELECT` and
no other statement, so it takes no write lock and opens no write transaction; a
campaign with no door row is refused without creating one. The empty-secret
term is there because a constant-time compare of two empty strings is a match
and an omitted JSON field decodes to the empty string. Only past those
refusals does `JoinAdmits` read the display names of every participant who is
not revoked, so of the callers of `POST /join`, only one holding the current
secret at an open door with budget left learns from it whether a name is held;
a held name is refused with
`ErrNameTaken` before the `UPDATE`, so that refusal also writes nothing.
The admission is one
`UPDATE` whose `WHERE` re-states the door and the budget, so two joiners racing
for the last slot are serialised by the database and the one whose update
matches no row is refused; that joiner, and one whose door was shut between the
read and the write, is refused after taking the write lock, which only a caller
holding the current secret at an open door with budget left can reach. The
secret is not re-checked in that `UPDATE`: a rotation landing between the read
and the write admits one in-flight holder of the old secret. A `CreateInvite`
failure after an admission leaves the slot spent; there is no compensating
decrement.

**The three refusals give one answer.** `handleJoin` writes one status and one
body for a shut door, a wrong secret and a spent budget, and the same for an
identity store that cannot answer.

**Admission is bounded by a count, per opening.** `SetJoinOpen` resets the
spent count on every call, writes the limit on every call, a close included, so
that `JoinBudget` reports it until the next `SetJoinOpen`, and coerces a non-positive
limit to `identity.DefaultAdmitLimit`, because protojson omits zero values and
an absent limit and a deliberate zero arrive as the same bytes.
`RotateJoinSecret` resets the spent count and leaves the door and the limit as
they are; on a campaign with no door row it writes a shut door with no budget,
which is what an absent row already reads as. `migrateLocked` gives a door that
stands open with no budget the default, keyed on that state and not on which
column it added.

**Promotion is a command.** `promote_participant` is ruled by `commandRoles`
like every other command (`TestEveryClientCommandHasRoleCells` holds that no
command is missing from it). The role it may grant is bounded in
`authorizePromotionTarget`, to player or spectator; the role it may act upon
is bounded in `handlePromotion`, which looks the target up and refuses one
`identity.Lookup` does not answer, a revoked participant among them, and one
whose current role is dm or agent, because `Authorize` performs no I/O and
cannot read the target's row. `identity.SetRole` changes the role column and
nothing else, reports a participant it did not find, and leaves a revoked
participant revoked. The handler then signals each of the promoted
participant's connections through `presenceRegistry.rejudge`, so each
re-judges what it watches by the new role (SPEC-015). When the promoted
participant is connected, the handler then sends a `PresenceChanged` frame
naming them through `announcePromotion` to every connection whose participant
is not revoked, theirs included (VTT-309); when they are not, nothing is sent
(VTT-310). It appends nothing; SPEC-007 names the commands that append nothing
and this is one of them.

**Removal is a command.** `remove_participant` is ruled dm-and-agent in
`commandRoles`, so a player and a spectator are refused it, and since a dm
or an agent cannot be its target, nobody can remove themselves. The role it
may act upon is bounded in `handleRemoval`, which looks the target up and
refuses one `identity.Lookup` does not answer, a revoked participant among
them, and one whose current role is dm or agent, for the reason
`handlePromotion` does (VTT-564). It then builds one `ActorControlRevoked`
for each actor whose controllers include the target, in actor-id order
(SPEC-007), and only then writes: `identity.Revoke`, then a signal to each
of the target's connections through `presenceRegistry.rejudge`, then the
batch through `campaign.AppendBatch`, or nothing when the target controls
no actor. Each signalled pump re-resolves through `pumpState.revoked`, whose
lookup answers `identity.ErrInvalidToken`, and closes its connection with
the policy-violation status and the reason `gateway: credential no longer
valid` (SPEC-011), so the connections close without waiting for a command
of theirs or an append (VTT-558), and no frame the batch causes reaches them
(VTT-566). A participant who is not connected is removed the same way
(VTT-557). Afterwards they are not listed, a dial with their credential is
answered 401, and their display name is free, as any revoked holder's is.
The DM's console offers the removal as Boot on a player's and a spectator's
row in `renderDMConsole`, asks first in a message holding their display
name, sends nothing on a no, and reads the roster again once the removal is
answered (VTT-568, VTT-569, VTT-570, VTT-571). The message says that their
sign-in stops working for good, that they no longer control any character,
and that they can come back only through the share link while it is open
(VTT-667).

**Revoked participants are not listed.** `identity.List` reads `WHERE revoked =
0`, ordered by display name and then id, and `GET /api/participants`, open to
the roles in `participantRoles`, is built on it.

**A display name a participant who is not revoked holds is refused to anyone
new.** `CreateInvite` and `JoinAdmits` read the names of every participant who
is not revoked and compare each with the name offered under
`strings.EqualFold`, letter by letter ignoring case, in Go, because SQLite's
`lower()` and `NOCASE` fold ASCII alone. `CreateInvite` refuses a held name
with `ErrNameTaken` before it inserts, and outside test files its `INSERT INTO
participants` is the only one (`grep -rln "INSERT INTO participants" internal
cmd | grep -v _test.go`), so every path that mints a participant is held to
it: `handleJoin`, `vtt invite` (`newInviteCmd`), which returns the error
before it prints a token, and `mintInvites` in `cmd/vtt/harness_boot.go`. A
name held only by a revoked participant is free. A campaign that already
holds two participants of one name keeps them; `List` orders them by display
name and then id, so two names alike byte for byte come in id order. A
failure to read the names is an error, never "not held": `JoinAdmits` refuses
and spends nothing, `CreateInvite` mints nothing.

**The join form shows the name's refusal and keeps what was typed.**
`requestJoin` in `client/src/join.ts` reads a 400 as the name's refusal: its
body trimmed of surrounding white space, then without a leading `gateway: `, or
a fixed refusal of the name when the trimmed body is empty. A 403 is the door's,
one message for every cause. `runJoin` in `client/src/app.ts` keeps the typed
name after any refusal, and `renderJoinView` in `client/src/view/join.ts` puts
it back in the box on each repaint.

**The door's columns arrive by migration, and the migration takes the write
lock only when it has something to do.** `identity.Open` applies its schema
with `CREATE TABLE IF NOT EXISTS`, which is why `join_access` is a table of its
own: a new column in that schema never reaches a campaign whose table exists.
The budget columns are added by `migrate`, which reads the table's shape and
the door first, returns when no column is missing and no open door lacks a
budget, and otherwise takes `BEGIN IMMEDIATE` on one pinned connection and
re-reads the shape under the lock before altering. Its `ALTER TABLE` statements
are `ADD COLUMN` statements in `migrateLocked` and nothing else (grep `ALTER
TABLE` in `internal/identity/identity.go`), so a column this package does not
name is left in place and is inert.

## Consequences

A join at a shut door, with a wrong secret, against a spent budget or against
an identity store that cannot answer the door's or the names' question is
refused with one answer; a `CreateInvite` that fails after an admission is
answered as a server error with its own body; a display name that is empty, over
`maxDisplayNameRunes`, or carries control, bidi or invisible-only content is
refused distinctly, before the door is consulted.

A join under a display name a participant who is not revoked holds is refused
with its own 400, after the door, and writes nothing; to a caller the door
refuses, a held name and a free one get the same answer. `vtt invite` naming a
held name exits non-zero and prints no token.

A promotion to dm or agent is refused, and so is a promotion of a dm or an
agent. A removal of a dm or an agent is refused, and a removal cannot be
undone: no statement sets `revoked` back to 0 (`grep -n 'SET revoked'
internal/identity/identity.go` finds `Revoke`'s `revoked = 1` alone), so the
person comes back only as a new participant, through the share link or a
new `vtt invite`. A
`set_join_door` that says neither open nor closed is refused.

A join refused on the read takes no write lock on the campaign file, which is
what lets `POST /join` run with no rate limiting: a stranger without the
current secret cannot reach the write. The day a refusal on the read writes,
that decision is void.

A role never travels in a presence frame: `PresenceChanged` in
`contract/vtt/v1/commands.proto` carries an id, a display name and a state, and
`PresenceSnapshot` repeats it. The roster with roles is `GET
/api/participants`.

Control of a character is not identity's: it is an `ActorControlGranted` in the
log, decided by `internal/gateway/authz.go` against the fold. `set_join_door`,
`rotate_join_link`, `promote_participant` and `remove_participant` are ruled
dm-and-agent in `commandRoles`. Which commands append nothing is SPEC-007's.
A campaign's DM is minted out of band, by `vtt invite`.

## Requirements

VTT-005, VTT-006, VTT-007, VTT-008, VTT-009, VTT-010, VTT-011, VTT-013,
VTT-014, VTT-015, VTT-016, VTT-017, VTT-018, VTT-020, VTT-021, VTT-022,
VTT-025, VTT-026, VTT-027, VTT-028, VTT-029, VTT-030, VTT-031, VTT-032,
VTT-033, VTT-034, VTT-036, VTT-037, VTT-038, VTT-039, VTT-040, VTT-041,
VTT-043, VTT-045, VTT-046, VTT-047, VTT-048, VTT-049, VTT-060, VTT-061,
VTT-062, VTT-063, VTT-064, VTT-065, VTT-066, VTT-067, VTT-068, VTT-069,
VTT-070, VTT-071, VTT-072, VTT-073, VTT-074, VTT-075, VTT-076, VTT-077,
VTT-256, VTT-309, VTT-310, VTT-352, VTT-376, VTT-542, VTT-543, VTT-544,
VTT-545, VTT-546, VTT-548, VTT-549, VTT-550, VTT-551, VTT-552, VTT-553,
VTT-554, VTT-555, VTT-556, VTT-557, VTT-558, VTT-564, VTT-566, VTT-568,
VTT-569, VTT-570, VTT-571, VTT-667.
