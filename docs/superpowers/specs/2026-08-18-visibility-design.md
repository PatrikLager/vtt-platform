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

**A perch may only target a player-controlled actor.** This is the constraint
the whole idea rests on: a spectator perched on the Goblin Archer would watch
the ambush from inside it, and the arc would be undone in a single click.
Enforced server-side, never by which names the UI happens to offer.

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

maps §11.2 says a trunk is smaller than its square, and **the format cannot
express that**: `SceneObject.width/height` are `int32` with a 1×1 minimum. Ruled
2026-08-18: **squares now, fractional later.** This arc ships whole-square
blockers. The seam is not an interface with one implementation — it is that the
sight test consumes **rectangles in continuous coordinates**, so a later arc
hands it narrower ones and the test never learns the difference.

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
pure and gains the visibility decision, emitting `DrawOp`s tagged bright,
dimmed, or absent. `canvas.ts` stays thin.

**The planner takes visible tokens as INPUT.** It must never receive all tokens
and filter at paint time — that is where a leak would hide, drawing a goblin
over remembered terrain in a room the player has left.

---

## 7. Testing

ADR-009 applies unchanged: tests first, behavioural RED, fault-injection proof
per load-bearing assertion.

**The keystone** is §4.3, run in both languages over a shared corpus.

**Three properties, each catching a class of bug:**

- **Symmetry.** If A sees B, B sees A — same ray, same blockers. Asymmetric
  sight is the classic geometry bug and is invisible in a screenshot.
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
