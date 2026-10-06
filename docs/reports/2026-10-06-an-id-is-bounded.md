# An id is bounded: the change

**Ticket:** `docs/superpowers/specs/2026-10-06-an-id-is-bounded-design.md`,
revised by its writer after verification and after sign-off (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-06-an-id-is-bounded.md`, verified
by `verify-ticket` (passes with gaps).
**The owner's rulings:** on 2026-10-06, a go-ahead for a ticket bounding ids,
raised by the name-bound report; at sign-off the same day, a bound of 128
bytes of UTF-8 in one constant `maxIDBytes` (Q1), an empty scene, token and
adventure id refused everywhere (Q2(b)), the scope of the four ids that
create an identity plus a map placement's token id (Q4(a)), and the rest as
the plan proposed. On QA's findings: a requirement row for the adventure
loader's existing bound on a scene id. On the review: every finding fixed,
the open ruleset path recorded as a debt entry, and the fixture the reviewer
emptied restored from `HEAD`. On the report's review: every finding fixed,
and `1d714fe`'s message left as it stands.
**Last code commit:** `1d714fe`, on `b9de835`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline b9de835..1d714fe

    1d714fe An id is bounded

`git diff --stat b9de835..1d714fe`: 65 files changed, 3749 insertions(+), 337
deletions(-).

The gate, `task check` whole, after `1d714fe`, once: it exited 0 with no step
failed. Its check steps' own verdict lines read `check:comments` clean over
268 files, `check:requirements-chain` 282 rows, 227 test files and 13
specifications, `check:doc-owner` 80 files, `check:new-prose` 2103 added lines
clean, `check:coverage` 20 packages at or above their floors
(`internal/engine` 99.0 %, `internal/gateway` 94.7 %, `internal/mapdef` 98.8
%, `internal/adventure` 92.0 % and `internal/campaign` 89.3 %),
`check:no-pack`, `check:no-retraction` and `check:no-create-scene` clean,
`task lint` 0 issues, `check:breaking` reporting pre-release with no
objection, `check:mutation` 14 packages with zero unadjudicated survivors
(eleven mutated afresh, `internal/identity`, `internal/campaigncfg` and
`internal/store` reusing their verdicts; six mutants timed out in
`internal/sight`, `internal/rules` and `internal/mcp` and counted as killed),
and `check:ts-mutation`, re-mutating since the client's sources and tests
changed, 2925 mutants, 2788 killed, 68 survivors all adjudicated equivalent,
zero unadjudicated, 69 timed out and counted as killed, over 123 minutes on a
machine whose load stood near 10 and whose swap was nearly full.
`check:mutation` began with 38.1 GiB free; `task check:drift` exited 0 after
the commit.

## Done looks like, answered

1. `[x]` `engine.Apply` refuses a `SceneCreated`, an `ActorAdded`, a
   `TokenPlaced` or an `AdventureLoaded` whose id is longer than 128 bytes,
   and accepts one of exactly 128:
   `internal/engine/apply_test.go#TestASceneWhoseIDExceedsTheBoundIsRefused`,
   `#TestAnActorWhoseIDExceedsTheBoundIsRefused`,
   `#TestATokenWhoseIDExceedsTheBoundIsRefused` and
   `#TestAnAdventureWhoseIDExceedsTheBoundIsRefused`, each red on `b9de835`'s
   source with only the constant added (`err = <nil>`), since
   `id_bound_internal_test.go` does not compile without it; and
   `internal/engine/apply_boundary_test.go#TestIDsAtCapAreAccepted`, whose
   at-bound half K5 holds. The empty ids the owner's Q2 ruling added are
   `#TestAnEmptySceneTokenOrAdventureIDIsRefused`, red the same way. QA's
   `internal/gateway/qa_id_bound_test.go` holds the same over multibyte ids
   and the order of checks.
2. `[x]` `client/src/fold.ts` refuses the same events at the same counts:
   `client/test/fold-rejections.test.ts#a scene id over 128 UTF-8 bytes is
   rejected`, `#an actor id longer than 128 bytes is rejected`, `#a token id
   longer than 128 bytes is rejected`, `#an adventure id longer than 128 bytes
   is rejected` and the three empty cases, each red on `b9de835`; and `#ids of
   exactly 128 bytes are ACCEPTED`, held by K13. QA's
   `client/test/qa-id-bound.test.ts`.
3. `[x]` A map whose id or placement token id is over the bound, and an
   adventure whose id, actor id or placement token id is, are refused at load,
   naming the file and the field, before anything is appended:
   `internal/mapdef/load_test.go#TestInvalidMapsAreRefusedWithAUsefulReason`'s
   `id-too-long` and `placement-token-id-too-long` cases,
   `internal/mapdef/installed_test.go#TestAnInstalledMapWhoseIDExceedsTheBoundIsRefused`
   and `internal/adventure/load_test.go#TestLoadInvalidFixtures`'s
   `adventure-id-too-long`, `actor-id-too-long` and
   `placement-token-id-too-long` cases, each red on `b9de835`. At the bound,
   `internal/mapdef/load_test.go#TestAMapIDOfExactlyTheBoundLoads` and
   `internal/adventure/load_test.go#TestLoadAcceptsValuesExactlyOnEveryLimit`,
   held by K21, K23, K28 and K32. That nothing is appended: QA's
   `internal/gateway/qa_id_bound_test.go#TestQAIDLoadMapOfAnInstalledMapOverTheBoundIsRefusedWithLoadInstalledsText`.
4. `[x]` An `add_actor` or a `place_token` whose id is over the bound is
   answered ok=false and appends nothing:
   `internal/gateway/server_test.go#TestAnActorWhoseIDExceedsTheBoundAppendsNothing`
   and `#TestATokenWhoseIDExceedsTheBoundAppendsNothing`, red on `b9de835`
   (`ok=true error=""`) and under K2, K3, K7, K9 and K10; QA's
   `TestQAIDAddActorOverTheBoundIsRefusedOnTheWireWithTheFoldsText`,
   `TestQAIDPlaceTokenOverTheBoundIsRefusedOnTheWireWithTheFoldsText` and
   `TestQAIDAnEmptyIDIsRefusedOnTheWireWithTheFoldsText`.
5. `[x]` Every golden, fixture and shipped content file still loads and folds:
   `internal/harness/fold_golden_test.go#TestFoldGoldenCorpus` folds every
   golden stream,
   `internal/adventure/conformance/conformance_test.go#TestConformanceOverAdventuresGlob`
   loads every shipped adventure, and
   `internal/gateway/map_test.go#TestTheShippedCampaignResolvesItsOwnArt`
   loads the shipped map; and `task check` whole is green, as the gate
   paragraph above records.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A scene's, an actor's, a token's and an adventure's id is at most a fixed number of UTF-8 bytes, in both folds. | VTT-277 |
| A map or an adventure that carries a longer id, a map's placements included, is refused at load. | VTT-279, and VTT-282 for an adventure's scene id, by the owner's ruling on QA's finding |
| A scene's, a token's and an adventure's id may not be empty, in both folds, as an actor's may not; a map placement with an empty token id is refused at load. | VTT-278 and VTT-280 |
| A command that carries a longer id appends nothing (VTT-162). | VTT-162 and VTT-161 gain the two wire tests; no new row |

VTT-281, that the `add_actor` and `place_token` tools state the bound, came
from Q5.

## The sort

Five rows accepted as D11 proposed (A to E, now VTT-277 to VTT-281), and
eleven candidates refused, as D11 lists them; none was reversed. A sixth row,
VTT-282, came after Phase 4a (Deviations).

## Phase 4a: QA adjudications

QA on opus, given VTT-277 to VTT-281, VTT-161, VTT-162, SPEC-018 whole,
SPEC-014's lookup, refusal and consequences paragraphs, SPEC-013's command
path and validators, SPEC-007's `load_map` batch, the eight message texts,
`go doc -all` of `engine`, `gateway`, `campaign`, `identity`, `mapdef`,
`adventure` and `rules`, `fold.ts`'s and `state.ts`'s export lines and the
two tool entries, with the name arc's four QA files as examples of harness.
It wrote `internal/gateway/qa_id_bound_test.go`,
`internal/mapdef/qa_id_bound_test.go`,
`internal/adventure/qa_id_bound_test.go` and
`client/test/qa-id-bound.test.ts`:
21 Go tests and 63 TS tests, none failing; every injection into its own
files red.

- The field's spelling in a refusal is unstated: QA matched patterns. As in
  the name arc, the implementer's tests pin each exact text. No change.
- Which of SPEC-018's two message shapes an id uses is unstated: scene, token
  and adventure say "1-128", the actor "at most 128" because its empty case
  has its own refusal, which SPEC-018 quotes. No change.
- The TS empty actor-id text is unstated: it predates this change. No change.
- Empty ids at load beyond VTT-280: the adventure loader already refused an
  empty adventure, actor, scene and placement token id before this change; a
  map with an empty id is not refused by `mapdef.Load` but is by
  `mapdef.LoadInstalled`, the path boot and `load_map` take; held in memory,
  its `SceneCreated` is refused by the fold now (VTT-278), appending nothing.
  The owner's Q2 ruling named only the map placement's token id at load. No
  change.
- `load_adventure`'s refusal text is unstated (SPEC-014 covers `load_map`):
  the fold's text arrived verbatim. No change.
- That a map's scene id is its id, and that an adventure's ids reach its
  events unchanged, is unstated in QA's inputs: SPEC-007 and SPEC-014 own
  them, and both held. No change.
- Loader check order against the filename/id agreement is unstated: QA's
  fixtures avoid combined faults. No change.
- The adventure loader's bound on a scene id had no row: it predates this
  change, held by `TestLoadInvalidFixtures`' `scene-id-too-long` row. The
  owner ruled a row for it, VTT-282 (Deviations).
- Requirements QA asked an id for: the refusal's form, the order of checks,
  `checkLen`'s TS wording, `campaign.Open` refusing a log, "with that text",
  SPEC-014's naming and forwarding sentences, its at-bound install consequence
  and SPEC-013's validator order. The form and the order are the sort's sixth
  and eighth refusals (D11); the rest are their own records' sentences, as in
  the name arc. None of these was dispensed; QA's tenth, the scene id missing
  from VTT-279, became VTT-282 above.
- Process: QA opened `internal/adventure/testdata/ruleset/ruleset.json`,
  committed data that predates the change, for its format; and its first
  injection harness keyed backups by basename and overwrote two of its own
  files, which it rebuilt and re-verified. Every file of the change outside
  its four hashed the same before and after.
- After the run, two of QA's tests moved package unchanged, for `go-arch-lint`
  (Deviations); every QA test is green in its new place.

## Phase 4b: the review

One reviewer on opus, given `git diff HEAD`, the untracked files, the plan
and the draft commit message. No Must. Its findings, each fixed by the
owner's ruling: artlib's cut doc had held the only live record of an open
path, now a debt entry; a toolgen test comment called `kind` the only
required field with a description; both specifications' Status lists left
out the QA files; the break script's parsing of bun's output dropped failing
names and did not check `generate:contract`'s exit; `at-every-boundary`'s
guide named one `>` on `maxIDBytes` where four stand; `MaxMessage`'s doc was
narrower than its use; two wordings in the commit message; and four
cross-references in the adjudication files pointed at coordinates that were
already stale at `b9de835`.

The reviewer also emptied a committed fixture by accident:
`python3 -m json.tool ../valid/*.json` took the second file the glob named,
`internal/mapdef/testdata/valid/no-terrain.json`, as its output and
truncated it. Its own restore was refused by the permission check, and the
file was restored from `HEAD` on the owner's word before anything was
staged; `git diff HEAD` was byte-identical to the review package otherwise.

## The breaks

The commit's message carries them, grouped where breaks share a shape, with
the checks that spoke; each was run in a scratch clone of the commit's final
tree and restored from its saved text, checked by hash. Thirty-three: the
plan's K1 to K32, and K33 for VTT-282. Every one went red where the K table
said, and K30, the TS scene-id check moved after the duplicate check, stayed
green as expected, since an over-long or empty id is never a stored key. The
script asserts each K's expected red by name. It ran twice: once before two of
QA's tests moved package, and again on the final tree, with the gateway suite
added to K21 and K23 to follow the moved test, which added the moved tests'
reds and, under K21 and K23,
`TestQAIDLoadMapOfAnInstalledMapOverTheBoundIsRefusedWithLoadInstalledsText`.

## Rule 9: how MapTool does this

Answered in the plan, from `~/dev/RPTool/maptool` at `f4b7fef6c`: every
MapTool id is generated, never typed, and has exactly one length. `Token`,
`Zone` and `Campaign` each hold a `GUID` drawn from `UUID.randomUUID()`, and
`GUID.validateGUID` refuses any length but 16 bytes wherever an id is
parsed, off the wire included. Borrowed: an id has one bound, checked where it
is read rather than trusted from its writer. Not borrowed: generation, since
here a map's id is its filename, an adventure's ids are written by hand, and
the agent chooses `add_actor`'s and `place_token`'s ids to address them next.
Checked and rejected: telling a name from an id by length, since nothing here
resolves a reference by name.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| the ticket as written before verification | its writer corrected it for the verification's defects; after sign-off, it added the empty-id rule | the verification, and Q2(b) |
| D11: five rows | six; VTT-282 states the adventure loader's bound on a scene id | QA found that bound, which predates this change, held by `TestLoadInvalidFixtures`' `scene-id-too-long` row and no requirement; the owner ruled a row for it, cited by the two tests that already held it (`TestLoadInvalidFixtures`, `TestLoadAcceptsValuesExactlyOnEveryLimit`), this change's `TestTheIDBoundMirrorsEngine`, and QA's `TestQAIDAnAdventureSceneIDOverTheBoundIsRefusedNamingFileAndField`; `1d714fe`'s message says "the three tests that already held it", which miscounts |
| D17: K1 to K32 exactly | K1 to K33 | K33 breaks the adventure loader's scene-id check, which predates this change; VTT-282, the row that now states it, did not exist when the table was written |
| D15: QA writes four files | five; two of QA's tests moved package unchanged, the `campaign.Open` test to `internal/campaign/qa_id_bound_test.go` and the at-bound map test that folds to `internal/gateway/qa_id_bound_test.go` | `go-arch-lint` refused them where QA put them: `gateway` may not import `store`, nor `mapdef` `engine`; QA's brief named `golangci-lint` and `tsc` and not the architecture check, and the pre-commit hook runs it |
| D13: artlib's doc cut to a warning, the history to this report | the same, and its one open entry to `docs/verification-debt.md` as "A ruleset's names reach a `use_ability` refusal at any length", with a recipe measured on `b9de835`'s `internal/rules` | the review's S1: two of the doc's three "still unbounded" entries are closed by this change, and the third is open work, not history |
| none | four cross-references in `tools/mutation-equivalents.txt` and `tools/ts-mutation-equivalents.txt` name their target instead of a coordinate | the review's N4, by the owner's ruling; they were stale before this change |

## What the cut comments recorded

The two cut blocks recorded, before this change, in
`internal/adventure/load.go`: that `maxIDBytes` was the one constant there
with no engine twin; that it bounded a scene id by arithmetic rather than
tidiness, since `adventure.Compile` prefixes every warning with `scene %q: `
and warnings do not collapse across scenes, so an id multiplies by scenes
times warnings on one `CommandResult`, the path
`TestABrokenBundleCannotPushALoadAdventurePastTheReadLimit` calls the
aggregating half of the bound; that a scene id, unlike a map's, is not a
filename, so the filesystem never bounded it; that 128 matched
`maxNoteKeyBytes`; that a map's scene id through `mapdef.LoadInstalled` was
left at the filesystem's limit rather than tightened, since that path does not
scene-qualify its warnings, which this change overturns; and that the value
was pinned only indirectly, by `scene-id-too-long`'s expected message. And in
`internal/artlib/artlib.go`: that `Clip` bounds because a sidecar value of any
length rides back on a `CommandResult` and a load's warnings once failed to
arrive past the client's read limit, and makes valid UTF-8 because
`CommandResult.warnings` is a proto3 string, a `json.RawMessage` keeps a
file's bytes verbatim, and a fixed byte cut splits a rune, either of which
makes protojson refuse the frame so a `load_map` answer never arrives; that
the art and map paths were surveyed on 2026-09-09 and every interpolation from
them passes `Clip` or `BoundErr`; the list of paths still unbounded, of which
`LoadInstalled`'s declared id and the adventure loader's actor and token
collision refusals are bounded by this change; that the list once went a day
without noticing two entries had left it, because the commits that closed them
updated only the art ticket's copy; that artlib and mapdef bounding one id are
two paths rather than two bounds, and reading their surviving mutants as
redundancy once deleted the warning side's only bound, measured at 20,041
bytes with the socket closing on "message too big"; and that the second
`ToValidUTF8` pass replaced a hand-rolled scan back over continuation bytes
whose `cut > 0` guard the mutation gate showed to be dead code. `Clip`'s own
doc recorded that it lives in `internal/artlib` because `internal/mapdef`
already imports it, and the alternative was a package of its own while gate
work was paused.

## What could not be established

- **A map with an empty id** is not refused by `mapdef.Load` itself;
  `mapdef.LoadInstalled`, which boot and `load_map` both use, refuses it,
  because a map's filename must equal the id it declares and `.json` names no
  map (`idIsAFilename`). Only a map built in memory reaches the fold with one,
  and its `SceneCreated` is refused there (VTT-278), appending nothing. Q2's
  ruling named only a placement's token id at load.
- **A ruleset's names** reach a `use_ability` refusal at any length;
  `docs/verification-debt.md` carries the recipe.
- **Nothing ties `fold.ts`'s `128` or either loader's `maxIDBytes` to the
  engine's constant** but each side's tests; the debt entry on the bounds'
  copies counts them.
- **The ids that only refer**, a move's token id or a grant's actor id, have
  no check of their own; where the fold looks one up, it is bounded through
  the fold's requirement that what it names exists (D11's fifth refusal). The
  plan's D3 names those it does not look up, which are therefore not bounded:
  `TokenMoved.scene_id`, never read; `TokenHidden.token_id`, which is not
  looked up but deletes nothing absent and stores nothing; and the ids an
  `AbilityUsed` or `AttackRolled` carries, which are testimony.
- **The other id-like strings**, a condition's id and source, a resource's or
  attribute's name, an ability's id, a scene object's id, an actor's
  `module_id`, a control event's participant id and an envelope's session id,
  have no bound in the fold; Q4(a) left them out of scope, and the plan's D3
  table measures each.

## What was deliberately left out, and where it went

- The other id-like strings: nowhere yet; raised to the owner with this
  report.
- The ids that only refer: by D11's fifth refusal, no row.
- `internal/gateway/scenario_test.go` is not `gofmt`-clean on `main` (the
  plan's Gap 12): not this ticket's to fix; nowhere yet, raised with this
  report.
- No proto changed; `check:breaking` names nothing.
