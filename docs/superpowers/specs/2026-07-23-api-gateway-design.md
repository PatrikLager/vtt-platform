# API Gateway & Permissions — Design Spec (sub-project 3)

**Date:** 2026-07-23
**Status:** Approved design (brainstorming output)
**Parent:** Platform spec §4.2/§4.5; pillars P1 (API-first), ADR-001/005/007
**Consumes:** `internal/campaign` (sub-project 2), `vtt.v1` contract (sub-project 1)

## 1. Purpose

The gateway is the platform's only door (P1): a WebSocket/HTTP surface over
`internal/campaign`, role-gated per participant, plus the `vtt` CLI binary
shell. After this sub-project, a human client, a headless harness, and the
LLM's MCP tools all drive a live table through the same wire API.

## 2. Decisions (locked in brainstorming)

1. **Ownership = `controller_id` on Actor** (additive contract change): the
   participant who may act as that actor; empty = DM/agent only.
2. **Auth = DM-minted invite tokens:** random secrets, stored hashed with
   role + controlled actors; revocable; no accounts. The agent joins with a
   token whose role is `agent`.
3. **ckeletin verdict (ADR-008): adopt the pattern, not the framework layer.**
   cobra with ultra-thin commands (run functions ≤30 lines delegating to
   `internal/`; viper DEFERRED until a config file exists — see ADR-008); no
   updateable `.ckeletin/` layer (our Taskfile/gates/ADRs already exist — a
   second framework-owned copy would duel with them).
4. **Append stays synchronous** (carried question resolved): `CommandResult`
   returns the assigned sequence; sync gives the LLM read-your-writes.

## 3. Wire protocol

- One WebSocket endpoint `/ws?token=<invite-token>`; protojson TEXT frames
  (wire-conventions doc governs; binary is a later optimization).
- Frame kinds (new contract messages, all additive):
  - `ClientCommand{request_id, oneof command}` — client→server
  - `ServerFrame{oneof: result CommandResult | event Envelope |
    catch_up_head CatchUpHead}` — server→client; the oneof key is the frame
    discriminator (same compiler-checked pattern as the envelope payload), so
    clients never sniff fields. `CommandResult{request_id, ok, error,
    sequence}`; `Envelope` frames carry the broadcast stream (every accepted
    event, to every connected participant, spectators included)
  - `CatchUpHead{head_sequence}` — **amended 2026-08-02** (Patrik approved;
    landed with the `fix/state-dump-truncation` branch). Sent ONCE, as the
    FIRST frame on every connection, naming the highest sequence the server
    has already queued as that connection's catch-up backlog. Sent
    unconditionally, including `head_sequence = 0` for an empty log, so a
    client never has to interpret "no frame yet".

    Why this was not in the original design: the spec assumed a client could
    tell catch-up from live traffic, and it cannot — backlog streams straight
    into live broadcast down one channel with no boundary. `vtt state dump`
    therefore guessed, stopping after 300ms of wire silence. Mid-replay that
    gap is ordinary, so the dump could print a SILENTLY INCOMPLETE state,
    from the command whose output the golden corpus and the TypeScript
    fold-parity keystone are compared against. The server always knew the
    number — `Store.Subscribe` preloads the whole backlog under its lock —
    it was simply never told to anyone.

    `catch_up_head` is the ONE frame that is positional by definition; that
    is what makes it usable as a boundary. Everything else in the ordering
    contract below still holds.
- Commands v1 (mirror the lifecycle 1:1): `MoveToken` (existing message) +
  new `CreateScene`, `AddActor`, `PlaceToken`, `StartSession`, `EndSession`,
  `RetractEvents`. Each is validated, role-gated, converted to its event,
  appended synchronously; result carries the sequence (RetractEvents
  included, as of a P6 pre-step; was a carry-forward).

  *AMENDED 2026-08-30 — `RetractEvents` is not a command any more.* Patrik's
  ruling of 2026-08-30 removed retraction from the platform; the message and its
  `ClientCommand` arm left the contract in `59542e1` and the handler left the
  gateway in `5396338` (sub-project 13,
  `2026-08-30-retraction-leaves-design.md`). It was also the one command that did
  NOT convert to its event the way this bullet describes — `ToEvent` returned a
  sentinel and `campaign.Undo` built the marker — so its removal makes the
  "mirror the lifecycle 1:1" claim truer than it was. The command surface has
  grown well past v1 since; `internal/gateway`'s `commandRoles` table and
  `TestEveryClientCommandConverts` are the live enumeration.
- Catch-up: client sends `after_sequence` at connect; server streams history
  then live (store.Subscribe semantics, buffer 256 — a named constant;
  overflow = WebSocket close, client reconnects and catches up).
- **Server-initiated ping/pong keepalive** — **amended 2026-08-26** (Patrik
  approved; found by session zero, not by reasoning). The server sends an
  unsolicited WebSocket PING control frame on a connection that has been
  otherwise silent for 20 seconds, and force-closes it if no PONG comes back
  within 60 seconds.

  NO CLIENT CODE IS REQUIRED, which is what lets this be added to a shipped
  contract without breaking anyone. A WebSocket implementation answers a ping
  below application level: a browser's stack replies without the page's
  JavaScript being involved, and the JS `WebSocket` API cannot send or suppress
  a ping frame at all. Every client here — browser, harness, MCP, CLI — already
  satisfies this, as does any conformant third-party client. What a client
  author must know is the CONSEQUENCE: stop answering and you are closed, with
  no close frame.

  WHY IT EXISTS. Neither existing deadline watches an idle connection.
  `store.SubscriberNoProgressTimeout` is armed only while an event waits to be
  handed over, so on a quiet table it never starts; `Server.writeTimeout` bounds
  a write, and an idle connection performs none. Both ask whether a client is
  READING — the right question, and no answer at all about a socket where
  neither side has anything to say. So an idle connection carried zero bytes in
  either direction indefinitely, and every hop between a player and the server
  (Cloudflare, carrier NAT, a home router's connection table, a corporate proxy)
  was entitled to reap it. In session zero a browser left idle came back reading
  `closed`. Patrik's requirement: "you have to be able to be 'inactive' without
  being kicked out."

  20s AND 60s, AND THE RATIO IS THE POINT. Both come from MapTool
  (net.rptools.clientserver), which runs a 20s heartbeat against a one-minute
  socket timeout; 3x means two pongs may be lost or arrive late before anyone is
  declared dead. Direction of error matters far more than speed of detection: a
  budget tight enough to lose a race with a phone on a slow cellular link reaps
  players who are perfectly fine, and a spuriously reaped player sees exactly
  what a genuinely disconnected one sees. Patrik's call, 2026-08-26, against 40s
  and 20s alternatives. The cost of that generosity is bounded: a peer that has
  genuinely gone stays listed 60-80s, holding one idle socket and one presence
  entry. Note the borrowing is of INTERVALS, not direction — MapTool's heartbeat
  is client-side and ours is server-side, per the paragraph above.

  THE CLOSE IS UNCEREMONIOUS, deliberately. A missed pong closes with no close
  frame and no reason, unlike revocation, which sends one. There is nobody left
  to tell: a peer that did not answer a ping will not read a close frame either,
  and a graceful close would first sit through its own handshake timeout. The
  connection then unwinds through the ordinary path, presence departure
  included — the half a table actually notices, and the half that was missing
  before. Until this landed, a peer that had silently gone stayed CONNECTED
  forever, because departure hangs off the connection handler returning and the
  handler was parked reading a socket nobody had told it was gone.

  WHAT IS NOT PROMISED. The ping is skipped entirely while the connection's
  writer is mid-frame, so a client receiving a burst may see no ping for well
  over 20 seconds. That is deliberate — a socket carrying data does not need a
  keepalive, and pinging into a busy writer makes the ping contend for the
  library's frame lock. **The interval is not a clock, and nothing may be
  inferred from a ping that does not arrive.**

  ONE KNOWN GAP, recorded rather than closed: whether a mobile OS keeps
  answering control frames for a FROZEN tab is unverified. If it does not, a
  frozen tab is reaped after ~80s — which is precisely one of the ways a player
  is "inactive". Worth a real-device check before anyone relies on background
  tabs surviving.

  NOT A CONTRACT CHANGE. Ping and pong are WebSocket control frames, not
  protobuf messages, so ADR-007's additive-only rule is not engaged and
  `check:breaking` has nothing to compare. Said out loud because "wire protocol
  change" normally implies a contract change in this repo, and this one is not.
- HTTP: `GET /healthz` only. (The static-file stub originally sketched here
  was dropped as YAGNI at build time; the browser client is sub-project 7's.)

## 4. Roles & authorization

One authorization TABLE (data, not scattered code), checked in exactly one
gateway function:

| Command | dm | agent | player | spectator |
|---|---|---|---|---|
| MoveToken | ✓ | ✓ | only own-controlled actors' tokens | — |
| CreateScene/AddActor/PlaceToken | ✓ | ✓ | — | — |
| StartSession/EndSession | ✓ | ✓ | — | — |
| ~~RetractEvents~~ **(removed 2026-08-30 — see below)** | ✓ | ✓ | — | — |
| (receive event stream) | ✓ | ✓ | ✓ | ✓ |

*AMENDED 2026-08-30.* The `RetractEvents` row above is history: no role may
retract, because there is no `retract_events` to authorize. The handler and its
authorization entry left in `5396338`; the message left the contract in
`59542e1` (sub-project 13, `2026-08-30-retraction-leaves-design.md`).

Flagged in the row itself rather than only in a note, because this table is the
design of `internal/gateway`'s `commandRoles`, and later specs lean on that
single table being the only place a "who may do what" answer lives —
`2026-08-08-joining-a-table-design.md` §3.1a routes promotion through a
`ClientCommand` for exactly that reason, "one authorization surface beats two",
and §5 there grows the same matrix rather than starting another. A stale row
here is therefore a stale answer in more places than this document.

The rows are also no longer the whole table: the command surface grew well past
v1 with the ruleset, adventure, map, presence, join and removal arcs.
`commandRoles` is the live enumeration, and its table-driven test asserts every
command × role cell literally rather than deriving expectations from the map
under test.

Invite management (`vtt invite`/`vtt revoke`) is CLI-only, DM-side — the
agent can never alter who is at the table.

Every accepted command stamps `actor_role` AND a new `participant_id` field
on the Envelope (additive contract change) — the log records who, forever;
this is what makes the agent auditable.

## 5. Identity & invites

- Campaign DB gains a `participants` table (managed by `internal/identity`):
  id, display_name, role, token_hash, revoked.
- **Deliberately NOT event-sourced:** identity is infrastructure, not game
  history — revocation must not be undoable via game-log mechanics. *(2026-08-30:
  the decision stands and its stated reason no longer bites — nothing in the log
  is undoable now that retraction is gone. The live reason is the one
  `2026-08-08-joining-a-table-design.md` §3.1 gives: a role is an UPDATE to
  `participants.role`, in the same table the token lives in, one source of
  truth. `internal/gateway/convert_test.go`'s `notConverted` entry for
  `promote_participant` says the same thing where the code can see it.)*
- `vtt invite --campaign f.db --name Lera --role player` prints the
  one-time-shown token; `vtt revoke` flips the flag.

**CORRECTED 2026-08-24.** The table used to carry a `controls` column and the
invite a `--controls` flag, both listed here as though they conferred control.
They never did: nothing updated the column after invite time, no
`ActorControlGranted` was ever emitted from it, and nothing that decided
anything read it — authorization and the roster read `controller_ids` from the
event log. A DM who invited someone "controlling act-lera" was told by
`/api/me` that they did, and they did not.

Both are removed. **An invite confers a seat and a role, and nothing else.**
Control is conferred separately, by a grant, which also declares whether the
character is a party member — see the visibility spec §5.1. That separation is
deliberate: an invite can only ever express the FIRST assignment, while a player
leaving, a DM covering for an absent one, or a creature changing hands are all
grants regardless.
- Token: 32 random bytes, stored as SHA-256 hash; compared constant-time at
  WebSocket connect; connection carries the participant identity thereafter.

## 6. The `vtt` binary

`cmd/vtt` (cobra): `serve --campaign <file> --addr <:port>`, `invite`,
`revoke`, `version`. Ultra-thin commands; all logic in `internal/`.
One campaign per serve invocation (YAGNI: multi-campaign management is a
later concern). Recorded as ADR-008.

## 7. Carry-forward resolutions (from sub-project 2's ledger)

1. **Notify-before-live-apply ordering — fixed at the source:** store gains
   append-without-notify + explicit `Notify`; campaign calls Notify AFTER
   advancing the live projection. A subscriber that sees event N can always
   read State ≥ N. Store/campaign tests updated accordingly.
2. **Shared-envelope immutability:** each connection's pump marshals its own
   protojson frame from the envelope (per-connection marshal — adjudicated at
   build time over a shared marshal-once cache, which leaked unboundedly;
   stateless and table-scale cheap, revisit with a broadcast hub only if
   client count grows); consumers never mutate the shared pointer.
3. **Subscribe buffer:** gateway uses 256 (named constant, commented).
4. **Sync-vs-async Append:** sync (see §2.4).

## 8. Contract additions (all additive, through the armed gates)

- `Actor.controller_id` (new field)
- `Envelope.participant_id` (new field)
- `ClientCommand`, `ServerFrame`, `CommandResult`, and the six new command
  messages
- toolgen manifest grows six entries (every command message) — after this
  sub-project the MCP tool set can run an entire table session.

## 9. Packages & enforcement

- New: `internal/gateway` (WS/HTTP server, authz table, frame codec),
  `internal/identity` (participants/invites), `cmd/vtt` (cobra shell).
- New deps (pinned exact at implementation, recorded): cobra, viper,
  a WebSocket library (coder/websocket or gorilla — chosen at plan time by
  maintenance status), golang.org/x/crypto only if needed.
- Arch-lint: `gateway → {campaign, identity, engine, contract}` (engine for
  the ownership check's State type); `identity → {}`
  (own DB access); `cmd → {gateway, identity, campaign, contract}`; nothing
  imports `cmd`. Vocabulary gate unchanged (scans the new packages
  automatically).

## 10. Testing & exit criteria

- Gateway tests run a REAL server on a random port with real WebSocket
  clients — no mocks of our own code.
- Authz table: table-driven test covering every command × role cell.
- Identity: invite/verify/revoke round-trip; constant-time comparison;
  revoked token rejected at connect.
- **Exit scenario (sub-project 4's harness seed, one layer up):** two human
  participant tokens + one agent token replay the event-core exit scenario
  over live WebSockets — including a player denied moving another's token,
  a spectator denied everything, and the agent performing a retraction —
  ending with state equality across all three clients' views after
  reconnect+catch-up. *(2026-08-30: the retraction leg is gone from
  `scenarios/three-role-exit.json` — the agent has no such command — and every
  other leg, including the state-equality ending, still runs on every
  `task check`.)*

## 11. Non-goals (YAGNI)

No TLS termination (home LAN/VPS reverse-proxy later; token secrecy noted as
transport-dependent in README). No multi-campaign serving. No rate limiting.
No MCP server itself (sub-project 6 — but its tool definitions are generated
now). No browser client (sub-project 7). No password auth, no accounts.

## 12. Open questions (deferred, with owners)

- WebSocket library choice: RESOLVED — `github.com/coder/websocket` v1.8.15.
- Whether `RetractEvents` needs a player-visible confirmation flow — a UX
  question for sub-project 7; the API just executes. *(CLOSED 2026-08-30, by
  removal rather than by answer: there is no `RetractEvents`. The question's
  premise — that a player might need telling their history had changed under
  them — is the objection that ended the operation.)*
- `Envelope.session_id` stamping: RESOLVED (Patrik, 2026-07-24) — the
  CAMPAIGN stamps it under its lock at Append from the currently-open
  session (single authority, no race window, same pattern as sequence
  stamping). Implemented as a small fix task on the next branch; wire events
  logged before that fix keep an empty session_id (no real campaigns yet).

## 13. Amendment (2026-07-25, sub-project 5a merge)

The §4 authz table gains two rows for 5a's new commands:

| Command | dm | agent | player | spectator |
|---|---|---|---|---|
| UseAbility | ✓ | ✓ | only self-controlled actors (via `Actor.controller_id`) | — |
| RemoveCondition | ✓ | ✓ | only self-controlled actors (via `Actor.controller_id`) | — |

Same ownership check as `MoveToken`. The table now covers 9 commands × 4
roles = 36 literal cells. `CommandResult` for `UseAbility` carries the
FIRST sequence of the atomic batch (5a's `AppendBatch`), not a single
event's sequence.
