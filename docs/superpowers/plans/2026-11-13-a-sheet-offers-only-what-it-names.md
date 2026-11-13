# Plan: a character is offered only the abilities its sheet names

- **Ticket:** `docs/superpowers/specs/2026-11-13-a-sheet-offers-only-what-it-names-design.md`
  (ADR-012, Build order step 1, item 4; blueprint principles 26 and 27).
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-13 by an agent that did not write the ticket,
    read-only against `5547d2b` on `feat/a-sheet-offers-only-what-it-names`
    in the worktree `vtt-platform-visibility`, with the ticket added with
    intent-to-add and uncommitted.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and gremlins were not run. Probe P1 ran through
    `go test -overlay`, its test file kept outside the tree.
- **To be committed as:** `docs/superpowers/plans/2026-11-13-a-sheet-offers-only-what-it-names.md`.
- **Base:** `5547d2b`. **Branch:** `feat/a-sheet-offers-only-what-it-names`.
- **Owner's rulings taken in:**
  - 2026-11-13: a ruleset holds every ability; a sheet names the abilities
    its character has; a sheet naming an ability its ruleset does not hold
    is refused at `add_actor` with a ruleset served, at adventure load, and
    by `vtt serve --ruleset`, as SPEC-025's keys are; with no ruleset,
    nothing is read against one. The use of an ability the caster's sheet
    does not name is refused for every role, kept on the server because the
    agent uses `use_ability` with no panel. The panel offers a character
    only what its sheet names. `/api/ruleset` and `get_ruleset` answer a
    player only the abilities named by the sheets of the characters they
    control; the DM and the agent read all; conditions and properties stay
    as today. Item 4 of step 1 goes before item 3. Out of scope: the
    console Add form's ability picker, the sheet view, and a command to
    give or take an ability on an existing character (the next ticket).
  - 2026-11-08: the DM owns the fit; no automatic defaults of any kind.
  - 2026-11-05: a character has only the powers and skills it has itself,
    and those alone show in its window.
  - 2026-11-02: plain words are for the board engine's refusals a player
    meets from the panel; `rules.Resolve`'s refusals keep their form until
    the rules-engine investigation (D6).
  - 2026-10-21: a near-theoretical edge with a small consequence gets no
    code, no test, no debt entry (D8, OD-4).
  - 2026-10-09: a field outside SPEC-018's first table is unbounded on
    purpose (OD-4).
  - 2026-09-23: a fact has one home (D1).
  - 2026-09-04: nobody uses the product; no migration. Principle 5 still
    binds every log already written: it opens, and its actors name no
    ability.
  - Standing: no counts or measured figures in anything written down.
- **Signed off:** by the owner, 2026-11-13: every recommendation of OD-1
  to OD-7.
- **Ids:** VTT-683 to VTT-703, allocated at Task 0 (under
  "Requirements"); the `R` labels are this plan's.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol it names is where it says: `Actor` in
   `contract/vtt/v1/events.proto`; `decodeStrict`, `actorJSON` and
   `loadActors`; `Resolve` and `Compiled`; `handleUseAbility`;
   `authorizeActorOwnership` and `playerRules`; `renderPlayerPanel`;
   `handleRuleset`; `getRulesetDescription` and its words "as every
   participant can"; `SheetFault` and `Ruleset.SheetKeyFault`;
   `validateAddActorWith`; `sheetsFit`; `shownWith`; `abilityLine`; the
   `goblin-scimitar` ability of `rulesets/dnd45e-minimal`; VTT-465 and
   VTT-658; every specification under "Specifications this moves"; and
   MapTool's `Token.macroPropertiesMap`, `ImpersonatePanel` and
   `MacroButtonProperties.command` (P7).
2. **"Done" is an observation. By command for items 1, 3 and 5, by reading
   for 4, 6 and 7.** Each item names what will be true and the tests that
   show it; no named test exists yet, so none could be run to fail.
   - Item 1's "today" half holds by probe: an actor file naming an
     `abilities` list is refused as an unknown field (P1).
   - Items 3's and 5's hold by existing tests run on `5547d2b` (P2): an
     actor whose sheet names nothing resolves `fists`, and a player's token
     reads the route that lists every ability.
   - Item 4's holds by reading: `renderPlayerPanel` loops over every
     ability it is handed. Item 6's: `app.ts` fetches `/api/ruleset` once,
     in its start-up chain.
   - Item 7 states no "today" half and has none: the list does not exist
     on `5547d2b`, and `shownWith` builds an actor from an allow-list, so
     the first commit that adds the field already holds item 7 with no
     code. Its test is a guard, red only under its deliberate break (G3).
3. **Each rule is breakable. By reading.** Each rule line maps to a row R1
   to R19 below, each with what turns it red.
4. **The scope matches the claim. By command, then reading.** The callers
   (P4): `SheetKeyFault` has the callers SPEC-025 names; `shownWith` and
   `sameIncarnation` are called from `recall`, `Resume`, `shownAs` and
   `unseenAs`; `handleRuleset` from `Handler`; `handleUseAbility` from
   `handleCommand`; `composeServer` from `newServeCmd` and
   `bootSelfContained`; `renderPlayerPanel` from `app.ts` alone;
   `buildActor` from `Compile`; `copyActor` from the client fold's
   `actorAdded` arm. The work reaches files the ticket does not name: G2.
   The ticket's search for the files that send a use misses the scenarios
   and the Playwright suite (P3).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-012's principle-35 line ("every participant, whatever the role,
     is served ... a summary of its abilities") is overturned for a player
     and a spectator, by the owner's ruling, and the ticket names SPEC-012.
   - VTT-632 ("answers 200 to a participant of every role") stays true; its
     test, `TestQARRRulesetRouteAnswersEveryRole`, compares the whole
     ability list for every role and narrows (Task 6).
   - SPEC-013's sight gate answers an unseen target "whatever else the
     command carries" (VTT-472); D5 keeps the new refusal after it.
   - SPEC-016 defines `sameIncarnation` over the fields only `ActorAdded`
     writes; the new list is one, and D4 adds it there.
   - SPEC-018's Consequences put a new event field into one of its tables
     in the same change, and its sentence that a ruleset's display names
     reach every participant through `/api/ruleset` becomes false for
     abilities: G1.
   - `rulesets/dnd45e-minimal/guide.md` says the wire has no `abilities`
     field and nothing binds an ability to an actor; it is a document, not a
     decision, and becomes false (principle 34): G2.
   - Refusing a use that `5547d2b` accepts tightens a command, not a log:
     every log written before opens, and its actors name no ability
     ("Campaigns already played"; principle 5 binds logs).
6. **The records the work moves are named. By command, then reading.**
   SPEC-025, SPEC-012, SPEC-013, SPEC-016, SPEC-022 and SPEC-007 resolve.
   SPEC-018 moves under every outcome of OD-4 and is not listed: G1.

**Split: not recommended (OD-7).** The tasks are ordered so the branch
passes through the state a split would merge, and that state changes
nothing at the table.

## The gaps this plan inherits

- **G1. SPEC-018 moves and is not under "Specifications this moves".** Its
  Consequences say a field added to an event without a bound goes into its
  second table, with what holds it, in the same change; and its sentence
  that a ruleset's display names "reach every participant through
  `/api/ruleset`" stops being true for an ability's. The ticket names
  SPEC-018 only under "What could not be established". Task 9 moves it.
- **G2. Files the work reaches that the ticket does not name.**
  - `client/src/state.ts` (`Actor`) and `client/src/fold.ts` (`copyActor`,
    `foldToDumpJSON`): the panel reads the sheet from the client's fold,
    and the fold is held byte-equal to `vtt state dump`. `Actor`'s fields
    are required by the rule its `kind` comment states, so every client
    test that builds a state `Actor` literal gains the field
    (`client:typecheck` finds them).
  - `client/src/player.ts`: its module header in `client/src/view/player.ts`
    says every rule the panel applies lives there as a tested pure
    function, and that header's list of rules changes.
  - `internal/perceive/projection.go`'s `sameIncarnation`, beyond reading
    `shownWith` (D4).
  - `rulesets/dnd45e-minimal/guide.md`: its reference statblocks are "the
    EXACT `actor` argument to pass to `add_actor`", it says there is no
    `abilities` field, and its "Ability lists (narrative discipline)" notes
    say `use_ability` accepts any ability from any actor.
  - `contract/testdata/expected_tools.json`, which
    `TestToolsMatchGolden` compares toolgen's output with, and
    `contract/roundtrip_test.go`.
  - `adventures/*/goldens/compiled-batch.json`; `scenarios/goblin-fight.json`,
    `toy-brawl.json`, `door-watch.json`; `scenarios/goldens/adventure-night`,
    `toy-brawl` and `door-watch`; `client/e2e/table.spec.ts` (P3).
  - `client/src/commands.ts`'s paste and `internal/mcp`'s
    `commandFromArgs` carry the field with no change (D2), and each gets a
    test that says so.
- **G3. Item 7 cannot fail on `5547d2b`.** It holds by construction from
  the commit that adds the field. Its tests are guards; the deliberate
  breaks in Task 3 are what show them able to fail.
- **G4. The alpha campaign.** `~/dev/vtt-alpha`'s adventure `two-rooms`
  (dnd45e-minimal), outside the repository, holds actor files that name no
  ability (P8). After this ticket every use in a campaign it seeded is
  refused, as the ticket's "Campaigns already played" says, until its
  actors are added again naming abilities or the next ticket's command
  gives them some; and its actor files need the field before it is loaded
  into a fresh campaign. The owner decides when.

## Measurements this plan stands on

- **P1.** A test placed in `internal/adventure` by `go test -overlay`, its
  file in the session's scratchpad: it copies `adventures/goblin-ambush`
  with `copyFixtureDirExcluding`, adds `"abilities": ["goblin-scimitar"]`
  to the copy's `act-cutter.json` after its `kind`, and calls
  `adventure.Load` against `rulesets/dnd45e-minimal`. On `5547d2b`: `json:
  unknown field "abilities"`; the unedited adventure loads.
- **P2.** On `5547d2b`, `go test -run` passes
  `TestUseAbilityHitProducesBatchFirstSequence` (the brawler, whose sheet
  names nothing, resolves `fists`),
  `TestUseAbilityResolveValidationErrorIsCleanOkFalse` and
  `TestTheRulesetRouteAnswersEveryPropertyInTheRulesetsWords` (a player's
  token reads `/api/ruleset`).
- **P3.** The ticket's search, `git grep -l -E
  'UseAbility\{|use_ability|useAbility\('`, prints nothing under
  `scenarios/` or `client/e2e/`; `git grep -l -E
  '"useAbility"|use-ability|UseAbility\{|use_ability|useAbility\(' --
  scenarios client/e2e` prints `client/e2e/table.spec.ts` and
  `scenarios/adventure-night.json`, `door-watch.json`, `goblin-fight.json`
  and `toy-brawl.json`. Of those scenarios, `adventure-night`, `door-watch`
  and `toy-brawl` have goldens; `goblin-fight` does not (its README).
- **P4.** The callers in check 4, by `git grep -n` over `'*.go'
  ':!*_test.go'` and over `client/src`. `git grep -n 'Resolve(' -- '*.go'
  ':!*_test.go'` prints `handleUseAbility` and the rules conformance
  suite's `smokeTest` and `runGolden` as `rules.Resolve`'s callers (and
  `internal/mapdef`'s own `Resolve`).
- **P5.** `fetchRulesetGuide` has no caller outside `client/test`, and the
  MCP tool `get_ruleset_guide` serves the guide from `--ruleset` on disk
  (`handleGetRulesetGuide`), not from the route; so no window and no agent
  tool reads `/api/ruleset/guide`.
- **P6.** `refuseValuelessCurrent`'s walk skips a field whose `Message()` is
  nil, so a repeated string on `vtt.v1.Actor` passes through it untouched;
  `parseActorJSON` decodes the paste with protobuf-es `fromJson`, which
  carries every field `ActorSchema` has.
- **P7.** MapTool at RPTool's `maptool` `f4b7fef6c`: `Token.macroPropertiesMap`
  holds a token's own `MacroButtonProperties`, each with its own `command`
  text; `ImpersonatePanel.init` offers "Impersonate Selected" only when
  `AppUtil.playerOwns` the token, and the panel resets on
  `TokenMacroChanged`, `TokenEdited`, `TokenPanelChanged` and
  `SelectionChanged`. `MacroButtonProperties.executeMacro` commits the
  macro through the clicking client's own `CommandPanel`.
- **P8.** `~/dev/vtt-alpha/adventures/two-rooms` declares `dnd45e-minimal`,
  and none of its `actors/*.json` names an ability (read with Python).
- **P9.** `rulesets/dnd45e-minimal/guide.md`'s "Reference statblocks" give
  each statblock an "Ability lists" note: the Goblin cutter
  `goblin-scimitar`; the Goblin archer `goblin-shortbow` and
  `goblin-scimitar`; the Human fighter `longsword-strike`, `crossbow-shot`,
  `hunters-flurry`, `staggering-blow` and `rally`. Neither
  `rulesets/tavern-brawl/guide.md` nor `adventures/cellar-rats/guide.md`
  gives a list.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no gate moves; `TestQARRRulesetRouteAnswersEveryRole`
  narrows to what the owner ruled, it is not loosened to pass); 3 (a
  field added to `Actor`, nothing renumbered or retyped; `check:breaking`
  reports, it does not enforce); 4 (`engine.Apply` is untouched: its
  `proto.Clone` carries the field); 5 (no game-system word in Go,
  `fists` and `goblin-scimitar` as fixture ids are allowed, `hp` is not);
  8 and 10 (names, not line numbers; a comment is a warning or a pointer,
  and a comment this change makes false is rewritten: the module header of
  `client/src/view/player.ts`, `SheetFault`'s doc and its `Field` comment,
  `AdventureActor`'s doc, `TestUseAbilityResolveValidationErrorIsCleanOkFalse`'s
  comment); 9 (answered below).
- **Principle 6.** The panel's filter is an aid at the input; the server's
  refusal is what stands between a participant and a use, and the server's
  read is what decides what a player is sent.
- **Principle 19.** When the gateway cannot tell what a player controls, it
  serves no ability (D7).
- **Principle 34.** Every tool description and the ruleset guide say what
  the server does at each commit that changes it (D9).
- **ADR-009.** A golden is derived by hand first: `state.json` and
  `compiled-batch.json` are edited by hand before `stream.json` is
  recorded (`scenarios/goldens/README.md`).
- **The project's mechanical gates**, as
  `docs/superpowers/plans/2026-11-05-a-ruleset-names-its-properties.md`
  lists them under its "Constraints that bind every task", and the Go
  mutation gate over `tools/check-mutation.py`'s `PACKAGES`, which hold
  `internal/rules`, `internal/adventure`, `internal/gateway`,
  `internal/perceive`, `internal/campaign` and `internal/mcp`; a client
  change committed with its `cmd/vtt/webdist`; client tests measured with
  `env -u CLAUDECODE`; stage and commit in separate calls.

## Rule 9: how MapTool does this

In MapTool at RPTool `maptool` `f4b7fef6c` (P7):
- A token carries its own macros, `Token.macroPropertiesMap`, and each macro
  is a full copy of its text (`MacroButtonProperties.command`). The
  impersonated token's panel, `ImpersonatePanel`, lists that token's macros
  alone, and is offered only for a token the player owns. The panel resets
  whenever the token's macros or the token change, or the selection does,
  from events the client already receives.
- A macro runs in the client that clicked it. Every client receives every
  token, its macros included, and what a player may see is a filter in the
  window.
- **Borrowed:** a character's own list on its own panel; and for item 6, a
  panel that refreshes on the change signal the client is already sent
  (here `WatchableChanged`), not on a reload.
- **Not borrowed:** the copy on the token, which principle 27 forbids (the
  sheet names the ruleset's entry by id); the distribution, which sends
  every client everything and filters in the window, the opposite of
  principle 6 and of the read this ticket narrows; and its absence of a
  server's judgment of a use, since MapTool has no server that judges one.

## Decisions

1. **D1. One home per question, in `internal/rules`.**
   - `Ruleset.SheetAbilityFault(ids []string) *SheetFault`, beside
     `SheetKeyFault`, answers the first id in byte order, from a sorted
     copy, that the ruleset's `Compiled` does not hold, as a `SheetFault`
     whose `Field` is the list's field name (`ability_ids` under OD-1's
     recommendation), whose `Key` is the id, and whose `Reason` is `names
     ability "<id>", which the ruleset does not hold`; nil when every id is
     held, and for an empty list.
   - `Offers(a *vttv1.Actor, abilityID string) bool` answers whether the
     actor's sheet names the ability. The gateway's use refusal (D5) and
     its read (D7) both ask it; the client's mirror (D10) is an aid.
     Step 1 item 5 makes what a sheet is offered reach through references,
     sets and conditions (ADR-012 §3); it changes this function and not its
     callers. A function rather than a method on `Ruleset` is a
     preference: today it reads no ruleset.
   - **Forced by** the 2026-09-23 ruling and SPEC-025, which keeps where a
     sheet is read against its ruleset in `internal/rules`;
     `.go-arch-lint.yml` already lets `gateway`, `adventure` and `cmd` use
     `rules`. `SheetKeyFault` and its tests are left as they are.
2. **D2. Where a sheet enters, it carries its list.**
   - `add_actor`: `ToEvent` copies the actor whole, so the field reaches the
     `ActorAdded` with no change; the console's paste and the MCP tool carry
     it too (P6). `validateAddActorWith` asks D1's check after
     `SheetKeyFault`, for an actor with an id and a ruleset served, and
     refuses with `gateway: add_actor: actor "<id>": <field>: <reason>`.
     With no ruleset, nothing is read; an actor with no id is left to the
     fold (VTT-154).
   - `adventure.Load`: `actorJSON` gains the list (`json:"ability_ids"`
     under OD-1), `loadActors` asks D1's check after `actorValues` and
     refuses with `fieldErr(path, <field>, reason)`; `AdventureActor` gains
     the list and `buildActor` copies it in its order. A list written
     `null` loads as none, as `"attributes": null` does.
   - The order inside an actor (kind, keys, values, then abilities) is a
     preference: it names the faults SPEC-025 already names first.
   - **Forced by** item 2 and SPEC-025's shapes for the same refusals.
3. **D3. The boot reads the lists.** `sheetsFit` names, per actor in byte
   order of ids, its first fault: `SheetKeyFault`'s, else
   `SheetAbilityFault`'s, as `actor "<id>": <field>: <reason>`, joined
   into the refusal SPEC-025 states, after which the identity store and the
   campaign are closed as today. With no ruleset, nothing is read.
   - **Forced by** item 2 and VTT-678's per-actor shape; naming the first
     fault per actor is SPEC-025's preference, kept.
4. **D4. The projection compares the list where it compares a sheet.**
   - `shownWith` is unchanged: its allow-list leaves the list out, so a
     creature and an unseen party member are held without it (item 7), and
     `shownAs` gives a party member in sight whole, the list included.
   - `sameIncarnation` compares the list (`slices.Equal`), because SPEC-016
     defines it over the fields only `ActorAdded` writes, and this ticket
     gives no event that changes the list. Without it, a party member
     introduced unseen whose sheet holds no attributes, resources or module
     data is never marked `reshaped`, so its eyes see it and it is never
     brought whole; and `Resume` accepts a held actor that lacks the world's
     list.
   - The next ticket, which gives a command that changes the list, moves it
     from `sameIncarnation` to `beliefOf`.
   - **Forced by** SPEC-016's definition and VTT-664 ("its sheet while it is
     a party member").
5. **D5. Where the use's refusal runs: OD-3.** Under the recommendation:
   in `handleUseAbility`, after its no-ruleset refusal and before `Resolve`;
   for an actor `st.Actors` holds whose sheet does not `Offers` the
   ability, `gateway: use_ability: actor "<id>" does not name ability
   "<ability>"`, ok=false, nothing appended. An actor the snapshot does not
   hold goes on to `Resolve`'s own `unknown actor`.
   - After the sight gate, which `handleCommand` runs before dispatch, so
     an unseen target is still answered as an unknown one whatever the
     ability (VTT-472, `TestQAASTheRefusalIsAnUnknownIdsWhateverElseTheCommandGetsWrong`).
   - Before `Resolve`'s `unknown ability`, so a player's answer for an id
     the ruleset does not hold and for one it holds that the sheet does not
     name is one string: an id oracle closed, as VTT-472 and VTT-506 close
     one for actors. A sheet names only ids the ruleset holds (D2, D3), so
     for every actor a campaign served with a ruleset holds, "not named"
     covers "unknown".
   - The ids in the text, not names: the panel never offers this use, so a
     player meets it only from an altered window, and the 2026-11-02
     narrowing leaves the rules engine's refusals in their form.
6. **D6. What the refusals say.** The words in D1, D2, D3 and D5, which
   tests compare. They name the field as OD-1 names it.
7. **D7. What `/api/ruleset` answers, by role.** `handleRuleset` keeps the
   participant `authed` returns and decides per request:
   - the DM and the agent: every ability, as today;
   - a player: the abilities of `Compiled` that `Offers` answers true for
     on some actor of `s.campaign.State()` that `controls` names them in,
     whatever its kind; none when they control none;
   - a spectator: none;
   - when `State()` is nil, which it is for a poisoned campaign, a player
     is served none, status 200.
   - Abilities stay sorted by id; `conditions`, `resources` and
     `properties` are every role's, as today.
   - **Forced by** the owner's ruling; principle 7 (decided at each
     request, so a grant or a revocation applies to the next read);
     principle 19 and VTT-632 (200 to every accepted token) for the nil
     state; `controls`, which `authorizeActorOwnership` also asks, so a
     player reads the abilities of exactly the actors they may use.
8. **D8. How a player's window learns to read again (item 6).**
   - `app.ts` keeps the key its newest ruleset read was issued under: the
     watchable set's actor ids in order and its no-party-member flag. In a
     window whose role is `player`, an `onChange` that finds
     `session.watchable` or `session.controlsNoPartyMember` changed from
     that key issues a new read; the start-up chain's read records its key
     too. Each read takes a ticket as `refreshSharing` does, and an answer
     whose ticket is no longer the newest is dropped. A DM's and an agent's
     window never reads again: they read every ability.
   - **Forced by** SPEC-015: the gateway sends `WatchableChanged` at each
     judgment that changes the set or its flag, after the append that
     changed it, so a read issued on it is answered from the state that
     holds the grant or the revocation; and by `refreshSharing`'s reason
     for a ticket: reads can be in flight at once.
   - A player's control of an actor that is not a party member is not in
     the set, so its grant or revocation issues no read. The panel shows
     only party members, so the consequence is a stale entry no panel
     offers; near-theoretical, no code (the 2026-10-21 ruling).
9. **D9. What the system says about itself (principle 34).**
   - `add_actor`'s new field doc, in `tools/toolgen/main.go`: "Optional.
     The abilities this actor has, each by its id in the served ruleset
     (get_ruleset lists them). use_ability refuses this actor any ability
     not named here, so an actor that names none can use none. With a
     ruleset served, an id the ruleset does not hold refuses the whole
     command." Task 2 writes it without its second sentence, which Task 5
     adds with the refusal it describes.
   - `use_ability`'s description: "Use an ability the acting actor's sheet
     names (its abilityIds) against explicit targets. A use of any other
     ability is refused, whatever your role."
   - `get_ruleset`'s description, replacing "as every participant can":
     "Read the table's ruleset as your role may: its id and name, its
     conditions, every property it declares (each with its key, its type,
     number or resource, the name the table calls it and what it means),
     and its abilities (range, targets, usage): every ability for the DM
     and the agent, and for a player only the abilities named by the sheets
     of the actors they control. A number property's value is under an
     actor's `attributes`, a resource property's under its `resources`, and
     the abilities an actor may use are the ids its sheet names
     (`abilityIds` in add_actor, `ability_ids` in get_state); use_ability
     refuses any other. With no ruleset loaded, id and name are empty and
     every list is []. Takes no arguments. For how to run this ruleset at
     the table, read get_ruleset_guide."
   - `rulesets/dnd45e-minimal/guide.md`: each reference statblock's JSON
     gains `abilityIds` with its "Ability lists" (P9); the wire-schema
     sentence names `abilityIds`; the sentence "There is NO `x`/`y` field
     and NO `abilities` field" keeps its `x`/`y` half; the "narrative
     discipline" notes, which say nothing binds an ability to an actor,
     go with the commit that makes the server refuse (Task 5), and "the
     caster OR the target of any ability" in "Conditions" reads "any
     ability its sheet names" from the same commit.
   - **Forced by** principle 34; each text is true from the commit that
     writes it.
10. **D10. The window offers what the sheet names.**
    - `client/src/player.ts` gains `offered(abilities, actor)`: the entries
      of the list `/api/ruleset` gave whose id the actor's `abilityIds`
      names, in the list's order. `renderPlayerPanel` lists, and arms from,
      that, for a player, the DM and the agent; with nothing offered it shows no
      "Abilities" heading, as with an empty list today (a preference).
    - The client fold carries the list: `copyActor` copies it,
      `client/src/state.ts`'s `Actor` gains `abilityIds: string[]`, and
      `foldToDumpJSON` writes `ability_ids` after `kind`, only when
      non-empty, as Go's `encoding/json` writes the generated field's
      `json:"ability_ids,omitempty"` in declared order.
    - **Forced by** the fold's byte parity with `vtt state dump`
      (`client/test/fold-dump.test.ts` and the corpus), and `Actor`'s rule
      that its fields are required.

## Open decisions for the owner

The owner approved every recommendation below on 2026-11-13.

- **OD-1. The contract field's name and shape. Recommended: A.**
  - **A: `repeated string ability_ids = 10;` on `vtt.v1.Actor`**, JSON
    `abilityIds`, and `ability_ids` in an adventure's actor file. A list of
    ids on the wire is spelled `<thing>_ids` (`controller_ids`,
    `target_ids`, `to_actor_ids`). ADR-012 §2 gives an entry's per-sheet
    state (its state, values per entry, copies) to a `set` property the
    ruleset declares; step 1 item 5 designs the catalogue and references,
    and decides whether this list becomes such a set. A wrapper now would
    be shaped before that design, and every literal, adventure file and
    agent argument would carry an object per id that nothing reads but the
    id.
  - **B: `repeated AbilityRef abilities = 10;`** with `message AbilityRef
    { string ability_id = 1; }`, so a reference can later gain fields
    additively (ADR-012 §3: "what the sheet holds about a reference ... is
    campaign data"). Cost: the object per id above, and item 5 may still
    choose a ruleset-declared set instead.
  - Under either, a proto field is never retyped, so item 5 adds beside it
    if it needs more.
- **OD-2. Which abilities each actor names. Recommended as listed.**

  | Actor | Where | Names | Why |
  |---|---|---|---|
  | `act-fighter` | `adventures/goblin-ambush`, `scenarios/goblin-fight.json` | `crossbow-shot`, `hunters-flurry`, `longsword-strike`, `rally`, `staggering-blow` | the ruleset guide's statblock list (P9); it holds the `flurry_uses` `hunters-flurry` spends |
  | `act-cutter` | the same | `goblin-scimitar` | the guide's list |
  | `act-archer` | the same | `goblin-scimitar`, `goblin-shortbow` | the guide's list |
  | `hollis`, `mara` | `adventures/cellar-rats` | `chair-swing`, `fists`, `sober-up`, `splash-of-water` | no guide gives a list; every tavern-brawl ability is a patron's move, and each holds the `stamina` `chair-swing` spends |
  | `act-brawler`, `act-patron` | `scenarios/toy-brawl.json` | as `hollis` | one rule for every tavern-brawl actor |
  | `act-watcher`, `act-hidden`, `act-latecomer` | `scenarios/door-watch.json` | as `hollis` | the same |

  Every use in the scenarios is then one its caster names, and the
  Playwright suite's fighter names `longsword-strike` through the
  adventure.
- **OD-3. Where the use's refusal runs. Recommended: A.**
  - **A: in `handleUseAbility`, before `Resolve`, through `rules.Offers`**
    (D5). `Resolve`'s callers are `handleUseAbility` and the rules
    conformance suite (P4); the suite judges a ruleset, not a campaign, so
    every use that reaches a campaign passes the check. Principle 4 holds:
    one gateway, one snapshot, one command path. The fixtures that change
    are the gateway's and the scenarios', which change under B too.
  - **B: in `rules.Resolve`**, before its `unknown ability`. It also
    changes every `Resolve` test in `internal/rules` (`resolve_test.go`,
    `resolve_v2_test.go`, `resolve_int32_test.go`,
    `resolve_dup_target_test.go`, `qa_ability_outcomes_test.go`,
    `qa_ruleset_bound_test.go`), the conformance suite's generated caster
    (`buildFixtureState`) and golden decoder (`goldenFile`'s actors, under
    `DisallowUnknownFields`), and every golden under `rulesets/*/goldens`.
    It buys a check that the conformance suite also runs, on data that
    names what the suite is told to use.
  - Under either, the refusal runs after the sight gate (D5). If step 1
    item 5 makes a use name the reference it comes through (ADR-012 §3, a
    copy as the action's own `this`), the check moves into `Resolve` then,
    from one call site.
- **OD-4. A name twice on one sheet, and a fold bound. Recommended: accept;
  no bound.**
  - A repeated id is accepted, with no code and no test: the use check, the
    read and the panel each ask whether the list holds an id, so a repeat
    changes nothing at the table (the 2026-10-21 ruling). Refusing it would
    also refuse, at boot, a campaign served with no ruleset that holds one.
  - No fold bound: no role but the DM and the agent writes the list (an
    `add_actor` or a file someone installs), so neither of SPEC-018's tests
    is met, and the owner ruled on 2026-10-09 that a field outside its
    first table is unbounded on purpose. The list goes into SPEC-018's
    second table: "with a ruleset served, `add_actor` and `adventure.Load`
    refuse an id the ruleset does not hold, and `internal/rules/load.go`
    bounds an ability id at `maxIDBytes`; with none, the WebSocket frame
    alone, for `add_actor`, a command of the DM and the agent".
- **OD-5. What a Story calls an ability its window cannot read.
  Recommended: A.**
  - **A: the id stands**, as `namer`'s fallback already draws it and
    VTT-477's row says ("else its id"). A player's and a spectator's Story
    then draws a creature's or a companion's ability by its id, such as
    `a creature used goblin-scimitar on Human Fighter: hit, rolled 17`. No
    code. Nothing is hidden by it either way: `AbilityUsed.outcome_summary`
    carries the display name to every log the event reaches (SPEC-016,
    VTT-488), and VTT-477 keeps the window from reading it.
  - **B: `/api/ruleset` answers every role every ability's id and display
    name**, and the rest of an entry only as D7 says. This needs the
    ruling amended: a player would read every ability's name.
  - **C: `AbilityUsed` gains the ability's display name**, written by
    `Resolve`, so the log names what the ruleset called it when used. A
    contract addition, and every `AbilityUsed` in the rules goldens and the
    scenario streams changes. A candidate for step 1 item 3, which designs
    what an outcome records of its ruleset.
- **OD-6. `/api/ruleset/guide` for a player and a spectator. Recommended:
  close it.** The guide is the DM's and the agent's material, and
  `rulesets/dnd45e-minimal/guide.md` names every creature's abilities and
  how they are used; left open, an altered window reads in prose what D7
  withholds, which is principle 6's case. No window and no agent tool reads
  the route (P5), so closing it changes nothing anyone is shown. Cost: a
  role map beside `adventureGuideRoles` (DM and agent; 403 `gateway: not
  authorized` for the others), a test, and SPEC-012's sentences on the
  route and principle 35. Left open, SPEC-012 says why.
- **OD-7. Split the ticket. Recommended: one ticket, tasks in order.**
  - The cut, if wanted, is after Task 4. **Part A** (Tasks 1 to 4): a
    sheet names its abilities and names only ones its ruleset holds,
    refused at each entry and at boot; the fold, the projection and the
    client fold carry the list; the data names its abilities. **Part B**
    (Tasks 5 to 7): the use refusal, the read by role, the panel and its
    re-read.
  - **Between the parts** the product holds and checks the lists and
    reads them nowhere: any actor still uses any ability, every role still
    reads every ability, the panel still offers every ability, and the
    ruleset guide's "narrative discipline" notes are still true. Nothing
    changes at the table.
  - **Why not split:** no Part B task can be reviewed without Part A's
    field, and Part A gives the owner nothing to see; a split costs a
    second `task check`, report and specification round. The task order
    gives the same intermediate state inside the branch.

## Requirements

Sorted and allocated at Task 0, 2026-11-13, after the owner's sign-off. The
plan's tasks keep the `R` labels; the register carries the ids. Every row
is new work and reads `**OPEN — no test yet**` until its task cites its
tests.

| R | Id | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|---|
| R1 | VTT-683 | 1 | An `add_actor` the server accepts naming abilities appends an actor naming the same ids in the same order, from the console's paste, the MCP tool or a WebSocket command | the list dropped or reordered |
| R2 | VTT-684 | 1 | `adventure.Load` reads an actor file's `ability_ids`, and `Compile`'s `ActorAdded` names the same, in the same order | refused as unknown, or dropped |
| R3 | VTT-685 | 2 | With a ruleset served, `add_actor` refuses, after its controller and kind refusals, an actor whose keys fit and whose list holds an id the ruleset does not, naming the actor and the first such id in byte order, appending nothing | the actor appended |
| R4 | VTT-686 | 2 | With no ruleset served, `add_actor` refuses no actor for its list | refused |
| R5 | VTT-687 | 2 | `adventure.Load` refuses, after its refusals of an actor's id, name, kind, keys and values, an actor file whose list holds an id the ruleset does not, naming the file, the field and the id | loads |
| R6 | VTT-688 | 2 | `vtt serve` with a ruleset refuses a campaign holding such an actor, naming each, and for one whose keys all fit its first such id in byte order | served |
| R7 | VTT-689 | 2 | `vtt serve` with no ruleset refuses no campaign for its actors' lists | refused |
| R8 | VTT-690 | 3 | A `use_ability` that authorization and sight let through, as an actor whose sheet does not name the ability, is refused for every role, naming the actor and the ability, appending nothing | resolves, or appends |
| R10 | VTT-691 | 5 | `/api/ruleset` answers a player the abilities named by the sheets of the actors they control, and no other, while the campaign is not poisoned | another ability listed, or a named one missing |
| R11 | VTT-692 | 5 | `/api/ruleset` answers a spectator no ability | any listed |
| R12 | VTT-693 | 5 | `/api/ruleset` answers the DM and the agent every ability the ruleset holds | one missing |
| R13 | VTT-694 | 5 | `get_ruleset`'s description says the DM and the agent read every ability, and a player only those named by the sheets of the actors they control | missing, or "as every participant can" |
| R14 | VTT-695 | 4 | The panel offers, for the actor it acts as, one button per ability of the window's list that the actor's sheet names, and none for another, in a player's, a DM's and an agent's window | an unnamed ability offered, or a named one missing |
| R15 | VTT-696 | 6 | A player's window reads `/api/ruleset` again when a `WatchableChanged` changes its set or its flag | the start-up list kept |
| R15b | VTT-697 | 6 | A player's window draws from the answer to its newest read, and drops an older answer arriving after it | an older answer drawn |
| R16 | VTT-698 | 7 | A character log is introduced to a creature, and to a party member its eyes do not see, with no `ability_ids` | a frame carries the list |
| R17 | VTT-699 | 7 | A character log holds a party member its eyes see with its sheet's `ability_ids`, one first introduced unseen with no other value included | the list missing once seen |
| R18a | VTT-700 | D9 | `add_actor`'s schema documents `abilityIds` as the actor's abilities by id, and that an id the served ruleset does not hold refuses the command | the sentence missing |
| R18b | VTT-701 | D9 | `use_ability`'s description says a use of an ability the actor's sheet does not name is refused, whatever the role | the sentence missing |
| R18c | VTT-703 | D9 | `add_actor`'s schema says an actor may use only the abilities its `abilityIds` names, so one naming none can use none | the sentence missing |
| R19 | VTT-702 | OD-6 | `/api/ruleset/guide` is answered to the DM and the agent, and refused to a player and a spectator with 403, carrying no guide | a player or a spectator served |

**What the sort reshaped:**
- R9 (a use the sheet names is judged by `Resolve` as before) got no id:
  VTT-137, VTT-482 and VTT-487 hold it, since their tests resolve a use
  through the gateway, for the DM, the agent and a player, with actors
  that name their ability from Task 5 on. A check that refuses a named use turns
  them red.
- R6 lost "leaves the campaign and the identity store closed": every
  `sheetsFit` refusal takes the one closing path in `composeServer` that
  VTT-678's refusal takes, and the ability fault joins that refusal
  without adding a path.
- R10 lost "in id order": VTT-118 holds the order of the abilities a read
  route lists, whatever the role; Task 6's player test asserts it and
  cites VTT-118 too.
- R13 lost "that `use_ability` refuses an ability the actor's sheet does
  not name": that sentence's home is `use_ability`'s own description,
  R18b. D9's text for `get_ruleset` keeps a pointer to it.
- R15 is split into R15 and R15b, and R18 into R18a, which Task 2 makes
  true, and R18b and R18c, which Task 5 does.

**Rows a task withdraws or allocates**, each in the task that changes what
it describes, so that every row is true at every commit:
- VTT-438 ("one button per ability the ruleset lists"), in Task 7,
  succeeded by VTT-695.
- VTT-117 ("the adventure list, the ruleset guide, the maps list and the
  art are answered to every role"), in Task 6, succeeded by VTT-702 and by
  a row allocated there for the adventure list, the maps list and the art,
  which the tests holding that half cite instead.
- VTT-115 ("a guide that does not exist is answered 404"), in Task 6,
  succeeded by a row allocated there for the DM and the agent: a player
  asking for either guide is answered 403 first.
- `GET /api/ruleset` answering `conditions` to every role: no row says
  the route answers them; `TestQARRRulesetRouteAnswersEveryRole` checks
  them for every role (`qaRRCheckConditions`). Task 6, which narrows that
  test, allocates a row for them, and the test cites it.

**Rejected at the sort**, each with what already holds it or why it is not
a rule:
- A repeated id refused, and a fold bound on the list: OD-4.
- A Story drawing an ability it cannot read by its id: VTT-477's row
  already says it (OD-5, A).
- An unseen target answered first whatever the ability: VTT-472 holds it,
  and `TestQAASTheRefusalIsAnUnknownIdsWhateverElseTheCommandGetsWrong`
  turns red if the new refusal moves before the sight gate.
- A use with no ruleset served refused first: VTT-132 holds it.
- Resources and properties answered to every role: VTT-631 and VTT-619
  hold them, worded for every caller, unchanged.
- A player of a poisoned campaign served none: D7's nil case,
  near-theoretical; kept in code, no row, and VTT-691 is worded for a
  campaign that is not poisoned.
- A player's control of a non-party actor issuing a read: D8,
  near-theoretical.
- The guide's statblocks naming their abilities: data the reading review
  checks against the adventures, not a rule.

VTT-465 and VTT-658 keep their rows; R16 adds the list beside them rather
than rewording them. VTT-632 keeps its row; its test narrows (Task 6).

## The existing tests and data that change

By search; each task reruns its own packages first and names what turned
red.
- **Every actor that resolves a use through a real gateway names its
  ability** (Task 5): in `internal/gateway`, `ruleset_test.go`
  (`newRulesetFixture`, and the actor `a` that
  `TestAThresholdRefusalReachesTheIssuerWithoutItsExpression` seeds itself,
  naming `guard-stance`), `qa_ability_sight_test.go`,
  `server_visibility_test.go`, `qa_issuer_test.go` and
  `qa_ruleset_bound_test.go`; `cmd/vtt/mcp_ruleset_e2e_test.go`'s
  `add_actor` arguments. `internal/gateway/authz_test.go` calls `Authorize`
  alone, and `internal/harness`'s use tests run against fakes: neither
  changes. The search is P3's wider pattern over `'*.go'`, and the bound is
  `go test ./internal/gateway/ ./cmd/vtt/` going green.
- **`TestQARRMetadataRoutesAnswerEveryRole`** reads
  `/api/ruleset/guide` among the routes every role is answered, and drops
  it (Task 6, OD-6); `TestMetadataRulesetGuideServedForEveryRole` becomes
  VTT-702's test.
- **`TestQARRRulesetRouteAnswersEveryRole`** compares the whole ability
  list for the DM and the agent and D7's list for a player and a spectator
  (Task 6). `TestMetadataRulesetAbilitiesAreSortedById` and
  `TestMetadataRulesetShapeMatchesTheContract` read as the DM and stay.
  `cmd/vtt/qa_get_ruleset_test.go` reads with the agent's token and stays.
- **Every client test that builds a state `Actor` literal** gains
  `abilityIds` (Task 1; `client:typecheck` names each). Every client test
  that renders the panel's abilities names them on its actor (Task 7):
  `client/test/spectator-view.test.ts`, `qa-ability-target.test.ts`,
  `token-card.test.ts`, `player.test.ts`, `app.test.ts`,
  `qa-property-names.test.ts`, `qa-what-a-window-shows.test.ts`,
  `qa-panel-say.test.ts`, as
  `bun test` finds them.
- **The data** (Task 4): `adventures/*/actors/*.json`,
  `adventures/*/goldens/compiled-batch.json`, the scenarios and goldens in
  P3, `rulesets/dnd45e-minimal/guide.md`'s statblocks.
- **Unchanged:** `rules.Resolve` and its tests, every golden under
  `rulesets/*/goldens` (OD-3, A); `SheetKeyFault` and its tests;
  `engine.Apply`.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** the owner answers OD-1 to OD-7 (done: every recommendation,
  2026-11-13); run the `requirements` sort over R1 to R19; allocate ids
  (VTT-683 to VTT-703); commit the ticket, this plan and
  `docs/requirements.md`.
- **True when:** `git status` is clean and the requirements chain passes.

### Task 1: the field (R1's wire half; D10's fold half)

- **Files:** `contract/vtt/v1/events.proto` (`Actor`, field 10, its
  comment a pointer to SPEC-025); what `task generate:contract` writes
  under `contract/gen` and to `cmd/vtt/tools.json`;
  `contract/testdata/expected_tools.json`; `contract/roundtrip_test.go`
  (an `ActorAdded` with the list round-trips, and the field is not proto3
  `optional`); `client/src/state.ts` (`Actor`), `client/src/fold.ts`
  (`copyActor`, `foldToDumpJSON`); `client/test/fold-dump.test.ts`; every
  client test literal of a state `Actor`.
- **True when:** the round trip passes; the client's dump writes
  `ability_ids` after `kind` when the list is non-empty and leaves it out
  when empty; `git diff main -- contract/vtt` adds the field and changes
  nothing else; `task check:breaking` reports nothing breaking;
  `client:typecheck` passes.
- **Deliberate breaks:** drop the list from `copyActor`, then from
  `foldToDumpJSON`; each turns the new `fold-dump.test.ts` case red.

### Task 2: the sheet's list against its ruleset (R1 to R7, R18a; D1, D2, D3, D6, D9)

- **Files:** `internal/rules/sheet.go` (`SheetAbilityFault`; `SheetFault`'s
  doc) and `internal/rules/sheet_test.go`; `internal/adventure/load.go`
  (`actorJSON`, `loadActors`), `compile.go` (`buildActor`),
  `AdventureActor`'s doc; `internal/gateway/add_actor_validate.go`;
  `cmd/vtt/serve_compose.go` (`sheetsFit`); `tools/toolgen/main.go`
  (`add_actor`'s `abilityIds` doc) and `tools/toolgen/main_test.go`; the
  regenerated `tools.json` copies and `expected_tools.json`; new tests in
  `internal/adventure`, `internal/gateway` (through the WebSocket, as
  `add_actor_fit_internal_test.go` does), `internal/mcp` (the tool carries
  the list), `client/test/commands.test.ts` (the paste carries it) and
  `cmd/vtt` (beside `campaign_fit_test.go`).
- **True when:** each refusal of D2 and D3 holds in D6's words, naming what
  it says; with no ruleset, `add_actor` and `vtt serve` accept any id; an
  adventure's and an `add_actor`'s list reach the fold in their order;
  a refused boot leaves the campaign openable again (SPEC-025's test
  shape); `TestConformanceOverAdventuresGlob` passes.
- **Deliberate breaks:** make `SheetAbilityFault` answer nil (the
  adventure, gateway and boot tests turn red); drop its call from
  `validateAddActorWith`, then from `sheetsFit`, then from `loadActors`
  (each package's test turns red); drop the copy in `buildActor` (the
  compile test turns red); answer a list in its given order rather than
  sorted (`sheet_test.go`'s first-in-byte-order case turns red).

### Task 3: the projection (R16, R17; D4)

- **Files:** `internal/perceive/projection.go` (`sameIncarnation`); new
  tests in `internal/perceive`: a creature is introduced without the list;
  a party member its eyes do not see is introduced without it; a party
  member in sight is introduced with it; a party member introduced unseen
  whose sheet holds the list and no value is brought whole, the list
  included, at the first event at which its eyes see it; `Resume` starts
  afresh from a fold whose held actor lacks the world's list.
- **True when:** each new test passes, and every existing `internal/perceive`
  and `internal/campaign` test passes unchanged.
- **Deliberate breaks:** add the list to `shownWith`'s literal (the
  creature and unseen tests turn red); drop the comparison from
  `sameIncarnation` (the brought-whole and `Resume` tests turn red).

### Task 4: the data (item 8's Part A half; OD-2)

- **Files:** `adventures/goblin-ambush/actors/*.json` and
  `adventures/cellar-rats/actors/*.json`; their
  `goldens/compiled-batch.json`, edited by hand first; the `add_actor`
  steps of `scenarios/goblin-fight.json`, `toy-brawl.json` and
  `door-watch.json`; `scenarios/goldens/adventure-night`, `toy-brawl` and
  `door-watch`: `state.json` edited by hand first; then `stream.json`, in
  which each `actorAdded` actor gains `abilityIds` and nothing else
  changes, so its recorded rolls stay the testimony `state.json` was
  derived from (the README's "Deriving a dice scenario's golden");
  `TestScenarioGoldenStreamsHaveNotDrifted` then compares it with a fresh
  capture, dice masked on both sides. Its failure message prints a masked
  capture, which is never committed;
  `rulesets/dnd45e-minimal/guide.md`'s statblocks and wire-schema sentence
  (D9's Part A half).
- **True when:** `TestConformanceOverAdventuresGlob`,
  `internal/harness.TestFoldGoldenCorpus`,
  `cmd/vtt.TestScenarioGoldenStreamsHaveNotDrifted` and
  `client/test/fold-parity.test.ts` pass; `vtt serve --ruleset rulesets/dnd45e-minimal
  --adventures-dir adventures` and the tavern-brawl equivalent boot; each
  statblock's `abilityIds` equals its adventure actor's `ability_ids`.
- **Deliberate break:** take one id out of a hand-derived `state.json`;
  the fold gate turns red.
- **The state here is OD-7's "between the parts".**

### Task 5: the use (R8, R18b, R18c; D5, D9; item 3)

- **Files:** `internal/rules` (`Offers`) and its test;
  `internal/gateway/ruleset.go` (`handleUseAbility`);
  `tools/toolgen/main.go` (`use_ability`'s description, and the second
  sentence of `add_actor`'s `abilityIds` doc, D9),
  `tools/toolgen/main_test.go`, the regenerated `tools.json` copies and
  `expected_tools.json`; `rulesets/dnd45e-minimal/guide.md` ("narrative
  discipline" notes and "any ability" in "Conditions"); the fixtures under
  "The existing tests and data that change"; new tests in
  `internal/gateway`: the DM, the agent and a player each refused naming
  the actor and the ability, nothing appended, the log's head unchanged; a
  named use resolves; an unknown actor still gets `Resolve`'s refusal; and
  through the MCP tool in `cmd/vtt/mcp_ruleset_e2e_test.go`, a use the
  sheet does not name returns a tool error naming the actor and the ability.
- **True when:** the new tests pass; every VTT-472 test passes unchanged;
  the tests holding VTT-137, VTT-482 and VTT-487 pass with their actors
  naming the ability (R9);
  `TestUseAbilityResolveValidationErrorIsCleanOkFalse` passes (the refusal
  names `no-such-ability`), its comment, which says `Resolve` rejected the
  command, rewritten to say the sheet's check does (rule 10), and
  VTT-132's `Resolve` clause held by
  `TestAThresholdRefusalReachesTheIssuerWithoutItsExpression`.
- **Deliberate breaks:** remove the check (the new tests turn red); move it
  into `handleCommand` before the sight gate
  (`TestQAASTheRefusalIsAnUnknownIdsWhateverElseTheCommandGetsWrong` turns
  red); make `Offers` answer true (the refusal tests turn red).

### Task 6: the read (R10 to R13, R19; D7, D9, OD-6; item 5)

- **Files:** `internal/gateway/metadata.go` (`handleRuleset`; under OD-6, a
  role map for `handleRulesetGuide`); `internal/mcp/ruleset_tool.go`
  (`getRulesetDescription`) and `internal/mcp/ruleset_tool_test.go`;
  `internal/gateway/qa_ruleset_route_test.go`; new tests in
  `internal/gateway`: a player controlling an actor reads its sheet's
  abilities alone; one controlling actors of different sheets reads their
  union; one controlling none reads none; a spectator none; the DM and the
  agent every one; a grant, then a revocation, change the next read with
  no reconnect; conditions, resources and properties as before for every
  role. Under OD-6 the guide route asks the role first, as
  `handleAdventureGuide` does, so a player is answered 403 whether or not
  a guide is loaded; VTT-115's test reads as the DM, and SPEC-012 says
  which comes first. `docs/requirements.md`: VTT-117 withdrawn, succeeded
  by VTT-702
  and by a row allocated here for the adventure list, the maps list and
  the art answered to every role, and a row for `conditions` answered to
  every role; the tests citing VTT-117 cite the row each holds. VTT-115
  ("a guide that does not exist is answered 404"), false for a player on
  both guide routes once the ruleset guide asks the role first, is
  withdrawn, succeeded by a row allocated here: a guide that does not
  exist is answered 404 to the DM and the agent.
- **True when:** the new tests and the narrowed QA test pass;
  `cmd/vtt/qa_get_ruleset_test.go` passes unchanged.
- **Deliberate breaks:** answer every role every ability (the player and
  spectator tests turn red); take the union over every actor rather than
  the controlled ones (the player test turns red); under OD-6, drop the
  role map (the guide test turns red).

### Task 7: the window (R14, R15, R15b; D8, D10; items 4 and 6)

- **Files:** `client/src/player.ts` (`offered`), `client/src/view/player.ts`
  (`renderPlayerPanel`, its module header), `client/src/app.ts` (the
  re-read of D8); `cmd/vtt/webdist`, rebuilt with `task build:client`; the
  client tests under "The existing tests and data that change"; new tests:
  `client/test/player.test.ts` (`offered`), a panel test for a player, for
  the DM and for the agent, and `client/test/app.test.ts` cases where a
  `WatchableChanged` naming a newly granted character issues a read whose
  answer the panel offers, one dropping it issues a read whose answer no
  longer holds them, and an older answer arriving last is not drawn.
  `docs/requirements.md`: VTT-438 withdrawn, succeeded by VTT-695; the
  tests citing VTT-438 cite the row each holds.
- **True when:** `env -u CLAUDECODE bun test` passes from `client/`;
  `client:typecheck` passes; `client/e2e/table.spec.ts`'s ability test
  passes under `task e2e`.
- **Deliberate breaks:** list every ability whatever the sheet (the panel
  tests turn red); arm from the unfiltered list (a test arming a carried-over
  selection turns red); drop the re-read (the grant case turns red); drop
  the ticket (the ordering case turns red).

### Task 8: the gate

- **Do:** `python3 tools/check-comments.py main` and `--write-ledger` once,
  last, if a ceiling can come down; check free space and the load; run
  `task check` whole on the last code commit, detached under `/bin/sh`,
  output to a file, requiring its completion line; run `task e2e`. Tell
  the owner the client's stored mutation report is out of date from Task 1
  and which commit it measured (SPEC-023); a full Stryker run is his to
  ask for.
- **True when:** `task check` and `task e2e` are green (item 9).

### Task 9: the report, then the specifications

- **The report:** `docs/reports/2026-11-13-a-sheet-offers-only-what-it-names.md`
  with the `implementation-report` skill, and its reading review: each
  "Done looks like" item with its observation; R1 to R19 and what turned
  each red; Rule 9; G1 to G4 and each OD's answer; Phase 4a's adjudications
  under their own heading; the gate.
- **The specifications**, with the `specification` skill, from the report:
  - **SPEC-025:** a sheet's ability ids; `SheetAbilityFault` and its
    callers; the adventure's field; the boot's refusal; `Offers`.
  - **SPEC-007:** `Actor`'s field 10; `add_actor`'s schema documents it.
  - **SPEC-018 (G1):** the list in its second table, with what holds it;
    the display-names sentence says an ability's reaches whom D7 says.
  - **SPEC-016:** `shownWith` and `unseenAs` leave the list out;
    `sameIncarnation` compares it.
  - **SPEC-013:** the use's refusal inside `handleUseAbility`, after the
    sight gate and before `Resolve`, for every role, and its words.
  - **SPEC-012:** `handleRuleset` by role; the principle-35 line; under
    OD-6, the guide's gate.
  - **SPEC-022:** the client fold carries the list; the panel offers what
    the sheet names; the re-read on `WatchableChanged`; the Story's naming
    under OD-5.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and the
surfaces as signatures and names, never the source: `go doc
./internal/rules` (`SheetAbilityFault`, `Offers`), `go doc
./internal/adventure` (`Load`, `AdventureActor`), the `add_actor`,
`use_ability` and `get_ruleset` entries in `contract/gen/tools/tools.json`
and `getRulesetDescription`, the `/api/ruleset` response shape from
SPEC-012, and `offered` and `renderPlayerPanel`'s signatures.
Tell it: never run `task check`, `task e2e`, Stryker or gremlins; its
files are new `qa_` files; semgrep bans game words in Go tests too (`hp`
among them); go-arch-lint holds the layer imports; the tree is hashed
before and after it. Known and not findings: a repeated id is accepted
(OD-4); the list has no fold bound (OD-4); a Story draws an ability its
window cannot read by its id (OD-5); a sheet naming no ability is offered
none, and an actor added before this ticket names none; a revocation of a
non-party actor issues no read (D8); `rules.Resolve`'s refusals keep their
words.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "whatever its kind") names the search that bounds it, P3
and P4 above all; check each comment under `internal/`, `cmd/` and
`client/src` is a warning or a pointer, and that the module header of
`client/src/view/player.ts` lists the sheet's rule; check the tool
descriptions and the ruleset guide against what the server refuses and
serves at that commit; check that `git diff main -- contract/vtt` adds
`Actor`'s field 10 and nothing else.
