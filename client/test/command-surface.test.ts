import { test, expect } from "bun:test";
import { ClientCommandSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import * as commands from "../src/commands";

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
    /** The control's data-action, for the panels. Task 4 fills these in and
     *  asserts each one renders; declared here so the shape does not change
     *  under a later task. */
    action?: string;
    /** Required when surface is "not-user-issued", and only then. */
    why?: string;
  }
> = {
  startSession: { surface: "dm-console" },
  endSession: { surface: "dm-console" },
  createScene: { surface: "dm-console" },
  placeToken: { surface: "dm-console" },
  addActor: { surface: "dm-console" },
  loadAdventure: { surface: "dm-console" },
  loadMap: { surface: "dm-console" },
  upsertNote: { surface: "dm-console" },
  deleteNote: { surface: "dm-console" },
  addNarration: { surface: "dm-console" },
  removeCondition: { surface: "dm-console" },
  retractEvents: { surface: "dm-console" },
  grantActorControl: { surface: "dm-console" },
  revokeActorControl: { surface: "dm-console" },
  promoteParticipant: { surface: "dm-console" },
  setJoinDoor: { surface: "join-flow" },
  rotateJoinLink: { surface: "join-flow" },
  moveToken: { surface: "board" },
  openDoor: { surface: "board" },
  closeDoor: { surface: "board" },
  useAbility: { surface: "player-panel" },
  setViewpoint: { surface: "spectator" },
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
