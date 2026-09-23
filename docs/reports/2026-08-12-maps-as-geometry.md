# Implementation report — maps as geometry (`internal/mapdef`)

Arc: `docs/superpowers/plans/2026-08-12-maps-as-geometry.md`
Spec: `docs/superpowers/specs/2026-08-12-maps-as-geometry-design.md`
Range: `5948b1b..f3d5c3d`, merged as `f94c529` (2026-08-18), epilogue `c40a450`
Report written 2026-09-19 against `main` at `48e3fe2`. Read-only; the tree was not modified (`git status --short` before and after: the same 17 sub-project-14 files).

---

## 1. Vad som byggdes

The arc turned a scene from four numbers into a described place. A map is a JSON file in which every square declares its **nature** — `wall`, `floor` or `door`, drawn from a closed vocabulary of eleven standard tile names — with art as a separate, purely cosmetic layer keyed by the same `"x,y"` square. `internal/mapdef` parses that file, validates it completely at load time, resolves each square into the facts the engine enforces plus the picture the renderer draws, and compiles the result into exactly one `SceneCreated` followed by one `TokenPlaced` per placement. Both file load paths — a standalone map and a scene embedded in an adventure — go through that one construction site, so a map cannot mean different things depending on how it was loaded. Around it the arc added `TileRef`/`SceneObject`/`DoorOpened`/`DoorClosed` to the contract, `engine.State.Blocked` plus folded door state, a gateway movement check that constrains players and leaves the DM free, a door-adjacency rule, HTTP routes for maps and pack art, a client renderer split into a pure planner and a thin canvas layer under a camera, and `docs/map-format.md` — the API document, which was the arc's actual deliverable. Twenty commits over five days.

---

## 2. Hur det fungerar i dag

Verified against the tree at `48e3fe2`. `go test ./internal/mapdef/` passes.

**The format and its loader.** `Load(path string) (*Map, error)` decodes strictly — `DisallowUnknownFields` — then runs the checks in a fixed order so the first error a broken file produces is the most useful one: format version, `cell_px`, grid sanity, then `CheckEverySquarePresent` → `CheckTilesInsideGrid` → `CheckTileNamesKnown` → `CheckOverridesInsideGrid` → `CheckOverridesRequireTiles` → `CheckObjectFootprints` → `CheckObjectArtDeclared` → `CheckPlacementsNotInWalls`. Placements run last because they are the only check that reads an already-validated tile name. Every error names the offending file and field. The `Check*` functions are exported and take a `FieldErrFunc`, which is the seam `internal/adventure/load.go` uses (lines 452–464) to apply the identical validation to an embedded scene without re-implementing any of it.

**The vocabulary.** `standard.go` is unchanged since the arc — 28 lines from `daa2b9c`, 15 from `a03e5fe`, nothing since. Eleven names, each mapping to a `kind` the engine reads and an opaque `material` the platform never interprets. `StandardTile` and `StandardTileNames` have the signatures the plan gave them.

**What later arcs changed.** Three arcs rewrote significant parts of this package after the merge:

- **art-is-a-flat-library (2026-09-02 → 09-10)** deleted the pack. `Pack`, `PackTile`, `LoadPack` and `Map.Pack` are gone; `load_test.go:287` bans the names. `Resolve`'s signature changed from the plan's `Resolve(m *Map, p *Pack, square string)` to `Resolve(m *Map, artDir, square string)`, and `Compile(m *Map, p *Pack)` to `Compile(m *Map, artDir string)`. Art now resolves by filename through `internal/artlib` against one flat directory. The rule the arc built — nature comes from `m.Tiles`, art supplies a picture and nothing else, a kind mismatch warns — survived the rewrite intact. What changed underneath it is that every art failure now **degrades** (the square keeps its nature and loses its picture, with a warning) instead of refusing, with exactly one exception: a sidecar declaring a `format_version` later than this server understands. A map file that still carries a top-level `"pack"` is now refused by name, and `TestAMapDeclaringAPackOrAPackageIsStillRefused` drives that through `Load` rather than trusting the sentence.
- **create-scene-leaves (2026-09-01)** added `installed.go` — a file that did not exist in this arc at all. `LoadInstalled(mapsDir, id, artDir)` is now the one function both boot (`cmd/vtt`'s `loadMapsDir`) and on-demand lookup (`internal/gateway`'s `mapByID`) go through, so the two cannot disagree about what a loadable map is. The same arc moved maps from `maps/<id>/map.json` into `campaigns/<c>/maps/<id>.json` and made the filename the id, which reversed one of this arc's own late corrections (see §4).
- **The visibility arc and sub-project 14** closed the merge commit's third known gap: `blocks_sight` is now read, at `internal/sight/sight.go:135`.

**What the arc built that is still load-bearing.** `MaxWireTiles = 3600` still caps a `SceneCreated`, still counts tiles rather than grid squares, and still disagrees with the client read limit in the direction the arc recorded (§4). `engine.State.Blocked`, `OpenDoors`, the two door `Apply` arms, `mayWorkDoor` in `internal/gateway/authz.go:278`, the `Blocked` call in `server.go:1284`, `camera.ts`, `scene-plan.ts` and `canvas.ts` are all present and in use. `BuildSceneCreated` is still the one place a map file's tile names become `TileRef`s, and `TestBothLoadPathsEmitIdenticalSceneEvents` (`internal/adventure/compile_test.go:199`) still pins the two file paths against each other.

---

## 3. Besluten och varför

**A hybrid grid, not polygons.** Walls, floors and doors are tiles; objects carry position, rotation and footprint. *Rejected:* polygons with wall segments (MapTool's VBL). *Reason:* authorability — an LLM emitting coordinate lists produces rooms that do not close, and painted floor tiles then align to nothing. The accepted cost is that a round tower is a staircase of squares.

**Two layers, both keyed by square, no implicit fallback.** *Rejected:* an ASCII grid plus a legend, which was designed and discarded. *Reason:* it compresses many squares into one character, which forces an override concept to express "this one differs" and makes the map two things that must be aligned by eye. The cost is real and was named as a risk: a 32×32 map is 1024 lines and you cannot see the room's shape by reading it. The risk was measured rather than assumed — exit criterion 6 tested it, it passed, and no ASCII shorthand was ever shipped (`grep -i ascii docs/map-format.md` returns nothing).

**Art has no authority.** Nature comes from `tiles`; an override supplies a picture. *Rejected:* an earlier draft made a kind mismatch a load error. *Reason for withdrawing it:* a wall that looks like a passage is an illusory wall — legitimate dungeon craft, and about to become a real feature one arc later. Refusing it would have forbidden a feature one arc away. The mismatch warns instead, and the plan required a fault injection proving the rule is enforced rather than merely documented.

**A door is one nature, not two.** `wood-door` is `kind: door`; whether it is open is folded state. *Rejected:* encoding openness in the tile name. *Reason:* that puts a mutable fact in an immutable declaration, and terrain is immutable. The consequence is that doors fold like everything else, so replay reconstructs door state for free.

**Doors start closed.** `Apply`'s `SceneCreated` arm initialises `OpenDoors` empty. *Reason:* a door whose state was never recorded is shut — failing closed. `Blocked` fails closed on an unknown scene for the same reason: refusing a move into a scene we cannot describe is recoverable, permitting one is not.

**The movement check lives in the gateway, not in `engine.Apply`.** *Rejected:* enforcing in the fold. *Reason:* `Apply` is the fold — by the time an event reaches it the move is already history, and history is not the place to say no. The command seam is where a request is still a request.

**Players are constrained; the DM and the agent are not.** *Reason:* staging a creature inside stone is a legitimate thing for a DM to do. The same split gives players a door-adjacency rule and gives the DM none.

**`kind` is spatial and closed; `material` is opaque.** *Rejected:* letting the platform mean anything by `water` or `stone`. *Reason:* CLAUDE.md rule 5 — no game-system vocabulary in platform code. `Blocked` is spatial only, deliberately: difficult terrain, flying and phasing are the ruleset's business. This is also why `material` was left unvalidated when `create_scene` was hardened — whitelisting it in platform code is exactly what the rule forbids.

**An object's `kind` is an open label and the collision with a tile's `kind` is deliberate.** *Reason:* the two fields answer different questions, and spec §3.4 warns a reader not to infer behaviour from an object's label. Structural effect comes only from `blocks_sight` and `blocks_move`. The line exists to stop `SceneObject` becoming a second, half-implemented entity system.

**One compiler for both file load paths.** *Rejected:* letting the adventure path build its own `SceneCreated`. *Reason:* a second resolver is what would let a map mean different things depending on how it was loaded. This is the decision that was mis-stated three times; see §4.

**Tiles became optional mid-arc** (Patrik's ruling, 2026-08-13). *Rejected:* making `tiles` mandatory. *Reason:* it broke the on-disk format for anything authored earlier, which a format meant to be written by third parties and by an LLM does not get to do. The invariant was not weakened — it was re-scoped to "complete **if declared**".

**`MaxWireTiles` counts tiles, not grid squares.** *Reason:* because tiles are optional, a large grid that declares no terrain costs nothing on the wire and must still load; `internal/rules/conformance` relies on that with a tile-less 100×100 scene. Sizing the cap on `GridW*GridH` would refuse maps that are free to send.

**The client renderer is split into a pure planner and a thin drawer.** *Reason:* happy-dom has no canvas implementation, so nothing touching a canvas context can be asserted — which is exactly how the `ArmakAsmeDM` defect shipped behind a passing DOM assertion. Every decision moved into `planScene`, which is pure and fully testable; `paint` is small enough to verify by reading.

---

## 4. Vad som visade sig fel

**The "one construction site" claim, wrong three times, and wrong right now.**
The plan's Task 3 originally said there is "literally one construction site", and the shipped doc comment on `BuildSceneCreated` repeated it. The plan carries its own correction:

> **CORRECTED 2026-08-18.** This step originally said "there is literally one construction site" […] Both were false: `internal/gateway/convert.go` builds a `SceneCreated` too, from a `CreateScene` COMMAND.

The comment was corrected again on 2026-09-02 (`0b9d6b7`), this time handing the reader the grep and stating the count:

> `grep -rn 'vttv1\.SceneCreated{' --include='*.go' . | grep -v _test.go | grep -v /gen/`
> Two live sites answer, and they have different inputs, which is why neither can be folded into the other

I ran that exact command. **Three sites answer**, not two: `internal/mapdef/compile.go:256`, `internal/gateway/project.go:523`, and `internal/eventgen/model.go:267`. The third arrived on 2026-09-12 (`773cd0ff`, `git blame -L 267,268`), and the block was edited again on 2026-09-16 (`53bb71f`) without the count being revisited. A comment written specifically to stop this claim going stale has gone stale for the third time, and it is the one comment in the package that tells a reader to check it.

**The wire-size numbers were a tile count wearing a byte limit's clothes.**
Task 4 shipped "45.5 KiB, about 45.5 bytes a tile" for a 32×32 and `events.proto` claimed a fully tiled scene "past about 60x60 exceeds the read limit". `4ee37b9` (2026-08-22) corrected both:

> The 60x60 came from MaxWireTiles = 3600, which is 60 squared. The sentence asserted a byte fact and quoted a TILE number, joined by an assumed constant bytes-per-tile rate. That rate does not exist.

`7b91a33` then corrected the correction: the spaced crossover is 67×67, not 68×68, because 67×67 prints as "200.4 KiB" and against a limit written as "200 KiB" that reads as under — a unit conversion hid a threshold crossing.

I re-measured all of it (see §5). Everything in the current comment reproduces, to the byte: 66×66 spaced is **199126** and under; 67×67 is **205224**, over by exactly **424**; a 60×60 at the cap is **157267** compact (153.6 KiB) and **164470** spaced (160.6 KiB); overriding all 3600 tiles with the shipped names lands at **214867** (209.8 KiB, `earth-1`) to **229267** (223.9 KiB, `flagstone-1`). The rate is not constant: 45.44 B/tile at 32×32, 45.72 at 67×67, 46.90 at 200×200, because the key strings lengthen. **The breach the arc recorded is still open**: 3600 tiles is inside `MaxWireTiles` and over the 200 KiB read limit as soon as every square is overridden. It was recorded rather than repaired, deliberately, and it has not been repaired since.

**The branch did not do the thing it was for.**
The whole-branch review's verdict was "no — not safe to merge", and `f1be666`'s own message concedes it: *"You could not load a map, and if you could it would not draw."* Six Criticals:

- **C1**: `load_map` did not exist. `mapdef.Compile`'s only production caller discarded its result as a boot dry run, so a map was validated, listed, its art served — and could never become events. Exit criteria 1–3 all named it.
- **C2**: nothing produced a `std:` image key, so every square without a custom override drew nothing — which is every square of both shipped adventures. Spec §4.1's "delete the overrides block and it renders identically" was **false**. The reason it was invisible is the sharpest fixture lesson in the arc: *"maps/cellar overrides 100% of its squares, which is exactly why the demo looked right and hid it."* That degeneracy was never removed — `campaigns/example/maps/cellar.json` today still has 90 tiles and 90 overrides.
- **C3**: spec §7's missing-tile marker was never built, and a test **pinned its absence**. That marker was the control that would have made C2 obvious on day one.
- **C4**: the canvas seam had zero coverage — seven mutations, including drawing the grid before terrain, all left 532 tests passing.
- **C6**: `task check` could not go green; six gate runs found four more real defects, including `genmappack` having no tests at all while its own commit message claimed reproducibility.
- **I1**: object art was validated nowhere, and `Pack.Objects` was loaded by `LoadPack` and read by nothing in Go. A typo'd art name on a `blocks_move` object produced a square players could not enter and could not see a reason for — spec §1.3's founding complaint, reintroduced from a typo. The rule that closed it promptly caught the conformance fixture added two commits earlier, whose objects declared no art: the fixture was itself an instance of the bug.

**Three comments promised a completeness the ruling had removed.**
`7266466`: after tiles became optional, three doc comments still asserted `Tiles` holds an entry for every square, unconditionally. Not a functional bug — every check handled the empty case — but *"Map.Tiles's own field doc is the thing a reader consults BEFORE writing code against the type, and it told them to assume len(Tiles) == GridW*GridH."*

**The gate that certified the arc had never run.**
`be34c6f`: *"The mutation gate had never completed on this branch. `task check` was SIGTERMed three times, always inside check:mutation, so every claim that this arc was gate-clean rested on a gate that had not run."* Walking all eleven packages one at a time found twelve unadjudicated survivors, eight of them real gaps. It also corrects `d0a53d9`, from five commits earlier:

> CORRECTION TO d0a53d9. That commit claims internal/mapdef reaches "100% efficacy, zero survivors". It does not, and the number came from a bare `gremlins unleash --tags=""` rather than the gate's invocation. […] gremlins counts a timed-out mutant as killed inside its efficacy percentage. […] A timeout is not a kill.

The eight survivors were all boundaries nothing had ever sat on: a token standing one square off the edge of the map; an object's bottom edge masked because the cellar fixture puts a wall below its boulder, so `Blocked` returned on the tile switch and never consulted `covers()`; a `DoorClosed` nil-guard which, negated, slams every other open door in the scene — alive because nothing had ever opened two doors; and `abs()` in the door-adjacency rule returning its argument unchanged, alive because the distant-door test put its token at (5,5) and the door at (0,1), both deltas positive, so `abs()` never had to do anything. Under that mutant a player works any door to the west or north from any distance. The earlier `d0a53d9` run had already found thirteen of the same shape, including `GridWidth < 1` → `<= 1`, which rejects a 1×1 map while 0 still fails, so every fixture kept passing.

**The camera's own fixtures were degenerate too, and nobody noticed for a week.**
Project memory `degenerate-fixtures-hide-mutants` records that every camera in the client suite was `fitCamera()` at scale 1, offset 0 — *"where `*scale` IS `/scale` and `+offset` IS `-offset`. That one degeneracy hid 18 arithmetic mutants, the camera transform three times over."* Found 2026-08-25, a week after the merge, at 100.00% line coverage.

**The plan deferred wiring that a generated surface publishes on sight.**
Task 6's `AMENDED 2026-08-13` note: adding a `ClientCommand` oneof arm makes a command reachable from the wire **and** publishes it as an MCP tool immediately, because the tool surface is generated from that oneof — so deferring the wiring left `open_door` and `close_door` advertised to a live agent and unable to convert. The plan records this rather than rewriting it, so the sequencing error stays visible.

**The spec omitted a tile name it had already decided on.** §3.3's first draft was prose listing ten names and omitted `wood-wall`; the plan's table carried eleven. The Task 2 review caught the divergence before anything depended on it. The durable half of the fix was turning the list into a table — §9 calls this vocabulary a one-way door, and a one-way door should not be specified in a form where an entry can go missing without anyone noticing.

**Exit criterion 5 was green with a caveat that has not been retired.** From the merge commit: *"Every gate is green, but `task check` as a single process has never completed on this machine — three SIGTERMs, all inside the long mutation phase."* The eleven packages were verified one at a time through the real orchestrator. That is weaker evidence than one clean end-to-end run, and the merge says so rather than smoothing it over.

**Two claims in the current tree that I could not carry into §2 as written.**

1. `format.go:22-24` says: *"Nothing here reads a manifest of any kind: the only art code in the tree is internal/artlib, and it looks a piece up by filename."* The first clause is true and correctly scoped to this package. The last clause is **false**. `client/public/std-pack/pack.json` is a live pack manifest with 11 tiles and 0 objects, generated by `tools/genmappack`, served from the static bundle, and read by `client/src/view/pack-assets.ts` — whose own header says so in as many words: *"IT IS A PACK AND IT KEEPS THE NAME. client/public/std-pack/pack.json is a real pack manifest on disk, generated by tools/genmappack, and this file reads it."* A reader applying `format.go`'s sentence literally would conclude no manifest exists anywhere in the tree.
2. `client/src/view/pack-assets.ts:62` cites *"mapdef.PackTile's own doc comment"*. `mapdef.PackTile` was deleted at art-is-a-flat-library Task 7, and `internal/mapdef/load_test.go:287` now bans the name. The citation points at a type that cannot be read. The absence test guards the name coming back into `mapdef`; nothing guards the citation pointing out of the client.

**The spec's own canonical example no longer loads.**
`docs/superpowers/specs/2026-08-12-maps-as-geometry-design.md:212` still carries `"pack": "mossy-keep"` inside §4.1's worked example, and §4.2 ("A pack") is intact with no supersession banner — `grep -n "art-is-a-flat\|2026-09" ` over that spec returns nothing. That file is refused today, by name, by `decodeStrict`'s `DisallowUnknownFields` (`TestAMapDeclaringAPackOrAPackageIsStillRefused`, passing). `docs/map-format.md` — the arc's API deliverable — **was** amended: it carries a `CORRECTION, 2026-09-04` banner at the top with a section-by-section table of what is now wrong. The design spec did not get the same treatment, and `6a66b6f` ("The specs say what is now true") ran a month before packs left and touched only §6.

**One of the arc's own late fixes was reversed by the next arc.** `1c3f2d7` closed a cold-author gap by ruling that *"the file must be named map.json and live in its own subdirectory (with the optional pack under tiles/)"*. `71b4635` (2026-09-01, "A map is a file, and its name is who it is") made the map a flat `<id>.json` with the filename as the id, and `6f47235` moved the directory under the campaign. Nothing in this arc's documents says so.

---

## 5. Spår

**How the range was established, and what git alone can prove.**

```
git log --oneline --follow -- internal/mapdef/ | tail -1
  -> daa2b9c  "mapdef: parse and validate a map file (maps-as-geometry Task 2)"
             2026-08-13 09:13:44 +0200

git log --merges --ancestry-path --format='%h %ad %s' --date=short daa2b9c..main | tail -1
  -> f94c529  2026-08-18  "Merge maps-as-geometry: a scene becomes a place"

git rev-parse f94c529^1 f94c529^2
  -> 5948b1b  base   ("Plan: ten tasks to make a scene a place", 2026-08-13 07:19:18)
  -> f3d5c3d  tip    ("Answer the last two questions the cold author had to guess at")

git rev-list --no-merges --count 5948b1b..f3d5c3d   ->  20
git rev-list             --count 5948b1b..f3d5c3d   ->  20   (no internal merges)
```

The arc therefore reads `5948b1b..f3d5c3d`, twenty commits, merged as `f94c529` at 2026-08-18 17:53:53. **Fifteen minutes later a second merge, `c40a450`, carries one more commit, `01e0f13`** — "Move the trees-as-pillars ruling into the repo, and unsay a false claim" — which lands spec §11 and the first correction of the construction-site claim. It is the arc's epilogue and I have counted it as in range; git cannot tell you that on its own, only that it is a one-commit merge on top of the arc's merge that touches the same spec and the same file.

**What git cannot prove here.** No branch ref survives: `git for-each-ref` lists only `main`, `origin/main` and `feat/per-character-logs`, and the arc's branch was deleted after merging (project memory `one-branch-per-issue-clean-up-after-merge`). Unlike most merges in this repo — `76e45c2 Merge feat/perch-ui`, `b674abb Merge sub-project 16` — `f94c529`'s subject names **the arc**, not a ref, so the branch's name is not recoverable from git at all. `.superpowers/sdd/2026-08-12-maps-as-geometry/` holds 38 task briefs, reports and review diffs, and is gitignored: I read from it but nothing in it is in history, so none of it is checkable by a future reader from the repository alone.

**How the comment ratio was measured.** A script in the session scratchpad over `internal/mapdef/*.go` excluding `_test.go`, counting a line as a comment if it begins `//`, begins `/*`, or falls inside an unterminated block:

```
compile.go   total 382  comment 230  code 135
format.go    total 194  comment 155  code  27
installed.go total 200  comment 134  code  57
load.go      total 576  comment 284  code 261
resolve.go   total 435  comment 325  code  98
standard.go  total  43  comment  15  code  25
TOTAL        comment 1143  code 603  ->  65.5%
blocks of >= 10 consecutive comment lines: 32
```

That reproduces the brief exactly. Run against `git show f94c529:<file>` for the five files that existed then, the same script gives **comment 412, code 450, 47.8%** — so this arc contributed 412 of the 1,143 lines, and the three arcs after it added 731 more while the code grew by 153. `installed.go` did not exist at the merge.

**How the wire figures were re-measured, without writing to the repo.** `go test -overlay=<json>` with a `_test.go` file living in the scratchpad, so the tree was never touched:

```
 32x32  tiles  1024  bytes   46534 ( 45.4 KiB)  45.44 B/tile  over 204800: no
 60x60  tiles  3600  bytes  164470 (160.6 KiB)  45.69 B/tile  over 204800: no   [spaced build]
 60x60  tiles  3600  bytes  157267 (153.6 KiB)  43.69 B/tile  over 204800: no   [compact build]
 66x66  tiles  4356  bytes  199126 (194.5 KiB)  45.71 B/tile  over 204800: no
 67x67  tiles  4489  bytes  205224 (200.4 KiB)  45.72 B/tile  over 204800: YES  (+424)
200x200 tiles 40000  bytes 1876072                46.90 B/tile
 60x60 + art "earth-1"     on all 3600 -> 214867 (209.8 KiB)  over: YES
 60x60 + art "flagstone-1" on all 3600 -> 229267 (223.9 KiB)  over: YES
```

Two of those rows came from two different builds of the same test in the same session and differ by 7,203 bytes — which is protojson's build-to-build space-after-comma randomisation happening live, the thing `MaxWireTiles`' comment describes. Every figure in that comment reproduced; none needed correcting.

**How block provenance was established.** `git blame -L <start>,<end> --line-porcelain` per block, tallied by commit, then each commit dated with `git log -1 --format='%ad %s'` and matched against the arc ranges above. The commit → arc mapping used: 2026-08-13..08-18 = this arc; 2026-08-22 (`4ee37b9`, `7b91a33`) = the post-merge byte-figure sweep; 2026-09-01 (`e110e9b`, `6f47235`, `71b4635`, `0b9d6b7`, `67218f6`) = create-scene-leaves; 2026-09-02..09-10 (`6c0f02c`, `bfd6ebb`, `8a28f34`, `390879a`, `a049e4d`, `035248e`, `66ef637`, `16a58dd`, `f04c63b`, `24ae4a0`, `2b5878d`, `91ab275`, `2cfed2c`) = art-is-a-flat-library; 2026-09-12/16 (`773cd0ff`, `53bb71f`) = sub-project 14.

---

## 6. Kommentarsblock denna rapport friar

All 32 blocks are listed. Not one is pure history — the third report's finding holds here without exception: every block wraps at least one rule in a story, and several wrap a rule that is still the only statement of it anywhere. **Seventeen of the 32 are not this arc's to free**, because their narrative was written by a later arc; they are named at the end so the inventory is complete and so the next reader does not have to re-blame them.

### Freed by this report

**`format.go:1-29` — package doc: what `mapdef` is, that a map declaring a pack is refused, and which file may touch the contract.**
Stays: `Load` reads and fully validates one map file, failing loud at load time rather than at the table. A tile name resolves only against `standard.go`'s vocabulary; an art override resolves by filename through `internal/artlib`. Nothing about `Kind`/`Material` ever comes from art. A map file carrying a top-level `"pack"` — or any unknown field — is refused by `decodeStrict`'s `DisallowUnknownFields`, not dropped. `compile.go` is the only file in this package that may import `contract/gen/go/vtt/v1`.
Correct before carrying: "the only art code in the tree is `internal/artlib`" is false — see §4.

**`format.go:132-146` — `Map.Tiles`: optional, and complete if declared.** (Entirely this arc: `7266466` + `daa2b9c`.)
Stays: `Tiles` is optional and empty is legal — a scene that declares no tiles has no terrain and still loads, and that must stay legal forever. If it holds any entry at all it must hold one for every square in `GridW × GridH`. **Do not assume `len(Tiles) == GridW*GridH`; assert on it and you reintroduce the breakage the ruling removed.**

**`load.go:87-122` — `Load`'s contract and the order its checks run in.**
Stays: decoding is strict and an unknown field is refused rather than dropped. `Load` returns `(nil, err)` on the first violation and every error names the offending file and field. The order is load-bearing and is the first thing to preserve if the checks are ever reordered: grid sanity gates everything; `Tiles` **shape** before `Tiles` **validity**; `Overrides` and `Objects` need only a sane grid; `Placements` must run last because it is the only check that reads an already-validated tile name.

**`load.go:230-248` — `CheckEverySquarePresent`: the completeness rule plus its opt-out.**
Stays: an empty `tiles` map means "no terrain, nothing to check" and returns nil; **any** entry means the map has committed to declaring terrain and completeness is then not negotiable. The walk itself is `RequireEverySquarePresent`; this function is that walk plus the opt-out, and the opt-out is the only difference. Callers run this before `CheckTileNamesKnown` so the more fundamental defect — a square with no answer at all — is reported first.

**`load.go:305-315` — `CheckOverridesRequireTiles`.** (Entirely this arc: `3d07345`.)
Stays: a non-empty `overrides` against an empty `tiles` is refused — there is nothing to attach the art to. Against a non-empty `tiles` no further check is needed here, because `CheckOverridesInsideGrid` already proves every key in-bounds and a complete `tiles` map covers every in-bounds square by construction.

**`load.go:344-353` — `CheckTileNamesKnown`.**
Stays: validates every `tiles` **value** against the standard vocabulary. Runs after the two shape checks, so walking the `tiles` map rather than the grid covers exactly the same squares. `Overrides` values are not checked here — that needs an art directory, which neither `Load` nor this function ever takes.

**`load.go:381-403` — `CheckObjectFootprints`.**
Stays: the whole **footprint** must be inside the grid, not merely the anchor. Size must be at least 1×1 — a zero or omitted size makes the footprint check and the anchor check the same comparison, and it breaks silently. The arithmetic is `int64` because `At` and `Size` come straight from author-supplied JSON and `at:2147483647, size:1` wraps an int32 sum negative, which also passes. **This function's job stops at geometry**; an object's art is checked by `CheckObjectArtDeclared` (declared at all) and `ResolveObjectArt` (actually resolves).

**`load.go:419-441` — `CheckObjectArtDeclared`.**
Stays: every object must name non-empty art. A tile's art may legally be empty because it falls back to the standard vocabulary; **an object has no such fallback**, so an object with empty art can never draw under any circumstances — the invisible-barrier defect spec §1.3 exists to prevent. Unlike a merely wrong name, which starts drawing the moment the right file is installed, an empty one names nothing that could ever be installed, so it is refused at `Load` rather than deferred to whichever caller has an art directory. Whether a non-empty name is installed is a separate question and is a warning, not a refusal.

**`load.go:452-465` — `CheckPlacementsNotInWalls`.** (Entirely this arc: `7266466` + `3d07345` + `daa2b9c`.)
Stays: a token must not start inside a wall. Run last, because it is the one check that reads a square's **tile name** rather than its coordinates. When `tiles` is empty every placement passes, and **that is correct, not merely tolerated** — a scene with no terrain has no walls to stand inside. A placement whose own square is outside the grid is caught here too, since `tiles` has no entry to read otherwise.

**`compile.go:11-56` — `MaxWireTiles`: why there is a cap and what it does and does not bound.**
Stays: 3600 is a **transport** limit, not a rule about maps. It is **counted in tiles, not grid squares** — tiles are optional, so a large grid declaring no terrain costs nothing on the wire and must still load; `internal/rules/conformance` relies on that with a tile-less 100×100 scene, and sizing this on `GridW*GridH` would refuse maps that are free to send. **The cap and the client read limit do not agree**: a fully art-overridden scene at the cap is over the 200 KiB limit while still inside this cap, because this counts tiles and the limit counts bytes. That is recorded rather than repaired, and changing it is a decision about the wire format. Past the cap the frame simply does not arrive and the connection is torn down mid-session. This is not a permanent ceiling — a compact palette-and-index encoding would remove it.
Note for the cleanup: every number in this block reproduces (§5). The history to lift out is the protojson-regime narration and the goblin-ambush anecdote, not the figures.

**`compile.go:59-78` — `Compile`'s contract.**
Stays: exactly one `SceneCreated` carrying the resolved terrain of every square the map **declares** — none, for a scene that declares no tiles, which is legal — plus its objects, followed by one `TokenPlaced` per placement **in declaration order**. Every warning is collected, deduplicated and returned rather than dropped, and `Compile` never refuses on one. The single case that does refuse is a sidecar declaring a `format_version` **later** than this server understands.

**`compile.go:100-172` — `BuildSceneCreated`: the one resolution site, its redacted twin, row-major order, and the empty-tiles skip.**
Stays: this is the one place a map **file**'s tile names are resolved into `TileRef`s, and both file load paths call this exact function, so they cannot drift. **It is not the only place a `SceneCreated` is built** — adding a field to `SceneCreated` means touching every builder, or deciding in writing that a projected seat is not supposed to have it; `TestBothLoadPathsEmitIdenticalSceneEvents` holds the map path against the adventure path and **both come through this function**, so it cannot see the redacted builder at all. Squares are resolved **row-major, walking the grid rather than ranging `m.Tiles`** — Go map iteration order is randomised per run, and golden stability depends on this being deterministic. Empty `m.Tiles` skips the square loop entirely, because `Resolve` has no notion of "this map opted out of terrain" and would fail per square. `artDir` may be empty, missing or unopenable; the map still loads and draws plain.
Correct before carrying: "Two live sites answer" is wrong — there are three (§4). The prescribed grep stays; the count must be replaced by the invariant, not by a new number.

**`compile.go:230-240` — object art resolved inside the same dry run as tile art.**
Stays: every object's art is resolved by this walk, so a bad object art name is reported by the boot-time dry run the same way an unresolvable tile override is, rather than riding through to a `SceneObject` nothing can ever draw. It reports rather than refuses: the object stays in the world, drawn from its kind.

**`resolve.go:17-104` — `Resolve`'s doc. Shared block; this report frees only lines ~89-100**, the paragraph about the two refusals that were deleted (`p == nil` and the map's-own-pack comparison). Everything above it is art-is-a-flat-library's history.
Stays from the freed paragraph: **an error must be true of the world rather than of the call** — which is why the degrade warning says the art is not installed rather than that no art directory was handed in.
Stays from the arc's surviving rule at lines 22-26, wherever this block ends up: **nature always comes from `m.Tiles`; the override supplies art and nothing else. A kind mismatch warns rather than refusing**, because a wall that looks like a passage is an illusory wall.

**`resolve.go:184-227` — `ResolveObjectArt`'s doc. Shared block; this report frees the finding-I1 sentence and the "own function" argument.**
Stays: `ResolveObjectArt` is deliberately its own function and **not a call to `Resolve`** — `Resolve` is keyed by a square `"x,y"`, and an object has no square key of its own, so forcing it through that signature would misrepresent what an object is. `idx` identifies the object by its position in `Map.Objects`, because an object's ID is author-supplied and neither required nor guaranteed unique. **The four-way split is written out twice and nothing forces the two switches to agree**, which is why the object-side tests are not redundant with the tile-side ones. An empty `o.Art` is refused here, redundantly with `CheckObjectArtDeclared`, because a caller that builds a `*Map` by hand can reach here without ever having run `Load`.

**Also this arc's, below the ten-line threshold and so outside the 32:** `standard.go` is 100% this arc's work and has not been touched since 2026-08-18. Its three comments carry rules that must survive — the vocabulary is a closed set of **natures**; `kind` is the closed spatial set the engine reads and `material` is opaque, which is what keeps CLAUDE.md rule 5 satisfied; a door is one nature and its openness is folded state; `StandardTileNames` returns a **copy** so a caller cannot mutate the package's vocabulary. One premise there is stale: *"A custom pack adds pictures; it never adds natures"* — there are no packs any more. The rule survives with its subject renamed: **art adds pictures; it never adds natures.**

### Not freed by this report — later arcs' history

These belong to the arcs named; blaming them again is wasted work.

*create-scene-leaves (2026-09-01/02):* `installed.go:20-65` (`LoadInstalled` as the one boot/on-demand path, and the absolute-path leak), `installed.go:126-151` (`idIsAFilename`), `load.go:256-292` (`RequireEverySquarePresent` sitting at no boundary), `load.go:505-523` (`decodeStrict`'s `display`/`path` split), `load.go:542-551` (`unpath`).

*art-is-a-flat-library (2026-09-02..09-10):* `format.go:43-86` (`MinCellPx`/`MaxCellPx` and the MapTool divergence), `format.go:111-129` (`CellPx`, and its move from campaign to map), `installed.go:74-88` and `installed.go:156-175` (`refuseCaseOnlyMatch`), `compile.go:188-198` (the one-snapshot `artlib.Open`), `compile.go:262-299` (`warningTally` and the 6840-byte correction), `compile.go:343-352` (`render`), `load.go:563-574` (`fieldErr` as a backstop), `resolve.go:153-164` (sidecar expected for tile art, never for object art), `resolve.go:264-284` (warnings name the art, never the square or a path), `resolve.go:310-416` (`artCannotBeUsed` and the warning-length corrections — at 107 lines the largest block in the package), `resolve.go:422-434` (`name`).

---

## Method note

Every claim above was checked by command against the tree at `48e3fe2`, not by reading a comment and finding the code consistent with it. Where a comment made a numeric claim I re-ran it; where it made an "only"/"every" claim I ran the search that bounds it, including the grep the comment prescribes for itself. Three claims failed that check and are in §4 rather than §2. The one class of thing I could not establish from the repository is anything that lived only in `.superpowers/` (gitignored) or in a deleted branch ref: the arc's branch name, and the task-by-task review rounds, are readable on this machine and not from git.
