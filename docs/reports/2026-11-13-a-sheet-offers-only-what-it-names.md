# A character is offered only the abilities its sheet names, and a sheet names only abilities its ruleset holds — report

Ticket: `docs/superpowers/specs/2026-11-13-a-sheet-offers-only-what-it-names-design.md`.
Plan: `docs/superpowers/plans/2026-11-13-a-sheet-offers-only-what-it-names.md`.
Branch `feat/a-sheet-offers-only-what-it-names` from main `5547d2b`. Commits:
`9ccef9d` (ticket, plan, register), `5b07b46` (Task 1, the field), `d6b3a8e`
(Task 2, the sheet against its ruleset), `9cf9a72` (Task 3, the projection),
`f3cd27b` (Task 4, the data), `c5cfa87` (Task 5, the use), `d95befd` (Task 6,
the read), `edaeab5` (Task 7, the window, the last code commit) and `33b74a8`
(Task 8's comment ledger). Every code reference below is to `edaeab5`.

## Done looks like

1. [x] **A sheet names its abilities.** `vtt.v1.Actor` gains `repeated string
   ability_ids = 10` (`contract/vtt/v1/events.proto`).
   - The wire: `contract/roundtrip_test.go`'s
     `TestActorAddedWithAbilityIdsRoundTripsInItsOrder` and
     `TestAnActorsAbilityIdsAreAPlainListOfStrings`, and
     `internal/engine/qa_actor_ability_ids_test.go`'s protojson tests.
   - An adventure's actor file: `internal/adventure/actor_abilities_test.go`'s
     `TestAnActorFilesAbilityIdsReachItsActorAddedInTheirOrder`.
   - `add_actor` over the WebSocket:
     `internal/gateway/add_actor_abilities_internal_test.go`'s
     `TestAddActorAppendsTheAbilityIdsItNamesInTheirOrder`, and
     `internal/gateway/qa_add_actor_abilities_test.go`; through the MCP tool,
     `internal/mcp/add_actor_abilities_internal_test.go` and
     `cmd/vtt/qa_campaign_abilities_fit_test.go`'s
     `TestQAAbTheMCPToolAppendsAnActorNamingItsAbilityIdsInTheirOrder`; the
     console's paste, `client/test/commands.test.ts`'s "a pasted actor's
     ability ids reach the wire in their order" and
     `client/test/qa-paste-abilities.test.ts`.
   - Both folds and the client's dump: `client/test/fold-dump.test.ts`,
     `client/test/qa-actor-ability-ids.test.ts` and
     `internal/engine/qa_actor_ability_ids_test.go` (VTT-704, VTT-705,
     VTT-706).
2. [x] **A sheet naming an ability its ruleset does not hold is refused.**
   `rules.Ruleset.SheetAbilityFault` (`internal/rules/sheet.go`), held by
   `internal/rules/sheet_test.go`'s
   `TestSheetAbilityFaultNamesTheFirstIdTheRulesetDoesNotHold` and
   `internal/rules/qa_sheet_ability_test.go`.
   - `adventure.Load`: `TestAnActorFileNamingAnAbilityTheRulesetDoesNotHoldIsRefused`
     and `TestAnActorFilesKeysAreReadBeforeItsAbilities`, and
     `internal/adventure/qa_actor_abilities_test.go`.
   - `add_actor` with a ruleset: `TestAddActorWithARulesetRefusesAnAbilityTheRulesetDoesNotHold`
     and `TestAddActorReadsAnActorsKeysBeforeItsAbilities`.
   - `vtt serve --ruleset`: `cmd/vtt/campaign_abilities_fit_test.go`'s
     `TestACampaignWhoseActorsNameAbilitiesTheRulesetDoesNotHoldIsNotServed`,
     which also opens the refused campaign again, and
     `cmd/vtt/qa_campaign_abilities_fit_test.go`.
   - With no ruleset served: `TestAddActorWithNoRulesetReadsNoAbilityAgainstOne`
     and `TestACampaignIsServedWithNoRulesetWhateverAbilitiesItsActorsName`.
3. [x] **The use of an ability the caster's sheet does not name is refused.**
   `handleUseAbility` (`internal/gateway/ruleset.go`) refuses, after the
   no-ruleset refusal and before `rules.Resolve`, through `rules.Offers`.
   - Every role: `internal/gateway/use_ability_sheet_test.go`'s
     `TestAUseTheCastersSheetDoesNotNameIsRefusedForEveryRole`, which also
     reads the next command's sequence to see that nothing was appended, and
     `internal/gateway/qa_use_unnamed_test.go`.
   - The order against sight: `TestAPlayersUseAtAnUnseenTargetIsAnsweredBySightBeforeTheSheet`
     (`internal/gateway/server_visibility_test.go`) and
     `TestQAUseThePlayersSightGateAnswersBeforeTheSheet`; against
     authorization and no ruleset, `TestQAUseAuthorizationAnswersBeforeTheSheet`
     and `TestQAUseNoRulesetAnswersBeforeTheSheet`.
   - The MCP tool: `cmd/vtt/mcp_ruleset_e2e_test.go`'s
     `TestMCPUseAbilityTheCastersSheetDoesNotNameIsAToolError` and
     `cmd/vtt/qa_use_unnamed_test.go`.
   - A use the sheet names resolves as before: the tests of VTT-137, VTT-482
     and VTT-487, whose actors now name their ability.
4. [x] **The panel offers a character only what its sheet names.**
   `offered` and `sheetNames` (`client/src/player.ts`), through which
   `renderPlayerPanel` lists and arms and `moveCommandFor` counts a selection
   as armed (`client/src/view/player.ts`). `client/test/panel-offers.test.ts`
   and `client/test/qa-window-offers.test.ts`, for a player, a DM and an
   agent.
5. [x] **A player reads only their characters' abilities.** `handleRuleset`
   filters through `abilitiesReadBy` (`internal/gateway/metadata.go`).
   `internal/gateway/ruleset_by_role_test.go`,
   `internal/gateway/qa_ruleset_by_role_test.go` and
   `cmd/vtt/qa_ruleset_by_role_test.go`; conditions, resources and properties
   for every role, `TestEveryRoleReadsTheSameConditionsResourcesAndProperties`;
   `get_ruleset`'s description, `internal/mcp/ruleset_tool_test.go`'s
   `TestGetRulesetSaysWhichAbilitiesEachRoleReads`. Under OD-6 the ruleset
   guide asks the role first: `TestMetadataRulesetGuideRoleTable` and
   `TestARulesetGuideThatDoesNotExistIs404ToTheDMAndTheAgent`.
6. [x] **A change of control reaches the panel without a reload.**
   `readRuleset` and `rereadIfStale` (`client/src/app.ts`).
   `client/test/app.test.ts`'s "a player's window reads the ruleset again
   when its watchable set or flag changes, and only then" and "an older
   ruleset read answering after a newer one is not drawn", and
   `client/test/qa-window-reread.test.ts`. The window also reads again when
   a promotion or a demotion makes it a player's window or stops it being
   one (VTT-712; see Deviations): "a spectator who already controls the only
   party member and is made a player is offered its abilities" and "a player
   made a spectator reads the ruleset again".
7. [x] **A log holds no ability list it may not.** `shownWith`'s allow-list
   leaves the list out; `sameIncarnation` compares it
   (`internal/perceive/projection.go`).
   `internal/perceive/companion_abilities_test.go`'s
   `TestACreatureIsIntroducedWithoutItsAbilities` and
   `TestAnUnseenCompanionIsIntroducedWithoutItsAbilities` are the guards the
   ticket asks for, red only under the break that copies the list into
   `shownWith`'s literal; `TestACompanionNamingOnlyAbilitiesIsBroughtWholeAtItsFirstSight`
   and `TestAResumedLogHoldingACompanionWithoutItsAbilitiesIsBroughtToThem`
   were red before `sameIncarnation` compared the list.
   `internal/perceive/qa_actor_abilities_shown_test.go` covers the rest.
8. [x] **The repository's data names the abilities its actors use.** Every
   actor under `adventures/` names the abilities OD-2 gives it; so do the
   `add_actor` steps of `scenarios/goblin-fight.json`, `toy-brawl.json` and
   `door-watch.json`, the adventures' compiled-batch goldens and the
   scenario goldens of `adventure-night`, `toy-brawl` and `door-watch`.
   `TestConformanceOverAdventuresGlob`, `TestFoldGoldenCorpus`,
   `TestScenarioGoldenStreamsHaveNotDrifted` and
   `client/test/fold-parity.test.ts` pass;
   `internal/adventure/conformance/qa_data_abilities_test.go` holds that every
   scenario use is one its caster's sheet names and that the ruleset guide's
   statblocks equal the adventure's actor files. That the data matches OD-2
   exactly was checked by the Phase 4a agent, by Task 4's reading review and
   by the report's, and is held by no test (see Phase 4b).
9. [x] **`task check` exits 0, and `task e2e` passes.** `task check` ran
   whole on `33b74a8`, the ledger commit that follows `edaeab5`: every Go
   package's mutation run has no unadjudicated survivor, and the client's
   mutation was not measured, its stored report having measured `eb0fa35`
   (SPEC-023; see "What could not be established"). The first run was
   refused by `check:mutation`'s free-space floor and passed after
   `go clean -cache`, as the gate's refusal prescribes; between `edaeab5` and
   `33b74a8` only `tools/comment-ceilings.txt` changes. `task e2e` passes;
   the test it skips is `client/e2e/handover.spec.ts`'s own `test.skip`, as
   on `main`.

## What the rules became

The ticket's rules, each with the id it earned:
- With a ruleset served, `add_actor` refuses an actor whose sheet names an
  ability the ruleset does not hold: VTT-685.
- `adventure.Load` refuses such an actor: VTT-687.
- `vtt serve --ruleset` refuses a campaign holding one: VTT-688.
- A `use_ability` whose actor's sheet does not name the ability is refused,
  for every role: VTT-690.
- `/api/ruleset` answers a player only their characters' sheets' abilities, a
  spectator none, the DM and the agent all: VTT-691, VTT-692, VTT-693.
- The panel offers a character only what its sheet names: VTT-695.
- A character log holds a creature, and a party member its eyes have not
  seen, without the list: VTT-698.

The plan's labels:

| Plan | Id | |
|---|---|---|
| R1 | VTT-683 | an accepted `add_actor` appends the list in its order |
| R2 | VTT-684 | `adventure.Load` reads it, `Compile` keeps it |
| R3 | VTT-685 | `add_actor` refuses an unheld id |
| R4 | VTT-686 | no ruleset, no reading |
| R5 | VTT-687 | `adventure.Load` refuses an unheld id |
| R6 | VTT-688 | `vtt serve` refuses such a campaign |
| R7 | VTT-689 | no ruleset, no refusal at boot |
| R8 | VTT-690 | a use the sheet does not name is refused |
| R10 | VTT-691 | a player reads their characters' abilities |
| R11 | VTT-692 | a spectator reads none |
| R12 | VTT-693 | the DM and the agent read every one |
| R13 | VTT-694 | `get_ruleset` says so |
| R14 | VTT-695 | the panel offers only what the sheet names |
| R15 | VTT-696 | a player's window reads again on a change of set or flag |
| R15b | VTT-697 | the newest read's answer is drawn |
| R16 | VTT-698 | a creature and an unseen companion arrive without the list |
| R17 | VTT-699 | a companion in sight holds it |
| R18a | VTT-700 | `add_actor`'s schema documents `abilityIds` |
| R18b | VTT-701 | `use_ability` says an unnamed use is refused |
| R18c | VTT-703 | `add_actor` says an actor uses only what it names |
| R19 | VTT-702 | the ruleset guide is the DM's and the agent's |

Allocated during the work:
- VTT-704, VTT-705 (Task 1, from Phase 4a): the client's dump writes the list
  as `vtt state dump` does; a grant, a revoke and a resource change keep it.
- VTT-706 (Task 1, from the reading review): both folds hold an added
  actor's list as its `ActorAdded` gave it.
- VTT-707, VTT-708 (Task 3): successors of VTT-470 and VTT-661.
- VTT-709, VTT-710, VTT-711 (Task 6, as the plan's "Rows a task withdraws or
  allocates" foresaw): the adventure list, the maps list and the art to every
  role; the conditions to every role; a guide that does not exist is 404 to
  the DM and the agent.
- VTT-712 (Task 7, from the reading review): a window reads again when a
  promotion or a demotion makes it a player's window or stops it being one.

Withdrawn, each by the task that made it false: VTT-470 and VTT-661 (Task 3),
VTT-115 and VTT-117 (Task 6), VTT-438 (Task 7).

Refused at the sort, as the plan's "Requirements" records: R9 (held by
VTT-137, VTT-482 and VTT-487), the halves of R6, R10 and R13 named there, and
the plan's rejected list.

No row says that the sheet's refusal of a use leaves the connection open:
the sort did not foresee it, and VTT-132's causes do not name it.
`TestAUseTheCastersSheetDoesNotNameIsRefusedForEveryRole`'s next command and
`internal/gateway/qa_use_unnamed_test.go`'s `qaUseStillUsable` hold it, and
the specification task records it among SPEC-012's connection-preserving
refusals.

The open decisions, as the owner signed them off: OD-1, the list is
`repeated string ability_ids = 10` on `vtt.v1.Actor`; OD-2, each actor's
abilities as item 8 says; OD-3, the use is refused in `handleUseAbility`,
before `rules.Resolve`, leaving `Resolve` and its goldens unchanged; OD-4,
repeats accepted and no fold bound; OD-5, a Story names by its id an ability
its window cannot read; OD-6, the ruleset guide is the DM's and the agent's;
OD-7, one ticket, one branch.

## The deliberate breaks

Each repaired, each named in its commit:
- Task 1: `copyActor`'s copy dropped; `client/src/fold.ts`'s `actorJSON`
  lost its `ability_ids` line and its omitempty turned to always-write; the
  proto field renumbered.
- Task 2: `SheetAbilityFault` answering nil; ids read in their given order;
  the check dropped from `validateAddActorWith`, `sheetsFit` and
  `loadActors`; abilities read before keys; `buildActor` dropping the list;
  the schema doc without its refusal sentence.
- Task 3: the list copied into `shownWith`'s literal; the comparison dropped
  from `sameIncarnation`; the list compared by length, as a sorted set or by
  emptiness alone.
- Task 4: an id taken out of `toy-brawl`'s `state.json`; the conformance dump
  dropping the list; the dump setting an empty list.
- Task 5: the check removed; `Offers` answering true and false; the check
  moved ahead of the sight gate; each tool text without its sentence.
- Task 6: every role reading every ability; the union over every actor; a
  spectator reading every ability; the DM and the agent reading none; the
  guide's role map dropped; `get_ruleset`'s description reverted.
- Task 7: the panel listing every ability; arming from the unfiltered list;
  `offered` and `sheetNames` answering true; the board click arming an
  unnamed selection, and its empty-selection guard dropped; the re-read
  dropped; the ticket dropped; a DM's window re-reading; a non-player's key
  carrying the set; no re-check after `/api/me`, and a demotion issuing no
  read.

## Rule 9: how MapTool does this

As the plan's "Rule 9" section answers it: the impersonated token's panel
offering that token's own macros is borrowed as the panel; the token's own
copy of each macro's text is not, since principle 27 has the sheet name the
ruleset's entry rather than copy it; nor is MapTool's distribution of every
token to every client.

## The plan's gaps

- G1: SPEC-018 moves; the specification task carries it.
- G2: the files the ticket did not name were changed as the plan's tasks
  listed them.
- G3: item 7 holds by construction; its tests are guards, shown able to fail
  by Task 3's break.
- G4: the alpha campaign's adventure `two-rooms` (`~/dev/vtt-alpha`, outside
  the repository) names no ability. Every use in a campaign it seeded is now
  refused until its actors name abilities; its files need the list before it
  seeds another. When is the owner's.

## Phase 4a adjudications

- Task 1, QA finding 1: spec gap — a bound on the list — OD-4 rules none —
  nothing changed.
- Task 1, QA finding 2: spec gap — the list surviving events that rewrite an
  actor — a rule — VTT-705 allocated.
- Task 1, QA finding 3: spec gap — the folded actor aliasing the event's
  slice — not a rule (both folds copy) — nothing changed.
- Task 1, QA's requirements needing an id — VTT-704 allocated for the dump;
  the rest cite VTT-683.
- Task 2, QA finding 1: requirements needing an id (the caller's order, a
  list written null, a refused boot reopened, a sheet naming none) — each
  held by an existing row — re-cited to VTT-683, VTT-684 and VTT-688.
- Task 2, QA finding 2: spec gaps (a repeat kept, no bound, the path's form,
  null from the paste or the tool, the order across files) — no rule each —
  nothing changed.
- Task 3, QA finding 1: spec ambiguity — SPEC-016 does not name the list, and
  says a character with no values is brought whole "by a correction", which
  cannot carry one — the specification task rewrites SPEC-016.
- Task 3, QA finding 2: spec gap — a companion removed and added again out of
  sight with only another list drops to its unseen form until next seen — as
  VTT-470 had it for any changed sheet — nothing changed; see "What could not
  be established".
- Task 3, QA finding 3: a quoted requirement — held by VTT-698 — re-cited.
- Task 4, QA finding 1: spec ambiguity — the conformance package's golden
  format omitted `ability_ids` — the doc example names it.
- Task 4, QA finding 2: its comments cited a requirement file outside the
  repository — re-pointed (and, after the reading review, removed with the
  table they described).
- Task 5, QA finding 1: requirements needing an id (the exact words, byte
  equality, the tool error) — held by VTT-690 — re-cited; an actor the
  campaign does not hold being left to `Resolve` — no row — a warning comment.
- Task 5, QA finding 2: spec gaps (the tool's error carries the result as
  JSON; the engine answers an unknown ability before an unknown actor;
  `Offers` on no actor; repeats) — nothing changed; SPEC-012's and SPEC-013's
  lists carried to the specification task.
- Task 6, QA finding 1: spec ambiguity — SPEC-012 still describes a route
  served alike to every role — carried to the specification task; principle
  35 of the blueprint holds as written ("by whoever may see it").
- Task 6, QA finding 2: requirements needing an id (the guide route's 401
  before its role; an ability listed once) — no row each (VTT-581 names other
  routes; `Compiled` is keyed by id) — warning comments.
- Task 7, QA finding 1: **behaviour defect** — with a selection the sheet
  does not name, the panel drew it unarmed and said "Click the board to
  move" while the click did nothing — `moveCommandFor` counts a selection as
  armed only when the acting actor's sheet names it.
- Task 7, QA finding 2: spec gaps — a start-up read issued before any
  `WatchableChanged` is followed by another, its answer dropped, or drawn and
  replaced, with nothing to draw it on, since a player's panel lists only
  the watchable set and that set is empty until the first
  `WatchableChanged`; a spectator's window became a player's only on
  reload, which VTT-712 now meets; a failed re-read, see "What could not be
  established".
- Task 7, QA's requirements needing an id (arming from what is offered, no
  heading, a DM's window reading once) — warning comments.

## Phase 4b

One reviewer per task, each re-reviewed until nothing was left at Must or
Should.
- Task 0: rows overclaiming against preconditions (VTT-698, VTT-685) and a
  tool sentence true only from Task 5 (VTT-700 narrowed, VTT-703 allocated).
- Task 1: fold tests citing VTT-683 with no server in their path (VTT-706).
- Task 2: `SheetFault`'s doc said "does not hold" of a key held under the
  other map; a QA test held VTT-154, not VTT-685.
- Task 3: VTT-470 and VTT-661 false once the list is compared; no test held
  the comparison against a same-length list.
- Task 4: a QA test pointed at the plan and held another copy of every list
  (removed; the match with OD-2 stands as checked by Task 4's reading review
  and Phase 4a); the conformance dump set an empty list; the guide's notes
  were false about the wire.
- Task 5: the tavern-brawl guide's "each by its id below" was false; a test
  citing VTT-690 passed with the check removed.
- Task 6: a fixture comment was false; the participant was shadowed by the
  loop variables of the ability and property loops.
- Task 7: a spectator already holding a character and then promoted got no
  abilities until a reload (VTT-712); a fixture had weakened a test.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Task 4 edits the compiled-batch goldens | `internal/adventure/conformance/conformance.go`'s `actorAddedDump` gained `AbilityIDs` | The golden decodes with `DisallowUnknownFields`, so it refused the field, and the dump's own comment says a field it drops is a field no golden pins |
| Task 5: moving the check before the sight gate turns `TestQAASTheRefusalIsAnUnknownIdsWhateverElseTheCommandGetsWrong` red | it stayed green | Moved ahead of sight, the refusal answers an unseen target and an absent one alike, so that test cannot tell; the order shows only as sight answering first whatever the sheet names, which `TestAPlayersUseAtAnUnseenTargetIsAnsweredBySightBeforeTheSheet` now holds |
| D8: a player's window reads again on a change of set or flag | it also reads again when a promotion or a demotion makes it a player's window or stops it being one (VTT-712) | Every joiner is a spectator; a spectator granted a character before being made a player has the same set as a player, so no `WatchableChanged` arrives, and the ruling that a player reads their characters' abilities was otherwise met only after a reload |
| The panel's filter in `client/src/view/player.ts`'s `renderPlayerPanel` | `moveCommandFor` changed too | Phase 4a found a board click doing nothing while the panel said it would move |
| The guide's "narrative discipline" notes go with Task 5 | their wire half went with Task 4 | Task 4's review found it false from that commit; the use half went with Task 5 |
| D9's guide work names `rulesets/dnd45e-minimal/guide.md` | `rulesets/tavern-brawl/guide.md` names `abilityIds` too | An agent reading only that guide would add actors that can use nothing |
| Rows withdrawn: VTT-438, VTT-115, VTT-117 | VTT-470 and VTT-661 too, succeeded by VTT-707 and VTT-708 | Their field lists omit the list `sameIncarnation` now compares |
| `offered`'s test in `client/test/player.test.ts` | in `client/test/panel-offers.test.ts` | beside the panel tests that use it |
| Task 7's "True when": `bun test` from `client/` | the gate's form, `bun test client/test contract contract-spike` from the root | From `client/`, bun also loads the Playwright specs, which fail there on `main` too |
| SPEC-021 is not under the ticket's "Specifications this moves" | the specification task changes it | It cites VTT-470 and VTT-661 |

## What could not be established

- **VTT-632's poisoned branch.** A player of a poisoned campaign is served no
  ability with status 200; no test reaches a poisoned campaign through the
  gateway, so that branch is observed by none. It stands under the ruling on
  near-theoretical edges, as the plan's sort rejected it.
- **A failed re-read.** A player's re-read that fails keeps the window's list
  and is not tried again until the set, the flag or the role changes; a 401
  there forgets no token, as the load chain's does. Near-theoretical; no code.
- **Campaigns already played.** An actor added before this ticket names no
  ability and is offered none; the DM's way is to remove it and add it again
  naming its abilities. A companion so added again out of a log's sight drops
  to its unseen form in that log until next seen, as for any changed sheet.
- **The client's mutation report.** It measured `eb0fa35` and is out of date
  from Task 1. A full run is the owner's to ask for; the reading review of
  Task 7 expects it to report these equivalents: `app.ts`'s starting ability
  list mutated to a list holding a string, which neither `offered` nor the
  namer can match by id, and `moveCommandFor`'s `actor &&`, a token's actor
  always existing.
- **Docs left incomplete, not false:** `adventure.Load`'s and
  `buildActor`'s doc comments do not name the list; each is a block over
  SPEC-010's bound, which a touch would have to rewrite. No ticket carries
  them; the next edit of either block does.

## What was deliberately left out

On the owner's word of 2026-11-13: giving or taking an ability on a character
already in a campaign (the next ticket); the console's choice of abilities
when adding an actor; the sheet view; and the log naming its ruleset and
version (step 1's item 3). Where the console's choice and the sheet view go
is not ruled. The proposal put to the owner: the console's choice rides the
give-or-take ticket, since the console's form makes no usable sheet without
it, and the sheet view is a client ticket of its own, which the owner
places. Under OD-5, a Story names by its id an ability its window cannot
read.
