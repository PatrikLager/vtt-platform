# The HTTP read surface has a record

## The problem

What a client may read from the gateway over HTTP, and what the server holds
from boot to answer it, is decided in `internal/gateway/metadata.go`
(`authed`, `handleMe`, `handleRuleset`, `handleRulesetGuide`,
`handleAdventures`, `handleAdventureGuide`, `handleMaps`, `handleArtFile`,
the role maps `adventureGuideRoles`, `joinLinkRoles` and `participantRoles`,
`artContentTypes`, `DefaultCellPx`, `WithAdventureGuides`), in
`internal/gateway/ruleset.go` (`handleUseAbility`, `errNoRulesetLoaded`), in
`internal/gateway/adventure.go` (`handleLoadAdventure`,
`errNoAdventuresAvailable`) and in `internal/gateway/server.go` (`Handler`'s
nine `GET /api/*` patterns, the `ruleset`, `roller`, `adventures` and
`adventureGuides` fields of `Server`, `WithRuleset`, `WithAdventures`), and
no specification states it. Every `/api/*` request carries its token in an
`Authorization: Bearer` header and `authed` answers a missing, unknown or
revoked one with the same 401; `/api/me` answers the caller's id, name and
role and nothing about what they control; `/api/ruleset` and `/api/adventures`
answer every role, with empty collections and 200 when nothing is loaded;
abilities, conditions, adventures and maps are listed in id order and
resources in the ruleset's declared order; `/api/ruleset/guide` answers every role and
404 when there is no guide; `/api/adventures/{id}/guide` answers the DM and
the agent, 403 to a player or spectator, and 404 for an unknown id; `/api/maps`
answers every role with each map's grid and its cell size resolved against the
campaign's, and the campaign's beside it; `GET /api/art/{file}` serves one
regular file out of the campaign's flat art directory after `authed` and
`artlib.IsArtFileName`, through `os.OpenRoot`, a picture inline as
`image/png` and a sidecar as an `application/octet-stream` attachment, both
`nosniff` and `no-cache`, and answers 404 for every other name, for a
subdirectory in any spelling, for a directory wearing an art name, for no
art directory and for one that cannot be opened, naming no path in any body.
The ruleset and the adventures are set once, before serving: `WithRuleset`
pairs the ruleset with `rules.NewCryptoRoller`, `WithAdventures` takes a map
keyed by manifest id and `WithAdventureGuides` the markdown beside it, none
read from disk in this package; `handleUseAbility` and `handleLoadAdventure`
turn a missing ruleset, missing adventures or an unknown id into an ok=false
result on a connection that stays open, append their batch atomically, and
return the batch's first sequence, `handleLoadAdventure` with the compile's
warnings for the issuer alone. Each of those sentences lives today in a
ticket's paragraph (`docs/superpowers/specs/2026-07-26-client-design.md` §3
and §5, `2026-07-25-ruleset-interpreter-design.md` §7,
`2026-07-26-adventure-format-design.md` §7), in a report
(`docs/reports/2026-09-02-art-is-a-flat-library.md` §2 and §4.3 for the art
route) or in a comment block; SPEC-009 carries the bearer sentence and the
`/api/join-link` and `/api/participants` routes, and nothing carries the rest.
`metadata.go` holds 412 comment lines of 698 non-blank (`b7b5b35` and
`a330040` alike; `python3 tools/check-comments.py --report` gives 59.0
percent, 36 banned lines, 12 blocks over the bound with the package doc
excepted), `ruleset.go` 43.3 percent with 3 blocks over, `adventure.go`
63.6 percent with 3 over. Three sentences among them are false: the
`WithAdventureGuides` comment in `metadata.go` and the `adventureGuides` field
comment in `server.go` each say the rule that `cmd/vtt` owns the filesystem
has "exactly one deliberate exception", `mapByID`'s probe, while
`handleArtFile` opens the art directory on every request; and `adventure.go`
says `errNoRulesetLoaded` is declared "above" when it is declared in
`ruleset.go`. One residue is recorded only in a comment: `adventure.Compile`
checks note-key collisions against the snapshot `handleCommand` took before
the call, and `campaign.AppendBatch`'s re-fold does not repeat that check
because `engine.Apply`'s `NoteUpserted` arm is an upsert, so an `upsert_note`
landing on the same key between the two calls is overwritten rather than
refused. No test holds it and no record names it.

## Done looks like

1. `docs/specifications/012-the-read-surface.md` exists with the five
   headings SPEC-007 uses, in the present tense: the bearer and its one
   401, the nine `GET /api/*` routes with each one's role gate and what it
   answers when the content is absent, the maps list's cell-size
   resolution, the art route's guard, confinement, content types and
   caching, the content set at boot and the two commands that spend it;
   every sentence names the symbol that holds it, and Phase 4b reads each
   against the code. `/api/join-link` and `/api/participants` point at
   SPEC-009 and are not restated.
2. `grep -rn 'deliberate exception' internal/gateway/*.go` prints nothing
   (today it prints two lines), and SPEC-012 names both places this package
   reads the filesystem at request time, `mapByID` and `handleArtFile`.
3. `docs/verification-debt.md` gains the note-key residue under "Open debt",
   with the edit that would put a test on it, and `adventure.go`'s comment
   points at the entry instead of carrying the argument.
4. Every rule the sort accepts has a row cited by a test under
   `internal/gateway/` that observes it; `task check:requirements-chain`
   prints more than 110 rows and holds; the report names any OPEN row.
5. `python3 tools/check-comments.py --report | grep -E 'gateway/(metadata|ruleset|adventure)\.go'`
   prints `banned 0` and `blocks>6 0` on all three lines, and every block
   left in them is a warning, a pointer or the doc sentence of an exported
   symbol (Phase 4b's reading, VTT-051); `server.go`'s blocks on the
   `ruleset`, `roller`, `adventures` and `adventureGuides` fields and on
   `WithRuleset` and `WithAdventures` are pointers to SPEC-012 or warnings,
   and `server.go`'s `blocks>6` falls from 34 to 29. The ledger rows are
   lowered by `--write-ledger` in the same commit.
6. No code line changes: the go/scanner token stream of `metadata.go`,
   `ruleset.go`, `adventure.go` and `server.go` is identical to `a330040`'s.
7. `task check` whole is green.

## Rules this puts on the system

Candidates, one line each, for the sort after sign-off; most have a test
already (`internal/gateway/metadata_test.go`, `adventure_test.go`,
`ruleset_test.go`, `artfile_internal_test.go`), and a test whose rule an
existing row states cites that row.

- A `/api/*` request without a bearer token the table knows, or with a
  revoked one, is answered 401, and the two are not told apart.
- `/api/me` answers the caller's id, name and role, and never what they
  control.
- A server with nothing loaded answers a list route with 200 and empty
  collections, and a guide route with 404.
- An adventure guide is answered to the DM and the agent only; the adventure
  list, the ruleset, the ruleset guide, the maps list and the art are
  answered to every role.
- The abilities, conditions, adventures and maps a read route lists are in
  id order.
- A maps entry carries the map's own cell size when it declares one and the
  campaign's otherwise, and the campaign's is answered beside the list.
- The art route serves a regular file whose name is an art filename and
  nothing else: a name in any other shape, a subdirectory in any spelling, a
  directory wearing an art name and a symlink out of the directory are 404.
- An absent or unopenable art directory is 404 at request time, never 500,
  and no art response names a path.
- A picture is served inline with its real content type and a sidecar as an
  attachment, both with `nosniff`.
- Art is served `no-cache`, so a file overwritten in place reaches the browser
  on its next request, and a file installed after the server started is
  served without a restart.
- `use_ability` without a ruleset, and `load_adventure` without adventures or
  with an unknown id, is refused with an ok=false result and the connection
  stays.
- A `load_adventure` whose scenes, actors or tokens collide with the table is
  refused whole, appends nothing and poisons nothing.
- A `load_adventure`'s warnings reach the issuer and nobody else, and their
  total is bounded so a broken bundle cannot push the result past the read
  limit.
- A `use_ability` or `load_adventure` result carries the first sequence of
  the batch it appended.

## What it touches

1. `docs/specifications/012-the-read-surface.md`, new
2. `docs/verification-debt.md`, one entry under "Open debt"
3. `internal/gateway/metadata.go`, `ruleset.go`, `adventure.go`, comments
   only
4. `internal/gateway/server.go`, the comment blocks on the `ruleset`,
   `roller`, `adventures` and `adventureGuides` fields and on `WithRuleset`
   and `WithAdventures` only
5. `internal/gateway/metadata_test.go`, `adventure_test.go`,
   `ruleset_test.go`, `artfile_internal_test.go`: citation lines where a row
   is dispensed or an existing one cited, and any comment block a re-aimed
   pointer sits in, sorted to the bound (`check:comments` refuses a line
   added to a block over it)
6. `docs/requirements.md`, rows after sign-off, by the dispenser
7. `tools/comment-ceilings.txt`, by `--write-ledger` only

One component; the specification first, the register with it, the comment
sort after, the ledger last, in one commit; the report in its own.

## Specifications this moves

New: the gateway's HTTP read surface — the bearer, the routes and their role
gates, the maps list, the art route, the content set at boot and the two
commands that spend it.

## What could not be established

- Whether the note-key residue is reachable at a table: it needs an
  `upsert_note` from another connection landing on the same key between
  `handleCommand`'s `campaign.State()` and `handleLoadAdventure`'s
  `AppendBatch`, and nothing drives that; the entry records the edit that
  would show it, not a reproduction.
- Whether `/api/maps` belongs here or to the maps record: the route and its
  cell-size resolution are `metadata.go`'s and are read here; what a map is,
  how it loads and `mapByID`'s probe are `map.go`'s and the next ticket's.
  SPEC-012 points there for the latter.
- Whether the dice rule ("rolled once, recorded on the event, never
  re-rolled on replay") is this record's: `WithRuleset` fixes the roller,
  but the roll and its recording are `internal/rules`'s; SPEC-012 states the
  gateway's half and points at the ruleset ticket for the rest, until that
  package has a record.
- Which of the four test files' comment blocks the pointer re-aims will
  force to the bound; the plan lists them after the sweep's pointers are
  known.
