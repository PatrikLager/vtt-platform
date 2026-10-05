# A name is bounded — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-05-a-name-is-bounded-design.md`
**Verified:** 2026-10-05 by `verify-ticket` (dev-cycle 0.4.0). The verifier is
an agent that did not write the ticket. It verified against `fbf2ff1` on
`feat/names-are-bounded`, which equals `main`, with the ticket untracked.
Verdict: **Passes with gaps.** The gaps are listed at the end and travel with
this plan. This plan does not edit the ticket.

**Goal, in the ticket's words:** both folds refuse a `SceneCreated`, an
`ActorAdded` or an `AdventureLoaded` whose name is longer than its bound, and
accept one of exactly the bound. A map file or an adventure carrying a longer
name is refused when it loads, naming the file and the field, before anything
is appended. An `add_actor` carrying one is answered ok=false and appends
nothing. Every golden, fixture and shipped content file still loads and
folds, and `task check` whole is green.

**MapTool (CLAUDE.md rule 9): it bounds no name.** Read in
`~/dev/RPTool/maptool` at `f4b7fef6c`:

- `Token.setName`, `Zone.setName` and `Campaign.setName` assign the string they
  are given.
- `CampaignDto.name` and `TokenDto.name` in
  `messages/src/main/proto/data_transfer_objects.proto` are plain `string`s.
- The one rule on a name is `Token.validateName`. It refuses a non-GM,
  untrusted rename that duplicates another token's name on the zone, ignoring
  case, and it has no length rule.
- `git grep` over `src/main/java` for a name compared against a length finds
  only emptiness tests (`length() > 0`), a `lib:` prefix test in
  `LibraryTokenManager`, and a font-name width in `ThemeFontTools`.

**Borrowed:** nothing. **Refused:** the unbounded name, because the log is
append-only, so a name the fold accepts once it must accept forever (SPEC-018's
first consequence). **Noted and not taken here:** MapTool keeps a second,
audience-specific name, `Zone.playerAlias` (with `getDisplayName` falling back
to `name`) and `Token.gmName`. That answers a different question: what a seat
is told a place is called. Every projected seat is sent a scene's name in its
introduction (SPEC-016), so the question is real, but it is a projection
ruling, not a bound.

## Verification, check by check

1. **Every path resolves (by command).** `ls` finds all 16 paths the ticket
   names: 12 files and 4 directories. `grep` finds each named symbol where the ticket puts it:
   - `func Compile` in both `compile.go`s;
   - `func ToEvent`, `func validateAddActor`, `maxIDBytes = 128`, and
     `const maxWSFrameBytes = 32768` in `internal/gateway/server.go`;
   - `string name = 2` on `SceneCreated`, `Actor` and `AdventureLoaded`, and
     `string name = 1` on `SessionStarted`;
   - the four `scene_id`, `actor_id`, `token_id` and `adventure_id` fields.

   SPEC-018's quoted sentence is there verbatim. The reason-bound report's
   "raised to the owner with this report; no ticket yet" is there too. That
   the owner asked for this ticket on 2026-10-05 is a reading; no command
   reaches it.
2. **"Done" is an observation (by command).** None of the named tests exists
   yet. Sketches of each fail today and pass after:
   - Item 1: P4's engine refusals are red with `err = <nil>`.
   - Item 2: P4's TS refusals are red with `Received function did not throw`.
   - Item 3: P4's mapdef and adventure probes are red with `err = <nil>`.
   - Item 4: P4's wire probe is red with `257-byte name: ok=true error=""`.
   - Item 5 is the gate. It is reachable only through work the ticket does not
     list (Gap 1).

   The "accepts one of exactly the bound" half of items 1 and 2 cannot fail
   today, because nothing refuses (Gap 5). K4 and K10 hold it.
3. **Each rule is breakable (a reading, then probes).** Each of the three rules
   has a single edit that reds a named test (K1 to K15, all measured):
   - The first rule becomes row A.
   - The second becomes row B.
   - The third is VTT-162 and VTT-161, which gain the wire test. The ticket
     says it cites them rather than restating them, and the sort agrees (D11).
4. **The scope matches the claim (by command, then a reading).** It does not.
   The work is larger than "What it touches" says, inside the same four
   components:
   - **Four gateway tests ride a 28,672-byte name** (`bigPaddingName` in
     `internal/gateway/server_internal_test.go`). Three use it as an
     `add_actor`'s `Actor.Name`. One appends it directly as a
     `SceneCreated.Name`. With the Go bound in place all four go red (P5).
     Done item 5 cannot be met until the padding moves (D7, Gap 1).
   - **The loaders' tests need fixtures the ticket does not name.**
     `TestLoadAcceptsValuesExactlyOnEveryLimit` holds that every byte cap
     `internal/adventure/load.go` checks has a value in `at-every-boundary`
     sitting exactly on it. `TestLoadInvalidFixturesCatalogueIsComplete`
     requires one `testdata/invalid` directory per refusal. Gremlins measured
     the new boundary mutants living in both loaders until at-cap values
     existed (P10).
   - **One more adjudication key, and two keys that stop being survivors.**
     `internal/adventure/load.go` carries a key that a constant in its const
     block would move. In `fold.ts` the change kills two adjudicated mutants,
     so their entries must go rather than be re-pointed (P11, D12, Gap 3).

   The ticket's other claims hold:
   - `engine.Apply`'s callers are unchanged: five calls in `internal/campaign`
     and one in `internal/harness`.
   - `SceneCreated` is built by `mapdef.Compile`/`BuildSceneCreated`,
     `adventure.Compile` (through `BuildSceneCreated`), `project.go`'s
     introduction and `internal/eventgen`.
   - `ActorAdded` is built by `adventure.Compile`, `ToEvent`, `project.go`'s
     `introduce` and `eventgen`.
   - `AdventureLoaded` is built by `adventure.Compile` alone.
   - Both projection sites copy names out of folded state, so they need
     nothing.
   - `eventgen` draws `prop-scn-N` and `prop-actor-N`, which cannot reach 256
     bytes. It draws no `AdventureLoaded`.
   - The MCP server is a wire client, so `add_actor` reaches the gateway only
     through a WebSocket frame.
5. **No recorded decision is contradicted silently (a reading).** SPEC-018
   records names as unbounded, and the ticket names SPEC-018 as moved.
   SPEC-013's "what the fold refuses is `engine.Apply`'s" and its validators
   paragraph stay true. `validateAddActor` gains nothing, because the fold
   refuses and VTT-161 and VTT-162 carry the refusal to the issuer.
   SPEC-014's "what a map file may say … are `internal/mapdef`'s" places the
   map loader's check in `mapdef`, which is where D6 puts it. ADR-007: no
   contract change.
6. **The records the work moves are named (by command, then a reading).** The
   section lists SPEC-018 and the path resolves. The reading:
   - SPEC-007, SPEC-011, SPEC-013, SPEC-014 and SPEC-016 hold no sentence the
     change makes false. Each was grepped for names, these events and the
     frame bound.
   - Two records outside `docs/specifications/` do become false and are not
     named. `docs/verification-debt.md`'s open entry "Nothing ties a byte
     bound's copies to the engine's constant" counts "the fold's six bounds"
     and three adventure copies (D10). The comment blocks on `bigPaddingName`
     describe a name that will no longer carry the padding (D7).

## Measurements this plan stands on

The verifier took every measurement by command, never in the working tree,
in a scratch clone of `fbf2ff1` (`git clone --no-hardlinks`,
`bun install --frozen-lockfile`). The probe tree is the sketch in D1 to D7:
names only, 256, empty allowed, the padding moved, at-cap fixtures, and
Q6(a)'s layout. The sketches were discarded and the clone deleted.

| # | What | Result |
|---|---|---|
| P1 | Base. An engine test folds a 10 KB name into `SessionStarted`, `SceneCreated`, `ActorAdded` and `AdventureLoaded`, plus empty names, an empty scene id, and a 10 KB scene id and actor id. A bun test folds the same names. | Both folds accept every one: `scene name 10240 B, actor name 10240 B, session name 10240 B; empty … accepted` |
| P2 | Base. Every `name`, id and event name in 906 tracked JSON files (905 parse; `client/tsconfig.json` is JSONC), by a walk over `git ls-files '*.json'` | The longest `name` value anywhere is 30 bytes: `hunters-flurry-usage-exhausted`, the name of a ruleset golden case, which no event carries. The longest names that reach an event: a scene's is 17 bytes (`The Sunken Cellar`, `campaigns/example/maps/cellar.json`); an actor's is 19 (`Vim Fighter (again)`, an adventure fixture); an adventure's is 17 (`Fixture Adventure`); a session's is 14 (`Shared Control`). No event, command or authored file carries an empty or absent name (29 scene, 80 actor, 7 adventure and 37 session occurrences). The longest id in any event is 18 bytes. Adventure scene-id fixtures hold 128 (`at-every-boundary`) and 200 (`scene-id-too-long`, refused). The YAML files are configuration, and `guide.md` is carried by path, so neither holds a name an event receives. |
| P3 | Base. Go and TS test data, by `grep` for `strings.Repeat`, `.repeat(` and names built from variables | `bigPaddingName = strings.Repeat("x", 28*1024)` is an `Actor.Name` in three `add_actor` commands (`TestAWedgedConnectionIsTornDownAndOthersKeepServing`, `TestAClientThatStopsReadingEntirelyIsTornDown`, `TestAForceClosedClientIsAnnouncedGone`). In `TestAJoinerDoesNotWaitForItsOwnArrivalToBeAnnounced` it is a `SceneCreated.Name` passed to `campaign.Append`. Every other generated name is an id: eventgen's, the soak's `soak-actor-N`, and `rules/conformance`'s. No TS test builds a long name. |
| P4 | Base. Each Done item's sketch | Engine, three refusals: red, `err = <nil>, want "engine: scene name must be at most 256 bytes, got 257"` and its two siblings. Engine at-cap: green. Wire: red, `257-byte name: ok=true error=""`. TS, three refusals: red, `Received function did not throw`; at-cap green. `mapdef.Load` with a 257-byte name: red, `err = <nil>`. `adventure.Load` with a 257-byte adventure, scene or actor name: red ×3. `mapdef.Load` with an empty or absent name: loads. |
| P5 | The Go fold's checks alone, then `go test ./internal/gateway/` | The four tests of P3 are red. `victim connection not closed — read timed out instead of observing close`. `a client that stopped reading entirely was never torn down …`. `a client force-closed by the write deadline was never announced gone …`. `engine: scene name must be at most 256 bytes, got 28672`. |
| P6 | P5 with the padding in `Actor.ModuleData`: a `structpb.Struct` holding `{"pad": bigPaddingName}`. The joiner's `SceneCreated` becomes an `ActorAdded` carrying it. | All four green. With `-count=15`, 60 runs, all green (44.5 s). |
| P7 | The whole probe tree | `go test -count=1 ./internal/... ./contract/... ./tools/... ./cmd/...`: 21 packages ok (`cmd/vtt` 166 s, scenario goldens included) in three of four runs; Gap 7 has the fourth. `bun test client/test contract contract-spike`: 877 pass, 0 fail (873 at base plus 4 probe cases). `golangci-lint` 0 issues; `go-arch-lint` OK; `semgrep scan --config .semgrep/ --error` exit 0; `client:typecheck` clean. `task build:client` changes `cmd/vtt/webdist/assets/index.js` only. `check-comments.py` clean over 264 files. `check-new-prose.py` all clean. `check-doc-owner.py`: 80 files. |
| P8 | P7 with a name required non-empty, 1 to 256 in both folds | 12 engine tests (`TestGrantIsIdempotent`, `TestTheGrantDeclaresTheActorsKind`, …) and 16 TS tests in `fold-unit`, `fold-dump` and `fold-rejections` go red. All of them add an actor with no name. The four re-vehicled tests, whose actors are nameless, go red too. |
| P9 | P7 plus `SessionStarted.name` in both folds (3 Go lines and 1 TS line) | `go test` over engine, campaign, eventgen, harness and mcp: green. bun: 877 pass. The Go keys land at 341, 452, 455 and 599, and every TS key one line below its names-only position. |
| P10 | Gremlins, the gate's own argv (`--workers 1 --timeout-coefficient 30 --output-statuses lctkvs`) | `internal/engine`: Killed 103, Lived 4, at `apply.go:338:58`, `449:15`, `452:30` and `596:41`. Those are the four adjudicated statements, moved +7, +10, +10, +10. The six new mutants (BOUNDARY and NEGATION on each check) are killed; mutator coverage is 100%. `internal/mapdef`: the new check's BOUNDARY lives without an at-cap test; with one: Killed 138, Lived 0, and 11 not covered, all in `compile.go` and `resolve.go`, none on an added line. `internal/adventure`: the three new BOUNDARY mutants live until `at-every-boundary`'s three names are 256 bytes; then Killed 95, Lived 1, the adjudicated `ARITHMETIC_BASE` at `load.go:124:58`, moved +1 by a constant placed in the const block. |
| P11 | Stryker on the edited regions of `fold.ts`, `--mutate client/src/fold.ts:<range>` | Layout (b), an `adventureLoaded` arm above an intact no-op group: 15 mutants, 11 killed, 4 survive. Three are existing entries re-pointed (`"attackRolled"`, `"abilityUsed"`, `default:`). One is new: `ConditionalExpression case "abilityUsed":`, which empties that clause so it falls through to `default:`. Layout (a), `default:` merged into the group: the tail has 6 mutants, 3 killed and 3 survive, exactly the three existing entries. Under both layouts, the old `case "adventureLoaded":` ConditionalExpression and its StringLiteral are KILLED. |
| P12 | Both mutation self-tests | At base: `OK`, `OK`. On the probe before re-pointing: the Go self-test flags all four `apply.go` keys. The TS self-test flags 10 of the 18 `fold.ts` keys. The other eight still read as valid at their old coordinates: `322:11`, `322:37`, `379:48`, `408:19`, `451:10`, `452:5`, `597:32` and `702:7`. After re-pointing by statement (D12): `OK`, `OK`. |
| P13 | `add_actor`'s `name` `fieldDocs` given the bound, then `task generate:contract` | `cmd/vtt/tools.json` and `contract/gen/tools/tools.json` change by one line each, and nothing else changes. `TestToolsMatchGolden` is red until `contract/testdata/expected_tools.json`'s line is edited by hand; then `tools/toolgen` is green. |
| P14 | `check-comments.py --report`, base → probe, with the ceiling | `apply.go` 42.7 → 42.1 / 42.8; `fold.ts` 43.0 → 42.8 / 43.1; `mapdef/load.go` 52.1 → 51.8 / 52.2; `adventure/load.go` 37.4 → 36.8 / 37.5; `server_internal_test.go` 36.8 → 36.6 / 36.9. None falls more than 1.0, but the probe did not cut D7's two comment blocks, which the real change must. |

**What breaks what.** Each break is one edit on the probe tree, restored
afterwards. All were run:

| # | Edit | Red |
|---|---|---|
| K1 | Go scene-name check disabled | engine scene refusal |
| K2 | Go actor-name check disabled | engine actor refusal, wire test |
| K3 | Go adventure-name check disabled | engine adventure refusal |
| K4 | Go scene check `>` → `>=` | engine at-cap test |
| K5 | `maxNameBytes` 300 | the three engine refusals, wire test |
| K6 | Go actor refusal reworded | engine actor refusal, wire test |
| K7 | TS scene `checkLen` removed | TS scene refusal |
| K8 | TS actor `checkLen` removed | TS actor refusal |
| K9 | TS `adventureLoaded` arm folded back into the no-op group | TS adventure refusal |
| K10 | TS actor bound 255 | TS actor refusal and at-cap |
| K11 | TS scene name counted in characters | TS scene refusal (its case is multibyte: `é` × 129) |
| K12 | mapdef check disabled | mapdef refusal |
| K13 | mapdef check `>=` | mapdef at-cap |
| K14 | the three adventure-loader checks disabled | adventure refusals |
| K15 | the three adventure-loader checks `>=` | `TestLoadAcceptsValuesExactlyOnEveryLimit` |

P5 is the sixteenth: the padding moved back to a name reds the four
re-vehicled tests.

**Records and gates at the base.**
- `check-requirements-chain.py .`: `273 rows, 221 test files, 13
  specifications`.
- `check-comments.py main`: `263 files, 0 added comment lines, 259 ledger
  rows; clean`, with the standing notice on `cmd/vtt/library_test.go`.
- `check-doc-owner.py .`: `80 files`.
- Both mutation self-tests: `OK`.
- `requirement-id` is at
  `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`.
- Free space on `/System/Volumes/Data`: 34,159,264 KiB at the start and
  32,910,884 KiB (31.4 GiB) after the clone was deleted.
- The Go cache grew from 7.2 to 8.4 GiB. Load averages ran from 1.0 to 5.9.

## Constraints that bind every task

- **Rule 2:** no gate is weakened. The ledger goes down only through
  `--write-ledger`. A key is re-pointed by statement, and removed only where
  its mutant is now killed (P11).
- **Rule 3:** no proto changes; `check:breaking` reports nothing.
- **Rule 4:** both folds refuse the same events at the same counts, in the same
  order within each arm (D4). A TS fold stricter than Go's freezes a client
  (`client/src/wire.ts`), so neither side lands alone.
- **Rule 5:** no new vocabulary. "Scene", "actor", "adventure", "session" and
  "name" are platform words, and semgrep passed on the probe.
- **Rule 8:** cite names, never lines. The adjudication files keep their
  coordinates. **Rule 9:** answered above. **Rule 10 and SPEC-010:** the only
  new comments are test citation lines, one-line pointers on the two loader
  constants, D5's two-line merged comment, and D7's rewritten blocks of six
  lines or fewer. No banned term appears on an added line.
- **`check:comments`' block rule:** a block of more than six lines that gains a
  line is refused. Any edit inside `bigPaddingName`'s doc,
  `TestAWedgedConnectionIsTornDownAndOthersKeepServing`'s payload block,
  `internal/adventure/load.go`'s const-block header or
  `TestSizeCapsMirrorEngine`'s doc therefore needs the whole block cut to six
  lines or fewer. D6 and D7 are shaped by this.
- **SPEC-008:** ids come from `requirement-id`, after sign-off, A then B then C.
- **Order and hygiene.** `check:drift` passes only on a committed tree, so the
  order is review, commit, gate. The review package is `git diff HEAD`, and
  nothing is stashed or checked out while a reviewer reads. `git add` and `git
  commit` run in separate calls, with `git show --stat HEAD` after each. A new
  test goes after a closing brace. The pre-push hook takes about three
  minutes; let it finish.
- The ticket and every report under `docs/reports/` are not edited.

## Decisions this plan makes

**D1. The bound: 256 bytes of UTF-8, inclusive, one constant (Q1).**
`maxNameBytes = 256` in `engine.Apply`'s const block bounds every name, as
`maxTextBytes` bounds two texts. The bound is forced by the label precedent:
256 is the fold's bound for every one-line label (`maxNoteTitleBytes`,
`maxNarrationAsBytes`, `maxMoveReasonBytes`). A name is a label shown on one
line (`spectator.ts`'s `describe`). 128 is the codebase's bound for an
identifier a human types (`maxNoteKeyBytes`, `maxIDBytes`), and a name is not
an identifier. The longest name any event carries today is 19 bytes (P2).
Inclusive, because every cap in `Apply` is written `> max`.

**D2. An empty name stays allowed in both folds (Q2).** Three things force it:
- P8: requiring a name reds 28 existing tests that add an actor with no name.
- The label precedent: a title, a speaker and a reason may each be empty.
- The ticket states no non-empty rule.

The loaders keep what they do today. `internal/adventure/load.go` refuses an
empty adventure, scene or actor name ("must not be empty"); no test pins any
of the three (Gap 6, Q7). `mapdef.Load` accepts an empty or absent name
(P4), and no shipped map has one (P2).

**D3. Scope: every `name` field an event carries, which is the three the
ticket names plus `SessionStarted.name` (Q3).** Measured by `grep` over
`events.proto`, an event carries exactly four `name` fields. `SessionStarted`'s
is the most exposed of them: `classify` forwards it to every seat, players and
spectators included, and the feed prints it (`session started — <name>`).
`AdventureLoaded` is withheld from every projected seat. Adding it costs three
Go lines, one TS line, a wire test and no loader, and breaks no test (P9).

**Ids are left to a ticket of their own.** Each of these is measured:
- Their natural bound is 128, a second constant.
- The folds disagree with themselves on empty ids today: an empty `actor_id`
  is refused, but an empty `scene_id` folds (P1).
- A map's id is its filename, bounded only by the filesystem at about 250
  bytes. A fold bound of 128 would make maps installable at boot and refused
  at `load_map`, which is a ruling on SPEC-014's format, not on names.

The ticket's writer adds `SessionStarted.name` to its rules if Q3 is (b), as
the reason-bound ticket was revised after its Q4.

**D4. The Go fold.**
- `maxNameBytes = 256` goes last in the const block. The block's two-line
  header ("Inclusive byte bounds on an event's free text (SPEC-018). …")
  stays true.
- The four checks, each `if len(<name>) > maxNameBytes { return
  fmt.Errorf("engine: <field> must be at most %d bytes, got %d", maxNameBytes,
  len(<name>)) }`, in the form of `move reason must be at most`. The field
  words are `scene name`, `actor name`, `adventure name` and `session name`.
- **`SessionStarted`:** after the open-session refusal.
- **`SceneCreated`:** after the duplicate refusal, before the terrain is
  translated.
- **`ActorAdded`:** after the controller refusal, before `proto.Clone`. The
  bound comes last in each arm, as `TokenMoved`'s does. A caller wrong in a
  controller and a name is therefore told the controller, the
  misunderstanding of the model, as `validateAddActor`'s own order puts it.
- **`AdventureLoaded`:** the check, then the existing `return nil //
  testimony, …`.

`Apply`'s doc still calls `AdventureLoaded` a no-op, in the sense it already
uses for `NarrationAdded`: it validates and does not mutate. That stays true.

**D5. The TS mirror, layout (a) (Q6).**
- `checkLen("session name", p.value.name, 0, 256);` goes after
  `sessionStarted`'s open-session throw.
- `checkLen("scene name", v.name, 0, 256);` goes after the duplicate-scene
  throw.
- `checkLen("actor name", a.name, 0, 256);` goes after the controller throw.
- `adventureLoaded` leaves the no-op group for an arm of its own:
  `case "adventureLoaded": checkLen("adventure name", p.value.name, 0, 256);
  return;`, placed above the group.
- `default:` joins the group. Its two comments merge into one two-line comment
  above `case "attackRolled":`: `// Recorded on the log, no effect on derived
  state; an unknown variant is` / `// skipped, as the server's own replay skips
  it.` That is true: `campaign`'s rebuild skips `engine.ErrUnknownVariant`.

Layout (a) is forced by P11: it leaves exactly the three existing equivalents
alive and adds none. The literals mirror the constant, as every bound's do
(Gap 10).

**D6. The loaders.**
- **`mapdef`:** `loadAs` checks `len(raw.Name) > maxNameBytes` after `cell_px`
  and before the grid, a property of the whole map as `cell_px` is. It refuses
  with `fieldErr(display, "name", fmt.Sprintf("must be at most %d bytes, got
  %d", …))`, which reads `mapdef: maps/<id>.json: field "name": must be at
  most 256 bytes, got 257`. `const maxNameBytes = 256` sits beside `loadAs`
  with one line: `// maxNameBytes mirrors internal/engine's bound on a name
  (SPEC-018).` Boot (`loadMapsDir`) and `mapByID` both reach it through
  `LoadInstalled`. `mapdef` may not import `engine` (`.go-arch-lint.yml`).
- **`adventure`:** after each of the three `"must not be empty"` name checks
  (in `loadManifest`, `loadActors` and `loadScenes`), the same check with
  `fieldErr(path, "name", …)`. `const maxNameBytes = 256` goes below the
  adjudicated statement, immediately before `type manifestJSON struct`, with
  the same one-line pointer. That moves no key (P10 measured the alternative:
  +1). It also leaves the const block's ten-line header true, since that
  header describes the three constants inside the block.
- **The adventure pin** is a new `TestTheNameBoundMirrorsEngine` in
  `internal/adventure/format_test.go`, `package adventure`, one-line doc,
  asserting `maxNameBytes == 256`. It is new rather than a line added to
  `TestSizeCapsMirrorEngine`, because that test's eleven-line doc names three
  constants and would become false.

**D7. The gateway.** No production change: the fold refuses, and VTT-161 and
VTT-162 carry the refusal to the issuer (P4's wire probe). The four tests of
P3 move their padding:
- A helper `bigPadding(t *testing.T) *structpb.Struct` wraps `bigPaddingName`
  as `{"pad": bigPaddingName}`.
- The three `add_actor` sites set `ModuleData: bigPadding(t)` in place of
  `Name: bigPaddingName`.
- The joiner's `SceneCreated` becomes an `ActorAdded{Actor: {ActorId:
  "act-%d", ModuleData: bigPadding(t)}}`.

`module_data` is the contract's opaque blob, carried whole by `ToEvent`,
`proto.Clone` and the broadcast (P6). Two comment blocks become false, and
each is cut to six lines or fewer, as a warning or a pointer:
- `bigPaddingName`'s doc. It says the string lands in `Actor.Name` and
  `SceneCreated.Name`, "carrying an unbounded string straight through".
- `TestAWedgedConnectionIsTornDownAndOthersKeepServing`'s "THE PAYLOAD IS
  DELIBERATELY MINIMAL BESIDE THE NAME" block.

The other mentions ("bigPaddingName (28KB)", "bigPaddingName-sized
broadcasts", `internal/harness/client.go`'s quotation) stay true, because the
variable stays a 28 KiB string. The history the blocks carry goes to the
report.

**D8. The tests, each red today except the at-cap ones (P4).**
- `internal/engine/apply_test.go`, at the end:
  `TestASceneWhoseNameExceedsTheBoundIsRefused`,
  `TestAnActorWhoseNameExceedsTheBoundIsRefused`,
  `TestAnAdventureWhoseNameExceedsTheBoundIsRefused` and
  `TestASessionWhoseNameExceedsTheBoundIsRefused`. Each sends 257 bytes,
  asserts the exact error, and asserts an unchanged `Snapshot`.
- `apply_boundary_test.go`, at the end: `TestNamesAtCapAreAccepted`, with all
  four events at 256 bytes. It asserts the stored scene and actor names.
- `internal/gateway/server_test.go`, after
  `TestAMoveWhoseReasonExceedsTheBoundAppendsNothing`'s closing brace:
  `TestAnActorWhoseNameExceedsTheBoundAppendsNothing`. A 257-byte name gets
  ok=false with the exact text, and `f.head` is unchanged. Then a 256-byte
  name answers ok at `head+1`, and the agent receives it whole.
- `TestASessionWhoseNameExceedsTheBoundAppendsNothing` follows. It first sends
  `end_session`, because `newGWFixture`'s seed opens a session and the
  open-session refusal comes first (D4).
- Both gateway tests cite A, VTT-161 and VTT-162.
- `client/test/fold-rejections.test.ts`, at the end, in a "names" section:
  - a refusal per field, the scene's multibyte (`é` × 129);
  - an at-cap case per field (`é` × 128 for the scene).
- `internal/mapdef`:
  - `testdata/invalid/name-too-long/map.json`, a 257-byte name;
  - a row `{"name-too-long", "at most 256 bytes, got 257"}` in
    `TestInvalidMapsAreRefusedWithAUsefulReason`;
  - `TestAMapNameOfExactlyTheBoundLoads` in `load_test.go`, in the form of
    `TestTheBoundsThemselvesAreAccepted`.
- `internal/adventure`:
  - `testdata/invalid/adventure-name-too-long`, `scene-name-too-long` and
    `actor-name-too-long`, each a copy of `scene-id-too-long`'s shape with one
    257-byte name;
  - three rows in `TestLoadInvalidFixtures` naming the file, `field "name"`
    and `at most 256 bytes, got 257`;
  - the three names added to the catalogue list;
  - `at-every-boundary`'s adventure, scene (`pin.json`) and actor
    (`edge-actor.json`) names at exactly 256 bytes. Three assertions in
    `TestLoadAcceptsValuesExactlyOnEveryLimit` lock them at 256, as the
    scene-id assertion locks 128. Its doc's invariant ("every BYTE CAP load.go
    checks has a value here sitting exactly on it") is what requires this.

The goldens need no case: no stream holds a name within 237 bytes of the
bound.

**D9. The tool descriptions state the bound (Q5).** `manifest`'s `add_actor`
`name` `fieldDocs` becomes `Optional display label for the actor. At most 256
bytes of UTF-8; a longer name refuses the whole command.` (P13).
`start_session` gains `overrides: {"vtt.v1.StartSession": {fieldDocs:
{"name": "The session's title, shown to everyone at the table. At most 256
bytes of UTF-8; a longer name refuses the command."}}}`, with
`requiredOverride` left nil. The schema derives `required: ["name"]` today,
so the text must not call the name optional.

Neither carries `maxLength`, because JSON Schema counts characters.
`TestAddActorAndStartSessionStateTheNameBound` in `tools/toolgen/main_test.go`
asserts both. `TestTheToolsStateTheFoldsNameBound` in a new
`internal/engine/name_bound_internal_test.go` (`package engine`) reads
`../../contract/gen/tools/tools.json`. It requires `fmt.Sprintf("At most %d
bytes", maxNameBytes)` in `add_actor`'s `actor.name` (a nested property) and
in `start_session`'s `name`. That is the link
`TestTheMoveToolStatesTheFoldsReasonBound` already makes. `task
generate:contract` regenerates both `tools.json` files, and
`expected_tools.json` gains its lines by hand.

**D10. The records.**

**SPEC-018**, written with the `specification` skill against the final tree:
- **Status:** add `maxNameBytes`; the `SessionStarted`, `SceneCreated`,
  `ActorAdded` and `AdventureLoaded` arms and their TS mirrors; the two
  loaders' `maxNameBytes`; the tool statements; and the new test files.
- **"Six fields are bounded"** becomes "Ten fields are bounded".
- **The table** gains `SessionStarted.name`, `SceneCreated.name`,
  `ActorAdded`'s `Actor.name` and `AdventureLoaded.name`, each 256
  (`maxNameBytes`), each "yes".
- **The order sentence** gains: "A `SceneCreated` is checked for a duplicate,
  then its name; an `ActorAdded` for an actor with an id, a duplicate and a
  declared controller, then its name; a `SessionStarted` for an open session,
  then its name."
- **"These six are the only texts `apply.go` bounds above"** becomes "These
  ten". The map-and-adventure clause, "every scene's name and an adventure's
  actor names among them, is bounded by nothing, except an adventure's scene
  id …", becomes "one a map or adventure file carries has no bound in the
  fold, except that an adventure's scene id is bounded at 128 bytes by its
  loader (`maxIDBytes` in `internal/adventure/load.go`)". It claims nothing
  about art ids, which artlib's `isArtID` bounds.
- **"for each of the six fields"** becomes "ten".
- **"Other mirrors"** gains the two loaders' `maxNameBytes`, pinned by
  `TestTheNameBoundMirrorsEngine` and by `mapdef`'s refusal and at-cap tests.
  "An adventure over one of them is refused at load" becomes "A map or an
  adventure over one of them …". The tool paragraph names `add_actor` and
  `start_session`.
- **Consequences**, second bullet: add "`internal/mapdef`'s copy" and "for a
  name, the `add_actor` and `start_session` descriptions".
- **Requirements:** add A, B and C.

Phase 4b verifies every replaced sentence by command.

**The register:** VTT-161 and VTT-162 gain the two wire tests. VTT-260 is
unchanged: `ToEvent` still carries the actor whole.

**`docs/verification-debt.md`:**
- The open entry "Nothing ties a byte bound's copies to the engine's constant"
  is amended to ten bounds, four adventure copies and `mapdef`'s copy. Its
  "only a move's reason has a link" becomes "a move's reason and a name". An
  open entry is a claim on future work, not a report, and a false count in it
  misleads.
- Per Q7(i), a new entry: the adventure loader's three empty-name refusals are
  pinned by no test. The recipe is to delete `raw.Name == ""`'s branch in
  `loadManifest`; `go test ./internal/adventure/...` stays green. Label: `test
  data missing`.

**D11. The sort.** Forced by SPEC-008. The rows are lettered so nothing reads
as an id.

| # | Rule | Proposed | Red when | Tests | Record |
|---|---|---|---|---|---|
| A | Both folds refuse a `SessionStarted`, `SceneCreated`, `ActorAdded` or `AdventureLoaded` whose name is longer than 256 bytes of UTF-8, and accept one of exactly 256. | accept: the ticket's first rule, and the refusal half of its third | K1 to K11 | D8's engine, TS and wire tests | SPEC-018 |
| B | A map file or an adventure that carries a name longer than 256 bytes of UTF-8 is refused when it loads, by an error naming the file and the field. | accept: the second rule | K12 to K15 | D8's loader tests and fixtures | SPEC-018 |
| C | The `add_actor` and `start_session` tools state the bound the fold enforces on a name. | accept, per Q5 | the description changed and regenerated; `maxNameBytes` changed | D9's two tests | SPEC-018 |

**Refused, one line each:**
1. "A name is at most 256 bytes" on its own: A holds it.
2. "A refused `add_actor` or `start_session` appends nothing": VTT-162, which
   gains the wire tests.
3. "It is answered ok=false on an open connection": VTT-161, likewise.
4. "`fold.ts` refuses what `engine.Apply` refuses" as a parity row: A names
   both folds.
5. "An empty name is allowed": today's behaviour, and no rule states it.
6. "The refusal names the bound": how A and B are observed, by exact text.
7. "A loader's copy equals the engine's constant": a consistency of copies,
   which is SPEC-018's consequence and the debt entry.
8. "A projected introduction carries a name within the bound": it follows
   from A, because introductions copy folded state.
9. "The oversized-broadcast tests still oversize": test apparatus, held by
   their own wedge witnesses (P5, P6).
10. "The bound is checked last in its arm": SPEC-018's order text, observed
    by exact messages only when two faults coexist.

Three rows accepted, ten refused, none withdrawn. A is one row, not one per
event: one constant and one check shape, and splitting it gives four rows
that always move together (Q8).

**D12. Mutation keys: re-point last, by statement.** These apply after the
review settles and before the commit, with Q3(b) and Q6(a). P9 and P11 give
the positions; re-read each at its statement.
- **`apply.go`**, four keys:
  - `id < standing` (`ActorRemoved`): +10, from `331:58` to `341:58`;
  - `computed < 0`: +13, from `439:15` to `452:15`;
  - `computed > int64(res.Max)`: +13, from `442:30` to `455:30`;
  - `len(objs) > 0`: +13, from `586:41` to `599:41`.

  The shift is the constant (+1), the session check (+3), the scene check
  (+3), the actor check (+3) and, below `ActorRemoved`, the adventure check
  (+3).
- **`internal/adventure/load.go`:** none moves, under D6's placement.
- **`fold.ts`**, from the base, with Q3(b) adding one line above every key:
  - `sceneSeen`'s four, `conditionRemoved`'s `?? []` (the file's second
    `st.Conditions[v.actorId] ?? []`), `resourceChanged`'s three, and
    `ensureOpenDoors`, `headSequence`, `foldToDumpJSON` and `actorJSON`'s
    five: +3;
  - `"attackRolled"` and `"abilityUsed"`: +7;
  - `default:`: +5.

  That is sixteen re-pointed.
- **Two entries are deleted:** `ConditionalExpression case "adventureLoaded":`
  and the `adventureLoaded` StringLiteral. Their mutants are killed (P11), and
  a key that matches no live mutant fails `check:ts-mutation`.
- The `default:` entry's reason says its body is "shared with the three
  no-effect event kinds above it". That becomes "two", because the reason is a
  record and must stay true.

Then run `python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q`. The TS self-test cannot see eight of the
eighteen old keys (P12), so its `OK` is necessary, not sufficient; Phase 4b
re-reads every key. If a review adds a line above an edit point, re-measure
every key. The new mutants are killed as follows:
- Go `>=`: by `TestNamesAtCapAreAccepted`.
- Go negation: by the refusal tests.
- The TS labels emptied: by the exact messages.
- The `adventureLoaded` clause emptied or renamed: by the TS adventure
  refusal.
- The loaders' `>=`: by the at-cap fixtures (P10).

**D13. Comments and the ledger.** Tests carry their `// VTT-NNN` line only.
The helper `bigPadding` and the new internal test file carry no doc beyond
one line. `fieldDocs` is prose under `tools/`, outside `check:comments`. D7's
two cuts lower `server_internal_test.go`'s share below P14's probe figure,
likely by more than 1.0. `--write-ledger` runs once, before C1, and its output
is read row by row.

**D14. One code commit and the report (Q9).** Forced by measurement:
- The Go fold alone reds the four gateway tests (P5), so D7 lands with it.
- The TS fold alone risks a frozen client.
- A description stating a bound no fold enforces would be false.

**C1** holds the code, the tests, the fixtures, the regenerated files, the
webdist, SPEC-018, the rows, the debt edits, the keys and the ledger. **C2**
holds the report.

**D15. Phase 4a for C1.** One QA agent per `qa-prompt.md`, never given the
diff, the source, the existing tests or the implementer's report. It is given:
- rows A, B and C, VTT-161 and VTT-162;
- SPEC-018 whole;
- SPEC-013's command-path and validators paragraphs, and SPEC-014's "What
  reaches a client from a refusal";
- the text of `SessionStarted`, `SceneCreated`, `Actor`, `ActorAdded`,
  `AdventureLoaded`, `AddActor` and `StartSession`;
- `go doc -all` of `internal/engine`, `internal/gateway`, `internal/campaign`,
  `internal/identity`, `internal/mapdef` and `internal/adventure`;
- `grep -n '^export' client/src/fold.ts`;
- the `add_actor` and `start_session` entries of `tools.json`.

It writes `internal/gateway/qa_name_bound_test.go`,
`internal/mapdef/qa_name_bound_test.go`,
`internal/adventure/qa_name_bound_test.go` and
`client/test/qa-name-bound.test.ts`. Adjudications go in the report, one line
per finding. An escape goes to the debt file as a recipe. A QA test that pins
an ABSENCE no rule states (an id or a `module_data` left unbounded) is
dropped, as the reason arc's were.

**D16. Phase 4b for C1, after 4a.** One reviewer at high effort, briefed to:
- verify by command every sentence C1 adds or rewrites (SPEC-018, D7's two
  blocks, D5's comment, the debt edits and the descriptions);
- re-read every re-pointed key at its statement;
- check the break lines in the draft message.

If the reviewer dies on a model limit, say so and re-dispatch the same brief
on `fable`.

**D17. The breaks, one per check relied on.** K1 to K15 and P5's, in a
scratch clone of C1's final tree, plus K16 (`start_session`'s description
says 200, regenerated) and K17 (the session check disabled). Each gate is
clean first, each break is one edit, and each is undone by hand with the
inverse edit.

**D18. Disk and load.** Forced by `check:mutation`'s 16 GiB floor and by
MCP e2e deadlines under load. Immediately before `task check`, run `go clean
-cache`, `df -k` and `uptime`. Launch the gate once after C1, in its own
session (`start_new_session=True`). Below 16 GiB, stop.

## Tasks, in dependency order

### Task 0 — Baselines

**Done when:** `df -k`, `uptime`, the chain, comments and doc-owner baselines
and both self-tests match Measurements, and `requirement-id` resolves.

### Task 1 — Rows

**Files:** `docs/requirements.md`. After sign-off, run `requirement-id` once
each for A, B and C, in that order. **Done when:** there are three new OPEN
rows.

### Task 2 — The Go fold

**Files:** `internal/engine/apply.go`, `apply_test.go`,
`apply_boundary_test.go`.

D8's engine tests come first (the refusals are red), then D4. **Done when:**
`go test -count=1 ./internal/engine/` is green. The gateway package is red
until Task 3; that is expected.

### Task 3 — The gateway

**Files:** `internal/gateway/server_test.go`,
`internal/gateway/server_internal_test.go`.

D8's two wire tests are red on the base and green on Task 2's tree. Then D7:
the helper, the four sites, and the two blocks cut. **Done when:** `go test
-count=1 ./internal/gateway/` is green, and `go test -count=5 -run
'<the four>' ./internal/gateway/` is green.

### Task 4 — The TS mirror

**Files:** `client/src/fold.ts`, `client/test/fold-rejections.test.ts`,
`cmd/vtt/webdist/`.

The cases come first, then D5, then `task build:client`. **Done when:** `bun
test client/test` is green, `client:typecheck` is clean, and `git status`
shows `index.js` as the only webdist change.

### Task 5 — The loaders

**Files:** `internal/mapdef/load.go`, `load_test.go`,
`testdata/invalid/name-too-long/`; `internal/adventure/load.go`,
`load_test.go`, `format_test.go`, the three `testdata/invalid/*-name-too-long/`
directories, and `testdata/at-every-boundary/{adventure.json,scenes/pin.json,actors/edge-actor.json}`.

The fixtures and rows come first (red), then D6. **Done when:** `go test
-count=1 ./internal/mapdef/ ./internal/adventure/...` is green and `go test
-count=1 ./cmd/vtt/` is green (shipped content boots).

### Task 6 — The tools

**Files:** `tools/toolgen/main.go`, `main_test.go`, both `tools.json`,
`contract/testdata/expected_tools.json`,
`internal/engine/name_bound_internal_test.go`.

D9's toolgen test is red first, then `fieldDocs`, `task generate:contract`, the
golden lines and the link test. **Done when:** `go test ./tools/toolgen/
./internal/engine/` is green and `git status` shows exactly the two
`tools.json` files regenerated.

### Task 7 — The records

**Files:** SPEC-018, `docs/requirements.md` (A, B and C evidence; VTT-161 and
VTT-162), `docs/verification-debt.md`.

D10. **Done when:** the chain prints `276 rows`, `13 specifications`, with A,
B and C not OPEN. `grep -c 'bounded by nothing' docs/specifications/018-*`
prints 0, and `grep -c 'six' docs/specifications/018-*` prints 0.

### Task 8 — Local gates

Run:
- `gofmt -l` over the touched files only (Gap 8);
- `go vet` and `task lint`;
- `go test -count=1 ./internal/... ./contract/... ./tools/... ./cmd/...`;
- `bun test client/test contract contract-spike`;
- semgrep, `go-arch-lint`, `check:comments`, `check:doc-owner` and
  `check:new-prose`.

**Done when:** each prints its own completion line. A Go failure is captured
whole and re-run in isolation before it is called a flake (Gap 7).

### Task 9 — Phase 4a, then 4b

D15 and D16. Findings are fixed, and the affected task's "done" is re-run. The
review settles before Task 10.

### Task 10 — Keys, ledger, commit C1

D12 and D13, then C1's message, which lists the ids and D17's lines. **Done
when:**
- both self-tests print `OK`, and every key reads its statement;
- `git show --stat HEAD` lists C1's files;
- `task check:drift` is clean.

### Task 11 — Breaks and the whole gate

D17, then D18, then `task check` once. **Done when:** each break gives its
red, and `task check` exits 0 with every step's own verdict,
`check:mutation` and `check:ts-mutation` included.

### Task 12 — The report

`docs/reports/2026-10-05-a-name-is-bounded.md`, per the
`implementation-report` skill. It covers:
- each Done item with its observation;
- the rows and refusals;
- the rulings taken at sign-off;
- the rule-9 answer;
- the breaks;
- the history D7 and D13 removed from comments;
- the gaps.

**Done when:** C2 holds it alone. Push after C2, and let the pre-push hook
finish.

## Gaps that travel with this plan

1. **"What it touches" omits the four gateway tests that carry a 28,672-byte
   name (P3, P5), and the two comment blocks describing it.** Done item 5 is
   unreachable without D7. The ticket's "the gateway test needs only the
   engine" holds for the new wire test only.
2. **"The longest name in any JSON file in the repository is 30 bytes" is
   true but measures the wrong set.** The 30-byte value names a ruleset golden
   case, not an event. The longest name that reaches an event is 19 bytes.
   The decisive data sits outside JSON: Go test data holds a 28,672-byte name
   (P2, P3).
3. **The mutation work is more than keys that move.** In `fold.ts` two
   adjudicated mutants become killed and their entries go, and layout (b)
   would add an entry (P11). `internal/adventure/load.go` carries a key the
   ticket does not name; D6 places the constant so that it does not move.
4. **The loaders' tests need fixtures the ticket does not name:**
   `at-every-boundary`'s three names at 256, three `testdata/invalid`
   directories, and a `mapdef` fixture and at-cap test. Without them
   gremlins' boundary mutants live (P10).
5. **The "accepts exactly the bound" halves of Done items 1 and 2 cannot fail
   today.** K4 and K10 hold them.
6. **The adventure loader's three empty-name refusals are pinned by no test.**
   No fixture has an empty name, and no test asserts `field "name"`'s "must
   not be empty". A deleted branch stays green (D10, Q7).
7. **One Go failure in one of four whole-suite runs on the final probe tree
   was not captured.** Three further runs of the suite were green, and so
   were sixty runs of the four re-vehicled tests. Which test failed is
   unknown.
8. **`gofmt -l` lists `internal/gateway/scenario_test.go` on `main`**
   (`0b9d6b7`). `.golangci.yml` runs no formatter. This is not this ticket's
   to fix, and Task 8 formats only what it touches.
9. **A campaign directory whose map or adventure holds a name over 256 bytes
   stops booting.** `loadMapsDir` and `loadAdventuresDir` fail loudly. A log
   holding such an event stops opening. No shipped content, fixture or
   eventgen draw is within 237 bytes of the bound (P2), and nobody uses the
   product (the owner, 2026-09-04).
10. **Nothing links `fold.ts`'s literals or the two loaders' copies to
    `maxNameBytes`** beyond each side's own tests. The open debt entry is
    amended rather than closed (D10).
11. **The problem paragraph's "every projected viewer is sent it" holds for
    scene and actor names**, and only for viewers to whom the scene or actor
    is introduced. `AdventureLoaded` reaches no projected seat (`classify`
    withholds it). `SessionStarted`, which the ticket leaves open, reaches
    every seat (D3).

## Questions for sign-off

1. **Is the bound 256 bytes of UTF-8, inclusive, one `maxNameBytes` for every
   name?** Recommend yes (D1). It is the fold's bound for a one-line label.
   128 is its bound for an identifier, and the longest name today is 19
   bytes.
2. **Does an empty name stay allowed in both folds, with the loaders
   unchanged on emptiness?** Recommend yes (D2). Requiring a name reds 28
   tests and states a rule the ticket does not.
3. **Scope:** (a) the three names the ticket lists; (b) those and
   `SessionStarted.name`; (c) (b) and the four ids at 128. Recommend (b).
   It is every `name` an event carries, and the session's is forwarded to
   every seat. It costs four lines and a wire test, and breaks nothing (P9).
   Ids get a ticket of their own: a second bound, an empty-id ruling, and a
   map-format ruling (D3). Under (b) the ticket's writer adds the session to
   its rules.
4. **The four oversized-broadcast tests:** move their padding into
   `Actor.module_data` and cut the two false comment blocks to six lines or
   fewer? Recommend yes (D7, P6). The opaque blob is the one field no rule
   reads. `module_id` would move again if ids are ever bounded.
5. **Do `add_actor`'s and `start_session`'s descriptions state the bound, held
   to `maxNameBytes` by an internal engine test reading `tools.json`?**
   Recommend yes (D9), as VTT-265 does for a move's reason. The agent is the
   main issuer of `add_actor`, and learns the bound before a refusal rather
   than after one.
6. **`fold.ts`'s no-op group:** (a) merge `default:` into it, which adds no
   adjudication and deletes two; (b) keep `default:` apart, which deletes two
   and adds `ConditionalExpression case "abilityUsed":`. Recommend (a)
   (P11). Every arm involved returns, and it leaves the equivalents file
   smaller with no new excuse in it.
7. **The adventure loader's unpinned empty-name refusals:** (i) a debt entry
   in C1, or (ii) three `*-name-empty` fixtures and an existing-behaviour row
   in C1? Recommend (i). Nothing breaks at the table if they go, and the
   ticket's rules are about length. (ii) costs three fixture directories and
   three table rows if the owner wants it.
8. **Rows A, B and C, with A one row for every name rather than one per
   event, and VTT-161 and VTT-162 gaining the wire tests?** Recommend yes
   (D11).
9. **One code commit and the report (D14)?** Recommend yes. The Go fold
   cannot land without D7, and the TS fold cannot land first.
