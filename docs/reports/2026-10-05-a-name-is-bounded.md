# A name is bounded: the change

**Ticket:** `docs/superpowers/specs/2026-10-05-a-name-is-bounded-design.md`,
revised by its writer after verification and after sign-off (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-05-a-name-is-bounded.md`, verified
by `verify-ticket` (passes with gaps).
**The owner's rulings:** on 2026-10-05, a go-ahead for a ticket bounding a
map's and an adventure's names, raised by the reason-bound report; at
sign-off the same night, a bound of 256 bytes of UTF-8 with an empty name
allowed (Q1, Q2), scope (b), the three names plus `SessionStarted.name` with
ids left to a ticket of their own (Q3), and Q4 to Q9 as the plan proposed:
the backpressure tests' padding moved into `module_data`, the `add_actor` and
`start_session` descriptions stating the bound, layout (a) in `fold.ts`, the
loader's unpinned empty-name refusals recorded as debt, rows A to C, one code
commit and the report. On the review, on 2026-10-06: every finding fixed but
N7, which the owner's ruling left.
**Last code commit:** `4edece4`, on `fbf2ff1`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline fbf2ff1..4edece4

    4edece4 A name is bounded

`git diff --stat fbf2ff1..4edece4`: 56 files changed, 2769 insertions(+), 180
deletions(-).

The gate, `task check` whole, after `4edece4`, once: it exited 0 with no
step failed. Its check steps' own verdict lines read `check:comments` clean
over 263 files, `check:requirements-chain` 276 rows, 221 test files and 13
specifications, `check:doc-owner` 80 files, `check:new-prose` 1439 added
lines clean, `check:coverage` 20 packages at or above their floors
(`internal/engine` 98.9 %, `internal/gateway` 94.7 %, `internal/mapdef`
98.8 % and `internal/adventure` 91.7 %), `check:no-pack`,
`check:no-retraction` and `check:no-create-scene` clean, `task lint` 0
issues, `check:breaking` reporting pre-release with no objection,
`check:mutation` 14 packages with zero unadjudicated survivors (ten mutated
afresh, `internal/identity`, `internal/artlib`, `internal/campaigncfg` and
`internal/store` reusing their verdicts; six mutants timed out in
`internal/sight`, `internal/rules` and `internal/mcp` and counted as killed),
and `check:ts-mutation`, re-mutating since the client's sources and tests
changed, 2921 mutants,
2824 killed, 68 survivors all adjudicated equivalent, zero unadjudicated, 29
timed out and counted as killed. `check:mutation` began with 38.7 GiB free;
`task check:drift` exited 0 after the commit.

## Done looks like, answered

1. `[x]` `engine.Apply` refuses a `SessionStarted`, a `SceneCreated`, an
   `ActorAdded` or an `AdventureLoaded` whose name is longer than 256 bytes,
   and accepts one of exactly 256:
   `internal/engine/apply_test.go#TestASceneWhoseNameExceedsTheBoundIsRefused`,
   `#TestAnActorWhoseNameExceedsTheBoundIsRefused`,
   `#TestAnAdventureWhoseNameExceedsTheBoundIsRefused` and
   `#TestASessionWhoseNameExceedsTheBoundIsRefused`, each red on `fbf2ff1`
   (`err = <nil>`), and
   `internal/engine/apply_boundary_test.go#TestNamesAtCapAreAccepted`, whose
   at-bound half K4 holds. QA's
   `internal/gateway/qa_name_bound_test.go#TestQANameTheGoFoldRefusesEveryNameOverTheBound`
   and `#TestQANameTheGoFoldAcceptsEveryNameUpToTheBound` hold the same over
   multibyte names.
2. `[x]` `client/src/fold.ts` refuses the same events at the same counts:
   `client/test/fold-rejections.test.ts#a session name longer than 256 bytes
   is rejected`, `#a scene name over 256 UTF-8 bytes is rejected`, `#an
   actor name longer than 256 bytes is rejected` and `#an adventure name
   longer than 256 bytes is rejected`, each red on `fbf2ff1`, and `#names of
   exactly 256 bytes are ACCEPTED`, held by K10; QA's
   `client/test/qa-name-bound.test.ts`.
3. `[x]` A map file and an adventure with a name over the bound are refused
   at load, naming the file and the field, before anything is appended:
   `internal/mapdef/load_test.go#TestInvalidMapsAreRefusedWithAUsefulReason`'s
   `name-too-long` case and
   `internal/adventure/load_test.go#TestLoadInvalidFixtures`'s three
   `*-name-too-long` cases, red on `fbf2ff1`; at the bound,
   `internal/mapdef/load_test.go#TestAMapNameOfExactlyTheBoundLoads` and
   `internal/adventure/load_test.go#TestLoadAcceptsValuesExactlyOnEveryLimit`,
   whose fixture's three names sit at 256. QA's
   `internal/mapdef/qa_name_bound_test.go`,
   `internal/adventure/qa_name_bound_test.go` and
   `internal/gateway/qa_name_bound_test.go#TestQANameLoadMapOfAnInstalledMapWithALongNameIsRefusedNamingFileAndField`.
4. `[x]` An `add_actor` or a `start_session` over the bound is answered
   ok=false and appends nothing:
   `internal/gateway/server_test.go#TestAnActorWhoseNameExceedsTheBoundAppendsNothing`
   and `#TestASessionWhoseNameExceedsTheBoundAppendsNothing`, red on
   `fbf2ff1` (`257-byte name: ok=true error=""`; the `add_actor` sketch is
   the plan's P4), and under K2 and K17; QA's
   `internal/gateway/qa_name_bound_test.go#TestQANameAddActorOverTheBoundIsRefusedOnTheWire`
   and `#TestQANameStartSessionOverTheBoundIsRefusedOnTheWire`.
5. `[x]` Every golden, fixture and shipped content file still loads and
   folds, and `task check` whole is green, as the gate paragraph above
   records.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A session's, a scene's, an actor's and an adventure's name is at most a fixed number of UTF-8 bytes, in both folds. | VTT-274 |
| A map or an adventure that carries a longer name is refused at load. | VTT-275 |
| A command that carries a longer name appends nothing (VTT-162). | VTT-162 and VTT-161 gain the two wire tests; no new row |

VTT-276, that the `add_actor` and `start_session` tools state the bound,
came from Q5.

## The sort

Three rows accepted as D11 proposed (A to C, now VTT-274 to VTT-276), and ten
candidates refused, as D11 lists them; none was reversed.

## Phase 4a: QA adjudications

**`4edece4` (VTT-274 to VTT-276, VTT-161, VTT-162), QA on opus, given those
rows, SPEC-018 whole, SPEC-013's command-path and validators paragraphs,
SPEC-014's "What reaches a client from a refusal", the text of
`SessionStarted`, `SceneCreated`, `Actor`, `ActorAdded`, `AdventureLoaded`,
`AddActor` and `StartSession`, `go doc -all` of `internal/engine`,
`internal/gateway`, `internal/campaign`, `internal/identity`,
`internal/mapdef` and `internal/adventure`, `fold.ts`'s export lines and the
two tools' entries; it wrote `internal/gateway/qa_name_bound_test.go`,
`internal/mapdef/qa_name_bound_test.go`,
`internal/adventure/qa_name_bound_test.go` and
`client/test/qa-name-bound.test.ts`.** Fifteen Go tests with 103 subtests and
57 TS tests, none failing; sixteen injections into its own files, all red,
each file restored by its hash. It built its fixtures at run time in
`t.TempDir()`; none is committed.

- SPEC-018 gives no exact text for a name's refusal, only its form: as in the
  reason arc, SPEC-018 quotes two texts as the form and the implementer's
  tests pin each field's exact text; QA asserts containment. No change.
- The client's field wording is unstated: the same. No change.
- Whether a loader names the file by the path opened or relatively: SPEC-018
  says "naming the file and the field", and SPEC-014 settles
  `LoadInstalled`'s form. No change.
- `campaign.Open` refusing a log that holds an over-long name: untested here,
  since QA had no store surface. No change; it goes to the last section.
- `AppendBatch`'s refusal asserted by containment: SPEC-014 says "forwarded
  as written", and the text was identical. No change.
- A map's name becoming `SceneCreated.name` was assumed and held. No change.
- `start_session`'s schema marks `name` required while SPEC-018 lets it be
  empty: required means present. No change.
- An over-long name together with a controller over the wire:
  `validateAddActor` refuses first (SPEC-013), and the fold's order is held by
  `TestQANameTheGoFoldChecksTheEarlierFaultBeforeTheName`. No change.
- Requirements QA asked an id for: six sentences: SPEC-018's on the refusal's
  form, on the client's message and on the order of checks, twice, all text
  the sort refused (its sixth and tenth), and SPEC-014's two, which are its
  own record's. None dispensed.
- The implementer changed QA's files once: the two empty-name cases gained a
  pointer to SPEC-018's "May be empty" column, per the reading review.

## The breaks

The commit's message carries them, grouped by the checks that spoke; each was
run in a scratch clone of the commit's final tree and restored from its saved
text, checked by hash, rather than by D17's inverse edit by hand. Eighteen:
the Go scene, actor, adventure and session checks disabled, each; the Go
scene check made `>=`;
`maxNameBytes` 300; the Go actor refusal reworded; the TS scene, actor,
adventure and session checks removed, each; the TS actor bound 255; the TS
scene name counted in characters; the map loader's check disabled, then
`>=`; the adventure loader's three checks disabled, then `>=`; and
`start_session`'s description saying 200, regenerated. Every one went red.
P5's, which D17 also named, was not among them; Deviations.

## Rule 9: how MapTool does this

Answered in the plan, from `~/dev/RPTool/maptool` at `f4b7fef6c`: MapTool
bounds no name. `Token.setName`, `Zone.setName` and `Campaign.setName` assign
what they are given, and the one rule on a name, `Token.validateName`, refuses
an untrusted, non-GM rename that duplicates another token's name on the zone,
ignoring case, and has no length rule. Borrowed: nothing.
Refused: the unbounded name, since the log is append-only. Noted and not
taken: MapTool's second, audience-specific names (`Zone.playerAlias`,
`Token.gmName`), which answer what a seat is told a place is called, a
projection question rather than a bound.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| the plan: "The gaps … travel with this plan. This plan does not edit the ticket."; D3: the writer adds `SessionStarted.name` if Q3 is (b) | its writer revised it for Gaps 1 to 5 and 11, naming the four backpressure tests, the 19-byte longest name, the adventure loader's key, the loaders' fixtures, the at-bound half's red, and that an `AdventureLoaded` reaches no projected seat; after sign-off, it added `SessionStarted.name` | so the ticket states what the verification found rather than leaving it in the plan alone, as the earlier tickets' writers did, and Q3 (b) |
| D17: K1 to K15 and P5's, plus K16 and K17; K5 reddening "the three engine refusals, wire test" | K1 to K18, K5 reddening the four engine refusals, `TestTheToolsStateTheFoldsNameBound`, both wire tests and six QA tests; P5's break, the padding moved back to a name, was left out of the breaks and run at `4edece4` for this report: all four re-vehicled tests red, the joiner's with `engine: scene name must be at most 256 bytes, got 28672` | the reading review: the TS session check had no break (K18), and K5's list predated the session's inclusion; P5's: an oversight, the break script having been written from the K table, outside which D17 named it |
| D7: cut `TestAWedgedConnectionIsTornDownAndOthersKeepServing`'s padding block to six lines or fewer | the whole 25-line block it shared with the backstop's reasoning cut to a four-line warning | only a code line ends a block in `check:comments`, and none stood between the backstop's two paragraphs and the payload's, so they were one block, and a line added to a block over six lines is refused; the history it carried follows below |
| D8: `at-every-boundary`'s names locked at 256 by three assertions | the same, and its test's 36-line doc cut to a four-line warning | the reading review: the doc listed the fixture's caps without the names, and a line added to a block over six lines is refused |
| D8: a TS refusal per field and an at-cap case per field | a refusal per field, the scene's named "a scene name over 256 UTF-8 bytes is rejected", and one at-cap case for all four, "names of exactly 256 bytes are ACCEPTED" | the register's evidence cell is comma-separated, and the scene case's first name carried a comma; one at-cap case folds all four names in one log, K10 reds it for the actor, and QA's per-field accepts cases hold each name |

The three cut blocks recorded, before this change: that `bigPaddingName` was
`bigSceneName` while every site rode on `create_scene`, renamed for its job
when that command left the platform on 2026-09-02 and the sites moved to
`add_actor`, chosen as DM- and agent-only, one command to one event, accepted
for a fresh id with no session or scene precondition, and carrying an
unbounded name through to the broadcast; that it then landed in two fields,
an `Actor.name` in three tests and a `SceneCreated.name` in the joiner's, four
uses, after an earlier draft of that comment had counted five; that the
driver-write backstop was 3 s and failed deterministically, five runs of five
at write 14, on a developer machine under disk pressure, because the writes
drain through SQLite-backed `Append`, passed at 30 s in 17.4 s, and was never
seen on CI's clean runner, and that it differs from the victim-read
deadlines, whose expiry is what those assert; and that `at-every-boundary`
was described by an invariant rather than a count after a comment had said
"seven" three times when `maxIDBytes` arrived, that it cannot reach
`rv.Current > rv.Max`, because `max: 0` short-circuits the guard before it,
so `testdata/valid`'s `brace-guard`, focus 10 of 10, pins that one, that its
placement at (0,0) is a deliberate redundant pin beside
`testdata/valid/scenes/gate.json`'s, and that its resource with `max: 0`
tests that zero means unlimited.

## What could not be established

- **Ids are unbounded** in both folds: a scene's, an actor's, a token's and
  an adventure's id. Q3 left them to a ticket of their own, which is not yet
  written.
- **Nothing ties `fold.ts`'s `256` or either loader's `maxNameBytes` to the
  engine's constant** but each side's tests; `docs/verification-debt.md`'s
  entry on the bounds' copies counts them.
- **The adventure loader's three empty-name refusals are pinned by no test**;
  `docs/verification-debt.md` carries the recipe.
- **`campaign.Open` refusing a log that already holds an over-long name** is
  observed by no test in this change; SPEC-018's sentence covers every bound.
- **Two of the four backpressure tests do not need the padding to be large**:
  `TestAClientThatStopsReadingEntirelyIsTornDown` and
  `TestAForceClosedClientIsAnnouncedGone` pass at this tree with
  `bigPaddingName` empty, the `module_data` wrapper still adding bytes, and,
  by the reading review's measurement, at `fbf2ff1` with a 16-byte name
  though not with one of 8 bytes or fewer; a few dozen bytes a frame
  suffices, not 28 KiB. It predates this change.
- **The one Go failure the plan's verification saw in four whole-suite runs
  on its probe tree (Gap 7)** was never identified; the gate's run after
  `4edece4` had none.

## What was deliberately left out, and where it went

- Ids: nowhere yet; raised to the owner with this report.
- `mapdef.Load`'s doc saying "grid sanity gates everything else", which the
  name check and `cell_px` both precede: left by the owner's ruling on the
  review (N7), since the block is not otherwise touched.
- A campaign directory whose map or adventure carries a name over 256 bytes
  no longer boots, since `loadMapsDir` and `loadAdventuresDir` refuse it (the
  plan's Gap 9): accepted, since no shipped content, fixture or eventgen draw
  comes within 237 bytes of the bound.
- No proto changed; `check:breaking` names nothing.
