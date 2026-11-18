# A condition is named in the ruleset's words wherever a window shows it

## The problem

**A condition is shown by its id.** Every window draws a disc's conditions
as dots (`tokensOnScene` in `client/src/view/grid.ts`, which gives each
`ConditionDot` the condition's id alone), and the board titles each dot by
that id (`dot.title = c.id` in `client/src/view/spectator.ts`). The Story and
the Events panel write a condition's line through `describe`
(`client/src/view/spectator.ts`): `<actor> gained <id>` and
`<actor> lost <id>`. The DM console's "Remove condition" group
(`client/src/view/dm.ts`) labels each button `<actor id>: <condition id> ✕`,
naming the actor by its id as well. A ruleset declares each condition's
name and description, `/api/ruleset` serves them to every role, and the
window keeps them from its newest answer (`layout.conditions` in
`client/src/app.ts`); only the player panel reads them, naming a condition
by its name, else its id, and titling it by its description (VTT-842).
`Namer` (`client/src/view/naming.ts`) names actors, tokens, abilities and
resources,
and no condition. A player hovering a dot reads `dazed-by-ale`, and the
Story tells the table `Lera gained dazed-by-ale`. Blueprint principle 13
says no message a participant receives shows an identifier where a name
exists.

**The owner's rulings of 2026-11-18.** Every place names a condition:
a disc's dot, the Story's and the Events panel's lines, and the DM
console's remove button, whose actor is named as the console's ability rows
name an actor. A dot's hover shows the condition's name and, after a line
break, its description, as an ability button's hover shows an ability's.

**What MapTool does.** In MapTool's core a token state is identified by its
name (`AbstractTokenOverlay`), so the word shown is the name. The owner's
4.5e framework under `~/dev/RPTool/rule-framework` gives a condition an `id`
and a `label` (`schema/condition.schema.ts`), and its scripts print the
label, else the id, when a condition is applied
(`library/mtscript/public/lib/RE_applyCondition.mts`) and in the
remove-condition picker (`library/mtscript/public/removeCondition.mts`).
Borrowed: the ruleset's name, else the id, wherever a condition is shown.

**What this is not.** The agent's tools, which answer condition ids as data
and `get_ruleset` their names. A condition's dot colour or icon. Any other
id the DM console shows. The wire: no field is added to the contract or to
`/api/ruleset`.

## Done looks like

1. **A dot names its condition.** In every window, a disc's dot for a
   condition the window's ruleset list declares is titled by that
   condition's name, else by its id when the name is empty, and, after a
   line break, its description when it is non-empty; a dot for a condition
   the list does not declare, or when the window holds no list, is titled
   by its id as today. A client test that fails on today's tree.
2. **The Story and the Events panel name it.** A `conditionApplied` and a
   `conditionRemoved` line names the condition by the name the window's
   ruleset list gives it, else by its id. A client test that fails on
   today's tree; the tests that read `gained prone` with no list stay
   green.
3. **The DM console names both.** A "Remove condition" button names the
   actor by its name, else its id, and the condition by its name, else its
   id; it still sends `remove_condition` with the ids. A client test that
   fails on today's tree.
4. **The newest answer names it.** The names come from the window's newest
   `/api/ruleset` answer, as the panel's do (VTT-845); an older answer
   arriving later changes none of them. A client test.
5. **The gate.** `task check` is green, the embedded bundle under
   `cmd/vtt/webdist` is rebuilt from the changed client, and `task e2e` is
   green.

## Rules this puts on the system

- A disc's condition dot is titled by the condition's name from the
  window's ruleset list, else by its id when the name is empty, and, after
  a line break, its description when it is non-empty; by its id alone when
  the list does not declare it.
- A `conditionApplied` or `conditionRemoved` line in the Story and the
  Events panel names the condition by its name in the window's ruleset
  list, else by its id.
- The DM console's remove-condition button names the actor by its name,
  else its id, and the condition by its name, else its id, and sends
  `remove_condition` with the actor's and the condition's ids.
- The condition list that names a condition is the window's newest
  `/api/ruleset` answer's.

## What it touches

In this order:

1. **The namer.** `client/src/view/naming.ts` (`Namer`, `namer`): a
   condition's name, else its id.
2. **The board and the lines.** `client/src/view/grid.ts` (`ConditionDot`,
   `tokensOnScene`), `client/src/view/spectator.ts` (`describe`, the dot's
   title, `ViewExtras`), and their tests (`client/test/grid.test.ts`,
   `client/test/spectator-view.test.ts`).
3. **The console.** `client/src/view/dm.ts` (`DMDeps`, the "Remove
   condition" group) and `client/test/dm-view.test.ts`.
4. **The window.** `client/src/app.ts` (handing the newest answer's
   conditions to the board, the lines and the console),
   `client/test/app.test.ts`, and `cmd/vtt/webdist`.
5. `docs/requirements.md`: the rows whose words change, as the plan
   decides.

This stays within the browser client: one component.

## Specifications this moves

- docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **A condition the list does not declare.** `rules.Resolve` and
  `perceive`'s projection are what write `ConditionApplied`, so a
  condition id the window's ruleset list does not declare reaches a window
  only from a log written under another ruleset's conditions; it is shown
  by its id, as the panel shows it today.
- **The real wire.** No spec under `client/e2e` applies or reads a
  condition, and a condition an ability applies depends on a roll's
  outcome; whether an e2e check is added, and how, is the plan's.
- **A long description in a dot's title**, and touch platforms that draw no
  title, as for an ability's button.
