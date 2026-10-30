# The DM speaks from the console, always, with /ooc named where they type

## The problem

**A DM who controls no character has nowhere to speak in the browser.**
`renderDMConsole` in `client/src/view/dm.ts` draws no Say form: nothing in
that file builds `addNarration` or `speakOutOfCharacter`, or a `say-as` or
`say-text` box. A DM's Say is the player panel's, `renderPlayerPanel` in
`client/src/view/player.ts`, which `client/src/app.ts` draws for any role
whose window does not perch (`canAct`), and which, for a role with no
controlled actor, draws "You do not control an actor yet." and returns
before the Say form. So a human DM narrates from the browser only through a
character the table has granted them, and otherwise only through the MCP
server's `add_narration`. `client/test/command-surface.test.ts` tags
`addNarration`'s control as the console's (`surface: "dm-console"`) while the
comment above that row says the control is drawn in the player panel.

**The DM's line box does not say that `/ooc` works.** In
`renderPlayerPanel`, `say-text`'s placeholder is "say or narrate" for a DM's
or an agent's panel (`speaksFreely`) and "say something, or /ooc to step out
of character" for a player's, while `submit` reads every role's line with
`readLine` in `client/src/player.ts`, so a DM's `/ooc` line already goes out
of character.

**How MapTool does this (rule 9).** In `~/dev/RPTool/maptool` at
`f4b7fef6c`, `CommandPanel` in
`src/main/java/net/rptools/maptool/client/ui/commandpanel/` is one
chat box for every client, the GM's included; the speaker is the impersonated
identity, kept until it is changed or cleared. This ticket borrows the one box that is always there
for the DM, by the owner's ruling "I konsolen, alltid" (2026-10-30).

## Done looks like

1. A client test renders the DM console for a DM who controls no actor and
   finds a Say form in it: a speaker box, a line box whose placeholder
   names `/ooc`, and a Send. Today the console draws none.
2. A client test types `Grimble` as the speaker and `Hello` as the line in
   the console's Say and presses Send: one narration is sent, text `Hello`,
   `as` `Grimble`. `/ooc brb` sends one line out of character with the text
   `brb`. A blank line, and `/ooc` with nothing after it, send nothing.
3. Client tests hold, for the console's Say, what holds for the panel's
   today: a refused line stays in the line box at the next paint; an
   accepted one empties the line box, keeps the speaker, and asks for a
   repaint; a second Send or Enter before the answer sends nothing; a press
   that sends draws nothing new before its answer; a send whose promise
   rejects is passed to `console.error` and frees the form.
4. A client test renders the player panel for a DM, and for an agent, who
   controls an actor: it draws no line box, no speaker box and no Send. The
   same panel for a player still draws its line box, with the placeholder
   naming `/ooc`, and no speaker box. Today the DM's and the agent's panels
   draw both boxes.
5. A client test in the composed app: a DM who controls no actor types a
   line into the console's Say and presses Enter; the socket receives one
   `add_narration`, and the page holds one line box.
6. A browser test: the DM, granted no character, says `Hello` as `Grimble`
   from the console, and the feed shows `Grimble: Hello`;
   `client/e2e/speaking.spec.ts` still passes.

## Rules this puts on the system

- The DM's and the agent's console draws a Say form whatever they control,
  and its line box's placeholder names `/ooc`.
- The console's Send speaks with what its speaker box holds as `as`, or none
  when it is blank, and a line whose first word is `/ooc` goes out of
  character, as the panel's does today (VTT-501, VTT-504).
- The player panel draws a Say form for a player only. This withdraws
  VTT-504.
- The console's Say keeps what VTT-518, VTT-520, VTT-521, VTT-522, VTT-523,
  VTT-524 and VTT-526 hold for the panel's; those rows were written of the
  panel's Say, a DM's included, and move with it.

## What it touches

One ticket and one branch (the owner's ruling "Ett ärende, en gren",
2026-10-30). It names more than ten files across the client, its tests and
`docs`; the owner was asked how to split it before it was written.

1. The console: `renderDMConsole` and `DMDeps` in `client/src/view/dm.ts`,
   and what `client/src/app.ts` hands it.
2. The panel: the Say form in `renderPlayerPanel`, `PlayerUIState`'s
   `sayAs`, and `speaksFreely`, in `client/src/view/player.ts`.
3. The tests of the DM's Say, which today render the panel:
   `client/test/qa-say-boxes.test.ts` (its table is a DM's),
   `client/test/spectator-view.test.ts`, `client/test/qa-speaking.test.ts`,
   `client/test/qa-refused-line.test.ts`, `client/test/app.test.ts`,
   `client/test/dm-view.test.ts`, and `client/test/command-surface.test.ts`'s
   row and comment for `addNarration`.
4. The browser: `client/e2e/refusals.spec.ts`'s DM line, which today is
   spoken through a granted character, and a test of the DM speaking from
   the console.
5. `docs/requirements.md`: VTT-504, and the rows of the fourth rule.
6. `cmd/vtt/webdist`, rebuilt by `task build:client`.

Any change under `client/src` moves the client mutation report's input hash;
the stored report measured `db81c37` and has been out of date since the ticket
`2026-10-29-a-refused-command-keeps-what-was-typed` changed `client/src`
(SPEC-023).

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- Where in the console the Say form sits, and whether its boxes keep the
  `data-field` names `say-as` and `say-text`: the plan's choice.
- Whether the console's Say draws its text and speaker across a repaint from
  `draft`, as the console's other forms do, or from state of its own: the
  plan's choice; the fourth rule holds either way.
- An agent's browser window: `app.ts`'s `isDM` draws the console for the
  agent as for the DM, so the agent's window gains the Say too. The agent
  narrates through MCP; whether a person ever opens an agent's window was
  not established.
- A DM who controls an actor keeps the panel for choosing that actor and its
  abilities, with no Say in it. Whether that panel then reads as complete
  was not checked in a browser.
- The server is unchanged: `mayVoice` in `internal/gateway/authz.go` already
  lets a DM and an agent name any speaker and speak out of character
  (SPEC-013).
