# The HTTP read surface has a record — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-27-the-read-surface-has-a-record-design.md`
**Verified:** 2026-09-27, by `verify-ticket`, an agent that did not write the
ticket, against `a330040` on `chore/spec-012-the-read-surface` (`main` at the
time; the ticket untracked). Verdict: **Passes with gaps.** The gaps are listed
at the end and travel with this plan. This plan does not edit the ticket; where
an item is thin, the plan decides around it and says so.

**Goal, in the ticket's words:** `docs/specifications/012-the-read-surface.md`
exists, the two "deliberate exception" sentences are gone, the note-key residue
is recorded debt, every rule the sort accepts has a row cited by a gateway test,
`metadata.go`, `ruleset.go` and `adventure.go` carry only warnings, pointers and
doc sentences with their ceilings lowered, `server.go`'s five named blocks are
pointers or warnings, and no code line changes.

**MapTool (CLAUDE.md rule 9), the answer this plan records.** MapTool has no
HTTP read API. An experimental Jetty web app, `net.rptools.maptool.webapi`,
first checked in on 2014-11-30 and removed on 2024-05-24 by the commit
"Remove experimental web app server and supporting jetty dependency", served a
WebSocket and token images with neither authentication nor a role check: its
`MTWebAppServer` registered a `WebSocketHandler` and a `/token`
`TokenImageHandler` and nothing that read a credential. The role is decided
once, in `ServerHandshake` (a GM challenge and a player challenge), and
`MapToolServer.addRemoteConnection` then pushes the whole campaign to every
client as one `SetCampaignMsg`, GM notes included; what a player may see is
decided on the client (`Zone.isVisibleToPlayers` and `MapTool.getPlayer().isGM()`
in `Zone`; `ClientHandshake` hiding the map select and the asset panel by
`ServerPolicy` unless the player is GM). Their answer does not fit: a
per-request bearer check and a server-side role gate are the opposite of a
model in which every client holds everything and the UI decides what to show,
which rule 9 already names as the one thing not to borrow (`internal/gateway/seat.go`).
The one thing kept is their list of GM-only data kinds — a token's `gmNotes`, a
zone's visibility, campaign macros — as a sanity check that this platform's
analogue, the adventure guide, is the read route gated by role and is gated on
the server (`adventureGuideRoles`; row F below). The verifier re-ran the search
by command in `~/dev/RPTool/maptool` (`git log` over `*webapi*`; grep over
`MapToolServer.java`, `ServerHandshake.java`, `ClientHandshake.java`,
`Zone.java`, `Token.java`; no servlet or HTTP server class in the current tree).
Where this is written is D3.

## Measurements this plan stands on

All at `a330040`, by command, run by the verifier.

`python3 tools/check-comments.py --report`, `grep -c '^\s*//'` / `grep -c '\S'`
per file, and the token instrument (D11):

| File | Comment lines / non-blank | Share | Ceiling | Banned | Blocks > 6 | Cites | Tokens |
|---|---|---|---|---|---|---|---|
| `metadata.go` | 412 / 698 | 59.0 | 59.1 | 36 | 12 | 0 | 2,065 |
| `ruleset.go` | 26 / 60 | 43.3 | 43.4 | 3 | 3 | 0 | 287 |
| `adventure.go` | 68 / 107 | 63.6 | 63.6 | 4 | 3 | 0 | 341 |
| `server.go` | 715 / 1,311 | 54.5 | 54.6 | 52 | 34 | 0 | 4,596 |
| `metadata_test.go` | 455 / 1,635 | 27.8 | 27.9 | 42 | 20 | 0 | 9,214 |
| `adventure_test.go` | 184 / 606 | 30.4 | 30.4 | 9 | 9 | 0 | 3,364 |
| `ruleset_test.go` | 78 / 320 | 24.4 | 24.4 | 3 | 5 | 0 | 2,009 |
| `artfile_internal_test.go` | 91 / 235 | 38.7 | 38.8 | 5 | 4 | 0 | 1,140 |

The three production files' figures are the ticket's. The package doc block at
the top of `metadata.go` (26 lines, one banned term) is excepted from the bound
by the gate and counted in the share.

**`metadata.go`, by block.** Thirty-five blocks; the twelve over the bound sit
above `adventureGuideRoles` (125 lines, 8 banned: the "RULING" section on the
art route), `joinLinkRoles` (12), `WithAdventureGuides` (15), `handleMe` (17),
`handleJoinLink` (12), `handleParticipants` (11), `DefaultCellPx` (47, 12
banned: `packRefJSON`'s obituary and the constant's doc), `handleMaps` (13),
inside `handleMaps` above the `writeJSON` call (13), `artContentTypes` (12),
`handleArtFile` (39), and inside `handleArtFile` above `fsys := root.FS()` (12).
The rest are one to six lines: `participantRoles` (6), the two blocks inside
`authed`, `writeJSON`'s, the section banners, the non-nil-slice and sort blocks
in `handleRuleset`, `joinLinkJSON`'s field block, the `cellPx` inheritance block,
the three trailing blocks inside `handleArtFile`.

**`ruleset.go`, by block.** Three, all over the bound: `errNoRulesetLoaded`
(8), `handleUseAbility` (10), and the stamping block inside it (8).

**`adventure.go`, by block.** Four: `errNoAdventuresAvailable` (7),
`handleLoadAdventure` (49, holding the two TOCTOU paragraphs), the stamping
block (5, under the bound), the warnings block above the final `return` (7).

**`server.go`, in scope.** Five blocks over the bound: the `ruleset`/`roller`
field block (8 lines, one block for both fields), `adventures` (9),
`adventureGuides` (8), `WithRuleset`'s doc (9), `WithAdventures`' doc (8).
34 − 5 = 29, the ticket's figure. `New`'s 4-line block (which says a server
without a ruleset refuses `use_ability`) and `Handler`'s one-line pointer are
not in the ticket's list and are not swept.

**The two sentences item 2 deletes.** `grep -rn 'deliberate exception'
internal/gateway/*.go` prints two lines: `WithAdventureGuides`' doc in
`metadata.go` and the `adventureGuides` field's doc in `server.go`. Both credit
`mapByID` as the one request-time filesystem read. The search that bounds the
truth: `grep -n -E '\bos\.[A-Z][A-Za-z]*\(|mapdef\.LoadInstalled|artlib\.(Open|Lookup|Validate)\('
internal/gateway/*.go` less test files and comment lines prints two code lines,
`os.OpenRoot(s.artDir)` in `handleArtFile` and `mapdef.LoadInstalled(...)` in
`mapByID`. The third false sentence: `adventure.go` says `errNoRulesetLoaded`
is declared "above"; it is declared in `ruleset.go`.

**The residue, against the code.** `handleCommand` calls `authorize`, which
takes `st := s.campaign.State()`, and hands that `st` to `handleLoadAdventure`.
`adventure.Compile` runs `checkCollisions(adv, st)`, which refuses a scene,
actor, token or note key already in `st`. `campaign.AppendBatch` then folds
clones against `c.state.Snapshot()` under `c.mu`; `engine.Apply`'s
`NoteUpserted` arm writes `st.Notes[nu.Key]` with no presence check, while its
`SceneCreated`, `ActorAdded` and `TokenPlaced` arms refuse a duplicate id. So an
`upsert_note` on the same key landing between the two calls is overwritten,
not refused. Nothing drives that window; the ticket says so.

**Every sentence of the ticket's first paragraph, against its symbol.** The
bearer: `authed` runs `strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")`
then `s.ids.Verify`, and writes one body, `gateway: unauthorized`, with 401 for
a missing prefix and for any `Verify` error. `/api/me`: `meJSON` carries
`ParticipantID`, `Name`, `Role` and nothing else. Empty collections:
`handleRuleset` starts every slice non-nil, `handleAdventures` and `handleMaps`
start with `[]`, so nothing loaded is 200 with `[]`; `handleRulesetGuide`
answers 404 when `s.ruleset == nil || s.ruleset.Guide == ""`;
`handleAdventureGuide` answers 403 outside `adventureGuideRoles` and 404 when
the id is unknown or the guide empty. Sorting: `slices.SortFunc` on abilities,
conditions, adventures and maps; **resources are not sorted** — `rs.Resources`
is a `[]ResourceDef` appended in declared order — so the ticket's "their lists
are sorted by id" over-claims by one list (gap 3). `/api/maps`: entries copied
under `mapsMu.RLock`, `m.CellPx == 0` resolved to `s.cellPx`, sorted, and
`cellPx: s.cellPx` beside the list; `New` sets `cellPx: DefaultCellPx` (64).
The art route, in order: `authed`; `s.artDir == "" || !artlib.IsArtFileName(name)`
→ 404; `os.OpenRoot(s.artDir)` per request, an error → the same 404;
`fs.Stat(root.FS(), name)` must be a regular file → else 404; `nosniff` and
`no-cache` on every success; `artContentTypes[filepath.Ext(name)]` (`.png` →
`image/png`, inline) else `application/octet-stream` with
`Content-Disposition: attachment; filename=...`; `http.ServeFileFS` off the same
`fs.FS`. `artlib.IsArtFileName` admits `<id>.png` and `<id>.json` with a
lowercase-kebab id and nothing else, which is what refuses a subdirectory in
either spelling. `WithRuleset` sets `s.roller = rules.NewCryptoRoller()`;
`WithAdventures` stores the map and does no I/O; `WithAdventureGuides` the same.
`handleUseAbility`: nil ruleset → `errNoRulesetLoaded`; `rules.Resolve(s.ruleset,
st, cmd, s.roller)`; stamps `EventId` (`newEventID`), `ParticipantId`,
`ActorRole`, `OccurredAt`; `s.campaign.AppendBatch(envs)`; `Sequence: firstSeq`.
`handleLoadAdventure`: `len(s.adventures) == 0` → `errNoAdventuresAvailable`;
unknown id → `gateway: unknown adventure %q`; `adventure.Compile(adv, st)`
returns the warnings that ride back as `Warnings` on the issuer's result only.
Every failure in both is an ok=false `CommandResult`.

**Callers (check 4).** The eight handlers are reached from `Handler` and from
tests only; `handleUseAbility` and `handleLoadAdventure` from `handleCommand`
only; `WithRuleset`, `WithAdventures` and `WithAdventureGuides` from
`cmd/vtt/serve_compose.go`'s `composeServer` and from tests; `DefaultCellPx`
from `New` and from `cmd/vtt/art_test.go`; the three role maps, the allowlist
and the two error strings have no callers outside their files (the MCP tools
name the strings' symbols in comments, by name). The one production test
outside `internal/gateway` that drives a read route is `cmd/vtt/maps_e2e_test.go`
on `/api/maps`; `static_test.go` drives `/api/ruleset` for its 401 (VTT-107's
territory).

**Pointers into the blocks this sweep cuts.** In the four test files:
`metadata_test.go`'s `get` (2-line doc: "see metadata.go for why the WS
precedent is deliberately not followed"), the 39-line header above `mapsFixture`
and the 29-line header above `artFixture` (both "metadata.go's own doc
section"); `artfile_internal_test.go`'s 38-line file header ("see metadata.go's
own doc section"); `adventure_test.go`'s 10-line doc of
`TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned` ("adventure.go's
handleLoadAdventure doc comment on the TOCTOU race posture") and the 29-line doc
of `TestLoadAdventureWithMultipleAdventuresLoadedServesRequestedContent`, which
cites the lookup by a bare line number, `adventure.go:76` — a line the sweep
moves. Outside the four: `server.go`'s `adventureGuides` field (in scope),
`map_test.go`'s 4-line body comment in the load-map stamping assertion ("see
adventure.go's handleLoadAdventure doc comment"), and
`client/src/view/pack-assets.ts` ("metadata.go's own doc section"). Pointers by
file or symbol only, which survive: `internal/artlib/artlib.go`, the three
`internal/mcp` tools, `internal/adventure/compile.go`, `internal/harness`,
`cmd/vtt/serve_test.go`, `client/src/metadata.ts`, `wire.ts`, `art-assets.ts`,
`map_internal_test.go`, `authz_test.go`, `server.go`'s dispatch block.

**Records and gates.** `docs/specifications/` holds 007 to 011; 012 is free.
`docs/requirements.md` has 110 rows, the last `VTT-110`;
`task check:requirements-chain` prints `110 rows, 193 test files, 5
specifications; every citation resolves and every row's evidence holds`.
`python3 tools/check-comments.py main` ends `239 files, 0 added comment lines,
239 ledger rows; clean`, preceded by a notice that `cmd/vtt/library_test.go` is
above its ceiling with no comment line added (a notice, not a refusal, and not
this work's). `python3 tools/check-doc-owner.py .` ends `79 files, every doc
comment sits on its own function`. `python3 tools/check_mutation_test.py -q`
prints `OK`. `go vet ./internal/gateway/` is silent and `go test -count=1
./internal/gateway/` passes in about thirty seconds. `gofmt -l internal/gateway/`
prints only `scenario_test.go`, a code alignment that predates this ticket.
`requirement-id` is on the path and, run bare, prints `a requirement needs its
sentence, not just an id`. `docs/verification-debt.md` has no entry mentioning a
note key. No `VTT-NNN` token exists in any of the four test files.

**The ledger at the base.** `--write-ledger` in a scratch clone at `a330040`
writes 239 rows and `git diff --stat` is empty: no row moves today, so after
the change only the rows of files this work changes can move (D12).

**`check:doc-owner`'s first-word rule.** It refuses a doc block above a function
whose first word names another function in the tree. Capitalised single-word
functions that are also imperative verbs: `Accept`, `Append`, `Apply`, `Close`,
`Compile`, `Fold`, `List`, `Load`, `Lookup`, `New`, `Open`, `Resolve`, `Run`,
`Validate`, `Verify`, `Write`. `Keep`, `Do`, `Never`, `Set`, `Check`, `Serve`,
`Stat`, `Answer`, `Refuse`, `Copy`, `Sort`, `Stamp`, `Return`, `Treat`, `Hold`,
`Guard` and `Read` name no function (D8).

**Records this ticket lifts from.** The client ticket's §3 says the endpoint
shapes are "documented in the client README"; there is no `client/README.md`.
SPEC-012 is the first record of the shapes. The comments credit ADR-008 with
"cmd/vtt owns the filesystem" ten times across `internal/gateway` and `cmd/vtt`;
ADR-008's text is about the cobra shell (thin commands, all logic in
`internal/`) and does not mention files, so the attribution resolves to nothing
(gap 6).

**Uncited tests in `metadata_test.go` that are SPEC-009's.**
`TestJoinLinkIsDMOnlyAndLiteralPerRole`, `TestJoinLinkReportsTheDoorAndTheSecret`,
`TestParticipantsIsDMOnlyAndLiteralPerRole`, `TestParticipantsNamesEveryoneAndTheirRole`
are cited by no row (grep over the register prints 0), and no row states the
join-link or the roster route's gate. Not this sort's (gap 7).

## Constraints that bind every task

- `CLAUDE.md` rule 10 and SPEC-010: a comment is an imperative warning, a
  pointer, or the one-line doc sentence of an exported symbol. VTT-051 is held
  by the Phase 4b reading; VTT-050, VTT-052, VTT-053 and VTT-055 by
  `check:comments`; VTT-059 says a citation line moves no share.
- `CLAUDE.md` rule 2: no gate is weakened. The ledger only goes down, only
  through `--write-ledger`.
- `CLAUDE.md` rule 8, narrowed by rule 10 for code: a comment points at a
  specification by number, a requirement by id, a test or symbol by name, or a
  report by its `docs/` path; never a line number, a date, a commit hash, a
  plan, a task, `§`, `#NNN`, an ADR or a `.superpowers/` path. This plan and
  the report obey rule 8 in full: blocks are named by the symbol they sit on.
- `CLAUDE.md` rule 3: the contract is not touched; `CommandResult`'s fields are
  SPEC-007's.
- `CLAUDE.md` rule 9: the answer above is recorded before any task runs.
- SPEC-008: ids come from `requirement-id`, after sign-off, never by hand.
- The `specification` skill's form and `catches.md`: five headings, present
  tense, no `Why`, no `Rejected`, no measurement, no line number, no past-tense
  account that is not about the code, no path outside the project, one
  decision per file, no claim taken from a comment without reading the code
  under it (item 8), every "only/every/none" with its search named (item 10).
- The `requirements` skill: one thing, what not how, breakable, measurable,
  named by a check or knowingly OPEN; refuse more than you accept.
- SPEC-009 and SPEC-011 are not edited: SPEC-012 points at them (D2).
- The ticket: no code line changes (D11 is the check); the three files whole,
  the five `server.go` blocks, the four test files' citation lines and the
  blocks a re-aimed pointer sits in, one debt entry, the register, the ledger.
- `internal/gateway/map.go`, `authz.go`, `seat.go`, `export_test.go`,
  `cmd/vtt/*`, `internal/artlib`, `internal/mcp`, `client/src` and the three
  source tickets are not touched. `docs/reports/` gains only this ticket's
  report.

## Decisions this plan makes

**D1. SPEC-012's sections, each lifted from a named source and held by named
symbols.** Forced by ticket item 1 and the `specification` skill's step 3.
Under "How it works", in this order, one bold-led paragraph each:

| Section | Source sentence (where it lives today) | Symbols that hold it | Rows (D5) |
|---|---|---|---|
| The bearer and its one 401 | `metadata.go`'s package doc ("Auth is a Bearer header, NOT ?token="); `authed`'s two body blocks; SPEC-009's sentence, pointed at | `authed`: `strings.CutPrefix` on `Authorization`, `s.ids.Verify`, one body `gateway: unauthorized`; called first by every `/api/*` handler | A |
| The routes and their gates | `Handler` (SPEC-011 lists the patterns; pointed at); the docs of `adventureGuideRoles`, `joinLinkRoles`, `participantRoles`; `handleAdventures`' body block | `handleMe`, `handleRuleset`, `handleRulesetGuide`, `handleAdventures`, `handleAdventureGuide` (`adventureGuideRoles`, 403), `handleMaps`, `handleArtFile`; `/api/join-link` and `/api/participants` are SPEC-009's; the HTTP gates are not cells of `commandRoles` (`authz.go`), whose keys `TestEveryClientCommandHasRoleCells` counts | F, G |
| Empty is not an error | the package doc's second section; `handleRuleset`'s non-nil-slice block; `handleAdventures`; `handleMaps` | `rulesetJSON{Abilities: []abilityJSON{}, ...}`, `out := []adventureJSON{}`, `out := []mapMetaJSON{}`; `handleRulesetGuide`'s nil-or-empty 404; `handleAdventureGuide`'s unknown-or-empty 404 | D, E |
| What `/api/me` answers | `handleMe`'s doc | `meJSON`: `ParticipantID`, `Name`, `Role` from the `identity.Participant` `authed` returns; control is the log's, decided in `authz.go` (pointer) | B, C |
| What `/api/ruleset` answers | `handleRuleset`'s body and its two blocks; client ticket §3 | `rulesetJSON`, `abilityJSON` (`id`, `name`, `range`, `maxTargets`, `usage`), `usageJSON` (`atWill` or `resource` with `resource` and `cost`), `conditionJSON`, `resources`; abilities and conditions sorted by id because `Compiled` and `Conditions` are maps; resources in the ruleset's declared order; `Guide` on `/api/ruleset/guide` | H |
| Adventures and the guide | `handleAdventures`, `handleAdventureGuide`, `WithAdventureGuides`' doc | `adventureJSON{ID, Name}` sorted by id; `s.adventureGuides[r.PathValue("id")]` | F, G, H |
| `/api/maps` and the cell size | `handleMaps`' doc and its two body blocks; `mapMetaJSON.CellPx`'s block; `DefaultCellPx`'s block | copy-out under `mapsMu.RLock`, response written outside it; `m.CellPx == 0` resolved to `s.cellPx`; sorted by id; `cellPx: s.cellPx` beside the list; `New` sets `DefaultCellPx`; `WithCellPx`; `cmd/vtt`'s `TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber` holds the equality with `campaigncfg.DefaultCellPx`; what a map is and how `mapByID` loads one is `map.go`'s and has no record yet | I, J, H |
| The art route | the "RULING" block above `adventureGuideRoles`; `artContentTypes`' doc; `handleArtFile`'s doc and its four body blocks; art report §2 and §4.3 | the order in the measurements above; `artlib.IsArtFileName` as the guard; the `{file}` pattern is not one (an encoded slash reaches `PathValue` as one segment); `os.OpenRoot` per request, nothing cached; `fs.Stat` on `root.FS()`; `nosniff`, `no-cache`; `artContentTypes`; the attachment fallback for the sidecar; `http.ServeFileFS` with `Last-Modified` answering a conditional request 304; one body `gateway: no such art`, no path in any response | K to U |
| What is set at boot | the `Server` field docs; `WithRuleset`, `WithAdventures`, `WithAdventureGuides` docs; ruleset ticket §7, adventure ticket §7; `composeServer` | `WithRuleset` sets `s.roller = rules.NewCryptoRoller()` with the ruleset and nothing configures the roller separately; `WithAdventures` takes a map keyed by `Adventure.ID` and does no I/O; `WithAdventureGuides` the markdown beside it; `WithCellPx`; `cmd/vtt`'s `composeServer` reads them (`rules.Load`, `loadAdventuresDir`, `loadAdventureGuides`, `campaigncfg`) before serving; this package reads the filesystem at request time in `handleArtFile` and `mapByID` and nowhere else (the search above) | none directly |
| The two commands that spend it | `handleUseAbility`'s and `handleLoadAdventure`'s docs; the two tickets' §7 | `handleCommand` → `authorize` (`s.campaign.State()`) → the handler with that `st`; `rules.Resolve(s.ruleset, st, cmd, s.roller)` / `adventure.Compile(adv, st)`; the four stamped fields; `s.campaign.AppendBatch(envs)`; `Sequence: firstSeq` (SPEC-007 states the convention; pointed at); `Warnings` on the issuer's result and nowhere else; every failure an ok=false result on a connection that stays | V to AA |
| The note-key residue | `handleLoadAdventure`'s two TOCTOU paragraphs | `checkCollisions` against the handler's `st`; `AppendBatch`'s re-fold against a fresh snapshot under `c.mu`; `engine.Apply`'s `NoteUpserted` arm is an upsert; the three id arms refuse duplicates; the debt entry (D4), pointed at | none |

Each sentence names the symbol that holds it, is checked against the code
before it is written and re-read by Phase 4b (D16). Status: `Accepted.
Implemented by internal/gateway/metadata.go, ruleset.go, adventure.go and
server.go (Server's ruleset, roller, adventures and adventureGuides fields,
WithRuleset, WithAdventures; Handler registers the routes, SPEC-011), fed by
cmd/vtt/serve_compose.go's composeServer; pinned by internal/gateway/metadata_test.go,
adventure_test.go, ruleset_test.go, artfile_internal_test.go and, for the
default cell size, cmd/vtt/art_test.go.` "Principles served" says what
SPEC-009 and SPEC-011 say: no blueprint; the principle missing from the record
rather than absent (here: what a seat may read is decided on the server, per
request). "Consequences" holds what a client author is bound by: the token
travels in a header and never in a URL; `[]` is an empty table and a 404 on a
guide is "no guide", neither is an error; control is never inferred from
`/api/me`; a map draws at its entry's `cellPx`; art is fetched as `<id>.png` or
`<id>.json` and by no other name; a browser reuses no art without revalidating;
a refused batch command leaves the connection usable and a bundle's warnings
arrive on the result. "Requirements" lists the ids the sort dispenses (D6) and
nothing else.

**D2. What SPEC-012 points at and does not restate.** Forced by `catches.md`
item 13 and the caller's brief. SPEC-009 owns the credential and what `authed`
promises (verified on every request; unknown and revoked answered alike) and
the `/api/join-link` and `/api/participants` routes with their role maps;
SPEC-012 names `authed`'s mechanism (the header, the prefix, the one body) and
says "as SPEC-009 states" for the promise, and lists the two routes in one
sentence that points at SPEC-009. SPEC-011 owns `Handler`, the
method-qualified patterns, `/healthz`, `/ws`, `POST /join` and the static route;
SPEC-012 says the nine `GET /api/*` handlers are registered there and points.
SPEC-007 owns `CommandResult` (`sequence` names the first event a command
produced; `warnings`); SPEC-012 states the batch case (`firstSeq` of the batch
`AppendBatch` assigned) and points. `map.go` (`mapByID`, `handleLoadMap`,
`WithMaps`, `WithMapsDir`, `WithArtDir`) has no record; SPEC-012 states that
`handleMaps` reads the set `mapByID` grows and that `mapByID` reads the
filesystem, and names `map.go` as the home of the rest. The dice rule (rolled
once, recorded on `AbilityUsed`, never re-rolled on replay) is `internal/rules`'
and has no record; SPEC-012 states the gateway's half — the roller is fixed by
`WithRuleset` and is not configurable apart from the ruleset — and no more.

**D3. Provenance stays out of the record.** Forced by `catches.md` items 3 and
12. The MapTool answer above is the rule-9 answer and lives in this plan and
the report. SPEC-012 states the role gate as a fact about this system and names
no other project. Q1.

**D4. The residue is a debt entry, and SPEC-012 describes what the code does.**
Forced by ticket item 3 and `catches.md` items 5 and 7. Under `## Open debt`,
in the file's shape: a bold title (**A `load_adventure` whose note key collides
with a note upserted after its snapshot overwrites it instead of refusing.**),
one paragraph naming `handleCommand`'s `authorize` (`campaign.State()`),
`adventure.Compile`'s `checkCollisions`, `campaign.AppendBatch`'s re-fold, and
`engine.Apply`'s `NoteUpserted` arm against its `SceneCreated`, `ActorAdded`
and `TokenPlaced` arms; the labels `test data missing` and `outside the tool`
(no fixture holds an `upsert_note` between the two calls, and no mutation
operator opens the window); the edit that would put a test on it — an internal
test in `internal/gateway` that takes `st := c.State()`, appends a
`NoteUpserted` on key K through `c.Append`, calls `s.handleLoadAdventure(...,
st, ...)` for an adventure declaring K, and asserts ok=false, which today
answers ok=true and overwrites; and "Recorded 2026-09-27, moved here from the
comment at `handleLoadAdventure`". The comment keeps a two-line warning at the
`adventure.Compile` call: `Do not rely on AppendBatch to refuse a note-key
collision: its re-fold upserts (docs/verification-debt.md).` Q10 asks about the
labels.

**D5. The sort's starting point is the table below; nothing is an id until
sign-off.** Forced by SPEC-008 and the `requirements` skill. Rows are lettered
so nothing here reads as an id. Each line: the rule as a what; the outcome
proposed; the evidence, verified by the verifier against the test bodies. The
reading after sign-off confirms or overrides each and reports what it refused.

| # | Rule (what) | Proposed | Evidence |
|---|---|---|---|
| A | A `/api/*` request with no bearer token, an unknown one or a revoked one is answered 401. | accept; "the two are not told apart" is prose (SPEC-009 states it, no test compares the bodies) — Q2 | `TestMetadataRejectsBadMissingAndRevokedTokens`, `TestArtFilesRequireAuth`, `TestMetadataMeIdentifiesTheCaller` (its last arm) |
| B | `/api/me` answers the caller's participant id, display name and role. | accept | `TestMetadataMeIdentifiesTheCaller` |
| C | `/api/me` answers nothing about what the caller controls. | accept — the ticket's second candidate split | `TestMeSaysWhoYouAreAndNeverWhatYouControl` |
| D | A server with nothing loaded answers `/api/ruleset`, `/api/adventures` and `/api/maps` with 200 and empty, non-null collections. | accept | `TestMetadataEmptyCollectionsWithNothingLoaded`, `TestMapsEmptyCollectionWithNothingLoaded` |
| E | A guide that does not exist is answered 404. | accept — the third candidate split | `TestMetadataEmptyCollectionsWithNothingLoaded` (the ruleset guide with no ruleset; an adventure guide for an unknown id) |
| F | An adventure guide is answered to the DM and the agent, refused to a player and a spectator, and a refusal carries no guide. | accept | `TestMetadataAdventureGuideRoleTable` |
| G | The adventure list, the ruleset guide, the maps list and the art are answered to every role. | accept, on the four observed routes; `/api/ruleset` per role is unobserved (gap 1) | `TestMetadataAdventuresListedForEveryRole`, `TestMetadataRulesetGuideServedForEveryRole`, `TestMapsListedForEveryRole`, `TestArtIsReadableByEveryRole` |
| H | The abilities, conditions, adventures and maps a read route lists are in id order. | accept, reworded: resources are in the ruleset's declared order (gap 3) | `TestMetadataRulesetAbilitiesAreSortedById`, `TestMetadataRulesetShapeMatchesTheContract`, `TestMetadataAdventuresListedForEveryRole`, `TestAMapsOwnCellPxOverridesTheCampaignDefault` |
| I | A maps entry reports the map's own cell size when it declares one and the campaign's otherwise. | accept | `TestMapsReportsTheCampaignCellPx`, `TestAMapsOwnCellPxOverridesTheCampaignDefault` |
| J | `/api/maps` reports the campaign's configured cell size beside the list. | accept — the sixth candidate split | `TestMapsReportsTheCampaignCellPx`, `TestACampaignsOwnCellPxReachesTheClient`, `TestAMapsOwnCellPxOverridesTheCampaignDefault` |
| K | The art route serves the picture and the sidecar of an installed piece with the file's own bytes, and answers 404 for a piece not installed. | accept | `TestArtIsServedAndUnknownArtIs404`, `TestASidecarIsServedSoAClientCanResolveADoor` |
| L | A name that is not an art filename is answered 404 whether or not a file of that name exists. | accept — the seventh candidate split by mechanism, as its own tests are | `TestArtNameNotAnArtFilenameIs404` |
| M | Nothing under a subdirectory of the art directory is reachable, in any spelling of the path. | accept | `TestArtInsideASubdirectoryIsNotReachable`, `TestHandleArtFileRefusesANestedNameThatRoutingWouldNeverProduce` |
| N | A directory wearing an art filename is answered 404 and nothing inside it is served. | accept | `TestADirectoryWEARINGAnArtFilenameIs404`, `TestHandleArtFileRefusesADirectoryWearingAnArtFilename` |
| O | No art request reaches a file outside the art directory, by traversal or by a symlink. | accept, worded on "reaches": the symlink test asserts not-200 and no leak, not 404 | `TestHandleArtFileRefusesTraversalEvenWithAPathValueSetDirectly`, `TestHandleArtFileRefusesSymlinkEscape`, `TestArtIsServedAndUnknownArtIs404` (its traversal arm) |
| P | An absent or unopenable art directory is answered 404 at request time. | accept — the eighth candidate split | `TestArtWithNoArtDirectoryConfiguredIs404`, `TestAnUnopenableArtRootDegradesAtRequestTime` |
| Q | No art response names a path on the server. | accept | `TestNoArtResponseNamesWhereTheCampaignLives` |
| R | A picture is served inline as `image/png` with sniffing forbidden. | accept — Q6 | `TestArtPictureGetsItsRealContentTypeInline` |
| S | A sidecar is served as an octet-stream attachment with sniffing forbidden. | accept — Q6 | `TestArtSidecarIsOctetStreamAttachment` |
| T | Art installed or overwritten while the server runs is served on the next request, without a restart. | accept — the tenth candidate split | `TestArtInstalledAfterTheServerStartedIsServed` |
| U | An art response requires revalidation, and a conditional request for unchanged art is answered 304 with no body. | accept | `TestArtIsSentWithCacheControlSoAnOverwriteReachesTheBrowser` |
| V | A `use_ability` or `load_adventure` the server cannot serve — no ruleset, no adventures, an unknown adventure, a command the ruleset refuses — is answered ok=false on a connection that stays open. | accept, one row: one refusal shape | `TestUseAbilityNoRulesetLoadedCleanError`, `TestLoadAdventureNoAdventuresConfiguredCleanError`, `TestLoadAdventureUnknownIdCleanError`, `TestUseAbilityResolveValidationErrorIsCleanOkFalse` |
| W | A `load_adventure` whose ids collide with the table is refused as a result and the table keeps serving. | accept, worded on the observation; "appends nothing" is unobserved (gap 9) | `TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned` |
| X | A `load_adventure` loads the adventure whose id it names and no other. | accept — from the test, not in the ticket's list | `TestLoadAdventureWithMultipleAdventuresLoadedServesRequestedContent` |
| Y | A `load_adventure`'s warnings ride on the issuer's result, naming the reference and the scene. | accept; "and nobody else" is unobserved and stays prose (gap 9) | `TestALoadAdventureWarningReachesTheIssuer` |
| Z | A `load_adventure` result stays readable whatever the bundle's warnings; the test carries the bound. | accept | `TestABrokenBundleCannotPushALoadAdventurePastTheReadLimit` |
| AA | A `use_ability` or `load_adventure` result carries the first sequence of the batch it appended. | accept-leaning — SPEC-007 states the convention for every command; Q3 | `TestUseAbilityHitProducesBatchFirstSequence`, `TestLoadAdventureProducesBatchFirstSequenceReachesAllParticipants` |

Refused, or not this ticket's, each with its reason: the batch reaching a
second connection contiguous and in the compile's order (observed by the two
AA tests) — `campaign.AppendBatch`'s atomicity and SPEC-011's delivery, not the
read surface's; `TestMetadataRulesetShapeMatchesTheContract`'s field list — a
row that lists fields is a list, and SPEC-012 states the shape (Q8);
`TestNoPackRouteIsServed` — an absence held by `check:no-pack`, not a rule of
this surface; `TestRemoveConditionAppliedThenRemoved` — the command path's,
neither the read surface nor the two batch commands;
`TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber` — `cmd/vtt`'s, and
item 4 wants a test under `internal/gateway`; the four join-link and roster
tests — SPEC-009's, uncited today, not this sort's (Q7). Five clauses of the
ticket's candidates are refused as rows and kept as SPEC-012 prose or named as
gaps: "not told apart", "the ruleset ... to every role", "a list" (resources),
"appends nothing and poisons nothing" (poisons nothing is W), "and nobody
else".

Twenty-seven accepted or leaning against six refused tests and five refused
clauses is more than the skill's caution expects, and the reading after
sign-off is asked to cut, not to add: each row above names a test that goes
red on its own edit, and the art route's seven rows are seven mechanisms whose
own test file says each needed its own proof.

**D6. Rows are dispensed after sign-off, cited by citation lines only, and an
OPEN row has no citer.** Forced by SPEC-008, the connection plan's D6, and the
debt file's open entry on OPEN rows. Every row above names an existing test, so
none is expected OPEN; if the reading refuses a test as evidence and keeps the
rule, that row is `**OPEN — no test yet**` with no citation line and the
report names it (ticket item 4). A citation line is `// VTT-NNN` directly above
`func Test`, below any doc block, several ids on one line where a test holds
several. Evidence cells are `internal/gateway/<file>#<Test>`, written by hand
after the dispenser; the chain gate refuses one that is wrong. SPEC-012's
Requirements line is copied from the register last.

**D7. `server.go`: what is swept and what stays, by symbol.** Forced by ticket
item 5. Swept: the `ruleset`/`roller` field block, the `adventures` field block,
the `adventureGuides` field block, `WithRuleset`'s doc, `WithAdventures`' doc.
Each becomes a doc sentence (the two exported methods) or a one-line pointer
(the fields) with at most one warning: `WithRuleset` keeps `Do not call a With*
method on a serving Server: they write without a lock.` Not swept, named for
the next sweeps: `New`'s block, `Handler`'s pointer and its inner warnings, and
every other block the connection report lists. After the sweep `server.go`
carries 29 blocks over the bound and about 46 banned lines, on those symbols;
the gate does not refuse them because no line is added to them.

**D8. The sweep's rules, repeating the connection plan's where they apply.**
Forced by rule 10, SPEC-010, and the identity plan's D1 to D8.

- *Three kinds and nothing else*: a warning is imperative, a verb first, the
  consequence in the present tense, at most three lines; a pointer is
  `SPEC-012`, `SPEC-009`, `SPEC-011`, `SPEC-007`, `VTT-NNN`, a test name, a
  symbol name, or `docs/verification-debt.md`, and may close a warning or a
  doc sentence in parentheses; a doc sentence is the first sentence `go doc`
  prints for an exported symbol. Section banners (`// --- /api/me ---`) are
  neither and go. Every fact a deleted block held that SPEC-012 does not state
  is either added to SPEC-012 (D9) or named in the report as dropped, with the
  reason.
- *Unexported symbols keep no doc sentence*: `authed`, `writeJSON`, every
  `handle*` in the three files, the three role maps, `artContentTypes`, the
  JSON types, `errNoRulesetLoaded`, `errNoAdventuresAvailable`. The exported
  symbols in scope are `DefaultCellPx`, `WithAdventureGuides`, `WithRuleset`
  and `WithAdventures`; each keeps one sentence.
- *A warning above a function opens with a word that names no function*:
  `check:doc-owner` refused "Run" and "Verify" last time. The safe first words
  are in the measurements; `Open`, `Close`, `Write`, `Load`, `List`, `Resolve`,
  `Verify`, `Run`, `Apply`, `Append`, `Accept`, `Compile`, `Validate`, `Lookup`
  and `New` are not used as a warning's first word above a function. A warning
  inside a body is not read by that gate.
- *The package doc*: `metadata.go`'s 26-line block above `package gateway` is a
  second package doc (the package's is `authz.go`'s); it goes whole and its two
  facts go to SPEC-012. `package gateway` then stands bare, as in `ruleset.go`.
- *A doc sentence is one sentence, on one line where it fits*; the wrap band is
  `SHORT, LONG = 55, 85` in `tools/check-comment-wrap.py`.
- *Test files*: citation lines (D6) and the re-aimed pointers with the blocks
  they sit in, sorted to the bound (D22). No other test-file edit; the doc
  blocks above tests, some of thirty lines with banned terms, are the test-prose
  sweep's.
- *No row is dispensed by the sweep itself*: rows come from D5's sort after
  sign-off, in the same commit.
- *What is left alone*: directives (none in the four files); `//` inside string
  literals (none); the trailing field comment `// "atWill" | "resource"` on
  `usageJSON.Kind`, which is a doc of an unexported field and goes; every block
  outside D7's list.

**D9. Facts only a comment holds go into SPEC-012.** Forced by ticket item 1
and identity D7. The reading of each swept block asks whether SPEC-012's draft
states the fact; if not, and the fact is about the code now, it is checked
against the symbol and written in. The candidates are listed below.

**D10. One commit for the change; the report in its own.** Forced by the band
(a file's deletions and its row land together), by the chain gate (a row's
evidence and its citation line land together, and neither hook runs
`check:requirements-chain` or `check:comments`), and by the pointers (`SPEC-012`
must have a target in the same tree). The commit carries the ticket (untracked
today), this plan, SPEC-012, the debt file, the register, the four production
files, the four test files, and the ledger. The pattern is `0565d19` then
`a330040`.

**D11. The comment-stripped comparison is a token stream.** Forced by identity
D11. The program is the identity plan's Task 0 listing (`go/scanner`, comments
dropped), built once in the scratchpad (`$S/codetokens/codetokens`, with a
`go.mod` beside it) and run from the repository root over the eight files:

    for f in internal/gateway/metadata.go internal/gateway/ruleset.go \
             internal/gateway/adventure.go internal/gateway/server.go \
             internal/gateway/metadata_test.go internal/gateway/adventure_test.go \
             internal/gateway/ruleset_test.go internal/gateway/artfile_internal_test.go; do
      git show "a330040:$f" > "$S/before.go"
      "$S/codetokens/codetokens" "$S/before.go" > "$S/before.tok" || echo "SCAN FAILED $f"
      "$S/codetokens/codetokens" "$f" > "$S/after.tok" || echo "SCAN FAILED $f"
      printf '%s %s/%s tokens ' "$f" "$(wc -l < "$S/before.tok")" "$(wc -l < "$S/after.tok")"
      cmp -s "$S/before.tok" "$S/after.tok" && echo same || echo DIFFERS
    done

Done reads eight `same` lines with the counts 2065, 287, 341, 4596, 9214,
3364, 2009 and 1140 on both sides. A run proves it ran by the counts; a zero is
a failed run. A `DIFFERS` on a test file means a string literal changed (last
time a failure message did), and Phase 4b names it.

**D12. The ledger, last, and seven rows.** Forced by SPEC-010 (the band) and by
`--write-ledger` lowering a row on any drop (`min(old, now)`), which the
connection report learned. After Phase 4b has settled, `python3
tools/check-comments.py --write-ledger`, then `git diff tools/comment-ceilings.txt`
must show exactly seven changed rows, each lowered: `internal/gateway/metadata.go`,
`ruleset.go`, `adventure.go`, `server.go`, `metadata_test.go`,
`adventure_test.go`, `artfile_internal_test.go`. `ruleset_test.go`'s row moves
only if a block in it is sorted, which D22 does not expect; a citation line
moves no share. No row moves at the base today. An eighth changed row means a
file changed that this plan does not name: stop, name it, and ask before
committing. The commit message lists the rows old and new.

**D13. The deliberate breaks, one per check this work relies on.** Forced by
the dev-cycle's rule that a check is proven by a red, and ticket items 2, 4, 5
and 6. In a scratch clone (`git clone --no-hardlinks` into the scratchpad, the
final `git diff HEAD` applied, the untracked files copied in, committed there,
that clone's `main` pointing at that commit): first each gate exits 0 with its
completion line; then each break is one edit, the finding recorded verbatim,
the inverse edit by hand (never `git checkout --`), `git diff --stat` printing
nothing before the next.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | `// VTT-999` directly above `TestRemoveConditionAppliedThenRemoved` in `ruleset_test.go` (uncited by design) | `check:requirements-chain`: `cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 | one new row's evidence entry re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `check:requirements-chain`: `does not carry the id, so the link walks one way only` |
| B3 | SPEC-012's Requirements line gains `VTT-999` | `check:requirements-chain`: `cites VTT-999 ... (specification citation)` |
| B4 | one pointer line, `// SPEC-012`, inside `handleUseAbility`'s body in `ruleset.go`, between two code lines where no block can absorb it, after the ledger is written | `check:comments`: `internal/gateway/ruleset.go: comment share X is above its ceiling Y and this change added a comment line to it (SPEC-010)`. Placed in `ruleset.go` for the arithmetic: about 34 code lines after the sweep, so one line moves the share by two points, far past the 0.1 the rounding can hide |
| B5 | `if cellPx == 0` made `if cellPx != 0` in `handleMaps` | D11's loop: `metadata.go ... DIFFERS` (and row I's `TestAMapsOwnCellPxOverridesTheCampaignDefault` red, which is the row's own proof) |
| B6 | "exactly one deliberate exception" re-inserted in `WithAdventureGuides`' doc | `grep -rn 'deliberate exception' internal/gateway/*.go` prints 1 (item 2's observation goes back toward today's) |
| B7 | the warning above `handleArtFile` rewritten to open with `Open` | `check:doc-owner`: a doc comment naming `Open` above `handleArtFile`; this proves the first-word rule D8 relies on |

**D14. Gate steps before the commit, in order, on a tree the review has
settled.** `gofmt -l internal/gateway/` prints only `scenario_test.go` (D21);
`go vet ./internal/gateway/`; `go test -count=1 ./internal/gateway/...` green;
D11's loop, eight `same`; `task check:comments` (expected at this point to
refuse the seven files for the band until Task 10 writes the ledger, and
nothing else); `task check:doc-owner`; `task check:requirements-chain`
(`<110 + N> rows, 193 test files, 6 specifications; every citation resolves and
every row's evidence holds`); `task check:new-prose` (every test and symbol
name the sweep writes must resolve; `SPEC-NNN` is not read by that gate, so a
`SPEC-012` pointer is held by the reading); `python3 tools/check_mutation_test.py -q`
and `python3 tools/check_ts_mutation_test.py -q` (no key names any of the four
files, so none moves); `task lint`. Then, after Task 10, `task check` whole,
once, on the final tree, launched in its own session (`start_new_session=True`),
and only after Phase 4b has settled, so no edit lands mid-run; `uptime` first,
since the e2e waits fail under load. Then the pre-commit hook's own set, the
review gate among them, with `git add` and `git commit` in separate calls.

**D15. Phase 4a is skipped, with its reason.** Independent QA derives tests
from a specification to find behaviour the implementer got wrong. This change
has no behaviour: D11 shows every Go file's code identical to `a330040`, and
what is added is prose (SPEC-012, a debt entry), register rows, citation lines
and comment deletions. What can be wrong is a sentence, and a reading holds
that: VTT-051 is `**READING — Phase 4b**`, and SPEC-012's sentences are held
the same way (D16). The report records the skip under its own heading.

**D16. Phase 4b is the check for VTT-051 and for SPEC-012, sentence by
sentence.** The reviewer gets `git diff HEAD`, SPEC-007, SPEC-009, SPEC-010,
SPEC-011, SPEC-012, the rows VTT-050 to VTT-059 and the new rows, the four
production files, the four test files, `map.go` and `serve_compose.go`. For
every surviving block it names the kind (D8) and, for a warning, the code it
guards and whether the consequence is true of that code; for a pointer, that
the target resolves; for a doc sentence, that the symbol is exported and the
sentence true. For every deleted block: did it hold a fact now in no record?
If yes, SPEC-012 (D9) or the report. For every SPEC-012 sentence: the symbol it
names, and whether the code under the symbol does what the sentence says —
`catches.md` item 8 is the one to watch, since every SPEC-012 sentence starts
life in a comment, and this plan already found one (the sorted resources). For
each dispensed row: the edit that would red the test named. For the debt
entry: the labels fit the table and the edit named would show the defect.

**D17. What the report records.** Per file: comment lines and share before and
after, blocks deleted, blocks kept by kind; the sentences SPEC-012 took from
comments, each with its symbol; the sort — every row accepted with its id and
test, every candidate refused with one line; `server.go`'s remaining blocks by
symbol; the seven ledger rows old and new; D11's counts; the breaks' finding
lines; the gaps below as found or closed; the pointers outside the four test
files that now aim at cut text (D19), for the next tickets. The report does not
revise the art report or the reports of the ruleset and adventure periods.

**D18. Order of work: the record first, then the files it is pointed at from.**
SPEC-012 is drafted before any comment is cut, so every pointer the sweep
writes has a target and every deleted fact has a home to be checked against.
Then `server.go`'s in-scope blocks (the `adventureGuides` field points into
`metadata.go`), then `metadata.go`, then `ruleset.go` and `adventure.go` with
the debt entry; then the test files' re-aims and forced sorts (D22); then,
after sign-off of the sort, the rows and citation lines; the ledger last (D12).

**D19. Things the ticket's scope leaves stale are left, and named.** The
pointer in `map_test.go`'s load-map stamping assertion and the one in
`client/src/view/pack-assets.ts` aim at blocks this sweep cuts; neither file is
in the ticket's list. The nine other `ADR-008` attributions outside the four
files (`map.go`, `WithCellPx`'s doc in `server.go`, six under `cmd/vtt`), the
`.superpowers`-style citations in `adventure_test.go`'s header and in
`TestLoadAdventureProducesBatchFirstSequenceReachesAllParticipants`'s doc, and
`Handler`'s inner warning that reads as if the `{file}` pattern kept `art/` flat
(SPEC-012 says the name check does) are all named in the report as the next
sweeps'. Q4 and Q5 ask about the two pointers that can be re-aimed cheaply.

**D20. Nothing is cited from the three source tickets or the art report.**
Their sections are the sources SPEC-012 lifts from (D1's table), and a
specification points at symbols, tests and other specifications. The tickets
and the report are not edited.

**D21. gofmt.** The base prints only `scenario_test.go`, whose difference is in
code and predates this ticket; it stays. If a surviving list-shaped comment
makes gofmt object to a swept file, the list is rewritten as sentences: gofmt's
own output is not committed, because a reflowed comment is a comment edit like
any other and Phase 4b reads it.

**D22. The test-file blocks a re-aimed pointer forces to the bound.** Forced by
`check:comments` (a line added to a block over six lines is refused, and a
re-aimed line is an added line), which the connection report's deviations
record, and by ticket item 5 of "What it touches", which allows exactly this.
Found by grepping the four test files for `metadata.go`, `adventure.go`,
`ruleset.go`, `doc comment` and `doc section` (the measurements above):

1. `metadata_test.go`, the 39-line header above `mapsFixture` — the pointer
   "written down in metadata.go's own doc section" — sorted to at most six
   lines: a warning that the fixture wires no static bundle because
   `TestNoPackRouteIsServed` depends on it, and a pointer to SPEC-012. Its six
   banned lines go with it.
2. `metadata_test.go`, the 29-line header above `artFixture` — "survived only
   as prose in metadata.go's own doc section" — sorted to a pointer to
   SPEC-012 and `artfile_internal_test.go`.
3. `artfile_internal_test.go`, the 38-line file header — "see metadata.go's
   own doc section" — sorted to at most six lines: every test calls
   `handleArtFile` directly with a clean `r.URL.Path` and the payload in
   `PathValue`, because `ServeMux` and `http.ServeFileFS` would mask a broken
   handler; every test asserts its control first; pointer to SPEC-012.
4. `adventure_test.go`, the 10-line doc of
   `TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned` — "see
   adventure.go's handleLoadAdventure doc comment on the TOCTOU race posture" —
   re-aimed at `docs/verification-debt.md` and sorted to the bound.
5. `adventure_test.go`, the 29-line doc of
   `TestLoadAdventureWithMultipleAdventuresLoadedServesRequestedContent`, only
   if Q4 says the `adventure.go:76` citation is re-aimed at the lookup in
   `handleLoadAdventure` by name; then it is sorted to the bound too.

Re-aimed without a sort, because the block is under the bound:
`metadata_test.go`'s 2-line doc of `get` ("see metadata.go for why the WS
precedent is deliberately not followed") becomes a pointer to SPEC-012. Not
re-aimed: `TestArtWithNoArtDirectoryConfiguredIs404`'s doc (aims at
`WithArtDir`'s doc in `server.go`, unswept), `adventure_test.go`'s header (aims
at `ruleset_test.go`'s doc comment, unswept), and
`TestUseAbilityHitProducesBatchFirstSequence`'s doc (aims at `newRulesetFixture`'s
doc, unswept). A sorted test block follows D8's three kinds: what survives is a
warning about the fixture or the assertion, or a pointer; the arguments and the
history go, and the report names what went.

**D23. `/api/ruleset` for every role has no test, and this ticket writes
none.** Forced by the ticket (no code line changes; test files get citation
lines and sorted blocks only). Row G is worded on the four observed routes;
SPEC-012 states that `handleRuleset` has no role gate, which is the code; the
report names the gap for the next ticket that touches `metadata_test.go`'s
tests, where the fix is one more table row in
`TestMetadataRulesetGuideServedForEveryRole`'s sibling shape. Not a second debt
entry: the ticket allows one, and the note-key residue is the defect; this is a
coverage gap the report can carry.

**D24. Rows land under SPEC-012's Requirements line, SPEC-009 untouched.**
Forced by check 6 (`New:` and no path beside it) and D2. Row A's observation is
the read surface's (`authed`), so its id goes on SPEC-012's line; SPEC-009
keeps stating the promise and points at nothing new. Q9.

## Candidates the reading starts from

Read by the verifier at `a330040`. A starting point, not a verdict.

**Facts for SPEC-012 that only a comment states today.** The token is read from
the `Authorization` header and never from the URL, and the WebSocket's query
parameter is the one exception because a browser cannot set a header on that
handshake. An unknown and a revoked token get one body, so the route is not a
token-probing oracle. Every slice a list route answers starts non-nil. `Compiled`
and `Conditions` are maps, which is why abilities and conditions are sorted;
resources are a slice in declared order. `/api/me` reveals nothing the caller
did not prove by holding the token; control is `Actor.controller_ids` in the
log, decided in `authz.go`. The three HTTP role maps are not cells of
`commandRoles`, whose cell count `TestEveryClientCommandHasRoleCells` holds;
`participantRoles` is a separate map from `joinLinkRoles` by decision
(SPEC-009's). `handleMaps` copies the entries out under `mapsMu.RLock` and
writes the response outside it; a zero `CellPx` on a map means it declared
nothing. `DefaultCellPx` duplicates `campaigncfg.DefaultCellPx` so this package
imports no file-reading package, and `cmd/vtt` holds the equality. The art
route: the name check runs before anything is opened; the mux's single-segment
`{file}` is a second layer whose independent contribution is nil against an
encoded slash; the root is opened per request and nothing is cached; `fs.Stat`
runs on the same `fs.FS` the serve uses, because `os.DirFS`'s `Stat` follows a
symlink out; the content type is set before `http.ServeFileFS` because
`serveContent` infers one only when none is set; `no-cache` rather than
`no-store` so `Last-Modified` turns a reload into a 304; absent and unopenable
roots answer alike; no body names a path. `WithRuleset` pairs the ruleset with a
fresh `rules.NewCryptoRoller` and the roller is not configurable apart from it.
`WithAdventures` is keyed by `Adventure.ID`, not the directory name; guides are
read by `cmd/vtt` at boot so an unreadable one fails there and never as a 500
mid-session. Both batch handlers stamp `EventId`, `ParticipantId`, `ActorRole`
and `OccurredAt` because `Resolve` and `Compile` leave them zero and
`store.AppendBatch` requires an `EventId`. The warnings of a `load_adventure`
ride on the issuer's result and nothing broadcasts them. The residue (D4).

**Warnings to keep, imperative, per file.** `metadata.go`: inside `authed`,
`Answer an unknown and a revoked token alike: telling them apart is a
token-probing oracle (SPEC-009).`; above `adventureGuideRoles`, `Keep the HTTP
gates out of commandRoles: its keys are ClientCommand fields and
TestEveryClientCommandHasRoleCells counts them.`; above `participantRoles`,
`Keep this a separate map from joinLinkRoles: widening one must not widen the
other (SPEC-009).`; inside `handleRuleset`, `Keep every slice non-nil: a JSON
null where the client expects an array crashes its first .map() (VTT-NNN).` and
`Sort: Compiled is a map, and an unsorted list reshuffles the picker on every
request (VTT-NNN).`; inside `handleMe`, `Do not answer control here: the log's
ActorControlGranted is the only authority (VTT-NNN).`; inside `handleAdventures`,
`Keep the list open to every role and the guide gated: a guide holds the DM's
secrets (SPEC-012).`; above `DefaultCellPx`, its doc sentence and `Keep it equal
to campaigncfg.DefaultCellPx without importing that package
(TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber).`; inside
`handleMaps`, `Copy the entries out under mapsMu and write outside it: a client
that stops reading must not hold the map set shut against load_map.` and `Zero
means the map declared nothing; this is the inheritance (SPEC-012).`; above
`artContentTypes`, `Keep this allowlist closed and set the type before
ServeFileFS, or net/http infers one (SPEC-012).`; above `handleArtFile`, `Check
the name with artlib.IsArtFileName before opening anything: the {file} pattern
does not stop an encoded slash and os.OpenRoot confines without flattening
(SPEC-012).`; inside it, `Answer an absent and an unopenable root alike, and
name no path: the body reaches every seat (VTT-NNN).`, `Stat on root.FS(),
never on the path: os.DirFS follows a symlink out and would mask the
confinement (artfile_internal_test.go).`, `Keep no-cache: without it a browser
reuses month-old art for days (VTT-NNN).`. `ruleset.go`: above
`handleUseAbility`, `Answer every failure as an ok=false result, never a close:
authorization already ran in handleCommand (SPEC-012).`; inside it, `Stamp the
four fields Resolve leaves zero: store.AppendBatch requires an EventId.`
`adventure.go`: above `handleLoadAdventure`, the same refusal warning; inside,
D4's two lines at the `Compile` call, the same stamping line, and `Return the
warnings to the issuer only; nothing broadcasts them (SPEC-012).` `server.go`:
D7's one warning at `WithRuleset`.

**Text that goes on sight.** Every `spec §`, `Task N`, `task-N`, `#NNN`, every
date, `measured`, `used to`, `previously`, `an earlier version`, `review
found`, `corrected in review`, every ADR number, every plan name, every mention
of the pack route and of `packRefJSON`, `WithPackFiles`, `mossy-keep`, the
"HONEST NOTE", "obituary" and "rubble" paragraphs, the RFC citations, the
localStorage argument, the cost comparison with `os.DirFS`, the `.DS_Store`
aside, every sentence that says what the code does rather than what to keep,
and every section banner.

Projection, not a target: `metadata.go` near 12 percent, `ruleset.go` near 13,
`adventure.go` near 20, `server.go` near 53 (its out-of-scope blocks stay).
The reading governs; nothing is cut for a figure.

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository; the scratchpad, `$S`.

Build D11's program (the listing is the identity plan's Task 0; a `go.mod` with
`module codetokens` beside it); run the loop (eight `same`). Run `python3
tools/check-comments.py --report | grep -E 'internal/gateway/(metadata|ruleset|adventure|server|metadata_test|adventure_test|ruleset_test|artfile_internal_test)\.go'`,
`task check:requirements-chain`, `python3 tools/check-comments.py main`,
`python3 tools/check-doc-owner.py .`, `gofmt -l internal/gateway/`, `grep -rn
'deliberate exception' internal/gateway/*.go`, and `grep -c 'VTT-' ` over the
four test files; keep the outputs. Confirm `requirement-id` is on the path.

**Done when:** the outputs match the measurements above (eight `same` with the
counts; `110 rows ... 5 specifications`; `239 files ... clean`; only
`scenario_test.go` from gofmt; two `deliberate exception` lines; zero
citations); `requirement-id` prints its one-line usage.

### Task 1 — SPEC-012

**Files:** `docs/specifications/012-the-read-surface.md`, new.

Per D1, D2, D3, D4's paragraph, D9's rule, D23's sentence; the `specification`
skill's steps 1 to 5 and 7 (`catches.md`, two passes at most). Step 6's
Requirements line is written `None yet; the sort of
2026-09-27-the-read-surface-has-a-record-design.md fills it` until Task 8 and
then copied from the register.

**Done when:** the file has the five headings in order (`grep -c '^## '` prints
5), no `Why`, no `Rejected`, no number that is a measurement, no line number,
no date outside a `docs/` path, no path outside the project, no other project's
name; every sentence under "How it works" names a symbol in the four files,
`map.go`, `authz.go`, `internal/artlib`, `internal/adventure`,
`internal/campaign`, `internal/engine` or `cmd/vtt`, and a table of sentence to
symbol is kept for Task 9; `task check:requirements-chain` prints `6
specifications`.

### Task 2 — `server.go`, the five blocks

**Files:** `internal/gateway/server.go`.

D7's list, under D8. `grep -c 'deliberate exception' internal/gateway/server.go`
prints 0.

**Done when:** D11 prints `same` for `server.go` with 4596 tokens; the five
blocks are each at most six lines and hold only a doc sentence, a pointer or
D7's warning; `go doc ./internal/gateway WithRuleset` and `WithAdventures` each
print one sentence; `gofmt -l internal/gateway/server.go` prints nothing;
`--report` shows `server.go` at `blocks>6 29`.

### Task 3 — `metadata.go`

**Files:** `internal/gateway/metadata.go`.

D8 whole; the warnings named above; the package-doc block deleted; every fact
of the "RULING" block, the `packRefJSON` obituary and `handleArtFile`'s doc
checked against SPEC-012's draft, each gap into SPEC-012 (D9) or the report's
dropped list.

**Done when:** D11 prints `same` with 2065 tokens; `--report` shows
`metadata.go` with `banned 0` and `blocks>6 0`; `grep -c 'deliberate exception'
internal/gateway/metadata.go` prints 0; `go doc ./internal/gateway DefaultCellPx`
and `WithAdventureGuides` each print one sentence; `gofmt -l` prints nothing
for it; `python3 tools/check-doc-owner.py .` still ends `79 files, every doc
comment sits on its own function`.

### Task 4 — `ruleset.go`, `adventure.go`, the debt entry

**Files:** those two, and `docs/verification-debt.md` (one entry under `## Open
debt`, D4).

D8 whole; the warnings named above; "above" for `errNoRulesetLoaded` gone with
the block that carried it; the two TOCTOU paragraphs into the entry.

**Done when:** D11 prints `same` for both (287, 341); `--report` shows each with
`banned 0` and `blocks>6 0`; the entry carries two labels from the file's
table, names `handleCommand`, `authorize`, `checkCollisions`, `AppendBatch` and
the `NoteUpserted` arm, and the edit that would show the defect; `grep -c
'verification-debt' internal/gateway/adventure.go` prints 1.

### Task 5 — The test files' re-aims and forced sorts

**Files:** `internal/gateway/metadata_test.go`, `adventure_test.go`,
`artfile_internal_test.go` (D22; `ruleset_test.go` is not touched here).

D22's five items (the fifth per Q4) and the one free re-aim, under D8's three
kinds. No citation line yet.

**Done when:** D11 prints `same` for the three (9214, 3364, 1140); no block in
the three files that this task touched exceeds six lines; `grep -n 'doc
section\|TOCTOU\|adventure.go:[0-9]' ` over the four test files prints nothing
(the last per Q4); every pointer written resolves to a symbol, a test, a
specification or the debt file; `python3 tools/check-comments.py main` refuses
nothing but the band (the files' shares fell) and no banned term.

### Task 6 — Local gates, first pass

**Files:** none changed.

D14's list up to and not including `task check` whole.

**Done when:** each step exits 0 with its completion line, except
`check:comments`, which at this point refuses the seven files for the band and
nothing else; `check:new-prose` reports no citation to a name the tree never
declared; `check:doc-owner` ends `79 files ...`.

### Task 7 — Sign-off of the sort

**Files:** none.

D5's table is presented with the reading's confirmations and overrides, each
override with its reason, and the questions below answered. Nothing is
dispensed before the answer.

**Done when:** each lettered row has one of: accept (with wording and
evidence), refuse (with reason), OPEN (with reason); Q1 to Q12 have answers.

### Task 8 — Rows and citation lines

**Files:** `docs/requirements.md` (by the dispenser, then evidence cells by
hand), `internal/gateway/metadata_test.go`, `adventure_test.go`,
`ruleset_test.go`, `artfile_internal_test.go` (citation lines only, D6),
`docs/specifications/012-the-read-surface.md` (the Requirements line, copied).

**Done when:** `task check:requirements-chain` prints `<110 + N> rows, 193 test
files, 6 specifications; every citation resolves and every row's evidence
holds` with N the accepted count; an OPEN row, if any, has no citer (`grep -rn
'VTT-1NN' internal/` prints nothing); D11 prints `same` for the four test files;
`--report` shows `cites` equal to the number of citation lines per file and the
four files' shares unchanged from Task 5's.

### Task 9 — Phase 4b, the reading review

**Files:** whatever its findings touch among the above.

Per D16. Findings are fixed and the affected task's "done" is re-run. The
review settles before Task 10 starts; if the reviewer dies on a model's limit,
say so and re-dispatch with the same brief.

**Done when:** the review record names every surviving block's kind, every
deleted block's outcome, every SPEC-012 sentence's symbol and verdict, every
row's red-making edit, the debt entry's labels, and reports no open finding;
D11 prints `same` for all eight files.

### Task 10 — The ledger

**Files:** `tools/comment-ceilings.txt`, by `--write-ledger` only.

Per D12.

**Done when:** `git diff tools/comment-ceilings.txt` shows exactly the seven
rows D12 names, each lowered; `task check:comments` ends `clean`; `--report |
grep -E 'internal/gateway/(metadata|ruleset|adventure)\.go'` prints `banned 0`
and `blocks>6 0` on all three lines and `server.go` at `blocks>6 29`.

### Task 11 — The breaks and the whole gate

**Files:** none in the repository.

D13 in a scratch clone; then `task check` whole, once, per D14.

**Done when:** the clone's clean run exits 0 with each completion line, and
each of B1 to B7 produces the one red D13 names; `task check` exits 0 with every
step, `check:comments`, `check:requirements-chain`, `check:doc-owner` and
`check:mutation` among them, printing its own verdict.

### Task 12 — Commit, then the report

**Files:** the commit's, per D10; then
`docs/reports/2026-09-27-the-read-surface-has-a-record.md`.

The commit message lists the seven ledger rows old and new, D11's counts, the
rows dispensed, and B4's finding line verbatim. After it: the report per D17
and the `implementation-report` skill, in its own commit.

**Done when:** `git show --stat HEAD~1` lists the ticket, this plan, SPEC-012,
the debt file, the register, the four production files, the four test files
and the ledger, and nothing else; `git diff --stat a330040 -- docs/reports/`
lists only the new report; `git diff --quiet a330040 -- internal/gateway/map.go
internal/gateway/authz.go internal/gateway/seat.go cmd/ contract/ client/src`
exits 0.

## Commits

| Commit | Carries | Gate steps it runs |
|---|---|---|
| C1 | the ticket, this plan, SPEC-012, the debt entry, the register, `metadata.go`, `ruleset.go`, `adventure.go`, `server.go`, the four test files, the ledger | D14's list by hand, `task check` whole (Task 11), then the pre-commit hook (lint, vet, tier-1, arch, vocabulary, doc-owner, secrets, typecheck, review gate) |
| C2 | the implementation report | pre-commit hook |

Push after C2: pre-push runs tiers 2 and 3 and the contract gates, about three
minutes; let it finish. No Go code changes, so no mutation key moves and
`check:drift` has no client change to compare.

## Gaps that travel with this plan

1. **`/api/ruleset` to every role is unobserved.** Only the DM's token drives
   it in `metadata_test.go`; `TestMetadataRulesetGuideServedForEveryRole` drives
   the guide, not the ruleset. Row G is worded on the four observed routes;
   D23 says why no test is added.
2. **"Not told apart" is observed as two 401s, not as one body.**
   `TestMetadataRejectsBadMissingAndRevokedTokens` asserts the status of each
   case and compares no bodies. Row A is worded on the 401; SPEC-009 and
   SPEC-012 state the rest.
3. **Resources are not sorted.** The ticket's "their lists are sorted by id"
   over-claims by one list; `rs.Resources` is a slice appended in the
   ruleset's declared order. Row H names the four sorted lists; SPEC-012
   states the resources' order. The ticket is not edited.
4. **`adventure.go:76` will rot.** The sweep moves the lookup line the citation
   in `TestLoadAdventureWithMultipleAdventuresLoadedServesRequestedContent`'s
   doc names by number; rule 8 says such a citation fails silently. Q4.
5. **Two pointers outside the four test files aim at cut text.** `map_test.go`'s
   stamping assertion and `client/src/view/pack-assets.ts`. Q5 for the first;
   the second is `client/src`'s sweep.
6. **The ADR-008 attribution resolves to nothing.** Ten comments credit ADR-008
   with "cmd/vtt owns the filesystem"; ADR-008 says thin commands delegate all
   logic to `internal/` and does not mention files. This sweep drops the four
   attributions in its files and SPEC-012 states the fact with its search; the
   other six are named in the report.
7. **The four join-link and roster tests are uncited.** No row states the
   `/api/join-link` or `/api/participants` gate, and SPEC-009 owns both. Not
   this sort's; the report names them for SPEC-009's next ticket. Q7.
8. **Two rule-8-forbidden citations in `adventure_test.go`.** The header cites
   a brief and `TestLoadAdventureProducesBatchFirstSequenceReachesAllParticipants`'s
   doc cites a gitignored report, both by `.superpowers` paths. Neither block
   is one a re-aim forces to the bound; the test-prose sweep's.
9. **"Appends nothing" and "nobody else" are unobserved.** The collision test
   reads a follow-up result, not the log's length; the warnings test reads one
   connection. Rows W and Y are worded on what the tests observe.
10. **The residue is not reproducible today.** The entry records the edit that
    would show it, not a run.
11. **Item 6 is an invariant, not a done that fails today.** D11 holds it at
    every task.
12. **Row count after the sort.** The ticket's "more than 110" holds for any
    N ≥ 1; the report states N.
13. **The client README the client ticket names does not exist.** SPEC-012 is
    the first record of the shapes; the ticket is historical and not edited.
14. **`New`'s block and `Handler`'s inner warning** are outside the ticket's
    `server.go` list; the first describes behaviour (a server without a
    ruleset refuses `use_ability`) and the second reads as if the `{file}`
    pattern kept `art/` flat. Both stay and are named in the report.

## Questions for sign-off

1. **Does SPEC-012 name MapTool?** Recommend no (D3): the record states the
   role gate as a fact about this system, and the borrowing and the
   checked-and-rejected model are the rule-9 answer in this plan and the
   report.
2. **Row A worded on the 401 alone, with "not told apart" as prose?** Recommend
   yes: no test compares the bodies, and a row nothing observes is the
   register's one failure mode; the alternative is an OPEN row that nothing
   cites.
3. **Row AA, the first sequence of the batch: accept under SPEC-012 or refuse
   as SPEC-007's convention?** Recommend accept: SPEC-007 carries no rows,
   the two batch tests observe the rule on a multi-event batch, and a client
   reads a batch by it; the codec round-trips were refused last time because
   they observe an encoding, not a rule a client relies on.
4. **Re-aim the `adventure.go:76` citation at the lookup by name, sorting its
   29-line block to the bound?** Recommend yes: this ticket is the change that
   moves the line, and leaving a citation it knows to be wrong is the silent
   failure rule 8 exists for; the cost is one more sorted block, under D22.
5. **Re-aim `map_test.go`'s four-line pointer at `docs/verification-debt.md`
   in this change?** Recommend yes: the block is under the bound, the edit is
   one line, and the file gains no other change; the alternative is a pointer
   known to be false landing on `main`. If no, it is named in the report.
6. **Rows R and S as one row or two?** Recommend two: each is one test and one
   edit (a picture's content type; a sidecar's disposition) that reds
   separately.
7. **Cite the four join-link and roster tests here?** Recommend no: no row
   exists for them, and dispensing one would put a SPEC-009 rule on SPEC-012's
   line; name them in the report.
8. **A row for the `/api/ruleset` shape?** Recommend no: a list of fields is a
   list, SPEC-012 states the shape, and the test holds the client's contract
   as a test does.
9. **Row A's id on SPEC-012's Requirements line, SPEC-009 untouched?**
   Recommend yes (D24): the observation is the read surface's and the ticket's
   "Specifications this moves" names no edit to SPEC-009.
10. **The debt entry's labels: `test data missing` and `outside the tool`?**
    Recommend both: no fixture reaches the window, and no mutation operator
    opens one; `spec silent` is wrong once SPEC-012 states the rule.
11. **One commit for the change, the report separately (D10)?** Recommend yes.
12. **If a swept file lands with every surviving line a true warning, pointer
    or doc sentence and a block still holds one over the bound?** Recommend
    the reading governs: stop, report the block, and let the ticket's writer
    decide, rather than cut a true warning for the bound.
