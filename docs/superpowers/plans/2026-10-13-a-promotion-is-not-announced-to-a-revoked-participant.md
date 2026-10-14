# Plan: a promotion is not announced to a revoked participant

Ticket: `docs/superpowers/specs/2026-10-13-a-promotion-is-not-announced-to-a-revoked-participant-design.md`.
Verified 2026-10-13 by an agent that did not write the ticket, on the tree at
`824e5cd`. Verdict: **Passes with gaps**. The verifier wrote this file and
changed nothing in the repository; every edit it tried was made in a clone of
`824e5cd`. This file is written for
`docs/superpowers/plans/2026-10-13-a-promotion-is-not-announced-to-a-revoked-participant.md`.

Every number below is a measurement at `824e5cd`. None is a target.

## How the ticket was verified

| Check | How settled | Result |
|---|---|---|
| 1. Every path resolves | command | Every file named exists. `handlePromotion`, `announcePromotion`, `announcePresence`, `announceDeparture`, `revoked`, `credentialGone`, `serve`, `enqueueEvents` are declared in `internal/gateway/server.go`; `announceIfPresent`, `announceIfAbsent`, `broadcast`, `joinAndSend` in `internal/gateway/presence.go`; `newRevokeCmd` in `cmd/vtt/revoke.go`; the three tests named in `internal/gateway/server_test.go`. `Lookup` and `Revoke` are methods of `identity.DB`. Both quoted specification sentences and the debt heading were found by `grep`. |
| 2. Done is an observation | command | Item 1's test was built from the ticket's words in a clone and fails on today's tree at its absence assertion: 30 runs of 30, and 10 of 10 under `-race`. Items 4 and 6 and half of item 5 are readings (G2, G3). Item 7 was not run. |
| 3. Each rule is breakable | reading, then a command | The ticket states no new rule. VTT-032's third clause is false on today's tree for this frame, and two edits that put the defect back each red the prototyped test (K1, K2 below). |
| 4. Scope matches the claim | command, then reading | `announceIfPresent` has one caller outside tests, `announcePromotion`, and three in `presence_internal_test.go`. `announcePromotion` has one caller, `handlePromotion`. `tools/mutation-equivalents.txt` holds no entry for `presence.go` or for `server.go` of `internal/gateway`. The ticket's file list is what the prototyped change touched. |
| 5. No decision contradicted silently | reading | SPEC-009 and SPEC-011 state the gap in the present tense and the ticket names both. SPEC-007 (a promotion appends nothing) and VTT-047 stand. ADR-011's decision reads "every presence frame then re-resolves the participant", which the ticket does not cite and brings the code to. |
| 6. Records moved are named and reachable | command, then reading | The section is present and both paths resolve. All fourteen specifications were searched for promotion, presence, announcement and revocation; SPEC-007, SPEC-012, SPEC-013 and SPEC-016 mention the subject and none says who a promotion's frame is sent to. |

## What was run

In a clone of `824e5cd`, with a second untouched copy beside it:

- `go test -count=1 ./internal/gateway/...` on today's tree: green.
- The prototyped test on today's tree: red 30 of 30, red 10 of 10 under
  `-race`, each time with `a revoked participant was told that "Ivo" is
  PRESENCE_STATE_CONNECTED`.
- The same with the promoted participant connecting after the revocation: red
  10 of 10. The revoked connection is told of somebody whose arrival
  `announcePresence` withheld from it.
- A prototype of the change: the new test green 30 of 30, and with
  `TestAPromotionIsAnnouncedToThePromotedPersonThemselves` green 10 of 10 under
  `-race`; the package green; `go vet` and `golangci-lint` over the package,
  `check-doc-owner.py`, `check-comments.py main`, `check-new-prose.py main` and
  `check-requirements-chain.py` clean.
- The breaks in the table further down.

## What the tree showed that the ticket does not say

**F1. A promotion's frame cannot be told from an arrival's.** Both are a
`PresenceChanged` with the participant's id, their display name and
`CONNECTED`. A control connection that joined before the promoted participant
holds their arrival, and a read for "a `PresenceChanged` naming them" is met by
it. Measured: with the promotion's fan-out made to reach nobody, a test whose
control had not read the arrival passed 3 of 3; the same test with the arrival
read first failed 3 of 3. Done 1 says the control "is sent the promotion's
`PresenceChanged`" and not how it is known to be the promotion's. D5 settles
it.

**F2. Neither mutation gate can see the new guard.** The filter is an index
into a map and the resolution is a call. Neither holds a token any of the six
mutators in `MUTATOR_TOKENS` (`tools/check-mutation.py`) rewrites. The
breaks made by hand are the only evidence that the test notices the guard.

**F3. `server.go` and `presence.go` cannot take one more comment line.**
`server.go` holds 203 comment lines in 804 non-blank lines, 25.249% against a
ceiling of 25.3; `presence.go` 32 in 199, 16.080% against 16.1. One comment
line added to either beside the prototyped code was refused by
`check-comments.py main` (25.31 and 16.26). Reworded in place, the gate was
clean. `server_test.go` holds 608 in 2492, 24.398% against 24.4.

**F4. The chain gate does not hold "the test cites VTT-032".** With the
`// VTT-032` line removed from above the new test and the register's row
naming it, `check-requirements-chain.py` stayed green, because
`server_test.go` carries the id above other tests.

**F5. More sentences state the old behaviour than the two the ticket quotes.**
All of them are in files the ticket lists. In SPEC-009, the paragraph
"Promotion is a command" says the frame goes "to every connection through
`announcePromotion`, theirs included". In SPEC-011, the presence paragraph says
`announcePromotion` runs one fan-out "to every connection including the
participant's own". In `server.go`, the comment above `announcePromotion` says
"to every connection, theirs included", and in `presence.go` the comment above
`participantIDs` names the two functions that consume a deny set.

**F6. Done 2 freezes a comment the change makes inexact.** Inside
`TestAPromotionIsAnnouncedToThePromotedPersonThemselves`: "broadcast excludes
nobody for exactly that reason". Item 2 says no line of that test changes.

## How MapTool does this (rule 9)

The ticket's answer was checked in `~/dev/RPTool/maptool`, whose `HEAD` is
`f4b7fef6`. `MapToolServer.bootPlayer` calls `releaseClientConnection`, which
calls `connection.close()` and `router.removeConnection`; `Router`'s two
`broadcastMessage` methods iterate its `clients` map. A booted client is out of
the set every broadcast walks, so MapTool filters nothing. The design is not
borrowed: there the server does the booting, and here `vtt revoke` writes the
identity table from another process (`newRevokeCmd` is the one caller of
`Revoke` outside tests) and nothing tells the gateway.

## Constraints

- `CLAUDE.md` rule 2. `task check` whole, on the tree that is committed. A
  change after a run means it runs again.
- Rule 3. No contract change: the frame is the `PresenceChanged` it is today.
- Rule 8, in every sentence written, this plan's included.
- Rule 10 and SPEC-010, with F3's arithmetic.
- SPEC-007 and VTT-036. A promotion appends nothing, so the repair is not an
  event that wakes the pumps.
- SPEC-011. `fanOut` before `mu`; the name is resolved, the frame built and the
  sends made inside one call of `announceIfPresent`; no identity read under the
  registry's locks.
- VTT-033. `revoked` is used as it stands: it matches `ErrInvalidToken` alone.
- VTT-047, and Done 2: no line of its test changes.
- The process is not edited from this repository.

## Decisions

1. **`announceIfPresent` gains `deny map[string]bool` and skips a denied
   participant's connections where it already collects its targets.** Forced:
   `targets` locks `mu` itself and `announceIfPresent` holds `mu` at that
   point; collecting targets in a second critical section is the gap its own
   comment and SPEC-011 close. Placing the parameter between `participantID`
   and `frame`, as `announceIfAbsent` has it, is a preference.
2. **`deny` is a resolved set, and `announcePromotion` resolves it with
   `s.revoked()` before the call, whether or not the participant turns out to
   be connected.** Forced by Done 4, `revoked`'s comment and SPEC-011. The
   registry holds no identity handle, so with a map parameter the read cannot
   happen inside it; what Phase 4b reads for Done 4 is that nothing on the path
   from `serve`'s read loop through `answerCommand`, `handleCommand` and
   `handlePromotion` holds `fanOut` or `mu`.
3. **`deny` withholds the frame; it does not enter the search that finds the
   promoted participant's name.** A preference: it is the smaller change and
   `announceIfAbsent` consults `deny` for targets only. No test can separate
   the two, since the promoted participant is in `deny` only if revoked between
   `handlePromotion`'s `Lookup` and `revoked`.
4. **One new test, at the wire, and none at the registry.** Measured: the wire
   test reds when `announcePromotion` resolves nothing and when
   `announceIfPresent` ignores what it is handed (K1, K2).
5. **The test's control is a bystander who has read the promoted
   participant's arrival before the promotion is sent.** Forced by F1.
6. **The revoked connection reads every arrival it is owed before the
   revocation, and its absence read is the test's last statement.** Forced:
   `expectNoPresenceWithin` fails on any `PresenceChanged`, and its comment
   says it is single-use per connection.
7. **No net comment line in `server.go` or `presence.go`; the comments of F5
   are reworded in place.** Forced by F3. In the new test, about one comment
   line to three of code at most, no block over six lines.
8. **QA is told which two sentences the ticket repeals.** Forced by the order:
   the specifications change last and QA runs before the commit. Unmarked,
   SPEC-009 tells QA that the promotion's frame "does not re-resolve".
9. **The debt entry's `Closed by` paragraph names the edit that puts the defect
   back.** Forced by the file's own header, which makes an entry a recipe; this
   entry's recipe says nothing puts it back.

## Tasks

**T1. The failing test.** `internal/gateway/server_test.go`.

`TestAPromotionIsNotAnnouncedToARevokedWatcher`, placed after a function's
closing brace, with `// VTT-032` on the line above it. It uses the wire and
`f.ids` only, so it compiles before T2. The sequence the prototype ran:

1. The stranger (the spectator) connects and reads its head and snapshot.
2. The witness (a player) connects; the stranger reads the witness's arrival.
3. The target (the other player) connects; the stranger and the witness each
   read the target's arrival.
4. The DM connects; the stranger reads the DM's arrival.
5. `f.ids.Revoke` on the stranger.
6. The DM sends `promote_participant` for the target and reads an ok result.
7. The witness reads a `PresenceChanged` naming the target.
8. `expectNoPresenceWithin` on the stranger.

Done when: on the tree without T2 it fails at step 8 and not before, on every
run of `go test -count=10`; no other line of the file differs.

The bound in step 8 is not what catches the defect. `handlePromotion` calls
`announcePromotion` before it returns its result, so the frame is in the
stranger's queue before the DM's result is.

**T2. The change.** `internal/gateway/presence.go`,
`internal/gateway/server.go`, `internal/gateway/presence_internal_test.go`.

Decisions 1, 2, 3 and 7. Done when: the new test passes on every run of
`go test -count=10`, and under `-race`;
`TestAPromotionIsAnnouncedToThePromotedPersonThemselves` passes with an empty
diff inside it; `go test -count=1 ./internal/gateway/...` passes; the diff of
`presence_internal_test.go` is the three calls of `announceIfPresent`, each
gaining a `nil`; F5's two code comments are true of the new code;
`python3 tools/check-comments.py main` is clean.

**T3. The breaks.** No file changes. Each row is applied, the edit confirmed
by `grep`, the named test watched red, and the edit undone by its inverse
edit and confirmed by `grep` again. Never by `git checkout`: the tree is
uncommitted.

| | The edit | What reds |
|---|---|---|
| K1 | `announcePromotion` resolves nothing: `deny` is a nil map. Passing a bare `nil` beside an unused `deny` does not compile. | The new test, at step 8. |
| K2 | `announceIfPresent` ignores `deny`. | The new test, at step 8. |
| K3 | `announceIfPresent` skips every connection. | The new test at step 7, after five seconds, and `TestAPromotionIsAnnouncedToThePromotedPersonThemselves`. |

K3 is run with `-run` naming those two tests:
`TestAnnounceIfPresentSaysNothingAboutSomebodyWhoHasLEFT` receives from a
channel with no default and hangs under it until the test binary's timeout.
After the last repair the package is green, which is the other direction.

**T4. The records that land with the code.** `docs/requirements.md`,
`docs/verification-debt.md`.

VTT-032's row names the new test as a fourth evidence entry. The entry headed
2026-09-24 gains a `Closed by` paragraph naming the test, its file, K1 and K2
as what reds it, and the day they were observed; no other paragraph of the file
is rewrapped. Done when `python3 tools/check-requirements-chain.py .` is clean
and `grep -B1 '^func TestAPromotionIsNotAnnouncedToARevokedWatcher'` prints
`// VTT-032` (F4).

**T5. The gate.** `task check`, whole. Neither `tools/comment-ceilings.txt`
nor `tools/mutation-equivalents.txt` changed under the prototype. If
`check:comments` asks for `--write-ledger`, the ledger is a file the ticket
does not list, and that is said in the report.

**T6. Phase 4a.** A QA file under `internal/gateway`, if QA derives tests.

The brief carries Done 1, VTT-032's row, the two specifications with decision
8's two sentences marked, the `go doc` surface, F1 as a fact about the
contract, `.go-arch-lint.yml`'s rule for the package, and SPEC-010's bounds: a
new file with no ledger row stays at or under 25.0% comment. Each QA test is
run under K1 and K2. Adjudications go in the report under their own heading.

**T7. Phase 4b.** One reading review. It holds Done 4 by decision 2's reading,
and reads F6.

**T8. The commit.** T1, T2, T4, QA's tests, the ticket and this plan, with a
line naming K1, K2 and K3 and the test that spoke for each.

**T9. The implementation report.** `docs/reports/`, its own commit, answering
Done 1 to 7 in the ticket's numbering.

**T10. The specifications.** `docs/specifications/009-identity-and-joining.md`,
`docs/specifications/011-the-connection.md`, their own commit, as `5cb42fa`
carried three.

In SPEC-009: the sentence beginning "The promotion nudge `announcePromotion`"
and the list before it of where `revoked` is resolved; and F5's sentence in
"Promotion is a command". In SPEC-011: F5's sentence, and the clause
"`announcePromotion` passes no deny set" with its pointer to the debt. Neither
`Requirements` list changes: VTT-032 is in SPEC-009's already and the work
earns no id. Done when
`grep -n 'without consulting\|passes no deny set\|2026-09-24'` over the two
files prints nothing, every sentence on who a promotion's frame reaches names
the revoked participant as skipped, and `task check` is green.

## Gaps carried

- **G1.** `task check` was not run by the verifier. Done 7 is unobserved.
- **G2.** Done 4 is a reading and no break exists for it. SPEC-011 states the
  rule and no row of `docs/requirements.md` holds it.
- **G3.** Done 6, and the citation half of Done 5, are held by `grep` and a
  reader. No gate reads `docs/verification-debt.md`, and F4.
- **G4.** F2: the guard is outside the mutation gate.
- **G5.** F6.
- **G6.** The ticket's own two: whether a table has met the defect, and whether
  an announcement that skips a revoked connection should close it.

## Questions for Patrik

- **Q1.** Does "no identity read happens under the registry's locks" get a row
  marked `READING`, or stay a sentence of SPEC-011 (G2)? The ticket says it
  puts no rule on the system.
- **Q2.** Does F6's comment stand as it is, which Done 2 requires, or is Done 2
  narrowed to the test's assertions?

## Amended by the ticket's writer

The ticket was amended after this verification, and the amended text was not
verified again.

- F1: Done 1 now says the control has read the promoted participant's arrival
  before the promotion, and why.
- F4: Done 5 now says which half the chain gate holds and names the `grep`
  that shows the citation.
- F5: the problem now names the two further sentences and the two comments.
- The list of where the gateway resolves a participant now has the backlog and
  `handlePromotion`'s own lookup of its target, and the enumeration of senders
  names `send`.
- F6 and Q2: Done 2 is narrowed to the test's statements, and the comment is
  reworded with the change. This is the writer's proposal; the owner's answer
  is recorded below.

The verifier went further than check 2 asks: it built the whole change in its
clone. That prototype is not the implementation. T1 and T2 are done in the
repository, test first, and Phase 4a's agent is given neither the prototype
nor the diff.

The sort had one candidate, which the ticket does not state and Q1 raises: "no
identity read happens under the registry's locks". Refused as a rule of how
and not of what: it names a locking order, and what it protects, that no
joiner and no other fan-out waits on the identity store, has no observation
named for it here. It stays a sentence of SPEC-011, held for this change by Phase
4b's reading (Done 4). The owner's answer to Q1 is recorded below.

## Answered 2026-10-13

- The owner approved the ticket and this plan.
- Q1: no row. The sentence stays SPEC-011's, and Done 4 is held by Phase 4b's
  reading.
- Q2: the comment is reworded with the change. Done 2 stands as amended: no
  statement of that test changes.

## After Phase 4a and the reading review

- Check 6's list above is incomplete: SPEC-019 also mentions revocation, as
  the record that `vtt revoke` runs beside a `vtt serve` on the same
  directory. It says nothing about who a promotion's frame reaches, so the
  check's conclusion stands.
- The sentence on what the locking rule protects said, until the reading
  review, that an announcement is not delayed by the identity store. That was
  false: every announcement waits on `revoked`. It now says what the rule
  protects.
- T2's "an empty diff inside it" and the constraint "no line of its test
  changes" are overruled by Q2: one comment line inside
  `TestAPromotionIsAnnouncedToThePromotedPersonThemselves` changed.
- QA cited two of its tests as `// SPEC-009`, finding no row for their rules.
  The owner ruled on 2026-10-14 that both earn ids; the dispenser allocated
  VTT-309 and VTT-310, and the tests cite them. SPEC-009's Requirements list
  gains both in T10.
- `tools/comment-ceilings.txt` changes after all: QA's file is given its row
  by `--write-ledger`, as every earlier QA file has one.
