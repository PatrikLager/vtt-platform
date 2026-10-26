# A player's line speaks as the character they play, and /ooc as the player: the change

**Ticket:** `docs/superpowers/specs/2026-10-26-a-player-speaks-as-their-character-design.md`.
An agent that did not write it verified it and passed it with gaps G1 to G8.
The writer then narrowed its first rule to a named party member, added the
"unless it is out of character" to its fifth, and revised item 3 to a
party member and item 6 to the DM's Events panel.
**Plan:** `docs/superpowers/plans/2026-10-26-a-player-speaks-as-their-character.md`,
written by that verifier from probes outside the tree, and amended by the
owner's answers to its two questions and by the ids Task 0 allocated.
**Where it came from:** the owner's question of 2026-10-26, on the Say
panel's "in character as" box that Send empties: "Rutan tillhör ju
karaktären? Varför skriva namnet? Har du kollat rptool?". The ticket of
`docs/reports/2026-10-25-a-repaint-keeps-what-is-being-typed.md` kept the
box as it was; its rule 9 section read MapTool's text area and not its
speaker.
**The owner's rulings, all 2026-10-26:**
- A player's narration speaks as the character they play; the free-text box
  goes for a player.
- "Ge möjlighet till out of character speech. Men det är alltså ett kommando
  /ooc."
- "Ja för DM": the DM keeps a free speaker name.
- On four questions: "Ja, som DM", the agent too; "Namn: (( text ))", an
  out-of-character line shows the name the player joined with, written by
  the server; "Alltid som karaktären", a player's line without `/ooc` always
  speaks as the character; "Ett ärende, servern först".
- On the plan's questions: Q1 "Ja", the DM and the agent may speak out of
  character too; Q2 "Nej, de använder as", the server refuses an actor id
  from them.
- The plan signed off: "Ja, kör".

**Last code commit:** `db81c37`, on `e33376f`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `01fe6ff` | Task 0: the ticket, the plan, VTT-495 to VTT-504, VTT-260 withdrawn |
| `f40af43` | Task 1: `AddNarration.actor_id` and `out_of_character`, `NarrationAdded.out_of_character`, regenerated, the fixtures and round trips, QA's tests, VTT-505 |
| `466cfbe` | Task 2: `mayVoice` in `Authorize`, `voice` after `ToEvent`, the tool's docs, story-table, the tests, QA's `internal/gateway/qa_speaking_test.go`, VTT-260's citations moved to VTT-503, VTT-506 |
| `b1944ce` | Task 3: `readLine`, `speakAs`, `speakOutOfCharacter`, the panel's Say form by role, the Story's and the Events panel's line out of character, the tests, QA's `client/test/qa-speaking.test.ts`, the bundle |
| `db81c37` | Task 4: `client/e2e/speaking.spec.ts`, and `asDM` shared in `client/e2e/as-dm.ts` |

## What the code did on `e33376f`

- `renderPlayerPanel` in `client/src/view/player.ts` drew two Say boxes, a
  free-text "in character as… (optional)" and "say or narrate", for every role
  that drew the panel. `submit` sent `addNarration(text, as || undefined)`,
  then emptied both.
- `playerRules`' entry for `add_narration` in `internal/gateway/authz.go` was
  `unrestricted`. `ToEvent` in `internal/gateway/convert.go` copied the
  command's `as` into `NarrationAdded.as`, and the fold refused only an `as`
  over 256 bytes. So a player's line could speak as anyone: another player's
  character, a creature, or "DM".
- A line with an empty `as` was drawn by `renderFeed` in
  `client/src/view/spectator.ts` as `narration`, the shape of the DM's.
  `actor_role` and `participant_id` reach no player or spectator (SPEC-016),
  so no window could tell the two apart.
- Nothing in `client/src` read `/ooc` or a leading `/`: `git grep -nE
  '/ooc|startsWith\("/"\)|\^\\/' e33376f -- client/src` prints nothing. A
  line typed as `/ooc …` was sent as that text.
- The contract had no field to say a line is out of character, or which actor
  it speaks as.

## Done looks like, answered

1. [x] `client/test/spectator-view.test.ts`'s "a player's Send speaks as the
   party member the panel acts as, and the panel draws no speaker box": a
   player plays `a2` of two party members. `I draw my sword` is sent with
   `actorId` `a2` and no `as`. No `[data-field="say-as"]` is in the panel.
   Red on `466cfbe`, and under K10 and K11.
2. [x] spectator-view's "a player's /ooc line goes out of character with the
   rest of the line": `/ooc where is the pizza` is sent out of character, with
   text `where is the pizza` and no `actorId`. Red on `466cfbe`, and under K13.
3. [x] `internal/gateway/narration_test.go`, through `Authorize`:
   - "TestAPlayerSpeaksAsTheirOwnPartyMemberAndNoOtherActor": refused as
     another player's party member, accepted as their own;
   - "TestAPlayersLineThatSpeaksAsNoPartyMemberOfTheirsIsRefused": refused as
     their own creature, with the refusal an absent id gets.

   Both red on `f40af43`; the first under K1, the second under K1 and K2.
4. [x] That same second test is refused with `as` `DM` and with `as` empty,
   and no actor named. Red on `f40af43`, and under K1.
5. [x] narration_test.go's "TestAPlayersLineIsAppendedUnderTheirPartyMembersName",
   over a live server: Lera speaks as `a1` with `as` `Someone Else`, and the
   agent's connection is sent `as` `Hero`, in character. QA's
   "TestQASpkInCharacterLineIsAppendedUnderTheActorsName" sends four other
   `as` values. The first was red on `f40af43`, and under K4.
6. [x] The appended line:
   - narration_test.go's "TestALineOutOfCharacterIsAppendedUnderItsSendersName":
     out of character with `as` `Hero`, the line is appended marked, under
     "Lera" for the player, "DM" and "Agent";
   - QA's "TestQASpkOutOfCharacterLineNamesItsSender": the same for a player
     who joined as `Ada`;
   - QA's "TestQASpkAPlayersConnectionIsSentTheMarkAndName": another player's
     connection is sent the mark and "Ada".

   The drawing: spectator-view's "a line out of character reads as its
   speaker and its text in double parentheses, in the Story" reads `Ada: ((
   where is the pizza ))`, and "a line out of character reads the same in the
   Events panel" reads `Ada: (( brb ))`, the ticket's shape with another
   text. The Events panel is the DM's and the
   agent's alone (VTT-445). narration_test.go's test was red on `f40af43`,
   and the drawing tests on `466cfbe`; K5, K6 and K19 red the first, and
   K15, K16, K21 and K22 the drawing.
7. [x] narration_test.go's "TestTheDMsAndTheAgentsLineKeepsTheAsTheyGave":
   the DM's and the agent's lines with `as` `Goblin chief` and with `as` empty
   are appended with that `as`. Green on `f40af43`, as the item says it holds
   today; red under K7.
8. [x] `client/e2e/speaking.spec.ts`'s "another player's Story names a line by
   its speaker, in character and out of it":
   - Lera plays `act-fighter`, and Watcher is promoted and given Wren.
   - Lera sends `I draw my sword`, and Watcher's Story reads it as speech:
     `Human Fighter: I draw my sword`.
   - Lera sends `/ooc brb`, and Watcher's Story reads it out of character:
     `Lera: (( brb ))`.
   - Lera's page draws no speaker box, and its narrations carry no `as`, so
     the names are the server's.

   On `f40af43` it reds at Lera's speaker box. With that one assertion removed
   in the copy, it reds at Watcher's Story: "Received: I draw my sword". It is
   red under K11, K15 and K24 built into a copy's bundle.

## What the rules became

- "A player's narration either speaks as a named party member that player
  controls or is out of character; the server refuses any other":
  - VTT-495, held by item 3's and 4's tests, "TestAPlayerCannotSpeakAsAPartyMemberWithNoName"
    and "TestALineOutOfCharacterSpeaksAsNoActor";
  - and VTT-506, the one text for an absent, another's or a creature's id,
    held by the second of item 3's tests and QA's
    "TestQASpkNamedRefusalsShareOneText".
- "Every viewer sees a player's in-character line under the name the world
  holds for the actor it speaks as": VTT-496, item 5's tests and W1. That the
  line reaches every active character log is VTT-362's, and the sort gave
  this rule no id of its own.
- "Every viewer sees a player's out-of-character line marked as such, under
  the name the player joined with, which the server sets": VTT-497, item 6's
  appending tests and W1.
- "The player panel's line speaks as the character the player plays, and a
  line that opens with `/ooc` is out of character instead": two rows.
  - VTT-500, the panel and its speaker: items 1 and 8, and
    "a player's switch of party member keeps the Say text, and Send speaks as
    the new one".
  - VTT-501, `/ooc`: item 2, client/test/player.test.ts's tests of `readLine`,
    and W1.
- "The DM's and the agent's narration keeps the speaker they give it, or
  none, unless it is out of character":
  - VTT-498, item 7;
  - VTT-499, that they name no actor by id: "TestTheDMAndTheAgentVoiceAnyoneByAsAndNoActorByID";
  - VTT-504, that their panel keeps the speaker box: "an in-character speaker
    is carried, and a blank one is omitted".
- Rows the ticket's rules do not name:
  - VTT-502, the plan's C7, the drawing: item 6's drawing tests and W1;
  - VTT-503, the plan's C8, VTT-260's successor: the event carries every
    field of the command it also has, but the `as` the server writes;
  - VTT-505, allocated in Task 1 when Phase 4a cited the plan's D1, that a
    `NarrationAdded` carries no actor id, held by
    `contract/roundtrip_test.go`'s "TestQANarrEventHasNoActorId" and its TS
    twin.
- Withdrawn: VTT-260, "The event a one-envelope command becomes carries
  every field the command gave": false once the server writes `as`.

The sorts refused, among others:
- "a player cannot send a nameless line", which follows from VTT-495 and
  VTT-496;
- `/ooc`'s case and word boundary, which are inside VTT-501;
- an empty `/ooc` sending nothing, which joins the older empty-line rule,
  which has no row;
- the wording of the nameless refusal;
- that every refusal wraps `ErrUnauthorized`, which is `Authorize`'s own
  contract;
- the class name `ooc`;
- the contract's field numbers and names, which the round trips hold;
- an old line reading as not out of character, which is proto3's zero
  value under ADR-007's additive rule;
- Enter sending, which is older and has no row;
- a display name over 256 bytes, which is near-theoretical (below).

## Phase 4a

One QA agent per task that delivered behaviour. Each was blind to the
implementation, the existing tests and the diff. Each was given its task's
decisions from the plan, the rows its task delivers (Task 1 had none yet),
the ticket read whole, the specifications its prompt named, and the
exported surface as signatures. Task 0 had no behaviour to derive,
and Task 4 adds a test and no behaviour.

| Task | Tests, as committed | Failures |
|---|---|---|
| 1 | 8 Go at the end of `contract/roundtrip_test.go`, 7 TS in `contract/qa-narration-fields.test.ts` | 0 |
| 2 | 8 Go in `internal/gateway/qa_speaking_test.go` | 0 |
| 3 | 36 TS in `client/test/qa-speaking.test.ts` | 0 |

### QA adjudications

- Task 1, adjusted: two asserted field counts and one exact field list, which
  a later additive field would red, became the invariant. No field of
  `NarrationAdded` may hold "actor" in its name.
- Task 1, a requirement it cited with no id: "The event carries no actor id"
  became VTT-505.
- Task 2, QA finding 1: spec ambiguity. The plan's D2 says the refusal of a
  nameless party member tells the player to "use `/ooc`". The server says
  "speak out of character". Ruling: the server's text stands. `/ooc` is the
  browser's word, and an MCP client sits at the same table. No change.
- Task 2, a requirement it cited with no id: D2's one text became VTT-506.
- Task 3: no defect. The gaps it named:
  - `/OOC` rests on the plan's preference for the exact word;
  - no sentence says how a line out of character with an empty `as` is
    drawn, and the server always writes one;
  - leading spaces before `/ooc` are trimmed first.

After Phase 4a the QA files changed, by the session's adjudication or by a
review's finding:
- Task 1's Go tests moved into `roundtrip_test.go`; their field counts
  became the invariant; the old-message test reads the committed
  `narration_added_envelope.json` instead of a message built from the new
  type; the omitted-zero test checks `Marshal`'s error.
- Task 2's comments, which pointed at the plan and the ticket, cite
  requirement ids, since rule 10 binds `internal/`; a failure message no
  longer names `/ooc`; helper comments that described behaviour are gone.
- Task 3's file gained a header naming the plan and the ticket its labels
  belong to, and an `afterEach` that empties the document.

Task 2's QA prompt named `go-arch-lint check` and the packages a test may
import. Task 1's did not, and its file failed `check:arch` (Deviations).

## Phase 4b

One reviewer per task, by command, with probes in scratch copies: hand
mutants in Tasks 1 to 3, Chromium in Task 4. Every Must and every Should
was taken. Four Nits were not:
- Task 0's: VTT-500 and VTT-504 keep their "and", since each names one
  panel's box and its Send, held by one test;
- Task 3's: the DM's placeholder naming `/ooc`, under "What was
  deliberately left out"; and VTT-500 and VTT-501 keep the builder fixture
  tests beside the panel tests, since those hold the wire shape;
- Task 4's: the browser test cannot run twice against one campaign, under
  "What could not be established".

| Task | Pass 1 | Pass 2 |
|---|---|---|
| 0 | 2 Must, 7 Should, 8 Nits | 2 Should, 2 Nits |
| 1 | 1 Must, 4 Should, 4 Nits | 1 Nit |
| 2 | 3 Must, 2 Should, 5 Nits | — |
| 3 | 1 Should, 6 Nits | — |
| 4 | 2 Should, 5 Nits | 2 Nits |

The findings that changed the work:
- Task 0's: Task 1 regenerated `contract/gen/ts`, which the bundle embeds,
  so Task 1 carries `cmd/vtt/webdist`. VTT-502 said every window draws an
  Events panel, against VTT-445. VTT-495 let a player's line out of
  character name their own party member, which D2 refuses. Both rows,
  still uncommitted, were reworded in place.
- Task 1's: QA's Go tests sat in a new `contract/*_test.go`, which
  `check:arch` refuses. VTT-505's tests checked one field name, and a
  `speaker_actor` field passed them.
- Task 2's: VTT-506 said one text for any line, while a line out of
  character that names an actor gets its own. `mayVoice`'s comment claimed
  one string where it answers otherwise.
- Task 3's: no test showed a player's switch of party member keeping the Say
  text with Send speaking as the new one. A player's `/ooc` sent the hidden
  speaker box's state (K23).
- Task 4's: a client typing "Human Fighter" itself would have passed W1, so
  W1 asserts the frames Lera's page sends (K24).

This report had one reviewer, two passes, by command, rerunning the named
tests at `f40af43`, `466cfbe` and `db81c37` in scratch copies and reading the
session's logs. Pass 1: 2 Must, 3 Should, 6 Nits. The Musts: VTT-502 and
VTT-503 were said to be beyond the plan's sort, which allocated both; and the
QA files were said to have changed only in what their labels cite. The
Shoulds: the QA agents' inputs; a deviation's why; a citation of "ticket 6".
Pass 2: 1 Should, 4 Nits: who changed the QA files, the session or a review.
All taken.

## The deliberate breaks

Each was made by hand, run, and undone by its inverse edit or by restoring a
saved copy, checked by `cmp`. A contract break was regenerated and the
regenerated files compared. A browser break was built into a copy's bundle,
and the copy was discarded.

| Break | What it did | Red |
|---|---|---|
| B1 | `AddNarration.actor_id` dropped | its command round trip, Go and TS |
| B2 | `AddNarration.out_of_character` dropped | its command round trip, Go and TS |
| B3 | `NarrationAdded.out_of_character` dropped | the envelope round trip, Go and TS |
| B4, B5 | `actor_id`, then `speaker_actor`, added to `NarrationAdded` | both VTT-505 tests |
| K1 | `Authorize` skips the speaker check | items 3 and 4, and the other Authorize tests |
| K2 | `controls` in place of `ownPartyMember` | item 3's creature |
| K3 | no refusal of a nameless party member | its test |
| K4 | no in-character stamp | item 5 |
| K5 | the command's `as` kept out of character | item 6's appending tests |
| K6 | `ToEvent` drops the mark | the same, and the converter test |
| K7 | the in-character stamp for every role | item 7 |
| K8 | a DM's actor id accepted | VTT-499's test |
| K9 | out of character naming an actor accepted | "TestALineOutOfCharacterSpeaksAsNoActor" |
| K17 | `ToEvent` drops the narration's anchor | the converter test |
| K19 | the out-of-character stamp for a player only | item 6's DM and agent cases |
| K20 | the one refusal text varied by id | both VTT-506 tests |
| K10 | the speaker box drawn for a player | item 1 |
| K11 | Send without the actor | item 1; W1 |
| K12 | the actor with no fallback | "with no party member chosen, a player's Send speaks as their first" |
| K13 | `/ooc` kept in the text | item 2 |
| K14 | a prefix match for the first word | "only /ooc as the line's first word steps out of character" |
| K15 | the Story draws a line out of character as speech | item 6's Story test; W1 |
| K16 | `describe` ignores the mark | item 6's Events panel test |
| K18 | the speaker box removed for every role | the DM's tests |
| K21, K22 | the feed drops the mark; no parentheses | item 6's Story test |
| K23 | a player's `/ooc` carries the hidden box | its test |
| K24 | the client sends `as` "Human Fighter" itself | W1's frames alone |

And by hand, as the mutation gates would make them:
- the gateway: 11 mutants, each new condition negated, the switch's cases
  included, which gremlins does not mutate;
- the client: 30 mutants, among them the regex's anchors and classes, the
  trims, the slice, the booleans, `speaksFreely`'s conditions, both
  placeholders, the builders' literals, the class and `describe`'s speaker.

Each reds a test.

## Rule 9: how MapTool does this

Read in `~/dev/RPTool/maptool` at `f4b7fef6c`.
- The speaker is the token a player impersonates, `CommandPanel`'s
  `TokenIdentity`. It is taken from the token's menu,
  `AbstractTokenPopupMenu.ImpersonateAction`, or by `/im`, and kept until
  changed.
- `MessageUtil.getFormattedSay` writes `<identity>: <text>`, and with
  nothing impersonated the identity is the player's own name.
- `OOCMacro` writes `getFormattedOoc`: the player's own name, then the text
  in `(( ))`, never the token's.
- Under restricted impersonation, `ImpersonateMacro.canImpersonate` lets a
  player take only a token they own, and the GM any token or bare name. The
  check runs in the client.

This client borrows the shape:
- the speaker is the character the player plays, here the viewpoint the
  panel acts as;
- `/ooc` is the first word, and `name: (( text ))`;
- the GM keeps a free name.

It does not borrow the trust. The server refuses a speaker the player does
not play and writes both names, as `CLAUDE.md`'s rule 9 says of MapTool's
distribution model. A player's line with nothing impersonated does not fall
back to the player's own name, by "Alltid som karaktären".

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The plan's C6 as one row | VTT-500 for the panel and its speaker, VTT-501 for `/ooc` | Two obligations, broken by different breaks |
| VTT-495 to VTT-504 | VTT-505 and VTT-506 as well | Phase 4a cited D1's "the event carries no actor id" and D2's one text, and the controller allocates an id a sentence earns by being cited |
| C1 and C7 as the plan's draft worded them, and VTT-506 as Phase 4a's citation worded it | VTT-495 and VTT-502 reworded in Task 0, VTT-506 in Task 2, each before its commit | Each was found false as worded by a review while still uncommitted and cited by nothing (Phase 4b) |
| Two new contract fixtures | Three | `add_narration_out_of_character_command.json` gives the command's mark its own round trip |
| Phase 4a's Go tests of Task 1 in a file of their own | At the end of `contract/roundtrip_test.go` | `.go-arch-lint.yml` excludes contract/'s test files one by one, and a new one fails `check:arch` in the pre-commit hook. Widening the exclusion would be a gate change (rule 2) |
| The protos' docs say who writes each field (Task 1) | Pointers to the rows only | The server reads the fields from Task 2 on, so a sentence about what it does would have been false at Task 1's commit. Who writes each goes to SPEC-007 |
| Unit tests of the stamp's three arms beside S1 to S4 (D3) | None | The live tests of items 5 to 7 reach each of `voice`'s arms, and M9 to M11 red them, so a unit test would hold nothing more |
| `client/test/qa-say-boxes.test.ts`, the QA file of `docs/reports/2026-10-25-a-repaint-keeps-what-is-being-typed.md`, "about the as box move to a DM's panel" | The whole file draws a DM's panel | Its tests are written against the two boxes, and the DM's panel keeps both. The player's single box is held by spectator-view's tests and Task 3's QA |
| The player's placeholder unchanged unless needed | "say something, or /ooc to step out of character" | `/ooc` has no other mark on screen. The two locators that found the box by its placeholder find it by `data-field` |
| `client/e2e/speaking.spec.ts` alone | `asDM` moved to `client/e2e/as-dm.ts`, shared with `typing.spec.ts` | Task 4's review: it was a verbatim copy |

## What could not be established

- Two participants may join under one display name: neither the
  `participants` table, whose `display_name TEXT` carries no constraint, nor
  `usableDisplayName` in `internal/gateway/join.go` refuses it, and nothing
  in an out-of-character line tells them apart. MapTool refuses a second
  connection under a connected name, since its ownership is by name. Here it
  is by participant id. Whether that matters at a table was not
  established.
- No test narrates from an MCP client seated as a player: the `add_narration`
  calls in `cmd/vtt`'s tests use the agent's or the DM's token, and
  `internal/mcp`'s go to a fake wire. Such a client is refused for a line with
  text alone, and the tool's docs are its only guide.
- A display name longer than 256 bytes, which `vtt invite --name` can make
  and a join cannot. That participant's line out of character is refused by
  the fold. This is near-theoretical with a small consequence, so it gets no
  code, by the owner's ruling of 2026-10-21.
- A party member whose token a player's window names otherwise speaks under
  its actor's name (D5).
- A player's nameless party member. Its chip shows its id, and Send is
  refused by the server, by which time the box has already been emptied, as
  for every refusal.
- Firefox and WebKit were not run. `playwright.config.ts` names no project.
- `client/e2e/speaking.spec.ts`, like `typing.spec.ts`, cannot run twice
  against one campaign: a second `loadAdventure` collides, and Wren already
  exists. It leaves no session open, which is what `client/e2e/setup.ts`
  asks.

## What was deliberately left out, and where it went

- The DM's placeholder, "say or narrate", does not name `/ooc`, though the
  DM's panel reads it. Raised to the owner with this report.
- A player who controls no party member perches and draws no panel, so
  cannot `/ooc` from the browser, though the server accepts the line. This
  is older than this ticket. Raised to the owner with this report as a
  candidate for a ticket.
- MapTool's other speech commands, `/me` and the whisper, were not asked for.
  No ticket.
- The DM's browser console has no Say form: of `client/src`, only
  `view/player.ts` sends a narration. A human DM narrates nowhere but through
  the player panel of a character granted to them, or through MCP. This is
  older than this ticket. Raised to the owner with this report.
- SPEC-013, SPEC-007, SPEC-016 and SPEC-022: the commit that follows this
  report.

## Figures

From the `task check` on `db81c37`, launched with this report and the
changes to SPEC-013, SPEC-007, SPEC-016 and SPEC-022 in the tree,
uncommitted; it exited with code 0. Every figure given for `e33376f` is one
the Figures of `docs/reports/2026-10-25-a-repaint-keeps-what-is-being-typed.md`
give for `5d01719`, whose code and tests `e33376f` carries unchanged.

- Go coverage: 21 packages at or above their floors; the gate notes 13 that
  clear theirs by more than 1.0pp. `internal/gateway` measured 94.2% against
  its 93.5% floor, up from 94.0% on `e33376f`, and `internal/perceive` 95.7%
  against its 94.5% floor, as on `e33376f`.
- Client coverage: 24 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors.
  - 13 packages ran, `internal/gateway` among them; `internal/artlib` and
    `internal/campaigncfg` reused the verdict of their last passing run,
    nothing in their dependency closure differing.
  - `internal/gateway`: 2 survivors, both adjudicated, one in `authz.go` and
    one in `viewpoint.go`.
  - 7 timed out, counted as killed and not measured: 4 in `internal/sight`,
    1 each in `internal/mcp`, `internal/perceive` and `internal/rules`, as on
    `e33376f`.
  - `tools/mutation-equivalents.txt` is unchanged since `a8bb0ef`, on `main`
    before this ticket.
- Client mutation, in its container: Stryker's own "Done in 86 minutes and
  38 seconds". 3276 mutants by the gate's line, Stryker's 3282 less the 6 it
  ignores: 3204 killed, 9 timed out and counted as killed, and 63 survivors,
  all adjudicated equivalent; on `e33376f` they were 3241, 3169, 9 and 63.
  - The 9 timed out are in `canvas.ts`, `scene-plan.ts` and `wire.ts`, which
    this ticket does not change.
  - `tools/ts-mutation-equivalents.txt` is unchanged since `65781c5`, on
    `main` before this ticket, and no survivor sits on a line this ticket
    added or changed, by the survivors' lines in
    `reports/mutation/mutation.json`, the run's own Stryker report, which is
    not tracked, against `git diff main -U0`.
  - `client/src/view/player.ts`: 269 killed, none survived, against 261.
  - `client/src/view/spectator.ts`: 458 killed and 2 survived, against 454
    and 2.
  - `client/src/commands.ts`: 177 killed, none survived.
  - `client/src/player.ts`: 84 killed and 15 survived, and
    `client/src/view/feed.ts`: 86 killed and 15 survived; each file has 15
    rows in `tools/ts-mutation-equivalents.txt`.
- Race: both runs pass.
- Requirements chain: 506 rows, 316 test files, 16 specifications.
- Comments: 339 files, clean, with 37 comment lines added against `main`.
  Its two notices are the ones `e33376f` had:
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6, and
  `internal/gateway/export_test.go` at 49.18 against 45.9.
- New prose, which reads Go and TypeScript only: 1758 added lines across 33
  files, clean.
- Disk: 26.7 GiB free, by the gate's own line, when the Go mutation step
  began. During the client mutation step a first watcher stopped at its
  first reading below 8 GiB, 7.8 GiB. A second, sampling every 20 s from
  14:47:07, read no less than 12.2 GiB, and `df -h` read 17 GiB within 20 s
  after the exit.
- Duration: 10130 s, from the launch at 12:20:00 to the exit at 15:08:50 on
  2026-10-26.

`task e2e`, once, against the same tree after the gate: 29 passed and 1
skipped, in 34.0 s. The skipped one is "a network drop while the table plays
on ends in a Reconnect", a `test.skip` in `client/e2e/handover.spec.ts`.
