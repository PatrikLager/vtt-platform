# The connection has a record: the change

**Ticket:** `docs/superpowers/specs/2026-09-27-the-connection-has-a-record-design.md`,
amended at sign-off and during the work on two points its gate forced (items 6
and 7 of "What it touches", below under Deviations).
**Plan:** `docs/superpowers/plans/2026-09-27-the-connection-has-a-record.md`,
verified by `verify-ticket` (Passes with gaps); its ten sign-off questions were
answered at sign-off on 2026-09-27, and this report is where the answers are
recorded: 1 to 7, 9 and 10 as the plan proposed; 8 yes, against the plan's
recommendation.
**Last commit that changes code:** `0565d19`, on `b7b5b35`, `main` at the time.
Every code reference below is to that tree.

## The period, in commits

    git log --oneline b7b5b35..0565d19

    0565d19 The connection has a record: SPEC-011, VTT-080 to VTT-110

`git diff --stat b7b5b35..0565d19`: 20 files changed, 1551 insertions(+), 1092 deletions(-).

The gate: `task check`, whole, over the tree of `0565d19` before it was
committed, on a quiet machine: exit 0, no step failed, and the check steps'
own verdict lines read `check:comments` `clean`,
`check:requirements-chain` reading 110 rows, `check:doc-owner` 79 files,
`check:new-prose` 226 added lines clean, `check:coverage` 20 packages at or
above their floors, `check:mutation` fourteen packages with zero unadjudicated
survivors, `check:ts-mutation` zero unadjudicated. Before it: `gofmt`, `go
vet`, `task lint`, the gateway package three times as the tree moved, the four
local gates named above, the token instrument, and the breaks; then the
pre-commit hook's nine checks.

Phase 4a was skipped: the change moves prose and changes no production line,
and the one test assertion it tightens is the reading review's finding.

## Done looks like, answered

1. `[x]` `docs/specifications/011-the-connection.md` exists with SPEC-007's
   five headings (`grep -c '^## '` prints 5 for both files): Status,
   Principles served, How it works, Consequences, Requirements. The reading
   review (Phase 4b, `pr-review-toolkit:code-reviewer` on `fable`) read its
   sentences against the symbols they name, by command; the sentences it
   refuted were rewritten before the commit (below, Deviations), and the one
   it could not verify is under "What could not be established". The review
   is a reading, recorded for the commit gate, and nothing in the tree
   restates its table.
2. `[x]` SPEC-007's `CatchUpHead` paragraph says what the seat's catch-up
   delivers, for the DM and the agent and for a projected seat, and points at
   SPEC-011, which names `seat.catchUp`. The first version, the plan's D9,
   said "the log's head for the DM and the agent"; the review showed the
   store answers `after` itself when the log holds nothing newer
   (`Store.SubscribeWithNoProgressTimeout` returns `sub.lastSeq`, which starts
   at `afterSeq`), so the paragraph says that.
3. `[x]` `docs/verification-debt.md` holds the shutdown race as an entry under
   "Open debt", the last at `0565d19`, and `grep -c 'superpowers' cmd/vtt/serve_compose.go`
   prints 0; the pointer is the two-line warning beside `closeFn` in
   `composeServer`, not in the doc comment (Deviations).
4. `[x]` Thirty-one rows, VTT-080 to VTT-110, each with the tests in its
   evidence cell citing it; `task check:requirements-chain` prints `110 rows,
   193 test files, 5 specifications; every citation resolves and every row's
   evidence holds`. No row of the thirty-one is OPEN.
5. `[x]` `python3 tools/check-comments.py --report` prints `banned 0` and
   `blocks>6 0` for `keepalive.go`, `presence.go` and `codec.go`; that every
   surviving block in the three files and in `server.go`'s swept region is a
   warning, a pointer or an exported symbol's doc sentence is the reading
   review's verdict (VTT-051), a reading and not a check; none is over six
   lines. `server.go`'s remaining blocks are
   untouched and named below under "The sweep". The ledger rows moved in the
   same commit, `--write-ledger` run last.
6. `[x]` The go/scanner token stream, comments dropped, of `keepalive.go`,
   `presence.go`, `codec.go`, `server.go` and `cmd/vtt/serve_compose.go` is
   identical to `b7b5b35`'s (392, 1166, 130, 4596 and 691 tokens; the
   instrument is the one plan D11 names, a `go/scanner` dump compared with
   `cmp` against `git show b7b5b35:<file>`); so is
   `cmd/vtt/serve.go`'s (396), touched beyond the ticket's list. Two test
   files differ: `keepalive_internal_test.go` in two string-literal tokens of
   one failure message, and `server_test.go` in the assertion the review
   tightened (Deviations).
7. `[x]` `task check` whole, exit 0, above.

## What the rules became

| Rule, as the ticket words it | Became |
|---|---|
| A connection is upgraded only for a credential that verifies; a bad or revoked token is answered with HTTP 401 and no upgrade | VTT-080 |
| Every frame a connection sends is written by one goroutine, in the order results, events and presence frames were queued | VTT-081 (the order); "one goroutine" refused as a row, kept as SPEC-011 prose |
| A connection's outbound queue is never closed, and teardown writes what is queued before the writer stops | VTT-084 (a frame handed over during teardown is dropped and the sender unharmed; "never closed" is how), VTT-082 (teardown drains), and VTT-083 from the test beside them (no write after a failed one) |
| A malformed or oversized inbound frame closes its own connection and no other | VTT-085 |
| The catch-up head a connection opens with is the last sequence that seat will be sent, which for a projected seat can be below the log's head | VTT-086; VTT-087 from the test beside it (a head that cannot be encoded refuses the connection) |
| A connection silent for the ping interval is pinged, and one that answers no pong within the pong budget is closed without a close frame; no ping is sent while the connection's writer is mid-frame | VTT-091, VTT-092, VTT-094; VTT-093 from the test beside them (a ping that could not be sent is no verdict); VTT-095, the ratio the constants keep, worded on what its test observes |
| A participant's arrival is announced to the table after their own connection's pump is running, and never before their snapshot | VTT-096, VTT-097 |
| A participant is announced gone only when their last connection ends and only if no connection of theirs exists when the announcement is sent | VTT-098, VTT-099 |
| A presence fan-out waits at most the send budget for each connection and then drops that frame for that connection | VTT-100, VTT-101 |
| A client force-closed for not reading is announced gone like a clean departure | VTT-104 |
| The static client is served at `/` without authentication and never shadows an API route | VTT-106, VTT-107; VTT-108 and VTT-109 from the tests beside them |

Rows the sort took from the problem paragraph, from tests and from the
presence ticket's 2026-08-11 amendment rather than from the list: VTT-088 (an envelope that cannot be encoded ends the
connection and no later envelope of its batch is sent), VTT-089 and VTT-090
(a connection that stops taking frames is closed, and closing it disturbs no
other), VTT-102 and VTT-103 (a joiner does not wait for another's fan-out;
announcements reach every connection in one order), VTT-105 (an arrival or
departure reaches a participant's other connections and not the one it is
about), VTT-110 (`GET /healthz` answers 200).

Refused, per the plan's sort: the registry and pinger internals
(`TestLeaveIsIdempotent`, `TestLeaveOfAnUnknownConnectionIsNotADeparture`,
`TestLeaveStopsDelivery`, `TestAFanOutAbandonsAConnectionThatLeftMidWalk`,
`TestACancelledConnectionStopsItsOwnPinger`), a how each; the three deny-set
tests, the mechanism under VTT-032, whose rule is stated on the wire (sign-off
question 6); the four codec round-trips, SPEC-007's wire convention; the
two-clients delivery test, the visibility record's; the authorization and
identity tests, SPEC-009's;
`TestDescribeBlockageRewritesTheTwoNonProseReasonsAndPassesTheRestThrough` and
`TestABlockedMoveRefusalStopsGrowingWithTheSceneryKind`, not the connection.

## The sweep

Comment lines over non-blank lines, counted by `tools/check-comments.py`'s
own `measure` at `b7b5b35` and at `0565d19`:

| File | Before | After | Blocks over the bound |
|---|---|---|---|
| `internal/gateway/keepalive.go` | 221 / 286 | 19 / 84 | 6 to 0 |
| `internal/gateway/presence.go` | 224 / 391 | 32 / 199 | 11 to 0 |
| `internal/gateway/codec.go` | 12 / 32 | 4 / 24 | 1 to 0 |
| `internal/gateway/server.go` | 1098 / 1694 | 715 / 1311 | 61 to 34 |
| `cmd/vtt/serve_compose.go` | 262 / 364 | 240 / 342 | 9 to 9 |

The four gateway files: 1,555 of 2,403 to 770 of 1,618. Ledger rows, old to
new: `keepalive.go` 77.3 to 22.7, `presence.go` 57.3 to 16.1, `codec.go` 37.5
to 16.7, `server.go` 64.9 to 54.6, `serve_compose.go` 72.0 to 70.2,
`keepalive_internal_test.go` 41.1 to 29.7, `server_internal_test.go` 37.6 to
37.4, `server_test.go` 28.8 to 27.8, and `cmd/vtt/mcp_ruleset_e2e_test.go`
24.6 to 24.4, a file whose share was already below its row on `main`.

`server.go`'s thirty-four remaining blocks over the bound, by the symbol each
sits on or in, are the next sweeps': the `Server` struct's field docs
(seven: `writeTimeout`, `ruleset`, `adventures`, `adventureGuides`, `maps`,
`artDir`, `cellPx`), `WithRuleset`, `WithAdventures`, `WithMaps`,
`WithMapsDir`, `WithArtDir`, `WithCellPx`, `describeBlockage`,
`answerCommand`, `authorize`, `handleSetViewpoint`, `handleCommand` (seven:
its doc and six command-conversion blocks in its body), `announcePresence`,
`announceDeparture` (two: its doc and the `revoked` block in its body),
`revoked`, `announcePromotion`, `handleRemoveActor`, `handleJoinDoor` (one, in
its body), `handlePromotion` (two), `credentialGone`. `cmd/vtt/serve_compose.go`
keeps its nine blocks over the bound, one of them `composeServer`'s doc, for
the `cmd/vtt` sweep.

## The breaks

| Break, one edit in a scratch clone of the changed tree | Red |
|---|---|
| B1 a `// VTT-999` line above `TestLeaveIsIdempotent` | `check:requirements-chain`: `presence_internal_test.go cites VTT-999 and no row in docs/requirements.md defines it` |
| B2 VTT-084's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `row VTT-084: internal/gateway/codec_test.go does not carry the id, so the link walks one way only` |
| B3 VTT-999 on SPEC-011's Requirements line | `docs/specifications/011-the-connection.md cites VTT-999 and no row ... defines it (specification citation)` |
| B4 a `// SPEC-011` line inside `keepAlive`, after the ledger was written | `check:comments: internal/gateway/keepalive.go: comment share 23.53 is above its ceiling 22.7 and this change added a comment line to it (SPEC-010)` |
| B5 `if busy()` made `if !busy()` | the token instrument prints `DIFFERS` |
| B6 the `.superpowers` path back in `composeServer`'s doc | `grep -c` prints 1 |
| B7 the malformed-frame arm of `serve` made `continue` | `TestMalformedFrameClosesOnlyThatConnection`: `want badConn closed with StatusPolicyViolation after a malformed frame, got failed to get reader: context deadline exceeded` |

B7 is the review's: before the assertion was tightened the same edit left the
test green, because a read that runs out its three-second context also
returns an error.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Sign-off question 4: row P worded "one lost pong never reaps a peer", so the test carries the figure. | VTT-095 reads "The pong budget is at least three ping intervals." | The review showed the wording false of the code: `keepAlive` runs one ping at a time and `conn.Ping` waits `pingTimeout` for that ping's pong, so one pong that never arrives reaps the peer, which `TestAPongThatNeverComesReapsThePeer` proves with one ping. The test in the evidence cell pins the ratio of the two constants and nothing else, so the row says that. |
| Sign-off question 8, yes: the five test-prose pointers into cut blocks re-aimed, five lines. | The five comment blocks holding them were sorted to the bound: the frameQueue header and `TestOversizedFrameClosesConnectionMaxLegalPayloadWorks`'s doc in `server_test.go`, the "wedge must have been real" block in `server_internal_test.go`, the file header with `TestAPongThatNeverComesReapsThePeer`'s doc and `TestThePongBudgetStaysAtLeastThreeIntervals`'s doc in `keepalive_internal_test.go`. | `check:comments` refuses a line added to a block of more than six lines, and a re-aimed line is an added line; each pointer sat in such a block. |
| Sign-off question 2: the race sentences of `composeServer`'s comment only, the rest of the block untouched. | The race paragraph was deleted whole and the two-line warning placed inside the function, beside `closeFn`. | The same refusal: `composeServer`'s doc is one block, 121 comment lines at `b7b5b35`, and a warning added to it is a line added to a block over the bound. A deletion adds nothing. |
| Ticket item 7 of "What it touches": test files, citation lines only. | Also the five blocks above, one word in a body comment of `TestServeNeverClosesAConnectionsOutboundChannel` ("quotes" became "quoted", since the shutdown comment no longer quotes `close(outCh)`), one failure-message string in `TestThePongBudgetStaysAtLeastThreeIntervals` ("the doc comment argues" became "SPEC-011 holds", since the doc comment is gone), and one assertion in `TestMalformedFrameClosesOnlyThatConnection`, which now demands `StatusPolicyViolation`. | The blocks: above. The word and the string: prose naming a comment that no longer exists is false. The assertion: the review's Must, VTT-085's malformed half could not go red (B7). |
| Plan D12: five ledger rows, seven at most. | Nine. | `--write-ledger` lowers a row on any drop, not only past the band (`min(old, now)` in `tools/check-comments.py`): the block sort lowered the three test files' shares by 11.4, 1.0 and 0.2 points, and `mcp_ruleset_e2e_test.go`'s share was already below its row on `main`. |
| Plan Task 11: the commit lists the named files and nothing else. | `cmd/vtt/serve.go` is in it: one trailing comment on the deferred `closeFn` line, re-aimed from "composeServer's hijack-contract note" to `docs/verification-debt.md`. | The note it pointed at was the deleted paragraph. A trailing comment on a code line is not a comment line to the gate, so this one could move; the second pointer, in the signal arm's block, could not (below). |
| Plan D9: SPEC-007 says "the log's head for the DM and the agent". | It says the last sequence queued as catch-up, or `after` itself when the log holds nothing newer. | The review, by reading `Store.SubscribeWithNoProgressTimeout`: `sub.lastSeq` starts at `afterSeq` and moves only when the history read is non-empty, so a DM dialling above the head is told `after`. SPEC-011's sentence moved the same way. |
| Task 1: SPEC-011 as the plan's D1 sections. | Three sentences arrived later: the encode seam as a `Server` field (from `codec.go`'s cut block, corrected by the review to exclude the `CommandResult`, which the read loop marshals with `EncodeFrame` directly), a reap coinciding with a reasoned `Close` (from `keepalive.go`'s cut block), and the library's own close of a ping that wins the frame lock and stalls (a fact the cut block carried and nothing else did, found by the review). Four more were corrected: the no-progress timer arms only while a frame waits; the deferred `leavePresence` backstops only a panic; `announceIfAbsent`'s re-check is under `mu` and a later reconnect is ordered by `fanOut`; `revoked` is resolved by `announcePresence` and `announceDeparture`, not `serve`. | Each was a claim the review refuted by command; the sweep is where the facts surfaced. |
| Plan D8: the warning at `keepAlive` as D8 writes it, and one at `handleWS`. | Each opens with the symbol's own name ("keepAlive must run on its own goroutine", "handleWS must verify the token before websocket.Accept"). | `check:doc-owner` reads the first word of a block above a function as the function it describes, and refused "Run" and "Verify", both functions elsewhere. |
| Plan D8: the busy-skip warning says a ping into a busy writer "reaps a healthy client". | It says the ping loses the library's five-second control-frame race, and that one which wins the lock on a stalled socket is closed by the library itself. | The review: since the verdict fix a failed send returns nil and reaps nobody; the reap belonged to the history the block also carried. The `shutdown` warning lost "the departure goes out on this connection too" for the same reason: `leave` removes the connection before `announceIfAbsent` reads its targets. |

## What could not be established

- The busy skip's wiring in `serve` is unpinned: VTT-094's three tests drive
  `keepAlive`, `writeActivity` and `stampedWrite` in isolation, and replacing
  `stampedWrite(&activity, ...)` in `serve` with the bare closure, or passing
  a predicate that always answers false instead of `activity.busy`, leaves
  all three green. The warning at `stampedWrite` says so; the gap is
  recorded in `docs/verification-debt.md` by this report's commit.
- VTT-084's evidence is a source-text assertion (`strings.Contains` on
  `server.go` with comments stripped); it observes that `close(outCh)` is
  absent, not that a frame is dropped harmlessly. VTT-109's test runs against
  a `fstest.MapFS`, so its traversal shapes cannot reach a filesystem whatever
  the handler does, and the mux normalises `/../go.mod` before the handler
  runs; it goes red only on a swap to a real directory. Both breakable, both
  weak; left as accepted with this note.
- SPEC-011's sentence that the WebSocket stack answers a ping below the
  application, with no client code taking part, is true of coder/websocket's
  own client (`handleControl` in its `Read`) and was not verified for a
  browser.
- `cmd/vtt/serve.go`'s signal arm still says "see composeServer's doc
  comment" about the hijack contract; the paragraph it points at is gone, and
  the pointer sits in a block over the bound that this change may not add a
  line to. The `cmd/vtt` sweep's.
- `TestNoPingGoesOutWhileTheWriterIsBusy`'s failure message says the
  five-second failure is one "this loop reads as peer death", which has been
  false since the verdict fix; untouched here, the test-prose sweep's.
- VTT-092 and VTT-098 each pair a rule with its control in one sentence
  (closed when no pong, not closed when every ping is answered; a second
  connection is not an arrival and closing one of two is not a departure).
  Sign-off question 5 chose one row for the second; the first followed it.

## What was deliberately left out, and where it went

- `server.go`'s blocks outside the connection, by symbol above: the command
  path, the `With*` configuration and the announcement helpers are the next
  gateway tickets' (the HTTP read surface, the authorization table, maps and
  art, seat and perch, projection, and `server.go` last).
- `composeServer`'s doc comment beyond the race paragraph: the `cmd/vtt`
  sweep's.
- MapTool's 20-second heartbeat against a one-minute socket timeout, the
  rule-9 answer the cut `gatewayPingInterval` block carried: plan D3 keeps
  provenance out of SPEC-011; it survives in the plan and at
  `store.SubscriberNoProgressTimeout`'s comment.
- `announcePromotion` passing no deny set: the 2026-09-24 entry in
  `docs/verification-debt.md`, unchanged; SPEC-011 names it.
- The keepalive's cost figures, the presence lock history and every measured
  race in the cut blocks: gone, on purpose; this report does not restate them.
- No production line under `cmd/vtt/`, `internal/` or `client/src` changed.

## The sort

Thirty-one candidates accepted as VTT-080 to VTT-110; about fifteen refused,
above.
