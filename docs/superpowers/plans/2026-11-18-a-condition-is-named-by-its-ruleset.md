# Plan: a condition is named in the ruleset's words wherever a window shows it

- **Ticket:** `docs/superpowers/specs/2026-11-18-a-condition-is-named-by-its-ruleset-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-18 by an agent that did not write the ticket,
    read-only against `b424e4c` on `feat/a-condition-is-named-by-its-ruleset`
    in the worktree `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and `task build:client` were not run. The base
    suites of M1 ran in the worktree, its status the same before and after;
    M1's whole suite, M2 to M4 and M7 ran in copies of the tree at
    `b424e4c` outside the repository; the rest are searches and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-18-a-condition-is-named-by-its-ruleset.md`.
- **Base:** `b424e4c`. **Branch:** `feat/a-condition-is-named-by-its-ruleset`.
- **Owner's rulings taken in:**
  - 2026-11-18: every place the ticket names names a condition: a disc's
    dot, the Story's and the Events panel's lines, and the DM console's
    remove button, whose actor is named as the console's ability rows name
    an actor. A dot's hover shows the condition's name and, after a line
    break, its description, as an ability button's hover shows an
    ability's.
  - 2026-11-17: a condition named by its id on a dot and in the Story is
    the ticket after the ability's description (G7 of the sheet's plan,
    `docs/superpowers/plans/2026-11-17-the-player-panel-is-the-sheet.md`).
  - 2026-09-04: nobody uses the product; no migration.
  - Standing: no counts or measured figures in anything written down; push
    and merge on the owner's word.
- **Signed off:** 2026-11-18, OD-1 A (an empty name reads as the id) and
  OD-2 A (a new e2e spec). The ticket was amended after verification,
  closing G1 (R3's sending of the ids is in its rules) and G3 (how an
  undeclared condition id reaches a window).
- **Ids:** R1 VTT-862, R2 VTT-863, R3 VTT-864, R4 VTT-865, allocated by
  `requirement-id` at Task 0.
- **The sort's refusals:** R4 is not merged into VTT-845, which stands for
  the panel while R4 holds the dots, the lines and the console; R5 (the
  "Remove condition" group's buttons, one per condition held) is today's
  behaviour, held by `dm-view.test.ts` and by no row, and this ticket does
  not change it.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists:
   `client/src/view/naming.ts`, `client/src/view/grid.ts`,
   `client/src/view/spectator.ts`, `client/src/view/dm.ts`,
   `client/src/app.ts`, `client/test/grid.test.ts`,
   `client/test/spectator-view.test.ts`, `client/test/dm-view.test.ts`,
   `client/test/app.test.ts`, `cmd/vtt/webdist`, `docs/requirements.md`,
   `docs/blueprint.md` and SPEC-022. Every symbol is where the ticket says:
   `Namer` and `namer` (naming actors, tokens, abilities and resources, and
   no condition); `ConditionDot` (the id alone) and `tokensOnScene`;
   `describe`, whose `conditionApplied` and `conditionRemoved` arms write
   `gained` and `lost` and the payload's `conditionId`; `renderDisc`'s
   `dot.title = c.id`; `ViewExtras`; `DMDeps` and the "Remove condition"
   group, labelling each button by the actor's id, `: `, the condition's id
   and ` ✕`; `layout.conditions` in `startSession`; VTT-842; blueprint
   principle 13's "shows an identifier where a name exists". In
   `~/dev/RPTool`: `AbstractTokenOverlay`,
   `rule-framework/schema/condition.schema.ts` (`id` and `label`),
   `rule-framework/library/mtscript/public/lib/RE_applyCondition.mts` and
   `rule-framework/library/mtscript/public/removeCondition.mts`, each
   printing the label, else the id. No spec under `client/e2e` names a
   condition (search).
2. **"Done" is an observation. By command for items 1 to 4; item 5 is the
   gate.** The ticket names no test, so none could be run red by name.
   Probe tests in a copy of the tree at `b424e4c` showed each item false
   today: a dot titled by the id where the window's list declares a name,
   a `conditionApplied` line reading `Lera gained dazed`, a remove button
   reading `a1: dazed ✕`, and a window drawing ids whatever its newest
   answer gave (M2). Under a probe of the change each passed (M3).
3. **Each rule is breakable. By reading.** Rule 1 fails when a dot's title
   is the id where the list gives a name, lacks the description, puts it
   first, or joins otherwise; rule 2 when a line shows the id where the
   list gives a name; rule 3 when the button shows an id where the fold or
   the list holds a name; rule 4 when an older answer's name is drawn after
   the newest's. Item 3's "still sends `remove_condition` with the ids" was
   a rule the list did not carry (G1, closed by the ticket's amendment);
   rule 1's "each when non-empty" reached a case the loader never serves
   (G2, answered by OD-1 A and the amended rule 1). R1 to R4 hold them.
4. **The scope matches the claim. By command, then reading.** `Namer` is
   built by `namer` alone, and the one test that writes a `Namer` literal,
   `client/test/qa-resource-change.test.ts`, casts it, so the client's
   typecheck passed with required members added (M3). `namer` is
   called from `renderSpectator`, `renderPlayerPanel` and
   `renderDMConsole`, and from tests, none of which hands it more than its
   present parameters. `ConditionDot` is built in `tokensOnScene` alone
   and read in `renderDisc` alone; no test builds one or a `TokenDisc`.
   `tokensOnScene` is called by `renderGrid` and by `grid.test.ts`,
   `visibility.test.ts` and `qa-property-names.test.ts`, its signature
   unchanged. `describe` is called in the client by the Story's `tell` and
   the Events panel's `renderTicker`, both handed the one namer
   `renderSpectator` builds. `ViewExtras` and `DMDeps` are built by `paint`
   in `startSession` and by tests. The ticket's files hold the change; the
   tests and records it reaches are wider than it names (G4).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-022's "A condition, a note's key and an adventure's id are shown
     as the entry names them" is overturned for a condition. The ticket
     names SPEC-022 and its problem says the lines show the id; not silent.
   - SPEC-022's "How the window names what it shows" lists what
     `renderSpectator` and `renderDMConsole` build their namers from; both
     lists grow.
   - VTT-734 and the owner's OD-5 of 2026-11-14 name an ability in the
     Story by every answer since the page loaded (`everRead`), because a
     role's answer holds only the abilities its set names (VTT-691 to
     VTT-693). A condition is not filtered: every role is answered every
     condition (VTT-710), and a server's ruleset is set once, by
     `WithRuleset` from `serve_compose.go` before it serves. So the newest
     answer holds every condition an earlier one held, and item 4's
     newest answer is coherent for the Story's past lines, as the
     properties that name a resource there already are (D4).
   - VTT-845 (the panel's names from the newest answer) is the form item 4
     extends; VTT-842 (the panel's own naming) is untouched.
   - Blueprint 13 is served; 27 holds, the names reaching each place
     through `/api/ruleset` and nothing copying them into the log.
6. **The records the work moves are named. By command, then reading.** The
   section is present, its path resolves, and nothing is `New:`. By
   search over `docs/specifications/` for `gained`, `Remove condition`,
   `remove_condition`, `ConditionDot`, `namer` and `Namer`: SPEC-022 alone
   describes what a window shows of a condition; SPEC-013 holds
   `remove_condition`'s authorization, unchanged, since the button sends
   the same ids; SPEC-018 bounds a condition id, unchanged; SPEC-012
   describes the route's `conditions`, unchanged. The ticket's list is
   right.

**Split: not offered.** The change is one client component, and its tasks
below are its commits.

## The gaps this plan inherits

- **G1. A rule the list did not carry.** Item 3's "it still sends
  `remove_condition` with the ids" was no rule in the ticket's list. R3
  carries it. Closed: the amended ticket lists it.
- **G2. An empty name.** Answered by OD-1 A: an empty name reads as the
  id; the ticket's item 1 and rule 1 were amended to say so. As verified,
  rule 1 said the name and the description "each
  when non-empty". `loadConditions` refuses a condition whose `name` is
  empty, so the route never serves one; only a client fixture can. Read
  literally, a declared condition with an empty name and an empty
  description was titled by nothing; OD-1 asked, and A was signed off.
- **G3. A loose account.** As verified, the ticket said an undeclared
  condition id can reach a window through "an adventure or a DM's command
  of an older ruleset"; closed by its amendment. By search, outside
  `internal/eventgen` (which draws histories for property tests and the
  soak), `ConditionApplied` is written by
  `rules.Resolve` and by `perceive`'s projection alone: no adventure
  applies a condition and no DM command does (`remove_condition` is the
  only condition command). The way is a log written under another
  ruleset's conditions. The behaviour planned, shown by its id, does not
  depend on how. The writer corrected it before Task 0's commit.
- **G4. Tests and records the ticket does not name.**
  `client/test/naming.test.ts` (the namer's own tests);
  `tools/ts-mutation-equivalents.txt`, whose entry on the console's namer
  line is re-keyed (D6); a new spec under `client/e2e` (OD-2); and the
  register's rows. The ticket says "their tests", so this is a gap, not a
  return.
- **G5. One read in a DM's window.** A window whose role is not `player`
  keys its read as "", so it issues one `/api/ruleset` read for its life
  (`watchKey`; a promotion is bounded to player and spectator). Item 4's
  older answer arriving later is reachable only in a player's window; the
  console and the Events panel, drawn only for a DM and an agent, take the
  same `layout.conditions` (M4).
- **G6. The ticket's "What could not be established" travels.** A long
  description in a dot's title, and touch platforms that draw no title, as
  for an ability's button. The real wire is OD-2.

## Measurements this plan stands on

- **M1. The base is green.** In the worktree, `env -u CLAUDECODE bun test`
  over `naming.test.ts`, `grid.test.ts`, `spectator-view.test.ts`,
  `dm-view.test.ts`, `qa-story-mechanical.test.ts`,
  `qa-property-names.test.ts`, `qa-what-a-window-shows.test.ts`,
  `qa-companion-first-sight.test.ts`, `qa-chips-by-role.test.ts`,
  `qa-window-number.test.ts`, `command-surface.test.ts`,
  `qa-dm-abilities.test.ts`, `token-card.test.ts`, `visibility.test.ts` and
  `app.test.ts` passed; in a copy at `b424e4c`, `bun test client/test`
  passed whole.
- **M2. Each item is false today.** In a copy at `b424e4c`, probe tests
  failed: `renderSpectator` handed a list declaring `dazed` ("Dazed",
  "Reeling") and `bare` ("Bare", no description), over an actor holding
  `dazed`, `bare` and an undeclared `odd`, titled its dots by the ids;
  `describe` with such a namer wrote `Lera gained dazed`;
  `renderDMConsole` labelled the buttons by the actor's id and the
  condition's id; a player's booted window whose newest answer named
  `dazed` "New" titled its dot `dazed`; a DM's labelled its button
  `a1: dazed ✕`.
- **M3. The change turns red the console's test alone.** In a copy at `b424e4c`
  with a probe of D1 to D7 (a last, optional parameter to `namer`,
  `Namer.condition` and `Namer.conditionTitle` required members,
  `ConditionDot.label`, `describe`'s arms, the dot's title,
  `ViewExtras.conditions`, `DMDeps.conditions` with one console namer and
  the actor's fold name, and `paint` handing `layout.conditions` to both),
  `bunx tsc --noEmit -p client/tsconfig.json` passed and
  `env -u CLAUDECODE bun test client/test` turned red
  `client/test/dm-view.test.ts`'s "a condition on an actor gets a removal
  button that sends removeCondition", and no other test. M2's probes
  passed, the newest answer's name holding on the dot and in the Story with
  an older answer arriving after it.
- **M4. A DM's window reads once.** In the same probe, a DM's booted
  window, sent a presence frame naming the DM (which re-reads `/api/me`
  and asks `rereadIfStale`), had issued one `/api/ruleset` read, and drew
  its button, its Events panel row, its Story line and its dot by that
  answer.
- **M5. The tests that read a condition's id today, and why each stays
  green.** By search over `client/test` and `client/e2e` for `gained`,
  ` lost `, `.dot`, `remove-condition`, `Remove condition` and non-empty
  `conditions:` lists. Every one runs with no list or an empty one, so the
  id stands:
  - Dots: `spectator-view.test.ts`'s "every condition shows as a dot
    carrying its name"; `app.test.ts`'s "a window whose role is
    read as %s draws a disc's resource chips: %p, and its condition dots
    always"; `qa-chips-by-role.test.ts`'s "${role} window draws the dots in
    the order applied", "applied order is not alphabetical order", "one
    condition draws one dot", "no condition draws no dot" and "dots appear
    for dm true, false and unset, in order applied";
    `qa-companion-first-sight.test.ts`'s "%s: the fold holds the companion
    whole after the run".
  - Lines: `spectator-view.test.ts`'s "describe renders a real label for
    every event kind, not the fallback"; `app.test.ts`'s "every role's
    Story draws what happens in the game and no mechanical entry, and the
    DM's and the agent's Events panel draws each";
    `qa-property-names.test.ts`'s "the Story line names the actor by token
    name, party name, or not"; `qa-what-a-window-shows.test.ts`'s "qaWS:
    describe words each case of the table as a player's window names it";
    `qa-companion-first-sight.test.ts`'s "a narration between the removal
    and the add: the run is told", "%s: the Story draws neither the run nor
    the token's arrival", "%s: the Events panel still shows every entry of
    the run", "%s: a session's window does not tell the run at a first
    sight" and "%s: entries arriving one at a time end with the run
    untold"; `qa-story-mechanical.test.ts`'s "${role.name}: a narration
    anchored to ${from}..${to}, all left out (… event ids), is drawn in a
    beat of its own at its own place", "${role.name}: a bare condition
    behind a run with a correction's sheet frame between is drawn",
    "${role.name}: a grant with an event id ends the run, and the bare
    condition behind it is drawn, the grant is not" and "${role.name}: an
    ability's use, a roll and a whisper read as today".
  - The console: `dm-view.test.ts`'s "a condition on an actor gets a
    removal button that sends removeCondition", which names its actor "A"
    and so changes (below). `command-surface.test.ts` and
    `qa-dm-abilities.test.ts` find the group by `data-action` and its
    heading, which do not change.
  - `qa-window-number.test.ts` declares `dazed` ("Dazed", "Reeling") and
    stayed green under the probe: it reads digits, not names.
- **M6. How each window builds its namer.** `renderSpectator` from the
  fold, the log, `ViewExtras.dm`, `ViewExtras.abilities` (every ability
  `everRead` holds) and `ViewExtras.properties` (the newest answer's);
  `renderPlayerPanel` from the fold, no log, the role, the newest answer's
  abilities and properties, for targets and costs, and its conditions are
  named by `sheetFor` from `layout.conditions` (VTT-842); `renderDMConsole`
  from the fold, no log, `true`, `DMDeps.abilities` and
  `DMDeps.properties`, keeping only `resource`. `readRuleset` sets
  `layout` only for the newest read's answer, so `layout.conditions` is
  the list item 4 asks for, at hand in `paint` for both `ViewExtras` and
  `DMDeps`.
- **M7. The comment ceilings.** `tools/comment-ceilings.txt` holds
  `client/src/view/naming.ts` and `client/src/view/grid.ts` where no added
  prose comment line fits, before or after the probe's added code lines;
  `client/src/view/spectator.ts`, `client/src/view/dm.ts` and
  `client/src/app.ts` just under their ceilings, admitting a prose line
  only beside added code. A line holding only requirement ids is set aside
  by `check:comments` and counts in no share. No file falls a full point
  under its ceiling in the probe.
- **M8. The mutation adjudications.** `tools/ts-mutation-equivalents.txt`
  holds a `BooleanLiteral` entry on the console's line
  `const named = namer(d.st, [], true, d.abilities, d.properties).resource;`,
  whose reason rests on `resource` reading `properties` alone. No entry
  sits on `describe`, `renderDisc`, `tokensOnScene`'s conditions line,
  `namer`'s body or `paint`'s extras. The TypeScript static check runs in
  `task check` only over a current stored report (SPEC-020, SPEC-023), so
  a stale entry is not caught until the next full run.
- **M9. The real wire can be driven.** `client/e2e/as-dm.ts`'s `asDM`
  sends a DM's commands on a raw socket; the e2e server serves
  `rulesets/dnd45e-minimal`; a DM may `use_ability` as any actor
  (SPEC-013's table); `staggering-blow` applies `dazed` on a hit, rolling
  `1d20 + @caster.str + 3` against `@target.fort`, and `rules.Resolve`
  hits when the total is at least the target's, so a target whose `fort`
  is at most the least total is hit on any roll. Each spec file gets a
  table of its own (`client/e2e/global-setup.ts`). `add_actor` refuses a resource some ability of the ruleset spends while no ability the actor's sheet names spends it (VTT-775), and `flurry_uses` is the only resource `dnd45e-minimal` spends, so
  the spec's actors hold no `flurry_uses`.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened: a test that read an
  id takes the new words exactly, never a looser match.
- **P2. Rule 3.** The contract and `/api/ruleset` do not change:
  `git diff --stat main -- contract internal` prints nothing.
- **P3. Rules 8 and 10.** Names, never line numbers; a comment under
  `client/src` is a warning or a pointer. No prose comment line is added
  to `naming.ts` or `grid.ts` (M7); a pointer there is a line of
  requirement ids alone.
- **P4. Rule 9,** answered below.
- **P5. Blueprint 13, 27 and 35.** Every window shows a condition by the
  ruleset's words where it has them; nothing copies them into the log; a
  condition's description reaches the board's hover.
- **P6. SPEC-022** as it stands for everything this does not change: the
  dots in the order applied (VTT-835), the Story's kinds (VTT-822), the
  panel's naming (VTT-842), the newest answer (VTT-845), the tree rebuilt
  at every paint.
- **P7. SPEC-020 and SPEC-023.** An adjudication names the code it
  excuses; the client's stored mutation report goes out of date with
  Task 1's commit, and `task check` names that and passes; a full run is
  the owner's to ask for.
- **P8. The mechanical gates.** A commit that changes `client/src` carries
  `cmd/vtt/webdist` rebuilt by `task build:client` (`check:drift`); bun
  runs under `env -u CLAUDECODE`; `task client:typecheck`;
  `tools/check-new-prose.py` and `python3 tools/check-comments.py` before
  each commit; stage and commit in separate calls; a client test reads no
  file outside `contract/gen` and `contract/testdata`. Each task gets
  Phase 4a's QA and one reviewer before its commit.

## Rule 9: how MapTool does this

The ticket's paragraph holds against `~/dev/RPTool`:
- In MapTool's core a token state's only words are its name:
  `AbstractTokenOverlay` holds a `name`, an order, a group, `mouseover` and
  who sees it, and no description. The token's State menu
  (`TokenPopupMenu.createStateMenu`, `ChangeStateAction`) lists each state
  by that name.
- The owner's framework gives a condition an `id` and a `label`
  (`ConditionDefSchema`: `id`, `label`, `icon`, `mechanicalEffects`), and no
  description. `RE_applyCondition.mts` prints "Condition: <label> applied",
  the label else the id; `removeCondition.mts` lists the token's
  conditions by label, else id, beside the duration, and reports
  "<token name>: <label> removed".
- **Borrowed:** the ruleset's name, else the id, wherever a condition is
  shown, the removal control included; the actor named by its name beside
  it, as `removeCondition.mts` names the token.
- **Not borrowed:** the duration beside the label (this platform's
  conditions carry none); an icon (out of the ticket).
- **Our own:** the description on the dot's hover has no MapTool
  precedent; it is the owner's ruling, as an ability's button shows its
  description.

## Decisions

1. **D1. The namer names a condition.** `namer` gains a last, optional
   parameter, the window's condition list (`{ id, name, description }`).
   `Namer` gains `condition(id)`, the name the list gives the id, else the
   id, by the same lookup `ability` uses, and `conditionTitle(id)`,
   `condition(id)` followed by a line break and the description the list
   gives the id when it is non-empty (OD-1 A).
   - **Forced by** `tokensOnScene` and `describe` being handed the namer
     and nothing else to name by, and every place the ruling names
     naming by one lookup.
2. **D2. A dot carries its hover.** `ConditionDot` gains `label`, which
   `tokensOnScene` sets from `conditionTitle`; `renderDisc` titles the dot
   by it, and the line's trailing comment goes; `id` stays.
   - **Forced by** `ResourceChip.label`, the chip's hover that
     `tokensOnScene` sets (VTT-815): the same seam; and `grid.test.ts`
     reading `id`.
3. **D3. The lines.** `describe`'s `conditionApplied` and
   `conditionRemoved` arms name the condition by `names.condition`; their
   words, `gained` and `lost`, stay. The Story's `tell` and the Events
   panel's `renderTicker` both read `describe` and change with it.
   - **Forced by** item 2, and VTT-822's tests reading `gained prone`
     with no list (M5).
4. **D4. The board and the lines take the newest answer.** `ViewExtras`
   gains `conditions`; `renderSpectator` hands it to `namer`; `paint` sets
   it to `layout.conditions`, for every role.
   - **Forced by** item 4, and by check 5: every role is answered every
     condition and a server's ruleset is set once, so the newest answer
     names every past line any answer could; `everRead` exists for
     abilities a role's set filters, which conditions are not.
5. **D5. The console.** `DMDeps` gains an optional `conditions`, since
   test files build `DMDeps` without it; `renderDMConsole`
   builds one namer, from the fold, no log, `true`, the abilities, the
   properties and the conditions, before the "Remove condition" group, and
   the Abilities group names a resource through it as today. Each button
   reads the actor's name in the fold, else its id, as an Abilities row
   heads an actor, then `: `, `names.condition` of the id and ` ✕`; its
   `data-action` and its `removeCondition(actor, id)` do not change. An
   actor the fold does not hold is named by its id.
   - **Forced by** the ruling ("as the console's ability rows name an
     actor", which reads the fold's `name`, not the namer's GM chain) and
     M8: one namer keeps one adjudicated line.
6. **D6. The adjudication is re-keyed in the console's task.** The
   `BooleanLiteral` entry on the console's namer line takes the new line
   as its `code:`, its offset checked against it, and its reason says
   `condition` and `conditionTitle`, like `resource`, read their list
   alone, so `dm` still reaches only token and actor naming; its closing
   sentence names `condition` beside `resource`.
   - **Forced by** SPEC-020, and M8: no gate asks before the next full
     run.
7. **D7. The window hands both.** `paint` passes `layout.conditions` as
   `ViewExtras.conditions` and `DMDeps.conditions`; no new state.
   - **Forced by** M6: `layout` already holds the newest answer's list.
8. **D8. The panel keeps its own naming.** `sheetFor`'s
   `d?.name || c.ID` (VTT-842) is left as it is.
   - **A preference,** named as one: the ticket does not touch
     `client/src/player.ts`, and VTT-842's tests stand. The lookup then
     stands in the namer and in `sheetFor`, as an ability's stands in the
     namer, the console's chips and the panel's buttons.
9. **D9. A description's own line breaks stand in the dot's title as
   written.**
   - **Forced by** VTT-842's panel, which titles a condition by its
     description as written, and SPEC-022's same rule for an ability's.
10. **D10. Pointers only where a ceiling binds.** A new member of `Namer`
    or `ConditionDot` gets no prose comment; a line of requirement ids may
    stand above it. `ViewExtras.conditions` and `DMDeps.conditions` take a
    one-line doc only if the file's added code keeps it under its ceiling
    (M7).
    - **Forced by** M7 and rule 10.
11. **D11. Every client commit carries its bundle.** Tasks 1 to 3 each run
    `task build:client` after their last `client/src` change and commit
    `cmd/vtt/webdist` with it, though Tasks 1 and 2 change nothing the
    running window draws.
    - **Forced by** `check:drift`, which pre-push runs.
12. **D12. The real wire** (OD-2 A): a new spec,
    `client/e2e/conditions.spec.ts`, its own table. Through `asDM` the DM
    starts a session, loads `goblin-ambush`, adds an attacker and a target
    with every number property `dnd45e-minimal` declares and `hp`, and no
    `flurry_uses`, since `add_actor` refuses a resource some ability of the ruleset spends while no ability the actor's sheet names spends it (VTT-775), and `flurry_uses` is the only resource `dnd45e-minimal` spends, the attacker's sheet naming
    `staggering-blow`, the target's `fort` at most the attack's least
    total, its `max_hp` equal to its `hp`'s max, and its `hp` above half of
    that after the blow's greatest damage, so `bloodied` does not follow, places their tokens on adjacent floor squares of the scene,
    the target's with no GM name of its own, so the namer's GM chain and
    the fold name it alike, and uses `staggering-blow`. The
    DM's page then shows: the target's dot titled the `name`, a line break
    and the `description` read from
    `rulesets/dnd45e-minimal/conditions/dazed.json`; a Story line and an
    Events row "<target> gained <name>"; a button "<target's name>: <name>
    ✕", whose click sends `removeCondition` with the target's id and
    `dazed` and is followed by a "lost" line and no dot. `afterAll` ends
    the session.
    - **Forced by** principle 33 as it stands: `conditionJSON`, the Go
      route test's struct and `ConditionDef` describe the route's
      conditions apart, and no e2e has read one, the panel's VTT-842 names
      included; and M9.
13. **D13. The register.** R1 to R4 are added; every row the work reaches
    stands (below), its evidence cells following the tests that change.
14. **D14. The specification last,** after the report, from the report:
    SPEC-022.

## Open decisions for the owner

- **OD-1. A declared condition whose name is empty** (G2; the loader never
  serves one).
  - **A (recommended).** The name reads as none and the id stands in its
    place, as `sheetFor` names it on the panel (VTT-842) and as the namer
    names an ability and a resource: the dot is titled by the id, then the
    description when it has one. One lookup serves every place.
  - **B.** The ticket's words read literally: the dot is titled by the
    description alone, and by nothing when it has neither; the lines and
    the console still need a word and take the id.
- **OD-2. A check on the real wire** (the ticket's second point).
  - **A (recommended).** D12: a new e2e spec, its own table, the DM's
    commands sent through `asDM`, a hit made certain by the target's
    `fort`, read in the DM's window.
  - **B.** No e2e check. The unit tests hold every place. The route's keys
    are pinned on the Go side by `TestQAReadEveryRoleReadsEveryCondition`,
    which reads `name` and `description` against the ruleset, and on the
    client's by `ConditionDef` and its hand-written fixtures; a key renamed
    in the client alone passes every client test.
  - **C.** A check in `client/e2e/table.spec.ts` on the adventure's own
    actors: a hit there depends on a roll, so it would read a condition
    only sometimes.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill, under the
signed-off answers; the register's words are the ones that bind. Every new
row reads `**OPEN — no test yet**` until its task cites its tests. The
table is the verifier's draft, under the answers the owner then signed
off; the register's rows bind, and the note below the table gives their
words as allocated.

| R | Item | Rule (as drafted; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | Every browser window titles each condition dot of a token disc by the name the condition list of its newest `/api/ruleset` answer gives the condition, else by its id, followed, when that list gives it a non-empty description, by a line break and the description as written; a condition the list does not declare, and any condition while the window holds no list, is titled by its id alone | the id where a name is declared, an empty name drawn empty, the description missing, first, joined otherwise or rewritten, a line break with nothing after it, or another condition's words |
| R2 | 2 | The Story's and the Events panel's line of a `conditionApplied` reads the actor as the window names it, ` gained ` and the condition, and of a `conditionRemoved` ` lost ` in its place, the condition named by the name its newest `/api/ruleset` answer's condition list gives it, else by its id | the id where a name is declared, or one panel naming it and the other not |
| R3 | 3 | Each button of the DM console's "Remove condition" group reads the actor's name in the fold, else its id, `: `, the condition named as R2 names it, and ` ✕`, and sends `remove_condition` naming the actor's id and the condition's id | an id where the fold or the list holds a name, the namer's GM chain for the actor, or a name sent in place of an id |
| R4 | 4 | A window names a condition on its discs' dots, in its Story and Events panel and on its DM console's buttons by the condition list of the answer to its newest `/api/ruleset` read, and an answer to an older read arriving after it changes none of them | an older answer's name drawn after the newest's |

As allocated at Task 0, after the reading review: R1 to R3 name a
condition by "the name the condition list the window holds gives it,
else, when the list gives it no name or an empty one, by its id" (OD-1
A), and R4 alone says which list a window holds. R4's older-answer clause
can be broken only in a player's window, since a DM's or an agent's window
reads once (G5); for the console and the Events panel it holds by that.

**Candidates the sort may refuse** (settled at Task 0: both refused, as
the header says):
- **R4 merged into VTT-845.** VTT-845 says the same of the panel's sheet
  and condition names; the sort may withdraw it and succeed it with one
  row for the panel, the dots, the lines and the console.
- **R5.** The "Remove condition" group draws one button per condition each
  actor holds and is absent when none holds one. Today's behaviour, held
  by `dm-view.test.ts`'s "with no conditions the removal group is absent
  entirely" and "the remove-condition group is absent when no actor has
  one", and by no row; the ticket does not change it.

**Rows checked and left standing,** their evidence cells updated where a
named test is rewritten or renamed:
- **VTT-835** stays true as worded: one dot per condition, in the order
  applied; R1 titles each. Its tests stay green (M5).
- **VTT-822** stays: the Story still shows each `conditionApplied` and
  `conditionRemoved` it showed; its tests read ids with no list (M5).
- **VTT-475**, **VTT-476** and **VTT-477** stand: the actor in a line is
  named as before, no line is a payload's name (a condition's name is the
  ruleset's), and an ability's line is untouched.
- **VTT-842** and **VTT-845** stand (D8; the merge was refused).
- **VTT-710** stands, and D4 rests on it. **VTT-734** stands: abilities
  in the Story still follow `everRead`.
- **VTT-834**, **VTT-730** and **VTT-144** stand: chips, the Abilities
  group and who may remove a condition do not change.

## The existing tests that change

By M3, the test below alone turns red, and it moves without weakening
(P1):
- **Task 2:** `client/test/dm-view.test.ts`'s "a condition on an actor
  gets a removal button that sends removeCondition" clicks the button
  labelled "A: dazed ✕", its fixture's actor being named "A" and holding
  `dazed` with no list, and asserts beside the command's case that it
  names actor `a1` and condition `dazed`.

Every test M5 names stays green unchanged and keeps its rows.

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R4 and
  the candidates as the answers leave them; allocate ids with
  `requirement-id`; commit the ticket, this plan and
  `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: the namer, the board and the lines (R1, R2; D1 to D4, D9, D10; items 1 and 2)

- **Files:** `client/src/view/naming.ts` (`Namer.condition`,
  `Namer.conditionTitle`, `namer`'s parameter);
  `client/src/view/grid.ts` (`ConditionDot.label`, `tokensOnScene`);
  `client/src/view/spectator.ts` (`describe`'s arms, `renderDisc`'s dot,
  `ViewExtras.conditions`, `renderSpectator`'s namer);
  `client/test/naming.test.ts`, `client/test/grid.test.ts` and
  `client/test/spectator-view.test.ts`, new tests written first and seen
  red: a list declaring several conditions, the one asked for not first,
  one with a description, one with none, one with an empty name and one
  whose description holds a line break; `condition` naming a declared id by its name, an
  undeclared one and any id with no list by the id; `conditionTitle`
  giving the name, a line break and the description, the name alone for
  an empty description, the id alone undeclared or with no list, a
  description's line break as written; `tokensOnScene`'s dots carrying
  `label` in the order applied, beside `id`; `renderSpectator` handed the
  list titling each dot so, and its Story and, with `ticker`, its Events
  panel writing "<actor> gained <name>" and "<actor> lost <name>", an
  undeclared condition by its id; `docs/requirements.md`; a rebuilt
  `cmd/vtt/webdist`.
- **True when:** the new tests pass and failed before the change;
  `env -u CLAUDECODE bun test client/test` and `task client:typecheck`
  pass; every test M5 names passes unchanged; `task build:client` has run
  after the last change to `client/src`; `task check:requirements-chain`,
  `tools/check-new-prose.py` and `python3 tools/check-comments.py` pass.
- **Deliberate breaks:**
  - Title the dot by `c.id` again: R1's tests turn red.
  - Put the description first, or join with a space: R1's tests turn red.
  - Drop the empty-description filter: the name-alone test turns red.
  - Read the name with `??` in place of `||`: the empty-name test turns
    red (OD-1 A; the test: a declared condition with an empty name and a
    description titles its dot by the id, a line break and the
    description, and a line names it by its id).
  - Match any list entry for the description: the not-first test turns
    red.
  - Write `p.value.conditionId` in either arm: R2's tests turn red.
  - Rewrite a description's line break as a space: D9's test turns red.
- **The state here:** a board and lines handed a condition list name each
  condition; `paint` hands none yet, so the running window is as today.

### Task 2: the console (R3; D5, D6; item 3)

- **Files:** `client/src/view/dm.ts` (`DMDeps.conditions`, one namer
  before "Remove condition", the button's label);
  `client/test/dm-view.test.ts`, new tests written first and seen red: a
  list declaring the condition and an actor named in the fold, whose token
  carries a different GM name, labelled by the actor's name and the
  condition's; an actor with an empty name and an actor the hand-built
  state does not hold, each by its id; an undeclared condition and no
  list, by the id; a click sending `removeCondition` with the ids; and the
  test "The existing tests that change" names;
  `tools/ts-mutation-equivalents.txt` (D6); `docs/requirements.md`; a
  rebuilt `cmd/vtt/webdist`.
- **True when:** the new tests pass and failed before the change; the
  checks of Task 1's last line pass; `qa-dm-abilities.test.ts`,
  `dm-abilities.test.ts` and `command-surface.test.ts` pass unchanged; the
  re-keyed entry's `code:` line stands in `dm.ts` exactly once and its
  offset opens `true`.
- **Deliberate breaks:**
  - Name the actor by `names.actor`: the GM-name test turns red.
  - Drop the id fallback for the actor: the empty-name test turns red.
  - Read the actor's name without guarding an actor the state lacks: the
    absent-actor test turns red.
  - Send the condition's name: the click test turns red.
  - Name the condition by its id: R3's tests turn red.
- **The state here:** a console handed a condition list names each
  button; `paint` hands none yet.

### Task 3: the window and the real wire (R4; D7, D12; items 4 and 5)

- **Files:** `client/src/app.ts` (`paint` hands `layout.conditions` to
  `ViewExtras` and `DMDeps`); `client/test/app.test.ts`, new tests
  written first and seen red, as "an older ruleset read answering after a
  newer one sets neither the sheet's pages nor its condition names" is
  built: a player's window whose newest answer names the condition titles
  the dot and writes the Story's line by it, an older answer arriving after
  it changing neither; a DM's window labels its button, its Events row,
  its Story line and its dot by its answer; `client/e2e/conditions.spec.ts`
  (D12, under OD-2 A); `docs/requirements.md`; a rebuilt
  `cmd/vtt/webdist`.
- **True when:** the new tests pass and failed before the change; the
  checks of Task 1's last line pass; VTT-845's and VTT-753's tests pass
  unchanged; the e2e spec reads the condition's words from the ruleset's
  file, writing none in the spec.
- **Deliberate breaks:**
  - Hand `rs.conditions` from every answer as it arrives: R4's
    older-answer test turns red.
  - Hand the console none: the DM's button test turns red.
  - Hand the board none: the player's dot test turns red.
  - Read the condition's name under another key on the client: the e2e
    spec turns red (D12, OD-2's reason).
- **The state here:** every window names a condition in the ruleset's
  words.

### Task 4: the gate (item 5)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger`
  once, last, if a ceiling can come down; check free space and the load
  (`os.getloadavg()`); run `task check` whole on the last code commit,
  detached under `/bin/sh`, output to a file, requiring its completion
  line; run `task e2e`. Tell the owner the client's stored mutation report
  is out of date from Task 1 and which commit it measured (SPEC-023), and
  that D6's entry is judged at the next full run; a full Stryker run is
  his to ask for.
- **True when:** `task check` and `task e2e` are green; the embedded bundle
  is the last client commit's.

### Task 5: the report, then the specification

- **The report:** `docs/reports/2026-11-18-a-condition-is-named-by-its-ruleset.md`
  with the `implementation-report` skill and its reading review: each
  "Done looks like" item with its observation; the rows and what turned
  each red; Rule 9; G1 to G6 and each OD's answer; Phase 4a's
  adjudications under their own heading; the gate.
- **SPEC-022,** with the `specification` skill, from the report; its
  "## Requirements" list gains VTT-862 to VTT-865:
  - "A condition, a note's key and an adventure's id are shown as the
    entry names them" loses the condition; a condition's line reads its
    name as R2 gives it.
  - "How the window names what it shows": `renderSpectator`'s namer is
    built from `ViewExtras.conditions` too, `paint` setting it to the
    newest answer's list, and why the Story's past lines can take it;
    `Namer.condition` and `Namer.conditionTitle`; a dot's `label` and its
    title as R1 gives it, a description's line breaks as written.
  - The console: the "Remove condition" group's buttons as R3 gives them,
    and the namer `renderDMConsole` builds from `DMDeps.conditions` too.
  - The Status names the new tests and the e2e spec.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specification; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specification as drafted, and the
surfaces as signatures and names, never the source: `Namer` and `namer`,
`ConditionDot` and `tokensOnScene`, `describe`, `ViewExtras` and
`renderSpectator`, `DMDeps` and `renderDMConsole`, the `ConditionDef`
type, and `boot` from `client/src/app.ts` with its fetch and socket seams
as `app.test.ts` uses them. Tell it: never run `task check`, `task e2e` or
Stryker; its files are new `qa-*` test files; run client tests with
`env -u CLAUDECODE bun test`; a client test reads no file outside
`contract/gen` and `contract/testdata`; fixtures with more than one
condition and the one asked for not first; the tree is hashed before and
after it. Known and not findings: the panel names a condition by its own
lookup (D8); the agent's tools answer condition ids; a long title, and
touch platforms that draw none (G6); the console's other ids, the
"Describe tokens" rows among them.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "no other") names the search that bounds it; check that
`git diff --stat main -- contract internal` is empty; check each comment
under `client/src` is a warning or a pointer, and none of prose was added
to `naming.ts` or `grid.ts`; check every assertion this work changed for
being weakened rather than moved (rule 2); check D6's entry against the
line it names and its reason against what the namer now reads; and read
the console's actor naming against the Abilities rows' heading.
