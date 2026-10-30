# The eight surviving mutants on the DM console's lock keys are adjudicated equivalent

## The problem

**The stored client mutation report is current and fails the step that
judges it.** `reports/mutation/mutation.json` and its stamp
`reports/mutation/ts-mutation-stamp.json` (both gitignored, so they exist on
the machine that ran `task check:ts-mutation:full`) measured `526885b` from a
clean tree. `python3 tools/ts-mutation-report.py check`, the command
`check:ts-mutation` runs as a step of `task check`, prints "the stored report
is current", judges it with `tools/check-ts-mutation.py`, prints eight
`SURVIVED StringLiteral` lines and exits 1. The report holds 3337 mutants: 71
survived, 63 of them claimed by entries in `tools/ts-mutation-equivalents.txt`
and 8 claimed by none.

**The eight are one mutation on one argument.** Each replaces the `key`
argument of `sending(d, key, press)` in `client/src/view/dm.ts` with `""`,
at one of its eight calls whose key is a string literal: `"add-narration"`
(the console's Say), `"start-session"`, `"add-actor"`, `"remove-actor"`,
`"actor-json"`, `"place-token"`, `"remove-token"` and `"upsert-note"`. The two
other calls build their key with a template literal,
`` `describe-token:${id}` `` and `` `grant-actor-control:${a.actorId}` ``.
`sending` uses the key for one thing: the member it asks for, adds to and
deletes from the module's `unanswered` Set. Nothing else in `client/` reads
that Set, and `sending` never iterates it or reads its size. A key mutated to
`""` is a member no other call's key equals: the seven other literals are
non-empty and differ from each other, and both template keys begin with a
non-empty prefix. So under each mutant the mutated control still holds back
itself, which VTT-523 requires of the seven forms and VTT-535 of the
console's Say, and no other control, which VTT-541 requires. The tests that
hold all three pass.

## Done looks like

1. `python3 tools/ts-mutation-report.py check`, against the stored report
   that measured `526885b`, exits 0 and prints no `SURVIVED` line. On
   `526885b` it prints eight and exits 1.
2. Each entry fails when it is missing. With any one of the eight entries
   deleted, the same command exits 1 and prints `SURVIVED` for that mutant
   alone.
3. The adjudications are true of the code. In a scratch copy of the tree,
   each of the eight keys replaced by `""` in turn leaves
   `bun test client/test` passing. This is the survivor reproduced by hand,
   which is the evidence an equivalence entry stands on.
4. `task check` exits 0 on the branch. Its `check:ts-mutation` step reports
   the stored report current and judges it, rather than printing THE
   CLIENT'S MUTATION WAS NOT MEASURED.
5. `git diff main -- tools/ts-mutation-equivalents.txt` adds eight entries
   and deletes no line.

## Rules this puts on the system

None. An equivalence entry is an adjudication, recorded where the project's
record says adjudications go. It adds no requirement. The rule the lock
holds is VTT-541, and it does not change.

## What it touches

- `tools/ts-mutation-equivalents.txt`: eight entries in its
  `client/src/view/dm.ts` section.
- `docs/reports/2026-10-30-the-console-lock-keys-are-equivalent.md`: new,
  the implementation report.

Not `client/src`, `client/test` or anything else the report's input hash
reads. An edit there makes the stored report out of date, and the step would
pass without judging it.

## Specifications this moves

None.

## What could not be established

- Whether the eight survive on another machine. The report is local and
  gitignored, and a fresh clone has none, so its `check:ts-mutation`
  prints THE CLIENT'S MUTATION WAS NOT MEASURED and passes. Item 3's hand
  mutations are the part another machine can repeat.
- The run's nine timed-out mutants (two in `client/src/view/canvas.ts`, six
  in `client/src/view/scene-plan.ts`, one in `client/src/wire.ts`). The
  checker counts them as killed and says the count measures the machine,
  whose one-minute load average read about 7 both times it was looked at
  during the run (the log records no load). They are outside this ticket.
