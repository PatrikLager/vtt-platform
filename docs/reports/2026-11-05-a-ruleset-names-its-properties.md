# A ruleset names its properties in the table's words: the change

**Ticket:** `docs/superpowers/specs/2026-11-05-a-ruleset-names-its-properties-design.md`.
An agent that did not write it verified it and passed it with gaps.
**Plan:** `docs/superpowers/plans/2026-11-05-a-ruleset-names-its-properties.md`,
written by that verifier and amended by Task 0's review.
**Where it came from:** `docs/adr/012-the-rules-contract.md`, its build
order, step 1, the first ticket: "the vocabulary". The owner, 2026-11-05:
"kör, börja med vokabulären".
**The owner's rulings:**
- 2026-11-05, scoping the ticket: visibility is its own ticket, directly
  after this one, with the projection withholding by it; the sheet form is
  the next ticket's; the only types are number and resource; the work is one
  ticket.
- 2026-11-05, at sign-off ("godkänd plan"): `format_version` "3", read
  before the shape, with the refusal text as the plan proposed it; the atom
  param kinds `attribute` and `defense` become `number`; `/api/ruleset` gains
  `properties` and keeps `resources`; a new MCP tool, `get_ruleset`, serves
  the descriptions the guides' prose gives up.
- 2026-11-05, on QA's finding that a `condition`-kind binding must be an
  identifier ("släpp det"): no ticket, no candidate.

**The last code commit is `03aa9e1`.** Every code reference below is to that
tree unless it names another commit.

## The period, in commits

| Commit | What it holds |
|---|---|
| `704e1e5` | Task 0: the ticket, the plan, VTT-610 to VTT-625, VTT-283 and VTT-287 withdrawn |
| `e61614b` | Task 1: format "3", the property list, the type checks, the adventure statblock check, every fixture and inline ruleset moved; VTT-626 to VTT-631 |
| `e6addd6` | Task 2: `/api/ruleset`'s `properties`, the MCP tool `get_ruleset`, the guides; VTT-632, VTT-633 |
| `b439bc8` | Task 3: the window names a resource by its display name; the rebuilt `cmd/vtt/webdist` |
| `24eed65` | Task 4: the comment ledger, written once by `check-comments.py --write-ledger` |
| `feb894a` | Task 5's review: the doc comments that review found false, rewritten, and the ledger written again |
| `03aa9e1` | Task 5's second reading: the test docs, fixture guides and schema descriptions still in format v2's words, rewritten; the key-charset tests renamed and the defense case deleted; and the ledger written again |

## What the code does now

- **The format.** A `ruleset.json` is `format_version` "3" and declares
  `properties`, each with `key`, `type` (`number` or `resource`), `name` and
  `description`. A resource property may carry `default_max_expr` and
  `thresholds`; a number property carries neither. `rules.Ruleset.Properties`
  holds them in declared order, each a `rules.Property` whose `Resource` is
  set exactly for a resource; `Ruleset.Property(key)` looks one up.
- **The version.** `loadManifest` in `internal/rules/load.go` reads
  `format_version` alone before the strict decode, through
  `checkFormatVersion`, and refuses anything but the string "3" with
  `formatVersionWant`, which says what format "3" declares and names
  ADR-012. A file that is not a JSON object is left to `decodeStrict`,
  except `null`, which reads as a file with no `format_version`.
- **The checks.** `loadManifest` refuses a property with no name or
  description (`loadProperty`), an unknown type, a key declared twice or a
  key an expression could not read (`checkPropertyKey`), and a number
  property carrying a resource's field. A key is checked by type through
  `checkPropertyRef`, which `git grep -n checkPropertyRef -- internal/rules
  ':!*_test.go'` finds called at these places and no other: a resource's
  `default_max_expr` and threshold `when` (`checkExprRefs`), an ability's
  cost (`compilePower`), a `number` or `resource` param binding
  (`bindAtom`), and the `roll`, `vs`, resource change and `delta_expr` of a
  composed atom (`compileTwoActorExpr`, `compileEffects`).
- **Adventures.** `checkStatblockKey` in `internal/adventure/load.go` refuses
  a statblock key the ruleset does not declare, or one under the other
  type's map.
- **The read surface.** `handleRuleset` in `internal/gateway/metadata.go`
  answers `properties` in declared order, `[]` with nothing loaded, and
  builds `resources` from the resource properties. `get_ruleset` in
  `internal/mcp/ruleset_tool.go` returns that body verbatim through
  `readMetadata`. Each guide under `rulesets/` points to it for what each
  property means.
- **The window.** `Namer.resource` in `client/src/view/naming.ts` names a
  key by its resource property's display name, else by the key. The chip's
  `label` (`tokensOnScene`), `describe`'s `resourceChanged` line (the Story
  and the Events panel) and `renderPlayerPanel`'s cost use it. `app.ts`
  carries `properties` from `fetchRuleset` beside `abilities`.

## Done looks like, answered

1. `[x]` Every ruleset under `rulesets/` declares every property with a
   key, a type, a name and a description, and loads:
   `TestConformanceOverRulesetsGlob` passes, and neither it nor any golden
   under `rulesets/*/goldens` or `internal/rules/conformance/testdata/*/goldens`
   changed between `38d11bd` and `03aa9e1` (`git diff` over those paths and
   `internal/rules/conformance/conformance_test.go` prints nothing). The
   engine's types are `rules.PropertyNumber` and
   `rules.PropertyResource`; any other is refused
   (`TestAPropertyIsRefusedWithoutANameADescriptionOrARunType`, its
   `category` case).
2. `[x]` In `internal/rules/property_test.go`:
   `TestAPropertyIsRefusedWithoutANameADescriptionOrARunType`,
   `TestAKeyDeclaredTwiceIsRefused`,
   `TestAnUndeclaredKeyIsRefusedWhereverItIsNamed` and
   `TestAPropertyReadAsTheOtherTypeIsRefused`, a case per place a key is
   read, each asserting the file, the field, the key and the fault. On
   `704e1e5`'s tree, before Task 1's code, they did not build, since
   `rules.Property` did not exist; the breaks below are the red that shows
   each check. "Wherever" is bounded to the places listed above.
3. `[x]` `TestLoadInvalidFixtures` in `internal/adventure/load_test.go`
   passes with its cases unedited (only its id line gained VTT-617), and
   `TestAStatblockGivingAPropertyTheOtherTypesValueIsRefused` names the type
   case in each direction.
4. `[x]` `TestARulesetInTheReplacedFormatIsRefusedWithWhatToDo` loads
   format v2's own `testdata/valid` manifest and asserts the whole refusal,
   which tells the author what format "3" declares. "1", an empty value and
   a number are refused with `unsupported value`, the value as written and
   the same text; an absent field with `is absent: ` and that text.
5. `[x]` `TestTheRulesetRouteAnswersEveryPropertyInTheRulesetsWords` in
   `internal/gateway/ruleset_properties_test.go`: every property, in the
   order `ruleset.json` itself declares them, with its key, type, name and
   description; red before Task 2's code, where the answer had no
   `properties`.
6. `[x]` `client/test/spectator-view.test.ts`'s "a resource chip, its Story
   line and an ability's cost name the resource by its display name" and
   `client/test/app.test.ts`'s "the ruleset's properties reach the chip, the
   Story and the cost", which goes through `boot`; red before Task 3's code,
   where the window showed the key.
7. `[x]` `task check` exits 0 on the branch. It exited 0 on `24eed65`, on
   `feb894a` and on `03aa9e1`, 2026-11-05; the last run's log records its
   HEAD and an empty `git status` before it started. On each, the Go
   mutation gate found no unadjudicated survivor; this branch deleted
   the adjudications whose code it removed and added none. The mutants it
   reported as timed out, counted as killed and not measured, sit in code
   this branch did not change: `internal/perceive`'s projection,
   `internal/sight`, and `Server.Run` in `internal/mcp`. On `feb894a` and on
   `03aa9e1` the gate reused those packages' verdicts, measured on the
   `24eed65` run, since nothing in their dependency closure differed; the
   packages whose closure did differ, `internal/rules`,
   `internal/adventure`, their `conformance` packages and
   `internal/gateway`, were mutated afresh on each. `check:ts-mutation` judged
   nothing, since the worktree holds no stored report (`reports/` is
   gitignored). The one in the main checkout measured `526885b`; the tickets
   since are those `docs/reports/2026-11-03-only-a-door-opens.md` lists,
   ending with its own, and this one. `task check:ts-mutation:full` measures
   it when the owner asks.

## What the rules became

Each row sits under its R label in the plan's Requirements table, and each
label under the ticket's rule it serves.

| The ticket's rule | Rows it earned |
|---|---|
| every property declared with a run type, a display name and an explanation | VTT-610 (name, description), VTT-611 (type), VTT-612 (a key declared once) |
| a read of an undeclared property, or of a property as the other type, is refused; so is such a statblock | VTT-613, VTT-614, VTT-617, VTT-618 |
| every property can be fetched through `GET /api/ruleset` | VTT-619, VTT-620 (`[]` with no ruleset loaded) |
| the chip, the Story line and the cost name a declared property by its display name | VTT-622, VTT-623 (an undeclared resource by its key) |
| VTT-283 succeeded by the same bound on a key | VTT-624 |

The plan's own rows, which no rule of the ticket's produced: VTT-615 (a
number carries no resource's field, D2), VTT-616 (the retired format, item
4), VTT-621 (`get_ruleset`, D10) and VTT-625, succeeding VTT-287, which Task
0's review found ambiguous.

Allocated during the work, each in the commit the table of commits names:
VTT-631 (`resources` built from the resource properties), for a filter Task
1's review found untested; and, for loader, route and tool rules QA quoted
that no row held, VTT-626 (a key's charset, `caster`/`target`), VTT-627 (an
unknown manifest field), VTT-628 (a threshold's fields), VTT-629 (a
condition reference), VTT-630 (bare or scoped by position), VTT-632 (every
role reads the route) and VTT-633 (a refused read is an error). The sort's
refusals are listed in the plan, under Requirements.

## Phase 4a

One QA agent per task, given the rows, a drafted specification and the
exported surface, never the source, the diff or the existing tests. Every row
passed in Tasks 1 and 2; in Task 3 QA found the behaviour defect below.

### QA adjudications

- Task 1, QA finding 1: spec ambiguity — "every refusal has the form
  `rules: <path>: field ...`" is false of the strict decode, which reads
  `json: unknown field`; the draft scoped the form to the rows' refusals.
- Task 1, QA finding 2: spec ambiguity — a `condition`-kind binding must be
  an identifier, so a hyphenated condition id can be named in an atom but not
  bound; predates this ticket; dropped by the owner (2026-11-05).
- Task 1, QA finding 3: no change — a type asserted by presence cannot tell
  which type is the key's, since the messages name both; the implementer's
  tests assert the whole phrase.
- Task 1: QA's tests of the "not checked by decision" cases and of a
  display name of spaces were left out, because each pins an absence chosen
  for cost and would go red on correct work.
- Task 2: no defect. The test that a resource's default maximum and
  thresholds are not served was left out on the same ground; the
  comparison of each entry was loosened from the whole object to its key,
  type, name and description, since the visibility ticket adds a field.
- Task 2: spec gap — `get_ruleset` refuses a non-200 as an MCP protocol
  error, not as an `isError` result; `readMetadata` has that shape for the
  door tools too, and VTT-633 is met by either.
- Task 3, QA finding 1: behaviour defect — a property of type `number`
  named a resource of the same key on the chip, the Story line and the cost,
  and an actor can carry such a resource since `add_actor`'s keys are not
  checked. Fixed: `Namer.resource` reads resource properties only; VTT-622
  and VTT-623 say "as a resource property".

## Phase 4b

One reviewer per task, on that task's diff; one on Task 0's records; one on
this report, which read it again each time its findings were applied; and
one on each fix those readings caused, before its commit. Every Must and
Should was taken. Of the
Nits, those left are a doc line on `ViewExtras.properties`, refused because
`client/src/view/spectator.ts` sits at its comment ceiling; dated "format v2"
wording in long comment blocks (Deviations); and the edges under What could
not be established. The findings that changed code or rows:

- Task 0: VTT-613 and VTT-614 claimed "anywhere" (narrowed); VTT-287 was
  ambiguous (withdrawn); breaks were missing for several checks (added).
- Task 1: mutation adjudications named code this change deleted
  (deleted; the plan's P14 had said none sat in a touched file); the
  `resources` filter had no test (VTT-631 and its test); false descriptions
  in the schemas and in dnd45e-minimal's `dex` (corrected); the schema
  walker had stopped probing thresholds because `testdata/valid` now listed
  a number first (`pool_a` moved first); VTT-627 claimed more than
  `encoding/json`'s case-insensitive match holds (reworded); the version
  probe printed its anonymous struct on a non-object file (moved into
  `loadManifest`).
- Task 2: QA's MCP tests built and ran `vtt serve` as a child process
  inside `internal/mcp`, which the mutation gate reruns per mutant (moved to
  `cmd/vtt` over `composeServer` in process); VTT-633 was wider than its
  test, a redirect being followed (narrowed to 4xx and 5xx); the route tests
  compared order with the loaded `Ruleset`, not with the file (they now read
  `ruleset.json`); "every property an actor carries" was false while
  `add_actor` is unchecked ("declares"); the dnd guide kept a copy of
  `max_hp`'s meaning (a pointer).
- Task 3: nothing tested `app.ts` carrying the properties into the views
  (a test through `boot`); the Events panel names a resource too (VTT-622
  widened, a test); the cost's `?? ""` was a new surviving mutant (a test);
  `properties` now starts `undefined` rather than as an array literal.
- Task 5, this report's review: doc comments the change had left false,
  still naming a manifest's attribute, defense and resource names, on
  `substTemplate` in `internal/rules/compile.go` and on `Refs` and
  `isValidIdentName` in `internal/rules/expr.go`; and `loadActors`' in
  `internal/adventure/load.go`, which checked statblocks "against the
  ruleset's declared names" where a key is now apart from a display name.
  The review of that rewrite, before it was committed as `feb894a`, found
  that `substTemplate`'s new doc had dropped `bindName` from the load rules
  its output is safe under: with `bindName`'s IDENT check removed, a
  condition id holding an operator spliced into a `vs` and every test
  stayed green. It found too that `loadAtoms`' doc still counted the param
  kinds format v2 had; that test docs in `internal/rules/compile_test.go`
  and `internal/rules/schema_test.go` still called a `number` param
  attribute-kind or defense-kind; and that `baseProperties`' doc in
  `internal/rules/property_test.go` called it `testdata/valid`'s property
  list, which lists `pool_a` first. All were corrected in `feb894a`. No
  test was added for `bindName`'s check: it is the check QA's Task 1
  finding 2 is about, which the owner dropped; and a ruleset's author writes
  its expressions anyway, so a binding that splices one crosses no boundary
  (the owner's ruling on near-theoretical edges of small consequence,
  2026-10-21). The doc now names the check.
- Task 5, the report's second reading and the sweep it prompted:
  `internal/rules/load_test.go`'s
  key-charset tests were still named and documented for a manifest's
  attribute, defense and resource names, and the `binding-kind-mismatch`
  and `reserved-scope-word-attribute` fixture guides under
  `internal/rules/testdata/invalid-v2` and the description of a resource
  change's `resource` in `atom.schema.json` still described format v2's
  manifest. The review of their rewrite, before its commit, found the same
  in `ability.schema.json`'s `bind` description, which named the param
  kinds the loader now refuses; in the guide of `testdata/valid` and its
  copies under `testdata/invalid`, which gave `strike-roll`'s param an
  attribute's kind; in `resource-default-max-undeclared-ref`'s guide, which
  quoted an attributes list; in `internal/adventure`'s fixture ruleset guide
  and `loadFixtureRuleset`'s doc; and in
  `internal/rules/resolve_v2_test.go`'s header. All were corrected in
  `03aa9e1`: the tests renamed for a number and a resource key, the defense
  case deleted as a second number case, the rest reworded. The gate was run
  again on it (item 7).

## The deliberate breaks

Each applied, its check run, then undone and the file compared byte for byte.
Every one went red.

- **Task 1:** the name, description, type and duplicate-key checks (K1 to
  K4); each site's type check (K5 to K10, K28, K29) and undeclared check
  (K23 to K27); the probe accepting "2" (K11) and skipping every object; a
  number carrying either resource field (K12); the key's bound (K30) and
  the schema stating it (K31); the statblock's type checks (K13, K14) and
  undeclared check (K32); the `resources` filter negated and sorted.
- **Task 2:** `properties` limited to resources (K15), its description
  dropped (K16), left nil (K17), sorted by key (K33); `get_ruleset` reading
  another route (K18) and missing from `GoRegisteredToolNames`; a `Load`
  that reversed the properties.
- **Task 3:** the namer returning the key (K19); the chip, the Story line
  and the cost each naming the key (K20 to K22); the type filter dropped;
  an undeclared key named ""; each line of `app.ts`'s wiring removed; the
  Events panel given a namer without properties; the cost's fallback.

## Rule 9: how MapTool does this

MapTool's `TokenProperty` (`~/dev/RPTool/maptool` at `f4b7fef6c`; the
plan's P1 named RPTool `63862e4`, which holds no MapTool source) has a
`name`, an optional `displayName`, a `shortName`, `ownerOnly` and `gmOnly`,
a `defaultValue` and `highPriority`, grouped per token type in
`CampaignProperties.tokenTypeMap`; every value is text. Borrowed: the display
name beside the key, and the reader that names a property by its display name
else its key. Deferred: `ownerOnly` and `gmOnly` are ADR-012's visibility, the
next ticket's; the grouping is the sheet form. Not borrowed: `shortName` (the
chip names on hover), `highPriority` (a "primary" property is a genre
assumption), `defaultValue` (a resource's default maximum is
`default_max_expr`), and untyped values.

## The plan's gaps

- G1, G2, G4 and G5 were closed in the ticket before sign-off; G3 (a key
  no property declares cannot be named by a display name) by D11, which names
  anything undeclared by its key, and by VTT-623's wording.
- G6 (items 2 and 3 red today for the wrong reason): held by the breaks
  K5 to K10, K13, K14, K28 and K29, each of which turns a type-case test red
  on the right assertion.
- G7 (items 5 and 6 could not be run before their tests): each test was run
  red before its task's code.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| D11: `Namer.property(key)` | `Namer.resource(key)`, reading resource properties only | QA's Task 3 finding: a number property named a resource of the same key |
| The plan's rows, VTT-610 to VTT-625 | Also VTT-626 to VTT-633 | Review and QA found rules no row held; What the rules became names each and why |
| D1: one text for every retired value | An absent field reads `is absent: <what>`; every other value `unsupported value <as written>: <what>` | An absent field has no value to name |
| The constraints: `resolve.go`'s doc on stats rewritten | Left, with the file doc in `compile.go` and `format.go`'s docs on `Atoms` and `AtomDef`, which still say "format v2" | Each is one comment block over `check:comments`' bound, so a changed line is refused unless the block is cut to the bound. `Resolve`'s doc describes the wire's attribute map, which stays (D3), and its words are step 2's (D13); the "format v2" docs are dated rather than false, since atoms and compositions are format v2's design and format "3" keeps it |
| "The existing chip and Story tests pass unedited" | `client/test/grid.test.ts`'s "EVERY resource is shown, in a stable order, with current and max" gained `label: "vigor"` | It compares the whole chip, and the chip gained a field |
| D12: fixtures keep their directories, content converted | Also: `bad-format-version` reads "99" (it read "3"), `minimal-load-error` is format "3" with no id (under "1" the probe refused the version first), `format-version-1` stays in format v1's shape, and `testdata/valid` lists `pool_a` first | Each fixture keeps the one fault it exists for; the reorder lets the schema walker reach the threshold fields |
| Task 2's QA tests in `internal/mcp` | In `cmd/vtt` | `go-arch-lint` keeps `internal/mcp` from importing the gateway, so QA ran a child process inside a mutated package |
| Task 4: `--write-ledger` once, then the gate on the last code commit | `feb894a` and `03aa9e1` after it, each writing the ledger again, with the gate run on each | This report's readings found docs the change had left false, which were corrected in the code rather than noted here; each correction moved comment shares the ledger holds and changed the tree the gate had passed |

Comments cut or rewritten, whose reasoning left the code (SPEC-010):
`AdventureActor`'s doc in `internal/adventure/format.go` kept that its Kind
is the author's, required at load, that it has no controller, and that the
kind is carried as the wire enum; the argument it carried was that who
drives a character is a fact about the table, unknown when the file is
written, while what a creature is belongs to the content and its author, so
the file is the place to say it and Load refuses an actor that does not.
`buildFixtureState`'s, `runCompiledGoldens`' and the compiled-goldens
header's docs in `internal/rules/conformance/conformance.go` lost the history
of the format v1 sunset; `TestLoadInvalidFixtures`' doc in
`internal/rules/load_test.go` lost a migration account, including its claim
that a composition's `usage.limited.resource` is never cross-validated, which
had been false since `compilePower` checked it. In `feb894a`,
`substTemplate`'s doc lost its worked example of a hygienic splice, which
`TestCompileHygienicSplice` pins, and its account of the hazards a binding
poses, a value escaping its parentheses and a name taking a scope position;
it keeps the load rules its output is safe under. `Refs`' lost the history
of format v1's literal dice; `isValidIdentName`'s, its example of a
hyphenated name lexing as a subtraction, its argument against a second
hand-written regex beside the lexer, and a pointer to a task report under
the gitignored tree; `loadActors`' a paragraph calling a kind the one field
whose absence is refused rather than interpreted, which pointed to the check
for why; `loadAtoms`' its catalogue references. In
`internal/rules/compile_test.go`, `TestLoadValidV2Fixture`'s doc lost its
listing of what `testdata/valid-v2` composes, which the fixture's files
hold, and the history of the review fixes behind tag-team and ward-shift; a
floating comment on `mustParse`, which `internal/rules/expr_test.go`
declares, was deleted. In `03aa9e1`, a floating comment in
`internal/rules/load_test.go` on `TestLoadRejectsTargetingMissingRange` and
`TestLoadAcceptsTargetingZeroRange`, deleted with format v1's ability-level
`targeting`, was deleted: it said their coverage lives on in
`TestSchemaRequiredFieldsMatchLoaderEnforcementV2`, which walks the atom
schema's targeting contribution, and in `testdata/valid`'s self-delivery
and melee-delivery atoms and `rulesets/tavern-brawl`'s sober-up, which
binds reach 0. `TestLoadRejectsANumberKeyThatIsNotAnIdentifier`, formerly
`TestLoadRejectsInvalidAttributeIdentifier`, lost its argument for refusing
such a key at load rather than with an "undeclared" error at its first
reference. The deleted `TestLoadRejectsInvalidDefenseIdentifier`'s doc
argued that a defense name kept the identifier charset, though no
expression read one, so that an author need not remember which kind of name
allowed a hyphen. `internal/rules/resolve_v2_test.go`'s header lost the
plan tasks it came from, which abilities of `testdata/valid-v2` its tests
exercise, what resolving a loaded composition proves beyond the hand-built
rulesets, its reasoning that `compile_test.go` was another task's file, and
its pointer to `TestResolveV2VsExpressionWithDiceRecordsInOrder` as the
hand-built case; it keeps the warning not to add an ability or atom to
`testdata/valid-v2` for a case there. `loadFixtureRuleset`'s doc in
`internal/adventure/load_test.go` lost the fixture's id and its listing of
the fixture's properties, which the fixture's `ruleset.json` holds, and its
argument that the vocabulary is generic to match `internal/rules`'
fixtures.

## What could not be established

- **`readMetadata` follows a redirect.** `http.DefaultClient` follows a 302,
  so a gateway that redirected `/api/ruleset` would have `get_ruleset` return
  another route's body as content. The gateway never redirects that route:
  `git grep -nE 'http\.Redirect|StatusFound|StatusMovedPermanently|StatusSeeOther|TemporaryRedirect|"Location"' -- 'internal/gateway/*.go' 'cmd/vtt/*.go' ':!*_test.go'`
  prints nothing. The shape is shared with the door tools and predates this
  ticket. VTT-633 is worded to the statuses it holds.
- **Near-theoretical edges, left:** `null` for a number property's
  `default_max_expr` or `thresholds` loads (VTT-615 is held for a present
  value); a manifest whose only version key is `FORMAT_VERSION` is read as
  `format_version`, since `encoding/json` matches without regard to case.

## What was deliberately left out, and where it went

- **Visibility**: the next ticket, with the projection withholding by it.
- **The sheet form**: the ticket after, campaign data fits the ruleset
  (principle 26), which also checks `add_actor`'s keys against the
  properties.
- **The other types** of ADR-012: the ticket that first reads each.
- **`rules.Resolve`'s refusals**, which keep keys and the words "attribute"
  and "defense": ADR-012 step 2, which replaces the resolver.
- **The window's fold-failure reason**, which names a resource by its key: a
  fault report, by the ticket.
- **A number property in the window**: shown nowhere yet (`git grep -n -iE
  '\battributes\b' -- client/src` prints nothing, and `Namer.resource`
  reads resource properties only); the sheet form's ticket.
- **A hyphenated condition id bound to a `condition` parameter**: dropped by
  the owner, 2026-11-05.
