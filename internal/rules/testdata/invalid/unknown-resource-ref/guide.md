# Test Ruleset

A minimal format-v2 ruleset fixture used to exercise the `internal/rules`
loader and interpreter in tests. Not a real game system.

## Atoms

- **melee-delivery** (`reach`: int) — targeting at the given reach.
- **self-delivery** — targeting at range 0.
- **strike-roll** (`attack_stat`: attribute) — `1d20 + attack_stat` vs
  `guard`, labeled `hit`/`miss`.
- **strike-damage** (`power`: int) — on a `strike-roll` hit, spends
  `power` points of `pool_a`.
- **apply-guarded** / **remove-guarded** — unconditionally apply/remove
  `guarded`.

## Abilities

- **Strike** — an at-will attack roll against `guard`; on hit, spends 1
  point of `pool_a`.
- **Guard Stance** — a limited-use ability (costs 1 `pool_a`) that applies
  the `guarded` condition.
- **Stand Down** — an at-will ability that removes `guarded`.

## Resources

- **pool_a** — defaults to `10 + brawn`; while non-zero, the actor is
  `guarded`.
