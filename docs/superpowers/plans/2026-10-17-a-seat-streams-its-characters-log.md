# A player's or a spectator's connection streams one character's log: implementation plan

- **Ticket:**
  `docs/superpowers/specs/2026-10-17-a-seat-streams-its-characters-log-design.md`,
  untracked at the time of verification.
- **Verdict:** PASSES WITH GAPS (G1-G10 below). An agent that did not write the
  ticket verified it, read-only. The probes ran in the working tree, wrote
  only to the scratchpad, and are not committed.
- **Signed off:** 2026-10-17 by the owner ("Ja, kör"), with the answers
  recorded under "Questions for the owner". Answers 2 and 3 amended the
  ticket after its verification; the places are named under those answers.
  The reading review of this plan's first commit amended it again, as listed
  under "Before Task 1".
- **Base:** main `fa6a067`.
- **Branch:** `feat/a-seat-reads-its-character-log`.
- **Lands at:** `docs/superpowers/plans/2026-10-17-a-seat-streams-its-characters-log.md`.

The agent that verified the ticket wrote this plan. Before writing it, that
agent read:

- in `internal/gateway`:
  - `server.go` whole;
  - `seat.go`, `project.go`, `viewpoint.go`, `authz.go` and `export_test.go`;
  - the test lists of every projection test file, and the helpers they share;
  - `server_test.go`'s fixture, frame reader, `TestMoveTokenBroadcastBackfillsSceneAndFrom`,
    `TestPlayerOwnershipDenialNoBroadcast` and `TestARevokedSpectatorStopsSeeingTheTable`;
  - `server_visibility_test.go`'s `TestEverySeatCanReachTheCatchUpHeadItIsGiven`;
  - `scenario_test.go`'s seat handling;
- `internal/campaign/campaign.go` whole, `foldprefix.go`, and the
  character-log test lists;
- `internal/harness`:
  - `engine.go` from `drainPreExisting` to `observeOnAll`;
  - `soak.go` from `SoakParticipants` to `settle`, `client.go`'s `Dial` and
    `readLoop`, and the `Conn` interface;
  - `late_leak_test.go`'s test list and assertions;
- every scenario's participants and steps, `client/src/wire.ts`'s frame
  dispatch, `tools/toolgen/main.go`'s `set_viewpoint` entry, and
  `cmd/vtt/client_e2e_test.go`'s `testStateDumpForAProjectedSeat`;
- SPEC-007, SPEC-009 (re-resolution), SPEC-011, SPEC-013, SPEC-015, SPEC-016
  and SPEC-021, and grep hits in SPEC-012, SPEC-017, SPEC-018 and SPEC-019;
- the per-character-logs arc design §3-§10, the 3a plan, and the owner's
  rulings in the project memory;
- `tools/check-comments.py`'s rules and the ceilings of every file touched;
  `check-requirements-chain.py`'s rules; `check:drift`; `check:ts-coverage`;
  `.go-arch-lint.yml`;
- MapTool at `~/dev/RPTool/maptool` `f4b7fef6c` (Rule 9, below).

## Constraints that bind every task

These are named here, not restated.

- **CLAUDE.md rules:**
  - 2: `task check` is never weakened. No floor is lowered and no ceiling
    raised. D10 narrows a harness check, and that narrowing is its own task
    and its own commit, with its reason.
  - 3: additive contract. One new message and one new `ServerFrame` arm.
    `check:breaking` reports and exits 0 before release.
  - 4: one fold. Nothing in this plan applies an event. The gateway's
    `FoldPrefix` calls go, and nothing replaces them.
  - 5: no game-system vocabulary. Fixtures reuse the existing hall, hero,
    goblin, cave and ambush.
  - 8: citations name durable things.
  - 9: RPTool first (answered below).
  - 10: a comment is a warning or a pointer. History goes to the report.
- **The `dev-cycle` package's phases.** They live in the package and are not
  restated here. One Phase 4b review and one commit per task.
- **The owner's rulings**, which this plan does not re-open:
  - one active character per connection, switched by clicking its token;
  - no viewpoint, no events;
  - a late log opens with the present;
  - a switch is announced by a new frame, then the new log from 1, and the
    client resets on it (ticket 4);
  - the move gate uses the active character's eyes;
  - the projection is deleted here with its goldens, with no dual path;
  - the default is the first own party member by id;
  - the DM and the agent keep the world, and `set_viewpoint` stays refused
    for them;
  - 3a's D6 hold;
  - ticket 4 comes after this one, so the client is unchanged except for
    deleting `client/test/projection-parity.test.ts`.
  - Client comment pointers to `project.go` and the projection goldens
    are ticket 4's
    (`docs/superpowers/plans/2026-10-15-the-perception-decision-leaves-the-gateway.md`:
    "The client and contract comment pointers wait for ticket 4").
- **SPEC-008:** ids come from `requirement-id`, after sign-off.
- **SPEC-010:**
  - A block of more than six comment lines may gain no line, and a block an
    edit touches must end at six lines or fewer. Blocks over six lines that
    this work touches:
    - `leakedSince`'s doc, 10 lines;
    - `leakedBelow`'s doc, 10 lines;
    - `FoldPrefix`'s doc, 39 lines;
    - `.go-arch-lint.yml`'s eventgen note, which `check:comments` does not
      scan;
    - the pointer blocks in `internal/engine/state.go` and `apply.go`,
      `internal/mapdef/compile.go`, `internal/eventgen/model.go` and
      `cmd/vtt/library_test.go`.
  - Ceilings at or near the line:
    - `seat.go` 29.6, measured 29.6;
    - `server.go` 25.3, measured 25.2;
    - `campaign.go` 29.8, measured 29.8;
    - `authz.go` 13.3, measured 13.3;
    - `viewpoint.go` 22.3;
    - `engine.go` 38.1;
    - `soak.go` 45.3.
  - A rewritten file adds comment lines, so its share must stay at or under
    its ceiling. A new file must be at 25.0 or under.
  - Run `tools/check-comments.py` against `main` and against `HEAD` before
    every commit.
  - A committed row is never recounted, and `--write-ledger` drops the rows
    of deleted files.
- **SPEC-020:** an adjudication names the code it excuses. D2 and D7 are
  shaped so that no equivalent mutant is needed.
- **SPEC-015's grep for `c.log.Append`** stays true. No append path is added.
- **Arch:** the dependency lists apply to test files too.

  | Package | May depend on |
  |---|---|
  | `gateway` | `contract`, `campaign`, `engine`, `identity`, `gateway`, `rules`, `adventure`, `mapdef`, `sight`, `artlib`, `eventgen`, `perceive` |
  | `harness` | `contract`, `engine` and `harness` only |
  | `campaign` | `contract`, `store`, `engine`, `campaign`, `eventgen` and `perceive` |

  After Task 4 the gateway imports neither `eventgen` nor `sight`. The list
  is left as it is, since tightening it is not this ticket's.
- **Operational lessons in the project memory, binding here:**
  - Review packages are built with `git diff HEAD`. Never stash while a
    reviewer reads.
  - Settle the review before the gate. Stage and commit in separate calls,
    then verify what landed.
  - Read the load with `os.getloadavg()`. Launch `task check` with
    `start_new_session=True`.
  - Rebuild `cmd/vtt/webdist` after any change to the contract or the client
    (`check:drift`). The commit hook does not catch a stale bundle, and a
    drift check passes only after the commit.
  - Measure `bun` with `env -u CLAUDECODE`.
  - Run `check:new-prose` by hand before the last gate run.
  - Give the pre-push hook three minutes.
  - Hash the tree before and after every QA or review agent.
  - Run the mutation gate's two self-tests before the gate.

## Rule 9: how MapTool solves this

Checked against `~/dev/RPTool/maptool` at `f4b7fef6c`.

**How a player's view is chosen:
`net.rptools.maptool.client.ui.zone.ZoneViewModel.makePlayerView(role, selected)`.**

- It takes the selected tokens and drops each one the player does not own or
  that has no sight (`AppUtil.playerOwns`, `getHasSight`).
- With none left, it falls back in turn to:
  - every token the player owns that has sight, when individual views are on
    or the server is personal (`zone.getOwnedTokensWithSight`);
  - otherwise every player token with sight, the whole party's union
    (`zone.getPlayerTokensWithSight`);
  - and finally a role-only `PlayerView(role)` with no tokens.
- `updatePlayerView` recomputes it on every `update`, on the client.

**How the client is told the selection changed.**

- `SelectionModel` posts `SelectionChanged(zone)` on the client's own
  `MapToolEventBus`.
- `ZoneRenderer.onSelectionChanged` clears the shown paths and repaints, and
  the next update rebuilds the view.
- Nothing crosses the network. The server never learns which token a client
  has selected, and every client already holds the whole campaign.

**Moves.** `ServerMessageHandler.handleMessage` relays `PUT_TOKEN_MSG` and the
`START`/`UPDATE`/`STOP_TOKEN_MOVE` messages with `sendToClients` and judges no
move by sight. MapTool has no server-side move gate.

**Borrowed:** the view follows the token the player selected, one character
at a time. Ticket 4 makes the click.

**Refused:**

- **Both union fallbacks**, owned-with-sight and all-player-tokens. The owner
  struck the union. Our default when nothing is chosen is the first own party
  member by id (2026-10-17).
- **The role-only fallback.** Ours is stricter: a seat that watches nobody is
  sent no event.
- **Selection as client-only state.** Here the server must know the
  viewpoint, because the stream is the boundary (§10 item 1 of the arc).
  So a switch is a command (`set_viewpoint`) and an answer
  (`ViewpointChanged`), which have no MapTool counterpart.
- **No move gate.** SPEC-013's gate stays and moves to the watched character
  (Task 5). MapTool has no precedent here, checked and found empty.
- **The distribution model**, as in every ticket of this arc.
- **"Show As Player" (`LocalPlayer.getEffectiveRole`)**, by the ruling of
  2026-10-17.

## Decisions this plan makes

Each decision names what forced it.

**D1. A seat that is sent no entry is woken by a payload-free signal from the
campaign. `Campaign.WatchAppends() (wake <-chan struct{}, stop func(), err
error)` returns a channel of capacity one.**

- `appendLocked` raises it with a non-blocking send to every watcher:
  - after the live apply;
  - before the world's `Notify` loop and before the `NotifyCharacter` loop.
- A refused, failed or poisoning append raises none, since all three return
  before that point.
- `Close` closes every watcher's channel and empties the registry.
  `WatchAppends` refuses while the campaign is poisoned (`errPoisoned`) and
  after `Close`.
- The register calls one registration an "append watch" (VTT-353 to
  VTT-357), so that "watcher" is not read as a person watching a log.
- Only character seats register. The DM's and the agent's seats keep the
  world's subscription and no wake.
- `internal/campaign` changes. The 3a store is untouched.

*Forced by:*

- **The ticket's two cases.** A player granted a first party member and a
  watched actor removed both need a wake that no entry gives. The third
  candidate (re-check only on an entry or a command) misses both.
- **Arc §10 item 1.** A player's connection has no access to the world log
  at all.
  - A world subscription per seat is that access, with its payload
    discarded.
  - One per server is the same reader with a longer life. `Server` has no
    `Close` to end it, and it needs a no-progress budget of its own.
- **D3's ordering proof.**
  - The signal is raised before the append's entries are notified, so an
    entry the append caused can never reach the pump ahead of the wake.
  - An envelope on a world subscription races the character subscription on
    a different channel, and no select can order the two.
- Cost: one non-blocking channel send per watcher per append, under `c.mu`.

**D2. What "may watch" is checked against: `Campaign.Actors()`, a clone of the
actor map taken under `c.mu`, nil while poisoned.**

- One rule function, `MayWatch(p, actorID, actors)` in `viewpoint.go`,
  replaces `MayPerch`. `defaultViewpoint(p, actors)` sits beside it.
  `Authorize` passes `st.Actors`, and the seat passes `Campaign.Actors()`.
- `defaultViewpoint` collects the ids of party members the player
  `controls`, applies `sort.Strings`, and takes the first. It uses no `<`
  comparison of its own.

*Forced by:*

- The rule reads two things only, an actor's kind
  (`engine.IsPartyMember`) and its `controller_ids`.
- `State()` deep-copies every scene's tiles (`engine.State.Snapshot`). Taken
  per seat per append under `c.mu`, it competes with appends.
- One function per rule keeps SPEC-013's single refusal path.
- A hand-written minimum over unique ids has an equivalent `<`→`<=` mutant,
  which `sort.Strings` avoids (SPEC-020).
- *The cost is not measured.* Task 2's report records `Actors()` against
  `State()` on the soak's campaign at 500 events.

**D3. When "may watch" is checked:**

- at the open;
- on every wake;
- before each entry and before each `ViewpointChanged`, whenever a wake is
  waiting. The pump does a non-blocking receive on the wake first.

**The credential is re-resolved (`credentialGone`) on every wake and before
every entry.** The rule "No entry ... is sent after the connection may no
longer watch" is read causally (G5). No entry caused by the event that took
the right away, or by any later event, is sent. Entries caused earlier that
are still queued when the loss is seen are dropped too, which fails closed.

*Forced by:*

- D1's order. The wake for event E is raised before E's entries are
  notified, so the pump sees the wake before any of them. Its check reads
  the actors after E.
- Reading `Actors()` per entry would cost a clone per entry per seat. The
  priority receive costs nothing when no wake waits.
- VTT-032 for a seat that is sent no entry (D12).

**D4. The wire and the frame order.**

- **Naming the log.** A query parameter `log=<actor_id>` goes beside
  `after`.
- **What `log` chooses.** A connection whose `log` names an actor its
  participant may watch (`MayWatch`) opens on that log, and its cursor is
  honoured there. Otherwise it opens on the default, and the subscription
  starts at 0, so a reconnect that names no log, or one it may no longer
  watch, is sent the opening log from its first entry. The DM and the agent
  ignore `log`. A spectator's `log` naming a party member opens perched on
  it.
- **The order at connect:**
  1. `CatchUpHead`: the opening subscription's head as the store gives it
     (VTT-331), or 0 for a seat that watches nobody;
  2. `PresenceSnapshot`;
  3. from the pump, `ViewpointChanged{actor_id}`, empty for nobody;
  4. the entries.
- **The order on a switch:** `ViewpointChanged{new}`, then the new log from
  its first entry. There is no second `CatchUpHead`.
- **`ViewpointChanged` carries `actor_id` only**, `ServerFrame` field 6.
  protojson writes `{"viewpointChanged":{}}` for nobody, so the arm's
  presence is the switch and an empty id is nobody.

*Forced by:*

- SPEC-007's positional promise: `CatchUpHead` comes first.
- SPEC-011: the pump is the only producer of envelopes, and
  `ViewpointChanged` must come before them, so the pump sends it.
- A query parameter is the shape `after` already has. A redial carries both,
  and the handshake gains no frame.
- The store's head is exact for one log (VTT-331), so `seat.catchUp`'s drain
  goes.
- `actor_id` only:
  - the owner's ruling names it;
  - the client resets on the frame and needs nothing else;
  - the contract is additive, so a head can be added later.
  - *Partly a preference.*

**D5. The open, and a switch, settle before they speak.**

- **At the open:**
  1. `WatchAppends`, first;
  2. then `Actors()`, resolve (the named `log` if `MayWatch` accepts it,
     else the default), `SubscribeCharacter`, and a non-blocking check for a
     waiting wake.
  3. If one waits, the seat consumes it and reads the actors again. If they
     resolve to the same viewpoint, it stops; if not, it unsubscribes and
     repeats from step 2.
  4. Only then is `CatchUpHead` queued.
- **On a switch,** the same loop runs before `ViewpointChanged` is sent.
- The loop has no counter. Each further pass needs an append after which
  the actors resolve to a different viewpoint.

*Forced by:*

- VTT-086: the head is the last sequence the seat is sent. A switch decided
  after the head was queued would give a head from the abandoned log.
- A counter would be a boundary mutant (SPEC-020).
- What stays open is G10: a switch after the head was sent and before it was
  reached.

**D6. The move gate reads the viewpoint the pump shows (Task 5).**

- The pump publishes the shown viewpoint in an `atomic.Pointer[string]` on
  the seat, written only after it sends `ViewpointChanged`. `handleCommand`
  reads it.
- The gate judges with `perceive.Eyes{Shoulder: v}`:
  - `v` is the shown viewpoint when `MayWatch(p, shown, st.Actors)` accepts
    it;
  - otherwise `v` is `defaultViewpoint(p, st.Actors)`.
- A `set_viewpoint` the pump has not applied yet is not used.

*Forced by:*

- SPEC-015 and SPEC-013. The seat is the pump's, and the command goroutine
  never waits on it. The published pointer is a mirror that is only read,
  not a lock on the seat. SPEC-015 gains one sentence for it.
- "The character they watch" is the log the connection is being sent.
- **The fallback makes the gate deterministic after a grant the pump has
  not applied yet.**
  - Without it, `planPendingGrant`'s grant followed by the soak's next
    `moveOwn` races the pump. So does `three-role-exit`'s step 7, three
    steps after the grant. Both would be refused "cannot see".
  - The fallback also fails closed after a revocation: a lost character's
    eyes are never used.
- Consequence G7: a player with no party member has no eyes, so every move
  they make is refused as unseen.

**D7. `perchBox` survives unchanged: one slot, latest wins, a wake of
capacity one, and `set` never blocks.**

- `handleSetViewpoint` hands every accepted viewpoint to it, the player's
  and the spectator's.
- The pump answers a viewpoint equal to the one shown with no frame.
- A player's empty `set_viewpoint` is refused, with its own string.

*Forced by:*

- VTT-188 and VTT-306 still hold.
- A switch now costs an unsubscribe and a `SubscribeCharacter` that holds
  `c.mu` while it reads history (3a's D6). A burst must not pay for that
  once per click.
- The ticket's rule: a change is announced, and no change sends nothing.
- The ticket refuses the empty viewpoint, because the default would take a
  party member again at once. *Stated to the owner as a default at sign-off,
  not asked.*

**D8. Authorization.**

- `commandRoles["set_viewpoint"]` gains the player cell.
- `Authorize`'s every-role check runs `MayWatch`, as it now runs `MayPerch`.
  `MayWatch` decides:

  | Role | Accepted | Refused |
  |---|---|---|
  | spectator | `""`, or a party member | anything else |
  | player | a party member they control | `""`, and anything else |
  | any other role | — | everything, naming the role |

- One string answers every refusal of a named actor, for both roles:
  `%w: %q is not a party member you may watch`. It is used for an absent
  actor, a non-party actor, and an actor the player does not control.
- The player's `playerRules` entry is `unrestricted`. SPEC-013's table writes
  the cell as `MayWatch`.

*Forced by:*

- The rule that a refusal says nothing about existence, party, or control.
  The spectator's existing string, "is not a party member", would be false
  for a party member the player does not control.
- One function writes one sentence.
- `TestEveryPlayerCommandHasARule` needs an entry, and a second call would
  be unobservable. *Partly a preference.*

**D9. What leaves the gateway.**

- **Deleted:**
  - from `project.go`: `Viewer`, `Projector`, `NewProjector`, `eyesFor`,
    `Project`, `perchSequence` and `reperch`;
  - from `seat.go`: `viewerFor`, `receive`, `pastResume`, `perch`,
    `catchUp` and `subscribeFrom`.
- **Renamed:** `projected` becomes `watchesALog`, false for the DM and the
  agent.
- **Changed:** `canSee(viewpoint, st, scene, at)` asks
  `perceive.CanSee(Eyes{Shoulder: viewpoint}, …)`.
  - In Task 4 it still passes the union, as `Eyes{Controller: p.ID}`. Task 5
    moves it.
- **`campaign.FoldPrefix` stays**, for campaign's own tests: VTT-323's
  evidence folds logs with it. Its 39-line doc names `seat.receive` and is
  cut to a pointer of six lines or fewer.

*Forced by:*

- The ruling: the projection is deleted.
- Rule 4: `FoldPrefix` existed so that the gateway had no fold loop of its
  own, and no gateway fold remains.
- SPEC-010: no edit inside a block over six lines.

**D10. The scenario and soak denial proofs read world seats only (Task 3, its
own commit).**

- `leakedSince` and `leakedBelow` skip character seats, the players and
  spectators named by `projectedSeats`.
- A scenario that has a denial and no world seat fails each denial as
  unprovable.

*Forced by:*

- **VTT-321.** A character seat's sequences are its log's own, so comparing
  them with a world sequence is a category error. It can report a false
  leak, or miss a real one.
  - The soak is the clear case. Its players' logs are far shorter than the
    world, so each witness's entries would be read as leaks.
  - Task 4 shows this in a deliberate break.
- **The narrowing loses nothing a denied append can cause.** A character
  entry exists only through an append's fan-out (SPEC-021), and a refused
  append delivers none (VTT-332). So "no append" on the world seat implies
  "no entry caused by it" on every character seat.
  - One denial sends entries with no append: a refused `set_viewpoint` that
    the pump applies anyway replays a log from its first entry. No world seat
    sees that, and D11's run check accepts a run that restarts at 1. The
    gateway's tests hold it: VTT-347's
    `TestAPlayerMayWatchOnlyTheirOwnPartyMembers` and VTT-193's
    `TestARefusedViewpointLeavesTheConnectionWhereItWas`. No committed
    scenario issues `set_viewpoint` today.
  - Every committed scenario dials the DM first, and the soak dials the DM
    and the agent.
  - The unprovable rule stops a scenario with no world seat from passing
    vacuously.
- **What the old check also caught**, a frame on a player's stream that no
  append caused, moves to D11 where it breaks the log's run, and to the
  gateway's new tests where it does not (the refused switch above).
- Rule 2: a narrowed scope is a gate change. It gets its own reviewed commit
  and its own reason.

**D11. A character seat's stream is checked for being one log (Task 6).**

- **A step fails** when a character seat receives an envelope whose
  sequence is neither the previous one plus one nor 1.
  - 1 is a log from its start.
  - A world event, a duplicate, or a stray frame breaks the run.
- **The soak** checks the same at each checkpoint.
- **`runReconnectStep`** on a character seat compares the replay with the
  live history since the last envelope of sequence 1. It compares by
  sequence and event id.
  - With `after > 0`, it expects the whole current log, because the harness
    names no log.
  - Passing `log` through the `Dialer` is ticket 4's.

*Forced by:*

- D10's "what the old check also caught".
- The ticket's rule that a switch replays from the first entry. A live
  history that spans two logs must not be compared with one log's replay.
- `client.go` drops `ViewpointChanged`, and a reset to 1 is visible without
  changing the `Conn` interface or its fakes.

**D12. Revocation for a seat that is sent nothing:** `credentialGone` runs on
every wake. A revoked participant who watches nobody is closed with
`StatusPolicyViolation` at the next append, as
`TestARevokedSpectatorStopsSeeingTheTable` already requires.

*Forced by:*

- VTT-032 and SPEC-009: a spectator issues no command.
- The same cost as today, where every projected seat re-resolves per world
  event.

**D13. The lifecycle of a seat that has no character subscription.**

- When the wake channel closes, which `Campaign.Close` does, the pump calls
  `CloseNow`. The same happens when a character subscription closes and the
  pump did not unsubscribe it itself; the pump swaps the channel before it
  unsubscribes.

*Forced by:*

- SPEC-011: a stream the server did not end on purpose closes the socket.
  Without a wake, a seat that watches nobody would never notice the campaign
  closing.

**D14. Where the tests go.**

- The new black-box tests go into `internal/gateway/watch_test.go`, new.
- New white-box tests go into `internal/gateway/seat_internal_test.go`, new.
  `TestARapidHopIsCoalescedToTheShoulderItEndedOn` and
  `TestAPerchSetWhileNothingWaitsStillWakesThePump` move there with their
  names unchanged.
- `server_visibility_test.go` stays, holding only its surviving tests,
  rewritten, so their evidence keys keep their path.
- `mustApply` and `twoRooms` move from `project_test.go` to
  `viewpoint_test.go`, with what they call: `envelope` for `mustApply`,
  `twoRoomsTiles` and `key` for `twoRooms`. `forwardedOf` goes with every
  file that uses it.

*Forced by:* the register. Evidence names `path#Test`, and keeping a
surviving test's path and name keeps its row's evidence true without an edit.

**D15. The order and the split.**

- **Task 4** moves the seat and deletes the projection in the same commit,
  as the ticket orders.
- **Task 5** moves the move gate.

*Forced by:*

- Review size. Task 4 alone deletes about 11,000 test lines.
- The gate is separable. For one commit, `perceive` still offers the union
  as `Eyes{Controller}`, which is exactly today's gate.
- Task 5's tests (item 7, D6) need the seat's published viewpoint, which
  Task 4 creates.

**D16. `tools/toolgen`'s `set_viewpoint` description** says who may choose
what:

- a spectator chooses any party member, or nobody;
- a player chooses one of their own party members;
- the DM and the agent are refused;
- the choice appends nothing and is announced by `ViewpointChanged`.

`task generate:contract` refreshes `cmd/vtt/tools.json` and
`contract/testdata/expected_tools.json`.

*Forced by:* `check:drift` and the ticket's item 6.

## Gaps carried from the verdict

- **G1.** `cmd/vtt/webdist` embeds `commands.proto`'s descriptor (probe: the
  `fileDesc` prefix appears once in `assets/index.js`). Task 1 rebuilds it
  with `task build:client` and commits it.
- **G2.** Register rows the ticket's classes miss. The table below sorts
  them.
- **G3.** SPEC-012 ("the projection's"), SPEC-017 ("unprojected (SPEC-015)")
  and SPEC-019 ("a seat fed it withholds") are amended in Phase 5.
- **G4.** Item 3 orders a result against pump frames. `watch_test.go` reads
  frames by kind: the result by request id, and `ViewpointChanged` before
  the new log's first entry.
- **G5.** The causal reading of "after", D3.
- **G6.** Files the ticket does not name:
  - `internal/campaign/foldprefix.go` (Task 4);
  - `internal/harness/late_leak_test.go` (Task 3);
  - the Go pointer sweep (Task 7);
  - `internal/gateway/qa_promotion_revoked_test.go`, which sends a
    spectator's empty `set_viewpoint` and stays valid.
- **G7.** A player with no party member moves nothing. Put to the owner.
- **G8.** Between this ticket and ticket 4, a player's or spectator's
  browser client is broken. It drops `ViewpointChanged`, and its redial
  sends no `log`, so the log replays from 1 onto a fold that already holds
  it. `vtt mcp` as a spectator who perches twice is broken the same way.
  `task check` holds no live-server client test, so it stays green.
- **G9.** VTT-189 survives only if "stall" means waiting on this seat's
  pump. An append waits for a switch's history read under `c.mu` (3a's D6).
  The report records a switch's catch-up time on the soak's longest log.
- **G10.** A switch made after `CatchUpHead` is sent and before its sequence
  is reached leaves a reader that waits for the head waiting. Such a reader
  must also stop at `ViewpointChanged`: ticket 4's client, and the harness's
  `state dump` there.
- **G11.** Found by the reading review of the first commit, and not decided
  here. A player's `CommandResult` still carries `sequence`, the first world
  sequence its command produced (SPEC-007). Two results tell a player how
  many world events landed between their commands, the DM's included. That
  is world-log information on a player's connection, outside VTT-366's
  wording. Changing it changes the wire and the harness, which takes a
  player's accepted step's witness from it. Put to the owner.

## Questions for the owner

Asked at sign-off on 2026-10-17, and answered:

1. **D10, rule 2.** The harness's denial proof narrowed to the world seats,
   with D11's run check for character seats: approved ("Godkänn").
2. **D4.** A reconnect's `log` chooses the opening viewpoint while the
   participant may watch it ("Tjuven direkt"): the default is for a
   connection that has chosen nothing. The ticket was amended to match:
   - its rulings paragraph's second bullet;
   - item 6;
   - three rules: the opens-on rule, the default rule and the cursor rule.

   D4, D5 and NEW-2a, NEW-2 and NEW-9 below follow it.
3. **G7.** Not accepted as the end state: a controlled creature that is not a
   party member gets a log of its own, under a new kind of grant, whose log
   is closed when the creature goes or the control ends and is never reused,
   so a second summoning or possession is a new log. That is its own ticket,
   after this one. Until then G7 stands as stated: such a player watches
   nobody and moves nothing (NEW-26). The ticket's rulings paragraph gained
   its fourth bullet from this answer.
4. **D15 and the size.** One ticket, nine tasks, Task 4 one commit: signed
   off ("Ja, kör").

**D7's empty `set_viewpoint` for a player** was stated to the owner as a
default (refused) and not asked; he did not object.

## The register: affected rows

Every row whose wording or evidence this work touches, found by a scan of the
register for player, spectator, projected, perch, viewer, seat, shoulder,
projection and viewpoint, and for evidence that names a file this work
deletes. Rows are never re-worded. A false row is withdrawn and succeeded,
and only evidence cells are re-pointed. The sort after sign-off decides.

### Withdrawn and succeeded

| Id | Why its wording becomes false | Successor | Task |
|---|---|---|---|
| VTT-141 | a player may now issue `set_viewpoint` | NEW-5, NEW-5a | 4 |
| VTT-155 | "they cannot see" meant the union | NEW-11 | 5 |
| VTT-177 | the stream is the log, not a projection | NEW-1 | 4 |
| VTT-178 | "projected seat"; resuming is a log's | NEW-9 (and VTT-329) | 4 |
| VTT-179 | "projected seat" | NEW-9 | 4 |
| VTT-183 | "a perch naming no actor is accepted" is false for a player's `set_viewpoint` | NEW-6 | 4 |
| VTT-185 | nothing is folded per seat; OPEN | held by nothing | 4 |
| VTT-186 | a perch has no frames; the log comes from 1 | NEW-4 | 4 |
| VTT-187 | no perch frames to filter | NEW-4 | 4 |
| VTT-190 | no projected seat | VTT-319 (the fan-out's analogue) | 4 |
| VTT-191 | no seat fold | VTT-320 | 4 |
| VTT-194 | the union goes | NEW-1 | 4 |
| VTT-208 | "every player and spectator" includes a seat that watches nobody | NEW-19 | 4 |
| VTT-216 | same | NEW-20 | 4 |
| VTT-217 | same | NEW-21 | 4 |
| VTT-225 | the two-sided keystone goes (arc §6) | VTT-323 | 4 |
| VTT-226 | a switch sends the whole log | NEW-4 | 4 |
| VTT-234 | "every player and spectator" | NEW-22 | 4 |
| VTT-241 | a controlled non-party actor is introduced only when seen | NEW-19; the non-party clause has no successor, by the "monster is no viewpoint" default | 4 |
| VTT-246 | "a player's own actors" was the union | NEW-23 | 4 |
| VTT-314 | no projected seat fold | VTT-320 | 4 |

Each successor row is allocated before Task 1 and reads OPEN until its test
lands. Its predecessor is withdrawn in the task given, with its evidence
`**READING — verify-ticket check 5 of 2026-10-17; withdrawn, held by
nothing**`, as VTT-211 was. Tests that still cite a withdrawn id resolve (the
row exists). The perceive tests are left unedited (item 8).

### Re-evidenced: the wording still holds, and evidence is re-pointed

| Id | Evidence after | Task |
|---|---|---|
| VTT-032 | its four `server_test.go` tests, unchanged in name; NEW-12 carries the nobody seat | 4 |
| VTT-086 | `server_visibility_test.go#TestEverySeatCanReachTheCatchUpHeadItIsGiven`, rewritten | 4 |
| VTT-089 | add `seat_internal_test.go#TestAWedgedWatcherIsTornDownAndOthersKeepServing` | 4 |
| VTT-156, VTT-158 | the `server_visibility_test.go` move tests, rewritten in place | 5 |
| VTT-160 | `server_test.go#TestMoveTokenBroadcastBackfillsSceneAndFrom`, reading the DM's stream | 4 |
| VTT-176 | `TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection`, kept (item 9) | 4 |
| VTT-180 | `watch_test.go#TestAPlayerWatchesTheirFirstPartyMemberByID`; `TestASpectatorWithNoPerchReceivesNoBoard`, rewritten | 4 |
| VTT-181, VTT-182 | `viewpoint_test.go` on `MayWatch`; `authz_test.go`; `TestASpectatorMayNotPerchOnTheGoblinArcher`, rewritten | 4 |
| VTT-184 | `TestPerchingAppendsNothingToTheLog`, rewritten for a player and a spectator | 4 |
| VTT-188, VTT-306 | `seat_internal_test.go`, the moved tests | 4 |
| VTT-189 | `TestHoppingWhileTheTableIsBusyKeepsOneOrder`, rewritten (G9) | 4 |
| VTT-192 | `watch_test.go#TestAPlayerSwitchesToAnotherOfTheirOwn`; `TestASpectatorHopsFromOneShoulderToAnother`, rewritten | 4 |
| VTT-193 (OPEN today) | `watch_test.go#TestARefusedViewpointLeavesTheConnectionWhereItWas` | 4 |
| VTT-195 | the perceive entry stays; `TestASpectatorHopsFromOneShoulderToAnother` | 4 |
| VTT-219 | `internal/campaign` table test (Task 2); the keystone entry is dropped | 2 / 4 |
| VTT-239 | `internal/campaign` table test (Task 2); the QA entry is dropped | 2 / 4 |
| VTT-262, VTT-272, VTT-273 | the perceive entries stay; `TestAPlayerIsSentAMoveWithoutItsReasonAndTheDMWithIt` and `TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause`, rewritten | 4 |
| VTT-202, 203, 204, 206, 212, 213, 218, 224, 233, 235-238, 240, 242, 247, 248, 251, 312, 313, 319 | the deleted gateway entries are removed; each cell keeps at least one surviving perceive or campaign entry (checked cell by cell) | 4 |

### Untouched

Wording and evidence both hold:

- VTT-010, VTT-025, VTT-027, VTT-066, VTT-116;
- VTT-139 and VTT-140, whose evidence test changes cells but still holds
  them;
- VTT-142-150, VTT-157, VTT-159, VTT-161, VTT-162;
- VTT-196-201, VTT-205, VTT-209, VTT-210, VTT-214, VTT-215, VTT-220-223,
  VTT-227, VTT-243-245, VTT-250, VTT-252;
- VTT-260, VTT-261, VTT-329-337.

### New rules, allocated after sign-off

The wording of each rule is the register's (`docs/requirements.md`, by id),
and this table does not restate it. It names the key the rest of this plan
uses, the test that is to hold the rule, and the task that lands that test.

| Key | Id | Test that holds it | Task |
|---|---|---|---|
| NEW-1 | VTT-338 | `watch_test.go#TestAPlayersConnectionIsItsCharactersLog` | 4 |
| NEW-2a | VTT-339 | `TestACursorIsHonouredOnlyInTheLogItWasReadFrom` | 4 |
| NEW-2 | VTT-340 | `TestAPlayerWatchesTheirFirstPartyMemberByID`, `TestAConnectionStopsWatchingAnActorItMayNoLongerWatch`, `TestALogNamingAnActorItMayNotWatchOpensOnTheDefault` | 4 |
| NEW-3 | VTT-341 | `TestAPlayerWatchesTheirFirstPartyMemberByID` | 4 |
| NEW-4 | VTT-342 | `TestAPlayerSwitchesToAnotherOfTheirOwn`, `TestASpectatorWatchesThePartyMemberItPerchesOn` | 4 |
| NEW-4b | VTT-367 | `TestAPlayerSwitchesToAnotherOfTheirOwn` | 4 |
| NEW-4c | VTT-368 | `TestAPlayersConnectionIsItsCharactersLog`, `TestASpectatorWithNoPerchReceivesNoBoard` | 4 |
| NEW-5 | VTT-343 | `TestAPlayerMayWatchOnlyTheirOwnPartyMembers`, `authz_test.go#TestAuthorizeTableAllCommandsAllRoles` | 4 |
| NEW-5a | VTT-344 | `TestAuthorizeTableAllCommandsAllRoles` | 4 |
| NEW-6a | VTT-345 | `viewpoint_test.go#TestUnperchingNamesNoActorAndIsAllowed`, rewritten | 4 |
| NEW-6b | VTT-346 | `viewpoint_test.go`, a player's empty viewpoint | 4 |
| NEW-7 | VTT-347 | `TestAPlayerMayWatchOnlyTheirOwnPartyMembers`, `viewpoint_test.go#TestAPerchRefusalDoesNotSayWhetherTheActorExists` | 4 |
| NEW-8 | VTT-348 | `TestAConnectionStopsWatchingAnActorItMayNoLongerWatch`, `seat_internal_test.go#TestNoEntryCausedByTheLossIsSent` | 4 |
| NEW-9 | VTT-349 | `TestACursorIsHonouredOnlyInTheLogItWasReadFrom` | 4 |
| NEW-11 | VTT-350 | `TestAPlayersMoveIsJudgedByTheCharacterTheyWatch` | 5 |
| NEW-13 | VTT-351 | `TestAMoveRightAfterAGrantIsJudgedByTheNewCharacter` | 5 |
| NEW-12 | VTT-352 | `server_test.go#TestARevokedSpectatorStopsSeeingTheTable`, `qa_promotion_revoked_test.go#TestQAPromoRevokedTheNextEventClosesARevokedWatcherWhoWasSentNoPromotion` | 4 |
| NEW-14a | VTT-353 | `internal/campaign/append_watch_test.go#TestEveryAppendWakesEveryWatcher` | 2 |
| NEW-14b | VTT-354 | `append_watch_test.go#TestARefusedOrFailedAppendWakesNobody` | 2 |
| NEW-15 | VTT-355 | `**READING — Phase 4b**`: no test can see the order, as 3a's VTT-333 | 2 |
| NEW-16a | VTT-356 | `append_watch_test.go#TestClosingTheCampaignEndsEveryWatcher` | 2 |
| NEW-16b | VTT-357 | `poison_internal_test.go#TestAPoisonedCampaignRefusesAWatcherAndHasNoActors` | 2 |
| NEW-17 | VTT-358 | `late_leak_test.go#TestADenialWithNoWorldSeatIsUnprovable` | 3 |
| NEW-18a | VTT-359 | `TestACharacterSeatsEnvelopeOutOfItsLogFailsTheStep`, `TestASoakCheckpointFailsACharacterStreamThatSkips` | 6 |
| NEW-18b | VTT-360 | `TestACharacterSeatsReconnectIsComparedWithItsCurrentLog` | 6 |
| NEW-19 | VTT-361 | `internal/campaign` table test, `TestEveryActiveLogIsToldWhatTheTableIsTold` | 2 |
| NEW-20 | VTT-362 | the same | 2 |
| NEW-21 | VTT-363 | the same | 2 |
| NEW-22 | VTT-364 | the same | 2 |
| NEW-23 | VTT-365 | the same | 2 |
| NEW-24 | VTT-366 | `**READING — Phase 4b**`: the seat's code holds no world subscription for a player or a spectator | 4 |
| NEW-25 | VTT-369 | `TestALogNamingAnActorItMayNotWatchOpensOnTheDefault` | 4 |
| NEW-26 | VTT-370 | `TestAPlayerWithNoPartyMemberMovesNothing` | 5 |

The sort split four rows of the verifier's table into two rules each:
NEW-6, NEW-14, NEW-16 and NEW-18. It added NEW-24, the arc's §10 item 1,
which is held by a reading. The reading review of the first commit split
NEW-4 into NEW-4 and NEW-4b, and added NEW-4c, NEW-25 and NEW-26. NEW-26 is
G7 for now; the controlled-creature ticket withdraws it. In the withdrawal
table, VTT-183 is succeeded by NEW-6a and NEW-6b, and VTT-141 by NEW-5 and
NEW-5a.

The sort refused these, one line each. Some are already rules:
- the spectator's any-party-member-or-nobody, which is VTT-181 and NEW-6a;
- the DM and the agent watching the world, which is VTT-140 and VTT-176;
- a log seat's catch-up head, which is VTT-086 and VTT-331;
- coalescing a burst of switches, which is VTT-188.

The others are not rules:
- `Actors()` returning a copy, and `perchBox` surviving, both describe how,
  not what;
- the projection's deletion is a one-time change, held by item 8's grep and
  standing as NEW-24;
- D10's narrowing of the harness's leak proof is a gate's scope, recorded
  here and in the report.

## Tasks

The tasks are in dependency order. Each one is one Phase 4b review and one
commit.

### Before Task 1: allocate the ids (after sign-off)

- Run the sort over the ticket's rules and the table above.
- Allocate each accepted row with `requirement-id`. Each reads `**OPEN — no
  test yet**`, except NEW-15 and NEW-24, which read `**READING — Phase
  4b**`.
- No row is withdrawn yet.
- The commit carries the ticket, this plan and the new rows.

**True when done:** `check:requirements-chain` is green.

**Done on 2026-10-17:** VTT-338 to VTT-370, by the table above. The reading
review of this commit corrected the wording of these rows before they were
committed: VTT-339, VTT-342, VTT-348, VTT-349, VTT-352, VTT-353, VTT-354,
VTT-356, VTT-357, VTT-359 and VTT-360 were re-dispensed under the same ids
from an unchanged register, and VTT-367 to VTT-370 were added after them.

### Task 1: the contract carries `ViewpointChanged`

**Files:**

- `contract/vtt/v1/commands.proto`: `message ViewpointChanged { string
  actor_id = 1; }`, with a doc of one or two lines that points at SPEC-007;
  and `ViewpointChanged viewpoint_changed = 6;` in `ServerFrame`'s oneof.
- `task generate:contract`, which regenerates `contract/gen/{go,ts}` and
  leaves `tools.json` unchanged.
- `task build:client`, which rebuilds `cmd/vtt/webdist` (G1).
- `contract/testdata/server_frame_viewpoint_changed.json` and
  `server_frame_viewpoint_changed_nobody.json`.
- `contract/roundtrip_test.go`: `TestServerFrameViewpointChangedRoundTrip`.
- `contract/events.test.ts`: the two fixtures added to its table.

**True when done:**

- The round trips pass. `check:drift` is green after the commit.
- `check:breaking` reports no breaking change.
- `git diff --stat HEAD -- internal client/src` prints nothing.

**Deliberate break:** misspell the fixture's oneof key. The Go and the TS
round trips both go red.

**Gate:** the pre-commit hook; then `task check:drift` and
`task check:breaking` after the commit.

**QA:** skipped. It is a shape with no behaviour, and the round trip and
`check:breaking` are the whole of it.

### Task 2: the campaign wakes a watcher after each append (D1, D2; NEW-14-16, NEW-19-23, VTT-219, VTT-239)

**Files:**

- `internal/campaign/campaign.go`:
  - `watchers` on `Campaign`;
  - `WatchAppends` and `Actors` beside `SubscribeCharacter`, each with a doc
    of at most two lines;
  - the wake loop in `appendLocked`, between the live apply and the
    `Notify` loop;
  - `Close` closes and clears the watchers.
  - The share must stay at or under 29.8: new code must be under 29.8%
    comments.
  - The `Campaign` doc block (8 lines) is not edited.
- `internal/campaign/append_watch_test.go` (new):
  - `TestEveryAppendWakesEveryWatcher`: two watchers and one append; both
    wake; a burst of three appends before either reads leaves one waiting
    signal;
  - `TestARefusedOrFailedAppendWakesNobody`, one test for each:
    - a refused `Append`, an unknown payload;
    - a failed `AppendBatch`, using a `BEFORE INSERT ON events` trigger with
      `RAISE(ABORT, …)` on a second handle, the 3a precedent;
  - `TestStoppingAWatcherEndsOnlyThat`;
  - `TestClosingTheCampaignEndsEveryWatcher`: the channel closes, and a
    later `WatchAppends` errs;
  - `TestActorsIsACopyOfTheLiveActors`: mutate the result; `State()` still
    holds the original;
  - `TestEveryActiveLogIsToldWhatTheTableIsTold`, the successor test for
    NEW-19-23, VTT-219 and VTT-239. It uses two party members in different
    rooms and one tokenless party member. It checks that:
    - every party member is introduced in each log;
    - narration, `SessionStarted`/`SessionEnded` and a public note reach
      each active log;
    - no `AdventureLoaded` entry is written;
    - making a note secret and deleting a note both reach a log that holds
      the note as the same bare `NoteDeleted`;
    - the tokenless member's own `ResourceChanged` reaches its own log.
- `internal/campaign/poison_internal_test.go`:
  `TestAPoisonedCampaignRefusesAWatcherAndHasNoActors`.
- `docs/requirements.md`: evidence for the rows this task holds.
- `tools/comment-ceilings.txt`: `--write-ledger`, if `campaign.go` falls by
  more than the band.

**True when done:**

- The new tests pass. The watch tests do not compile on `fa6a067`. The
  table test passes on arrival, since it holds ticket 2's behaviour.
- `go test -race ./internal/campaign/ ./internal/store/` is green.
- Every existing campaign and gateway test passes unchanged.
- The report records `Actors()` against `State()` on the soak's campaign
  (D2).

**Deliberate breaks:**

1. Raise the wake before the live apply. No deterministic test goes red.
   This is NEW-15's READING, and the review reads where the loop sits.
2. Raise the wake on the refused path. The refused test goes red.
3. Send on the wake blocking. The burst test hangs, under `-timeout`.
4. `Close` skips the watchers. The close test goes red.
5. `Actors` returns the live map. The copy test goes red.
6. For the table test, one break per rule it holds:
   - a fan-out with no narration arm goes red on narration (NEW-20);
   - withholding a party member's introduction goes red on NEW-19;
   - withholding `SessionStarted` and `SessionEnded` goes red on NEW-21;
   - withholding a public note goes red on NEW-22;
   - dropping the log's own character from what counts as seen goes red on
     NEW-23.
   The breaks are made in `perceive` (`classify`, `look` and the note and
   session arms). They are reverted, and nothing in `perceive` is
   committed.

**Gate:**

- the pre-commit hook;
- `go test -race ./internal/campaign/`;
- `check:requirements-chain`;
- `check-comments.py` against `main` and `HEAD`;
- `go-arch-lint check`.

**QA:** runs (see Phase 4a below).

### Task 3: the denial proofs read world seats only (D10; NEW-17)

**Files:**

- `internal/harness/engine.go`:
  - `leakedSince` takes the character-seat set and skips it. Its 10-line
    doc is cut to six lines or fewer.
  - `RunScenario` marks every denied step unprovable when no world seat is
    connected. The detail names it, as `markUnprovableSilence` does.
- `internal/harness/soak.go`: `leakedBelow` skips the character seats, with
  its doc cut to six lines or fewer.
- `internal/harness/late_leak_test.go`:
  - `TestDeniedCommandLeakFailsTheDeniedStep` asserts the leak line names
    `dm`. It no longer asserts `player`.
  - `TestDenialFollowedByReconnectDoesNotPanic` reconnects the DM, so the
    `min` guard is still reached on a seat it reads.
  - New: `TestADenialWithNoWorldSeatIsUnprovable`.
  - New: `TestALeakTheWorldSeatSawFailsTheDenialWhateverACharacterSeatSaw`.
- `internal/harness/soak_test.go`: a leak injected on the agent's stream
  only is still caught.
- `docs/requirements.md`: NEW-17's evidence.

**True when done:**

- The harness tests pass.
- `TestScenarioLibraryRunsSelfContained`,
  `TestThreeRoleExitScenarioOverLiveServeSubprocess` and
  `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts` pass,
  with the projection still in place.
- The soak's pinned counts are unchanged.

**Deliberate breaks:**

1. The fake DM receives a leaked envelope. The denied step goes red.
2. Remove the DM from a fake scenario that has a denial. The step is red as
   unprovable.
3. Revert the skip in `leakedBelow`. Nothing goes red now. Task 4's break 6
   is where this shows.

**Gate:**

- the pre-commit hook;
- `go test ./internal/harness/`;
- `go test -run 'TestScenarioLibrary|TestThreeRoleExit|TestClientSoak' ./cmd/vtt/`;
- `check-comments.py` against `main` and `HEAD`.

**QA:** runs (see Phase 4a below).

### Task 4: a seat streams its character's log, and the projection goes (D2-D5, D7-D9, D12-D14, D16; items 1-6, 8-10)

**Files, production:**

- `internal/gateway/seat.go`, rewritten:
  - the seat, holding the participant, the shown viewpoint, the character
    subscription and the wake;
  - `perchBox`, unchanged;
  - `watchesALog`;
  - the settle loop (D5);
  - the priority receive and the checks (D3);
  - `canSee`, still passed `Eyes{Controller: p.ID}` until Task 5.
- `internal/gateway/server.go`:
  - `handleWS` parses `log`;
  - `serve` splits by `watchesALog`. The DM and the agent keep their path
    unchanged. A character seat opens per D4 and D5, and its pump runs D3,
    D7, D12 and D13;
  - `handleSetViewpoint` is unchanged, apart from now serving the player.
- `internal/gateway/viewpoint.go`: `MayWatch` and `defaultViewpoint` (D2,
  D8).
- `internal/gateway/authz.go`: the player cell, the `playerRules` entry, and
  `Authorize` calling `MayWatch`.
- `internal/gateway/project.go`: deleted.
- `internal/gateway/export_test.go`: `ReperchForTest` and
  `NewProjectorWithSentForTest` deleted.
- `internal/campaign/foldprefix.go`: the doc cut to six lines or fewer (D9).
- `tools/toolgen/main.go` (D16), then `task generate:contract`, which
  refreshes `cmd/vtt/tools.json` and `contract/testdata/expected_tools.json`.
- `internal/harness/engine.go`: `projectedSeats`' doc says character seats.
  The function is renamed `characterSeats`.

**Files, tests deleted.** Each file below drives `Projector` or reads the
projection goldens:

- `project_test.go`, `project_internal_test.go` and `project_property_test.go`;
- `keystone_test.go`, `history_fold_test.go` and `testdata/history-fold/`;
- `resume_test.go`, `character_log_test.go` and `viewpoint_internal_test.go`;
- `qa_fold_memory_test.go`, `qa_note_projection_test.go` and
  `qa_note_adventure_test.go`;
- `qa_testimony_eyes_test.go`, `qa_testimony_removal_test.go` and
  `qa_testimony_sight_test.go`;
- the `Projector`-driving tests of `qa_issuer_test.go` and
  `qa_move_reason_test.go`. Their over-the-wire tests for the DM and the
  agent stay.
- From `server_visibility_test.go`: every test but the survivors below.
  `TestSessionZeroCannotHappenAgain`'s claim moves to item 1's test.

**Files, tests rewritten in place.** Names are kept (D14):

- in `server_visibility_test.go`:
  - `TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection`;
  - `TestEverySeatCanReachTheCatchUpHeadItIsGiven`;
  - `TestPerchingAppendsNothingToTheLog`;
  - `TestHoppingWhileTheTableIsBusyKeepsOneOrder`;
  - `TestASpectatorWithNoPerchReceivesNoBoard`;
  - `TestASpectatorHopsFromOneShoulderToAnother`;
  - `TestASpectatorMayNotPerchOnTheGoblinArcher`;
  - `TestAPlayerIsSentAMoveWithoutItsReasonAndTheDMWithIt`;
  - `TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause`;
  - the three move-gate tests, unchanged until Task 5;
- `server_test.go`: `TestMoveTokenBroadcastBackfillsSceneAndFrom`, reading
  the DM's stream;
- `viewpoint_test.go`: on `MayWatch`, with the helpers it needs;
- `authz_test.go`: the player cell.

**Files, tests new:**

- `internal/gateway/watch_test.go` (new):
  - `TestAPlayersConnectionIsItsCharactersLog` (item 1): `proto.Equal`
    against `Campaign.CharacterLog`, then one live entry, then nothing more;
  - `TestAPlayerWatchesTheirFirstPartyMemberByID` (item 2). Its last case
    is a grant while connected;
  - `TestAPlayerSwitchesToAnotherOfTheirOwn` (item 3), reading frames by
    kind (G4);
  - `TestAPlayerMayWatchOnlyTheirOwnPartyMembers` (item 3, four cases, one
    string for three of them);
  - `TestASpectatorWatchesThePartyMemberItPerchesOn` (item 3);
  - `TestAConnectionStopsWatchingAnActorItMayNoLongerWatch` (item 5): a
    revoke, a non-party grant and a removal, then for a spectator;
  - `TestACursorIsHonouredOnlyInTheLogItWasReadFrom` (item 6): `log`
    naming the player's second party member at N opens on it after N;
    `log` naming one they may no longer watch, and no `log`, open on the
    default from its first entry;
  - `TestALogNamingAnActorItMayNotWatchOpensOnTheDefault` (item 6, NEW-2,
    NEW-25): a player's `log` naming a party member another player controls
    opens on the player's default from its first entry; a spectator's `log`
    naming the goblin opens watching nobody;
  - `TestARefusedViewpointLeavesTheConnectionWhereItWas` (VTT-193).
- `internal/gateway/seat_internal_test.go` (new), white-box because it sets
  the server's unexported no-progress budget, as
  `TestAWedgedConnectionIsTornDownAndOthersKeepServing` does:
  - `TestAWedgedWatcherIsTornDownAndOthersKeepServing` (VTT-089, D13);
  - the two moved `perchBox` tests;
  - `TestNoEntryCausedByTheLossIsSent`, white-box: a waiting wake and a
    queued entry; after the actors change, the entry is not forwarded;
  - `TestAClosedWakeClosesTheConnection` (D13).
- `docs/requirements.md`:
  - evidence for NEW-1-9 and NEW-12;
  - the re-evidenced rows;
  - the withdrawals marked Task 4 in the table.
- `tools/comment-ceilings.txt`: `--write-ledger`, which drops the deleted
  files' rows.

**True when done:**

- **Item 8's first grep:** `grep -rn
  'Projector\|pastResume\|subscribeFrom\|reperch\|perchSequence'
  internal/gateway cmd` prints nothing.
- **The ticket's own grep** now also prints the gateway file that calls
  `SubscribeCharacter`. SPEC-021's "What this record does not decide" is
  updated in Phase 5.
- **The new tests pass**, and none compiles on `fa6a067`.
- **`go test -race ./internal/gateway/ ./internal/campaign/` is green.** So
  are `./internal/perceive/` and `./internal/harness/`. `git diff --stat
  HEAD -- internal/perceive` prints nothing.
- **Items 9 and 10:**
  - `TestScenarioLibraryRunsSelfContained`, `TestEveryScenarioFileIsActuallyRun`,
    `TestThreeRoleExitScenarioOverLiveServeSubprocess`,
    `TestClientRunSelfContainedRunsCommittedThreeRoleExitScenario`,
    `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts` and
    `TestScenarioGoldenStreamsHaveNotDrifted` pass.
  - So do `cmd/vtt`'s state dump, events tail and MCP end-to-end tests.
  - `testStateDumpForAProjectedSeat` finishes in under 10 s with zero
    tokens.
- **`check:drift` is green** after the commit, with `tools.json` refreshed.
- **The client suite passes**, unchanged.

**Deliberate breaks:** each is made in the working tree, seen red, and
undone. Assert that each edit landed before trusting a green run.

1. The pump forwards one world envelope to a player. Item 1 goes red.
2. `defaultViewpoint` takes the last id. Item 2 goes red.
3. The switch resumes the new log at the old cursor. Item 3 goes red.
4. No priority receive: the pump forwards an entry before it checks a
   waiting wake.
   - `TestNoEntryCausedByTheLossIsSent` goes red, deterministically.
   - Item 5 may stay green, which is a race. The report names which.
5. The cursor is honoured whatever `log` names. Item 6 goes red. So does
   ignoring `log` and always opening on the default.
6. Revert Task 3's skip in `leakedBelow`. Expected: the seed-1 soak reports
   a leak on `player1` or `player2`.
   - If it does not, D10's forcing reason is weaker than stated, and the
     report says so.
7. `credentialGone` is not run on a wake.
   `TestARevokedSpectatorStopsSeeingTheTable` goes red.
8. The pump ignores a closed wake. `TestAClosedWakeClosesTheConnection` goes
   red.
9. `MayWatch` gives a distinct string for an uncontrolled party member.
   Item 3's one-string assertion goes red.

**Gate:**

- the pre-commit hook;
- `go test -race ./internal/gateway/ ./internal/campaign/ ./internal/harness/`;
- `go test ./cmd/vtt/`;
- `bun test client/test` under `env -u CLAUDECODE`;
- `check:requirements-chain`;
- `check-comments.py` against `main` and `HEAD`;
- `go-arch-lint check`;
- `check:drift` after the commit.

**QA:** runs (see Phase 4a below).

### Task 5: the move gate uses the watched character's eyes (D6; item 7, NEW-11, NEW-13)

**Files:**

- `internal/gateway/seat.go`: the published `atomic.Pointer[string]`,
  written after each `ViewpointChanged`; and `canSee(viewpoint, …)` with
  `Eyes{Shoulder}`.
- `internal/gateway/server.go`: `handleCommand`'s gate reads the published
  viewpoint, or falls back to the default at `st`.
- `internal/gateway/watch_test.go`:
  - `TestAPlayersMoveIsJudgedByTheCharacterTheyWatch` (item 7). It waits for
    `ViewpointChanged` before the second move.
  - `TestAMoveRightAfterAGrantIsJudgedByTheNewCharacter` (NEW-13): a grant
    and an immediate move. The test makes no read in between.
  - `TestAPlayerWithNoPartyMemberMovesNothing` (NEW-26, G7): a player who
    controls only a non-party actor has every move of its token refused with
    the unseen-square string.
- `server_visibility_test.go`: the three move-gate tests (VTT-156, VTT-158)
  rewritten for one character.
- `docs/requirements.md`: NEW-11, NEW-13 and NEW-26's evidence; VTT-155
  withdrawn.

**True when done:**

- Item 7 passes, and fails on Task 4's tree. On `fa6a067` it does not
  compile.
- Items 9 and 10's tests pass again. The soak's counts are unchanged.
- `perceive.Eyes{Controller` has no non-test caller:
  `grep -rn 'Eyes{Controller' --include='*.go' . | grep -v _test.go` prints
  nothing.

**Deliberate breaks:**

1. The gate uses the union. Item 7 goes red.
2. No fallback. `TestAMoveRightAfterAGrantIsJudgedByTheNewCharacter` goes
   red. So does the soak, probably on its first `moveOwn` after a grant.
3. The fallback uses the shown viewpoint even when it is invalid. A
   revoke-then-move case in item 7's test goes red.

**Gate:**

- the pre-commit hook;
- `go test -race ./internal/gateway/`;
- `go test ./cmd/vtt/`;
- `check:requirements-chain`;
- `check-comments.py` against `main` and `HEAD`.

**QA:** runs (see Phase 4a below).

### Task 6: the harness checks that a character seat's stream is one log (D11; NEW-18)

**Files:**

- `internal/harness/engine.go`:
  - the contiguity check in the character-seat drain of `observeOnAll` and
    `observeBatchOnAll`;
  - `runReconnectStep` compares a character seat with its last run from 1.
- `internal/harness/soak.go`: the same check at each checkpoint.
- `internal/harness/engine_test.go` and `soak_test.go`, using fake conns:
  - `TestACharacterSeatsEnvelopeOutOfItsLogFailsTheStep`;
  - `TestACharacterSeatsReconnectIsComparedWithItsCurrentLog`, including a
    switch mid-history;
  - `TestASoakCheckpointFailsACharacterStreamThatSkips`.
- `docs/requirements.md`: NEW-18's evidence.

**True when done:** the harness tests pass, and so do items 9 and 10's
tests.

**Deliberate breaks:**

1. A fake player's stream carries a world sequence. The step goes red.
2. `runReconnectStep` compares the whole history across a switch. The
   switch test goes red.
3. In the real gateway, temporarily send `ViewpointChanged` without
   resetting the subscription. `TestThreeRoleExitScenarioOverLiveServeSubprocess`
   is expected to go red. If it stays green, the report says so.

**Gate:**

- the pre-commit hook;
- `go test ./internal/harness/ ./cmd/vtt/`;
- `check-comments.py` against `main` and `HEAD`.

**QA:** runs (see Phase 4a below).

### Task 7: the projection goldens and the pointers to them go (item 8)

**Files:**

- Deleted:
  - `scenarios/goldens/*/projections/`, seven directories;
  - `client/test/projection-parity.test.ts`.
- `scenarios/goldens/README.md`: the projections section goes.
- The Go-side pointer sweep (G6). Each touched block ends at six lines or
  fewer:
  - `internal/engine/state.go` (two blocks) and `apply.go`;
  - `internal/mapdef/compile.go`;
  - `internal/eventgen/model.go`;
  - `cmd/vtt/library_test.go`;
  - `internal/harness/corpus_actor_kind_test.go`;
  - `tools/check-no-create-scene.py`'s doc;
  - `.go-arch-lint.yml`'s eventgen note;
  - `internal/gateway/server_internal_test.go`: the doc of
    `TestAnEncodeFailureTearsTheConnectionRatherThanTheBatch`, which names
    `project.go` in a block of about 22 lines, cut to six or fewer.
- Left for ticket 4 (the 2026-10-15 plan's "The client and contract comment
  pointers wait for ticket 4"): `client/src` and `client/test` comment
  pointers.
- `tools/comment-ceilings.txt`: `--write-ledger`.

**True when done:**

- `ls scenarios/goldens/*/projections` fails. `ls
  internal/gateway/testdata/history-fold` already fails, since Task 4.
- `grep -rn
  'project\.go\|Projector\|reperch\|perchSequence\|seat\.receive\|pastResume\|projections/\|projection-parity'
  --include='*.go' --include='*.py' --include='*.yml' internal cmd tools
  .go-arch-lint.yml` prints nothing.
- `check:ts-coverage` is green. The verifier's probe: without
  `projection-parity.test.ts`, `fold.ts` is at 400/400 lines and 33/33
  functions, and `wire.ts`, `session.ts` and `app.ts` are at 100%.
- `TestTheCorpusNeverConfersControlAtCreationNorGrantsInSilence` and
  `TestEveryScenarioFileIsActuallyRun` pass.

**Deliberate break:** none new. This task is pure deletion.

**Gate:**

- the pre-commit hook;
- `bun test client/test contract`;
- `python3 tools/check-ts-coverage.py tools/ts-coverage-thresholds.txt`;
- `go test ./internal/harness/ ./cmd/vtt/`;
- `check-comments.py` against `main` and `HEAD`.

**QA:** skipped. It is pure deletion and comment edits.

### Task 8: the gates hold the new code (item 11)

**Files:**

- `tools/coverage-thresholds.txt`: the gateway (93.5), campaign (94.0) and
  harness (85.0) floors raised to a little under what they measure, if they
  rose. They are never lowered. A gateway figure under 93.5 means tests
  must be added in this task.
- `tools/mutation-equivalents.txt`: gateway and campaign survivors are
  killed with tests. D2, D5 and D8 are shaped so that none is equivalent.
  An adjudication needs a one-sentence observable (SPEC-020). Run the gate's
  two self-tests first.
- `tools/comment-ceilings.txt`: `--write-ledger` only.

**True when done:**

- `check:coverage`, `check:comments`, `check:requirements-chain` and
  `check:ts-coverage` are green.
- `check:mutation` is green for gateway and campaign, with the runtimes
  recorded.
- `check:new-prose` has been run by hand and is green.

**Deliberate break:** delete `TestNoEntryCausedByTheLossIsSent` and run the
gateway's mutation pass. Record whether a mutant survives, or whether only
coverage holds, and say which.

### Task 9: `task check`, whole

- Check the load and the free disk first. 17-24 GiB is free, and GOCACHE is
  shared.
- Launch with `start_new_session=True`.
- Do not touch the tree during the run.

**True when done:** green, which is item 11.

### Then the process's Phases 4a, 4b and 5

- **Phase 4a** runs on Tasks 2, 3, 4, 5 and 6, each before its Phase 4b.
  Hash the tree before and after each QA agent. QA is never given the diff,
  the source, the existing tests or this plan. Each QA agent is given:
  - the rows the task holds, as allocated;
  - the ticket's "Rules this puts on the system";
  - the specifications as they stand: SPEC-021 for Task 2; SPEC-007, 011,
    013 and 015 for Tasks 4 and 5;
  - the exported surface of the uncommitted tree:
    - Task 2: `go doc ./internal/campaign Campaign`;
    - Tasks 4 and 5: `go doc -all ./internal/gateway`, plus the `/ws` query
      parameters and the frame order of D4;
    - Tasks 3 and 6: `go doc -all ./internal/harness`;
  - the arch lists above. Gateway tests may not import `store`. Harness
    tests may import `contract`, `engine` and `harness` only.

  QA's passing tests are committed with the task. Its adjudications go into
  the report, one line each.
- **The report,
  `docs/reports/2026-10-17-a-seat-streams-its-characters-log.md`, holds:**
  - "Done looks like", with the observation for each item;
  - the verifier's probes:
    - the bundle prefix;
    - the TS coverage without the parity test;
    - the guards green on `fa6a067`;
  - which test spoke on each deliberate break, Task 4's breaks 4 and 6 in
    particular;
  - D2's `Actors()` against `State()` measurement;
  - a switch's catch-up time on the soak's longest log (G9);
  - G1-G10 and what became of them;
  - the runtimes, the deviations and the QA adjudications.
- **The specifications, after the report:**
  - **SPEC-015**, rewritten:
    - the seat streams one log;
    - who may watch whom (`MayWatch`), the default, `perchBox`;
    - the settle loop and the priority wake;
    - the published viewpoint and the gate's fallback;
    - "a seat is touched from the pump alone", plus that one read-only
      mirror.
  - **SPEC-016**:
    - the gateway half goes (`Viewer`, `Projector`, `Project`, `reperch`,
      `perchSequence`, `eyesFor`, the `sent` memory);
    - the decision's only production caller is the fan-out;
    - `Eyes{Controller}` is held by `perceive`'s tests and has no production
      caller.
  - **SPEC-011**: the catch-up head per log; the wake; `CloseNow` on a closed
    wake or character subscription.
  - **SPEC-013**: the cell, `MayWatch`, and the gate's viewpoint.
  - **SPEC-007**: `ViewpointChanged`, `log`, and the frame order.
  - **SPEC-009**: re-resolution on every wake and every entry.
  - **SPEC-021**:
    - connections follow logs;
    - "What this record does not decide" gets the new grep result;
    - the consequence about the gateway's projection cost is retired;
    - `WatchAppends` and `Actors` are added.
  - **SPEC-012, SPEC-017 and SPEC-019**: one sentence each (G3).
  - The Requirements lines gain the new ids and lose the withdrawn ones.

## What this plan does not do

- No client change but the parity test's deletion. Ticket 4 makes the client
  reset on `ViewpointChanged`, redial with `log`, and fix its comment
  pointers.
- The harness does not fold each character's log, and its `Dialer` does not
  pass `log`. Both are ticket 4's.
- No change to `internal/perceive`, `internal/store`, `engine` or `sight`.
- `Subscribe`'s early unlock and 3a's D6 hold stay as they are.
- `.go-arch-lint.yml`'s gateway list is not tightened.
- No sight memo, and no measurement of the fan-out's cost beyond G9 and D2.
  Re-measuring SPEC-021's cost is its own ticket.
