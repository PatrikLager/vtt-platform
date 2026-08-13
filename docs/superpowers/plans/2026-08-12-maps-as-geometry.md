# Maps as Geometry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A scene becomes a described space — walls, floors, doors, objects, and
optional per-square art — authorable as a file an LLM can write, loadable
standalone or inside an adventure, enforced for movement, and rendered through a
camera.

**Architecture:** A new `internal/mapdef` package parses and validates a map
file and its art pack, then compiles it to events. Both load paths (standalone
`maps/` and adventure-embedded scenes) go through that one compiler.
`engine.State` gains terrain and door state so `Apply` can fold doors and a pure
query can answer passability; the gateway enforces that for players only. The
client folds terrain into its own state, turns it into a **pure list of draw
instructions**, and a thin canvas layer executes them under a camera transform.

**Tech Stack:** Go 1.x, protobuf (`vtt.v1`, buf), SQLite, TypeScript + Vite +
bun test, happy-dom, Canvas 2D.

**Spec:** `docs/superpowers/specs/2026-08-12-maps-as-geometry-design.md` (96d2967)

## Global Constraints

- **ADR-009 airtight TDD.** Tests first. **Behavioural RED, not compile-failure
  RED** — add a compiling stub so the test fails on behaviour. Every load-bearing
  assertion needs fault-injection proof: break the implementation, watch the test
  catch it, restore.
- **`task check` is the single gateway.** Run it as `task check > file 2>&1; echo $?`
  — **never pipe into `tail`**, which measures `tail`'s exit status and has already
  produced one false green in this project.
- **Contract evolution is additive only** (ADR-007). `task check:breaking`
  enforces. Generated code is committed; regenerate with `task generate:contract`.
- **No game-system vocabulary in platform code** (CLAUDE.md rule 5, semgrep
  enforces). `material` is an **opaque string** the platform never interprets.
  `kind` on a tile is spatial (`wall`/`floor`/`door`), not a game concept.
- **One fold.** `engine.Apply` is the only code that changes game state.
- **Art never decides nature** (spec §3.2). A wall drawn as floorboards is still
  a wall. A kind mismatch **warns**, never refuses.
- **Two resolution levels only** (spec §4.2): the map's own pack, then the
  standard vocabulary. No campaign tier.
- **Review before commit.** The dev-cycle hook enforces it.

---

## File Structure

**New — `internal/mapdef/`** (parse, validate, resolve, compile; used by both load paths)
- `format.go` — the parsed types: `Map`, `Object`, `Pack`, `PackTile`
- `standard.go` — the standard tile vocabulary as data
- `load.go` — `Load`, `LoadPack`, all validation
- `resolve.go` — tile name → resolved kind/material/art
- `compile.go` — `Compile(*Map) []*vttv1.Envelope`
- `testdata/invalid/<case>/` — one directory per refusal (pattern: `internal/rules/testdata/invalid-v2/`)

**New — client**
- `client/src/view/scene-plan.ts` — **pure**: state + camera + pack → draw instructions
- `client/src/view/camera.ts` — **pure**: fit/zoom/pan maths and screen↔world transforms
- `client/src/view/canvas.ts` — **thin**: walks instructions calling `drawImage`

**New — content and docs**
- `maps/cellar/` — the demo map, with cover
- `docs/map-format.md` — **the API document**, the thing handed to an LLM

**Modified**
- `contract/vtt/v1/events.proto`, `commands.proto`
- `internal/engine/state.go`, `internal/engine/apply.go`
- `internal/adventure/load.go`, `internal/adventure/compile.go`
- `internal/gateway/authz.go`, `internal/gateway/convert.go`, `internal/gateway/server.go`
- `cmd/vtt/serve.go`
- `client/src/state.ts`, `client/src/view/grid.ts`, `client/src/view/spectator.ts`, `client/src/style.css`

---

## Task 1: Contract additions

**Files:**
- Modify: `contract/vtt/v1/events.proto`
- Modify: `contract/vtt/v1/commands.proto`
- Test: `contract/contract_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `vttv1.TileRef{Kind, Material, Art string}`;
  `vttv1.SceneObject{ObjectId, Kind string, At *GridPosition, Width, Height, RotationDegrees int32, BlocksSight, BlocksMove bool, Art string}`;
  `SceneCreated.Tiles map[string]*TileRef`, `SceneCreated.Objects []*SceneObject`;
  same two fields on `CreateScene`; `OpenDoor{SceneId, At}`, `CloseDoor{SceneId, At}`,
  `DoorOpened{SceneId, At}`, `DoorClosed{SceneId, At}`.

- [ ] **Step 1: Write the failing test**

In `contract/contract_test.go`:

```go
func TestSceneCarriesTerrainAndDoorsExist(t *testing.T) {
	sc := &vttv1.SceneCreated{
		SceneId: "s1", Name: "Cellar", GridWidth: 3, GridHeight: 3,
		Tiles: map[string]*vttv1.TileRef{
			"0,0": {Kind: "wall", Material: "stone", Art: ""},
			"1,1": {Kind: "floor", Material: "wood", Art: "planks-3"},
		},
		Objects: []*vttv1.SceneObject{{
			ObjectId: "o1", Kind: "boulder",
			At:    &vttv1.GridPosition{X: 1, Y: 1},
			Width: 1, Height: 1, BlocksSight: true, BlocksMove: true,
		}},
	}
	if sc.GetTiles()["1,1"].GetArt() != "planks-3" {
		t.Fatal("art did not survive the round trip")
	}
	// Doors are a nature plus folded state, never two natures (spec §3.3).
	_ = &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
		DoorOpened: &vttv1.DoorOpened{SceneId: "s1", At: &vttv1.GridPosition{X: 0, Y: 1}}}}
	_ = &vttv1.ClientCommand{Command: &vttv1.ClientCommand_OpenDoor{
		OpenDoor: &vttv1.OpenDoor{SceneId: "s1", At: &vttv1.GridPosition{X: 0, Y: 1}}}}
}
```

- [ ] **Step 2: Run it and watch it fail to compile**

Run: `go test ./contract/ -run TestSceneCarriesTerrainAndDoorsExist`
Expected: FAIL — `undefined: vttv1.TileRef`. This is the one place a
compile-failure RED is correct: the contract *is* the type, so there is no
behaviour to stub.

- [ ] **Step 3: Add the messages**

In `events.proto`:

```protobuf
// TileRef is one square, resolved at LOAD from a tile name into the facts the
// engine needs plus the art name the renderer needs. Resolved rather than
// referenced so the log never depends on a pack file being present at replay
// (maps-as-geometry spec §5).
message TileRef {
  string kind = 1;      // "wall" | "floor" | "door" — spatial, closed set
  string material = 2;  // OPAQUE. The ruleset's business, never the platform's.
  string art = 3;       // pack tile name; empty means the standard picture
}

// SceneObject is SCENERY. Anything that acts, moves or holds state is an actor
// with a token (spec §3.4) — this line is what stops SceneObject becoming a
// second entity system.
//
// kind here is an OPEN descriptive label ("boulder", "chest") for the DM and
// the LLM to talk about. It is NOT the closed spatial set TileRef.kind uses,
// and no behaviour may be inferred from it: structural effect comes only from
// blocks_sight and blocks_move.
message SceneObject {
  string object_id = 1;
  string kind = 2;
  GridPosition at = 3;
  int32 width = 4;
  int32 height = 5;
  int32 rotation_degrees = 6;
  bool blocks_sight = 7;
  bool blocks_move = 8;
  string art = 9;
}

message DoorOpened { string scene_id = 1; GridPosition at = 2; }
message DoorClosed { string scene_id = 1; GridPosition at = 2; }
```

Add to `SceneCreated` (next free field numbers) and to the `Envelope` oneof
(next free tags). In `commands.proto`, add `OpenDoor`/`CloseDoor` messages, the
matching `ClientCommand` oneof arms, and the same two fields on `CreateScene`.

- [ ] **Step 4: Regenerate and verify**

```bash
task generate:contract
go test ./contract/ -run TestSceneCarriesTerrainAndDoorsExist
task check:breaking
```
Expected: test PASS; `check:breaking` PASS (every change is a new field or a new
oneof arm — nothing renumbered, nothing removed).

- [ ] **Step 5: Commit**

```bash
git add contract/
git commit   # message: contract carries terrain, scenery and door events
```

---

## Task 2: `internal/mapdef` — parse and validate a map

**Files:**
- Create: `internal/mapdef/format.go`, `internal/mapdef/standard.go`, `internal/mapdef/load.go`
- Create: `internal/mapdef/load_test.go`
- Create: `internal/mapdef/testdata/valid/cellar.json`
- Create: `internal/mapdef/testdata/invalid/<case>/map.json` (one dir per refusal)

**Interfaces:**
- Consumes: nothing from Task 1 yet (compile comes in Task 4).
- Produces:
  ```go
  type Map struct {
      ID, Name string
      GridW, GridH int32
      Pack string
      Tiles map[string]string      // "x,y" -> tile name
      Overrides map[string]string  // "x,y" -> pack tile name
      Objects []Object
      Placements []Placement       // same shape as adventure.Placement
  }
  type Object struct {
      ID, Kind string
      X, Y, W, H, Rotation int32
      BlocksSight, BlocksMove bool
      Art string
  }
  func Load(path string) (*Map, error)
  func StandardTile(name string) (kind, material string, ok bool)
  ```

- [ ] **Step 1: Write the failing tests**

`internal/mapdef/load_test.go`:

```go
func TestLoadsAMapWhereEverySquareNamesItsTile(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.Tiles["0,0"] != "stone-wall" {
		t.Fatalf("square 0,0 is %q, want stone-wall", m.Tiles["0,0"])
	}
	// The override changes the PICTURE only; the square is still what tiles says.
	if m.Tiles["1,1"] != "wood" || m.Overrides["1,1"] != "planks-split-3" {
		t.Fatalf("override did not stay separate from nature: %v / %v",
			m.Tiles["1,1"], m.Overrides["1,1"])
	}
}

// Every refusal in spec §4.4 gets a case. Table-driven over fixture dirs,
// following internal/rules/testdata/invalid-v2/'s pattern.
func TestInvalidMapsAreRefusedWithAUsefulReason(t *testing.T) {
	for _, c := range []struct{ dir, want string }{
		{"missing-square", "no tile"},
		{"unknown-tile-name", "unknown tile"},
		{"override-outside-grid", "outside the grid"},
		{"object-outside-grid", "outside the grid"},
		{"token-inside-wall", "inside a wall"},
		{"zero-grid", "must be >= 1"},
	} {
		t.Run(c.dir, func(t *testing.T) {
			_, err := mapdef.Load(filepath.Join("testdata/invalid", c.dir, "map.json"))
			if err == nil {
				t.Fatal("this map was accepted; every square must be accounted for")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error was %q, want it to mention %q", err, c.want)
			}
		})
	}
}
```

Create `testdata/valid/cellar.json` as a 3×3: walls around, `wood` across the
middle row, `"overrides": {"1,1": "planks-split-3"}`, one placement at `2,1`.
Create one fixture directory per invalid case above, each differing from the
valid file in exactly one way.

- [ ] **Step 2: Run and watch it fail behaviourally**

Add `format.go` with the structs and `load.go` with
`func Load(path string) (*Map, error) { return &Map{}, nil }` first — a stub that
compiles. Then:

Run: `go test ./internal/mapdef/`
Expected: FAIL — `square 0,0 is "", want stone-wall`, and every invalid case
fails with "this map was accepted". Behavioural RED, not a compile error.

- [ ] **Step 3: Implement `Load` and the standard vocabulary**

`standard.go` — the vocabulary is data, deliberately small (spec §9: adding a
nature later is additive; removing one is not):

```go
// standardTiles is the published vocabulary of NATURES. A custom pack adds
// pictures; it never adds natures (spec §3.3). A door is ONE nature — whether
// it is open is folded state, never part of the name.
var standardTiles = map[string]struct{ Kind, Material string }{
	"stone-wall": {"wall", "stone"},
	"wood-wall":  {"wall", "wood"},
	"wood-door":  {"door", "wood"},
	"stone":      {"floor", "stone"},
	"wood":       {"floor", "wood"},
	"earth":      {"floor", "earth"},
	"grass":      {"floor", "grass"},
	"sand":       {"floor", "sand"},
	"water":      {"floor", "water"},
	"metal":      {"floor", "metal"},
	"ice":        {"floor", "ice"},
}

func StandardTile(name string) (kind, material string, ok bool) {
	t, ok := standardTiles[name]
	return t.Kind, t.Material, ok
}
```

`load.go` — decode strictly, then validate in this order so the first error is
the most useful: grid sane; every square in `w × h` present in `Tiles`; every
tile name in the standard vocabulary (pack names are resolved in Task 3); every
`Overrides` key inside the grid; every object inside the grid; every placement
on a square whose resolved kind is not `wall`.

- [ ] **Step 4: Run to green, then fault-inject**

```bash
go test ./internal/mapdef/
```
Expected: PASS.

Then prove the load-bearing assertion: delete the "every square present" loop,
re-run, confirm `missing-square` fails. Restore. Do the same for the
token-inside-wall check. **Record both injections in the commit message** — this
is the ADR-009 requirement, not a nicety.

- [ ] **Step 5: Commit**

```bash
git add internal/mapdef/
git commit   # message: a map file where every square must account for itself
```

---

## Task 3: Packs — manifest, resolution, and the mismatch warning

**Files:**
- Create: `internal/mapdef/resolve.go`, `internal/mapdef/resolve_test.go`
- Modify: `internal/mapdef/format.go` (add `Pack`, `PackTile`)
- Modify: `internal/mapdef/load.go` (`LoadPack`; wire pack names into validation)
- Create: `internal/mapdef/testdata/packs/mossy-keep/pack.json`

**Interfaces:**
- Consumes: `Map`, `StandardTile` from Task 2.
- Produces:
  ```go
  type PackTile struct {
      Name, Kind, Material  string
      File, FileOpen, FileClosed string
      Desc string
  }
  type Pack struct {
      ID, Name string
      CellPx   int32
      Tiles    map[string]PackTile
      Objects  map[string]PackTile
  }
  func LoadPack(dir string) (*Pack, error)
  // Resolved is what one square becomes on the wire.
  type Resolved struct{ Kind, Material, Art string }
  func Resolve(m *Map, p *Pack, square string) (Resolved, []string, error) // warnings, error
  ```

- [ ] **Step 1: Write the failing test**

```go
func TestOverrideChangesThePictureAndNothingElse(t *testing.T) {
	m, _ := mapdef.Load("testdata/valid/cellar.json")
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	got, _, err := mapdef.Resolve(m, p, "1,1")
	if err != nil {
		t.Fatal(err)
	}
	// Nature comes from tiles["1,1"] == "wood". The override supplies art ONLY.
	if got.Kind != "floor" || got.Material != "wood" {
		t.Fatalf("the override changed the square's nature: %+v", got)
	}
	if got.Art != "planks-split-3" {
		t.Fatalf("art is %q, want planks-split-3", got.Art)
	}
}

func TestAWallDrawnAsFloorboardsIsStillAWall(t *testing.T) {
	// Spec §3.2, and it is deliberately NOT an error: this is how an illusory
	// wall is built, one arc before illusions become a feature.
	m, _ := mapdef.Load("testdata/valid/cellar.json")
	m.Overrides["0,0"] = "planks-split-3" // a floor tile on a wall square
	p, _ := mapdef.LoadPack("testdata/packs/mossy-keep")

	got, warnings, err := mapdef.Resolve(m, p, "0,0")
	if err != nil {
		t.Fatalf("a kind mismatch was REFUSED; it must only warn: %v", err)
	}
	if got.Kind != "wall" {
		t.Fatalf("art decided the nature: kind is %q, want wall", got.Kind)
	}
	if len(warnings) == 0 {
		t.Fatal("a kind mismatch produced no warning at all")
	}
}

func TestAnUnknownTileNameIsRefusedRatherThanFallingThrough(t *testing.T) {
	// There are exactly two levels and no name-chasing between packs: a custom
	// name means nothing outside the pack that defines it (spec §4.2).
	m, _ := mapdef.Load("testdata/valid/cellar.json")
	m.Overrides["1,1"] = "no-such-tile"
	p, _ := mapdef.LoadPack("testdata/packs/mossy-keep")
	if _, _, err := mapdef.Resolve(m, p, "1,1"); err == nil {
		t.Fatal("an unresolvable art name was accepted")
	}
}
```

- [ ] **Step 2: Run and watch it fail behaviourally**

Stub `Resolve` returning `Resolved{}, nil, nil` and `LoadPack` returning
`&Pack{}, nil`.

Run: `go test ./internal/mapdef/ -run 'Override|Floorboards|Unknown'`
Expected: FAIL — `the override changed the square's nature: {  }`.

- [ ] **Step 3: Implement**

```go
// Resolve turns one square into the facts the engine needs plus the art name
// the renderer needs.
//
// NATURE ALWAYS COMES FROM m.Tiles. The override supplies art and nothing else
// (spec §3.2, Patrik: "the ART will never decide the nature of the
// square/item"). A kind mismatch WARNS rather than refusing, because a wall
// that looks like a passage is an illusory wall — legitimate dungeon craft, and
// refusing it would forbid a feature one arc away.
func Resolve(m *Map, p *Pack, square string) (Resolved, []string, error) {
	base, ok := m.Tiles[square]
	if !ok {
		return Resolved{}, nil, fmt.Errorf("mapdef: square %s has no tile", square)
	}
	kind, material, ok := StandardTile(base)
	if !ok {
		return Resolved{}, nil, fmt.Errorf("mapdef: square %s names unknown tile %q", square, base)
	}
	art, hasArt := m.Overrides[square]
	if !hasArt {
		return Resolved{Kind: kind, Material: material}, nil, nil
	}
	pt, ok := p.Tiles[art]
	if !ok {
		return Resolved{}, nil, fmt.Errorf(
			"mapdef: square %s names art %q, which pack %q does not define", square, art, p.ID)
	}
	var warnings []string
	if pt.Kind != kind {
		warnings = append(warnings, fmt.Sprintf(
			"square %s is %s but its art %q was drawn for %s", square, kind, art, pt.Kind))
	}
	return Resolved{Kind: kind, Material: material, Art: art}, warnings, nil
}
```

- [ ] **Step 4: Run to green, then fault-inject**

Run: `go test ./internal/mapdef/`
Expected: PASS.

Inject: change `Resolved{Kind: kind, ...}` to use `pt.Kind` when art is present.
Confirm `TestAWallDrawnAsFloorboardsIsStillAWall` fails with "art decided the
nature". Restore. That injection is the proof the whole §3.2 rule is enforced
rather than merely documented.

- [ ] **Step 5: Commit**

```bash
git add internal/mapdef/
git commit   # message: art resolves to a picture and never to a fact
```

---

## Task 4: One compiler, two load paths

**Files:**
- Create: `internal/mapdef/compile.go`, `internal/mapdef/compile_test.go`
- Modify: `internal/adventure/load.go:229-242` (`sceneJSON` gains tiles/overrides/objects)
- Modify: `internal/adventure/compile.go:44-50` (delegate scene events to `mapdef`)
- Modify: `internal/adventure/format.go:36-40` (`AdventureScene` gains the same)

**Interfaces:**
- Consumes: `Map`, `Pack`, `Resolve` from Tasks 2–3.
- Produces: `func Compile(m *Map, p *Pack) ([]*vttv1.Envelope, []string, error)` —
  emits exactly one `SceneCreated` (with `Tiles` and `Objects` populated) followed
  by one `TokenPlaced` per placement, in declaration order.

- [ ] **Step 1: Write the failing test**

```go
// The two load paths must be ONE path. A standalone map and the same scene
// embedded in an adventure must produce byte-identical scene events, or the
// formats will drift and only one of them will be tested.
func TestBothLoadPathsEmitIdenticalSceneEvents(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := mapdef.LoadPack("testdata/packs/mossy-keep")
	standalone, _, err := mapdef.Compile(m, p)
	if err != nil {
		t.Fatal(err)
	}

	adv, err := adventure.Load("testdata/adventures/cellar-adv", testRuleset(t))
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		t.Fatal(err)
	}

	want := firstSceneCreated(t, standalone)
	got := firstSceneCreated(t, embedded)
	if !proto.Equal(want, got) {
		t.Fatalf("the two load paths diverged:\nstandalone: %v\nembedded:   %v", want, got)
	}
}

func TestASceneCreatedCarriesEverySquare(t *testing.T) {
	m, _ := mapdef.Load("testdata/valid/cellar.json")
	p, _ := mapdef.LoadPack("testdata/packs/mossy-keep")
	envs, _, _ := mapdef.Compile(m, p)
	sc := firstSceneCreated(t, envs)
	if len(sc.GetTiles()) != 9 {
		t.Fatalf("3x3 scene carries %d squares, want 9", len(sc.GetTiles()))
	}
	if sc.GetTiles()["1,1"].GetMaterial() != "wood" {
		t.Fatalf("material did not survive compile: %v", sc.GetTiles()["1,1"])
	}
}
```

Create `internal/mapdef/testdata/adventures/cellar-adv/` — a minimal adventure
whose single `scenes/cellar.json` has the **same** tiles, overrides, objects and
placements as `testdata/valid/cellar.json`.

- [ ] **Step 2: Run and watch it fail**

Stub `Compile` returning `nil, nil, nil`.

Run: `go test ./internal/mapdef/ -run 'BothLoadPaths|EverySquare'`
Expected: FAIL — no `SceneCreated` found in the standalone output.

- [ ] **Step 3: Implement, and make the adventure delegate**

`mapdef/compile.go` builds `SceneCreated` by calling `Resolve` for every square
in row-major order (deterministic map iteration is required for goldens — sort
the keys), then one `TokenPlaced` per placement.

In `internal/adventure/compile.go`, replace the inline `SceneCreated`
construction with a call into `mapdef.Compile`'s scene half, so **there is
literally one construction site.** In `internal/adventure/load.go`, `sceneJSON`
gains `Tiles`, `Overrides`, `Objects`, and `loadScenes` reuses `mapdef`'s
validation rather than re-implementing it.

- [ ] **Step 4: Run to green, including the existing goldens**

```bash
go test ./internal/mapdef/ ./internal/adventure/...
```
Expected: PASS. The existing `adventures/*/goldens/` may need regenerating —
**inspect the diff before accepting it.** A golden that changes shape is
expected here; a golden that changes *values* is a bug.

- [ ] **Step 5: Commit**

```bash
git add internal/mapdef/ internal/adventure/ adventures/
git commit   # message: one compiler, whichever door the map came in through
```

---

## Task 5: Engine — terrain in state, doors folded, passability queryable

**Files:**
- Modify: `internal/engine/state.go:12-15` (`Scene` gains terrain and door state)
- Modify: `internal/engine/apply.go:60-75` (SceneCreated), and add DoorOpened/DoorClosed arms
- Create: `internal/engine/terrain.go`, `internal/engine/terrain_test.go`

**Interfaces:**
- Consumes: `vttv1.TileRef`, `vttv1.SceneObject`, `DoorOpened`, `DoorClosed` (Task 1).
- Produces:
  ```go
  type Tile struct{ Kind, Material, Art string }
  // on engine.Scene: Tiles map[string]Tile; Objects []SceneObject; OpenDoors map[string]bool
  func (st *State) Blocked(sceneID string, x, y int32) (blocked bool, why string)
  ```

- [ ] **Step 1: Write the failing test**

```go
func TestBlockedAnswersForWallsClosedDoorsAndScenery(t *testing.T) {
	st := stateWithCellar(t) // 3x3: walls around, wood middle row, door at 0,1
	for _, c := range []struct {
		name       string
		x, y       int32
		wantBlock  bool
		wantReason string
	}{
		{"a wall", 0, 0, true, "wall"},
		{"open floor", 2, 1, false, ""},
		{"a closed door", 0, 1, true, "door"},
		{"outside the grid", 9, 9, true, "outside"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, why := st.Blocked("cellar", c.x, c.y)
			if got != c.wantBlock {
				t.Fatalf("Blocked(%d,%d) = %v, want %v", c.x, c.y, got, c.wantBlock)
			}
			if c.wantBlock && !strings.Contains(why, c.wantReason) {
				t.Fatalf("reason %q does not mention %q", why, c.wantReason)
			}
		})
	}
}

func TestOpeningADoorUnblocksItAndSurvivesReplay(t *testing.T) {
	st := stateWithCellar(t)
	if blocked, _ := st.Blocked("cellar", 0, 1); !blocked {
		t.Fatal("the door started open; this test proves nothing")
	}
	must(engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
		DoorOpened: &vttv1.DoorOpened{SceneId: "cellar", At: pos(0, 1)}}}))
	if blocked, why := st.Blocked("cellar", 0, 1); blocked {
		t.Fatalf("an opened door still blocks: %s", why)
	}
	must(engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{
		DoorClosed: &vttv1.DoorClosed{SceneId: "cellar", At: pos(0, 1)}}}))
	if blocked, _ := st.Blocked("cellar", 0, 1); !blocked {
		t.Fatal("a closed door does not block — door state is not folding")
	}
}

func TestSceneryWithBlocksMoveIsImpassable(t *testing.T) {
	st := stateWithCellar(t) // carries a boulder at 1,1 with BlocksMove
	if blocked, why := st.Blocked("cellar", 1, 1); !blocked {
		t.Fatalf("a boulder did not block movement: %q", why)
	}
}
```

- [ ] **Step 2: Run and watch it fail behaviourally**

Add `Blocked` returning `false, ""` and the two Apply arms as `return nil`.

Run: `go test ./internal/engine/ -run 'Blocked|Door|Scenery'`
Expected: FAIL — `Blocked(0,0) = false, want true`.

- [ ] **Step 3: Implement**

```go
// Blocked reports whether a square cannot be entered, and why.
//
// SPATIAL ONLY, deliberately. "A token cannot stand inside solid rock" is the
// same kind of fact as "inside the grid" — it is not a game-system rule, so
// CLAUDE.md rule 5 is untouched. Difficult terrain, flying and phasing are the
// ruleset's business and must never appear here.
//
// An unknown scene blocks. Failing closed is the right direction: refusing a
// move into a scene we cannot describe is recoverable; permitting one is not.
func (st *State) Blocked(sceneID string, x, y int32) (bool, string) {
	sc, ok := st.Scenes[sceneID]
	if !ok {
		return true, fmt.Sprintf("unknown scene %q", sceneID)
	}
	if x < 0 || y < 0 || x >= sc.GridWidth || y >= sc.GridHeight {
		return true, "outside the grid"
	}
	key := fmt.Sprintf("%d,%d", x, y)
	switch t := sc.Tiles[key]; t.Kind {
	case "wall":
		return true, "a wall"
	case "door":
		if !sc.OpenDoors[key] {
			return true, "a closed door"
		}
	}
	for _, o := range sc.Objects {
		if o.BlocksMove && covers(o, x, y) {
			return true, "scenery: " + o.Kind
		}
	}
	return false, ""
}
```

Apply's `SceneCreated` arm copies `Tiles`/`Objects` into the scene and
initialises `OpenDoors` empty (**doors start closed** — a door whose state was
never recorded is shut, which fails closed). The two door arms set and clear the
map entry, erroring on an unknown scene the way every other arm does.

- [ ] **Step 4: Run to green, then fault-inject**

Run: `go test ./internal/engine/`
Expected: PASS.

Inject: make the `door` case return `false, ""` unconditionally. Confirm
`TestOpeningADoorUnblocksItAndSurvivesReplay` fails on the *closed* assertion.
Restore.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/
git commit   # message: the map decides where you can stand, and doors fold
```

---

## Task 6: Gateway — enforce movement for players, free for the DM

**Files:**
- Modify: `internal/gateway/server.go:745-775` (validate before Append)
- Modify: `internal/gateway/convert.go:58-70` (OpenDoor/CloseDoor → events)
- Modify: `internal/gateway/authz.go` (two `commandRoles` rows + adjacency)
- Test: `internal/gateway/server_test.go`, `internal/gateway/authz_test.go`

**Interfaces:**
- Consumes: `engine.State.Blocked` (Task 5); `OpenDoor`/`CloseDoor` (Task 1).
- Produces: no new exported symbols. Behaviour: `MoveToken` from a **player**
  into a blocked square returns `CommandResult{Ok:false}`; from **dm/agent** it
  succeeds.

- [ ] **Step 1: Write the failing test**

```go
func TestAPlayerCannotWalkIntoAWallButTheDMCan(t *testing.T) {
	f := newCellarFixture(t) // player controls act-fighter at 2,1; wall at 0,0
	player := f.dial(f.playerToken, 0)
	res := f.command(player, moveToken("tok-fighter", 0, 0))
	if res.GetOk() {
		t.Fatal("a player walked into a wall — the map does not constrain anyone")
	}
	if !strings.Contains(res.GetError(), "wall") {
		t.Fatalf("the refusal does not say why: %q", res.GetError())
	}

	// The DM is authoring the world, not moving through it (spec §6).
	dm := f.dial(f.dmToken, 0)
	if res := f.command(dm, moveToken("tok-fighter", 0, 0)); !res.GetOk() {
		t.Fatalf("the DM was refused: %q", res.GetError())
	}
}

func TestAPlayerMayOnlyWorkADoorTheyAreNextTo(t *testing.T) {
	f := newCellarFixture(t) // fighter at 2,1; door at 0,1 — two squares away
	player := f.dial(f.playerToken, 0)
	if res := f.command(player, openDoor("cellar", 0, 1)); res.GetOk() {
		t.Fatal("a player opened a door across the room")
	}
	f.moveAsDM("tok-fighter", 1, 1) // now adjacent
	if res := f.command(player, openDoor("cellar", 0, 1)); !res.GetOk() {
		t.Fatalf("an adjacent player was refused: %q", res.GetError())
	}
}

func TestASpectatorMayNotWorkDoors(t *testing.T) {
	f := newCellarFixture(t)
	sp := f.dial(f.spectatorToken, 0)
	if res := f.command(sp, openDoor("cellar", 0, 1)); res.GetOk() {
		t.Fatal("a spectator changed the world")
	}
}
```

- [ ] **Step 2: Run and watch it fail behaviourally**

Add the `commandRoles` rows and the `ToEvent` arms first so the commands are
recognised; leave the movement check out.

Run: `go test ./internal/gateway/ -run 'WalkIntoAWall|WorkADoor|Spectator'`
Expected: FAIL — "a player walked into a wall".

- [ ] **Step 3: Implement**

In `handleCommand`, immediately after `Authorize` and before `ToEvent`:

```go
// The map constrains PLAYERS; the DM and the agent author the world and are
// free of it (spec §6, Patrik: "hard for players, free for DM"). Staging a
// creature inside stone is a legitimate thing for a DM to do.
//
// Checked here rather than in engine.Apply because Apply is the FOLD: by the
// time an event reaches it the move is already history, and history is not the
// place to say no. This is the seam where a command is still a request.
if p.Role == identity.RolePlayer {
	if mt, ok := cmd.GetCommand().(*vttv1.ClientCommand_MoveToken); ok {
		tok, known := st.Tokens[mt.MoveToken.GetTokenId()]
		if known {
			to := mt.MoveToken.GetTo()
			if blocked, why := st.Blocked(tok.SceneID, to.GetX(), to.GetY()); blocked {
				return &vttv1.CommandResult{
					RequestId: requestID, Ok: false,
					Error: "gateway: cannot move there — " + why,
				}
			}
		}
	}
}
```

Adjacency for doors goes in `authz.go` beside the role table, since it is a
question about *who may*, not about what the world is:

```go
// A player may work a door only if a token they control stands next to it.
// Otherwise anyone could fling open a door across the dungeon. Spatial, not
// game-system: it asks where you are, not what edition you are playing.
func mayWorkDoor(p *identity.Participant, st *engine.State, sceneID string, at *vttv1.GridPosition) error {
	if p.Role != identity.RolePlayer {
		return nil
	}
	for _, tok := range st.Tokens {
		if tok.SceneID != sceneID || !controls(st, p.ID, tok.ActorID) {
			continue
		}
		if abs(tok.X-at.GetX()) <= 1 && abs(tok.Y-at.GetY()) <= 1 {
			return nil
		}
	}
	return errors.New("gateway: no token you control is next to that door")
}
```

- [ ] **Step 4: Run to green, then fault-inject**

```bash
go test ./internal/gateway/
```
Expected: PASS, including the existing `HasRoleCellsForTest` reflection test —
it will fail if either new command is missing a role row, so **do not** special-case it.

Inject: change `p.Role == identity.RolePlayer` to `p.Role == identity.RoleDM`.
Confirm both halves of the first test fail (player permitted, DM refused).
Restore.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/
git commit   # message: players are constrained by the world; the DM authors it
```

---

## Task 7: Serving maps and packs

**Files:**
- Modify: `cmd/vtt/serve.go` (add `--maps-dir`, load at boot)
- Modify: `internal/gateway/metadata.go` (serve pack files)
- Modify: `internal/gateway/server.go` (`WithMaps` option)
- Test: `internal/gateway/metadata_test.go`, `cmd/vtt` tests

**Interfaces:**
- Consumes: `mapdef.Load`, `mapdef.LoadPack` (Tasks 2–3).
- Produces: `func (s *Server) WithMaps(m map[string]*mapdef.Map, packs map[string]*mapdef.Pack) *Server`;
  routes `GET /api/maps`, `GET /api/packs/{pack}/{file}`.

- [ ] **Step 1: Write the failing test**

```go
func TestPackImagesAreServedAndUnknownOnesAre404(t *testing.T) {
	f := newGatewayWithPack(t)
	if code, body := f.get("/api/packs/mossy-keep/planks_03.png"); code != 200 {
		t.Fatalf("pack image returned %d: %s", code, body)
	}
	if code, _ := f.get("/api/packs/mossy-keep/../../etc/passwd"); code == 200 {
		t.Fatal("path traversal escaped the pack directory")
	}
	if code, _ := f.get("/api/packs/no-such-pack/x.png"); code != 404 {
		t.Fatalf("unknown pack returned %d, want 404", code)
	}
}

func TestBootRefusesAnInvalidMapRatherThanServingIt(t *testing.T) {
	// adventure-format §7's posture: fail loud at startup, not at the table.
	_, err := cmdvtt.LoadMapsDir("testdata/maps-with-one-broken")
	if err == nil {
		t.Fatal("a broken map loaded; the table would find out instead of us")
	}
}
```

- [ ] **Step 2: Run and watch it fail**

Run: `go test ./internal/gateway/ ./cmd/vtt/ -run 'PackImages|BootRefuses'`
Expected: FAIL — 404 on the valid image (no route yet).

- [ ] **Step 3: Implement**

Serve pack files from an `fs.FS` rooted at each pack directory, so traversal is
impossible **by construction** rather than by sanitising the path — `fs.FS`
rejects `..` itself. `LoadMapsDir` loads and validates every map at boot and
returns the first error with its file path.

- [ ] **Step 4: Run to green**

```bash
go test ./internal/gateway/ ./cmd/vtt/
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/vtt/ internal/gateway/
git commit   # message: serve maps and packs, and refuse a broken one at boot
```

---

## Task 8: Client — terrain in state, and a pure scene plan

**Files:**
- Modify: `client/src/state.ts:17-22` (`Scene` gains `Tiles`, `Objects`, `OpenDoors`)
- Modify: `client/src/fold.ts` (SceneCreated, DoorOpened, DoorClosed)
- Create: `client/src/view/camera.ts`, `client/src/view/scene-plan.ts`
- Test: `client/test/scene-plan.test.ts`, `client/test/camera.test.ts`

**Interfaces:**
- Consumes: the wire shapes from Task 1.
- Produces:
  ```ts
  export interface Camera { scale: number; offsetX: number; offsetY: number }
  export function fitCamera(sceneW: number, sceneH: number, cell: number,
                            viewW: number, viewH: number): Camera
  export function worldFromScreen(px: number, py: number, cam: Camera): {x: number; y: number}
  export interface DrawOp { image: string; sx: number; sy: number; sw: number; sh: number; rot: number }
  export function planScene(st: State, sceneId: string, cam: Camera,
                            cell: number, viewW: number, viewH: number): DrawOp[]
  ```

- [ ] **Step 1: Write the failing test**

```ts
test("a scene bigger than the viewport is scaled to fit, not cropped", () => {
  const cam = fitCamera(32, 32, 44, 900, 600);
  // 32*44 = 1408 world px into 600 of height — the binding dimension.
  expect(cam.scale).toBeCloseTo(600 / 1408, 4);
  expect(cam.scale).toBeLessThan(1);
});

test("clicking maps back to the square under the cursor at any zoom", () => {
  const cam = fitCamera(10, 10, 44, 440, 440);          // 1:1
  expect(worldFromScreen(45, 45, cam)).toEqual({ x: 1, y: 1 });
  const zoomed = { ...cam, scale: 0.5 };
  expect(worldFromScreen(45, 45, zoomed)).toEqual({ x: 2, y: 2 });
});

test("planScene emits one op per visible square and culls the rest", () => {
  const st = cellarState();            // 3x3
  const cam = fitCamera(3, 3, 44, 132, 132);
  const ops = planScene(st, "cellar", cam, 44, 132, 132);
  expect(ops.filter(o => o.image.startsWith("tile:")).length).toBe(9);

  // Half the map off-screen: culled, not drawn and clipped.
  const panned = { ...cam, offsetX: -88 };
  const fewer = planScene(st, "cellar", panned, 44, 132, 132);
  expect(fewer.length).toBeLessThan(ops.length);
});

test("an override changes the image and nothing else", () => {
  const st = cellarState();
  const ops = planScene(st, "cellar", fitCamera(3, 3, 44, 132, 132), 44, 132, 132);
  const at11 = ops.find(o => o.sx === 44 && o.sy === 44)!;
  expect(at11.image).toBe("tile:planks-split-3");
});
```

- [ ] **Step 2: Run and watch it fail behaviourally**

Stub `fitCamera` returning `{scale:1,offsetX:0,offsetY:0}` and `planScene`
returning `[]`.

Run: `bun test client/test/camera.test.ts client/test/scene-plan.test.ts`
Expected: FAIL — `expected 0.426…, received 1`.

- [ ] **Step 3: Implement**

Both files are **pure**: no DOM, no canvas, no globals. `planScene` walks the
squares intersecting the camera's viewport, emits a `DrawOp` per square and per
object, and resolves each image name to `tile:<name>` or `std:<kind>/<material>`
so the drawing layer needs no knowledge of resolution rules.

- [ ] **Step 4: Run to green, then fault-inject**

Run: `bun test client/test/`
Expected: PASS.

Inject: remove the culling condition in `planScene`. Confirm the culling test
fails. Restore.

- [ ] **Step 5: Commit**

```bash
git add client/src client/test
git commit   # message: what to draw is a pure function; nothing here touches a canvas
```

---

## Task 9: Client — the canvas layer, and the board stops driving page height

**Files:**
- Create: `client/src/view/canvas.ts`
- Modify: `client/src/view/spectator.ts:68-90` (`renderGrid` draws through the camera)
- Modify: `client/src/style.css:24-36` (board pane is fixed-size; map scrolls inside it)
- Test: `client/test/spectator-view.test.ts`

**Interfaces:**
- Consumes: `DrawOp`, `Camera`, `planScene`, `fitCamera` (Task 8).
- Produces: `export function paint(ctx: CanvasRenderingContext2D, ops: DrawOp[], images: ImageMap): void`

- [ ] **Step 1: Write the failing test**

happy-dom has **no canvas**, so nothing here asserts pixels. What it can assert
is the layout defect that made this arc necessary:

```ts
test("the board does not size itself to the scene", () => {
  // Backlog T1/#19: the board was gridWidth*CELL px tall (1408 for 32x32), so
  // the controls sat ~1450px down the page, below every laptop fold.
  const root = document.createElement("div");
  renderSpectator(root, bigSceneState(32, 32), [], "open", {});
  const board = root.querySelector(".board") as HTMLElement;
  expect(board.style.height).toBe("");        // not "1408px"
  expect(board.style.width).toBe("");
});

test("paint issues one drawImage per op, in order", () => {
  const calls: string[] = [];
  const ctx = fakeCtx(calls);                 // records drawImage/save/restore
  paint(ctx, [
    { image: "tile:a", sx: 0,  sy: 0,  sw: 44, sh: 44, rot: 0 },
    { image: "tile:b", sx: 44, sy: 0,  sw: 44, sh: 44, rot: 0 },
  ], stubImages());
  expect(calls).toEqual(["drawImage:tile:a", "drawImage:tile:b"]);
});
```

- [ ] **Step 2: Run and watch it fail behaviourally**

Run: `bun test client/test/spectator-view.test.ts`
Expected: FAIL — `expected "", received "1408px"`. **That failure is backlog
T1/#19 reproduced as a test**, which is why this task closes it.

- [ ] **Step 3: Implement**

`canvas.ts` is deliberately thin — a loop over ops calling `save`, `translate`,
`rotate`, `drawImage`, `restore`, and nothing else. All decisions were made in
Task 8.

`renderGrid` stops setting pixel width/height on `.board` and mounts a canvas
sized to the *pane*; `style.css` gives `.board` a `min-height: 0` inside the grid
row so an oversized scene can never push `.player` down the page. Wheel zooms,
drag pans, and the camera refits when the scene changes.

Add a comment on `paint` recording why it is this small:

```ts
// Deliberately thin. happy-dom has NO canvas implementation, so nothing this
// function does can be asserted by the suite — which is exactly how the
// participant list shipped rendering as "ArmakAsmeDM" behind a passing test
// (backlog #13). Every decision therefore lives in planScene, which is pure and
// fully tested; this loop is small enough to verify by reading.
```

- [ ] **Step 4: Run to green, then check by eye**

```bash
bun test client/test/
task client:typecheck
```
Expected: PASS. Then `task client:dev`, load the demo map, and **actually look
at it** — that is the only check that covers the canvas, and the reason the
split exists.

- [ ] **Step 5: Commit**

```bash
git add client/
git commit   # message: draw through a camera, and stop letting the map size the page
```

---

## Task 10: The demo map, the API document, and the LLM test

**Files:**
- Create: `maps/cellar/map.json`, `maps/cellar/tiles/pack.json` + images
- Create: `docs/map-format.md`
- Modify: `README.md` (name the maps directory)
- Test: `internal/mapdef/apidoc_test.go`

**Interfaces:**
- Consumes: everything above.
- Produces: `maps/cellar` — a playable map **with cover**, which the visibility
  arc will use as its fixture.

- [ ] **Step 1: Write the failing test**

```go
// The API document is the deliverable (spec §1.5). If it drifts from the code,
// the LLM that reads it produces maps that do not load — and nothing else in
// the suite would notice.
func TestEveryStandardTileIsDocumented(t *testing.T) {
	doc, err := os.ReadFile("../../docs/map-format.md")
	if err != nil {
		t.Fatal(err)
	}
	for name := range mapdef.StandardTileNames() {
		if !strings.Contains(string(doc), "`"+name+"`") {
			t.Fatalf("standard tile %q is not in docs/map-format.md — an author "+
				"reading the docs cannot know it exists", name)
		}
	}
}

func TestTheDemoMapLoadsAndHasCover(t *testing.T) {
	m, err := mapdef.Load("../../maps/cellar/map.json")
	if err != nil {
		t.Fatalf("the demo map does not load: %v", err)
	}
	var blockers int
	for _, o := range m.Objects {
		if o.BlocksSight {
			blockers++
		}
	}
	if blockers == 0 {
		t.Fatal("the demo map has no cover — the visibility arc would have " +
			"nothing to hide behind, which is how goblin-ambush failed")
	}
}
```

- [ ] **Step 2: Run and watch it fail**

Run: `go test ./internal/mapdef/ -run 'Documented|DemoMap'`
Expected: FAIL — `docs/map-format.md` does not exist.

- [ ] **Step 3: Write the map and the document**

`docs/map-format.md` is written **for a reader with no access to this
repository**: the two layers, the `"x,y"` key convention, every standard tile
name with its kind and material, the pack manifest shape including `desc`, the
object fields, and a complete worked example. State plainly that art never
decides nature.

`maps/cellar` is a small dungeon room with pillars and crates — enough cover
that an ambush is possible, which `goblin-ambush` never had.

- [ ] **Step 4: Run to green, then run the real exit criterion**

```bash
go test ./internal/mapdef/
task check > /tmp/check.log 2>&1; echo "EXIT=$?"     # never pipe into tail
```

Then **exit criterion 6**: give a fresh LLM session *only* `docs/map-format.md`
and `maps/cellar/tiles/pack.json`, ask for a new map, and load the result. If it
fails, that is the arc's stated risk (spec §9) arriving — record what went wrong
and add the ASCII shorthand, which is additive and changes no semantics.

- [ ] **Step 5: Commit**

```bash
git add maps/ docs/ README.md internal/mapdef/
git commit   # message: the format is the API, so here is the document and a map that proves it
```

---

## Self-review notes

**Spec coverage.** §3.1 hybrid structure → Tasks 2, 8. §3.2 art has no authority
→ Task 3 (with the injection that proves it). §3.3 standard vocabulary and doors
as one nature → Tasks 2, 5. §3.4 objects are scenery → Task 1's comment plus
Task 5's `BlocksMove`. §4.1 two keyed layers → Task 2. §4.2 packs, two levels →
Task 3. §4.3 standalone maps, one compiler → Tasks 4, 7. §4.4 boot validation →
Tasks 2, 7. §5 contract → Task 1. §6 movement and doors → Tasks 5, 6. §7
rendering and camera → Tasks 8, 9. §8 the canvas-blindness split → Tasks 8, 9.
§10 exit criteria → Tasks 9, 10.

**Not covered by any task, deliberately:** the `warnings` returned by `Resolve`
are produced and tested but only surfaced at load; wiring them into the DM
console is not in this arc.

**Known risk carried from spec §9:** keyed-only authoring may defeat the LLM.
Task 10 step 4 tests it directly rather than assuming either way.
