# SPEC-022: A client holds one log

## Status

Accepted. The browser client implements it in `client/src/wire.ts`,
`client/src/session.ts`, `client/src/app.ts`, `client/src/metadata.ts`,
`client/src/view/spectator.ts`, `client/src/view/feed.ts`,
`client/src/view/player.ts`, `client/src/view/naming.ts`,
`client/src/view/grid.ts`, `client/src/view/dm.ts`, `client/src/view/focus.ts`,
`client/src/view/join.ts`, `client/src/commands.ts`, `client/src/player.ts`
and `client/src/style.css`;
the Go readers in `internal/harness/client.go`, `internal/harness/engine.go`,
`internal/harness/soak.go`, `cmd/vtt/client_run.go`'s `dialerFor`,
`internal/mcp/server.go`, `internal/mcp/read_tools.go` and
`cmd/vtt/state_dump.go`. That the browser client's window shows no sequence
outside the reason it gives for a log it cannot fold is held by
`client/test/app.test.ts`'s "no role's window shows a log sequence number",
`client/test/spectator-view.test.ts`, `client/test/qa-window-number.test.ts`
and `client/e2e/table.spec.ts` (VTT-393); that it holds one log and a redial
names it, by `client/test/wire.test.ts`, `client/test/session.test.ts`,
`client/test/app.test.ts`, `client/test/qa-one-log.test.ts`,
`client/test/qa-switch.test.ts`, `client/test/qa-remembered-log.test.ts` and
`client/test/qa-reload-viewpoint.test.ts` (VTT-394 to VTT-398, VTT-407); that
until its first `ViewpointChanged` a page names the actor the last one sent to
a page in its tab under its token named, and no `log` when none was, whatever
another token's or another tab's pages were sent, by
`client/test/wire.test.ts`, `client/test/app.test.ts`,
`client/test/qa-remembered-log.test.ts` and
`client/test/qa-reload-viewpoint.test.ts` (VTT-442 to VTT-444); that only a
DM's and an agent's window shows the Events panel, the newest entries of the
log it holds, newest first and bounded, while every role's window shows in the
Story each entry of the log it holds, both outside the reason it gives for a
log it cannot fold, by `client/test/app.test.ts`,
`client/test/spectator-view.test.ts` and `client/test/qa-events-panel.test.ts`
(VTT-445 to VTT-449); that a player and a spectator are offered the party
members a `WatchableChanged` names and switch between them, by
`client/test/app.test.ts`, `client/test/player.test.ts`,
`client/test/spectator-view.test.ts` and `client/test/qa-switch.test.ts`
(VTT-403 to VTT-405, VTT-408, VTT-457, VTT-460); that a player whose set says
they control no party member is drawn the perch and no panel, that its "No
shoulder" sends nobody, and that their board click sends nothing, by
`client/test/app.test.ts` and `client/test/qa-perched-player.test.ts` (VTT-458,
VTT-459, VTT-461); that a spectator chooses a shoulder and a player switches in
the browser, and that a reload keeps either, by `client/e2e/table.spec.ts`
(VTT-394, VTT-403, VTT-405, VTT-442, VTT-444) and `client/e2e/handover.spec.ts`
(VTT-442); that a player whose last party member the DM takes back keeps
watching it with the perch, chooses nobody by "No shoulder" and then another
shoulder, by `client/e2e/handover.spec.ts` (VTT-458, VTT-459); and that a
player's and a spectator's page shows no Events panel and a DM's does, by
`client/e2e/table.spec.ts` (VTT-445, VTT-446); that a window names what it
shows by its role's chain and never a token or actor by its id, shows a token's
card, and words every payload, by `client/test/naming.test.ts`,
`client/test/token-card.test.ts`, `client/test/spectator-view.test.ts`,
`client/test/app.test.ts`, `client/test/qa-ability-target.test.ts`,
`client/test/qa-what-a-window-shows.test.ts` and `client/e2e/table.spec.ts`
(VTT-474 to VTT-477, VTT-483, VTT-484, VTT-489, VTT-490); and the console's
rewrite, by `client/test/dm-view.test.ts` and
`client/test/qa-describe-token.test.ts` (VTT-464); and that a repaint gives the
box being typed in its focus and selection back, focuses nothing else and
leaves the page scrolled where it was, and that the Say boxes keep what was
typed, `say-text` until the server accepts a line and `say-as` until it is
changed, by `client/test/focus.test.ts`, `client/test/app.test.ts`,
`client/test/spectator-view.test.ts`, `client/test/qa-repaint-focus.test.ts`,
`client/test/qa-say-boxes.test.ts`, `client/test/qa-speaking.test.ts`,
`client/test/qa-refused-line.test.ts`, `client/e2e/typing.spec.ts`,
`client/e2e/qa-repaint.spec.ts` and `client/e2e/refusals.spec.ts` (VTT-491,
VTT-492, VTT-494, VTT-518 and VTT-520 to VTT-522); and that a console field
keeps what was typed until the server accepts the command it fed, that a
press of one of the console's controls that clear a field, or of the panel's
Send or Enter in `say-text`, whose command is unanswered sends nothing, that
a press of one of them that sends changes nothing drawn until its answer, and
that such a send whose promise rejects clears nothing and frees its
control, by `client/test/dm-view.test.ts`, `client/test/app.test.ts`,
`client/test/spectator-view.test.ts`, `client/test/qa-refused-command.test.ts`,
`client/test/qa-refused-line.test.ts` and `client/e2e/refusals.spec.ts`
(VTT-517, VTT-519, VTT-523, VTT-524, VTT-526); and that a player's line
speaks as their party member, `/ooc` steps out of character, a DM's and an
agent's panel draws its speaker box, and a line out of character is drawn
under its speaker in `(( ))`, by
`client/test/spectator-view.test.ts`, `client/test/player.test.ts`,
`client/test/commands.test.ts`, `client/test/feed.test.ts`,
`client/test/qa-speaking.test.ts` and `client/e2e/speaking.spec.ts` (VTT-500
to VTT-502, VTT-504). Every `client/e2e/` file runs outside `task check`.

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

Every role's window shows the Story, which `renderSpectator` draws with
`renderFeed` from `buildFeed` (`client/src/view/feed.ts`): each entry of the
log the window holds (VTT-449). Only a DM's and an agent's window shows the
Events panel, by the owner's ruling B-3 of 2026-10-22, and the agent counts as
the DM there, as it does for the DM's console, by the owner's ruling "Ja, som
DM-konsolen" of 2026-10-23. The panel, `renderTicker`, draws one row per entry,
`describe`'s text, for the newest entries of the log the window holds, the
newest first (VTT-447), and no more rows than the fixed number `renderTicker`
keeps (VTT-448). `paint` in `startSession` asks `renderSpectator` for the panel
through `ViewExtras.ticker`, which it sets to `isDM`, true when the role the
page last read from `/api/me` is `dm` or `agent`, the predicate the console is
drawn by. So a page draws no panel until `/api/me` has answered one of those
two: neither a player's, nor a spectator's, nor one that read any other role or
none, nor one whose first read has not answered or has failed (VTT-445). A page
whose last read answered `dm` or `agent` draws it (VTT-446). The page reads its
role when its session starts (`startSession`) and again at a presence frame
naming its own participant, once a first read has answered; while a re-read is
pending, and after one fails, it keeps the role last read. While the window
gives the reason it cannot fold its log, that reason is all `paint` draws, so
neither the Story nor the panel is shown (VTT-446, VTT-449).

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

A window perches when the role the page last read is `spectator`, or `player`
with a last `WatchableChanged` that says they control no party member, by the
owner's ruling B-4 of 2026-10-22 (`mayPerch` in `client/src/app.ts`). A
perching window is drawn the perch and no player panel (VTT-403, VTT-458).
The DM's and the agent's windows never perch.

A player whose window does not perch is drawn the panel, which offers a chip
for each party member the last `WatchableChanged` names, in its order, and
marks the one they watch (VTT-457). While it offers none it reads "You do not
control an actor yet.": before the client is sent a set, since a set the
gateway sends naming none says the participant controls no party member
(SPEC-007), which perches the window. The perch offers a button for each party
member the set names, and "No shoulder" for nobody (VTT-403, VTT-458). Its
buttons are ordered by label and then by id, each compared by `<`, which is
UTF-16 code unit order rather than a locale's. It reads "No party members
yet." for a set naming none, and "Perched on: " with the watched one's name
from the set, its id when the set gives no name or does not name it, and
"nobody" for nobody. A chip or a button is labelled by the name the set gives
it, or by its id when that name is empty (VTT-404). A click sends
`set_viewpoint` naming its party member, the watched one included, or nobody
for "No shoulder" (VTT-405, VTT-459); the gateway applies nothing for the one
already shown (SPEC-015).

A perching window's board click sends nothing (VTT-461; for a spectator, the
owner's ruling "Bara knappar", 2026-10-20), and its board shows no door tool
armed: `paint` sets `canAct` false for it, hands the board
`canAct && ui.doorsArmed` as its doors bit, and keeps `ui.doorsArmed` itself.
In a player's window that does not perch, with doors not armed, a board click
on a cell of the current scene holding a token of a listed party member other
than the one they watch sends `set_viewpoint` naming the lowest such id, an
armed ability notwithstanding (VTT-460). A click on a cell holding the watched
one's token and no other listed member's sends nothing. Any other click, with
no ability armed, moves the watched one's token, if it has one, to that cell
(VTT-408). A player watching nobody holds no log and is shown no board, since
a seat on nobody is sent no entry (SPEC-015). With doors armed, a board click
works a door or sends nothing. A player's selected actor follows the
`ViewpointChanged`, whether their window perches or not, and a switch disarms
their armed ability. A DM's and an agent's chips select which of the actors
granted them a board click moves and whose abilities the panel offers, and
send nothing.

The client rebuilds the tree it draws at every paint, and the box being typed
in keeps its focus and its selection (the owner's rulings "Återställ fokus",
"Konsol, Say och join", "Ja, preventScroll", "Lägg till overflow-anchor: none"
and "Ja, via activeElement", 2026-10-25). `renderSpectator` for the table and
`renderJoinView` for the join form end in `replaceKeepingFocus` in
`client/src/view/focus.ts`. A box is an input or a textarea carrying a
`data-field`. Every input and textarea the console, the join form and the
player panel draw carries one, and no select or button does. When a box of the
root is `document.activeElement`, in a tab in front or behind, the repaint
gives the new tree's box with the same `data-field` the focus, with
`preventScroll`, and the old box's selection start, end and direction, unless
its type has no selection (VTT-491), and focuses nothing else (VTT-492). The
page stays scrolled where it was, whether the box is above, in or below the
viewport (VTT-494): `preventScroll` keeps the focus from scrolling to the box,
and `#app`'s `overflow-anchor: none` keeps scroll anchoring from moving the
page when the tree is replaced. What a box shows after a repaint comes from
outside the tree: the console's from `draft` in `client/src/view/dm.ts`, or,
for a box with no draft, from the world and the join link the console is drawn
from, or nothing; the join form's from `JoinViewState.name`; and the player
panel's Say boxes from `PlayerUIState`'s `sayAs` and `sayText`. `sayText`
holds what was typed in `say-text` since the server last accepted a line from
the panel, and `sayAs` what was last typed in `say-as`, which no Send and no
answer changes (VTT-522). A focused select or button loses the focus at a
repaint, and so does a box when `paint` draws the reason it cannot fold a log
in its place.

**Saying something.** A player's panel draws one Say box, `say-text`, and no
box for a speaker; a DM's and an agent's draws `say-as` beside it (the owner's
rulings of 2026-10-26, "Ja för DM" and "Ja, som DM"). Send reads the line with
`readLine` in `client/src/player.ts`: a line whose first word is exactly
`/ooc` goes out of character, with the rest of the line, trimmed, as its text,
and Send sends nothing when the rest is empty, nor for an empty line in
character (VTT-501). Any other line goes in character: a player's by
`speakAs`, as the party member the panel acts as, the viewpoint (VTT-500); a
DM's and an agent's by `addNarration`, with what `say-as` holds, trimmed, as
`as`, and no `as` when it is blank (VTT-504). A line out of character goes by
`speakOutOfCharacter`, carrying `say-as` only from a DM's or an agent's panel.
Send, or Enter in `say-text`, leaves `say-as` holding what it held, refused or
accepted, so the next line carries the same `as` until `say-as` is changed
(VTT-521; the owner's ruling "Ja, behåll namnet" of 2026-10-29).
The server writes the name a player's line in character and every line out of
character is shown under, over any `say-as` a DM's or an agent's panel sent
with it; a DM's and an agent's line in character keeps its `as` (SPEC-013).
The Story draws a line out of character as its `as`, `: ` and its text between
`(( ` and ` ))`, with the class `ooc`, and `describe` writes the same for the
Events panel (VTT-502).

**What a command's answer leaves typed** (the owner's rulings "Ja, Say och
alla formulär" and "Töm ändå", 2026-10-29). The console's controls that
clear a field, Start session, Add actor, Remove actor, Add from JSON, Place
token, Remove token, a row's Describe, Save note and a row's Grant, each send
through `sending` in `client/src/view/dm.ts`. The player panel's Send, and
Enter in `say-text`, send through `submit` in `renderPlayerPanel`. Both are
handed `act` from `client/src/app.ts` as `send`: it resolves to the command's
`CommandResult`, shows a refusal as the toast `refused: ` and its error, and
paints at every answer before the console's or the panel's own handling of
that answer runs. Until the answer, what was typed for the command stays in
its field and in `draft` or `PlayerUIState`. When the server accepts it
(`ok`):
- the console clears the drafts of the fields that fed the command, Save
  note's key, title and text excepted, and asks for a repaint through
  `DMDeps.rerender`, which `app.ts` sets to `paint`. Each of those fields is
  then drawn as with nothing typed: empty, the blank option, or for a
  Describe box the token's present text (VTT-519). Remove actor is fed by
  `actor-id` alone and Remove token by `token-id` alone, so the rest of their
  forms stays as typed;
- the panel empties `PlayerUIState.sayText` and the `say-text` box it read
  the line from, and asks for a repaint through its `rerender`, which
  `app.ts` sets to `paint` (VTT-520).

Text typed into those fields after the command was sent is cleared with
them. Any other answer clears nothing and asks for no repaint of its own: a
refusal, and the `ok: false` that `Wire.send` gives a command on a socket that
is not open or that closes before the result arrives, alike (VTT-517,
VTT-518). A refusal is not logged: `git grep -n 'console\.' client/src`
finds only the two handlers of a rejected send.

**One press, one command** (VTT-523). While a command one of those console
controls sent is unanswered, pressing that control again sends nothing; while
a line the panel sent is unanswered, Send and Enter in `say-text` send
nothing. The console keeps its locks in `unanswered`, beside `draft`, so they
last across paints, one per control: keyed by the control's action, by the
action and the row's token or actor id for a Describe or a Grant, and by
`actor-json` for Add from JSON. So another control, or another row's
Describe or Grant, sends meanwhile, Add actor and Remove actor included
though they share `actor-id`. The panel's lock is `PlayerUIState.saying`, one
for Send and Enter. A press the console's own checks refuse, or a line in
which `readLine` finds nothing to send, sends nothing and takes no lock. A
lock is taken when the command is sent and freed at its answer, whatever it
is, a socket's close included, or when the send's promise rejects.

**A press of one of those controls that sends draws nothing new before its
answer** (VTT-524; the owner's ruling "Nej, osynligt lås" of 2026-10-29). A
press of one of them that sends neither clears, nor disables, nor repaints
anything, and a press a lock stops shows no notice and asks for no repaint.
The toast, or the cleared field, follows the answer.

**A send of one of those controls whose promise rejects** clears nothing,
asks for no repaint, frees the control that sent it, and passes its error to
`console.error` (VTT-526; the owner's ruling "console.error(e)" of
2026-10-29).

### How the window names what it shows

`namer` in `client/src/view/naming.ts` names each token and actor the window
shows, except in the panel's chips and the perch's buttons. `renderSpectator`
builds one per paint, from the fold, the log, `ViewExtras.dm` and
`ViewExtras.abilities`, for a disc's initial and its native `title`, its card,
and the Story and the Events panel; `paint` in `startSession` sets `dm` to
`isDM` and `abilities` to the list `/api/ruleset` gave, for every role.
`renderPlayerPanel` builds its own for its target buttons, from the fold alone
with no log, by the role `/api/me` last gave (VTT-475, VTT-484, by the owner's
rulings A-6 and K-1 of 2026-10-22).
- **A window that is not a DM's or an agent's** names a token by its `Name`,
  else its actor's name when the actor is a party member, else `a creature`
  (`UNNAMED`).
- **A DM's or an agent's window** names a token by its `GMName`, else its
  actor's name, else `a creature`.
- **An actor** is named by the first of its tokens, in id order, that its
  window's chain names by the token's own field, else as above. The tokens
  read are those the fold holds and those the window's log placed.
- **What the fold no longer holds** is named as the newest `tokenPlaced`,
  `tokenDescribed` or `actorAdded` entry of the log that named it, so a Story
  line keeps naming what the window was told (VTT-489).
- **The initial** is the naming's first character in upper case, and `?` for
  `a creature`. An ability is named by the display name `/api/ruleset` gives
  it, else its id.

A player's chips and the perch's buttons name a party member as the
`WatchableChanged` set names it, and a DM's or an agent's chips an actor as
the fold holds it, each by its id when that name is empty (VTT-404); the
naming above does not reach them.

**The card.** Each disc carries a `.token-card`: the naming, then, in a DM's
or an agent's window, "players see: " and what a player's window names the
token (`Namer.shown`, VTT-490), then the token's notes, then, in a DM's or an
agent's window, its GM notes, each only when non-empty (VTT-474, VTT-483).
`client/src/style.css` shows it on `.token:hover` and on `.token.pinned`, with
`pointer-events: none`, so a left click still reaches the board's handler and
moves as before (VTT-408; the owner's ruling "Ja, som i dag" of 2026-10-24). A
disc in the right or the bottom half of its scene opens its card toward the
middle (`card-left`, `card-up`), since `.grid` clips what crosses its edge. A
right-click on a disc is prevented and hands its token's id to
`ViewExtras.onCardPin`; `paint` keeps the pinned id across paints, and a
right-click on the pinned disc unpins it.

**The Story's lines.** `describe` in `client/src/view/spectator.ts` words
every case of `Envelope.payload`, and an envelope with none as "something
happened"; it has no `default`, so a payload added to the contract fails
`task client:typecheck` until it is worded (VTT-476). An actor's arrival
reads "<actor> is in the party" or "<actor> comes into sight", a token's
placement "<token> is at x,y", and a move "<token> moved to x,y", with " — "
and its reason when it has one. An ability's line is "<actor> used
<ability>", then, for each of its `outcomes` in order, " on <target>:
<result>, rolled <total>", joined by ";", and with no outcomes " on " and its
targets joined by ", ". It never reads `outcome_summary` or `rolls`, so it
shows what was rolled and never the total that roll was compared with (VTT-477,
by the owner's ruling of 2026-10-24 on the ability's line). A condition, a
resource, a note's key and an adventure's id are shown as the entry names
them.

**The console's rewrite.** `renderDMConsole` in `client/src/view/dm.ts` draws,
when the world holds a token, a "Describe tokens" group with one row per token
of the world, by id, each with the token's id and boxes for its name, notes,
GM name and GM notes, the two notes as textareas. A box with no draft shows
the token's present text. "Describe" sends `describeToken` with the four as
typed, empty ones included (VTT-464; "Ja, en kontroll i konsolen", the owner's
ruling of 2026-10-24). Once the server accepts it, the console clears that
row's drafts and no other row's, so the row's boxes are drawn as with no
draft (VTT-519).

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

- A window that is not a DM's or an agent's names no actor or token by its id
  on a disc, a card, a target button or a Story line; the chips and the perch
  name an unnamed party member by its id (VTT-404). The frames it is sent
  still carry the ids, and `outcome_summary` (SPEC-007).
- A payload added to the contract needs a line in `describe` before the
  client typechecks.
- An input or a textarea added to a view needs a `data-field` unique in the
  tree it is drawn in, or a repaint takes its focus, or hands the focus to the
  first box in tree order with the same field.
- Text a box must keep across a repaint needs a home outside the tree, as
  `draft`, `JoinViewState` and `PlayerUIState` are; a box keeps none itself.
- A view handed `act` that changes what it draws at an answer asks for its
  own repaint: `act`'s paint runs before the view's handling of the answer.
- A command whose answer never comes holds the control that sent it:
  nothing in `client/src` times a command out (`git grep -n
  'setTimeout\|setInterval' client/src` prints nothing).

- The Events panel is no boundary: it shows only entries of the log the window
  holds, and the window's Story shows each of them. What a connection is sent
  is SPEC-015's.
- A page whose first read of `/api/me` fails is not read again until it is
  loaded again, so a DM's or an agent's page whose first read fails shows no
  Events panel until it is loaded again.
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
  to nobody on the perch: every `set_viewpoint` it sends goes through
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

VTT-393, VTT-394, VTT-395, VTT-396, VTT-397, VTT-398, VTT-403, VTT-404,
VTT-405, VTT-407, VTT-408, VTT-409, VTT-410, VTT-411, VTT-412, VTT-413,
VTT-414, VTT-415, VTT-416, VTT-418, VTT-419, VTT-420, VTT-421, VTT-422,
VTT-423, VTT-424, VTT-425, VTT-426, VTT-427, VTT-428, VTT-429, VTT-430,
VTT-431, VTT-432, VTT-433, VTT-434, VTT-435, VTT-442, VTT-443, VTT-444,
VTT-445, VTT-446, VTT-447, VTT-448, VTT-449, VTT-457, VTT-458, VTT-459,
VTT-460, VTT-461, VTT-464, VTT-474, VTT-475, VTT-476, VTT-477, VTT-483,
VTT-484, VTT-489, VTT-490, VTT-491, VTT-492, VTT-494, VTT-500, VTT-501,
VTT-502, VTT-504, VTT-517, VTT-518, VTT-519, VTT-520, VTT-521, VTT-522,
VTT-523, VTT-524, VTT-526.
