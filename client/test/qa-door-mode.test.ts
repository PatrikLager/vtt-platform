import "./support/dom";
import { test, expect, afterEach } from "bun:test";
import { fromJsonString } from "@bufbuild/protobuf";
import { ClientCommandSchema, type ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { boot } from "../src/app";
import { renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { newState, type State } from "../src/state";
import type { Me } from "../src/metadata";
import { ok, settle } from "./support/answers";

// Requirements: VTT-936, VTT-938, VTT-939, VTT-885, VTT-888, VTT-876, VTT-886,
// VTT-940, VTT-408, and Done item 6 of docs/superpowers/specs/'s
// 2026-11-20-a-door-has-a-button-design.md (the DM's arming stands).

const ARM_DOORS = '[data-action="arm-doors"]';
const WORK_DOOR = '[data-action="work-door"]';

// The hall: 4x4, a closed door at 2,1, an open door at 3,0, an open door at
// 0,3 that a party member's token stands in, and a wall at 0,1.
const SCENE = "hall";
type Cell = { x: number; y: number };
const CLOSED_DOOR: Cell = { x: 2, y: 1 };
const OPEN_DOOR: Cell = { x: 3, y: 0 };
const OCCUPIED_OPEN_DOOR: Cell = { x: 0, y: 3 };
const WALL: Cell = { x: 0, y: 1 };
const PLAIN: Cell = { x: 1, y: 0 };
const PLAIN_LOW: Cell = { x: 3, y: 2 };

function tiles(withDoors: boolean): Record<string, { kind: string; material: string; art: string }> {
  const door = withDoors ? "door" : "floor";
  return {
    [`${CLOSED_DOOR.x},${CLOSED_DOOR.y}`]: { kind: door, material: "wood", art: "" },
    [`${OPEN_DOOR.x},${OPEN_DOOR.y}`]: { kind: door, material: "wood", art: "" },
    [`${OCCUPIED_OPEN_DOOR.x},${OCCUPIED_OPEN_DOOR.y}`]: { kind: door, material: "wood", art: "" },
    [`${WALL.x},${WALL.y}`]: { kind: "wall", material: "stone", art: "" },
  };
}

// ---- the window's seams -------------------------------------------------

class FakeSocket {
  static all: FakeSocket[] = [];
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  readyState = 0;
  sent: string[] = [];
  onopen: ((e?: unknown) => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e?: unknown) => void) | null = null;
  onerror: ((e?: unknown) => void) | null = null;
  constructor(public url: string) {
    FakeSocket.all.push(this);
  }
  send(s: string): void {
    this.sent.push(s);
  }
  close(): void {}
}

const realFetch = globalThis.fetch;
const realWebSocket = globalThis.WebSocket;
let savedToken: string | null = null;
let roots: HTMLElement[] = [];
let sessions: { close(): void }[] = [];
let sessionKeysBefore: string[] = [];

afterEach(() => {
  for (const s of sessions) s.close();
  sessions = [];
  for (const r of roots) r.remove();
  roots = [];
  globalThis.fetch = realFetch;
  globalThis.WebSocket = realWebSocket;
  if (savedToken === null) localStorage.removeItem("vtt.token");
  else localStorage.setItem("vtt.token", savedToken);
  for (let i = sessionStorage.length - 1; i >= 0; i--) {
    const k = sessionStorage.key(i);
    if (k !== null && !sessionKeysBefore.includes(k)) sessionStorage.removeItem(k);
  }
  FakeSocket.all = [];
});

async function quiet(): Promise<void> {
  for (let i = 0; i < 3; i++) {
    for (let j = 0; j < 10; j++) await Promise.resolve();
    await settle();
  }
}

interface Window {
  root: HTMLElement;
  sock: FakeSocket;
  deliver: (frame: unknown) => Promise<void>;
}

async function openWindow(me: Me): Promise<Window> {
  // A page URL another file left behind must not change the fetched paths.
  (globalThis as unknown as { happyDOM: { setURL(u: string): void } }).happyDOM.setURL("http://localhost/");
  savedToken = localStorage.getItem("vtt.token");
  sessionKeysBefore = [];
  for (let i = 0; i < sessionStorage.length; i++) {
    const k = sessionStorage.key(i);
    if (k !== null) sessionKeysBefore.push(k);
  }
  localStorage.setItem("vtt.token", `qa-door-mode-${me.participantId}`);
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const path = new URL(url).pathname;
    const json = (body: unknown) => new Response(JSON.stringify(body), {
      status: 200, headers: { "content-type": "application/json" },
    });
    if (path === "/api/me") return json(me);
    if (path === "/api/ruleset") {
      return json({ id: "qa", name: "QA", abilities: [], conditions: [], resources: [], properties: [], sheet: [] });
    }
    if (path === "/api/adventures") return json({ adventures: [] });
    return new Response("", { status: 404 });
  }) as unknown as typeof fetch;
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;

  const root = document.createElement("div");
  document.body.appendChild(root);
  roots.push(root);
  const session = boot(root);
  if (session) sessions.push(session);
  await quiet();
  const sock = FakeSocket.all[FakeSocket.all.length - 1];
  if (!sock) throw new Error("the window opened no socket");
  sock.readyState = 1;
  sock.onopen?.();
  await quiet();
  return {
    root, sock,
    deliver: async (frame: unknown) => {
      sock.onmessage?.({ data: JSON.stringify(frame) });
      await quiet();
    },
  };
}

let seq = 0;
function ev(participantId: string, payload: Record<string, unknown>): unknown {
  seq += 1;
  return {
    event: {
      eventId: `ev-${seq}`, sequence: String(seq), sessionId: "", actorRole: "dm", participantId,
      ...payload,
    },
  };
}

function sceneFrame(withDoors: boolean): Record<string, unknown> {
  return {
    sceneCreated: {
      sceneId: SCENE, name: "Hall", gridWidth: 4, gridHeight: 4, tiles: tiles(withDoors),
    },
  };
}

function actorFrame(actorId: string, name: string, kind: string): Record<string, unknown> {
  return { actorAdded: { actor: { actorId, name, kind } } };
}

/** The commands the window sent after `from`, each parsed as a ClientCommand. */
function sentSince(w: Window, from: number): ClientCommand[] {
  return w.sock.sent.slice(from).map((s) => fromJsonString(ClientCommandSchema, s, { ignoreUnknownFields: true }));
}

/** A command in a word: its case, and what it names. */
function said(c: ClientCommand): string {
  const v = c.command;
  switch (v.case) {
    case "moveToken": return `move ${v.value.tokenId} to ${v.value.to?.x ?? 0},${v.value.to?.y ?? 0}`;
    case "setViewpoint": return `viewpoint ${v.value.actorId}`;
    case "openDoor": return `open ${v.value.sceneId} ${v.value.at?.x ?? 0},${v.value.at?.y ?? 0} actor=${v.value.actorId}`;
    case "closeDoor": return `close ${v.value.sceneId} ${v.value.at?.x ?? 0},${v.value.at?.y ?? 0} actor=${v.value.actorId}`;
    default: return `other ${String(v.case)}`;
  }
}

/** Click the board on a cell; in this DOM a 4x4 scene's cells are 120px, row 0 at clientY 0, cell 0,0 holding clientX 90. */
async function clickCell(w: Window, cell: Cell): Promise<string[]> {
  const grid = w.root.querySelector<HTMLElement>(".grid");
  if (!grid) throw new Error("the window draws no board");
  const from = w.sock.sent.length;
  grid.dispatchEvent(new MouseEvent("click", { clientX: 120 * cell.x + 100, clientY: 120 * cell.y + 60, bubbles: true }));
  await quiet();
  return sentSince(w, from).map(said);
}

// ---- a player's window ---------------------------------------------------

const PLAYER: Me = { participantId: "p-lera", name: "Lera's player", role: "player" };

/** A player watching Lera (token at 1,2), with Bren listed (token in the open door at 0,3). */
async function playerWindow(withDoors: boolean): Promise<Window> {
  seq = 0;
  const w = await openWindow(PLAYER);
  await w.deliver({ watchableChanged: { actors: [{ actorId: "act-lera", name: "Lera" }, { actorId: "act-bren", name: "Bren" }] } });
  await w.deliver({ viewpointChanged: { actorId: "act-lera" } });
  const p = "p-dm";
  await w.deliver(ev(p, sceneFrame(withDoors)));
  await w.deliver(ev(p, actorFrame("act-lera", "Lera", "ACTOR_KIND_PARTY_MEMBER")));
  await w.deliver(ev(p, { actorControlGranted: { actorId: "act-lera", participantId: PLAYER.participantId, kind: "ACTOR_KIND_PARTY_MEMBER" } }));
  await w.deliver(ev(p, actorFrame("act-bren", "Bren", "ACTOR_KIND_PARTY_MEMBER")));
  await w.deliver(ev(p, { actorControlGranted: { actorId: "act-bren", participantId: PLAYER.participantId, kind: "ACTOR_KIND_PARTY_MEMBER" } }));
  await w.deliver(ev(p, { tokenPlaced: { tokenId: "tok-lera", sceneId: SCENE, actorId: "act-lera", position: { x: 1, y: 2 } } }));
  await w.deliver(ev(p, { tokenPlaced: { tokenId: "tok-bren", sceneId: SCENE, actorId: "act-bren", position: OCCUPIED_OPEN_DOOR } }));
  if (withDoors) {
    await w.deliver(ev(p, { doorOpened: { sceneId: SCENE, at: OPEN_DOOR } }));
    await w.deliver(ev(p, { doorOpened: { sceneId: SCENE, at: OCCUPIED_OPEN_DOOR } }));
  }
  return w;
}

test("VTT-408: a player's click on a plain square moves the watched character's token there (the baseline the door cases compare with)", async () => {
  const w = await playerWindow(true);
  expect(await clickCell(w, PLAIN_LOW)).toEqual(["move tok-lera to 3,2"]);
  expect(await clickCell(w, PLAIN)).toEqual(["move tok-lera to 1,0"]);
});

test("VTT-936 VTT-938: a player's click on a closed door square sends nothing, where the same click on a square with no door moves", async () => {
  const doored = await playerWindow(true);
  expect(doored.root.querySelector(ARM_DOORS)).toBeNull(); // VTT-885, in the window
  expect(await clickCell(doored, CLOSED_DOOR)).toHaveLength(0);
  const plain = await playerWindow(false);
  expect(await clickCell(plain, CLOSED_DOOR)).toEqual([`move tok-lera to ${CLOSED_DOOR.x},${CLOSED_DOOR.y}`]);
});

test("VTT-938 VTT-939 VTT-408: a player's click on an open door square moves the watched token there, as the same click does where no door is", async () => {
  const doored = await playerWindow(true);
  expect(doored.root.querySelector(ARM_DOORS)).toBeNull(); // VTT-885, in the window
  const onDoor = await clickCell(doored, OPEN_DOOR);
  const plain = await playerWindow(false);
  const onFloor = await clickCell(plain, OPEN_DOOR);
  expect(onDoor.filter((s) => s.startsWith("open ") || s.startsWith("close "))).toHaveLength(0);
  expect(onDoor).toEqual([`move tok-lera to ${OPEN_DOOR.x},${OPEN_DOOR.y}`]);
  expect(onDoor).toEqual(onFloor);
});

test("VTT-938 VTT-940: a player's click on an open door square holding a listed party member's token switches to them, as the same click does where no door is", async () => {
  const doored = await playerWindow(true);
  const onDoor = await clickCell(doored, OCCUPIED_OPEN_DOOR);
  const plain = await playerWindow(false);
  const onFloor = await clickCell(plain, OCCUPIED_OPEN_DOOR);
  expect(onDoor.filter((s) => s.startsWith("open ") || s.startsWith("close "))).toHaveLength(0);
  expect(onDoor).toEqual(["viewpoint act-bren"]);
  expect(onDoor).toEqual(onFloor);
});

// ---- a player's panel ----------------------------------------------------

function panelState(): State {
  const st = newState();
  st.Scenes[SCENE] = {
    ID: SCENE, Name: "Hall", GridWidth: 4, GridHeight: 4,
    Tiles: {
      [`${CLOSED_DOOR.x},${CLOSED_DOOR.y}`]: { Kind: "door", Material: "wood", Art: "" },
      [`${OPEN_DOOR.x},${OPEN_DOOR.y}`]: { Kind: "door", Material: "wood", Art: "" },
      [`${OCCUPIED_OPEN_DOOR.x},${OCCUPIED_OPEN_DOOR.y}`]: { Kind: "door", Material: "wood", Art: "" },
    },
    OpenDoors: { [`${OPEN_DOOR.x},${OPEN_DOOR.y}`]: true, [`${OCCUPIED_OPEN_DOOR.x},${OCCUPIED_OPEN_DOOR.y}`]: true },
  };
  for (const [id, name] of [["act-lera", "Lera"], ["act-bren", "Bren"]] as const) {
    st.Actors[id] = {
      actorId: id, name, moduleId: "", attributes: {}, resources: {},
      controllerId: PLAYER.participantId, controllerIds: [PLAYER.participantId],
      kind: ActorKind.PARTY_MEMBER, abilityIds: [], references: {},
    };
  }
  st.Tokens["tok-lera"] = { ID: "tok-lera", SceneID: SCENE, ActorID: "act-lera", X: 1, Y: 2 };
  st.Tokens["tok-bren"] = { ID: "tok-bren", SceneID: SCENE, ActorID: "act-bren", X: OCCUPIED_OPEN_DOOR.x, Y: OCCUPIED_OPEN_DOOR.y };
  return st;
}

for (const doorsArmed of [false, true]) {
  test(`VTT-885: a player's panel beside two doors draws no arm-doors toggle, with doorsArmed ${doorsArmed}`, () => {
    const ui: PlayerUIState = { selectedActorId: "act-lera", selectedAbilityId: "", doorsArmed };
    const node = renderPlayerPanel(
      panelState(), PLAYER, [], ui, async () => ok(), () => {},
      { actors: [{ actorId: "act-lera", name: "Lera" }, { actorId: "act-bren", name: "Bren" }], onSwitch: () => {} },
    );
    // Precondition: the watched token stands beside doors, so the door
    // area is drawn (VTT-881).
    expect(node.querySelectorAll(WORK_DOOR).length).toBeGreaterThan(0);
    expect(node.querySelectorAll(ARM_DOORS).length).toBe(0);
    const words = Array.from(node.querySelectorAll("button")).map((b) => (b.textContent ?? "").trim());
    expect(words.filter((t) => /arm doors|doors armed/i.test(t))).toEqual([]);
  });
}

// ---- a DM's and an agent's window ---------------------------------------

/** A DM's or an agent's window, granted the goblin (token at 2,2, beside the closed door), chip selected. */
async function consoleWindow(me: Me): Promise<Window> {
  seq = 0;
  const w = await openWindow(me);
  const p = me.participantId;
  await w.deliver(ev(p, sceneFrame(true)));
  await w.deliver(ev(p, actorFrame("act-gob", "Goblin", "ACTOR_KIND_NON_PARTY")));
  await w.deliver(ev(p, { actorControlGranted: { actorId: "act-gob", participantId: p, kind: "ACTOR_KIND_NON_PARTY" } }));
  await w.deliver(ev(p, { tokenPlaced: { tokenId: "tok-gob", sceneId: SCENE, actorId: "act-gob", position: { x: 2, y: 2 } } }));
  await w.deliver(ev(p, { doorOpened: { sceneId: SCENE, at: OPEN_DOOR } }));
  await w.deliver(ev(p, { doorOpened: { sceneId: SCENE, at: OCCUPIED_OPEN_DOOR } }));
  const chip = Array.from(w.root.querySelectorAll<HTMLButtonElement>("button"))
    .find((b) => !b.closest(".dm") && (b.textContent ?? "").trim() === "Goblin");
  if (!chip) throw new Error("the window draws no chip for the goblin");
  chip.click();
  await quiet();
  return w;
}

async function arm(w: Window): Promise<void> {
  const toggle = w.root.querySelector<HTMLButtonElement>(`.dm ${ARM_DOORS}`);
  if (!toggle) throw new Error("the console draws no arm-doors toggle");
  toggle.click();
  await quiet();
}

for (const role of ["dm", "agent"] as const) {
  const me: Me = { participantId: `p-${role}`, name: role.toUpperCase(), role };

  test(`Done item 6 VTT-886: a ${role}'s console offers "Arm doors"; armed, the board carries the armed label; its panel draws no work-door button`, async () => {
    const w = await consoleWindow(me);
    const toggle = w.root.querySelector(`.dm ${ARM_DOORS}`);
    expect((toggle?.textContent ?? "").trim()).toBe("Arm doors");
    expect(w.root.querySelectorAll(WORK_DOOR).length).toBe(0);
    await arm(w);
    expect((w.root.querySelector(`.dm ${ARM_DOORS}`)?.textContent ?? "").trim()).toBe("Doors armed");
    expect(w.root.querySelector(".grid")?.classList.contains("armed")).toBe(true);
    expect(w.root.querySelector(".armed-label")).not.toBeNull();
    expect(w.root.querySelectorAll(WORK_DOOR).length).toBe(0);
  });

  test(`VTT-888 VTT-876: a ${role}'s armed click on a closed door opens it and on an open door closes it, at that square, naming no actor`, async () => {
    const w = await consoleWindow(me);
    await arm(w);
    expect(await clickCell(w, CLOSED_DOOR)).toEqual([`open ${SCENE} 2,1 actor=`]);
    expect(await clickCell(w, OPEN_DOOR)).toEqual([`close ${SCENE} 3,0 actor=`]);
    expect(await clickCell(w, OCCUPIED_OPEN_DOOR)).toEqual([`close ${SCENE} 0,3 actor=`]);
  });

  test(`VTT-888: a ${role}'s armed click on a plain square, a wall or its own token's square sends nothing`, async () => {
    const w = await consoleWindow(me);
    await arm(w);
    expect(await clickCell(w, PLAIN)).toEqual([]);
    expect(await clickCell(w, WALL)).toEqual([]);
    expect(await clickCell(w, { x: 2, y: 2 })).toEqual([]);
    expect(await clickCell(w, PLAIN_LOW)).toEqual([]);
  });

  test(`Done item 6 (background): a ${role}'s unarmed click moves the chip's actor, on a plain square and on a door square alike, and works no door`, async () => {
    const w = await consoleWindow(me);
    expect(await clickCell(w, PLAIN)).toEqual(["move tok-gob to 1,0"]);
    expect(await clickCell(w, CLOSED_DOOR)).toEqual(["move tok-gob to 2,1"]);
    expect(await clickCell(w, OPEN_DOOR)).toEqual(["move tok-gob to 3,0"]);
  });
}
