# A sheet holds only values its ruleset declares and none written empty, and a campaign holding another is not served — report

Ticket: `docs/superpowers/specs/2026-11-08-campaign-data-fits-its-ruleset-design.md`.
Plan: `docs/superpowers/plans/2026-11-08-campaign-data-fits-its-ruleset.md`.
Branch `feat/campaign-data-fits-its-ruleset` from main `eb0fa35`. Commits:
`3132330` (ticket, plan, register), `1e32bd3` (Task 1), `49dd8d7` (Task 2),
`b428516` (Task 3, the last code commit), and `6cabbe6`, which merges main's
`301cb10` (the lint ticket; see Deviations). Every code reference below is
to `b428516`. Of the files this ticket changes, `6cabbe6` changes only the
register, which gains VTT-682.

## Done looks like

1. [x] **`add_actor` refuses a value the ruleset does not have.**
   `internal/gateway/add_actor_fit_internal_test.go`'s
   `TestAddActorWithARulesetRefusesAKeyTheRulesetDoesNotHave` and
   `TestAddActorWithARulesetAcceptsDeclaredKeys`, and
   `internal/gateway/qa_add_actor_keys_test.go`'s tests, which drive a served
   gateway over its WebSocket; `TestAddActorWithNoRulesetReadsNoKeyAgainstOne`
   and `TestQAKeysWithNoRulesetNoKeyIsReadAgainstOne` for no ruleset.
   `validateAddActorWith` in `internal/gateway/add_actor_validate.go` answers
   `validateAddActor`'s refusals first, then `rules.Ruleset.SheetKeyFault`.
2. [x] **An adventure refuses a value that has none.**
   `internal/adventure/actor_value_test.go`'s
   `TestAnActorValueWrittenNullOrACurrentLeftOutIsRefused` and
   `TestAnActorValueOfZeroAndAResourceWithNoMaximumLoad`, and
   `internal/adventure/qa_actor_value_test.go`. `actorJSON` and
   `resourceValJSON` decode through pointers; `actorValues` refuses.
3. [x] **A resource whose current has no value is refused where it is
   written.** `internal/mcp/resource_current_internal_test.go`'s
   `TestAddActorArgumentsRefuseACurrentWithNoValue` and
   `TestAddActorArgumentsPassACurrentOfZero`, `internal/mcp/qa_add_actor_current_test.go`
   through `mcp.New` and a tool call; `client/test/commands.test.ts`'s "a
   pasted actor whose resource current is null, left out or empty is
   refused, naming it" and "a pasted actor whose resource current is 0, or
   has no maximum, reaches the wire", and
   `client/test/qa-add-actor-paste.test.ts`.
   `refuseValuelessCurrent` in `internal/mcp/resource_current.go` runs in
   `commandFromArgs` after `protojson`; `valuelessCurrent` in
   `client/src/commands.ts` runs in `parseActorJSON` after its controller and
   kind refusals.
4. [~] **A campaign holding a value the ruleset does not have is not
   served.** `cmd/vtt/campaign_fit_test.go`'s
   `TestACampaignWhoseSheetsDoNotFitTheRulesetIsNotServed` (which reopens
   the campaign after the refusal), `TestACampaignWhoseSheetsFitTheRulesetIsServed`
   and `TestACampaignIsServedWithNoRulesetWhateverKeysItsActorsHold`, and
   `cmd/vtt/qa_campaign_fit_test.go`. `sheetsFit` in `cmd/vtt/serve_compose.go`
   runs right after `rules.Load`. "Leaves nothing open" is observed for the
   campaign's hold alone, as the plan's D5 set it: the test opens the
   campaign again after the refusal, as does `cmd/vtt/qa_campaign_fit_test.go`'s
   `TestQAFitARefusedCampaignCanBeOpenedAgain`. It moved because only the
   campaign's hold has an effect a test reads: a second `campaign.Open` fails
   while a handle holds the campaign (plan P7), and the identity store's
   handle leaves nothing a test reads. The refusal closes the identity store
   too, and no test observes that close: removing it leaves `go test
   ./cmd/vtt/` green, and so does removing every identity close in
   `composeServer`, so the same holds of every other refusal there.
5. [x] **`task check` exits 0 on the branch.** `task check` on `6cabbe6`,
   with golangci-lint 2.15.0, exited 0. Its
   lint step printed `0 issues.`, and the Go mutation gate reported no
   unadjudicated survivor, with mutants NOT EVALUATED and counted as killed
   in `internal/perceive`, `internal/sight` and `internal/mcp`; the one in
   `internal/mcp` is in `Server.Run`, not in the walk this ticket adds. The
   client's mutation was not judged (below). `task e2e` passed on the same
   tree, `handover.spec.ts`'s skipped test aside.

The rows the ticket leaves standing, VTT-617 and VTT-618, keep their text.
The tests that held them, in `internal/adventure/statblock_type_test.go`,
`internal/adventure/qa_statblock_property_test.go` and
`internal/adventure/load_test.go`, are unchanged by the branch and pass
under the gate, and their cells gained Task 1's QA tests in
`internal/adventure/qa_actor_value_test.go`:
`TestQAAVAUndeclaredAttributeKeyIsRefused` and
`TestQAAVAFirstFaultInByteOrderOfKeysIsNamed` for VTT-617,
`TestQAAVAAttributeKeyDeclaredAsResourceIsRefused` for VTT-618.

## What the rules became

R1 to R8 are the plan's labels.

- With a ruleset, `add_actor` refuses a key the ruleset does not have (R1):
  **VTT-669**, withdrawn after Phase 4a as ambiguous ("key order"),
  succeeded by **VTT-677**, which also states that the controller and kind
  refusals come first. With no ruleset (R2): **VTT-675**.
- `adventure.Load` and a value written null (R3): **VTT-670**.
- The MCP tool (R4): **VTT-671**. The console's paste (R5): **VTT-672**.
  The tool refuses before any command is sent, so before the gateway's
  controller and kind refusals, and names an actor with no id `actor ""`;
  the rows allow both. Of several valueless resources, the paste names the
  first in UTF-16 code-unit order of the keys and the tool the first in
  byte order; the two orders differ only on keys that are near-theoretical,
  and no row fixes either.
- `vtt serve` (R6): **VTT-673**, withdrawn with VTT-669 for the same
  reason, succeeded by **VTT-678**. With no ruleset (R8): **VTT-676**.
- The tool's schema requires `current` and not `max` (R7, from D6):
  **VTT-674**.
- Not in the ticket's rules, allocated as the work showed them: a `max`
  left out or null has no maximum (**VTT-679**, the owner's word of
  2026-11-08, after Task 2's Phase 4a); the tool states what it refuses in
  a sheet (**VTT-680**, after Task 2's reading review, as VTT-296 does for
  the fold's bounds); D5's order of the actors the boot names (**VTT-681**,
  after Task 3's reading review).
- R2 was refused at the first sort and R8 not drawn; both were allocated
  after Task 0's reading review, since a check that treats no ruleset as
  one declaring nothing passes R1 and R6 and refuses every key.

## The deliberate breaks

Each break was applied, run red, and undone. Task 1's commit names the
test each turned red, but for the type check, whose rules test was named
when this report's reading review ran the break again; Task 2's names
none, and Task 3's names the test for the campaign left open and for no
ruleset.
Elsewhere the table names the row that now holds the test that went red;
some of those rows were allocated after the break.

| Break | What went red |
|---|---|
| The key check reads resources before attributes, or leaves its keys unsorted (D1) | `TestSheetKeyFaultNamesTheFirstKeyThatDoesNotFit`, `TestSheetKeyFaultIsTheSameWhateverOrderItsKeysArrive` |
| The key check sorts its caller's keys in place | `TestSheetKeyFaultLeavesItsCallersKeysInTheirOrder` |
| No type check | `TestSheetKeyFaultNamesTheFirstKeyThatDoesNotFit`, and `TestAStatblockGivingAPropertyTheOtherTypesValueIsRefused` (VTT-618) |
| A null number read as `0`, a null resource skipped, a missing `current` read as `0` | `TestAnActorValueWrittenNullOrACurrentLeftOutIsRefused` (VTT-670) |
| A missing `max` refused | `TestAnActorValueOfZeroAndAResourceWithNoMaximumLoad` (VTT-679) |
| The loader passing one key | `TestAnActorsFirstUndeclaredKeyIsNamedOnRepeatedLoads` |
| The gateway's key check skipped; keys read before the controller | VTT-677 |
| No ruleset read as an empty one, at the gateway | VTT-675 |
| The MCP walk skipped; a null `current` read as a value; a `0` refused; the actor id not read; the JSON name lookup inverted; map keys unsorted | VTT-671 |
| The paste unchecked; a null read as a value; keys unsorted; read before the kind | VTT-672 |
| `max` still required in the schema | VTT-674 |
| The tool's docs silent on what it refuses | VTT-680 |
| The boot's check not called; only the first misfit actor named | VTT-678 |
| The campaign left open on refusal | `TestACampaignWhoseSheetsDoNotFitTheRulesetIsNotServed`, which reopens it (VTT-678) |
| The actors named in reverse order | VTT-681 |
| No ruleset read as an empty one, at the boot | `TestACampaignIsServedWithNoRulesetWhateverKeysItsActorsHold` (VTT-676) |

## Rule 9: how MapTool does this

The plan's Rule 9 section (MapTool at RPTool `63862e4`) stands: MapTool
stores any key with any value on a token and reads a missing one as its
property type's default. Nothing is borrowed.

## The plan's gaps

- **G1 and OD-1.** The gateway cannot tell a `current` left out from `0`,
  since both clients leave a `0` out of the command they send. The owner
  answered OD-1 **A** on 2026-11-08: the value is checked where it is
  written, and the ticket's writer narrowed item 3's WebSocket half to the
  console's raw paste before Task 0.
- **G2.** The files followed A: `client/src/commands.ts`,
  `client/test/commands.test.ts`, `cmd/vtt/webdist` and SPEC-022 joined,
  and `internal/gateway/codec.go` is unchanged.
- **G3.** D5 settles it: every actor that does not fit is named, each with
  its first key that does not fit, in byte order of their ids (VTT-681).

## Phase 4a adjudications

Task 1 (QA handed VTT-617, VTT-618, VTT-670, SPEC-024's statblock
paragraphs, and the surfaces of `adventure.Load` and `rules.SheetKeyFault`):
- A key that does not fit is named before a value written null: the key
  check reads the whole sheet first. As intended.
- Values written null are found attributes first, each in byte order. As
  built.
- The field named for a null: `attributes.<key>`, `resources.<key>`,
  `resources.<key>.current`. The update of SPEC-024 that follows this
  report states it.
- `SheetFault.Reason` carries VTT-618's words, which the loader uses whole:
  VTT-618's tests hold them, and nothing changes.
- Whether the key check sorts its caller's keys in place: it does not,
  held by `TestSheetKeyFaultLeavesItsCallersKeysInTheirOrder`, which QA was
  not handed.
- `"attributes": null` or `"resources": null` as a whole map loads as a
  sheet holding nothing there: no value field is written, and what a sheet
  holds is the DM's.
- A duplicate key in an actor file, the last one read: `encoding/json`'s,
  unchanged, near-theoretical (the owner's ruling of 2026-10-21).
- A negative `current` loads: before this ticket and outside its rules.

Task 2 (QA handed VTT-669, VTT-671, VTT-672, VTT-674, VTT-675 and
VTT-152 to VTT-154, the tool's schema and the surfaces):
- VTT-669 said "key order", which can be read without regard to case, while the
  check and its tests use byte order: a defect in the row. Withdrawn,
  succeeded by VTT-677; VTT-673 the same, succeeded by VTT-678.
- A resource written as `null` through the tool is refused by `protojson`
  with its own words, naming neither actor nor key: refused as the owner
  wants, and the DM's and the agent's refusals keep their form (2026-11-02).
- The tool's refusal of a bad argument is an error of the call, not a
  result: the path every bad argument already takes.
- An actor with no id and a stray key: the key check reads only an actor
  with an id, and the fold refuses the rest (VTT-154).
- The owner's "a max with no value is fine" held no row: VTT-679.
- Of several valueless resources, the first is named: in byte order of the
  keys by the tool (`slices.Sorted`), in UTF-16 code-unit order by the paste
  (`.sort()`). `TestAddActorArgumentsRefuseACurrentWithNoValue`'s case "the
  first resource in byte order of the keys" and "a pasted actor names its
  first valueless resource, its keys sorted" hold that the keys are sorted;
  their keys are ASCII, where the two orders agree.
- The tool's description is pinned by `TestAddActorSaysWhatItRefusesInASheet`.
- SPEC-024's Consequences sentence on `add_actor` no longer holds: the
  update of SPEC-024 that follows this report makes it present.

Task 3 (QA handed VTT-678 and VTT-676, the surfaces of `campaign` and
`composeServer`, and drove the built `vtt serve` too):
- The order of the actors the refusal names was in no row: Phase 4a
  answered with D5's preference, and the reading review then gave it a row,
  VTT-681.
- The boot names the map of the key that does not fit, which VTT-678 does
  not require: harmless, and kept.
- "Any of whose actors" reads as the live actors: a removed actor does not
  refuse the campaign.
- VTT-676's "other than for their length" cannot be reached through the
  boot: the fold refuses an over-long key when it is written and when the
  log is read.
- With an adventures directory as well, the campaign's misfit is reported
  first: the check runs before the adventures load. No test holds that
  order, and no row asks for it.
- The specifications lag the code: the updates that follow this report.

## Phase 4b

Each task's diff was read before its commit and read again after its
fixes. Tasks 2 and 3 ended on a round that found nothing to change in the
diff; Tasks 0 and 1 ended on a round whose findings were applied without
another. The findings that changed code:
- Task 2: QA's gateway test imported `internal/harness`, which go-arch-lint
  refuses in `internal/gateway`; it was rewritten on the gateway tests' own
  `websocket.Dial`. The walk's comment gave depth as the reason it runs
  after `protojson`; the comment now says the walk trusts the shapes
  `protojson` accepts, and the guard for maps of numbers was restored.
  Mutants of `valuelessCurrent` the reviewer found would survive were
  answered by "a pasted actor whose resources are written null holds none,
  and reaches the wire", by asserting the refusal's tail as well as its
  head, and by removing code nothing could observe. VTT-680 was allocated
  for the tool's description, which no row held.
- Task 3: QA's test of the actors' order pointed at the plan, which a code
  comment may not (SPEC-010); the order became VTT-681, cited by both tests
  that assert it.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| D4: the MCP tool's walk reads the arguments before `protojson` decodes them, and the paste's read runs before `fromJson` | The walk runs in `commandFromArgs` after `protojson` has accepted the arguments; `valuelessCurrent` runs after `fromJson` and the controller and kind refusals, reading the paste as written | The walk reads the arguments by the command's descriptor and trusts the shapes `protojson` accepts, as the warning at its call says, so it is handed only arguments that decode, and a malformed argument is refused in `protojson`'s words as any other is. The paste's read likewise sees only an actor `fromJson` accepted, and still reads what was written, since `fromJson` reads a `current` with no value as `0`. |
| `git diff main -- contract ':!contract/gen/tools'` prints nothing (plan, Phase 4b) | It prints `contract/testdata/expected_tools.json` | That file is the golden `TestToolsMatchGolden` reads, and it changes with the tool's docs. No `.proto` file changed. |
| Rows R1 to R8 | VTT-669 and VTT-673 withdrawn; VTT-677 to VTT-681 added | Phase 4a and the reading reviews, above. The plan's id list stops at R8. |
| `handleCommand` gains a key check after `validateAddActor` | `validateAddActorWith` does both | A second call pushed `handleCommand` over the lint gate's complexity bound; the bound is not raised to pass it. |
| The MCP walk as first written, never committed | Removed: a lookup of a field's descriptor by its proto name, and an arm for repeated fields; a guard for maps of numbers was removed for a while and restored. `valueOf` reads a field's value by either name, which `actor_id` needs | Every message field the walk enters has one spelling for both names (`module_data`, the one with two, is a `google.protobuf.Struct` the walk never enters), and no command holds a repeated message field, so neither branch could change an outcome and each would have been a mutant nothing can kill. The guard came back once the reading review showed what it protects and that its negation is killed. A warning names when a repeated field needs an arm. |
| The plan's "The existing tests and data that change" | Also changed: `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication` now wants `attributes` and `resources` to say "only the properties this actor has" where it wanted "opaque", and `internal/adventure/testdata/at-every-boundary/guide.md` names `val.Max` where it named `rv.Max` | D6 rewrote those docs, which no longer call a sheet's maps opaque now that their keys are read against the ruleset; and `actorValues` reads the decoded maximum into `val`, which the guide's table cites as the check that a `max` of `0` is no maximum. |
| Task 4: `task check` on the last code commit, `b428516` | On `6cabbe6`, which merges main's lint fixes | The first run, on `b428516` under golangci-lint 2.11.4, was stopped by a free-space watchdog during the race tests. Then 2.11.4, which the repository linted with, left the machine, and under the 2.15.0 that remained lint went red on code this branch does not change. Moving the repository to 2.15 became its own ticket at the owner's choice (2026-11-12), merged to main as `301cb10`. The merge also gives this branch's register VTT-682, which was dispensed on the lint branch, before this branch dispenses another id. |

## What could not be established

- Whether any writer other than the browser and the MCP tool sends the
  gateway a `current` left out or null, beyond the one that ships in the
  binary, `vtt client run`'s scenarios, which no participant plays through.
  Such a command reads as `0`.
- The identity store's close on a refused boot: no test observes it.
- The client's mutation on this branch: `task check` judged nothing, since
  the stored report measured `eb0fa35` and this branch changes
  `client/src/commands.ts`. `valuelessCurrent` has not been through a full
  run; `task check:ts-mutation:full` measures it when the owner asks for it.

## What was deliberately left out

- Forms, defaults, a check that a sheet holds every property, a resource's
  start carried by its first change, and the console's choice of form: the
  owner, 2026-11-08.
- A command that changes a sheet's keys, which a refused campaign would need
  to be brought up to date from inside: not asked for; the DM serves it with
  a ruleset it fits, or with none, removes the actor that does not fit and
  adds a corrected one under a new id.
- A line in the README's "Content directories" for the boot's new refusal:
  that section names no refusal of a campaign's actors, and says nothing the
  refusal makes false. The update of SPEC-024 that follows this report
  records it.
- The sheet view (right-click, "Show character sheet"), friend and foe, and
  things off the board: step 1's later tickets.
