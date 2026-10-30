# Plan: the DM speaks from the console, always, with /ooc named where they type

- **Ticket:**
  `docs/superpowers/specs/2026-10-30-the-dm-speaks-from-the-console-design.md`.
- **Verdict:** passes, with gaps G1 to G7 (below).
  - Verified 2026-10-30 by an agent that did not write the ticket,
    read-only against HEAD `9e33202`.
  - Probes ran under
    `~/.claude/projects/-Users-patriklager-dev-vtt-platform/scratch-t10/verify/`,
    importing the client by absolute path. P14's console is a scratch copy
    of `client/src/view/dm.ts` with a Say group added; nothing under
    `client/` was edited.
  - `task check`, `task e2e`, Stryker and gremlins were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-30-the-dm-speaks-from-the-console.md`.
- **Base:** main `9e33202`.
- **Branch:** `fix/the-dm-speaks-from-the-console`.
- **Owner's rulings taken in:**
  - 2026-10-30, where the DM speaks: "I konsolen, alltid", to the option
    "Konsolen får en Say-ruta med "as"-ruta och en platshållare som nämner
    /ooc. Panelen slutar rita Say för DM och agent, så det finns en enda
    plats att tala på, som i MapTool. VTT-504 ersätts av en regel om
    konsolen.";
  - 2026-10-30, layout: "Ett ärende, en gren";
  - 2026-10-26 (the ticket `2026-10-26-a-player-speaks-as-their-character`):
    the DM and the agent keep a free speaker name; `/ooc` as the first word
    steps out of character; the server sets an out-of-character line's
    speaker;
  - 2026-10-29 (the ticket
    `2026-10-29-a-refused-command-keeps-what-was-typed`): a refused line keeps
    its text; an accepted one empties the line box and keeps the speaker ("Ja,
    behåll namnet", "Töm ändå"); one lock per control, nothing drawn
    differently while a command is unanswered ("Nej, osynligt lås"); a
    rejected send goes to `console.error`.

  The ticket is consistent with each of them. It quotes the two of
  2026-10-30, and reaches the 2026-10-29 rulings through its fourth rule.
- **Signed off:** 2026-10-30, "Ja, kör", with Q1 "Först, över Session", Q4
  "Inget", and Q2 and Q3 "Ja, båda".

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   `cmd/vtt/webdist` and `client/e2e/refusals.spec.ts` included. Every
   symbol stands where it says: `renderDMConsole` and `DMDeps` in
   `client/src/view/dm.ts`; `renderPlayerPanel`, `PlayerUIState.sayAs`,
   `speaksFreely` and `submit` in `client/src/view/player.ts`; `canAct` and
   `isDM` in `client/src/app.ts`; `readLine` in `client/src/player.ts`;
   `mayVoice` in `internal/gateway/authz.go`; the `addNarration` row and its
   comment in `client/test/command-surface.test.ts`. `dm.ts` imports neither
   `addNarration` nor `speakOutOfCharacter` and draws no `say-as` or
   `say-text` (P1). The two placeholders read as the ticket quotes them
   (P2). One citation was off: the MapTool path (G1), corrected in the
   ticket before Task 0's commit.
2. **"Done" is an observation.** The tests the items describe do not exist
   yet.
   - Item 1 fails today, by command: the console drawn for a DM with no
     actor has no `say-as`, no `say-text` and no `add-narration` (P1).
   - Items 2 and 3 fail today, by command through item 1: there is no
     console Say to type into.
   - Item 4 fails today, by command, for the DM and the agent: their panel
     draws both boxes and Send. Its player half holds today (P2, G5).
   - Item 5 fails today, by command for each half and by reading for the
     whole: a DM who controls no actor gets "You do not control an actor
     yet." and no Say from the panel (P3), and none from the console (P1).
   - Item 6 fails today, by reading: the console has no Say form, and
     `client/e2e/refusals.spec.ts` speaks the DM's line through
     `.player .say` after granting a character. Its `speaking.spec.ts` half
     is a guard (G5).
3. **Each rule is breakable. A reading.** Each rule has an observation that
   goes red; see "Requirements". The first two rules join two obligations
   each and are split (C1 and C2; C3 and C4). The fourth rule names rows
   that "move with it", which the register cannot do: a row is never
   reworded (SPEC-008, and the `requirements` skill). How the plan realises
   it, and the row the rule misses, is G2.
4. **The scope matches the claim. By command, then a reading.**
   - `renderPlayerPanel` has 22 calls: one in app.ts and 21 in nine test
     files. Six of those files render a DM's or an agent's panel; four
     touch its Say (P5).
   - `renderDMConsole` has 26 calls: one in app.ts and 25 in eight test
     files. Every console those files draw gains the Say boxes (P5).
   - The DM's Say is held by about 55 tests across `qa-say-boxes.test.ts`
     (19), `qa-speaking.test.ts` (7 templates, run for the DM and the
     agent), `qa-refused-line.test.ts` (12), `spectator-view.test.ts` (10
     touched) and `app.test.ts` (1). The ticket names those files.
   - The reading: the ticket does not name `client/test/qa-refused-command.test.ts`
     or `dm.ts`'s two comments, which cite VTT-519; nor the register's
     evidence cells the moved tests leave; nor the comment ceiling the
     panel's change runs into (G3, G4).
5. **No recorded decision is contradicted silently. A reading.**
   - SPEC-022's "Saying something" records the owner's rulings of
     2026-10-26, "Ja för DM" and "Ja, som DM": a DM's and an agent's panel
     draws `say-as`. The ticket overturns where, by "I konsolen, alltid",
     and says so by withdrawing VTT-504. It keeps the free speaker name.
   - SPEC-013 does not move: `mayVoice` already lets a DM and an agent name
     any speaker and speak out of character.
   - `2026-10-29-a-refused-command-keeps-what-was-typed`'s report left "a DM
     with no actor and no Say form" out of scope. This ticket is that case.
   - No gate changes (rule 2), and the contract is untouched (rule 3).
6. **The records it moves. By command, then a reading.**
   - The section is present, and SPEC-022 resolves.
   - The reading: of `docs/specifications/`, only SPEC-022 mentions
     `say-as`, `say-text`, `Say`, `renderPlayerPanel`, `speaksFreely` or
     `add-narration` (P13). It moves in the places Task 5 lists.

## The gaps this plan inherits

- **G1. The MapTool path is relative to the wrong directory.** The ticket
  wrote "In `~/dev/RPTool/maptool` at `f4b7fef6c`, `CommandPanel` in
  `maptool/src/main/java/net/rptools/maptool/client/ui/commandpanel/`".
  Inside that repository the directory is
  `src/main/java/net/rptools/maptool/client/ui/commandpanel/`; the path as
  written resolves from `~/dev/RPTool`. The commit resolves and the claims
  hold (P10). The ticket's writer corrected it before Task 0's commit.
- **G2. The fourth rule misses two rows, and "move" needs a meaning.**
  - **VTT-519 becomes false.** "A command the server accepts leaves each DM
    console field that fed it as it is drawn when nothing has been typed in
    it, except the notes form's key, title and text." The console's
    `say-as` feeds a line and, by "Ja, behåll namnet", keeps its text
    after an accepted one. VTT-519 is withdrawn and succeeded by C7, which
    adds that exception (D11).
  - **VTT-517 already reaches the console's Say.** "A command the server
    refuses clears no DM console field that fed it" holds of it as worded.
    It gains evidence, and no new row is needed.
  - **VTT-518 and VTT-520 name "the player panel's `say-text`".** They stay
    true of a player's and do not move. Their console halves are VTT-517
    and C7.
  - **VTT-521 and VTT-522's first half name "a DM's or an agent's
    `say-as`"**, by role and field, with no surface. By Q2's answer the
    console's box keeps that name, and both rows stay true as worded.
    Their evidence moves with the tests.
  - **VTT-523, VTT-524 and VTT-526 list their controls.** The console's Say
    is not among them, and they stay true. The console's Say gets C9 to
    C11 (D11).
  - VTT-522's second half names the panel's `say-text`. The console's
    gets C8.
- **G3. Files the ticket does not name.**
  - `client/test/qa-refused-command.test.ts` and `client/src/view/dm.ts`
    cite VTT-519 (P11).
  - The register's evidence cells of VTT-500, VTT-501, VTT-517, VTT-518,
    VTT-520 and VTT-524 change. Those of VTT-491, VTT-521 and VTT-522 keep
    their names under D13.
  - A test helper, `client/test/support/say.ts` (D10).
  - `tools/comment-ceilings.txt`, only if a share falls past the band (D15).
  - `client/src/app.ts` does not change (D4). The ticket's "what
    `client/src/app.ts` hands it" is nothing new.
- **G4. Deleting the DM's Say raises `view/player.ts`'s comment share
  above its ceiling** (P6). The share is 21.21 against a ceiling of 21.3.
  The deletion is code, so the share rises. `check-comments.py` refuses a
  file above its ceiling that the change added a comment line to, and a
  rewritten comment line is an added one. D6 keeps the file under.
- **G5. Some done items hold today.**
  - Item 4's player half: a player's panel draws `say-text` with `/ooc` in
    its placeholder and no `say-as` (P2).
  - Item 2's "a blank line, and `/ooc` with nothing after it, send nothing"
    holds of any form with no Send. S5 pairs it with a line that sends.
  - Item 6's "`speaking.spec.ts` still passes" is a guard.
- **G6. The mutation report's date.** The ticket said the report "is already
  out of date since `9e33202`". It measured `db81c37` and is out of date at
  `9e33202` (P7). It has been since
  `2026-10-29-a-refused-command-keeps-what-was-typed`'s first `client/src`
  commit. The ticket's writer reworded it before Task 0's commit.
- **G7. The ticket's own unestablished points.**
  - The agent's window gains the Say, since `isDM` draws the console for
    both roles. C1 holds either way. Whether a person opens an agent's
    window was not established here either.
  - Whether a DM's panel with no Say reads as complete: E2's screenshot
    shows it to the owner (Task 3).

## Measurements this plan stands on

All on 2026-10-30 at `9e33202`, outside the repository unless named.

| # | Probe | Result |
|---|---|---|
| P1 | Scratch `bun test`: `renderDMConsole` with an empty world | no `[data-field="say-as"]`, no `[data-field="say-text"]`, no `[data-action="add-narration"]` |
| P2 | Scratch `bun test`: `renderPlayerPanel` for a dm, an agent and a player controlling `a1` | dm and agent: `say-as`, `say-text` and Send, placeholder "say or narrate"; player: `say-text` and Send, no `say-as`, placeholder "say something, or /ooc to step out of character" |
| P3 | The same for a dm controlling nothing | no Say; text "Your turnYou do not control an actor yet." |
| P4 | `env -u CLAUDECODE bun test` over `qa-say-boxes`, `spectator-view`, `qa-speaking`, `qa-refused-line`, `app`, `dm-view`, `command-surface`, `qa-refused-command`, `player`, `focus`, `qa-repaint-focus` | 610 pass, 0 fail, 11 files |
| P5 | `grep` for `renderPlayerPanel(` and `renderDMConsole(` under `client/` | panel: 1 in app.ts, 21 in 9 test files (`player.test.ts` 11, `spectator-view.test.ts` 4, one each elsewhere); console: 1 in app.ts, 25 in 8 test files (`qa-token-texts.test.ts` 13, `dm-view.test.ts` 6) |
| P6 | `measure` and `share_of` from `tools/check-comments.py`, on the files and on scratch drafts of `view/player.ts` | `view/player.ts` 56 of 264, 21.21, ceiling 21.3; `view/dm.ts` 265 of 707, 37.48, ceiling 37.5. The panel with the DM's Say deleted: two comments rewritten, 22.40; those two as citation-only lines, 21.46; and `// --- speaking ---` also gone, 52 of 246, 21.14 |
| P7 | `report_state` from `tools/ts-mutation-report.py` | `stale`, "the report's inputs changed since it was made", measured `db81c37` |
| P8 | `entry_faults(read_equivalents(…))` from `tools/check-ts-mutation.py` | 63 entries, no fault. `view/dm.ts` has four rows; `view/player.ts` none |
| P9 | `go build ./...`; `python3 tools/check-requirements-chain.py .` | builds; 526 rows, every citation resolves |
| P10 | `~/dev/RPTool/maptool` at `f4b7fef6c` | `MapToolFrame` builds one `CommandPanel` for every client; `globalIdentity` stays until `setGlobalIdentity` or `clearGlobalIdentity`; `commitCommand()` empties `commandTextArea`; `OOCMacro` (`/ooc`) is typed into the same box |
| P11 | `git grep -n "VTT-504\|VTT-519" -- client cmd internal docs/specifications tools` | `dm.ts` 2 comments; tests in `app.test.ts` 1, `dm-view.test.ts` 3, `qa-refused-command.test.ts` 4, `qa-refused-line.test.ts` 2, `qa-speaking.test.ts` 2, `spectator-view.test.ts` 2; SPEC-022 6 lines |
| P12 | `grep` for `say-` and `add-narration` in `client/e2e/` | `.player .say input[data-field=…]` in `refusals.spec.ts` (the DM's line), `speaking.spec.ts` and `typing.spec.ts` (a player's) |
| P13 | `grep` of `docs/specifications/` for the Say's names | SPEC-022 alone |
| P14 | A scratch copy of `view/dm.ts` with D1's Say group, driven by a scratch `bun test` | 5 of 5 pass: drawn with no actor, placeholder names `/ooc`; `Grimble`/`Hello` sends `as` `Grimble`; `/ooc brb` out of character; blank and bare `/ooc` send nothing; refused keeps both boxes, no repaint; accepted empties `say-text`, keeps `say-as`, one repaint after the answer; Send then Enter while held sends one, after release a press sends; a rejection reaches `console.error` once, no repaint, text kept. Share 266 of 724, 36.74 |
| P15 | `git grep -n 'console\.' client/src` | two lines, `sending` in `dm.ts` and `submit` in `view/player.ts` |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone (D15). Nothing is raised, widened or skipped.
  - 3: no contract change. The server is untouched.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `client/src` is a warning or a pointer. An added
    comment line carries no banned term. A block that gains a line ends at
    six lines or fewer. No file's share rises above its ceiling with an
    added comment line in it; `view/player.ts` gains none (D6).
- **The coverage floors are 100%** for `view/dm.ts` and `view/player.ts`.
  Each new branch has a test that reaches it: no actor and actors, a
  blank speaker and a named one, `/ooc` and not, a blank line, Enter and
  another key, each answer, the panel's role check.
- **Nobody starts Stryker** (SPEC-023). The report is already out of date
  (P7).
- **The adjudication rows stay true.** No line P8 names is edited or
  copied. `entry_faults` is run after each code task and finds no fault.
- **Module state survives tests.** `draft` and `unanswered` in `dm.ts` live
  for the whole bun process. Every test that types into the console's Say
  leaves `say-as` and `say-text` empty when it ends (D10). Every held
  answer is released inside its test.
- **An answer is awaited, never assumed**, with `settle()`.
- **A negative assertion needs its positive.** "Nothing was sent" comes
  with a press that sends.
- **A paint is compared by `outerHTML` and by each box's `.value`.**
  `outerHTML` does not carry what typing sets.
- **A test the register cites has a literal name.** The chain cannot
  resolve a template literal. The templated tests of
  `qa-speaking.test.ts` move but are cited by no row.
- `bun test` is measured with `env -u CLAUDECODE`.
- **Before every commit:**
  - `python3 tools/check-new-prose.py main`. The 85-column band and its
    SHORT rule hold for test files too;
  - `bunx tsc --noEmit -p client/tsconfig.json`;
  - `python3 tools/check-requirements-chain.py .`;
  - `python3 tools/check-comments.py main`, read for added-line refusals.
- **`cmd/vtt/webdist` is rebuilt by `task build:client`** after the last
  `client/src` change, and committed.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo a break. Use the inverse edit, checked
  with grep.
- **The review gate refuses untracked files.** Park scratch files outside
  the tree with `cp` and `cmp`.
- **Hash the tree before and after any reviewer.**
- **A new test goes after a closing brace**, or it inherits its
  neighbour's comment.

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c` (P10):

- **One box, always there, for every client.** `MapToolFrame` builds one
  `CommandPanel`, whoever the player is. The GM speaks from the same box
  as everyone, with nothing to be granted first.
- **The speaker stays.** `globalIdentity`, a `TokenIdentity`, is kept until
  `setGlobalIdentity` changes it or `clearGlobalIdentity` clears it.
- **`/ooc` is typed into that box.** `OOCMacro` is a slash command of the
  same panel.
- **The box empties at send.** `commitCommand()` clears `commandTextArea`
  and keeps the line in `commandHistory`.

Borrowed: one Say the DM always has (C1), a speaker that stays (VTT-521),
and `/ooc` in the same box, named where it is typed (C2). Not borrowed:
the box that empties at send, by the 2026-10-29 rulings; a player's free
identity, by the 2026-10-26 rulings (VTT-500); the command history, which is a
ticket of its own.

## Design questions for the owner

1. **Q1. Where in the console does the Say sit?**
   - Recommended: **first**, under the "DM console" heading and above
     Session, headed "Say something" as the panel's Say is.
     - It is the control a DM uses most at the table, and the console is
       long: thirteen groups when the table is full.
     - Of `client/test` and `client/e2e`, only "elements created without a
       class carry no class, not the string 'undefined'" reads the
       console's first heading, and only for its class, which a `group()`
       heading leaves empty (checked by `grep`). `typing.spec.ts` needs
       `session-name` below a 300-pixel viewport, which a group above it
       keeps.
   - Alternative: last, after Sharing this table; or between Session and
     Add actor.
   - **Answered 2026-10-30:** "Först, över Session", the recommendation.
2. **Q2. Do the console's boxes keep the names `say-as`, `say-text` and
   `add-narration`?**
   - Recommended: **yes.**
     - VTT-521 and VTT-522 name "a DM's or an agent's `say-as`". They stay
       true as worded, and their evidence moves.
     - A repaint finds the box by its `data-field`. No window draws two
       after Task 2, since a player's window has no console and a DM's
       panel no Say (C6).
     - Most selectors in the moved tests and specs port as they stand.
   - Alternative: new names, such as `console-say-as`. VTT-521 and VTT-522
     then name a box no window draws: both are withdrawn and succeeded,
     and every moved test changes its selectors.
   - **Answered 2026-10-30:** "Ja, båda" (with Q3), the recommendation.
3. **Q3. Where does the console's Say keep its text across a repaint?**
   - Recommended: **in `draft`, as every console form does**, and it sends
     through `sending` with `clears: ["say-text"]` (D1, P14).
     - One mechanism in the console: `sending` already gives refused,
       accepted, the lock, the invisible press and a rejected send.
     - No new `DMDeps` member, so the 25 test consoles and app.ts are
       untouched.
     - `git grep 'console\.' client/src` stays at its two handlers (P15),
       as SPEC-022 says.
     - The cost: `draft` and the lock outlive a test. Every test that
       types into the console's Say empties both boxes when it ends (D10).
   - Alternative: a state object of its own, as `PlayerUIState` is the
     panel's, handed in `DMDeps` and owned by app.ts. Tests are isolated,
     but the console holds text two ways, needs a second answer handler
     (a third `console.error`), and every `DMDeps` literal gains a line.
   - **Answered 2026-10-30:** "Ja, båda" (with Q2), the recommendation.
4. **Q4. Does a DM who controls an actor keep any trace of Say in the
   panel?**
   - Recommended: **none.** The panel keeps the chips, the abilities, the
     targets and the move hint; the Say is the console's alone, as the
     ruling says, "en enda plats att tala på".
   - Alternative: a line in the panel such as "Speak from the DM console."
   - **Answered 2026-10-30:** "Inget", the recommendation.

## Decisions

1. **D1. The console's Say is one more console form** (Q3).
   - `input("in character as… (optional)", "say-as")` and
     `input(<placeholder naming /ooc>, "say-text", "wide")`;
     `button("Send", say, "add-narration")`; a `keydown` on `say-text`
     that calls the same `say` for `Enter` and for no other key.
   - `say` is `sending(d, "add-narration", press)`. `press` reads the line
     with `readLine`; returns nothing when its text is empty; otherwise
     returns `addNarration(text, as)` in character, or
     `speakOutOfCharacter(text, as)`, with `as` the trimmed `say-as` or
     none when blank, and `clears: ["say-text"]`.
   - Forced by: Q3; P14, where this shape held every behaviour item 3
     names; VTT-517, C7 and C9 to C11, which `sending` then holds by
     construction.
   - The placeholder's words are the implementer's. They name `/ooc`, and
     differ from the speaker box's.
2. **D2. The Say group is the console's first** (Q1).
3. **D3. The boxes keep their names** (Q2).
4. **D4. `client/src/app.ts` does not change.** `renderDMConsole` already
   gets `send: act` and `rerender: paint`, and `isDM` draws it for the DM
   and the agent. Forced by D1.
5. **D5. The panel draws Say for a player only** (Q4).
   - The Say section starts with a role check; `speaksFreely`, the
     `say-as` box, `PlayerUIState.sayAs` and the `addNarration` import
     go. A player's `submit` sends `speakAs` or `speakOutOfCharacter` with
     no speaker, as now.
   - Forced by the ruling, and by the owner's "delete the old solution
     first" (2026-09-04): nothing keeps the DM's panel Say behind a flag.
6. **D6. `view/player.ts` gains no comment line** (G4, P6).
   - "Never draw or send a typed speaker for a player (VTT-500)." and the
     `// --- speaking ---` divider are deleted.
   - "Never empty or repaint at the press, nor touch `say-as` (VTT-521,
     VTT-524)." and `PlayerUIState`'s "What the Say boxes hold across
     paints (VTT-521, VTT-522)." become citation-only lines, `// VTT-524`
     and `// VTT-522`. A citation-only line counts in no share and is not
     an added comment line. A citation line for C6 may stand at the role
     check.
   - Measured on a scratch draft: 21.14 against 21.3, under the ceiling
     and inside the band. Rewriting the two comments as prose measured
     22.40, which `check-comments.py` refuses.
   - Forced by rule 2 and SPEC-010. The implementer re-measures with
     `--report` after the edit.
7. **D7. `dm.ts`'s comments.** The Say group carries at most a citation
   line for its ids. `sending`'s doc and `DMDeps.rerender`'s doc name C7
   for VTT-519. P14's share is 36.74 against 37.5, inside the band.
8. **D8. One `style.css` change.** `say-text` takes the console's existing
   `wide` class; `say-as` the console's default width, a preference E2's
   screenshot shows. Task 2 deletes `.say .as { width: 150px; }`, which no
   element carries once the panel draws no `say-as` ("delete the old
   solution first").
9. **D9. A line out of character from the console carries `say-as`**, as
   the DM's panel sent it. A preference: the server writes that line's
   speaker (VTT-497), so the wire changes nothing a window shows.
10. **D10. A helper empties the console's Say drafts.**
    `client/test/support/say.ts` exports `forgetTheSay()`. It draws a
    console with a stub `DMDeps` and types `""` into `say-as` and
    `say-text`. Every file whose tests type into the console's Say calls
    it in `afterEach`, `app.test.ts` in each such test's `finally`. Forced
    by `draft` living for the bun process: a name left in `say-as` shows in
    every later file's console. The name is a preference.
11. **D11. Rows that stay true keep their ids.** Per SPEC-008 and the
    `requirements` skill, a row is never reworded. A row the change makes
    false is withdrawn and succeeded; a row that stays true keeps its id,
    and its evidence follows the tests. So VTT-504 and VTT-519 are
    withdrawn, and the console's Say gets C9 to C11 rather than VTT-523,
    VTT-524 and VTT-526 being withdrawn to add one control to their lists.
12. **D12. The console first (Task 1), the panel second (Task 2).** The
    moved tests need a console Say to move to.
    - Between the two commits, a DM who controls an actor draws two Say
      forms that share their `data-field` names. A repaint then hands the
      focus to the first in tree order, the panel's.
    - No test fails on that window until A2, red on Task 1's commit and
      green on Task 2's: app.test's DM test reads it, and `querySelector`
      finds the panel's box first.
13. **D13. No DM Say test is deleted.** Each moves to the console, in its
    own file, under its own name where the register cites it. A name that
    says "panel" of the console's Say is renamed, and its evidence
    follows. Forced by the project's lesson that a deleted test takes
    coverage nobody named.
14. **D14. The browser tests.**
    - E1 rewrites `refusals.spec.ts`'s DM line: no grant, the console's
      boxes. The DM has been granted nothing in that file's campaign when
      it runs, and a narration needs no open session.
    - E2, after it in the same file: the DM is granted `act-cutter`, and
      the DM's page draws the panel with no Say and one `say-text`, the
      console's. Its screenshot answers G7's second point.
    - `speaking.spec.ts` and `typing.spec.ts` stay as they are.
15. **D15. `python3 tools/check-comments.py --write-ledger` runs once,
    after the last code commit, and only if `check-comments.py main` finds
    a file more than 1.0 under its ceiling.** P6 and P14 put both views
    inside the band. A lowered row cannot be raised again.
16. **D16. No Stryker run.** At Task 6 the owner is told that the stored
    report measured `db81c37`, with the tickets since as the step lists
    them, and that `task check:ts-mutation:full` is his to ask for.

## Requirements

Allocated at Task 0, 2026-10-30: C1 VTT-527, C2 VTT-528, C3 VTT-529, C4
VTT-530, C5 VTT-531, C6 VTT-532, C7 VTT-533, C8 VTT-534, C9 VTT-535, C10
VTT-536, C11 VTT-537. VTT-504 and VTT-519 withdrawn.

The candidates of the sort, allocated above with `requirement-id`, each
`**OPEN — no test yet**` until its tests land. The C, K and test labels
are this plan's.

- **C1.** "The DM console draws a speaker box `say-as`, a line box
  `say-text` and a Send, whether or not its DM or agent controls an
  actor." S1, A1. Red under K1.
- **C2.** "The line box of the DM console's Say and of a player's panel
  names `/ooc` in its placeholder." S2, U2. Red under K2.
- **C3.** "The DM console's Send, or Enter in its `say-text`, sends a line
  whose first word is not `/ooc` in character, with what `say-as` holds,
  trimmed, as `as`, and no `as` when it is blank." S3, S6, A1, E1. Red
  under K3; K11 reddens S6's other-key half, which no row holds.
- **C4.** "A line the DM console sends whose first word is `/ooc` goes out
  of character, with the rest of the line as its text." S4. Red under K4.
- **C5.** "The DM console sends nothing for a line that is blank, or that
  is `/ooc` with nothing after it." S5. Red under K5.
- **C6.** "The player panel draws a Say form only for a player." U1, A2,
  E2. Red under K9, K10. With C1 and C3, it succeeds VTT-504.
- **C7.** "A command the server accepts leaves each DM console field that
  fed it as it is drawn when nothing has been typed in it, except the
  notes form's key, title and text, and `say-as`."
  - VTT-519's successor: its tests re-cite C7, and S8, A1 and E1 add the Say.
    Red under K7, and under the K3 (`d.rerender()` removed) and K4 (the
    accepted branch's `clearDraft` removed) of
    `2026-10-29-a-refused-command-keeps-what-was-typed`'s plan. K6 reddens
    VTT-521, which S8 also cites.
- **C8.** "After a repaint, the DM console's `say-text` shows what was
  typed in it since the server last accepted a line from the console."
  S12 and the ported `qa-say-boxes.test.ts`. Red under K13.
- **C9.** "While a line the DM console's Send or Enter in its `say-text`
  sent is unanswered, Send and Enter there send nothing." S9. Red under
  K8, K14.
- **C10.** "A press of the DM console's Send, or Enter in its `say-text`,
  that sends a line changes nothing the console draws until that line is
  answered." S10. Red under K12.
- **C11.** "A line the DM console's Send or Enter in its `say-text` sent,
  whose promise rejects, clears nothing, asks for no repaint, frees Send
  and Enter, and is reported with console.error." S11. Red under K14.

The sort, and what it rejected:
- "The page holds one line box" (item 5): it follows from C1 and C6. A1
  and A2 observe it.
- The browser test (item 6): an observation of C3, VTT-521 and C7 (E1),
  and of C6 (E2). No row of its own.
- "`speaking.spec.ts` still passes": a guard. VTT-496, VTT-497 and VTT-500
  to VTT-502 cite it.
- Where the Say sits, its heading, its field names, its classes: how (D2,
  D3, D8).
- "The agent's window gains the Say": C1 covers both roles.
- A DM who controls an actor keeps the chips and abilities: existing
  behaviour, which SPEC-022 records; not this ticket's.
- "The server is unchanged": not a rule. VTT-497 to VTT-499 hold the
  server.
- The `addNarration` row's comment in `command-surface.test.ts`: how.
  "every command surfaced in a panel has a control a person can reach"
  holds the row.
- That an out-of-character line from the console carries `say-as`: a
  preference (D9), which the server overwrites (VTT-497).
- The console's `say-as` after a repaint: VTT-522, as worded (Q2).
- A refused console line keeping both boxes: VTT-517, as worded.

Withdrawn at Task 0, in the register's form, `WITHDRAWN <date>: <its
text>. <why>. Succeeded by …`, with the evidence `**READING —
verify-ticket check 5 of <date>; withdrawn, held by nothing**`:
- **VTT-504**: false once the panel draws Say for a player only, by "I
  konsolen, alltid". Succeeded by C1, C3 and C6.
- **VTT-519**: false once the console's Say keeps `say-as` after an
  accepted line. Succeeded by C7.

Kept, wording still true, evidence changed:
- **VTT-500**: "a player's /ooc sends no speaker, whatever the panel's
  state holds" is renamed (it set `sayAs`, which goes).
- **VTT-501**: loses "a DM's /ooc line goes out of character and still
  carries the speaker box", which holds C4.
- **VTT-517**: gains S7 and the console's refused-line tests.
- **VTT-518**: loses "a DM's or an agent's refused line leaves say-as and
  say-text as typed", which holds VTT-517.
- **VTT-520**: loses its DM entries, which hold C7: two in
  `spectator-view.test.ts`, the one in `app.test.ts`, the `qaSay` one,
  and E1's.
- **VTT-521** and **VTT-522**: the same names, now the console's (D13);
  VTT-522 gains C8's companions where they cite it.
- **VTT-524**: "a press of Send or Enter that sends a line draws nothing
  new until its answer" keeps its name and holds C10 for its DM half.
- **VTT-491**: its `qaSay` and "what a DM typed in Say" entries keep
  their names, on the console.
- **VTT-523** and **VTT-526**: unchanged; their cited tests are a
  player's.

| Ticket item | Observed by |
|---|---|
| 1 | S1, S2 |
| 2 | S3, S4, S5, S6 |
| 3 | S7 refused, S8 accepted, S9 lock, S10 press, S11 rejection, S12 repaint |
| 4 | U1, U2 |
| 5 | A1, A2 |
| 6 | E1, E2; `speaking.spec.ts` unchanged |

## The existing tests that change

- **`client/test/qa-say-boxes.test.ts`** (all 19 are a DM's):
  - `table()` draws, as `startSession`'s paint does, the DM's panel (no
    Say after Task 2) and the console, `rerender` repainting, into
    `renderSpectator`;
  - `hidePanel` becomes `hideConsole`; "qaSay: a paint without the panel in
    between loses nothing typed" is renamed for the console;
  - the file's opening comment says it is the console's Say;
  - `afterEach` calls `forgetTheSay()`;
  - each test cites the row its observation makes: `say-text` halves C8 or
    C7, `say-as` halves VTT-521 or VTT-522.
- **`client/test/spectator-view.test.ts`:**
  - `sayTable(who)` draws the console for a DM or an agent and the panel
    for a player, and finds the boxes by `data-field`, since the console's
    sit in no `.say`;
  - "what a DM typed in Say survives a repaint, with the as box focused",
    "an accepted Send empties say-text and leaves say-as for the paints
    after it", "a DM's or an agent's say-as keeps its name after an
    accepted line, and the next line speaks as it" and "a DM's or an
    agent's refused line leaves say-as and say-text as typed" run on it,
    names kept; the first's comment about "every one the DM console draws
    beside them" is false and goes;
  - VTT-524's "a press of Send or Enter that sends a line draws nothing
    new until its answer" keeps both roles, the DM's on the console;
  - "a DM's /ooc line goes out of character and still carries the speaker
    box" and "an in-character speaker is carried, and a blank one is
    omitted" move from `panel(…, dmMe)` to `sayTable`, names kept,
    re-citing VTT-530 and VTT-529;
  - "a player's /ooc sends no speaker, whatever the panel's state holds"
    loses `{ sayAs: "DM" }` and its last clause;
  - "the panel's structure carries the classes the stylesheet targets"
    loses its `dmMe` `.as` line; "both say-something boxes carry a
    placeholder" reads the console's two boxes and a player's line box;
  - new U1, U2; `afterEach` calls `forgetTheSay()`.
- **`client/test/qa-speaking.test.ts`:** `rig(me)` draws the console for a
  DM or an agent; the "DM and agent" templates keep their bodies, "panel
  draws the as box" becomes "console draws the as box", their VTT-504
  comments name C1 and C3, and the rest re-cite as
  `qa-refused-line.test.ts`'s do; `afterEach` calls `forgetTheSay()`.
- **`client/test/qa-refused-line.test.ts`:** `table(role, answer)` draws
  the console for `dm` and `agent`. The twelve DM and agent tests keep
  their names and re-cite: VTT-504 to C3, VTT-501 to C4, VTT-518 to
  VTT-517, VTT-520 to C7, VTT-523 to C9, VTT-524 to C10, VTT-526 to C11,
  and a `say-text` half of VTT-522 to C8. The opening comment
  names the console's Say beside the panel's. `afterEach` calls
  `forgetTheSay()`.
- **`client/test/app.test.ts`:** "a DM's accepted line keeps the speaker's
  name in say-as and empties say-text" reads the console's boxes from Task
  2 on, empties both in its `finally`, and cites C7 and VTT-521; new A1
  and A2; the "// VTT-519" citation names C7.
- **`client/test/dm-view.test.ts`:** new S1 to S12; "every input the
  console owns is reachable by its stable data-field" and "every box
  carries a placeholder saying what belongs in it" gain `say-as` and
  `say-text`; "the console's styling hooks are the ones the stylesheet
  targets" gains `say-text` as `wide`; its three VTT-519 citations name
  C7.
- **`client/test/qa-refused-command.test.ts`:** its four VTT-519
  citations name C7. No assertion changes.
- **`client/test/command-surface.test.ts`:** the six-line comment above
  `addNarration` goes; the row's `dm-console` is now true.
- **`client/e2e/refusals.spec.ts`:** D14.
- Every other file P5 names draws the new boxes and asserts nothing about
  them. Each is run.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off, with answers to Q1 to Q4.
  - The ticket's writer corrects G1's path, and G6's wording if he
    chooses.
  - Allocate C1 to C11 with `requirement-id`, adjusted to the answers.
  - Withdraw VTT-504 and VTT-519 in the register's form (Requirements).
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean; the rows exist; VTT-504 and
  VTT-519 read `WITHDRAWN` and name their successors; the chain passes.

### Task 1: the console's Say (items 1 to 3; C1 to C5, C7 to C11; D1 to D3, D7, D10)

- **Files:**
  - `client/src/view/dm.ts`: the Say group, imports of `addNarration`,
    `speakOutOfCharacter` and `readLine`, the two C7 pointers;
  - new `client/test/support/say.ts`;
  - `client/test/dm-view.test.ts`, `client/test/app.test.ts` (A1 and the
    citation), `client/test/qa-refused-command.test.ts` (citations);
  - `docs/requirements.md`: the evidence of C1, C2's console half, C3 to
    C5, C7 to C11, and VTT-517.
- **New tests**, in `dm-view.test.ts` unless named, each with a literal
  name and its id line:
  - **S1** (C1): an empty world, and a world whose actors someone
    controls: the console draws `say-as`, `say-text` and Send.
  - **S2** (C2): `say-text`'s placeholder contains `/ooc`, and differs
    from `say-as`'s.
  - **S3** (C3): `  Grimble ` and `Hello`, Send: one `addNarration`, text
    `Hello`, `as` `Grimble`, not out of character, no actor id. A blank
    `say-as` sends no `as`.
  - **S4** (C4): `/ooc brb` sends one line out of character, text `brb`;
    `/oocx hi` and `say /ooc now` go in character as typed.
  - **S5** (C5): a blank line, `/ooc` and `/ooc   ` send nothing and take
    no lock: a line typed after them sends.
  - **S6** (C3): Enter in `say-text` sends; another key does not.
  - **S7** (VTT-517): refused, in character and out: the next render's
    `say-as` and `say-text` read as typed; no repaint asked.
  - **S8** (C7, VTT-521): accepted: the next render's `say-text` is `""`
    and `say-as` `Grimble`; one repaint, asked after the answer, none
    before; `Bye` next carries `as` `Grimble`. Text typed into `say-text`
    after Send and before the answer is emptied too ("Töm ändå").
  - **S9** (C9): with a held answer, Send twice, Send then Enter, and Enter
    twice each send one line; after the release a press sends; while the
    line is held, another console control still sends, its answer
    released and its field emptied inside the test.
  - **S10** (C10): a press that sends, by Send and by Enter, leaves the next
    render's `outerHTML` and every box's `.value` as before the press; no
    repaint asked.
  - **S11** (C11): a send that rejects: `console.error` gets the error,
    no repaint, both boxes as typed at the next render, and a second press,
    by Enter, sends.
  - **S12** (C8): typed text survives a render; after an accepted line,
    text typed since then survives one.
  - **A1** (item 5; C1, C3, C7), `app.test.ts`: a DM controlling no actor,
    `Hello` in `say-text`, Enter: the socket gets one `add_narration` with
    text `Hello`; the page holds one `say-text`, inside `.dm`; the socket
    answers `ok`: `say-text` reads `""`.
- **Order:**
  1. Write the tests on Task 0's commit. All are red: there is no console
     Say.
  2. Then write the code.
  3. Then make the breaks.
- **True when:**
  - P4's eleven files and the other files of P5 are green, `support/say.ts`
    typechecks,
    with no test skipped, run once in one process so a leaked draft would
    show;
  - the typecheck is green; `entry_faults` finds none;
  - `check-comments.py --report` shows `view/dm.ts` at or under 37.5 and
    over 36.5, the band's lower edge: the drafts measured 36.60 and 36.74,
    so a few more code lines cross it (D15).
- **Breaks**, each undone by its inverse edit:
  - **K1:** the Say group drawn only when the world has an actor. S1's
    empty half and A1 red.
  - **K2:** `/ooc` taken out of the placeholder. S2 red.
  - **K3:** `as` never passed. S3 and S8's second line red.
  - **K4:** `readLine`'s `ooc` ignored, every line in character. S4 red.
  - **K5:** the empty-text return removed. S5 red.
  - **K6:** `clears: ["say-text", "say-as"]`. S8 red (VTT-521).
  - **K7:** `clears: []`. S8 and S12 red.
  - **K8:** Enter given its own `sending` key. S9's mixed pair red.
  - **K11:** Enter's key check removed. S6 red.
  - **K12:** the press deletes `draft["say-text"]` before it returns. S7
    and S10 red.
  - **K13:** `say-text` built without reading `draft`. S12 red.
  - **K14:** the Say sends through `d.send` outside `sending`. S9 and S11
    red.

### Task 2: the panel's Say for a player only (item 4, the DM's tests move; C2, C6; D5, D6, D12, D13)

- **Files:**
  - `client/src/view/player.ts`: the role check, the deletions, the
    comments per D6;
  - `client/test/spectator-view.test.ts`, `qa-say-boxes.test.ts`,
    `qa-speaking.test.ts`, `qa-refused-line.test.ts`, `app.test.ts`,
    `command-surface.test.ts`;
  - `docs/requirements.md`: C2's panel half, C6 and the kept rows'
    evidence.
- **New tests:**
  - **U1** (C6), `spectator-view.test.ts`: the panel for `dmMe` and
    `agentMe`, each controlling `a1`, draws no `say-as`, no `say-text`, no
    `add-narration` and no "Say something"; for a player, `say-text` and
    Send.
  - **U2** (C2): a player's `say-text` placeholder contains `/ooc`.
  - **A2** (C6), `app.test.ts`: the DM and the agent controlling `a1`
    (`tableAs`): the page holds one `say-text`, inside `.dm`, and `.player`
    holds no `add-narration`; a player's page holds one, inside `.player`.
- **Order:**
  1. Move the DM's tests per "The existing tests that change", on Task 1's
     commit. They are green there, except where a window draws both forms
     and a test counts one box.
  2. Write U1, U2 and A2. U1's DM and agent half and A2 are red.
  3. Then the code, then the breaks.
- **True when:**
  - green as Task 1's, the typecheck included; `entry_faults` none;
  - `check-comments.py --report` shows `view/player.ts` at or under 21.3,
    and `check-comments.py main` finds no added comment line in it;
  - `git grep -n "VTT-504\|VTT-519\|ui\.sayAs\|sayAs?:\|speaksFreely" -- client` prints
    nothing.
- **Breaks:**
  - **K9:** the role check removed. U1 and A2 red.
  - **K10:** the role check admits the agent too. U1's agent half and A2
    red.

### Task 3: the browser (item 6; D14)

- **Files:** `client/e2e/refusals.spec.ts`; `cmd/vtt/webdist`.
- **Tests**, each with its id line above it:
  - **E1** rewrites "a DM's accepted line empties say-text and keeps
    say-as for the next line": no `asDM`; `.dm [data-field="say-as"]`
    `Grimble`, `say-text` `Hello`, Enter. The feed shows `Grimble: Hello`;
    `say-text` reads `""` and `say-as` `Grimble`; `Bye` shows as
    `Grimble: Bye`. The page holds no `.player [data-field="say-text"]`.
    Cites C3, C7 and VTT-521.
  - **E2:** `asDM` starts a session, loads `goblin-ambush` and grants
    `act-cutter` to `ids.dm` as `ACTOR_KIND_NON_PARTY`. The DM's page
    shows the panel's chip, no `say-text` in `.player` and one in `.dm`.
    A screenshot of the page. Cites C6.
- **Order:**
  1. `task build:client`.
  2. `task e2e` whole, after checking the machine's load.
  3. Commit `cmd/vtt/webdist` with the spec.
- **True when:** every spec passes, `speaking.spec.ts` and `typing.spec.ts`
  among them; E2's screenshot is in `client/e2e/.artifacts/`.

### Task 4: the report

- **Do:** write `docs/reports/2026-10-30-the-dm-speaks-from-the-console.md`
  and give it its reading review.
- **It covers:** each "Done looks like" item with its observation; K1 to
  K14 with what went red; Rule 9; G1 to G7 and Q1 to Q4's answers; D6's
  measurement; E2's screenshot; Phase 4a's adjudications under their own
  heading. Leave room for the gate's figures.
- **True when:** it answers the ticket's six items in its numbering, each
  `[x]` naming its observation.

### Task 5: the specification

With the `specification` skill, from the report, SPEC-022:
- the Status paragraph: the Say boxes' sentence, the console's sentence
  and the speaking sentence name the console's Say, its tests and the new
  ids, and drop VTT-504 and VTT-519;
- the repaint paragraph: the console's Say boxes come from `draft`; the
  panel's `say-text`, a player's, from `PlayerUIState.sayText`; `sayAs`
  goes;
- "Saying something": the console draws `say-as`, `say-text` and Send for
  the DM and the agent, whatever they control; the panel draws `say-text`
  for a player alone; both line boxes name `/ooc`; the console sends by
  `addNarration` or `speakOutOfCharacter` with `say-as`;
- "What a command's answer leaves typed": the console's Say among the
  controls that send through `sending`; `say-as` among the fields an
  accepted command leaves (C7);
- "One press, one command": the Say's Send and Enter share one lock,
  keyed `add-narration`;
- "The console's rewrite": VTT-519 becomes C7;
- the Requirements list: loses VTT-504 and VTT-519, gains C1 to C11.

**True when:** each changed sentence has been run against the code by
someone other than its writer; `git grep -n "VTT-504\|VTT-519\|sayAs" docs/specifications`
prints nothing; and every id range in the Status paragraph is read, since
a range broken across a line hides an id from that grep.

### Task 6: the ledger, the gate, whole, and the commits

- **Do:**
  - Run `python3 tools/check-comments.py main`. If it finds a file more
    than 1.0 under its ceiling, run `--write-ledger` once and commit the
    ledger (D15).
  - Check free space and the machine's load.
  - Run `task check` whole on the last code commit. Launch it with
    `start_new_session=True`, send all output to a file, and require the
    gate's own completion line and `check:ts-mutation`'s own line.
  - Write the figures into the report. The client's mutation was not run,
    by the owner's rule of 2026-10-26. Say what the step printed.
  - Commit the report, then the specification.
  - Tell the owner D16's reminder.
- **True when:**
  - the gate is green, `check:comments`, `check:new-prose`,
    `check:requirements-chain` and `check:mutation` included;
  - `check:ts-mutation` printed that the client's mutation was not
    measured, naming `db81c37` and this ticket;
  - the log holds no Stryker line.
- Push and merge on the owner's word.

## For Phase 4a

Hand the QA agent:
- the allocated rules' texts, C1 to C11, and VTT-517, VTT-521 and VTT-522;
- SPEC-022's changed paragraphs;
- the surfaces, as signatures and names, never the source:
  - `renderDMConsole(d: DMDeps): HTMLElement`, with `DMDeps`'s members
    (`send` resolving to a `CommandResult`, and `rerender`);
  - `renderPlayerPanel(st, me, abilities, ui, send, rerender, switcher?)`
    and `PlayerUIState`;
  - `renderSpectator(root, st, log, status, extras)`, for a repaint;
  - `CommandResultSchema` and `create`, for an answer;
  - the Say's `data-field` and `data-action` names;
  - `client/test/support/dom`, `client/test/support/answers.ts` and
    `client/test/support/say.ts`.

Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- measure with `env -u CLAUDECODE bun test <file>`;
- the console's typed text and its locks persist across renders and
  across test files in one bun process: release every held answer inside
  its test, and end every test with `forgetTheSay()`;
- compare a paint by `outerHTML` and by each box's `.value`;
- an answer is awaited, never assumed, and a "nothing was sent" assertion
  comes with a press that sends;
- give each test a literal name;
- its tests go in `client/test/qa-console-say.test.ts`, each citing its
  rule's id. The tree is hashed before and after it.
