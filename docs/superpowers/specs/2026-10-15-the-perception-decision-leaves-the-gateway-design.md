# The perception decision leaves the gateway and resumes from a fold

## The problem

What a player or a spectator is sent of each event is decided by the
`Projector` in `internal/gateway/project.go` (SPEC-016). `newSeat` in
`internal/gateway/seat.go` gives each projected connection its own, and
`seat.receive` feeds it every envelope from the log's first, folds the whole
received prefix with `campaign.FoldPrefix`, and hands `Project` the event and
the state after it. The `Projector` remembers what it has sent in nine maps,
`scenes`, `actors`, `tokens`, `seen`, `doors`, `notes`, `sighted`, `belief`
and `gone`. `NewProjector` makes them empty, and only `Project` and `reperch`
change them, so a `Projector` knows what its viewer holds only by having been
fed the log from the first event, and its doc comment says never to seed one
from a snapshot. The maps live as long as the connection does.

The per-character-logs design,
`docs/superpowers/specs/2026-08-30-per-character-logs-design.md`, makes the
same decision at append time: inside `campaign.Append` and
`campaign.AppendBatch`, for every party member, in the transaction that writes
the event (§4.1, §4.2). It keeps each character's memory as the fold of that
character's own log, rebuilt by folding the log on open (§4.3), because a
character log is never regenerated from the world (§3.3). Two things in the
tree stop that. `.go-arch-lint.yml` lets `campaign` depend on `contract`,
`store`, `engine`, `campaign` and `eventgen`, and the decision lives in
`gateway` and calls `sight`. And no `Projector` can be built from what a
viewer holds: nothing derives its maps from a fold.

Seven of the maps record what the viewer was sent, and the fold of those
frames holds the same. Two do not. `sighted` is the last look's `sees`, which
the world and the eyes give again. `gone` marks an actor the world removed
while no eye saw it, and no fold or state tells that apart from an actor
never removed once the world adds one again under the same id. And `belief`
keeps an actor's conditions and controllers in the world's order where the
viewer's fold keeps its own. So in three histories a decision rebuilt from
the fold and the world would send other frames than one fed from the first
event: an actor added again, unchanged, under an id the viewer still holds;
a correction that removes several conditions; and a correction that restates
a controller to carry a changed kind. VTT-211 says an actor id the world uses
again is introduced afresh, which the first history contradicts.

The owner ruled on 2026-10-15 that the arc is four tickets in order, and this
is the first. The decision moves, and what every viewer is sent stays as it
is: a player still sees through every actor it controls, and a spectator
through the party member it perches on, until the third ticket gives each
connection one character at a time. The owner ruled the same day that the
viewer's fold is the decision's memory at every event, not only on a resume,
so that the three histories send what the fold and the world give, and each
must leave the viewer's fold as it leaves it today. One variant falls short
of that, and the owner accepted it at sign-off the same day: an actor added
again unchanged, with conditions or controllers that cross the re-add, is
left equal as sets, its conditions possibly in another order. What MapTool
does here, and what this arc borrows from it and refuses, is the section
"Rule 9: how MapTool solves this" in
`docs/superpowers/plans/2026-09-14-per-character-logs.md`.

## Done looks like

1. The decision is a package that `campaign` could depend on.
   `internal/perceive` exists, and `.go-arch-lint.yml` names a `perceive`
   component that may depend on `contract`, `engine`, `sight` and itself and
   on nothing else. `go-arch-lint check` is green, and goes red when a file
   under `internal/perceive`, a test file included, imports
   `internal/gateway`, `internal/identity`, `internal/campaign` or
   `internal/store`.
2. The gateway no longer decides. `grep -n 'func (pr \*Projector)
   \(classify\|look\|transitions\)' internal/gateway/*.go` prints nothing,
   where on `main` it prints three lines.
3. What the corpus sends is unchanged. `git diff --stat main --
   scenarios/goldens` prints nothing, and
   `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`,
   `TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees` and every
   test that cites a row of SPEC-015's or SPEC-016's Requirements pass,
   wherever they now live. `task check:requirements-chain` is green.
4. The decision resumes from what its viewer holds. A new test decides each
   history below twice at every prefix: once continuing from the first event,
   and once starting over from the fold of the frames sent up to that prefix
   and the world state at it. Both send the same frames for every later
   event. The histories are every oracle seat (each `viewer.json` under
   `scenarios/goldens/*/projections/` whose role is `spectator` and whose
   `viewpoint` is not empty), seeds of `internal/eventgen` with player
   seats, spectator seats and a spectator that perches as it goes, and the
   three histories of The problem. It cannot pass on `main`, where nothing
   builds a `Projector` from a fold.
5. The three histories are pinned. A test for each shows the frames the
   decision now sends, and that the viewer's fold after them equals the fold
   `main`'s projection leaves. An actor added again under an id the viewer
   holds, differing in a field only `ActorAdded` writes (another name or
   another module id among them), is still introduced afresh. VTT-211 is
   marked withdrawn in `docs/requirements.md`, succeeded by the third rule
   below, and SPEC-016
   says in what order a correction now sends removals and revocations and
   which controller it restates.
6. `internal/perceive` is held by the Go mutation gate and by a coverage floor
   in `tools/coverage-thresholds.txt`, through tests in its own package, and
   the adjudications in `tools/mutation-equivalents.txt` that name
   `internal/gateway project.go` name the code where it now stands, per
   SPEC-020.
7. `task check` whole is green.

## Rules this puts on the system

- No file of the perception decision, test files included, imports a package
  of this module but `contract`, `engine` and `sight`.
- A decision started over from the fold of what its viewer was sent, and the
  world, sends what one fed from the first event sends.
- An actor the world adds again after a removal is introduced afresh to a
  viewer that sees it when the viewer no longer holds the id, or holds one
  that differs in a field only `ActorAdded` writes, and is corrected
  otherwise.

SPEC-015's and SPEC-016's other rules are unchanged and keep their ids.

## What it touches

1. `.go-arch-lint.yml`, the `perceive` component, first: nothing under
   `internal/perceive` passes `check:arch` before it.
2. `internal/perceive`, new, and its own tests. Every test that kills a
   `project.go` mutant today is in `internal/gateway`, and the mutation gate
   and the coverage floor count a package's own tests only, so the tests that
   hold the moved code are ported into `internal/perceive`. That port is most
   of the work.
3. `internal/gateway/project.go` and `internal/gateway/seat.go`.
4. `internal/gateway/project_internal_test.go` and
   `internal/gateway/viewpoint_internal_test.go`, which call moved functions.
   The other gateway tests use `NewProjector`, `Viewer` and `Project`, and
   change only where those do.
5. `docs/requirements.md`: the rows whose tests move, VTT-211, and the rows
   the sort allocates.
6. `tools/check-mutation.py`'s package list, `tools/coverage-thresholds.txt`,
   `tools/mutation-equivalents.txt` and `tools/comment-ceilings.txt`.
7. SPEC-015, SPEC-016 and SPEC-018.

## Specifications this moves

docs/specifications/015-the-seat-and-the-perch.md
docs/specifications/016-the-projection.md
docs/specifications/018-the-fold-bounds-free-text-in-bytes.md

## What could not be established

- Comments outside the gateway that name `project.go`'s `classify`, `look`,
  `reperch` or `perchSequence`: in `client/src/app.ts`, `client/src/fold.ts`,
  `client/src/view/spectator.ts`, `client/src/wire.ts`,
  `client/test/app.test.ts`, `client/test/session.test.ts`,
  `client/test/spectator-view.test.ts` and `contract/events.test.ts`. Those
  naming `classify` or `look` stop resolving here. The owner ruled on
  2026-10-15 that they are corrected in the fourth ticket, which rewrites the
  client, and the third deletes `reperch` and `perchSequence`.
- Whether the port in `~/.claude/backups/vtt-per-character-logs-20260920/`
  helps. It was taken from a `project.go` older than the note visibility and
  the testimony rules, and is outside the repository.
