# Every client mutant that survived at 37929bf is killed or adjudicated equivalent — report

Ticket: `docs/superpowers/specs/2026-11-07-every-client-survivor-is-killed-or-judged-design.md`.
Plan: `docs/superpowers/plans/2026-11-07-every-client-survivor-is-killed-or-judged.md`.
Branch `fix/every-client-survivor-is-judged` from main `37929bf`. Commits:
`25711db` (ticket, plan, register), `869a303` (Task 1), `5e13196` (Task 2, the
last code commit). Every code reference below is to `5e13196`.

## Done looks like

1. [x] **Each mutant no input tells apart has an entry.** At `869a303`,
   against the stored report of `37929bf`, `python3
   tools/ts-mutation-report.py check` printed `SURVIVED` for no entry's
   mutant, and only for the mutants item 3 kills. With each entry of
   `tools/ts-mutation-equivalents.txt`'s `readLine` block removed in turn from
   a copy of the file, `python3 tools/check-ts-mutation.py <copy>` printed
   `SURVIVED` for that entry's mutant again. The reading review reproduced
   both.
2. [x] **The entries are true of the code.** Each of those mutations, applied
   in a copy of the tree outside the repository, left `bun test client/test`
   green. The ticket's verifier and the reading review each compared the
   original `readLine` with every mutant over generated inputs, the review
   under both Bun and Node, and found no input that tells them apart, while
   the same comparison told apart the `readLine` mutants item 3 kills.
3. [x] **Each other mutant fails a test this ticket adds.** Applied by hand in
   a copy of the tree, against the whole of `client/test`; the unmutated copy
   passes:

   | Mutant | Fails |
   |---|---|
   | `readLine`'s `/dm` pattern without `^`; `rest.slice(1).trim()` and `rest.slice(1, close).trim()` without `trim()` | `player.test.ts`'s "a /w name in double quotes is trimmed, closed or not, and a /dm later in a /w line is its text", and a test each in `qa-survivor-rules.test.ts` |
   | `whisperTargets`'s `name === ""` as `false`, its `""` as `"Stryker was here!"`, `named.length > 1` as `false` | `dm-view.test.ts`'s "a /w line with no name, or with a name two party members share, says which" |
   | Boot's question with its second or third string emptied | `qa-survivor-rules.test.ts`'s "Boot's question to the DM" tests |
   | `submit`'s `line.kind === "toDM"` as `true` | `spectator-view.test.ts`'s "a blank line or a bare /ooc in the player panel sends nothing and tells nothing", and QA's panel tests |
   | `submit`'s `notify?.(...)` as `notify(...)`, at either hint | `spectator-view.test.ts`'s "a panel handed no notify raises no error for a /w line or a bare /dm, and sends nothing" |
   | `boot`'s `me?.role` as `me.role` | `app.test.ts`'s "sets that drop a party member before /api/me answers leave the window offering the last one, and no toast" |
   | `Session`'s kept `[]` as `["Stryker was here"]` | `session.test.ts`'s "a set saying the participant controls none tells every party member the set before named, whatever its id" |
   | `reintroductions`'s `back.case !== "actorAdded"` as `false` | `feed.test.ts`'s "a bare removal is told when the entry after it holds nothing this client reads" |
   | `reintroductions`'s `back.value.actor?.actorId` as `back.value.actor.actorId` | `feed.test.ts`'s "buildFeed tells a bare removal and an actorAdded with no actor after it" |

   Each of these survived the suite at `37929bf`: the stored report says so.
4. [ ] **`task check:ts-mutation:full` reports no unadjudicated survivor.**
   Not run: the full run happens on the owner's word, and has not been asked
   for. The item stands. Until it runs, `task check` says the client's
   mutation was not measured, because the tests added here make the stored
   report out of date.
5. [?] **`task check` exits 0 on the branch.** Not run when this report was
   written. Free space on the machine was below the floor `check:mutation`
   refuses to start under (`MIN_FREE_BYTES` in `tools/check-mutation.py`), and
   what would free enough, emptying the Go build cache every Go project on the
   machine shares, is the owner's to allow. The steps that read the records
   were run on the branch's final tree instead, each clean:
   `tools/check-requirements-chain.py` and `tools/check-new-prose.py`; so was
   `check:doc-owner`; and on `5e13196` the client's own: `bun test
   client/test`, `task client:typecheck`, `task build:client` (no change to
   `cmd/vtt/webdist`) and the client mutation tools' self-tests. Whether the
   whole gate passes is not established.

## What the rules became

- The player panel's blank line or bare `/ooc`: **VTT-666**.
- What Boot's question says: **VTT-667**.
- VTT-592's reading with the unclosed quote said: **VTT-665**, then, when
  Phase 4a found its "it" could be read as the next word, withdrawn and
  succeeded by **VTT-668**. VTT-592 is withdrawn, succeeded by VTT-665.
- Refused at the plan's sort, each with its reason there: a rule that the
  panel raises no error without `notify`, a rule for `buildFeed` on an
  `actorAdded` with no actor, and a rule per refusal text of `whisperTargets`.
  The two direct-call tests hold no row.

## Phase 4a adjudications

QA was handed VTT-665, VTT-666 and VTT-667, the governing sentences of
SPEC-022 and SPEC-009, and the declaration surfaces, and wrote
`client/test/qa-survivor-rules.test.ts`. No test failed.

- "It" in VTT-665 can be read as the next word: a defect in the row. VTT-665
  is withdrawn; VTT-668 says "that opening quote" and "what follows the
  opening quote".
- Whether "the words between" the quotes keeps their spacing: VTT-668 says
  "what stands between", so it does, as the code does.
- `/w "" hi`: VTT-668 reads an empty name with the text `hi`; the console
  refuses a whisper with no name (VTT-594).
- A tab between words: a word ends at whitespace; nothing to change.
- `/OOC` alone: SPEC-022 says the first word must be exactly `/ooc`, so it is
  a line in character; nothing to change.
- "Tells nothing" read as no call of `notify`: the panel tells the player
  through `notify` and draws no message of its own; nothing to change.
- VTT-667 holds the same question for a player's and a spectator's row,
  whatever the link's state: the row states it unconditionally. QA's tests
  hold its meaning, not its words, as the plan's D5 says.
- QA's Boot tests cited the range "VTT-568 to VTT-571": made VTT-569 and
  VTT-570.
- QA's VTT-569 test asserted that the question leaves the participant's id
  out, which no row states (found by the reading review): that assertion is
  removed.
- QA's tests that a line whose first word is not `/w` or `/dm` is no whisper
  cite VTT-668 for its converse (found by the reading review): kept, as
  `client/test/qa-whisper-say.test.ts` does.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Nothing in `tools/check-ts-mutation.py` changes | It gains a Regex offset rule, with the tests `test_a_regex_key_whose_column_does_not_open_a_regex_is_rejected` and `test_a_regex_key_on_its_slash_is_accepted` (the plan's D7, added during Task 1); SPEC-020, not in the ticket's list, moves with it | Its own test `test_every_real_adjudication_still_points_at_its_mutant` refuses an entry whose mutator has no position rule, and the `readLine` entries are the file's first Regex entries. The rule is the stricter of the two tables a regular expression could go in. |
| VTT-665 succeeds VTT-592 | VTT-668 succeeds VTT-665 | Phase 4a found VTT-665 ambiguous. |
| QA handed VTT-665 to VTT-667 | QA's file also holds VTT-569, VTT-570 and VTT-599, and those rows carry its tests | QA derived tests from the specification sentences it was handed, which include Boot's ask (VTT-569, VTT-570) and the aliases (VTT-599). |
| Boot's mutants killed by a Task 2 test in `dm-view.test.ts`, asserted by what the question says (D5) | The test written there pinned the wording; removed, and QA's tests kill both mutants | The reading review found it against D5, and QA's tests hold VTT-667 by what the question says. |

## What could not be established

- Whether the whole of `task check` passes (item 5).
- Whether Stryker agrees. Item 4 waits for the owner's word; the hand
  mutations are the evidence until then.
- The timed-out mutants of the run at `37929bf`, in
  `client/src/view/canvas.ts`, `client/src/view/scene-plan.ts` and
  `client/src/wire.ts`: the checker counts them as killed and says the count
  measures the machine; not judged here.

## What was deliberately left out

- Making `renderPlayerPanel`'s `notify` required, which would remove the two
  `notify?.` mutants by changing an argument SPEC-022 describes as optional:
  the owner's to choose (the plan's D3).
- The full run (item 4), for the owner's word.
