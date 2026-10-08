# A command's and a map's ids are bounded: the change

**Ticket:** `docs/superpowers/specs/2026-10-07-a-commands-and-a-maps-ids-are-bounded-design.md`,
revised by its writer after verification and after sign-off (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-07-a-commands-and-a-maps-ids-are-bounded.md`,
verified by `verify-ticket` (passes with gaps).
**The owner's rulings:** on 2026-10-06, a split of the id-like strings into
two tickets by source, the ruleset's first and this one second; at sign-off, a
bound of 128 bytes through the fold's own `maxIDBytes` for every field
(Q1(a)); an object id refused empty or repeated in both folds and both loaders
(Q3(c)); the session id and the move's scene id bounded in both folds (Q4(a),
whose "empty admitted" the Q2 ruling below overrode); and, in the owner's
words "resten ok", the rest as the plan proposed: `add_actor` stating the
bound (Q5(b)), no identity check on a grant (Q6(a)), the map's other strings
out of scope (Q7(a)), and the rows and one code commit plus the report (Q8).
On Q2, the owner asked why empty ids should be admitted at all, and on a
re-asked Q2 whose recommended option was refusal ruled every one of the fields
refused empty except `module_id`, where the plan had recommended admitting
them. On the review: every finding fixed. On the report's review: every
finding fixed.
**Last code commit:** `53188d2`, the gate change that followed `6b5585d`, on
`9760d23`, `main` at the time. Every code reference below is to that tree.

## The period, in commits

    git log --oneline 9760d23..53188d2

    53188d2 Stryker's test command bails and prints only failures
    6b5585d A command's and a map's ids are bounded

`git diff --stat 9760d23..53188d2`: 78 files changed, 5060 insertions(+), 234
deletions(-).

The gate, `task check` whole, after `53188d2`: it exited 0 with no step
failed. A first run, after `6b5585d`, failed at its last step (Deviations).
The second run's check steps' own verdict lines read `check:comments` clean
over 278 files, `check:requirements-chain` 297 rows, 239 test files and 13
specifications, `check:doc-owner` 80 files, `check:new-prose` 3078 added lines
clean, `check:coverage` 20 packages at or above their floors
(`internal/engine` 99.4 %, `internal/mapdef` 98.9 %, `internal/gateway` 94.8 %
and `internal/adventure` 92.1 %), `check:ts-coverage` 22 of 23 source files
gated and one excluded, `check:no-pack`, `check:no-retraction` and
`check:no-create-scene` clean over 1343 files, `task lint` 0 issues,
`check:breaking` reporting pre-release with no objection, `check:mutation` 14
packages with zero unadjudicated survivors (six mutants timed out in
`internal/sight`, `internal/rules` and `internal/mcp` and counted as killed),
and `check:ts-mutation` 2944 mutants, 2868 killed, 67 survivors all
adjudicated equivalent, zero unadjudicated, nine timed out and counted as
killed (`UpdateOperator`s in `client/src/view/canvas.ts` and
`client/src/view/scene-plan.ts` and a `BlockStatement` in
`client/src/wire.ts`; the failed run had 93), Stryker's own run taking 32
minutes and 42 seconds and writing a 36.8 MB report. `check:mutation` began
with 33.2 GiB free; `task check:drift` exited 0 after `6b5585d`.

## Done looks like, answered

1. `[x]` `engine.Apply` refuses an `ActorControlGranted` or
   `ActorControlRevoked` whose participant id, an `ActorAdded` whose actor's
   `module_id` or one of whose `resources` or `attributes` keys, a
   `SceneCreated` one of whose objects' ids, a `SessionStarted` whose
   envelope's session id, and a `TokenMoved` whose scene id is longer than 128
   bytes, and each of them empty but the `module_id`, and a `SceneCreated` two
   of whose objects share an id:
   `internal/engine/apply_test.go#TestAParticipantIDThatExceedsTheBoundIsRefused`,
   `#TestAParticipantIDIsMeasuredBeforeItsActorIsLookedUp`,
   `#TestAnActorWhoseModuleIDOrKeyExceedsTheBoundIsRefused`,
   `#TestASceneObjectWhoseIDExceedsTheBoundIsRefused`,
   `#TestAnEmptyOrRepeatedSceneObjectIDIsRefused`,
   `#TestASessionWhoseIDExceedsTheBoundIsRefused`,
   `#TestAMoveWhoseSceneIDExceedsTheBoundIsRefused` and
   `#TestAnEmptySessionMoveSceneOrActorKeyIsRefused`, each red on `9760d23`'s
   source (`err = <nil>`, or the unknown actor's text before the id's); and
   `internal/engine/apply_boundary_test.go#TestCommandAndFileIDsAtCapAreAccepted`,
   green there and held by K2, K5, K7, K9, K12, K17, K19 and K21. An empty
   participant id was refused before this change. QA's
   `internal/engine/qa_cmd_id_bound_test.go`.
2. `[x]` `client/src/fold.ts` refuses the same events at the same byte counts:
   fifteen cases in `client/test/fold-rejections.test.ts`, from `#a
   participant id longer than 128 bytes is rejected in a grant` to `#an empty
   attribute name is rejected`, each red on `9760d23`; and `#command and file
   ids of exactly 128 bytes are ACCEPTED`, held by K24, K27, K29, K31, K34,
   K38 and K40. QA's `client/test/qa-cmd-id-bound.test.ts`.
3. `[x]` A map file or an adventure one of whose scene objects' id is over the
   bound, empty or shared with another of its objects is refused when it
   loads, naming the file and the field (`objects[<i>].id`):
   `internal/mapdef/load_test.go#TestInvalidMapsAreRefusedWithAUsefulReason`'s
   `object-id-too-long`, `object-id-empty` and `duplicate-object-id` rows, red
   on `9760d23`'s source (the map was accepted), and
   `internal/adventure/load_test.go#TestLoadInvalidFixtures`' three rows of
   the same names, red there (`want error, got nil`); the at-bound half,
   `internal/mapdef/load_test.go#TestAnObjectIDOfExactlyTheBoundLoads` and
   `internal/adventure/load_test.go#TestLoadAcceptsValuesExactlyOnEveryLimit`,
   held by K42, K48 and K49. QA's `internal/mapdef/qa_cmd_id_bound_test.go`
   and `internal/adventure/qa_cmd_id_bound_test.go`.
4. `[x]` A `grant_actor_control`, `revoke_actor_control` or `add_actor`
   carrying such a string is answered ok=false and appends nothing:
   `internal/gateway/server_test.go#TestAControlCommandWhoseParticipantIDExceedsTheBoundAppendsNothing`
   and `#TestAnActorWhoseModuleIDOrKeyExceedsTheBoundAppendsNothing`, red on
   `9760d23`'s source (`grant-long: ok=true` in the control test, `module:
   ok=true` in the actor test); each then sends a 128-byte command on the same
   connection and sees it accepted. QA's
   `internal/gateway/qa_cmd_id_bound_test.go`.
5. `[x]` Every golden, fixture and shipped content file still loads and folds,
   and `task check` whole is green:
   `internal/harness/fold_golden_test.go#TestFoldGoldenCorpus` folds every
   golden stream through the Go fold and `client/test/fold-parity.test.ts#fold
   parity: <name>` through `fold.ts`;
   `internal/adventure/conformance/conformance_test.go#TestConformanceOverAdventuresGlob`
   loads every shipped adventure and
   `internal/rules/conformance/conformance_test.go#TestConformanceOverRulesetsGlob`
   every shipped ruleset;
   `internal/gateway/map_test.go#TestTheShippedCampaignResolvesItsOwnArt`
   loads `campaigns/example/maps/cellar.json` through `mapdef.Load`, and
   `cmd/vtt/library_test.go#TestScenarioLibraryRunsSelfContained` boots every
   scenario, preloading and validating the maps it names from
   `scenarios/maps/`. No fold golden and no shipped content file changed; the
   `at-every-boundary` fixture gained an object whose id is 128 bytes, and
   `contract/testdata/expected_tools.json` carries the three new descriptions.
   Tests that built a `SessionStarted` with no session id or a move with no
   scene id were given one (Deviations). The gate paragraph above records the
   gate, in which all of them ran.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A participant id in a control event, an actor's `module_id` and the keys of its `resources` and `attributes`, a scene object's id, a session id and a `TokenMoved`'s scene id are at most a fixed number of UTF-8 bytes, in both folds. | VTT-289 (participant id), VTT-290 (module id and keys), VTT-291 (object id), VTT-293 (session id and move scene id) |
| Each of them but the `module_id` is refused empty in both folds, and a scene's objects' ids are unique in it. | VTT-292 for an object id, VTT-297 for the session id, the move's scene id and an actor's keys; an empty participant id was already refused, which SPEC-018 states (D12's twelfth refusal) |
| A map file or an adventure that carries a scene object id longer than that, empty or repeated is refused at load. | VTT-294, and VTT-295 for the empty and repeated ids |
| The `add_actor` tool states the bound on an actor's `module_id` and its `resources` and `attributes` names. | VTT-296 |
| A command that carries a longer one appends nothing. | no row of its own: VTT-162, and VTT-161 for the open connection, each gaining the two over-the-wire tests (D12's second and third refusals) |

## The sort

Eight rows accepted as D12 proposed (A to H, now VTT-289 to VTT-296), and a
ninth, VTT-297, for the empties the owner's Q2 ruling refused. Twelve
candidates refused, as D12 lists them; none was reversed.

## Phase 4a: QA adjudications

QA on opus, given VTT-289 to VTT-297, VTT-161 and VTT-162, SPEC-018, SPEC-014,
SPEC-013 and SPEC-016 whole, the text of `Envelope`, `Actor`, `ActorAdded`,
`ActorControlGranted`, `ActorControlRevoked`, `SceneObject`, `SceneCreated`,
`SessionStarted`, `TokenMoved`, `GrantActorControl`, `RevokeActorControl` and
`AddActor`, the generated `add_actor`, `grant_actor_control` and
`revoke_actor_control` tool entries, `go doc -all` of `engine`, `gateway`,
`campaign`, `identity`, `mapdef` and `adventure`, `fold.ts`'s and
`state.ts`'s export lines, and `internal/mapdef/testdata/valid/cellar.json`
and `internal/adventure/testdata/cellar-adv/` as format examples. Its brief
named `go-arch-lint check` and the imports each file may use. It wrote
`internal/engine/qa_cmd_id_bound_test.go`,
`internal/mapdef/qa_cmd_id_bound_test.go`,
`internal/adventure/qa_cmd_id_bound_test.go`,
`internal/gateway/qa_cmd_id_bound_test.go`,
`internal/campaign/qa_cmd_id_bound_test.go` and
`client/test/qa-cmd-id-bound.test.ts`: 43 Go tests and 116 TS tests, none
failing; forty-eight injections into its own files, all red; every file
outside its six hashed the same before and after.

- SPEC-013 said `engine.Apply`'s `TokenMoved` arm never reads `SceneId`: false
  since this change, and missed by the ticket's list of prose made false and
  by the plan's check 6. SPEC-013 now says it reads the lengths of `Reason`
  and `SceneId` and never `From`.
- "Then each object in order for its id's length and then a repeat" reads
  two ways, per object or all lengths before all repeats: a spec ambiguity.
  SPEC-018 now says each object in turn, its length and then whether an
  earlier object has the same id, which is what both folds do.
- An empty key beside an over-long one: which length the refusal names was
  unstated. SPEC-018 now says the Go refusal names 0 when any key is empty,
  however long the others are, which `engine.Apply` does; `fold.ts`'s
  `checkKeys` also refuses the empty key first, with `checkLen`'s text, which
  names no length.
- `engine: <event> requires a participant id` left `<event>` unspelled.
  SPEC-018 now names `actor_control_granted` and its `actor_control_revoked`
  twin.
- The TS texts for a repeated object id and an empty participant id are
  unstated: SPEC-018 states `checkLen`'s wording only. No change.
- A loader field's spelling is unstated: the refusals name `objects[<i>].id`,
  as the id arc's rows name their fields. No change.
- `load_adventure`'s refusal text is SPEC-012's, which QA was not given. No
  change.
- No input states the session id `campaign` stamps: `stampSessionIDAgainst`
  writes `sess-` and 32 hex characters, 37 bytes. No change.
- A move's scene id naming its token's scene: not pinned, on purpose, as D12's
  eleventh refusal. No change.
- Requirements QA asked an id for: SPEC-018's refusal form, its order
  sentences, the repeat text, `campaign.Open` and `checkLen`'s wording;
  SPEC-014's naming and on-demand sentences; SPEC-013's validator order and
  backfill; SPEC-016's forwarding. SPEC-018's are its own sentences, held by
  the bound rows as in the two arcs before, and the others are earlier
  records'. None dispensed.

## Phase 4b: the review

One reviewer on opus, given `git diff HEAD`, the untracked files, the plan,
QA's adjudications, the draft commit message, the break script and its log,
and the test-first run. Nothing that breaks at the table in the code: both
folds refuse the same events in the same order, and every production writer
it traced produces what the fold accepts. Its findings, each fixed by the
owner's ruling:
- `docs/map-format.md` §5 told map authors that nothing refuses an empty or
  duplicated object id, where such a map now stops the boot. §5 now states
  the rule, §10 lists it, and §5's pointer to the size rule, which named rule
  5, names rule 7.
- The debt entry on the bounds' copies named two tests that red when the
  engine's bound falls below a loader's copy; three in `internal/adventure`
  red too, and it names all five.
- SPEC-014's consequence left out that an object id may not be empty.
- `keyLength`'s doc described behaviour on an unexported symbol (CLAUDE.md
  rule 10); it keeps only its warning.
- The commit message gave K10's count as eighteen, not twenty-one, named
  `internal/engine` among the packages whose fixtures changed, and said
  without qualification that no production writer sends an empty value.
- SPEC-018's Status named neither `keyLength` nor `checkKeys`; `ToObject`'s
  doc named `CheckObjectFootprints` and not `CheckObjectIDs`; `fold.ts`'s move
  comment described rather than warned, and is removed.

It re-read the nineteen re-pointed keys at their statements and found no other
key that moved (`internal/adventure/load.go`'s one key sits above the edit).
Removing `fold.ts`'s move comment afterwards moved the fifteen `fold.ts` keys
up a line, and they were re-pointed again, to +10 and +11 from `9760d23`; each
reads its statement in `6b5585d`. It checked K1 to K61's script against the
plan and its log, and broke the three repeat-set inserts and `keyLength`'s
empty-map guard itself, each red. Its `vite build`, in its own clone, went
through the `node_modules` symlink and touched `node_modules/.vite-temp`,
which is gitignored and was left empty.

## The breaks

The commit's message carries them, grouped where breaks share a shape, with
the checks that spoke; each was run in a scratch clone holding the files then
staged for the commit, restored from its saved text and checked by hash.
Sixty-one: the plan's K1 to K52, and K53 to K61 for the empty halves the Q2
ruling added. Each asserted its expected reds by name, and every one went red
where the table said. They ran before the review; the review's fixes changed
comments, documents and the message, no statement, and every break's anchor
was checked again against the committed tree, each resolving once.

## Rule 9: how MapTool does this

Answered in the plan, from `~/dev/RPTool/maptool` at `f4b7fef6c`. A token's
owners are player names held in `Token.ownerList`; the `setOwner` macro
(`TokenPropertyFunctions`) adds any string, checked against neither the player
list nor a length, and a player's name is checked only for emptiness at
connect (`ConnectToServerDialog`). A token's property type, the nearest thing
to `module_id`, and its property names are stored at any length. Every
drawable carries a generated `GUID`, and `GUID.valueOf` refuses any length but
16 where one is read off the wire. MapTool has no session id, and its frames
carry any length (`AbstractConnection.readMessage`). Borrowed: an object's id
has one bound, checked where it is read, and is unique within its map; MapTool
gets uniqueness by generation, and here the author writes the id, so the
loaders and the folds check it. Not borrowed: owners, property types and
property names of any length, since the server here reads at most
`maxWSFrameBytes` a frame and the log keeps forever what the fold accepts.
Checked and rejected: generating object ids at compile time, which would
change every compiled scene's golden; checking an owner against the
participant list, which `setOwner` does not do and Q6 declined.

## What the cut comments recorded

`controlTarget`'s doc in `internal/engine/apply.go`, and
`requireControlTarget`'s JSDoc in `client/src/fold.ts` for the same two
refusals, recorded why each exists. An unknown actor is an error rather than a
no-op for the reason `ConditionApplied` and `ConditionRemoved` refuse one: an
event that names something absent leaves the log meaning nothing, and a silent
skip makes the divergence surface later, somewhere unrelated. An empty
participant is refused because `""` in the set would make `controller_ids`
non-empty while `controller_id` mirrors an empty string, reintroducing the "is
this shared or unowned?" ambiguity the mirror rule exists to prevent.
`fold.ts`'s `tokenMoved` comment, that `from` and `sceneId` were ignored as Go
ignores them, became false with this change and was removed, not recorded.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| the ticket as written before verification | its writer added an adventure's scene objects, corrected the sentence on a move's scene id, and listed the prose made false and both loaders; after sign-off, the rules on empty and repeated ids | the verification's Gaps 1 to 3, and Q2 and Q3 |
| D2, Q2(a) and Q4(a): an empty session id, move scene id and actor key admitted | refused in both folds; VTT-297, K53 to K61 | the owner's Q2 ruling, after asking why empty ids should be admitted at all, and on a re-asked Q2 whose recommended option was refusal |
| D2: "An empty session id is folded on 902 Go `SessionStarted`s and 15 TS ones", left as it was under Q2(a) | `SessionId` and `SceneId` given to test envelopes in the gateway, harness and client tests, `internal/gateway/qa_name_bound_test.go` and `client/test/qa-name-bound.test.ts` among them | under Q2 the folds refuse a `SessionStarted` with no session id and a move with no scene id, which those tests built; no golden or content file held either |
| D4's `longestKey` | `keyLength`, returning `(n, ok)`: 0 when any key is empty, else the longest key's length, and `ok` false for an empty map | under Q2 an empty key is refused with length 0 whatever the other keys are, which the longest key's length cannot say; `ok` keeps an actor with no keys apart from one with an empty key |
| the ticket's list of prose made false | SPEC-013's sentence on the `TokenMoved` arm and `docs/map-format.md` §5 and §10 corrected as well, and §5's pointer to the size rule | QA found the first and the review the second; the pointer was wrong before this change and was found while §10 was edited |
| D11: the debt entry adds `TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts` beside the rules test that reds when the engine's bound falls below `internal/rules`' copy | five tests, in `internal/rules` and `internal/adventure` | the review's S1, reproduced by setting the engine's `maxIDBytes` to 127 |
| D13: `fold.ts` keys +11 and +6 | +10 and +11, measured by statement | D13's figures were for the tree under Q2(a); `checkKeys` and the move comment's removal moved them |
| D6: a per-key `checkLen` loop and the `tokenMoved` one-liner rewritten; D10: "At most 128 bytes of UTF-8 per name" | `checkKeys`, which refuses an empty key first; the one-liner removed; the `attributes` and `resources` descriptions add "and no empty name; either refuses the whole command", held by `TestToolsMatchGolden` and QA's `TestQACmdAddActorStatesTheBoundTheGoFoldEnforces` | Q2's refusal of an empty key, and the review's nit on the one-liner |
| Tasks 9, 10 and 11 in that order, and D13's keys "after the review settles" | the breaks first, on the base keys and ledger; then the keys; then Phase 4b; after its fixes the fifteen `fold.ts` keys re-pointed again and the ledger written | the review was briefed to check the break log and the keys, which needed both to exist first |
| D16: SPEC-014's lookup, refusal and consequences paragraphs, SPEC-013's command path and validators paragraph, SPEC-016's `transitions` paragraph | each whole | the QA prompt's own rule: the whole spec, never a section list, which inherits the dispatcher's blind spot |
| D14: `internal/adventure/load.go` and `load_test.go` "Not lowered" | twelve rows lowered, those two among them, five added for QA's Go files, none raised | `--write-ledger` writes every row at the final tree's share; D14 predicted which rows would fall more than a point |
| D19 and Task 13: `task check` once, after C1 | it failed at its last step, `check:ts-mutation:docker`: node ran out of heap writing Stryker's report after every mutant had run, so no survivor was compared with its adjudication; `53188d2` made Stryker's command `bun test --bail --only-failures client/test`, after its own review, and the gate ran again whole | the report carries each killed mutant's test output, 387 MB on 2026-10-06, and C1's 131 TS tests took it past the container's heap; the owner chose the flags, which change what is printed and when a failing run stops but no verdict, and a whole re-run |

## What could not be established

- **The order sentences of SPEC-018** are held by QA's order tests in both
  folds; among the breaks, only the participant order is moved (K3, K25), as
  the plan's Gap 8 says. No break is added per order sentence: a refused event
  is refused either way and only its text differs.
- **A placement's `actor_id`** is held at 128 bytes by the fold's lookup, but
  `load_map`'s refusal of an unknown one quotes it at any length, and the
  adventure loader's refusal of an undeclared actor key quotes the key at any
  length, at boot only (D3's table). Raised to the owner with this report,
  with Q7's strings.
- **Nothing ties the copies of a bound to the engine's constant** but each
  side's tests, and the five at-bound fold tests red only when the engine's id
  bound falls below a loader's copy; the debt entry stays open.
- **The adventure loader's dry run refuses a scene object with no `art` under
  the wrong field**, `field "overrides": mapdef: objects[0] has no art …` (the
  plan's Gap 10). Unchanged here; raised to the owner with this report.
- **What killed two mutants of the 2026-10-06 TS mutation run is not in its
  report.** Stryker runs the test command with `exec` and no `maxBuffer`, so a
  run printing more than 1 MiB is killed and counted Killed; 23 of that run's
  kills stop near 1 MiB, and two of them, in `client/src/view/player.ts` and
  `client/src/app.ts`, show no failing test in the output kept. Recorded in
  `docs/verification-debt.md` as "A test run that prints more than 1 MiB is a
  killed mutant".
- **A campaign whose log already holds such a value, or an empty session id or
  move scene id, no longer opens, and a map holding a bad object id no longer
  boots** (the plan's Gap 5). No tracked golden, fixture or content file holds
  one. Nobody uses the product.

## What was deliberately left out, and where it went

- The other strings a map carries, an object's `kind` and `art`, a tile's
  kind, material and art, and a placement's `actor_id` (Q7(a)): nowhere yet;
  raised to the owner with this report.
- An envelope's `event_id`, `participant_id` and `actor_role`, an actor's
  `module_data`, and every other envelope's `session_id`, which
  `stampSessionIDAgainst` overwrites (D3's table): the first four not named by
  the owner, the last held by the stamp to 37 bytes or empty; nowhere, unless
  the owner names them.
- A grant refusing a participant identity does not know (Q6(a)): a ticket of
  its own if the owner wants it.
- `docs/map-format.md` §10 lists neither a map's name and id bounds nor a
  placement's token id bound, which the name arc (`4edece4`) and the id arc
  (`1d714fe`) added; found while §10 was edited, and raised to the owner with
  this report.
- The adventure loader's misnamed art refusal: raised to the owner with this
  report.
- No proto changed; `check:breaking` names nothing.
