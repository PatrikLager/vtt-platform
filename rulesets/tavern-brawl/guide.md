# Tavern Brawl

A minimal, deliberately silly ruleset for a bar fight. It exists to exercise
every feature of the ruleset format (P4: the platform is provably generic
across game systems) — not to be a serious combat system. Run it exactly
like any other ruleset: load it, add actors with the stats it declares, and
resolve `use_ability` commands against it.

## Attributes

- **brawn** — raw physical force. Drives every attack roll here.
- **grit** — how much punishment a patron can shrug off before the room
  starts spinning. Drives `drink`'s default capacity (see below).

## Defenses

- **footing** — how hard a patron is to knock off balance. Every attack in
  this ruleset rolls `1d20 + brawn` against the target's `footing`.

## Resources

- **drink** — how much ale has ended up on or in a patron. Defaults to
  `3 + grit`. While non-zero, the patron is `dazed-by-ale`; `sober-up`
  drains it back to exactly zero, which removes the condition again (this
  resource's threshold sets `remove_when_false: true` for exactly that
  reason — accumulation is reversible, not a one-way trip).
- **stamina** — how many big, showy moves a patron has left in them this
  scene. Defaults to `2`. `chair-swing` spends 1 per swing; there's no way
  to recover it in this v1 slice (no turn/rest engine — see the platform's
  non-goals).

## Conditions

- **dazed-by-ale** — stumbling, slow, easy to rattle. Purely a DM-narrated
  marker in this v1 format (conditions carry no mechanical enforcement of
  their own); applied automatically once `drink` goes non-zero, and
  directly by `chair-swing` on a solid hit (getting clocked with a chair
  is disorienting all by itself, independent of anything it happens to
  spill on you).

## Abilities

- **Fists** — an at-will attack. On a hit, splashes one point of `drink`
  onto the target (getting punched near a full mug rarely ends cleanly).
- **Chair Swing** — a limited-use attack (costs 1 `stamina`). On a hit,
  splashes `1 + 1d4` points of `drink` on the target AND applies
  `dazed-by-ale` directly — a good enough hit daze on its own, whether or
  not the `drink` threshold would have caught it too (it won't double up:
  the condition is idempotent, see below).
- **Sober Up** — an at-will, self-targeting (range 0) ability that drains
  the caster's own `drink` to exactly zero, clearing `dazed-by-ale` if the
  threshold had applied it.
- **Splash of Water** — an at-will ability (range 1) that removes
  `dazed-by-ale` from a target directly, independent of their `drink`
  level (a friend snapping you out of it, not the ale itself running out).

## A worked example

A patron with `brawn: 3` swings a chair (`chair-swing`) at a target with
`footing: 10` and `drink: 0`. The attack rolls `1d20 + 3`; suppose it comes
up 19 (hit). The batch: `AbilityUsed` (testimony, with both the attack
roll and the `1 + 1d4` damage roll recorded), a `ResourceChanged` spending
1 `stamina` from the caster, a `ResourceChanged` raising the target's
`drink`, and a `ConditionApplied` for `dazed-by-ale` (from the ability's
own hit list — since the target wasn't already dazed, the `drink`
threshold's own apply check is then a no-op, not a duplicate event).
