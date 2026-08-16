# The vtt-platform map format

This document describes the on-disk JSON format for a **map** — a described
space of walls, floors, doors and scenery that an LLM game master can reason
about directly. It assumes nothing about the surrounding codebase: everything
you need to author a map that loads and plays is here.

If you take one sentence away, take this one: **art never decides nature.**
A wall drawn to look like floorboards is still a wall. Everything below
follows from that.

## 1. Why a map is two separate things

A map has two layers, and they answer two different questions:

- **What a square *is*** — a wall, a floor, a door, and (if it's a floor or
  wall) what it's made of. This is **nature**. It is what the game engine
  enforces: you cannot walk through a wall or a closed door regardless of
  what picture is drawn on top of it.
- **What a square *looks like*** — the picture the renderer draws. This is
  **art**. It can change freely — reskin a whole dungeon with a different
  tileset — without touching a single rule about where anyone can stand.

Keeping these separate is a deliberate design choice, not an accident of
layering. A tool that mixed the two would let "this square looks like a
passage" quietly mean "this square is a passage", which is exactly how a
secret door stops being a secret. Splitting them means an *illusory wall* — a
wall drawn to look like open floor — is legitimate, ordinary dungeon craft,
not a bug.

Concretely, a map file has (up to) two top-level maps, both keyed by square,
at the same granularity:

- `tiles` — required, one entry **per square**, naming the square's nature.
- `overrides` — optional and sparse, naming a *picture* for a square whose
  nature is already declared in `tiles`. Delete the entire `overrides` block
  and the map still loads and plays identically in every way that matters —
  only the pictures change, back to whatever the standard vocabulary's
  default look is.

There is no implicit fallback anywhere in `tiles`: every square that has
terrain at all names its own tile, explicitly. That is what makes the format
mechanically checkable (a loader can tell you "square 4,7 has no tile" rather
than silently guessing) and what makes two maps diffable square by square.

`tiles` is the one part of a map that can also be **entirely absent**. A map
with no `tiles` key has no terrain at all — a bare grid, exactly like a scene
before this format existed. That is legal and stays legal forever, because a
format meant to be authored by third parties (and by an LLM) does not get to
retroactively invalidate what came before it. What is **not** legal is a
*partial* `tiles` map: if you declare terrain for even one square, every
square in the grid needs an entry. Completeness is the whole point of the
format once you've opted in.

## 2. The `"x,y"` key convention

Every square is addressed by a string key: `"x,y"` — the column first, then
the row, separated by a **comma**. Both `tiles` and `overrides` use this
convention, and so does the door-state map on the wire.

- `x` is the column, counting from `0` at the left.
- `y` is the row, counting from `0` at the top.
- The separator is a **comma**, not a period. A period would read as a
  decimal point (`"4.7"` could be misread as one number), and a comma cannot
  be.

So `"0,0"` is the top-left square of the grid, and for a map that is
`grid_width` squares wide and `grid_height` squares tall, valid keys run `x`
from `0` to `grid_width - 1` and `y` from `0` to `grid_height - 1`.

## 3. The standard tile vocabulary

Every map can use a fixed set of **standard tiles** — natures that need no
custom pack at all. This list is deliberately short and is meant to stay
that way: adding a nature later is easy (it only ever adds a new possible
value), but removing one is not (an existing map might already use it), so
the platform ships as few of these as it can defend.

Each standard tile has a **name** (what you write in `tiles`), a **kind** —
the closed, structural set the engine actually reads (`wall`, `floor`, or
`door`) — and a **material**, which is an *opaque* label the platform never
interprets. `material` exists for a ruleset or a renderer to hang meaning on
(does difficult terrain slow you down on `sand`? does `ice` need a save? —
none of that is this format's business), never for the platform itself.

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

A tile's `kind` is what the engine enforces spatially: a player's token may
not enter a square whose kind is `wall`, or a `door` that is currently
closed. `material` carries no such authority — it is a tag, not a rule.

**A door is one nature, not two.** `wood-door` names the tile whether the
door is currently open or shut; openness is *folded state* (see §6), tracked
separately from the tile's declared nature, exactly the way a token's
position is tracked separately from the map that constrains it. A map never
declares a door "open" or "closed" in `tiles` — it only declares that a
square *is a door*.

## 4. Custom tiles: the `overrides` layer and packs

`overrides` lets a square keep its standard nature (from `tiles`) while
showing a **custom picture** instead of the standard tile's default look. An
override's value is the *name of a tile inside a pack* (see §5) — never a
standard tile name, and never anything that changes what the square *is*.

```json
"tiles":     { "4,2": "stone-wall" },
"overrides": { "4,2": "mossy-blockwork-3" }
```

Square `4,2` is still, structurally, a wall — a token still cannot walk
through it, and nothing about line of sight (a later feature that reads this
same nature) changes. Only the picture drawn for it changes, from whatever
the standard `stone-wall` default is to the pack's `mossy-blockwork-3`
picture.

**Resolution has exactly two levels, and only two.** A tile name resolves
first against the map's own pack (if it declares one and the square is
overridden), and otherwise against the standard vocabulary in §3. There is
no third, campaign-wide, "everyone's tiles" tier — a pack is scoped to the
one map (or adventure) that names it.

A tile whose `kind` (from its pack entry's own *advisory* metadata, see §5)
disagrees with the base tile's actual kind is **not an error**. It produces a
warning, and nothing more — that mismatch is precisely how an illusory wall
gets made. Refusing it would forbid a legitimate trick.

## 5. Objects: scenery, not actors

An `objects` array lists scenery: things that occupy space and may block
sight or movement, but never act, move on their own, or hold changing state.
(Anything that *does* act — a monster, an NPC, a player's character — is
represented as an actor with a token, a different and already fully-modelled
part of the platform. If you find yourself wanting an object to "do"
something, it should probably be an actor instead.)

```json
{
  "id": "crate-1",
  "kind": "crate",
  "at": [6, 2],
  "size": [1, 1],
  "rot": 0,
  "blocks_sight": true,
  "blocks_move": true,
  "art": "crate-wood"
}
```

| field | meaning |
|---|---|
| `id` | a unique identifier for this object within the map |
| `kind` | an **open, descriptive label** — `"crate"`, `"pillar"`, `"table"` — for a human or an LLM to talk about the object by. The platform never interprets it. Do not infer behaviour from it: a `kind: "boulder"` is not, by itself, anything. |
| `at` | `[x, y]` — the square the object's top-left corner occupies |
| `size` | `[width, height]` in squares — a `[2, 1]` object spans two squares horizontally |
| `rot` | rotation in **degrees**, applied about the footprint's centre |
| `blocks_sight` | `true` if the object blocks line of sight through its footprint |
| `blocks_move` | `true` if a player's token may not enter its footprint |
| `art` | the name of a picture in the map's pack (see §5.1) |

**`kind` here is a different thing from a tile's `kind`, on purpose, and the
name collision is deliberate.** A tile's `kind` is the closed, three-value
set from §3 that the engine enforces. An object's `kind` is an open label
the platform never reads for meaning. Do not assume an object's `kind`
implies anything about `blocks_sight` or `blocks_move` — those two flags are
the *entire* mechanical effect an object has. A `"boulder"` with both flags
`false` is, mechanically, nothing more than a picture; a `"curtain"` with
`blocks_sight: true` is real cover.

An object's `art` must name a picture declared in the map's pack — there is
no standard-vocabulary fallback for objects the way there is for tiles,
because objects have no platform-defined set of natures to fall back to in
the first place.

## 6. Placements: putting tokens on the map

A standalone map can declare starting positions for tokens:

```json
"placements": [
  { "token_id": "tok-fighter", "actor_id": "act-fighter", "x": 2, "y": 1 }
]
```

`x`/`y` are the placement's own square, addressed as plain numbers here (not
a `"x,y"` string — placements are a list, not a keyed map, so there is no
key to build). A placement's square must not be a `wall`, and must not be a
closed `door` — a token can never start somewhere it could not otherwise
stand.

## 7. Doors: folded state, not part of the tile

A door's tile name (`wood-door`, or a custom door tile from a pack) never
changes. Whether it is currently open is tracked as a separate, mutable fact
— the same way a token's position is separate from the geometry it moves
across. This matters for two reasons:

1. **Terrain is immutable.** A map's `tiles` never change once loaded. If
   "open" were part of the tile name, opening a door would mean *rewriting
   the map*, which nothing in this format (or the platform underneath it)
   is built to do.
2. **A door needs two pictures, not two tiles.** A pack's door entry (§5.1)
   supplies `file_closed` and `file_open` — the renderer picks between them
   based on the door's current folded state, while the tile itself stays
   exactly one thing throughout.

## 8. A pack: the art a map draws with

A **pack** is a `pack.json` manifest sitting beside the image files it
names, in a directory called `tiles/`:

- for a standalone map: `maps/<map-id>/tiles/pack.json`
- for a map embedded in an adventure: `adventures/<adventure-id>/tiles/pack.json`

Both are served the same way over HTTP, alongside the images — a pack is
content, never something written into the campaign's event log, so nothing
about it is frozen the way the wire contract is.

```json
{
  "id": "cellar-basics",
  "name": "Cellar Basics",
  "cell_px": 64,
  "tiles": [
    {
      "name": "masonry-1",
      "kind": "wall",
      "material": "stone",
      "file": "masonry_1.png",
      "desc": "coursed stone blockwork, the standard wall face for the cellar pack"
    },
    {
      "name": "cellar-door",
      "kind": "door",
      "material": "wood",
      "file_closed": "cellar_door_closed.png",
      "file_open": "cellar_door_open.png",
      "desc": "a banded wooden door"
    }
  ],
  "objects": [
    {
      "name": "crate-wood",
      "file": "crate_wood.png",
      "desc": "a stacked wooden shipping crate — good cover, or just clutter"
    }
  ]
}
```

| field | meaning |
|---|---|
| `id` | the pack's own identifier — this is what a map's top-level `"pack"` field names |
| `name` | a display name for the pack |
| `cell_px` | the pixel size each image is drawn at (images should be square, this size) |
| `tiles` | an array of named tile pictures — see below |
| `objects` | an array of named object pictures — same shape as `tiles`, in a separate list |

Each entry in `tiles` or `objects` (they share one shape):

| field | meaning |
|---|---|
| `name` | the identifier a map's `overrides` value, or an object's `art` field, refers to. Must be unique within its own array. |
| `kind`, `material` | **advisory only.** These describe what the picture *looks like it is*, for a human or an LLM choosing a tile deliberately. They carry **no authority** — see §1. A tile whose declared `kind` disagrees with the base tile it overrides produces a warning, not a refusal. Objects generally leave these blank; there is nothing for an object's picture to disagree with. |
| `file` | the image filename, for a tile or object with exactly one picture |
| `file_open`, `file_closed` | for a **door** tile only: two pictures instead of one, selected by the door's current folded state (§6). A door tile has these instead of `file`, never both. |
| `desc` | a free-text description — see below |

### Why `desc` exists

`desc` is not decoration. It is what lets a reader — human or LLM — choose
*which* tile or object to reach for without opening every image file and
looking. A pack with useful descriptions is a pack an LLM game master can
author *with*, picking `"cracked-flagstone-2"` over `"flagstone-1"` because
the description says one is chipped and stained and the scene calls for
that. A pack with empty or generic descriptions defeats the entire reason
packs carry metadata at all (see the design rationale, §1.5: *"An LLM handed
the format document and a pack manifest, with no other help, authors a map
that loads and plays."* — that promise depends on the manifest actually
telling the reader something).

## 9. A map file, top to bottom

```json
{
  "id": "shrine",
  "name": "Obsidian Shrine",
  "grid_width": 3,
  "grid_height": 3,
  "pack": "mossy-keep",

  "tiles": {
    "0,0": "stone-wall", "1,0": "stone-wall", "2,0": "stone-wall",
    "0,1": "wood",       "1,1": "wood",       "2,1": "wood",
    "0,2": "stone-wall", "1,2": "stone-wall", "2,2": "stone-wall"
  },

  "overrides": {
    "1,1": "planks-split-3"
  },

  "objects": [
    { "id": "boulder-1", "kind": "boulder", "at": [0, 1], "size": [1, 1], "rot": 0,
      "blocks_sight": true, "blocks_move": true, "art": "boulder-mossy-2" }
  ],

  "placements": [
    { "token_id": "tok-fighter", "actor_id": "act-fighter", "x": 2, "y": 1 }
  ]
}
```

| field | meaning |
|---|---|
| `id` | the map's own identifier — also becomes the scene's id when the map is loaded into a campaign |
| `name` | a display name |
| `grid_width`, `grid_height` | the grid's size in squares |
| `pack` | the id of the pack `overrides` and `objects[].art` resolve against. May be omitted (or empty) for a map that uses only standard tiles and no objects with art. |
| `tiles` | see §1, §3 |
| `overrides` | see §1, §4 |
| `objects` | see §5 |
| `placements` | see §6 |

Beside `shrine.json`'s directory sits its pack:
`maps/shrine/tiles/pack.json` (§8), and the images it names.

## 10. What gets refused, and why

A map is validated fully before it is ever served to a table — never at the
table. In order, roughly:

1. `grid_width` and `grid_height` must each be at least `1`.
2. If `tiles` is non-empty, **every** square in the grid must have an entry
   (§1) — no missing squares, and no extra entries naming a square outside
   the grid.
3. Every `tiles` value must be a known standard tile name (§3) — a typo, or
   a name that does not exist, is refused with the offending square and
   name named directly.
4. Every `overrides` key must name a square inside the grid. `overrides`
   with a non-empty `tiles` needs no further check here; a non-empty
   `overrides` against an **empty** `tiles` is refused outright — there is
   no nature for the art to attach to.
5. Every object's full **footprint** (not just its anchor square) must lie
   inside the grid, and its `size` must be at least `[1, 1]`.
6. Every `placements` entry must name a square inside the grid, and that
   square must not currently be a wall or a closed door.
7. (Once a pack is involved) every `overrides` value and every `objects[].art`
   must actually name something the pack declares.

Every refusal names the offending file, field, and (where relevant) the
exact square — so a fix is a matter of reading the message, not guessing.

## 11. On per-square explicitness

This format asks every square to name its own tile, with no shorthand.
That is a deliberate trade: it makes a map fully explicit and diffable
square by square, at the cost of a large map being a long file that a human
cannot eyeball the shape of by reading the raw JSON.

Tools exist that organise map authoring differently — assign a tileset to a
whole *room* or *section*, and let the tool fan that out to individual
squares, with per-square painting only as an override on top. That
organisation is a reasonable and useful one. It belongs in an **editor** —
something that reads author intent and *produces* a map in this format —
rather than in the format itself. This document describes the format a
loader reads, not a tool for writing it; keeping "how you'd like to author
this" separate from "what gets stored and loaded" is what lets either one
change without the other having to.

---

**The one rule everything above exists to serve:** art never decides
nature. If you remember nothing else from this document, remember that a
wall drawn as floorboards is still a wall.
