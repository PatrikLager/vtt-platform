# Loading a map has a record — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-28-loading-a-map-has-a-record-design.md`
**Verified:** 2026-09-28, by `verify-ticket`, an agent that did not write the
ticket, against `80cb2b7` on `chore/spec-014-maps-and-art` (`main` at the
time; the ticket untracked). Verdict: **Passes with gaps.** The gaps are listed
at the end and travel with this plan. This plan does not edit the ticket; where
an item is thin or a sentence of it is loose, the plan decides around it and
says so.

**Goal, in the ticket's words:** `docs/specifications/014-loading-a-map.md`
exists; the false ADR-008 credit is gone from `server.go`; every rule the sort
accepts has a row cited by a gateway test; `map.go` carries only warnings and
pointers with no banned line and no block over the bound; `server.go`'s nine
blocks on the map-and-art fields and configuration calls are pointers,
warnings or exported doc sentences, taking its `blocks>6` from 18 to 11; the
two specifications that say `handleLoadMap` has no record point at SPEC-014;
and no code line changes.

**MapTool (CLAUDE.md rule 9), the answer this plan records.** MapTool loads a
map on the client. `AppActions.LOAD_MAP`, offered to the host and to a GM
(`isAvailable`), opens a `JFileChooser` on the GM's own machine and hands the
file to `MapLoader`, whose `doInBackground` calls `PersistenceUtil.loadMap`.
That reads a packed `.rpmap` (`PackedFile`) holding the `Zone` and every asset
it uses, checks the program version (`versionCheck`), puts the assets into
`AssetManager` (`loadAssets`, which logs and skips an asset missing from the
pack and ignores one marked broken), renames a zone whose name the campaign
already holds (`fixupZoneName`, "Import N of ...") and mints fresh ids all the
way down (`new Zone(z, false)`). `MapTool.addZone` then sends the zone with
`serverCommand().putZone`, and `ServerMessageHandler`'s `PUT_ZONE_MSG` arm puts
it into the server's campaign and relays it to every client with no validation
and no role check (`grep -n 'isGM(\|getRole\|isOwner\|playerOwns'` over
`server/ServerMessageHandler.java` prints nothing). Assets are content-addressed
by `MD5Key`, cached per application (`AssetManager`'s `assetcache` directory
under the app home), uploaded by a remote client with `putAsset`, and served to
a client that asks with `GET_ASSET_MSG`, chunked by an `AssetProducer`; an
asset the server cannot find is answered with `Asset.createBrokenImageAsset`, a
placeholder rather than a refusal. So: the degrade is the shape this platform
already has (an unresolved piece costs its square's art and a warning, never
the map, `mapdef.Resolve`), and nothing further is borrowed. Refused, each with
its reason: the client-side load and a server that stores whatever zone a
client sends are the distribution model rule 9 names (`internal/gateway/seat.go`);
renaming a colliding map and minting new ids is the opposite of this
platform's ruling that a map's filename is its scene id, so a second load is
refused and the remedy is a copy under a new id; MapTool has no server-side
maps directory to look in, so install-then-load has no counterpart to borrow;
and a map file that carries its own assets by hash is what the flat,
per-campaign `art/` directory named by filename replaced. The verifier ran the
search by command in `~/dev/RPTool/maptool` (`grep` over
`client/AppActions.java`, `util/PersistenceUtil.java`, `client/MapTool.java`,
`server/ServerMessageHandler.java`, `model/AssetManager.java`,
`model/Asset.java`). Where this is written is D3.

## Verification, check by check

1. **Every path resolves — by command.** `internal/gateway/map.go` declares
   `errNoMapsAvailable`, `handleLoadMap` and `mapByID`; `server.go` declares the
   `Server` fields `maps`, `mapsDir`, `artDir`, `cellPx` and `mapsMu` and the
   methods `WithMaps`, `WithMapsDir`, `WithArtDir` and `WithCellPx`;
   `internal/mapdef` declares `LoadInstalled` (`installed.go`) and `Compile`,
   `BuildSceneCreated` and `warningTally` (`compile.go`); `campaign.AppendBatch`
   and `engine.ErrSceneExists` exist; `cmd/vtt` declares `loadMapsDir`
   (`maps.go`) and `composeServer` (`serve_compose.go`). The three source
   tickets exist with the sections named (`### 4.3 Maps are their own object`,
   `### 4.4 Validation, at boot, fail loud`; `## 4. Install, then load`, `## 5.
   Lookup on demand`; `### 3.6 Art resolves on demand`, `## 4. Unresolvable
   art degrades; it does not refuse`), and so does
   `docs/reports/2026-09-02-art-is-a-flat-library.md`.
   `TestTheShippedCampaignResolvesItsOwnArt` is in `map_test.go`;
   `metadata_test.go`'s "WithArtDir's own doc comment" is in the doc of
   `TestArtWithNoArtDirectoryConfiguredIs404`. The three coverage figures the
   ticket quotes are `--report`'s (`format.go` 85.2, `resolve.go` 76.8,
   `artlib.go` 69.4). `docs/specifications/014*` does not exist.
2. **"Done" is an observation — by command where a command exists.** Item 1:
   the file does not exist. Item 2: `grep -c 'ADR-008'
   internal/gateway/server.go` prints 1, and `docs/adr/008-vtt-cli-shell.md`
   says nothing about a filesystem (`grep -n -i 'file\|disk\|path'` over it
   finds only the Taskfile, a config file that does not exist yet, and "one
   file per command"). Item 3: `task
   check:requirements-chain` prints `162 rows, 193 test files, 7
   specifications; every citation resolves and every row's evidence holds`,
   and neither map test file carries a citation (`grep -c 'VTT-'` prints 0 and
   0). Item 4: `--report` prints `banned 23 blocks>6 3` for `map.go` and
   `blocks>6 18` for `server.go`. Item 5 is an invariant (D11 holds it at
   every task). Item 6: the sentences exist, one in SPEC-012 and two in
   SPEC-013 (D14). Item 7 is the gate. The half of items 1 and 4 that says
   every surviving block is a warning, a pointer or a doc sentence is Phase
   4b's reading, as the ticket says.
3. **Each rule is breakable — a reading, then probes.** D5 names, per
   candidate, the test that goes red and the one edit that reds it. Every edit
   named there was run by the verifier in a scratch clone, compiles, and reds
   the named test, with one probabilistic exception (row F, red in 37 of 40
   runs) and one edit that lives outside the gateway (row M, in
   `internal/mapdef/resolve.go`). Three of the ticket's candidates are loose
   as worded and D5 rewords them (rows D, E, I); two carry a half that is
   another package's (rows F and M); one is content (row P).
4. **Scope matches the claim — by command, then a reading.** No code line
   changes, so callers do not widen the work. `handleLoadMap` is called from
   `handleCommand` alone; `mapByID` from `handleLoadMap` and
   `map_internal_test.go`; the four `With*` methods from `composeServer` alone
   in production (`grep -rn 'WithMapsDir(\|WithMaps(\|WithArtDir(\|WithCellPx('`
   outside tests prints only `cmd/vtt/serve_compose.go`), which calls all four
   unconditionally. What could widen the work is a pointer INTO a block being
   cut, from a file the ticket does not list; the search is under
   Measurements and finds one, the one the ticket names in `metadata_test.go`.
   Neither `map_test.go` nor `map_internal_test.go` holds a pointer into a cut
   block, so the ticket's open question on which of their blocks a re-aim
   forces to the bound is answered: none.
5. **No recorded decision is contradicted — a reading.** SPEC-007 (the batch
   section: one `SceneCreated`, one `TokenPlaced` per placement, a map never
   creates actors, the batch accepted or refused whole; the ordering section:
   a result's `sequence` names the first event), SPEC-009 (`actor_role`
   stamped by the gateway and the batch handlers), SPEC-011, SPEC-012 and
   SPEC-013 were read against the ticket, and every sentence in them about
   `load_map`, maps or art was checked against the code. One is false, and the
   ticket would contradict it silently: SPEC-012's boot paragraph says "This
   package reads the filesystem at request time in two places and no other:
   `handleArtFile` opens the art directory, and `mapByID` in `map.go` probes
   the campaign's maps directory on a lookup miss." `handleLoadMap` calls
   `mapdef.Compile(m, s.artDir)` on every load, and `BuildSceneCreated` calls
   `artlib.Open(artDir)` unconditionally, which reads the directory;
   `mapByID`'s `LoadInstalled` reads the art directory as well as the maps
   directory; and `handleLoadAdventure`'s `adventure.Compile` reads the
   adventure's own `ArtDir` through the same `BuildSceneCreated`. SPEC-014
   states that art is read at every load, so SPEC-012's sentence must change
   with it (gap 1, Q5). The ticket overturns no decision. `docs/adr/` holds no
   sentence about the maps directory or `load_map`.
6. **The records the work moves are named — by command, then a reading.** The
   section exists and reads `New: loading a map ...` with SPEC-012 and
   SPEC-013 beside it; both paths resolve. The reading: the work changes no
   behaviour, so the only sentences that go stale are the ones that say
   `handleLoadMap` has no record — one in SPEC-012, two in SPEC-013, not one
   (D14) — plus SPEC-012's false request-time sentence (check 5).
   `docs/verification-debt.md` holds nothing about maps.

## Measurements this plan stands on

All at `80cb2b7`, by command, run by the verifier.
`python3 tools/check-comments.py --report`, the checker's own `measure`
(`cite = cc.cite_pattern(cc.register_tag()); measure(lines, False, cite)`),
and `tools/comment-ceilings.txt`:

| File | Comment / non-blank | Share | Ceiling | Banned | Blocks > 6 | Cites |
|---|---|---|---|---|---|---|
| `internal/gateway/map.go` | 163 / 241 | 67.6 | 67.7 | 23 | 3 | 0 |
| `internal/gateway/server.go` | 473 / 1,069 | 44.2 | 44.3 | 29 | 18 | 0 |
| `internal/gateway/map_test.go` | 646 / 1,498 | 43.1 | 43.2 | 69 | 24 | 0 |
| `internal/gateway/map_internal_test.go` | 29 / 118 | 24.6 | 24.6 | 3 | 1 | 0 |
| `internal/gateway/metadata_test.go` | 393 / 1,573 | 25.0 | 25.0 | 34 | 18 | 28 |

The ticket's figures for `map.go` are these. Token counts, D11's instrument
(comments dropped): `map.go` 596, `server.go` 4596, `map_test.go` 7029,
`map_internal_test.go` 620, `metadata_test.go` 9214.

**`map.go`, by block.** Five blocks, 163 lines, 23 banned:
`errNoMapsAvailable`'s doc (6, 1 banned); `handleLoadMap`'s doc (74, 15
banned, over); the stamping block inside `handleLoadMap` above `now :=
timestamppb.Now()` (4, 0); the block above the `engine.ErrSceneExists` arm
(24, 1 banned, over); `mapByID`'s doc (55, 6 banned, over). No package doc.

**`server.go`, the nine blocks in scope**, by the symbol each sits on, with the
line count `measure` gives and the banned lines in each: the `maps` field (16,
5), `mapsDir` (5, 1), `artDir` (13, 1), `cellPx` (7, 0), `mapsMu` (3, 0);
`WithMaps`' doc (16, 6), `WithMapsDir`'s (25, 2), `WithArtDir`'s (31, 4),
`WithCellPx`'s (14, 2). 130 lines, 21 banned, seven over the bound (`maps`,
`artDir`, `cellPx`, and the four methods); 18 − 7 = 11, the ticket's figure.
The eleven that remain over the bound sit on: the `writeTimeout` field,
`announcePresence`, `announceDeparture` (its doc and its body),
`revoked`, `announcePromotion`, `handleRemoveActor`, `handleJoinDoor` (its
body), `handlePromotion` (its doc and its body), `credentialGone`.

**The false sentence, against the record.** `WithCellPx`'s doc says campaign.json's
`cell_px` is "read by cmd/vtt (ADR-008: cmd owns the filesystem)". ADR-008
decides a CLI shell pattern (cobra, thin commands, viper deferred) and never
mentions a filesystem. The true reason the gateway takes a number, not a path,
is already SPEC-012's: `.go-arch-lint.yml` gives `gateway` no edge to
`campaigncfg`.

**Every sentence of the ticket's first paragraph, against its symbol.** True as
written: the set read under `mapsMu.RLock`; with `mapsDir == ""`,
`errNoMapsAvailable` for an empty set and `gateway: unknown map %q` otherwise;
`LoadInstalled(s.mapsDir, id, s.artDir)` with no lock held; `fs.ErrNotExist`
translated to `gateway: unknown map %q: nothing installed at maps/%s.json in
this campaign`; every other error forwarded; `mapsMu.Lock`, a second look, and
insertion only when absent (the nil set made first); `mapdef.Compile(m,
s.artDir)`; the four stamped fields; `campaign.AppendBatch`; `errors.Is(err,
engine.ErrSceneExists)` the only translated append failure; ok=true with
`Sequence: firstSeq` and `Warnings`. Three compressions: "returns warnings for
art it could not use" — `Resolve` also warns on art it does use, when the
piece's declared kind disagrees with the square's (`Art: art` is kept);
"becomes loadable ... and two racing loads keep one entry" — the entry is made
when the lookup validates the map, before the compile and the append, so a
load refused afterwards (an absent actor, a scene collision, art changed
between the dry run and the live compile) leaves the map in the set and on
`/api/maps`; and "on a miss with no maps directory configured" — no production
server is built that way, since `composeServer` always calls `WithMapsDir`, so
`vtt serve` never answers `gateway: no maps available` and an empty campaign's
DM is told `gateway: unknown map "x": nothing installed at maps/x.json in this
campaign`. `errNoMapsAvailable`'s doc, which this sweep cuts, and
`TestLoadMapNoMapsConfiguredCleanError`'s doc, which it does not, both say the
opposite of that last point.

**Pointers into the blocks this sweep cuts, from outside them.** Searched over
`internal/`, `cmd/` and `client/src` for every comment naming `mapByID`,
`handleLoadMap`, `WithMaps`, `WithMapsDir`, `WithArtDir`, `WithCellPx`,
`errNoMapsAvailable`, `mapsMu`, `map.go` or `server.go` together with "doc",
"comment", "note", "says", "see" or "per":

1. `metadata_test.go`, the 5-line doc of `TestArtWithNoArtDirectoryConfiguredIs404`:
   "A campaign that has installed no art is ordinary (WithArtDir's own doc
   comment)". Under the bound. The same block says the harness builds a server
   with no `WithArtDir` at all, which is false: the harness boots through
   `composeServer`, which wires it unconditionally (Q6).

Inside the cut blocks, and going with them: the `mapsDir` field's "see
mapByID's own doc comment", the `maps` field's "its doc comment says why",
`mapsMu`'s "each says so at its own field above", `WithMaps`' "see WithArtDir",
`WithArtDir`'s "for the reason WithMapsDir gives above", and `mapByID`'s "the
state map_test.go's own newMapFixture still pins". Pointers that name a symbol
or a file whose code survives, not re-aimed: `internal/mapdef/installed.go`
(`mapByID` forwards all but the not-installed error; true of the code),
`installed_test.go`, `internal/mapdef/load_test.go` (`handleLoadMap`'s refusal
arm), `internal/engine/apply.go` (`handleLoadMap` does no pre-check),
`internal/adventure/compile.go`, `internal/harness/engine.go` and
`engine_test.go`, `internal/harness/soak_test.go`, `server.go`'s
`handleRemoveActor` doc and body (`handleLoadMap` as the precedent for the
batch and its stamping loop), `authz_test.go`'s `loadMapCmd` doc,
`adventure_test.go`, `remove_actor_test.go`, `scenario_test.go`,
`metadata.go`'s `handleMaps` body, and under `cmd/vtt` `maps.go`,
`maps_test.go`, `serve_compose.go`, `client_soak.go`, `client_e2e_test.go`,
`mcp_ruleset_e2e_test.go`, `library_test.go`, `harness_boot.go`,
`maps_e2e_test.go`. `map_test.go` and `map_internal_test.go` point at code and
at each other, never into a cut block (`grep -n -iE "comment|doc says|its
doc|own note|paragraph"` over both, each hit read).

**Adjudication keys.** `tools/mutation-equivalents.txt` holds no key naming
`internal/gateway/map.go` or `internal/gateway/server.go` (its two `server.go`
keys are `internal/mcp`'s), and no entry's text cites either by coordinate;
`tools/ts-mutation-equivalents.txt` names neither. `python3
tools/check_mutation_test.py -q` (107 tests) and `python3
tools/check_ts_mutation_test.py -q` (50 tests) print `OK` at the base. So,
unlike the authorization change, no key moves; the self-tests are still run
after every edit and before the commit, because this is the claim a moved
line would falsify.

**Records and gates.** `docs/specifications/` holds 007 to 013; 014 is free.
`docs/requirements.md` has 162 rows, the last `VTT-162`. `python3
tools/check-comments.py main` ends `239 files, 0 added comment lines, 239
ledger rows; clean`, after the standing notice on `cmd/vtt/library_test.go`.
`python3 tools/check-doc-owner.py .` ends `79 files, every doc comment sits on
its own function`. `gofmt -l internal/gateway/` prints only
`scenario_test.go`. `requirement-id` is at
`~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/` and, run bare,
prints `a requirement needs its sentence, not just an id`. Load was 1.3 at
verification.

**What no test observes, by probe.** In a scratch clone:
`handleLoadMap`'s `mapdef.Compile` refusal arm made to answer `Ok: true` leaves
`go test ./internal/gateway/` green (and `./cmd/vtt/ -run 'Map|Art|Load'`); the
arm is reachable by a map in the set whose art is overwritten with a sidecar
declaring a later format, and its refusal then names the art and not the map
(gap 2). Deleting `env.ParticipantId = p.ID` leaves
`TestLoadMapProducesBatchCarryingTilesAndObjects` green; that test checks
`EventId`, `ActorRole` and `OccurredAt` only.

**`check:doc-owner`'s first-word rule.** It refuses a doc block above a
function whose first word names ANY function declared outside a test file,
lowercase names included (`declared_functions`). The single-word capitalised
names at `80cb2b7` are unchanged from the authorization plan's list: `Accept`,
`Accepted`, `Actors`, `Append`, `Apply`, `Arm`, `Authorize`, `Blocked`,
`Blockers`, `Clear`, `Clip`, `Close`, `Compile`, `Dial`, `Error`, `Eval`,
`Events`, `Fold`, `Handler`, `List`, `Load`, `Lookup`, `New`, `Notify`,
`Open`, `Parse`, `Project`, `Refs`, `Resolve`, `Revoke`, `Roll`, `Run`,
`Scenes`, `Scopes`, `Snapshot`, `State`, `Step`, `String`, `Subscribe`,
`Tokens`, `Unwrap`, `Validate`, `Verify`, `Write`. Multi-word exported names
count too: `LoadInstalled`, `BuildSceneCreated`, `AppendBatch`. So no warning
above `mapByID` or `handleLoadMap` may open with `Load`, `LoadInstalled`,
`Compile`, `Lookup`, `Open`, `Resolve`, `Append` or `New`. `Hold`, `Keep`,
`Never`, `Check`, `Translate`, `Forward`, `Stamp`, `Answer`, `Put`, `Say`,
`Return`, `Guard`, `Take` name no function. A comment inside a body or on a
struct field is not read by that gate.

## Constraints that bind every task

- `CLAUDE.md` rule 10 and SPEC-010: a comment is an imperative warning, a
  pointer, or the one-line doc sentence of an exported symbol. VTT-051 is held
  by the Phase 4b reading; VTT-050, VTT-052, VTT-053 and VTT-055 by
  `check:comments`; VTT-059 says a citation line moves no share.
- `CLAUDE.md` rule 2: no gate is weakened. The ledger only goes down, only
  through `--write-ledger`.
- `CLAUDE.md` rule 8, narrowed by rule 10 for code: a comment points at a
  specification by number, a requirement by id, a test or symbol by name, or a
  report by its `docs/` path; never a line number, a date, a commit hash, a
  plan, a task, `§`, `#NNN`, an ADR or a `.superpowers/` path. This plan and
  the report name blocks by the symbol they sit on.
- `CLAUDE.md` rule 3: the contract is not touched; `LoadMap`, `CommandResult`
  and its `warnings` field are SPEC-007's.
- `CLAUDE.md` rule 4: one fold. SPEC-014 says the batch is refused by
  `campaign.AppendBatch`'s fold of it and never describes a second place that
  applies a map.
- `CLAUDE.md` rule 5: SPEC-014 names no rules concept; a map is terrain,
  objects and placements.
- `CLAUDE.md` rule 9: the answer above is recorded before any task runs.
- SPEC-008: ids come from `requirement-id`, after sign-off, never by hand.
- The `specification` skill's form and `catches.md`: five headings, present
  tense, no `Why`, no `Rejected`, no measurement, no line number, no past-tense
  account that is not about the code, no path outside the project, one
  decision per file, no claim taken from a comment without reading the code
  under it (item 8, the one to watch: the three sentences above that the
  ticket compressed all started life in a comment), every "only/every/none"
  with its search named (item 10), the old text not left beside the new (item
  13, for the SPEC-012 and SPEC-013 edits).
- The `requirements` skill: one thing, what not how, breakable, measurable,
  named by a check or knowingly OPEN; refuse more than you accept.
- SPEC-007, SPEC-009 and SPEC-011 are not edited; SPEC-012 and SPEC-013 only
  at the sentences D14 names.
- The ticket: no code line changes (D11 is the check); `map.go`'s five blocks,
  `server.go`'s nine, citation lines in the two map test files, the one
  `metadata_test.go` block, the two specifications' sentences, the register,
  the ledger.
- `internal/mapdef`, `internal/artlib`, `internal/engine`, `internal/campaign`,
  `internal/adventure`, `cmd/vtt`, `client/src`, `contract/`, every other file
  under `internal/gateway` and the three source tickets are not touched.
  `docs/reports/` gains only this ticket's report.

## Decisions this plan makes

**D1. SPEC-014's sections, each lifted from a named source and held by named
symbols.** Forced by ticket item 1 and the `specification` skill's step 3.
File `docs/specifications/014-loading-a-map.md`. Title: `SPEC-014: A map
enters play by its id, from what the campaign holds when it is asked for`.
Under "How it works", in this order, one bold-led paragraph each:

| Section | Source (where it lives today) | Symbols that hold it | Rows (D5) |
|---|---|---|---|
| What `load_map` asks for and answers | `handleLoadMap`'s doc; SPEC-007's ordering section | `LoadMap.map_id` names the map; `handleCommand` dispatches to `handleLoadMap` after authorization (SPEC-013, pointed at); every failure is an ok=false `CommandResult` whose `Error` is the text below, on a connection that stays; success is ok=true with `Sequence`, the first sequence `campaign.AppendBatch` assigned (the convention SPEC-007 states), and `Warnings` | N, J |
| The map set and the lookup on a miss | `mapByID`'s doc (its three steps); the `maps`, `mapsDir`, `mapsMu` field docs; create-scene-leaves ticket §4, §5 | `Server.maps`, filled by `WithMaps` before serving; `mapByID` reads it under `mapsMu.RLock` and releases; with `mapsDir == ""` a miss is refused (`errNoMapsAvailable` for an empty set, `gateway: unknown map %q` otherwise), which no `vtt serve` reaches because `composeServer` always calls `WithMapsDir`; otherwise `mapdef.LoadInstalled(s.mapsDir, id, s.artDir)` runs with no lock held, then `mapsMu.Lock`, a second look, the existing entry returned if another lookup made one, else the set made if nil and the map inserted; every racing caller holds one `*mapdef.Map`; the set only grows, and a map joins it when the lookup validates it, whatever the compile and the append then answer; `handleMaps` reads it under the read lock (SPEC-012, pointed at) | B, C, D, E, F (A per Q2) |
| What reaches a client from a refusal | `mapByID`'s "what reaches a client" paragraph; `LoadInstalled`'s doc (pointed at) | four texts are the gateway's own (the two no-directory answers, the not-installed translation, the scene-id translation); everything else is forwarded verbatim: `LoadInstalled`'s (which names a file as `maps/<id>.json` and never by the path opened, and bounds author text through `artlib`), `Compile`'s, `newEventID`'s and `AppendBatch`'s; that the forwarded text names no path is `internal/mapdef`'s and `internal/artlib`'s, and the gateway adds none | B, G |
| Art is read at the load, from one directory | `handleLoadMap`'s doc; the `artDir` field doc; `WithArtDir`'s doc; art-is-a-flat-library ticket §3.6 and §4 | `Server.artDir` is passed to `LoadInstalled` in `mapByID` and to `mapdef.Compile` in `handleLoadMap`, so art is read from the directory as it is at each load and nothing is cached; an empty or absent directory resolves nothing and refuses nothing; what an unusable piece costs (its square drawn from the base tile, with a warning) and what refuses a map (a sidecar declaring a later format) are `mapdef.Resolve`'s and `ResolveObjectArt`'s over `artlib`, pointed at; a map loaded on a miss is compiled twice, once by `LoadInstalled`'s dry run, whose warnings it discards, and once by `handleLoadMap`, whose warnings reach the issuer | I, E, H |
| The batch and its stamping | the stamping block in `handleLoadMap`; SPEC-007's batch section (pointed at); SPEC-009's `actor_role` sentence (pointed at) | `mapdef.Compile`'s envelopes, whose shape SPEC-007 states; each is stamped with an `EventId` from `newEventID`, the issuer's `ParticipantId` and `ActorRole`, and one `OccurredAt` shared by the batch, because `Compile` leaves them zero and the store refuses an envelope with no `EventId`; `campaign.AppendBatch` folds the whole batch and persists all of it or none; a placement whose actor the world lacks is refused there with the fold's own message | N (stamping: prose, D5's O) |
| The one translated refusal | the block above the `engine.ErrSceneExists` arm | `errors.Is(err, engine.ErrSceneExists)` alone is rewritten, to `gateway: load_map: scene id %q is already in play ...` naming the requested map id and the remedy (install a copy under a new id and load that); it says "scene id" because the sentinel knows only that a scene id is taken, and a loaded adventure's scene can hold the same id; every other append failure keeps its own message | K, L |
| The warnings and who receives them | `handleLoadMap`'s warnings paragraphs; art-is-a-flat-library ticket §4 | `Warnings` on the ok=true result and nowhere else: the read loop queues the result on the issuing connection (SPEC-013, SPEC-011, pointed at) and nothing broadcasts one; a refused load carries none; one line per distinct message with its count, and the bound on author text, are `mapdef`'s `warningTally` and `artlib.Clip`, pointed at | J, H, M |
| The configuration calls | the four `With*` docs; the five field docs | `New` sets `cellPx` to `DefaultCellPx` and `WithCellPx` overrides it (SPEC-012, pointed at); `WithMaps` takes maps keyed by each map's declared id and does no I/O and no validation, the caller having validated each through `LoadInstalled`; `WithMapsDir` and `WithArtDir` take paths that need not exist and read nothing, a directory that appears later being found at the next load; all four write without a lock and are called before the server serves; `mapsMu` guards `maps` alone; `composeServer` calls all four unconditionally, `WithMaps` with what `loadMapsDir` returns (`cmd/vtt`, named as the caller) | none |
| What this record does not decide | the ticket's "What could not be established" | the batch's shape and that a map creates no actor (SPEC-007); the listing, the cell size and the art route (SPEC-012); who may issue `load_map` and the command path (SPEC-013); what a map file may say, that its filename is its id, how a reference resolves, what degrades and what refuses, and how warnings collapse (`internal/mapdef`); art names, formats, the clip and confinement to the directory (`internal/artlib`); what the fold refuses (`engine.Apply`); the boot walk (`cmd/vtt`'s `loadMapsDir`) | none |

Each sentence names the symbol that holds it, is checked against the code
before it is written and re-read by Phase 4b (D16). Status: `Accepted.
Implemented by internal/gateway/map.go (handleLoadMap, mapByID,
errNoMapsAvailable) and internal/gateway/server.go (the maps, mapsDir, artDir,
cellPx and mapsMu fields of Server; WithMaps, WithMapsDir, WithArtDir,
WithCellPx), over internal/mapdef's LoadInstalled and Compile and
internal/campaign's AppendBatch; fed by cmd/vtt/serve_compose.go's
composeServer; pinned by internal/gateway/map_test.go and
map_internal_test.go.` "Principles served" says what SPEC-012 and SPEC-013
say: no blueprint; the principle missing from the record rather than absent
from the system (here: a place enters play when the DM asks for it, from what
the campaign holds at that moment, and never from what the server read when it
started). "Consequences" holds what a DM, an operator and a tool author are
bound by: a map is installed by writing `maps/<id>.json` into the campaign and
enters play by `load_map` with that id, no restart; an installed map that the
boot walk would refuse is refused on demand with the same reason; a second
load of a scene id already in play is refused, and the remedy is a copy under
a new id; the actors a map places are added first; art installed since boot is
used at the next load, an unusable piece costs its square's art and a warning,
and a sidecar written for a later format refuses the map; warnings arrive on
the issuer's result only; no lookup or art failure names a path on the server;
a map that joined the set stays listed after a refused load. "Requirements"
lists the ids the sort dispenses (D6) and nothing else.

**D2. What SPEC-014 points at and does not restate.** Forced by `catches.md`
item 13 and the ticket's item 1. SPEC-007 owns the batch's shape, that a map
creates no actor and that the batch is refused whole, and the `Sequence`
convention; SPEC-014 names `mapdef.Compile` and `campaign.AppendBatch` and says
"SPEC-007". SPEC-009 owns what `actor_role` means. SPEC-011 owns the read loop
and the pump. SPEC-012 owns `/api/maps` (including `handleMaps`' read lock and
the cell-size fallback), `DefaultCellPx`, `WithCellPx`'s number and the art
route; SPEC-014 names `cellPx` and `WithCellPx` as configuration and points.
SPEC-013 owns who may issue `load_map` (the table's row) and the command path
up to the dispatch. `internal/mapdef` and `internal/artlib` own the map format,
filename-is-id, `idIsAFilename`, the case-only-match refusal, resolution,
degrade-versus-refuse, `warningTally` and the clip; SPEC-014 states what the
gateway does with `LoadInstalled`'s and `Compile`'s answers and names the
functions. `cmd/vtt`'s boot walk is named as the caller that fills the set,
and `artRootIsOpenable` and `artlib.Validate` stay `cmd/vtt`'s.

**D3. Provenance stays out of the record.** Forced by `catches.md` items 3 and
12. The MapTool answer above is the rule-9 answer and lives in this plan and
the report. SPEC-014 states filename-is-id and refusal-on-collision as facts
about this system and names no other project, no ticket section and no ruling
date. Q1.

**D4. The false ADR-008 credit.** Forced by ticket item 2. `WithCellPx`'s doc
becomes one sentence pointing at SPEC-012, which already states the true
reason; `grep -c 'ADR-008' internal/gateway/server.go` prints 0. The other
ADR-008 credits (six under `cmd/vtt`) are outside this ticket and named in the
report, as the read-surface report already named them.

**D5. The sort's starting point is the table below; nothing is an id until
sign-off.** Forced by SPEC-008 and the `requirements` skill. Rows are lettered
so nothing here reads as an id. Each line: the rule as a what; the outcome
proposed; the evidence, verified against the test bodies; the one edit that
reds it. Every edit in the last column was run by the verifier in a scratch
clone of `80cb2b7`: it compiles, and the named test fails; the inverse edit
restored the file each time.

| # | Rule (what) | Proposed | Evidence (verified against the body) | The edit that reds it (run) |
|---|---|---|---|---|
| A | A `load_map` to a server with no maps directory and no maps is refused, saying none are available. | OPEN, Q2 — leaning refuse: no production server is built this way | `TestLoadMapNoMapsConfiguredCleanError` (`newMapFixture(t, false)`: no `WithMaps`, no `WithMapsDir`) | `if loaded == 0` made `if loaded < 0` in `mapByID` |
| B | A `load_map` naming a map the server neither holds nor finds installed is refused, naming the map. | accept | `TestAnUnknownMapIsRefusedByName` (with a maps directory: names `nowhere`, no `no such file`, no directory), `TestLoadMapUnknownIdCleanError` (without one) | `errors.Is(err, fs.ErrNotExist)` made `fs.ErrExist` (reds the first); `loaded == 0` made `loaded >= 0` (reds the second) |
| C | A map written into the campaign's maps directory after boot is loadable, with no restart. | accept — the ticket's third candidate split | `TestAMapInstalledAfterBootIsLoadable` (booted with no `maps/`; the `SceneCreated` read back on the agent's connection) | `if s.mapsDir == ""` made `if s.mapsDir == "" \|\| id != ""` |
| D | A map loaded on demand joins the map set `/api/maps` lists. | accept — split from C; worded on what the tests observe, since the entry is made at the lookup, not after the append (SPEC-014 states the difference; gap 4) | `TestAMapInstalledAfterBootJoinsTheListing`, `TestConcurrentLookupsOfANewlyInstalledMapCompileItOnce` (the set holds exactly the one map) | `s.maps[id] = installed` deleted |
| E | An installed map the boot walk would refuse is refused on demand, with the boot walk's reason. | accept — the ticket's fourth and eighth candidates as the gateway's half: the probe runs the boot walk's function with the boot walk's art root. "Does not compile" is reworded: the fixture fails `LoadInstalled`'s filename-is-id check before any compile. "Stays out of the set" is refused as a clause: `LoadInstalled` returns a nil map on every error, so no gateway edit short of inventing a map reds it; SPEC-014 states it | `TestAnInstalledButBrokenMapIsRefusedAndStaysUnloaded` (id mismatch, both strings, twice; the listing empty), `TestAMapWithArtFromALaterFormatIsRefusedOnDemandExactlyAsAtBoot` (`map "level-5"` and `maps/level-5.json` in the refusal) | `LoadInstalled(s.mapsDir, id, s.artDir)` made `LoadInstalled(s.mapsDir, id, "")` (reds the second, and `TestNoRefusalTellsAClientWhereTheCampaignLives`); `LoadInstalled(...)` made `mapdef.Load(s.mapsDir + "/" + id + ".json")` (reds the first) |
| F | Racing lookups of one newly installed map all answer with the same map, and the set gains one entry. | accept, Q8 — the ticket's fifth candidate's first half; "and one scene" is refused as the fold's (`ErrSceneExists`), and its wire test is cited under K | `TestConcurrentLookupsOfANewlyInstalledMapCompileItOnce` (six lookups and four listings behind one barrier; pointer equality; `len(s.maps) == 1`) | the second look made `if won, ok := s.maps[id]; ok && false` — red in 37 of 40 runs (`-count=40`), not every run: it needs two lookups inside `LoadInstalled` at once |
| G | No refusal a `load_map`'s lookup or compile produces names a path on the server. | accept — the sixth candidate split by channel, and narrowed: an append failure's text is forwarded verbatim and no test drives one (gap 7). The mechanism is `mapdef`'s `unpath` and display name and `artlib`'s `bareCause`; the promise is this command's, like VTT-127's for the art route | `TestNoRefusalTellsAClientWhereTheCampaignLives` (seven refusals, each for a different reason, none naming the maps directory, the art directory or the temp root, each naming the id), `TestAnUnknownMapIsRefusedByName` | the forwarding `return nil, err` in `mapByID` made `return nil, fmt.Errorf("%s: %w", s.mapsDir, err)` |
| H | No warning a `load_map` carries names a path on the server. | accept — the sixth candidate's other channel | `TestNoWarningTellsAClientWhereTheCampaignLives` (an unopenable art root; every other art warning, each producer asserted to have fired) | `Warnings: warnings` made `Warnings: append(warnings, s.artDir)` |
| I | A `load_map` resolves art against the campaign's art directory as it stands at the load, so art installed after boot is drawn with no restart. | accept — the seventh candidate, narrowed: "overwritten" has no gateway test (the art route's VTT-131 is SPEC-012's) and stays prose. The run edit shows "against the directory"; "as it stands at the load" is held by the gateway holding a path and caching nothing, which no one-token edit undoes | `TestArtInstalledAfterBootDrawsWithoutARestart` (no warnings; `late-stone` on the wire) | `mapdef.Compile(m, s.artDir)` made `mapdef.Compile(m, "")` |
| J | A `load_map`'s warnings ride on the issuer's result. | accept — the ninth candidate; "and nobody else" is prose, as VTT-135's was | `TestALoadMapWarningReachesTheIssuer` (a kind mismatch, `masonry-1` named) | `Warnings: warnings` made `Warnings: warnings[:0]` (deleting the field does not compile: `warnings` becomes unused) |
| K | A second `load_map` of a scene id already in play is refused with a sentence naming the map, saying its scene id is in play and how to load it under a new id. | accept — the tenth candidate's first half | `TestASecondLoadTellsTheDMTheMapIsLoadedAndHowToReload` (`"cellar"`, `already in play`, `new id`, not `engine:`, not the over-claim), `TestLoadMapDoubleLoadCollisionRejectedCleanNotPoisoned`, `TestTwoRacingLoadsOfANewlyInstalledMapProduceOneScene` (the loser's text) | `errors.Is(err, engine.ErrSceneExists)` made `errors.Is(err, engine.ErrUnknownVariant)` (deleting the arm does not compile: `engine` becomes unused) |
| L | A `load_map` the log refuses for any reason but a scene collision is answered with the log's own message. | accept — the tenth candidate's second half | `TestANonCollisionFailureKeepsItsOwnMessage` (a placement for an actor nobody added: `unknown actor`, neither `already in play` nor `new id`) | `errors.Is(err, engine.ErrSceneExists)` made `... \|\| err != nil` |
| M | A `load_map` result stays readable whatever the art's warnings. | OPEN, Q3 — leaning accept, for parity with VTT-136; the edit that reds it is `internal/mapdef`'s, not the gateway's | `TestBrokenArtCannotPushAResultPastTheReadLimit` (twelve 20,000-byte kinds; the result must be read under the client's limit, ok=true, warnings present, their total under 32 KiB) | in `internal/mapdef/resolve.go`, the mismatch warning's `artlib.Clip(piece.Kind, artlib.MaxFragment)` made `piece.Kind` (`message too big: read limited at 204801 bytes`) |
| N | A `load_map` result carries the first sequence of the batch it appended. | OPEN, Q4 — leaning accept, for parity with VTT-137; SPEC-007 states the convention | `TestLoadMapProducesBatchCarryingTilesAndObjects` (`res.Sequence != 2`) | `Sequence: firstSeq` made `Sequence: firstSeq + 1` |

Refused, or not this sort's, each with its reason. **O**, every envelope a
`load_map` appends carries an event id, the issuer's id and role and a time —
SPEC-012 states the same stamping for the other two batch handlers as prose
and dispensed no row, SPEC-009 owns `actor_role`, and the test observes three
of the four fields (deleting `ParticipantId`'s assignment leaves it green);
SPEC-014 states the stamp and names the fields. **P**, the shipped campaign's
maps resolve their own art (`TestTheShippedCampaignResolvesItsOwnArt`) —
content, not a rule: the edit that reds it is a file deleted under
`campaigns/example/art`, and the test stays uncited. **Q**, a load produces one
`SceneCreated` with its tiles and objects, then one `TokenPlaced` per
placement (`TestLoadMapProducesBatchCarryingTilesAndObjects`' body) —
SPEC-007's sentence and `mapdef.Compile`'s construction; the test is cited
under N. **R**, a collision leaves the table serving (the follow-up
`start_session` in `TestLoadMapDoubleLoadCollisionRejectedCleanNotPoisoned`)
— VTT-161's rule, not re-cited. **S**, a placement for an absent actor refuses
the whole batch — SPEC-007's sentence and `engine.Apply`'s `TokenPlaced` arm.
**T**, art declaring a later format refuses a map — `artlib`'s format rule and
`mapdef.Resolve`'s; E holds the gateway's half. **U**, two racing loads put one
scene in the world — the fold's `ErrSceneExists`; the wire test is K's
evidence. **V**, an installed-but-invalid map stays out of the set — held by
construction (E). Three clauses stay prose: "overwritten" (I), "and nobody
else" (J), "and one scene" (F, U).

Eleven accepted, three OPEN leaning two ways, against eight refused rows and
three refused clauses. Of the eleven, G and H have their mechanism in
`internal/mapdef` and `internal/artlib` and are accepted because the promise
is to whoever reads a `load_map` answer and a disclosure is a security
finding; M is the one whose every red-making edit is another package's, and
Q3 asks.

**D6. Rows are dispensed after sign-off, cited by citation lines only, and an
OPEN row has no citer.** Forced by SPEC-008 and the two previous plans' D6.
Every row above names an existing test, so none is expected to stay OPEN; if
the reading refuses a test as evidence and keeps the rule, that row is
`**OPEN — no test yet**` with no citation line and the report names it (ticket
item 3). A citation line is `// VTT-NNN` directly above `func Test`, below any
doc block, several ids on one line where a test holds several
(`TestAnUnknownMapIsRefusedByName` carries B's and G's,
`TestConcurrentLookupsOfANewlyInstalledMapCompileItOnce` D's and F's).
Evidence cells are `internal/gateway/<file>#<Test>`, written by hand after
the dispenser; the chain gate refuses one that is wrong. SPEC-014's
Requirements line is copied from the register last. Ids start at VTT-163.

**D7. `server.go`: the nine blocks by symbol, and what each becomes.** Forced
by ticket items 2 and 4. The four methods are exported and keep one doc
sentence each; the five fields are unexported and keep a pointer, with a
warning where an edit could break something silently:

| Block (by symbol) | Becomes |
|---|---|
| the `maps` field (16) | `// See WithMaps and mapByID; take mapsMu for every access once s serves (SPEC-014).` |
| the `mapsDir` field (5) | `// See WithMapsDir (SPEC-014).` |
| the `artDir` field (13) | `// See WithArtDir (SPEC-014).` |
| the `cellPx` field (7) | `// See WithCellPx (SPEC-012).` |
| the `mapsMu` field (3) | `// Guard maps with this and nothing else: mapsDir, artDir and cellPx are written only before s serves (SPEC-014).` |
| `WithMaps`' doc (16) | `// WithMaps sets the maps s starts with, keyed by each map's own id and validated by the caller (SPEC-014).` |
| `WithMapsDir`'s doc (25) | `// WithMapsDir names the campaign's maps directory, where a load_map looks for a map s does not hold (SPEC-014).` |
| `WithArtDir`'s doc (31) | `// WithArtDir names the campaign's art directory, which every map load resolves art against (SPEC-014).` |
| `WithCellPx`'s doc (14) | `// WithCellPx sets how many pixels one grid square of this campaign occupies (SPEC-012).` |

Each text is a starting point, wrapped to the band (D8); the reading may
shorten it and may not lengthen it past three lines. The `With*` lock warning
already stands on `WithRuleset` (`Do not call a With* method on a serving
Server: they write without a lock.`) and is not repeated. Keep the blank lines
between the five fields: with one comment line above each and a blank line
between, gofmt realigns nothing; if a field's comment goes and two fields
touch, gofmt aligns their types, which changes whitespace and no token.

**D8. The sweep's rules, repeating the authorization plan's where they
apply.** Forced by rule 10, SPEC-010, and the identity plan's D1 to D8.

- *Three kinds and nothing else*: a warning is imperative, a verb first, the
  consequence in the present tense, at most three lines; a pointer is
  `SPEC-014`, `SPEC-007`, `SPEC-009`, `SPEC-011`, `SPEC-012`, `SPEC-013`,
  `VTT-NNN`, a test name, a symbol name, or `docs/verification-debt.md`, and
  may close a warning or a doc sentence in parentheses; a doc sentence is the
  first sentence `go doc` prints for an exported symbol. Every fact a deleted
  block held that SPEC-014 does not state is either added to SPEC-014 (D9) or
  named in the report as dropped, with the reason.
- *Exported symbols keep one sentence*: the four `With*` methods (D7).
  Nothing in `map.go` is exported, so it keeps no doc sentence; a block above
  `handleLoadMap` or `mapByID` is a pointer sentence opening with the
  function's own name, the shape `handleCommand`'s block took, or a warning.
- *A warning above a function opens with a word that names no function*: the
  list is under Measurements; `Load`, `LoadInstalled`, `Compile`, `Lookup`,
  `Append` and `Open` are on it.
- *A doc sentence is one sentence, on one line where it fits*; the wrap band is
  `SHORT, LONG = 55, 85` in `tools/check-comment-wrap.py`, which
  `check:new-prose` applies to added lines.
- *Test files*: citation lines (D6) and `metadata_test.go`'s one re-aimed
  block (D13). No other test-file edit; the doc blocks above the map tests,
  many over the bound with banned terms, are the test-prose sweep's.
- *What is left alone*: every block in `server.go` outside D7's nine,
  including `New`'s and the `Server` type's own doc; every comment in
  `internal/mapdef`, `internal/artlib` and `cmd/vtt`, stale or not (D18).

**D9. Facts only a comment holds go into SPEC-014.** Forced by ticket item 1
and the identity plan's D7. The reading of each cut block asks whether
SPEC-014's draft states the fact; if not, and the fact is about the code now,
it is checked against the symbol and written in, or named in the report as
dropped. The candidates are listed under "Candidates the reading starts from".

**D10. `map.go`: the five blocks and what each becomes.** Forced by ticket
item 4. Modelled on `adventure.go`, whose sweep is the precedent for this
file's twin:

| Block (by symbol) | Becomes |
|---|---|
| `errNoMapsAvailable`'s doc (6) | goes, as `errNoAdventuresAvailable`'s did |
| `handleLoadMap`'s doc (74) | `// Answer every failure as an ok=false result, never a close: authorization already ran in handleCommand (SPEC-014).` |
| the stamping block (4) | `// Stamp the four fields Compile leaves zero: store.AppendBatch requires an EventId (SPEC-014).` |
| the `ErrSceneExists` block (24) | `// Translate this sentinel and no other: a DM sent to copy a map over an unrelated fault cannot act on it (TestANonCollisionFailureKeepsItsOwnMessage).` and `// Say "scene id", never "map": a loaded adventure can hold the id too (TestASecondLoadTellsTheDMTheMapIsLoadedAndHowToReload).` |
| `mapByID`'s doc (55) | `// mapByID is the lookup SPEC-014 states: the set, then the campaign's maps directory on a miss.` |

Three warnings move into `mapByID`'s body, beside the code each guards, where
`check:doc-owner` does not read them: above the `LoadInstalled` call, `// Hold
no lock across this: it reads the disk, and GET /api/maps reads the set.`;
above the not-installed arm, `// Translate not-installed alone and forward
the rest: LoadInstalled names maps/<id>.json and never the path it opened
(SPEC-014).`; above the second look, `// Look again under the write lock:
racing lookups must end on one map
(TestConcurrentLookupsOfANewlyInstalledMapCompileItOnce).` And one beside the
ok=true return, as `adventure.go` has it: `// Return the warnings to the
issuer only; nothing broadcasts them (SPEC-014).` Projection, not a target:
`map.go` near 14 comment lines of about 92 non-blank, near 15 percent. The
reading governs; nothing is cut for a figure.

**D11. The comment-stripped comparison is a token stream.** Forced by the
identity plan's D11. The program is that plan's Task 0 listing
(`docs/superpowers/plans/2026-09-24-sweep-identity.md`: `go/scanner`, mode 0,
`fmt.Printf("%s %q\n", tok, lit)`), built once in the scratchpad
(`$S/codetokens/codetokens`, a `go.mod` with `module codetokens` beside it)
and run from the repository root:

    for f in internal/gateway/map.go internal/gateway/server.go \
             internal/gateway/map_test.go internal/gateway/map_internal_test.go \
             internal/gateway/metadata_test.go; do
      git show "80cb2b7:$f" > "$S/before.go"
      "$S/codetokens/codetokens" "$S/before.go" > "$S/before.tok" || echo "SCAN FAILED $f"
      "$S/codetokens/codetokens" "$f" > "$S/after.tok" || echo "SCAN FAILED $f"
      printf '%s %s/%s tokens ' "$f" "$(wc -l < "$S/before.tok")" "$(wc -l < "$S/after.tok")"
      cmp -s "$S/before.tok" "$S/after.tok" && echo same || echo DIFFERS
    done

Done reads five `same` lines at 596, 4596, 7029, 620 and 9214. A run proves
it ran by the counts; a zero is a failed run. A `DIFFERS` on a test file means
a string literal changed, and Phase 4b names it.

**D12. The ledger, last, and two rows.** Forced by SPEC-010 (the band) and by
`--write-ledger` lowering a row on any drop. After Phase 4b has settled,
`python3 tools/check-comments.py --write-ledger`, then `git diff
tools/comment-ceilings.txt` must show exactly two rows changed, each lowered:
`internal/gateway/map.go` and `internal/gateway/server.go`. `map_test.go` and
`map_internal_test.go` gain citation lines only, which move no share;
`metadata_test.go`'s re-aim keeps its block's length, and even with Q6's
clause removed its share stays in the same tenth (392 / 1,572 rounds up to
25.0). A third changed row means a file changed that this plan does not name:
stop, name it, and ask before committing. The commit message lists the rows
old and new.

**D13. `metadata_test.go`, one block.** Forced by ticket item 4 of "What it
touches". In the doc of `TestArtWithNoArtDirectoryConfiguredIs404`, "(WithArtDir's
own doc comment)" becomes "(SPEC-014)". The block is five lines, under the
bound, so a changed line is allowed; the citation line `// VTT-126` below it
is not part of it. Q6 asks whether the same edit drops "the harness and",
which is false.

**D14. The specification edits, sentence by sentence.** Forced by ticket item
6 and check 5.

- SPEC-012, the `/api/maps` paragraph's last sentence, "What a map is, how
  `load_map` compiles one and how `mapByID` grows the set at request time are
  `map.go`'s and have no record yet.", becomes "How `load_map` looks a map up
  and grows the set is SPEC-014's; what a map is, `internal/mapdef`'s."
- SPEC-012, the boot paragraph's last sentence ("reads the filesystem at
  request time in two places and no other"), per Q5: "This package reaches the
  filesystem at request time through four calls and no other: `handleArtFile`
  opens the art directory; `mapByID` runs `mapdef.LoadInstalled`, which reads
  the maps directory and the art directory; `handleLoadMap` runs
  `mapdef.Compile` and `handleLoadAdventure` runs `adventure.Compile`, each of
  which reads an art directory through `internal/artlib` (SPEC-014)." with the
  search it rests on named in the sentence or beside it (`grep -n '\bos\.'`
  over the package's non-test files, which prints `handleArtFile`'s
  `os.OpenRoot` alone, and `grep -n 'mapdef\.\|adventure\.Compile\|artlib\.'`
  for the calls that reach the disk through another package).
- SPEC-013, "What is dispatched": "`load_map` (`handleLoadMap`, `map.go`) and
  `remove_actor` (`handleRemoveActor`) have no record of their own, and
  SPEC-007 states what their batches carry." becomes "`load_map`
  (`handleLoadMap`) is SPEC-014's, `remove_actor` (`handleRemoveActor`) has no
  record of its own, and SPEC-007 states what both batches carry."
- SPEC-013, "What this record does not decide": "the batch handlers are
  SPEC-012's, `handleLoadMap`'s and `handleRemoveActor`'s" becomes "the batch
  handlers are SPEC-012's, SPEC-014's and `handleRemoveActor`'s".

No other sentence in either file changes; `git diff` on each shows only these.

**D15. The deliberate breaks, one per check this work relies on.** Forced by
the dev-cycle's rule that a check is proven by a red, and ticket items 2 to
6. In a scratch clone (`git clone --no-hardlinks` into the scratchpad, the
final `git diff HEAD` applied, the untracked files copied in, committed there,
that clone's `main` pointing at that commit): first each gate exits 0 with
its completion line; then each break is one edit, the finding recorded
verbatim, the inverse edit made by hand (never `git checkout --`), `git diff
--stat` printing nothing before the next.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | `// VTT-999` added to the citation line above `TestAnUnknownMapIsRefusedByName` | `check:requirements-chain`: `map_test.go cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 | one new row's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `check:requirements-chain`: `does not carry the id, so the link walks one way only` |
| B3 | SPEC-014's Requirements line gains `VTT-999` | `check:requirements-chain`: `cites VTT-999 ... (specification citation)` |
| B4 | one pointer line, `// SPEC-014`, inside `handleLoadMap`'s body between two code lines where no block absorbs it, after the ledger is written | `check:comments`: `internal/gateway/map.go: comment share X is above its ceiling Y and this change added a comment line to it (SPEC-010)`; about ninety-two non-blank lines, so one line moves the share by about a point |
| B5 | `errors.Is(err, fs.ErrNotExist)` made `fs.ErrExist` in `mapByID` | D11's loop: `map.go ... DIFFERS`, and `TestAnUnknownMapIsRefusedByName` red (run at verification) |
| B6 | "ADR-008: cmd owns the filesystem" re-inserted in `WithCellPx`'s doc | `grep -c 'ADR-008' internal/gateway/server.go` prints 1 |
| B7 | the pointer above `mapByID` rewritten to open with `LoadInstalled` | `check:doc-owner`: the doc comment above `mapByID` begins by describing `LoadInstalled`, a different function |

**D16. Phase 4b is the check for VTT-051 and for SPEC-014, sentence by
sentence.** The reviewer gets `git diff HEAD`, SPEC-007, SPEC-009 to SPEC-014,
the rows VTT-050 to VTT-059 and the new rows, `map.go`, `server.go`, the three
test files, and the code SPEC-014 points into: `internal/mapdef/installed.go`,
`compile.go`, `resolve.go`, `load.go`, `internal/artlib/artlib.go`,
`internal/engine/apply.go`'s `SceneCreated` and `TokenPlaced` arms,
`internal/campaign/campaign.go`'s `AppendBatch`, `cmd/vtt/serve_compose.go`
and `cmd/vtt/maps.go`. For every surviving block it names the kind (D8) and,
for a warning, the code it guards and whether the consequence is true of that
code; for a pointer, that the target resolves; for a doc sentence, that the
symbol is exported and the sentence true. For every deleted block: did it
hold a fact now in no record? If yes, SPEC-014 (D9) or the report. For every
SPEC-014 sentence: the symbol it names and whether the code under it does
what the sentence says — `catches.md` item 8 is the one to watch, since every
sentence starts life in a comment and this plan found three compressions and
two false sentences already. For each dispensed row: the edit that would red
the test named. For the SPEC-012 and SPEC-013 edits: that nothing else in
either file changed, and that the new SPEC-012 sentence's search, re-run,
prints what it says. For the rule-9 answer: that SPEC-014 names no other
project.

**D17. Phase 4a is skipped, with its reason.** Independent QA derives tests
from a specification to find behaviour the implementer got wrong. This change
has no behaviour: D11 shows every Go file's code identical to `80cb2b7`, and
what is added is prose, register rows, citation lines and comment deletions.
What can be wrong is a sentence, and a reading holds that (D16). The report
records the skip under its own heading.

**D18. Things the ticket's scope leaves stale are left, and named.**
`TestLoadMapNoMapsConfiguredCleanError`'s doc ("a server for a campaign whose
maps/ is absent, or which has no maps installed yet"), false for every server
`vtt serve` builds; `map_test.go`'s file header and fixture docs, histories
over the bound; `TestAMapWithArtFromALaterFormatIsRefusedOnDemandExactlyAsAtBoot`'s
and `TestBrokenArtCannotPushAResultPastTheReadLimit`'s docs, whose "artlib.clip
... was called from ONE place" is history; `cmd/vtt/maps.go`'s file comment
naming `gateway.WithMaps/WithPackFiles`, where `WithPackFiles` no longer
exists; the six ADR-008 credits under `cmd/vtt`. All named in the report as
the test-prose and `cmd/vtt` sweeps'. The report does not revise the reports
of the periods the cut blocks narrate.

**D19. One commit for the change; the report in its own.** Forced by the band
(a file's deletions and its row land together), by the chain gate (a row's
evidence and its citation line land together, and neither hook runs
`check:requirements-chain` or `check:comments`), and by the pointers
(`SPEC-014` must have a target in the same tree). The commit carries the
ticket (untracked today), this plan, SPEC-014, the SPEC-012 and SPEC-013
edits, the register, `map.go`, `server.go`, `map_test.go`,
`map_internal_test.go`, `metadata_test.go` and the ledger. The pattern is
`8128a61` then `80cb2b7`.

**D20. Gate steps before the commit, in order, on a tree the review has
settled.** `gofmt -l internal/gateway/` prints only `scenario_test.go`; `go vet
./internal/gateway/`; `go test -count=1 ./internal/gateway/...` green; D11's
loop, five `same`; `task check:comments` (expected, before Task 10, to refuse
`map.go` and `server.go` for the band and nothing else); `task
check:doc-owner`; `task check:requirements-chain` (`<162 + N> rows, 193 test
files, 8 specifications; every citation resolves and every row's evidence
holds`); `task check:new-prose` (every test and symbol name the sweep writes
must resolve; `SPEC-014` is not read by that gate, so the pointer is held by
the reading); `python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q` (no key names either file; `OK` expected
without a re-point, and if either reds, stop: a key this plan says does not
exist does); `task lint`. Then, after Task 10, `task check` whole, once, on
the final tree, launched in its own session (`start_new_session=True`), and
only after Phase 4b has settled, so no edit lands mid-run; `uptime` first,
since the e2e waits fail under load. Then the pre-commit hook's own set, with
`git add` and `git commit` in separate calls, and what landed checked with
`git show --stat HEAD`.

## Candidates the reading starts from

Read by the verifier at `80cb2b7`. A starting point, not a verdict.

**Facts for SPEC-014 that only a comment states today.** `mapByID` holds no
lock across `LoadInstalled` because it is disk I/O plus the whole of `mapdef`'s
validation, and holding the write lock would stall every reader of the set,
`GET /api/maps` included. The second look under the write lock is what makes
every racing caller end on one `*mapdef.Map`, and `map_internal_test.go` is
the only place that is observable. Boot-time preloading is the same lookup run
early by `loadMapsDir`, so an operator learns of a broken map before anyone
connects. The on-demand probe runs the function the boot walk runs, with the
same art root, so a map that boots is not refused on reload nor the reverse.
`WithMapsDir` and `WithArtDir` take paths, not an `fs.FS`, because
`LoadInstalled` and `artlib` work in paths and `artlib` confines every file it
opens to the directory through `os.OpenRoot`; an id that is not one plain
filename is refused by `LoadInstalled` before it joins anything. `LoadInstalled`
names every file as `maps/<id>.json`, so nothing the probe forwards carries the
server's layout; the gateway translates only not-installed and forwards the
rest, because a broken map is the DM's to act on and `mapdef` says it best. A
map loaded on a miss has its warnings computed twice in one request, once by
`LoadInstalled`'s dry run and discarded, once by `handleLoadMap`'s `Compile`
and kept. A map never creates actors, so a placement for an actor the world
lacks is the fold's refusal at `AppendBatch` (SPEC-007); the DM adds the actor
first. The one translated refusal says "scene id" because that is all the
sentinel knows; the remedy clause holds because `Compile` takes the scene id
from the map's id. The scene-id refusal is the one a DM reaches by ordinary
use: art not installed degrades and the load commits, so installing the
picture and loading again collides with the first load. Warnings go on the
issuer's result and nothing broadcasts them. `WithMaps` does no I/O and no
validation and expects each map already through `LoadInstalled`; an empty
art directory means no art resolves and nothing is refused. `mapsMu` guards
`maps` alone; `mapsDir`, `artDir` and `cellPx` are written once, before the
server serves.

**Text that goes on sight.** Every `spec §`, `Task N`, `task-N`, date,
`Patrik's ruling`, `round 1`, `used to`, `until 2026-`, `CORRECTED`, `C1
remediation`, `whole-branch-review`, `maps-as-geometry Task 7`, `design spec
§`, `a first draft of this message`; the "THE ADVENTURE SIDE NO LONGER
DISCARDS ANYTHING" and "This used to say 'the other two production call
sites'" paragraphs of `handleLoadMap`'s doc; the "THE PACK-NOT-LOADED ANSWER
IS GONE" paragraph of `mapByID`'s; `WithMaps`' "IT TOOK A SECOND ARGUMENT";
`WithArtDir`'s "This method landed in Task 3 rather than Task 4" and the
boot-order narrative; `WithCellPx`'s "It was a PACK field until" and the
ADR-008 credit; the `maps` field's "NO LONGER BOOT TIME ONLY" and its claim
that `loadMapsDir` refuses an id collision (its own doc says the collision
cannot arise and nothing checks it); the byte figure the warnings paragraph
warns against restoring; every sentence that says what the code does rather
than what to keep.

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository; the scratchpad, `$S`.

Build D11's program; run the loop. Run `python3 tools/check-comments.py
--report | grep -E 'internal/gateway/(map|server|map_test|map_internal_test|metadata_test)\.go'`,
`task check:requirements-chain`, `python3 tools/check-comments.py main`,
`python3 tools/check-doc-owner.py .`, `gofmt -l internal/gateway/`, `grep -c
'ADR-008' internal/gateway/server.go`, both mutation self-tests, and `grep -c
'VTT-'` over the two map test files; keep the outputs. Confirm
`requirement-id` is on the path.

**Done when:** the outputs match the measurements above (five `same` at 596,
4596, 7029, 620, 9214; `162 rows ... 7 specifications`; `239 files ... clean`;
only `scenario_test.go` from gofmt; `1`; both self-tests `OK`; citations 0
and 0); `requirement-id` prints its one-line usage.

### Task 1 — SPEC-014

**Files:** `docs/specifications/014-loading-a-map.md`, new.

Per D1, D2, D3 and D9; the `specification` skill's steps 1 to 5 and 7
(`catches.md`, two passes at most). Step 6's Requirements line is written
`None yet; the sort of 2026-09-28-loading-a-map-has-a-record-design.md fills
it` until Task 8 and then copied from the register.

**Done when:** the file has the five headings in order (`grep -c '^## '`
prints 5), no `Why`, no `Rejected`, no number that is a measurement, no line
number, no date outside a `docs/` path, no path outside the project, no other
project's name; every sentence under "How it works" names a symbol in
`map.go`, `server.go`, `internal/mapdef`, `internal/artlib`,
`internal/campaign`, `internal/engine` or `cmd/vtt`, and a table of sentence
to symbol is kept for Task 9; the three compressions under Measurements are
stated as the code has them; `task check:requirements-chain` prints `8
specifications`.

### Task 2 — SPEC-012 and SPEC-013

**Files:** `docs/specifications/012-the-read-surface.md`,
`docs/specifications/013-authorization.md`.

D14's sentences, the SPEC-012 request-time sentence per Q5.

**Done when:** `grep -c 'no record yet' docs/specifications/012-the-read-surface.md`
prints one less than before and `grep -n "handleLoadMap" docs/specifications/013-authorization.md`
shows every mention beside `SPEC-014`; `git diff --stat` on the two files
shows only D14's lines; the new SPEC-012 sentence's search, run, prints what
the sentence says.

### Task 3 — `server.go`, the nine blocks

**Files:** `internal/gateway/server.go`.

D7's table, under D8.

**Done when:** D11 prints `same` for `server.go` at 4596; each of the nine
blocks is at most three lines and holds only D7's text or the reading's
shortening of it; `grep -c 'ADR-008' internal/gateway/server.go` prints 0;
`go doc ./internal/gateway Server.WithMaps` (and the other three) prints one
sentence; `gofmt -l internal/gateway/server.go` prints nothing; `--report`
shows `server.go` at `blocks>6 11`; `check:doc-owner` still ends `79 files ...`.

### Task 4 — `map.go`

**Files:** `internal/gateway/map.go`.

D10's table and the four body warnings, under D8; every fact of the cut blocks
checked against SPEC-014's draft, each gap into SPEC-014 (D9) or the report's
dropped list.

**Done when:** D11 prints `same` for `map.go` at 596; `--report` shows `map.go`
with `banned 0` and `blocks>6 0`; `gofmt -l` prints nothing for it;
`check:doc-owner` still ends `79 files ...`; no block opens with a word on the
first-word list.

### Task 5 — `metadata_test.go`'s re-aim

**Files:** `internal/gateway/metadata_test.go`.

D13, and Q6's clause if yes.

**Done when:** D11 prints `same` for it at 9214; `grep -n "WithArtDir's own
doc" internal/gateway/metadata_test.go` prints nothing; the block is at most
five lines; `python3 tools/check-comments.py main` refuses nothing but the
band on `map.go` and `server.go`.

### Task 6 — Local gates, first pass

**Files:** none changed.

D20's list up to and not including `task check` whole.

**Done when:** each step exits 0 with its completion line, except
`check:comments`, which at this point refuses `map.go` and `server.go` for
the band and nothing else; both mutation self-tests print `OK`;
`check:new-prose` reports no citation to a name the tree never declared.

### Task 7 — Sign-off of the sort

**Files:** none.

D5's table is presented with the reading's confirmations and overrides, each
override with its reason, and the questions below answered. Nothing is
dispensed before the answer.

**Done when:** each lettered row has one of: accept (with wording and
evidence), refuse (with reason), OPEN (with reason); Q1 to Q12 have answers.

### Task 8 — Rows and citation lines

**Files:** `docs/requirements.md` (by the dispenser, then evidence cells by
hand), `internal/gateway/map_test.go`, `map_internal_test.go` (citation lines
only, D6), `docs/specifications/014-loading-a-map.md` (the Requirements line,
copied).

**Done when:** `task check:requirements-chain` prints `<162 + N> rows, 193 test
files, 8 specifications; every citation resolves and every row's evidence
holds` with N the accepted count; an OPEN row, if any, has no citer; D11
prints `same` for both test files; `--report` shows `cites` equal to the
number of citation lines per file and both files' shares unchanged.

### Task 9 — Phase 4b, the reading review

**Files:** whatever its findings touch among the above.

Per D16. Findings are fixed and the affected task's "done" is re-run. The
review settles before Task 10 starts; if the reviewer dies on a model's limit,
say so and re-dispatch with the same brief on `fable`.

**Done when:** the review record names every surviving block's kind, every
deleted block's outcome, every SPEC-014 sentence's symbol and verdict, every
row's red-making edit, the SPEC-012 and SPEC-013 diffs, and reports no open
finding; D11 prints five `same`.

### Task 10 — The ledger

**Files:** `tools/comment-ceilings.txt`, by `--write-ledger` only.

Per D12.

**Done when:** `git diff tools/comment-ceilings.txt` shows exactly the two
rows, each lowered; `task check:comments` ends `clean`; `--report` prints
`banned 0` and `blocks>6 0` for `map.go` and `blocks>6 11` for `server.go`.

### Task 11 — The breaks and the whole gate

**Files:** none in the repository.

D15 in a scratch clone; then `task check` whole, once, per D20.

**Done when:** the clone's clean run exits 0 with each completion line, and
each of B1 to B7 produces the one red D15 names; `task check` exits 0 with
every step, `check:comments`, `check:requirements-chain`, `check:doc-owner`
and `check:mutation` among them, printing its own verdict.

### Task 12 — Commit, then the report

**Files:** the commit's, per D19; then
`docs/reports/2026-09-28-loading-a-map-has-a-record.md`.

The commit message lists the ledger rows old and new, D11's counts, the rows
dispensed, and B4's finding line verbatim. After it: the report per the
`implementation-report` skill, in its own commit — per file comment lines and
share before and after, blocks kept by kind, the facts SPEC-014 took and the
facts dropped with reasons, the sort (every row with its id and test, every
refusal in one line), the breaks' finding lines, the gaps below as found or
closed, D18's stale list, the sign-off answers, and the rule-9 answer.

**Done when:** `git show --stat HEAD~1` lists the ticket, this plan, SPEC-014,
SPEC-012, SPEC-013, the register, `map.go`, `server.go`, `map_test.go`,
`map_internal_test.go`, `metadata_test.go` and the ledger, and nothing else;
`git diff --stat 80cb2b7 -- docs/reports/` lists only the new report; `git
diff --quiet 80cb2b7 -- internal/mapdef internal/artlib internal/engine
internal/campaign internal/adventure cmd/ contract/ client/src` exits 0.

## Commits

| Commit | Carries | Gate steps it runs |
|---|---|---|
| C1 | the ticket, this plan, SPEC-014, the SPEC-012 and SPEC-013 edits, the register, `map.go`, `server.go`, `map_test.go`, `map_internal_test.go`, `metadata_test.go`, the ledger | D20's list by hand, `task check` whole (Task 11), then the pre-commit hook (lint, vet, tier-1, arch, vocabulary, doc-owner, secrets, typecheck, review gate) |
| C2 | the implementation report (and a debt entry if Q9 says so) | pre-commit hook |

Push after C2: pre-push runs tiers 2 and 3 and the contract gates, about three
minutes; let it finish. No Go code changes, so no mutation key moves and
`check:drift` has no client change to compare.

## Gaps that travel with this plan

1. **SPEC-012 says the package reads the filesystem at request time in two
   places and no other, and it reads it through four calls.** `handleLoadMap`'s
   `mapdef.Compile` and `handleLoadAdventure`'s `adventure.Compile` both reach
   `artlib.Open` on every load. SPEC-014 will say so, and the ticket's item 6
   does not name the sentence. D14 and Q5.
2. **`handleLoadMap`'s `Compile` refusal arm has no test.** Probed: answering
   it with ok=true leaves the gateway package green. A map already in the set
   whose art is overwritten with a later-format sidecar reaches it, and its
   refusal names the art and not the map. Q9.
3. **"No maps available" and the bare "unknown map" are reachable only for a
   server built without `WithMapsDir`,** which `composeServer` never builds;
   `TestLoadMapNoMapsConfiguredCleanError`'s doc says otherwise (D18). Q2.
4. **A map joins the set at the lookup, not at the load.** A load refused
   afterwards (an absent actor, a collision, art changed between the dry run
   and the live compile) leaves it listed. True of the code, observed by no
   test; SPEC-014 states it and row D is worded on what is observed. Q10.
5. **Row F's red is probabilistic.** Removing the second look reds the
   internal test in 37 of 40 runs; the race is the rule's own nature, and the
   internal test is the only observer. Q8.
6. **`ParticipantId`'s stamp is unobserved.** Deleting it leaves the one test
   that checks stamping green; row O is refused, and SPEC-014 states the stamp.
7. **An append failure's text is forwarded verbatim and no test drives one.**
   Whether a store or driver error can carry the campaign's path is not
   established; row G is narrowed to the lookup and the compile.
8. **"Overwritten" art and "nobody else" for warnings are unobserved** in the
   gateway's tests; both are SPEC-014 prose.
9. **`metadata_test.go`'s touched block holds a false clause** (the harness
   builds a server with no `WithArtDir`). Q6.
10. **Item 5 is an invariant, not a done that fails today.** D11 holds it at
    every task.
11. **Row count after the sort.** The ticket's "more than 162" holds for any
    N ≥ 1; the report states N.

## Questions for sign-off

1. **Does SPEC-014 name MapTool?** Recommend no (D3): filename-is-id and the
   refused second load are stated as facts about this system; the refused
   client-side model and rename-on-collision are the rule-9 answer in this
   plan and the report.
2. **Row A, "no maps available" without a maps directory: row, OPEN or
   prose?** Recommend refuse as a row and state it in SPEC-014 as what a
   server built without `WithMapsDir` answers, with `composeServer` named as
   always wiring it: a row about a configuration no production caller builds
   holds nothing a DM relies on, and worded on "an empty campaign" it would be
   false. `TestLoadMapNoMapsConfiguredCleanError` stays uncited.
3. **Row M, the result staying readable: accept under SPEC-014 though its
   mechanism is `mapdef`'s?** Recommend accept, worded as VTT-136 is, with the
   red-making edit named in `resolve.go` and SPEC-014 pointing at
   `warningTally` and `artlib.Clip`: the consequence is at the table (the
   issuer's socket closes after the scene is broadcast), `mapdef` has no record
   to hold it, and VTT-136 set the precedent for `load_adventure`. Refusing is
   the brief's lean and would leave the test uncited until `mapdef` has a
   record.
4. **Row N, the first sequence: accept?** Recommend yes, for parity with
   VTT-137; SPEC-007 states the convention and the row pins it for this
   command.
5. **Correct SPEC-012's request-time sentence in this change (D14)?** Recommend
   yes: the file is already in the ticket's moves, SPEC-014 would otherwise
   contradict it on landing, and the correction is one sentence with its
   search. The `handleLoadAdventure` half is SPEC-012's own subject and is
   corrected with it rather than left false.
6. **In `metadata_test.go`'s re-aimed block, also drop "the harness and"?**
   Recommend yes: the block is touched anyway, stays under the bound, and the
   clause is false (the harness boots through `composeServer`); leaving it
   means Phase 4b reads a false sentence in a line this change owns.
7. **Rows G and H, path non-disclosure, split by channel and accepted under
   SPEC-014?** Recommend yes: two tests, two mechanisms, two red-making
   edits, and a disclosure is a security finding, ranked at full depth.
8. **Row F with a red in 37 of 40 runs: accept?** Recommend accept, with the
   rate in the report: the rule is about a race and cannot be observed
   deterministically without a seam this ticket may not add; the edit reds it
   in most runs, and `-race` is the second observer.
9. **The untested `Compile` refusal arm: report only, or an Open-debt entry in
   C2?** Recommend an Open-debt entry, with the recipe (overwrite a boot-loaded
   map's art with a `format_version` 99 sidecar, load it) and the test that
   would close it: `CLAUDE.md` names `docs/verification-debt.md` as the one
   file for known coverage gaps, and the authorization change took the same
   step for VTT-149's bound. The ticket's "What it touches" does not list the
   file, so this needs the yes.
10. **SPEC-014 states that a map joins the set at the lookup and stays after a
    refused load?** Recommend yes, as prose: it is what the code does, the
    ticket's "joins the listing once loaded" compresses it, and a DM who sees
    a refused map on `/api/maps` needs the sentence. No row, since no test
    observes it.
11. **One commit for the change, the report separately (D19)?** Recommend yes.
12. **If a swept block lands with every line a true warning, pointer or doc
    sentence and still over the bound?** Recommend the reading governs: stop,
    report the block, and let the ticket's writer decide, rather than cut a
    true warning for the bound.
