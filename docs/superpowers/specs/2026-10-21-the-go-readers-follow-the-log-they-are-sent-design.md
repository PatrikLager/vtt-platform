# The Go readers follow the log a player's or a spectator's connection is sent

## The problem

The gateway sends a player's or a spectator's connection one character log,
numbered by that log from 1, and announces each switch with a
`ViewpointChanged` naming the new log, which then follows from its first
entry; a redial that names no `log` is opened on the default log from its
first entry, whatever its `after` (SPEC-007, SPEC-015). The browser client
follows this since 2026-10-20 (VTT-394 to VTT-398). The Go readers do not.

**The harness client cannot see a switch, and cannot name a log.**
`Client.readLoop` in `internal/harness/client.go` has arms for a result, an
event and `CatchUpHead`, and skips every other kind, `ViewpointChanged` and
`WatchableChanged` among them. `Dial` puts `token` and `after` in the
handshake and has no way to put `log`; neither has `Dialer` in
`internal/harness/engine.go`, the shape `cmd/vtt/client_run.go`'s
`dialerFor` and `cmd/vtt/client_soak.go`'s `buildSoakDialer` build.

**So the scenario engine and the soak see a switch only as a restart at 1.**
`logBreak` and `currentRun` find a log's run by its entry of sequence 1, and
`runReconnectStep`'s own doc says to expect a false failure once a scenario
switches a log seat and then reconnects it, since the redial opens on the
default. The open debt entry "The harness cannot see a log seat switched on
its old subscription" in `docs/verification-debt.md` records that a gateway
which announces a switch while the old log keeps arriving passes every
`cmd/vtt` scenario test. `runSoakCheckpoint` in `internal/harness/soak.go`
compares a fresh catch-up's fold with the live one for the observer, a world
seat, only; no log seat's fold is compared with a fresh dial of its log.

**`vtt mcp` seated as a player or a spectator loses the log it switches to.**
`set_viewpoint` is one of its tools (`tools/toolgen/main.go`). `pump` in
`internal/mcp/server.go` hands each envelope to `recordEvent`, which keeps it
only when its sequence is above `lastSeq`. After a switch, the new log's
entries up to the old log's highest sequence are dropped and the rest are
appended to the old log's history, which `handleGetState` in
`internal/mcp/read_tools.go` folds and `handleGetEventsSince` pages by
sequence. `redial` dials `after=lastSeq` with no `log`, so the gateway opens
the default log from its first entry. Neither answer says which log its
sequences number, so a caller's `afterSequence` from one log is read against
another.

**`vtt state dump` folds whatever follows a switch.** `cmd/vtt/state_dump.go`
waits, in `drainToHead`, for an envelope at the announced `CatchUpHead`. A
seat switched before that point is sent the new log from 1: when the new log
is shorter, the dump waits out `dumpCatchUpTimeout` and refuses with a
cause that does not name the switch; when it is as long, `harness.Fold` is
handed the start of one log followed by another.

The owner ruled on 2026-10-20 that this is the second half of the fourth
per-character-logs ticket ("Två, klienten först"), and on 2026-10-21 that the
three browser-client comments pointing at SPEC-015 for the perch are
re-pointed in this ticket, and, the same day, that the client's rules get a
record of their own, SPEC-022, which these readers' rules join.

Rule 9, how MapTool does this: a client that joins is sent the whole campaign
in one `SetCampaignMsg` (`MapToolServer.addRemoteConnection`), and
`ClientMessageHandler.handle(SetCampaignMsg)` replaces whatever campaign the
client held with it. A reconnect is a fresh join; there is no cursor and no
log to name. The replace-on-arrival shape is the one the browser client took
at a `ViewpointChanged`; the cursor and the `log` a redial names have no
MapTool equivalent, because MapTool sends every client everything.

## Done looks like

1. A new test in `internal/harness` sends a `harness.Client` a
   `ViewpointChanged` naming one party member and its log, then one naming
   another and that log from its first entry, and the client's reader is
   told of each switch, in order with the envelopes. It fails today: both
   frames are skipped.
2. A new test in `internal/harness` dials a `harness.Client` naming a log and
   a cursor, and the handshake the server sees carries both `log` and
   `after`. A dial naming no log carries no `log`. It fails today, since
   `Dial` takes no log.
3. A scenario in `scenarios/` in which a player switches from one party
   member's log to another's and is then reconnected passes against today's
   gateway. Run with today's harness it fails, at the reconnect step. Under
   the open debt entry's recipe in `seat.watch` it fails, and the entry
   moves out of "Open debt" with what closes it.
4. At each soak checkpoint, each log seat whose stream has not broken its
   log has its fold of the log it holds compared with the fold of a fresh
   dial naming that log, and a seat whose two folds differ fails the
   checkpoint. A test in `internal/harness` shows a checkpoint failing when a
   log seat's live history and its fresh dial differ. Today no log seat's fold
   is compared.
5. A new test in `internal/mcp` seats the server as a player that is sent a
   `ViewpointChanged` naming another party member and that log from its first
   entry: `get_state` answers that log's fold alone, `get_events_since` from 0
   answers that log's entries alone, and both answers name the log. It fails
   today at the first: entries at or below the first log's highest sequence
   are dropped.
6. A new test in `internal/mcp` drops a player's connection after a switch,
   and the redial names the log it holds in `log` and that log's highest
   sequence in `after`. It fails today: the redial names no `log`.
7. A new test in `cmd/vtt` switches a player's seat between `CatchUpHead` and
   the dump reaching it. The dump prints no fold of two logs, and does not
   wait out `dumpCatchUpTimeout` for a head the new log does not reach. It
   fails today on one of the two, by the new log's length. A world dump
   that sees no switch prints the same JSON as today; a player's or a
   spectator's prints that JSON plus the log it is the fold of.
8. `grep -n 'SPEC-015' client/src/view/spectator.ts client/src/app.ts`
   prints nothing. Today it prints three lines, at `PerchControl`'s doc, at
   `ViewExtras.perch` and at `mayPerch`.
9. `task check` is green on the last code commit.
10. The specification that says what a client holds says what these readers
    hold and what a redial of theirs names, and SPEC-007's Status no longer
    records whether the client's rules get a record of their own as open.

## Rules this puts on the system

- A Go reader of a player's or a spectator's connection holds one log: at a
  `ViewpointChanged` naming another log than the one it holds, it discards
  what it holds and keeps what follows.
- A Go reader's redial of a player's or a spectator's connection names the
  log it holds.
- `vtt mcp`'s redial resumes after the highest sequence it was sent of the
  log it holds.
- `get_state`, `get_events_since` and `vtt state dump` name the log their
  answer is of.
- A dump never prints the fold of more than one log.
- A soak checkpoint fails when a log seat whose stream has not broken its
  log folds it otherwise than a fresh dial of the same log.

VTT-394 to VTT-398 say "the client" and are held by the browser's tests.
Whether the first two rules are those rows with more evidence or rows of
their own is the sort's question. VTT-360, that a log seat's reconnect fails
when its replay differs from its live history since its last entry of
sequence 1, is false once the seat is told of its switches and its redial
names the log, so it is withdrawn and succeeded.

## What it touches

In this order:

1. SPEC-022, new, holding what moves out of SPEC-007; SPEC-015's pointers;
   and the comments of `client/src/view/spectator.ts`, `client/src/app.ts` and
   `client/src/wire.ts` that point at what moves. The bundle in
   `cmd/vtt/webdist` carries no comment, so it is not rebuilt unless
   `task build:client` changes it.
2. `internal/harness/client.go`, `internal/harness/engine.go` (`Conn`,
   `Dialer`, `runReconnectStep`, `currentRun`), `internal/harness/soak.go`,
   their tests, and `scenarios/` for the new scenario.
3. `cmd/vtt/client_run.go` and `cmd/vtt/client_soak.go`, which build a
   `Dialer`, and the `cmd/vtt` scenario tests.
4. `internal/mcp/server.go`, `internal/mcp/read_tools.go`, and their tests.
5. `cmd/vtt/state_dump.go` and a test beside `state_dump_head_test.go`.
6. `docs/verification-debt.md`, `docs/requirements.md`.

The plan names every file; the verifier counted about 35.

Four components, the harness, `cmd/vtt`, `internal/mcp` and the browser
client's comments, and more than ten files. The owner set the scope on
2026-10-20 and added the client comments on 2026-10-21. No contract change:
every frame these readers need exists.

## Specifications this moves

docs/specifications/007-the-wire-contract.md
docs/specifications/015-the-seat-and-the-perch.md
New: what a client holds and offers, the browser's rules moved out of SPEC-007 and the Go readers' added (SPEC-022)

## What could not be established

- What `vtt state dump` does at a switch was open when this was written. The
  owner ruled on 2026-10-21 that it refuses and names both logs.
- Whether `vtt events tail` should mark a switch. Its output is one protojson
  `Envelope` per line, so a marker changes the format; this ticket leaves it
  as it is.
- Whether `vtt mcp` should tell an agent seated as a player whom it may watch
  (`WatchableChanged`). Nothing in the owner's scope asks for it.
- How long `task check` takes: a comment change under `client/src`
  re-mutates the whole client, and the change to `internal/harness`,
  `internal/mcp` and `cmd/vtt` re-mutates those that the mutation gate
  covers.
