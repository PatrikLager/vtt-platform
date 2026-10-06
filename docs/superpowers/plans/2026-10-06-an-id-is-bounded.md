# An id is bounded — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-06-an-id-is-bounded-design.md`
**Verified:** 2026-10-06 by `verify-ticket` (dev-cycle 0.4.0). The verifier is
an agent that did not write the ticket. It verified against `b9de835` on
`feat/ids-are-bounded`, which equals `main`, with the ticket untracked.
Verdict: **Passes with gaps.** The gaps are listed at the end and travel with
this plan. This plan does not edit the ticket.

**Goal, in the ticket's words:** both folds refuse a `SceneCreated`, an
`ActorAdded`, a `TokenPlaced` or an `AdventureLoaded` whose id is longer than
its bound, and accept one of exactly the bound. A map or an adventure carrying
a longer id is refused when it loads, naming the file and the field, before
anything is appended. An `add_actor` or a `place_token` carrying one is
answered ok=false and appends nothing. Every golden, fixture and shipped
content file still loads and folds, and `task check` whole is green.

**MapTool (CLAUDE.md rule 9): every id is generated, never typed, and has
exactly one length.** Read in `~/dev/RPTool/maptool` at `f4b7fef6c`:

- `Token`, `Zone` (a map) and `Campaign` each hold `private GUID id = new
  GUID()`. No person or macro chooses one.
- `GUID.generateGUID` takes `UUID.randomUUID()`, strips the dashes, and
  redraws while the result is all digits. `toString` renders the 16 bytes as
  32 upper-case hex characters.
- `GUID_LENGTH = 16`, and `validateGUID` refuses any other length ("GUID
  length is invalid"). `GUID(String)` parses hex and then validates, so an id
  read off the wire (`Token.fromDto`'s `GUID.valueOf(dto.getId())`,
  `Campaign`'s, `Zone`'s) is held to the same length where it is parsed.
- People type names. Macro functions that take "a name or an id" tell them
  apart by length alone: `GUID.isNotGUID` is `arg.length() != GUID_LENGTH *
  2` (`MapFunctions`, `FindTokenFunctions`).

**Borrowed:** an id has one bound, and the bound is checked where the id is
read, not trusted from the writer. **Not borrowed:** generation. Here an id is
chosen by its author: a map's id is its filename (`idIsAFilename`'s doc
already says the filename buys "the guarantee MapTool buys with a UUID"), an
adventure's ids are written by hand, and the agent picks `add_actor`'s and
`place_token`'s ids so it can address them in the next call. MapTool's fixed
16 bytes does not fit a readable id; its rule that an id has a length the
reader enforces does. **Checked and rejected:** "name or id by length", since
nothing here resolves a reference by name.

## Verification, check by check

1. **Every path resolves (by command).** `[ -e ]` finds all 18 paths the
   ticket names: 12 files and 6 directories. `grep` finds each named symbol
   where the ticket puts it: `func LoadInstalled` in `installed.go`, `func
   Compile` in `internal/mapdef/compile.go`, `maxIDBytes = 128` in
   `internal/adventure/load.go`, `func ToEvent` in `convert.go`, `const
   maxWSFrameBytes = 32768` in `server.go`, `actor_added requires an actor
   with an id` in `apply.go`, the VTT-162 and VTT-276 rows, and the name
   report's "raised to the owner with this report". That the owner asked for
   this ticket on 2026-10-06 is a reading; no command reaches it.
2. **"Done" is an observation (by command).** None of the named tests exists
   yet. Sketches of each fail today and pass on the probe tree (P4):
   - Item 1: seven engine refusals red with `err = <nil>`.
   - Item 2: seven TS refusals red with `Received function did not throw`.
   - Item 3: `mapdef.Load` and `mapdef.LoadInstalled` accept a 129-byte and a
     250-byte map id; `adventure.Load` accepts a 129-byte adventure, actor
     and placement token id.
   - Item 4: `add_actor` with a 129-byte id, `ok=true error="" head 5->6`;
     `place_token` likewise.
   - Item 5 is the gate. It is reachable only through work the ticket does not
     list (Gap 4).

   The "accepts one of exactly the bound" halves of items 1 and 2 cannot fail
   today, because nothing refuses (Gap 5). K5 and K13 hold them.
3. **Each rule is breakable (a reading, then probes).** Each rule has a single
   edit that reds a named test. K1 to K30 in the K table were run; K31 and
   K32 need C1's tree.
   - The first rule becomes row A, and B if Q2 is (b).
   - The second becomes row C, and D if Q2 is (b).
   - The third is VTT-162 and VTT-161, which gain the two wire tests. The
     ticket cites them rather than restating them, and the sort agrees (D11).
4. **The scope matches the claim (by command, then a reading).** It does not.
   The work is larger than "What it touches" says:
   - **A fifth way in.** A map's placements become `TokenPlaced` events
     (`mapdef.Compile`), and `loadAs` checks nothing about a placement's
     `token_id`. The shipped `campaigns/example/maps/cellar.json` places
     `tok-fighter`. With only the fold bounded, a map with a long token id
     boots and is refused at `load_map`, which breaks the ticket's second
     rule (Gap 1, D6).
   - **`loadScenes` sits at the gocyclo ceiling.** It measures 30 on the base
     (`min-complexity: 30` in `.golangci.yml`; the name arc's check brought it
     there). The placement token-id check makes it 31, and `golangci-lint`
     fails. Extracting the placement loop brings it to 22 (P6, D6).
   - **A toolgen test pins the absence the tools question would change.**
     `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication` requires
     `actorId` to carry no description (P10, D9).
   - **Two comment blocks become false.** `internal/adventure/load.go`'s
     35-line block on `maxIDBytes` says it has "NO engine twin" and that a
     map's scene id is bounded by the filesystem's ~255. `internal/artlib/artlib.go`'s
     74-line const-block doc lists "STILL UNBOUNDED … mapdef.LoadInstalled's
     use of the file's own declared id … internal/adventure's collision
     refusals for an ACTOR id and a TOKEN id", and this change bounds all
     three paths at 128 bytes (P15, D13, Q6).
   - **Fixtures the ticket does not name.** `TestLoadInvalidFixturesCatalogueIsComplete`
     requires one `testdata/invalid` directory per refusal, and
     `TestLoadAcceptsValuesExactlyOnEveryLimit`'s invariant needs an at-cap
     value for each new check. Gremlins kills each new boundary mutant only
     with an at-cap value (P8).
   - **The ledger.** `internal/adventure/load.go` falls from 36.9 to 32.6 and
     `internal/mapdef/load.go` from 51.8 to 50.9. Both are more than 1.0 under
     their ceilings, which `check:comments` reports as a finding until
     `--write-ledger` runs (P12).

   The ticket's other claims hold:
   - `engine.Apply`'s callers are unchanged: five calls in `internal/campaign`
     and one in `internal/harness`.
   - Production code builds the four creating events in `mapdef.Compile` and
     `BuildSceneCreated`, `adventure.Compile`, `ToEvent`, and `project.go`'s
     introductions. `eventgen` and the harness soak build them as test
     apparatus.
   - The introductions copy folded state, so they need nothing.
   - The MCP server is a wire client.
5. **No recorded decision is contradicted silently (a reading).** SPEC-018
   records ids as unbounded in the fold, and the ticket names SPEC-018 as
   moved. SPEC-014 lists what `LoadInstalled` refuses and says a map is
   installed by writing `maps/<id>.json`. The ticket names SPEC-014 too.
   SPEC-013's validators paragraph ("leaves an `add_actor` with no actor or no
   `actor_id` to the fold") stays true, because nothing is added to
   `validateAddActor`. SPEC-007's `load_map` batch stays true. ADR-007: no
   contract change.

   The adventure loader's `maxIDBytes` comment records a decision: the map
   path was "left alone rather than tightened to match". This change
   overturns that decision. The ticket asks for exactly that, but does not
   say it overturns it.
6. **The records the work moves are named (by command, then a reading).** The
   section lists SPEC-018 and SPEC-014, and both resolve. The reading:
   - SPEC-007, SPEC-011, SPEC-012, SPEC-013, SPEC-016 and SPEC-017 hold no
     sentence the change makes false. Each was grepped for ids, these events
     and the frame bound.
   - Records outside `docs/specifications/` that become false and are not
     named:
     - the debt entry "Nothing ties a byte bound's copies to the engine's
       constant", which counts "ten bounds";
     - the two comment blocks above;
     - `at-every-boundary/guide.md`'s table;
     - `fold.ts`'s one-line `tokenPlaced` order comment.

## Measurements this plan stands on

Every measurement was taken by command, never in the working tree. Three
scratch clones of `b9de835` were used (`git clone --no-hardlinks`, `bun
install --frozen-lockfile`):
- `base`, left as committed, for baselines and red sketches;
- `instr`, `engine.Apply` and `fold.ts` wrapped to log every id;
- `probe`, the sketch in D1 to D9 with Q2(b) and Q4(a).

The sketch tests were named `zz_*`. All three clones and their scratch files
were deleted afterwards.

| # | What | Result |
|---|---|---|
| P1 | Base. An engine test applies a 10 KiB id on each of the four events, and each event with an empty id. | All eight fold except the empty actor id: `engine: actor_added requires an actor with an id`. The TS sketch agrees: 129-byte and empty scene, token and adventure ids all fold. |
| P2 | Base. Every scene, actor, token and adventure id, map `id` and `maps/` filename in the 925 tracked JSON files (924 parse; `client/tsconfig.json` is JSONC), by a walk over `git ls-files '*.json'` | Content outside the loader fixtures is at most 18 bytes: `act-spectator-deny` and `tok-spectator-deny` (`scenarios/denials.json`), and `scn-goblin-warrens` (`contract-spike/fixtures/token_moved.json`). An adventure id is at most 17 (`fixture-adventure`). A map's id and filename are at most 16 (`scn-goblin-fight`). Two exceptions: `internal/adventure/testdata/at-every-boundary/scenes/pin.json`'s scene id is **128** bytes, and loads; `scene-id-too-long`'s is 200, and is refused. Empty ids appear only in `placement-actor-id-empty` and `placement-token-id-empty`, both refused at load. No map fixture lacks an `id` (31 map files). |
| P3 | `instr`. `engine.Apply` and `fold.ts`'s `apply` log every id that is empty or over 18 bytes, with its verdict. Then `go test -count=1 ./internal/... ./contract/... ./tools/... ./cmd/...` (21 packages ok; `cmd/vtt` 198 s, its spawned `vtt` binaries included) and every `*.test.ts` file run alone (38 files, all exit 0). | **The longest creating id any test, eventgen draw, harness scenario or golden folds is 20 bytes.** It is `TokenPlaced` `qanp-tok-qanp-hero-a`, built as `"qanp-tok-" + hero` in `internal/gateway/qa_note_projection_test.go`. No `SceneCreated`, `ActorAdded` or `AdventureLoaded` id over 18 is folded. **No empty scene, token or adventure id is folded anywhere**, in Go or TS. Empty actor ids: 2 in Go and 4 in TS, all refused. Over 18 and refused: `prop-*-absent-N` and `tok-qa-bound-absent` (19 to 20). Over 18 and accepted, outside the four: participant ids of 32 hex characters, session ids of 37 (`sess-` plus 32 hex), and condition sources of 23 and 27. The only long id built from a repeated string is `strings.Repeat("z", 260)`, a `load_map` id in `map_test.go` and `installed_test.go`. The filesystem refuses it before anything folds. eventgen draws `prop-scn-N`, `prop-actor-N` and `prop-tok-N`, and no `AdventureLoaded`. No Go fuzz corpus is committed. |
| P4 | Base, then probe. Each Done item's sketch | Engine: 7 subtests red on base (`err = <nil>, want "engine: scene id must be 1-128 bytes, got 129"` and siblings); at-cap green. Probe: all green. TS: 7 red on base, 8 of 8 green on the probe. `mapdef`: base loads all five cases and `LoadInstalled` of 128, 129 and 250 bytes; a 300-byte id fails `file name too long`. Probe: `mapdef: maps/<id 129>.json: field "id": must be at most 128 bytes, got 129`, the same at 250, `field "placements[0].token_id": must be at most 128 bytes, got 129`, and `… must not be empty`. Adventure: base loads 129 on all three; the probe refuses each by file and field. Wire: base `add_actor 129: ok=true error="" head 5->6`, `place_token 129: ok=true`, `place_token empty: ok=true`. Probe: `ok=false error="engine: actor id must be at most 128 bytes, got 129" head 5->5`, then `engine: token id must be 1-128 bytes, got 129`, then `… got 0`. |
| P5 | The whole probe tree | `go test -count=1 ./internal/... ./contract/... ./tools/... ./cmd/...`: 21 packages ok (`cmd/vtt` 165.6 s, scenario goldens included). `bun test client/test contract contract-spike`: 943 pass, 0 fail (935 on base plus the 8 sketches). `bunx tsc --noEmit -p client/tsconfig.json` clean. `semgrep scan --config .semgrep/ --error` exit 0. `golangci-lint`: see P6. |
| P6 | `gocyclo -over 20`, base then probe | Base: `loadScenes` 30, `loadAs` 21. Probe with the token-id check: `loadScenes` **31**, and `golangci-lint` reports `cyclomatic complexity 31 of func loadScenes is high (> 30)`. With the placement loop moved into an unexported `loadPlacements`, `loadScenes` is 22 and `loadAs` 24, and lint is clean on the three packages. |
| P7 | Keys mapped base → probe by `difflib` over each file, every line compared | `apply.go`: `341:58`→`351:58` (+10), `452:15`→`465:15`, `455:30`→`468:30`, `599:41`→`612:41` (+13). `internal/adventure/load.go`: `123:58`→`89:58` (−34, because the 35-line block was cut to one line). `fold.ts`: eight keys +3 and eight +4 (D12). |
| P8 | Gremlins with the gate's argv (`--workers 1 --timeout-coefficient 30 --output-statuses lctkvs`, `--exclude-files '^conformance/'` for adventure), after `go clean -testcache` | `internal/engine`: Killed 116, Lived 4, Not covered 0, mutator coverage 100%. The four lived are exactly the four keys at their new places. `internal/mapdef`: Killed 143, Lived 0, Not covered 11, all in `compile.go` and `resolve.go` and none on an added line. `internal/adventure`: Killed 103, Lived 1 (the adjudicated `ARITHMETIC_BASE` at `load.go:89:58`), Not covered 0. Every new boundary mutant is killed, but only because the sketches held an at-cap value for each check. |
| P9 | Stryker on the edited `fold.ts` ranges (`--mutate client/src/fold.ts:77-81,…:147-151,…:195-200,…:455-463`) | 34 mutants, 32 killed, 2 survive. The survivors are the existing `"attackRolled"` and `"abilityUsed"` `StringLiteral` entries at `461:10` and `462:10`. No new adjudication is needed, and none is deleted. The label `StringLiteral`s die only to exact-message tests. |
| P10 | `fieldDocs` for `add_actor`'s `actorId` and a new `vtt.v1.PlaceToken` override for `tokenId`, then `task generate:contract` | Each of the two `tools.json` files gains 2 lines, and nothing else changes. `TestToolsMatchGolden` is red until `contract/testdata/expected_tools.json` gains the two lines by hand. `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication` is red (`has a description, want none (nothing to steer it away from)`) until its `actorId` assertion changes. Then `tools/toolgen`, `contract`, `internal/mcp` and `internal/engine` are green. |
| P11 | A link test reading `contract/gen/tools/tools.json` for `fmt.Sprintf("At most %d bytes", maxIDBytes)` | Green; with `maxIDBytes = 130`, red for both tools. |
| P12 | `check-comments.py`, base → probe | Base: `263 files, 0 added comment lines, 263 ledger rows; clean`. Probe: `2 finding(s)`, `internal/adventure/load.go` 36.9 → 32.6 against 37.0 and `internal/mapdef/load.go` 51.8 → 50.9 against 51.9, each "fallen more than 1.0 … record it". `apply.go` 41.9 → 41.1, `fold.ts` 42.7 → 42.5. No banned term on an added line. |
| P13 | Map filenames this volume accepts (APFS), by `open()` in scratch | 250 `z` + `.json` ok; 251 refused. `é`×250 + `.json` (505 bytes) ok; `😀`×126 + `.json` refused. `€`×250 + `.json` ok: **a 750-byte map id**. APFS counts 255 UTF-16 units, not bytes. On ext4, NAME_MAX is 255 bytes, so an id is at most 250 (`artlib`'s `maxArtIDLen = 255 - len(sidecarExt)` states the same arithmetic for art). |
| P14 | Readers of `at-every-boundary` | Only `TestLoadAcceptsValuesExactlyOnEveryLimit` loads it; nothing compiles or folds it. Its `guide.md` table lists the scene id at 128 and omits the three 256-byte names the name arc added. |
| P15 | Prose that states an id is unbounded, by `git grep` over `internal`, `cmd`, `client/src`, the specifications and the debt file | SPEC-018's sentences (D10). `internal/adventure/load.go`'s `maxIDBytes` block. `internal/artlib/artlib.go`'s const-block doc ("STILL UNBOUNDED …"). Its copy in `docs/superpowers/specs/2026-09-02-art-is-a-flat-library-design.md` is a ticket, and tickets are not edited. Also found: `actor-name-too-long`, `adventure-name-too-long` and `scene-name-too-long`'s `guide.md` files are copies of `scene-id-too-long`'s and say "`scenes/cellar.json` declares an id of 200 `s` characters". Their scene ids are 6 bytes. |
| P16 | Both mutation self-tests on the probe | Before re-pointing: the Go self-test fails on the keys. The TS self-test flags 12 of the 16 `fold.ts` keys, and **cannot see** `382:48`, `411:19`, `600:32` and `705:7`. After re-pointing by statement (D12): `OK`, `OK`. |

**What breaks what.** Each break is one edit on the probe tree. The file was
restored from its saved text and checked by hash. All were run. The red
column names the sketch test; the real test D8 names takes its place.

| # | Edit | Red |
|---|---|---|
| K1 | Go scene-id check disabled | engine scene refusal, and its empty case |
| K2 | Go actor-id check disabled | engine actor refusal, `add_actor` wire test |
| K3 | Go token-id check disabled | engine token refusal and its empty case, `place_token` wire test |
| K4 | Go adventure-id check disabled | engine adventure refusal and its empty case |
| K5 | Go scene-id `>` → `>=` | engine at-cap test |
| K6 | Go `len(sc.SceneId) == 0 \|\|` removed | engine empty scene case |
| K7 | Go `len(tp.TokenId) == 0 \|\|` removed | engine empty token case |
| K8 | Go `len(…AdventureId) == 0 \|\|` removed | engine empty adventure case |
| K9 | `maxIDBytes` 129 | all four engine refusals and three empty cases, the link test, the `add_actor` wire test |
| K10 | Go actor-id refusal reworded | engine actor refusal, `add_actor` wire test |
| K11 | TS scene `checkLen` removed | TS scene refusal and its empty case |
| K12 | TS actor `checkLen` removed | TS actor refusal |
| K13 | TS token bound 127 | TS at-cap case, TS token refusal |
| K14 | TS token `checkLen` removed | TS token refusal and its empty case |
| K15 | TS adventure `checkLen` removed | TS adventure refusal and its empty case |
| K16 | TS scene minimum 0 | TS empty scene case |
| K17 | TS token minimum 0 | TS empty token case |
| K18 | TS adventure minimum 0 | TS empty adventure case |
| K19 | TS scene id measured with `.length` (characters) | TS scene refusal (its id is `é`×64 + `i`, 129 bytes, 65 characters) |
| K20 | `mapdef` id check disabled | map id refusal |
| K21 | `mapdef` id check `>=` | map id at-cap |
| K22 | `mapdef` placement token-id length check disabled | map token refusal |
| K23 | `mapdef` placement token-id check `>=` | map token at-cap |
| K24 | `mapdef` empty placement token allowed | map empty-token refusal |
| K25 | adventure-loader id check disabled | adventure id refusal |
| K26 | adventure-loader actor-id check disabled | actor id refusal |
| K27 | adventure-loader placement token-id check disabled | token id refusal |
| K28 | the three adventure-loader checks `>=` | the at-cap load |
| K29 | `add_actor`'s description says 200, regenerated | the link test |
| K30 | TS scene-id check moved after the duplicate check | **nothing**: an over-long or empty id is never a stored key, so the order against the duplicate check cannot be observed (Gap 11) |
| K31 | `place_token`'s description says 200, regenerated | the link test (not run on the probe; K29 is its twin) |
| K32 | `at-every-boundary`'s adventure id shortened to 127 | `TestLoadAcceptsValuesExactlyOnEveryLimit`'s new assertion (not run: the fixture does not exist yet) |

K6 to K8, K16 to K18 and K24 exist under Q2(b) only. K22 and K23 hold row C,
and K24 holds row D.

**Records and gates at the base.**
- `check-requirements-chain.py .`: `276 rows, 221 test files, 13
  specifications`.
- `check-comments.py main`: `263 files, 0 added comment lines, 263 ledger
  rows; clean`.
- `check-doc-owner.py .`: `80 files`.
- Both mutation self-tests: `OK`.
- `requirement-id` is at
  `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`.
- Free space on `/System/Volumes/Data`: 33,619,264 KiB at the start and
  30,735,148 KiB (29.3 GiB) before the clones were deleted.
- The Go cache grew from 7.3 to 8.3 GiB. Load averages ran from 2.0 to 7.3;
  another session was running.

## Constraints that bind every task

- **Rule 2:** no gate is weakened, and no `//nolint:gocyclo` is added to
  `loadScenes` (D6). The ledger goes down only through `--write-ledger`. A
  key is re-pointed by statement.
- **Rule 3:** no proto changes; `check:breaking` reports nothing.
- **Rule 4:** both folds refuse the same events at the same counts, in the same
  order within each arm (D4, D5). A TS fold stricter than Go's freezes a
  client (`client/src/wire.ts`), so neither side lands alone.
- **Rule 5:** no new vocabulary. "Scene", "actor", "token", "adventure", "map"
  and "id" are platform words, and semgrep passed on the probe.
- **Rule 8:** cite names, never lines. The adjudication files keep their
  coordinates. **Rule 9:** answered above. **Rule 10 and SPEC-010:** new
  comments are limited to:
  - test citation lines;
  - one-line pointers on the two loader constants;
  - the edited one-line order comment in `fold.ts`;
  - the cut blocks of D13, each six lines or fewer.

  No banned term appears on an added line.
- **`check:comments`' block rule:** only a code line ends a block, and a block
  of more than six lines that the change touches is refused. That rule shapes
  every block edit in D13.
- **SPEC-008:** ids come from `requirement-id`, after sign-off, in letter
  order.
- **Order and hygiene.**
  - `check:drift` passes only on a committed tree, so the order is review,
    commit, gate.
  - The review package is `git diff HEAD`, and nothing is stashed or checked
    out while a reviewer reads.
  - `git add` and `git commit` run in separate calls, with `git show --stat
    HEAD` after each.
  - A new test goes after a closing brace.
  - The pre-push hook takes about three minutes; let it finish.
- The ticket and every report under `docs/reports/` are not edited. Neither is
  any ticket under `docs/superpowers/specs/`, the art ticket's stale list
  included.

## Decisions this plan makes

**D1. The bound: 128 bytes of UTF-8, inclusive, one constant for all four ids
(Q1).** `maxIDBytes = 128` goes last in `engine.Apply`'s const block. The
block's header ("Inclusive byte bounds on an event's free text (SPEC-018).
Never lower one …") stays true, since SPEC-018 calls every string field a
text. What forced 128:
- **A scene id already has a bound.** `internal/adventure/load.go`'s
  `maxIDBytes` holds an adventure's scene ids at 128, and a map's id is a
  scene id too. A fold bound below 128 would lower a loader bound, and refuse
  `at-every-boundary`. Above 128, the two paths into one field disagree,
  unless the adventure's bound is raised, and it was set for a cost that
  multiplies per scene (its own block's argument).
- **The codebase's identifier bound is 128:** `maxNoteKeyBytes`.
- **Nothing comes near it.** The longest id in content is 18 bytes (P2), and
  the longest any test folds is 20 (P3).

The other options, measured:
- **250** (`maxArtIDLen`'s arithmetic, NAME_MAX minus `.json`): admits every
  ext4 map filename. It would have to raise the adventure's bound to 250, or
  leave a scene id with two bounds. It still does not admit every APFS
  filename (P13).
- **255:** no map id reaches it. The `.json` suffix spends five bytes on ext4,
  and on APFS an id reaches 750 bytes. "255 to admit any filename" is not an
  option a byte bound can offer (Gap 3).
- **64:** lowers the adventure's existing 128.

At 128, a map installed as a 129- to 250-byte filename is refused at boot and
at `load_map` by a field error naming the file (P4). No such map exists (P2).

**D2. Empty scene, token and adventure ids are refused in both folds (Q2(b)).**
- **Nothing breaks.** No test, golden, scenario or eventgen draw folds an
  empty one (P3), and the whole probe tree, which refuses them, is green (P5).
- **The loaders already refuse them.** The adventure loader refuses all four
  empty ids, and `LoadInstalled` refuses an empty map id (`idIsAFilename`).
  What still lets one through is `place_token` with `token_id: ""`
  (measured `ok=true`, P4), a map placement with an empty token, and a
  hand-written log.
- **An empty id is the absence the contract warns about.** protojson omits an
  empty string, so on the wire a `TokenPlaced` with `token_id: ""` is
  byte-identical to one with no id. That is the "fact carried by ABSENCE"
  `events.proto`'s `Actor.controller_id` comment refuses, and the actor's id
  is already held to it.

Under Q2(a) the three `== 0` arms, K6 to K8, K16 to K18, row B, row D and
`mapdef`'s empty-token check all go. The keys do not move either way, because
the check lines are the same lines.

**D3. Scope: the four creating ids, plus a map placement's token id at load
(Q4(a)).** A map's placement token id is in scope because its `TokenPlaced`
is one of the four events. Refusing it only in the fold would break the
ticket's second rule (Gap 1).

**The referring ids need no check.** In every arm that reads one, an accepted
event's id equals a key a creating arm stored, and so it passed the bound:

| Arm | Field | Held by |
|---|---|---|
| `TokenPlaced` | `scene_id`, `actor_id` | `st.Scenes`, `st.Actors` lookups |
| `TokenMoved`, `TokenRemoved` | `token_id` | `st.Tokens` lookup |
| `ActorRemoved`, both control events, `ConditionApplied`, `ConditionRemoved`, `ResourceChanged` | `actor_id` | `st.Actors` lookup (`controlTarget` for the two control events) |
| `DoorOpened`, `DoorClosed`, `SceneSeen` | `scene_id` | `st.Scenes` lookup |
| `NoteDeleted` | `key` | `st.Notes` lookup; the key is bounded at 128 already |

Not held, and named so nobody assumes they are:
- `TokenMoved.scene_id` and `from` are never read (SPEC-013). From a command,
  the gateway overwrites them from the snapshot.
- `TokenHidden.token_id` is not looked up, but deletes nothing absent and
  stores nothing.
- `AbilityUsed` and `AttackRolled` ids are testimony and are not read.

There is no `NoteDeleted.adventure_id`: the message carries only `key`.

**The other id-like strings are left to a ticket of their own (Q4).**

| String | Stored in the fold | Comes from | Longest measured |
|---|---|---|---|
| `ConditionApplied.condition_id` | `st.Conditions` (and `ConditionRemoved` must match it) | the rules interpreter, from ruleset files | 12 in content |
| `ConditionApplied.source` | `st.Conditions` | the rules interpreter | 27 folded |
| `ActorControlGranted.participant_id` | `Actor.controller_ids`, sent wherever the actor is | `grant_actor_control`, DM or agent; identity ids are 32 hex | 32 folded |
| `Actor.module_id` | `Actor` | `add_actor` | 14 in content |
| `Actor.resources` and `attributes` keys | `Actor` (and `ResourceChanged.resource` must match one) | `add_actor`; the adventure loader checks them against the ruleset's names | 12 for a resource name |
| `SceneObject.object_id` | `Scene.Objects` | map and adventure files | 16 |
| `Envelope.session_id` | `Session.ID` | the gateway, `sess-` plus 32 hex | 37 folded |

Each has a different source and, where one exists, a different loader. Each
is a smaller exposure than the four, which a command or an authored file
chooses freely. The report raises the list.

**D4. The Go fold.** Each id check comes first in its arm. The `ActorAdded`
check comes right after the existing nil-or-empty check:
- **`SceneCreated`:** `if len(sc.SceneId) == 0 || len(sc.SceneId) >
  maxIDBytes { return fmt.Errorf("engine: scene id must be 1-%d bytes, got
  %d", …) }`. Then the duplicate check and the name check.
- **`ActorAdded`:** after `a == nil || a.ActorId == ""`: `if len(a.ActorId) >
  maxIDBytes { return fmt.Errorf("engine: actor id must be at most %d bytes,
  got %d", …) }`. Then the duplicate, controller and name checks. The
  existing empty-id text stays, because SPEC-013 quotes it as "says the id is
  missing".
- **`TokenPlaced`:** the same 1-128 form, `engine: token id must be 1-%d
  bytes, got %d`. Then the duplicate, scene, actor and position checks.
- **`AdventureLoaded`:** the same form, `engine: adventure id …`. Then the
  name check and the existing `return nil // testimony, …`.

The texts take SPEC-018's two forms: `note key must be 1-128 bytes, got 0`
and `move reason must be at most 256 bytes, got 257`.

First, because the id's length is a property of the event alone. Its order
against the duplicate check cannot be observed (K30). Its order against the
scene, actor, controller and name checks can, and the TS mirror matches it.
Under Q2(a), all four use the "must be at most" form.

**D5. The TS mirror.** One `checkLen` per arm, at the same place in the arm:
- `checkLen("scene id", v.sceneId, 1, 128);` before the duplicate-scene throw.
- `checkLen("actor id", a.actorId, 0, 128);` after `if (!a || a.actorId ===
  "") throw …`. The minimum is 0 because the line above has already refused
  an empty id, and a minimum of 1 would be a second, unreachable refusal.
- `checkLen("token id", v.tokenId, 1, 128);` first in `tokenPlaced`. Its
  one-line comment becomes `// Error ORDER matters: Go checks the id,
  duplicate, scene, actor, position.`
- `checkLen("adventure id", p.value.adventureId, 1, 128);` before the
  adventure name check, in the existing `case "adventureLoaded":` arm.

No arm changes shape, so there is no layout decision this time. P9 found no
new adjudication. The literals mirror the constant, as every bound's do
(Gap 9).

**D6. The loaders.**
- **`mapdef`:**
  - `// maxIDBytes mirrors internal/engine's bound on an id (SPEC-018).` and
    `const maxIDBytes = 128` go below `maxNameBytes`. `mapdef` may not import
    `engine` (`.go-arch-lint.yml`).
  - In `loadAs`, after the two `format_version` checks and before `cell_px`:
    `if len(raw.ID) > maxIDBytes { return nil, fieldErr(display, "id",
    fmt.Sprintf("must be at most %d bytes, got %d", …)) }`. The `cell_px`
    comment's "after format_version and before the geometry" stays true.
  - In the placements loop, under Q2(b): `p.TokenID == ""` refuses with
    `fieldErr(display, fmt.Sprintf("placements[%d].token_id", i), "must not
    be empty")`. Then the length check, with the same field. The field name
    is the adventure loader's.
  - The map's own empty id stays `LoadInstalled`'s, through `idIsAFilename`
    and the declared-id check. No map fixture lacks one (P2).
  - Boot (`loadMapsDir`) and `load_map` (`mapByID`) both reach `loadAs`
    through `LoadInstalled`. **There is no install command for maps:** SPEC-014
    says a map is installed by writing `maps/<id>.json`, and `vtt` has an
    `art install` and no `map` command. So "refuse at install" does not exist
    to choose (Q3).
  - No pre-check is added in `LoadInstalled`. An id longer than the
    filesystem allows is refused by the open (`file name too long`), and
    `installed_test.go`'s and `map_test.go`'s 260-byte cases still hold:
    they ask for `maps/<id>.json` and the id in the text.
- **`adventure`:**
  - After each existing "must not be empty" check on `raw.ID` in
    `loadManifest`, `raw.ActorID` in `loadActors`, and a placement's
    `TokenID`, a length check with `fieldErr(path, <same field>,
    fmt.Sprintf("must be at most %d bytes, got %d", maxIDBytes, …))`.
  - A placement's `actor_id` needs none: it must name an actor the loader
    has already bounded.
  - `maxIDBytes` is unchanged at 128.
- **`loadScenes`' placement loop moves into an unexported `loadPlacements(path
  string, raw []placementJSON, w, h int32, actorIDs, seenToken
  map[string]bool) ([]Placement, error)`.** This is forced by gocyclo (P6),
  and by rule 2, since no `//nolint`. The order of the checks and their
  messages are unchanged, `loadScenes` calls it where the loop stood, and
  `loadScenes`' doc stays true. No adjudication key sits in the moved lines.
- **`TestTheIDBoundMirrorsEngine`** goes at the end of
  `internal/adventure/format_test.go`, in the form of
  `TestTheNameBoundMirrorsEngine`. It asserts `maxIDBytes == 128` and has a
  one-line message.

**D7. The gateway.** No production change. The fold refuses, and VTT-161 and
VTT-162 carry the refusal to the issuer (P4). Nothing is added to
`validateAddActor`, so SPEC-013's paragraph stays true.

**D8. The tests, each red today except the at-cap ones (P4).**
- **`internal/engine/apply_test.go`, at the end:**
  - `TestASceneWhoseIDExceedsTheBoundIsRefused`,
    `TestAnActorWhoseIDExceedsTheBoundIsRefused`,
    `TestATokenWhoseIDExceedsTheBoundIsRefused` and
    `TestAnAdventureWhoseIDExceedsTheBoundIsRefused`. Each sends 129 bytes,
    asserts the exact error, and asserts an unchanged `Snapshot`. The token
    test's state holds the scene and the actor.
  - Under Q2(b), `TestAnEmptySceneTokenOrAdventureIDIsRefused`, with three
    subtests asserting `… must be 1-128 bytes, got 0`.
- **`apply_boundary_test.go`, at the end:** `TestIDsAtCapAreAccepted`. All four
  events carry 128-byte ids, the token referring to the scene and the actor,
  and the test asserts the stored token.
- **`internal/gateway/server_test.go`, after
  `TestASessionWhoseNameExceedsTheBoundAppendsNothing`'s closing brace:**
  - `TestAnActorWhoseIDExceedsTheBoundAppendsNothing`: 129 bytes, ok=false
    with the exact text and `f.head` unchanged; then 128 bytes answers ok at
    `head+1`. The command sets `Kind`, because `validateAddActor` refuses a
    kindless one first.
  - `TestATokenWhoseIDExceedsTheBoundAppendsNothing`: the same, on
    `newGWFixture`'s `scn1` and `a1`; under Q2(b), an empty id as well.
  - Both cite A, VTT-161 and VTT-162.
- **`client/test/fold-rejections.test.ts`, at the end, in an "ids" section:**
  - a refusal per field, the scene's multibyte (`é`×64 + `i`);
  - under Q2(b), an empty case for scene, token and adventure;
  - one at-cap case folding all four at 128, which asserts the stored token.

  One at-cap case for all four, as the name arc did, because the register
  cell is comma-separated.
- **`internal/mapdef`:**
  - `testdata/invalid/id-too-long/map.json`, with a 129-byte declared id
    (`Load` does not compare filenames);
  - `testdata/invalid/placement-token-id-too-long/map.json`, and under
    Q2(b) `placement-token-id-empty/map.json`;
  - their rows in `TestInvalidMapsAreRefusedWithAUsefulReason`, with full
    `field "…": …` texts, since `object-art-empty` already asserts "must not
    be empty";
  - `TestAMapIDOfExactlyTheBoundLoads` in `load_test.go`: an id and a
    placement token id of 128 each;
  - `TestAnInstalledMapWhoseIDExceedsTheBoundIsRefused` in
    `installed_test.go`. It writes `maps/<129 bytes>.json` declaring the
    same id, and asserts `maps/<id>.json: field "id": must be at most 128
    bytes, got 129`. That is the path boot and `load_map` share.
- **`internal/adventure`:**
  - `testdata/invalid/adventure-id-too-long`, `actor-id-too-long` and
    `placement-token-id-too-long`. Each is a copy of `scene-id-too-long`'s
    shape with one 129-byte id, and each has its own correct `guide.md`.
  - Three rows in `TestLoadInvalidFixtures`, naming the file, the field
    (`id`, `actor_id`, `placements[0].token_id`) and `at most 128 bytes, got
    129`.
  - The three names added to the catalogue list.
  - `at-every-boundary`: `adventure.json`'s id, `edge-actor.json`'s
    `actor_id` (with `pin.json`'s placement `actor_id` to match), and
    `pin.json`'s placement `token_id`, each at exactly 128 bytes.
  - Three assertions at the end of `TestLoadAcceptsValuesExactlyOnEveryLimit`
    lock those values, with no comment, so no block is touched.
  - `guide.md`'s table gains the three id rows and the three name rows the
    name arc left out (Q7).

The goldens need no case: no stream holds an id within 108 bytes of the bound.

**D9. The tool descriptions state the bound (Q5).**
- `add_actor`: `manifest`'s `vtt.v1.Actor` `fieldDocs` gains `"actorId": "The
  new actor's id, unique in the campaign. At most 128 bytes of UTF-8; a longer
  id refuses the whole command."`.
- `place_token` gains `overrides: {"vtt.v1.PlaceToken": {fieldDocs:
  {"tokenId": "The new token's id, unique in the campaign. At most 128 bytes
  of UTF-8; a longer id refuses the command."}}}`, with `requiredOverride`
  left nil.
- Neither carries `maxLength`, because JSON Schema counts characters.

This **overturns an assertion**: `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication`
requires `actorId` to carry no description, "nothing to steer it away from".
Its `actorId` half becomes a requirement to state the bound. Its five-line
doc says the one required field "must carry none", and that clause changes
to match. `kind` is the precedent: a required field that carries guidance.

The other tests and regeneration:
- `TestAddActorAndPlaceTokenStateTheIDBound` in `tools/toolgen/main_test.go`
  asserts both descriptions.
- `TestTheToolsStateTheFoldsIDBound`, in a new
  `internal/engine/id_bound_internal_test.go` (`package engine`), reads
  `../../contract/gen/tools/tools.json` and requires `fmt.Sprintf("At most %d
  bytes", maxIDBytes)` in `add_actor`'s `actor.actorId` and `place_token`'s
  `tokenId` (P11).
- `task generate:contract` regenerates both `tools.json` files.
  `expected_tools.json` gains its two lines by hand.

**D10. The records.**

**SPEC-018**, written with the `specification` skill against the final tree:
- **Status:**
  - add `maxIDBytes`;
  - "the same seven arms" becomes eight, adding `TokenPlaced`;
  - add both loaders' `maxIDBytes`, the two tool statements, and the new
    test files.
- **"Ten fields are bounded"** becomes "Fourteen". The table gains
  `SceneCreated.scene_id`, `ActorAdded`'s `Actor.actor_id`,
  `TokenPlaced.token_id` and `AdventureLoaded.adventure_id`, each 128
  (`maxIDBytes`). The "May be empty" column is "no" for all four; under Q2(a)
  it is "yes" for all but the actor's.
- **The order sentences** gain:
  - each of the four checks its id first;
  - an `ActorAdded` checks for an actor with an id, then the id's length,
    then a duplicate, a declared controller and its name;
  - a `TokenPlaced` checks its id, then a duplicate, a known scene, a known
    actor and a position;
  - an `AdventureLoaded` checks its id, then its name.
- **"These ten are the only texts … beyond them it requires an ActorAdded's
  actor id and a control event's participant id to be non-empty"** becomes
  "These fourteen …; beyond them it requires a control event's participant id
  to be non-empty".
- **The clause "one a map or adventure file carries is bounded only by what
  its loader checks, which for an adventure's scene id is 128 bytes"** becomes
  "one a map or adventure file carries has no bound in the fold". The loaders'
  copies move to "Other mirrors".
- **"for each of the ten fields"** becomes "fourteen".
- **"Other mirrors"** gains:
  - `internal/adventure/load.go`'s `maxIDBytes`, for an adventure's id, its
    scenes' and actors' ids and its placements' token ids, pinned by
    `TestTheIDBoundMirrorsEngine`;
  - `internal/mapdef/load.go`'s `maxIDBytes`, for a map's id and its
    placements' token ids, held by the refusal rows and
    `TestAMapIDOfExactlyTheBoundLoads`;
  - the tool paragraph names `add_actor`'s actor id and `place_token`'s token
    id, held by `TestTheToolsStateTheFoldsIDBound`.
- **Consequences**, second bullet: add "for an id, the `add_actor` and
  `place_token` descriptions". The third bullet stays true.
- **Requirements:** add the new rows.
- **The heading** keeps its number and path: two plans cite the path. Whether
  "free text" stays in the title is the `specification` skill's call.

**SPEC-014:**
- The sentence "an id that is not one plain filename, a filename that differs
  from the id the file declares, and a file that does not compile are refused
  there" gains: an id longer than 128 bytes of UTF-8 (SPEC-018), and a
  placement whose token id is longer, or under Q2(b) empty.
- The first consequence becomes: "A map is installed by writing
  `maps/<id>.json`, with an id of at most 128 bytes of UTF-8, into the
  campaign …".
- "What this record does not decide" keeps "what a map file may say … are
  `internal/mapdef`'s". The bound is SPEC-018's, and this record states only
  that `LoadInstalled` refuses it at both doors.

Phase 4b verifies every replaced sentence in both records by command.

**The register:** VTT-161 and VTT-162 gain the two wire tests. VTT-154 and
VTT-172 are unchanged.

**`docs/verification-debt.md`'s open entry "Nothing ties a byte bound's
copies to the engine's constant"** is amended:
- "ten bounds" becomes "fourteen";
- `internal/adventure/load.go`'s copies gain `maxIDBytes`, and
  `internal/mapdef/load.go`'s gain `maxIDBytes`;
- "Only a move's reason and a name have a link" becomes "a move's reason, a
  name and an id".

An open entry is a claim on future work, and a false count in it misleads.
No new debt entry is needed under Q6(a) and Q7(yes).

**D11. The sort.** Forced by SPEC-008. The rows are lettered so nothing reads
as an id.

| # | Rule | Proposed | Red when | Tests | Record |
|---|---|---|---|---|---|
| A | Both folds refuse a `SceneCreated`, `ActorAdded`, `TokenPlaced` or `AdventureLoaded` whose id is longer than 128 bytes of UTF-8, and accept one of exactly 128. | accept: the ticket's first rule, and the refusal half of its third | K1 to K5, K9 to K15, K19 | D8's engine, TS and wire tests | SPEC-018 |
| B | Both folds refuse a `SceneCreated`, `TokenPlaced` or `AdventureLoaded` whose id is empty, as they refuse an `ActorAdded`'s. | accept under Q2(b) | K6 to K8, K16 to K18 | the empty cases | SPEC-018 |
| C | A map file whose id or placement token id, or an adventure whose id, actor id or placement token id, is longer than 128 bytes of UTF-8 is refused when it loads, by an error naming the file and the field. | accept: the second rule, with the map's placements (Gap 1) | K20 to K23, K25 to K28 | D8's loader tests and fixtures, `TestTheIDBoundMirrorsEngine` | SPEC-018, SPEC-014 |
| D | A map file whose placement has an empty token id is refused when it loads, naming the file and the field. | accept under Q2(b) | K24 | `mapdef`'s empty-token row | SPEC-014 |
| E | The `add_actor` and `place_token` tools state the bound the fold enforces on the id they create. | accept under Q5 | K9, K29, K31 | D9's two tests | SPEC-018 |

**Refused, one line each:**
1. "An id is at most 128 bytes" on its own: A holds it.
2. "A refused `add_actor` or `place_token` appends nothing": VTT-162, which
   gains the wire tests.
3. "It is answered ok=false on an open connection": VTT-161, likewise.
4. "`fold.ts` refuses what `engine.Apply` refuses" as a parity row: A names
   both folds.
5. "A referring id is bounded": it follows from A through the existence checks
   (D3's table), and is no rule of its own.
6. "The refusal names the bound and the length": how A and C are observed, by
   exact text.
7. "A loader's copy equals the engine's constant": a consistency of copies,
   which is SPEC-018's consequence and the debt entry.
8. "The id is checked first in its arm": SPEC-018's order text. Against the
   duplicate check it cannot be observed (K30).
9. "A projected introduction carries an id within the bound": it follows from
   A.
10. "A map id the filesystem cannot hold is refused at `load_map`": existing
    behaviour, held by the 260-byte cases.
11. "An empty actor id is refused": existing behaviour of both folds, which
    SPEC-013 states.

Five rows are proposed (three under Q2(a)), eleven candidates refused, none
withdrawn. A is one row, not one per event: one constant and one check shape,
and four rows would always move together.

**D12. Mutation keys: re-point last, by statement.** These apply after the
review settles and before the commit. P7 gives the positions; re-read each at
its statement.
- **`apply.go`**, four keys, the same under Q2(a) or (b):
  - `id < standing` (`ActorRemoved`): +10, from `341:58` to `351:58`;
  - `computed < 0`: +13, from `452:15` to `465:15`;
  - `computed > int64(res.Max)`: +13, from `455:30` to `468:30`;
  - `len(objs) > 0`: +13, from `599:41` to `612:41`.

  The shift is the constant (+1) and the scene, actor and token checks (+3
  each), then the adventure check (+3) below `ActorRemoved`.
- **`internal/adventure/load.go`:** `attrOrDefSet := make(…)`'s
  `ARITHMETIC_BASE` moves by whatever D13's cut of the `maxIDBytes` block
  removes. With a one-line pointer it moves −34, from `123:58` to `89:58`.
  Every line kept adds one. The extraction and the new checks sit below it.
- **`fold.ts`**, sixteen keys:
  - sceneSeen's four (`323:11`, `324:11`, `325:11`, `325:37`),
    conditionRemoved's `?? []` (`382:48`) and resourceChanged's three
    (`411:19`, `420:11`, `421:26`): +3, from the scene, actor and token lines;
  - `"attackRolled"` (`457:10`), `"abilityUsed"` (`458:10`), `default:`
    (`459:5`), ensureOpenDoors (`488:7`), headSequence (`578:34`),
    foldToDumpJSON (`600:32`) and actorJSON's two (`705:7`, `705:21`): +4,
    with the adventure line.

  None is deleted and none added (P9). The reasons stay true: `705:7`'s
  equivalence rests on an actor id never being empty, which both folds still
  hold.
- **`internal/mapdef` and `internal/artlib`** carry no keys.

Then run `python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q`. The TS self-test cannot see `382:48`,
`411:19`, `600:32` or `705:7` move (P16), so its `OK` is necessary, not
sufficient; Phase 4b re-reads every key. If a review adds a line above an edit
point, re-measure every key. The new mutants are killed as follows:
- Go `>=`: by the at-cap test.
- Go negations: by the refusals and by every valid fold.
- The TS labels emptied: by the exact messages.
- The loaders' `>=`: by the at-cap fixtures and tests (P8).

**D13. Comments and the ledger.** Tests carry their `// VTT-NNN` line only.
`loadPlacements` carries no doc, or one line at most. The block edits are
these, each sized for `check:comments`' rule:
- **`internal/adventure/load.go`'s 35-line `maxIDBytes` block** is cut to the
  one-line pointer `// maxIDBytes mirrors internal/engine's bound on an id
  (SPEC-018).`. Two of its paragraphs become false, and a touched block over
  six lines is refused. The arithmetic argument (warnings prefixed by a scene
  id multiply per scene) and its history go to the report. The const block's
  nine-line header names only the other three constants and stays.
- **`internal/artlib/artlib.go`'s 74-line const-block doc** is cut to six
  lines or fewer (Q6(a)). Its "STILL UNBOUNDED" paragraph becomes false for
  two of its three entries. What stays is the warning that matters, as a
  warning: every author string that reaches a message goes through `Clip` or
  `BoundErr`, and a surviving mutant on a second clip means a missing test,
  not a spare guard.
  - `MaxFragment` and `MaxMessage` keep a one-line doc each.
  - `Clip`'s nine-line doc says "See the package doc and clip below". If that
    pointer dangles after the cut, `Clip`'s doc is cut to six lines too.
  - The history goes to the report.
- **`fold.ts`'s `tokenPlaced` order comment** stays one line.
- **`tools/toolgen/main_test.go`'s five-line doc** is under `tools/`, outside
  `check:comments`.

`--write-ledger` runs once, before C1, and its output is read row by row. It
lowers at least `internal/adventure/load.go`, `internal/mapdef/load.go` and,
under Q6(a), `internal/artlib/artlib.go`.

**D14. One code commit and the report (Q9).** Forced by measurement:
- The folds land together (rule 4).
- The loaders cannot trail the fold: a map that boots and is then refused at
  `load_map` breaks the second rule.
- A description stating a bound no fold enforces would be false.

**C1** holds:
- the code, the tests and the fixtures;
- the regenerated files and the golden lines;
- SPEC-018 and SPEC-014, the rows and the debt edit;
- the keys and the ledger.

`cmd/vtt/webdist/assets/index.js` is rebuilt by `task build:client` after
`fold.ts` changes, and goes in C1 too. **C2** holds the report.

**D15. Phase 4a for C1.** One QA agent per `qa-prompt.md`. It is never given
the diff, the source, the existing tests or the implementer's report. It is
given:
- rows A to E, VTT-161 and VTT-162;
- SPEC-018 whole;
- SPEC-014's lookup, refusal and consequences paragraphs;
- SPEC-013's command-path and validators paragraphs;
- SPEC-007's `load_map` batch paragraph;
- the text of `SceneCreated`, `Actor`, `ActorAdded`, `TokenPlaced`,
  `AdventureLoaded`, `AddActor`, `PlaceToken` and `LoadMap`;
- `go doc -all` of `internal/engine`, `internal/gateway`,
  `internal/campaign`, `internal/identity`, `internal/mapdef` and
  `internal/adventure`;
- `grep -n '^export' client/src/fold.ts`;
- the `add_actor` and `place_token` entries of `tools.json`.

It writes:
- `internal/gateway/qa_id_bound_test.go`;
- `internal/mapdef/qa_id_bound_test.go`;
- `internal/adventure/qa_id_bound_test.go`;
- `client/test/qa-id-bound.test.ts`.

It builds its fixtures at run time in `t.TempDir()` and commits none, as the
name arc's QA did. A 129-byte map id is a 134-byte filename, legal on every
filesystem.

Adjudications go in the report, one line per finding. An escape goes to the
debt file as a recipe. A QA test that pins an ABSENCE no rule states is
dropped. Examples are a `condition_id`, a participant id or a `module_id` left
unbounded, or a referring id checked by length.

**D16. Phase 4b for C1, after 4a.** One reviewer at high effort, briefed to:
- verify by command every sentence C1 adds or rewrites: SPEC-018, SPEC-014,
  D13's cut blocks, the debt edit, the two `guide.md` sets and the two
  descriptions;
- re-read every re-pointed key at its statement;
- check the break lines in the draft message.

If the reviewer dies on a model limit, say so and re-dispatch the same brief
on `fable`.

**D17. The breaks: exactly the K table, and none named elsewhere.**
- **Run** K1 to K32 in a scratch clone of C1's final tree, against the real
  tests. K6 to K8, K16 to K18 and K24 run only under Q2(b), and K29 and K31
  only under Q5.
- **K30 is expected to stay green**, and its line in the message says so.
- **The script** is written from this table alone, and the table is complete.
  The name arc missed P5's break because it stood outside the table.
- **Each break** is one edit, made after every gate is clean. Each file is
  restored from its saved text and checked by hash.

**D18. Disk and load.** Forced by `check:mutation`'s 16 GiB floor and by MCP
e2e deadlines under load. Immediately before `task check`, run `go clean
-cache`, `df -k` and `python3 -c "import os; print(os.getloadavg())"`. Launch
the gate once after C1, in its own session (`start_new_session=True`). Below
16 GiB, stop. Above a load of about 8, wait.

## Tasks, in dependency order

### Task 0 — Baselines

**Done when:** `df -k`, the load, the chain, comments and doc-owner baselines
and both self-tests match Measurements, and `requirement-id` resolves.

### Task 1 — Rows

**Files:** `docs/requirements.md`. After sign-off, run `requirement-id` once
per accepted row, in letter order. **Done when:** that many new OPEN rows
exist.

### Task 2 — The Go fold

**Files:** `internal/engine/apply.go`, `apply_test.go`,
`apply_boundary_test.go`.

D8's engine tests come first (the refusals are red), then D4. **Done when:**
`go test -count=1 ./internal/engine/` is green and `go test -count=1
./internal/...` shows nothing else red (P5 found nothing that depends on a
long or empty id).

### Task 3 — The gateway

**Files:** `internal/gateway/server_test.go`.

D8's two wire tests: red on the base, green on Task 2's tree. **Done when:**
`go test -count=1 ./internal/gateway/` is green.

### Task 4 — The TS mirror

**Files:** `client/src/fold.ts`, `client/test/fold-rejections.test.ts`,
`cmd/vtt/webdist/`.

The cases come first, then D5, then `task build:client`. **Done when:** `bun
test client/test` is green, `client:typecheck` is clean, and `git status`
shows `index.js` as the only webdist change.

### Task 5 — The loaders

**Files:**
- `internal/mapdef/load.go`, `load_test.go`, `installed_test.go`, and the two
  or three new `testdata/invalid/*/` directories;
- `internal/adventure/load.go`, `load_test.go`, `format_test.go`, the three
  new `testdata/invalid/*-too-long/` directories, `at-every-boundary`'s
  `adventure.json`, `actors/edge-actor.json`, `scenes/pin.json` and
  `guide.md`, and, under Q7, the three name-arc `guide.md` files.

The fixtures and rows come first (red), then D6, the extraction last. D13's
cut of the `maxIDBytes` block goes in the same edit as the checks. **Done
when:**
- `go test -count=1 ./internal/mapdef/ ./internal/adventure/...` is green;
- `go test -count=1 ./cmd/vtt/` is green (shipped content boots);
- `gocyclo -over 30 internal/adventure/load.go` prints nothing;
- `golangci-lint run ./internal/adventure/ ./internal/mapdef/` reports 0
  issues.

### Task 6 — The tools

**Files:**
- `tools/toolgen/main.go` and `main_test.go`;
- both `tools.json` files and `contract/testdata/expected_tools.json`;
- `internal/engine/id_bound_internal_test.go`.

D9's toolgen test is red first. Then the `fieldDocs`, `task generate:contract`,
the golden lines, the changed `actorId` assertion and the link test. **Done
when:** `go test ./tools/toolgen/ ./internal/engine/ ./internal/mcp/
./contract/` is green, and `git status` shows exactly the two `tools.json`
files regenerated.

### Task 7 — artlib's doc (under Q6(a))

**Files:** `internal/artlib/artlib.go`.

D13's cut. **Done when:**
- `go doc ./internal/artlib MaxFragment` and `go doc ./internal/artlib Clip`
  each print a true one-line doc;
- `grep -c 'STILL UNBOUNDED' internal/artlib/artlib.go` prints 0;
- `check-comments.py main` reports no touched block over six lines.

### Task 8 — The records

**Files:** SPEC-018, SPEC-014, `docs/requirements.md` (the new rows' evidence;
VTT-161 and VTT-162), `docs/verification-debt.md`.

D10. **Done when:**
- the chain prints 276 rows plus the new ones and `13 specifications`, with
  none of the new rows OPEN;
- `grep -c 'Ten fields\|These ten\|of the ten' docs/specifications/018-*`
  prints 0;
- `grep -c 'adventure.s scene id is 128' docs/specifications/018-*` prints 0.

### Task 9 — Local gates

Run:
- `gofmt -l` over the touched files only (Gap 12);
- `go vet` and `task lint`;
- `go test -count=1 ./internal/... ./contract/... ./tools/... ./cmd/...`;
- `bun test client/test contract contract-spike`;
- semgrep, `go-arch-lint`, `check:comments`, `check:doc-owner` and
  `check:new-prose`.

**Done when:** each prints its own completion line. A Go failure is captured
whole and re-run in isolation before it is called a flake.

### Task 10 — Phase 4a, then 4b

D15 and D16. Findings are fixed, and the affected task's "done" is re-run. The
review settles before Task 11.

### Task 11 — Keys, ledger, commit C1

D12 and D13's `--write-ledger`, then C1's message, which lists the ids and
D17's lines. **Done when:**
- both self-tests print `OK`, and every key reads its statement;
- `git show --stat HEAD` lists C1's files;
- `task check:drift` is clean.

### Task 12 — Breaks and the whole gate

D17, then D18, then `task check` once. **Done when:** each break gives its
red (K30 its green), and `task check` exits 0 with every step's own verdict,
`check:mutation` and `check:ts-mutation` included.

### Task 13 — The report

`docs/reports/2026-10-06-an-id-is-bounded.md`, per the
`implementation-report` skill. It covers:
- each Done item with its observation;
- the rows and refusals;
- the rulings taken at sign-off;
- the rule-9 answer;
- the breaks;
- the history D13 removed from comments;
- D3's list of id-like strings left unbounded, raised to the owner;
- the gaps.

**Done when:** C2 holds it alone. Push after C2, and let the pre-push hook
finish.

## Gaps that travel with this plan

1. **The problem paragraph's "four ways" omits a fifth: a map's placements.**
   `load_map` emits one `TokenPlaced` per placement (`mapdef.Compile`;
   SPEC-007), and the shipped `campaigns/example/maps/cellar.json` places
   `tok-fighter`. Done item 3 and the second rule name a map's id only. D3, D6
   and row C cover the placement's token id.
2. **"The longest id in the repository's content … is 18 bytes, … apart from
   `scene-id-too-long`'s 200-byte id" omits
   `at-every-boundary/scenes/pin.json`'s 128-byte scene id**, which loads
   (P2). The 18 holds for everything else. The longest id any test folds is
   20 bytes (P3).
3. **"APFS and most Linux filesystems allow up to 255 bytes" is wrong for APFS
   and off by the suffix for both.** APFS counts 255 UTF-16 units, and a
   750-byte map id is a legal filename on this machine. On ext4 the `.json`
   suffix leaves an id 250 bytes (P13; `maxArtIDLen`). No byte bound "admits
   any filename".
4. **"What it touches" omits work that Done item 5 needs:**
   - `loadScenes`' extraction for gocyclo (P6);
   - `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication`'s `actorId`
     assertion (P10);
   - the two false comment blocks in `internal/adventure/load.go` and
     `internal/artlib/artlib.go` (P15);
   - the ledger's two findings (P12);
   - the fixtures and the catalogue entries (D8);
   - `expected_tools.json`.
5. **The "accepts exactly the bound" halves of Done items 1 and 2 cannot fail
   today.** K5 and K13 hold them.
6. **The ticket overturns a recorded decision without saying so.** The
   adventure loader's `maxIDBytes` block records that the map path was "left
   alone rather than tightened to match" (check 5).
7. **Three `guide.md` files from the name arc are false.** They describe a
   200-byte scene id their fixtures do not have. `at-every-boundary/guide.md`
   omits the name rows (P15, P14, Q7).
8. **A campaign whose log already holds an id over 128 bytes, or an empty
   scene, token or adventure id, stops opening.** So does one whose `maps/`
   holds such a map, which stops booting. No shipped content, fixture, golden
   or eventgen draw comes within 108 bytes of the bound or holds an empty id
   that folds (P2, P3). Nobody uses the product (the owner, 2026-09-04).
9. **Nothing links `fold.ts`'s `128` or either loader's `maxIDBytes` to the
   engine's constant** beyond each side's tests. The open debt entry is
   amended rather than closed (D10).
10. **The TS self-test cannot see four of the sixteen keys move** (P16).
    Phase 4b's re-read is the check.
11. **The id check's order against the duplicate check cannot be observed**
    (K30). SPEC-018 states it, and no test can hold it.
12. **`gofmt -l` lists `internal/gateway/scenario_test.go` on `main`.** This
    is not this ticket's to fix, and Task 9 formats only what it touches.

## Questions for sign-off

1. **Is the bound 128 bytes of UTF-8, inclusive, one `maxIDBytes` for all four
   ids?** Recommend yes (D1). An adventure's scene ids are already held at
   128, a map's id is a scene id, and 128 is the codebase's identifier bound.
   250 would split a scene id between two bounds and still not admit every
   APFS filename, and nothing in the tree exceeds 20 bytes.
2. **Empty ids:** (a) keep today's asymmetry; (b) refuse an empty scene, token
   or adventure id in both folds, and an empty map placement token at load.
   Recommend (b) (D2). It breaks nothing (P5), the loaders already refuse
   empties, and on the wire an empty id is indistinguishable from an absent
   one.
3. **Where is a map's over-long id refused?** In `mapdef`'s `loadAs`, which
   boot and `load_map` both reach through `LoadInstalled`, with SPEC-014
   saying so. Recommend yes (D6). There is no map install command to refuse
   at (SPEC-014: a map is installed by writing the file).
4. **Scope:** (a) the four creating ids, and a map placement's token id at
   load; (b) (a) plus `ActorControlGranted.participant_id`; (c) (a) plus
   every id-like string in D3's table. Recommend (a). Every referring id is
   held by an existence check (D3's table). The rest have different sources,
   and the report raises them as a ticket. Under (a) the ticket's writer adds
   the map placements to its second rule, as the name ticket was revised
   after its Q3.
5. **Do `add_actor`'s `actorId` and `place_token`'s `tokenId` state the bound,
   held to `maxIDBytes` by an internal engine test, overturning the toolgen
   assertion that `actorId` carries no description?** Recommend yes (D9). The
   agent chooses both ids, and learns the bound before a refusal rather than
   after.
6. **`internal/artlib/artlib.go`'s 74-line doc, whose "STILL UNBOUNDED" list
   this change makes false for two of three entries:** (a) cut it, and
   `Clip`'s doc if its pointer dangles, to six lines or fewer in C1, moving
   the history to the report; (b) leave it and record its falsity as a debt
   entry. Recommend (a) (D13). `check:comments` refuses a partial edit, and a
   comment known to be false is the defect this project measures most.
7. **The three name-arc `guide.md` files that describe a 200-byte scene id
   they do not have, and `at-every-boundary/guide.md`'s missing name rows:**
   fix them in C1? Recommend yes. They sit beside the fixtures this change
   adds, and the new fixtures would otherwise copy the same template a
   fourth time.
8. **Rows A to E (A and C only, plus E, under Q2(a)), with A one row for all
   four ids, and VTT-161 and VTT-162 gaining the wire tests?** Recommend yes
   (D11).
9. **One code commit and the report (D14)?** Recommend yes. The two folds
   cannot land apart, and the loaders cannot trail them.
