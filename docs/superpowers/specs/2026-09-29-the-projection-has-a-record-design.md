# The projection has a record

## The problem

What a player or a spectator is sent of each event is decided in
`internal/gateway/project.go` (`Viewer`, `Projector`, `NewProjector`,
`Project`, `perchSequence`, `reperch`, `sightView`, `look`, `eyes`,
`transitions`, `sceneSeenFor`, `objectInSight`, `verdict`, `passIf`,
`classify`, `canSeeSquare`, `doorTransitions`, `doorSubject`, `squareAt`,
`squareKey` and the sorted-set helpers), and no specification states it.
`Project` answers the DM and the agent with the event unchanged, a role it
does not know with nothing, and a player or a spectator with what the
projection admits. `eyes` gives a player the actors they control and a
spectator the one party member they perch on; `look` computes, from those
actors' tokens and `sight.VisibleFrom`, the squares visible now, the tokens
standing on them and the actors the viewer may know (those tokens' actors and
every party member). `transitions` compares that with the projector's memory
(`scenes`, `actors`, `tokens`, `seen`, `doors`), forgets each actor the world
no longer has, and synthesizes the frames that change it: a scene introduced
by name and size, doors seen to have changed, actors introduced with their
controllers as grants and their conditions, tokens hidden and placed, and, for
each scene whose visible squares changed, a `SceneSeen` of them with their
tiles and every object with a square of its footprint among them. `classify`
rules on every arm of the envelope oneof: forwarded (session start and end,
narration), withheld (scene, actor and token creation, a token's removal, the
projection-only payloads, notes, adventure loads), forwarded only when the
viewer sees the subject (a token's move seen before and after, a door's
square), forwarded only when the viewer knows every actor named (attacks,
abilities, control changes), and forwarded only to a viewer who already holds
the actor (resource and condition changes, an actor's removal); a payload it
has never heard of is sent to no player or spectator. A perch's frames carry
`perchSequence`, 0; every other frame of an event carries that event's
sequence. Each of those sentences lives today in
a ticket (`docs/superpowers/specs/2026-08-18-visibility-design.md`), a report
(`docs/reports/2026-08-18-visibility.md`) or a comment block; SPEC-015 carries
how a seat feeds its projector and applies a perch, and says the projection
has no record. `project.go` holds 723 comment lines of 1173 non-blank
(`python3 tools/check-comments.py --report` gives 61.6 percent, 64 banned
lines, 30 blocks over the bound, at `db1f77d`). Three passages are false or
stale: `transitions`' doc declares a torn-batch hazard "UNRESOLVED, AND IT
BELONGS TO WHOEVER WIRES THE PUMP" and cites `client/src/session.ts` by line,
while `seat.pastResume` and the client's reconnect resolve it; `reperch`'s doc
says a measurement is "written up at perchBox", whose comment the seat change
cut; and the narration arm is "FLAGGED FOR ADJUDICATION", which the owner
resolved on 2026-09-29 (narration is forwarded to every viewer).

The owner also ruled, the same day, two changes to this behaviour: a note
carries a visibility flag, public or DM-only, and only what a viewer sees gives
them information, for every actor, party members included. Neither is
implemented; each is its own ticket after this one. This ticket records the
projection as it is.

## Done looks like

1. `docs/specifications/016-the-projection.md` exists with the five headings
   SPEC-007 uses, in the present tense: which roles are projected and what
   `Project` answers each; whose eyes a viewer has; what `look` computes; the
   projector's memory and what `transitions` synthesizes from it, in its
   order; `classify`'s rulings, arm by arm, the unrecognised default included;
   doors; the sequence every frame carries; and what this record does not
   decide, pointing at SPEC-015 for the seat and the perch, at `internal/sight`
   for what a square can see, and at SPEC-007 for the payloads. Its Status says
   which of its sentences the two rulings of 2026-09-29 will change. Every
   sentence names the symbol that holds it, and Phase 4b reads each against the
   code.
2. `grep -c 'UNRESOLVED' internal/gateway/project.go`, `grep -c 'written up
   at perchBox' internal/gateway/project.go` and `grep -c 'FLAGGED FOR
   ADJUDICATION' internal/gateway/project.go` print 0, 0 and at most 1 (the
   notes arm, until its ticket); today they print 1, 1 and 2.
3. Every rule the sort accepts has a row cited by a test under
   `internal/gateway/` that observes it; `task check:requirements-chain`
   prints more than 193 rows and holds; the report names any OPEN row.
4. `python3 tools/check-comments.py --report | grep 'gateway/project\.go'`
   prints `banned 0` and `blocks>6 0`, and every block left in it is a warning,
   a pointer or the doc sentence of an exported symbol (Phase 4b's reading,
   VTT-051). The ledger rows are lowered by `--write-ledger` in the same
   commit.
5. No code line changes: the go/scanner token stream of `project.go` and of
   every test file this touches is identical to `db1f77d`'s.
6. The specifications that say the projection has no record (SPEC-013,
   SPEC-015) point at SPEC-016.
7. `task check` whole is green.

## Rules this puts on the system

Candidates, one line each, for the sort after sign-off; most have a test
already (`internal/gateway/project_test.go`, `project_internal_test.go`,
`project_property_test.go`, `keystone_test.go`, `server_visibility_test.go`,
`viewpoint_internal_test.go`).

- A player sees through the actors they control and a spectator through the
  party member they perch on; no viewer sees through anyone else.
- A viewer is sent a scene only once a token of one of its eyes stands in it,
  and then only its name and size.
- A viewer is sent the tiles of the squares it sees in a scene now and each
  object with a square of its footprint among them, and is told when that set
  changes.
- A token reaches a viewer as a placement only while it stands on a square the
  viewer sees, and leaves as a `TokenHidden` when it stops.
- An actor is introduced to a viewer when it is a party member or when its
  token is first seen, with its controllers as grants and its conditions.
- A token's move is forwarded only when the viewer saw the token before and
  after it.
- A door's change is forwarded only when its square is seen, and a door that
  changed unseen is corrected when it is seen.
- An attack, an ability or a control change is forwarded only when the viewer
  knows every actor it names.
- A resource or condition change, or an actor's removal, is forwarded only to a
  viewer who already holds the actor.
- Narration and a session's start and end reach every viewer.
- A note reaches no player or spectator.
- A scene, actor or token created or removed in the log, and a payload only
  the projection issues, is never forwarded as written.
- A payload the projection has never heard of reaches no player or spectator,
  and every arm of the envelope has a ruling.
- Folding a viewer's projected stream gives the world the server says that
  viewer sees.
- Every frame of one event carries that event's sequence.

## What it touches

1. `docs/specifications/016-the-projection.md`, new
2. `internal/gateway/project.go`, comments only
3. The test files above: citation lines where a row is dispensed, and any
   comment block a re-aimed pointer sits in, sorted to the bound
   (`check:comments` refuses a line added to a block over it)
4. `docs/specifications/013-authorization.md` and
   `015-the-seat-and-the-perch.md`, the sentences that say the projection has
   no record
5. `docs/requirements.md`, rows after sign-off, by the dispenser
6. `tools/comment-ceilings.txt`, by `--write-ledger` only
7. `tools/mutation-equivalents.txt`, the three keys in `project.go`, which
   move with the comment lines above them
8. `docs/verification-debt.md`, an entry for a rule the sort finds no test
   for, with the report

`internal/engine/apply.go`'s sentence that `project.go` quotes its format
string is left as it stands and named in the report.

One component; the specification first, the register with it, the comment
sort after, the mutation keys and the ledger last, in one commit; the report
and the debt entry in their own.

## Specifications this moves

New: the projection — whose eyes a viewer has, what they can see, what the
projector remembers and synthesizes, and how each payload is ruled.
docs/specifications/013-authorization.md
docs/specifications/015-the-seat-and-the-perch.md

## What could not be established

- How SPEC-016's Status carries the two rulings of 2026-09-29 (a note's
  visibility flag; only what a viewer sees gives them information, for every
  actor) before their tickets exist: the notes arm and the "knows" and
  "already holds" arms describe the code, and the rulings will change them.
  The plan proposes the wording.
- Whether the keystone property (a folded projection equals what the server
  says the viewer sees) is one rule or the sum of the others.
- Which of the test files' blocks a re-aimed pointer forces to the bound;
  `project_test.go` alone holds 44 blocks over it.
- `sight.VisibleFrom`'s own rules (walls, closed doors, objects that block
  sight, range) are `internal/sight`'s, which has no record; this ticket states
  that `look` asks it and with what.
