# Plan: a ruleset names its properties in the table's words

- **Ticket:** `docs/superpowers/specs/2026-11-05-a-ruleset-names-its-properties-design.md`
  (ADR-012, Build order step 1, its first ticket).
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-05 by an agent that did not write the ticket, read-only
    against `38d11bd` on `feat/a-ruleset-names-its-properties`, with the
    ticket uncommitted.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and gremlins were not run.
- **To be committed as:** `docs/superpowers/plans/2026-11-05-a-ruleset-names-its-properties.md`.
- **Base:** `38d11bd`.
- **Branch:** `feat/a-ruleset-names-its-properties`.
- **Owner's rulings taken in:**
  - 2026-11-05, recorded in the ticket's "Not part of done": visibility is the
    next ticket's, with the projection withholding by it; the sheet form is
    the next ticket's; the only types are number and resource. Recorded in
    the paragraph after its "What it touches": the work is not split.
  - 2026-11-05: no counts or measured figures in anything written down.
  - 2026-11-04 (ADR-012): format v2 is replaced outright; the rulesets in the
    repository are the project's own systems.
  - 2026-09-04: nobody uses the product; no migration, no phased removal.
  - 2026-09-23: a fact has one home; a duplicate points at its source (D10).
  - 2026-10-09: a field outside SPEC-018's table is unbounded on purpose (D7).
  - 2026-10-21: a near-theoretical edge with a small consequence gets no code,
    no test, no debt entry (D7).
  - 2026-10-26: the client's full mutation run is the owner's to ask for
    (SPEC-023).
- **Signed off:** by the owner, 2026-11-05 (under Decisions).
- **Ids:** allocated at Task 0, after sign-off. R1 to R9 below are this
  plan's labels; the table under Requirements gives their ids.

## The verdict, check by check

1. **Every path resolves. By command** (P1). Every path the ticket names
   exists, and every symbol it names is in the file it names:
   `manifestJSON`, `resourceJSON`, `crossValidateResources`,
   `supportedFormatVersions` and `formatVersion1RejectedMsg` in `load.go`;
   `Ruleset.Attributes`, `Defenses` and `Resources` in `format.go`;
   `compileCompositions`, `buildFixtureState`, `handleRuleset`,
   `RulesetMeta`, `mcp.Config.RulesetGuide`; `Actor.attributes` and
   `Actor.resources` in `events.proto`; `chip.title = r.name` and the
   `resourceChanged` arm of `describe` in `client/src/view/spectator.ts`;
   `TokenProperty` and `CampaignProperties.tokenTypeMap` in MapTool. A
   claim points at the wrong file: G1.
2. **"Done" is an observation. By command, where a command can reach it.**
   - Item 1: `TestConformanceOverRulesetsGlob` passes today (P2), so
     "unchanged" is a claim that can be held.
   - Items 2 to 6 name tests that do not exist yet, so none can be run
     today. What each will pin is absent today, by command (P9, P10): the
     loader accepts only `"2"` and decodes no property list; `rulesetJSON`
     declares no field for attributes, defenses or properties; `Namer` has no
     member that names a property; the chip's title is the key. Item 4's
     case loads today: the shipped rulesets are in format v2's shape and
     the conformance glob loads them.
   - The redness of items 2 and 3 today is partly the wrong reason (G6).
   - Item 7 is the gate.
3. **Each rule is breakable. A reading.** Each of the ticket's rules has an
   observation that a break turns red; the breaks below name them. These need
   the plan's help: "anywhere in the ruleset" is bounded by the search in P7
   and enumerated in D6, so every site has its own case; and "never by its
   key" cannot hold in this ticket for a key the ruleset does not declare
   (G3).
4. **The scope matches the claim. By command, then a reading.** The readers
   the ticket lists are incomplete, and so is "What it touches" (P3 to P8;
   G2). The extra files change shape, not size: the same rules engine, read
   surface and window, plus the agent's read tool that D10 adds. The owner's
   ruling that the work is not split stands.
5. **No recorded decision is contradicted silently. A reading.**
   - ADR-012 §2 gives each property a visibility; the ticket defers it by the
     owner's dated ruling. Not silent.
   - ADR-012's "format v2 is replaced outright" is followed (D1).
   - SPEC-022's "a condition, a resource, a note's key and an adventure's id
     are shown as the entry names them" is overturned for a resource; the
     ticket names SPEC-022.
   - SPEC-012's "`resources` as the names of the ruleset's resource
     definitions" moves; the ticket names SPEC-012. D9 keeps the field, so
     VTT-114 stays true as worded.
   - SPEC-018's decision that a ruleset's display names and descriptions are
     unbounded holds (D7). Its wording moves and the ticket does not name it
     (G4).
   - `docs/superpowers/specs/2026-11-02-a-refusal-says-why-design.md` left
     `rules.Resolve`'s refusals in their words until the rules engine
     consumed a ruleset; this ticket defers them again, to ADR-012 step 2,
     and says so.
   - VTT-283 becomes false as worded, and the ticket does not say so (G5).
   - Principle 5: an operation once offered never changes meaning. D9 keeps
     `/api/ruleset` additive.
6. **The records it moves. By command, then a reading.** The section exists;
   SPEC-012 and SPEC-022 resolve; the new specification is written `New:`.
   `git grep -i` for `defense`, `ruleset.json`, `rules.Load` and
   `internal/rules` over `docs/specifications` (P12) finds SPEC-012 and
   SPEC-018. SPEC-018 moves and is not named (G4). If D10 stands, SPEC-012
   also gains the agent's reader of `/api/ruleset`.

## The gaps this plan inherits

The ticket was amended before sign-off. G1, G2, G4 and G5 are closed in it.
They, with checks 1, 4, 5 and 6 above, describe the ticket as it was
verified.

- **G1. The ability's cost is in `client/src/view/player.ts`.** The ticket
  names `client/src/player.ts`. The text a player reads is the ability
  button's `title` in `renderPlayerPanel`, in the view module.
  `client/src/player.ts`'s `affordable` reads the key to index the actor's
  resources, which is right, and keeps it.
- **G2. Files the ticket does not list** (P3 to P8):
  - `internal/rules/resolve.go`: `evalThresholds` reads `Ruleset.Resources`;
  - the `rules.Ruleset` literals in `resolve_test.go`, `resolve_int32_test.go`
    and `resolve_v2_test.go`;
  - `internal/rules/schema/atom.schema.json` and `validParamKinds`: the atom
    param kinds `attribute` and `defense` (D4), and every atom, fixture or
    inline, that uses them;
  - `schema_test.go`, `id_bound_internal_test.go` and
    `qa_ruleset_bound_test.go`, which walk the schemas by path;
  - the inline manifests of `internal/adventure`'s `qa_cmd_id_bound_test.go`
    and `qa_id_bound_test.go`;
  - `client/src/view/naming.ts` (`Namer`), `client/src/view/player.ts`,
    `client/src/app.ts`, and the rebuilt `cmd/vtt/webdist`;
  - by D10, `internal/mcp`: a new read tool and `GoRegisteredToolNames`.
- **G3. "Never by its key" cannot hold yet for an undeclared key.**
  `validateAddActor` checks no key against the ruleset until principle 26's
  ticket, so the log can name a resource no property declares; and a window
  has no list before `/api/ruleset` answers. D11 names a declared resource by
  its display name and anything else by its key, as SPEC-022 names an ability
  by its display name, else its id. R8 is worded so.
- **G4. SPEC-018 moves and is not named.** Its Status and its paragraph on
  `internal/rules/load.go`'s `maxIDBytes` name "attribute, defense and
  resource names"; its search sentence says a grep of `load.go`'s length
  checks prints only the resource-name bound, which a bound on a property's
  key will not match. Task 6 moves it.
- **G5. VTT-283 becomes false as worded.** It names "an attribute, defense or
  resource name". It is withdrawn and succeeded by R9 at Task 0.
- **G6. Items 2 and 3 are red today for the wrong reason.** Any manifest in
  the new shape is refused today by `decodeStrict`, as an unknown field; and
  a property read as the other type is refused today as undeclared, since
  today's sets are per list. The red that proves each check is its break
  (K1 to K14), and each type-case test asserts the type named in the fault,
  not only a refusal.
- **G7. Items 5 and 6 could not be run** before their tests exist; check 2
  says what was confirmed instead.

## Measurements this plan stands on

All on 2026-11-05 at `38d11bd`, by command.

| # | Probe | Result |
|---|---|---|
| P1 | `test -e` on every path; `grep` for every symbol; MapTool at RPTool `63862e4` | all resolve; `TokenProperty` holds `name`, `shortName`, `displayName`, `ownerOnly`, `gmOnly`, `defaultValue`, `highPriority`; `TokenPropertyFunctions` names a property by its display name when it has one, else its name |
| P2 | `go test ./internal/rules/conformance/ -run TestConformanceOverRulesetsGlob -count=1` | ok |
| P3 | `git grep -E '\.Attributes\b\|\.Defenses\b\|\.Resources\b' -- '*.go'`, tests excluded | readers of the `Ruleset`'s lists: `load.go`, `compile.go`, `resolve.go` (`evalThresholds`), `conformance.go`, `adventure/load.go`, `gateway/metadata.go`. The other hits read an `Actor`'s maps |
| P4 | `git grep 'rules\.Ruleset{'` | `resolve_test.go`, `resolve_int32_test.go`, `resolve_v2_test.go` |
| P5 | `git ls-files` for `ruleset.json` | `rulesets/`; `internal/rules/testdata` (`valid`, `valid-v2`, `invalid`, `invalid-v2`); `internal/rules/conformance/testdata`; `internal/adventure/testdata/ruleset`; `internal/adventure/conformance/testdata/rulesets` |
| P6 | `git grep -l defenses` in Go tests | inline manifests in `internal/rules` (`compile_test.go`, `load_test.go`, `load_dice_test.go`, `qa_ability_outcomes_test.go`, `qa_ruleset_bound_test.go`), `internal/adventure` (`qa_cmd_id_bound_test.go`, `qa_id_bound_test.go`) and `internal/gateway` (`qa_ability_sight_test.go`, `qa_cmd_id_bound_test.go`, `qa_id_bound_test.go`, `qa_ruleset_bound_test.go`) |
| P7 | the sites in `load.go`, `compile.go` and `adventure/load.go` that check a name against a declared list | `checkExprRefs` (a resource's `default_max_expr`, a threshold's `when`); `compileTwoActorExpr` (`roll`, `vs`, `delta_expr`); `bindAtom` (name-kind params); `compilePower` (`usage.limited.resource`); `compileEffects` (`resource_change.resource`); `loadActors` in `adventure/load.go` (a statblock's `attributes` and `resources`) |
| P8 | `git grep` for a resource named in `client/src` | the chip's `title` and `describe`'s `resourceChanged` arm in `view/spectator.ts`; the ability button's `title` in `view/player.ts`; `affordable` in `player.ts`, which indexes by key |
| P9 | reading `loadManifest` | `decodeStrict` runs before the `format_version` check, so a format v2 manifest under a new shape is refused by `encoding/json`'s unknown-field text before its version is read |
| P10 | `sed` over `rulesetJSON`, `Namer`, `RulesetMeta` | no field or member for a property |
| P11 | `internal/mcp` | `readMetadata` in `door_tools.go` returns a gateway route's body verbatim; `GoRegisteredToolNames` in `read_tools.go` is the one list of hand-registered tools, which `cmd/vtt/mcp_e2e_test.go` and `read_tools_test.go` read; `TestMCPRulesetGuideWithAndWithoutRulesetFlag` requires `get_ruleset_guide` to return `guide.md` verbatim |
| P12 | `git grep -i` over `docs/specifications` | SPEC-012, SPEC-018 |
| P13 | `docs/requirements.md` | VTT-283 names attribute, defense and resource names; VTT-114 names `resources`; VTT-118 lists no properties; VTT-287 is worded for any bounded name |
| P14 | `tools/mutation-equivalents.txt`, `tools/ts-mutation-equivalents.txt` | no Go adjudication sits in a touched Go file; the client's sit on comparators in `player.ts`, `view/spectator.ts` and `view/grid.ts`, away from the lines this plan changes |
| P15 | `.semgrep/vocabulary.yml` | bans, in every Go file under `internal/` and `cmd/`, tests included, among others a bare `hp` and `fortitude` |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone, once, last.
  - 3: the protobuf contract does not change: no field, no message, no
    regenerated file (D3).
  - 4: one fold. `engine.Apply` does not change.
  - 5: no game-system vocabulary in Go, tests included (P15). A Go test about
    `dnd45e-minimal` compares against the loaded `Ruleset`, never a literal
    key.
  - 8: cite names; never `file:line`, never `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `internal/`, `cmd/` or `client/src` is a warning, a
    pointer, or an exported symbol's one-line doc. A comment this change makes
    false is rewritten to that shape, among them: `format.go`'s package doc,
    `Ruleset`'s fields and `ParamDef`'s doc; `load.go`'s `Load`,
    `supportedFormatVersions`, the attribute-and-defense collision comment in
    `loadManifest`, `crossValidateResources` and `reservedScopeWords`;
    `compile.go`'s `attrOrDefSet` comment and `compileTwoActorExpr`'s doc;
    `resolve.go`'s doc on stats; `buildFixtureState`'s doc; the union comment
    in `adventure/load.go` and `adventure/format.go`; the schemas'
    descriptions.
- **Keys do not change.** Every property keeps today's name as its key, so
  the goldens, the scenarios, the adventures' statblocks and their golden
  streams stay byte for byte.
- **`check:new-prose`** holds added comment lines to its column band, tests
  included. `python3 tools/check-new-prose.py main`,
  `python3 tools/check-requirements-chain.py .` and
  `python3 tools/check-comments.py main` (after staging) before every commit.
- **`go-arch-lint`.** `internal/mcp` imports no `internal/rules`; D10 needs
  none.
- **The coverage floors** in `tools/coverage-thresholds.txt` hold for every
  touched package, and the client's per-file floors for every touched file.
- **The Go mutation gate runs in `task check`** over `internal/rules`,
  `internal/rules/conformance`, `internal/adventure`, `internal/gateway` and
  `internal/mcp`. Each new condition has a test that goes red when it is
  negated. gremlins does not mutate a `case`, so each arm of a switch on a
  type has its own case in a table test.
- **A new function or test goes after a closing brace** (`check:doc-owner`).
- **A test the register cites has a literal name** and its rule's id line.
- **A client test reads no file outside `client/`** but one
  `tools/ts-mutation-inputs.py` hashes.
- **A client change ships with `task build:client`'s `cmd/vtt/webdist`** in
  the same commit; `check:drift` passes only after that commit.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo a break: the inverse edit, checked with
  grep, after asserting the broken text is not already there.
- **The review gate refuses untracked files.** Park scratch files outside the
  tree with `cp` and `cmp`. Hash the tree before and after any reviewer.
- **The pre-push hook takes minutes.** Its longest timeout, its whole output
  to a file, the machine's load checked first.

## Rule 9: how MapTool does this

In MapTool at RPTool `63862e4` (P1):

- **A property is a `TokenProperty`**: `name`, the key a macro reads; an
  optional `displayName`; a `shortName`; `ownerOnly` and `gmOnly`;
  `defaultValue`; `highPriority`, shown on the stat sheet. The properties are
  grouped per token type in `CampaignProperties.tokenTypeMap`. Every value is
  text.
- **Borrowed:** the display name beside the key, and the reader that names a
  property by its display name else its key (`TokenPropertyFunctions`). Here a
  declared property must have one, so the fallback reaches only a key the
  ruleset does not declare (D11).
- **Deferred:** `ownerOnly` and `gmOnly` are ADR-012 §2's visibility, the next
  ticket's. The grouping per token type is the sheet form, also the next
  ticket's.
- **Not borrowed:**
  - `shortName`: the chip shows current over max and names the resource on
    hover; nothing here is short of room;
  - `highPriority`: a "primary" property is the genre assumption
    `tokensOnScene`'s own doc refuses;
  - `defaultValue`: a sheet's starting value belongs to the form; a
    resource's default maximum is already `default_max_expr`;
  - untyped values: every value here has a type.

## Decisions

1. **D1. The replaced format is `format_version` `"3"`, and the version is
   read before the shape.** `loadManifest` reads `format_version` as raw
   JSON with a lenient decode first and refuses any value but the string
   `"3"`, a number and an absent field among them, naming the file, the
   field and the value as written, and saying what format `"3"` declares;
   then it decodes strictly. `formatVersion1RejectedMsg` and its v1 wording are deleted: the
   same text serves every retired value.
   - **Forced by item 4.** Under `"2"`, a format v2 manifest fails
     `decodeStrict` with `encoding/json`'s unknown-field text before any
     version check (P9). An actionable reason would need a sniff over key
     names, which cannot tell an old manifest from a new one with a stray
     key. A version check reads its own field alone, as v1's retirement did.
   - **Forced by rule 8.** "Format v2" names the replaced shape in ADR-012
     and in `docs/superpowers/specs/2026-07-25-format-v2-composition-design.md`.
     Reusing `"2"` gives the old shape's name to the new one.
   - The text, signed off by the owner as proposed on 2026-11-05:
     `unsupported value "2": this loader reads format_version "3", which declares every property
     under "properties" with its key, type, name and description
     (docs/adr/012-the-rules-contract.md)`.
2. **D2. The shape.** A top-level `properties` list, in the ruleset's order.
   Each entry has `key`, `type`, `name` and `description`. A `resource`
   entry may carry `default_max_expr` and `thresholds`, as a resource does
   today. A `number` entry carrying either is refused, as `contributionKeys`
   refuses a field of another kind. No `visibility` field: the strict decode
   refuses one, so no visibility is declared that nothing enforces.
   - **Top level, not inside a form.** ADR-012 §2 names properties at the
     ruleset, and §3's form lists which it has. The form is the next ticket.
   - **`name` and `description`, forced by one word per concept.** The format
     and `/api/ruleset` already call a condition's display name `name` and its
     explanation `description` (`conditionJSON` in `load.go` and in
     `metadata.go`), and SPEC-018's decision on unbounded strings names "a
     ruleset's display names, descriptions". ADR-012's "explanation" is prose,
     and its names are working names.
   - **`key`,** the ticket's word; SPEC-018 calls these an actor's resources
     and attributes keys. A key keeps today's rules for a name: the IDENT
     charset, the reserved scope words and `maxIDBytes`.
   - **Per-type fields on a flat entry** is a preference, following
     `contributionKeys` in the same loader.
3. **D3. Where a value lives.** A number property's value is an entry of
   `Actor.attributes`, a resource's an entry of `Actor.resources`. `@` reads a
   number property and `#` a resource's current value, as today. Forced by
   rule 3 and by item 1: those maps are what a sheet holds.
4. **D4. The atom param kinds `attribute` and `defense` become `number`.**
   `resource` now means a resource property; `int`, `expr` and `condition`
   stay. Forced by "a property used as a type it is not, anywhere": once the
   lists merge, `defense` has nothing to be checked against; and nobody uses
   the product, so there is no alias. Cost: `int`, a value, sits beside
   `number`, the name of a number property; `atom.schema.json`'s description
   says which is which.
5. **D5. The `Ruleset`.** `Properties`, in the ruleset's order, replaces
   `Attributes`, `Defenses` and `Resources`. A `Property` holds its key, type,
   name and description, and a `*ResourceDef` set exactly when its type is
   resource; `ResourceDef` loses its `Name`. A lookup by key is a plain scan,
   forced by the hand-built `Ruleset` literals in the resolve tests (P4),
   which an index built by `Load` would not reach. The final names are the
   new specification's.
6. **D6. A key is checked at the sites of P7, each with its own case.**
   `checkExprRefs`, `compileTwoActorExpr`, `bindAtom`'s `number` and
   `resource` kinds, `compilePower`'s cost and `compileEffects`'
   `resource_change`, against a set per type. Each refusal names the file,
   the field, the key and the fault: undeclared, or a resource read as a
   number, or a number read as a resource. A single-actor position may now
   read a key that was a defense; that widening is accepted.
   - **Not checked:** an atom no ability composes, which is never compiled,
     and an `expr` binding no template splices in. Neither is read, so a key
     named there changes nothing: near-theoretical, small consequence (the
     owner, 2026-10-21). VTT-613 and VTT-614 therefore name the sites rather
     than say "anywhere", and the ticket's item 2 is worded so.
7. **D7. A display name and a description have no bound.** Forced by
   SPEC-018's decision on a string a ruleset carries and its loader does not
   bound, and by the owner's 2026-10-09 ruling; neither reaches the log.
   Absent or empty is refused (the ticket). A name of spaces alone is not
   checked: near-theoretical, small consequence.
8. **D8. `adventure.Load` checks a statblock per type.** An `attributes` key
   must be a number property and a `resources` key a resource property. The
   refusal keeps naming the file, the field and the key, so
   `TestLoadInvalidFixtures`' undeclared rows pass unedited, and names the
   type when the key is the other type's. It lands in Task 1, because Task 1
   already rewrites `loadActors`' sets.
9. **D9. `/api/ruleset` gains `properties` and keeps `resources`.** Each entry
   carries `key`, `type`, `name` and `description`, in the ruleset's order,
   and the list is `[]`, never `null`, with no ruleset loaded. `resources`
   keeps its meaning: the keys of the resource properties.
   - Forced by principle 5: an operation once offered never changes meaning.
     VTT-114 stays true as worded.
   - Declared order, not sorted, as `resources` is today (SPEC-012).
   - A resource's `default_max_expr` and thresholds are not served, as how an
     ability resolves is not (SPEC-012).
10. **D10. The agent gains the properties; the guide keeps no copy.** A
    Go-registered MCP read tool, `get_ruleset`, returns `GET /api/ruleset`'s
    answer verbatim through `readMetadata`, as `get_join_link` and
    `get_participants` return theirs, and joins `GoRegisteredToolNames`. Each
    guide's prose on what one property means moves into that property's
    `description`, and the guide says where descriptions are read.
    `get_ruleset_guide`'s description points to `get_ruleset` for them.
    - **Forced by one home** (2026-09-23): a guide that keeps its prose makes
      every description a copy.
    - **Rejected:** `cmd/vtt/mcp.go` appending a rendered property list to
      `RulesetGuide`. It is a second shape of `/api/ruleset`'s facts, and
      `get_ruleset_guide` would stop returning `guide.md` verbatim, which
      `TestMCPRulesetGuideWithAndWithoutRulesetFlag` requires.
    - It reads the server's ruleset, the one `Resolve` uses, and works
      without `vtt mcp --ruleset`.
    - The agent gains `/api/ruleset`'s abilities and conditions with it
      (principle 35).
11. **D11. The window names through `Namer`.** `Namer` gains `property(key)`:
    the display name `/api/ruleset` gives a declared key, else the key, as
    `ability` names an ability. The chip's name comes from it in
    `tokensOnScene`, the Story line from it in `describe`, and the cost from
    it in `renderPlayerPanel`. `app.ts` carries `properties` from
    `fetchRuleset` beside `abilities`; `ViewExtras` gains it, optional as
    `abilities` is; `renderPlayerPanel` takes it as an optional trailing
    parameter, so its callers stay. Chips stay sorted by key.
    - The fallback is forced by G3, and by the tests that pin a key when no
      list is given: spectator-view's "every resource shows as a current/max
      chip, named on hover" and qa-what-a-window-shows' Story lines. They pass
      unedited.
    - An ability's cost always names a declared resource in a real window:
      abilities and properties arrive in one `fetchRuleset` answer, and
      VTT-613 refuses a cost naming an undeclared one. So VTT-623 claims no
      cost; spectator-view's "an affordable resource ability names its cost
      without the shortfall note", which expects `vigor 1` with no list
      given, passes unedited all the same.
12. **D12. Fixtures keep their directories.** `invalid-v2` and the rest keep
    their names; renaming them is not this ticket's. A preference. The new
    checks' cases are written inline by a helper, in new test files held to
    the default comment ceiling.
13. **D13. Not this ticket:** visibility; the sheet form; the other types;
    `rules.Resolve`'s refusals, which keep their keys and their words
    "attribute" and "defense"; how a resource behaves on `/api/ruleset`; a
    condition's naming in the window, since a condition is not a property;
    `add_actor`'s keys against the ruleset, principle 26's ticket.

**Signed off by the owner on 2026-11-05**, D1's text, D4, D9 and D10
among it. Noted beside the sign-off:
- D9 keeps `resources`. Deleting it under the 2026-09-04 ruling would break
  principle 5 by the owner's decision, and VTT-114 would be withdrawn.
- D10 adds an MCP tool, which the ticket's "What it touches" item 4 names
  since sign-off.
- `/api/ruleset` serves every property's definition to every role. Whether a
  property the visibility ticket makes the DM's is listed to a player is that
  ticket's question.
- `Resolve`'s refusal "missing defense" keeps a word the format no longer has.

## Requirements

Allocated at Task 0 with `requirement-id`, after the sort and its reading
review, each `**OPEN — no test yet**` until its test lands. The wording is
the register's, `docs/requirements.md`, and nowhere else; this table says
what each of this plan's labels became and which break turns it red. The
sort split R1, R2, R5, R6 and R8 into one rule a row.

| R | Ticket item | Id | Red under |
|---|---|---|---|
| R1 | 2 | VTT-610, a property with no name or description | K1, K2 |
| R1 | 2 | VTT-611, a type the engine does not run | K3 |
| R1 | 2 | VTT-612, a key two properties declare | K4 |
| R2 | 2 | VTT-613, an undeclared key at a site of D6 | K23 to K27 |
| R2 | 2 | VTT-614, a property read as the other type at a site of D6 | K5 to K10, K28, K29 |
| R3 | D2 | VTT-615, a number property with a resource's fields | K12 |
| R4 | 4 | VTT-616, any `format_version` but `"3"` | K11 |
| R5 | 3 | VTT-617, a statblock key the ruleset does not declare | K32 |
| R5 | 3 | VTT-618, a statblock key given the other type's value | K13, K14 |
| R6 | 5 | VTT-619, every property, in order, in its words | K15, K16, K33 |
| R6 | 5 | VTT-620, `properties` empty, never null | K17 |
| R7 | D10 | VTT-621, `get_ruleset` returns a 200's body verbatim | K18 |
| R8 | 6 | VTT-622, a declared resource by its display name | K19 to K22 |
| R8 | 6 | VTT-623, an undeclared resource by its key | Task 3's other direction |
| R9 | | VTT-624, succeeding VTT-283: the bound on a key | K30 |
| | | VTT-625, succeeding VTT-287: the schemas state it | K31 |

- **Withdrawn:** VTT-283, false as worded; VTT-287, ambiguous as worded once
  a property's `name` is unbounded and its key is bounded.
- **VTT-617** is behaviour `adventure.Load` already has: the
  `undeclared-attribute` and `undeclared-resource` rows of
  `TestLoadInvalidFixtures` hold it, and Task 1 gives that test the id.
- **Kept, wording still true:** VTT-114, VTT-115, VTT-118, VTT-284, VTT-288,
  VTT-475, VTT-476, VTT-477, and the rules `TestLoadInvalidFixtures` holds.

Refused by the sort, one line each:
- the ticket's "every property can be fetched through `/api/ruleset`":
  VTT-619 already, not a second row;
- the ticket's "never by its key": VTT-622 at its sites; not true of the
  fold-failure reason, which the ticket no longer claims;
- D1's "the version is read before the shape": how; VTT-616 observes it;
- D2's absent `visibility`: the strict decode already refuses an unknown
  field, and a row naming visibility would be withdrawn by the next ticket;
- D3's where a value lives, D4's param kinds and D5's `Ruleset` shape: how,
  observed through VTT-613 and VTT-614; the new specification records them;
- D7's unbounded name and description: a decision with nothing to go red,
  recorded by SPEC-018;
- D9's kept `resources`: VTT-114 holds it, unchanged;
- D10's "the guide keeps no copy": prose in two files, read by Task 2's
  review, not a rule of the running system;
- D12's fixture names, the keys staying as they are, the chips' order and
  the single-actor widening: constraints or preferences on this work;
- items 1 and 7: checks this work keeps green, not rules it adds.

| Ticket item | Observed by |
|---|---|
| 1 | `TestConformanceOverRulesetsGlob` and the goldens, unedited |
| 2 | the tests of VTT-610 to VTT-614 |
| 3 | `TestLoadInvalidFixtures`' undeclared rows, unedited; VTT-618's test |
| 4 | VTT-616's test |
| 5 | the tests of VTT-619 and VTT-620 |
| 6 | the tests of VTT-622 and VTT-623 |
| 7 | Task 4 |

## The existing tests that change

Each change is a manifest's shape, an atom's param kind, a `Ruleset` literal
or a schema path, unless named otherwise:
- `internal/rules`: `load_test.go` (its assertions on a duplicate, reserved
  or ill-charset attribute, defense or resource name now name a property's
  key; its format v1 case reads D1's text), `load_dice_test.go`,
  `compile_test.go` (its `FormatVersion` assertion reads `"3"`),
  `schema_test.go`, `id_bound_internal_test.go`, `qa_ruleset_bound_test.go`,
  `qa_ability_outcomes_test.go`, `resolve_test.go`, `resolve_int32_test.go`,
  `resolve_v2_test.go`, and every fixture under `testdata`.
- `internal/rules/conformance`: every fixture under `testdata`.
- `internal/adventure`: `qa_cmd_id_bound_test.go`, `qa_id_bound_test.go`, the
  fixture rulesets under `testdata/ruleset` and
  `conformance/testdata/rulesets`.
- `internal/gateway`: `qa_ability_sight_test.go`, `qa_cmd_id_bound_test.go`,
  `qa_id_bound_test.go`, `qa_ruleset_bound_test.go`.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** sign-off, with D1's text settled; run the `requirements` sort over
  R1 to R9 and report what it refuses; allocate the ids; withdraw VTT-283,
  succeeded by R9's id, and VTT-287, which the reading review found
  ambiguous; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean; the rows exist; the chain passes.

### Task 1: the format (R1 to R5, R9; D1 to D8, D12; items 1 to 4)

- **Files:** `internal/rules/format.go`, `load.go`, `compile.go`,
  `resolve.go`, `schema/ruleset.schema.json`, `schema/atom.schema.json`,
  `conformance/conformance.go`; `internal/adventure/load.go`,
  `internal/adventure/format.go`'s comment; `internal/gateway/metadata.go`,
  which builds `resources` from the resource properties and nothing more
  yet; `rulesets/*/ruleset.json` and `rulesets/*/atoms`, each property given a
  name and a description written from its guide's prose; every fixture and
  test of "The existing tests that change"; new test files
  `internal/rules/property_test.go` and `internal/adventure/statblock_type_test.go`.
- **New tests**, each with a literal name and its id line:
  - **R1:** `TestAPropertyIsRefusedWithoutANameADescriptionOrARunType`, a
    table: no `name`, an empty `name`, no `description`, an empty one, the
    type `category`, an empty type, no type; and
    `TestAKeyDeclaredTwiceIsRefused`, a number and a resource sharing a key
    among its cases. Each asserts the
    file, the field, the key and the fault.
  - **R2:** `TestAnUndeclaredKeyIsRefusedWhereverItIsNamed` and
    `TestAPropertyReadAsTheOtherTypeIsRefused`, a case per site of D6, each
    asserting the type in the fault.
  - **R3:** `TestANumberPropertyCarriesNoResourceField`.
  - **R4:** `TestARulesetInTheReplacedFormatIsRefusedWithWhatToDo`: a copy of
    today's `internal/rules/testdata/valid/ruleset.json` is refused with D1's
    text whole; `"1"`, an empty value, the number `3` and an absent field
    read the same, each naming its value as written.
  - **R5:** `TestAStatblockGivingAPropertyTheOtherTypesValueIsRefused`, in
    each direction.
- **Ids re-pointed:** every `// VTT-283` to VTT-624, the one in
  `internal/gateway/qa_ruleset_bound_test.go` among them; every
  `// VTT-287` to VTT-625; `TestLoadInvalidFixtures` in `internal/adventure`
  gains VTT-617, and the register cites it.
- **Order:** the tests, red (G6: each red today, R4's for the right reason);
  the code; the fixtures; the breaks.
- **True when:** `go test ./internal/rules/... ./internal/adventure/...
  ./internal/gateway/ ./internal/mcp/ ./cmd/vtt/` passes with
  `TestConformanceOverRulesetsGlob`, the goldens and `TestLoadInvalidFixtures`
  unedited; `go vet`, `semgrep` and `go-arch-lint` are clean; `share_of`
  holds every touched file; `git diff main -- contract client` prints
  nothing.
- **Breaks**, each undone by its inverse edit:
  - **K1:** the display-name check removed. R1's name cases red.
  - **K2:** the description check removed. R1's description cases red.
  - **K3:** the type check accepts any non-empty string. R1's `category` case
    red.
  - **K4:** the duplicate check keyed per type. R1's shared-key case red.
  - **K5:** `compileTwoActorExpr`'s `@` check reads the keys of every type.
    R2's `@` on a resource red.
  - **K6:** its `#` check reads every key. R2's `#` on a number red.
  - **K7:** `bindAtom`'s `number` kind accepts any declared key. R2's binding
    case red.
  - **K8:** `compilePower`'s cost check accepts any declared key. R2's cost
    case red.
  - **K9:** `compileEffects`' `resource_change` check accepts any declared key.
    R2's change case red.
  - **K10:** `checkExprRefs` reads every key for `@`. R2's `default_max_expr`
    and `when` cases red.
  - **K11:** the probe accepts `"2"`. R4's test red: the manifest then fails
    on `decodeStrict`'s text, which lacks D1's words.
  - **K12:** a number property's resource fields accepted. R3 red.
  - **K13, K14:** `loadActors` checks `attributes`, then `resources`, against
    every declared key. R5's matching direction red.
  - **K23 to K27:** the undeclared check removed, one site at a time:
    `checkExprRefs`, `compileTwoActorExpr`, `bindAtom`, `compilePower`'s
    cost, `compileEffects`' `resource_change`. That site's undeclared case
    red.
  - **K28:** `checkExprRefs` reads every key for `#`. R2's `#` on a number in
    a `when` red.
  - **K29:** `bindAtom`'s `resource` kind accepts any declared key. R2's
    resource binding case red.
  - **K30:** the bound on a key dropped from the manifest's bounds check.
    VTT-624's tests red.
  - **K31:** the bound dropped from the key's description in
    `ruleset.schema.json`. `TestTheSchemasStateTheIDBound` red.
  - **K32:** `loadActors`' undeclared check removed.
    `TestLoadInvalidFixtures`' undeclared rows red.
  - The other direction, on the repaired code: the shipped rulesets and
    `valid-v2` load (K-controls in each table).
- **Then:** Phase 4a, Phase 4b, the fixes, a commit.

### Task 2: the read surface and the agent (R6, R7; D9, D10; item 5)

- **Files:** `internal/gateway/metadata.go`; `internal/mcp/ruleset_tool.go`,
  new; `internal/mcp/read_tools.go` (`GoRegisteredToolNames`);
  `internal/mcp/guide_tool.go`'s description; `rulesets/*/guide.md`; new
  tests `internal/gateway/ruleset_properties_test.go` and
  `internal/mcp/ruleset_tool_test.go`.
- **New tests:**
  - **R6:** `TestTheRulesetRouteAnswersEveryPropertyInTheRulesetsWords`:
    `newMetaFixture` with content whose declared order is not key order;
    every property of the loaded `Ruleset`, in its order, with its key, type,
    name and description; with no content, `properties` is `[]`. Red today:
    no such field.
  - **R7:** `TestGetRulesetReturnsTheRulesetRouteVerbatim`, against a test
    server as `get_participants`' tests use one.
- **True when:** `go test ./internal/gateway/ ./internal/mcp/ ./cmd/vtt/`
  passes, `TestMCPRulesetGuideWithAndWithoutRulesetFlag` unedited; each
  guide's per-property prose is gone and the guide says where descriptions
  are read; no guide says "format v2".
- **Breaks:**
  - **K15:** `handleRuleset` lists resource properties only. R6 red.
  - **K16:** `description` dropped from the entry. R6 red.
  - **K17:** `properties` left nil. R6's empty case red.
  - **K18:** `get_ruleset` reads `/api/ruleset/guide`. R7 red.
  - **K33:** `handleRuleset` sorts the properties by key. R6's order red.
- **Then:** Phase 4a, Phase 4b, the fixes, a commit.

### Task 3: the window (R8; D11; item 6)

- **Files:** `client/src/metadata.ts` (a `Property` type,
  `RulesetMeta.properties`), `client/src/view/naming.ts`,
  `client/src/view/grid.ts`, `client/src/view/spectator.ts`,
  `client/src/view/player.ts`, `client/src/app.ts`; `client/test/naming.test.ts`
  and `client/test/spectator-view.test.ts`; `cmd/vtt/webdist`.
- **New tests:** "a resource is named by the ruleset's display name, else its
  key" (`naming.test.ts`); "a resource chip, its Story line and an ability's
  cost name the resource by its display name" (`spectator-view.test.ts`). Red
  today: the window shows the key.
- **True when:** `bun test`, `task client:typecheck` and the client's lint
  pass; the existing chip and Story tests pass unedited; `task
  build:client` has rebuilt `cmd/vtt/webdist`, committed with the change.
- **Breaks:**
  - **K19:** `property` returns its key. R8's tests red.
  - **K20:** `tokensOnScene` names a chip by its key. The chip case red.
  - **K21:** `describe` names the resource by its key. The Story case red.
  - **K22:** the cost names its key. The cost case red.
  - The other direction: `property` returning `""` or `a creature` for an
    undeclared key turns the existing "vigor" tests red.
- **Then:** Phase 4a, Phase 4b, the fixes, a commit.

### Task 4: the gate

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger` once if
  a ceiling can come down; check free space and the load; run `task check`
  whole on the last code commit, detached, all output to a file; require the
  gate's own completion line and `check:ts-mutation`'s. Tell the owner which
  commit the client's stored mutation report measured and the tickets since;
  the full run is the owner's to ask for.
- **True when:** the gate is green (item 7).

### Task 5: the report

- **Do:** write `docs/reports/2026-11-05-a-ruleset-names-its-properties.md`
  with the `implementation-report` skill and give it its reading review.
- **It covers:** each "Done looks like" item with its observation; K1 to K33
  with what went red; Rule 9; the gaps and D1's settled text; Phase 4a's
  adjudications under their own heading; the gate.

### Task 6: the specifications

With the `specification` skill, from the report:
- **New:** how a ruleset declares its properties, the rules engine's first
  specification, at the next free number: the format, the types and where
  their values live (D3), the load rules (VTT-610 to VTT-618), the
  retired-format refusal, and that a display name and a description are unbounded (D7).
  Principles served: 13, 23, 24, 25 and 35.
- **SPEC-012:** `/api/ruleset`'s `properties`, `resources` kept (D9), and
  `get_ruleset` as its agent's reader (D10); principle 35's line.
- **SPEC-022:** the window names a declared resource by its display name, and
  the rest by its key (D11).
- **SPEC-018:** its Status and its `maxIDBytes` paragraph name a property's
  key; its search sentence on `load.go`'s length checks is re-run and
  rewritten to what it prints (G4); its list of ids carries VTT-624 and
  VTT-625 in place of VTT-283 and VTT-287.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer, and every id resolves.

Commit the report, then the specifications. Push and merge on the owner's
word.

## For Phase 4a

Hand the QA agent the rows VTT-610 to VTT-625 and VTT-114, the new
specification as drafted, and the surfaces as signatures and names, never the source:
- `go doc ./internal/rules`: `Load`, `Ruleset`, `Property`, `ResourceDef`,
  `ParamDef`; `go doc ./internal/adventure`: `Load`;
- `GET /api/ruleset` and its JSON; the MCP tool `get_ruleset`;
- `namer`, `describe`, `tokensOnScene`, `renderPlayerPanel`, `RulesetMeta`.

Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- its files are new `qa_` files beside the task's tests, held to the default
  comment ceiling; `go-arch-lint` runs in the hook;
- `semgrep` bans game words in every Go file under `internal/` and `cmd/`,
  tests included;
- a client test reads no file outside `client/` but a hashed one;
- known and not findings: visibility, the sheet form and other types absent;
  `Resolve`'s refusals keep keys and the words "attribute" and "defense";
  `resources` kept on `/api/ruleset`; a key the ruleset does not declare
  named by its key; a display name of spaces alone accepted; a former defense
  readable in a `default_max_expr`; `add_actor`'s keys unchecked;
- give each test a literal name and its rule's id;
- the tree is hashed before and after it.

## For Phase 4b

Hand the reviewer the diff with `git diff HEAD` and tell it:
- check each scope claim ("every", "anywhere", "only") names the search that
  bounds it, D6's sites above all;
- check each comment under `internal/`, `cmd/` and `client/src` is a warning
  or a pointer (rule 10), each citation a name (rule 8), and that no comment
  the change made false survives, the list under the constraints first;
- check each shipped property's description against the guide prose it
  replaced, and that no guide keeps a copy;
- check D1's text against what sign-off settled, word for word, and that
  `git diff main -- contract` prints nothing.
