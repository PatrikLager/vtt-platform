# SPEC-014: A map enters play by its id, from the set or from the campaign's maps directory

## Status

Accepted. Implemented by `internal/gateway/map.go` (`handleLoadMap`,
`mapByID`, `errNoMapsAvailable`) and `internal/gateway/server.go` (the `maps`,
`mapsDir`, `artDir`, `cellPx` and `mapsMu` fields of `Server`; `WithMaps`,
`WithMapsDir`, `WithArtDir`, `WithCellPx`), over `internal/mapdef`'s
`LoadInstalled` and `Compile` and `internal/campaign`'s `AppendBatch`; fed by
`cmd/vtt/serve_compose.go`'s `composeServer`; pinned by
`internal/gateway/map_test.go` and `map_internal_test.go`, and the id bound
`LoadInstalled` holds by `internal/mapdef/load_test.go`, `installed_test.go`
and `qa_id_bound_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: a place enters play when the DM asks for it, and a map or a piece of
art installed while the server runs is used without a restart.

## How it works

**What `load_map` asks for and answers.** `LoadMap.map_id` names the map.
`handleCommand` hands the command to `handleLoadMap` after authorization,
which is SPEC-013's. Every failure is
an ok=false `CommandResult` whose `Error` is one of the texts below, on a
connection that stays open. Success is ok=true with `Sequence`, the first
sequence `campaign.AppendBatch` assigned, the convention SPEC-007 states, and
`Warnings`.

**The map set, and the lookup on a miss.** `Server.maps` holds the maps the
server knows, keyed by each map's declared id. `WithMaps` fills it before the
server serves; after that it changes only in `mapByID`, and it only grows:
`grep -n 's\.maps\b'` over the package's non-test files prints `WithMaps`'
assignment, `handleMaps`' range, and `mapByID`'s reads, `make` and insertion.
`mapByID` reads the set under `mapsMu`'s read lock and releases it; a map the
set holds is answered from the set, so a map file edited on disk after its map
joined the set is not read again. On a miss:

- With `mapsDir` empty, the miss is refused with `gateway: no maps available`
  when the set is empty and `gateway: unknown map "<id>"` otherwise. No server
  `vtt serve` builds reaches this arm: `composeServer` always calls
  `WithMapsDir`.
- Otherwise `mapByID` calls `mapdef.LoadInstalled(s.mapsDir, id, s.artDir)`
  holding no lock, since it reads the disk and compiles the map while `GET
  /api/maps` reads the set. It turns `fs.ErrNotExist` into `gateway: unknown
  map "<id>": nothing installed at maps/<id>.json in this campaign` and
  forwards every other error as `LoadInstalled` wrote it. Then it takes the
  write lock and looks again: an entry another lookup made in the meantime is
  answered, so racing lookups of one new map end on one `*mapdef.Map` and the
  set gains one entry; otherwise the set is made if nil and the map inserted.

A map therefore joins the set when its lookup validates it, before the compile
and the append that follow, and a load refused after that leaves the map in
the set and on `/api/maps`. `LoadInstalled` is the function `cmd/vtt`'s boot
walk (`loadMapsDir`) runs on each `maps/*.json` file with the same art
directory, so what `LoadInstalled` refuses at boot it refuses on demand, with
the same reason; an id that is not one plain filename, an id longer than 128
bytes of UTF-8 (SPEC-018), a filename that differs from the id the file
declares, a placement whose token id is empty or longer than 128 bytes, and a
file that does not compile are refused there. The walk's own `os.Stat` of
each entry and its skipping of a directory are not run on demand.
`handleMaps` reads the set under the read lock (SPEC-012).

**What reaches a client from a refusal.** Four texts are `map.go`'s own: the
two answers with no maps directory, the not-installed translation, and the
scene-id translation below. Everything else is forwarded as written:
`LoadInstalled`'s refusals, which name the file as `maps/<id>.json` and never
by the path opened, and `Compile`'s, `newEventID`'s and `AppendBatch`'s. That
the forwarded text of a lookup or a compile names no path on the server is
`internal/mapdef`'s and `internal/artlib`'s doing, and the gateway adds none.

**Art is read at each load, from one directory.** `Server.artDir` is passed to
`LoadInstalled` in `mapByID` and to `mapdef.Compile` in `handleLoadMap`, and
`Compile` opens it through `artlib.Open` each time it builds a scene, so art
is read from the directory as it is at each load: `grep -n '^var \|sync\.'`
over `internal/artlib/artlib.go` and `internal/mapdef`'s non-test files prints
only error sentinels and `standardTiles`, so no package-level state holds art
between loads, and art installed or overwritten since boot is used by the next
`load_map`. An empty or absent directory resolves no art and refuses no map.
What an unusable piece costs (a square drawn from its base tile, an object
from its kind, with a warning) and what refuses a map (a sidecar declaring a
`format_version` later than this server understands) are `mapdef.Resolve`'s
and `mapdef.ResolveObjectArt`'s, over `internal/artlib`. A map loaded on a
miss is compiled twice in one request: once in `LoadInstalled`'s dry run,
whose warnings are discarded, and once in `handleLoadMap`, whose warnings
reach the issuer.

**The batch and its stamping.** `mapdef.Compile` returns the batch SPEC-007
describes, its `SceneCreated`'s scene id being the map's id, with `EventId`,
`ParticipantId`, `ActorRole` and `OccurredAt` left zero. `handleLoadMap`
stamps each envelope with an `EventId` from `newEventID`, the issuer's
`ParticipantId` and `ActorRole`, and one `OccurredAt` shared by the batch,
since the store refuses an envelope with no `EventId`, and appends the batch
with `campaign.AppendBatch`, which folds it whole and persists all of it or
none. A refusal by the fold, including SPEC-007's for a placement whose actor
is absent, is answered with the fold's own message.

**The one translated refusal.** An `AppendBatch` failure that `errors.Is`
`engine.ErrSceneExists` is answered with `gateway: load_map: scene id "<id>"
is already in play — a map's scene id is its own id, and a loaded adventure
can claim one too; to load this map as well, install a copy under a new id and
load that`. It says "scene id" because the sentinel knows only that a scene id
is taken, and a scene a loaded adventure created can hold the same id. Every
other append failure keeps its own message.

**The warnings, and who receives them.** `mapdef.Compile`'s warnings ride on
the ok=true result as `Warnings`, and nowhere else: the read loop queues a
result on the issuing connection alone (SPEC-011, SPEC-013), and nothing
broadcasts a warning (`grep -n 'Warnings'` over the package's non-test files
prints only the two ok=true returns, in `adventure.go` and `map.go`). A
refused load carries none. They include art that could not be used and art
whose declared kind disagrees with its square's. That each distinct message
appears once with a count, and that author text in one is clipped, are
`mapdef`'s `warningTally` and `artlib.Clip`.

**The configuration calls.** `WithMaps` takes maps keyed by each map's own id,
validated by the caller through `LoadInstalled`, and does no I/O.
`WithMapsDir` and `WithArtDir` take paths that need not exist and read
nothing; a directory that appears later is found at the next load. `New` sets
`cellPx` to `DefaultCellPx` and `WithCellPx` overrides it (SPEC-012). All four
write without a lock and are called before the server serves; `mapsMu` guards
`maps` alone, and `mapsDir`, `artDir` and `cellPx` are read without it.
`composeServer` calls all four, unconditionally: `WithMaps` with what
`loadMapsDir` returns, which refuses to start the server on a map it cannot
load, `WithMapsDir` with the campaign's `maps` directory and `WithArtDir` with
its `art` directory.

**What this record does not decide.** The batch's shape and that a map
creates no actor are SPEC-007's; the listing, the cell size and the art route
are SPEC-012's; who may issue `load_map` and the command path to the dispatch
are SPEC-013's; what a map file may say, that its filename is its id, how a
reference resolves, what degrades and what refuses, and how warnings collapse
are `internal/mapdef`'s; art names, formats, the clip and confinement to the
directory are `internal/artlib`'s; what the fold refuses is `engine.Apply`'s;
and the boot walk is `cmd/vtt`'s `loadMapsDir`.

## Consequences

A DM, an operator and a tool author are bound by these:

- A map is installed by writing `maps/<id>.json`, with an id of at most 128
  bytes of UTF-8, into the campaign, and enters play by `load_map` with that
  id, with no restart; a map edited on disk after it joined the set is not
  read again.
- An installed map the boot walk would refuse is refused on demand, with the
  same reason.
- A second load of a scene id already in play is refused; the remedy is a
  copy under a new id.
- The actors a map places are added first.
- Art installed since boot is used at the next load; an unusable piece costs
  its square's art and a warning, and a sidecar written for a later format
  refuses the map. A square whose art did not resolve at a map's load stays
  plain, since the scene is in the log as it was compiled, and loading the
  map again is refused as a second load; a piece overwritten under a name the
  scene already carries is served by the art route (SPEC-012).
- Warnings arrive on the issuer's result only.
- No refusal a lookup or a compile produces, and no warning, names a path on
  the server.
- A map that joined the set stays listed after a refused load.

## Requirements

VTT-163, VTT-164, VTT-165, VTT-166, VTT-167, VTT-168, VTT-169, VTT-170,
VTT-171, VTT-172, VTT-173, VTT-174, VTT-175, VTT-279, VTT-280.
