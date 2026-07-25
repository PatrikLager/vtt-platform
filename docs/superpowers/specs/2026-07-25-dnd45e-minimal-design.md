# dnd45e-minimal — Design Spec (sub-project 5b)

**Date:** 2026-07-25
**Status:** Approved design (brainstorming output)
**Parent:** 5a spec (`2026-07-25-ruleset-interpreter-design.md`) — format v1 is
FROZEN; this sub-project is pure content plus a wire scenario. Pillars P2/P4:
the ruleset ships with ZERO platform changes and passes the untouched
conformance suite.

## 1. Purpose

The first REAL ruleset: Patrik's 4.5e house system, demo-minimal slice, as
pure data under `rulesets/dnd45e-minimal/`. Acceptance is the demo rematch —
Patrik watches Claude DM a real goblin fight through `vtt mcp` with real
rules. Merge is gated on the demo (Patrik-decided, like the MCP gateway's).

## 2. Decisions (locked in brainstorming, 2026-07-25)

1. **Scope = demo-minimal:** the goblin-fight essentials only (~6 abilities,
   3 conditions, hp thresholds). Small enough to golden exhaustively.
2. **Fidelity = Patrik's 4.5e conventions:** `1d20 + attack stat +
   proficiency` vs a named defense, NO half-level anywhere, bloodied at half
   hp. Proficiency and enhancement are baked into each ability's roll
   expression (v1 has no equipment layer — that is future work).
3. **Demo rematch is the merge gate.**

## 3. The v1 max-reference workaround (documented constraint)

v1 expressions can reference attributes (`@x`) and CURRENT resource values
(`#x`) — not a resource's max. Bloodied-at-half therefore uses a statblock
attribute: every statblock declares `max_hp` (equal to the hp resource's
max). The guide documents this duplication rule for DMs adding actors.
Carry-forward for format v2: a max-reference (e.g. `#hp.max`).

A second v1 constraint: the closed grammar has NO comparison operators —
a threshold's `when` is true when the expression is non-zero. Boolean
conditions are therefore written as clamped arithmetic using the
`max(0, …)` idiom (documented in the guide):
- bloodied (`hp*2 <= max_hp`): `max(0, @max_hp - #hp * 2 + 1)`
- dying (`hp == 0`, hp is floor-clamped): `max(0, 1 - #hp)`
Carry-forward for format v2: comparison operators in the grammar.

## 4. Ruleset content

Directory `rulesets/dnd45e-minimal/`:

- **ruleset.json** — attributes: `str, dex, con, max_hp` (int/wis/cha
  omitted: nothing in the slice reads them; YAGNI); defenses: `ac, fort,
  ref, will`; resources:
  - `hp` (no default_max_expr — statblocks set max explicitly), thresholds:
    - `{when: "max(0, @max_hp - #hp * 2 + 1)", apply_condition:
       "bloodied", remove_when_false: true}`
    - `{when: "max(0, 1 - #hp)", apply_condition: "dying",
       remove_when_false: true}`
  - `flurry_uses` — fuel for the encounter power (max 1 in statblocks).
- **abilities/** (seven; Patrik's conventions, prof baked into the roll; NO
  unary minus in the grammar, so damage delta_exprs are written `0 - (…)`):
  - `goblin-scimitar` — melee (range 1), roll `1d20 + @dex + 2` vs `ac`,
    hit: resource_change hp `0 - (1d6 + 2)`.
  - `goblin-shortbow` — ranged (range 10), roll `1d20 + @dex + 2` vs `ac`,
    hit: resource_change hp `0 - (1d8)`.
  - `longsword-strike` — PC melee basic (range 1), roll `1d20 + @str + 3`
    vs `ac`, hit: resource_change hp `0 - (1d8 + @str)`.
  - `crossbow-shot` — PC ranged basic (range 15), roll `1d20 + @dex + 2`
    vs `ac`, hit: resource_change hp `0 - (1d8)`.
  - `hunters-flurry` — encounter power: limited `{resource: flurry_uses,
    cost: 1}`, range 10, max_targets 2, roll `1d20 + @dex + 2` vs `ac`,
    hit: resource_change hp `0 - (1d8 + @dex)`, miss: resource_change hp
    `0 - half(1d8 + @dex)` (exercises miss lists; dice in outcomes are
    recorded per 5a's testimony contract).
  - `staggering-blow` — melee (range 1), roll `1d20 + @str + 3` vs `fort`,
    hit: resource_change hp `0 - (1d6 + @str)` then `apply_condition:
    dazed` (exercises condition outcomes).
  - `rally` — non-attack (no attack block), range 1 (self or adjacent),
    effect: resource_change hp `1d6 + @con` (the slice's one positive
    resource change — drives the bloodied/dying upward-crossing removals;
    NOT a surge system, just a morale recovery).

  Seven abilities total.
- **conditions/**: `bloodied`, `dying`, `dazed` — tracked markers with 4.5e
  descriptions; enforcement is the DM's narration (v1).
- **goldens/**: at least one per ability (conformance-enforced), PLUS
  dedicated threshold goldens: bloodied crossing, dying crossing,
  bloodied removal on upward crossing (positive resource_change), flurry
  usage-exhausted rejection, out-of-range rejection, staggering-blow dazed
  application, two-target flurry (multi-target event ordering).
- **guide.md** — the DM affordances: reference statblocks (goblin cutter,
  goblin archer, human fighter PC-analog) with exact add_actor
  attribute/resource maps (incl. the max_hp duplication rule and
  flurry_uses), suggested turn flow, when to apply/remove conditions
  manually, how thresholds fire automatically, sample opening prompt.

Statblock instances live in guide.md as reference text (per 5a's binding:
statblock FORMAT is the schema's; INSTANCES are content — the DM creates
actors via add_actor following the guide).

## 5. Wire scenario

`scenarios/goblin-fight.json` — top-level `"ruleset": "dnd45e-minimal"`;
session, scene, fighter + two goblins with guide-accurate statblocks,
tokens in range, longsword hit path (invariant assertions — crypto dice),
staggering-blow dazed application (hasCondition probe), goblin counterattack,
remove_condition, denial rows (player driving goblin, spectator). Joins the
committed scenario library in `task check` forever.

## 6. Testing

Zero platform changes — the diff touches only `rulesets/dnd45e-minimal/`,
`scenarios/goblin-fight.json`, and this spec/plan. Conformance suite runs
the new ruleset UNTOUCHED via the existing `rulesets/` glob (load,
per-ability smoke, per-ability goldens — the P4 promise kept). Goldens are
hand-derived against the conformance fixed-seed roller and verified by the
suite. ADR-009 applies in data form: goldens are written from the FORMAT
contract first, then verified against the runtime — any mismatch is
investigated before adjusting (a golden adjusted to match observed output
without derivation is the data equivalent of impl-then-test).

## 7. Demo rematch (the merge gate)

Runbook (guide.md appendix + presented at the gate): `vtt serve --campaign
<fresh> --ruleset rulesets/dnd45e-minimal`; `vtt invite --role agent`;
existing `.mcp.json` from the P7 demo (add `--ruleset` to the mcp args for
get_ruleset_guide); suggested opening prompt telling Claude to read the
guide, set the scene, add actors from the reference statblocks, and run the
fight with use_ability. Patrik watches live; his acceptance merges 5b.

## 8. Non-goals (YAGNI)

Turn/initiative engine; opportunity actions; healing surges (raw positive
resource_change is allowed and exercised by a golden; the surge SYSTEM is
not modeled); equipment/enhancement layer; marks/aura enforcement; areas;
int/cha attributes; more monsters (the bestiary grows with the adventure
format); any platform or format change whatsoever.
