# A repaint keeps the box being typed in, and no adjudication excuses a mutant a test can see

## The problem

**A repaint takes the box someone is typing in.** The browser client rebuilds
its whole tree on every paint. `startSession`'s `paint` in `client/src/app.ts`
runs on every change to the session, on each connect-time fetch that resolves
and on a resize, and `renderSpectator` in `client/src/view/spectator.ts` ends in
`root.replaceChildren(...nodes)`. `runJoin`'s `paint` repaints the join form on
a submit and on a refusal, and `renderJoinView` in `client/src/view/join.ts`
ends in `root.replaceChildren(wrap)`. Nothing in `client/src` notes or restores
the focus: `grep -rnE 'focus|activeElement|setSelectionRange' client/src`
prints no code. So whatever box held the focus is detached, the focus goes to
`body`, and a keystroke after the repaint is lost. Three places meet it:

- The DM console, `renderDMConsole` in `client/src/view/dm.ts`. Its boxes keep
  their text across a rebuild through `draft`, but not the focus or the
  selection. In a browser, a DM who types `Open` into the session-name box,
  sees an event arrive and types ` Table` holds `Open`, with
  `document.activeElement` the `body`. Playwright's `fill` focuses a box and
  then inserts its text, so a repaint between the two leaves the box empty, and
  Start session answers "a session needs a name": that is how
  `client/e2e/joining.spec.ts` failed once in `task e2e` on `eb6d423`.
- The player panel's "Say something" boxes, `as` and `text` in
  `renderPlayerPanel` in `client/src/view/player.ts`. They carry no
  `data-field` and nothing keeps what was typed, so a repaint loses the text
  as well as the focus. In a browser, a player who types `I draw my` into "say
  or narrate" and sees the DM's narration arrive holds an empty box, with the
  focus on `body`.
- The join form. Its name box is re-seeded from `JoinViewState.name` on every
  paint, but the paint that marks the join busy and the one that reports a
  refusal both rebuild it without the focus, so a joiner who pressed Enter and
  was refused must click back into the box to correct their name.

The console's selects keep their choice through `draft` and nobody types into
them; they are outside this ticket.

**One adjudication excuses a mutant a test can see.** The entry in
`tools/ts-mutation-equivalents.txt` for `client/src/view/dm.ts`'s `if (cls)
i.className = cls;`, replacement `true`, says no element can tell the
always-assign from the original. Under it, an input built with no class gets
an empty `class` attribute where the original gets none; happy-dom shows it as
`hasAttribute("class")` true. The file's header says an equivalent verdict
asserts that no test can ever kill the mutant. `client/test/dm-view.test.ts`'s
"an input created without a class carries no class at all" compares only
`className`, which is `""` either way, so the mutant survives and the entry
excuses it.

## Done looks like

1. `client/test/dm-view.test.ts`'s "an input created without a class carries
   no class at all" also asserts the box has no `class` attribute, and is red
   under `if (true) i.className = cls;`; today that mutant leaves it green.
2. `tools/ts-mutation-equivalents.txt` holds no entry for `if (cls)
   i.className = cls;`, and `task check`'s `check:ts-mutation` passes with
   that mutant counted killed.
3. A client test focuses the DM console's session-name box, types into it,
   has the session change so the table repaints, and finds the new
   session-name box focused, holding the text, with the caret where it was;
   today the focus is on `body`.
4. A client test selects part of a console box's text, repaints, and finds
   the same range selected in the new box; today nothing is focused.
5. A browser test in `client/e2e/`: as the DM, focus the session-name box,
   have another connection add a narration, wait for it in the feed, insert
   `Open Table` and press Start session; a session named "Open Table" starts.
   Today Start session answers "a session needs a name".
6. A client test types into both of the player panel's Say boxes, repaints,
   and finds both holding their text and the focused one focused; Send then
   sends the narration with both. Today both are empty after the repaint.
7. A browser test: a player types into "say or narrate", the DM's narration
   arrives, and the box still holds the text and the focus. Today it is empty.
8. A client test focuses the join form's name box, submits a join that is
   refused, and finds the name box focused, holding the name, after the
   refusal's repaint; today the focus is on `body`.
9. Still true afterwards, each by a test: a repaint while the focus is outside
   the root, or on an element of the root that is not a box with a
   `data-field`, focuses nothing in the new tree; a repaint after which the
   focused box no longer exists, such as the session-name box once a session
   has started, leaves the focus on `body` and throws nothing.
10. `docs/verification-debt.md`'s open-debt entry "A repaint takes the focus
    from the box the DM is typing in" carries a "Closed by" paragraph naming
    the tests of items 3, 5 and 7, each observed red on a tree without the
    restoring of the focus.
11. Browser tests: a repaint that gives back the focus to a box scrolled
    out of view, below the viewport or above it, leaves `window.scrollY`
    where it was. Below, it is red when the box is focused with a plain
    `focus()`; above, it is red today, since the page already moves there
    at a repaint.

## Rules this puts on the system

- A box someone is typing in keeps the focus and its selection across a
  repaint of the table or of the join form.
- What a player has typed in a Say box survives a repaint until Send sends it.
- A repaint focuses nothing in the new tree unless a box of the root held the
  focus and the new tree has a box with its `data-field`.
- A repaint that gives a box back its focus leaves the page scrolled where it
  was ("Ja, preventScroll", the owner's answer of 2026-10-25).

The false entry adds no rule. It breaks the adjudication file's own, that an
equivalent verdict names a mutant no test can kill, which no requirement id
carries.

## What it touches

Client, in this order:

1. `client/test/dm-view.test.ts` and `tools/ts-mutation-equivalents.txt`: the
   assertion, then the entry's removal.
2. The shared restoring of the focus, used by `renderSpectator` in
   `client/src/view/spectator.ts` and `renderJoinView` in
   `client/src/view/join.ts`. A new module for it also needs its rows in
   `tools/comment-ceilings.txt` and `tools/ts-coverage-thresholds.txt`.
3. `renderPlayerPanel` and `PlayerUIState` in `client/src/view/player.ts`:
   the Say boxes' `data-field` and their kept text.
4. Tests: `client/test/app.test.ts`, `client/test/player.test.ts`,
   `client/test/spectator-view.test.ts` or a new test file, and a browser test
   under `client/e2e/`.
5. `client/src/style.css`, for the page's scroll ("Lägg till
   overflow-anchor: none", the owner's answer of 2026-10-25).
6. `client/src/app.ts`: `runJoin`'s comment that a rebuild mid-typing throws
   away the caret, false once the rebuild keeps it.
7. `cmd/vtt/webdist`, rebuilt by `task build:client`.
8. `docs/verification-debt.md`'s "Closed by".

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- Whether Firefox and WebKit restore a `textarea`'s selection direction as
  Chromium does; the tests of items 3 and 4 run in happy-dom, and the e2e in
  Chromium alone.
- Whether Playwright's `fill` keeps the selection it made when the focus
  returns between its select and its insert; item 5 inserts after the repaint
  instead, which is what the e2e failure needed.
