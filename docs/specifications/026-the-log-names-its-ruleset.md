# SPEC-026: The log names the ruleset its outcomes are decided under

## Status

Accepted. Implemented by `contract/vtt/v1/events.proto` (`RulesetNamed`,
arm 40 of `Envelope.payload`), `internal/rules/format.go`
(`Ruleset.Version`) and `internal/rules/load.go` (`manifestJSON`,
`loadManifest`, `checkManifestBounds`, `maxNameBytes`, `loadV2`),
`internal/engine/apply.go` (`Apply`'s `RulesetNamed` arm) and
`internal/engine/state.go` (`State`'s `RulesetID`, `RulesetVersion` and
`RulesetName`, `Snapshot`), `internal/gateway/ruleset.go` (`rulesetNamed`,
`handleUseAbility`) and `internal/gateway/ability_change.go`
(`handleTakeAbility`), `internal/perceive/projection.go` (`classify`,
`forwardable`), `internal/campaign/fanout.go` (`opening`, `resume`,
`rulesetNamed`), `internal/harness/soak.go` (`statesEqual`),
`client/src/state.ts` (`State`), `client/src/fold.ts` (`apply`'s
`rulesetNamed` arm, `foldToDumpJSON`), `client/src/view/spectator.ts`
(`describe`) and `client/src/view/feed.ts` (`STORY`); drawn for the
property tests by `internal/eventgen/model.go`'s `nameRuleset`. Pinned by
`internal/rules/ruleset_version_test.go`,
`internal/rules/qa_ruleset_version_test.go`,
`internal/engine/ruleset_named_test.go`,
`internal/engine/qa_ruleset_named_test.go`,
`internal/gateway/ruleset_named_test.go`,
`internal/gateway/qa_ruleset_named_use_test.go`,
`internal/gateway/ruleset_test.go`'s
`TestUseAbilityHitProducesBatchFirstSequence`,
`internal/perceive/ruleset_named_internal_test.go`,
`internal/perceive/qa_ruleset_named_test.go`,
`internal/campaign/ruleset_named_log_test.go`,
`internal/campaign/qa_ruleset_named_log_test.go`,
`internal/harness/states_equal_ruleset_internal_test.go`,
`cmd/vtt/campaign_fit_test.go`'s
`TestACampaignWhoseLogNamesAnotherRulesetIsServed`,
`cmd/vtt/ruleset_named_dump_test.go`, `contract/roundtrip_test.go`'s
`TestRulesetNamedEnvelopeRoundTrip`, `client/test/fold-ruleset-named.test.ts`,
`client/test/qa-ruleset-named.test.ts`, `client/test/fold-rejections.test.ts`,
`client/test/spectator-view.test.ts` and `client/test/app.test.ts`, and,
through the `toy-brawl`, `door-watch` and `adventure-night` goldens under
`scenarios/goldens`, by `cmd/vtt/scenario_goldens_test.go`'s
`TestScenarioGoldenStreamsHaveNotDrifted`,
`internal/harness/fold_golden_test.go`'s `TestFoldGoldenCorpus` and
`client/test/fold-parity.test.ts`.

## Principles served

Principles 2, 10, 11, 13, 16, 25 and 28 of `docs/blueprint.md`:

- **2.** A change of ruleset is a further entry; nothing rewrites the
  outcomes the log already holds, those appended before the log names any
  ruleset included.
- **10.** The entry is appended in the batch of the use or the take it
  opens, and names the participant who issued that command.
- **11.** Both folds refuse the same entries and hold the same ruleset after
  one, and `vtt state dump` and the client's dump print the same keys.
- **13.** A window words the entry by the ruleset's display name and its
  version, never by its id.
- **16.** Which ruleset the log names is what the table was told: every
  active character log is sent the entry whatever its eyes see, and a
  player or a spectator reads it in the log they watch.
- **25.** A ruleset whose id, version or name the entry could not carry is
  refused when it loads (SPEC-024), so the entry naming a served ruleset
  always folds.
- **28.** For each use, and each take appended with a ruleset served, that
  follows the log's first `RulesetNamed`, the nearest one before it names
  the ruleset and the version that decided it, and a change of either shows
  as a further entry from which outcomes are decided under the new one.

## How it works

**A ruleset's version.** A `ruleset.json` declares `version`, free text its
author writes and keeps true, and `rules.Load` keeps it as written as
`Ruleset.Version` (VTT-970). What the manifest must declare, and the bounds
its id, version and name are held to when it loads, are SPEC-024's
(VTT-971, VTT-972).

**The entry.** `RulesetNamed` carries `ruleset_id` (field 1), `version`
(field 2) and `name` (field 3), a ruleset's id, version and display name;
it is arm 40 of `Envelope.payload` (SPEC-007). In a use's or a take's
batch they are the served ruleset's, as its manifest declares them; in a
character log's opening or resume they are those the world's state names,
which may be a ruleset the server no longer serves. Its three fields are
bounded by both folds as SPEC-018's table states (VTT-974).

**What the fold holds.** `engine.Apply`'s `RulesetNamed` arm refuses an
entry whose ruleset id, version or name is empty or past its bound, in the
order and in the form SPEC-018 states. Otherwise it sets `State.RulesetID`,
`State.RulesetVersion` and `State.RulesetName` to the entry's three fields,
replacing what they held, and changes nothing else; it accepts an entry
naming what the state already names. `Snapshot` copies the three. `git grep
-n -E '\.Ruleset(ID|Version|Name)[^=!]* = ' -- '*.go' ':!*_test.go'
':!contract/gen'` finds them assigned only in `Apply`'s arm and in
`Snapshot`, and `-E 'Ruleset(ID|Version|Name):'` over the same files finds
only `internal/adventure`'s own `Adventure.RulesetID`. `vtt state dump` and
`get_state` print them under the keys `RulesetID`, `RulesetName` and
`RulesetVersion`,
and, since each is `json:",omitempty"`, nothing of a ruleset for a log that
holds no `RulesetNamed` (VTT-979, VTT-987). The client's fold mirrors the
arm, the refusals and the dump (SPEC-022, VTT-983).

**When it is written.** `(*Server).rulesetNamed` compares the state a
command was judged against, its `RulesetID` and `RulesetVersion`, with the
served ruleset's `ID` and `Version`, each by string equality, so `"1"` and
`"1.0"` are two versions. When either differs, the state naming none
included, it returns a `RulesetNamed` of the served ruleset's id, version
and name; otherwise nil. It compares neither the name nor any file of the
ruleset, by the owner's ruling of 2026-11-23 that the version is the
author's to keep true, so a ruleset served again under the id and the
version the log last names, its name or its abilities changed, gains no
entry (VTT-978). `handleUseAbility` and `handleTakeAbility` call it, and
`git grep -n -E '\.rulesetNamed\(' -- '*.go' ':!*_test.go'` finds no other
caller:
- `handleUseAbility`, once the sheet's refusal (SPEC-013) and
  `rules.Resolve` have passed, puts the entry first in the batch `Resolve`
  returns, so the use appends in one batch the entry and then the outcome
  (VTT-975). With no ruleset served the use is refused before anything is
  resolved or written (VTT-132), and a use refused anywhere appends neither.
- `handleTakeAbility`, with a ruleset served, puts the entry first, before
  the `AbilityTaken` and any `ResourceRemoved` the ruleset's `Spends`
  chooses (VTT-988, VTT-991): what a take removes is the ruleset's choice,
  by the owner's ruling of 2026-11-24. With none served a take writes none
  and removes no resource, whatever the log names, since no ruleset decides
  anything of it (VTT-989, VTT-768).

Each handler stamps the entry as it stamps the rest of its batch: an event
id, the time, and the issuer's participant id and role (VTT-976, VTT-799);
and `campaign.AppendBatch` writes the open session's id, or none. The batch
folds whole or not at all, so a use or a take the fold refuses leaves no
entry. The result carries the batch's first sequence, which is the entry's
when one opens the batch (SPEC-007, VTT-137, VTT-992).

**What writes none.** A `give_ability` writes none, whatever the log names,
since what it writes is the DM's (VTT-990). No other command calls
`rulesetNamed`, `remove_condition`, `add_actor`, `load_map` and
`load_adventure` among them, and nothing is written when the server boots:
`composeServer` reads no ruleset id from the log, and the writers of a
`RulesetNamed` outside tests are the handlers above, through
`rulesetNamed`, and the character logs' `opening` and `resume`, through
`internal/campaign`'s own `rulesetNamed` (`git grep -n
'Envelope_RulesetNamed{' -- '*.go' ':!*_test.go' ':!contract/gen'`, besides
`internal/eventgen`'s draw for property tests). So a
campaign whose log names one ruleset id, served under a ruleset of another
id whose sheets fit it (SPEC-025), is served (VTT-977), and its next use or
take opens with the entry naming the ruleset served.

**Two at once.** The comparison reads the snapshot the command was judged
against, and the fold accepts an entry naming what the state already names.
So two first uses or takes after a change, each judged before the other
lands, each append an entry naming the same ruleset. By the owner's ruling
of 2026-10-21 on a near-theoretical edge case, this has no code, no test
and no debt entry.

**Who is sent it.** The DM's and the agent's connections are sent the world,
the entry with its issuer (VTT-377, VTT-976). `classify` forwards a
`RulesetNamed` to every active character log whatever its eyes see, as it
does `SessionStarted` and `SessionEnded` (VTT-980), and `forwardable`
clears its participant id and role (VTT-272), keeping its event id, session
id and time (SPEC-021). A player and a spectator watching a party member are
sent it through the character log they watch; a spectator watching nobody
is sent no event at all (SPEC-007).

**A log that opens or resumes after it.** A character log is sent nothing
appended before it opens or while it is paused (SPEC-021), so `opening` and
`resume` write the ruleset the world names into it, each from the world
before the event that opens or resumes it:
- An opening holds, when that world names a ruleset, a `RulesetNamed` of
  it right after the open session's `SessionStarted`, or first when no
  session is open, and before the party and the public notes (VTT-981).
- A resume holds, after its session entries and before its `TokenHidden`
  entries, a `RulesetNamed` of the world's ruleset when that world names one
  and the log's fold names none or another id or version (VTT-982). A log
  whose fold names the world's id and version resumes with none, and keeps
  the name its fold holds even where the world's last entry carries another
  name for that id and version.

Such an entry carries no event id, session id or time, as no event caused
it (SPEC-021). A reopen of the campaign writes nothing into any log, since
`Campaign.appendLocked` is the one production caller of the store's
`AppendBatch` (`git grep -n 'AppendBatch(' -- '*.go' ':!*_test.go'
':!contract/gen'` prints the store's and the campaign's definitions, its
call, and the gateway's calls of the campaign's), and a log that
resumes after one is compared as above, so one whose fold names the world's
id and version is written no second entry.

**How a window words it.** `describe` words it `rules: <name>, version
<version>`, and every window's Story draws it, with an event id or none;
the DM's and the agent's Events panel draws it as every entry (SPEC-022,
VTT-984, VTT-985).

## Consequences

- A path added to append an outcome a ruleset decides must call
  `rulesetNamed` and put its entry first in that batch. No gate makes it do
  so; `rulesetNamed`'s doc comment names the use's and the take's batches
  and points here.
- Outcomes a log holds from before its first `RulesetNamed` name no ruleset,
  and stay so.
- A correction a ruleset's author makes without changing `version` leaves no
  mark in the log; the version is the author's to keep true.
- A ruleset renamed under one id and version writes no entry, so the world
  and every log keep the name the last entry carried, and windows word it by
  that name.
- A switch of ruleset id is recorded, never refused; which ruleset a
  campaign is served under is the operator's, within what SPEC-025's fit
  refuses.
- A server restarted under another ruleset, with no use or take after it,
  adds nothing to the log.
- A log whose eyes see none of the actors a use or a take names is still
  sent the entry that opens its batch, at that point in its log: which
  ruleset is in force is the table's mechanical record, not the story,
  which principle 17 governs (the owner's ruling of 2026-11-24); the Story
  draws it all the same (SPEC-022).
- `/api/ruleset` and `get_ruleset` answer no version (SPEC-012); the ruleset
  an adventure declares, `adventure.json`'s `ruleset`, names an id and no
  version; and the ruleset `vtt mcp --ruleset` loads for the agent's own use
  names nothing in the log.
- ADR-012 gives a ruleset options; the manifest's fields are SPEC-024's and
  hold none, so the entry names none.

## Requirements

VTT-970, VTT-971, VTT-972, VTT-974, VTT-975, VTT-976, VTT-977,
VTT-978, VTT-979, VTT-980, VTT-981, VTT-982, VTT-983, VTT-984, VTT-985,
VTT-986, VTT-987, VTT-988, VTT-989, VTT-990, VTT-991, VTT-992.
