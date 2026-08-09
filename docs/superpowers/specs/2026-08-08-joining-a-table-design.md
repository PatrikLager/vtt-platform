# Joining a table — design

**Status: ACCEPTED — Patrik, 2026-08-09.** Written 2026-08-08 against decisions
Patrik made the same day, and approved the next. Nothing here is built yet; the
plan beside it carries the task breakdown.

## 1. The problem

Patrik, on the current invite model: *"that is a complex way of getting players
to join a game."*

Today a DM runs `vtt invite` once per person and sends each of them a private
link carrying their own 32-byte token. The security properties are good and this
design keeps every one of them:

- per-person revocation (`vtt revoke`, an `id`, nobody else re-keyed)
- roles bound to the credential rather than typed at connect
- tokens never stored — only `sha256` in `participants.token_hash`
- identity unforgeable; contrast MapTool, which keys ownership on a player-name
  STRING typed at connect, so typing someone's name makes you them
- character ownership surviving reconnects, because control is keyed to the
  participant id behind the token (§3.1a of the presence design)

The cost is ergonomic and it is real: N invites, N private channels, and a
re-invite for anyone who loses their link. Fine for a fixed group; painful for
"come play tonight".

## 2. The shape

**One shared join link. Per-person credentials minted on first use.**

The DM shares a single URL. A browser opening it with no stored token asks for a
display name; the server mints that person their own participant and token and
the client stores it. From that moment everything behaves exactly as today —
same reconnect story, same revocation, same ownership.

Four decisions, Patrik 2026-08-08:

| | |
|---|---|
| **Admission** | automatic — no waiting for the DM to be at a keyboard |
| **Role granted** | **spectator**, always. The DM promotes afterwards |
| **The link** | rotatable INDEPENDENTLY of the participants it has minted |
| **The door** | CLOSED by default; the DM opens it explicitly |

Each of those earns its place:

- **Automatic** admission, because an approval queue makes joining depend on the
  DM watching a screen, and the spectator default already contains the risk.
- **Spectator** because it is safe by construction. A stranger who finds the
  link can watch and nothing else — no token moves, no abilities, no narration.
  Compare admitting as a player, where anyone through the door can act before
  the DM has looked at them.
- **Rotatable** because otherwise a leaked link cannot be closed without
  re-inviting the whole table — reintroducing exactly the pain this removes, at
  the worst possible moment.
- **Closed by default** because a shared link is a public-ish endpoint that
  CREATES DATABASE ROWS. With the door shut the link is inert, so a leak is
  harmless outside the window and there is no standing endpoint to hammer.

**This is why there is no rate limiting, and its absence is a decision rather
than an oversight.** Rate limits bound how fast an open endpoint can be abused;
a closed door means there is no endpoint to abuse. If the door ever becomes
default-open, limits become required in the same change.

## 3. Promotion, which is the hard part

Q1 and Q2 together REQUIRE a way to change a participant's role. There isn't
one: `identity.CreateInvite` fixes the role at creation and nothing alters it
afterwards. Two things about the current code decide how this should work, and
both were verified rather than assumed.

### 3.1 Role must stay identity-side, not become an event

`internal/engine/state.go` and `apply.go` contain **zero** references to `Role`.
The fold knows nothing about authorization, and that separation is deliberate —
`internal/gateway` "imports engine.State only to answer the player-ownership
question".

Putting roles in the event log would drag an identity concern into the fold, and
worse, it would create a SECOND source of truth for authorization beside
`participants.role`. This repo has just spent a whole branch on what that costs:
`controller_id` mirroring `controller_ids` needed a documented invariant,
fault-injection proof on both folds, and a golden scenario before it could be
trusted. Authorization is not where to repeat that.

**So: a role change is an UPDATE to `participants.role`, in the same table the
token already lives in. One source of truth.**

### 3.1a How promotion reaches the wire

**Decided by Patrik 2026-08-09**, because §3.1 and §5 together left a real
tension: a `ClientCommand` is structurally *a thing that becomes an event* —
`TestEveryClientCommandConverts` enforces it — while a role change deliberately
produces no event.

**It is a `ClientCommand`**, with an entry in that gate's `notConverted`
allowlist beside `use_ability`, `load_adventure` and `retract_events`. The
alternative was an authenticated HTTP endpoint beside `/join`, which is more
honest about what promotion IS but sidesteps `commandRoles` — and one
authorization surface beats two. §5's matrix is where every "who may do what"
answer already lives, and splitting it is how a cell goes missing.

**`promote_participant` may target ONLY `player` or `spectator`.** A shared
join link mints spectators; letting promotion reach `dm` or `agent` would make
that link a path to full authority in two steps, which is precisely what
admitting-as-spectator exists to prevent. Minting a DM stays with `vtt invite`
— a deliberate, out-of-band act (§6.4).

The audit question — "who let this person act?" — is real and is answered
separately, not by relocating the data. Options for the spec to choose from:

- a narration event ("the DM made Kim a player"), which is already how the table
  records things it wants to remember, and costs nothing new; or
- an identity-side audit table, if the requirement is tamper-evidence rather
  than table-visible history.

The first is enough for a table. The second is a different requirement and
should be named as such before being built.

### 3.2 Role and connection are separate, and the code conflates them

**Rewritten 2026-08-09 on Patrik's call.** The earlier version of this section
treated "a promotion does not reach a live connection" as a constraint to work
around, and recommended closing the promoted participant's sockets so they
reconnect. That was wrong, and the reason it was wrong is worth keeping.

`handleWS` calls `identity.Verify` **once**, before the WebSocket upgrade, and
holds the resulting `*identity.Participant` for the connection's entire life.
Every `Authorize(p, cmd, st)` reads that one struct. Nothing re-reads the
database. So the server answers "who is this, and what may they do?" once, at
connect time, and treats the answer as fixed.

**Authentication is a connection-time fact. Authorization is a live one.**
Conflating them is the defect; a reconnect is not a fix for it, it is a way of
paying for it.

Two things follow, and the second is the serious one:

1. **Every joiner arrives as a spectator** (§2), so a reconnect-to-promote
   would sit on the critical path of *everybody who ever joins*, immediately
   after they join. The shared link would be more cumbersome than the
   per-person invites it replaces. The feature would defeat its own purpose.
2. **`vtt revoke` does not remove anybody.** Verified 2026-08-09: the only
   `Verify` in the WS path is at connect, so a revoked participant keeps
   playing — moving tokens, using abilities — until they choose to disconnect.
   Throwing someone out of your table currently does nothing until they
   cooperate. This predates the joining work and is the same root cause.

**So: the participant is re-resolved per command, not cached for the
connection.** A role change and a revocation both take effect on the very next
thing that person does. No reconnect, no dropped socket, no waiting.

The objection is cost — a database read on the hot path of every command, and
`Authorize`'s doc comment says it does no I/O. That comment describes the
current design, not a law, and the read is a local SQLite lookup by primary
key. It is to be MEASURED rather than argued about, and the measurement
recorded here; if it is genuinely too expensive, a short-lived cache with an
invalidation hook is the fallback, not a reconnect.

## 4. What gets built

- **Campaign-scoped operational state**: a join secret and an open/closed flag.
  Neither belongs in the event log — they are operational, like presence, and
  replaying a campaign must not reopen a door. `internal/identity`'s SQLite is
  the natural home; it already holds the other credential state.
- **A join endpoint**: given the secret and a display name, mint a participant
  with role `spectator` through the existing `CreateInvite` path and return the
  token. Reusing that path is what keeps §3.1a true — same person, same
  participant id, characters return on reconnect.
- **Role promotion** per §3.
- **DM console**: open/close the door, show and rotate the link, promote a
  spectator. Promotion belongs beside T7's "Who controls what" panel, so
  promoting and assigning a character are one place rather than two.
- **Client**: a join view for the display name, then store the returned token
  exactly as the invite flow does — `localStorage`, stripped from the URL.

## 5. Authorization consequences

- A self-minted spectator **must not be able to promote itself**, or the
  spectator default is decoration. That is a new cell, and `commandRoles` is
  literal, so the matrix grows from 60.
- Opening/closing the door and rotating the link are DM-and-agent only, by the
  same argument that gates `grant_actor_control`.
- The join endpoint is **unauthenticated by construction** — that is its whole
  point — so it must be impossible to reach any other capability through it.
  It mints a spectator and returns a token; it does nothing else.

## 6. Deliberately out of scope

1. **Rate limiting.** §2 — the closed door removes the endpoint, and this
   becomes required the moment the door defaults open.
2. **A QR code.** Obvious once one link exists, and purely additive.
3. **Self-service display-name changes.** A joiner names themselves once; a
   rename is a different feature with its own impersonation questions.
4. **Removing `vtt invite`.** It stays. Minting a DM or an agent is not a
   thing a shared player link should do.
