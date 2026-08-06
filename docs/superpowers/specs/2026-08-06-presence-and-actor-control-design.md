# Presence and actor control — design

**Status:** Accepted (signed off by Patrik, 2026-08-06). Not yet implemented —
the plan follows.

Two capabilities, one architectural idea borrowed from MapTool
(`net.rptools.clientserver`, read from the local checkout rather than
recalled): **session state is ephemeral, control is durable.**

MapTool stores ownership on the token itself (`Token.owners`, a
`Set<String>` of player ids, persisted with the campaign) and never in the
connection. Its `releaseClientConnection` frees the seat and broadcasts
`PlayerDisconnectedMsg`, but touches no ownership. So a player who drops
keeps their characters, and reconnecting under the same name restores control
with no handover step at all.

We already have the durable half: `Actor.controller_id` lives in the event
log. What is missing is the presence signal, and the ability for control to
move.

## 1. Scope

| | |
|---|---|
| **In** | presence notices to peers; grant/revoke actor control; many controllers per actor |
| **Out** | automatic client reconnect (§2); taking control from a live controller by anyone but DM/agent; presence in the campaign log |

## 2. Reconnect stays MANUAL — decided, with the reason

MapTool has **no** client auto-reconnect. The only retry loop in its codebase
is `WebRTCServer.retryConnect()`, which is the *server* re-establishing its
link to the WebRTC **signalling** service — not a player rejoining a game.
A dropped player reconnects by hand.

Patrik's reasoning independently matches: the server cannot know when a
user's network is back, so a retry policy is a guess. We keep it manual.

What this costs today is small and already nearly closed. `wire.ts` has
`reconnect()`, which redials at `after=<lastSeq>` precisely so replay does not
repeat history, and `session.ts` exposes it — **but nothing calls either**.
`ws.onclose` sets status `"closed"`, rejects in-flight commands, and stops.

So the work here is UI, not protocol: surface the `"closed"` status and offer
an explicit reconnect action that calls the code already written. A player who
reloads the page instead recovers fully today (`after=0`, full fold) — nothing
is lost either way; the difference is one click versus one reload.

## 3. Presence is a WIRE FRAME, not an event

**Decided (Patrik, 2026-08-06): a new `ServerFrame` variant, not a logged
event.**

The append-only log is campaign history that `engine.Apply` folds into game
state. Who happened to be online is not campaign history: appending it would
make every replay reconstruct session noise, change what `vtt state dump`
prints, and put transient facts in front of the one fold. MapTool agrees by
construction — `PlayerDisconnectedMsg` is a message, never campaign state.

`ServerFrame` already carries a non-event frame for exactly this kind of
out-of-band fact (`CatchUpHead`), so the shape exists.

```proto
// additive: a new oneof arm, nothing renumbered (ADR-007)
message PresenceChanged {
  string participant_id = 1;
  string display_name   = 2;
  bool   connected      = 3;   // false = gone
}
```

Emitted by the gateway when a connection is established and when `serve`
returns — which now covers **both** of Patrik's scenarios, because the server
already treats them identically: a network drop and a clean quit both unwind
through the same teardown (as does MapTool's single `releaseClientConnection`
handler for both). Nothing new is needed to *capture* the drop; PR #18 already
force-closes the socket and releases the subscription.

**Decided (Patrik): SNAPSHOT ON JOIN.** A joining client receives the current
presence set once, immediately after `CatchUpHead`, then deltas. A client must
never have to infer who is online from silence — this repo has been bitten
three separate times by inferring a condition from absence (a denial "proved"
by a dead connection, a batch read as complete because the stream went quiet,
a mutant scored killed because a command exited non-zero).

Two consequences the plan has to settle, both new server state:

**The gateway needs a presence registry.** It has none today — connections are
independent `serve` goroutines that know nothing of each other. Something must
hold "who is connected" to answer a snapshot, and it must be updated on both
teardown paths.

**Presence is per PARTICIPANT, not per connection, and they can differ.** Our
invite tokens are reusable (`identity.Verify` does not consume them), so the
same participant may hold two connections — a second browser tab, a phone
alongside a laptop. MapTool cannot hit this: its `playerMap` is keyed by
connection id and a player is one connection. Recommendation: reference-count
per participant, and emit `connected=false` only when the LAST connection for
that participant goes. Otherwise closing one tab tells the table someone left
while they are still sitting there — an "absence" that is not one, which is the
same class of bug as the three above.

## 3a. What already holds — verified, not assumed

Three properties Patrik asked for are already true in the code, and the spec
records them so nobody "implements" them twice:

**The credential and the character are already separate things.** An invite
token authenticates a `identity.Participant`; a character is an `Actor` whose
`controller_id` names a participant. Nothing melds them. `token` in this
repo's vocabulary means a figure placed on a scene (`TokenPlaced`,
`TokenMoved`) — the credential is an invite, and this spec never touches it.

**You can join a campaign controlling nothing.** A participant no actor names
simply controls no actors: they connect, receive the whole log, and may issue
whatever their role allows. Assignment comes later, which is the ordinary
opening state.

**DM authority is ORTHOGONAL to ownership, not a stronger form of it**
(`internal/gateway/authz.go:66`):

```go
if p.Role != identity.RolePlayer {
    return nil    // DM and agent skip every ownership check
}
```

So the DM already moves and uses any token while a player still controls it,
without revoking anything — MapTool's model, where ownership governs PLAYER
access and the GM is simply outside it. This is why the grant/revoke commands
below are about REASSIGNMENT ONLY. The DM never needs control transferred in
order to act, and any design that made them take it first would be a
regression dressed as a feature.

A consequence worth keeping in view: because ownership gates players alone,
widening `controller_id` to a set changes exactly one code path — the two
player checks at `authz.go:90` and `:106`. Nothing else in authorization is
affected.

## 4. Control becomes a SET, additively

Today `Actor.controller_id` is one string, and `authz.go:90`/`:106` both read
it as `controller_id == "" || controller_id != p.ID → deny`, with empty
meaning "DM/agent only".

MapTool has many owners per token. Matching that means a set — and ADR-007
forbids changing or removing `controller_id`.

```proto
message Actor {
  // ...
  string controller_id = 7;             // UNCHANGED, still populated
  repeated string controller_ids = N;   // additive; the authoritative set
}
```

The fold maintains both: `controller_ids` is truth, and `controller_id`
mirrors its single element (or is empty when the set is empty or larger than
one). Old readers keep working for the single-controller case, which is every
actor that exists today. Authz reads the set, falling back to
`controller_id` only while migrating.

Commands are imperative, events past-tense (CLAUDE.md rule 3):

| command | event | who may |
|---|---|---|
| `GrantActorControl{actor_id, participant_id}` | `ActorControlGranted` | DM, agent |
| `RevokeActorControl{actor_id, participant_id}` | `ActorControlRevoked` | DM, agent; **or** a player revoking themselves |

**Decided (Patrik):** a player may release an actor they control, but may not
grant to others, take from others, or claim an uncontrolled actor. Taking
control from a live controller is a DM/agent action.

**Decided (Patrik):** control is NOT presence-dependent. A disconnected player
keeps their actors until someone with authority reassigns them — MapTool's
behaviour, and the reason ownership lives on the token there. Making authz
depend on who is currently connected would make authorization time-dependent
and racy, and would mean a network blip could silently hand a character to
someone else.

Note this makes the "can control be taken from a live controller" question
much narrower than it first appeared. The DM never needs to take control to
ACT (§3a); revocation exists only to reassign a character — when a player
leaves the table for good, or a character changes hands between sessions.

This is what "one user can play many tokens through one connection" resolves
to: one participant appearing in several actors' `controller_ids`. That
already works the moment the set exists — it needs no separate feature.

## 5. What this does NOT change

- **The invite token is untouched.** It authenticates a participant and is
  reusable across reconnects (`identity.Verify` does not consume it; only an
  explicit `Revoke` invalidates). Manual disconnect does **not** release a
  credential for someone else to use — an early reading of "release the player
  token", corrected before any code was written. `token` in this repo means a
  figure placed on a scene (`TokenPlaced`, `TokenMoved`); the credential is an
  invite.
- **The one fold.** `ActorControlGranted`/`Revoked` fold like any other event.
- **The store and gateway teardown**, which PR #18 already made correct.

## 6. Test obligations (ADR-009)

- Authz: the grant/revoke cells across all four roles, added to the existing
  literal command × role matrix — not sampled.
- The fold: `controller_id` and `controller_ids` cannot disagree; a
  fault-injection proof per load-bearing assertion.
- Presence: a frame on connect AND on both teardown paths (clean quit, wedged
  client force-closed). The second is the one that will be forgotten, and it is
  the one PR #18's zombie fix made reachable.
- A player revoking themselves is allowed; a player revoking someone else is
  denied. Both directions, because a guard that only ever says "yes" is not a
  guard.

## 7. Open for the plan

1. Presence registry shape, and reference-counting per participant (§3).
2. Whether `controller_id` is eventually deprecated in doc only, or kept
   indefinitely as the single-controller convenience.
3. Client UI for reconnect (§2) — same sub-project or separate.

## 8. Decisions on the record

All from Patrik, 2026-08-06, during the design conversation:

- Presence is a wire frame, never a logged event.
- Snapshot on join, then deltas.
- Reconnect stays MANUAL; MapTool has no player auto-reconnect either.
- DM/agent grant and revoke; a player may release themselves only.
- Control is not presence-dependent; a disconnected player keeps their actors.
- The character token and the connection credential stay separate things.
