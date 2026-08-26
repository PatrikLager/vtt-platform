# Session zero — findings

Dry run for the first real session.
Spec: `docs/superpowers/specs/2026-08-12-first-real-session-design.md`
Plan: `docs/superpowers/plans/2026-08-12-first-real-session.md`

Working notes. Every entry is something that needed a human to notice, explain
or work around — **including things that worked but were awkward**, because
those are the ones memory discards before session one.

Format per entry:

- **WHAT** happened
- **WHERE** — event sequence, screen, or command
- **COST** — stopped me / cost time / awkward

Tunnel hostnames are redacted throughout: a quick tunnel is a live door to the
machine while `cloudflared` runs, the full name adds nothing as evidence, and
the URL is dead within hours anyway.

---

## Entries

### 1. WebSocket upgrade through the tunnel — PASS

**WHAT.** The gateway calls `websocket.Accept(w, r, nil)`. With nil options
coder/websocket applies its default origin policy (`accept.go:239`): the
browser's `Origin` host must equal the `Host` header the server sees, with no
fallback patterns. Whether cloudflared preserves the original `Host` or rewrites
it to the origin URL is config-dependent and could not be settled by reading
code — so this was the plan's Task 1 and gated everything else.

**It passes.** cloudflared preserves the Host; the policy is satisfied.

| probe | result | role |
|---|---|---|
| localhost, `Origin: http://localhost:8080` | `101 Switching Protocols` | positive control |
| localhost, `Origin: https://evil.com` | `403 Forbidden` | negative control |
| tunnel, `Origin: https://katrina-….trycloudflare.com` | `101 Switching Protocols` | the answer |

**The negative control is what makes this a result.** Without it, `101` through
the tunnel could equally have meant "the origin check never ran". With a
confirmed `403` on a bad origin, we know the check is live *and* satisfied.

**WHERE.** 2026-08-12, cloudflared 2026.7.3, quick tunnel, `vtt serve` on :8080.

**COST.** None. No allowed-origin configuration is needed, so spec §3's
fix-properly path is not entered.

### 2. Two false results while establishing entry 1 — METHOD, not platform

Recorded because anyone re-running this will hit both, and because each looked
like an answer.

**`426 Upgrade Required` is not a rejection.** The first probe used plain
`curl`, which negotiates HTTP/2 with Cloudflare — and a classic
`Upgrade: websocket` header is HTTP/1.1 only. It reads like a refusal and is
actually the wrong protocol. **`--http1.1` is mandatory for this probe.**

**A malformed probe returned a plausible `101`.** The next attempt built the
header with `${2:+-H "Origin: $2"}`, which word-splits in the shell: the header
arrived as `"Origin:` + `localhost"`. Its controls returned `400`, which is what
exposed it — otherwise the tunnel's `101` would have been reported as success
while actually taking the *no-Origin* path (`accept.go:230` allows an absent
Origin), proving nothing about a browser.

**COST.** Awkward, and instructive. This is the fourth harness in a week to
produce a confident wrong answer, and the tell was the same every time: a
control that made no sense. Positive **and** negative controls, always.

### 3. "Test it on cellular" was justified wrongly — CORRECTION

**WHAT.** The plan told Patrik to verify from the iPad on cellular with wifi
off, on the grounds that same-network access "can succeed for reasons that will
not hold for a guest". He asked why, and the reason does not survive the
question.

The tunnel hostname is a public Cloudflare address. DNS resolves to their edge,
so traffic goes iPad → internet → Cloudflare → the Mac regardless of the iPad's
network. There is no local shortcut available to produce a false pass. The
argument would be sound for a LAN address such as `http://192.168.x.x:8080`,
and it was carried across to a case where it does not hold.

What IS true, and weaker: cellular exercises a different network path — mobile
latency, carrier NAT, possibly IPv6-only — which is closer to a remote guest's
conditions. Realism, not correctness.

**COST.** None to the result; entry 1's controls stand on their own and did not
depend on this. Recorded because a wrong reason in a plan is worse than no
reason: it gets followed, and later relied on.

### 4. Housekeeping from the probes

Four `OriginProbe*` agent participants were minted into `/tmp/vtt-session/session.db`
while testing. Harmless — session one starts on a fresh campaign per the plan —
but the campaign used for evidence must not be this one.

### 5. The DM opened the table with ZERO nudges — PASS, and informative

**WHAT.** One prompt — "load `goblin-ambush`, open the door for 2, tell me the
secret" — and the agent chained `load_adventure` → `set_join_door` →
`get_join_link` unprompted, in the right order, across five tool calls.

It also volunteered, without being asked:
- that it knows the secret but NOT which address the players can reach;
- that closing the door later does not kick anyone already through, and the
  secret admits anyone holding it, so pass it to players directly;
- a summary of what loaded (scene, actors, tokens, notes);
- two things it noticed were off (see below).

The first two are verbatim the caveats written into `get_join_link`'s and
`set_join_door`'s tool descriptions in #45, yesterday. **The descriptions are
doing real work** — the agent is passing on the reasoning rather than just
calling the tool.

**WHY IT MATTERS beyond "it worked".** The drive gap (§5 of the spec) is about
the agent not being WOKEN. It is not, on this evidence, about the agent being
unable to decide what to do once addressed. A task-shaped request produced a
correct multi-step plan with no hand-holding. That narrows what an autonomous
loop would have to supply: wake-ups and turn-taking, not reasoning.

**COST.** None. Recorded as a positive result because the arc's job is to
replace guesses with evidence in both directions.

### 6. `goblin-ambush` is a conformance fixture, not a playable ambush

**WHAT.** The agent flagged that "the tokens are all bunched along the top row
at their placement defaults rather than in ambush positions". Verified against
the source rather than taken on trust — `adventures/goblin-ambush/scenes/ravine.json`:

    tok-fighter (0,0)   tok-cutter (1,0)   tok-archer (5,0)

Three tokens, all on row 0, on a 32×32 grid. The fighter is adjacent to a goblin
at the start and nobody is concealed or elevated. It is not an ambush.

**WHY.** The directory carries a `goldens/` folder: this adventure exists to
pin the compiler's output, and its placements were chosen to be checkable, not
to stage an encounter. Nothing is broken — it is doing its job. It is simply not
content anyone would play.

**WHERE.** `adventures/goblin-ambush/scenes/ravine.json`, verified 2026-08-12.

**COST.** Awkward for session zero — the walk can continue. Potentially
"stopped play" for session one, since the first thing real players see is a
tableau that contradicts the adventure's name.

**WHAT IT SUGGESTS, for the next arc.** The founding spec's content candidate is
"full 4.5e ruleset", i.e. RULES. This points somewhere adjacent: the gap may be
adventures authored FOR PLAY rather than for tests. Both existing adventures
should be checked for the same shape before session one, and one of them staged
properly — which is content work, not engine work.

### 7. An idle connection dies, because nothing keeps it alive — STOPPED PLAY

**WHAT.** Patrik's browser was left idle through the tunnel while play was
happening elsewhere. It came back reading `closed`, and he had to reconnect by
hand. His requirement, stated on the spot: *"you have to be able to be
'inactive' without being kicked out."*

**It is not our timeout.** Checked rather than assumed, because the obvious
suspect is `store.SubscriberNoProgressTimeout` (30s) and it is innocent.
`internal/store/subscribe.go:145-163` puts that deadline INSIDE the select that
hands an envelope over, so the timer is only armed while an event is WAITING.
On a quiet table nothing is pending, the pump parks on `wake`/`done`, and the
timer never starts. The code says so itself: *"a subscriber is judged on whether
it is reading — never on how much it was sent."* The gateway's `writeTimeout`
is the same shape — it bounds a write, and an idle connection performs none.

**Nothing pings.** `internal/gateway/`, `internal/harness/` and `client/src/`
contain no ping, pong or keepalive of any kind. So an idle connection carries
ZERO BYTES in either direction, indefinitely, and every intermediary between a
player and the server treats a silent socket as a dead one eventually.

**Which is galling, because we wrote down the fix and did not build it.**
`internal/gateway/server.go:80`, in `gatewayNoProgress`'s own doc comment:
*"Prior art: MapTool (net.rptools.clientserver) … detects a departed client
purely with a socket timeout — one minute, against a 20s client heartbeat."*
Patrik independently recalled the same 20s figure from the same prior art
before that comment was re-read. We took MapTool's timeout and left its
heartbeat on the table.

**WHY IT COMPOUNDS.** Three things multiply here, and only the first is
obvious:

- Reconnection is MANUAL by design (client spec §3.4). Nothing retries.
- Tabletop pacing is minutes of silence — an idle socket is the NORMAL state
  during roleplay, not an edge case.
- The only signal is a small `closed` label. A player who is listening rather
  than looking has no reason to check it, so they discover the disconnect by
  acting and having nothing happen.

Worse at the table than for the person who fell off: a dead client stays
CONNECTED in the presence registry, because departure hangs off `serve`
returning and `serve` is blocked in `conn.Read` on a socket nobody has told it
is gone. The DM sees a full table and narrates to a ghost.

**NOT tunnel-specific.** Cloudflare has an idle timeout, but so does carrier
NAT, so does a home router's connection table, and so do corporate proxies.
Removing the tunnel would move the number, not the problem.

**WHERE.** 2026-08-12, session zero, browser on the tunnel origin, idle during
DM-side activity.

**COST.** Stopped play — for one participant, silently, with a manual recovery
that only works if you notice.

**WHAT WAS NOT MEASURED.** The elapsed idle time before the drop, and which hop
dropped it. Patrik declined to measure it (*"no need to measure"*) and he is
right that it does not change the fix: a keepalive is required at any of the
plausible numbers, and knowing whether Cloudflare's timer or the carrier's won
the race would not move the interval. Recorded so nobody later reads a
confident cause into this entry that nothing here establishes.

**THE FIX** — spec §3, so properly: a server-side ping every 20 seconds
(Patrik's call, matching MapTool), failing the connection when the pong does
not come back. Both halves matter. The ping keeps intermediaries from reaping a
silent socket; the pong is what MapTool actually used it for — *"to check if
any connections were down on the player side"* — and it is what finally lets
the table stop seeing ghosts. Cheap: ~12 bytes per round trip, about 18 KB
across six players for a ninety-minute session — roughly twenty-six
`PresenceSnapshot`s' worth in total, so about seventeen an hour.

*(CORRECTED 2026-08-26. That last clause read "less than a single
`PresenceSnapshot` costs per hour" and was wrong by a factor of about twenty; a
six-participant snapshot runs around 700 bytes. The 18 KB was right. The
CONCLUSION — negligible — survives the correction untouched, which is exactly
why nobody checked the arithmetic underneath it.)*

Server-side rather than client-side, which is not a coin flip. A browser's
WebSocket stack answers a ping frame below JavaScript, so pinging FROM the
server keeps every client honest — including the ones we did not write — and
needs no client change at all. The reverse direction would require every client
to implement it, and the JS `WebSocket` API cannot send a ping frame at all.
(True of a RUNNING page. A tab the OS has frozen is a different state, and
whether iOS keeps answering control frames through it is unverified — which
matters, because a frozen tab is one of the ways a player is "inactive".)

**NOT YET FIXED (as of 2026-08-12 — see the update below), and the first attempt was wrong** — recorded because it is
evidence about the fix rather than about the platform. A ping was written
during the walk, reviewed, and found to reap healthy players:
coder/websocket wraps every control frame in a hard-coded five-second context,
so a ping that merely loses the frame lock to a busy writer looks identical to
a dead peer. Since the interval (20s) is shorter than `writeTimeout` (30s),
every write using its full budget would have straddled a tick and been killed
at ~5s — against a policy whose own comment says a merely BUSY client keeps its
frame. **A ping is a verdict about the pong, never about our own writer**, and
the first version conflated them.

Two open questions the fix must answer rather than assume:

- **Which case do we choose to lose?** A pong deadline generous enough for a
  frozen phone leaves a ghost at the table longer; one tight enough to clear
  ghosts quickly reaps players who are merely asleep. Patrik's call, and it
  belongs in the backlog rather than in a constant chosen quietly.
- **Where is this written down?** An unsolicited control frame every 20s and a
  close on a missed pong is wire-visible behaviour every client author must
  tolerate. It belongs in the gateway spec (rule 7), not only here.

**UPDATE 2026-08-26 — fixed, and both questions answered.** The heading above
stays as written; it was true for a fortnight and the reason it stayed true is
part of the record.

- *Which case do we lose?* Patrik chose the ghost over the reap: a 60s pong
  budget, three intervals, MapTool's ratio. Two pongs may be lost or arrive
  late before anyone is declared dead, and a genuinely dead peer lingers 60-80s holding one idle socket and one presence entry. Reaping a live player is
  the worse failure and it is invisible from the outside — it looks exactly
  like the disconnect this whole entry is about.
- *Where is it written down?* `internal/gateway/keepalive.go` carries the
  reasoning; the wire contract goes in the gateway spec, which is the last
  piece of this arc still outstanding.

The fix landed in two commits: the verdict and the busy-skip, then the seam in
`serve()` that makes them live. A ping is now a verdict about the pong and
nothing else, and a tick is skipped entirely while the writer holds the frame
lock — so the contention that produced the first wrong version mostly does not
arise, and cannot be misread when it does.

### 8. `join-link show` prints a placeholder nobody can substitute from there — AWKWARD

**WHAT.** Fetching the link to send to the iPad:

    $ vtt join-link show --campaign session.db
    door: open
    admissions: 1 of 2 left
    secret: E-rq…
    share: <your-server-url>/?join=E-rq…

The `share:` line is not a URL. Spec §4.2 predicted this exactly — *"a
candidate finding if it trips anyone"* — and it tripped the first person to
use it, which is the evidence the prediction was waiting for.

**WHY IT IS NOT SIMPLY A BUG.** The CLI genuinely cannot know the answer. It
reads a SQLite file; the campaign has no idea it is being fronted by a
cloudflared hostname, and `--addr :8080` is what the process binds, not what a
guest can reach. Printing `http://localhost:8080/?join=…` would be WORSE — a
confident URL that works for exactly one person and silently fails for every
guest. The placeholder is honest.

**The DM console has no such problem** because it builds the URL from the
browser's own origin, so it is tunnel-correct for free. Two writers of the same
URL, one of which can be right by construction and one of which cannot.

**COST.** Awkward. Nothing stopped; it costs one substitution and one chance
to paste the wrong host. At a live table the DM is doing this while people
wait, which is where a fiddly step turns into a delay.

**WHAT IT SUGGESTS.** The fix is not a smarter CLI — it is `serve` learning its
own public address (a `--public-url` it can hand to both the console and the
CLI), or the CLI declining to print a `share:` line at all rather than printing
one that is not one. Backlog, not now.

### 9. The admission budget is spent before the walk needs it

**WHAT.** `admissions: 1 of 2 left`. The DM opened the door for two; one was
consumed by the earlier browser join. The iPad join takes the last one, so any
retry — a mistyped URL, a failed first attempt, the tunnel hiccuping — lands on
a closed door mid-walk.

**WHY IT MATTERS beyond this walk.** This is #42 behaving exactly as designed
(a budget is the point), but it exposes the operational shape: the budget is
consumed by ATTEMPTS THAT SUCCEEDED, and a person who fails to *stay* joined
still spent one. Session zero's own disconnect (entry 7) means a reconnect is
routine, and if a reconnect spends an admission then a table of six needs a
budget well above six.

**UNVERIFIED, and it decides how bad this is:** whether reconnecting with an
already-issued token spends a second admission, or whether the budget only
governs minting new participants. `get_join_link`'s description says the latter
("bound what an open door can MINT"), which would make this benign — but that
is read from a doc comment, not measured, and this note does not get to assume.

**COST.** Awkward here. Potentially "cost time" at a live table, where the DM
discovers it while a player is locked out.

### 10. The LLM DM cannot tell who is actually at the table — COST TIME, and it compounds entry 7

**WHAT.** With the iPad seated, the roster the agent can read looks like this:

    Agent (agent)          Armak (spectator)      Asme (spectator)
    DM (agent)             LivenessProbe (player) OriginProbe (agent)
    OriginProbe2 (agent)   OriginProbe3 (agent)   OriginProbe4 (agent)
    Patrik (player)

Ten names. Three connections. Six of those are dead probes from entry 4 and one
is a liveness check.

**WHY, verified in the route rather than inferred.** `get_participants` reads
`/api/participants`, and `internal/gateway/metadata.go:369-391` calls
`s.ids.List()` — the identity database — and never touches `s.presence`. It is
an *invitation* list, not an attendance list. Its own tool description says
"List everyone at the table", which is true only if the table is defined as
everyone ever admitted and not revoked.

**There is no other route to the answer.** The MCP tool surface has no presence
tool at all, and presence deliberately never enters the event log — spec §4:
*"Wire state, never appended to the log — replaying a campaign must not
resurrect a session."* That decision is right and this is its cost: because
presence is not an event, it is not in `get_events_since`, so the one channel
the agent polls cannot carry it either.

**The human DM has no such problem.** The browser console renders
`PresenceSnapshot`/`PresenceChanged` frames and shows who is connected live. So
the capability exists, is already on the wire, and stops precisely at the seam
where the LLM sits.

**WHY IT MATTERS more than it looks.** This lands directly on spec §5's
argument. The drive gap was framed as the agent not being WOKEN, with the tool
surface described as complete after #45. It is not complete: once woken, the
agent cannot answer "who is here?" — so it cannot know whether to wait for
someone, whom to address, or that a player has dropped.

And it compounds entry 7 exactly. Players fall off silently, AND the DM is
blind to it, AND a dead client lingers as CONNECTED anyway. Three separate
mechanisms all pointing at the same table-level symptom: the DM talking to
people who are not there.

**WHERE.** 2026-08-12, immediately after the iPad joined.
`internal/gateway/metadata.go:369-391`; MCP tool list has no presence entry.

**COST.** Cost time at a live table, and it is the kind that is invisible while
it is happening. It is also the clearest capability gap the walk has produced —
§6's sharpest debrief question is "what did you try to do that you couldn't",
and this is the DM's answer to it, found before any guest arrived.

### 11. Joining is fast — PASS

**WHAT.** Link tapped to board visible: **under five seconds**, on the iPad,
over cellular, through the tunnel. Patrik: *"joining the board and seeing the
board takes less than 5 sec."*

This is one of the two questions spec §4.1 says session zero exists to answer,
and it is a clean pass. Worth stating plainly because the arc is mostly
collecting problems: the join path — mint, admit, upgrade, catch up, render —
is not a place that needs work.

**COST.** None.

### 12. A spectator gets a board with no controls and nothing that says why — COST TIME

**WHAT.** Seated on the iPad, Patrik: *"I do not have any buttons, not on
laptop browser or ipad browser. Maybe because I am a spectator?"*

He is right. That is exactly why. **The person who built this had to guess**,
from two devices, and phrased it as a question.

**VERIFIED, not assumed.** `client/src/view/spectator.ts:245-271` assembles
status, grid, feed, notes and ticker — no controls, by design (`player.ts:70`:
*"A spectator acts as nobody. That is a UI affordance, not a defence"*). The
design is correct and matches joining-a-table §2: spectator by default, because
it is safe by construction.

**The gap is that nothing communicates it.** `renderStatus`
(`spectator.ts:192-223`) shows the connection state, the session name, and the
display names of who is present. It never shows **your own role**, and it never
says what happens next. The participant list carries no roles either, so a new
arrival cannot tell who the DM is — i.e. cannot tell who to ask.

So the spectator floor is indistinguishable from a broken client. The two
readings — "I am waiting to be promoted" and "this thing does not work" — look
identical, and only one of them is true.

**WHY IT COMPOUNDS.** Spec §4.2 already flags that promotion costs *"two
deliberate acts by the DM per player, while everyone else waits"*. This adds
the other half: during that wait the player has no idea that waiting is what
they are supposed to be doing. The natural response is to report a fault,
which lands on the DM at the busiest moment of the evening.

**COST.** Cost time here — it interrupted the walk to diagnose. For a guest the
cost is plausibly higher, since they have neither the source nor the vocabulary
to form the hypothesis. That inference is NOT measured, and session one is
where it gets tested; four friends landing on a silent board is precisely the
experiment.

**WHAT IT SUGGESTS.** Cheap and entirely in the client: name the viewer's own
role in the status bar, and give the spectator floor one line of text — you are
watching; the DM can give you a character. Roles beside names in the participant
list would answer "who do I ask" for free. No engine or contract change.

### 13. Who is present renders as one run-on word, and who arrived is nowhere — COST TIME

Two mechanisms, one symptom, and Patrik named the fix from prior art.

**(a) The names are concatenated.** Observed: `ArmakAsmeDM`. Not a typo — it is
three participants with no separator between them.

VERIFIED IN THE CSS, not guessed. `renderStatus`
(`client/src/view/spectator.ts:204-208`) builds a `<span class="present">`
holding one `<span class="participant">` per person. `client/src/style.css:32`
gives `.status` `display: flex; gap: 16px` — but that gap applies to the
status bar's OWN children (`.conn`, `.session`, `.present`), not inside
`.present`. And **`.present` and `.participant` have no CSS rule at all** —
grep returns nothing. So they are plain inline spans, emitted with no
whitespace and no styling, and the browser runs them together.

With three short names it is a puzzle. With six players called Anna, Sam and Jo
it is unreadable, and there is no way to tell where one name ends.

**WHY THE SUITE MISSED IT, which is the interesting part.**
`client/test/spectator-view.test.ts:824-825` does assert this element: it finds
`.present` and maps `.participant` text into an array of names. That assertion
is CORRECT and it passes — the DOM structure is exactly right. What is broken
is the rendering, and happy-dom does no layout and applies no CSS, so a run-on
line and a spaced list are the same tree.

`spectator.ts`'s own header comment called this shot: *"What is here is DOM
assembly, which is the part a screenshot verifies better than an assertion
could."* The file knew which half of it a unit test cannot see. Nobody then
looked at a screenshot of this element, and it shipped.

**(b) Arrivals and departures are absent from the story.** Patrik: *"in the
story window you can not see who has connected or reconnected (maybe by
design?)"* — by design, and verified: `buildFeed`
(`client/src/view/feed.ts:32`) takes `Envelope[]`, the event log, and presence
is deliberately never an event (spec §4, so replay cannot resurrect a session).
Correct decision; this is its cost. There is no record anywhere of who came and
went.

**WHAT IT COMPOUNDS WITH.** Entry 7: players drop silently. Entry 10: the LLM
DM cannot see presence at all. Add this and **nobody at the table — human or
agent — has any usable account of who is actually here.** Three independent
mechanisms, one table-level failure. That convergence is the strongest signal
the walk has produced, and it was invisible from any one of them alone.

**PRIOR ART, Patrik's:** *"If you check RPTool, you can see each
player/spectator as its own 'thing' in a sidebar."* MapTool gives each
participant a row of its own with state attached, rather than a name in a
sentence. Same source that supplied the 20s heartbeat in entry 7, and the same
lesson: the shape of the answer already exists.

**COST.** Cost time. Neither half stopped the walk; both would be noticed
immediately by a guest, and (a) gets worse with every additional player, which
is the wrong direction for a feature whose whole point is a table.

**WHAT IT SUGGESTS.** One change covers all of it and entry 12 as well: a real
participant panel — one row per person, showing name, ROLE (entry 12: nobody
can tell who the DM is), and connection state. Client-only; presence frames
already carry everything it needs.

### 14. There is no hidden information anywhere in the platform — STOPPED PLAY

The largest finding of the walk. **The DM agent raised it unprompted, and then
it happened.**

**WHAT THE AGENT SAID**, before anything went wrong, while reporting a
successful promotion:

> *"Asme sees the goblin positions in the shared log the moment they look,
> since the ambush setup at seq 12–13 is already written there. The ambush is a
> surprise to the character, not to the player."*

**WHAT THEN HAPPENED.** Within ~90 seconds of being granted the fighter, Asme
moved it seven times (seq 15–21). Six were shuffles around (0,0). Seq 20 put
`tok-fighter` on **(19,8) — the Goblin Archer's exact square** — and seq 21
moved it back. The DM found this by polling the log and reported it: *"Landing
precisely on the archer's hidden square isn't a plausible accident on a 32×32
grid."*

**VERIFIED IN SOURCE, and it is worse than the log.** The agent named one
channel; there are two, and it named the harder one.

- **No such concept exists.** `grep -rn "hidden|visib|fog|secret"` across
  `contract/vtt/v1/*.proto` returns NOTHING outside join-link machinery. There
  is no visibility field on a token, an actor, a scene or an event. Not
  unimplemented — unmodelled.
- **The board shows everything.** `client/src/view/grid.ts:68`,
  `tokensOnScene`, filters on one thing: `tok.SceneID !== sceneId`. Every token
  on the scene is drawn for every viewer. So the goblins were not merely
  *readable* — they were **on screen**, as discs, for anyone seated.
- **The log goes to everyone in full.** The gateway's pump forwards every
  envelope to every connection, and catch-up from `after=0` replays the lot.
  Role gates what you may WRITE (`commandRoles`); nothing gates what you read.

So the mechanism was not the sophisticated one the agent guessed. **CONFIRMED
BY THE PLAYER**, asked directly — Patrik, as Asme on the iPad: *"yes, I could
see the goblin token on the board — there is no fog/limited view mechanism as
far as I can see at least."*

A player did not have to parse an event log. **They could see the goblin and
tap it.** That matters for where a fix starts: the log is the subtle channel and
the one an engineer reaches for first, but the board is the one that leaks to
somebody who is not even trying. Any fix that filters the log and leaves the
grid rendering every token would close the harder hole and leave the obvious
one open.

Recorded as a corrected inference: the agent's read was reasonable and wrong in
a direction that would have misdirected the work.

**WHY THIS IS "STOPPED PLAY" AND NOT "AWKWARD".** It is the only finding so far
that makes a class of content unrunnable rather than unpleasant. An ambush, a
hidden trap, a secret door, a monster's remaining HP, a sealed DM note — every
one is disclosed to every participant by construction. `goblin-ambush` cannot
be run as an ambush by anyone, ever, on this platform as it stands. Entry 6
concluded that adventure was a fixture rather than content because of where its
tokens sat; this is the deeper reason, and it applies to content we have not
written yet.

**AND IT CANNOT BE UNDONE.** The agent was straight about this too: *"I can't
retract those moves cleanly… retracting doesn't unshow what people already
saw."* Correct on both counts — `retract_events` is inclusive-range and those
sequences are now buried under later control changes, and disclosure is not a
state you can roll back. Demoting Asme back to spectator does not un-see it,
and spectators watch everything anyway.

**WHERE.** 2026-08-12, seq 12–23. `contract/vtt/v1/` (absence),
`client/src/view/grid.ts:68`, gateway pump + catch-up.

**COST.** Stopped play. The encounter's premise was spent before it began, and
the DM's own remediation options were all bad: reposition the goblins where the
next player watches it happen live, or move the ambush to a fresh scene.

**WHAT IT SUGGESTS, and this is a real arc rather than a fix.** Hidden
information is a first-class VTT concept, not a feature bolt-on: it touches the
contract (visibility on tokens/notes/events), the fold, the gateway's fan-out
(per-recipient filtering, which today does not exist — every connection gets
identical bytes), catch-up, and the client. It also collides head-on with
"replay is the source of truth": if the server filters per recipient, two
participants no longer share one log. **This is a strong candidate for the next
arc**, and unlike §1's candidate list it is now evidenced rather than guessed.

### 15. The DM handled promote, grant, demote and re-grant correctly — PASS, with one caveat about how it was asked

**WHAT.** Two instructions, both executed correctly:

- *"promote Asme participant from spectator to player and give control of the
  fighter token to Asme"* → 4 tool calls, done.
- *"demote Asme to spectator and remove its control of the fighter — instead
  promote Armak to player and give him control"* → 7 tool calls, done.

It found a fresh spectator's participant id (the exact gap #45 closed), ordered
promote-before-grant unprompted, and after the reassignment **verified its own
work**: *"act-fighter.controller_ids is ["7c35217c…"] — a single entry, so the
revoke landed and control didn't accumulate."* Checking that the control SET
did not accumulate is precisely the invariant T2/T3 were built around, and
nobody asked it to check.

It also volunteered, unasked: both join slots spent; Armak still a spectator
controlling nothing; `goblin-ambush` ships only one player actor so there is no
second PC to hand out; and the fighter still standing at (0,0).

**THE CAVEAT, recorded so this is not read as more than it is.** The
instruction was rung 2 of the planned ladder — it named the promotion AND the
grant AND the target explicitly. Rung 1 (*"Asme just joined, give her a
character"*), which is what a DM would actually say and which tests whether the
agent infers the two-step, **was not tried**. So this is evidence the agent can
EXECUTE a specified two-step, not that it can infer one. Entry 5 is the
stronger data point for inference.

**COST.** None. Recorded as a pass, and as a limit on what the pass shows.

### 16. `goblin-ambush` has exactly one player actor — CONFIRMS entry 6

The agent, unprompted: *"there's no second PC in this adventure to hand over.
goblin-ambush shipped one player actor."* So the adventure cannot seat two
players at all, which is independent confirmation of entry 6 from a different
direction — that entry inferred "fixture, not content" from token placement;
this is the roster proving it. A table of four has nothing to give three of
them.

### 17. On a LAPTOP the controls sit below 1408px of board — COST TIME on the primary target

**WHAT.** Patrik, once Armak had controls: *"the board is too big, but not
sideways, but up and down. I can not see the controls when looking at the
board, I need to scroll down to see them."* Then, when this note first filed it
as a tablet problem: ***"The issue I described was on the laptop — I need to
scroll to see the controls."***

**THE MISATTRIBUTION IS ITSELF WORTH RECORDING.** This was asked for as part of
a handheld pass, so the answer was filed as a handheld finding and ranked as
deferred scope. It is neither. It is the desktop layout, on the platform's
declared primary target, and it was one clarification away from being sorted
into a pile nobody was going to look at. The lesson is the same one entry 2
taught about probes: the question you asked shapes how you read the answer, and
an answer arriving in a context does not belong to that context.

**THE MECHANISM, exactly.** Three facts, each verified:

1. **The board is sized in hard pixels.** `spectator.ts:84-85` sets
   `board.style.width/height` to `gridWidth * CELL` by `gridHeight * CELL`,
   with `CELL = 44`. `ravine.json` is `grid_width: 32, grid_height: 32`. So the
   board is **1408 x 1408 px**. No scaling, no `viewBox`, no `max-width`.
2. **The controls are placed in the grid row directly beneath it.**
   `style.css:85`: `#app { grid-template-areas: "status status" "board feed"
   "player ticker" "notes ticker" }`. `.player` — the chips, the say box,
   everything you act with — begins after 1408px of board.
3. **There are no breakpoints at all.** `grep -c "@media" client/src/style.css`
   returns **0**. The layout is a fixed two-column grid that never adapts to
   viewport.

**DO THE ARITHMETIC AND IT CANNOT FIT ANY LAPTOP.** The board alone is 1408px
tall; add the status bar and `#app`'s 16px padding and `.player` begins around
**1450px down the page**. A typical laptop has 800-1000px of viewport height.
So the controls are not marginally below the fold — they are roughly a screen
and a half down, on every laptop, always. Acting means scrolling away from the
thing you are acting on and then scrolling back to see what happened. That is
the core loop of play.

**WHY NO HORIZONTAL SCROLL, and why that is screen-dependent rather than
reassuring.** `#app` is `minmax(0, 2fr) minmax(280px, 1fr)`, so the board column
is about two thirds of the window. The 1408px board fits horizontally only on a
window wider than roughly 2100px — a large external monitor. On a 1440px
laptop screen the board column is about 930px and the SAME scene would overflow
sideways too. So "not sideways" is a fact about the monitor it was viewed on,
not about the layout.

**THE SPEC WAS MISQUOTED, and it is our own error.** This arc's spec §4.1 says:
*"`client-design.md` says tablet-first responsive."* It does not.
`client-design.md` §8 (Non-goals, YAGNI) says the opposite:

> *"mobile layout polish (**desktop-first, degrade gracefully**)"*

The client was deliberately built desktop-first with mobile deferred. §4.1
inverted that, and then used the invented claim to justify the handheld test.

This changes what the backlog item IS. Not "a responsive layout is broken" —
**"deferred scope is still deferred, and here is what it costs."** Different
work, different size, and the wrong framing would have sent someone hunting a
regression that does not exist. Same failure as entry 3: a wrong reason written
into a plan gets followed and then relied on.

What IS fair against the spec's own words: *degrade gracefully*. Putting the
controls under 1408px of board is not graceful degradation — nothing responds
to the viewport at all, so there is nothing to degrade. The qualifier is unmet
on its own terms.

**PATRIK'S SCOPE CALL, 2026-08-12:** *"lets not focus on 'tablet' playability
right now. Lets get the functionality and look to work on a computer/laptop,
then we can migrate that to a tablet/phone design."*

That defers the RESPONSIVE work — breakpoints, stacked layouts, touch targets —
and restores exactly what `client-design.md` §8 said all along
(*"desktop-first, degrade gracefully"*). This arc's §4.1 misquoted that as
"tablet-first responsive"; the correction returns to the original position
rather than inventing a new one.

**It does not defer this finding.** This one is squarely inside "make it work on
a computer/laptop", which is the part Patrik just put FIRST. Deferring mobile
raises this item's rank rather than lowering it: the desktop layout is now the
whole target, and on the whole target the controls are a screen and a half below
the board.

**COST.** Cost time, on the primary target, on every session, for every
participant with an actor — it is not an edge case but the default state of
the screen.

**WHAT IT SUGGESTS.** Make the board fit its column instead of dictating the
page: scale `CELL` to the available width, or give the board pane its own
scroll so an oversized scene does not push the rest of the layout down. Desktop
only, no breakpoints, no media queries — the responsive migration Patrik
describes is separate and later.

**AND IT IS PROBABLY NOT JUST THE BOARD.** `CELL = 44` is a constant with no
relationship to the viewport, and the same is true of the scene: a 32x32 map is
what the adventure declares. Whatever fixes this has to answer "what happens
when a scene is bigger than the window", which is a question the client has
never had to answer because nothing had ever rendered a real adventure at real
size until today.

### 18a. Consequence for session one: guests need a computer

Recorded here because it changes the invitation, not the code.

Spec §4.2 assumes guests *"open that origin in a browser on whatever device
they have"*. With mobile layout deliberately deferred (above), that assumption
no longer holds: a friend who joins on a phone gets a board whose controls are
below a screen and a half of grid.

**Session one's invitation should say laptop or desktop.** Cheap to state up
front, expensive to discover at the table with four people waiting. This is a
planning constraint produced by a scope decision, not a defect.

### 18. Disconnect and reconnect: clean — PASS

**WHAT.** Patrik killed wifi on the iPad, restored it, reconnected by hand:
*"killed wifi, then turned it on, and manually reconnected, no issues."*

The manual-reconnect path (client spec §3.4) works as designed. The Reconnect
button is offered ONLY when the connection is actually closed
(`spectator.ts:215`), and rejoining restored the table with nothing lost — the
`after` cursor and the log did their job.

**WHAT THIS DOES AND DOES NOT SHOW.** The recovery MECHANISM is sound. It does
not soften entry 7, and the difference is the whole point: here Patrik knew he
had disconnected, because he broke it himself and was watching for it. Entry 7
is the case where nobody tells you — an idle socket reaped by an intermediary,
discovered minutes later by acting and having nothing happen. Working recovery
plus no notification is still a player sitting out half an encounter.

### 19. The board has no camera — it IS the page, at 1:1 forever

Patrik, on entry 17: *"why is not the board size aligned to the screen — I
think that is how RPTool does it, it follows the screen."*

It is not aligned to the screen because **there is nothing to align it with.**

**VERIFIED BY ABSENCE.** `grep -rniE "zoom|pan|camera|scale|fit"` across
`client/src/**` returns no hits (only `<span>` matching on "pan"). The client
spec mentions zoom, pan, cell size, oversized scenes and scrolling **nowhere**.
There is no viewport, no scale factor, no fit-to-window, no drag-to-pan, no
wheel-to-zoom.

**THE TWO MODELS, and the difference is architectural.**

- **MapTool:** the map is a WORLD drawn on a canvas, and the screen is a CAMERA
  onto it — zoom, pan, fit-to-window. Map size and window size are independent
  by construction, so a 32x32 or a 200x200 map is the same problem.
- **Ours:** the board IS the DOM, at 1:1. `spectator.ts:84-85` sets the board
  element to `gridWidth * CELL` pixels and absolutely positions each token at
  `x * CELL`, `y * CELL`. The map does not fit in the window; the map fits in
  the DOCUMENT, and the window scrolls the document.

At 1:1 with `CELL = 44`, scene size and screen size are welded together. A
scene is playable only if the adventure author happened to pick dimensions that
fit the player's monitor — which no author can know.

**`CELL = 44` IS A TELL.** `client/src/view/spectator.ts:16`:

    export const CELL = 44;

A bare constant, no doc comment, no justification. In a codebase where
`gatewayBuffer`, `gatewayNoProgress` and `maxWSFrameBytes` each carry a
paragraph or more explaining themselves, this one carries nothing. That is
what an undecided default looks like as opposed to a decision — and it survived
because until today nothing had rendered a real adventure at real size. Every
test scene is 10x10 (440px) and fits anything.

**THE GOOD NEWS, and it changes the size of the work.** The geometry is ALREADY
parameterised. `grid.ts:52`, `cellFromPoint`, divides by `geom.cell` rather than
by a constant, and `Geometry` carries `{ cell, width, height }`. So hit-testing
is correct at ANY cell size already — clicking the right square keeps working
if `CELL` becomes computed. Fit-to-width is therefore a small change threaded
through an existing seam, not a rewrite.

Full zoom and pan is a bigger piece of work, but it uses the same seam, and it
is what makes scene size and screen size independent the way MapTool's is.

**COST.** This is the root of entry 17 rather than a separate cost. Recorded
separately because the FIX is a different shape: 17 says the controls are
unreachable, 19 says the reason is that the client has no concept of a viewport
onto a larger world — and any fix that does not introduce one only moves the
threshold at which the problem returns.

**Third time MapTool has supplied the answer today** (20s heartbeat in entry 7,
per-participant sidebar in entry 13, camera here). Worth noticing as a pattern:
the questions this platform is now hitting are the ones every VTT hits, and
there is a mature implementation to read.

---

## Still to walk (plan Task 2)

- [x] Seat the agent in the MCP host against the tunnel; confirm tools appear.
      24 tools, connected over the tunnel.
- [x] DM loads an adventure and opens the door; record nudges needed. Entry 5:
      zero nudges.
- [x] Join from the iPad as a guest would. **Time it**: link opened → seated.
      Under 5s (entry 11).
- [~] Handheld check. NOT actually answered: the finding it produced (entry 17)
      turned out to be the LAPTOP layout, and tablet work is now deferred by
      decision. Touch targets and small-screen overflow remain unassessed, and
      deliberately so.
- [x] Promote + grant an actor; record how many nudges. One instruction each
      for grant and for reassignment (entry 15), but at ladder rung 2 — the
      inference case was not tested.
- [x] Move a token from the iPad. Seven moves, seq 15–21 — and it surfaced
      entry 14.
- [x] Break the connection for ten seconds; reconnect. PASS (entry 18) — the
      mechanism works; the gap is notification, not recovery.

**The walk is complete.** Next: rank the backlog (spec §7) and hold the merge
gate on entry 17's open question.
