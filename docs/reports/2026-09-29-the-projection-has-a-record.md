# The projection has a record: the change

**Ticket:** `docs/superpowers/specs/2026-09-29-the-projection-has-a-record-design.md`,
amended at sign-off on the six points its verification found: withheld no
longer claims an actor's removal, which `classify` forwards to a viewer that
held the actor, nor a scene's, which no payload removes; an unrecognised
payload reaches no player or spectator rather than nobody, since the DM and
the agent are sent it; a scene is introduced once an eye stands in it, not
once the viewer can see into it; an object is sent whole once any square of
its footprint is visible; `transitions` forgets actors the world no longer
has and sends a `SceneSeen` only when the set changes; and "What it touches"
gained `tools/mutation-equivalents.txt` and `docs/verification-debt.md`, and
names `internal/engine/apply.go`'s pointer as left.
**Plan:** `docs/superpowers/plans/2026-09-29-the-projection-has-a-record.md`,
verified by `verify-ticket` (Passes with gaps). Its eleven sign-off questions
were answered on 2026-09-29, all eleven as the plan proposed: Status names the
two rulings and that their tickets are not yet written; V, W and AA held for
those tickets; the keystone accepted as one rule; C refused and stated as
prose; H2 OPEN with a debt entry; O, Q and X accepted as worded, O with the
testimony ticket free to reach it; SPEC-015's five pointers re-aimed;
`project_test.go`'s pointer re-aimed and `apply.go`'s left; the mutation keys
re-pointed in the change's own commit; Z and AC accepted on the goldens' byte
comparison; and the reading governs over the bound.
**Last commit of the change (no code line changed):** `48f9310`, on `db1f77d`,
`main` at the time. Every code reference below is to that tree.

## The period, in commits

    git log --oneline db1f77d..48f9310

    48f9310 The projection has a record: SPEC-016, VTT-194 to VTT-227

`git diff --stat db1f77d..48f9310`: 14 files changed, 1922 insertions(+), 753 deletions(-).

The gate: `task check`, whole, over the tree of `48f9310` before it was
committed: exit 0, no step failed, and the check steps' own verdict lines
read `check:comments` `clean`, `check:requirements-chain` 227 rows and 10
specifications, `check:doc-owner` 79 files, `check:new-prose` 175 added
lines clean, `check:coverage` 20 packages at or above their floors,
`check:no-pack` clean, `task lint` 0 issues, `check:mutation` fourteen
packages with zero unadjudicated survivors (`internal/gateway` measured
afresh, 4 survivors, all adjudicated, the three re-pointed keys among them;
the other thirteen reusing their last verdicts, six mutants among them not
evaluated and counted as killed),
`check:ts-mutation` 2882 mutants, 2783 killed, 29 timed out and counted as
killed by that gate's own rule, 70 survivors all adjudicated, zero
unadjudicated. Before it: `gofmt`, `go vet`, `task lint`, the gateway
package's tests, `check:comments`, `check:doc-owner`,
`check:requirements-chain`, `check:new-prose`, both mutation self-tests, the
token instrument, and the breaks; then the pre-commit hook's nine checks.

Phase 4a was skipped: the change moves prose and changes no code line, and
what can be wrong in it is a sentence, which the reading review holds.

**Rule 9.** The seat change's answer stands: MapTool decides what a client
sees on the client, from a whole campaign every client holds, and there is no
projection at all. `MapToolServer.addRemoteConnection` sends a joiner every
zone with all its tokens, GM layer included, and `ServerMessageHandler`
relays each change to every client; vision is computed per token on each
client (`ZoneView.getTokenVisibleArea`, `FogUtil.calculateVisibility`) and
cached per view; `ZoneView.getVisibility` keeps what a view sees now apart
from what it has explored; a token entering or leaving view is a render
decision (`ZoneRenderer.isTokenInNeedOfClipping`), never a message; and a map
hidden from players is still sent to them. MapTool has no doors. Borrowed:
the split between what a view sees now and what it remembers (here
`SceneSeen`'s visible set against the client's explored terrain), a view as
the union of its eye tokens' sight, and sight recomputed from the tokens
rather than stored. Refused: the whole campaign sent and clipped on the
client, which rule 9 names, since a projected seat is sent only what
`classify` and `transitions` admit; a sighting as render state, since a
client never sent a token cannot draw it, so arrival and departure travel as
`TokenPlaced` and `TokenHidden`; and a hidden map sent and hidden by the UI,
since a scene reaches a viewer only when one of its eyes stands in it. The
door correction, `doorTransitions`, has no MapTool precedent. SPEC-016 names
no other project.

## Done looks like, answered

1. `[x]` `docs/specifications/016-the-projection.md` exists with the five
   headings (`grep -c '^## '` prints 5): which viewers are projected and what
   `Project` answers; whose eyes a viewer has; what a look sees; what the
   projector remembers; what `transitions` sends, in order; doors; how each
   payload is ruled; a perch; the sequence every frame carries; and what it
   does not decide, pointing at SPEC-015, `internal/sight`, SPEC-007,
   SPEC-011, SPEC-013, `engine.IsPartyMember` and the two folds. Its Status
   names the two rulings of 2026-09-29, the sentences each will change, and
   that their tickets are not yet written. The reading review (Phase 4b, on
   `opus`) read every sentence against its symbol, by command and by probe;
   the one sentence it refuted and those it found imprecise were rewritten
   before the commit (Deviations).
2. `[x]` `grep -c 'UNRESOLVED'`, `grep -c 'written up at perchBox'` and `grep
   -c 'FLAGGED FOR ADJUDICATION'` over `internal/gateway/project.go` print 0,
   0 and 0; at `db1f77d` they printed 1, 1 and 2. The notes arm's flag went
   too, since the owner's notes ruling adjudicated it.
3. `[x]` Thirty-four rows, VTT-194 to VTT-227: thirty-two cited by the tests
   in their evidence cells, and VTT-201 and VTT-227 OPEN with no citation
   line, named below. `python3 tools/check-requirements-chain.py .` prints
   `227 rows, 193 test files, 10 specifications; every citation resolves and
   every row's evidence holds`.
4. `[x]` `python3 tools/check-comments.py --report` prints `banned 0` and
   `blocks>6 0` for `project.go`. That every surviving block is a warning, a
   pointer or the doc sentence of `Viewer`, `Projector` or `Project` is the
   reading review's verdict (VTT-051). The ledger row moved in the same
   commit.
5. `[x]` The go/scanner token stream, comments dropped, of `project.go`
   (3502), `project_test.go` (16828), `project_property_test.go` (1600),
   `keystone_test.go` (4593), `viewpoint_internal_test.go` (2192) and
   `server_visibility_test.go` (6280) is identical to `db1f77d`'s.
6. `[x]` SPEC-013's move-gate paragraph says what a projection sees is
   SPEC-016's, and SPEC-015's six sentences that sent a projection rule to
   `project.go` name SPEC-016 instead; `git diff --word-diff` shows no other
   word changed.
7. `[x]` `task check` whole, exit 0, above.

## What the rules became

| Rule, as the amended ticket words it | Became |
|---|---|
| A player sees through the actors they control and a spectator through the party member they perch on; no viewer sees through anyone else | VTT-194 (a player), VTT-195 (a spectator); "no one else" refused as a row (C) and stated as SPEC-016 prose, since `viewerFor` and `MayPerch` keep both cases off the wire (VTT-141, VTT-180, VTT-181) |
| A viewer is sent a scene only once a token of one of its eyes stands in it, and then only its name and size | VTT-196, VTT-197 |
| A viewer is sent the tiles of the squares it sees in a scene now and each object with a square of its footprint among them, and is told when that set changes | VTT-198 (squares and tiles), VTT-199 (objects), VTT-200 (told when it comes to see more or fewer squares), VTT-201 OPEN (a change to as many other squares), VTT-202 (a scene gone dark, once) |
| A token reaches a viewer as a placement only while it stands on a square the viewer sees, and leaves as a `TokenHidden` when it stops | VTT-203, VTT-204; a token removed from the world, VTT-205 |
| An actor is introduced to a viewer when it is a party member or when its token is first seen, with its controllers as grants and its conditions | VTT-207 (not a party member: once seen), VTT-208 (a party member: seen or not), VTT-209 (the controllers), VTT-210 (the conditions), VTT-211 (an id used again) |
| A token's move is forwarded only when the viewer saw the token before and after it | VTT-206 |
| A door's change is forwarded only when its square is seen, and a door that changed unseen is corrected when it is seen | VTT-212, VTT-213; a seen change arrives once, VTT-214 |
| An attack, an ability or a control change is forwarded only when the viewer knows every actor it names | held for the testimony ticket (V) |
| A resource or condition change, or an actor's removal, is forwarded only to a viewer who already holds the actor | the removal VTT-215; resources and conditions held for the testimony ticket (W) |
| Narration and a session's start and end reach every viewer | VTT-216, VTT-217 |
| A note reaches no player or spectator | held for the notes ticket (AA) |
| A scene, actor or token created or removed in the log, and a payload only the projection issues, is never forwarded as written | VTT-218 (creations), VTT-205 (a token's removal); an actor's removal is forwarded (VTT-215); the projection-only payloads refused as a row (AD), unreachable from any command |
| A payload the projection has never heard of reaches no player or spectator, and every arm of the envelope has a ruling | VTT-220 (no player), VTT-227 OPEN (no spectator); every arm has a ruling refused as a row (AF), a guard on the contract that SPEC-016 states as a Consequence |
| Folding a viewer's projected stream gives the world the server says that viewer sees | VTT-225, as one rule |
| Every frame of one event carries that event's sequence | VTT-221 |

Five rows came from the plan's sort rather than the ticket's list: VTT-219,
an adventure load is forwarded to no player or spectator; VTT-222, projecting
an event changes neither the event nor the state; VTT-223, one log projects
to one stream every time; VTT-224, every frame folds onto those before it;
VTT-226, a perch sends everything the new eyes see that the viewer was never
sent. VTT-227 came from the reading review, which split the unrecognised rule
by role when no test observed its spectator half.

## The sweep

Comment lines over non-blank lines, counted by `tools/check-comments.py`'s own
`measure` with citation lines set aside, at `db1f77d` and at `48f9310`:

| File | Before | After | Banned | Blocks over the bound |
|---|---|---|---|---|
| `internal/gateway/project.go` | 723 / 1173 | 115 / 565 | 64 to 0 | 30 to 0 |

The five test files gained citation lines and, above two tests, a blank line
(Deviations): `project_test.go` 42,
`project_property_test.go` 1, `keystone_test.go` 2,
`viewpoint_internal_test.go` 3, and `server_visibility_test.go` none, whose
four tests gained ids on the lines they already carried. `project_test.go`
also had one line re-aimed, inside a block within the bound. Their comment
shares did not move. Ledger row, old to new: `project.go` 61.7 to 20.4, the
one row plan D11 named, and no other.

What the kept blocks are. Doc sentences: `Viewer`, `Projector` (with the
warning to feed it from the first event), `Project` (with the warning never
to write to the event). Pointer sentences above `perchSequence`, `reperch`,
`sightView`, `look`, `eyes`, `transitions`, `sceneSeenFor`, `objectInSight`,
`verdict`, `classify`, `doorTransitions`, `doorSubject`, `squareAt`,
`squareKey` and `sortedSceneIDsUnion`, most closing on a warning. Warnings:
on the `scenes`, `actors`, `seen` and `doors` fields; on the sight-range
constants; in `Project`, on the DM and agent arm, the unknown-role arm and the
unrecognised check; on `reperch`'s nil guard; in `look`, on the unordered
walk, the scene entry and the party-member loop; in `eyes`, on each arm; in
`transitions`, on the forgetting loop, the scene introduction, the door call,
the clone, the grants, the conditions, the union walk, the unguarded scene
read, the empty `SceneSeen` and forgetting a dark scene; in `sceneSeenFor`, on
the sorted visible set; in `classify`, on the narration, `SceneCreated`,
`TokenRemoved`, `TokenHidden`/`SceneSeen`, notes, `AdventureLoaded` and
`TokenMoved` and `ActorRemoved` arms and on the knows and already-held
dividers; in
`doorTransitions`, on the unparseable key; and the floating block above the
sorters. The reading review named each block's kind and found every
warning's consequence true of the code it guards, `reperch`'s nil guard by
probe: with its guard alone removed
`TestASeatPerchesOnlyAgainstAWorldItHasSeen` passes, with `look`'s alone
removed it passes, and with both removed it panics.

Facts the cut blocks held that SPEC-016 took: a live event is one envelope
every seat shares, so `Project` writes to nothing it is given; the DM's and
the agent's answer reads no state; an unknown role gets nothing; an
unrecognised payload derives nothing; the projector's maps are a function of
the log prefix and the viewer, not of the state, so a projector is fed from
the first event; a scene is never withdrawn; `look` is recomputed every call
and writes nothing; standing in a scene earns its board; party members are
always known; a player's viewpoint is ignored; the order `transitions` keeps
and that both folds require; departures before arrivals; an actor forgotten
after `classify`; the introduction as a copy, controllers as grants carrying
the kind, conditions behind it with the introduction's sequence; the union
walk and the dark report once; an empty `SceneSeen` forgets no terrain; the
sorted visible set; an object with no square sent to nobody, as
`sight.Blockers` casts no shadow for it; `classify` before `transitions`;
each arm's ruling and its reason where the reason is a fact about the code;
a door out of sight keeps its belief, and `OpenDoors` travels in neither
introduction; `squareAt` reads a key strictly; `squareKey` builds
`sight.VisibleFrom`'s key.

Facts dropped, each with the reason. History, rule 10: the undo and
retraction accounts in `perchSequence`'s, `transitions`' and the forgetting
loop's blocks, the scene-forgetting loop's obituary and its argument about the
mutation gate, the dated corrections, the "used to" sentences, the task and
finding numbers. Quotations of another document: the visibility ticket's
sentences in the `Viewer`, `Projector`, scene-introduction and narration
blocks, and "the bird remembers every shoulder it has sat on". Measurements
and a proposal, which `docs/reports/2026-08-18-visibility.md` section 6
lists by the same blocks: `sight.VisibleFrom`'s cost per eye per event (15
ms on a sparse 60 by 60 scene, 176 ms on a dense one), the protojson
encoder's ordering of the tiles map, and the memo key the cut block called
sound for `look`, the scene id, the eye's position and the set of open
doors, since terrain is fixed once a scene is created; no memo is built.
False or stale, and gone: the torn-batch
paragraph, which `seat.pastResume` and the client's reconnect resolve and
which cited `client/src/wire.ts` and `client/src/session.ts` by line;
`reperch`'s pointer to a perch measurement "written up at perchBox", whose
comment the seat change cut; both `FLAGGED FOR ADJUDICATION` paragraphs, which
the owner's rulings settled. Arguments, since the ruling is the decision and
SPEC-016 states it: why a forwarding default leaks, why six scene names are a
table of contents, the three ways a token leaves a board, why knowing an
actor is weaker than seeing it, and the "fourth copy of a three-line format"
case for `squareKey`. Facts that belong to another component, and are that
component's to record: both folds tolerate a repeated `TokenHidden`; a
duplicate `ResourceChanged` folds silently in three cases (a delta of 0, a
negative delta at 0, a positive one at a positive maximum); a revoke is
idempotent by construction in both folds; the grant arm writes the kind only
when a grant states one; `remove_actor`'s batch removes an actor's tokens
before the actor; and `squareAt` mirrors `mapdef`'s `parseSquareKey`. Implied
by what SPEC-016 states, and dropped as a sentence: after an unrecognised
event the board catches up at the next recognised one; a door event forwarded
for a visible square never names a scene the viewer lacks; the session arm's
reason, that the client builds its session panel from those events. And one
fact about the code, dropped with its warning: `transitions` stores `look`'s
own map in `seen`, which is safe because `look` builds fresh maps on every
call, a property `look`'s kept warning against a memo protects.

## The breaks

In a scratch clone of the changed tree, committed there as that clone's
`main`; each gate first ran clean (`227 rows ... every row's evidence holds`,
`239 files, 0 added comment lines, 239 ledger rows; clean`, `79 files, every
doc comment sits on its own function`, the mutation self-test `OK`,
`project.go` `same`, `grep -c 'UNRESOLVED'` 0), then one edit per break,
reverted by its inverse, `git diff --stat` empty before the next.

| Break | Red |
|---|---|
| B1 VTT-999 added to the citation line above `TestAnUnrecognisedPayloadIsWithheldFromAPlayer` | `check:requirements-chain: internal/gateway/project_test.go cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 VTT-205's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `row VTT-205: internal/gateway/codec_test.go does not carry the id, so the link walks one way only` |
| B3 VTT-999 on SPEC-016's Requirements line | `docs/specifications/016-the-projection.md cites VTT-999 and no row in docs/requirements.md defines it (specification citation)` |
| B4 a `// SPEC-016` line inside `sameSet`'s loop, after the ledger was written | `check:comments: internal/gateway/project.go: comment share 20.49 is above its ceiling 20.4 and this change added a comment line to it (SPEC-010)` |
| B5 `classify`'s `TokenMoved` test made `now.tokens[id]` alone | the token instrument prints `DIFFERS` for `project.go`, and `TestSteppingIntoViewArrivesRatherThanMoves` is red: `a move OUT of the dark names the dark square it started in` |
| B6 "UNRESOLVED" back above `transitions`' doc | `grep -c` prints 1 |
| B7 the pointer above `classify` opened with "Project" | ``check:doc-owner: ... the doc comment above `classify` begins by describing `Project`, which is a different function`` |
| B8 the key `project.go:365:26` moved to `366:26` in `tools/mutation-equivalents.txt` | `check_mutation_test.py`'s `test_every_real_adjudication_still_points_at_its_mutant` fails: `column 26 of 'for x := max(o.X, 0); ...' holds ' < sc.Gr', and CONDITIONALS_BOUNDARY applies only to <= >= < >` |

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as first written. | Amended at sign-off on six points (header), and the plan's gaps 4 and 5, which said the ticket would stay as written, say so. | The verification found them. |
| Plan D6: a citation line directly above `func Test`, below the doc block, and no other test-file edit. | Above `TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees` and `TestAnIntroductionCarriesNoControllerAndTheGrantsBehindIt`, the citation line is separated from the doc block by a blank line. | Both doc blocks end in a Go doc list. With the citation line appended, gofmt adds a `//` line to end the list, and `check:comments` refuses a comment line added to a block over its bound (18 lines above the introduction test, 19 with gofmt's line; 103 above the keystone, whose share also rose above its ceiling). A blank line is neither a comment line nor a token, though it leaves each block floating above its test rather than documenting it; SPEC-008 asks only that the id sit on the line above the check. |
| Plan D5's row H1, dispensed as VTT-200: "sent a scene's SceneSeen when the squares it sees there change, and not when they do not". | "when it comes to see more or fewer squares there, and not when the squares it sees there do not change". | The review: H2's edit, `sameSet` comparing sizes alone, breaks the first wording and leaves both cited tests green; both H1 edits still red the second. |
| Plan D5's row AE, dispensed as VTT-220: "sent to no player or spectator". | "sent to no player", and VTT-227, OPEN, for the spectator. | The review: with the unrecognised check made to hold for a player alone, `./internal/gateway/...` is green; re-run before the fix. |
| Plan D5's evidence for A, O and AB (VTT-194, VTT-208, VTT-218). | Each gained a test: the keystone for VTT-194, `TestASpectatorGetsNoSightFromAnNPCTheDMControls` for VTT-208, `TestASpectatorHopsFromOneShoulderToAnother` for VTT-218. | The review: a player's eyes cut to the first actor, the party left off a spectator's roster, and an `ActorAdded` forwarded to a spectator each left every cited test green and reddened the test added. |
| SPEC-016's draft Consequence: a viewer learns of a scene, an actor or a token "once each". | "is never introduced to one it holds", with the two re-introductions named. | The review: a token that leaves sight is placed again on its return, and a reused actor id is introduced afresh (VTT-211). |
| SPEC-016's draft: "a token's departure precedes any arrival", after the order both folds require. | The sentence says no fold requires it, and what it keeps off the board. | The review: it read as a fold requirement, and the reason for the order was in a cut block and no record. |
| SPEC-016's draft Status, Principles, remembers paragraph, dark report and two Consequences. | `newSeat` and `project_internal_test.go` named; ruling (3) also names the party Consequence; "or the table is told"; the grep's file operand; the perches applied along the way; the dark report's empty-set edge; the state warning's own reason; a ruleset that supplies range and tolerance. | The review's nits, each checked against the code. |
| Plan D7's texts for `objectInSight`, `doorSubject`, `Viewer`, `look`, the `doors` field, `squareKey`, the knows divider and `sceneSeenFor`'s visible set. | The two docs point at SPEC-016; `Viewer` is who a projection is for, not a seat; `look` names what it may know; the `doors` warning states its consequence; `squareKey`'s consequence is every token and door lookup; the divider no longer calls grants testimony; the visible-set warning points at SPEC-016 and stays within three lines. | The review: two docs described and pointed at nothing, and the rest overstated or named the wrong thing. |
| Plan D14: rewrite the keys' current coordinates. | The sortedSceneIDsUnion entry's contrast sentence also moved, from `:1140` to `:607`. | It had named `:1140` since `59542e1` (2026-08-31), right when written, and stale from `cb1039e` the same evening through every later re-point, including those that claimed to have re-resolved it; the same entry's new sentence in `48f9310` said "since before 2026-08-31", corrected in the report's commit. |
| Plan: one debt entry, for H2. | Two: H2 (VTT-201) and the spectator's unrecognised payload (VTT-227). | The owner ruled the review's findings in whole. |

## What could not be established

- VTT-201 is OPEN: a viewer whose visible squares in a scene change to as
  many other squares is sent no `SceneSeen` with `sameSet` comparing sizes
  alone, and `./internal/gateway/... ./cmd/vtt/...` stays green.
  VTT-227 is OPEN: a spectator sent `transitions`' frames for an unrecognised
  payload leaves `./internal/gateway/...` green. `docs/verification-debt.md`
  carries both recipes.
- VTT-217 and VTT-219 are observed by one test only,
  `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`, a comparison of
  committed stream bytes: any change reds it and a regenerated golden greens
  it. A named assertion for each belongs to a later ticket.
- The testimony and already-held arms are observed thinly: an attack naming
  known actors withheld leaves the closure green; a grant, a revoke or a
  resource change withheld from a viewer entitled to it reds only the goldens'
  byte comparison; and the already-held test loosened to knows is green and
  cannot differ in production, since a projector is fed from the first event.
  Rows V and W are held for the testimony ticket, whose tests inherit this.
- Defence arms observed only by their own unit tests: a player's viewpoint
  honoured, a spectator on a non-party actor, an unknown role, a nil event or
  state (`TestTheProjectionFailsClosedWhenItHasNothingToGoOn`), and
  `reperch`'s nil guard alone, which no test observes because `look` guards
  nil too. SPEC-016 states each.
- `classify`'s `TokenHidden`/`SceneSeen` arm is unobserved and unreachable:
  no command produces either.
- None of the probes reddened the keystone alone; the `DoorClosed` arm made
  to forward reds it with the property test and the goldens. VTT-225 was
  accepted as one rule on the plan's recommendation.
- Left stale by this change's scope: `internal/engine/apply.go`'s sentence
  that `project.go` quotes its format string;
  `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh`'s quotation of a
  `transitions` comment that no longer says it;
  `TestAnObjectIsRevealedOnlyByTheSquaresItStandsOn`'s `project.go:510:54`
  and `511:54`; the projection tests' docs over the bound, histories among
  them; and `keystone_test.go`'s "internal/gateway/seat.go's own doc comment
  says so". The test-prose sweep's. Correct, but naming
  `internal/gateway/project.go` rather than SPEC-016: `client/src/wire.ts`'s
  replay-cursor block, the client's.

## What was deliberately left out, and where it went

- A note's visibility flag, public or DM-only: the notes ticket, next.
- Only what a viewer sees gives them information, for every actor, party
  members included: the testimony ticket, after the notes ticket. It rewrites
  `classify`'s knows and already-held arms, may reach VTT-208, and sorts rows
  V and W.
- What a square can see: `internal/sight`'s, which has no record.
- `server.go`'s blocks: the next gateway ticket. A single-writer lock on the
  campaign directory: its own ticket, after the gateway sweep.
- No file under `cmd/`, `contract/` or `client/src` changed, nor any under
  `internal/` outside `project.go` and the five test files, whose token
  streams are identical to `db1f77d`'s (item 5).

## The sort

Thirty-four rows, VTT-194 to VTT-227: the plan's thirty accepted outright,
Z and AC accepted on the goldens' byte comparison, H2 OPEN, and the review's
split of AE into VTT-220 and VTT-227, OPEN. Held for the rulings' tickets:
V, an attack, an ability or a control change forwarded when the viewer knows
every actor named; W, a resource or condition change forwarded to a viewer
that held the actor; AA, a note reaching no player or
spectator. Refused: C, no viewer sees through anyone else (defence behind
`viewerFor` and `MayPerch`, SPEC-016 prose); AD, a `TokenHidden` or
`SceneSeen` from the log withheld (unreachable); AF, every payload has a
ruling (a guard on the contract, a Consequence); AM, leaving a shoulder takes
its creatures and not its terrain (the sum of VTT-202 and VTT-204); AN, the
DM and the agent answered with the event (VTT-176); AO, an unknown role, a
nil event and a nil state answered with nothing (defence); AP, a perch on a
seat that has folded nothing (SPEC-015's); AQ, the two actor-creation tests
(`add_actor`'s and the grant's, SPEC-013); AR, visible and explored from
different sources (`engine.Apply`'s); AS, the corpus's power to tell a
projection from a passthrough (a guard on the corpus); AT, `canSeeSquare`
false for a missing position (defence). Three clauses stay prose: a player's
viewpoint ignored and a spectator on a non-party actor seeing nothing, and the
`SceneSeen` walk's equal-size case until VTT-201 is closed.
