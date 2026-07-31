# What check:ts-mutation gates, and what it does not yet

`stryker.conf.json`'s `mutate` list is the gated set. It is deliberately
NARROWER than `client/src/**/*.ts` today, and this file records exactly how
much narrower, so the gap is a number somebody can act on rather than an
impression.

The rule this follows is the one in tools/check-mutation.py: a gate scoped to
what its author happened to touch is not a gate. So the full client WAS
measured before scoping — 1667 mutants, 4m55s, **63.89% mutation score with
602 survivors**, against 95.91% line coverage. That gap is the entire argument
for this gate existing.

## Gated

| file | survivors | note |
|---|---|---|
| `client/src/undo.ts` | 0 unadjudicated | 17 -> 0 by writing boundary tests; 2 adjudicated equivalent |

## Measured, not yet gated

Baseline taken 2026-07-31 on the full-client run. Each of these must reach zero
unadjudicated survivors before it joins `mutate`.

| file | survivors |
|---|---|
| `client/src/view/dm.ts` | 166 |
| `client/src/view/player.ts` | 89 |
| `client/src/view/spectator.ts` | 63 |
| `client/src/fold.ts` | 93 |
| `client/src/app.ts` | 48 |
| `client/src/view/feed.ts` | 42 |
| `client/src/wire.ts` | 22 |
| `client/src/player.ts` | 22 |
| `client/src/view/grid.ts` | 15 |
| `client/src/commands.ts` | 11 |
| `client/src/metadata.ts` | 11 |
| `client/src/auth.ts` | 2 |
| `client/src/session.ts` | 1 |
| `client/src/state.ts` | 0 (already clean) |

`fold.ts` is the one to do next despite not being the largest: it is the
parity keystone, and a mutant that survives there is a way the client's
derived state can disagree with the server's without any test noticing.

## Why this is not in `task check` yet

Because it would be a gate over one file while thirteen sit unmeasured by it,
and `task check` going green would then mean less than it does today. It runs
on demand (`task check:ts-mutation`) until the list above is worked down far
enough that gating is honest. The same judgement, for the same reason, keeps
internal/harness out of check:mutation on the Go side.

## A trap this cost an hour

Stryker's JSON reports a `replacement` string but NOT which sub-expression it
replaces. At undo.ts:39 the line is

    if (best === null || e.sequence > best) best = e.sequence;

and there are mutants at 39:9, 39:26 and two more, with replacements `false`,
`true`, `e.sequence >= best`. Reading `39:9 -> false` as "the whole condition
becomes false" is wrong: 39:9 is the LEFT OPERAND, `best === null`, and the
mutated line still assigns via the right operand. Hand-applying the reading
rather than the actual mutant produced a suite failure and very nearly a bug
report against Stryker. Read the instrumented file in `.stryker-tmp` (set
`cleanTempDir: false`) before concluding a mutant is misreported.
