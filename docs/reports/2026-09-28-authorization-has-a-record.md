# Authorization has a record: the change

**Ticket:** `docs/superpowers/specs/2026-09-28-authorization-has-a-record-design.md`.
**Plan:** `docs/superpowers/plans/2026-09-28-authorization-has-a-record.md`,
verified by `verify-ticket` (Passes with gaps). Its twelve sign-off questions
were answered on 2026-09-28, all twelve as the plan proposed: SPEC-013 names
no other project; the DM-and-agent column and the player's refused cells are
rows; the grant's row sits on SPEC-013 and SPEC-007 is untouched; the refusal
shape is SPEC-013's; `server_internal_test.go` joins the change and its
`describeBlockage` test is cited; the two table-shape tests are not cited;
the `campaign unavailable` arm gets no row; the perch's rows are the
seat-and-perch ticket's; the sight gate's rows are SPEC-013's; the perch's
reconnect cost is one sentence in SPEC-013's Consequences; one commit and the
report apart; and the reading governs over the bound. The sort was signed
off separately, with the changes under Deviations.
**Last commit of the change (no code line changed):** `8128a61`, on `664cacf`,
`main` at the time. Every code reference below is to that tree.

## The period, in commits

    git log --oneline 664cacf..8128a61

    8128a61 Authorization has a record: SPEC-013, VTT-138 to VTT-162

`git diff --stat 664cacf..8128a61`: 16 files changed, 1919 insertions(+), 731 deletions(-).

The gate: `task check`, whole, over the tree of `8128a61` before it was
committed: exit 0, no step failed, and the check steps' own verdict lines
read `check:comments` `clean`, `check:requirements-chain` 162 rows,
`check:doc-owner` 79 files, `check:new-prose` 161 added lines clean,
`check:coverage` 20 packages at or above their floors, `check:no-pack` clean,
`check:mutation` fourteen packages with zero unadjudicated survivors,
`check:ts-mutation` 2882 mutants, 2783 killed, 29 timed out and counted as
killed by that gate's own rule, 70 survivors all adjudicated, zero
unadjudicated, verified from its stored report because no client input had
changed. Before it: `gofmt`, `go vet`, `task lint`, the gateway package's
tests, `check:comments`, `check:doc-owner`, `check:requirements-chain`,
`check:new-prose`, both mutation self-tests, the token instrument, and the
breaks; then the pre-commit hook's nine checks.

Phase 4a was skipped: the change moves prose and changes no code line, and
what can be wrong in it is a sentence, which the reading review holds.

**Rule 9.** MapTool's ownership predicate, `Token.isOwner`, is membership in
an owner list plus an owned-by-all wildcard. The membership half is the shape
`controls` already has over `Actor.controller_ids`; the wildcard is refused,
and VTT-143 holds the opposite answer, an actor with no controllers being
nobody's. The rest has no counterpart: MapTool checks ownership in the client
(`AppUtil.playerOwns`, from some thirty client files) and not in
`ServerMessageHandler`, decides a role once at the handshake, and has no
spectator, agent, promotion, adjacency rule or sight gate. The client-side
model is the one CLAUDE.md rule 9 says not to take; SPEC-013 names no other
project.

## Done looks like, answered

1. `[x]` `docs/specifications/013-authorization.md` exists with SPEC-007's
   five headings (`grep -c '^## '` prints 5): the table with every command's
   roles, refusal by default, the two checks for every role, the player
   rules one by one with what each asks of the fold, the command path's
   order with the move gate and its wording, what is dispatched and what
   becomes one envelope, and what the record does not decide, pointing at
   SPEC-009 and at `viewpoint.go`'s `MayPerch`. The reading review (Phase 4b,
   `pr-review-toolkit:code-reviewer` on `fable`) read every sentence against
   the symbol it names, by command, and the table cell by cell against
   `commandRoles`; four sentences were corrected or added before the commit
   (Deviations).
2. `[x]` `grep -c 'READS one file' internal/gateway/authz.go`, `grep -c 'one
   additional check' internal/gateway/authz.go` and `grep -c 'only a
   persisted' internal/gateway/server.go` print 0, 0 and 0; at `664cacf`
   each printed 1.
3. `[x]` Twenty-five rows, VTT-138 to VTT-162, each with the tests in its
   evidence cell citing it; VTT-025, VTT-027 and VTT-038 keep their
   citations, and VTT-028's line above `TestAuthorizeTableAllCommandsAllRoles`
   now carries VTT-139, VTT-140 and VTT-141 beside it. `task
   check:requirements-chain` prints `162 rows, 193 test files, 7
   specifications; every citation resolves and every row's evidence holds`.
   No row of the twenty-five is OPEN.
4. `[x]` `python3 tools/check-comments.py --report` prints `banned 0` and
   `blocks>6 0` for `authz.go`, `grant_validate.go` and
   `add_actor_validate.go`, and `blocks>6 18` for `server.go` (29 at
   `664cacf`). That every surviving block in the three files and in
   `server.go`'s sixteen is a warning, a pointer or an exported symbol's doc
   sentence is the reading review's verdict (VTT-051), a reading and not a
   check. The ledger rows moved in the same commit, `--write-ledger` run
   last.
5. `[x]` The go/scanner token stream, comments dropped, of `authz.go`,
   `grant_validate.go`, `add_actor_validate.go` and `server.go` is identical
   to `664cacf`'s (1790, 61, 119 and 4596 tokens; the instrument is the
   program plan D11 names, compared with `cmp` against `git show
   664cacf:<file>`); so are the six test files' (`authz_test.go` 6622,
   `grant_validate_test.go` 388, `add_actor_validate_test.go` 981,
   `server_test.go` 12711, `server_visibility_test.go` 6280,
   `server_internal_test.go` 6583).
6. `[x]` `task check` whole, exit 0, above.

## What the rules became

| Rule, as the ticket words it | Became |
|---|---|
| A command with no row in the table, an unset command included, is refused for every role | VTT-138 |
| A spectator may issue `set_viewpoint` and no other command | VTT-139 |
| A player may move a token only when they are among its actor's controllers; a second controller may, a non-controller may not, and an actor with no controllers is denied to every player | VTT-142, VTT-143 (two rows: the empty set is its own decision, with its own test) |
| A player may use an ability with, or remove a condition from, only an actor they control | VTT-144 |
| A player may revoke only their own control, and only of an actor they control; the DM and the agent may revoke anyone's | VTT-145, VTT-146; the DM-and-agent half is VTT-147 |
| The DM and the agent may act on an actor a player controls | VTT-147, which also carries granting, revoking and working a door from anywhere |
| An empty participant id controls nothing | VTT-148 |
| A player may work a door only with a controlled token on the door's scene within one square of it, diagonals included; the DM and the agent need no token | VTT-149; the DM-and-agent half is VTT-147 |
| A player command that has a role cell and no player rule is refused | VTT-150 |
| Every player cell has a player rule and every player rule has a cell | refused: a check on the tables' shape; SPEC-013 names `TestEveryPlayerCommandHasARule` as what holds it |
| A grant that does not say what the actor is is refused, and every kind the contract offers is accepted | VTT-151 |
| An `add_actor` that names a controller, by either field or as an empty set, is refused; one that states no kind is refused; one with no actor or no id is left to the fold | VTT-152, VTT-153, VTT-154 |
| A player may not move onto a square they cannot see, and the refusal reads the same whatever stands there | VTT-155, VTT-156 |
| A blocked move is refused with the obstruction named, and an authored scenery kind in it is bounded | VTT-157, VTT-159 |
| The event a move appends records the scene and square the token left | VTT-160 |

Five rows came from the plan's sort rather than the ticket's list: VTT-140,
the DM and the agent may issue every command but `set_viewpoint`; VTT-141,
the commands a player may not issue, even for an actor or token they
control; VTT-158, the DM may move a token onto any square, a wall and a
square no player can see included; VTT-161, a command the table, a player rule or the fold refuses is
answered ok=false on a connection that stays open; VTT-162, a refused
command appends nothing.

## The sweep

Comment lines over non-blank lines, counted by `tools/check-comments.py`'s
own `measure` with citation lines set aside, at `664cacf` and at `8128a61`:

| File | Before | After | Blocks over the bound |
|---|---|---|---|
| `internal/gateway/authz.go` | 230 / 439 | 32 / 241 | 13 to 0 |
| `internal/gateway/grant_validate.go` | 46 / 60 | 5 / 19 | 1 to 0 |
| `internal/gateway/add_actor_validate.go` | 132 / 156 | 8 / 32 | 2 to 0 |
| `internal/gateway/server.go` | 681 / 1277 | 473 / 1069 | 29 to 18 |
| `internal/gateway/authz_test.go` | 452 / 1184 | 428 / 1160 | 20 to 19 |
| `internal/gateway/server_visibility_test.go` | 325 / 1088 | 317 / 1080 | 20 to 19 |
| `internal/gateway/server_internal_test.go` | 504 / 1350 | 493 / 1339 | 29 to 28 |

`server_test.go`, `grant_validate_test.go` and `add_actor_validate_test.go`
changed only by citation lines and, in `server_test.go`, one re-aimed block
of the same length. Ledger rows, old to new: `add_actor_validate.go` 84.7 to
25.0, `authz.go` 52.4 to 13.3, `authz_test.go` 38.2 to 36.9,
`grant_validate.go` 76.7 to 26.4, `server.go` 53.4 to 44.3,
`server_internal_test.go` 37.4 to 36.9, `server_visibility_test.go` 29.9 to
29.4.

What the kept blocks are, by symbol. `authz.go`: the package doc sentence;
warnings above `commandRoles`, the two removal rows, `authorizePlayer`,
`playerRules`, `mayWorkDoor`, `controls` (two), `authorizeSelfRevoke`,
`authorizePromotionTarget` and `commandName`; inside `Authorize`, the two
every-role checks; inside `authorizePlayer`, the missing-rule arm; the doc
sentences of `Authorize` and `ErrUnauthorized`, the latter unchanged.
`grant_validate.go`: two warnings above `validateGrantActorControl`.
`add_actor_validate.go`: two above `validateAddActor`, one in its controller
arm, one above the id check. `server.go`: one warning or pointer on each of
`describeBlockage`, `answerCommand` (and its two arms), `authorize`,
`handleSetViewpoint` and `handleCommand` (its doc, the player gate, the
sight check, the obstruction, the validators, the batch dispatch, the three
that append nothing, the backfill); the `add_actor` and `remove_actor`
blocks went.

Facts the cut blocks held that SPEC-013 took: that `Authorize` does no I/O;
that an unset command is refused like an unknown one; why the two checks run
for every role; that `authorizePlayer` must be passed the name derived with
the command; that `unrestricted` is an entry, not an absence; that
`mayWorkDoor` is spatial and Chebyshev; that `controls` reads the set and
never the mirror; the empty-id guard on the revoke path; that a revoke of
control never held is refused before the log; that both kind checks refuse
`UNSPECIFIED` only; that the controller refusal comes first and names
`grant_actor_control`; that the fold refuses a controller on `ActorAdded` and
accepts a kindless `ActorAdded` and a kindless grant; the sight-first order
and its cost; the clip and the author-written kind; that `answerCommand`
queues no frame; that `handleSetViewpoint` applies nothing itself, what its
ok means, and that a perch ends with its connection; the `TokenMoved`
backfill; removal holding the authoring cells; the join door, its link and
promotion being the DM's and the agent's.

Facts dropped, each with the reason: the dated and task-numbered histories
of every row and helper (history, rule 10); the gocyclo figure that split
`answerCommand` out of `serve` (history); the byte measurements before the
clip and why the refusal is an error rather than a warning (history, and the
clip is VTT-159's); the RPTool `clearAllOwners` aside and its line coordinate
(another project, and a line number); the "one writer beats two" and "the
danger was never multiplicity" arguments (the rule is VTT-152 and SPEC-013's
sentence); why an unassigned player does not perch and the DM needs no
shoulder (the perch's rule is the seat-and-perch ticket's); that a
`move_token` validates a destination and never a path (implied by
`canSee(…, to)`, not stated); that `internal/adventure`'s `loadActors`
refuses a kindless `actors/*.json` (true, and the adventure format's record
has not been written); that `remove_token` needs no board-position seam
because the fold's unknown-token guard is its whole validation (the fold's);
the rule-9 answer on `describeBlockage`, that MapTool's movement blocking is
geometric and names no obstruction (another project; the art-is-a-flat-library
ticket keeps it); that notes are the DM's because world facts are (the
cells are VTT-141's).

`server.go`'s eighteen remaining blocks over the bound, by the symbol each
sits on or in: the `Server` struct's field docs (four: `writeTimeout`,
`maps`, `artDir`, `cellPx`), `WithMaps`, `WithMapsDir`, `WithArtDir`,
`WithCellPx`, `announcePresence`, `announceDeparture` (two, its doc and its
body), `revoked`, `announcePromotion`, `handleRemoveActor`, `handleJoinDoor`
(one, in its body), `handlePromotion` (two, its doc and its body),
`credentialGone`.

## The breaks

In a scratch clone of the changed tree, committed there as that clone's
`main`; each gate first ran clean (`162 rows ... every row's evidence
holds`, `239 files, 0 added comment lines, 239 ledger rows; clean`, `79
files, every doc comment sits on its own function`, four `same`), then one
edit per break, reverted by its inverse, `git diff --stat` empty before the
next.

| Break | Red |
|---|---|
| B1 `// VTT-142` above `TestAuthorizePlayerUnknownTokenDenied` made `// VTT-142 VTT-999` | `check:requirements-chain: internal/gateway/authz_test.go cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 VTT-148's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `row VTT-148: internal/gateway/codec_test.go does not carry the id, so the link walks one way only` |
| B3 VTT-999 on SPEC-013's Requirements line | `docs/specifications/013-authorization.md cites VTT-999 and no row in docs/requirements.md defines it (specification citation)` |
| B4 a `// SPEC-013` line inside `validateGrantActorControl`, after the ledger was written | `check:comments: internal/gateway/grant_validate.go: comment share 30.00 is above its ceiling 26.4 and this change added a comment line to it (SPEC-010)` |
| B5 `abs(tok.X-at.GetX()) <= 1` made `<= 2` in `mayWorkDoor` | the token instrument prints `DIFFERS` for `authz.go`, and `TestAPlayerMayOnlyWorkADoorTheyAreNextTo` is red |
| B6 "one additional check for players moving tokens" back in `Authorize`'s doc | `grep -c 'one additional check' internal/gateway/authz.go` prints 1 |
| B7 the warning above `authorizeSelfRevoke` rewritten to open with "Authorize" | ``check:doc-owner: ... the doc comment above `authorizeSelfRevoke` begins by describing `Authorize`, which is a different function`` |

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Plan D5's sort, twenty-four rows A to X. | Twenty-five rows. D names twelve commands, not fifteen; F drops "and a condition"; H is two rows (VTT-145, VTT-146); the plan's L is folded into VTT-147; R is the player's half only (the DM's is VTT-158); X is two rows (VTT-161, VTT-162); Q is worded on what its tests observe. | Signed off at Task 7. VTT-028 already states `set_join_door`, `rotate_join_link` and `promote_participant`; no test drives `remove_condition` on an actor with no controllers; H and X each joined two obligations with "and"; L's own edit, deleting `mayWorkDoor`'s early return, leaves `TestAuthorizeDMMayWorkDoorRegardlessOfTokenPosition` green, because the test goes through `Authorize`, which returns before any player rule for the DM and the agent. |
| Plan D5: row A is reddened by deleting `!known \|\|` from `Authorize`. | Deleting `!known \|\|` alone does not compile (`known` declared and not used); with the binding dropped too (`roles := commandRoles[name]`), `TestAuthorizeUnknownCommandDeniedForEveryRole` stays green; `known && !roles[p.Role]` reddens it (its dm, agent and spectator subtests). | A lookup in a nil map answers false, so `!roles[p.Role]` alone still refuses a command with no row. |
| Plan D14: no adjudication key names any of the four files. | `tools/mutation-equivalents.txt` keyed `abs`'s `if n < 0` at `authz.go:301:7`; the key moved to `143:7`, its live sibling from `302:10` to `144:10`, and the two entries that cross-reference it by coordinate (`internal/sight`'s `abs32`, and the `want <= 0` entry) moved their live sentence with it. The file joined the commit (signed off at Task 7). | `check_mutation_test.py`'s `test_every_real_adjudication_still_points_at_its_mutant` went red in Task 6: the cut blocks above `abs` moved it 158 lines. Re-pointed after the review settled, located by searching for `if n < 0 {`, which occurs once. |
| Plan D12: seven or eight ledger rows move, `server_test.go` among them. | Seven, without `server_test.go`. | Its one re-aimed block kept its length, and citation lines move no share (VTT-059). |
| No code line changes. | The token streams are identical; `gofmt` re-aligned `commandRoles`' map literal, so `git diff` shows its rows as changed. | Removing the comments between rows joined every row below the surviving removal warning into one alignment group; `move_token`, `add_actor` and `place_token`, above it, stay their own. Whitespace only. |
| Plan D7's replacement texts. | Six are rewritten as imperative warnings rather than descriptions (the `isPerch` arm, the player gate, the obstruction, the validators, the batch dispatch, the three that append nothing); the validators' warning keeps the `ToEvent` pointer (`TestEveryClientCommandConverts`) the cut block argued. | Rule 10 admits a warning, a pointer or an exported doc sentence; six of D7's texts were descriptions of the code. |
| The plan's warnings list for `authz.go`. | The membership warning sits above `controls`, where `GetControllerIds` is read, not above `authorizeTokenOwnership`; a removal warning stands above the `remove_token` and `remove_actor` rows; `commandName` keeps a warning that its names are the proto field's. | Where the read happens is where an edit would break it; the removal rows and `commandName` each hold a decision a later edit could undo in silence. |
| The reading review's findings. | Applied before the commit: the `commandRoles` warning no longer says "the one place the question is asked" (`MayPerch` asks the role too); the backfill warning says `engine.Apply` never reads `SceneId` or `From`, not that it reads only `To`; SPEC-013's Consequences bound a move refusal's text only, where they had said every refusal's text is safe to show; SPEC-013 states that the fold accepts a kindless grant; VTT-161 names the refusals its tests observe; VTT-158 names the DM, whom its tests drive; SPEC-013 points at SPEC-009 and VTT-028 for the standing sentence and names the search behind "every refusal wraps `ErrUnauthorized`"; the empty-id warning and one test doc were reworded. One finding was refused: the review's token counts (4978 for `server.go`) come from a program other than plan D11's listing, which prints 4596 at `664cacf`. | Each checked by command before it was applied. |
| Plan Task 5's done check: `grep` for the old pointer phrases over the six test files prints nothing. | It prints four lines, in `grant_validate_test.go` and `add_actor_validate_test.go`. | Those lines point at the proto doc comments of `ActorKind` and `Actor.controller_ids` and at `TestEveryClientCommandConverts`' doc, which survive; plan D22 leaves them, and the grep was wider than the rule. |
| The plan's C2 carries the report alone (Commits; D23's "the ticket allows none"). | C2 also adds an Open-debt entry for VTT-149's y bound. | B5's probes found the bound unobserved from above; CLAUDE.md names `docs/verification-debt.md` as the one file for known coverage gaps, and D23's ruling was about the `campaign unavailable` arm, which stays in this report. Signed off before C1. |
| Plan D13's B5: `TestAuthorizePlayerMayNotWorkADoorItStandsWestOrNorthOf` goes red. | With the x bound at `<= 2`, the only red test is `TestAPlayerMayOnlyWorkADoorTheyAreNextTo`; with both bounds at `<= 2`, `TestAuthorizePlayerMayNotWorkDiagonallyDistantDoor` too; with the y bound alone, none (next section). | Probed in the clone while running the break. |

## What could not be established

- VTT-149's y bound is unobserved from above: `abs(tok.Y-at.GetY()) <= 2` in
  `mayWorkDoor` leaves every door test green, and the x bound at `<= 2` is
  held by one wire test alone. `docs/verification-debt.md`
  carries it under Open debt, with the recipe and the test that would close
  it.
- VTT-158's agent half: the three tests drive the DM; the row names the DM,
  and SPEC-013 states both roles from `handleCommand`'s `if p.Role ==
  identity.RolePlayer`.
- `authorize`'s `campaign unavailable` arm has no test (plan D23): SPEC-013
  states it, and a `Campaign` whose `State()` answers nil is one fixture
  away in `server_internal_test.go`.
- `mayWorkDoor`'s own early return for a role other than player cannot be
  reached through `Authorize`, which returns before `authorizePlayer` for
  every such role; no test drives it, and none can through `Authorize`.
- `handleSetViewpoint`'s warning, that a frame emitted from the command
  goroutine lands inside a projected batch, is SPEC-011's ruling and was
  read, not re-derived.
- Left stale by this change's scope, for the test-prose sweep: the `authzCases`
  argument in `authz_test.go`, no longer pointed at from `commandRoles`;
  `TestARevokedSpectatorStopsSeeingTheTable`'s doc, "commandRoles has no
  spectator row anywhere", false since the `set_viewpoint` row;
  `TestEveryPlayerCommandHasARule`'s doc, which says its reasoning was
  written on a row comment that no longer exists; the bare mutant
  coordinates `authz.go:152:15` and `authz.go:164:10` in the docs of
  `TestAuthorizePlayerMayWorkAdjacentDoorAwayFromTheOrigin` and
  `TestAuthorizePlayerMayNotWorkADoorItStandsWestOrNorthOf`, stale before
  this change and outside the adjudication files; `TestASpectatorMayNotWorkDoors`'
  doc naming a commit; `client/src/view/doors.ts`' and `player.ts`' sentences
  about `mayWorkDoor`, true after the sweep and the client sweep's.

## What was deliberately left out, and where it went

- `server.go`'s eighteen blocks over the bound, by symbol above: the maps and
  art configuration, the announcement helpers and the remaining command
  handlers are the next gateway tickets'.
- `MayPerch`'s rule and its block in `viewpoint.go`, and the perch's rows
  (`TestPerchingAppendsNothingToTheLog`,
  `TestAuthorizeSpectatorMayNotPerchOnAnNpc`,
  `TestASpectatorMayNotPerchOnTheGoblinArcher`): the seat-and-perch ticket's.
- What a seat can see (`canSee`, `viewerFor`, the `Projector`): the
  projection's record, which SPEC-013 points at as having none yet.
- The batch handlers `handleLoadMap` and `handleRemoveActor`: no record yet;
  SPEC-013 names them and SPEC-007 states what their batches carry.
- No file under `cmd/`, `contract/` or `client/src` changed, nor any under
  `internal/` outside the four swept production files and six test files,
  whose token streams are identical to `664cacf`'s (item 5).

## The sort

Twenty-three of the plan's twenty-four candidates accepted as twenty-five
rows, VTT-138 to VTT-162 (H and X split, L folded into VTT-147). Refused: the
unknown-token and unknown-actor refusals, which fall out of VTT-142 and
VTT-144 and are cited under them; the two table-shape checks,
`TestEveryPlayerCommandHasARule` and `TestEveryClientCommandHasRoleCells`,
which SPEC-013 names; `authorize`'s `campaign unavailable`, which no test
drives; the perch's rules, the seat-and-perch ticket's; `TestNoEventPayloadNamesARole`,
VTT-038's; `TestTheDMCanActuallyOpenTheDoor` and
`TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt`, `handleJoinDoor`'s and
SPEC-009's; the four promotion rules, already VTT-025, VTT-026, VTT-027 and
VTT-036. Of the ticket's clauses, three live inside rows rather than as
rows of their own ("an unset command included" in VTT-138, "the DM and the
agent need no token" in VTT-147's "from anywhere", "the named actor" in
VTT-144's wording), and two stayed prose: "before it asks the terrain" is
VTT-156's edit, and "the four batch commands, promotion and the door
commands are dispatched to their handlers" is SPEC-013's dispatch
paragraph, held by each handler's own tests.
