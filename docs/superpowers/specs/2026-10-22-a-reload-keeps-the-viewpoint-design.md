# A reload keeps whose eyes the window looks through

## The problem

A browser page that is loaded afresh forgets which party member it watched.
`Wire` in `client/src/wire.ts` holds the log it watches in its `log` field,
which `handleFrame`'s `viewpointChanged` arm sets and nothing writes anywhere
else. `boot` in `client/src/app.ts` keeps one thing in `localStorage`, the
token (`Auth`, `client/src/auth.ts`, key `vtt.token`). So a new page builds a
`Wire` whose `log` is null and whose cursor is 0, and its first dial names no
`log`. The gateway then opens the seat on the participant's default
(`openSeat` in `internal/gateway/seat.go`, `defaultViewpoint` in
`internal/gateway/viewpoint.go`; SPEC-015): nobody for a spectator, and for a
player the first party member they control by id. It was found at the table
on 2026-10-22 (finding A-9 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`): a spectator
perched on Tobbe reloaded and came back on nobody, and a player holding Lera
and Tobbe, watching Tobbe, reloaded and came back on Lera. The gateway already
opens a seat on a named log from its first entry when the dial names, with
`after` 0, an actor the participant may watch (`openSeat`; VTT-339, VTT-349),
so what is missing is on the client. No rule says a page loaded afresh keeps
the viewpoint: SPEC-022's redial (VTT-397) is the same page dialling again,
with the log it already holds.

## Done looks like

1. A browser e2e in `client/e2e/table.spec.ts` has a spectator perch on a
   party member and reload the page, and observes, with no further click, that
   the perch reads "Perched on: " and that party member's name, and that the
   window shows that party member's log: its token on the board (`tok-fighter`
   for the adventure's Human Fighter) and its entries in the Story (`.feed`),
   which on nobody reads "Nothing has happened yet.". It reads neither the
   ticker nor `.tick`, which the owner's ruling B-3 gives to the DM alone. It
   fails on today's tree, where the perch reads "Perched on: nobody".
2. A browser e2e in `client/e2e/handover.spec.ts` has a player who controls
   two party members switch to the one that is not first by id and reload the
   page, and observes that the chip marked as watched is still that one. It
   fails on today's tree, which comes back on the first by id.
3. A client test in `client/test/` shows that a page loaded afresh in a tab
   whose last page, under the same token, watched an actor dials naming that
   actor in `log` with `after` 0. It fails on today's tree, whose first dial
   names no `log`.
4. Still true afterwards:
   - A DM's or an agent's page dials naming no `log`, a page loaded afresh
     included. VTT-397's tests stay green, and a client test loads a DM's page
     in a tab whose earlier page, under a player's token, watched a party
     member, and observes a dial with no `log`.
   - A page whose remembered party member the participant may no longer watch,
     revoked or removed while the page was closed, shows the one the gateway's
     opening `ViewpointChanged` names, the default, and not the remembered one
     (VTT-340, VTT-369, VTT-407). Until that `ViewpointChanged` arrives, a dial
     that fails included, the window shows nobody as watched, the remembered one
     included (VTT-407). A test observes both.
   - A page loaded afresh in a tab under another participant's token, when no
     page of that participant's in the tab was sent a `ViewpointChanged`, does
     not dial with the viewpoint the earlier participant's page in that tab
     remembered. The token may come from a `?token=` link opened in that tab,
     or from another tab of the profile that replaced `vtt.token`. A test
     observes both.
   - A page in another tab of the same browser profile, under the same token,
     none of whose tab's earlier pages was sent a `ViewpointChanged`, does not
     open on what the first tab remembered. A browser e2e opens a
     second page in the first page's browser context after the first page
     perched, and observes that its dial names no `log`. A client test observes
     the same for a page booted with the token in `localStorage` and an empty
     `sessionStorage`.

## Rules this puts on the system

- A page loaded afresh in a tab opens on the party member its participant's
  last page in that tab watched, while the participant may still watch it.
- A remembered viewpoint is used only in the tab whose page remembered it, and
  only by the participant whose page it was. Nothing promises it outlives the
  tab (the owner's ruling of 2026-10-22, "per filk", meaning per flik).

## What it touches

In this order:

1. `client/src/app.ts`, which boots the page and keeps what a tab remembers;
   `client/src/wire.ts`, whose dials name the log; and `client/src/session.ts`,
   the only source file that builds a `Wire`. Their tests,
   `client/test/app.test.ts` and `client/test/wire.test.ts`, and the
   `beforeEach` of every test file that calls `boot()` (`app.test.ts`,
   `qa-one-log.test.ts`, `qa-switch.test.ts` and `qa-window-number.test.ts`),
   which clears `sessionStorage` beside `localStorage`.
2. The committed bundle `cmd/vtt/webdist/`, from `task build:client`.
3. `client/e2e/table.spec.ts` for the spectator and
   `client/e2e/handover.spec.ts` for the player.
4. `docs/requirements.md`.

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- How the alpha's two windows held two participants in one browser. Two tabs
  of one browser profile share `localStorage`, so they share one `vtt.token`:
  the second link opened replaces the first's, and a reload of the first tab
  comes back as the second participant. Whether the alpha used two profiles,
  two browsers or a private window was not recorded.
