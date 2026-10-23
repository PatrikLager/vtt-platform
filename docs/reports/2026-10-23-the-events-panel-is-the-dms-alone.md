# The Events panel is the DM's alone: the change

**Ticket:** `docs/superpowers/specs/2026-10-23-the-events-panel-is-the-dms-alone-design.md`,
verified by an agent that did not write it (passes with gaps G1-G5, which the
writer carried into the ticket before its commit).
**Plan:** `docs/superpowers/plans/2026-10-23-the-events-panel-is-the-dms-alone.md`,
written by that verifier and amended at sign-off and by the reading review of
`2cf722b`, the commit that carried both.
**Where it came from:** ruling B-3 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`, the third of the
nine tickets in the order the owner agreed.
**The owner's rulings:** B-3, 2026-10-22: the Events panel is the DM's alone,
players and spectators do not have it, and the Story stays as it is ("kändes
bra, just nu iaf"). 2026-10-23: the plan signed off ("Ja, kör"), and Q1,
whether "the DM" includes the agent, "Ja, som DM-konsolen".

**Last code commit:** `0d7ebc8`, on `4fac397`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `2cf722b` | The ticket, the plan and VTT-445 and VTT-446 |
| `0d7ebc8` | Task 1: `ViewExtras.ticker` and `ticker: isDM`; the comments the change made false; A1-A3 and S1; the edits that make the panel's tests draw a DM's window; the e2e; QA's `client/test/qa-events-panel.test.ts`; the bundle; the register's cells and VTT-447 to VTT-449 |

## What the code did on `4fac397`

`renderSpectator` in `client/src/view/spectator.ts` built the same sections
for every window: the status, the board, the Story (`renderFeed`), the notes
and the Events panel (`renderTicker`). Of the sections it drew, only the perch,
the player's panel and the DM's console depended on the role, and `paint` in
`startSession` chose which of those to pass. So every role's window showed the
panel, and so did a page whose `/api/me` had not answered, or had not answered
with a role. `spectator.ts`'s header listed "event ticker" in the spectator
floor, after section 4 of `docs/superpowers/specs/2026-07-26-client-design.md`,
which gives "Everyone (spectator floor)" an event ticker; the ticket overturns
that, as VTT-393 overturned the same section's sequence numbers.

## Done looks like, answered

1. [x] "a player's and a spectator's window draws no Events panel, and its
   Story shows what it was sent" in `client/test/app.test.ts` (A1) boots each
   role, requires its own panel (`.player`, `.perch`), and requires no
   `section.ticker`, no heading reading "Events" and five `.feed .mechanical`
   rows. Before the code it was red on the panel's count (expected 0,
   received 1).
2. [x] "a page whose /api/me answers no role draws no Events panel, and its
   Story shows what it was sent" (A2) serves no metadata route, so `/api/me`
   answers 404, and requires five Story rows and no `section.ticker`. Red
   before the code the same way. QA's "qaEP: until /api/me answers, a page
   sent the world draws no Events panel, and draws it once the answer is dm"
   holds the pending case.
3. [x] In `client/e2e/table.spec.ts`, "a spectator sees the table and no
   controls" waits for `.perch` and requires no `.ticker`; "the story feed
   and ticker record what happened" waits for the player's `.player`, requires
   no `.ticker`, then opens a DM's page in a new browser context and requires
   `.ticker .tick` rows. Run first on `4fac397`'s bundle: the spectator,
   shoulder and player checks went red on `.ticker` (expected 0, received 1).
4. Still true afterwards:
   - [x] A DM's and an agent's window show the panel, newest first: "a DM's
     and an agent's window draws a row in the Events panel for each entry it
     was sent, newest first" (A3), whose rows are the Story's reversed and
     whose first reads "t1 placed at 1,1"; and the tests in
     `client/test/spectator-view.test.ts` that read `.ticker` and `.tick`,
     which now draw a DM's window. The exception, a window that gives the
     reason it cannot fold its log, is held by QA's "qaEP: dm: while the
     window gives the reason it cannot fold its log, it draws no Events panel"
     and its `agent` twin.
   - [x] VTT-263 and VTT-393: their tests are green. VTT-263's, "the feed and
     the ticker show a move's reason" in `client/test/spectator-view.test.ts`,
     now draws a DM's window. "no role's window shows a log sequence number"
     reads the panel for `dm` and `agent` and its absence for `player` and
     `spectator`; the VTT-393 e2e reads the player's Story and the DM's
     panel.
   - [x] The Story is unchanged: `git diff 4fac397 0d7ebc8 --
     client/src/view/feed.ts` is empty, and `renderSpectator` draws
     `renderFeed` for every role.
   - [x] "a spectator chooses a shoulder and is shown that log" reads
     `.feed .mechanical` where it read `.tick`, on the shoulder and after the
     "No shoulder" button, where it now also requires `.feed .empty`; green.

`task check`, whole, exited 0 on `0d7ebc8` in 5071 s, launched detached; no
floor was lowered and no ceiling raised. `task e2e` on the same commit: 19
passed and 1 skipped, in 22.9 s. The skipped one, "a network drop while the
table plays on ends in a Reconnect", is a `test.skip` in
`client/e2e/handover.spec.ts`.

## What the rules became

- The ticket's one rule, "only a window the client knows to be a DM's or an
  agent's shows the Events panel", became two rows, since its halves break
  apart (K1-K4 against K5-K6):
  - **VTT-445**, "a browser window draws no Events panel unless the role the
    client last read for its participant is `dm` or `agent`";
  - **VTT-446**, "a browser window whose participant's role the client last
    read as `dm` or `agent` draws the Events panel, except while it shows the
    reason it cannot fold its log".
  Both were reworded before their first commit, while nothing cited them:
  VTT-446 as first worded was false for a window whose log cannot fold, which
  `paint` replaces with the reason before `renderSpectator` runs, and VTT-445
  took "last read", which is what `paint` reads.
- **VTT-447**, the panel's rows are the newest entries of the log its window
  holds, one row each, the newest first; **VTT-448**, the panel draws no more
  than a fixed number of rows, the figure left to the test; and **VTT-449**,
  the Story shows every entry of that log whatever the role, with VTT-446's
  exception. All three were allocated after Phase 4a, for sentences QA's
  tests cited. VTT-447 and VTT-448 reverse the plan's own sort, which had
  refused "at most 40, newest first" as behaviour this ticket does not
  change; VTT-449 is the positive form of its refused "The Story is
  unchanged".
- VTT-263 and VTT-393 are unchanged in wording; the tests that hold them read
  the panels each role's window has.

The sort at sign-off refused five candidates: "at most 40, newest first" and
"the Story is unchanged" (both since reversed, above); the flag's name, the
DOM order and the stylesheet, which are how, not what; "the panel is no
boundary", which has nothing to go red; and the shoulder e2e reading the
Story, a test's mechanics. After Phase 4a it refused three more of the
sentences QA cited: the panel's heading, a definition held under VTT-446;
"the page knows no role" until `/api/me` answers one, which is VTT-445; and
the role's re-read at a presence frame, behaviour this ticket does not change,
held here through VTT-445 and VTT-446. QA's citation of SPEC-022's sentence on
a discard was dropped where the test already cites VTT-395.

## Phase 4a

QA ran once, for Task 1; Task 0 is prose. The agent was given a brief stating
the owner's rulings, VTT-445, VTT-446, VTT-393 and VTT-263, and sentences
without an id on the panel's heading, its bound and order, the Story, and when
the page knows its role; SPEC-022 whole; and a hand-written surface (the
declarations `tsc --declaration` emits; the project records `go doc` for Go
only). It was withheld the source, Task 1's tests, the plan and the diff.
It wrote 42 tests, of which 41 passed; one was withdrawn (finding 1). It made
about 26 injections into its own fixtures and expectations, all red but one,
which it reported: with the role's-panel precondition removed, a player's
absence test passes on a page with no role at all, so that precondition is
what makes those tests able to fail.

### QA adjudications

- QA finding 1: spec ambiguity — the brief QA was handed, written by the
  controller, said a 404 from `/api/me` leaves no role, and for a re-read that
  is false: VTT-445's "last read" keeps the role a failed re-read did not
  replace — the test was withdrawn, QA's boot-time 404 test now says "first",
  and SPEC-022, updated after this report, is to say a failed re-read keeps
  the role last read. The failed re-read also rejects unhandled, since
  `onPresence`'s `fetchMe` has no `catch`; that predates this ticket and shows
  only as a console error, so no code changed (the owner's ruling of
  2026-10-21 on near-theoretical cases, the plan's constraint "A
  near-theoretical edge case with small consequence is no debt").
- Spec gap 1: which role holds while a re-read is pending, or after it fails
  at the network — the last read, VTT-445 — SPEC-022, updated after this
  report, is to say so.
- Spec gap 2: SPEC-022 said nothing of the panel — SPEC-022, updated after
  this report.
- Spec gap 3: what a row says, beyond a move's reason — `describe`'s labels
  are A-2's, the fifth ticket — no change.
- Spec gap 4: "newest" by arrival or by sequence — the two agree on the
  wire — no change.
- Spec gap 5: that a log which cannot fold replaces the whole window — said
  only by QA's surface — SPEC-022, updated after this report, is to name it
  as the exception.
- Spec gap 6: "this work does not change" the Story cannot be tested without a
  baseline — the empty diff of `feed.ts` holds it — no change.
- Spec gap 7: four QA tests re-read a role into or out of `dm` or `agent`,
  which no promotion does (SPEC-009 refuses a promotion to `dm` or `agent` and
  of a `dm` or an `agent`): "qaEP: a dm re-read as player …", "qaEP: a player
  re-read as dm …", "qaEP: a spectator re-read as agent …" and "qaEP: a dm
  whose re-read answers with no role field …" — kept: they pin the client's
  "last read" — no change.
- The surface's presence example named `PRESENCE_STATE_PRESENT`, which the
  contract does not define; QA used `PRESENCE_STATE_CONNECTED` — the
  controller's error — no change.
- Exposure, disclosed by QA: bun's error output printed six lines of
  `client/src/metadata.ts`, `fetchMe`'s first three and the three before
  them, a file that is not the unit under test, after the test that drew it
  was written — no change.
- Outside this ticket: the Story labels a grant with its payload's case name,
  "actorControlGranted" — A-2, the fifth ticket.
- Sentences needing an id: six. Two earned ids, three in all, VTT-447 to
  VTT-449; three were refused and one dropped (above). QA's citations of its
  brief, a file outside the tree, were replaced by ids, or deleted where the
  test already carried its id, and "after an entry of sequence 1 discards …"
  gained `VTT-447 VTT-395`. QA's file header gained that it drives the page
  once through `renderSpectator` directly, and one comment was rewrapped.

## Phase 4b

One reviewer per task, two passes each, by command, on copies of the tree.

- Task 0 (`2cf722b`). Pass 1 found one Must: VTT-446 as first worded was
  false for a window whose log cannot fold. Its Shoulds: the rows' evidence
  lists left out checks the plan relied on; the plan's table of items dropped
  the ticket's item 4.4; and K6's expected reds were short. Pass 2 found one
  Should: the ticket's item 4.1 lacked the same exception.
- Task 1 (`0d7ebc8`). Pass 1 found no Must and four Shoulds:
  - the register's evidence missed tests that hold parts of VTT-445 to
    VTT-449; its probes, a Story or a panel that drops narration, stayed
    green on every test then listed;
  - the commit message did not say the sort reversed the plan's;
  - QA finding 1's wording, and the name of QA's boot-time 404 test;
  - `client/src/style.css`'s perch comment, "since the perch is the only
    panel a spectator has, this template names no others", had become false:
    the template names `ticker`, which a spectator never fills.
  Its nits included the field's doc pointing at SPEC-022, which says nothing
  of the panel before this ticket's specification step, and
  `qa-window-number.test.ts`'s world render asking for the panel without
  checking it is drawn. Pass 2 found one Should, a miscount in the commit
  message.

## The deliberate breaks

Each applied by a script to the working tree and undone by its inverse edit,
with the file's hash compared afterwards. Red counts are the client tests of
the five test files Task 1 touched, QA's included, on Task 1's tree. The
Task 1 reviewer re-ran K1-K10 on copies, and the reading review of this report
re-ran all eleven on a copy of `0d7ebc8`; both matched every count.

| Break | Red |
|---|---|
| K1: the panel drawn whatever the flag | 14: A1, A2, S1, the VTT-393 unit test and ten of QA's; in the browser, the spectator, shoulder and player checks |
| K2: `ticker: me === null \|\| isDM`, MapTool's null player | 3: A2 and two of QA's |
| K3: `ticker: canAct` | 5: A1's player half, the VTT-393 unit test, three of QA's |
| K4: `ticker: me !== null && me.role !== "player"`, the panel for every `/api/me` answer but `player`'s, one with no role included | 7: A1's spectator half, the VTT-393 unit test, five of QA's |
| K5: `dm` alone | 14: A3's agent half, the VTT-393 unit test, twelve of QA's |
| K6: the push deleted | 43: A3, six in `spectator-view.test.ts`, six in `qa-move-reason.test.ts`, the VTT-393 unit test, `qa-window-number.test.ts`'s world render, 28 of QA's; in the browser, the DM's half of the VTT-393 test |
| K7: the Story emptied without the flag | 29, A1, A2 and S1 among them (39 across all of `client/test`) |
| K8: the panel oldest first | 19, A3 among them |
| K9: the panel unbounded | 10, "the ticker is bounded — an all-night session must not render thousands of rows" among them |
| K10: the Story bounded at 40 | 7, S1 among them |
| F1: A1's `/api/me` dropped | A1, on its role's-panel anchor (expected 1, received 0) |

A3, a correct DM's and agent's window, stays green under K1-K4 and K7.

## Rule 9: how MapTool does this

MapTool builds its GM macro frame (`MTFrame.GM`) for every client, and
`GmPanel.init` fills it only when `MapTool.getPlayer() == null ||
MapTool.getPlayer().isGM()`: a player's frame is present and empty, and a
client with no player yet fills it. Whether MapTool's default layout shows that
frame to a player was not checked. It keeps no event log for anyone; its
chat carries text, rolls and macro output, and `MapTool.addServerMessage`
drops a GM, GM-me, not-GM or whisper message on a client it is not for. The
role gate in one predicate is borrowed (`isDM`, as `isGM`); the null player
falling open (K2) and the empty frame are not. The panel shows only the log
the gateway sent, which the Story shows too, so this is a choice about the
window and not a boundary. The plan's rule 9 section carries the detail.

## Figures

From the one `task check` on `0d7ebc8`:

- Go coverage: 21 packages at or above their floors; the gate notes 11 that
  clear theirs by more than 1.0pp. The ticket changed no Go code.
- Client coverage: 22 files gated, each at 100.00%, `client/src/app.ts` and
  `client/src/view/spectator.ts` among them; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors. Each reused its
  last passing verdict, since nothing in its dependency closure differed. 38
  survivors, all adjudicated; 7 timed out and counted as killed, not
  measured.
- Client mutation: in its container, Stryker's own "Done in 67 minutes and 1
  second": 2966 mutants, 2874 killed, 29 timed out and counted as killed, 63
  survivors all adjudicated equivalent. `client/src/view/spectator.ts`: 378
  killed and 2 survived, against 376 and 2 on `e9cb153`, the previous
  ticket's last code commit and the gate's last run; its two survivors are
  the `<` to `<=` of `renderPerch`'s sort, on the label and on the tie by
  `actorId`, which this ticket did not touch.
  `client/src/app.ts`: 224 killed, none survived, as on `e9cb153`. Of the 29
  timeouts, none is in a file this ticket touched: 20 are in
  `client/src/fold.ts`, against 1 on `e9cb153`, where that file went from
  484 killed, 1 timed out and 14 survived to 467, 20 and 12; 6 in
  `client/src/view/scene-plan.ts` and 2 in `client/src/view/canvas.ts`, as
  on `e9cb153`; and 1 in `client/src/wire.ts`, the `BlockStatement` of
  `ws.onerror` in `connect`, as on `e9cb153`. The gate reads such a count as
  a measurement of the machine and says to re-run on an idle one before
  changing a test; no test was changed.
- Race: both runs pass.
- Requirements chain: 449 rows, 287 test files, 16 specifications.
- Comments: 319 files, 2 comment lines added against `main`, clean:
  `renderTicker`'s warning and `ViewExtras.ticker`'s one-line doc. Its two
  notices are the ones `e9cb153` had:
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6 and
  `internal/gateway/export_test.go` at 49.18 against 45.9.
  `tools/comment-ceilings.txt` is unchanged.
- New prose: 651 added lines across 8 files, clean.
- Disk: 37.8 GiB free when the Go mutation step began, and 38 GiB by `df -g`
  just after it ended.
- Duration: 5071 s, from the launch at 14:34:05 to the exit at 15:58:36,
  against 4701 s on `e9cb153`; Stryker's run took 67 minutes and 1 second
  here and 59 minutes and 26 seconds there.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| The sort refuses "at most 40, newest first" and "the Story is unchanged" | VTT-447 to VTT-449 | QA's tests cited the sentences, and a cited requirement gets an id; VTT-448 leaves the figure to the test |
| D3: `style.css` is not touched | Its perch comment lost a clause; comment only, and `index.css` is byte-identical | The Task 1 review: the clause had become false. The plan's E5 had kept it, reading it as true while every role's template named `ticker` |
| D4: the field's doc points at SPEC-022 | It points at VTT-445 | SPEC-022 says nothing of the panel until this ticket's specification step |
| S1 renders two events | 45 | So a Story bounded like the panel turns a test the register can cite red (K10) |
| Breaks K1-K7 and F1 | K8-K10 too | VTT-447 to VTT-449 are rules, and each needed a break of its own |
| D5: the world render asks for the panel | It also checks the panel is drawn | The Task 1 review: K6 left it green |
| Order step 1: the e2e runs red on `4fac397`'s bundle | It did, and `handover.spec.ts` and `joining.spec.ts` also timed out, in a run of 10.5 minutes | The machine's load was 20 to 34 from other processes; both are green in the two whole runs since, one of them at a load of 1.6 |

## What could not be established

- The page's layout without the panel was not measured beyond the e2e's
  screenshots. In `04-player-feed` the player's page shows one more gap
  between its panel and the notes: the `dm` row of the template the DM's, the
  agent's and the player's pages share, the last bare `#app` rule in
  `client/src/style.css`, which the spectator's `#app:has(.perch)` template
  does not name; `04b-spectator-on-a-shoulder` shows none. The ticket changed
  only a comment in that file, so the row was named before too. Whether it
  showed as a gap while the panel spanned it was not measured, since the
  screenshots are overwritten on each run.
- Whether `client/src/fold.ts`'s 19 more timeouts than on `e9cb153` would be
  killed on an idle machine: that file went from 484 killed, 1 timed out and
  14 survived to 467, 20 and 12, so 17 fewer were killed and 2 fewer survived.
  The gate says to re-run on an idle machine before changing a test; none was
  run and no test was changed. The ticket did not touch that file.

## What was deliberately left out, and where it went

- The macro button whose attack carries text and mechanics: later, by the
  owner's ruling B-3.
- What a row says: the panel and the Story print some payloads by their case
  name (`actorControlGranted`, `narrationAdded`, `sceneSeen`), which is A-2,
  the fifth ticket's.
- The failed re-read's unhandled rejection: near-theoretical with a console
  error for its consequence, so no code, no test and no debt entry, by the
  owner's ruling of 2026-10-21.
- B-4, the fourth ticket: a player who controls no party member keeps the
  role `player`, so VTT-445 already keeps the panel from them, as the plan
  says under Forward coupling.
- Letting the Story fill the area the panel leaves: the ruling keeps the
  Story as it is (D3).
- SPEC-022's new sentences: who has the panel and what it shows (VTT-445 to
  VTT-448), the Story (VTT-449), the exception for a log that cannot fold,
  and that a pending or failed re-read keeps the role last read. They go to
  the plan's Task 4, after this report; the last is not in that task's list,
  and QA finding 1 and spec gap 1 added it.
