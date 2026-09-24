# SPEC-009: Identity lives beside the log, not in it

## Status

Accepted. Implemented by `internal/identity/identity.go`,
`internal/gateway/join.go`, `internal/gateway/authz.go`,
`internal/gateway/server.go` and `internal/gateway/metadata.go`; pinned by the
checks the rows under Requirements name, in
`internal/identity/identity_test.go`,
`internal/identity/fault_internal_test.go`, `internal/gateway/join_test.go`,
`internal/gateway/authz_test.go`, `internal/gateway/server_test.go`,
`internal/engine/role_test.go` and `internal/engine/qa_role_test.go`.

## Principles served

This project has no blueprint (`CLAUDE.md`, "Where the blueprint is"). The
principle this record would serve, that what a person may do is a fact about
now and not about when they connected, is missing from the record rather than
absent from the system; the blueprint is its own ticket.

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

**Authentication happens once; authorization happens continuously.** `handleWS`
verifies the token against the plain HTTP request, before `websocket.Accept`; a
token `identity.Verify` refuses gets a 401 and the connection is never
upgraded. From then on the participant is re-resolved through
`identity.Lookup`: in the read loop before every command, through
`credentialGone` before the backlog and before every event taken from the log
(the frames a `set_viewpoint` perch delivers are not re-checked; the command
that placed the perch was), and through `revoked` before a connect or departure
announcement in `announcePresence` and `announceDeparture`. The promotion nudge
`announcePromotion` sends to every connection without consulting `revoked`: it
is the one presence frame that does not re-resolve, and
`docs/verification-debt.md`'s entry dated 2026-09-24 carries it as a defect
against VTT-032. A lookup that answers `identity.ErrInvalidToken` ends the
connection, from the read loop with a policy-violation close and from delivery
through `credentialGone`. Any other lookup error is operational:
`answerCommand` refuses that one command with `gateway: identity unavailable`
and the connection stays; the delivery and presence checks match
`ErrInvalidToken` alone and let an operational failure through. The `/api/*`
routes verify the bearer token on every request in `authed`, and answer an
unknown and a revoked token identically. `identity.Verify` finds the row by
equality on the credential's SHA-256 hash, which is not secret to anyone
without the credential, and confirms the match with
`subtle.ConstantTimeCompare`. A stored role that does not parse is refused by
`Verify`, `Lookup` and `List`, never skipped or defaulted.

**One shared link admits people; the DM promotes them afterwards.** `POST
/join` is unauthenticated. It reads at most `maxJoinBody` bytes, refuses a
display name `usableDisplayName` rejects with its own 400 before the door is
consulted, and asks `identity.JoinAdmits` with the secret the body carries. A
refusal, and an identity error, are answered with one 403 and one body. An
admission mints a participant at `identity.RoleSpectator` through
`CreateInvite`, and the credential it returns is the only copy: the table holds
its SHA-256 hash. A participant who reconnects with that credential resolves to
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
and an omitted JSON field decodes to the empty string. The admission is one
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
`authorizePromotionTarget`, to player or spectator; the role it may act upon is
bounded in `handlePromotion`, which refuses a target whose current role is dm
or agent, because `Authorize` performs no I/O and cannot read the target's row.
`identity.SetRole` changes the role column and nothing else, reports a
participant it did not find, and leaves a revoked participant revoked. When the
promoted participant is connected, the handler then sends a `PresenceChanged`
frame naming them to every connection through `announcePromotion`, theirs
included; when they are not, nothing is sent. It appends nothing; SPEC-007
names the commands that append nothing and this is one of them.

**Revoked participants are not listed.** `identity.List` reads `WHERE revoked =
0`, ordered by display name and then id, and `GET /api/participants`, open to
the roles in `participantRoles`, is built on it.

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
an identity store that cannot answer the door's question is refused with one
answer; a `CreateInvite` that fails after an admission is answered as a server
error with its own body; a display name that is empty, over
`maxDisplayNameRunes`, or carries control, bidi or invisible-only content is
refused distinctly, before the door is consulted.

A promotion to dm or agent is refused, and so is a promotion of a dm or an
agent. A `set_join_door` that says neither open nor closed is refused.

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
`rotate_join_link` and `promote_participant` are ruled dm-and-agent in
`commandRoles`. Which commands append nothing is SPEC-007's. A campaign's DM is
minted out of band, by `vtt invite`.

## Requirements

VTT-005, VTT-006, VTT-007, VTT-008, VTT-009, VTT-010, VTT-011, VTT-012,
VTT-013, VTT-014, VTT-015, VTT-016, VTT-017, VTT-018, VTT-019, VTT-020,
VTT-021, VTT-022, VTT-023, VTT-024, VTT-025, VTT-026, VTT-027, VTT-028,
VTT-029, VTT-030, VTT-031, VTT-032, VTT-033, VTT-034, VTT-036, VTT-037,
VTT-038, VTT-039, VTT-040, VTT-041, VTT-042, VTT-043, VTT-044, VTT-045,
VTT-046, VTT-047, VTT-048, VTT-049.
