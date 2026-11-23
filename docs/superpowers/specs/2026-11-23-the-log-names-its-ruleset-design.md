# The log names the ruleset and the version a campaign is played under

## The problem

**A ruleset has no version.** A ruleset's `ruleset.json` is decoded into
`manifestJSON` (`internal/rules/load.go`): `id`, `name`, `format_version`,
`properties` and, optionally, `sheet` and `free_usage_name`. `loadManifest`
decodes it through `decodeStrict`, which refuses a field the format does not
know, so a `ruleset.json` that declares a `version` is refused today.
`rules.Ruleset` (`internal/rules/format.go`) carries `ID`, `Name` and
`FormatVersion`, and `format_version` is the format's version, "3" for every
ruleset (`formatVersion`), not the ruleset's.

**The log names no ruleset.** A campaign is played under the ruleset
`vtt serve --ruleset <dir>` loads when it boots: `composeServer`
(`cmd/vtt/serve_compose.go`) loads it with `rules.Load`, checks the
campaign's sheets against it (`sheetsFit`, SPEC-025) and hands it to the
gateway (`WithRuleset`). Nothing about it is written to the log. An outcome
is the batch `rules.Resolve` returns, which `handleUseAbility`
(`internal/gateway/ruleset.go`) appends, and no payload of `Envelope`
(`contract/vtt/v1/events.proto`) names a ruleset. The same campaign can be
served at its next boot under a corrected ruleset, or under another one, and
its log reads the same either way. An outcome in the record therefore cannot
be traced to the rules that decided it, and a correction made between two
sessions leaves no mark (principle 28 of `docs/blueprint.md`).

**Every append answers a command today.** `git grep -nE
'\.Append(Batch)?\(' -- '*.go' ':!*_test.go' ':!contract/gen'` finds
the campaign's own write to the store, in `Campaign.appendLocked`, and the
calls in `internal/gateway`, each on the path that answers a command
(`handleCommand`, `handleUseAbility`, `handleLoadAdventure`,
`handleLoadMap`, `appendChange` for a give and a take, `handleRemoveActor`,
`handleRemoval`, `handlePromotion`). The boot in `composeServer` appends
nothing.

**What MapTool does.** An add-on library's `library.json` carries its
`namespace` and a `version`, free text. `AddOnLibrary` meant to refuse a
library without one (`Objects.requireNonNull` with
`library.error.emptyVersion`), but the version is read from a proto3
string, which is never null, so a library with an empty version loads. A
campaign file keeps the libraries' current copies, each stored under its
MD5 (`PersistenceUtil.saveAddOnLibraries`, `loadAddOnLibraries`).
`AddOnLibraryManager.registerLibrary` refuses a second library under a
namespace already registered; importing one asks the operator whether to
replace it (`library.error.addOnLibraryExists`), and `LibraryManager`
deregisters the old copy. Nothing records which version decided an
outcome, or when one copy replaced another. This ticket borrows the version
every ruleset declares, free text, and holds it to what MapTool meant: never
empty. It does not borrow the copy kept inside the campaign: a ruleset stays
a directory the operator serves, and the log names it.

**What the owner ruled.** This is step 1 item 3 of ADR-012's build order,
taken after item 4 and before item 5, whose references will depend on it (the
owner, 2026-11-13). On 2026-11-23: a campaign served under a ruleset of
another id than the one its log names has the switch recorded, not refused;
nothing checks whether a ruleset's files changed under one version, which is
the author's to keep true, as in MapTool; and every participant is sent the
entry that names the ruleset.

**Not this ticket.** A ruleset's options (ADR-012 §1): no ruleset declares
one yet. The version in `/api/ruleset` or `get_ruleset`. The ruleset an
adventure declares (`adventure.json`'s `ruleset`), which names an id and no
version. The ruleset `vtt mcp --ruleset` loads for the agent's own use.

## Done looks like

1. **A ruleset declares its version.** A `ruleset.json` with a non-empty
   `version` loads, and `rules.Ruleset` carries it; one with no `version`, or
   an empty one, is refused at load, naming the file and the field, as `id`
   and `name` are. Both rulesets under `rulesets/` declare one. An
   `internal/rules` test of each fails on today's tree, which refuses
   `version` as an unknown field.
   - The fold bounds what an entry carries (SPEC-018), and `loadManifest`
     bounds neither the ruleset's `id` nor its `name` today
     (`checkManifestBounds` bounds the property keys). A ruleset whose id or
     version the entry could not carry is refused when it loads, naming the
     field, not when the entry is appended. This reverses part of SPEC-018's
     paragraph on the strings a loader does not bound, which names the
     manifest's id and name among them.
2. **An outcome names the rules that decided it.** A campaign served under a
   ruleset R at version V, in which an ability is then used: the log holds an
   entry naming R's id and V before the `AbilityUsed`, and no entry naming
   another ruleset lies between them. A test that boots, uses an ability and
   reads the log fails on today's tree, where no entry names a ruleset.
3. **A correction shows where it began.** The same campaign served again
   under R at version V′, and an ability used: the log holds an entry naming
   V′ after the first `AbilityUsed` and before the second, so each outcome's
   nearest preceding entry names the version that decided it.
4. **A switch of ruleset is recorded.** The same campaign served under a
   ruleset of another id, whose sheets fit it (SPEC-025): the server starts,
   and an entry naming the new id and its version precedes the next outcome.
5. **Nothing is added when nothing changed.** The same campaign served again
   under the ruleset and version its log last names gains no entry naming a
   ruleset, and neither does a campaign served with no ruleset. A ruleset whose
   files changed while its version did not is served like an unchanged one
   (the owner's ruling). This holds today, when nothing is written; it guards
   the code that writes the entry.
6. **Every participant is sent it.** The DM's, the agent's, a player's and a
   spectator's streams, and every character log (SPEC-021), carry the entry,
   and so does a character log that opens or resumes after the entry was
   written, since a log that opens is not sent earlier entries (SPEC-021).
   Projection and fan-out tests in `internal/perceive` and
   `internal/campaign` hold each.
7. **The client folds it as the server does.** The client's fold skips a
   payload it does not know (`client/src/fold.ts`), so folding without an
   error shows nothing. The fold-parity goldens (`client/test/fold-parity.test.ts`,
   `scenarios/goldens`) carry the entry, and the client's fold of them agrees
   with the server's.
8. **The gate.** `task check` and `task e2e` are green.

## Rules this puts on the system

- A ruleset declares a version, and one that declares none, or an empty one,
  is refused when it loads.
- A ruleset whose id or version the entry naming it could not carry is
  refused when it loads.
- An outcome is appended only after an entry naming the id and the version
  of the ruleset that decides it, with no entry naming another ruleset
  between them. Outcomes a log already holds stay as they are.
- A campaign served under a ruleset whose id or version differs from the one
  its log last names gains an entry naming it before its next outcome.
- A campaign served under the ruleset and version its log last names gains no
  entry naming a ruleset.
- A campaign served with no ruleset gains no entry naming a ruleset.
- A campaign served under a ruleset of another id than the one its log names,
  whose sheets fit it, is served, not refused.
- Every seat, and every character log, is sent the entry naming the ruleset,
  a character log that opens or resumes after it was written among them.

## What it touches

In the order the change must reach them:

- The ruleset format: `internal/rules/load.go` (`manifestJSON`,
  `loadManifest`, `checkManifestBounds`), `internal/rules/format.go`
  (`Ruleset`), `internal/rules/schema/ruleset.schema.json`, both
  `rulesets/*/ruleset.json`, and every manifest a test writes: the files
  `git ls-files '*ruleset.json'` lists and the manifests written inline in
  Go tests. (`git grep -l format_version` also finds maps, adventures, art
  and campaign files, which carry a field of that name of their own.)
- The contract: `contract/vtt/v1/events.proto` (`Envelope`), additively
  (ADR-007), what `task generate:contract` regenerates under `contract/gen`,
  and `contract/testdata`.
- The fold: `internal/engine/apply.go` (`Apply`), the one place game state
  changes, and `internal/engine/state.go` (`State`, `Snapshot`) if the
  state holds the ruleset the log last names.
- Where the entry is written: `cmd/vtt/serve_compose.go` (`composeServer`)
  or `internal/gateway` (`WithRuleset`, `handleUseAbility`).
- The projection and the character logs: `internal/perceive/projection.go`
  (`classify`) and `internal/campaign/fanout.go` (a log's opening and
  resumption).
- The client: `client/src/state.ts` and `client/src/fold.ts`;
  `client/src/view/spectator.ts` (`describe`, which words every entry kind
  for the Events panel and the Story); `client/src/view/feed.ts` (`STORY`,
  which decides for each kind whether the Story draws it); and the rebuilt
  `cmd/vtt/webdist`.
- What reads every payload kind or the log's batches: `internal/eventgen`,
  the tests that walk every payload kind, `scenarios/goldens`, and, if the
  entry is written at boot, `internal/harness`, `cmd/vtt/harness_boot.go` and
  `client/e2e/setup.ts`.

This crosses more than two components and names more than ten files, most of
them fixtures.

## Specifications this moves

- docs/specifications/024-a-ruleset-declares-its-properties.md
- docs/specifications/007-the-wire-contract.md
- docs/specifications/016-the-projection.md
- docs/specifications/021-each-party-member-keeps-its-own-log.md
- docs/specifications/022-a-client-holds-one-log.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
- docs/specifications/012-the-read-surface.md
- New: the log names the ruleset and the version a campaign is played under.

## What could not be established

- **Whether the Story draws a line for the entry.** The DM's Events panel
  draws every entry, worded by `describe` (`client/src/view/spectator.ts`),
  so it draws this one too. Whether the Story does, in a player's window and
  in the DM's, is `STORY`'s entry for the kind (`client/src/view/feed.ts`),
  and is not ruled; the plan proposes and the sign-off decides.
- **When the entry is written**: at the boot that serves a changed ruleset,
  or ahead of the first outcome after it. Both meet the rules above. An entry
  written at boot is the first append no command causes, so it names no
  participant who caused it. The plan measures each, and the sign-off
  decides.
- **The alpha campaign** in `~/dev/vtt-alpha`, outside the repository, is
  served under `rulesets/dnd45e-minimal`; once it is served under a ruleset
  that declares a version, its log gains the entry before its next outcome.
