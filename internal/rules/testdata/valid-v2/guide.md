# Proving Grounds

A minimal format-v2 ruleset fixture used to exercise `internal/rules`'
atom/composition loader and compiler in tests. Not a real game system —
`vim`/`vigor`/`brace`/`focus` are generic invented vocabulary.

## Abilities

- **Quick Jab** — an at-will clash (attack) at reach 1: `1d20 + vim + 2`
  vs. `brace`. On a `connect`, the target's `focus` drops by
  `caster vigor + 3`.
- **Rally** — a limited-use ability (costs 1 `focus`) that unconditionally
  raises the target's `focus` by 5.

## Resources

- **focus** — defaults to `10 + vim`; while non-zero, the actor is
  `winded`.
