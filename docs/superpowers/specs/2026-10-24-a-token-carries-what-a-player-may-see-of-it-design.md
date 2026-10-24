# A token carries what a player may see of a creature, and a player is sent no creature's name or statblock

## The problem

A token is a placement and nothing else. `PlaceToken` and `TokenPlaced` in
`contract/vtt/v1/` carry an id, a scene, an actor and a position, and so does
`engine.Token` in `internal/engine/state.go`. Everything that names or
describes a creature lives on its `Actor`. When a creature's token comes into
a character's sight, `Projection.introduce` in `internal/perceive/projection.go`
sends that character's log an `ActorAdded` of a copy of the whole actor, with
only its controllers cleared: its name, its attributes, its resources and its
module data. A player's window therefore holds what the DM holds of the
creature.
- `tokensOnScene` in `client/src/view/grid.ts` and `renderDisc` in
  `client/src/view/spectator.ts` draw the actor's name as the token's initial
  and its native hover title, and each resource as a chip ("6/6").
- `describe` in `client/src/view/spectator.ts` writes the Story line
  "actor Goblin Archer joined". It names most other events by a token or actor
  id ("tok-archer moved to 5,1"), and an event it has no wording for by its
  payload's name (`sceneSeen`, `actorControlGranted`, `tokenHidden`).
- `renderPlayerPanel` in `client/src/view/player.ts` labels each target button
  with the target actor's name.
- `classify` forwards every `ResourceChanged` of an actor the character saw,
  so a player is told each change to the creature's hp.

The owner ruled on 2026-10-22 (A-6, K-1 and B-5 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`):
- "servern ska inte skicka det": the server sends a player no non-party
  actor's name and no statblock.
- What the player is told instead is what the character perceives, "a sleeping
  creature" for example, shown when they hover over or right-click the token.
- That information lives on the token, with MapTool's split borrowed whole: a
  name and notes that players see, and a GM name and GM notes that only the GM
  sees.

On 2026-10-24 the owner answered four questions, each with the recommended
option:
- "Inget hp; utfallsraden står": no attributes, resources or module data
  reach a player, and so no hp. `AbilityUsed`'s `outcome_summary` is still sent
  as it is.
- "Ja, ett eget kommando": the DM and the agent may rewrite a placed token's
  four fields.
- "Ja, som i dag": a left click still moves (A-5).
- "En ticket, servern först".

Two more defects come with this ticket.
- **A-10.** A player's `use_ability` is authorized by `authorizeActorOwnership`
  in `internal/gateway/authz.go`, which asks only whether the player controls
  the caster. `rules.Resolve` in `internal/rules/resolve.go` then asks whether
  each target exists and is in range. So a player may aim at an actor no
  character of theirs sees. The answer tells them whether the id exists:
  `unknown target actor` for an id that does not, a range refusal or a
  resolved ability for one that does.
- **An unstable test.**
  `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody` in
  `internal/gateway/server_test.go` opens its own handle on the campaign file
  with `sql.Open("sqlite", campaign.LogPath(f.path))`, with no
  `busy_timeout`. It renames the `participants` table straight after the
  presence snapshot, while the server reads the same file. The rename then
  fails at once with `database is locked (5) (SQLITE_BUSY)`.
  - Measured on 2026-10-24: 6 of 1000 runs failed on `1e24277`, and 2 of 200
    on `f2dd819`.
  - With `?_pragma=busy_timeout(5000)` on that handle, given by
    `go test -overlay`, 0 of 1000 failed.
  - It stopped a pre-push hook the same day. The owner said "ta ticket 5 och
    rätta det instabila testet i den".

## Done looks like

1. `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody` opens its
   handle with `busy_timeout(5000)`, the timeout `internal/store` and
   `internal/identity` open theirs with.
   - With another handle holding a read transaction across the rename, the
     rename waits for the read and succeeds. Without the timeout it fails at
     once with `SQLITE_BUSY`. A probe shows both, recorded in the commit.
   - `go test -count=1000 -run
     '^TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody$'
     ./internal/gateway/` passes.
2. A token carries four more fields: a name and notes, which players see, and
   a GM name and GM notes, which only the DM and the agent see.
   - `place_token` sets them, and so does a placement in an adventure's or a
     map's scene.
   - `engine.Apply` and `client/src/fold.ts` both store them on the token.
   - None of the four is bounded in bytes. They are outside SPEC-018's first
     table on purpose, since neither of its two tests is met: only the DM and
     the agent write them. SPEC-018's second table names them, held by the
     WebSocket frame for the two commands and by nothing in a file.
   - The contract's round trip, the fold tests and the loader tests fail
     today.
3. A DM or agent command rewrites a placed token's four fields, and the event
   it appends folds in both folds.
   - A player's and a spectator's are refused by authorization.
   - One naming a token that does not exist is refused, and appends nothing.
   - The DM console offers it: a control per placed token with the four
     fields and a button that sends the command, built in
     `client/src/commands.ts` (the owner's answer of 2026-10-24, "Ja, en
     kontroll i konsolen"). So `COMMAND_SURFACE` in
     `client/test/command-surface.test.ts` declares the command with a
     surface.
   - The gateway, fold and console tests fail today, since no such command
     exists.
4. A player's or a spectator's character log is introduced to an actor that is
   not a party member with its id, its module and its kind, and with no name,
   no attributes, no resources and no module data.
   - A party member is introduced whole, as today.
   - The DM's log, which is the world, holds both whole.
   - A `perceive` test that fails today shows it, and so does a gateway test
     over a live connection: a player whose character sees a creature is sent
     no frame carrying its name or any of its resources.
5. A character log is sent no `ResourceChanged` of an actor that is not a
   party member, whether its character sees the actor or not.
   - A party member's is sent as today, while seen.
   - A `perceive` test fails today.
6. A token arrives in a character log with its name and notes, and without
   its GM name and GM notes.
   - A rewrite of a token in the character's sight is sent the same way, and
     only when its name or notes changed.
   - A rewrite of a token out of sight is not sent: the token arrives with its
     present name and notes when it next comes into sight.
   - `perceive` tests fail today.
7. A creature a character log holds does not arrive again at each event.
   - While the creature stays in sight, a run of events sends no
     `ActorRemoved` or `ActorAdded` of it after its introduction.
   - A creature a grant makes a party member while it is in sight arrives in
     that log with its name and statblock.
   - A party member a grant makes a non-party actor while in sight is held
     without them from then on.
   - `perceive` tests; the second and third fail today.
8. A player's `use_ability` is refused, and appends nothing, when any target
   it names is an actor the eyes that judge the player's moves do not see
   (`seat.judgedBy`). The answer is exactly the one an id naming no actor gets.
   - A target in range behind a wall is refused this way. Today it is
     resolved.
   - A target the character sees, in range, is resolved as today.
   - The DM's and the agent's abilities are not judged by sight.
   - Gateway tests fail today.
9. In the browser, hovering over a token shows a card of what the window's
   log holds of it. A right-click shows the same card, and a left click
   still moves as it does today (A-5, "Ja, som i dag").
   - A player and a spectator see the token's name and notes.
   - The DM also sees its GM name and GM notes.
   - Tests in `client/test/` fail today.
10. The browser names an actor, to a window that is not the DM's, by what
    that window may know of it. That is the token's name, or else the actor's
    name for a party member, or else a fixed wording for an unnamed creature,
    and never an id. This holds for the token's initial, its card, the player
    panel's target buttons and the Story.
    - The DM's window names a creature by its GM name, or else its actor's
      name.
    - Tests in `client/test/` fail today.
11. No Story line is a payload's name or an id, and an actor's arrival reads
    as something seen, not as someone joining the table (A-2).
    - A test over every `Envelope` payload case fails today on
      "actor … joined" and on the cases `describe` has no wording for.
    - An ability's line shows the ability, each target by the window's
      naming, its result and what was rolled, and never the defense it was
      rolled against. The owner's answer of 2026-10-24 was "man visar att det
      har blivit träff, men inte 'varför' … bara longsword strike hit (dice:
      17) typ. vad man slog, men inte att det var för att goblin hade 15 i
      ac".
    - The frame still carries `outcome_summary` whole ("utfallsraden står").
      What changes is what the window shows of it.
    - A client test fails today, where the line is "<actor id> used <ability
      id>".
12. `adventures/goblin-ambush`, the one shipped adventure with non-party
    actors, gives each placement of one a name a player may see and a GM
    name. `adventures/cellar-rats` places only party members and is left
    as it is.
    - Its golden is updated by hand and checked by the adventure conformance
      runner.
    - `client/e2e/table.spec.ts`'s "a player's ability reaches the creature
      they aim it at" aims at the creature by its token's name.
    - While the player's page is open it records every frame received, and no
      frame carries the creature's actor name or hp.
    - It fails on today's bundle.
13. `place_token`'s tool description and schema in `tools/toolgen/main.go`
    carry the four fields, and a tool exists for the rewrite.
    `use_ability`'s description says a player's target must be seen.
    `check:drift` stays green with the regenerated `tools.json`, and
    `TestToolsMatchGolden` with `contract/testdata/expected_tools.json`.
14. The existing tests whose expectation is today's rule change to the new
    one, and keep what they were written to hold.
    - These are `perceive` tests that compare an introduced creature with the
      world's actor, `internal/campaign` tests of a character log's pause and
      reopen, gateway tests that read a creature's name or hp from a player's
      stream, client tests of `describe` and of the token's title, the e2e
      test in item 12, and `table.spec.ts`'s "the story feed and ticker record
      what happened", which reads `/^tok-lera placed at/`.
    - The plan names them from a prototype run.

## Rules this puts on the system

- A token carries a name and notes that players and spectators may be sent,
  and a GM name and GM notes that only the DM and the agent are sent.
- Only the DM and the agent may rewrite a placed token's name, notes, GM name
  and GM notes.
- The DM console offers the rewrite for each placed token.
- A character log is sent an actor that is not a party member without its
  name, attributes, resources or module data.
- A character log is sent no resource change of an actor that is not a party
  member.
- A character log is sent a rewrite of a token only while the token is in
  its sight, and only when the name or notes changed.
- A player's ability is refused, with the answer an unknown target gets, for a
  target the eyes judging the player's moves do not see.
- The browser shows, on hover and on right-click, a token's name and notes,
  and to the DM its GM name and GM notes.
- The browser names an actor to a window that is not the DM's by its token's
  name, or a party member's actor name, and never by an id.
- No Story line is a payload's name or an id.
- The Story shows an ability's result and what was rolled, never the defense
  it was rolled against.

The unstable test puts no rule on the system.

Existing rows this makes false as worded, each withdrawn and succeeded, never
edited:
- VTT-248: "An actor that comes into a viewer's sight … is brought in its fold
  to its present resources, conditions, controllers and kind." A creature has
  no resources in a character log.
- VTT-313: an actor added again is introduced afresh when the viewer "holds
  one that differs in a field only ActorAdded writes (name, module id,
  attributes, module data, resource names or maxima)". A character log holds
  none of those of a creature but its module id.
- VTT-247: a fold holds each unseen actor "at the status the actor had after
  the last event at which the viewer saw it". A creature's status in a
  character log has no resources.
- VTT-440: "The player panel labels a target button with the name of the
  actor its token shows, when that actor has one." The button is now labelled
  by the window's naming, which puts the token's name first.

The plan confirms these by a search of `docs/requirements.md` and names any
others.

## What it touches

In this order:

1. `internal/gateway/server_test.go`, for the unstable test alone.
2. `contract/vtt/v1/commands.proto` and `contract/vtt/v1/events.proto` (the
   four fields on `PlaceToken` and `TokenPlaced`, and the rewrite's command and
   event), and what `task generate:contract` regenerates from them:
   `contract/gen/`, `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`;
   `contract/testdata/`; `tools/toolgen/main.go`;
   `contract/testdata/expected_tools.json`.
3. `internal/engine/state.go` and `internal/engine/apply.go`, with their
   tests; `client/src/fold.ts` and `client/src/state.ts`; the gateway's
   conversion and authorization (`internal/gateway/convert.go`,
   `internal/gateway/authz.go`); `internal/eventgen/model.go`, which generates
   `TokenPlaced` for the folds' parity.
4. `internal/adventure/format.go`, `load.go` and `compile.go`, and
   `internal/mapdef/format.go`, `load.go` and `compile.go`, with their tests
   and testdata; `docs/map-format.md` §6; `adventures/goblin-ambush`, with
   its golden.
5. `internal/perceive/projection.go` (`introduce`, `sameIncarnation`,
   `correct`, `statusFrames`, `classify`, `transitions`, `forwardable`), with
   the tests in `internal/perceive/`.
6. `internal/gateway/server.go` (`handleCommand`, for A-10), with gateway
   tests; `internal/rules/resolve.go`, which writes each target's result and
   roll beside `outcome_summary` for item 11, with its tests and the
   scenario streams under `scenarios/goldens/` whose recorded ability has a
   resolution.
7. `client/src/view/spectator.ts` (`describe`, `renderDisc`),
   `client/src/view/grid.ts`, `client/src/view/player.ts`, a new naming
   module under `client/src/view/`, `client/src/app.ts`,
   `client/src/style.css`,
   `client/src/view/dm.ts` and `client/src/commands.ts` (the console's
   rewrite control), with tests in `client/test/`, `COMMAND_SURFACE` in
   `client/test/command-surface.test.ts` among them;
   `client/e2e/table.spec.ts`; the bundle `cmd/vtt/webdist/assets/index.js`.
8. `docs/requirements.md`.

This crosses seven components (contract and its generated tools, engine,
loaders, projection, gateway, browser client, shipped adventures) and names far
more than ten files. Asked whether to split it, the owner answered on
2026-10-24 "En ticket, servern först": one branch, its tasks in the order
above. The console's place-token form in `client/src/view/dm.ts` keeps its
fields. A human DM names a token with the rewrite control after placing it.

## Specifications this moves

docs/specifications/016-the-projection.md
docs/specifications/007-the-wire-contract.md
docs/specifications/013-authorization.md
docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
docs/specifications/014-loading-a-map.md
docs/specifications/022-a-client-holds-one-log.md
docs/specifications/021-each-party-member-keeps-its-own-log.md

## What could not be established

- **The defense in the frame.** `rules.Resolve` writes `outcome_summary` as
  "<ability> on <target id>: <branch> (<total> vs <defense>)". The owner chose
  to send it as it is ("utfallsraden står"), so a player's frames carry the
  creature's defense, though item 11 keeps the window from showing it. How
  the window finds the result and the roll is settled under "What it
  touches": from fields `rules.Resolve` writes beside that text, not from the
  text itself.
- **Ids on the wire.** Actor and token ids are the author's, "act-archer" and
  "tok-archer" in `adventures/goblin-ambush`, and every frame about a creature
  carries them. Item 10 keeps the window from showing one. Keeping them out of
  the frames is not part of this ticket.
- **A kind change while in sight.** Re-sending an actor whose shape changes
  needs a bare `ActorRemoved` first. Both folds accept that only while no token
  of the actor is on the board, and `transitions` hides tokens after its actor
  walk today. Whether item 7's second and third cases need the token hidden
  and placed again around the re-introduction has not been tried.
