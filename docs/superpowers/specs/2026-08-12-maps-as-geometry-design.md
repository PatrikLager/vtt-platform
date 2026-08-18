# Maps as geometry — design

**Status:** draft, awaiting Patrik's approval
**Chosen from:** `docs/superpowers/notes/2026-08-12-session-zero-backlog.md`
**Supersedes in ordering:** backlog item S1 (hidden information), which this
arc was originally going to be. See §1.3.

## 1. Why this, and why now

### 1.1 A scene is currently four numbers

`SceneCreated` carries `scene_id`, `name`, `grid_width`, `grid_height`. That is
the whole of a place. There are no walls, no floors, no doors, no furniture —
`client/src/view/grid.ts` draws a plain lattice and positions tokens on it.
Session zero was the first time anyone rendered a real adventure at real size,
and three separate findings turned out to be the same missing thing.

### 1.2 An LLM cannot reason about a picture

The founding spec files maps under *"Asset Service — maps/portraits/audio, incl.
AI-generated media"*. Maps as images. That framing does not survive this
platform's own premise.

**The DM is a language model.** Handed a PNG it cannot say the corridor turns
left, that there is cover behind the pillar, or which squares a goblin can
reach. Everything a map is *for* — Patrik, 2026-08-12: *"where they can go and
what choices for movement they have"* — is exactly what a picture cannot give
the one participant who has to narrate it.

A map declared as geometry can be read, reasoned about, and authored by a model.
Art then lands on top of structure that already exists, rather than structure
being reverse-engineered from art. Portraits and audio are media. **A map is
world structure that happens to be drawn.**

### 1.3 Why this precedes hidden information

The backlog ranked hidden information first — it stopped play, and it is the
most expensive thing to retrofit. This arc goes first anyway, for three reasons
that emerged while designing it.

1. **A wall with no map is an invisible barrier** (Patrik, 2026-08-12). Fog
   revealing featureless lattice reveals nothing, and a player halted by
   geometry they cannot see is worse off than one who sees too much.
2. **Line of sight has nothing to bite on.** `goblin-ambush` is a 32x32 field
   with three tokens and no cover. The goblins stood ~20 squares from the
   fighter in daylight; **no visibility model hides them**, because correctly
   they are visible. That ambush failed for want of terrain, not of an engine
   feature — which the backlog's S1 entry did not see.
3. **Committing wall vocabulary before knowing what a map is means committing
   it twice.** The wire contract is additive-only (ADR-007, `check:breaking`),
   so a shape we regret is permanent.

Visibility follows immediately, and consumes this arc's output: `blocks_sight`
and door state are declared here and read there.

### 1.4 It also closes a defect

Backlog T1/#19: the board is sized `gridWidth * CELL` (1408px for a 32x32
scene), so the page grows with the map and the controls sit roughly 1450px down
— below the fold on every laptop. There is no camera anywhere in the client:
grep for zoom, pan, scale or fit returns nothing.

A vector map needs a camera by its nature, and `cellFromPoint` (`grid.ts:52`)
already divides by `geom.cell` rather than a constant, so the seam exists. That
defect is closed as a consequence of doing maps properly.

### 1.5 Success criterion

**An LLM handed the format document and a pack manifest, with no other help,
authors a map that loads and plays.** Not "we can write maps" — *they* can.
That is the test of whether this is really an API, and §10 makes it an exit
criterion rather than an aspiration.

## 2. Non-goals

Each is a plausible thing to start doing halfway through.

- **Line of sight, fog of war.** The next arc. This one supplies its input.
- **Lights, night, darkvision.** Arc C. Faking light now means redoing it, and
  a painted glow that does not match where light actually falls is the same
  species of lie as a wall in the wrong place.
- **Stealth versus perception.** The one form of concealment that is not
  geometry. Needs the ruleset; lands after it.
- **A map editor or painter.** Patrik, 2026-08-12: *"at this point, i would
  say, no editor — we describe the API, this is how you can define your maps,
  these are the choices."* The format is the interface.
- **AI art generation.** A pack is drawn once, offline, by an image model or a
  human. Not a pipeline.
- **Terrain mutation.** No collapsing walls. Terrain is immutable per scene;
  adding mutation later is additive.
- **Elevation, multiple levels.** A whole coordinate system.
- **Curved and diagonal walls.** The accepted cost of grid-aligned structure.
- **Object interactivity.** See §3.4.
- **Pathfinding.** Movement is validated, never computed.
- **Touch and tablet layout.** Deferred by Patrik, 2026-08-12; desktop-first
  per `client-design.md` §8 — which this arc's predecessor spec misquoted as
  "tablet-first responsive" and which is corrected at the merge gate.

## 3. The model

### 3.1 Structure snaps; furniture does not

Three structural models were considered: a pure tile grid, polygons with wall
segments (MapTool's VBL), and a hybrid. **The hybrid is chosen.** Walls, floors
and doors are tiles, so the grid stays authorable and painted floor tiles align
by construction. Objects carry a position, rotation and footprint, so a chest
sits at an angle and a rug spans four squares.

Polygons were rejected on authorability: an LLM emitting coordinate lists
produces rooms that do not close, and painted floor tiles then align to nothing.
The cost is that a round tower is a staircase of squares. Accepted.

### 3.2 Nature and art are different things, and only one has authority

The rule, in Patrik's words (2026-08-12):

> *"The ART will never decide the 'nature' of the square/item. That is in the
> base rule — you can not go through a wall or closed door. Has nothing to do
> with what the tile is called."*

- **Nature** is declared per square by a standard tile name, and enforced by
  the engine. It decides movement now and sight next arc.
- **Art** is a picture. The renderer reads it; nothing else does. It cannot
  change a single fact.

**A wall drawn as floorboards is still a wall.** An earlier draft made a
kind mismatch a load error; that was withdrawn, because a wall that looks like
a passage is an *illusory wall* — classic dungeon craft, and about to become a
real feature one arc from now. Mismatch is permitted; at most it warns.

### 3.3 Standard tiles are a vocabulary of natures

Patrik, 2026-08-12: *"you have a standard of 'tiles', like earth, wood, water.
And you always define those in your map. Those can then be replaced by
custom/user tiles."*

The standard pack ships a documented set of natures — eleven, each carrying a
`kind`, a `material` and a default picture:

| name | kind | material |
|---|---|---|
| `stone-wall` | wall | stone |
| `wood-wall` | wall | wood |
| `wood-door` | door | wood |
| `stone` | floor | stone |
| `wood` | floor | wood |
| `earth` | floor | earth |
| `grass` | floor | grass |
| `sand` | floor | sand |
| `water` | floor | water |
| `metal` | floor | metal |
| `ice` | floor | ice |

**Amended 2026-08-13 (Patrik's ruling), and recorded rather than corrected
silently** per CLAUDE.md rule 7. This section's first draft was prose listing
ten names and omitted `wood-wall`; the implementation plan's table carried
eleven, and the Task 2 review caught the divergence before anything depended on
it. `wood-wall` stays: `stone-wall` and `wood-door` were both already present,
so it completes an obvious pair, and a wooden partition or palisade is ordinary
dungeon furniture. The omission was a slip in prose, not a decision.

The list is a **table now rather than a sentence**, which is the durable half of
the fix — §9 calls this vocabulary a one-way door ("adding a nature later is
additive; removing one is not"), and a one-way door should not be specified in
a form where an entry can go missing without anyone noticing.

**A door is one nature, not two.** `wood-door` is `kind: door`; whether it is
open is folded state (§6), never part of the tile name. A pack therefore
supplies two pictures for a door tile — `file_open` and `file_closed` — and the
renderer picks by state. Encoding openness in the vocabulary would put a mutable
fact in an immutable declaration, and terrain is immutable (§2).

**A custom pack adds pictures; it never adds natures.** Wanting a new nature —
`obsidian` — is a request to extend the shared vocabulary, which is a different
and more serious thing than shipping a nice-looking tile. The spec draws that
line explicitly so the vocabulary cannot grow by accident.

`kind` is a small closed set the platform understands **spatially**: `wall`,
`floor`, `door`. `material` is an **opaque tag** the platform never interprets
— whether water slows you is the ruleset's business. That is what keeps
CLAUDE.md rule 5 satisfied: no game vocabulary reaches platform code, because
the platform only ever sees strings it does not read.

### 3.4 Objects are scenery; anything that acts is an actor

An object has a position, a footprint, `blocks_sight` and `blocks_move` flags,
and a picture. **Anything that acts, moves, or holds state is an actor with a
token** — which the platform already models fully.

**An object's `kind` is not a tile's `kind`, and the collision is deliberate
only in name.** A tile's `kind` is the closed spatial set of §3.3 —
`wall`/`floor`/`door` — and the engine reads it. An object's `kind` is an
**open descriptive label** (`boulder`, `chest`, `table`) that the platform never
interprets: it exists so the DM and the LLM can talk about the thing. An
object's structural effect comes from its two flags, never from its label. A
reader must not infer behaviour from `kind: "boulder"`.

A chest that matters mechanically is an actor. A chest that is furniture is an
object. This line is what stops `SceneObject` quietly becoming a second entity
system with its own half-implemented lifecycle.

## 4. The format — the API

### 4.1 A map

Two layers, both keyed by square, at the same level of granularity — Patrik's
requirement that each square have a direct relationship with its tile:

```json
{ "id": "shrine", "name": "Obsidian Shrine",
  "grid_width": 3, "grid_height": 3,
  "pack": "mossy-keep",

  "tiles": { "0,0":"stone-wall", "1,0":"stone-wall", "2,0":"stone-wall",
             "0,1":"wood",       "1,1":"wood",       "2,1":"wood",
             "0,2":"stone-wall", "1,2":"stone-wall", "2,2":"stone-wall" },

  "overrides": { "0,1":"wood-planks-split-3" },

  "objects": [ {"kind":"boulder","at":[1,1],"size":[1,1],"rot":0,
                "blocks_sight":true,"blocks_move":true,
                "art":"boulder-mossy-2"} ],

  "placements": [ {"token_id":"tok-fighter","actor_id":"act-fighter","x":2,"y":1} ] }
```

- **`tiles`** — required for **every** square. A standard tile name. This is
  what the square *is*. Keys are `"x,y"`, column then row; the separator is a
  comma because a dot reads as a decimal.
- **`overrides`** — optional and sparse. A custom tile from `pack`. Changes
  **only the picture**. Delete the whole block and the map renders and plays
  identically in every way that matters.

There is no implicit fallback anywhere. Every square names its own tile, which
is why the two layers can be read independently.

**Keyed only; no ASCII shorthand in v1.** An ASCII grid plus a legend was
designed and discarded: it compresses many squares into one character, which
forces an override concept to express "this one differs" and makes the map two
things that must be aligned by eye. Keyed is unambiguous. The cost is real —
a 32x32 map is 1024 lines and you cannot see the room's shape by reading it —
and §9 treats it as a measured risk rather than a settled question.

### 4.2 A pack

`pack.json` beside the images. The descriptions are not decoration: they are
what let a model choose tiles deliberately rather than at random, and without
them §1.5 cannot be met.

**Where a pack lives** (added 2026-08-13; the first draft said only "beside the
images" and never said beside *which* images, which a Task 4 review correctly
called unsanctioned format surface — a path becomes an API the moment somebody
else's adventure ships one):

- **Inside an adventure:** `adventures/<id>/tiles/pack.json`. Mirrors
  `guide.md`, which already lives in the adventure directory and is already
  served over HTTP.
- **Beside a standalone map:** `maps/<id>/tiles/pack.json`, the same shape, so
  a map moving into or out of an adventure does not change where its art sits.

Both are served over HTTP alongside the images (§7), never through the log.

```json
{ "id": "mossy-keep", "name": "Mossy Keep", "cell_px": 64,
  "tiles": [ {"name":"wood-planks-split-3", "file":"planks_03.png",
              "kind":"floor", "material":"wood",
              "desc":"worn pine planks, one board split along the grain"},
             {"name":"oak-door-banded", "kind":"door", "material":"wood",
              "file_closed":"door_shut.png", "file_open":"door_open.png",
              "desc":"iron-banded oak door"} ],
  "objects": [ {"name":"boulder-mossy-2", "file":"boulder_02.png",
                "footprint":[1,1],
                "desc":"granite boulder, moss on its north face"} ] }
```

A tile's declared `kind`/`material` here is **advisory metadata** for authoring
and selection. It carries no authority (§3.2).

Resolution has exactly two levels, per Patrik: *"there are user designed maps
with user designed tiles and then there are standard tiles. Nothing in
between."* A name resolves in the adventure's or map's own pack; failing that
it is a standard name. **A campaign-wide override tier was proposed and
rejected.**

### 4.3 Maps are their own object

Patrik, 2026-08-12: *"map should be its own object, with the ability to be
loaded outside the adventure."*

A `maps/` directory beside `adventures/` and `rulesets/`, served the same way.
A map loads directly — pick-up play, someone drops in a dungeon and you use it.

An adventure still carries its own maps, because adventure-format §2.2 chose
self-containment deliberately (*"No bestiary format"* — shared libraries were
rejected to avoid resolution and versioning problems), and a map is content.

**Both paths compile through one code path to the same events.** The only
difference is where the file was read from. This matters beyond tidiness: it
decouples this arc from the content problem. Backlog S3 says `goblin-ambush`
cannot seat a second player, so if maps existed only inside adventures, nothing
here could be demonstrated until someone authored a whole adventure.

### 4.4 Validation, at boot, fail loud

Matching adventure-format §7's existing posture — fail at startup, not at the
table.

- Every square in the grid has a `tiles` entry; no entry lies outside the grid.
- Every tile name resolves, in the declared pack or the standard vocabulary.
- Every `overrides` key names a square that exists.
- Objects lie inside the grid; every `art` name resolves.
- **Placements do not land inside a wall** — a class of error the current
  format cannot even express.
- A `kind` mismatch between an override and its base **warns**; it never
  refuses (§3.2).

## 5. Contract additions

Additive only, permanent, so this is the shortest list that works.

Terrain rides on the scene rather than arriving as its own event:

```protobuf
message SceneCreated {
  string scene_id = 1; string name = 2;
  int32 grid_width = 3; int32 grid_height = 4;
  map<string, TileRef> tiles = 5;      // NEW: "x,y" -> resolved nature + art
  repeated SceneObject objects = 6;    // NEW
}
```

An old reader sees a scene with no terrain — **incomplete but never wrong**,
the same failure direction `Actor.controller_id` already established.
`CreateScene` gains the matching fields.

with the two referenced messages:

```protobuf
message TileRef {                     // one square, resolved at load
  string kind = 1;                    // "wall" | "floor" | "door"
  string material = 2;                // opaque; the ruleset's business
  string art = 3;                     // pack tile name, or empty for standard
}

message SceneObject {
  string object_id = 1;
  string kind = 2;                    // OPEN descriptive label (§3.4)
  GridPosition at = 3;
  int32 width = 4; int32 height = 5;  // footprint in squares
  int32 rotation_degrees = 6;
  bool blocks_sight = 7;
  bool blocks_move = 8;
  string art = 9;
}
```

**The loader resolves names into facts.** `TileRef` carries the resolved `kind`
and `material` *and* the art name, so the event log holds the facts the engine
needs without depending on a pack file being present at replay time. Packs are
needed to author and to draw, never to fold.

**Doors carry no open/closed field here** (§3.3): openness is folded state, and
terrain is immutable.

**Nothing for art beyond that reference.** Pack manifests and images are assets
served over HTTP beside `guide.md` — never in the log, and therefore never
frozen by ADR-007.

Doors are the one dynamic part:

```protobuf
message OpenDoor  { string scene_id = 1; GridPosition at = 2; }
message CloseDoor { string scene_id = 1; GridPosition at = 2; }
message DoorOpened { string scene_id = 1; GridPosition at = 2; }
message DoorClosed { string scene_id = 1; GridPosition at = 2; }
```

Door state folds like everything else, so replay reconstructs it and undo works
on it for free. `open_door` and `close_door` appear as MCP tools automatically,
the way `load_adventure` did.

## 6. Movement and doors

**Doors are dynamic, and that follows from movement being enforced.** An
earlier draft made them static on the grounds that no state would read them;
that was wrong the moment the map constrains movement, because a closed door
that cannot open is just a wall, and a dungeon whose doors never open is not a
dungeon.

**A map you can walk through is a picture of a map.** Enforcement is what makes
this arc demonstrable on its own rather than paying off only in the next one.

Patrik's rule, 2026-08-12: **hard for players, free for the DM.**

- A player's token may not enter a `wall`, a closed `door`, or any square
  covered by an object whose `blocks_move` is set.
- DM and agent may place anything anywhere — they are authoring the world, not
  moving through it. Staging a creature inside stone is legitimate.
- A player may work a door only if a token they control is **adjacent** to it;
  DM and agent unrestricted. Otherwise anyone could fling open a door across
  the dungeon.

Both rules are **spatial, not game-system**, so rule 5 is untouched: "a token
cannot stand inside solid rock" is the same kind of check as "inside the grid".
Difficult terrain, flying and phasing remain the ruleset's business.

`commandRoles` gains two rows:

| command | dm | agent | player | spectator |
|---|---|---|---|---|
| `OpenDoor` / `CloseDoor` | yes | yes | yes (adjacent) | no |

## 7. Rendering and the camera

**The client composes the picture; the server never draws.** Each viewer needs
their own camera and, next arc, their own line of sight — a server-rendered
image could serve neither.

**The board becomes a fixed viewport with a camera inside it.** The pane takes
the space the layout gives it and the map is drawn through a scale-and-offset
transform. Scene size stops affecting page height entirely: a 200x200 outdoor
map and a 10x10 room lay out identically. **This is the T1/#19 fix.**

**Amended 2026-08-13 (Patrik's ruling), because the sentence above was true of
LAYOUT and false of the WIRE.** Task 4 measured it: `SceneCreated` carries one
`TileRef` per square as protojson, so a 32x32 scene is **45.5 KiB** and a
200x200 scene is **~1.79 MiB in a single frame**. coder/websocket's default
read limit is 32768 bytes, which is why loading `goblin-ambush` tore down every
connection until the reading side raised it — a bug this arc found and fixed
only because an implementer read "existing adventures must still compile" as
"must still work end to end".

So **the honest limit today is roughly a 60x60 scene** against the 200 KiB read
limit now set. A 200x200 map lays out identically and does not arrive.

**Not fixed in this arc, deliberately.** The remedy is a compact wire encoding —
a palette plus index rows, which would put 200x200 near 40 KB — and it costs an
additive contract change plus its own task. Nothing in §10's exit criteria needs
a scene larger than the demo map, and growing an arc mid-flight to chase a size
nobody is using yet is how arcs stop landing. **Filed as its own follow-up.**

Worth stating plainly because it is a consequence of a decision rather than an
accident: the per-square explicitness of §4.1 — every square naming its own
tile, which is what makes the format atomic and diffable — is exactly what makes
it large on the wire. **Authoring and transport need not be the same shape.**
The loader already resolves tile NAMES into facts, so the wire form is ours to
choose freely without touching how a map is written.

- Fit on scene change, so you always start seeing the whole map.
- Wheel to zoom, drag to pan. Desktop-first; touch gestures are deferred.
- `cellFromPoint` needs only the camera transform applied before it.

Two layers, chosen for what each is good at:

- **Canvas 2D for the map** — tiles, walls, doors, scenery. A thousand tiles is
  nothing for canvas and stays smooth at forty thousand with viewport culling;
  a thousand DOM nodes would not.
- **DOM for tokens and overlays** — they already carry resource chips,
  condition dots and click handlers. Both layers share one transform.

An art name that resolves nowhere draws a **visible missing-tile marker**, not
a blank square. Boot validation should have refused it already; if something
slips through it must be obvious rather than silently absent.

**Everyone still sees the whole map.** No filtering in this arc. Stated rather
than assumed, because maps and fog arrive together in most people's heads and
here they deliberately do not.

**v1 stops at props** (Patrik, 2026-08-12): textured surfaces and painted
furniture, no shadows, glow or vignette. Ambience waits until arc C makes light
real, because faked light that does not match where light falls is the same lie
as a misplaced wall.

## 8. Testing

ADR-009 throughout: tests first, behavioural RED, fault-injection proof per
load-bearing assertion.

- **Format and validation** — table tests over invalid fixtures, following the
  `internal/rules/testdata/invalid-v2/` pattern. Every refusal in §4.4 gets a
  case.
- **Compile** — map and adventure to events, pinned by goldens; the `goldens/`
  precedent already exists. **Both load paths, one compiler**, asserted by
  producing identical events from an inline scene and a standalone map.
- **Engine** — movement refused into a wall and a closed door, accepted through
  an open one, DM unconstrained; door state folds and survives replay.
- **Authz** — new rows, plus the existing reflection test asserting every
  command in the oneof has role cells (`HasRoleCellsForTest`).
- **Client** — tile resolution, camera transform, hit-testing through the
  camera.

**The testing risk that needs a design response.** The client suite runs on
happy-dom, which has **no canvas implementation**. Nothing touching a canvas
context can be asserted. That is exactly how the `ArmakAsmeDM` defect shipped
(backlog #13): a correct DOM assertion over a rendering nobody looked at.

So the renderer is split. **What to draw is a pure function** — given a scene, a
camera and a pack, produce a list of image-at-rect instructions, fully testable
and mutation-testable with no browser. **Drawing is a thin layer** that walks
that list calling `drawImage`, small enough to verify by looking. This mirrors
`grid.ts`, where `tokensOnScene` returns data and the view assembles the DOM.

## 9. What could go wrong

- **Keyed-only authoring defeats the LLM.** The plausible failure of §1.5: a
  model emitting 1024 keyed lines works blind and cannot see the room's shape.
  **This is measured, not assumed** — exit criterion 6 tests it directly. If it
  fails, an ASCII shorthand that expands into keys at load is additive, cheap,
  and changes no semantics.
- **The standard vocabulary is wrong and it is permanent.** Adding a nature
  later is additive; removing one is not. Response: ship deliberately few.
- **The art layer swallows the arc.** Bounded by stopping at props, with no
  editor and no generation pipeline.
- **Canvas is a new primitive in this codebase.** Confined by §8's split.
- **Two load paths drift.** Prevented by construction: one compiler, and a test
  asserting both paths emit identical events.
- **Objects become a second entity system.** Held off by §3.4's line, which
  must be defended in review rather than assumed.

## 10. Exit criteria

1. A standalone map with real cover loads via `maps/` and renders with the
   standard pack.
2. A token walks; is **refused** by a wall; a door opens; it walks through.
3. The same map loads with a custom pack: the art changes, nothing else does.
4. The board fits the window at any scene size and the controls are reachable.
   **Backlog T1/#19 closed.**
5. `task check` green, no gate weakened, coverage ratchets held.
6. **An LLM given only the format document and the standard pack manifest
   authors a map that loads and plays.** If this fails, the format is not yet
   an API — and that is the criterion that decides whether this arc delivered
   what was asked for.
