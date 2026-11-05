# A ruleset names its properties in the table's words

## The problem

**A ruleset's properties are bare keys.** `ruleset.json` lists `attributes`,
`defenses` and `resources` by key alone (`manifestJSON` and `resourceJSON` in
`internal/rules/load.go`), and `rules.Load` keeps them as
`Ruleset.Attributes`, `Ruleset.Defenses` and `Ruleset.Resources`
(`internal/rules/format.go`). A key has no type beyond the list that names
it, no name for the table and no explanation. The only words about them are
the prose under "## Attributes" in each ruleset's `guide.md`, which nothing
reads as data. Every value is an `int32` in `Actor.attributes` or an entry of
`Actor.resources` (`contract/vtt/v1/events.proto`), so an attribute and a
defense differ only in which list names them.

What a participant meets as a result:

- `GET /api/ruleset` (`handleRuleset` in `internal/gateway/metadata.go`)
  answers with the resources as bare keys and with no attributes or defenses
  at all. Nothing a participant can fetch says what a ruleset's properties
  are or mean (blueprint principle 35).
- The window names a resource by its key: a resource chip's title
  (`chip.title = r.name` in `client/src/view/spectator.ts`), a Story line for
  `resourceChanged` in the same file, and an ability's cost (the button's
  title in `renderPlayerPanel`, `client/src/view/player.ts`). A player
  reads `flurry_uses` where the ruleset could give a name (principle 13).

The readers of the lists are `crossValidateResources`
(`internal/rules/load.go`), `compileCompositions`
(`internal/rules/compile.go`), `evalThresholds` (`internal/rules/resolve.go`),
`buildFixtureState` (`internal/rules/conformance/conformance.go`), the
statblock check in `adventure.Load` (`internal/adventure/load.go`) and
`handleRuleset`. An atom's parameters are typed by the same split:
`validParamKinds` in `internal/rules/load.go` has the kinds `attribute` and
`defense`. `supportedFormatVersions` in `internal/rules/load.go` accepts only
format v2.

ADR-012 decided that a ruleset names every property and gives it a type, a
display name, an explanation and a visibility (its Decision §2), and that
format v2 is replaced outright. Nothing in the tree does any of that yet.

MapTool (CLAUDE.md rule 9) describes a token's property as a
`TokenProperty` (`maptool/src/main/java/net/rptools/maptool/model/TokenProperty.java`
in `~/dev/RPTool`): a name, a short name, a display name, `ownerOnly`,
`gmOnly` and a default value, grouped per token type in
`CampaignProperties.tokenTypeMap`. It has no type, since every value is text,
and no explanation. The display name and the per-property visibility are the
parts to borrow; the type and the explanation are this platform's.

## Done looks like

1. Every ruleset under `rulesets/` declares each property it has with a key,
   a type, a display name and an explanation, and loads. The engine runs a
   number type and a resource type, which are what a sheet can hold today.
   `TestConformanceOverRulesetsGlob` and the goldens under
   `rulesets/*/goldens` pass unchanged.
2. `rules.Load` refuses, naming the property and the fault, a ruleset with: a
   property with no display name or no explanation; a type the engine does
   not run; a key declared twice; a key no property declares, or a property
   used as a type it is not, wherever a resource, an ability or an atom an
   ability composes reads it. A test in `internal/rules` names each, and
   fails on today's tree.
3. `adventure.Load` refuses a statblock that names a key the ruleset does not
   declare, or gives a property a value of a type it is not. The tests that
   hold today's statblock check pass, and a test names the type case.
4. A `ruleset.json` in format v2's shape, with bare `attributes`, `defenses`
   and `resources` lists, is refused at load with a reason its author can act
   on. A test names it.
5. `GET /api/ruleset` answers with every property: its key, type, display
   name and explanation. A gateway test names it, and fails on today's tree,
   where attributes and defenses are absent.
6. The window names a resource by its display name on a chip's title, a
   Story line and an ability's cost. A client test names each, and fails on
   today's tree.
7. `task check` exits 0 on the branch.

Not part of done, by the owner's rulings of 2026-11-05 where dated:

- **Visibility.** ADR-012 gives each property a visibility. It arrives in its
  own ticket, directly after this one, together with the projection
  (SPEC-016) withholding a value by it, so that no visibility is declared
  that nothing enforces (2026-11-05).
- **The sheet form.** It arrives with the next ticket, campaign data fits the
  ruleset (principle 26), which is the first to read one (2026-11-05).
- **The other types** of ADR-012's list arrive with the ticket that first
  reads each (2026-11-05).
- **The refusals `rules.Resolve` gives an ability** keep naming keys and
  actor ids. The resolver is replaced in ADR-012's step 2, and its refusals
  are written then.
- **The reason a window gives when it cannot fold its log** keeps naming a
  resource by its key: it is a fault report, not the table's words.

## Rules this puts on the system

- A ruleset is refused at load unless every property it has is declared with
  a type the engine runs, a display name and an explanation.
- A ruleset whose resources, abilities or composed atoms read an
  undeclared property, or a property as a type it is not, is refused at
  load, and so is an adventure whose statblock does.
- Every declared property, with its display name and explanation, can be
  fetched through `GET /api/ruleset`.
- A resource chip's title, a Story line and an ability's cost name a
  property the ruleset declares by its display name.

VTT-283 names "an attribute, defense or resource name", which stops being
true; it is withdrawn and succeeded by the same bound on a property's key.

## What it touches

In this order:

1. The rules engine: `internal/rules/format.go`, `internal/rules/load.go`,
   `internal/rules/compile.go`, `internal/rules/resolve.go`,
   `internal/rules/schema/ruleset.schema.json`,
   `internal/rules/schema/atom.schema.json`,
   `internal/rules/conformance/conformance.go`, with every `ruleset.json`
   under `internal/rules/testdata` and `internal/rules/conformance/testdata`,
   the `rules.Ruleset` literals and manifests written inline in tests, and
   the tests that walk the schemas by path. The atoms under both
   `testdata` directories change with them.
2. The rulesets: `rulesets/tavern-brawl/ruleset.json`,
   `rulesets/dnd45e-minimal/ruleset.json`, their atoms, and their
   `guide.md`.
3. Adventures: `internal/adventure/load.go`, with the `ruleset.json` fixtures
   under `internal/adventure/testdata` and `internal/adventure/conformance`,
   and the manifests inline in its tests.
4. The read surface: `internal/gateway/metadata.go`, and the manifests inline
   in gateway tests; and the agent's, in `internal/mcp`: a read tool beside
   `get_ruleset_guide` that answers what `/api/ruleset` answers (the owner,
   2026-11-05).
5. The window: `client/src/metadata.ts` (`RulesetMeta`),
   `client/src/view/naming.ts`, `client/src/view/spectator.ts`,
   `client/src/view/grid.ts`, `client/src/view/player.ts`,
   `client/src/app.ts`, and the rebuilt `cmd/vtt/webdist`.
6. `docs/requirements.md`: the rules above.

This crosses more than two components, the rules engine, the adventure
loader, the read surface, the agent's tools and the window, and names more
than ten files, most of them fixtures. It stays one ticket (the owner,
2026-11-05): without the read surface and the window, nothing reads the
names.

## Specifications this moves

docs/specifications/012-the-read-surface.md
docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
docs/specifications/022-a-client-holds-one-log.md
New: how a ruleset declares its properties, the rules engine's first specification

## What could not be established

- **The format's version.** Whether the replaced format keeps `"2"` or takes
  a new version. The plan's.
- **The agent's words.** `vtt mcp` gets a ruleset's words only from
  `guide.md`, through `mcp.Config.RulesetGuide`; no MCP tool fetches
  `GET /api/ruleset`. If an explanation moves from the guide's prose into its
  property, the agent stops reading it unless the MCP surface serves it.
  Whether the guide keeps its prose or the agent gains the properties is the
  plan's.
