# Presence and actor control — design

**Status: ACCEPTED — Patrik, 2026-08-07.**

Written 2026-08-07, after Tasks 1 and 2 had already been built and reviewed, to
close a real gap: the plan cited this document as "Accepted, Patrik 2026-08-06"
and it did not exist. The design below is the one actually agreed in
conversation and implemented; nothing here was new invention at the time of
writing. It was reviewed and accepted on 2026-08-07.

Where an already-shipped decision is recorded, this document says so and points
at the code, so a reader can check the claim rather than trust it.

## 1. The problem

Two things are missing, and they are not the same thing.

**Peers cannot see each other.** Nobody at the table learns that a player
joined, left, or dropped. Patrik: a disconnect message to other players is
something "we definitively have".

**An actor has exactly one controller, forever.** `Actor.controller_id` is a
single string set at `ActorAdded` and never changed by any event. So a character
cannot be handed to another player, cannot be shared, and cannot be picked up by
the DM or an agent when its player leaves.

## 2. Vocabulary, stated once

This repo and MapTool/RPTool use "token" for different things, and the
conversation that produced this design used the RPTool sense. In this document:

| Term | Meaning here |
|---|---|
| **actor** | the character, monster or NPC — MapTool's "token" |
| **token** | this repo's `Token`: an actor's PLACEMENT on a scene (scene, x, y) |
| **participant** | a person or agent holding an invite credential |
| **connection** | one live WebSocket. A participant may hold several |

**Control attaches to the ACTOR, not to the token.** Moving a figure is
authorised through the actor its token refers to — already the case at
`internal/gateway/authz.go:84`, which resolves token → actor → controller.

## 3. What must be true

### 3.1 The connection and the character are not melded

Patrik, explicitly: the character token and the connection token are two
different things and must not be "hard melded" together.

- A participant logs in with **no** actors. That is how every campaign starts.
- Actors are assigned afterwards, and a participant may control **many** actors
  through **one** connection.
- Losing a connection must never destroy the assignment. Presence is
  connection-scoped; control is campaign-scoped and lives in the event log.
  See §3.1a: this REVERSES the original manual-disconnect-releases scenario,
  adjudicated 2026-08-08.

### 3.1a Disconnecting does NOT release a character

**Adjudicated 2026-08-08, Patrik's call.** This REVERSES the scenario that
prompted the feature, so it is recorded rather than left implicit.

The original ask was: a manual disconnect should release the character so any
other player, DM or agent can take it over. What shipped keeps control until
somebody revokes it. Both readings survived into this document unnoticed —
§3.1's "losing a connection must never destroy the assignment" contradicted the
scenario, and the spec was accepted with the contradiction in it.

Three reasons the shipped behaviour won:

1. **A clean quit and a dropped network are the same event to a server.** It
   cannot tell them apart unless the client announces it, which is why
   MapTool's `releaseClientConnection` handles both through one path. Releasing
   on any disconnect means dropped wifi hands your character to the table.
2. **It is what MapTool does.** `Token.ownerList` is a `Set<String>` persisted
   with the campaign, and the only callers of `removeOwner`/`clearAllOwners`
   are two UI dialogs — the edit dialog and the token popup menu. No disconnect
   path touches ownership; a human takes a character away, or nobody does.
3. **Handover has a home now.** The DM console's "Who controls what" panel
   (T7) does deliberately what an automatic release would do by accident, and
   §3.2 already lets the DM act on a held character without revoking anyone.

How a returning player gets their characters back: the invite TOKEN. Verify
hashes it to a stable participant id, the browser keeps it in localStorage, and
control is keyed on that id — so the same credential resolves to the same
person across any number of disconnects. Stronger than MapTool, which keys
ownership on a player-name STRING typed at connect time.

Still open, and deliberately not built here: spec §5.3 already lets a PLAYER
revoke their own control, and no client offers it. That is the piece which
would give the original scenario its intent — putting a character down on
purpose — without making a network blip do it for you.

### 3.2 The DM can always take an actor

The DM (and an agent) can act on any actor **while a player controls it**,
without revoking that player's control. Grabbing is not a transfer.

Already true, and this design must not break it: `commandRoles`
(`internal/gateway/authz.go:19-49`) grants DM and agent every command
unconditionally, and the ownership helpers at `:84` and `:104` are consulted
only for `RolePlayer`. Ownership has never gated the DM.

### 3.3 Control is a set

`addOwner`/`removeOwner`, in Patrik's words. Concretely `ActorControlGranted`
and `ActorControlRevoked`, folded into a `controller_ids` set.

### 3.4 Reconnection is manual

Checked against MapTool before deciding: it has **no player auto-reconnect**.
Its only retry loop is `WebRTCServer.retryConnect()`, for signalling, not for a
dropped player session.

We keep it manual, for a reason independent of the precedent: the server cannot
know when someone's network is back. A timer would guess, and a wrong guess
either hammers a dead link or reconnects into a session the player has left.
`client/src/wire.ts` already has `reconnect()` redialling at `after=<lastSeq>`;
what is missing is a surfaced status and a control that calls it.

## 4. Presence

**Presence is a wire frame, never an event.** It is not appended to the log and
does not survive a restart. Who is currently connected is not a fact about the
campaign's history — replaying a campaign must not resurrect a session.

`PresenceChanged{participant_id, display_name, state}` and
`PresenceSnapshot{present[]}` ride on `ServerFrame` alongside
`result`/`event`/`catch_up_head`.

`state` is an **enum** (`PRESENCE_STATE_UNSPECIFIED` / `CONNECTED` /
`DISCONNECTED`), not a `bool connected`. protojson omits zero values, so a
`bool` would send DISCONNECT as an ABSENT field — the single most important
transition would be carried by silence. An explicit enum makes it a value on the
wire, and reserves UNSPECIFIED to catch a sender that forgot to set it.

**Reference-counted per participant, not per connection.** One invite may hold
two connections (a tab and a phone). Closing one must not tell the table that
person left; `DISCONNECTED` is emitted only when the last connection for that
participant goes.

### 4.1 Delivery is best-effort, with a bounded wait

**Ratified by Patrik 2026-08-07.** A presence broadcast waits up to
`presenceSendBudget` (3s) for each connection and then DROPS that frame.

The wait exists because an instant drop was wrong: a connection's outbound
queue is filled by the CATCH-UP BACKLOG on every fresh connect, so a full queue
usually means "replaying a large campaign", not "wedged". Measured during
review, a healthy client draining 400 backlogged events missed a joiner's
arrival outright, and nothing re-sends it — reconnection is manual (§3.4), so
the table stayed wrong until the user acted.

The bound exists because an unbounded wait would let ONE stalled reader hold
an announcement hostage for everyone — the fan-out wedge the store's
per-subscriber queue was built to prevent, reintroduced a layer up.

So a frame IS still lost if a client does not drain for three seconds. That
client is being torn down anyway under the write deadline, and its replacement
connection opens with a fresh snapshot — which, per §4's replace semantics, is
also what repairs any list that drifted. Presence is soft state: it is repaired
by the next snapshot, never by the log.

**Both teardown paths must be covered**, and the second is the one that gets
forgotten:

1. a clean quit, and
2. a wedged client force-closed by the write deadline.

MapTool treats these identically — `releaseClientConnection` handles a network
failure and a clean quit through the same path — and we should too, because a
client that has stopped reading is gone whether or not it said so. Path 2 exists
here because of the store's per-subscriber queue and the gateway write deadline
(PR #18); before that work there was a window in which a wedged connection was
neither serving nor released.

## 5. Actor control

### 5.1 The set, and the mirror

`Actor` gains `repeated string controller_ids`. `controller_id` **stays**, as a
MIRROR of `controller_ids[0]`, empty only when the set is empty.

The mirror is what makes this additive rather than a breaking change. Readers
that predate the set still consult the scalar: `internal/gateway/authz.go:90`
and `:106`, `client/src/player.ts`'s "your actors" filter, `vtt state dump`, and
MCP `get_state`. If the two disagree, someone silently gains or loses a
character.

**Rejected: "empty when shared."** It reads as the tidier rule — a single
controller in the scalar, blank when ambiguous — and it is wrong. protojson
omits empty strings, so a SHARED actor would be byte-identical on the wire to an
UNOWNED one, and empty already means DM/agent-only at `authz.go:90`. Every old
reader would silently reclassify a shared character as unowned. With
`controller_ids[0]` an old reader is *incomplete but never wrong*: it sees one
of the controllers instead of all of them.

*(The plan's Task 2 section described the rejected rule; it was amended to match
on 2026-08-07, Patrik's call, with the reasoning above recorded there too.)*

### 5.2 States that must be unrepresentable

The log is append-only, so a bad state written once is permanent. Three are
excluded by construction, in **both** folds:

- **An empty id in the set.** It would make the set non-empty while the mirror
  is `""` — exactly the shared-or-unowned ambiguity above — and revoke could
  never remove it, since removing it means naming an empty participant.
  Filtered at `ActorAdded`; `controlTarget` covers only grant/revoke, and
  `internal/gateway/convert.go` passes the client's `Actor` through verbatim,
  so the payload route is reachable from outside.
- **A self-contradicting payload** (`controller_id:"p-a"` with
  `controller_ids:["p-b"]`). The set wins. Erasure fails closed and a later
  grant recovers it; honouring a scalar the set contradicts hands someone a
  character nobody granted them.
- **Scalar drift from the set on any fold path**, not merely the control arms.

### 5.3 Authorisation

`grant_actor_control`: DM and agent only. `revoke_actor_control`: DM and agent,
plus a player naming **themselves** — you may put a character down, not take one
from someone else.

The ownership helpers change from "equals `controller_id`" to "is a member of
`controller_ids`" at `authz.go:90` and `:106`. An empty set keeps its present
meaning: DM/agent only.

### 5.4 What this does NOT change

The invite credential. `identity.CreateInvite(name, role, controls)` already
carries a `controls` list, and none of it moves — control granted at invite time
and control granted by event must simply agree on the same set.

## 6. Two folds, one rule

There are two implementations of every rule above: `internal/engine/apply.go`
and `client/src/fold.ts`. Both are compared against `scenarios/goldens` — by
`internal/harness` `TestFoldGoldenCorpus` and `client/test/fold-parity.test.ts`.

**Divergence between them is the defect class that matters most here**, because
each fold's own tests can be green while the two disagree. One was already found
and fixed during Task 2: no TS test granted control to an actor whose set was
EMPTY, so the mirror call could be deleted from the TS grant arm with every TS
test passing while Go failed. A DM granting a player their first character would
have produced `controller_id="p-player"` in Go and `""` in TypeScript.

Every load-bearing assertion needs fault-injection proof (ADR-009): delete the
guard, and a named test must fail.

## 7. Deliberately out of scope

1. **Auto-reconnect.** §3.4.
2. **Deprecating `controller_id`.** It is now derived, and eventually
   redundant. That is a decision to take once the set has lived a while, not on
   the day it ships — and it would be a breaking change, which ADR-007 makes a
   deliberate act.
3. **Per-actor presence** ("which character is someone looking at"). No
   requirement asked for it.
4. **`State.Actors map[string]*vttv1.Actor`.** That a projection holds wire
   types is why a presentation concern reaches the contract at all. It surfaced
   during this design; one sighting is not evidence, and it earns an ADR only if
   it bites again.

## 8. Demo

Two browser clients, one DM and one player. The DM grants a second character to
the player; the player moves both; the player's tab closes and the table sees
them leave; they reconnect and still control both. Screenshots at each beat —
Patrik often reviews from a phone.
