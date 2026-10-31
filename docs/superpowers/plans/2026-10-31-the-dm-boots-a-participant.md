# Plan: the DM boots a participant from the console

- **Ticket:**
  `docs/superpowers/specs/2026-10-31-the-dm-boots-a-participant-design.md`.
- **Verdict:** passes, with gaps G1 to G12 (below).
  - Verified 2026-10-31 by an agent that did not write the ticket,
    read-only against HEAD `275b449`.
  - Amended the same day for the owner's answers to Q1 to Q3. Q3's answer
    adds G11, G12, P13 to P18, D18 to D26 and Task 3; its probes ran in
    the same scratch copy.
  - Probes ran in a scratch copy of the tree outside the repository. P9's
    prototype is a scratch edit of `contract/vtt/v1/commands.proto` and its
    regeneration; nothing in the repository was edited but this file.
  - `task check`, `task e2e`, Stryker and gremlins were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-31-the-dm-boots-a-participant.md`.
- **Base:** main `275b449`.
- **Branch:** `feat/the-dm-boots-a-participant`.
- **Owner's rulings taken in:**
  - 2026-10-31, Boot: the console's Boot revokes the participant for good,
    as `vtt revoke` does. Their credential dies, they leave the roster,
    their name becomes free, and their open connections close at once. It
    works on a participant who is not connected. It cannot be undone, so it
    asks first. Rejected: disconnect only (MapTool's Boot); two buttons.
  - 2026-10-31, control: Boot also revokes their control of every
    character, one `ActorControlRevoked` in the log per character, so each
    stands unowned and can be granted to someone else. Rejected: leaving
    control in place.
  - 2026-10-21: a near-theoretical edge with a small consequence gets no
    code, no test, no debt entry, no specification sentence. Measure first.
  - 2026-10-15: a table has one DM in practice, the agent or a human, not
    both.
  - 2026-09-06: control is durable and nothing reaps it.
  - 2026-09-04: delete the old solution first.
  - 2026-10-26: the client's full mutation run is the owner's to ask for.
  - 2026-10-31, Q1, "DM och agent": the dm and the agent may remove a
    participant.
  - 2026-10-31, Q2, "Ja, kör som ett ärende": one ticket on one branch.
  - 2026-10-31, Q3, "Visa text och glöm behörigheten": the removed
    participant's window says they were removed, in place of Reconnect,
    and forgets the dead credential, so the share link works again in that
    browser.
- **Signed off:** 2026-10-31, with Q1 to Q3 answered as above. The
  amendment for Q3(c) was shown to the owner, and D22's reload case with
  its 503 confirmed: "Ja, täck reload med 503".
- **The ids, allocated at Task 0:** R1 to R15 are VTT-557 to VTT-571, F1 to
  F7 are VTT-572 to VTT-578, N442 is VTT-579, F5b is VTT-580 and N111 is
  VTT-581. VTT-442 and VTT-111 are withdrawn. Task 0's review reworded R2,
  R11, R13, R15, F5, F6 and F7 before their commit, split F5 in two (F5b)
  and found VTT-111 made false by F7 (N111); the rows below carry the new
  words.
- **The owner on a 401 at start, 2026-10-31:** a stored token can get 401
  because another campaign now runs on the same address. Asked whether the
  window should still say it was removed: "\"Removed\" i båda fallen". The
  window shows the same line on the 1008 close and on a first 401 from
  `/api/me`, and forgets the token either way.

## The verdict, check by check

1. **Every path resolves. By command** (P1).
   - Every file the ticket names exists.
   - Every symbol stands where it says:
     - `Revoke`, `Lookup`, `List`, `ErrInvalidToken` and `ErrNameTaken` in
       `internal/identity/identity.go`;
     - `newRevokeCmd` and its `--campaign` and `--id` in `cmd/vtt/revoke.go`;
     - `RevokeActorControl`, `PromoteParticipant` and `ClientCommand` in
       `contract/vtt/v1/commands.proto`;
     - `renderDMConsole`, "Who may do what", "Make player", "Make
       spectator", the row's `data-participant` and `known` in
       `client/src/view/dm.ts`;
     - `commandRoles` in `internal/gateway/authz.go`, `handlePromotion` in
       `internal/gateway/server.go`, `presenceRegistry.rejudge` in
       `internal/gateway/presence.go`;
     - `promote_participant` in `cmd/vtt/tools.json`.
   - `identity.Revoke` has one caller outside tests, `newRevokeCmd` (P2), as
     the ticket says. `vtt revoke` appends nothing.
   - `client/src/app.ts` holds no "credential no longer valid": no client
     code reads a close's code or reason (G5).
   - The MapTool symbols resolve at `f4b7fef6c` (P11).
2. **"Done" is an observation.** The tests the items describe do not exist
   yet, and no `ClientCommand` arm removes a participant (P1).
   - Items 1, 3, 4, 5 and 6 fail today for want of the command.
   - Item 3 fails today on its own terms too, by command: a connected
     spectator revoked as `vtt revoke` does is still connected 1.5 s later
     while nothing is appended (P3).
   - Item 2 has no control to cancel today.
   - Item 7 holds today; it is a guard. Item 8 is the gate.
3. **Each rule is breakable. A reading.** Each has an observation that goes
   red; see "Requirements" and the breaks. Two are thin as worded (G6): "at
   once" names no bound, and no rule says who may not issue a removal,
   though item 6 does.
4. **The scope matches the claim. By command, then a reading.**
   - A prototype of the oneof arm alone (P9) turns red: `toolgen`'s
     manifest test; the gateway's role-table and conversion tests; every
     MCP test P9 ran that builds a server, since `mcp.New` refuses a oneof
     arm with no tool; and the client's surface and builder tests.
   - So the MCP tool is not optional (G1), and the arm cannot land without
     its client control (Q2, D14).
   - Files the ticket does not name: G2.
   - `presence.go` does not change; the handler calls `rejudge` (D7).
5. **No recorded decision is contradicted silently. A reading.**
   - SPEC-009, SPEC-013 and SPEC-007 move, and the ticket names them.
   - SPEC-013's "The DM and the agent hold every row but `set_viewpoint`",
     and VTT-140, would have been overturned by "the DM only". The ticket
     left that open; Q1's answer keeps them.
   - The 2026-09-06 ruling holds: control is taken by the DM's explicit
     command and recorded in the log, not reaped.
   - SPEC-009's "Control of a character is not identity's" holds: the
     credential is revoked in identity, control in the log.
   - `client/src/view/dm.ts`'s head comment, "Exactly one of them asks
     first", becomes false (G10). It is a comment, not a record.
6. **The records it moves. By command, then a reading.**
   - The section is present, and SPEC-009, SPEC-013 and SPEC-007 resolve.
   - The reading: SPEC-011 says of `rejudge` "its caller is SPEC-009's";
     a removal is a second caller (G3). SPEC-015 says nothing a removal
     makes false. Q3's answer moves SPEC-022 and SPEC-012, which the
     ticket could not name (G11).

## The gaps this plan inherits

- **G1. The MCP tool is required, whatever Q1 says.** The ticket makes
  `tools/toolgen/main.go` and `cmd/vtt/tools.json` conditional on the agent
  being given the command. `TestManifestCoversAllCommandMessages` requires
  a manifest entry for every arm, and `mcp.New` refuses to start when an arm
  has no tool ("oneof fields with no tools.json entry", P9). Q1 decides only
  whether `commandRoles` lets the agent issue it.
- **G2. Files the ticket does not name.**
  - `contract/testdata/expected_tools.json`, the golden
    `TestToolsMatchGolden` reads; `contract/gen/tools/tools.json`.
  - `internal/gateway/authz_test.go` (`commandFor` and the table),
    `internal/gateway/convert_test.go` (`notConverted`).
  - `internal/mcp/tools_test.go` and `tools_internal_test.go`
    (`wantCommandToolNames`), and `getParticipantsDescription` in
    `internal/mcp/door_tools.go`, which names the tools that need an id.
  - `client/test/command-surface.test.ts` (`COMMAND_SURFACE`, whose comment
    on `dynamic` says it is true for one entry), `client/test/commands.test.ts`,
    `client/test/dm-view.test.ts`.
  - `cmd/vtt/webdist`, rebuilt by `task build:client`.
  - `tools/comment-ceilings.txt`, lowered by `--write-ledger` (D16).
- **G3. SPEC-011 moves by a word.** "its caller is SPEC-009's" becomes
  "its callers are SPEC-009's". SPEC-009's "every promotion signal it
  takes" becomes a promotion's or a removal's.
- **G4. The browser beat does not go in `client/e2e/joining.spec.ts`.**
  That file cannot run twice against one campaign
  (`docs/verification-debt.md`, Open debt), and a rerun is how a new e2e is
  checked for flakiness (`client/e2e/setup.ts`). D11 gives it a file of its
  own.
- **G5. What the removed participant's window shows, today** (P4, P8).
  - The pump closes each connection with 1008 and the reason "gateway:
    credential no longer valid". `Wire.connect`'s close handler reads no
    code and no reason, so the window says "closed", keeps the board it
    last drew, and offers Reconnect.
  - Reconnect dials with the dead credential and is answered 401.
  - The dead credential stays in `localStorage`, and a stored token wins
    over `?join=` in `boot`, so that browser cannot use the share link
    again until its site data is cleared.
  - All of this is equally true after `vtt revoke` today. Q3(c) changes
    it: D18 to D26, Task 3.
- **G6. Two rules are thin as worded.** "Close at once" is worded as
  "without waiting for a command of theirs or an append", which P3 makes
  observable. The issuer's bound is item 6's; R9 states it.
- **G7. Item 6's "the issuer" needs no check of its own.** Only a dm or an
  agent may issue a removal (Q1), so naming oneself is naming a dm or an
  agent, refused by D3's bound. A test names it.
- **G8. `identity.Revoke` does not refuse a participant already revoked**
  (P7). "Already revoked is refused" rests on the handler's `Lookup`, and K7
  pins it.
- **G9. The ticket's unestablished points, answered.**
  - The agent: Q1, the DM and the agent.
  - The removed window: Q3(c), D18 to D26.
  - The order of the two writes: D4, measured by P5.
  - A command in flight from the removed participant: D6.
- **G10. Every file touched sits within 0.1 of its comment ceiling** (P10),
  and `client/src/view/dm.ts`'s head comment becomes false. It is rewritten
  as a warning with a pointer (CLAUDE.md rule 10), within the ceiling.
- **G11. Q3's answer reaches past the ticket's four components.** It
  changes `client/src/wire.ts`, `session.ts`, `app.ts`,
  `client/src/view/spectator.ts` and `internal/gateway/metadata.go`; moves
  SPEC-022 and SPEC-012; withdraws VTT-442 and moves its 44 citations
  (D24); and changes six test files' socket fakes (D23). The ticket's
  "What it touches" and "Specifications this moves" predate the answer;
  this is not a fault of the ticket.
- **G12. Today a 401 from `/api/*` does not mean the credential is dead**
  (P14). `authed` answers 401 to every `Verify` error, and an identity
  store that cannot answer makes `Verify` fail for a live credential.
  Forgetting on that 401 would delete the only copy of a live credential.
  D22 makes the 401 mean unknown or revoked and nothing else.

## Measurements this plan stands on

All on 2026-10-31 at `275b449`, in a scratch copy outside the repository.

| # | Probe | Result |
|---|---|---|
| P1 | `test -e` on every file the ticket names; `git grep` for every symbol | all resolve; "credential no longer valid" is in `server.go` and in no client file; no `ClientCommand` arm names a participant but `promote_participant` and the control commands |
| P2 | `git grep '\.Revoke('` | one production call, in `newRevokeCmd`; 22 calls in 9 test files |
| P3 | Scratch internal gateway test: a spectator controlling nothing, connected; `ids.Revoke` alone | still open 1.5 s later, nothing appended |
| P4 | The same, then `presence.rejudge` | closed 0.41 ms later, status 1008, reason `gateway: credential no longer valid`; the DM's connection told `DISCONNECTED` 0.72 ms after the signal. A player with two connections: both closed, 0.22 and 0.25 ms. A dial with the dead credential: 401 |
| P5 | Scratch: a connected player controlling two party members, drained; order A (`Revoke`, `rejudge`, `AppendBatch` of two `ActorControlRevoked`) against order B (`AppendBatch`, `Revoke`, `rejudge`); frames received before the close | A: 0 of 20 runs sent a frame. B: 18 of 20 |
| P6 | Scratch: `campaign.State()` to `AppendBatch`'s return, two revocations, 200 runs; `Lookup` plus `Revoke`, 200 runs | min 0.32 ms, median 0.36, p99 0.84, max 1.65; `Lookup` plus `Revoke` median 0.29 ms, p99 4.46, max 5.66 |
| P7 | Scratch: the fold, and `identity.Revoke` | an `ActorControlRevoked` of control never held is accepted; a batch naming an actor removed after the snapshot is refused whole and nothing is applied; `Revoke` of a revoked participant answers no error |
| P8 | Scratch `bun test` beside `app.test.ts`: a stored token, the socket opened then closed; then a stored token with `?join=s3cret` | `.conn` reads `closed`, a `Reconnect` button, the token still stored; with `?join=`: a session started, one socket, no join form |
| P9 | `remove_participant = 35` and `message RemoveParticipant { string participant_id = 1; }` added in scratch, `go tool buf generate`, the completeness tests run | red: `TestManifestCoversAllCommandMessages`; `TestEveryClientCommandHasRoleCells` ("commandFor: unknown command name"); `TestEveryClientCommandConverts` ("advertised and dead"); in `internal/mcp`, `mcp.New` refuses ("oneof fields with no tools.json entry [remove_participant]") in `TestEveryManifestCommandIsRegisteredAsATool`, `TestListToolsReturnsEveryCommandAndReadTool`, `TestListToolsReturnsAllCommandToolsByName` and four read-tool tests; client: "every command the contract defines has a declared human surface", "every command with a human surface has a builder in commands.ts". `contract` stays green |
| P10 | `measure` and `share_of` from `tools/check-comments.py` against `tools/comment-ceilings.txt` | `dm.ts` 36.65 (36.7); `commands.ts` 41.03 (41.1); `app.ts` 48.51 (48.6); `wire.ts` 29.92 (30.0); `session.ts` 35.14 (35.2); `authz.go` 12.59 (12.6); `server.go` 21.96 (22.0); `authz_test.go` 34.68 (34.7); `convert_test.go` 13.17 (13.2); `server_test.go` 23.89 (23.9); `mcp/tools_test.go` 26.60 (26.7); `mcp/tools_internal_test.go` 23.74 (23.8); `mcp/door_tools.go` 34.88 (34.9); `presence.go` 15.60 (15.6). `tools/toolgen` is outside the ledger's roots |
| P11 | `~/dev/RPTool/maptool` at `f4b7fef6c` | see Rule 9 |
| P12 | `tools/coverage-thresholds.txt`, `tools/ts-coverage-thresholds.txt` | `internal/gateway` 93.5, `internal/mcp` 87.5, `tools/toolgen` 85.0; `client/src/view/dm.ts`, `commands.ts`, `app.ts`, `wire.ts`, `session.ts`, `view/spectator.ts` 100 |
| P13 | Scratch: a WHATWG `WebSocket` (bun) on a live connection whose participant is then revoked and signalled; then a dial with that credential | the close arrives with code 1008 and the reason `gateway: credential no longer valid`, `wasClean` false. The dial with the dead credential gets an error event, and no code or reason to read |
| P14 | Scratch: `/api/me` with a revoked token, with a live one, and with the live one after `participants` is renamed as `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody` renames it | 401; 200; 401. `Verify`'s error then is `identity: query participant: … no such table: participants`, not `ErrInvalidToken` |
| P15 | Scratch: `authed` answering 503 to a `Verify` error that is not `ErrInvalidToken`; `go test ./internal/gateway/` | no test fails but P9's two, the prototype arm still in place. `metadata.go` 11.55 of 11.8, from 11.73 |
| P16 | Scratch prototype of D18 to D23 in `wire.ts`, `session.ts`, `app.ts` and `view/spectator.ts`; `env -u CLAUDECODE bun test --coverage client/test` | before the fakes pass an event, 223 failures, each `undefined is not an object (evaluating 'ev.code')`; after, none but P9's two. Lines covered: `wire.ts`, `session.ts`, `spectator.ts` 100; `app.ts` 98.15, its uncovered lines `forget`'s body. Shares: `wire.ts` 29.56 (30.0), `session.ts` 34.92 (35.2), `app.ts` 47.72 (48.6), `spectator.ts` 40.25 (40.4) |
| P17 | Scratch `bun test` beside `app.test.ts`, on P16's prototype | the credential's close: the line, no Reconnect, `vtt.token` and `vtt.viewpoint.<token>` gone. Another window's token stored meanwhile: kept. 1008 with `gateway: malformed frame`, 1006, and 1000 with the credential's reason: nothing forgotten, Reconnect offered. `/api/me` 401 at the start: forgotten, the line shown; 503: neither. After forgetting, `?join=` shows the join form and dials no socket |
| P18 | `git grep -c VTT-442 -- client internal cmd` | 44 lines in 8 files: `client/src/wire.ts` 2, `session.ts` 2, `app.test.ts` 5, `wire.test.ts` 4, `qa-reload-viewpoint.test.ts` 20, `qa-remembered-log.test.ts` 8, `client/e2e/handover.spec.ts` 2, `table.spec.ts` 1; and SPEC-022 |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone (D16).
  - 3: the contract change is additive: a new message and a new oneof arm
    at the next free number, 35. 11 and 16 stay unused (SPEC-007).
    Regenerate with `task generate:contract`; generated code is committed.
  - 4: no second fold. The handler appends through `campaign.AppendBatch`.
  - 5: no game-system vocabulary.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `internal/`, `cmd/` or `client/src` is a warning or
    a pointer. History and argument go to the report.
- **The comment ceilings** (P10). Every file touched is at its ceiling. A
  test cites its rule on a citation-only line, which counts in no share.
  Each file is measured with `share_of` before its commit. Task 3's: P16's
  prototype puts `wire.ts`, `session.ts`, `app.ts` and `spectator.ts`
  under theirs with one doc line each and the comment in `forget`;
  `metadata.go` takes no comment line (P15).
- **`go-arch-lint`.** The handler needs nothing `internal/gateway` does not
  already import.
- **Coverage floors** (P12). Each new branch has a test that reaches it,
  but for the append's error arm, which no deterministic test reaches (D4).
  `wire.ts`, `session.ts`, `app.ts` and `view/spectator.ts` stay at 100.
- **The Go mutation gate runs in `task check`.** Each condition the handler
  adds has a test that goes red when it is negated.
- **Nobody starts Stryker** (D17).
- **A negative assertion needs its positive.** "Refused" comes with a
  removal of a player at the same table that succeeds; "nothing appended"
  reads the head before and after.
- **A connection's close is asserted as a close**, `websocket.CloseStatus`
  not -1, never as a read that timed out;
  `TestRevokingRemovesSomebodyWhoIsStillConnected` says why.
- **A new function or test goes after a closing brace** (`check:doc-owner`).
- **A test the register cites has a literal name.**
- `bun test` is measured with `env -u CLAUDECODE`.
- **Before every commit:**
  - `python3 tools/check-new-prose.py main`;
  - `python3 tools/check-requirements-chain.py .`;
  - `python3 tools/check-comments.py main`, read for added-line refusals,
    after staging;
  - for a client commit, `bunx tsc --noEmit -p client/tsconfig.json`, and
    `task build:client` with `cmd/vtt/webdist` committed if it changed.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo a break. Use the inverse edit, checked
  with grep.
- **The review gate refuses untracked files.** Park scratch files outside
  the tree with `cp` and `cmp`.
- **Hash the tree before and after any reviewer.**
- **The pre-push hook takes minutes.** Give it 600 000 ms, send its whole
  output to a file, and check the machine's load first.

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c` (P11):

- **Offered to the GM, and to whoever hosts.** `AppActions.BOOT_CONNECTED_PLAYER`
  is labelled "Boot..." (`action.bootConnectedPlayer`), and its
  `isAvailable` answers `MapTool.isHostingServer() ||
  MapTool.getPlayer().isGM()`. It acts on the player selected in
  `ClientConnectionPanel`.
- **It refuses oneself, and asks first.** Booting your own player shows
  `msg.error.cantBootSelf`. Otherwise, for a connected player, it asks
  `msg.confirm.bootPlayer`, "Are you sure you want to boot {0}?", and after
  a yes says `msg.info.playerBooted`, "{0} has been disconnected."
  `msg.error.cantBootGM`, "You can't boot another GM.", is in the bundle
  and no Java reads it.
- **It reaches only a connected player.** A player who is not connected
  gets `msg.error.failedToBoot`, "Boot attempt failed: player not found."
- **The server's arm asks nothing of the sender.** `ServerMessageHandler`'s
  `BOOT_PLAYER_MSG` arm calls `MapToolServer.bootPlayer`, which releases
  that player's connection, and relays the message to the other clients;
  nothing before it in `handleMessage` asks whether the sender is a GM.
  The gate is the client's menu.
- **The booted client says why.** `ClientMessageHandler`'s
  `handle(BootPlayerMsg)` disconnects and shows "You have been booted from
  the server." The player can connect again under the same name.
- **Ownership is untouched.** A token's owners are player names, and a
  boot changes none of them.

Borrowed: the word Boot, a refusal of oneself, a confirmation that names
the person, and, by Q3, a window that says why it was closed. Not
borrowed:
- the gate on the client alone. Here `commandRoles` decides on the server
  (SPEC-013);
- disconnect-only, connected-only and untouched ownership, by the owner's
  rulings of 2026-10-31;
- reconnecting by name. MapTool has nothing to forget, since the name is
  the identity. Here the credential is the identity, so the window forgets
  it (D21).

## Design questions for the owner

All three answered 2026-10-31: Q1 the DM and the agent, Q2 one ticket, Q3
alternative (c).

1. **Q1. May the agent remove a participant?** Answered "DM och agent".
   - Recommended: **the DM and the agent**, as for every other DM-side
     command.
     - The ruling of 2026-10-15: a table's DM is the agent or a human. At
       an agent-run table there is no console to press Boot in, and `vtt
       revoke` leaves control behind, which is the problem this ticket
       fixes.
     - SPEC-013 and VTT-140 say the DM and the agent hold every row but
       `set_viewpoint`. "The DM only" withdraws VTT-140 and rewrites that
       sentence.
     - The tool exists either way (G1). "The DM only" ships the agent a
       tool that is always refused, as `set_viewpoint` is.
   - The cost: an agent can remove somebody with no confirmation. The
     tool's description says the removal cannot be undone (R11).
   - Alternative: the DM only. Then R9 reads "Only a dm", VTT-140 is
     withdrawn and succeeded, and the tool's description says the agent is
     refused.
2. **Q2. One ticket and one branch, or split?** The ticket asks this.
   Answered "Ja, kör som ett ärende".
   - Recommended: **one.** P9: the arm cannot land without a tool, its
     role cells and the console's control. The client's surface tests run
     in the pre-commit hook and refuse an arm without a builder and a
     reachable control. A server half alone would need the surface table's
     "not-user-issued" hatch with a reason that is false.
3. **Q3. What does the removed participant's window show?** Today: G5.
   Answered (c), "Visa text och glöm behörigheten"; D18 to D26.
   - Recommended: **unchanged in this ticket**, and offered as a ticket of
     its own. It is equally true after `vtt revoke` today. The person in
     that window is the one the DM removed, so nothing breaks for those
     still at the table.
   - Alternative (b): the window says it was removed, in place of
     Reconnect, on a close with 1008 and that reason. `Wire.connect` hands
     the close's code and reason on, `app.ts` paints the line. `wire.ts`,
     `session.ts` and `app.ts` are at 100 coverage and at their ceilings
     (P10, P12). A reload still says "closed": a 401 handshake reaches the
     browser with no reason.
   - Alternative (c): (b), and the window forgets the dead credential, so
     the share link works again in that browser. SPEC-022 records that a
     logout removes the viewpoint key with the token and withdraws VTT-442;
     (c) is that logout. The plan covers the load too, which (b) did not
     (D22).

The decisions below carry those answers.

## Decisions

1. **D1. The command is `RemoveParticipant { string participant_id = 1; }`,
   arm `remove_participant = 35`.**
   - Imperative, additive, at the next free number (rule 3, SPEC-007).
   - "Remove" is the ticket's word, and says what happens at the table.
     Rejected: `RevokeParticipant`, because "revoke" already names two acts
     here, the credential (`identity.Revoke`) and control
     (`RevokeActorControl`), and this command does both. Rejected:
     `BootParticipant`, because MapTool's boot is a disconnect, which the
     owner rejected; the word stays the console's label.
   - Its proto comment warns, as `PromoteParticipant`'s does: what may be
     removed, that it appends control revocations, and that nothing undoes
     it.
2. **D2. A removal appends one `ActorControlRevoked` per actor whose
   `controller_ids` hold the participant, in actor-id order, as one
   `campaign.AppendBatch`, and nothing when they control none.**
   - Forced by ruling 2. One batch because the fold accepts or refuses it
     whole (P7), as `remove_actor`'s.
   - Each envelope is stamped as `handleRemoveActor` stamps its batch:
     `EventId`, the issuer's `ParticipantId` and `ActorRole`, `OccurredAt`.
   - Actor-id order makes the log the same on every run of the same
     command, as `remove_actor` orders its tokens.
   - `AppendBatch` refuses an empty batch, so a participant who controls
     nothing gets no append. The result carries the batch's first sequence
     when there is one, and none otherwise (SPEC-007's rule for a result).
   - SPEC-007 moves: `remove_participant` is not in its list of commands
     that append nothing, and joins "The commands that yield a batch".
   - `ToEvent` does not convert it; `TestEveryClientCommandConverts`'s
     `notConverted` names why.
3. **D3. The handler looks the target up, and refuses one `Lookup` does not
   answer and one whose role is dm or agent**, as `handlePromotion` does.
   - Forced by SPEC-013: `Authorize` performs no I/O and cannot read the
     target's row. `Authorize` gets a table row and nothing else.
   - `ErrInvalidToken` covers unknown and revoked alike (G8); any other
     lookup error refuses too. Nothing is revoked or appended on either.
   - The issuer is a dm or an agent (Q1), so naming oneself is refused by
     the same bound (G7).
4. **D4. The order: `Lookup`, the bound, `identity.Revoke`, `rejudge`,
   then the batch.**
   - Forced by P5. Revoking first means the pump's next lookup finds the
     participant gone, so their connections are sent nothing the batch
     causes: 0 of 20 runs. Appending first sends a removed player their
     own revocations, a `WatchableChanged` and entries, in 18 of 20. R10
     holds that.
   - Forced by VTT-162, "a refused command appends nothing". In the other
     order, a `Revoke` that fails after the append would answer a refusal
     that appended.
   - Signalling straight after `Revoke` closes the connections without
     waiting on the append (P4).
   - Build and stamp the batch (`newEventID`) before `Revoke`, so that
     `AppendBatch` is the only failure that can fall between the two
     writes.
   - **What a failure between the two writes leaves.** `AppendBatch` fails
     only on a fold refusal, a poisoned campaign, or a failure to project
     or store the batch. A fold refusal needs an actor of theirs removed
     between the snapshot and the append (P7). Then the participant is
     revoked and closed and their control stands: the state `vtt revoke`
     leaves today. Each character's row still names the holder by id and
     offers its Revoke chip. The result is a refusal carrying the append's
     error.
   - Near-theoretical with a small consequence: no compensating write, no
     test, no debt entry, no specification sentence (2026-10-21). The arm
     is the natural `if err != nil` return; no deterministic test reaches
     it, and its negation is killed by every successful removal's test.
   - Rejected: one transaction. Identity is not event-sourced and holds
     its own handle on the file (SPEC-009).
5. **D5. The batch is built from `authorize`'s snapshot**, as
   `handleRemoveActor` builds its batch.
   - P6: from snapshot to append is a median of about 0.65 ms (`Lookup`
     and `Revoke` 0.29, the append 0.36), p99 about 5 ms.
   - A grant to the participant inside that window leaves one character
     controlled by a revoked id. It takes the one DM granting a character
     to the person they are removing, from a second window, inside those
     milliseconds. The consequence is the `vtt revoke` state on one
     character, repaired by its Revoke chip.
   - Near-theoretical, small consequence: nothing (2026-10-21). Rejected:
     a campaign call that builds the batch under its own lock, and a grant
     validator that reads identity. Both are machinery for this window, and
     a grant to a revoked id is accepted today too.
6. **D6. A command in flight from the removed participant.**
   - Nothing is half-applied. Each append is one `Append` or one
     `AppendBatch`, all or nothing, and `Revoke` is one `UPDATE`.
   - A command whose `Lookup` ran before the `Revoke` can still land after
     the removal, as can any command whose right is withdrawn while it is
     being judged. Likewise a handshake verified just before the `Revoke`
     is announced `CONNECTED`, then `DISCONNECTED` when its pump's first
     lookup closes it.
   - Both take the participant acting within the same millisecond.
     Near-theoretical, small consequence: nothing.
7. **D7. "Closed at once" is the promotion's signal.** After `Revoke`, the
   handler calls `presence.rejudge(target)`. Each connection's pump takes
   it, `pumpState.revoked` finds `ErrInvalidToken` and closes with 1008 and
   "gateway: credential no longer valid" (P4).
   - A connection registered after the signal is caught by its pump's
     opening lookup. A dial afterwards is answered 401 (VTT-080).
   - The teardown announces `DISCONNECTED` to the table (VTT-104, P4).
   - No new close path, and `presence.go` does not change.
8. **D8. Who may issue it: the DM and the agent** (Q1). `commandRoles`
   gives the row both cells; VTT-140 and SPEC-013's sentence stand.
9. **D9. The console.**
   - A button labelled **Boot** on each "Who may do what" row whose role is
     player or spectator, and on no other row, with the data-action
     `boot-<participant id>`, as promotion's `promote-<id>`. It mirrors
     D3's bound, so the console offers nothing the server refuses.
   - Pressed, it asks through `DMDeps.confirm`, which `app.ts` makes
     `window.confirm`, as New link does. The message names the person and
     says it cannot be undone. Proposed: "Boot Kim? They leave the table
     for good: their link stops working, and they no longer control any
     character." The words are the implementer's.
   - Cancelled, it sends nothing. Confirmed, it sends `removeParticipant(id)`
     and, after the answer, `refreshSharing`, the shape promotion and New
     link share.
   - No `sending` lock. Make player has none either; the modal confirm
     stands before a second send, and a second removal of the same person
     is refused and writes nothing (D3).
   - Afterwards: the row goes at the roster's re-read. The character rows
     lose that holder's chip as the batch's events fold. The grant select
     stops offering them once roster and presence both drop them, and
     presence drops them at their `DISCONNECTED`. `known` does not change.
   - `dm.ts`'s head comment (G10) becomes a warning with a pointer: ask
     first, through `confirm`, before an action that cannot be undone.
10. **D10. The removed participant's window: Q3(c),** decided in D18 to
    D26 and built in Task 3.
11. **D11. One browser beat, E1, in a new file `client/e2e/booting.spec.ts`.**
    - Forced by G4. `global-setup` gives every spec file its own server and
      campaign, so the file needs no registration.
    - Rerunnable by construction: the guest's name and the actor's id
      carry `test.info().repeatEachIndex`, and the file closes the door it
      opened, in `test.afterAll`. It opens no session.
    - Its "True when" includes `--repeat-each=2`.
    - A second beat, E2, is the load case (D22): a participant removed
      while their page is closed opens the app again.
12. **D12. The tool's description says what is lost.** An agent reading a
    tool called remove could take it for a disconnect. The description says
    it is permanent, closes their connections, frees the name and takes
    every actor they control, one revocation each. A toolgen test holds
    the first and the last (R11).
13. **D13. A contract fixture and its round trip**, as
    `remove_actor_command.json` has. A preference: it pins the JSON key the
    client and the e2e send.
14. **D14. The arm lands in one commit with every table that names each
    command** (P9): the toolgen manifest and golden, the role row, the
    conversion list, the MCP name lists, the client builder, the surface
    row and the console's control. Between Tasks 1 and 2 the server answers
    a removal with `ToEvent`'s "gateway: unknown or empty command", and
    nothing is revoked.
15. **D15. `vtt revoke` does not change.** It takes no hold on a served
    campaign (VTT-270), so it cannot append, and stays identity-only. Item
    7 is held by its existing tests, run whole.
16. **D16. `python3 tools/check-comments.py --write-ledger` runs once, after
    the last code commit,** and only if `check-comments.py main` finds a
    file more than 1.0 under its ceiling. A lowered row cannot be raised
    again.
17. **D17. No Stryker run.** `client/src` and `client/test` change, so the
    client's stored mutation report goes out of date. At Task 7 the owner
    is told which commit it measured and the tickets since, and that
    `task check:ts-mutation:full` is his to ask for.

The removed participant's window, by Q3(c):

18. **D18. What sets it off: a close with code 1008 and the reason
    `gateway: credential no longer valid`, both exactly.**
    - The gateway sends that pair from two places, the read loop and
      `pumpState.revoked`, and only when `Lookup` answers `ErrInvalidToken`
      (SPEC-009, SPEC-011). An operational lookup failure never sends it.
      P13: a WHATWG socket receives both.
    - The code alone is not enough: 1008 also closes a malformed frame,
      with `gateway: malformed frame`.
    - At a load, `/api/me` answered 401 sets it off too (D22). A refused
      handshake cannot: a browser socket gets an error and no code or
      reason (P13).
19. **D19. What the window shows: one line, in place of Reconnect.**
    - Proposed: "You were removed from this table. To join again, open the
      link your DM sent you." Plain, and in the voice of the client's own
      "No invite token. Open the link your DM sent you." B1 and E1 hold
      that it says removed; the exact words are the implementer's.
    - It stands where `renderStatus` in `client/src/view/spectator.ts` draws
      Reconnect, with the class `removed`, styled as `.fatal` is, whatever
      the status reads. Reconnect is not drawn: it would dial a credential
      that is gone.
    - The rest stays as a closed window's does today: the board, the Story
      and the panels as last drawn. That is the alternative's own wording,
      "in place of Reconnect". Rejected: replacing the whole window, as the
      fold failure does. VTT-446, VTT-449, VTT-457 and VTT-458 each except
      the fold failure alone, so all four would become false as worded, and
      their 45 citations would move, for a window nobody at the table sees.
    - It offers nothing to press. The window cannot know the share link's
      current secret; the person holds the link.
20. **D20. Where it is decided.** `Wire`'s close handler compares the close
    with `CREDENTIAL_GONE` and calls its `onCredentialGone` handlers;
    `Session.onCredentialGone` passes them on; `startSession` in
    `client/src/app.ts` owns `forget`, since it holds the token and the
    viewpoint key, and `boot` hands it the `Auth`. `forget` sets the flag
    `paint` passes to the view as `ViewExtras.removed`, and paints.
21. **D21. What is forgotten: `vtt.token` in `localStorage`, only while it
    holds this window's own token, and `vtt.viewpoint.<token>` in the
    tab's `sessionStorage`.**
    - The key carries the token, so a logout removes both (SPEC-022; the
      warning in `startSession`).
    - Only its own token: every window of the browser shares
      `localStorage`, and another may have stored its own since. The alpha
      test runs two participants in one browser. Clearing another's token
      would log that window out at its next load (P17).
    - Other tabs' keys under the dead token cannot be reached from here.
      They name a log no dial under that token can open.
    - Afterwards the browser holds no token, so `boot` reads `?join=` and
      shows the join form (P17): the owner's intent.
22. **D22. The load case is covered: `/api/me` answered 401 forgets too,
    and `authed` stops answering 401 when identity cannot answer.**
    - A window that was closed when its participant was removed keeps the
      dead credential, and its next load dials it. The handshake's 401
      reaches the page as an error with no code (P13), so the socket cannot
      tell. `/api/me` can: `getJSON` already throws "metadata: unauthorized
      — the token is unknown or revoked" on a 401, and `startSession`'s
      startup chain already catches it. The catch calls `forget` on that
      message and on no other.
    - But P14: today `authed` answers 401 to a live token while the
      identity store cannot answer. Forgetting then would delete the only
      copy of a live credential (the table holds its hash, SPEC-009), the
      DM's included, whom only `vtt invite` restores. That consequence is
      not small, so the 2026-10-21 ruling does not apply.
    - So `authed` answers a `Verify` error that is not `ErrInvalidToken`
      with 503 and `gateway: identity unavailable`, the words
      `answerCommand` gives the same failure. A missing header, an unknown
      token and a revoked one keep their 401 (VTT-111). The 503 is no
      oracle: it follows the store, not the token, and while the store
      cannot answer an unknown token gets it too.
    - P15: no existing test depends on the old answer. It also makes
      `refreshSharing`'s comment true: a transient keeps the DM's panel,
      where today an identity outage reads as a refusal and drops it.
    - Cost: about five lines in `metadata.go`, one gateway test, and
      SPEC-012's paragraph on the one 401. `handleWS` keeps its 401 for
      both, since no browser reads a handshake's status.
    - Not covered: `refreshSharing`'s refusals and the role re-read at a
      presence frame. The socket's close or the next load reaches the same
      window first.
23. **D23. The test fakes pass a close event.** Six test files' socket fakes
    call `onclose` with no event, which no browser does; with D18's
    comparison over 220 tests fail (P16; Task 0's review measured 221). Each fake passes
    `{ code: 1000, reason: "" }`, as `qa-reload-viewpoint.test.ts`'s does
    already. No guard for an absent event goes in `wire.ts`: no browser
    reaches it.
24. **D24. VTT-442 is withdrawn and succeeded by N442.** Once a page can be
    opened under a token whose remembered viewpoint its tab has forgotten,
    "names the actor named by the last `ViewpointChanged` sent to an
    earlier page in its tab under its token" is false (SPEC-022 records
    this of a logout). Its 44 citations name N442 (P18), `wire.ts`'s and
    `session.ts`'s pointer comments among them. VTT-443 and VTT-444 stay
    true: neither speaks of a page whose earlier pages under its token were
    sent a `ViewpointChanged`.
25. **D25. One literal on each side, pinned by
    `contract/testdata/credential_close.json`.**
    - The gateway's two sites take one constant; S2 compares the close it
      observes with the fixture; W3 compares `CREDENTIAL_GONE` with it.
    - Forced by how it would fail: a reason reworded on one side leaves the
      window saying "closed" with Reconnect, and no test goes red. The join
      link's format is pinned the same way, by
      `contract/testdata/join_url_format.json`.
26. **D26. Near-theoretical, nothing:** a keepalive reap that coincides with
    the pump's close costs the peer its reason (SPEC-011). That window says
    "closed" and offers Reconnect, and its next load forgets by D22.

## Requirements

Allocated at Task 0 with `requirement-id`, each `**OPEN — no test yet**`
until its tests land. The R, F, N, S, T, W, C, A, B, E and K labels are
this plan's.

- **R1.** "A removal by a dm or an agent of a player or a spectator revokes
  that participant, whether or not they are connected." S1, S2, S3. Red
  under K1.
- **R2.** "Every connection of a removed participant is closed with the
  policy-violation status and the reason `gateway: credential no longer
  valid`, without waiting for a command of theirs or an append." S2. Red under K2.
- **R3.** "A removal appends one `ActorControlRevoked` naming the removed
  participant for each actor whose controllers include them, in actor-id
  order, and no other event." S3. Red under K3, K4, K9.
- **R4.** "A removal leaves every other controller of an actor among its
  controllers." S3. Red under K5.
- **R5.** "A removal of a participant who controls no actor appends
  nothing." S4. Red under K13.
- **R6.** "Every event a removal appends carries the participant id and the
  role of whoever issued it." S3. Red under K10.
- **R7.** "A removal's result carries the first sequence of the batch it
  appended." S3. Red under K11.
- **R8.** "A removal whose target is a dm, an agent, unknown or already
  revoked is refused, revokes nobody and appends nothing." S6. Red under K6,
  K7.
- **R9.** "Only a dm or an agent may remove a participant." The
  `remove_participant` rows of `TestAuthorizeTableAllCommandsAllRoles`, S7.
  Red under K8.
- **R10.** "A removed participant's connections are sent nothing the
  removal's own events cause." S5. Red under K12.
- **R11.** "The `remove_participant` tool says the removal cannot be undone
  and revokes the participant's control of every actor they control." T1. Red under
  K14.
- **R12.** "The console's 'Who may do what' row of a player or a spectator
  offers Boot, and the row of a dm or an agent does not." A1. Red under K15.
- **R13.** "Boot asks the DM to confirm, in a message holding the
  participant's display name, and sends nothing when the DM cancels." A2. Red under K16.
- **R14.** "A confirmed Boot sends one removal of its row's participant."
  A3, E1. Red under K17.
- **R15.** "A confirmed Boot asks for the roster to be read again when its
  removal is answered, and not when it sends it." A3. Red under K18.

The removed participant's window (Q3(c)):

- **F1.** "A browser window whose connection the gateway closes with code
  1008 and the reason `gateway: credential no longer valid` shows, in place
  of Reconnect, that its participant was removed from the table." W2, B1,
  E1. Red under K23, K28.
- **F2.** "A browser window whose first read of `/api/me` is answered 401
  shows, in place of Reconnect, that its participant was removed from the
  table." B4, E2. Red under K25.
- **F3.** "When a window shows that its participant was removed, the
  credential stored in its browser is forgotten if it is that window's
  own, and kept if it is another's." B1, B2, B4. Red under K21, K23.
- **F4.** "When a window shows that its participant was removed, the
  viewpoint its tab remembers under that window's credential is forgotten."
  B1. Red under K22.
- **F5.** "A close with any other code or reason makes a window neither
  forget anything nor show that its participant was removed." W2, B3. Red
  under K19, K20.
- **F5b.** "A first read of `/api/me` that fails other than with 401 makes
  a window neither forget anything nor show that its participant was
  removed." B4. Red under K24.
- **F6.** "Once a window has forgotten the credential stored in its
  browser, the share link opened there, with no credential stored since,
  shows the join form." B5, E1, E2.
  Red under K23.
- **F7.** "A request to a route `Handler` registers under `/api/`, carrying
  a bearer token, made while the identity store cannot answer, is answered
  503 and not 401." S9.
  Red under K26.
- **N442**, succeeding VTT-442: "Until a `ViewpointChanged` arrives, each
  dial of a browser page names in `log`, with `after` 0, the actor named by
  the last `ViewpointChanged` sent to an earlier page in its tab under its
  token, and names no `log` when that one named nobody or when its tab has
  since forgotten what it remembered under that token." VTT-442's tests,
  and B5. Red under K22.

Withdrawn at Task 0, in the register's form, with the evidence `**READING
— verify-ticket amendment of 2026-10-31; withdrawn, held by nothing**`:
VTT-442, "False as worded once a window forgets the viewpoint its tab
remembers under a credential the gateway declared dead", succeeded by N442.
And VTT-111, found by Task 0's review, "False as worded once a request
carrying a bearer token while the identity store cannot answer is answered
503", succeeded by **N111**: "A request to `/api/ruleset`, `/api/me` or
`/api/art/{file}` with no bearer token, an unknown one or a revoked one is
answered 401 while the identity store can answer." Its three tests in
`internal/gateway/metadata_test.go` name N111 at Task 3.

Kept, wording still true, evidence added:
- **VTT-034** ("Revoked participants are not listed"): S1.
- **VTT-074** (`Lookup` answers nobody for a revoked participant): S1.
- **VTT-080** (401 for a credential that does not verify): S2's redial.
- **VTT-104** (a connection the server closes is announced gone): S2.
- **VTT-376** (a revoked participant's connection closes with the
  policy-violation status): S2.
- **VTT-544** (a name held only by revoked participants is free): S8, E1.
- **VTT-392** (a revocation leaves a party member a party member): S3.
- **VTT-162** (a refused command appends nothing): S6, S7.
- **VTT-140** holds through `TestAuthorizeTableAllCommandsAllRoles`, which
  it already cites (Q1).
- **N111** (VTT-111's successor): S9's controls.

Kept because the removed window keeps its board (D19): VTT-446, VTT-449,
VTT-457 and VTT-458. B1 asserts the Story is still drawn after the close.
VTT-443 and VTT-444 hold (D24).

Read and kept as they stand: VTT-028, VTT-032, VTT-033, VTT-067, VTT-147,
VTT-270, VTT-352. None says anything a removal makes false. VTT-028 names
the door, the link and promotion and claims no others; R9 is the removal's.

The sort, and what it rejected:
- "As `vtt revoke` does" (rule 1): a pointer, not an observation. R1 says
  what is true afterwards, and the kept rows say the rest.
- "Close at once": R2's "without waiting for a command of theirs or an
  append" is what P3 shows today's tree does not do.
- "Oneself": inside R8, since the issuer is a dm or an agent (G7).
- The order of the writes: R10 is its observable; the order itself is how.
- The race and the failure between the writes: no rows (D4, D5, D6).
- "Says they were removed": F1 and F2; the line's words are how (D19).
- "The share link works again in that browser": F6, held by F3 and F4.
- The lost reason under a keepalive reap: no row (D26).

| Ticket item | Observed by |
|---|---|
| 1 | S1, S8, A3, E1 |
| 2 | A2 |
| 3 | S2 |
| 4 | S3 |
| 5 | S1, E2 |
| 6 | S6, S7, the table test |
| 7 | `cmd/vtt` and `internal/identity` tests that run `vtt revoke` and `Revoke`, run whole |
| 8 | Task 7 |
| Q3(c) | W2, W3, B1 to B5, S9, E1, E2 |

## The existing tests that change

- `TestAuthorizeTableAllCommandsAllRoles` gains four rows; `commandFor`
  gains a case.
- `TestEveryClientCommandConverts`'s `notConverted` gains
  `remove_participant`, with its reason.
- `wantCommandToolNames` in `internal/mcp/tools_test.go` and its internal
  twin `theCommandTools` in `tools_internal_test.go` gain the name.
- `COMMAND_SURFACE` gains `removeParticipant: { surface: "dm-console",
  action: "boot-", dynamic: true }`. Its comments on `dynamic`, "True for
  the one entry" and "and only one entry needs to", are reworded for two.
- `TestToolsMatchGolden` reads the regenerated golden.
- The socket fakes in `client/test/app.test.ts`, `wire.test.ts`,
  `qa-switch.test.ts`, `qa-one-log.test.ts`, `qa-events-panel.test.ts` and
  `qa-window-number.test.ts` pass a close event (D23).
- Every test citing VTT-442 cites N442 (D24, P18).
- S2, Task 2's test, reads its close from
  `contract/testdata/credential_close.json` once Task 3 adds it (D25).
- No other existing test changes. P9 ran every completeness test the arm
  reaches, P15 the gateway suite under D22, and P16 the client suite under
  D18 to D23.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Sign-off is given (Q1 to Q3 answered 2026-10-31).
  - Allocate R1 to R15, F1 to F7 and N442 with `requirement-id`.
  - Withdraw VTT-442 in the register's form, naming N442.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean; the rows exist; the chain passes.

### Task 1: the contract, its tables, and Boot (R9, R11 to R15; D1, D9, D12 to D14)

- **Files:**
  - `contract/vtt/v1/commands.proto`; `task generate:contract`, which
    writes `contract/gen/` and `cmd/vtt/tools.json`;
    `contract/testdata/expected_tools.json`;
    `contract/testdata/remove_participant_command.json` and
    `contract/roundtrip_test.go`;
  - `tools/toolgen/main.go` (the manifest entry) and
    `tools/toolgen/main_test.go`;
  - `internal/gateway/authz.go` (`commandRoles`, `commandName`),
    `authz_test.go`, `convert_test.go`;
  - `internal/mcp/tools_test.go`, `tools_internal_test.go`,
    `door_tools.go` (`getParticipantsDescription` names the new tool);
  - `client/src/commands.ts` (`removeParticipant`),
    `client/src/view/dm.ts` (Boot; the head comment),
    `client/test/commands.test.ts`, `client/test/dm-view.test.ts`,
    `client/test/command-surface.test.ts`;
  - `cmd/vtt/webdist`;
  - `docs/requirements.md`: R9, R11 to R15.
- **New tests**, each with a literal name and its id line:
  - **T1** (R11), `tools/toolgen/main_test.go`,
    `TestRemoveParticipantSaysItIsPermanentAndTakesTheirActors`: the
    description says it cannot be undone and that every actor they control
    is taken from them.
  - **W1**, `contract/roundtrip_test.go`,
    `TestRemoveParticipantCommandRoundTrip`, against the fixture.
  - **C1**, `commands.test.ts`: `removeParticipant("p-7")` is the
    `removeParticipant` case naming `p-7` and carries a request id.
  - **A1** (R12), `dm-view.test.ts`: a roster of a spectator, a player, a
    dm and an agent. Boot renders on the spectator's and the player's rows,
    `boot-<id>`, labelled "Boot", and on neither of the others.
  - **A2** (R13), `dm-view.test.ts`: `confirm` answers no. It was asked
    once, its message holds the row's name, and nothing was sent.
  - **A3** (R14, R15), `dm-view.test.ts`: `confirm` answers yes. One
    command, `removeParticipant`, naming that row's participant and not
    the other row's; `refreshSharing` is 0 before the answer and 1 after.
- **Order:**
  1. The proto arm and message, `task generate:contract`, then every table
     P9 names, until `go test ./tools/toolgen/ ./contract/
     ./internal/gateway/ ./internal/mcp/` and `bun test client/test` pass.
  2. T1, W1, C1, A1 to A3, red where the code is not there yet.
  3. The builder, Boot, the head comment, the tool description.
  4. The breaks.
- **True when:**
  - `go test ./tools/toolgen/ ./contract/ ./internal/gateway/
    ./internal/mcp/ ./cmd/vtt/` passes;
  - `env -u CLAUDECODE bun test client/test` passes; the typecheck is
    green; `client/src/view/dm.ts` and `commands.ts` are at 100 coverage;
  - a removal sent to the server is refused, and nothing is revoked;
  - `share_of` holds every file at or under its ceiling;
  - `task generate:contract` run again changes nothing.
- **Breaks**, each undone by its inverse edit:
  - **K8:** `commandRoles`' row gains the player. The table test red.
  - **K14:** the description loses "undone". T1 red.
  - **K15:** Boot is drawn on every row. A1 red.
  - **K16:** the `confirm` call is dropped. A2 red.
  - **K17:** Boot sends the first roster row's id. A3 red.
  - **K18:** `refreshSharing` is called beside the send. A3 red.

### Task 2: the removal itself (R1 to R8, R10; D2 to D7)

- **Files:** `internal/gateway/server.go` (the dispatch in `handleCommand`,
  and `handleRemoval` beside `handlePromotion`);
  `internal/gateway/remove_participant_test.go`, new, package
  `gateway_test`, on `newGWFixture`; `docs/requirements.md`: R1 to R8, R10
  and the kept rows' evidence.
- **New tests**, each with a literal name and its id line:
  - **S1** (R1, VTT-034, VTT-074),
    `TestTheDMRemovesASpectatorWhoIsNotConnected`: ok; `Lookup` answers
    `ErrInvalidToken`; `List` omits them; nobody else's `Lookup` changes.
  - **S2** (R2, VTT-080, VTT-104, VTT-376),
    `TestEveryConnectionOfARemovedParticipantClosesAtOnce`: a spectator
    who controls nothing, on two connections, sending nothing. The DM
    removes them; both close with `StatusPolicyViolation` within 5 s; the
    head has not moved; the DM is told `DISCONNECTED`; a dial with their
    token is answered 401.
  - **S3** (R1, R3, R4, R6, R7, VTT-392),
    `TestARemovalRevokesEveryControlTheParticipantHolds`: a player holding
    five actors, inserted in the order `a3`, `a1`, `a5`, `a2`, `a4`, which
    no rotation of Go's small-map iteration sorts, a party member and a
    creature among them; one also held by another player; and a sixth
    actor the player does not hold, which the batch must not name. The result's sequence is
    the batch's first; the batch is five `ActorControlRevoked` in actor-id
    order naming the player, each with the DM's participant id and role,
    and nothing else; the shared actor keeps the other player; each kind
    is unchanged.
  - **S4** (R5), `TestARemovalOfSomebodyWhoControlsNothingAppendsNothing`:
    ok, sequence 0, the head unchanged.
  - **S5** (R10), `TestARemovedPlayerIsSentNothingTheRemovalCaused`: ten
    players, each connected, holding a party member they watch, drained.
    Each removed in turn: every frame they receive before their close is
    counted, and the count is 0.
  - **S6** (R8, VTT-162),
    `TestARemovalOfADMAnAgentTheIssuerAnUnknownOrARevokedTargetIsRefused`:
    the DM names another dm, the agent, itself, an id never minted, and a
    player revoked by `ids.Revoke` who still controls an actor. Each is
    refused; every target still `Lookup`s as before; the head has not
    moved; the agent's open connection stays open. The control: the same
    DM then removes a player, ok.
  - **S7** (R9, VTT-162), `TestAPlayerAndASpectatorMayNotRemoveAnybody`:
    each sends a removal of the other over the wire; refused, both still
    resolve, the head unchanged.
  - **S8** (VTT-544), `TestARemovedParticipantsNameIsFreeAtTheShareLink`:
    `Kim` held, then removed; at an open door, `POST /join` with `kim` is
    answered 200.
- **Order:** write S1 to S8 on Task 1's commit; all but S7 are red there.
  Then the handler, then the breaks.
- **True when:** `go test ./internal/gateway/` passes; S1 to S8 pass under
  `go test -cpu 1,2 -count=5 -run` naming each; `server.go` is at or under
  its ceiling; `go vet` is clean.
- **Breaks:**
  - **K1:** `ids.Revoke` not called. S1, S2, S3 red.
  - **K2:** `rejudge` not called. S2 red: the spectator who controls nothing
    stays open. S3's player is still closed by the append's wake, which is
    why S2's target controls nothing.
  - **K3:** the batch names every actor. S3 red.
  - **K4:** the batch is never appended. S3 red.
  - **K5:** the batch revokes every controller of each actor the target
    holds. S3's shared actor red.
  - **K6:** the dm and agent bound removed. S6's dm, agent and issuer red.
  - **K7:** a target `Lookup` answers `ErrInvalidToken` for is not refused:
    the handler goes on to `Revoke` without the role bound. S6's revoked
    target red: `Revoke` succeeds again (P7) and their control is appended.
    The unknown id stays refused, by `Revoke`'s own error.
  - **K9:** the actor-id sort removed. S3's order red on every run: Go 1.26
    iterates a map of up to eight entries as a rotation, and no rotation of
    S3's insertion order is sorted (measured by Task 0's review).
  - **K10:** the batch's `ParticipantId` left empty. S3 red.
  - **K11:** the result's `Sequence` left out. S3 red.
  - **K12:** the batch appended before `Revoke` and `rejudge`. S5 red: P5
    measured 18 of 20 removals sent a frame.
  - **K13:** the batch appended when empty. S4 red: `AppendBatch` refuses
    it.

### Task 3: the window of a removed participant (F1 to F7, N442; D18 to D26)

- **Files:**
  - `internal/gateway/metadata.go` (`authed`) and `metadata_test.go`;
  - `internal/gateway/server.go`: the two "gateway: credential no longer
    valid" literals become one constant;
  - `contract/testdata/credential_close.json`, new, and S2 reading it;
  - `client/src/wire.ts` (`CREDENTIAL_GONE`, `onCredentialGone`, the close
    handler), `client/src/session.ts` (`onCredentialGone`),
    `client/src/app.ts` (`boot` hands `startSession` the `Auth`; `forget`;
    the startup chain's catch; `removed` to the view),
    `client/src/view/spectator.ts` (`ViewExtras.removed`, `renderStatus`),
    `client/src/style.css` (`.removed`);
  - the six socket fakes (D23), and VTT-442's 44 citations (D24);
  - `client/test/wire.test.ts`, `client/test/app.test.ts`;
  - `cmd/vtt/webdist`;
  - `docs/requirements.md`: F1 to F7, N442, VTT-111.
- **New tests**, each with a literal name and its id line:
  - **S9** (F7, VTT-111), `metadata_test.go`,
    `TestAnAPIRequestWhileIdentityCannotAnswerIs503`: a live token's
    `/api/me` answers 200 and a garbage token's 401; with `participants`
    renamed, as `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody`
    renames it, the live token's answers 503 with `gateway: identity
    unavailable`; with the table put back, 200 again.
  - **W2** (F1, F5), `wire.test.ts`, "a close says the credential is gone
    only with the credential's code and reason": the fixture's close calls
    `onCredentialGone` once; 1008 with `gateway: malformed frame`, 1006
    with no reason and 1000 with the credential's reason never call it.
  - **W3** (F1), `wire.test.ts`, "the credential's close is the one in
    contract/testdata/credential_close.json": `CREDENTIAL_GONE` equals it.
  - **B1** (F1, F3, F4), `app.test.ts`, "a window closed for a dead
    credential says it was removed in place of Reconnect, and forgets its
    credential and its tab's viewpoint": `?token=tok-k`, a
    `ViewpointChanged` naming `a1`, so the key holds `a1`, then the
    fixture's close. `.removed` reads the line; no `.reconnect`;
    `vtt.token` is gone and so is the key; the Story still shows the
    entries sent before the close.
  - **B2** (F3), "a dead credential's window keeps another window's stored
    credential": after the page opens, `vtt.token` is set to `tok-other`;
    after the fixture's close it still holds `tok-other`.
  - **B3** (F5), "a window closed any other way forgets nothing and offers
    Reconnect": W2's three other closes, each on its own page.
  - **B4** (F2, F3, F5), "a page whose /api/me answers 401 says it was
    removed and forgets its credential, and one answered 503 does
    neither": a stored token, `/api/me` stubbed 401, then 503.
  - **B5** (F6, N442), "after a removal the share link opens the join
    form, and a page under the dead token names no log": after B1's close,
    `boot` with `?join=s3cret` shows the join form and dials nothing; then
    `boot` with `?token=tok-k` dials naming no `log`.
- **Order:**
  1. The fakes pass a close event; the suite stays green.
  2. The constant and the fixture; S2 reads the fixture.
  3. S9, red; `authed`.
  4. W2, W3, B1 to B5, red; the client code.
  5. The citations; then the breaks.
- **True when:**
  - `go test ./internal/gateway/` passes;
  - `env -u CLAUDECODE bun test client/test` passes, with `wire.ts`,
    `session.ts`, `app.ts` and `view/spectator.ts` at 100 coverage; the
    typecheck is green;
  - `share_of` holds each file at or under its ceiling;
  - `git grep -n "VTT-442" -- client internal cmd` prints nothing;
  - `git grep -n "Auth.clear\|auth.clear" -- client/src` finds `forget`
    and the warning in `startSession`.
- **Breaks:**
  - **K19:** the wire compares the code alone. W2 red, and B3's malformed
    frame.
  - **K20:** the wire compares the reason alone. W2's 1000 case red.
  - **K21:** `forget` clears the stored credential whoever's it is. B2 red.
  - **K22:** `forget` leaves the viewpoint key. B1 red, and B5's dial.
  - **K23:** the close never calls `forget`. B1 red.
  - **K24:** the startup catch forgets on any failure. B4's 503 red.
  - **K25:** the startup catch forgets nothing. B4's 401 red.
  - **K26:** `authed` answers 401 to every `Verify` error again. S9 red.
  - **K27:** the client's reason differs from the fixture by a word. W3 red.
  - **K28:** `renderStatus` draws Reconnect beside the line. B1 red.

### Task 4: the browser (items 1, 2, 3, 4, 5; F1, F2, F6; D11)

- **Files:** `client/e2e/booting.spec.ts`, new.
- `i` is `test.info().repeatEachIndex` in both tests.
- **E1** (R14, F1, F6, VTT-544), `the DM boots a player, who leaves the
  table and their characters, and can join again by the link`.
  - With `asDM`: add a party member `boot-hero-<i>`, open the door.
  - A guest context opens the link and joins as `Kit<i>`; with `asDM`,
    promote them and grant them the actor.
  - In the DM's console: the row reads `Kit<i>`; a dialog handler asserts
    the message holds `Kit<i>` and accepts it; Boot on that row is pressed.
  - Then: the guest's page shows the removed line and no Reconnect; the
    roster has no `Kit<i>` row; the actor's row holds no `.held-who`
    reading `Kit<i>`.
  - The same guest page opens the DM's current link: the join form, empty
    of any session. It joins as `kit<i>` and is connected.
  - Screenshots of the console before and after, and of the guest's page
    removed and rejoined.
- **E2** (F2, F6, item 5), `a player removed while away is told so when
  they come back, and can join again by the link`.
  - A guest context joins as `Jo<i>` and closes its page, keeping its
    storage.
  - The DM boots `Jo<i>` from the console.
  - A new page in that context opens the table's address: the removed
    line. It then opens the link: the join form.
- `test.afterAll` closes the door.
- **Order:** `task build:client`; check the machine's load; `task e2e`
  whole; then `bunx playwright test booting.spec.ts --repeat-each=2`.
- **True when:** every spec passes, and the rerun passes twice.

### Task 5: the report

- **Do:** write `docs/reports/2026-10-31-the-dm-boots-a-participant.md` and
  give it its reading review.
- **It covers:** each "Done looks like" item with its observation; K1 to
  K28 with what went red; Rule 9; G1 to G12 and Q1 to Q3's answers; P5,
  P6 and P14, said once; the history cut from `dm.ts`'s head comment;
  Phase 4a's adjudications under their own heading. Leave room for the
  gate's figures.
- **True when:** it answers the ticket's eight items in their numbering,
  each `[x]` naming its observation, and Q3(c)'s F1 to F7 after them.

### Task 6: the specifications

With the `specification` skill, from the report:
- **SPEC-009:**
  - a paragraph, "Removal is a command": who may issue it; `Lookup` and the
    bound in the handler and why; `Revoke`, then `rejudge`, then the batch;
    the pump's lookup closes each connection; no frame the batch causes
    reaches them; the name is free (the paragraph on names already says a
    revoked holder's is);
  - "every promotion signal it takes" names a removal's too (G3);
  - Consequences: `remove_participant` joins the commands ruled DM-side;
  - Status: `remove_participant_test.go`, the QA's files, the client tests
    and `booting.spec.ts`.
- **SPEC-013:** the table's row; step 6 of the command path; "What is
  dispatched"; the sentence on what confers standing takes removal in.
- **SPEC-007:** removal appends one revocation per actor held, in actor-id
  order, or nothing; "The commands that yield a batch"; "What a command
  refuses": DM-side; Status: T1 and W1.
- **SPEC-011:** "its caller" becomes "its callers" (G3).
- **SPEC-022:**
  - what a window does when the gateway closes it with the credential's
    code and reason, or when its first `/api/me` is answered 401: the
    line in place of Reconnect, the rest kept; its own stored credential
    and its tab's viewpoint key forgotten, another window's credential
    kept; the pin in `contract/testdata/credential_close.json`;
  - the paragraph on what a tab remembers: N442 replaces VTT-442;
  - the consequence "The browser client has no logout" becomes what
    `forget` is, and that it is the client's one logout;
  - the consequence on a failed first read of `/api/me`: a 401 forgets,
    any other failure does not;
  - Status and Requirements: VTT-442 out; N442 and F1 to F6 in; B1 to B5,
    W2, W3, E1 and E2.
- **SPEC-012:** "The bearer, and its one 401": a `Verify` error that is not
  `ErrInvalidToken` is answered 503 with `gateway: identity unavailable`;
  the consequence "a 401 says only that the token did not verify" says a
  401 is an unknown or a revoked token and a 503 an identity store that
  cannot answer; Requirements: F7.

No sentence on the race, the failure between the writes, a command in
flight, or a reason lost to a keepalive reap (D4 to D6, D26).

**True when:** each changed sentence has been run against the code by
someone other than its writer; every id in each Requirements section
resolves; `git grep -n "VTT-442" docs/specifications` prints nothing.

### Task 7: the ledger, the gate, whole, and the commits

- **Do:**
  - Run `python3 tools/check-comments.py main`; if it finds a file more
    than 1.0 under its ceiling, run `--write-ledger` once and commit the
    ledger (D16).
  - Check free space and the machine's load.
  - Run `task check` whole on the last code commit. Launch it with
    `start_new_session=True`, send all output to a file, and require the
    gate's own completion line and `check:ts-mutation`'s own line.
  - Write the figures into the report; commit the report, then the
    specifications.
  - Tell the owner D17's reminder.
- **True when:** the gate is green, `check:comments`, `check:new-prose`,
  `check:requirements-chain`, `check:coverage`, `check:mutation` and
  `check:drift` included; `check:ts-mutation` printed that the client's
  mutation was not measured, naming its commit and this ticket; the log
  holds no Stryker line.
- Push and merge on the owner's word.

## For Phase 4a

Hand the QA agent:
- R1 to R15, F1 to F7, N442 and the kept rows' texts;
- SPEC-009's, SPEC-013's, SPEC-007's, SPEC-022's and SPEC-012's changed
  paragraphs;
- the surfaces, as signatures and names, never the source:
  - the wire: `{"removeParticipant":{"participantId":"…"}}` in a
    `ClientCommand`, and its `CommandResult`;
  - `gateway.New(c, ids).Handler()`, `/ws`, `POST /join`,
    `GET /api/participants`;
  - `go doc ./internal/identity`: `CreateInvite`, `Lookup`, `List`,
    `Revoke`, `Verify`, `SetJoinOpen`, `JoinSecret`, `ErrInvalidToken`;
  - `newGWFixture` and its fields, and `sendCommand`, in
    `internal/gateway/server_test.go`;
  - `removeParticipant`, `renderDMConsole` and `DMDeps`;
  - `boot`, `Auth`, `Wire.onCredentialGone`, `Session.onCredentialGone`,
    `CREDENTIAL_GONE`, `ViewExtras.removed`; the storage keys `vtt.token`
    and `vtt.viewpoint.<token>`; `contract/testdata/credential_close.json`;
    `/api/me`'s 200, 401 and 503;
  - the socket fake in `client/test/app.test.ts`, which now passes a close
    event.

Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- its files: `internal/gateway/qa_remove_participant_test.go`, package
  `gateway_test`, held to the default comment ceiling, 25.0, for the
  removal and the 503; and `client/test/qa-boot.test.ts`, for the console
  and the removed window;
- a close is asserted by its status, never by a read that timed out;
  "refused" comes with a removal of a player that succeeds at the same
  table; "appends nothing" reads the head before and after;
- a grant to the participant while they are removed, an actor of theirs
  removed while they are removed, and a command of theirs judged before the
  removal are known and left open by the owner's rule on near-theoretical
  cases (D4 to D6). A test of them is not a finding;
- `identity.Revoke` does not refuse a participant already revoked (P7);
- the removed window shows the line in place of Reconnect and keeps the
  rest as last drawn (D19); it forgets a stored credential only while that
  is its own (D21);
- a refused handshake reaches a browser socket with no code or reason
  (P13), so a load is caught by `/api/me` alone (D22);
- while the identity store cannot answer, an unknown token gets 503 too:
  the answer follows the store, not the token (D22);
- a reason lost to a keepalive reap that coincides with the close is known
  and left open (D26);
- give each test a literal name and its rule's id;
- measure with `env -u CLAUDECODE bun test <file>`;
- the tree is hashed before and after it.
