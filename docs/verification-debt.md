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
key K through `c.Append`, with the `EventId` `store.Append` requires, calls `s.handleLoadAdventure(..., st, ...)` for an
adventure declaring K (`goblin-ambush` declares `ravine-trail-warning`), and
asserts ok=false; today it answers ok=true and the
note is overwritten. Recorded 2026-09-27, moved here from the comment at
`handleLoadAdventure`. Since 2026-10-01 an adventure's notes load public
(`docs/superpowers/specs/2026-09-29-a-note-says-who-may-read-it-design.md`),
so the overwrite also replaces a DM-only note on that key with the
adventure's public one: the DM's text leaves the current state, the log keeps
it, and no frame a player or spectator is sent carries it.

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
