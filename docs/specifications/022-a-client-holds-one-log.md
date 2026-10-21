# SPEC-022: A client holds one log

## Status

Accepted, and partly implemented. The browser client implements it in
`client/src/wire.ts`, `client/src/session.ts`, `client/src/app.ts`,
`client/src/view/spectator.ts` and `client/src/view/player.ts`. That its
window shows no sequence outside the reason it gives for a log it cannot fold
is held by `client/test/app.test.ts`'s "no role's window shows a log sequence
number", `client/test/spectator-view.test.ts`,
`client/test/qa-window-number.test.ts` and `client/e2e/table.spec.ts`
(VTT-393), the last outside `task check`; that it holds one log and a redial
names it, by `client/test/wire.test.ts`, `client/test/session.test.ts`,
`client/test/app.test.ts`, `client/test/qa-one-log.test.ts` and
`client/test/qa-switch.test.ts` (VTT-394 to VTT-398, VTT-407); that a player
and a spectator are offered the party members a `WatchableChanged` names and
switch between them, by `client/test/app.test.ts`,
`client/test/player.test.ts`, `client/test/spectator-view.test.ts` and
`client/test/qa-switch.test.ts` (VTT-402 to VTT-406, VTT-408); that a
spectator chooses a shoulder and a player switches in the browser, by
`client/e2e/table.spec.ts` (VTT-394, VTT-403, VTT-405) and
`client/e2e/handover.spec.ts`, both outside `task check`.

`internal/harness`'s client hands each `ViewpointChanged` to its reader
(VTT-409); its scenario engine and soak hold each character seat's log and
name it when they dial (VTT-411 to VTT-414, VTT-416, VTT-428 to VTT-431), and
`vtt mcp` holds the log it is sent and names it (VTT-418 to VTT-422).
`vtt state dump` does not yet: it skips the switch, and its dial names no
`log`. The change is carried by
`docs/superpowers/specs/2026-10-21-the-go-readers-follow-the-log-they-are-sent-design.md`.

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

### What a client holds, and what it offers

A client holds one log (`client/src/wire.ts`, `client/src/session.ts`): the
world for the DM and the agent, otherwise the log of the actor the last
`ViewpointChanged` named, or none for nobody. A `ViewpointChanged` naming
another actor than the one whose log it holds, or nobody while it holds one,
discards what it folded and folds what follows (VTT-394); one naming the actor
whose log it holds keeps it (VTT-396). An entry of sequence 1, once it has been
sent an entry of its log since it last discarded, discards what it folded and
folds from that entry (VTT-395), so a redial the gateway opens on the held log
from its first entry is folded afresh. Its cursor is the highest sequence it
has been sent of that log since it last discarded: an entry above it raises it,
a discard returns it to 0, and nothing else moves it, `CatchUpHead` included
(VTT-398). A redial dials `after` at that cursor and `log` naming the actor
whose log it holds, and no `log` for the world or for nobody (VTT-397). A
discard clears the reason the window gives for a log it could not fold, and
keeps the last `WatchableChanged`. The window shows as watched the actor the
last `ViewpointChanged` named, whatever a command's result said (VTT-407).

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

## Consequences

- A client that holds a character's log names it on a redial: the gateway
  ignores a cursor that names no `log` and opens on the default, from its
  first entry (SPEC-015).
- Whom a connection may watch is the gateway's to decide. The browser client
  offers a switch only to a party member the last `WatchableChanged` names, or
  to nobody on a spectator's perch: every `set_viewpoint` it sends goes through
  `switchTo` in `client/src/app.ts`, which the chips and the perch are handed
  with that set, or through `playerClickFor` in `client/src/view/player.ts`,
  which filters by it (`git grep -n 'setViewpoint(' client/src`). A refused
  `set_viewpoint` changes neither what it holds (VTT-394 to VTT-396) nor whom
  it shows as watched (VTT-407).
- VTT-394 to VTT-398 are the browser client's, held by its tests. A Go
  reader's rules are rows of their own.

## Requirements

VTT-393, VTT-394, VTT-395, VTT-396, VTT-397, VTT-398, VTT-402, VTT-403,
VTT-404, VTT-405, VTT-406, VTT-407, VTT-408.
