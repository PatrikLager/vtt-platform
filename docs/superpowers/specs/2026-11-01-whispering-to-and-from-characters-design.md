# The DM whispers to a character, and a character to the DM

## The problem

**Everything said at the table reaches every character.** The one command
that carries words, `AddNarration` in `contract/vtt/v1/commands.proto`,
becomes a `NarrationAdded`, and `Projection.classify` in
`internal/perceive/projection.go` forwards that event to every viewer ("it is
addressed to the table"), so it is written into every active character log
(VTT-362, SPEC-016). No command or event in the contract is addressed to one
character: each arm of `classify` forwards to every log, withholds from
every log, or decides by sight, by a note's visibility or by party
membership, and a case-insensitive search for "whisper" in `contract/vtt/`,
`internal/` and `client/src/` finds nothing. So the DM
cannot tell Tobbe that an arrow came from the dark without telling Lera too,
and the DM can only write what the least informed character may know. A
player cannot say anything to the DM that the other characters do not hear.

The alpha test of 2026-10-22 found it, and the owner ruled B-2 then: a whisper is addressed to a CHARACTER and is shown only in that
character's log; a whisper to a PLAYER is shown in every character that player
holds; a player's whisper to the DM comes from the character ("Tobbe viskar")
and is shown only in that character's log and to the DM
(`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`). On 2026-11-01 the
owner ruled how it is written, "/w och /dm i Say-rutan": the DM writes
`/w Tobbe text` in the console's Say box, a player writes `/dm text` in theirs,
as `/ooc` is written today (`readLine` in `client/src/player.ts`); and who sees
it, "Alla som ser genom Tobbe": whoever watches through Tobbe's log, his
controllers and anyone perched on him.

MapTool's `/w` (`WhisperMacro`) whispers to a player by name, quoted when it
has a space, and `/gm` (`ToGMMacro`) to the GM. Every MapTool client receives
every whisper and drops the ones not addressed to it (`TextMessage`'s
`WHISPER` channel). The syntax is borrowed; that distribution is not, because
here the server sends a log only what its character perceives.

## Done looks like

1. A whisper from the DM to a party member is written into the world log and
   into that party member's character log, and into no other character log.
   A campaign test names this, and it fails on today's tree, where no whisper
   exists.
2. A whisper from the DM naming a player reaches the log of every party member
   that player controls, and no other log.
3. A player's whisper to the DM, from a party member they control, is written
   into the world log and that party member's log, and into no other log. It
   is refused, and nothing is appended, when the player does not control that
   party member and when its issuer is a spectator. A player cannot whisper
   to a character.
4. A whisper's text is bounded as a narration's text is (SPEC-018), and an
   empty one is refused.
5. In the DM console's Say box, `/w Tobbe text` sends a whisper to the party
   member named Tobbe, `/w "Two Words" text` to one whose name has a space,
   and a player's name in place of a character's to the party members that
   player controls. An unknown name, a player who controls no party member, or
   a whisper with no text is told in plain words, sends nothing, and leaves
   the box as typed. A client test names each.
6. In the player panel's Say box, `/dm text` sends a whisper to the DM from
   the party member the window watches. A line that begins with `/w` is not
   said aloud: the player is told to whisper with `/dm`. A client test names
   both.
7. The Story shows a whisper as a whisper, naming whom it is from and to, in
   the DM's window and in the window of whoever watches through a recipient.
   A browser test with the DM, Lera and Tobbe in three windows: the DM
   whispers to Tobbe, Tobbe's window shows it and Lera's does not; Tobbe
   answers with `/dm`, the DM's window shows it and Lera's does not; a
   spectator perched on Tobbe sees both.
8. The agent has a whisper tool, and its description says whom a whisper
   reaches.
9. `task check` exits 0 on the branch.

## Rules this puts on the system

- A whisper is written into the world log and into the logs of the party
  members it is addressed to, and into no other log.
- A whisper addressed to a player reaches every party member that player
  controls.
- A player whispers only to the DM, and only from a party member they control;
  a spectator does not whisper.
- Only the DM and the agent whisper to a character.
- A whisper's text is bounded as a narration's text is.
- A Say line that begins with a whisper command is never said aloud.

## What it touches

In this order, because each layer is what the next one calls:

1. `contract/vtt/v1/commands.proto`, `contract/vtt/v1/events.proto` and the
   generated code under `contract/gen/`: a new command and a new event,
   additive (ADR-007, CLAUDE.md rule 3).
2. `internal/engine/apply.go`: the event's arm and its bound (SPEC-018).
3. `internal/gateway/authz.go` (`commandRoles`, `playerRules`, and
   `mayWhisper` beside `mayVoice`, for every role) and
   `internal/gateway/convert.go` (`ToEvent`'s arm, carrying the command's
   fields as they came; `voice` does not change).
4. `internal/perceive/projection.go` (`classify`): a whisper forwarded only
   to a log it is addressed to.
5. `tools/toolgen/main.go`, `contract/gen/tools/tools.json` and
   `cmd/vtt/tools.json`: the agent's tool.
6. `client/src/commands.ts`, `client/src/player.ts` (`readLine`),
   `client/src/view/dm.ts`, `client/src/view/player.ts` and
   `client/src/view/spectator.ts` (`describe`), with their tests, and a new
   browser test under `client/e2e/`.
7. `docs/requirements.md`: the rules above.

Six components and more than ten files. Whether to split them is asked at
sign-off.

## Specifications this moves

docs/specifications/007-the-wire-contract.md
docs/specifications/013-authorization.md
docs/specifications/016-the-projection.md
docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- Whether the DM's whisper speaks under the console's "in character as…" name
  when one is given, so that a creature can whisper, or always as the DM.
  MapTool's whisper speaks as the identity the sender has chosen. The plan
  proposes, the owner decides.
- Where a whisper to a player becomes a list of party members: in the console
  when it is sent, or in the server when it is appended. A grant landing
  between the two decides which characters hear it. The plan's.
- Whether each recipient's log is told who else a whisper went to. A whisper
  to a player with two characters reaches two logs; neither may learn of a
  character outside the whisper. Not read.
- Whether a party member with no name can whisper to the DM. `mayVoice`
  refuses an in-character line from one ("ask the DM for one"). Not decided.
