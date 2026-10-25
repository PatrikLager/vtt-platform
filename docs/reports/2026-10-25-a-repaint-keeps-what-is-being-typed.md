# A repaint keeps the box being typed in, and no adjudication excuses a mutant a test can see: the change

**Ticket:** `docs/superpowers/specs/2026-10-25-a-repaint-keeps-what-is-being-typed-design.md`.
An agent that did not write it verified it, and passed it with gaps G1 to
G6. The writer then revised its third rule to the sense the plan gives it
and added `client/src/app.ts` to its list.
**Plan:** `docs/superpowers/plans/2026-10-25-a-repaint-keeps-what-is-being-typed.md`,
written by that verifier from probes outside the tree, and amended by two of
the owner's answers to the reading review of `bfbbc07`, the commit that
carried both.
**Where it came from:** two things the ticket before this one raised. Its
report, `docs/reports/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md`,
raised the open debt "A repaint takes the focus from the box the DM is typing
in", which made `client/e2e/joining.spec.ts` fail once in that ticket's
`task e2e`. Its last code commit, `eb6d423`, raised to the owner a false entry
in `tools/ts-mutation-equivalents.txt`. `bfbbc07`'s message puts both on the
report.
**The owner's rulings, all 2026-10-25:**
- "falska adjudikeringen och fokusdefekten ska åtgärdas".
- "Återställ fokus": before a repaint, the focused box is noted by its
  `data-field` and its selection; after it, the new box with that field gets
  them back; one shared helper for the table and the join form; the rebuild
  of the whole tree stays.
- "Konsol, Say och join": the DM console, the player panel's Say boxes, whose
  text is kept as well, and the join form.
- "Ett ärende, en gren": one ticket, one branch, the adjudication first.
- The plan signed off: "Ja, kör", with the order "Efter rapport och specar":
  this report and the specification are written and reviewed before the
  whole gate runs, and its figures go into this report before its commit.
- On the first review of `bfbbc07`: "Ja, preventScroll".
- On its second: "Lägg till overflow-anchor: none".
- On Task 2's review: "Ja, via activeElement".

**Last code commit:** `5d01719`, on `d1de412`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `bfbbc07` | Task 0: the ticket, the plan, VTT-491 to VTT-494 |
| `65781c5` | Task 1: the assertion on a box built with no class, the false entry's removal |
| `1001ec5` | Task 2: `client/src/view/focus.ts`, `renderSpectator` and `renderJoinView` ending in it, `#app`'s `overflow-anchor`, `runJoin`'s comment, the tests, QA's `client/test/qa-repaint-focus.test.ts` and `client/e2e/qa-repaint.spec.ts`, focus.ts's two rows, the bundle |
| `5d01719` | Task 3: the Say boxes' fields and kept text, their tests, QA's `client/test/qa-say-boxes.test.ts`, the debt entry closed, the bundle |

## What the code did on `d1de412`

`tools/ts-mutation-equivalents.txt` called `if (cls) i.className = cls;` ->
`true`, in `input()` in `client/src/view/dm.ts`, equivalent: "No element can
tell the two apart". Under it a box built with no class carried an empty
`class` attribute where the original carried none, and happy-dom showed it.
`client/test/dm-view.test.ts`'s "an input created without a class carries no
class at all" compared only `className`, which is `""` either way.

The browser client rebuilt its whole tree at every paint: `startSession`'s
`paint` in `client/src/app.ts` ran on every change to the session, on each
connect-time fetch that resolved and on a resize, and `renderSpectator` ended
in `root.replaceChildren(...nodes)`; `renderJoinView` ended in
`root.replaceChildren(wrap)`. Nothing in `client/src` noted or restored the
focus, so the box that held it was detached and the focus went to `body`.
- The DM console's boxes kept their text through `draft` and lost the focus
  and the selection.
- The player panel's Say boxes, `as` and `text` in `renderPlayerPanel`,
  carried no `data-field` and kept nothing: a repaint emptied them.
- The join form re-seeded its name from `JoinViewState.name` and lost the
  focus at its busy paint and at a refusal's.

In Chromium, scroll anchoring moved the page at a repaint when the focused
box was above the viewport or in view with the page scrolled down: on
`d1de412`'s bundle the e2e below measured 439 to 208 and 320 to 27, each in 2
of 3 runs of the report's reviewer. In the third, a repaint that fell between
the test's focus and its scroll had already taken the focus, so no focused box
was left for anchoring to follow.

## Done looks like, answered

1. [x] dm-view's "an input created without a class carries no class at all"
   now also asserts `hasAttribute("class")` is false. Under `if (true)
   i.className = cls;`, made by hand, it is the only red test of the file's
   112 (K1); `d1de412`'s file passes all 112 under the same mutant, measured
   by Task 1's reviewer in a scratch copy.
2. [x] `grep -n 'code: if (cls) i.className = cls;'
   tools/ts-mutation-equivalents.txt` prints nothing, and `check:ts-mutation`
   passes in the `task check` on `5d01719` (Figures). Stryker's report from
   that run holds the two mutants it made of that line in `input()`, the
   condition forced `true` and forced `false`, both `Killed`.
3. [x] `client/test/app.test.ts`'s "an event's repaint leaves the DM typing
   in the same box, caret and all" boots a DM's page, types `Open` into the
   session-name box with the caret at 2, and delivers a narration: the new
   box is focused, holds `Open`, caret at 2. The repaint is a narration's, a
   change to the session in the ticket's sense. Red under K2 and K4.
4. [x] `client/test/focus.test.ts`'s "a repaint hands the focused box's focus
   and backward selection to its successor": `actor-json`, a textarea and not
   the tree's first box, selected 1 to 4 backward, gets the same range in the
   new tree. Red under K2, K4, K5 and K7.
5. [x] `client/e2e/typing.spec.ts`'s "a box below the viewport keeps the focus
   across a repaint, and the page stays put": as the DM, the session-name box
   focused, another connection's narration waited for in the feed,
   `insertText("Open Table")`, Start session, and `.session` reads "session:
   Open Table". On `65781c5`'s bundle, from before the helper, `.session`
   read "sessions: 0".
6. [x] `client/test/spectator-view.test.ts`'s "what a player typed in Say
   survives a repaint, with the as box focused" and "…, with the text box
   focused": both boxes typed into, one focused, a repaint through
   `renderSpectator` with one `PlayerUIState`, both hold their text, the
   focused one is focused, and Send in the new tree sends both. Red under K8
   and K10. On Task 2's code the two tests stop at their check of the field
   names; with that check left out, both boxes were empty after the repaint.
   "Send empties both Say boxes for the paints after it" holds the Send half.
7. [x] typing.spec.ts's "what a player types in Say survives a repaint, focus
   and all": the player types `I draw my`, the DM's narration arrives, and
   the box holds the text and is focused, both read by retrying matchers. On
   `1001ec5`'s bundle the box was empty.
8. [x] app.test.ts's "a refused join leaves the joiner typing in the name
   box": Enter in the focused name box, a 403, and after the refusal's repaint
   the name box is focused and holds the name. Red under K3.
9. [x] focus.test.ts's "a repaint leaves alone a focused box outside the root
   that shares a field" and "a repaint gives no box the focus a button of the
   root held", and app.test.ts's "a repaint that takes the focused box away
   focuses nothing in its place", which also throws nothing. Each is green on
   `d1de412`, as negative cases are, and red under K6 or K12, K12, and K7 or
   K11.
10. [x] The entry moved, as it stood, to a dated section of
    `docs/verification-debt.md`, "2026-10-25 — a repaint took the focus from
    the box the DM was typing in", with a recipe and a **Closed by** naming
    the tests of items 3, 5 and 7, each seen red on a tree that did not
    restore the focus of the box it types in: item 3's under the recipe, item
    5's on `65781c5`'s bundle, from before the helper, and item 7's on
    `1001ec5`'s bundle, where the helper existed and the Say box carried no
    `data-field` for it to find. On `5d01719` with only `say-text`'s
    `data-field` removed, the report's reviewer saw item 7's test red at its
    focus check while the text held.
11. [x] Below the viewport, item 5's test checks the page stays at the top, and
    is red under K13 (0 to 192). Above it, "a repaint leaves the page put when
    the focused box is above the viewport" is red on `d1de412`'s bundle (439 to
    208, in 2 of 3 runs, for the reason under "What the code did") and under
    K14 (495 to 0). focus.test.ts's "a repaint focuses the successor without
    scrolling the page to it" holds the `preventScroll` argument in happy-dom,
    which observes no scroll.

## What the rules became

- "A box someone is typing in keeps the focus and its selection across a
  repaint of the table or of the join form": VTT-491, held by the tests of
  items 3 to 8, `qa-say-boxes`'s "qaSay: the focused text box keeps the focus
  and the caret mid-text", and "a box in a background tab keeps the focus
  across a repaint".
- "What a player has typed in a Say box survives a repaint until Send sends
  it": VTT-493, held by the tests of items 6 and 7 and `qa-say-boxes`'s
  "qaSay: the DM's narration repaints and both boxes keep what was typed".
- "A repaint focuses nothing in the new tree unless a box of the root held the
  focus and the new tree has a box with its `data-field`": VTT-492, held by
  the tests of item 9.
- "A repaint that gives a box back its focus leaves the page scrolled where it
  was": VTT-494, held by the tests of item 11 and "a repaint leaves the page
  put when the focused box is in view, scrolled down".
- The false entry adds no rule: it broke the adjudication file's own, that an
  equivalent verdict names a mutant no test can kill, which no id carries.

The plan's sort refused: "throws nothing" as a rule of its own, which a throw
in item 9's "a repaint that takes the focused box away focuses nothing in its
place" already reds; the console's and the join form's kept text, held by
tests older than this ticket and no rule of its; the selects; the uniqueness
of a `data-field`, a precondition of VTT-491.

## Phase 4a

One QA agent per task that delivered behaviour, blind to the implementation,
the existing tests and the diff, given the ticket, the rows, SPEC-022, named as
governing in both QA briefs, which are not in the tree, and the exported
surface as signatures. Task 0 and Task 1 had no behaviour to derive.

| Task | Tests, as committed | Failures |
|---|---|---|
| 2 | 26 in `client/test/qa-repaint-focus.test.ts`, 3 in `client/e2e/qa-repaint.spec.ts` | 1 |
| 3 | 19 in `client/test/qa-say-boxes.test.ts` | 0 |

### QA adjudications

- Task 2, finding 1: behaviour defect, latent — a focused input of type
  `number` made `setSelectionRange` throw `InvalidStateError` after the
  replace and the focus — fixed: `replaceKeepingFocus` restores a selection
  only when the old box has one, and QA's "qaRF: a focused input with no
  selection API is focused, nothing thrown" holds it (K15). No box in
  `client/src` sets a `type` today.
- Task 2's QA named as gaps: several boxes sharing a field (the first in tree
  order gets the focus; the console's fields are unique), a selection longer
  than the new text, the resize repaint, Firefox and WebKit. All four are
  under "What could not be established".
- Task 3's QA found no defect, and its 22 injections each redded a test. The
  gaps it named are older than this ticket, Send trimming its text, a blank
  Send sending nothing and Enter sending from the text box, each held by a
  spectator-view test from before; or they are the plan's, under "What was
  deliberately left out".
- Task 2's QA asked for ids for item 5's session start, item 9's "throws
  nothing", and the text half of items 3 and 8: no new id. Item 5's session is
  one of the observations VTT-491 is seen through in a browser, and the plan's
  sort refused the other two (above).

The new tests' element assertions, and QA's, compare identities (`expect(a ===
b).toBe(true)`) or tag names: a failing `toBe(element)` made bun print the
whole window, and one full-suite run under a break hung past 500 s.

## Phase 4b

One reviewer per task, by command, with Chromium probes in Tasks 0, 2 and 3.
Every Must and Should was taken. One Nit was not: Task 3's two tests leave
their table in the shared document, which no other test was seen to read.
Task 1's third Nit, an older test comment, was carried here rather than acted
on (below).

| Task | Pass 1 | Pass 2 |
|---|---|---|
| 0 | 1 Must, 5 Should, 11 Nits | 1 Must, 2 Should, 3 Nits |
| 1 | 1 Must, 1 Should, 3 Nits | — |
| 2 | 1 Must, 3 Should, 5 Nits | 2 Nits |
| 3 | 1 Must, 2 Should, 5 Nits | — |

The findings that changed the work:
- Task 0's first: a plain `focus()` scrolled the page to a box out of view,
  0 to 1437 in Chromium. The owner answered "Ja, preventScroll".
- Task 0's second: with the focused box above the viewport, scroll anchoring
  moved the page inside `replaceChildren`, which `preventScroll` does not
  reach. The owner answered "Lägg till overflow-anchor: none".
- Task 2's Should, sent to the owner: `:focus` matches nothing in a background
  tab or window. The owner answered "Ja, via activeElement".
- Task 3's: the Say box's browser test read the focus through
  `Locator.evaluate`, which finds the element and then runs on it; one of the
  narration's repaints fell between the two in 5 of 6 runs.

This report had one reviewer, two passes, by command, with browser runs on
`65781c5`'s, `1001ec5`'s and `5d01719`'s bundles. Pass 1: 4 Must, 5 Should,
13 Nits. The Musts: where the false entry came from; a "worst jump" nothing
measured as worst; "every case moved", which one case did not; and a resize
test said not to exist. The Shoulds: the old bundle's reds in 2 of 3 runs;
item 10's reason; the notes form's exception; destinations for the
omissions; an id request with no line. Pass 2: 2 Must, 1 Should, 3 Nits. The
Musts: the header put the misattribution on the ticket, which makes none; and
a debt entry for the selects and buttons, which VTT-492 contradicts, now a
requirement question for the owner. All taken.

## The deliberate breaks

Each was made by hand, run, and undone by its inverse edit, checked by grep or
`cmp`; a browser break was rebuilt into the bundle and the bundle rebuilt
after.

| Break | What it did | Red |
|---|---|---|
| K1 | `if (true) i.className = cls;` | "an input created without a class carries no class at all" |
| K2 | `renderSpectator` ends in `root.replaceChildren(...nodes)` | items 3 and 4, and "a repaint focuses the successor without scrolling the page to it" |
| K3 | `renderJoinView` ends in `root.replaceChildren(wrap)` | item 8 |
| K4 | no `setSelectionRange` | items 3 and 4 |
| K5 | the direction dropped | item 4 |
| K6 | `held` is `document.activeElement`, unscoped | the box outside the root |
| K7 | `next` is the new tree's first box | items 4 and 9 |
| K11 | the first box when the field is gone | item 9's box taken away |
| K12 | the first box when no box of root held the focus | item 9's outside box and button |
| K13 | `focus()` with no option | "a repaint focuses the successor without scrolling the page to it"; in Chromium, item 5's scroll check, 0 to 192, and the above-the-viewport test |
| K14 | `overflow-anchor: none` removed | the above-the-viewport test, 495 to 0; the in-view test, 376 to 27 |
| K15 | the selection guard removed | QA's number-input test |
| K16 | `held` found by `:focus` again | "a box in a background tab keeps the focus across a repaint", alone |
| K8 | a Say box not seeded from `ui` | item 6; item 7 in Chromium |
| K9 | Send leaves a `ui` field | "Send empties both Say boxes for the paints after it" |
| K10 | a Say box without its `data-field` | item 6, at its check of the field names |

And 20 hand mutants of `focus.ts`, the ones Stryker was expected to make there
(its selector, its two lookups, the early returns, the field comparison, the
focus argument, the guard), and 12 of the Say boxes' new lines in `player.ts`;
each reds a test. Two of the latter, a `data-field` set to `""`, survived until
item 6's tests pinned the names: the field stayed unique, so the focus was
still restored.

## Rule 9: how MapTool does this

`CommandPanel` in `~/dev/RPTool/maptool` at `f4b7fef6c`
(`src/main/java/net/rptools/maptool/client/ui/commandpanel/CommandPanel.java`)
builds `commandTextArea` once, behind `if (commandTextArea == null)`, and an
arriving message goes to `messagePanel.addMessage`, never to the text area;
`commitCommand` clears the area with `setText("")` once it has sent. So
MapTool never meets the problem: Swing keeps the component, and with it the
focus and the caret. This client rebuilds its tree by design, and the ruling
kept that, so the rebuild carries the focus across instead. Clearing the Say
boxes on Send is `commitCommand`'s.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The plan's D3: the focused box found by a `:focus` query scoped to root | Found as the box of root that is `document.activeElement` | Task 2's reviewer measured, in Chromium's full headless with focus emulation off, that a background tab's focused box is `document.activeElement` and matches no `:focus`; the owner answered "Ja, via activeElement" |
| The plan's D4: the selection read before the replace | Read from the old box after it | Chromium and happy-dom both keep a detached box's selection, measured by Task 2's reviewer, so the old box is read once it is known to have a successor |
| No guard on the selection, since no box in `client/src` sets a `type` | `if (held.selectionStart === null) return;` before `setSelectionRange` | QA's finding 1: a `number` box throws after the tree is already replaced, which would end the paint |
| The plan's W1 and W3 as the browser tests of focus and scroll | Also "a box in a background tab keeps the focus across a repaint" and "a repaint leaves the page put when the focused box is in view, scrolled down" | The first holds the owner's answer and K16 alone; the second is the case the plan's W1 and W3 left uncovered, which Task 2's reviewer measured jumping on `d1de412` (269 to 7), and K14 reds it |
| typing.spec.ts in Playwright's default headless shell | The whole file runs with `channel: "chromium"` | The shell keeps every page focused, emulation on or off, so the background-tab test could not go red there; Playwright refuses `channel` inside a describe |
| The setup of "a repaint leaves the page put when the focused box is above the viewport", as the plan wrote it | A first narration waited for in the feed, and the scroll and measurement made in one `page.evaluate` that reads the box where it runs | It flaked twice, each mechanism isolated: the log's catch-up could still grow the Story row above the console after the scroll, and a repaint could replace the box between the locator's lookup and the call, a detached box measuring all zeros |
| The plan's S1 and S2, with S2 green on the old code | The boxes found by class, and S1 as two tests with literal names | By `data-field`, S2 could not find the boxes on the old code; the chain checker cites only a test name it finds in quotes |
| The plan's Task 2 named the rows `--write-ledger` would lower: `focus.ts`'s new one, `spectator.ts`'s, `join.ts`'s, and `app.ts`'s if two comment lines went | It also lowered `dm.ts`'s, from 38.7 to 38.6 | `dm.ts` has measured 38.60 since `eb6d423`, and the ledger lowers every row to its share rounded up; the plan's own measurement put `dm.ts` at 38.60 and its list left it out |

## What could not be established

- Firefox and WebKit were not run: whether they restore a textarea's selection
  direction, keep a detached box's selection, anchor scrolling the same way, or
  keep a background tab's box as `document.activeElement`.
  `playwright.config.ts` names no project, so the e2e runs Chromium alone.
- Whether `overflow-anchor: none` brings back a jump that scroll anchoring
  prevented. Task 2's reviewer measured a DM's page at 1280 by 300, with and
  without the rule, on `d1de412`'s bundle and on this branch's: with nothing
  focused, nothing moved either way; with a focused box in view or above the
  viewport, the page moved with anchoring and not without; with the box below
  and the page at the top, it stayed at 0 either way. A player's page with a
  character, and a long session's feed, were not measured.
- Several boxes sharing one `data-field`: the first in tree order gets the
  focus. Nothing tests it, and the console's and the panel's fields are
  unique.
- A selection longer than the new box's text: `setSelectionRange` clamps it
  in Chromium and happy-dom; nothing tests it.
- The resize repaint goes through the same `paint`, which app.test.ts's "the
  board is repainted when its container resizes, not only when an event
  arrives" drives through a fake `ResizeObserver`; no test holds a box's
  focus across it.
- Whether Playwright's `fill` keeps the selection it made when the focus comes
  back between its select and its insert; item 5's test inserts after the
  repaint instead.

## What was deliberately left out, and where it went

- The console's selects and a focused button lose the focus at a repaint, as
  before, and VTT-492 requires it: focus.test.ts's "a repaint gives no box the
  focus a button of the root held" and QA's "qaRF: a focused console select or
  button focuses nothing new" pin it. Restoring their focus would change
  VTT-492; raised to the owner with this report as a requirement question,
  who decides whether it becomes a ticket.
- `client/test/dm-view.test.ts`'s comment on "a group's row holds only its
  elements, with no stray text" says forcing el()'s `text !== undefined`
  guard true leaks text into containers; Task 1's reviewer measured that it
  reddens nothing, and the report's reviewer that the client suite stays
  green under it. Outside this ticket; it went to the open debt in
  `docs/verification-debt.md`, "A dm-view comment says el()'s guard forced
  true leaks text".
- Send empties the "in character as" box as well as the text, so a player
  speaking in character retypes the name for each line: the screen Send left
  before, kept by the plan's D6. Raised to the owner with this report, who
  decides whether it becomes a ticket.
- A refused narration's text is gone, as a refused console command's is,
  except the notes form's, whose Save keeps its key, title and text. Raised
  to the owner with this report, who decides whether it becomes a ticket.
- SPEC-022: the commit that follows this report.

## Figures

From the `task check` on `5d01719`, launched with this report, SPEC-022's
change and `docs/verification-debt.md`'s in the tree, uncommitted; it exited
with code 0. Every figure given for `eb6d423` is from the Figures of
`docs/reports/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md`.

- Go coverage: 21 packages at or above their floors; the gate notes 13 that
  clear theirs by more than 1.0pp. `internal/perceive` measured 95.7% against
  its 94.5% floor, and `internal/gateway` 94.0% against 93.5%.
- Client coverage: 24 files gated, `focus.ts` among them, each at 100.00%;
  1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors.
  - The ticket changes no Go file, and every package reused the verdict of
    its last passing run, nothing in its dependency closure differing.
  - The reused verdicts carry 7 timed out, counted as killed and not
    measured: 4 in `internal/sight`, 1 each in `internal/mcp`,
    `internal/perceive` and `internal/rules`, the counts given for
    `eb6d423`.
- Client mutation, in its container: Stryker's own "Done in 59 minutes and
  28 seconds". 3241 mutants: 3169 killed, 9 timed out and counted as killed,
  and 63 survivors, all adjudicated equivalent; on `eb6d423` they were 3203,
  3130, 9 and 64.
  - The 9 timed out are in `canvas.ts`, `scene-plan.ts` and `wire.ts`, which
    this ticket does not change.
  - `tools/mutation-equivalents.txt` is unchanged since `a8bb0ef`, on `main`
    before this ticket. `tools/ts-mutation-equivalents.txt` lost one row in
    `65781c5`, the one on `input()`'s `if (cls) i.className = cls;`, and
    gained none; `dm.ts`'s four other rows are unchanged.
  - `client/src/view/focus.ts`, new: 24 killed, none survived. The 20 hand
    mutants under "The deliberate breaks" were made before Stryker ran on it.
  - `client/src/view/dm.ts`: 487 killed and 4 survived, against 486 and 5 on
    `eb6d423`. The file is unchanged since `eb6d423`, its four survivors are
    its four remaining rows, and the fifth, item 2's, is now killed.
  - `client/src/view/player.ts`: 261 killed, none survived, against 247.
  - `client/src/view/spectator.ts`: 454 killed and 2 survived, as on
    `eb6d423`.
  - `client/src/view/join.ts`: 33 killed, none survived.
  - `client/src/app.ts`: 226 killed, none survived, as on `eb6d423`.
- Race: both runs pass.
- Requirements chain: 494 rows, 312 test files, 16 specifications.
- Comments: 337 files, clean, with 7 comment lines added against `main`.
  Its two notices are the ones `eb6d423` had:
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6, and
  `internal/gateway/export_test.go` at 49.18 against 45.9.
- New prose, which reads Go and TypeScript only: 1916 added lines across 13
  files, clean.
- Disk: 31.8 GiB free, by the gate's own line, when the Go mutation step
  began.
- Duration: 4650 s, from the launch at 23:23:01 on 2026-10-25 to the exit at
  00:40:31 on 2026-10-26.

`task e2e`, once, against the same tree after the gate: 28 passed and 1
skipped, in 31.3 s. The skipped one is "a network drop while the table plays
on ends in a Reconnect", a `test.skip` in `client/e2e/handover.spec.ts`.
