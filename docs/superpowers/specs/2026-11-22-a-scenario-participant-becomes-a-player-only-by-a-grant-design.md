# A scenario invites a participant as the server does, names a person, and makes a player only by a grant

## The problem

**A scenario declares a role the server does not mint.** A scenario file under
`scenarios/` lists its participants, each with a `name` and a `role`
(`Participant` in `internal/harness/scenario.go`). Every scenario there but
`smoke.json` declares a participant `"role": "player"`. The server mints no
player: `CreateInvite` in `internal/identity/identity.go` refuses that role
with `identity: a participant becomes a player when given a character; invite
them as a spectator` (VTT-911), because a participant who is neither the DM
nor the agent is a player exactly while they control an actor (SPEC-009).
`mintInvites` in `cmd/vtt/harness_boot.go` turns a scenario's `player` into
`spectator` before it calls `CreateInvite`, and `parseScenario` in
`internal/harness/scenario.go` checks no participant's role. So the file says
`player` where the server mints a spectator, and the participant becomes a
player only if a `grantActorControl` step grants it an actor. `SoakParticipants`
in `internal/harness/soak.go` declares the soak's two players the same way and
reaches the same rewrite through `cmd/vtt/client_soak.go`.

**A participant is named for a role.** `a-player-switches-and-reconnects.json`,
`adventure-night.json`, `denials.json`, `session-zero.json`, `story-table.json`
and `three-role-exit.json` name a participant `player`; `adventure-night.json`,
`denials.json`, `goblin-fight.json`, `session-zero.json`, `story-table.json`,
`three-role-exit.json` and `toy-brawl.json` name one `spectator`; the soak
roster names `player1` and `player2`. A participant's name is its display
name and the seed of its id in the goldens under `scenarios/goldens/`
(`p-player`). Read as a role, it is wrong whenever a grant has not happened
yet or a revocation has: a participant named `player` is a spectator until
granted, and one named `spectator` would be a player once granted.

**Two scenarios test a player that never is one.** `adventure-night.json`'s
`player` is granted no actor, so its `load_adventure`, refused as `not
authorized`, is a spectator's refusal and repeats the scenario's spectator's
own. `denials.json`'s `player` sends `load_map`, `add_actor`, `place_token`,
`start_session`, `end_session` and `move_token`, each refused as `not
authorized`, before the scenario's only grant to it, which is itself refused
(`must say what the actor IS`); so those are a spectator's refusals too, and
the scenario's spectator repeats several of them. Neither scenario tests a
player's refusal.

**The owner's rulings, 2026-11-22.** Asked why a participant is named
`player` when connecting makes a spectator and a grant makes a player: "Varför
inte ge de två anslutna namn istället, så man fattar vad detta handlar om?"
Offered three changes, the participants named for a person, the scenario format
refusing the role `player`, and `adventure-night.json` granting its participant
an actor before it acts as a player: "kör alla tre", as one ticket.

**What MapTool does.** `Player.Role` in
`maptool/src/main/java/net/rptools/maptool/model/player/Player.java` is
`PLAYER` or `GM`, chosen when a client connects, and token ownership is held
apart from it. Here the role is derived from control (SPEC-009), so its model
does not fit; and it has no scripted run of several connections to compare the
scenario format with.

**What this is not.** A change to the server, to `CreateInvite`, to the roles
or to what a grant does. A change to Go tests that build a participant in
their own code rather than read a scenario file (`internal/harness/engine_test.go`,
`internal/gateway/scenario_test.go`). A rename of a scenario file: a file's name
says what it tests. A change to a participant whose name is not a role's
(`fighter`, `brawler`, `patron`, `watcher`, `latecomer`, `ana`, `bo`, `cy`,
`dee`), or to the participants named `dm` and `agent`, whose roles are minted
and never change.

## Done looks like

1. **A scenario declaring a player is refused when loaded.** `LoadScenario` on
   a scenario with a participant whose role is `player` fails, naming the
   participant and saying it is invited as a spectator; a scenario whose
   participants are `dm`, `agent` and `spectator` loads as today. A harness
   test that fails on today's tree.
2. **The harness mints what the scenario declares.** `mintInvites` hands
   `CreateInvite` each participant's role unchanged, so a scenario built in
   code with a `player` participant fails to mint with `CreateInvite`'s
   refusal. A test that fails on today's tree.
3. **Every scenario and the soak roster declare a role the server mints.**
   `git grep -n -E '"role": *"player"' -- scenarios` prints nothing, and
   `SoakParticipants` declares no `player`. The scenario library and the soak
   still run green (`cmd/vtt/library_test.go`, `cmd/vtt/scenario_goldens_test.go`,
   the soak's tests).
4. **No participant is named for a role a grant changes.** `git grep -n -E
   '"name": *"(player|spectator)"' -- scenarios` prints nothing, and the soak
   roster names no `player1` or `player2`. The goldens are re-derived for the
   new names and `TestScenarioGoldenStreamsHaveNotDrifted` passes.
5. **The two scenarios test a player.** In `adventure-night.json` and
   `denials.json`, the participant whose refusals stand for a player's holds an
   actor, granted by a `grantActorControl` step that is accepted, before its
   first refused command, and the goldens hold that grant. A check that fails
   on today's `adventure-night.json` and `denials.json`.
6. **The gate.** `task check` is green.

## Rules this puts on the system

- A scenario declares each participant with a role the server mints, and a
  scenario declaring a participant `player` is refused when loaded.
- The harness mints each scenario participant the role the scenario declares,
  unchanged.
- A scenario names no participant for a role a grant or a revocation changes.
- A scenario step standing for a player's refusal is sent by a participant
  holding an actor.

## What it touches

In this order:

1. **The format.** `internal/harness/scenario.go` (`parseScenario`,
   `Participant`), `internal/harness/scenario_test.go`, and
   `internal/harness/engine.go` (`characterSeats`, which names `player`).
2. **The mint and the soak roster.** `cmd/vtt/harness_boot.go` (`mintInvites`),
   `internal/harness/soak.go` (`SoakParticipants`, `soakPlayer1`,
   `soakPlayer2`), `cmd/vtt/client_soak.go`, and the tests that read the
   soak's names (`internal/harness/qa_log_seat_test.go`,
   `internal/harness/qa_soak_log_test.go`) or a role `player`
   (`internal/harness/qa_one_log_test.go`).
3. **The library.** Every scenario under `scenarios/` that declares a player
   or names a participant `player` or `spectator`; their goldens under
   `scenarios/goldens/` (`stream.json`, `state.json`, `projections/`);
   `scenarios/goldens/README.md`.
4. **The readers of the goldens.** `cmd/vtt/scenario_goldens_test.go`,
   `cmd/vtt/library_test.go`, `cmd/vtt/client_e2e_test.go`, and the client tests
   that read the goldens (`client/test/fold-unit.test.ts`,
   `client/test/fold-parity.test.ts`, `client/test/removal-batch-parity.test.ts`,
   `client/test/visibility.test.ts`).

This crosses more than two components and names more than ten files; the
owner ruled the three changes one ticket (2026-11-22).

## Specifications this moves

docs/specifications/009-identity-and-joining.md

## What could not be established

- **What text a player's refusal carries in `denials.json`.** A player sending
  `move_token` for a token it does not control may be refused with a text other
  than `not authorized`; the expectation each refused step carries after the
  grant was not traced.
- **Whether a grant before `adventure-night.json`'s first player step moves
  its recorded dice.** Its goldens rest on recorded rolls (its `stream.json` and
  `state.json` were edited by hand in an earlier ticket rather than re-recorded,
  per `docs/reports/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md`);
  whether an added grant shifts what `cmd/vtt/scenario_goldens_test.go`
  compares was not traced.
- **Whether `characterSeats`' `player` arm is still reached.** Once no scenario
  file and no soak roster declares `player`, only a scenario built in code could;
  whether any test does, and so whether the arm stays, was not traced.
- **Where else the soak's names are read.** The search above is over `*.go` and
  `*.ts`; the soak's reports and goldens, if any, were not searched.
