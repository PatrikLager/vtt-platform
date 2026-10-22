# SPEC-022: A client holds one log

## Status

Accepted. The browser client implements it in `client/src/wire.ts`,
`client/src/session.ts`, `client/src/app.ts`, `client/src/view/spectator.ts`
and `client/src/view/player.ts`; the Go readers in
`internal/harness/client.go`, `internal/harness/engine.go`,
`internal/harness/soak.go`, `cmd/vtt/client_run.go`'s `dialerFor`,
`internal/mcp/server.go`, `internal/mcp/read_tools.go` and
`cmd/vtt/state_dump.go`. That the browser client's window shows no sequence
outside the reason it gives for a log it cannot fold is held by
`client/test/app.test.ts`'s "no role's window shows a log sequence number",
`client/test/spectator-view.test.ts`,
`client/test/qa-window-number.test.ts` and `client/e2e/table.spec.ts`
(VTT-393), the last outside `task check`; that it holds one log and a redial
names it, by `client/test/wire.test.ts`, `client/test/session.test.ts`,
`client/test/app.test.ts`, `client/test/qa-one-log.test.ts`,
`client/test/qa-switch.test.ts`, `client/test/qa-remembered-log.test.ts` and
`client/test/qa-reload-viewpoint.test.ts` (VTT-394 to VTT-398, VTT-407); that
until its first `ViewpointChanged` a page names the actor the last one sent to
a page in its tab under its token named, and no `log` when none was, whatever
another token's or another tab's pages were sent, by
`client/test/wire.test.ts`, `client/test/app.test.ts`,
`client/test/qa-remembered-log.test.ts` and
`client/test/qa-reload-viewpoint.test.ts` (VTT-442 to VTT-444); that a player
and a spectator are offered the party members a `WatchableChanged` names and
switch between them, by `client/test/app.test.ts`,
`client/test/player.test.ts`, `client/test/spectator-view.test.ts` and
`client/test/qa-switch.test.ts` (VTT-402 to VTT-406, VTT-408); that a
spectator chooses a shoulder and a player switches in the browser, and that
a reload keeps either, by `client/e2e/table.spec.ts` (VTT-394, VTT-403,
VTT-405, VTT-442, VTT-444) and `client/e2e/handover.spec.ts` (VTT-442), both
outside `task check`.

That `internal/harness`'s client hands its reader every entry and each switch
and that a dial names a log is held by
`internal/harness/client_switch_test.go` and
`internal/harness/qa_client_switch_test.go` (VTT-409, VTT-410, VTT-427); that
the scenario engine holds one log per character seat and its reconnect names
it, by `internal/harness/one_log_test.go`,
`internal/harness/qa_one_log_test.go`, `internal/harness/qa_log_seat_test.go`
and `internal/harness/engine_test.go`, and against a gateway by
`scenarios/a-player-switches-and-reconnects.json` in
`TestScenarioLibraryRunsSelfContained` (VTT-411 to VTT-415); that the soak
compares each character seat's fold with a fresh dial of its log, by
`internal/harness/soak_log_seat_test.go` and
`internal/harness/qa_soak_log_test.go` (VTT-411, VTT-416, VTT-428 to
VTT-431); that `vtt mcp` holds the log it is sent and names it, by
`internal/mcp/one_log_test.go`, `internal/mcp/qa_one_log_test.go` and
`cmd/vtt/mcp_player_e2e_test.go` (VTT-418 to VTT-422); and that
`vtt state dump` folds one log and names it, by
`cmd/vtt/state_dump_switch_test.go` and
`cmd/vtt/qa_state_dump_switch_test.go` (VTT-423 to VTT-426, VTT-432 to
VTT-435). That `vtt events tail` prints no switch is held by no row.

## Principles served

vtt-platform has no blueprint. This section is owed and cannot be written
honestly until one exists — naming principles here would invent them from a
single record, which is the failure a blueprint is written to prevent.

## How it works

**What this record does not decide.** What the wire carries,
`ViewpointChanged`'s and `WatchableChanged`'s shapes among it, is SPEC-007's;
when the gateway sends either is SPEC-015's.

### What the window shows

The window a client draws, `renderSpectator` in
`client/src/view/spectator.ts` and what `client/src/app.ts` adds to it per
role, shows what happened as text and no sequence, to any role, except in the
reason it gives for a log it cannot fold (VTT-393; the owner's rulings of
2026-10-19). Why the wire carries sequences at all is SPEC-007's.

### What the browser client holds, and what it offers

The browser client holds one log (`client/src/wire.ts`,
`client/src/session.ts`): the world for the DM and the agent, otherwise the log
of the actor the last `ViewpointChanged` named, and none for nobody or before
the first one. A `ViewpointChanged` naming another actor than the one whose log
it holds, or nobody while it holds one, discards what it folded and folds what
follows (VTT-394); one naming the actor whose log it holds keeps it (VTT-396).
An entry of sequence 1, once it has been sent an entry of its log since it last
discarded, discards what it folded and folds from that entry (VTT-395), so a
redial the gateway opens on the held log from its first entry is folded afresh.
Its cursor is the highest sequence it has been sent of that log since it last
discarded: an entry above it raises it, a discard returns it to 0, and nothing
else moves it, `CatchUpHead` included (VTT-398). A redial dials `after` at that
cursor. For the world it names no `log`, since a DM's or an agent's connection
is sent no `ViewpointChanged` (SPEC-015) and no promotion makes a participant a
DM or an agent (SPEC-009), so nothing is remembered under its token; once a
`ViewpointChanged` has arrived, it names in `log` the actor whose log it holds,
and no `log` for nobody (VTT-397). A discard clears the reason the window gives
for a log it could not fold, and keeps the last `WatchableChanged`. The window
shows as watched the actor the last `ViewpointChanged` named, whatever a
command's result said (VTT-407).

A tab remembers, per token, whose eyes its pages last looked through (the
owner's ruling of 2026-10-22). `startSession` in `client/src/app.ts` writes the
actor each `ViewpointChanged` names, `""` for nobody, to its tab's
`sessionStorage` under `vtt.viewpoint.` followed by its token
(`Session.onViewpoint`), and hands what it finds there to the page's `Session`.
Until the page's first `ViewpointChanged`, every dial, a reconnect included,
names that actor in `log` with `after` 0, and no `log` when it is nobody
(VTT-442). The 0 is the cursor's: a player's or a spectator's connection is
sent its `ViewpointChanged` before any entry (SPEC-015). Until its first
`ViewpointChanged`, a page none of whose earlier pages in its tab under its
token was sent one names no `log`, whatever a page under another token in its
tab (VTT-443) or a page under its token in another tab (VTT-444) was sent.
Until its first `ViewpointChanged` a page shows nobody as watched, not the
remembered actor either (VTT-407). Whether the named log is opened is the
gateway's (SPEC-015).

A player's panel offers a chip for each party member the last
`WatchableChanged` names, in its order, and marks the one they watch (VTT-402);
a set naming none reads "You do not control an actor yet.". A spectator's perch
offers a button for each, ordered by label and then by id, and "No shoulder"
for nobody (VTT-403); it reads "No party members yet." for a set naming none,
and "Perched on: " with the watched one's name from the set, its id when the
set gives no name or does not name it, and "nobody" for nobody. A chip or a
button is labelled by the name the set gives it, or by its id when that name is
empty (VTT-404), and a click sends `set_viewpoint` naming its party member, the
watched one included, or nobody for "No shoulder" (VTT-405); the gateway
applies nothing for the one already shown (SPEC-015). With doors not armed, a
player's board click on a cell of the current scene holding a token of a listed
party member other than the one they watch sends `set_viewpoint` naming the
lowest such id, an armed ability notwithstanding (VTT-406); a click on a cell
holding the watched one's token and no other listed member's sends nothing; any
other click, with no ability armed, moves the watched one's token, if it has
one, to that cell (VTT-408). A player watching nobody holds no log and is shown
no board, since a seat on nobody is sent no entry (SPEC-015). With doors armed,
a board click works a door or sends nothing. A player's selected actor follows
the `ViewpointChanged`, and a switch disarms their armed ability. A spectator's
board click sends nothing (the owner's ruling "Bara knappar", 2026-10-20). A
DM's and an agent's chips select which of the actors granted them a board click
moves and whose abilities the panel offers, and send nothing. Only a spectator
is offered the perch (`mayPerch` in `client/src/app.ts`): a player switches by
chip or by token, and the DM's and the agent's chips switch nothing.

### What a Go reader holds

`internal/harness`'s `Client` hands its reader on `Events()`, a `Delivery`
each, every entry it is sent, whatever its sequence or event id (VTT-427),
and each `ViewpointChanged`, nobody's included, in the order the connection
was sent them (VTT-409). It hands nothing else there: a result answers its
command, `CatchUpHead` is read through `CatchUpHead`, and `Client.readLoop`
drops the other arms, `WatchableChanged` and the presence frames among them.
`Dial` given a log names it in the handshake's `log` beside `after`, and
given none sends no `log` (VTT-410).

The scenario engine (`RunScenario` in `internal/harness/engine.go`) keeps each
seat's history as what it was sent, switches included. A seat holds the log its
last switch names (`heldLog`), and before any switch it holds none. Its current
run (`currentRun`) starts at its last entry of sequence 1 after its last switch
to another log than the one it held, or straight after that switch when no such
entry follows; a seat never told starts it at its last entry of sequence 1, and
has none without one. A character seat whose first entry after a switch to
another log is not of sequence 1 fails the step that drained it (VTT-411,
`logBreak`). A reconnect step drains a character seat's old connection, then
redials through the `Dialer`, which `dialerFor` in `cmd/vtt/client_run.go`
builds, naming in `log` the actor whose log the seat holds, and no `log` when
it holds nobody or was never told (VTT-414). It expects the current run's
entries above the step's cursor when the seat holds an actor's log, and the
whole current run otherwise, since the gateway ignores the cursor of a dial
that names no `log` (VTT-349, `replayWanted`). It fails a redial told another
log when the seat held an actor's log (VTT-412), and compares a character
seat's replay entry by entry, whole (VTT-413), and a world seat's by event id
and sequence, which a world event's unique id allows (VTT-382). A character
seat's history then becomes what it held before its current run, followed, when
the redial names an actor's log, by the run's switches and its entries at or
below the cursor, and then by what the redial sent; a world seat's becomes what
it held at or below the cursor, followed by the replay. An accepted command
step whose result names no sequence waits for no world seat and drains the
character seats (VTT-415). The probes fold the first participant's log
(`probeLog`): its current run when it is a character seat, and its every entry
when it is a world seat.

The soak (`RunSoak` in `internal/harness/soak.go`) keeps what each seat was
sent, switches included. It counts only entries as a leak (`leakedBelow`,
`grewSince`) and when it waits for a sequence (`waitFor`), while `settle` waits
for a seat's whole history, switches included, to stop growing. A checkpoint
fails when a character seat's stream, since the last checkpoint, breaks its
log, a first entry after a switch to another log that is not of sequence 1
included (VTT-359, VTT-411), and that seat is not compared at that checkpoint
or at any later one. After the observer's comparison, `checkLogSeat` takes each
character seat whose stream has not broken its log: it dials the seat naming
the log it holds, or no `log` for nobody or a seat never told (VTT-428), reads
to the highest sequence of the seat's current run, and resumes an overflow with
the same log. The checkpoint fails when the fold of the current run differs
from the fresh dial's (VTT-416), when either does not fold (VTT-431), when the
fresh dial ends short of that sequence (VTT-430), or when the seat was told a
log and the fresh dial is told another, nobody included (VTT-429).

`vtt mcp` (`Server` in `internal/mcp/server.go`) holds the log its connection
was last told: none for a world connection or one never told, `""` for nobody,
an actor's id otherwise. At a `ViewpointChanged` naming another log, the first
included, it discards its history and its cursor (VTT-418); at one naming the
log it holds, it keeps them (VTT-419); both are `recordSwitch`. Its cursor is
the highest sequence it has been sent of that log since it last discarded, and
`recordEvent` keeps no entry at or below it. A redial names in `log` the actor
whose log it holds, and no `log` for the world or nobody (VTT-420), with
`after` at the cursor (VTT-421). `get_state` and `get_events_since` read one
copy of the history, its head and its log (`readSnapshot`), and name the log in
`"log"`: the actor's id, `""` for nobody, and no key while it holds none, as on
a world connection (VTT-422). Their `headSequence` counts within that log, and
is 0 after a discard until an entry of the new log arrives: `get_state`'s
description tells the agent to compare a `headSequence` only with one answered
under the same log, and `get_events_since`'s to page again from `afterSequence`
0 when the log differs from the one its last answer named.

`vtt state dump` (`newStateDumpCmd` and `drainToHead` in
`cmd/vtt/state_dump.go`) dials with `after` 0 and names no `log` (VTT-432), so
it opens on the world for a DM or an agent, and on the seat's default log
otherwise (SPEC-015). It remembers the log the first `ViewpointChanged` names,
and any later `ViewpointChanged` ends its read (SPEC-007), one naming the log
it holds included (VTT-435), since every later `ViewpointChanged` is followed
by its log from the first entry, or by none, for nobody. At one before its
catch-up has reached its head, the dump refuses: it prints no state and exits
non-zero (VTT-423), and its error, which wraps `errMovedBeforeHead`, names the
log it held and the one the seat was moved to, or nobody (VTT-424). At one
after its head, it prints the fold of the log it held (VTT-425). A
`PresenceSnapshot`, `PresenceChanged` or `WatchableChanged` neither ends its
read nor enters its fold (VTT-433). It prints the fold with `headSequence`, the
highest sequence it folded, entries sent after its head included (VTT-434), and
with `"log"` as `get_state` does: none when it read no `ViewpointChanged`, as
on a world connection (VTT-426).

`vtt events tail` (`cmd/vtt/events_tail.go`) dials naming no `log` and prints
each entry it is sent, and no switch.

## Consequences

- A client that holds a character's log names it on a redial: the gateway
  ignores the cursor of a dial that names no `log` and opens on the default,
  from its first entry (SPEC-015).
- A reload under the same token keeps the viewpoint: a page loaded afresh in
  a tab opens on the actor the last `ViewpointChanged` sent to a page in that
  tab under its token named, while the participant may still watch it
  (SPEC-015). Nothing promises it outlives the tab (VTT-444), and a
  duplicated or restored tab, which a browser may give a copy of the tab's
  storage, is neither promised it nor refused it.
- The browser client has no logout: nothing calls `Auth.clear`
  (`git grep -n 'Auth.clear\|auth.clear' client/src` finds only the warning
  in `startSession`). The key carries the token, so a logout must remove the
  key with the token; a page under that token in that tab then names no
  `log`, which VTT-442 forbids, so a logout withdraws VTT-442 and brings a new
  id.
- Whom a connection may watch is the gateway's to decide. The browser client
  offers a switch only to a party member the last `WatchableChanged` names, or
  to nobody on a spectator's perch: every `set_viewpoint` it sends goes through
  `switchTo` in `client/src/app.ts`, which the chips and the perch are handed
  with that set, or through `playerClickFor` in `client/src/view/player.ts`,
  which filters by it (`git grep -n 'setViewpoint(' client/src`). A refused
  `set_viewpoint` changes neither what it holds (VTT-394 to VTT-396) nor whom
  it shows as watched (VTT-407). `vtt mcp`'s `set_viewpoint` tool takes any
  id, and the gateway refuses a perch the participant may not watch
  (SPEC-015).
- VTT-394 to VTT-398 are the browser client's, held by its tests. A Go
  reader's rules are rows of their own; those this record carries are listed
  below.
- The scenario engine compares a character seat's replay whole, not by event id
  and sequence alone as it does a world seat's: a log's own entries carry no
  event id, and the entries forwarded from the world share its ids (SPEC-021).
  The soak compares a character seat's folds rather than its entries.
- A dump never prints the fold of more than one log: one moved before its
  head refuses rather than splice two logs, and the caller can dial again.
- Each soak checkpoint dials afresh every character seat whose stream has
  not broken its log, beside the observer's fresh dial.

## Requirements

VTT-393, VTT-394, VTT-395, VTT-396, VTT-397, VTT-398, VTT-402, VTT-403,
VTT-404, VTT-405, VTT-406, VTT-407, VTT-408, VTT-409, VTT-410, VTT-411,
VTT-412, VTT-413, VTT-414, VTT-415, VTT-416, VTT-418, VTT-419, VTT-420,
VTT-421, VTT-422, VTT-423, VTT-424, VTT-425, VTT-426, VTT-427, VTT-428,
VTT-429, VTT-430, VTT-431, VTT-432, VTT-433, VTT-434, VTT-435, VTT-442,
VTT-443, VTT-444.
