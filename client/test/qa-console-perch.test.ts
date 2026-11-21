// Independent QA: the console's "Make spectator" and the window's perch.
// Derived from the requirement rows, not from the implementation.

import "./support/dom";
import { afterAll, afterEach, describe, expect, test } from "bun:test";
import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import {
  ClientCommandSchema, type ClientCommand, type CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { renderDMConsole } from "../src/view/dm";
import { newState, type Actor, type State } from "../src/state";
import type { Participant } from "../src/session";
import type { Roster } from "../src/metadata";
import { promoteParticipant } from "../src/commands";
import { boot } from "../src/app";
import { held, ok, refused, settle } from "./support/answers";

// ---------------------------------------------------------------------------
// The console
// ---------------------------------------------------------------------------

const DM = "p-dm";
const AGENT = "p-agent";
const ANN = "p-ann";
const BO = "p-bo";
const SID = "p-sid";

function roster(): Roster[] {
  return [
    { participantId: DM, name: "Dana", role: "dm" },
    { participantId: AGENT, name: "Otto", role: "agent" },
    { participantId: ANN, name: "Ann", role: "player" },
    { participantId: BO, name: "Bo", role: "player" },
    { participantId: SID, name: "Sid", role: "spectator" },
  ];
}

// Presence names differ from the roster's, so a question naming the
// presence name instead of the roster's is caught.
function presence(): Participant[] {
  return [
    { participantId: ANN, displayName: "Annie on her phone" },
    { participantId: BO, displayName: "Bobby" },
  ];
}

function actor(id: string, name: string, controllers: string[]): Actor {
  return {
    actorId: id, name, moduleId: "", attributes: {}, resources: {},
    controllerId: controllers[0] ?? "", controllerIds: controllers,
    kind: ActorKind.PARTY_MEMBER, abilityIds: [],
  };
}

// Inserted out of order on purpose. Ids do not sort like names; two
// actors share a name; one has none; one is shared between the players.
function world(): State {
  const st = newState();
  const add = (a: Actor): void => { st.Actors[a.actorId] = a; };
  add(actor("zz-9", "", [ANN]));
  add(actor("a-2", "Abe", [ANN]));
  add(actor("a-1", "Zed", [ANN]));
  add(actor("q-1", "Quinn", [BO]));
  add(actor("a-0", "Abe", [BO, ANN]));
  add(actor("n-1", "Nobody's", []));
  add(actor("a-3", "Mira", [ANN]));
  return st;
}

const ANN_QUESTION =
  "Make Ann a spectator? They no longer control Abe and Abe and Mira and Zed and zz-9.";
const BO_QUESTION = "Make Bo a spectator? They no longer control Abe and Quinn.";

interface Console {
  node: HTMLElement;
  asked: string[];
  sent: ClientCommand[];
  order: string[];
  refreshes: () => number;
  release: (answer?: CommandResult) => void;
}

function openConsole(answer: boolean, st: State = world()): Console {
  const asked: string[] = [];
  const sent: ClientCommand[] = [];
  const order: string[] = [];
  let refreshes = 0;
  const h = held();
  const node = renderDMConsole({
    st,
    participants: presence(),
    adventures: [],
    maps: [],
    abilities: [],
    guideFor: async () => null,
    joinLink: null,
    roster: roster(),
    origin: "http://table.example",
    refreshSharing: () => { refreshes++; order.push("refresh"); },
    send: (c) => { sent.push(c); order.push("send"); return h.send(c); },
    rerender: () => {},
    notify: () => {},
    confirm: (msg) => { asked.push(msg); order.push("confirm"); return answer; },
    doorsArmed: false,
    toggleDoors: () => {},
  });
  return { node, asked, sent, order, refreshes: () => refreshes, release: h.release };
}

function rowOf(node: HTMLElement, id: string): HTMLElement {
  const row = node.querySelector<HTMLElement>(`[data-participant="${id}"]`);
  if (!row) throw new Error(`no roster row for ${id}`);
  return row;
}

function controlsOf(row: HTMLElement): string[] {
  return Array.from(row.querySelectorAll("button, select, input, textarea, a"))
    .map((c) => (c.textContent ?? "").trim())
    .sort();
}

function makeSpectator(node: HTMLElement, id: string): void {
  const b = Array.from(rowOf(node, id).querySelectorAll<HTMLButtonElement>("button"))
    .find((x) => (x.textContent ?? "").trim() === "Make spectator");
  if (!b) throw new Error(`no Make spectator on ${id}'s row`);
  b.click();
}

describe("the console's Who may do what", () => {
  test("VTT-905: a spectator's row offers Boot and nothing else", () => {
    const c = openConsole(false);
    const row = rowOf(c.node, SID);
    expect(row.textContent).toContain("Sid");
    expect(controlsOf(row)).toEqual(["Boot"]);
  });

  test("VTT-905: each player's row offers Make spectator and Boot", () => {
    const c = openConsole(false);
    for (const [id, name] of [[ANN, "Ann"], [BO, "Bo"]] as const) {
      const row = rowOf(c.node, id);
      expect(row.textContent).toContain(name);
      expect(controlsOf(row)).toEqual(["Boot", "Make spectator"]);
    }
  });

  test("VTT-905: no control anywhere in the console is a promotion to player", () => {
    const c = openConsole(false);
    const promote = Array.from(c.node.querySelectorAll("[data-action]"))
      .map((e) => e.getAttribute("data-action") ?? "")
      .filter((a) => a.startsWith("promote-"));
    expect(promote).toEqual([]);
    const texts = Array.from(c.node.querySelectorAll("button"))
      .map((b) => (b.textContent ?? "").trim().toLowerCase());
    expect(texts.filter((t) => t.includes("make player") || t.includes("promote")))
      .toEqual([]);
    for (const id of [DM, AGENT, ANN, BO, SID]) {
      const row = c.node.querySelector(`[data-participant="${id}"]`);
      if (!row) continue;
      expect(Array.from(row.querySelectorAll("[data-action]"))
        .some((e) => (e.getAttribute("data-action") ?? "").startsWith("promote-")))
        .toBe(false);
    }
  });
});

describe("the console's Make spectator", () => {
  test("VTT-901 VTT-902: the question names the roster's name and the actors, sorted", () => {
    const c = openConsole(false);
    makeSpectator(c.node, ANN);
    expect(c.asked).toEqual([ANN_QUESTION]);
  });

  test("VTT-901 VTT-902: a shared actor is named in the other player's question too", () => {
    const c = openConsole(false);
    makeSpectator(c.node, BO);
    expect(c.asked).toEqual([BO_QUESTION]);
  });

  test("VTT-902: equal names are each named, and an unnamed actor sorts by its id", () => {
    const st = newState();
    st.Actors["m-9"] = actor("m-9", "Twin", [ANN]);
    st.Actors["m-1"] = actor("m-1", "Twin", [ANN]);
    st.Actors["m-5"] = actor("m-5", "Twin", [ANN]);
    const c = openConsole(false, st);
    makeSpectator(c.node, ANN);
    expect(c.asked).toEqual(["Make Ann a spectator? They no longer control Twin and Twin and Twin."]);
    // A tie's order is invisible in the text. An unnamed actor whose id
    // sorts between two names pins sorting by what it is shown by.
    const st2 = newState();
    st2.Actors["b-2"] = actor("b-2", "Bard", [ANN]);
    st2.Actors["Cleric"] = actor("Cleric", "", [ANN]);
    st2.Actors["a-1"] = actor("a-1", "Druid", [ANN]);
    const c2 = openConsole(false, st2);
    makeSpectator(c2.node, ANN);
    expect(c2.asked).toEqual(["Make Ann a spectator? They no longer control Bard and Cleric and Druid."]);
  });

  test("VTT-902: one actor is named alone, with no joiner", () => {
    const st = newState();
    st.Actors["solo"] = actor("solo", "Wren", [ANN]);
    st.Actors["other"] = actor("other", "Kai", [BO]);
    const c = openConsole(false, st);
    makeSpectator(c.node, ANN);
    expect(c.asked).toEqual(["Make Ann a spectator? They no longer control Wren."]);
  });

  test("VTT-901: a declined question sends nothing", async () => {
    const c = openConsole(false);
    makeSpectator(c.node, ANN);
    await settle();
    expect(c.asked.length).toBe(1);
    expect(c.sent).toEqual([]);
  });

  test("VTT-901 VTT-903: a confirmed question is asked first, then one promotion to spectator is sent", async () => {
    const c = openConsole(true);
    makeSpectator(c.node, ANN);
    await settle();
    expect(c.order.slice(0, 2)).toEqual(["confirm", "send"]);
    expect(c.sent.length).toBe(1);
    const cmd = c.sent[0]!;
    expect(cmd.command.case).toBe("promoteParticipant");
    if (cmd.command.case !== "promoteParticipant") throw new Error("unreachable");
    expect(cmd.command.value.participantId).toBe(ANN);
    expect(cmd.command.value.role).toBe("spectator");
    c.release(ok());
    await settle();
    expect(c.sent.length).toBe(1);
  });

  test("VTT-903: the confirmed promotion names the player whose row was pressed", async () => {
    const c = openConsole(true);
    makeSpectator(c.node, BO);
    await settle();
    expect(c.sent.length).toBe(1);
    const v = c.sent[0]!.command;
    expect(v.case).toBe("promoteParticipant");
    if (v.case !== "promoteParticipant") throw new Error("unreachable");
    expect(v.value.participantId).toBe(BO);
    expect(v.value.role).toBe("spectator");
    c.release(ok());
    await settle();
  });

  test("VTT-903: promoteParticipant names spectator", () => {
    const cmd = promoteParticipant("p-x");
    expect(cmd.command.case).toBe("promoteParticipant");
    if (cmd.command.case !== "promoteParticipant") throw new Error("unreachable");
    expect(cmd.command.value.participantId).toBe("p-x");
    expect(cmd.command.value.role).toBe("spectator");
  });

  test("VTT-904: the roster is read again when the promotion is answered, not when it is sent", async () => {
    const c = openConsole(true);
    makeSpectator(c.node, ANN);
    expect(c.sent.length).toBe(1);
    expect(c.refreshes()).toBe(0);
    await settle();
    await settle();
    expect(c.refreshes()).toBe(0);
    c.release(ok());
    await settle();
    expect(c.refreshes()).toBe(1);
    expect(c.order).toEqual(["confirm", "send", "refresh"]);
  });

  test("VTT-904: a refused promotion is answered too, and the roster is read again", async () => {
    const c = openConsole(true);
    makeSpectator(c.node, ANN);
    await settle();
    expect(c.refreshes()).toBe(0);
    c.release(refused("not allowed"));
    await settle();
    expect(c.refreshes()).toBe(1);
  });

  test("VTT-901: a declined question asks for no roster read", async () => {
    const c = openConsole(false);
    makeSpectator(c.node, ANN);
    await settle();
    expect(c.refreshes()).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// The window
// ---------------------------------------------------------------------------

const ORIGIN = "http://localhost:8080";
const ME = "p-me";
const g = globalThis as unknown as Record<string, unknown>;
const startURL = location.href;
const realFetch = g["fetch"];
const realWebSocket = g["WebSocket"];

class FakeSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  readyState = 0;
  sent: string[] = [];
  onopen: ((e: Event) => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e: unknown) => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  constructor(readonly url: string) { sockets.push(this); }
  send(d: string): void { this.sent.push(d); }
  close(): void { this.readyState = 3; }
  addEventListener(): void {}
  removeEventListener(): void {}
}
let sockets: FakeSocket[] = [];

type MeAnswer =
  | { kind: "role"; role: string }
  | { kind: "pending" }
  | { kind: "fail" };

interface Win {
  root: HTMLElement;
  ws: FakeSocket;
  fetched: string[];
  setMe: (a: MeAnswer) => void;
  frame: (f: JsonValue) => void;
  ev: (body: Record<string, JsonValue>) => void;
  rulesetReads: () => number;
  meReads: () => number;
  close: () => void;
}

let opened: Win[] = [];
let tokenSeq = 0;

async function openWindow(me: MeAnswer): Promise<Win> {
  const token = `tok-qa-perch-${++tokenSeq}`;
  const happy = g["happyDOM"] as { setURL: (u: string) => void };
  happy.setURL(`${ORIGIN}/?token=${token}`);
  let answer = me;
  const fetched: string[] = [];
  g["fetch"] = async (u: unknown): Promise<Response> => {
    const path = new URL(String(u), ORIGIN).pathname;
    fetched.push(path);
    if (path === "/api/me") {
      if (answer.kind === "pending") return new Promise<Response>(() => {});
      if (answer.kind === "fail") return new Response("down", { status: 500 });
      return Response.json({ role: answer.role, participantId: ME, displayName: "Me" });
    }
    if (path === "/api/ruleset") return Response.json({ abilities: [] });
    if (path === "/api/adventures") return Response.json([]);
    if (path === "/api/maps") return Response.json({ maps: [] });
    return new Response("{}", { status: 404 });
  };
  g["WebSocket"] = FakeSocket;
  sockets = [];
  const root = document.createElement("div");
  document.body.appendChild(root);
  const session = boot(root);
  if (!session) throw new Error("boot drew no table");
  await settle();
  const ws = sockets[sockets.length - 1];
  if (!ws) throw new Error("boot opened no socket");
  ws.readyState = 1;
  ws.onopen?.(new Event("open"));
  let seq = 0;
  const frame = (f: JsonValue): void => { ws.onmessage?.({ data: JSON.stringify(f) }); };
  const w: Win = {
    root, ws, fetched,
    setMe: (a) => { answer = a; },
    frame,
    ev: (body) => {
      seq++;
      frame({ event: { eventId: `e-${seq}`, sequence: String(seq), ...body } });
    },
    rulesetReads: () => fetched.filter((p) => p === "/api/ruleset").length,
    meReads: () => fetched.filter((p) => p === "/api/me").length,
    close: () => {
      session.close();
      root.remove();
      try {
        if (localStorage.getItem("vtt.token") === token) localStorage.removeItem("vtt.token");
        sessionStorage.removeItem(`vtt.viewpoint.${token}`);
      } catch { /* storage is optional */ }
    },
  };
  opened.push(w);
  await settle();
  return w;
}

afterEach(() => {
  for (const w of opened) w.close();
  opened = [];
  g["fetch"] = realFetch;
  g["WebSocket"] = realWebSocket;
});

afterAll(() => {
  (g["happyDOM"] as { setURL: (u: string) => void }).setURL(startURL);
});

function sentCommands(w: Win, from = 0): ClientCommand[] {
  return w.ws.sent.slice(from).map((raw) => fromJson(ClientCommandSchema, JSON.parse(raw) as JsonValue));
}

// A switch button is one whose press sends set_viewpoint: press every
// button in the window and keep the ones that do, with whom they name.
async function switchButtons(w: Win): Promise<{ label: string; target: string }[]> {
  const out: { label: string; target: string }[] = [];
  const count = w.root.querySelectorAll("button").length;
  for (let i = 0; i < count; i++) {
    const b = w.root.querySelectorAll<HTMLButtonElement>("button")[i];
    if (!b) break;
    const label = (b.textContent ?? "").trim();
    const before = w.ws.sent.length;
    b.click();
    await settle();
    for (const c of sentCommands(w, before)) {
      if (c.command.case === "setViewpoint") out.push({ label, target: c.command.value.actorId });
    }
  }
  return out;
}

function targets(list: { target: string }[]): string[] {
  return list.map((x) => x.target).sort();
}

function perches(w: Win): boolean {
  return (w.root.textContent ?? "").includes("Whose eyes") || !!w.root.querySelector(".perch");
}

function hasPlayerPanel(w: Win): boolean {
  return !!w.root.querySelector("section.player");
}

const OLD_LINE = "You play no character; the DM can give you one.";

const SET = {
  actors: [
    { actorId: "act-b", name: "Bree" },
    { actorId: "act-c", name: "Cy" },
    { actorId: "act-u" },
  ],
};

describe("a window's perch", () => {
  test("VTT-917: a spectator's window perches, a button per party member and one for nobody, no panel", async () => {
    const w = await openWindow({ kind: "role", role: "spectator" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: SET });
    await settle();
    expect(perches(w)).toBe(true);
    expect(w.root.textContent).toContain("Whose eyes");
    expect(hasPlayerPanel(w)).toBe(false);
    expect(w.root.querySelector('[data-field="say-text"]')).toBeNull();
    expect(targets(await switchButtons(w))).toEqual(["", "act-b", "act-c", "act-u"]);
    expect(w.root.textContent).not.toContain(OLD_LINE);
  });

  test("VTT-917: a spectator's window perches whatever the set's flag says", async () => {
    const w = await openWindow({ kind: "role", role: "spectator" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: { ...SET, controlsNoPartyMember: true } });
    await settle();
    expect(perches(w)).toBe(true);
    expect(hasPlayerPanel(w)).toBe(false);
    expect(targets(await switchButtons(w))).toEqual(["", "act-b", "act-c", "act-u"]);
    w.frame({ watchableChanged: { ...SET, controlsNoPartyMember: false } });
    await settle();
    expect(perches(w)).toBe(true);
    expect(hasPlayerPanel(w)).toBe(false);
    expect(w.root.textContent).not.toContain(OLD_LINE);
  });

  test("VTT-917: a spectator's window perches before any set arrives, offering nobody alone", async () => {
    const w = await openWindow({ kind: "role", role: "spectator" });
    w.frame({ viewpointChanged: {} });
    await settle();
    expect(perches(w)).toBe(true);
    expect(hasPlayerPanel(w)).toBe(false);
    expect(targets(await switchButtons(w))).toEqual([""]);
  });

  test("VTT-917: a player's window never perches, the flag false or true", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: SET });
    await settle();
    expect(perches(w)).toBe(false);
    expect(hasPlayerPanel(w)).toBe(true);
    w.frame({ watchableChanged: { ...SET, controlsNoPartyMember: true } });
    await settle();
    expect(perches(w)).toBe(false);
    expect(hasPlayerPanel(w)).toBe(true);
    expect(w.root.textContent).not.toContain(OLD_LINE);
    expect(targets(await switchButtons(w))).not.toContain("");
  });

  test("VTT-917: a player's window whose set names none and is flagged draws the panel", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: { controlsNoPartyMember: true } });
    await settle();
    expect(perches(w)).toBe(false);
    expect(hasPlayerPanel(w)).toBe(true);
    expect(w.root.textContent).not.toContain(OLD_LINE);
  });

  test("VTT-917: a player's last party member taken back leaves the panel, no perch and no old line", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree" }] } });
    await settle();
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree" }], controlsNoPartyMember: true } });
    await settle();
    expect(perches(w)).toBe(false);
    expect(hasPlayerPanel(w)).toBe(true);
    expect(w.root.textContent).not.toContain(OLD_LINE);
  });

  test("VTT-917: the window follows the role last read: spectator, then player, then spectator", async () => {
    const w = await openWindow({ kind: "role", role: "spectator" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: SET });
    await settle();
    expect(perches(w)).toBe(true);
    w.setMe({ kind: "role", role: "player" });
    w.frame({ presenceChanged: { participantId: ME, displayName: "Me", state: "PRESENCE_STATE_CONNECTED" } });
    await settle();
    await settle();
    expect(perches(w)).toBe(false);
    expect(hasPlayerPanel(w)).toBe(true);
    w.setMe({ kind: "role", role: "spectator" });
    w.frame({ presenceChanged: { participantId: ME, displayName: "Me", state: "PRESENCE_STATE_CONNECTED" } });
    await settle();
    await settle();
    expect(perches(w)).toBe(true);
    expect(hasPlayerPanel(w)).toBe(false);
    expect(w.root.textContent).not.toContain(OLD_LINE);
  });

  test("VTT-917: while a re-read of the role is pending the window keeps the role last read", async () => {
    const w = await openWindow({ kind: "role", role: "spectator" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: SET });
    await settle();
    w.setMe({ kind: "pending" });
    w.frame({ presenceChanged: { participantId: ME, displayName: "Me", state: "PRESENCE_STATE_CONNECTED" } });
    await settle();
    expect(w.meReads()).toBe(2);
    expect(perches(w)).toBe(true);
  });

  for (const role of ["dm", "agent"]) {
    test(`VTT-917: a${role === "agent" ? "n" : ""} ${role}'s window sent a set does not perch`, async () => {
      const w = await openWindow({ kind: "role", role });
      w.frame({ watchableChanged: { ...SET, controlsNoPartyMember: true } });
      await settle();
      expect(w.root.textContent).toContain("DM console");
      expect(perches(w)).toBe(false);
      expect(w.root.textContent).not.toContain("No shoulder");
      expect(w.root.textContent).not.toContain(OLD_LINE);
    });
  }

  test("VTT-917: a window whose role has not been read does not perch", async () => {
    const w = await openWindow({ kind: "pending" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: { ...SET, controlsNoPartyMember: true } });
    await settle();
    expect(w.root.textContent).toContain("Story");
    expect(perches(w)).toBe(false);
    expect(w.root.textContent).not.toContain(OLD_LINE);
  });

  test("VTT-917: a window whose first read of the role failed does not perch", async () => {
    const w = await openWindow({ kind: "fail" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: { ...SET, controlsNoPartyMember: true } });
    await settle();
    expect(w.root.textContent).toContain("Story");
    expect(perches(w)).toBe(false);
  });

  test("VTT-917: a spectator's window showing why it cannot fold its log draws no perch", async () => {
    const w = await openWindow({ kind: "role", role: "spectator" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: SET });
    await settle();
    expect(perches(w)).toBe(true);
    w.ev({ actorAdded: { actor: { actorId: "act-x", name: "X", kind: "ACTOR_KIND_PARTY_MEMBER", controllerIds: ["p-1"] } } });
    await settle();
    expect(w.root.textContent).toContain("Cannot derive state");
    expect(perches(w)).toBe(false);
    expect(await switchButtons(w)).toEqual([]);
  });
});

describe("a player's switch buttons", () => {
  test("VTT-918: none before the window is sent a set", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: {} });
    await settle();
    expect(await switchButtons(w)).toEqual([]);
    expect(perches(w)).toBe(false);
  });

  test("VTT-918: exactly the party members the last set names", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: SET });
    await settle();
    expect(targets(await switchButtons(w))).toEqual(["act-b", "act-c", "act-u"]);
    w.frame({ watchableChanged: { actors: [{ actorId: "act-c", name: "Cy" }, { actorId: "act-d", name: "Dee" }] } });
    await settle();
    expect(targets(await switchButtons(w))).toEqual(["act-c", "act-d"]);
  });

  test("VTT-918: exactly the party members a flagged set names", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: { ...SET, controlsNoPartyMember: true } });
    await settle();
    expect(targets(await switchButtons(w))).toEqual(["act-b", "act-c", "act-u"]);
  });

  test("VTT-918: none for a set that names none", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: SET });
    await settle();
    w.frame({ watchableChanged: {} });
    await settle();
    expect(await switchButtons(w)).toEqual([]);
  });

  test("VTT-918: none while the window shows why it cannot fold its log", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: SET });
    await settle();
    w.ev({ actorAdded: { actor: { actorId: "act-x", name: "X", kind: "ACTOR_KIND_PARTY_MEMBER", controllerIds: ["p-1"] } } });
    await settle();
    expect(w.root.textContent).toContain("Cannot derive state");
    expect(await switchButtons(w)).toEqual([]);
  });
});

// A scene with the watched one's token, a listed member's and an
// unlisted party member's.
function seedBoard(w: Win): void {
  w.ev({ sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 4, gridHeight: 3 } });
  for (const [id, name] of [["act-b", "Bree"], ["act-c", "Cy"], ["act-z", "Zia"]]) {
    w.ev({ actorAdded: { actor: { actorId: id!, name: name!, kind: "ACTOR_KIND_PARTY_MEMBER" } } });
  }
  w.ev({ actorControlGranted: { actorId: "act-b", participantId: ME } });
  w.ev({ actorControlGranted: { actorId: "act-c", participantId: ME } });
  w.ev({ actorControlGranted: { actorId: "act-z", participantId: "p-someone" } });
  w.ev({ tokenPlaced: { tokenId: "t-b", sceneId: "s1", actorId: "act-b", position: { x: 0, y: 0 } } });
  w.ev({ tokenPlaced: { tokenId: "t-c", sceneId: "s1", actorId: "act-c", position: { x: 2, y: 1 } } });
  w.ev({ tokenPlaced: { tokenId: "t-z", sceneId: "s1", actorId: "act-z", position: { x: 3, y: 2 } } });
}

function clickToken(w: Win, tokenId: string): void {
  const tok = w.root.querySelector<HTMLElement>(`[data-token-id="${tokenId}"]`);
  if (!tok) throw new Error(`no token ${tokenId} on the board`);
  const grid = w.root.querySelector<HTMLElement>(".grid");
  if (!grid) throw new Error("no board");
  const x = parseFloat(tok.style.left) + parseFloat(tok.style.width) / 2;
  const y = parseFloat(tok.style.top) + parseFloat(tok.style.height) / 2;
  grid.dispatchEvent(new MouseEvent("click", { clientX: x, clientY: y, bubbles: true }));
}

function viewpointsSent(w: Win, from: number): string[] {
  return sentCommands(w, from).flatMap((c) =>
    c.command.case === "setViewpoint" ? [c.command.value.actorId] : []);
}

describe("a player's board click", () => {
  for (const flag of [false, true]) {
    test(`VTT-940: on a listed party member's token switches to it (flag ${flag})`, async () => {
      const w = await openWindow({ kind: "role", role: "player" });
      w.frame({ viewpointChanged: { actorId: "act-b" } });
      w.frame({
        watchableChanged: {
          actors: [{ actorId: "act-b", name: "Bree" }, { actorId: "act-c", name: "Cy" }],
          controlsNoPartyMember: flag,
        },
      });
      seedBoard(w);
      await settle();
      const before = w.ws.sent.length;
      clickToken(w, "t-c");
      await settle();
      expect(viewpointsSent(w, before)).toEqual(["act-c"]);
    });
  }

  test("VTT-940: on an unlisted party member's token does not switch to it", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree" }, { actorId: "act-c", name: "Cy" }] } });
    seedBoard(w);
    await settle();
    const before = w.ws.sent.length;
    clickToken(w, "t-z");
    await settle();
    expect(viewpointsSent(w, before)).toEqual([]);
    // The click reached the board: it moved the watched one instead (VTT-408).
    expect(sentCommands(w, before).map((c) => c.command.case)).toEqual(["moveToken"]);
  });

  test("VTT-940: on the watched one's own token sends no switch", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree" }, { actorId: "act-c", name: "Cy" }] } });
    seedBoard(w);
    await settle();
    const before = w.ws.sent.length;
    clickToken(w, "t-b");
    await settle();
    expect(sentCommands(w, before)).toEqual([]);
  });
});

describe("a window's ruleset reads", () => {
  test("VTT-920: a player's window reads the ruleset again when the set's actors change", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree" }] } });
    await settle();
    const base = w.rulesetReads();
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree" }, { actorId: "act-c", name: "Cy" }] } });
    await settle();
    expect(w.rulesetReads()).toBe(base + 1);
    w.frame({ watchableChanged: { actors: [{ actorId: "act-c", name: "Cy" }] } });
    await settle();
    expect(w.rulesetReads()).toBe(base + 2);
  });

  test("VTT-920: an equal set issues no read", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree" }] } });
    await settle();
    const base = w.rulesetReads();
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree" }] } });
    await settle();
    expect(w.rulesetReads()).toBe(base);
  });

  test("VTT-732: a player's window reads the ruleset again when a party member's ability ids change", async () => {
    const w = await openWindow({ kind: "role", role: "player" });
    w.frame({ viewpointChanged: { actorId: "act-b" } });
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree", abilityIds: ["a1"] }] } });
    await settle();
    const base = w.rulesetReads();
    w.frame({ watchableChanged: { actors: [{ actorId: "act-b", name: "Bree", abilityIds: ["a1", "a2"] }] } });
    await settle();
    await settle();
    expect(w.rulesetReads()).toBe(base + 1);
  });

  test("VTT-712: a window reads the ruleset again when the role read makes it a player's and stops it", async () => {
    const w = await openWindow({ kind: "role", role: "spectator" });
    w.frame({ viewpointChanged: {} });
    w.frame({ watchableChanged: SET });
    await settle();
    const base = w.rulesetReads();
    expect(base).toBeGreaterThanOrEqual(1);
    w.setMe({ kind: "role", role: "player" });
    w.frame({ presenceChanged: { participantId: ME, displayName: "Me", state: "PRESENCE_STATE_CONNECTED" } });
    await settle();
    await settle();
    expect(w.meReads()).toBe(2);
    expect(w.rulesetReads()).toBe(base + 1);
    w.setMe({ kind: "role", role: "spectator" });
    w.frame({ presenceChanged: { participantId: ME, displayName: "Me", state: "PRESENCE_STATE_CONNECTED" } });
    await settle();
    await settle();
    expect(w.meReads()).toBe(3);
    expect(w.rulesetReads()).toBe(base + 2);
  });
});
