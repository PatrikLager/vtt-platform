# SPEC-012: What a seat may read over HTTP is decided on the server, per request

## Status

Accepted. Implemented by `internal/gateway/metadata.go` (`authed`,
`writeJSON`, `handleMe`, `handleRuleset`, `handleRulesetGuide`,
`handleAdventures`, `handleAdventureGuide`, `handleMaps`, `handleArtFile`,
`adventureGuideRoles`, `artContentTypes`, `DefaultCellPx`,
`WithAdventureGuides`), `internal/gateway/ruleset.go` (`handleUseAbility`),
`internal/gateway/adventure.go` (`handleLoadAdventure`) and
`internal/gateway/server.go` (the `ruleset`, `roller`, `adventures` and
`adventureGuides` fields of `Server`, `WithRuleset`, `WithAdventures`;
`Handler` registers the routes, SPEC-011), fed by `cmd/vtt/serve_compose.go`'s
`composeServer`; pinned by `internal/gateway/metadata_test.go`,
`adventure_test.go`, `ruleset_test.go`, `artfile_internal_test.go` and, for
the default cell size, `cmd/vtt/art_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: what a seat may read is decided on the server, on every request, and
never by a client hiding what it was sent.

## How it works

**The bearer, and its one 401.** Every `/api/*` handler calls `authed` first.
`authed` reads the `Authorization` header, requires the `Bearer ` prefix with
`strings.CutPrefix`, and hands the rest to `identity.Verify`; a missing
header, a header without the prefix, an unknown token and a revoked token are
all answered with status 401 and the one body `gateway: unauthorized`, so the
route tells a caller nothing about which token it tried. The token travels in
a header and never in the URL; the WebSocket handshake's `?token=` is the one
exception, because a browser sets no header on that handshake (SPEC-011). What
`identity.Verify` promises, and that an unknown and a revoked credential are
answered alike, is SPEC-009's.

**The routes, and their gates.** `Handler` registers nine `GET /api/*` patterns
(SPEC-011 lists them beside `/healthz`, `/ws`, `POST /join` and the static
route). Seven are this record's: `/api/me` (`handleMe`), `/api/ruleset`
(`handleRuleset`), `/api/ruleset/guide` (`handleRulesetGuide`),
`/api/adventures` (`handleAdventures`), `/api/adventures/{id}/guide`
(`handleAdventureGuide`), `/api/maps` (`handleMaps`) and `/api/art/{file}`
(`handleArtFile`). `/api/join-link` and `/api/participants`, the DM's console
routes, are SPEC-009's, with their role maps `joinLinkRoles` and
`participantRoles`. Of the seven, one is gated by role: `handleAdventureGuide`
answers the roles in `adventureGuideRoles`, the DM and the agent, and refuses a
player or a spectator with status 403 and the body `gateway: not authorized`,
because an adventure guide holds the DM's secrets. The other six answer any
participant `authed` returns, whatever the role, and `handleRuleset` and
`handleRulesetGuide` consult no role at all. The HTTP role maps are not cells
of `commandRoles` in `authz.go`: that table's keys are the `ClientCommand`
oneof's fields, and `TestEveryClientCommandHasRoleCells` walks that oneof.

**Empty is not an error, and a missing guide is.** `handleRuleset` builds its
answer with every slice non-nil (`Abilities`, `Conditions`, `Resources` start
as empty slices), and `handleAdventures` and `handleMaps` start from an empty
slice, so a server with nothing loaded answers `/api/ruleset`,
`/api/adventures` and `/api/maps` with status 200 and `[]` where a client
expects an array, never `null`. A guide is different: `handleRulesetGuide`
answers 404 with `gateway: no ruleset guide available` when there is no
ruleset or its `Guide` is empty, and `handleAdventureGuide` answers 404 with
`gateway: no guide for that adventure` when the id names no guide or an
empty one, because an empty document would read as a broken one.

**What `/api/me` answers.** `handleMe` writes `meJSON`: the `participantId`,
`name` and `role` of the `identity.Participant` `authed` returned, and
nothing else. It answers nothing about what the caller controls: control is
an `ActorControlGranted` in the log, decided in `authz.go` against the fold
(SPEC-009), and a client reads it from the folded actors' controller ids. The
route reveals nothing the caller did not prove by holding the token.

**What `/api/ruleset` answers.** `handleRuleset` writes `rulesetJSON`: the
ruleset's `id` and `name`, `abilities` as `abilityJSON` (`id`, `name`,
`range`, `maxTargets` and `usage`, which is `usageJSON` with `kind` `atWill`,
or `kind` `resource` with `resource` and `cost`), `conditions` as
`conditionJSON` (`id`, `name`, `description`), and `resources` as the names
of the ruleset's resource definitions. Abilities and conditions are sorted by
id with `slices.SortFunc`, because `rules.Ruleset.Compiled` and `Conditions`
are maps and Go randomises map iteration; resources are a slice and keep the
ruleset's declared order. `handleRulesetGuide` writes `{"guide": ...}` with
the ruleset's `Guide` markdown.

**Adventures, and the guide.** `handleAdventures` writes
`{"adventures": [...]}` with an `adventureJSON` (`id`, `name`) per adventure
in `Server.adventures`, sorted by id; the list is answered to every role,
because which adventures exist is not a secret. `handleAdventureGuide` looks
the path's `{id}` up in `Server.adventureGuides` and writes `{"guide": ...}`
with the markdown, to the roles above.

**`/api/maps`, and the cell size.** `handleMaps` copies the entries of
`Server.maps` out under `mapsMu.RLock` and writes the response after releasing
it, so a client that stops reading holds no lock against a `load_map`. Each
`mapMetaJSON` carries the map's `id`, `name`, `gridWidth`, `gridHeight` and
`cellPx`; `cellPx` is the map's own `CellPx` when it declares one and
`Server.cellPx` when the map's is zero, which is what a map that declared
nothing carries. The list is sorted by id, and the response carries the
campaign's own `cellPx` beside it, so a client knows both what this map draws
at and what the next map that declares nothing will draw at. `New` sets
`Server.cellPx` to `DefaultCellPx`, and `WithCellPx` overrides it with the
campaign's; `DefaultCellPx` is 64 and duplicates `campaigncfg.DefaultCellPx` on
purpose, because `.go-arch-lint.yml` gives `gateway` no edge to `campaigncfg`
(`WithCellPx` takes a number, not a path), and `cmd/vtt`'s
`TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber` holds the two
equal. How `load_map` looks a map up and grows the set is SPEC-014's; what a
map is, `internal/mapdef`'s.

**The art route.** `handleArtFile` serves one file out of the campaign's flat
art directory, `Server.artDir`, and is the only route that hands a browser raw
bytes an operator installed under a content type of their own; a guide's
markdown travels JSON-wrapped. The name is checked before anything is opened:
`artlib.IsArtFileName` admits exactly `<id>.png` and `<id>.json`, the same rule
a map load resolves art by, and everything else, a subdirectory in any spelling
included, is answered 404. The route's single-segment `{file}` pattern is not a
guard: `ServeMux` decodes `%2F` before matching, so `/api/art/a%2Fb.png`
reaches `PathValue` as one segment `a/b.png`, and `os.OpenRoot` confines a
lookup to the directory without flattening it. Then the root is opened with
`os.OpenRoot` on every request and nothing is cached, so a piece installed or
overwritten while the server runs is served on its next request; an art
directory that is unset, absent or unopenable is answered 404, never 500.
`fs.Stat` runs on `root.FS()`, the same `fs.FS` the serve uses, and requires a
regular file, so a directory wearing an art name is 404 and a symlink out of
the directory is not followed. Every served file carries
`X-Content-Type-Options: nosniff` and `Cache-Control: no-cache` (a refusal
carries the `nosniff` that `http.Error` sets and no `Cache-Control`), and the
content type is set before `http.ServeFileFS` runs, because net/http infers one
only when none is set: a name in `artContentTypes`, which holds `.png` alone,
is served inline as `image/png`, and any other name, which is the sidecar, as
`application/octet-stream` with `Content-Disposition: attachment`. The
allowlist is closed, and the sidecar an attachment, because a file served with
a browser-executable content type at a same-origin URL would let script read
the client's bearer token from its storage and call every route as that seat;
markdown a guide route returns is JSON-wrapped and never executed. `no-cache`
rather than `no-store`: a browser may keep the bytes and must revalidate, and
`http.ServeFileFS`'s `Last-Modified` answers an unchanged file's conditional
request with 304 and no body. Every refusal is status 404 with the one body
`gateway: no such art`, and no response names a path on the server, because a
body reaches every authenticated seat.

**What is set at boot.** The content the routes answer is set before the server
serves and read from disk by `cmd/vtt`, not here. `WithRuleset` sets
`Server.ruleset` and pairs it with a fresh `rules.NewCryptoRoller` in
`Server.roller`; nothing configures the roller apart from the ruleset. The roll
itself, recorded on the `AbilityUsed` event and never repeated on replay, is
`internal/rules`' and has no record yet. `WithAdventures` takes a map keyed by
each adventure's own `Adventure.ID`, not the directory it was loaded from, and
does no I/O; `WithAdventureGuides` takes the guides' markdown keyed the same
way, so an unreadable guide fails at boot and never as a 500 mid-session;
`WithCellPx` takes the campaign's number. `composeServer` reads them:
`rules.Load` for the ruleset, `loadAdventuresDir` for the adventures,
`loadAdventureGuides` for their guides, `campaigncfg.Load` for `cell_px`. The
`With*` methods write without a lock and are called before the server serves.
Apart from the campaign's log and the identity database, which `campaign` and
`identity` hold open, this package reads a file at request time through four
calls and no other: `handleArtFile` opens the art directory; `mapByID` runs
`mapdef.LoadInstalled`, which reads the maps directory and the art directory;
and `handleLoadMap` runs `mapdef.Compile` and `handleLoadAdventure` runs
`adventure.Compile`, each of which reads an art directory through
`internal/artlib` (SPEC-014). Over the package's non-test files, `os.` is
called only in `handleArtFile`, and of its calls into `mapdef`, `adventure`,
`artlib` and `rules`, only `mapdef.LoadInstalled`, `mapdef.Compile` and
`adventure.Compile` read a file.

**The two commands that spend it.** `handleCommand` runs `authorize`, which
takes `st := s.campaign.State()`, and passes that snapshot to
`handleUseAbility` (`rules.Resolve(s.ruleset, st, cmd, s.roller)`) or
`handleLoadAdventure` (`adventure.Compile(adv, st)`). Each stamps the four
fields the compile leaves zero, `EventId` from `newEventID`, `ParticipantId`,
`ActorRole` and `OccurredAt`, because `store.AppendBatch` requires an
`EventId`, and appends the whole batch with `campaign.AppendBatch`, which
persists all of it or none. The result carries the first sequence of the
batch, the convention SPEC-007 states for every command, and
`handleLoadAdventure`'s carries the compile's `Warnings` for the issuer;
nothing broadcasts a warning. Every failure is an ok=false `CommandResult` on
a connection that stays open: no ruleset (`gateway: no ruleset loaded`), no
adventures (`gateway: no adventures available`), an unknown adventure id, a
command `rules.Resolve` refuses, a compile collision, an `AppendBatch`
refusal. Authorization ran in `handleCommand` before either handler, so
nothing from here on is an authorization question.

**The note-key residue.** `adventure.Compile` runs `checkCollisions` against
the snapshot `authorize` took, and `campaign.AppendBatch` re-folds the batch
against a fresh snapshot under its own lock, where `engine.Apply`'s
`SceneCreated`, `ActorAdded` and `TokenPlaced` arms refuse a duplicate id and
its `NoteUpserted` arm upserts. So a second load of the same adventure is
refused by `checkCollisions`, or by the re-fold when its snapshot predates the
first, and a note upserted on the same key between the two calls is overwritten
rather than refused. `docs/verification-debt.md` carries it under Open debt.

## Consequences

A client author is bound by these:

- The token goes in `Authorization: Bearer`, never in a URL; a 401 says only
  that the token did not verify.
- `[]` is an empty table and a 404 on a guide is "no guide"; neither is an
  error the UI should raise.
- Who the caller is comes from `/api/me`; what they control comes from the
  log, never from `/api/me`.
- A map draws at its entry's `cellPx`, and the campaign's `cellPx` is what a
  map that declares nothing will draw at.
- Art is fetched as `<id>.png` or `<id>.json` and by no other name; a browser
  reuses no art without revalidating, so an overwrite reaches the next
  reload.
- A refused `use_ability` or `load_adventure` leaves the connection usable,
  and a bundle's warnings arrive on the issuer's result.
- Nothing this surface answers is filtered per seat beyond the role gate; what
  a seat SEES of the campaign is the projection's, not this record's.

## Requirements

VTT-111, VTT-112, VTT-113, VTT-114, VTT-115, VTT-116, VTT-117, VTT-118,
VTT-119, VTT-120, VTT-121, VTT-122, VTT-123, VTT-124, VTT-125, VTT-126,
VTT-127, VTT-128, VTT-129, VTT-130, VTT-131, VTT-132, VTT-133, VTT-134,
VTT-135, VTT-136, VTT-137.
