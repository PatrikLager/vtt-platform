# ADR-011: Identity and authorization live beside the log, not in it

Status: accepted.

## Context

A table needs to know who is at it and what each person may do. Both questions
change while a session runs: somebody joins, somebody is promoted, somebody is
revoked. The campaign's event log is append-only and is replayed to rebuild
state, so anything recorded there is permanent and comes back on every replay.
That is right for what happened at the table and wrong for who is currently
allowed to sit at it.

## Decision

**Identity is not event-sourced.** Participants, their roles, their tokens and
the campaign's join door live in their own SQLite tables. `internal/engine`
holds no reference to a role, so the fold cannot express an authorization
question, and replaying a campaign cannot reopen a door or restore a revoked
participant.

**Authentication happens once; authorization happens continuously.** A token is
verified when a connection is established. Every command, every delivered event
and every presence frame then re-resolves the participant through
`identity.Lookup`, because what a person may do is a fact about now, not about
when they connected. Caching the answer for the life of a socket is the defect,
not the optimisation. Only an invalid token ends a connection; any other error
refuses the action and leaves the socket open.

**One shared link admits people; the DM promotes them afterwards.** The campaign
carries one join secret, an open/closed door and an admission budget, all on a
single row. An unauthenticated request carrying the secret mints its caller a
participant at the lowest role. Reconnecting through the same link returns the
same participant, so a person keeps their identity and their characters.

**Every refusal at that door is decided in Go, from one read, before anything is
written.** The secret is compared in constant time inside `internal/identity`
and never leaves it. A wrong secret, a shut door and a spent budget are all
decided from the same `SELECT` and all return before the write. This is the
security property the design rests on: identity shares its SQLite file with the
event log, so any write on a refusal path — even one matching no rows — takes
the write lock on the file the table is appending events to. A refused stranger
must cost the table nothing.

The admission itself re-states the whole condition in its `UPDATE`, so
simultaneous callers are serialised by the database and the loser matches no row.

**The three refusals give one answer.** A shut door, a wrong secret and a spent
budget are indistinguishable to the caller. A distinct answer would tell a prober
which half of their guess was right.

**Admission is bounded by a count, not by a clock.** A DM knows how many people
they expect more reliably than how long those people will take to arrive, and a
window that expires mid-arrival fails in a way the DM must diagnose from the
other end of a chat. Opening the door resets the count. A budget that is absent
or non-positive becomes `identity.DefaultAdmitLimit`, never "admit nobody",
because the wire encoding cannot distinguish an absent number from a deliberate
zero and of the two readings only one is debuggable.

**Promotion is a command, not an endpoint.** It travels the same path as every
other client command so that `commandRoles` remains the single place any
"who may do what" question is answered. It changes only the role: the token, the
participant id and the display name belong to the person and survive, as do the
characters they hold, which live in the log where a role change cannot reach.
Two bounds apply and they are separate: the role a promotion may grant is
bounded in `Authorize`, and the role a promotion may act upon is bounded in the
handler, because `Authorize` performs no I/O and cannot look up whom it is
about. A revoked participant stays revoked; promotion is not a way back in.

**Revoked participants are not listed.** They cannot connect and cannot act, so
offering the DM controls for them would describe a table that does not exist.

## What this does not decide

**Rate limiting.** The join endpoint has none. The argument for that is the
refusal path above: while the door is shut there is no state for a stranger to
change and no write for them to provoke. If a refusal ever begins writing, this
decision is void and rate limiting has to be reconsidered.

**Who may be granted control of which character.** That is authorization over
campaign content and is decided in the gateway against the log, not here.

## How this is enforced

- `.semgrep/event-sourcing.yml` keeps state mutation inside `engine.Apply`;
  `internal/engine` contains no role.
- `TestEveryClientCommandHasRoleCells` requires every command to appear in
  `commandRoles`, so a new command cannot arrive unruled.
- `TestPromotionMayOnlyTargetPlayerOrSpectator` and
  `TestAnAgentMayNotPromoteAnyoneToDMOrAgent` bound the two halves of promotion.
- `TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath` pins the empty-secret
  guard to the function the live path actually calls.

The rule that a refusal writes nothing is stated once per refusal. The
wrong-secret and spent-budget refusals are held by
`TestAWrongSecretRefusesWithoutTouchingTheDatabase` and
`TestASpentBudgetRefusesWithoutTouchingTheDatabase`, and the rule that refusals
are indistinguishable by `TestAClosedDoorAndAWrongSecretAreRefusedIDENTICALLY`.
The shut-door refusal has no test that can see its write;
`docs/verification-debt.md` records what the two tests aiming at it can and
cannot see.
