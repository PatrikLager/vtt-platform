import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect } from "bun:test";
import { ClientCommandSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import * as commands from "../src/commands";
import { newState, type State } from "../src/state";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { Ability, Me } from "../src/metadata";
import { renderDMConsole } from "../src/view/dm";
import { renderPlayerPanel, type PlayerUIState } from "../src/view/player";

/**
 * WHERE A HUMAN REACHES EACH COMMAND.
 *
 * maps-as-geometry shipped with three commands (open_door, close_door,
 * load_map) that only the MCP agent could send: the web client had no builder
 * and no control, and nothing anywhere said so. Three arcs later it was still
 * true. This table is what says so.
 *
 * A count would rot by addition — the failure this table exists to prevent —
 * so nothing here asserts how many commands there are. It asserts that the
 * generated oneof and this table name the same set.
 */
type Surface =
  | "dm-console"
  | "player-panel"
  | "board"
  | "spectator"
  | "join-flow"
  | "not-user-issued";

export const COMMAND_SURFACE: Record<
  string,
  {
    surface: Surface;
    /**
     * The control's data-action, for the panels. Task 4 fills these in and
     * asserts each one renders; declared here so the shape does not change
     * under a later task.
     *
     * MATCHED EXACTLY (controlExistsFor below), not by prefix — a prefix
     * match here would let a renamed action string (`"create-scene"` typo'd
     * to `"create-scene-v2"`, `"use-ability"` widened to
     * `"use-ability-target"`) keep passing, since the new string still
     * STARTS WITH the old one. Fix round 1 (2026-08-30) demonstrated exactly
     * that hole against the first draft of this table, which matched every
     * entry by prefix. Only `dynamic: true` below opts an entry OUT of exact
     * matching, and only one entry needs to.
     */
    action?: string;
    /**
     * True for the one entry whose control's data-action is not a single
     * literal string: dm.ts renders promoteParticipant per roster row as
     * `promote-${participantId}` / `demote-${participantId}`
     * (dm-view.test.ts's own promotion tests pin those exact dynamic
     * strings), so no fixed action string ever appears in the DOM for it.
     * `action` for such an entry is a PATTERN PREFIX, matched by
     * controlExistsFor as `^${action}.+` (at least one character after the
     * prefix, so the bare prefix itself — a row nobody renders — does not
     * count as a match) rather than by equality. Every other entry has no
     * reason to set this and is matched exactly.
     */
    dynamic?: true;
    /** Required when surface is "not-user-issued", and only then. */
    why?: string;
  }
> = {
  startSession: { surface: "dm-console", action: "start-session" },
  endSession: { surface: "dm-console", action: "end-session" },
  createScene: { surface: "dm-console", action: "create-scene" },
  placeToken: { surface: "dm-console", action: "place-token" },
  addActor: { surface: "dm-console", action: "add-actor" },
  loadAdventure: { surface: "dm-console", action: "load-adventure" },
  loadMap: { surface: "dm-console", action: "load-map" },
  upsertNote: { surface: "dm-console", action: "upsert-note" },
  deleteNote: { surface: "dm-console", action: "delete-note" },
  // Rendered in the PLAYER PANEL (view/player.ts's "Say something" — a DM
  // also sees that panel beside the console, per app.ts's `canAct`), despite
  // the "dm-console" tag above: that tag predates this task and is left
  // alone here (fixing it is not this task's job), but controlExistsFor
  // renders both surfaces together, so the control is found regardless of
  // which of the two this row claims.
  addNarration: { surface: "dm-console", action: "add-narration" },
  removeCondition: { surface: "dm-console", action: "remove-condition" },
  grantActorControl: { surface: "dm-console", action: "grant-actor-control" },
  revokeActorControl: { surface: "dm-console", action: "revoke-actor-control" },
  promoteParticipant: { surface: "dm-console", action: "promote-", dynamic: true },
  setJoinDoor: { surface: "join-flow" },
  rotateJoinLink: { surface: "join-flow" },
  moveToken: { surface: "board" },
  // A LIVE NEAR-MISS, recorded rather than fixed, because fixing it would
  // create the trap it warns about: dm.ts's join-door buttons ALREADY carry
  // data-action="open-door"/"close-door" while sending setJoinDoor (the
  // shared-link door, spec §2) — nothing to do with these two board
  // commands (the map-tile door, maps-as-geometry spec). Neither openDoor
  // nor closeDoor has an action here because board-surfaced commands never
  // do (they fire from a board click, not a button) and this test's filter
  // only looks at "dm-console"/"player-panel" rows. The moment either of
  // these two gains an `action` field, controlExistsFor would find the
  // JOIN-door button by that string and wrongly report the map-door command
  // reachable — a control that opens a different door entirely. Do NOT
  // rename the join-door actions to dodge this: client/e2e selects on
  // "open-door"/"close-door" exactly as they are today.
  openDoor: { surface: "board" },
  closeDoor: { surface: "board" },
  useAbility: { surface: "player-panel", action: "use-ability" },
  setViewpoint: { surface: "spectator" },
  // removeToken (retraction-leaves Task 8, fix round 1): a Remove button
  // beside Place token's own token-id input (view/dm.ts). This is a
  // restoration, not a new capability — Task 3 of this same branch deleted
  // the Undo group, and with it the only way a DM could take a token off the
  // board at all; this row and its control are what put that back.
  removeToken: { surface: "dm-console", action: "remove-token" },
};

/** The oneof's case names, read from the generated descriptor. */
function commandCases(): string[] {
  const oneof = ClientCommandSchema.oneofs.find((o) => o.name === "command");
  if (!oneof) throw new Error("ClientCommand has no `command` oneof");
  return oneof.fields.map((f) => f.localName);
}

test("every command the contract defines has a declared human surface", () => {
  const declared = new Set(Object.keys(COMMAND_SURFACE));
  const missing = commandCases().filter((c) => !declared.has(c));
  expect(missing).toEqual([]);
});

test("the table declares nothing the contract does not define", () => {
  const defined = new Set(commandCases());
  const stale = Object.keys(COMMAND_SURFACE).filter((c) => !defined.has(c));
  expect(stale).toEqual([]);
});

test("a command nobody can issue must say why, so the hatch costs something", () => {
  const bare = Object.entries(COMMAND_SURFACE)
    .filter(([, v]) => v.surface === "not-user-issued" && !v.why?.trim())
    .map(([k]) => k);
  expect(bare).toEqual([]);
});

test("every command with a human surface has a builder in commands.ts", () => {
  // The builder's name is the case name: commands.ts has named them that way
  // since it was written, and this test is what keeps that true.
  const withoutBuilder = commandCases()
    .filter((c) => COMMAND_SURFACE[c]?.surface !== "not-user-issued")
    .filter((c) => typeof (commands as Record<string, unknown>)[c] !== "function");
  expect(withoutBuilder).toEqual([]);
});

test("no command builder can retract, because the platform cannot", () => {
  // Patrik, 2026-08-30: retraction leaves the platform. A retraction's purpose
  // is to make something not have happened, and it cannot do that — the player
  // read the log and knows what it said. This asserts the ABSENCE, so it is
  // written before the removal and must fail now.
  const retractors = Object.keys(commands).filter((k) => /retract/i.test(k));
  expect(retractors).toEqual([]);
});

// --- the control-level half (Task 4) ----------------------------------------
//
// The table above can name a surface without a real control ever existing for
// it — that was true of every dm-console/player-panel row until this task.
// The tests below render the ACTUAL panels against a fixture built to expose
// every group each one can show, and read the DOM rather than the table, so
// this cannot drift the way a second hand-maintained list would.

/**
 * A state rich enough to expose every group BOTH panels can render: a scene,
 * one actor the fixture participant controls with a token placed on it (so
 * player.ts's early "you control nothing" return never fires and the
 * targets/say-something sections build), and one condition on that actor (so
 * the DM console's "Remove condition" group — otherwise absent entirely,
 * per its own `if (condRow.length > 0)` — renders too).
 */
function richState(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 4, GridHeight: 4 };
  st.Actors["a1"] = {
    actorId: "a1", name: "Hero", moduleId: "", attributes: {}, resources: {},
    controllerId: "p-me", controllerIds: ["p-me"], kind: ActorKind.PARTY_MEMBER,
  };
  st.Tokens["t1"] = { ID: "t1", SceneID: "s1", ActorID: "a1", X: 1, Y: 1 };
  st.Conditions["a1"] = [{ ID: "prone", Source: "src", AppliedSeq: 1 }];
  return st;
}

/**
 * The DM console, rendered with adventures, maps, a joinLink, and a roster
 * holding one SPECTATOR (so the promote-* row renders — a player's row would
 * only ever show demote-*, and one row is enough to prove the command has a
 * reachable control at all). `open` picks which of the mutually exclusive
 * session groups shows: startSession and endSession can never both render
 * from one state (renderDMConsole's own `openSession ? [...] : [...]`), so
 * controlExistsFor below renders this fixture BOTH ways and searches across
 * both, rather than trying to make one state show both at once.
 */
function dmFixture(open: boolean): HTMLElement {
  const st = richState();
  if (open) st.Sessions.push({ ID: "sess-1", Name: "Night", StartSeq: 1, EndSeq: 0 });
  return renderDMConsole({
    st,
    adventures: [{ id: "adv-1", name: "Adventure" }],
    maps: [{ id: "map-1", name: "Map", gridWidth: 4, gridHeight: 4 }],
    guideFor: async () => null,
    participants: [{ participantId: "p-me", displayName: "Hero's Player" }],
    joinLink: { open: false, secret: "s3cret" },
    roster: [{ participantId: "p-watch", name: "Watcher", role: "spectator" }],
    origin: "https://table.example",
    refreshSharing: () => {},
    send: async () => {},
    notify: () => {},
    confirm: () => true,
    doorsArmed: false,
    toggleDoors: () => {},
  });
}

/**
 * The player panel, with one ability ARMED and a token in range of itself
 * (targetableTokens always includes the acting token at distance 0 — see
 * renderPlayerPanel's own comment on why that list can never be empty), so
 * the Targets section — the only place useAbility's control lives — builds.
 */
function playerFixture(): HTMLElement {
  const st = richState();
  const me: Me = { participantId: "p-me", name: "Hero's Player", role: "player" };
  const ability: Ability = { id: "poke", name: "Poke", range: 1, maxTargets: 1, usage: { kind: "atWill" } };
  const ui: PlayerUIState = { selectedActorId: "a1", selectedAbilityId: "poke", doorsArmed: false };
  return renderPlayerPanel(st, me, [ability], ui, () => {}, () => {});
}

/**
 * Whether commandName's declared action renders SOMEWHERE across the three
 * fixtures above. Looks the action up from COMMAND_SURFACE itself, so a
 * command declared with no action at all (the gap this task closes) reads as
 * unreachable rather than throwing — exactly the failure mode Step 2 of the
 * brief describes as the working list for this task.
 */
function controlExistsFor(commandName: string): boolean {
  const decl = COMMAND_SURFACE[commandName];
  const action = decl?.action;
  if (!decl || action === undefined) return false;
  // EXACT by default (`v === action`); PATTERN only for the one entry
  // declared `dynamic: true`, and even then it requires at least one
  // character after the prefix (`^${action}.+`) so the bare prefix string
  // itself — a row nothing ever renders — cannot count as a match. A CSS
  // attribute selector cannot express "at least one more character", which
  // is why this reads every data-action value and tests it in JS rather
  // than querying `[data-action^="..."]` directly.
  const pattern = decl.dynamic ? new RegExp(`^${action}.+`) : null;
  const matches = (value: string): boolean => (pattern ? pattern.test(value) : value === action);
  return [dmFixture(false), dmFixture(true), playerFixture()].some((node) =>
    Array.from(node.querySelectorAll<HTMLElement>("[data-action]")).some((el) =>
      matches(el.dataset["action"] ?? ""),
    ),
  );
}

test("every command surfaced in a panel has a control a person can reach", () => {
  // Renders the real panels and looks for the real controls, so this table
  // cannot drift from the UI: the assertion reads the DOM, not the table.
  const unreachable = Object.entries(COMMAND_SURFACE)
    .filter(([, v]) => v.surface === "dm-console" || v.surface === "player-panel")
    .filter(([name]) => !controlExistsFor(name))
    .map(([name]) => name);
  expect(unreachable).toEqual([]);
});
