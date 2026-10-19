# Plan: the window shows what happened, and no log number

- **Ticket:**
  `docs/superpowers/specs/2026-10-19-the-window-shows-no-log-number-design.md`.
- **Verdict:** passes with gaps (G1-G6, below). Verified 2026-10-19 by an
  agent that did not write the ticket, read-only, against HEAD `866bc28`.
  Its probes ran on copies of the tree made with `git archive HEAD` and wrote
  nothing into the repository. They used `bun test`, `tsc`, `vite build`,
  Playwright, Stryker, `check-comments.py` and `task check:drift`.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-19-the-window-shows-no-log-number.md`.
- **Base:** main `866bc28`.
- **Branch:** `fix/the-window-shows-no-log-number`.
- **Signed off:** 2026-10-19 by the owner ("Ja, kör"), with the two owner
  points answered: the e2e opens as the player ("Spelaren", D8), and the fold
  failure's reason is left as R1's exception ("Låt det vara", G2, D10). Review
  findings are fixed and each task committed without asking, except a design
  question; push and merge on the owner's word.

## The gaps this plan inherits

- **G1. "To any role" has no observation in the ticket.** Item 1's test
  renders `renderSpectator` with no role, but `app.ts` chooses the panel,
  console and perch per role. D7 adds the test that observes it.
- **G2. A fold failure's reason can print a log number into the window.**
  - `app.ts` replaces the window with "Cannot derive state from the log:
    <message>".
  - `fold.ts`'s "session already open at sequence N" and "session ended with
    none open at sequence N" name the sequence.
  - A player's or a spectator's log switch reaches it today (G8 of
    2026-10-17). It was measured in the client harness: "… session already
    open at sequence 1".
  - The rulings concern the window that shows what happened, and a failure
    reason is not that. R1 names this as its one exception, and D10 leaves
    the code alone. The owner ruled so at sign-off ("Låt det vara").
- **G3. Item 6's spectator clause.** A spectator's results never carry a
  sequence, so SPEC-007 says what is true (D11).
- **G4. Item 3 is met by a role change.** It hides G8's spectator half from
  `task e2e` (D8). The owner chose the player at sign-off ("Spelaren").
- **G5. `spectator.ts` ends above its comment ceiling.** It is a notice, and
  it is accepted (D9).
- **G6. Not established:** how long `task check` takes today (Task 2). The
  verdict's second point is settled by the ticket's writer:
  `TestThreeRoleExitScenarioOverLiveWebSockets` in
  `internal/gateway/scenario_test.go`, at its "player-move-lera" step, has
  `issueAndVerify` require the player's `result.Sequence` to equal the
  sequence of the event the DM's and the agent's connections receive.

## Constraints that bind every task

The rules are named here and stated where they live:

- **CLAUDE.md rules:**
  - 2: no floor, ceiling or exemption moves to pass, and
    `tools/comment-ceilings.txt` is not touched.
  - 3: no contract change.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: below.
  - 10: no comment line is added under `client/src`. Test files carry
    citation lines.
- **The owner's rulings of 2026-10-19, as the ticket quotes them.**
  - Only the window changes ("Bara fönstret").
  - Every log number stays inside the engine and on the wire.
  - `CommandResult.sequence` stays.
- **What follows from them, by this plan:**
  - Nothing changes in the wire, the gateway, the harness, `client/src/fold.ts`
    or `client/src/app.ts`.
  - Under `contract/`, `internal/` and `cmd/`, only `cmd/vtt/webdist` moves
    (item 4).
- **Tier 1 runs at pre-commit:** `bun test client/test contract
  contract-spike` and `task client:typecheck`.
  - The source and its tests go in one commit.
  - Map a `NodeList` through `Array.from`, or the typecheck fails.
- **bun is quiet under `CLAUDECODE`.** Run it as `env -u CLAUDECODE bun test
  …`.
- **The bundle.**
  - The commit hook does not catch a stale bundle. Run `task build:client`
    before the commit.
  - `task check:drift` reads `git status --porcelain` over `cmd/vtt/webdist`,
    so it can pass only after the commit.
  - `task e2e` builds `vtt` from the working tree, with whatever webdist is
    there. Run it after `task build:client`.
- **VTT-263's evidence cell names
  `client/test/spectator-view.test.ts#the feed and the ticker show a move's reason`.**
  That test keeps its name.
- **Comments.** `python3 tools/check-comments.py` runs against main and
  against HEAD, after staging, because a move shows only then.
- **TS mutation.** Any change to `client/src` or `client/test` makes
  `check:ts-mutation` re-mutate the whole client in its Linux container. The
  stamp in `reports/mutation/` matches HEAD's inputs today.
- **Coverage.** `client/src/view/spectator.ts` stays at 100.00 (measured).
- **`review-record` refuses untracked files.** Commit the ticket and this plan
  before Task 1's review, and park scratch files outside the tree.
- **Mechanics.**
  - A new test goes after a closing brace, or it inherits its neighbour's doc
    comment.
  - Stage and commit in separate calls.
  - Undo a break with the inverse edit and check it with grep, never with
    `git checkout --`.

## Rule 9: how MapTool does this

The ticket's answer holds, checked in `~/dev/RPTool/maptool`:

- `MessagePanel.addMessage(final TextMessage message)` renders the message's
  text.
- `TextMessage` holds `channel`, `target`, `message`, `source` and
  `transform`, and no sequence (`channel` is an `int`, a channel's code).
- Every method of `ServerCommand` returns `void`.

What we take is the chat window that shows text alone. We need nothing else
from it, because MapTool has no per-log sequence to hide. Ours stays on the
wire, which is where the rulings keep it.

## Decisions

1. **D1. A ticker row is one element whose only text is its description:**
   `wrap.appendChild(el("p", "tick", describe(e)))`.
   - Forced by item 1, "exactly its event's describe text".
   - Today's text node is `` ` ${describe(e)}` ``, so dropping only the span
     would leave a leading space.
   - `renderTicker`'s comment stays as it is (D9).

2. **D2. A row keeps no sequence anywhere:** no `data-` attribute, no `title`,
   no hidden element. Forced by:
   - **The rulings.** The window is "bara texten", and the numbers stay
     "internt" and on the wire. A player can still read sequences in the raw
     frames, by the "Bara fönstret" ruling.
   - **No consumer.** Nothing in `client/src` reads a sequence back from the
     DOM. The client's own uses of sequences read what it was sent: the
     resume cursor is `wire.ts`'s `seenSeq`, set from each event frame, and
     rollback and `perchShaped` read `session.events`, which keeps every
     `Envelope` whole.
   - **D4 needs none.** A hidden copy would put a log's number into the page
     a player inspects, only so a test could look a row up.

3. **D3. `.seq` leaves `client/src/style.css`.**
   - No other user: `grep -rn '"seq"\|\.seq\b' client/src
     client/index.html client/public` finds only `style.css`'s rule and
     `renderTicker`'s span. The `seq` that `feed.ts` and `fold.ts` name is a
     property or a variable, not a class.
   - The rebuilt `index.css` and `index.js` carry no `seq` class (measured).

4. **D4. `tickFor` finds a row by its place.**
   - It becomes `tickFor(root, nth)`: the row of the log's nth event, counting
     from 1 for the oldest. It reads `rows[rows.length - nth]` and throws when
     there is no such row.
   - Its comment says rows are newest first.
   - Why position:
     - The row carries nothing but its text (D2).
     - Finding a row by its text would make the lookup depend on the label
       under test.
     - Every QA log here is far under the bound of 40.
   - One call changes, in "a move with no reason renders as before in both
     places", whose log holds one event: `tickFor(root, 4)` becomes
     `tickFor(root, 1)`.
   - Every other call already passes the event's place, because those logs
     are numbered from 1.

5. **D5. Names.** Two tests name the number and are renamed. Neither name is
   cited anywhere (grep).
   - "the ticker shows sequence numbers, newest first" becomes "the ticker
     shows the newest event first". It asserts
     `ticks[0].textContent` is `'scene "H"'`.
   - "each ticker row carries its sequence number and its description"
     becomes "each ticker row carries its description, under the Events
     heading". It asserts the row's text is `"session started — S"` and
     drops the `.seq` assertion.

   "the feed and the ticker show a move's reason" keeps its name, because
   VTT-263 cites it. It now expects `"t1 moved to 3,4 — the floor gives way"`.

6. **D6. T1, for item 1:** `client/test/spectator-view.test.ts` "each ticker
   row is its event's description alone, newest first".
   - It goes after the closing brace of "the ticker shows the newest event
     first".
   - Three entries: a session, a scene and a move.
   - The rows must equal
     `["t1 moved to 3,4", 'scene "H"', "session started — S"]`.
   - The expectations are literal strings, not `describe(e)`'s output. A
     `describe` that put the sequence into its label would pass an
     expectation built from it, and it fails this one (measured).
   - Red today, because the rows begin "#N ". Green after D1. Both measured.

7. **D7. T2, for G1:** `client/test/app.test.ts` "no role's window shows a
   log sequence number", after the file's last test.
   - For each of `spectator`, `player`, `dm` and `agent`:
     1. stub `/api/me` with that role;
     2. `boot` with the file's `FakeSocket`;
     3. deliver entries 7001-7006: `SessionStarted`, `SceneCreated`,
        `ActorAdded` (a party member with no controller, since the fold
        refuses one), `ActorControlGranted` to the participant, a public
        `NoteUpserted` and a `NarrationAdded`;
     4. assert the narration is drawn, six `.ticker .tick` rows are drawn,
        the window's `textContent` contains none of 7001-7006, and no
        `title` or `aria-label` value in it contains one either: those the
        window shows on hover or reads aloud.
   - The grant is there so the player's panel has a character and the
     spectator's perch lists a party member.
   - When it fails, the received text names the role by its panel: "Whose
     eyes", "Your turn" or "DM console".
   - Measured:
     - green after D1;
     - red today, on the ticker;
     - red with a feed line suffixed by its sequence;
     - red with the console's session label carrying `StartSeq`;
     - and, to be measured in Task 1, red with a row given a `title` of its
       sequence, which leaves T1 and the text assertion green (measured by
       the Task 0 review on a copy).

8. **D8. The e2e opens as the player**, the owner's choice at sign-off.
   - **Why the spectator cannot be kept.**
     - Its connection watches nobody and is sent nothing until it perches
       (SPEC-015, VTT-341, VTT-369).
     - So its perch lists no party member: "No party members yet." in the
       page snapshot while Lera is one.
     - The client can neither dial with `log` nor read `ViewpointChanged`.
     - Making a spectator perch is ticket 4's client work, and a design
       question SPEC-015 leaves open. Taking it on here would be a scope
       change.
   - **Why the player rather than the DM.**
     - A player's connection is a log seat, as a perched spectator's would
       be: one character log, numbered by that log and replayed from its
       first entry. The DM's is the world.
     - The player is the role the ruling is about.
     - The DM version also passes on the new bundle and fails on the old one
       (measured), and is the alternative.
   - **The test becomes:**
     - open as `"player"`;
     - `.tick` rows exist;
     - exactly one `.tick` has text matching
       `/^tok-lera placed at \d+,\d+$/`;
     - no `.tick` has text matching `/#\d/`;
     - the screenshot is `04-player-feed-and-ticker`.
   - **Its comment** names the test that loads the adventure:
     - Lera's log, replayed from its first entry, holds what she saw of the
       adventure that "a DM starts a session and loads an adventure"
       loaded. That is the replay path working.
     - Each row is its description alone.
   - **Measured:**
     - red on the old bundle, at the exact-row assertion;
     - green on the new bundle;
     - whole `task e2e`: 16 passed, 1 skipped (handover's `test.skip`).
   - **The cost.**
     - `04-spectator-populated` leaves the demo screenshots.
     - The spectator's empty floor (G8) has no red test in `task e2e` until
       ticket 4.
     - The report says both.

9. **D9. The comment notice is accepted.**
   - The change removes four code lines and adds one. That takes
     `spectator.ts` from 49.60 to 49.84, above its 49.6 row, with no comment
     line added.
   - `check-comments.py` reports a notice and ends "clean" (measured).
   - Getting under the row would take deleting three comment lines this
     ticket has no reason to touch. Deleting `renderTicker`'s two-line
     comment leaves 49.68 (measured).
   - The row is never raised (rule 2, SPEC-010).
   - The next change that adds a comment line to `spectator.ts` must bring
     the file under its row. The report says so.
   - `internal/gateway/export_test.go` already carries a notice at main. It
     is not this ticket's.

10. **D10. Nothing else moves.**
    - The fold-failure reason (G2) stays, for three reasons:
      - the rulings concern the window that shows what happened;
      - `engine.Apply`'s session arms refuse with no sequence ("engine:
        session already open", "engine: no open session to end"); its
        `NarrationAdded` arm's anchor refusals carry one, and no control in
        the client sends an anchor (`addNarration`'s one caller,
        `client/src/view/player.ts`, passes none);
      - ticket 4, which resets the client at a `ViewpointChanged`, removes
        the switch that reaches it today.
    - The owner ruled at sign-off not to widen this ticket to `app.ts`
      ("Låt det vara").

11. **D11. SPEC-007 gains one passage in "The ordering contract".** It goes
    after the paragraph that ends "restarts them at the new log's first
    entry", and reads roughly:

    > **A sequence is for correlating and tracing, never for showing.** A
    > client correlates, resumes and rolls back by the sequences on the wire,
    > and whoever traces a problem reads them in the logs; the window a client
    > draws shows what happened as text and no sequence, to any role, except
    > in the reason it gives for a log it cannot fold (VTT-393; the owner's
    > rulings of 2026-10-19). A result's `sequence` names the first world
    > event its command produced on every connection, a player's included.
    > A player's events are numbered by the log they watch, so on a player's
    > connection a result's `sequence` names no event that connection is
    > sent, and may equal the sequence of an unrelated entry in that log: a
    > player's client correlates a result by `request_id` alone. A command
    > that appends nothing and a refused command answer with no sequence;
    > `set_viewpoint`, the one command a spectator may issue, is such a
    > command.

    - The Status line gains the two tests and R1.
    - The Requirements list gains R1.
    - The specification step reads every clause against the committed code.
      The player clause is held by
      `TestThreeRoleExitScenarioOverLiveWebSockets` (G6).
    - Why: item 6, corrected by G3. SPEC-007 is the one record that says
      what a client does with what it is sent. No client specification
      exists, and none describes the ticker.

12. **D12. Phase 4a: one QA agent, on Task 1.**
    - The two new tests were written by the people who expect them to pass.
    - The rule says "any role", and the window is assembled from five
      renderers.
    - An independent derivation from the rule is the check on what those
      tests leave out.

## Requirements

One row, allocated at sign-off by the dispenser (`requirement-id`) as
**VTT-393**, which starts `**OPEN — no test yet**`.

- **R1, VTT-393** (the ticket's rule, bounded by G2): "The client's window
  shows no log sequence number to any role, except in the reason it gives
  for a log it cannot fold."
  - **Verified by:**
    `client/test/app.test.ts#no role's window shows a log sequence number`,
    `client/test/spectator-view.test.ts#each ticker row is its event's description alone, newest first`.
  - **The observation.**
    - T2 goes red when any text a role's window draws carries the sequence of
      an entry it was delivered. Today that is the ticker's "#N" (measured).
    - T1 goes red when a ticker row carries anything besides its
      description.
  - If the owner reverses "Låt det vara", the exception clause goes and a
    test for the failure reason joins the row.

The sort refused five candidates:

- **"Every sequence stays on the wire."** Not new, as the ticket says itself.
- **"A player's result names a world sequence."** Existing behaviour that the
  owner ruled to keep, held already by
  `TestThreeRoleExitScenarioOverLiveWebSockets` through `issueAndVerify`, and
  SPEC-007 states it (D11).
- **"The ticker shows the newest 40, newest first."** Existing behaviour,
  held by "the ticker shows the newest event first" and "the ticker is
  bounded". This ticket does not change it, and it has no row of its own.
- **"A row carries no `data-` sequence."** That is how, not what (D2). T2
  reads the text and every `title` and `aria-label` value, which the window
  shows on hover or reads aloud; a `data-` attribute is shown by neither.
- **"The e2e opens as the player."** A choice of test, not a rule (D8).

## Tasks

The pattern in Task 1:

1. Write the tests and see each fail for the reason stated.
2. Implement, and see them pass.
3. Apply each break, see the named test go red, then restore it and check
   the restore with grep.
4. Run `python3 tools/check-comments.py` against main and HEAD after
   staging.

Each task is its own commit through the review gate.

### Task 0: sign-off and the id

- **Files:**
  - `docs/requirements.md`: R1's row, from the dispenser;
  - the ticket and this plan;
  - all three committed together.
- **True when:**
  - The owner has signed off D1-D12 and the owner points: D8's role and
    G2's exception (done, 2026-10-19).
  - R1 is allocated and OPEN.
  - `task check:requirements-chain` is green.

### Task 1: the window shows text alone (ticket items 1-5)

- **Files:**
  - `client/src/view/spectator.ts` (D1);
  - `client/src/style.css` (D3);
  - `client/test/spectator-view.test.ts`: T1 with `// VTT-393`, and D5;
  - `client/test/qa-move-reason.test.ts` (D4);
  - `client/test/app.test.ts`: T2 with `// VTT-393` (D7);
  - `client/e2e/table.spec.ts` (D8);
  - `cmd/vtt/webdist/assets/index.js` and `index.css`, by
    `task build:client`;
  - `docs/requirements.md`: R1's evidence;
  - Phase 4a's test file, if it writes one.
- **In order:**
  1. **The tests.** Write T1, T2, the D5 rewrites and D4. Run
     `env -u CLAUDECODE bun test client/test/spectator-view.test.ts client/test/qa-move-reason.test.ts client/test/app.test.ts`.
     Exactly eight tests fail (measured):
     - T1;
     - "the ticker shows the newest event first";
     - "each ticker row carries its description, under the Events heading";
     - "the feed and the ticker show a move's reason";
     - in `qa-move-reason`: "in a mixed log the reason labels only the move
       whose frame carries it", "a move with no reason renders as before in
       both places", and "an empty reason renders the plain label in the feed
       and the ticker";
     - T2.

     Each fails because its row begins with "#N ".
  2. **The source.** Apply D1 and D3.
     - The same run is green.
     - `env -u CLAUDECODE bun test client/test contract contract-spike` is
       green: 1167 tests on the probe copy.
     - `task client:typecheck` is green.
  3. **The bundle.** Run `task build:client`. `git status --short
     cmd/vtt/webdist` names `assets/index.js` and `assets/index.css` and
     nothing else.
  4. **The e2e.** Apply D8, then run `task e2e`: 16 passed, 1 skipped.
     Then check that no `vtt serve` or browser process is left and nothing
     listens on 8900-8910.
  5. **The breaks.** Each one was measured in the verification.
     - Put back the span: T1, T2, the three D5 tests and `qa-move-reason`'s
       three ("in a mixed log the reason labels only the move whose frame
       carries it", "a move with no reason renders as before in both places",
       "an empty reason renders the plain label in the feed and the ticker")
       go red.
       - With the bundle rebuilt from it, `bunx playwright test
         client/e2e/table.spec.ts` goes red at the exact-row assertion.
       - Copy `cmd/vtt/webdist` aside first. After restoring, rebuild, and
         `diff -r` against the copy prints nothing.
     - Suffix `renderFeed`'s mechanical line with its entry's sequence: T2,
       "the feed and the ticker show a move's reason" and `qa-move-reason`'s
       same three go red.
     - Put `StartSeq` into the console's session label (`renderDMConsole`'s
       session group): T2 and `dm-view`'s "the session group names the open
       session and offers End" go red.
     - Prefix `describe`'s `sessionStarted` arm with the sequence: T1, T2,
       "each ticker row carries its description, under the Events heading"
       and "describe renders a real label for every event kind it handles,
       not the fallback" go red.
     - Give each ticker row a `title` of `#` and its sequence: T2 goes red,
       and T1 stays green.
     - Drop `.reverse()` in `renderTicker`: T1, "the ticker shows the newest
       event first" and `qa-move-reason`'s "in a mixed log the reason labels
       only the move whose frame carries it" go red. The last one goes red
       because D4 reads rows by place.
  6. **Phase 4a**, below. Its tests are committed with Task 1.
  7. **Staging.** Stage, then run `python3 tools/check-comments.py main` and
     the same against HEAD.
     - The expected result is one notice for `spectator.ts` (49.84 above
       49.6), the notice main already carries for `export_test.go`, and no
       finding.
     - `tools/comment-ceilings.txt` is unchanged.
     - `task check:requirements-chain` is green with R1's evidence filled in.
  8. **The review.** Reading review (Phase 4b), then the review record, then
     the commit.
  9. **After the commit.**
     - `task check:drift` is green.
     - `git diff --stat main -- contract internal cmd` names only
       `cmd/vtt/webdist/assets/index.css` and `index.js` (item 4).
- **True when** steps 1-9 hold.

This is the last code commit.

### Task 2: `task check`, whole, on Task 1's commit (ticket item 7)

- **Before launching:**
  - Run `docker info`. `check:ts-mutation` will re-mutate the whole client
    in its container. The figure on record is 62 minutes, measured
    2026-08-27. Today's figure is not known.
  - Read the load with `os.getloadavg()`.
- **Launch** with `start_new_session=True`, and read the gate's own
  completion lines.
- **True when:**
  - it exits 0;
  - no floor is lowered and no ceiling raised;
  - `spectator.ts`'s notice is the only new one.
- **Records after this change no `.ts` or `.go` file.** Before each record
  commit, run `task check:requirements-chain`, `task check:new-prose` and
  `python3 tools/check-comments.py` against main and HEAD. No hook runs them.

### Task 3: the implementation report

- **File:** `docs/reports/2026-10-19-the-window-shows-no-log-number.md`,
  written by the `implementation-report` skill against the ticket's seven
  items, each with its observation.
- **It carries:**
  - G1-G6;
  - the three rulings;
  - that G11 of 2026-10-17 is settled by "Bara fönstret". The 2026-10-17
    report stays as written.
  - D8's role change, and its two costs;
  - D9's notice, and what it asks of the next change to `spectator.ts`;
  - the fold-failure reason, and the owner's ruling on it;
  - each break and what it reddened;
  - Phase 4a's adjudications, one line each;
  - Task 2's figures, at their commit.

### Task 4: the specification step (ticket item 6)

- **File:** `docs/specifications/007-the-wire-contract.md` (D11).
- **True when:**
  - the passage passes the `specification` skill's catches list, read
    against the committed code;
  - the player clause names `TestThreeRoleExitScenarioOverLiveWebSockets`;
  - `task check:requirements-chain` is green with SPEC-007 naming R1.

## For Phase 4a

One QA agent, on Task 1, before its commit.

- **It receives:**
  - R1's text and the three rulings;
  - SPEC-007's "The ordering contract" and "What the connection opens with";
  - SPEC-015's "Who is sent what" and "The default";
  - the exported signatures of `client/src/view/spectator.ts` and
    `client/src/app.ts`'s `boot`;
  - `client/test/support/dom.ts`;
  - `client/test/app.test.ts` as it is at main, for its `FakeSocket` and
    `stubMetadata` harness.
- **It is withheld:** T1, T2, this plan, the verdict, and the bodies of
  `spectator.ts`, `app.ts`, `dm.ts` and `player.ts`.
- **Its brief says:**
  - Run bun as `env -u CLAUDECODE bun test`.
  - Map a `NodeList` through `Array.from`, or `task client:typecheck` fails.
  - The fold-failure reason is R1's named exception (G2). A test that shows a
    number there is adjudicated in the report, not committed red.
  - A number the window shows on hover or reads aloud (`title`,
    `aria-label`) is in the window too.
- **Around it:** hash the tree before and after it runs.
- **Its output.** Its file is `client/test/qa-window-number.test.ts`,
  citing R1, and it is committed with Task 1. Its adjudications go in the
  report.

## Amended by the reading review before Task 0's commit

One reviewer read the ticket, this plan and VTT-393 against `866bc28`, by
command and on copies of the tree. Its Shoulds were applied before the
commit:

- D11's draft: a player's result may equal an unrelated entry's sequence in
  that player's log, so the passage says a player's client correlates a
  result by `request_id` alone, and it carries VTT-393's exception;
- a `title` holding the sequence passed T1 and T2, so T2 reads every `title`
  and `aria-label` value, with a break of its own (D7, Task 1, Phase 4a);
- the ticket's rule and item 6 now carry G2's exception, which the owner
  ruled at sign-off, and its problem quotes all three 2026-10-19 rulings and
  says that "Bara fönstret" narrows G11's ruling of 2026-10-17 to the window;
- the ticket's search sentence names what its search found, the fold
  failure's path among it;
- MapTool's `TextMessage` carries no sequence, though its channel is a
  number.

Its Nits were applied too: the break lists, D9's line count, D7's grant,
D2's resume cursor, D10's engine arms, the rulings' consequences labelled,
R1's reversal, Task 1's name, D3's search and a quotation's spelling.
