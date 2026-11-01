# The DM removes a participant from the console: the change

**Ticket:** `docs/superpowers/specs/2026-10-31-the-dm-boots-a-participant-design.md`.
An agent that did not write it verified it and passed it with gaps G1 to
G10, then amended its plan for the owner's answers (G11, G12).
**Plan:** `docs/superpowers/plans/2026-10-31-the-dm-boots-a-participant.md`,
written by that verifier and amended by Task 0's review.
**Where it came from:** a candidate raised in
`docs/reports/2026-10-31-a-name-belongs-to-one-participant.md`:
"revoking one is `vtt revoke --id`, and the console shows no id". The
ticket states the problem from the code: a participant who lost their
credential rejoins under a new name, and the old one stays, holding their
characters. The owner asked how MapTool does it, then: "Kör boot player
som nästa ärende".
**The owner's rulings, all 2026-10-31:**
- Boot revokes the participant permanently, as `vtt revoke` does, whether
  or not they are connected, after a confirmation: "Återkalla permanent".
- Boot also revokes their control of every character: "Kontrollen tas
  bort också".
- The dm and the agent may remove a participant: "DM och agent".
- One ticket on one branch: "Ja, kör som ett ärende".
- The removed participant's window says so and forgets the dead
  credential: "Visa text och glöm behörigheten". The reload case is
  covered, with the read surface answering 503 rather than 401 when
  identity cannot answer: "Ja, täck reload med 503". A 401 at a load says
  "removed" too, though another campaign served on the same address can
  cause it: "\"Removed\" i båda fallen".

**The last code commit is `695457c`.** Every code reference below is to
that tree unless it names another commit.

## The period, in commits

| Commit | What it holds |
|---|---|
| `aa70855` | Task 0: the ticket, the plan, VTT-557 to VTT-581; VTT-442 and VTT-111 withdrawn |
| `7cffd44` | Task 1: `RemoveParticipant` in the contract, its tables and tool, and Boot in the console |
| `23d6784` | Task 2: the removal in the gateway |
| `957566e` | Task 3: the removed participant's window, and the read surface's 503 |
| `783a78f` | Task 4: the browser tests |
| `695457c` | QA's console test reads the tools table the mutation hash reads; the last code commit |

## What the code does now

- **The command.** `RemoveParticipant { participant_id }` is the
  `ClientCommand` oneof's arm 35. `commandRoles` gives it to the dm and
  the agent; the agent's `remove_participant` tool says it cannot be
  undone and revokes their control of every actor they control.
- **The console.** `renderDMConsole`'s "Who may do what" row of a player
  or a spectator carries Boot (`boot-<participant id>`), and a dm's or an
  agent's does not. Boot asks through `DMDeps.confirm`, "Boot Kim? Their
  sign-in stops working for good, and they no longer control any
  character. They can come back only through the share link while it is
  open.", sends nothing on a no, and on a yes sends one removal and reads
  the roster again when it is answered.
- **The removal.** `handleRemoval` in `internal/gateway/server.go` looks
  the target up and refuses one `Lookup` does not answer, a dm and an
  agent. It builds and stamps one `ActorControlRevoked` per actor whose
  controllers include the target, in actor-id order, then calls
  `identity.Revoke`, signals the target's connections through
  `presenceRegistry.rejudge`, and only then appends the batch, or nothing
  when the target controls none. The pump re-resolves its participant on
  the signal and closes with the policy-violation status and
  `credentialGone`, "gateway: credential no longer valid", before any
  event of the batch can reach it.
- **The removed window.** `Wire` compares a close with `CREDENTIAL_GONE`,
  the code and the reason both, which `contract/testdata/credential_close.json`
  pins on both sides. On that close, and on a 401 from the startup chain's
  metadata reads, `startSession`'s `forget` clears the stored credential
  while it is the window's own, removes the tab's viewpoint key under it,
  and makes `renderStatus` draw "You were removed from this table. To join
  again, open the link your DM sent you." in place of Reconnect. The board
  and the Story stay as last drawn.
- **The read surface.** `authed` answers `ErrInvalidToken` with 401 as
  before, and any other `Verify` error with 503 and "gateway: identity
  unavailable", so a 401 to a request carrying a bearer token now means a
  dead credential.

## Done looks like, answered

1. [x] **Boot revokes a player or a spectator.**
   - The console: `client/test/dm-view.test.ts`'s "Boot is offered on a
     player's and a spectator's row, and on no dm's or agent's" and "a
     confirmed Boot removes its row's participant and reads the roster
     again once answered". Both were red before Task 1's code.
   - The gateway: `TestTheDMRemovesASpectatorWhoIsNotConnected` in
     `internal/gateway/remove_participant_test.go`: `Lookup` answers
     `ErrInvalidToken` and `List` omits them. Red on `7cffd44`, where the
     server refused the command.
   - The name: `TestARemovedParticipantsNameIsFreeAtTheShareLink` joins as
     `kim` after `Kim`'s removal, 200.
   - In a browser: `client/e2e/booting.spec.ts`'s first test boots a
     player who holds a character and has them join again under the freed
     name.
2. [x] **A no sends nothing.** `dm-view.test.ts`'s "Boot asks first,
   naming the participant, and a no sends nothing", and QA's
   `TestQABootCancelSendsNothing`.
3. [x] **Every open connection closes at once, and the credential is
   refused afterwards.** `TestEveryConnectionOfARemovedParticipantClosesAtOnce`:
   a spectator who controls nothing, on two connections, sending nothing;
   both close with the fixture's code and reason, the head does not move,
   the DM is told `DISCONNECTED`, and a dial with their credential is
   answered 401. It goes red with the signal removed (K2), which is why
   its target controls nothing: the batch's append would otherwise close
   the connection anyway.
4. [x] **Their control goes, and nobody else's.**
   `TestARemovalRevokesEveryControlTheParticipantHolds`: five actors
   inserted out of order, a creature among them, one shared with another
   player, and a sixth the removed player does not hold. The batch is five
   `ActorControlRevoked` in actor-id order, each stamped with the DM's id
   and role; the shared actor and the sixth keep the other player; no kind
   changes.
5. [x] **A participant who is not connected is removed the same way.**
   `TestTheDMRemovesASpectatorWhoIsNotConnected`, and QA's
   `TestQARemovalOfASpectatorWhoIsNotConnected` (101 before, 401 after);
   in a browser, `booting.spec.ts`'s second test boots a player whose page
   presence shows gone.
6. [x] **Refusals.**
   `TestARemovalOfADMAnAgentTheIssuerAnUnknownOrARevokedTargetIsRefused`:
   another dm, the agent, the issuer itself, an id never minted and a
   player revoked by `ids.Revoke` who still controls an actor; each
   refused, nothing revoked, the head unchanged, the revoked player's
   control untouched; then the same DM removes a player. A player and a
   spectator issuing it: `TestAPlayerAndASpectatorMayNotRemoveAnybody`
   and the role table, `TestAuthorizeTableAllCommandsAllRoles`.
7. [x] **`vtt revoke` works as before.** It did not change; the
   `cmd/vtt` and `internal/identity` tests that run it pass whole in the
   gate (Figures).
8. [x] **`task check` exits 0 on the branch.** On `695457c`: `TASK_EXIT=0`,
   2026-11-01 from 01:36 to 01:55, the tree unchanged across the run. The
   first run, on `783a78f`, stopped at `ts_mutation_inputs_test`; see
   Deviations.

## The removed participant's window, by the owner's answer

Not one of the ticket's items: the owner's answer to the plan's Q3.

- **The open window:** `app.test.ts`'s "a window closed for a dead
  credential says it was removed in place of Reconnect, and forgets its
  credential and its tab's viewpoint", and the first browser test.
- **Another window's credential is kept:** "a dead credential's window
  keeps another window's stored credential".
- **Any other close forgets nothing:** `wire.test.ts`'s "a close says the
  credential is gone only with the credential's code and reason" and
  `app.test.ts`'s "a window closed any other way forgets nothing and
  offers Reconnect".
- **A load answered 401:** "a page whose /api/me answers 401 says it was
  removed and forgets its credential, and one answered 503 does neither",
  which ends the dial as a refused handshake does, and the second browser
  test.
- **The link afterwards:** "after a removal the share link opens the join
  form, and a page under the dead token names no log".
- **The 503:** `TestAnAPIRequestWhileIdentityCannotAnswerIs503` in
  `internal/gateway/metadata_test.go`, and QA's three tests in
  `internal/gateway/qa_me_503_test.go`.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A dm can remove a player or a spectator, revoking them as `vtt revoke` does | VTT-557 (connected or not), VTT-565 (only a dm or an agent), and the console's VTT-568 to VTT-571 |
| Removing a participant also revokes their control of every character | VTT-559 (one revocation each, in actor-id order), VTT-560 (other controllers keep it), VTT-561 (nothing when they hold nothing), VTT-562 (stamped with the issuer), VTT-563 (the result's sequence), VTT-567 (the tool says so) |
| A removed participant's open connections close at once | VTT-558, naming the reason, and VTT-566 (sent nothing the removal's own events cause) |
| A removal of a dm, an agent, oneself, or a participant unknown or already revoked is refused and changes nothing | VTT-564; "oneself" is inside it, since only a dm or an agent can issue one (G7) |

The window, by the owner's answer: VTT-572 to VTT-578 and VTT-580.
Withdrawn and succeeded: VTT-442 by VTT-579, made false once a window
forgets the viewpoint its tab remembers, and VTT-111 by VTT-581, made
false by the 503.

## Phase 4a

One QA agent for each of Tasks 1, 2 and 3, each given the rows, the
governing specifications and the exported surface, never the source, the
existing tests or a diff. Task 4 had none: it adds browser tests of
behaviour the three delivered.

| Task | QA's test cases | Failures |
|---|---|---|
| 1 | 10 | none |
| 2 | 8 | none |
| 3 | 21 | none |

Task 3's client file defines 11 tests, two of which run 5 and 4 cases;
with its 3 Go tests that makes 21.

### QA adjudications

- QA finding 1.1: spec gap — SPEC-013's table has no `remove_participant`
  row — the specification task adds it.
- QA finding 1.2: spec gap — the confirmation's words and Boot's place in
  the row — by design (D9); the rows hold the participant's name.
- QA finding 1.3: spec gap — whether the roster is read again after a
  refused removal — VTT-571 says "when its removal is answered", either
  answer, and the code reads it then.
- QA finding 2.1: spec gap — the result's sequence when nothing was
  appended — `TestARemovalOfSomebodyWhoControlsNothingAppendsNothing`
  holds it at 0; no row needed.
- QA finding 2.2: spec gap — VTT-566 is observable only for frames before
  the close — by its nature; `TestARemovedPlayerIsSentNothingTheRemovalCaused`
  counts what arrives.
- QA finding 2.3: spec gap — a log seat's removal — that same test removes
  ten players each watching their party member's log.
- QA finding 2.4: spec gap — a failure between the identity write and the
  append — unobservable from outside; D4 and D5.
- QA finding 3.1: spec gap — the line's words — by design (D19); the tests
  match "removed".
- QA finding 3.2: spec gap — a request with no bearer while identity
  cannot answer — VTT-578 names a bearer; a missing header stays 401
  (VTT-581).
- QA finding 3.3: needs an id — no 401 for a live credential on
  `/ws?token=` or `POST /join` during an outage — by design (D22):
  `handleWS` keeps its 401, since no browser reads a handshake's status,
  and `POST /join` carries no credential.
- QA finding 3.4: needs an id — the share link admitting them, not only
  showing the form — the join itself is SPEC-009's, and the first browser
  test rejoins.

## Phase 4b

One reviewer per commit, given the diff and the plan; `695457c`, one
changed line, was read whole without a reviewer agent.

| Commit | Must | Should | Nit |
|---|---|---|---|
| `aa70855` | 2 | 6 | 6 |
| `7cffd44` | 1 | 5 | 3 |
| `23d6784` | 1 | 4 | 4 |
| `957566e` | 0 | 1 | 5 |
| `783a78f` | 1 | 1 | 3 |
| `695457c` | 0 | 0 | 0 |

Every finding was taken or adjudicated but one, recorded here: the comment
above `authzCases` in `internal/gateway/authz_test.go` calls it "the full
88-cell matrix", and the assertion below reads 96. It was stale before
this branch: at `275b449` it stood over an assertion of 92. A second
"88-cell matrix" stands in the comment that begins "commandRoles says which
ROLES may issue a command". A fix must pass the six-line bound and the ban
on "spec §" across a 29-line history block.

The ones that changed what the work says:
- VTT-111 was made false by the 503 and not named, and the row for any
  other failure contradicted the row for a 401 at a load; it was split in
  two (VTT-576 and VTT-580).
- The tool's description said the characters "stand with no one", false
  of a shared one; the confirmation said "their link stops working",
  false for someone who joined by the share link.
- VTT-557's agent half had no test that observed it: with `Revoke` called
  for a dm alone, every removal test stayed green. QA's agent test now
  asserts the revocation.
- QA's order test was seeded with a sorted rotation and missed a lost sort
  one run in four; reseeded, it goes red every run.
- The two tests of a 401 at a load left the socket open, so neither saw
  Reconnect missing; one now ends the dial as a refused handshake does.
- The second browser test's name claimed a player, away, joining again;
  it observed only the line. It now promotes them, sees presence drop
  before the boot, and joins again.

## The deliberate breaks

Each was applied, its tests run, and undone by the inverse edit, the file
then compared with a copy of the green one; K9 is the exception, below.

| Break | Went red |
|---|---|
| K8: the role row given the spectator | the role table test's spectator cell. Given the player, as the plan wrote it, nothing went red: `authorizePlayer` refuses a player command no player rule decides |
| K14: the tool description without "cannot be undone" | its toolgen test |
| K15 to K18: Boot on every row, without its confirm, sending the first row's id, reading the roster beside the send | each its console test |
| K1: no `Revoke` | the not-connected, close, nothing-sent and free-name tests. Not the batch test, which does not look the participant up, contrary to the plan's K1. `23d6784`'s message names a "redial" test in place of the nothing-sent one; the redial is a step of the close test, run only after its close check |
| K2: no `rejudge` | the close test |
| K3, K4, K5, K9, K10, K11: the batch naming every actor, never appended, revoking every controller, unsorted, unstamped, the result without its sequence | the batch test |
| K6, K7: the dm and agent bound removed; a revoked target taken for a player | the refusal test |
| K12: the batch appended before the revoke and the signal | the nothing-sent test |
| K13: an empty batch appended | four tests: `AppendBatch` refuses an empty batch, so every removal of someone holding nothing was refused |
| K19, K20: the wire comparing the code alone, or the reason alone | the wire's close test and the other-closes test |
| K21: `forget` clearing whoever's credential | the other-window test |
| K22: `forget` leaving the viewpoint key | the removed-window and share-link tests |
| K23: the close never calling `forget` | three tests |
| K24: the startup chain forgetting on any failure | the 401/503 test and three existing tests whose `/api/me` fails otherwise |
| K25: the startup chain forgetting nothing | the 401/503 test |
| K26: the read surface answering 401 to every `Verify` error | the 503 test |
| K27: the client's reason off by a word | five tests |
| K28: Reconnect drawn beside the line | the removed-window test, and the 401 test once review made it close its socket |
| In the browser, the client's reason off by a word; the startup chain forgetting nothing | the first browser test; the second |

Twice a break's text was not unique in its file: K4 and K9 in
`server.go`, beside `handleRemoveActor`'s identical lines. K4's script
stopped before writing, and K4 was applied again at a unique anchor. K9
was applied and could not be undone by replacing its text back; it was
restored by hand with its unique neighbour, the `len(actorIDs)` line, and
compared equal to the green copy. From Task 3 on the script refused any
break whose broken text already occurred in the file: K22 and K26 stopped
there before writing, and were applied again with a unique marker.

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c`: `AppActions.BOOT_CONNECTED_PLAYER`
is offered to the GM and the host, refuses one's own player
(`msg.error.cantBootSelf`), asks "Are you sure you want to boot {0}?", and
reaches only a connected player. `MapToolServer.bootPlayer` releases that
player's connection; nothing on the server asks who sent it. The booted
client says "You have been booted from the server." and may connect again
under the same name, whose tokens it still owns. Borrowed: the word Boot,
the refusal of oneself, a confirmation naming the person, and a window
that says why it closed. Not borrowed: a gate in the client alone, a
disconnect alone, a reach to the connected alone, and untouched
ownership, by the owner's rulings; and reconnecting by name, since here
the credential is the identity.

## The plan's gaps

- **G1:** the MCP tool exists whatever the agent ruling, since `mcp.New`
  refuses an arm with no tool.
- **G2:** files the ticket did not name: the tools golden, the role and
  conversion tests, the two MCP name lists, `door_tools.go`, the surface
  test, the bundle.
- **G3:** SPEC-011 moves by a word.
- **G4:** the browser tests went in a new file that runs twice against
  one campaign, since `joining.spec.ts` cannot (recorded as open debt).
- **G5:** what the removed window showed before: "closed" and a Reconnect
  that got 401, with the dead credential stored and winning over the link.
- **G6, G7, G8:** "at once" bound by VTT-558's "without waiting for a
  command of theirs or an append", by naming the reason, and by the
  nothing-sent row; the issuer inside the role bound; a revoked target
  refused by `Lookup`, since `Revoke` does not refuse one.
- **G9:** the ticket's open points, below.
- **G10:** every file within its comment ceiling, and `dm.ts`'s head
  comment, false once Boot asked first, rewritten as a warning with a
  pointer (below).
- **G11, G12:** the owner's answer reached `wire.ts`, `session.ts`,
  `app.ts`, `spectator.ts` and `metadata.go`, and moves SPEC-012 and
  SPEC-022; a 401 meant a dead credential only once `authed` stopped
  answering it to an outage.

## The measurements the plan stood on

- **P5, the order of the writes.** In a scratch test, a connected player
  controlling two party members: revoking and signalling before the
  append sent them a frame before the close in 0 of 20 runs; appending
  first, in 18 of 20. Hence D4's order and VTT-566.
- **P6, the window a grant can slip through.** From the snapshot to
  `AppendBatch`'s return, two revocations, 200 runs: median 0.36 ms, p99
  0.84. `Lookup` plus `Revoke`, 200 runs: median 0.29 ms, p99 4.46. Each
  timed the real calls separately; summed, about 0.65 ms at the medians
  and 5 ms at the p99s.
- **P14, the 401 before this ticket.** `/api/me` answered 401 to a revoked
  token, 200 to a live one, and 401 to the live one while `participants`
  could not be read: `Verify`'s error was then not `ErrInvalidToken`.
  Forgetting on that 401 would have deleted a live credential's only copy,
  so the read surface now answers 503 (G12, D22).

## History cut from the code (rule 10)

`client/src/view/dm.ts`'s head comment called the console "client spec
§4" and said: "Exactly one of them asks first, and it is the one whose
damage lands OUTSIDE the room: rotating the join link strands every copy
of the old one, including the copies held by people who are not here to
be told", and that invite management stayed CLI as a "spec §4 non-goal".
Boot made the first false. It is now a five-line warning: ask first
before an action nothing undoes, citing VTT-569.

## The specifications

The ticket's SPEC-007, SPEC-009 and SPEC-013, SPEC-011 by G3, and SPEC-012
and SPEC-022 by G11, move in the commit after this report's.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Task 1 ships Boot without touching other browser tests | `joining.spec.ts`'s promote locator changed in Task 4 | Boot on a spectator's row made `.roster-row:has-text("Robin") button` match two buttons; Task 1's own checks ran no browser test, so `task e2e` found it in Task 4 |
| The second browser test opens the link after the load | it joins by it, after promotion and a presence check | Task 4's review: the test's name claimed more than it observed |
| K8 as the plan wrote it (the player's cell) | the spectator's cell | the player's cell is refused anyway by `authorizePlayer` |
| The plan's K1 reds the batch test | it does not | the batch test does not look the participant up; the not-connected test holds the revocation |
| `metadata.go` takes no comment line (P15) | a citation-only line, `// VTT-578`, on the 503 branch | the implementer wrote a two-line warning there against folding the branch back into the 401, which the plan had not asked for; it pushed `metadata.go` over its ceiling, 12.08 against 11.8, so it became the citation-only line, which counts in no share |
| Task 4's commit is the last code commit | `695457c` follows it | `task check` on `783a78f` stopped at `ts_mutation_inputs_test`: QA's `client/test/qa-boot.test.ts` imported `cmd/vtt/tools.json`, which the client mutation report's input hash does not read (VTT-516). It now imports `contract/gen/tools/tools.json`, the byte-identical file the hash names. That test runs only in `task check`, so neither Task 1's commit hook nor its review saw it |

## What could not be established

- **A failure between the identity write and the append** leaves the
  participant revoked and closed while their control stands, the state
  `vtt revoke` leaves; the DM's Revoke buttons on the character rows clear
  it. Not observable from outside, and left by the near-theoretical
  ruling (D4, D5).
- **A grant to the participant being removed, landing between the
  removal's read of the state and its append,** leaves that grant
  standing. The window is P6's, below; no code (D5).
- **A command in flight from the removed participant.** One whose
  `Lookup` ran before the `Revoke` can land after the removal, and a
  handshake verified just before it is announced `CONNECTED`, then
  `DISCONNECTED`. Nothing is half-applied, and both take the participant
  acting within the same millisecond; no code (D6).
- **Another campaign on the same address.** One `vtt.token` per origin, so
  a token from campaign A meets campaign B's 401 and the window says
  "removed", by the owner's choice; the DM returns to A by its invite
  link.
- **A 401 from a later step of the startup chain** (the ruleset, the
  adventures, the maps) forgets too. That is correct, since a 401 now
  means a dead credential, but no row states it.
- **QA's 503 test lists the `/api/` routes by hand,** so a tenth route
  would go unchecked against VTT-578.

## What was deliberately left out, and where it went

- The stale "88-cell" history above `authzCases`: recorded here.
- A removed window still draws its board and panels, its Say among them;
  pressing Send there is refused by a closed wire. By D19, not changed.
- The client's full mutation run. `client/src` and `client/test` changed,
  so the stored report is out of date until the owner asks for
  `task check:ts-mutation:full`.

## Figures

From `task check` on `695457c`, 2026-11-01, after `go clean -cache`, with
30.2 GiB free when the Go mutation gate began.

- **Go tests:** every package passed, `internal/gateway` (119 s),
  `cmd/vtt` (199 s), `internal/identity`, `contract` and `tools/toolgen`
  among them, under `-race` too.
- **Go coverage:** 21 packages at or above their floors; `internal/gateway`
  94.8% (floor 93.5), `internal/identity` 93.0% (92.1), `cmd/vtt` 87.1%
  (85.0), `tools/toolgen` 88.2% (85.0).
- **Client coverage:** 24 files gated, each at its floor; `app.ts`,
  `wire.ts`, `session.ts`, `commands.ts`, `view/dm.ts` and
  `view/spectator.ts` at 100%.
- **Go mutation:** 15 packages, zero unadjudicated survivors;
  `internal/gateway` 2 survivors, both adjudicated before this branch. Six
  mutants timed out and were counted as killed, NOT measured: one in
  `internal/perceive`, four in `internal/sight`, one in `internal/mcp`,
  none in a file this branch touched.
- **Client mutation:** not measured. The stored report measured
  `526885bc0c77`; since then the tickets for the console lock keys, a name
  belonging to one participant, and this one. `task check:ts-mutation:full`
  measures it when the owner asks.
- **Requirements chain:** 581 rows, 333 test files, 17 specifications;
  every citation resolves.
- **New prose:** 2117 added lines across 39 files, clean.
- **Comments:** 346 files, 26 added comment lines, clean.
- **Browser tests** (outside `task check`): `task e2e` on `783a78f`, 34
  passed and 1 skipped; `booting.spec.ts` passed under
  `--repeat-each=2`.
