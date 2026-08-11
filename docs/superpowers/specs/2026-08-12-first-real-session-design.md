# First real session — design

**Status:** approved by Patrik, 2026-08-12
**Kind:** experiment, not a feature. The deliverable is a ranked, evidenced
backlog. No code is *planned* — but §3 governs what happens when the session
turns something up, and what it says is not "nothing".

## 1. Why this, and why now

The platform was built foundations-first, deliberately before any session was
played. ADR-006 names the risk it was hedging — *"building a full session's
worth of features before any API is exercised end-to-end risks discovering
architectural problems only after a lot of UI and content work is sunk into
them"* — and the hedge was the simulation harness, built alongside the
foundations rather than after.

The hedge worked, and it has a limit. **Every foundation was validated by a
harness we wrote.** A harness inherits its author's assumptions about what a
session looks like; a real table does not. Eight sub-projects were built against
that inheritance and none of them has met a player.

The founding spec's build order is otherwise complete. Sub-projects 1–8 all have
specs and merged branches. Sub-project 9, *"Asset service & AI content
generation"*, has neither, and the stated foundations exit criteria — a full
simulated combat encounter through the real API, the toy ruleset passing
conformance, `task check` green with all Section 8 gates active — are met. The
next arc is therefore a genuine choice rather than the next item on a list, and
the candidates the spec itself names (asset service, full 4.5e ruleset, voice
pipeline) are all bets about what a table needs.

**A session decides between them with evidence instead of taste.** That is the
whole argument for doing it first: it is the cheapest arc on the list and the
only one that makes the others' priorities knowable.

## 2. Non-goals

Stated explicitly, because each is a plausible thing to start doing halfway
through:

- No autonomous DM loop (see §5).
- No asset service, no 4.5e content work, no client rework.
- No new subsystems of any kind.

Every one of those is a candidate the session might *justify*. That is the
point. None of them is in this arc.

## 3. The rule that keeps it honest

**Anything that must be fixed to make the session happen is recorded as a
finding FIRST, then fixed properly** — full TDD per ADR-009, reviewed,
`task check` green, integrated into the engine like any other change.

No workarounds. No "temporary" patches to get the evening running (Patrik,
2026-08-12, amending an earlier draft that said "fixed minimally"). A
temporary fix would corrupt the experiment twice over: it hides the finding,
and it means the session measures a platform that does not exist.

**The consequence is that the date moves, not the standard.** If session zero
turns up something real the evening before, the fix takes its normal cycle and
the game is rescheduled. Friends should be invited on that basis rather than
told a fixed date.

This is the same discipline the enforcement layer already runs on: a gate is
never weakened to pass it, and "verified" without a measurement is a claim
rather than a fact.

## 4. Shape: two sessions

### 4.1 Session zero — the dry run

Patrik and the agent, no guests, about an hour. Not a rehearsal of the story; a
check that the machinery is real.

**THROUGH THE TUNNEL, not localhost** (corrected 2026-08-12, after the first
draft said localhost). The gateway calls `websocket.Accept(w, r, nil)`, and
coder/websocket's default policy is `accept.go:239` — the browser's `Origin`
host must equal the `Host` header the server sees, with no fallback patterns
because the options are nil. On localhost those always match. Through a tunnel
they match only if cloudflared preserves the original `Host` rather than
rewriting it to the origin URL, which is config-dependent and cannot be settled
by reading.

So a localhost dry run would pass cleanly and derisk NOTHING about the
transport session one uses, and the first person to discover whether the
upgrade survives a tunnel would discover it with guests waiting. Session zero
runs against the same `wss://…trycloudflare.com` origin the guests will use.

If the upgrade is refused, §3 applies: it is a finding, and the fix is a real
one — an explicit allowed-origin configuration on the server, with the security
reasoning written down — not a nil-options shortcut or a flag that turns the
check off.

It walks the whole path once: serve a campaign, seat the agent over MCP, open
the door, take the join link, join as a second participant in a browser, get
promoted, be granted an actor, move a token, have the DM narrate, disconnect
and reconnect.

Two things it must answer that nothing has ever tested:

- **Is the client usable on a real handheld?** `client-design.md` says
  tablet-first responsive. The DM console and the join view have only ever been
  driven by tests and by their author. Session zero uses **one computer and one
  iPad** (Patrik, 2026-08-12) — real devices, not browser emulation, which
  catches layout but not touch targets or a mobile browser's own behaviour.
- **How long does joining actually take**, from link sent to seated player?

The output is a list, not a verdict. Anything that needed a human to notice,
explain or work around goes on it — *including things that worked but were
awkward*, because those are the ones memory discards before session one.

**Done when the whole path completes without a workaround** — either nothing
broke, or what broke was fixed properly and the walk was repeated. Not when we
have "seen enough".

Session zero also chooses the adventure: whichever of `goblin-ambush` or
`cellar-rats` it gets through cleanly, on `dnd45e-minimal`.

### 4.2 Session one — the real test

The LLM DMs. Patrik and friends play.

**Topology.** `vtt serve` on Patrik's machine; a **cloudflared quick tunnel**
(`cloudflared tunnel --url http://localhost:8080`) supplies an external
`wss://` origin. Chosen because guests install NOTHING — they open a link — and
it needs no account on Patrik's side either. Its URL is fresh per run, so the
link is minted on the night; a tunnel that restarts mid-session invalidates
every shared URL, though tokens already issued keep working. Guests open that origin in a browser on whatever device they
have. The agent runs `vtt mcp --server wss://…`, seated as the agent
participant and judged by the same authz table as every other client.

A tunnel rather than hosting is deliberate: reachability is the only hard
blocker in the arc, and a VPS with TLS is setup whose value depends on answers
this session has not produced yet.

**Length: ninety minutes, one encounter.** Short on purpose. The goal is
evidence, and a long evening mostly produces more instances of the same
findings.

**The opening sequence is itself under test.** The DM opens the door with an
admission budget set to the expected number, shares the link, and each player
lands as a **spectator** who must then be **promoted** and **granted an actor** —
two deliberate acts by the DM per player, while everyone else waits. That is
the design working as specified (joining-a-table §2: spectator by default,
because it is safe by construction). It is also the first real measurement of
what that safety costs in attention and elapsed time at a live table.

**Known asymmetry to watch:** the DM console builds the share URL from the
browser's own origin, so it is tunnel-correct automatically, while
`vtt join-link show` prints a `<your-server-url>` placeholder a human
substitutes. Not a defect; a candidate finding if it trips anyone.

## 5. Roles, and the gap that shapes them

**Patrik drives the DM and also plays** (his call, 2026-08-12).

Driving means having the MCP host open and prompting the agent between beats.
It is necessary because **MCP is poll-only** — `internal/mcp/server.go:45`, in
the code's own words: *"There are no push notifications here; you must poll."*
The MCP server accumulates the table's events into history, but nothing wakes
the model. Players act, events land, and the DM sits idle until prompted.

The *tool* surface for an LLM DM is complete. `get_join_link` and
`get_participants` (#45) closed the last gaps — before them an agent could open
a door and not tell anyone the URL, and could promote a participant only by
supplying an id it had no way to learn. What is missing is not a capability but
a **drive**.

Building that loop is the obvious candidate arc, and it is deliberately **not**
this one. Every decision inside it — when a DM should speak rather than wait,
how it avoids narrating over players, what stops a runaway — is currently a
guess. Session one's nudges are the requirements for it, written by a real
table instead of by us.

**The cost of Patrik driving** is recorded rather than waved away: he is behind
the curtain, so his own answer to "was it fun" is not clean data. The debrief
leans on the friends.

## 6. Evidence

Three streams, two of them free.

**The event log.** Lossless and already there: every action in order with
sequence numbers. It gives the mechanical record and the timing — how long the
join took, where play stalled, what the DM actually did as against what it said
it did. The campaign file is kept afterwards; it replays.

**The driver's transcript.** Patrik's MCP conversation, verbatim. Every nudge,
in order, without anyone writing anything down mid-game. This is the arc's most
valuable artifact and it costs nothing but remembering to save it.

**The debrief**, the only part needing deliberate effort, conducted **at the
table while people are still there**. A form sent the next day gets no answers.
Four questions, five minutes:

- What confused you?
- What were you waiting for?
- What did you try to do that you couldn't?
- Would you play again?

The third is the sharpest: it surfaces missing capability rather than defects,
which is what decides the next arc.

## 7. The deliverable

A written backlog. **Each item cites at least one stream** — a sequence range, a
transcript line, or a named person's words. Ranked by what it cost the table:

1. **Stopped play** — the table could not continue without intervention.
2. **Cost time** — play continued but people waited.
3. **Awkward** — it worked; nobody would choose it.

Items with no evidence do not go in the backlog. They go in a separate
**suspicions** list, so they are not lost and not acted on either. This
separation is the point of the arc: the platform has repeatedly been surprised
by things that were believed rather than measured, and a backlog that mixes the
two inherits that problem.

## 8. What could go wrong

- **Nothing reaches the server.** Most likely failure, and the reason session
  zero exists. Tunnel setup is verified end-to-end from a device that is not on
  the local network before guests are invited.
- **Session zero finds something real.** Then §3 applies: record, fix properly,
  reschedule.
- **The agent stalls constantly.** Expected to some degree — §5 — and the
  frequency is data, not failure. If it stalls so badly that the table cannot
  play, that is itself the finding, and the session is ended early rather than
  puppeteered into looking successful.
- **The evening produces one enormous finding that masks everything else** (for
  example: unusable on a phone). Acceptable. One measured blocker is worth more
  than a list of guesses.

## 9. Exit criteria

- Session zero's walk completes without a workaround.
- Session one is played, or abandoned for a recorded reason.
- The backlog exists, every item evidenced, ranked by cost to the table.
- The campaign file and the driver transcript are kept as artifacts.
- The next arc is chosen **from that backlog**, not from §1's list of
  candidates.
