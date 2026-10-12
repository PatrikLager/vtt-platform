# A gateway behaviour a recorded recipe breaks goes red in a test that names it

## The problem

Seven entries under `Open debt` in `docs/verification-debt.md` each give an
edit to `internal/gateway` that changes what a table is shown or allowed, and
each says the package's tests stay green under it. Each edit was applied to a
clone of `d64cc9c` on 2026-10-11 and `go test -count=1 ./internal/gateway/...`
run under it. Four entries hold as written:

- `mayWorkDoor` (`authz.go`), the y bound `<= 1` made `<= 2`: green. A player
  works a door from two squares north or south of it. The x bound made `<= 2`
  reds `TestAPlayerMayOnlyWorkADoorTheyAreNextTo` in `server_test.go`, over
  the wire, and no test in `authz_test.go`, where VTT-149's other tests stand.
- `handleLoadMap` (`map.go`), the `mapdef.Compile` refusal arm answering
  `Ok: true`: green, and so is `go test ./cmd/vtt/ -run 'Map|Art|Load'`. A map
  the set already holds, whose art has since been overwritten by a sidecar of
  a later format, is then answered ok with nothing appended.
- `Project` (`project.go`), its `unrecognised` arm narrowed to players: green.
  A spectator is then sent `transitions`' frames for an event the projection
  cannot read. VTT-227 states the rule, its row is `OPEN`, and no test file
  carries the id.
- `handleRemoveActor` (`server.go`), the `ParticipantId` and `ActorRole`
  assignments left out of its stamping loop: green, so a removal's events can
  reach the log naming nobody. And `handleJoinDoor` (`server.go`), shutting
  the door in its refusal arm: green, because
  `TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt` reads the door after
  the refusal from a shut start.

Three no longer hold as written. Each recipe reds tests newer than its entry,
none of which states the behaviour the recipe breaks:

- `newPerchBox` (`seat.go`), `wake` unbuffered: reds connection tests that
  perch a spectator, and which of them varies by run.
  `TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause` and
  `TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause` were each
  red in six to eight runs of eight, and in a further package run the second
  stayed green. Both are about the issuer and the cause a perched spectator
  is sent; `set`'s send is dropped only while the pump is not waiting on
  `wake`, so each passes under the recipe whenever its perch reaches the box
  while the pump waits.
- `Projector.reperch` (`project.go`), answering nothing for a shoulder it has
  served before: reds `TestQANoteProjectionAPerchChangesNothingAboutNotes`,
  a test of notes that waits for a lit `SceneSeen` as its marker that a perch
  landed, and fails by its 30-second timeout.
- `sameSet` (`project.go`), its membership test disabled: reds
  `TestQATestimonySightEveryPairOfStepsKeepsEachSeatToWhatItSees`,
  `TestQATestimonySightEveryRandomWalkKeepsEachSeatToWhatItSees` and
  `TestQATestimonySightAChangeUndoneOutOfSightSendsNothingOnSight`, eight runs
  of eight. VTT-201 states the rule, its row is `OPEN`, and no test file
  carries the id.

The verification of this ticket found a gap of another kind, and the owner
ruled it in on 2026-10-11. The seven `TestQAMoveReasonOverTheWire...` tests in
`qa_move_reason_test.go` can fail on a tree that is right. `qaMRConn.command`
reads frames until its result arrives. `qaMRStartLive`, after its perch, and
`qaMRLive.barrier`, after its narration, then wait with `qaMRConn.until` for a
frame that command caused, and `until` looks only at frames not yet read.
SPEC-007 leaves a result unordered against its command's events, so when the
event arrives first the wait ends at the fixture's 30-second deadline. With
`time.Sleep(50 * time.Millisecond)` added directly after `result :=
s.answerCommand(now, err, cmd, perches)` in `serve` (`server.go`), the seven
fail and no other test in the package does.

Not in this ticket: every other `Open debt` entry, the gateway's among them
(the busy skip's wiring in `serve`, the `load_adventure` note-key overwrite, a
connection that outlives `Shutdown`, `Modifier.source`, a golden log with a
removal), and any change to a non-test file under `internal/gateway`.

## Done looks like

1. With `mayWorkDoor`'s y bound made `<= 2`, and separately its x bound, a
   test in `internal/gateway/authz_test.go` goes red. On today's tree neither
   edit reds a test in that file.
2. With `handleLoadMap`'s `Compile` refusal arm answering `Ok: true`, a test
   in `internal/gateway/map_test.go` goes red. It loads a map the set already
   holds whose art has been overwritten by a sidecar declaring a later
   `format_version`, and asserts ok=false, the art named, no path on the
   server, and nothing appended.
3. With `wake` unbuffered, a test in
   `internal/gateway/viewpoint_internal_test.go` goes red on every run. It
   asserts the hand-off itself: a perch set while nothing waits on `wake`
   still wakes the pump.
4. With `reperch` answering nothing for a shoulder served before, a test in
   `internal/gateway/viewpoint_internal_test.go` goes red at once, not by a
   timeout. It perches on a shoulder, hops to another, hops back, and asserts
   the first shoulder's room is lit again and its token placed again.
5. With `sameSet`'s membership test disabled, a test in
   `internal/gateway/project_test.go` that carries VTT-201 goes red, and
   VTT-201's row names it.
6. With `Project`'s `unrecognised` arm narrowed to players, a test in
   `internal/gateway/project_test.go` that carries VTT-227 goes red, and
   VTT-227's row names it.
7. With the two assignments left out of `handleRemoveActor`'s stamping loop, a
   test in `internal/gateway/remove_actor_test.go` goes red.
8. With `handleJoinDoor`'s refusal arm shutting the door, a test in
   `internal/gateway/server_test.go` goes red. It starts from an open door.
9. On the unbroken tree every new test passes, and `git diff --stat` against
   `d64cc9c` names no non-test file under `internal/gateway`.
10. Each of the seven entries in `docs/verification-debt.md` carries a
    `**Closed by**` paragraph naming the test that holds it; the three whose
    recipe already reddened say which tests it reddened and since when.
11. `task check` whole is green.
12. With that sleep in `serve`, `go test -count=1 ./internal/gateway/` is
    green; on today's tree it fails the seven `TestQAMoveReasonOverTheWire...`
    tests. And the seven still tell a leaked reason from none: without the
    sleep, and with `forwardable` (`project.go`) keeping a `TokenMoved`'s
    reason, tests among them go red.

## Rules this puts on the system

Existing rules this adds evidence to, by id: VTT-149, VTT-168, VTT-195,
VTT-201, VTT-227, VTT-256.

Rules no row states today, in their own words, unnumbered until the sort:

- A `load_map` whose map no longer compiles when it is loaded is refused and
  appends nothing.
- A perch set while its connection's pump is not waiting for one is applied
  without another perch.
- Every event a `remove_actor` appends carries the participant id and role of
  whoever issued it.

## What it touches

1. `internal/gateway/authz_test.go`, `internal/gateway/map_test.go`,
   `internal/gateway/viewpoint_internal_test.go`,
   `internal/gateway/project_test.go`,
   `internal/gateway/remove_actor_test.go`, `internal/gateway/server_test.go`;
   `internal/gateway/qa_move_reason_test.go` (`qaMRConn.until`,
   `qaMRConn.command`).
2. `docs/verification-debt.md`; `docs/requirements.md`.
3. `tools/comment-ceilings.txt`, if a test file's comment share falls.
4. The specifications below, their `Requirements` lists only.

One component, and up to thirteen files when each specification below gains an
id; the owner ruled on 2026-10-11 that it stays one ticket.

## Specifications this moves

Each gains an id in its `Requirements` list if the sort allocates one under
it; none changes what it says the system does.

docs/specifications/014-loading-a-map.md
docs/specifications/015-the-seat-and-the-perch.md
docs/specifications/017-removing-an-actor.md

## What could not be established

- Whether a new test finds the behaviour it pins absent. Each recipe was read
  against the code it edits and each behaviour is there to the eye; a test
  written to pass that is red on the unbroken tree is a defect, and the plan
  stops there.
- Whether `TestQANoteProjectionAPerchChangesNothingAboutNotes` reds under the
  `reperch` recipe on every run. It was run once; the wait it fails on does
  not depend on timing as read.
- How often the seven move-reason tests fail on an unbroken tree. One failed
  once in the verifier's package runs and none in the writer's.
- Which of the three unstated rules the sort keeps as rows of their own, and
  whether the hop back to a shoulder is VTT-195's or a rule of its own.
