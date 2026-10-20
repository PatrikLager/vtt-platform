# A player or a spectator chooses whose log the browser shows, and the browser follows it

## The problem

The gateway sends a player's or a spectator's connection one character log,
numbered by that log, and announces each switch with a `ViewpointChanged`
naming the new log, which follows from its first entry (SPEC-007, SPEC-015).
The browser client was written for the projection that preceded it, and
three things a person at the table does no longer work.

**A switch breaks the window.** `Wire.handleFrame` in `client/src/wire.ts`
has arms for a result, an event and the two presence frames, and drops every
other kind, `viewpointChanged` among them. `Session` in
`client/src/session.ts` keeps the log it holds and appends the new one to it,
and `fold` refuses a log that opens a session, or introduces a scene or an
actor, a second time. `client/src/app.ts` then replaces the window with
"Cannot derive state from the log: " and the reason (QA's finding 2 in
`docs/reports/2026-10-19-the-window-shows-no-log-number.md`). The gateway
moves a seat without being asked when a player's watched character is
revoked, made a non-party actor or removed, when a player watching nobody is
granted a party member, and when a promotion changes what the seat may watch.

**A redial opens the wrong log.** `Wire.connect` dials
`?token=…&after=…` and never sends `log`, so the gateway ignores the cursor
and opens a player's or a spectator's redial on the default log from its
first entry (SPEC-015), on top of the log the client still holds.
`Wire.reconnect` resumes one sequence below the highest it has seen and has
`Session.rollback` drop that sequence first, because a projected seat was
once sent several envelopes under one sequence; a character log numbers each
entry from 1 without gaps (SPEC-021), and so does the world.

**Nobody can choose whom to watch.**
- A player cannot switch character. `renderPlayerPanel` in
  `client/src/view/player.ts` offers one button per actor
  `controlledActors` finds in the fold, and a click only sets
  `ui.selectedActorId`, the actor `moveCommandFor` moves. A click on the
  board sends a door command or a `move_token`. Nothing sends a player's
  `set_viewpoint`: `perchOn` in `app.ts` is offered only where `mayPerch`
  holds, and `mayPerch` is the spectator role.
- A spectator cannot choose a shoulder. It opens on nobody (VTT-369) and is
  sent no entry while it watches nobody (VTT-341), so `renderPerch` in
  `client/src/view/spectator.ts` lists the party members of an empty fold and
  says "No party members yet." No frame names whom a connection may watch:
  `ServerFrame` in `contract/vtt/v1/commands.proto` carries a result, an
  event, the catch-up head, the two presence frames and `ViewpointChanged`,
  and `GET /api/me` answers a participant's id, name and role.
- A player's buttons are what the watched log's fold says they control. A
  control change on a party member the watched character does not see is not
  written to that log (SPEC-016), so the buttons can miss a character the
  gateway would let them watch and offer one it would refuse.

The owner ruled on 2026-10-15 that a player is active on one character at a
time, switched by clicking its token, the board emptying and the other log
replaying; on 2026-10-20 that this ticket is the client's half of the fourth
per-character-logs ticket, the Go readers being the next ("Två, klienten
först"); and that the gateway sends a player or a spectator the party
members they may watch, at connect and whenever that changes, never in any
log, a spectator still opening on nobody ("Servern skickar listan").

Comments under `client/src`, `client/test` and `contract/` still explain the
projection's machinery by deleted names: `internal/gateway/project.go`,
`project_internal_test.go`, `MayPerch`, `perchSequence`, `reperch`,
`look()`, "the projector" and `client/test/projection-parity.test.ts`.

Rule 9, how MapTool does this: it has no spectator role. A player's view is
the vision of the selected tokens they own that have sight, or, with none
selected, of every token they own that has sight when the server uses
individual views and of every player token with sight otherwise
(`ZoneViewModel.makePlayerView`), so selecting a token is what moves the
view, the shape the 2026-10-15 ruling borrows. MapTool's client holds every
token and its owner list, so it never needs to be told whom it may see
through; the list this ticket adds has no MapTool equivalent, because that
distribution model is the one this platform refuses.

## Done looks like

1. A new test in `client/test/app.test.ts` drives a player's connection
   through a `ViewpointChanged` naming one party member and that log, then a
   `ViewpointChanged` naming another and the second log from its first
   entry, which opens a session and introduces a scene the first log also
   held. The window shows the second log's board and no "Cannot derive state
   from the log". It fails today, at that reason. The same holds for a
   spectator.
2. A new test in `client/test/wire.test.ts` closes a player's connection
   that was told one log and folded it to sequence N, and the redial's URL
   names that log in `log` and N in `after`. It fails today: the URL carries
   no `log`, and `after` is N-1. A DM's redial names its last world sequence
   in `after`.
3. A redial the gateway opens on another log than the one the client holds,
   or on the same log from its first entry, ends with the window showing the
   log the gateway sends, folded from its first entry, and no fold failure:
   a new test in `client/test/app.test.ts`.
4. A new test in `client/test/app.test.ts` shows a player watching one
   party member and offered another switching to it, both by its button and
   by a click on its token on the board: each sends `set_viewpoint` naming
   it. It fails today: the button sends nothing and the board click sends
   `move_token`.
5. A new test under `internal/gateway` shows a player's connection sent the
   party members it may watch, at its opening and again after a grant, a
   revocation, a kind change or a removal changes them, a grant of a party
   member the watched character does not see included; a spectator's sent
   every party member; and a DM's and an agent's sent none. It fails today,
   since no frame names them.
6. A player is offered a switch to exactly the party members the gateway's
   list names, and a spectator watching nobody is offered every one, each
   labelled by its name: new tests in `client/test/app.test.ts` and
   `client/test/spectator-view.test.ts`. They fail today: the spectator is
   offered none.
7. `client/e2e/table.spec.ts` opens as the spectator, which is offered the
   adventure's party member by name, chooses it, and is then shown ticker
   rows from that log; `client/e2e/handover.spec.ts` shows the player who
   holds Ash and Bram switching to Bram and back. Both fail today. Every
   other test `task e2e` runs passes, `handover.spec.ts`'s skipped one aside.
8. `grep -rnE 'project\.go|project_internal_test|MayPerch|perchSequence|reperch|projector|look\(\)|projection-parity' client/src client/test contract/vtt contract/events.test.ts`
   prints nothing. Today it prints lines from each of the four.
9. `task check` is green on the last code commit, and `task check:drift`
   after the commit that rebuilds `cmd/vtt/webdist`.
10. SPEC-007 says what the new frame carries and what a client does at a
    switch and at a redial, SPEC-011 names the new frame among those the
    pump queues, and SPEC-015 says when a seat is sent it.

## Rules this puts on the system

- A client holds one log at a time: at a `ViewpointChanged` naming another
  log than the one it holds, and when the log it is sent starts again from
  its first entry, it discards what it folded and folds what follows.
- A redial names the log the client holds and the last sequence it folded
  from it.
- The gateway sends a player's or a spectator's connection the party members
  `MayWatch` accepts for it, at its opening and whenever that set changes,
  and sends a DM's or an agent's connection none.
- A player is offered a switch to each party member that list names, by its
  button and by a click on its token, and a spectator to each one and to
  nobody.

That a switch or the list appends nothing is not a new rule: a frame is not
an `Envelope`, and `set_viewpoint` appends nothing today (SPEC-007).

## What it touches

In this order:

1. `contract/vtt/v1/commands.proto` (a new message and a `ServerFrame` arm),
   regenerated into `contract/gen/go` and `contract/gen/ts`;
   `contract/roundtrip_test.go`, `contract/events.test.ts` and
   `contract/testdata/` for its fixture; `contract/vtt/v1/events.proto`'s
   comment.
2. `internal/gateway/server.go` (the pump), `internal/gateway/viewpoint.go`
   or `internal/gateway/seat.go` (the set), and a new test file in
   `internal/gateway`.
3. `client/src/wire.ts`, `client/src/session.ts`, `client/src/app.ts`,
   `client/src/view/spectator.ts`, `client/src/view/player.ts`, and the
   comments in `client/src/fold.ts`, `client/src/commands.ts` and
   `client/src/view/dm.ts`.
4. `client/test/wire.test.ts`, `client/test/session.test.ts`,
   `client/test/app.test.ts`, `client/test/spectator-view.test.ts`,
   `client/test/player.test.ts`, and the comments in
   `client/test/dm-view.test.ts`, `client/test/commands.test.ts`,
   `client/test/removal-batch-parity.test.ts` and
   `client/test/fold-unit.test.ts`.
5. `client/e2e/table.spec.ts`, `client/e2e/handover.spec.ts`.
6. `cmd/vtt/webdist`, rebuilt by `task build:client`.
7. `docs/requirements.md`, for the rules' rows.
8. The three specifications below.

Three components, the contract, the gateway and the client, and more than
ten files: the owner chose the frame knowing it costs the contract and the
gateway's pump as well as the client. `internal/harness/client.go`'s
`readLoop` skips a frame kind it has no arm for, and `internal/mcp` reads
through it, so the Go readers need no change for the new frame; what they
do at a switch is the next ticket's.

## Specifications this moves

docs/specifications/007-the-wire-contract.md
docs/specifications/011-the-connection.md
docs/specifications/015-the-seat-and-the-perch.md

## What could not be established

- Whether the list's name for a party member can differ from the name the
  watched log's roster holds. A party member is on every log's roster, and
  one added again as another is introduced at once (SPEC-016); no other
  path that renames an actor was searched for.
- Where the list is sent relative to the opening `ViewpointChanged`, and
  whether a client may act on it before the log's first entry. SPEC-007
  orders `CatchUpHead`, `PresenceSnapshot` and `ViewpointChanged` and says
  nothing of a frame that does not exist yet.
- What a player who holds only non-party actors is offered. They watch
  nobody and move nothing (VTT-370), and `renderPlayerPanel` offers those
  actors' buttons for moving today.
- How long `task check` takes: any change to the client's sources or tests
  makes `check:ts-mutation` re-mutate the whole client, and the change to
  `internal/gateway` re-mutates that package.
