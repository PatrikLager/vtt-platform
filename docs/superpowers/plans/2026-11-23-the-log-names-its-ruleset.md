# Plan: the log names the ruleset and the version a campaign is played under

- **Ticket:** `docs/superpowers/specs/2026-11-23-the-log-names-its-ruleset-design.md`
  (blueprint principle 28; ADR-012 §1, §19 and "Build order", step 1 item 3).
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-23 by an agent that did not write the ticket, against
    `8c06de7` on `feat/the-log-names-its-ruleset` in the worktree
    `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker, gremlins, `task build:client` and `requirement-id`
    were not run. Probes P1, P2, P6 and P7 ran through `go test -overlay`
    (for `cmd/vtt`, `GOFLAGS=-overlay=…`, so the binaries its tests build
    carry it too), their files kept in the session's scratchpad; `git status`
    after each showed only the ticket. The rest are searches and readings.
- **The ticket was revised after this verification**, before Task 0's
  commit, to take in G1 to G9 and the rule 9 corrections of check 1. The
  verdict, the checks and the gaps below record the draft that was verified:
  where they find the ticket wrong or short, they describe that draft; the
  committed ticket has taken each of those in.
- **To be committed as:** `docs/superpowers/plans/2026-11-23-the-log-names-its-ruleset.md`.
- **Base:** `8c06de7`. **Branch:** `feat/the-log-names-its-ruleset`.
- **Owner's rulings taken in:**
  - 2026-11-13: this is ADR-012 step 1 item 3, taken after item 4 and
    before item 5, whose references will depend on it.
  - 2026-11-23: (1) a campaign served under a ruleset of another id than its
    log names has the switch recorded, not refused; (2) nothing checks that a
    ruleset's files changed under one version — no fingerprint, no digest;
    the version is the author's to keep true, as in MapTool; (3) every
    participant — the DM, the agent, each player, each spectator, and every
    character log — is sent the entry.
  - 2026-11-24, asked by Task 4's reading review: a `take_ability` after a
    ruleset change also opens its batch with the entry, since the resource
    it removes is chosen by the ruleset's `Spends`; a `give_ability` does
    not, since what it writes is the DM's. Task 4b carries it.
  - Out of scope, by the ticket: a ruleset's options; the version in
    `/api/ruleset` or `get_ruleset`; the ruleset an adventure declares;
    `vtt mcp --ruleset`.
  - Standing: a near-theoretical edge case with a small consequence gets no
    code, no test and no debt (2026-10-21); one source per fact, pointed at
    (2026-09-23); nobody uses the product, so no migration (2026-09-04);
    gate and tooling work is paused (2026-08-27); no counts or measured
    figures in prose meant to last; a code comment is a warning or a pointer
    (rule 10); the client's full mutation run only when the owner asks; push
    and merge on his word.
- **Signed off:** 2026-11-23 by the owner: OD-1 A, OD-2 A, OD-3 A, and the
  plan "Godkänd" with OD-4 A and D3's, D2's and D12's preferences
  (`RulesetNamed`, a version bounded at 128 bytes, both shipped rulesets at
  `"1"`).
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off. The
  `R` labels below are this plan's; each task's citations read through this
  list.
  - R1 VTT-970, R2 VTT-971, R3 VTT-972, R4 VTT-973 (succeeds VTT-861 in
    Task 1), R5 VTT-974.
  - VTT-986, added by the sort: the schema describes `version` as required,
    non-empty and at most 128 bytes, in VTT-858's shape. Its test names
    `version` in the schema's `required`; dropping it from `required` turns
    that test red.
  - R6 VTT-975, which also holds R7 and R8's entry: one rule, the served
    ruleset named first in the batch of a use the server resolves to an
    outcome against a state whose log names no ruleset, or last names another id or another
    version; its test has a case for each, and D9's breaks "compare the id
    alone" and "the version alone" turn those cases red.
  - VTT-976, added by the sort: the entry carries the issuer's participant
    id and role, as VTT-307, VTT-562 and VTT-799 hold for other batches
    (principle 10); red when the entry is stamped with its event id and time
    but not the issuer's participant id and role (Task 4).
  - R8's other half VTT-977 (served, not refused), R9 VTT-978. Both
    VTT-975 and VTT-978 are read against the state the use was resolved
    against, so D5's race leaves both true.
  - R10 VTT-979, and its dump clause as its own row, VTT-987, in VTT-704's
    shape (Task 2's break "write the ruleset's keys in `foldToDumpJSON`
    when empty").
  - R11 VTT-980, without "without its issuer", which VTT-272 holds.
  - R12 VTT-981 (succeeds VTT-663 in Task 3; the entry sits right after the
    open session, before the party, as D7 places it), R13 VTT-982 (succeeds
    VTT-318 in Task 3), R14 VTT-983.
  - R15 split: VTT-984 (succeeds VTT-822 in Task 2, with `rulesetNamed`
    added) and VTT-985 (the Story and the Events panel word it by the
    ruleset's name and its version). Its exact words are not a row; the
    tests pin them, as the precedents' sorts ruled.
  - Task 4b, allocated after the owner's ruling of 2026-11-24: VTT-988 (a
    take after a change opens its batch with the entry), VTT-989 (a take
    under the ruleset the log names, or with none served, writes none),
    VTT-990 (a give writes none); Task 4b's review added VTT-991, which
    succeeds VTT-767 (a take's events after any entry), and VTT-992 (a take's
    result carries its batch's first sequence).
  - Refused besides, each held elsewhere: R7 (VTT-975); the shipped
    rulesets' versions (VTT-971 and the conformance suite); the DM's and the
    agent's streams (VTT-377); a campaign served with no ruleset (VTT-132);
    and D5's repeated entry, near-theoretical (the owner's ruling of
    2026-10-21).

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol it names is where it says: `manifestJSON`,
   `loadManifest`, `checkManifestBounds`, `decodeStrict` and `formatVersion`
   in `internal/rules/load.go`; `Ruleset` with `ID`, `Name` and
   `FormatVersion` in `internal/rules/format.go`;
   `internal/rules/schema/ruleset.schema.json`; both
   `rulesets/*/ruleset.json`; `Envelope` in `contract/vtt/v1/events.proto`;
   `Apply` in `internal/engine/apply.go`; `composeServer`, its
   `rules.Load(rulesetDir)`, `sheetsFit` and `WithRuleset(rs)` in
   `cmd/vtt/serve_compose.go`; `WithRuleset` in `internal/gateway/server.go`;
   `handleUseAbility` and its `rules.Resolve` in `internal/gateway/ruleset.go`;
   `classify` in `internal/perceive/projection.go`; `client/src/fold.ts`,
   `client/src/view/feed.ts`, `client/src/view/spectator.ts`,
   `cmd/vtt/webdist`, `cmd/vtt/harness_boot.go`, `internal/harness`,
   `internal/eventgen`, `scenarios/goldens`, `client/e2e/setup.ts`,
   `client/test/fold-parity.test.ts`; every specification under
   "Specifications this moves". The append search the Problem quotes prints
   what it says (P3), each function it names defined once in
   `internal/gateway`. MapTool's `library.error.emptyVersion`,
   `PersistenceUtil.saveAddOnLibraries` and `loadAddOnLibraries`, and
   `AddOnLibraryManager.registerLibrary` exist (P13); what two of them do is
   not quite what the ticket says (G7).
2. **"Done" is an observation. By command for item 1, by reading for the
   others.** Each item names what will be true and what shows it; no test it
   names exists yet.
   - Item 1: a `ruleset.json` declaring `version` is refused today as
     `json: unknown field "version"`, both shipped rulesets and
     `internal/rules/testdata/valid` alike (P1); a manifest whose id is longer
     than an id's bound, or whose name is longer than a name's, loads today
     (P2). Item 1's tests therefore fail on `8c06de7`.
   - Items 2, 3 and 4: no payload of `Envelope`'s oneof names a ruleset
     (reading `events.proto`), so a test that reads the log for one fails
     today. Item 4's "the server starts" holds today — `composeServer` reads
     no ruleset id — and its "an entry precedes the next outcome" does not.
   - Item 5 holds today, vacuously: no entry is ever written. It is the
     guard the new comparison must keep, not work the ticket asks for; its
     tests pass before and after, and what shows they bite is their
     deliberate break (Task 4).
   - Item 6 names projection tests; it does not observe a log that opens or
     resumes after the entry was written (G2).
   - Item 7: today the client fold skips a variant it does not know
     (`default: return`), so "folds without a fold error" would hold for a
     regenerated contract with no arm. What item 7 adds is the parity, which
     the goldens hold once they carry the entry (Task 5).
   - Item 8 names the gate.
3. **Each rule is breakable. By reading.** Each rule line maps to a row of
   "Requirements" with what turns it red, or is refused there with what
   already holds it. "A campaign served with no ruleset gains no entry" holds
   by construction under OD-1's recommendation and is refused at the sort
   under it (see there).
4. **The scope matches the claim. By command, then reading.**
   - Callers (P3, P4, P9, P10): `loadManifest` is called by `Load` alone;
     `rules.Load` by `composeServer`, `cmd/vtt/mcp.go`, and the tests and
     conformance suites that load the fixtures; `rules.Resolve`'s one appending
     caller is `handleUseAbility` (`internal/rules/conformance` resolves without
     appending); `handleUseAbility` is called from `handleCommand` alone, with
     the snapshot `authorize` took; `opening` and `resume` from `fanOutLocked`
     alone; `describe` from the Story's `tell` and `renderTicker`.
   - The ticket names files that, under OD-1's recommendation, do not change
     (`cmd/vtt/serve_compose.go`, `cmd/vtt/harness_boot.go`,
     `internal/harness`'s scenario runner, `client/e2e/setup.ts`), and omits
     files that must (G5).
   - The fixture search it names overcounts (G6).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-018 records "its manifest's id, name and `free_usage_name`" among
     the ruleset strings left unbounded by decision. Item 1's sub-bullet bounds
     the id at load, and under OD-3's recommendation the name too. The ticket
     lists SPEC-018 and states the bound, but not that it reverses that
     sentence (G4). The reversal follows SPEC-018's own rule — a value that
     reaches the log does so only as a field of its first table — and
     `AdventureLoaded`'s file-sourced id and name are its precedent.
   - Blueprint principle 10, "Every entry names the participant who caused
     it": an entry written at boot is caused by no participant. The ticket
     offers the boot as meeting every rule without saying so. OD-1's
     recommended option keeps principle 10; option C would have to overturn or
     narrow it, and says so.
   - The harness's binding assumption that a scenario starts on a fresh
     campaign (`Scenario`'s doc, `errFreshCampaignRequired`) is not shifted by
     a boot-time entry, as the ticket's "What could not be established" says:
     it is refused (P7). Option C must change it.
   - ADR-012 §1 and §19 say every outcome names the ruleset, its version and
     its options. The ticket names them in an entry before the outcome, which
     principle 28's effect ("can be named") allows (D14); options are out of
     scope, by the ticket.
   - SPEC-007's convention that a result's `sequence` names the first world
     event the command produced is kept under OD-1 A: the first use after a
     change answers the entry's sequence.
   - SPEC-021's "Nothing appended earlier is forwarded into it" stands: the
     opening's entry is made from the world before the event, as its
     `SessionStarted` is (D7).
6. **The records the work moves are named. By command, then reading.** Every
   path under "Specifications this moves" resolves, and the new one is
   written `New:`. By reading: SPEC-012 states that `handleUseAbility`'s
   batch is what `rules.Resolve` returns, stamped; under OD-1 A it gains the
   entry, and SPEC-012 is not listed (G3). SPEC-025 describes the boot
   (`sheetsFit`); it moves only under OD-1 C. SPEC-024's heading "The
   version, read first" means `format_version`; with a `version` field
   beside it, the heading is ambiguous and moves with SPEC-024.

## Notes on the problem statement, not failures

- "Every append answers a command today" is true as measured (P3), and it is
  also what principle 10 rests on; that is why it matters to OD-1.
- "An outcome is the batch `rules.Resolve` returns" is true at `8c06de7`: no
  other path writes an outcome (P4). ADR-012 step 2 adds resolutions that
  write batches at points; D9's helper is the seam they will call.

## The gaps this plan inherits

- **G1. The Events panel is not a choice.** `renderTicker` draws every entry
  of the log a DM's or an agent's window holds (VTT-447), and `describe` has
  no `default`, so `client:typecheck` fails until the new kind is worded.
  `STORY` is a `Record<Kind, boolean>`, so the Story needs a ruling too.
  "Whether a window draws a line" reduces to the Story's (OD-2) and to the
  words (OD-3).
- **G2. Logs that open or resume after the entry.** SPEC-021's opening holds
  the open session, the party and the public notes, and forwards nothing
  appended earlier. A party member added after the first outcome would hold
  outcomes with no entry before them in its own log, and a log paused across
  a correction would never hold the new version. The ticket's rule ("every
  character log is sent the entry") covers both; its Done item 6 does not
  observe them. D7 and R12, R13 do.
- **G3. SPEC-012 moves under OD-1 A and is not listed.** Task 7 moves it.
- **G4. SPEC-018's sentence on the manifest's id and name is reversed and not
  named.** Task 7 rewrites it.
- **G5. "What it touches", under OD-1 A.**
  - Unchanged: `cmd/vtt/serve_compose.go`, `cmd/vtt/harness_boot.go`,
    `internal/harness`'s runner, `client/e2e/setup.ts`.
  - Not named, and changed: `internal/engine/state.go` (`State`,
    `Snapshot`); `internal/campaign/fanout.go` (`opening`, `resume`);
    `internal/harness/soak.go` and `internal/campaign/scenario_test.go`
    (each `statesEqual`); `client/src/state.ts`; `contract/roundtrip_test.go`
    and a `contract/testdata` fixture; the oneof walkers
    (`TestEveryEnvelopePayloadArmHasAnExplicitRuling`,
    `TestNoEventPayloadNamesARole`, `TestQANoEventPayloadNamesARole`, and the
    client tests that walk `EnvelopeSchema`'s payload); the client tests that
    list every kind (`feed.test.ts`, `qa-story-mechanical.test.ts`,
    `spectator-view.test.ts`, `qa-what-a-window-shows.test.ts`,
    `app.test.ts`'s `EVERY_KIND`, `qa-companion-first-sight.test.ts`'s
    `MECHANICAL`); `internal/rules/qa_declared_sheet_test.go`, which pins the
    schema's top-level fields and its `required`; the gateway tests P6 turned
    red; `internal/eventgen/model_test.go`; `docs/requirements.md`.
- **G6. The fixture search overcounts.** `git grep -l format_version` lists
  every map, adventure, art sidecar and campaign file, which share the field
  name. The set this work edits is the ruleset manifests and the inline
  manifest literals (P5).
- **G7. Rule 9: two of the ticket's MapTool claims, corrected (P13).**
  `library.error.emptyVersion` is raised by
  `Objects.requireNonNull(dto.getVersion(), …)` in `AddOnLibrary`'s
  constructor, and `AddOnLibraryDto` is a proto3 message parsed by
  `JsonFormat`, whose `string version` getter never returns null; so the
  check never fires, and MapTool loads a library with an empty or absent
  version. `registerLibrary` does refuse a second library under one
  namespace, but the import dialog
  (`AddOnLibrariesDialogController`) asks
  `library.error.addOnLibraryExists` — "already exists. Do you want to replace
  it?" — and on yes deregisters the old and registers the new: an operator
  replaces a library mid-campaign, and nothing records it. Neither changes
  what is borrowed; the first means the non-empty rule is this platform's own.
- **G8. The alpha campaign.** `~/dev/vtt-alpha/serve.log` and
  `vtt-mcp.sh` serve `rulesets/dnd45e-minimal` from the shared tree, not
  `tavern-brawl` as the ticket says (P14). Its log gains the entry at its
  first outcome once this lands; the outcomes already in it name no ruleset,
  and nothing can make them (the log only grows; no migration).
- **G9. The ticket's third rule is false as worded for a log written before
  this lands** (G8's earlier outcomes). R6's row is worded for outcomes
  appended under a ruleset that declares a version.

## Measurements this plan stands on

All at `8c06de7`.

- **P1.** An overlay test in `internal/rules` copies each of
  `rulesets/tavern-brawl`, `rulesets/dnd45e-minimal` and `testdata/valid`,
  adds `"version": "1"` after `format_version`, and calls `rules.Load`: each
  is refused, `json: unknown field "version"`.
- **P2.** The same, with `tavern-brawl`'s id made one byte longer than
  `maxIDBytes`, and separately its name made longer than the engine's
  `maxNameBytes`: both load.
- **P3.** `git grep -nE '\.Append(Batch)?\(' -- '*.go' ':!*_test.go'
  ':!contract/gen'` prints `Campaign.appendLocked`'s write and lines inside
  `appendChange`, `handleLoadAdventure`, `handleLoadMap`, `handleUseAbility`,
  `handleCommand`, `handleRemoveActor`, `handleRemoval` and
  `handlePromotion`; `cmd/vtt` appends nothing.
- **P4.** `git grep -n 'rules\.Resolve('` over non-test Go prints
  `handleUseAbility` and `internal/rules/conformance/conformance.go`, which
  resolves without appending.
- **P5.** `git ls-files '*ruleset.json'`: every manifest but
  `internal/rules/testdata/invalid/bad-format-version` and
  `invalid/format-version-1` holds exactly one line matching
  `^\s*"format_version": "3",$`; those two are refused on `format_version`
  before any other field is read (SPEC-024). The inline literals are what
  `git grep -n -F '"format_version": "3"' -- '*.go' '*.ts'` and
  `git grep -n -F '"format_version":"3"' -- '*.go' '*.ts'` print: Go test
  files in `internal/rules`, `internal/gateway` and `internal/adventure`, and
  none in TypeScript.
- **P6. OD-1 A, measured.** An overlay of `internal/gateway/ruleset.go` puts
  an inert envelope (`AttackRolled{}`: a no-op in both folds, forwarded to
  every log because it names no actor) first in the batch of each server's
  first use. Red: `TestUseAbilityHitProducesBatchFirstSequence`,
  `TestQARuleAUseWhoseIDsAreAtTheBoundIsRecordedWhole`,
  `TestQAUseEachRoleIsRefusedAnAbilityTheSheetDoesNotName`,
  `TestQAUseTheRefusedUsesWouldResolveForAnActorWhoseSheetNamesThem` and
  `TestQAUseEachRoleResolvesAUseTheSheetNames`, each because it reads the
  first event at the result's sequence as the `AbilityUsed`; and in
  `cmd/vtt`, `TestScenarioGoldenStreamsHaveNotDrifted` for `adventure-night`,
  `door-watch` and `toy-brawl`. Green: `internal/mcp`, `internal/harness`,
  `internal/campaign` and the rest of `cmd/vtt`, MCP end-to-end tests
  included.
- **P7. OD-1 C, measured.** An overlay of `cmd/vtt/serve_compose.go` appends
  the same inert envelope after `WithRuleset`. Red, every one with `harness:
  scenario requires a fresh campaign; found 1 pre-existing events`:
  `TestScenarioLibraryRunsSelfContained` for `adventure-night`, `door-watch`,
  `goblin-fight` and `toy-brawl`, and `TestScenarioGoldenStreamsHaveNotDrifted`
  for the three with goldens. Each of those scenarios lists the DM first,
  whose catch-up `RunScenario` drains. Green: the rest of `cmd/vtt`. The
  soak boots no ruleset (`buildSoakDialer`). The probe appends at every boot,
  so it is an upper bound for boots of a log that already names the ruleset.
- **P8.** By reading `internal/campaign/fanout.go`: `opening(before)` emits
  the open session's `SessionStarted`, then `Perch` through no eyes, then
  each public note; `resume(fold, before)` ends and starts sessions, then
  hides every held token; `fanOutLocked` forwards each envelope through
  `perceive.Projection.Event`.
- **P9.** `engine.State.Snapshot` builds a fresh state and copies each field
  by name; `internal/harness/soak.go`'s `statesEqual` and
  `internal/campaign/scenario_test.go`'s compare each field by name. A field
  either omits is dropped or ignored silently.
- **P10.** `client/src/view/feed.ts`'s `STORY` is a `Record<Kind, boolean>`;
  `describe` in `spectator.ts` switches over every case with no `default`;
  `renderTicker` draws `describe` for the newest entries of the log.
- **P11.** `client/e2e/setup.ts` boots every spec's server with `--ruleset
  rulesets/dnd45e-minimal` on a fresh campaign. `conditions.spec.ts` and
  `table.spec.ts` use an ability. `table.spec.ts` asserts that no Events row
  matches `/#\d/` and that no Story line contains ` vs `.
- **P12.** In the three ruleset goldens, the sequences at or after the first
  use's batch are each session's `EndSeq`, a condition's `AppliedSeq` where
  applied after it, and `headSequence`; no scenario definition names an
  absolute sequence.
- **P13.** MapTool at RPTool `maptool` `f4b7fef6c`: `AddOnLibrary`'s
  constructor, `messages/src/main/proto/library.proto`'s `AddOnLibraryDto`,
  `AddOnLibraryImporter`'s `JsonFormat.parser()`,
  `AddOnLibraryManager.registerLibrary`, `LibraryManager.addOnLibraryExists`,
  `AddOnLibrariesDialogController`'s import action,
  `PersistenceUtil.saveAddOnLibraries` (each library stored under its MD5) and
  `loadAddOnLibraries`.
- **P14.** `~/dev/vtt-alpha/serve.log`'s first line and `vtt-mcp.sh` name
  `rulesets/dnd45e-minimal`; its server was not running.
- **P15.** Register rows false as worded once this lands: VTT-861 (its list
  of top-level fields), VTT-663 (what an opening holds), VTT-318 (a resume's
  first entries), and, under OD-2 A, VTT-822 (the kinds the Story shows).
  VTT-137 stays true; its evidence test is re-pointed.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no gate moves; a test whose assertion changes
  changes because a decision here changed the rule, and the task says which);
  3 (additive: one message and the arm `ruleset_named = 40` of
  `Envelope.payload`, never a gap or a renumbering; `check:breaking` reports,
  it does not enforce); 4 (`engine.Apply` is the one fold: the ruleset the
  log names is folded there and nowhere else, and the gateway reads it from
  the snapshot); 5 (`ruleset` and `version` pass semgrep; no game words in
  tests); 8 and 10 (names, not line numbers; a new comment is a warning or a
  pointer, to SPEC-026 or SPEC-007); 9 (answered below).
- **Principles** 4 and 11 (both folds judge the entry alike), 5 (the new
  bounds are set with the event, the only time they are free), 10 (the entry
  names its issuer), 13 (the words a window shows), 28.
- **ADR-007** for the contract; **ADR-009** for the goldens: `state.json` is
  derived by hand before `stream.json` is recorded
  (`scenarios/goldens/README.md`).
- **The project's mechanical gates**, as
  `docs/superpowers/plans/2026-11-14-a-sheets-abilities-change-in-play.md`
  lists them, and the Go mutation gate over `tools/check-mutation.py`'s
  `PACKAGES`, which hold `internal/rules`, `internal/engine`,
  `internal/perceive`, `internal/campaign` and `internal/gateway`: every new
  bound gets a test at the bound and one past it. A client change is
  committed with its `cmd/vtt/webdist`, rebuilt with `task build:client`;
  client tests are measured with `env -u CLAUDECODE`; a new
  `contract/testdata` fixture is followed by `ts_mutation_inputs_test`
  before the commit; `tools/check-new-prose` and
  `python3 tools/check-comments.py` run before each commit; stage and commit
  in separate calls.

## Rule 9: how MapTool does this

Read at RPTool `maptool` `f4b7fef6c` (P13, G7):
- An add-on library's `library.json` names a `namespace` and a `version`,
  free text. The `emptyVersion` check is written and cannot fire, so in
  practice a version may be empty.
- A campaign file keeps each library's current copy, stored under its MD5
  (`saveAddOnLibraries`), and reloads it (`loadAddOnLibraries`).
  `registerLibrary` refuses a second library under a namespace; the import
  dialog replaces one after the operator confirms. Nothing records which
  version decided an outcome, or when one replaced another.
- **Borrowed:** the version as free text the author declares, and whose truth
  is the author's (the owner's ruling 2, D8).
- **Not borrowed:** the copy kept inside the campaign (a ruleset stays a
  directory the operator serves); the silence (the log names it, principle
  28); the empty version (refused at load here, D1, since the check MapTool
  meant to make is the one this work needs); the MD5 (the owner's ruling 2:
  no digest).

## Decisions

1. **D1. A ruleset declares `version`.** `manifestJSON` gains `Version
   string \`json:"version"\``; `loadManifest` refuses it empty, absent
   included, as `field "version": must not be empty`, as it does `id` and
   `name`; `loadedManifest` and `rules.Ruleset` carry `Version`, which
   `loadV2` copies. `ruleset.schema.json` lists `version` as required, a
   string of `minLength` 1 whose description states its bound, and the
   descriptions of `id` and `name` state theirs (D2): VTT-625's test reads
   the `id`'s, and VTT-986 the `version`'s. **Forced by** item 1, the first
   rule and VTT-625.
2. **D2. The bounds, at load and in the fold.** `checkManifestBounds` refuses
   an `id` or a `version` longer than `maxIDBytes` and, under OD-3 A, a
   `name` longer than a new `maxNameBytes` (256, mirroring
   `internal/engine`'s), each naming the field. Both folds refuse a
   `RulesetNamed` whose id or version is empty or longer than 128 bytes, or,
   under OD-3 A, whose name is empty or longer than 256, in that order.
   **Forced by** item 1's sub-bullet and principle 5: a bound added after the
   first log holds the event refuses that log. **Preference:** the version's
   bound is an id's (128), not a name's (256), since it is half of what the
   comparison reads (D8); either meets the ticket.
3. **D3. The event.** `message RulesetNamed { string ruleset_id = 1; string
   version = 2; string name = 3; }` — `name` only under OD-3 A — and the arm
   `RulesetNamed ruleset_named = 40;` of `Envelope.payload`. The message's
   comment points at SPEC-026, the arm's at SPEC-007. **Forced by** rule 3
   (past tense, the next free number after 39). **Preference:** the name
   `RulesetNamed`; `RulesetServed` reads wrong under OD-1 A, where the entry
   is written at an outcome rather than at serving.
4. **D4. Where "the ruleset the log last names" lives.** `engine.State` gains
   `RulesetID string \`json:",omitempty"\`` and `RulesetVersion string
   \`json:",omitempty"\``, set by `Apply`'s new arm and copied by `Snapshot`;
   both `statesEqual` compare them. The client's `State` gains both as
   optional fields, set by `fold.ts`'s arm, and `foldToDumpJSON` writes each,
   between `Notes` and `Scenes`, only when non-empty. **Forced by** rule 4 (the
   gateway compares against what the fold holds, not a second reading of the
   log), principle 11 and `fold-parity.test.ts`'s byte comparison;
   `omitempty` keeps every dump of a log with no entry byte-identical, so no
   golden moves before Task 5. Under OD-3 A it holds the name too, as
   `RulesetName`, for the opening and the resume to write it (D7); nothing
   compares it (D8).
5. **D5. The fold accepts an entry naming what it already names.** Two first
   uses after a change, sent at once, take snapshots that name the old
   version, and each batch begins with an entry; both land. The consequence is
   one repeated entry, near-theoretical, so no fold refusal and no test (the
   owner's ruling of 2026-10-21). A refusal could be added later without
   breaking a log; one added now would refuse the second use with an engine
   message.
6. **D6. The projection forwards it to every viewer.** `classify`'s arm
   returns `forwarded`, as for `SessionStarted`, whatever the viewer's eyes
   see; `forwardable` clears the issuer. **Forced by** the owner's ruling 3.
7. **D7. A character log opens and resumes naming the world's ruleset.**
   `opening(before)` emits, when `before` names a ruleset, a `RulesetNamed`
   of it after the open session's `SessionStarted` and before the party;
   `resume(fold, before)` emits one after its session entries and before the
   `TokenHidden`s, when `before` names a ruleset and `fold` names another id
   or version, or none. Under OD-3 A the emitted entry carries the name
   `State` holds (D4). **Forced by** the ticket's eighth rule and G2.
8. **D8. The comparison reads the id and the version alone.** Not the name,
   not a file, not a digest (the owner's ruling 2).
9. **D9. Under OD-1 A, the gateway writes it.** A `Server` method compares
   `st.RulesetID` and `st.RulesetVersion` with `s.ruleset`'s and returns a
   `RulesetNamed` of the served ruleset when either differs, else nil; its
   doc comment warns that every path appending an outcome calls it first
   (SPEC-026). `handleUseAbility` puts the entry first in `rules.Resolve`'s
   batch, before the stamping loop, so it carries the issuer, an event id and
   the time, and the campaign stamps its session; the result answers its
   sequence (SPEC-007). `composeServer` is unchanged, so a campaign whose log
   names one id is served under another whose sheets fit (the owner's ruling
   1). P6's tests are re-pointed to find the `AbilityUsed` in the batch by
   kind; `TestUseAbilityHitProducesBatchFirstSequence` asserts the first use's
   batch begins with the entry.
10. **D10. The fixture churn, mechanical and checked.** In Task 1, in this
    order: (a) `Version` decodes but is not yet required; (b) a script inserts
    a line `"version": "1",` after the one `"format_version": "3",` line of
    every manifest P5 finds, at its indentation, failing on any file that
    does not hold that line once, and the two format fixtures are left as
    they are; (c) each inline literal P5 lists gains `"version": "1"` beside
    its `format_version`, by hand; (d) `go test ./internal/rules/...
    ./internal/adventure/... ./internal/gateway/` passes as on `8c06de7`;
    (e) `version` becomes required. After (e), `git grep -L '"version"' --
    '*ruleset.json'` prints only the two format fixtures, and only the new
    tests change outcome. **Forced by** rule 2: (d) shows the edit changed no
    test's outcome before the requirement could.
11. **D11. `internal/eventgen` draws a `RulesetNamed`** of a small set of ids
    and versions, so the property tests and the character-log oracle meet it
    at every prefix, openings and resumes included.
12. **D12. Both shipped rulesets declare `"version": "1"`.** A preference;
    the version is the author's, and the owner may name another at sign-off.
13. **D13. No semgrep change.** `no-direct-state-writes` keys on the shape of
    a map write, and does not see a write to a string field of `State`; its
    own KNOWN GAP comment names that blind spot. Gate work is paused
    (2026-08-27); the reading review checks that only `engine` writes the new
    fields.
14. **D14. ADR-012's "every outcome names".** Through the nearest entry
    before it, which principle 28's effect allows ("can be named"). Options
    stay out (the ticket).
15. **D15. `get_state` and `vtt state dump` show the new keys.** Both marshal
    `engine.State`; their descriptions state the convention, not a list of
    keys, and stay true.

## Open decisions for the owner

- **OD-1. When the entry is written. Recommended: A.**
  - **A: first in the batch of the first outcome after a change** (D9). One
    atomic append: an entry is written only with an outcome it precedes, and
    no entry can fall between them. It names the participant whose use it
    precedes (principle 10), carries that use's session, and is written once
    however many times the server restarts without play. Measured cost (P6):
    P6's gateway tests re-pointed and the three ruleset goldens re-recorded.
    Its own costs: every future outcome path calls D9's helper; a race leaves
    a repeated entry (D5); the first use's result answers the entry's
    sequence.
  - **B: its own append, just before the outcome's batch.** The result still
    answers the `AbilityUsed`, so P6's tests stand. Two appends: another
    command may land between them, an outcome refused after it leaves an entry
    nothing follows, and SPEC-007's convention needs a rewording, since the
    command produced both.
  - **C: at the boot that serves a changed ruleset** (`composeServer`).
    No race, and no helper for later outcome paths. Measured cost (P7): every
    scenario served with a ruleset is refused by the harness's fresh-campaign
    rule, whose binding assumption must change; the entry names no
    participant, against principle 10's effect, which must be narrowed;
    `composeServer` needs an event id from a package that does not export
    one; every e2e campaign starts with it; a restart under another ruleset
    with no play writes an entry naming rules that decided nothing; and
    SPEC-025 moves.
- **OD-2. Whether the Story draws a line for it. Recommended: A.**
  The Events panel draws one in the DM's and the agent's window whatever this
  says (G1).
  - **A: in every window's Story**, whether the entry carries an event id
    (forwarded) or none (an opening's or a resume's). The owner ruled that
    every participant is sent it, and a player's window shows nothing it is
    sent but in the Story; a correction between sessions is what a table
    disputes (principle 28's why), and this is where the table sees where it
    began. VTT-822 is reworded to add the kind.
  - **B: in no Story**, as `adventureLoaded`. Only the DM and the agent see
    it, in the Events panel; a player is sent it and never shown it.
- **OD-3. What the entry carries. Recommended: A under OD-2 A, else B.**
  - **A: the ruleset's id, its version and its name** (D3), as
    `AdventureLoaded` carries an adventure's id and name. A window words it in
    the table's words, "rules: Tavern Brawl, version 1" (principle 13), and
    after a switch an older entry still names its own ruleset. Cost: a third
    load bound the ticket does not state (`name`, D2), SPEC-018's sentence
    moves for the name as for the id, and `State` holds the name (D7).
  - **B: the id and the version**, the ticket's words. A window words it by
    id, "rules: tavern-brawl, version 1", as the Events panel words an
    adventure by id today.
- **OD-4. Which specification records it. Recommended: A.**
  - **A: a new `docs/specifications/026-the-log-names-its-ruleset.md`**,
    which the ticket already writes `New:`: the event, when it is written,
    what the fold holds and how it is compared, the opening and the resume,
    the switch recorded and not refused, and the race. SPEC-024, SPEC-018,
    SPEC-007, SPEC-016, SPEC-021, SPEC-022 and SPEC-012 each move for their own
    part and point at it.
  - **B: an extension of SPEC-024 or SPEC-025.** SPEC-024 records the
    format, not the log; SPEC-025 records the boot's fit, which this touches
    only under OD-1 C. Either would hold a decision about the log in a record
    about something else.

## Requirements

Proposed for Task 0's sort, to be allocated after sign-off. Each row reads
`**OPEN — no test yet**` until its task cites its tests.

| R | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|
| R1 | 1 | `rules.Load` loads a `ruleset.json` whose `version` is non-empty, and `Ruleset.Version` holds it as written | refused, or another value held |
| R2 | 1 | `rules.Load` refuses a `ruleset.json` whose `version` is absent or empty, by an error naming the file and the field | loads |
| R3 | 1 | `rules.Load` refuses a `ruleset.json` whose `id` or `version` is longer than 128 bytes of UTF-8 (under OD-3 A, or whose `name` is longer than 256), naming the file and the field, and loads one whose each is exactly its bound | loads past a bound, or refuses at it |
| R4 | 1, D1 | A `ruleset.json` holding a field the format does not define, at its top level one whose name matches none of `id`, `name`, `format_version`, `version`, `properties`, `sheet` and `free_usage_name`, or within `sheet` one its form does not define, compared without regard to case, is refused when it loads, naming the file and the field (succeeds VTT-861) | loads |
| R5 | D2 | Both folds refuse a `RulesetNamed` whose ruleset id or version is empty or longer than 128 bytes (under OD-3 A, or whose name is empty or longer than 256), naming the field, and accept each at its bound | accepted past a bound, or refused at it |
| R6 | 2 | A `use_ability` under a ruleset that declares a version, on a campaign whose log names no ruleset, or another, appends in one batch a `RulesetNamed` of that ruleset's id and version and then the outcome; no entry naming another ruleset lies between them | no entry, or one after the outcome |
| R7 | 3 | A campaign whose log last names a ruleset at one version, served under the same id at another, gains a `RulesetNamed` of the new version first in its next outcome's batch | no entry |
| R8 | 4 | A campaign whose log last names one ruleset id, served under a ruleset of another id whose sheets fit, is served, and gains a `RulesetNamed` of the new id and version first in its next outcome's batch | refused at boot, or no entry |
| R9 | 5 | A campaign whose log last names a ruleset and version, served under that id and version, gains no `RulesetNamed` at its outcomes, also when the ruleset's other files or its name changed | an entry |
| R10 | D4 | Both folds hold, after a `RulesetNamed`, its id and version as the ruleset the log names, change nothing else, and a dump of a log with none prints neither key | another field differs, or a key printed |
| R11 | 6 | Every character log active at a `RulesetNamed` is sent it, without its issuer, whatever its eyes see | withheld, or an issuer kept |
| R12 | 6, D7 | A character log that opens after the world names a ruleset opens with a `RulesetNamed` of it, after the open session and before the party, and otherwise holds what VTT-663 states (succeeds VTT-663) | opened without it |
| R13 | 6, D7 | A paused log that resumes while its fold names another ruleset id or version than the world's, or none, resumes with a `RulesetNamed` of the world's; one whose fold names the world's resumes with none; otherwise as VTT-318 states (succeeds VTT-318) | missing, or sent when equal |
| R14 | 7 | The client's fold of a `RulesetNamed` agrees with `engine.Apply`'s as `vtt state dump` prints it, refusals included | a dump or a refusal differs |
| R15 | OD-2, OD-3 | A window words a `rulesetNamed` "rules: ⟨name, else id⟩, version ⟨version⟩"; under OD-2 A the Story shows it in every window, with an event id or none (VTT-822 succeeded with the kind added); under B the Story draws none | other words, or drawn against the ruling |

**Refused at the sort**, each with what holds it:
- Both shipped rulesets declare a version: R2, and the conformance suite,
  which loads every shipped ruleset.
- The DM's and the agent's streams carry the entry: they are sent the world
  (VTT-377), which R6 writes it to.
- A campaign served with no ruleset gains no entry: under OD-1 A the
  writers are `handleUseAbility`, which refuses before anything is resolved
  or written when no ruleset is served (VTT-132), and, after Task 4b,
  `handleTakeAbility`, which writes none with no ruleset served (VTT-989).
  Under OD-1 C it is a row.
- Not refused after all: the schema states `version` required. Task 0's
  review found that `TestSchemaRequiredFieldsMatchLoaderEnforcement` builds
  its cases from the schema's own `required`, so it stays green when
  `version` leaves it; the rule is VTT-986.

**Rows a task withdraws or rewords**, each in the task that makes it false:
VTT-861 by R4 (Task 1); VTT-822 under OD-2 A (Task 2); VTT-663 by R12 and
VTT-318 by R13 (Task 3). VTT-137 stays; its test is re-pointed (Task 4).
VTT-767 by VTT-991 (Task 4b).
VTT-625 stays true through Task 1's schema edit: once `checkManifestBounds`
bounds the ruleset's `id`, `TestQAPFSchemasStateTheBoundsLoadEnforces`'s
"ruleset id" probe requires the schema's `id` description to state 128
bytes, which D1 writes; the probe's `inScope` is never changed to pass.

## The existing tests and data that change

By search; each task reruns its own packages first and names what turned red.
- **Task 1:** every manifest and inline literal (D10);
  `internal/rules/qa_declared_sheet_test.go`'s top-level field list and its
  `required` length; `internal/rules/qa_property_format_test.go`'s
  `TestQAPFSchemasStateTheBoundsLoadEnforces`, which turns red on the "ruleset
  id" probe until the schema's `id` states its bound (VTT-625).
- **Task 2:** the oneof walkers and the client tests that list every kind (G5).
- **Task 3:** `internal/eventgen/model_test.go`'s `TestAWalkDrawsEveryKind`.
- **Task 4:** P6's gateway tests.
- **Task 5:** `scenarios/goldens/{toy-brawl,door-watch,adventure-night}`.
- **Unchanged:** `rules.Resolve` and its tests; `composeServer` and
  `sheetsFit` (under OD-1 A); every scenario definition; `client/e2e`'s
  specs, whose Events and Story assertions filter by text (P11).

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** the owner answers OD-1 to OD-4 (and D12's version, if not "1");
  run the `requirements` sort over R1 to R15 and the refusals above, against
  the rulings; allocate ids with
  `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/requirement-id`;
  write the rows; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and the requirements chain passes.

### Task 1: a ruleset declares its version (R1 to R4, VTT-986; D1, D2's load half, D10, D12; item 1)

- **Files:** `internal/rules/load.go` (`manifestJSON`, `loadedManifest`,
  `loadManifest`, `checkManifestBounds`, `loadV2`, `maxNameBytes` under
  OD-3 A); `internal/rules/format.go` (`Ruleset.Version`);
  `internal/rules/schema/ruleset.schema.json`; both
  `rulesets/*/ruleset.json`; every manifest and literal of D10;
  `internal/rules/qa_declared_sheet_test.go`; a new
  `internal/rules/ruleset_version_test.go`: a manifest with a version loads
  and keeps it; one without, and one empty, are refused naming the file and
  `version`; an id and a version exactly at the bound load, one byte past is
  refused naming the field (and the name, under OD-3 A); a case variant
  `"Version"` is read as `version`, as SPEC-024 says of every field; a
  schema test naming `version` in `ruleset.schema.json`'s `required`, with
  `minLength` 1 and a description stating 128 bytes (VTT-986);
  `internal/rules/qa_property_format_test.go`, its assertions unchanged
  and green once the schema's `id` states its bound (VTT-625), its manifest
  literal gaining `version` by D10(c);
  `docs/requirements.md` (VTT-861 → R4).
- **True when:** the new tests pass and fail on `8c06de7` (P1, P2); D10's
  step (d) passed before step (e); `go test ./internal/rules/...
  ./internal/adventure/... ./internal/gateway/ ./internal/mcp/` passes;
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` runs a `version` case.
- **Deliberate breaks:** drop the empty check (R2's tests turn red); drop
  `loadV2`'s copy of `Version` (R1's); drop each bound in
  `checkManifestBounds` in turn (R3's case for that field); drop `version`
  from the schema's `required` (VTT-986's test turns red, by name; so does
  `qa_declared_sheet_test.go`'s count); drop the `id` description's bound
  (`TestQAPFSchemasStateTheBoundsLoadEnforces`'s "ruleset id" probe).
- **The state here:** every ruleset declares a version; nothing reads it.

### Task 2: the event, both folds, the projection and the words (R5, R10, VTT-987, R11, R14's fold half, R15; D2's fold half, D3, D4, D6; OD-2, OD-3)

- **Files:** `contract/vtt/v1/events.proto` (`RulesetNamed`, arm 40); what
  `task generate:contract` writes under `contract/gen`;
  `contract/testdata/ruleset_named_envelope.json` and
  `contract/roundtrip_test.go`; `internal/engine/apply.go` (the arm) and
  `internal/engine/state.go` (`State`'s fields, `Snapshot`), with a new
  `internal/engine` test: each bound at and past, the fields held, nothing
  else changed, `Snapshot` copies them; `internal/harness/soak.go`'s
  `statesEqual` and an internal test beside
  `states_equal_notes_internal_test.go`;
  `internal/campaign/scenario_test.go`'s `statesEqual`;
  `internal/perceive/projection.go` (`classify`'s arm) and a new
  `internal/perceive` test: a log whose eyes see nothing is sent the entry,
  without issuer; `client/src/state.ts`, `client/src/fold.ts` (the arm,
  `foldToDumpJSON`) and the client fold tests
  (`fold-rejections.test.ts` for each bound, a fold test for the fields);
  `client/src/view/spectator.ts` (`describe`), `client/src/view/feed.ts`
  (`STORY`), and the client tests that list every kind (G5);
  `docs/requirements.md` (VTT-822 under OD-2 A); a rebuilt `cmd/vtt/webdist`.
- **True when:** the new tests pass; `TestEveryEnvelopePayloadArmHasAnExplicitRuling`,
  `TestNoEventPayloadNamesARole`, `TestQANoEventPayloadNamesARole` and the
  client walkers pass; `client:typecheck` passes; `TestFoldGoldenCorpus` and
  `fold-parity.test.ts` pass unchanged (no stream holds the event yet);
  `git diff main -- contract/vtt` adds the message and its arm and nothing
  else; `task check:breaking` reports nothing breaking.
- **Deliberate breaks:** drop one bound in each fold (that fold's refusal
  test turns red); drop `Snapshot`'s copy (the engine test); return
  `withheld` from the arm (the perceive test); write both keys in
  `foldToDumpJSON` when empty (VTT-987: parity turns red on every golden;
  under OD-3 A the ruleset's keys are three, D4); flip
  `STORY`'s entry (the Story's every-kind tests).
- **The state here:** both folds and every window read the event; nothing
  writes it.

### Task 3: the character logs (R11's corpus, R12, R13; D7, D11)

- **Files:** `internal/campaign/fanout.go` (`opening`, `resume`);
  new tests beside `internal/campaign/character_log_test.go`: a log that
  opens after the world names a ruleset opens with its entry between the
  session and the party, and its first outcome after has it before; a log
  open before the first entry is sent the world's; a paused log resumes with
  the new version when the world changed while it was paused, and with none
  when the world's equals its own; a reopened campaign writes no second one;
  `internal/eventgen/model.go` (D11) and `model_test.go`;
  `docs/requirements.md` (VTT-663 → R12, VTT-318 → R13).
- **True when:** the new tests pass; `TestALateLogOpensWithThePresent`,
  `TestEveryCharacterLogFoldsAtEveryPrefix`, the oracle and the property
  tests pass with the new draws.
- **Deliberate breaks:** drop the opening's entry (the late-log test turns
  red); emit the resume's entry unconditionally (the equal-fold test); drop
  it (the changed-version test).

### Task 4: the gateway writes the entry before the first outcome after a change (R6 to R9, VTT-976; D8, D9; OD-1 A; items 2 to 5)

- **Files:** `internal/gateway/ruleset.go` (D9's method; `handleUseAbility`);
  a new `internal/gateway/ruleset_named_test.go`, through the WebSocket over
  one campaign directory closed and opened again under each ruleset in turn
  (copies of `tavern-brawl` written to a temporary directory): under R at V,
  a use's batch begins with the entry, the result answers its sequence, the
  DM's stream carries its issuer and a player's does not; a second use adds
  none; reopened under R at V′, the next use's batch begins with V′'s; under
  Q at the same version string as the log last names, with Q's; under R at
  V′ with another name and an ability's description changed, none; with no ruleset, the use is refused and the log unchanged;
  P6's tests re-pointed; a new test in `cmd/vtt` beside
  `TestACampaignWhoseSheetsFitTheRulesetIsServed`: `composeServer` serves a
  campaign whose log names one id under a ruleset of another.
- **True when:** the new tests pass and fail on `8c06de7`; `go test
  ./internal/gateway/ ./internal/mcp/` passes; `go test ./cmd/vtt/` passes
  but `TestScenarioGoldenStreamsHaveNotDrifted`'s three, which Task 5 brings
  back before any push.
- **Deliberate breaks:** drop the prepend (R6's test turns red); compare the
  id alone (R7's); the version alone (R8's); prepend always (R9's); compare
  the name too (R9's renamed case); drop `Snapshot`'s copy again (R9's: every
  use then writes one); stamp the entry's event id and time but not the
  issuer's participant id and role (VTT-976's case of the DM's stream);
  `composeServer` refuses a campaign whose log names another ruleset id
  (VTT-977's `cmd/vtt` test).

### Task 4b: a take after a change opens with the entry too (VTT-988, VTT-989, VTT-990; the owner's ruling of 2026-11-24)

- **Files:** `internal/gateway/ability_change.go` (`handleTakeAbility`: with
  a ruleset served, D9's entry first in the batch, before `appendChange`
  stamps it); `internal/gateway/ruleset_named_test.go`: a take after each
  change (none named, another version, another id) opens its batch with the
  entry and then the `AbilityTaken` and the `ResourceRemoved` the ruleset
  chooses, and the result answers the entry's sequence; a take under the
  ruleset the log names, and one with no ruleset served, write none; a give
  writes none, and the use after it still writes the entry; tests that read
  a take's batch by position, re-pointed.
- **True when:** VTT-988's tests fail on `b53fa12`; VTT-989's and VTT-990's
  pass on it and bite by the breaks below; `go test ./internal/gateway/
  ./cmd/vtt/` passes, the goldens unchanged (`git grep -l takeAbility --
  scenarios/` prints `scenarios/toy-brawl.json` alone, which takes after its
  first use); VTT-767 withdrawn for VTT-991, its evidence and the code's
  citations moved.
- **Deliberate breaks:** drop the take's entry (VTT-988's test); write it
  always (VTT-989's); write it with no ruleset served (VTT-989's, or a
  panic); put it in a give's batch too (VTT-990's).

### Task 5: the goldens (R14; item 7)

- **Files:** `scenarios/goldens/toy-brawl`, `door-watch` and
  `adventure-night`: `state.json` edited by hand first — the two keys (three
  under OD-3 A, D7), the served ruleset's id and version, and every sequence
  P12 names at or after the first use's batch moved by one — then
  `stream.json` recorded, in which the entry opens the first use's batch, its
  rolls the run's own.
- **True when:** `TestFoldGoldenCorpus`, `TestScenarioGoldenStreamsHaveNotDrifted`,
  `TestScenarioLibraryRunsSelfContained`, `fold-parity.test.ts` and the
  corpus half of `TestEveryCharacterLogFoldsAtEveryPrefix` pass.
- **Deliberate break:** the client fold sets the id and not the version
  (parity turns red).

### Task 6: the gate (item 8)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger` once,
  last, if any file's comment share fell (SPEC-010); `go clean -cache`; check
  free space and the load; `task check` whole on the last code commit,
  detached under `/bin/sh`, output to a file, requiring its completion line;
  `task e2e`, where the DM's Events panel in `conditions.spec.ts` and
  `table.spec.ts` gains the row (P11). Tell the owner the client's stored
  mutation report is out of date from Task 2 and which commit it measured
  (SPEC-023).
- **True when:** `task check` and `task e2e` are green.

### Task 7: the report, then the specifications

- **The report:** `docs/reports/2026-11-23-the-log-names-its-ruleset.md`
  with the `implementation-report` skill: each "Done looks like" item with
  its observation; R1 to R15 and what turned each red; rule 9 and G7; G1 to
  G9 and each OD's answer; Phase 4a's adjudications under their own heading;
  the gate.
- **The specifications**, with the `specification` skill, from the report:
  - **SPEC-026 (new, under OD-4 A):** the event; when it is written; what the
    fold holds and what is compared, and what is not (no digest); the opening
    and the resume; a switch recorded, not refused; the race (D5).
  - **SPEC-024:** `version` in the manifest, its refusal and bounds; the
    heading "The version, read first" says `format_version`.
  - **SPEC-018:** the event's rows in the first table; the manifest sentence
    rewritten for the id and the version (and the name under OD-3 A) (G4).
  - **SPEC-007:** the event and its arm; a use's result under OD-1 A; a
    take's batch opening with the entry ("the ability event first").
  - **SPEC-016:** `classify`'s arm.
  - **SPEC-021:** the opening and the resume.
  - **SPEC-022:** the client fold, the Events row's words, the Story under
    OD-2.
  - **SPEC-012 (G3):** `handleUseAbility`'s batch.
  - **SPEC-013 and SPEC-025 (Task 4b):** a take's batch, whose first
    envelope after a change is the entry, not the `AbilityTaken`
    (VTT-988, VTT-991).
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and the surfaces as
signatures and names, never the source: `go doc ./internal/rules` (`Load`,
`Ruleset`), `go doc ./internal/engine` (`Apply`, `State`), `go doc
./internal/perceive` (`Event`), `go doc ./internal/campaign`
(`CharacterLog`), the `RulesetNamed` shape from SPEC-007, and the gateway's
WebSocket surface as SPEC-011 states it. Tell it: never run `task check`,
`task e2e`, Stryker or gremlins; its files are new `qa_` files; semgrep bans
game words in Go tests; go-arch-lint holds the layer imports, and
`internal/perceive`'s tests import only its own package, `contract`,
`engine` and `sight`; the tree is hashed before and after it. Known and not
findings: a repeated entry from two first uses at once (D5); a log written
before this lands holds outcomes no entry precedes (G8, G9); nothing compares
a ruleset's files beyond its id and version (the owner's ruling 2); the
version is free text.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "never", "whatever") names the search that bounds it, P3,
P4 and P5 above all; check each comment under `internal/`, `cmd/` and
`client/src` is a warning or a pointer; check that only `internal/engine`
writes `State`'s new fields (D13); check that `git diff main --
contract/vtt` adds `RulesetNamed` and its arm and nothing else.
