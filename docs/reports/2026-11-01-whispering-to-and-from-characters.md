# The DM whispers to a character, and a character to the DM: the change

**Ticket:** `docs/superpowers/specs/2026-11-01-whispering-to-and-from-characters-design.md`.
An agent that did not write it verified it and passed it with gaps G1 to
G12.
**Plan:** `docs/superpowers/plans/2026-11-01-whispering-to-and-from-characters.md`,
written by that verifier and amended by Task 0's review.
**Where it came from:** finding B-2 of the alpha test of 2026-10-22,
item 6 of the nine in `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`:
everything said at the table reached every character, so the DM could not
tell one character what the others must not hear. The owner: "då kör vi
B-2".
**The owner's rulings:**
- 2026-10-22, B-2: a whisper is addressed to a character and shown only in
  that character's log and to the DM; a whisper to a player is shown in
  every character that player holds; a player's whisper to the DM comes
  from the character ("Tobbe viskar").
- 2026-11-01, how it is written: "/w och /dm i Say-rutan".
- 2026-11-01, who sees it: "Alla som ser genom Tobbe".
- 2026-11-01, at sign-off: a DM's whisper always speaks as the DM, never
  under the console's "in character as…" name (Q1 (a)); one ticket (Q2).
- 2026-11-01, during Task 2's QA: MapTool's `/whisper` reads as `/w`, and
  `/gm` and `/togm` as `/dm`: "Ja, som alias".

**The last code commit is `b612de4`.** Every code reference below is to
that tree unless it names another commit.

## The period, in commits

| Commit | What it holds |
|---|---|
| `cf91612` | Task 0: the ticket, the plan, VTT-582 to VTT-598; VTT-529 withdrawn |
| `dd6ebb6` | Task 1: `WhisperAdded`, both folds, `classify` and the cut, the Story's line; QA's `internal/campaign/qa_whisper_test.go` and `client/test/qa-whisper.test.ts` |
| `f30b2ad` | Task 2: `AddWhisper`, `mayWhisper`, the `add_whisper` tool, `readLine`'s kinds and MapTool's aliases, the console's `/w`, the panel's `/dm`; VTT-598 withdrawn for VTT-600, VTT-599 new; QA's `internal/gateway/qa_whisper_test.go` and `client/test/qa-whisper-say.test.ts` |
| `b612de4` | Task 3: `client/e2e/whispering.spec.ts` |
| `08f3c58` | The comment ceilings, written once after the last code commit (D18) |

## What the code does now

**The event.** `WhisperAdded` (Envelope arm 35) carries a text and either
`to_actor_ids`, the party members the DM or the agent whispers to, or
`from_actor_id`, the party member whose player whispers to the DM.
`engine.Apply` and the client's `fold` refuse an empty text, one over 8192
bytes, and one naming an actor they have not folded, and change no state.

**Where it is written.** The world log takes every whisper. A character
log is written through `perceive.Projection.Event`, and `classify`
forwards a whisper only when the log's eyes include its `from_actor_id` or
one of its `to_actor_ids`. `Event` cuts the forwarded copy's
`to_actor_ids` to the log's own eyes, so a log never learns who else was
whispered the same. `look` sets the eyes in `sightView.eyes`.

**The command.** `AddWhisper` (ClientCommand arm 36) carries the same three
fields, and `ToEvent` copies them as they came. `commandRoles` gives it to
the dm, the agent and the player. `mayWhisper`, in `Authorize` for every
role, refuses a DM's or an agent's whisper that names no actor, one that
is not a party member, or a whisperer. It refuses a player's that names a
recipient, and refuses with one text one whose whisperer is empty, unknown,
another player's or not a party member. A nameless party member of the
player's own is refused with "this party member has no name; ask the DM
for one". The agent has the tool `add_whisper`, whose description says
whom a whisper reaches and that a party member's whisper to the DM is
answered with `add_whisper`, never `add_narration`.

**The boxes.** `readLine` reads a Say line as one of four kinds: a line
said in character, a line out of character after an exact `/ooc`, a
whisper after `/w` or `/whisper`, and a whisper to the DM after `/dm`,
`/gm` or `/togm`, those five in any case. The console's Say sends a
whisper to the party member the name names, or else to every party member
of the participant it names, in actor-id order (`whisperTargets`). An
unknown name, a name two party members share, a participant who controls
no party member, a missing name, a missing text and a whisper to the DM
are told through `notify`, send nothing and keep the line. The panel's Say
sends a `/dm` line from the party member it acts as, and tells a player
who types `/w` to use `/dm`, through a new optional `notify` that
`app.ts` wires to the toast, as it wires the console's. Both placeholders
name the whisper command beside `/ooc`.

**The line.** `describe` words a whisper "The DM whispers to Tobbe, Brann:
…" or "Tobbe whispers to the DM: …", naming each actor as the window names
it, and `renderFeed` draws it with the class `whisper`, in italics.

## Done looks like, answered

1. **[x]** A whisper from the DM to a party member is written into the
   world log and that party member's log and no other.
   `TestAWhisperReachesTheWorldAndTheLogsItNamesAlone` in
   `internal/campaign/whisper_test.go`, over the wire
   `TestTheDMWhispersToAPartyMemberOverTheWire` in
   `internal/gateway/whisper_test.go`, and in the browser E1.
2. **[~]** A whisper naming a player reaches every party member that player
   controls, and no other log. **It is the console's, not the server's.**
   The contract carries actor ids alone (D2), so the console turns a
   participant's name into that participant's party member ids when the
   line is sent (D7, G4), and the agent names each party member, as its
   tool says. dm-view's "a participant's name, whatever its case, whispers
   to every party member they control, by id, and a party member's name
   wins over a participant's" holds the console; H1 holds a whisper to two
   party members reaching their two logs alone; E1's `/w Watcher` holds
   both in the browser, and on a rerun the DM's line names every Tobbe in
   id order.
3. **[x]** A player's whisper to the DM is written into the world log and
   that party member's log and no other, and is refused, appending nothing,
   when the party member is not theirs, when a spectator sends it, and when
   it names a character to whisper to.
   `TestAPartyMembersWhisperReachesTheWorldAndItsOwnLogAlone` (campaign),
   `TestAPlayerWhispersToTheDMFromTheirOwnPartyMemberAlone` and
   `TestASpectatorCannotWhisper` (gateway), and the four `add_whisper` rows
   of `TestAuthorizeTableAllCommandsAllRoles`.
4. **[x]** The text is bounded as a narration's: 0 and 8193 bytes refused,
   8192 accepted, in both folds. `TestAWhisperWhoseTextIsEmptyOrOverTheBoundIsRefused`,
   `TestAWhisperAtTheTextCapIsAccepted`, the fold-rejections whisper tests,
   and QA's `TestQAWhAWhispersTextIsBoundedOverTheWire` over the wire.
5. **[x]** The console's `/w`, a quoted name, a player's name, and each
   refusal told, sent nothing and kept. dm-view's four whisper tests and
   "the console whispers /whisper Name as /w Name, and refuses /gm and
   /togm as /dm"; the quoted name by player.test's "a Say line whose first
   word is /w or /dm, in any case, is a whisper", and at the console by
   QA's "Enter whispers a quoted name, any case, and a participant's name to
   their party members".
6. **[x]** The panel's `/dm`, and a `/w` line told and never said aloud.
   spectator-view's "a player's /dm whispers to the DM from the party
   member the panel acts as, and keeps the line until accepted", "a
   player's /w line, or a /dm line with nothing after it, sends nothing,
   tells them why and keeps the line", "a player's /gm and /togm whisper to
   the DM, and a /whisper line is refused", and app's "a player's /w line
   is told in the toast".
7. **[x]** The Story shows a whisper as a whisper, naming whom it is from
   and to, in the DM's window and the windows that watch through a
   recipient. spectator-view's "the Story draws the DM's whisper naming
   whom it is to, with the class whisper" and "the Story and the Events
   panel draw a party member's whisper to the DM by its name", and E1 with
   four windows: the DM, Lera, Tobbe and a spectator perched on Tobbe. The
   ticket named three; the fourth, the spectator, joins by the share link
   (G11).
8. **[x]** The agent's tool says whom a whisper reaches.
   `TestAddWhisperSaysWhomItReachesAndHowToAnswer` in
   `tools/toolgen/main_test.go`.
9. **[x]** `task check` exits 0 on the branch. On `08f3c58`, the ledger
   commit after the last code commit: `TASK_EXIT=0`, from 2026-11-01 22:45
   to 2026-11-02 00:13, the tree unchanged across the run. See Figures.

## What the rules became

- A whisper is written into the world log and the logs it names, and no
  other: **VTT-582** (to party members), **VTT-583** (from one), and
  **VTT-584** (each copy names only its log's own character).
- A whisper to a player reaches every party member that player controls:
  **VTT-593**, the console's; **VTT-591**, the tool tells the agent to name
  each.
- A player whispers only to the DM, from a named party member of their own:
  **VTT-588**, with **VTT-589** for the one text; a spectator does not:
  **VTT-590**.
- Only the DM and the agent whisper to a character: **VTT-587** for what
  theirs may name, and VTT-588 for the player's.
- A whisper's text is bounded as a narration's: **VTT-585**; the folds
  refuse an actor they have not folded: **VTT-586**.
- A Say line that begins with a whisper command is never said aloud:
  **VTT-592** (the reading), **VTT-599** (MapTool's aliases), **VTT-594**
  and **VTT-596** (the refusals), **VTT-600** (every other line).
- Not in the ticket's rules, added by the plan: VTT-584, VTT-586, VTT-587
  and VTT-588's name clause (G6), listed above under the rule each serves;
  **VTT-595** (the panel's `/dm`) and **VTT-597** (the Story's line).
- Withdrawn: **VTT-529** at Task 0, succeeded by VTT-598, a console line
  whose first word is `/w` no longer being said aloud; **VTT-598** at Task
  2, succeeded by **VTT-600**, by the owner's alias ruling.

## Phase 4a

Task 1's QA wrote 10 Go tests and 25 client tests, Task 2's 14 and 24, all
from the requirements and the specifications without the source; every one
passed. Task 3 adds a test and no behaviour, so it had none.

### QA adjudications

- Task 1: the folds accept a whisper with both fields set — by design: the
  command refuses it (VTT-587, VTT-588), and the gateway is the only writer.
- Task 1: the folds accept a whisper with neither field — by design, for
  the same reason.
- Task 1: a repeated recipient keeps `[tobbe, tobbe]` and reads "Tobbe,
  Tobbe" — the console never repeats an id; only the agent's tool could.
  A small consequence: no code.
- Task 1: a whisper to or from a creature alone writes no character log —
  by design: the command refuses it (VTT-587, VTT-589).
- Task 1: a paused log is not written — by design (VTT-316).
- Task 1: SPEC-018's table lacked the whisper's text — taken in by the
  specifications commit that follows this report.
- Task 1, an observation: Bun 1.3.10's `TextEncoder.encode` counts
  `"a".repeat(8191) + "é"` as 8192 bytes, where `Buffer.byteLength` says
  8193; narration's bound alike. QA builds that input with `Array.from`.
- Task 2: two participants whose names differ only in case — refused at
  join (VTT-542) and at `vtt invite` (VTT-543). Not a finding.
- Task 2: the console's people are the roster and presence united, the
  roster's name winning, as the grant select's are. By design.
- Task 2: an unclosed quote takes the rest of the line as the name and
  leaves no text; the console says nobody has that name, or, when the rest
  is a name, that there is no text — D10's choice; the DM retypes.
- Task 2: leading whitespace and tabs read as the command — `readLine`
  trims. By design.
- Task 2: `/whisper Tobbe hi` was said aloud — **a design question, put to
  the owner**: "Ja, som alias". VTT-598 withdrawn for VTT-600, VTT-599
  new, breaks K18b and K18c.
- Task 2: a nameless party member's refusal advised speaking "out of
  character" — **fixed**; it now says "ask the DM for one".
- Task 2: actor-id order is UTF-16 code-unit order, Go's order for any id
  without a character above U+FFFF. By design.

## Phase 4b

| Commit | Must | Should | Nit |
|---|---|---|---|
| `cf91612` | 2 | 5 | 5 |
| `dd6ebb6` | 0 | 4 | 5 |
| `f30b2ad` | 1 | 3 | 4 |
| `b612de4` | 0 | 2 | 2 |

Every finding was taken but two, Task 1's, recorded here:
- A copy with both fields set would name another character through
  `from_actor_id` in a recipient's log. No code: the event is reachable only
  through the command, which refuses it.
- `internal/eventgen` generates no `WhisperAdded`, so the character-log
  property and oracle tests never see one. Outside the plan; teaching the
  oracle a whisper's route is its own work.

The ledger commit `08f3c58` was checked by script: no ceiling raised, 22
lowered, nine new rows.

The ones that changed what the work says:
- The ticket's draft said `convert.go` would write the speaker as `voice`
  does; the plan, and the code, write nothing (Task 0's first Must).
- E1 first did not observe item 2; it gained `/w Watcher` (Task 0's second
  Must).
- W1, W2, W8, W10, C2, C4 and C5 were reworded before their commit (Task 0).
- A test sat under another test's doc comment (Task 2's Must).
- A3 read only the world log for VTT-583; it now reads both players' logs
  (Task 2).
- E1's DM window now reads its own two whispers, and on a rerun the line
  for `/w Watcher` names every Tobbe in id order (Task 3).

## The deliberate breaks

H1 to H3, F1 to F3, S1 to S4, A1 to A4, T1, L1, V1 to V3 and E1 are the
plan's test labels. Each break was applied by a script that asserted the edit could be undone and had
landed, ran the named check, and restored the file byte for byte.

| Break | What went red |
|---|---|
| K1 `classify` forwards every whisper | H1 |
| K2 the arm ignores `from_actor_id` | H2, H3 |
| K3 the cut removed | H1, H3 |
| K4 `engine.Apply` takes 8193 bytes | F1's refusal test |
| K5 `engine.Apply` skips the recipients' actor check; K5b the whisperer's | F2 |
| K6 `fold.ts` takes an empty text | "a whisper with no text is rejected" |
| K7 `fold.ts` skips the actor check | "a whisper naming an actor the fold does not hold is rejected" |
| K8 `describe` leaves out the recipients | S1 |
| K9 `renderFeed` draws a whisper `mechanical` | S1 |
| K10 `describe` names the DM for a party member's whisper | S2 |
| K11 the spectator given a cell | the authorisation table, A4 |
| K12 the DM may name a creature | A2 |
| K13 the DM may set a whisperer | A2 |
| K14 a player may name a recipient | A3 |
| K15 a party member not the player's | A3 |
| K16 the name check dropped | A3 |
| K25 an absent whisperer refused with a text of its own | A3 |
| K17 the tool loses "never add_narration" | T1 |
| K18 `readLine` matches `/w` in lower case only | L1 |
| K18b `/whisper` no alias; K18c `/gm` and `/togm` no aliases | the alias tests |
| K19 participants tried before party members | V2 |
| K20 a participant's party members unsorted | V2 |
| K21 a shared name whispers to the first | V3 |
| K22 a console refusal clears the line | V3 |
| K23 the panel whispers from its first party member | S3 |
| K24 the panel says a `/w` line aloud | S4 |

K14 first showed A3's whisper to a character refused for the wrong reason:
it named no whisperer, so the party-member check refused it before the
recipient check could. The test was fixed before the break ran. Task 1's
breaks ran again after its review's fixes and after `classify`'s three
cases were merged; Task 2's affected breaks ran again after each change to
the tests they hold.

## Rule 9: how MapTool does this

MapTool's `/w` (`WhisperMacro`, alias of `whisper`) whispers to a player by
name, quoted when it has a space; `/gm` (`ToGMMacro`, alias `togm`) to the
GM; `/reply` (`WhisperReplyMacro`) to the last whisperer. A whisper speaks
under the sender's chosen identity. Every client receives every
`TextMessage`, and `MapTool.addServerMessage` drops a whisper not
addressed to it, on the client. The syntax is borrowed, now with
MapTool's long and GM forms as aliases; the distribution is not: here a
whisper is written only into the world log and the logs it names. Not borrowed: `/reply`,
and the identity, by Q1 (a).

## The plan's gaps

- G1, D-2 and D-3 citing nothing: fixed in the ticket before Task 0.
- G2, the files the ticket did not name: each was changed as the plan
  listed, and `server.go` and `voice` were not.
- G3, VTT-529: withdrawn at Task 0; the tests' citations moved to VTT-600
  at Task 2, and SPEC-022's two move with the specifications commit that
  follows this report.
- G4, the player-name rule's layer: the console's (item 2).
- G5, which boxes, commands and cases: both boxes, any case, and by the
  later ruling the aliases.
- G6, the omitted rules: VTT-586, VTT-584, VTT-588's name clause, VTT-587,
  VTT-597.
- G7, the four open points: Q1 asked; D7, D4 and D8 settled. D8 said the
  refusal would use `mayVoice`'s words; QA showed those words advise
  speaking aloud, so it has its own.
- G8, the tool required: written with the command.
- G9, the panel's `notify`: added.
- G10, the ceilings and 100% coverage: every new test went into a new file
  or a file with room; the seven client files the ticket changed that are
  held at 100% (`app.ts`, `commands.ts`, `fold.ts`, `player.ts`,
  `view/dm.ts`, `view/player.ts`, `view/spectator.ts`) stayed there; the
  ceilings were written once at `08f3c58`.
- G11, four people: the fourth joins by the share link.
- G12, the scope claim: the ticket now cites `classify`'s arms.

## The measurements the plan stood on

P4 (the command arm cannot land without its tool, its role cells, its
conversion, the MCP name lists, a builder and a human surface) held: each
table refused the arm until it was filled. P5 (a scratch prototype of the
event's fold and projection against real character logs) held: the
shipped routing is the prototype's, and its two breaks are K1 and K3.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| `readLine` reads `/w` and `/dm` (D10) | It reads `/whisper`, `/gm` and `/togm` too | The owner's ruling during Task 2's QA, after QA found `/whisper Tobbe hi` said aloud: a MapTool habit must not say a secret aloud |
| N529 (VTT-598) | Withdrawn in Task 2 for VTT-600 | VTT-598 sent a `/whisper` line in character; the ruling made that false |
| A nameless party member refused with `mayVoice`'s words (D8) | `mayVoice`'s words without their last clause, ", or speak out of character" | `mayVoice`'s "or speak out of character" advises saying it aloud |
| The plan changed no placeholder | Both name the whisper command beside `/ooc` | The ruling's "same pattern as /ooc"; VTT-528 names `/ooc` in the placeholder, and a player cannot use a `/dm` nobody told them of |
| `classify` gains an arm | Three of its withheld arms are now one case | The pre-commit lint found its cyclomatic complexity at 32 of 30; `SceneCreated`, `ActorAdded` and `TokenPlaced` share one verdict and one comment, as `TokenHidden` and `SceneSeen` already did |
| The ledger lowers what this ticket moved | It lowered 22 ceilings, five of them in files this ticket did not touch (`session.ts`, `wire.ts`, `metadata.go`, `metadata_test.go`, `server.go`), and wrote rows for four of the previous ticket's test files | `--write-ledger` writes every file's present share |

## What could not be established

- Whether a browser's `TextEncoder` shares Bun's undercount (QA, Task 1).
  Not checked; narration's bound is alike.
- The Events panel half of VTT-597 is observed by unit tests (S2, QA), not
  in the browser.
- Whether the agent will repeat an id in `to_actor_ids`; the line would
  read "Tobbe, Tobbe". Not measured.

## What was deliberately left out, and where it went

- Whispering from one player to another: a non-goal; the owner never ruled
  on it.
- `/reply`: not borrowed, not asked for.
- A grant landing between the console's paint and the append (D7): the
  owner's rule on near-theoretical cases; no code, no test, no debt entry.
- A whisper stays in the character's log, so whoever watches through that
  character later reads it (D15): told to the owner at sign-off.
- Whispers in `internal/eventgen`: Task 1's review; recorded above.
- The client's full mutation run (D20): `client/src` changed, so the
  stored report is out of date until the owner asks for
  `task check:ts-mutation:full`.

## Figures

From `task check` on `08f3c58`, 2026-11-01 to 2026-11-02, after
`go clean -cache` and the deletion of three old review clones, with
18.7 GiB free when the Go mutation gate began; it warned that it was
heading for its 16 GiB floor and ran.

- **Go tests:** every package passed, `internal/gateway` (126 s),
  `cmd/vtt` (212 s), `internal/campaign`, `internal/perceive`,
  `internal/engine`, `contract` and `tools/toolgen` among them, under
  `-race` too.
- **Go coverage:** 21 packages at or above their floors;
  `internal/gateway` 95.0% (floor 93.5), `internal/perceive` 95.7%
  (94.5), `internal/engine` 99.4% (97.5), `internal/campaign` 95.0%
  (94.7), `internal/mcp` 89.9% (87.5), `tools/toolgen` 88.2% (85.0).
- **Client coverage:** 24 files gated, each at its floor; the seven
  `.ts` files this branch changed, `app.ts`, `commands.ts`, `fold.ts`,
  `player.ts`, `view/dm.ts`, `view/player.ts` and `view/spectator.ts`,
  at 100%.
- **Go mutation:** 15 packages, zero unadjudicated survivors; no
  adjudication was added by this branch. Seven mutants timed out and
  were counted as killed, NOT measured: one in `internal/perceive`, the
  loop counter of `objectInSight`, which this branch did not change;
  four in `internal/sight`, one in `internal/rules` and one in
  `internal/mcp`, none in a file this branch touched.
- **Client mutation:** not measured. The stored report measured
  `526885bc0c77`; since then the tickets for the console lock keys, a
  name belonging to one participant, the DM booting a participant, and
  this one. `task check:ts-mutation:full` measures it when the owner
  asks.
- **Requirements chain:** 600 rows, 340 test files, 17 specifications;
  every citation resolves.
- **New prose:** 2401 added lines across 42 files, clean.
- **Comments:** 351 files, 40 added comment lines, clean.
- **Browser tests** (outside `task check`): `task e2e` in Task 3, 35
  passed and 1 skipped; `whispering.spec.ts` passed under
  `--repeat-each=2` on the tree committed as `b612de4`.
