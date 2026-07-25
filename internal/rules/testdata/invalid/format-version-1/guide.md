# Test Ruleset

A minimal ruleset fixture used to exercise the `internal/rules` loader and
interpreter in tests. Not a real game system.

## Abilities

- **Strike** — an at-will attack roll against `guard`; on hit, spends 1
  point of `pool_a`.
- **Guard Stance** — a limited-use ability (costs 1 `pool_a`) that applies
  the `guarded` condition.
- **Stand Down** — an at-will ability that removes `guarded`.

## Resources

- **pool_a** — defaults to `10 + brawn`; while non-zero, the actor is
  `guarded`.
