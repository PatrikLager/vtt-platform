# The console's Delete note asks the DM first

## The problem

**Delete note sends at once.** The DM console's Notes group
(`client/src/view/dm.ts`, action `delete-note`) has a Delete beside Save.
Pressed with a key in `note-key`, it sends `delete_note` for the trimmed key,
with no question asked; pressed with the field empty or white space, it shows
`name the note to delete` and sends nothing. The server appends a
`NoteDeleted` (a key no note holds is refused, `engine.Apply` in
`internal/engine/apply.go`), and the client's fold drops the note from
`Notes` (`noteDeleted` in `client/src/fold.ts`). Its title and text stay in
the log, but nothing in the client shows them again: the DM writes them
anew. Save sits beside it and reads the same `note-key`, so a press on the
wrong button of the pair, or a wrong key, takes away text the DM wrote.

**Delete note does not send through `sending`.** It calls `d.send` directly.
Unlike Save, Remove actor and Remove token, a press while a `delete_note` it
sent is unanswered sends another, and a send whose promise rejects is caught
by nothing (why VTT-525 was withdrawn). The rows that name the controls
`sending` holds, VTT-523, VTT-524, VTT-526 and VTT-538, do not name it.

**The console's other removals.** Remove actor (VTT-950) and Remove token
(VTT-952) ask first, and so do New link, Boot and Make spectator. Removing a
condition, taking an ability and revoking control do not ask either; they
are not this ticket's.

**The owner's ruling, 2026-11-23.** Told that Delete note is the one console
removal that takes away something the DM wrote, and that it could ask first,
naming the note, and send through `sending`: "ok, kör den".

**What MapTool does.** MapTool has no campaign notes. Two of its named,
user-written objects ask before they are deleted, naming the object: a lookup
table, `Delete table "{0}"?` (`LookupTablePanel.confirm.delete`), and a
macro, `You are about to delete the {0} macro.` (`confirm.macro.delete`), in
`maptool`'s `i18n.properties`. Borrowed: asking before a named piece of
writing is deleted, and naming it.

**What this is not.** A change to the server, to `delete_note` or to what a
deletion does. A change to Save or to any other control. A way for the agent,
which deletes notes through MCP's `delete_note`, to be asked.

## Done looks like

1. **Delete note asks first.** Pressing it with a key in `note-key` asks the
   DM, in a message naming the note; when the DM declines, nothing is sent and
   the key field keeps what was typed; when the DM accepts, one `delete_note`
   naming the trimmed key is sent, as today. A client test that fails on
   today's tree.
2. **A blank key still asks nothing.** Pressing it with `note-key` empty or
   white space shows `name the note to delete`, asks nothing and sends
   nothing.
3. **An unanswered deletion holds Delete note back.** While a `delete_note`
   it sent is unanswered, pressing it again asks nothing and sends nothing;
   once that command is answered it sends again. A send whose promise rejects
   frees it and is reported with `console.error`. Client tests that fail on
   today's tree.
4. **The gate.** `task check` is green, `cmd/vtt/webdist` is rebuilt from the
   changed client, and `task e2e` is green.

## Rules this puts on the system

- The console's Delete note asks the DM first, in a message naming the note,
  and sends nothing when the DM declines.
- While a `delete_note` the console's Delete note sent is unanswered,
  pressing it again sends nothing.
- A `delete_note` the console's Delete note sent whose promise rejects frees
  it and is reported with `console.error`.

## What it touches

`client/src/view/dm.ts`, `client/test/dm-view.test.ts`, `docs/requirements.md`,
and `cmd/vtt/webdist` rebuilt.

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **How the question names the note.** A note has a key, which is what was
  typed, and a title, which may be empty; the console's state holds the DM's
  notes in `Notes`. Which the message shows, and in what order of fallback,
  was not decided.
- **The question's words.** What it says is lost, and whether it says that
  nothing brings it back, was not decided.
- **How the rows that list `sending`'s controls take Delete note in.**
  VTT-523, VTT-524, VTT-526 and VTT-538 name the controls by list; whether
  they are withdrawn and succeeded with Delete note in their lists, or Delete
  note earns rows of its own, is the plan's to decide.
