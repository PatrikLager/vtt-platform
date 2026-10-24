# Verification debt, and recipes for defects that escaped

Two things live here that were previously written in code comments, where
nobody acts on them.

**Verification debt** is a coverage gap someone already knows about. Written as
a comment it explains one test; written here it is a claim on future work.

**A recipe** is how to put an escaped defect back. Recipes are cheap at the
moment of escape — you have just fixed it, so you have the exact edit — and
expensive afterwards, because the tree moves. Write one when a defect escapes;
do not backfill old ones.

Every entry names the gate that should have caught it and why it did not. The
labels are deliberately few:

| Label | Meaning |
|---|---|
| `spec silent` | no requirement determines the behaviour, so no test can pin it |
| `test data missing` | the requirement is clear; no fixture reaches the state |
| `test asserts nothing` | a fixture reaches it; the assertion cannot fail |
| `outside the tool` | no mutation operator expresses the defect |
| `dispatch too narrow` | the checker was given a filtered view of the spec |
| `requirement unnamed` | the rule exists but nothing could cite it durably |

Without that label an entry cannot be used in hindsight — it says a defect
escaped but not what to change so the next one does not.

---

*The three entries dated 2026-09-16 and 2026-09-17 describe `internal/perceive`,
which was rolled back uncommitted on 2026-09-20 to be redone under the process;
the package and the tests they name are not in the tree. They stay as the record
of their period.*

## 2026-09-16 — a refused `Look` also dropped the roster

**What escaped.** `Look` refuses sight to anything but a party member. Expressed
as an early return, that refusal also skipped the roster loop below it, which is
a different rule. Survived two review rounds; caught by a reviewer reading the
code, not by any test.

**Labels:** `test data missing`, `outside the tool`.

**Recipe.** In `perceive.Look`, change the refusal body from `eyes = nil` to
`return v`.

**Which gate should have caught it.** None could, and the first version of this
paragraph got the measurement wrong in the direction that mattered — corrected
2026-09-17 after a reviewer re-ran it. What is actually true:

- `if true ||` reds the oracle at FOUR seats, all on `ActorControlGranted`:
  `door-watch/act-latecomer`, `door-watch/act-watcher`, `shared-control/act-scout`,
  `shared-control/act-warden`.
- `if false &&` — the roster off for everyone — reds **nothing** except
  `TestARefusedLookStillFillsTheRoster`. Not one of the eleven seats notices.

So the two are COMPLEMENTARY, each holding one direction, and neither is
redundant:

- **roster too permissive** (`if true ||`, every actor named) — held by the
  ORACLE's four seats. `TestARefusedLookStillFillsTheRoster` passes under it.
- **roster empty** (`if false &&`) — held ONLY by that test. All eleven seats
  pass under it.

The corpus is blind to the WITHHOLDING direction by construction: `classify_test.go`
builds `Shown` by folding the seat's own recorded stream, which already carries
the synthesized introductions, so a missing `Lit.actors` entry is masked by
`Shown.Actors`. It is not blind to the permissive one — an actor the roster
should not name has no `Shown` entry to hide behind, which is what those four
`ActorControlGranted` seats catch. Deleting them to "save" duplication would
remove the only thing that reds when the roster over-shares, which is finding 14
and the reason the roster rule (the actor roster is projected too) exists.

The defect itself is additionally a control-flow restructure, which no operator
mutation expresses — so the mutation gate was never going to reach it either.

**What the wrong version would have cost.** It told the next reader that the
corpus defends the roster loop. Delete the test on that basis and a roster that
goes empty for everyone ships green: every character's log stops naming their
own party, and under the rule that a character log is the record of what that
character experienced, that is permanent for the characters it touched.

**Closed by** `TestARefusedLookStillFillsTheRoster`, pinning the rule that a refusal
of sight is not a refusal of the roster, which reds on the recipe above. The probe is `ActorControlGranted` with `Shown` empty
of the granted actor: a `knows()` arm carrying no position, so its verdict is a
direct read of the roster, which is otherwise unobservable.

**Fixture check.** Putting `a-hero` into `Shown` makes the same test pass with
the defect present — the empty `Shown` is load-bearing, not decoration.

---

## 2026-09-16 — the QA dispatch hid the requirement it was testing

**What escaped.** The first run of the independent QA stage reported that the
spec does not determine which component keeps the roster promise. It does:
visibility spec §5, *"The actor roster is projected too."* That section was not
in the four-section list the dispatch handed over, because the list was built by
reading headings and §5 is titled "Contract additions".

**Label:** `dispatch too narrow`.

**Recipe.** Dispatch the QA stage with a section list rather than the whole
spec, and choose the sections by heading.

**Closed by** the change to `qa-prompt.md` that forbids section lists: a
filtered view inherits the dispatcher's blind spot, which is the one thing the
stage exists not to inherit.

**Closed 2026-09-17, labelled `requirement unnamed`.** One rule from the same
finding lived in a code comment with no home in any spec: introduction has
exactly one code path. It is now a rule in the visibility ticket, citing the
per-character-logs rule that a character log is the record of what that
character experienced. Pinned by
`TestQAActorAddedIsWithheldEvenForAPartyMember`. What made it debt rather than
a tidy-up: the comment was inherited from `internal/gateway/project.go`, which
Task 12 of `docs/superpowers/plans/2026-09-14-per-character-logs.md` deletes, and it survived the port only because the port was verbatim.

---

## 2026-09-17 — the mutation gate reaches neither line the roster defect lived on

**What it is.** `internal/perceive` was gated the day it was written, and its
first run reports `Killed: 4, Lived: 0, Test efficacy 100.00%` over a file of
roughly five hundred lines — the exact count moved three times the same day, so
it pins nothing.
Nine `if` statements; five generate no mutant, and two of those five are the
sight refusal (`!ok || !engine.IsPartyMember(a)`) and the roster loop's
`if engine.IsPartyMember(a)`. A mutant appears where an ENABLED mutator
family's operator token appears — gremlins defaults five families to true, only
two of them comparisons, so "comparisons only" is not the rule. The roster line
holds no operator any mutator rewrites; the refusal holds a `||`, rewritten
only by `INVERT_LOGICAL`, which is off by default. The if-with-init is not the
cause — adding one comparison to that same statement produces mutants at it.

**Label:** `outside the tool`.

**Why it is recorded rather than fixed — with one correction.** A first version
of this entry said "there is nothing to fix in the gate; the mutator set is what
it is." That is false: the set is a FLAG. gremlins ships `INVERT_LOGICAL`
(`&&`/`||`), off by default, and with it enabled a mutant lands on the `||` of
the sight refusal and the suite KILLS it. What remains true is that the gate as
CONFIGURED does not reach those two lines, and that nobody should read `100.00%`
on this package as coverage of its security decision. Enabling the flag is an
open decision costing one new survivor at the `knows()` chain's first `&&`.

**What actually holds those two lines:** `TestLookRefusesEyesToAnythingButA-
PartyMember` and `TestARefusedLookStillFillsTheRoster`, both proved red by HAND
injection. That is not belt-and-braces; it is the only measurement those lines
have. Deleting either removes the whole defence and no gate will say so.

Published with numbers in `tools/mutation-scope.md`, per that file's job.

---

## 2026-09-23 — the shut-door refusal has no test that can see its write

**What it is.** `JoinAdmits` in `internal/identity` reads the door once and
refuses by one guard of four terms: the door not open, an empty stored secret,
a secret that does not match, a spent budget. The rule ADR-011 rests on is that
a refusal writes nothing, because identity shares its SQLite file with the
event log and a write on a refusal path takes the write lock on the file the
table is appending events to. The mismatch and spent-budget terms are held by
`TestAWrongSecretRefusesWithoutTouchingTheDatabase` and
`TestASpentBudgetRefusesWithoutTouchingTheDatabase`, which arm a database fault
so the `UPDATE` itself fails if it is reached; the empty-secret term by
`TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath`. The door term has no
such test.

**Labels:** `test asserts nothing`, `outside the tool`.

**Recipe.** In `JoinAdmits`, remove the shut-door term from the guard that
refuses before the `UPDATE`. Measured 2026-09-23 on the tree with that edit
applied: the identity and gateway suites stay green.

**Why the two tests that aim at it cannot see it.** `TestAClosedDoorSpendsNothing`
reads the admission counter afterwards. That proves the write had no EFFECT,
because the `UPDATE`'s own condition refuses the increment anyway, not that it
was never ATTEMPTED, which is the property. `TestARefusedJoinWritesNothingAtAll`
counts rows in the door's table before and after, with no row present at all,
so `JoinAdmits` returns at `sql.ErrNoRows` before the guard the recipe edits;
its path never reaches the line.

**Why the mutation gate cannot reach it.** It rewrites operators, and the defect
is a whole term disappearing from a chain of refusals.

**What closes it.** The instrument the other two refusals already use: arm the
database fault so the statement fails if it is reached, with a shut door in
place of a wrong secret. Not closed here; the joining-a-table arc's ticket is
where it is closed or recorded as open.

**Closed by** `TestAShutDoorRefusesWithoutTouchingTheDatabase` in
`internal/identity/fault_internal_test.go`, which reds on the recipe above
(observed 2026-09-24).

---

## 2026-09-24 — the promotion nudge reaches a revoked participant who is still connected

**What it is.** `announcePromotion` in `internal/gateway/server.go` sends a
`PresenceChanged` frame to every connection through
`presenceRegistry.announceIfPresent`, which takes no deny set. The connect and
departure announcements pass `revoked()`; this one does not. A participant
revoked while connected, who has sent no command and received no event since,
receives the frame. VTT-032 says a revoked participant is refused on their next
presence frame; this is the one presence frame that does not re-resolve. Found
by the reading review of SPEC-009 on 2026-09-24, in the tree at `8ebb2b1`. Not
closed here: a Phase 2 item for the joining-a-table arc's second ticket, or the
next.

**Labels:** `test data missing`, `outside the tool`.

**Recipe.** It is in the tree; nothing puts it back. What closes it is passing
`s.revoked()` through `announceIfPresent` the way `announcePresence` passes it
to `broadcast`, and a test that revokes a connected watcher, promotes somebody
else, and reads nothing on the watcher's socket.

**Which gate should have caught it, and why it did not.** VTT-032's three tests
exercise a command, a delivered event and an arrival announcement after
revocation; none promotes anyone after revoking a connected watcher, so the
frame is never read for. The mutation gate cannot reach it: the defect is a
parameter that was never there, not an operator it can rewrite.

**Closed by** `TestAPromotionIsNotAnnouncedToARevokedWatcher` in
`internal/gateway/server_test.go`. It reds when `announcePromotion` hands
`announceIfPresent` an empty deny set in place of `s.revoked()`, which is the
edit that puts the defect back, and when `announceIfPresent` ignores the set
it is handed (observed 2026-10-14).

## 2026-09-29 — two writers on one campaign directory could write a log that no longer opened

**Two `Campaign`s on one campaign directory can write a log that no longer
opens.** Nothing locks a campaign directory to one writer: `grep -rn -i
'flock\|lockfile\|O_EXCL'` over `internal/store`, `internal/campaign` and
`cmd/vtt` prints nothing, and each `Campaign` validates an append against its
own in-memory state. Recipe: open two `campaign.Open` handles on one
directory, in one process or two, and have each append a `SceneCreated` for
the same scene id; both are accepted, a projected seat fed the log logs
`campaign: corrupt log at seq 2: engine: scene "s" already exists` and is sent
no further event, and the next `campaign.Open` refuses the log, so the
campaign no longer boots. Two writes that do not conflict (two
`NarrationAdded`) fold, so the damage depends on what the second writer
appends. VTT-191, a seat withholding an event whose fold fails, is OPEN
because no test hands a seat a prefix that does not fold. None needs a second
`Campaign` to: an internal test that calls `receive` with two `SceneCreated`
for one scene id and then a `NarrationAdded` is sent no frame for either,
where a fresh seat is sent one for the narration. Which gate should have
caught it, and why not: the tier-1 tests, and none did. No requirement says a
campaign has one writer, no fixture gives a seat a prefix that does not fold,
and no mutation operator adds a second writer. Labels: `test data missing`,
`spec silent`. Closing it needs a single-writer lock on the campaign
directory, which is a ticket of its own, and a test that a second `Open` of a
locked directory is refused; closing VTT-191 needs only the internal test
above, cited `// VTT-191`. Recorded 2026-09-29 by the verification and review
of
`docs/superpowers/specs/2026-09-29-the-seat-and-the-perch-have-a-record-design.md`,
whose change changes no code line.

**Closed by** `TestASecondOpenInTheSameProcessIsRefused` and
`TestAnOpenWhileAnotherProcessHoldsIsRefused` in
`internal/campaign/writer_hold_test.go`, which red when `takeHold`'s
`syscall.Flock` call is removed, and, for VTT-191,
`TestASeatIsSentNothingForAnEventWhoseFoldFails` in
`internal/gateway/viewpoint_internal_test.go`, which reds when `receive`
forwards an event whose fold failed (both observed 2026-10-05).

## 2026-09-30 — the deadline fixture failed when its shell started late

**The `TestQAMCPConnect*` deadline fixture fails when its shell starts
late.** In `cmd/vtt/qa_e2e_wait_test.go`, `qaScript` writes a shell script
whose first command writes its pid to a file, and the four tests that bound a
subprocess that does not answer give `connectMCPSubprocess`
`neverAnswersBound`, 500 ms, which includes the shell's own start-up. On a
loaded machine the connect's deadline passes, and the subprocess is killed,
before the shell reaches that first command; the test then fails on its
precondition, "the fixture never got to run: open …/pid: no such file or
directory", not on the behaviour it pins. Recipe: none is needed to put it
back; it is there. Seen 2026-09-30 in `task check`'s `check:coverage` step,
whose `-cover -p 1` run holds `cmd/vtt`, on `106dcaf`, which changed no Go
file: `TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessIgnoresStdio`. The
same family failed before, as gap 11 of
`docs/superpowers/plans/2026-09-29-the-seat-and-the-perch-have-a-record.md`
and gap 9 of `docs/superpowers/plans/2026-09-29-the-projection-has-a-record.md`
record. No gate catches it: the fixture is green on an idle machine, where
Phase 4a and 4b of `3aed0ce` ran it. Labels: `test data missing`. Closing it
needs the fixture to wait for the pid file before the bound starts, or a
bound that excludes start-up, in a ticket of its own. Recorded 2026-09-30 by
the note-visibility change, whose gate run it stopped. Seen again 2026-10-01
in `check:race`, on `e75c416`, which changed no file under `cmd/vtt`:
`TestQAMCPConnectEndsASubprocessThatIgnoresSIGTERM`.

**Closed by** `TestQAMCPConnectReachesItsAssertionsWhenTheFixtureStartsLate` in
`cmd/vtt/qa_e2e_wait_test.go`, which reds when `qaConnect` connects without
waiting for the pid line (observed 2026-10-01).

## 2026-10-02 — `move_token`'s `reason` never reached the log

**`move_token`'s `reason` never reaches the log.** `MoveTokenRequest` carries
`optional string reason`, which `contract/vtt/v1/commands.proto` says is shown
in the log and the MCP `move_token` tool offers; `TokenMoved` has no field for
it, so `ToEvent` drops it and the command answers ok=true. No test sends a
reason through `ToEvent`: `internal/mcp/tools_test.go` sets one against a fake
server and `contract/testdata/client_command.json` round-trips one, and
neither reaches the gateway. The gateway's tier-1 tests should have caught it;
`TestToEventMoveTokenProducesTokenMoved` sets no reason. The reading review of
`597051b` found it by calling `ToEvent` with one. Recipe: none is needed to
put it back; it is there. Labels: `spec silent`, `test data missing`. Closing
it is the next ticket, which adds the field to `TokenMoved`, additively, and
decides who is shown it. Recorded 2026-10-02 by the removal record's report.

**Closed by** `TestToEventMoveTokenProducesTokenMoved` in
`internal/gateway/convert_test.go` and `TestAMoveWithAReasonAppendsItsReason` in
`internal/gateway/server_test.go`, which red when `ToEvent` drops the reason
(observed 2026-10-04).

## 2026-10-04 — the client's fold accepted a narration speaker the server refused

**`fold.ts` never bounded `NarrationAdded.as`.** `engine.Apply` refuses a
speaker over `maxNarrationAsBytes` (256 bytes) since `61e35be`; the TypeScript
fold that came after it (`8551df4`) checked a narration's text and anchors and
never its speaker, so a DM client would have folded an `as` the server's fold
refuses. Nothing reached a table: the server never accepted such a narration,
so no client was sent one. Recipe: in `client/src/fold.ts`'s `narrationAdded`
arm, delete `checkLen("narration as", v.as, 0, 256);`. The fold-parity and
rejection tests should have caught it and had no case for a speaker over the
bound; the TS mutation gate cannot add a missing check. Labels: `test data
missing`, `outside the tool`.

**Closed by** `a narration speaker longer than 256 bytes is rejected` in
`client/test/fold-rejections.test.ts`, which reds when that line is deleted
(observed 2026-10-04, in `a4543e6`'s breaks).

## 2026-10-17 — a soak checkpoint read a log seat's stream before it had drained

**`runSoakCheckpoint` checked the log seats without settling them.** The soak
drains each connection in its own goroutine (`soakHistories.start`), so at a
periodic checkpoint a player's history could still lack entries the server had
sent, and a break in them was reported by a later checkpoint under its own
number; `ece9014` settled the log seats before the final checkpoint only.
Recipe: in `internal/harness/soak.go`, move
`histories.settle(logNames, denialAbsenceWindow, observeTimeout)` out of
`runSoakCheckpoint` and put it back before `RunSoak`'s final
`runSoakCheckpoint` call. Tier-1 `go test` should have caught it, through
`TestASoakCheckpointFailsACharacterStreamThatSkips` and
`TestQALSSoakPlayerStreamBrokenMidRunFailsItsCheckpoint`, and did not: both
fail on it only when the drain goroutines lag, which on this machine means
GOMAXPROCS 1 (every run) or 2 (some), never the default 8, and every run
before the commit was at the default. Task 7's commit hook, under load, was
the first to see it. Labels: `test data missing`, `outside the tool`.

**Closed by** `TestAPeriodicSoakCheckpointReadsWhatALateFanOutDelivers` in
`internal/harness/soak_test.go`, whose fan-out lands on timers that synctest
holds until the main goroutine blocks, so it reds on the recipe at every
GOMAXPROCS (60 of 60 across 1, 2 and 8, observed 2026-10-17 by the fix's
reading review).

## 2026-10-17 — the harness could not see a log seat switched on its old subscription

**The harness cannot see a log seat switched on its old subscription.**
Recipe: in `seat.watch` (`internal/gateway/seat.go`), at the top of its loop,
set `shown` and `told` and return when `s.unsub` is not nil, so a switch is
announced while the old log keeps arriving. Every `cmd/vtt` scenario test
stays green, the seed-1 soak included. The corpus's only switches are
`scenarios/shared-control.json`'s revocations, which move a seat from a log to
nobody, and a stream that keeps the old log then simply continues its run;
`internal/harness`'s readers skip `ViewpointChanged`, so `logBreak` sees a
switch only when the new log restarts at 1. The gateway's
`TestAConnectionStopsWatchingAnActorItMayNoLongerWatch` goes red on the same
edit (VTT-348). The same blindness makes `runReconnectStep` fail a correct
server once a scenario switches a log seat and then reconnects it: the redial
opens on the default (VTT-340), and the harness compares it with the log the
seat was switched to. Labels: `test data missing`, `outside the tool`.
Closing it needs the harness to know a seat's viewpoint, by surfacing
`ViewpointChanged` through `Conn` or by passing `log` through the `Dialer`,
which the fourth per-character-logs ticket carries. Recorded 2026-10-17 by
ticket 3b's Task 6, whose deliberate break found it.

**Closed by** `scenarios/a-player-switches-and-reconnects.json`, run by
`TestScenarioLibraryRunsSelfContained`, which fails under the recipe at its
first narration, "player's stream breaks its log: sequence 5 straight after a
switch to act-bram", and at its first reconnect (observed 2026-10-21 on the
commit that closes it). The scenario engine now holds each seat's switches
(VTT-411, held by `TestASeatThatAnnouncesASwitchAndKeepsTheOldLogFailsTheStep`
in `internal/harness/one_log_test.go`), and its reconnect names the log the
seat holds (VTT-414).

## 2026-10-18 — a promotion re-judged a seat only at the next append's wake

**A promotion re-judges a seat only at the next append's wake.**
`handlePromotion` called `identity.DB.SetRole` and `announcePromotion` and
woke no seat, and a log seat's pump re-judged what it showed only on an
append's wake, or on one `admit` found waiting. A spectator promoted
to player kept the log they watched, and were sent its queued rest, until the
next append anywhere in the campaign, though VTT-340 says a player watches
their first party member "whenever" they watch an actor they may not. The
same held for a promotion that landed while the connection opened. Recipe:
in `internal/gateway`, delete the `s.presence.rejudge` call in
`handlePromotion`, the block in `pumpState.onEntry` that calls
`ps.sub.rejudge()`, and the `|| !ps.moved(sub.recheck())` of `pump`'s start.
Tier-1 `go test` should have caught it through VTT-340's tests and did not:
`TestAPromotedSpectatorWatchesTheirFirstPartyMember` appends after the
promotion, and that append's wake re-judges the seat, so no test saw the
window. The mutation gate cannot rewrite a call that is not there. Recorded
2026-10-18 by ticket 3b's specification step, whose reading review found it.
Labels: `test data missing`, `outside the tool`.

**Closed by** the 2026-10-18 promotion ticket
(`docs/superpowers/specs/2026-10-18-a-promotion-re-judges-the-promoted-participants-seats-design.md`).
`TestAPromotionMovesASpectatorOffAShoulderThePlayerMayNotWatch`,
`TestAPromotedSpectatorMovesOntoThePartyMemberTheyHold` and
`TestAPromotionMovesEveryConnectionOfTheParticipant` in
`internal/gateway/watch_test.go` red on the recipe's first edit;
`TestAnEntryTakenAfterAPromotionIsJudgedByTheNewRole` and
`TestARoleReadAfterAdmitStillJudgesTheEntry` in
`internal/gateway/seat_internal_test.go` on the second;
`TestAPromotionAsTheSeatOpensMovesItOntoTheirPartyMember` there on the third,
and `TestAPromotionAsTheSeatOpensSendsNoEntryOfTheShoulder` on the second and
third together. The recipe's own case,
`TestQAQueuedPromotedSpectatorOnAnothersPartyMemberMovesToNobody` in
`internal/gateway/qa_promotion_queued_test.go`, reds on the first and second
together (observed 2026-10-18 on `3ddf8e7`).

## 2026-10-19 — the agent was told to grant a creature a player runs as a monster

**The agent's tool descriptions told it to grant a creature a player runs as
non-party.** `grant_actor_control`'s `kind` description in
`tools/toolgen/main.go` offered `ACTOR_KIND_NON_PARTY` "when you are handing
out a monster, NPC or creature for someone to operate", beside its own "grant
again with the other value when a monster is charmed into the party's
service", and `add_actor`'s `kind` said "a charmed monster is still a
monster". An agent that followed them would grant a summoned creature
`ACTOR_KIND_NON_PARTY`. The creature would then have no log, `MayWatch` would
refuse a `set_viewpoint` on it, and the move gate would judge a move of its
token by the eyes of the character the player watched. Recipe: in
`tools/toolgen/main.go`, in `grant_actor_control`'s `kind`, replace "for a
monster or NPC you run yourself, including one you take over to run" with "for
a monster or NPC you run yourself, or a creature for someone to operate,
including one you take over to run"; in `add_actor`'s `kind`, replace "and a
monster you run yourself is still a monster." with "a monster you run yourself
is still a monster, and a charmed monster is still a monster."; and run
`task generate:contract`. The reading review of `42f7af9` should have caught
it. That commit put the kind on the grant after the owner's revision of
2026-08-23, wrote the "for someone to operate" clause beside its own charm
clause, and left `80dfa0e`'s sentence in `add_actor`'s description. Why the
review passed them cannot be established: the commit's message says what it
found and that `add_actor`'s description was corrected in the same round, not
what the review was asked to read. A creature a player summons or possesses
was ruled on only on 2026-10-19, so before then no spec said which kind it
has. No requirement named what the descriptions say, so no test could pin
them: `TestToolsMatchGolden` holds the bytes whatever they say, no test read
`grant_actor_control`'s `kind`, and
`TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication` asserted only that
`add_actor`'s names both kinds, says "not about who controls" and does not say
omitting it is safe. Recorded 2026-10-19 by the creature ticket's Phase 1,
which found it. Labels: `spec silent`.

**Closed by** the 2026-10-19 creature ticket
(`docs/superpowers/specs/2026-10-19-a-creature-a-player-runs-is-a-party-member-design.md`).
`TestGrantActorControlSaysACreatureAPlayerRunsIsAPartyMember` (VTT-389) in
`tools/toolgen/main_test.go` reds on the recipe's first edit, and
`TestAddActorSaysAnIDIsNeverReused` (VTT-390) on its second (observed
2026-10-19 on `fd2a9ee`).

## 2026-10-20 — a dial that failed left its rejection unhandled

**Neither of the browser client's dials caught a failed dial.** On `0db3d18`,
`client/src/app.ts` started the first dial with `void session.start();` and
the Reconnect button's with `void redial();`, whose `session.reconnect()` or
`session.restart()` rejects when the dial fails, and nothing caught either
rejection: a browser logs it as uncaught, and bun aborted the test that made
one. Recipe: in
`client/src/app.ts`, replace `void session.reconnect().catch(() =>
undefined);` with `void session.reconnect();`, and `void
session.start().catch(() => undefined);` with `void session.start();`.
Tier-1 `bun test` should have caught it and did not: `client/test/wire.test.ts`
"close and reconnect are safe before anything was ever connected" showed
`Wire.reconnect` rejecting on a failed dial, and no test of `app.ts` made a
dial fail; the TS mutation gate cannot add a missing catch. Recorded
2026-10-20 by the Phase 4a QA of the client's one log
(`docs/superpowers/plans/2026-10-20-the-client-follows-the-log-it-is-sent.md`,
Task 3), which found it. Labels: `test data missing`, `outside the tool`.

**Closed by** "window: a Reconnect whose dial fails keeps the fold and offers
Reconnect again, naming the same log and cursor" in
`client/test/qa-one-log.test.ts` and "a first dial that fails leaves the page
offering Reconnect" in `client/test/app.test.ts`, both red under the recipe,
with bun reporting an unhandled error between tests (observed 2026-10-20 on
`982b4cc`).

## 2026-10-22 — a player's ability was aimed at the token, not the creature

**The player panel sent the target token's id as `UseAbility.target_ids`.**
`rules.Resolve` reads each target as an actor id and refuses one that names no
actor, so every ability a player aimed from the browser was refused with
`unknown target actor`, a DM's, aiming as an actor granted to them, included.
Every token in the shipped adventures has an id of its own (`tok-cutter` shows
`act-cutter`). It reached the table: the alpha test of 2026-10-22 found it as
S-1 (`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`). Recipe: in
`client/src/view/player.ts`, in `renderPlayerPanel`'s target button, replace
`useAbility(actorId, armed.id, [t.ActorID])` with `useAbility(actorId,
armed.id, [t.ID])`, then run `task build:client`. Tier-1 `bun test` should have
caught it and instead pinned it: "a target button sends the ability at that
token, then disarms" in `client/test/spectator-view.test.ts` gave the token and
its actor different ids and expected the token's. Nothing that test's author
would read said otherwise: no requirement row, no comment on the contract's
`UseAbility.target_ids` and no description in `use_ability`'s tool definition
said which id the field holds, and the contract's own examples
(`contract/testdata/use_ability_command.json` and
`ability_used_envelope.json`) put token ids in both fields. SPEC-016 and
SPEC-018's table, both written after the test (`0c8b6e3`), said it only in
passing, of `AbilityUsed.target_ids`. `task e2e`, which runs outside
`task check`, had no test in which a player used an ability. Labels: `spec
silent`, `test data missing`.

**Closed by** VTT-436's ten tests and two of VTT-441's, all red under the
recipe: "a target button sends the ability at that token's actor, then
disarms" in `client/test/spectator-view.test.ts`, "a player's ability reaches
the creature they aim it at" in `client/e2e/table.spec.ts` (refused, as at the
table), and VTT-436's eight QA tests with VTT-441's "qaAT: the command names
the caster as actorId and the armed ability as abilityId" and "qaAT: with no
actor selected the first controlled actor by id casts, from its own token" in
`client/test/qa-ability-target.test.ts` (observed 2026-10-22, in `80007aa`'s
breaks). VTT-437 says in the contract and in the tool which id the field holds,
and `0b62fa0` made the contract's examples name actors.

## 2026-10-24 — the browser's dump did not escape what Go's does

**`foldToDumpJSON` in `client/src/fold.ts` wrote `<`, `>`, `&`, U+2028 and
U+2029 raw, while `writeDump` in `cmd/vtt/state_dump.go` escapes them.** The
function's doc says it renders the fold "as cmd/vtt's writeDump does a world
seat's, byte for byte". `encoding/json` escapes `<`, `>` and `&` under its HTML
escaping, which `writeDump` leaves on, and U+2028 and U+2029 always, so any dump
holding one of the five in a string differed. It never reached the table: only
the dump-parity tests compare the two. Phase 4a's QA of Task 3 of
`docs/superpowers/plans/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md`
found it, with a token's notes holding `<trap> & "pit"`.

Recipe: in `foldToDumpJSON`'s return, replace
`.replace(/[<>&\u2028\u2029]/g, goEscape) + "\n"` with ` + "\n"`. Then delete
`goEscape` and its comment, or `task client:typecheck` refuses the unused
function.

The gate that should have caught it is `client/test/fold-parity.test.ts` over
`scenarios/goldens/`. No golden stream holds any of the five characters, so the
keystone only compared strings that never needed escaping. Labels: `test data
missing`.

**Closed by** two tests, both red under the recipe (observed 2026-10-24):
- "the dump of a named token matches the committed dump, as Go's writeDump does"
  in `client/test/fold-unit.test.ts`, over
  `contract/testdata/named_token_log.json`, which holds all five;
- QA's "qaTT: the TS dump of texts holding < > & and U+2028 is Go's bytes" in
  `client/test/qa-token-texts.test.ts`.

`TestTheDumpOfANamedTokenCarriesItsTexts` in `cmd/vtt` holds the committed
dump to `writeDump`'s bytes, so the first test compares against what Go
writes.

## Open debt

**`migrateLocked`'s re-read error arm is unreachable through `testdb`.**
`migrateLocked` in `internal/identity` re-reads the table's shape under the
write lock and wraps a failure there as `identity: migrate:`; nothing holds
that arm. `testdb.Arm` is one-shot and substring-matched, and
`migrationPending` runs the identical `PRAGMA` first, so the one armed fault is
always spent before the re-read. Labels: `test data missing`, `outside the
tool`. Closing it needs a fault that fires on the second matching statement.
Recorded 2026-09-24, moved here from the comment at the arm.

**`TestVerifyUsesConstantTimeCompare` reads `identity.go` as text.** It passes
while the string `subtle.ConstantTimeCompare` appears anywhere in the file;
today it appears three times, in `Verify`'s code, in `JoinAdmits`' code and in
`Verify`'s doc comment. Removing the call from `Verify` alone leaves it green
because the other two remain, and a comment alone would satisfy it with no
call at all. The branch it
guards is unreachable by construction, since the row is selected by the same
hash it then compares, so no behavioural test can observe the compare; the test
pins the call's presence and nothing finer. Labels: `test asserts nothing`,
`outside the tool`. Recipe: in `Verify`, replace the `subtle.ConstantTimeCompare`
call with `bytes.Equal`; the doc comment and `JoinAdmits` keep the identifier,
and the test and the package stay green. Closing it needs a check that finds the call inside
`Verify`'s own body, by `go/ast` or by a reading. Recorded 2026-09-24 by the
identity sweep, which changes no code line; the fix waits for the next ticket
that touches the package's tests.

**A test that cites a row marked `OPEN` passes the chain gate.**
`check:requirements-chain` refuses a citation with no row and an evidence entry
that resolves to nothing; it does not notice a row whose evidence says no test
holds it while a test cites it. Found by QA on 2026-09-23 and left open: the
fix is one more refusal in `tools/check-requirements-chain.py`, written when an
arc produces the case.

**The oracle corpus contains no non-party viewpoint.** `MayPerch` produced every
seat in it, so any rule that only diverges for a non-party or not-yet-existing
viewpoint is invisible to all eleven. `TestARefusedLookStillFillsTheRoster`
covers the one such rule known today; the corpus limit itself is unchanged, and
the next rule of that shape will need its own hand-built fixture.

**A WebSocket connection can outlive `Shutdown` and see its handles closed under
it.** `composeServer` in `cmd/vtt/serve_compose.go` returns a close func that
closes the identity and campaign handles. `http.Server.Shutdown` waits for
active HTTP handlers and not for hijacked connections, which every WebSocket
connection is, so a connection still reading or writing when `Shutdown`
returns runs against handles `closeFn` is about to close. `vtt serve`'s `RunE`
calls `Shutdown`, then `Close`, then `closeFn` regardless; the e2e test in
`cmd/vtt/serve_e2e_test.go` closes its one connection before `Shutdown` and so
never reaches the window. Labels: `test data missing`, `outside the tool`.
Closing it needs `Shutdown` to drain every gateway connection, or `closeFn` to
wait for them, and a test that holds a connection open across `Shutdown`.
Recorded 2026-09-27, moved here from the comment at `composeServer`.

**Closed 2026-10-15 by the owner's ruling, not by a change.** The gap stays.
It opens only when `vtt serve` is stopped while a client is connected, and
`main` returns once `RunE` has, so the process exits with the connection in
it. The ruling's ground is a reading of the code, not a test: `Campaign.Close`
takes the campaign's lock, which `Append` and `AppendBatch` hold for the whole
append, so an append already under way finishes before the log closes; a
command read after `closeFn` has closed the identity handle is refused with
"gateway: identity unavailable" (VTT-033); and one that passed `Lookup`
before then and appends after `Campaign.Close` gets the closed database's
error and is answered with `ok` false, as is an identity write that passed
`Lookup` and runs after the identity handle closes. A command the exit
overtakes never reaches the log. Should a table ever need the gap closed, the
sentence above beginning "Closing it needs" says what that takes.

**The busy skip's wiring in `serve` is unpinned.** `keepAlive` skips a tick
while `busy` reports the writer mid-frame, and `serve` wires that through
`stampedWrite` around its real write and `activity.busy` into
`pingUntilStopped`. VTT-094's three tests drive `keepAlive`, `writeActivity`
and `stampedWrite` in isolation; replacing `stampedWrite(&activity, ...)` in
`serve` with the bare closure, or passing a predicate that always answers false
in place of `activity.busy`, leaves them and the rest of the package green.
Labels: `test data missing`, `outside the tool`. Closing it needs a connection
test whose writer is held mid-frame across a tick and whose ping is observed
not to go out. Recorded 2026-09-27 by the reading review of SPEC-011.

**A `load_adventure` whose note key collides with a note upserted after its
snapshot overwrites it instead of refusing.** `handleCommand` in
`internal/gateway/server.go` runs `authorize`, which takes
`st := s.campaign.State()`, and hands that snapshot to `handleLoadAdventure`;
`adventure.Compile` runs `checkCollisions` against it and refuses a scene,
actor, token or note key the snapshot already holds. `campaign.AppendBatch`
then re-folds the batch against a fresh snapshot under its own lock, where
`engine.Apply`'s `SceneCreated`, `ActorAdded` and `TokenPlaced` arms refuse a
duplicate id and its `NoteUpserted` arm upserts; so an `upsert_note` on the
same key landing between the two calls is overwritten, while the three id
kinds are refused. Labels: `test data missing`, `outside the tool`: no
fixture holds an `upsert_note` between the two calls, and no mutation operator
opens the window. Recipe for the test that would show it: an internal test in
`internal/gateway` that takes `st := c.State()`, appends a `NoteUpserted` on
key K through `c.Append`, with the `EventId` `store.AppendBatch` requires, calls `s.handleLoadAdventure(..., st, ...)` for an
adventure declaring K (`goblin-ambush` declares `ravine-trail-warning`), and
asserts ok=false; today it answers ok=true and the
note is overwritten. Recorded 2026-09-27, moved here from the comment at
`handleLoadAdventure`. Since 2026-10-01 an adventure's notes load public
(`docs/superpowers/specs/2026-09-29-a-note-says-who-may-read-it-design.md`),
so the overwrite also replaces a DM-only note on that key with the
adventure's public one: the DM's text leaves the current state, the log keeps
it, and no frame a player or spectator is sent carries it.

**Closed 2026-10-15 by the owner's ruling, not by a change.** The overwrite
stays. It needs a second connection of the DM or the agent to upsert a note,
on a key the adventure declares, between one `load_adventure`'s `authorize`
and its `AppendBatch`; one connection's read loop answers a command before it
reads the next (SPEC-013). The ruling's ground is that such a write must land
within that window, on that key, and that a table has either a human or an
agent as its DM, which the gateway does not enforce. Should a table ever need
it closed, the check has to run under the campaign's lock in the same hold as
the append: re-reading the state in `handleLoadAdventure` before `AppendBatch`
only narrows the window, and the recipe's test passes on that all the same
(measured 2026-10-15 in a copy of `8217272`).

**VTT-149's door bound is unobserved from above on the y axis.** `mayWorkDoor`
in `internal/gateway/authz.go` accepts a player's door command when a
controlled token on the door's scene stands at most one square from it on each
axis. No test puts a token two squares from the door on the y axis and within
one on the x axis, so that bound is held from below and not from above.
Recipe: in `mayWorkDoor`, make `abs(tok.Y-at.GetY()) <= 1` read `<= 2`; every
test in `internal/gateway` stays green. The x bound made `<= 2` alone is red
in one test only, `TestAPlayerMayOnlyWorkADoorTheyAreNextTo`, over the wire.
The mutation gate rewrites operators and not constants, so it never meets
either edit. Labels: `test data missing`, `outside the tool`. Closing it needs
an `authz_test.go` door test with the token at (dx, dy) = (1, 2) and (2, 1),
each refused. Recorded 2026-09-28 by the breaks in
`docs/reports/2026-09-28-authorization-has-a-record.md`, whose change
(`8128a61`) changes no code line; the test waits for the next ticket that
touches the package's tests.

**Closed by** `TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis` in
`internal/gateway/authz_test.go`, which reds on the recipe above at its row
two squares off on the y axis, and on the x bound made `<= 2` at its other row
(observed 2026-10-11).

**`handleLoadMap`'s `Compile` refusal arm is driven by no test.** In
`internal/gateway/map.go`, a `mapdef.Compile` error is answered ok=false with
the error's text. Recipe: make that arm return `Ok: true` instead; `go test
./internal/gateway/` stays green, and so does `go test ./cmd/vtt/ -run
'Map|Art|Load'`. The arm is reachable: a map already in the set, whose art is
overwritten after its lookup with a sidecar declaring a `format_version` later
than this server understands, passes the lookup and is refused by the live
compile, and that refusal names the art and not the map. VTT-168's "or
compile" is observed only through `LoadInstalled`'s dry-run compile. The
mutation gate covers `internal/gateway`, but no gremlins operator rewrites a
boolean literal, so it never makes this edit. Labels: `test data missing`,
`outside the tool`. Closing it needs a `map_test.go` case that boots with a
map, overwrites one of its art sidecars with `format_version` 99, loads it,
and asserts ok=false with the art named and no path. Recorded 2026-09-28 by
the verification of
`docs/superpowers/specs/2026-09-28-loading-a-map-has-a-record-design.md`,
whose change changes no code line; the test waits for the next ticket that
touches the package's tests.

**Closed by** `TestAMapInTheSetWhoseArtStoppedCompilingIsRefused` in
`internal/gateway/map_test.go`, which reds on the recipe above: it boots with
the map in the set, overwrites one of its art sidecars with `format_version`
99, and loads the map (observed 2026-10-11). The rule is VTT-305.

**`perchBox.wake`'s capacity is unobserved.** In `internal/gateway/seat.go`,
`newPerchBox` makes `wake` with capacity 1, and `set` sends to it without
blocking. Recipe: make it `make(chan struct{})`; `go test -count=1
./internal/gateway/...` stays green. Unbuffered, `set`'s send is dropped
whenever the pump is not parked in its `select`, so a perch set while the pump
is delivering an event waits in the slot until the next hop, and a spectator
who hops once at a busy table stays on their old shoulder until they hop
again. The mutation gate does not change a `make` capacity. Labels: `test data
missing`, `outside the tool`. Closing it needs a test that sets one perch
while the pump is busy delivering, sends nothing else, and asserts the new
shoulder's frames arrive. Recorded 2026-09-29 by the Phase 4b review recorded
in `docs/reports/2026-09-29-the-seat-and-the-perch-have-a-record.md`, whose
change (`7be685a`) changes no code line.

**Closed by** `TestAPerchSetWhileNothingWaitsStillWakesThePump` in
`internal/gateway/viewpoint_internal_test.go`, which reds on the recipe above
in thirty runs of thirty (observed 2026-10-11). It holds the box: a perch set
while nothing waits on `wake` leaves a wake-up, and `take` then hands the
shoulder over (VTT-306). No test holds a connection whose pump is busy
delivering when the perch is set, which is what this entry asked for; the
owner ruled on 2026-10-11 that the box-level test closes it. Since `904f79b`
the recipe no longer leaves the package green on every run: it reds connection
tests that perch a spectator before the pump waits, and which ones varies by
run. The seven `TestQAMoveReasonOverTheWire...` tests that commit brought
redden rarely (19 red in 1,050 runs of them under the recipe, across all
seven); `TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause` and
`TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause`, which
`a0c48c9` brought, were each seen red in six to eight runs of eight.

**No test asks for a shoulder a spectator sat on and left.** SPEC-015 states
that naming a shoulder again restores it;
`TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt` observes that only
for a shoulder a burst flew past, which the projector's memory never held.
Recipe: give `Projector.reperch` a per-projector set of shoulders already
served and return nil for an `actorID` in it; `go test -count=1
./internal/gateway/...` stays green, although a spectator who hops from Asme
to Armak and back to Asme would then be sent no frame on the way back, and the
creatures Asme sees would stay hidden. The mutation gate does not add state.
Labels: `test data missing`, `outside the tool`. It moves here from
`TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt`'s doc at `74c547f`,
which this change cut; the review re-ran it. Closing it needs a test that
perches on a shoulder, hops away, hops back, and asserts the first shoulder's
board is served again. Recorded 2026-09-29 by the Phase 4b review recorded in
`docs/reports/2026-09-29-the-seat-and-the-perch-have-a-record.md`.

**Closed by** `TestAShoulderSatOnAndLeftIsRestoredByHoppingBackToIt` in
`internal/gateway/viewpoint_internal_test.go`, which reds at once on the
recipe above (observed 2026-10-11); the rule is VTT-195's. Since `ce31229` the
recipe also reds `TestQANoteProjectionAPerchChangesNothingAboutNotes` in
`internal/gateway/qa_note_projection_test.go`, by its 30-second deadline: that
test's fourth perch names its first shoulder again.

**A visible set that becomes another of the same size is unobserved.** In
`internal/gateway/project.go`, `transitions` sends a scene's `SceneSeen` only
when `sameSet` finds the squares a viewer sees there now differ from those
last sent. Recipe: in `sameSet`, make `if !b[k] {` read `if !b[k] && false {`;
`go test -count=1 ./internal/gateway/... ./cmd/vtt/...` stays green. With it,
a viewer whose visible squares in a scene change to as many other squares, a
token stepping along a corridor whose count of visible squares holds, is sent
no `SceneSeen`, and its client keeps the old lit area while the tokens it is
sent move on. VTT-201 states the rule and is OPEN. The mutation gate does not
drop a comparison from a condition. Labels: `test data missing`, `outside the
tool`. Closing it needs a test that moves an eye so its count of visible
squares holds while the set changes, and asserts a `SceneSeen` carrying the
new set. Recorded 2026-09-29 by the verification of
`docs/superpowers/specs/2026-09-29-the-projection-has-a-record-design.md`,
whose change changes no code line.

**Closed by** `TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain`, in
`internal/perceive/board_test.go` since it moved from
`internal/gateway/project_test.go`, which reds on the recipe above and is
VTT-201's evidence (observed 2026-10-11). Since `390239a` the recipe also reds
`TestQATestimonySightEveryPairOfStepsKeepsEachSeatToWhatItSees`,
`TestQATestimonySightEveryRandomWalkKeepsEachSeatToWhatItSees` and
`TestQATestimonySightAChangeUndoneOutOfSightSendsNothingOnSight` in
`internal/gateway/qa_testimony_sight_test.go`, in eight runs of eight.

**No test sends a spectator an event whose payload the build does not know.**
In `internal/gateway/project.go`, `Project` sends nothing for an
`unrecognised` verdict, to a player and a spectator alike. Recipe: make `if v
== unrecognised {` read `if v == unrecognised && pr.viewer.Role ==
identity.RolePlayer {`; `go test -count=1 ./internal/gateway/...` stays
green, although a spectator would then be sent `transitions`' frames for an
event the projection cannot read. VTT-227 states the rule and is OPEN;
`TestAnUnrecognisedPayloadIsWithheldFromAPlayer` holds VTT-220, the player's
half, alone. The mutation gate does not add a condition. Labels: `test data
missing`, `outside the tool`. Closing it needs that test's shape for a
spectator perched on a party member. Recorded 2026-09-29 by the Phase 4b
review recorded in `docs/reports/2026-09-29-the-projection-has-a-record.md`.

**Closed by** `TestAnUnrecognisedPayloadIsWithheldFromASpectator`, in
`internal/perceive/roster_test.go` since it moved from
`internal/gateway/project_test.go`, which reds on the recipe above and is
VTT-227's evidence (observed 2026-10-11).

**No golden log carries an actor's removal, so the keystone holds none.**
`TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees` runs
`keystoneRemovalDiff` at every prefix of every golden under
`scenarios/goldens/`, and none of those logs holds an `ActorRemoved`
(`grep -l actorRemoved scenarios/goldens/*/stream.json` prints nothing), nor
reuses an actor id, so the keystone's removal clause and its ghost handling
in `keystoneStatusDiff` run over nothing. The rule that a viewer is told of a
removal only when it saw the actor before it (VTT-251) is held by
`project_property_test.go#TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer`,
whose eventgen walk removes actors, and by unit tests; a reused id
(VTT-252) by unit tests alone, since eventgen never reuses one. Labels:
`test data missing`. Closing it needs a scenario that removes an actor some
seats saw and others did not, and one that adds an actor under a removed id.
Recorded 2026-10-01 by the testimony change's reading review.

**Closed 2026-10-17 by deletion, not by a test.** The keystone and
`project_property_test.go` left with the gateway's projection in ticket 3b
(`docs/superpowers/specs/2026-10-17-a-seat-streams-its-characters-log-design.md`),
so no gate runs a removal clause over the corpus any more. VTT-251 and VTT-252
are held by `internal/perceive`'s unit tests, which their rows name; the
corpus still holds no `ActorRemoved` and reuses no actor id.

**`check:doc-owner` does not read test files, so a test can take another's
doc comment.** `tools/check-doc-owner.py` skips every file ending
`_test.go`. A new test inserted directly above an existing test's `// VTT-NNN`
line lands between that test's doc block and its function: the block then
documents the new test and the old one keeps only its citation, and the gate
prints "every doc comment sits on its own function". Recipe: in any
`_test.go`, put a new `func TestX` with its citation line directly above
another test's citation line. It happened in
`internal/gateway/server_visibility_test.go` in the note-visibility change and
in `internal/gateway/project_test.go` in this one, and the reading review
caught both. Labels: `outside the tool`. Closing it needs the checker to read
test files, which may find findings older than this entry. Recorded 2026-10-01
by the testimony change's report review.

**Nothing observes a removal batch's participant and role, nor that a refused
`set_join_door` leaves an open door open.** `handleRemoveActor` in
`internal/gateway/server.go` stamps every envelope of its batch with the
issuer's `ParticipantId` and `ActorRole`, and `handleJoinDoor` refuses a door
that says neither open nor closed before it calls `SetJoinOpen` (VTT-256).
Recipes: leave the stamping loop's `ParticipantId` and `ActorRole` assignments
out; or call `s.ids.SetJoinOpen(false, 0)` in `handleJoinDoor`'s refusal arm
before it returns. Under each, `go test -count=1 ./internal/gateway/...` stays
green (probes P8 and P9b of
`docs/superpowers/plans/2026-10-02-removing-an-actor-has-a-record.md`), and no
test under `cmd/vtt` drives `remove_actor` or `set_join_door` (that plan's
check 3). `TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt` reads the door
after its refusal, but from a shut start, so a refusal that shuts the door
cannot change what it sees. The tier-1 tests should have caught both; no
mutation operator deletes an assignment or adds a call. Labels: `test data
missing` and `outside the tool` for the stamping; `test asserts nothing` for
the door. Closing it needs a removal test that reads the batch's envelopes
back for their participant and role, and a door test that opens the door,
sends an unspecified door, and asserts the door is still open. Recorded
2026-10-02 by the removal record's report.

**Closed by** `TestARemovalBatchCarriesItsIssuersParticipantAndRole` in
`internal/gateway/remove_actor_test.go`, which reds on the first recipe above
(VTT-307), and by `TestAnUnspecifiedDoorLeavesAnOpenDoorOpen` in
`internal/gateway/server_test.go`, which starts from an open door and reds on
the second (observed 2026-10-11).

**Nothing ties a byte bound's copies to the engine's constant.**
`client/src/fold.ts` holds each of the fold's twenty-four bounds as a literal
in a `checkLen` call, `internal/adventure/load.go` holds its own copies of
`maxNoteKeyBytes`, `maxNoteTitleBytes`, `maxTextBytes`, `maxNameBytes` and
`maxIDBytes`, `internal/mapdef/load.go` its own `maxNameBytes` and
`maxIDBytes`, the second also bounding a scene object's id for both loaders
through `CheckObjectIDs`, and `internal/rules/load.go` its own `maxIDBytes`,
which the ruleset schemas state too; each side's tests pin its own numbers
(`fold-rejections.test.ts`, `TestSizeCapsMirrorEngine`,
`TestTheNameBoundMirrorsEngine`, `TestTheIDBoundMirrorsEngine` in
`internal/adventure` and in `internal/rules`, `TestTheSchemasStateTheIDBound`,
`TestAMapNameOfExactlyTheBoundLoads`, `TestAMapIDOfExactlyTheBoundLoads`,
`TestAnObjectIDOfExactlyTheBoundLoads`), so a bound changed in
`internal/engine/apply.go` alone leaves every mirror stale and every mirror's
own tests green, save that the tests in `internal/rules` and
`internal/adventure` that fold a loader's at-bound output through
`engine.Apply` (`TestARulesetAtTheBoundResolvesToEventsTheFoldAccepts`,
`TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts`,
`TestQAIDAnAdventureWhoseIDsAreAtTheBoundLoadsCompilesAndFolds`,
`TestQACmdAnAdventureSceneWithObjectIDsUpToTheBoundLoadsCompilesAndFolds` and
`TestQACmdAMapWithObjectIDsUpToTheBoundCompilesAndFolds`) red when the
engine's id bound falls below the loaders' copies. Only a move's reason, a
name, an id and an actor's module id and keys have a link, from the MCP tools'
descriptions to `maxMoveReasonBytes`, `maxNameBytes` and `maxIDBytes`
(`TestTheMoveToolStatesTheFoldsReasonBound`,
`TestTheToolsStateTheFoldsNameBound`, `TestTheToolsStateTheFoldsIDBound`,
`TestTheToolsStateTheFoldsBoundOnAnActorsModuleAndKeys`). Recipe: set
`maxNoteTitleBytes` to 300 in `apply.go`; `go test ./internal/adventure/...`
and `bun test client/test` stay green. No gate compares a copy with its
constant, and no mutation operator changes two files. Labels: `outside the
tool`. Closing it needs a test that reads each mirror's number against the
engine's. Recorded 2026-10-04 by the reason-bound report.

**A ruleset's names reach a `use_ability` refusal at any length.**
`rules.Load` bounds no ability id and no resource, attribute or defense name,
and `rules.Resolve` interpolates them into its refusals with `%q` (`has no
resource %q required by ability %q's usage cost` and its siblings);
`handleUseAbility` in `internal/gateway/ruleset.go` returns `err.Error()` as
the result's error, so a ruleset author's text of any length reaches the
issuer unclipped, where the map and art paths pass theirs through
`artlib.Clip` or `artlib.BoundErr`. `internal/artlib/artlib.go`'s const-block
doc listed this path as outstanding until the id bound cut that doc. Recipe:
in a copy of `internal/rules/conformance/testdata/minimal-smoke-fail`, replace
`"focus"` in `ruleset.json` and `abilities/big-move.json` with a 70,000-byte
name; `rules.Load` accepts it, and `rules.Resolve` of `big-move` by an actor
without that resource returns a 70,089-byte error (measured 2026-10-06 on
`b9de835`'s `internal/rules`). No gate measures a refusal's size. Labels:
`outside the tool`. Closing it needs a bound on a ruleset's names at load, or
`Clip` at `Resolve`'s interpolations, and a test that reads the refusal off a
real connection. Recorded 2026-10-06 by the id bound's review.

**Closed by** `TestARulesetNameOfSeventyThousandBytesIsRefusedAtLoad` in
`internal/rules/load_test.go`, which reds on the recipe above, and by
`TestAThresholdRefusalNamesItsPositionNotItsExpression` in
`internal/rules/resolve_test.go` and
`TestAThresholdRefusalReachesTheIssuerWithoutItsExpression` in
`internal/gateway/ruleset_test.go`, which hold the one path a bound on names
leaves, a threshold's expression in a refusal, the second off a real
connection (observed 2026-10-06).

**Nothing in the gate builds for another platform.** `task check`, the
pre-commit and the pre-push hooks build and test for the machine they run on,
and none sets `GOOS` (`grep -rn GOOS` over `Taskfile.yml`, `.lefthook.yml`
and `.github` prints nothing); CI runs only when dispatched by hand. Since
`855fafd`, `internal/campaign` calls `syscall.Flock` and does not build for
`GOOS=windows`, which SPEC-019 records, and nothing would report that
changing, or another platform breaking. Recipe: add `_ =
syscall.F_FULLFSYNC` as the first line of `takeHold` in
`internal/campaign/campaign.go`; `go build` and `go vet` pass on darwin, and
`GOOS=linux go build ./internal/campaign/` fails with `undefined:
syscall.F_FULLFSYNC`. The gate should have caught it, and builds for one
platform only. Labels: `outside the tool`. Closing it needs a gate step that
builds `./...` for linux and darwin and asserts the windows build of
`internal/campaign` fails as SPEC-019 says, or a ruling that it is not
wanted. Recorded 2026-10-05 by the writer-hold report, the plan's Gap 5.

**`AttackRolled.Modifier.source` reaches a player as the log holds it.**
`forwardable` in `internal/perceive/projection.go` clears an envelope's issuer
and the cause of a `TokenMoved`, `ResourceChanged`, `ConditionApplied` or
`ConditionRemoved` and keeps every other field, and `classify` forwards an
`AttackRolled` when the viewer saw its attacker and its target, so each
`Modifier.source` reaches a player or a spectator as written. Nothing in
production writes an `AttackRolled` today (`git grep -n 'AttackRolled' --
'*.go' ':!*_test.go' ':!contract/gen' ':!contract-spike'` prints only
`engine.Apply`'s and `classify`'s arms, which read it), so no table has met it; a writer that sets a
modifier's source to an ability or an actor the viewer does not see would name
it. Recipe: project an `AttackRolled` whose attacker and target a player sees,
with `Modifiers: []*vttv1.Modifier{{Source: "ability:ambush:hit", Value: 2}}`;
the player's frame carries the source. QA's
`TestQAIssAForwardedFrameKeepsEverythingButIssuerAndCause` does exactly this
with the source `brawn` and asserts it is kept, so the behaviour is pinned as
correct, not merely unobserved. No gate should have caught it, and none
did: no rule says what a modifier's source names or who reads it (VTT-273 is
status changes only), and `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable`
walks the envelope's own fields, not a payload's. Labels: `spec silent`.
Closing it needs a ruling on `Modifier.source` before anything writes an
`AttackRolled`; if it is cleared, an arm in `forwardable` with its test, and
`qaIssLessIssuerAndCause` and that test's modifier assertion changed with it,
since today four QA tests red under that arm. Recorded 2026-10-05 by the issuer
report, from QA's finding and Gap 7 of
`docs/superpowers/plans/2026-10-05-a-viewer-is-told-no-issuer-and-no-unseen-cause.md`.

**The adventure loader's three empty-name refusals are pinned by no test.**
`internal/adventure/load.go` refuses an adventure, a scene and an actor whose
`name` is empty ("must not be empty") in `loadManifest`, `loadScenes` and
`loadActors`, and no fixture under `internal/adventure/testdata/invalid`
carries an empty name. Recipe: delete `loadManifest`'s `raw.Name == ""`
branch; `go test ./internal/adventure/...` stays green (observed 2026-10-06).
The tier-1 tests should have caught it and have no case for an empty name.
Labels: `test data missing`. Closing it needs three invalid fixtures, one per
file, each with an empty name, their rows in `TestLoadInvalidFixtures`, and
their names in `TestLoadInvalidFixturesCatalogueIsComplete`'s list.
Recorded 2026-10-06 by the name-bound plan's Gap 6, by the owner's ruling on
its Q7.

**A test run that prints more than 1 MiB is a killed mutant.** Stryker's
command runner (`node_modules/@stryker-mutator/core`'s
`command-test-runner.js`) starts the test command with `exec` and no
`maxBuffer`, so Node kills a run whose output passes its 1 MiB default, the
exit code is null, and Stryker counts a null exit as a failed run, which is
Killed; `tools/check-ts-mutation.py` reads only the status. The 2026-10-06
report held 23 killed mutants whose output stops between 1,049,251 and
1,081,428 bytes with no `(fail)` line: 21 show an assertion failure that
printed a DOM element before the cut, and for two, a `BooleanLiteral` in
`client/src/view/player.ts` and a `StringLiteral` in `client/src/app.ts`, the
output stops among passing tests, so the report does not show what killed
them. `--only-failures` in `stryker.conf.json`'s command makes a passing run
print almost nothing, which leaves a test that prints a great deal on its own.
Recipe: add to any file under `client/test` a test that passes after
`console.log("x".repeat(1_100_000))`; every mutant `check:ts-mutation` runs is
then Killed, the surviving ones included, and the gate is green (not run). The
gate should have caught it, and trusts the exit code it is given. Labels:
`outside the tool`. Closing it needs `maxBuffer` raised through Stryker's
configuration if it offers one, or a check in `tools/check-ts-mutation.py`
that refuses a killed mutant whose output holds no failing test. Recorded
2026-10-08 by the command-id report, from the review of the `--bail` change.

**The contract's doc comment on `PromoteParticipant` says it is the only
command that appends no event.** `contract/vtt/v1/commands.proto` documents
`PromoteParticipant` as "the only command that changes IDENTITY rather than
campaign state, and so the only one that produces NO EVENT", and the comment
is generated into `contract/gen/go/vtt/v1/commands.pb.go` and
`contract/gen/ts/vtt/v1/commands_pb.ts`. SPEC-007 lists four commands that
append nothing, `set_viewpoint`, `set_join_door`, `rotate_join_link` and
`promote_participant`, and `set_join_door` and `rotate_join_link` write the
identity store too, through `identity.DB.SetJoinOpen` and
`identity.DB.RotateJoinSecret`. It is in the tree; nothing puts it back. No
gate reads a comment in the contract against SPEC-007. Labels: `outside the
tool`. Closing it needs the comment corrected in `commands.proto` and the
generated code regenerated with `task generate:contract`, as a contract change
of its own. Recorded 2026-10-14 by the promotion report, from its QA agent's
finding, on the owner's ruling.

**After a dropped frame set, a restart handed the world before the event
goes unseen.** When a character's frames do not fold onto its log,
`fanOutLocked` in `internal/campaign/fanout.go` resumes its decision with
`perceive.Resume(eyesOf(id), l.fold, world)`, the world after the event. The
world before would let `classify` forward a status event about an actor that
has just left sight, from the previous `sighted`. Recipe: replace `world`
with `before` in that call; the campaign and gateway suites stay green.
Labels: `test data missing`. It needs a drop at an event that changes the
character's sight, which no forged drop reached; closing it needs such a
fixture under VTT-320. Recorded 2026-10-16 by the each-party-member-keeps-
its-own-log ticket's Task 8 review.

**A failed resume lead in `fanOutLocked` is unreachable and unheld.** When a
paused log resumes, its session bridge and hides are folded first, and the
branch for a lead that does not fold leaves the log paused. No history
reaches it: each entry folds by construction (the bridge ends a session only
when the log holds one open and starts one only after that end or when it
holds none, and `TokenHidden` is a no-op for an absent token). Were it
reached, the pause would exist only in memory, and a reopen would resume the
log through `perceive.Resume` without its bridge or hides. Labels:
`outside the tool`. Closing it needs a fault inside the fold of a resume's
lead. Recorded 2026-10-16 by the same ticket's Task 6 review.

**`queueFrame`'s encode-failure close is unpinned.** In
`internal/gateway/server.go`, `queueFrame` closes the connection with
`StatusInternalError` and "gateway: encode failed" when the frame it is handed
cannot be encoded, as SPEC-011 says, and `sendViewpoint` did the same on
`0db3d18`. Recipe: delete the `if !closing.Load()` block in `queueFrame`'s
error arm, leaving its `return false`; `go test ./internal/gateway/` stays
green. Only `TestCatchUpHeadEncodeFailureClosesTheConnection` and
`TestAnEncodeFailureTearsTheConnectionRatherThanTheBatch` swap `encodeFrame`,
and neither fails a frame `queueFrame` is handed. Labels: `test data missing`.
Closing it needs a `server_internal_test.go` case whose `encodeFrame` fails a
`WatchableChanged` or a `ViewpointChanged` and asserts that close, with no
frame after it. Recorded 2026-10-21 by the reading review of the
specification step of
`docs/superpowers/specs/2026-10-20-the-client-follows-the-log-it-is-sent-design.md`.
