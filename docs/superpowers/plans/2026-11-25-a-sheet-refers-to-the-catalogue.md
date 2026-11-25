# Plan: a sheet refers to the catalogue, and the DM and the agent change what it holds

- **Ticket:** `docs/superpowers/specs/2026-11-25-a-sheet-refers-to-the-catalogue-design.md`
  (blueprint principles 11, 12, 23, 25, 26, 27, 34 and 35; ADR-012 §2, §3,
  §9 and "Build order", step 1 item 5, the second of its three tickets).
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-25 with the dev-cycle `verify-ticket` skill by an agent
    that did not write the ticket, against `13eeca0` (= `origin/main`) on
    `feat/a-sheet-refers-to-the-catalogue` in the worktree
    `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited. `task check`, `task e2e`,
    Stryker, gremlins, `task build:client`, `go clean -cache` and
    `requirement-id` were not run. Probes P1 to P4 ran in a copy of the tree
    made with `git archive HEAD` in the session's scratchpad; P4 read a copy
    of the alpha campaign's `log.db`, never the file itself. `git status`
    afterwards showed only the ticket. The rest are searches, targeted
    `go test` runs in that copy and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-25-a-sheet-refers-to-the-catalogue.md`.
- **Base:** `13eeca0`. **Branch:** `feat/a-sheet-refers-to-the-catalogue`.
- **Owner's rulings taken in:**
  - 2026-11-24: item 5 is three tickets: 5a, done (SPEC-027); this one, a
    sheet's references, slots and sets with the campaign's state per
    reference; 5c, the actions reached through references in place of
    `Actor.ability_ids`. Only the DM and the agent change what a sheet
    carries until step 2 makes equipping an effect. dnd45e-minimal gets the
    first real catalogue; tavern-brawl only what tests need.
  - 2026-11-24, during 5a: "Du kan sätta vilka värden du vill när det gäller
    utrustning. Det viktiga är att vi kan läsa och använda vilka värden som
    helst för vilken utrustning som helst oberoende av regelverket." Every
    test uses invented keys; nothing special-cases dnd45e's weapon or armor.
  - 2026-11-25: one ticket, its tasks committed separately.
  - 2026-11-25: a place on a sheet has no limit, no count and no capacity:
    "It is normaly weight vs strength(some attribute) that determines how
    much a character can carry"; that comes as the ruleset's own sum once
    derived properties exist (step 1 item 6). Until then the DM judges.
  - 2026-11-25: the 5a hole (OD-2 A of
    `docs/superpowers/plans/2026-11-24-a-ruleset-declares-forms-and-a-catalogue.md`)
    closes as "an item's value is refused": a value a catalogue form lists
    belongs to the item, and a sheet holding it is refused. Not "the sheet
    becomes a form".
  - 2026-11-06, "ett spel bland vänner": no visibility per property; a
    player whose character sees a companion sees its whole sheet, the DM sees
    everything, a creature's sheet only the DM.
  - 2026-11-08: the DM owns the fit; no value on a sheet the DM did not
    write. 2026-11-15, the precedent: a change carries the value it needs, 0
    is a value, a missing one is refused.
  - 2026-11-14: the player is told when an ability is given or taken (the
    precedent OD-2 asks about).
  - 2026-11-22 and 2026-11-23: every removal in the DM console asks first
    (Remove, Remove token, Delete note, Remove condition).
  - Standing: a near-theoretical edge case gets no code, no test and no debt
    (2026-10-21); no migration (2026-09-04); one source per fact (2026-09-23);
    no counts or measured figures in prose meant to last; a code comment is a
    warning or a pointer (rule 10); the client's full mutation run only when
    the owner asks; push and merge on his word.
  - 2026-11-25, at sign-off: OD-1 A, no charges in this ticket; and OD-2 B,
    with the owner's reason: "2, för att om en tjuv skulle stjäla något,
    utan att karaktären märker det "pick pocket sucess" då kan vi inte
    produkten berätta att - du blev av med ditt långsvärd". The product never
    tells a player that their character gained or lost an item; the sheet
    shows it once the DM changes it.
- **Signed off:** 2026-11-25, "Approved", with OD-1 A and OD-2 B. The
  decisions, rows and tasks below are written for those answers.
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off, on
  2026-11-25. The `R` labels below are this plan's; the sort split R10, R13,
  R16, R19, R25, R26, R27 and R42, each into rows of one rule, added R19c
  from D22 and R46 from D14, and refused R36.
  R1 VTT-1010, R2 VTT-1011, R3 VTT-1012, R4 VTT-1013, R5 VTT-1014, R6
  VTT-1015, R7 VTT-1016, R8 VTT-1017, R9 VTT-1018 (succeeds VTT-996), R10
  VTT-1019 (succeeds VTT-825), R10b VTT-1060 (a section's places named),
  R11 VTT-1020, R13a VTT-1021 (the schema
  describes places), R13b VTT-1022 (succeeds VTT-1009), R14 VTT-1023, R15
  VTT-1024, R16a VTT-1025 (a removal), R16b VTT-1026 (a change), R17
  VTT-1027, R18 VTT-1028, R19a VTT-1029 (the Story; succeeds VTT-809), R19b
  VTT-1030 (the Events panel), R19c VTT-1031 (no watchable set changes:
  OD-2 B), R20 VTT-1032, R21 VTT-1033, R22 VTT-1034, R23 VTT-1035, R24
  VTT-1036, R25a VTT-1037 (a reference fits), R25b VTT-1038 (the copies
  rule), R26a VTT-1039 (served), R26b VTT-1040 (an empty id, served or
  not), R26c VTT-1041 (none served), R27a VTT-1042 (read and compiled),
  R27b VTT-1043 (refused), R28 VTT-1044, R29 VTT-1045, R30 VTT-1046, R31
  VTT-1047, R32 VTT-1048, R33 VTT-1049, R34 VTT-1050, R35 VTT-1051, R37
  VTT-1052, R38 VTT-1053, R39 VTT-1054, R40 VTT-1055, R41 VTT-1056
  (succeeds VTT-840), R42a VTT-1057 (the console changes), R42b VTT-1058
  (it asks before a removal), R45 VTT-1059, R46 VTT-1061 (none served, in
  play). Each succeeded row is withdrawn in the task that makes it false or
  narrower than its tests: VTT-996, VTT-825 and VTT-1009 in Task 2,
  VTT-809 in Task 4, VTT-840 in Task 10.

## The verdict, check by check

1. **Every path resolves. By command.** Every file under "What it touches"
   and "Specifications this moves" exists, and so do
   `~/dev/RPTool/rule-framework/src/RuleEngine.ts` and
   `~/dev/RPTool/Documentation/migration-guide.md`. Every symbol the ticket
   names is where it says (a `grep -E` per symbol): `Actor` with
   `attributes`, `resources`, `module_data`, `ability_ids`; `AbilityGiven`,
   `AbilityTaken`, `ResourceAdded`, `ResourceRemoved`; `GiveAbility`,
   `TakeAbility`; `actorJSON` with `actor_id` and `ability_ids`;
   `manifestJSON` with `properties`, `forms` and `sheet`; `SheetSection`;
   `engine.Apply`; `Ruleset.SheetKeyFault`; `sheetsFit`; `validateForm`;
   `ToEvent`; `renderPlayerPanel`; `getRulesetDescription` and its sentence
   "A number property's value is under an actor's `attributes`";
   `add_actor`'s "under a key the served ruleset declares as a number
   property"; guide.md's "never set it on a creature"; and
   `EquippedWeapon`, `EquippedImplement` and `EquippedOffhand` in
   `RuleEngine.ts`. **One location is off:** the sheet the panel draws is
   built by `sheetFor` in `client/src/player.ts`, which the ticket does not
   name; `client/src/view/player.ts` only renders it (G2).
2. **"Done" is an observation. By command for items 1 to 4, by reading for
   5 to 9.** The ticket names no test by name; each item says a test of it
   fails today, and these show why.
   - Item 1: a `ruleset.json` with `"places": []` is refused as `json:
     unknown field "places"` for `testdata/valid`, dnd45e-minimal and
     tavern-brawl alike (P1).
   - Item 2: with dnd45e-minimal loaded, `SheetKeyFault` answers nil for
     each of `proficiency`, `damage_dice`, `damage_die`, `range` and
     `armor_bonus` under `attributes` (P2), so all three entry points accept
     such a sheet today (P5).
   - Items 3 and 4: `protojson` refuses an `Actor` holding `references` as
     an unknown field (P3), and `actorJSON` and `go doc` of `vttv1.Actor`
     show no such field.
   - Items 5 to 8, by reading: no command, event, projection arm, route
     field, sheet section kind or client section names a reference today;
     `getRulesetDescription` and `add_actor`'s docs say nothing of one.
   - Item 9, by reading: `actorAddedDump` has no field for references, so
     the compiled golden cannot name one.
   - Item 10 names the gate.
3. **Each rule is breakable. By reading.** Each rule line maps to rows of
   "Requirements" with what turns them red. Two are thin: "in a state the
   place names" does not say whether a place may name none (G4), and
   "keeps each copy's state and charges apart" leans on charges, which the
   ticket leaves undefined (G5). The rows observe both once D3 and OD-1 are
   settled.
4. **The scope matches the claim. By command, then reading.**
   - `git grep -nE 'Sheet(Key|Ability|Spend)Fault\(' -- '*.go' ':!*_test.go'`
     prints `sheetFault` (add_actor), `loadActors` (`adventure.Load`) and
     `sheetsFit`, and `validateGiveAbility`'s `SheetAbilityFault`: a refusal
     inside `SheetKeyFault` reaches every entry point item 2 names, and
     nothing else (P5).
   - `git grep -n 'Envelope_ActorAdded{' -- '*.go' ':!*_test.go'
     ':!contract/gen'` prints `adventure.Compile`, `ToEvent`, the
     projection's `introduce` and `internal/eventgen` (P6).
   - The tests that iterate the contract's oneofs —
     `TestEveryEnvelopePayloadArmHasAnExplicitRuling`,
     `TestEveryClientCommandConverts`, `TestEveryClientCommandHasRoleCells`,
     `TestManifestCoversAllCommandMessages` — pass today and force a ruling
     on every new arm; `STORY` in `client/src/view/feed.ts` is a
     `Record<Kind, boolean>` that fails `client:typecheck` until it names
     each new payload, while the client fold skips an unknown payload (P7,
     P8).
   - The ticket's list omits files and data this work moves (G2), and one
     golden it moves without naming it: `scenarios/goldens/adventure-night`
     loads goblin-ambush (P10).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-025's consequence that a property a form lists is a key a sheet
     may hold, and SPEC-027's "No sheet refers to an entry", are overturned
     by item 2 and item 3, by the owner's 2026-11-25 ruling, named.
   - ADR-012 §3's slot capacity is put off by the 2026-11-25 ruling, named;
     §2's per-property and per-form visibility is answered by the
     2026-11-06 ruling, named; §2's set conditions, budget and values per
     entry are under "Not this ticket".
   - The 2026-11-14 ruling that the player is told of a changed ability
     list is not contradicted; the ticket is silent on whether a changed
     reference is told (G8, OD-2).
   - VTT-825 and VTT-840 become false as worded once a sheet section can
     show places (G9); a row, not a decision, and the plan succeeds both.
6. **The records the work moves are named. By command, then reading.**
   Every path under "Specifications this moves" resolves, and the new one is
   written `New:`. By reading, searching the specifications for the
   subjects (`grep -lE 'AbilityGiven|give_ability|SheetKeyFault|sheetsFit|catalogue'
   docs/specifications/*.md`): SPEC-021 moves and is not named (its reopen
   sentence lists what a resumed log holds of a party member, G3); SPEC-015
   moves under OD-2 A; SPEC-026 does not move, since no new handler calls
   `rulesetNamed` (D14) and its "No other command calls `rulesetNamed`"
   stays true.

## The gaps this plan inherits

- **G1. Rule 9.** The ticket's MapTool paragraph is right in what it says
  (P15); the rule-9 section below completes it with the fixed second weapon
  of the framework's older form, the framework's fallback die and MapTool
  core's `TokenProperty` default and visibility, none of which is borrowed.
- **G2. "What it touches", measured.** Not named, and changed:
  `client/src/player.ts` (`sheetFor`, `ShownSection`), `client/src/state.ts`
  (`Actor`), `client/src/view/feed.ts` (`STORY`), `client/src/view/spectator.ts`
  (`describe`), `client/src/view/naming.ts` (`Namer`), `client/src/app.ts`
  (what the panel and the console are handed);
  `internal/rules/qa_declared_sheet_test.go` (the manifest's field list),
  `id_bound_internal_test.go` and `qa_property_format_test.go` (VTT-1009),
  `schema_test.go`'s tables and `testdata/valid` (without which
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` skips a new required
  field silently, P9); `contract/testdata`'s fold fixtures and a test in
  `cmd/vtt` and in `client/test` that fold them (P11);
  `internal/perceive/projection_internal_test.go`; `internal/eventgen/model_test.go`;
  an `internal/campaign` reopen test; `internal/adventure/conformance/qa_data_abilities_test.go`
  (the guide's statblocks, P12); `scenarios/goldens/adventure-night` (P10);
  `docs/requirements.md`; `tools/comment-ceilings.txt`.
  Named and, under this plan, unchanged: `internal/harness/engine.go` (each
  new command appends one envelope, so `isBatchCommand` stays as it is,
  D14) and `rulesets/tavern-brawl/` (D20).
- **G3. SPEC-021 is not listed.** Its reopen sentence says what a resumed
  log holds of a party member; references join it, held by a reopen test
  (R24).
- **G4. Item 1's refusals are narrower than the rules.** It names no
  refusal of a place whose form list is empty or repeats a form, nor of a
  state past the bound on an id, which reaches the log; and it does not say
  whether a place may name no state. D3.
- **G5. Charges are undefined.** OD-1 A: none in this ticket.
- **G6. An empty reference id with no ruleset served.** Item 4 says that
  with none served `add_actor` reads none of it against one; an empty id is
  the one thing both folds and the projection key on. D13 refuses it with or
  without a ruleset, as VTT-747 does an empty ability id.
- **G7. The live campaign, established.** A copy of
  `~/dev/vtt-alpha/campaign/log.db`, last written 2026-11-15, folds to
  actors none of which holds a key a form lists (P4): `vtt serve --ruleset`
  would not refuse it under item 2.
- **G8. A player looking through another of their characters.** A player
  watches one character's log at a time; a reference changed on another of
  their characters reaches them only when they switch, unless the watchable
  set carries it, as it carries ability ids since the 2026-11-14 ruling
  (P14). OD-2 B: not told (D22).
- **G9. "Where the ruleset's sheet lays them out" names no section kind.**
  D5 adds one; VTT-825 ("a section showing none or more than one of values,
  conditions and abilities") and VTT-840 (the default page's three
  sections) become false as worded and are succeeded.
- **G10. "The weapons they fight with".** By their abilities: the fighter a
  longsword and a crossbow, the cutter a scimitar, the archer a scimitar and
  a shortbow. Which armour each wears is content this plan proposes (D21).
- **G11. SPEC-018 for the new fields.** The ticket bounds the place key
  only. D3 and D10.
- **G12. A sheet section showing a property a form lists.** After item 2 no
  sheet holds such a value, so a `values` section naming one shows nothing;
  nothing ships one. D6 leaves it loading.

## Measurements this plan stands on

All at `13eeca0`, in the scratchpad copy unless said otherwise.

- **P1.** A probe test in `internal/rules` copies each of `testdata/valid`,
  `rulesets/dnd45e-minimal` and `rulesets/tavern-brawl`, adds `"places":
  []` to `ruleset.json` and calls `rules.Load`: each is refused, `json:
  unknown field "places"`.
- **P2.** The same probe loads dnd45e-minimal and asks `SheetKeyFault` of
  each property a form lists: nil under `attributes`, and under `resources`
  `gives number property "<key>" a resource's value`.
- **P3.** `protojson.Unmarshal` of `{"actorId":"a","references":{}}` into a
  `vttv1.Actor` is refused, `unknown field "references"`; of a map written
  with one key twice, `duplicate map key "x"`. A Go program decoding
  `{"attributes":{"a":1,"a":2}}` with `encoding/json` and
  `DisallowUnknownFields` keeps the last value and reports nothing, which is
  how an adventure file is decoded today.
- **P4.** A Go program in the copy, `campaign.Open` over a copy of the
  alpha campaign's `log.db` and `rules.Load` of dnd45e-minimal, folds five
  actors (`gob-archer`, `gob-cutter`, `gob-sleeper`, `lera`, `tobbe`), none
  holding a key a form lists, and `SheetKeyFault` answers nil for each.
- **P5.** `git grep -nE 'Sheet(Key|Ability|Spend)Fault\(' -- '*.go' ':!*_test.go'`:
  callers in `internal/gateway/add_actor_validate.go` (`sheetFault`),
  `internal/adventure/load.go` (`loadActors`), `cmd/vtt/serve_compose.go`
  (`sheetsFit`), and `internal/gateway/ability_validate.go` for an ability
  id alone.
- **P6.** `git grep -n 'Envelope_ActorAdded{' -- '*.go' ':!*_test.go' ':!contract/gen'`:
  `internal/adventure/compile.go`, `internal/eventgen/model.go`,
  `internal/gateway/convert.go`, `internal/perceive/projection.go`.
- **P7.** By reading: `client/src/fold.ts`'s `apply` ends `default:
  return`, so an unknown payload folds to nothing; `STORY` in
  `client/src/view/feed.ts` is `Record<Kind, boolean>`; `.lefthook.yml`'s
  pre-commit runs `client:typecheck` and tier-1 tests, so the commit that
  adds a payload carries its `STORY` entry, its `classify` arm and an arm
  of `describe` in `client/src/view/spectator.ts`, whose switch has no
  default and returns a string; the same holds for a new member of
  `SheetSection`'s union and `sheetFor` in `client/src/player.ts` (a
  `tsc --strict` probe of both gives TS2366).
- **P8.** `go test ./internal/perceive/ -run TestEveryEnvelopePayloadArmHasAnExplicitRuling`,
  `go test ./internal/gateway/ -run 'TestEveryClientCommandConverts$|TestEveryClientCommandHasRoleCells$|TestEveryPlayerCommandHasARule$'`
  and `go test ./tools/toolgen/` pass; each iterates the oneof it guards.
- **P9.** `go test ./internal/rules/ -run 'TestQASheetSchemaDescribesTheSheet$|TestTheSchemaDescribesEveryFormField$|TestSchemaRequiredFieldsMatchLoaderEnforcement$|TestTheSchemasStateTheIDBound$' -v`
  passes, and the walker skips the sheet's `spends`, `conditions` and
  `abilities`, which `testdata/valid` does not demonstrate. By reading:
  the first pins the manifest's top-level fields, `forms` last; the second
  pins a form's four fields and their number.
- **P10.** By reading `scenarios/adventure-night.json` and its golden: it
  loads goblin-ambush and grants `act-fighter` to a player;
  `scenarios/goldens/adventure-night/state.json`, derived by hand, and
  `stream.json`, recorded, hold each goblin-ambush actor's `ActorAdded`.
  `client/e2e/table.spec.ts` reads the shipped `sheet` and checks the
  Human Fighter's second page opens on that page's first section.
- **P11.** By reading: `contract/testdata/ability_change_log.json`,
  `ability_change_dump.json` and `ability_change_refused.json` are folded
  by `cmd/vtt/ability_change_dump_test.go` and
  `client/test/fold-abilities.test.ts` (VTT-727): both folds against one
  committed dump, and every refused envelope refused by both.
- **P12.** `go test ./internal/adventure/conformance/ -run 'TestQADataGuideStatblocksNameAbilities|TestQADataScenarioGoldensCarryAbilityIDs'`
  passes; the first keeps each statblock in guide.md naming its adventure
  actor's `abilityIds`, the second each golden's `ability_ids` right after
  `kind`.
- **P13.** By reading `internal/perceive/projection.go`: `shownWith` builds
  a creature's form from its id, module id, kind and controllers alone, so a
  new `Actor` field is withheld from a creature by construction;
  `actorBelief`, `beliefOf`, `statusFrames` and `kindMovesList` read
  resources and ability ids, and `classify`'s sheet arm forwards a party
  member's sheet events to a log that saw it.
- **P14.** By reading: `watchable` in `internal/gateway/viewpoint.go` copies
  `ability_ids` into each `WatchableActor` the participant controls, and
  `pump` sends the set again whenever it differs (`proto.Equal`), which is
  how a give reaches a player watching another character.
- **P15.** RPTool at `63862e4`, MapTool at `f4b7fef6c`:
  `rule-framework/src/RuleEngine.ts` reads `EquippedWeapon`,
  `EquippedImplement` and `EquippedOffhand` from the caster's properties and
  falls back to `"1d4"` when the off-hand or the weapon gives no die;
  `Documentation/migration-guide.md` maps the older `Weapon` and `Weapon2`
  token properties (`Weapon2` "Not implemented") onto `EquippedWeapon`'s
  JSON; `grep -rli 'inventory|carried|backpack'` over the framework's
  sources, data and documentation, and `grep -rl inventory` over MapTool's
  `src/main/java`, print nothing; `TokenProperty` holds `defaultValue`,
  `ownerOnly` and `gmOnly`.
- **P16.** By reading: `handleTakeAbility` opens its batch with
  `rulesetNamed`, `handleGiveAbility` never does (VTT-990, "what a give
  writes is the DM's"), and SPEC-026 says no other command calls it.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no gate moves; a test whose assertion changes does
  so because a decision here changed the rule, and the task names it); 3
  (every `.proto` change adds and renames nothing: new messages, `Actor`
  field 11, new oneof arms; generated code regenerated by `task
  generate:contract` and committed with it; `check:breaking` only reports);
  4 (the three events fold in `engine.Apply` and nowhere else); 5 (no game
  word in Go under `internal/` or `cmd/`, tests included; SPEC-027's `git
  grep -n -i -E 'weapon|armou?r' -- internal cmd ':!*_test.go'` keeps
  printing nothing); 8 and 10 (names, never line numbers; a new comment is a
  warning or a pointer, to SPEC-028, SPEC-025 or a requirement); 9 (below).
- **Principles** 11 (both folds derive one present), 23, 25 (every fault in
  a ruleset refused at load, file and field named), 26 (a sheet that does
  not fit is refused where it enters, whichever way), 27 (a sheet names an
  entry and copies nothing of it), 34, 35.
- **SPEC-018's mirror rule:** an id the loader bounds is bounded at
  `maxIDBytes` and its schema says so (VTT-1009's successor).
- **The order of refusals:** every refusal a sheet or a ruleset meets today
  keeps its message and its place; what this work adds comes after what is
  there (D3, D5, D6, D13).
- **The mechanical gates**, as `docs/superpowers/plans/2026-11-23-the-log-names-its-ruleset.md`
  lists them. The Go mutation gate's `PACKAGES` (`tools/check-mutation.py`)
  hold every `internal/` package this work changes but `internal/eventgen`;
  `cmd/vtt` and `tools/toolgen` are outside it. Each new branch gets a test
  that kills it, each bound a test at it and one past it. A client change is
  committed with the `cmd/vtt/webdist` that `task build:client` makes of it;
  client tests are measured with `env -u CLAUDECODE` and
  `tools/ts_mutation_inputs_test.py` runs before a commit that adds one;
  `tools/check-new-prose` and `python3 tools/check-comments.py` run before
  each commit; stage and commit in separate calls; a new test goes after a
  closing brace, never inside a neighbour's doc comment.

## Rule 9: how MapTool does this

Read at the commits P15 names.
- **MapTool's core has no item and no list of what a token carries.** A
  token holds properties its type declares, each a string with a
  `defaultValue`, an `ownerOnly` and a `gmOnly`.
- **The owner's rule framework** keeps equipment in three token properties
  of fixed name, `EquippedWeapon`, `EquippedImplement` and
  `EquippedOffhand`, each a JSON object with the item's own numbers (its die,
  proficiency, enhancement, reach or range), which an ability reads; an
  attack whose weapon gives no die rolls `1d4`. The older framework it
  migrates from had `Weapon` and `Weapon2`, a second weapon being a second
  property of fixed name.
- **Borrowed:** a named place on the character holding the item an action
  will read; the reading itself is 5c's.
- **Not borrowed:** the copy on the token (principle 27: a reference names
  an entry and holds only its place and state); place names fixed
  in the engine (principle 23: the places are the ruleset's, and one place
  may hold several references, so a second weapon needs no second name); the
  fallback die and `TokenProperty`'s default (the 2026-11-08 ruling: no
  value the DM did not write); `ownerOnly` and `gmOnly` (the 2026-11-06
  ruling: sight decides, never a flag per property).

## Decisions

1. **D1. One kind of place.** A place holds references to entries of the
   forms it names, says whether it holds one entry more than once, and may
   name the states a reference there is in. ADR-012's slot and set become
   one kind. **Forced by** the 2026-11-25 ruling: what set a slot apart was
   its capacity, and a place now has none; a capacity, an `occupies`, a
   set's `admits`, budget and values arrive later as additions to a place.
   **Preference:** the word `places` (ADR-012's names are working names).
   ADR-012 §10 calls a contribution's target a place; that word is not yet
   in code, and the step 2 ticket that builds contributions takes another,
   since `places` in `ruleset.json` and `Reference.place` on the wire are
   permanent from this ticket (rule 3).
2. **D2. Places are their own list in `ruleset.json`.** `manifestJSON`
   gains `Places []placeJSON`; `rules.Ruleset` gains `Places []Place` in
   declared order, nil when absent, `null` or `[]`. Not a property type in
   `properties`. **Forced by** the 2026-11-25 ruling (the sheet does not
   become a form) and 5a's P11: a third property type widens
   `Property.type` and fails `client:typecheck` in `sheetFor`, and VTT-611,
   VTT-614, VTT-615, VTT-618, VTT-677, VTT-678 and VTT-826 are worded for
   two types.
3. **D3. A place.** `{"key", "name", "description", "forms", "copies",
   "states"}`; `Place` is `{Key, Name, Description string; Forms []string;
   Copies bool; States []string}`. Refused, naming `ruleset.json` and
   `places[<i>].<field>`, place by place: a key, name or description absent
   or empty, in that order; a key an earlier place declares; under `forms`,
   an absent or empty list, then `forms[<j>]` naming no declared form or
   one listed already; under `states[<j>]`, an empty state or one listed
   already. `checkManifestBounds` refuses a key or a state longer than
   `maxIDBytes`, after the form keys. `copies` is optional, absent meaning
   once (the loud direction: a second copy is refused, not silently kept);
   `states` is optional, and a place naming none holds references with no
   state. A place's key is free in its characters and may equal a
   property's or a form's key: nothing reads it in an expression, and
   SPEC-027's one namespace holds entries, abilities and conditions only.
   `loadPlaces` runs after `loadForms`. **Forced by** item 1, G4 and the
   mirror rule; the state's bound by SPEC-018, since a state reaches the
   log. The two bounds sit in `checkManifestBounds`, after its bounds on
   the id, version, name, property keys and form keys; it runs before any
   property or form is read, so they come before every property, form,
   sheet and catalogue refusal a ruleset meets today. No ruleset that loads
   today declares a place, so none meets a refusal in another order.
4. **D4. No charges** (OD-1 A). A form gains no field and a reference
   carries no number; charges come with step 2, beside what spends them.
5. **D5. The sheet's places section.** A section shows exactly one of
   `values`, `conditions`, `abilities` and `places`; `places` is a
   non-empty list of declared place keys, none shown twice on the sheet,
   refused as `values` are (VTT-825, VTT-826, VTT-827's shapes); with a
   sheet declared, a place no section shows is refused after the abilities'
   check, naming the file and the place (VTT-828's twin). `SheetSection`
   gains `Places []string`. **Forced by** item 7 and principle 35: a
   reference in a place the sheet does not show is one a player cannot read.
6. **D6. An item's value is refused, inside `SheetKeyFault`.** After the
   attributes sweep and the resources sweep, a third reads `attributes` in
   byte order and answers the first key a form lists, as `SheetFault{Field:
   "attributes"}` with a reason naming the key and the first form in
   declared order that lists it. **Forced by** the 2026-11-25 ruling, P5
   (one change reaches `add_actor`, `adventure.Load` and `sheetsFit`) and
   the order constraint: VTT-617, VTT-618, VTT-677 and VTT-678 each name
   the first undeclared or other-type key, and stay true only if the new
   refusal comes after both sweeps. Not read: `resources` (a form lists
   `number` properties only, VTT-995, so such a key there is already refused
   as the other type, P2). Not changed: a `values` section naming such a key
   (G12) and an expression reading one, which fails at use as any key a
   sheet does not hold does today; 5c decides how an expression reads an
   entry.
7. **D7. A reference.** `vtt.v1.Reference {string entry_id = 1; string
   place = 2; string state = 3;}` in a new
   `Actor` field `map<string, Reference> references = 11`, keyed by the
   reference's id, which is the sheet's own (two sheets may use one id).
   The id is written by whoever adds the reference: the DM, the agent or
   the adventure's author. **Forced by** the platform's precedent that no id
   is minted for a caller (`place_token`'s `tokenId`, `add_actor`'s
   `actorId`, the console's id fields), by an adventure needing its ids to
   compile the same events on every load, and by item 3 (two references to
   one entry told apart). A map, as `resources` is: `protojson` refuses one
   id written twice (P3).
8. **D8. Three events and three commands.** `ReferenceAdded {actor_id,
   reference_id, Reference reference}`, `ReferenceRemoved {actor_id,
   reference_id}`, `ReferenceChanged {actor_id, reference_id, place,
   state}`, at `Envelope.payload` 41 to 43; `AddReference`,
   `RemoveReference` and `ChangeReference`, the same fields, at
   `ClientCommand.command` 39 to 41, as `add_reference`, `remove_reference`
   and `change_reference`. A change sends the reference's place and state
   whole and keeps its entry; another entry is a removal and an addition.
   **Forced by** item 5 with the least contract: D9 makes a move carry a
   state, so a move and a change of state are one write, sent whole, as
   `TokenDescribed` is.
9. **D9. A change carries what it lands with.** A move names the state the
   reference takes in the new place, required when that place names states
   and refused when it names none. Nothing is kept from the old place or
   filled in. **Forced by** the 2026-11-08 ruling and the 2026-11-15
   precedent.
10. **D10. Both folds.** `ActorAdded` keeps an actor's references whole and
    refuses an empty reference id, after its attribute names.
    `ReferenceAdded` refuses an empty reference id, then a missing
    reference, then an actor the state does not hold, then an id the sheet
    holds already; `ReferenceRemoved` and `ReferenceChanged` refuse an actor
    the state does not hold, then an id the sheet does not hold. Each writes
    a fresh map and changes nothing else. No length bound: neither of
    SPEC-018's tests is met, as the 2026-11-14 OD-6 ruling found for an
    ability id; with a ruleset served the loader bounds the entry id, the
    place and the state (D3). The client mirrors each refusal's order and
    words, and its dump writes `references` after `ability_ids`, each
    reference's fields in field order (`encoding/json` over the generated
    struct).
11. **D11. Fold parity by fixtures.** `contract/testdata/reference_change_log.json`,
    `reference_change_dump.json` and `reference_change_refused.json`,
    folded by a `cmd/vtt` test and a `client/test` test, as P11's are. No
    scenario is added for the events; adventure-night carries
    references through `ActorAdded` once the content lands (P10).
12. **D12. The projection.** References are what an event can change of a
    party member: `actorBelief` holds them, `beliefOf` reads them,
    `sameIncarnation` does not compare them. `statusFrames` brings a held
    party member's references to the world's, in byte order of their ids,
    after the ability frames: a `ReferenceRemoved` for one the world does
    not hold or holds as another entry, a `ReferenceAdded` for one the log
    does not hold or held as another entry, a `ReferenceChanged` for one
    whose place or state differs. `kindMovesList` counts
    references; `Resume` follows through `statusFrames`; `classify`'s
    sheet arm and `sheetActor` take the three events. **Forced by** item 6,
    SPEC-016's rule for a sheet's events, the 2026-11-06 ruling and P13.
13. **D13. Where a sheet enters.** `Ruleset.ReferenceFault(id, r)` reads one
    reference: its entry against `Catalogue`, its place against `Places`,
    the entry's form against the place's forms, and its state against the
    place's states.
    `Ruleset.SheetReferenceFault(refs)` asks it of each in byte order of
    ids, then refuses the first reference, in that order, naming an entry
    an earlier one names in the same place when that place holds it once.
    Each answers a `SheetFault{Field: "references"}` whose reason names the
    reference. Callers read it after the spend rule. An empty reference id
    is refused by `add_actor` and `adventure.Load` with a ruleset served or
    none, before the ruleset reads them (VTT-747, VTT-748's precedent, G6);
    with none served nothing else is read.
14. **D14. The commands in play.** `validateForm` gains
    `validateAddReference`: an empty id with or without a ruleset, then,
    with one, `ReferenceFault`. A handler beside `handleGiveAbility`, with
    a ruleset served and an actor the snapshot holds, refuses an addition
    the copies rule refuses against the sheet; for a change of a reference
    the sheet holds it builds the reference as it will be, its entry from
    the sheet, and asks `ReferenceFault` and the copies rule against the
    sheet's others. `remove_reference` goes through `ToEvent` alone. Each
    appends one envelope and no `RulesetNamed` (P16: what each writes is the
    DM's), so `isBatchCommand` is unchanged. Roles: the DM and the agent;
    no `playerRules` entry.
15. **D15. The tools.** A manifest entry each for the three commands, whose
    descriptions say what changes, what is refused with and without a
    ruleset, that a change sends the place and the state whole, that no
    player is sent a message of it and that whoever is sent the sheet sees
    it there (OD-2 B; `give_ability`'s says each controlling player is told,
    VTT-735, so the agent would otherwise assume the same),
    and DM/agent only; `vtt.v1.Reference`'s override requires
    `entryId` and `place`; `add_actor`'s `references` says what it refuses
    and `attributes` that a key a form lists refuses the whole command.
    **Forced by** principle 34.
16. **D16. The read surface.** `/api/ruleset` answers `places`, each
    `{key, name, description, forms, copies, states}` in declared order,
    `states` as `[]` when empty (a place's `forms` never is, VTT-1017), to
    every role, `[]` when none or no ruleset; a section of the `places` kind
    as `{name, kind: "places", places}`. `getRulesetDescription` gains what
    each says and that a value a form lists is refused on a sheet.
    `client/src/metadata.ts` types `places` in Task 3 and the section's
    `places` kind in Task 10, beside `sheetFor`'s arm, since
    `client:typecheck` runs at every commit (P7).
    **Forced by** item 8 and SPEC-012's every-list-non-null rule.
17. **D17. The panel.** A `places` section shows the references of the
    character the panel follows in the order the section lists its places,
    then by reference id, each by its entry's name (its id when the window's
    catalogue has none), with its state as written, titled by the entry's
    description; a section with none is
    left out (VTT-839). With no sheet declared, after "Abilities", one
    section per place that holds a reference, in the ruleset's order and
    then by key for a place it does not declare, named by the place's name
    or its key (VTT-840 succeeded).
18. **D18. The console.** Per actor, its references, each removable after
    the console asks (the 2026-11-22 and 2026-11-23 rulings), changeable by
    place and state, and an addition by id, entry, place and state, each
    sent through `sending`. **Preference:** the group's
    title, "Catalogue".
19. **D19. The Story and the Events panel.** The Story draws no line for the
    three events (VTT-809's rule for a sheet's bookkeeping); the Events
    panel names the actor for each as it names any actor, and the entry and
    the place by the names the window holds or else by their ids. A removal and a change carry no
    entry, so the namer walks the log for each reference's last entry and
    place, as `pastActors` does for an actor; Task 4 gives `describe` its
    three arms naming the actor and the reference id, and Task 5 the
    names.
20. **D20. Unchanged:** `rulesets/tavern-brawl/` (every test of the format
    reads fixtures); `internal/harness/engine.go`; SPEC-026's writers;
    ADR-012 §2's `ref` (one entry of a form on a sheet is a capacity of one,
    which the 2026-11-25 ruling put off, and nothing shipped needs one);
    dnd45e-minimal's `"version": "1"` (**preference**, as 5a's D16: no
    outcome changes).
21. **D21. The content. Preference** (the 2026-11-24 ruling makes the
    equipment's details this plan's): dnd45e-minimal declares `wielded`
    (weapon, copies), `worn` (armor) and `carried` (weapon and armor,
    copies), none with states, and a section "Equipment" on the "Character"
    page showing the three. goblin-ambush: the fighter wields a longsword,
    carries a crossbow and wears chainmail; the cutter wields a scimitar and
    wears leather armor; the archer wields a shortbow, carries a scimitar and
    wears leather armor; each reference's id is its entry's id. guide.md's
    statblocks carry the same references, and it says how to add, change and
    remove one and that a key a form lists is refused.
22. **D22. The player is not told** (OD-2 B, and the owner's reason: a
    pickpocket's success is not the product's to announce).
    `WatchableActor` gains no field and the window shows no message: a
    change reaches a player only as the sheet of the character they look
    through, once the DM makes it. The Story draws no line for it (D19), and
    the Events panel is the DM's and the agent's alone (VTT-445).
23. **D23. The records.** A new SPEC-028 holds places, references, their
    refusals where a sheet enters and changes, the three events and what a
    log is sent of them; SPEC-007, SPEC-012, SPEC-013, SPEC-016, SPEC-018,
    SPEC-021, SPEC-022, SPEC-024, SPEC-025 and SPEC-027 move for their part
    (Task 14).

## Open decisions for the owner

- **OD-1. Charges: what a reference counts. Recommended: B. Answered A,
  2026-11-25.** A wand's
  charges, the arrows left in a quiver: the DM writes the number on the
  character and changes it as they are spent. Nothing spends them by itself
  until step 2.
  - **A: no charges in this ticket.** A reference has a place and a state
    only; charges come with step 2, beside what spends them and destroys an
    item at a bound. Cost: Done items 3 and 5 lose "charges", and until then
    the DM keeps the count in a note.
  - **B: the ruleset says which kinds of item have charges.** A form says
    whether its entries have charges; a reference to one carries a number
    the DM writes, 0 included, and a reference to any other carries none.
    With a ruleset served, a missing count or one on an item without
    charges is refused. No bounds. Cost: a field on a form (its check,
    schema, route and tool description), the `optional` field on the
    contract, and a check where a sheet enters and changes.
  - **C: any reference may carry a number.** Nothing declares charges.
    Cost: the contract field alone; a longsword with charges is accepted,
    and the ruleset says nothing about which items count.
- **OD-2. Is the player told when what their character carries changes?
  Recommended: A. Answered B, 2026-11-25, with the reason quoted under the
  owner's rulings.** A player looks through one character at a time. Today
  a player whose other character is given or loses an ability is told at
  once (the owner, 2026-11-14); a sword taken from the character they are
  not looking through shows only when they switch to it.
  - **A: told, as for abilities.** A message names the character and each
    item added and taken away; a move or a changed state or charge is not
    told, since the sheet shows it. Cost: one contract field on the
    watchable set, the server's copy into it, the window's message, and
    their tests; SPEC-015 and SPEC-007 move.
  - **B: not told.** The sheet shows the change when the player looks
    through that character. Cost: none now; at the alpha table, where one
    player plays two characters, a change to the other character goes
    unnoticed until the switch.

## Requirements

Sorted at Task 0 under the answers given, OD-1 A and OD-2 B, and allocated
as "Ids:" lists; the register's rows are the wording that binds. Each row reads `**OPEN — no test yet**` until its task
cites its tests. Every test uses invented keys (For Phase 4a).

| R | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|
| R1 | 2 | `SheetKeyFault` answers, after both sweeps, the first `attributes` key in byte order that a form lists, naming the key and a form that lists it | nil, or before an undeclared key |
| R2 | 2 | With a ruleset served, `add_actor` refuses an actor holding such a key, naming the actor, `attributes` and the key, after its other key refusals, appending nothing | appended |
| R3 | 2 | `adventure.Load` refuses an actor file holding such a key, naming the file, the field and the key | loads |
| R4 | 2 | `vtt serve` with a ruleset refuses a campaign any of whose actors holds such a key, naming the actor | served |
| R5 | 2, 8 | `add_actor`'s `attributes` doc and `get_ruleset`'s description say a value a form lists is refused on a sheet | the sentence gone |
| R6 | 1 | `rules.Load` keeps `places`, each place's key, name, description, forms, copies and states as written, in declared order; none when absent, `null` or `[]` | a place lost, changed or reordered |
| R7 | 1 | `rules.Load` refuses, naming the file and `places[<i>].<field>`, a place whose key, name or description is absent or empty, whose key an earlier place declares, or whose key is longer than 128 bytes, and loads one at 128 | loads past a fault |
| R8 | 1 | `rules.Load` refuses a place whose `forms` is absent or empty or names an undeclared form or one twice, and a state that is empty, repeated or longer than 128 bytes | loads |
| R9 | D2 | A `ruleset.json` holding a field the format does not define, at its top level one matching none of `id`, `name`, `format_version`, `version`, `properties`, `sheet`, `free_usage_name`, `forms` and `places`, or within `sheet`, a form or a place one the format does not define there, compared without regard to case, is refused, naming the file and the field (succeeds VTT-996) | loads |
| R10 | 7, D5 | `rules.Load` refuses, naming the file and the field, a `sheet` holding no page, a page with an empty name, with the name another page holds or with no section, a section with an empty name, a section showing none or more than one of values, conditions, abilities and places, and a section showing an empty list (succeeds VTT-825) | loads |
| R10b | D5 | `rules.Load` refuses a section's `places` naming an undeclared place or one the sheet shows already, naming the file and the field | loads |
| R11 | D5 | `rules.Load` refuses a ruleset that declares a `sheet` and a place no section shows, naming the file and the place | loads |
| R13 | D3 | The schemas describe `places`, every field of a place and a section's `places`, and state the bound on a place key and a state (succeeds VTT-1009) | undescribed or unstated |
| R14 | 3 | Both folds keep an `ActorAdded`'s references whole and refuse one holding an empty reference id | lost, or folded |
| R15 | 5 | Both folds refuse a `ReferenceAdded` with an empty id, with no reference, for an unknown actor, or of an id the sheet holds, in that order, and otherwise add it, changing nothing else | refused when legal, or anything else changed |
| R16 | 5 | Both folds refuse a `ReferenceRemoved` or a `ReferenceChanged` for an unknown actor or an id the sheet does not hold, and otherwise remove it, or set its place and state and keep its entry, changing nothing else | the entry changed, or anything else changed |
| R17 | 3 | The contract round-trips an `Actor` with references and each of the three events | a field lost |
| R18 | 3, 5 | The committed reference log folds to the committed dump in both folds, and both refuse each committed refused envelope | dumps differ |
| R19 | D19 | The Story draws no line for a reference event (R19a, succeeds VTT-809); the Events panel names the actor as it names any actor, and the entry and the place, or their ids where the window holds no name (R19b) | a Story line, or an id where a name is known |
| R19c | 6, D22 | None of the three commands sends a participant a new watchable set (OD-2 B) | a `WatchableChanged` sent |
| R20 | 6 | A party member's reference events reach a log that saw it, and no log is sent a creature's | withheld, or sent |
| R21 | 6 | A character log is introduced to a creature, and to a party member its eyes do not see, with no references | references sent |
| R22 | 6 | A log holding a party member whose references differ from the world's is sent, at the first event its eyes see it, the frames that bring them to the world's, and no `ActorRemoved` or `ActorAdded` for it | lags, or re-introduced |
| R23 | 6 | `perceive.Resume` resumes a fold holding each party member in sight at the world's references, and starts afresh from one that differs | resumes a lagging fold |
| R24 | 6 | A campaign reopened after references are added, changed and removed resumes every log (SPEC-021) | a log starts over |
| R25 | 4 | `SheetReferenceFault` answers, in byte order of ids, the first reference naming an entry the catalogue does not hold, a place the ruleset does not declare, an entry of a form its place does not hold, or a state its place does not name or one where it names none; then the first naming an entry an earlier one names in a place that holds it once | nil past a fault |
| R26 | 4 | With a ruleset served, `add_actor` refuses, after its spend rule, an actor whose references do not fit, naming the actor, `references` and the reference; with one or none, a reference id that is empty; with none, nothing else of them | appended |
| R27 | 4 | `adventure.Load` reads an actor file's `references`, refuses an empty id and then the same faults after its spend rule, naming the file and the reference, and `Compile`'s `ActorAdded` carries them as written | loads, or lost |
| R28 | 4 | `vtt serve` with a ruleset refuses a campaign any of whose actors' references do not fit, naming each actor and its first fault | served |
| R29 | 9 | `toActorAddedDump` writes an actor's references when it holds some, and no field when it holds none | missing |
| R30 | 5 | An `add_reference` from the DM or the agent appends one `ReferenceAdded`; one the fold refuses, and with a ruleset served one R25 refuses, the copies rule against the sheet included, appends nothing | appended |
| R31 | 5 | A `change_reference` from the DM or the agent appends one `ReferenceChanged`; one the fold refuses, and with a ruleset served one whose place or state does not fit the reference's entry, or that the copies rule refuses in its new place, appends nothing | appended |
| R32 | 5 | A `remove_reference` from the DM or the agent appends one `ReferenceRemoved`; the fold's refusals reach the issuer and append nothing | appended |
| R33 | 5 | A spectator is refused each of the three, and so is a player, for an actor the player controls too | appended |
| R34 | D14 | No reference command's append holds a `RulesetNamed` | one written |
| R35 | 8 | Each of the three tools' descriptions says what it changes, what it refuses with and without a ruleset, that no player is sent a message of it and whoever is sent the sheet sees it there, and DM/agent only; `add_actor`'s `references` doc says what it refuses | the sentence gone |
| R36 | 5 | Through the MCP server, an addition, a change and a removal each land, and a refused one appends nothing | not landed |
| R37 | 8 | `GET /api/ruleset` answers every role `places`, each with its key, name, description, forms, copies and states, in declared order, a place's `states` and `places` itself `[]` when none | missing, reordered or `null` |
| R38 | 8 | A `places` section is answered with its place keys | missing |
| R39 | 8 | `get_ruleset`'s description says it answers the places and what a section of places shows | the sentence gone |
| R40 | 7 | The player panel draws a `places` section's references as D17 says, and leaves out one with none | missing, misordered or unnamed |
| R41 | 7 | With no sheet declared, the panel draws after "Abilities" a section per place holding a reference (succeeds VTT-840) | missing |
| R42 | 7 | The DM console adds, changes and removes a reference, asking before a removal, and a "no" sends nothing | sent unasked |
| R45 | 9 | dnd45e-minimal's guide statblocks name the same references as their adventure actors | they differ |
| R46 | 5, D14 | With no ruleset served, an `add_reference` or a `change_reference` is refused for no entry, place, state or copy | refused |

R12 (a form's `charges`), R43 and R44 (the player told) left with OD-1 A and
OD-2 B; their labels are not reused.

**Refused at the sort**, each with what holds it:
- R36, an addition, a change and a removal landing through the MCP server:
  not a rule of its own but R30 to R32's agent half; Task 9's MCP test
  cites those rows.
- A place holds any number of references: R6 to R8 refuse nothing for a
  number, R25's copies half refuses only a second reference to one entry in
  a place that holds it once, and R30's tests add several references to one
  place that holds copies.
- The conformance suites load dnd45e-minimal and goblin-ambush:
  `TestConformanceOverRulesetsGlob` and VTT-802.
- goblin-ambush's actors hold their weapons and armor: its compiled golden,
  through R29.

**Rows a task withdraws or succeeds:** VTT-996 by R9 (Task 2); VTT-1009 by
R13b (Task 2); VTT-825 by R10 (Task 2); VTT-809 by R19a (Task 4); VTT-840
by R41 (Task 10). VTT-996, VTT-825 and VTT-840 become false as worded;
VTT-1009 and VTT-809 stay true and become narrower than their tests, and
are withdrawn in the words of VTT-625's withdrawal ("Narrower than its
tests … Succeeded by"). **Kept as worded, by D6's order:** VTT-617, VTT-618,
VTT-677 and VTT-678, each still naming the first undeclared or other-type
key; VTT-675 and VTT-746, which read nothing with no ruleset served of the
fields they name. SPEC-025's consequence on a property a form lists and
SPEC-027's "No sheet refers to an entry" are prose, rewritten in Task 14.

## The existing tests and data that change

By search; each task reruns its own packages first and names what turned
red.
- **Task 1:** none expected: `git grep -n "a form lists\|form lists" --
  '*_test.go' '*.test.ts'` finds no test that puts such a key on a sheet,
  and `git grep -ln heft -- internal cmd` finds it only in fixtures and
  tests that serve no ruleset or none listing it on a sheet.
- **Task 2:** `qa_declared_sheet_test.go`'s field list (gains `places`);
  `id_bound_internal_test.go` and
  `qa_property_format_test.go` (a row each for a place key and a state);
  `schema_test.go`'s two tables; `testdata/valid/ruleset.json`.
- **Task 3:** `qa_ruleset_route_test.go`'s list of fields answered `[]`
  with nothing loaded, and its comparisons over `qaRRRulesetDirs`, gain
  `places`; `internal/mcp/ruleset_tool_test.go`.
- **Task 4:** `cmd/vtt/tools.json` and `contract/gen`; `contract/testdata`;
  `projection_internal_test.go`'s rulings.
- **Task 6:** `internal/eventgen/model_test.go`'s kind lists.
- **Task 12:** `rulesets/dnd45e-minimal/ruleset.json` and `guide.md`;
  `adventures/goblin-ambush/actors/*.json` and
  `goldens/compiled-batch.json`; `scenarios/goldens/adventure-night`'s
  `state.json` by hand and `stream.json` re-recorded (P10).
- **Unchanged:** every other manifest and inline manifest literal (places
  are optional); `rulesets/tavern-brawl/`; every compiled ability golden;
  the client tests of the default page, whose actors hold no reference
  (`git grep -n '"Values", "Conditions", "Abilities"' -- client/test`).

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** the owner answered OD-1 A and OD-2 B and approved the rest,
  D18's and D21's preferences among it, on 2026-11-25; run the
  `requirements` sort over R1 to R45, R12, R43 and R44 aside, and the
  refusals above; allocate ids with
  `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/requirement-id`;
  write the rows; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and the requirements chain passes.

### Task 1: an item's value is refused on a sheet (R1 to R5; D6; items 2, 8)

- **Files:** `internal/rules/sheet.go` (`SheetKeyFault`'s third sweep,
  `SheetFault`'s doc); tests in `internal/rules` (the sweep's order against
  an undeclared and an other-type key, the form named when two list one
  key), `internal/gateway` (`add_actor` with and without a ruleset),
  `internal/adventure` and `cmd/vtt` (`sheetsFit`); `tools/toolgen/main.go`
  (`attributes`) and `cmd/vtt/tools.json`; `internal/mcp/ruleset_tool.go`
  and its test; `rulesets/dnd45e-minimal/guide.md` (the sentence "never
  set it on a creature" becomes "`add_actor` refuses it");
  `docs/requirements.md`.
- **True when:** the new tests pass and fail on `13eeca0` (P2); `go test
  ./internal/rules/... ./internal/adventure/... ./internal/gateway/
  ./internal/mcp/ ./cmd/vtt/ ./tools/toolgen/` passes.
- **Deliberate breaks:** drop the sweep (R1 to R4 turn red); run it before
  the attributes sweep (the order test turns red); name the last form that
  lists the key (the two-forms test turns red).
- **The state here:** the 5a hole is closed; no sheet refers to anything.

### Task 2: places and the sheet's places section (R6 to R11, R13; D1 to D3, D5; items 1, 7's layout)

- **Files:** `internal/rules/format.go` (`Place`, `Ruleset.Places`,
  `Ruleset.Place`, `SheetSection.Places`);
  `internal/rules/load.go` (`manifestJSON.Places`, `placeJSON`,
  `loadPlaces` after `loadForms`, `checkManifestBounds`, `loadV2`'s copy,
  `checkSheet`'s fourth kind and its every-place check after the
  abilities'); `internal/rules/schema/ruleset.schema.json`;
  `internal/rules/testdata/valid/ruleset.json` (a place with states and
  copies, one without either, both shown by a `places` section); a new
  `internal/rules/places_test.go`; the tests of "The existing tests and
  data that change"; `docs/requirements.md` (VTT-996,
  VTT-1009, VTT-825).
- **True when:** the new tests pass and fail on `13eeca0` (P1);
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` runs a case for each of
  a place's required fields; `go test ./internal/rules/...
  ./internal/adventure/... ./internal/gateway/` passes.
- **Deliberate breaks:** drop each refusal in turn; drop the bound from
  `checkManifestBounds` (the 129-byte cases turn red) and from the schema
  (the VTT-1009 probes); drop `loadV2`'s copy (R6); count `places` out of
  the section's kinds (R10); drop the every-place check (R11).
- **The state here:** a ruleset may declare places; nothing serves them.

### Task 3: the read surface (R37 to R39; D16; item 8)

- **Files:** `internal/gateway/metadata.go` (`placeJSON`, `rulesetJSON`'s
  `places`, `sheetSectionJSON`'s `places`, kind
  `places`); a new `internal/gateway/places_route_test.go` (every role,
  declared order, `[]` with none and with no ruleset, `states: []`);
  `internal/mcp/ruleset_tool.go` and its test; `client/src/metadata.ts`
  (`PlaceDef` and `RulesetMeta.places`; `SheetSection`'s `places` member
  waits for Task 10, P7).
- **True when:** the new tests pass and fail on `13eeca0`; `go test
  ./internal/gateway/ ./internal/mcp/ ./cmd/vtt/` and `task client:typecheck`
  pass; `task build:client` leaves `cmd/vtt/webdist` unchanged, or the
  bundle is committed.
- **Deliberate breaks:** start `places` nil (the `[]` case); answer the DM
  alone (the every-role case); drop the sentence (R39).
- **The state here:** places are read; no sheet holds a reference.

### Task 4: the contract and the server's fold (R14 to R17, R19a, R20; D7, D8, D10; items 3, 6's forwarding)

- **Files:** `contract/vtt/v1/events.proto` (`Reference`,
  `Actor.references`, the three events, payload 41 to 43), regenerated
  into `contract/gen` and `cmd/vtt/tools.json`; `contract/roundtrip_test.go`
  and `contract/testdata` fixtures; `internal/engine/apply.go` (the
  `ActorAdded` check and three arms) and a new `internal/engine` test;
  `internal/perceive/projection.go` (`classify`'s sheet arm, `sheetActor`)
  and `projection_internal_test.go`, with tests that a party member's events
  reach a log that saw it and a creature's reach none; `tools/toolgen/main.go`
  (`add_actor`'s `references` doc, saying what it is, and
  `vtt.v1.Reference`'s override); `client/src/view/feed.ts` (`STORY`, each
  false), `client/src/view/spectator.ts` (`describe`'s three arms, naming
  the actor and the reference id; P7, D19) and `cmd/vtt/webdist`;
  `docs/requirements.md` (VTT-809).
- **True when:** the new tests pass; `git diff main -- contract/vtt` adds
  and changes nothing else; `TestEveryEnvelopePayloadArmHasAnExplicitRuling`
  and `client:typecheck` pass; `task check:breaking` reports nothing
  breaking.
- **Deliberate breaks:** drop each fold refusal in turn; write the map in
  place (the aliasing test turns red); drop `now.party` from the arm (the
  creature test).
- **The state here:** nothing writes the events; an `add_actor` with
  references is read against nothing until Task 7, and the client fold
  skips the events until Task 5 (P7). No log holds either yet.

### Task 5: the client's fold (R14 to R18, R19b; D10, D11, D19; item 3)

- **Files:** `client/src/state.ts` (`Actor.references`), `client/src/fold.ts`
  (`copyActor`, the three arms, `actorJSON`); `contract/testdata/reference_change_log.json`,
  `reference_change_dump.json` and `reference_change_refused.json`; a new
  `cmd/vtt/reference_change_dump_test.go` and `client/test` fold test;
  `client/src/view/naming.ts` (`Namer`'s entry and place names, from the
  log for a removal's and a change's entry and place (D19), and from the
  window's catalogue and places), `client/src/view/spectator.ts`
  (`describe`) and `client/src/app.ts` (handing the namer both lists from
  `RulesetMeta`); `cmd/vtt/webdist`.
- **True when:** both folds give the committed dump and refuse each refused
  envelope in the same words' order; the Events lines name the entry and
  the place; `client:typecheck` passes.
- **Deliberate breaks:** drop `references` from `copyActor` (the dump
  differs); drop a
  refusal (the refused fixture folds in one fold only).
- **The state here:** both folds keep references; nothing writes them.

### Task 6: the projection's corrections (R21 to R24; D12; item 6)

- **Files:** `internal/perceive/projection.go` (`actorBelief`, `beliefOf`,
  `statusFrames`, `kindMovesList`, and the docs that name their fields);
  new `internal/perceive` tests: a creature, and an unseen party member,
  introduced with none; a log that held a party member unseen is sent, at
  first sight, the removals, additions and changes that bring it to the
  world's references, and no re-introduction; a reference removed and added
  back as another entry under one id while unseen; `Resume` from a fold at
  the world's references and from one that lags; a creature holding only
  references made a party member; `internal/campaign`: a campaign reopened
  after references change resumes every log; `internal/eventgen/model.go`
  (draws of the three events, each with its refusal) and `model_test.go`.
- **True when:** the new tests pass and every other `internal/perceive` and
  `internal/campaign` test passes unchanged; `TestEveryCharacterLogFoldsAtEveryPrefix`
  and `TestACharacterLogIsWhatItsCharactersEyesAreGiven` pass with the new
  draws.
- **Deliberate breaks:** compare references in `sameIncarnation` (the
  no-re-introduction tests); drop them from `statusFrames` (the first-sight
  and `Resume` tests); compare a reference's entry only (the change test);
  drop them from `kindMovesList` (the creature-made-party test).
- **The state here:** a log is sent references as it is the sheet's other
  values; nothing writes them.

### Task 7: where a sheet enters, served (R25, R26, R28, R35's `add_actor` half; D13; item 4)

- **Files:** `internal/rules/sheet.go` (`ReferenceFault`,
  `SheetReferenceFault`) and a new `internal/rules` test;
  `internal/gateway/add_actor_validate.go` (`sheetFault`) and tests;
  `cmd/vtt/serve_compose.go` (`sheetsFit`) and tests;
  `tools/toolgen/main.go` (`references`' refusals) and `cmd/vtt/tools.json`.
- **True when:** each refusal of D13 is named by its own test, with the
  ruleset served and, for the empty id, without; with none, a reference of
  any other kind is appended; the order after the spend rule is tested.
- **Deliberate breaks:** drop each check of `ReferenceFault`; compare the
  copies rule across places (a test of one entry in two places turns red);
  read references before the spend rule (the order test).
- **The state here:** a sheet's references fit where it enters by command
  and at boot; an adventure carries none yet.

### Task 8: an adventure's sheets (R27, R29; D13; items 4, 9's golden)

- **Files:** `internal/adventure/load.go` (`actorJSON.References`,
  `loadActors` after the spend rule), `format.go` (`AdventureActor`),
  `compile.go` (`buildActor`), `internal/adventure/conformance/conformance.go`
  (`actorAddedDump`, `toActorAddedDump`); tests in `internal/adventure` and
  its conformance package.
- **True when:** an actor file's references load and compile as written; an
  empty id and each R25 fault are refused, naming the file and the
  reference; every golden is unchanged
  (`git diff --stat` names no `goldens/`).
- **Deliberate breaks:** drop the copy in `buildActor` (the compile test);
  dump references always (an unchanged golden moves, and the dump of an
  actor holding none turns red).
- **The state here:** an adventure may declare references; none does.

### Task 9: the commands in play (R19c, R30 to R35, R46; D8, D9, D14, D15, D22; item 5)

- **Files:** `contract/vtt/v1/commands.proto` (`AddReference`,
  `RemoveReference`, `ChangeReference`, command 39 to 41), regenerated;
  `contract/roundtrip_test.go` and fixtures; `internal/gateway/convert.go`
  (`ToEvent`), `authz.go` (`commandRoles`, `commandName`), `server.go`
  (`validateForm`, the dispatch), a new `internal/gateway/reference_change.go`
  and tests (each refusal with a ruleset, the copies rule on an addition and
  on a move, none read with no ruleset (R46), the player and spectator
  refused, no `RulesetNamed`, and no new `WatchableChanged` sent to a player
  controlling the actor after each of the three, R19c); `tools/toolgen/main.go` (three entries) and
  `cmd/vtt/tools.json`; a new `cmd/vtt` MCP e2e test, as
  `mcp_ability_change_e2e_test.go` is; `client/src/commands.ts` (three
  builders) and its test.
- **True when:** the new tests pass and fail on `13eeca0`;
  `TestEveryClientCommandConverts`, `TestEveryClientCommandHasRoleCells`,
  `TestManifestCoversAllCommandMessages` and `TestEveryManifestCommandIsRegisteredAsATool`
  pass.
- **Deliberate breaks:** drop each refusal; give the player a cell (R33);
  read the change against the old place (the move test); compare a change
  with the reference itself under the copies rule (a change kept in a place
  that holds an entry once turns red, R31); write a `RulesetNamed` (R34);
  copy a sheet's references into `WatchableActor`'s ability ids or send the
  set again after a change (R19c).
- **The state here:** the DM and the agent change references by tool.

### Task 10: the panel and the console (R40 to R42; D17, D18; item 7)

- **Files:** `client/src/metadata.ts` (`SheetSection`'s `places` member),
  `client/src/player.ts` (`sheetFor`, `ShownSection`'s `places` kind),
  `client/src/view/player.ts` (`PanelLayout`, drawing the section),
  `client/src/view/dm.ts` (the group), `client/src/app.ts` (handing the
  catalogue and places to the panel and the console); client tests;
  `cmd/vtt/webdist`;
  `docs/requirements.md` (VTT-840).
- **True when:** the new tests pass and fail on `13eeca0`; `bun test`,
  measured with `env -u CLAUDECODE`, and `client:typecheck` pass;
  `tools/ts_mutation_inputs_test.py` passes.
- **Deliberate breaks:** order by entry name (the order test); show a
  section with none (VTT-839's case); send a removal unasked (R42).
- **The state here:** the table sees and the DM changes what a sheet
  carries.

### Task 11: withdrawn

OD-2 B: the player is not told (D22). The number stays, so the tasks after
it keep theirs.

### Task 12: the content (R45; D21; item 9)

- **Files:** `rulesets/dnd45e-minimal/ruleset.json` (the places and the
  section) and `guide.md`; `adventures/goblin-ambush/actors/*.json`;
  `adventures/goblin-ambush/goldens/compiled-batch.json`;
  `scenarios/goldens/adventure-night/state.json` (by hand, from the
  scenario) and `stream.json` (recorded); a reference twin of
  `TestQADataGuideStatblocksNameAbilities` in
  `internal/adventure/conformance/qa_data_abilities_test.go`.
- **True when:** `TestConformanceOverRulesetsGlob`, the adventure
  conformance suite, `TestFoldGoldenCorpus`,
  `TestScenarioGoldenStreamsHaveNotDrifted` and the new guide check pass;
  the compiled golden names each actor's references; no ability golden
  moves.
- **Deliberate breaks:** a statblock with another reference (R45); a
  golden left unmoved (the conformance suite).

### Task 13: the gate (item 10)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger`
  once, last, if a file's comment share fell (SPEC-010); `go clean
  -cache`; check free space and the load; `task check` whole on the last
  code commit, detached under `/bin/sh`, output to a file, requiring its
  completion line; `task e2e`, whose `table.spec.ts` reads the shipped
  sheet (P10). Tell the owner whether the client's stored mutation report is
  out of date and which commit it measured (SPEC-023).
- **True when:** `task check` and `task e2e` are green.

### Task 14: the report, then the specifications

- **The report:** `docs/reports/2026-11-25-a-sheet-refers-to-the-catalogue.md`
  with the `implementation-report` skill: each Done item with its
  observation; each R and what turned it red; rule 9 and G1 to G12; each
  OD's answer; Phase 4a's adjudications under their own heading; the gate.
- **The specifications**, with the `specification` skill, from the report:
  - **SPEC-028 (new):** places and their refusals; a reference, its id, its
    entry, place and state; where a sheet enters and in what order;
    the three commands and events and their refusals; the copies rule; two
    writers at once (below); principles 23, 25, 26, 27, 34, 35.
  - **SPEC-025:** the item-value refusal and its order; references read
    after the spend rule; its consequence on a property a form lists.
  - **SPEC-027:** "No sheet refers to an entry" and its searches.
  - **SPEC-024:** `places` in the manifest and its field list; the
    section's fourth kind.
  - **SPEC-012:** `places` and the section kind; `get_ruleset`'s
    sentences.
  - **SPEC-013:** the three commands' cells, `validateAddReference` and the
    handler in `handleCommand`'s order.
  - **SPEC-016:** references in what an event can change of an actor and
    in `statusFrames`.
  - **SPEC-018:** the rows for a reference's fields, and a place key and a
    state among the ids the loader bounds.
  - **SPEC-021:** a reopen after reference changes.
  - **SPEC-022:** the fold's arms, the panel's section and default page,
    the console.
  - **SPEC-007:** the messages, the tools.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand each task's QA agent the task's rows, SPEC-028 as drafted, and the
surfaces as signatures and names, never the source: `go doc ./internal/rules`
(`Load`, `Ruleset`, `Place`, `Form`, `SheetFault`, `SheetKeyFault`,
`ReferenceFault`, `SheetReferenceFault`), `go doc` of `vttv1.Reference` and
the three events and commands, the schema, `/api/ruleset`'s shape as
SPEC-012 states it, and the tools' descriptions from `cmd/vtt/tools.json`.
Tell it: never run `task check`, `task e2e`, Stryker or gremlins; its files
are new `qa_` files; semgrep bans game words in Go under `internal/` and
`cmd/`, tests included; go-arch-lint holds the layer imports; the tree is
hashed before and after it. Use invented keys only: forms `zulu` and
`yankee`; number properties `quebec` and `romeo`, which they
list, and `sierra`, which none does; places `mike` (both forms, copies,
states `lima` and `oscar`) and `kilo` (`yankee` only, once, no state);
entries `echo` (`zulu`) and `delta` (`yankee`). Known and not findings: an
adventure file writing one reference id twice keeps the last, as it does an
attribute (P3); a `values` section naming a key a form lists loads and shows
nothing (G12); an expression reading such a key fails at use, as for any key
a sheet does not hold (D6); two additions of one entry to a place that holds
it once, each judged before the other lands, can both append, and the next
`vtt serve --ruleset` refuses the campaign, by the 2026-10-21 ruling with no
code.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "never", "whatever") names the search that bounds it, P5
and P6 above all; check each comment under `internal/`, `cmd/` and
`client/src` is a warning or a pointer; check that `git grep -n -i -E
'weapon|armou?r' -- internal cmd ':!*_test.go'` prints nothing; check every
refusal a sheet or a ruleset meets today keeps its message and its order
(D3, D5, D6, D13); check both folds' refusals agree in order; check that
`git diff main -- contract/vtt` only adds; check `cmd/vtt/webdist` was
rebuilt with each client change.
