# D&D 4.5e (Minimal)

Patrik's 4.5e house-rules conventions, demo-minimal slice: a goblin fight's
worth of content — enough to run a real skirmish through `use_ability`, not
a full 4.5e character-building system. `1d20 + attack stat + proficiency`
vs a named defense; NO half-level bonus anywhere (it's stripped from both
attacks and defenses in this house system, since they cancel out); bloodied
at half hp. Proficiency and any enhancement bonus are already baked into
each ability's roll expression — there is no separate equipment layer in
this slice (future work).

Format v2 (atoms + compositions): every ability below is built from this
ruleset's OWN atomic statements under `atoms/` — no borrowed vocabulary, no
shared atom with any other ruleset. See "Atoms" below for what each one
does; see "Abilities" for how they're composed.

## Attributes

- **str** — melee attack/damage stat (longsword, staggering blow).
- **dex** — ranged and finesse attack/damage stat (goblin weapons, crossbow,
  hunter's flurry).
- **con** — drives `rally`'s recovery amount.
- **max_hp** — this creature's maximum hit points, duplicated as an
  attribute (see "The max_hp duplication rule" below). int/wis/cha are
  omitted from this slice: nothing here reads them.

## Defenses

- **ac** — Armor Class. Every weapon attack in this slice rolls against it.
- **fort** — Fortitude. `staggering-blow` attacks Fortitude instead of AC.
- **ref**, **will** — declared for future powers; nothing in this slice
  attacks them yet.

Format v2 convention: a defense's runtime value lives in the SAME
`attributes` map an actor's attributes do (there is no separate `defenses`
object anywhere in the wire format or in `add_actor`) — a resolution's
`vs` expression reads it as `@target.ac`, exactly like an attribute ref.
The split into "Attributes" / "Defenses" sections above is purely for this
guide's readability; when you call `add_actor`, put `ac`/`fort`/`ref`/`will`
directly alongside `str`/`dex`/`con`/`max_hp` in one `attributes` object —
see the reference statblocks below.

## Resources

- **hp** — current hit points. No `default_max_expr`: every statblock sets
  both `current` and `max` explicitly when the DM calls `add_actor` (there
  is no formula to derive max hp from attributes in this slice — it's a
  fixed statblock number). Two thresholds fire automatically as hp changes:
  - **bloodied** — fires once `hp * 2 <= max_hp`. The grammar has no
    comparison operators (a threshold's `when` is "true" exactly when it
    evaluates non-zero), so this is written as the clamped idiom
    `max(0, @max_hp - #hp * 2 + 1)`: strictly positive while at or below
    half hp, exactly `0` the instant hp rises above half. `remove_when_false:
    true`, so healing back above half removes `bloodied` again — it's not a
    one-way trip.
  - **dying** — fires once `hp` hits exactly `0` (hp is floor-clamped at 0 by
    the platform, so "hp == 0" and "hp <= 0" are the same event here):
    `max(0, 1 - #hp)`. Also `remove_when_false: true` — any healing off of 0
    hp clears `dying` automatically.
- **flurry_uses** — fuel for `hunters-flurry`, the one encounter power in
  this slice. Only actors who actually have `hunters-flurry` on their sheet
  need this resource at all (see the human fighter statblock below); a
  goblin never needs it. Set `current: 1, max: 1` for a fresh encounter;
  once spent it does not refill in this slice (no rest/encounter engine —
  see the platform's non-goals). No thresholds.

### The max_hp duplication rule

An expression can read an attribute (`@x`) or a resource's CURRENT value
(`#x`) — there is no way to read a resource's MAX from inside an
expression (format v2 carries this v1 constraint forward unchanged). Since
the `bloodied` threshold needs to compare current hp against max hp, every
statblock in this ruleset carries a `max_hp` **attribute** whose value must
be kept equal to the `hp` resource's `max`. When you `add_actor`, set both:

```
attributes: { ..., "max_hp": 28 }
resources:  { "hp": { "current": 28, "max": 28 }, ... }
```

If you ever change a creature's max hp mid-campaign (level up, a buff),
update BOTH the `max_hp` attribute and the `hp` resource's `max` together —
they are two copies of the same number by construction, not two
independent facts.

## Conditions

- **bloodied** — at or below half hp; a pure marker (some 4.5e powers key
  off it, none in this slice do yet). Applied/removed automatically by the
  `hp` threshold above.
- **dying** — at 0 hp, unconscious and failing; needs stabilizing or
  healing. Applied/removed automatically by the `hp` threshold above.
- **dazed** — can act only once per turn, grants combat advantage, no
  opportunity actions. Applied directly by `staggering-blow` on a hit (not
  threshold-driven — it's a mechanical result of the attack, not a hp
  level). Remove it manually (`remove_condition`) when the DM judges the
  effect has run its course — this ruleset's conditions carry no automatic
  duration or save-ends timer of their own (conditions are DM-narrated
  markers; the platform tracks structurally only).

## Atoms

Seven small, D&D-flavored statements, composed into the seven abilities
below. None of these names, shapes, or mechanics are shared with any other
ruleset — they model THIS ruleset's own attack-roll-vs-defense idea in its
own words.

- **melee-delivery** (`reach`: int) — contributes targeting for a
  hand-to-hand ability: range `reach`, one target. Composed by every melee
  ability, and by `rally` too (its "self or adjacent" range is the same
  shape as a melee reach).
- **ranged-delivery** (`distance`: int, `targets`: int) — contributes
  targeting for a ranged ability: range `distance`, up to `targets`
  simultaneous targets. `hunters-flurry` is the one ability in this slice
  that binds `targets` above 1 — everything else binds it to 1.
- **weapon-attack-roll** (`attack_stat`: attribute, `prof`: int, `defense`:
  defense) — the universal attack roll: `1d20 + attack_stat + prof` against
  the target's named `defense`, labeled `hit`/`miss`. Every attack in this
  slice — all six of goblin-scimitar, goblin-shortbow, longsword-strike,
  crossbow-shot, hunters-flurry, and staggering-blow — composes this SAME
  atom, differing only in which stat, which flat bonus, and which defense
  they bind. This is the ruleset's biggest single piece of reuse: one
  roll shape, six abilities.
- **weapon-damage** (`damage_expr`: expr) — on a `weapon-attack-roll` hit,
  subtracts `damage_expr` points from the target's `hp`. Composed by all
  six attacking abilities, each supplying its own dice-plus-stat damage
  expression (a flat `1d6 + 2` for a goblin's baked-in proficiency, a
  caster-scoped `1d8 + @caster.str` for a PC's weapon swing, and so on).
- **graze-damage** (`damage_expr`: expr) — on a `weapon-attack-roll` MISS,
  subtracts `half(damage_expr)` points from the target's `hp`. Composed
  ONLY by `hunters-flurry`, bound to the exact same `damage_expr` its own
  `weapon-damage` composition uses — Hunter's Flurry is the one power in
  this slice whose arrows still graze on a miss.
- **condition-rider** (`condition`: condition) — on a `weapon-attack-roll`
  hit, applies `condition` to the target directly (independent of any hp
  threshold). Composed ONLY by `staggering-blow`, bound to `dazed` — the
  daze is the blow's own mechanical result, not a side effect of hp loss.
- **recovery** (`amount`: expr) — unconditionally raises the ability's
  target's `hp` by `amount`. Composed ONLY by `rally`, bound to
  `1d6 + @target.con` — the recipient's OWN constitution drives how much
  they rally back, whether that recipient is the caster themselves or an
  ally standing next to them. This is a raw positive `hp` change, not a
  healing-surge system (out of scope for this slice) — just morale
  recovery that can push a bloodied or dying creature back over those
  thresholds, removing the condition automatically.

### Caster vs. target scoping

Every two-actor expression in this ruleset (every `weapon-attack-roll`
roll/vs, every `weapon-damage`/`graze-damage` delta, `recovery`'s amount)
must name which actor a ref reads from. The rule of thumb this ruleset
follows throughout: **whoever's stat drives the number is who it's scoped
to.** Weapon damage flows attacker → victim, so the attacker's stat is
`@caster.str`/`@caster.dex` even though the `hp` resource that actually
changes belongs to the target. Rally's recovery is scaled off the
RECIPIENT's own toughness, so its `con` ref is `@target.con` — the same
whether you rally yourself or shout encouragement to an ally standing next
to you (target and caster happen to be the same actor for a self-cast, but
the expression is written from the recipient's point of view either way).
Thresholds (`bloodied`/`dying`) stay bare, unscoped refs — they're a
single-actor position (the resource's own owner), unchanged from v1.

## Abilities

- **Goblin Scimitar** — `melee-delivery(1)` + `weapon-attack-roll(dex, 2,
  ac)` + `weapon-damage(1d6 + 2)`: a goblin's melee basic attack. On a hit,
  `1d6 + 2` damage (the `+2` is the goblin's own baked-in proficiency, not
  a stat — goblins don't carry a damage-relevant `str`/`dex` bonus here).
- **Goblin Shortbow** — `ranged-delivery(10, 1)` + `weapon-attack-roll(dex,
  2, ac)` + `weapon-damage(1d8)`: a goblin's ranged basic attack (range
  10). On a hit, flat `1d8` damage.
- **Longsword Strike** — `melee-delivery(1)` + `weapon-attack-roll(str, 3,
  ac)` + `weapon-damage(1d8 + @caster.str)`: the fighter's melee basic
  attack. On a hit, `1d8` plus the fighter's OWN `str` modifier.
- **Crossbow Shot** — `ranged-delivery(15, 1)` + `weapon-attack-roll(dex,
  2, ac)` + `weapon-damage(1d8)`: the fighter's ranged basic attack (range
  15). On a hit, flat `1d8` damage.
- **Hunter's Flurry** — `ranged-delivery(10, 2)` + `weapon-attack-roll(dex,
  2, ac)` + `weapon-damage(1d8 + @caster.dex)` +
  `graze-damage(1d8 + @caster.dex)`: the fighter's one encounter power
  (costs 1 `flurry_uses`), up to two simultaneous targets at range 10. On a
  hit, `1d8 + dex` damage; on a MISS, still `half(1d8 + dex)` — the damage
  die is rolled and recorded even on a miss (every roll an ability makes is
  recorded on `AbilityUsed`, hit or miss — the platform's testimony
  contract).
- **Staggering Blow** — `melee-delivery(1)` + `weapon-attack-roll(str, 3,
  fort)` + `weapon-damage(1d6 + @caster.str)` + `condition-rider(dazed)`:
  the fighter's melee attack against Fortitude instead of AC. On a hit,
  `1d6 + str` damage AND applies `dazed` directly — the daze is the
  attack's own effect, independent of whatever the `bloodied`/`dying`
  thresholds do to that same hit's damage.
- **Rally** — `melee-delivery(1)` + `recovery(1d6 + @target.con)`: a
  non-attack power (range 1, self or an adjacent ally); no attack roll, no
  hit/miss. Unconditionally heals `1d6` plus the recipient's own `con`
  modifier. Not a healing-surge system — just morale recovery that can
  push a bloodied or dying creature back over those thresholds, removing
  the condition automatically.

## A worked example

The fighter (`str: 4`) swings `longsword-strike` at a goblin (`ac: 15`,
`max_hp: 8`, `hp: 8/8`). `weapon-attack-roll`'s roll is
`1d20 + @caster.str + (3)`; suppose it comes up 9 (total 16, a hit against
15). The batch: `AbilityUsed` (testimony, carrying both the attack roll and
`weapon-damage`'s `0 - (1d8 + @caster.str)` damage roll — the parentheses
around `(3)` and around `(1d8 + @caster.str)` are the composition splice:
an `int`- or `expr`-kind param binding always splices in as a parenthesized
subtree, format v2's hygiene guarantee — see the compiled-form goldens
under `goldens/compiled/` for exactly what each ability flattens to), a
`ResourceChanged` lowering the goblin's `hp`, and — if that hit brought the
goblin to or below half hp — a `ConditionApplied` for `bloodied`, fired
automatically by the `hp` resource's own threshold, not by the ability
itself.

## Suggested turn flow

This slice has no initiative/turn engine (platform non-goal) — the DM
narrates turn order and calls `use_ability` for whichever token acts next:

1. Goblins open with their basic attacks (`goblin-scimitar` if adjacent,
   `goblin-shortbow` at range).
2. The fighter answers with `longsword-strike` in melee or `crossbow-shot`
   at range; save `hunters-flurry` for when two goblins are close together
   (it's the one power that can hit both at once).
3. `staggering-blow` is a good opener against a dangerous single target —
   the `dazed` condition buys the party a round.
4. If the fighter (or an ally) drops toward `bloodied` or `dying`, `rally`
   can pull them back — narrate it as a shout of encouragement, a
   second wind, whatever fits the table.
5. Thresholds fire on their own — the DM never needs to manually apply or
   remove `bloodied`/`dying`; the engine does it every time `hp` changes.
   `dazed` (and any other narrated condition) IS the DM's job to clear with
   `remove_condition` once the effect has run its course.

## Reference statblocks

Copy these attribute/resource maps into `add_actor` verbatim (adjust `x`/`y`
for where the token starts). Remember the max_hp duplication rule above —
`max_hp` (attribute) always equals `hp`'s `max` (resource). Per format v2
convention, `ac`/`fort`/`ref`/`will` live directly IN the `attributes` map,
alongside `str`/`dex`/`con`/`max_hp` — there is no separate defenses object.

### Goblin cutter

```
attributes: { "str": 1, "dex": 3, "con": 1, "max_hp": 8,
              "ac": 15, "fort": 12, "ref": 14, "will": 11 }
resources: { "hp": { "current": 8, "max": 8 } }
abilities: goblin-scimitar
```

### Goblin archer

```
attributes: { "str": 0, "dex": 4, "con": 1, "max_hp": 6,
              "ac": 14, "fort": 11, "ref": 15, "will": 11 }
resources: { "hp": { "current": 6, "max": 6 } }
abilities: goblin-shortbow, goblin-scimitar
```

### Human fighter

```
attributes: { "str": 4, "dex": 2, "con": 3, "max_hp": 28,
              "ac": 17, "fort": 15, "ref": 13, "will": 12 }
resources: { "hp": { "current": 28, "max": 28 },
              "flurry_uses": { "current": 1, "max": 1 } }
abilities: longsword-strike, crossbow-shot, hunters-flurry, staggering-blow, rally
```

Only actors that actually have `hunters-flurry` on their ability list need
the `flurry_uses` resource — a goblin never does.

## Demo runbook

To run this ruleset live:

1. Start a fresh campaign server pointed at this ruleset:
   ```
   vtt serve --campaign <fresh> --ruleset rulesets/dnd45e-minimal
   ```
2. Invite the DMing agent:
   ```
   vtt invite --role agent
   ```
3. In the agent's `.mcp.json`, reuse the existing entry from the earlier
   demo and add `--ruleset` to the `vtt mcp` args (so `get_ruleset_guide`
   can serve this document):
   ```json
   {
     "mcpServers": {
       "vtt": {
         "command": "vtt",
         "args": ["mcp", "--ruleset", "rulesets/dnd45e-minimal"]
       }
     }
   }
   ```
4. Suggested opening prompt for the DMing agent:
   > Read the ruleset guide with `get_ruleset_guide`, then set the scene
   > and add the fighter and two goblins from the reference statblocks
   > above (adjust starting positions so the goblins are within range of
   > the fighter). Run the fight turn by turn with `use_ability`, narrating
   > each roll's outcome, applying `staggering-blow`'s `dazed` narratively
   > when it lands, and calling `remove_condition` when a narrated effect
   > ends. Let the automatic `bloodied`/`dying` thresholds speak for
   > themselves in your narration.

Patrik watches the fight live; his acceptance of the session is the merge
gate for this ruleset.
