# A refusal tells the player why, in plain words

## The problem

**A refusal reaches the player in the server's own words.** `act` and
`switchTo` in `client/src/app.ts` show every refused command as the toast
`refused: ` followed by `CommandResult.error`, and the server fills that field
with text the refusing code wrote. Those texts are written for
whoever reads the code. A player's move into a wall they see reads
`refused: gateway: cannot move there — a wall` (`handleCommand` and
`describeBlockage` in `internal/gateway/server.go`); `gateway:` is a package.
A line in character from a party member with no name reads
`refused: gateway: not authorized: this party member has no name; ask the DM
for one, or speak out of character` (`mayVoice` in
`internal/gateway/authz.go`, wrapping `ErrUnauthorized`), and `mayWhisper`
refuses a whisper to the DM from one in the same form.

**A player who loses a character is told only by a log line, and not
always.** When the DM revokes a player's control, the gateway sends a
`WatchableChanged` (`watchable` in `internal/gateway/viewpoint.go`), and
`Session` in `client/src/session.ts` keeps its set and repaints. A browser
probe on main `8f933f8` showed two cases. When it was the player's last party
member, the seat stays on it (VTT-456), the window perches (`mayPerch` in
`client/src/app.ts`) and draws `renderPerch` in
`client/src/view/spectator.ts`, "Whose eyes" and "Perched on: Lera", and the
Story draws the `ActorControlRevoked` as "control of Lera taken back",
because the Story draws every event in the log the window holds
(`buildFeed` in `client/src/view/feed.ts`). When the player controlled two
and lost the one they watched, the seat moves to the other (VTT-340), the
lost one's chip leaves the panel, and the Story is the other's log, which in
the probe held no line about it: nothing in the window said why. Nothing
tells a player who reloads a perched window that they play no character.

The alpha test of 2026-10-22 found both: A-3, "refusals speak developer", and
A-7, "a player is not told why their window emptied"
(`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`). On 2026-11-02
the owner ruled the shape. "Servern, en text för alla": the server writes the
words where it refuses, one text for every reader, the agent's included; the
window shows them without `refused: ` before them. "Toast + rad i perch-vyn":
the window says "You no longer play Lera." when it happens, and the perch
tells a player who controls no party member that they play no character. The
question that ruling answered described the window as perching without
explanation; told of the probe above, the owner kept it ("Ja, beslutet
står").

The same day the owner drew the line this ticket stays inside. Sight,
movement, joining a table, and a character granted or taken back belong to
the board engine; the rules engine, which runs a game's rules on the board,
is separate and must take any game system. This ticket is the board
engine's. The refusals `rules.Resolve` in `internal/rules/resolve.go` gives
an ability keep their words: they name what a ruleset calls its attributes,
defenses and resources, which `ruleset.json` lists as bare keys (`str`,
`ac`, `flurry_uses`), so plain words for them wait on how the rules engine
consumes a ruleset.

MapTool keeps the sentences a user reads in a catalog of their own, plain and
in the second person ("You must be a GM to use \"/{0}\".",
`slash.mustBeGM` in its `i18n.properties`), apart from its exceptions. It
does not refuse a move into a wall with a message: the path stops at the wall.
No message about a token's changed ownership was found in that catalog.

## Done looks like

1. A player's move to a square they see that a wall blocks is refused with
   `You cannot move there: a wall.`, and every other reason the move gate
   gives for a square the player sees, a closed door or scenery among them,
   is in the same form. A gateway test names each, and fails on today's
   tree.
2. A player's move to a square they cannot see is refused with one plain text
   whatever stands on the square, and the tests that hold VTT-156 pass with
   that text in place of today's.
3. A player's line in character, and a whisper to the DM, from a party member
   with no name are refused in plain words with no package prefix. A gateway
   test names each.
4. The window shows a refused command's error with nothing put before it. A
   client test names it, and fails on today's tree, which shows `refused: `.
5. When a party member leaves the set a player controls, the window says
   "You no longer play" and that party member's name. A client test names
   it, and a browser test shows it: the DM revokes a player's characters one
   after the other, and the player's window says so after each.
6. The perch of a player who controls no party member says that they play no
   character and that the DM can give them one; a spectator's perch does not.
   A client test names both.
7. Every refusal outside items 1 to 3, every refusal of `rules.Resolve`
   among them, reads as it does today, and the tests that pin those texts
   pass unchanged.
8. `task check` exits 0 on the branch.

## Rules this puts on the system

- The move gate's refusal of a player's move, for a square they cannot see
  or one that blocks, and the refusal of a line or a whisper from a party
  member with no name, name no package, no id and no participant id.
- The window shows a refusal as the server words it, with nothing put before
  it.
- A player is told which party member left the set they control, when it
  leaves.
- A player who controls no party member is told so where they choose whose
  eyes to watch through.

VTT-156 stands as worded: a refusal for an unseen square is one text.

## What it touches

In this order, because the window shows what the server writes:

1. `internal/gateway/server.go` (`handleCommand`'s move gate) and
   `internal/gateway/authz.go` (`mayVoice`, `mayWhisper`), with their tests,
   among them `internal/gateway/server_test.go`,
   `internal/gateway/qa_move_gate_test.go`, `internal/gateway/watch_test.go`
   and `internal/gateway/seat_internal_test.go`.
2. `client/src/app.ts` (`act`, `switchTo`, and the toast on a lost party
   member), `client/src/session.ts` (the set it keeps),
   `client/src/view/spectator.ts` (`renderPerch`), their tests, and a
   browser test under `client/e2e/`.
3. `docs/requirements.md`: the rules above.

Two components. Whether to split them is asked at sign-off.

## Specifications this moves

docs/specifications/013-authorization.md
docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- What the plain text for an unseen square reads. VTT-156 holds that it is
  one text; its words are the plan's.
- What the toast says when one `WatchableChanged` takes two party members at
  once, and what it names for a party member whose name is empty. The plan's.
