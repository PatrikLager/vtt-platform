# A command's and a map's ids are bounded — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-07-a-commands-and-a-maps-ids-are-bounded-design.md`
**Verified:** 2026-10-07 by `verify-ticket` (dev-cycle 0.4.0). The verifier is
an agent that did not write the ticket. It verified against `9760d23` on
`feat/command-and-file-ids-are-bounded`, which equals `main`, with the ticket
untracked. Verdict: **Passes with gaps.** The gaps are listed at the end and
travel with this plan. This plan does not edit the ticket.

**Goal, in the ticket's words:** both folds refuse a control event whose
participant id, an `ActorAdded` whose actor's `module_id` or one of whose
`resources` or `attributes` keys, a `SceneCreated` one of whose objects' ids,
a `SessionStarted` whose envelope's session id, and a `TokenMoved` whose scene
id is longer than the bound, and accept each at exactly the bound. A map file
carrying an object id over the bound is refused when it loads, naming the file
and the field. A `grant_actor_control`, `revoke_actor_control` or `add_actor`
carrying such a string is answered ok=false and appends nothing. Every golden,
fixture and shipped content file still loads and folds, and `task check` whole
is green.

**Where this meets the ruleset ticket.** The owner split the remaining
id-like strings in two by source; the ruleset-sourced half landed as
`53acb19`. Its plan's D7 asks this ticket to bound an actor's keys at
`maxIDBytes`, because a declared name is the only key the adventure loader
admits. Checked against the tree, by reading and by command, and it holds:
- `loadActors` (`internal/adventure/load.go`) refuses every attribute key not
  in `attrOrDefSet` and every resource key not in `resSet`, and `Load` builds
  both sets from the ruleset's `Attributes`, `Defenses` and `Resources` alone.
  `adventure.Compile` copies an actor's keys from the loaded actor and adds
  none.
- `rules.Load` holds those names at 128 bytes (VTT-283).
- `internal/rules/qa_ruleset_bound_test.go`'s
  `TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts` already folds an
  actor holding 128-byte resource, attribute and defense names through
  `engine.Apply` (P3). With the fold's key bound at `maxIDBytes`, that test
  goes red when a key check refuses 128 bytes (`>` → `>=`, K7 and K9) or the
  constant falls to 127 (K21). So the two bounds are equal by construction, and a test
  already notices one falling below the other.

A key bound below 128 would let a ruleset declare a name no actor can carry:
the adventure would load and `load_adventure` would be refused by the fold.
`add_actor` writes keys that nothing compares with a ruleset, and only the fold
can bound those.

**MapTool (CLAUDE.md rule 9): an owner and a property are free text of any
length, and an object is a generated id of one length.** Read in
`~/dev/RPTool/maptool` at `f4b7fef6c`:

- **Who controls a token.** A token's owners are player NAMES, held in
  `Token.ownerList` and added by `Token.addOwner(String)`. The UI adds a
  connected player's name (`TokenPopupMenu`'s `PlayerOwnershipMenu`,
  `EditTokenDialog`). The `setOwner` macro (`TokenPropertyFunctions`) adds any
  string it is given, checked against neither the player list nor a length. A
  player's name is checked only for emptiness at connect: `ConnectToServerDialog`
  refuses `username.length() == 0`.
- **Module-like and key-like strings.** A token's property type, the nearest
  thing to `module_id`, is stored by the `setPropertyType` macro unchecked. Token
  property names are stored at any length; the ruleset plan read `setProperty`
  for that.
- **Map objects.** Every drawable carries a generated id. `AbstractDrawing`'s
  constructor draws `new GUID()`, and its copy constructor draws a fresh one.
  A drawable read off the wire goes through `GUID.valueOf(dto.getId())`
  (`DrawnLabel`, `DrawablesGroup`, the templates), and `validateGUID` refuses
  any length but `GUID_LENGTH = 16`. Object ids are therefore unique because
  they are generated, and length-checked where they are read.
- **Sessions.** MapTool has no session id.
- **Frames.** `AbstractConnection.readMessage` reads a 4-byte length prefix,
  so a frame carries any length.

**Borrowed:** an object's id has one bound, checked where it is read, and is
unique within its map. MapTool gets uniqueness by generation; here a map's
author writes the id, so the loader must check uniqueness instead. **Not
borrowed:** owners, property types and property names of any length. MapTool
can afford none, since its frames carry any length. Here the server reads at
most `maxWSFrameBytes` per frame, and the log keeps forever what the fold
accepts. **Checked and rejected:**
- Generating object ids at compile time. A map's object id is authored, and
  `internal/adventure/conformance`'s compiled-batch golden carries
  `pillar-1` and `rug-1`, so generating ids would change every compiled scene.
- Checking an owner against the participant list. MapTool's `setOwner` does
  not, and Q6 says why it does not fit here either.

## Verification, check by check

1. **Every path resolves (by command).** `[ -e ]` finds all 21 paths the
   ticket names: 15 files and 6 directories. `grep` finds each named symbol
   where the ticket puts it:
   - `func Apply`, `func controlTarget`, `func mergeObjects` and `maxIDBytes =
     128` in `internal/engine/apply.go`;
   - `introduce` in `internal/gateway/project.go`;
   - `func ToEvent` in `convert.go`, `func validateGrantActorControl` in
     `grant_validate.go`, `maxWSFrameBytes` in `server.go` and SPEC-011;
   - `func loadActors` in `internal/adventure/load.go`, `func Load` in
     `internal/rules/load.go` and in `internal/mapdef/load.go`, `func Compile`
     in `internal/mapdef/compile.go`;
   - `func stampSessionIDAgainst` and `"sess-" + hex.EncodeToString` in
     `internal/campaign/campaign.go`;
   - the VTT-162, VTT-276, VTT-281 and VTT-283 rows, and the debt entry
     "Nothing ties a byte bound's copies to the engine's constant".

   The problem paragraph's claims hold, with one omission and one falsehood
   (check 4):
   - The paragraph says "A `TokenMoved`'s `scene_id` is never set by
     `ToEvent`". That is true, but it leaves out the writer that does set it.
     `handleCommand` backfills `SceneId` and `From` from the snapshot's token
     (SPEC-013's command path, step 7; VTT-160).
   - "What could not be established" then says "no writer sets the second".
     That is false. It also omits that every server-written scene id is a
     folded token's scene id, which VTT-277 holds at 128 (Gap 2).
2. **"Done" is an observation (by command).** None of the named tests exists
   yet. Sketches of each fail on the base and pass on the probe tree (P5):
   - Item 1: twelve engine cases are red. Eleven show `err = <nil>`, and the
     participant-order case shows the unknown actor's text.
   - Item 2: eleven TS cases are red, ten with "Received function did not
     throw" and the order case with the unknown actor's message.
   - Item 3: `mapdef.Load` accepts a 129-byte, an empty and a repeated object
     id (`err = <nil>`), and so does `adventure.Load` (Gap 1).
   - Item 4: every command answers `ok=true error=""` and moves the head:
     `grant` 5→6, `revoke` 7→8, a 129-byte module id 9→10, a resource name
     11→12 and an attribute name 13→14.
   - Item 5 is the gate. It is reachable only through work the ticket does not
     list (Gap 3).

   The at-bound halves of items 1 and 2 cannot fail today, as the ticket says.
   K2, K5, K7, K9, K12, K17, K19, K24, K27, K29, K31, K34, K38 and K40 hold
   them.
3. **Each rule is breakable (a reading, then probes).** Each rule has a single
   edit that reds a named sketch test. K1 to K48 were run on the probe (P12);
   K49 to K52 need the final tree.
   - The first rule becomes rows A, B and C, E if Q4 is (a), and D if Q3 is
     (c).
   - The second becomes row F, widened to an adventure's scenes (Gap 1), and G
     if Q3 is (b) or (c).
   - The third is VTT-162 and VTT-161, which gain the wire tests. The ticket
     cites them rather than restating them, and the sort agrees (D12).
4. **The scope matches the claim (by command, then a reading).** It does not.
   The work is larger than "What it touches" says:
   - **A second way in for object ids.** An adventure's scenes carry objects
     too. `loadScenes` decodes them as `mapdef.ObjectJSON` and runs
     `mapdef.CheckObjectFootprints`, and nothing else on their ids.
     `adventure.Compile` hands them to `mapdef.BuildSceneCreated`, which copies
     `ObjectId` into the event. If only `mapdef` is fixed, an adventure with a
     long object id boots and is then refused by the fold at `load_adventure`,
     which breaks the ticket's second rule (Gap 1, D7).
   - **Two doc blocks become false and are too long to edit.**
     - `controlTarget`'s 12-line doc says it rejects "an unknown actor and an
       empty participant".
     - `requireControlTarget`'s 7-line JSDoc in `fold.ts` says "the same two
       rejections".

     A third rejection makes both lists incomplete. `check:comments` refuses
     any edit to a block over six lines, so both are cut (D5, D6).
   - **Three one-line texts become false.**
     - `fold.ts`'s `tokenMoved` comment says "`from` and `sceneId` are ignored
       entirely".
     - `server.go`'s backfill comment says "engine.Apply never reads SceneId
       or From".
     - `client/test/fold-unit.test.ts` has a test titled "tokenMoved ignores
       the event's from and sceneId entirely". No row cites it.
   - **The vocabulary gate reaches test fixtures.**
     `semgrep.no-game-system-vocabulary-in-engine` (`.semgrep/vocabulary.yml`)
     covers `internal/` and `cmd/`, tests included. It refused the sketch's
     `"hp"` resource key (P6), so every fixture key is neutral (`pool_a`,
     `attr_a`).
   - **The ledger.** Three production rows fall more than a point on the
     sketch alone (P10). `tools/comment-ceilings.txt` is named in the ticket,
     and `--write-ledger` is certain (D14).

   The ticket's other claims hold:
   - `engine.Apply`'s callers are unchanged: five in `internal/campaign` and
     one in `internal/harness`.
   - Only the gateway appends: `server.go`, `adventure.go`, `map.go` and
     `ruleset.go`.
   - Production code builds these events in a fixed set of places:
     - control events: `ToEvent`, `introduce` and `statusFrames`;
     - `ActorAdded`: `ToEvent`, `adventure.Compile` and `introduce`;
     - a `SceneCreated` with objects: `mapdef.BuildSceneCreated`, whose
       callers are `mapdef.Compile` and `adventure.Compile`;
     - `SessionStarted`: `ToEvent`, stamped by `stampSessionIDAgainst`;
     - `TokenMoved`: `ToEvent`, backfilled by `handleCommand`.
   - `introduce` and `statusFrames` copy folded state, so they need nothing.
   - `mapdef.CheckObjectFootprints` has two callers, `loadAs` and
     `loadScenes`. `controlTarget` and `requireControlTarget` have two callers
     each.
5. **No recorded decision is contradicted silently (a reading).**
   - SPEC-018 records these fields as unbounded, and the ticket names SPEC-018
     as moved.
   - SPEC-014 lists what `LoadInstalled` refuses, and the ticket names it too.
   - SPEC-013's validators paragraph ("leaves an `add_actor` with no actor or
     no `actor_id` to the fold") stays true, because no validator changes;
     VTT-154 stays true.
   - SPEC-009 records "Control of a character is not identity's: it is an
     `ActorControlGranted` in the log". The ticket's last open question, a
     grant checked against identity, would cut into that; this plan does not
     (Q6).
   - SPEC-016's `transitions` paragraph, under which a `SceneSeen` carries each
     object of the folded scene in sight, stays true.
   - ADR-007: no contract change.
   - `server.go`'s backfill comment records why the backfill sits in
     `handleCommand`: "engine.Apply never reads SceneId or From". The reason
     survives, but the sentence does not, and the ticket does not say so.
6. **The records the work moves are named (by command, then a reading).** The
   section lists SPEC-018 and SPEC-014, and both resolve. The reading:
   - SPEC-007, SPEC-009, SPEC-011, SPEC-012, SPEC-013, SPEC-016 and SPEC-019
     hold no sentence the change makes false. Each was grepped for participant
     ids, session ids, module ids, object ids, scene ids and keys.
   - Records outside `docs/specifications/` that become false and are not
     named:
     - the four comments and the test title of check 4;
     - `at-every-boundary/guide.md`'s table, which gains a row;
     - the debt entry's count, which the ticket does name.

## Measurements this plan stands on

Every measurement was taken by command, never in the working tree. Three
scratch clones of `9760d23` were used (`git clone --no-hardlinks`, `bun install
--frozen-lockfile`):
- `base`, left as committed, for baselines and red sketches;
- `instr`, with `engine.Apply` and `fold.ts`'s `fold` wrapped to log every value
  of the eight fields that was empty or unusually long, with its verdict;
- `probe`, the sketch of D1 to D8 under Q2(a), Q3(c), Q4(a) and Q5(b), with the
  keys re-pointed as D13 says.

The sketch tests were named `zz*`.

| # | What | Result |
|---|---|---|
| P1 | Base. Paths and symbols (check 1) | All resolve. |
| P2 | Base. Every value of the eight fields in the 946 tracked JSON files (945 parse), by a walk over `git ls-files '*.json'`, schema and tool files left out | Longest participant id 16 (`{{id:latecomer}}`, a scenario template), session id 17 (`sess-happy-dragon`), module id 14 (`dnd45e-minimal`), attribute key 12 (`constitution`), resource key 11 (`flurry_uses`), map object id 16 (`crate-stack-east`, in the shipped `campaigns/example/maps/cellar.json`), `TokenMoved` scene id 18. 26 objects in 20 files: none without an id, none empty, none repeated in its file. |
| P3 | `instr`, Go. `go test -count=1 -p 2 ./internal/... ./contract/... ./tools/... ./cmd/...`, the spawned `vtt` binaries inheriting the log | 21 packages ok (`cmd/vtt` 158.9 s). **Participant ids:** none over 32 bytes; two empty, both refused. **Module ids:** none over 16. **Keys:** none empty; over 20 bytes, exactly four, each 128 and accepted (`res_…` twice, `attr_…`, `def_…`, from `internal/rules/qa_ruleset_bound_test.go`). **Object ids:** none empty, over 20 or repeated in a `SceneCreated`, and none in any `SceneSeen`. **Session ids other than 37 bytes:** empty 902 accepted and 15 refused; `s1` 122 and 2; `sess` 17; `sess-1` 9,923; `qa-iss-session` 18; none over 37. **`TokenMoved` scene ids:** empty 11,558 accepted and 1 refused; none over 20. |
| P4 | `instr`, TS. Each of the 38 `client/test` and `contract` test files run alone | All exit 0. Session ids: `sess-1` 417 accepted and 4 refused; empty 15 accepted and 8 refused. Move scene ids: empty 4 accepted and 5 refused. Two empty participant ids, refused. Nothing else logged. |
| P5 | Base, then probe. Each Done item's sketch | **Engine:** 12 red on base (the revoke case observed in a test of its own, since the grant case's `t.Fatal` stopped the first), at-cap green; all green on probe, with each exact text of D4. **TS:** 11 red on base, 12 green on probe. **`mapdef`:** base loads all three faults; probe refuses them with `field "objects[1].id": must be at most 128 bytes, got 129`, `field "objects[0].id": must not be empty` and `field "objects[1].id": duplicate object id "a"`, and loads a 128-byte id. **`adventure`:** the same three texts behind `adventure: <path>/scenes/cellar.json:`; base loads all three. A scene object with no `art` is refused by the loader's dry run (`field "overrides": mapdef: objects[0] has no art …`), so every adventure fixture object declares art (Gap 10). **Wire:** base `ok=true error=""` for all five with the head moving; probe ok=false with the fold's exact text, the head unchanged, then the 128-byte command ok at `head+1`. |
| P6 | The whole probe tree | `go test` as P3: 21 packages ok (`cmd/vtt` 160.8 s). `bun test client/test contract contract-spike`: 1058 pass, 0 fail (1046 on base plus the 12 sketches). `bunx tsc --noEmit -p client/tsconfig.json` clean. `golangci-lint run ./internal/engine/ ./internal/mapdef/ ./internal/adventure/`: 0 issues. `go-arch-lint check`: OK. `check-doc-owner.py`: 80 files. `semgrep scan --config .semgrep/ --error --quiet internal/ cmd/`: exit 1 on the sketch's `"hp"` key in an engine test, then exit 0 with `pool_a`. |
| P7 | `gocyclo`, base then probe | `Apply` 112 → 120 (under `//nolint:gocyclo`); `controlTarget` 3 → 4; `loadAs` 24 → 25; `loadScenes` 22 → 23; new `CheckObjectIDs` 5, `longestKey` 2. None but `Apply` is within 5 of the ceiling of 30. |
| P8 | Keys mapped base → probe by `difflib`, every line compared | `apply.go`: `351:58`→`374:58`, `468:15`→`491:15`, `471:30`→`494:30`, `618:41`→`641:41` (+23 each). `fold.ts`: `326:11`, `327:11`, `328:11`, `328:37`, `386:48`, `415:19`, `424:11`, `425:26`, `465:10`, `466:5`, `495:7` +11; `585:34`, `607:32`, `712:7`, `712:21` +6. `internal/adventure/load.go`'s `89:58` unchanged; `internal/mapdef` holds no key. Columns unchanged. |
| P9 | Both mutation self-tests on the probe | Before re-pointing, the Go self-test names all four `apply.go` keys. The TS self-test flags 6 of the 15 `fold.ts` keys and **cannot see** `326:11`, `327:11`, `328:11`, `328:37`, `386:48`, `415:19`, `466:5`, `607:32` or `712:7` move. After re-pointing by P8: `OK`, `OK`, and each key reads its statement. |
| P10 | `check-comments.py 9760d23` and the shares, base → probe | 4 added comment lines, no banned term, no touched block over six lines. **Three findings:** `apply.go` 40.73 → 37.91 against 40.8; `fold.ts` 42.30 → 41.13 against 42.4; `internal/mapdef/load.go` 50.89 → 49.23 against 50.9. `internal/adventure/load.go` 32.46 → 32.30 against 32.5 stays inside the band. Code lines each test file absorbs before a finding, with no comment line added: `apply_test.go` 106, `apply_boundary_test.go` 7, `server_test.go` 96, `internal/mapdef/load_test.go` 13, `internal/adventure/load_test.go` 20. `check-new-prose.py 9760d23`: one finding, `LONG internal/mapdef/load.go … [101]`, the sketch's one-line `CheckObjectIDs` doc (LONG is 85 columns), so D7 wraps it. |
| P11 | Q5(b)'s three `fieldDocs`, then `go run ../tools/toolgen` and the copy to `cmd/vtt/tools.json` | Each `tools.json` changes 3 lines and nothing else. `TestToolsMatchGolden` is red until `contract/testdata/expected_tools.json`'s three lines change by hand. `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication` stays green, since "opaque" is kept. `contract`, `internal/mcp` and `internal/engine` are green. |
| P12 | The breaks, one edit each on the probe, the file restored from its saved text and checked by hash | K1 to K48 each red where the K table says, against the sketches. K7 and K9 also red `TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts`. K20 reds 21 tests and K21 reds 26, earlier arcs' included. |
| P13 | Records and gates on the base | `check-requirements-chain.py .`: 288 rows, 233 test files, 13 specifications. `check-comments.py main`: 273 files, clean (one notice on `cmd/vtt/library_test.go`, also on the base). `check-doc-owner.py .`: 80 files. Both self-tests `OK`. `requirement-id` is at `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`. Free on `/System/Volumes/Data`: 32,212,692 KiB at the start, 31,685,224 KiB at the end. Load 2.0 at the start, 1.2 at the end. Go cache 8.7 GiB. |
| P14 | Base. `protojson.Marshal` of an `Actor` whose `attributes` is `{"": 3}`, and of a `SceneObject` with an empty id | `{"actorId":"a", "attributes":{"":3}}`: an empty key is on the wire. `{"kind":"k"}`: an empty object id is not. |

## Constraints that bind every task

- **Rule 2.** No gate is weakened. No `//nolint` is added; `Apply`'s existing
  one covers D4's eight field checks, the same shape as the sixteen bound
  checks already under it. The ledger goes down only through `--write-ledger`.
  A key is re-pointed by statement.
- **Rule 3.** No proto changes. `check:breaking` reports nothing.
- **Rule 4.** Both folds refuse the same events at the same counts, in the
  same order within each arm (D4, D6). A TS fold stricter than Go's freezes a
  client, so neither side lands alone.
- **Rule 5.** No new vocabulary. "Participant", "module", "resource",
  "attribute", "object", "session" and "scene" are the contract's words.
  `semgrep.no-game-system-vocabulary-in-engine` reads test fixtures under
  `internal/` too: no `hp`, `hit_points`, `fortitude`, `saving_throw`,
  `bloodied` or the rest of its list, in any test (P6).
- **Rule 8.** Cite names, never lines. The adjudication files keep their
  coordinates.
- **Rule 9.** Answered above.
- **Rule 10 and SPEC-010.** New comments are limited to these:
  - test citation lines;
  - `CheckObjectIDs`' doc, one sentence on two lines;
  - a two-line warning on `longestKey`;
  - the one-line pointers that replace the two cut blocks;
  - the two edited one-line texts in `fold.ts` and `server.go`.

  No banned term appears on an added line; "measures" is not banned, but
  "measured" is. No block of more than six lines gains a line. No added
  comment line is wider than 85 columns, which `check:new-prose` refuses as
  LONG, and none under 55 is followed by more of the same sentence (SHORT).
- **SPEC-008.** Ids come from `requirement-id`, after sign-off, in letter
  order.
- **Architecture** (`.go-arch-lint.yml`). `engine` may import `contract` and
  itself; `mapdef` may import `mapdef`, `contract` and `artlib`, never
  `engine`; `adventure` may import `engine`, `contract`, `rules`, `mapdef` and
  itself; `gateway` may not import `store`. Every test file this plan adds
  stays inside its package's list, and so does every QA file (D16).
- **Test first, on a tree that builds.** Every new test reaches the code
  through what exists on the base, so its red is observed there:
  - engine tests through `engine.Apply`;
  - loader tests through `mapdef.Load` and `adventure.Load`, never the new
    `CheckObjectIDs`;
  - the tool link test through `maxIDBytes`, which already exists.

  No new constant is added.
- **Order and hygiene.**
  - `check:drift` passes only on a committed tree, so the order is review,
    then commit, then gate.
  - The review package is `git diff HEAD`, and nothing is stashed or checked
    out while a reviewer reads.
  - `git add` and `git commit` run in separate calls, with `git show --stat
    HEAD` after each.
  - A new test goes after a closing brace, never above another test's citation
    line.
  - The pre-push hook takes about three minutes; let it finish.
- The ticket, every report under `docs/reports/` and every ticket under
  `docs/superpowers/specs/` are not edited.

## Decisions this plan makes

**D1. The bound: 128 bytes of UTF-8, inclusive, the fold's own `maxIDBytes`,
for all eight fields (Q1).** No constant is added. At load, `internal/mapdef`'s
existing `maxIDBytes` bounds object ids for both loaders through `CheckObjectIDs`
(D7). What forced it:
- **An actor's keys must be at least the ruleset's names**, which are held at
  `internal/rules`' `maxIDBytes` (the D7 check above). Equal by construction is
  one constant.
- **A participant id is an identity's 32 hex characters** in production, and
  `"p1"`-style strings in tests. A fold bound of 32 would write identity's id
  format into `internal/engine`, which may not import identity
  (`.go-arch-lint.yml`), while SPEC-009 keeps control out of identity. 128
  admits identity's ids with a margin and is the codebase's id bound.
- **Nothing comes near it.** The longest value in content is 18 bytes (P2).
  The longest folded is 37 bytes (a session id), apart from the four 128-byte
  keys the ruleset arc's QA folds on purpose (P3, P4).

**D2. Empty values (Q2, Q3).**
- **A participant id stays refused when empty**, with its existing text
  `engine: <event> requires a participant id`. SPEC-018 already states that
  refusal.
- **An object id is refused when empty** (Q3(c)). protojson omits an empty
  `object_id`, so on the wire it cannot be told from an absent one (P14).
  Nothing in the tree carries one (P2, P3).
- **A module id, a session id, a move's scene id and an actor's key are
  admitted empty** (Q2(a)):
  - An empty module id is the ordinary case.
  - An empty session id is folded on 902 Go `SessionStarted`s and 15 TS ones
    (P3, P4).
  - An empty move scene id is folded on 11,558 Go `TokenMoved`s, because
    `ToEvent` never sets one and tests build moves without it.
  - A map key `""` is present on the wire, unlike an empty id field (P14).
    Refusing one would also add a second refusal over an unordered map, whose
    order both folds would then have to fix (Q2(b)).

**D3. Scope: the eight fields, and object ids at both loaders (Q3, Q4,
Q7).** An adventure's scene objects are in scope because they reach the same
`SceneCreated` (Gap 1).

**Fields a bound on another field already holds**, which this plan does not
check:

| Field | Held by |
|---|---|
| `SceneSeen.objects[].object_id` | built by `sceneSeenFor` from a folded scene's objects, so held by row C |
| `ResourceChanged.resource` | the fold's lookup in the actor's resources, whose keys row B bounds |
| a control event's participant id in `introduce` and `statusFrames` | copied from a folded actor's `controller_ids`, which row A bounds |
| a server-written `TokenMoved.scene_id` | copied by the backfill from a folded token's scene, which VTT-277 bounds |

**Fields left out, and named so nobody assumes they are bounded.** The report
raises them:

| String | Where it goes | Longest measured | Why left |
|---|---|---|---|
| every other envelope's `session_id` | the store's column, every viewer | 37 | `stampSessionIDAgainst` overwrites it on every append, with a stored id or `""` |
| `Envelope.event_id`, `participant_id`, `actor_role` | the store and the DM's stream | 32 (participant) | stamped by the gateway; not named by the owner |
| `Actor.module_data` | `Actor` (a `google.protobuf.Struct`) | none in content | opaque by contract; not named |
| an object's `kind` and `art` | `Scene.Objects` | — | the stored `art` is `""` or a name that resolved (`ResolveObjectArt`), so `artlib`'s id rule bounds it; `kind` is stored whole and clipped where `describeBlockage` renders it (SPEC-013) |
| a tile's kind, material and art | `Scene.Tiles` | — | tile names are a closed set (`CheckTileNamesKnown`) |
| a map placement's `actor_id` | `TokenPlaced.actor_id` | 11, apart from `at-every-boundary`'s 128 | must name a folded actor (VTT-277), but `load_map`'s refusal of an unknown one quotes it at any length |
| the adventure loader's undeclared-key refusal | a boot error | — | quotes the key at any length, at boot only (the ruleset plan's D7) |

**D4. The Go fold.** Each check goes where SPEC-018's existing order puts the
least change.
- **`SessionStarted`**, after the name check: `if len(env.SessionId) >
  maxIDBytes { return fmt.Errorf("engine: session id must be at most %d bytes,
  got %d", …) }`.
- **`SceneCreated`**, in the objects loop, after the name check and before
  anything is stored. `objectIDs := make(map[string]bool, len(sc.Objects))`
  goes above the loop. Each object is checked first for `if n :=
  len(o.GetObjectId()); n == 0 || n > maxIDBytes` with `engine: object id must
  be 1-%d bytes, got %d`. Then, under Q3(c), for a repeat: `engine: object %q
  appears twice in scene %q`. The repeat refusal quotes an id the length
  check has already held to 128 bytes. Under Q3(a) or (b), the length check
  takes the "must be at most" form and the repeat check goes.
- **`ActorAdded`**, after the name check:
  - `engine: module id must be at most %d bytes, got %d`;
  - `if n := longestKey(a.GetResources()); n > maxIDBytes` with `engine:
    resource name must be at most %d bytes, got %d`;
  - the same for `GetAttributes()` with `attribute name`.

  These go after the name check, so the arm's 25-line comment block is not
  touched.
- **`controlTarget`**, after the empty-participant check and before the actor
  lookup: `if len(participantID) > maxIDBytes { return nil,
  fmt.Errorf("engine: participant id must be at most %d bytes, got %d", …) }`.
  The length is a property of the event alone, so it is checked with the
  emptiness, before anything is looked up. A test holds that order (D9, K3).
- **`TokenMoved`**, after the reason check: `engine: move scene id must be at
  most %d bytes, got %d`. "move scene id" follows "move reason", because "scene
  id" is the `SceneCreated` text.
- **`longestKey`**, new and unexported, at the end of `apply.go`, below every
  key, so it moves none:

  ```go
  func longestKey[V any](m map[string]V) int {
  	n := 0
  	for k := range m {
  		n = max(n, len(k))
  	}
  	return n
  }
  ```

  What forced it:
  - **Go randomises map order.** A first-match loop would name a different
    length on each run when two keys are over the bound. The `ActorRemoved`
    arm already refuses that shape: "an error message that changes between
    runs cannot be asserted on".
  - **No equivalent mutant.** The builtin `max` gives gremlins no operator to
    mutate. A hand-written `if len(k) > n { n = len(k) }` would breed a
    `CONDITIONALS_BOUNDARY` mutant that no test can kill.

  It carries the two-line warning D14 names.

The texts take SPEC-018's two forms. Cost under `//nolint:gocyclo`: `Apply` goes
from 112 to 120 (P7). That is eight field checks inside existing arms, the same
shape as the sixteen bound checks the function already holds under that nolint.

**D5. `controlTarget`'s doc is cut.** Its 12 lines become `// controlTarget
resolves the actor a control event names (SPEC-018).` Its two arguments move
to the report's "What the cut comments recorded":
- why an unknown actor is an error;
- why an empty participant breaks the `controller_id` mirror.

`check:comments` refuses a partial edit of a block over six lines, and
"rejecting an unknown actor and an empty participant" would otherwise read as
the whole list.

**D6. The TS mirror, at the same place in each arm.**
- `sessionStarted`: `checkLen("session id", env.sessionId, 0, 128);` after
  the session name.
- `sceneCreated`: after the scene name, and before the six-line comment, which
  stays untouched:

  ```ts
  const objectIds = new Set<string>();
  for (const o of v.objects) {
    checkLen("object id", o.objectId, 1, 128);
    if (objectIds.has(o.objectId)) throw new FoldError(`duplicate object "${o.objectId}" in scene "${v.sceneId}"`);
    objectIds.add(o.objectId);
  }
  ```

  Under Q3(a) or (b) this is one line with a minimum of 0. A `Set` is not a
  dictionary, so `emptyMap`'s prototype rule does not reach it.
- `actorAdded`, after the actor name:
  - `checkLen("module id", a.moduleId, 0, 128);`
  - `for (const k of Object.keys(a.resources)) checkLen("resource name", k, 0, 128);`
  - the same with `a.attributes` and `"attribute name"`.

  Key order needs no fixing here, because a TS refusal carries no length.
- `tokenMoved`: `checkLen("move scene id", v.sceneId, 0, 128);` after the
  reason. Its one-line comment becomes `` // `from` is ignored and `sceneId`
  only bounded, exactly as Go does. ``
- `requireControlTarget`: `checkLen("participant id", participantId, 0, 128);`
  after the empty throw. The minimum is 0 because the line above has already
  refused an empty id; this is D5 of the id plan. Its 7-line JSDoc becomes
  `/** requireControlTarget mirrors internal/engine's controlTarget (SPEC-018).
  */`, for D5's reason.
- `client/test/fold-unit.test.ts`'s "tokenMoved ignores the event's from and
  sceneId entirely" is retitled "tokenMoved looks nothing up by the event's from
  or sceneId". Its body and its comment stay true. No row cites it.

**D7. The loaders.**
- **`CheckObjectIDs`**, new and exported, goes in `internal/mapdef/load.go`
  directly after `CheckObjectFootprints`. Its doc is one sentence over two
  lines, since one line is 101 columns and `check:new-prose` refuses past 85
  (P10):

  ```go
  // CheckObjectIDs refuses an object id that is empty, repeated, or longer than
  // maxIDBytes (SPEC-018).
  ```

  For each object, with field `objects[i].id`, it refuses in this order:
  - `must not be empty`;
  - `must be at most %d bytes, got %d`;
  - `duplicate object id %q`, where the quoted id is already at most 128 bytes.

  Under Q3(a), only the length check remains.
- **`loadAs`** calls it just before `CheckObjectFootprints`. `Load`'s doc says
  "then Overrides and Objects", and that stays true.
- **`loadScenes`** calls `mapdef.CheckObjectIDs` just before
  `mapdef.CheckObjectFootprints`. That closes Gap 1, and its refusals carry
  `adventure: <path>:` from its own `errf`.
- `gocyclo`: `loadAs` 25 and `loadScenes` 23 (P7).
- Boot (`loadMapsDir`) and `load_map` (`mapByID`) reach `loadAs` through
  `LoadInstalled`. Boot and `load_adventure` reach `loadScenes` through
  `adventure.Load`.
- **No adventure-loader change for keys.** The membership check plus VTT-283
  bound them already (D7 check above).

**D8. The gateway.** No production logic changes. The fold refuses, and
VTT-161 and VTT-162 carry the refusal to the issuer (P5). Nothing is added to
`validateGrantActorControl` or `validateAddActor`, so SPEC-013's validators
paragraph and VTT-154 stay true. `server.go`'s three-line backfill comment
becomes three lines, each under 85 columns:

```go
	// Keep the backfill here: engine.Apply never reads From and only bounds SceneId,
	// and nothing after this point holds both the pre-move token and the envelope
	// (VTT-160).
```

**D9. The tests.** Each is red today except the at-cap ones (P5). Keys and ids
are neutral strings (`pool_a`, `attr_a`, `k`, `p`), never game vocabulary.
- **`internal/engine/apply_test.go`, at the end, after
  `TestAConditionIDIsMeasuredBeforeItsActorIsLookedUp`.** `env` has no case for
  the control events, so the tests build those envelopes directly, or `env`
  gains the two cases.
  - `TestAParticipantIDThatExceedsTheBoundIsRefused`, with grant and revoke
    subtests, each 129 bytes on a known actor.
  - `TestAParticipantIDIsMeasuredBeforeItsActorIsLookedUp`: a 129-byte
    participant on an unknown actor gets the participant's text.
  - `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused`, with four
    subtests:
    - `module id`;
    - `resource name`, with `pool_a` beside a 129-byte key;
    - `attribute name`, with `attr_a` beside a 129-byte key;
    - `the longest key is named`, with keys of 129 and 200 bytes and `x`,
      asserting `got 200`.
  - `TestASceneObjectWhoseIDExceedsTheBoundIsRefused`: a valid object, then
    `é`×64 + `o` (129 bytes, 65 characters).
  - Under Q3(c), `TestAnEmptyOrRepeatedSceneObjectIDIsRefused`, with `empty`
    and `repeated` subtests (`o1`, `o2`, `o1`).
  - Under Q4(a), `TestASessionWhoseIDExceedsTheBoundIsRefused` and
    `TestAMoveWhoseSceneIDExceedsTheBoundIsRefused`.

  Each asserts the exact text and an unchanged `Snapshot`.
- **`internal/engine/apply_boundary_test.go`, at the end:**
  `TestCommandAndFileIDsAtCapAreAccepted`. One stream folds every field at 128:
  - a session id;
  - an object id of `é`×64;
  - a module id and one resource and one attribute key;
  - a grant and then a revoke of a 128-byte participant;
  - a move whose scene id is 128.

  It asserts each stored value: `Session.ID`, `ObjectID`, the module id, the
  keys and the controller.
- **`internal/gateway/server_test.go`, after
  `TestATokenWhoseIDExceedsTheBoundAppendsNothing`'s closing brace:**
  - `TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing`.
    For a grant and then a revoke on `newGWFixture`'s `a1`: 129 bytes, ok=false
    with the exact text and `f.head` unchanged. Then the 128-byte command,
    **the next command on the same connection**, answers ok at `head+1`.
  - `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing`: the same for
    `add_actor` with a 129-byte module id, resource name and attribute name.
    Each command sets `Kind`, since `validateAddActor` refuses a kindless one
    first.
  - Both cite the row (A or B), VTT-161 (the fold refuses; the connection
    stays open for the next command, which the test sends) and VTT-162 (the
    head does not move).
- **`client/test/fold-rejections.test.ts`, at the end, in a "command and file
  ids" section**, these cases, by these names, which the K table uses:
  - "a participant id longer than 128 bytes is rejected in a grant";
  - "a participant id longer than 128 bytes is rejected in a revoke";
  - "a participant id is measured before its actor is looked up";
  - "a module id longer than 128 bytes is rejected";
  - "a resource name longer than 128 bytes is rejected", with `pool_a` beside
    it;
  - "an attribute name longer than 128 bytes is rejected", with `attr_a`
    beside it;
  - "an object id over 128 UTF-8 bytes is rejected" (`é`×64 + `o`);
  - under Q3(c), "an empty object id is rejected" and "a repeated object id is
    rejected";
  - under Q4(a), "a session id longer than 128 bytes is rejected" and "a
    move's scene id longer than 128 bytes is rejected";
  - "command and file ids of exactly 128 bytes are ACCEPTED", folding every
    field at 128 and asserting the stored session id, object id and module id.

  `env`'s spread lets a case set `sessionId`.
- **`internal/mapdef`:**
  - `testdata/invalid/object-id-too-long/map.json`, and under Q3(b) or (c)
    `object-id-empty/map.json` and `duplicate-object-id/map.json`. Each is
    `placement-token-id-too-long`'s map with a valid token id and the one
    object fault.
  - Their rows at the end of `TestInvalidMapsAreRefusedWithAUsefulReason`, with
    the full `field "objects[0].id": …` texts.
  - `TestAnObjectIDOfExactlyTheBoundLoads` in `load_test.go`, after
    `TestAMapIDOfExactlyTheBoundLoads`.
- **`internal/adventure`:**
  - `testdata/invalid/object-id-too-long`, and under Q3(b) or (c)
    `object-id-empty` and `duplicate-object-id`. Each copies
    `placement-token-id-too-long`'s shape with its token id restored, one object
    fault, and objects that declare `art` (P5). Each has its own `guide.md`.
  - Their rows at the end of `TestLoadInvalidFixtures`.
  - Their names in `TestLoadInvalidFixturesCatalogueIsComplete`.
  - `at-every-boundary/scenes/pin.json` gains one object whose id is exactly
    128 bytes, with `kind` and `art`. It loads on the probe.
  - An assertion at the end of `TestLoadAcceptsValuesExactlyOnEveryLimit`, with
    no comment, and a row in `at-every-boundary/guide.md`.

The goldens need no case: no stream holds one of these values within 91 bytes
of the bound (P3).

**D10. The tool descriptions (Q5).** Under Q5(b), `add_actor`'s
`vtt.v1.Actor` `fieldDocs` keep their sentence and gain one:
- `moduleId`: "Optional; omit unless a rule module instructs otherwise —
  moduleData is opaque. At most 128 bytes of UTF-8; a longer one refuses the
  whole command."
- `attributes` and `resources`: the same, with "At most 128 bytes of UTF-8 per
  name".

None carries `maxLength`, because JSON Schema counts characters.
`TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication` stays green, since
"opaque" is kept (P11). The new tests:
- `TestAddActorStatesTheBoundOnItsModuleAndKeys` in
  `tools/toolgen/main_test.go`;
- `TestTheToolsStateTheFoldsBoundOnAnActorsModuleAndKeys` in
  `internal/engine/id_bound_internal_test.go`, after
  `TestTheToolsStateTheFoldsIDBound`. It reads
  `../../contract/gen/tools/tools.json` and requires `fmt.Sprintf("At most %d
  bytes", maxIDBytes)` in the three descriptions.

`task generate:contract` regenerates both `tools.json` files.
`expected_tools.json`'s three lines change by hand. The participant ids get no
description: the agent copies an identity's id, it does not choose one.

**D11. The records.**

**SPEC-018**, written with the `specification` skill against the final tree:
- **Status:**
  - `Apply`'s arms gain `ActorControlGranted` and `ActorControlRevoked`
    (through `controlTarget`), and `fold.ts`'s "the same ten arms" becomes
    twelve (through `requireControlTarget`);
  - add `internal/mapdef/load.go`'s `CheckObjectIDs`, which both loaders call;
  - under Q5(b), add `add_actor`'s module id, attributes and resources to the
    tool list;
  - add the new test files and, when they exist, the QA files.
- **"Sixteen fields are bounded"** becomes twenty-four under the
  recommendations. The table gains eight rows, all 128 (`maxIDBytes`):
  - `ActorControlGranted.participant_id` and
    `ActorControlRevoked.participant_id`, "May be empty" no;
  - `ActorAdded`'s `Actor.module_id`, yes;
  - its `Actor.resources` keys and its `Actor.attributes` keys, each "yes"
    under Q2(a);
  - `SceneCreated`'s `SceneObject.object_id`, "no" under Q3(c);
  - `SessionStarted`'s `Envelope.session_id`, yes;
  - `TokenMoved.scene_id`, yes.
- **The refusal sentence** gains a second exception: a control event whose
  participant id is empty is refused as `engine: <event> requires a
  participant id`. It also gains that a key's refusal names the longest key's
  length.
- **The order sentences:**
  - a `SessionStarted` is checked for an open session, then its name, then its
    session id;
  - a `SceneCreated` for its id, then a duplicate, then its name, then each
    object in order, its id's length and then, under Q3(c), a repeat;
  - an `ActorAdded` gains "then its module id, its resource names and its
    attribute names";
  - a control event is checked for a participant id, then its length, then a
    known actor;
  - a `TokenMoved` gains "then its scene id".
- **"These sixteen are the only texts … beyond them it requires a control
  event's participant id to be non-empty"** becomes "These twenty-four … ;
  beyond them it requires a scene's object ids to be distinct" under Q3(c).
  Otherwise it ends at "bounds above".
- **"for each of the sixteen fields"** becomes twenty-four.
- **"Other mirrors":** `internal/mapdef/load.go`'s `maxIDBytes` now also bounds
  object ids, a map's through `loadAs` and an adventure's scenes' through
  `loadScenes`, both by `CheckObjectIDs`. It is held by the `object-id-*`
  refusal rows, `TestAnObjectIDOfExactlyTheBoundLoads` and
  `TestLoadAcceptsValuesExactlyOnEveryLimit`. Under Q5(b), the tool paragraph
  gains `add_actor`'s module id and names, held by
  `TestTheToolsStateTheFoldsBoundOnAnActorsModuleAndKeys`.
- **Consequences:** under Q5(b), the second bullet's "for an id the
  `add_actor` and `place_token` descriptions" gains "and `add_actor`'s module
  id, attribute and resource descriptions".
- **Requirements:** add the new rows.
- **The title and path** stay.

**SPEC-014:**
- "a placement whose token id is empty or longer than 128 bytes, and a file
  that does not compile are refused there" gains "an object whose id is longer
  than 128 bytes of UTF-8 (SPEC-018), or under Q3(b)/(c) empty or repeated in
  the map,".
- The first consequence gains: "and object ids of at most 128 bytes, distinct
  within the map".
- **Requirements:** gains F (and G).

Phase 4b verifies every replaced sentence in both records by command.

**The register:** VTT-161 and VTT-162 gain the two wire tests. VTT-154 is
unchanged.

**`docs/verification-debt.md`'s open entry "Nothing ties a byte bound's copies
to the engine's constant"** is amended:
- "sixteen bounds" becomes twenty-four;
- `internal/mapdef/load.go`'s `maxIDBytes` is said to bound object ids for both
  loaders;
- add that `TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts` reds when
  the engine's id bound falls below 128;
- under Q5(b), "Only a move's reason, a name and an id have a link" gains the
  add_actor module and keys.

The entry stays open: nothing compares a copy with the constant.

**D12. The sort.** Forced by SPEC-008. The rows are lettered so nothing reads
as an id.

| # | Rule | Proposed | Red when | Tests | Record |
|---|---|---|---|---|---|
| A | Both folds refuse an `ActorControlGranted` or `ActorControlRevoked` whose participant id is longer than 128 bytes of UTF-8, and accept one of exactly 128. | accept: the ticket's first rule, and the refusal half of its third | K1 to K3, K20 to K25 | D9's participant tests, engine, TS and wire, and the at-cap tests | SPEC-018 |
| B | Both folds refuse an `ActorAdded` whose actor's module id, or one of whose resource or attribute names, is longer than 128 bytes of UTF-8, and accept each at exactly 128. | accept: the first rule | K4 to K10, K20, K21, K26 to K31 | D9's actor tests, engine, TS and wire, the at-cap tests | SPEC-018 |
| C | Both folds refuse a `SceneCreated` one of whose objects' id is longer than 128 bytes of UTF-8, and accept one of exactly 128. | accept: the first rule | K11, K12, K15, K20, K21, K32, K34, K35 | D9's object refusals, the at-cap tests | SPEC-018 |
| D | Both folds refuse a `SceneCreated` one of whose objects' id is empty or the same as another of its objects'. | accept under Q3(c) | K13, K14, K33, K36 | the empty and repeated cases | SPEC-018 |
| E | Both folds refuse a `SessionStarted` whose session id, or a `TokenMoved` whose scene id, is longer than 128 bytes of UTF-8, and accept one of exactly 128. | accept under Q4(a) | K16 to K19, K37 to K40 | D9's session and move tests, the at-cap tests | SPEC-018 |
| F | A map file, or an adventure's scene, one of whose objects' id is longer than 128 bytes of UTF-8 is refused when it loads, by an error naming the file and the field. | accept: the second rule, with the adventure's scenes (Gap 1) | K41, K42, K45 to K49 | the `object-id-too-long` rows, `TestAnObjectIDOfExactlyTheBoundLoads`, `TestLoadAcceptsValuesExactlyOnEveryLimit` | SPEC-018, SPEC-014 |
| G | A map file, or an adventure's scene, one of whose objects' id is empty or the same as another of its objects' is refused when it loads, naming the file and the field. | accept under Q3(b) or (c) | K43, K44 | the `object-id-empty` and `duplicate-object-id` rows | SPEC-014 |
| H | The `add_actor` tool states the bound the fold enforces on an actor's module id and its resource and attribute names. | accept under Q5(b) | K20, K50 to K52 | D10's two tests | SPEC-018 |

**Refused, one line each:**
1. "These values are at most 128 bytes" on its own: A to E hold it.
2. "A refused `grant_actor_control`, `revoke_actor_control` or `add_actor`
   appends nothing": VTT-162, which gains the wire tests.
3. "It is answered ok=false on a connection that takes the next command":
   VTT-161, likewise.
4. "`fold.ts` refuses what `engine.Apply` refuses" as a parity row: each row
   names both folds.
5. "A `SceneSeen`'s objects are bounded": it follows from C and D through
   `sceneSeenFor` (D3).
6. "An adventure actor's keys are bounded at load": it follows from VTT-283
   through `loadActors`' membership checks.
7. "An introduction or a correction carries values within the bound": it
   follows from A and B.
8. "A key's refusal names the longest key": how B is observed, and SPEC-018's
   text.
9. "The participant id is checked before its actor": SPEC-018's order text,
   held by a test cited under A.
10. "A loader's copy equals the engine's constant": a consistency of copies,
    which is SPEC-018's consequence and the debt entry.
11. "A `TokenMoved`'s scene id names its token's scene": a different rule. The
    backfill makes it true on every command path.
12. "An empty participant id is refused": existing behaviour of both folds,
    stated by SPEC-018.

Eight rows are proposed under the recommendations, and four (A, B, C and F) if
every option is declined. Twelve candidates are refused.

**D13. Mutation keys: re-point last, by statement.** These apply after the
review settles and before the commit. P8 gives the positions under the
recommendations; re-read each at its statement.
- **`apply.go`**, all four keys move +23: `id < standing` from `351:58` to
  `374:58`, `computed < 0` from `468:15` to `491:15`, `computed >
  int64(res.Max)` from `471:30` to `494:30`, and `len(objs) > 0` from `618:41`
  to `641:41`. The shift is:
  - the session check, +3;
  - the object checks, +8;
  - the module and key checks, +9;
  - the move check, +3.

  Under Q3(a) or (b) the object checks are +3 (total +18). Under Q4(b) the
  session and move checks go (−6).
- **`fold.ts`:**
  - `326:11`, `327:11`, `328:11`, `328:37`, `386:48`, `415:19`, `424:11`,
    `425:26`, `465:10`, `466:5` and `495:7` move +11. That is the session
    line (+1), the object block (+6), the module and key lines (+3) and the
    move line (+1).
  - `585:34`, `607:32`, `712:7` and `712:21` move +6: the same +11, the
    participant line (+1) and the JSDoc's cut (−6).
  - Under Q3(a) or (b), subtract 5 from each. Under Q4(b), subtract 2.
- **No entry is deleted and none is added.** No adjudicated arm starts
  checking something:
  - `attackRolled` and `default:` are untouched.
  - `415:19`'s reason, that `copyActor` always builds `resources`, stays true.
  - `712:7`'s reason, that a stored actor id is never empty, stays true.
- **`internal/adventure`'s `89:58`** sits above every edit, and
  **`internal/mapdef`** holds no key.

Then run `python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q`. The TS self-test cannot see nine of the
fifteen moves (P9), so its `OK` is necessary and not sufficient; Phase 4b
re-reads every key. If a review adds a line above an edit point, re-measure
every key.

The new mutants die as follows:
- Go `>=`: by the at-cap tests.
- Go negations and `||` → `&&`: by the refusals and by every valid fold.
- The TS labels emptied: by the exact messages.
- `CheckObjectIDs`' `>=`: by `TestAnObjectIDOfExactlyTheBoundLoads` and
  `at-every-boundary`.
- `longestKey` has no operator to mutate.

The mutation gates were not run here (Gap 9).

**D14. Comments and the ledger.**
- **Tests** carry their `// VTT-NNN` line only.
- **Production comments**, the only comment lines this change adds:
  - D5's and D6's one-line pointers, in place of 12 and 7 lines;
  - `CheckObjectIDs`' two-line doc (D7);
  - on `longestKey`, a two-line warning, wrapped under 85 columns:

    ```go
    // Keep the maximum: map order is random, and a refusal must name one length
    // on every run.
    ```
  - the two edited one-liners in `fold.ts` and `server.go`.
- **The cut blocks' history** goes to the report.
- **`--write-ledger` is certain.** On the sketch alone, three production rows
  fall more than a point (P10): `apply.go` 37.91 against 40.8, `fold.ts` 41.13
  against 42.4 and `internal/mapdef/load.go` 49.23 against 50.9. The ruleset
  arc predicted "no ledger change" from its probe and was wrong, because the
  final tests were longer than the sketch's. So this plan predicts from each
  test file's capacity (P10) against D9's tests:
  - **Lowered, almost surely:** `apply_boundary_test.go`, which absorbs 7
    lines against an at-cap test of about 20, and `internal/mapdef/load_test.go`,
    13 against about 17.
  - **Likely lowered:** `apply_test.go`, 106 against about 110 to 130, and
    `server_test.go`, 96 against about 100.
  - **Not lowered:** `internal/adventure/load.go` (32.30 against 32.5) and
    `internal/adventure/load_test.go` (20 against about 8).
  - **New files** (QA files, the fixtures' Go files if any) gain rows. A
    missing row is no finding.

  Task 10 runs it once on the final tree and reads its output row by row. No
  row is raised.

**D15. One code commit and the report (Q8).** Forced by measurement:
- The folds land together (rule 4).
- The loaders cannot trail the fold: a map or an adventure that boots and is
  then refused at `load_map` or `load_adventure` breaks the second rule.

**C1** holds:
- the code, the tests and the fixtures;
- the rebuilt `cmd/vtt/webdist/assets/index.js` (`task build:client`);
- under Q5(b), the regenerated files and the golden lines;
- SPEC-018 and SPEC-014, the rows and the debt edit;
- the keys and the ledger.

**C2** holds the report.

**D16. Phase 4a for C1.** One QA agent per `qa-prompt.md`. It is never given
the diff, the source, the existing tests or the implementer's report. It is
given:
- the accepted rows, VTT-161 and VTT-162;
- SPEC-018 whole;
- SPEC-014's lookup, refusal and consequences paragraphs;
- SPEC-013's command path and validators paragraph;
- SPEC-016's `transitions` paragraph;
- the text of these messages: `Envelope`, `Actor`, `ActorAdded`,
  `ActorControlGranted`, `ActorControlRevoked`, `SceneObject`, `SceneCreated`,
  `SessionStarted`, `TokenMoved`, `GrantActorControl`, `RevokeActorControl` and
  `AddActor`;
- `go doc -all` of `./internal/engine`, `./internal/gateway`,
  `./internal/campaign`, `./internal/identity`, `./internal/mapdef` and
  `./internal/adventure`;
- `grep -n '^export' client/src/fold.ts client/src/state.ts`;
- the `add_actor`, `grant_actor_control` and `revoke_actor_control` entries of
  `tools.json`;
- as committed format examples, which are data:
  `internal/mapdef/testdata/valid/cellar.json` and
  `internal/adventure/testdata/cellar-adv/`.

It is told that an adventure scene's object must declare `art`, or its load is
refused for that instead (P5).

It writes these files, each importing only what `.go-arch-lint.yml` allows its
package:
- `internal/engine/qa_cmd_id_bound_test.go` (`engine_test`: `engine`,
  `contract`);
- `internal/gateway/qa_cmd_id_bound_test.go` (`gateway`, `campaign`,
  `engine`, `identity`, `contract`, `mapdef`, `adventure`, `rules`; never
  `store`);
- `internal/mapdef/qa_cmd_id_bound_test.go` (`mapdef`, `contract`, `artlib`;
  never `engine`);
- `internal/adventure/qa_cmd_id_bound_test.go` (`adventure`, `rules`,
  `mapdef`, `engine`, `contract`);
- `client/test/qa-cmd-id-bound.test.ts` (`client/src`, `contract/gen/ts`);
- only if it tests a log that no longer opens,
  `internal/campaign/qa_cmd_id_bound_test.go` (`campaign`, `store`, `engine`,
  `contract`, `eventgen`).

Before it reports, it runs on each package it touched:
- `go vet` and `golangci-lint run` (every helper begins with `t.Helper()`);
- **`go-arch-lint check`**;
- `semgrep scan --config .semgrep/ --error --quiet internal/ cmd/`, with no
  `hp` or other listed word in any fixture;
- `bunx tsc --noEmit -p client/tsconfig.json`;
- its own tests.

It builds every fixture in `t.TempDir()` and commits none. It runs no command
that writes outside its files: no formatter, no `python3 -m json.tool a b`, no
glob as an output argument. The tree is hashed before and after.

Adjudications go in the report, one line per finding. An escape goes to the
debt file as a recipe. A QA test that pins an ABSENCE no row states is dropped:
a bound on `module_data`, on an envelope's `event_id`, or on an object's
`kind`, for example, or a check of a referring id by length.

**D17. Phase 4b for C1, after 4a.** One reviewer at high effort, given `git diff
HEAD`, the untracked files, this plan, the draft message and the break script.
It is briefed to:
- verify by command every sentence C1 adds or rewrites:
  - SPEC-018 and SPEC-014;
  - the debt entry;
  - the two cut docs and the two edited one-liners;
  - the renamed TS test;
  - the new `guide.md` files and the `at-every-boundary` row;
  - under Q5(b), the three descriptions;
- re-read every re-pointed key at its statement, the nine the TS self-test
  cannot see included;
- check the break lines in the draft message against the script's output;
- write nothing: no formatter, no `json.tool` with two arguments. The tree is
  hashed before and after.

If the reviewer dies on a model limit, say so and re-dispatch the same brief
on `fable`.

**D18. The breaks: exactly the K table, and none named elsewhere.**
- **When and where.** The breaks run before C1, in a scratch clone holding the
  files C1 will commit, against the real tests. The ruleset report records
  that order as the process's: the commit's message carries the result.
- **Which.** K1 to K52. Some exist only under a question:
  - K13, K14, K33 and K36 under Q3(c);
  - K43 and K44 under Q3(b) or (c);
  - K16 to K19 and K37 to K40 under Q4(a);
  - K50 to K52 under Q5(b).
- **The script** is written from this table alone. It asserts each K's
  expected reds **by name, every one listed**, and other reds may appear.
- **Each break** is one edit, made after every gate is clean. Each file is
  restored from its saved text and checked by hash.
- **K3 and K25** move a three- or four-line check, which is one edit each.

| # | Edit | Red, at least |
|---|---|---|
| K1 | `controlTarget`'s length check removed | `TestAParticipantIDThatExceedsTheBoundIsRefused`, `TestAParticipantIDIsMeasuredBeforeItsActorIsLookedUp`, `TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing` |
| K2 | `controlTarget`'s `>` → `>=` | `TestCommandAndFileIDsAtCapAreAccepted`, `TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing` (its 128 half) |
| K3 | `controlTarget`'s length check moved after the actor lookup | `TestAParticipantIDIsMeasuredBeforeItsActorIsLookedUp` |
| K4 | Go module-id check removed | `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused/module_id`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing` |
| K5 | Go module-id `>` → `>=` | `TestCommandAndFileIDsAtCapAreAccepted`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing` |
| K6 | Go resource-name check removed | `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused/resource_name`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused/the_longest_key_is_named`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing` |
| K7 | Go resource-name `>` → `>=` | `TestCommandAndFileIDsAtCapAreAccepted`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing`, `TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts` |
| K8 | Go attribute-name check removed | `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused/attribute_name`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing` |
| K9 | Go attribute-name `>` → `>=` | `TestCommandAndFileIDsAtCapAreAccepted`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing`, `TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts` |
| K10 | `longestKey`'s `max` → `min` | `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused/resource_name`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused/attribute_name`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused/the_longest_key_is_named`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing` |
| K11 | Go object-id length check removed | `TestASceneObjectWhoseIDExceedsTheBoundIsRefused`, and under Q3(c) `TestAnEmptyOrRepeatedSceneObjectIDIsRefused/empty` |
| K12 | Go object-id `>` → `>=` | `TestCommandAndFileIDsAtCapAreAccepted` |
| K13 | Go `n == 0 \|\|` removed from the object check | `TestAnEmptyOrRepeatedSceneObjectIDIsRefused/empty` |
| K14 | Go repeated-object check removed | `TestAnEmptyOrRepeatedSceneObjectIDIsRefused/repeated` |
| K15 | Go object id measured in runes (`len([]rune(…))`) | `TestASceneObjectWhoseIDExceedsTheBoundIsRefused` (`é`×64 + `o`, 65 runes) |
| K16 | Go session-id check removed | `TestASessionWhoseIDExceedsTheBoundIsRefused` |
| K17 | Go session-id `>` → `>=` | `TestCommandAndFileIDsAtCapAreAccepted` |
| K18 | Go move-scene-id check removed | `TestAMoveWhoseSceneIDExceedsTheBoundIsRefused` |
| K19 | Go move-scene-id `>` → `>=` | `TestCommandAndFileIDsAtCapAreAccepted` |
| K20 | `engine`'s `maxIDBytes` 129 | This change's: `TestAParticipantIDThatExceedsTheBoundIsRefused`, `TestAParticipantIDIsMeasuredBeforeItsActorIsLookedUp`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused`, `TestASceneObjectWhoseIDExceedsTheBoundIsRefused`, `TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing`, `TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing`; under Q3(c) `TestAnEmptyOrRepeatedSceneObjectIDIsRefused`; under Q4(a) `TestASessionWhoseIDExceedsTheBoundIsRefused`, `TestAMoveWhoseSceneIDExceedsTheBoundIsRefused`; under Q5(b) `TestTheToolsStateTheFoldsBoundOnAnActorsModuleAndKeys`. Earlier arcs': `TestASceneWhoseIDExceedsTheBoundIsRefused`, `TestAnActorWhoseIDExceedsTheBoundIsRefused`, `TestATokenWhoseIDExceedsTheBoundIsRefused`, `TestAnAdventureWhoseIDExceedsTheBoundIsRefused`, `TestAnEmptySceneTokenOrAdventureIDIsRefused`, `TestAConditionWhoseIDExceedsTheBoundIsRefused`, `TestAnAbilityWhoseIDExceedsTheBoundIsRefused`, `TestAnEmptyConditionOrAbilityIDIsRefused`, `TestAConditionIDIsMeasuredBeforeItsActorIsLookedUp`, `TestQARuleTheGoFoldChecksAConditionIDBeforeItsActorAndADuplicate`, `TestQARuleTheGoFoldChecksAnAbilityUsedForItsIDAlone`, `TestQARuleTheGoFoldRefusesAnEmptyID`, `TestQARuleTheGoFoldRefusesAnIDOverTheBoundNamingFieldBoundAndLength`, `TestTheToolsStateTheFoldsIDBound` |
| K21 | `engine`'s `maxIDBytes` 127 | This change's: `TestCommandAndFileIDsAtCapAreAccepted`, every K20 refusal of this change (their text), both wire tests. Earlier arcs': `TestIDsAtCapAreAccepted`, `TestConditionAndAbilityIDsAtCapAreAccepted`, `TestARulesetAtTheBoundResolvesToEventsTheFoldAccepts`, `TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts`, `TestQARuleTheGoFoldAcceptsAnIDUpToTheBound`, and K20's earlier-arc list but `TestQARuleTheGoFoldChecksAConditionIDBeforeItsActorAndADuplicate` |
| K22 | Go participant refusal reworded | `TestAParticipantIDThatExceedsTheBoundIsRefused`, `TestAParticipantIDIsMeasuredBeforeItsActorIsLookedUp`, `TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing` |
| K23 | TS participant `checkLen` removed | "a participant id longer than 128 bytes is rejected in a grant", "… in a revoke", "a participant id is measured before its actor is looked up" |
| K24 | TS participant maximum 127 | the three of K23 (their text), "command and file ids of exactly 128 bytes are ACCEPTED" |
| K25 | TS participant `checkLen` moved after the actor lookup | "a participant id is measured before its actor is looked up" |
| K26 | TS module `checkLen` removed | "a module id longer than 128 bytes is rejected" |
| K27 | TS module maximum 127 | "a module id longer than 128 bytes is rejected", "command and file ids of exactly 128 bytes are ACCEPTED" |
| K28 | TS resource loop removed | "a resource name longer than 128 bytes is rejected" |
| K29 | TS resource maximum 127 | "a resource name longer than 128 bytes is rejected", "command and file ids of exactly 128 bytes are ACCEPTED" |
| K30 | TS attribute loop removed | "an attribute name longer than 128 bytes is rejected" |
| K31 | TS attribute maximum 127 | "an attribute name longer than 128 bytes is rejected", "command and file ids of exactly 128 bytes are ACCEPTED" |
| K32 | TS object `checkLen` removed | "an object id over 128 UTF-8 bytes is rejected", and under Q3(c) "an empty object id is rejected" |
| K33 | TS object minimum 0 | "an empty object id is rejected" |
| K34 | TS object maximum 127 | "an object id over 128 UTF-8 bytes is rejected", "command and file ids of exactly 128 bytes are ACCEPTED" |
| K35 | TS object id measured with `.length` (characters) | "an object id over 128 UTF-8 bytes is rejected" (`é`×64 + `o`, 65 characters) |
| K36 | TS repeated-object throw removed | "a repeated object id is rejected" |
| K37 | TS session `checkLen` removed | "a session id longer than 128 bytes is rejected" |
| K38 | TS session maximum 127 | "a session id longer than 128 bytes is rejected", "command and file ids of exactly 128 bytes are ACCEPTED" |
| K39 | TS move-scene `checkLen` removed | "a move's scene id longer than 128 bytes is rejected" |
| K40 | TS move-scene maximum 127 | "a move's scene id longer than 128 bytes is rejected", "command and file ids of exactly 128 bytes are ACCEPTED" |
| K41 | `CheckObjectIDs`' length check removed | `TestInvalidMapsAreRefusedWithAUsefulReason/object-id-too-long`, `TestLoadInvalidFixtures/object-id-too-long` |
| K42 | `CheckObjectIDs`' `>` → `>=` | `TestAnObjectIDOfExactlyTheBoundLoads`, `TestLoadAcceptsValuesExactlyOnEveryLimit` |
| K43 | `CheckObjectIDs`' empty check removed | `TestInvalidMapsAreRefusedWithAUsefulReason/object-id-empty`, `TestLoadInvalidFixtures/object-id-empty` |
| K44 | `CheckObjectIDs`' repeat check removed | `TestInvalidMapsAreRefusedWithAUsefulReason/duplicate-object-id`, `TestLoadInvalidFixtures/duplicate-object-id` |
| K45 | `loadAs`' call to `CheckObjectIDs` removed | `TestInvalidMapsAreRefusedWithAUsefulReason/object-id-too-long`, and under Q3(b)/(c) `…/object-id-empty` and `…/duplicate-object-id` |
| K46 | `loadScenes`' call to `mapdef.CheckObjectIDs` removed | `TestLoadInvalidFixtures/object-id-too-long`, and under Q3(b)/(c) `…/object-id-empty` and `…/duplicate-object-id` |
| K47 | `internal/mapdef`'s `maxIDBytes` 129 | `TestInvalidMapsAreRefusedWithAUsefulReason/id-too-long`, `…/placement-token-id-too-long`, `…/object-id-too-long`, `TestAnInstalledMapWhoseIDExceedsTheBoundIsRefused`, `TestQAIDAMapWhoseIDIsOverTheBoundIsRefusedNamingFileAndField`, `TestQAIDAMapPlacementTokenIDOverTheBoundIsRefusedNamingFileAndField`, `TestLoadInvalidFixtures/object-id-too-long` |
| K48 | `internal/mapdef`'s `maxIDBytes` 127 | `TestAMapIDOfExactlyTheBoundLoads`, `TestAnObjectIDOfExactlyTheBoundLoads`, `TestAnInstalledMapWhoseIDExceedsTheBoundIsRefused`, `TestInvalidMapsAreRefusedWithAUsefulReason/id-too-long`, `…/placement-token-id-too-long`, `…/object-id-too-long`, `TestLoadAcceptsValuesExactlyOnEveryLimit`, `TestLoadInvalidFixtures/object-id-too-long` |
| K49 | `at-every-boundary`'s object id shortened to 127 | `TestLoadAcceptsValuesExactlyOnEveryLimit`'s new assertion (not run: the fixture does not exist yet) |
| K50 | `add_actor`'s `moduleId` description says 200, regenerated | `TestTheToolsStateTheFoldsBoundOnAnActorsModuleAndKeys`, `TestAddActorStatesTheBoundOnItsModuleAndKeys` (not run) |
| K51 | its `attributes` description says 200, regenerated | the same two (not run) |
| K52 | its `resources` description says 200, regenerated | the same two (not run) |

Go subtest names print with spaces as underscores; the script matches either
form. The TS names in the red column are D9's TS cases. K20's and K21's
earlier-arc reds are the ones P12 saw, and the script lists them by name.

**D19. Disk and load.** Forced by `check:mutation`'s 16 GiB floor, by MCP e2e
deadlines under load, and by this machine's 8 GiB of memory. Immediately before
`task check`:
- run `go clean -cache`, `df -k` and `python3 -c "import os;
  print(os.getloadavg())"`;
- launch the gate once after C1, in its own session
  (`start_new_session=True`);
- below 16 GiB free, stop; above a load of about 8, wait.

## Tasks, in dependency order

### Task 0 — Baselines

**Done when:** `df -k`, the load, the chain, comments and doc-owner baselines
and both self-tests match P13, and `requirement-id` resolves.

### Task 1 — Rows

**Files:** `docs/requirements.md`. After sign-off, run `requirement-id` once
per accepted row, in letter order. **Done when:** that many new OPEN rows
exist.

### Task 2 — The Go fold

**Files:** `internal/engine/apply.go`, `apply_test.go`,
`apply_boundary_test.go`.

D9's engine tests come first, and they build on the base. The refusals are red
with `err = <nil>`, and the participant-order test with the unknown actor's
text. Then D4 and D5. **Done when:**
- `go test -count=1 ./internal/engine/ ./internal/rules/...` is green;
- `go test -count=1 -p 2 ./internal/...` shows nothing else red (P6);
- `gocyclo -over 30 internal/engine/apply.go` prints only `Apply`;
- `golangci-lint run ./internal/engine/` reports 0 issues;
- semgrep's vocabulary rule passes.

### Task 3 — The gateway

**Files:** `internal/gateway/server_test.go`, and `internal/gateway/server.go`
(D8's comment only).

D9's two wire tests are red on the base (`ok=true`, the head moving) and green
on Task 2's tree. **Done when:** `go test -count=1 ./internal/gateway/` is
green, and `git diff internal/gateway/server.go` shows the comment's lines
alone.

### Task 4 — The TS mirror

**Files:** `client/src/fold.ts`, `client/test/fold-rejections.test.ts`,
`client/test/fold-unit.test.ts` (the retitle), `cmd/vtt/webdist/`.

The cases come first, then D6, then `task build:client`. **Done when:**
- `bun test client/test` is green;
- `client:typecheck` is clean;
- `git status` shows `index.js` as the only webdist change.

### Task 5 — The loaders

**Files:**
- `internal/mapdef/load.go` and `load_test.go`, and the new
  `testdata/invalid/` directories;
- `internal/adventure/load.go` and `load_test.go`, the new `testdata/invalid/`
  directories with their `guide.md` files, `at-every-boundary`'s
  `scenes/pin.json` and `guide.md`.

The fixtures and rows come first (red: the base loads them), then D7.
**Done when:**
- `go test -count=1 ./internal/mapdef/ ./internal/adventure/...` is green;
- `go test -count=1 ./cmd/vtt/` is green, so shipped content boots;
- `gocyclo -over 30 internal/mapdef/load.go internal/adventure/load.go` prints
  nothing;
- `golangci-lint run ./internal/mapdef/ ./internal/adventure/...` reports 0
  issues.

### Task 6 — The tools (under Q5(b))

**Files:** `tools/toolgen/main.go` and `main_test.go`, both `tools.json` files,
`contract/testdata/expected_tools.json`, and
`internal/engine/id_bound_internal_test.go`.

D10's two tests come first (red), then the `fieldDocs`, `task
generate:contract` and the golden's three lines. **Done when:** `go test
./tools/toolgen/ ./internal/engine/ ./internal/mcp/ ./contract/` is green, and
`git status` shows exactly the two `tools.json` files regenerated.

### Task 7 — The records

**Files:** SPEC-018, SPEC-014, `docs/requirements.md` (the new rows' evidence;
VTT-161 and VTT-162), `docs/verification-debt.md`.

D11. **Done when:**
- the chain prints 288 rows plus the new ones and `13 specifications`, with
  none of the new rows OPEN;
- `grep -c 'Sixteen fields\|These sixteen\|of the sixteen'
  docs/specifications/018-*` prints 0;
- `grep -c 'sixteen bounds' docs/verification-debt.md` prints 0.

### Task 8 — Local gates

Run:
- `gofmt -l` over the touched Go files only;
- `go vet` and `task lint`;
- `go test -count=1 -p 2 ./internal/... ./contract/... ./tools/... ./cmd/...`;
- `bun test client/test contract contract-spike`;
- semgrep, `go-arch-lint check`, `check:comments`, `check:doc-owner` and
  `check:new-prose`.

`check:comments` is expected to report D14's falling rows here. They are
recorded in Task 10, and every other finding is fixed now. **Done when:** each
prints its own completion line, the expected ledger findings aside. A Go
failure is captured whole and re-run in isolation before it is called a flake.

### Task 9 — Phase 4a, then 4b

D16 and D17. Findings are fixed, and the affected task's "done" is re-run. The
review settles before Task 10.

### Task 10 — Keys and ledger

D13, then `python3 tools/check-comments.py --write-ledger`, its output read row
by row against D14's prediction. **Done when:**
- both self-tests print `OK`, and every key reads its statement;
- `python3 tools/check-comments.py main` is clean;
- `git diff tools/comment-ceilings.txt` lowers or adds rows and raises none.

### Task 11 — The breaks

D18, in a scratch clone holding the files C1 will commit, its `git diff HEAD
--stat` matching the tree's. **Done when:** each break gives every red its K
row names, and the script's output is saved for the message.

### Task 12 — Commit C1

C1's message lists the ids and D18's lines. **Done when:** `git show --stat
HEAD` lists C1's files, and `task check:drift` is clean.

### Task 13 — The whole gate

D19, then `task check` once. **Done when:** it exits 0 with every step's own
verdict, `check:mutation` and `check:ts-mutation` included.

### Task 14 — The report

`docs/reports/2026-10-07-a-commands-and-a-maps-ids-are-bounded.md`, per the
`implementation-report` skill. It covers:
- each Done item with its observation;
- the rows and refusals;
- the rulings taken at sign-off;
- the rule-9 answer;
- the breaks;
- the history D5 and D6 removed from comments;
- D3's lists of what stays unbounded, raised to the owner;
- Gap 10;
- the gaps.

**Done when:** C2 holds it alone. Push after C2, and let the pre-push hook
finish.

## Gaps that travel with this plan

1. **An adventure's scenes carry objects, and the ticket names only a map
   file.** `loadScenes` decodes them as `mapdef.ObjectJSON`, and
   `adventure.Compile` sends them through `mapdef.BuildSceneCreated` into the
   same `SceneCreated`. Done item 3, the second rule and "What it touches" name
   only `internal/mapdef`. D7, row F and Task 5 cover the adventure path.
2. **"Since `campaign` stamps the first and no writer sets the second, so only
   a log written by hand could carry either" is false in its middle clause.**
   `handleCommand` backfills a `TokenMoved`'s `SceneId` from the snapshot's
   token (SPEC-013's step 7, VTT-160), and eventgen and many tests set it too.
   The conclusion survives: the backfilled id is a folded token's scene id,
   which VTT-277 holds at 128 bytes, so only a hand-written log carries a long
   one.
3. **"What it touches" omits work that Done item 5 needs:**
   - `internal/adventure/load.go`, its tests and three fixture directories,
     and `at-every-boundary` (Gap 1);
   - the cut of `controlTarget`'s 12-line doc and `requireControlTarget`'s
     7-line JSDoc (D5, D6);
   - `server.go`'s backfill comment, `fold.ts`'s `tokenMoved` comment and
     `fold-unit.test.ts`'s test title (D6, D8);
   - semgrep's vocabulary rule on test fixtures (P6);
   - under Q5(b), the toolgen test and the link test.
4. **The at-bound halves of Done items 1 and 2 cannot fail today**, as the
   ticket says. The `>=` breaks hold them (check 2).
5. **A campaign whose log already holds such a value over 128 bytes, or an
   empty or repeated object id, stops opening.** The same goes for a map that
   carries one: it stops booting. No shipped content, fixture, golden, eventgen
   draw or test fold comes within 91 bytes of the bound or holds such an
   object id (P2 to P4), save the four 128-byte keys folded on purpose. Nobody
   uses the product (the owner, 2026-09-04).
6. **Nothing links `fold.ts`'s literals or `internal/mapdef`'s `maxIDBytes` to
   the engine's constant** beyond each side's tests. The open debt entry is
   amended rather than closed (D11).
7. **The TS self-test cannot see nine of the fifteen moved `fold.ts` keys**
   (P9). Phase 4b's re-read is the check.
8. **SPEC-018 states orders that only one test holds.** For example, a module
   id is checked after the name, and a move's scene id after its reason. As in
   both earlier arcs, only the participant order has a test (K3, K25). A
   refused event is refused either way; only the text differs.
9. **The mutation gates were not run** (8 GiB of memory, swap nearly full
   today). D13's kills are predicted from the sketches' reds (P12), and Task
   13's `task check` is the measurement.
10. **Found in passing, not this ticket's:** the adventure loader's dry run
    refuses a scene object with no `art` under the wrong field:
    `field "overrides": mapdef: objects[0] has no art …` (P5). The report
    raises it.
11. **The adventure loader's refusal of an undeclared actor key quotes the key
    at any length**, at boot only. It is unchanged here, as the ruleset plan's
    D7 left it (D3's table).

## Questions for sign-off

1. **The number and its constant.**
   (a) 128, the fold's own `maxIDBytes`, for all eight fields, with
   `internal/mapdef`'s existing copy bounding object ids at load;
   (b) participant ids at 32, identity's length, and the rest at 128;
   (c) a constant of its own for an actor's keys.
   Recommend (a) (D1). The keys must be at least the ruleset's names, which
   are already held at `maxIDBytes`, and one constant keeps them equal.
   (b) writes identity's id format into the engine, which may not import
   identity. Apart from the ruleset arc's four 128-byte keys, folded on
   purpose, nothing in the tree comes within 91 bytes.
2. **Empty values other than an object id.**
   (a) An empty module id, session id, move scene id and actor key are
   admitted, and an empty participant id stays refused as today;
   (b) as (a), but also refuse an empty resource or attribute name.
   Recommend (a) (D2). Measured: 902 and 11,558 Go folds of an empty session
   id and move scene id. A map key `""` is present on the wire, unlike an
   empty id field. No ruleset declares an empty name, and nothing folds an
   empty key, so (b) would break nothing. But it adds a second refusal over an
   unordered map, whose order both folds must then fix.
3. **Object ids.**
   (a) length only, in both folds and both loaders;
   (b) (a), and an empty or repeated object id refused at load;
   (c) (b), and both folds refuse an empty or repeated one too.
   Recommend (c) (D2, D4, D7). `mergeObjects` and the TS `sceneSeen` arm merge
   a viewer's objects by id. A scene with two objects sharing an id therefore
   shows a player only one of them, while the server's sight is blocked by
   both. That is a blocking object the player cannot see; this is a reading of
   `mergeObjects` and of SPEC-016's `sceneSeenFor`. Nothing in the tree has an
   empty or repeated object id. (c) matches your rulings on the id arc's
   Q2(b) and the ruleset arc's Q4(b), that ids are refused empty in both
   folds.
4. **A session id and a move's scene id.**
   (a) bound both in both folds, empty admitted;
   (b) no fold bound, and SPEC-018 records each one's only writer.
   Recommend (a). You named both, and only a hand-written log can carry a long
   one. But the fold decides what a log may hold forever, and each costs one
   check per fold. Every other envelope's session id stays unbounded either
   way (D3).
5. **Do the tools state the bound?**
   (a) no;
   (b) `add_actor`'s `moduleId`, `attributes` and `resources` do;
   (c) (b), and `grant_actor_control`'s and `revoke_actor_control`'s
   `participantId` too.
   Recommend (b) (D10). The agent writes those three, and every other
   `add_actor` field the fold bounds already says so (VTT-276, VTT-281). The
   agent copies a participant id from identity rather than choosing one.
6. **Should `grant_actor_control` refuse a participant identity does not
   know?**
   (a) not here;
   (b) yes, in this ticket.
   Recommend (a). `identity.Lookup` gives a revoked participant the same error
   as an unknown one, so a revoke checked the same way would strand control of
   a departed player's character. Revocation exists for exactly that
   reassignment. SPEC-009 records that "Control of a character is not
   identity's". A grant-only check would be a new rule with its own failure
   modes, so it belongs in a ticket of its own if you want it.
7. **The other strings a map file carries:** an object's `kind` and `art`, a
   tile's kind, material and art, and a placement's `actor_id`.
   (a) out of scope, raised with the report;
   (b) in scope.
   Recommend (a) (D3's table). Art names already have `artlib`'s bound, a
   rendered kind is clipped, and tile names are a closed set. A placement's
   actor id is held at 128 by the fold's lookup, but its `load_map` refusal
   quotes it at any length. That is a refusal-size question, like the
   `use_ability` debt, and not an id bound.
8. **Rows A to H, and one code commit plus the report?** Under every declined
   option, rows A, B, C and F remain. VTT-161 and VTT-162 gain the wire tests.
   Recommend yes (D12, D15). The folds cannot land apart, and the loaders
   cannot trail them.
