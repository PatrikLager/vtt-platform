import "./support/dom";
import { test, expect } from "bun:test";
import type { ClientCommand, CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { newState, type State, type Tile } from "../src/state";
import type { Me } from "../src/metadata";
import { doorButtonsFor, doorDirection } from "../src/view/doors";
import { renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { boot } from "../src/app";
import { ok, settle } from "./support/answers";

// Independent QA of "a player works a door by a button", Task 1: VTT-881,
// VTT-882, VTT-883, VTT-886, VTT-887.

const ANN: Me = { participantId: "p-ann", name: "Ann", role: "player" };
const DORA: Me = { participantId: "p-dm", name: "Dora", role: "dm" };
const OTTO: Me = { participantId: "p-agent", name: "Otto", role: "agent" };

const door = (): Tile => ({ Kind: "door", Material: "wood", Art: "" });
const floor = (): Tile => ({ Kind: "floor", Material: "stone", Art: "" });
const wall = (): Tile => ({ Kind: "wall", Material: "stone", Art: "" });

function actor(st: State, id: string, name: string, controllers: string[], kind: ActorKind): void {
  st.Actors[id] = {
    actorId: id, name, moduleId: "", attributes: {}, resources: {},
    controllerId: controllers[0] ?? "", controllerIds: controllers, kind, abilityIds: [], references: {},
  };
}

function token(st: State, id: string, scene: string, actorId: string, x: number, y: number): void {
  st.Tokens[id] = { ID: id, SceneID: scene, ActorID: actorId, X: x, Y: y };
}

// Two scenes: "scene-b" is current (sorts last) though inserted first. Bea
// (pc-b, the one asked for, second by id) stands at 10,7 on scene-b beside a
// closed door north-east, an open door west and a closed door south; floor
// and wall beside her are no doors; 12,7 and 10,5 are two squares off. Her
// token on scene-a stands beside scene-a's doors, and beside 4,3, which is a
// door on scene-b at 5,4 only. Arn (pc-a) stands at 3,9 beside an open door
// north and a closed door south-east. Cora, nobody's, stands beside 6,2.
function threeState(controller: string, bea: [number, number] = [10, 7]): State {
  const st = newState();
  const b: Record<string, Tile> = {};
  b["10,8"] = door(); b["9,7"] = door(); b["11,6"] = door();
  b["12,7"] = door(); b["10,5"] = door(); b["5,4"] = door();
  b["3,8"] = door(); b["4,10"] = door(); b["6,2"] = door();
  b["10,6"] = floor(); b["11,7"] = floor(); b["9,8"] = wall();
  st.Scenes["scene-b"] = {
    ID: "scene-b", Name: "Hall", GridWidth: 14, GridHeight: 12, Tiles: b,
    OpenDoors: { "9,7": true, "3,8": true },
  };
  st.Scenes["scene-a"] = {
    ID: "scene-a", Name: "Cellar", GridWidth: 14, GridHeight: 12,
    Tiles: { "5,3": door(), "10,6": door(), "11,7": door() }, OpenDoors: { "11,7": true },
  };
  actor(st, "pc-a", "Arn", [controller], ActorKind.PARTY_MEMBER);
  actor(st, "pc-b", "Bea", [controller], ActorKind.PARTY_MEMBER);
  actor(st, "npc-c", "Cora", [], ActorKind.NON_PARTY);
  token(st, "t-a", "scene-b", "pc-a", 3, 9);
  token(st, "t-b", "scene-b", "pc-b", bea[0], bea[1]);
  token(st, "t-b2", "scene-a", "pc-b", 4, 3);
  token(st, "t-c", "scene-b", "npc-c", 5, 2);
  return st;
}

// A door on each of the nine squares around and under Bea at 10,7 on "s9",
// the current scene ("s9" sorts after "s10", which is inserted after it).
// Open: north, here, south-west.
function ringState(): State {
  const st = newState();
  const t: Record<string, Tile> = {};
  for (const k of ["11,8", "9,6", "10,7", "11,6", "9,8", "10,6", "11,7", "9,7", "10,8"]) t[k] = door();
  t["12,7"] = door(); t["10,5"] = door(); t["8,9"] = door(); t["3,8"] = door();
  st.Scenes["s9"] = {
    ID: "s9", Name: "Ring", GridWidth: 14, GridHeight: 12, Tiles: t,
    OpenDoors: { "10,6": true, "10,7": true, "9,8": true },
  };
  st.Scenes["s10"] = {
    ID: "s10", Name: "Other", GridWidth: 14, GridHeight: 12,
    Tiles: { "5,3": door(), "4,2": door() }, OpenDoors: {},
  };
  actor(st, "pc-a", "Arn", ["p-ann"], ActorKind.PARTY_MEMBER);
  actor(st, "pc-b", "Bea", ["p-ann"], ActorKind.PARTY_MEMBER);
  token(st, "t-a", "s9", "pc-a", 3, 9);
  token(st, "t-b", "s9", "pc-b", 10, 7);
  token(st, "t-b2", "s10", "pc-b", 4, 3);
  return st;
}

const RING_LABELS = [
  "Open door (north-west)", "Close door (north)", "Open door (north-east)",
  "Open door (west)", "Close door (here)", "Open door (east)",
  "Close door (south-west)", "Open door (south)", "Open door (south-east)",
];

interface DoorCmd { kind: string; sceneId: string; x: number; y: number; actorId: string }

const RING_CMDS: DoorCmd[] = [
  [9, 6, "openDoor"], [10, 6, "closeDoor"], [11, 6, "openDoor"],
  [9, 7, "openDoor"], [10, 7, "closeDoor"], [11, 7, "openDoor"],
  [9, 8, "closeDoor"], [10, 8, "openDoor"], [11, 8, "openDoor"],
].map(([x, y, kind]) => ({ kind: kind as string, sceneId: "s9", x: x as number, y: y as number, actorId: "pc-b" }));

const BEA_LABELS = ["Open door (north-east)", "Close door (west)", "Open door (south)"];
const BEA_CMDS: DoorCmd[] = [
  { kind: "openDoor", sceneId: "scene-b", x: 11, y: 6, actorId: "pc-b" },
  { kind: "closeDoor", sceneId: "scene-b", x: 9, y: 7, actorId: "pc-b" },
  { kind: "openDoor", sceneId: "scene-b", x: 10, y: 8, actorId: "pc-b" },
];
const ARN_LABELS = ["Close door (north)", "Open door (south-east)"];
const ARN_CMDS: DoorCmd[] = [
  { kind: "closeDoor", sceneId: "scene-b", x: 3, y: 8, actorId: "pc-a" },
  { kind: "openDoor", sceneId: "scene-b", x: 4, y: 10, actorId: "pc-a" },
];

function doorCmd(c: ClientCommand): DoorCmd | null {
  const k = c.command;
  if (k.case !== "openDoor" && k.case !== "closeDoor") return null;
  return { kind: k.case, sceneId: k.value.sceneId, x: k.value.at?.x ?? 0, y: k.value.at?.y ?? 0, actorId: k.value.actorId };
}

function ui(selectedActorId: string): PlayerUIState {
  return { selectedActorId, selectedAbilityId: "", doorsArmed: false };
}

function panel(st: State, me: Me, selected: string): { node: HTMLElement; sent: ClientCommand[] } {
  const sent: ClientCommand[] = [];
  const send = async (c: ClientCommand): Promise<CommandResult> => { sent.push(c); return ok(); };
  const node = renderPlayerPanel(st, me, [], ui(selected), send, () => {});
  return { node, sent };
}

function doorButtons(node: ParentNode): HTMLElement[] {
  return Array.from(node.querySelectorAll<HTMLElement>('[data-action="work-door"]'));
}

function labels(node: ParentNode): string[] {
  return doorButtons(node).map((b) => (b.textContent ?? "").trim());
}

// ---- module: doorButtonsFor / doorDirection ----

test("VTT-881 VTT-882 VTT-887: doorButtonsFor offers each of the nine doors at and around the watched token, labelled, in reading order", () => {
  expect(doorButtonsFor(ringState(), ANN, "pc-b").map((b) => b.label)).toEqual(RING_LABELS);
});

test("VTT-883: doorButtonsFor's commands open a closed door and close an open one at its square in the current scene, naming the actor", () => {
  expect(doorButtonsFor(ringState(), ANN, "pc-b").map((b) => doorCmd(b.command()))).toEqual(RING_CMDS);
});

test("VTT-882: doorDirection names each of the nine squares by the eight directions, north up, and here", () => {
  const st = ringState();
  const got = [
    [9, 6], [10, 6], [11, 6], [9, 7], [10, 7], [11, 7], [9, 8], [10, 8], [11, 8],
  ].map(([x, y]) => doorDirection(st, ANN, "pc-b", { x: x as number, y: y as number }));
  expect(got).toEqual([
    "north-west", "north", "north-east", "west", "here", "east", "south-west", "south", "south-east",
  ]);
});

test("VTT-881 VTT-887: doorButtonsFor offers only the current scene's doors beside the asked-for character's token there, by row then column", () => {
  expect(doorButtonsFor(threeState("p-ann"), ANN, "pc-b").map((b) => b.label)).toEqual(BEA_LABELS);
  expect(doorButtonsFor(threeState("p-ann"), ANN, "pc-b").map((b) => doorCmd(b.command()))).toEqual(BEA_CMDS);
});

test("VTT-881: doorButtonsFor for the other character offers that character's doors and none of Bea's", () => {
  const got = doorButtonsFor(threeState("p-ann"), ANN, "pc-a");
  expect(got.map((b) => b.label)).toEqual(ARN_LABELS);
  expect(got.map((b) => doorCmd(b.command()))).toEqual(ARN_CMDS);
});

test("VTT-881: doorButtonsFor offers nothing when the asked-for character's token stands beside no door", () => {
  expect(doorButtonsFor(threeState("p-ann", [12, 2]), ANN, "pc-b")).toEqual([]);
});

// ---- panel: renderPlayerPanel ----

test("VTT-881 VTT-882 VTT-887: a player's panel draws one work-door button per door beside the watched token, labelled, by row then column", () => {
  expect(labels(panel(threeState("p-ann"), ANN, "pc-b").node)).toEqual(BEA_LABELS);
});

test("VTT-881 VTT-882 VTT-887: a player's panel draws all nine doors at and around the watched token in reading order", () => {
  expect(labels(panel(ringState(), ANN, "pc-b").node)).toEqual(RING_LABELS);
});

test("VTT-883: a click on each door button sends open_door or close_door at its square in the current scene naming the watched character", async () => {
  const { node, sent } = panel(threeState("p-ann"), ANN, "pc-b");
  const got: (DoorCmd | null)[] = [];
  for (const b of doorButtons(node)) {
    const before = sent.length;
    b.click();
    await settle();
    expect(sent.length).toBe(before + 1);
    const last = sent[sent.length - 1];
    got.push(last ? doorCmd(last) : null);
  }
  expect(got).toEqual(BEA_CMDS);
});

test("VTT-883: a click on the 'here' door button closes the open door the watched token stands on", async () => {
  const { node, sent } = panel(ringState(), ANN, "pc-b");
  const here = doorButtons(node).find((b) => (b.textContent ?? "").trim() === "Close door (here)");
  expect(here).toBeDefined();
  here?.click();
  await settle();
  expect(sent.map(doorCmd)).toEqual([{ kind: "closeDoor", sceneId: "s9", x: 10, y: 7, actorId: "pc-b" }]);
});

test("VTT-881: a player's panel watching the other character draws that character's doors", () => {
  expect(labels(panel(threeState("p-ann"), ANN, "pc-a").node)).toEqual(ARN_LABELS);
});

test("VTT-881: a player's panel draws no work-door button while the watched token stands beside no door", () => {
  expect(doorButtons(panel(threeState("p-ann", [12, 2]), ANN, "pc-b").node)).toHaveLength(0);
});

test("VTT-886: a DM's and an agent's panel draw no work-door button, whatever actor is selected", () => {
  for (const me of [DORA, OTTO]) {
    for (const selected of ["pc-b", "pc-a", ""]) {
      expect(doorButtons(panel(threeState(me.participantId), me, selected).node)).toHaveLength(0);
      expect(doorButtons(panel(ringState(), me, selected).node)).toHaveLength(0);
    }
  }
});

// ---- window: boot ----

class FakeSocket {
  static instances: FakeSocket[] = [];
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  readyState = 0;
  sent: string[] = [];
  onopen: ((ev?: unknown) => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: ((ev?: unknown) => void) | null = null;
  onerror: ((ev?: unknown) => void) | null = null;
  constructor(public url: string) { FakeSocket.instances.push(this); }
  send(s: string): void { this.sent.push(s); }
  close(): void { this.readyState = 3; }
}

async function calm(): Promise<void> {
  for (let i = 0; i < 3; i++) {
    for (let j = 0; j < 10; j++) await Promise.resolve();
    await settle();
  }
}

interface Window_ { root: HTMLElement; sock: FakeSocket; deliver: (frames: object[]) => Promise<void> }

async function openWindow(me: Me, run: (w: Window_) => Promise<void>): Promise<void> {
  const g = globalThis as unknown as Record<string, unknown>;
  const realFetch = g["fetch"];
  const realSocket = g["WebSocket"];
  const answers: Record<string, unknown> = {
    "/api/me": me,
    "/api/ruleset": { id: "qa", name: "QA", abilities: [], conditions: [], resources: [], properties: [], sheet: [] },
    "/api/adventures": { adventures: [] },
    "/api/maps": { maps: [], cellPx: 64 },
  };
  g["fetch"] = async (input: unknown): Promise<Response> => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : (input as Request).url;
    const path = url.split("?")[0] ?? "";
    for (const [route, body] of Object.entries(answers)) {
      if (path.endsWith(route)) return new Response(JSON.stringify(body), { status: 200, headers: { "content-type": "application/json" } });
    }
    return new Response("", { status: 404 });
  };
  g["WebSocket"] = FakeSocket;
  FakeSocket.instances = [];
  localStorage.clear();
  sessionStorage.clear();
  localStorage.setItem("vtt.token", `tok-${me.participantId}`);
  const root = document.createElement("div");
  document.body.appendChild(root);
  try {
    boot(root);
    for (let i = 0; i < 20 && FakeSocket.instances.length === 0; i++) await calm();
    const sock = FakeSocket.instances[FakeSocket.instances.length - 1];
    if (!sock) throw new Error("boot opened no socket");
    sock.readyState = 1;
    sock.onopen?.();
    await calm();
    const deliver = async (frames: object[]): Promise<void> => {
      for (const f of frames) sock.onmessage?.({ data: JSON.stringify(f) });
      await calm();
    };
    await run({ root, sock, deliver });
  } finally {
    root.remove();
    g["fetch"] = realFetch;
    g["WebSocket"] = realSocket;
    localStorage.clear();
    sessionStorage.clear();
  }
}

function sentDoorCmds(sock: FakeSocket): DoorCmd[] {
  const out: DoorCmd[] = [];
  for (const s of sock.sent) {
    let j: Record<string, unknown>;
    try { j = JSON.parse(s) as Record<string, unknown>; } catch { continue; }
    for (const kind of ["openDoor", "closeDoor"]) {
      const v = j[kind] as { sceneId?: string; at?: { x?: number; y?: number }; actorId?: string } | undefined;
      if (v) out.push({ kind, sceneId: v.sceneId ?? "", x: v.at?.x ?? 0, y: v.at?.y ?? 0, actorId: v.actorId ?? "" });
    }
  }
  return out;
}

function answerLast(w: Window_): Promise<void> {
  const last = w.sock.sent[w.sock.sent.length - 1];
  const requestId = last ? ((JSON.parse(last) as { requestId?: string }).requestId ?? "") : "";
  return w.deliver([{ result: { requestId, ok: true } }]);
}

const wtile = (kind: string): object => ({ kind, material: "wood", art: "" });

// The same layout as threeState, as a log: Bea's token at bea, Arn's at arn.
function hallLog(arn: [number, number], bea: [number, number], world?: { participant: string }): object[] {
  let seq = 0;
  const e = (payload: object): object => {
    seq++;
    const head = world ? { actorRole: "dm", participantId: world.participant } : {};
    return { event: { eventId: `ev-${seq}`, sequence: String(seq), sessionId: "", ...head, ...payload } };
  };
  const owner = world ? world.participant : "p-ann";
  const kind = world ? "ACTOR_KIND_NON_PARTY" : "ACTOR_KIND_PARTY_MEMBER";
  return [
    e({ sceneCreated: { sceneId: "scene-b", name: "Hall", gridWidth: 14, gridHeight: 12, tiles: {
      "10,8": wtile("door"), "9,7": wtile("door"), "11,6": wtile("door"),
      "12,7": wtile("door"), "10,5": wtile("door"), "5,4": wtile("door"),
      "3,8": wtile("door"), "4,10": wtile("door"), "6,2": wtile("door"),
      "10,6": wtile("floor"), "11,7": wtile("floor"), "9,8": wtile("wall"),
    } } }),
    e({ sceneCreated: { sceneId: "scene-a", name: "Cellar", gridWidth: 14, gridHeight: 12, tiles: {
      "5,3": wtile("door"), "10,6": wtile("door"), "11,7": wtile("door"),
    } } }),
    e({ actorAdded: { actor: { actorId: "pc-a", name: "Arn", kind } } }),
    e({ actorAdded: { actor: { actorId: "pc-b", name: "Bea", kind } } }),
    e({ actorControlGranted: { actorId: "pc-a", participantId: owner, kind } }),
    e({ actorControlGranted: { actorId: "pc-b", participantId: owner, kind } }),
    e({ tokenPlaced: { tokenId: "t-a", sceneId: "scene-b", actorId: "pc-a", position: { x: arn[0], y: arn[1] } } }),
    e({ tokenPlaced: { tokenId: "t-b", sceneId: "scene-b", actorId: "pc-b", position: { x: bea[0], y: bea[1] } } }),
    e({ tokenPlaced: { tokenId: "t-b2", sceneId: "scene-a", actorId: "pc-b", position: { x: 4, y: 3 } } }),
    e({ doorOpened: { sceneId: "scene-b", at: { x: 9, y: 7 } } }),
    e({ doorOpened: { sceneId: "scene-b", at: { x: 3, y: 8 } } }),
    e({ doorOpened: { sceneId: "scene-a", at: { x: 11, y: 7 } } }),
  ];
}

const WATCHABLE = { watchableChanged: { actors: [{ actorId: "pc-a", name: "Arn" }, { actorId: "pc-b", name: "Bea" }] } };

test("VTT-881 VTT-882 VTT-887: a player's window draws the watched character's door buttons, labelled by the log's door state, by row then column", async () => {
  await openWindow(ANN, async (w) => {
    await w.deliver([WATCHABLE, { viewpointChanged: { actorId: "pc-b" } }, ...hallLog([3, 9], [10, 7])]);
    expect(labels(w.root)).toEqual(BEA_LABELS);
  });
});

test("VTT-883: a click on a player's window's door button sends open_door or close_door at its square in the current scene naming the watched character", async () => {
  await openWindow(ANN, async (w) => {
    await w.deliver([WATCHABLE, { viewpointChanged: { actorId: "pc-b" } }, ...hallLog([3, 9], [10, 7])]);
    const west = doorButtons(w.root).find((b) => (b.textContent ?? "").trim() === "Close door (west)");
    expect(west).toBeDefined();
    west?.click();
    await w.deliver([]);
    expect(sentDoorCmds(w.sock)).toEqual([BEA_CMDS[1] as DoorCmd]);
    await answerLast(w);
    const ne = doorButtons(w.root).find((b) => (b.textContent ?? "").trim() === "Open door (north-east)");
    expect(ne).toBeDefined();
    ne?.click();
    await w.deliver([]);
    expect(sentDoorCmds(w.sock)).toEqual([BEA_CMDS[1] as DoorCmd, BEA_CMDS[0] as DoorCmd]);
  });
});

test("VTT-881: after a switch to the other character the player's window draws that character's doors, and its clicks name it", async () => {
  await openWindow(ANN, async (w) => {
    await w.deliver([WATCHABLE, { viewpointChanged: { actorId: "pc-b" } }, ...hallLog([3, 9], [10, 7])]);
    expect(labels(w.root)).toEqual(BEA_LABELS);
    await w.deliver([{ viewpointChanged: { actorId: "pc-a" } }, ...hallLog([3, 9], [10, 7])]);
    expect(labels(w.root)).toEqual(ARN_LABELS);
    doorButtons(w.root)[1]?.click();
    await w.deliver([]);
    expect(sentDoorCmds(w.sock)).toEqual([ARN_CMDS[1] as DoorCmd]);
  });
});

test("VTT-881: after a switch to a character beside no door the player's window draws no door button", async () => {
  await openWindow(ANN, async (w) => {
    await w.deliver([WATCHABLE, { viewpointChanged: { actorId: "pc-b" } }, ...hallLog([3, 9], [10, 7])]);
    expect(labels(w.root)).toEqual(BEA_LABELS);
    await w.deliver([{ viewpointChanged: { actorId: "pc-a" } }, ...hallLog([12, 2], [10, 7])]);
    expect(doorButtons(w.root)).toHaveLength(0);
  });
});

for (const [me, who] of [[DORA, "a DM's"], [OTTO, "an agent's"]] as const) {
  test(`VTT-886: ${who} window draws no work-door button, whatever actor its chips select`, async () => {
    await openWindow(me, async (w) => {
      await w.deliver(hallLog([3, 9], [10, 7], { participant: me.participantId }));
      expect(doorButtons(w.root)).toHaveLength(0);
      for (const name of ["Bea", "Arn", "Bea"]) {
        const chips = Array.from(w.root.querySelectorAll<HTMLElement>("button"))
          .filter((b) => (b.textContent ?? "").trim() === name);
        expect(chips.length).toBeGreaterThan(0);
        chips[0]?.click();
        await w.deliver([]);
        expect(doorButtons(w.root)).toHaveLength(0);
      }
    });
  });
}
