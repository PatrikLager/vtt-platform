# Tavern Brawl

A minimal, deliberately silly ruleset for a bar fight. It exists to exercise
every feature of the ruleset format (P4: the platform is provably generic
across game systems) — not to be a serious combat system. Run it exactly
like any other ruleset: load it, add actors with the stats it declares, and
resolve `use_ability` commands against it.

Format v2 (atoms + compositions): every ability below is built from this
ruleset's OWN atomic statements under `atoms/` — no borrowed vocabulary,
no shared atom with any other ruleset. See "Atoms" below for what each one
does; see "Abilities" for how they're composed.

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
  to recover it in this slice (no turn/rest engine — see the platform's
  non-goals).

## Conditions

- **dazed-by-ale** — stumbling, slow, easy to rattle. Purely a DM-narrated
  marker (conditions carry no mechanical enforcement of their own);
  applied automatically once `drink` goes non-zero, and directly by
  `chair-swing` on a solid hit (getting clocked with a chair is
  disorienting all by itself, independent of anything it happens to spill
  on you).

## Atoms

Six small, tavern-flavored statements, composed into the four abilities
below. None of these names, shapes, or mechanics are shared with any
other ruleset — they exist to model THIS ruleset's own drink-brawl idea in
its own words.

- **brawl-reach** (`reach`: int) — contributes targeting: how far a swing,
  a splash, or a moment of self-control reaches. Every ability composes
  this, bound to whatever range fits it.
- **footing-contest** (`attack_stat`: attribute) — the swing-vs-balance
  roll: `1d20 + attack_stat` against the target's `footing`, labeled
  `hit`/`miss`.
- **drink-soak** (`amount`: expr) — on a `footing-contest` hit, splashes
  `amount` points of `drink` onto the target. The amount is an ability's
  own choice: a flat splash for a bare fist, a sloppier `1 + 1d4` for a
  swung chair.
- **stagger-from-drink** — on a `footing-contest` hit, applies
  `dazed-by-ale` directly (getting hit by furniture is disorienting on its
  own, independent of anything it spills).
- **sober-swig** — unconditionally drains the ability's target's own
  `drink` to exactly zero.
- **towel-off** — unconditionally removes `dazed-by-ale` from the target.

## Abilities

- **Fists** — `brawl-reach(1)` + `footing-contest(brawn)` +
  `drink-soak(1)`: an at-will attack. On a hit, splashes one point of
  `drink` onto the target (getting punched near a full mug rarely ends
  cleanly).
- **Chair Swing** — `brawl-reach(1)` + `footing-contest(brawn)` +
  `drink-soak(1 + 1d4)` + `stagger-from-drink`: a limited-use attack
  (costs 1 `stamina`). On a hit, splashes `1 + 1d4` points of `drink` on
  the target AND applies `dazed-by-ale` directly — a good hit daze on its
  own, whether or not the `drink` threshold would have caught it too (it
  won't double up: the condition is idempotent, see below).
- **Sober Up** — `brawl-reach(0)` + `sober-swig`: an at-will,
  self-targeting (range 0) ability that drains the caster's own `drink` to
  exactly zero, clearing `dazed-by-ale` if the threshold had applied it.
- **Splash of Water** — `brawl-reach(1)` + `towel-off`: an at-will ability
  (range 1) that removes `dazed-by-ale` from a target directly,
  independent of their `drink` level (a friend snapping you out of it, not
  the ale itself running out).

## A worked example

A patron with `brawn: 3` swings a chair (`chair-swing`) at a target with
`footing: 10` and `drink: 0`. `footing-contest`'s roll is
`1d20 + @caster.brawn`; suppose it comes up 19 (hit). The batch:
`AbilityUsed` (testimony, with both the attack roll and `drink-soak`'s
`(1 + 1d4)` damage roll recorded — the parentheses are the composition
splice: `chair-swing` binds `drink-soak`'s `amount` param to the
expression `1 + 1d4`, and an `expr`-kind binding always splices in as a
parenthesized subtree, spec §2's hygiene guarantee), a `ResourceChanged`
spending 1 `stamina` from the caster, a `ResourceChanged` raising the
target's `drink`, and a `ConditionApplied` for `dazed-by-ale` (from
`stagger-from-drink` — since the target wasn't already dazed, the `drink`
threshold's own apply check is then a no-op, not a duplicate event).
