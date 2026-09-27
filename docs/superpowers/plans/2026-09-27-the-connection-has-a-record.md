# The gateway's connection has a specification, and its own files carry only warnings and pointers — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-27-the-connection-has-a-record-design.md`
**Verified:** 2026-09-27, by `verify-ticket`, an agent that did not write the
ticket, against `b7b5b35` on `chore/spec-011-the-connection` (`main` at the
time; the ticket untracked). Verdict: **Passes with gaps.** The gaps are listed
at the end and travel with this plan. This plan does not edit the ticket; where
an item is thin, the plan decides around it and says so.

**Goal, in the ticket's words:** `docs/specifications/011-the-connection.md`
exists, SPEC-007's `CatchUpHead` paragraph says what `seat.catchUp` does, the
shutdown race is recorded debt, every rule the sort accepts has a row, and the
four gateway files carry only warnings, pointers and doc sentences, their
ceilings lowered, with no code line changed.

**MapTool (CLAUDE.md rule 9), in one line.** Already answered at the source
and repeated here: the heartbeat's intervals are borrowed from
`net.rptools.clientserver` (a 20 s heartbeat against a one-minute socket
timeout, the ticket's 2026-08-26 amendment, Patrik's call) and its direction is
not (MapTool's heartbeat is client-side; ours is server-side, so no client code
takes part); the two teardown paths are treated as one as MapTool's
`releaseClientConnection` treats a network failure and a clean quit; the
distribution model is not borrowed (`internal/gateway/seat.go`, CLAUDE.md
rule 9). Where that provenance is written is D3.

## Measurements this plan stands on

All at `b7b5b35`, by command, run by the verifier.

`python3 tools/check-comments.py --report | grep -E 'gateway/(keepalive|presence|codec|server)\.go'`
and `grep -c '^\s*//'` / `grep -c '\S'` per file:

| File | Comment lines / non-blank | Share | Ceiling | Banned | Blocks > 6 | Tokens (D11) |
|---|---|---|---|---|---|---|
| `keepalive.go` | 221 / 286 | 77.3 | 77.3 | 5 | 6 | 392 |
| `presence.go` | 224 / 391 | 57.3 | 57.3 | 14 | 11 | 1,166 |
| `codec.go` | 12 / 32 | 37.5 | 37.5 | 2 | 1 | 130 |
| `server.go` | 1,098 / 1,694 | 64.8 | 64.9 | 74 | 61 | 4,596 |
| `cmd/vtt/serve_compose.go` | not swept; one comment | — | 72.0 | — | — | 691 |

Total 1,555 of 2,403, the ticket's figures. `grep -c superpowers
cmd/vtt/serve_compose.go` prints 1. `cmd/vtt/library_test.go`'s doc block
above `TestThreeRoleExitScenarioOverLiveServeSubprocess` also names "the
connection-drain carry-forward (docs/superpowers/sdd/progress.md)", a path
that does not exist; it is a second comment about the race and not a record
(gap 3).

**`server.go`, by block.** One hundred comment blocks. The blocks on the
symbols the ticket names hold about 476 of the 1,098 comment lines, 27 of the
61 blocks over the bound and 22 of the 74 banned lines; the rest sit on
`Server`'s fields, the `With*` constructors, `describeBlockage`,
`answerCommand`, `authorize`, `handleSetViewpoint`, `handleCommand`, the
`announce*` and `revoked` helpers and the batch handlers (D7 lists both
sides). `Handler` registers thirteen patterns: `/healthz`, `/ws`, `POST
/join`, nine `GET /api/...` routes, and `/` when `WithStatic` was called; its
doc names two. `serve`'s doc names two producers of `outCh`, the read loop and
the pump; `presenceRegistry.send` in `presence.go` writes the snapshot,
arrivals, departures and promotion nudges into the same channel, and `serve`
itself enqueues the catch-up head before either goroutine exists.

**The seat's head.** `seat.catchUp` returns `logHead` unchanged for an
unprojected seat (`s.pr == nil`) or an empty log, and for a projected seat the
sequence of the last envelope `s.receive` produced — which is below the log's
head when the last log events project to nothing for that seat, and 0 when
nothing does. SPEC-007's paragraph, and the doc comment above `message
CatchUpHead` in `contract/vtt/v1/commands.proto`, both say "the highest
sequence already queued as this connection's catch-up backlog" and "0 means the
log was empty at subscribe time". `TestEverySeatCanReachTheCatchUpHeadItIsGiven`
in `server_visibility_test.go` asserts the seat's head for a player and a
spectator and the log's for the DM and the agent; it carries no `VTT-NNN`.

**The two false sentences.** `serve`'s "Both the command loop ... and the
broadcast pump goroutine ... only ever hand byte slices to outCh": the registry
is a third source (above). `Handler`'s "routing /healthz and /ws (spec §3)":
thirteen patterns (above).

**Every sentence of the ticket's first paragraph, against its symbol.** 401
before upgrade: `handleWS` calls `s.ids.Verify` before `websocket.Accept` and
answers `http.StatusUnauthorized`; a malformed `after` is answered 400 first,
by `parseAfter`. Never closed and drained: `serve` closes `flush`, never
`outCh`; `writeUntilFlushed` drains on `flush`. Malformed closes its own:
`serve`'s `DecodeCommand` arm calls `shutdown()` then `conn.Close(StatusPolicyViolation)`;
oversized: `conn.SetReadLimit(maxWSFrameBytes)` in `handleWS` makes `conn.Read`
fail, and the library closes with `StatusMessageTooBig`. 256, 30 s, 30 s,
32768: `gatewayBuffer`, `gatewayNoProgress`, `New`'s `writeTimeout:
gatewayNoProgress`, `maxWSFrameBytes`. 20 s and 60 s: `gatewayPingInterval`,
`gatewayPingTimeout`. No ping while busy: `keepAlive`'s `if busy() { continue }`,
fed by `stampedWrite` around the real write. Closed without a close frame:
`pingUntilStopped`'s `closeNow` is `conn.CloseNow`. Arrival after the pump:
`s.announcePresence(pc)` follows the `go func()` that runs the pump. Departure
only if still gone: `announceDeparture` calls `announceIfAbsent`, which
re-reads `r.counts` under `fanOut` then `mu`. 3 s: `presenceSendBudget`.
Static at `/` last: `Handler`'s final `mux.Handle("/", ...)`.

**Records and gates.** `docs/specifications/` holds 007 to 010; 011 is free.
`docs/requirements.md` has 79 rows, the last `VTT-079`; `task
check:requirements-chain` prints `79 rows, 193 test files, 4 specifications`.
`python3 tools/check-comments.py main` ends `239 files, 0 added comment lines,
239 ledger rows; clean`; `python3 tools/check-doc-owner.py .` ends `79 files,
every doc comment sits on its own function`; `python3
tools/check_mutation_test.py -q` prints `OK`. `docs/verification-debt.md` has
an `## Open debt` heading whose entries are a bold title, a paragraph, and
`Labels:` from the table at the top of the file.

**The ledger at the base.** `--write-ledger` in a scratch clone at `b7b5b35`
changes ONE row: `cmd/vtt/mcp_ruleset_e2e_test.go` 24.6 to 24.4, a file this
work does not touch whose share fell within the band before this ticket. So
the sweep's `--write-ledger` moves five rows, not the ticket's four (D12).

**gofmt at the base.** `gofmt -l internal/gateway/` prints `keepalive.go` and
`scenario_test.go`. `keepalive.go`'s only difference is inside
`pingUntilStopped`'s doc block (gofmt wants blank `//` lines between the list's
items), a block the sweep cuts; `scenario_test.go`'s is a struct-literal
alignment in code, not this ticket's (D21).

**Mutation keys.** `tools/mutation-equivalents.txt` names one gateway file,
`project.go`; no key names any of the four files or `serve_compose.go`, so no
key moves. `./internal/gateway/` is in `tools/check-mutation.py`'s `PACKAGES`
and is re-mutated by `task check`.

**Citations in the gateway's test files** (`grep -o 'VTT-[0-9]*' | sort |
uniq -c`): `server_test.go` 11 tokens over nine ids (VTT-020, 025, 026, 031,
032 three times, 033, 036, 043, 047); `authz_test.go` 5; `join_test.go` 14;
`qa_joining_test.go` 3 (VTT-036); `server_internal_test.go`,
`presence_internal_test.go`, `keepalive_internal_test.go`,
`keepalive_conn_internal_test.go`, `static_test.go`, `codec_test.go` and
`server_visibility_test.go` none.

**Test prose that points into blocks the sweep cuts** (gap 1): `server_test.go`'s
frame-demultiplexing header (`see serve's "Writer choice" comment in server.go,
which says so outright -- "the ordering of interleaved results/events is
whatever order they arrive at outCh"`) and
`TestOversizedFrameClosesConnectionMaxLegalPayloadWorks`'s doc (`server.go's
maxWSFrameBytes doc comment`); `server_internal_test.go`'s
`TestAJoinerDoesNotWaitForItsOwnArrivalToBeAnnounced` body (`deliberately — see
server.go`) and `TestServeNeverClosesAConnectionsOutboundChannel` body (`the
shutdown comment quotes close(outCh)`); `keepalive_internal_test.go`'s
`TestThePongBudgetStaysAtLeastThreeIntervals` doc (`gatewayPingTimeout's doc
comment calls "a floor rather than a nicety"`) and its `t.Errorf` string, which
is code.

**`TestServeNeverClosesAConnectionsOutboundChannel` reads `server.go` as
text**, comments stripped at the first `//` on each line, and asserts that
`outCh := make(chan []byte` is declared and `close(outCh)` absent. A comment
sweep cannot move it; D11's token comparison is what keeps its premise true.

**The break's arithmetic (D13).** `--write-ledger` writes `ceil1(share)`, at
most 0.1 of headroom. After the sweep one comment line moves `keepalive.go`'s
share by about a point (some 65 code lines plus what survives), `codec.go`'s by
several, `presence.go`'s by about 0.4, and `server.go`'s by about 0.08 — under
the rounding, so a `// SPEC-011` line in `server.go` may NOT red the gate. The
break is placed in `keepalive.go`.

**D11's instrument** built and run: five pairs identical at the base, counts in
the table. Deleting every `//` line from `keepalive.go` gives the same 392.

## Constraints that bind every task

- `CLAUDE.md` rule 10 and SPEC-010: a comment is an imperative warning, a
  pointer, or the one-line doc sentence of an exported symbol. VTT-051 is held
  by the Phase 4b reading; VTT-050, VTT-052, VTT-053 and VTT-055 by
  `check:comments`; VTT-059 says a citation line moves no share.
- `CLAUDE.md` rule 2: no gate is weakened. The ledger only goes down, only
  through `--write-ledger`.
- `CLAUDE.md` rule 8, narrowed by rule 10 for code: a comment points at a
  specification by number, a requirement by id, a test or symbol by name, or a
  report by its `docs/` path; never a line number, a date, a commit hash, a
  plan, a task, `§`, `#NNN` or a `.superpowers/` path.
- `CLAUDE.md` rule 3: the contract is not touched. The proto's `CatchUpHead`
  doc is SPEC-007's and out of this ticket (gap 2).
- SPEC-008: ids come from `requirement-id`
  (`~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`, on the
  path), after sign-off, never by hand; every `VTT-NNN` a test file carries
  today stays.
- The `specification` skill's form and `catches.md`: five headings, present
  tense, no `Why`, no `Rejected`, no measurement, no line number, no past-tense
  account that is not about the code, no path outside the project, one
  decision per file.
- The `requirements` skill: one thing, what not how, breakable, measurable,
  named by a check or knowingly OPEN; refuse more than you accept.
- The ticket: no code line changes (D11 is the check); the four files, the
  named `server.go` symbols, the `composeServer` comment, test files' citation
  lines, SPEC-007's one paragraph, one debt entry, the register, the ledger.
- `internal/gateway/seat.go`, `export_test.go`, `cmd/vtt/serve.go`,
  `cmd/vtt/library_test.go` and the two source tickets are not touched.
  `docs/reports/` gains only this ticket's report.

## Decisions this plan makes

**D1. SPEC-011's sections, each lifted from a named source and held by named
symbols.** Forced by ticket item 1 (the sections it lists) and the
`specification` skill's step 3 (each paragraph about something opened). Under
"How it works", in this order, one bold-led paragraph each:

| Section | Source sentence (where it lives today) | Symbols that hold it | Tests that pin it |
|---|---|---|---|
| The handshake and its refusals | api-gateway §3 "One WebSocket endpoint `/ws?token=`"; SPEC-009 "`handleWS` verifies the token ... before `websocket.Accept`; a token `identity.Verify` refuses gets a 401" | `handleWS`, `parseAfter` (400 for a malformed `after`, before the token), `websocket.Accept`, `conn.SetReadLimit(maxWSFrameBytes)` | `TestConnectBadTokenRejectedBeforeUpgrade`, `TestConnectRevokedTokenRejectedBeforeUpgrade` |
| The routes | `Handler`'s body | `Handler`: the thirteen patterns by name; `/` only with `WithStatic` | `TestHealthzOK`, `TestStaticDoesNotShadowTheAPI` |
| The single writer and its queue | `serve`'s "Writer choice" doc; presence §4.1's 2026-08-11 amendment "a connection's outbound channel is never closed ... the writer stops on a separate `flush` signal and drains what is queued" | `serve` (`outCh`, `flush`, `writerDone`), `writeUntilFlushed`, `stampedWrite`, the four sources: the read loop, the pump (`deliver`, `enqueueEvents`), `presenceRegistry.send`, and `serve`'s own head frame | `TestTheWriterDrainsWhatIsQueuedBeforeItStops`, `TestTheWriterStopsAtTheFirstFailedWrite`, `TestServeNeverClosesAConnectionsOutboundChannel` |
| The bounds | api-gateway §3 "buffer 256 — a named constant; overflow = WebSocket close"; the three constants' docs; `New` | `gatewayBuffer`, `gatewayNoProgress` (the store's no-progress budget through `campaign.SubscribeWithNoProgressTimeout`, and `writeTimeout` by `New`), `maxWSFrameBytes`, the writer's `context.WithTimeout`, the pump's post-loop `CloseNow` | `TestAClientThatStopsReadingEntirelyIsTornDown`, `TestAWedgedConnectionIsTornDownAndOthersKeepServing`, `TestMalformedFrameClosesOnlyThatConnection`, `TestOversizedFrameClosesConnectionMaxLegalPayloadWorks` |
| The catch-up head as the seat's | `seat.catchUp`'s doc ("the sequence THIS SEAT's catch-up actually ends at"); api-gateway §3's 2026-08-02 amendment | `newSeat`, `subscribeFrom` (a projected seat subscribes from 0), `seat.catchUp`, `serve`'s `CatchUpHead` encode and its fail-closed arm | `TestEverySeatCanReachTheCatchUpHeadItIsGiven`, `TestCatchUpHeadEncodeFailureClosesTheConnection`, `TestConnectAfterZeroReceivesFullHistoryThenLive` |
| Delivery | the pump's comments; `enqueueEvents`' "ALL OR NOTHING PER BATCH"; SPEC-009's re-resolution sentence (pointed at, not restated) | the pump goroutine: `credentialGone` before the backlog and before each event, `sub.receive`, `perches`, `deliver`, `enqueueEvents`, `errEncodeFrame`, `errWriterGone` | `TestAnEncodeFailureTearsTheConnectionRatherThanTheBatch`; VTT-032, VTT-033 by pointer to SPEC-009 |
| Keepalive | api-gateway §3's 2026-08-26 amendment, whole | `gatewayPingInterval`, `gatewayPingTimeout`, `keepAlive`, `pingUntilStopped` (only `pctx`'s deadline is a verdict; `CloseNow`), `writeActivity`, `stampedWrite`, `serve`'s `go pingUntilStopped` after the arrival announcement | the seven tests in `keepalive_internal_test.go`, the three in `keepalive_conn_internal_test.go` |
| Presence | presence §4, §4.1 and its three amendments | `presenceRegistry` (`fanOut` then `mu`), `joinAndSend`, `leave`, `send`, `presenceSendBudget`, `targets`, `broadcast`, `announceIfPresent`, `announceIfAbsent`, `participantIDs`; `announcePresence`, `announceDeparture`, `revoked`, `announcePromotion`; `serve`'s order: head, `joinAndSend`, pump, then `announcePresence` synchronously | the twenty-one tests in `presence_internal_test.go`; `TestPresenceAnnouncesAnArrival`, `TestPresenceAnnouncesACleanDeparture`, `TestAReconnectingPlayerIsNeverAnnouncedGone`, `TestASecondDeviceIsNotASecondArrivalOrDeparture`, `TestAJoinerDoesNotWaitForItsOwnArrivalToBeAnnounced` |
| Teardown | `shutdown`'s comment; presence §4.1 "Both teardown paths must be covered" | `shutdown` (`leavePresence`, `closing`, `unsubscribe`, `pumpDone`, `flush`, `writerDone`), the deferred `leavePresence` and `CloseNow`, the paths that reach it: a read error, a malformed frame, a revoked credential, a dead writer, the pump's forced close, the pinger's `CloseNow`, the write deadline | `TestAForceClosedClientIsAnnouncedGone`, `TestPresenceAnnouncesACleanDeparture`, `TestAClientThatStopsPongingIsAnnouncedGone` |
| The static client | `Handler`'s closing comment; `WithStatic`'s doc | `WithStatic`, `Handler`'s `http.FileServerFS` at `/` | the five tests in `static_test.go` |
| Process shutdown | `composeServer`'s comment; `cmd/vtt/serve.go`'s `RunE` | `composeServer`'s `closeFn`; `serve.go`'s `serveShutdownTimeout`, `srv.Shutdown`, `srv.Close`, then `closeFn` regardless | none; the debt entry (D4) |

Each sentence names the symbol that holds it, and each is checked against the
code before it is written and re-read by Phase 4b (D16). Status:
`Accepted. Implemented by internal/gateway/server.go, keepalive.go, presence.go,
codec.go, seat.go and cmd/vtt/serve_compose.go; pinned by <the tests above>.`
"Principles served" says what SPEC-009 and SPEC-010 say: no blueprint, the
principle missing from the record rather than absent (here: a connection is a
seat, not a person). "Consequences" holds what a client author is bound by that
SPEC-007 does not already say: a bad token is refused before any frame; an
oversized frame ends the connection; a pong not answered ends it with no close
frame; a presence frame may be dropped for a connection that does not drain
within the budget and is repaired by the next snapshot; a head is the seat's
and is always reachable. "Requirements" lists the ids the sort dispenses (D6)
and nothing else.

**D2. What SPEC-011 does not restate.** Forced by `catches.md` item 13 (two
sources). SPEC-007 owns what is on the wire: the frame kinds, the ordering
contract, what the connection opens with, that presence frames are not
Envelopes, that `DISCONNECTED` is per participant. SPEC-009 owns
authentication and re-resolution. SPEC-011 states the mechanism of each and
points at the record that owns the promise (`SPEC-007`, `SPEC-009`, `VTT-032`,
`VTT-033`) rather than repeating its sentence. Where a SPEC-011 paragraph needs
the promise to read (the per-participant count), it says "as SPEC-007 states"
and gives the mechanism.

**D3. Provenance stays out of the record.** Forced by `catches.md` items 3 and
12. "Borrowed from MapTool" is history and MapTool is outside the project. The
ticket's last bullet asks the specification to "repeat the borrowing and the
direction it does not borrow"; this plan writes the DIRECTION as a fact about
this system (the heartbeat is server-side; a browser answers below JavaScript;
no client code takes part) and the borrowing as the rule-9 answer in this plan
and the report. Q1.

**D4. The shutdown race is a debt entry, and SPEC-011 describes what the code
does.** Forced by ticket item 3 and `catches.md` items 5 and 7. SPEC-011's
process-shutdown paragraph states the present: `http.Server.Shutdown` does not
wait for hijacked connections, `vtt serve` runs `srv.Shutdown` under
`serveShutdownTimeout`, then `srv.Close`, then `closeFn` whether or not a
connection is mid-teardown, and the e2e test closes its one connection first.
Nothing is "not yet implemented", so no ticket is owed by the record; the
claim on future work is the debt entry, under `## Open debt`, in the file's
shape: a bold title (**A WebSocket connection can outlive `Shutdown` and see
its handles closed under it.**), the mechanism in one paragraph naming
`composeServer`, `closeFn`, `serve.go`'s `RunE` and `serve_e2e_test.go`, the
label `test data missing` (no fixture holds a connection mid-teardown across
`Shutdown`), what closing it needs (draining every open gateway connection in
shutdown, or a wait on `serve` returns), and "Recorded 2026-09-27, moved here
from the comment at `composeServer`". The `composeServer` comment keeps a
two-line warning: `Do not call closeFn until every gateway connection has
unwound: Shutdown does not wait for hijacked connections
(docs/verification-debt.md).` The rest of that block — the campaign-directory
history, the `rulesetDir`, `adventuresDir`, maps and art paragraphs — is not
this ticket's (item 6 says the `composeServer` comment; the sentence about the
race is the reason) and stays. Q2 asks whether the whole block is meant.

**D5. The sort's starting point is the table below; nothing is an id until
sign-off.** Forced by SPEC-008 and the `requirements` skill. Rows are lettered
so nothing here reads as an id. Each line: the rule as a what; the outcome
proposed; the evidence. The reading after sign-off confirms or overrides each
and reports what it refused.

| # | Rule (what) | Proposed | Evidence |
|---|---|---|---|
| A | A credential that does not verify is answered 401 and the connection is never upgraded. | accept | `TestConnectBadTokenRejectedBeforeUpgrade`, `TestConnectRevokedTokenRejectedBeforeUpgrade` (`server_test.go`) |
| B | Frames reach a connection in the order they were queued. | accept — the what of the ticket's "one goroutine"; the goroutine is how, SPEC-011 prose | `TestTheWriterDrainsWhatIsQueuedBeforeItStops`, `TestConnectAfterZeroReceivesFullHistoryThenLive` |
| C | Teardown writes what is queued before the writer stops. | accept — the ticket's third candidate split | `TestTheWriterDrainsWhatIsQueuedBeforeItStops` (`server_internal_test.go`) |
| D | A connection whose write has failed is not written to again. | accept — from the test | `TestTheWriterStopsAtTheFirstFailedWrite` |
| E | A frame handed to a connection during its teardown is dropped, and the sender is unharmed. | accept, worded on the what; "never closed" is how | `TestServeNeverClosesAConnectionsOutboundChannel` (source-level; Q3) |
| F | A malformed or oversized inbound frame closes the sending connection and no other. | accept as one (one trigger class, one isolation) | `TestMalformedFrameClosesOnlyThatConnection` (both halves), `TestOversizedFrameClosesConnectionMaxLegalPayloadWorks` (the close, and a frame at the limit accepted; it does not observe "no other") |
| G | The catch-up head a connection opens with is the last sequence that seat is sent. | accept; "below the log's head for a projected seat" is explanation, SPEC-011 prose | `TestEverySeatCanReachTheCatchUpHeadItIsGiven` (`server_visibility_test.go`) |
| H | A connection that cannot be told its catch-up head is closed rather than served. | accept — from the test | `TestCatchUpHeadEncodeFailureClosesTheConnection` |
| I | An envelope that cannot be encoded ends the connection; no later envelope of that batch is sent. | accept — from the test | `TestAnEncodeFailureTearsTheConnectionRatherThanTheBatch` |
| J | A connection that stops taking frames is closed. | accept — the problem paragraph's sentence, not in the ticket's list; the two budgets are how, and the tests carry the figures | `TestAClientThatStopsReadingEntirelyIsTornDown`, `TestAWedgedConnectionIsTornDownAndOthersKeepServing` |
| K | Closing a connection that stopped taking frames disturbs no other connection. | accept | `TestAWedgedConnectionIsTornDownAndOthersKeepServing` |
| L | A connection silent for the ping interval is pinged. | accept — the ticket's sixth candidate split in three | `TestAnIdleConnectionIsPinged` (`keepalive_conn_internal_test.go`) |
| M | A connection that answers no pong within the pong budget is closed, and one that answers every ping is not. | accept; "without a close frame" is SPEC-011 prose — no test observes the close status (gap 8) | `TestAClientThatStopsPongingIsAnnouncedGone`, `TestAPongThatNeverComesReapsThePeer`, `TestAClientThatKeepsAnsweringIsNotReaped` |
| N | A ping that could not be sent is not a verdict about the peer. | accept — from the test | `TestASendFailureIsNotAVerdictAboutThePeer` |
| O | No ping is sent while the connection's writer is mid-frame. | accept | `TestNoPingGoesOutWhileTheWriterIsBusy`, `TestTheWriterIsReportedBusyForExactlyTheDurationOfAWrite`, `TestWriteActivityReportsAWriteInFlight` |
| P | One lost pong never reaps a peer. | accept, reworded so the test carries the figure (the 3x floor) | `TestThePongBudgetStaysAtLeastThreeIntervals` (Q4) |
| Q | A joiner's catch-up is not delayed by the announcement of its own arrival. | accept — the what of "after the pump is running" | `TestAJoinerDoesNotWaitForItsOwnArrivalToBeAnnounced` |
| R | The snapshot a connection opens with lists every present participant once, the joiner included. | accept — the what of "never before their snapshot": the joiner never receives its own arrival; its snapshot is its picture | `TestSnapshotListsEachParticipantOnceIncludingTheJoiner`, `TestPresenceAnnouncesAnArrival` |
| S | Presence is announced per participant: a second connection is not an arrival and closing one of two is not a departure. | accept as one row (one thing: the count is per participant); Q5 | `TestJoinReportsFirstConnectionOnlyOnce`, `TestLeaveReportsLastOnlyWhenEveryConnectionIsGone`, `TestASecondDeviceIsNotASecondArrivalOrDeparture` |
| T | A departure is not announced if the participant has reconnected before it is sent. | accept | `TestADepartureIsNotAnnouncedIfTheyHaveAlreadyComeBack`, `TestARealDepartureIsStillAnnounced`, `TestAReconnectingPlayerIsNeverAnnouncedGone` |
| U | A presence announcement waits for a connection that is merely busy. | accept — the ticket's ninth candidate split | `TestBroadcastWaitsForAConnectionThatIsMerelyBusy` |
| V | One connection that does not drain costs no other connection its announcement. | accept; "then drops that frame" is the consequence, SPEC-011 prose | `TestBroadcastIsBoundedByAWedgedConnectionNotStalledByIt` |
| W | A joiner does not wait for another connection's fan-out. | accept — from the 2026-08-11 amendment | `TestAJoinerDoesNotWaitOutSomebodyElsesFanOut` |
| X | Presence announcements reach every connection in one order. | accept — from the tests | `TestConcurrentFanOutsReachEveryConnectionInTheSameOrder`, `TestAPromotionInFlightIsNotOvertakenByTheDeparture`, `TestAnnounceIfPresentSaysNothingAboutSomebodyWhoHasLEFT` |
| Y | A connection the server closes is announced gone as a clean departure is. | accept | `TestAForceClosedClientIsAnnouncedGone`, `TestPresenceAnnouncesACleanDeparture`, `TestAClientThatStopsPongingIsAnnouncedGone` |
| Z | An announcement reaches a participant's other connections, and not the connection it is about. | accept-leaning; the reading decides | `TestBroadcastReachesEveryoneButTheExcludedConnection`, `TestBroadcastReachesEverySecondDeviceOfTheSameParticipant` |
| AA | The static client is served at `/` without authentication. | accept — the ticket's eleventh candidate split | `TestStaticServesTheClientAtRoot`, `TestStaticIsUnauthenticated` |
| AB | The static client never shadows an API route. | accept | `TestStaticDoesNotShadowTheAPI` |
| AC | A server without a bundle answers `/` with 404 and serves the API. | accept — from the test | `TestStaticAbsentIsNotAServerError` |
| AD | No request reaches a file outside the bundle. | accept-leaning | `TestStaticRefusesPathTraversal` |
| AE | `GET /healthz` answers 200. | accept-leaning; VTT-079's waits rely on it | `TestHealthzOK` |

Refused, or not this ticket's, each with its reason: `TestLeaveIsIdempotent`,
`TestLeaveOfAnUnknownConnectionIsNotADeparture`, `TestLeaveStopsDelivery`,
`TestAFanOutAbandonsAConnectionThatLeftMidWalk`,
`TestACancelledConnectionStopsItsOwnPinger` — registry and pinger internals, a
how each; `TestBroadcastSkipsAnybodyTheCallerHasDenied`,
`TestADepartureIsWithheldFromAnybodyTheCallerHasDenied`,
`TestSuppressionIsPerParticipantNotPerConnection` — the mechanism under
VTT-032, whose rule is stated on the wire and cited by `server_test.go`; not
cited (Q6); `TestDecodeCommandRoundTrip`, `TestDecodeCommandMalformedJSONErrorsCleanly`,
`TestEncodeFrameResultArmRoundTrips`, `TestEncodeFrameEventArmRoundTrips` —
the wire convention is SPEC-007's, which carries no rows; F holds the
connection-level consequence; `TestTwoClientsBothReceiveAcceptedCommandAsEvent`
— delivery is projected per seat, the visibility record's, not this one's;
`TestPlayerOwnershipDenialNoBroadcast`, `TestSpectatorCommandDenied`, the door
and promotion tests — authorization and identity, SPEC-009's and the authz
record's; `TestDescribeBlockage...` and `TestABlockedMoveRefusal...` — not
the connection. The ticket's sentence "every frame ... written by one
goroutine" is refused as a row and kept as SPEC-011 prose.

Thirty-one accepted or leaning against about fifteen refused is more than the
skill's caution expects, and the reading after sign-off is asked to cut, not
to add: the connection has many small promises and each row above names a
test that goes red on its own.

**D6. Rows are dispensed after sign-off, cited by citation lines only, and an
OPEN row has no citer.** Forced by SPEC-008, the identity-rules plan's D3 and
D12, and the debt file's open entry ("A test that cites a row marked OPEN
passes the chain gate"). Every row above names an existing test, so none is
expected OPEN; if the reading refuses a test as evidence and keeps the rule,
that row is `**OPEN — no test yet**` with no citation line, and the report
names it (ticket item 4). A citation line is `// VTT-NNN` directly above `func
Test`, below any doc block, one line per test, several ids on one line where a
test holds several (the register's convention, `TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse`
carries two). Test files get NO other edit (ticket item 7): the doc blocks
above these tests, some of twenty lines with banned terms, are the next
ticket's sweep, and the prose pointers in gap 1 stay as they are.
`server_visibility_test.go` is in scope for G's citation line and nothing
else. Evidence cells are written by hand after the dispenser, each entry
`internal/gateway/<file>#<Test>`; the chain gate refuses one that is wrong.

**D7. `server.go`: what is swept and what stays, by symbol.** Forced by ticket
item 5 ("the comment blocks on the symbols named above only"). Swept: the
blocks on `writeUntilFlushed`, `gatewayBuffer`, `gatewayNoProgress`,
`maxWSFrameBytes`, `WithStatic`, `Handler` (its doc and the three blocks
inside it), `handleWS` (its doc and the `SetReadLimit` block), `serve` (its
doc and every block inside it, from `newSeat` to the read loop's
`answerCommand` block), `deliver` (its doc and the block inside),
`enqueueEvents`; and the one trailing comment inside `handleWS` (`return //
Accept already wrote the HTTP error response.`), which becomes a warning if
kept. Not swept, named for the next sweeps: `Server`'s doc and its field docs
(`buffer`, `noProgress`, `writeTimeout`, `pingInterval`, `encodeFrame`,
`presence`, `onServeDone`, `ruleset`, `adventures`, `adventureGuides`,
`static`, `maps`, `mapsDir`, `artDir`, `cellPx`, `mapsMu`), `New`, `WithRuleset`,
`WithAdventures`, `WithMaps`, `WithMapsDir`, `WithArtDir`, `WithCellPx`,
`describeBlockage`, `answerCommand`, `authorize`, `handleSetViewpoint`,
`handleCommand` and its inner blocks, `announcePresence`, `announceDeparture`,
`revoked`, `announcePromotion`, `handleRemoveActor`, `handleJoinDoor`,
`handleRotateJoinLink`, `handlePromotion`, `credentialGone`, the sentinel
`var` block. `credentialGone` is in the caller's brief and not the ticket's
list; its seven-line block describes what SPEC-009 already records and stays
(gap 6). The seam fields describe the connection and stay (gap 5); SPEC-011
names them as what `New` sets. After the sweep `server.go` still carries about
34 blocks over the bound and about 52 banned lines, on the symbols above; the
report lists them by symbol as the next sweeps' and the gate does not refuse
them (no line is added to those blocks).

**D8. The sweep's rules, repeating the identity plan's where they apply.**
Forced by rule 10, SPEC-010, and `docs/superpowers/plans/2026-09-24-sweep-identity.md`
D1 to D8.

- *Three kinds and nothing else* (identity D1): a warning is imperative, a
  verb first, the consequence in the present tense, at most three lines; a
  pointer is `SPEC-011`, `SPEC-007`, `SPEC-009`, `VTT-NNN`, a test name, a
  symbol name, or `docs/verification-debt.md`, and may close a warning or a
  doc sentence in parentheses; a doc sentence is the first sentence `go doc`
  prints for an exported symbol. History, measurements, arguments, comparisons
  with earlier drafts, `§`, `#NNN`, dates, commit hashes and plan or task names
  go. Every fact a deleted block held that SPEC-011 does not state is either
  added to SPEC-011 (D9) or named in the report as dropped with the reason.
- *Unexported symbols keep no doc sentence* (identity D2). In these files that
  is `writeUntilFlushed`, `gatewayBuffer`, `gatewayNoProgress`,
  `maxWSFrameBytes`, `handleWS`, `parseAfter`, `serve`, `deliver`,
  `enqueueEvents`, `writeActivity` and its methods, `gatewayPingInterval`,
  `gatewayPingTimeout`, `keepAlive`, `pingUntilStopped`, `stampedWrite`,
  `presenceConn`, `presenceRegistry`, `newPresenceRegistry`,
  `presenceSendBudget`, `joinAndSend`, `send`, `targets`, `leave`,
  `announceIfPresent`, `announceIfAbsent`, `participantIDs`, `broadcast`: each
  keeps its warnings and pointers, and a block that holds neither goes. The
  exported ones in scope are `EncodeFrame`, `DecodeCommand`, `WithStatic` and
  `Handler`; `Server` and `New` are out of scope. `check:doc-owner` fires only
  when a doc's first word names another function, so a deleted doc cannot trip
  it. Q7.
- *The package doc* (identity D3) sits in `authz.go`, outside the four files;
  nothing here is exempt from the bound.
- *A doc sentence is one sentence, on one line where it fits* (identity D4);
  `check:new-prose`'s wrap band is `SHORT, LONG = 55, 85` in
  `tools/check-comment-wrap.py`, so a crammed line is refused. Phase 4b names
  each two-line case.
- *Test files* (identity D5) do NOT apply: ticket item 7 limits them to
  citation lines (D6).
- *No row is dispensed by the sweep itself* (identity D6): rows come from D5's
  sort after sign-off, in the same commit.
- *A decision only a comment holds moves to SPEC-011 in the same commit*
  (identity D7), D9 below.
- *What is left alone* (identity D8): `//` lines inside string literals (none
  in these files); directives (none: no `//go:` or `//nolint` line in the four
  files); the trailing comment in `handleWS`, which is invisible to the gate
  and gets the D1 sort anyway; and every comment outside D7's list.
- *Keepalive's busy-skip argument* (the ticket's second "could not be
  established"): both. SPEC-011 states the fact under Keepalive (a ping is
  skipped while the writer is mid-frame, because a control frame contends for
  the library's frame lock and loses in five seconds); `keepAlive` keeps a
  two-line warning at the `busy()` check: `Keep the skip: a ping into a busy
  writer loses the library's five-second control-frame race and reaps a
  healthy client (SPEC-011).` `pingUntilStopped` keeps one at the verdict:
  `Reap only on pctx's deadline: a send failure or net.ErrClosed says nothing
  about the peer, and the read loop's own error is what ends a dead socket
  (SPEC-011).` `serve` keeps one where the pinger starts: `Start the pinger
  after the hand-rolled teardowns above: a CloseNow from here would pre-empt
  their reasoned Close.`

**D9. Facts only a comment holds go into SPEC-011; SPEC-007 changes in one
paragraph.** Forced by ticket items 1 and 2 and identity D7. The reading of
each swept block asks whether SPEC-011's draft states the fact; if not, and the
fact is about the code now, it is checked against the symbol and written in.
SPEC-007's "What the connection opens with" paragraph becomes: `CatchUpHead` is
sent once, first, carrying the last sequence THIS SEAT's catch-up will deliver:
the log's head for the DM and the agent, and for a projected seat the sequence
of the last envelope its projection produces, which can be below the log's
head and is 0 when the log is empty or nothing in it reaches that seat
(SPEC-011). A client wanting a point-in-time snapshot reads until it has seen
`head_sequence`; one wanting a live tail ignores the frame. The
`PresenceSnapshot` sentence stays. Nothing else in SPEC-007 changes; its
Requirements line stays `None allocated`.

**D10. One commit for the change; the report in its own.** Forced by three
things: the band refuses a file more than 1.0 under its ceiling, so each
file's deletions and its row must land together; a row's evidence and its
citation line must land together or `task check` reds a commit nothing at
commit time notices (neither hook runs `check:requirements-chain` or
`check:comments`); and the pointers the sweep writes (`SPEC-011`) must have a
target in the same tree. The commit carries the ticket (untracked today), this
plan, SPEC-011, SPEC-007, the debt file, the register, the four files,
`serve_compose.go`, the test files' citation lines and the ledger. The
pattern is `8ac4913` then `8181d47`.

**D11. The comment-stripped comparison is a token stream.** Forced by identity
D11 (a printer re-derives blank lines; `gofmt` output would show false
differences) and by `TestServeNeverClosesAConnectionsOutboundChannel`, whose
premise is that `server.go`'s code is what it was. The program is the identity
plan's Task 0 listing, built once in the scratchpad (`$S/codetokens/codetokens`)
and run from the repository root over the five Go files:

    for f in internal/gateway/keepalive.go internal/gateway/presence.go \
             internal/gateway/codec.go internal/gateway/server.go \
             cmd/vtt/serve_compose.go; do
      git show "b7b5b35:$f" > "$S/before.go"
      "$S/codetokens/codetokens" "$S/before.go" > "$S/before.tok" || echo "SCAN FAILED $f"
      "$S/codetokens/codetokens" "$f" > "$S/after.tok" || echo "SCAN FAILED $f"
      printf '%s %s/%s tokens ' "$f" "$(wc -l < "$S/before.tok")" "$(wc -l < "$S/after.tok")"
      cmp -s "$S/before.tok" "$S/after.tok" && echo same || echo DIFFERS
    done

Done reads five `same` lines with the counts 392, 1166, 130, 4596 and 691 on
both sides. A run proves it ran by the counts; a zero is a failed run. The
test files are checked the same way after the citation lines go in (a
citation line is a comment and is dropped by the scanner), over every
`internal/gateway/*_test.go` that gained one.

**D12. The ledger, last, and five rows.** Forced by SPEC-010 (the band) and the
measurement above. After Phase 4b has settled and no comment will change,
`python3 tools/check-comments.py --write-ledger`, then `git diff
tools/comment-ceilings.txt` must show exactly five changed rows, each lowered:
`internal/gateway/codec.go`, `keepalive.go`, `presence.go`, `server.go`, and
`cmd/vtt/mcp_ruleset_e2e_test.go` from 24.6 to 24.4, the last pre-existing at
`b7b5b35` and not this work's doing. `serve_compose.go`'s row (72.0) moves
only if the `composeServer` edit lowers its share by more than the rounding;
the diff says. A sixth changed row means a file changed that this plan does not
name: stop, name it, and ask before committing. The commit message lists the
rows old and new.

**D13. The deliberate breaks, one per check this work relies on.** Forced by
the dev-cycle's rule that a check is proven by a red, and ticket items 3, 4
and 5. In a scratch clone (`git clone --no-hardlinks` into the scratchpad,
the final `git diff HEAD` applied with `git apply`, the untracked files copied
in, committed there, that clone's `main` pointing at that commit): first each
gate exits 0 with its completion line; then each break is one edit, the
finding recorded verbatim, the inverse edit, `git diff --stat` printing
nothing before the next.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | `// VTT-999` directly above one uncited test in `presence_internal_test.go` | `check:requirements-chain`: `cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 | one new row's evidence entry re-pointed at a test in a file that does not carry the id (`internal/gateway/codec_test.go#TestDecodeCommandRoundTrip`) | `check:requirements-chain`: `does not carry the id, so the link walks one way only` |
| B3 | SPEC-011's Requirements line gains `VTT-999` | `check:requirements-chain`: `cites VTT-999 ... (specification citation)` |
| B4 | one pointer line, `// SPEC-011`, inside `keepAlive`'s body in `keepalive.go`, between two code lines where no block can absorb it | `check:comments`: `internal/gateway/keepalive.go: comment share X is above its ceiling Y and this change added a comment line to it (SPEC-010)`, naming the file and both shares; the line carries no banned term and joins no block, so this is the only finding. Placed in `keepalive.go`, not `server.go`, for the arithmetic above |
| B5 | `if busy()` changed to `if !busy()` in `keepAlive` | D11's loop: `keepalive.go ... DIFFERS` |
| B6 | `grep -c superpowers cmd/vtt/serve_compose.go` after re-inserting the old path in the `composeServer` warning | prints 1 (item 3's observation goes back to today's) |

**D14. Gate steps before the commit, in order, on a tree the review has
settled.** `gofmt -l internal/gateway/ cmd/vtt/` prints only
`internal/gateway/scenario_test.go` (D21); `go vet ./internal/gateway/
./cmd/vtt/`; `go test -count=1 ./internal/gateway/...` green; D11's loop, five
`same`; `task check:comments` (expected at this point to refuse the four
files for the band until Task 9 writes the ledger, and nothing else);
`task check:doc-owner` (doc comments move); `task check:requirements-chain`
(`<79 + N> rows, 193 test files, 5 specifications; every citation resolves and
every row's evidence holds`); `task check:new-prose` (citations and wrap on
the added comment lines: every `SPEC-011`, test and symbol pointer the sweep
writes must resolve, and `tools/check-citations.py` reads `TestX` names and
three-segment identifiers); `python3 tools/check_mutation_test.py -q` and
`python3 tools/check_ts_mutation_test.py -q` (no key moves; the self-tests
prove the instrument); `task lint` (the `whitespace` linter refuses a blank
line left at the top of a block when a comment there goes). Then, after
Task 9, `task check` whole, once, on the final tree, launched in its own
session (`start_new_session=True`, or the run dies at the parent's teardown),
and only after Phase 4b has settled, so no edit lands mid-run. Then the
pre-commit hook's own set, the review gate among them, with `git add` and `git
commit` in separate calls.

**D15. Phase 4a is skipped, with its reason.** Independent QA derives tests
from a specification to find behaviour the implementer got wrong. This change
has no behaviour: D11 shows every Go file's code identical to `b7b5b35`, and
what is added is prose (SPEC-011, one SPEC-007 paragraph, a debt entry),
register rows, citation lines and comment deletions. What can be wrong is a
sentence, and a reading holds that: VTT-051 is `**READING — Phase 4b**`, and
SPEC-011's sentences are held the same way (D16). The report records the skip
under its own heading with this reason.

**D16. Phase 4b is the check for VTT-051 and for SPEC-011, sentence by
sentence.** The reviewer gets `git diff HEAD` (which shows staged deletions),
SPEC-007, SPEC-009, SPEC-010, SPEC-011, the register rows VTT-050 to VTT-059
and the new rows, the four files, `seat.go` and `serve_compose.go`. For every
surviving block it names the kind (D8) and, for a warning, the code it guards
and whether the consequence is true of that code; for a pointer, that the
target resolves; for a doc sentence, that the symbol is exported and the
sentence true. For every deleted block: did it hold a fact now in no record?
If yes, SPEC-011 (D9) or the report. For every SPEC-011 sentence: the symbol
it names, and whether the code under the symbol does what the sentence says —
the `specification` skill's catches item 8 (a claim taken from a comment) is
the one to watch, since every SPEC-011 sentence starts life in a comment; the
verifier's own re-reading of the ticket's paragraph is the first pass and not
the last. For the SPEC-007 paragraph: re-read against `seat.catchUp`. For
each dispensed row: the test named goes red if the rule is broken (name the
edit that would red it). For the debt entry: the label fits the table.

**D17. What the report records.** Per file: comment lines and share before and
after, blocks deleted, blocks kept by kind, the trailing comment's fate; the
sentences SPEC-011 took from comments, each with its symbol; the sort — every
row accepted with its id and test, every candidate refused with one line;
`server.go`'s remaining blocks by symbol; the five ledger rows old and new;
D11's counts; the breaks' finding lines; the gaps below as found or closed;
the test prose that now points at cut text (gap 1) and the proto's sentence
(gap 2), for the next tickets. The report does not revise
`docs/reports/2026-08-18-visibility.md` or the reports of the presence and
keepalive periods.

**D18. Order of work: the record first, then the files it is pointed at from.**
SPEC-011 is drafted before any comment is cut, so every pointer the sweep
writes has a target and every deleted fact has a home to be checked against.
Then `server.go`'s in-scope blocks (test prose and `keepalive.go`'s and
`presence.go`'s blocks point into them), then `keepalive.go`, `presence.go`,
`codec.go`, `serve_compose.go`; then SPEC-007 and the debt entry; then, after
sign-off of the sort, the rows and citation lines; the ledger last (D12).

**D19. Two things the ticket's scope leaves stale are left, and named.** The
test prose in gap 1 and the proto's `CatchUpHead` doc in gap 2. The ticket's
item 7 says citation lines only, and the proto is the contract (rule 3,
`task generate:contract`, `check:drift`). Both are listed in the report as the
next tickets' first lines. Q8 asks whether the five test-prose pointers may be
re-aimed in this change instead.

**D20. Nothing is cited from the two source tickets.** Their amendments are
the sources SPEC-011 lifts from (D1's table), and a specification points at
symbols, tests and other specifications, not at tickets. The tickets are not
edited: they are historical, and the record now is SPEC-011.

**D21. gofmt.** `keepalive.go`'s difference at the base is inside a block the
sweep cuts, so after the sweep `gofmt -l internal/gateway/` prints only
`scenario_test.go`, whose difference is in code and predates this ticket; it
stays and is named in the report. If a surviving list-shaped comment makes
gofmt object to a swept file, the list is rewritten as sentences: gofmt's own
output is not committed, because a reflowed comment is a comment edit like any
other and Phase 4b reads it.

## Candidates the reading starts from

Read by the verifier at `b7b5b35`. A starting point, not a verdict.

**Facts for SPEC-011 that only a comment states today.** `parseAfter` refuses a
malformed `after` with 400 before the token is read (`handleWS`). The write
deadline is `gatewayNoProgress`, set by `New`. A projected seat subscribes
from 0 whatever `after` says (`subscribeFrom`). The head frame is enqueued by
`serve` before the pump starts, and for a projected seat after the backlog is
projected (`seat.catchUp`). The backlog is sent by the pump behind one
`credentialGone` check; a perch frame and an event each behind `deliver`. An
encode failure of an event closes with `StatusInternalError` and a reason; a
dead writer closes with none (`deliver`). The pump's post-loop `CloseNow`
fires when the store ended the subscription and `shutdown` did not. Presence:
the snapshot is enqueued under `mu` and cannot park because `out` holds at most
the head at that moment and the buffer is `gatewayBuffer`; `joinAndSend` takes
`mu` alone; `announceIfPresent` and `announceIfAbsent` take `fanOut` first; a
`deny` set is resolved by `revoked()` before the fan-out, one `Lookup` per
connected participant; an encode failure of a presence frame sends nothing and
ends nothing. The keepalive verdict: only `pctx`'s deadline; the pinger has
three exits (ctx, stop, a reaping ping). `shutdown`'s order. Teardown reaches
`leavePresence` by `shutdown` and by the defer. The pinger starts after the
arrival announcement.

**Warnings to keep, imperative, per file.** `server.go`: do not write a second
response after `Accept` fails; keep the read limit pinned; never close
`outCh` (pointer to `TestServeNeverClosesAConnectionsOutboundChannel`); the
write deadline is load-bearing for the wedged path; `closing` before
`unsubscribe`; do not call `shutdown` from the pump; the credential check
before the backlog is unguarded on purpose (the mutation gate); `deliver`'s
`closing` is a pointer for `vet`; stop on the first encode failure (the batch
is ordered); start the pinger after the hand-rolled teardowns. `keepalive.go`:
the skip, the verdict, the ticker (missed ticks coalesce), the three exits,
`defer` in `stampedWrite`. `presence.go`: never close `out`; `fanOut` before
`mu`, never the reverse; resolve `deny` before taking `fanOut`; `leave` closes
`done` behind the membership check; the snapshot send under `mu` needs a bound
if the buffer ever shrinks. `codec.go`: keep the encoder on `Server`, not a
package global (a test swapping a global races another connection's
teardown). `serve_compose.go`: D4's two lines.

**Text that goes on sight.** Every `spec §`, `#47`, `#55`, `#18`, `PR #18`,
`656079f`, every date, `measured`, `used to`, `an earlier design`, `first
draft`, `review found`, `task-5`, `Task 4`, the `progress.md` path; the cost
arithmetic under `gatewayPingInterval`; the account of `#47` under `flush`,
`shutdown`, `presenceConn.out` and `send`; the `CatchUpHead` history under the
head frame; the "AN EARLIER VERSION OF THIS COMMENT ARGUED SOMETHING ELSE"
paragraph; the "told-why-SOMETIMES" measurement; every sentence that says what
the code does rather than what to keep.

Projection, not a target: `keepalive.go` near 30 percent, `presence.go` near
25, `codec.go` near 20, `server.go` near 54 (its out-of-scope blocks stay).
The reading governs; nothing is cut for a figure.

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository; the scratchpad, `$S`.

Build D11's program; run the loop (five `same`). Run `python3
tools/check-comments.py --report | grep -E 'gateway/(keepalive|presence|codec|server)\.go'`,
`task check:requirements-chain`, `python3 tools/check-comments.py main`,
`python3 tools/check-doc-owner.py .`, `gofmt -l internal/gateway/ cmd/vtt/`,
and the citation multiset per gateway test file; keep the outputs. Confirm
`requirement-id` is on the path.

**Done when:** the outputs match the measurements above (five `same` with the
counts; `79 rows ... 4 specifications`; `239 files ... clean`; `keepalive.go`
and `scenario_test.go` from gofmt); `requirement-id` prints its usage.

### Task 1 — SPEC-011

**Files:** `docs/specifications/011-the-connection.md`, new.

Per D1, D2, D3, D4's paragraph, D9's rule; the `specification` skill's steps
1 to 5 and 7 (`catches.md`, two passes at most). Step 6's Requirements line is
written `None yet; the sort of 2026-09-27-the-connection-has-a-record-design.md
fills it` until Task 7 and then copied from the register.

**Done when:** the file has the five headings in order, no `Why`, no
`Rejected`, no number that is a measurement, no line number, no date outside a
`docs/` path, no path outside the project; every sentence under "How it
works" names a symbol in the four files, `seat.go` or `cmd/vtt/`, and a table
of sentence to symbol is kept for Task 8; `task check:requirements-chain`
prints `5 specifications`.

### Task 2 — `server.go`, the in-scope blocks

**Files:** `internal/gateway/server.go`.

D7's list, under D8. Every deleted fact checked against SPEC-011's draft; each
gap goes into SPEC-011 (D9) or the report's dropped list. `Handler`'s doc
becomes one true sentence; `serve`'s doc becomes one true sentence naming the
four sources or a pointer to SPEC-011.

**Done when:** D11 prints `same` for `server.go` with 4596 tokens; `grep -cE
'spec §|#[0-9]+|20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]|measured|used to' ` over
the in-scope symbols' blocks prints 0 (checked by reading the diff hunks, since
the out-of-scope blocks keep theirs); no in-scope block exceeds six lines; `go
doc ./internal/gateway` lists `Handler` and `WithStatic` with a one-sentence
doc; `gofmt -l internal/gateway/server.go` prints nothing.

### Task 3 — `keepalive.go`, `presence.go`, `codec.go`

**Files:** those three.

D8 whole; the warnings named above; the busy-skip fact confirmed in SPEC-011
and its warning at `keepAlive`. `presence.go`'s pointers to `spec §4`, `§3.4`,
`#47`, `#55` become `SPEC-011` or `SPEC-007` pointers or go.

**Done when:** D11 prints `same` for the three (392, 1166, 130); `--report`
shows each with `banned 0` and `blocks>6 0`; `gofmt -l internal/gateway/`
prints only `scenario_test.go`; `go doc ./internal/gateway EncodeFrame` and
`DecodeCommand` each print one sentence.

### Task 4 — `serve_compose.go`, SPEC-007, the debt entry

**Files:** `cmd/vtt/serve_compose.go` (the `composeServer` comment's race
sentences only, D4), `docs/specifications/007-the-wire-contract.md` (the one
paragraph, D9), `docs/verification-debt.md` (one entry under `## Open debt`,
D4).

**Done when:** `grep -c superpowers cmd/vtt/serve_compose.go` prints 0; D11
prints `same` for it with 691 tokens; `git diff docs/specifications/007-the-wire-contract.md`
shows one paragraph changed and nothing else; the debt entry carries a label
from the file's table and names `composeServer`, `closeFn`, `RunE` and
`serve_e2e_test.go`; `cmd/vtt/serve.go`'s `see composeServer's hijack-contract
note` still finds a warning about the race at `composeServer`.

### Task 5 — Local gates, first pass

**Files:** none changed.

D14's list up to and not including `task check` whole.

**Done when:** each step exits 0 with its completion line, except
`check:comments`, which at this point refuses the four files for the band and
nothing else; `check:new-prose` reports no citation to a name the tree never
declared.

### Task 6 — Sign-off of the sort

**Files:** none.

D5's table is presented with the reading's confirmations and overrides, each
override with its reason. Nothing is dispensed before the answer.

**Done when:** each lettered row has one of: accept (with wording and
evidence), refuse (with reason), OPEN (with reason).

### Task 7 — Rows and citation lines

**Files:** `docs/requirements.md` (by the dispenser, then evidence cells by
hand), `internal/gateway/server_test.go`, `server_internal_test.go`,
`presence_internal_test.go`, `keepalive_internal_test.go`,
`keepalive_conn_internal_test.go`, `static_test.go`,
`server_visibility_test.go` (citation lines only, D6),
`docs/specifications/011-the-connection.md` (the Requirements line, copied).

**Done when:** `task check:requirements-chain` prints `<79 + N> rows, 193 test
files, 5 specifications; every citation resolves and every row's evidence
holds` with N the accepted count; every pre-existing `VTT-NNN` token is still
in its file (Task 0's multisets are a subset of the new ones); an OPEN row, if
any, has no citer (`grep -rn 'VTT-0NN' internal/` prints nothing); D11 over the
test files that changed prints `same` for each.

### Task 8 — Phase 4b, the reading review

**Files:** whatever its findings touch among the above.

Per D16. Findings are fixed and the affected task's "done" is re-run. The
review settles before Task 9 starts.

**Done when:** the review record names every surviving block's kind, every
deleted block's outcome, every SPEC-011 sentence's symbol and verdict, every
row's red-making edit, and reports no open finding; D11 prints `same` for all
five files.

### Task 9 — The ledger

**Files:** `tools/comment-ceilings.txt`, by `--write-ledger` only.

Per D12.

**Done when:** `git diff tools/comment-ceilings.txt` shows exactly the five
rows D12 names (six if `serve_compose.go` moved by more than the rounding, and
D12 says how to read a seventh); `task check:comments` ends `clean`;
`--report | grep -E 'gateway/(keepalive|presence|codec)\.go'` prints
`banned 0` and `blocks>6 0` on all three lines.

### Task 10 — The breaks and the whole gate

**Files:** none in the repository.

D13 in a scratch clone; then `task check` whole, once, per D14.

**Done when:** the clone's clean run exits 0 with each completion line, and
each of B1 to B6 produces the one red D13 names; `task check` exits 0 with
every step, `check:comments`, `check:requirements-chain` and `check:mutation`
among them, printing its own verdict.

### Task 11 — Commit, then the report

**Files:** the commit's, per D10; then
`docs/reports/2026-09-27-the-connection-has-a-record.md`.

The commit message lists the five ledger rows old and new, D11's counts, the
rows dispensed, and B4's finding line verbatim. After it: the report per D17
and the `implementation-report` skill, in its own commit.

**Done when:** `git show --stat HEAD~1` lists the ticket, this plan, SPEC-011,
SPEC-007, the debt file, the register, the four files, `serve_compose.go`, the
test files that gained a citation line, and the ledger, and nothing else;
`git diff --stat b7b5b35 -- docs/reports/` lists only the new report; `git
diff --quiet b7b5b35 -- internal/gateway/seat.go internal/gateway/export_test.go
cmd/vtt/serve.go cmd/vtt/library_test.go contract/` exits 0.

## Commits

| Commit | Carries | Gate steps it runs |
|---|---|---|
| C1 | the ticket, this plan, SPEC-011, SPEC-007's paragraph, the debt entry, the register, the four files, `serve_compose.go`, the citation lines, the ledger | D14's list by hand, `task check` whole (Task 10), then the pre-commit hook (lint, vet, tier-1, arch, vocabulary, doc-owner, secrets, typecheck, review gate) |
| C2 | the implementation report | pre-commit hook |

Push after C2: pre-push runs tiers 2 and 3 and the contract gates, about three
minutes; let it finish. No Go code changes, so no mutation key moves and
`check:drift` has no client change to compare.

## Gaps that travel with this plan

1. **Test prose that points into cut blocks.** Five places (measurements
   above) describe or quote comments the sweep removes. Ticket item 7 allows
   citation lines only, so they stay stale; D19 names them in the report for
   the test-file sweep. Q8.
2. **The proto's `CatchUpHead` doc carries SPEC-007's sentence.** "the highest
   sequence the server has already queued ... 0 means the log was empty" in
   `contract/vtt/v1/commands.proto` is wrong for a projected seat in the same
   way. Comments under `contract/` are SPEC-007's and regenerate through
   `task generate:contract`; out of this ticket, named in the report.
3. **"Its only record."** `cmd/vtt/library_test.go`'s doc block also names the
   carry-forward, by a `docs/superpowers/sdd/progress.md` path that does not
   exist. Neither comment is a record, so the ticket's claim holds in
   substance; the file is out of scope and named in the report.
4. **The candidate list is thin in two directions.** Seven of the ticket's
   eleven candidates are two rules or a how (D5 splits them: B/C/E, L/M/N/O,
   Q/R, S/T, U/V, AA/AB), and the tests hold rules the list omits (D, H, I, J,
   K, P, W, X, Z, AC, AD, AE). The sort after sign-off decides; the ticket is
   not edited.
5. **`Server`'s seam fields** (`buffer`, `noProgress`, `writeTimeout`,
   `pingInterval`, `pingTimeout`, `encodeFrame`, `presence`, `onServeDone`)
   document the connection and are not among the named symbols; they stay,
   `writeTimeout`'s block at seven lines among them, and SPEC-011 names them as
   what `New` sets.
6. **`credentialGone`** is in the caller's brief, not the ticket's list; its
   block describes SPEC-009's rule and stays.
7. **Item 6 is an invariant, not a done that fails today.** D11 holds it at
   every task.
8. **"Closed without a close frame"** is observed by no test: the keepalive
   tests observe the reap through the injected `closeNow` and the departure
   announcement, not the close status a client sees. SPEC-011 states it; no row
   carries it.
9. **The fifth ledger row.** `cmd/vtt/mcp_ruleset_e2e_test.go` moves 24.6 to
   24.4 by any `--write-ledger` run at `b7b5b35`; the ticket's "four files"
   reads as four rows this work lowers and one it carries.
10. **Row count after the sort.** The ticket's "more than 79" holds for any
    N ≥ 1; the report states N.
11. **The provenance sentence.** The ticket wants SPEC-011 to repeat the
    MapTool borrowing; D3 keeps provenance out of the record. Q1.

## Questions for sign-off

1. **Does SPEC-011 name MapTool?** Recommend no (D3): the record states the
   heartbeat's direction as a fact about this system, and the borrowing is the
   rule-9 answer in this plan and the report.
2. **The `composeServer` comment: the race sentences only, or the whole
   block?** Recommend the race sentences only (D4), since item 6 names the
   comment for its `.superpowers/` path and the rest of the block is the
   next `cmd/vtt` sweep's; the alternative is one more file at its ceiling.
3. **Is a source-reading test acceptable evidence for row E?**
   `TestServeNeverClosesAConnectionsOutboundChannel` reads `server.go` with
   comments stripped and guards its own vacuity by checking the declaration
   first. Recommend yes, with the row worded on the what.
4. **Row P, the 3x floor.** Recommend accepting it reworded ("one lost pong
   never reaps a peer") so the test carries the figure; the alternative is to
   refuse it as a measurement and leave the test uncited.
5. **Row S as one row or two?** Recommend one: the per-participant count is
   one thing and the wire test observes both halves.
6. **Do the three deny-set registry tests cite VTT-032?** Recommend no: they
   observe the mechanism, and VTT-032's rule is stated on the wire where
   `server_test.go` already holds it.
7. **Unexported symbols lose their descriptive doc sentences (D8)?** Recommend
   yes, as the identity sweep did; a warning on an unexported symbol stays.
8. **May the five test-prose pointers into cut blocks be re-aimed in this
   change?** Recommend no, per item 7's "citation lines only", and list them
   in the report; if yes, each becomes a pointer to `SPEC-011` or a symbol,
   five lines, and Phase 4b reads them.
9. **One commit for the change, the report separately (D10)?** Recommend yes.
10. **If a swept file lands with every surviving line a true warning, pointer
    or doc sentence and `server.go`'s in-scope blocks still hold one over the
    bound?** Recommend the reading governs: stop, report the block, and let
    the ticket's writer decide, rather than cut a true warning for the bound.
