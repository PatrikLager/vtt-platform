# Authorization has a record

## The problem

Who may issue which command, and what a player must control, own or stand
next to before their command is accepted, is decided in
`internal/gateway/authz.go` (`commandRoles`, `Authorize`, `authorizePlayer`,
`playerRules`, `errUndecided`, `unrestricted`, `mayWorkDoor`,
`authorizeTokenOwnership`, `authorizeActorOwnership`, `controls`,
`authorizeSelfRevoke`, `authorizePromotionTarget`, `commandName`,
`ErrUnauthorized`), in `internal/gateway/grant_validate.go`
(`validateGrantActorControl`), in `internal/gateway/add_actor_validate.go`
(`validateAddActor`) and in `internal/gateway/server.go`'s command path
(`answerCommand`, `authorize`, `handleSetViewpoint`, `handleCommand`,
`describeBlockage`), and no specification states it. `commandRoles` is one
table of twenty-two commands by four roles, and a command not in it, an
unset command included, is refused for every role; a spectator's only cell
is `set_viewpoint`. Two checks run for every role after the table: a
promotion may make someone a player or a spectator and nothing else
(`authorizePromotionTarget`), and a perch may target a party member only
(`MayPerch`, in `viewpoint.go`). For a player, `playerRules` is a second
table keyed like the first: `move_token` needs the token's actor to count
the player among its `controller_ids`, `use_ability` and `remove_condition`
the named actor, `revoke_actor_control` the player naming themselves and
controlling the actor, `open_door` and `close_door` a controlled token on the
door's scene within one square by Chebyshev distance, `add_narration` nothing
(`unrestricted`); a player cell with no rule is refused by `errUndecided`;
an actor with no controllers is denied to every player; an empty participant
id controls nothing. After `Authorize`, `handleCommand` refuses a
`grant_actor_control` with no kind and an `add_actor` that names a controller
or states no kind (`validateGrantActorControl`, `validateAddActor`), refuses a
player's `move_token` onto a square they cannot see before it asks the
terrain, so the refusal reads the same whatever stands there, and renders a
blocked move's reason through `describeBlockage`, which clips an authored
scenery kind; the four batch commands, promotion and the door commands are
dispatched to their handlers, and every other command becomes one envelope
whose `TokenMoved` is backfilled with the scene and square it left before
`campaign.Append`. Each of those sentences lives today in a ticket's section
(`docs/superpowers/specs/2026-07-23-api-gateway-design.md` §4,
`2026-08-06-presence-and-actor-control-design.md` §5.3,
`2026-08-12-maps-as-geometry-design.md` §6,
`2026-08-08-joining-a-table-design.md` §3.1a), in the commit `b2445ab` for
the missing-rule arm, or in a comment block; ADR-011 and SPEC-009 carry
promotion's two bounds and re-resolution, and nothing carries the rest.
`authz.go` holds 230 comment lines of 439 non-blank (`664cacf`; `python3
tools/check-comments.py --report` gives 52.4 percent, 29 banned lines, 13
blocks over the bound with the package doc excepted), `grant_validate.go`
76.7 percent with 1 block over, `add_actor_validate.go` 84.6 percent with 2
over; `server.go`'s sixteen blocks on the five symbols above hold 240 lines,
11 of them over the bound. Three sentences among them are false: `authz.go`'s
package doc says the package "READS one file", `mapByID`'s probe, while
`handleArtFile` opens the art directory on every request; `Authorize`'s doc
says the table is followed by "one additional check for players moving
tokens", while six player rules and two checks for every role follow it; and
`handleCommand`'s doc says "only a persisted event/marker produces ok=true",
while `handlePromotion`, `handleJoinDoor` and `handleRotateJoinLink` answer
ok=true and append nothing.

## Done looks like

1. `docs/specifications/013-authorization.md` exists with the five headings
   SPEC-007 uses, in the present tense: the table, with every command's
   roles stated; refusal by default; the two checks for every role; the
   player rules one by one, with what each asks of the fold; what
   `handleCommand` checks after `Authorize` and in which order, the player's
   move gate and its wording included; what is dispatched to a handler and
   what becomes one envelope; and what this record does not decide, pointing
   at SPEC-009 for promotion's bounds and re-resolution and at `viewpoint.go`
   for `MayPerch`'s body. Every sentence names the symbol that holds it, and
   Phase 4b reads each against the code.
2. `grep -c 'READS one file' internal/gateway/authz.go`, `grep -c 'one
   additional check' internal/gateway/authz.go` and `grep -c 'only a
   persisted' internal/gateway/server.go` each print 0; today each prints 1.
3. Every rule the sort accepts has a row cited by a test under
   `internal/gateway/` that observes it, and a rule an existing row states
   (VTT-025, VTT-027, VTT-028, VTT-038) is cited by that row; `task
   check:requirements-chain` prints more than 137 rows and holds; the report
   names any OPEN row.
4. `python3 tools/check-comments.py --report | grep -E 'gateway/(authz|grant_validate|add_actor_validate)\.go'`
   prints `banned 0` and `blocks>6 0` on all three lines, and every block
   left in them is a warning, a pointer or the doc sentence of an exported
   symbol (Phase 4b's reading, VTT-051); `server.go`'s blocks on
   `answerCommand`, `authorize`, `handleSetViewpoint`, `handleCommand` and
   `describeBlockage` are pointers to SPEC-013 or warnings, and `server.go`'s
   `blocks>6` falls from 29 to 18. The ledger rows are lowered by
   `--write-ledger` in the same commit.
5. No code line changes: the go/scanner token stream of `authz.go`,
   `grant_validate.go`, `add_actor_validate.go` and `server.go` is identical
   to `664cacf`'s.
6. `task check` whole is green.

## Rules this puts on the system

Candidates, one line each, for the sort after sign-off; most have a test
already (`internal/gateway/authz_test.go`, `grant_validate_test.go`,
`add_actor_validate_test.go`, `server_test.go`, `server_visibility_test.go`),
and a test whose rule an existing row states cites that row.

- A command with no row in the table, an unset command included, is refused
  for every role.
- A spectator may issue `set_viewpoint` and no other command.
- A player may move a token only when they are among its actor's
  controllers; a second controller may, a non-controller may not, and an
  actor with no controllers is denied to every player.
- A player may use an ability with, or remove a condition from, only an
  actor they control.
- A player may revoke only their own control, and only of an actor they
  control; the DM and the agent may revoke anyone's.
- The DM and the agent may act on an actor a player controls.
- An empty participant id controls nothing.
- A player may work a door only with a controlled token on the door's scene
  within one square of it, diagonals included; the DM and the agent need no
  token.
- A player command that has a role cell and no player rule is refused.
- Every player cell has a player rule and every player rule has a cell.
- A grant that does not say what the actor is is refused, and every kind the
  contract offers is accepted.
- An `add_actor` that names a controller, by either field or as an empty
  set, is refused; one that states no kind is refused; one with no actor or
  no id is left to the fold.
- A player may not move onto a square they cannot see, and the refusal reads
  the same whatever stands there.
- A blocked move is refused with the obstruction named, and an authored
  scenery kind in it is bounded.
- The event a move appends records the scene and square the token left.

## What it touches

1. `docs/specifications/013-authorization.md`, new
2. `internal/gateway/authz.go`, `grant_validate.go`, `add_actor_validate.go`,
   comments only
3. `internal/gateway/server.go`, the comment blocks on `answerCommand`,
   `authorize`, `handleSetViewpoint`, `handleCommand` and `describeBlockage`
   only
4. `internal/gateway/authz_test.go`, `grant_validate_test.go`,
   `add_actor_validate_test.go`, `server_test.go`,
   `server_visibility_test.go`: citation lines where a row is dispensed or an
   existing one cited, and any comment block a re-aimed pointer sits in,
   sorted to the bound (`check:comments` refuses a line added to a block over
   it)
5. `docs/requirements.md`, rows after sign-off, by the dispenser
6. `tools/comment-ceilings.txt`, by `--write-ledger` only

One component; the specification first, the register with it, the comment
sort after, the ledger last, in one commit; the report in its own.

## Specifications this moves

New: the gateway's authorization — the role table, refusal by default, the
checks for every role, the player rules, and what the command path checks
after them.

## What could not be established

- Whether the player's sight gate on `move_token` is this record's rule or
  the projection record's: the rule is about what a seat can see, which the
  projection decides, and the seam and its ordering against the terrain check
  are the command path's. SPEC-013 states the seam and its order and points
  at the projection's home once it has one; the sort decides whether the row
  is dispensed here.
- Whether `describeBlockage` belongs here: it renders a refusal the command
  path produces, and its rule-9 answer sits in its own comment. It is read
  here because the block is the command path's; the sort decides its row.
- Which of the five test files' comment blocks the pointer re-aims will
  force to the bound; the plan lists them after the sweep's pointers are
  known. `commandRoles`' rows on `remove_token` and `remove_actor` point at
  `authz_test.go`'s `authzCases` comment for their argument, and that
  argument has no home in a record.
- Whether `MayPerch`'s own comment block in `viewpoint.go` is this ticket's
  or the seat-and-perch ticket's: it is left to the latter, and SPEC-013
  points at the function.
