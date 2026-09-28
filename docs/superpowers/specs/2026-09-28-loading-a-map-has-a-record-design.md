# Loading a map has a record

## The problem

How a map reaches play is decided in `internal/gateway/map.go`
(`handleLoadMap`, `mapByID`, `errNoMapsAvailable`) and in the map-and-art
configuration of `internal/gateway/server.go` (the `Server` fields `maps`,
`mapsDir`, `artDir`, `cellPx` and `mapsMu`; `WithMaps`, `WithMapsDir`,
`WithArtDir`, `WithCellPx`), and no specification states it. `load_map`
names a map by id. `mapByID` answers from the in-memory set under `mapsMu`'s
read lock; on a miss with no maps directory configured, a configuration
`cmd/vtt`'s `composeServer` never builds since it always calls `WithMapsDir`,
it refuses with `gateway: no maps available` when the set is empty and
`gateway: unknown map` otherwise; with one configured it calls `mapdef.LoadInstalled(s.mapsDir, id,
s.artDir)` holding no lock, turns `fs.ErrNotExist` into `gateway: unknown map
%q: nothing installed at maps/%s.json in this campaign`, forwards every other
error verbatim, and inserts under the write lock only if no other caller got
there first, so a map written into the campaign's `maps/` while the server
runs becomes loadable without a restart, joins the set when the lookup
validates it (before the compile and the append, so a load refused afterwards
leaves it listed), and two racing lookups keep one entry.
`handleLoadMap` then calls `mapdef.Compile(m, s.artDir)`, which resolves art
against the one flat art directory at load time and returns warnings about
the art it resolves; stamps `EventId`, `ParticipantId`, `ActorRole` and
`OccurredAt` on every envelope; appends the batch with
`campaign.AppendBatch`; translates `engine.ErrSceneExists`, and only that
sentinel, into a sentence telling the DM the scene id is already in play and
how to load the map under a new id; and answers ok=true with the first
sequence and the warnings on the issuer's result. `WithMaps`, `WithMapsDir`,
`WithArtDir` and `WithCellPx` are configuration calls made before the server
serves, and `mapsMu` guards `maps` alone. Each of those sentences lives today
in a ticket's section
(`docs/superpowers/specs/2026-08-12-maps-as-geometry-design.md` §4.3 and
§4.4, `2026-09-01-create-scene-leaves-design.md` §4 and §5,
`2026-09-02-art-is-a-flat-library-design.md` §3.6 and §4), in a report
(`docs/reports/2026-09-02-art-is-a-flat-library.md`), or in a comment block;
SPEC-007 carries the batch's shape and SPEC-012 the `/api/maps` listing, the
cell size and the art route, and nothing carries the rest. SPEC-012 and
SPEC-013 say that `handleLoadMap` has no record, one sentence in SPEC-012
and two in SPEC-013, and SPEC-012 says the package reads the filesystem at
request time "in two places and no other", while `handleLoadMap`'s
`mapdef.Compile` and `handleLoadAdventure`'s `adventure.Compile` both read an
art directory through `artlib.Open` on every load. `map.go` holds 163
comment lines of 241 non-blank (`python3 tools/check-comments.py --report`
gives 67.6 percent, 23 banned lines, 3 blocks over the bound, at `80cb2b7`);
`server.go`'s nine blocks on the fields and methods above hold 130 lines, 7
of them over the bound. One sentence among them is false: `WithCellPx`'s doc
credits ADR-008 with "cmd owns the filesystem", and `docs/adr/008-vtt-cli-shell.md`
says nothing about a filesystem.

## Done looks like

1. `docs/specifications/014-loading-a-map.md` exists with the five headings
   SPEC-007 uses, in the present tense: what `load_map` asks for and what it
   answers; the map set, the lookup on a miss and the lock discipline; what
   reaches a client from a refusal; art read at load time from one directory,
   and what an unusable piece costs; the batch and its stamping; the one
   translated refusal; the warnings and who receives them; the configuration
   calls; and what this record does not decide, pointing at SPEC-007 for the
   batch's shape, at SPEC-012 for the listing, the cell size and the art
   route, and at `internal/mapdef` and `internal/artlib` for the map format,
   the resolution rules and the art names. Every sentence names the symbol
   that holds it, and Phase 4b reads each against the code.
2. `grep -c 'ADR-008' internal/gateway/server.go` prints 0; today it prints 1.
3. Every rule the sort accepts has a row cited by a test under
   `internal/gateway/` that observes it; `task check:requirements-chain`
   prints more than 162 rows and holds; the report names any OPEN row.
4. `python3 tools/check-comments.py --report | grep 'gateway/map\.go'`
   prints `banned 0` and `blocks>6 0`, and every block left in `map.go` and
   in `server.go`'s nine is a warning, a pointer or the doc sentence of an
   exported symbol (Phase 4b's reading, VTT-051); `server.go`'s `blocks>6`
   falls from 18 to 11. The ledger rows are lowered by `--write-ledger` in the
   same commit.
5. No code line changes: the go/scanner token stream of `map.go` and
   `server.go` is identical to `80cb2b7`'s.
6. SPEC-012's one sentence and SPEC-013's two saying `handleLoadMap` has no
   record point at SPEC-014 instead, and SPEC-012's request-time sentence
   names every call that reaches the filesystem.
7. `task check` whole is green.

## Rules this puts on the system

Candidates, one line each, for the sort after sign-off; most have a test
already (`internal/gateway/map_test.go`, `map_internal_test.go`).

- A `load_map` on a server with no maps and no maps directory is refused,
  saying none are available.
- A `load_map` naming a map neither in the set nor installed is refused,
  naming the map.
- A map written into the campaign's maps directory after boot is loadable
  without a restart, and joins the set its lookup validated it into.
- An installed map the boot walk would refuse is refused on demand, with the
  boot walk's reason.
- Two loads of the same newly installed map racing each other leave one
  entry and one scene.
- No refusal and no warning a `load_map` produces names where the campaign
  lives on disk.
- Art installed or overwritten after boot is used by the next `load_map`
  with no restart.
- Art declaring a format this server does not understand refuses the map on
  demand exactly as at boot.
- A `load_map`'s warnings reach the issuer's result.
- A second `load_map` of a map whose scene id is already in play is refused
  with a sentence saying so and how to load it under a new id, and any other
  append failure keeps its own message.
- A map whose art is broken cannot push its result past the connection's read
  limit.
- The shipped campaign's maps resolve their own art.

## What it touches

1. `docs/specifications/014-loading-a-map.md`, new
2. `internal/gateway/map.go`, comments only
3. `internal/gateway/server.go`, the comment blocks on the `maps`, `mapsDir`,
   `artDir`, `cellPx` and `mapsMu` fields and on `WithMaps`, `WithMapsDir`,
   `WithArtDir` and `WithCellPx` only
4. `internal/gateway/map_test.go`, `map_internal_test.go`: citation lines
   where a row is dispensed, and any comment block a re-aimed pointer sits in,
   sorted to the bound (`check:comments` refuses a line added to a block over
   it); `internal/gateway/metadata_test.go`, one block, whose pointer at
   "WithArtDir's own doc comment" aims into a block this cuts
5. `docs/specifications/012-the-read-surface.md` and
   `013-authorization.md`, the sentences that say `handleLoadMap` has no
   record, and SPEC-012's request-time sentence
6. `docs/requirements.md`, rows after sign-off, by the dispenser
7. `tools/comment-ceilings.txt`, by `--write-ledger` only
8. `docs/verification-debt.md`, one Open-debt entry for `handleLoadMap`'s
   untested `Compile` refusal arm, with the report

One component; the specification first, the register with it, the comment
sort after, the ledger last, in one commit; the report in its own.

## Specifications this moves

New: loading a map — the map set, the lookup on demand, art at load time, the
batch, the translated refusal and the warnings.
docs/specifications/012-the-read-surface.md
docs/specifications/013-authorization.md

## What could not be established

- Where the gateway's half ends and `internal/mapdef`'s begins. What a map
  file may say, how an override degrades to its base tile, how warnings
  collapse per art name, and when a format version refuses are `mapdef`'s and
  `artlib`'s, whose own comments hold them (`format.go` 85 percent,
  `resolve.go` 77, `artlib.go` 69). This ticket states what the gateway does
  with `mapdef.Compile`'s and `mapdef.LoadInstalled`'s answers and points at
  the packages; the sort decides whether a row whose test sits in
  `internal/gateway` but whose rule is `mapdef`'s is dispensed here.
- Whether `TestTheShippedCampaignResolvesItsOwnArt` holds a rule or checks
  content.
- The boot-time walk that fills `WithMaps` (`cmd/vtt`'s `loadMapsDir`,
  `composeServer`) is `cmd/vtt`'s; SPEC-014 names it as the caller.
- Which of `map_test.go`'s blocks a re-aimed pointer forces to the bound;
  the plan lists them.
