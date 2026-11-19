import "./support/dom";
import { afterEach, describe, expect, test } from "bun:test";
import { toJson } from "@bufbuild/protobuf";
import { ClientCommandSchema, type ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { newState, type State } from "../src/state";
import type { Me } from "../src/metadata";
import { openDoor, closeDoor } from "../src/commands";
import { doorCommandFor, mayWorkDoor } from "../src/view/doors";
import { actingActor, renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { boot } from "../src/app";
import { ok, settle } from "./support/answers";

const j = (c: ClientCommand | null) => {
  if (!c) return null;
  const { requestId: _drop, ...rest } = toJson(ClientCommandSchema, c) as Record<string, unknown>;
  return rest;
};

const player: Me = { participantId: "p1", name: "Pia", role: "player" };
const dm: Me = { participantId: "d1", name: "Dag", role: "dm" };
const agent: Me = { participantId: "g1", name: "Age", role: "agent" };

// "z-near" stands beside the door; "a-far" two squares off on x.
const DOOR = { x: 2, y: 1 };

function world(): State {
  const st = newState();
  st.Scenes["s1"] = {
    ID: "s1", Name: "Hall", GridWidth: 4, GridHeight: 4,
    Tiles: { "2,1": { Kind: "door", Material: "wood", Art: "" } },
    Objects: [], OpenDoors: {},
  } as never;
  for (const id of ["z-near", "a-far"]) {
    st.Actors[id] = {
      actorId: id, name: id, moduleId: "", attributes: {}, resources: {},
      controllerId: "p1", controllerIds: ["p1"], kind: ActorKind.PARTY_MEMBER, abilityIds: [],
    };
  }
  st.Tokens["t-far"] = { ID: "t-far", SceneID: "s1", ActorID: "a-far", X: 0, Y: 1 };
  st.Tokens["t-near"] = { ID: "t-near", SceneID: "s1", ActorID: "z-near", X: 3, Y: 2 };
  return st;
}

function ui(selected: string, armed = true): PlayerUIState {
  return { selectedActorId: selected, selectedAbilityId: "", doorsArmed: armed };
}

describe("module level", () => {
  // VTT-873
  test("a player's window acts as the selected actor, else the first controlled by id", () => {
    const st = world();
    expect(actingActor(st, player, ui("z-near"))).toBe("z-near");
    expect(actingActor(st, player, ui(""))).toBe("a-far");
  });

  // VTT-873
  test("a player's open door command names the selected actor beside the door", () => {
    const st = world();
    const actor = actingActor(st, player, ui("z-near"));
    expect(j(doorCommandFor(st, player, true, DOOR, actor))).toEqual({
      openDoor: { sceneId: "s1", at: { x: 2, y: 1 }, actorId: "z-near" },
    });
  });

  // VTT-873
  test("a player's close door command names the actor", () => {
    const st = world();
    st.Scenes["s1"]!.OpenDoors = { "2,1": true };
    expect(j(doorCommandFor(st, player, true, DOOR, "z-near"))).toEqual({
      closeDoor: { sceneId: "s1", at: { x: 2, y: 1 }, actorId: "z-near" },
    });
  });

  // VTT-874
  test("a player's click on a door the acting actor is not beside sends nothing, though another is", () => {
    const st = world();
    expect(doorCommandFor(st, player, true, DOOR, "a-far")).toBeNull();
    expect(doorCommandFor(st, player, true, DOOR, actingActor(st, player, ui("")))).toBeNull();
  });

  // VTT-874
  test("a player's door command needs the actor's token on the current scene", () => {
    const st = world();
    st.Scenes["s0"] = { ID: "s0", Name: "Old", GridWidth: 4, GridHeight: 4, Tiles: {}, Objects: [], OpenDoors: {} } as never;
    st.Tokens["t-near"]!.SceneID = "s0";
    expect(doorCommandFor(st, player, true, DOOR, "z-near")).toBeNull();
  });

  test("unarmed, a door click sends nothing", () => {
    expect(doorCommandFor(world(), player, false, DOOR, "z-near")).toBeNull();
  });

  // VTT-874
  test("the actor's token one square away on x and two on y is too far", () => {
    const st = world();
    st.Tokens["t-near"]!.X = 3;
    st.Tokens["t-near"]!.Y = 3;
    expect(doorCommandFor(st, player, true, DOOR, "z-near")).toBeNull();
    expect(mayWorkDoor(st, player, "z-near", DOOR)).toBe(false);
  });

  // VTT-874 VTT-875
  test("mayWorkDoor is true only for the actor beside the door", () => {
    const st = world();
    expect(mayWorkDoor(st, player, "z-near", DOOR)).toBe(true);
    expect(mayWorkDoor(st, player, "a-far", DOOR)).toBe(false);
  });

  // VTT-876
  test("a DM's and an agent's door commands name no actor whatever actor is handed", () => {
    const st = world();
    for (const me of [dm, agent]) {
      for (const actor of ["z-near", "a-far", ""]) {
        expect(j(doorCommandFor(st, me, true, DOOR, actor))).toEqual({
          openDoor: { sceneId: "s1", at: { x: 2, y: 1 } },
        });
      }
    }
    st.Scenes["s1"]!.OpenDoors = { "2,1": true };
    expect(j(doorCommandFor(st, dm, true, DOOR, "z-near"))).toEqual({
      closeDoor: { sceneId: "s1", at: { x: 2, y: 1 } },
    });
  });

  // VTT-876
  test("openDoor and closeDoor name an actor only when given one", () => {
    expect(j(openDoor("s1", DOOR))).toEqual({ openDoor: { sceneId: "s1", at: { x: 2, y: 1 } } });
    expect(j(closeDoor("s1", DOOR))).toEqual({ closeDoor: { sceneId: "s1", at: { x: 2, y: 1 } } });
    expect(j(openDoor("s1", DOOR, "z-near"))).toEqual({ openDoor: { sceneId: "s1", at: { x: 2, y: 1 }, actorId: "z-near" } });
    expect(j(closeDoor("s1", DOOR, "z-near"))).toEqual({ closeDoor: { sceneId: "s1", at: { x: 2, y: 1 }, actorId: "z-near" } });
  });

  // VTT-875
  test("the panel offers the toggle only for a selected actor beside a door", () => {
    const st = world();
    const draw = (sel: string) =>
      renderPlayerPanel(st, player, [], ui(sel, false), async () => ok(), () => {})
        .querySelector('[data-action="arm-doors"]');
    expect(draw("z-near")).not.toBeNull();
    expect(draw("a-far")).toBeNull();
    expect(draw("")).toBeNull();
  });
});

// ---- the window, through boot ----

class FakeSocket {
  static last: FakeSocket;
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  readyState = 1;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) { FakeSocket.last = this; }
  send(s: string) { this.sent.push(s); }
  close() { this.readyState = 3; }
}

const realFetch = globalThis.fetch;
const realWS = (globalThis as { WebSocket: unknown }).WebSocket;
let tokenBefore: string | null = null;

afterEach(() => {
  globalThis.fetch = realFetch;
  (globalThis as { WebSocket: unknown }).WebSocket = realWS;
  try {
    if (tokenBefore === null) localStorage.removeItem("vtt.token");
    else localStorage.setItem("vtt.token", tokenBefore);
  } catch { /* storage may be absent */ }
  document.body.innerHTML = "";
});

function install(me: Me) {
  tokenBefore = localStorage.getItem("vtt.token");
  localStorage.setItem("vtt.token", "tok");
  const answers: Record<string, unknown> = {
    "/api/me": me,
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
    "/api/participants": [],
  };
  globalThis.fetch = (async (input: unknown) => {
    const url = String(typeof input === "object" && input && "url" in input ? (input as { url: string }).url : input);
    const key = Object.keys(answers).find((k) => url.includes(k));
    return new Response(JSON.stringify(key ? answers[key] : {}), { status: 200, headers: { "content-type": "application/json" } });
  }) as typeof fetch;
  (globalThis as { WebSocket: unknown }).WebSocket = FakeSocket;
  const root = document.createElement("div");
  document.body.appendChild(root);
  boot(root);
  return root;
}

let seq = 0;
function deliver(frame: unknown) {
  FakeSocket.last.onmessage!({ data: JSON.stringify(frame) });
}
function ev(payload: Record<string, unknown>) {
  seq += 1;
  deliver({ event: { eventId: `e${seq}`, sequence: String(seq), sessionId: "ss", actorRole: "dm", participantId: "d1", ...payload } });
}

function sceneLog(withControl: boolean) {
  seq = 0;
  ev({ sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 4, gridHeight: 4, tiles: { "2,1": { kind: "door", material: "wood", art: "" } } } });
  for (const id of ["a-far", "z-near"]) {
    ev({ actorAdded: { actor: { actorId: id, name: id, kind: "ACTOR_KIND_PARTY_MEMBER" } } });
    if (withControl) ev({ actorControlGranted: { actorId: id, participantId: "p1", kind: "ACTOR_KIND_PARTY_MEMBER" } });
  }
  ev({ tokenPlaced: { tokenId: "t-far", sceneId: "s1", actorId: "a-far", position: { x: 0, y: 1 } } });
  ev({ tokenPlaced: { tokenId: "t-near", sceneId: "s1", actorId: "z-near", position: { x: 3, y: 2 } } });
}

async function pump() { await settle(); await settle(); }

const CELL_PX = 120;
function clickCell(root: HTMLElement, cx: number, cy: number) {
  const grid = root.querySelector(".grid")!;
  grid.dispatchEvent(new MouseEvent("click", { clientX: 90 + CELL_PX * cx, clientY: 10 + CELL_PX * cy, bubbles: true }));
}
const doorCmds = (s: FakeSocket) =>
  s.sent.map((x) => { const c = JSON.parse(x); delete c.requestId; return c; }).filter((c) => c.openDoor || c.closeDoor);

describe("the window", () => {
  // VTT-873 VTT-874 VTT-875 VTT-879
  test("a player watching the character beside the door names it; switching to the far one removes the tool", async () => {
    const root = install(player);
    await pump();
    const s = FakeSocket.last;
    s.onopen!();
    deliver({ watchableChanged: { actors: [{ actorId: "a-far", name: "a-far" }, { actorId: "z-near", name: "z-near" }] } });
    deliver({ viewpointChanged: { actorId: "z-near" } });
    sceneLog(true);
    await pump();

    const toggle = root.querySelector('[data-action="arm-doors"]') as HTMLElement;
    expect(toggle).not.toBeNull();
    toggle.click();
    await pump();
    expect(root.querySelector(".grid")!.classList.contains("armed")).toBe(true);

    clickCell(root, 2, 1);
    await pump();
    expect(doorCmds(s)).toEqual([{ openDoor: { sceneId: "s1", at: { x: 2, y: 1 }, actorId: "z-near" } }]);

    s.sent.length = 0;
    deliver({ viewpointChanged: { actorId: "a-far" } });
    sceneLog(true);
    await pump();
    expect(root.querySelector('[data-action="arm-doors"]')).toBeNull();
    expect(root.querySelector(".grid")!.classList.contains("armed")).toBe(false);
    clickCell(root, 2, 1);
    await pump();
    expect(doorCmds(s)).toEqual([]);
  });

  test("a DM's armed click on a door names no actor", async () => {
    const root = install(dm);
    await pump();
    const s = FakeSocket.last;
    s.onopen!();
    sceneLog(false);
    await pump();
    const toggle = root.querySelector('.dm [data-action="arm-doors"]') as HTMLElement;
    expect(toggle).not.toBeNull();
    toggle.click();
    await pump();
    clickCell(root, 2, 1);
    await pump();
    expect(doorCmds(s)).toEqual([{ openDoor: { sceneId: "s1", at: { x: 2, y: 1 } } }]);
  });
});
