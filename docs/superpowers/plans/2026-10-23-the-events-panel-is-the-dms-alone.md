# Plan: the Events panel is the DM's alone

- **Ticket:**
  `docs/superpowers/specs/2026-10-23-the-events-panel-is-the-dms-alone-design.md`,
  from ruling B-3 of `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`,
  the third ticket in that note's order of work.
- **Verdict:** passes with gaps (G1-G5, below). Verified 2026-10-23 by an agent
  that did not write the ticket, read-only against HEAD `4fac397`. Probes ran
  outside the tree, in a `git archive HEAD` copy: scratch bun tests through
  `boot`, and a prototype of D1-D5 for its measurements. `task check`,
  `task e2e` and the mutation gates were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-23-the-events-panel-is-the-dms-alone.md`.
- **Base:** main `4fac397`.
- **Branch:** `fix/the-events-panel-is-the-dms-alone`.
- **Owner's ruling taken in:** B-3, 2026-10-22. The Events panel is the DM's
  alone, players and spectators do not have it, and the Story stays as it is
  ("kändes bra, just nu iaf"). A macro button carrying text and mechanics is
  later, not now.
- **Signed off:** 2026-10-23, by the owner ("Ja, kör"). Q1 answered the same
  day: "Ja, som DM-konsolen", the recommendation (D2).

## The gaps this plan inherits

Each gap gives the text the verifier proposed. The writer revised the ticket
for each before sign-off, and the ticket's text is the authority: where it
differs, the ticket stands.

- **G1. A citation that does not resolve (check 1).** The problem says ruling
  B-3 "settles question D-1, in
  `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`". That note holds
  no D-1. The question is in the owner's walk notes, which are outside the tree.
  Proposed: "The owner ruled on 2026-10-22 (ruling B-3 in
  `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`) that …".
- **G2. `table.spec.ts` has three tests that read the panel as somebody other
  than the DM, and the ticket names one (check 4).**
  - "a spectator chooses a shoulder and is shown that log" uses the
    spectator's `.tick` as its proof that the log arrived. After the change no
    `.tick` appears, so the check on the shoulder goes red, and the check after
    "No shoulder" passes without proving anything. The plan for the second
    ticket handed this test to B-3 in its "Forward coupling".
  - "the story feed and ticker record what happened" is VTT-393's browser
    evidence, cited by name in the register, and reads the player's `.tick`.
  - Proposed for "What it touches" item 3: "`client/e2e/table.spec.ts`: "a
    spectator sees the table and no controls", which asserts the panel visible;
    "a spectator chooses a shoulder and is shown that log", which reads the
    spectator's `.tick` as its evidence of the log; and "the story feed and
    ticker record what happened", VTT-393's browser evidence, which reads the
    player's `.tick`."
  - Proposed, a new point under item 4: ""a spectator chooses a shoulder and is
    shown that log" reads the Story (`.feed .mechanical`) where it read `.tick`,
    on the shoulder and after "No shoulder", and stays green."
- **G3. One test file goes thin without going red (check 4).**
  `client/test/qa-window-number.test.ts`'s "renderSpectator over a %s log
  writes no sequence as text, onto the canvas or into the page" draws a world
  log, which only a DM's or an agent's window holds. Once the change lands it
  draws no panel unless asked for one (P2). Proposed, appended to "What it
  touches" item 1's tests: "and `client/test/qa-window-number.test.ts`, whose
  world-log render is a DM's window".
- **G4. The absence observations have nothing positive to stand on (check 2).**
  A page whose role is not yet known draws no panel once the change lands. So
  "no `.ticker`" holds before `/api/me` answers for any role, and an absence
  read at that moment proves nothing about the role.
  - Item 1, proposed: "… delivers events, and observes, once the role's own
    panel is drawn (`.player`, `.perch`), that the window has no
    `section.ticker` and no heading reading "Events", while the Story
    (`.feed`) shows those events."
  - Item 2, proposed: "… delivers events, and observes that the Story shows
    them and that the window has no `section.ticker`."
  - Item 3, proposed: "… observes, once the spectator's perch and the player's
    panel are visible, that neither window has a `.ticker`, and that the DM's
    window has one with at least one `.tick`."
- **G5. The ticket overturns a recorded decision without saying so (check 5).**
  Section 4 of the client design (`docs/superpowers/specs/2026-07-26-client-design.md`)
  gives "Everyone (spectator floor)" an "event ticker with sequence numbers".
  VTT-393 already overturned the sequence numbers. This ticket overturns the
  "Everyone". `client/src/view/spectator.ts`'s header points at that section
  (D4). Proposed, appended to "The problem": "This overturns, for the Events
  panel, section 4 of `docs/superpowers/specs/2026-07-26-client-design.md`,
  which gives "Everyone (spectator floor)" an event ticker; VTT-393 already
  overturned that section's "with sequence numbers"."

The ticket's own "What could not be established" is Q1 below.

## Established here, not in the ticket

- **E1. Thirteen tests go red, in three of the four test files the ticket
  names (P2).** One in `app.test.ts`, six in `spectator-view.test.ts` and six
  of the seven in `qa-move-reason.test.ts`'s group. Every other test stays
  green, `qa-window-number`'s included (G3).
- **E2. The Story already shows everything the panel shows a player.**
  `buildFeed` lists every entry except a narration as a `.mechanical` row
  through `describe`. Taking the panel away loses only the newest-first view
  and the panel's bare `narrationAdded` rows.
- **E3. `spectator.ts`'s header cannot be edited in place (P6).** Its first
  eight lines are one comment block, over `check:comments`' bound of 6, and the
  first line holds the banned term `spec §`. Its first sentence lists "event
  ticker" in the spectator floor, which stops being true once the change lands.
- **E4. `renderTicker`'s comment becomes false too:** "a spectator only ever
  looks at the recent tail".
- **E5. The stylesheet needs no change (D3).** `style.css` names a `ticker`
  area in each `#app` template. An area no element fills is an empty cell.
  On a player's page the `dm` row, which no element then fills, may show as
  one more gap between the panel and the notes. Reasoned from the template,
  not measured in a browser. The perch template's comment, "since the perch is
  the only panel a spectator has, this template names no others", stays true:
  it counts the role panels, and its template names `ticker` today too.

## Measurements this plan stands on

Every probe ran outside the repository, at `4fac397`.

| # | Probe | Result |
|---|---|---|
| P1 | `env -u CLAUDECODE bun test client/test`, untouched copy | 1302 pass |
| P2 | Prototype of D1-D2 alone, whole `client/test` | 13 red. `app.test.ts`: "no role's window shows a log sequence number". `spectator-view.test.ts`: "the ticker shows the newest event first", "each ticker row is its event's description alone, newest first", "the ticker is bounded — …", "the feed and the ticker show a move's reason", "each ticker row carries its description, under the Events heading", "the ticker section carries its class, and headings carry none". `qa-move-reason.test.ts`: six of the seven under "renderSpectator: the feed and the ticker" |
| P3 | A1-A3 and S1 as Task 1 writes them, on today's tree and on the prototype | today: A1, A2 and S1 red, A3 green. Prototype: all green |
| P4 | Breaks K1-K7 applied one at a time to the prototype, A1-A3 run; S1 run under K1 and K7 | see Task 1's breaks. Each goes red where it says, and A3 stays green under every break but K5 and K6 |
| P5 | Prototype plus D4 and D5, `qa-window-number.test.ts`'s edit included, whole `client/test`, typecheck, `check-ts-coverage.py` | 1306 pass (1302 plus A1-A3 and S1, as Task 1 writes them; re-measured by the Phase 4b reviewer). `bunx tsc --noEmit -p client/tsconfig.json` clean. `app.ts` and `spectator.ts` 100.00%, exit 0 (coverage measured before the `qa-window-number` edit, which touches no source) |
| P6 | `check-comments.py`'s `measure`, `--report`, and `main` after staging; `check-new-prose.py main` | the comment budget below. Both gates clean with D4's edits |
| P7 | `bunx vite build --config client/vite.config.ts` on the prototype | only `cmd/vtt/webdist/assets/index.js` changes |
| P8 | grep | `renderSpectator(` is called from `app.ts` and five test files (`qa-move-reason`, `qa-note-console`, `qa-window-number`, `spectator-view`, `visibility`); `qa-note-console` and `visibility` read no panel. `renderTicker` is called only in `renderSpectator`. Under `client/e2e`, `.ticker` and `.tick` are read only in `table.spec.ts`. No Go code reads them. No specification but SPEC-022 describes the window, and no ADR mentions the ticker. `tools/ts-mutation-equivalents.txt` holds two `spectator.ts` keys, both in `renderPerch`'s comparator, and none for `app.ts`. `stryker.conf.json` mutates both files |
| P9 | A failing `expect(element).toBeNull()` under bun | 20-30 s to print the element (A1 30.3 s, A2 20.1 s, today's tree) |

Not measured: the browser e2e (a verifier constraint), the mutation run, and
the page's layout in a browser.

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no floor, ceiling or exemption moves. `tools/comment-ceilings.txt`,
    `tools/ts-coverage-thresholds.txt` and `tools/ts-mutation-equivalents.txt`
    are not touched. `app.ts` and `spectator.ts` are held at 100% TS coverage.
  - 3: the contract does not change.
  - 4 and 5: no fold and no game-system word is added.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment is a warning, a pointer, or an exported symbol's one-line
    doc. No date, hash or task goes beside the code.
- **The comment budget (P6).** Compared unrounded. Run
  `python3 tools/check-comments.py main` after staging.

  | File | main | After D4 | Ceiling |
  |---|---|---|---|
  | `client/src/view/spectator.ts` | 268 of 580, 46.21% | 265 of 578, 45.85% | 46.3 |
  | `client/src/app.ts` | 262 of 529, 49.53% | 262 of 530, 49.43% | 49.7 |

  With the field's doc as its only comment edit, `spectator.ts` stands at
  46.22%, with room for no other added comment line. D4's two deletions are
  what make room for its rewording. After D4 it is 0.45 under the ceiling,
  inside the 1.0 band, so the ledger needs no `--write-ledger`.
- **Ruling B-3.** No player's or spectator's window draws the panel, and the
  Story is not changed: `git diff main -- client/src/view/feed.ts` is empty
  and `renderFeed` is not edited.
- **A near-theoretical edge case with small consequence is no debt.** A role
  string `/api/me` never sends gets no test of its own: `isDM` names two roles,
  and every other string fails it the way `player` does.
- **Requirement ids** come from `requirement-id` after sign-off, never by hand
  (SPEC-008). A test or spec file that cites an id needs the row first. An e2e
  spec named as evidence must carry the id in a `// VTT-NNN` line.
- **Tier 1 runs at pre-commit:** `bun test client/test contract contract-spike`
  and `task client:typecheck`. Source, tests and bundle go in one commit.
- **bun is quiet under `CLAUDECODE`.** Run `env -u CLAUDECODE bun test …`.
- **The bundle.**
  - The commit that changes `client/src` carries `cmd/vtt/webdist`, rebuilt by
    `task build:client`. The commit hook does not catch a stale bundle.
  - `check:drift` passes only after the commit. Pre-push runs it.
  - `task e2e` serves whatever webdist is in the tree.
  - Check the machine load before `task e2e` (`os.getloadavg()`).
- **TS mutation.** Both files are mutated (P8). No adjudication sits on code
  this plan edits, and none is expected: the one new conditional's two mutants
  are K1 and K6.
- **`review-record` refuses untracked files.** Commit the ticket and this plan
  in Task 0.
- **Mechanics.**
  - A new test goes after a closing brace.
  - Stage and commit in separate calls.
  - Undo a break with its inverse edit and check with grep, never with
    `git checkout --`.

## Rule 9: how MapTool does this

Checked read-only in `~/dev/RPTool/maptool`, at `f4b7fef6c`:

- **What MapTool does.**
  - `MapToolFrame` builds the GM macro frame (`MTFrame.GM`, a scroll pane
    around its `GmPanel`) for every client, beside the Global, Campaign,
    Selection and Impersonated frames. Nothing in that code checks the role.
    Whether the default layout shows the frame to a player was not checked.
  - `GmPanel.init` fills the frame with the campaign's GM macro buttons
    (`Campaign.getGmMacroButtonPropertiesArray`) only when
    `MapTool.getPlayer() == null || MapTool.getPlayer().isGM()`. A player's GM
    frame is therefore present and empty. A client with no player yet fills
    it: the check fails open.
  - MapTool keeps no event log for anyone. A search of `src/main/java` finds
    no event-log, ticker or activity-log class. Its chat (`CommandPanel`,
    `MessagePanel`) carries text, rolls and macro output. Every other client
    is sent each message that is not ME-only, and `MapTool.addServerMessage`
    drops a GM, GM-me,
    not-GM or whisper message on a client it is not for: a second client-side
    role gate.
- **What we borrow.** The client gates a GM-only panel on the role, in one
  predicate: `isDM` in `paint`, as `isGM` in `GmPanel.init`.
- **What we do not borrow.**
  - The null player falling open. A page here draws no panel until `/api/me`
    has named the role `dm` or `agent`. K2 is MapTool's shape, and A2 catches
    it.
  - The empty frame. Other roles get no section and no heading at all, not an
    empty panel.
- **The distribution, which rule 9 always excludes, is not in play.** The panel
  shows only the log the client holds, which the gateway already chose
  (SPEC-015, SPEC-022), and the Story shows the same entries. Hiding the panel
  withholds nothing a window is sent. It is a choice about the window, not a
  boundary, which is the one place a client-side role filter is the right
  shape.
- **Later, by the owner's ruling:** a macro button whose attack carries text
  and mechanics, as MapTool's chat shows macro output.

## Design question for the owner

**Q1. Does "the DM" include the agent?** It is the ticket's open point.
Answered 2026-10-23: yes, as the console ("Ja, som DM-konsolen").

- **Recommended: yes.** The panel follows `isDM`, which already gives the
  console to `dm` and `agent`.
  - SPEC-022 gives both the world. SPEC-015 sends neither a
    `ViewpointChanged`.
  - The owner's ruling of 2026-10-15: a table has one DM in practice, a
    person or the agent.
  - The agent plays through `vtt mcp`, so an agent's browser window is rare,
    and either answer costs little.
- **Alternative: the `dm` role alone.** D2's predicate becomes
  `me !== null && me.role === "dm"`, A3's agent half asserts no panel, and
  R1 and R2 change their wording.

## Decisions

1. **D1. The panel is a flag on `ViewExtras`, and an absent flag draws none.**
   - `ViewExtras.ticker?: boolean | undefined`. `renderSpectator` pushes
     `renderTicker(log)` only when it is set.
   - Forced by the ticket's rule, "a window the client knows to be". The
     default `{}`, which every caller that names no role gets (a page before
     `/api/me` answers, a test), must draw none. That is the opposite of
     MapTool's null player (rule 9).
   - Named `ticker` after the section's class and its renderer.
   - Rejected: `ticker?: HTMLElement`, built by `app.ts` as the panel and the
     console are. It exports `renderTicker` and hands the log over twice.
   - Rejected: `renderSpectator` learning the role. It knows no role today, and
     the role logic lives in `paint`.
2. **D2. `paint` in `startSession` passes `ticker: isDM`.**
   - The predicate the console already uses. No second definition of "a DM's
     window".
   - Forced by Q1's recommendation. Q1's alternative changes this line only.
3. **D3. A DM's window is node for node what it is today, and `style.css` is
   not touched.**
   - The push goes straight after the array that ends with the notes, before
     the perch, the panel and the console. A DM's and an agent's nodes keep
     their order.
   - The `ticker` grid areas stay (E5). Filling them with the Story would
     change the Story, which B-3 keeps. A preference, given the ruling.
4. **D4. Three comment edits in `spectator.ts`, and none in `app.ts`.**
   - The field gets one one-line doc: `/** Whether the Events panel is drawn:
     for dm and agent only (SPEC-022). */`.
   - `renderTicker`'s two lines become one warning: `// Keep it bounded: an
     all-night session is thousands of events.` (E4).
   - The header's first sentence and the `//` line after it are deleted (E3,
     G5). A deletion adds no line, so neither the block bound nor the banned
     term applies. The "Deliberately thin." paragraph stays.
   - All three are forced by false prose and the gate (P6). `ticker: isDM,`
     needs no comment.
5. **D5. A test that reads the panel draws a DM's window.**
   - `spectator-view.test.ts`'s `render` gains a third parameter,
     `extras: ViewExtras = {}`, and its six tests that read the panel pass
     `{ ticker: true }`. Every other caller is unchanged.
   - `qa-move-reason.test.ts`'s `render` passes `{ ticker: true }`. Its group
     "renderSpectator: the feed and the ticker" holds seven tests, six of which
     read the panel, and the feed does not depend on the flag.
   - `qa-window-number.test.ts`'s world-log render passes `ticker: true` when
     `kind === "world"` (G3).
   - `app.test.ts`'s "no role's window shows a log sequence number" asserts
     six `.ticker .tick` for `dm` and `agent`, no `section.ticker` for
     `spectator` and `player`, and five `.feed .mechanical` for all four. The
     six entries are five events and one narration. Its id line becomes
     `// VTT-393 VTT-445 VTT-446`.
   - Forced by the ticket's item 4: "each reading only panels that role's
     window has". No test is renamed, so VTT-263's and VTT-393's register
     cells stand.
6. **D6. An absence is read only after the role is known, and as a count.**
   - Unit tests read `querySelectorAll(…).length` as 0 once the role's own
     panel is drawn, or, for a page with no role, once the Story shows the
     events (G4). Not `toBeNull()` on an element, which takes 20-30 s to fail
     (P9).
   - The e2e reads `toHaveCount(0)` once `.perch` or `.player` is visible.
7. **D7. The e2e keeps its test names.** "the story feed and ticker record
   what happened" stays VTT-393's browser evidence. It reads the Story in the
   player's window and the panel in the DM's.

## Requirements

Allocated by `requirement-id` at Task 0, after sign-off, and written
`**OPEN — no test yet**`. The sort ran over the ticket's rule and the
verifier's candidates R1 and R2.

- **VTT-445 (R1).** "A browser window draws no Events panel unless the role
  the client last read for its participant is `dm` or `agent`."
  - Evidence: A1, A2, S1, `app.test.ts`'s "no role's window shows a log
    sequence number" as D5 edits it, and `table.spec.ts`'s "a spectator sees
    the table and no controls" and "the story feed and ticker record what
    happened".
  - Goes red under K1-K4.
- **VTT-446 (R2).** "A browser window whose participant's role the client last
  read as `dm` or `agent` draws the Events panel, except while it shows the
  reason it cannot fold its log."
  - The exception is `paint`'s: a log that cannot fold replaces the whole
    window with its reason (`p.fatal`) before `renderSpectator` runs, so the
    window draws neither the panel nor the Story, as
    `qa-window-number.test.ts`'s "%s: a log that cannot fold shows the reason,
    and no sequence outside it" reaches for `dm` and `agent`. VTT-393 carries
    the same exception.
  - Evidence: A3, `app.test.ts`'s "no role's window shows a log sequence
    number" as D5 edits it, and `table.spec.ts`'s "the story feed and ticker
    record what happened" (its DM half).
  - Goes red under K5 and K6. Today no row says the DM has the panel.
  - Reworded before its first commit, while nothing cited it: its first
    wording, "… draws the Events panel.", was false for a window whose log
    cannot fold (Phase 4b). VTT-445 was reworded with it from "has read its
    participant's role as" to "the role the client last read", which is what
    `paint` reads.
- The sort refused:
  - R2's "at most 40, newest first": behaviour this ticket does not change,
    held by `spectator-view.test.ts`'s "the ticker shows the newest event
    first" and "the ticker is bounded — an all-night session must not render
    thousands of rows", and a figure belongs in the test, not the rule.
  - "The Story is unchanged": a non-change, held by the empty diff and by A1's
    and A2's Story half (K7).
  - The flag's name, the DOM order and the stylesheet (D1, D3): how, not what.
  - "The panel is no boundary" (rule 9): nothing to observe.
  - The shoulder e2e reading the Story: a test's mechanics.
- **Existing rows** are not reworded. VTT-263 holds wherever a panel is drawn,
  and VTT-393 holds for every window.

| Ticket item | Observed by |
|---|---|
| 1 (player and spectator, no panel, Story shows) | A1, and the edited VTT-393 test |
| 2 (`/api/me` does not answer) | A2 |
| 3 (e2e) | `table.spec.ts`: "a spectator sees the table and no controls", "the story feed and ticker record what happened" |
| 4.1 (DM and agent keep it) | A3, `spectator-view.test.ts`'s six with `{ ticker: true }` |
| 4.2 (VTT-263, VTT-393) | their tests, edited by D5 and Task 1 |
| 4.3 (Story unchanged) | `git diff main -- client/src/view/feed.ts` empty; A1's and A2's Story half |
| 4.4 (the shoulder e2e reads the Story) | `table.spec.ts`: "a spectator chooses a shoulder and is shown that log", green on the new bundle |

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off on this plan and an answer to Q1.
  - Allocate R1 and R2 with `requirement-id` (VTT-445, VTT-446), which writes
    their rows to `docs/requirements.md` with OPEN evidence. Then commit the
    ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and the two rows exist.

### Task 1: the window draws the panel for the DM alone (R1, R2, items 1-4)

- **Files:**
  - `client/src/view/spectator.ts` (`ViewExtras`, `renderSpectator`,
    `renderTicker`, the header) and `client/src/app.ts` (`paint` in
    `startSession`);
  - `client/test/app.test.ts`, `client/test/spectator-view.test.ts`,
    `client/test/qa-move-reason.test.ts`,
    `client/test/qa-window-number.test.ts`;
  - `client/e2e/table.spec.ts`;
  - `cmd/vtt/webdist/` from `task build:client`;
  - `docs/requirements.md` (R1's and R2's evidence).
- **E2e changes in `table.spec.ts`,** each test gaining the id lines of the
  rows that name it:
  - "a spectator sees the table and no controls": wait for `.perch` to be
    visible, then `.ticker` `toHaveCount(0)` in place of `toBeVisible()`.
  - "a spectator chooses a shoulder and is shown that log": on the shoulder,
    `.feed .mechanical` is not empty where `.tick` was, and `.ticker` has count
    0. After "No shoulder", `.feed .mechanical` has count 0 and `.feed .empty`
    is visible.
  - "the story feed and ticker record what happened":
    - As the player, once `.player` is visible: `.ticker` has count 0;
      `.feed .mechanical` is not empty, holds one row matching
      `/^tok-lera placed at \d+,\d+$/` and none matching `/#\d/`; and no
      `.feed .mechanical` row draws `::before` or `::after` content.
      Screenshot `04-player-feed`.
    - Then a DM page in a new browser context, once `.dm` is visible:
      `.ticker .tick` is not empty, none matches `/#\d/`, and none draws
      `::before` or `::after` content. Screenshot `04d-dm-ticker`.
- **Unit tests,** each with the id lines of the rows that name it:
  - **A1** in `app.test.ts`, after the last test: "a player's and a
    spectator's window draws no Events panel, and its Story shows what it was
    sent". For `player` and for `spectator`: `dialAs(role)`,
    `opensOn(sock, "a1")`, `seedTable(sock)`, `settle()`. The role's panel
    (`.player`, `.perch`) is drawn. `.feed .mechanical` has 5 rows,
    `section.ticker` has 0, and no `h2` reads "Events". Red today (P3).
  - **A2** "a page whose /api/me answers no role draws no Events panel, and its
    Story shows what it was sent". `stubMetadata({})`, `boot`, open,
    `seedTable`, `settle`. `.feed .mechanical` has 5 rows and
    `section.ticker` has 0. Red today.
  - **A3** "a DM's and an agent's window draws a row in the Events panel for
    each entry it was sent, newest first". For `dm` and `agent`:
    `tableAs(role)`, and `.dm` is drawn. The `.ticker .tick` texts equal the
    `.feed .mechanical` texts reversed, and the first reads
    "t1 placed at 1,1". It compares the two lists rather than spelling out
    labels, so a later rewording of `describe` (A-2) leaves it standing. Green
    today, as a "still true" item must be.
  - **S1** in `spectator-view.test.ts`, after "the ticker section carries its
    class, and headings carry none": "a window not asked for the Events panel
    draws none, and its Story still shows the log". `render(world(), log)`
    with two events: `section.ticker` has 0, no `h2` reads "Events", and the
    `.feed .mechanical` texts are the two `describe` labels. Red today.
- **Code:** D1-D4.
- **Order:**
  1. Write the e2e changes and run `task e2e` on main's bundle. Expect red on
     "a spectator sees the table and no controls" (the panel is there), on the
     shoulder test's `.ticker` count, and on the VTT-393 test's player
     `.ticker` count. Expect green on everything else. This is item 3's "fails
     on today's tree".
  2. Write A1-A3 and S1. A1, A2 and S1 go red, and A3 stays green.
  3. Make the code change. The 13 tests of P2 go red.
  4. Edit them as D5 says, and edit `qa-window-number.test.ts`.
  5. Run `task build:client`. Then every client test, `task client:typecheck`,
     `task e2e`, and `check-comments.py main` and `check-new-prose.py main`
     after staging.
- **True when:** all of the above are green;
  `git diff main -- client/src/view/feed.ts` is empty; and `git status` lists
  the files above and `cmd/vtt/webdist/assets/index.js`, and no
  `assets/index.css`.
- **Breaks,** each applied, run, and undone by its inverse edit. Those that
  name the e2e are rebuilt and run there too, then undone and rebuilt. Each of
  K1-K7 ran against A1-A3 on the prototype, and S1 under K1 and K7 (P4). What
  the e2e and the edited existing tests do under a break is expected, not
  measured.
  - **K1:** `renderSpectator` pushes `renderTicker(log)` whatever the flag. A1,
    A2 and S1 go red. A3 stays green. In the e2e, the spectator and player
    halves are expected to go red.
  - **K2:** `ticker: me === null || isDM`, MapTool's null player. Only A2 goes
    red.
  - **K3:** `ticker: canAct`. A1 goes red on its player half. A2 and A3 stay
    green.
  - **K4:** `ticker: me !== null && me.role !== "player"`, so the panel leaks
    to the spectator alone. A1 goes red on its spectator half. A2 and A3 stay
    green.
  - **K5:** `ticker: me !== null && me.role === "dm"`. A3 goes red on its agent
    half. A1 and A2 stay green.
  - **K6:** the push is deleted. Fourteen go red (measured by the Phase 4b
    reviewer): A3, the six tests D5 flags in `spectator-view.test.ts`, the six
    in `qa-move-reason.test.ts` that read the panel, and `app.test.ts`'s edited
    "no role's window shows a log sequence number". The VTT-393 e2e's DM half
    is expected to go red too.
  - **K7:** the Story draws the log only with the flag
    (`renderFeed(buildFeed(extras.ticker ? log : []))`). A1, A2 and S1 go red
    on `.feed .mechanical`. A3 stays green.
- **Opposite direction:** A3, a DM's and an agent's correct window, stays green
  under K1-K4 and K7 (P4). A1's role-panel anchor is drawn under every break,
  so A1 goes red on the absence or the Story, never on the anchor.
- **Fixture check F1:** in A1, drop `/api/me` from the stub for one run. A1 must
  go red on the role-panel anchor, not pass on the absence. Undo it.

### Task 2: the gate, whole

- **Do:** run `task check` whole on Task 1's commit, the last that changes
  code, then `task e2e` on the same commit.
- **True when:** both are green, `check:drift` and `check:ts-mutation`
  included, and `check:requirements-chain` resolves R1 and R2.

### Task 3: the report

- **Do:** write the implementation report under `docs/reports/`, with its own
  reading review and its own commit. It covers:
  - each "Done looks like" item with its observation (the table above);
  - K1-K7 and F1, with what went red;
  - Q1's answer;
  - Phase 4a's adjudications under their own heading.

### Task 4: SPEC-022 (the specification step)

- **Do,** with the `specification` skill, last and from the report, in "What
  the window shows":
  - Every role's window shows the Story (`renderFeed`), except while it shows
    the reason it cannot fold its log.
  - Only a DM's and an agent's window shows the Events panel (`renderTicker`),
    with the same exception: the newest entries of the log it holds, newest
    first, one row each by `describe` (R2).
  - `paint` in `startSession` asks for it through `ViewExtras.ticker` once
    `/api/me` has named the role `dm` or `agent`. A page whose role is not yet
    known, or is any other, draws none (R1).
  - Cite the owner's ruling B-3 of 2026-10-22.
  - Add A1-A3, S1 and the e2e to the Status paragraph, and R1 and R2 to the
    Requirements list.
- **True when:**
  - `python3 tools/check-requirements-chain.py` passes on its commit, run by
    hand: Task 2's gate ran before this file changed, and no hook reads it.
  - Each new sentence has been run against the code by someone other than its
    writer, Phase 4b's reading included.

## For Phase 4a

Hand the QA agent:

- R1, R2, VTT-263 and VTT-393;
- the signatures `renderSpectator(root, st, log, status, extras?)`, the field
  `ViewExtras.ticker`, and `boot(root)`;
- that the panel is a `section.ticker` headed "Events" with one `.tick` per
  row, and that the role comes from `/api/me`.

Tell it:

- the page draws before `/api/me` answers, so an absence read before the role's
  own panel is drawn proves nothing (D6);
- a log that cannot fold replaces the whole window with its reason, a
  `p.fatal` reading "Cannot derive state from the log: …", which is R2's
  exception;
- bun runs every test file in one process, most boot tests share the token
  `tok`, and a boot test clears both storages in `beforeEach`;
- a failing `toBeNull()` on an element takes 20-30 s to print (P9);
- nothing in Go changes.

## Forward coupling

- **B-4 (the fourth ticket)** lets a player who controls no party member watch
  as a spectator does. The role stays `player`, so R1 already keeps the panel
  from them.
- **A-2** is the DM's panel and the Story printing bare payload names
  (`narrationAdded`, `actorControlGranted`). Unchanged here. A3 compares the
  panel with the Story, so a rewording leaves it standing.
- **The macro button** that carries text and mechanics is later, by the
  owner's ruling.
