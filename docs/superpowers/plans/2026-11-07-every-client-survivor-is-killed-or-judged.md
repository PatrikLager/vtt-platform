# Every client mutant that survived at 37929bf is killed or adjudicated equivalent — plan

Ticket: `docs/superpowers/specs/2026-11-07-every-client-survivor-is-killed-or-judged-design.md`,
verified by an agent that did not write it: passes with gaps, carried below.
Branch `fix/every-client-survivor-is-judged` from main `37929bf`, in
`~/dev/vtt-platform-visibility`, the checkout that holds the stored report.
Asked for by the owner on 2026-11-07 ("kan du inte reda ut överlevarna
nu?"). The plan itself was not put to him before the work, by his standing
rule to ask only at design questions; D3's open choice goes to him with the
report.

## Constraints

- CLAUDE.md rule 2: the gate is not weakened. Nothing in `stryker.conf.json`,
  `tools/ts-mutation-report.py` or the hashed input list changes, and
  `tools/check-ts-mutation.py` gains only the Regex offset rule of D7.
- SPEC-020: an entry is keyed as the checker prints it, and carries its code
  line and its reason.
- SPEC-023 and the owner's rule of 2026-10-26: the full run happens on the
  owner's word.
- SPEC-008: an id comes from the dispenser; a row false as worded is
  withdrawn and succeeded, never edited.
- CLAUDE.md rule 10: a comment in a test is a warning or a pointer.
- `tools/check-new-prose.py` reads the lines a change adds to `.ts` files,
  tests included; `tools/check-comments.py` reads `internal/`, `cmd/` and
  `client/src/`, so here only `readLine`'s doc comment. Both run before each
  commit, the ledger last.

## Decisions

- D1. The entries come before every other edit. Forced by the stamp: only a
  current report judges an entry, and any edit under `client/test` or
  `client/src` makes it out of date.
- D2. Every mutant some input tells apart is killed, never excused: the
  sentinel id in `client/src/session.ts`, and the mutants only a direct call
  with an argument no caller in `client/src` passes tells apart (the
  `notify?.` pair in `client/src/view/player.ts`, the `actor?.` in
  `client/src/view/feed.ts`). Forced by the header of
  `tools/ts-mutation-equivalents.txt` ("Prefer killing a mutant to excusing
  it"), by its `client/src/view/player.ts` section, which records a mutant
  reachable only past the fold killed rather than excused, and by SPEC-018:
  the fold bounds an actor id in bytes and nothing else, so the sentinel id
  can be folded. The sentinel test carries a warning that the id must stay
  the string Stryker writes; the direct-call tests hold no row.
- D3. No change under `client/src` but VTT-592's citation in `readLine`'s doc
  comment. The survivors are tests that do not look, not code that is wrong.
  Making `renderPlayerPanel`'s `notify` required would remove two mutants by
  changing an argument SPEC-022 describes as optional; that is the owner's to
  choose, and the tests leave it open.
- D4. VTT-592 is withdrawn, false as worded for a `/w` line whose next word
  opens with a double quote that no later one closes (the verifier's G2), and
  succeeded by a row that says it. Two rules are new: the player panel's
  blank line, and what Boot's question says. The tests that kill those
  mutants assert what no row states, and a test stricter than its row goes
  red on correct work.
- D5. The whisper refusals and Boot's question are asserted by what each
  says, not word for word: that the name is missing, that two party members
  share it, and Boot's three consequences. Forced by D4's own reason: VTT-594
  says the console tells the DM why, and VTT-667 names what the question
  says; a test of the whole text goes red on a rewording both allow.
- D6. Phase 4a runs over VTT-665, VTT-666 and VTT-667, which describe
  behaviour SPEC-022 and SPEC-009 will carry: one QA agent, handed those rows
  and the specification sentences, never the source or the tests. The other
  tests are shown to fail by their own mutants applied by hand, and each
  entry by its survivor reproduced by hand; the reading review reads both.
- D7. `tools/check-ts-mutation.py` gains a Regex offset rule, found while
  doing Task 1 and not foreseen here. Forced by the checker's own test
  `test_every_real_adjudication_still_points_at_its_mutant`, which refuses an
  entry whose mutator has no position rule; the `readLine` entries are the
  file's first Regex entries. The rule, that the offset opens the literal at
  its `/`, is the stricter of the two tables a regular expression could go
  in: `NO_POSITION_RULE` would check no column. An entry with a wrong offset can only go stale,
  never excuse a mutant. SPEC-020 lists the column rules and moves with it.

## The requirements sort

Accepted, each with the observation that goes red:

- VTT-592's successor, VTT-665: `readLine('/w "  Unclosed')` reads the name
  `Unclosed`, and `rest.slice(1).trim()` as `rest.slice(1)` reads
  `  Unclosed`; `readLine('/w Tobbe /dm hi')` reads a whisper to Tobbe, and
  the `/dm` pattern with its `^` dropped reads a whisper to the DM.
- VTT-666, the player panel sends nothing and tells the player nothing for
  a blank line or a bare `/ooc`: with `line.kind === "toDM"` as `true` it
  tells the empty `/dm` hint.
- VTT-667, Boot's question says the three consequences: with either of its
  later strings emptied, the question loses one.

Refused:

- "Given no `notify`, the panel raises no error": a rule about an argument no
  caller in `client/src` omits; its test kills the mutants and holds no row.
- "`buildFeed` keeps an entry after a bare removal that is an `actorAdded`
  with no actor": a log the fold refuses never reaches it; the same.
- A rule per refusal text of `whisperTargets`: VTT-594's "tells the DM why"
  already holds each text.

## Tasks

1. **The entries.** `tools/ts-mutation-equivalents.txt`, its
   `client/src/player.ts` section. Done when the ticket's items 1 and 2
   hold: the check prints SURVIVED for none of the entries' mutants, and for
   each when its entry is removed from a copy; each mutation applied in a
   copy of the tree outside the repository leaves `bun test client/test`
   passing.
2. **The tests and the register.** `client/test/player.test.ts`,
   `dm-view.test.ts`, `spectator-view.test.ts`, `app.test.ts`,
   `session.test.ts`, `feed.test.ts`; `docs/requirements.md` (the rows from
   the dispenser, VTT-592 withdrawn, evidence on VTT-457, VTT-594, VTT-605,
   VTT-606 and VTT-662); VTT-592's citations in `client/src/player.ts`,
   `client/test/player.test.ts` and `client/test/qa-whisper-say.test.ts`
   pointed at its successor. Done when the ticket's item 3 holds: each
   mutation applied by hand fails a named new test, and the unmutated tree
   passes `bun test`, `task client:typecheck` and `task lint`.
3. **Phase 4a** over VTT-665, VTT-666 and VTT-667 (D6), before Task 2's
   commit and its reading review. QA's passing tests are committed with
   Task 2's; each adjudication goes in the report.
4. **The report**, `docs/reports/2026-11-07-every-client-survivor-is-killed-or-judged.md`.
5. **The specifications**: SPEC-022 (VTT-592's three citations, near the
   list of what the client holds, in the Say paragraph and in its
   Requirements, pointed at its successor; the panel's blank line, VTT-666),
   SPEC-009 (Boot's question, VTT-667) and SPEC-020 (the Regex column rule,
   D7).
6. **The gate**, last, on the branch's final tree: `task check` (item 5).
   The full run on the owner's word (item 4).
