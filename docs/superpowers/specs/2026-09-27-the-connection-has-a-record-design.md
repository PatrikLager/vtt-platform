# The gateway's connection has a specification, and its own files carry only warnings and pointers

## The problem

How a WebSocket connection to the gateway behaves is decided in
`internal/gateway/server.go` (`handleWS`, `serve`, `writeUntilFlushed`,
`deliver`, `enqueueEvents`, the constants `gatewayBuffer`,
`gatewayNoProgress`, `maxWSFrameBytes`, `WithStatic`, `Handler`),
`keepalive.go` (`keepAlive`, `pingUntilStopped`, `gatewayPingInterval`,
`gatewayPingTimeout`, `writeActivity`), `presence.go`
(`presenceRegistry`, `presenceSendBudget`, `joinAndSend`, `leave`,
`announceIfPresent`, `announceIfAbsent`, `broadcast`) and `codec.go`
(`EncodeFrame`, `DecodeCommand`), and no specification states it. The
token is verified before the upgrade and a bad one gets HTTP 401; one
goroutine writes every frame, fed by three producers through a queue that
is never closed and is drained at teardown; a malformed or oversized inbound
frame closes only its own connection; the queue holds 256 frames and a
connection that accepts none for 30 seconds, or does not finish a write in
30 seconds, is closed; a connection silent for 20 seconds is pinged and one
that answers no pong within 60 seconds is closed without a close frame, and
no ping goes out while the writer is mid-frame; presence is a per-participant
count whose arrival is announced after the joiner's own pump runs, whose
departure is announced only if the participant is still gone, and whose
fan-out waits at most 3 seconds per connection; the static client is served
at `/`, unauthenticated, after every other route. Each of those sentences
lives today in a ticket's amendment (`docs/superpowers/specs/2026-07-23-api-gateway-design.md`
§3, `2026-08-06-presence-and-actor-control-design.md` §4 and §4.1) or in a
comment block, and the four files carry 1,555 comment lines out of 2,403
non-blank (counted at `b7b5b35`; `python3 tools/check-comments.py --report`
gives the shares:
`keepalive.go` 77.3 percent, `presence.go` 57.3, `codec.go` 37.5,
`server.go` 64.8 with 74 banned lines and 61 blocks over the bound). Two
records are wrong beside them: SPEC-007 says `CatchUpHead` carries "the
highest sequence already queued as this connection's catch-up backlog",
which is the log's head, while `seat.catchUp` answers a projected seat with
the last sequence that seat is sent, which can be lower; and the shutdown
race in `cmd/vtt/serve_compose.go` (`http.Server.Shutdown` does not wait for
hijacked WebSocket connections, so `closeFn` is safe only after every
connection has unwound) has as its only record a comment that cites
`.superpowers/sdd/progress.md`, a path CLAUDE.md rule 8 forbids. Two
comment sentences in `server.go` are false: `serve`'s doc counts two
producers of `outCh` where the presence registry is a third, and `Handler`'s
says it routes `/healthz` and `/ws` where it routes thirteen patterns.

## Done looks like

1. `docs/specifications/011-the-connection.md` exists with the five
   headings SPEC-007 uses, in the present tense: the handshake and its
   refusals, the single writer and its queue, the inbound frame bounds, the
   catch-up head as the seat's, keepalive, presence, teardown order, the
   static client, and the shutdown race as recorded debt; every sentence in
   it names the symbol that holds it, and Phase 4b reads each against the
   code.
2. SPEC-007's `CatchUpHead` paragraph says what `seat.catchUp` does for a
   projected seat, and points at SPEC-011.
3. `docs/verification-debt.md` gains the shutdown race under "Open debt",
   and the comment in `cmd/vtt/serve_compose.go` points at it instead of at
   the gitignored path: `grep -c 'superpowers' cmd/vtt/serve_compose.go`
   prints 0; today it prints 1.
4. Every rule the sort accepts has a row cited by a test under
   `internal/gateway/` that observes it; `task check:requirements-chain`
   prints more than 79 rows and holds; the report names any OPEN row.
5. `python3 tools/check-comments.py --report | grep -E 'gateway/(keepalive|presence|codec)\.go'`
   prints `banned 0` and `blocks>6 0` on all three lines, and every block
   left in them is a warning, a pointer or the doc sentence of an exported
   symbol (Phase 4b's reading, VTT-051); `server.go`'s blocks on the symbols
   the problem names are pointers to SPEC-011 or warnings, and its remaining
   blocks are untouched and named in the report as the next sweeps'. The
   ledger rows of the four files are lowered by `--write-ledger` in the same
   commit.
6. No code line changes: the go/scanner token stream of each of the four
   files, and of `cmd/vtt/serve_compose.go`, is identical to `b7b5b35`'s.
7. `task check` whole is green.

## Rules this puts on the system

Candidates, one line each, for the sort after sign-off; most have a test
already (`internal/gateway/server_test.go`, `presence_internal_test.go`,
`keepalive_internal_test.go`, `static_test.go`), and a test whose rule an
existing row states cites that row.

- A connection is upgraded only for a credential that verifies; a bad or
  revoked token is answered with HTTP 401 and no upgrade.
- Every frame a connection sends is written by one goroutine, in the order
  results, events and presence frames were queued.
- A connection's outbound queue is never closed, and teardown writes what
  is queued before the writer stops.
- A malformed or oversized inbound frame closes its own connection and no
  other.
- The catch-up head a connection opens with is the last sequence that seat
  will be sent, which for a projected seat can be below the log's head.
- A connection silent for the ping interval is pinged, and one that answers
  no pong within the pong budget is closed without a close frame; no ping is
  sent while the connection's writer is mid-frame.
- A participant's arrival is announced to the table after their own
  connection's pump is running, and never before their snapshot.
- A participant is announced gone only when their last connection ends and
  only if no connection of theirs exists when the announcement is sent.
- A presence fan-out waits at most the send budget for each connection and
  then drops that frame for that connection.
- A client force-closed for not reading is announced gone like a clean
  departure.
- The static client is served at `/` without authentication and never
  shadows an API route.

## What it touches

1. `docs/specifications/011-the-connection.md`, new
2. `docs/specifications/007-the-wire-contract.md`, the `CatchUpHead`
   paragraph
3. `docs/verification-debt.md`, one entry under "Open debt"
4. `internal/gateway/keepalive.go`, `presence.go`, `codec.go`, comments only
5. `internal/gateway/server.go`, the comment blocks on the symbols named
   above only
6. `cmd/vtt/serve_compose.go`, the `composeServer` comment's race paragraph,
   deleted, and a warning beside `closeFn` (`check:comments` refuses a line
   added to a block over the bound, and that comment is one block)
7. `internal/gateway/*_test.go`, citation lines where a row is dispensed or
   an existing one cited, and the five comment blocks whose pointers aimed at
   blocks this sweep cuts, sorted to the bound (sign-off question 8 answered
   yes; the same refusal as item 6 makes a re-aimed line a sorted block)
8. `docs/requirements.md`, rows after sign-off, by the dispenser
9. `tools/comment-ceilings.txt`, by `--write-ledger` only

One component; the specification first, the register with it, the comment
sort after, the ledger last, in one commit; the report in its own.

## Specifications this moves

New: the gateway's connection — handshake, writer, bounds, keepalive,
presence, teardown, static client.
docs/specifications/007-the-wire-contract.md

## What could not be established

- Which of `server.go`'s 61 blocks over the bound belong to the connection
  and which to the command path, projection or presence-announcement seams
  it also hosts; the plan lists them by symbol, and the reading decides.
- Whether `keepalive.go`'s argument for skipping a ping while the writer is
  busy (a control frame contends for the library's frame lock and loses in
  five seconds) is a fact SPEC-011 states or a warning the code keeps; both,
  probably, since the next editor of `keepAlive` needs it at the line.
- Whether the shutdown race is a debt entry or a ticket: it is unsolved
  since 2026-08, no test holds it, and `vtt serve` runs `closeFn` after a
  bounded `Shutdown` regardless; the entry is the record, a ticket is the
  fix.
- Rule 9 of CLAUDE.md: MapTool's `net.rptools.clientserver` is the source
  of the 20-second heartbeat and one-minute timeout already, recorded in the
  keepalive amendment; the specification repeats the borrowing and the
  direction it does not borrow.
