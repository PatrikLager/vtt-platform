# Art Is A Flat Library — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete packs. Art becomes one flat `campaign/art/` directory where the
filename is the identity, any map may use any art, and unresolvable art degrades
one square to the built-in vocabulary instead of refusing the map.

**Architecture:** A new `internal/artlib` resolves an art id to a piece by
filename — no scan, no registry, no cache. `mapdef.Resolve` takes that library
instead of a `*Pack` and warns instead of erroring when a name does not resolve.
The gateway reads art at map-load time, so there is no boot-time art load to get
wrong. `mapdef.Pack` and the whole pack walk are deleted last, once nothing calls
them.

**Tech Stack:** Go 1.24+ (`os.OpenRoot` for symlink-safe FS), protobuf/buf,
TypeScript + bun for the client, Taskfile gates, gremlins + Stryker mutation.

**Spec:** `docs/superpowers/specs/2026-09-02-art-is-a-flat-library-design.md`

## Global Constraints

- **`art/` is FLAT.** No subdirectories, ever. A subfolder is a namespace and a
  namespace is a pack (spec §3.3).
- **The filename stem IS the art id.** Nothing declares an id. Stems are
  kebab-case; picture and sidecar share a stem.
- **Uniqueness is the filesystem's.** No duplicate check is written anywhere.
- **A sidecar is REQUIRED for tile art, OPTIONAL for object art** (spec §3.4).
- **Unresolvable art degrades ONE square** to its `tiles` kind/material and
  warns; it never refuses the map (spec §4).
- **Nature always comes from `m.Tiles`.** Art never decides a square's kind or
  material. This is unchanged and must stay true.
- **`engine.Apply` remains the only fold** (CLAUDE.md rule 4).
- **Airtight TDD (ADR-009):** tests first, behavioural RED before the solution;
  after-the-fact assertions need fault-injection proof.
- **`task check` is the single gateway** (CLAUDE.md rule 2). Never weaken a gate.
- **Contract additive only** (ADR-007). Exactly one contract addition is planned:
  `CommandResult.warnings`. The `"pack"` field removal is a breaking change and
  is permissible ONLY because `contract/RELEASED` does not exist; this must land
  before that file does (spec §7).
- **Citations name durable things** (CLAUDE.md rule 8).
- `cell_px` default is **64** when `campaign/campaign.json` is absent.

---

## Pre-flight finding: the adventure path, which the spec does not mention

**Found by the pre-flight scan, 2026-09-03, before Task 1.** `internal/adventure`
is a second consumer of everything this sub-project changes, and neither the spec
nor the first draft of this plan named it:

- `Adventure.Pack *mapdef.Pack` (`internal/adventure/format.go`) — an adventure
  carries its OWN embedded art, loaded from `<adventure>/tiles/pack.json` by
  `loadEmbeddedPack` (`internal/adventure/load.go`).
- `internal/adventure/compile.go` calls `mapdef.BuildSceneCreated(sc.asMap(), adv.Pack)`.
- `internal/adventure/load.go` calls `BuildSceneCreated` again as a dry run.

So spec §6's deletion of `mapdef.Pack` and `LoadPack` would remove how adventures
ship art, and spec §3's flat `campaign/art/` says nothing about art that travels
*inside* a bundle.

**Ruling (controller, 2026-09-03): an adventure keeps its art self-contained, in
its own flat `<adventure>/art/`, resolved by the SAME `artlib` rooted at the
adventure directory.**

*Why:* an adventure is a bundle you hand to someone, and its art must travel with
it — that is what `loadEmbeddedPack` exists for and it should not be lost. The
alternative, installing an adventure's art into `campaign/art/` on load, collides
with the flat namespace by construction: two adventures shipping `masonry-1`
would fight, and the DM never chose either name. Two roots and one mechanism
keeps the flat rule intact inside each root, needs no namespace, and means
`artlib` is the only art code in the tree.

*What it costs if wrong:* the adventure format changes shape
(`tiles/pack.json` → `art/`), which is breaking and touches `adventures/*/` on
disk. Cheap to revisit while `contract/RELEASED` is absent; expensive after.

**This is a spec gap, so under CLAUDE.md rule 7 the spec is amended at the merge
gate and the amendment needs Patrik's approval.** Flagged to him at dispatch
time rather than after.

**Plan changes this forces:** Task 3 must update `internal/adventure`'s call
sites or the tree will not compile, and Task 7 must replace the embedded pack
rather than merely delete it. Both are written into those tasks below.

---

## Design decision this plan settles

**Spec §3.1 says a directory under `art/` is an error, and spec §3.6 says art
resolves on demand. Taken naively those conflict:** a scan-free lookup by
filename never sees a stray directory, and a scan on every map load is the
boot-time load this design exists to remove, just moved.

**Resolution: two entry points, one cheap and one rare.**

- `artlib.Lookup(dir, id)` — the hot path. Reads `<dir>/<id>.json` and
  `<dir>/<id>.png` directly. **No `ReadDir`, no cache, no registry.** This is
  what a map load uses, and it is what makes "the filename IS the identity"
  literally true rather than merely enforced.
- `artlib.Validate(dir)` — one `ReadDir` plus a read of each SIDECAR (not of
  any picture). Refuses a subdirectory, a symlink, a filename that is not a
  legal art id, and a sidecar whose named pictures are missing. Called by
  `vtt art install` and once at server start.

  *(That list was incomplete when first written — it named only the
  subdirectory and missing-picture cases. Corrected 2026-09-03 after the fixer
  checked it against the tree rather than against this sentence.)*

  **Amended 2026-09-03, Task 1 fix round.** This said "no file contents" until
  C1 proved that impossible: a door names its two pictures INSIDE its sidecar
  (`cellar-door-open.png`, `cellar-door-closed.png`, and no `cellar-door.png`),
  so checking that a sidecar's pictures exist requires reading the sidecar.
  `Validate` now resolves each sidecar through `Lookup` itself, so install and
  map-load cannot diverge on what is legal. The consequence is real and is
  accepted: **a malformed sidecar now fails server start.** Spec §5 already
  asks for exactly that ("an unreadable sidecar, an unsupported
  `format_version`" are named as loader refusals), and a LOUD boot failure on
  broken data is not the defect sub-project 15 shipped — that one was a SILENT
  gate on one directory suppressing the load of another. Pictures are still
  never read.

The boot call is a **shape check, not a load**: it reads no art, caches nothing,
and a missing `art/` passes. So it cannot reproduce sub-project 15's boot-order
defect, where a guard on one directory silently gated the loading of another.

---

## File Structure

**Created**

- `internal/artlib/artlib.go` — `Piece`, `Lookup`, `Validate`, `ErrNotFound`.
- `internal/artlib/artlib_test.go` — the package's own tests.
- `internal/campaigncfg/campaigncfg.go` — `Config{CellPx}`, `Load(dir)`.
- `campaigns/example/art/` — the migrated example art.
- `campaigns/example/campaign.json`
- `tools/check-no-pack.py`, `tools/check_no_pack_test.py` — the removal gate.

**Modified**

- `internal/mapdef/resolve.go` — `Resolve` takes an art lookup; degrades + warns.
- `internal/mapdef/compile.go` — threads the library through `BuildSceneCreated`.
- `internal/mapdef/format.go` — `Map.Pack` deleted; `Pack`/`PackTile` deleted.
- `internal/mapdef/load.go` — refuse a map declaring `"pack"`; `LoadPack` deleted.
- `internal/mapdef/installed.go` — `LoadInstalled` takes an art dir.
- `internal/gateway/server.go` — `WithArtDir`; `packs`/`packFS`/`WithPackFiles` deleted.
- `internal/gateway/map.go` — resolve art on demand; `ErrPackNotLoaded` arm deleted.
- `internal/gateway/metadata.go` — `packRefJSON` deleted; `cellPx` reported directly.
- `cmd/vtt/maps.go` — the pack walk deleted.
- `cmd/vtt/serve_compose.go` — the `os.Stat(maps)` guard deleted.
- `cmd/vtt/art.go` — new `vtt art install` subcommand.
- `contract/vtt/v1/commands.proto` — `CommandResult.warnings = 5`.
- `client/src/metadata.ts`, `client/src/wire.ts` — `cellPx` shape; warnings surfaced.
- `tools/genmappack/` — emits flat art.
- `Taskfile.yml` — `check:no-pack`.

**Deleted**

- `campaigns/example/packs/`, every `pack.json`, `mapdef.Pack`, `mapdef.PackTile`,
  `mapdef.LoadPack`, `ErrPackNotLoaded`, `Server.packs`, `Server.packFS`,
  `WithPackFiles`, `packRefJSON`, `loadMapsDir`'s pack half.

---

## Shared helpers used by several tasks

Define once; later tasks reference them by name.

```go
// internal/artlib/artlib_test.go — used by every test in this package.
func writeArt(t *testing.T, dir, stem, sidecar string) {
	t.Helper()
	if sidecar != "" {
		if err := os.WriteFile(filepath.Join(dir, stem+".json"), []byte(sidecar), 0o644); err != nil {
			t.Fatalf("write sidecar %s: %v", stem, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, stem+".png"), []byte("png bytes"), 0o644); err != nil {
		t.Fatalf("write picture %s: %v", stem, err)
	}
}
```

---

### Task 1: The art library

**Files:**
- Create: `internal/artlib/artlib.go`, `internal/artlib/artlib_test.go`

**Interfaces:**
- Produces: `artlib.Piece{ID, Kind, Material, File, Open, Closed string}`;
  `artlib.Lookup(dir, id string) (Piece, error)`;
  `artlib.Validate(dir string) error`; `artlib.ErrNotFound`.
- Consumes: nothing.

- [ ] **Step 1: Write the failing tests**

```go
package artlib_test

func TestObjectArtNeedsNoSidecar(t *testing.T) {
	dir := t.TempDir()
	writeArt(t, dir, "pillar-stone", "")
	p, err := artlib.Lookup(dir, "pillar-stone")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.ID != "pillar-stone" || p.File != "pillar-stone.png" || p.Kind != "" {
		t.Fatalf("got %+v, want object art with an empty Kind", p)
	}
}

func TestTileArtDeclaresItsNature(t *testing.T) {
	dir := t.TempDir()
	writeArt(t, dir, "masonry-1", `{"format_version":1,"kind":"wall","material":"stone"}`)
	p, err := artlib.Lookup(dir, "masonry-1")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Kind != "wall" || p.Material != "stone" {
		t.Fatalf("got %+v, want kind=wall material=stone", p)
	}
}

func TestADoorCarriesBothPictures(t *testing.T) {
	dir := t.TempDir()
	writeArt(t, dir, "cellar-door", `{"format_version":1,"kind":"door","material":"wood",
		"open":"cellar-door-open.png","closed":"cellar-door-closed.png"}`)
	p, err := artlib.Lookup(dir, "cellar-door")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Open != "cellar-door-open.png" || p.Closed != "cellar-door-closed.png" {
		t.Fatalf("got %+v, want both door pictures", p)
	}
}

func TestMissingArtIsErrNotFoundSoCallersCanDegrade(t *testing.T) {
	dir := t.TempDir()
	_, err := artlib.Lookup(dir, "nothing-here")
	if !errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound — Resolve distinguishes absent art (degrade) "+
			"from malformed art (refuse) by this sentinel", err)
	}
}

func TestAnUnsupportedFormatVersionIsRefusedNotDegraded(t *testing.T) {
	dir := t.TempDir()
	writeArt(t, dir, "future", `{"format_version":2,"kind":"wall","material":"stone"}`)
	_, err := artlib.Lookup(dir, "future")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want a refusal that is NOT ErrNotFound: art that exists and "+
			"cannot be read is a defect to fix, not a square to draw plain", err)
	}
	if !strings.Contains(err.Error(), "declares 2") || !strings.Contains(err.Error(), "understands 1") {
		t.Fatalf("error %q must name both versions", err)
	}
}

func TestASidecarWithNoPictureIsRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "orphan.json"),
		[]byte(`{"format_version":1,"kind":"wall","material":"stone"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := artlib.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "orphan") {
		t.Fatalf("got %v, want a refusal naming orphan", err)
	}
}

func TestASubdirectoryIsRefusedByName(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "cellar-basics"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := artlib.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "cellar-basics") {
		t.Fatalf("got %v, want a refusal naming cellar-basics — a subfolder is a "+
			"namespace and a namespace is a pack (spec §3.3)", err)
	}
}

func TestValidateAcceptsAnAbsentArtDirectory(t *testing.T) {
	if err := artlib.Validate(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatalf("Validate on an absent art/: %v — a campaign with no art yet is "+
			"ordinary, and refusing it would reproduce the boot-order defect this "+
			"design removes", err)
	}
}

func TestLookupRefusesAnIdThatIsNotAFilename(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"../secret", "a/b", "", ".", "..", "x/../../y"} {
		if _, err := artlib.Lookup(dir, id); err == nil {
			t.Fatalf("Lookup(%q) was accepted; an id becomes a path and must be a "+
				"bare filename", id)
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/artlib/ -count=1`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement**

```go
// Package artlib reads art out of a campaign's flat art/ directory.
//
// THE FILENAME IS THE IDENTITY. Nothing declares an id, so nothing can
// disagree with one (design spec §3.2). That is why Lookup does no ReadDir:
// knowing the id IS knowing the path, and a scan would only be a slower way to
// arrive at the same filename — while quietly becoming the boot-time load this
// design exists to delete.
//
// UNIQUENESS IS THE FILESYSTEM'S (spec §3.3). There is no duplicate check in
// this package because two files cannot share a name. Subdirectories are
// refused for exactly that reason: art/a/x.png and art/b/x.png coexist happily,
// and the moment they can, something has to decide what "x" means.
package artlib

const FormatVersion int32 = 1

var ErrNotFound = errors.New("artlib: no such art")

// Piece is one art entry. Kind == "" means OBJECT art, which needs no sidecar
// because the map already declares an object's blocking behaviour (spec §3.4).
type Piece struct {
	ID                 string
	Kind, Material     string
	File               string
	Open, Closed       string
}

type sidecar struct {
	FormatVersion int32  `json:"format_version"`
	Kind          string `json:"kind"`
	Material      string `json:"material"`
	Open          string `json:"open"`
	Closed        string `json:"closed"`
}

// idIsAFilename mirrors mapdef's guard of the same shape: an id arriving from a
// map file becomes a path, so it must be one path SEGMENT and nothing else.
func idIsAFilename(id string) bool {
	return id != "" && id != "." && id != ".." &&
		!strings.ContainsAny(id, `/\`) && filepath.Base(id) == id
}

func Lookup(dir, id string) (Piece, error) {
	if !idIsAFilename(id) {
		return Piece{}, fmt.Errorf("artlib: art id %q must be a plain filename", id)
	}
	raw, err := os.ReadFile(filepath.Join(dir, id+".json"))
	switch {
	case err == nil:
		var sc sidecar
		if jsonErr := json.Unmarshal(raw, &sc); jsonErr != nil {
			return Piece{}, fmt.Errorf("artlib: art/%s.json: %w", id, jsonErr)
		}
		if sc.FormatVersion != FormatVersion {
			return Piece{}, fmt.Errorf(
				"artlib: art/%s.json: field \"format_version\": declares %d; this server understands %d",
				id, sc.FormatVersion, FormatVersion)
		}
		p := Piece{ID: id, Kind: sc.Kind, Material: sc.Material,
			Open: sc.Open, Closed: sc.Closed, File: id + ".png"}
		if sc.Kind == "door" && (sc.Open == "" || sc.Closed == "") {
			return Piece{}, fmt.Errorf(
				"artlib: art/%s.json: a door declares both \"open\" and \"closed\"", id)
		}
		return p, nil
	case errors.Is(err, fs.ErrNotExist):
		// No sidecar: object art, if the picture is there.
		if _, statErr := os.Stat(filepath.Join(dir, id+".png")); statErr != nil {
			return Piece{}, fmt.Errorf("artlib: art/%s: %w", id, ErrNotFound)
		}
		return Piece{ID: id, File: id + ".png"}, nil
	default:
		return Piece{}, fmt.Errorf("artlib: art/%s.json: %w", id, err)
	}
}

// Validate is a SHAPE check, not a load: one ReadDir, no file contents, no
// cache. An absent art/ passes, because a campaign with no art yet is ordinary.
func Validate(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("artlib: read art dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			return fmt.Errorf(
				"artlib: art dir %s contains a subdirectory %q; art/ is flat — "+
					"a subfolder is a namespace, and a namespace is a pack", dir, e.Name())
		}
		if strings.HasSuffix(e.Name(), ".json") {
			stem := strings.TrimSuffix(e.Name(), ".json")
			if _, statErr := os.Stat(filepath.Join(dir, stem+".png")); statErr != nil {
				return fmt.Errorf(
					"artlib: art dir %s: %s has no picture beside it (%s.png)", dir, e.Name(), stem)
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/artlib/ -count=1 -v`
Expected: PASS, all nine.

- [ ] **Step 5: Commit**

```bash
git add internal/artlib/
git commit -m "Art is found by its filename, not by a registry"
```

---

### Task 2: Warnings reach the client

The channel spec §4 depends on does not exist: `mapdef.Compile`/`Resolve` return warnings and
no contract message carries them, so today a warning dies in Go. This is the one
additive contract change (ADR-007).

**Files:**
- Modify: `contract/vtt/v1/commands.proto`, `internal/gateway/` command result
  construction, `client/src/wire.ts`
- Test: `internal/gateway/map_test.go`, `client/test/wire.test.ts`, `contract/`

**Interfaces:**
- Produces: `CommandResult.warnings` (repeated string, field 5).

- [ ] **Step 1: Write the failing test**

```go
func TestALoadMapWarningReachesTheIssuer(t *testing.T) {
	// A map naming art that is not installed loads, and the DM is TOLD.
	// Silence here is worse than the refusal it replaces (spec §4).
	f := newInstallableMapFixture(t)
	writeMap(t, f.mapsDir, "shrine", `{"format_version":1,"id":"shrine","name":"Shrine",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},"overrides":{"0,0":"absent-art"}}`)
	res := f.loadMap(t, "shrine")
	if !res.GetOk() {
		t.Fatalf("load_map refused: %s — unresolvable art degrades, it does not refuse", res.GetError())
	}
	if len(res.GetWarnings()) == 0 {
		t.Fatal("no warnings: the square rendered plain and nobody was told why")
	}
	if !strings.Contains(strings.Join(res.GetWarnings(), "\n"), "absent-art") {
		t.Fatalf("warnings %q must name the reference that did not resolve", res.GetWarnings())
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/gateway/ -run TestALoadMapWarningReachesTheIssuer -count=1`
Expected: FAIL — `res.GetWarnings` undefined.

- [ ] **Step 3: Add the field, additively**

In `contract/vtt/v1/commands.proto`, `message CommandResult`:

```proto
message CommandResult {
  string request_id = 1;
  bool ok = 2;
  string error = 3;
  int64 sequence = 4;

  // Non-fatal facts about a command that SUCCEEDED. load_map fills it when a
  // map names art that is not installed: that square renders from its tiles
  // kind and material (art design spec §4) and this says which references
  // were dropped. Without it §4 is a silence — the map goes plain and nobody
  // learns why — which is strictly worse than the refusal it replaces.
  repeated string warnings = 5;
}
```

Run: `task generate:contract`

- [ ] **Step 4: Plumb it**

Carry `mapdef`'s existing `[]string` warnings from the load path onto the
`CommandResult` the issuer receives. Do not broadcast them: a warning is for
whoever issued the command, not for the table.

- [ ] **Step 5: Run tests and the client half**

Run: `go test ./internal/gateway/ -count=1 && bun test client/test contract`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "A warning that dies in Go is a silence"
```

---

### Task 3: Resolve degrades instead of refusing

This is the sharpest seam in the sub-project. `Resolve` already returns
`Resolved{Kind, Material}` with no `Art` when a square has no override — the
degrade path exists and is exercised. This task routes the *unresolvable* case
into it.

**Files:**
- Modify: `internal/mapdef/resolve.go`, `internal/mapdef/compile.go`,
  `internal/adventure/compile.go`, `internal/adventure/load.go`,
  `internal/gateway/map.go`, `internal/mapdef/installed.go`
- Test: `internal/mapdef/resolve_test.go`, `internal/adventure/compile_test.go`

**`internal/gateway/map.go` and `internal/mapdef/installed.go` are on this list
because they CALL `mapdef.Compile`** (added 2026-09-03 after Task 2's review
found the omission). Changing `Resolve`/`Compile`'s signature without them
leaves the tree non-compiling until Task 4, and this task's own commit step
stages `internal/mapdef/` alone. Stage everything that must compile together.

**The adventure path compiles or nothing does.** `internal/adventure/compile.go`
calls `mapdef.BuildSceneCreated(sc.asMap(), adv.Pack)` and
`internal/adventure/load.go` calls it again as a dry run. Changing `Resolve` and
`BuildSceneCreated` breaks both immediately. For THIS task, pass the adventure's
art directory (`<adventure>/art/`) as the art root; the embedded pack itself is
replaced in Task 7. If that directory does not exist yet, every override in an
adventure scene degrades and warns — which is correct and temporary, and Task 8
installs the files.

**Interfaces:**
- Consumes: `artlib.Lookup`, `artlib.ErrNotFound` (Task 1).
- Produces: `func Resolve(m *Map, artDir, square string) (Resolved, []string, error)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestArtThatIsNotInstalledDegradesTheSquareAndWarns(t *testing.T) {
	artDir := t.TempDir()
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "absent"}}
	got, warnings, err := mapdef.Resolve(m, artDir, "0,0")
	if err != nil {
		t.Fatalf("Resolve: %v — absent art degrades, it does not refuse (spec §4)", err)
	}
	if got.Kind != "wall" || got.Material != "stone" {
		t.Fatalf("got %+v, want the nature from Tiles, unchanged", got)
	}
	if got.Art != "" {
		t.Fatalf("got Art=%q, want empty: there is no picture to draw", got.Art)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "absent") {
		t.Fatalf("warnings %q must name the reference", warnings)
	}
}

func TestArtThatExistsButCannotBeReadStillRefuses(t *testing.T) {
	// The distinction that makes §4 safe: ABSENT art is a square to draw
	// plain; MALFORMED art is a defect to fix. Degrading both would let a
	// broken sidecar ship silently.
	artDir := t.TempDir()
	writeArt(t, artDir, "broken", `{"format_version":99,"kind":"wall","material":"stone"}`)
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "broken"}}
	if _, _, err := mapdef.Resolve(m, artDir, "0,0"); err == nil {
		t.Fatal("malformed art was degraded; it must refuse")
	}
}

func TestASquareWithNoOverrideIsUnchanged(t *testing.T) {
	// The control. This path predates the change and must not move.
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "earth"}}
	got, warnings, err := mapdef.Resolve(m, t.TempDir(), "0,0")
	if err != nil || len(warnings) != 0 || got.Kind != "floor" || got.Art != "" {
		t.Fatalf("got %+v warnings=%v err=%v; the no-override path must not change",
			got, warnings, err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/mapdef/ -run 'Degrades|CannotBeRead|NoOverride' -count=1`
Expected: FAIL — `Resolve` still takes `*Pack`.

- [ ] **Step 3: Change the signature and the two error returns**

Replace the `p == nil` refusal and the `m.Pack != p.ID` refusal with a lookup:

```go
	art, hasArt := m.Overrides[square]
	if !hasArt {
		return Resolved{Kind: kind, Material: material}, nil, nil
	}
	piece, err := artlib.Lookup(artDir, art)
	if errors.Is(err, artlib.ErrNotFound) {
		// DEGRADE, not refuse (spec §4). The nature is already in hand from
		// m.Tiles, which is the whole reason this is safe: the square keeps
		// being a wall, it just stops being a PARTICULAR wall.
		return Resolved{Kind: kind, Material: material},
			[]string{fmt.Sprintf("square %s names art %q, which is not installed; "+
				"drawing it plain", square, art)}, nil
	}
	if err != nil {
		// Art that EXISTS and cannot be read is a defect to fix, not a square
		// to draw plain. Degrading here would ship a broken sidecar silently.
		return Resolved{}, nil, err
	}
```

Keep the existing kind-mismatch WARNING exactly as it is — an illusory wall is
legitimate dungeon craft and this task must not turn it into a refusal.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/mapdef/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mapdef/
git commit -m "A missing picture is a plain square, not a refused map"
```

---

### Task 4: The gateway resolves art on demand

**Files:**
- Modify: `internal/gateway/server.go`, `internal/gateway/map.go`,
  `internal/mapdef/installed.go`
- Test: `internal/gateway/map_test.go`, `cmd/vtt/client_e2e_test.go`

**Interfaces:**
- Produces: `(*Server).WithArtDir(dir string) *Server`.
- Consumes: `mapdef.Resolve` (Task 3).

- [ ] **Step 1: Write the failing tests**

**Carry the assertion Task 2 had to retire.** Task 2's brief drove `load_map`
with a map naming absent art and asserted `ok=true` plus warnings naming the
reference. Task 3 had not landed, so on that tree absent art still refused, and
Task 2 correctly substituted a kind-mismatch fixture instead. That means the
POSITIVE end-to-end §4 case — a map naming absent art loads, succeeds, and
names the dropped reference — is tested nowhere and was scheduled nowhere until
this line. It belongs here, because this task already builds the fixture it
needs (`composeServer` plus a real `art/`). Add it alongside the negative case
below; the negative one alone would pass on a server that never warns at all.

```go
func TestArtInstalledAfterBootIsFoundWithoutARestart(t *testing.T) {
	// The defect sub-project 15 shipped, inverted into a requirement. This
	// MUST go through composeServer: the pack version of this test used a
	// bare gateway.Server and therefore could not see the boot-order bug at
	// all (whole-branch review, 2026-09-02).
	camp := t.TempDir()
	mustMkdirAll(t, filepath.Join(camp, "maps"), filepath.Join(camp, "art"))
	srv, _, err := composeServer(camp, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	// ... serve, dial as DM ...
	writeArt(t, filepath.Join(camp, "art"), "late-stone",
		`{"format_version":1,"kind":"wall","material":"stone"}`)
	writeMap(t, filepath.Join(camp, "maps"), "hall", `{"format_version":1,"id":"hall",
		"name":"Hall","grid_width":1,"grid_height":1,"tiles":{"0,0":"stone-wall"},
		"overrides":{"0,0":"late-stone"}}`)
	res := loadMap(t, conn, "hall")
	if !res.GetOk() {
		t.Fatalf("load_map: %s", res.GetError())
	}
	if len(res.GetWarnings()) != 0 {
		t.Fatalf("warnings %q: the art WAS installed, before the load and after the boot",
			res.GetWarnings())
	}
}

func TestArtOverwrittenInPlaceChangesWhatAReloadDraws(t *testing.T) {
	// Patrik's scenario, 2026-09-02: "i find a better art ... i should be able
	// to overwrite it ... And then when I reload the map. It will use the new
	// art." Nothing is cached across a load, and this is what pins that.
	// ... load once, overwrite the sidecar's material, load again, assert the
	// second SceneCreated carries the new material ...
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./cmd/vtt/ -run 'ArtInstalledAfterBoot|OverwrittenInPlace' -count=1`
Expected: FAIL — `WithArtDir` undefined.

- [ ] **Step 3: Implement**

Add `artDir` to `Server` with `WithArtDir`, wired **unconditionally** in
`composeServer` — outside any guard, for the reason sub-project 15 learned the
hard way. Call `artlib.Validate(artDir)` once at compose time and fail the boot
on a subdirectory or an orphan sidecar; it reads no art and an absent `art/`
passes.

`LoadInstalled(mapsDir, id, artDir)` replaces the `packs` map parameter.

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/vtt/ ./internal/gateway/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Art is read when the map is loaded, not once at boot"
```

---


---

## Execution order changed 2026-09-04 — delete the pack NEXT, not seventh

**Patrik:** *"Some times it is better to delete the old solution before building
the new. Our decision to keep the old packs while building the new only create
challenges unnecessarily since no one is using the product."*

He is right and the cost is measurable. This plan ordered the pack's deletion
LAST, copying `create_scene`'s removal in sub-project 15 — build the
replacement, prove it, then remove. That caution was correct there and wrong
here, and the difference was never checked: `create_scene` was a **contract
command** with five completeness gates and live clients, so removing it early
would have left a hole on the wire. Packs are **internal Go types with no
external consumer and no released contract**, and the product has no users. The
same caution bought nothing.

What it cost across Tasks 1-5: `Map.Pack` stayed live so every task threaded a
dying type (Task 3 reached 22 files, Task 5 reached 32); `metadata.go` kept
serving `packRefJSON` until Task 5 deleted it as "rubble from this deletion";
14 fixtures were MIGRATED off `"pack"` when deleting the pack would have had
them rewritten once, in Task 8, where they were going anyway; and a false
`cellPx` cost comment existed only because `packRefJSON` was still alive to be
mourned — it would have misdirected Task 6.

**New order: 5 → 7 → 4b → 6 → 8 → 9.** Task 7 runs next. Then Task 6 builds
`GET /api/art/{file}` into a tree with no `GET /api/packs/{pack}/{file}` beside
it to work around, and Task 8's migration is a rewrite rather than a
reconciliation.

**No gap is created by moving it.** Nothing serves art bytes today, and
`campaigns/example/art/` does not exist until Task 8 — so deleting the pack
route removes a capability nothing is currently using.

**The test for next time is not "is this a removal?" but "who is standing on it
while I take it away?"** Here the answer was nobody.

---
### Task 4b: A corrupt sidecar degrades; a newer format still refuses

Patrik's ruling, 2026-09-04. Added after Task 4's review measured that one
corrupt sidecar named by one committed map **stops the server booting** — exit
status 1, with every other map fine — because `composeServer` turns any
map-load error into a refusal to start.

It is deliberately its own task rather than a rider on Task 4: it changes
behaviour, it amends the spec (§4, §5, criterion 5), and Task 4 was already
green and reviewed.

**Files:**
- Modify: `internal/mapdef/resolve.go` (the artlib-error arm in `Resolve` and
  `ResolveObjectArt`), `internal/artlib/artlib.go` (a sentinel the caller can
  branch on)
- Test: `internal/mapdef/resolve_test.go`, `internal/artlib/artlib_test.go`,
  `cmd/vtt/maps_e2e_test.go`

**Interfaces:**
- Produces: a way for `Resolve` to tell "cannot be read" from "declares a format
  I do not understand". `artlib.ErrNotFound` and `artlib.ErrArtDirUnreadable`
  already exist; this needs a third, e.g. `artlib.ErrFormatVersion`.

- [ ] **Step 1: Write the failing tests.** A map naming art whose sidecar is
  corrupt JSON loads, that square draws plain, and the warning names the piece
  AND the cause — never the not-installed sentence, for §3.4's reason. A map
  naming art whose sidecar declares `format_version: 99` is still refused, and
  the refusal names both versions. A campaign holding the corrupt file **boots**.

- [ ] **Step 2: Run them RED.** Behavioural, not compile-failure: today both
  cases refuse, so the corrupt one fails on the refusal and the boot one fails
  on exit status 1.

- [ ] **Step 3: Split the arm.** Only artlib errors, and only in `Resolve` /
  `ResolveObjectArt`. A structurally broken MAP — an unknown tile name, a square
  with no tile — must still refuse; this ruling is about art, not about maps.

- [ ] **Step 4: Run the suites**, including `cmd/vtt`, which is where the boot
  behaviour is observable.

- [ ] **Step 5: Commit.**

---

### Task 5: A map declaring "pack" is refused

> **AMENDED 2026-09-06 — DO NOT RE-EXECUTE THIS TASK AS WRITTEN.** Patrik:
> *"We never used the platform, there is no need for a migration route. We
> talked about this before."* The refusal below and its `"package"` sibling both
> put MIGRATION INSTRUCTIONS in the error message, and so did the adventure
> bundle's `tiles/pack.json` refusal added under Task 7. All three were deleted:
> nothing has ever shipped, `contract/RELEASED` does not exist, every campaign
> that has ever existed is in this repository, and Step 3 of this very task
> already rewrote all fourteen fixtures that declared a pack.
>
> **What survives, unchanged, is that a map declaring `"pack"` is REFUSED** —
> `mapdef.loadAs` decodes through `decodeStrict`'s `DisallowUnknownFields`, so
> the two arms below could only ever be reached by a decoder that had already
> accepted the field. The assertion that says so is
> `TestAMapDeclaringAPackOrAPackageIsStillRefused`, which pins
> `json: unknown field "pack"` rather than the migration text. The bundle
> refusal has no successor: an adventure shipping `tiles/pack.json` loads, and
> its unresolved art degrades and warns exactly as a bundle with no `art/`
> already did.
>
> The Step-1 test body below therefore asserts `art/`, which nothing produces
> any more. It is kept as the record of what Task 5 did, not as an instruction.
> Design spec §7 carries the same amendment.

**Files:**
- Modify: `internal/mapdef/format.go` (delete `Map.Pack`), `internal/mapdef/load.go`
- Test: `internal/mapdef/load_test.go`

**Before you start: one existing test WILL go red here, and the cheap fix is
wrong.** `internal/gateway/map_test.go`'s `TestALoadMapWarningReachesTheIssuer`
(Task 2) uses a fixture declaring `"pack":"cellar-basics"` to produce a
kind-mismatch warning. This task makes that declaration a refusal. **Migrate it
to a sidecar-based kind mismatch; do not delete it.** It is the only test on the
branch that proves a warning traverses the whole channel to the issuer, and
deleting a test deletes coverage nothing will shout about.

- [ ] **Step 1: Write the failing test**

```go
func TestAMapDeclaringAPackIsRefusedByName(t *testing.T) {
	// Spec §7: no compatibility layer, and none added later. Ignoring the
	// field would load a map whose art references were written against a
	// namespace that no longer exists — and draw the wrong thing.
	dir := t.TempDir()
	writeMap(t, dir, "old", `{"format_version":1,"id":"old","name":"Old","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"},"pack":"cellar-basics"}`)
	_, err := mapdef.Load(filepath.Join(dir, "old.json"))
	if err == nil {
		t.Fatal("a map declaring \"pack\" was accepted")
	}
	if !strings.Contains(err.Error(), "pack") || !strings.Contains(err.Error(), "art/") {
		t.Fatalf("error %q must name the field AND point at art/", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/mapdef/ -run DeclaringAPack -count=1`, expect FAIL (accepted and ignored).

- [ ] **Step 3: Implement.** Keep `pack` in the raw decode struct **solely** to
detect and refuse it; delete `Map.Pack`.

- [ ] **Step 4: Run tests** — `go test ./internal/mapdef/ -count=1`.

- [ ] **Step 5: Commit** — `git commit -m "A map that names a pack is refused, not quietly ignored"`

---

### Task 6: The outward-facing art surface — cell_px, the route, and `vtt art install`

Three things, one deliverable: everything outside the platform that touches art.
A reviewer would accept or reject them together, because each is meaningless
without the flat `art/` directory the other two assume.

**Files:**
- Create: `internal/campaigncfg/campaigncfg.go`, `cmd/vtt/art.go`
- Modify: `internal/gateway/metadata.go`, `cmd/vtt/serve_compose.go`,
  `client/src/metadata.ts`
- Test: `internal/campaigncfg/campaigncfg_test.go`,
  `internal/gateway/metadata_test.go`, `cmd/vtt/art_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestAnAbsentCampaignJSONGivesTheDefaultCellPx(t *testing.T) {
	cfg, err := campaigncfg.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v — campaign.json is optional", err)
	}
	if cfg.CellPx != 64 {
		t.Fatalf("CellPx = %d, want the documented default 64", cfg.CellPx)
	}
}
```

Plus: metadata reports `cellPx` at the top level and carries no `pack` object;
`GET /api/art/{file}` serves a file from `art/`; a symlink pointing outside
`art/` is refused (`os.OpenRoot`, not `os.DirFS` — the reason the pack loader
already records).

And for the CLI (spec §5), where the point is that it has **no authority**:

```go
func TestArtInstallWarnsBeforeOverwritingAnExistingStem(t *testing.T) {
	// Patrik's rule, 2026-09-02: "it will ask if you want to overwrite,
	// otherwise it will ask you to change name/id". Copying by hand stays
	// legal and skips this — the command is a convenience, and the LOADER is
	// the backstop that cannot be bypassed.
	// ... install masonry-1 twice; assert the second refuses without --force
	// and names the existing file ...
}

func TestArtInstallRefusesADirectory(t *testing.T) {
	// art/ is flat. Accepting a directory here would install a pack.
}

func TestArtInstallValidatesTheSidecarAtInstallRatherThanAtTheTable(t *testing.T) {
	// ... install a sidecar with format_version 99; assert it is refused now ...
}
```

- [ ] **Step 2: Run to verify they fail.**

- [ ] **Step 3: Implement.** `campaigncfg.Load` returns `Config{CellPx: 64}` when
the file is absent. `packRefJSON` is deleted; metadata reports `cellPx` directly.
Route `GET /api/art/{file}` over `os.OpenRoot(artDir)`.

**An unresolved OBJECT paints a magenta checkerboard, and the spec says it is
"drawn from its kind". Fix that here.** Found by Task 4b's review, 2026-09-05,
by checking the CLIENT rather than the server. `objectImage` returns `tile:` for
an empty `Art` and `canvas.ts`'s `paint` sends that to `drawMissingTile` — a 2x2
checkerboard — while spec §4 promises the object stays, drawn from its kind, and
`ResolveObjectArt`'s own warning tells the DM exactly that.

Tiles are fine and the asymmetry is the point: `tileImage` falls back to
`std:<kind>/<material>`, so a degraded tile stays a wall and a degraded door
stays a door for sight, movement and the door tool. **Objects have no such
fallback.** Every argument this sub-project has made about degrading safely was
verified on tiles and silently assumed for objects.

`campaigns/example/maps/cellar.json` names four object arts — `barrel`,
`brazier`, `crate-wood`, `pillar-stone` — so once Task 8 installs them, one
corrupt `pillar-stone.json` boots the server and paints checkerboards where the
pillars are. This is a CLAUDE.md rule 7 deviation: delivered behaviour
contradicts a spec sentence the code quotes. The spec is right; the code is
wrong.

**Task 7 deleted seven security proofs along with the pack route, and this task
owes every one of them back.** The pack route's tests for path traversal,
symlink escape, the content-type allowlist, SVG exclusion, the unknown-file 404
and end-to-end bytes are gone. Nothing regresses today because no route serves
bytes — but the ruling now survives only as prose in
`internal/gateway/metadata.go`, and `internal/artlib` pins the symlink half at
the LOOKUP layer, never at a route. Re-establish each at the new route.

**And one requirement has no pack precedent to copy — but NOT for the reason
this paragraph originally gave.** It claimed the pack route was saved from
serving a nested file by net/http's single-segment `{file}` wildcard. **Measured
2026-09-05 and false:** the wildcard rejects the literal form
(`/api/art/a/x.png` → 404) and passes the ENCODED one —
`/api/art/pack-ish%2Fx.png` arrives at the handler with
`PathValue("file") == "pack-ish/x.png"`, because the mux decodes `%2F` into the
value after matching. So the pattern's independent contribution against a
determined request is **zero**, and the pack route had the identical hole; it
was never protected by its shape.

The real guard is the name check, and it must be explicit. `art/` being flat
does not make a subdirectory unreachable — only refusing a name that is not a
plain art filename does.

**The route MUST NOT serve a file inside a subdirectory**, and `os.OpenRoot`
alone does not stop it: a root CONFINES but does not FLATTEN, and `fs.ValidPath`
rejects only `..`, so `art/pack-ish/x.png` is legitimately inside the root. Two
things keep a subdirectory inert and this task must keep at least one: the
pattern stays `{file}` — net/http's single-segment wildcard does not match
across `/`, which is what the pack route already relied on — and/or the handler
runs `isPictureName` on the name. Test it directly: the spec's whole
no-subfolders rule (§3.1, §3.3) rests on nothing inside `art/pack-ish/` being
reachable, and this route is the only place that could make it reachable. `vtt art install <path>...`
copies files in, refuses a directory, refuses an existing stem without `--force`,
and runs `artlib.Lookup` on each installed stem so a malformed sidecar is caught
at install rather than at the table.

- [ ] **Step 4: Run tests** — `go test ./... -count=1 && bunx tsc --noEmit -p client/tsconfig.json`.

- [ ] **Step 5: Commit** — `git commit -m "A grid is uniform, so its cell size belongs to the campaign"`

---

### Task 7: Delete the pack

Nothing calls it now. This is the task that removes sub-project 15's boot-order
defect by removing the code it lived in.

**Files:**
- Modify: `internal/mapdef/format.go`, `internal/mapdef/load.go`,
  `internal/gateway/server.go`, `internal/gateway/map.go`, `cmd/vtt/maps.go`,
  `cmd/vtt/serve_compose.go`

- [ ] **Step 1: Write the absence test FIRST**, beside the `create_scene`
precedent in `client/test/command-surface.test.ts` and as a Go compile-level
assertion — no `mapdef.Pack`, no `LoadPack`, no `WithPackFiles`.

- [ ] **Step 2: Run it RED.**

- [ ] **Step 3: Replace the adventure's embedded pack FIRST**, because it is the
one caller that needs something rather than nothing. `Adventure.Pack *mapdef.Pack`
becomes an art directory rooted at the adventure, and `loadEmbeddedPack` is
deleted in favour of `artlib` reading `<adventure>/art/`. An adventure with no
`art/` is legal, exactly as `tiles/pack.json` was optional — its scenes then draw
from the built-in vocabulary and warn, which is spec §4 applied to the same
mechanism rather than a second one.

- [ ] **Step 4: Delete**, outward-in: `cmd/vtt/maps.go`'s pack walk and the
`os.Stat(mapsDir)` guard in `serve_compose.go` (the guard's whole reason was that
`loadMapsDir` failed on a missing `maps/`; make the maps walk tolerate absence the
way the pack walk already did, then the guard has nothing left to do);
`Server.packs`, `Server.packFS`, `WithPackFiles`; `ErrPackNotLoaded` and its arm
in `map.go`; finally `mapdef.Pack`, `PackTile`, `LoadPack`.

- [ ] **Step 5: Run everything** — `go build ./... && go test ./... -count=1 && bun test client/test contract`.

- [ ] **Step 6: Commit** — `git commit -m "The pack leaves, and takes a boot-order defect with it"`

---

**Before you install real sidecars, close one inherited defect.** Found by Task
6's review, 2026-09-05, in Task 3's code: a sidecar declaring
`"format_version": 0` returns `artlib.ErrFormatVersion`, so `mapdef.Resolve`
**refuses the map** — and `composeServer` refuses the BOOT when a committed map
names it. The message is neutral so no operator is misdirected, but a typo'd `0`
is not "content newer than this server", and §4 reserves the one surviving
refusal for exactly that case.

It is the same absent-versus-zero shape fixed twice already — `mapJSON.Pack`
(Task 5) and `campaigncfg`'s two fields (Task 6) — one directory over, and it
has been harmless only because no real sidecar existed. **This task is what
makes real sidecars exist.** Fix it here, with the `*int32` precedent both
earlier fixes used.

### Task 8: Migrate the fixtures and the generator

**Files:**
- Create: `campaigns/example/art/*`, `campaigns/example/campaign.json`
- Delete: `campaigns/example/packs/`
- Modify: `campaigns/example/maps/cellar.json`, `scenarios/`, `scenarios/goldens/`,
  `tools/genmappack/`, `adventures/*/` (each `tiles/pack.json` becomes `art/`)

- [ ] **Step 1:** Rewrite `campaigns/example/` — every pack tile and object
becomes `art/<stem>.png` plus, for tile art, `art/<stem>.json`. File stems become
kebab-case so the stem IS the referenced id (`masonry_1.png` → `masonry-1.png`).
`cellar.json` drops its `"pack"` field; its `overrides` values are unchanged
because they were already the art names.

- [ ] **Step 2:** `tools/genmappack` emits the flat layout and takes `cell_px`
as its own flag rather than writing it into a pack.

- [ ] **Step 2b: Carry the assertion Tasks 4 and 8 pass between them.** Nothing
tests that the SHIPPED campaign's art reaches the wire —
`TestLoadMapProducesBatchCarryingTilesAndObjects` loads the real `cellar.json`
but resolves it against a synthetic art directory. Task 4 correctly judged the
fixture could not exist before this task creates `campaigns/example/art/`, and
left a note in `cellarArtDir`'s doc comment. That note is in
`internal/gateway/map_test.go`, which this task does not otherwise open — so it
is written here too, where an implementer actually looks. Assert the shipped
campaign's own art resolves and reaches a seat.

- [ ] **Step 3:** Regenerate goldens where art metadata reaches the wire. Goldens
have **no `-update` flag** by design; a changed golden is re-derived by hand and
the change explained in the commit.

- [ ] **Step 4:** Run `go test ./... -count=1 && bun test client/test contract`.

- [ ] **Step 5: Commit** — `git commit -m "The example campaign keeps its art in one place"`

---

### Task 9: Prove the removal with a gate, and run the whole thing

**Files:**
- Create: `tools/check-no-pack.py`, `tools/check_no_pack_test.py`
- Modify: `Taskfile.yml`

- [ ] **Step 1:** Build the gate in the shape that already works. Follow
`tools/check-no-create-scene.py` exactly: mask comments and string literals per
language, read CODE positions only, exempt by WORD not by file, scan `.json`
object KEYS only. **Read its `KNOWN LIMITS` block first** — it is accurate about
what it does and does not cover, including that it reads twelve extensions rather
than every language, and that a Go-registered MCP tool name is a string literal
it cannot see.

Measure first, as that gate's docstring does, and put the real number in: the
word `pack` is ordinary English and will appear in prose that must survive.

- [ ] **Step 2:** Wire it as its own `check:` task **and** into `check` itself.
Verify by running `task --dry check`, not by reading the file — six gates were
once added and omitted from the aggregate, repaired at `ac307d5`.

- [ ] **Step 3:** Run the whole gate from cold.

```bash
go clean -cache
task check
```

Before starting: check disk (`df -h /`, 16 GiB floor) and check no detached
mutation run is already going (`ps -eo etime,command | grep -E 'gremlins|stryker'`)
— one from a previous session was found still running a day later on 2026-09-02.

- [ ] **Step 4: Adjudicate what the gates find.** Every surviving mutant is killed
with a test or adjudicated with a stated observable, and you try to FALSIFY your
own equivalence claim first.

**If the gate reports an adjudication as stale, do not delete it on that alone.**
`stale` is `set(equivalents) - claimed` where `claimed` holds only `LIVED`, so any
run that fails to observe a mutant living turns a sound entry into a red gate
whose printed remedy is destructive. Check the file is byte-unchanged, hand-apply
the mutation and run the package, check it is not a coverage loss, then re-run
gremlins on that one package and read the actual status token.

- [ ] **Step 5: Confirm both verdicts describe THIS tree** by recomputing every
gated package's `package_fingerprint` against `reports/mutation-skip-cache.json`
and the TS stamp against `reports/mutation/ts-inputs.sha256`. Use the gate's own
`PACKAGES` strings — the package spelling is inside the hash.

- [ ] **Step 6: Verify and stop.** Report the full gate output. Do not commit.

---

## Merge gate

`task check` green from cold; the spec's nine exit criteria walked one at a time
with the result recorded; every gated fingerprint recomputed against the tree
being merged; and the spec amended for any deviation, which needs Patrik's
approval under CLAUDE.md rule 6.

**Carried debt to close here, from sub-project 15's whole-branch review** — these
were left unfixed at that merge because this sub-project deletes the pack, but
three of them survive it and must not be lost:

- `tools/check-no-create-scene.py`'s `KNOWN LIMITS` claims any `create_scene`
  tool needs a contract oneof arm "and that arm IS a code position this gate
  sees". False: `internal/mcp` registers tools by Go string literal.
- `internal/mapdef/format.go`'s `Map.FormatVersion` doc claims every live `*Map`
  carries `MapFormatVersion`; `internal/adventure/format.go`'s `asMap()` builds
  one with 0.
- `internal/mapdef/installed_test.go`'s header asserts the id-shape refusal has
  no boot-time counterpart, which the same file's own
  `TestTheBootWalksOwnUnusableIdsAreRefusedByFilename` exists to disprove.
- `rulesets/dnd45e-minimal/guide.md` cites `298f677` for the statblocks; that
  commit added the section heading, but `52812d2` added the JSON `actor` form the
  sentence is actually about.
- Three committed specs still describe `create_scene` as live contract
  (`docs/superpowers/specs/2026-07-23-api-gateway-design.md` among them) and need
  the dated amendment `59542e1` set the precedent for.
