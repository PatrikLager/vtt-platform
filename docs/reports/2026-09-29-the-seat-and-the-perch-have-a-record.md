# The seat and the perch have a record: the change

**Ticket:** `docs/superpowers/specs/2026-09-29-the-seat-and-the-perch-have-a-record-design.md`,
amended at sign-off on the ten points its verification found: the problem
paragraph names what SPEC-011, SPEC-013, SPEC-009 and SPEC-007 already carry,
the three SPEC-013 sentences that move, and a second false comment
(`projected`'s "changes no byte on any wire"); Done items 2, 5 and 6 cover that
comment, every touched test file and the three sentences; three candidate
rules were reworded (the unknown role, "whoever controls it", "the world the
seat has been shown"); "What it touches" gained `authz_test.go`,
`keystone_test.go` and the debt file and lost `project_test.go`; and the open
question on the fold-failure arm was answered.
**Plan:** `docs/superpowers/plans/2026-09-29-the-seat-and-the-perch-have-a-record.md`,
verified by `verify-ticket` (Passes with gaps). Its sixteen sign-off questions
were answered on 2026-09-29, all sixteen as the plan proposed: one record
despite the "and" in its name; no other project named; G, O, P and the
projection rows refused; `#nosec` kept, shortened, directly above its call;
the `authz_test.go` perch test cited; `keystone_test.go`'s two small blocks
re-aimed; `project.go`'s stale pointer left; K, S and U OPEN; L and Q
accepted, Q with its rate; R accepted; a debt entry for the fold-failure arm;
one commit and the report apart; the reading governs over the bound. The
owner also ruled, on the two-writer finding, a debt entry now and a ticket of
its own after the gateway sweep; and, on the reading review's findings, three
debt entries rather than one.
**Last commit of the change (no code line changed):** `7be685a`, on `74c547f`,
`main` at the time. Every code reference below is to that tree.

## The period, in commits

    git log --oneline 74c547f..7be685a

    7be685a The seat and the perch have a record: SPEC-015, VTT-176 to VTT-193

`git diff --stat 74c547f..7be685a`: 13 files changed, 1727 insertions(+), 459 deletions(-).

The gate: `task check`, whole, over the tree of `7be685a` before it was
committed: exit 0, no step failed, and the check steps' own verdict lines
read `check:comments` `clean`, `check:requirements-chain` 193 rows,
`check:doc-owner` 79 files, `check:new-prose` 89 added lines clean,
`check:coverage` 20 packages at or above their floors, `check:no-pack` clean,
`check:mutation` fourteen packages with zero unadjudicated survivors,
`check:ts-mutation` 2882 mutants, 2783 killed, 29 timed out and counted as
killed by that gate's own rule, 70 survivors all adjudicated, zero
unadjudicated, verified from its stored report because no client input had
changed. It was the second run over that tree; the first is under Deviations.
Before it: `gofmt`, `go vet`, `task lint`, the gateway package's tests,
`check:comments`, `check:doc-owner`, `check:requirements-chain`,
`check:new-prose`, both mutation self-tests, the token instrument, and the
breaks; then the pre-commit hook's nine checks.

Phase 4a was skipped: the change moves prose and changes no code line, and
what can be wrong in it is a sentence, which the reading review holds.

**Rule 9.** MapTool decides what a client sees on the client, from a whole
campaign every client holds: `MapToolServer` sends a joiner the campaign with
every zone, token and GM layer, relays every change to everyone, and
`ServerMessageHandler` checks no role or ownership; vision is computed per
client (`ZoneView`, `FogUtil`) and applied when drawing. Its eyes are chosen
by `ZoneViewModel.makePlayerView`: selected tokens the player owns, else the
player's own or every PC token with sight, else a global view; there is no
spectator role, and the nearest thing to a perch is the GM's local "Show As
Player". Remembered fog is kept per token and computed by the moving client.
Borrowed: a view as a role plus a set of eye tokens, and shared party vision
as the analogue of a spectator riding a party member. Refused: the
whole-campaign distribution with a client-side clip, which rule 9 names; a
fallback to some tokens when none is chosen, which here would be a shoulder
the server picked for someone who asked for none (`viewerFor` opens on
nobody); eyes picked by client selection under a lax ownership check
(`MayPerch` binds a perch, on the server, to an actor whose kind is party
member); memory merged by selection; and movement checked against remembered
area, where `canSee` asks what is seen now. SPEC-015 names no other project.

## Done looks like, answered

1. `[x]` `docs/specifications/015-the-seat-and-the-perch.md` exists with
   SPEC-007's five headings (`grep -c '^## '` prints 5): which seats are
   projected; how a projected seat is fed, against which state, and what it
   drops; that a failed fold withholds the event and every later one; the
   viewpoint a connection opens with; who may perch, on whom, and the one
   refusal; how a perch travels and is applied; `canSee`; a projected seat's
   catch-up; and what it does not decide, pointing at SPEC-011, SPEC-013,
   SPEC-007, SPEC-009 and `project.go`. The reading review (Phase 4b,
   `pr-review-toolkit:code-reviewer` on `opus`) read every sentence against
   its symbol, by command and by probe; the sentences it refuted were
   rewritten before the commit (Deviations).
2. `[x]` `grep -c 'campaign.Append is the only writer'
   internal/gateway/seat.go` and `grep -c 'changes no byte'
   internal/gateway/seat.go` print 0 and 0; at `74c547f` each printed 1.
3. `[x]` Eighteen rows, VTT-176 to VTT-193: fifteen cited by the tests in
   their evidence cells, and VTT-185, VTT-191 and VTT-193 OPEN, with no
   citation line, as the report names below. `python3
   tools/check-requirements-chain.py .` prints `193 rows, 193 test files, 9
   specifications; every citation resolves and every row's evidence holds`.
4. `[x]` `python3 tools/check-comments.py --report` prints `banned 0` and
   `blocks>6 0` for `seat.go` and `viewpoint.go`. That every surviving block
   in them is a warning, a pointer or `MayPerch`'s doc sentence is the reading
   review's verdict (VTT-051). The ledger rows moved in the same commit.
5. `[x]` The go/scanner token stream, comments dropped, of `seat.go` (824),
   `viewpoint.go` (110), `viewpoint_test.go` (630),
   `viewpoint_internal_test.go` (2192), `server_visibility_test.go` (6280),
   `authz_test.go` (6622) and `keystone_test.go` (4593) is identical to
   `74c547f`'s.
6. `[x]` SPEC-013's three sentences name SPEC-015: `MayPerch`'s rule in "Two
   checks for every role", `canSee` and `viewerFor` in the move gate's
   paragraph, and the perch's rule and what a seat is sent in "What this
   record does not decide".
7. `[x]` `task check` whole, exit 0, above.

## What the rules became

| Rule, as the amended ticket words it | Became |
|---|---|
| Only the DM and the agent receive the log unfiltered; every other role is projected | VTT-176 (the DM and the agent), VTT-177 (a player and a spectator); an unparseable role is `identity.Verify`'s refusal and SPEC-015 prose |
| A projected seat that reconnects is sent exactly what it missed | VTT-178 (what left its view), VTT-179 (no frame of an event at or below the cursor) |
| A spectator watches nobody until they name a shoulder | VTT-180 |
| A spectator may perch only on a party member; controlling an actor does not make it one | VTT-181; a party member nobody controls being perchable is unobserved and stays SPEC-015 prose |
| Only a spectator perches | refused: VTT-139, VTT-140 and VTT-141 hold it where it is reachable; `MayPerch`'s own arm is SPEC-015 prose, `TestOnlyASpectatorRidesAShoulder` uncited |
| A perch refusal reads the same whether the named actor exists or not | VTT-182 |
| Naming no actor leaves the shoulder and is allowed | VTT-183 |
| A perch appends nothing to the log | VTT-184 |
| A perch is judged against the state after the last event the seat received, never the campaign's head | VTT-185, OPEN, reworded at review to "the state its seat last folded" |
| A perch's frames carry no sequence, and a resume cursor does not filter them | VTT-186, VTT-187 |
| A burst of hops ends on the last shoulder, and a shoulder the burst flew past is restored by hopping back to it | VTT-188 for the burst; the restore is `reperch`'s, the projection's, and its test stays uncited |
| Hopping while the table is busy keeps one order on the wire and stalls no other participant's command | VTT-189 for the stall (its edit, `set` made blocking and `wake` unbuffered, red in 10 of 12 runs at verification, the base in 0 of 12); the order is SPEC-011's single producer and refused as a row |

Four rows came from the plan's sort rather than the ticket's list: VTT-190, a
projected seat judges each event against the state it produced, never the
head; VTT-191, OPEN, a seat is sent nothing for an event whose fold fails;
VTT-192, a perch is sent at once; VTT-193, OPEN, a refused perch leaves the
watcher where they were.

## The sweep

Comment lines over non-blank lines, counted by `tools/check-comments.py`'s
own `measure` with citation lines set aside, at `74c547f` and at `7be685a`:

| File | Before | After | Banned | Blocks over the bound |
|---|---|---|---|---|
| `internal/gateway/seat.go` | 311 / 430 | 50 / 169 | 17 to 0 | 11 to 0 |
| `internal/gateway/viewpoint.go` | 47 / 68 | 6 / 27 | 8 to 0 | 1 to 0 |
| `internal/gateway/viewpoint_internal_test.go` | 167 / 414 | 98 / 345 | 11 to 8 | 7 to 4 |
| `internal/gateway/keystone_test.go` | 554 / 1188 | 548 / 1182 | 34 to 34 | 19 to 18 |

`viewpoint_test.go`, `server_visibility_test.go` and `authz_test.go` gained
citation lines only (five, ten and one; nineteen with
`viewpoint_internal_test.go`'s three). Ledger rows, old to new:
`keystone_test.go` 46.7 to 46.4, `seat.go` 72.4 to 29.6, `viewpoint.go` 69.2
to 22.3, `viewpoint_internal_test.go` 40.4 to 28.5: the four rows plan D12
named, and no other.

What the kept blocks are. `seat.go`: warnings above `viewerFor`, `projected`,
the `seat` type, the `perchBox` type, `set`, `take`, `receive`, `pastResume`,
`perch` and `canSee`; warnings on the `pr`, `resume`, `received` and `wake`
fields and a pointer on `world`; pointer sentences above `newSeat`,
`subscribeFrom` and `catchUp`; inside `receive`, the state warning, the
fold-failure warning with its `#nosec G706` directive directly above
`slog.Error`, and the `world` warning; inside `pastResume`, `perch` and
`catchUp`, one warning each on the comparison, the unprojected arm, the closed
channel, the head and the stop. `set`'s trailing `default:` comment went.
`viewpoint.go`: `MayPerch`'s doc sentence and a warning on each of its two
arms.

Facts the cut blocks held that SPEC-015 took: which roles are projected and
that an unprojected seat has no projector; what `projected` decides for the
DM and the agent (the subscription, the head, the fold); that a projected seat
is fed from 0 and judges each event against the state after it, from what it
received, never the head; that the gateway keeps no fold of its own; that
input at or below the cursor is still folded; the fold-failure arm, its log
line and the one-writer bound; the empty opening viewpoint; the kind rule, the
one refusal string and the empty id; `perchBox`'s one slot, latest wins, `set`
never blocking, `mu` ordering nothing; the perch applied at once against the
last folded state, carrying sequence 0 and never filtered by the cursor;
`canSee` touching no seat; the catch-up's stop and `perches` made after it.

Facts dropped, each with the reason: the dated histories, the "A mutex was
tried" and retraction paragraphs, the commit-count parenthetical in
`pastResume` and the gosec investigation narrative (history, rule 10); the
frame counts of a coalesced burst against a queued one and the stall shares
(measurements, which `docs/reports/2026-08-18-visibility.md` holds); the
visibility ticket's quotations in `MayPerch`'s and `perch`'s docs (another
document's words); that `received` grows with the log for the life of the
connection (a cost the report states and no record needs); that `perchBox`'s
coalescing bounds the re-projection by the pump's speed rather than the
sender's (the reason for the design, which SPEC-015's "latest wins" states
without, and which `docs/reports/2026-08-18-visibility.md` holds); and two
false sentences, `receive`'s "Unreachable while campaign.Append is the only
writer" and `projected`'s "changes no byte on any wire". Also dropped:
`projected`'s argument that naming the two unprojected roles is the only
direction that stays closed when the set of roles grows (a design argument;
SPEC-015 states what `projected` answers); `receive`'s argument for logging a
failed fold rather than swallowing it (an argument; the `slog.Error` call is
the decision, and SPEC-015 states it); and `perch`'s cost, one `look` per hop
(a measurement).

`viewpoint_internal_test.go`'s three re-aimed blocks held measurements. The
frame counts and the blocking hand-off's share are in
`docs/reports/2026-08-18-visibility.md`. The rest is recorded here as that
file stated it at `74c547f`: each assertion of
`TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt` was reddened by one
injected fault — the whole scene list folded into `look`'s squares, the memory
fast-forwarded over every scene in `transitions`, every scene marked
introduced after `reperch` computes its frames, and `pr.seen[id] =
now.squares[id]` in the introduction loop. A fifth, a `served` set in
`reperch`, reddened nothing and is now the third debt entry. A first-wins
`set` passed `TestARapidHopIsCoalescedToTheShoulderItEndedOn` while its burst
ended where it started.

## The breaks

In a scratch clone of the changed tree, committed there as that clone's
`main`; each gate first ran clean (`193 rows ... every row's evidence holds`,
`239 files, 0 added comment lines, 239 ledger rows; clean`, `79 files, every
doc comment sits on its own function`, `seat.go` and `viewpoint.go` `same`),
then one edit per break, reverted by its inverse, `git diff --stat` empty
before the next.

| Break | Red |
|---|---|
| B1 VTT-999 added to the citation line above `TestAPerchRefusalDoesNotSayWhetherTheActorExists` | `check:requirements-chain: internal/gateway/viewpoint_test.go cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 VTT-192's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `row VTT-192: internal/gateway/codec_test.go does not carry the id, so the link walks one way only` |
| B3 VTT-999 on SPEC-015's Requirements line | `docs/specifications/015-the-seat-and-the-perch.md cites VTT-999 and no row in docs/requirements.md defines it (specification citation)` |
| B4 a `// SPEC-015` line above `pastResume`'s return, after the ledger was written | `check:comments: internal/gateway/seat.go: comment share 30.00 is above its ceiling 29.6 and this change added a comment line to it (SPEC-010)` |
| B5 `pastResume`'s `>` made `>=` | the token instrument prints `DIFFERS` for `seat.go`, and `TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway` is red: `catch-up from after=12 re-sent something the seat already holds` |
| B6 "Unreachable while campaign.Append is the only writer" back above the `#nosec` line | `grep -c` prints 1 |
| B7 the pointer above `catchUp` opened with "Project" | ``check:doc-owner: ... the doc comment above `catchUp` begins by describing `Project`, which is a different function`` |

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as first written. | Amended at sign-off on ten points (header). | The verification found them; each is named in the plan's "Verification, check by check" and its Measurements. |
| Plan D1: "within one process every prefix a seat receives folds", and a second process reaches the fold-failure arm. | SPEC-015 bounds it by one `Campaign` per log, a second `Campaign` "in another process or the same one" reaches the arm only when it appends an envelope that does not fold, and the search for the store's callers is `grep -rln 'vtt-platform/internal/store"'`. | The review opened two `Campaign`s on one directory in one process: two `SceneCreated` for one scene reached the arm and the next `Open` refused the log; two `NarrationAdded` did neither. `grep -rn 'c.log.Append'` over `internal/campaign` could not see a caller elsewhere. |
| VTT-179 as dispensed: "sent nothing at or below it". | "sent no frame of an event at or below it", and SPEC-015's drop paragraph likewise. | The review: a perch's frames carry sequence 0, below every cursor, and VTT-186 and VTT-187 require them sent. |
| Plan D4's fold-failure warning: "Never log its event id, which is participant text." | The sentence went. | The review: every production event id comes from `newEventID()`, random hex, and no command carries one. The cut block made the same false claim. |
| Plan D13's text for the burst test's doc: "either alone passes an empty perch". | "without the second, the first passes an empty perch", and "served in full, since the projector's memory never held it". | The review: the test's `sawEnd` check fails an empty perch; only the "only r-c" loop passes one. The shoulder is served in full because the memory never held it, not from the memory. |
| Plan D1: a failed fold withholds that event. | SPEC-015 adds that every later event is withheld too, and `world` stays the last state that folded; VTT-185 was reworded to "the state its seat last folded". | The review probed a third event after a failed fold: `received` keeps the bad envelope, so every later fold fails. |
| Plan D7's `wake` warning: "a waiting signal covers whatever the slot holds". | "unbuffered, set's send is dropped while the pump is busy and the shoulder waits for the next hop". | The review: with `wake` unbuffered the gateway's tests stay green; the hazard the capacity guards is untested, and `docs/verification-debt.md` now carries it. |
| Plan D1's role-arm sentence: "reached through `Authorize` by a spectator alone". | "through `Authorize` the role arm never refuses", with the search that makes `Authorize` its one production caller. | The review: the sentence read as if a spectator got the role refusal. |
| SPEC-015's title and several phrasings from plan D1. | "whose viewpoint only a spectator may set"; "the one writer goroutine"; the catch-up paragraph keeps only what SPEC-011 lacks; `catchUp`'s pointer covers both kinds of seat; SPEC-013's last paragraph rewrapped. | The review's nits: a DM's grant moves a player's eyes; "the one writer" collided with the log's writers; the head is SPEC-011's; `catchUp` serves an unprojected seat too. |
| Plan: one debt entry, for the fold-failure arm. | Three: the two-writer recipe, `wake`'s capacity, and the untested return to a shoulder sat on and left. | The review re-ran both probes, and both left the gateway green (the second was recorded in the burst test's doc at `74c547f`, which this change cut); the owner ruled all three into the debt file. |
| One `task check` run. | Two. The first was killed during `check:mutation` by a disk watchdog set at 17 GiB free; the second ran whole with the watchdog at 8 GiB. | The first reached `check:mutation` with 18.2 GiB free, by that step's own line, and was killed during it. `check-mutation.py` refuses to start below 16 GiB and budgets 8 GiB a run (`RUN_COST_BYTES`, its measured 7.6 rounded up), so a 17 GiB watchdog stops a run the gate admits. After the Go cache and a `gremlins-*` leftover were cleared, the second reached that step with 20.1 GiB free. |

## What could not be established

- VTT-185, VTT-191 and VTT-193 are OPEN. A perch judged against the head, an
  event forwarded after a failed fold, and a refused perch reaching the pump
  each leave `./internal/gateway/... ./cmd/vtt/...` green; the tests that
  would close VTT-185 and VTT-193 are named in the plan's sign-off questions 9
  and 16, and VTT-191's is the internal test the two-writer entry in
  `docs/verification-debt.md` names.
- `docs/verification-debt.md` carries three entries from this change: two
  `Campaign`s on one directory can write a log that no longer opens; `wake`'s
  capacity is unobserved; and no test asks for a shoulder a spectator sat on
  and left.
- `TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive`'s doc says it pins
  which state the pump projects against; it stays green when each event is
  judged against the state before it, and reds only when the head is
  plumbed in.
- The `#nosec G706` line is inert under the pinned `golangci-lint` 2.11.4:
  deleted, gosec still reports nothing. It stays, bound to its call.
- The busy-table rule's red is a rate: VTT-189's red-making edit reds 10 runs
  in 12, 8 in 12 with only `set`'s `default:` arm removed, 0 in 12 at the
  base; P, refused, was 2 in 12.
- Left stale by this change's scope: `project.go`'s `reperch` doc, which says
  the perch measurement is "written up at perchBox"; `keystone_test.go`'s
  "internal/gateway/seat.go's own doc comment says so" inside a 103-line
  block; `perchFixtureLog`'s doc and its `ActorControlGranted` entry with no
  arm; the perch tests' docs over the bound; the debt file's oracle-corpus
  entry naming `TestARefusedLookStillFillsTheRoster`, which the tree no longer
  holds. The projection's, the test-prose sweep's and the debt file's.

## What was deliberately left out, and where it went

- What a projection computes: `Project`, `reperch`, `eyes`, `look`,
  `canSeeSquare`, `perchSequence` and the `Viewer` type, `project.go`'s, the
  projection ticket's, which first needs three decisions.
- The rows whose tests observe the projection's output
  (`TestLeavingAShoulderTakesTheCreaturesAndNotTheTerrain`,
  `TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen`,
  `TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt`, `project_test.go`'s
  perch tests, the DM and agent arm of `Project`): the projection ticket's.
- A single-writer lock on the campaign directory: its own ticket, after the
  gateway sweep.
- No file under `cmd/`, `contract/` or `client/src` changed, nor any under
  `internal/` outside `seat.go`, `viewpoint.go` and the five test files, whose
  token streams are identical to `74c547f`'s (item 5).

## The sort

Eighteen rows, VTT-176 to VTT-193: the plan's fifteen accepted and its three
OPEN. Refused: G, only a spectator perches (VTT-139 to VTT-141); O, a shoulder
a burst flew past served in full (`reperch`'s); P, one order on the wire while
the table is busy (SPEC-011's single producer); V, `canSee` records nothing
(`look`'s, and VTT-155 and VTT-156 hold the gate); W, the catch-up head
(VTT-086); X and Y, what leaving a shoulder takes and a perch arriving with
its doors open (the projection's); Z, a perch on an unprojected seat or one
that has folded nothing (unreachable over the wire, and `reperch`'s guard);
AA, a spectator seeing through the shoulder it rides and a player's viewpoint
ignored (`eyes`'s); AB, the rest of the hop test (the projection's output);
AC, `Project`'s DM and agent arm. Three clauses stayed prose: an unknown role
(`identity.Verify` refuses it first), a party member nobody controls, and
"nobody else is told" of a perch.
