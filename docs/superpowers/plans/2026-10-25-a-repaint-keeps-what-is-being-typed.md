# Plan: a repaint keeps the box being typed in, and no adjudication excuses a mutant a test can see

- **Ticket:**
  `docs/superpowers/specs/2026-10-25-a-repaint-keeps-what-is-being-typed-design.md`,
  from the open debt "A repaint takes the focus from the box the DM is typing
  in" in `docs/verification-debt.md` and the false adjudication on `input()`'s
  `if (cls) i.className = cls;` in `tools/ts-mutation-equivalents.txt`.
- **Verdict:** passes, with gaps G1 to G6 (below).
  - Verified 2026-10-25 by an agent that did not write the ticket, read-only
    against HEAD `d1de412`.
  - Probes ran in a scratch directory outside the tree, none committed: a
    happy-dom probe through the views, a mutated copy of HEAD's client and
    contract, the two browser probes the open debt records, and a Chromium
    probe with no server.
  - `task check`, `task e2e` and the mutation gates were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-25-a-repaint-keeps-what-is-being-typed.md`.
- **Base:** main `d1de412`.
- **Branch:** `fix/a-repaint-keeps-what-is-being-typed`.
- **Owner's rulings taken in, all 2026-10-25:**
  - "falska adjudikeringen och fokusdefekten ska åtgärdas": both are fixed.
  - "Återställ fokus":
    - before each repaint, the focused box is noted by its `data-field` and
      its selection;
    - after it, the new box with the same field is focused and the
      selection restored;
    - one shared helper, used by `renderSpectator` and `renderJoinView`;
    - the wholesale rebuild stays.
  - "Konsol, Say och join":
    - the DM console, for the focus;
    - the player panel's Say boxes, for the text, kept like the console's
      draft, and for the focus;
    - the join view, for the focus.
  - "Ett ärende, en gren": one ticket, one branch, the adjudication task
    first, then the focus.
  - "Efter rapport och specar", given for the last ticket: the report and the
    specification are written and reviewed before one `task check` and one
    `task e2e` (Design questions).
- **Signed off:** 2026-10-25, "Ja, kör", with the design question answered
  "Efter rapport och specar".
- **Amended after sign-off,** by two answers of the owner's on 2026-10-25
  to Task 0's review:
  - "Ja, preventScroll": the box gets its focus back without scrolling the
    page (D4, C4, F4, K13, W1);
  - "Lägg till overflow-anchor: none": the page does not move when the box
    is above the viewport either (D10, K14, W3).

## The gaps this plan inherits

- **G1. The Say boxes' "until Send sends it" has no observation.** The
  ticket's second rule has two halves, and its "Done looks like" observes only
  the first: the text survives a repaint. Nothing observes that a Send empties
  the next repaint's boxes. S2 is the plan's, with break K9.
- **G2. The third rule cannot hold as worded.** "A repaint moves the focus to
  no element that did not hold it before" fails for the fix itself: the box
  the repaint focuses is a new element, which never held the focus. Item 9
  says what is meant, and candidate C2 states that. The ticket's writer
  revised the rule to C2's sense after this verification.
- **G3. `client/src/app.ts` is not in "What it touches".** `runJoin`'s
  comment "NO repaint on a keystroke: rebuilding the input mid-typing throws
  away the caret" gives a reason that is false once a rebuild keeps the caret.
  Task 2 carries it (D8). The writer added it to the ticket's list after
  this verification.
- **G4. Only Chromium is known.** The ticket's first open point is settled for
  Chromium by P8: an input and a textarea both get back the range and the
  direction. Firefox and WebKit were not run, and `playwright.config.ts`
  names no project, so the e2e runs Chromium alone.
- **G5. Playwright's `fill` stays open.** Whether `fill` keeps its selection
  when the focus comes back between its select and its insert was not
  established. W1 inserts after the repaint instead, as the ticket says.
- **G6. Item 2's second half is observed only at the gate.** "`check:ts-mutation`
  passes with that mutant counted killed" needs Stryker, which runs only in
  Docker on this machine. Nobody has run it for this plan. P6 shows that no
  test kills the mutant today, and that the new assertion kills it.

## Measurements this plan stands on

All at `d1de412`, outside the repository.

| # | Probe | Result |
|---|---|---|
| P1 | happy-dom: `renderSpectator` with a `renderDMConsole` console; `session-name` focused, `Open` typed with an input event, 1 to 3 selected; repaint | `document.activeElement` is `body`; the new box holds `Open`, unfocused, with the caret at 4 to 4 |
| P2 | happy-dom: `renderJoinView`; `join-name` focused, `Ann` typed; the busy repaint, then the refusal's | `body` after each; the box holds `Ann` |
| P3 | happy-dom: `renderSpectator` with a player's `renderPlayerPanel`; both Say boxes typed, `text` focused; repaint with the same `ui` | neither box has a `data-field`; both are empty; `body` is focused |
| P4 | happy-dom: a focused input removed by `replaceChildren`; and a focused input outside the root across a repaint | the first leaves `body` focused; the second stays focused |
| P5 | happy-dom: an input with no `className` assignment beside one assigned `""` | `className` is `""` for both; `hasAttribute("class")` is false, then true |
| P6 | A copy of HEAD's `client/src`, `client/test` and `contract`, with `if (true) i.className = cls;` | `client/test/dm-view.test.ts`: 112 pass under the mutant. The whole client suite fails the same seven tests with and without it: they read files outside the copy. With `hasAttribute("class")` asserted false: red under the mutant, green without it |
| P7 | The debt's two browser probes, against a binary built from the tree | DM: types `Open`, a narration arrives, then ` Table`: the box holds `Open` and `body` is focused. DM: `focus()`, a narration, `insertText("Open Table")`, Start session: "a session needs a name", `sessions: 0`. Player: types `I draw my`, the DM's narration arrives: the box is empty and `body` is focused |
| P8 | Chromium, no server: a tree replaced wholesale; the new box with the same `data-field` focused and handed the old box's range | input and textarea both get back 1, 4, `backward`, and are focused. A keystroke after the restore lands in the new box: `Open` then ` Table` reads `Open Table` |
| P9 | happy-dom primitives | `root.querySelector("input[data-field]:focus, textarea[data-field]:focus")` finds the focused box, and is null with a button focused. A box detached by `replaceChildren` keeps `selectionStart`, `selectionEnd` and `selectionDirection` |
| P10 | Comment shares, by `check-comments.py`'s own `measure` | the budget table under Constraints |
| P11 | Callers, by grep over `client/src`, `client/test` and `client/e2e` | `renderSpectator`: `paint` in `startSession` and 9 test files. `renderJoinView`: `runJoin`'s `paint` and `client/test/join.test.ts`. `renderPlayerPanel`: `paint` and 6 test files. `PlayerUIState` literals: `startSession`'s `ui` and 13 in 6 test files. `root.replaceChildren` in `client/src`: `renderSpectator`, `renderJoinView`, and `paint`'s fold-failure branch. No test reads `activeElement`, `focus()` or a selection |
| P12 | Chromium, Task 0's review: a box below the fold of a tall page, the page at the top, focused with a plain `focus()` after a wholesale replace | scrollY 0 → 1437; with `preventScroll`, 0 → 0 |
| P13 | Chromium, Task 0's review, HEAD's binary and bundle, the helper emulated in the page on `#app`'s `replaceChildren`; the DM focuses `session-name`, scrolls it out of view, another connection adds a narration | box above the viewport: HEAD 408 → 198, the fix 460 → 0, a plain `focus()` 553 → 328. The same with `overflow-anchor: none`: HEAD 398 → 398, the fix 491 → 491, a plain `focus()` 584 → 511. Box below the fold, page at the top: HEAD 0 → 0, the fix 0 → 0, a plain `focus()` 0 → 440. The jump moved inside `replaceChildren`, before any focus call; with nothing focused, a repaint at 420 stayed at 420 |

Not measured:
- `task check`, `task e2e` and both mutation gates;
- Firefox and WebKit (G4);
- the helper itself, which was probed only as primitives (P8, P9).

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no ceiling, floor or exemption rises.
    - `tools/comment-ceilings.txt` rows only fall, by
      `python3 tools/check-comments.py --write-ledger`. Run it once per task,
      last, after staging: a row it lowers cannot come back up, even to HEAD's
      value.
    - Neither mutation adjudication file gains a row for code this plan
      writes. A survivor on new code is fixed in the code or the tests.
      Removing the false row is item 2's.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `client/src` is a warning, a pointer or an exported
    symbol's one-line doc, with no date, hash or task beside the code.
- **The comment budget (P10).** Shares are compared unrounded. Run
  `python3 tools/check-comments.py main` after staging, because a line moved
  between files shows only then.

  | File | HEAD | Ceiling | Room |
  |---|---|---|---|
  | `client/src/app.ts` | 262 of 538, 48.70% | 48.7 | none: no comment line may be added |
  | `client/src/view/spectator.ts` | 259 of 633, 40.92% | 41.0 | one comment line at most, once the import is in |
  | `client/src/view/join.ts` | 24 of 70, 34.29% | 34.3 | none: one added comment line puts it at 34.72%, once the import is in |
  | `client/src/view/player.ts` | 54 of 234, 23.08% | 23.1 | about three code lines per comment line |
  | `client/src/view/dm.ts` | 264 of 684, 38.60% | 38.7 | unchanged by this plan |

  - **A new file gets its row only at or under 25.0%.** `--write-ledger`
    writes no row for a file above that default, and the gate refuses a file
    with no row above it.
  - **A file more than 1.0 under its ceiling is refused** until
    `--write-ledger` lowers its row. `player.ts` may end there in Task 3.
- **A new client file needs two rows.** `client/src/view/focus.ts` needs one
  in `tools/comment-ceilings.txt`, written by `--write-ledger`, and a floor in
  `tools/ts-coverage-thresholds.txt`, written by hand at 100.00, where every
  gated client file sits. The last ticket's gate stopped at
  `check:ts-coverage` because its plan named only the first. No hook runs
  either check: run `python3 tools/check-ts-coverage.py
  tools/ts-coverage-thresholds.txt` by hand.
- **The adjudications.** SPEC-020 keys a row by the code it sits on.
  - Keep byte-identical the lines of `spectator.ts`'s two rows, `renderPerch`'s
    two sort comparisons.
  - `dm.ts`'s other four rows stay, since this plan does not edit `dm.ts`.
  - Run both self-tests before the gate: `python3
    tools/check_ts_mutation_test.py -q` and `python3
    tools/check_mutation_test.py`.
- **Stryker does not run natively on macOS 26 here** (the `check:ts-mutation`
  task's own description, which offers `VTT_FORCE_TS_MUTATION=1` on a fresh
  boot only). `task check` runs it in Docker when the client inputs change,
  which they will, so start Docker Desktop first. Expect about an hour.
- **Write no guard that no test can reach.** Each one is an equivalent mutant
  this plan may not adjudicate. Examples:
  - `document.activeElement` is never null in a document with a body;
  - `selectionDirection` is never null on an input with no `type` or on a
    textarea;
  - a box a query has just found is not undefined.

  Narrow by type instead, as `mayShare`'s comment in `startSession`'s file
  argues.
- **Requirement ids** come from
  `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/requirement-id`
  after sign-off, never by hand (SPEC-008).
  - A test that cites an id needs the row first.
  - An e2e test named as evidence carries the id in a `// VTT-NNN` line.
- **Tier 1 runs at pre-commit:** `go test` tier 1, `bun test client/test
  contract contract-spike`, `task client:typecheck`, lint, vet, arch,
  vocabulary, doc-owner, secrets and the review gate. `check:comments`,
  `check:ts-coverage`, `check:new-prose` and `check:requirements-chain` run
  only in `task check`, so run each by hand before a commit.
- **bun is quiet under `CLAUDECODE`.** Run `env -u CLAUDECODE bun test
  client/test contract contract-spike`.
- **`check-new-prose.py` reads Go and TypeScript, test files included.** Its
  85-column band and its SHORT rule hold `client/test` and `client/e2e`. Run
  `python3 tools/check-new-prose.py main` before each commit.
- **A NodeList needs `Array.from`.** `client/tsconfig.json`'s `lib` has no
  `DOM.Iterable`: bun runs a `for…of` over a NodeList, and
  `task client:typecheck` refuses it.
- **`dm.ts`'s `draft` is module state, and bun runs every test file in one
  process.** A test that types into a console box must leave its draft empty
  when it ends, by sending the command the box fed or by an input event
  carrying `""`. The second leaves `draft[field]` at `""`, which hides a
  describe box's present text in later files, so use it only on a box with
  no present text. Otherwise the text reaches later files:
  `client/test/dm-view.test.ts`'s "starting a session without a name is
  refused before anything is sent" would find `session-name` filled.
- **The bundle.**
  - Each commit that changes `client/src` carries `cmd/vtt/webdist`, rebuilt by
    `task build:client`. The commit hook does not catch a stale bundle.
  - `check:drift` passes only after the commit. Pre-push runs it.
  - The e2e builds its binary from the working tree's `cmd/vtt/webdist`, so a
    spec sees a source change only after `task build:client`.
- **One browser run at a time.** The e2e specs share a state file in the
  system temporary directory with any probe. Never run two at once.
- **Pushing** is on the owner's word. The pre-push hook takes about three
  minutes. Check the load with `os.getloadavg()` first, write the whole output
  to a file, and never bypass the hook.
- **`review-record` refuses untracked files.** Commit the ticket and this plan
  in Task 0.
- **Mechanics.**
  - Stage and commit in separate calls.
  - Undo a break with its inverse edit and check it with grep, never
    `git checkout --`.
  - Hash the tree before and after any agent that reads it.

## Rule 9: how MapTool does this

Checked read-only in `~/dev/RPTool/maptool`, at `f4b7fef6c`, in
`CommandPanel`
(`src/main/java/net/rptools/maptool/client/ui/commandpanel/CommandPanel.java`).

- **What MapTool does.**
  - `getCommandTextArea()` builds `commandTextArea`, a `JTextPane`, once,
    behind `if (commandTextArea == null)`, and returns that one instance from
    then on.
  - An arriving message goes to a separate component:
    `onChatMessageAdded` calls `addMessage`, which calls
    `messagePanel.addMessage`. Nothing touches the text area when a message
    arrives.
  - `commitCommand` reads the text, commits it, then clears the area with
    `setText("")`.
  - So MapTool never meets the problem: Swing keeps the component, and with
    it the focus and the caret.
- **What we borrow.**
  - The effect: the box being typed in keeps its focus and its caret while
    the table changes around it.
  - Clearing the box when what it holds is sent (D6).
- **What we do not borrow.**
  - The structure, a component kept alive across updates. This client
    rebuilds its tree wholesale by design, and the owner's ruling keeps that.
    So the rebuild has to carry the focus across instead.

## Design questions for the owner

1. **The order of the last three tasks.** This plan follows "Efter rapport
   och specar", which the owner gave for the last ticket:
   - the report and SPEC-022 are written and reviewed first;
   - then one `task check` and one `task e2e` run on the last code commit;
   - their figures go into the report before it is committed, and the
     specification is committed last.

   Answered at sign-off: "Efter rapport och specar". Tasks 4 to 6 keep that
   order.

Nothing else is open.

## Decisions

1. **D1. The false entry leaves in the same commit as the assertion that
   kills its mutant, after a by-hand kill, and the dm.ts section's prose is
   corrected with it.**
   - Forced by three things:
     - a stale entry fails `check:ts-mutation` (`check-ts-mutation.py`'s
       stale loop sets `fail`);
     - SPEC-020: an entry with no survivor is removed only once a test is
       seen to kill the mutation made by hand;
     - the `client/src/view/dm.ts` section header, "166 survivors -> 1",
       counts this entry, and no gate reads the prose between keys.
2. **D2. One helper, `replaceKeepingFocus(root, ...nodes)`, in a new module,
   `client/src/view/focus.ts`.** It replaces the `root.replaceChildren` that
   ends `renderSpectator` and the one that ends `renderJoinView`.
   - Forced by the ruling "Återställ fokus": one helper, both callers, the
     rebuild kept.
   - It gets a module of its own because the two callers share none.
     `join.ts`'s header keeps the join form off the shared `el()` helper, and
     importing from `spectator.ts` would tie the join form to the table's
     renderer.
   - `paint`'s fold-failure branch keeps its bare `root.replaceChildren()`.
     It draws the reason alone, with no box, so there is nothing to restore.
3. **D3. A box is an `input` or a `textarea` carrying a `data-field`.**
   - The focused one is found before the replace, scoped to root:
     `root.querySelector("input[data-field]:focus, textarea[data-field]:focus")`
     (P9).
   - Its counterpart is the new tree's box whose `dataset.field` equals the
     old one's, compared over `Array.from(root.querySelectorAll(...))`. It is
     never found through a selector built from the field.
   - Forced by:
     - grep: every input and textarea in `client/src` carries a `data-field`,
       except the two Say boxes, which gain one (D5). The selects carry none
       and are outside the ticket;
     - `describe-name:${id}` and its siblings in `renderDMConsole` embed a
       token id. The engine checks only an id's length (`maxIDBytes`, 1 to
       128 bytes, in `internal/engine/apply.go`), not its characters, so a
       selector built from one would need escaping, a second rule to get
       right;
     - the constraint against unreachable guards: the scoped `:focus` query
       answers "a box of root holds the focus" with no null check on
       `document.activeElement`.
4. **D4. The selection restored is the old box's `selectionStart`,
   `selectionEnd` and `selectionDirection`, read before the replace. The new
   box gets `focus({ preventScroll: true })`, then `setSelectionRange`.**
   - Forced by:
     - the ruling, which restores the selection;
     - P8 and P9: Chromium and happy-dom both carry all three;
     - the owner's answer "Ja, preventScroll". Task 0's review measured in
       Chromium that a plain `focus()` on a box out of view scrolls the page
       to it, from 0 to 1437 on a tall page, while `preventScroll` leaves it
       at 0 (P12). Without it, a repaint at a live table would pull the page
       down to a box below the viewport for everyone who scrolled up while
       it held the focus. A box above the viewport is D10's.
   - happy-dom observes no scroll, so F4 holds the option with a spy on
     `HTMLElement.prototype.focus` that records its argument, as the
     `client/src/app.ts` section of `tools/ts-mutation-equivalents.txt`
     records `history.replaceState`'s; it kills the literal's ObjectLiteral
     and BooleanLiteral mutants. W1 holds the scroll itself.
   - No box in `client/src` sets a `type`, so `setSelectionRange` cannot throw
     on any of them.
5. **D5. The Say boxes carry `data-field` `say-as` and `say-text`.**
   - Forced by D3, which finds a box by its field.
   - The names must differ from every field in `renderDMConsole`:
     `startSession`'s `paint` draws the panel beside the console for a DM,
     because `canAct` admits one.
6. **D6. The kept text lives on `PlayerUIState`, as two optional fields,
   `sayAs` and `sayText`.**
   - Each box's `input` listener writes its field. The box shows
     `ui.sayAs ?? ""` or `ui.sayText ?? ""`.
   - Send still reads the boxes, then sets both fields to `""`. That is the
     screen Send leaves today, since its `rerender` rebuilds `as` empty.
   - Forced by:
     - the ticket, which names `PlayerUIState`;
     - `startSession`'s `ui`, the one object every `paint` hands the panel,
       which already carries the panel's selection across repaints;
     - the precedent of the 2026-10-24 plan's D6: optional fields spare
       literals that would change for no behaviour. Here that is 13
       literals in 6 test files (P11), and `startSession`'s `ui` needs no
       edit;
     - the fallback is reached in production, at every paint before a first
       keystroke, so it is no unreachable branch;
     - "like the console's draft": the console's session, actor, token,
       describe and grant boxes are cleared when they send, refused or not
       ("a REFUSED start still clears the name — the text is gone with the
       command"), while the notes form keeps its key, title and text.
       MapTool's `commitCommand` clears its box the same way;
     - Send reading the boxes keeps green the tests that set `.value` with no
       input event: "Send narrates the trimmed text, clears the box and
       repaints" and "an in-character speaker is carried, and a blank one is
       omitted".
7. **D7. The e2e tests go in a new spec file, `client/e2e/typing.spec.ts`.**
   - Forced by `startFixtures` in `client/e2e/setup.ts`: one server and one
     campaign per spec file, and each file closes any session it opens in
     `test.afterAll`.
   - W1 starts a session from the console, which `table.spec.ts`'s own
     session tests would then find already open.
   - W1 ends its session before W2 runs, and the file's `test.afterAll` ends
     any session left open.
8. **D8. `runJoin`'s comment loses its false reason, and app.ts gains no
   comment line.**
   - Forced by rule 10 and G3. The comment budget puts `app.ts` exactly at its
     ceiling (P10).
   - The second sentence, that the value is re-seeded on the paints that
     happen, stays true.
9. **D9. The tasks run in dependency order.** The adjudication comes first,
   by the ruling. The helper and both its callers come next. The Say boxes
   come last, because their focus needs the helper.
10. **D10. `client/src/style.css` turns scroll anchoring off for the app's
    tree with `overflow-anchor: none`.**
    - Forced by the owner's answer "Lägg till overflow-anchor: none" and by
      P13: with a focused box above the viewport, Chromium's scroll
      anchoring moves the page inside `replaceChildren`, at HEAD too, and
      `preventScroll` does not stop it; the property does.
    - Task 2 puts it on the narrowest of `html`, `body` and `#app` that keeps
      W3 green, and says which in the report.
    - Not established: whether anchoring prevents any jump today that the
      rule would bring back. The tree is replaced at every paint, so no
      anchor node outlives one; the report says what W1, W3 and the rest of
      the e2e showed.

## Requirements

Each candidate names the observation that would go red if it broke.
Allocated at Task 0, after sign-off: C1 is VTT-491, C2 VTT-492, C3
VTT-493 and C4 VTT-494.

- **C1.** "When an input or textarea of the root carrying a `data-field`
  holds the focus, a repaint of the table or of the join form gives the
  focus, with the same selection start, end and direction, to the new tree's
  input or textarea with that `data-field`."
  - Evidence: A1, F1, A3, W1, and the focus halves of S1 and W2.
  - Red under K2, K3, K4, K5, K7 and K10.
- **C2.** "A repaint focuses no element of the new tree unless an input or
  textarea of the root carrying a `data-field` held the focus and the new tree
  has one with that `data-field`."
  - Evidence: F2, F3 and A2.
  - Red under K6, K11 and K12.
- **C3.** "After a repaint, the player panel's Say boxes show what was typed
  in them since the panel last sent a narration."
  - Evidence: S1, S2 and W2.
  - Red under K8 and K9.
- **C4.** "A repaint that gives a box back its focus leaves the page scrolled
  where it was." Allocated as VTT-494, after the owner's answer.
  - Evidence: F4, W1's scroll check and W3.
  - Red under K13 and K14.

The sort, and what it rejected:
- The false entry adds no rule. The adjudication file's header rule, that an
  equivalent verdict names a mutant no test can kill, carries no id (the
  ticket).
- "Throws nothing" when the box is gone is not a rule of its own. A throw
  reds A2.
- The console's `draft` and the join form's re-seeded name already hold the
  text, through "text survives a re-render, so an incoming event cannot eat
  what the DM is typing" and "a refusal is shown, and the name the person
  already typed survives it". Neither has a row, and none is added: the
  ticket states four rules and these are not among them.
- The selects: outside the ticket.
- Unique `data-field` values: a precondition of C1, held by D5 for the two
  new names, and no rule.

| Ticket item | Observed by |
|---|---|
| 1 | "an input created without a class carries no class at all", red under K1 |
| 2 | `grep` for the entry's code line in `tools/ts-mutation-equivalents.txt`; `check:ts-mutation` at Task 6 |
| 3 | A1 |
| 4 | F1 |
| 5 | W1, red on Task 1's bundle |
| 6 | S1, S2 |
| 7 | W2, red on Task 2's bundle |
| 8 | A3 |
| 9 | F2, F3, A2 |
| 10 | the "Closed by" paragraph, Task 3 |
| 11 | F4; W1's scroll check; W3 |

## The existing tests that change

- `client/test/dm-view.test.ts`, "an input created without a class carries no
  class at all", also asserts `h.field("actor-id").hasAttribute("class")` is
  false (item 1).

No other test is expected to change:
- no test reads the focus or a selection (P11);
- the Say-box tests set `.value` and Send reads it (D6);
- the `PlayerUIState` literals stay valid with optional fields (D6).

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off on this plan, and an answer to the design
    question.
  - Allocate C1 to C3 with `requirement-id`, and C4 after the owner's
    answer, each row
    `**OPEN — no test yet**`.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:**
  - `git status` is clean;
  - the four rows exist;
  - `python3 tools/check-requirements-chain.py` passes.

### Task 1: the false adjudication (items 1 and 2)

- **Files:** `client/test/dm-view.test.ts`;
  `tools/ts-mutation-equivalents.txt`.
- **Do:**
  1. Add the assertion to "an input created without a class carries no class
     at all".
  2. **Break K1.** Make the mutation by hand: `if (cls) i.className = cls;`
     becomes `if (true) i.className = cls;` in `input()`. Run
     `client/test/dm-view.test.ts`: the test is red (P6). Undo it with the
     inverse edit. `grep -c 'if (cls) i.className = cls;'
     client/src/view/dm.ts` prints 1, and `git diff --stat client/src` is
     empty.
  3. Delete the entry: its header, its `code:` line and its reason.
  4. Make the `client/src/view/dm.ts` section's prose true without the
     entry. Its "166 survivors -> 1" counts it. The `client/src/app.ts`
     section's "No entries… were killed" is the precedent.
  5. Run both adjudication self-tests, the client suite and
     `check-new-prose.py`.
- **True when:**
  - `grep -n 'code: if (cls) i.className = cls;'
    tools/ts-mutation-equivalents.txt` prints nothing;
  - K1 was seen red and undone;
  - both self-tests and `env -u CLAUDECODE bun test client/test contract
    contract-spike` pass;
  - the commit carries no `client/src` change and no bundle.

### Task 2: the helper, the table and the join form (items 3, 4, 5, 8, 9 and 11)

- **Files:**
  - `client/src/view/focus.ts`, new: `replaceKeepingFocus` (D2 to D4), with
    its one-line doc and no other comment unless it is a warning;
  - `client/src/view/spectator.ts`: `renderSpectator`'s last line, and the
    import;
  - `client/src/view/join.ts`: `renderJoinView`'s last line, and the import;
  - `client/src/app.ts`: `runJoin`'s comment (D8);
  - `client/src/style.css`: `overflow-anchor: none` (D10);
  - `client/test/focus.test.ts`, new: F1 to F4;
  - `client/test/app.test.ts`: A1 to A3;
  - `client/e2e/typing.spec.ts`, new: W1 and W3;
  - `tools/comment-ceilings.txt`, by `--write-ledger`, which writes
    `focus.ts`'s row and lowers every row whose file now measures under it:
    `spectator.ts` from 41.0 to 40.9 and `join.ts` from 34.3 to 33.9 once
    the imports are in, and `app.ts` from 48.7 to 48.6 if D8 takes out two
    comment lines;
  - `tools/ts-coverage-thresholds.txt`: `client/src/view/focus.ts` at
    100.00;
  - `cmd/vtt/webdist/`;
  - `docs/requirements.md`: C1's, C2's and C4's evidence.
- **New tests:**
  - **F1** (item 4): `renderSpectator` with a DM console, into a root on
    `document.body`.
    - Focus a textarea that is not the tree's first box, `actor-json`, and
      type into it.
    - Select part of it with direction `backward`, then repaint.
    - The new `actor-json` is focused, with the same start, end and
      direction.
    - Using a textarea, a box that is not first and a backward selection
      makes the textarea half of the query, a pick of the first box and a
      dropped direction each observable.
    - End by emptying the box with an input event carrying `""`.
  - **F2** (item 9): an input outside root, on `document.body`, carrying the
    `data-field` `session-name` that the new tree also holds. It is focused
    across a repaint and stays focused, and no box of root is.
  - **F3** (item 9): `renderSpectator` with a DM console, so the new tree has
    boxes; a button of root holds the focus. After a repaint, `body` does.
  - **F4** (C4): with a spy on `HTMLElement.prototype.focus` that records
    each call's argument, a repaint that restores a box calls it once, with
    `{ preventScroll: true }`. Restore the prototype's method when it ends.
  - **A1** (item 3): `boot` as a DM on the fake socket.
    - Focus `session-name`, type `Open` with an input event, and put the caret
      at 2, inside the text. A box whose value is set puts its caret at the
      end (P1), so a caret at 4 would not tell a restore from none.
    - Deliver a narration: the new `session-name` is focused, holds `Open`,
      with the caret at 2.
    - Then Start session, which clears the draft.
  - **A2** (item 9): `boot` as a DM. Focus `session-name` without typing, then
    deliver `sessionStarted`. Focus is on `body`, nothing throws, and the
    console shows End session.
  - **A3** (item 8): `boot` with `?join=` and the join route stubbed to
    refuse. Focus `join-name`, type a name and press Enter. After the
    refusal's repaint, the name box is focused and holds the name.
  - **W1** (item 5), in `typing.spec.ts`:
    - as the DM, in a 1280 by 300 viewport, focus `session-name`, then
      `scrollTo(0, 0)`, and check the box is below the viewport (P13);
    - add a narration from another connection on the DM's token, opened in
      the page as `table.spec.ts`'s "what the DM is typing survives events
      arriving" does, and wait for it in `.feed`;
    - `window.scrollY` is what it was before the narration (C4);
    - `page.keyboard.insertText("Open Table")`, then Start session;
    - `.session` reads `session: Open Table`;
    - end the session;
    - it carries C1's and C4's ids.
  - **W3** (item 11), in `typing.spec.ts`: as the DM, focus `session-name`,
    scroll the page past it so the box is above the viewport, and have
    another connection add a narration; `window.scrollY` is unchanged. It is
    red today (P13: 408 → 198) and carries C4's id.
- **Order:**
  1. Write W1 and run `task e2e` on Task 1's commit, whose bundle is HEAD's.
     W1 is red on its `.session` text, `sessions: 0`, with the toast "a
     session needs a name" on screen, and every other spec is green unless
     the race this ticket fixes takes one; rerun it then. That is item 5's
     "today".
  2. Write W3 beside W1; it is red on the same run. Write F1 to F4 and A1 to
     A3. F1, F4, A1 and A3 are red; F4 because nothing calls `focus`. F2, F3
     and A2 are green, as negative cases should be.
  3. Write `focus.ts`, the two calls and the CSS rule. Correct `runJoin`'s
     comment.
  4. Make the breaks.
  5. Run `task build:client`, the client suite, `task client:typecheck`,
     `task e2e`, `check-new-prose.py`, `check-ts-coverage.py` and
     `check-comments.py main` after staging. Run `--write-ledger` last.
- **True when:**
  - all of the above are green, W1 included;
  - `tools/comment-ceilings.txt` has a row for `client/src/view/focus.ts`, at
    or under 25.0;
  - `check-ts-coverage.py` counts `focus.ts` as gated;
  - `grep -rn 'replaceChildren' client/src/view` prints only `focus.ts`'s
    line.
- **Breaks**, each undone by its inverse edit:
  - **K2:** `renderSpectator` ends in `root.replaceChildren(...nodes)` again.
    A1 and F1 go red, and W1 too, once the bundle is rebuilt.
  - **K3:** `renderJoinView` ends in `root.replaceChildren(wrap)` again. A3
    goes red.
  - **K4:** the helper focuses the box and sets no selection. F1 and A1's
    caret go red.
  - **K5:** the helper drops the direction. F1 goes red.
  - **K6:** the helper reads `document.activeElement`, unscoped. F2 goes red.
  - **K7:** the helper focuses the new tree's first box instead of the one
    with the field. F1 goes red.
  - **K11:** when the new tree has no box with the field, the helper focuses
    its first box. A2 goes red.
  - **K12:** when no box of root held the focus, the helper focuses the new
    tree's first box. F2 and F3 go red.
  - **K13:** the helper calls `focus()` with no option. F4 goes red, and W1's
    scroll check once the bundle is rebuilt.
  - **K14:** the `overflow-anchor: none` rule removed. W3 goes red once the
    bundle is rebuilt.
  - Before the gate, apply by hand the mutants Stryker will make on the
    helper: its selector strings, its two early returns and its field
    comparison. Each must turn a test red. A survivor is fixed in the code or
    the tests, never adjudicated.

### Task 3: the Say boxes, and the debt closed (items 6, 7 and 10)

- **Files:**
  - `client/src/view/player.ts`: `PlayerUIState`'s `sayAs?` and `sayText?`,
    and `renderPlayerPanel`'s two boxes and its `submit` (D5, D6);
  - `client/test/spectator-view.test.ts`: S1 and S2;
  - `client/e2e/typing.spec.ts`: W2;
  - `cmd/vtt/webdist/`;
  - `tools/comment-ceilings.txt`: `player.ts`'s row, lowered by
    `--write-ledger` to its new share rounded up, if that is lower. The file
    ends more than 1.0 under its ceiling with, for instance, 11 code lines
    and no comment line added;
  - `docs/verification-debt.md` (item 10);
  - `docs/requirements.md`: C3's evidence.
- **New tests:**
  - **S1** (item 6): `renderSpectator` with a player's `renderPlayerPanel`,
    on `document.body`, with one `ui` across paints.
    - A fresh panel's two boxes are empty.
    - Type into both with input events, focus one, repaint. Both hold their
      text, and the focused one is focused.
    - Run it once with each box focused, so a wrong `data-field` on either
      shows.
    - Then Send, in the new tree: the narration carries both texts.
  - **S2:** type into both boxes, Send, repaint with the same `ui`. Both
    boxes are empty (G1).
  - **W2** (item 7), in `typing.spec.ts`:
    - the DM, on a raw socket, starts a session, loads `goblin-ambush` and
      grants `act-fighter` to the player as a party member;
    - the player types `I draw my` into "say or narrate";
    - the DM's narration arrives in the player's `.feed`;
    - the box still holds `I draw my`, and `document.activeElement` is that
      box;
    - it carries C1's and C3's ids.
- **Order:**
  1. Write W2 and run `task e2e` on Task 2's commit. W2 is red: the box is
     empty and unfocused, since Task 2 keeps neither the Say boxes' text nor,
     without a `data-field`, their focus.
  2. Write S1 and S2. S1 is red. S2 is green, as a negative case should be,
     and K9 reds it.
  3. Make D5 and D6.
  4. Make the breaks.
  5. Rebuild the bundle and run everything Task 2 ran.
  6. Close the debt entry. Move it out of "Open debt" into a dated section,
     "## 2026-10-25 — a repaint took the focus from the box the DM was typing
     in", before "## Open debt", as the entries closed before it were moved.
     Add a **Closed by** paragraph naming A1, W1 and W2, each with what it
     was seen red on:
     - A1 under K2;
     - W1 on Task 1's bundle;
     - W2 on Task 2's bundle, where the Say boxes carry no `data-field` and
       keep no text.
  7. Run `--write-ledger` last.
- **True when:** all green, W2 included, and the entry sits in its dated
  section with its paragraph.
- **Breaks:**
  - **K8:** the boxes are not seeded from `ui`. S1 and W2 go red.
  - **K9:** Send leaves `ui`'s fields. S2 goes red.
  - **K10:** the boxes carry no `data-field`. S1's focus half goes red, and
    so does W2's: C1's, not C3's.

### Task 4: the report

- **Do:** write `docs/reports/2026-10-25-a-repaint-keeps-what-is-being-typed.md`
  and give it its reading review. It covers:
  - each "Done looks like" item with its observation (the table above);
  - K1 to K14, with what went red;
  - Rule 9;
  - G1 to G6, and what became of each;
  - Phase 4a's adjudications, under their own heading.

  Leave room for the gate's figures, which Task 6 writes in.
- **True when:** it answers the ticket's eleven items in the ticket's numbering,
  each `[x]` naming its observation, and it has had its reading review.

### Task 5: the specification

- **Do,** with the `specification` skill, from the report. SPEC-022,
  `docs/specifications/022-a-client-holds-one-log.md`:
  - `client/src/view/focus.ts` and `client/src/view/join.ts` join the files
    its Status names. The tests that hold C1 to C4 join the sentence that
    lists them.
  - Under "What the browser client holds, and what it offers", say what a
    repaint keeps:
    - the wholesale rebuild through `replaceKeepingFocus`, for the table and
      the join form;
    - the box rule of C1 and C2;
    - what keeps each box's text: `draft` for the console,
      `JoinViewState.name` for the join form, and `PlayerUIState` until Send
      for the Say boxes (C3);
    - that the box gets its focus back without moving the page, by
      `preventScroll` below the viewport and `overflow-anchor: none` above
      it (C4);
    - that a select or a button loses the focus at a repaint, and the
      fold-failure reason restores nothing.
  - Its Requirements list gains C1 to C4's ids.
  - Cite the owner's rulings of 2026-10-25, "Återställ fokus", "Konsol, Say
    och join", "Ja, preventScroll" and "Lägg till overflow-anchor: none".
- **True when:** each new sentence has been run against the code by someone
  other than its writer, Phase 4b's reading included.

### Task 6: the gate, whole, and the commits

- **Do:**
  - Run `task check` whole on Task 3's commit, the last that changes code.
    - Launch it with `start_new_session=True`, check free disk first, and
      require the gate's own completion line, not just its exit code.
  - Then run `task e2e` once on the same commit.
  - Write both runs' figures into the report.
  - Commit the report, then SPEC-022.
- **True when** both are green, including:
  - `check:ts-mutation`, with the entry's mutant counted killed (item 2) and
    no new adjudication;
  - `check:ts-coverage` and `check:comments`;
  - `check:drift`;
  - `check:requirements-chain`, which resolves C1 to C4.

  The report is committed alone, before the specification.

## For Phase 4a

Hand the QA agent:
- C1 to C4's texts, with their ids;
- the exported surface, as signatures. There is no `go doc` for the client:
  - `replaceKeepingFocus(root: HTMLElement, ...nodes: Node[]): void`;
  - `renderSpectator(root, st, log, status, extras?)`;
  - `renderJoinView(root, state, on)`;
  - `renderPlayerPanel(st, me, abilities, ui, send, rerender, switcher?)`;
  - `PlayerUIState`, with `sayAs?` and `sayText?`;
  - `boot(root)`;
- that a box is an input or a textarea with a `data-field`, and that the
  console's selects carry none;
- that happy-dom observes no scroll, so a scroll rule is seen there only
  through what `focus` is called with.

Tell it:
- **The DOM.**
  - happy-dom moves the focus to `body` when the focused element leaves the
    document. It honours `selectionDirection` and `:focus`.
  - A root must be on `document.body` for the focus to mean anything.
- **Shared state.** bun runs every test file in one process, and `dm.ts`'s
  `draft` is module state. A test that types into a console box empties it
  before it ends. A boot test clears both storages in `beforeEach`.
- **Commands.** Run `env -u CLAUDECODE bun test …`.
- **Browsers.** Never start a browser run beside another one: the e2e and
  any probe share a state file.
