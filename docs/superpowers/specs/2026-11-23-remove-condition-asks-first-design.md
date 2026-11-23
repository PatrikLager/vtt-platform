# The console's Remove condition asks the DM first

## The problem

**Remove condition sends at once.** The DM console's "Remove condition" group
(`renderDMConsole` in `client/src/view/dm.ts`, action `remove-condition`)
draws one button per condition each actor of the world holds, labelled the
actor's name in the fold, else its id, `: `, the condition as
`Namer.condition` names it, and ` ✕` (VTT-864). A press sends
`remove_condition` naming the actor's and the condition's ids, with no
question asked. The server appends a `ConditionRemoved` (a condition the actor
does not hold is refused, `engine.Apply` in `internal/engine/apply.go`), and
the client's fold drops it from `Conditions` (`conditionRemoved` in
`client/src/fold.ts`). Nothing in the console puts a condition back: no
control of it sends a command that applies one, and the only command that does
is `use_ability`, whose outcome a ruleset declares, sent from the player panel
of a window that offers the ability, a DM's included, or by the agent. The
buttons sit side by side, one per condition, so a press on the wrong one takes
away a condition the DM cannot give back.

**Remove condition does not send through `sending`.** Each button calls
`d.send` directly. Unlike Remove actor, Remove token and Delete note, a press
while a `remove_condition` it sent is unanswered sends another, and nothing in
the console catches a send whose promise rejects. The rows that name the
controls `sending` holds, VTT-523, VTT-524, VTT-526 and VTT-538, do not name
it.

**The console's other removals.** Remove actor (VTT-950), Remove token
(VTT-952) and Delete note (VTT-956) ask first, and so do New link, Boot and
Make spectator. Taking an ability and revoking control do not ask; they are
not this ticket's.

**The owner's ruling, 2026-11-23.** Told that Remove condition does not ask
and that a removed condition cannot be put back from the console: "ja, kan
vi göra det här också, alltså condition, eftersom det är likadan logik?"

**What MapTool does.** A token's state is set and cleared from its popup
menu (`ChangeStateAction` in `TokenPopupMenu`), a checkbox per state, with
no question asked; `i18n.properties` holds no confirmation for a state. Its
answer does not fit: the same menu puts a cleared state back with one press,
and the console has no such control. Borrowed: nothing.

**What this is not.** A change to the server, to `remove_condition` or to
what a removal does. A control that applies a condition. A change to any
other control. A way for the agent, which removes conditions through MCP, to
be asked.

## Done looks like

1. **Remove condition asks first.** Pressing a button of the "Remove
   condition" group asks the DM, in a message naming the actor and the
   condition as the button does; when the DM declines, nothing is sent; when
   the DM accepts, one `remove_condition` naming the actor's and the
   condition's ids is sent, as today. A client test that fails on today's
   tree.
2. **An unanswered removal holds its button back.** While a
   `remove_condition` a button sent is unanswered, pressing that button again
   asks nothing and sends nothing; once that command is answered it sends
   again. A send whose promise rejects frees it and is reported with
   `console.error`. Client tests that fail on today's tree.
3. **The browser test still removes a condition.**
   `client/e2e/conditions.spec.ts`'s test presses the button, accepts the
   question and still sees `Dummy lost …` in the feed and the dot gone.
4. **The gate.** `task check` is green, `cmd/vtt/webdist` is rebuilt from the
   changed client, and `task e2e` is green.

## Rules this puts on the system

- A button of the console's "Remove condition" group asks the DM first, in a
  message naming the actor and the condition, and sends nothing when the DM
  declines.
- While a `remove_condition` a button of that group sent is unanswered,
  pressing it again sends nothing.
- A `remove_condition` a button of that group sent whose promise rejects
  frees it and is reported with `console.error`.

## What it touches

`client/src/view/dm.ts`, `client/test/dm-view.test.ts`,
`client/e2e/conditions.spec.ts`, `docs/requirements.md`, and
`cmd/vtt/webdist` rebuilt.

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **The question's words.** That the condition goes, and whether it says
  that nothing in the console puts it back or that only an ability does, was
  not decided.
- **Which presses one lock holds back.** Whether a button's unanswered
  removal holds back that button alone, the group's other buttons too, or
  the same condition on another actor, was not decided.
- **How VTT-864 and the rows that list `sending`'s controls take the
  question in.** VTT-864 says a button sends `remove_condition`; after this
  it sends on a yes. VTT-523, VTT-524, VTT-526 and VTT-538 name the controls
  by list. Whether these are withdrawn and succeeded, or the removal earns
  rows of its own, is the plan's to decide.
