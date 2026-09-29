# The seat and the perch have a record — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-29-the-seat-and-the-perch-have-a-record-design.md`
**Verified:** 2026-09-29, by `verify-ticket`, an agent that did not write the
ticket, against `74c547f` on `chore/spec-015-seat-and-perch` (`main` at the
time; the ticket untracked). Verdict: **Passes with gaps.** The gaps are listed
at the end and travel with this plan. This plan does not edit the ticket; where
an item is thin or a sentence of it is loose, the plan decides around it and
says so.

**Goal, in the ticket's words:** `docs/specifications/015-the-seat-and-the-perch.md`
exists and states which seats are projected, how a projected seat is fed and
what it drops, what a failed fold does, the viewpoint a connection opens with,
who may perch on whom, how a perch travels and is applied, and `canSee`; the
false "only writer" sentence is gone from `seat.go`; every rule the sort
accepts has a row cited by a gateway test; `seat.go` and `viewpoint.go` carry
only warnings, pointers and `MayPerch`'s doc sentence, with no banned line and
no block over the bound; SPEC-013's sentences that say the perch's rule has
no record point at SPEC-015; and no code line changes.

**MapTool (CLAUDE.md rule 9), the answer this plan records.** MapTool decides
what a client sees on the client, from a whole campaign every client holds.
On connect `MapToolServer` sends the joiner one `SetCampaignMsg` carrying
`campaign.toDto()`, every zone and token, GM layer included, and relays every
later change to every client; `server/ServerMessageHandler.java` never asks
`isOwner`, `playerOwns` or `isGM` (grep prints nothing). Vision is computed per
client in `client/ui/zone/` (`ZoneView.getVisibility`, per token with sight
through `FogUtil.calculateVisibility`) and applied at render time
(`ZoneRenderer`, `FogRenderer`). Whose eyes a view uses is
`ZoneViewModel.makePlayerView`: the selected tokens with sight that the player
owns (`AppUtil.playerOwns`); failing those, the player's own tokens with sight
when the server policy says `isUseIndividualViews`, or every PC token with
sight when vision is shared; failing those, a view with no token list, which
`ZoneView.getTokensForView` fills from the zone's tokens with sight.
`Player.Role` has two values, `PLAYER` and `GM`; there is no spectator, and the
nearest thing to a perch is the GM's local "Show As Player" toggle. Remembered
fog is kept per token (`Zone.exposedAreaMeta`, keyed by
`Token.exposedAreaGUID`), a view's exposed area is the union of its tokens'
histories, and the moving client computes the geometry the server stores.
There is no event log and no per-seat catch-up: a joiner gets the snapshot. So:
borrowed is the shape of a view as a role plus a set of eye tokens, and shared
party vision as the analogue of a spectator riding a party member. Refused,
each with its reason: whole-campaign distribution with a client-side clip is
the distribution model rule 9 names, and a projected seat is its opposite (the
server sends each seat only what its projection admits); a view that falls
back to some tokens when none is chosen is refused because a server-chosen
shoulder is one nobody asked for (`viewerFor` opens on nobody); eyes picked by
client selection under a lax `playerOwns` are refused because `MayPerch` binds
a perch, on the server, to an actor whose kind is party member; memory merged
by selection is refused because a seat's memory here is its projector's, fed
from the log. The notes behind this paragraph were written by another agent at
`f4b7fef6c` and spot-checked by the verifier with grep in `~/dev/RPTool/maptool`
(`makePlayerView`'s fallback chain, `MapToolServer`'s `SetCampaignMsg`,
`Player.Role`, `PlayerView`'s token list, `Zone.exposedAreaMeta`, and the empty
grep over `ServerMessageHandler`). Where this is written is D3.

## Verification, check by check

1. **Every path resolves — by command.** `internal/gateway/seat.go` declares
   `viewerFor`, `projected`, `seat` (fields `pr`, `resume`, `received`,
   `world`), `perchBox` (fields `mu`, `shoulder`, `full`, `wake`),
   `newPerchBox`, `set`, `take`, `newSeat`, `subscribeFrom`, `receive`,
   `pastResume`, `perch`, `canSee` and `catchUp`; `viewpoint.go` declares
   `MayPerch`; `project.go` declares `Viewer`, `Projector`, `NewProjector`,
   `Project`, `perchSequence`, `reperch`, `look`, `eyes` and `canSeeSquare`;
   `internal/campaign` declares `FoldPrefix` (`foldprefix.go`) and `Append`,
   `AppendBatch` and `Open` (`campaign.go`); `engine.IsPartyMember` is in
   `internal/engine/actorkind.go`; `server.go` declares `serve`,
   `answerCommand`, `handleSetViewpoint` and `deliver`. The four test files the
   ticket names exist. `docs/superpowers/specs/2026-08-18-visibility-design.md`
   has §3.1, §3.1.1, §4.1, §4.4, §5.1 and §8, and
   `docs/reports/2026-08-18-visibility.md` exists. `docs/specifications/015*`
   does not exist.
2. **"Done" is an observation — by command where a command exists.** Item 1:
   the file does not exist. Item 2: `grep -c 'campaign.Append is the only
   writer' internal/gateway/seat.go` prints 1. Item 3: `python3
   tools/check-requirements-chain.py .` prints `175 rows, 193 test files, 8
   specifications; every citation resolves and every row's evidence holds`;
   `viewpoint_test.go`, `viewpoint_internal_test.go` and `project_test.go`
   carry no citation line, and `server_visibility_test.go` carries four
   (`VTT-155 VTT-158`, `VTT-156`, `VTT-155`, `VTT-086`), none on a perch test.
   Item 4: `--report` prints `banned  17  blocks>6  11` for `seat.go` and
   `banned   8  blocks>6   1` for `viewpoint.go`. Item 5 is an invariant (D11
   holds it at every task). Item 6: the sentences exist, three of them (D14).
   Item 7 is the gate. The half of items 1 and 4 that says every surviving
   block is a warning, a pointer or a doc sentence, and that every sentence of
   SPEC-015 is true of its symbol, is Phase 4b's reading, as the ticket says.
3. **Each rule is breakable — a reading, then probes.** D5 names, per
   candidate, the test that goes red and the one edit that reds it. Every edit
   named there was run by the verifier in a scratch clone of `74c547f`: it
   compiles (`go vet ./internal/gateway/` clean) and reds the named test; the
   inverse edit restored the file and `git status` was clean each time. Every
   claim in this plan that a thing is observed by no test was probed across
   `go test -count=1 ./internal/gateway/... ./cmd/vtt/...`, the whole set of
   packages that import the gateway (`grep -rln` for its import path prints
   `cmd/vtt` and the gateway's own tests). Five of the ticket's twelve
   candidates join two rules and are split (A and B, I and X, L and M, N and
   O, P and Q), and one more is split because its halves are reddened by
   different edits (C and D); three are loose as worded (E's "watches
   nobody", F's "whoever controls it", K's "the world the seat has been
   shown"); four rows are added from the ticket's own problem paragraph and
   Done item 1 (R, S, T, U).
4. **Scope matches the claim — by command, then a reading.** No code line
   changes, so callers do not widen the work: `projected` is called by
   `newSeat` alone; `newSeat`, `subscribeFrom`, `catchUp`, `receive` and
   `perch` by `serve` alone (`grep -n '\bsub\.' internal/gateway/server.go`);
   `perchBox.set` by `handleSetViewpoint` alone; `canSee` by `handleCommand`
   alone; `MayPerch` by `Authorize` alone (every `grep -rn '<name>('` over
   non-test files under `internal/` and `cmd/`). What widens the work is the
   pointers INTO the blocks being cut, from files the ticket does not list:
   `keystone_test.go` quotes `seat.go`'s comments three times, `project.go`'s
   `reperch` doc says the perch measurement is "written up at perchBox", and
   `authz_test.go` holds `TestAuthorizeSpectatorMayNotPerchOnAnNpc`, which the
   authorization change left to this ticket by name. The list is under
   Measurements; D13 and Q6 to Q8 decide each.
5. **No recorded decision is contradicted — a reading.** SPEC-007 (what the
   connection opens with; `set_viewpoint` appends nothing), SPEC-009
   (re-resolution; the perch's frames are not re-checked), SPEC-011 (the
   subscription start, the catch-up head, the pump) and SPEC-013 (the command
   path, `MayPerch` as an every-role check, `perchBox.set`, a perch ending with
   its connection) were read against the ticket and the code, and every
   sentence in them about seats, perches, viewpoints or catch-up is true of the
   code at `74c547f`. SPEC-015 overturns none of them; it points at each. Two
   sentences in the tree are false and must not be carried into SPEC-015:
   `receive`'s "Unreachable while campaign.Append is the only writer" (the
   ticket's), and `projected`'s "making this return true for them changes no
   byte on any wire", which a probe disproves (with the change, a DM resumed
   at the log's head is told `CatchUpHead` 0 where it is told the head today;
   see Measurements). `docs/adr/` holds no sentence about perching; ADR-003 names
   spectator catch-up in passing and is frozen evidence.
6. **The records the work moves are named — by command, then a reading.** The
   section reads `New: the seat and the perch ...` with
   `docs/specifications/013-authorization.md` beside it; the path resolves. The
   reading: the work changes no behaviour, so what goes stale is the three
   SPEC-013 sentences that send the perch's rule to `viewpoint.go` and what a
   seat can see to `seat.go` (D14), not one sentence as item 6's wording
   suggests. SPEC-011's Status names `seat.go`'s `newSeat`, `subscribeFrom`
   and `catchUp`, which stays true; SPEC-011, SPEC-007 and SPEC-009 are not
   edited. `docs/verification-debt.md` holds nothing about seats or perches
   beyond the oracle-corpus entry, which this work does not touch.

## Measurements this plan stands on

All at `74c547f`, by command, run by the verifier.
`python3 tools/check-comments.py --report`, the checker's own `measure`
(`cite = cc.cite_pattern(cc.register_tag()); measure(lines, False, cite)`),
and `tools/comment-ceilings.txt`:

| File | Comment / non-blank | Share | Ceiling | Banned | Blocks > 6 | Cites |
|---|---|---|---|---|---|---|
| `internal/gateway/seat.go` | 311 / 430 | 72.3 | 72.4 | 17 | 11 | 0 |
| `internal/gateway/viewpoint.go` | 47 / 68 | 69.1 | 69.2 | 8 | 1 | 0 |
| `internal/gateway/viewpoint_test.go` | — | 45.1 | 45.2 | 6 | 3 | 0 |
| `internal/gateway/viewpoint_internal_test.go` | — | 40.3 | 40.4 | 11 | 7 | 0 |
| `internal/gateway/server_visibility_test.go` | — | 29.4 | 29.4 | 21 | 19 | 4 |
| `internal/gateway/project_test.go` | — | 33.5 | 33.6 | 75 | 44 | 0 |
| `internal/gateway/authz_test.go` | — | 36.9 | 36.9 | 43 | 19 | 33 |
| `internal/gateway/keystone_test.go` | — | 46.6 | 46.7 | 34 | 19 | 0 |

The ticket's figures for `seat.go` and `viewpoint.go` are these. Token counts,
D11's instrument (comments dropped): `seat.go` 824, `viewpoint.go` 110,
`viewpoint_test.go` 630, `viewpoint_internal_test.go` 2192,
`server_visibility_test.go` 6280, `project_test.go` 16828, `authz_test.go`
6622, `keystone_test.go` 4593, `project.go` 3502, `server.go` 4596.

**`seat.go`, by block**, each by the symbol it sits on or in, with the line
count `measure` gives and its banned lines: `viewerFor`'s doc (16, 2, over);
`projected`'s doc (19, 1, over); the `seat` type's doc (29, 1, over); the `pr`
field (4, 0); `resume` (3, 0); `received` (20, 2, over); `world` (7, 0, over);
the `perchBox` type's doc (63, 3, over); the `wake` field (3, 0); `set`'s doc
(3, 0); `take`'s (2, 0); `newSeat`'s (3, 0); `subscribeFrom`'s (4, 0);
`receive`'s doc (5, 0); inside `receive`, the block above `campaign.FoldPrefix`
(4, 0), the fold-failure block ending in the `#nosec` line (28, 2, over) and
the block above `s.world = world` (2, 0); `pastResume`'s doc (28, 1, over) and
the block inside its loop (2, 0); `perch`'s doc (20, 2, over) and its
unprojected arm (5, 0); `canSee`'s doc (4, 0); `catchUp`'s doc (24, 3, over);
inside `catchUp`, the closed-channel arm (4, 0), the block above the head
assignment (7, 0, over) and the block above the log-head test (2, 0). 26
blocks, 311 lines, 17 banned, 11 over. One trailing comment,
`default: // already signalled; ...` in `set`, is a code line to `measure` and
is swept by the reading (D7).

**`viewpoint.go`, by block:** `MayPerch`'s doc (38, 8, over); inside it, the
empty-id arm (4, 0) and the refusal arm (5, 0). No package doc.

**Every sentence of the ticket's first paragraph, against its symbol.** True as
written: `projected`'s two answers; an unprojected seat's `pr` is nil and
`receive` returns the event it was handed; a projected seat subscribes from 0
(`subscribeFrom`); `receive` folds `s.received` with `campaign.FoldPrefix` and
hands `Project` that state; the fold-failure arm returns nil; `pastResume`
keeps output strictly above `resume`; `viewerFor` leaves `Viewpoint` empty;
`MayPerch`'s three answers and its one actor-refusal string; `perchBox`'s one
slot, latest wins, `set` never blocking; `perch` calls `reperch` with
`s.world` and skips `pastResume`; `canSee` builds a fresh `Projector` and asks
`look` once; `catchUp`'s head. Three compressions: "SPEC-011 carries where a
seat's subscription starts ... and nothing carries the rest" — SPEC-011 also
carries that frames at or below `after` are dropped and that the pump applies
a perch, SPEC-013 carries that `set` does not block and that a shoulder
replaced before the pump takes it is never applied, SPEC-013's Consequences
that a perch does not survive a reconnect, and SPEC-009 that the perch's frames
are not re-checked; "the world the seat last folded" is the state after the
last event the seat RECEIVED, withheld ones included, not the world it has
been shown (row K's wording); and "`catchUp` projects the backlog" is true of
a projected seat only — for an unprojected one it returns the store's head and
drains nothing (SPEC-011).

**The fold-failure arm, established.** Within one process the events table is
written by `store.Append` and `store.AppendBatch` alone (`grep -rn 'INSERT INTO
events'` over non-test files prints `internal/store/store.go` twice), which
`campaign.Append` and `campaign.AppendBatch` alone call (`grep -rn
'c.log.Append'`), and the gateway's five append sites all go through those two.
Each folds its envelopes against a snapshot of the live state, under the
campaign's mutex, before persisting; `Open` folds the whole log and refuses one
that does not fold; a live apply that fails after persisting poisons the
campaign and notifies nobody, and a poisoned campaign refuses `Subscribe`. So
every prefix a projected seat receives folds, and `FoldPrefix` fails only on a
log another writer shaped: nothing locks a campaign directory to one process
(`grep -rn -i 'flock\|lockfile\|O_EXCL'` over `internal/store`,
`internal/campaign` and `cmd/vtt` prints nothing). Probed with a scratch test,
never committed: two `campaign.Open` handles on one temporary directory each
append a `SceneCreated` for scene `s`, and both are accepted; a projected seat
fed the log through the first handle's `Subscribe(0, ...)` logs `gateway:
projection fold failed; withholding event sequence=2 error="campaign: corrupt
log at seq 2: engine: scene \"s\" already exists"`, and a third `Open` of the
directory refuses the log with the same error, so the campaign no longer boots.
What observes the arm: nothing — with the arm forwarding the event it could not
judge, or with a failed fold clearing `world`, the closure stays green (the
second run's only reds are the MCP deadline tests, gap 11). The false
sentence's premise is that `campaign.Append` is the only writer; the true
premise is that both writers fold first, and the conclusion ("unreachable")
holds only within one process.

**What `projected` decides for the DM and the agent, probed.** `projected`'s
doc says making it answer true for them "changes no byte on any wire". With
`case identity.RoleDM, identity.RoleAgent:` made `case identity.RoleAgent:`,
the whole closure stays green; but a scratch test that dials the DM at the
log's head and reads `CatchUpHead` prints `a DM resumed at 11 is told head 0`,
where the base prints `head 11`: the projected seat's `catchUp` answers 0 when
it sends nothing, the unprojected one answers the cursor. What it decides is
where their subscription starts, the head they are told and whether each event
costs a fold; which events reach them is `Project`'s identity arm as well
(`project.go`). SPEC-015 says so and does not repeat the comment.

**The `#nosec` line, established.** `// #nosec G706 -- int64 sequence,
structured attribute; see above.` sits directly above the `slog.Error` call in
`receive`'s fold-failure arm, as the last line of the 28-line block. gosec
binds a `#nosec` comment through the comment group that sits directly above a
statement: probed with the pinned `golangci-lint` 2.11.4 (the version
`.github/workflows/ci.yml` installs) on `internal/harness/soak.go`'s `#nosec
G115`, a warning line above it or below it in the same group still suppresses
the finding, and a blank line between the group and the statement detaches it
(`G115: integer overflow conversion int -> int32`). At `seat.go` itself the
directive is inert today: with it deleted, `golangci-lint run --enable-only
gosec ./internal/gateway/` on a fresh cache prints `0 issues`, although gosec
2.24.8 carries G706 ("Log injection via taint analysis"). D4 keeps it anyway,
shortened, as the last line of the group directly above the call, and Q5 asks.

**Pointers into the blocks this sweep cuts, from outside them.** Searched over
`internal/`, `cmd/` and `client/src` for every comment naming `seat.go`,
`viewpoint.go`, a symbol of either, or "its own comment", "the field's
comment", "type's comment", "written up at":

1. `viewpoint_internal_test.go` (listed by the ticket), three blocks, all over
   the bound: `TestARapidHopIsCoalescedToTheShoulderItEndedOn`'s doc (19 lines:
   "see perchBox for the measurement and for what coalescing costs"),
   `TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt`'s doc (50 lines:
   "measured, and written up at perchBox", "perchBox has to become the FIFO its
   comment explains away") and `threeRoomLog`'s doc (10 lines: "the bench
   perchBox's FRAME COUNTS were taken on"). This answers the ticket's third
   open question: these three are the blocks a re-aimed pointer forces to the
   bound (D13). The measurements they point at are already in
   `docs/reports/2026-08-18-visibility.md` (its `perchBox` paragraphs).
2. `keystone_test.go` (not listed): the doc above
   `TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees` (103 lines,
   over: "internal/gateway/seat.go's own doc comment says so"), a 2-line block
   in the seat list of its oracle ("seat.go: "a connection opens perched on
   nobody""), and `projectedSeat`'s doc (8 lines, over: "(seat.go: the perch
   "is connection state, like the catch-up point")"). Q7.
3. `project.go` (not listed, the projection's): `reperch`'s doc (29 lines,
   over): "measured, and written up at perchBox with what the difference is".
   Left, named (D18).
4. `project_property_test.go` (not listed): a 3-line block, "the ordering
   internal/gateway's seat.go uses, and which its own comment calls what
   Project is specified to read" — it points at the block above
   `campaign.FoldPrefix` in `receive`; D7 keeps that block's sense so the
   pointer stays true, and the file is not touched.

Pointers that name a symbol or a file whose code survives, not re-aimed:
`server.go`'s blocks above `newSeat(p, after)` ("(seat.go)"), above
`sub.catchUp` ("(seat.catchUp)") and above `newPerchBox()` ("(handleSetViewpoint,
perchBox)"); `project.go`'s `Viewer` doc and `eyes` body ("viewpoint.go"),
`perchSequence`'s ("seat.catchUp takes its head from projected output") and
`transitions`' body ("seat.received only grows", "receive's FAIL CLOSED", which
names what the code still does); `project_test.go` ("seat.received only
grows"); `internal/campaign/foldprefix.go` and `foldprefix_test.go` ("seat.receive
is exactly that caller", "seat.go's receive FAILS CLOSED"); `viewpoint_test.go`
("MayPerch's subject"), `authz_test.go`, `server_visibility_test.go`'s
`TestASpectatorMayNotPerchOnTheGoblinArcher` ("MayPerch is unit-tested in
viewpoint_test.go"), `metadata_test.go`, `internal/engine/apply.go`; and under
`client/src`, `app.ts`, `commands.ts`, `view/dm.ts` and `view/spectator.ts`,
which quote `MayPerch`'s refusal string or name the file.

**Adjudication keys.** `tools/mutation-equivalents.txt` holds no key naming
`seat.go` or `viewpoint.go`, and no entry's text names either file or a symbol
of either by coordinate (`grep -n -i
'seat\|pastResume\|catchUp\|MayPerch\|perchBox\|viewerFor'` over both files
finds two entries that use a word of it in prose: `abs()`'s, which narrates
`authz.go` moving when `MayPerch`'s call joined `Authorize`, and a `wire.ts`
entry on `seenSeq` that says "a seat"; neither names either file). `python3
tools/check_mutation_test.py -q` (107 tests) and `python3
tools/check_ts_mutation_test.py -q` (50 tests) print `OK` at the base. So no
key moves; the self-tests are still run after every edit and before the
commit, because this is the claim a moved line would falsify, and if Q7 adds
`keystone_test.go` the same holds (test files carry no keys).

**Records and gates.** `docs/specifications/` holds 007 to 014; 015 is free.
`docs/requirements.md` has 175 rows, the last `VTT-175`. `python3
tools/check-comments.py main` ends `239 files, 0 added comment lines, 239
ledger rows; clean`, after the standing notice on `cmd/vtt/library_test.go`.
`python3 tools/check-doc-owner.py .` ends `79 files, every doc comment sits on
its own function`. `gofmt -l internal/gateway/` prints only
`scenario_test.go`. `requirement-id` is on the path, at
`~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`. Load was
between 2.8 and 4.3 during the probes.

**What no test observes, by probe over the whole closure.** Each edit below
compiles; with it, `go test -count=1 ./internal/gateway/... ./cmd/vtt/...`
gives:

- `projected`'s last line made `return r == identity.RolePlayer || r ==
  identity.RoleSpectator` (a role this build does not know unprojected):
  green.
- `case identity.RoleDM, identity.RoleAgent:` made `case identity.RoleAgent:`
  (the DM projected): green.
- `MayPerch`'s actor test made `!ok || !engine.IsPartyMember(a) ||
  len(a.GetControllerIds()) == 0` (a party member nobody controls refused):
  the gateway green; `cmd/vtt` red only in
  `TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessIgnoresStdio`, which no
  perch reaches (gap 11).
- `MayPerch`'s role arm made `if false {`: red in
  `TestOnlyASpectatorRidesAShoulder` alone.
- the fold-failure arm's `return nil` made `return []*vttv1.Envelope{env}`:
  green.
- `s.world = world` moved above the error check (a failed fold clears
  `world`): the gateway green; `cmd/vtt` red only in four `TestQAMCPConnect*`
  deadline tests (gap 11).
- the pump's `sub.perch(actorID)` made `sub.pr.reperch(actorID,
  s.campaign.State())` (a perch judged against the head): green.
- `handleSetViewpoint`'s `perches.set` moved above `authorize` (a refused
  perch reaches the pump): green (row U).
- `FoldPrefix(s.received)` made `FoldPrefix(s.received[:len(s.received)-1])`
  (each event judged against the state before it): red in five tests, and
  green in `TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive`, whose doc
  says it pins this (row R).

**A dry run of D4, D7, D8 and D13, in the scratch clone and never in the
tree.** With the draft texts in `seat.go`, `viewpoint.go`,
`viewpoint_internal_test.go` and `keystone_test.go`: D11 prints `same` at
824, 110, 2192 and 4593; `gofmt -l internal/gateway/` prints only
`scenario_test.go`; `go vet` is clean; `go test -count=1
./internal/gateway/...` is green; `golangci-lint run ./internal/gateway/`
prints `0 issues`; `go doc ./internal/gateway MayPerch` prints the one
sentence; `python3 tools/check-comments.py main` refuses exactly three files,
each for having fallen more than the band under its ceiling (`seat.go` at
27.88, `viewpoint.go` at 19.23, `viewpoint_internal_test.go` at 28.20), and
nothing else; `python3 tools/check-new-prose.py main` prints `65 added
line(s) across 4 file(s), all clean`; `python3 tools/check-doc-owner.py .`
still ends `79 files, every doc comment sits on its own function`; then
`--write-ledger` changes exactly four rows (`keystone_test.go` 46.7 to 46.4,
`seat.go` 72.4 to 27.9, `viewpoint.go` 69.2 to 19.3,
`viewpoint_internal_test.go` 40.4 to 28.2) and `check:comments` ends
`clean`. B4 and B7 (D15) were run on that tree and gave the lines D15 quotes.
The clone was restored from `74c547f` afterwards and `git status` was clean.

**`check:doc-owner`'s first-word rule, re-derived at `74c547f`.** It refuses
a doc block above a function whose first word names ANY function declared
outside a test file (576 names over 79 files, lowercase included). The
single-word capitalised names are: `Accepted`, `Actors`, `Append`, `Apply`,
`Arm`, `Authorize`, `Blocked`, `Blockers`, `Clear`, `Clip`, `Close`, `Compile`,
`Dial`, `Error`, `Eval`, `Events`, `Fold`, `Handler`, `List`, `Load`, `Lookup`,
`New`, `Notify`, `Open`, `Parse`, `Project`, `Refs`, `Resolve`, `Revoke`,
`Roll`, `Run`, `Scenes`, `Scopes`, `Snapshot`, `State`, `Step`, `String`,
`Subscribe`, `Tokens`, `Unwrap`, `Validate`, `Verify` (`Accept` and `Write`,
on the authorization plan's list, are no longer declared). The lowercase ones
this sweep could trip are the file's own and its neighbours': `set`, `take`,
`perch`, `receive`, `look`, `eyes`, `reperch`, `deliver`, `serve`. So no
warning above `receive`, `perch` or `catchUp` may open with `Project`, `Fold`,
`Apply`, `Subscribe`, `State`, `Events` or `Append`. `Leave`, `Answer`,
`Never`, `Report`, `Return`, `Add`, `Call`, `Build`, `Keep`, `Touch`, `Drop`,
`Hand`, `Withhold`, `Take`, `Stop`, `Accept` name no function at `74c547f`. A
comment inside a body, on a struct field or above a type is not read by that
gate. D7's draft texts were run through its `described` on a scratch copy:
every function's block passes.

## Constraints that bind every task

- `CLAUDE.md` rule 10 and SPEC-010: a comment is an imperative warning, a
  pointer, or the one-line doc sentence of an exported symbol. VTT-051 is held
  by the Phase 4b reading; VTT-050, VTT-052, VTT-053 and VTT-055 by
  `check:comments`; VTT-059 says a citation line moves no share.
- `CLAUDE.md` rule 2: no gate is weakened. The ledger only goes down, only
  through `--write-ledger`. The `#nosec` directive stays (D4).
- `CLAUDE.md` rule 8, narrowed by rule 10 for code: a comment points at a
  specification by number, a requirement by id, a test or symbol by name, or a
  report by its `docs/` path; never a line number, a date, a commit hash, a
  plan, a task, `§`, `#NNN`, an ADR, `CLAUDE.md` or a `.superpowers/` path.
  This plan and the report name blocks by the symbol they sit on.
- `CLAUDE.md` rule 3: the contract is not touched; `SetViewpoint`,
  `CatchUpHead` and the envelope's `sequence` are SPEC-007's.
- `CLAUDE.md` rule 4: one fold. SPEC-015 says a seat's state is
  `campaign.FoldPrefix` over what it received and never describes a fold of
  the gateway's own.
- `CLAUDE.md` rule 5: SPEC-015 names no rules concept; a party member is an
  actor kind, `engine.IsPartyMember`'s.
- `CLAUDE.md` rule 9: the answer above is recorded before any task runs.
- SPEC-008: ids come from `requirement-id`, after sign-off, never by hand.
- The `specification` skill's form and `catches.md`: five headings, present
  tense, no `Why`, no `Rejected`, no measurement, no line number, no past-tense
  account that is not about the code, no path outside the project, one
  decision per file (item 14, the one to watch here: the ticket's file name has
  "and" in it, Q1), no claim taken from a comment without reading the code
  under it (item 8: `projected`'s "no byte" and `receive`'s "unreachable" are
  two comments already found false), every "only/every/never/none" with its
  search named (item 10), the old text not left beside the new (item 13, for
  the SPEC-013 edits).
- The `requirements` skill: one thing, what not how, breakable, measurable,
  named by a check or knowingly OPEN; refuse more than you accept, and report
  every refusal in one line.
- SPEC-007, SPEC-009 and SPEC-011 are not edited; SPEC-013 only at the
  sentences D14 names.
- The ticket: no code line changes (D11 is the check); `seat.go`'s 26 blocks
  and one trailing comment, `viewpoint.go`'s three, citation lines in the test
  files D6 names, `viewpoint_internal_test.go`'s three blocks (D13), SPEC-013's
  sentences, the register, the ledger. `keystone_test.go` and `authz_test.go`
  only if Q7 and Q6 say yes.
- `internal/campaign`, `internal/engine`, `internal/store`, `cmd/vtt`,
  `client/src`, `contract/`, `project.go`, `server.go`, `authz.go`, every
  other file under `internal/gateway` and the visibility ticket and report are
  not touched. `docs/reports/` gains only this ticket's report.

## Decisions this plan makes

**D1. SPEC-015's sections, each lifted from a named source and held by named
symbols.** Forced by ticket item 1 and the `specification` skill's step 3.
File `docs/specifications/015-the-seat-and-the-perch.md` (the ticket's name;
Q1). Title: `SPEC-015: What a connection is sent is decided by its seat, whose
eyes only a spectator may move`. Under "How it works", in this order, one
bold-led paragraph each:

| Section | Source (where it lives today) | Symbols that hold it | Rows (D5) |
|---|---|---|---|
| Which seats are projected | `projected`'s doc; `receive`'s doc; the `pr` field; visibility §3.1 | `projected` answers false for `identity.RoleDM` and `identity.RoleAgent` and true for every other role, including one this build does not know, which `identity.Verify` never admits (SPEC-009); `newSeat` gives an unprojected seat no `Projector` and a projected one `NewProjector(viewerFor(p))` with `resume` set to the connection's cursor; an unprojected seat's `receive` returns the event it was handed, the pointer the pump holds, and folds nothing; `Project` also answers the DM and the agent with the event itself (`project.go`), so what `projected` decides for them is where their subscription starts and the head they are told (SPEC-011, pointed at) and whether each event costs a fold, not which events reach them | A, B |
| How a projected seat is fed, and against which state | the `seat` type's doc; the `received`, `resume` and `world` field docs; the block above `campaign.FoldPrefix`; visibility §4.1 | `subscribeFrom` answers 0 (SPEC-011, pointed at); `receive` appends each envelope to `received`, folds the whole slice with `campaign.FoldPrefix` (campaign's own fold, so the gateway keeps none: CLAUDE.md rule 4), hands `Project` the event and that state — the state after the event, from what this seat has received, never `campaign.State()`, which during catch-up is ahead of the event being judged — and keeps the state as `world`; every event folds the whole prefix received so far; that a projector must be fed from the start of the log is the `Projector`'s (`project.go`, pointed at) | R |
| What a projected seat drops | `pastResume`'s doc and loop block | `pastResume` keeps the frames whose sequence is strictly greater than `resume`, as `store.Subscribe`'s `seq > afterSeq`; input at or below `resume` is still folded; so a seat resumed at a cursor is sent what its projection produces after the cursor, departures during its absence included, and nothing at or below it | C, D |
| A fold that fails withholds the event | the fold-failure block in `receive` | on a `FoldPrefix` error `receive` logs the event's sequence and the error with `slog.Error`, never the event id, returns no frame for the event, and leaves `world` at the last state that folded; within one process every prefix a seat receives folds, for the reason and with the searches under Measurements, so the arm is reached when another process has written the same campaign's log, which nothing prevents | S |
| The viewpoint a connection opens with | `viewerFor`'s doc; visibility §3.1.1 and §4.4 | `viewerFor` gives the participant's id and role and an empty `Viewpoint`; `eyes` reads an empty viewpoint as no eyes and ignores `Viewpoint` for a player (`project.go`, pointed at); so a spectator's connection is shown no board until a perch names a shoulder, and no shoulder is chosen for it; the viewpoint lives in the connection's seat, which `serve` makes per connection (SPEC-013 states that a perch ends with its connection) | E |
| Who may perch, on whom, and the one refusal | `MayPerch`'s doc and its two body blocks; visibility §3.1.1, §5.1 | `MayPerch` refuses every role but spectator, naming the role; accepts an empty actor id; otherwise accepts an actor `st.Actors` holds whose kind `engine.IsPartyMember` accepts, reading no controller; answers an absent actor and a non-party one with one string that names only the id sent; every refusal wraps `ErrUnauthorized` (SPEC-013); `Authorize` runs it for every `set_viewpoint` after the role table, which leaves only a spectator (SPEC-013; VTT-139, VTT-140, VTT-141), so the role arm is reached through `Authorize` by a spectator alone; `eyes` refuses a non-party viewpoint a second time (`project.go`) | F, H, I (G refused) |
| How a perch travels to the pump | the `perchBox` type's doc; `set`'s, `take`'s and the `wake` field's docs; SPEC-013's `set_viewpoint` paragraph (pointed at) | `handleSetViewpoint` calls `perchBox.set` after `authorize` (SPEC-013); `set` stores the shoulder and marks the slot full under `mu`, then signals `wake`, capacity one, dropping the signal when one is already waiting, and never blocks; `take` returns the slot's shoulder and whether it was full, and empties it; so the pump is handed the last shoulder set before its `take`, a shoulder replaced before that is never applied, a second wake-up with nothing new applies nothing, and the empty id travels as a value; `mu` guards the slot and orders nothing on the wire | N, Q, U |
| How a perch is applied | `perch`'s doc and arm; `pastResume`'s perch paragraph; `reperch`'s and `perchSequence`'s docs (pointed at) | `serve`'s pump takes the shoulder when `wake` fires and calls `perch` at once, so the new view is sent without waiting for an event, and delivers the frames as it delivers an event's (SPEC-011), without re-checking the credential (SPEC-009); `perch` returns nothing for an unprojected seat, which `Authorize` never lets perch (VTT-140); otherwise `Projector.reperch(actorID, world)`: the new eyes judged against the state after the last event this seat received, never the campaign's head; nothing when the seat has folded nothing; every frame carries `perchSequence`, 0; its output never passes `pastResume`, so no cursor filters it; `handleSetViewpoint` and `perch` append nothing (the searches in D10; SPEC-007 names `set_viewpoint` among the commands that append nothing); what `reperch` sends, and that a shoulder named again is served in full from the projector's memory, are `project.go`'s | J, K, L, M, T |
| `canSee` | `canSee`'s doc | `canSee` builds a fresh `Projector` for the viewer, asks `look` once and answers `canSeeSquare` for the one square, so no seat's projector is touched; `handleCommand`'s move gate is its one production caller (SPEC-013) | none (VTT-155, VTT-156 hold the gate) |
| A projected seat's catch-up | `catchUp`'s doc and its three body blocks | the head is SPEC-011's and VTT-086's, pointed at; for a projected seat `catchUp` runs each backlog envelope through `receive`, takes the head from the last frame an event produced (every frame of one event carries that event's sequence), stops at the first envelope at or past the log's head, and answers what it has when `events` closes or `ctx` ends; perch frames never reach it, since `serve` makes `perches` after it returns | none |
| What this record does not decide | the ticket's "What could not be established" | where a subscription starts, the catch-up head, the pump and the one writer (SPEC-011); the command path to `MayPerch` and `perchBox.set`, and the role table (SPEC-013); that `set_viewpoint` appends nothing and what `CatchUpHead` means to a client (SPEC-007); re-resolution, and that perch frames are not re-checked (SPEC-009); what a projection computes — `Project`, `reperch`, `eyes`, `look`, `canSeeSquare`, `perchSequence`, the `Viewer` type — which is `project.go`'s and has no record yet; what a party member is (`engine.IsPartyMember`) | none |

Each sentence names the symbol that holds it, is checked against the code
before it is written and re-read by Phase 4b (D16). Status: `Accepted.
Implemented by internal/gateway/seat.go (viewerFor, projected, seat,
perchBox, newPerchBox, set, take, newSeat, subscribeFrom, receive,
pastResume, perch, canSee, catchUp) and internal/gateway/viewpoint.go
(MayPerch), over internal/campaign's FoldPrefix and the Projector in
internal/gateway/project.go; called from internal/gateway/server.go (serve,
handleSetViewpoint, handleCommand) and authz.go (Authorize); pinned by
internal/gateway/viewpoint_test.go, viewpoint_internal_test.go and
server_visibility_test.go` (and `authz_test.go` if Q6 says yes). "Principles
served" says what SPEC-013 and SPEC-014 say: no blueprint; the principle
missing from the record rather than absent from the system (here: what a
person is sent is decided on the server, per connection, from the log that
connection has been fed, and a watcher sees through one party member's eyes at
a time and never through the DM's). "Consequences" holds what a client author
and whoever changes the code are bound by: a spectator's connection shows no
board until it perches; a perch's frames carry sequence 0 and move no resume
cursor; of a burst of perches only the last may be applied, and naming a
shoulder again restores it; a perch refusal says nothing about whether the
actor exists; the DM's and the agent's streams are the log; every connection
of a player or a spectator replays the log from its start on the server, one
fold of the prefix per event; a seat is touched from the pump alone once
catch-up is drained, and never from a second goroutine through a lock; the
fold-failure arm is reached only when another process writes the campaign's
log. "Requirements" lists the ids the sort dispenses (D6) and nothing else.

**D2. What SPEC-015 points at and does not restate.** Forced by `catches.md`
item 13 and the ticket's item 1. SPEC-011 owns `subscribeFrom`'s answer as a
subscription start, the catch-up head and `CatchUpHead`'s queueing, the pump,
`deliver` and the one writer; SPEC-015 says "SPEC-011" at each. SPEC-013 owns
`answerCommand`'s routing, `handleSetViewpoint`'s order (authorize, then
`set`), the role table's `set_viewpoint` cells, `MayPerch` running for every
role inside `Authorize`, the ok result meaning the shoulder is recorded, and a
perch ending with its connection. SPEC-007 owns `set_viewpoint` appending
nothing and what `CatchUpHead` means to a client. SPEC-009 owns
`credentialGone` and that perch frames are not re-checked. `project.go` owns
everything a `Projector` computes, and SPEC-015 names the functions and says
the file has no record yet. `internal/engine` owns what a party member is.

**D3. Provenance stays out of the record.** Forced by `catches.md` items 3 and
12. The MapTool answer above is the rule-9 answer and lives in this plan and
the report. SPEC-015 states the empty opening viewpoint, the kind rule and the
server-side refusal as facts about this system and names no other project, no
ticket section, no ruling date and no quotation. Q2.

**D4. The false sentence, the fold-failure arm and the directive.** Forced by
ticket item 2 and the brief's constraint on the `#nosec` line. The 28-line
block in `receive`'s fold-failure arm becomes three lines, in one comment
group, directly above the `slog.Error` call with no blank line between:

    // Withhold the event when the fold fails: forwarding one nobody judged is the
    // leak (SPEC-015). Never log its event id, which is participant text.
    // #nosec G706 -- the sequence is an int64 given as a structured attribute.

The directive keeps its rule id and a reason that is a warning, not a pointer
to a block that no longer exists ("see above" goes). It stays the last line of
the group, adjacent to the call (the binding probed under Measurements). It is
inert under the pinned toolchain; keeping it is the brief's constraint and
costs nothing, and removing it is a lint decision this ticket does not name
(Q5). `grep -c 'campaign.Append is the only writer'
internal/gateway/seat.go` prints 0. SPEC-015 states the arm's reachability
with its searches (D1) and does not call it unreachable.

**D5. The sort's starting point is the table below; nothing is an id until
sign-off.** Forced by SPEC-008 and the `requirements` skill. Rows are lettered
so nothing here reads as an id. Each line: the rule as a what; the outcome
proposed; the evidence, verified against the test bodies; the one edit that
reds it. Every edit in the last column was run by the verifier in a scratch
clone of `74c547f`: it compiles (`go vet ./internal/gateway/` clean), and the
named test fails with the message quoted; the inverse edit restored the file
each time. "Closure" means `go test -count=1 ./internal/gateway/...
./cmd/vtt/...`.

| # | Rule (what) | Proposed | Evidence (verified against the body) | The edit that reds it (run) |
|---|---|---|---|---|
| A | A DM or agent connection is sent every event of the log after its cursor, each unchanged. | accept — the ticket's first candidate, first half, worded on the seat's promise; that `Project` also answers them with the event itself is `project.go`'s (`TestTheDMReceivesEverythingUnchanged`, `TestTheAgentSeatReceivesEverythingUnchangedToo`, refused as the projection's) | `TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection` (both roles; protojson of each envelope equal to the log's, one per event; resumed at head−1, exactly the last event) | in `subscribeFrom`, the unprojected `return after` made `return 0` (`dm resuming at after=10 received 11 envelopes, want exactly 1`); in `receive`, `return []*vttv1.Envelope{env}` made `return nil` (`dm received 0 envelopes, want 11`) |
| B | A player's or a spectator's connection is sent its projection of the log, never the log itself. | accept — the first candidate's second half, worded on the two roles a test drives; "an unknown one included" is refused as a clause: `identity.Verify` refuses a stored role that does not parse (SPEC-009), so no such connection is served, and no test observes it (closure green with `projected` answering true for player and spectator alone). SPEC-015 states it | `TestSessionZeroCannotHappenAgain` (player: the goblin not on the wire, the player's own token and board present), `TestASpectatorWithNoPerchReceivesNoBoard` (spectator) | `case identity.RoleDM, identity.RoleAgent:` made `..., identity.RolePlayer:` (`the goblin reached a player's connection`); made `..., identity.RoleSpectator:` (`a spectator perched on nobody was given a board`) |
| C | A projected seat resumed at a cursor is told what left its view while it was away. | accept — the second candidate split; "exactly what it missed" presumes the cursor is right, so each half is worded on what the test observes | `TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway` (the goblin's `TokenHidden` in the catch-up; `"x":19` absent) | in `subscribeFrom`, the projected `return 0` made `return after` (`a reconnecting player was never told the goblin left view`) |
| D | A projected seat resumed at a cursor is sent nothing at or below it. | accept — the second candidate's other half | the same test's last loop (no frame with sequence ≤ the cursor) | in `pastResume`, `e.GetSequence() > s.resume` made `>=` (`catch-up from after=12 re-sent something the seat already holds`) |
| E | A spectator's connection is shown no board until a perch names a shoulder. | accept — the third candidate; "watches nobody" is `eyes`'s reading of the empty viewpoint, and the seat's half is `viewerFor` leaving it empty | `TestASpectatorWithNoPerchReceivesNoBoard` (nothing naming `ambush` or `tok-`; the narration still arrives), `TestASpectatorHopsFromOneShoulderToAnother`'s first block | in `viewerFor`, `Viewpoint: "act-fighter"` added — the fixture's party member, since no edit can name a shoulder without naming one (`a spectator perched on nobody was given a board`) |
| F | A spectator may perch only on an actor whose kind is party member; controlling an actor does not make it one. | accept — the fourth candidate; "whoever controls it" is split by direction: a controlled non-party actor refused is observed, a party member nobody controls accepted is not (closure green with `MayPerch` also requiring a controller), so that direction is SPEC-015 prose | `TestASpectatorMayPerchOnAPartyMemberButNotOnAnNPC`, `TestASpectatorMayNotPerchOnAnNPCTheDMControls`, `TestAnActorWithNoDeclaredKindIsNoShoulderHoweverManyHoldIt` (all `viewpoint_test.go`), `TestASpectatorMayNotPerchOnTheGoblinArcher` (the wire), and `TestAuthorizeSpectatorMayNotPerchOnAnNpc` (`authz_test.go`, Q6) | `!ok \|\| !engine.IsPartyMember(a)` made `!ok \|\| len(a.GetControllerIds()) == 0` (reds the second and third); made `!ok \|\| a == nil` (reds the first, the fourth and the fifth) |
| G | Only a spectator perches. | refuse — the fifth candidate is VTT-139, VTT-140 and VTT-141 where it is reachable: the role table refuses `set_viewpoint` to every other role before `MayPerch` runs. `MayPerch`'s own role arm is defence for an exported function; with it made `if false {`, the closure reds `TestOnlyASpectatorRidesAShoulder` alone. SPEC-015 states the arm; the test stays uncited (Q3) | `TestOnlyASpectatorRidesAShoulder` | `p.Role != identity.RoleSpectator` made `p.Role == identity.RoleDM` (`role "player" must not perch`) |
| H | A perch refusal is the same text whether the named actor exists or not. | accept — the sixth candidate | `TestAPerchRefusalDoesNotSayWhetherTheActorExists` (byte equality over one id, two worlds) | the refusal's `ErrUnauthorized, actorID)` made `ErrUnauthorized, a.GetName())` (`a perch refusal must not say whether the actor exists`) |
| I | A perch naming no actor is accepted. | accept — the seventh candidate, narrowed: what the empty perch then sends (the creatures go, the terrain stays) is `reperch`'s, refused below as X | `TestUnperchingNamesNoActorAndIsAllowed`, `TestPerchingAppendsNothingToTheLog` (its `un-perch` must be ok) | `if actorID == "" {` made `if actorID == "" && false {` (`perch on "" refused: ... "" is not a party member`) |
| J | A perch appends nothing to the log. | accept — the eighth candidate, as VTT-036 is for a promotion; SPEC-007 states it for `set_viewpoint` and SPEC-015 points there; "and nobody else is told" is the same test's second assertion, held by the same edit, and stays prose | `TestPerchingAppendsNothingToTheLog` (the log's length unchanged over three perches; a DM connection hears nothing) | in `handleSetViewpoint`, after `perches.set`, one inserted statement appending a `NarrationAdded` through `s.campaign.Append` under a fresh `newEventID()` (`perching appended 3 event(s) to the campaign's history`); an absence has no token to flip, so its break is an insertion |
| K | A perch is judged against the state after the last event its seat received, never the campaign's head. | OPEN, Q9 — the ninth candidate, reworded: the seat's `world` includes events whose frames were withheld from it, so "the world the seat has been shown" is loose. No test observes it: with the pump's `sub.perch(actorID)` made `sub.pr.reperch(actorID, s.campaign.State())`, the closure stays green. The two differ only while events wait in the seat's subscription, a race no test builds | none | none reds a test (the probe above) |
| L | Every frame a perch sends carries sequence 0. | OPEN, Q10 — leaning accept: the tenth candidate's first half. The number is `project.go`'s (`perchSequence`, passed by `reperch`), the test drives `seat.perch`, and the edit that reds it is in `project.go`, as row M of the maps change was `mapdef`'s | `TestAPerchCarriesNoSequenceAtAll` (every frame of a perch on the hero has sequence 0) | `const perchSequence int64 = 0` made `= 7` (`a perch frame must carry no sequence, got 7`) |
| M | A resume cursor does not filter a perch's frames. | accept — the tenth candidate's second half | `TestAPerchIsNotFilteredByTheResumeCursor` (a seat resumed at the fixture's head), `TestAPerchOnAConnectionThatResumedAtHeadStillSendsTheBoard` (the wire) | in `perch`, `return s.pr.reperch(actorID, s.world)` made `return s.pastResume(s.pr.reperch(actorID, s.world))` (reds both: `got 0 frame(s)`) |
| N | Of a burst of perches set before the pump takes one, the pump is handed the last, an empty shoulder included. | accept — the eleventh candidate's first half | `TestARapidHopIsCoalescedToTheShoulderItEndedOn` (hero, goblin, hero, then ""; `take` answers "" with ok, then nothing) | in `set`, the assignment wrapped in `if !b.full { ... }` (first wins: `got "hero" (ok=true)`); `actorID, true` made `actorID, actorID != ""` (`got "" (ok=false)`) |
| O | A shoulder a burst flew past is served in full when named again. | refuse — the eleventh candidate's second half is `reperch`'s: it diffs against the projector's memory, and every edit that reds the test is in `project.go` (the test's own doc lists four). SPEC-015 states it as the reason coalescing loses nothing, pointing at `project.go`; the test stays uncited until the projection has a record (Q4) | `TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt` | not run (the projection's) |
| P | Perching while the table is busy keeps the watcher's stream in the order its projector changed. | OPEN, Q11 — leaning refuse: the twelfth candidate's first half is SPEC-011's single producer (the pump delivers perch frames and event frames from one goroutine), and the edit that reds it is `serve`'s. With the perch's frames enqueued from a goroutine of their own (the projection still computed in the pump), the test reds in 2 of 12 runs (`the watcher's own stream stops folding at envelope 10 of 38: engine: scene seen for unknown scene "ambush"`); the base is green in 12 of 12 | `TestHoppingWhileTheTableIsBusyKeepsOneOrder` (forty hops against a DM moving a token; the watcher's stream must fold) | in `serve`'s perches arm, `if ok && !deliver(s.encodeFrame, sub.perch(actorID), ...) { return }` made `if ok { frames := sub.perch(actorID); go deliver(s.encodeFrame, frames, ...) }` (red in 2 of 12) |
| Q | Perching stalls no other participant's command. | OPEN, Q11 — the twelfth candidate's second half, `perchBox.set` never blocking. Leaning accept with its rate, as the maps change accepted a race's 37 in 40: with `set` made to block, the DM's own command inside the test times out (`readResult: no CommandResult within 3s`, from the DM's goroutine) in 10 of 12 runs; with only the `default:` arm removed, 8 of 12; at the base, 0 of 12 | `TestHoppingWhileTheTableIsBusyKeepsOneOrder` (the DM's commands inside it) | in `set`, the `default:` arm removed from the `select` and, in `newPerchBox`, `make(chan struct{}, 1)` made `make(chan struct{})` (red in 10 of 12; the arm removed alone, 8 of 12) |
| R | A projected seat judges each event against the state that event produced, never the campaign's head. | accept, Q12 — from Done item 1 ("against which state") and the ticket's problem paragraph. The "state that event produced" half is observed widely; the "never the head" half is observed by `TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive` alone, through a probe no single edit makes: the seat holds no campaign, so the head was plumbed in by a package variable set in `serve` and read in `receive` (three insertions), and the test reds (`catch-up envelope 0 differs from the one seen live`) | `TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway` (the state after the event), `TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive` (not the head) | in `receive`, `campaign.FoldPrefix(s.received)` made `campaign.FoldPrefix(s.received[:len(s.received)-1])`, the state before the event: reds `TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway`, `TestASpectatorHopsFromOneShoulderToAnother`, `TestMoveTokenBroadcastBackfillsSceneAndFrom`, `TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen` and `TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt`, and leaves `TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive` green, whose doc says it pins this |
| S | A projected seat is sent nothing for an event whose fold fails. | OPEN — no test yet, Q13. From the ticket's problem paragraph. No test observes it (closure green with the arm forwarding the event); within one process the arm is not reachable, and two handles on one campaign reach it (`sequence=2 error="campaign: corrupt log at seq 2: engine: scene \"s\" already exists"`; Measurements) | none | in `receive`, the arm's `return nil` made `return []*vttv1.Envelope{env}` (closure green) |
| T | A perch is sent at once, without waiting for an event. | accept — from the ticket's "how a perch ... is applied" and Done item 1; the pump applies a perch when `wake` fires | `TestASpectatorHopsFromOneShoulderToAnother` (each perch's frames drained with nothing else happening at the table) | in `serve`'s perches arm, `sub.perch(actorID)` replaced by a function that sets `sub.pr.viewer.Viewpoint = actorID` and returns no frame, so the new eyes are used from the next event on (`sitting on Asme must show Asme's board, and show it at once`; also reds `TestAPerchOnAConnectionThatResumedAtHeadStillSendsTheBoard`) |
| U | A refused perch leaves the watcher on the shoulder they were on. | OPEN — no test yet, Q16. From the same wire test's second half ("AND NOTHING MOVES"), which does not observe it: its watcher is perched on nobody, and `eyes` refuses the archer a second time, so a refused shoulder that reached the pump would change nothing it can see. The order that holds the rule is `handleSetViewpoint`'s (SPEC-013 states it); the promise is the perch's. A watcher perched on Asme who asks for the archer would, under the edit, be silently taken off Asme | none (`TestASpectatorMayNotPerchOnTheGoblinArcher` is cited under F for the refusal) | in `handleSetViewpoint`, `perches.set` moved above `authorize`: the named test stays green, and so does the closure |

Refused, or not this sort's, each with its reason. **V**, `canSee` records
nothing — its one caller is the move gate, whose rules are VTT-155 and VTT-156,
and what it computes is `look`'s. **W**, the catch-up head is the seat's —
VTT-086, SPEC-011's. **X**, leaving a shoulder takes its creatures and not its
terrain (`TestLeavingAShoulderTakesTheCreaturesAndNotTheTerrain`) — `reperch`
and `transitions`, the projection's. **Y**, a perch arrives with the doors it
can see already open (`TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen`) —
`doorTransitions`, the projection's. **Z**, a perch on an unprojected seat or
on a seat that has folded nothing yields nothing
(`TestAnUnprojectedSeatCannotBePerched`, `TestASeatPerchesOnlyAgainstAWorldItHasSeen`)
— the first is unreachable over the wire (VTT-140), the second is `reperch`'s
nil guard; SPEC-015 states both. **AA**, a spectator sees through the shoulder
it rides, a perch on an NPC yields no sight, a player's viewpoint is ignored
(`project_test.go`'s `TestASpectatorRidesTheShoulderTheyPerchOn`,
`TestAPerchOnAnNpcYieldsNoSightAtAll`, `TestAPlayerCannotBorrowAnNpcsEyesByPerching`)
— `eyes`, the projection's. **AB**, the rest of
`TestASpectatorHopsFromOneShoulderToAnother` (the roster offered, Asme's room
without the archer, Armak's with it, both rooms remembered) — the projection's
output; the test is cited under E and T for what the seat decides. **AC**, the
DM and agent answered by `Project` with the event itself (`TestTheDMReceivesEverythingUnchanged`,
`TestTheAgentSeatReceivesEverythingUnchangedToo`) — `project.go`'s arm. Three
clauses stay prose: "an unknown one included" (B), a party member nobody
controls being perchable (F), "nobody else is told" (J).

Thirteen accepted outright (A to F, H, I, J, M, N, R, T), two leaning accept
at sign-off (L, Q), three OPEN with no test yet (K, S, U), and eleven refused
(G, O, P leaning, V to AC) with three refused clauses. Of the accepted, J's
red is an insertion rather than a flipped token, R's head half needs a
three-line probe because the seat holds no campaign, and Q's red is 10 in 12.

**D6. Rows are dispensed after sign-off, cited by citation lines only, and an
OPEN row has no citer.** Forced by SPEC-008 and the previous plans' D6. A row
the sign-off keeps OPEN is written `**OPEN — no test yet**` with no citation
line, and the report names it (ticket item 3). A citation line is `// VTT-NNN`
directly above `func Test`, below any doc block, several ids on one line where
a test holds several (`TestPerchingAppendsNothingToTheLog` carries I's and J's,
`TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway` C's and D's and, per
Q12, R's; `TestASpectatorWithNoPerchReceivesNoBoard` B's and E's;
`TestASpectatorHopsFromOneShoulderToAnother` E's and T's;
`TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive` R's alone). Evidence
cells are `internal/gateway/<file>#<Test>`, written by hand after the
dispenser; the chain gate refuses one that is wrong. SPEC-015's Requirements
line is copied from the register last. Ids start at VTT-176. The test files
that gain citation lines are `viewpoint_test.go`, `viewpoint_internal_test.go`
and `server_visibility_test.go`, and `authz_test.go` if Q6 says yes;
`project_test.go`, which the ticket lists, gains none, because every candidate
whose evidence is in it is refused as the projection's (AA, AC).

**D7. `seat.go`: the 26 blocks by symbol, and what each becomes.** Forced by
ticket items 2 and 4. Nothing in `seat.go` is exported, so it keeps no doc
sentence: a block above a function is a pointer sentence opening with the
function's own name, the shape `handleCommand`'s block took, or a warning
opening with a word on no function's list. The texts below were built into a
scratch copy of `seat.go` (the scratch copy, not the tree): `gofmt -l` prints
nothing, D11 prints `same` at 824, `measure` gives 46 comment lines of 165
non-blank (27.9 percent), `banned 0`, `blocks>6 0`, `check-comment-wrap.py`
flags nothing, and `check:doc-owner`'s `described` passes every function. A
starting point; the reading may shorten a text and may not lengthen one past
three lines.

| Block (by symbol) | Becomes |
|---|---|
| `viewerFor`'s doc (16) | `// Leave Viewpoint empty: a spectator watches nobody until a perch names a shoulder (SPEC-015).` |
| `projected`'s doc (19) | `// Answer false for the DM and the agent alone: a player answered false is sent the whole log (TestSessionZeroCannotHappenAgain).` |
| the `seat` type's doc (29) | `// Touch a seat from the pump alone once serve has drained its catch-up, and hand it anything else through a channel, never a lock: frames must leave in the order its projector changed (SPEC-011, SPEC-015).` |
| the `pr` field (4) | `// Keep pr nil for an unprojected seat: receive then folds nothing (SPEC-015).` |
| the `resume` field (3) | `// Drop output at or below resume, never input (pastResume, SPEC-015).` |
| the `received` field (20) | `// Fold these with campaign.FoldPrefix, never with a loop of your own (SPEC-015).` |
| the `world` field (7) | `// See receive, which sets it, and perch, which reads it (SPEC-015).` |
| the `perchBox` type's doc (63) | `// Keep one slot, latest wins, and never block set: a queue's depth is the client's to choose, and a blocking hand-off stalls the table (SPEC-015, docs/reports/2026-08-18-visibility.md). Guard the slot with mu and nothing else: the pump orders frames, not this lock.` |
| the `wake` field (3) | `// Keep wake at capacity 1: a waiting signal covers whatever the slot holds.` |
| `set`'s doc (3) | `// Never block here: the command goroutine must not wait on the pump (SPEC-015).` |
| `set`'s trailing `default: // already signalled; ...` | the comment goes; the `wake` field's warning holds it |
| `take`'s doc (2) | `// Report ok=false once the slot is empty: a second wake-up must not re-apply a shoulder (TestARapidHopIsCoalescedToTheShoulderItEndedOn).` |
| `newSeat`'s doc (3) | `// newSeat builds the seat SPEC-015 states for p's connection.` |
| `subscribeFrom`'s doc (4) | `// subscribeFrom is where the seat's subscription starts (SPEC-011), not where its output starts (pastResume).` |
| `receive`'s doc (5) | `// Return an unprojected seat's event itself, unchanged: the DM's and the agent's streams are the log (TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection).` |
| the block above `campaign.FoldPrefix` (4) | `// Hand Project the state after env, folded from what this seat received, never campaign.State(): during catch-up the head is the future (SPEC-015).` |
| the fold-failure block (28) | D4's three lines |
| the block above `s.world = world` (2) | `// Set world only after a fold succeeded: perch keeps the last good one.` |
| `pastResume`'s doc (28) | `// Add no fast path for resume 0: the filter already keeps every envelope there, and the guard would be a mutant no test can kill.` |
| the block in `pastResume`'s loop (2) | `// Keep this strictly greater, as store.Subscribe's is: after=N means the client holds N.` |
| `perch`'s doc (20) | `// Call perch from the pump alone, beside receive, and never pass its output through pastResume: its frames carry sequence 0 (SPEC-015).` |
| `perch`'s unprojected arm (5) | `// Return nothing: an unprojected seat has no projection to move (TestAnUnprojectedSeatCannotBePerched).` |
| `canSee`'s doc (4) | `// Build a fresh Projector per question: canSee must record nothing a seat was shown (SPEC-015).` |
| `catchUp`'s doc (24) | `// catchUp is the projected seat's catch-up SPEC-011 states; its head is the last sequence it returns, never the log's (VTT-086).` |
| `catchUp`'s closed-channel arm (4) | `// Answer what this seat got: a head it cannot reach is a second failure.` |
| the block above the head assignment (7) | `// Take the last frame's sequence, not a max: one event's frames share its sequence, and a max comparison is a mutant no test can kill.` |
| the block above the log-head test (2) | `// Stop at the first envelope at or past the log's head: the pump sends the rest.` |

The block above `campaign.FoldPrefix` keeps "the state after env" and names
`Project`, so `project_property_test.go`'s pointer ("which its own comment
calls what Project is specified to read") still finds its sense there. gofmt:
every field that has a comment today keeps a one-line comment above it, so no
field joins another's alignment group and gofmt changes nothing. If the reading
drops the `wake` field's comment, gofmt aligns `wake` with `mu`, `shoulder`
and `full`, which changes whitespace and no token; if it drops the comments
above `pr`, `resume` and `received`, the three align likewise. Either is named
in the report.

**D8. `viewpoint.go`: the three blocks.** Forced by ticket item 4. `MayPerch`
is exported and keeps one sentence; the two body blocks become warnings. Built
into the same scratch copy: `gofmt -l` prints nothing, D11 prints `same` at
110, 5 comment lines of 26 non-blank (19.2 percent), `banned 0`, `blocks>6 0`.

| Block (by symbol) | Becomes |
|---|---|
| `MayPerch`'s doc (38) | `// MayPerch reports whether p may perch on actorID's shoulder in st (SPEC-015).` |
| the empty-id arm (4) | `// Accept the empty id: it is how a spectator leaves a shoulder (TestUnperchingNamesNoActorAndIsAllowed).` |
| the refusal arm (5) | `// Answer an absent actor and a non-party one with this one string: a difference enumerates the DM's cast (TestAPerchRefusalDoesNotSayWhetherTheActorExists).` |

`go doc ./internal/gateway MayPerch` prints the signature and that sentence.

**D9. The sweep's rules, repeating the previous plans' where they apply.**
Forced by rule 10, SPEC-010 and the identity plan's D1 to D8.

- *Three kinds and nothing else*: a warning is imperative, a verb first, the
  consequence in the present tense, at most three lines; a pointer is
  `SPEC-007`, `SPEC-009`, `SPEC-011`, `SPEC-013`, `SPEC-015`, `VTT-NNN`, a
  test name, a symbol name or a `docs/` path, and may close a warning or a doc
  sentence in parentheses; a doc sentence is the first sentence `go doc`
  prints for an exported symbol. Every fact a deleted block held that SPEC-015
  does not state is either added to SPEC-015 (D10) or named in the report as
  dropped, with the reason.
- *Exported symbols keep one sentence*: `MayPerch` (D8).
- *A warning above a function opens with a word that names no function*: the
  list is under Measurements; `Project`, `Fold`, `Apply`, `Subscribe`,
  `State`, `Events`, `Append` and the lowercase `set`, `take`, `perch`,
  `receive`, `look`, `eyes` are on it.
- *The directive*: D4.
- *A doc sentence is one sentence, on one line where it fits*; the wrap band
  is `SHORT, LONG = 55, 85` in `tools/check-comment-wrap.py`, which
  `check:new-prose` applies to added lines.
- *Test files*: citation lines (D6) and the re-aimed blocks (D13). No other
  test-file edit; the doc blocks above the perch tests, many over the bound
  with banned terms, are the test-prose sweep's.
- *What is left alone*: every comment in `project.go`, `server.go`,
  `authz.go`, `internal/campaign` and `client/src`, stale or not (D18).

**D10. Facts only a comment holds go into SPEC-015.** Forced by ticket item 1
and the identity plan's D7. The reading of each cut block asks whether
SPEC-015's draft states the fact; if not, and the fact is about the code now,
it is checked against the symbol and written in, or named in the report as
dropped. The candidates are under "Candidates the reading starts from". The
searches SPEC-015 names for its absolutes, each run at verification: that a
perch appends nothing (`grep -n 'Append' ` over `handleSetViewpoint`'s body
and `seat.go` prints nothing, and `campaign.` in `seat.go` is `FoldPrefix`
alone); that `perch`, `receive`, `subscribeFrom` and `catchUp` are called
from `serve` alone (`grep -n '\bsub\.' internal/gateway/server.go`); that
`MayPerch`, `canSee`, `perchBox.set` and `projected` each have one production
caller (`grep -rn 'MayPerch(\|canSee(\|\.set(\|projected('` over non-test
files); the two writers and the absent lock (Measurements).

**D11. The comment-stripped comparison is a token stream.** Forced by the
identity plan's D11. The program is that plan's Task 0 listing
(`docs/superpowers/plans/2026-09-24-sweep-identity.md`: `go/scanner`, mode 0,
`fmt.Printf("%s %q\n", tok, lit)`), built once in the scratchpad
(`$S/codetokens/codetokens`, a `go.mod` with `module codetokens` beside it)
and run from the repository root:

    for f in internal/gateway/seat.go internal/gateway/viewpoint.go \
             internal/gateway/viewpoint_test.go internal/gateway/viewpoint_internal_test.go \
             internal/gateway/server_visibility_test.go; do
      git show "74c547f:$f" > "$S/before.go"
      "$S/codetokens/codetokens" "$S/before.go" > "$S/before.tok" || echo "SCAN FAILED $f"
      "$S/codetokens/codetokens" "$f" > "$S/after.tok" || echo "SCAN FAILED $f"
      printf '%s %s/%s tokens ' "$f" "$(wc -l < "$S/before.tok")" "$(wc -l < "$S/after.tok")"
      cmp -s "$S/before.tok" "$S/after.tok" && echo same || echo DIFFERS
    done

Done reads five `same` lines at 824, 110, 630, 2192 and 6280, plus
`authz_test.go` at 6622 and `keystone_test.go` at 4593 when Q6 and Q7 add
them. A run proves it ran by the counts; a zero is a failed run. A `DIFFERS`
on a test file means a string literal changed, and Phase 4b names it. Ticket
item 5 names `seat.go` and `viewpoint.go` only; the plan holds every file it
touches to the same check.

**D12. The ledger, last.** Forced by SPEC-010 (the band) and by
`--write-ledger` lowering a row on any drop. After Phase 4b has settled,
`python3 tools/check-comments.py --write-ledger`, then `git diff
tools/comment-ceilings.txt` must show exactly these rows changed, each lowered:
`internal/gateway/seat.go`, `internal/gateway/viewpoint.go` and
`internal/gateway/viewpoint_internal_test.go`, and `keystone_test.go` if Q7
says yes. The dry run's figures (72.4 to 27.9, 69.2 to 19.3, 40.4 to 28.2, 46.7
to 46.4) are what the draft texts give; the reading's final texts decide the
numbers. `viewpoint_test.go`, `server_visibility_test.go` and `authz_test.go`
gain citation lines only, which move no share (VTT-059). A further changed row
means a file changed that this plan does not name: stop, name it, and ask
before committing. The commit message lists the rows old and new.

**D13. The test-file blocks a re-aimed pointer forces to the bound.** Forced by
the ticket's "What it touches" item 3 and its third open question. Three
blocks in `viewpoint_internal_test.go`, all over the bound, point into
`perchBox`'s doc, which D7 cuts; a changed line in a block over the bound is
refused by `check:comments`, so each is sorted to the bound as it is re-aimed.
Built into a scratch copy: D11 prints `same` at 2192, and the file goes from
167 of 414 comment lines (40.3 percent) to 97 of 344 (28.2 percent), with
`blocks>6` from 7 to 4 and banned lines from 11 to 8.

| Block (by symbol) | Becomes |
|---|---|
| `TestARapidHopIsCoalescedToTheShoulderItEndedOn`'s doc (19) | `// TestARapidHopIsCoalescedToTheShoulderItEndedOn pins perchBox's latest-wins slot (SPEC-015). Keep the burst ending on a shoulder it did not start on, and on the empty one: a first-wins box and a box that reads "" as empty pass anything else.` |
| `TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt`'s doc (50) | `// TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt holds what coalescing rests on: a shoulder named again is served in full from the projector's memory (SPEC-015). Keep both checks on the burst's own room: either alone passes an empty perch.` |
| `threeRoomLog`'s doc (10) | `// Keep three rooms: a burst needs a shoulder to start on, one to fly past and one to end on.` |

The measurements those blocks narrate (the 11-against-3 frame count, the
stall shares, the four injections and the fifth that did not bite) are the
visibility report's (`docs/reports/2026-08-18-visibility.md` holds the first
two) and this change's report's; nothing is lost from the record. Per Q7,
`keystone_test.go`'s two small blocks are re-aimed the same way: the 2-line
block in its oracle's seat list becomes `// A spectator who has not named a
shoulder (SPEC-015).`, and `projectedSeat`'s 8-line doc becomes `//
projectedSeat is scenarios/goldens/<golden>/projections/<seat>/viewer.json.
Declare the viewpoint here, beside the stream: a perch is never in the log
(SPEC-015).` (scratch copy: D11 `same` at 4593, 554 of 1188 to 548 of 1182,
46.6 to 46.4 percent). Its third pointer, inside the 103-line doc above
`TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees`, is left: the
fact it cites is still in `Projector`'s doc in `project.go`, only the file it
names is wrong, and re-aiming one line there forces a 103-line block to the
bound, which is the test-prose sweep's (D18).

**D14. The specification edits, sentence by sentence.** Forced by ticket item
6 and check 6. SPEC-013 never says the perch's rule "has no record"; it sends
it to `viewpoint.go` twice and sends what a seat can see to `seat.go` twice,
once saying it has no record. All three sentences change:

- "Two checks for every role": "For `set_viewpoint` it runs `MayPerch`, whose
  rule is `viewpoint.go`'s." becomes "For `set_viewpoint` it runs `MayPerch`,
  whose rule is SPEC-015's."
- "The player's move gate", its last sentence: "What a seat can see is
  `canSee`'s and `viewerFor`'s in `seat.go` and the `Projector`'s in
  `project.go`, and has no record yet." becomes "`canSee` and `viewerFor` are
  SPEC-015's; what a projection sees is the `Projector`'s in `project.go`,
  which has no record yet."
- "What this record does not decide": "the perch's rule is `MayPerch`'s in
  `viewpoint.go`; what a seat can see is `seat.go`'s;" becomes "the perch's
  rule and what a seat is sent are SPEC-015's;".

No other sentence in SPEC-013 changes (its `set_viewpoint` paragraph stays its
own, and its Consequences keep "A perch does not survive a reconnect", which
SPEC-015 points at); `git diff` on it shows only these. SPEC-011, SPEC-007 and
SPEC-009 are pointed at and not edited.

**D15. The deliberate breaks, one per check this work relies on.** Forced by
the dev-cycle's rule that a check is proven by a red, and ticket items 2 to
6. In a scratch clone (`git clone --no-hardlinks` into the scratchpad, the
final `git diff HEAD` applied, the untracked files copied in, committed there,
that clone's `main` pointing at that commit): first each gate exits 0 with
its completion line; then each break is one edit, the finding recorded
verbatim, the inverse edit made by hand (never `git checkout --`), `git diff
--stat` printing nothing before the next.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | `// VTT-999` added to the citation line above `TestAPerchRefusalDoesNotSayWhetherTheActorExists` | `check:requirements-chain`: `viewpoint_test.go cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 | one new row's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `check:requirements-chain`: `does not carry the id, so the link walks one way only` |
| B3 | SPEC-015's Requirements line gains `VTT-999` | `check:requirements-chain`: `cites VTT-999 ... (specification citation)` |
| B4 | one pointer line, `// SPEC-015`, inside `pastResume`'s loop between two code lines where no block absorbs it, after the ledger is written | `check:comments`: `internal/gateway/seat.go: comment share X is above its ceiling Y and this change added a comment line to it (SPEC-010)` (the dry run: `28.31 is above its ceiling 27.9`) |
| B5 | `e.GetSequence() > s.resume` made `>=` in `pastResume` | D11's loop: `seat.go ... DIFFERS`, and `TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway` red (run at verification) |
| B6 | "Unreachable while campaign.Append is the only writer" re-inserted above the `#nosec` line | `grep -c 'campaign.Append is the only writer' internal/gateway/seat.go` prints 1 |
| B7 | the warning above `catchUp` rewritten to open with "Project" | `check:doc-owner`: ``the doc comment above `catchUp` begins by describing `Project`, which is a different function`` (run on the dry run) |

**D16. Phase 4b is the check for VTT-051 and for SPEC-015, sentence by
sentence.** The reviewer gets `git diff HEAD`, SPEC-007, SPEC-009, SPEC-010,
SPEC-011, SPEC-013 and SPEC-015, the rows VTT-050 to VTT-059, VTT-086,
VTT-139 to VTT-141 and the new rows, `seat.go`, `viewpoint.go`, the touched
test files, and the code SPEC-015 points into: `project.go`'s `Viewer`,
`Project`, `perchSequence`, `reperch`, `look`, `eyes` and `canSeeSquare`;
`internal/campaign/foldprefix.go` and `campaign.go`'s `Open`, `Append` and
`AppendBatch`; `server.go`'s `serve`, `answerCommand`, `handleSetViewpoint`
and the move gate in `handleCommand`; `authz.go`'s `Authorize`;
`internal/engine/actorkind.go`'s `IsPartyMember`; `internal/identity`'s
`Verify`. For every surviving block it names the kind (D9) and, for a
warning, the code it guards and whether the consequence is true of that code;
for a pointer, that the target resolves; for the doc sentence, that the
symbol is exported and the sentence true. For every deleted block: did it
hold a fact now in no record? If yes, SPEC-015 (D10) or the report. For every
SPEC-015 sentence: the symbol it names and whether the code under it does
what the sentence says — `catches.md` item 8 is the one to watch, since this
plan found two comments false already. For each dispensed row: the edit that
would red the test named. For the SPEC-013 edits: that nothing else changed.
For the rule-9 answer: that SPEC-015 names no other project.

**D17. Phase 4a is skipped, with its reason.** Independent QA derives tests
from a specification to find behaviour the implementer got wrong. This change
has no behaviour: D11 shows every Go file's code identical to `74c547f`, and
what is added is prose, register rows, citation lines and comment deletions.
What can be wrong is a sentence, and a reading holds that (D16). The report
records the skip under its own heading.

**D18. Things the ticket's scope leaves stale are left, and named.**
`project.go`'s `reperch` doc ("measured, and written up at perchBox with what
the difference is"), which after this change points at a measurement that is
in the visibility report and no longer at `perchBox`; `keystone_test.go`'s
"internal/gateway/seat.go's own doc comment says so" in the 103-line doc above
`TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees`, and all three
of its pointers if Q7 says no; `TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive`'s
doc ("It is also the test that pins WHICH state the pump projects against"),
which row R's probe shows is true of the head and not of the state before the
event; `perchFixtureLog`'s doc ("Sequences 1..6") and its seventh entry, an
`ActorControlGranted` its type switch has no arm for, so it is sent with no
payload and the fold skips it as an unknown variant — harmless to every perch
rule, since perching reads kind and not control; `TestOnlyASpectatorRidesAShoulder`'s
and the other perch tests' docs, histories over the bound;
`docs/verification-debt.md`'s oracle-corpus entry, which names
`TestARefusedLookStillFillsTheRoster`, a test the tree no longer holds. All
named in the report as the projection's, the test-prose sweep's or the debt
file's. The report does not revise the reports of the periods the cut blocks
narrate.

**D19. One commit for the change; the report in its own.** Forced by the band
(a file's deletions and its row land together), by the chain gate (a row's
evidence and its citation line land together, and neither hook runs
`check:requirements-chain` or `check:comments`), and by the pointers
(`SPEC-015` must have a target in the same tree). The commit carries the
ticket (untracked today), this plan, SPEC-015, the SPEC-013 edits, the
register, `seat.go`, `viewpoint.go`, `viewpoint_test.go`,
`viewpoint_internal_test.go`, `server_visibility_test.go`, `authz_test.go` and
`keystone_test.go` per Q6 and Q7, and the ledger. The pattern is `bb7b535`
then `74c547f`.

**D20. Gate steps before the commit, in order, on a tree the review has
settled.** `gofmt -l internal/gateway/` prints only `scenario_test.go`; `go vet
./internal/gateway/`; `go test -count=1 ./internal/gateway/...` green; D11's
loop, every `same`; `task check:comments` (expected, before Task 10, to refuse
`seat.go`, `viewpoint.go` and `viewpoint_internal_test.go` for the band and
nothing else); `task check:doc-owner`; `task check:requirements-chain`
(`<175 + N> rows, 193 test files, 9 specifications; every citation resolves
and every row's evidence holds`); `task check:new-prose` (every test and
symbol name the sweep writes must resolve; `SPEC-015` is not read by that
gate, so the pointer is held by the reading); `python3
tools/check_mutation_test.py -q` and `python3 tools/check_ts_mutation_test.py
-q` (no key names either file; `OK` expected without a re-point, and if
either reds, stop: a key this plan says does not exist does); `task lint`
(the `#nosec` line in place, `0 issues`). Then, after Task 10, `task check`
whole, once, on the final tree, launched in its own session
(`start_new_session=True`), and only after Phase 4b has settled, so no edit
lands mid-run; `uptime` first, since the e2e waits fail under load (the
`TestQAMCPConnect*` deadline tests failed twice during verification's closure
runs, under edits that cannot reach them, and one of them failed at the base
too, once in three runs). Then the pre-commit
hook's own set, with `git add` and `git commit` in separate calls, and what
landed checked with `git show --stat HEAD`.

## Candidates the reading starts from

Read by the verifier at `74c547f`. A starting point, not a verdict.

**Facts for SPEC-015 that only a comment states today.** `projected` is not
the boundary on its own: `Project` answers the DM and the agent with the event
itself as well, and what `projected` decides for them is the subscription
start, the head and the cost — the comment's "changes no byte on any wire" is
false (Measurements). A player answered false by `projected` would be sent the
whole log. An unprojected seat's `pr` is nil, not a pass-through projector, so
neither `FoldPrefix` nor `sight.VisibleFrom` runs on its path. `received` is
kept because each event's state is a fold of the prefix, and advancing one
state here would be a second fold; it is per connection, grows with the log
and goes with the connection. `world` is nil until the first event folds, and
a failed fold leaves the last good one. One goroutine owns a seat once catch-up
is drained, and there is no lock because of it; the command goroutine hands a
shoulder across `perchBox` and never reaches in. `perchBox`'s `mu` guards the
slot alone and orders nothing on the wire. `wake` has capacity one and carries
no data, so a dropped second signal loses nothing. The command goroutine still
pays per hop an `authorize` and an identity lookup; the re-projection is what
coalesces, one shoulder per wake, whatever arrived. A perch is sent at once,
at a quiet table as at a busy one, against the state after the last event the
seat received, because judging it against the head would introduce what the
seat is still replaying. A perch's frames skip `pastResume` because its
question ("does the client already hold this?") is asked by sequence, and a
perch's frames carry 0; filtered, a spectator resumed at the head would be
told ok and shown nothing. `pastResume`'s comparison is strictly greater,
matching `store.Subscribe`'s. `catchUp` for a projected seat queues its head
after the whole backlog is projected, and when `events` closes mid-backlog it
answers what the seat got. `MayPerch`: only an actor whose kind is party
member, enforced on the server; `eyes` refuses the same perch a second time;
the roster, `eyes` and `MayPerch` share one predicate; the role arm exists
because `MayPerch` is exported; the two refusals are one string because two
would let a watcher enumerate the DM's cast; the refusal names only the id
sent; the empty id is how a spectator leaves a shoulder and shows nothing.

**Text that goes on sight.** Every `spec §`, `Task N`, date, `Patrik ruled`,
`MEASURED`, `used to`, `until review`, `This comment claimed`, `NO SECOND
MACHINE`, `12 runs in 12`, `15 in 24, 23 in 36, 8 in 12`, `0 in 72`, `EIGHT
separate occasions` and its parenthetical; the whole retraction paragraph of
the `received` field ("A SINGLE engine.State WOULD NOW DO ... Retraction left
on 2026-08-31"); "the pump's own comment used to say it had none"; the
"A mutex was tried" paragraph; every sentence that says what the code does
rather than what to keep; `receive`'s "Unreachable while campaign.Append is
the only writer" (false) and the gosec investigation narrative ("checked by
removing each argument in turn"); `projected`'s "changes no byte on any wire
— MEASURED" (false); `MayPerch`'s quotation of the visibility ticket and its
"Task 5's move_token oracle one command over".

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository; the scratchpad, `$S`.

Build D11's program; run the loop. Run `python3 tools/check-comments.py
--report | grep -E 'internal/gateway/(seat|viewpoint|viewpoint_test|viewpoint_internal_test|server_visibility_test|authz_test|keystone_test)\.go'`,
`python3 tools/check-requirements-chain.py .`, `python3
tools/check-comments.py main`, `python3 tools/check-doc-owner.py .`, `gofmt -l
internal/gateway/`, `grep -c 'campaign.Append is the only writer'
internal/gateway/seat.go`, both mutation self-tests, and `grep -c '^// VTT-'`
over the three perch test files; keep the outputs. Confirm `requirement-id`
is on the path.

**Done when:** the outputs match the measurements above (`same` at 824, 110,
630, 2192, 6280, and 6622 and 4593 if needed; `175 rows ... 8
specifications`; `239 files ... clean`; only `scenario_test.go` from gofmt;
`1`; both self-tests `OK`; citations 0, 0 and 4); `requirement-id` prints its
one-line usage.

### Task 1 — SPEC-015

**Files:** `docs/specifications/015-the-seat-and-the-perch.md`, new.

Per D1, D2, D3 and D10; the `specification` skill's steps 1 to 5 and 7
(`catches.md`, two passes at most). Step 6's Requirements line is written
`None yet; the sort of 2026-09-29-the-seat-and-the-perch-have-a-record-design.md
fills it` until Task 8 and then copied from the register.

**Done when:** the file has the five headings in order (`grep -c '^## '`
prints 5), no `Why`, no `Rejected`, no number that is a measurement, no line
number, no date outside a `docs/` path, no path outside the project, no other
project's name; every sentence under "How it works" names a symbol in
`seat.go`, `viewpoint.go`, `project.go`, `server.go`, `authz.go`,
`internal/campaign`, `internal/store`, `internal/identity` or
`internal/engine`, and a table of sentence to symbol is kept for Task 9; the
three compressions under Measurements and the two false comments are stated
as the code has them; every "only", "never", "every", "none" and "alone"
names its search; `python3 tools/check-requirements-chain.py .` prints `9
specifications`.

### Task 2 — SPEC-013

**Files:** `docs/specifications/013-authorization.md`.

D14's three sentences.

**Done when:** `grep -n "viewpoint.go\|seat.go" docs/specifications/013-authorization.md`
prints no line that sends the perch's rule or what a seat is sent anywhere but
SPEC-015; `git diff --stat` on the file shows only D14's lines.

### Task 3 — `seat.go`

**Files:** `internal/gateway/seat.go`.

D7's table and D4, under D9; every fact of the cut blocks checked against
SPEC-015's draft, each gap into SPEC-015 (D10) or the report's dropped list.

**Done when:** D11 prints `same` for `seat.go` at 824; `--report` shows
`seat.go` with `banned 0` and `blocks>6 0`; `grep -c 'campaign.Append is the
only writer'` prints 0; the `#nosec G706` line is the last line of the comment
group directly above `slog.Error` with no blank line between; `gofmt -l`
prints nothing for it; `check:doc-owner` still ends `79 files ...`; `task
lint` prints `0 issues`.

### Task 4 — `viewpoint.go`

**Files:** `internal/gateway/viewpoint.go`.

D8's table, under D9.

**Done when:** D11 prints `same` at 110; `--report` shows `banned 0` and
`blocks>6 0`; `go doc ./internal/gateway MayPerch` prints one sentence;
`gofmt -l` prints nothing for it.

### Task 5 — The re-aimed test blocks

**Files:** `internal/gateway/viewpoint_internal_test.go`, and
`internal/gateway/keystone_test.go` per Q7.

D13.

**Done when:** D11 prints `same` for each; `grep -n "perchBox for the
measurement\|written up at perchBox\|FIFO its comment\|bench perchBox" internal/gateway/viewpoint_internal_test.go`
prints nothing; each touched block is at most six lines; per Q7, `grep -n
'seat.go: ' internal/gateway/keystone_test.go` prints nothing; `python3
tools/check-comments.py main` refuses nothing but the band on the swept files.

### Task 6 — Local gates, first pass

**Files:** none changed.

D20's list up to and not including `task check` whole.

**Done when:** each step exits 0 with its completion line, except
`check:comments`, which at this point refuses the swept files for the band and
nothing else; both mutation self-tests print `OK`; `check:new-prose` reports
no citation to a name the tree never declared.

### Task 7 — Sign-off of the sort

**Files:** none.

D5's table is presented with the reading's confirmations and overrides, each
override with its reason, and the questions below answered. Nothing is
dispensed before the answer.

**Done when:** each lettered row has one of: accept (with wording and
evidence), refuse (with reason), OPEN (with reason); Q1 to Q16 have answers.

### Task 8 — Rows and citation lines

**Files:** `docs/requirements.md` (by the dispenser, then evidence cells by
hand), `internal/gateway/viewpoint_test.go`, `viewpoint_internal_test.go`,
`server_visibility_test.go` and, per Q6, `authz_test.go` (citation lines only,
D6), `docs/specifications/015-the-seat-and-the-perch.md` (the Requirements
line, copied).

**Done when:** `python3 tools/check-requirements-chain.py .` prints `<175 + N>
rows, 193 test files, 9 specifications; every citation resolves and every
row's evidence holds` with N the accepted count plus the OPEN rows; an OPEN
row has no citer; D11 prints `same` for every test file; `--report` shows
`cites` equal to the citation lines per file and each file's share unchanged
by them.

### Task 9 — Phase 4b, the reading review

**Files:** whatever its findings touch among the above.

Per D16. Findings are fixed and the affected task's "done" is re-run. The
review settles before Task 10 starts; if the reviewer dies on a model's limit,
say so and re-dispatch with the same brief on `fable`.

**Done when:** the review record names every surviving block's kind, every
deleted block's outcome, every SPEC-015 sentence's symbol and verdict, every
row's red-making edit, the SPEC-013 diff, and reports no open finding; D11
prints every `same`.

### Task 10 — The ledger

**Files:** `tools/comment-ceilings.txt`, by `--write-ledger` only.

Per D12.

**Done when:** `git diff tools/comment-ceilings.txt` shows exactly D12's rows,
each lowered; `task check:comments` ends `clean`; `--report` prints `banned 0`
and `blocks>6 0` for `seat.go` and `viewpoint.go`.

### Task 11 — The breaks and the whole gate

**Files:** none in the repository.

D15 in a scratch clone; then `task check` whole, once, per D20.

**Done when:** the clone's clean run exits 0 with each completion line, and
each of B1 to B7 produces the one red D15 names; `task check` exits 0 with
every step, `check:comments`, `check:requirements-chain`, `check:doc-owner`,
`check:mutation` and `lint` among them, printing its own verdict.

### Task 12 — Commit, then the report

**Files:** the commit's, per D19; then
`docs/reports/2026-09-29-the-seat-and-the-perch-have-a-record.md`.

The commit message lists the ledger rows old and new, D11's counts, the rows
dispensed, and B4's finding line verbatim. After it: the report per the
`implementation-report` skill, in its own commit — per file comment lines and
share before and after, blocks kept by kind, the facts SPEC-015 took and the
facts dropped with reasons, the sort (every row with its id and test, every
refusal in one line), the breaks' finding lines, the gaps below as found or
closed, D18's stale list, the sign-off answers, the rule-9 answer, and the
fold-failure arm's reachability with the probe that reached it.

**Done when:** `git show --stat HEAD~1` lists the ticket, this plan, SPEC-015,
SPEC-013, the register, `seat.go`, `viewpoint.go`, the touched test files and
the ledger, and nothing else; `git diff --stat 74c547f -- docs/reports/` lists
only the new report; `git diff --quiet 74c547f -- internal/campaign
internal/engine internal/store internal/identity cmd/ contract/ client/src
internal/gateway/project.go internal/gateway/server.go
internal/gateway/authz.go` exits 0.

## Commits

| Commit | Carries | Gate steps it runs |
|---|---|---|
| C1 | the ticket, this plan, SPEC-015, the SPEC-013 edits, the register, `seat.go`, `viewpoint.go`, the touched test files, the ledger | D20's list by hand, `task check` whole (Task 11), then the pre-commit hook (lint, vet, tier-1, arch, vocabulary, doc-owner, secrets, typecheck, review gate) |
| C2 | the implementation report (and a debt entry if Q13 says so) | pre-commit hook |

Push after C2: pre-push runs tiers 2 and 3 and the contract gates, about three
minutes; let it finish. No Go code changes, so no mutation key moves and
`check:drift` has no client change to compare.

## Gaps that travel with this plan

1. **The fold-failure arm is observed by no test and is reachable only from
   outside the process.** With the arm forwarding the event it could not
   judge, and with a failed fold clearing `world`, the closure stays green
   (the second run's `cmd/vtt` failures were the MCP deadline tests, gap 11).
   Two `campaign.Open` handles on one directory reach it (Measurements), and
   the same log then refuses the next `Open`, so the campaign stops booting.
   Row S, Q13.
2. **The ticket's premise for "unreachable" is false and its conclusion holds
   only within one process.** `campaign.AppendBatch` writes the log too, and
   both writers fold first; a second process can still shape a log no prefix
   of which folds. SPEC-015 states it with its searches.
3. **`projected`'s own comment is false.** Making it answer true for the DM and
   the agent leaves the closure green, yet changes the `CatchUpHead` a DM
   resumed at the head is told (11 at the base, 0 under the edit). SPEC-015
   states what `projected` decides without the comment's claim.
4. **A perch judged against the head is unobserved** (row K): with the pump's
   perch judged against `s.campaign.State()`, the closure stays green. The two
   states differ only while events wait in the seat's subscription. Q9.
5. **`TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive` does not pin what
   its doc says.** Judging each event against the state before it leaves it
   green (live and catch-up then agree on the wrong answer); it reds only when
   `receive` is handed the campaign's head, which no single edit of `seat.go`
   can do. Row R cites it for the head and the reconnect test for the state
   after the event; its doc is left stale (D18).
6. **Two clauses of the ticket's candidates are unobserved**: "an unknown
   [role] included" (closure green with `projected` answering true for player
   and spectator alone; unreachable, since `identity.Verify` refuses a role
   that does not parse) and a party member nobody controls being perchable
   (closure green with `MayPerch` also requiring a controller: every actor a
   test passes through `MayPerch` has one, and the seat-level fixtures' actors
   that have none — `perchFixtureLog`'s hero, whose grant its own type switch
   drops, and `threeRoomLog`'s three — never reach `MayPerch`). Both are
   SPEC-015 prose.
7. **The `#nosec G706` directive is inert under the pinned toolchain**, so its
   binding to the call cannot be observed at `seat.go`; it was probed on a
   `G115` directive elsewhere. D4 keeps it; Q5.
8. **Rows P and Q rest on a probabilistic red.** Q's edit reds in 10 of 12
   runs, P's in 2 of 12, the base in none of 12; the races are the rules' own
   nature, and nothing deterministic observes either without a seam this ticket
   may not add. Q11.
9. **The ticket's file list omits files that point into the cut blocks or hold
   a test deferred to it**: `authz_test.go`
   (`TestAuthorizeSpectatorMayNotPerchOnAnNpc`, named by the authorization
   report as this ticket's), `keystone_test.go` (three quotations of
   `seat.go`), `project.go` (`reperch`'s "written up at perchBox"). Q6, Q7, Q8.
10. **Item 5 names `seat.go` and `viewpoint.go` only**; D11 holds every touched
    file to the same check. **Item 6's "sentences that say the perch's rule has
    no record"** matches no sentence literally: SPEC-013 has three that send
    the rule or what a seat can see elsewhere (D14). **Item 3's "more than
    175"** holds for any N ≥ 1; the report states N.
11. **The `cmd/vtt` MCP deadline tests failed during two closure runs** whose
    edits cannot reach them (`TestQAMCPConnect*`, a subprocess that ignores
    stdio, under a load of 4 to 7 from a parallel session). At the base, `go
    test -count=3 -run TestQAMCPConnect ./cmd/vtt/` failed once in three
    (`TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessNeverWrites`), so the
    failures are the environment's; `task check` is launched with `uptime`
    first (D20).
12. **SPEC-015's file name carries "and"**, which `catches.md` item 14 reads as
    two records. Q1.

## Questions for sign-off

1. **One record or two?** The ticket names `015-the-seat-and-the-perch.md`,
   and `catches.md` item 14 says a file that needs "and" in its title is two
   records. Recommend one record, the ticket's file name kept and the title
   stating one decision (D1's: what a connection is sent is decided by its
   seat, whose eyes only a spectator may move): a perch is applied only
   through the seat (`perch` against `world`, beside `receive`, skipping
   `pastResume`), so two records would each point at the other for half of
   every perch sentence.
2. **Does SPEC-015 name MapTool?** Recommend no (D3): the empty opening
   viewpoint, the kind rule and the server-side refusal are stated as facts
   about this system; the refused whole-campaign distribution, fallback eyes
   and selection-merged memory are the rule-9 answer in this plan and the
   report.
3. **Row G, only a spectator perches: refuse as a row?** Recommend refuse:
   VTT-139, VTT-140 and VTT-141 hold it wherever it is reachable, and with
   `MayPerch`'s role arm removed the closure reds only
   `TestOnlyASpectatorRidesAShoulder`, a test of an exported function's
   defence. SPEC-015 states the arm; the test stays uncited.
4. **Rows O, X, Y, AA and AC, the projection's outputs: refuse here and leave
   their tests uncited until `project.go` has a record?** Recommend yes: every
   edit that reds them is in `project.go`, and dispensing them under SPEC-015
   would make SPEC-015 the projection's record by the back door. SPEC-015
   states O as the reason coalescing loses nothing and points at `project.go`.
5. **The `#nosec G706` line: keep it, shortened, though it suppresses nothing
   under golangci-lint 2.11.4?** Recommend keep (D4): the brief makes it a
   constraint, gosec flagged the call when the line was written and may again
   after an upgrade, removing a suppression is a lint decision this ticket
   does not name, and the cost is one line of the three that remain. The
   report records that it is inert today.
6. **`authz_test.go`: cite `TestAuthorizeSpectatorMayNotPerchOnAnNpc` under
   row F?** Recommend yes, a citation line only: the authorization report
   named it as this ticket's, it is the test that shows `Authorize` asks
   `MayPerch`, and a citation line moves no share (VTT-059). The ticket's file
   list does not name `authz_test.go`, so this needs the yes.
7. **`keystone_test.go`: re-aim its two small blocks that quote `seat.go`'s cut
   comments, and leave the third?** Recommend yes (D13): after the sweep the
   quotations name text that no longer exists; the 2-line block takes one
   line and the 8-line `projectedSeat` doc goes to three; the third sits in a
   103-line block whose fact still has a home in `Projector`'s doc, and sorting
   it is the test-prose sweep's. The file is not in the ticket's list.
8. **`project.go`'s `reperch` doc: leave "written up at perchBox" stale?**
   Recommend yes, named in the report (D18): the sentence sits in a 29-line
   block of the projection's file, one changed line forces the block to the
   bound, and the measurement it points at is in
   `docs/reports/2026-08-18-visibility.md`.
9. **Row K, a perch judged against the seat's last state and never the head:
   OPEN, or SPEC-015 prose?** Recommend OPEN — no test yet: the rule is real
   and breakable (the probe's edit is one expression in `serve`), a test that
   holds an event in the subscription while a perch is taken is writable, and
   an OPEN row records the absence as an absence. The report carries the
   probe.
10. **Row L, a perch's frames carry sequence 0: accept under SPEC-015 though
    the constant is `project.go`'s?** Recommend accept: the promise is about
    what a perch sends, the test drives `seat.perch`, the projection has no
    record to hold it, and the maps change set the precedent with a row whose
    red-making edit was another package's. SPEC-015 names `perchSequence` in
    `project.go`.
11. **Rows P and Q, the busy table: refuse P, accept Q with its rate?**
    Recommend both. P is SPEC-011's: the pump is the one producer, and
    SPEC-011's delivery paragraph already says a perch is applied and
    delivered there so the order sent is the order the projector's memory
    changed in; its red is `serve`'s and comes 2 times in 12. Q is the seat's:
    its mechanism is `perchBox.set` never blocking, its consequence is at the
    table (the DM's own command times out), and its red comes 10 times in 12;
    the report carries the rate.
12. **Row R, each event judged against the state it produced: accept, and with
    which evidence?** Recommend accept, citing
    `TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway` (the state after
    the event: judged against the state before it, the reconnect loses the
    goblin's departure) and `TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive`
    (never the head: judged against the head, catch-up differs from live), and
    naming in the report that the second is reddened only by a three-line
    probe, because the seat holds no campaign.
13. **Row S, the fold-failure arm: OPEN row, and an Open-debt entry in C2?**
    Recommend both: a check could exist (the verifier's two-handle probe is its
    recipe), so the rule keeps an id with its absence recorded, and
    `CLAUDE.md` names `docs/verification-debt.md` as the one file for known
    coverage gaps. The entry also records that nothing stops a second process
    writing one campaign's log, which is outside this ticket. The ticket's
    file list does not name the debt file, so this needs the yes.
14. **One commit for the change, the report separately (D19)?** Recommend yes.
15. **If a swept block lands with every line a true warning, pointer or doc
    sentence and still over the bound?** Recommend the reading governs: stop,
    report the block, and let the ticket's writer decide, rather than cut a
    true warning for the bound.
16. **Row U, a refused perch leaves the watcher where they were: OPEN — no test
    yet?** Recommend OPEN, with the test that would close it named in the
    report (perch on `act-fighter`, ask for `act-goblin-archer`, assert the
    refusal and no `TokenHidden` for `tok-fighter`): the rule holds today by
    `handleSetViewpoint`'s order, and the one wire test that looks like it
    observes it cannot, because its watcher starts on nobody. Writing the test
    is outside a comments-only ticket.
