# SPEC-022: A client holds one log

## Status

Accepted. The browser client implements it in `client/src/wire.ts`,
`client/src/session.ts`, `client/src/app.ts`, `client/src/metadata.ts`,
`client/src/view/spectator.ts`, `client/src/view/feed.ts`,
`client/src/view/player.ts`, `client/src/view/naming.ts`,
`client/src/view/grid.ts`, `client/src/view/dm.ts`, `client/src/view/focus.ts`,
`client/src/view/join.ts`, `client/src/commands.ts`, `client/src/player.ts`,
`client/src/fold.ts`, `client/src/state.ts` and `client/src/style.css`;
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
`client/test/qa-reload-viewpoint.test.ts` (VTT-443, VTT-444, VTT-579); that only a
DM's and an agent's window shows the Events panel, the newest entries of the
log it holds, newest first and bounded, while every role's window shows in the
Story the entries of the log it holds of the kinds `STORY` marks drawn, a
door only with an event id, but the runs that withdraw and introduce again one
actor, both outside the reason it gives for a log it cannot fold, by
`client/test/app.test.ts`, `client/test/spectator-view.test.ts`,
`client/test/feed.test.ts`, `client/test/qa-events-panel.test.ts`,
`client/test/qa-companion-first-sight.test.ts` and
`client/test/qa-story-mechanical.test.ts` (VTT-445 to VTT-448, VTT-809,
VTT-810, VTT-812, VTT-818 to VTT-820, VTT-822, VTT-823), and that the Events
panel labels a move with its reason, by `client/test/spectator-view.test.ts`,
`client/test/qa-move-reason.test.ts` and `client/test/qa-events-panel.test.ts`
(VTT-821); that a player and a spectator are offered the party
members a `WatchableChanged` names and switch between them, by
`client/test/app.test.ts`, `client/test/player.test.ts`,
`client/test/spectator-view.test.ts` and `client/test/qa-switch.test.ts`
(VTT-403 to VTT-405, VTT-408, VTT-457, VTT-460); that a player whose set says
they control no party member is drawn the perch and no panel, that its "No
shoulder" sends nobody, and that their board click sends nothing, by
`client/test/app.test.ts` and `client/test/qa-perched-player.test.ts` (VTT-458,
VTT-459, VTT-461); that a spectator chooses a shoulder and a player switches in
the browser, and that a reload keeps either, by `client/e2e/table.spec.ts`
(VTT-394, VTT-403, VTT-405, VTT-444, VTT-579) and `client/e2e/handover.spec.ts`
(VTT-579); that a player whose last party member the DM takes back keeps
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
typed, `say-text` until the server accepts a line and the console's `say-as`
until it is changed, by `client/test/focus.test.ts`, `client/test/app.test.ts`,
`client/test/spectator-view.test.ts`, `client/test/dm-view.test.ts`,
`client/test/qa-repaint-focus.test.ts`, `client/test/qa-say-boxes.test.ts`,
`client/test/qa-speaking.test.ts`, `client/test/qa-refused-line.test.ts`,
`client/test/qa-console-say.test.ts`, `client/test/qa-panel-say.test.ts`,
`client/e2e/typing.spec.ts`, `client/e2e/qa-repaint.spec.ts` and
`client/e2e/refusals.spec.ts` (VTT-491, VTT-492, VTT-494, VTT-518, VTT-520
to VTT-522, VTT-534); and that a console field keeps what was typed until
the server accepts the command it fed, that a press of one of the console's
controls that clear a field, Send and Enter in the console's `say-text`
among them, or of a player's panel's Send or Enter in `say-text`, whose
command is unanswered sends nothing, that such a command holds back no other
console control, the console's Send and Enter counting as one, that a press
of one of them that sends changes nothing drawn until its answer, that at a
refusal of what one of those controls sent neither the console nor the
panel asks for a repaint of its own, and that such a send whose promise
rejects clears nothing and frees its control, by
`client/test/dm-view.test.ts`, `client/test/app.test.ts`,
`client/test/spectator-view.test.ts`, `client/test/qa-refused-command.test.ts`,
`client/test/qa-refused-line.test.ts`, `client/test/qa-say-boxes.test.ts`,
`client/test/qa-console-say.test.ts`, `client/test/qa-panel-say.test.ts` and
`client/e2e/refusals.spec.ts` (VTT-517, VTT-523, VTT-524, VTT-526, VTT-533,
VTT-535, VTT-536, VTT-537, VTT-538, VTT-541); and that
the DM console draws a Say whatever its DM or agent controls, that the
player panel draws one for a player alone and no trace of one for a DM or an
agent, that both line boxes name `/ooc`, that a player's line speaks as
their party member and the console's as what `say-as` holds, that `/ooc`
steps out of character, and that a line out of character is drawn under its
speaker in `(( ))`, by `client/test/dm-view.test.ts`,
`client/test/spectator-view.test.ts`, `client/test/app.test.ts`,
`client/test/player.test.ts`, `client/test/commands.test.ts`,
`client/test/feed.test.ts`, `client/test/qa-speaking.test.ts`,
`client/test/qa-console-say.test.ts`, `client/test/qa-panel-say.test.ts`,
`client/e2e/refusals.spec.ts` and `client/e2e/speaking.spec.ts` (VTT-500 to
VTT-502, VTT-527, VTT-528, VTT-530 to VTT-532, VTT-540, VTT-600); and that a
window the gateway closes
for a dead credential, or whose first read of `/api/me` is answered 401,
says it was removed in place of Reconnect, forgets its own stored credential
and its tab's viewpoint and keeps another window's credential, that any
other close, and a first read of `/api/me` that fails other than with 401,
forget nothing, and that the share link then opens the join form, by
`client/test/wire.test.ts`, `client/test/app.test.ts`,
`client/test/qa-removed-window.test.ts` and `client/e2e/booting.spec.ts`
(VTT-572 to VTT-577, VTT-580); and that `readLine` reads a whisper after
`/w`, `/whisper`, `/dm`, `/gm` or `/togm`, that the console whispers to a
party member or to a participant's party members and tells what it cannot,
that a player's panel whispers to the DM, says no `/w` line aloud and tells
nothing for a blank line or a bare `/ooc`, and that the Story and the Events
panel draw a whisper, by `client/test/player.test.ts`,
`client/test/dm-view.test.ts`, `client/test/spectator-view.test.ts`,
`client/test/app.test.ts`, `client/test/commands.test.ts`,
`client/test/qa-whisper.test.ts`, `client/test/qa-whisper-say.test.ts`,
`client/test/qa-survivor-rules.test.ts` and `client/e2e/whispering.spec.ts`
(VTT-593 to VTT-597, VTT-599, VTT-600, VTT-666, VTT-668); and that a window
shows a refusal as the server words it, tells a player each party member a
set stops naming as controlled, and nothing at any other set or in any other
window, and tells a perching player that they play no character, by
`client/test/app.test.ts`, `client/test/session.test.ts`,
`client/test/spectator-view.test.ts`, `client/test/qa-refusal-words.test.ts`
and `client/e2e/handover.spec.ts` (VTT-604 to VTT-607). That the panel
offers, for the actor it acts as, only the abilities of the window's list
that the actor's sheet names, in a player's, a DM's and an agent's window,
is held by `client/test/panel-offers.test.ts`,
`client/test/qa-window-offers.test.ts`,
`client/test/qa-ability-target.test.ts`, `client/test/qa-panel-say.test.ts`
and `client/test/spectator-view.test.ts` (VTT-695); that a window reads
`/api/ruleset` again when a player's set or its flag changes, or when a
change of role makes it a player's window or stops it being one, and that
its panel offers from its newest read's answer, by `client/test/app.test.ts`
and `client/test/qa-window-reread.test.ts` (VTT-696, VTT-712, VTT-753); and
that the fold holds an actor's ability ids as its `actorAdded` gave them
until a give or a take changes them and its dump writes them as
`vtt state dump` does, by `client/test/fold-dump.test.ts` and
`client/test/qa-actor-ability-ids.test.ts`, and the server's fold by
`internal/engine/qa_actor_ability_ids_test.go` (VTT-704, VTT-705,
VTT-737). That the fold gives and takes an ability as `engine.Apply` does,
refusals included, and that a log of gives and takes dumps as
`vtt state dump` prints it, is held by `client/test/fold-abilities.test.ts`,
`client/test/qa-ability-change.test.ts` and `client/test/fold-parity.test.ts`,
beside `internal/engine/ability_change_test.go`,
`internal/engine/qa_ability_change_test.go`,
`cmd/vtt/ability_change_dump_test.go` and
`internal/harness/fold_golden_test.go` (VTT-715, VTT-717, VTT-720 to
VTT-722, VTT-727); that the fold adds and removes a resource as
`engine.Apply` does, refusals included, and that a log of them dumps as
`vtt state dump` prints it, by `client/test/fold-resources.test.ts`,
`client/test/qa-resource-change.test.ts` and
`client/test/fold-parity.test.ts`, beside
`internal/engine/resource_change_test.go`,
`internal/engine/qa_resource_change_test.go`,
`cmd/vtt/resource_change_dump_test.go` and
`cmd/vtt/qa_resource_change_dump_test.go` (VTT-769 to VTT-774); that the
Events panel words a give and a take, by
`client/test/spectator-view.test.ts` and
`client/test/qa-ability-change.test.ts` (VTT-813), and a resource added and
removed, by `client/test/spectator-view.test.ts` and
`client/test/qa-resource-change.test.ts` (VTT-814); that the DM console
offers and sends both, by `client/test/dm-abilities.test.ts`,
`client/test/qa-dm-abilities.test.ts`, `client/test/app.test.ts` and
`client/test/command-surface.test.ts` (VTT-730), and asks for and sends a
give's value, by `client/test/dm-abilities.test.ts`,
`client/test/qa-give-value.test.ts` and `client/test/commands.test.ts`
(VTT-788, VTT-789, VTT-805 to VTT-808); and that a player's window
reads `/api/ruleset` again when a set changes a party member's ability ids
and tells the player what changed, naming each ability by every list read
since load, while no other window does either, by
`client/test/sheet-news.test.ts`, `client/test/qa-sheet-news.test.ts`,
`client/test/session.test.ts` and `client/test/naming.test.ts` (VTT-732 to
VTT-734, VTT-752). That an older read's answer arriving after the newest's
names an ability in the Story at once, by the name it gives, while the
player panel's ability button keeps naming the resource it spends by the name
the newest gave it, and that a read with no change to tell leaves the message
standing, is held by `client/test/sheet-news.test.ts` and
`client/test/qa-older-answer.test.ts` (VTT-734, VTT-758, VTT-759, VTT-817).
That a window names
a resource its ruleset declares by the property's display name and any other
by its key is held by `client/test/naming.test.ts`,
`client/test/spectator-view.test.ts`, `client/test/app.test.ts` and
`client/test/qa-property-names.test.ts` (VTT-815, VTT-816). That the
console's paste refuses a resource whose `current` has no value, and
passes a `current` of `0` and a resource with no maximum, is held by
`client/test/commands.test.ts` and `client/test/qa-add-actor-paste.test.ts`
(VTT-672, VTT-679). Every `client/e2e/` file runs outside `task check`.

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
VTT-435). That `vtt events tail` prints no switch is held by no row. That a
disc's resource chips are drawn in a DM's and an agent's window alone and
its condition dots in every window is held by `client/test/grid.test.ts`,
`client/test/app.test.ts`, `client/test/qa-chips-by-role.test.ts`,
`client/test/qa-companion-first-sight.test.ts` and `client/e2e/table.spec.ts`
(VTT-834, VTT-835); that the player panel is the character's sheet, laid out
as the window's ruleset declares, by `client/test/panel-sheet.test.ts`,
`client/test/spectator-view.test.ts`,
`client/test/qa-panel-sheet.test.ts`, `client/test/sheet.test.ts`,
`client/test/qa-sheet-model.test.ts`, `client/test/app.test.ts` and
`client/e2e/table.spec.ts` (VTT-836 to VTT-846); and that an ability's
button is titled by its description and its cost line, by
`client/test/panel-sheet.test.ts`, `client/test/app.test.ts`,
`client/test/spectator-view.test.ts`, `client/test/qa-panel-sheet.test.ts`,
`client/test/qa-ability-title.test.ts` and `client/e2e/table.spec.ts`
(VTT-854, VTT-855).

## Principles served

Principles 7, 11, 12, 13, 14, 22, 23, 27 and 35 of `docs/blueprint.md`:

- **7.** A grant, a revocation, a change of role, a give or a take reaches
  the abilities a player's window offers without a reload.
- **11.** A reader folds one log, from its first entry, and never splices two.
- **12.** A player is told when a party member stops being theirs, when an
  ability is given to or taken from a party member they control, and a
  perching player that they play no character.
- **13.** `namer` never shows a token or an actor by its id, names a resource
  its ruleset declares by its display name, and `describe` words every
  payload (VTT-474 to VTT-477, VTT-815).
- **14.** A player's click on a cell holding the watched one's token and no
  other listed member's sends nothing.
- **22.** A reload keeps the viewpoint.
- **23.** With a sheet declared, the panel draws a character's sheet as the
  window's ruleset lays it out, and invents no grouping of its own; with a
  sheet or without, it writes no word of its own for an ability that spends
  no resource.
- **27.** The panel offers a character only the abilities its sheet names.
- **35.** An ability's button is titled by the ruleset's description of
  it, so a player can read what an ability does where they choose it.

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
`renderFeed` from `buildFeed` (`client/src/view/feed.ts`): the entries of the
log the window holds that `told` keeps, but the runs `reintroductions` names,
below (VTT-822).
Only a DM's and an agent's window shows the
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
neither the Story nor the panel is shown (VTT-446, VTT-822).

The Story is the game's story, and what is mechanical goes to the Events
panel, by the owner's rulings of 2026-11-16. `told` reads `STORY` in
`client/src/view/feed.ts`, a table over every case of `Envelope.payload`, so
a payload added to the contract fails `task client:typecheck` until the
table says whether the Story draws it. The Story draws a `sessionStarted`, a
`sessionEnded`, an `actorAdded`, an `actorRemoved`, a `conditionApplied`, a
`conditionRemoved`, an `abilityUsed`, an `attackRolled`, a `narrationAdded`
and a `whisperAdded` (VTT-822), and an envelope with no payload. It draws no
`resourceChanged`, `abilityGiven`, `abilityTaken`, `resourceAdded` or
`resourceRemoved` (VTT-809), no `actorControlGranted` or
`actorControlRevoked` (VTT-810), no `tokenPlaced`, `tokenMoved`,
`tokenRemoved`, `tokenDescribed`, `sceneCreated`, `sceneSeen`,
`noteUpserted`, `noteDeleted` or `adventureLoaded` (VTT-818), and no
`tokenHidden` (VTT-823), with an event id or none. It draws a `doorOpened`
or a `doorClosed` only when the entry carries an event id (VTT-820): one with
none is the projection's correction of a door the log believed otherwise
(SPEC-016). Of the writers
`git grep -n 'Envelope_DoorOpened{\|Envelope_DoorClosed{' -- internal cmd`
finds, `doorTransitions` in `internal/perceive/projection.go` is the one that
sets no event id.

`buildFeed` leaves out, before it groups, each run `reintroductions` names,
found in the whole log, and then each entry `told` refuses. A run is an
`actorRemoved` with no event id that the `actorAdded` of the same actor
directly follows, that `actorAdded`, and each of that actor's
`actorControlGranted` and `conditionApplied` entries with no event id behind
it, up to the first entry that is not one: the order in which `introduce`
writes an introduction's grants and conditions (SPEC-016). In a character log
such a run opens with the projection's own withdrawal and introduction again of
one actor, never a removal the log's character saw: a bare `ActorRemoved` is
sent only directly before an introduction of the same id (VTT-252), and no
character log is written a forwarded `ActorRemoved` (SPEC-016). So at a
companion's first sight, at a kind change, at a correction that re-introduces
and at an id the world adds again as another, the Story draws none of the run's
lines: neither "<actor> left" nor "<actor> is in the party" nor "<actor> comes
into sight" nor its conditions' lines. It draws the entries around it that
`told` keeps. Since the runs are found before `told` leaves an entry out, a
correction's sheet frames behind a run end it, and the bare conditions behind
them are drawn. Every entry of the world log carries an event id:
`Store.appendLocked` in `internal/store/store.go` is the only writer of the
`events` table outside tests, only `Store.AppendBatch` calls it, and
`AppendBatch` refuses an envelope without one. So the Story of a window that
holds the world log leaves out no run and no door, only the kinds `STORY`
refuses. A narration anchored only to entries the Story leaves out is drawn in
a beat of its own, at its own place in the log (VTT-812). The fold, the namer
and the Events panel are handed the whole log.

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
names that actor in `log` with `after` 0, and no `log` when it is nobody or
the tab has forgotten it (VTT-579). The 0 is the cursor's: a player's or a
spectator's connection is
sent its `ViewpointChanged` before any entry (SPEC-015). Until its first
`ViewpointChanged`, a page none of whose earlier pages in its tab under its
token was sent one names no `log`, whatever a page under another token in its
tab (VTT-443) or a page under its token in another tab (VTT-444) was sent.
Until its first `ViewpointChanged` a page shows nobody as watched, not the
remembered actor either (VTT-407). Whether the named log is opened is the
gateway's (SPEC-015).

A window learns two ways that its participant was removed. `Wire` compares
each close with `CREDENTIAL_GONE` in `client/src/wire.ts`, the code 1008 and
the reason `gateway: credential no longer valid` both, and on a match calls
what `Session.onCredentialGone` was handed; that pair is the one
`contract/testdata/credential_close.json` holds, and tests on both sides of
the wire hold their constants to it (SPEC-011). And the chain `startSession`
runs at load, `/api/me` and then the ruleset, the adventures and the maps,
ends in a catch that acts on a 401 from any of them and on nothing else.
Either way `forget` in `startSession` runs: it clears the stored credential
(`Auth.clear`) only while it is still the window's own token, since another
window of the browser may have stored its own since (VTT-574); removes the
tab's `vtt.viewpoint.` key under that token (VTT-575); and sets `removed`,
which makes `renderStatus` in `client/src/view/spectator.ts` draw "You were
removed from this table. To join again, open the link your DM sent you." in
place of Reconnect (VTT-572, VTT-573). The board, the Story and the panels
stay as last drawn. A close with any other code or reason, and a failure of
the chain other than a 401, forget nothing and draw no removal (VTT-576,
VTT-580). With no credential stored, the share link opens the join form
(VTT-577).

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
member the set names, and "No shoulder" for nobody (VTT-403, VTT-458).
Under its heading, a player's perch reads "You play no character; the DM can
give you one." and a spectator's does not: `app.ts` sets
`PerchControl.playsNone` from the role, so a reload draws it again
(VTT-607). Its buttons are ordered by label and then by id, each compared by
`<`, which is UTF-16 code unit order rather than a locale's. It reads "No
party members yet." for a set naming none, and "Perched on: " with the
watched one's name from the set, its id when the set gives no name or does
not name it, and "nobody" for nobody. A chip or a button is labelled by the
name the set gives it, or by its id when that name is empty (VTT-404). A
click sends `set_viewpoint` naming its party member, the watched one
included, or nobody for "No shoulder" (VTT-405, VTT-459); the gateway
applies nothing for the one already shown (SPEC-015). `switchTo` shows a
refusal of it as the toast of its error, with nothing before it (VTT-604).

When a `WatchableChanged` stops naming as controlled some party members the
previous set named so, the `Session` in `client/src/session.ts`, comparing by
id, hands them as the previous set names them, in its order, to each handler
`onPartyMembersLeft` registered, before the set is replaced. A set names party
members as controlled only when it does not say the participant controls none,
so a page's first set, an equal set and a renamed one hand over nobody, and,
since a discard keeps the last set, so does a redial sent the same one. A
window whose role the page last read is `player` then shows the toast "You no
longer play " and their names joined by " and ", each "an unnamed character"
when its name is empty, and "." (VTT-605, VTT-606; the owner's ruling "Toast +
rad i perch-vyn" of 2026-11-02). Any other window is told nothing.

When a `WatchableChanged` changes the ability ids of party members the
previous set also names, the `Session` hands each handler `onSheetChanged`
registered one `SheetChange` per such party member, in the new set's order
(`sheetChanges`): its id, the name the new set gives it, the ids the new set
names that the previous did not, in the new set's order, and those the
previous named that the new does not, in the previous set's order, each id
once. It hands them after `onPartyMembersLeft`'s handlers and before the
set is replaced, and hands nothing when either set says the participant
controls no party member. So a page's first set, an equal set, a renamed
one, a set whose ids differ from the last only in their order and a set
beside a true flag hand over nothing, and, since a discard keeps the last
set, a redial sent a set that changed meanwhile hands over the change
(VTT-733). A window whose role the page last read is `player` keeps each
change until the `/api/ruleset` read the set issues (below) has answered or
failed, and then shows as the toast what `sheetNews` in
`client/src/view/naming.ts` writes of them: per change, in order, "<Name>
can now use <A>." for the ids given and "<Name> can no longer use <B>."
for the ids taken, several abilities joined by " and " and the sentences by
a space, the name the new set gives, else "An unnamed character", and each
ability by the display name some answer since the page loaded gave it,
else by its id (VTT-733, VTT-734; the owner's rulings OD-3 and OD-5 of
2026-11-14). Any other window shows nothing for such a set and issues no
read for it (VTT-752).

The fold, `client/src/fold.ts`, holds an actor's `abilityIds` as its
`actorAdded` gave them, in their order, repeats included, and an empty list
for an actor added with none (`copyActor`, into the state's `Actor`), until
an `abilityGiven` or an `abilityTaken` changes them; a control grant, a
control revoke and a resource change leave the list unchanged, as
`engine.Apply` does (VTT-705, VTT-737). Its `abilityGiven` arm refuses an
empty ability id, then an actor the fold does not hold, then an id the
actor's list names, and otherwise sets the list to a new array ending with
the id; its `abilityTaken` arm refuses an actor the fold does not hold, then
an id the list does not name, and otherwise sets it to a new array holding
no occurrence of the id, the others in their order. Each refusal is a
`FoldError` in the client's own words, and the two arms refuse and change
what `engine.Apply`'s do (SPEC-025), nothing else of the actor or the state
included (VTT-715, VTT-717, VTT-720, VTT-721, VTT-722). `foldToDumpJSON`
writes the list as `ability_ids`, after the actor's `kind`, in its order,
and not at all when it is empty, as `vtt state dump` does (VTT-704), so the
client's fold of a log of gives and takes dumps as `vtt state dump` prints
it, its refusals included (VTT-727).

Its `resourceAdded` arm refuses a resource name shorter than 1 or longer than
128 bytes (SPEC-018), then an event with no value, then an actor the fold
does not hold, then a resource the actor already holds, and otherwise sets
the actor's resources to a new map holding the resource at the event's
current and max, whatever their numbers; its `resourceRemoved` arm refuses
an actor the fold does not hold, then a resource the actor does not hold,
and otherwise sets them to a new map without it (`withResources`). Each
refusal is a `FoldError` in the client's own words, and the two arms refuse
and change what `engine.Apply`'s do (SPEC-025), nothing else of the actor or
the state included (VTT-769 to VTT-773), so the client's fold of a log of
resources added and removed dumps as `vtt state dump` prints it, its
refusals included (VTT-774).

The panel offers abilities for the actor it acts as: a player's the party
member they watch, a DM's and an agent's the actor its chip selects. It
offers one button for each ability of the window's list that the actor's
sheet names, and none for any other (`offered` and `sheetNames` in
`client/src/player.ts`), in a player's, a DM's and an agent's window alike,
by principle 27 and the owner's ruling of 2026-11-05 (VTT-695). Where each
button stands is the sheet's (below): with a sheet declared, on the page
whose section names its usage, ordered by section and then by the list's
order; with none declared, in the "Abilities" section, in the list's order.
With none offered it draws no "Abilities" heading. It draws
target buttons only for a selection it offers, and a board click counts a
selection as armed only when the acting actor's sheet names it
(`moveCommandFor`). The window's list is the last one `/api/ruleset` gave
the window's participant in answer to what was then the window's newest read
(SPEC-012): every ability for a DM and an agent, and for a player those named
by the sheets of the actors they control.

`startSession` reads `/api/ruleset` in its start-up chain, after `/api/me`
(`readRuleset`), and again through `rereadIfStale`, which it runs at every
change of its session's state and after each read of `/api/me` that a
presence frame naming its participant starts. `rereadIfStale` reads again
when the window's key differs from the key it held when it issued its
newest read: for a window whose role the page last read is `player`, each
actor of the last `WatchableChanged` set, in its order, by its id and the
ability ids the set gives it, in their order, and whether the set says the
participant controls no party member; for any other window, the empty key.
So a player's window reads again when a set changes which party members it
names or its flag (VTT-696), or the ability ids of a party member it names
(VTT-732), as at a give or a take to a party member the player controls
(SPEC-015), and a window reads again when the role `/api/me` answers makes
it a player's window or stops it being one (VTT-712); an equal set, a
renamed set and a redial sent the same set issue no read, a set whose ids
differ from the last only in their order issues one, and a window that is
not a player's reads once, at start-up, until a promotion makes it one, a
set that changes ability ids included (VTT-752). A grant or a revocation
that leaves its actor not a party member, and a give or a take to an actor
that is not one, change neither the set nor its flag, so they issue no read
and tell nothing, and the window's list keeps, or lacks, the abilities that
actor's sheet alone names until a change of the key or a reload reads again; a
player's panel never acts as such an actor. The newest read's answer sets the
window's abilities, from which the panel offers and the console gives, and its
properties, and an answer to an older read arriving after it sets neither
(VTT-753). Every answer, an older read's included, adds the abilities it gives
to `everRead` in `client/src/app.ts`, kept by id, an answer arriving later
replacing an earlier one's entry, by which the Story, the Events panel and the
toast name an ability (below); a reload forgets it. So an older read's
answer arriving after the newest's replaces, in the Story, the name the newest
gave an ability both name (VTT-759), and leaves the window's properties, by
which the player panel's ability button names the resource it spends, as the
newest set them (VTT-817). The window
draws itself again at every answer and every failure of a read, so a name an
older read's answer adds reaches the Story at once (VTT-734). A read the window
issued for no change of ability ids has nothing to tell and sets no message,
so its answer or failure leaves the message the window shows standing
(VTT-758). A read `rereadIfStale`
issued that fails keeps the window's lists and forgets nothing, a 401
included, and no read is issued again until the key changes; of the ruleset's
reads, only the start-up chain's, through that chain's catch, forgets the
credential on a
401 (above), since `grep -n 'readRuleset()' client/src/app.ts` finds the
chain's call and `rereadIfStale`'s, which catches every failure.

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
(VTT-408); a selection the watched one's sheet does not name is not armed,
so such a click moves (`client/test/qa-window-offers.test.ts`'s "a
player's board click with a carried-over ability the sheet does not name
moves the watched token"). A player watching nobody holds no log and is
shown no board, since a seat on nobody is sent no entry (SPEC-015). With
doors armed, a board click works a door or sends nothing. A player's
selected actor follows the `ViewpointChanged`, whether their window perches
or not, and a switch disarms their armed ability. A DM's and an agent's
chips select which of the actors granted them a board click moves and whose
abilities the panel offers, and send nothing.

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
from, or nothing; the join form's from `JoinViewState.name`; and a player's
panel's `say-text` from `PlayerUIState.sayText`. The console's Say keeps both
its boxes in `draft` (the owner's ruling "Ja, båda" of 2026-10-30):
`say-text` shows what was typed in it since the server
last accepted a line from the console (VTT-534), and `say-as` what was last
typed in it, which no Send and no answer changes (VTT-522). `sayText` holds
what was typed in the panel's `say-text` since the server last accepted a
line from the panel (VTT-522). A focused select or button loses the focus at
a repaint, and so does a box when `paint` draws the reason it cannot fold a
log in its place.

**Saying something.** The DM console, which `paint` draws for the DM and the
agent (`isDM`), draws a Say first, under its "DM console" heading and above
Session, whatever its DM or agent controls (VTT-527; the owner's rulings "I
konsolen, alltid" and "Först, över Session" of 2026-10-30). The group,
headed "Say something", holds a speaker box `say-as`, in which the DM and
the agent name any speaker (the owner's rulings of 2026-10-26, "Ja för DM"
and "Ja, som DM"), a line box `say-text` whose placeholder reads "say or
narrate, /ooc to step out of character, /w Name to whisper", and Send
(`add-narration`);
the three keep the panel's names by the owner's ruling "Ja, båda" of
2026-10-30. The player panel draws a Say for a player alone: one line box,
`say-text`, whose placeholder reads "say something, /ooc to step out of
character, /dm to whisper to the DM", Send (`add-narration`), and no box for
a speaker (VTT-500,
VTT-528, VTT-532). `renderPlayerPanel` returns before its Say for any role but
`player`, so a DM's or an agent's panel draws no Say heading, no line box, no
speaker box, no Send, and no line pointing to the console's Say (VTT-540; the
owner's ruling "Inget" of 2026-10-30).

Send, and Enter in `say-text`, read the line with `readLine` in
`client/src/player.ts`, in the console and in a player's panel alike, as one
of four kinds. A line whose first word is exactly `/ooc` goes out of
character, with the rest of the line, trimmed, as its text, and nothing is
sent when the rest is empty, nor for an empty line in character (VTT-501,
VTT-530, VTT-531); a player's panel tells the player nothing for either
(VTT-666). A line whose first word is `/w` or `/whisper`, in any case, is a
whisper to the next word, with the rest of the line, trimmed, as its text;
when that word opens with a double quote, to what stands between that quote
and the next one, trimmed, with what follows the closing quote, trimmed, as
its text, and when no later quote closes it, to what follows the opening
quote, trimmed, with no text. One whose first word is `/dm`, `/gm` or
`/togm`, in any case, is a whisper to the DM (VTT-668, VTT-599; the owner's
rulings "/w och /dm i Say-rutan" and "Ja, som alias" of 2026-11-01). Any
other line goes in character: a player's by `speakAs`, as the party member
the panel acts as, the viewpoint (VTT-500); the console's by `addNarration`,
naming no actor, with what `say-as` holds, trimmed, as `as`, and no `as` when
it is blank (VTT-600). A line out of character goes by
`speakOutOfCharacter`, carrying `say-as` the same way from the console, and
no `as` from a player's panel. Send, or Enter in the console's `say-text`,
leaves `say-as` holding what it held, refused or accepted, so the next line
carries the same `as` until `say-as` is changed (VTT-521; the owner's ruling
"Ja, behåll namnet" of 2026-10-29). The server writes the name a player's
line in character and every line out of character is shown under, over any
`as` the console sent with it; the console's line in character keeps its
`as` (SPEC-013). The Story draws a line out of character as its `as`, `: `
and its text between `(( ` and ` ))`, with the class `ooc`, and `describe`
writes the same for the Events panel (VTT-502).

**Whispering.** The console's Say sends a whisper with `addWhisper`, and never
reads `say-as` for it, so a whisper always comes from the DM (the owner's
answer (a) to the plan's Q1, at its sign-off of 2026-11-01). `whisperTargets`
in `client/src/view/dm.ts` resolves the name: to the party member whose name
equals it without regard to case; else to every party member controlled by the
participant whose name equals it so, as the console knows the participants for
the grant select, the roster and presence together, in actor-id order
(VTT-593). An unknown name, a name two party members share, a participant who
controls no party member, a whisper with no name or no text, and a whisper to
the DM are told through `DMDeps.notify`, send nothing and leave `say-text` as
typed (VTT-594). A player's panel sends a `/dm` line with text after it as a
whisper to the DM from the party member the panel acts as (VTT-595), and tells
the player of a `/w` line or an empty `/dm` line through `renderPlayerPanel`'s
optional `notify`, to which `paint` passes the same `notify` it hands the
console, which sets the toast and repaints, sending nothing and leaving
`say-text` as typed (VTT-596). A whisper takes the Say's lock, and is kept and
cleared as a line is.

**Adding an actor from JSON.** Add from JSON hands the paste to
`parseActorJSON` in `client/src/commands.ts`, which returns an `addActor`
command or an error that the console tells through `DMDeps.notify`,
sending nothing. It refuses, in this order, a paste that is not JSON, an
actor `fromJson` refuses, and an actor with no `actorId`, one that names a
controller and one with no `kind`, which the server refuses too (SPEC-013);
then one holding a resource whose `current` is written `null`
or left out, naming the actor and the key (VTT-672, SPEC-025).
`valuelessCurrent` reads that from the paste as written, since `fromJson`
reads such a `current` as `0`. A `current` of `0` passes, and a `max` left
out or written `null` is no maximum (VTT-679).

**What a command's answer leaves typed** (the owner's rulings "Ja, Say och
alla formulär" and "Töm ändå", 2026-10-29). The console's controls that
clear a field, Start session, Add actor, Remove actor, Add from JSON, Place
token, Remove token, a row's Describe, Save note, a row's Grant, a row's
Give, and the Say's Send and Enter in its `say-text`, each send through
`sending` in `client/src/view/dm.ts`, and so does an ability's take chip,
which clears nothing. A player's panel's Send, and Enter in its
`say-text`, send through `submit` in `renderPlayerPanel`. Both are handed
`act` from `client/src/app.ts` as `send`: it resolves to the command's
`CommandResult`, shows a refusal as the toast of its error, with nothing
before it (VTT-604), and paints at every answer before the console's or the
panel's own handling of that answer runs. Until the answer, what was typed
for the command stays in its field and in `draft` or `PlayerUIState`. When
the server accepts it (`ok`):
- the console clears the drafts of the fields that fed the command, Save
  note's key, title and text and the Say's `say-as` excepted, and, for a
  row's Give, both its value fields whether or not they fed it, and asks for a
  repaint through `DMDeps.rerender`, which `app.ts` sets to `paint`. It
  empties no box on the page itself: the repaint draws each of those fields
  anew, as with nothing typed: empty, the blank option, or for a Describe box
  the token's present text (VTT-533). Remove actor is fed by `actor-id` alone
  and Remove token by `token-id` alone, so the rest of their forms stays as
  typed;
- the panel empties `PlayerUIState.sayText` and the `say-text` box it read
  the line from, and asks for a repaint through its `rerender`, which
  `app.ts` sets to `paint` (VTT-520).

Text typed into a field that is cleared, after the command was sent, is
cleared with it. Any other answer clears nothing and asks for no repaint of
its own: a refusal, and the `ok: false` that `Wire.send` gives a command on a
socket that is not open or that closes before the result arrives, alike
(VTT-517, VTT-518, VTT-538). A refusal is not logged:
`git grep -n 'console\.' client/src` finds only the handlers of a rejected
send, in `sending` and in `submit`.

**One press, one command** (VTT-523, VTT-535). While a command one of those
console controls sent is unanswered, pressing that control again sends
nothing; while a line the console's Say or a player's panel sent is
unanswered, its Send and Enter in its `say-text` send nothing. The console
keeps its locks in `unanswered`, beside `draft`, so they last across paints,
one per control: keyed by the control's action, by the action and the row's
token or actor id for a Describe or a Grant, by the action, the row's actor
id and the ability's id for a Give or a take, and by `actor-json` for Add
from JSON; the Say's Send and Enter share one, keyed `add-narration`. So a
command one of them sent holds back no other, another row's Describe or
Grant and Remove actor beside Add actor included, though those share
`actor-id`, and a give unanswered holds back only that actor's give of that
ability; the Say's Send and Enter count as one (VTT-541). The
panel's lock is `PlayerUIState.saying`, one for Send and Enter. A press the
console's own checks refuse, or a line in which `readLine` finds nothing to
send, in the console's Say or a player's panel, sends nothing and takes no
lock. A lock is taken when the command is sent and freed at its answer,
whatever it is, a socket's close included, or when the send's promise
rejects.

**A press of one of those controls that sends draws nothing new before its
answer** (VTT-524, VTT-536; the owner's ruling "Nej, osynligt lås" of
2026-10-29). A press of one of them that sends neither clears, nor disables,
nor repaints anything, and a press a lock stops shows no notice and asks for
no repaint. The toast, or the cleared field, follows the answer.

**A send of one of those controls whose promise rejects** clears nothing,
asks for no repaint, frees the control that sent it, and passes its error to
`console.error` (VTT-526, VTT-537; the owner's ruling "console.error(e)" of
2026-10-29).

**The panel is the character's sheet.** The panel is headed by the name of the
character it shows, as its chip names it, else by the fold's name, else by its
id, and draws no heading while it shows none (VTT-836). Below the chips it
draws the sheet `sheetFor` (`client/src/player.ts`) computes for that
character from the window's abilities and properties and the sheet and
conditions the window's newest `/api/ruleset` answer gave (`layout` in
`startSession`; an answer without either reads as none), and an answer to an
older read arriving after it changes neither (VTT-845). It keeps that
answer's `freeUsageName` in `layout` beside them in the same way, an answer
without it read as none (VTT-855).
- **Pages.** With a sheet of more than one page it draws one `.tab` per
  page in `.tabs`, named and ordered as declared, and shows the first page
  until a tab is clicked; it keeps the page shown in `PlayerUIState.page`,
  by its name, across a repaint, a switch of character and a new answer,
  and shows the first page when the sheet no longer declares that name; a
  sheet of one page draws no tab (VTT-837, VTT-838).
- **Sections.** In `.sheet` it draws the page's sections in their order,
  each an `h3` of its name, and leaves out a section with nothing to show
  for the character (VTT-839). A values section draws its values in the
  order it names them, each a `.value` reading the name, a space and the
  value, a number as itself and a resource as `current/max`, named by the
  display name of the property declaring its key as that type and titled
  by its description, else by its key with no title; a key held both as
  an attribute and as a resource draws both, the attribute first, and a
  key the sheet does not hold draws nothing (VTT-841). A conditions section
  draws the character's conditions in the order the fold applied them,
  each a `.condition` named and titled by the ruleset, else named by its id
  with no title (VTT-842). An abilities section draws a `.row` of the
  buttons the panel offers whose usage one of its usages names, in the
  window's list order, each labelled by the ability's name, else its id
  (VTT-843, VTT-846).
- **No sheet declared.** With no sheet, or an empty one, it draws no tab
  and one page of the sections "Values", "Conditions" and "Abilities", in
  that order, each left out when empty, its values those of the window's
  properties in their order, then every key no property declares as that
  type, in key order, an attribute before a resource of one key (VTT-840).
- **Outside the page.** The target buttons, or "Click the board to move.",
  the arm-doors toggle and the Say form are drawn below the page on every
  page, so an ability armed on one page keeps its targets on another.

### How the window names what it shows

`namer` in `client/src/view/naming.ts` names each token and actor the window
shows, except in the panel's chips, its heading and the perch's buttons, and
a resource
as below. `renderSpectator` builds one per paint, from the fold, the log,
`ViewExtras.dm`, `ViewExtras.abilities` and `ViewExtras.properties`, for a
disc's initial, its native `title` and its resource chips, its card, and the
Story and the Events panel; `paint` in `startSession` sets `dm` to `isDM`,
`properties` to the list the newest `/api/ruleset` read answered, and
`abilities` to every ability `everRead` holds, those every answer since the
page loaded gave (above), for every role. `renderPlayerPanel` builds its own
for its target buttons and its abilities' costs, from the fold, the
abilities and the properties with no log, by the role `/api/me` last gave
(VTT-475, VTT-484, by the owner's rulings A-6 and K-1 of 2026-10-22).
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
  `a creature`. An ability is named by the display name the abilities the
  namer is handed give it, else by its id: for the Story and the Events panel,
  every ability an answer since the page loaded gave, so a window names by its
  id an ability no answer since it loaded held, and by its name one that a
  later answer does not hold (VTT-734; the owner's ruling OD-5 of
  2026-11-14).
- **A resource** is named by `Namer.resource`: by the display name of the
  property `/api/ruleset` gives under its key as a `resource`, else by its
  key, and by its key while the window has no properties; a `number`
  property names no resource (VTT-815, VTT-816). The console's value fields
  of a give are named so, by a namer `renderDMConsole` builds from the
  fold, the abilities and the properties `DMDeps` hands it, with no log
  (VTT-806). A disc's resource chip, which `tokensOnScene` gives a disc
  only in a DM's and an agent's window (VTT-834), shows `current/max` and
  is titled so (`ResourceChip.label`, which `tokensOnScene` sets), and a
  token's chips stay sorted by key. Every window draws a disc's condition
  dots, in the order the fold applied them (VTT-835). The Events
  panel's line of a `resourceChanged` reads the actor, the
  resource so named, ` → ` and the new value. On the panel, an ability's
  button is titled by its non-empty description and its non-empty cost
  line, joined by a line break, the description first, and is untitled
  with neither (`buttonTitle`, VTT-854); a description's own line breaks
  stand as written. A resource ability's cost line is the resource so
  named, a space and its `current/max` on the acting actor's sheet when
  the sheet holds it, `, costs ` and the cost, and ` — not enough` when the
  actor cannot afford it (VTT-844); the cost line of an ability that
  spends no resource is the `freeUsageName` of `layout`, and none when it
  is absent or empty, so the panel writes no word of its own for it
  (VTT-855).

A player's chips and the perch's buttons name a party member as the
`WatchableChanged` set names it, and a DM's or an agent's chips an actor as
the fold holds it, each by its id when that name is empty (VTT-404); the
panel's heading names its character as its chip does, else by the fold's
name, else by its id (VTT-836). The naming above does not reach them.

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

**The lines.** `describe` in `client/src/view/spectator.ts` words every case
of `Envelope.payload`, and an envelope with none as "something happened"; it
has no `default`, so a payload added to the contract fails
`task client:typecheck` until it is worded (VTT-476). The Events panel's rows
are `describe`'s words. The Story draws each entry it keeps with
`tell`, which words a door "a door opened" or "a door closed", naming no
square (VTT-819), and every other entry as `describe` does. An actor's
arrival reads "<actor> is in the party" or "<actor> comes into sight", and a
door on the Events panel "a door opened at x,y" or "a door closed at x,y". A
token's placement reads "<token> is at x,y", and a move "<token> moved to
x,y", with " — " and its reason when it has one (VTT-821), on the Events
panel alone. An ability's line is "<actor> used
<ability>", then, for each of its `outcomes` in order, " on <target>:
<result>, rolled <total>", joined by ";", and with no outcomes " on " and its
targets joined by ", ". It never reads `outcome_summary` or `rolls`, so it
shows what was rolled and never the total that roll was compared with (VTT-477,
by the owner's ruling of 2026-10-24 on the ability's line). On the Events
panel alone, a give reads "<actor> can now use <ability>" and a take
"<actor> can no longer use <ability>", the actor and the ability as the
window names them (VTT-813), and a resource added reads "<actor> now has
<resource> at <current>" and a resource removed "<actor> no longer has
<resource>", the resource as `Namer.resource` names it (VTT-814): the Story
writes nothing from a sheet, by the owner's rulings of 2026-11-16. A whisper
reads
"The DM whispers to ", its `to_actor_ids` named as the window names actors
and joined by ", ", then ": " and its text, or its `from_actor_id` so named,
" whispers to the DM: " and its text, and the Story draws it with the class
`whisper` (VTT-597). A condition, a note's key and an adventure's id are
shown as the entry names them, and a resource as `Namer.resource` names it.

**The console's rewrite.** `renderDMConsole` in `client/src/view/dm.ts` draws,
when the world holds a token, a "Describe tokens" group with one row per token
of the world, by id, each with the token's id and boxes for its name, notes,
GM name and GM notes, the two notes as textareas. A box with no draft shows
the token's present text. "Describe" sends `describeToken` with the four as
typed, empty ones included (VTT-464; "Ja, en kontroll i konsolen", the owner's
ruling of 2026-10-24). Once the server accepts it, the console clears that
row's drafts and no other row's, so the row's boxes are drawn as with no
draft (VTT-533).

**The console's abilities.** `renderDMConsole` draws, after "Remove condition"
and before "Who controls what", an "Abilities" group whenever the world holds
an actor: one row per actor of the world, by id compared with `<`
(`data-actor`), headed by its name, else its id. A row holds one chip per
distinct id the actor's sheet names, in the sheet's order, labelled the
display name `DMDeps.abilities` gives it, else its id, and " ✕" (`data-action`
`take-ability`, `data-ability` the id), which sends `takeAbility`. When
`DMDeps.abilities` holds abilities the sheet does not name, the row then holds
a select of those, in the list's order, each labelled its display name, else
its id, after a first option of no value labelled "choose an ability", the
choice kept in `draft` under `give-ability-` and the actor's id; a current
field and a max field, kept in `draft` under `give-ability-current-` and
`give-ability-max-` and the actor's id, as their `data-field`; and a Give
button (`data-action` `give-ability`). The two fields are shown exactly
while the chosen ability's `usage` spends a resource the actor's sheet does
not hold, and hidden otherwise. They follow each change of the choice
without a repaint, and each takes as its placeholder the resource named as
`Namer.resource` names it, then " now" or " max" (VTT-788, VTT-806). Give
with none chosen tells "Choose an ability to give first." through
`DMDeps.notify` and sends nothing (VTT-730). With the fields shown, Give
reads each trimmed, and an integer is an optional sign and digits that a
32-bit signed field can hold. A current that is not one, an empty one
included, tells "Give <resource> a current value that is a whole number from
-2147483648 to 2147483647.", and then a max that is neither empty nor one
"Give <resource> a max that is a whole number from -2147483648 to
2147483647, or leave it empty for none.", each sending nothing (VTT-789,
VTT-805, VTT-808); otherwise Give sends `giveAbility` of the chosen ability
with that resource under `resources` at the current and the max, an empty
max as 0, none (VTT-788). With the fields hidden it sends no resources,
whatever was typed for an earlier choice (VTT-807). Neither the select nor a
button carries a `data-field`. `app.ts` hands `DMDeps.abilities` the abilities
of the newest `/api/ruleset` answer, every ability the served ruleset holds
for the DM and the agent (SPEC-012), and `DMDeps.properties` its
properties, so with no ruleset served the console
offers no give and still offers a take of each id a sheet names. Whether the
server accepts either is SPEC-013's.

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
  on a disc, a card, a target button or a Story line; the chips, the perch
  and the panel's heading name an unnamed party member by its id (VTT-404,
  VTT-836). The frames it is sent
  still carry the ids, and `outcome_summary` (SPEC-007).
- A player's and a spectator's board shows no resource value: a player
  reads their character's values on the panel's sheet, and a seen
  companion's nowhere, by the owner's ruling of 2026-11-17.
- A kind of sheet section the route adds, once `SheetSection` in
  `client/src/metadata.ts` names it, fails `task client:typecheck` at
  `sheetFor`'s switch until the client handles it.
- A payload added to the contract needs a line in `describe`, and an answer
  in `STORY` whether the Story draws it, before the client typechecks.
- An input or a textarea added to a view needs a `data-field` unique in the
  tree it is drawn in, or a repaint takes its focus, or hands the focus to the
  first box in tree order with the same field.
- Text a box must keep across a repaint needs a home outside the tree, as
  `draft`, `JoinViewState` and `PlayerUIState` are; a box keeps none itself.
- The console's Say and a player's panel's Say share the field `say-text`
  and the action `add-narration`. No window draws both: a player's window
  draws no console, and a DM's or an agent's panel draws no Say (VTT-532).
- `draft` and `unanswered` belong to the module, not to one console: they
  last as long as the page, or the bun process that runs the client's tests.
  A test that types into the console's Say empties both its boxes when it
  ends (as `forgetTheSay` in `client/test/support/say.ts` does), and a test
  that holds a line unanswered answers it before it ends. Otherwise a later
  test in that process finds the text in its console, or its Send sending
  nothing.
- A view handed `act` that changes what it draws at an answer asks for its
  own repaint: `act`'s paint runs before the view's handling of the answer.
- A command whose answer never comes holds the control that sent it:
  nothing in `client/src` times a command out (`git grep -n
  'setTimeout\|setInterval' client/src` prints nothing).

- The Events panel is no boundary and leaves out no kind and no run: it
  shows only entries of the log the window holds, the newest of them
  (VTT-448), and a window that draws it holds the world log, which holds no
  run, so that window's Story shows each entry `told` keeps. What a
  connection is sent is SPEC-015's.
- A correction's grants and applied conditions of a run's actor, written
  directly behind the run at a later event with nothing between, are left out
  with it, as when the world adds an id again as another, out of sight, as a
  party member with no attributes, resources or module data, which
  `add_actor`'s description tells the agent never to do (VTT-390). An
  `abilityGiven`, an `abilityTaken`, a `resourceAdded` or a `resourceRemoved`
  is no part of a run, so where such a correction sends some, they end it:
  the conditions behind them are drawn, and they and the grants are not.
- A party member whose sheet holds no attributes or module data, corrected at
  its first sight in a log that held it unseen, or in its own log when that
  log opens for it already a party member (SPEC-021, SPEC-016), draws no line
  of its resources, its abilities or its control; each condition it holds is
  a bare `conditionApplied` and is drawn, unless the correction sits directly
  behind a run of the same actor (above). A run at a companion's first sight
  draws no line, and its token's arrival is a `tokenPlaced`: the board draws
  it, and the Story does not.
- A token going out of sight draws no line, nor do the `tokenHidden` entries
  by which a log that resumes hides every token it holds before the
  projection places what its character sees (SPEC-021).
- A player learns of a grant of control by the window offering the
  character: nothing in the Story says it (the owner's ruling of
  2026-11-16). A taking back is told by the toast "You no longer play …"
  (`onPartyMembersLeft` in `client/src/app.ts`).
- A page opened after a give or a take is told nothing of it, since its first
  set hands over nothing (VTT-733), and a reload forgets the names earlier
  answers gave.
- A creature made a party member in a character log's sight is drawn no
  "<actor> is in the party" line, though the log held it as a creature until
  then: the run that introduces it whole is left out, and so is the
  forwarded grant behind it.
- A live window can draw "<actor> left" for a moment: it folds and paints
  each entry as it arrives (`Session.ingest`, then `paint`), and a bare
  `actorRemoved` that is the log's last entry is not yet a run, so it is drawn
  until the `actorAdded` behind it arrives.
- A page whose first read of `/api/me` fails is not read again until it is
  loaded again, so a DM's or an agent's page whose first read fails shows no
  Events panel until it is loaded again. A 401 from that read, or from a
  later step of the chain, says the window's participant was removed, though
  a token stored for another campaign served on the same address is answered
  401 too (the owner's ruling of 2026-10-31); any other failure forgets
  nothing, since the read surface answers an identity store that cannot
  answer with 503 (SPEC-012).
- A client that holds a character's log names it on a redial: the gateway
  ignores the cursor of a dial that names no `log` and opens on the default,
  from its first entry (SPEC-015).
- A reload under the same token keeps the viewpoint: a page loaded afresh in
  a tab opens on the actor the last `ViewpointChanged` sent to a page in that
  tab under its token named, while the participant may still watch it
  (SPEC-015). Nothing promises it outlives the tab (VTT-444), and a
  duplicated or restored tab, which a browser may give a copy of the tab's
  storage, is neither promised it nor refused it.
- The browser client's one logout is `forget`, the only call of `Auth.clear`
  (`git grep -n 'Auth.clear\|auth.clear' client/src` finds it and the warning
  above the key in `startSession`), and it runs only when the gateway has
  declared the credential dead; nothing in the window logs out by choice.
  The key carries the token, so `forget` removes the key with the token, and
  a page under that token in that tab then names no `log` (VTT-579).
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
VTT-431, VTT-432, VTT-433, VTT-434, VTT-435, VTT-443, VTT-444,
VTT-445, VTT-446, VTT-447, VTT-448, VTT-457, VTT-458, VTT-459,
VTT-460, VTT-461, VTT-464, VTT-474, VTT-475, VTT-476, VTT-477, VTT-483,
VTT-484, VTT-489, VTT-490, VTT-491, VTT-492, VTT-494, VTT-500, VTT-501,
VTT-502, VTT-517, VTT-518, VTT-520, VTT-521, VTT-522, VTT-523, VTT-524,
VTT-526, VTT-527, VTT-528, VTT-530, VTT-531, VTT-532, VTT-533, VTT-534,
VTT-535, VTT-536, VTT-537, VTT-538, VTT-540, VTT-541, VTT-572, VTT-573,
VTT-574, VTT-575, VTT-576, VTT-577, VTT-579, VTT-580, VTT-593, VTT-594,
VTT-595, VTT-596, VTT-597, VTT-599, VTT-600, VTT-604, VTT-605, VTT-606,
VTT-607, VTT-666, VTT-668, VTT-672, VTT-679,
VTT-695, VTT-696, VTT-704, VTT-705, VTT-712, VTT-715, VTT-717, VTT-720,
VTT-721, VTT-722, VTT-727, VTT-730, VTT-732, VTT-733, VTT-734,
VTT-737, VTT-752, VTT-753, VTT-758, VTT-759, VTT-769, VTT-770,
VTT-771, VTT-772, VTT-773, VTT-774, VTT-788, VTT-789, VTT-805,
VTT-806, VTT-807, VTT-808, VTT-809, VTT-810, VTT-812, VTT-813, VTT-814,
VTT-815, VTT-816, VTT-817, VTT-818, VTT-819, VTT-820, VTT-821, VTT-822,
VTT-823, VTT-834, VTT-835, VTT-836, VTT-837, VTT-838, VTT-839, VTT-840,
VTT-841, VTT-842, VTT-843, VTT-844, VTT-845, VTT-846, VTT-854, VTT-855.
