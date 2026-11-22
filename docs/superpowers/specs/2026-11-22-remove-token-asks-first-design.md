# The console's Remove token asks the DM first

## The problem

**Remove token sends at once.** The DM console's "Remove" beside Place
(`client/src/view/dm.ts`, action `remove-token`) sends `remove_token` for the
token id typed in the token field, with no question asked. The server appends a
`TokenRemoved` and the piece leaves the board. Placing the actor again is a new
token: the removed one's position, its name and notes, and its DM name and
notes (`Token` in `client/src/state.ts`) do not come back. A player whose
character saw the piece is sent that it went out of sight
(`internal/perceive/projection.go`), which their window draws no line for
(VTT-823).

**The console's Remove for an actor now asks** (VTT-950, SPEC-022), and so do
New link, Boot and Make spectator; Remove token, the other control that takes
something off the board, does not. (Deleting a note, removing a condition,
taking an ability and revoking control do not ask either; they are not this
ticket's.) The comment above it describes the control and carries history
("retraction-leaves Task 8, spec §5.1", "this branch's own Task 3"), which
`CLAUDE.md` rule 10 keeps out of `client/src`.

**The owner's ruling, 2026-11-22.** Told that Remove token asks nothing while
MapTool asks before deleting a token: "kör den".

**What MapTool does.** Deleting tokens asks by default:
`msg.confirm.deleteToken`, "Are you sure you want to delete the selected
tokens?", behind the preference `tokensWarnWhenDeleted` (`AppPreferences`,
default on), which the dialog lets the user switch off. Borrowed: asking before
a token is removed. Whether to borrow the switch is the plan's to put to the
owner; the actor's Remove borrowed none.

**What this is not.** A change to the server, to `remove_token` or to what a
removal does or who is told of it. A change to the actor's Remove or any other
control. A way for the agent, which removes through MCP, to be asked.

## Done looks like

1. **Remove token asks first.** Pressing it with a token id in the field asks
   the DM, in a message naming the token; when the DM declines, nothing is sent
   and the field keeps what was typed; when the DM accepts, `remove_token` is
   sent as today. A client test that fails on today's tree.
2. **An empty field still asks nothing.** Pressing it with the field empty or
   blank says "name the token to remove", asks nothing and sends nothing.
3. **The comment above it is a warning or a pointer.** No history and no
   description of the control remain in it.
4. **The gate.** `task check` is green, `cmd/vtt/webdist` is rebuilt from the
   changed client, and `task e2e` is green.

## Rules this puts on the system

- The console's Remove token asks the DM first, in a message naming the token,
  and sends nothing when the DM declines.

## What it touches

`client/src/view/dm.ts`, `client/test/dm-view.test.ts`, `docs/requirements.md`,
and `cmd/vtt/webdist` rebuilt.

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **How the question names the token.** A token carries a name and a DM name,
  either of which may be empty, and an actor; the token id is what was typed.
  Which the message shows, and in what order of fallback, was not decided.
- **The question's words.** "Nothing brings it back", the actor's Remove's
  phrase, is not quite true of a token, which can be placed again as a new one;
  what the question should say is lost (its place, its name and notes) was not
  decided.
- **Whether a press the console's lock holds back asks nothing**, as it does for
  the actor's Remove (VTT-950's clause), follows from where the question is
  placed; the plan decides.
