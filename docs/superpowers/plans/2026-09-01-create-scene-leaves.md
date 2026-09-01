# create_scene Leaves the Platform — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove `create_scene` from the platform, and make a campaign a directory that owns its maps — installed by writing a file, brought into play by `load_map`, found without a restart.

**Architecture:** A campaign becomes a directory holding its log, a flat `maps/` of human-named JSON files, and a `packs/` of tileset directories. Installing a map is a filesystem act outside the platform; `load_map` is the only platform step. The server preloads at boot as it does today and, on a miss, probes one path before refusing. `create_scene` is deleted last, once its replacement works.

**Tech Stack:** Go 1.26 (`internal/mapdef`, `internal/campaign`, `internal/gateway`, `cmd/vtt`), TypeScript + Bun (`client/`), protobuf via buf (`contract/`), Task for gates.

**Spec:** `docs/superpowers/specs/2026-09-01-create-scene-leaves-design.md`

## Global Constraints

- **Airtight TDD (ADR-009).** Tests first, RED observed and recorded before the solution exists. Behavioural RED over compile-failure RED wherever a stub can compile. For a REMOVAL the RED is inverted: the test proving the thing is gone must fail while it is still present.
- **`task check` is the single gateway.** Never weaken a gate to pass it. `task check:fast` is an inner-loop convenience and never satisfies the gate.
- **Citations name durable things** (CLAUDE.md rule 8): a function, a test, a constant, a commit hash, a dated decision. Never a bare `file.go:123`, never a path into gitignored `.superpowers/`, never "this task". Mutation-adjudication coordinates are the sole exception.
- **Contract evolution is additive only from first release** (ADR-007 as amended). `contract/RELEASED` is absent, so `check:breaking` reports and exits 0 — it must still PRINT every objection.
- **One fold.** `engine.Apply` stays the only code that changes game state.
- **No implicit fallback.** A missing declaration is refused, never defaulted. This is why `format_version` is required rather than assumed.
- **Goldens are never machine-regenerated.** `cmd/vtt/scenario_goldens_test.go` has no `-update` flag by deliberate decision: `state.json` is hand-derived from the scenario definition, `stream.json` is recorded from the server, and their agreement is evidence only because neither was produced from the other.
- **Adding or removing lines in a gated package moves mutation adjudication keys.** Re-key from the gate's OUTPUT by rendering each coordinate against the tree, never by applying an offset, and re-resolve any `file:line` written inside an entry's prose.
- **Rebuild the embedded bundle** after any `client/src` change (`task build:client`) and stage `cmd/vtt/webdist`.
- **Report from `git diff`,** never from the task list. If a row cannot be pointed at a hunk, it did not happen.

## File Structure

| File | Responsibility |
|---|---|
| `internal/mapdef/format.go` | `Map` and `Pack` structs gain `FormatVersion`; the supported-version constants live here |
| `internal/mapdef/load.go` | `Load` and `LoadPack` read and check `format_version` |
| `cmd/vtt/maps.go` | `loadMapsDir` walks the new flat layout; filename/id agreement enforced here |
| `internal/campaign/campaign.go` | `Open` takes a campaign DIRECTORY and refuses a bare log file |
| `internal/gateway/server.go` | `handleLoadMap` probes the campaign's `maps/` on a miss |
| `cmd/vtt/serve_compose.go` | wires the campaign directory's maps; `--maps-dir` removed |
| `scenarios/*.json` + `scenarios/maps/` | eight scenarios load authored maps instead of creating scenes |
| `tools/check-no-create-scene.py` | the removal gate, in the shape of `tools/check-no-retraction.py` |


## Test helpers this plan uses

Two exist and are used as-is: `writeFile(t, path, content)` in `cmd/vtt`'s map
tests, and `newGWFixture(t)` in `internal/gateway`'s server tests.

Three do NOT exist and are written by the first task that needs them. They are
named here so later tasks can rely on the same shapes rather than inventing
parallel ones:

```go
// writeMap writes a minimal valid map at path, creating parent directories.
// The id is BOTH the file's stem and its declared ID, because Task 3 makes a
// disagreement a refusal — a helper that let them drift would hide the bug the
// refusal exists to catch.
func writeMap(t *testing.T, path, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"format_version":1,"id":%q,"name":%q,
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"floor"}}`, id, id)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// loadMap builds the ClientCommand a DM sends to bring a map into play.
func loadMap(id string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{
		Command: &vttv1.ClientCommand_LoadMap{LoadMap: &vttv1.LoadMap{MapId: id}},
	}
}

// getJSON issues a GET against a composed server and returns the body.
func getJSON(t *testing.T, srv *http.Server, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", path, rec.Code)
	}
	return rec.Body.String()
}
```

`newGWFixture` gains a `campaignDir` field in Task 6, since the fixture must be
able to write a map into the campaign it booted.

---

### Task 1: A map declares the format it is written in

**Files:**
- Modify: `internal/mapdef/format.go`, `internal/mapdef/load.go`
- Modify: `maps/cellar/map.json`, and every map fixture under `internal/mapdef/testdata/`, `cmd/vtt/testdata/`
- Test: `internal/mapdef/load_test.go`

**Interfaces:**
- Produces: `mapdef.MapFormatVersion = 1` (the only version this server understands); `Map.FormatVersion int32`.

- [ ] **Step 1: Write the failing tests**

```go
func TestLoadRefusesAMapWithNoFormatVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "no-version.json")
	writeFile(t, p, `{"id":"x","name":"X","grid_width":1,"grid_height":1,
		"tiles":{"0,0":"floor"}}`)

	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal("want a map with no format_version refused: an undeclared format is " +
			"undeclared, and this platform does not default one")
	}
	if !strings.Contains(err.Error(), "format_version") {
		t.Fatalf("error = %q, want it to name format_version", err)
	}
}

func TestLoadRefusesAFormatThisServerDoesNotUnderstand(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "future.json")
	writeFile(t, p, `{"format_version":2,"id":"x","name":"X","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"floor"}}`)

	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal("want format 2 refused while this server understands only 1")
	}
	// The message must name BOTH numbers: a reader who only learns their file
	// is wrong cannot tell whether to downgrade the file or upgrade the server.
	for _, want := range []string{"2", "1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to name version %s", err, want)
		}
	}
}

func TestLoadAcceptsTheVersionItUnderstands(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.FormatVersion != mapdef.MapFormatVersion {
		t.Fatalf("FormatVersion = %d, want %d", m.FormatVersion, mapdef.MapFormatVersion)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/mapdef/ -run FormatVersion -count=1 -v`
Expected: FAIL — `Map` has no `FormatVersion` field. Record the compile error, then add the field alone and re-run to get a BEHAVIOURAL red (`want a map with no format_version refused`), which is the one to quote.

- [ ] **Step 3: Implement**

In `format.go`:

```go
// MapFormatVersion is the map format this server understands. A map declares
// its own, and a mismatch is refused by name rather than guessed at — see
// LoadPack's PackFormatVersion for why the two version independently.
const MapFormatVersion int32 = 1
```

Add `FormatVersion int32` to `Map` and `format_version` to `mapJSON`. In `Load`, immediately after `decodeStrict`:

```go
if raw.FormatVersion == 0 {
	return nil, fieldErr(path, "format_version", fmt.Sprintf(
		"required: this server understands %d, and an undeclared format is not "+
			"assumed to be any of them", MapFormatVersion))
}
if raw.FormatVersion != MapFormatVersion {
	return nil, fieldErr(path, "format_version", fmt.Sprintf(
		"declares %d; this server understands %d", raw.FormatVersion, MapFormatVersion))
}
```

Note `decodeStrict` rejects unknown fields, so a file carrying `format_version` fails to parse until `mapJSON` gains it — add both together.

- [ ] **Step 4: Give every existing map fixture its version**

`maps/cellar/map.json` plus every map under `internal/mapdef/testdata/` and `cmd/vtt/testdata/`. Find them with `grep -rl '"grid_width"' --include='*.json' .`; each gains `"format_version": 1` as its first key.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/mapdef/... ./cmd/vtt/... -count=1
```

- [ ] **Step 6: Verify and stop.** Do not commit.

---

### Task 2: A pack declares its own format, separately

**Files:**
- Modify: `internal/mapdef/format.go`, `internal/mapdef/load.go`
- Modify: `maps/cellar/tiles/pack.json` and every pack fixture
- Test: `internal/mapdef/load_test.go`

**Interfaces:**
- Consumes: Task 1's pattern.
- Produces: `mapdef.PackFormatVersion = 1`; `Pack.FormatVersion int32`.

- [ ] **Step 1: Write the failing test**

```go
func TestLoadPackRefusesAFormatThisServerDoesNotUnderstand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pack.json"), `{"format_version":2,"id":"p",
		"name":"P","cell_px":64,"tiles":[]}`)

	_, err := mapdef.LoadPack(dir)
	if err == nil {
		t.Fatal("want pack format 2 refused while this server understands only 1")
	}
	if !strings.Contains(err.Error(), "format_version") {
		t.Fatalf("error = %q, want it to name format_version", err)
	}
}

// A PACK AND A MAP VERSION INDEPENDENTLY, and this test says so by using
// them where only separate values can work. If a later edit makes one an
// alias of the other, a pack written against a moved map format is accepted
// here and this fails.
func TestAPackVersionIsNotAMapVersion(t *testing.T) {
	dir := t.TempDir()
	// A pack declaring the MAP's version rather than its own.
	writeFile(t, filepath.Join(dir, "pack.json"), fmt.Sprintf(
		`{"format_version":%d,"id":"p","name":"P","cell_px":64,"tiles":[]}`,
		mapdef.MapFormatVersion+1))

	if _, err := mapdef.LoadPack(dir); err == nil {
		t.Fatal("a pack must be judged against PackFormatVersion, not the map's")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/mapdef/ -run PackRefuses -count=1 -v`

- [ ] **Step 3: Implement, mirroring Task 1**

```go
// PackFormatVersion is the pack format this server understands, and it moves
// INDEPENDENTLY of MapFormatVersion. A pack is shared: one tileset backs many
// maps, so a single shared number would force every pack to be rewritten the
// first time the map format moved.
const PackFormatVersion int32 = 1
```

- [ ] **Step 4: Give every pack fixture its version.** `grep -rl '"cell_px"' --include='*.json' .`

- [ ] **Step 5: Run** `go test ./internal/mapdef/... ./cmd/vtt/... -count=1`

- [ ] **Step 6: Verify and stop.** Do not commit.

---

### Task 3: Maps are flat files whose name is their id

**Files:**
- Modify: `cmd/vtt/maps.go` (`loadMapsDir`)
- Move: `maps/cellar/map.json` → `maps/cellar.json`; `maps/cellar/tiles/` → `packs/cellar-basics/`
- Test: `cmd/vtt/maps_test.go`

**Interfaces:**
- Produces: `loadMapsDir(dir string)` reading `<dir>/maps/*.json` and `<dir>/packs/*/pack.json`, unchanged return types.

- [ ] **Step 1: Read the current loader before changing it**

`loadMapsDir` walks subdirectories of the maps directory and skips non-directories, expecting `<name>/map.json` with `<name>/tiles/pack.json` beside it. It returns maps, packs and a `packFS` per pack for serving images. The new layout separates the two: maps are flat files, packs are directories in a sibling tree.

- [ ] **Step 2: Write the failing tests**

```go
func TestLoadMapsDirReadsFlatFilesNamedByTheirID(t *testing.T) {
	root := t.TempDir()
	writeMap(t, filepath.Join(root, "maps", "cellar.json"), "cellar")

	maps, _, _, err := loadMapsDir(root)
	if err != nil {
		t.Fatalf("loadMapsDir: %v", err)
	}
	if _, ok := maps["cellar"]; !ok {
		t.Fatalf("maps = %v, want a map keyed cellar from maps/cellar.json", keys(maps))
	}
}

// THE FILENAME IS THE ID, and this is the refusal that keeps it true. Without
// it, renaming a file and loading it produces a SECOND scene for the same
// place, silently, while the original stays in the world — because
// mapdef.Compile takes the scene's id from the map's own ID field.
func TestLoadMapsDirRefusesAFilenameThatDisagreesWithTheID(t *testing.T) {
	root := t.TempDir()
	writeMap(t, filepath.Join(root, "maps", "sunken-cellar.json"), "cellar")

	_, _, _, err := loadMapsDir(root)
	if err == nil {
		t.Fatal("want a filename/id mismatch refused")
	}
	for _, want := range []string{"sunken-cellar", "cellar"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to name %q", err, want)
		}
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test ./cmd/vtt/ -run LoadMapsDir -count=1 -v`
Expected: FAIL — the loader skips non-directories, so `maps/cellar.json` is ignored and `maps` is empty.

- [ ] **Step 4: Implement**

Walk `<dir>/maps` for `*.json`; the id is `strings.TrimSuffix(e.Name(), ".json")`. After `mapdef.Load`, refuse a disagreement:

```go
if m.ID != id {
	return nil, nil, nil, fmt.Errorf(
		"maps/%s.json declares id %q: a map's filename is its id, and a "+
			"disagreement would put a second scene in the world for the same "+
			"place — mapdef.Compile takes the scene id from the map's ID",
		id, m.ID)
}
```

Walk `<dir>/packs` for subdirectories, each with `pack.json`, keeping the existing `packFS` behaviour.

- [ ] **Step 5: Move the shipped fixture**

```bash
git mv maps/cellar/map.json maps/cellar.json
git mv maps/cellar/tiles packs/cellar-basics
```
Confirm `packs/cellar-basics/pack.json` declares `"id": "cellar-basics"` and that `maps/cellar.json` names it.

- [ ] **Step 6: Run** `go test ./cmd/vtt/... -count=1`

- [ ] **Step 7: Verify and stop.** Do not commit.

---

### Task 4: A campaign is a directory

**Files:**
- Modify: `internal/campaign/campaign.go` (`Open`)
- Modify: `cmd/vtt/serve_compose.go`
- Test: `internal/campaign/campaign_test.go`

**Interfaces:**
- Produces: `campaign.Open(dir string)` — `dir` is a campaign directory containing `log.db`; `campaign.LogPath(dir) string` for callers that need the log itself.

- [ ] **Step 1: Write the failing tests**

```go
func TestOpenTakesACampaignDirectory(t *testing.T) {
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("Open on a directory: %v", err)
	}
	defer c.Close()
	if _, err := os.Stat(filepath.Join(dir, "log.db")); err != nil {
		t.Fatalf("want log.db created inside the campaign directory: %v", err)
	}
}

// REFUSED, NOT ADOPTED. Treating a bare log file as a campaign with no maps is
// exactly the implicit fallback this platform keeps ruling against — the same
// reasoning that makes terrain mandatory and gives the console's selects a
// blank first option.
func TestOpenRefusesABareLogFile(t *testing.T) {
	dir := t.TempDir()
	lone := filepath.Join(dir, "old.db")
	writeFile(t, lone, "")

	_, err := campaign.Open(lone)
	if err == nil {
		t.Fatal("want a bare log file refused rather than adopted")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("error = %q, want it to say what to do", err)
	}
}
```

- [ ] **Step 2: Run them** — `go test ./internal/campaign/ -run Open -count=1 -v`

- [ ] **Step 3: Implement**

```go
// Open opens the campaign in dir. A campaign is a DIRECTORY — its log, its
// maps and its packs — because a map belongs to the campaign that uses it
// (spec 2026-09-01-create-scene-leaves §3).
func Open(dir string) (*Campaign, error) {
	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		return nil, fmt.Errorf("campaign: %s is a file; a campaign is a "+
			"directory holding log.db, maps/ and packs/ — put it in one", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s, err := store.Open(LogPath(dir))
	...
}

// LogPath names the log inside a campaign directory.
func LogPath(dir string) string { return filepath.Join(dir, "log.db") }
```

- [ ] **Step 4: Update the one non-test caller.** `composeServer` passes `campaignPath`; it now passes a directory.

- [ ] **Step 5: A read-only campaign directory earns a clear error**

An operator running from a read-only mount loses improvisation entirely (spec §12). `os.MkdirAll` will fail with a bare permissions error that says nothing about campaigns. Wrap it:

```go
if err := os.MkdirAll(dir, 0o755); err != nil {
	return nil, fmt.Errorf("campaign: cannot open %s for writing — a campaign "+
		"directory must be writable, since installing a map means putting a "+
		"file in it: %w", dir, err)
}
```

- [ ] **Step 6: Run** `go test ./... -count=1`

- [ ] **Step 7: Verify and stop.** Do not commit.

---

### Task 5: Maps come from the campaign, and `--maps-dir` goes

**Files:**
- Modify: `cmd/vtt/serve_compose.go`, `cmd/vtt/serve.go`, `cmd/vtt/maps.go`
- Test: `cmd/vtt/serve_test.go`

**Interfaces:**
- Consumes: Task 3's `loadMapsDir(dir)`, Task 4's campaign directory.
- Produces: `composeServer(campaignDir, addr, rulesetDir, adventuresDir string)` — the `mapsDir` parameter is gone; maps come from `campaignDir`.

- [ ] **Step 1: Write the failing test**

```go
func TestTheServerServesTheCampaignsOwnMaps(t *testing.T) {
	dir := t.TempDir()
	writeMap(t, filepath.Join(dir, "maps", "cellar.json"), "cellar")

	srv, closeFn, err := composeServer(dir, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	defer closeFn()
	// GET /api/maps answers from the campaign's own maps/, not an operator flag.
	body := getJSON(t, srv, "/api/maps")
	if !strings.Contains(body, "cellar") {
		t.Fatalf("GET /api/maps = %s, want the campaign's cellar map", body)
	}
}
```

- [ ] **Step 2: Run it** — `go test ./cmd/vtt/ -run CampaignsOwnMaps -count=1 -v`

- [ ] **Step 3: Implement.** Drop the `mapsDir` parameter and the `--maps-dir` flag; call `loadMapsDir(campaignDir)`.

- [ ] **Step 4: Run** `go test ./... -count=1` and check no caller still passes five arguments.

- [ ] **Step 5: Verify and stop.** Do not commit.

---

### Task 6: A map installed during play is found without a restart

**Files:**
- Modify: `internal/gateway/server.go` (`handleLoadMap`), `internal/gateway/map.go`
- Test: `internal/gateway/map_test.go`

**Interfaces:**
- Consumes: Task 5's campaign-scoped maps.
- Produces: `handleLoadMap` probes `<campaignDir>/maps/<id>.json` on a miss.

- [ ] **Step 1: Write the failing tests**

```go
// THE WHOLE POINT OF THE SUB-PROJECT. A place the DM authored mid-session is
// loadable without a restart, because install and load are two separate acts.
func TestAMapInstalledAfterBootIsLoadable(t *testing.T) {
	f := newGWFixture(t) // boots with an empty maps/
	writeMap(t, filepath.Join(f.campaignDir, "maps", "level-2.json"), "level-2")

	res := f.send(t, dmConn, loadMap("level-2"))
	if !res.GetOk() {
		t.Fatalf("load_map after install: %s", res.GetError())
	}
}

func TestAnUnknownMapIsRefusedByName(t *testing.T) {
	f := newGWFixture(t)
	res := f.send(t, dmConn, loadMap("nowhere"))
	if res.GetOk() {
		t.Fatal("want an unknown map refused")
	}
	if !strings.Contains(res.GetError(), "nowhere") {
		t.Fatalf("error = %q, want it to name the map", res.GetError())
	}
}
```

- [ ] **Step 2: Run them** — `go test ./internal/gateway/ -run InstalledAfterBoot -count=1 -v`
Expected: FAIL — the map set is boot-time only, so the freshly written file is not found.

- [ ] **Step 3: Implement**

On a miss, probe one path — not a directory walk, because Task 3 makes the filename the id. Take the same lock the map set lives behind so two racing `load_map`s cannot both compile and cache it. Every failure is a readable `CommandResult`, never a torn connection.

**Boot and on-demand must share ONE validation function** (spec §12). Two similar paths reaching `mapdef` will drift, and the drift is invisible until a map that booted cleanly is refused on reload — or worse, the reverse. Extract whatever the boot loader does and call it from both; if they cannot share, say why in the report rather than duplicating.

- [ ] **Step 4: Run** `go test ./internal/gateway/... -count=1 -race`

- [ ] **Step 5: The mutation gate.** `internal/gateway` is gated. Run `task check:mutation`, then confirm coherence by RECOMPUTING `package_fingerprint` against `reports/mutation-skip-cache.json` rather than reading the output.

- [ ] **Step 6: Verify and stop.** Do not commit.

---

### Task 7: The scenario corpus loads maps

**Files:**
- Create: `scenarios/maps/*.json` (one per scene the corpus creates)
- Modify: the eight scenario definitions that call `createScene`
- Modify: seven golden directories, both files each, plus `session-zero/projections/`
- Modify: `cmd/vtt/harness_boot.go` to point the harness at the scenario maps

**Interfaces:**
- Consumes: everything above.
- Produces: a corpus in which no scenario issues `createScene`.

- [ ] **Step 1: Inventory before editing**

Eight scenario files issue eleven `createScene` steps. `goblin-fight` has no golden directory; `adventure-night` loads an adventure and creates no scene, so **seven** goldens move. Write the inventory into the report before touching anything.

- [ ] **Step 2: Author one map per created scene**

Each becomes `scenarios/maps/<scene-id>.json` with the same grid and the same terrain the scenario declares today, plus `"format_version": 1`. The filename must equal the scene id, because the scene id is what every later step references.

- [ ] **Step 3: Replace each `createScene` step with `loadMap`**

- [ ] **Step 4: Re-derive each `state.json` BY HAND**

Work one scenario at a time. For each, report what changed and why. A diff you cannot explain is a golden that stopped asserting.

- [ ] **Step 5: Re-record each `stream.json`** and check it against the hand-derived state. If a pair disagrees, say WHICH is wrong before changing either.

- [ ] **Step 6: Run the golden gates**

```bash
go test ./cmd/vtt/ -run 'TestScenario' -count=1
go test ./internal/harness/ -run 'TestFoldGoldenCorpus' -count=1
```

- [ ] **Step 7: Verify and stop.** Do not commit.

---

### Task 8: `create_scene` leaves

**Files:**
- Modify: `contract/vtt/v1/commands.proto`, regenerate
- Delete: `internal/gateway/create_scene_validate.go` and its test
- Modify: `internal/gateway/{server.go,authz.go}`, `tools/toolgen/main.go`
- Modify: `client/src/{commands.ts,view/dm.ts}`, `client/test/{command-surface,commands,dm-view}.test.ts`

**Interfaces:**
- Consumes: Task 7's corpus, which no longer needs the command.

- [ ] **Step 1: Write the failing test**

```ts
test("no command builder can create a scene, because the kernel does not make maps", () => {
  // Patrik, 2026-09-01: the kernel serves maps, it does not make them. A
  // create_scene is a ONE-SHOT command doing ITERATIVE work, so every mistake
  // is permanent. This asserts the ABSENCE, so it is written before the
  // removal and must fail now.
  expect(Object.keys(commands).filter((k) => /createScene/i.test(k))).toEqual([]);
});
```

- [ ] **Step 2: Run it** — `bun test client/test/command-surface.test.ts`. Expected: FAIL with `["createScene"]`.

- [ ] **Step 3: Delete, outward in.** The client builder and console group first, then the gateway handler, authz row and `commandName` arm, then `create_scene_validate.go`, then the toolgen entry, and the proto message and oneof arm LAST — a command the contract declares is reachable from five descriptor-driven completeness gates at once, so the contract is the only clean cut point.

- [ ] **Step 4: Regenerate and rebuild.** `task generate:contract`, `task build:client`, stage `cmd/vtt/webdist`.

- [ ] **Step 5: Confirm the breaking gate REPORTS**

Run `task check:breaking`. Expected: exit 0, with the pre-release banner AND buf's objection about the deleted message printed beneath it. Paste both. A run that prints nothing would mean the marker had switched the gate off.

- [ ] **Step 6: Run everything** — `go test ./... -count=1`, `bun test client/test contract`, `task check:drift`.

- [ ] **Step 7: Verify and stop.** Do not commit.

---

### Task 9: Prove the removal with a gate, and run the whole thing

**Files:**
- Create: `tools/check-no-create-scene.py` and its test
- Modify: `Taskfile.yml`

- [ ] **Step 1: Build the gate in the shape that already works**

`tools/check-no-retraction.py` masks comments and string literals per language and reads code positions only, exempting by WORD rather than by file so the sites that assert absence are not themselves the blind spot. Follow it exactly. Do NOT write a bare grep: the dated records explaining why `create_scene` left must survive, and a word-search reds on all of them.

- [ ] **Step 2: Wire it into `Taskfile.yml`** as its own `check:` task and into `check` itself, and run it. For every hit, decide out loud whether it is a leftover or a deliberate record.

- [ ] **Step 3: Run the whole gate from cold**

```bash
go clean -cache
task check
```

- [ ] **Step 4: Adjudicate what the gates find.** Every surviving mutant is killed with a test or adjudicated with a stated observable. If you believe a mutant is equivalent, try to falsify your own claim first by constructing a state that distinguishes it.

- [ ] **Step 5: Confirm both mutation verdicts describe THIS tree** by recomputing every gated package's `package_fingerprint` against `reports/mutation-skip-cache.json`, and the TypeScript stamp against `reports/mutation/ts-inputs.sha256`. Reading a gate's output tells you what it said; recomputing tells you what it was talking about.

- [ ] **Step 6: Verify and stop.** Report the full gate output. Do not commit.

---

## Merge gate

`task check` green from cold; the spec's eight exit criteria walked one at a time with the result recorded; every gated fingerprint recomputed against the tree being merged; and the spec amended for any deviation, which needs Patrik's approval under rule 6.
