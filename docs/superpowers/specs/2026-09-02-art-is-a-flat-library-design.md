# Art is a library, not a claim

**Sub-project 16.** Packs leave the platform. Art moves to one flat `art/`
directory where the filename is the identity, any map may use any art, and the
filesystem is the only uniqueness rule there is.

**Prerequisite: sub-project 15**, which made a campaign a directory and gave
maps lookup on demand. This does the same for art and deletes the pack.

---

## 1. Why this, and why now

**A pack is an access boundary, and it should never have been one.**

`mapdef.Map.Pack` is a single string. Every value in `overrides` resolves
against that one pack, and `objects[].art` does too. So a map may use art from
exactly one pack, and art is reachable only by a map that has claimed the pack
containing it. Art identity is a property of the claim rather than of the art.

The direct cost is duplication: a map that wants a masonry wall and a forest
tree, where those live in different packs, cannot have both. One must be copied
into the other's pack.

**The indirect cost is worse, and it is what makes this urgent.** The path of
least resistance under a one-pack rule is a bespoke pack per map, holding
whatever that map happens to need. That is not a hypothetical — it is what the
constraint rewards. Every shared tile then exists once per map that uses it,
and the pack has stopped being a distribution unit and become a per-map bundle.

**Patrik's ruling, 2026-09-02:** *"each 'art' is unique and should be able to be
used by any map"*, and on subfolders: *"we do not allow underlying folders. You
want a different masonry_1, then you have to call it something else. very
simple."*

**Why now.** Sub-project 15 left the pack machinery in a state that is not worth
repairing. Its whole-branch review found a boot-order defect — `composeServer`
gates the pack load on `maps/` existing, so a campaign with art and no map yet
boots with no art at all, and then blames the operator for installing it late —
plus two comments that describe the pack system incorrectly. Those were
deliberately left unfixed at the merge of 15, because this sub-project deletes
the code they are in. Repairing it first would be work thrown away.

---

## 2. Non-goals

**An art editor.** Art arrives as files, the same way maps do. Making art is
outside the platform, exactly as ADR-era ruling 2026-09-01 put map authoring
outside it.

**Subfolders under `art/`, in any form.** Ruled out explicitly. They are what
re-creates namespacing, and namespacing is what re-creates the pack.

**A global art registry shared between campaigns.** `art/` belongs to its
campaign, like `maps/`.

**Changing what a tile MEANS.** The built-in vocabulary — the `kind` and
`material` a square carries, which sight and movement reason about — is
untouched. Only the picture layer changes.

**Deduplicating image bytes.** If two campaigns hold the same PNG, they hold it
twice. That is a packaging concern and not this design's problem.

---

## 3. The model

### 3.1 One flat directory

```
campaign/
  campaign.db
  maps/
    cellar.json
  art/
    masonry-1.png
    masonry-1.json
    earth-1.png
    earth-1.json
    cellar-door.json
    cellar-door-open.png
    cellar-door-closed.png
    pillar-stone.png
```

`art/` has no subdirectories. A directory found inside it is an error, named as
such, rather than being walked or ignored — silence there would let a pack-shaped
tree sit unused and look installed.

### 3.2 The filename is the identity

An art piece's id is its filename stem. `masonry-1.png` is `masonry-1`, and a
map names it as `masonry-1`. Nothing declares an id anywhere, so nothing can
disagree with the filename.

This is the same rule maps already carry — `maps/<id>.json`, filename equals
declared id, disagreement refused — with the declaration removed entirely
rather than merely checked. Sub-project 15 needed the check because a map file
has an `id` field for other reasons; art has no such need, so it gets the
stronger form.

Stems are kebab-case, and the picture and its sidecar share a stem.

### 3.3 Uniqueness is the filesystem's job

Two art pieces cannot share a name because two files cannot share a name. There
is no duplicate check to write, no collision prompt to design, and no registry
to keep consistent.

**This is the whole reason subfolders are refused.** `art/a/masonry-1.png` and
`art/b/masonry-1.png` coexist happily at the OS level, and the moment they can,
the platform must decide which one `masonry-1` means — which is a namespace,
which is a pack. Flatness is what makes the filesystem sufficient.

Replacing art is therefore a file write: drop a better `masonry-1.png` over the
old one and every map using it is restyled. Being asked first is `cp -i`.

### 3.4 The sidecar carries what a picture cannot

A picture does not know it is a wall. Sight and movement need `kind`, and a door
needs two pictures rather than one.

```json
// art/masonry-1.json
{ "format_version": 1, "kind": "wall", "material": "stone" }

// art/cellar-door.json
{ "format_version": 1, "kind": "door", "material": "wood",
  "closed": "cellar-door-closed.png", "open": "cellar-door-open.png" }
```

`format_version` is per art piece, on its own clock, for the reason packs had
their own: art is authored and distributed independently of the maps that use
it, and a piece written against a later format must say so itself.

**A sidecar is REQUIRED for tile art and OPTIONAL for object art.** Tile art
asserts something the engine acts on, so it must be declared. Object art is a
picture the map has already described — the map's own `blocks_sight`,
`blocks_move`, `kind`, `size` and `rot` say everything the engine needs — so a
bare `pillar-stone.png` with no sidecar is complete and usable.

That asymmetry is deliberate and it is the improvisation case: dropping a PNG
into `art/` and referencing it as furniture must be a one-step act. Requiring a
sidecar for it would put a JSON file between the DM and a piece of scenery for
no gain.

A sidecar with no matching picture is an error. A picture with no sidecar is
object art.

**When a picture with no sidecar is named as TILE art, that square degrades and
warns — it does not refuse.** Amended 2026-09-03 on Patrik's ruling, after Task
3's review measured the cost of the stricter reading: because `composeServer`
turns any boot-walk error into a refusal to start, a single sidecar-less PNG
named on a tile took the whole campaign down for everyone. That is the mistake
this very section teaches — §3.4 celebrates dropping a PNG in and using it as
furniture, and naming one on a tile is the natural next move — and §4's own
posture says a campaign that will not load beats one that loads slightly plain
exactly backwards. The warning must be its OWN sentence, not the not-installed
one, because the file is sitting right there: *"has a picture but no sidecar;
drawing it plain — write `art/<id>.json`"*. This also removes an asymmetry: the
mirror half-install, a sidecar with no picture, already degraded.

### 3.5 A map names art directly

```json
{
  "format_version": 1,
  "id": "cellar",
  "name": "The Sunken Cellar",
  "grid_width": 10, "grid_height": 9,

  "tiles":     { "0,1": "stone-wall", "1,1": "earth",   "5,4": "wood-door" },
  "overrides": { "0,1": "masonry-1",  "1,1": "earth-1", "5,4": "cellar-door" },

  "objects": [
    {"id": "pillar-west-1", "kind": "pillar", "at": [2,2], "size": [1,1],
     "rot": 0, "blocks_sight": true, "blocks_move": true, "art": "pillar-stone"}
  ]
}
```

There is no `pack` field. The two layers that already existed are unchanged:
`tiles` says what a square IS, in the built-in vocabulary, needing no art at
all; `overrides` says which picture to draw on it.

### 3.6 Art resolves on demand

Art is read when a map is loaded, not once at boot. A piece installed while the
server is running is found; a piece overwritten is picked up on the next
`load_map`. Nothing is cached across a load, and no operation asks the operator
to restart.

This is the same ruling sub-project 15 made for maps, applied to the half that
did not get it. The boot-order defect of §1 cannot recur, because there is no
boot-time art load to order.

---

## 4. Unresolvable art degrades; it does not refuse

**A missing or unreadable art reference drops that ONE square (or object) to the
built-in vocabulary. The map loads.**

**Art that cannot be READ degrades; art that declares a FORMAT THIS SERVER DOES
NOT UNDERSTAND refuses** (Patrik, 2026-09-04). These share one code path today
and must be split.

A corrupt sidecar — a missing brace, a truncated copy — degrades that square
with its own sentence naming the piece and the cause. Until this ruling it
refused the map, and because `composeServer` turns any map-load error into a
refusal to start, **one bad file stopped the server booting** while every other
map sat there fine; measured 2026-09-03, exit status 1. That is the same shape
ruled against three times already — the sidecar-less PNG, the unopenable art
root, and boot-time `Validate` — and it survived only because nobody re-asked
after the warning channel existed to carry the information a refusal used to
carry.

`format_version` is the opposite case and keeps its refusal. It does not mean
the file is broken; it means the CONTENT IS NEWER THAN THE SERVER. Degrading it
would turn a whole v2 art set into hundreds of plain squares and a wall of
warnings, which reads as "my art is broken" when the true diagnosis is "this
server is too old". One refusal naming the version says that; ninety warnings
do not.

**An art DIRECTORY that cannot be opened is strict at boot and lenient at
request time** (Patrik, 2026-09-03). At boot the operator is at a terminal and
can act on a filesystem path, so the server refuses to start and names the
directory. At request time a DM cannot act on that path from a browser, and a
campaign that worked five minutes ago should not stop working — so an unopenable
art root is treated exactly like an absent one: the map loads, art draws plain,
one warning. Before this ruling both readers got the same answer and one of them
could not use it.

Today a missing pack refuses the whole map. That was always wrong given the
layering: `tiles` already describes every square without art, and a map with no
`overrides` at all is legal and renders. The information needed to draw a
plainer version of the map is present by construction.

So a map referencing three art pieces, one of which is not installed, loads and
plays. The square renders from its `kind` and `material`. The DM is told which
references did not resolve, once, as a warning on the load — not an error, and
not silence.

**Once means once per art NAME, not once per square.** Measured 2026-09-03 on
the shipped `cellar.json`: the per-square form produced 96 warnings — 90
squares over four art names, plus six objects over four more — and the client
joins them into a single toast. Eight facts buried in 96 near-identical
sentences is a channel a DM stops reading, which is the
silence this section exists to prevent, arrived at from the other direction.

***Amended 2026-09-06 — this rule was reversed and restored the same day, and
the round trip is the argument.*** *Patrik first ruled the opposite: "We should
report each square that uses the bad art", on the ground that a count sends a DM
to grep the map file for the half they can act on. Per-square naming was built,
and then withdrawn by the same person for a reason the payload arithmetic had
missed — ONE SQUARE CAN CARRY MANY THINGS. Objects are keyed by their anchor, so
four crates stacked on one square render that coordinate four times; a list that
repeats one place four times is not locality, it is noise wearing locality's
clothes. The count survives because it states magnitude without pretending to
state position.*

*A second reason surfaced in review and outlives the first: `adventure.Compile`
concatenates EVERY scene's warnings into one `CommandResult`. A per-square list
is bounded per scene by `MaxWireTiles` and not bounded at all per adventure, so a
bundle whose `art/` did not travel could outgrow that frame on load —
delivering no warning at the exact moment every one of them was true. Once per
art name is what bounds it.*

*The kind mismatch keeps its squares, capped. It collapses per distinct
SENTENCE rather than per art name — the sentence carries the square's kind, so
one art mismatched onto a wall and onto a floor is two lines, and bounded either
way:
its remedy is one of two opposite things — a deliberate illusory wall, or the
wrong art pasted onto a square — and only the square tells them apart.*

**That channel does not exist yet and this design adds it.** `mapdef.Compile`
and `mapdef.Resolve` already return a `warnings` slice, and nothing carries it any
further: no
contract message has a warnings field, so today a warning dies in Go. The
`load_map` result gains a repeated string field for them. That is an additive
contract change (ADR-007) and the only one this sub-project makes.

Without it §4 is not a design but a silence: art would go missing and the map
would render plain with nobody told why, which is strictly worse than the
refusal being replaced.

**Object art that does not resolve leaves the object in place**, with its
blocking behaviour intact, drawn from its `kind`. An object is a thing in the
world before it is a picture, and dropping it because its picture is missing
would change what the room *is*.

**What this deliberately gives up:** a typo in an art name is now a warning
rather than a refusal. That is accepted. A campaign that will not load because
one tile is misspelled is worse at the table than one that loads slightly plain,
and the warning names every unresolved reference so the typo is visible.

---

## 5. Installing art

Copying files into `art/` is the primitive, and it is complete. Everything in
§3.3 follows from the filesystem.

`vtt art install <path>...` exists for convenience, not authority: it copies
files in, refuses a directory, warns before overwriting an existing stem, and
validates each sidecar so a malformed one is caught at install rather than at
the table. A DM who prefers `cp` is not doing anything wrong and loses only the
early validation.

**The loader is the backstop that cannot be bypassed**, and it is where the
rules actually bind: a subdirectory in `art/`, a sidecar with no picture, an
unreadable sidecar, an unsupported `format_version`.

**"Binds" means the FILE is named, not that anything is refused.** Amended
2026-09-03 and corrected twice on 2026-09-04, because the first amendment
repeated a sentence that five Go files had already retired: *"each broken piece
is then refused individually the moment a map names it."* That is false, and it
is false in a way no amount of care about wording would have fixed — **a
`Validate` finding does not predict what a map load does with it.** The symlink
arm spans two outcomes and the subdirectory arm two different routes to the
same one, because a load is decided by what `Lookup` finds at `<stem>.json` and
`<stem>.png`, not by which arm reported the entry. `art/pack-ish/` degrades as
ABSENT art and `art/masonry-1.png/` degrades as art that cannot be read — two
different sentences to the DM, out of one `Validate` arm. A relative in-root
symlink renders; a dangling symlinked sidecar degrades.

*The second correction of 2026-09-04 is the ruling in §4. Those last two
sentences read "`art/masonry-1.png/` refuses" and "a dangling symlinked sidecar
refuses" until it landed, and both went the other way with the class: neither
declares a `format_version`, so neither is the one thing that still refuses.
Nothing else in this section moved — a `Validate` finding still predicts
nothing.*

At boot every problem is reported and the server starts. `vtt art install`
refuses outright, because there the operator is holding the file. A subdirectory is the mildest
case of all: art ids must be plain filenames, so nothing inside `art/pack-ish/`
is reachable by any map — the tree is inert, and the boot warning exists to stop
a DM wondering why art they installed does nothing. Refusing to start over an
inert folder would take a campaign down for a mistake that cannot affect play.

---

## 6. What this deletes

- `packs/` and every `pack.json`
- `mapdef.Pack`, `LoadPack`, and the pack's `id`, `name`, `cell_px` fields
- `Map.Pack` and the map's `"pack"` field
- the cross-pack duplicate-id check, which flatness makes unrepresentable
- `ErrPackNotLoaded` and the *"Packs are read once, at startup… until this
  server restarts"* message
- the boot-time pack walk in `cmd/vtt/maps.go`, and the `os.Stat(maps)` guard in
  `cmd/vtt/serve_compose.go` that gated it — the §1 defect goes with them
- `Server.packs`, `Server.packFS`, `WithPackFiles`

**Kept, unchanged:** the built-in tile vocabulary; `internal/sight`;
`engine.Apply` as the only fold (CLAUDE.md rule 4); the map format's `tiles`,
`objects`, `placements` and `format_version`; `load_map` and its on-demand
lookup; the campaign-as-directory model.

**Rehomed:** `cell_px` — how many pixels one grid square of art occupies. It was
a pack-level field, and it is NOT only a generator concern:
`internal/gateway/metadata.go` serves it to the client as `pack.cellPx` on every
map's metadata, so the renderer reads it. With no pack to hold it, it becomes a
campaign-level setting in a new `campaign/campaign.json`:

```json
{ "format_version": 1, "cell_px": 64 }
```

The file is optional and `cell_px` defaults to 64 when it is absent, so an
existing campaign keeps working untouched and a new one need not create it. The
map metadata endpoint drops its `pack` object and reports `cellPx` directly.

Making it campaign-level rather than per-art is deliberate: a grid is uniform,
and art pieces at differing native resolutions on the same board is a rendering
problem, not a capability. One number per campaign says that plainly.

**Amended 2026-09-05 on Patrik's ruling: `cell_px` is a property of the MAP, and
the campaign's value is the DEFAULT a map inherits by declaring none.** Taken
from how MapTool solves the same problem — grid size lives on the Zone, not the
campaign (`Grid.size`, clamped `MIN_GRID_SIZE` 9 to `MAX_GRID_SIZE` 350). The
paragraph above is right about the reason and wrong about the scope: a grid is
uniform across ONE MAP, and grid size is exactly what differs between an art set
drawn at 64 and one drawn at 128 — so the first time a DM installs both, a
campaign-wide number is wrong for one of them. A map file may now declare
`"cell_px"`, bounded 8..1024 and **refused rather than clamped** outside that
(0 and 100000 are not smaller and larger squares, they are a file that cannot
mean what it says); `GET /api/maps` reports each entry's resolved value beside
the campaign default.

**The bounds differ from MapTool's 9..350 deliberately.** The floor is
essentially theirs — the same judgement that below about a dozen pixels a square
carries no tile detail — and 8 is only the power of two every export dialog
offers. The ceiling is the real divergence: theirs is a RENDERING bound, because
MapTool draws at `gridSize * zoom` every frame, and nothing here draws at native
size, so 1024 exists to catch a typo rather than to police resolution. If this
client ever gains zoom, that reasoning expires and their 350 stops being a
divergence and starts being data. `internal/mapdef`'s `MinCellPx` carries the
full argument. This is additive to the map format and needs no contract
change — `cell_px` never crosses the protobuf wire.

**Also amended 2026-09-05: what `cell_px` is FOR.** It has one reader,
`vtt art install`, which warns when an installed picture is not a whole multiple
of the campaign's default — with the operator holding the file, which is the only
place anyone can act on it. It is deliberately NOT read by the renderer:
`client/src/view/spectator.ts`'s `CELL` is a SCREEN size and `cell_px` a SOURCE
size, and `drawImage` scales one to the other whatever they are. The board's own
size is a separate question, answered by the container it sits in — see the
amendment to §9 below.

**Renamed:** `GET /api/packs/{pack}/{file}` becomes `GET /api/art/{file}`. The
route serves one flat directory, so it takes one segment. It must remain
symlink-safe: `os.OpenRoot` over `art/`, not `os.DirFS`, for the reason the pack
loader already records.

---

## 7. Migration

Campaigns in this repo are the only ones that exist, so migration is a rewrite
of `campaigns/example/` and any scenario fixture, not a compatibility layer.

**There is no compatibility layer**, and none is added later. A map carrying a
`"pack"` field is refused, by `mapdef.Load`'s strict decoding — `json: unknown
field "pack"`, and the same for the field under any rename. Silently ignoring it
would load a map whose art references were written against a namespace that no
longer exists, and draw the wrong thing.
`TestAMapDeclaringAPackOrAPackageIsStillRefused` (`internal/mapdef/load_test.go`)
is what keeps the refusal honest.

**No migration route accompanies that refusal**, and none is written later.
Nothing has ever shipped and every campaign that has ever existed is in this
repository, so a message written to walk somebody through the change has no
reader. A human migrating a map by hand is served by `docs/map-format.md`, which
carries the instructions and is where a reader looks anyway.

`contract/RELEASED` does not exist, so ADR-007 reports rather than enforces
(CLAUDE.md rule 3). This design is not additive, and that is only permissible
because nothing has been released. It must land before that file does.

---

## 8. Testing

**A map loads with every art reference unresolvable**, and every square renders
from its `kind`. This is §4's whole claim and it is the keystone: it fails if
anything in the load path still treats art as required.

**Art installed after boot is found without a restart**, and art overwritten in
place changes what a reload draws. Both are exercised against a running server,
not a constructed `Server` value — the §1 defect existed precisely because its
test never went through `composeServer`.

**A subdirectory under `art/` is refused**, with the directory named.

**Object art with no sidecar loads**, and tile art with no sidecar **degrades
that square with its own warning** — not the not-installed one, because the file
is sitting right there. These are the two halves of §3.4's asymmetry and neither
is safe to assume.

**A campaign holding a corrupt sidecar BOOTS**, and that is a boot-level test
rather than a `Resolve` one: the whole cost of the refusal it replaces was that
`composeServer` turned it into `exit status 1` for every map in the campaign, and
a unit test on one square cannot see that. Its companion is the other half of the
split — **a sidecar declaring a `format_version` LATER than this server's still
refuses, naming both versions**, and refuses at that same boot. *(Added
2026-09-04 with §4's ruling. **Amended 2026-09-07**: this said "a `format_version`
this server does not understand", which reads as `!=`. The guard is
`*version > FormatVersion` in `artlib`'s `pieceFromSidecar` (reached from
`Lookup` via `lookupIn`) and has been since `035248e`
narrowed it; a version BELOW this server's degrades like any other broken
sidecar. The direction is written out here because this section is the one a
task's tests get written from, and `artlib.ErrFormatVersion`'s own doc records
what the `!=` reading cost: a whole campaign's boot refused, for every seat, over
a mistyped digit.)*

*(Amended 2026-09-03 with §3.4 and exit criterion 6. This paragraph said
"is refused" until Task 3's re-review caught the contradiction: §8 is the list
Tasks 8 and 9 write their tests from, so a reader working forward from a stale
§8 writes the wrong test and then "fixes" working code to match it.)*

**A map declaring `"pack"` is refused** — by strict decoding, naming the field
as `unknown field "pack"`.

**Two art pieces cannot collide**, which is not a test of platform code but of
the claim in §3.3. It is asserted by a test that tries to construct the
collision and finds the filesystem prevents it, so that the reasoning is
recorded where a future reader will look.

Per CLAUDE.md rule 1 the deletion tests come first and run RED, and any
after-the-fact assertion carries fault-injection proof.

---

### 6.1 The board follows the window

*(Added 2026-09-05 with §6's amendment, on Patrik's ruling.)*

`cell_px` was mistaken for the answer to "the board is a fixed 640x480". It is
not: `client/src/view/spectator.ts` hard-coded `PANE_W`/`PANE_H`, and that was
the whole of it. The renderer's own architecture already anticipated the fix —
`planScene`, `planGrid` and `planFog` all take `viewW`/`viewH` as arguments, and
only the call site fed them literals.

**The board's size follows its CONTAINER: neither the scene nor a constant.**
Both of the other answers have been shipped and both were wrong. Sized by the
SCENE, the board was `gridWidth * CELL` px tall — 1408 for a 32x32 map — so the
page grew with the map and the controls sat below every laptop fold (backlog
T1/#19). Sized by a CONSTANT, a 200x200 outdoor map and a 10x10 room laid out
identically, which was the point, and a 27-inch display drew the same postage
stamp as a laptop, which was not.

The stylesheet gives the board `width: 100%` and a height clamped against the
VIEWPORT, so it can never grow the page again; the renderer measures what that
produced, sizes the canvas backing store by `devicePixelRatio` so it is not soft
on a high-DPI display, and re-plans on resize. A deterministic fallback keeps
every existing render test asserting the same geometry it always did.

**Zoom and pan are still absent and are not this.** MapTool multiplies grid size
by a zoom scale every frame; this client has a fit-once camera and no way to move
it. That is a real gap and its own sub-project.

## 9. What could go wrong

**A flat directory grows large.** Several hundred files in one folder is
ordinary for a filesystem and awkward for a human browsing it. Accepted
deliberately: the alternative is subfolders, and subfolders are the pack.
Naming discipline (`cellar-masonry-1` rather than `masonry-1`) is the answer,
and it is the DM's to apply.

**A name collision between two downloaded art sets** must be resolved by
renaming one, and by hand-fixing any map that referenced it. This is the
accepted cost of a flat namespace and the reason §3.3 states it plainly.

**Degrading instead of refusing hides a typo.** §4 accepts this and mitigates it
with a warning that names every unresolved reference.

**On-demand art resolution costs a directory read per load.** A map load already
reads a file and folds a batch; a few small JSON reads beside it are not the
expensive part.

---

### DONE 2026-09-09 for the art and map paths; the rest is still carried

***Closed for internal/artlib and internal/mapdef.*** *Every interpolation
reaching a `CommandResult` from those two packages now passes through
`artlib.Clip` or `artlib.BoundErr`. Measured: a 20 KB sidecar `kind` produced
20,191 bytes of warnings and now 234; an unknown map field 20,043 and now 264;
an oversized tile name 20,128 and now 171, still carrying "(not in the standard
vocabulary…)", which a whole-message bound had truncated away. Two boundary
tests state it as a frame a client can read —* `TestBrokenArtCannotPushAResultPastTheReadLimit`
*and* `TestABrokenBundleCannotPushALoadAdventurePastTheReadLimit` *— both red
first with* `websocket: message too big: read limited at 204801 bytes`.

***NOT closed, and this section claimed otherwise for one round.*** *The first
version of this note said "every interpolation that can reach a CommandResult".
Review found that false the same day. Still unbounded, and still carried:*
`internal/adventure`*'s collision refusals for an ACTOR id and a TOKEN id — the
other two arms of* `checkCollisions` *are bounded now, a scene id by*
`maxIDBytes` *and a note key by* `maxNoteKeyBytes`*,*
`mapdef.LoadInstalled`*'s use of a map's own declared id,* `internal/engine`*'s
terrain kind reaching a `move_token` refusal through* `internal/gateway`*, and*
`internal/rules`*' ability and resource names reaching a `use_ability` result.*

***Scene-id prefix: closed 2026-09-09.*** *`loadScenes` now refuses a scene id
over `maxIDBytes` (128). It is the MULTIPLYING one — not the last unbounded
string in a bundle, and the first version of this note said "the same door every
other author-controlled string already had", which review found false the same
day. Four were already bounded (`opening_narration`, note `key`, note `title`,
note `text`); the manifest's `id` and `name`, a scene's `name`, an actor's
`actor_id` and `name`, and a placement's `token_id` are still non-empty-only.
None of them multiplies BY WARNINGS, which is the multiplier that matters here
and why the id went first. Two of them do recur — an `actor_id` and a
`token_id`, once per placement in `TokenPlaced` — but that count is bounded by
the placements an author wrote, not by how many warnings a scene produces. The
adventure-format spec §4 now carries the same distinction, in the same words.*

*Two author-controlled strings are in NEITHER list and belong in the inventory:
a scene override's VALUE and an object's* `art`*. Neither is bounded at LOAD —*
`CheckOverridesInsideGrid` *validates the KEY and says outright that the value
is not inspected — and both reach warnings once per scene, so they multiply the
way a scene id did. Two things hold them instead, and it matters which:*
`artlib`*'s* `isArtID` *refuses an id over* `maxArtIDLen` *(250, NAME_MAX minus
the sidecar suffix) so an over-long value is ErrNotFound, and* `mapdef`*'s*
`name()` *=* `artlib.Clip(art, artlib.MaxFragment)` *clips what the warnings
render.*

***`artCannotBeUsed`: closed 2026-09-09.*** *It was the one warning in*
`resolve.go` *rendering* `artlib`*'s error instead of composing its own, and it
clipped neither half — its* `art` *went in raw at both call sites, and the error
carried the id a SECOND time because* `artlib` *stamped* `artlib: art/<id>.json`
*unclipped as well. Measured on the tile arm before the fix: a 100-byte id gave
a 292-byte warning and a 250-byte id gave 592, growing by twice the excess.*

***The first version of this entry called it unbounded. It never was*** *—*
`isArtID` *capped it at 250 all along, and an id failing that check lands in a
different, already-clipped arm. Nor was it "six times its siblings", which the
first draft of this entry also said. It is a RANGE, because* `artlib` *composes
eight messages behind this one arm: it ran 592..850 and runs 178..366 now,
against siblings of 84, 102, 105, 212, 433 and 523 at the same 250-byte id. So
1.62x the largest sibling at worst. Two corrections were needed to get there —
"six times" was invented, and the figure that replaced it quoted the mildest
shape as though it were the arm.*

***Still open, and now the largest:*** *the two case-mismatch sentences (523
tile, 433 object) interpolate* `mismatch.Real` *and* `onDisk` *whole. Those are
real directory entries, so they are operator-controlled and NAME_MAX-bounded
rather than written by a campaign file — a smaller worry than an override value,
which is why they are listed rather than fixed. They were missing from this
inventory because* `artCannotBeUsed`*'s own doc counted eight sentences when
there are ten, omitting exactly these two. Nothing pins them: the test below
guards the closed item, not this one.*

***What holds the closed one.***
*`TestACannotBeUsedWarningStopsGrowingWithTheArtName` asserts that two
over-long ids differing by 150 bytes produce warnings of EQUAL length — an
invariant rather than a byte count, so it survives a rewording — and it fails
if either copy is left unclipped. It runs one shape per* `artlib` *message that
can reach the arm, because review found seven still shipping 228 bytes of the
id after the first fix, and ten of the thirteen clips with no test observing
them at all.* `TestTheRefusalPathBoundsTheArtNameToo` *covers the one message
mapdef never renders:* `unsupportedFormat` *travels out as an error, and its
own clip is the only bound there is.*

*The prefix was the multiplier rather than a single long value:*
`adventure.Compile` *stamps* `scene %q:` *onto EVERY warning a scene produces
and they do not collapse across scenes, so one oversized id was paid once per
warning per scene. Bounding at the loader is what makes that arithmetic finite,
and it binds the* `%w` *error path in* `Compile` *as well — the same id goes
into* `adventure: compile: scene %q` *a few lines above the warning append.*

*`TestLoadAcceptsValuesExactlyOnEveryLimit` holds the direction — an id of
exactly the limit is legal — because a `>=` here would tell an author their id
"must be at most 128 bytes" while reporting "got 128".*

***The mistake worth keeping.*** *Two clips on one path make both mutants
unkillable — that part is true. But `artlib` bounding an id inside ITS error and
`mapdef` bounding the same id inside ITS warning are two PATHS: mapdef composes
its own sentence and never renders artlib's. Reading the surviving mutants as
redundancy deleted the only bound on the warning side, and an override value of
any length reached a client — 20,041 bytes, socket closed. A surviving mutant
means no test drives that path. Look for the missing test before concluding the
guard is spare.*

***And a fixture that proved the wrong thing.*** *The first boundary fixture wrote
`open` and `closed` beside the huge `kind`, which routed it into artlib's
already-bounded door error. Without those two fields the kind travels out as
`Piece.Kind` and mapdef renders it whole — 234 bytes versus 20,057 from the same
value. The fixture, not the assertion, decided what the test could see.*

*The original text follows, unchanged, because its reasoning is what made the
work findable.*

### Carried forward (closed): author-controlled bytes are not bounded on the way to a client

`[anchor:author-controlled-bytes-unbounded]`

*Added 2026-09-07 at the merge gate, from the whole-branch review. Patrik: this
one is important and gets done — it is recorded here rather than in a review
transcript so it survives the branch.*

`artlib.clip` bounds a fragment of author-controlled sidecar text to 40
characters, and it is called from **one** place: the `format_version` arm. Every
other interpolation of sidecar bytes into a message passes them through whole —
the strict-decode `%w`, the door-name `%q`, and the kind-mismatch `%q` on
`sc.Kind` — and those messages become `CommandResult.warnings` entries.
`mapdef`'s own `decodeStrict` `%w` and `fieldErr` callers do the same onto
`CommandResult.error`; that half predates this sub-project.

**The scope is EVERY such interpolation, deliberately not a number.** Naming a
count invites bounding that many and stopping: the three artlib sites above are
the ones this review named, `internal/mapdef/resolve.go` adds six more and
`load.go`'s `decodeStrict` another, and `artCannotBeUsed`'s own doc comment
already says "THAT IS TWO OF THE EIGHT SENTENCES THE ART PATH CAN PRODUCE". The
invariant is what to work to: no author-controlled bytes reach a
`CommandResult` unbounded.

**Why it matters, and why the bar is lower than it looks.** Warnings collapse per
art NAME within a scene, but `adventure.Compile` scene-QUALIFIES them, so they do
not collapse across scenes. The total is what has to cross a read limit, not any
single value: twenty scenes x five bad art names x a couple of KB of interpolated
sidecar text gets there. Past `internal/harness`'s `readLimit`, `handleLoadMap`
has already committed and broadcast the scene when the frame is built — so every
other seat's board changes, and the ISSUER's socket closes with `message too
big`, reconnects into a campaign that silently changed, and is never told why.
Go clients only: the MCP agent seat and `cmd/vtt`. A browser has no read limit
and renders the block instead, which §4's toast bound now contains.

**Not remotely triggerable.** `load_map` is gated to DM and agent by
`internal/gateway/authz.go`, and no route writes map or art files. This is a
broken-content and third-party-bundle footgun, not a denial of service. It is
carried rather than fixed at this gate because routing seven interpolation sites
through a shared bound deserves its own tests, not a patch written at a merge.

**What done looks like:** every author-controlled interpolation reaching a
`CommandResult` passes through one bound; a test drives an oversized sidecar
value through `load_map` AND through `load_adventure` (the aggregating path) and
asserts the frame stays inside `readLimit`; and `clip`'s doc comment loses the
sentence "no one has re-surveyed the tree" (mid-comment, not its last line),
which is true today and is
exactly what stops being true when this is done. That sentence is the marker for
this item — it was left deliberately, and it should not outlive the work.

---

## 10. Exit criteria

1. `art/` is flat, the filename is the id, and no art declares an id anywhere.
2. Any map may reference any installed art, with no map declaring a container.
3. A map whose art is entirely missing loads, plays, and renders from the
   built-in vocabulary, warning once per unresolved reference.
4. Art installed or overwritten while the server runs takes effect on the next
   `load_map`, with no restart and no message suggesting one.
5. **Every malformed thing is NAMED at boot**, and some of them RENDER anyway.
   Those are the two claims that survive every arm, and they are deliberately
   the only ones stated: three earlier drafts of this criterion tried to
   enumerate outcomes by kind and each was measurably wrong, because a
   `Validate` finding does not predict a load (see §5). The rendering cases are
   a resolvable in-root symlink, and a wrong-cased filename on a
   case-insensitive filesystem — for those, the boot report is the only notice
   anyone gets. **Exactly one malformed thing stops the boot on its own**: an
   art root that cannot be opened. **Exactly one stops a MAP**: a sidecar
   declaring a `format_version` NUMBER **later than this server's** — and
   that one stops the boot as well, whenever a committed map names it. A
   `format_version` that is absent, that holds something which is not a version
   number at all, **or that is below this server's**, is a broken file and
   degrades with them. *(The direction was added 2026-09-07; this criterion
   previously said "does not understand", which reads as `!=` and is not what
   `pieceFromSidecar`'s `*version > FormatVersion` does — see §8.)* *(Both sentences
   were added 2026-09-04 with §4's ruling; before it, every unreadable sidecar
   did both.)*
6. Object art needs no sidecar. Tile art without one **degrades that square
   with its own warning** (amended 2026-09-03 — see §3.4; the original criterion
   said "is refused", which in practice refused the whole server).
7. No `pack` identifier survives in platform code, enforced by a gate in the
   shape of `check-no-create-scene.py`.
8. `task check` green, both mutation gates included.
9. A cold reader given only this spec can install a piece of art, reference it
   from a map, and say what happens when it is missing.
