# ADR-012: The rules contract

Status: accepted (Patrik, 2026-11-04).

## Context

ADR-002 put game logic in ruleset data that one engine interprets, and ADR-004
keeps every game-system concept out of the engine. Neither says what a ruleset
may express. The rulesets built so far could say one thing: a roll against a
threshold, once per ability (`docs/superpowers/specs/2026-07-25-ruleset-interpreter-design.md`,
format v1, and `docs/superpowers/specs/2026-07-25-format-v2-composition-design.md`,
format v2). Format v2's own §10 named what was left out: modifiers from one
thing to another's roll, secondary attacks, degrees of success, conditions that
change a roll, a comparison whose direction was fixed, reactions and
interrupts, a turn engine, areas and healing.

The blueprint's section on the rules engine says what the engine must
guarantee, and principle 21 ties every measure on the board to the ruleset. No
specification said how any of them is met.

So before anything was built, these questions had to be answered:

- what must a ruleset be able to say, as data, for the platform to run real
  tabletop systems; and
- where does the engine stop and the DM begin?

This ADR answers both. Its decision is the **rules contract**: the closed list
of forms a ruleset may use, the line between engine and DM, how a resolution
runs and is written to the log, and the order the engine is built in.

## How the decision was reached

Nothing was built. The contract was written as a draft and tested on paper in
rounds, against systems chosen to disagree with one another.

**The systems.** These were broken down first:

- D&D 5e, from the System Reference Document 5.1 (CC BY 4.0);
- OSRIC, an open clone of AD&D first edition;
- Middle-earth Role Playing, second edition;
- Shadowrun, fifth edition, from its free Quick-Start Rules;
- Dungeon World, from its authors' CC BY 3.0 source;
- Old School RuneScape, from its wiki.

Between them they cover a d20 against a target number, open-ended percentile
rolls read on tables, dice pools counting hits, two dice read in bands with no
turn order at all, and a real-time tick clock. More were added later, to
test the draft on systems it had not been shaped by, because a draft that is
tested only on the systems it was shaped by shows only that its author fixed
what was found. Their findings then shaped drafts 0b-5 to 0b-7:

- **Blades in the Dark**, from its SRD (CC BY 3.0): no turn order, progress
  clocks, position and effect, resistance paid in stress;
- **D&D 4.5e**, a variant of D&D 4e and the system played at this table:
  powers with several attacks, immediate interrupts and reactions, opportunity
  attacks, marks, forced movement.

**The situations.** Each system was broken down into the same situations, step
by step, with a source for every step: time and turn order, an attack, a spell
with an area and a resistance roll, a test and a contest, healing and rest, a
condition with a duration, and who rolls which dice. Blades and 4.5e added
their own: teamwork, clocks, downtime, interrupts and Ready.

**Cold encoding.** An agent that had read only the contract draft and one
system's breakdown wrote that system's situations as ruleset data. Where the
draft fell short it marked the gap:

- **MISSING**: the form does not exist;
- **UNCLEAR**: the draft can be read two ways, and the two play differently;
- **STRETCH**: a form was used for something its text does not say;
- **DM**: the step can only go to the DM.

Whoever wrote a draft never encoded against it.

**The rounds.** Each round after the first tested the draft the previous round
had led to. The draft names follow the investigation's steps: 0b-2 was the
owner's decisions on round 1's findings, not a draft.

| Round | Draft | Encoded |
|---|---|---|
| 1 | 0b-1 | the first systems, a few situations each |
| 2 | 0b-3 | the first systems, the situations round 1 could not encode |
| 3 | 0b-4 | Blades in the Dark |
| 4 | 0b-5 | D&D 4.5e |
| 5 | 0b-6 | D&D 4.5e, its hardest situations |

Round 1's commonest finding was a missing form. Round 2 encoded mostly the
situations round 1 could not, so the two do not compare like for like; what
changed was the kind of finding. Unclear readings had overtaken missing forms,
and what remained was mostly unwritten meaning: what a roll yields, when an
optional bonus is chosen, the order inside one resolution.

Round 3's largest gap was a single hole: the draft assumed a creature acting,
and Blades keeps much of its state in things no creature owns. The next largest
was teamwork. Round 4 found more missing forms than Blades did, the largest of
them in the core of tactical combat: reactions that land inside someone else's
attack or move, and forced movement. The owner had warned of exactly this when
the Blades results came in: the contract must not be built for Blades, because
Blades is unusual, so a D&D or MERP must be tested too. To hold to that, every
form Blades led to was given a use in another system, with one exception: keys
drawn from the campaign's entities, a crew's standing with each faction, which
no other system tested needed. The forms added for Blades did not get in 4.5e's
way; a skill challenge became a thing with a DM action and a scale, and
opportunity attacks became the ruleset's own reactions. Round 5 tested only
where the 4.5e fixes landed. In the same situations, the round-4 draft's
commonest finding had been a missing form; against the round-5 draft it was an
unclear reading, and every case it was asked about could be written, except that
Ready could be written only in part and an interrupt that turns a hit into a
miss only under one reading. What it found was the next layer down: a reaction
is a choice its owner makes and pays for, a roll must know which target it is
against, and a duration belongs to the effect that applies a condition.

Draft 0b-7 is the draft after round 5, and this ADR records it.

**Where the evidence is.** The breakdowns and the encodings are not in this
repository. They quote rulebooks under copyright, and the repository is
public. The findings described above and named in the sections below are the
record.

**RPTool, per CLAUDE.md rule 9.** Its documentation was read before the first
draft. Its rule framework, built for 4e, was read after the first round, at the
owner's pointer. These of its ideas were taken:

- a relation between attacker and target is computed when the roll is made and
  never stored;
- conditions applied together are saved against together, which became the end
  group (Decision §11);
- a condition remembers its own chain of saving throws, not the power that
  applied it;
- its own warning, that a condition nobody's code reads is stored, shown and
  does nothing, became a load rule (Decision §10).

One idea was taken and then withdrawn: a fixed order in which the start and end
of a turn resolve. The owner ruled that the order is the ruleset's (Alternatives
weighed, 3). RPTool tells friend from foe by asking whether a token is a player
character, which cannot express an allied NPC; this contract does not take
that either (Decision §15).

## Alternatives weighed

Each was argued during the investigation and decided. The owner's rulings
carry their dates.

1. **A scripting language inside rulesets.** Rejected. ADR-002 already turned
   away from imperative macros, and principle 24 says the engine runs only the
   forms its contract defines. Every gap the encodings found was closed with a
   new form or a written meaning, never with an escape into code.

2. **The DM drives secondary effects.** Rejected (2026-11-04). Ongoing damage,
   a saving throw at the end of a turn, a condition that ends when its source
   does: the engine runs them. Where a step goes to the DM is decided case by
   case and is held to the kinds in Decision §17.

3. **The engine owns the order inside a turn**, as RPTool does: durations end,
   then periodic effects, then reactions. Proposed, then withdrawn on the
   owner's objection (2026-11-04): when an effect happens, at the start of a
   round, at its end or after it, is the ruleset's to say, and the engine only
   applies it. The ruleset declares its own points (Decision §13).

4. **The DM runs turn order by hand.** Rejected (2026-11-04): of the systems
   broken down first, only Dungeon World has no turn order, so the engine runs
   it, as a procedure the ruleset chooses.

5. **Extend format v2.** Rejected (2026-11-04): format v2 is replaced outright,
   and the rulesets in the repository are the project's own systems. They borrow
   shapes from published games, never their content.

6. **One write to the log per resolution, with reactions after it.** This was
   the draft until round 4. It is right for a fireball and wrong for 4.5e,
   where a reaction comes after the first of two claws and an opportunity attack
   when a square is left. Replaced by points inside a resolution where others
   may act (the owner's ruling, 2026-11-04; Decision §13).

7. **Reactions run by themselves.** This was the draft until round 5. It would
   take every opportunity attack automatically. Replaced by a reaction that is
   offered to its owner, who may answer in advance (the owner's ruling,
   2026-11-04; Decision §12). The owner gave the reason: a player may want to
   keep their one opportunity attack for an enemy they expect to move away.

8. **The DM assigns sides when a fight starts.** Proposed for OSRIC's side
   initiative and rejected: who is friend and who is foe is the campaign's
   information, not a rule (the owner, 2026-11-04). The rules engine reads it
   (Decision §15).

9. **Neutral parties.** Not now. A participant in another group is an enemy,
   and there is no third stance, to begin with (the owner, 2026-11-04: "är du
   inte med oss, är du mot oss").

10. **A ceiling on dice that explode or are open-ended.** Rejected under the
    owner's ruling that what is almost certainly theoretical gets no code
    (2026-11-04): a chain without end is theoretical.

11. **A third round on the first systems.** Rejected in favour of systems the
    draft was never shaped by, because another round on the same systems would
    show only that the findings had been fixed.

12. **Initiative set by the platform.** Rejected (2026-11-04): each participant
    rolls their own, players for their characters and the DM for the creatures,
    and the DM's rolls may be left to the platform. Initiative is part of the
    fight.

## Decision

### The owner's rulings

The contract rests on these. Each was asked and answered during the
investigation.

On 2026-11-02:

- a sheet refers to the ruleset's catalogue and never copies from it;
- each ruleset has its own clock, and the engine only counts;
- dice are rolled by the platform or by hand;
- a critical result's text is the ruleset's, and the DM chooses what to do
  with it;
- the forms are closed;
- equipment is the ruleset's content;
- the project needs a blueprint (written, and merged on 2026-11-04).

On 2026-11-04:

- the engine runs turn order, as a procedure the ruleset chooses;
- the engine drives secondary effects, and steps that go to the DM are decided
  case by case;
- a ruleset may say that a roll is always made by the platform;
- who rolls is the ruleset's default, and the table may change it;
- the rulesets in the repository are the project's own systems, and format v2
  is replaced outright;
- a simultaneous group reads the state at the group's start and is written as
  one batch;
- a resolution that is waiting lives in the log;
- the order of things inside an occasion is the ruleset's;
- the DM may pick a difficulty from a scale the ruleset declares, and a
  mechanical consequence from a menu the ruleset declares;
- a procedure may repeat, within a ceiling the ruleset states;
- initiative is never set for a participant: each rolls their own;
- friend and foe is the campaign's information, with no neutral parties to
  begin with;
- what is almost certainly theoretical gets no code;
- the campaign may hold things that are not on the board, the DM may act
  without a sheet, and a question only the story can answer is declared as the
  DM's by the ruleset;
- the contract must not be built for Blades alone, because Blades is unusual;
  a D&D or MERP must be tested too;
- the DM steps take the kinds a–f as proposed;
- a turn's action economy is the ruleset's budget, and the engine only counts
  within it;
- the ruleset may mark points inside a resolution where others may act, and
  what happened up to the point is written to the log there;
- the rules engine may ask the board engine to move a piece;
- a player is asked whether to take a reaction, and may answer in advance,
  because a player may want to keep an opportunity attack for another enemy.

### The contract

The rest of this section is the contract: what parts a ruleset has, what each
may express, and what none may. Names in `code` are the draft's working names.
The specification written for each part as it is built fixes the final names
and fields.

Every list below is **closed**. The engine runs these forms and no others
(principle 24), and a new form is added by a recorded decision, as an addition
that changes nothing already accepted (principle 5).

#### §1 Identity and options

A ruleset has an id, a version and a name, and declares the options a table may
switch on, such as facing or written declarations. An expression reads an
option as `option(name)`. Every outcome in the log names the ruleset, its
version and the options in force (principle 28).

#### §2 Properties

The ruleset names every property and gives it a type, a display name, an
explanation and a visibility (principles 13 and 35). The types are:

- `number`, an integer;
- `category`, a value from a finite list; an `ordered` category can be
  compared, put in bands and used as a table key;
- `tags`, a set from a declared list;
- `resource`, see below;
- `set`, references to catalogue entries, see below;
- `dice`, a dice form, optionally chosen by a category ("1d8 against Small and
  Medium");
- `ref`, a reference to a catalogue entry, an entity or a thing;
- `text`, stored and shown, never read by an expression or a condition;
- `derived`, an expression that is also a place others contribute to (§10). A
  derived property may take parameters and may have one expression per key.

A **resource** has a direction, a minimum and an optional maximum, a buffer
emptied first (temporary hit points), an overflow into another resource (stun
spilling into physical damage; a chain of overflows is allowed and a cycle is
refused at load), keys (slots per spell level, runes per kind, one cooldown
per kind of action), keys drawn from the campaign's entities or things
(a crew's standing with each faction), a refill occasion and thresholds. On a
resource with keys, the minimum, maximum and thresholds hold per key. Without an
overflow, a change stops at the bound, and the event carries the amount that
changed and the amount that was absorbed. A threshold applies a condition in
the same write as the change that crossed it, or starts a procedure as a new
resolution after that write. A condition set by several thresholds is removed
when none of them holds.

A **set** has the form of its entries, whether an entry may appear more than
once, the states an entry may be in (one at a time), a condition an entry must
meet, values per entry (an asset's quality), and a budget checked at every
addition (spell levels summing to at most the caster's level plus one).

Visibility is `seen`, `owner` or `dm`, per property or per form. The projection
(SPEC-016) applies on top.

#### §3 Forms, catalogue and where actions come from

A **form** says what a sheet, an item, a condition or a thing looks like: its
properties, references, slots and sets. A slot may have a capacity, such as two
hands, and an item says how much of it it takes. A form may override a
property's expression or thresholds. The names of forms are the ruleset's; the
engine knows only the concept (principle 23).

A **catalogue entry** has a form and may extend another. A sheet refers to
entries and never copies them (principle 27); what the sheet holds about a
reference (its state, charges, copies) is campaign data. An entry's own derived
properties are computed for whoever holds it, and override the form's property
of the same name.

**An entry with a procedure is an action.** A sheet is offered the actions it
reaches through its references, its sets, its slots and the conditions it holds
or is the source of: the wizard's spell from the prepared set, an attack from
the weapon in hand, the escape a grab grants the grabbed. When an action is
reached through a copy in a set, that copy is the action's own `this`, so
casting one prepared copy leaves the other.

**The DM's actions.** A ruleset may declare actions the DM takes without a
sheet: starting a clock, advancing a faction, a lair action, starting a skill
challenge. The DM's input in them is the DM's own play, but they can do only
what their procedure says.

**The ruleset's own rules.** A ruleset may hold its own contributions, its own
timed procedures and its own reactions at the top level, each saying which form
it applies to: every character, every clock. No condition has to carry them,
and none can switch them off. An opportunity attack is one; so is an action
point gained at a milestone.

The ruleset declares its **action types** as a list, and every reference to one
is checked at load.

#### §4 Tables

A table has one or two keys. A key is a band of integers or a category value.
A cell is a compound result: a label, named numbers, effects each with its own
condition ("if no arm armour"), and text offered to the DM. What lies beyond the
table is clamped or given by an expression. A step looks a table up, or an
expression reads a cell.

#### §5 Expressions and conditions

**Expressions** are integer arithmetic: `+ − * /`, where `/` rounds down,
`floor`, `min`, `max`, `clamp` and `if(condition, a, b)`. They are always
deterministic; dice exist only in the roll step. Every number in a form, a
contribution or a dice form is an expression. Everything is computed when it is
read, except a condition instance's values, which are computed once, when it is
applied; whatever must be frozen goes there.

An expression refers to sheets (the actor, the target, the holder, the source,
this), to the step collecting contributions and its tags, to a resolution's
earlier results and declarations, to the event a reaction answers, to
parameters, to turn facts that are numbers, and to the board, the campaign, a
table or an option.

**Aggregates** (`sum`, `max`, `min`, `count`) run over a set, a slot, the
active conditions, the campaign's things, the entities in the scene, a tag set,
a list a loop collected, and the instances one entity is the source of. Each
may filter its entries. An aggregate over nothing is 0.

**Conditions** are a closed list: comparison, has a tag, has a condition, is in
a set, wears or wields what a filter describes, board facts, campaign facts,
turn facts, an occasion in progress, an option, an earlier label, an optional
contribution chosen in this roll, is a target of this resolution, and a
relation between attacker and target. They combine with all, any and not.

#### §6 Procedures

A procedure has parameters, steps and an optional result. **The steps are a
closed list**: roll, input, let, compare, opposed, bands, lookup, outcome,
effect, branch, each, call, emit, delayed, commit and area. Every step may have
a condition and is skipped when it is false.

`each` runs over targets, participants, sides, members of a side, a set or slot,
a list, active conditions, things, the entities in the scene, or a count. **A
count has a ceiling written as a literal number, checked at load** (the owner,
2026-11-04: a power may attack several targets, or one target several times),
and may stop early when a condition holds. A loop may collect one value per lap
into a list, and may write to the log after each lap. `call` runs another
procedure, or an action's targeting, conditions and procedure inside the current
resolution. `area` works out an area's members without asking anyone.

**Bindings** are assigned once on each path and live only in their resolution.
A name read after a branch must be set in every branch, and that is checked at
load.

**A step sees the state as it is when the step runs**: what is in the log, plus
what the resolution has done and not yet written. The second attack of a power
sees the damage of the first, and an attack sees what an interrupt did. In a
simultaneous group the starting point is the group's start (§13).

**Repetition is always bounded inside a procedure**: a loop by its domain or its
ceiling, the call graph by being acyclic, a held action that repeats by becoming
a new resolution each time. Dice that explode or are open-ended repeat for as
long as the dice say so.

**An action runs in this order**: parameters (Force, when the radius is Force in
metres), the cost's inputs, targets, conditions, the DM's confirmation where the
action has one, the cost, the event "an action begins", and the procedure. A
cost that cannot be paid refuses the action. A cost may be a procedure. An
action may contribute to the places read in its own resolution, so an attack
that gives the defender −2 shows that −2 with the attack as its source.

#### §7 Dice

**The dice forms are a closed list**: `NdM` (count and sides as expressions, or
a dice property taken a number of times, as in "2[W]"), keep the highest or
lowest k of n, open-ended (adding on a high roll, subtracting on a low one, with
a condition that stops the chain), a pool counting hits, a uniform range, and a
k-in-N chance. Every die declares its faces; a die read as two tens is two dice.

A roll may name its place (places are hierarchical, so a penalty to `test`
reaches `test.attack.ranged`, and a place may carry a parameter over a finite
set), whose contributions it collects (`for`), who touches the dice (`by`),
which target it is against (one, or a list for a shared damage roll whose other
contributions are counted per target), a multiplier on its dice and dice
contributions but not on flat ones (a critical hit in 5e), a condition that
maximizes its dice (a critical hit in 4e), and its own cap.

A roll yields each die, the deciding die (the kept one under keep), the total
with every contribution, the dice alone, the count of dice before they exploded,
counts of faces, hits capped and uncapped, and the same fields as they were
before any reroll or added die (a glitch in Shadowrun is read there). A roll of
zero dice asks for nothing and yields a total of 0.

#### §8 Inputs

**The inputs are a closed list**: one choice from a menu, several choices
within a budget, a number in a range, a target (anywhere in a procedure, also
from the action's own targets, and without a range from the campaign's entities
and things), a ruling, an order, and free text that is only shown. A menu's
options may come from a set, a resource's keys, active conditions, or a party's
actions. An option whose condition is false is not shown, and an input with no
options asks nothing.

A **ruling** comes only from the DM and only where §17 allows. It may have a
default and may decline to wait: then the default is taken when the DM has not
answered, and the testimony says so.

#### §9 Effects

**The effects are a closed list**:

- change a resource;
- apply a condition, with values, source, end group and name, and optionally a
  duration of its own (§11);
- remove a condition;
- change a value in a condition instance;
- convert an instance into another condition;
- change a set;
- change an item: its charges, destroy or remove it, equip or unequip it
  (putting on a shield is an action with a cost and this effect);
- create or remove a thing (§15);
- change the turn's budget;
- **move a piece**: a mode (the ruleset's word: push, pull, slide, shift,
  teleport), a distance as a ceiling or exact, a direction and who chooses the
  path. The rules engine may ask for it (the owner, 2026-11-04); the board
  engine checks the path like any move, every square is a point where others
  may act (§13), and the mover's own blocks apply: restrained stops a push;
- move a participant's place in the turn order (Ready, Delay).

#### §10 Contributions

A contribution applies to a **place**: a step's slot, a derived property or one
of its keys, an entry point (an action is declared or taken, a move is made, a
turn advances) or a relation between an attacker and a target. Places are named
by the ruleset.

The engine collects contributions from everything active: conditions, items in
slots, entries in the sheet's sets, catalogue properties, the action being
taken, board facts, the ruleset's own contributions, and the DM where the
ruleset declares it (§17 c). A contribution's value is computed when its place
is read. Only a contribution that relabels or replaces an outcome, or is chosen
after the roll, may read the roll itself.

Every contribution says whom it applies to (its holder rolling, someone rolling
against its holder, the source of its condition rolling, or always), its kind,
its condition, its order, its stacking, its group (which may be an expression,
so "the highest bonus of each type" is written once), whether it is optional,
whether the first matching use consumes it, and its source, which the testimony
shows (principle 29).

**The kinds are a closed list**: add, multiply, extra dice (with who rolls them,
whether they are rolled once per source and shared, as a protector's
counterspelling dice are, and on a pool whether they join it or count apart),
advantage and disadvantage, an automatic outcome, a cap (or no cap at all),
shifting a table key, a reroll (with a closed list of selectors and a limit),
replacing an outcome, relabelling, exploding, dice added after the roll, and
deny, which cancels another contribution.

**The stacking rules are a closed list**: sum, highest, once per roll, binary,
and once per source.

**An optional contribution** says who chooses (the roller, someone else, as when
another creature spends its reaction on the roll, or the DM, which is the only
form a DM's modifier takes), when (before the roll, after it, or when all the
resolution's rolls are made, choosing which roll gets it), whether per roll or
per die, per target on a shared roll, its cost (paid by whoever chooses), a
bundle of contributions bought as one choice, when an offer before the roll
closes, and an **offer** made by one party, accepted by another, at a price the
offerer sets each time (a Devil's Bargain in Blades, a success at a cost in
5e). A group may be exclusive, so choosing one optional contribution hides the
others.

**The load rule.** A contribution to a place that no step, property or entry
point reads is refused when the ruleset loads. A condition that does nothing
must never do it silently.

A **relation** between an attacker and a target is computed for the pair when
the roll is made and never stored. It takes contributions from the target, from
the attacker and from board facts such as flanking, and either side may deny
the other's. It is read as a number, or as a condition when it is binary.

#### §11 Conditions

A condition is a catalogue entry of a form the ruleset makes a condition. **An
instance** has typed values set when it is applied, its source (the entity, and
the instance that applied it), an end group (instances that end on one shared
roll) and a name later steps can use.

**A definition** has:

- contributions;
- blocks on action types, tags, budget units and modes of movement, each with
  its own duration if it needs one;
- immunities: conditions that cannot be applied to the holder;
- gates: a procedure that runs before an action type and may stop it;
- timed procedures, at an occasion and point, every N times, with a first time
  and a count from application;
- reactions (§12);
- end clauses (below);
- a stacking rule;
- what it converts to;
- a pause while a condition holds, and what happens when the pause ends;
- whether it ends with its source;
- the conditions it implies, and whether they persist after it;
- an action it grants to its holder or its source (sustain, escape), one per
  instance;
- a controller who makes the holder's choices while it lasts (dominated);
- the group its stacking works within.

**Stacking is a closed list**: ignore a new application, restart, separate
instances, separate and summed, sum into one, replace within the group, keep the
highest within a key (ongoing damage of one type), and keep the longest (with
identical save-ends counted once).

**A duration may be given when a condition is applied.** The same condition can
be "save ends" from one power and "until the end of your next turn" from
another, and stays one condition for immunity, stacking and the question "does
the holder have it". The ruleset may name its durations.

**End clauses are a closed list**: after N occurrences of an occasion and point,
counted for the holder, the source, anyone, or an entity named in the instance;
a value reaching zero; the source ending; the resolution that applied it ending,
or the one that triggered a reaction ending; a roll at an occasion and point;
being consumed; a condition becoming true, tested after each write; the holder
or source choosing, with a time and a cost; a named occasion. Each clause has
its own consequence.

**The ending roll** is made once per end group, and each of its labels maps to
an outcome. The engine counts the outcomes per instance, so "the first failed
saving throw" is a label of its own. The chain lives in the instance, not in
the action that applied it.

#### §12 Reactions

A reaction listens for **a trigger from a closed list**: a resource changes, a
threshold is crossed, a condition is applied or removed, an occasion and point
arrives, a step with a tag runs or yields a label, an action is declared or
begins, an entity enters or leaves a square within a distance (filtered by mode
of movement).

**Every event carries** who caused it, the action, its targets, the target the
roll was against, the step's tags and the resolution's labels, and its own
fields: the amount, the condition, the mover and the mode. A reaction filters on
them with its condition.

**Timing.** A reaction runs **before** what triggered it takes effect, and may
change or stop it, or **after** the write that triggered it, as a new
resolution. An after-reaction runs only if its instance still exists after that
write: whoever falls unconscious from the damage that would break their
concentration does not roll to keep it. A reaction runs once per event or once
per resolution, with the amounts summed.

**A reaction is a procedure the engine runs, or an action offered to its owner**
(the owner, 2026-11-04). An immediate action or an opportunity attack is an
offered action: it costs its budget unit, its blocks and conditions apply, and
its target may come from the event. The engine offers it only to whoever can
take it: the trigger matches, the unit is left, nothing blocks it. The question
waits for an answer, as a request for dice does. **The player may answer in
advance**, per reaction: always, never or ask, also for chosen targets only, so
an opportunity attack can be kept for the enemy expected to move away. The
advance answer and every refusal are in the log.

Several reactions to one event run in the order §13's determinism gives, unless
the ruleset orders them.

#### §13 Time, turn order, declaration and waiting

**Occasions.** The ruleset declares its occasions, each with an ordered list of
points: a turn with its start and end points, a round, a short rest, the end of
an encounter. Everything bound to time names an occasion and a point and may
have an order within it. An occasion is raised by the turn order, by a procedure
or by a participant (the DM among them, §17 a), and may name a subject, so
"until you next prepare spells" ends only for the one who does. An occasion may
last until another arrives, which is a phase: "during the encounter", "during
downtime". **The engine owns no order.** It owns only these rules: two things on
one point with no order from the ruleset run in the order they were applied,
which the log shows (principles 3 and 11); and a reference to an undeclared
occasion or point is refused at load. A point, or a run of points, may let the
holder choose the order of what lies on it, when the ruleset says so: in 4e a
creature chooses the order of its saving throws, ongoing damage and ending
effects.

**Turn order.** The engine runs turn order in a form the ruleset chooses. **The
forms are a closed list:**

| Form | What it is | Drawn from |
|---|---|---|
| sorted once | a key per participant, sorted once; identical creatures may share a roll; ties broken by a procedure or resolved together; Delay | 5e, 4e |
| sorted each round | the same, with a new key each round | – |
| passes | a key counted down in passes while it lasts | Shadowrun |
| declare, then phases | everyone declares, then phases resolve by type and key, equal keys together | MERP |
| sides crossed | a die per side, the other side's die gives one's segment, with adjustments and precedence | OSRIC |
| cooldown | one wait per kind of action per actor, counted down each tick, with an order within the tick | OSRS |
| none | no turns at all | Dungeon World, Blades |

**Initiative is rolled by the participants.** A key's roll is asked of whoever
controls the participant: a player for a character, the DM for a creature. The
table may let the platform roll for the DM's creatures. Each form defines what
its round and its turn mean, and rounds count from one; a ruleset may declare a
round zero that exists only when its condition holds, with only some
participants taking a turn in it (a surprise round).

**Simultaneous groups.** Where a form resolves things together, every
resolution on the same point is in the group, except one that precedence lifts
out. Each reads the state at the group's start plus its own effects, and the
group is written as one batch (the owner, 2026-11-04): damage from both sides
lands even if one dies. A resolution that begins waiting in a group waits from
the group's start.

**The budget.** A turn's action economy is the ruleset's budget, and the engine
only counts within it (the owner, 2026-11-04): it takes a unit, refuses an
action when the unit is spent, refills when the ruleset says, and writes it to
the log. The units are the ruleset's words. A unit refills at the turn's start,
at the start of the holder's own turn or at the round's start; may be used
outside one's own turn, and may be barred on it; may stand in for another (a
4e standard action used as a move or a minor); and a cap may hold a sum of units
(dazed: one action of three).

**Turn facts are a closed list**: the round, the segment, whose turn it is,
whether one has acted this round, whether a tag was used this turn, what a party
declared, rounds in a row of one type, and who is ordered before whom.

**Declaration.** An action may have a declaration: inputs, targets among them,
and effects, run when it is declared, in one or more moments, and read later by
the procedure. A turn-order form says when declarations happen and in what
order. A declaration's visibility may change over time, so a written
declaration is hidden until it resolves. After declaring, a participant may
change freely, only abstain, not change at all, or replace the action from a
menu. Abstaining costs nothing unless the ruleset says so.

**Waiting.** A resolution may wait for inputs and for occasions. **The forms
of waiting are a closed list:**

- **a delayed part**: what the resolution did before it is written, and the
  rest waits for an occasion (a hit landing ticks later, a second attack at the
  round's end), checking its blocks again when it runs;
- **a duration with interrupts**: the action resolves when its count completes,
  counted from when it comes in turn, with its cost paid when the wait begins,
  and lost if an interrupting event comes first (a spell's casting time);
- **holding**: an action held until a trigger or the holder's choice, which the
  engine offers at every point and after every write. A held action may be
  chosen when the hold is made, may draw its unit when released, and expires;
  releasing it is a new declaration. 4e's Ready and MERP's opportunity action
  are holds;
- **repeating**: a held action that runs again until a condition holds or
  another action cancels it, each run a new resolution in the log.

**Points inside a resolution** (the owner, 2026-11-04). A ruleset may mark
points inside a resolution where others may act. At such a point what the
resolution has done so far is written to the log, and the reactions to it run. A
step can mark one; a loop can mark one after each lap; the ruleset can say once
that every step with a tag marks one, so in 4e no power forgets the point after
an attack. Every square of a move, forced moves included, is such a point. A
before-reaction writes its own batch before what it interrupted goes on; that
resolution then reads the state anew, tries its blocks, targets and range again,
and, if the trigger was a step's label, runs that step again with the same dice.
An attack that an interrupt turns into a miss is a miss, and an action whose
actor an interrupt has knocked out is lost after its cost is paid.

#### §14 The board

The ruleset ties its properties to the board engine's roles: sight, movement,
reach, size, length per square, the distance metric. The board engine measures
with them (principle 21).

**Board facts are a closed list**, each taking an entity or a point: distance,
in the ruleset's unit and in the plane; distance bands from a table; sight; line
of effect; flanking; the arc one stands in relative to another's front, only
with the facing option; cover, with a filter on what blocks; movement this
round; and a square's flags, such as a ceiling.

**Areas** are a burst from a point (with whom line of sight runs from), a square
around a target or placed next to the user, a cone, a line, a volume to fill,
and a burst around a creature's whole space, each saying whether a large
creature is in it by any square or by its anchor. A condition can shrink an area
to the primary target, and the DM may pick an area's members where the rule says
so. The targets always include the primary target.

**Targets** have a count, a range, a filter, requirements such as sight, a
capacity, a spread, a choice (random, nearest, farthest, least or most of
something), and may be a point.

The rules engine may **ask the board engine to move a piece** (§9).

#### §15 Campaign facts

**Friend and foe** is the campaign's information (the owner, 2026-11-04): a
fact about the world, like who controls a piece. Every participant belongs to a
group. A player's character joins the players' group when it is made; the DM
sets the group of the DM's creatures, and an adventure may set it in advance.
A group changes when the story changes, and the change is the DM's and in the
log. There may be more than two groups. Another group is an enemy; there are no
neutral parties, to begin with. The rules engine reads group, ally and enemy
and never writes them. They are what "each enemy in the burst" and "an ally on
the far side" read, and the sides of a side-initiative turn order.

**Things off the board** (the owner, 2026-11-04). The campaign may hold things
that are not pieces: a clock with a name, a size and a count that fills; a
faction. Their forms are the ruleset's. A procedure or a DM action creates and
removes them; they change like a sheet, may carry conditions and timed
procedures, are targets without a range, and are counted and looped over. They
have no board facts. Their visibility follows their form, so a clock of danger
may be the DM's alone.

#### §16 Who answers

Every roll and input names a **party**: the actor, the target's controller, the
holder, the source, the DM, the controller, each target, a side (a controller of
one of its members, or the DM for a side with no players), anyone (with a
filter, so "a character other than the roller"), or a party an earlier step
gave. The ruleset sets the default, and the table may change it. Who touches the
dice and whose contributions count are separate, so a player can roll a
monster's damage against their own character and the monster's bonuses count.

**Dice by hand** (principle 30). A roll may be left to the platform or entered
by hand, die by die; a value the die cannot show is refused, and the testimony
says who rolled. **A request for dice** is a whole roll step, made after its
contributions are collected: it names the form, the count, the party and the
step's tags, so a player with advantage is asked for two d20 at once. Dice that
follow (an explosion, a reroll, dice added after) are a new wave from the same
party, recorded in the same roll. The requests of a loop are made together when
no lap writes an effect, and one at a time otherwise, as the loader decides:
everyone saves against a fireball at once; the second claw waits for the first.

Every binding has a **visibility**: seen, the roller and the DM, or the DM
alone. It may change at an occasion and point.

#### §17 Steps that go to the DM

Steps may go to the DM only in the kinds below. Kind f is a prohibition.

| Kind | Rule |
|---|---|
| a | The DM may raise an occasion, with a subject where it takes one. The engine computes and applies what follows. |
| b | Whether a rule applies at all is the engine's where the board engine knows the answer: sight, areas, a valid target. A question only the story can answer is declared as the DM's by the ruleset, as a ruling, so a player can see in the ruleset where the DM decides, and nothing else becomes a ruling. |
| c | Cover and flanking come from the board engine. The DM's own modifier exists only where the ruleset declares it, as an optional contribution chosen by the DM, and shows with the DM as its source. |
| d | Story is text. A DM's input exists where the choice decides whether mechanics run at all. |
| e | Ties in turn order are broken by a procedure in the ruleset. Whether declarations are open or written is a table option. |
| f | No DM input for a gap in the source text. The rulesets are the project's own, and whoever writes the ruleset decides. |
| g | The DM picks a value from a **scale** the ruleset declares, with a default: a difficulty, a DC, a position. The same scale may be read as a table column or as a modifier. The choice shows with the DM as its source. |
| h | The DM picks one or more **mechanical consequences from a menu** the ruleset declares: damage, a debility, which debility and from which creature. The engine applies them, and the DM picks nothing outside the menu. |

**What is not a DM step.** Setting the campaign's data (a creature's group, a
monster's properties when it is made) and the DM's own actions (§3) are the DM's
play, in the log, and the rules engine reads them.

#### §18 Story

Text in a cell, a branch, a menu option or a catalogue entry is offered to the
DM and never sent to a player by itself (principles 32 and 17). The mechanics
apply regardless.

#### §19 A resolution and the log

A resolution that waits lives in the log (the owner, 2026-11-04):

1. **It begins** with an entry naming the action, who takes it, its parameters
   and its targets.
2. **Inputs are recorded as they arrive**: every die with who rolled it, every
   wave of following dice, every choice and ruling, also when a ruling took its
   default. A roll cannot be taken back by abandoning the resolution.
3. **It waits** for inputs and occasions (§13).
4. **A step sees the log plus the resolution's own unwritten effects** (§6).
5. **It writes batches**: normally one with every effect when it ends, or its
   group's batch. It is split only by a point inside the resolution, a square
   of a move, a duration (its cost when the wait begins) and a delayed part.
   Every outcome names the ruleset, its version and its options (principle 28).
6. **The testimony** is every die (with its values before any reroll, and its
   waves), every contribution with its source, what was compared, and what was
   ignored or refused: an application under "ignore", a condition an immunity
   stopped, a default a ruling took (principle 29). The projection decides who
   sees what (principle 16).
7. **After-reactions** start new resolutions after a write, if their instance
   remains. A before-reaction writes its batch before its trigger goes on.
8. **The DM may abort a resolution or refuse a confirmation**, and that is in
   the log.

The fold holds open resolutions in its state. It is still one fold (CLAUDE.md
rule 4). The contract gains new events, and the addition is additive
(principle 5, ADR-007).

### How the contract reads the blueprint

- **21.** Every measure on the board comes from the ruleset; the board engine
  owns only geometry. Friend and foe are the campaign's. When the rules engine
  moves a piece it asks the board engine, which checks the path.
- **23.** The names of forms, places, units, occasions and conditions are the
  ruleset's. The engine knows only the concepts.
- **24.** The engine runs only the closed lists above. A binding is assigned
  once on each path and lives only in its resolution; whatever must last is
  written as an effect. Repetition inside a procedure is always bounded. The
  one exception is a chain of dice that explode or are open-ended, which rolls
  again for as long as the dice say so.
- **25.** Loading refuses, among others: a cycle of calls; a name not set in
  every branch; a contribution to a place nothing reads; an undeclared occasion,
  point or action type; turn facts under the form without turns; a loop over a
  count without a ceiling; a declaration read before its moment; a cycle of
  overflows; facing read outside its option; a place with a parameter over an
  open set; a board fact about a thing; a contribution reading the roll that may
  not.
- **26.** Campaign data fits the ruleset it is played under.
- **27.** A sheet refers to the catalogue, and its state per reference is the
  campaign's. Its actions come through its references.
- **28.** Every outcome names the ruleset, its version and its options.
- **29.** The testimony of §19.
- **30.** A die entered by hand is checked against its faces and recorded with
  who rolled it, wave by wave.
- **31.** Turn order's forms and an occasion's points are forms; their names and
  order are the ruleset's.
- **32.** Text is offered, and steps go to the DM only as §17 allows.

## Consequences

**What this replaces.** Format v2, which had already superseded format v1, is
replaced outright, with the tickets that defined both (`docs/superpowers/specs/2026-07-25-ruleset-interpreter-design.md`,
`docs/superpowers/specs/2026-07-25-format-v2-composition-design.md`). Every
ruleset written under format v2 is rewritten under the contract as the
project's own system.

**What this overturns.** Format v2's §10 says that equipment swaps and
structured loadouts are not a platform concept; a weapon's numbers lay among
the actor's attributes. The equipment and its numbers stay the ruleset's
content (the owner, 2026-11-02). What overturns §10 is the mechanism the
owner's first ruling requires: a sheet that refers to catalogue entries, slots
that hold them, and equipping as an effect (§3, §9). The engine gains the
mechanism without knowing what a longsword is. The same section's residue
(contributions to another's roll, secondary attacks, degrees of success,
conditions that change a roll, the fixed comparison direction, reactions, a
turn engine, areas, healing) is now in the contract.

**What changes in the engine.** When this was decided, the engine resolved a
command in one call (`rules.Resolve`), with the dice rolled inside it. Under
the contract a resolution is a dialogue:
it asks for dice and choices, waits, writes at its points, and lives in the log
while it waits. The fold holds open resolutions, and stays the only code that
changes game state (CLAUDE.md rule 4). The wire contract gains the events and
commands this needs (requests for dice, offers, rulings, the writes of a
resolution), additively (ADR-007).

**What the board engine and the campaign gain.** The board engine answers the
facts of §14 and moves a piece on request. The campaign holds groups and things
off the board. The DM gains actions that need no sheet.

**What stays.** ADR-002 and ADR-004 stand: game logic is data, and no game
concept enters the engine. ADR-004's toy-module proof stays: every ruleset
passes the same conformance suite.

**What the closed list costs.** A system that needs a form not listed here
cannot be written until a recorded decision adds it. That is the intent:
each round of the test found a layer the previous one missed, and real rulesets
will find more. Each addition is additive and decided on its own; none is made
silently.

## Build order

The owner accepted these steps on 2026-11-04, together with the plan for
the paper test that came before them. Step 1 is one ticket each:

**Step 1, foundations.**

1. The vocabulary, and the first specification of the rules engine.
2. Campaign data fits the ruleset it is played under (principle 26).
3. The log names the ruleset and its version (principle 28), before any
   reference depends on it.
4. A sheet offers only what it names (principle 27).
5. The catalogue and references, overturning format v2's §10 on equipment.
6. Derived properties.
7. Testimony with sources (principle 29).

**Step 2, the resolver's forms**, one form per ticket: procedures and their
steps, dice, inputs, effects, contributions, conditions and reactions.

**Step 3**: the clock and durations (principle 31), dice by hand (principle 30),
text offered to the DM (principle 32), and the couplings to the board
(principle 21).

The rounds after that order was set added parts it did not name. This ADR places
them: friend and foe and things off the board join step 1 as campaign data; the
DM's actions and the ruleset's own rules join the catalogue; offered reactions
join step 2's reactions; the budget, turn order, waiting and points inside a
resolution join step 3's clock; moving a piece joins step 3's board couplings.

## What this does not decide

- **Names and fields.** The draft's names are working names. Each part's
  specification fixes its own, when that part is built.
- **The wire messages** for requests, offers and rulings, which their tickets
  design under ADR-007.
- **How a question is asked** at the table: the screens, and whether a client
  may answer for a player who has left.
- **Time limits** on a question that waits. The engine has no clock of its own
  (principle 31), and none was decided.
- **Neutral parties**, left for later by the owner.
- **Height on the board** beyond a square's ceiling.
