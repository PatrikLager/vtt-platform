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

**Conditions are narration, not enforcement.** The engine tracks which
conditions are on which actor, and fires `bloodied`/`dying` automatically
off the `hp` thresholds above — but it never READS a condition anywhere
inside `use_ability`'s resolution. A `dying` (0 hp) or `dazed` actor can
still be the caster OR the target of any ability, with no rejection, no
penalty, nothing: the platform will happily let a creature at 0 hp swing a
weapon, or let a `dazed` actor act as many times as the DM calls
`use_ability` for it. "Unconscious and failing" (`dying`) and "can act
only once per turn" (`dazed`) describe what the FICTION says that creature
can do, not a rule the engine enforces. Stopping a dying goblin from
striking back, or capping a dazed fighter at one action, is entirely on
the DM's narration at the table — the same call a human DM would make,
just without any engine backup.

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

Each JSON block below is the EXACT `actor` argument to pass to `add_actor`
— copy it verbatim, wire field for wire field. `add_actor`'s wire schema is
`actorId`/`name`/`controllerId`/`attributes`/`resources` (plus the opaque
`moduleId`/`moduleData` pair this slice doesn't use) — there is NO `x`/`y`
field and NO `abilities` field, and the MCP layer rejects unknown fields
outright (`mcp: invalid arguments for add_actor: ...`). Positioning is a
SEPARATE call, `place_token`, made AFTER `add_actor` — see "Combat setup
sequence" below. Each statblock's "Ability lists" note explains why
abilities aren't part of the payload either.

Remember the max_hp duplication rule above — `max_hp` (attribute) always
equals `hp`'s `max` (resource). Per format v2 convention, `ac`/`fort`/
`ref`/`will` live directly IN the `attributes` map, alongside `str`/`dex`/
`con`/`max_hp` — there is no separate defenses object.

### Goblin cutter

```json
{
  "actorId": "act-cutter",
  "name": "Goblin Cutter",
  "attributes": {"str": 1, "dex": 3, "con": 1, "max_hp": 8,
                 "ac": 15, "fort": 12, "ref": 14, "will": 11},
  "resources": {"hp": {"current": 8, "max": 8}}
}
```

**Ability lists (narrative discipline):** goblin-scimitar. The platform
does not enforce per-actor ability lists anywhere on the wire —
`use_ability` accepts any ability from this ruleset called from ANY actor,
regardless of what it "should" know. Which creature knows which move is
the DM's discipline to maintain in the fiction, not something the engine
checks or rejects.

### Goblin archer

```json
{
  "actorId": "act-archer",
  "name": "Goblin Archer",
  "attributes": {"str": 0, "dex": 4, "con": 1, "max_hp": 6,
                 "ac": 14, "fort": 11, "ref": 15, "will": 11},
  "resources": {"hp": {"current": 6, "max": 6}}
}
```

**Ability lists (narrative discipline):** goblin-shortbow, goblin-scimitar.
Same caveat as above — nothing on the wire binds these two abilities to
this actor specifically; it is narration, not a platform rule.

### Human fighter

```json
{
  "actorId": "act-fighter",
  "name": "Human Fighter",
  "attributes": {"str": 4, "dex": 2, "con": 3, "max_hp": 28,
                 "ac": 17, "fort": 15, "ref": 13, "will": 12},
  "resources": {"hp": {"current": 28, "max": 28},
                "flurry_uses": {"current": 1, "max": 1}}
}
```

**Ability lists (narrative discipline):** longsword-strike, crossbow-shot,
hunters-flurry, staggering-blow, rally. Same caveat again — this is a note
for the DM's own bookkeeping, not a wire-enforced list. The `flurry_uses`
**resource**, by contrast, IS wire-real: it's what `hunters-flurry` actually
spends, and it only exists on an actor whose `add_actor` call included it
in `resources` — the Human Fighter above is the only statblock in this
guide that needs it; neither goblin statblock carries `flurry_uses` at
all.

## Combat setup sequence

`use_ability` hard-requires a placed token for the caster AND for every
target — range resolution reads token positions, and an actor with no
token anywhere fails outright (`rules: resolve: actor "..." has no token
placed (cannot determine range)`). The wire order that gets every
combatant to a usable state, every time:

1. `create_scene` — once, for the encounter map.
2. `add_actor` — once per combatant, using the statblocks above verbatim.
3. `place_token` — once per combatant, in that scene, at a starting
   position. Positioning happens HERE, not in `add_actor` (which has no
   `x`/`y` field — see "Reference statblocks" above).
4. THEN, and only then, `use_ability` — call it before every combatant has
   a placed token and the very first attack fails with "no token placed".

Range is Chebyshev distance between the two tokens (diagonals cost the
same as orthogonal moves — a king's-move metric), checked against the
ability's declared range. This slice's ranges: melee abilities
(`goblin-scimitar`, `longsword-strike`, `staggering-blow`, `rally`) reach 1
square; `goblin-shortbow` and `hunters-flurry` reach 10; `crossbow-shot`
reaches 15. Place the goblins within melee/shortbow reach of the fighter
(and of each other, if you want `hunters-flurry` to catch both at once) —
not out past crossbow range, or even the fighter's opening melee attack
will be out of range.

## Demo runbook

To run this ruleset live:

1. Start a fresh campaign server pointed at this ruleset:
   ```
   vtt serve --campaign <fresh> --addr :8443 --ruleset rulesets/dnd45e-minimal
   ```
2. Invite the DMing agent — `--campaign` must be the exact SAME file
   `serve` just opened in step 1; `--name` is any label for this
   participant:
   ```
   vtt invite --campaign <same file as serve> --name claude-dm --role agent
   ```
   This prints the token exactly once — capture it now, it cannot be
   recovered later:
   ```
   participant id: <id>
   token (shown once — store it now, it cannot be recovered): <token>
   ```
3. In the SAME shell you are about to launch Claude Code from, capture
   that token into the environment without it ever landing in shell
   history:
   ```
   read -s VTT_TOKEN && export VTT_TOKEN
   ```
   (prompts silently, nothing echoed, nothing to scroll back through — or
   set `HISTIGNORE='export VTT_TOKEN=*'` first if you'd rather type it
   directly). Do this BEFORE opening Claude Code: a subprocess only
   inherits the environment of the shell it was launched from, and `vtt
   mcp` reads `VTT_TOKEN` from its own process environment
   (`resolveMCPToken`'s env fallback; see also the top-level README's
   "Claude Code" section) — no token value is ever written to `.mcp.json`
   itself.
4. Point the agent's `.mcp.json` at the running gateway AND this ruleset:
   ```json
   {
     "mcpServers": {
       "vtt": {
         "command": "vtt",
         "args": ["mcp", "--server", "ws://localhost:8443/ws", "--ruleset", "rulesets/dnd45e-minimal"]
       }
     }
   }
   ```
   `--server` must match the `--addr` `serve` used in step 1; `--ruleset`
   is what makes `get_ruleset_guide` serve this document.
5. Open Claude Code from that SAME shell (so the subprocess inherits
   `VTT_TOKEN`) with this repo's `.mcp.json` in scope.
6. Suggested opening prompt for the DMing agent:
   > Read the ruleset guide with `get_ruleset_guide`. Then follow the
   > "Combat setup sequence" it describes: `create_scene`, then
   > `add_actor` for the fighter and two goblins from the reference
   > statblocks, then `place_token` for all three — positioned so the
   > goblins start within the fighter's melee/shortbow range, not out at
   > crossbow range. Only once every combatant has a placed token, run the
   > fight turn by turn with `use_ability`, narrating each roll's outcome,
   > applying `staggering-blow`'s `dazed` narratively when it lands, and
   > calling `remove_condition` when a narrated effect ends. Remember: the
   > engine never stops a dying or dazed actor from acting — that
   > enforcement is entirely yours as narrator. Let the automatic
   > `bloodied`/`dying` thresholds speak for themselves in your narration.

Patrik watches the fight live; his acceptance of the session is the merge
gate for this ruleset.
