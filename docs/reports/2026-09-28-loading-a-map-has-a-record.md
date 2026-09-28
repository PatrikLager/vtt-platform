# Loading a map has a record: the change

**Ticket:** `docs/superpowers/specs/2026-09-28-loading-a-map-has-a-record-design.md`,
amended at sign-off on six points, five its verification found and one from
sign-off question 9: the problem paragraph now says the no-maps-directory arm
is a configuration `composeServer` never builds, that a map joins the set when
its lookup validates it, and that `mapdef.Compile` warns about the art it
resolves rather than only art it could not use; it names SPEC-012's
request-time sentence and both SPEC-013 sentences among the records it moves;
two candidate rules were reworded ("joins the set its lookup validated it
into", "refused on demand, with the boot walk's reason"); and
`docs/verification-debt.md` joined "What it touches".
**Plan:** `docs/superpowers/plans/2026-09-28-loading-a-map-has-a-record.md`,
verified by `verify-ticket` (Passes with gaps). Its twelve sign-off questions
were answered on 2026-09-28, all twelve as the plan proposed: SPEC-014 names no
other project; row A is not a row and SPEC-014 states the arm; rows M and N are
rows; SPEC-012's request-time sentence is corrected in this change;
`metadata_test.go`'s "the harness and" goes; rows G and H are split by
channel; row F is accepted with its probabilistic red; the untested `Compile`
refusal arm is a debt entry in the report's commit; SPEC-014 states that a map
joins the set at the lookup; one commit and the report apart; and the reading
governs over the bound.
**Last commit of the change (no code line changed):** `bb7b535`, on `80cb2b7`,
`main` at the time. Every code reference below is to that tree.

## The period, in commits

    git log --oneline 80cb2b7..bb7b535

    bb7b535 Loading a map has a record: SPEC-014, VTT-163 to VTT-175

`git diff --stat 80cb2b7..bb7b535`: 12 files changed, 1438 insertions(+), 309 deletions(-).

The gate: `task check`, whole, over the tree of `bb7b535` before it was
committed: exit 0, no step failed, and the check steps' own verdict lines
read `check:comments` `clean`, `check:requirements-chain` 175 rows,
`check:doc-owner` 79 files, `check:new-prose` 54 added lines clean,
`check:coverage` 20 packages at or above their floors, `check:no-pack` clean,
`check:mutation` fourteen packages with zero unadjudicated survivors,
`check:ts-mutation` 2882 mutants, 2783 killed, 29 timed out and counted as
killed by that gate's own rule, 70 survivors all adjudicated, zero
unadjudicated, verified from its stored report because no client input had
changed. It was launched behind a load guard of a five-minute average under
4.0, looser than the previous change's fifteen-minute 3.0, because a Docker
virtual machine another session had started held the load near 4 for an hour.
Before it: `gofmt`, `go vet`, `task lint`, the gateway package's tests,
`check:comments`, `check:doc-owner`, `check:requirements-chain`,
`check:new-prose`, both mutation self-tests, the token instrument, and the
breaks; then the pre-commit hook's nine checks.

Phase 4a was skipped: the change moves prose and changes no code line, and
what can be wrong in it is a sentence, which the reading review holds.

**Rule 9.** MapTool builds every map on the client: `AppActions.LOAD_MAP`, a
new map, a quick map and the importers each make a whole `Zone`, and
`MapTool.addZone` sends it with `putZone`; `ServerMessageHandler`'s
`PUT_ZONE_MSG` arm stores and relays it with no role or content check, and
every client receives the whole campaign, GM layer included. Grid size is per
`Zone` with a per-installation default; assets are content-addressed by
`MD5Key` in a flat per-user cache with a sidecar, and a missing asset is
drawn as a placeholder (`Asset.createBrokenImageAsset`); nothing watches a
directory, and a folder is re-listed when it is asked for. Not decided here:
a cell's size is per campaign in this platform and SPEC-012's; this change
only re-points `cellPx`'s comment. Borrowed, or already this platform's
shape: a map loaded whole (here stricter, one atomic append); a missing piece
costing its picture and not the map; a flat store with a sidecar; re-reading
on demand rather than watching. Refused: the client-side build and the server
that trusts it, the whole-campaign distribution rule 9 names, uploads whose
hash is not checked, and rename-on-collision with fresh ids, where this
platform refuses a second load of a scene id and the remedy is a copy under a
new id. Two ideas for later: a piece overwritten under its name keeps its
name here, where MapTool's key changes with the bytes (the art route answers
`no-cache`, SPEC-012); and a map loaded but hidden from players
(`Zone.isVisible`) would belong in the projection, never the client. SPEC-014
names no other project.

## Done looks like, answered

1. `[x]` `docs/specifications/014-loading-a-map.md` exists with SPEC-007's
   five headings (`grep -c '^## '` prints 5): what `load_map` asks for and
   answers; the map set, the lookup on a miss and the lock discipline; what
   reaches a client from a refusal; art read at each load from one directory
   and what an unusable piece costs; the batch and its stamping; the one
   translated refusal; the warnings and who receives them; the configuration
   calls; and what the record does not decide, pointing at SPEC-007,
   SPEC-012, SPEC-013, `internal/mapdef` and `internal/artlib`. The reading
   review (Phase 4b, `pr-review-toolkit:code-reviewer` on `opus`) read every
   sentence against the symbol it names, by command; the sentences it refuted
   or found unbounded were rewritten before the commit (Deviations).
2. `[x]` `grep -c 'ADR-008' internal/gateway/server.go` prints 0; at `80cb2b7`
   it printed 1.
3. `[x]` Thirteen rows, VTT-163 to VTT-175, each with the tests in its
   evidence cell citing it; `task check:requirements-chain` prints `175 rows,
   193 test files, 8 specifications; every citation resolves and every row's
   evidence holds`. No row of the thirteen is OPEN.
4. `[x]` `python3 tools/check-comments.py --report` prints `banned 0` and
   `blocks>6 0` for `map.go` and `blocks>6 11` for `server.go` (18 at
   `80cb2b7`). That every surviving block in `map.go` and in `server.go`'s
   nine is a warning, a pointer or an exported symbol's doc sentence is the
   reading review's verdict (VTT-051), a reading and not a check. The ledger
   rows moved in the same commit, `--write-ledger` run last.
5. `[x]` The go/scanner token stream, comments dropped, of `map.go` and
   `server.go` is identical to `80cb2b7`'s (596 and 4596 tokens; the
   instrument is the program plan D11 names, compared with `cmp` against `git
   show 80cb2b7:<file>`); so are `map_test.go`'s (7029),
   `map_internal_test.go`'s (620) and `metadata_test.go`'s (9214).
6. `[x]` SPEC-012's `/api/maps` paragraph says how `load_map` looks a map up
   is SPEC-014's; SPEC-013's dispatch paragraph and its "does not decide"
   paragraph name SPEC-014 for `handleLoadMap`; SPEC-012's request-time
   sentence names the four calls that read a file, the identity database and
   the log aside, with the search it rests on.
7. `[x]` `task check` whole, exit 0, above.

## What the rules became

| Rule, as the amended ticket words it | Became |
|---|---|
| A `load_map` on a server with no maps and no maps directory is refused, saying none are available | refused as a row (sign-off question 2): no server `vtt serve` builds reaches the arm; SPEC-014 states it, and `TestLoadMapNoMapsConfiguredCleanError` stays uncited |
| A `load_map` naming a map neither in the set nor installed is refused, naming the map | VTT-163 |
| A map written into the campaign's maps directory after boot is loadable without a restart, and joins the set its lookup validated it into | VTT-164, VTT-165 |
| An installed map the boot walk would refuse is refused on demand, with the boot walk's reason | VTT-166 |
| Two loads of the same newly installed map racing each other leave one entry and one scene | VTT-167 for the entry; "one scene" is the fold's (`ErrSceneExists`) and its wire test is cited under VTT-172 |
| No refusal and no warning a `load_map` produces names where the campaign lives on disk | VTT-168 (refusals of the lookup and the compile), VTT-169 (warnings) |
| Art installed or overwritten after boot is used by the next `load_map` with no restart | VTT-170, "overwritten" left as SPEC-014 prose: no `load_map` test overwrites a piece (the art route's overwrite test, `TestArtInstalledAfterTheServerStartedIsServed`, is SPEC-012's) |
| Art declaring a format this server does not understand refuses the map on demand exactly as at boot | VTT-166's second test; the format rule itself is `artlib`'s and `mapdef`'s |
| A `load_map`'s warnings reach the issuer's result | VTT-171 |
| A second `load_map` of a map whose scene id is already in play is refused with a sentence saying so and how to load it under a new id, and any other append failure keeps its own message | VTT-172, VTT-173 |
| A map whose art is broken cannot push its result past the connection's read limit | VTT-174 (sign-off question 3; its red-making edit is in `internal/mapdef/resolve.go`) |
| The shipped campaign's maps resolve their own art | refused: content, not a rule |

One row came from the plan's sort rather than the ticket's list: VTT-175, a
`load_map` result carries the first sequence of the batch it appended
(sign-off question 4, for parity with VTT-137).

## The sweep

Comment lines over non-blank lines, counted by `tools/check-comments.py`'s
own `measure` with citation lines set aside, at `80cb2b7` and at `bb7b535`:

| File | Before | After | Banned | Blocks over the bound |
|---|---|---|---|---|
| `internal/gateway/map.go` | 163 / 241 | 18 / 96 | 23 to 0 | 3 to 0 |
| `internal/gateway/server.go` | 473 / 1069 | 358 / 954 | 29 to 8 | 18 to 11 |

`map_test.go` and `map_internal_test.go` gained seventeen citation lines and
changed nothing else; `metadata_test.go`'s one block kept its five lines.
Ledger rows, old to new: `map.go` 67.7 to 18.8, `server.go` 44.3 to 37.6: the
two rows plan D12 named, and no other.

What the kept blocks are, by symbol. `map.go`: a warning above
`handleLoadMap`; inside it, the stamping warning, the two warnings on the
`ErrSceneExists` arm and the one beside the ok=true return; a pointer above
`mapByID`, and inside it the three warnings on the unlocked `LoadInstalled`
call, the not-installed translation and the second look. `server.go`: a
warning and pointer on the `maps` field, pointers on `mapsDir`, `artDir` and
`cellPx`, a warning on `mapsMu`, and one doc sentence on each of `WithMaps`,
`WithMapsDir`, `WithArtDir` and `WithCellPx`. `errNoMapsAvailable`'s doc went.

Facts the cut blocks held that SPEC-014 took: the three steps of `mapByID`
and why the disk read holds no lock; that racing lookups end on one map; that
the no-directory arm is what a server built without `WithMapsDir` answers;
that `LoadInstalled` names a file as `maps/<id>.json`, so the probe forwards no
layout, and that the gateway translates not-installed alone; that the probe
runs the boot walk's function with the same art directory; that a map loaded
on a miss is compiled twice and only the second compile's warnings are kept;
that art is read at each load and nothing is cached; that an empty art
directory refuses nothing; the stamping and why; that a map adds no actor and
an absent one is the fold's refusal; the scene-id translation, why it says
"scene id" and why it is the refusal a DM reaches by ordinary use; that
warnings go to the issuer alone; what each configuration call takes and that
`mapsMu` guards `maps` alone.

Facts dropped, each with the reason: the dated and task-numbered histories of
every block (history, rule 10); the path-rather-than-`fs.FS` rationale for
`WithMapsDir` and `WithArtDir` (a design argument, and `artlib`'s confinement
is its own record's); `composeServer` running `artlib.Validate` and
reporting rather than refusing (`cmd/vtt`'s); the shipped-content example of
`cellar` against `cellar-rats` (content); `cellPx`'s "a zero would read as no
grid" (`New` sets the default, SPEC-012); `internal/adventure`'s dry-run
discard (its own comment holds it); the claim that `loadMapsDir` refuses an id
collision and the claim that an empty production campaign answers "no maps
available" (both false); and the ADR-008 credit (false).

`server.go`'s eleven remaining blocks over the bound, by the symbol each sits
on or in: the `writeTimeout` field, `announcePresence`, `announceDeparture`
(two, its doc and its body), `revoked`, `announcePromotion`,
`handleRemoveActor`, `handleJoinDoor` (one, in its body), `handlePromotion`
(two, its doc and its body), `credentialGone`.

## The breaks

In a scratch clone of the changed tree, committed there as that clone's
`main`; each gate first ran clean (`175 rows ... every row's evidence holds`,
`239 files, 0 added comment lines, 239 ledger rows; clean`, `79 files, every
doc comment sits on its own function`, `map.go` and `server.go` `same`), then
one edit per break, reverted by its inverse, `git diff --stat` empty before
the next.

| Break | Red |
|---|---|
| B1 VTT-999 added to the citation line above `TestAnUnknownMapIsRefusedByName` | `check:requirements-chain: internal/gateway/map_test.go cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 VTT-175's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `row VTT-175: internal/gateway/codec_test.go does not carry the id, so the link walks one way only` |
| B3 VTT-999 on SPEC-014's Requirements line | `docs/specifications/014-loading-a-map.md cites VTT-999 and no row in docs/requirements.md defines it (specification citation)` |
| B4 a `// SPEC-014` line inside `handleLoadMap`, after the ledger was written | `check:comments: internal/gateway/map.go: comment share 19.59 is above its ceiling 18.8 and this change added a comment line to it (SPEC-010)` |
| B5 `fs.ErrNotExist` made `fs.ErrExist` in `mapByID` | the token instrument prints `DIFFERS` for `map.go`, and `TestAnUnknownMapIsRefusedByName` is red |
| B6 the ADR-008 credit back in `WithCellPx`'s doc | `grep -c 'ADR-008' internal/gateway/server.go` prints 1 |
| B7 the pointer above `mapByID` opened with "LoadInstalled" | ``check:doc-owner: ... the doc comment above `mapByID` begins by describing `LoadInstalled`, which is a different function`` |

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as first written. | Amended at sign-off on six points (header). | The verification found them: SPEC-012's false request-time sentence and SPEC-013's second sentence unnamed, "joins the listing once loaded", "does not compile ... stays out of the set", "warnings for art it could not use", and a no-maps arm no production server reaches; the sixth, the debt file, came from sign-off question 9. |
| Plan D14: SPEC-012's new sentence says the package "reaches the filesystem at request time through four calls and no other". | It says that, apart from the campaign's log and the identity database, the package reads a file at request time through four calls, and names the search. | The review: `s.ids.Verify` and `s.ids.Lookup` read a SQLite file, and every batch handler's `AppendBatch` writes one, so "no other" was false as the plan worded it. |
| Plan D14: SPEC-013's dispatch sentence says SPEC-007 states what "both batches" carry. | It names the `load_map` and `remove_actor` batches. | A first edit here said "the batches", which read as all four; SPEC-007 says nothing of `use_ability` or `load_adventure`. |
| Plan D1's art sentence: what refuses a map is "a sidecar declaring a format this server does not understand". | "a sidecar declaring a `format_version` later than this server understands". | The review: `artlib` refuses only a version above `FormatVersion`; one that is lower, absent or not a number degrades. |
| SPEC-014's first draft: a map installed after boot is refused on demand "exactly when" the boot walk would refuse it. | SPEC-014's How it works now says that what `LoadInstalled` refuses at boot it refuses on demand, and its Consequences bullet and VTT-166 keep plan D1's "with the same reason"; the boot walk's own `os.Stat` and its skipping of a directory are not run on demand. | The review probed both: a directory named `x.json` is skipped at boot and refused on demand; a dangling link fails the boot walk's `os.Stat` and is answered "nothing installed" on demand. |
| SPEC-014's first draft: "`Compile` opens it through `artlib.Open` on every call". | "each time it builds a scene". | `BuildSceneCreated` refuses a map over `MaxWireTiles` before it opens the directory. |
| SPEC-014's first draft: art installed after a map's load does not reach its scene. | A square whose art did not resolve at the load stays plain; a piece overwritten under a name the scene carries is served by the art route (SPEC-012). | The review: `TileRef.Art` carries the name, and the client fetches it by name. |
| SPEC-014's first draft: its principle said a place enters play "never only from what the server read when it started", it repeated SPEC-013's cell and SPEC-007's batch, and it named no search for "changes only in `mapByID`", "nothing broadcasts a warning" or "nothing is cached". | The principle says a map or a piece of art installed while the server runs is used without a restart; the repeats point at SPEC-013 and SPEC-007; the three searches are named. | The review: the principle contradicted the record's own sentence that a map in the set is not re-read, and catches item 10 asks for the search behind each absolute. |
| Plan D10's warning on the `ErrSceneExists` arm: say "scene id", never "map". | "Say the scene id is in play, never that the map is loaded". | The review: the message it guards says "map" three times, and what the test forbids is the claim that the map is loaded. |
| Plan D13: `metadata_test.go`'s pointer re-aimed at SPEC-014. | Re-aimed at SPEC-012. | The review: the route's 404 for an unset art directory is SPEC-012's sentence and VTT-126's. |
| Plan D7's text for the `maps` field opened with "See". | It opens with the warning, "Take mapsMu for every access once s serves". | A warning leads with its verb (rule 10). |

## What could not be established

- `handleLoadMap`'s `Compile` refusal arm is driven by no test: answering it
  ok=true leaves the gateway package green. `docs/verification-debt.md`
  carries it under Open debt, with the recipe and the test that would close
  it. VTT-168's "or compile" is observed only through `LoadInstalled`'s dry
  run.
- `ParticipantId`'s stamp is observed only by an uncited test: deleting it
  leaves `TestLoadMapProducesBatchCarryingTilesAndObjects` green and reds
  `TestThreeRoleExitScenarioOverLiveWebSockets` (`dm-load-map: dm's connection
  saw ParticipantId=""`). Row O was refused on plan gap 6's reading that no
  test observes it; SPEC-014 states the stamp, and a row citing the scenario
  test is the next ticket's to dispense.
- No test drives an append failure other than the fold's (VTT-172 and VTT-173
  drive those); whether a store or driver error can carry the campaign's path
  is not established, so VTT-168 is narrowed to the lookup and the compile.
- A map joins the set at the lookup and stays listed after a load refused
  later; SPEC-014 states it and no test observes it.
- "Overwritten" art and "nobody else" for warnings are unobserved by any
  `load_map` test; both are SPEC-014 prose.
- Left stale by this change's scope, for the test-prose and `cmd/vtt` sweeps:
  `TestLoadMapNoMapsConfiguredCleanError`'s doc, which says an empty campaign
  answers "no maps available", false for every server `vtt serve` builds;
  `map_test.go`'s file header and fixture docs;
  `TestAMapWithArtFromALaterFormatIsRefusedOnDemandExactlyAsAtBoot`'s doc,
  which narrates a review and a rename;
  `TestBrokenArtCannotPushAResultPastTheReadLimit`'s, whose "was called from
  ONE place" is history; `cmd/vtt/maps.go`'s file comment naming
  `gateway.WithMaps/WithPackFiles`, where `WithPackFiles` no longer exists;
  the five ADR-008 filesystem credits under `cmd/vtt` (`embed.go`,
  `harness_boot.go`, `maps.go`, `serve_compose.go`, `serve_e2e_test.go`);
  `main.go`'s citation of ADR-008 for the shell's design is true.

## What was deliberately left out, and where it went

- `server.go`'s eleven blocks over the bound, by symbol above: the
  announcement helpers, `revoked`, `credentialGone`, the remaining command
  handlers and `writeTimeout` are the `server.go` ticket's, the last of the
  gateway's.
- The map format, filename-is-id, how a reference resolves, degrade against
  refuse, the warning collapse and the art names and formats: `internal/mapdef`
  and `internal/artlib`, whose comments hold them (`format.go` 85.2 percent,
  `resolve.go` 76.8, `artlib.go` 69.4 at `80cb2b7`) and which have no record.
- The boot walk and `artlib.Validate` at boot: `cmd/vtt`'s.
- No file under `cmd/`, `contract/` or `client/src` changed, nor any under
  `internal/` outside `map.go`, `server.go` and the three test files, whose
  token streams are identical to `80cb2b7`'s (item 5).

## The sort

Thirteen candidates accepted as VTT-163 to VTT-175. Refused: A, the
no-maps-directory answer, a configuration no production server builds; O, the
four stamped fields, of which the cited test observes three (the fourth only
in the uncited scenario test); P, the shipped campaign's art, content; Q, the
batch's shape, SPEC-007's; R, a collision leaving the table serving,
VTT-161's; S, a placement for an absent actor refusing the batch, SPEC-007's
and the fold's; T, the format rule, `artlib`'s and `mapdef`'s, whose gateway
half is VTT-166; U, one scene when two loads race, the fold's; V, a broken map
staying out of the set, true by construction. Three clauses stayed prose:
"overwritten" (VTT-170), "and nobody else" (VTT-171), "and one scene"
(VTT-167).
