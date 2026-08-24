# Visibility: one log, many views

**Sub-project 11.** The arc maps-as-geometry was built to enable, and the one
session zero stopped play over.

---

## 1. Why this, and why now

Session zero, 2026-08-12. The DM agent said it first, while reporting something
unrelated:

> *"Asme sees the goblin positions in the shared log the moment they look, since
> the ambush setup at seq 12–13 is already written there. The ambush is a
> surprise to the character, not to the player."*

Within ninety seconds, seq 20 put `tok-fighter` on **(19,8) — the Goblin
Archer's exact square**, and seq 21 moved it back. The DM found it by polling
the log: *"Landing precisely on the archer's hidden square isn't a plausible
accident on a 32×32 grid."*

The agent named the subtle channel and missed the obvious one. Patrik, playing
Asme on an iPad, when asked directly: *"yes, I could see the goblin token on the
board — there is no fog/limited view mechanism as far as I can see at least."*

Finding 14 records the consequence, and it is this arc's scope in one sentence:

> *the log is the subtle channel and the one an engineer reaches for first, but
> the board is the one that leaks to somebody who is not even trying. Any fix
> that filters the log and leaves the grid rendering every token would close the
> harder hole and leave the obvious one open.*

**Why now, and not in August.** This arc was chosen first and deferred
deliberately: line of sight has nothing to bite on without terrain, because a
wall nobody declared is an invisible barrier. maps-as-geometry (merged
`c40a450`) supplies the input — walls, doors, objects, and `blocks_sight`
carried faithfully into `engine.State` and read by nothing at all. This arc is
what reads it.

---

## 2. Non-goals

Each is a plausible thing to start doing halfway through.

- **Light, darkness, darkvision.** Sight RANGE is an input to this arc
  (§3.4), never a thing it computes. A lantern later changes the number
  arriving; it does not change this design.
- **Stealth versus perception.** Patrik, 2026-08-12: *"in future complexity,
  even hidden creatures can be spotted if a perception roll succeeds or
  something like that."* That is a ruleset concern and lands after it. This arc
  is geometry only: **if you are in line of sight and not behind something, you
  are seen.**
- **Hiding by acquaintance.** Patrik, same conversation: *"They are not
  'hidden' just because the players haven't met them. Everything is about line
  of sight."* No "discovered" flag on a creature.
- **Elevation.** Excluded by maps §2 and unreached by the pillar model (maps
  §11.3): a tree is cover, a hill is cover AND vantage.
- **Sub-square blockers.** Deferred by ruling (§3.5); the sight test is built
  so they cost nothing later.
- **Per-square lighting or shadows as ART.** The client dims remembered
  terrain. It does not render a light gradient.
- **Hiding the DM's view from the DM.** The DM and agent see everything, as
  they do for doors ("hard for players, free for DM").

---

## 3. The model

### 3.1 Whose eyes

**A seat sees the union of what its controlled actors see.** Sight belongs to
actors; a participant inherits it from the actors they control.

**A seat with no actor is not in a scene, so it has no board yet** — scene
knowledge follows actor presence (§4.2), and a seat that is nowhere has nothing
to draw. The moment the DM grants a character standing somewhere, that scene's
board arrives: its outline, black, filling in as they look around. That is the
normal first minute at a table, since onboarding assigns no character.

Note the distinction §4.2 turns on: this is "you are not in a scene", NOT "you
are in a scene you cannot see". A player who IS in a scene always has its
board.

### 3.1.1 A spectator rides a shoulder

An earlier draft left spectators blind — no actor, so nothing at all — which is
a strange shape for a role whose name means "one who watches". Patrik,
2026-08-18: *"I would rather that you as a spectator, can jump between tokens -
like a bird hopping from one shoulder to another. You can sit on any of the
characters, and you can choose to shift to another character's view, whenever -
but you will only know as much as the party does, not what the DM has planned to
happen."*

**So `viewer` is a participant PLUS a viewpoint.** For a player the viewpoint is
fixed: the union of the actors they control. For a spectator it is whichever
shoulder they are currently sitting on. §4.1's purity is untouched — the
projection is still a pure function of `(log-so-far, viewer)`; `viewer` simply
carries one more field.

**A perch may only target a PARTY MEMBER.** This is the constraint the whole
idea rests on: a spectator perched on the Goblin Archer would watch the ambush
from inside it, and the arc would be undone in a single click. Enforced
server-side, never by which names the UI happens to offer.

*CORRECTED 2026-08-24.* This read "a player-controlled actor" until §5.1
overturned that rule on 2026-08-23. The distinction is not pedantry: keyed on
control, one `grant_actor_control` on a hidden monster made it perchable, which
is precisely the leak §5.1 closed. `MayPerch`, `eyes`, the roster exception and
the spectator's own control are all keyed on **kind** — and "player-controlled"
is the exact rule the UI had to be told *not* to implement.

**The bird remembers every shoulder it has sat on.** Explored terrain (§3.2)
accumulates for the spectator across perches, so hopping Armak → Asme leaves
them holding both rooms. Over an evening that converges on exactly what Patrik
described: as much as the party collectively knows, and never what the DM has
planned, because there is no shoulder on the DM's side of the screen to sit on.

**Perching is NOT logged.** Patrik: *"we do not need to log anything about
what/where the spectator sees."* Correct, and the reason generalises: the log is
the campaign's history, and where a spectator points their camera is not a fact
about the campaign. It is a view preference, like zoom or which panel is open.
Logging it would replay forever, add story-panel noise, and — absurdly — make it
retractable, so a DM could "undo" somebody having looked at Asme. It is a
connection setting, like the catch-up point. The only cost is that a perch does
not survive a reconnect, and the client re-sends it on connect.

**An unassigned PLAYER does not perch.** They see nothing until the DM grants a
character (§3.1). Perching is the spectator's affordance; a player's answer to
an empty board is to be given a character, which is the onboarding flow working
as intended.

**DM and agent see everything.** Their projection is the identity function, so
their stream is byte-for-byte what it is today.

### 3.2 Terrain is remembered; creatures are not

**Terrain you have seen stays drawn, dimmed, forever.** Walk out of a room and
it remains on your map.

**Creatures are pure line of sight.** They appear when visible and vanish when
not. There is no memory of "a goblin was here", because that is exactly the
knowledge §2 refuses to model.

Memory is **per participant**, not shared by the party: a scout who enters a
room alone has mapped it, and the rest of the party has not.

**This applies to your own party, and that is deliberate.** A rogue two rooms
away is a creature, so their TOKEN is not drawn for you — *"everything is about
line of sight."* You still know the rogue EXISTS, because player-controlled
actors are always in your roster (§5); you simply cannot see where they are.
Many virtual tabletops show allies unconditionally, so this will surprise
people, and it is the ruling rather than an oversight. If it plays badly at the
table, the fix is a named exception for player actors, not a change to the
model.

### 3.3 What blocks sight

- **Wall tiles and closed door tiles.** They fill their square.
- **Objects carrying `blocks_sight`.** They block over their footprint.
- **An open door blocks nothing** — the same folded state movement already
  reads, so opening a door reveals a room in one event.

Rotation is **ignored**, exactly as `covers()` in `internal/engine/terrain.go`
ignores it for movement. Its reasoning holds here: no spec defines how rotation
reshapes a footprint, and inventing that transform for sight alone would make
sight and movement disagree about the same object.

### 3.3.1 Nine points, a tolerance, and NO symmetry

**Sight is measured from the viewer's CENTRE to NINE points on the target** —
its four corners, its four edge midpoints, and its centre. A **tolerance**
says how many of the nine must be reachable before the target counts as seen.

This is MapTool's design, adopted deliberately after reading it (Patrik,
2026-08-19: *"keep the asymmetry and use the nine points"*). Tolerance is an
INPUT in the same sense sight range is (§3.4): tolerance 1 means a sliver of
exposure reveals you, tolerance 9 means you must be fully in the open, and
which it should be is a rules question rather than a platform one.

**SIGHT IS THEREFORE NOT SYMMETRIC, and this spec previously claimed it was.**
An earlier draft made symmetry a keystone property — *"if A sees B, B sees A,
same ray, same blockers"* — which is false under centre-to-many sampling, and
the test written for it could not fail because its fixture was an open
corridor. A Task 1 review found the counterexample by exhaustive search:

    3x3 grid, all floor except one wall at 1,0.
      from 0,0 the square 2,1 is NOT visible
      from 2,1 the square 0,0 IS     visible

The cause is structural, not rounding: one point at the viewer against many at
the target is not a symmetric relation, whatever the geometry.

**Kept rather than fixed, for two reasons.** MapTool has shipped exactly this
for two decades, so asymmetry is not a defect that sinks a virtual tabletop.
And symmetry would foreclose something we will want: MapTool's **Hill VBL and
Pit VBL** are deliberately one-directional — outside a hill you see into it but
not beyond, inside it you see out — which is cover AND vantage, the thing maps
§11.3 said the pillar model could not reach, achieved with **no coordinate
system at all**. §2 excludes elevation as "a whole coordinate system"; direction
dependent blockers are how that exclusion gets lifted later without one.
A symmetric predicate cannot express a hill.

### 3.4 Sight range is an input

Patrik, 2026-08-18: *"this should not be driven by the engine. It should be
input, to the engine. what 'length of sight' does a character have. If the
character sheet does not have that, it should work as choice 1 —
unobstructed."*

So the platform asks only **"does anything block this ray"**. How far a
creature can see is a rules fact supplied by the actor; absent, sight is
unobstructed at any distance. This keeps CLAUDE.md rule 5 intact — no
game-system vocabulary in platform code — and means light, when it arrives,
changes an input rather than this design.

### 3.5 Trees are pillars

maps §11, Patrik 2026-08-14: *"trees are pillars — you cannot see THROUGH a
tree, only BETWEEN trees."*

Concealment stays **binary**. The graded feel of a wood emerges from **density**
— deeper sightlines meet more trunks, and along a forest edge the gaps line up.
No degree-valued predicate.

maps §11.2 says a trunk is smaller than its square, and **the schema cannot
express that**: `SceneObject.width/height` are `int32`, so a trunk occupying
0.4 of a square is not a value the wire can carry. That is why "fractional
later" needs an additive contract change and not merely a looser loader.

Ruled 2026-08-18: **squares now, fractional later.** This arc ships
whole-square blockers. The seam is not an interface with one implementation —
it is that the sight test consumes **rectangles in continuous coordinates**, so
a later arc hands it narrower ones and the test never learns the difference.

**Three separate facts, which an earlier draft of this section ran together
and which `internal/sight`'s own comments then got wrong in both directions
(Task 1, rounds 1–2).** Keep them apart:

1. **The proto type cannot express a fractional footprint** — `int32`. This is
   the sentence above, and it is about EXPRESSIVENESS.
2. **The 1×1 minimum is not in the type; every ingest path enforces it.**
   `create_scene` goes through `validateCreateSceneTerrain` →
   `mapdef.CheckObjectFootprints` before anything is appended, and the map-file
   and adventure loaders call the same check. MCP is not a bypass: a tool call
   becomes a `ClientCommand` into the same handler.
3. **The fold does not re-validate replayed history.** `campaign.foldEvents` →
   `engine.Apply` copies stored objects verbatim, so a log persisted before
   that validation existed can still carry a degenerate footprint.

Fact 3 is why `internal/sight` guards its input rather than trusting it: a
geometry library cannot see which path produced the `engine.Scene` it was
handed, and a zero-width blocker casts a shadow line while movement walks
straight through it — sight and movement disagreeing about the same object,
which §3.3's rotation ruling exists to prevent.

---

## 4. The projection

**One log, many views.** `internal/store` keeps exactly one append-only log;
`engine.Apply` via `campaign.foldEvents` remains the only code that changes
state. Nothing about visibility enters the fold. What changes is what a
CONNECTION receives.

`internal/gateway/server.go`'s `serve(ctx, conn, p *identity.Participant, after)`
already holds the participant for the life of the connection and already
marshals **per connection** — its own comment says so: *"Marshaled per
connection, deliberately: each pump encodes straight off its own subscription
channel with no shared cache."* The seam exists; this arc uses it.

### 4.1 The projection is a pure function of `(log-so-far, viewer)`

This is the load-bearing property and everything else falls out of it.

Because it is pure, **live streaming and reconnect catch-up compute the same
answer from the same inputs**, so a player who drops mid-fight and returns
cannot be shown what the live stream had already hidden. Nothing is stored per
connection: no cache to go stale, no second source of truth.

Catch-up composes for free. `client/src/wire.ts` advances `lastSeq` only on
events it actually receives and reconnects with `after=<lastSeq>`, so a filtered
player's replay filters identically and the gaps are invisible to them.

### 4.2 What a player receives

- Events about what their actors can see.
- A synthesized `ActorAdded` + `TokenPlaced` when something enters view. Both
  already exist; a token appearing mid-stream is something the fold does today.
- `TokenHidden` when something leaves view.
- `SceneSeen` carrying the tiles and objects currently visible.
- `SceneCreated` with grid dimensions but **empty tiles and objects** — legal
  since the tiles-optional ruling (2026-08-13), so a redacted scene needs no new
  shape.

**Two different things, and they must not be confused.**

**The scene you are IN gives you a board.** You receive its `scene_id`, name and
grid dimensions — the full outline — with tiles and objects empty. Everything
you have not seen is black, and it fills in as you explore. Patrik, 2026-08-18:
*"of course there is a board, but you do not know what is in the black area
before you enter the black area."* That is ordinary fog of war, and knowing the
room you stand in is 48×48 is not a leak; it is the shape of the paper.

**A scene you have NOT entered does not exist for you.** Patrik, same day: *"we
can not have that players know anything about a campaign's scenes that have not
yet [been] encountered… You can only know/see a scene that you participate
in."* A campaign with six scenes loaded must not hand a player six names and
sizes — that is a table of contents for an adventure they have not played.

So `SceneCreated` is projected like everything else: a player receives it at the
moment their actor is placed in that scene, synthesized and stamped with the
causing sequence exactly as a token's introduction is. Before that they learn
nothing of it; from then on they have its board and fill it in by walking.

This closes a leak an earlier draft of this spec accepted. A campaign with six
scenes loaded would otherwise have handed every player a list of six names and
sizes — *The Dragon's Lair, 40×40* — which is the shape of finding 14 with the
map rather than a goblin as its subject. A scene list is a table of contents for
the adventure, and the players are not supposed to have read ahead.

**A player who leaves a scene keeps it**, dimmed, under §3.2's terrain memory:
they were there, and that is knowledge they legitimately hold.

**Synthesized events carry the sequence of the event that caused the visibility
change.** Retraction is a range over sequence NUMBERS — `EventsRetracted`
expands `[from,to]` into a set, and both the fold and `client/src/undo.ts` skip
by number, not by identity. Stamping a synthesized introduction with its causing
sequence keeps the two coherent: retract the goblin's move and the player
correctly forgets the sighting. The residual divergence (the goblin should
return to where it stood; the player merely forgets it) **fails closed**, which
is the direction every error in this arc must run.

`retract_events` is DM/agent only (`internal/gateway/authz.go`), and the DM
receives the identity projection, so `lastUndoable` only ever runs over a
complete log. The dangerous case — a client computing undo targets from a
stream that is not the log — cannot arise.

### 4.3 The keystone invariant

> For any log and any viewer, folding that viewer's projection yields exactly
> the state the server believes that viewer can see.

    fold(project(log, viewer)) == visibleState(fold(log), viewer)

Both sides are computable today: the left with the existing Go and TS folds, the
right with the sight test over `engine.State`. Disagreement in either direction
is a defect — a player seeing a goblin they should not, or missing one they
should — and this catches both.

Today's `scenarios/goldens/` corpus becomes the `viewer = DM` case, where
projection is identity. The cross-language parity work extends rather than gets
replaced.

**AMENDED 2026-08-22, Patrik's ruling: the keystone runs at EVERY PREFIX.**

The equation above says the right-hand side is computed "with the sight test
over `engine.State`" — a function of the FINAL state. `Explored` cannot be
computed that way. It is terrain MEMORY: the union of every visible set across
history, and it is populated only by `SceneSeen`, which exists only in
projections. So folding a real log leaves `Explored` empty on every scene while
folding that same log's projection leaves it populated, and the two sides differ
on that field BY CONSTRUCTION. No final-state oracle can close the gap. §4.3 was
written before `Explored` had the shape §6 gave it.

Running the keystone at every prefix fixes this and makes the test STRONGER
rather than narrower:

- **`Visible`, tokens and actors are compared at every prefix, both
  directions.** A prefix-wise check catches a leak at the step it happens
  rather than only if it survives to the end — and a leak that appears and is
  then covered by later events is exactly the kind a final-state check misses.
- **`Explored` leaves the direct comparison, and is IMPLIED IN ONE DIRECTION —
  the one that matters.** Be precise about why, because an earlier draft of this
  amendment was not: `Explored` is unioned from each `SceneSeen`'s **`tiles`
  keys**, NOT from its `visible` set (`apply.go`'s and `fold.ts`'s SceneSeen
  arms). Those are different parts of the message. `sceneSeenFor` only ever
  builds `tiles` from the visible squares, so per message `tiles ⊆ visible` —
  and that SUBSET relation, not an identity, is what the exclusion rests on.
  It gives exactly the guarantee needed: **nothing can be remembered that was
  never visible**, so verifying `Visible` at every prefix bounds `Explored` from
  above and a leak must first appear as a currently-visible square at some
  prefix, where the check catches it. It does NOT pin `Explored` from below, and
  that is correct rather than a gap: a visible square carrying no terrain is
  deliberately never remembered, because there is no terrain to remember. On a
  bare-canvas scene `Explored` therefore stays empty however much is visible.
- The reason `Explored` is excluded must be stated where the test lives, not
  only here. "Excluded because it is path-dependent and implied by the
  prefix-wise result" and "excluded because we could not make it pass" look
  identical in a diff a year from now.

**The corpus must gain projected streams.** As of this amendment
`scenarios/goldens/` contains no `sceneSeen` at all, so the existing corpus pins
`Visible` and `Explored` being ABSENT — which is the correct DM case and proves
nothing about the populated one. A keystone run only over today's goldens would
be a test of the identity projection wearing the name of the general one.

**The oracle must be INDEPENDENT of the projection.** If `visibleState` is
implemented by calling the projection code, the equation becomes a tautology
that holds however wrong the projection is. Two derivations that share an
implementation agree about their shared bug.

### 4.4 Failure direction

**When the projection is uncertain, it omits.** A player losing a sighting is a
bug. A player gaining one is the defect this arc exists to prevent.

The projection must be **exhaustive over the `Envelope` oneof and fail closed on
an unrecognised payload.** A `default:` that forwards is how this ships broken:
`AttackRolled` names a target, `NarrationAdded` may describe a room,
`ConditionApplied` names an actor, and a note can say anything.

---

## 5. Contract additions

Additive only (ADR-007; `check:breaking` enforces). New messages and new
`Envelope` oneof field numbers after 27; nothing renumbered, nothing removed.

    message TokenHidden { string token_id = 1; }

    message SceneSeen {
      string scene_id = 1;
      map<string, TileRef> tiles = 2;
      repeated SceneObject objects = 3;
    }

`SceneSeen` carries the viewer's **whole current visible set**, not a delta.
Idempotent by construction, so there is no per-connection "what did I already
send" bookkeeping to desynchronise, and the client unions it into memory.

**Both are projection-only.** No command produces them, so they cannot
structurally reach the log — and the arc asserts that rather than assuming it.

**The actor roster is projected too.** Without it a player's character list
names every goblin in the dungeon with none on screen — finding 14 one layer up.
NPC actors are introduced only when first seen, with one explicit exception:
**actors controlled by any player are always known.** You know your party exists
when the rogue is two rooms away; you merely cannot see their token. Dropping a
party member from your own roster because they turned a corner reads as a bug,
not as fog.

### 5.1 The exception is about WHAT an actor is, not who holds it

AMENDED 2026-08-23, Patrik's ruling, after the whole-branch review found the
rule delivered one degree looser than written.

**The defect.** The sentence above says "controlled by any PLAYER". The code
implements "has any CONTROLLER" — `len(a.GetControllerIds()) > 0`, evaluated
over every actor in state with no visibility test. `controller_ids` holds
participant ids and carries no role, and nothing constrains who a grant targets.
So one `grant_actor_control` on a hidden monster publishes it to every player's
roster in full — a whole cloned `Actor` with name, attributes, resources and
`module_data`, plus its conditions — while its token stays correctly hidden. It
simultaneously opens `MayPerch` and `eyes()` on that creature, which §3.1.1
calls the constraint the whole idea rests on. Nothing caught it: §4.3's oracle
transcribes the same predicate, so both sides of the keystone agree while both
are wrong.

**The ruling: an actor carries a KIND, and the GRANT is what declares it.**
Party members are always known. Everything else is known only when seen — no
matter who currently controls it.

REVISED 2026-08-23, same day, by Patrik. A first draft of this section put kind
on the actor, fixed at creation. That was wrong in a way worth recording,
because the reasoning that replaced it is the useful part.

**Kind is not a fact about a character. It is a fact about that character's
standing right now.** A charmed monster becomes a player's to run and then
becomes a monster again. That is a TRANSITION, and a transition belongs on the
event that makes it — `ActorControlGranted` — not on a property stamped once
when the actor is created and never revisited.

Putting it on the grant resolves the ambiguity that made this defect
unfixable-looking. Two grants are byte-identical in every other respect: the DM
assigning Hollis Ketch to a player, and an agent taking the Goblin Archer to run
it. No rule could tell them apart, because the information was not present.
It was not present because nobody was asked. Ask at the grant and each case
states its own answer.

**SUPERSEDED 2026-08-24, in every clause.** This paragraph said adventure
content does not change at all, and that actors shipping with no kind was
correct because an unassigned pregenerated character is not yet in play. Both
shipped adventures do mix player characters and monsters in one directory —
`cellar-rats` ships Hollis Ketch and Mara Voss, `goblin-ambush` a Human Fighter
alongside two goblins — and the conclusion drawn from that was wrong. The
adventure format now REQUIRES `kind` on every actor, because the path written
deliberately in advance by someone who knew exactly what they were making was
the only one that could not say so.

**What survives, and is worth keeping.** An earlier draft proposed having the
compiler STAMP every adventure actor as non-party. That would have dropped all
three of those characters out of the party's roster the moment they turned a
corner — the exact regression §5 exists to prevent, on the only two adventures
that exist. Stamping remains wrong; the fix was to let content speak, not to
answer for it. Three options, and only the third works: infer (wrong), stamp
(wrong), or ask (right).

**And it separates two things that were tangled.** Kind describes the
character's standing in the fiction; control describes who is driving. They are
independent, which is what lets an agent play a party member and a person play a
monster without either becoming a special case.

Three rules follow, and the third is load-bearing:

- **Every actor states its kind at birth, and a grant may change it.**
  SUPERSEDED 2026-08-24 — this rule used to read *"an ungranted actor is NOT a
  party member,"* on the reasoning that a monster nobody has granted carries no
  kind and so defaults closed. Both halves went stale within the day: creation
  now REQUIRES a kind (`add_actor` refuses `UNSPECIFIED`, and the adventure
  format demands `"party_member"` or `"non_party"`), so an ungranted actor
  certainly can be a party member — a pregenerated character sitting in a
  campaign before anyone is assigned to it IS one, and the party seeing the
  available characters is correct rather than a leak. The rule's second clause,
  *"correct for every actor both shipped adventures contain,"* was separately
  false: three of those five are player characters. An absent kind remains
  not-a-party-member, but as belt-and-braces on a case nothing can now produce,
  not as something anything relies on.
- **Kind survives revocation.** A player leaving the table does not turn their
  character into a monster. Revocation reassigns control; it does not restate
  what the character is.
- **A grant that does not state a kind is REFUSED.** proto3 has no `required`,
  so the command handler enforces it. Without this an agent that simply omits
  the field reproduces the original leak exactly, and the migration rule below
  cannot save it — that rule cannot distinguish "a log written before this
  existed" from "a grant issued today that forgot".

RPTool arrived at the same shape independently: `Token.Type` is PC or NPC, and
`StatSheetListener` gates the stat sheet on
`isGM() || playerOwns(token) || token.getType() != Type.NPC` — an NPC you do not
own shows nothing at all. Same field, same purpose. (Theirs is a rendering
convention over data every client already holds; ours is enforced on the wire.
See §6.2.)

**There is no migration rule, and deleting the one we wrote is the most
valuable change in this section.**

Two drafts of §5.1 carried one. It read *"absent kind + has a controller → party
member; absent + none → not"*, and it existed to keep already-written logs
behaving — a plain fail-closed default would otherwise have dropped existing
party members from every roster the moment they turned a corner.

**Patrik, 2026-08-24: no campaign or ruleset is in use by anyone. There is no
history to preserve.** So the rule was protecting something that does not exist,
and it is deleted. What replaces it is one line:

> **An absent kind is NOT a party member. Always.**

Fail closed. No inference from control, no second branch, no case analysis.

**This matters more than the tidying it looks like, because the migration rule
is what made the archer leak reachable.** It was the thing that could not tell
"a log written before kind existed" from "a grant issued today that forgot" —
and since both present as an actor holding a controller with no kind, it had to
treat the forgetful grant as a party member. With the rule gone that ambiguity
has nowhere to live: the refusal of a kindless grant (above) becomes
belt-and-braces rather than the only thing standing between the system and the
bug this section exists to close.

The general shape is worth keeping even after this specific rule is gone:
**a compatibility rule is a permanent widening of behaviour bought to protect a
finite set of existing data.** It is worth it when that data exists. When it does
not, all you have bought is the widening — and here the widening was the
vulnerability itself.

**One rule, both call sites.** The roster and `MayPerch`/`eyes()` read the same
predicate today. Fixing only the roster leaves an agent-held monster perchable,
which is the half `viewpoint.go` already flags for adjudication.

**This does not close the neighbouring finding.** An actor legitimately glimpsed
and then lost from sight still receives every `ResourceChanged`,
`ConditionApplied`, `AttackRolled` and `AbilityUsed` naming it, permanently,
because `pr.actors` never forgets. That is a separate rule and a separate
decision; §3.2's "no memory of 'a goblin was here'" argues against it and the
justification in code is written about party members. Not in scope here, and
recorded so this amendment is not mistaken for closing it.

---

## 6. The client

**Two new fold arms, in both languages.** `TokenHidden` forgets a token;
`SceneSeen` unions into the explored set. Go's `Apply` gains them too — not
because a log ever contains them, but because §4.3's keystone folds a projection
and must be runnable on the Go side as well as the TS side. The two folds stay
mirrors, which is the property `client/src/fold.ts` exists to protect: *"a fold
that quietly tolerates a malformed event would diverge from the server's own view
of history."*

**Client state gains one field:** explored tiles and objects per scene. It lives
only in the client, grows and never shrinks, and is rebuilt from `after=0` on a
fresh page load — which `wire.ts` already does on first connect.

**Rendering keeps the split maps-as-geometry established**, for the same reason:
happy-dom has no canvas, so nothing drawn can be asserted. `scene-plan.ts` stays
pure and gains the visibility decision. `canvas.ts` stays thin.

**The visible set is INPUT, never a filter at the draw site.** Nothing that
paints may receive everything and decide what to skip — that is where a leak
would hide, drawing a goblin over remembered terrain in a room the player has
left. This holds for tokens and terrain alike; only the mechanism differs, and
§6.1 records which is which and why.

### 6.1 Where each decision lives

AMENDED 2026-08-21, Patrik's ruling, after reading how RPTool solves the same
problem. The original §6 said the planner "takes visible tokens as INPUT" and
emits `DrawOp`s "tagged bright, dimmed, or absent". Half of that was
unbuildable and half was worse than the alternative.

**Tokens stay DOM discs, and the visible set is passed in.** The planner does
not draw tokens and never did — `planScene` emits tiles and objects, and tokens
are discs from `tokensOnScene`, appended by `renderGrid`, which is
`renderSpectator`'s own private helper and has no other caller. So `tokensOnScene`
is the seam, and it takes the visible set as an argument rather than deriving
one. RPTool reached the same seam: `ZoneViewModel.updateVisibleTokens()` decides
which tokens exist for a viewer with no `Graphics2D` anywhere in it, publishing
a `Set<GUID>`. But it then hands its renderer the FULL layer list and skips
inside the paint loop — and beside that renderer sits `ZoneCompositor`, whose
comment says it is "responsible for providing the Zone Renderer with what needs
to be rendered" and whose body is a `// placeholder` nothing calls. That stub is
the migration they began and abandoned. The cost is legible: across all 57 of
their test files, not one exercises token visibility, and the pure seam that
could have been tested reaches a Swing frame singleton, so it cannot be built
without a window. Ours is reachable under happy-dom. Finishing the move they
abandoned costs us a parameter and buys the tests they never got.

**Remembered-but-unseen terrain is a FOG PASS, not a per-tile flag.** `planFog`
returns the geometry, `shadeFog` fills it — the same division `planGrid` and
`strokeGrid` already are, and for the same reason `canvas.ts` gives for owning
`gridInk`: *where* is planning and is asserted, *how it looks* is presentation
and is the only thing that untestable layer may own. A per-op `dim` flag fails
that test twice. It makes `canvas.ts` decide what dim means for each op kind,
and it can be forgotten on one — a brightly lit door in a remembered room, which
tells the player they can see it right now. The overlay makes that impossible by
construction. It also survives §3.5's "squares now, fractional later": a fog region
is not welded to tile edges, so the fractional seam costs a constant instead of a
redesign.

**One fog level, not RPTool's two.** They fill explored ground at partial alpha
(`FogRenderer`, 100/255) and never-seen ground opaque, because their client holds
the whole campaign and hides what you may not know. Ours never receives it: the
server redacts unexplored terrain before it reaches the wire. Unexplored is
therefore the ABSENCE of a `DrawOp`, with the background showing through, and
building a heavy fog for it would imply the client has terrain to conceal —
quietly reinstating the model this entire arc exists to remove.

**Order: terrain, then fog, then grid.** The lattice stays crisp over remembered
ground. You remember a room's shape; dimming its grid would make remembered floor
harder to count for no gain.

**Tokens on remembered-but-unseen ground are not drawn at all**, which is §3.2
restated at the render layer. RPTool agrees and arrived there independently:
`updateVisibleTokens` tests current line of sight, never the explored area. You
remember the room, not the goblin standing in it.

### 6.2 A token is a free object, so sight travels as itself

AMENDED 2026-08-22, Patrik's ruling. §6.1 left the client deriving its
visible-square set from the terrain it had been sent. That was wrong, and the
way it was wrong is worth keeping because it took two false framings to reach.

**The false framings, recorded so neither is repeated.** First: "a hole in a
map's terrain silently deletes a creature." Unreachable —
`mapdef.CheckEverySquarePresent` is all-or-nothing, and it guards both doors,
the map-file path and the `CreateScene` command path. A partial tile map cannot
arrive. Second: "a scene with no terrain is a degenerate case to tolerate." Also
wrong. **A token is a FREE OBJECT and needs no terrain to exist** (Patrik,
2026-08-22). A scene with no tiles is not a broken map; it is a bare canvas, and
creatures standing on it are real. RPTool settles it: it has no per-cell terrain
data at all, token position is unbounded pixels, `Zone.putToken` performs no
bounds or terrain validation, and sight is blocked by separate `Area` geometry
independent of any board image. Nothing there gates a token's existence,
position or rendering on map data.

**The actual defect was a disagreement, not an edge case.** `sight.VisibleFrom`
walks the GRID, not the tile map, so a scene with no tiles still has every
square in range as a candidate. (`Blockers` draws from two INDEPENDENT sources —
wall and closed-door entries in `Tiles`, and objects carrying `blocks_sight` —
so a tile-less scene has no TERRAIN blockers but can still be shadowed by
objects. Sight needs no terrain; it simply has less to stop it.) `look()` marks a token visible from that
square set, with terrain never entering. **So the server decides correctly and
sends the token.** Then `sceneSeenFor` projected those squares into `Tiles` and
dropped every square with no terrain, destroying the square set on the wire —
and the client re-derived a visible set from those tiles and hid the token. The
client was overruling a correct server decision using a lossy proxy for a
decision the server had already made. The terrain-free scene is merely where the
disagreement becomes total.

**The ruling.** `SceneSeen` carries the visible-square set explicitly, as an
additive field. `Scene.Visible` comes from that field, never from the keys of
`tiles`. `Scene.Explored` keeps its terrain-keyed meaning — a square with no
terrain has none to remember, so there is nothing to fog there — which means the
two fields now come from DIFFERENT SOURCES and may legitimately differ; that is
pinned in both languages rather than left for a reader to assume. The token
filter stays, but it is no longer an independent decision: it is a consistency
check against the server's own set, and with the correct set the two cannot
disagree.

**Server decides what you may see; the client draws it.** Every visibility fact
on the client now originates server-side. The client owns exactly one thing —
MEMORY — because §4.1 keeps the projection a pure function of
`(log-so-far, viewer)` and so holds no per-viewer history. That is not a
loophole: `Explored` is unioned from each `SceneSeen`'s `tiles` keys, and
`sceneSeenFor` builds those only from squares this viewer can currently see, so
`Explored` can contain nothing that was withheld. (It is a SUBSET of what was
sent, not an identity with it — see §4.3's amendment, where the distinction is
load-bearing.) RPTool's client memory has the
identical shape and the opposite guarantee, purely because its clients are sent
the entire campaign to begin with — every zone, every token including GM-only
ones, with no per-recipient filtering anywhere in its server. Borrow its
geometry; never its distribution.

---

## 7. Testing

ADR-009 applies unchanged: tests first, behavioural RED, fault-injection proof
per load-bearing assertion.

**The keystone** is §4.3, run in both languages over a shared corpus.

**Three properties, each catching a class of bug:**

- **Asymmetry is PINNED, not assumed.** An earlier draft asserted symmetry
  here and was wrong (§3.3.1). Assert the counterexample instead — from 0,0 the
  square 2,1 is unseen while from 2,1 the square 0,0 is seen — so that anyone
  who later "fixes" the sampling into symmetry has to come here and read why it
  is that way. A property this spec got wrong once is worth a test that states
  the truth out loud.
- **Purity.** The projection computed live and from catch-up are identical.
  That is what makes a mid-fight reconnect safe.
- **Monotonic memory.** Explored terrain never shrinks.

**The founding test is session zero itself.** Replay it: a player at seq 20 must
not be able to see, or move a token onto, (19,8). The defect that stopped play
becomes an executable assertion.

**Every visibility test needs a PLAYER seat.** The DM sees everything, so no
test exercising the DM can catch a projection bug — it is easy to write a suite
that proves nothing.

**The sight test is pure geometry** and gets table-driven boundary tests.
Expect mutation survivors at `>=` versus `>` on ray endpoints and footprint
edges; that is exactly where `internal/mapdef`'s thirteen lived.

**The new package enters `tools/check-mutation.py` PACKAGES in the same commit
that creates it.** `internal/mapdef` shipped in neither the gate nor
`tools/mutation-scope.md`, so it was silently ungated; adding it later exposed
thirteen live mutants and one real bug. Not repeating that.

**Client tests assert `DrawOp` arrays, never pixels.** The negative case is the
one that matters: a token outside line of sight produces NO `DrawOp`, not a
`DrawOp` something later declines to paint.

---

## 8. What could go wrong

**The projection is security-critical code that did not exist before.**
Everything prior leaked by design; from here a bug in one function is the
difference between fog and none. It earns the paranoia normally reserved for
authz.

**A channel nobody projected** is the most likely real bug — see §4.4.

**Performance is a cliff, not a slope.** Sight recomputes per viewer on every
event that could change it. At 60×60 with five rays a square and a handful of
participants this is nothing; the risk is an outdoor map with many actors where
one move triggers a full recompute for everyone. Measure before it becomes a
table complaint. The wire ceiling (maps §7, `mapdef.MaxWireTiles`) caps how
large a tiled scene can get anyway.

**Partial visibility of a moving token** is the edge to watch hardest. A goblin
walking along a wall enters and leaves view repeatedly. Get the transition wrong
and it flickers — or worse, a `TokenHidden` arrives for a token the client never
had, and the strict fold throws, taking the whole client down rather than merely
showing too much.

**Two known gaps sit directly in this arc's path**, both found at the maps demo
gate, neither in scope here:

- The web client **cannot send `open_door`** — only the MCP agent seat can. If
  opening a door is how a room is revealed, a human DM cannot run the fog.
- The standard pack ships **zero object art**, so `blocks_sight` scenery cannot
  be authored without a custom pack.

---

## 9. Exit criteria

1. A player in a scene has its full board outline from the start, black where
   unexplored — and sees the room they are in, but not the room beyond a closed
   door.
2. Opening the door reveals the room beyond, in one event, to everyone who can
   see through it.
3. A player who leaves a room keeps its terrain, dimmed, and loses the creatures
   in it.
4. A seat with no actor sees no scene at all — not its name, not its size.
   Being granted a character gives it both a place to stand and eyes.
5. A spectator perches on a party member and sees exactly what that character
   sees; hopping to another shoulder switches the view and keeps what the bird
   has already seen. A perch on an NPC is REFUSED by the server.
6. **A scene a player has never entered is absent from their stream entirely.**
   Load six scenes into a campaign; a player in one of them can enumerate
   exactly one.
7. **Session zero cannot happen again:** the goblin is not on the wire, not on
   the board, and its square cannot be targeted by a player who cannot see it.
8. The DM's stream is byte-for-byte unchanged, and the MCP agent seat notices
   nothing.
9. `task check` green, no gate weakened, the new package mutation-gated from its
   first commit.
