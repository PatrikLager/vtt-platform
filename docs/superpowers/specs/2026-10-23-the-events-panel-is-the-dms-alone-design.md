# The Events panel is the DM's alone

## The problem

Every role's window shows the Events panel. `renderSpectator` in
`client/src/view/spectator.ts` builds the same fixed list of sections for every
page: the status, the board, the Story (`renderFeed`), the notes and the Events
panel (`renderTicker`, a `section.ticker` headed "Events" that lists the 40
newest events of the log, newest first, each by `describe`). Of the sections it
draws, only the perch, the player's panel and the DM's console depend on the
role, and `paint` in `startSession` (`client/src/app.ts`) decides which of
those three to pass. So a spectator's and a player's window show the newest
events of their log twice, once in the Story beside the narration that anchors
them and once again in the Events panel, and a page whose `/api/me` has not
answered, or did not answer with a role, shows the panel too. The owner ruled
on 2026-10-22 (ruling B-3 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`) that the Events
panel is the DM's alone, that players and spectators do not have it, and that
the Story stays as it is. This overturns, for the Events panel, section 4 of
`docs/superpowers/specs/2026-07-26-client-design.md`, which gives "Everyone
(spectator floor)" an event ticker; VTT-393 already overturned that section's
"with sequence numbers".

## Done looks like

1. A client test in `client/test/app.test.ts` boots a page as a player and as a
   spectator, delivers events, and observes, once the role's own panel is drawn
   (`.player`, `.perch`), that the window has no `section.ticker` and no
   heading reading "Events", while the Story (`.feed`) shows those events. It
   fails on today's tree, where both windows draw the panel.
2. A client test boots a page whose `/api/me` answers no role, delivers
   events, and observes that the Story shows them and that the window has no
   `section.ticker`. It fails on today's tree.
3. A browser e2e in `client/e2e/table.spec.ts` observes, once the spectator's
   perch and the player's panel are visible, that neither window has a
   `.ticker`, and that the DM's window has one with at least one `.tick`. The
   spectator half fails on today's tree, where "a spectator sees the table and
   no controls" asserts the panel visible.
4. Still true afterwards:
   - A DM's window and an agent's window show the Events panel, one row per
     event up to the newest 40, newest first, except while it shows the reason
     it cannot fold its log: the tests in
     `client/test/spectator-view.test.ts` that read `.ticker` and `.tick` stay
     green for a DM's window, and a client test boots a page as a DM and as an
     agent and observes a `.tick` per event delivered.
   - The panel labels a move with its reason when its frame carries one
     (VTT-263), and no role's window shows a log sequence number (VTT-393).
     Their tests stay green, each reading only panels that role's window has.
   - The Story is unchanged: `git diff main -- client/src/view/feed.ts` is
     empty, and `renderSpectator` draws `renderFeed` for every role.
   - "a spectator chooses a shoulder and is shown that log" reads the Story
     (`.feed .mechanical`) where it read `.tick`, on the shoulder and after the
     "No shoulder" button, and stays green.

## Rules this puts on the system

- Only a window the client knows to be a DM's or an agent's shows the Events
  panel.

## What it touches

In this order:

1. `client/src/view/spectator.ts` (`renderSpectator`, `ViewExtras`,
   `renderTicker`) and `client/src/app.ts` (`paint`). Their tests:
   `client/test/spectator-view.test.ts`, `client/test/qa-move-reason.test.ts`,
   `client/test/app.test.ts`, and `client/test/qa-window-number.test.ts`, whose
   world-log render is a DM's window.
2. The committed bundle `cmd/vtt/webdist/`, from `task build:client`.
3. `client/e2e/table.spec.ts`: "a spectator sees the table and no controls",
   which asserts the panel visible; "a spectator chooses a shoulder and is
   shown that log", which reads the spectator's `.tick` as its evidence of the
   log; and "the story feed and ticker record what happened", VTT-393's
   browser evidence, which reads the player's `.tick`.
4. `docs/requirements.md`.

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- Whether the owner's "the DM" includes the agent. The ticket reads it as both
  roles that `paint` gives the DM's console (`isDM`: `dm` and `agent`), the
  roles SPEC-022 gives the world, since a table's DM is either a person or the
  agent. The plan names that reading for the owner's sign-off.
