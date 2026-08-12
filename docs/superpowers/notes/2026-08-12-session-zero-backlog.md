# Session zero — the ranked backlog

The deliverable of the first-real-session arc (spec §7).
Spec: `docs/superpowers/specs/2026-08-12-first-real-session-design.md`
Findings, with full reasoning and verification: `2026-08-12-session-zero-findings.md`

**Ranked by what it cost the table**, per spec §7:

1. **Stopped play** — the table could not continue without intervention.
2. **Cost time** — play continued but people waited.
3. **Awkward** — it worked; nobody would choose it.

**Every item cites a stream** — an event sequence range, a line from the
driver's MCP transcript, or a named person's words. Items with no evidence are
not here; they are in **Suspicions** at the bottom, recorded so they are not
lost and not acted on either.

Session zero was Patrik and the agent, 2026-08-12, through a cloudflared quick
tunnel, one laptop and one iPad. No guests. Nineteen findings; five of them are
passes.

---

## 1. Stopped play

### S1. There is no hidden information anywhere in the platform

**Evidence.** Driver transcript, unprompted, before anything went wrong:
*"Asme sees the goblin positions in the shared log the moment they look, since
the ambush setup at seq 12–13 is already written there. The ambush is a
surprise to the character, not to the player."* Then seq 15–21: seven token
moves in ~90s, **seq 20 placing `tok-fighter` on (19,8) — the Goblin Archer's
exact square**. Patrik, asked how: *"yes, I could see the goblin token on the
board — there is no fog/limited view mechanism."*

**What it is.** Not unimplemented — **unmodelled**. `grep` for
hidden/visibility/fog across `contract/vtt/v1/*.proto` returns nothing.
`client/src/view/grid.ts:68` filters tokens on `SceneID` alone. The gateway's
pump sends identical bytes to every connection; role gates what you may WRITE,
nothing gates what you read.

**Why it stops play.** Every ambush, trap, secret door, monster HP total and
sealed DM note is disclosed to every participant by construction. It makes a
class of content unrunnable rather than unpleasant.

**Shape of the work: an arc, not a fix.** Touches the contract (visibility on
tokens/notes/events), the fold, the gateway's fan-out — which today has no
per-recipient filtering at all — catch-up, and the client. It collides directly
with "replay is the source of truth": if the server filters per recipient, two
participants no longer share one log. That collision needs designing, not
patching.

**Finding 14.**

### S2. An idle connection is reaped, and nothing keeps it alive

**Evidence.** Patrik: *"why did it disconnect, was it because I was inactive or
because the network went down — you have to be able to be 'inactive' without
being kicked out."*

**What it is.** Nothing pings anywhere. Our own deadlines are innocent and were
checked rather than assumed: `store.SubscriberNoProgressTimeout` is armed
inside the select that hands an envelope over
(`internal/store/subscribe.go:152-163`), so it only runs while an event is
waiting; `writeTimeout` bounds a write, and an idle connection performs none.
So an idle connection carries zero bytes and any intermediary may reap it.
Compounded by manual-only reconnection (client spec §3.4) and tabletop pacing,
where silence is the normal state.

**Status: a fix was attempted, reviewed, and REJECTED.** Parked in
`git stash@{0}`; findings on task #71. It reaped healthy clients:
coder/websocket wraps every control frame in a hard-coded 5s context
(`write.go:277`), so a ping merely losing the frame lock to a busy writer read
as peer death. **A ping is a verdict about the pong, never about our own
writer.** Redo properly, and settle the open question below.

**PATRIK'S CALL REQUIRED.** How long may a pong take before we declare someone
gone? Generous leaves ghosts at the table longer; tight reaps players who are
merely asleep — a frozen phone tab is indistinguishable from a dead peer.
The literal requirement is "be inactive without being kicked out".

**Findings 7, 18.**

### S3. `goblin-ambush` cannot be played as an adventure

**Evidence.** Driver transcript, unprompted: *"there's no second PC in this
adventure to hand over. goblin-ambush shipped one player actor."* And
`adventures/goblin-ambush/scenes/ravine.json`: all three tokens on row 0 —
(0,0), (1,0), (5,0) — on a 32×32 grid, fighter adjacent to a goblin, nobody
concealed.

**What it is.** A conformance fixture with a `goldens/` folder, doing its job
correctly. It is not content. One player actor means a table of four has
nothing to give three of them, and the tokens are at placement defaults rather
than in ambush positions.

**Why it stops play.** Session one as currently planned cannot be run. This is
the only adventure the walk exercised, and `cellar-rats` has not been checked
for the same shape.

**Shape of the work: content, not engine.** Stage an adventure FOR PLAY. Also
check `cellar-rats` before assuming it differs.

**Findings 6, 16.**

---

## 2. Cost time

### T1. The board has no camera, so it does not fit the screen

**Evidence.** Patrik: *"the board is too big… I can not see the controls when
looking at the board, I need to scroll down to see them"* — then, correcting a
misfiling, *"The issue I described was on the laptop."* And: *"why is not the
board size aligned to the screen — I think that is how RPTool does it, it
follows the screen."*

**What it is.** `spectator.ts:84-85` sizes the board at `gridWidth × CELL` by
`gridHeight × CELL`, `CELL = 44`, so a 32×32 scene is **1408×1408 px**, with
`.player` placed in the grid row beneath it — controls begin about 1450px down
the page against 800–1000px of laptop viewport. There is no zoom, pan, camera,
scale or fit anywhere in the client, and the spec mentions none of them. The
board is not a view of a world; it IS the document, at 1:1.

`CELL = 44` is a bare constant with no doc comment, in a codebase where every
other constant carries a paragraph — an undecided default that survived because
every test scene is 10×10 and fits anything.

**Shape of the work.** Smaller than it looks: `cellFromPoint` (`grid.ts:52`)
already divides by `geom.cell` rather than a constant, so hit-testing stays
correct the moment `CELL` becomes computed. Fit-to-width threads an existing
seam. Zoom/pan is bigger, uses the same seam, and is what makes scene size and
screen size independent.

**Desktop only.** Patrik: *"lets get the functionality and look to work on a
computer/laptop, then we can migrate that to a tablet/phone design."*

**Findings 17, 19.**

### T2. Nobody can tell who is actually at the table

Three independent mechanisms, one symptom. **The convergence is the finding** —
none looked serious alone.

**(a) The LLM DM is blind to presence.** `get_participants` reads
`/api/participants`, which calls `s.ids.List()` and never touches `s.presence`
(`internal/gateway/metadata.go:369-391`). It is an invitation list, not an
attendance list. With the iPad seated it returned **ten names for three
connections**. There is no other route: MCP has no presence tool, and presence
deliberately never enters the event log (spec §4), so the one channel the agent
polls cannot carry it.

**(b) The human list is unreadable.** Observed: `ArmakAsmeDM`. `.status` has
`display:flex; gap:16px`, but that gap applies to the status bar's own children;
`.present` and `.participant` have **no CSS rule at all** — bare inline spans,
no whitespace, no styling. Three short names is a puzzle; six is unreadable.

**(c) Arrivals and departures are nowhere.** `buildFeed`
(`client/src/view/feed.ts:32`) takes `Envelope[]`, and presence is not an event.
No record of who came or went, anywhere.

**Add S2** and the table-level result is: players drop silently, the DM cannot
see it, dead clients linger as CONNECTED, and there is no history to check.

**Shape of the work.** One change covers (b), (c) and T3: a participant panel,
one row per person with name, **role** and connection state — Patrik's RPTool
prior art, *"each player/spectator as its own 'thing' in a sidebar."*
Client-only; presence frames already carry it. (a) needs a presence-aware MCP
tool or route, which is a separate, small piece.

**Findings 10, 13, and 7.**

### T3. A spectator gets a board with no controls and nothing that says why

**Evidence.** Patrik, seated: *"I do not have any buttons, not on laptop browser
or ipad browser. Maybe because I am a spectator?"* — **the platform's author,
from two devices, phrasing it as a question.**

**What it is.** The behaviour is correct (`player.ts:70`: *"A spectator acts as
nobody"*). Nothing communicates it. `renderStatus` (`spectator.ts:192-223`)
shows connection state, session name and present names — never **your own
role**, never what happens next. The participant list carries no roles, so a
newcomer cannot tell who the DM is, i.e. who to ask. The spectator floor and a
broken client look identical.

**Compounds spec §4.2**, which already flags that promotion costs *"two
deliberate acts by the DM per player, while everyone else waits"*. The player
does not know that waiting is what they are supposed to be doing, so the natural
response is to report a fault — landing on the DM at the busiest moment.

**Shape of the work.** Name the viewer's role in the status bar; one line of
text on the spectator floor; roles beside names. Client-only, no contract
change. Merges naturally with T2's participant panel.

**Finding 12.**

---

## 3. Awkward

### A1. `join-link show` prints a placeholder nobody can substitute from there

**Evidence.** `share: <your-server-url>/?join=E-rq…`. Spec §4.2 predicted this
exactly — *"a candidate finding if it trips anyone"* — and it tripped the first
person to use it.

Not simply a bug: the CLI reads a SQLite file and cannot know it is fronted by a
tunnel hostname. Printing `localhost:8080` would be worse — a confident URL that
works for one person. The DM console has no such problem because it builds the
URL from the browser's own origin. **Finding 8.**

### A2. The admission budget is consumed before it is needed, and its semantics are unverified

**Evidence.** `admissions: 1 of 2 left` before the iPad join; `0 of 2` after.
Driver transcript: *"Both join slots are spent… it admits nobody further."*

#42 behaving as designed. The operational shape is the finding: budget is spent
by attempts that SUCCEEDED, and with S2 in play reconnection is routine.
**Unverified and it decides how bad this is:** whether reconnecting with an
already-issued token spends a second admission. The tool description implies not
("bound what an open door can MINT") — read from a doc comment, not measured.
**Finding 9.**

### A3. Probe participants litter the roster permanently

Four `OriginProbe*` agents and a `LivenessProbe` player are in the roster
forever; six of the ten names above are debris. Harmless for a fresh campaign,
and it makes T2(a) visibly worse. **Finding 4.**

---

## Passes — recorded so they are not re-litigated

- **P1. WebSocket upgrade survives the tunnel.** With a positive AND a negative
  control (`403` on a bad origin), so `101` means the check ran and passed, not
  that it never ran. No allowed-origin configuration needed. **Finding 1.**
- **P2. The DM opened the table with zero nudges.** One prompt chained
  `load_adventure` → `set_join_door` → `get_join_link` in the right order, and
  volunteered #45's caveats verbatim — the tool descriptions are doing real
  work. **Finding 5.**
- **P3. Joining takes under five seconds**, cellular, through the tunnel.
  One of the two questions spec §4.1 exists to answer. **Finding 11.**
- **P4. Promote / grant / demote / re-grant all correct**, and the agent
  **verified its own work** unprompted: *"act-fighter.controller_ids is
  ["7c35217c…"] — a single entry, so the revoke landed and control didn't
  accumulate."* Caveat: the instruction named both steps, so this shows it can
  EXECUTE a specified two-step, not infer one. **Finding 15.**
- **P5. Disconnect and reconnect is clean.** Manual reconnect restored the table
  with nothing lost. Does not soften S2 — here Patrik knew he had disconnected
  because he broke it himself. **Finding 18.**

---

## Corrections owed — spec amendments at the merge gate (rule 7)

Not backlog items; documentation debt this arc created or exposed.

1. **§4.1 misquotes the client spec.** It says *"client-design.md says
   tablet-first responsive."* `client-design.md` §8 says the opposite:
   *"mobile layout polish (desktop-first, degrade gracefully)"*. The
   misquote nearly filed T1 as deferred tablet work.
2. **§4.2 assumes guests use "whatever device they have."** With mobile
   deliberately deferred, **session one's invitation must say laptop or
   desktop.** Cheap to state up front, expensive to discover at the table.
3. **§5 says the tool surface is complete after #45.** T2(a) shows it is not:
   the agent cannot see who is connected. The drive gap is not only about being
   woken.
4. **§3's cellular justification was wrong** and is already corrected in the
   findings (finding 3). Realism, not correctness.

---

## Suspicions — no evidence, recorded, NOT to be acted on

Spec §7 keeps these separate deliberately: this platform has repeatedly been
surprised by things believed rather than measured.

- **Does a frozen mobile browser tab keep answering WebSocket pings?** Decides
  how safe any pong deadline is. Raised by the keepalive review; unverified.
- **Does reconnecting spend an admission?** See A2.
- **Are touch targets usable on a handheld?** Deliberately unassessed — tablet
  work is deferred, and the layout problem came first.
- **Can the agent INFER a two-step promotion** from *"Asme just joined, give her
  a character"*? Rung 1 of the nudge ladder was never tried; P4 tested rung 2.
- **Is `cellar-rats` also a fixture rather than content?** Assumed similar to
  `goblin-ambush`, never checked.
- **How long was the idle connection alive before it was reaped, and which hop
  reaped it?** Not measured; Patrik judged it would not change the fix, which
  is right.

---

## Choosing the next arc (spec §9)

Spec §9 requires the next arc to be chosen **from this backlog**, not from §1's
list of candidates (asset service, full 4.5e ruleset, voice pipeline). None of
those three appears anywhere above, which is the arc working as designed.

**The evidence points at S1, hidden information.** It is the only item that
makes content unrunnable rather than unpleasant, it is the one a table notices
first, and it is genuinely architectural — so it is the one most expensive to
retrofit later, and the one most likely to invalidate other work if deferred.
It also decides something about the fan-out (per-recipient filtering) that T2
and any future work will build on top of.

**Two smaller pieces are worth doing regardless**, because they are cheap,
client-side, and independent of that design: T2's participant panel (which
absorbs T3) and T1's fit-to-width. Neither blocks S1 and both make session one
materially better.

**S2's fix is written and rejected**; it needs a redo and one decision from
Patrik about the pong budget.

Not a recommendation to skip session one — S3 says session one cannot run on
`goblin-ambush` as it stands, so *some* content work precedes it whatever else
is chosen.
