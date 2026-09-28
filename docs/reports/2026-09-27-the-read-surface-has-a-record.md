# The HTTP read surface has a record: the change

**Ticket:** `docs/superpowers/specs/2026-09-27-the-read-surface-has-a-record-design.md`,
amended at sign-off on the one point its verification found: the problem
paragraph and one candidate said every list a read route answers is sorted by
id, where resources keep the ruleset's declared order.
**Plan:** `docs/superpowers/plans/2026-09-27-the-read-surface-has-a-record.md`,
verified by `verify-ticket` (Passes with gaps); its twelve sign-off questions
were answered at sign-off on 2026-09-27, and this report is where the answers
are recorded: eleven as the plan proposed; 5 yes, with SPEC-012 rather than
`docs/verification-debt.md` as the target, because the comment is about the
four fields the compile leaves zero, which SPEC-012 states, and the debt
entry is about note keys.
**Last commit of the change (no code line changed):** `f2d11c2`, on `a330040`,
`main` at the time.
Every code reference below is to that tree.

## The period, in commits

    git log --oneline a330040..f2d11c2

    f2d11c2 The HTTP read surface has a record: SPEC-012, VTT-111 to VTT-137

`git diff --stat a330040..f2d11c2`: 14 files changed, 1586 insertions(+), 721 deletions(-).

The gate: `task check`, whole, over the tree of `f2d11c2` before it was
committed: exit 0, no step failed, and the check steps'
own verdict lines read `check:comments` `clean`, `check:requirements-chain`
137 rows, `check:doc-owner` 79 files, `check:new-prose` 127 added lines
clean, `check:coverage` 20 packages at or above their floors,
`check:mutation` fourteen packages with zero unadjudicated survivors,
`check:ts-mutation` 2882 mutants, 2783 killed, 29 timed out and counted as
killed by that gate's own rule, 70 survivors all adjudicated, zero
unadjudicated. Before it: `gofmt`, `go vet`, `task lint`, the gateway package,
`check:comments`, `check:doc-owner`, `check:requirements-chain` and
`check:new-prose`, the token instrument, and the breaks; then the pre-commit hook's nine checks.

Phase 4a was skipped: the change moves prose and changes no code line, and
what can be wrong in it is a sentence, which the reading review holds.

## Done looks like, answered

1. `[x]` `docs/specifications/012-the-read-surface.md` exists with SPEC-007's
   five headings (`grep -c '^## '` prints 5 for both files): Status,
   Principles served, How it works, Consequences, Requirements.
   `/api/join-link` and `/api/participants` are named in one sentence that
   points at SPEC-009. The reading review (Phase 4b,
   `pr-review-toolkit:code-reviewer` on `fable`) read its sentences against
   the symbols they name, by command; the six clauses it refuted were
   rewritten before the commit (Deviations), and none was left unverifiable.
   The review is a reading, recorded for the commit gate.
2. `[x]` `grep -rn 'deliberate exception' internal/gateway/*.go` prints
   nothing (it printed two lines at `a330040`), and SPEC-012's boot paragraph
   names `handleArtFile` and `mapByID` as the two places this package reads
   the filesystem at request time, bounded by a search the review re-ran.
3. `[x]` `docs/verification-debt.md` holds the note-key residue as an entry
   under "Open debt", the last at `f2d11c2`, with the internal test that
   would show it; `grep -c 'verification-debt' internal/gateway/adventure.go`
   prints 1, the two-line warning at the `adventure.Compile` call.
4. `[x]` Twenty-seven rows, VTT-111 to VTT-137, each with the tests in its
   evidence cell citing it; `task check:requirements-chain` prints `137 rows,
   193 test files, 6 specifications; every citation resolves and every row's
   evidence holds`. No row of the twenty-seven is OPEN.
5. `[x]` `python3 tools/check-comments.py --report` prints `banned 0` and
   `blocks>6 0` for `metadata.go`, `ruleset.go` and `adventure.go`; that
   every surviving block in them and in `server.go`'s five swept blocks is a
   warning, a pointer or an exported symbol's doc sentence is the reading
   review's verdict (VTT-051), a reading and not a check. `server.go`'s
   `blocks>6` reads 29. The ledger rows moved in the same commit,
   `--write-ledger` run last.
6. `[x]` The go/scanner token stream, comments dropped, of `metadata.go`,
   `ruleset.go`, `adventure.go` and `server.go` is identical to `a330040`'s
   (2065, 287, 341 and 4596 tokens; the instrument is the one plan D11
   names, compared with `cmp` against `git show a330040:<file>`); so are the
   four test files' (9214, 3364, 2009, 1140).
7. `[x]` `task check` whole, exit 0, above.

## What the rules became

| Rule, as the ticket words it | Became |
|---|---|
| A `/api/*` request without a bearer token the table knows, or with a revoked one, is answered 401, and the two are not told apart | VTT-111, narrowed to the three routes its tests drive (`/api/ruleset`, `/api/me`, `/api/art/{file}`); "not told apart" is SPEC-012 prose, since no test compares the two bodies |
| `/api/me` answers the caller's id, name and role, and never what they control | VTT-112, VTT-113 |
| A server with nothing loaded answers a list route with 200 and empty collections, and a guide route with 404 | VTT-114 (narrowed to the collections the test decodes: abilities, resources, adventures, maps), VTT-115 |
| An adventure guide is answered to the DM and the agent only; the adventure list, the ruleset, the ruleset guide, the maps list and the art are answered to every role | VTT-116, VTT-117 (the four observed routes; `/api/ruleset` per role has no test) |
| The abilities, conditions, adventures and maps a read route lists are in id order | VTT-118 |
| A maps entry carries the map's own cell size when it declares one and the campaign's otherwise, and the campaign's is answered beside the list | VTT-119, VTT-120 |
| The art route serves a regular file whose name is an art filename and nothing else: a name in any other shape, a subdirectory in any spelling, a directory wearing an art name and a symlink out of the directory are 404 | VTT-121, VTT-122, VTT-123, VTT-124, VTT-125 (the symlink row worded on "reaches", since its test asserts not-200 and no leak rather than a particular status) |
| An absent or unopenable art directory is 404 at request time, never 500, and no art response names a path | VTT-126, VTT-127 |
| A picture is served inline with its real content type and a sidecar as an attachment, both with `nosniff` | VTT-128, VTT-129 (sign-off question 6, two rows) |
| Art is served `no-cache`, so a file overwritten in place reaches the browser on its next request, and a file installed after the server started is served without a restart | VTT-131, VTT-130 |
| `use_ability` without a ruleset, and `load_adventure` without adventures or with an unknown id, is refused with an ok=false result and the connection stays | VTT-132, one row for the one refusal shape, a command the ruleset refuses included |
| A `load_adventure` whose scenes, actors or tokens collide with the table is refused whole, appends nothing and poisons nothing | VTT-133, worded on what the test observes: refused as a result and the table keeps serving; "appends nothing" is unobserved |
| A `load_adventure`'s warnings reach the issuer and nobody else, and their total is bounded so a broken bundle cannot push the result past the read limit | VTT-135 ("and nobody else" unobserved, SPEC-012 prose), VTT-136 |
| A `use_ability` or `load_adventure` result carries the first sequence of the batch it appended | VTT-137 (sign-off question 3, accepted under SPEC-012) |

One row came from a test rather than the list: VTT-134, a `load_adventure`
loads the adventure whose id it names and no other.

Refused, per the plan's sort: `TestMetadataRulesetShapeMatchesTheContract`'s
field list, a list rather than a rule (question 8); `TestNoPackRouteIsServed`,
an absence `check:no-pack` holds; `TestRemoveConditionAppliedThenRemoved`, the
command path's; `TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber`,
`cmd/vtt`'s; the four join-link and roster tests, SPEC-009's and uncited
(question 7); the batch reaching a second connection in order, SPEC-011's
delivery. Five clauses of the candidates stayed prose: "not told apart", the
ruleset to every role, "a list" for resources, "appends nothing", "and nobody
else".

## The sweep

Comment lines over non-blank lines, counted by `tools/check-comments.py`'s
own `measure` with citation lines set aside, at `a330040` and at `f2d11c2`:

| File | Before | After | Blocks over the bound |
|---|---|---|---|
| `internal/gateway/metadata.go` | 412 / 698 | 38 / 324 | 13 to 0 |
| `internal/gateway/ruleset.go` | 26 / 60 | 4 / 38 | 3 to 0 |
| `internal/gateway/adventure.go` | 68 / 107 | 7 / 46 | 3 to 0 |
| `internal/gateway/server.go` | 715 / 1311 | 681 / 1277 | 34 to 29 |
| `internal/gateway/metadata_test.go` | 455 / 1635 | 393 / 1573 | 20 to 18 |
| `internal/gateway/adventure_test.go` | 184 / 606 | 154 / 576 | 9 to 7 |
| `internal/gateway/artfile_internal_test.go` | 91 / 235 | 58 / 202 | 4 to 3 |

`ruleset_test.go` gained three citation lines and moved nothing else. Ledger
rows, old to new: `metadata.go` 59.1 to 11.8, `ruleset.go` 43.4 to 10.6,
`adventure.go` 63.6 to 15.3, `server.go` 54.6 to 53.4, `metadata_test.go`
27.9 to 25.0, `adventure_test.go` 30.4 to 26.8, `artfile_internal_test.go`
38.8 to 28.8: the seven rows plan D12 named, and no other.

`server.go`'s twenty-nine remaining blocks over the bound, by the symbol each
sits on or in, are the next sweeps': the `Server` struct's field docs (four:
`writeTimeout`, `maps`, `artDir`, `cellPx`), `WithMaps`, `WithMapsDir`,
`WithArtDir`, `WithCellPx`, `describeBlockage`, `answerCommand`, `authorize`,
`handleSetViewpoint`, `handleCommand` (seven: its doc and six
command-conversion blocks in its body), `announcePresence`,
`announceDeparture` (two), `revoked`, `announcePromotion`,
`handleRemoveActor`, `handleJoinDoor` (one, in its body), `handlePromotion`
(two), `credentialGone`.

## The breaks

| Break, one edit in a scratch clone of the changed tree | Red |
|---|---|
| B1 a `// VTT-999` line above `TestRemoveConditionAppliedThenRemoved` | `check:requirements-chain`: `ruleset_test.go cites VTT-999 and no row in docs/requirements.md defines it` |
| B2 VTT-121's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `row VTT-121: internal/gateway/codec_test.go does not carry the id, so the link walks one way only` |
| B3 VTT-999 on SPEC-012's Requirements line | `docs/specifications/012-the-read-surface.md cites VTT-999 ... (specification citation)` |
| B4 a `// SPEC-012` line inside `handleUseAbility`, after the ledger was written | `check:comments: internal/gateway/ruleset.go: comment share 12.82 is above its ceiling 10.6 and this change added a comment line to it (SPEC-010)` |
| B5 `if cellPx == 0` made `if cellPx != 0` in `handleMaps` | the token instrument prints `DIFFERS`, and `TestAMapsOwnCellPxOverridesTheCampaignDefault` is red |
| B6 "exactly one deliberate exception" back in `WithAdventureGuides`' doc | `grep -rn` prints 1 |
| B7 the warning above `handleArtFile` rewritten to open with "Open" | `check:doc-owner`: `the doc comment above handleArtFile begins by describing Open, which is a different function` |

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Task 1's draft of SPEC-012 named `loadAdventuresDir` as the reader of the adventures and their guides. | It names `loadAdventuresDir` for the adventures and `loadAdventureGuides` for the guides. | The review: `loadAdventuresDir` in `cmd/vtt/adventures.go` calls `adventure.Load` per directory and never opens a guide; `composeServer` calls `loadAdventureGuides` after it. |
| The plan's candidates list, D9's input: the duplicated `DefaultCellPx` is explained by this package importing no package that reads files. | It is explained by `.go-arch-lint.yml` giving `gateway` no edge to `campaigncfg`. | The review, by `go list`: the package imports `artlib`, `mapdef`, `adventure`, `rules`, `campaign` and `identity`, each of which reads files, and `mapByID` reads at request time. The same stale claim stands in `cmd/vtt/art_test.go`, outside this ticket. |
| The `adventureGuideRoles` warning and SPEC-012 say `TestEveryClientCommandHasRoleCells` counts `commandRoles`' keys. | Both say the test walks the `ClientCommand` oneof. | The review: the test ranges the oneof's fields and never the map; an HTTP key added to `commandRoles` keeps it green. The rule stands; its reason was false. |
| Plan D5: row A on every `/api/*` route; row D on every collection. | VTT-111 names `/api/ruleset`, `/api/me` and `/api/art/{file}`; VTT-114 names abilities, resources, adventures and maps. | The review: deleting `s.authed` in six of the nine handlers keeps VTT-111's three tests green, and `Conditions: nil` keeps VTT-114's tests green. A row says what its tests observe; the other six routes' 401 and the conditions' `[]` are SPEC-012 prose held by the reading. |
| SPEC-012 said a second load of the same adventure is refused by the re-fold, every art response carries `no-cache`, and the art route is the only one handing a browser operator-installed bytes. | It says `checkCollisions` refuses the second load and the re-fold only one whose snapshot predates the first; every served file carries `no-cache` and a refusal only the `nosniff` that `http.Error` sets; the art route is the only one handing raw bytes under a content type of their own. | Each refuted by the review reading the body: `TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned` asserts `checkCollisions`' wording, the three 404 refusals in `handleArtFile` precede the header sets, and the guide routes return `guide.md` bytes JSON-wrapped. |
| Plan D9: every fact a cut block held goes to SPEC-012 or the report. | One sentence was added at review: the threat the closed allowlist and the attachment answer, script at a same-origin URL reading the client's bearer token. | The review found it in no record; it survived only in `artlib.IsArtFileName`'s doc and a test's message. |
| Plan D22: five test blocks sorted to the bound and one free re-aim. | Six blocks sorted: the `get` helper's doc (two lines, re-aimed and shortened), the headers above `mapsFixture` and `artFixture`, `artfile_internal_test.go`'s file header, and the two `adventure_test.go` docs (question 4 yes). | The gate's refusal of a line added to a block over the bound, as the connection change recorded; the `get` doc was under the bound and was rewritten rather than re-aimed because its second line was the pointer. |
| Sign-off question 5: `map_test.go`'s four-line pointer at `handleLoadAdventure`'s doc re-aimed at `docs/verification-debt.md` in this change. | It was missed in `f2d11c2`, which touches `map_test.go` not at all, and landed as the commit after it, `3c1b94a`, one comment line pointing at SPEC-012 rather than the debt file, the file's tokens identical to `a330040`'s. | The sweep's Task 5 listed the four test files the ticket names and the plan's D22 list, and the question's file was in neither list; the report's own check of question 5 found it. |
| Plan D22: the `mapsFixture` header keeps a warning that the fixture wires no static bundle, on which `TestNoPackRouteIsServed` depends. | The header says only that `mapsFixture` is separate from `metaFixture`. | `TestNoPackRouteIsServed` demands 404 for the pack route, and `http.FileServerFS` at `/` would answer 404 for that path too, so the dependence the plan asserted could not be shown; a warning nobody could verify was not written. |

## What could not be established

- `/api/ruleset` for every role has no test: `TestMetadataRejectsBadMissingAndRevokedTokens`
  drives it with the DM's token only. SPEC-012 states that `handleRuleset`
  consults no role, and VTT-117 names the four routes that are observed. A
  later ticket that touches `metadata_test.go`'s tests can add the row to
  `TestMetadataRulesetGuideServedForEveryRole`'s sibling shape.
- The other six `/api/*` handlers' 401 (VTT-111's narrowing) and
  `handleRuleset`'s non-nil `conditions` (VTT-114's) are observed by no test;
  the review's grep that every handler calls `authed` first is what holds the
  first, and the reading the second.
- VTT-118's maps half rests on a two-element sort: deleting `slices.SortFunc`
  in `handleMaps` leaves the test red in a minority of runs (the report's
  review measured 11 red of 50), while a reversed comparator is red always
  (5 of 5). The mutation gate rewrites operators and does not
  delete statements, so it does not meet this mutant.
- VTT-115's empty-`Guide` arm in `handleRulesetGuide` is unobserved (the
  fixture has no ruleset), and `rules.Load` does not refuse an empty
  `guide.md`, so the arm is reachable. VTT-112's test asserts a non-empty
  `participantId`, not equality with the caller's. VTT-132's tests for an
  unknown id and a refused command send no follow-up command, so "on a
  connection that stays open" is observed for two of the four causes.
- The note-key residue is not reproduced; the debt entry records the test
  that would show it.
- `cmd/vtt/art_test.go` says above
  `TestTheCellPxConstantsAgreeAcrossThePackagesThatCarryThem` that
  `internal/gateway` reads no files at all, which is false for `handleArtFile`
  and `mapByID`; outside this ticket's files, the `cmd/vtt` sweep's.
- Pointers into the cut blocks that remain at `f2d11c2`: `map_test.go`'s,
  gone in `3c1b94a`; `client/src/view/pack-assets.ts`'s, the client
  sweep's; and `internal/mcp/door_tools.go`'s sentence that the reasoning for
  admitting an agent to the shared secret is recorded at `joinLinkRoles`,
  which is now one line pointing at SPEC-009, where the reasoning lives, the
  `internal/mcp` sweep's. `ADR-008` is credited with a rule about the
  filesystem it does not state in six places under `cmd/vtt` and one in
  `server.go`'s `WithCellPx` doc (`git grep ADR-008` at `f2d11c2` over
  `internal/` and `cmd/`); the other three that the four production files
  held at `a330040` are gone with their blocks.

## What was deliberately left out, and where it went

- `server.go`'s twenty-nine blocks over the bound, by symbol above: the
  authorization table, the command path, the maps and art configuration and
  the announcement helpers are the next gateway tickets'.
- What a map is, how `load_map` compiles one and how `mapByID` grows the set
  at request time: `map.go`'s, the maps and art ticket's; SPEC-012 says so
  and points there.
- The dice rule, rolled once and recorded on `AbilityUsed`: `internal/rules`',
  with no record yet; SPEC-012 states the gateway's half.
- MapTool's model, decided once at the handshake and hidden on the client:
  the rule-9 answer in the plan; SPEC-012 names no other project.
- `composeServer`'s doc comment, the remaining `cmd/vtt` comments and the
  test-prose blocks not touched by a re-aim: the `cmd/vtt` and test-prose
  sweeps'.
- No production line under `cmd/vtt/`, `internal/` or `client/src` changed.

## The sort

Twenty-seven candidates accepted as VTT-111 to VTT-137; the refusals above.
