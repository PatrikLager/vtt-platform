# Plan: the Go readers follow the log a player's or a spectator's connection is sent

- **Ticket:**
  `docs/superpowers/specs/2026-10-21-the-go-readers-follow-the-log-they-are-sent-design.md`.
- **Verdict:** passes with gaps (G1-G13, below).
  - Verified 2026-10-21, read-only, against HEAD `a801300`, by an agent that
    did not write the ticket.
  - Its probes ran on a copy of the tree made with `git archive HEAD`, and
    wrote nothing into the repository.
  - They used `go test`, `bunx vite build`, a probe scenario, a raw WebSocket
    reader, and the open debt entry's recipe applied to the copy and reverted.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-21-the-go-readers-follow-the-log-they-are-sent.md`.
- **Base:** main `a801300`.
- **Branch:** `feat/the-go-readers-follow-the-log-they-are-sent`.
- **Signed off 2026-10-21** by the owner ("Ja, kör, samma regel"): D1-D20,
  with Q1 to Q3 answered as recommended (below), and Q4 to Q6 as the plan
  settles them. The reading review of the same day then amended D15, D16, D18,
  D19, the task order and the rows' wording, under the standing rule.
  - The standing rule holds: review findings are fixed and each task is
    committed without asking, except a design question. Push and merge happen
    on the owner's word.

## The gaps this plan inherits

- **G1. A command that appends nothing fails as a scenario step.**
  - On the probe, a `set_viewpoint` step returned ok with no sequence, then
    failed: "event (sequence 0) not observed matching by: dm".
  - `runCommandStep` waits for every world seat to observe the result's
    sequence.
  - No committed scenario has run such a command.
  - Item 3 needs one. D8 settles it.
- **G2. VTT-360 is false as worded once the change lands.** D7 and D15
  withdraw it and allocate a successor.
- **G3. A log seat's reconnect is compared by `EventId` and `Sequence`.**
  - A log's own entries carry no `EventId` (SPEC-021).
  - On the probe, entries 1-3 of two different logs were byte-identical.
  - Today's reconnect matched three entries of the wrong log before it failed.
  - `TestQALSReconnectComparesByEventIDAndSequenceOnly` pins the weak
    comparison. D7 overturns it.
- **G4. Nothing holds `vtt mcp` keeping its log at a redial.** No item checks
  that a `ViewpointChanged` naming the log it holds leaves its entries in
  place. Task 5 adds the test (VTT-419).
- **G5. The dump's new key would move the golden corpus.**
  - The nine golden `state.json` files are DM dumps in `vtt state dump`'s
    shape.
  - `dumpJSON` in `internal/harness/fold_golden_test.go` and `foldToDumpJSON`
    in `client/src/fold.ts` both mirror that shape byte for byte.
  - D14 and Q3 settle it.
- **G6. One of the ticket's two dump options contradicts SPEC-007.** SPEC-007
  says a snapshot reader stops at a `ViewpointChanged` after the first. D13 and
  Q2 settle it.
- **G7. Unnamed files and functions.** Every task below lists its files.
  `gocyclo` leaves little room under its 30:
  - `RunSoak` is at 28;
  - `RunScenario` is at 28;
  - `runReconnectStep` is at 25.
- **G8. The client gets SPEC-022 (Q1), so more records move.** D16 lists
  them. SPEC-011's pointer stays, since the snapshot sentence it names stays
  in SPEC-007.
- **G9. The soak's new comparison collides with "blamed once".** A seat whose
  stream broke would be named at every later checkpoint. D11 settles it.
- **G10. Rule 9 correction.** MapTool has no automatic reconnect (below).
- **G11. Item 3 also fails today at its `set_viewpoint` step,** not only at
  the reconnect step.
- **G12. Timing, established:**
  - `internal/mcp` re-mutates, at about 857 s by the gate's own figure.
  - `internal/harness` and `cmd/vtt` are outside the mutation gate.
  - The TS client re-mutates. Its last run took 42 min 47 s, and the last
    green `task check` took 3663 s.
- **G13. The seed-1 soak cannot show "a fresh dial naming that log".** Each of
  its players holds one party member that is never revoked, so a dial with no
  `log` opens the same log. Task 4's test uses a fake whose dial with no `log`
  opens another.

## Constraints that bind every task

**CLAUDE.md's rules.** They are named here and stated where they live:
- **2.** No floor, ceiling, threshold, deadline, exemption or adjudication
  moves to get a green run.
  - The MCP e2e deadlines are never weakened.
  - A ceiling row only goes down, by `--write-ledger`.
- **3.** No contract change. Every frame these readers need exists.
- **4.** No second fold loop. Every fold here is `harness.Fold`.
- **5.** No game-system vocabulary. "Party member", "log" and "viewpoint" are
  the platform's words.
- **8.** Cite names: never `file:line`, never a `.superpowers/` path, never
  "this task".
- **9.** Answered below.
- **10.** A comment added under `internal/`, `cmd/` or `client/src` is a
  warning, a pointer, or an exported symbol's one-line doc. Test files carry
  citation lines.

**The owner's rulings.** They are recorded in the ticket, in SPEC-021 (two
steps, VTT-392) and here:
- 2026-10-20, "Två, klienten först".
- 2026-10-21: VTT-400 keeps its wording; the three perch comments are
  re-pointed; this ticket decides SPEC-022.
- The DM and the agent keep the world. A spectator opens on nobody. A player
  opens on their first party member by id.
- A redial naming `log` opens that log while it may still be watched. One
  naming none opens the default from its first entry.
- Two steps stay: no auto-revocation.
- Requirement ids come from the dispenser alone, after sign-off. No row's
  text is edited except by the register's withdrawal form.

**The import boundary.** It is `.go-arch-lint.yml`:
- `internal/harness` may import only `contract`, `engine`, `coder/websocket`,
  vendor packages (protobuf among them) and the standard library.
- `internal/mcp` adds `harness` and the MCP SDK.
- Tests are included.
- `go-arch-lint` is clean after every task.

**Tiers.**
- Tier 1 runs `internal/harness` at pre-commit.
- Pre-push runs `internal/mcp` (tier 2) and `cmd/vtt` (tier 3). Give a push
  600000 ms, send its whole output to a file, and read the load first.

**`gocyclo` refuses a function above 30** (`min-complexity: 30`). Extract a helper rather than grow `RunSoak`,
`RunScenario` or `runReconnectStep` past it. `settleTrailingDenials` is the
precedent, and its doc says why.

**The comment gate (SPEC-010, `tools/check-comments.py`).**
- **Nearly every touched file sits at or within 0.1 of its row** (measured,
  `--report`): among them `engine.go` 30.1, `soak.go` 42.7, `client.go` 48.2
  of 48.3, `mcp/server.go` 48.1, `read_tools.go` 30.8, `state_dump.go` 45.1,
  `client_run.go`, `events_tail.go`, `app.ts`, `spectator.ts` and most of the
  touched tests. An added comment line needs added code lines to pay for it.
- **At 0.0:** `qa_log_seat_test.go` and `qa_world_seat_test.go`. Their edits
  add no comment line.
- **Long blocks.** A block over six lines that a change adds a line to is
  refused. A block that becomes false is rewritten whole, to six lines or
  fewer, and its history goes to the report (D18).
- **New files.** Each new test file gets its row by `--write-ledger`.
- **Running it.** Run `python3 tools/check-comments.py main` and the same
  against HEAD, after staging.

**Mutation.**
- **`internal/mcp` is gated.** Its three adjudications are keyed to code this
  plan leaves alone: `if limit > maxEventsSinceLimit {`,
  `if backoff < redialMaxBackoff {` and `if backoff > redialMaxBackoff {`.
- **New survivors** are killed by a test where a test can see them, and only
  otherwise adjudicated by the code they sit on (SPEC-020).
- **`internal/harness` and `cmd/vtt` are outside the gate**
  (`tools/check-mutation.py`'s `PACKAGES`, `tools/mutation-scope.md`). Their
  new checks get no fault injection but this plan's breaks, so every new check
  has one.

**Coverage floors** (`tools/coverage-thresholds.txt`): `internal/harness`
89.5, `internal/mcp` 87.5, `cmd/vtt` 85.0.

**A log's own entries carry no `EventId`.**
- An opening's, a resume's and a projection's entries have none. A forwarded
  entry shares the world's.
- Compare a log seat's entries with `proto.Equal`.
- Every QA brief says so.

**Scheduling.**
- A new test that depends on goroutine order is stressed with
  `go test -race -count=20 -cpu 1,2 -run '<names>'`, at a load read first with
  `os.getloadavg()`.
- The soak and engine tests run in `testing/synctest` bubbles. Keep new ones
  there where the existing file does.

**The bundle.**
- Comment-only edits under `client/src` leave `cmd/vtt/webdist` byte-identical
  (measured on the probe).
- Check that with `git status --short cmd/vtt/webdist`, which prints nothing.
- Do not rebuild it otherwise.

**Mechanics.**
- Commit the ticket and this plan in Task 0: `review-record` refuses
  untracked files.
- A new test goes after a closing brace.
- Stage and commit in separate calls, and check what landed.
- Undo a break by the inverse edit and check it with `grep` or `cmp` against
  `git show HEAD:<file>`, never with `git checkout --`.
- Hash the tree before and after each QA agent, and never stash while a
  reviewer reads.

## Rule 9: how MapTool does this

Checked in `~/dev/RPTool/maptool/src/main/java`. The ticket's answer holds,
with one correction (G10).

- **A join is the whole campaign.**
  - `MapToolServer.addRemoteConnection` calls `installConnection`, then sends
    the joiner one `SetCampaignMsg` holding `campaign.toDto()`.
  - `ClientMessageHandler.handle(SetCampaignMsg)` calls `MapTool.setCampaign`.
  - That calls `client.setCampaign`, clears the zone renderers, the initiative
    panel and the token tree, and chooses a map to show.
  - Whatever the client held is replaced.
- **There is no reconnect at all.** This is the correction.
  - On an unexpected disconnect from a remote server,
    `MapToolClient.onDisconnect` calls `MapTool.disconnect` and
    `MapTool.stopServer`. It shows "msg.error.server.disconnected", clears the
    campaign data, and starts a personal server with a blank campaign.
  - Joining again is the user's act, a fresh join sent `SetCampaignMsg`.
  - So there is no cursor, no log to name, and no client-side resume.

**What we take:**
- Replace on arrival. Each Go reader discards what it holds at a
  `ViewpointChanged` naming another log, as the browser client does (VTT-394).
- A snapshot taker that cannot get a whole answer gets none. The dump refuses
  at a switch rather than splicing (D13). MapTool's client is never handed
  half a campaign.

**What we leave:**
- The distribution model, which hands every client the whole campaign. That
  is CLAUDE.md rule 9's refusal.
- The manual rejoin. `vtt mcp` redials by itself, and the scenario engine's
  reconnect step is a scripted redial. Both name the log and the cursor, which
  MapTool has no equivalent of, because it sends every client everything.

## Decisions

1. **D1. The switch travels in the same channel as the entries.**

   `Conn.Events()` and `(*Client).Events()` return `<-chan harness.Delivery`:

   ```go
   // Delivery is one thing a connection was sent, in order: an entry or a switch.
   type Delivery struct {
   	Envelope  *vttv1.Envelope
   	Viewpoint *vttv1.ViewpointChanged
   }
   ```

   - **The arm.** `readLoop` gains a `ServerFrame_ViewpointChanged` arm. It
     hands over a `Delivery` with `Viewpoint` set through `deliverEvent`'s
     overflow policy. Nobody (`{"viewpointChanged":{}}`) is delivered too: the
     arm's presence is the switch (SPEC-007).
   - **`WatchableChanged`** stays ignored, as the `default:` arm does today.
     The ticket leaves it out.
   - **Forced, on two counts.**
     - Item 1 says "in order with the envelopes".
     - A switch to nobody is followed by no entry. On the probe it was the
       opening `{"viewpointChanged":{}}`, with nothing after it until a grant.
     - So a second list of switches, stamped with positions, is never read by
       a consumer that is waiting on entries. The `vtt mcp` pump would go on
       answering the old log, and the dump would wait out its deadline.
   - **Preferences:**
     - the type's name and its two-pointer shape;
     - keeping the method's name `Events`, so that `Conn`'s long doc block
       stays true untouched.
   - **Cost (measured):**
     - the four fake `Conn`s and their direct pushes: six in `engine_test.go`,
       and the preload in `soak_test.go`;
     - nine reads in `client_test.go`;
     - twelve production consumers: seven in `engine.go`, two in `soak.go`,
       and one each in `mcp/server.go`, `events_tail.go` and `state_dump.go`;
     - twelve `drainToHead` calls in `state_dump_head_test.go`;
     - four `tailUntilDone` calls in `events_tail_truncation_test.go`;
     - `drainToCatchUpHead` in `scenario_goldens_test.go`.

2. **D2. `Dial(ctx, wsURL, token string, after int64, log string)`.**
   - A non-empty `log` is set in the query. An empty one sends no `log` key at
     all.
   - **Forced:**
     - by item 2's "a dial naming no log carries no log";
     - by the browser's model, which names no `log` for the world or for
       nobody (VTT-397).
   - One function rather than a second `DialLog` is a preference.
   - `url.Values.Get` cannot tell a missing key from an empty one. Item 2's
     test reads `q["log"]`.

3. **D3. `type Dialer func(name string, after int64, log string) (Conn, error)`.**
   - **Forced.** `dialerFor` builds the URL from what it is handed. A `Conn`
     method could tell the engine the log, but never the Dialer, so it only
     moves the question.
   - **Cost (measured):**
     - 33 literals and methods in seven `internal/harness` test files;
     - four `fixedDialer(world)(name, after)` calls;
     - the call in `cmd/vtt/dialer_test.go`'s
       `TestDialerForRejectsUnknownParticipant`;
     - `dialerFor`.

     Each takes `_ string` or passes it through, and adds no comment line.
   - `buildSoakDialer` and `buildDialer` return `dialerFor`'s result and do
     not change.

4. **D4. What a consumer does with a switch.**
   - **Task 2: it skips the switch,** so behaviour is today's. Today the
     client drops the frame, so a skip is the same behaviour, and every
     existing test stays green.
   - **Task 3 onward: it records the switch** for the seat it came from.
   - **Only an envelope is an event.**
     - `drainPreExisting` counts envelopes only.
     - `drainAllForSilence` and `grewSince` count only envelopes as a leak.
   - **Forced.** A log seat's fresh connection opens with a
     `ViewpointChanged` (measured). Counted as an event, it would refuse every
     scenario whose first participant is a log seat as "not a fresh campaign".
     A switch is a frame, never an `Envelope` (SPEC-007).
   - **Recorded where it is read.** `drainPreExisting` reads participant 0's
     opening frames. From Task 3 it records that seat's switches; it does not
     drop them.

5. **D5. The engine holds one log per log seat.**
   - **What a seat keeps:**
     - its envelopes, in `history` as today;
     - its switches, each with an index into `history` and a log id.
   - **What it holds.** Its held log is the last switch's id. Before any
     switch it holds none ("never told").
   - **Its current run** starts at the later of:
     - the last switch naming another log;
     - the last sequence-1 entry after that switch.
   - **Forced:**
     - by the ticket's first rule;
     - by SPEC-007, where a switch is followed by the new log from its first
       entry;
     - for the sequence-1 half, by the existing fakes. They model a switch as
       a restart at 1, with no frame: `qaLSSwitchedSeat` and `logStepSend`.
       This keeps them meaningful without rewriting them all.
   - In production the two halves coincide. On the probe, every switch was
     followed by entry 1.
   - "Never told" is reached by the fakes alone. The gateway tells every log
     seat (VTT-368).

6. **D6. After a switch naming another log, the next entry must be 1.** An
   entry whose sequence is not 1 breaks the seat's log, in `brokenLogs` and in
   `soakHistories.brokenLogs`.
   - **Forced** by the debt recipe, measured on the probe. Under it the player
     was sent `{"viewpointChanged":{"actorId":"act-bram"}}` and then `act-ash`'s
     entry 5. Without it, `act-bram`'s entries 1-5 followed.
   - Also forced by VTT-342.

7. **D7. A log seat's reconnect.** Extracted from `runReconnectStep` into its
   own function (`gocyclo` 25):
   - **The redial.** It calls `dial(name, AfterSequence, held)` when the seat
     holds an actor's log. When it holds nobody, or was never told, the redial
     names no log.
   - **What it expects.** For an actor's log, the current run's entries with
     sequence above `AfterSequence`. Otherwise, the whole current run: with no
     `log` the gateway ignores the cursor (VTT-349). On the probe,
     `after=3, log=""` replayed `act-ash` from 1.
   - **The redial's own switch.** The redial's `ViewpointChanged` must name the
     held log when that is an actor's. When the seat held an actor's log, a
     redial told another log fails the step. A redial told the held log is not
     a switch.
   - **The comparison** is entry for entry with `proto.Equal`. World seats keep
     `EventId` and `Sequence` (VTT-382), because a world event's id is unique.
     - **Forced** by SPEC-021, which gives a log's own entries no `EventId`.
     - **Forced** by the measured identical openings (G3).
   - **The history afterwards** is what the seat held up to the cursor, then
     what the redial sent.
   - **Its doc.** `runReconnectStep`'s "Expect a false failure" sentence goes.
   - **Tests that change, one of them renamed:**
     - `TestACharacterSeatsReconnectIsComparedWithItsCurrentLog`: its replay
       entries gain the live payload;
     - `TestQALSReconnectComparesByEventIDAndSequenceOnly`: renamed
       `TestQALSReconnectComparesWholeEntries`, its reworded payloads now
       expected to fail, and its row's evidence moved.

8. **D8. A command step whose accepted result names no sequence is not failed
   for want of a world seat observing an event.**
   - **What it does.** It waits for no world seat, and drains the log seats
     into their histories with `drainQuietInto`.
   - **No absence claim.** VTT-184 holds that `set_viewpoint` appends nothing,
     and the gateway's tests prove it. A stray world event would still fail
     the next accepted step, at the world seat that reads it.
   - **Forced** by item 3 (G1).

9. **D9. Probes fold participant 0's current run.**
   - **Forced** by D5: a log seat's history may hold two logs.
   - Participant 0 is the DM in every committed scenario, so no probe changes.
   - The change keeps `RunScenario` at or under 30, by a helper.

10. **D10. The scenario, `scenarios/a-player-switches-and-reconnects.json`.**
    - **Participants:** `dm` (dm) and `player` (player).
    - **Steps:**
      1. `startSession`;
      2. `addActor` `act-ash`, `ACTOR_KIND_PARTY_MEMBER`;
      3. `addActor` `act-bram`, `ACTOR_KIND_PARTY_MEMBER`;
      4. `grantActorControl` of `act-ash`, then, as a step of its own, of
         `act-bram`, each to `{{id:player}}` with kind
         `ACTOR_KIND_PARTY_MEMBER`. The player moves from nobody onto
         `act-ash` and stays there;
      5. the player's `setViewpoint` `act-bram`;
      6. the DM's `addNarration`;
      7. the player's reconnect at `afterSequence` 0;
      8. a second `addNarration`;
      9. the player's reconnect at `afterSequence` 4.
    - **Probes:** `sessionCount` (open 1, total 1) and `actorExists`
      `act-bram`.
    - **What the probe measured.** `act-bram`'s log after the switch holds:
      - entries 1-3: opening entries with no event id;
      - entry 4: the grant;
      - entry 5: the first narration.

      So the second narration is 6, and the second reconnect expects 5 and 6.
    - **On today's harness** it fails at the `setViewpoint` step and at the
      first reconnect, both measured. The second reconnect passes today: the
      first redial reopened `act-ash`, and the seat's run became `act-ash`'s.
    - **Under the recipe** with the new harness, it fails at the first
      narration (D6) and at the first reconnect (D7).
    - `TestScenarioLibraryRunsSelfContained` runs it. It needs no golden: the
      corpus is a subset.

11. **D11. The soak checks every log seat at each checkpoint.**
    - **When.** After the observer's comparison, for each log seat.
    - **What it does.**
      - Dial naming the held log, or no log for nobody or never told.
      - Drain to the highest sequence of its current run.
      - Resume across an overflow with the same log.
      - Compare `Fold` of the current run with `Fold` of what the fresh dial
        sent (`statesEqual`).
    - **When it fails.** It writes a FAIL line carrying the checkpoint's number
      (VTT-383):
      - when the two folds differ;
      - when the fresh dial's `ViewpointChanged` names another log.
    - **A seat whose stream has broken its log** is not compared at that
      checkpoint or any later one.
    - **Forced:**
      - The comparison, by item 4.
      - The `log`, by `drainFreshCatchUp`'s overflow resume: a resume with no
        `log` has its cursor ignored (VTT-349), and would fold the log twice.
      - The order, by `TestRunSoakKeystoneHoldsAcrossAnOverflowDisconnect`. Its
        scripted overflow fires on the first dial after the four participants'.
      - The skip, by `TestASoakCheckpointFailsACharacterStreamThatSkips` and
        `qaLSSoakBreak` (G9). A broken history can also fail `Fold`, which
        `RunSoak` reports as a framework error.
    - **The code.** `drainFreshCatchUp` is generalised to a name and a log. The
      per-seat half is a helper (`RunSoak` is at 28).
    - **The cost** is about two dials per checkpoint. The seed-1 keystone
      measured 13.7 s before the change.

12. **D12. `vtt mcp` holds one log.**
    - **What it keeps.** `Server` keeps `log`: unset for the world or never
      told, "" for nobody, an actor's id otherwise.
    - **At a `ViewpointChanged` naming another log,** the first included:
      - empty `history`;
      - set `lastSeq` to 0;
      - set `log`.
    - **At one naming the held log:** nothing.
    - **`redial`** calls `harnessDial(ctx, url, token, lastSeq, log)`. It names
      `log` when it is an actor's id.
    - **The tool descriptions** gain a sentence each: `afterSequence` counts
      within the log the answer names, and starts again from 0 when that log
      changes.
    - **Forced:**
      - by the ticket's rules;
      - by G4's case for the keep half. A redial naming the held log is sent a
        `ViewpointChanged` naming it (measured).
    - `recordEvent`'s dedupe stays.

13. **D13. The dump at a switch.** This is Q2's recommended answer.
    - **What the dump does.** It remembers the log the first
      `ViewpointChanged` names.
      - At one naming another log before it reaches its head, it refuses. It
        exits non-zero with a sentinel error naming both logs, and prints
        nothing.
      - At one after the head, it stops reading and prints the fold of the log
        it held.
    - **Forced:**
      - by SPEC-007's snapshot sentence;
      - by `dumpQuietWindow`'s doc: a stop for the new log could only be the
        quiet-window guess;
      - by the dump's own rule, "FAIL, never print".
    - **A move at the opening** of a seat whose head is above its cursor is a
      switch before the head: the second `ViewpointChanged` of VTT-387, after
      a head that named the log the seat opened on. A seat opened on nobody is
      told head 0 and has reached it, so a move then is after the head (VTT-425).

14. **D14. The key that names the log.** This is Q3's recommended answer.
    - **The key** is `"log"`: the actor's id for a character log, `""` for
      nobody, and absent for the world, as `log` is absent from a world
      connection's handshake.
    - **It applies to** `get_state`, `get_events_since` and the dump.
    - **Forced, if accepted.** All nine golden streams are the DM's, measured,
      so neither the corpus nor its two byte-for-byte mirrors move.
    - **Q3's alternative** puts the key on every dump, `null` for the world.
      That moves:
      - all nine golden `state.json` files;
      - `dumpJSON` in `internal/harness/fold_golden_test.go`;
      - `foldToDumpJSON` in `client/src/fold.ts`.

15. **D15. Requirements.** This is Q4's recommended answer.
    - New rows per reader (VTT-409 to VTT-426 below).
    - VTT-394 to VTT-398 stay the browser's, and SPEC-022 says so.
    - VTT-360 is withdrawn and succeeded by VTT-412 and VTT-413.
    - **Forced.** VTT-398's "the highest sequence the client has been sent …
      since it last discarded" is false of the scenario engine, whose cursor
      is the scenario's. VTT-360 is false as worded (G2). A row that holds for
      one client and not another cannot share one evidence cell.

16. **D16. The records (Q1: yes).** Task 1, before any code task, creates
    SPEC-022 and moves into it what a client holds and offers:
    - SPEC-007's section "What a client holds, and what it offers", with its
      client rows (VTT-394 to VTT-398, VTT-402 to VTT-408) and their Status
      sentences;
    - VTT-393's browser half, that the window shows no sequence. The wire half
      of SPEC-007's "A sequence is for correlating and tracing, not for
      showing" stays in "The ordering contract";
    - the pointers at what moves: SPEC-007's own, under "What the connection
      opens with"; SPEC-015's two, in "What this record does not decide" and
      its consequence; the header of `client/src/wire.ts`; and the three perch
      comments.

    SPEC-011's consequence keeps SPEC-007, since the snapshot sentence it names
    stays in "What the connection opens with". SPEC-022 says that VTT-394 to
    VTT-398 are the browser's. The specification step then adds the Go readers
    to SPEC-022.
    - **Forced** by SPEC-015's own "What this record does not decide", which
      says what a client does and offers is SPEC-007's. The comments' SPEC-015
      is the wrong record.
    - **The order is forced by D18:** a pointer a code task writes must resolve
      in the commit that writes it.

17. **D17. One real-gateway test for each reader a fake models.**
    - **The engine:** D10's scenario, in `cmd/vtt`.
    - **The soak:** the seed-1 keystone, unchanged counts.
    - **`vtt mcp`:** a player seated in `vtt mcp` that switches by its own
      `set_viewpoint` tool (Task 5).
    - **The dump:** a player's dump that names its log (Task 6).
    - **Forced** by the debt this closes. Every fake modelled a switch as a
      restart at 1, and every test passed while the harness was blind. The
      fakes here are written by the same hands as the readers.

18. **D18. Comments that become false are rewritten.** Each is rewritten
    whole, to six lines or fewer: a warning, or a pointer to SPEC-022 and the
    rows. SPEC-022 exists from Task 1, and its paragraphs on the Go readers
    land in Task 9, since the specification step comes last. Its history goes
    to the report. The task that falsifies
    each one rewrites it:
    - **Task 2:** `Client.Events`'s doc and `readLoop`'s doc.
    - **Task 3:** `Dialer`'s, `RunScenario`'s (VTT-360 there),
      `runReconnectStep`'s, `currentRun`'s, `logBreak`'s, and
      `drainPreExisting`'s eleven-line block.
    - **Task 4:** `runSoakCheckpoint`'s.
    - **Task 5:**
      - `Server.history`'s ten-line block, and `lastSeq`'s;
      - `pump`'s, `redial`'s, `recordEvent`'s eight-line block, and `Run`'s;
      - `handleGetState`'s thirteen-line block, and `marshalStateWithHead`'s.
    - **Task 6:** the twelve-line block that `newStateDumpCmd`'s doc ("one
      field beyond the folded state itself") and `dumpAfter`'s form, which
      moves onto the function or shrinks to six lines; `writeDump`'s; and
      `drainToHead`'s long block.

    Shares are expected to fall, and each row is lowered by `--write-ledger` in
    its task.

19. **D19. Task order follows what each task depends on.**
    - Task 1, the records, comes first, so every pointer the code tasks write
      resolves (D16, D18).
    - Task 2, the surface, comes before every other code task.
    - Task 3 comes before Task 4: the soak shares the seat's switches and
      `logBreak`.
    - Tasks 5 and 6 need only Task 2. Placing them after Task 4 is a
      preference.
    - Task 7 runs on the last code commit, which is Task 6's.

20. **D20. Phase 4a.**
    - One QA agent for each of Tasks 2 to 6. Each delivers behaviour a rule
      describes, which is the installed process's test for QA.
    - Task 1 moves a record and changes comments, and gets no QA agent.

## Requirements

Ids are allocated after sign-off by the dispenser (`requirement-id`), and each
row starts `**OPEN — no test yet**`. Each rule below carries:
- what it says;
- the observation that goes red when it is broken;
- the break that shows it.

**VTT-409** (Task 2). "A `harness.Client` hands its reader each `ViewpointChanged`
it is sent, nobody's included, in the order the connection was sent it among
the entries."
- *Observation:* item 1, with nobody added. A fake sends `ViewpointChanged`
  a1, a1#1, a1#2, `ViewpointChanged` a2, a2#1 and `{"viewpointChanged":{}}`,
  and the reader gets six deliveries in that order. Red today: both frames are
  skipped.
- *Breaks:* B1, B2, B3.

**VTT-410** (Task 2). "`harness.Dial` given a log names it in the handshake's
`log` beside `after`, and given none sends no `log`."
- *Observation:* item 2.
- *Breaks:* B4, B5.

**VTT-411** (Tasks 3 and 4). "A character seat whose first entry after a
`ViewpointChanged` naming another log than the one it holds is not of sequence
1 fails the scenario step or the soak checkpoint that drained it."
- *Observation:* a unit test, D10's scenario under the debt recipe, and Task
  4's old-log test. This row closes the open debt.
- *Breaks:* C1, the recipe, and S7.

**VTT-412** (Task 3; with VTT-413, succeeds VTT-360). "A character seat's reconnect
step fails when the seat held an actor's log and the redial is told another
log."
- *Observation:* `TestAReconnectToldAnotherLogFails`.
- *Break:* C3.

**VTT-413** (Task 3; with VTT-412, succeeds VTT-360). "A character seat's reconnect
step fails when its replay differs, compared whole, from what the seat was sent
live in its current run: the entries after the step's cursor when it held an
actor's log, and the whole run otherwise."
- *The current run* is D5's: from the last switch naming another log, or from
  the last sequence-1 entry after it. That second half is how the eight tests
  whose citations move from VTT-360, whose fakes send no `ViewpointChanged`,
  hold this row.
- *Observation:* `TestAReconnectOfAHeldLogIsComparedAfterTheCursor`,
  `TestAReconnectComparesWholeEntries`,
  `TestAReconnectOfASeatOnNobodyExpectsNothing`, those eight tests, and D10's
  two reconnects.
- *Breaks:* C2, C4, C5.

**VTT-414** (Task 3). "A character seat's reconnect step redials naming in `log`
the actor whose log the seat holds, and names no `log` when it holds nobody."
- *Observation:* a Dialer that records what it was handed, and D10's
  reconnects on the real gateway.
- *Breaks:* C6, C7.

**VTT-415** (Task 3). "A scenario's accepted command step whose result names no
sequence is not failed for want of a world seat observing an event."
- *Observation:* D10's `setViewpoint` step, and a unit test.
- *Break:* C8.

**VTT-416** (Task 4). "A soak checkpoint fails when, for a character seat whose
stream has not broken its log, the fold of the log it holds differs from the
fold of a fresh dial naming that log."
- *Observation:* item 4, G13's test and the overflow test.
- *Breaks:* S1, S2, S3, S5, S6.

**VTT-417** (Task 4). "A soak checkpoint fails when the fresh dial it makes for a
character seat, naming the log the seat holds, is told another log."
- *Observation:* `TestASoakCheckpointFailsAFreshDialToldAnotherLog`.
- *Break:* S4.

**VTT-418** (Task 5). "`vtt mcp`, at a `ViewpointChanged` naming another log than
the one it holds, discards the entries it holds, so that `get_state` folds and
`get_events_since` pages the entries of the log that follows alone."
- *Observation:* item 5.
- *Break:* M1.

**VTT-419** (Task 5; G4). "`vtt mcp`, at a `ViewpointChanged` naming the log it
holds, keeps the entries it holds."
- *Observation:* the redial test that keeps the fold.
- *Break:* M2.

**VTT-420** (Task 5). "`vtt mcp`'s redial names in `log` the actor whose log it
holds, and names no `log` when it holds the world or nobody."
- *Observation:* item 6.
- *Break:* M3.

**VTT-421** (Task 5). "`vtt mcp`'s redial's `after` is the highest sequence it
has been sent of the log it holds since it last discarded."
- *Observation:* item 6's `after` half.
- *Break:* M4.

**VTT-422** (Task 5). "`get_state` and `get_events_since` name the log their
answer is of in `log`: the actor's id for a character log, empty for nobody,
and no `log` for the world."
- *Observation:* item 5's naming half, and the agent's answers.
- *Breaks:* M5, M6.

**VTT-423** (Task 6). "`vtt state dump` of a seat moved to another log before
its catch-up reaches its head prints no state and exits non-zero."
- *Observation:* item 7, at both lengths.
- *Break:* F1.

**VTT-424** (Task 6). "The refusal of a `vtt state dump` whose seat moved before
its head names the actor whose log it held and the one it was moved to, or
nobody."
- *Observation:* the refusal test's sentinel and id assertions, a move to
  nobody among its cases.
- *Break:* F2.

**VTT-425** (Task 6). "`vtt state dump` stops reading at a `ViewpointChanged`
naming another log after it has reached its head, and prints the fold of the
log it held."
- *Observation:* Task 6's after-head test.
- *Break:* F3.

VTT-423 and VTT-425 together are the ticket's "a dump never prints the fold of
more than one log"; VTT-424 says what the refusal names.

**VTT-426** (Task 6). "`vtt state dump` names the log its fold is of in `log`, as
`get_state` does."
- *Observation:* item 7's second half, and D17's real-gateway dump.
- *Breaks:* F4, F5.

**VTT-360 is withdrawn**, in the register's form:
- *The row becomes:* "WITHDRAWN 2026-10-21: A character seat's reconnect step
  fails when its replay differs from the seat's live history since its last
  entry of sequence 1. False as worded once a character seat is told of its
  switches and its redial names the log it holds: the seat's run starts at its
  last switch, and a redial naming an actor's log is replayed after the step's
  cursor. Succeeded by VTT-412 and VTT-413."
- *Its evidence cell becomes:* `**READING — verify-ticket check 5 of
  2026-10-21; withdrawn, held by nothing**`.
- *Its tests* cite VTT-413 on their citation lines. Seven of them are in
  `qa_log_seat_test.go`, and one is in `engine_test.go`.
- *Three comments name VTT-360 and are re-pointed to VTT-413:*
  - `RunScenario`'s doc in `engine.go`;
  - `observeOnAll`'s drain comment in `engine.go`;
  - `TestObserveOnAllNeverRequiresALogSeatToSeeAnything`'s doc in
    `batch_truncation_internal_test.go`.

  `runReconnectStep`'s doc is rewritten anyway (D7).

**The sort refused seven candidates:**
- "A switch is not counted as an event." That is how (D4).
- "A run also starts at a sequence-1 entry when no switch is known." It keeps
  the fakes; it is not a product rule (D5).
- "Probes fold participant 0's current run." No committed scenario reaches it
  (D9).
- "A seat whose stream broke is not compared again." It is VTT-416's qualifier,
  not a rule of its own (D11).
- "The per-log check runs after the observer's." That is how (D11).
- "`vtt events tail` prints no switch." This is unchanged behaviour, outside
  the ticket.
- "`task check` is green", and "no comment points at SPEC-015 for the perch."
  Both are process.

## Tasks

Each code task follows this pattern:

1. Write the tests and see each fail for its stated reason.
2. Implement, and see them pass.
3. Apply each break and see the named test go red, with every other new test
   green. Restore the break by the inverse edit, and check the restore with
   `cmp` or `grep`.
4. Stage, then run the comment gate against main and against HEAD.

Then come the reading review, the review record and the commit. Each task is
its own commit through the review gate.

### Task 0: sign-off and ids

- **Files:**
  - `docs/requirements.md`: VTT-409 to VTT-426 from the dispenser. VTT-360 is
    withdrawn in Task 3, the commit that makes it false, with its tests'
    citations;
  - the ticket and this plan;
  - all committed together.
- **True when:**
  - The owner has signed off D1-D20 and answered Q1 to Q3.
  - The rows are allocated and OPEN, and the plan's labels are replaced by
    their ids.
  - `task check:requirements-chain` is green.

### Task 1: SPEC-022 holds what a client holds, and the client's pointers name it (item 8; item 10's Status half; D16)

- **Files:**
  - **New:** `docs/specifications/022-a-client-holds-one-log.md`, holding what
    D16 moves, under the `specification` skill's five headings.
  - `docs/specifications/007-the-wire-contract.md`, without what moves, and
    with its own pointer at SPEC-022. The open question in its Status is
    closed.
  - `docs/specifications/015-the-seat-and-the-perch.md`: its two pointers.
  - Comments only, at SPEC-022: `client/src/view/spectator.ts`
    (`PerchControl`'s doc and `ViewExtras.perch`'s), `client/src/app.ts`
    (`mayPerch`'s doc) and `client/src/wire.ts` (its header).
- **Break:** **K1.** Put one `SPEC-015` back: item 8's grep prints it.
- **True when:**
  - item 8's grep prints nothing;
  - every `SPEC-007` pointer left under `client/src` is about the wire, which
    the reading review checks one by one;
  - `git status --short cmd/vtt/webdist` prints nothing;
  - the comment gate is clean;
  - `task check:requirements-chain` is green, with SPEC-022 listing its rows
    and SPEC-007 no longer listing them;
  - the reading review has checked that every moved sentence reads as it did,
    and that nothing is left in both records.
- **Phase 4a:** none. Nothing here has behaviour to derive.

### Task 2: the client hands its reader each switch, and a dial names a log (items 1, 2; VTT-409, VTT-410; D1, D2, D4)

- **Files:**
  - `internal/harness/client.go`:
    - `Delivery`;
    - `Events()` of `Delivery`;
    - the `ViewpointChanged` arm;
    - `Dial`'s `log`;
    - two docs rewritten (D18).
  - `internal/harness/engine.go` and `soak.go`: each consumer skips a switch.
    No comment line is added.
  - The test fakes and reads:
    - `internal/harness/engine_test.go`, `soak_test.go`,
      `soak_drain_internal_test.go`, `qa_log_seat_test.go`,
      `qa_world_seat_test.go` and `client_test.go`;
    - every `harness.Dial` caller in tests.
  - `internal/mcp/server.go`: the pump skips a switch, and both dials pass
    "". The stubs in `server_internal_test.go`, `redial_internal_test.go` and
    `record_event_internal_test.go` take the new signature.
  - `cmd/vtt/client_run.go` (`dialerFor` passes ""), `events_tail.go` and
    `state_dump.go`. These skip a switch: the tail's output is unchanged.
  - `cmd/vtt/state_dump_head_test.go`, `events_tail_truncation_test.go`,
    `scenario_goldens_test.go`, `mcp_e2e_test.go` and `client_e2e_test.go`.
  - **New:** `internal/harness/client_switch_test.go`, with:
    - `TestAClientHandsItsReaderEachSwitchInOrderWithTheEntries`;
    - `TestADialNamesTheLogItIsGivenAndNoneWhenItIsGivenNone`.

    It reuses `client_test.go`'s `newFakeServer`, `sendFrame` and `lastQuery`,
    reading `lastQuery` under `fs.mu`. It carries citation lines only.
  - `tools/comment-ceilings.txt` and `docs/requirements.md` (VTT-409 and VTT-410's
    evidence).
- **Breaks:**
  - **B1.** Remove the arm: item 1's test goes red.
  - **B2.** Hold a switch until after the next entry: item 1's order assertion
    goes red.
  - **B3.** Deliver a switch only when `ActorId != ""`: the nobody step goes
    red.
  - **B4.** `Dial` ignores `log`: item 2 goes red.
  - **B5.** `Dial` always sets `log`: item 2's no-log half goes red.
- **True when:**
  - `go test -count=1 ./internal/harness/ ./internal/mcp/ ./cmd/vtt/` is
    green. Every existing test passes unchanged in what it asserts: a switch
    is skipped, as the client used to drop it. If a `cmd/vtt` deadline test
    fails under load, rerun it alone and read the load.
  - `go doc ./internal/harness` shows `Delivery` and the new `Dial`.
  - `golangci-lint run ./internal/harness/ ./internal/mcp/ ./cmd/vtt/` finds
    nothing, and `go-arch-lint` is clean.
- **Phase 4a:** one QA agent. Its file is
  `internal/harness/qa_client_switch_test.go`, prefix `qaCS`.

### Task 3: the scenario engine holds one log per seat (item 3; VTT-411 to VTT-415; D3, D5-D10; closes the debt)

- **Files:**
  - `internal/harness/engine.go`:
    - `Dialer` (D3);
    - the seats' switches, held log and run (D5);
    - `logBreak` and `brokenLogs` (D6);
    - the extracted log-seat reconnect (D7);
    - `runCommandStep`'s no-sequence branch (D8);
    - the probe fold (D9);
    - `drainPreExisting` recording switches (D4);
    - six docs (D18).
  - Every `Dialer` literal and method (D3). `engine_test.go` gains a
    `trySwitch` beside `trySend`.
  - The tests that change, one of them renamed:
    - `TestACharacterSeatsReconnectIsComparedWithItsCurrentLog`'s replay
      payloads;
    - `TestQALSReconnectComparesByEventIDAndSequenceOnly`, renamed and
      inverted (D7);
    - the citation lines of VTT-360's tests, and the three comments that name
      it, one of them in `internal/harness/batch_truncation_internal_test.go`.
  - `cmd/vtt/client_run.go`: `dialerFor` passes `log` to `Dial`. And
    `cmd/vtt/dialer_test.go`'s call of the `Dialer`.
  - **New:** `internal/harness/one_log_test.go`, with:
    - `TestASeatThatAnnouncesASwitchAndKeepsTheOldLogFailsTheStep` (VTT-411);
    - `TestAReconnectOfAHeldLogIsComparedAfterTheCursor` (VTT-413);
    - `TestAReconnectToldAnotherLogFails` (VTT-412);
    - `TestAReconnectComparesWholeEntries` (VTT-413): an entry with no `EventId`
      that differs only in its payload;
    - `TestAReconnectOfASeatOnNobodyExpectsNothing` (VTT-413);
    - `TestAReconnectNamesTheLogTheSeatHolds` (VTT-414): the Dialer records
      `(name, after, log)`;
    - `TestACommandThatAppendsNothingPasses` (VTT-415): result ok, sequence 0,
      passing well inside `observeTimeout`.
  - **New:** `scenarios/a-player-switches-and-reconnects.json` (D10).
  - `docs/verification-debt.md`. The open entry moves out as a dated entry
    under its original title. It keeps its recipe and labels, and gains
    "**Closed by**" D10's scenario and VTT-411's test, each red under the recipe.
  - `tools/comment-ceilings.txt` and `docs/requirements.md` (VTT-411 to VTT-415 and
    VTT-360).
- **Breaks:**
  - **C1.** `logBreak` ignores switches: VTT-411's unit test goes red.
  - **C2.** Compare by `EventId` and `Sequence`:
    `TestAReconnectComparesWholeEntries` goes red.
  - **C3.** Do not check the redial's `ViewpointChanged`:
    `TestAReconnectToldAnotherLogFails` goes red.
  - **C4.** Expect a named log from its first entry:
    `TestAReconnectOfAHeldLogIsComparedAfterTheCursor` goes red, and so does
    D10's second reconnect.
  - **C5.** Run from the last sequence-1 entry alone:
    `TestAReconnectOfASeatOnNobodyExpectsNothing` goes red.
  - **C6.** `dialerFor` drops `log`: D10's reconnects go red, because the
    default `act-ash` opens.
  - **C7.** The engine hands the Dialer "": VTT-414's unit test goes red.
  - **C8.** Wait on world seats for sequence 0: VTT-415's test and D10's
    `setViewpoint` step go red.
  - **The recipe** in `seat.watch`, on the working tree, restored by the
    inverse edit and checked by `cmp` against `git show HEAD:internal/gateway/seat.go`:
    D10's scenario goes red at its first narration and its first reconnect.
    Run it with
    `go test -count=1 -run 'TestScenarioLibraryRunsSelfContained/a-player' ./cmd/vtt/`.
- **True when:**
  - `go test -count=1 ./internal/harness/ ./cmd/vtt/` is green, and every
    committed scenario still passes.
  - `go test -race -count=20 -cpu 1,2` over the new tests is green at a load
    read first.
  - `gocyclo` reports no function above 30 in `internal/harness`.
  - Lint and `go-arch-lint` are clean.
- **Phase 4a:** one QA agent. Its file is
  `internal/harness/qa_one_log_test.go`, prefix `qaOL`.

### Task 4: the soak compares each log seat's fold (item 4; VTT-411's soak half, VTT-416, VTT-417; D11; G9, G13)

- **Files:**
  - `internal/harness/soak.go`:
    - `soakHistories` records switches;
    - `drainFreshCatchUp` takes a name and a log;
    - the per-seat comparison, as a helper;
    - `runSoakCheckpoint`'s doc.
  - **New:** `internal/harness/soak_log_seat_test.go`. Its fake gives each
    player a log of its own, announced by a `ViewpointChanged`. Its knobs:
    - a fresh dial whose entry differs in a payload with no `EventId`;
    - a dial with no `log` that opens another log;
    - an overflow on a log seat's fresh dial;
    - a fresh dial told another log;
    - a live switch followed by the old log's next entry.

    Its tests:
    - `TestASoakCheckpointFailsALogSeatWhoseFreshDialFoldsDifferently`
      (item 4);
    - `TestASoakCheckpointDialsALogSeatNamingTheLogItHolds` (G13);
    - `TestALogSeatsFreshDialThatOverflowsResumesTheSameLog`;
    - `TestASoakCheckpointFailsAFreshDialToldAnotherLog`;
    - `TestASoakCheckpointFailsALogSeatThatKeepsItsOldLogAfterASwitch`
      (VTT-411). It asserts the FAIL line that names the broken log, so that
      a fold difference cannot stand in for it.
  - `tools/comment-ceilings.txt` and `docs/requirements.md` (VTT-411, VTT-416, VTT-417).
- **Breaks:**
  - **S1.** Skip the per-seat comparison: item 4's test goes red.
  - **S2.** The fresh dial names no `log`: G13's test goes red.
  - **S3.** Compare `EventId`s instead of folds: item 4's test goes red.
  - **S4.** Do not check the fresh dial's `ViewpointChanged`: the told-another
    test goes red.
  - **S5.** The resume dials with no `log`: the overflow test goes red.
  - **S6.** Compare a seat whose stream has broken its log at a later
    checkpoint: `TestASoakCheckpointFailsACharacterStreamThatSkips` goes red at
    its "checkpoint 2" assertion, or the run errors.
  - **S7.** `soakHistories` drops switches: the old-log test goes red.
- **True when:**
  - `go test -count=1 ./internal/harness/` is green, the QA soak tests
    included.
  - `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts` passes
    with its counts unchanged. Report its time against 13.7 s.
  - `-race -count=20 -cpu 1,2` over the new tests is green.
  - `gocyclo` reports no function above 30.
- **Phase 4a:** one QA agent. Its file is
  `internal/harness/qa_soak_log_test.go`, prefix `qaSLS`.

### Task 5: `vtt mcp` holds one log (items 5, 6; VTT-418 to VTT-422; D12, D14, D17; G4)

- **Files:**
  - `internal/mcp/server.go`: `log`, the switch, `redial`, and the docs
    (D18).
  - `internal/mcp/read_tools.go`:
    - the `log` key in both answers (D14);
    - the two descriptions;
    - `handleGetState`'s and `marshalStateWithHead`'s docs.

    The clamp line is left as it is.
  - **New:** `internal/mcp/one_log_test.go`. It uses `server_test.go`'s
    `newFakeServer`, `sendFrame`, `startSession`, and `read_tools_test.go`'s
    `callGetState`, `callGetEventsSince` and `seedEvents`. Its tests:
    - `TestASwitchedPlayerIsAnsweredFromTheNewLogAlone` (item 5);
    - `TestARedialNamesTheHeldLogAndItsHighestSequence` (item 6). The first
      connection holds a1 to 3 and then a2 to 2, so the redial's query is
      `log=a2`, `after=2`;
    - `TestARedialToldTheHeldLogKeepsWhatWasHeld` (VTT-419). The query is
      `log=a1`, `after=3`; the redial sends `ViewpointChanged` a1 and then a1
      4; `get_state` folds 1-4;
    - `TestTheAgentsAnswersNameNoLog` (VTT-422).
  - **New:** `cmd/vtt/mcp_player_e2e_test.go`, with
    `TestMCPSeatedAsAPlayerFollowsItsSetViewpoint` (D17). It runs against a
    real gateway:
    1. the DM grants `act-ash` and `act-bram`;
    2. `vtt mcp`, holding the player's token, calls its `set_viewpoint` tool
       naming `act-bram`;
    3. `get_state` names `act-bram` and folds `act-bram`'s grant, not
       `act-ash`'s;
    4. `get_events_since` from 0 answers `act-bram`'s entries alone.

    Its deadlines are the file's existing ones.
  - `tools/comment-ceilings.txt` and `docs/requirements.md` (VTT-418 to VTT-422).
- **Breaks:**
  - **M1.** No discard at a switch: item 5 goes red.
  - **M2.** Discard at every `ViewpointChanged`: VTT-419's test goes red.
  - **M3.** `redial` names no `log`: item 6 goes red.
  - **M4.** `lastSeq` is kept across a switch: item 6's `after` half goes red.
  - **M5.** No `log` key: item 5's naming half goes red.
  - **M6.** The world gets a `log` key: the agent's test goes red.
  - **M7.** The real-gateway test, against Task 4's commit's `server.go`: it
    goes red.
- **True when:**
  - `go test -count=1 -p 1 ./internal/mcp/ ./cmd/vtt/` is green, with
    `TestMCPSpecSevenExitCriteria`'s `reflect.DeepEqual` unchanged. The
    agent's answers carry no `log` key and nor does its dump (D14).
  - `-race -count=20 -cpu 1,2` over the new `internal/mcp` tests is green.
  - The three `internal/mcp` adjudications' code lines are untouched.
- **Phase 4a:** one QA agent. Its file is `internal/mcp/qa_one_log_test.go`,
  prefix `qaMOL`.

### Task 6: `vtt state dump` folds one log (item 7; VTT-423 to VTT-426; D13, D14, D17)

- **Files:**
  - `cmd/vtt/state_dump.go`:
    - `drainToHead` tracks the first `ViewpointChanged` and stops at a switch
      (D13);
    - a sentinel beside `errCatchUpDeadline`;
    - `writeDump`'s `log` (D14);
    - three docs (D18).
  - **New:** `cmd/vtt/state_dump_switch_test.go`. Its fake gateway is shaped
    like `truncatingGateway`. Its tests:
    - `TestADumpSwitchedBeforeItsHeadRefusesAndNamesBothLogs`. It runs at
      both lengths, the new log shorter than the head and as long, and for a
      move to nobody. It asserts:
      - nothing on stdout;
      - `errors.Is` the sentinel;
      - both ids in the error;
      - a return within 5 s, where today's code takes `dumpCatchUpTimeout`.
    - `TestADumpSwitchedAfterItsHeadPrintsTheLogItHeld` (VTT-425).
    - `TestADumpNamesTheLogItIsTheFoldOf`. A player's dump carries
      `"log":"a1"`, one on nobody `"log":""`, and a world dump carries exactly
      today's keys.
    - `TestAPlayersDumpNamesItsLogAgainstARealGateway` (D17), through
      `bootSelfContained`.
  - `tools/comment-ceilings.txt` and `docs/requirements.md` (VTT-423 to VTT-426).
- **Breaks:**
  - **F1.** No stop at a switch: both lengths go red, one printing a fold of
    two logs and one waiting out the 5 s bound.
  - **F2.** A refusal that does not name the switch: the sentinel and id
    assertions go red.
  - **F3.** Read past a switch after the head: VTT-425's test goes red.
  - **F4.** No `log` key: the player half goes red.
  - **F5.** A `log` key on the world: the world half goes red.
- **True when:**
  - `go test -count=1 -p 1 ./cmd/vtt/` is green, `scenario_goldens_test.go`
    and `mcp_e2e_test.go` included.
  - `go test -count=1 ./internal/harness/ -run TestFoldGoldenCorpus` and
    `env -u CLAUDECODE bun test client/test/fold-parity.test.ts` are green
    with no golden touched (D14).
- **Phase 4a:** one QA agent. Its file is
  `cmd/vtt/qa_state_dump_switch_test.go`, prefix `qaSDS`.

### Task 7: `task check`, whole, on Task 6's commit (item 9)

- **Before launching:**
  - Run `docker info`. `check:ts-mutation` re-mutates the client, which
    needs the container on this Mac.
  - Read the load.
  - Expect Stryker at about 43 minutes and `internal/mcp` at about 857 s.
    The last green run took 3663 s (G12).
- **Launch:** with `start_new_session=True`, and read the gate's own
  completion lines.
- **True when:**
  - it exits 0;
  - no floor is lowered, and no ceiling raised.
- **A survivor** goes back to its task's file: a test where a test can see
  the mutant, otherwise an adjudication. The gate is then rerun on the new
  last code commit.

### Task 8: the implementation report

- **File:** `docs/reports/2026-10-21-the-go-readers-follow-the-log-they-are-sent.md`,
  written by the `implementation-report` skill against the ticket's ten
  items.
- **It carries:**
  - G1-G13;
  - Q1-Q4's answers;
  - the probe measurements quoted in this plan;
  - each break and what it reddened;
  - Phase 4a's adjudications, one line each;
  - the rows lowered;
  - the soak's new time;
  - Task 7's figures at their commit;
  - the history moved out of the rewritten comments (D18).

### Task 9: the specification step (item 10; G8)

- **Files:**
  - **SPEC-022 (D16)** gains the Go readers:
    - what each holds, and what it discards and keeps at a
      `ViewpointChanged`;
    - what each redial names;
    - that the dump refuses at a switch before its head and stops at one
      after it;
    - the `log` key (D14);
    - the soak's per-seat comparison.

    The Status line gains their tests.
  - Each Requirements list gains its rows. No specification lists VTT-360.
- **True when:**
  - each passage passes the `specification` skill's catches list, read
    against the committed code;
  - `task check:requirements-chain` is green.

## For Phase 4a

The installed process's `qa-prompt.md` governs:
- each agent is handed the governing specifications whole, never a section
  list;
- it gets its task's rows and the observable "True when" of its task;
- it gets the exported surface, extracted for it in advance;
- it is withheld the source bodies, the task's tests, the diff, this plan's
  decisions and the verdict.

**For every agent:**
- Hash the tree before and after it runs.
- It writes only its own file, with its prefix, and cites the rows.
- Its tests are committed with the task. Its adjudications go in the report.

**Every brief names:**
- `go-arch-lint` and the package's import boundary;
- `-cpu 1`;
- that a log's own entries carry no `EventId`, so compare whole entries with
  `proto.Equal`;
- that the opening frames of a log seat are `CatchUpHead`, `PresenceSnapshot`,
  `WatchableChanged` and `ViewpointChanged`, and a reader must take each by
  kind.

**By task:**

| Task | Rows | Specifications, whole | Exported surface | Writes its own |
|---|---|---|---|---|
| 2 (`qaCS`) | VTT-409, VTT-410 | SPEC-007, SPEC-022, SPEC-011 | `go doc -all ./internal/harness` | fake gateway, on `coder/websocket` |
| 3 (`qaOL`) | VTT-411 to VTT-415 | SPEC-007, SPEC-022, SPEC-015, SPEC-021 | `go doc -all ./internal/harness` | fake `Conn`s |
| 4 (`qaSLS`) | VTT-411's soak half, VTT-416, VTT-417 | SPEC-007, SPEC-022, SPEC-015, SPEC-021 | `go doc -all ./internal/harness` | fake soak world |
| 5 (`qaMOL`) | VTT-418 to VTT-422 | SPEC-007, SPEC-022, SPEC-015 | `go doc -all ./internal/mcp`, `go doc -all ./internal/harness` | fake gateway; reaches the server through the SDK's in-memory transports |
| 6 (`qaSDS`) | VTT-423 to VTT-426 | SPEC-007, SPEC-022 | `vtt state dump --help`, and the signature of `cmd/vtt`'s `runCLI` | fake gateway |

## Questions for the owner

Each has a recommended answer first.

Answered 2026-10-21, each as recommended.

1. **Q1. Does the browser client's rule set get a record of its own?**
   **Answered: yes, SPEC-022.**
   - *Recommended: yes, SPEC-022, "A client holds one log",* holding the
     browser's rules (VTT-393, VTT-394 to VTT-398, VTT-402 to VTT-408) and the
     Go readers' (VTT-409 to VTT-426). SPEC-007 keeps the wire.
   - *Why:*
     - SPEC-007 calls itself the record "where the wire conventions live";
     - its Status already lists eight client test files and two e2e specs;
     - adding the Go readers would make it the home of four clients'
       behaviour;
     - a client's behaviour is a decision somebody would re-open apart from
       the wire's.
   - *Cost:* D16. SPEC-007 and SPEC-015 move three pointers, `wire.ts` one,
     and Task 1 moves the section.
   - *Otherwise:* the comments point at SPEC-007 and the section grows there.
2. **Q2. What does `vtt state dump` do when its seat is switched before it
   reaches its head?** **Answered: refuse, naming both logs.**
   - *Recommended: refuse,* naming both logs, and print nothing. At a switch
     after the head, stop and print the log held (D13).
   - *Why:* SPEC-007's snapshot reader stops at a `ViewpointChanged` after the
     first. Printing the new log needs an end the wire does not give for it,
     which is the quiet window `dumpQuietWindow`'s doc records as the defect
     it replaced. The existing refusal already tells the caller to retry, and
     a retry opens the log the seat now shows.
   - *Otherwise:* re-dial once, naming the new log, and dump that.
3. **Q3. How do `get_state`, `get_events_since` and the dump name the log?**
   **Answered: a `"log"` key, absent for the world.**
   - *Recommended: a `"log"` key,* the actor's id for a character log, `""`
     for nobody, and absent for the world, as `log` is absent from a world
     connection's handshake (D14).
   - *Why:* measured, every golden stream is the DM's. So the nine golden
     `state.json` files, `dumpJSON` and `client/src/fold.ts`'s
     `foldToDumpJSON` stay byte-identical.
   - *Otherwise:* the key on every answer, `null` for the world. That edits
     the nine hand-derived goldens, both mirrors and `fold-parity.test.ts`'s
     expectations. It is also code under `client/src` beyond comments.
4. **Q4. Are VTT-394 to VTT-398 extended to the Go readers, or are new rows
   allocated?**
   - *Recommended: new rows* (VTT-409 to VTT-426). VTT-394 to VTT-398 stay the browser's,
     and VTT-360 is withdrawn and succeeded by VTT-412 and VTT-413 (D15, G2).
   - *Why:* VTT-398 is false of the scenario engine, whose cursor is the
     scenario's. VTT-360 is false as worded once a seat is told of its
     switches.
   - *Otherwise:* extend VTT-397 alone. Its text holds for the engine, `vtt
     mcp` and the soak's fresh dial, and it would gain their tests.
5. **Q5. How does the Dialer learn a log?** Settled by D3, a parameter. A
   `Conn` method cannot reach `dialerFor`, which builds the URL. The cost is
   33 test sites, four inner calls and one call in `cmd/vtt/dialer_test.go`,
   each one token, with no comment line.
   The owner may overturn it.
6. **Q6. Does the soak's per-seat check dial with `log`?** Settled by D11:
   yes, forced by the overflow resume (VTT-349). G13's test is the only one
   that can show it.
