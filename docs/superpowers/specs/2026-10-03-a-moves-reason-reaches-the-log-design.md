# A move's reason reaches the log

## The problem

`MoveTokenRequest` in `contract/vtt/v1/commands.proto` carries
`optional string reason`, which its comment says is "shown in the log"; the
MCP `move_token` tool offers it to an agent (`contract/gen/tools/tools.json`),
`client/src/commands.ts`'s `moveToken` sends it when given one, and
`commandRoles` in `internal/gateway/authz.go` lets the DM, an agent and a
player issue `move_token`. `TokenMoved` in `contract/vtt/v1/events.proto` has
no field for it (`token_id`, `scene_id`, `from`, `to`), so `ToEvent` in
`internal/gateway/convert.go` builds the event without it, `campaign.Append`
appends it, and the command answers ok=true: the reason is lost without a
word. No test sends a reason through `ToEvent`; `internal/mcp/tools_test.go`
sets one against a fake server and `contract/testdata/client_command.json`
round-trips one, and neither reaches the gateway. SPEC-013 states the drop
("`move_token`'s `reason` has none in `TokenMoved` and is dropped"), VTT-257
is worded around it ("every field the command gave that the event has a field
for"), and `docs/verification-debt.md` carries it as open debt. Were the field
added and nothing else changed, `Projector.Project` would forward the whole
`TokenMoved` envelope to every player and spectator `classify` lets see the
move (one who held the token before and sees it after), so a DM's or agent's
annotation would reach them word for word. The client's feed and ticker label
a move with `describe` in `client/src/view/spectator.ts`, `"<token> moved to
x,y"`, and show nothing else of it.

## Done looks like

1. A `move_token` that carries a reason appends a `TokenMoved` whose reason is
   the command's: a named test under `internal/gateway/` that sends one
   through `handleCommand` and reads the appended event fails on today's tree
   and passes after. A move with no reason appends a `TokenMoved` with none,
   and every golden stream under `scenarios/goldens/` is byte-identical, since
   no scenario gives a move a reason.
2. What a player or spectator is sent of a move with a reason follows the
   owner's ruling at sign-off, and a named test under `internal/gateway/`
   observes it for a viewer that sees the move; the DM and the agent receive
   the event unchanged (`TestTheDMReceivesEverythingUnchanged` still passes).
3. The client's feed and ticker show a move's reason when the frame carries
   one: a named test under `client/test/` that renders such a frame fails on
   today's tree and passes after.
4. The contract change is additive: `TokenMoved` gains one field under a new
   number and nothing else in `contract/` changes shape; `task
   check:breaking` reports no breaking change; the generated code is
   regenerated with `task generate:contract` and `task check:drift` is clean.
5. `TestToEventMoveTokenProducesTokenMoved` gives a reason and asserts it on
   the event; VTT-257 no longer needs "that the event has a field for" to be
   true; SPEC-013 no longer says the reason is dropped;
   `docs/verification-debt.md`'s entry is closed by the test that holds it.
6. `task check` whole is green.

## Rules this puts on the system

- A move's reason reaches the event the move appends.
- A move with no reason records none.
- A player or spectator is sent a move's reason only as the owner rules.
- The client shows a move's reason where it labels the move.

## What it touches

1. `contract/vtt/v1/events.proto` (`TokenMoved`) and the comment on
   `MoveTokenRequest.reason` in `contract/vtt/v1/commands.proto`,
   `contract/gen/` (regenerated), `contract/testdata/` (an event fixture with
   a reason), `contract/roundtrip_test.go`, `contract/events.test.ts`
2. `internal/gateway/convert.go` (`ToEvent`), `internal/gateway/project.go`
   (`Projector.Project`, `classify`, for what a projected seat is sent), and
   their tests (`convert_test.go`, `project_test.go`, `server_test.go`,
   `server_visibility_test.go`); `internal/eventgen/model.go` and
   `internal/gateway/project_property_test.go`, for the property walk;
   `cmd/vtt/scenario_goldens_test.go`, a citation line
3. `client/src/view/spectator.ts` (`describe`) and a test under
   `client/test/`; `cmd/vtt/webdist` (rebuilt)
4. `docs/specifications/013-authorization.md`,
   `docs/specifications/016-the-projection.md`; `docs/requirements.md`;
   `docs/verification-debt.md`; `tools/comment-ceilings.txt` if a share moves

Three components, in this order: the contract first, then the gateway, then
the client. It crosses more than two components; it is one rule end to end,
and a contract field no layer writes or shows would be the defect this
ticket closes.

## Specifications this moves

docs/specifications/013-authorization.md
docs/specifications/016-the-projection.md

## What could not be established

- Who is shown a move's reason. The DM and the agent read the log, so they
  are. A player or spectator who sees the move may be sent the reason, or the
  move without it; the reason is free text a DM or agent wrote and can name
  what the viewer does not see, the case the owner's ruling of 2026-09-29
  ("only what a viewer sees gives it information") was made for. The owner
  rules at sign-off; the recommendation is that a projected seat's
  `TokenMoved` carries no reason.
- Whether a player may give a reason at all. `commandRoles` lets a player
  move a token, and nothing refuses a reason from one; the proto calls it a
  "DM/agent annotation". The plan proposes; the owner rules.
- A forwarded `ResourceChanged` already carries its `reason` to every viewer
  who sees the actor (the 2026-10-01 testimony report's open question on a
  forwarded change's reason and author). This ticket does not change it; a
  ruling here may answer it too, and the plan says whether it does.
