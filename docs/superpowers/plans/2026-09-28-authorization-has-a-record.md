# Authorization has a record — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-28-authorization-has-a-record-design.md`
**Verified:** 2026-09-28, by `verify-ticket`, an agent that did not write the
ticket, against `664cacf` on `chore/spec-013-authorization` (`main` at the
time; the ticket untracked). Verdict: **Passes with gaps.** The gaps are listed
at the end and travel with this plan. This plan does not edit the ticket; where
an item is thin, the plan decides around it and says so.

**Goal, in the ticket's words:** `docs/specifications/013-authorization.md`
exists, the three false sentences are gone, every rule the sort accepts has a
row cited by a gateway test, `authz.go`, `grant_validate.go` and
`add_actor_validate.go` carry only warnings, pointers and doc sentences with
their ceilings lowered, `server.go`'s sixteen blocks on `answerCommand`,
`authorize`, `handleSetViewpoint`, `handleCommand` and `describeBlockage` are
pointers to SPEC-013 or warnings, and no code line changes.

**MapTool (CLAUDE.md rule 9), the answer this plan records.** MapTool's
ownership predicate is `Token.isOwner` in `model/Token.java`:
`ownerType == OWNER_TYPE_ALL || ownerList.contains(playerId)`, membership in a
list plus an owned-by-all wildcard. The membership half is the shape this
platform's `controls` already has over `Actor.controller_ids`; the wildcard is
the shape this platform has the opposite of, since an empty controller set is
DM-and-agent only (`controls` answers false for an empty set, and row G below
holds it). Everything else does not fit. Every check runs in the client:
`AppUtil.playerOwns` answers true for a GM, true when
`ServerPolicy.useStrictTokenManagement` is off, and `token.isOwner` otherwise,
and it is called from thirty client files (`PointerTool`, `StampTool`,
`AbstractTokenPopupMenu`, `TokenPopupMenu`, `EditTokenDialog` among them) and
from none on the server: `ServerMessageHandler` handles `PUT_TOKEN_MSG`,
`EDIT_TOKEN_MSG` and `UPDATE_TOKEN_PROPERTY_MSG` with no owner, role or policy
check (`grep -n 'isOwner\|playerOwns\|isGM(\|getRole'` over it prints
nothing), so ownership changes travel as whole-token messages any client may
send, and with strict token management off ownership is not enforced at all.
`Player.Role` is `PLAYER` and `GM`, decided once in `ServerHandshake` by which
password the client answered (`getPlayerWithRole(..., Role.GM)` or
`Role.PLAYER`); there is no spectator, no agent, no join door, no promotion.
There is no adjacency or reach rule; door-like actions are macros.
`Token.removeOwner` is called from `TokenPopupMenu` alone, never from a
disconnect path, which is the durable-control ruling already recorded. So: the
membership predicate is borrowed and already in place; the server-side
per-command table, the player rules, the adjacency rule and the sight gate have
no MapTool counterpart; the client-side model is rejected, as rule 9 says
(`internal/gateway/seat.go`). The verifier re-ran the search by command in
`~/dev/RPTool/maptool` (`grep` over `model/Token.java`, `client/AppUtil.java`,
`server/ServerMessageHandler.java`, `server/ServerHandshake.java`,
`model/player/Player.java`; `grep -rl` for `isOwner(`/`playerOwns(` callers).
Where this is written is D3.

## Verification, check by check

1. **Every path resolves — by command.** The four production files and the
   five test files exist. Every symbol the ticket names is declared where it
   says: in `authz.go` `commandRoles`, `Authorize`, `authorizePlayer`,
   `playerRules`, `errUndecided`, `unrestricted`, `mayWorkDoor`,
   `authorizeTokenOwnership`, `authorizeActorOwnership`, `controls`,
   `authorizeSelfRevoke`, `authorizePromotionTarget`, `commandName`,
   `ErrUnauthorized`; `validateGrantActorControl` in `grant_validate.go`;
   `validateAddActor` in `add_actor_validate.go`; `answerCommand`,
   `authorize`, `handleSetViewpoint`, `handleCommand`, `describeBlockage` in
   `server.go`; `MayPerch` in `viewpoint.go` (`grep -n '^func'`). The four
   source tickets exist with the sections named (`## 4. Roles & authorization`
   in the api-gateway ticket, `### 5.3 Authorisation` in the presence ticket,
   `## 6. Movement and doors` in the maps ticket, `### 3.1a How promotion
   reaches the wire` in the joining ticket). Commit `b2445ab` exists ("The
   player half of authorization fell through to 'allowed'"). Rows VTT-025,
   VTT-027, VTT-028, VTT-031, VTT-032, VTT-033, VTT-038 exist, and
   `authz_test.go` carries five citation lines: VTT-028 above
   `TestAuthorizeTableAllCommandsAllRoles`, VTT-025 above
   `TestPromotionMayOnlyTargetPlayerOrSpectator` and
   `TestAnAgentMayNotPromoteAnyoneToDMOrAgent`, VTT-027 above
   `TestASpectatorCannotPromoteItself`, VTT-038 above
   `TestNoEventPayloadNamesARole`. SPEC-007, SPEC-009, SPEC-010, SPEC-011,
   SPEC-012 and ADR-011 exist; `docs/specifications/013*` does not.
2. **"Done" is an observation — by command where a command exists.** Item 2's
   three greps each print 1 today. Item 1: the file does not exist. Item 3:
   `task check:requirements-chain` prints `137 rows, 193 test files, 6
   specifications; every citation resolves and every row's evidence holds`.
   Item 4: `--report` prints `banned 29 blocks>6 13` for `authz.go`, `banned 4
   blocks>6 1` for `grant_validate.go`, `banned 6 blocks>6 2` for
   `add_actor_validate.go`, `blocks>6 29` for `server.go`. Item 5 is an
   invariant (D11 holds it at every task). Item 6 is the gate. The half of
   items 1 and 4 that says every surviving block is a warning, a pointer or a
   doc sentence is Phase 4b's reading, as the ticket says.
3. **Each rule is breakable — a reading.** D5's table names, per candidate,
   the test that goes red and the one edit that reds it. Two of the ticket's
   sentences are not breakable by their own edit and are refused as rows
   (D5: the unknown-token and unknown-actor refusals, which fall out of the
   membership rule; and the two tables' agreement, which is a check on the
   code's shape). One arm has no test at all (`authorize`'s `campaign
   unavailable`; gap 2).
4. **Scope matches the claim — by command, then a reading.** `Authorize` is
   called from `server.go`'s `authorize` and from tests; `validateGrantActorControl`
   and `validateAddActor` from `handleCommand` and their own tests;
   `answerCommand` from `serve`; `handleSetViewpoint` and `handleCommand` from
   `answerCommand`; `describeBlockage` from `handleCommand` and
   `server_internal_test.go`; everything else in `authz.go` from `authz.go`
   and `export_test.go`. `commandRoles` is read by `metadata.go`,
   `viewpoint.go` and `project.go` (by name, in comments and one
   `HasRoleCellsForTest`); `ErrUnauthorized` by `viewpoint.go` and
   `grant_validate.go`'s test. No production line changes, so callers do not
   widen the work; what widens it is a pointer INTO a block being cut from a
   file the ticket does not list: `server_internal_test.go`'s doc of
   `TestDescribeBlockageRewritesTheTwoNonProseReasonsAndPassesTheRestThrough`
   says "see its own doc comment on the task-5 review finding", aimed at
   `describeBlockage`'s block (gap 1, Q5). Every other pointer from outside
   the nine files is by symbol or file name and survives (the list is under
   Measurements). The ticket's sixteen blocks and 240 lines are exact.
5. **No recorded decision is contradicted — a reading.** SPEC-007 ("What a
   command refuses", "What appends to the log"), SPEC-009 ("Promotion is a
   command", its Consequences), SPEC-011 (the read loop, the pump, `perches`),
   SPEC-012 (the two batch handlers; the HTTP role maps are not cells of
   `commandRoles`) and ADR-011 were read against the ticket. None is
   contradicted and none is overturned; each sentence they hold about this
   area is true of `664cacf`. Where SPEC-007 already states a rule this
   ticket lists (the grant's kind; the three DM-and-agent-only door commands;
   `revoke_actor_control`'s player half), SPEC-013 points and does not
   restate (D2).
6. **The records the work moves are named — by command, then a reading.** The
   section exists and reads `New: the gateway's authorization ...`. The
   reading: the work changes no behaviour, so no specification's sentence goes
   stale; SPEC-009's Status naming `authz.go` and SPEC-012's sentence about
   `commandRoles`' keys stay true. `None.` would have been wrong and was not
   written.

## Measurements this plan stands on

All at `664cacf`, by command, run by the verifier.
`python3 tools/check-comments.py --report`, the checker's own `measure`
(`tag = cc.register_tag(); cite = cc.cite_pattern(tag); measure(lines, False,
cite)`) and `tools/comment-ceilings.txt`:

| File | Comment / non-blank | Share | Ceiling | Banned | Blocks > 6 | Cites |
|---|---|---|---|---|---|---|
| `authz.go` | 230 / 439 | 52.4 | 52.4 | 29 | 13 | 0 |
| `grant_validate.go` | 46 / 60 | 76.7 | 76.7 | 4 | 1 | 0 |
| `add_actor_validate.go` | 132 / 156 | 84.6 | 84.7 | 6 | 2 | 0 |
| `server.go` | 681 / 1,277 | 53.3 | 53.4 | 46 | 29 | 0 |
| `authz_test.go` | 452 / 1,184 | 38.2 | 38.2 | 44 | 20 | 5 |
| `grant_validate_test.go` | 37 / 91 | 40.7 | 40.7 | 3 | 2 | 0 |
| `add_actor_validate_test.go` | 121 / 248 | 48.8 | 48.8 | 8 | 8 | 0 |
| `server_test.go` | 605 / 2,182 | 27.7 | 27.8 | 34 | 27 | 19 |
| `server_visibility_test.go` | 325 / 1,088 | 29.9 | 29.9 | 22 | 20 | 1 |
| `server_internal_test.go` | 504 / 1,350 | 37.3 | 37.4 | 27 | 29 | 10 |

The ticket's figures are these. `server.go`'s token count is 4,596 (the
read-surface report's count at `f2d11c2`; `git log f2d11c2..664cacf` over the
four files prints nothing). The other counts are Task 0's.

**`authz.go`, by block.** Thirty-two blocks; the package doc (15 lines, above
`package gateway`) is excepted from the bound and counted in the share. The
thirteen over the bound sit on: the `remove_actor` row of `commandRoles` (8),
the `add_narration`/`upsert_note`/`delete_note` rows (7), the
`grant_actor_control`/`revoke_actor_control` rows (7), the `set_viewpoint`
row (12), the `set_viewpoint` check inside `Authorize` (9), `authorizePlayer`'s
doc (11), `playerRules`' doc (27), `mayWorkDoor`'s doc (12),
`authorizeTokenOwnership`'s doc (8), `authorizeActorOwnership`'s doc (8),
`controls`' doc (16), `authorizeSelfRevoke`'s doc (7),
`authorizePromotionTarget`'s doc (11). Under the bound: `commandRoles`' doc
(4), the `remove_token` row (6), the `use_ability`/`remove_condition` rows
(6), the `load_adventure` row (5), the `load_map` row (5), the
`promote_participant` row (3), the door-command rows (3), the
`open_door`/`close_door` rows (5), `ErrUnauthorized`'s doc (1), `Authorize`'s
doc (5), the promotion check inside `Authorize` (3), the missing-rule arm
inside `authorizePlayer` (6), `playerRule`'s doc (2), the `add_narration`
entry of `playerRules` (6), `errUndecided`'s doc (4), `unrestricted`'s doc
(3), `abs`'s doc (3), `commandName`'s doc (2).

**`grant_validate.go`, by block.** One, `validateGrantActorControl`'s doc,
46 lines. **`add_actor_validate.go`, by block.** Three: `validateAddActor`'s
doc (116), the block inside its controller arm (12), the block above the id
check (4).

**`server.go`, the sixteen blocks in scope**, by the symbol each sits on or
in, with the line count `measure` gives: `describeBlockage`'s doc (51);
`answerCommand`'s doc (19), the `lookupErr` arm's block (4), the `isPerch`
arm's block (6); `authorize`'s doc (9); `handleSetViewpoint`'s doc (36);
`handleCommand`'s doc (8), the "map constrains PLAYERS" block above the
player branch (10), the sight block above `canSee` (39), the "AND ONLY THEN"
block above `Blocked` (4), the `grant_actor_control` block (16), the
`add_actor` block (13), the batch-dispatch block (8), the `remove_actor`
block (3), the `promote_participant` block (5), the `TokenMoved` backfill
block (9). Sixteen blocks, 240 lines, eleven over the bound; 29 − 11 = 18,
the ticket's figure.

**The three false sentences, against the code.** The package doc of
`authz.go` says the package "also READS one file", `mapByID`'s probe;
`handleArtFile` in `metadata.go` calls `os.OpenRoot(s.artDir)` on every
request, and SPEC-012 names both places. `Authorize`'s doc says the table is
followed by "one additional check for players moving tokens"; the body runs
`authorizePromotionTarget` and `MayPerch` for every role, then
`authorizePlayer`, whose `playerRules` holds seven entries, six of them
restricting. `handleCommand`'s doc says "only a persisted event/marker
produces ok=true"; `handlePromotion`, `handleJoinDoor` and
`handleRotateJoinLink` return `Ok: true` with no `Append`, and SPEC-007 lists
the four commands that append nothing.

**Every sentence of the ticket's first paragraph, against its symbol.**
`commandRoles` holds twenty-two keys (counted), each a map over
`identity.Role`, whose values are `dm`, `agent`, `player`, `spectator`
(`internal/identity/identity.go`); `commandName` returns `""` for an unset
oneof and `Authorize` refuses `!known || !roles[p.Role]`. The one
`RoleSpectator` cell is `set_viewpoint`'s. After the table, `Authorize` runs
`authorizePromotionTarget` when `name == "promote_participant"` (player or
spectator, else refused) and `MayPerch` when `name == "set_viewpoint"`, both
before `if p.Role != identity.RolePlayer { return nil }`. `authorizePlayer`
looks `name` up in `playerRules` and returns `errUndecided` when absent.
`playerRules`: `move_token` → `authorizeTokenOwnership` (token in
`st.Tokens`, its actor in `st.Actors`, `controls`); `use_ability` and
`remove_condition` → `authorizeActorOwnership` on the command's `actor_id`;
`revoke_actor_control` → `authorizeSelfRevoke` (`participant_id == p.ID`,
then `authorizeActorOwnership`); `open_door` and `close_door` → `mayWorkDoor`
(non-player → nil; any token on the command's scene whose actor `controls`
with `abs(dx) <= 1 && abs(dy) <= 1`); `add_narration` → `unrestricted`.
`controls`: `participantID == ""` → false; membership in
`GetControllerIds()`; an empty set matches nobody. `handleCommand`, in order:
`authorize` (`campaign.State()` nil → `gateway: campaign unavailable`;
`Authorize` error → its text); for a player's `move_token` whose token `st`
holds, `canSee(viewerFor(p), st, tok.SceneID, to)` else `gateway: cannot move
there — you cannot see that square`, then `st.Blocked` else `gateway: cannot
move there — ` + `describeBlockage(why)`; `validateGrantActorControl`
(`kind == UNSPECIFIED` refused); `validateAddActor` (`controller_id` non-empty
or `len(controller_ids) > 0` refused first, naming `grant_actor_control`;
`actor_id == ""` → nil; `kind == UNSPECIFIED` refused); dispatch of
`use_ability`, `load_adventure`, `load_map`, `remove_actor`,
`promote_participant`, `set_join_door`, `rotate_join_link`; `ToEvent`; the
`TokenMoved` backfill of `SceneId` and `From` from `st.Tokens`;
`campaign.Append`; `Ok: true, Sequence: seq`. `describeBlockage`: `scenery:
<kind>` → `something (a <artlib.Clip(kind, artlib.MaxFragment)>) is in the
way`; `unknown scene ...` → a constant; the rest unchanged. `answerCommand`:
`lookupErr != nil` → `gateway: identity unavailable`; `set_viewpoint` →
`handleSetViewpoint` (`authorize`, `perches.set`, `Ok: true`); else
`handleCommand`. `serve`'s read loop decodes, re-resolves through
`identity.Lookup`, and calls `s.answerCommand(now, err, cmd, perches)`.
All as the ticket says. One nuance the ticket compresses: a `move_token` from
a player naming an unknown token never reaches the sight gate, since
`authorizeTokenOwnership` refuses it first, so the gate's `known` guard is
only ever false for the DM and the agent, who are not gated.

**The table of commands by roles, verified cell by cell against
`commandRoles`.** This is what SPEC-013 states (D1):

| Command | dm | agent | player | spectator |
|---|---|---|---|---|
| `move_token` | yes | yes | yes, rule | no |
| `add_actor` | yes | yes | no | no |
| `place_token` | yes | yes | no | no |
| `remove_token` | yes | yes | no | no |
| `remove_actor` | yes | yes | no | no |
| `start_session` | yes | yes | no | no |
| `end_session` | yes | yes | no | no |
| `use_ability` | yes | yes | yes, rule | no |
| `remove_condition` | yes | yes | yes, rule | no |
| `add_narration` | yes | yes | yes, `unrestricted` | no |
| `upsert_note` | yes | yes | no | no |
| `delete_note` | yes | yes | no | no |
| `load_adventure` | yes | yes | no | no |
| `load_map` | yes | yes | no | no |
| `grant_actor_control` | yes | yes | no | no |
| `revoke_actor_control` | yes | yes | yes, rule | no |
| `promote_participant` | yes | yes | no | no |
| `set_join_door` | yes | yes | no | no |
| `rotate_join_link` | yes | yes | no | no |
| `open_door` | yes | yes | yes, rule | no |
| `close_door` | yes | yes | yes, rule | no |
| `set_viewpoint` | no | no | no | yes |

Twenty-two commands, four roles, the 88 cells `authzCases` writes out
literally and `TestAuthorizeTableAllCommandsAllRoles` asserts. Seven player
cells, seven `playerRules` entries; `TestEveryPlayerCommandHasARule` diffs the
two.

**Pointers into the blocks this sweep cuts**, found by `measure` over the
five test files and `server_internal_test.go`, each placed in its block with
the block's length:

1. `authz_test.go`, the 27-line body block inside
   `TestAnAgentMayNotPromoteAnyoneToDMOrAgent`: "its doc says it is 'checked
   for every role including the DM's own'", aimed at
   `authorizePromotionTarget`'s doc, which goes. Over the bound.
2. `authz_test.go`, the 4-line banner above `TestAuthorizePlayerUseAbilityOwnActorOK`:
   "see authorizeActorOwnership's doc comment". Under the bound.
3. `server_test.go`, the 5-line body block inside
   `TestABlockedMoveRefusalStopsGrowingWithTheSceneryKind`: "the sight arm's
   ... whose own comment says the order is the whole point". Under the bound.
4. `server_visibility_test.go`, the 2-line body block inside
   `TestAPlayerCannotStepWhereItCannotSeeButTheDMCan`: "see handleCommand's
   note on why this must not name the occupant". Under the bound.
5. `server_visibility_test.go`, the 11-line doc of
   `TestAPlayerCannotStepOntoTerrainItRemembersButCannotSee`: "which the
   rule's own doc comment names". Over the bound.
6. `server_internal_test.go`, the 13-line doc of
   `TestDescribeBlockageRewritesTheTwoNonProseReasonsAndPassesTheRestThrough`:
   "see its own doc comment on the task-5 review finding". Over the bound;
   outside the ticket's list (Q5).

Pointers by symbol or file that survive the sweep, not re-aimed: in
`authz_test.go` every mention of `mayWorkDoor`, `controls()`,
`commandRoles`, `playerRules`, `authorizeSelfRevoke`,
`authorizeTokenOwnership` and `Authorize` by name, including `authzCases`'
own comments; in `server_test.go` "describeBlockage's 'scenery: ' rewrite
(server.go)" above `TestAPlayerCannotWalkOntoBlockingSceneryAndTheRefusalNamesIt`,
"authz.go's mayWorkDoor" above `TestAPlayerMayOnlyWorkADoorTheyAreNextTo`,
"commandRoles' open_door/close_door rows" above `TestASpectatorMayNotWorkDoors`,
"server.go's handleCommand has no world-layer-specific branch" above
`TestNoteAndNarrationRejectionSurfacesCleanNotPoisoned`; outside the nine
files, `internal/gateway/convert.go` (`authz.go`'s `mayWorkDoor`),
`internal/gateway/map.go`, `project.go`, `export_test.go`,
`internal/artlib/artlib.go` (`describeBlockage` bounded), `internal/sight/sight.go`,
`internal/engine/apply.go`, `internal/harness/soak.go` and its tests,
`internal/mapdef/resolve_test.go`, `cmd/vtt/mcp_e2e_test.go`,
`client/src/view/doors.ts` ("authz.go's own commandRoles table grants
open_door/close_door to dm, agent and player only", true after the sweep),
`client/src/view/player.ts`, `client/src/commands.ts`, `client/src/view/dm.ts`,
`client/src/player.ts`. The `commandRoles` rows on `remove_token` and
`remove_actor` point the other way, at `authz_test.go`'s `authzCases` comment;
the pointer goes with the rows' blocks and the argument stays in the test file
(D9 gives its rule a home).

**Records and gates.** `docs/specifications/` holds 007 to 012; 013 is free.
`docs/requirements.md` has 137 rows, the last `VTT-137`; the chain gate's line
is quoted above. `python3 tools/check-comments.py main` ends `239 files, 0
added comment lines, 239 ledger rows; clean`, after the standing notice on
`cmd/vtt/library_test.go`. `python3 tools/check-doc-owner.py .` ends `79
files, every doc comment sits on its own function`. `gofmt -l
internal/gateway/` prints only `scenario_test.go`. `requirement-id` is on the
path (`~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`) and, run
bare, prints `a requirement needs its sentence, not just an id`. No test drives
`authorize`'s `campaign unavailable` arm (`grep -rn 'campaign unavailable'
--include='*_test.go' internal cmd` prints nothing). Tests in the five files
that observe this area and are cited by no row: `TestEveryClientCommandHasRoleCells`,
`TestEveryPlayerCommandHasARule`, `TestAPlayerCommandNobodyRuledOnIsRefused`,
`TestPerchingAppendsNothingToTheLog`, `TestNoteAndNarrationRejectionSurfacesCleanNotPoisoned`,
`TestNarrationForwardAnchorRejectedCleanConnectionIntact`,
`TestAuthorizeSpectatorMayNotPerchOnAnNpc`, `TestASpectatorMayNotPerchOnTheGoblinArcher`,
`TestDMGrantsControlOverTheWire`, and every `TestAuthorize*` test not named
under check 1; `TestDescribeBlockageRewritesTheTwoNonProseReasonsAndPassesTheRestThrough`
in `server_internal_test.go` likewise.

**`check:doc-owner`'s first-word rule.** It refuses a doc block above a
function whose first word names another function in the tree. The
single-word capitalised function names at `664cacf` (`grep -rhoE '^func
(\([^)]*\) )?[A-Z][a-z]+\('` over `internal` and `cmd`): `Accept`, `Accepted`,
`Actors`, `Append`, `Apply`, `Arm`, `Authorize`, `Blocked`, `Blockers`,
`Clear`, `Clip`, `Close`, `Compile`, `Dial`, `Error`, `Eval`, `Events`,
`Fold`, `Handler`, `List`, `Load`, `Lookup`, `New`, `Notify`, `Open`,
`Parse`, `Project`, `Refs`, `Resolve`, `Revoke`, `Roll`, `Run`, `Scenes`,
`Scopes`, `Snapshot`, `State`, `Step`, `String`, `Subscribe`, `Tokens`,
`Unwrap`, `Validate`, `Verify`, `Write`. `Authorize` is on the list, so no
warning above `authorizePlayer`, `authorizeTokenOwnership`,
`authorizeActorOwnership`, `authorizeSelfRevoke` or
`authorizePromotionTarget` may open with the word "Authorize" (D8). `Keep`,
`Do`, `Never`, `Pass`, `Check`, `Refuse`, `Answer`, `Ask`, `Read`, `Return`,
`Treat`, `Hold`, `Guard`, `Sort`, `Stamp`, `Derive` name no function.

## Constraints that bind every task

- `CLAUDE.md` rule 10 and SPEC-010: a comment is an imperative warning, a
  pointer, or the one-line doc sentence of an exported symbol. VTT-051 is
  held by the Phase 4b reading; VTT-050, VTT-052, VTT-053 and VTT-055 by
  `check:comments`; VTT-059 says a citation line moves no share.
- `CLAUDE.md` rule 2: no gate is weakened. The ledger only goes down, only
  through `--write-ledger`.
- `CLAUDE.md` rule 8, narrowed by rule 10 for code: a comment points at a
  specification by number, a requirement by id, a test or symbol by name, or
  a report by its `docs/` path; never a line number, a date, a commit hash, a
  plan, a task, `§`, `#NNN`, an ADR or a `.superpowers/` path. This plan and
  the report obey rule 8 in full: blocks are named by the symbol they sit on.
- `CLAUDE.md` rule 3: the contract is not touched; `CommandResult`'s fields
  and the `ClientCommand` oneof are SPEC-007's.
- `CLAUDE.md` rule 4: one fold. SPEC-013 says where a refusal runs and never
  moves one into `engine.Apply`.
- `CLAUDE.md` rule 5: SPEC-013 states the adjacency rule as squares on a
  grid and names no reach, movement cost or edition.
- `CLAUDE.md` rule 9: the answer above is recorded before any task runs.
- SPEC-008: ids come from `requirement-id`, after sign-off, never by hand.
- The `specification` skill's form and `catches.md`: five headings, present
  tense, no `Why`, no `Rejected`, no measurement, no line number, no
  past-tense account that is not about the code, no path outside the project,
  one decision per file, no claim taken from a comment without reading the
  code under it (item 8, the one to watch: every sentence of SPEC-013 starts
  life in a comment, and three of those comments are already known false),
  every "only/every/none" with its search named (item 10).
- The `requirements` skill: one thing, what not how, breakable, measurable,
  named by a check or knowingly OPEN; refuse more than you accept.
- SPEC-007, SPEC-009, SPEC-011 and SPEC-012 are not edited: SPEC-013 points
  at them (D2).
- The ticket: no code line changes (D11 is the check); the three files whole,
  the sixteen `server.go` blocks, the five test files' citation lines and the
  blocks a re-aimed pointer sits in, the register, the ledger.
- `internal/gateway/viewpoint.go`, `seat.go`, `map.go`, `convert.go`,
  `metadata.go`, `export_test.go`, `internal/engine`, `internal/artlib`,
  `internal/mcp`, `client/src`, `cmd/vtt` and the four source tickets are not
  touched. `docs/reports/` gains only this ticket's report.
  `server_internal_test.go` is touched only if Q5 says yes, and then for one
  block.

## Decisions this plan makes

**D1. SPEC-013's sections, each lifted from a named source and held by named
symbols.** Forced by ticket item 1 and the `specification` skill's step 3.
Title: `SPEC-013: Who may issue a command is decided on the server, per
command, against the fold`. Under "How it works", in this order, one bold-led
paragraph each:

| Section | Source (where it lives today) | Symbols that hold it | Rows (D5) |
|---|---|---|---|
| The one function and the table | `commandRoles`' doc and its row comments; `Authorize`'s doc; api-gateway ticket §4; `authzCases`' comment | `Authorize` is the one caller of the table; `commandName` maps the oneof arm to the key and answers `""` for an unset command; `commandRoles` keyed by the `ClientCommand` field name, the table above stated whole; `ErrUnauthorized` wraps every refusal; `TestEveryClientCommandHasRoleCells` walks the oneof; the HTTP role maps are not cells (SPEC-012, pointed at) | A, B, C, D |
| Refusal by default | `Authorize`'s body; `HasRoleCellsForTest`'s comment | `!known \|\| !roles[p.Role]` refuses a command with no row, an unset one, and a row with no cell for the role; the message names the role and the command | A |
| The two checks for every role | `Authorize`'s two body blocks; the `set_viewpoint` row's comment | after the table and before the player half: `authorizePromotionTarget` for `promote_participant` (player or spectator, else refused, the DM's own promotion included; SPEC-009 states the bound and the handler's second bound, pointed at); `MayPerch` for `set_viewpoint` (a party member only, or no actor; `viewpoint.go`, pointed at) | none new (VTT-025 cited; the perch's rows are the seat-and-perch ticket's, Q8) |
| The player half | `authorizePlayer`'s doc, the missing-rule arm's block, `playerRules`' doc, `errUndecided`'s and `unrestricted`'s docs; `b2445ab` | `p.Role != RolePlayer` returns before it; `playerRules` keyed like `commandRoles`; a player cell with no rule is refused by `errUndecided`, whose message names the command; `unrestricted` is an entry, not an absence; `authorizePlayer` takes the name `Authorize` derived with the command; `TestEveryPlayerCommandHasARule` diffs the two tables; `AuthorizeUndecidedForTest` reaches the arm | O |
| The rules, one by one | the six helpers' docs; presence ticket §5.3; maps ticket §6 | `authorizeTokenOwnership`, `authorizeActorOwnership`, `authorizeSelfRevoke`, `mayWorkDoor` with Chebyshev adjacency on one scene, `unrestricted`; each asks the fold's `st.Tokens` and `st.Actors` and nothing else | F, G, I, J, L, M |
| Control is membership | `controls`' doc; presence ticket §5.1 and §5.3 | `controls` reads `GetControllerIds()` and never `controller_id`; an empty set is nobody's, so DM-and-agent only; an empty participant id matches nothing, the last guard on the revoke path; control itself is an `ActorControlGranted` in the log (SPEC-009, pointed at) | G, L |
| What the DM and the agent are free of | `Authorize`'s early return; `mayWorkDoor`'s early return; `handleCommand`'s player branch; presence ticket §3.2; maps ticket §6 | no player rule, no adjacency, no sight gate, no terrain check; acting on an actor a player controls transfers nothing | K, N, V |
| The command path | `answerCommand`'s and `authorize`'s docs; `handleCommand`'s doc; `serve`'s read loop (SPEC-011, pointed at) | `serve` decodes, re-resolves through `identity.Lookup` (SPEC-009) and calls `answerCommand`; `answerCommand`'s three arms; `authorize` takes `campaign.State()` once and refuses on nil with `gateway: campaign unavailable`, then `Authorize`; `handleCommand`'s order, stated as a list: the player's move gate, `validateGrantActorControl`, `validateAddActor`, dispatch, `ToEvent`, the backfill, `campaign.Append`, `Sequence`; every refusal an ok=false `CommandResult` on a connection that stays | Z, Y |
| The player's move gate | the sight block and the "AND ONLY THEN" block in `handleCommand`; the "map constrains PLAYERS" block; maps ticket §6 | for a player's `move_token` whose token the fold holds: `canSee(viewerFor(p), st, tok.SceneID, to)` first, refused with the one string `gateway: cannot move there — you cannot see that square` whatever stands there; then `st.Blocked`, refused with `gateway: cannot move there — ` + `describeBlockage(why)`; what "can see" means is `seat.go`'s (`canSee`, `viewerFor`, `Projector`), the projection's, pointed at | U1, U2, W |
| `describeBlockage` | its doc | rewrites `scenery: <kind>` to `something (a <kind>) is in the way` with the kind clipped by `artlib.Clip` to `artlib.MaxFragment`, rewrites `unknown scene ...` to a constant naming no id, passes every other `Blocked` reason through; the kind is the one author-written string a refusal renders; `engine`'s reasons are `terrain.go`'s and a new one that interpolates a map file's text needs the same clip | W, X |
| The two validators | `validateGrantActorControl`'s doc; `validateAddActor`'s doc and its two body blocks; the two `handleCommand` blocks; SPEC-007's grant sentence (pointed at) | after `Authorize`, for every role, before anything is written: a grant whose `kind` is `UNSPECIFIED` is refused and any other value the enum carries is accepted; an `add_actor` whose actor names a controller by `controller_id`, by `controller_ids`, or as a declared empty set is refused first, naming `grant_actor_control`; one with no `actor_id` is left to the fold, which names the id; one whose `kind` is `UNSPECIFIED` is refused, and a value the enum does not define is accepted; `engine.Apply` refuses the controller shape too and does not refuse a kindless `ActorAdded` | Q, R, S, T |
| What is dispatched and what becomes one envelope | the dispatch blocks in `handleCommand` | `use_ability` and `load_adventure` (SPEC-012), `load_map` (`handleLoadMap`, `map.go`, no record yet) and `remove_actor` (`handleRemoveActor`) yield a batch; `promote_participant`, `set_join_door` and `rotate_join_link` go to their handlers and append nothing (SPEC-007; the handlers are SPEC-009's); every other command becomes one envelope through `ToEvent`, a `TokenMoved` gaining `SceneId` and `From` from the fold's token before `campaign.Append`; `set_viewpoint` never reaches `handleCommand` | Y |
| `set_viewpoint` | `handleSetViewpoint`'s doc; the `isPerch` block | `answerCommand` routes it to `handleSetViewpoint`, which runs `authorize` and `perches.set(actorID)` and answers ok=true; it appends nothing (SPEC-007) and applies nothing: the pump takes the perch from `perches` and projects (SPEC-011); ok means the shoulder is recorded, not that frames are sent | none (Q8) |
| What this record does not decide | the ticket's "What could not be established" | promotion's bounds and re-resolution (SPEC-009); `MayPerch`'s body (`viewpoint.go`); what a seat can see (`seat.go`); the batch handlers (SPEC-012, `map.go`, `handleRemoveActor`); the door and promotion handlers (SPEC-009); what the fold refuses (`engine.Apply`) | none |

Each sentence names the symbol that holds it, is checked against the code
before it is written and re-read by Phase 4b (D16). Status: `Accepted.
Implemented by internal/gateway/authz.go, grant_validate.go,
add_actor_validate.go and server.go (answerCommand, authorize,
handleSetViewpoint, handleCommand, describeBlockage), against
internal/engine's State; pinned by internal/gateway/authz_test.go,
grant_validate_test.go, add_actor_validate_test.go, server_test.go,
server_visibility_test.go and server_internal_test.go.` "Principles served"
says what SPEC-009, SPEC-011 and SPEC-012 say: no blueprint; the principle
missing from the record rather than absent from the system (here: who may do
what is decided on the server, per command, against the fold as it stands,
never by a client and never once at connection time). "Consequences" holds
what a client and a tool author are bound by: a spectator's only command is
`set_viewpoint`; a player acts only through actors that count them among
their controllers, and an actor nobody controls is the DM's; a player works
a door from beside it and moves only where they can see; a refusal is an
ok=false result on a connection that stays, and its text is safe to show; a
grant and an `add_actor` say what the actor is; an `add_actor` never confers
control; the DM and the agent are bound by the table's cells and by
promotion's bound and by nothing a player rule says. "Requirements" lists the
ids the sort dispenses (D6) and nothing else.

**D2. What SPEC-013 points at and does not restate.** Forced by `catches.md`
item 13 and the caller's brief. SPEC-009 owns promotion's two bounds
(`authorizePromotionTarget` in `Authorize`, the target's current role in
`handlePromotion`), the re-resolution before every command and on delivery,
`answerCommand`'s `gateway: identity unavailable` arm (VTT-033), the door
commands' handlers, and that control is an `ActorControlGranted` in the log;
SPEC-013 names each symbol once and says "SPEC-009". SPEC-007 owns which
commands append nothing, the `Sequence` convention, the `actor_role` stamp,
and states the grant's kind rule and `revoke_actor_control`'s player half as
prose; SPEC-013 states the validator's mechanism and the player rule's
mechanism (its subject) and points at SPEC-007 for the wire's promise. SPEC-011
owns `serve`, the read loop, the pump and `perches`; SPEC-013 says the read
loop hands each command to `answerCommand` and that the pump applies a perch,
and points. SPEC-012 owns `handleUseAbility` and `handleLoadAdventure` and the
sentence that the HTTP role maps are not cells of `commandRoles`; SPEC-013
points at it for both. `viewpoint.go`'s `MayPerch` and `seat.go`'s `canSee`
and `viewerFor` are named as the symbols `Authorize` and `handleCommand` call,
with what each is asked, and their bodies are the seat-and-perch and
projection tickets'. ADR-011 is frozen evidence and is not cited for the
present tense. `map.go`'s `handleLoadMap` and `server.go`'s
`handleRemoveActor` are named as batch handlers with no record yet.

**D3. Provenance stays out of the record.** Forced by `catches.md` items 3
and 12. The MapTool answer above is the rule-9 answer and lives in this plan
and the report. SPEC-013 states membership in `controller_ids` as a fact
about this system and names no other project. Q1.

**D4. The three false sentences, and what replaces each.** Forced by ticket
item 2. The package doc of `authz.go` becomes the one sentence `go doc`
prints for the package, `Package gateway is the WebSocket and HTTP gateway
over vtt.v1 commands and events: authorization, conversion, the connection,
the read surface and the seat's projection.`, or a shorter true one, with no
second paragraph; SPEC-012 already names the two request-time filesystem
reads, and SPEC-013 says `Authorize` does no I/O with `go vet`'s import list
as the search (item 10). `Authorize`'s doc becomes `Authorize decides whether
p may issue cmd against st: the role table, the two checks for every role,
then the player rules (SPEC-013).` `handleCommand`'s doc becomes a pointer,
`handleCommand runs the command path SPEC-013 states, in its order.`, plus the
warning D8 names. `grep -c` prints 0, 0, 0.

**D5. The sort's starting point is the table below; nothing is an id until
sign-off.** Forced by SPEC-008 and the `requirements` skill. Rows are lettered
so nothing here reads as an id. Each line: the rule as a what; the outcome
proposed; the evidence, verified by the verifier against the test bodies; the
one edit that reds it. The reading after sign-off confirms or overrides each
and reports what it refused.

| # | Rule (what) | Proposed | Evidence (verified against the body) | The edit that reds it |
|---|---|---|---|---|
| A | A command with no row in the table, an unset command included, is refused for every role. | accept | `TestAuthorizeUnknownCommandDeniedForEveryRole` (an empty `ClientCommand`, four roles) | `!known \|\|` deleted from `Authorize` |
| B | A spectator may issue `set_viewpoint` and no other command. | accept | `TestAuthorizeTableAllCommandsAllRoles` (twenty-two spectator cells, one true), `TestSpectatorCommandDenied` (`end_session` over the wire), `TestASpectatorMayNotWorkDoors` | `RoleSpectator: true` added to any other row, or removed from `set_viewpoint`'s |
| C | The DM and the agent may issue every command but `set_viewpoint`. | accept, Q2 | `TestAuthorizeTableAllCommandsAllRoles` (forty-four cells) | any `RoleDM` or `RoleAgent` cell deleted |
| D | A player may issue none of `add_actor`, `place_token`, `remove_token`, `remove_actor`, `start_session`, `end_session`, `upsert_note`, `delete_note`, `load_adventure`, `load_map`, `grant_actor_control`, `promote_participant`, `set_join_door`, `rotate_join_link` and `set_viewpoint`, an actor or token the player controls included. | accept, Q2; this is the home the ticket says the `authzCases` argument lacks: `commandFor` names `a1` and `t1`, which the fixture gives the player, so the `remove_actor` and `remove_token` cells are false against the strongest case | `TestAuthorizeTableAllCommandsAllRoles` | `RolePlayer: true` on any of the fifteen |
| E | A player may move a token only when they are among its actor's controllers; a second controller may, a non-controller may not. | accept | `TestAuthorizePlayerOwnTokenOK`, `TestAuthorizePlayerOtherTokenDenied`, `TestAuthorizeSecondControllerMayMoveTheToken`, `TestAuthorizeNonControllerStillDeniedWhenActorIsShared`, `TestPlayerOwnershipDenialNoBroadcast` (over the wire) | `!controls(actor, p.ID)` made `false` in `authorizeTokenOwnership` |
| F | An actor with no controllers is denied to every player, for a move, an ability and a condition alike. | accept — the ticket's third candidate split; the MapTool wildcard refused | `TestAuthorizePlayerControllerlessActorTokenDenied`, `TestAuthorizePlayerUseAbilityControllerlessActorDenied` | `controls` answering true for an empty set |
| G | A player may use an ability with, or remove a condition from, only an actor they control. | accept | `TestAuthorizePlayerUseAbilityOwnActorOK`, `TestAuthorizePlayerUseAbilityOtherActorDenied`, `TestAuthorizePlayerRemoveConditionOwnActorOK`, `TestAuthorizePlayerRemoveConditionOtherActorDenied` | `"remove_condition": unrestricted` |
| H | A player may revoke only their own control, and only of an actor they control. | accept | `TestAuthorizePlayerRevokeSelfOK`, `TestAuthorizePlayerRevokeOtherParticipantDenied`, `TestAuthorizePlayerRevokeSelfOnAnActorTheyDoNotControlDenied` | the `!= p.ID` refusal deleted from `authorizeSelfRevoke` |
| I | The DM and the agent may act as, and grant or revoke control of, an actor a player controls, and the player keeps control. | accept | `TestAuthorizeDMMayActOnAnActorAPlayerControls`, `TestAuthorizeDMMayRevokeAnotherParticipantsControl`, `TestDMGrantsControlOverTheWire` | `if p.Role != identity.RolePlayer { return nil }` deleted from `Authorize` |
| J | An empty participant id controls nothing. | accept | `TestAuthorizeEmptyParticipantMatchesNothing` | the `participantID == ""` guard deleted from `controls` |
| K | A player may work a door only with a controlled token on the door's scene at most one square away on each axis, so a diagonal neighbour counts. | accept, one row: one predicate, seven directions | `TestAuthorizePlayerMayWorkAdjacentDoor`, `TestAuthorizePlayerMayNotWorkDistantDoor`, `TestAuthorizePlayerMayWorkAdjacentDoorAwayFromTheOrigin`, `TestAuthorizePlayerMayNotWorkADoorItStandsWestOrNorthOf`, `TestAuthorizePlayerDoorAdjacencyIgnoresOtherScenes`, `TestAuthorizePlayerMayWorkDiagonallyAdjacentDoor`, `TestAuthorizePlayerMayNotWorkDiagonallyDistantDoor`, `TestAPlayerMayOnlyWorkADoorTheyAreNextTo` (over the wire, refused far and allowed near) | `<= 1` made `<= 2`; `abs(dx) <= 1 && abs(dy) <= 1` made a Manhattan sum |
| L | The DM and the agent work a door from anywhere. | accept — split from K, since its edit is `mayWorkDoor`'s own early return | `TestAuthorizeDMMayWorkDoorRegardlessOfTokenPosition` | `if p.Role != identity.RolePlayer { return nil }` deleted from `mayWorkDoor` |
| M | A player command the table allows and no player rule decides is refused. | accept | `TestAPlayerCommandNobodyRuledOnIsRefused` (through `AuthorizeUndecidedForTest`) | `return errUndecided(p, name)` made `return nil` |
| N | A grant that states no kind is refused, and a grant stating any kind the contract offers is accepted. | accept; SPEC-007's sentence stays, the row's id goes on SPEC-013's line (Q3) | `TestAGrantWithNoKindIsRefused`, `TestAGrantThatSaysWhatItIsGrantingIsAccepted`, `TestEveryActorKindTheContractOffersIsAcceptedByAGrant` | `== ACTOR_KIND_UNSPECIFIED` made `!= ACTOR_KIND_PARTY_MEMBER` |
| O | An `add_actor` that names a controller, by either field or as a declared empty set, is refused, naming the command that confers control; one naming no controller is accepted. | accept | `TestAddActorSeedingAControllerIsRefused`, `TestAddActorSeedingAControllerSetIsRefused`, `TestAddActorSeedingAnEmptyControllerSetIsRefused`, `TestAddActorWithoutAControllerIsAccepted` | `\|\| len(a.GetControllerIds()) > 0` deleted |
| P | An `add_actor` that states no kind is refused, and one stating a kind the enum does not yet define is accepted. | accept | `TestAddActorWithNoKindIsRefused`, `TestAddActorAcceptsAKindTheEnumDoesNotYetDefine`, `TestAddActorDeclaresAPartyMemberWithoutGrantingIt` | the kind refusal deleted |
| Q | An `add_actor` with no actor or no actor id is left to the fold, after the controller refusal and before the kind refusal. | accept | `TestAddActorWithNoActorAtAllIsNotRefusedHere`, `TestAddActorWithAnActorThatHasNoIdIsNotRefusedHere` | the `actor_id == ""` return moved above the controller check |
| R | A player may not move onto a square they cannot see; the DM and the agent may. | accept under SPEC-013 (D25) | `TestAPlayerCannotStepWhereItCannotSeeButTheDMCan`, `TestAPlayerCannotStepOntoTerrainItRemembersButCannotSee` | `!canSee(...)` made `false` |
| S | A player's refusal for a square they cannot see reads the same whatever stands on the square, and names no terrain. | accept under SPEC-013 (D25): the order sight-then-terrain is this record's | `TestAPlayerCannotProbeTheDarkWithMoveCommands` (four kinds, byte-equal, six forbidden words; the control names a seen wall) | the `Blocked` check moved above `canSee` |
| T | A player's move onto a blocked square is refused with the obstruction named. | accept | `TestAPlayerCannotWalkIntoAWallButTheDMCan` (the wall named), `TestAPlayerCannotWalkOntoBlockingSceneryAndTheRefusalNamesIt` (the exact string), `TestDescribeBlockageRewritesTheTwoNonProseReasonsAndPassesTheRestThrough` (Q5) | the `Blocked` arm deleted; the `scenery: ` rewrite deleted |
| U | The DM and the agent move a token onto any square, a wall and an unseen square included. | accept — "free for DM" for movement, split from L | `TestAPlayerCannotWalkIntoAWallButTheDMCan` (DM half), `TestAPlayerCannotStepWhereItCannotSeeButTheDMCan` (DM half), `TestAPlayerMayOnlyWorkADoorTheyAreNextTo` (the DM's move onto the boulder) | `if p.Role == identity.RolePlayer` deleted |
| V | An authored scenery kind in a refusal is bounded, and the refusal still carries it. | accept | `TestABlockedMoveRefusalStopsGrowingWithTheSceneryKind` | `artlib.Clip(kind, artlib.MaxFragment)` made `kind` |
| W | The event a `move_token` appends records the scene and the square the token left. | accept | `TestMoveTokenBroadcastBackfillsSceneAndFrom` | the backfill block's two assignments deleted |
| X | A command refused by the table, a player rule, a validator or the fold is answered ok=false, reaches no other connection, and the connection stays open for the next command. | accept, Q4 | `TestPlayerOwnershipDenialNoBroadcast`, `TestSpectatorCommandDenied`, `TestNoteAndNarrationRejectionSurfacesCleanNotPoisoned`, `TestNarrationForwardAnchorRejectedCleanConnectionIntact` | a refusal in `serve` made a `conn.Close`; `Ok: true` on the refusal |

Refused, or not this sort's, each with its reason: the unknown-token and
unknown-actor refusals (`TestAuthorizePlayerUnknownTokenDenied`,
`TestAuthorizePlayerUseAbilityUnknownActorDenied`) — deleting the `!ok` arm
alone leaves both tests green, because a missing token or actor has no
controllers and E and G refuse it; the two tests are cited under E and G
instead. "Every player cell has a player rule and every player rule has a
cell" (`TestEveryPlayerCommandHasARule`) and "every command the contract
offers has a row" (`TestEveryClientCommandHasRoleCells`) — checks on the
tables' shape, how not what; M is the observable rule, and SPEC-013 names both
tests as what holds the tables complete (Q6). The `campaign unavailable`
refusal in `authorize` — no test drives it; SPEC-013 states it; no row and no
OPEN row (D23, Q7). The perch appending nothing and reaching nobody
(`TestPerchingAppendsNothingToTheLog`) and the perch refused on a non-party
actor (`TestAuthorizeSpectatorMayNotPerchOnAnNpc`,
`TestASpectatorMayNotPerchOnTheGoblinArcher`) — `MayPerch`'s and the pump's,
the seat-and-perch ticket's (Q8). `TestNoEventPayloadNamesARole` — VTT-038,
SPEC-009's, already cited. `TestTheDMCanActuallyOpenTheDoor`,
`TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt` — `handleJoinDoor`'s,
SPEC-009's, uncited today and not this sort's. The four promotion rules —
VTT-025, VTT-026, VTT-027, VTT-036 exist and are cited. Five clauses of the
ticket's candidates stay prose: "an unset command included" is A's example
rather than a second rule; "the DM and the agent need no token" is L; "the
named actor" for `use_ability` is G's wording; "before it asks the terrain" is
S's edit; "the four batch commands, promotion and the door commands are
dispatched to their handlers" is SPEC-013's dispatch paragraph, held by the
existing tests of each handler and by no row here.

Twenty-four accepted or leaning against nine refused tests and five refused
clauses is more than the skill's caution expects, and the reading after
sign-off is asked to cut, not to add: each row above names a test that goes
red on its own edit, and the edits are distinct.

**D6. Rows are dispensed after sign-off, cited by citation lines only, and an
OPEN row has no citer.** Forced by SPEC-008, the read-surface plan's D6 and
the debt file's open entry on OPEN rows. Every row above names an existing
test, so none is expected OPEN; if the reading refuses a test as evidence and
keeps the rule, that row is `**OPEN — no test yet**` with no citation line and
the report names it (ticket item 3). A citation line is `// VTT-NNN` directly
above `func Test`, below any doc block, several ids on one line where a test
holds several; `TestAuthorizeTableAllCommandsAllRoles`' line grows from
`// VTT-028` to carry B, C and D's ids beside it. Evidence cells are
`internal/gateway/<file>#<Test>`, written by hand after the dispenser; the
chain gate refuses one that is wrong. SPEC-013's Requirements line is copied
from the register last.

**D7. `server.go`: the sixteen blocks by symbol, and what each becomes.**
Forced by ticket item 4. None of the five symbols is exported, so no doc
sentence survives; each block becomes a pointer or a warning of at most three
lines, or goes:

| Block (by symbol) | Becomes |
|---|---|
| `describeBlockage`'s doc (51) | `Keep the kind clipped: it is the one author-written string a refusal renders, and a new Blocked reason that interpolates a map file's text needs the same (SPEC-013).` |
| `answerCommand`'s doc (19) | `Never enqueue a frame here: the pump is the only producer of a connection's envelopes (SPEC-011).` |
| the `lookupErr` arm's block (4) | `// Refuse this command and keep the connection (SPEC-009, VTT-033).` |
| the `isPerch` arm's block (6) | `// A perch appends nothing and is answered here, not in handleCommand (SPEC-013).` |
| `authorize`'s doc (9) | `Refuse on a nil state: authorizing against no world is deciding from nothing (SPEC-013).` |
| `handleSetViewpoint`'s doc (36) | `Hand the perch to the pump and apply nothing here: a frame emitted from this goroutine lands inside a projected batch and the watcher's fold stops (SPEC-011, SPEC-013).` |
| `handleCommand`'s doc (8) | `handleCommand runs the command path SPEC-013 states, in its order.` and `Never close the connection or write to the wire here; serve owns transport (SPEC-011).` |
| the "map constrains PLAYERS" block (10) | `// The gate below is the player's; the DM and the agent author the world (SPEC-013).` |
| the sight block (39) | `// Ask sight FIRST and answer with one string: a refusal that varies with the occupant is a terrain oracle (TestAPlayerCannotProbeTheDarkWithMoveCommands).` |
| the "AND ONLY THEN" block (4) | `// Reached only for a square this player has been sent, so the obstruction may be named (SPEC-013).` |
| the `grant_actor_control` block (16) | `// Format validity, checked here for every role before anything is written; not a rule about who (SPEC-013).` |
| the `add_actor` block (13) | goes; the warning above covers both validators |
| the batch-dispatch block (8) | `// These four yield a batch and never a single envelope (SPEC-012, SPEC-013).` |
| the `remove_actor` block (3) | goes |
| the `promote_participant` block (5) | `// These three append nothing (SPEC-007).` |
| the `TokenMoved` backfill block (9) | `// Keep the backfill here: engine.Apply never reads SceneId or From, and this is the one place holding both the pre-move state and the envelope (VTT-NNN).` |

The `remove_actor` block's fact (one ordered batch, `TokenRemoved`s then
`ActorRemoved`) is SPEC-007's already. After the sweep `server.go` carries
eighteen blocks over the bound, on the `Server` struct's field docs
(`writeTimeout`, `maps`, `artDir`, `cellPx`), `WithMaps`, `WithMapsDir`,
`WithArtDir`, `WithCellPx`, `announcePresence`, `announceDeparture` (two),
`revoked`, `announcePromotion`, `handleRemoveActor`, `handleJoinDoor` (one, in
its body), `handlePromotion` (two), `credentialGone`; the gate does not refuse
them because no line is added to them.

**D8. The sweep's rules, repeating the read-surface plan's where they
apply.** Forced by rule 10, SPEC-010, and the identity plan's D1 to D8.

- *Three kinds and nothing else*: a warning is imperative, a verb first, the
  consequence in the present tense, at most three lines; a pointer is
  `SPEC-013`, `SPEC-007`, `SPEC-009`, `SPEC-011`, `SPEC-012`, `VTT-NNN`, a
  test name, a symbol name, or `docs/verification-debt.md`, and may close a
  warning or a doc sentence in parentheses; a doc sentence is the first
  sentence `go doc` prints for an exported symbol. Section banners go. Every
  fact a deleted block held that SPEC-013 does not state is either added to
  SPEC-013 (D9) or named in the report as dropped, with the reason.
- *Exported symbols keep one sentence*: `Authorize` and `ErrUnauthorized`
  (`ErrUnauthorized is wrapped by every refusal Authorize returns.` stays as
  it is, one line), and the package doc's first sentence (D4). Every other
  symbol in the three files is unexported and keeps no doc sentence:
  `commandRoles`, `authorizePlayer`, `playerRule`, `playerRules`,
  `errUndecided`, `unrestricted`, `mayWorkDoor`, `abs`,
  `authorizeTokenOwnership`, `authorizeActorOwnership`, `controls`,
  `authorizeSelfRevoke`, `authorizePromotionTarget`, `commandName`,
  `validateGrantActorControl`, `validateAddActor`.
- *A warning above a function opens with a word that names no function*: the
  list is in the measurements; `Authorize` is on it, so a warning above any
  `authorize*` helper opens with `Keep`, `Never`, `Pass`, `Do`, `Refuse` or
  `Check`, never with "Authorize is the only caller". `Ask`, `Read`,
  `Answer`, `Hold`, `Treat`, `Guard` are safe. A warning inside a body is not
  read by that gate.
- *The package doc*: `authz.go`'s is the package's own and stays one sentence
  (D4); the gate excepts it from the bound and counts it in the share.
- *A doc sentence is one sentence, on one line where it fits*; the wrap band
  is `SHORT, LONG = 55, 85` in `tools/check-comment-wrap.py`.
- *Test files*: citation lines (D6) and the re-aimed pointers with the blocks
  they sit in, sorted to the bound (D22). No other test-file edit; the doc
  blocks above tests, some of thirty lines with banned terms and two with bare
  mutant coordinates, are the test-prose sweep's.
- *No row is dispensed by the sweep itself*: rows come from D5's sort after
  sign-off, in the same commit.
- *What is left alone*: directives (none in the four files); `//` inside
  string literals (none; the refusal strings carry none); every block outside
  D7's list in `server.go`; `viewpoint.go`'s `MayPerch` block, which
  `Authorize`'s cut block points at and which the seat-and-perch ticket
  sorts.

**D9. Facts only a comment holds go into SPEC-013.** Forced by ticket item 1
and identity D7. The reading of each swept block asks whether SPEC-013's draft
states the fact; if not, and the fact is about the code now, it is checked
against the symbol and written in. The candidates are listed below. The
`authzCases` argument for the `remove_token` and `remove_actor` cells gets its
rule in row D and one clause in SPEC-013's table paragraph (removal authors
the board and the cast, so it takes `place_token`'s and `add_actor`'s cells,
not `move_token`'s); the argument in full stays in `authz_test.go` for the
test-prose sweep.

**D10. One commit for the change; the report in its own.** Forced by the band
(a file's deletions and its row land together), by the chain gate (a row's
evidence and its citation line land together, and neither hook runs
`check:requirements-chain` or `check:comments`), and by the pointers
(`SPEC-013` must have a target in the same tree). The commit carries the
ticket (untracked today), this plan, SPEC-013, the register, the four
production files, the five test files (and `server_internal_test.go` per Q5),
and the ledger. The pattern is `f2d11c2` then `664cacf`.

**D11. The comment-stripped comparison is a token stream.** Forced by identity
D11. The program is the identity plan's Task 0 listing
(`docs/superpowers/plans/2026-09-24-sweep-identity.md`: `go/scanner`,
comments dropped), built once in the scratchpad (`$S/codetokens/codetokens`,
with a `go.mod` beside it) and run from the repository root over the nine or
ten files:

    for f in internal/gateway/authz.go internal/gateway/grant_validate.go \
             internal/gateway/add_actor_validate.go internal/gateway/server.go \
             internal/gateway/authz_test.go internal/gateway/grant_validate_test.go \
             internal/gateway/add_actor_validate_test.go internal/gateway/server_test.go \
             internal/gateway/server_visibility_test.go internal/gateway/server_internal_test.go; do
      git show "664cacf:$f" > "$S/before.go"
      "$S/codetokens/codetokens" "$S/before.go" > "$S/before.tok" || echo "SCAN FAILED $f"
      "$S/codetokens/codetokens" "$f" > "$S/after.tok" || echo "SCAN FAILED $f"
      printf '%s %s/%s tokens ' "$f" "$(wc -l < "$S/before.tok")" "$(wc -l < "$S/after.tok")"
      cmp -s "$S/before.tok" "$S/after.tok" && echo same || echo DIFFERS
    done

Done reads ten `same` lines with `server.go` at 4596 on both sides and the
other counts as Task 0 records them. A run proves it ran by the counts; a zero
is a failed run. A `DIFFERS` on a test file means a string literal changed,
and Phase 4b names it.

**D12. The ledger, last, and seven rows, eight at most.** Forced by SPEC-010
(the band) and by `--write-ledger` lowering a row on any drop (`min(old,
now)`), which the connection report learned. After Phase 4b has settled,
`python3 tools/check-comments.py --write-ledger`, then `git diff
tools/comment-ceilings.txt` must show exactly these rows changed, each
lowered: `internal/gateway/authz.go`, `grant_validate.go`,
`add_actor_validate.go`, `server.go`, `authz_test.go`, `server_test.go`,
`server_visibility_test.go`, and `server_internal_test.go` only if Q5 says
yes. `grant_validate_test.go` and `add_actor_validate_test.go` gain citation
lines only, which move no share, so their rows do not move; a moved row there
means a block was edited that D22 does not name. No row moves at the base
today. A ninth changed row means a file changed that this plan does not name:
stop, name it, and ask before committing. The commit message lists the rows
old and new.

**D13. The deliberate breaks, one per check this work relies on.** Forced by
the dev-cycle's rule that a check is proven by a red, and ticket items 2, 3,
4, 5 and 6. In a scratch clone (`git clone --no-hardlinks` into the
scratchpad, the final `git diff HEAD` applied, the untracked files copied in,
committed there, that clone's `main` pointing at that commit): first each gate
exits 0 with its completion line; then each break is one edit, the finding
recorded verbatim, the inverse edit by hand (never `git checkout --`), `git
diff --stat` printing nothing before the next.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | `// VTT-999` directly above `TestAuthorizePlayerUnknownTokenDenied` in `authz_test.go` (cited under E, so the line gains a second id) | `check:requirements-chain`: `authz_test.go cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 | one new row's evidence entry re-pointed at `internal/gateway/codec_test.go#TestDecodeCommandRoundTrip` | `check:requirements-chain`: `does not carry the id, so the link walks one way only` |
| B3 | SPEC-013's Requirements line gains `VTT-999` | `check:requirements-chain`: `cites VTT-999 ... (specification citation)` |
| B4 | one pointer line, `// SPEC-013`, inside `validateGrantActorControl`'s body in `grant_validate.go`, between two code lines where no block can absorb it, after the ledger is written | `check:comments`: `internal/gateway/grant_validate.go: comment share X is above its ceiling Y and this change added a comment line to it (SPEC-010)`. Placed there for the arithmetic: about fourteen non-blank lines after the sweep, so one line moves the share by several points |
| B5 | `<= 1` made `<= 2` on the x axis in `mayWorkDoor` | D11's loop: `authz.go ... DIFFERS` (and row K's `TestAuthorizePlayerMayNotWorkDistantDoor` stays green while `TestAuthorizePlayerMayNotWorkADoorItStandsWestOrNorthOf` goes red, which is the row's own proof) |
| B6 | "one additional check for players moving tokens" re-inserted in `Authorize`'s doc | `grep -c 'one additional check' internal/gateway/authz.go` prints 1 (item 2's observation goes back toward today's) |
| B7 | the warning above `authorizeSelfRevoke` rewritten to open with `Authorize` | `check:doc-owner`: a doc comment naming `Authorize` above `authorizeSelfRevoke`; this proves the first-word rule D8 relies on, with the one function name that is also this file's subject |

**D14. Gate steps before the commit, in order, on a tree the review has
settled.** `gofmt -l internal/gateway/` prints only `scenario_test.go` (D21);
`go vet ./internal/gateway/`; `go test -count=1 ./internal/gateway/...` green;
D11's loop, ten `same`; `task check:comments` (expected at this point to
refuse the seven or eight files for the band until Task 10 writes the ledger,
and nothing else); `task check:doc-owner`; `task check:requirements-chain`
(`<137 + N> rows, 193 test files, 7 specifications; every citation resolves
and every row's evidence holds`); `task check:new-prose` (every test and
symbol name the sweep writes must resolve; `SPEC-NNN` is not read by that
gate, so a `SPEC-013` pointer is held by the reading); `python3
tools/check_mutation_test.py -q` and `python3 tools/check_ts_mutation_test.py
-q` (no key names any of the four files, so none moves); `task lint`. Then,
after Task 10, `task check` whole, once, on the final tree, launched in its
own session (`start_new_session=True`), and only after Phase 4b has settled,
so no edit lands mid-run; `uptime` first, since the e2e waits fail under load
(load was 2.4 at verification). Then the pre-commit hook's own set (lint, vet,
tier-1, arch, vocabulary, doc-owner, secrets, typecheck, review gate), with
`git add` and `git commit` in separate calls.

**D15. Phase 4a is skipped, with its reason.** Independent QA derives tests
from a specification to find behaviour the implementer got wrong. This change
has no behaviour: D11 shows every Go file's code identical to `664cacf`, and
what is added is prose (SPEC-013), register rows, citation lines and comment
deletions. What can be wrong is a sentence, and a reading holds that: VTT-051
is `**READING — Phase 4b**`, and SPEC-013's sentences are held the same way
(D16). The report records the skip under its own heading.

**D16. Phase 4b is the check for VTT-051 and for SPEC-013, sentence by
sentence.** The reviewer gets `git diff HEAD`, SPEC-007, SPEC-009, SPEC-010,
SPEC-011, SPEC-012, SPEC-013, the rows VTT-050 to VTT-059 and the new rows,
the four production files, the five test files, `server_internal_test.go`,
`viewpoint.go`, `seat.go` and `export_test.go`. For every surviving block it
names the kind (D8) and, for a warning, the code it guards and whether the
consequence is true of that code; for a pointer, that the target resolves;
for a doc sentence, that the symbol is exported and the sentence true. For
every deleted block: did it hold a fact now in no record? If yes, SPEC-013
(D9) or the report. For every SPEC-013 sentence: the symbol it names, and
whether the code under the symbol does what the sentence says — `catches.md`
item 8 is the one to watch, since every SPEC-013 sentence starts life in a
comment, and this plan already found three false ones; the table of
commands by roles is re-read cell by cell against `commandRoles`. For each
dispensed row: the edit that would red the test named. For the two rule-9
sentences that survive as warnings (`describeBlockage`'s, the sight block's):
that neither names MapTool.

**D17. What the report records.** Per file: comment lines and share before
and after, blocks deleted, blocks kept by kind; the sentences SPEC-013 took
from comments, each with its symbol; the sort — every row accepted with its
id and test, every candidate refused with one line; `server.go`'s remaining
blocks by symbol; the ledger rows old and new; D11's counts; the breaks'
finding lines; the gaps below as found or closed; the pointers outside the
files this change touches that now aim at cut text (D19), for the next
tickets; the sign-off answers. The report does not revise the reports of the
periods the cut blocks narrate.

**D18. Order of work: the record first, then the files it is pointed at
from.** SPEC-013 is drafted before any comment is cut, so every pointer the
sweep writes has a target and every deleted fact has a home to be checked
against. Then `server.go`'s sixteen blocks (two of them point into `authz.go`
and `viewpoint.go`), then `authz.go`, then `grant_validate.go` and
`add_actor_validate.go`; then the test files' re-aims and forced sorts (D22);
then, after sign-off of the sort, the rows and citation lines; the ledger last
(D12).

**D19. Things the ticket's scope leaves stale are left, and named.** The
`authzCases` argument in `authz_test.go`, no longer pointed at from
`commandRoles`; `TestARevokedSpectatorStopsSeeingTheTable`'s doc in
`server_test.go`, which says "commandRoles has no spectator row anywhere",
false since the `set_viewpoint` row, in a block over the bound this change
does not add to; the two bare mutant coordinates (`authz.go:152:15`,
`authz.go:164:10`) in the docs of `TestAuthorizePlayerMayWorkAdjacentDoorAwayFromTheOrigin`
and `TestAuthorizePlayerMayNotWorkADoorItStandsWestOrNorthOf`, stale already
(the first coordinate lands in `Authorize`'s `set_viewpoint` check today, not
in `mayWorkDoor`'s arithmetic) and outside the adjudication files where rule 8
tolerates them; `TestASpectatorMayNotWorkDoors`'
doc naming commit `092381a`; `TestAnAgentMayNotPromoteAnyoneToDMOrAgent`'s
body block, which the re-aim sorts (D22) and whose "measured" paragraph goes
with the sort; `TestEveryPlayerCommandHasARule`'s doc, whose "its reasoning
was written on commandRoles' own row" is past tense about a comment that will
no longer exist; `client/src/view/doors.ts`'s and `player.ts`'s sentences
about `mayWorkDoor`, true after the sweep and the client sweep's. All named
in the report as the test-prose and client sweeps'.

**D20. Nothing is cited from the four source tickets or from `b2445ab`.**
Their sections are the sources SPEC-013 lifts from (D1's table), and a
specification points at symbols, tests and other specifications. The tickets
are not edited; the commit is named in this plan and the report only.

**D21. gofmt.** The base prints only `scenario_test.go`, whose difference is
in code and predates this ticket; it stays. If a surviving list-shaped comment
makes gofmt object to a swept file, the list is rewritten as sentences:
gofmt's own output is not committed, because a reflowed comment is a comment
edit like any other and Phase 4b reads it.

**D22. The test-file blocks a re-aimed pointer forces to the bound.** Forced
by `check:comments` (a line added to a block over six lines is refused, and a
re-aimed line is an added line), which the connection and read-surface reports
record, and by ticket item 4 of "What it touches", which allows exactly this.
Found by `measure` over the six test files (the measurements above):

1. `authz_test.go`, the 27-line body block inside
   `TestAnAgentMayNotPromoteAnyoneToDMOrAgent` — "its doc says it is 'checked
   for every role including the DM's own'" — sorted to at most six lines: a
   warning that the agent row is the one this test pins (the DM-shaped
   bypasses are `TestPromotionMayOnlyTargetPlayerOrSpectator`'s) and a pointer
   to SPEC-009 and SPEC-013. Its "MEASURED" paragraph and its history go.
2. `server_visibility_test.go`, the 11-line doc of
   `TestAPlayerCannotStepOntoTerrainItRemembersButCannotSee` — "which the
   rule's own doc comment names" — sorted to the bound: a warning that this
   test pins a remembered square, not an unseen one, and changes if the rule is
   ever relaxed; pointer to SPEC-013.
3. `server_internal_test.go`, the 13-line doc of
   `TestDescribeBlockageRewritesTheTwoNonProseReasonsAndPassesTheRestThrough`
   — "see its own doc comment on the task-5 review finding" — only if Q5 says
   yes; then sorted to the bound: a warning that the assertion is exact
   equality because the passthrough arm satisfies a substring; pointer to
   SPEC-013.

Re-aimed without a sort, because the block is under the bound:
`authz_test.go`'s 4-line banner above `TestAuthorizePlayerUseAbilityOwnActorOK`
("see authorizeActorOwnership's doc comment" becomes "SPEC-013");
`server_test.go`'s 5-line body block inside
`TestABlockedMoveRefusalStopsGrowingWithTheSceneryKind` ("whose own comment
says the order is the whole point" becomes "SPEC-013 states the order");
`server_visibility_test.go`'s 2-line body block inside
`TestAPlayerCannotStepWhereItCannotSeeButTheDMCan` ("see handleCommand's note"
becomes "SPEC-013"). Not re-aimed, because they name a symbol or a file that
survives: every pointer under Measurements' surviving list. A sorted test
block follows D8's three kinds: what survives is a warning about the fixture
or the assertion, or a pointer; the arguments and the history go, and the
report names what went. `grant_validate_test.go` and
`add_actor_validate_test.go` hold no pointer into a cut block (theirs aim at
the proto doc comments of `ActorKind` and `Actor.controller_ids` and at
`TestEveryClientCommandConverts`' doc), so no block in them is touched.

**D23. `authorize`'s `campaign unavailable` arm has no test, and this ticket
writes none.** Forced by the ticket (no code line changes; test files get
citation lines and sorted blocks only). SPEC-013 states the arm; no row is
dispensed for it and no OPEN row either, since the read-surface plan's rule
that a rule narrowed to what its tests observe beats a row nothing can red
applies; the report names the gap for the next ticket that touches
`server_internal_test.go`'s tests, where a `Campaign` whose `State()` answers
nil is one fixture away. Not a debt entry: the ticket allows none, and this is
a coverage gap the report can carry. Q7.

**D24. Rows land under SPEC-013's Requirements line; SPEC-007's and SPEC-009's
untouched.** Forced by check 6 (`New:` and no path beside it) and D2. Row N's
observation is `validateGrantActorControl`'s, which is this record's subject,
so its id goes on SPEC-013's line and SPEC-007 keeps stating the promise; the
existing ids VTT-025, VTT-027, VTT-028, VTT-036 and VTT-038 stay on SPEC-009's
line and are not copied. Q3.

**D25. The sight gate's rows are this record's; what "can see" means is not.**
Forced by the ticket's first open question. Rows R and S observe `handleCommand`'s
seam and its order (sight before terrain, one string), which live in the
command path and nowhere else; the predicate behind `canSee` is `seat.go`'s
and the projection's. SPEC-013 states the seam and the order, names `canSee`
and `viewerFor`, and says the meaning of sight has no record yet. Q9.

**D26. `server_internal_test.go` joins the change for one block.** Forced by
check 4 and the read-surface report's deviation on `map_test.go`, which was
missed for the same reason (a pointer in a file the ticket did not list) and
landed as a separate commit. The one block is D22's third; the file gains no
citation line unless row T cites `TestDescribeBlockageRewritesTheTwoNonProseReasonsAndPassesTheRestThrough`,
which is Q5's second half. If Q5 says no, the pointer is named in the report
as the next sweep's, and row T cites the two `server_test.go` tests alone.

## Candidates the reading starts from

Read by the verifier at `664cacf`. A starting point, not a verdict.

**Facts for SPEC-013 that only a comment states today.** `Authorize` does no
I/O and imports `engine.State` only to answer the ownership and adjacency
questions. `commandName` answers `""` for an unset oneof, so an unset command
and an unknown one are refused alike and the refusal names `""`. The two
every-role checks run before the player half so that a DM is bound by the
promotion bound and a spectator, for whom the player half never runs, is
bound by `MayPerch`. `authorizePlayer` must be passed the name derived with
the command, since a mismatched pair judges a command by another's rule.
`unrestricted` is a decision written in the table so that an absence can be
told from an oversight. `mayWorkDoor` is spatial only: where a controlled
token stands, never reach or edition. `controls` reads the set and never the
mirror `controller_id`, which holds only the first controller. The empty-id
guard is the last line on the revoke path, since `"" != ""` passes
`authorizeSelfRevoke`. Naming yourself on an actor you do not control is a
no-op the fold would append, so it is refused before the log. The kind checks
are `UNSPECIFIED`-only, not an allowlist, so the enum may grow. The controller
refusal is answered here as well as in the fold so that the caller is told the
command that confers control; the kind refusal is here and nowhere else, since
a kindless `ActorAdded` is a legal state on a recorded event. A declared empty
controller set is refused because the caller meant to seed one. The
controller check runs before the id check so a command wrong in both is told
the one that is a misunderstanding of the model. The sight gate runs first so
that `engine.Blocked`, which answers for any square, never leaks unseen
terrain; the cost is that a remembered square out of sight cannot be walked
onto. `describeBlockage` clips the kind because a map's `objects[].kind` is
free text and `mapdef.Load` never reads it; the unknown-scene arm discards
its id. `answerCommand` writes no envelope: the pump is the only producer.
`handleSetViewpoint` applies nothing itself because two goroutines enqueuing
put a pump frame inside a perch batch; ok means the shoulder is recorded and
`perchBox.set` never blocks, so a superseded hop is skipped. A perch does not
survive a reconnect; the client re-sends it. `handleCommand` never closes the
connection or writes to the wire. The `TokenMoved` backfill is the one place
holding both the pre-move state and the envelope, and `engine.Apply` reads
only `To`.

**Warnings to keep, imperative, per file.** `authz.go`: above `commandRoles`,
`Keep every "who may issue" answer in this table: it is the one place the
question is asked (SPEC-013, TestEveryClientCommandHasRoleCells).`; inside
`Authorize` above the promotion check, `// Checked for every role, the DM
included: it bounds what a promotion may DO (SPEC-009).`; above the perch
check, `// Checked for every role: a spectator never reaches the player half
below (SPEC-013).`; above `authorizePlayer`, `Pass the name Authorize derived
with cmd: a mismatched pair judges a command by another's rule.`; inside it,
`// Refuse a player cell no rule decides; a fall-through here is the leak
TestAPlayerCommandNobodyRuledOnIsRefused pins.`; above `playerRules`, `Keep
this a table keyed like commandRoles, and write unrestricted rather than
leaving a name out: TestEveryPlayerCommandHasARule diffs the two.`; above
`mayWorkDoor`, `Keep this spatial: where a controlled token stands, never
reach or edition (CLAUDE.md rule 5, SPEC-013).`; above `authorizeTokenOwnership`,
`Check membership in controller_ids, never equality with controller_id: the
mirror holds one controller of several (SPEC-013).`; above `controls`, `Keep
the empty-id guard: it is the last refusal on the revoke path, where "" != ""
passes (TestAuthorizeEmptyParticipantMatchesNothing).`; above
`authorizeSelfRevoke`, `Refuse a self-revoke of control never held here: the
fold would append it as a no-op (SPEC-013).`; above `authorizePromotionTarget`,
`Keep this narrower than ParseRole: it must refuse "dm" (SPEC-009).`
`grant_validate.go`: above `validateGrantActorControl`, `Refuse UNSPECIFIED
and nothing else: an allowlist refuses a kind the contract may add
(TestEveryActorKindTheContractOffersIsAcceptedByAGrant).` and `Never wrap this
in ErrUnauthorized: a missing field is not a missing permission (SPEC-013).`
`add_actor_validate.go`: above `validateAddActor`, `Check the controller
before the id and the kind: a caller wrong in both must be told the one that is
a misunderstanding of the model (SPEC-013).`; inside the controller arm, `//
Refuse a declared-but-empty set too: the caller meant to seed a controller
(TestAddActorSeedingAnEmptyControllerSetIsRefused).`; above the id check,
`// Leave a missing id to the fold, which names it (VTT-NNN).` `server.go`:
D7's table.

**Text that goes on sight.** Every `spec §`, `Task N`, `task-N`, `C1`, `T2`,
`J3`, `J6`, every date, `measured`, `used to`, `until 2026-`, `an earlier
draft`, `review finding`, `Patrik:` quotations, every ADR number, every plan
name, every mention of `create_scene`, `retract_events`, `validateCreateSceneTerrain`,
`AuthorizeUndecidedForTest`'s first version, the gocyclo figure, the byte
measurements, the "THE HOLE IT CLOSED" and "THE DANGER WAS NEVER
MULTIPLICITY" narratives, the RPTool `clearAllOwners` aside, the
`EditTokenDialog.java:885` coordinate, every sentence that says what the code
does rather than what to keep, and every section banner.

Projection, not a target: `authz.go` near 14 percent, `grant_validate.go` near
15, `add_actor_validate.go` near 15, `server.go` near 39. The reading governs;
nothing is cut for a figure.

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository; the scratchpad, `$S`.

Build D11's program (the identity plan's Task 0 listing; a `go.mod` with
`module codetokens` beside it); run the loop (ten `same`, `server.go` at
4596). Run `python3 tools/check-comments.py --report | grep -E
'internal/gateway/(authz|grant_validate|add_actor_validate|server|authz_test|grant_validate_test|add_actor_validate_test|server_test|server_visibility_test|server_internal_test)\.go'`,
`task check:requirements-chain`, `python3 tools/check-comments.py main`,
`python3 tools/check-doc-owner.py .`, `gofmt -l internal/gateway/`, the three
`grep -c` of item 2, and `grep -c 'VTT-'` over the five test files; keep the
outputs. Confirm `requirement-id` is on the path.

**Done when:** the outputs match the measurements above (ten `same` with the
counts; `137 rows ... 6 specifications`; `239 files ... clean`; only
`scenario_test.go` from gofmt; `1 1 1`; citations 5, 0, 0, 19, 1);
`requirement-id` prints its one-line usage.

### Task 1 — SPEC-013

**Files:** `docs/specifications/013-authorization.md`, new.

Per D1, D2, D3, D4's sentences, D9's rule, D23's and D25's sentences; the
`specification` skill's steps 1 to 5 and 7 (`catches.md`, two passes at most).
Step 6's Requirements line is written `None yet; the sort of
2026-09-28-authorization-has-a-record-design.md fills it` until Task 8 and
then copied from the register.

**Done when:** the file has the five headings in order (`grep -c '^## '`
prints 5), no `Why`, no `Rejected`, no number that is a measurement, no line
number, no date outside a `docs/` path, no path outside the project, no other
project's name; the table of commands by roles matches D1's cell for cell;
every sentence under "How it works" names a symbol in the four files,
`viewpoint.go`, `seat.go`, `export_test.go`, `internal/engine`,
`internal/artlib`, `internal/campaign` or `internal/identity`, and a table of
sentence to symbol is kept for Task 9; `task check:requirements-chain` prints
`7 specifications`.

### Task 2 — `server.go`, the sixteen blocks

**Files:** `internal/gateway/server.go`.

D7's table, under D8. `grep -c 'only a persisted' internal/gateway/server.go`
prints 0.

**Done when:** D11 prints `same` for `server.go` with 4596 tokens; each of the
sixteen blocks is gone or at most three lines and holds only a pointer or
D7's warning; `gofmt -l internal/gateway/server.go` prints nothing;
`--report` shows `server.go` at `blocks>6 18`; `python3
tools/check-doc-owner.py .` still ends `79 files, every doc comment sits on
its own function`.

### Task 3 — `authz.go`

**Files:** `internal/gateway/authz.go`.

D8 whole; D4's package doc and `Authorize` doc; the warnings named above;
every fact of `playerRules`' doc, `controls`' doc, the `set_viewpoint` row's
comment and `authorizePlayer`'s doc checked against SPEC-013's draft, each gap
into SPEC-013 (D9) or the report's dropped list.

**Done when:** D11 prints `same` with Task 0's count; `--report` shows
`authz.go` with `banned 0` and `blocks>6 0`; `grep -c 'READS one file'` and
`grep -c 'one additional check'` print 0; `go doc ./internal/gateway
Authorize` and `ErrUnauthorized` each print one sentence; `go doc
./internal/gateway | head -1` prints the package sentence; `gofmt -l` prints
nothing for it; `check:doc-owner` still ends `79 files ...`.

### Task 4 — `grant_validate.go`, `add_actor_validate.go`

**Files:** those two.

D8 whole; the warnings named above; the two `UNSPECIFIED`-only arguments and
the fold-versus-boundary arguments checked against SPEC-013's validators
paragraph.

**Done when:** D11 prints `same` for both; `--report` shows each with `banned
0` and `blocks>6 0`; `gofmt -l` prints nothing for them; no doc sentence
stands above either unexported function.

### Task 5 — The test files' re-aims and forced sorts

**Files:** `internal/gateway/authz_test.go`, `server_test.go`,
`server_visibility_test.go`, and `server_internal_test.go` per Q5 (D22;
`grant_validate_test.go` and `add_actor_validate_test.go` are not touched
here).

D22's two or three sorts and three free re-aims, under D8's three kinds. No
citation line yet.

**Done when:** D11 prints `same` for the files touched; no block this task
touched exceeds six lines; `grep -n "own doc comment\|doc says\|handleCommand's
note\|rule's own doc\|whose own comment\|authorizeActorOwnership's doc"` over
the six test files prints nothing (the `server_internal_test.go` line per Q5);
every pointer written resolves to a symbol, a test or a specification;
`python3 tools/check-comments.py main` refuses nothing but the band (the
files' shares fell) and no banned term.

### Task 6 — Local gates, first pass

**Files:** none changed.

D14's list up to and not including `task check` whole.

**Done when:** each step exits 0 with its completion line, except
`check:comments`, which at this point refuses the seven or eight files for
the band and nothing else; `check:new-prose` reports no citation to a name the
tree never declared; `check:doc-owner` ends `79 files ...`.

### Task 7 — Sign-off of the sort

**Files:** none.

D5's table is presented with the reading's confirmations and overrides, each
override with its reason, and the questions below answered. Nothing is
dispensed before the answer.

**Done when:** each lettered row has one of: accept (with wording and
evidence), refuse (with reason), OPEN (with reason); Q1 to Q12 have answers.

### Task 8 — Rows and citation lines

**Files:** `docs/requirements.md` (by the dispenser, then evidence cells by
hand), `internal/gateway/authz_test.go`, `grant_validate_test.go`,
`add_actor_validate_test.go`, `server_test.go`, `server_visibility_test.go`
(citation lines only, D6; `server_internal_test.go` per Q5),
`docs/specifications/013-authorization.md` (the Requirements line, copied).

**Done when:** `task check:requirements-chain` prints `<137 + N> rows, 193
test files, 7 specifications; every citation resolves and every row's evidence
holds` with N the accepted count; an OPEN row, if any, has no citer (`grep -rn
'VTT-1NN' internal/` prints nothing); D11 prints `same` for every test file;
`--report` shows `cites` equal to the number of citation lines per file and
the files' shares unchanged from Task 5's.

### Task 9 — Phase 4b, the reading review

**Files:** whatever its findings touch among the above.

Per D16. Findings are fixed and the affected task's "done" is re-run. The
review settles before Task 10 starts; if the reviewer dies on a model's limit,
say so and re-dispatch with the same brief on `fable`.

**Done when:** the review record names every surviving block's kind, every
deleted block's outcome, every SPEC-013 sentence's symbol and verdict, every
table cell's verdict, every row's red-making edit, and reports no open
finding; D11 prints `same` for all ten files.

### Task 10 — The ledger

**Files:** `tools/comment-ceilings.txt`, by `--write-ledger` only.

Per D12.

**Done when:** `git diff tools/comment-ceilings.txt` shows exactly the rows
D12 names, each lowered; `task check:comments` ends `clean`; `--report | grep
-E 'internal/gateway/(authz|grant_validate|add_actor_validate)\.go'` prints
`banned 0` and `blocks>6 0` on all three lines and `server.go` at
`blocks>6 18`.

### Task 11 — The breaks and the whole gate

**Files:** none in the repository.

D13 in a scratch clone; then `task check` whole, once, per D14.

**Done when:** the clone's clean run exits 0 with each completion line, and
each of B1 to B7 produces the one red D13 names; `task check` exits 0 with
every step, `check:comments`, `check:requirements-chain`, `check:doc-owner`
and `check:mutation` among them, printing its own verdict.

### Task 12 — Commit, then the report

**Files:** the commit's, per D10; then
`docs/reports/2026-09-28-authorization-has-a-record.md`.

The commit message lists the ledger rows old and new, D11's counts, the rows
dispensed, and B4's finding line verbatim. After it: the report per D17 and
the `implementation-report` skill, in its own commit.

**Done when:** `git show --stat HEAD~1` lists the ticket, this plan, SPEC-013,
the register, the four production files, the five test files
(`server_internal_test.go` per Q5) and the ledger, and nothing else; `git
diff --stat 664cacf -- docs/reports/` lists only the new report; `git diff
--quiet 664cacf -- internal/gateway/viewpoint.go internal/gateway/seat.go
internal/gateway/map.go internal/gateway/convert.go
internal/gateway/metadata.go internal/gateway/export_test.go cmd/ contract/
client/src internal/engine internal/artlib internal/mcp` exits 0.

## Commits

| Commit | Carries | Gate steps it runs |
|---|---|---|
| C1 | the ticket, this plan, SPEC-013, the register, `authz.go`, `grant_validate.go`, `add_actor_validate.go`, `server.go`, the five test files (and `server_internal_test.go` per Q5), the ledger | D14's list by hand, `task check` whole (Task 11), then the pre-commit hook (lint, vet, tier-1, arch, vocabulary, doc-owner, secrets, typecheck, review gate) |
| C2 | the implementation report | pre-commit hook |

Push after C2: pre-push runs tiers 2 and 3 and the contract gates, about three
minutes; let it finish. No Go code changes, so no mutation key moves and
`check:drift` has no client change to compare.

## Gaps that travel with this plan

1. **A pointer into `describeBlockage`'s block sits in a file the ticket does
   not list.** `server_internal_test.go`'s doc of
   `TestDescribeBlockageRewritesTheTwoNonProseReasonsAndPassesTheRestThrough`
   says "see its own doc comment on the task-5 review finding", a 13-line block
   over the bound. The `map_test.go` shape from the read-surface change. D26
   and Q5.
2. **`authorize`'s `campaign unavailable` arm has no test.** SPEC-013 states
   it; D23 dispenses no row; Q7.
3. **The sight gate's home** is the ticket's own open question; D25 decides
   rows R and S are this record's and the predicate is not; Q9.
4. **`TestARevokedSpectatorStopsSeeingTheTable`'s doc is false**
   ("commandRoles has no spectator row anywhere") and sits in a 14-line block
   this change may not add a line to. Named for the test-prose sweep (D19).
5. **The two table-shape tests are uncited** (`TestEveryClientCommandHasRoleCells`,
   `TestEveryPlayerCommandHasARule`); D5 refuses them as rows and SPEC-013
   names them as the checks that hold the tables complete. Q6.
6. **The perch's tests are uncited** (`TestPerchingAppendsNothingToTheLog`,
   `TestAuthorizeSpectatorMayNotPerchOnAnNpc`,
   `TestASpectatorMayNotPerchOnTheGoblinArcher`); the seat-and-perch ticket's.
   Q8.
7. **Two bare mutant coordinates and one commit hash** stand in
   `authz_test.go`'s and `server_test.go`'s test docs, outside the
   adjudication files; the test-prose sweep's (D19).
8. **The `authzCases` argument loses its pointer** from `commandRoles` and
   keeps its text in the test file; row D and one clause in SPEC-013 are its
   home for the rule; the argument's length is the test-prose sweep's.
9. **Row count after the sort.** The ticket's "more than 137" holds for any
   N ≥ 1; the report states N.
10. **Item 5 is an invariant, not a done that fails today.** D11 holds it at
    every task.
11. **The unknown-token and unknown-actor refusals are not independently
    breakable** (D5's first refusal); their tests are cited under E and G, and
    SPEC-013 says the helpers refuse a token or actor the fold does not hold as
    a fact the membership rule implies.
12. **`handleSetViewpoint`'s cost sentence** (a perch does not survive a
    reconnect; the client re-sends it) is a client contract nothing in
    `internal/gateway` tests; it goes to SPEC-013's Consequences as prose
    held by the reading, or to the seat-and-perch ticket. Q10.

## Questions for sign-off

1. **Does SPEC-013 name MapTool?** Recommend no (D3): the record states
   membership in `controller_ids` as a fact about this system; the borrowed
   predicate and the rejected client-side model are the rule-9 answer in this
   plan and the report.
2. **Rows C and D, the DM-and-agent column and the player's refused cells, as
   rows or as SPEC-013 prose?** Recommend rows: each is one edit to one cell
   with one test red, VTT-028 already lists three commands in one row, and
   row D is the only home the ticket's `authzCases` argument gets. If no, the
   table stands in SPEC-013 alone and `TestAuthorizeTableAllCommandsAllRoles`
   keeps its one citation.
3. **Row N's id on SPEC-013's line, SPEC-007's grant sentence untouched?**
   Recommend yes (D24): the observation is `validateGrantActorControl`'s, and
   the ticket's "Specifications this moves" names no edit to SPEC-007.
4. **Row X, the refusal shape, under SPEC-013 or SPEC-011?** Recommend
   SPEC-013: the tests observe a refused command's result and the next
   command's success, which is the command path's promise; SPEC-011 states
   what ends a connection and this is the case that does not.
5. **Touch `server_internal_test.go` for the one re-aim, and cite its
   `describeBlockage` test under row T?** Recommend yes to both (D26): the
   file's row moves and the commit lists one more file; the alternative is a
   pointer known to be false landing on `main`, which the read-surface change
   did and then repaired in a second commit.
6. **Cite `TestEveryClientCommandHasRoleCells` and `TestEveryPlayerCommandHasARule`
   anywhere?** Recommend no: they hold the tables' shape, not a rule a client
   relies on; SPEC-013 names them under the table paragraph, and row M holds
   the observable half.
7. **The `campaign unavailable` arm: no row, an OPEN row, or a debt entry?**
   Recommend no row (D23): a row nothing can red is the register's failure
   mode, the ticket allows no debt entry, and the report names the gap for the
   next ticket that touches the internal tests.
8. **The perch's rows here or in the seat-and-perch ticket?** Recommend there:
   `handleSetViewpoint`'s block is this ticket's, but `MayPerch`'s body and the
   pump's application are not, and a row split across two records is two
   half-rows.
9. **Rows R and S under SPEC-013 (D25)?** Recommend yes: the seam and its order
   are `handleCommand`'s and nothing else's; when the projection has a record
   it states what `canSee` means and points back.
10. **The reconnect cost of a perch: SPEC-013's Consequences or the seat-and-perch
    ticket's?** Recommend SPEC-013's Consequences, one sentence, since the
    block that carries it is cut here and the sentence is true of the code
    (`perches` is per connection, in `serve`).
11. **One commit for the change, the report separately (D10)?** Recommend yes.
12. **If a swept file lands with every surviving line a true warning, pointer
    or doc sentence and a block still holds one over the bound?** Recommend
    the reading governs: stop, report the block, and let the ticket's writer
    decide, rather than cut a true warning for the bound.
