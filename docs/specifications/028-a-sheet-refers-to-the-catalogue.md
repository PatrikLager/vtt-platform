# SPEC-028: A sheet refers to catalogue entries, each in a place its ruleset declares

## Status

Accepted. Implemented by `internal/rules/format.go` (`Place`,
`Ruleset.Places`, `Ruleset.Place`), `internal/rules/load.go`
(`manifestJSON.Places`, `placeJSON`, `loadManifest`, `loadPlaces`,
`checkManifestBounds`) and the schema
`internal/rules/schema/ruleset.schema.json`; `internal/rules/sheet.go`
(`SheetFault`, `Ruleset.ReferenceFault`, `Ruleset.SheetReferenceFault`,
`copiesFault`, `Ruleset.CopyFault`, `copyFault`, `referenceFault`);
`contract/vtt/v1/events.proto` (`Actor.references`, `Reference`,
`ReferenceAdded`, `ReferenceRemoved`, `ReferenceChanged`) and
`contract/vtt/v1/commands.proto` (`AddReference`, `RemoveReference`,
`ChangeReference`), numbered as SPEC-007 says; `internal/engine/apply.go`
(`Apply`'s `ActorAdded`, `ReferenceAdded`, `ReferenceRemoved` and
`ReferenceChanged` arms); `internal/gateway/add_actor_validate.go`
(`sheetFault`), `internal/gateway/reference_change.go`
(`validateAddReference`, `handleReferenceChange`, `referenceSheetFault`,
`referenceChangeFault`), `internal/gateway/server.go` (`validateForm`,
`handleSheetChange`), `internal/gateway/authz.go` (`commandRoles`,
`referenceCommandName`) and `internal/gateway/convert.go` (`ToEvent`);
`internal/adventure/load.go` (`actorJSON`, `referenceJSON`, `loadActors`,
`actorReferences`, `wireReferences`), `internal/adventure/format.go`
(`AdventureActor`, `Reference`), `internal/adventure/compile.go`
(`buildActor`) and `internal/adventure/conformance/conformance.go`
(`toActorAddedDump`, `referenceDump`); `cmd/vtt/serve_compose.go`
(`sheetsFit`); and `tools/toolgen/main.go` (`referenceOverride`, the
`add_reference`, `change_reference` and `remove_reference` entries and
`add_actor`'s `references`). `rulesets/dnd45e-minimal` declares places, and
the actors of `adventures/goblin-ambush` hold references to its entries. The
places are served by `handleRuleset` (SPEC-012), a party member's references
reach character logs through the projection (SPEC-016), and the browser client
folds, draws and changes them (SPEC-022). Pinned by
`internal/rules/places_test.go`, `internal/rules/qa_places_test.go`,
`internal/rules/declared_sheet_test.go`,
`internal/rules/qa_declared_sheet_test.go`,
`internal/rules/sheet_reference_test.go`,
`internal/rules/qa_reference_fit_test.go`,
`internal/engine/reference_change_test.go`,
`internal/engine/qa_reference_fold_test.go`, `contract/roundtrip_test.go`,
`contract/qa_reference_roundtrip_test.go`,
`cmd/vtt/reference_change_dump_test.go`,
`client/test/fold-references.test.ts`,
`client/test/qa-reference-fold.test.ts`,
`internal/gateway/add_actor_references_internal_test.go`,
`internal/gateway/qa_add_actor_references_test.go`,
`internal/gateway/reference_change_internal_test.go`,
`internal/gateway/reference_change_test.go`,
`internal/gateway/qa_reference_commands_test.go`,
`cmd/vtt/mcp_reference_change_e2e_test.go`,
`internal/adventure/actor_references_test.go`,
`internal/adventure/qa_actor_references_test.go`,
`internal/adventure/conformance/references_dump_test.go`,
`internal/adventure/conformance/qa_references_dump_test.go`,
`internal/adventure/conformance/qa_shipped_references_test.go`,
`internal/adventure/conformance/qa_data_abilities_test.go`'s
`TestQADataGuideStatblocksNameReferences`,
`cmd/vtt/campaign_reference_fit_test.go` and `tools/toolgen/main_test.go`.

## Principles served

Principles 11, 13, 23, 25, 26, 27, 34 and 35 of `docs/blueprint.md`:

- **11.** Both folds refuse the same reference events in the same order and
  hold the same references after the rest, and the committed log of
  reference events folds in each to the one committed dump (VTT-1028).
- **13.** Every place has a display name and a description, so whoever is
  shown one can be told it in the table's words.
- **23.** A place is the ruleset's: `internal/rules` knows one only as a key
  holding references to entries of the forms it lists, once each or in
  copies, in the states it names, so `wielded`, `worn` and `carried` are
  `rulesets/dnd45e-minimal`'s words: no non-test Go under `internal/` or
  `cmd/` names a weapon, an armour, `wielded` or `worn` (`git grep -n -i -E
  'weapon|armou?r|wield|worn' -- internal cmd ':!*_test.go'` prints
  nothing), nor holds `"carried"` (`git grep -n '"carried"' -- internal cmd
  ':!*_test.go'` prints nothing).
- **25.** A place with no key, name, description or form, a key another
  place declares, a form the ruleset does not declare, a form or a state it
  lists twice, an empty state, and a key or a state past the bound on an
  id, are refused when the ruleset loads, with the file and the field
  named.
- **26.** A reference naming an entry the catalogue does not hold, a place
  the ruleset does not declare, an entry of a form its place does not hold
  or a state its place does not allow, and a second reference to one entry
  in a place that holds an entry once, are refused where a sheet enters, by
  `add_actor` with a ruleset served, by `adventure.Load` and, for a
  campaign holding one, by `vtt serve --ruleset`, and where it changes, by
  `add_reference` and `change_reference` with a ruleset served; each
  refusal names the reference and the actor or its file.
- **27.** A reference names its entry by id and holds nothing else of it,
  and a sheet holds no value a catalogue form lists (SPEC-025), so a
  correction to an entry reaches every sheet that refers to it at the
  ruleset's next load.
- **34.** The descriptions of `add_reference`, `change_reference` and
  `remove_reference` say what each changes and what it refuses with a
  ruleset served and with none, that no player is sent a message of the
  change and that whoever is sent the sheet sees it there, and that only
  the DM and the agent may use it, and `add_actor`'s description of
  `references` says what it refuses (VTT-1051, SPEC-007).
- **35.** Every place is served as data, with its description, to every
  role, and a sheet section of places names the places it shows (SPEC-012).

## How it works

**A place.** `ruleset.json` may hold `places`, each a `key`, a `name`, a
`description` and `forms`, the keys of the forms whose entries the place
holds, and, optionally, `copies`, whether it holds one entry more than
once, absent meaning once, and `states`, the states a reference in it may
be in, one at a time. `loadManifest` reads them through `loadPlaces` once
the forms have loaded, and `rules.Load` keeps them as `Ruleset.Places`,
each a `rules.Place` as written, in declared order, and nil when `places`
is absent, `null` or `[]` (VTT-1015); `Ruleset.Place(key)` returns the
place declared under a key, or nil. A place's `Forms` keep the order they
are written in, and its `States` are nil when it names none, `states`
absent, `null` or `[]` alike. Naming `ruleset.json` and the field as
`places[<i>].<field>`, `loadPlaces` refuses, place by place: a `key`,
`name` or `description` that is empty, absent and `null` read as empty, in
that order; a key an earlier place declares, as `duplicate place key
"<key>"` (VTT-1016); a `forms` that is empty, absent and `null` read as
empty, as `place "<key>": must name a form`; then, as
`places[<i>].forms[<j>]`, a key no form declares, as `names "<key>", which
no form declares`, and one the place lists already; then, as
`places[<i>].states[<j>]`, an empty state and one the place lists already
(VTT-1017). Before any property is read, and after the form keys' bound,
`checkManifestBounds` refuses, place by place, a key and then a state
longer than `maxIDBytes` (VTT-1016, VTT-1017, SPEC-018); `loadPlaces`'
refusals come within the manifest's, after the forms', so before any
condition, atom, ability, sheet or catalogue refusal. Nothing else is
checked of a key or a state: their characters are free, keys and states
are compared byte for byte, and a place key may equal a property's or a
form's key or an entry's, an ability's or a condition's id, since no
expression reads it and `checkOneNamespace` reads entries, abilities and
conditions alone (SPEC-027). The strict decode refuses a field a place
does not define (VTT-1018, SPEC-024). `Place` has no field bounding how
many references a place holds, and no refusal below bounds how many a
sheet or a place holds: the copies rule refuses a second reference to one
entry, whatever the number of others.

**A sheet's section of places.** A declared `sheet` shows a sheet's
references in sections of the kind `places`, each listing place keys, and
with a sheet declared each place the ruleset declares stands in exactly
one section (SPEC-024). How the browser client draws such a section is
SPEC-022's.

**The schema.** `ruleset.schema.json` describes `places`, every field of a
place with `key`, `name`, `description` and `forms` required, and a sheet
section's `places` (VTT-1021); the descriptions of a place's key and of its
states state the bound on an id (VTT-1022, SPEC-018). It types `places` and
a place's `states` as arrays, so it admits no `null` there, which the
loader reads as none.

**A reference.** `vtt.v1.Reference` holds `entry_id`, the id of a catalogue
entry, `place`, the key of the place that holds it, and `state`, its state
there, empty for none. `Actor.references` (`references` in JSON; its
field number is SPEC-007's) maps a reference's id to its `Reference`. The id is
the sheet's own, written by whoever adds the reference, the DM, the agent
or an adventure's author, so two sheets may hold one id
(`TestQARC2TwoSheetsMayUseOneReferenceId`), and two references to one
entry are told apart by theirs. A reference holds nothing of its entry but
its id. `protojson` refuses an `Actor` whose `references` writes one id
twice (`TestQARFOneReferenceIdWrittenTwiceIsRefused`), and the contract
round-trips an `Actor` holding references and each reference event, every
field kept (VTT-1027).

**A reference against its ruleset.** `Ruleset.ReferenceFault(id, r)` reads
one reference, `id` being its id, and answers its first fault as a
`SheetFault` with the field `references`, `id` as its key and a reason
opening `reference "<id>" `, or nil (VTT-1037): an entry the catalogue
does not hold, `names entry "<entry>", which the catalogue does not hold`;
a place the ruleset does not declare, `is in place "<place>", which the
ruleset does not declare`; an entry whose form the place does not list,
`names entry "<entry>" of form "<form>", which place "<place>" does not
hold`; then its state: none where the place names states, `names no
state, and place "<place>" names its states`, one where it names none, `is
in state "<state>", and place "<place>" names none`, and one the place
does not name, `is in state "<state>", which place "<place>" does not
name`. A reference left out reads as one naming the entry `""`, which no
catalogue holds, since the loader refuses an entry whose id is empty
(SPEC-027). `Ruleset.SheetReferenceFault(refs)` asks `ReferenceFault` of
every reference in byte order of their ids and answers the first fault;
when every reference fits, it applies the copies rule in the same order,
and answers the first reference naming an entry an earlier one names in
the same place, when that place holds an entry once, with the reason
`reference "<id>" names entry "<entry>", which reference "<earlier>" holds
in place "<place>" already, and that place holds an entry once`; and
otherwise nil (VTT-1038). The rule reads a place and an entry together, so
one entry may be held once in each of two places, and a place with
`copies` holds one entry under any number of ids.
`Ruleset.CopyFault(refs, id, r)` asks the copies rule of `r`, under `id`,
against the other references of `refs`: for a place that holds an entry
once, it answers the first id but `id`, in byte order, holding `r`'s entry
in `r`'s place, with the same reason naming `id`, and nil for a place that
holds copies or that the ruleset does not declare. It names the reference
asked about whatever the order of the ids, which `SheetReferenceFault`
over the sheet with that reference added does not
(`TestCopyFaultNamesTheReferenceAskedAboutAgainstTheOthersAlone`). `git
grep -n -E '(SheetReference|Reference|Copy)Fault\(' -- '*.go'
':!*_test.go'` finds, beside the definitions and `SheetReferenceFault`'s
own call, the callers below and no other.

**Where a sheet enters with references.** Each reads a sheet's references
after the spend rule, so a sheet is refused first for any fault SPEC-025
reads, and `add_actor`, with a ruleset served or none, and
`adventure.Load` refuse an empty reference id before any reference is
read against a ruleset:
- `handleCommand`, for an `add_actor`, in `sheetFault`, for an actor with
  an id: after the spend rule, or with no ruleset served after an empty
  ability id, an actor holding a reference whose id is empty, as `gateway:
  add_actor: actor "<id>": references: must not hold an empty id`
  (VTT-1040); then, with a ruleset served, `SheetReferenceFault`, as
  `gateway: add_actor: actor "<id>": references: <reason>` (VTT-1039,
  VTT-1070). With none served, no reference is read for its entry, its
  place or its state (VTT-1041). A refused `add_actor` appends nothing
  (SPEC-013).
- `adventure.Load`, through `loadActors`: `actorJSON` decodes an actor
  file's `references` as a map from a reference's id to a pointer to a
  `referenceJSON`, with `entry_id`, `place` and `state`, so a reference
  written `null` is seen; `references` absent or `null` holds none, a
  field of a reference written `null` reads as left out, and the strict
  decode refuses a field a reference does not define. After the spend
  rule, `actorReferences` refuses an empty id, as `adventure: <path>: field
  "references": must not hold an empty id`, then the first reference in
  byte order of ids written `null`, as `adventure: <path>: field
  "references": reference "<id>" must be an object giving its entry_id and
  place, not null` (VTT-1073); then `SheetReferenceFault`, as `adventure:
  <path>: field "references": <reason>` (VTT-1043, VTT-1072). A file that
  writes one id twice holds the last, as for an attribute's key, since
  `encoding/json` keeps the last of a repeated key. `buildActor` copies
  each reference into a fresh `vtt.v1.Reference` through `wireReferences`,
  so the `ActorAdded` `Compile` gives for the actor holds them as written
  (VTT-1042), and `toActorAddedDump` writes them under `references`, each
  with its `entry_id`, its `place` and its `state` when it has one, only
  for an actor holding some, so an adventure's compiled-batch golden pins
  each actor's references and an actor holding none writes no field there
  (VTT-1045).
- `composeServer`, through `sheetsFit`, when it is handed a ruleset: after
  the spend rule, `SheetReferenceFault`, its fault joining the refusal of
  the boot SPEC-025 states as `actor "<id>": references: <reason>`
  (VTT-1044, VTT-1071). It reads no empty id apart: both folds refuse an
  `ActorAdded` holding one and a `ReferenceAdded` naming one (below), so no
  campaign that opens holds one.

**The commands.** `add_reference`, `remove_reference` and
`change_reference`: `AddReference` carries `actor_id`, `reference_id` and
a `Reference`, `RemoveReference` `actor_id` and `reference_id`, and
`ChangeReference` `actor_id`, `reference_id`, `place` and `state`. A
change sends the reference's place and state whole and keeps its entry; to
name another entry is to remove the reference and add one. `ToEvent`
writes each as one event of the same fields, `ReferenceAdded`,
`ReferenceRemoved` and `ReferenceChanged`. `commandRoles` gives the three
to the DM and the agent alone, so a player's and a spectator's are refused
by the role table, a player's for an actor they control too, and
`playerRules` has no entry for them (VTT-1049). In `handleCommand`'s order
(SPEC-013):
- `validateForm` reads an `add_reference` through `validateAddReference`:
  an empty reference id, with a ruleset served or none, as `gateway:
  add_reference: actor "<actor>": reference_id: must not be empty`; then,
  with a ruleset served, `ReferenceFault` of its reference, as `gateway:
  add_reference: actor "<actor>": references: <reason>`, whether or not
  the campaign holds the actor. It reads no `change_reference` or
  `remove_reference`.
- `handleSheetChange` dispatches an `add_reference` and a
  `change_reference` to `handleReferenceChange`, which asks
  `referenceSheetFault` of the actor's sheet in the snapshot
  `authorize` took: for an addition whose id that sheet does not hold,
  `CopyFault` against the sheet's references; for a change of a reference
  the sheet holds, `ReferenceFault` of the reference as it will be, its
  entry from the sheet and its place and state from the command, and then
  `CopyFault` against the sheet's other references, as `gateway:
  <command>: actor "<actor>": references: <reason>`. An actor the snapshot
  does not hold, an addition's id the sheet holds already and a change's
  id it does not hold are left to the fold, and with no ruleset served
  nothing is read. It then appends the one event `ToEvent` makes with
  `campaign.Append`.
- A `remove_reference` goes through `ToEvent` and `campaign.Append` alone:
  `validateForm` has no arm for it and `handleSheetChange` does not
  dispatch it, so it is read against no ruleset.

The fold then refuses, for an addition, a reference left out, an actor the
state does not hold and an id the sheet holds already, and for a change or
a removal, an actor the state does not hold and an id the sheet does not
hold (below). A refused command reaches its issuer and appends nothing
(VTT-1046, VTT-1047, VTT-1048, SPEC-013); an accepted one appends its one
event, whose sequence its result carries, and no `RulesetNamed`, whatever
the log names, since what each writes is the DM's (VTT-1050, SPEC-026).
With no ruleset served, an addition and a change are refused neither for
an entry, a place or a state nor by the copies rule (VTT-1061), so a
change then sets whatever place and state it sends. The handler reads the
new or changed reference, and the copies rule against the others, and
never the fit of the sheet's other references, so a sheet holding a
reference out of fit refuses no change of another
(`TestQARC2ASheetOutOfFitRefusesNoOtherReference`), and a change that
keeps a reference in a place that holds an entry once is not read against
itself (`TestAChangeKeptInAPlaceHoldingAnEntryOnceIsNotReadAgainstItself`).

**Both folds.** `engine.Apply`'s `ActorAdded` arm keeps an actor's
references whole, and refuses, after its attribute names, an actor holding
a reference whose id is empty, as `engine: actor_added: actor "<id>" holds
a reference with an empty id` (VTT-1023). Its `ReferenceAdded` arm refuses
an empty reference id (`engine: reference_added requires a reference
id`), then an event carrying no reference (`engine: reference_added
requires a reference`), then an actor the state does not hold (`engine:
reference_added names unknown actor "<actor>"`), then an id the actor's
sheet holds already (`engine: actor "<actor>" already holds reference
"<id>"`), and otherwise writes the actor's references as a new map holding
a new `Reference` of the event's (VTT-1024). Its `ReferenceRemoved` arm
refuses an actor the state does not hold (`engine: reference_removed names
unknown actor "<actor>"`), then an id the sheet does not hold (`engine:
actor "<actor>" does not hold reference "<id>"`), and otherwise writes a
new map without it (VTT-1025). Its `ReferenceChanged` arm refuses as the
removal does, naming itself in the first (`engine: reference_changed names
unknown actor "<actor>"`), and otherwise writes a new map holding a new
`Reference` of the held entry and the event's place and state, whatever
they are (VTT-1026). No arm changes anything else of the actor or of the
state, bounds a field's length (SPEC-018) or reads a ruleset, which
`internal/engine` does not import, and the client's fold refuses and
changes what this one does (SPEC-022). The committed
`contract/testdata/reference_change_log.json` folds in both to
`contract/testdata/reference_change_dump.json`, and both refuse each
envelope of `contract/testdata/reference_change_refused.json` (VTT-1028).

**What a participant is told.** Nothing tells a player or a spectator
that a sheet's references changed but the sheet: no reference command
sends a participant a new watchable set (VTT-1031, SPEC-015), and the
Story draws no line for a reference event, while the DM's and the agent's
Events panel words each (SPEC-022). Which character logs are sent a
party member's reference events, and its references at first sight and at
a resume, is SPEC-016's and SPEC-021's, and how the player's panel, the
Events panel and the DM console show and change references SPEC-022's.

**Two writers at once.** `handleReferenceChange` reads the snapshot
`authorize` took, and the fold reads no ruleset, so the copies rule holds a
sheet only as it stood when each command was judged: two commands that
each bring one entry into a place that holds it once, each judged before
the other lands, both append. The next `vtt serve --ruleset` refuses the
campaign, naming the actor and the later of the two references in byte
order of ids (VTT-1071), and a `remove_reference` of either, which reads no
ruleset, brings the sheet back into fit. By the owner's ruling of
2026-10-21 on a near-theoretical edge case, this has no code, no test and
no debt entry.

**The shipped content.** `rulesets/dnd45e-minimal` declares the places
`wielded`, of the form `weapon`, holding copies, `worn`, of `armor`,
holding each entry once, and `carried`, of both, holding copies, none
naming a state, and the section "Equipment" of its page "Character" shows
them (`TestQASRTheRulesetDeclaresD21sThreePlaces`,
`TestQASRTheSheetShowsThePlacesInOneSectionAfterItsPage`);
`rulesets/tavern-brawl` declares no place. Each actor of
`adventures/goblin-ambush` holds references to dnd45e-minimal's weapons
and armor in those places, each under its entry's id, and every one fits
(`TestQASRTheAdventureLoadsAndEveryReferenceFits`); the actors of
`adventures/cellar-rats` hold none. `scenarios/goldens/adventure-night`,
whose scenario loads goblin-ambush, carries them in its recorded
`ActorAdded` entries
(`TestQASRTheScenarioGoldensAgreeWithCompileAndTheFold`). The statblocks
of `rulesets/dnd45e-minimal/guide.md` name the same references as the
actor files they describe (VTT-1059), and the guide says how to add,
change and remove one
(`TestQASRTheGuideSaysHowToAddChangeAndRemoveAReference`).

## Consequences

- No ability reads a reference, so where a reference is decides no roll:
  `rules.Resolve` reads none (`git grep -n -E
  'GetReferences\(\)|\.References([^A-Za-z]|$)' -- internal/rules
  ':!*_test.go'` prints nothing), and an entry's values decide no outcome
  (SPEC-027).
- An actor's references arrive with its `ActorAdded` and change by the
  reference events alone: `git grep -n -E '\.References *=' --
  internal/engine ':!*_test.go'` finds the `ReferenceAdded`,
  `ReferenceRemoved` and `ReferenceChanged` arms and no other. In the world
  those events come from the DM's and the agent's commands: `git grep -n -E
  'Envelope_Reference(Added|Removed|Changed)\{' -- '*.go' ':!*_test.go'
  ':!contract/gen'` finds `ToEvent`'s, the projection's correction of a
  character log (SPEC-016) and `internal/eventgen`'s draws for property
  tests. No player changes what a sheet refers to.
- A reference holds only its entry's id, so a correction to an entry
  reaches every sheet that refers to it at the ruleset's next load, and an
  entry, a place or a state the ruleset no longer declares leaves the
  sheets that refer to it out of fit: the next `vtt serve --ruleset`
  refuses the campaign, naming each such actor (VTT-1044). The DM brings
  it up to date by serving it with a ruleset its references fit, or with
  none, under which no reference command reads a reference against a
  ruleset, and changing or removing what does not fit.
- A change sends its place and its state whole and keeps nothing of the
  old place: a state it leaves out is none, so with a ruleset served a move
  into a place that names states is refused unless it names one, and a
  move into a place that names none is refused if it does. A change keeps
  its entry, so a reference is made to name another entry by a removal and
  an addition.
- A reference carries no number, so nothing on a sheet counts an entry's
  uses or charges.
- Nothing limits how many references a sheet or a place holds: what a
  character can carry is the DM's to judge (the owner's ruling of
  2026-11-25).
- No message tells a player that a sheet's references changed: a player
  learns of it only from a sheet the log they watch holds (SPEC-016), so a
  change to a character they are not looking through shows when they next
  look through it, a theft among them (the owner's ruling of 2026-11-25).
- With no ruleset served, a sheet's references are read for their ids
  alone, so a campaign served with none may hold any entry, place and
  state, and is refused when it is next served with a ruleset they do not
  fit.
- With a ruleset served, two writers at once can leave a sheet out of the
  copies rule (above), and the next boot under that ruleset refuses it.

## Requirements

VTT-1015, VTT-1016, VTT-1017, VTT-1021, VTT-1023, VTT-1024, VTT-1025,
VTT-1026, VTT-1028, VTT-1031, VTT-1037, VTT-1038, VTT-1039, VTT-1040,
VTT-1041, VTT-1042, VTT-1043, VTT-1044, VTT-1045, VTT-1046, VTT-1047,
VTT-1048, VTT-1049, VTT-1050, VTT-1059, VTT-1061, VTT-1070, VTT-1071,
VTT-1072, VTT-1073.
