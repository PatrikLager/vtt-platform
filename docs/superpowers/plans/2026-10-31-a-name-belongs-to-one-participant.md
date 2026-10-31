# Plan: a display name belongs to one participant at a time

- **Ticket:**
  `docs/superpowers/specs/2026-10-31-a-name-belongs-to-one-participant-design.md`.
- **Verdict:** passes, with gaps G1 to G8 (below).
  - Verified 2026-10-31 by an agent that did not write the ticket,
    read-only against HEAD `690ff7e`.
  - Probes ran in scratch copies of the tree outside the repository. P10's
    prototype is a scratch edit of `internal/identity/identity.go` and
    `internal/gateway/join.go`; nothing in the repository was edited but
    this file.
  - `task check`, `task e2e`, Stryker and gremlins were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-31-a-name-belongs-to-one-participant.md`.
- **Base:** main `690ff7e`.
- **Branch:** `fix/a-name-belongs-to-one-participant`.
- **Owner's rulings taken in:**
  - 2026-10-31, the share link: a join under a display name that a
    participant who is not revoked already holds is refused ("choose
    another"). Compared ignoring case, against every participant who is
    not revoked, DM and agent included. Checked only after the link is
    accepted. Rejected: an auto-suffix ("Kim 2"); allow and tag in the
    console; refuse only while the other is connected (MapTool's rule).
  - 2026-10-31, `vtt invite --name`: the same rule, whichever path a
    participant came in by. Duplicates already in a campaign stay; no
    migration.
  - 2026-10-21: a near-theoretical edge with a small consequence gets no
    code, no test, no debt entry, no specification sentence. Measure first.
  - 2026-10-30: the false comment above `joinSecretFrom` in
    `client/src/join.ts` is corrected in the next ticket that touches
    `join.ts` or the join flow.
  - 2026-10-26: the client's full mutation run is the owner's to ask for.
  - 2026-09-04: delete the old solution first; nothing is kept beside
    the new one.
- **Signed off:** 2026-10-31, "Ja, kör som ett ärende": Q1 answered one
  ticket on one branch.

## The verdict, check by check

1. **Every path resolves. By command.**
   - Every file the ticket names exists.
   - Every symbol stands where it says:
     - `handleJoin` and `usableDisplayName` in `internal/gateway/join.go`;
     - `JoinAdmits`, `CreateInvite` and `JoinBudget` in
       `internal/identity/identity.go`, where the `participants` table
       declares `display_name TEXT` with no constraint;
     - `newInviteCmd`'s `--name` in `cmd/vtt/invite.go`;
     - `renderDMConsole`, the "Who may do what" group, `person.name`,
       "Make player", "Make spectator" and `known` in
       `client/src/view/dm.ts`;
     - `p.displayName` in `client/src/view/spectator.ts`;
     - `requestJoin` in `client/src/join.ts`;
     - `vtt revoke --id` in `cmd/vtt/revoke.go`.
   - `internal/gateway/convert.go` writes `n.As = p.Name`, inside `voice`,
     for a line out of character. The ticket's `As = p.Name` is that line.
   - The MapTool symbols resolve at `f4b7fef6c` (P13).
   - The census: 144 `.CreateInvite(` calls in 43 test files (P5); 148
     lines match `CreateInvite(`, four of them `t.Fatalf` messages.
2. **"Done" is an observation.** The tests the items describe do not exist
   yet.
   - Item 1 fails today, by command: `kim` against `Kim`, `åsa` against
     `Åsa` and `dm` against a DM's `DM` are each admitted, 200, and a
     participant is minted (P1).
   - Item 3 fails today, by command: the taken name is admitted and spends
     one admission, 0 to 1 (P2).
   - Item 6 fails today, by command: `vtt invite --name kim` while `Kim`
     is held succeeds, prints a token and makes two participants (P3).
   - Item 2's client half holds today, by command (P4, G4).
   - Items 4 and 5 hold today, by command (P2). They are guards.
   - Items 7, 8 and 9 are guards and the gate. Item 8's test breaks under
     any rule inside `CreateInvite` (P6), so it changes (D10).
3. **Each rule is breakable. A reading.** Each has an observation that
   goes red; see "Requirements" and the breaks. The first rule is false as
   worded on the day it lands (G1), and the plan words it as two refusals.
   The second is strengthened (D12). The third is VTT-009 as worded (D13).
4. **The scope matches the claim. By command, then a reading.**
   - `CreateInvite` has three production callers: `handleJoin`,
     `newInviteCmd` and `mintInvites` in `cmd/vtt/harness_boot.go`. With
     the rule inside it, one test of 144 calls fails (P6).
   - No scenario file holds two names equal ignoring case (P7).
   - `JoinAdmits` has one production caller and 32 test call sites in four
     files. The rule must reach inside it, so all 32 change (G3).
   - `cmd/vtt/invite.go` needs no source change (D9). The client needs no
     behaviour change (G4).
   - Every Go file the work touches sits within 0.1 of its comment ceiling
     (P11, G6).
5. **No recorded decision is contradicted silently. A reading.**
   - SPEC-009's statements about the door hold. A door refusal still runs
     one `SELECT` and writes nothing; the new read comes after the door's
     decision (P10).
   - SPEC-009's "A join refused on the read takes no write lock", on which
     `POST /join` runs with no rate limiting, holds for the taken name: it
     is refused on two reads (D12).
   - The register is contradicted where the ticket does not say so: five
     rows become false as worded (G2).
   - The 2026-10-30 ruling on `join.ts` applies, and the ticket does not
     carry it (G5).
   - No gate changes (rule 2). `POST /join` is JSON outside the protobuf
     contract, so rule 3 is not reached.
6. **The records it moves. By command, then a reading.**
   - The section is present, and SPEC-009 resolves.
   - The reading: of `docs/specifications/`, SPEC-009 alone states how a
     join or an invite treats a name. SPEC-022 says the join form keeps
     `JoinViewState.name` across a repaint, and SPEC-019 that `vtt invite`
     takes no hold; neither moves.

## The gaps this plan inherits

- **G1. The first rule is false as worded.** "No two participants who are
  not revoked hold the same display name" is false in every campaign that
  already holds two, which the owner's ruling keeps. It is also false
  under a race that D6 leaves open. The register gets the rule as two
  refusals, one per path (C1, C2), which hold in both.
- **G2. Five rows become false as worded, and the ticket names none.**
  Each says a join is admitted, or two joiners are two participants, with
  no word about the name:
  - VTT-012, "... and an ordinary non-ASCII name is admitted": `åsa` is
    refused while `Åsa` is held, Done 1's own example.
  - VTT-019, "Rotating the link ... admits the new one".
  - VTT-024, "Two joiners through the same link are two participants
    with distinct credentials".
  - VTT-042, "A join carrying the current secret at an open door with
    budget remaining is admitted". Its test calls `JoinAdmits`, which now
    refuses exactly that join.
  - VTT-044, "... admits a holder of the new secret".

  D11 withdraws and succeeds all five. VTT-009, VTT-011, VTT-013, VTT-046,
  VTT-070 and VTT-072 stay true and gain evidence.
- **G3. Files the ticket does not name.**
  - The name check must sit between `JoinAdmits`'s read and its `UPDATE`
    (D2). Its 32 test call sites change: `identity_test.go` 18,
    `fault_internal_test.go` 6, `qa_joining_internal_test.go` 4 and
    `cmd/vtt/cli_test.go` 4 (P14).
  - `client/test/app.test.ts` gets item 2's test, and
    `client/test/join-url-format.test.ts` a pointer (D14).
  - `tools/comment-ceilings.txt` is lowered by `--write-ledger` (D16).
  - `client/e2e/joining.spec.ts` (D15), and `cmd/vtt/webdist` if its
    build changes.
  - The register's rows of G2 and their tests' citation lines, and three
    pointers to VTT-012 in `join.go` and one to VTT-044 in `identity.go`.
- **G4. Item 2's client half holds today.** `requestJoin` reads any 400's
  body as the name's refusal, and `runJoin` never empties `state.name`. A
  scratch boot answered 400 with a taken-name body shows the body without
  the prefix and keeps `Kim` in the box (P4). Item 2 can fail today only
  end to end, where the server admits the name. A1 pins the client half,
  J1's 400 the server half, and E1 joins them (D15).
- **G5. The 2026-10-30 ruling on `join.ts`.**
  - The block above `joinSecretFrom` ends with two claims SPEC-023 made
    false. The ruling puts its cut in the next ticket that touches the
    join flow; this is that ticket.
  - The file's head comment says a 400 "reveals nothing about the door".
    A taken-name 400 is given only when the door would admit, so it says
    the door is open to that caller.
  - Both go in Task 3 (D14). `client/test` is hashed into the client
    mutation report's inputs too (P12), so item 2's test alone makes the
    report stale. The comment edits add no cost.
- **G6. Every Go file touched is at its comment ceiling** (P11):
  `identity.go` 19.89 of 19.9, `join.go` 17.86 of 17.9, `identity_test.go`
  2.41 of 2.5, `fault_internal_test.go` 7.32 of 7.4, `join_test.go` 25.67
  of 25.7, `cli_test.go` 25.51 of 25.6. A new comment line goes in only
  with the code that keeps the file under. A test cites its rule on a
  citation-only line, which counts in no share.
- **G7. The ticket's unestablished points, answered.**
  - Two joiners racing under one name: measured, left open (D6, P9).
  - Unicode normalisation and surrounding whitespace: measured, no code
    (D5, P8).
  - Duplicates already in a campaign: they stay and are listed in a fixed
    order (VTT-072, D10).
  - A stale participant: unchanged. Someone who lost their credential and
    whose name is held chooses another name or asks the DM to revoke the
    old one. The console still shows no participant id; that is not this
    ticket.
  - The `CreateInvite` census: one test of 144 calls (P6).
- **G8. `vtt invite` neither trims nor runs `usableDisplayName`.** Not this
  ticket's rule, and no ruling asks for it. Nothing changes.

## Measurements this plan stands on

All on 2026-10-31 at `690ff7e`, in scratch copies outside the repository.

| # | Probe | Result |
|---|---|---|
| P1 | Scratch gateway test, today's tree: `Kim` (spectator), `Åsa`, `DM` (dm) held; open door; join `kim`, `åsa`, `dm` | each 200 with a credential; participants 1 to 2; spent 0 to 1 |
| P2 | Scratch gateway test, today's tree: `Kim` held | shut door, wrong secret, spent budget: the taken name and a free one get the same 403 and body; open door, `kim`: 200, spent 0 to 1; `Kim` revoked, then `kim`: 200 |
| P3 | Scratch `cmd/vtt` test, today's tree: `newInviteCmd` with `Kim`, then `kim` | no error, a token printed, two participants |
| P4 | Scratch `bun test` in `app.test.ts`, today's client: `?join=`, `Kim` typed, `/join` answers 400 `gateway: that name is already taken at this table; choose another\n` | `.error` reads `that name is already taken at this table; choose another`; box `Kim`; no token; Join enabled. `client/test/join.test.ts`: 11 pass |
| P5 | `grep -rn 'CreateInvite(' --include='*_test.go' internal cmd`; the same without the filter | 144 calls in 43 files (148 matching lines, four of them `t.Fatalf` messages): 34 in `internal/gateway`, 7 in `cmd/vtt`, 2 in `internal/identity`. Production: `handleJoin`, `newInviteCmd`, `mintInvites` |
| P6 | `CreateInvite` refusing a held name (`strings.EqualFold` over `revoked = 0`), then `go test` | `internal/identity`: one failure, `TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder`. `internal/gateway` ok (80.9 s); `cmd/vtt` ok (180 s); `internal/harness` and `internal/mcp` ok |
| P7 | The names of `scenarios/*.json` and `internal/harness/testdata/*.json`, compared with Python `casefold`; `SoakParticipants` | no file holds two equal names; the soak's are `dm`, `player1`, `player2`, `agent` |
| P8 | `modernc.org/sqlite` v1.54.0; Go `strings.EqualFold` | `lower('Åsa') = lower('åsa')` is 0; `'Åsa' = 'åsa' COLLATE NOCASE` is 0; `'Kim' = 'kim' COLLATE NOCASE` is 1. EqualFold: `Åsa`/`åsa`, `ΣΑΣ`/`σας`, `Ǆ`/`ǆ`, the Kelvin sign/`k` true; a composed `Å` against `A` and U+030A false (true after NFC); `ß`/`SS`, `Kim`/`Kim `, `İ`/`i` false |
| P9 | D2's order as autocommit statements on a temporary file: door read, names read, `UPDATE`, `INSERT` | from the names read to the insert's commit: min 0.50 ms, median 0.62 ms, p99 1.25 ms, max 3.29 ms (300 runs). Two joiners released together under one name: 200 of 200 pairs both admitted. Started 50 ms apart: 0 of 50 |
| P10 | A scratch prototype of D1 to D4 and D7, the 32 `JoinAdmits` calls given `"Newcomer"` | builds; `internal/identity`: only VTT-072's test fails; `internal/gateway` ok (80.9 s); `cmd/vtt` ok (177.6 s). With `Kim`, `Åsa` and `DM` held: shut door, wrong secret and spent budget each get the door's 403 and body; at an open door `kim`, `  KIM `, `dm`, `åsa` each get 400 and the taken body, spent 0 to 0, participants 3 to 3. In identity, a wrong secret under a taken name answers `(false, nil)` and an armed names read is not reached; a names read that fails answers `(false, error)` and spends nothing; a taken name answers `ErrNameTaken` |
| P11 | `measure` and `share_of` from `tools/check-comments.py`, against `tools/comment-ceilings.txt` | `identity.go` 104 of 523, 19.89 (19.9); `join.go` 15 of 84, 17.86 (17.9); `invite.go` 23.53 (23.6); `client/src/join.ts` 61 of 115, 53.04 (53.1); `identity_test.go` 2.41 (2.5); `fault_internal_test.go` 7.32 (7.4); `qa_joining_internal_test.go` 12.15 (12.2); `join_test.go` 25.67 (25.7); `cli_test.go` 25.51 (25.6). The prototype: `identity.go` 106 of 553, 19.17; `join.go` 15 of 90, 16.67, with one citation line |
| P12 | `report_state` from `tools/ts-mutation-report.py`; `TREES` in `tools/ts-mutation-inputs.py` | `current`, measured `526885b`. The inputs hash `client/src` and `client/test` |
| P13 | `~/dev/RPTool/maptool` at `f4b7fef6c` | see Rule 9 |
| P14 | `grep -rn '\.JoinAdmits(' --include='*_test.go' .` | 32: `identity_test.go` 18, `fault_internal_test.go` 6, `qa_joining_internal_test.go` 4, `cmd/vtt/cli_test.go` 4 |
| P15 | `git grep` for each G2 id under `internal`, `cmd`, `client` | VTT-012: `join.go` 3 comments, `join_test.go` 2 tests; VTT-019: `join_test.go` 1, `identity_test.go` 2; VTT-024: one test each in `join_test.go` and `identity_test.go`; VTT-042: `identity_test.go` 1; VTT-044: `identity.go` 1 comment, `identity_test.go` 1 |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone (D16). No exemption is widened: no file is added
    to `.go-arch-lint.yml`'s `excludeFiles`.
  - 3: the contract is untouched. `POST /join` is JSON outside it.
  - 4: no fold changes. Identity is not event-sourced (SPEC-009).
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `internal/`, `cmd/` or `client/src` is a warning or
    a pointer. History goes to the report.
- **The comment ceilings** (G6, P11). In `join.go` the new branch carries a
  citation-only line and no prose (P10 measured 16.67 with one). Every file
  is measured with `share_of` before its commit.
- **`go-arch-lint`.** `internal/identity` may depend on itself alone.
  A test that imports `internal/testdb` or `internal/campaign` goes in
  `identity_test.go`, `fault_internal_test.go` or
  `qa_joining_internal_test.go`, the three files already excluded.
- **Coverage floors:** `internal/identity` 92.1, `internal/gateway` 93.5,
  `cmd/vtt` 85.0; `client/src/join.ts` and `client/src/view/join.ts` 100.
  Each new branch has a test that reaches it, but for the reader's
  iteration-error arm, which I7 says `internal/testdb` cannot reach (I6 and
  I7 reach the names
  read's error arms).
- **The Go mutation gate runs in `task check`.** Each condition the work
  adds has a test that goes red when it is negated. A negated match refuses
  the free names the positive controls admit; a negated error arm is K8's
  or K10's.
- **Nobody starts Stryker** (SPEC-023, P12).
- **A negative assertion needs its positive.** "Refused" comes with a free
  name that is admitted, at the same door, in the same test.
- **A refusal's body is compared raw**, as `post` in `join_test.go` returns
  it, never decoded.
- **A new function or test goes after a closing brace**, never between a
  doc comment and its function (`check:doc-owner`).
- **A test the register cites has a literal name.**
- `bun test` is measured with `env -u CLAUDECODE`.
- **Before every commit:**
  - `python3 tools/check-new-prose.py main`: the 85-column band and its
    SHORT rule hold for test files too;
  - `python3 tools/check-requirements-chain.py .`;
  - `python3 tools/check-comments.py main`, read for added-line refusals;
  - for a client commit, `bunx tsc --noEmit -p client/tsconfig.json`.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo a break. Use the inverse edit, checked
  with grep.
- **The review gate refuses untracked files.** Park scratch files outside
  the tree with `cp` and `cmp`.
- **Hash the tree before and after any reviewer.**

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c` (P13):

- **A name already connected is refused, with its own message.**
  `ServerHandshake.handle(ClientInitMsg)` asks
  `server.isPlayerConnected(clientInitMsg.getPlayerName())` and answers
  `HandshakeResponseCodeMsg.PLAYER_ALREADY_CONNECTED`; the client shows
  `Handshake.msg.playerAlreadyConnected`, "That player name is already
  connected."
- **Ignoring case, letters beyond ASCII included.**
  `MapToolServer.getPlayer` compares each connected player's name with
  `String.equalsIgnoreCase`, which folds one character at a time, as
  `strings.EqualFold` does; the two differ on a few letters, `İ` against
  `i` among them, which Java folds together and Go does not. Nothing
  normalises.
- **The name is checked before the password.** The `isPlayerConnected`
  test is the first statement of `handle`; `playerDatabase.getPlayer` and
  the password come after. Whoever reaches the server learns whether a
  name is connected without knowing the password.
- **The client trims.** `ConnectToServerDialog` reads the username with
  `getText().trim()`.

Borrowed: a taken name refused with its own words (C5), and a compare that
folds case letter by letter, non-ASCII included (D4). Not borrowed:
- "Connected" as the scope, by the owner's ruling. Here identity is the
  participant id, and control survives disconnection (2026-09-06), so an
  absent participant's name is still on the console.
- The order. Here the door decides first (D2, VTT-009), so a stranger
  learns nothing about which names exist.
- Trimming is already `handleJoin`'s.

## Design questions for the owner

1. **Q1. One ticket and one branch, or split?** The ticket asks this at
   sign-off. Answered 2026-10-31: one.
   - Recommended: **one.** The client half is one test, two citations
     and two comment cuts, and it makes no behaviour change (G4). Split
     off, it would be a ticket whose tests pass on today's tree. The
     server half is what makes item 2 observable at all.
   - Alternative: the server half here, the client half and the browser
     beat as a ticket of their own.

Nothing else is the owner's. The rulings settle the design; D5 and D6
apply the ruling of 2026-10-21; D11 applies the register's own rule.

## Decisions

1. **D1. The rule lives in `internal/identity`, in one unexported reader
   that `JoinAdmits` and `CreateInvite` both call.**
   - Forced by item 3 with item 4. The check must come after the door's
     decision and before the spend, and both are inside `JoinAdmits`. A
     check in the gateway could sit there only if `JoinAdmits` were split
     into an exported read-only door and a spend: two door reads per join
     and a second door API.
   - Forced by "whichever path" (ruling 2). Every participant is minted
     through `CreateInvite`: by `handleJoin`, `newInviteCmd` and
     `mintInvites`. A check in the CLI would leave the harness, and any
     path written later, outside the rule.
   - Its cost is measured: one of 144 test calls breaks (P6), and the 32
     `JoinAdmits` call sites change mechanically (P10).
   - Rejected: a combined `Join(secret, name)` that mints. Every door test
     would mint, and the racing ones would need a name each. SPEC-009's
     split between deciding and minting, which many rows' tests stand on,
     would move. Its one gain, spend and insert in one transaction, is
     D6's, which the 2026-10-21 ruling rules out.
2. **D2. `JoinAdmits(candidate, name string) (bool, error)`.** The names are
   read after the door's refusal and before the `UPDATE`. A held name
   answers `(false, ErrNameTaken)`. A names read that fails answers
   `(false, error)`, which `handleJoin` refuses as the door's (VTT-046).
   - Forced by items 3 and 4, and by SPEC-009: a door refusal still runs
     one `SELECT` (I3), and a taken refusal writes nothing (I2).
   - "Delete the old solution first": there is no one-argument
     `JoinAdmits` beside it.
3. **D3. One exported sentinel, `identity.ErrNameTaken`**, returned by both
   producers. `vtt invite` reports it through its existing
   `fmt.Errorf("vtt invite: %w", err)`, and `handleJoin` finds it with
   `errors.Is`. It mirrors `ErrInvalidToken`. Its text is the
   implementer's.
4. **D4. The compare is Go's `strings.EqualFold`**, over
   `SELECT display_name FROM participants WHERE revoked = 0`.
   - Forced by item 1's `åsa`: SQLite's `lower()` and `NOCASE` fold ASCII
     alone (P8).
   - The stored name and the offered one are compared as they are. A
     table holds tens of rows; every one is read.
   - A `NULL` `display_name`, which only a raw write makes, fails the read
     and refuses, as `List` refuses a role that does not parse (I7).
5. **D5. No normalisation and no trimming inside the compare.**
   - What slips past `EqualFold` (P8):
     - a composed `Å` against `A` and a combining ring. A keyboard types
       the composed form; the other arrives by paste;
     - a name `vtt invite` stored with surrounding spaces against a
       joiner's. `handleJoin` already trims the joiner's, so only a DM who
       quotes a trailing space on the command line makes one;
     - `ß` against `SS`.
   - The consequence of each is two rows that look alike: today's state,
     which ruling 2 keeps for existing campaigns, ended by `vtt revoke`.
   - Near-theoretical, small consequence: no code and no specification
     sentence, by the ruling of 2026-10-21. NFC would also make
     `golang.org/x/text` a direct dependency.
6. **D6. The race of two same-name joiners is not closed.**
   - P9: the window from the names read to the insert's commit is about a
     millisecond (p99 1.25 ms). Two joiners released together both get in,
     every time; 50 ms apart, never.
   - At a table it takes two people typing one name, ignoring case, at one
     opening, and pressing Join within that millisecond. The consequence
     is G1's two alike rows.
   - Near-theoretical, small consequence: no transaction, no constraint,
     no test, no debt entry, no specification sentence (2026-10-21).
   - A unique index is ruled out on its own grounds:
     - it fails to build on a campaign that already holds two (ruling 2);
     - it needs a folded column SQLite cannot compute (P8), and a
       migration that takes the write lock once on every campaign;
     - `participants` is created `IF NOT EXISTS`, so a new column arrives
       only by `migrate`.
   - The register and SPEC-009 claim no impossibility (G1).
7. **D7. `handleJoin` answers `ErrNameTaken` with 400 and a body that says
   the name is taken.** The prototype's words were "gateway: that name is
   already taken at this table; choose another", "choose another" being
   the ruling's.
   - Forced by `requestJoin`. It reads any 400's body, without the prefix,
     as the name's refusal (P4). It maps any other failing status but 403
     to "Could not reach the table", so a 409 would need client code and
     would tell the joiner nothing until it was written.
   - Forced by VTT-013: the name's refusal is a 400, the door's a 403.
   - Who can learn from it: only a caller holding the current secret at
     an open door with budget left reaches it. Such a caller learns
     whether a name is held, the DM's, the agent's and an absent
     participant's included, which the presence strip would not show them
     (it draws only who is present, and `/api/participants` is the DM's
     and the agent's). They can ask as often as they like, since a
     refusal spends nothing (D12). The owner's ruling puts the check after
     the door, which bounds the asking to holders of the link, and no
     more than that is claimed.
   - The branch comes before the 403 branch and carries a citation line
     only (G6). The comment above the call, "refuse its error as a
     refusal", is reworded to say every other error.
8. **D8. A `CreateInvite` error after an admission stays the 500.** A
   racing joiner whose name `CreateInvite`'s own read catches gets "could
   not join" with the slot spent, as SPEC-009 already records for any
   `CreateInvite` failure. No branch is added for it (D6).
9. **D9. `cmd/vtt/invite.go` does not change.** The refusal arrives as
   `CreateInvite`'s error; `RunE` returns it before the token is printed,
   and `main` exits 1. P10's `cmd/vtt` ran green with the prototype.
10. **D10. VTT-072's test seeds a campaign from before the rule.**
    `TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder` inserts its four
    `Kim` rows by raw SQL, as `TestListingRefusesACorruptRowRatherThanInventingARole`
    writes its row. Their ids are fixed and inserted in an order other
    than sorted, so a lost tie-break cannot pass by the luck of random ids
    landing in sorted order, one run in 24 today. Its name stays, and its
    comment describes a campaign that held four before the rule. Forced by
    P6 and ruling 2.
11. **D11. Rows false as worded are withdrawn and succeeded** (G2).
    - The register never rewords a row: a false one is withdrawn,
      `WITHDRAWN <date>: <text>. <why>. Succeeded by …`, as VTT-183 was for
      one role.
    - Phase 4a reads rows literally. `åsa` against VTT-012 is Done 1's own
      example.
    - Each successor adds the name, and S19, S42 and S44 say "usable". That
      word also mends what VTT-012's own refusals already made false of
      VTT-019, VTT-042 and VTT-044, whose successors would otherwise
      inherit it.
    - The tests keep their names; their citation lines name the
      successors. So do `join.go`'s three VTT-012 pointers and
      `identity.go`'s VTT-044 pointer.
12. **D12. The ticket's second rule becomes "writes nothing".**
    - Writing nothing implies spending nothing, and it is what SPEC-009's
      consequence needs: `POST /join` runs with no rate limiting because a
      refusal on the read writes nothing, and "the day a refusal on the
      read writes, that decision is void".
    - I2 observes it with the QA's own probes: every write verb armed, and
      `PRAGMA data_version`. J2 observes item 3's words, `JoinBudget`.
13. **D13. The ticket's third rule is VTT-009 as worded.** "A shut door, a
    wrong secret and a spent budget are refused with the same status and
    the same body" holds whatever name the join carries. J3 and I3 join
    its evidence; no new row.
14. **D14. The client gets tests and comments, no behaviour.**
    - A1 in `app.test.ts` (item 2).
    - `join.test.ts`'s "a refused name is reported as the joiner's own,
      and without the wire prefix" cites C6, and "a refusal is shown, and
      the name the person already typed survives it" cites C7.
    - `client/src/join.ts` (G5, ruling 2026-10-30):
      - the block above `joinSecretFrom` keeps only why the function
        exists: one reader against three writers, pinned through
        `contract/testdata/join_url_format.json`;
      - its history moves to this ticket's report;
      - the head block becomes a warning with pointers: keep 400 the
        name's refusal and 403 the door's (SPEC-009, VTT-009, VTT-013).
        No sentence may remain that says a 400 reveals nothing about the
        door.
    - `join-url-format.test.ts`'s "See joinSecretFrom's own comment for
      the whole story" names this ticket's report instead.
15. **D15. One browser beat, E1, in `client/e2e/joining.spec.ts`.**
    - It is the only test where a real server's 400 reaches a real join
      form (G4). It costs one `task e2e` run.
    - A preference over leaving the seam to the two halves' tests. It
      catches a status change that both halves' stubs would miss.
16. **D16. `python3 tools/check-comments.py --write-ledger` runs once,
    after the last code commit,** and only if `check-comments.py main`
    finds a file more than 1.0 under its ceiling.
    - P10 put `join.go` 1.23 under. The test files and `client/src/join.ts`
      will fall too.
    - A lowered row cannot be raised again.
17. **D17. No Stryker run.** The client half makes the report stale (P12).
    At Task 7 the owner is told it measured `526885b`, with the tickets
    since, and that `task check:ts-mutation:full` is his to ask for.
18. **D18. Identity first.** The signature change compiles only with its
    callers. Task 1's commit carries `handleJoin`'s new argument and
    `cli_test.go`'s four calls.
    - Between Tasks 1 and 2, a taken name at the join gets the door's 403.
    - J1 is red on Task 1's commit and green on Task 2's.

## Requirements

Allocated at Task 0 with `requirement-id`, each `**OPEN — no test yet**`
until its tests land. The C, S, K and test labels are this plan's. The ids:
C1 VTT-542, C2 VTT-543, C3 VTT-544, C4 VTT-545, C5 VTT-546, C6 VTT-547,
C7 VTT-548, S12 VTT-549, S19 VTT-550, S24 VTT-551, S42 VTT-552, S44
VTT-553, S23 VTT-554, C8 VTT-555. S23 and C8, and the wording of C6 and
S24, came from Task 0's reading review, before any of them was committed:
S24 as first allocated was false of a third participant's name and of an
empty one, C6 of an empty body, VTT-023 was made false and not named, and
nothing said the join compares the trimmed name. C1 and C2 were allocated
saying "compared letter by letter ignoring case" for "compared ignoring the
case of any letter": `ß` against `SS` is a case difference that
`strings.EqualFold` does not fold (P8), and the row says what D4 compares.

- **C1.** "A join carrying the current secret at an open door with budget
  remaining, under a display name that a participant who is not revoked
  holds, compared letter by letter ignoring case, is refused." I1, J1,
  E1. Red under K1, K2, K9, and under K7 by I1.
- **C2.** "`vtt invite` naming a display name that a participant who is
  not revoked holds, compared letter by letter ignoring case, exits
  non-zero and creates no participant." I5, V1. Red under K1, K2, K6, K9.
- **C3.** "A display name held only by participants who are revoked is
  not refused for being held, through the share link or by `vtt invite`."
  I4, V2, J4. Red under K3.
- **C4.** "A join refused for a taken display name writes nothing." I2, J2.
  Red under K5, K7, and K12 by J2's `  KIM `.
- **C5.** "A join refused for a taken display name is answered 400, with a
  body that says the name is taken." J1, E1. Red under K7, K10, K11, K12.
- **C6.** "The join form shows the body of a 400 answer to a join as its
  error, trimmed and without a leading `gateway: `, or a fixed refusal of
  the name when nothing remains." Existing behaviour: `join.test.ts`'s "a
  refused name is reported as the joiner's own, and without the wire
  prefix"; A1, E1. Red under K14.
- **C7.** "A refused join leaves the name that was typed in the join form's
  box." Existing behaviour: `join.test.ts`'s "a refusal is shown, and the
  name the person already typed survives it" (the view); A1, E1 (the
  flow). Red under K13.
- **S12**, succeeding VTT-012: "A display name that is empty, longer than
  the bound, or carries a control, bidi or invisible-only content is
  refused, and an ordinary non-ASCII name that no participant who is not
  revoked holds is admitted." `TestADisplayNameIsBoundedAndPrintable`,
  `TestAnEmptyDisplayNameIsRefused`.
- **S19**, succeeding VTT-019: "Rotating the link refuses the old secret,
  and admits the new one under a usable display name that no participant
  who is not revoked holds." `TestRotatingTheLinkRefusesTheOldSecret`,
  `TestRotatingTheSecretInvalidatesTheOldLink`,
  `TestRotatingAfterASpentBudgetGivesAWorkingLink`.
- **S24**, succeeding VTT-024: "Two joiners through the same link, under
  usable display names that differ letter by letter ignoring case and that
  no participant who is not revoked holds, are two participants with
  distinct credentials." `TestTwoJoinersGetDistinctIdentities`,
  `TestTwoInvitesProduceDistinctTokensAndIDs`.
- **S23**, succeeding VTT-023: "Two joiners racing for the last admission,
  under usable display names that no participant who is not revoked holds:
  exactly one is admitted." `TestOnlyOneJoinerTakesTheLastSlot`.
- **C8.** "A join's display name is trimmed of surrounding white space
  before it is judged or stored." Existing for the storing; new for the
  name's check. J1's `  KIM ` (judged) and J5 (stored). Red under K12 and
  K15.
- **S42**, succeeding VTT-042: "A join carrying the current secret at an
  open door with budget remaining, under a usable display name that no
  participant who is not revoked holds, is admitted."
  `TestTheDoorNeedsBOTHTheFlagAndTheSecret`.
- **S44**, succeeding VTT-044: "At an open door, a link rotated after its
  budget is spent admits a holder of the new secret under a usable display
  name that no participant who is not revoked holds."
  `TestRotatingAfterASpentBudgetGivesAWorkingLink`.

Withdrawn at Task 0, in the register's form, with the evidence `**READING
— verify-ticket check 5 of 2026-10-31; withdrawn, held by nothing**`, each
"False as worded once a join under a display name that a participant who
is not revoked holds is refused", succeeded by its S row: VTT-012,
VTT-019, VTT-024, VTT-042, VTT-044. VTT-023 is withdrawn in the same words,
succeeded by S23, its evidence naming Task 0's reading review, which found
it: two joiners racing under a held name are both refused.

Kept, wording still true, evidence added:
- **VTT-009**: J3 and I3.
- **VTT-011**: J1 (no participant).
- **VTT-013**: J1 (400, a body that is not the door's).
- **VTT-046**: I6 (a names read that fails admits nobody).
- **VTT-070**: I6 (`CreateInvite` and `JoinAdmits` report a names read
  that fails).
- **VTT-072**: its test changes its fixture (D10) and keeps its name.

Read and kept as they stand: VTT-010, VTT-020 to VTT-022, VTT-066 and
VTT-075. None says anything a taken name makes false: a taken refusal
spends nothing, so a door still admits its whole budget.

The sort, and what it rejected:
- "No two unrevoked participants share a name" (rule 1 as worded): false
  on the day it lands (G1). C1 and C2 are its refusals.
- "Ignoring case, DM and agent included": inside C1 and C2's words, held
  by the `åsa` and `dm` cases.
- "Checked only after the link is accepted" (rule 3): VTT-009 (D13).
- "Prints no token": follows from C2. No participant, no token.
- "Duplicates already in a campaign stay": the absence of a migration is
  not observable. VTT-072 holds that they are listed in a fixed order.
- The race, normalisation, whitespace: no rows (D5, D6).
- "The name read comes after the door's": how. I3 holds VTT-009 by it.
- The taken body's words: how (D7). C5 holds that they say "taken".

| Ticket item | Observed by |
|---|---|
| 1 | I1, J1, E1 |
| 2 | A1, E1; `join.test.ts`'s two tests |
| 3 | I2, J2 |
| 4 | I3, J3 |
| 5 | I4, V2, J4 |
| 6 | I5, V1 |
| 7 | the three files' existing tests, run whole |
| 8 | `TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder` (D10) |
| 9 | Task 7 |

## The existing tests that change

- **The 32 `JoinAdmits` calls** in `identity_test.go`,
  `fault_internal_test.go`, `qa_joining_internal_test.go` and
  `cmd/vtt/cli_test.go` pass a name no participant in their test holds.
  P10 passed `"Newcomer"` everywhere and every test stayed green.
- **`TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder`**: D10.
- **Citation lines**:
  - VTT-012's two tests name S12;
  - VTT-019's three name S19;
  - VTT-024's two name S24;
  - `TestTheDoorNeedsBOTHTheFlagAndTheSecret` names S42;
  - `TestRotatingAfterASpentBudgetGivesAWorkingLink` names S44 and S19;
  - `TestOnlyOneJoinerTakesTheLastSlot` names S23;
  - `join.test.ts`'s two tests gain C6 and C7.
- **`client/test/join-url-format.test.ts`**: one pointer (D14).
- No other existing test changes. P6 and P10 ran every package that mints.
- `tools/check-comments.py`'s usage text, `tools/check_comments_qa_test.py`
  and `tools/check_comments_test.py` use `VTT-042` as an example of a
  citation line. The chain scans `tools`, and the withdrawn row still
  resolves it. They do not change.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off, with Q1's answer.
  - Allocate C1 to C8 and S12, S19, S23, S24, S42, S44 with
    `requirement-id`.
  - Withdraw VTT-012, VTT-019, VTT-023, VTT-024, VTT-042 and VTT-044 in the
    register's form.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean; the rows exist; the six read
  `WITHDRAWN` and name their successors; the chain passes.

### Task 1: identity, and the invite (items 3, 5, 6, 8; C1 to C4; D1 to D6, D9, D10, D18)

- **Files:**
  - `internal/identity/identity.go`: `ErrNameTaken` and its doc line; the
    unexported names reader; `JoinAdmits`'s parameter and its call to the
    reader; `CreateInvite`'s call; `JoinAdmits`'s and `CreateInvite`'s doc
    comments; the VTT-044 pointer in `RotateJoinSecret` names S44, and the
    VTT-023 pointer in `JoinAdmits` names S23;
  - `internal/gateway/join.go`: `JoinAdmits(req.Secret, name)`, and
    nothing else;
  - `internal/identity/identity_test.go`, `fault_internal_test.go`,
    `qa_joining_internal_test.go`, `cmd/vtt/cli_test.go`;
  - `docs/requirements.md`: the evidence of C1 to C4's identity and invite
    halves, S19, S23, S24, S42, S44, VTT-009, VTT-046, VTT-070.
- **New tests**, each with a literal name and its id line:
  - **I1** (C1), `identity_test.go`. `Kim` (spectator), `Åsa` (player),
    `DM` (dm) and `Agent` (agent) held, an open door with budget.
    `JoinAdmits(secret, …)` for `kim`, `åsa`, `dm`, `AGENT` each answers
    `(false, ErrNameTaken)`; `Kimberly` answers `(true, nil)`.
  - **I2** (C4), `fault_internal_test.go`, with `qaFaultDB`, `qaWatch` and
    `refuseUnderEachArm`. `Kim` held, an open door: `JoinAdmits(secret,
    "kim")` under each write verb armed reaches none, and answers
    `ErrNameTaken`. `PRAGMA data_version` does not move, and `JoinBudget`
    reads as before.
  - **I3** (VTT-009), `fault_internal_test.go`. `Kim` held. A wrong secret,
    a shut door and a spent budget, each with the names read armed:
    `JoinAdmits(…, "kim")` answers `(false, nil)` and the arm is not
    reached. The control: at an open door with the current secret, the
    same arm is reached. Arm the reader's whole statement: `testdb`
    matches a substring, and `Lookup`'s statement also begins
    `SELECT display_name`.
  - **I4** (C3), `identity_test.go`. `Kim` invited, then revoked:
    `JoinAdmits(secret, "kim")` answers `(true, nil)`, and
    `CreateInvite("KIM", …)` succeeds.
  - **I5** (C2), `identity_test.go`. `Kim`, `Åsa` and `DM` held:
    `CreateInvite` of `kim`, `åsa` and `dm`, at any role, answers
    `ErrNameTaken` and `List` keeps its length; `Kimberly` is minted.
  - **I6** (VTT-046, VTT-070), `fault_internal_test.go`. The names read
    armed to fail. `JoinAdmits(secret, "Zed")` at an open door answers
    `(false, err)`, `err` not `ErrNameTaken`, and `JoinBudget` reads as
    before. `CreateInvite("Zed", …)` answers an error, no token, and no
    row. Each arm is reached.
  - **I7** (no row: it reaches an error arm), `identity_test.go`. A
    `display_name` set to `NULL` by
    raw SQL: `JoinAdmits` and `CreateInvite` answer an error that is not
    `ErrNameTaken`. The reader's iteration error arm is not reachable
    through `internal/testdb`, which arms statements; it stays uncovered.
  - **V1** (C2), `cmd/vtt/cli_test.go`, through `runCLI`. `invite --name
    Kim`, then `--name kim --role dm`: an error, `errors.Is` it
    `ErrNameTaken`, output without the token line, `List` of length 1. The
    same for `Åsa` and `åsa`.
  - **V2** (C3), `cmd/vtt/cli_test.go`. `invite Kim`, `revoke --id`, `invite
    kim`: succeeds.
- **Order:**
  1. Give `JoinAdmits` its `name` parameter, unused. Update the 32 calls and
     `handleJoin`'s. Everything is green.
  2. Write I1 to I7, V1, V2 and D10's fixture. I1, I2, I3's control, I5,
     I6, I7 and V1 are red; I4 and V2 are green, guards until K3.
  3. Then the code. Then the breaks.
- **True when:**
  - `go test ./internal/identity/ ./cmd/vtt/ ./internal/gateway/` passes,
    J1 aside, which does not exist yet;
  - `share_of` holds `identity.go` and each test file at or under its
    ceiling, and `check-comments.py main` finds no added-line refusal;
  - `go vet` and `go-arch-lint` are clean.
- **Breaks**, each undone by its inverse edit:
  - **K1:** `strings.EqualFold(held, name)` becomes `held == name`. I1, I5
    and V1 red.
  - **K2:** the compare moves into SQL, `display_name = ? COLLATE NOCASE`.
    The `åsa` halves of I1, I5 and V1 red, and only they.
  - **K3:** the read loses `WHERE revoked = 0`. I4 and V2 red.
  - **K4:** `JoinAdmits` calls the reader above its door refusal. I3 red.
  - **K5:** `JoinAdmits` calls the reader after the `UPDATE`. I2 red.
  - **K6:** `CreateInvite`'s call removed. I5 and V1 red; I1 green.
  - **K7:** `JoinAdmits`'s call removed. I1 and I2 red.
  - **K8:** the reader's query error answered as "not held". I6 red.
  - **K9:** the read gains `AND role = 'spectator'`. The `dm` and `AGENT`
    halves of I1 and I5 red.

### Task 2: the join's answer (items 1, 3, 4, 5; C1, C3, C4, C5; D7, D8)

- **Files:**
  - `internal/gateway/join.go`: the `ErrNameTaken` branch, 400, before the
    403; the comment above the call reworded; the three VTT-012 pointers
    name S12;
  - `internal/gateway/join_test.go`;
  - `docs/requirements.md`: the gateway halves of C1, C3, C4, C5; C8; S12,
    S19, S24; VTT-009, VTT-011, VTT-013.
- **New tests**, in `join_test.go`, with `newJoinFixture` and `post`:
  - **J1** (C1, C5, C8, VTT-011, VTT-013). `Kim`, `Åsa` (player) and `DM`
    (dm) held, an open door with budget. `kim`, `  KIM `, `åsa` and `dm` are
    each answered 400, a raw body containing "taken" and unlike a wrong
    secret's, and the participant count does not move. `Kimberly` is
    answered 200.
  - **J2** (C4). A budget of 1, `Kim` held. `kim` and `  KIM ` are each
    answered 400, and `JoinBudget` reads 0 before and after. `Ann` is then
    answered 200, and `Bo` 403.
  - **J3** (VTT-009). `Kim` held. `kim` at a shut door with the current
    secret, `kim` with a wrong secret at an open door, and `kim` with the
    current secret against a spent budget each get the status and raw body
    of `Zed` with a wrong secret.
  - **J4** (C3). `Kim` revoked: `kim` is answered 200.
  - **J5** (C8). `  Zed  ` at an open door is answered 200 with `name`
    `Zed`, and `List` names the new participant `Zed`.
- **Order:** write J1 to J5 on Task 1's commit. J1 and J2 are red:
  the taken name gets the door's 403 there (D18). Then the code, then the
  breaks.
- **True when:** `go test ./internal/gateway/` passes;
  `join.go`'s added comment lines are citation-only or within its ceiling
  by `share_of`; the gateway tests run whole.
- **Breaks:**
  - **K10:** the `ErrNameTaken` branch removed. J1 and J2 red.
  - **K11:** the branch answers with `usableDisplayName`'s body. J1 red.
  - **K12:** `JoinAdmits` is passed `req.DisplayName`, untrimmed. J1's
    `  KIM ` red, and J2's: `CreateInvite` refuses it after the spend.
  - **K15:** `CreateInvite` is passed `req.DisplayName`, untrimmed. J5
    red.
  - K1, K2 and K7 again: J1 goes red with I1.

### Task 3: what the joiner reads (item 2; C6, C7; D14)

- **Files:** `client/test/app.test.ts`, `client/test/join.test.ts`
  (citations), `client/test/join-url-format.test.ts` (the pointer),
  `client/src/join.ts` (comments only); the comment in the `known` loop of
  `client/src/view/dm.ts` and its twin in `client/test/dm-view.test.ts`,
  "identity.CreateInvite does not validate the name", which is half true
  once `CreateInvite` refuses a held name: each says it does not refuse an
  empty one; `docs/requirements.md`: C6, C7.
- **New test:**
  - **A1** (C6, C7), `app.test.ts`, beside "a refused join leaves the form
    up, says why, and stores nothing". `?join=s3cret`; `/join` answers 400
    with J1's body, prefix and newline included; `Kim` typed, Join
    pressed. Then:
    - `.error` reads the body without `gateway: ` and without the newline,
      and is not the door's text;
    - the box holds `Kim`;
    - no token is stored, no socket dialled, and Join is enabled.
- **Order:**
  1. A1 passes on Task 2's commit (G4).
  2. Run K13 and K14 to show it can fail.
  3. Then the comments.
- **True when:**
  - `app.test.ts`, `join.test.ts`, `join-url-format.test.ts`,
    `qa-repaint-focus.test.ts` and `focus.test.ts` pass, run once in one
    process;
  - the typecheck is green, and coverage of `client/src/join.ts` is 100;
  - `git grep -n "reveals nothing about the door\|only thing that runs it unasked\|skips itself on" -- client`
    prints nothing;
  - `task build:client` has run, and `cmd/vtt/webdist` is committed if it
    changed.
- **Breaks:**
  - **K13:** `runJoin` empties `state.name` on a refusal. A1 red.
  - **K14:** `requestJoin` answers a 400 with the door's reason and text.
    A1 red, and "a refused name is reported as the joiner's own, and
    without the wire prefix".

### Task 4: the browser (items 1, 2; D15)

- **Files:** `client/e2e/joining.spec.ts`.
- **Test:**
  - **E1**, a beat after beat 2, once the guest is connected as `Robin`. A
    new browser context opens the current link and types `robin`; Join is
    pressed. Then:
    - `.error` contains "taken" and the box holds `robin`;
    - the context shows no `.conn`, and the DM's roster still holds one
      row reading `Robin`.

    A screenshot. Cites C1, C5, C6 and C7.
- **Order:** `task build:client`; check the machine's load; `task e2e`
  whole.
- **True when:** every spec passes; E1's screenshot is in
  `client/e2e/.artifacts/`.

### Task 5: the report

- **Do:** write `docs/reports/2026-10-31-a-name-belongs-to-one-participant.md`
  and give it its reading review.
- **It covers:**
  - each "Done looks like" item with its observation;
  - K1 to K14 with what went red;
  - Rule 9, G1 to G8 and Q1's answer;
  - the history cut from `client/src/join.ts`;
  - P9's measurement, said once;
  - Phase 4a's adjudications under their own heading.

  Leave room for the gate's figures.
- **True when:** it answers the ticket's nine items in its numbering, each
  `[x]` naming its observation.

### Task 6: the specification

With the `specification` skill, from the report, SPEC-009:
- **Status:** the test files that pin the new rows, `cmd/vtt/cli_test.go`
  and Phase 4a's among them.
- **"One shared link admits people":**
  - `handleJoin` asks `JoinAdmits` with the secret and the trimmed name;
  - a taken name gets its own 400, after the door;
  - `CreateInvite` refuses a name that a participant who is not revoked
    holds, compared with `strings.EqualFold`, before it inserts, so
    `vtt invite` and `mintInvites` are held to it.
- **"Every refusal the one read can decide":** the door's refusals still
  run one `SELECT`. A candidate the door would admit then has the names of
  every participant who is not revoked read. A held name is refused with
  `ErrNameTaken` before the `UPDATE`, so it writes nothing. Only a caller
  holding the current secret at an open door with budget left reaches that
  read.
- **"Revoked participants are not listed":** a revoked participant's name
  is free; a campaign holding two participants of one name from before the
  rule keeps them, and `List` orders them by id.
- **Consequences:** a taken name is refused distinctly, after the door, and
  writes nothing.
- **Requirements:** the six withdrawn ids out, C1 to C8 and the six
  successors in.

No sentence on the race, normalisation or whitespace (D5, D6), and none
claiming two participants never share a name.

**True when:** each changed sentence has been run against the code by
someone other than its writer;
`git grep -n "VTT-012\|VTT-019\|VTT-023\|VTT-024\|VTT-042\|VTT-044" docs/specifications`
prints nothing; and every id range in the Status paragraph is read, since
a range broken across a line hides an id from that grep.

### Task 7: the ledger, the gate, whole, and the commits

- **Do:**
  - `git grep -n "VTT-012\|VTT-019\|VTT-023\|VTT-024\|VTT-042\|VTT-044" -- internal cmd client`
    prints nothing.
  - Run `python3 tools/check-comments.py main`. If it finds a file more
    than 1.0 under its ceiling, run `--write-ledger` once and commit the
    ledger (D16).
  - Check free space and the machine's load.
  - Run `task check` whole on the last code commit. Launch it with
    `start_new_session=True`, send all output to a file, and require the
    gate's own completion line and `check:ts-mutation`'s own line.
  - Write the figures into the report; commit the report, then the
    specification.
  - Tell the owner D17's reminder.
- **True when:**
  - the gate is green, `check:comments`, `check:new-prose`,
    `check:requirements-chain`, `check:coverage` and `check:mutation`
    included;
  - `check:ts-mutation` printed that the client's mutation was not
    measured, naming `526885b` and this ticket;
  - the log holds no Stryker line.
- Push and merge on the owner's word.

## For Phase 4a

Hand the QA agent:
- the allocated rules' texts, C1 to C8 and the six successors, with
  VTT-009, VTT-011, VTT-013, VTT-046, VTT-070 and VTT-072;
- SPEC-009's changed paragraphs;
- the surfaces, as signatures and names, never the source:
  - `go doc ./internal/identity`: `Open`, `JoinSecret`, `SetJoinOpen`,
    `JoinAdmits(candidate, name string) (bool, error)`, `JoinBudget`,
    `RotateJoinSecret`, `CreateInvite`, `Revoke`, `List`, `Verify`,
    `ErrNameTaken`, `ErrInvalidToken`;
  - `gateway.New(c, ids).Handler()` and `POST /join`: a JSON body
    `{secret, displayName}`; 200 with `{token, participantId, name, role}`;
    400 for a name; 403 for the door;
  - `vtt invite --campaign --name --role`, its two output lines, and
    `runCLI` in `cmd/vtt/cli_test.go`;
  - `requestJoin`, `JoinOutcome`, `renderJoinView`, `JoinViewState` and
    `boot`;
  - `internal/testdb`'s `Arm`, and `qaFaultDB`, `qaWatch` and
    `refuseUnderEachArm` in `internal/identity`.

Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- `go-arch-lint`: an identity test that imports `internal/testdb` or
  `internal/campaign` goes in `qa_joining_internal_test.go`; any other new
  identity test file imports only `internal/identity`, the standard
  library and `modernc.org/sqlite`;
- a new Go test file is held to the default comment ceiling, 25.0;
  `qa_joining_internal_test.go` sits at its own (12.15 of 12.2), so what
  is appended there cites its ids on citation-only lines and adds no
  prose comment;
- a refusal's body is compared raw; "refused" comes with a free name
  admitted at the same door;
- the race of two same-name joiners is known and left open by the owner's
  rule on near-theoretical cases (D6). A test of it is not a finding;
- the join trims the name it is sent before anything is judged (C8);
  `vtt invite` does not trim. Apart from that trim, a name that differs
  only in composition (a composed `Å` against `A` and a combining ring),
  or by `ß` against `SS`, or by `İ` against `i`, is a different name
  (D5, P8);
- give each test a literal name and its rule's id;
- its files: `internal/identity/qa_joining_internal_test.go` (appended),
  `internal/gateway/qa_names_test.go`, `cmd/vtt/qa_invite_names_test.go`,
  `client/test/qa-join-names.test.ts`;
- measure with `env -u CLAUDECODE bun test <file>`;
- the tree is hashed before and after it.
