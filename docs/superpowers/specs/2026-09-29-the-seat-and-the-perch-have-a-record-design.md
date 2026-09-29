# The seat and the perch have a record

## The problem

What one connection is sent, and whose eyes a spectator watches through, is
decided in `internal/gateway/seat.go` (`viewerFor`, `projected`, `seat`,
`perchBox`, `newSeat`, `subscribeFrom`, `receive`, `pastResume`, `perch`,
`canSee`, `catchUp`) and `internal/gateway/viewpoint.go` (`MayPerch`), and no
specification states it. `projected` answers false for the DM and the agent
and true for every other role, one this build has never heard of included; an
unprojected seat has no `Projector` and receives each event unchanged. A
projected seat subscribes from sequence 0 whatever cursor it asked for, feeds
every event to its `Projector` against the state after that event
(`campaign.FoldPrefix` over what it has received, never the campaign's head),
withholds an event whose fold fails, and drops output at or below its resume
cursor (`pastResume`). `viewerFor` gives a connection no viewpoint, so a
spectator watches nobody until they name a shoulder. `MayPerch` lets only a
spectator perch, only on an actor `engine.IsPartyMember` accepts, lets an
empty actor id leave the shoulder, and answers one refusal string whether the
named actor exists or not. A perch is handed from the command goroutine to the
pump through `perchBox`, one slot where the latest shoulder wins and setting
never blocks; the pump's `perch` re-projects against the world the seat last
folded (`Projector.reperch`), and its output skips `pastResume`. `canSee` asks
the projection's `look` once and records nothing. `catchUp` projects the
backlog and answers the sequence this seat's catch-up ends at. Each of those
sentences lives today in a ticket's section
(`docs/superpowers/specs/2026-08-18-visibility-design.md` §3.1, §3.1.1, §4.1,
§4.4, §5.1 and §8), in a report (`docs/reports/2026-08-18-visibility.md`), or
in a comment block; SPEC-011 carries where a seat's subscription starts, that
its catch-up head is its own, that output at or below a cursor is dropped and
that the pump applies a perch; SPEC-013 the command path that reaches
`MayPerch` and `perchBox.set`, that `set` does not block, that a shoulder
replaced before the pump takes it is never applied and that a perch does not
survive a reconnect; SPEC-009 that a perch's frames are not re-checked;
SPEC-007 that a perch appends nothing; and nothing carries the rest. SPEC-013
sends the perch's rule to `viewpoint.go` twice and what a seat can see to
`seat.go` twice, once saying it has no record. `seat.go` holds 311 comment
lines of 430 non-blank and `viewpoint.go` 47 of 68 (`python3
tools/check-comments.py --report` gives 72.3 and 69.1 percent, 17 and 8 banned
lines, 11 and 1 blocks over the bound, at `74c547f`). Two sentences among them
are false: `receive`'s block says a fold failure is "unreachable while
campaign.Append is the only writer", and `campaign.AppendBatch` writes the log
too, and a second process on the same campaign directory, which nothing
prevents, reaches the arm; and `projected`'s doc says that making it answer
true for the DM and the agent "changes no byte on any wire", while a DM
resumed at the log's head is then told a `CatchUpHead` of 0.

## Done looks like

1. `docs/specifications/015-the-seat-and-the-perch.md` exists with the five
   headings SPEC-007 uses, in the present tense: which seats are projected and
   what an unprojected seat receives; how a projected seat is fed, against
   which state, and what it drops; that a fold failure withholds an event; the
   viewpoint a connection opens with; who may perch, on whom, and the one
   refusal; how a perch travels to the pump and is applied; `canSee`; and what
   this record does not decide, pointing at SPEC-011 for the subscription and
   the catch-up head, at SPEC-013 for the command path, at SPEC-007 for a perch
   appending nothing, and at `project.go` for what a projection computes.
   Every sentence names the symbol that holds it, and Phase 4b reads each
   against the code.
2. `grep -c 'campaign.Append is the only writer' internal/gateway/seat.go`
   and `grep -c 'changes no byte' internal/gateway/seat.go` each print 0;
   today each prints 1.
3. Every rule the sort accepts has a row cited by a test under
   `internal/gateway/` that observes it; `task check:requirements-chain`
   prints more than 175 rows and holds; the report names any OPEN row.
4. `python3 tools/check-comments.py --report | grep -E 'gateway/(seat|viewpoint)\.go'`
   prints `banned 0` and `blocks>6 0` on both lines, and every block left in
   them is a warning, a pointer or the doc sentence of an exported symbol
   (Phase 4b's reading, VTT-051). The ledger rows are lowered by
   `--write-ledger` in the same commit.
5. No code line changes: the go/scanner token stream of `seat.go`,
   `viewpoint.go` and every test file this touches is identical to
   `74c547f`'s.
6. SPEC-013's three sentences that send the perch's rule to `viewpoint.go`
   and what a seat can see to `seat.go` point at SPEC-015.
7. `task check` whole is green.

## Rules this puts on the system

Candidates, one line each, for the sort after sign-off; most have a test
already (`internal/gateway/viewpoint_test.go`, `viewpoint_internal_test.go`,
`server_visibility_test.go`, `project_test.go`).

- Only the DM and the agent receive the log unfiltered; every other role is
  projected (a role that does not parse is refused by `identity.Verify` before
  a seat exists).
- A projected seat that reconnects is sent exactly what it missed.
- A spectator watches nobody until they name a shoulder.
- A spectator may perch only on a party member; controlling an actor does not
  make it one.
- Only a spectator perches.
- A perch refusal reads the same whether the named actor exists or not.
- Naming no actor leaves the shoulder and is allowed.
- A perch appends nothing to the log.
- A perch is judged against the state after the last event the seat
  received, never the campaign's head.
- A perch's frames carry no sequence, and a resume cursor does not filter
  them.
- A burst of hops ends on the last shoulder, and a shoulder the burst flew
  past is restored by hopping back to it.
- Hopping while the table is busy keeps one order on the wire and stalls no
  other participant's command.

## What it touches

1. `docs/specifications/015-the-seat-and-the-perch.md`, new
2. `internal/gateway/seat.go` and `internal/gateway/viewpoint.go`, comments
   only
3. `internal/gateway/viewpoint_test.go`, `viewpoint_internal_test.go`,
   `server_visibility_test.go`, `authz_test.go`: citation lines where a row is
   dispensed, and any comment block a re-aimed pointer sits in, sorted to the
   bound (`check:comments` refuses a line added to a block over it);
   `internal/gateway/keystone_test.go`, the two blocks under the bound that
   quote `seat.go`'s cut comments; `project.go`'s pointer at "perchBox" is
   left and named
4. `docs/specifications/013-authorization.md`, the sentences that say the
   perch's rule has no record
5. `docs/requirements.md`, rows after sign-off, by the dispenser
6. `tools/comment-ceilings.txt`, by `--write-ledger` only
7. `docs/verification-debt.md`, one Open-debt entry for the fold-failure arm
   two processes reach, with the report

One component; the specification first, the register with it, the comment
sort after, the ledger last, in one commit; the report in its own.

## Specifications this moves

New: the seat and the perch — which seats are projected, how a projected seat
is fed and what it drops, who may perch on whom, and how a perch is applied.
docs/specifications/013-authorization.md

## What could not be established

- Where the seat ends and the projection begins. `Projector.Project`,
  `reperch`, `eyes`, `look` and the `Viewer` type sit in `project.go`, whose
  comment blocks and three unresolved rulings are the projection ticket's;
  this ticket states what the seat asks of them and points at the file. The
  sort decides whether a row whose test observes a projection's output
  (a perch arriving with the doors it can see already open, a shoulder left
  behind taking its creatures and not the terrain) is dispensed here.
- The fold-failure arm in `receive` is reachable only from outside the
  process: two `campaign.Open` handles on one directory both append, the log
  stops folding, and the next `Open` refuses it. No test observes the arm; the
  lock that would prevent the second writer is a ticket of its own.
- Which test files' comment blocks a re-aimed pointer forces to the bound;
  the plan lists them.
