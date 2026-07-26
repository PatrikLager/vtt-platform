# Goblin Ambush

A single-scene skirmish for one human fighter against two goblins in a
narrow ravine — the same fight `scenarios/goblin-fight.json` proves at the
platform level, prepared here as real table content: place three tokens,
read the opening, and run it.

**Load order:** fetch this guide with `get_adventure_guide` before the
table sits down, but ALSO fetch `get_ruleset_guide` for `dnd45e-minimal` —
this adventure assumes you already know that ruleset's attack-roll shape,
its `bloodied`/`dying` thresholds, and its wire quick-reference for
`use_ability`/`move_token`. This guide only adds what's specific to THIS
ambush; the ruleset guide is where the mechanics live.

## Setup

Load `goblin-ambush` (`load_adventure`). It places `The Ravine Trail`
(32x32), one Human Fighter (`act-fighter`, `tok-fighter` at `(0, 0)`), and
two goblins already positioned to spring the ambush: Goblin Cutter
(`act-cutter`, `tok-cutter` at `(1, 0)` — melee range of the fighter from
the first round) and Goblin Archer (`act-archer`, `tok-archer` at `(5, 0)`
— within shortbow range but out of the fighter's melee reach). All three
statblocks are the dnd45e-minimal ruleset guide's own reference blocks,
copied verbatim — nothing about them differs from a hand-built encounter,
which is the point: an adventure is just prepared setup events, not a new
kind of thing. One world note, `ravine-trail-warning`, is REVEALED from the
moment the table loads — the party can already see it without you doing
anything.

## Suggested beats

1. Read the opening narration aloud (it's already in the log as the
   batch's last event — `NarrationAdded`). Let the party react to the
   ravine before anyone rolls anything.
2. The goblins act first — they set this ambush, they spring it: cutter
   charges into melee with `goblin-scimitar`, archer opens at range with
   `goblin-shortbow`.
3. The fighter answers however the player chooses — `longsword-strike` or
   `staggering-blow` on the cutter (adjacent), `crossbow-shot` on the
   archer, or `hunters-flurry` once the goblins are close enough together
   to both be in range.
4. Keep narrating `bloodied`/`dying` as they fire automatically off `hp` —
   you never apply or remove either one yourself.
5. Watch the archer's `hp`. See "The secret" below the instant it crosses
   half.

## When to reveal the note

`ravine-trail-warning` is already revealed at load — there is nothing to
hide about IT. Use it as the hook for the ambush itself: a player who asks
to examine the trail before advancing, or who rolls well on a perception-
style check (this ruleset has no formal skill system — adjudicate it
narratively), notices the markings and can reasonably conclude something
is watching from above BEFORE the goblins act. If nobody looks, the
ambush simply opens with the goblins' first attacks and the party
discovers the marking's meaning the hard way.

## The secret

The Goblin Archer is a coward under its bluster: the instant its `hp`
drops to or below half (the same moment `bloodied` fires on it — you'll
see the `ConditionApplied` event), it breaks off and runs for the cliffs
rather than keep fighting. Narrate this the turn it happens — a
half-panicked retreat, `move_token` walking `tok-archer` back toward the
ravine wall and out of easy pursuit — not a fight to the death. If the
party corners it before it can flee, or catches it with a ranged attack as
it runs, that's a fair complication your narration is free to lean into;
the archer's cowardice is a personality trait for you to play, not a rule
the engine enforces (see the reminder below) — nothing stops you from
calling `use_ability` for it again if a player's action would obviously
provoke that.

## Reminder: conditions are narration

Exactly like the ruleset guide says: the engine tracks `bloodied` and
`dying` and fires them automatically off `hp`, but it never reads a
condition anywhere inside `use_ability`'s resolution. A `dying` goblin can
still be targeted normally; a `dazed` fighter (from `staggering-blow`, if a
goblin ever lands one) isn't mechanically stopped from acting twice. The
archer fleeing at bloodied is the SAME kind of fact — a piece of fiction
this guide hands you, not something `use_ability` will ever refuse to let
you contradict. Playing it straight is entirely on you as narrator.
