# Removing an actor has a record, and the gateway's last three files are swept

## The problem

`remove_actor` is handled by `handleRemoveActor` in
`internal/gateway/server.go`, and no specification states it as a decision.
The handler reads the actor's tokens from the snapshot `handleCommand` took
through `authorize`, sorts their ids, builds one `TokenRemoved` per token in that order and
then the `ActorRemoved`, stamps each envelope with a fresh event id, the
participant, its role and one `OccurredAt`, and appends them in one
`campaign.AppendBatch`; it checks nothing itself. `engine.Apply` refuses an
unknown actor ("removed unknown actor") and an actor with a token still on
the board, so an unknown id and a token placed after the snapshot are both
refused by the fold and the batch appends nothing. No event ends a control
grant: `controller_ids` lives on the actor, which is gone. SPEC-007 states
what the batch carries and in what order; SPEC-013 says `remove_actor`
"has no record of its own" and sends the batch handlers to
"`handleRemoveActor`'s" record, which does not exist. The rest of what these
three files decide already has a home: the connection, presence and its
announcements, `writeTimeout` and `credentialGone` in SPEC-011; revocation,
the join door, the join link, promotion and the display name in SPEC-009;
the command path, `ToEvent` and the grant's kind in SPEC-013; batches and
what appends nothing in SPEC-007. Their comment blocks repeat those decisions
in the past tense, with ticket sections, task numbers, issue numbers and dates
beside them. At `e52b4c2`, `python3 tools/check-comments.py --report` gives
`server.go` 358 comment lines of 959 (37.3 percent, 8 banned lines, 11 blocks
over the bound), `convert.go` 59 of 165 (35.8, 7, 3) and `join.go` 78 of 147
(53.1, 4, 4); every other production file under `internal/gateway/` shows
`banned 0` and `blocks>6 0`. One sentence among them is false: `convert.go`'s
block above the `RemoveCondition` arm says `handleUseAbility` is
"server.go's", and it is in `internal/gateway/ruleset.go`.

## Done looks like

1. A specification states, in the present tense, what `remove_actor`
   appends and in what order, that the batch is accepted or refused whole,
   that the handler checks nothing and what the fold refuses in its place,
   and that no event ends a control grant on the removed actor; every
   sentence names the symbol that holds it, and Phase 4b reads each against
   the code. `grep -c 'has no record of its own' docs/specifications/013-authorization.md`
   prints 0; today it prints 1.
2. `grep -c "(server.go's" internal/gateway/convert.go` prints 0; today it
   prints 1.
3. Every rule the sort accepts has a row cited by a test under
   `internal/gateway/` that observes it; `task check:requirements-chain`
   prints more than 253 rows and holds; the report names any OPEN row.
4. `python3 tools/check-comments.py --report | grep -E 'gateway/(server|convert|join)\.go '`
   prints `banned   0` and `blocks>6   0` on all three lines, and every block
   left in them is a warning, a pointer or the doc sentence of an exported
   symbol (Phase 4b's reading, VTT-051). The ledger rows are lowered by
   `--write-ledger` in the same commit. Afterwards every production file
   under `internal/gateway/` shows `banned 0` and `blocks>6 0`.
5. No code line changes: the go/scanner token stream of `server.go`,
   `convert.go`, `join.go` and every test file this touches is identical to
   `e52b4c2`'s.
6. `task check` whole is green.

## Rules this puts on the system

Candidates, one line each, for the sort after sign-off; most have a test
already (`internal/gateway/remove_actor_test.go`, `server_test.go`,
`convert_test.go`, `join_test.go`).

- Removing an actor appends one `TokenRemoved` per token of that actor, in
  token-id order, and then the `ActorRemoved`.
- A removal is appended whole or not at all.
- Removing an actor that does not exist is refused and appends nothing.
- A token placed for the actor after the snapshot the removal read makes the
  removal refused, never leaves a token without its actor.
- Removing an actor ends every control grant on it, with no event of its own.
- Removing an actor with no token appends the `ActorRemoved` alone.
- A `set_join_door` that says neither open nor closed is refused and changes
  nothing.
- A one-envelope command's conversion carries every field the command gave,
  a grant's kind included.

## What it touches

1. A specification for `remove_actor` (new, or a section of an existing one;
   the plan decides), and `docs/specifications/013-authorization.md`, whose
   sentences send the reader to a record that does not exist
2. `internal/gateway/server.go`, `internal/gateway/convert.go` and
   `internal/gateway/join.go`, comments only
3. `internal/gateway/remove_actor_test.go`, `server_test.go`,
   `server_internal_test.go`, `convert_test.go` and `join_test.go`: citation
   lines where a row is dispensed, and any comment block a re-aimed pointer
   sits in, sorted to the bound (`check:comments` refuses a line added to a
   block over it)
4. `docs/requirements.md`, rows after sign-off, by the dispenser
5. `tools/comment-ceilings.txt`, by `--write-ledger` only
6. `docs/specifications/011-the-connection.md` and
   `docs/specifications/009-identity-and-joining.md`, where the sort or a
   swept block puts a sentence or an id; `cmd/vtt/scenario_goldens_test.go`,
   a citation line; `docs/verification-debt.md`, with the report

One component; the specification first, the register with it, the comment
sort after, the ledger last, in one commit; the report in its own.

## Specifications this moves

New: removing an actor — what `remove_actor` appends, in what order, and what
the fold refuses in the handler's place.
docs/specifications/013-authorization.md
docs/specifications/011-the-connection.md
docs/specifications/009-identity-and-joining.md

## What could not be established

- Whether `remove_actor`'s record is a specification of its own or a section
  of SPEC-007, which already states the batch's contents and order; the plan
  decides, and names the file.
- Whether any swept block holds a decision that SPEC-007, SPEC-009, SPEC-011
  or SPEC-013 lacks. If one does, the plan names the specification that takes
  the sentence, and this ticket's "Specifications this moves" grows by it.
- How many of the test files' blocks a re-aimed pointer forces to the bound;
  the plan lists them.
