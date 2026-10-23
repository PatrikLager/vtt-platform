# Plan: a player who controls no party member watches as a spectator does

- **Ticket:**
  `docs/superpowers/specs/2026-10-23-a-player-without-a-character-watches-as-a-spectator-design.md`,
  from ruling B-4 of `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`,
  the fourth ticket in that note's order of work.
- **Verdict:** passes, with one gap, which the ticket names itself (G1).
  - Verified 2026-10-23 by an agent that did not write the ticket, read-only
    against HEAD `f2dd819`.
  - The first verification returned the ticket on four findings:
    - F1: SPEC-021 was not named;
    - F2: an e2e test and two gateway test files were missing from its scope;
    - F3: `expected_tools.json` was called regenerated;
    - F4: VTT-402 and VTT-406 were hedged.
  - The writer revised the ticket. The second verification checked those four
    findings alone, and all four are fixed.
  - Probes ran outside the tree, in `git archive HEAD` copies:
    - a prototype of the ticket's first three rules, run over the whole Go
      tree;
    - prototypes of the client options for Q1;
    - scratch gateway and client tests;
    - breaks K1-K9 and Kc1-Kc4.
  - `task check`, `task e2e` and the mutation gates were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-23-a-player-without-a-character-watches-as-a-spectator.md`.
- **Base:** main `f2dd819`.
- **Branch:** `fix/a-player-without-a-character-watches-as-a-spectator`.
- **Owner's rulings taken in:**
  - **B-4, 2026-10-22.** "A player whose character the DM has taken watches as
    a spectator does." The role stays player. What changes is whom they may
    watch while they control no party member.
  - **"Stay on Tobbe", 2026-10-23.** A player watching their only party member
    when the DM revokes it keeps watching that party member.
  - **"One ticket, server first", 2026-10-23.** One branch, tasks in the order
    the ticket's "What it touches" gives.
  - **"Field on the list", 2026-10-23.** Q1 is answered: the browser learns it
    from an additive field on `WatchableChanged` (D5).
  - **Earlier rulings that stand:**
    - "Bara knappar", 2026-10-20: a spectator switches by its buttons alone;
    - the server sends whom one may watch (`WatchableChanged`);
    - kind and control are separate;
    - a table has one DM in practice.
- **Signed off:** 2026-10-23, by the owner ("Ja, kör").

## The gap this plan inherits

- **G1. The console's Revoke button has never run under `client/e2e/`.** It is
  the ticket's own open point, and the only one left.
  - Task 3's e2e is the first browser test to click it:
    `.control-actor[data-actor=…] button[data-action="revoke-actor-control"]`
    in `client/src/view/dm.ts`, one per holder.
  - Its unit coverage is `client/test/dm-view.test.ts`.
  - Whether it works in a real browser shows the first time Task 4 runs
    `task e2e`.

## Established here, not in the ticket

- **E1. The set alone cannot tell the browser whom a player controls.**
  - Take a table whose only party member is the player's. Revoking it leaves
    the set's actors identical, `{a1:Hero}` before and after (P5).
  - The revocation sends no `ViewpointChanged` either, because the player stays
    (P5).
  - So a table where the player holds Tobbe and one where Tobbe is someone
    else's send the same set.
  - This forced Q1's field.
- **E2. An empty set no longer arrives as `{"watchableChanged":{}}`.**
  - An empty set means no party member is watchable. Under D4 that means no
    party member counts the participant among its controllers, so every empty
    set carries `controlsNoPartyMember: true`.
  - SPEC-007's sentence about `{}` changes (Task 6).
  - `TestQAWatchableAConnectionWithNoneIsSentAnEmptySet`'s raw check changes
    with it (Task 2).
  - `contract/roundtrip_test.go`'s `TestAnEmptyWatchableSetStillSerializesAsItsFrame`
    stays true of an empty message, and stays.
- **E3. Today's bundle refuses the new key.**
  - `Wire.handleFrame` calls `fromJson` without `ignoreUnknownFields`.
  - On HEAD's schema `{"watchableChanged":{…,"controlsNoPartyMember":true}}`
    throws `key "controlsNoPartyMember" is unknown` (P7).
  - Regenerating `contract/gen/ts` alone changes
    `cmd/vtt/webdist/assets/index.js` (P7).
  - So the commit in which the gateway first sends the field must carry the
    rebuilt bundle (D6).
- **E4. A player with no party member has no narration box today.**
  `renderPlayerPanel` returns at an empty set before its "Say something"
  section. So a perched player losing the panel loses nothing that such a
  player has today.
- **E5. Nothing outside `internal/gateway` encodes today's rule in Go (P2).**
  Under the prototype, `cmd/vtt` passes (the scenario library and the MCP e2e
  included), and so do `internal/harness` and `internal/mcp`. The same holds
  with the field added (P2b).
- **E6. Two of the tests the ticket's first draft named stay green and keep
  their meaning (P2).**
  - They are `TestQASeatAPlayerWithNoPartyMemberWatchesNobodyUntilGranted` and
    `TestQARejudgeControllingOnlyANonPartyActorWatchesNobody`.
  - In both, a player who controls none and names no party member still opens
    on nobody, so neither is edited.
- **E7. The revocation's own entry reaches a player who stays on the revoked
  one (P5).** Order is the new set, then the `ActorControlRevoked` entry.
- **E8. The ticket's earlier open point, "the remembered viewpoint across a
  grant", is closed (P8).**
  - Under the prototype, take a player who controls none and whose dial names
    `a2`. They are granted `a3`, then dial again naming `a2`.
  - The dial opens on `a3` with the flag false.
- **E9. Prose outside the gateway that states today's rule:**
  - `set_viewpoint`'s description in `tools/toolgen/main.go`;
  - `SetViewpoint`'s comment in `contract/vtt/v1/commands.proto`;
  - `mayPerch`'s doc in `client/src/app.ts`;
  - in `client/src/view/spectator.ts`, the perch field's doc ("The spectator's
    perch; absent for every other role") and the comment in `renderSpectator`
    ("Only the spectator is ever given one, and a spectator has neither a panel
    nor a console");
  - the perch template's comment in `client/src/style.css` ("Only a spectator
    is ever sent this control"), which no gate reads;
  - in `client/test/app.test.ts`, the name and comment of "only a spectator is
    offered a shoulder", which no register row cites;
  - in `client/test/spectator-view.test.ts`, the comment in "no perch
    supplied, no control at all" ("only a spectator chooses a shoulder");
  - SPEC-021's consequence.

  Found, and left alone:
  - `client/src/commands.ts`'s `setViewpoint` comment ("`""` is how a
    spectator watches nobody") stays true. It is narrower than the new rule,
    and not false.
  - `internal/mcp`'s server instructions, `internal/harness` and `scenarios/`
    state no watching rule.

## Measurements this plan stands on

Every probe ran outside the repository, at `f2dd819`. The copies:

- the untouched copy, `base`;
- the server prototype, `copy` (`MayWatch` and `judgedBy` changed, then the
  field and the client's option A added);
- the client's option B, `copyB`;
- the dedup break, `k7`.

| # | Probe | Result |
|---|---|---|
| P1 | `go test ./internal/gateway/`; `env -u CLAUDECODE bun test client/test contract`, untouched copy | gateway ok; 1381 pass |
| P2 | Server rule prototype: `MayWatch` lets a player who controls none watch any party member or nobody; `judgedBy` asks control, not `MayWatch`. `go test` over every package | 23 red, all in `internal/gateway`, listed under Task 1. Every other package ok, `cmd/vtt` included |
| P2b | P2 plus the field (D5) | the same 23 red in `internal/gateway`; `contract` and `tools/toolgen` ok; every other package ok |
| P3 | HEAD, live connection: a player who controls nothing asks for `a2`, then for nobody | both refused, the first with `"a2" is not a party member you may watch`, the second with `a player watches one of their own party members` |
| P4 | HEAD, the player holds `a1`, `a2` is uncontrolled; revoke `a1` | an empty set, then `ViewpointChanged("")` |
| P5 | Prototype, the same two tables; frames after the revocation | one party member: only the revocation's entry. Two: `Watchable[a1 a2]`, then the entry, no switch. Re-grant: `Watchable[a1]`, then the entry |
| P5b | P5's one-member table with the field | a set with the same actors and the flag true, then the entry |
| P5c | P5b with `tell`'s dedup comparing actors alone (K7) | only the entry: no set |
| P6 | Client option A on the whole client suite, then options B and A' (B: the browser derives it from its fold; A': the field's opposite polarity) | A: 1381 of 1381 pass. B: 6 red (below). A': 40 red |
| P7 | `fromJson` of the new key on HEAD's schema; `vite build` before and after regenerating `contract/gen/ts` alone | throws `key "controlsNoPartyMember" is unknown`; `index.js` changes (HEAD's build reproduces its committed hash) |
| P8 | Prototype, dials of a player who controls none | names `a2`: opens on `a2`, the set every party member, the flag true. Names another's `a1`: opens on `a1`. Names none: nobody. After a grant of `a3`, names `a2`: opens on `a3`, the flag false |
| P9 | Scratch client tests zz1-zz3 (shapes of C1-C3) | on the tree with the contract regenerated and the client unchanged: zz1 and zz2 red at the `.perch` count. On option A: all green |
| P10 | Breaks (below), each applied, run, undone | see Tasks 1-3 |
| P11 | `check-comments.py`'s `measure` on HEAD and on the prototype | the comment budget below |
| P12 | Item 4's two edits on HEAD and on the prototype | the `log=pc-c` edit: red on HEAD (`ViewpointChanged names "", want "pc-c"`), green on the prototype. `TestQAMGLosingTheOnlyPartyMemberLeavesNoTokenMovable` with only its "watching nobody" line removed passes on HEAD, so it must also assert the stay (Task 1) |

P6's six red under option B:
- in `app.test.ts`: "a player gets the panel but not the console";
- in `qa-events-panel.test.ts`: two qaEP tests that re-read a role at a
  presence frame;
- in `qa-reload-viewpoint.test.ts`: three qaRV tests, among them "qaRV: a
  player's chips mark nobody before the switch arrives, the remembered actor
  included".

The last one no fixture can repair. Before its first `ViewpointChanged` a
player holds no log, so option B draws the perch for every player at every page
load until their log arrives.

Not measured: `task e2e` (a verifier limit), the mutation runs, and the edited
23 tests themselves, apart from P12's two.

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no ceiling, floor or exemption rises.
    - `tools/comment-ceilings.txt` rows only fall, by
      `check-comments.py --write-ledger`, when a file ends more than 1.0 under
      its ceiling.
    - `tools/mutation-equivalents.txt` and `tools/ts-mutation-equivalents.txt`
      gain no row for code this plan writes.
  - 3: the one contract change is additive: a new field, number 2, on
    `WatchableChanged`. `check:breaking` only reports pre-release, so nothing
    but this rule guards it.
  - 4 and 5: no fold and no game-system word is added.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment is a warning, a pointer or an exported symbol's one-line doc,
    with no date, hash or task beside the code.
- **The comment budget (P11).** Shares compared unrounded. Run
  `python3 tools/check-comments.py main` after staging (memory: a move shows
  only then).

  | File | main | Prototype | Ceiling | Room |
  |---|---|---|---|---|
  | `internal/gateway/viewpoint.go` | 7 of 61, 11.48% | 7 of 72, 9.72% | 11.5 | one added comment line at about 72 lines. A finish more than 1.0 under needs `--write-ledger` |
  | `internal/gateway/seat.go` | 33 of 207, 15.94% | 33 of 207, 15.94% | 16.0 | none: `judgedBy`'s doc and `told`'s comment are reworded in place |
  | `client/src/app.ts` | 262 of 530, 49.43% | 262 of 531, 49.34% | 49.7 | one line at most. `mayPerch`'s doc is reworded in place |
  | `client/src/session.ts` | 64 of 179, 35.75% | 65 of 185, 35.14% | 36.0 | the getter's one-line doc fits |
  | `client/src/wire.ts` | 79 of 264, 29.92% | 79 of 264, 29.92% | 30.6 | ample |

- **The adjudication on `watchable`.**
  - `tools/mutation-equivalents.txt` holds
    `internal/gateway viewpoint.go CONDITIONALS_BOUNDARY offset=80`, on the
    sort line.
  - Keep that line byte-identical (SPEC-020 keys it by its code).
  - Keep its reason true: "nothing in that loop writes to `actors` (MayWatch
    only reads it)". D1's predicate only reads.
- **Requirement ids** come from `requirement-id` after sign-off, never by hand
  (SPEC-008).
  - A test or spec that cites an id needs the row first.
  - An e2e test named as evidence carries the id in a `// VTT-NNN` line.
  - A renamed test's register cells change in the same commit, or
    `check:requirements-chain` refuses the stale name.
  - No test cites a withdrawn id once its successor exists. The convention,
    checked by grep, is that VTT-183, 141, 155, 177, 417 and 086 are cited
    nowhere.
- **The citations to re-point** (counted by grep at `f2dd819`):

  | Id | Test lines | Source pointers |
  |---|---|---|
  | VTT-343 | 23, in 8 files | none |
  | VTT-346 | 3, in 3 files | none |
  | VTT-351 | 16, in 2 files | 2 in `seat.go`: the `told` field's comment and `judgedBy`'s doc |
  | VTT-402 | 4 | 3: `wire.ts`'s `onWatchable`, `session.ts`'s `watchable`, `player.ts`'s `Switcher` |
  | VTT-406 | 13 | 1: `player.ts`'s `playerClickFor` |

  The last task to touch a file re-points it.
  `grep -rn -E 'VTT-(343|346|351|402|406)([^0-9]|$)' internal cmd client tools`
  prints nothing after Task 3.
- **Tier 1 runs at pre-commit:**
  - `go test` tier 1;
  - `bun test client/test contract contract-spike`;
  - `task client:typecheck`;
  - lint, vet, arch, vocabulary, secrets.
- **bun is quiet under `CLAUDECODE`.** Run `env -u CLAUDECODE bun test …`.
- **The bundle.**
  - Each commit that changes `contract/gen/ts` or `client/src` carries
    `cmd/vtt/webdist`, rebuilt by `task build:client` (E3, D6). The commit hook
    does not catch a stale bundle.
  - `check:drift` passes only after the commit. Pre-push runs it.
- **Scheduling tests.** The promotion and watchable tests Task 1 edits race the
  pump. Run them with `-count=3 -cpu 1,2` before each commit (memory: a test
  that relies on goroutine order is green at the default GOMAXPROCS and red at
  `-cpu 1`).
- **Pushing.** The pre-push hook takes about three minutes. Check the load with
  `os.getloadavg()` first, and never bypass the hook.
- **`review-record` refuses untracked files.** Commit the ticket and this plan
  in Task 0.
- **Mechanics.**
  - A new test goes after a closing brace.
  - Stage and commit in separate calls.
  - Undo a break with its inverse edit and check with grep, never
    `git checkout --`.
  - Hash the tree before and after any agent that reads it.

## Rule 9: how MapTool does this

Checked read-only in `~/dev/RPTool/maptool`, at `f4b7fef6c`:

- **What MapTool does.**
  - `ZoneViewModel.makePlayerView` builds a player's view from their selected
    tokens that have sight and that `AppUtil.playerOwns` passes.
  - Without such a selection it uses `Zone.getOwnedTokensWithSight` when
    `ServerPolicy.isUseIndividualViews` holds (or on a personal server).
    Otherwise it uses `Zone.getPlayerTokensWithSight`, every PC token with
    sight, owned or not.
  - `useIndividualViews` is a field no constructor sets, so it is false by
    default, and the start-server dialog's preference also defaults to false
    (`StartServerDialogPreferences.getUseIndividualViews`).
  - By default, then, a player who owns no token already sees through the whole
    party. With individual views on, such a player gets a `PlayerView` with no
    tokens.
  - A move is gated by ownership alone: `PointerTool` asks
    `AppUtil.playerOwns`, which passes every token when strict token management
    is off. What the player's view sees never gates a move.
- **What we borrow.** A player with no character of their own sees through the
  party.
- **What we do not borrow.**
  - The union. MapTool shows every PC's sight at once. Here a player who
    controls none chooses one party member's shoulder, as a spectator does,
    since a client holds one log (SPEC-021, SPEC-022).
  - The ownership-only move rule. A move here is judged by the eyes of a party
    member the player controls, and a player who controls none moves nothing
    (rule 3, D3). With today's `judgedBy` kept, a perched player would move
    their non-party pet wherever another player's character sees, and be told
    what blocks a square through that character's eyes (P10, K4). MapTool's
    rule is the shape this ticket forbids.
  - The distribution, which rule 9 always excludes. MapTool sends every client
    the whole campaign and filters the view on the client. Here the gateway
    sends a connection the one log it watches (SPEC-015).

## Design questions for the owner

**Q1. How the browser learns that a player controls no party member.**
Answered 2026-10-23: "Field on the list", the recommendation (D5).

- **Recommended: an additive field on `WatchableChanged`.** Measured:
  - 0 of 1381 existing client tests go red (P6).
  - The field adds no Go red beyond the rule's own 23 (P2b).
  - A change in the flag alone re-sends the set, before the revocation's entry
    (P5b).
- **Rejected: deriving it from the client's fold.**
  - 6 of 1381 go red, one beyond repair (P6).
  - At a revocation the set arrives before the entry (P5), so for that interval
    the player panel would offer a chip for every party member, others'
    included.
  - It also goes against SPEC-015's "never from its log's roster" and
    qa-switch's "the chips are exactly the set, not the actors the folded log
    shows under their control".
- **Rejected: `/api/me` carrying it.** A grant or a revocation sends no
  presence frame, so nothing re-reads `/api/me` then.
- **Rejected: a new frame arm.** It needs an order of its own and a dedup of
  its own, which a field on `WatchableChanged` already has.
- **Rejected: a flag per actor.** The default polarity reads every listed actor
  as not the player's, and the flag is redundant: under the rule a player's set
  is all theirs or none theirs.

No other question is open. A perched player has no narration box. That matches
both the ticket's rule ("no player panel") and today's player with no actor
(E4). B-2, which comes later, decides whether a player without a character may
speak.

## Decisions

1. **D1. One predicate decides "controls a party member".**
   - An unexported helper in `viewpoint.go`, true when some actor is a party
     member (`engine.IsPartyMember`) and `controls` it for the participant's
     id.
   - `MayWatch`, `defaultViewpoint`, `judgedBy` and the flag (D4) all ask it.
   - Forced by K9. A flag predicate that also counted a non-party actor was
     caught by no existing test (P10). Two definitions of one fact drift (the
     owner's ruling of 2026-09-23: one source).
   - It reads `actors` only, which keeps the adjudication's reason true.
2. **D2. `MayWatch` treats a player who controls no party member as it treats
   a spectator.**
   - For a player who controls one, nothing changes.
   - For one who controls none, it accepts the empty id and any party member.
   - Every refusal of a named actor keeps today's one string, for every role
     (VTT-347, VTT-182), and the empty id's refusal keeps its own.
   - Forced by rules 1 and 2. K1, K2 and K3 each go red (P10).
3. **D3. `judgedBy` stops asking `MayWatch`.**
   - It answers the told viewpoint while that is a party member the player
     controls (D1), and `defaultViewpoint` otherwise.
   - Every answer equals today's, for every player. Today the told one passes
     `MayWatch` exactly when it is a party member the player controls, so the
     change only keeps the gate where it is once `MayWatch` widens.
   - Forced by rule 3. With `judgedBy` left on `MayWatch` (K4), item 4's two
     tests go red on moves accepted where another player's character sees, and
     on the crate square answered "something (a crate) is in the way" (P10).
   - `defaultViewpoint` is unchanged in behaviour. It still gives a player who
     controls none nobody, as a spectator's default is nobody.
4. **D4. The flag is a fact, set on every log seat's set.**
   - `watchable` sets it true when D1 finds no party member the participant
     controls, whatever the role. A world seat is sent no set (VTT-401).
   - Forced by the ticket's rule: "A player's or a spectator's connection is
     told whether its participant controls no party member."
   - `tell` needs no change. Its `proto.Equal` dedup covers the new field, so a
     change in the flag alone queues the set (P5b), and K7 shows the dedup is
     what does it (P5c).
5. **D5. The field is `bool controls_no_party_member = 2` on
   `WatchableChanged`.**
   - The name and polarity are forced. The default `false` must read as a
     player who controls one, today's panel. The positive polarity
     (`controls_a_party_member`) turns 40 of 1381 existing client tests red,
     because every fixture that sends a player a set without the flag would
     draw the perch (P6, A'). The negative turns none.
   - Number 2 is the message's next free number.
   - Its proto comment is one warning: keep `false` meaning the participant
     controls a party member, since a client draws a player the perch by
     `true` (SPEC-022).
   - `SetViewpoint`'s comment ends: "An empty actor_id is nobody, which a
     player who controls a party member may not ask for (SPEC-007, SPEC-015)."
6. **D6. The field, its gateway code and the rebuilt bundle land in one
   commit, Task 2.**
   - The field is born in the contract, so `watchable` cannot set it earlier.
   - Today's bundle throws on the key (E3), so a commit whose gateway sends it
     without the rebuilt bundle would break every perched page in the browser.
   - The ticket's "What it touches" item 2 says the same.
7. **D7. The browser draws a player the perch while their last set says they
   control none.**
   - `wire.ts` hands `onWatchable` handlers a second argument, the flag. The
     one zero-argument handler in `client/test/wire.test.ts` still compiles.
   - `Session` keeps it beside the set as `controlsNoPartyMember`, which a
     discard does not touch, as it does not touch the set.
   - In `paint`, `mayPerch` becomes: the role is `spectator`, or the role is
     `player` and that flag is true.
     - A perched player gets no player panel: `canAct` is false for them.
     - Their `onCell` is `undefined` ("Bara knappar").
     - They are left out of `isPlayer`'s selection sync.
   - Forced by Kc2, Kc3, Kc3b and Kc4 (P10):
     - The opening `ViewpointChanged` discards, so a flag cleared on discard is
       lost at once (Kc4).
     - A wired board click finds a listed party member's token or a controlled
       non-party token to act on (Kc3, Kc3b).
   - `client/src/view/player.ts` and `client/src/view/spectator.ts` change no
     code: `player.ts` is edited only to re-point its VTT-402 and VTT-406
     pointers, and `spectator.ts` only to reword its two comments E9 names, in
     place. `renderPerch` takes any caller's set.
8. **D8. The revocation e2e is a second test in
   `client/e2e/handover.spec.ts`.**
   - That file is the one about control, and its first test leaves the player
     holding Ash and Bram on the file's own server.
   - Revoking in `table.spec.ts` would take Lera from the tests after it.
   - Playwright runs one worker, serially (`playwright.config.ts`).
9. **D9. The 23 red tests keep what they were written to hold (Task 1).**
   - Where a test's name becomes false, it is renamed and its register cells
     change with it.
   - The renamed promotion tests' old fixture, a promoted player who controls
     none, is kept as a new test of the stay. The behaviour has changed, not
     vanished.
10. **D10. An absence is read only after an anchor, and as a count.**
    - `.player` and `.ticker` count 0 once `.perch` is visible, and vice versa,
      never before `/api/me` answers.
    - In unit tests, `querySelectorAll(…).length` rather than `toBeNull()`,
      whose failure takes 20-30 s to print (P9 of
      `2026-10-23-the-events-panel-is-the-dms-alone`).
11. **D11. Withdrawals at Task 0, re-pointing per task.**
    - VTT-343, VTT-346, VTT-351, VTT-402 and VTT-406 are marked withdrawn when
      their successors are allocated.
    - Each test citation moves to its successor in the task that edits that
      test's file, and nothing cites a withdrawn id after Task 3.

## Requirements

Allocated by `requirement-id` at Task 0, after sign-off, as VTT-450 to
VTT-461 in the order R1 to R12, and written `**OPEN — no test yet**`. The sort ran over the ticket's five rules and the
withdrawals it names. Each row below gives the observation that would go red if
it broke.

- **Withdrawn**, each marked
  `WITHDRAWN 2026-10-23: <text>. <why>. Succeeded by VTT-NNN.`, with evidence
  `**READING — verify-ticket check 5 of 2026-10-23; withdrawn, held by nothing**`:
  - VTT-343, false once a player who controls none may watch any party member.
    Succeeded by VTT-450 (R1) and VTT-452 (R3).
  - VTT-346, false once such a player may watch nobody. Succeeded by VTT-451
    (R2) and VTT-452 (R3).
  - VTT-351, false once a connection shows a party member its player does not
    control. Succeeded by VTT-453 (R4).
  - VTT-402, false once such a player is offered "No shoulder" too. Succeeded
    by VTT-457 (R8) and VTT-458 (R9).
  - VTT-406, false once such a player's board click sends nothing. Succeeded by
    VTT-460 (R11) and VTT-461 (R12).
- **VTT-450 (R1).** "A `set_viewpoint` from a player who controls a party member is
  accepted only for a party member they control."
  - Evidence: VTT-343's current tests, re-pointed, with
    `TestQACreatureSetViewpointNeedsAPartyMemberTheyControl` as Task 1 edits it.
  - Red under K1.
- **VTT-451 (R2).** "A `set_viewpoint` naming no actor from a player who controls a party
  member is refused."
  - Evidence: VTT-346's current tests, re-pointed.
  - Red under K3.
- **VTT-452 (R3).** "A player who controls no party member may watch any party member, or
  nobody, and no other actor."
  - Evidence:
    - N1;
    - N3;
    - N4's perch;
    - `TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken` as edited;
    - `TestQARejudgePromotedSpectatorWhoControlsNoneKeepsItsPerch`.
  - Red under K2. The refusal half of N1 is red under a `MayWatch` that
    accepts any actor for such a player.
- **VTT-453 (R4).** "At the command's snapshot, a player's move is judged by
  the eyes of the party member their connection shows while they control it,
  otherwise by their first party member by id, or by none when they control no
  party member."
  - Evidence:
    - VTT-351's current tests, re-pointed;
    - N4;
    - the two move-gate tests as edited.
  - Red under K4 and K5.
- **VTT-454 (R5).** "Every `WatchableChanged` sent to a player's or a spectator's
  connection says whether no party member counts its participant among its
  controllers."
  - Evidence:
    - N7;
    - the contract round trip (Go and TS);
    - `TestQAWatchableAConnectionWithNoneIsSentAnEmptySet` as Task 2 edits it.
  - Red under K6 and K9.
- **VTT-455 (R6).** "A player's or a spectator's connection is sent a new
  `WatchableChanged`, before any entry of the append that changed it, whenever
  whether its participant controls no party member changes, even when the
  party members it may watch do not."
  - Evidence: N8.
  - Red under K7 (P5c).
- **VTT-456 (R7).** "An event that leaves a player controlling no party member
  does not move a connection of theirs off the party member it watches, when
  that one is still a party member after the event."
  - Evidence:
    - N6;
    - N8;
    - `TestQAWatchableARevocationChangesOnlyTheRevokedPlayersSet` and
      `TestQAMGLosingTheOnlyPartyMemberLeavesNoTokenMovable`, as edited.
  - Red under K8.
- **VTT-457 (R8).** "A browser window whose participant's role the client last
  read as `player`, where the last `WatchableChanged` the client was sent does
  not say they control no party member, offers a switch button for exactly the
  party members that set names, and none before the client is sent one, except
  while it shows the reason it cannot fold its log."
  - Evidence: VTT-402's current tests, re-pointed, and C2.
  - Red when chips come from the fold, which the qa-switch tests already
    catch.
- **VTT-458 (R9).** "A browser window whose participant's role the client last
  read as `player`, where the last `WatchableChanged` the client was sent says
  they control no party member, draws the perch, with a switch button for
  exactly the party members that set names and one for nobody, and no player
  panel, except while it shows the reason it cannot fold its log."
  - Evidence: C1, C2 and W1 (the new e2e).
  - Red under Kc1, Kc2 and Kc4 (P9, P10).
- **VTT-459 (R10).** "A click on "No shoulder" in a player's perch sends `set_viewpoint`
  naming nobody."
  - Evidence: C1.
  - Red under Kc5.
- **VTT-460 (R11).** "With doors not armed, a board click in a browser window
  whose participant's role the client last read as `player`, where the last
  `WatchableChanged` the client was sent does not say they control no party
  member, on a cell holding a token of a party member that set names other than
  the one they watch, sends `set_viewpoint` naming one of those party members."
  - Evidence: VTT-406's current tests, re-pointed.
  - Red under the existing VTT-406 breaks.
- **VTT-461 (R12).** "A board click in a browser window whose participant's
  role the client last read as `player`, where the last `WatchableChanged` the
  client was sent says they control no party member, sends nothing."
  - Evidence: C3.
  - Red under Kc3 and Kc3b (P10).
- **The sort refused:**
  - "`watchable` names every party member for a player who controls none":
    VTT-399 and VTT-400 already say "the party members `MayWatch` accepts", and
    R3 makes that every party member. N2, N5 and the edited watchable tests
    join their evidence.
  - Item 5 (granted, moves to the first by id): VTT-340 as worded. N5 joins its
    evidence.
  - Item 2 (perch and reload): VTT-339, VTT-342, VTT-399 and VTT-442. N3 joins
    the first three.
  - Item 3's one refusal string: VTT-347, unchanged, with
    `TestQACreatureSetViewpointNeedsAPartyMemberTheyControl` as edited.
  - "A player who controls none moves no token": VTT-370, unchanged. N4 joins
    its evidence.
  - A perched player has no Events panel: VTT-445, whose rule is the role.
  - Item 12's wording: the rule it states is R1-R3's. `TestToolsMatchGolden`
    and `check:drift` hold the text, and a grep shows the old claims gone.
  - VTT-405 is not withdrawn. Its party-member half covers any switch button,
    and R10 covers the player's "No shoulder".
- **Existing rows** are not reworded. VTT-036's and VTT-340's evidence cells
  change where Task 1 renames a test, and VTT-388's gains the stay test.

| Ticket item | Observed by |
|---|---|
| 1 | N1, N2 |
| 2 | N3 |
| 3 | `TestAPlayerMayWatchOnlyThePartyMembersTheyControl`, `TestAPlayerMayNotWatchNobody`, `TestAPerchRefusalDoesNotSayWhetherTheActorExists`, green |
| 4 | N4; `TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken` and `TestQAMGLosingTheOnlyPartyMemberLeavesNoTokenMovable` as edited |
| 5 | N5 |
| 6 | N6 and the edited `TestQAWatchableARevocationChangesOnlyTheRevokedPlayersSet` (two party members); N8 (one) |
| 7 | N7, N8, the contract round trip |
| 8 | the 23 edits, all green |
| 9 | C1, C3 |
| 10 | "a player is offered exactly the party members the set names", "a player switches by a button and by a click on the token", green; C2 |
| 11 | W1 and beat 2 of "a DM hands a second character over, and the player keeps both across a reconnect", in `task e2e` |
| 12 | `TestToolsMatchGolden`, `check:drift`, and `grep -n 'may watch only a party member they control\|only a spectator may ask' tools/toolgen/main.go contract/vtt/v1/commands.proto` printing nothing |

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off on this plan.
  - Allocate R1-R12 with `requirement-id`: VTT-450 to VTT-461.
  - Mark VTT-343, 346, 351, 402 and 406 withdrawn as the Requirements section
    says.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean, the twelve rows exist,
  `python3 tools/check-requirements-chain.py` passes, and the five rows read
  `WITHDRAWN 2026-10-23`.

### Task 1: whom a player may watch, and the move gate (R1-R4, R7; items 1-6 and 8)

- **Files:**
  - `internal/gateway/viewpoint.go` (D1, D2; `watchable` unchanged in this
    task);
  - `internal/gateway/seat.go` (`judgedBy`, D3, and its doc and `told`'s
    comment re-pointed in place);
  - the tests below;
  - `docs/requirements.md` (evidence, renames).
- **New tests**, each after a closing brace, each with its rows' ids:
  - **N1**, `viewpoint_test.go`,
    `TestAPlayerWhoControlsNoPartyMemberMayWatchAnyPartyMemberOrNobody`.
    - Two players: one who controls nothing, and one who controls only a
      `NON_PARTY` actor (K9).
    - Both are accepted for every party member (another player's and an
      uncontrolled one) and for the empty id.
    - Both are refused for a non-party actor and an absent id, with the very
      string a spectator gets for the same id.
    - Red today (P3).
  - **N2**, a package-`gateway` test beside the other `watchable` users in
    `seat_internal_test.go`,
    `TestWatchableNamesEveryPartyMemberForAPlayerWhoControlsNone`: the set's
    ids and names are every party member's, for both of N1's players. Red
    today.
  - **N3**, `watchable_test.go`,
    `TestAPlayerWhoControlsNoPartyMemberPerchesAndReopensOnIt`.
    - A player who controls none is sent every party member at the opening,
      perches on one, is answered ok, and is sent its `ViewpointChanged` and
      its log from the first entry.
    - A second dial naming it opens on it.
    - Red today at the refusal (P3).
  - **N4**, `qa_move_gate_test.go`, `TestQAMGAPerchedPlayerMovesNoToken`.
    - A player who controls only a non-party pet perches on another player's
      `pc-c` with `set_viewpoint`.
    - Every pet move is refused as unseen, the crate square included (its
      refusal is the unseen string, not the obstruction).
    - Red today at the perch. Under K4 it is red on the moves.
  - **N5**, `watchable_test.go`, `TestAPerchedPlayerGrantedAPartyMemberMovesToTheFirst`.
    - A player perched on an uncontrolled party member is granted two of their
      own.
    - The set naming only theirs comes before a `ViewpointChanged` to the lower
      id, then its log from the first entry.
    - Red today at the perch.
  - **N6**, `watch_test.go`, `TestAPlayerWhoseLastPartyMemberIsRevokedKeepsWatchingIt`.
    - The player holds `a1`, and `a2` is an uncontrolled party member.
    - After the revocation, the set naming `a1` and `a2` arrives, then the
      revocation's own entry of `a1`'s log (E7), then a later narration's.
    - No `ViewpointChanged` follows.
    - Red today (P4).
  - **The stay**, `qa_promotion_rejudge_test.go`,
    `TestQARejudgePromotedSpectatorWhoControlsNoneKeepsItsPerch`: today's
    fixture of the renamed rejudge test, with no switch, quiet, and the next
    entry still of `a1`. Red today. It joins VTT-388's evidence as well as
    R3's.
- **The 23 red tests (P2), by why they go red:**
  - **(a) A player who controls none is now told every party member.** The
    expected set changes, and nothing else does. Six tests:
    - `TestAConnectionIsToldWhomItMayWatchBeforeItsOpeningViewpoint`: `other`
      opens on nobody with `a1:Hero p2:Quill`.
    - `TestTheSetIsSentAgainWhenItChanges`: after `a2` leaves the party the set
      is `a1:Hero`; after `a3`'s removal, `a1:Hero` again.
    - `TestQAWatchableAChangeThatLeavesTheSetAloneSendsNone`: Ivo's opening set
      is every party member.
    - `TestQAWatchableAConnectionWithNoneIsSentAnEmptySet`: the table is
      emptied of party members, as its spectator half already does, so "even
      when there are none" holds for Ivo.
    - `TestQAWatchableAGrantChangesTheSetsItReaches`: Ivo's set changes with
      the party, then he moves on his own grant.
    - `TestQAWatchableANonPartyGrantTakesTheActorOutOfEverySet`: Ivo's set
      loses each actor too, as the name says.
  - **(b) A promotion to a player who controls none now leaves the seat where
    it was.** Each test gives the promoted participant a party member of their
    own and expects the move to it, with no entry of the shoulder after the
    promotion. Eleven tests:
    - `TestAPromotionTellsTheSeatTheNewRolesSet`;
    - `TestAPromotionMovesASpectatorOffAShoulderThePlayerMayNotWatch`;
    - `TestAPromotionMovesEveryConnectionOfTheParticipant`;
    - `TestAPromotionAsTheSeatOpensSendsNoEntryOfTheShoulder`;
    - `TestAnEntryTakenAfterAPromotionIsJudgedByTheNewRole` and
      `TestARoleReadAfterAdmitStillJudgesTheEntry`, through their helpers
      `seatOpenedOnX1` and `seatNoEntryOfX1`, which only these two use;
    - `TestQARejudgeBothConnectionsOfAPromotedSpectatorAreRejudged`;
    - renamed, as its name becomes false:
      - `TestQARejudgePromotedSpectatorOnAnothersPartyMemberWatchesNobody`
        becomes `…MovesToTheirOwn`;
      - `TestQAQueuedPromotedSpectatorOnAnothersPartyMemberMovesToNobody`
        becomes `…MovesToTheirOwn`;
      - `TestQAOpeningAPromotedSpectatorOnAnothersPartyMemberEndsWatchingNobody`
        becomes `…EndsWatchingTheirOwn`.

      VTT-036's and VTT-340's cells change with the three renames.
    - `TestQAWatchableAPromotionChangesTheSet`'s case "a perched spectator made
      a player who holds nothing" is the exception. It turns to expect no set
      and no switch, and is renamed "… keeps its set and its perch".
  - **(c) A revocation of the player's only party member now leaves the seat on
    it.** Four tests:
    - `TestNoEntryCausedByTheLossIsSent` (VTT-348): the player holds a second
      party member, the seat moves to it, and the loss's entry is dropped.
    - `TestAConnectionStopsWatchingAnActorItMayNoLongerWatch`, case "control
      revoked" (VTT-348): the same second party member, and a move to it. The
      lost-entry check is unchanged. The other two cases keep nobody.
    - `TestQAWatchableARevocationChangesOnlyTheRevokedPlayersSet`: the last
      revocation expects the set `a1:Hero a2:Ash` and no switch (item 6, two
      party members).
    - `TestQAMGLosingTheOnlyPartyMemberLeavesNoTokenMovable`: after the
      revocation, a narration's entry of `pc-b`'s log arrives with no switch
      before it (P12), and every pet move is still refused.
  - **(d) A player who controls none is now accepted any party member.** Two
    tests:
    - `TestQACreatureSetViewpointNeedsAPartyMemberTheyControl`: the second
      player is granted a party member of their own first, so "not theirs" is
      still refused, with one string (R1, VTT-347).
    - `TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken`: `log=pc-c`
      opens on `pc-c`, and every refusal stays (P12).
- **Order:**
  1. Write N1-N6 and the stay test, and run them. Each is red where it says.
  2. Make D1-D3.
  3. Run `go test ./internal/gateway/`: the 23 of P2 go red, and N1-N6 and the
     stay test go green.
  4. Edit the 23 as above, re-point every VTT-343, 346 and 351 citation under
     `internal/`, and update the register's cells.
  5. Run the whole package, then the edited race tests with `-count=3 -cpu 1,2`.
  6. Run `check-comments.py main` after staging.
- **Between this commit and Task 3's,** the embedded bundle still draws every
  player the player panel. So a player who controls none sees a chip for every
  party member the server now names, and a chip's click is accepted. The branch
  is not merged before Task 4, and `task e2e` is first run in Task 3.
- **True when:**
  - `go test ./...` is green;
  - `grep -rn -E 'VTT-(343|346|351)([^0-9]|$)' internal` prints nothing;
  - `check-requirements-chain.py` passes;
  - the adjudicated sort line is byte-identical to main's.
- **Breaks,** each applied, run, and undone by its inverse edit:
  - **K1:** `MayWatch` accepts any party member for every player. Seven of
    the existing viewpoint, seat and move-gate tests it was run against go red
    (measured), among them `TestAPlayerMayWatchOnlyThePartyMembersTheyControl`
    and `TestAPerchRefusalDoesNotSayWhetherTheActorExists`.
  - **K2:** `MayWatch` keeps today's player rule. N1, N3, N4, N5 and the stay
    test go red.
  - **K3:** the empty id is accepted for every player. Three of the same tests
    go red (measured), among them `TestAPlayerMayNotWatchNobody`.
  - **K4:** `judgedBy` left on `MayWatch`. N4 and both edited move-gate tests
    go red (measured for the two).
  - **K5:** `judgedBy` answers the told viewpoint whenever it is a party
    member. The same tests go red (expected).
  - **K8:** `resolve` moves a player who lost their last party member to the
    default. N6 and the edited revocation tests go red (expected).
  - **K9 on `MayWatch`:** D1 counts a non-party actor. N1's second player and
    the edited `log=pc-c` case go red (expected).
- **Fixture check:** N1's second player is the one that catches K9. Run N1 with
  that player removed under K9 once, and see it pass. Undo it.

### Task 2: the contract, the field, the tool (R5, R6; items 6, 7 and 12)

- **Files:**
  - `contract/vtt/v1/commands.proto` (the field and `SetViewpoint`'s comment,
    D5);
  - `internal/gateway/viewpoint.go` (`watchable` sets the field, D4);
  - `tools/toolgen/main.go` (`set_viewpoint`'s description);
  - what `task generate:contract` writes: `contract/gen/`,
    `contract/gen/tools/tools.json`, `cmd/vtt/tools.json`;
  - `contract/testdata/expected_tools.json`, by hand;
  - a new `contract/testdata/server_frame_watchable_changed_controls_none.json`;
  - `contract/roundtrip_test.go`, `contract/events.test.ts`;
  - the tests below;
  - `cmd/vtt/webdist/` from `task build:client` (D6);
  - `docs/requirements.md`.
- **`set_viewpoint`'s description:** "Choose the character log this connection
  is sent: a spectator, and a player who controls no party member, may watch any
  PARTY MEMBER, or nobody with an empty actor_id; a player who controls a party
  member may watch only the party members they control." The rest of today's
  text stays.
- **New tests:**
  - **N7**, `watchable_test.go`,
    `TestTheSetSaysWhetherTheParticipantControlsNoPartyMember`. The flag is:
    - false for a player who controls a party member;
    - true for one who controls nothing;
    - true for one who controls only a non-party actor (K9);
    - true for a spectator who controls nothing.
    
    Red today: the field does not exist.
  - **N6 gains the flag:** the set it reads after the revocation also says the
    player controls no party member (item 6 at a table of two party members).
  - **N8**, `watchable_test.go`,
    `TestARevocationOfATablesOnlyPartyMemberSendsTheSetAgain`.
    - `a1`, the player's, is the only party member.
    - After the revocation, a set with the same actors and the flag true
      arrives before the revocation's entry, and no switch.
    - Red today (P4). Under K7 it is red at the set (P5c).
  - **The round trip:** the new fixture
    `{"watchableChanged":{"actors":[{"actorId":"act-hero","name":"Hero"}],"controlsNoPartyMember":true}}`,
    with `TestServerFrameWatchableChangedControlsNoneRoundTrip` in Go and a row
    in `events.test.ts`'s cases. Both are red today.
- **Edits:**
  - `TestQAWatchableAConnectionWithNoneIsSentAnEmptySet`'s raw check expects
    the arm with no `actors` key and `"controlsNoPartyMember":true` (E2).
  - `TestAnEmptyWatchableSetStillSerializesAsItsFrame`'s comment drops "a
    connection that may watch nobody" and says what it holds: an empty message
    still identifies itself as the frame.
- **Order:**
  1. Write N7, N8 and the round trip against the regenerated types: red.
  2. Add the field.
  3. Run `task generate:contract`, then hand-edit `expected_tools.json` until
     `TestToolsMatchGolden` is green.
  4. Make D4.
  5. Run `task build:client`.
  6. Run every Go test and `env -u CLAUDECODE bun test client/test contract`:
     all 1381 existing client tests stay green, as P6 measured with this field.
  7. Run `check-comments.py main` after staging.
- **True when:**
  - all of the above are green;
  - the item-12 grep prints nothing;
  - `git status` lists the files above, `cmd/vtt/webdist/assets/index.js`
    among them.
- **Breaks:**
  - **K6:** the flag false for every seat. N7 goes red.
  - **K7:** `tell` compares actors alone. N8 goes red (measured, P5c).
  - **K9 on the flag:** D1 counts a non-party actor. N7's third case goes red;
    no existing test catches it (measured).

### Task 3: the browser draws a player who controls none the perch (R8-R12; items 9-11)

- **Files:**
  - `client/src/wire.ts`, `client/src/session.ts`, `client/src/app.ts` (D7;
    `mayPerch`'s doc reworded in place);
  - `client/src/view/player.ts` (re-pointing only) and
    `client/src/view/spectator.ts` (two comments, in place), D7;
  - `client/src/style.css` (the perch template's comment);
  - `client/test/app.test.ts`, its "only a spectator is offered a shoulder"
    renamed "a player who controls a party member, the DM and the agent are
    offered no shoulder", with its comment;
  - `client/test/spectator-view.test.ts` (one comment);
  - `client/test/player.test.ts` and `client/test/qa-switch.test.ts`
    (re-pointing);
  - `client/e2e/handover.spec.ts`;
  - `cmd/vtt/webdist/`;
  - `docs/requirements.md`.
- **New unit tests in `app.test.ts`**, after the "a player switches between the
  party members they may watch" group:
  - **C1:** "a player whose set says they control no party member is offered
    the perch, and no player panel".
    - The set names Lera and Bram with the flag. The opening is nobody.
    - Assert: `.perch` drawn, headed "Whose eyes", with buttons Bram and Lera
      and "No shoulder", and `.player` count 0.
    - A click on Lera sends `set_viewpoint` naming `a1`, and on "No shoulder"
      naming nobody.
    - After `ViewpointChanged(a1)` and Lera's log, it reads "Perched on: Lera".
    - Red on the tree after Task 2 (P9).
  - **C2:** "the set's flag moves a player between the perch and the player
    panel": panel, then perch on a set with the flag, then panel again on a set
    without it. Red after Task 2 (P9).
  - **C3:** "a perched player's board click sends nothing, on a listed party
    member's token or on an empty square".
    - The fold holds a listed party member's token and a token of a non-party
      actor the player controls.
    - Without both, Kc3 survives (measured).
    - Two clicks, and nothing is sent.
- **The e2e, in `handover.spec.ts`:**
  - **Beat 2** of "a DM hands a second character over, and the player keeps
    both across a reconnect": once `.perch` is visible, it reads "Perched on:
    nobody", offers "Ash" and "Bram", and `.player` has count 0. It keeps the
    `h2` screenshot.
    - Beat 3's first grant then draws the panel with Ash's chip, the perch's
      way back.
  - **W1**, a second test, "the DM takes a player's only character back, and
    the player chooses another shoulder":
    - A DM context and a player context. The player opens watching Ash.
    - The DM clicks Revoke on Bram's row, then on Ash's (G1).
    - Once `.perch` is visible, it reads "Perched on: Ash", and `.player` has
      count 0.
    - A click on "Bram" reads "Perched on: Bram", shows `tok-act-hand-b`, and
      a non-empty `.feed .mechanical`.
    - After a reload, it still reads "Perched on: Bram" (VTT-442 for such a
      player).
    - Screenshots `h11-player-perched-after-the-revoke` and
      `h12-player-on-bram` (`h9` and `h10` are the skipped test's).
    - It carries R9's id.
- **Order:**
  1. Write the e2e changes. Run `task e2e` on Task 2's bundle: beat 2 and W1
     go red at the perch, and everything else stays green. This is item 11's
     "fails on today's bundle".
  2. Write C1-C3: C1 and C2 go red.
  3. Make D7. Re-point VTT-402 and 406 under `client/`.
  4. Run `task build:client`, every client test, `task client:typecheck`,
     `task e2e`, and `check-comments.py main` after staging.
- **True when:**
  - all of the above are green, item 10's two tests among them;
  - the full re-pointing grep prints nothing;
  - `git status` lists the files above and `cmd/vtt/webdist/assets/index.js`.
- **Breaks:**
  - **Kc1:** `mayPerch` ignores the flag. C1 and C2 go red (measured, P9).
  - **Kc2:** the panel is also drawn for a perched player. C1 and C2 go red at
    `.player` (measured).
  - **Kc3:** `onCell` stays wired for a perched player. C3 goes red (measured).
  - **Kc3b:** Kc3 plus a perched player routed through `playerClickFor`. C3
    goes red (measured).
  - **Kc4:** a discard clears the flag. C1 and C3 go red (measured).
  - **Kc5:** the perch's `onPerch` drops `""` for a player. C1 goes red at
    "No shoulder".
- **Opposite direction:** item 10's two tests and C2's panel half stay green
  under Kc1-Kc5.

### Task 4: the gate, whole

- **Do:** run `task check` whole on Task 3's commit, the last that changes
  code, then `task e2e` on the same commit.
- **True when:** both are green, including:
  - `check:drift`;
  - `check:breaking`, which reports nothing breaking;
  - both mutation gates, with no new adjudication on code this plan wrote;
  - `check:requirements-chain`, which resolves R1-R12.

### Task 5: the report

- **Do:** write the implementation report under `docs/reports/`, with its own
  reading review and its own commit. It covers:
  - each "Done looks like" item with its observation (the table above);
  - K1-K9 and Kc1-Kc5, with what went red;
  - Q1's answer;
  - the Revoke button's first browser run (G1);
  - Phase 4a's adjudications under their own heading.
- **True when:** the report answers every one of the ticket's twelve items in
  the ticket's numbering, each `[x]` naming its observation; it has had its
  reading review; and it is committed alone, after Task 4's gate.

### Task 6: the specifications

- **Do,** with the `specification` skill, last and from the report:
  - **SPEC-015:**
    - who may watch whom (R1-R3);
    - the default;
    - `watchable` and its field (R5);
    - the stay (R7, "Stay on Tobbe");
    - the resend on the flag alone (R6);
    - `judgedBy` (R4);
    - the consequence "A player who controls no party member moves nothing,
      and watches nobody from the pump's next re-judging of the seat" is
      rewritten: they move nothing (VTT-370); they may watch any party member
      or nobody, as a spectator may (R3); the event that leaves them
      controlling no party member does not move them off a party member they
      watch that is still one (R7); a grant of a party member moves them to
      their first by id (VTT-340);
    - the sentences that cite the withdrawn ids in prose: "never the empty id
      (VTT-343, VTT-346)" and the `judgedBy` sentence's "(VTT-351)";
    - the Requirements list, with successors in place of the withdrawn rows.
  - **SPEC-013:** its consequence "a player's `set_viewpoint` chooses which of
    their party members they watch" stays for a player who controls one, and
    gains that a player who controls none chooses as a spectator does.
  - **SPEC-022:**
    - a player whose last set says they control none is offered the perch and
      no panel (R9, R10), and their board click sends nothing (R12);
    - "Only a spectator is offered the perch" goes;
    - "You do not control an actor yet." is shown to a player only before
      their first set;
    - the sentences that cite the withdrawn ids in prose: "(VTT-402 to
      VTT-406, VTT-408)" in Status, the chips sentence's "(VTT-402)" and the
      token-click sentence's "(VTT-406)";
    - the Requirements list.
  - **SPEC-007:** the field, and the empty set's wire form (E2).
  - **SPEC-021:** its consequence on a non-party creature names the move rule
    of SPEC-015 rather than restating it. A player who controls none moves it
    nowhere.
  - Cite the owner's rulings B-4, "Stay on Tobbe" and "Field on the list".
- **True when:**
  - `python3 tools/check-requirements-chain.py` passes on its commit, run by
    hand.
  - Each new sentence has been run against the code by someone other than its
    writer, Phase 4b's reading included.

## For Phase 4a

Hand the QA agent:

- R1-R12, VTT-339, VTT-340, VTT-347, VTT-348, VTT-370, VTT-387, VTT-388,
  VTT-399 and VTT-400;
- `go doc ./internal/gateway`, with `MayWatch`'s signature;
- `WatchableChanged`'s fields, and that the browser's role comes from
  `/api/me`.

Tell it:

- **The field.**
  - An empty set now always carries `controlsNoPartyMember: true` (E2).
  - The set is re-sent when the flag alone changes.
  - Today's bundle refuses the key (E3).
- **Character logs.** A character log's own entries (opening, resume,
  projection) carry no `EventId`, and entries forwarded from the world share
  its ids, so compare entries with `proto.Equal`.
- **Imports.** `go-arch-lint` holds the layer imports: a test in
  `internal/gateway` may not reach every package.
- **Pump-order tests.** Run them at `-cpu 1,2`.
- **The browser.** bun runs every test file in one process, so a boot test
  clears both storages in `beforeEach`; a failing `toBeNull()` on an element
  takes 20-30 s to print.
- **The Revoke button** is new to the e2e suite (G1).

## Forward coupling

- **B-2 (whispering)** decides whether a player who controls no party member
  may speak. Today they have no narration box, as before (E4).
- **B-5, K-1, A-6 (what a token shows).** A perched player is sent the log of
  the party member they chose, so whatever that ticket withholds from a
  character's log, it withholds from them too.
- **A player who controls only a non-party creature** still passes `Authorize`
  for its `use_ability` and `open_door`, which judge no sight, through
  `vtt mcp`. Unchanged here, and the browser offers neither to a perched
  player.
- **"You do not control an actor yet."** remains for the DM and the agent and,
  for a player, only until their first set arrives.
