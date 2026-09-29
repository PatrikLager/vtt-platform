# The projection has a record — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-29-the-projection-has-a-record-design.md`
**Verified:** 2026-09-29, by `verify-ticket`, an agent that did not write the
ticket, against `db1f77d` on `chore/spec-016-projection` (`main` at the time;
the ticket untracked). Verdict: **Passes with gaps.** The gaps are listed at
the end and travel with this plan. This plan does not edit the ticket; where an
item is thin or a sentence of it is loose, the plan decides around it and says
so.

**Goal, in the ticket's words:** `docs/specifications/016-the-projection.md`
exists and states which roles are projected and what `Project` answers each,
whose eyes a viewer has, what `look` computes, the projector's memory and what
`transitions` synthesizes from it in its order, `classify`'s rulings arm by
arm, doors, the sequence every frame carries, and what the record does not
decide; its Status says which of its sentences the owner's two open rulings
will change; the `UNRESOLVED` and `written up at perchBox` passages are gone
from `project.go` and at most one `FLAGGED FOR ADJUDICATION` is left; every
rule the sort accepts has a row cited by a gateway test; `project.go` carries
only warnings, pointers and the doc sentences of its exported symbols, with no
banned line and no block over the bound; SPEC-013 and SPEC-015 point at
SPEC-016 where they say the projection has no record; and no code line
changes.

**The owner's rulings of 2026-09-29, which bound this plan.** (1a) Narration
stays forwarded to every viewer: this resolves the narration arm's `FLAGGED FOR
ADJUDICATION`. (2c) A note will carry a visibility flag, public or DM-only: a
later ticket (contract, projection, DM console, notes panel). (3) Only what a
viewer sees gives them information, for every actor, party members included: a
later ticket (`classify`'s knows and already-holds arms, and a party member's
status only while seen). Order: this record, then the notes flag, then the
testimony rule. SPEC-016 describes today's code, its Status names what (2c) and
(3) will change (D3), and the sweep leaves the least text on the arms (2c) and
(3) will rewrite (D7).

**MapTool (CLAUDE.md rule 9), the answer this plan records.** The seat arc's
answer (`docs/superpowers/plans/2026-09-29-the-seat-and-the-perch-have-a-record.md`)
stands: MapTool decides what a client sees on the client, from a whole campaign
every client holds. What matters for the projection: there is no projection at
all. `MapToolServer.addRemoteConnection` sends a joiner
`SetCampaignMsg(campaign.toDto())`, every zone with all its tokens, GM layer
included, its topology and its exposed-area history, and
`ServerMessageHandler` relays each change with `sendToClients` or
`sendToAllClients`; a grep over `server/` for
`isOwner|playerOwns|isGM|getRole|isVisible|filter` finds only handshake role
setup. Vision is computed per token on each client: `ZoneView.getTokenVisibleArea`
runs the token's `SightType.getVisionShape` through `FogUtil.calculateVisibility`
against the zone's noded topology, caches it per token and per view, and
`flush`/`flushFog` drop the caches on `WallTopologyChanged`,
`MaskTopologyChanged`, `TokensChanged` and `TokensAdded`. `ZoneView.getVisibility`
returns a `Visibility` record of four areas: visible (what the view's tokens see
now), exposed (the fog-of-war history), soft fog (cleared of hard fog) and clear
(visible and exposed). A token entering or leaving view is a render decision,
never a message: `ZoneViewModel.updateTokenPositions` skips a token that is not
`isVisible`, is on the GM layer or is visible only to its owner, and
`ZoneRenderer.isTokenInNeedOfClipping` clips the rest to the visible area; a grep
for `TokenVisibilityChanged|tokenEntered|tokensSeen` prints nothing. A map hidden
from players (`Zone.isVisible`) is still sent to them (`SET_ZONE_VISIBILITY_MSG`
goes out with `sendToAllClients`) and hidden by their UI. MapTool has no doors;
walls and masks change for every client at once. So: borrowed is the split
between what a view sees now and what it remembers (here `SceneSeen`'s visible
set against the client's `Explored`), a view as the union of its eye tokens'
sight, and sight recomputed from the tokens rather than stored. Refused, each
with its reason: the whole campaign sent and clipped on the client, which is the
distribution model rule 9 names, and a projected seat is its opposite (the
server sends each viewer only what `classify` and `transitions` admit); a
sighting as render state, refused because a client that was never sent a token
cannot draw it, so arrival and departure travel as `TokenPlaced` and
`TokenHidden`; a hidden map sent and hidden by the UI, refused because a scene
reaches a viewer only when one of its eyes stands in it. The door correction
(`doorTransitions`) has no MapTool precedent. The notes behind this paragraph
were written by an Explore agent against `~/dev/RPTool/maptool` and spot-checked
by the verifier with grep (`record Visibility` in `ZoneView`, `flush` and
`flushFog`, the empty sighting grep, `SetCampaignMsg` in `MapToolServer`,
`isTokenInNeedOfClipping`, `SET_ZONE_VISIBILITY_MSG` relayed with
`sendToAllClients`). Where this is written is D4; SPEC-016 names no other
project.

## Verification, check by check

1. **Every path resolves — by command.** `internal/gateway/project.go` declares
   every symbol the ticket names: `Viewer`, `Projector` (fields `viewer`,
   `scenes`, `actors`, `tokens`, `seen`, `doors`), `NewProjector`, `Project`,
   `sightRangeNotSupplied`, `toleranceNotSupplied`, `perchSequence`, `reperch`,
   `sightView`, `look`, `eyes`, `transitions`, `sceneSeenFor`, `objectInSight`,
   `verdict` (`unrecognised`, `withheld`, `forwarded`), `passIf`, `classify`
   (with its `knows` closure), `canSeeSquare`, `doorTransitions`, `doorSubject`,
   `squareAt`, `squareKey`, `sortedSet`, `sortedSceneIDs`,
   `sortedSceneIDsUnion` and `sameSet`. `internal/sight` declares `VisibleFrom`,
   `Blockers` and `Clear`; `seat.go` declares `pastResume`, `receive`, `perch`
   and `canSee`; `client/src/session.ts` exists. The six test files the ticket
   names exist. `docs/superpowers/specs/2026-08-18-visibility-design.md` has
   §3.1, §3.1.1, §3.2, §4.1 to §4.4, §5 and §5.1, and
   `docs/reports/2026-08-18-visibility.md` exists. `docs/specifications/016*`
   does not exist; 016 is the next free number.
2. **"Done" is an observation — by command where a command exists.** Item 1:
   the file does not exist. Item 2: `grep -c` prints `1`, `1` and `2` for
   `UNRESOLVED`, `written up at perchBox` and `FLAGGED FOR ADJUDICATION` in
   `project.go`, as the ticket says. Item 3: `python3
   tools/check-requirements-chain.py .` prints `193 rows, 193 test files, 9
   specifications; every citation resolves and every row's evidence holds`;
   `project_test.go`, `project_internal_test.go`, `project_property_test.go`
   and `keystone_test.go` carry no citation line. Item 4: `--report` prints
   `61.6%  ceiling  61.7  banned  64  blocks>6  30` for `project.go`. Item 5 is
   an invariant (D10 holds it at every task). Item 6: the sentences exist, one
   in each specification (D13). Item 7 is the gate. That every surviving block
   is a warning, a pointer or a doc sentence, and that every sentence of
   SPEC-016 is true of its symbol, is Phase 4b's reading, as the ticket says.
3. **Each rule is breakable — a reading, then probes.** D5 names, per
   candidate, the tests that go red and the one edit that reds them. Every edit
   named there was run by the verifier in a scratch clone of `db1f77d`: it
   compiles (`go vet ./internal/gateway/` clean) and reds the tests named; the
   file was restored after each run and `git status` was clean. Every claim in
   this plan that a thing is observed by no test was probed across
   `go test -count=1 ./internal/gateway/... ./cmd/vtt/...`, the whole set of
   packages that import the gateway. Of the ticket's fifteen candidates, six
   join two or more rules and are split (the first into A and B, the third into
   F, G and H, the fourth into J and K, the fifth into N, O, P and Q, the
   seventh into S, T and U, the thirteenth into AE and AF); four are loose as
   worded (the second's "once it can see into it", the third's objects "for
   the squares it sees", the twelfth's "removed", the thirteenth's "reaches
   nobody"); three are the arms rulings (2c) and (3) rewrite (V, W, AA); and
   the rest of D5's rows come from the ticket's problem paragraph, from the
   comments the sweep cuts, and from the seat arc's refusals left to this
   record.
4. **Scope matches the claim — by command, then a reading.** No code line
   changes, so callers do not widen the work: `Project` is called by
   `seat.receive` alone, `reperch` by `seat.perch` alone, and `look` by
   `Project`, `reperch` and `canSee` (`grep -rn '\.Project(\|\.reperch(\|\.look('`
   over non-test files under `internal/` and `cmd/`); `Viewer`, `Projector` and
   `NewProjector` are used outside the package only by the gateway's own
   external tests (`project_test.go`, `project_property_test.go`,
   `keystone_test.go`).
   What widens the work is three things the ticket's file list omits:
   `tools/mutation-equivalents.txt` holds three keys in `project.go`, which
   every deletion above them moves (D14); `internal/engine/apply.go` says
   "internal/gateway/project.go quotes this format string", a quotation the
   sweep cuts (D17); and `project_test.go`'s
   `TestNarrationReachesAPlayerAndANoteDoesNot` says "See project.go for the
   full reasoning" (D12). The full list of pointers into the cut blocks is under
   Measurements.
5. **No recorded decision is contradicted — a reading.** SPEC-007 (the
   payloads), SPEC-011 (delivery: one producer, the order the projector's
   memory changed in, a projected batch whose order is load-bearing), SPEC-013
   (the move gate's `canSee`) and SPEC-015 (which seats are projected, the
   state a seat hands `Project`, `pastResume`, how a perch is applied,
   `perchSequence` 0, `Project`'s DM and agent arm, `eyes`' second refusal)
   were read against the ticket and the code, and every sentence in them about
   the projection is true of the code at `db1f77d`. SPEC-016 overturns none;
   it points at each. The visibility ticket's §4.1 says `client/src/wire.ts`
   "reconnects with `after=<lastSeq>`", which is no longer true (it reconnects
   at `seenSeq - 1`); the ticket is history and is not edited, and SPEC-016
   does not repeat it. `docs/adr/` holds no sentence about the projection's
   arms.
6. **The records the work moves are named — by command, then a reading.** The
   section reads `New: the projection ...` with SPEC-013 and SPEC-015 beside it;
   both paths resolve. The reading: SPEC-013 has one sentence that says the
   projection has no record (the move gate's last), SPEC-015 has one (in "What
   this record does not decide") and five more that send a projection rule to
   `project.go` (D13, Q7). SPEC-012's "what a seat SEES of the campaign is the
   projection's, not this record's" stays true and is not edited. SPEC-011 and
   SPEC-007 stay true. `docs/verification-debt.md` holds no entry about
   `project.go`'s arms; the seat arc's two perch entries stay.

## Measurements this plan stands on

All at `db1f77d`, by command, run by the verifier. `python3
tools/check-comments.py --report`, the checker's own `measure`
(`cite = cc.cite_pattern(cc.register_tag()); measure(lines, False, cite)`),
and `tools/comment-ceilings.txt`:

| File | Comment / non-blank | Share | Ceiling | Banned | Blocks > 6 | Cites |
|---|---|---|---|---|---|---|
| `internal/gateway/project.go` | 723 / 1173 | 61.6 | 61.7 | 64 | 30 | 0 |
| `internal/gateway/project_test.go` | 860 / 2567 | 33.5 | 33.6 | 75 | 44 | 0 |
| `internal/gateway/project_internal_test.go` | 16 / 48 | 33.3 | 33.4 | 2 | 1 | 0 |
| `internal/gateway/project_property_test.go` | 122 / 340 | 35.9 | 35.9 | 7 | 6 | 0 |
| `internal/gateway/keystone_test.go` | 548 / 1182 | 46.4 | 46.4 | 34 | 18 | 0 |
| `internal/gateway/server_visibility_test.go` | 317 / 1080 | 29.4 | 29.4 | 21 | 19 | 14 |
| `internal/gateway/viewpoint_internal_test.go` | 98 / 345 | 28.4 | 28.5 | 8 | 4 | 3 |

The ticket's figures for `project.go` are these. Token counts, D10's
instrument (comments dropped): `project.go` 3502, `project_test.go` 16828,
`project_internal_test.go` 260, `project_property_test.go` 1600,
`keystone_test.go` 4593, `server_visibility_test.go` 6280,
`viewpoint_internal_test.go` 2192, `seat.go` 824, `server.go` 4596,
`internal/sight/sight.go` 1112.

**`project.go`, by block.** 73 blocks, each named in D7's table by the symbol
it sits on or in, with the line count `measure` gives and its banned lines. Of
the 30 over the bound, the largest are `transitions`' doc (57 lines),
the block above `transitions`' forgetting loop (52), `Projector`'s doc (51),
the already-held divider in `classify` (38), `ActorRemoved`'s arm (31),
`doorTransitions`' doc (31), `reperch`'s doc (29) and `perchSequence`'s doc
(28). Five trailing comments are code lines to `measure` and are swept by the
reading (D7): on `Viewer.Viewpoint`, on `Project`'s `return nil` after the
nil-state check, and on `sightView`'s three fields.

**Every sentence of the ticket's problem paragraph, against its symbol.** True
as written: `Project`'s three answers; `eyes`' two; `look`'s squares, tokens
and actors (those tokens' actors and every party member); the projector's five
maps; that `transitions` introduces a scene by name and size, corrects doors,
introduces actors with their controllers as grants and their conditions, hides
and places tokens and sends a `SceneSeen` of the visible squares with their
tiles and objects, in that order; the seen-subject arms, the knows arms and
the already-holds arms; `perchSequence` 0 and every other frame carrying its
event's sequence; the comment figures; the three stale or flagged passages.
Two false clauses: "withheld (scene, actor and token creation **and removal**
...)" — `ActorRemoved` is forwarded to a viewer that held the actor (the same
paragraph lists it again under "already holds"), and no payload removes a
scene; "a payload it has never heard of is sent to **nobody**" — the DM and
the agent are sent it unchanged, since `Project` answers their roles before
`classify` runs. Compressions: a scene is introduced by its id as well as its
name and size; before anything is introduced `transitions` forgets each actor
the world has lost; a `SceneSeen` goes out only when the visible set differs
from the one last sent, and an empty one once when a scene goes dark; an object
is sent when any square of its footprint is visible; `transitions`' doc cites
`client/src/wire.ts`, `server.go` and `client/src/session.ts` by line, four
citations in all.

**The torn-batch hazard, established resolved.** `transitions`' doc says a
connection dropped after two of a batch of five has no expressible resume
point. The client resumes one sequence below the highest it saw
(`client/src/wire.ts`'s `reconnect`: `seenSeq - 1`) and rolls that sequence
out of its log first (`Session.rollback` in `client/src/session.ts`); a
projected seat re-projects from 0 and `pastResume` keeps frames strictly above
the cursor (SPEC-015), so the torn sequence arrives again, whole.
`client/test/session.test.ts`'s "a reconnect after a torn batch takes that
sequence again, and folds it exactly once" pins it. SPEC-016 states that one
event's frames share one sequence and points at SPEC-015 for the cursor; it
does not restate the hazard.

**Pointers into the blocks this sweep cuts, from outside them.** Searched over
`internal/`, `cmd/` and `client/src` for every comment and string naming
`project.go`, a symbol of it followed by "comment", "doc" or "'s own", or
"see Project":

1. `project_test.go` (listed by the ticket). The string literal in
   `TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed`, "Projector's doc
   comment is wrong about why the log must be replayed from the beginning",
   and that test's doc and `runLogStepsResumingAt`'s ("the construction
   Projector's doc comment forbids") point at `Projector`'s doc: D7 keeps the
   feed-from-the-start warning there, with its reason, so all three stay true
   and no token changes. `aWholeFight`'s step-20 block ("project.go's claim
   that it must be cloned") points at the clone block in `transitions`: D7
   keeps a clone warning. `TestNarrationReachesAPlayerAndANoteDoesNot`'s body
   block (5 lines, 1 banned) says "See project.go for the full reasoning": D12
   re-aims that one line. `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh`'s
   doc quotes `transitions`' comment ("nothing removes an actor from the world
   today either ...") in words that are not in the comment at `db1f77d`; it is
   stale already and is left (D17). `TestAnObjectIsRevealedOnlyByTheSquaresItStandsOn`
   cites `project.go:510:54` and `511:54`, coordinates that are stale already
   (the lines are 760 and 761); left (D17).
2. `viewpoint_internal_test.go` (listed): `TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen`'s
   doc (11 lines, over) says "it is the claim reperch's own comment makes: no
   door is SKIPPED on the perch path". D7 keeps that sense in `reperch`'s doc,
   so the block is not touched.
3. `internal/gateway/server_internal_test.go` (not listed): "project.go's
   order within them is load-bearing — an actor before its token, a scene
   before what stands in it". D7 keeps the order warning on `transitions`.
4. `keystone_test.go` (listed): "classify, not transitions, not sceneSeenFor,
   not Projector's maps" names symbols whose code survives; the seat arc's
   stale "seat.go's own doc comment says so" stays stale (D17).
5. `internal/engine/apply.go` (not listed): `ErrSceneExists`' doc says
   "internal/gateway/project.go quotes this format string" and that giving the
   sentinel the full text "would have ... made all four quotations false".
   `Projector`'s doc holds the quotation today; D7 cuts it. Left, named (D17,
   Q8).
6. `tools/mutation-equivalents.txt` (not listed): the `project.go:760:26`
   entry says "objectInSight's doc comment says so" of the clamp being a
   performance guard against an enormous or overflowing footprint. D7 keeps a
   clamp warning on `objectInSight` that says so.

Pointers that name a symbol or a file whose code survives, not re-aimed:
`client/src/wire.ts`, `client/src/fold.ts` ("project.go's classify"),
`client/src/app.ts` ("project.go's perchSequence", "project.go's reperch"),
`client/src/view/spectator.ts` ("project.go's look() walks st.Actors flat"),
`internal/mapdef/compile.go` ("internal/gateway/project.go's per-viewer scene
introduction"), `project_internal_test.go` and `project_property_test.go`
(which name `classify`'s arms), `seat.go`'s and `server.go`'s blocks (which
name `Projector`, `perch` and `receive`, and SPEC-015 and SPEC-011).

**Adjudication keys.** `tools/mutation-equivalents.txt` holds three keys in
`project.go`: `project.go:760:26 CONDITIONALS_BOUNDARY` and `761:27
CONDITIONALS_BOUNDARY` (`objectInSight`'s grid clamp) and `1219:38
ARITHMETIC_BASE` (`sortedSceneIDsUnion`'s map hint). Their bodies cite current
coordinates too: the `760:26` entry names its siblings at `760:54` and
`760:66`, the `761:27` entry names `761:54`, and the `1219:38` entry names the
slice hint at `:1220` and `1220:33`; every other coordinate in those bodies
narrates an earlier re-point. `tools/ts-mutation-equivalents.txt` names
`project.go` nowhere. `python3 tools/check_mutation_test.py -q` (107 tests) and
`python3 tools/check_ts_mutation_test.py -q` print `OK` at the base. The first
self-test reads every key against the tree (its
`suspect_positions` case): with D7's draft in place it fails, naming all three
keys ("there is no such line in the tree"), and passes again with the keys at
the draft's `365:26`, `366:27` and `606:38` (dry run below). So the keys move,
and D14 re-points them last.

**Records and gates.** `docs/specifications/` holds 007 to 015; 016 is free.
`docs/requirements.md` has 193 rows, the last `VTT-193`. `python3
tools/check-comments.py main` ends `239 files, 0 added comment lines, 239
ledger rows; clean`, after the standing notice on `cmd/vtt/library_test.go`.
`python3 tools/check-doc-owner.py .` ends `79 files, every doc comment sits on
its own function`. `gofmt -l internal/gateway/` prints only
`scenario_test.go`. `project.go` holds no `nolint`, `#nosec`, `//go:`,
`doc-owner:ok` or `wrap:ok` line. `requirement-id` is at
`~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`.

**What no test observes, by probe over the whole closure.** Each edit below
compiles; with it, `go test -count=1 ./internal/gateway/... ./cmd/vtt/...`
gives:

- `classify`'s `AttackRolled` arm made `return withheld` (an attack naming
  only actors the viewer knows withheld): green.
- `classify`'s `ConditionApplied` arm made `passIf(knows(...))` (the
  already-held test loosened to knows): green, and it cannot differ in
  production: a projector is fed from the log's first event (SPEC-015), so
  every actor `now.actors` holds and `pr.actors` does not became knowable on
  this event, and a `ConditionApplied` or `ResourceChanged` moves no token,
  door, controller or kind.
- `sameSet`'s content test made `!b[k] && false` (a visible set that changes
  to another of the same size counts as unchanged, so no `SceneSeen` is sent):
  green. Row H2.
- `classify`'s `TokenHidden`/`SceneSeen` arm made `return forwarded`: green.
  No command produces either (`grep -rn 'Envelope_TokenHidden{\|Envelope_SceneSeen{'`
  over non-test Go under `internal/` and `cmd/`, generated code aside, prints
  only `project.go`), so no log holds one.
- `reperch`'s nil-world guard alone disabled: green (`look` guards nil and
  `transitions` reads the state only inside loops a nil look never enters);
  with `look`'s guard disabled as well, `TestASeatPerchesOnlyAgainstAWorldItHasSeen`
  reds.

And these are observed only by `keystone_test.go`'s
`TestTheProjectedGoldensAreWhatTheProjectionActuallySends`, a comparison of
committed stream bytes, in the gateway package: the `ResourceChanged` arm made
`return withheld`; the `ActorControlGranted` and `ActorControlRevoked` arms
made `return withheld` (gateway runs); and, over the whole closure, the session
arm made `return withheld` and the `AdventureLoaded` arm made `return
forwarded`. That test compares each projected golden's stream with the
committed `stream.json`, so any change reds it, and regenerating the golden
greens it again (Q10).

Defence that the wire cannot reach, each red only in its own unit tests: a
player's `Viewpoint` honoured by `eyes` (red in
`TestAPlayerCannotBorrowAnNpcsEyesByPerching` alone; `viewerFor` leaves the
viewpoint empty and `MayPerch` refuses a player, SPEC-015); `eyes`' party test
dropped for a spectator (red in `TestAPerchOnAnNpcYieldsNoSightAtAll`,
`TestASpectatorGetsNoSightFromAnNPCTheDMControls` and the keystone's
`spectator-on-npc` seats; `MayPerch` refuses the perch first, VTT-181);
`Project`'s nil-state guard removed, or its unknown-role arm let through (red
in `TestTheProjectionFailsClosedWhenItHasNothingToGoOn` alone; `receive` hands
`Project` a folded state and `identity.Verify` refuses an unparseable role,
SPEC-009); `Project`'s DM arm removed for the DM (red in
`TestTheDMReceivesEverythingUnchanged`, the keystone's identity assertion and
the property test's DM seat; the DM's seat has no `Projector`, SPEC-015); the
cmd/vtt run of that last probe also redded
`TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessIgnoresStdio`, which no
projection reaches (gap 9).

**A dry run of D7, in a scratch clone and never in the tree.** With D7's texts
written into `project.go` by a script that replaces each block by its starting
line at `db1f77d` (`sweep.py` in the verifier's scratchpad), then `gofmt -w`:
D10 prints `same` at 3502; the file goes from 1244 lines to 631, and `measure`
gives 115 comment lines of 565 non-blank (20.35 percent), 54 blocks, `banned
0`, `blocks>6 0`; `gofmt -l` prints nothing for it; `go vet` is clean;
`golangci-lint run ./internal/gateway/` prints `0 issues`, after one fix the
first run found: deleting the divider comment at the top of `classify`'s
switch left a blank line after `switch ... {`, which the `whitespace` linter
refuses ("unnecessary leading newline"), so a deleted block directly after an
opening brace takes the blank line below it too (whitespace, no token);
`python3 tools/check-comment-wrap.py` flags nothing; `python3
tools/check-new-prose.py main` prints `120 added line(s) across 1 file(s), all
clean` on the draft before its last two wording fixes; `python3 tools/check-doc-owner.py .` still ends `79
files ...`; `go doc` prints `Viewer`'s, `Projector`'s and `Project`'s
sentences; `python3 tools/check-comments.py main` refuses exactly one thing,
`project.go` fallen more than the band under its ceiling; then
`--write-ledger` changes exactly one row (`project.go` 61.7 to 20.4) and
`check:comments` ends `clean`. Breaks B4 and B7 (D15) were run on that tree
and gave the lines D15 quotes. The clone is scratch; the tree was not touched.

**`check:doc-owner`'s first-word rule, re-derived at `db1f77d`.** It refuses a
doc block above a function, and a floating block, whose first word names any
function declared outside a test file (576 names over 79 files). The
single-word capitalised names are: `Accepted`, `Actors`, `Append`, `Apply`,
`Arm`, `Authorize`, `Blocked`, `Blockers`, `Clear`, `Clip`, `Close`,
`Compile`, `Dial`, `Error`, `Eval`, `Events`, `Fold`, `Handler`, `List`,
`Load`, `Lookup`, `New`, `Notify`, `Open`, `Parse`, `Project`, `Refs`,
`Resolve`, `Revoke`, `Roll`, `Run`, `Scenes`, `Scopes`, `Snapshot`, `State`,
`Step`, `String`, `Subscribe`, `Tokens`, `Unwrap`, `Validate`, `Verify`. Every
function of `project.go` is on it in its own spelling (`look`, `eyes`,
`transitions`, `classify`, `reperch`, `passIf`, `canSeeSquare`,
`doorTransitions`, `doorSubject`, `squareAt`, `squareKey`, `sortedSet`,
`sortedSceneIDs`, `sortedSceneIDsUnion`, `sameSet`, `sceneSeenFor`,
`objectInSight`), and so are `receive`, `perch`, `set` and `take`. So a block
above a function opens with that function's own name or with a word on no
list; `Clear`, `Project`, `State`, `Apply`, `Fold`, `Tokens`, `Actors` and
`Scenes` never open a warning. `Keep`, `Never`, `Forget`, `Send`, `Withhold`,
`Forward`, `Sort`, `Skip`, `Pass`, `Clone`, `Require`, `Walk`, `Create`,
`Know`, `Ignore`, `Refuse`, `Read`, `Correct`, `Introduce`, `Emit`, `Return`,
`Add`, `Call` and `Compare` name no function at `db1f77d`. The floating block
above `sortedSet` is read by the same rule. A comment inside a body, on a field
or above a type or constant is not read by that gate.

## Constraints that bind every task

- `CLAUDE.md` rule 10 and SPEC-010: a comment is an imperative warning, a
  pointer, or the one-line doc sentence of an exported symbol (here `Viewer`,
  `Projector` and `Project`; `NewProjector` has no doc and gains none). VTT-051
  is held by the Phase 4b reading; VTT-050, VTT-052, VTT-053 and VTT-055 by
  `check:comments`; VTT-059 says a citation line moves no share.
- `CLAUDE.md` rule 2: no gate is weakened. The ledger only goes down, only
  through `--write-ledger`. Mutation keys are re-pointed to where their
  mutants now are, never deleted to pass (D14).
- `CLAUDE.md` rule 8, narrowed by rule 10 for code: a comment points at a
  specification by number, a requirement by id, a test or symbol by name, or a
  report by its `docs/` path; never a line number, a date, a commit hash, a
  plan, a task, `§`, an ADR, `CLAUDE.md` or a `.superpowers/` path. This plan
  and the report name blocks by the symbol they sit on. The adjudication files
  keep writing coordinates, as rule 8 says.
- `CLAUDE.md` rule 3: the contract is not touched; the payloads are SPEC-007's.
- `CLAUDE.md` rule 4: one fold. SPEC-016 says `look` and `transitions` read
  the state and never write it, and names `engine.Apply` and
  `client/src/fold.ts` for what a fold refuses.
- `CLAUDE.md` rule 5: SPEC-016 names no rules concept; sight range and
  tolerance are inputs `internal/sight` receives, and a party member is an
  actor kind, `engine.IsPartyMember`'s.
- `CLAUDE.md` rule 9: the answer above is recorded before any task runs.
- SPEC-008: ids come from `requirement-id`, after sign-off, never by hand.
- The `specification` skill's form and `catches.md`: five headings, present
  tense, no `Why`, no `Rejected`, no measurement, no line number, no past-tense
  account that is not about the code, no path outside the project, one
  decision per file, no claim taken from a comment without reading the code
  under it (item 8: two comments in this file are already false — `transitions`'
  `UNRESOLVED` and the ticket's inherited "withheld ... removal"), every "only,
  never, every, none" with its search named (item 10), the old text not left
  beside the new (item 13, for the SPEC-013 and SPEC-015 edits).
- The `requirements` skill: one thing, what not how, breakable, measurable,
  named by a check or knowingly OPEN; refuse more than you accept, and report
  every refusal in one line.
- SPEC-007, SPEC-009, SPEC-011 and SPEC-012 are not edited; SPEC-013 and
  SPEC-015 only at the sentences D13 names.
- The ticket: no code line changes (D10 is the check). Files: SPEC-016,
  `project.go`'s comments, citation lines in the test files D6 names, the one
  re-aimed line D12 names, SPEC-013's and SPEC-015's sentences, the register,
  the ledger, and the three mutation keys (D14, which the ticket's list omits;
  Q9).
- `internal/sight`, `internal/engine`, `internal/campaign`, `cmd/vtt`,
  `client/src`, `contract/`, `seat.go`, `server.go`, `viewpoint.go`, every other
  file under `internal/gateway` and the visibility ticket and report are not
  touched. `docs/reports/` gains only this ticket's report.

## Decisions this plan makes

**D1. SPEC-016's sections, each lifted from a named source and held by named
symbols.** Forced by ticket item 1 and the `specification` skill's step 3.
File `docs/specifications/016-the-projection.md` (the ticket's name). Title:
`SPEC-016: What a player or a spectator is sent of each event is decided by
the projection`. Under "How it works", in this order, one bold-led paragraph
each:

| Section | Source (where it lives today) | Symbols that hold it | Rows (D5) |
|---|---|---|---|
| Which viewers are projected, and what `Project` answers | `Project`'s doc and its role arms; the unrecognised check; visibility §3.1, §4.4 | `Project` answers a nil event with nothing; `identity.RoleDM` and `identity.RoleAgent` with the event itself, the same pointer, reading no state; any role but player and spectator with nothing; a nil state with nothing; otherwise it asks `look`, then `classify`, and for an `unrecognised` verdict sends nothing at all, `transitions` included; for `withheld` it sends `transitions`' frames; for `forwarded` those frames and then the event. Which seats call `Project` (`seat.receive` alone, `grep -rn '\.Project('` over non-test files) and with which state is SPEC-015's, pointed at | AE (AN, AO refused) |
| Whose eyes a viewer has | `eyes`' arms; `Viewer`'s doc; visibility §3.1, §3.1.1 | `eyes` gives a player every actor in `st.Actors` whose `ControllerIds` holds the player's participant id, sorted, and ignores `Viewpoint`; a spectator the actor its `Viewpoint` names when `st.Actors` holds it and `engine.IsPartyMember` accepts it, and otherwise none, the empty viewpoint included; `MayPerch` refuses a non-party perch first (SPEC-015) | A, B (C refused) |
| What a look sees | `look`'s doc and body blocks; the range and tolerance constants; `sightView`'s doc; visibility §3.2, §3.4, §5 | `look` walks each eye's tokens in scenes `st` holds; each such scene gets an entry whether or not anything is visible from there; its squares are the union of `sight.VisibleFrom(scene, x, y, sightRangeNotSupplied, toleranceNotSupplied)`, both 0, which `internal/sight` reads as unlimited range and one of nine points; its tokens are every token of `st` on a square in its scene's entry; its actors are those tokens' actors and every actor `engine.IsPartyMember` accepts. `look` runs on every call, keeps no memo, and writes nothing to `st` | D, J, N, O |
| What the projector remembers | `Projector`'s doc and field docs; `NewProjector` | five maps, empty from `NewProjector`: `scenes` introduced, never removed (`grep -n 'delete(pr.scenes' internal/gateway/project.go` prints nothing); `actors` introduced, removed only by `transitions`' forgetting loop; `tokens` on the board now; `seen` the visible set last sent per scene, dropped after a dark report; `doors` the squares believed open per scene. The memory depends on the whole prefix fed (an actor seen and then hidden stays known), so a projector is fed the log from its first event and never seeded from a state; the seat does so (SPEC-015) | R, AJ |
| What `transitions` sends, in order | `transitions`' doc and its body blocks; `sceneSeenFor`'s, `objectInSight`'s and the sorters' blocks; visibility §4.2, §5, §5.1 | first, each actor `pr.actors` holds and `st` lacks is forgotten, and nothing is sent; then, by id: each scene in the look not yet introduced, as a `SceneCreated` of its id, name, width and height with no tile and no object; `doorTransitions`' frames; each actor in the look not yet introduced, as an `ActorAdded` of a clone with `ControllerId` and `ControllerIds` cleared, one `ActorControlGranted` per controller in the actor's order carrying the actor's kind, and one `ConditionApplied` per condition `st` holds for it; a `TokenHidden` for each token on the board and not in the look; a `TokenPlaced` (scene, actor, position) for each token in the look and not on the board; and a `SceneSeen` for each scene in `seen` or in the look whose visible set differs from the one last sent (`sameSet`), built by `sceneSeenFor` with the visible squares sorted, the tile of each visible square that has one, and each object any square of whose footprint is visible, none with a width or height below one; a scene with no square in the look gets an empty one and leaves `seen`. Each scene precedes the doors, tokens and `SceneSeen` in it and each actor its grants, conditions and tokens, the order both folds require. Every walk that emits is sorted, so one log projects to one stream | E, F, G, H1, H2, I, K, P, Q, AI, AJ |
| Doors | `doorTransitions`' doc and body blocks; `doorSubject`'s and `squareAt`'s docs; the `DoorClosed` arm | for each scene in the look and each visible square by key, where `st`'s `OpenDoors` differs from the belief, `doorTransitions` sends a `DoorOpened` or `DoorClosed` there and records it; the square of the door event being projected is recorded without a frame, since `classify` forwards that event; a perch has no cause (`doorSubject` reports none for nil), so every such square is sent; a key `squareAt` cannot parse is skipped; a door out of sight keeps the viewer's belief. `classify` forwards `DoorOpened` and `DoorClosed` only when `canSeeSquare` finds the square in the look; a missing position is never in sight | S, T, U |
| How each payload is ruled | `classify`'s doc, its arms' blocks, `verdict`'s and `passIf`'s | `classify` runs before `transitions`, so the board and roster it reads are the viewer's before the event. Forwarded: `SessionStarted`, `SessionEnded`, `NarrationAdded`. Withheld: `SceneCreated`, `ActorAdded`, `TokenPlaced` (the projection introduces these), `TokenRemoved` (its `TokenHidden` comes from `transitions`), `TokenHidden` and `SceneSeen` (only the projection issues them), `NoteUpserted`, `NoteDeleted`, `AdventureLoaded`. `TokenMoved`: forwarded when the token was on the board before and is in the look after. `DoorOpened`, `DoorClosed`: above. `AttackRolled` (attacker, target), `AbilityUsed` (actor, targets), `ActorControlGranted`, `ActorControlRevoked`: forwarded when every non-empty actor id named is held or is in the look (`knows`). `ResourceChanged`, `ConditionApplied`, `ConditionRemoved`, `ActorRemoved`: forwarded when the actor was held before the event. Any other payload, no payload included, is `unrecognised`; `TestEveryEnvelopePayloadArmHasAnExplicitRuling` walks the oneof | L, M, S, X, Y, Z, AB, AC, AE (V, W, AA held out; AD, AF refused) |
| A perch | `reperch`'s doc and nil block; visibility §3.1.1 | `reperch` sets `Viewpoint`, answers a nil state with nothing, and otherwise returns `transitions` with no cause and `perchSequence` against a fresh look: what the new eyes see that the memory lacks is introduced, what they do not see is hidden, a scene no eye stands in goes dark, and no terrain is withdrawn, since no frame withdraws it. When and against which state a perch runs is SPEC-015's | AL, T, I, K |
| The sequence every frame carries | `transitions`' doc; `perchSequence`'s doc | `Project` hands `transitions` the event's sequence and every frame it builds carries it, as the forwarded event does; `reperch` hands it `perchSequence`, 0 (SPEC-015, VTT-186). So one event's frames share one sequence, and SPEC-015 states what a resume cursor does with it | AG |
| What this record does not decide | the ticket's "What could not be established" | which seats are projected, the state a seat hands `Project`, what its cursor drops, when a perch is taken and applied, and `canSee` (SPEC-015); what a square can see — walls, closed doors, objects that block sight, range, tolerance — which is `internal/sight`'s and has no record; the payloads' fields (SPEC-007); what a party member is (`engine.IsPartyMember`); what each fold refuses (`engine.Apply`, `client/src/fold.ts`); delivery and its one producer (SPEC-011); the move gate (SPEC-013) | none |

Each sentence names the symbol that holds it, is checked against the code
before it is written and re-read by Phase 4b (D16). "Principles served" says
what SPEC-013 to SPEC-015 say: no blueprint; the principle missing from the
record rather than absent from the system (here: a player or a spectator is
sent of each event only what their eyes see or their roster already holds,
decided on the server, and when the projection cannot tell, it sends nothing).
"Consequences" holds what a client author and whoever changes the code are
bound by: a projected stream folds in the order it is sent, and several of its
frames share one sequence; a viewer learns of a scene, an actor or a token only
from the projection's introductions; a scene once introduced is never
withdrawn, and its terrain arrives only as seen; the newest `SceneSeen` of a
scene is its visible set and an empty one means dark; a party member is on
every projected roster, seen or not; a condition an introduction carries bears
the introduction's sequence, not the one it was applied at; a viewer present at
a grant that introduces an actor receives the grant twice, which both folds
accept; a player's and a spectator's notes panel is empty; a payload added to
the contract needs an arm in `classify`, or no player or spectator is sent it
and its event derives nothing for them; `Project` writes to neither the event
nor the state, and a live event is one envelope shared by every seat; sight
range and tolerance are not supplied. "Requirements" lists the ids the sort
dispenses (D6) and nothing else.

**D2. What SPEC-016 points at and does not restate.** Forced by `catches.md`
item 13 and the ticket's item 1. SPEC-015 owns `projected`, `newSeat`,
`receive` and the state it folds, `pastResume`, `perch`, `perchBox`, `canSee`
and `catchUp`, and the value 0 of a perch's sequence and what it does to a
cursor; SPEC-016 says "SPEC-015" at each. `internal/sight` owns
`VisibleFrom`'s rules and has no record; SPEC-016 names the call and its two
inputs and says so. SPEC-007 owns the payloads. SPEC-011 owns delivery.
SPEC-013 owns `canSee`'s use in the move gate and who may issue each command
(who may add narration or a note). `internal/engine` owns what a party member
is and what `engine.Apply` refuses; `client/src/fold.ts` its mirror.

**D3. SPEC-016's Status, and how it carries rulings (2c) and (3).** Forced by
the `specification` skill's step 2 and `catches.md` items 5 to 7, and by the
brief. The record's own decision is today's projection, which the code
implements, so its first paragraph is the skill's first form. The rulings are
decisions the code has not caught up with and this record does not yet state;
writing them as its decision would make it "partly implemented" with no ticket
for the rest, which item 7 calls a wish. So Status says they exist, which
sentences they will change, and that tickets not yet written carry them,
without stating them as the record's own. Proposed:

    Accepted. Implemented by `internal/gateway/project.go` (`Viewer`,
    `Projector`, `NewProjector`, `Project`, `perchSequence`, `reperch`,
    `sightView`, `look`, `eyes`, `transitions`, `sceneSeenFor`,
    `objectInSight`, `verdict`, `passIf`, `classify`, `canSeeSquare`,
    `doorTransitions`, `doorSubject`, `squareAt`, `squareKey`, `sortedSet`,
    `sortedSceneIDs`, `sortedSceneIDsUnion`, `sameSet`), over
    `internal/sight`'s `VisibleFrom` and `engine.IsPartyMember`; called from
    `internal/gateway/seat.go` (`receive`, `perch`, `canSee`); pinned by
    `internal/gateway/project_test.go`, `project_property_test.go`,
    `keystone_test.go`, `viewpoint_internal_test.go` and
    `server_visibility_test.go`.

    Two decisions the owner has taken will change this record, and neither is
    implemented. A note will carry a visibility flag, public or DM-only; that
    changes the notes ruling under "How each payload is ruled" and the
    Consequence that a player's notes panel is empty. Only what a viewer sees
    will give them information, for every actor, party members included; that
    changes the rulings for a payload forwarded when the viewer knows every
    actor it names or already holds the actor, and what a party member's
    introduction carries while no eye sees it. Each is carried by a ticket of
    its own, the notes flag first and the testimony rule second, and neither
    ticket is written yet; both decisions are recorded in
    `docs/superpowers/specs/2026-09-29-the-projection-has-a-record-design.md`.
    Until each lands, every sentence below describes the code, and the ticket
    that lands it rewrites this paragraph and the sentences it names.

The test-file list carries only the files that gain citation lines (D6).
Q1 asks whether the two tickets should exist before this commit so Status can
name them by path. Provenance stays out of the record otherwise: SPEC-016
names no other project, no ticket section, no quotation and no date outside a
`docs/` path (`catches.md` items 3 and 12).

**D4. The rule-9 answer lives here and in the report.** Forced by rule 9 and
D3. SPEC-016 states the server-side introductions, the per-viewer `SceneSeen`
and the scene reached only when an eye stands in it as facts of this system.

**D5. The sort's starting point is the table below; nothing is an id until
sign-off.** Forced by SPEC-008 and the `requirements` skill. Rows are lettered
so nothing here reads as an id. Each line: the rule as a what; the outcome
proposed; the evidence, verified against the test bodies; the one edit that
reds it. Every edit was run by the verifier in a scratch clone of `db1f77d`
(`go vet` clean; the tests named went red with the message quoted; the file
restored after each). "Closure" means `go test -count=1
./internal/gateway/... ./cmd/vtt/...`; a count is of top-level tests red in the
gateway package.

| # | Rule (what) | Proposed | Evidence (verified against the body) | The edit that reds it (run) |
|---|---|---|---|---|
| A | A player sees through every actor whose controllers include them, and through no other actor. | accept — the first candidate's player half | `TestSceneSeenCarriesOnlyTheSquaresInSight` (the hero's room is described), `TestAnNPCHeldByTheDMIsNotPublishedToThePartysRoster` (a goblin the DM holds lends no eyes, so it stays off the roster) | in `eyes`, `if c == pr.viewer.ParticipantID {` made `if c != "" {` (7 red: `a monster the DM happens to hold is still a monster`); `return ids` made `return ids[:0]` (39 red, the first among them) |
| B | A spectator sees through the party member it perches on. | accept — the first candidate's spectator half | `TestASpectatorRidesTheShoulderTheyPerchOn`, and on the wire `TestASpectatorHopsFromOneShoulderToAnother` (VTT-180, VTT-192) | the spectator arm's `return []string{pr.viewer.Viewpoint}` made `return nil` (11 red: `a spectator must see what the shoulder they ride sees`) |
| C | No viewer sees through anyone else: a player's viewpoint lends no eyes; a spectator on a non-party actor sees nothing. | refuse — defence behind SPEC-015: `viewerFor` leaves a player's viewpoint empty and `MayPerch` refuses a player and a non-party shoulder (VTT-181). The player half reds `TestAPlayerCannotBorrowAnNpcsEyesByPerching` alone; the spectator half reds `TestAPerchOnAnNpcYieldsNoSightAtAll`, `TestASpectatorGetsNoSightFromAnNPCTheDMControls` and the keystone's `spectator-on-npc` seats, the closure otherwise green. SPEC-016 states both (Q4) | as named | `eyes`' player arm appending `Viewpoint`; the spectator test made `!ok \|\| a == nil` |
| D | A viewer is introduced to a scene when a token of one of its eyes stands in it, and to no other scene. | accept — the second candidate, reworded: "once it can see into it" is loose, since `look` gives a scene an entry before asking sight | `TestAPlayerLearnsOnlyTheSceneTheirActorStandsIn`, `TestASeatWithNoActorIsToldOfNoSceneAtAll` | in `look`, every scene of `st` given an entry before the eyes loop (9 red: `a player must be able to enumerate exactly the scene they stand in`) |
| E | A scene is introduced by its id, name and size, with no tile and no object. | accept — the second candidate's other half | `TestAnIntroducedSceneCarriesTheOutlineButNoTerrain` | the introduction given `Tiles` built from `sc.Tiles` (4 red: `an introduced scene must carry NO tiles, got 21`) |
| F | A viewer's `SceneSeen` of a scene carries exactly the squares it sees there now, and the tiles of those squares. | accept — the third candidate's first half | `TestSceneSeenCarriesOnlyTheSquaresInSight`, `TestSceneSeenCarriesTheVisibleSquaresEvenWithNoTerrain` | `sceneSeenFor` walking `sc.Tiles` instead of `squares` (9 red: `the far room is behind a closed door and must not be in the visible set`); `Visible: sortedSet(squares)` made `Visible: nil` (6 red: `every square of a bare 3x3 is visible, got 0`) |
| G | An object is sent in a `SceneSeen` when a square of its footprint is visible, and not otherwise. | accept — the third candidate, reworded: objects are not sent "for the squares", they are sent whole | `TestAnObjectIsRevealedOnlyByTheSquaresItStandsOn`, `TestSightBlockingSceneryHidesWhatStandsBehindIt` | `objectInSight`'s first test made `if o.Width >= 0 { return true }` (both red: `got [pillar crate ghost]`) |
| H1 | A viewer is sent a scene's `SceneSeen` when the squares it sees there change, and not when they do not. | accept — the third candidate's "told when that set changes" | `TestADoorOpenedOutOfSightArrivesWhenTheSquareComesIntoView` (`moving must re-report what the viewer can see`), `TestARemovalBatchProjectsToTheBytesBothFoldsRead` (no `SceneSeen` for an unchanged set) | `if sameSet(pr.seen[id], lit) {` made `... \|\| pr.seen[id] != nil {` (6 red); made `... && false {` (4 red: `the seat received 12 envelopes, the fixture holds 9`) |
| H2 | A viewer is sent a scene's `SceneSeen` when the squares it sees there change to as many other squares. | OPEN — no test yet, Q5. The closure is green with `sameSet` comparing sizes alone: a viewer whose visible set of a scene becomes another of the same size is sent nothing, and its client keeps the old lit area | none | `sameSet`'s `if !b[k] {` made `if !b[k] && false {` (closure green) |
| I | A scene a viewer can no longer see anything of is reported dark once, with an empty `SceneSeen`. | accept — from `transitions`' union walk | `TestASceneThatLeavesSightEntirelyIsReportedDark`, `TestASceneAlreadyReportedDarkIsNotReportedDarkAgain`, `TestLeavingAShoulderTakesTheCreaturesAndNotTheTerrain` | the walk over `sortedSceneIDsUnion(pr.seen, now.squares)` made `sortedSceneIDs(now.squares)` (3 red: `must be reported dark`); `delete(pr.seen, id)` removed (1 red: `a scene already reported dark must stay silent`) |
| J | A token is placed on a viewer's board only while it stands on a square the viewer sees. | accept — the fourth candidate's first half | `TestAPlayerNeverReceivesATokenBehindAClosedDoor`, `TestSightBlockingSceneryHidesWhatStandsBehindIt`, and on the wire `TestSessionZeroCannotHappenAgain` (VTT-177) | in `look`'s token loop, the visibility test made `if true {` (33 red: `the goblin is behind a closed door`) |
| K | A token that stops standing on a square the viewer sees leaves its board as a `TokenHidden`. | accept — the fourth candidate's other half | `TestClosingTheDoorHidesTheGoblinAgain`, `TestLeavingAShoulderTakesTheCreaturesAndNotTheTerrain`, `TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway` (VTT-178) | the `TokenHidden` append removed from the departures loop (14 red: `closing the door must hide the goblin`) |
| L | A token removed from the world reaches a viewer that held it as a `TokenHidden`, never as the removal. | accept — from `classify`'s `TokenRemoved` arm | `TestARemovedTokenReachesAPlayerOnlyAsHidden` | the arm's `return withheld` made `return forwarded` (5 red: `a removed token must reach a player as TokenHidden, never the raw TokenRemoved`) |
| M | A token's move is forwarded only when the token was on the viewer's board before it and stands on a square the viewer sees after it. | accept — the sixth candidate, "saw" made exact | `TestSteppingIntoViewArrivesRatherThanMoves` (before), `TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway` (after), `TestAVisibleTokensMoveDoesReachThePlayer` (both, forwarded) | `pr.tokens[id] && now.tokens[id]` made `now.tokens[id]` (2 red: `a move OUT of the dark names the dark square it started in`); made `pr.tokens[id]` (4 red); made `... && false` (5 red: `a player must watch their own character walk`) |
| N | An actor that is not a party member is introduced to a viewer only once a token of it stands on a square the viewer sees, whoever controls it. | accept — the fifth candidate's first half | `TestAnNPCHeldByTheDMIsNotPublishedToThePartysRoster`, `TestAKindlessGrantConfersControlAndNothingElse`, `TestAnActorWithNoDeclaredKindIsNotAPartyMemberWhoeverHoldsIt`, `TestGrantingAnAgentTheShippedGoblinArcherDoesNotPublishItToThePlayers` | `look`'s roster test made `engine.IsPartyMember(a) \|\| len(a.GetControllerIds()) > 0` (6 red: `must not reach a player's roster`) |
| O | A party member is introduced to every player and spectator, seen or not. | accept — the fifth candidate's second half; Q6 on ruling (3) | `TestAPartyMemberStaysKnownEvenWhenOutOfSight`, `TestAPartyMemberIsKnownEvenWhenHeldByTheDM`, `TestTheShippedHumanFighterIsAPartyMemberBeforeAnybodyIsAssignedToIt`, `TestTheSameShippedArcherAssignedToAPlayerIsAPartyMember` | `look`'s roster test made `engine.IsPartyMember(a) && false` (9 red: `the scout is a party member and must be introduced at all`) |
| P | An actor introduced to a viewer arrives with the controllers it has, in the order it has them. | accept — the fifth candidate's "controllers as grants", worded as the what: the grants are how, since both folds refuse an `ActorAdded` that names a controller | `TestAnIntroductionCarriesNoControllerAndTheGrantsBehindIt` | `clone.ControllerIds = nil` removed (15 red: `the introduction carries controller_ids [p-2 p-3]`); the grants loop over `controllers[:0]` (3 red: `want one grant per controller (2), got 0`) |
| Q | An actor introduced when a token of it comes into sight arrives with the conditions it carries. | accept, worded so ruling (3) leaves it standing — the fifth candidate's last clause; Q6 | `TestAConditionAppliedOutOfSightArrivesWithTheActor` | the conditions loop over `st.Conditions[id][:0]` (3 red: `an introduced actor must arrive with the conditions already on it`) |
| R | An actor id the world uses again after a removal is introduced to a viewer afresh. | accept — from `transitions`' forgetting loop | `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh` | the loop's `!ok` made `!ok && false` (1 red: `want the reused actor id introduced again`) |
| S | A door's opening or closing is forwarded only when its square is in sight. | accept — the seventh candidate's first half | `TestADoorInARoomYouAreNotInStaysSilent` (opened), `TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer` (closed) | the `DoorOpened` arm made `return forwarded` (6 red: `a door in a scene this player has never entered must not reach them`); the `DoorClosed` arm likewise (3 red, the property test's `door closed in unknown scene` among them) |
| T | A door a viewer sees is shown in the state the table has it, on an event and on a perch alike. | accept — the seventh candidate's second half; the seat arc's Y | `TestADoorOpenedOutOfSightArrivesWhenTheSquareComesIntoView`, `TestAnIntroducedSceneArrivesWithItsDoorsAlreadyOpen`, `TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen` | `doorTransitions`' frames dropped (`_ = pr.doorTransitions(...)`) (5 red: `must not be left drawing it shut`) |
| U | A door change a viewer sees reaches it once. | accept — from `doorTransitions`' cause skip | `TestADoorYouCanSeeDoesReachThePlayer`, `TestADoorYouCanSeeSwingShutReachesThePlayerOnce` | the cause test made `... && false` (3 red: `exactly once, got 2`) |
| V | An attack, an ability or a control change is forwarded only when the viewer knows every actor it names. | hold out — ruling (3) rewrites it; its ticket sorts it, Q2. Observed thinly: an attack naming known actors withheld leaves the closure green; a grant or revoke withheld from a knower reds only the goldens' bytes | `TestAnEventNamingAnUnknownActorIsWithheld` (attack, ability) | the `AttackRolled` arm made `return forwarded` (1 red) |
| W | A resource or condition change is forwarded only to a viewer that held the actor before the event. | hold out — ruling (3), Q2. A resource change withheld from a holder reds only the goldens' bytes; held loosened to knows is green and cannot differ in production (Measurements) | `TestAnEventNamingAnUnknownActorIsWithheld`, `TestAConditionAppliedOutOfSightArrivesWithTheActor` | the `ResourceChanged` arm made `return forwarded` (3 red) |
| X | An actor's removal is forwarded only to a viewer that held the actor. | accept — from the ticket's already-holds list; its reason is the fold, not testimony, Q6 | `TestARemovedActorReachesOnlyTheSeatThatHeldIt` | the arm made `return withheld` (4 red: `want the raw ActorRemoved forwarded`); made `return forwarded` (2 red: `must not be announced as removed`) |
| Y | Narration is forwarded to every player and spectator. | accept — the tenth candidate's first half; ruling (1a) | `TestNarrationReachesAPlayerAndANoteDoesNot`, and on the wire `TestASpectatorWithNoPerchReceivesNoBoard` (VTT-177, VTT-180) | the arm made `return withheld` (4 red: `withholding narration from players silences the table's story channel`; `a spectator must still hear the table`) |
| Z | A session's start and end are forwarded to every player and spectator. | accept, Q10 — the tenth candidate's second half. Observed by one test only, a byte comparison of the committed projected streams: the closure is otherwise green with the arm withholding | `TestTheProjectedGoldensAreWhatTheProjectionActuallySends` | the arm made `return withheld` (closure: that test alone, `the projection no longer emits the committed stream`) |
| AA | A note reaches no player or spectator. | hold out — ruling (2c) rewrites it; its ticket sorts it, Q2 | `TestNarrationReachesAPlayerAndANoteDoesNot` | the notes arm made `return forwarded` (2 red) |
| AB | No scene, actor or token the log creates is forwarded to a player or spectator as written. | accept — the twelfth candidate reworded: an actor's removal is forwarded (X), and no payload removes a scene | `TestAProjectedStreamFoldsCleanly` (scene), `TestSessionZeroCannotHappenAgain` (actor), `TestAPlayerNeverReceivesATokenBehindAClosedDoor` (token) | the `ActorAdded` arm made `return forwarded` (11 red: `the goblin reached a player's connection`); `SceneCreated` likewise (10 red: `scene "s" already exists`); `TokenPlaced` likewise (23 red) |
| AC | An adventure load is forwarded to no player or spectator. | accept, Q10 — from the ticket's withheld list. Observed by the same byte comparison alone | `TestTheProjectedGoldensAreWhatTheProjectionActuallySends` | the arm made `return forwarded` (closure: that test alone) |
| AD | A `TokenHidden` or `SceneSeen` found in the log is forwarded to no player or spectator. | refuse — unreachable: no command produces either (the grep under Measurements), and the closure is green with the arm forwarding. SPEC-016 states the arm | none | closure green |
| AE | An event whose payload this build does not know is sent to no player or spectator and derives nothing for them. | accept — the thirteenth candidate's first half, reworded: the DM and the agent are sent it | `TestAnUnrecognisedPayloadIsWithheldFromAPlayer` | the unrecognised check made `v == unrecognised && false` (1 red: `got 5`); the default made `return forwarded` (2 red) |
| AF | Every payload the contract defines has a ruling. | refuse — the thirteenth candidate's second half is a guard on the contract, not a rule a viewer meets. With an arm made to answer `unrecognised`, the closure reds `TestEveryEnvelopePayloadArmHasAnExplicitRuling` alone. SPEC-016 states it as a Consequence | `TestEveryEnvelopePayloadArmHasAnExplicitRuling` | the `AdventureLoaded` arm made to `return unrecognised` first (closure: that test alone, `payload "adventure_loaded" (field 25) has no explicit ruling`) |
| AG | Every frame an event produces carries that event's sequence. | accept — the fifteenth candidate | `TestOpeningTheDoorIntroducesTheGoblinToThePlayer`, `TestARemovalBatchProjectsToTheBytesBothFoldsRead` | `transitions(env, env.GetSequence(), ...)` made `env.GetSequence()+1` (4 red: `a synthesized introduction carries the CAUSING sequence`) |
| AH | Projecting an event changes neither the event nor the state it is judged against. | accept — from `Project`'s doc | `TestProjectingChangesNeitherTheEventNorTheState`, `TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed` (its isolated-state loop) | `clone := proto.Clone(a).(*vttv1.Actor)` made `clone := a` (15 red: `the projection must not touch state`) |
| AI | The same log projects to the same stream for a viewer every time. | accept — from the sorters' block | `TestTheSameLogProjectsTheSameStreamEveryTime`, `TestTheVisibleSetIsSentInAStableOrder` | `sort.Strings(out)` removed from `sortedSet` (both red in 12 of 12 runs, `-count=12`: `the visible set must be emitted sorted`) |
| AJ | Every frame a viewer is sent folds onto the frames sent before it. | accept — from `transitions`' order | `TestAProjectedStreamFoldsCleanly`, `TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer` | the `TokenPlaced` loop moved above the actor introductions (14 red: `token placed for unknown actor "hero"`) |
| AK | Folding a viewer's stream at every prefix gives exactly the world the server says that viewer sees. | accept, Q3 — the fourteenth candidate, as one rule | `TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees` | the `DoorClosed` arm made `return forwarded`: red in the keystone, the property test and the goldens alone (`door closed in unknown scene "door-hall"`) |
| AL | A perch sends everything the new eyes see that the viewer was never sent. | accept — the seat arc's O, left to this record | `TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt` | in `reperch`, every scene of `st` marked introduced after `transitions` returns (1 red: `hopping back to a shoulder a burst flew past must introduce its room`) |

Refused, or not this sort's, each with its reason. **AM**, leaving a shoulder
takes its creatures and not its terrain (the seat arc's X,
`TestLeavingAShoulderTakesTheCreaturesAndNotTheTerrain`) — the sum of I and K;
the test is cited under both. **AN**, the DM and the agent answered with the
event itself (the seat arc's AC, `TestTheDMReceivesEverythingUnchanged`,
`TestTheAgentSeatReceivesEverythingUnchangedToo`) — VTT-176 holds the wire and
the DM's seat has no `Projector`; SPEC-016 states the arm. **AO**, an unknown
role, a nil event and a nil state answered with nothing
(`TestTheProjectionFailsClosedWhenItHasNothingToGoOn`) — defence;
`identity.Verify` and `receive` keep all three away. **AP**, a perch on a
seat that has folded nothing (the seat arc's Z,
`TestASeatPerchesOnlyAgainstAWorldItHasSeen`) — SPEC-015's, and `reperch`'s
guard alone is unobserved. **AQ**, `TestTheShippedGoblinArcherCannotBeGivenAControllerAtCreation`
and `TestCreatingAnActorAndThenGrantingItIsTheTwoStepThatReplacesTheOne` —
`add_actor`'s and the grant's (SPEC-013). **AR**,
`TestVisibleAndExploredComeFromDifferentSourcesAndMayDiffer` — `engine.Apply`'s
`SceneSeen` arm. **AS**, `TestTheKeystoneCorpusCanTellAProjectionFromAPassthrough`
— a guard on the corpus. **AT**, `canSeeSquare` answering false for a missing
position — defence; prose. Three clauses stay prose: a player's viewpoint
ignored and a spectator on a non-party actor seeing nothing (C), and the
`SceneSeen` walk's equal-size case until H2 is closed.

Thirty accepted outright (A, B, D to G, H1, I to U, X, Y, AB, AE, AG to AL),
two accepted on one byte comparison (Z, AC; Q10), one OPEN with no test (H2), three held out for rulings (2c) and
(3) (V, W, AA), and eleven refused (C, AD, AF, AM to AT). Of the accepted, AI's
red is a rate (12 of 12), and AK's red is shared by two other tests only.

**D6. Rows are dispensed after sign-off, cited by citation lines only, and an
OPEN row has no citer.** Forced by SPEC-008 and the previous plans' D6. A row
the sign-off keeps OPEN is written `**OPEN — no test yet**` with no citation
line, and the report names it (ticket item 3). A citation line is `// VTT-NNN`
directly above `func Test`, below any doc block, several ids on one line where
a test holds several; a test that already carries a line (the four
`server_visibility_test.go` tests D5 cites, VTT-177, VTT-178 and VTT-180 among
their ids) gains the new id on that line. Evidence cells are
`internal/gateway/<file>#<Test>`, written by hand after the dispenser; the
chain gate refuses one that is wrong. SPEC-016's Requirements line is copied
from the register last. Ids start at VTT-194. The test files that gain
citation lines are `project_test.go`, `project_property_test.go`,
`keystone_test.go`, `viewpoint_internal_test.go` and
`server_visibility_test.go`; `project_internal_test.go`, which the ticket
lists, gains none, because its one test is AF's, refused.

**D7. `project.go`: the 73 blocks by symbol, and what each becomes.** Forced
by ticket items 2 and 4 and by the pointers under Measurements. Exported:
`Viewer`, `Projector`, `Project`, which keep a doc sentence that `go doc`
prints, and `Projector`'s and `Project`'s keep a warning behind it. Every other
block above a function is a pointer sentence opening with the function's own
name or a warning opening with a word on no list (Measurements). The arms
rulings (2c) and (3) rewrite get one line each or nothing: `look`'s
party-member loop, the notes arm, the knows divider, the already-held divider,
the `knows` closure and the `ActorControlRevoked` arm. Every text below was
built into a scratch copy (the dry run under Measurements). A starting point;
the reading may shorten a text and may not lengthen one past the bound, and a
text may not drop the sense a pointer from outside relies on (the four named
under Measurements).

| Block (by symbol; lines, banned) | Becomes |
|---|---|
| `Viewer`'s doc (16, 2, over) | `// Viewer is one participant's seat and, for a spectator, the shoulder it perches on (SPEC-016).` |
| `Projector`'s doc (51, 5, over) | `// Projector is one connection's projection of the log (SPEC-016). Feed it the log from the first event, never a snapshot of engine.State: its actors map remembers what left sight, and both folds refuse a second introduction (TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed).` |
| the `scenes`/`actors` fields (7, 1, over) | `// Never delete from scenes: a scene introduced twice is a fold error. Forget an actor only where transitions does, after classify has forwarded its ActorRemoved.` |
| the `tokens` field (2, 1) | goes |
| the `seen` field (3, 1) | `// Compare seen, never merge it: SceneSeen carries the whole current set.` |
| the `doors` field (4, 0) | `// Correct a door belief only for a square in sight (doorTransitions).` |
| the range and tolerance constants (10, 1, over) | `// Pass these as arguments when a ruleset supplies them; never read a sight range off Actor.Attributes, which is game-system vocabulary (SPEC-016).` |
| `Project`'s doc (13, 0, over) | `// Project returns the frames this viewer is sent for env, judged against st, the state after it (SPEC-016). Never write to env or to anything it points at: a live event is one envelope shared by every seat (TestProjectingChangesNeitherTheEventNorTheState).` |
| `Project`, the DM and agent arm (4, 1) | `// Return env itself: the DM's and the agent's streams are the log (TestTheDMReceivesEverythingUnchanged).` |
| `Project`, the player and spectator arm (1, 0) | goes |
| `Project`, the unknown-role arm (5, 1) | `// Send a role this build does not know nothing (SPEC-016).` |
| `Project`, the unrecognised check (5, 0) | `// Emit nothing for a payload classify does not know, not even transitions: what it did to the world is unknown (SPEC-016).` |
| `perchSequence`'s doc (28, 4, over) | `// perchSequence is the sequence of every frame a perch sends (SPEC-015). Keep it 0: a perch has no causing event, and a borrowed number names a frame no event caused.` |
| `reperch`'s doc (29, 2, over) | `// reperch is transitions with no causing event, for the perch SPEC-015 applies (SPEC-016). Pass no cause: doorTransitions then skips no door, and new eyes need every door they see corrected (TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen).` |
| `reperch`, the nil-world guard (8, 2, over) | `// Keep this guard with look's: without both, a perch on a seat that has folded nothing panics (TestASeatPerchesOnlyAgainstAWorldItHasSeen).` |
| `sightView`'s doc (3, 0) | `// sightView is one look at the world through this viewer's eyes (SPEC-016).` |
| `look`'s doc (12, 3, over) | `// look is what this viewer's eyes see of st, recomputed on every call (SPEC-016). Add no memo that skips events: deciding which events cannot change sight is where a leak would hide.` |
| `look`, the eyes loop (4, 0) | `// Walk this map unordered only because the loop unions a set; sort every walk that emits frames (sortedSet).` |
| `look`, the scene entry (4, 1) | `// Create the scene's entry before asking sight: standing in a scene earns its board even when nothing is visible from there.` |
| `look`, the token loop (2, 0) | goes |
| `look`, the party-member loop (13, 4, over) | `// Know every party member, seen or not (SPEC-016).` |
| `eyes`' doc (1, 0) | `// eyes are the actors this viewer sees through (SPEC-016).` |
| `eyes`, the player arm (2, 1) | `// Ignore Viewpoint for a player: it comes from the client, and honouring it lends a player an NPC's eyes (TestAPlayerCannotBorrowAnNpcsEyesByPerching).` |
| `eyes`, the spectator arm (6, 1) | `// Refuse a shoulder that is not a party member here too, with MayPerch's predicate, so the two refusals cannot drift (SPEC-015).` |
| `transitions`' doc (57, 5, over) | `// transitions are the frames that bring this viewer's board from what it was sent to what it sees now, each carrying seq (SPEC-016). Keep each scene before the doors, tokens and SceneSeen in it, and each actor before its grants, conditions and tokens: both folds refuse them the other way round. cause is nil on a perch (reperch).` |
| `transitions`, above the forgetting loop (52, 4, over) | `// Forget an actor the world no longer has, after classify and before any introduction: this seat was forwarded its ActorRemoved, and an id used again must be introduced afresh (TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh).` |
| `transitions`, the scene introduction (7, 2, over) | `// Introduce the outline alone: tiles and objects arrive in SceneSeen.` |
| `transitions`, above `doorTransitions`' call (3, 0) | `// Correct doors only after the scenes are introduced: both folds refuse a door in a scene they do not have.` |
| `transitions`, the clone (4, 0) | `// Clone the actor: st.Actors holds live pointers a later grant mutates.` |
| `transitions`, the grants (23, 2, over) | `// Clear the controllers from the introduction, since both folds refuse an ActorAdded that names one, and send each behind it as a grant stating the actor's kind (TestAnIntroductionCarriesNoControllerAndTheGrantsBehindIt).` |
| `transitions`, the conditions (22, 3, over) | `// Send its conditions behind it: the Actor does not carry them (TestAConditionAppliedOutOfSightArrivesWithTheActor).` |
| `transitions`, the `SceneSeen` walk (9, 0, over) | `// Walk the scenes last reported as well as those in sight: a scene with no eye left is reported dark once, or the client keeps it lit (TestASceneThatLeavesSightEntirelyIsReportedDark).` |
| `transitions`, the unguarded `st.Scenes` read (7, 1, over) | `// Read st.Scenes unguarded: every id here came from st, and nothing removes a scene.` |
| `transitions`, the empty `SceneSeen` (9, 0, over) | `// Send an empty SceneSeen when the scene goes dark: it darkens the view and forgets no terrain (TestAnEmptySceneSeenDarkensTheSceneAndForgetsNoTerrain).` |
| `transitions`, forgetting a dark scene (4, 0) | `// Forget the scene once it is reported dark, or the empty SceneSeen repeats (TestASceneAlreadyReportedDarkIsNotReportedDarkAgain).` |
| `transitions`, storing the set (2, 0) | goes |
| `sceneSeenFor`'s doc (4, 1) | `// sceneSeenFor is the whole of what this viewer sees of sc now, never a delta (SPEC-016).` |
| `sceneSeenFor`, the visible set (20, 1, over) | `// Send the square set itself, sorted: tiles lose every square with no terrain, and an unsorted walk changes the bytes between runs (TestSceneSeenCarriesTheVisibleSquaresEvenWithNoTerrain).` |
| `sceneSeenFor`, a square with no tile (2, 0) | goes |
| `objectInSight`'s doc (16, 0, over) | `// objectInSight reports whether any square of o's footprint is visible. Keep the walk clamped to the grid and compared in int64: an enormous footprint otherwise spins, and an overflowing one wraps.` |
| `verdict`'s doc (1, 0) | `// verdict is classify's ruling on one payload (SPEC-016).` |
| the `unrecognised` constant (2, 0) | goes |
| the `withheld` constant (2, 0) | goes |
| the `forwarded` constant (1, 0) | goes |
| `classify`'s doc (12, 1, over) | `// classify rules on env for this viewer (SPEC-016). Keep an arm for every payload and the default unrecognised: a default that forwards leaks (TestEveryEnvelopePayloadArmHasAnExplicitRuling). Call it before transitions, which moves pr.tokens and pr.actors past this event.` |
| `classify`, the `knows` closure (4, 0) | goes |
| `classify`, the forwarded divider (1, 0) | goes |
| `classify`, the session arm (3, 0) | goes |
| `classify`, the narration arm (10, 2, over) | `// Forward narration to every viewer: it is addressed to the table.` |
| `classify`, the withheld divider (1, 0) | goes |
| `classify`, the `SceneCreated` arm (5, 1) | `// Withhold what transitions introduces: a second path to one introduction sends a duplicate that both folds refuse (SPEC-016).` |
| `classify`, the `ActorAdded` arm (10, 1, over) | goes |
| `classify`, the `TokenPlaced` arm (3, 0) | goes |
| `classify`, the `TokenRemoved` arm (17, 1, over) | `// Withhold: a viewer that held the token is sent a TokenHidden (TestARemovedTokenReachesAPlayerOnlyAsHidden).` |
| `classify`, the `TokenHidden`/`SceneSeen` arm (4, 1) | `// Withhold: only the projection issues these.` |
| `classify`, the notes arm (9, 1, over) | `// Withhold notes from every player and spectator (SPEC-016).` |
| `classify`, the `AdventureLoaded` arm (5, 0) | `// Withhold: the load names the adventure, and each event of its batch is projected on its own.` |
| `classify`, the seen-subject divider (1, 0) | goes |
| `classify`, the `TokenMoved` arm (6, 0) | `// Require the token held before and seen after: a move names both ends (TestSteppingIntoViewArrivesRatherThanMoves).` |
| `classify`, the `DoorClosed` arm (4, 0) | goes |
| `classify`, the knows divider (7, 1, over) | `// Forward testimony when the viewer knows every actor it names (SPEC-016).` |
| `classify`, the `ActorControlRevoked` arm (3, 0) | goes |
| `classify`, the already-held divider (38, 0, over) | `// Forward these to a viewer that held the actor before this event: its introduction already carries the change (SPEC-016).` |
| `classify`, the `ActorRemoved` divider (2, 0) | goes |
| `classify`, the `ActorRemoved` arm (31, 2, over) | `// Forward only to a seat that held the actor: a fold without it refuses the removal (TestARemovedActorReachesOnlyTheSeatThatHeldIt).` |
| `doorTransitions`' doc (31, 1, over) | `// doorTransitions corrects what this viewer believes of the doors it sees now (SPEC-016). Skip the square of the door event being projected: classify forwards that event, and a second frame would repeat it.` |
| `doorTransitions`, the ordinary square (2, 0) | goes |
| `doorTransitions`, the unparseable key (4, 0) | `// Skip a key that names no square rather than invent coordinates.` |
| `doorSubject`'s doc (4, 0) | `// doorSubject reports the scene and square a door event is about; a nil cause, a perch's, reports none.` |
| `squareAt`'s doc (6, 0) | `// squareAt is squareKey's inverse, refusing a key whose halves strconv does not consume whole.` |
| `squareKey`'s doc (9, 1, over) | `// squareKey must build the key sight.VisibleFrom builds, or no square is visible to anyone.` |
| the floating block above `sortedSet` (6, 1) | `// Sort every set before emitting from it: map order is random, and one log must project to the same stream on every run (TestTheSameLogProjectsTheSameStreamEveryTime).` |
| `sortedSceneIDsUnion`'s doc (5, 0) | `// sortedSceneIDsUnion is sortedSceneIDs over every id in either map, once.` |

Trailing comments: `Viewer.Viewpoint`'s, the one after `Project`'s nil-state
`return nil`, and `sightView`'s three field comments go; the three field lines
lose their trailing padding when `gofmt -w` runs, which changes whitespace and
no token. Every other field that has a comment today keeps a one-line comment
above it except `tokens`, whose name is as wide as `scenes` and `actors`, so no
field joins another's alignment group and gofmt re-aligns nothing (dry run).
Where a deleted block sat directly after an opening brace, the blank line
below it goes too, or `golangci-lint`'s `whitespace` check refuses the file
(dry run: `classify`'s switch).

**D8. The sweep's rules, repeating the previous plans' where they apply.**
Forced by rule 10, SPEC-010 and the identity plan's D1 to D8.

- *Three kinds and nothing else*: a warning is imperative, a verb first, the
  consequence in the present tense, at most three lines; a pointer is
  `SPEC-015`, `SPEC-016`, `VTT-NNN`, a test name, a symbol name or a `docs/`
  path, and may close a warning or a doc sentence in parentheses; a doc
  sentence is the first sentence `go doc` prints for an exported symbol. Every
  fact a deleted block held that SPEC-016 does not state is either added to
  SPEC-016 (D9) or named in the report as dropped, with the reason.
- *A warning above a function opens with a word that names no function*: the
  list is under Measurements.
- *A doc sentence is one sentence, on one line where it fits*; the wrap band is
  `SHORT, LONG = 55, 85` in `tools/check-comment-wrap.py`, which
  `check:new-prose` applies to added lines.
- *Test files*: citation lines (D6) and one re-aimed line (D12). No other
  test-file edit; the doc blocks above the projection tests, 44 of them over
  the bound in `project_test.go` alone, are the test-prose sweep's.
- *What is left alone*: every comment in `seat.go`, `server.go`,
  `internal/sight`, `internal/engine` and `client/src`, stale or not (D17).

**D9. Facts only a comment holds go into SPEC-016 or the report.** Forced by
ticket item 1 and the identity plan's D7. The reading of each cut block asks
whether SPEC-016's draft states the fact; if not, and the fact is about the
code now, it is checked against the symbol and written in, or named in the
report as dropped. The candidates are under "Candidates the reading starts
from". Most of what the cut blocks narrate is held already by
`docs/reports/2026-08-18-visibility.md` (its section 6.C lists `project.go`'s
narrative blocks by the same ranges as `db1f77d`'s, and its section 3 holds
the decisions and their rejected alternatives), which the report names rather
than repeats. The searches SPEC-016 names for its absolutes, each run at
verification: `grep -rn '\.Project(\|\.reperch(\|\.look('` over non-test files
(`Project` from `receive`, `reperch` from `perch`, `look` from `Project`,
`reperch` and `canSee`); `grep -n 'delete(pr.scenes'
internal/gateway/project.go` (nothing) and no `delete` from a state's `Scenes`
under `internal/` (so nothing removes a scene); `grep -rn
'Envelope_SceneCreated{\|Envelope_ActorAdded{\|Envelope_TokenPlaced{\|Envelope_TokenHidden{\|Envelope_SceneSeen{'`
over non-test gateway files (`transitions` builds every introduction a
projected viewer is sent; `convert.go`'s two build log events from commands);
the `TokenHidden`/`SceneSeen` grep under Measurements.

**D10. The comment-stripped comparison is a token stream.** Forced by the
identity plan's D11. The program is that plan's Task 0 listing
(`docs/superpowers/plans/2026-09-24-sweep-identity.md`: `go/scanner`, mode 0,
`fmt.Printf("%s %q\n", tok, lit)`), built once in the scratchpad
(`$S/codetokens/codetokens`, a `go.mod` with `module codetokens` beside it) and
run from the repository root:

    for f in internal/gateway/project.go internal/gateway/project_test.go \
             internal/gateway/project_property_test.go internal/gateway/keystone_test.go \
             internal/gateway/viewpoint_internal_test.go internal/gateway/server_visibility_test.go; do
      git show "db1f77d:$f" > "$S/before.go"
      "$S/codetokens/codetokens" "$S/before.go" > "$S/before.tok" || echo "SCAN FAILED $f"
      "$S/codetokens/codetokens" "$f" > "$S/after.tok" || echo "SCAN FAILED $f"
      printf '%s %s/%s tokens ' "$f" "$(wc -l < "$S/before.tok")" "$(wc -l < "$S/after.tok")"
      cmp -s "$S/before.tok" "$S/after.tok" && echo same || echo DIFFERS
    done

Done reads six `same` lines at 3502, 16828, 1600, 4593, 2192 and 6280. A run
proves it ran by the counts; a zero is a failed run. A `DIFFERS` on a test
file means a string literal changed, and Phase 4b names it.

**D11. The ledger, last.** Forced by SPEC-010 (the band) and by
`--write-ledger` lowering a row on any drop. After Phase 4b has settled,
`python3 tools/check-comments.py --write-ledger`, then `git diff
tools/comment-ceilings.txt` must show exactly one row changed, lowered:
`internal/gateway/project.go` (the dry run: 61.7 to 20.4; the reading's final
texts decide the number). The test files gain citation lines and one changed
line only, which move no share (VTT-059; the changed line replaces a comment
line with a comment line). A further changed row means a file changed that
this plan does not name: stop, name it, and ask before committing. The commit
message lists the row old and new.

**D12. The one test-file line re-aimed.** Forced by the pointer under
Measurements. In `TestNarrationReachesAPlayerAndANoteDoesNot`'s body block (5
lines, under the bound), "world record the DM keeps. See project.go for the
full reasoning; this" becomes "world record the DM keeps. SPEC-016 states both
rulings; this". A changed line in a block within the bound is allowed; the
line carries no banned term (the block's `spec §4.4` is on another line and is
not added). D10 prints `same` at 16828. Q8 asks whether to leave it instead,
since ruling (2c)'s ticket rewrites the test.

**D13. The specification edits, sentence by sentence.** Forced by ticket item
6 and check 6.

- SPEC-013, "The player's move gate", its last sentence: "`canSee` and
  `viewerFor` are SPEC-015's; what a projection sees is the `Projector`'s in
  `project.go`, which has no record yet." becomes "`canSee` and `viewerFor`
  are SPEC-015's; what a projection sees is SPEC-016's."
- SPEC-015, "What this record does not decide": "what a projection computes
  (`Project`, `reperch`, `eyes`, `look`, `canSeeSquare`, `perchSequence`, the
  `Viewer` type) is `project.go`'s and has no record yet;" becomes "what a
  projection computes (`Project`, `reperch`, `eyes`, `look`, `canSeeSquare`,
  `perchSequence`, the `Viewer` type) is SPEC-016's;".
- Per Q7, SPEC-015's five other sentences that send a projection rule to
  `project.go`: "(`project.go`)" after `Project`'s DM and agent answer in
  "Which seats are projected", after `eyes`' reading of the viewpoint in "The
  viewpoint a connection opens with" and after `eyes`' second refusal in "Who
  may perch"; "That a projector must be fed from the start of the log is the
  `Projector`'s (`project.go`)." in "How a projected seat is fed"; and "What
  `reperch` sends, and that a shoulder named again is served in full, since the
  projector's memory never held it, are `project.go`'s." in "How a perch is
  applied". Each names SPEC-016 in place of `project.go`, and no other word
  changes.

`git diff` on both files shows only these. SPEC-007, SPEC-009, SPEC-011 and
SPEC-012 are pointed at and not edited.

**D14. The mutation keys move, and are re-pointed last.** Forced by the
self-test under Measurements and by rule 2. After Phase 4b has settled and
before the ledger: locate each entry's quoted expression in the final
`project.go` (`y < sc.GridHeight && int64(y) < int64(o.Y)+int64(o.Height)`,
its `x` twin, and `seen := make(map[string]bool, len(a)+len(b))`), re-derive
line and column from the text there, never by offset, and rewrite the three
keys and the four current-coordinate references in their bodies (`760:54`,
`760:66`, `761:54`, and the slice hint's `:1220` and `1220:33` where the body
names it as current), appending one re-point sentence to each entry in the
file's own form. Coordinates that narrate earlier re-points stay. Then both
self-tests print `OK`. The dry run's positions are `365:26`, `366:27` and
`606:38`, and the columns do not change because the code lines do not. Any
edit to `project.go` after the re-point re-opens this decision, so the
re-point is the last edit to that file.

**D15. The deliberate breaks, one per check this work relies on.** Forced by
the dev-cycle's rule that a check is proven by a red, and ticket items 2 to 6.
In a scratch clone (`git clone --no-hardlinks` into the scratchpad, the final
`git diff HEAD` applied, the untracked files copied in, committed there, that
clone's `main` pointing at that commit): first each gate exits 0 with its
completion line; then each break is one edit, the finding recorded verbatim,
the inverse edit made by hand (never `git checkout --`), `git diff --stat`
printing nothing before the next.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | `// VTT-999` added to the citation line above `TestAnUnrecognisedPayloadIsWithheldFromAPlayer` | `check:requirements-chain`: `project_test.go cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 | one new row's evidence re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `check:requirements-chain`: `does not carry the id, so the link walks one way only` |
| B3 | SPEC-016's Requirements line gains `VTT-999` | `check:requirements-chain`: `cites VTT-999 ... (specification citation)` |
| B4 | one pointer line, `// SPEC-016`, inside `sameSet`'s body, after the ledger is written | `check:comments`: `internal/gateway/project.go: comment share X is above its ceiling Y and this change added a comment line to it (SPEC-010)` (dry run: `20.49 is above its ceiling 20.4`) |
| B5 | `pr.tokens[id] && now.tokens[id]` made `now.tokens[id]` in `classify` | D10's loop: `project.go ... DIFFERS`, and `TestSteppingIntoViewArrivesRatherThanMoves` red (run at verification) |
| B6 | "UNRESOLVED" re-inserted into `transitions`' doc | `grep -c 'UNRESOLVED' internal/gateway/project.go` prints 1 |
| B7 | the pointer above `classify` rewritten to open with "Project" | `check:doc-owner`: ``the doc comment above `classify` begins by describing `Project`, which is a different function`` (dry run) |
| B8 | one `project.go` key in `tools/mutation-equivalents.txt` moved one line down | `python3 tools/check_mutation_test.py -q` fails in its key-position case, naming that key and why its mutator cannot apply there (the dry run's unmoved keys failed the same case: `there is no such line in the tree`) |

**D16. Phase 4b is the check for VTT-051 and for SPEC-016, sentence by
sentence.** The reviewer gets `git diff HEAD`, SPEC-007, SPEC-010, SPEC-011,
SPEC-013, SPEC-015 and SPEC-016, the rows VTT-050 to VTT-059, VTT-176 to
VTT-193 and the new rows, `project.go`, the touched test files, and the code
SPEC-016 points into: `seat.go`'s `receive`, `perch` and `canSee`;
`internal/sight/sight.go`'s `VisibleFrom` and `Blockers`;
`internal/engine/actorkind.go`'s `IsPartyMember`; `engine.Apply`'s arms for the
payloads `transitions` builds and `client/src/fold.ts`'s mirror of them; the
contract's `Envelope` oneof. For every surviving block it names the kind (D8)
and, for a warning, the code it guards and whether the consequence is true of
that code; for a pointer, that the target resolves; for a doc sentence, that
the symbol is exported and the sentence true. For every deleted block: did it
hold a fact now in no record? If yes, SPEC-016 (D9) or the report. For every
SPEC-016 sentence: the symbol it names and whether the code under it does what
the sentence says — `catches.md` item 8 is the one to watch, since this plan
found two sentences false already. For each dispensed row: the edit that would
red the test named. For Status: that it claims nothing the code does not do.
For the SPEC-013 and SPEC-015 edits: that nothing else changed. For the rule-9
answer: that SPEC-016 names no other project.

**D17. Phase 4a is skipped, with its reason, and things the scope leaves
stale are named.** Independent QA derives tests from a specification to find
behaviour the implementer got wrong; this change has no behaviour (D10 shows
every Go file's code identical to `db1f77d`), so what can be wrong is a
sentence, and a reading holds that (D16). The report records the skip under
its own heading. Left stale and named in the report:
`internal/engine/apply.go`'s "internal/gateway/project.go quotes this format
string" (Q8); `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh`'s
quotation of a `transitions` comment that no longer says it;
`TestAnObjectIsRevealedOnlyByTheSquaresItStandsOn`'s `project.go:510:54` and
`511:54`; the projection tests' docs over the bound, histories among them;
`keystone_test.go`'s "internal/gateway/seat.go's own doc comment says so";
`client/src/wire.ts`'s replay-cursor block, which describes the projection's
batches correctly and cites nothing in `project.go`'s comments. The report
does not revise the visibility report.

**D18. One commit for the change; the report in its own.** Forced by the band
(a file's deletions and its row land together), by the chain gate (a row's
evidence and its citation line land together, and neither hook runs
`check:requirements-chain` or `check:comments`), by the pointers (`SPEC-016`
must have a target in the same tree), and by the keys (a moved key and the
moved line land together). The commit carries the ticket (untracked today),
this plan, SPEC-016, the SPEC-013 and SPEC-015 edits, the register,
`project.go`, the test files D6 and D12 name, `tools/mutation-equivalents.txt`
and the ledger. The pattern is `7be685a` then `db1f77d`.

**D19. Gate steps before the commit, in order, on a tree the review has
settled.** `gofmt -l internal/gateway/` prints only `scenario_test.go`; `go vet
./internal/gateway/`; `go test -count=1 ./internal/gateway/...` green; D10's
loop, every `same`; `task check:comments` (expected, before Task 10, to refuse
`project.go` for the band and nothing else); `task check:doc-owner`; `task
check:requirements-chain` (`<193 + N> rows, 193 test files, 10
specifications; every citation resolves and every row's evidence holds`);
`task check:new-prose` (every test and symbol name the sweep writes must
resolve); `python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q` (`OK` after D14, and the first reds before
it); `task lint` (`0 issues`). Then, after Task 10, `task check` whole, once, on
the final tree, launched in its own session (`start_new_session=True`), and
only after Phase 4b has settled, so no edit lands mid-run; `uptime` first,
since the e2e waits fail under load (gap 9); `check:mutation` runs over the
re-pointed keys and must report no unadjudicated survivor and no key it cannot
place. Then the pre-commit hook's own set, with `git add` and `git commit` in
separate calls, and what landed checked with `git show --stat HEAD`.

## Candidates the reading starts from

Read by the verifier at `db1f77d`. A starting point, not a verdict.

**Facts for SPEC-016 that only a comment states today.** A live event is one
envelope handed to every seat (`store.notifyLocked` enqueues the same pointer
to each subscriber; a catch-up preload is read per subscriber), so `Project`
writes to nothing it is given. The DM's and the agent's answer reads no state.
`identity.Role` is a string, so a row can carry a role this build does not
know, and `Project` sends it nothing. An unrecognised payload derives nothing,
because the projection cannot tell what it did to the world. The projector's
maps are the running result of a function of the log prefix and the viewer,
not a cache of the state: two projectors fed one prefix hold the same maps;
`actors` is path-dependent (an NPC seen and then hidden stays known). A viewer
learns of a scene, an actor or a token by one introduction, and both folds
refuse a second (`engine: scene %q already exists`, `duplicate scene`);
`TokenHidden` is tolerated when repeated. `look` is recomputed on every call
because deciding which events cannot change sight is where a leak would hide;
it reads the state and writes nothing. Standing in a scene earns its board even
when nothing is visible from there. Party members are always known, by kind.
A player's `Viewpoint` comes from the client, and honouring it would lend any
player an NPC's eyes. `transitions`: the order is load-bearing in both folds
(scene before tokens and `SceneSeen` and doors, actor before token and grants
and conditions); departures come before arrivals, the one pair the folds do not
constrain, so a viewer never holds more tokens than it is entitled to within a
batch. An actor is forgotten after `classify` has seen that the seat held it,
so forgetting and forwarding `ActorRemoved` are one decision. The introduction
clones the actor because `st.Actors` holds live pointers that a grant mutates
in place. An `ActorAdded` naming a controller is refused by both folds, so
controllers travel as grants carrying the kind; a viewer present at the grant
that introduces an actor receives it twice, which both folds accept. Resources
ride in the `Actor`; conditions are a separate map, so they follow the
introduction, and a later `ConditionRemoved` would otherwise be refused; a
condition so carried bears the introduction's sequence. The `SceneSeen` walk
covers scenes last sent as well as those in sight, because a scene with no eye
left has no entry in the look and the client would keep it lit; an empty
`SceneSeen` darkens and forgets no terrain; the dark scene then leaves `seen`
so the empty frame is not repeated. The visible set is sent sorted, and the
tiles map is ordered by the encoder. An object with no square is shown to
nobody, as `sight.Blockers` casts no shadow for it; the footprint walk is
clamped to the grid and compared in int64. `classify` is called before
`transitions`. Narration is addressed to the table; a note is a record nothing
addresses; `add_narration` is open to player, DM and agent and `upsert_note`
to DM and agent (SPEC-013's table). An adventure load names the adventure, is a
no-op for the engine, and each event of its batch is projected on its own. A
move names both ends, so a move from a square the viewer never saw would hand
it that square. A door out of sight keeps what the viewer last saw, as terrain
does; `OpenDoors` travels in neither the redacted `SceneCreated` nor
`SceneSeen`, so without `doorTransitions` a door opened before a viewer had
eyes stays shut on its board. `squareAt` reads a key strictly. `squareKey` must
build the key `sight.VisibleFrom` builds.

**Text that goes on sight.** Every `spec §`, `Task N`, date, `Patrik`,
`MEASURED`, `used to`, `an earlier version`, `(Corrected 2026-08-21: ...)`,
`retraction-leaves Task 8/9`, `exit criterion`, `finding 14`, the Patrik
quotations, the undo and retraction histories in `perchSequence`'s,
`transitions`' and the forgetting loop's blocks, the scene-forgetting loop's
obituary and its argument about the mutation gate, the torn-batch paragraph
and its line citations (`wire.ts:175`, `:363`, `:411`, `session.ts:158-160`),
the 15 ms and 176 ms figures, the three shapes where a duplicate
`ResourceChanged` folds silently, the protojson encoder argument, the
"fourth copy of a three-line format" argument, both `FLAGGED FOR
ADJUDICATION` paragraphs, and `reperch`'s "measured, and written up at
perchBox".

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository; the scratchpad, `$S`.

Build D10's program; run the loop. Run `python3 tools/check-comments.py
--report | grep -E 'internal/gateway/(project|project_test|project_internal_test|project_property_test|keystone_test|server_visibility_test|viewpoint_internal_test)\.go'`,
`python3 tools/check-requirements-chain.py .`, `python3
tools/check-comments.py main`, `python3 tools/check-doc-owner.py .`, `gofmt -l
internal/gateway/`, the three `grep -c` of ticket item 2, both mutation
self-tests, and `grep -c '^// VTT-'` over the five test files that will gain
citations; keep the outputs. Confirm `requirement-id` is on the path.

**Done when:** the outputs match the measurements above (`same` at 3502,
16828, 1600, 4593, 2192 and 6280; `193 rows ... 9 specifications`; `239 files
... clean`; only `scenario_test.go` from gofmt; `1`, `1`, `2`; both
self-tests `OK`; citations 0, 0, 0, 3 and 14); `requirement-id` prints its
one-line usage.

### Task 1 — SPEC-016

**Files:** `docs/specifications/016-the-projection.md`, new.

Per D1, D2, D3 and D9; the `specification` skill's steps 1 to 5 and 7
(`catches.md`, two passes at most). Step 6's Requirements line is written
`None yet; the sort of 2026-09-29-the-projection-has-a-record-design.md fills
it` until Task 7 and then copied from the register.

**Done when:** the file has the five headings in order (`grep -c '^## '`
prints 5), no `Why`, no `Rejected`, no number that is a measurement, no line
number, no date outside a `docs/` path, no path outside the project, no other
project's name; Status is D3's two paragraphs or the sign-off's replacement;
every sentence under "How it works" names a symbol in `project.go`, `seat.go`,
`internal/sight`, `internal/engine` or `client/src/fold.ts`, and a table of
sentence to symbol is kept for Task 8; the two false clauses of the ticket's
problem paragraph and the three compressions are stated as the code has them;
every "only", "never", "every", "none" and "alone" names its search; `python3
tools/check-requirements-chain.py .` prints `10 specifications`.

### Task 2 — SPEC-013 and SPEC-015

**Files:** `docs/specifications/013-authorization.md`,
`docs/specifications/015-the-seat-and-the-perch.md`.

D13's sentences.

**Done when:** `grep -n "has no record" docs/specifications/013-authorization.md
docs/specifications/015-the-seat-and-the-perch.md` prints no line about the
projection; `git diff --stat` on each shows only D13's lines.

### Task 3 — `project.go`

**Files:** `internal/gateway/project.go`.

D7's table under D8; every fact of the cut blocks checked against SPEC-016's
draft, each gap into SPEC-016 (D9) or the report's dropped list; then `gofmt
-w`.

**Done when:** D10 prints `same` for `project.go` at 3502; `--report` shows
`project.go` with `banned 0` and `blocks>6 0`; the three `grep -c` of ticket
item 2 print 0, 0 and 0; `gofmt -l` prints nothing for it; `check:doc-owner`
still ends `79 files ...`; `task lint` prints `0 issues`; `go doc` prints one
paragraph for each of `Viewer`, `Projector` and `Project`.

### Task 4 — The re-aimed test line

**Files:** `internal/gateway/project_test.go`, per Q8.

D12.

**Done when:** D10 prints `same` at 16828; `grep -c 'See project.go for the
full reasoning' internal/gateway/project_test.go` prints 0.

### Task 5 — Local gates, first pass

**Files:** none changed.

D19's list up to and not including `task check` whole, the mutation
self-tests excepted (they red until Task 9).

**Done when:** each step exits 0 with its completion line, except
`check:comments`, which at this point refuses `project.go` for the band and
nothing else; `check:new-prose` reports no citation to a name the tree never
declared.

### Task 6 — Sign-off of the sort

**Files:** none.

D5's table is presented with the reading's confirmations and overrides, each
override with its reason, and the questions below answered. Nothing is
dispensed before the answer.

**Done when:** each lettered row has one of: accept (with wording and
evidence), refuse (with reason), OPEN (with reason), held for its ruling's
ticket; Q1 to Q11 have answers.

### Task 7 — Rows and citation lines

**Files:** `docs/requirements.md` (by the dispenser, then evidence cells by
hand), `internal/gateway/project_test.go`, `project_property_test.go`,
`keystone_test.go`, `viewpoint_internal_test.go` and `server_visibility_test.go`
(citation lines only, D6), `docs/specifications/016-the-projection.md` (the
Requirements line, copied).

**Done when:** `python3 tools/check-requirements-chain.py .` prints `<193 + N>
rows, 193 test files, 10 specifications; every citation resolves and every
row's evidence holds` with N the accepted count plus the OPEN rows; an OPEN
row has no citer; D10 prints `same` for every test file; `--report` shows each
file's share unchanged by its citation lines.

### Task 8 — Phase 4b, the reading review

**Files:** whatever its findings touch among the above.

Per D16. Findings are fixed and the affected task's "done" is re-run. The
review settles before Task 9 starts; if the reviewer dies on a model's limit,
say so and re-dispatch with the same brief on `fable`.

**Done when:** the review record names every surviving block's kind, every
deleted block's outcome, every SPEC-016 sentence's symbol and verdict, every
row's red-making edit, the SPEC-013 and SPEC-015 diffs and Status, and reports
no open finding; D10 prints every `same`.

### Task 9 — The mutation keys

**Files:** `tools/mutation-equivalents.txt`.

D14.

**Done when:** `python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q` print `OK`; each of the three keys reads,
at its new line and column, the token its mutator rewrites (the test is the
check); `git diff tools/mutation-equivalents.txt` touches those three entries
alone.

### Task 10 — The ledger

**Files:** `tools/comment-ceilings.txt`, by `--write-ledger` only.

Per D11.

**Done when:** `git diff tools/comment-ceilings.txt` shows exactly D11's row,
lowered; `task check:comments` ends `clean`; `--report` prints `banned 0` and
`blocks>6 0` for `project.go`.

### Task 11 — The breaks and the whole gate

**Files:** none in the repository.

D15 in a scratch clone; then `task check` whole, once, per D19.

**Done when:** the clone's clean run exits 0 with each completion line, and
each of B1 to B8 produces the one red D15 names; `task check` exits 0 with
every step, `check:comments`, `check:requirements-chain`, `check:doc-owner`,
`check:mutation` and `lint` among them, printing its own verdict.

### Task 12 — Commit, then the report

**Files:** the commit's, per D18; then
`docs/reports/2026-09-29-the-projection-has-a-record.md`.

The commit message lists the ledger row old and new, D10's counts, the rows
dispensed, the keys' old and new positions, and B4's finding line verbatim.
After it: the report per the `implementation-report` skill, in its own commit —
comment lines and share before and after, blocks kept by kind, the facts
SPEC-016 took and the facts dropped with reasons (naming the visibility report
where it holds them), the sort (every row with its id and test, every refusal
and every held-out row in one line), the breaks' finding lines, the gaps below
as found or closed, D17's stale list, the sign-off answers, the rule-9 answer,
and the unobserved arms with the probes that showed them.

**Done when:** `git show --stat HEAD~1` lists the ticket, this plan, SPEC-016,
SPEC-013, SPEC-015, the register, `project.go`, the touched test files,
`tools/mutation-equivalents.txt` and the ledger, and nothing else; `git diff
--stat db1f77d -- docs/reports/` lists only the new report; `git diff --quiet
db1f77d -- internal/sight internal/engine internal/campaign internal/store
cmd/ contract/ client/src internal/gateway/seat.go internal/gateway/server.go
internal/gateway/viewpoint.go` exits 0.

## Commits

| Commit | Carries | Gate steps it runs |
|---|---|---|
| C1 | the ticket, this plan, SPEC-016, the SPEC-013 and SPEC-015 edits, the register, `project.go`, the touched test files, the mutation keys, the ledger | D19's list by hand, `task check` whole (Task 11), then the pre-commit hook (lint, vet, tier-1, arch, vocabulary, doc-owner, secrets, typecheck, review gate) |
| C2 | the implementation report (and a debt entry if Q5 says so) | pre-commit hook |

Push after C2: pre-push runs tiers 2 and 3 and the contract gates, about three
minutes; let it finish. No Go code changes, so `check:drift` has no client
change to compare; the mutation keys move with the comments and are re-pointed
in C1.

## Gaps that travel with this plan

1. **A visible set that changes to another of the same size is unobserved**
   (row H2). With `sameSet` comparing sizes alone, the closure stays green; a
   viewer walking along a wall, whose count of visible squares holds, would
   keep an old lit area on its client. Q5.
2. **The testimony and already-held arms are observed thinly**, and ruling (3)
   rewrites them: an attack naming known actors withheld is green over the
   closure; a grant, a revoke or a resource change withheld from a viewer
   entitled to it reds only the goldens' byte comparison; the already-held test
   loosened to knows is green and cannot differ in production. Rows V and W
   are held for that ticket (Q2), and its tests inherit this list.
3. **Four arms are observed only by a byte comparison of committed streams**
   (`TestTheProjectedGoldensAreWhatTheProjectionActuallySends`): a session's
   start and end forwarded (Z), an adventure load withheld (AC), and, in the
   gateway package, a grant, a revoke and a resource change forwarded to a
   viewer entitled to them. Any change reds that test and a regenerated golden
   greens it. Q10.
4. **The ticket's file list omits `tools/mutation-equivalents.txt`**, whose
   three `project.go` keys move with any comment deleted above them; the
   mutation self-test reds until they are re-pointed (D14, Q9). It also omits
   the pointer in `internal/engine/apply.go` (Q8) and the line in
   `project_test.go` (D12). The ticket's author named all three at sign-off.
5. **Two clauses of the ticket's problem paragraph are false**: `ActorRemoved`
   is forwarded to a viewer that held the actor, not withheld, and no payload
   removes a scene; an unrecognised payload reaches the DM and the agent.
   SPEC-016 states the code, and the ticket's author corrected both at
   sign-off.
6. **Defence arms are observed only by their own unit tests** (C, AN, AO, AP,
   and `reperch`'s guard alone by nothing): the wire cannot reach them, and
   SPEC-016 states each as prose.
7. **Status names tickets not yet written** (D3): the rulings' tickets do not
   exist, so Status names where the rulings are recorded and says so. Q1.
8. **Whether the keystone is one rule or the sum of the others** (the ticket's
   question): none of the sixty-odd probes reddened the keystone alone; the
   `DoorClosed` edit reds it with the property test and the goldens only. D5
   accepts it as one rule, Q3.
9. **The `cmd/vtt` MCP deadline tests failed during a closure run** whose edit
   cannot reach them (`TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessIgnoresStdio`,
   under a load between 7 and 25 while three probe clones ran at once); the
   seat arc measured one of that family failing at the base once in three. `task
   check` is launched with `uptime` first (D19).
10. **Done item 2 allows one `FLAGGED FOR ADJUDICATION` for the notes arm**;
    ruling (2c) has adjudicated it, so D7 takes it to 0, within the item.

## Questions for sign-off

1. **SPEC-016's Status and the two rulings.** D3 proposes the skill's first
   form for the code as it is, then a paragraph naming the two rulings, the
   sentences each will change, and that their tickets are not yet written,
   with the rulings' only record, this ticket, named by path. Recommend that
   wording, and that each later ticket's first edit is that paragraph. The
   alternative, writing both tickets before C1 so Status can name them by path,
   delays this record for two tickets it does not need.
2. **Rows V, W and AA — hold them for the rulings' tickets?** Recommend yes:
   each is rewritten within two tickets, a withdrawn row keeps its number for
   good (SPEC-008), and the tests stay uncited until the ticket that changes
   them sorts them. SPEC-016 states the arms as they are.
3. **Row AK, the keystone, as one rule?** Recommend yes: it is the only check
   whose oracle is written apart from the code, it runs at every prefix of a
   real corpus, and one edit (the `DoorClosed` arm forwarding) reds it with the
   property test and the goldens and nothing else.
4. **Row C refused, both clauses prose?** Recommend yes: `MayPerch` (VTT-181)
   and `viewerFor` (VTT-180) keep both cases off the wire, and each clause reds
   only its own unit tests.
5. **Row H2 OPEN, and a debt entry in C2?** Recommend both: the rule is real
   and its consequence is at the table (a stale lit area), a test is writable
   (a token stepping along a corridor whose visible count holds), and
   `docs/verification-debt.md` is the one file for known coverage gaps. The
   entry's recipe is the probe's edit. The ticket's file list does not name the
   debt file, so this needs the yes.
6. **Rows O, Q and X under ruling (3).** Recommend accept all three as worded:
   O says a party member is introduced, not what the introduction carries; Q is
   worded for an actor introduced when seen, which ruling (3) leaves standing;
   X's reason is the fold (a seat without the actor refuses the removal), not
   testimony. If the owner reads ruling (3) as reaching any of them, hold that
   row with V and W.
7. **SPEC-015's five other pointers to `project.go`: re-aim them to SPEC-016?**
   Recommend yes (D13): after this change the rules they name are stated in
   SPEC-016, and the comments a reader following "(`project.go`)" would land on
   are the ones this sweep cuts. The ticket's item 6 names only the sentences
   that say "no record", so this needs the yes.
8. **Two pointers outside the sweep: re-aim `project_test.go`'s "See project.go
   for the full reasoning" (D12) and leave `internal/engine/apply.go`'s "project.go
   quotes this format string" stale?** Recommend both: the first is one line in
   a block within the bound; the second sits in a 13-line block of a package
   this ticket does not touch, and the sentinel's warning still stands on its
   other three quotations.
9. **The mutation keys: re-point them in C1 (D14)?** Recommend yes; it is not
   a choice, since the self-test and `check:mutation` refuse a key the tree
   cannot place, but the ticket's list omits the file.
10. **Rows Z and AC, observed only by the goldens' byte comparison: accept, or
    OPEN until a named assertion exists?** Recommend accept: the test does red
    under each edit and the rule is real at the table (a spectator with no
    session panel; a player told which adventure was loaded). The report names
    the weakness — a regenerated golden greens the test — and a named assertion
    for each is a few lines in `TestNarrationReachesAPlayerAndANoteDoesNot`'s
    shape, for a later ticket.
11. **If a swept block lands with every line a true warning, pointer or doc
    sentence and still over the bound?** Recommend the reading governs: stop,
    report the block, and let the ticket's writer decide, rather than cut a
    true warning for the bound.
