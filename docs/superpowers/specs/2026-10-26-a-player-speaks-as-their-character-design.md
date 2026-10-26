# A player's line speaks as the character they play, and /ooc as the player

## The problem

**A player names their own speaker, and may name anyone.** The player
panel's "Say something" form, in `renderPlayerPanel` in
`client/src/view/player.ts`, draws a free-text box, "in character as…
(optional)" (`data-field` `say-as`), beside "say or narrate" (`say-text`).
Send, or Enter in the text box, calls `submit`, which sends `addNarration(t,
as.value.trim() || undefined)` and empties both boxes, so a player speaking
in character retypes the name before each line. The panel already knows whom
the player plays: `startSession`'s `paint` in `client/src/app.ts` sets
`ui.selectedActorId` to the viewpoint, the character the player acts as
(VTT-408), and the panel draws it as the chosen chip; the form does not use
it. On the server, `ToEvent` in `internal/gateway/convert.go` copies the
command's `as` into `NarrationAdded.as`, `playerRules`' entry for
`add_narration` in `internal/gateway/authz.go` is `unrestricted` (SPEC-013),
and the fold refuses only an `as` over 256 bytes (`maxNarrationAsBytes`,
SPEC-018). So a player's line can speak as another player's character, as a
creature or as "DM", and every window draws it so: `renderFeed` in
`client/src/view/spectator.ts` writes `as: text` as a `speech` line, and
`describe` words it the same for the ticker.

**A player's nameless line is the DM's.** With `as` empty, `renderFeed` draws
the line as `narration`, the shape of the DM's narration. `NarrationAdded`
carries nothing else about who wrote it, and the envelope's `actor_role` and
`participant_id` never reach a player or a spectator (SPEC-016), so no window
can tell the two apart.

**There is no way to speak out of character.** Nothing in `client/src` reads
a leading `/`: a line typed as `/ooc where is the pizza` is sent as that text,
and drawn as whatever the `as` box held.

**The DM and the agent narrate with a free speaker, and keep it** (the owner's
rulings "Ja för DM" and "Ja, som DM", 2026-10-26). They narrate through the
MCP server's `add_narration`, whose description in `tools/toolgen/main.go`
says to set `as` to voice an NPC or a PC and to omit it to speak as oneself;
the DM's browser console draws no narration box.

**How MapTool does this (rule 9).** In `~/dev/RPTool/maptool` at `f4b7fef6c`
there is no speaker box. The speaker is the token the player impersonates,
`CommandPanel`'s `TokenIdentity`, taken from the token's menu
("Impersonate") or by `/im`, and kept until changed; `MessageUtil
.getFormattedSay` writes `<identity>: <text>`. `/ooc`, `OOCMacro`, writes
`MessageUtil.getFormattedOoc`: the player's own name, then the text in `(( ))`,
never the token's. Under restricted impersonation, `ImpersonateMacro
.canImpersonate` lets a player take only a token they own, and the check runs
in the client. This platform borrows the shape and not the trust, as rule 9 of
`CLAUDE.md` says of MapTool's distribution model: here the server decides who
may speak as whom.

## Done looks like

1. A client test renders the player panel for a player who controls two party
   members and plays the second, types `I draw my sword` and presses Send: the
   narration sent speaks as the second party member. The panel draws no box
   for the speaker. Today it draws "in character as…", and the narration sent
   names no speaker.
2. A client test types `/ooc where is the pizza` into the same panel and
   presses Send: the narration sent is out of character, its text `where is
   the pizza`, and speaks as no character. Today the text sent is `/ooc where
   is the pizza`.
3. A gateway test: a player's `add_narration` speaking as an actor the player
   does not control, or as a creature they control that is not a party
   member, is refused, and one speaking as a party member they control is
   accepted. Today all three are accepted.
4. A gateway test: a player's `add_narration` that neither speaks as an actor
   they control nor is out of character is refused, with `as` set to `DM` and
   with `as` empty. Today both are accepted.
5. A test reads the `NarrationAdded` that a player's in-character line
   appends: its speaker is the name the world holds for the actor it speaks
   as, whatever name the client sent. Today it is whatever the client sent.
6. A test reads the `NarrationAdded` that an out-of-character line from a
   player who joined as `Ada` appends: it is marked out of character and names
   `Ada`, whatever name the client sent; the Story of another seat, and the
   DM's Events panel, draw it as `Ada: (( where is the pizza ))` (the owner's
   ruling "Namn: (( text ))", 2026-10-26). Today no line is out of
   character.
7. A gateway test: the DM's and the agent's `add_narration` with `as` set to
   `Goblin chief`, and with `as` empty, are accepted and append that `as`. This
   holds today and must still hold.
8. A browser test: a player plays a party member and sends `I draw my sword`;
   another player's Story reads `<that party member's name>: I draw my
   sword`. Then the first sends `/ooc brb` and the other's Story reads `<the
   first player's name>: (( brb ))`. Today the first line reads `I draw my
   sword` as narration, and the second `/ooc brb`.

## Rules this puts on the system

- A player's narration either speaks as a named party member that player
  controls or is out of character; the server refuses any other.
- Every viewer sees a player's in-character line under the name the world
  holds for the actor it speaks as.
- Every viewer sees a player's out-of-character line marked as such, under the
  name the player joined with, which the server sets.
- The player panel's line speaks as the character the player plays, and a
  line that opens with `/ooc` is out of character instead.
- The DM's and the agent's narration keeps the speaker they give it, or none,
  unless it is out of character.

## What it touches

One ticket and one branch, the server first (the owner's ruling "Ett ärende,
servern först", 2026-10-26). It crosses more than two components and names
more than ten files, which is why the order was put to the owner.

1. The contract, additively (ADR-007): `NarrationAdded` in
   `contract/vtt/v1/events.proto` has no way to say a line is out of
   character, and `AddNarration` in `contract/vtt/v1/commands.proto` none to
   say which actor a line speaks as. Regenerated by `task generate:contract`.
2. The server: `playerRules`' entry for `add_narration` in
   `internal/gateway/authz.go`, `ToEvent` in `internal/gateway/convert.go`, and
   `internal/engine/apply.go`'s check of a narration if a new field needs a
   bound. Tests in `internal/gateway/authz_test.go` and a gateway test of what
   is appended.
3. The agent's tool: `add_narration`'s description in `tools/toolgen/main.go`,
   if its fields change, and what it generates.
4. The client: `addNarration` in `client/src/commands.ts`; `renderPlayerPanel`
   and `PlayerUIState` in `client/src/view/player.ts`; `renderFeed` and
   `describe` in `client/src/view/spectator.ts`; `Narration` in
   `client/src/view/feed.ts`; the narration's bounds in `client/src/fold.ts`.
   Tests in `client/test/spectator-view.test.ts` and a browser test under
   `client/e2e/`.
5. The tests of the Say box that go with it: `spectator-view.test.ts`'s "what
   a player typed in Say survives a repaint, with the as box focused",
   `client/test/qa-say-boxes.test.ts`, and the rows of VTT-491 and VTT-493 in
   `docs/requirements.md` that cite them.
6. `cmd/vtt/webdist`, rebuilt by `task build:client`.

## Specifications this moves

docs/specifications/013-authorization.md
docs/specifications/016-the-projection.md
docs/specifications/007-the-wire-contract.md
docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- Whether `docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`
  moves: only if the contract gains a free-text field, which is the plan's
  choice.
- Whether two participants can join under one name. The `participants`
  table in `internal/identity/identity.go` declares `token_hash BLOB UNIQUE`
  and `display_name TEXT` with no constraint, and no join path was read for a
  check of its own; two players named alike would be told apart by nothing in
  an out-of-character line.
- What an out-of-character line tells a viewer that the table did not know.
  `PresenceSnapshot` already carries every present participant's display name
  to every connection (SPEC-007); the line ties a name to what was said, by the
  owner's ruling.
- A player who controls no party member draws no player panel and so no Say
  form (`canAct` in `startSession`'s `paint`); this ticket leaves that so.
