import "./support/dom";
import { afterEach, describe, expect, test } from "bun:test";
import { create, fromJsonString, toJsonString, type MessageInitShape } from "@bufbuild/protobuf";
import {
  ClientCommandSchema, PresenceState, ServerFrameSchema, type ClientCommand,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind, EnvelopeSchema } from "../../contract/gen/ts/vtt/v1/events_pb";
import { boot } from "../src/app";
import type { Ability } from "../src/metadata";
import type { Session } from "../src/session";

type QaWinRole = "player" | "dm" | "agent" | "spectator";
type QaWinFrameInit = NonNullable<MessageInitShape<typeof ServerFrameSchema>["frame"]>;
type QaWinPayload = NonNullable<MessageInitShape<typeof EnvelopeSchema>["payload"]>;

const qaWinAbilities: Record<string, Ability> = {
  slash: { id: "slash", name: "Slash", range: 1, maxTargets: 1, usage: { kind: "atWill" } },
  heal: { id: "heal", name: "Heal", range: 3, maxTargets: 1, usage: { kind: "atWill" } },
  bolt: { id: "bolt", name: "Bolt", range: 5, maxTargets: 1, usage: { kind: "atWill" } },
  dart: { id: "dart", name: "Dart", range: 5, maxTargets: 1, usage: { kind: "atWill" } },
};

function qaWinList(...ids: string[]): Ability[] {
  return ids.map((id) => qaWinAbilities[id]!);
}

class QaWinSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static made: QaWinSocket[] = [];
  readonly url: string;
  readyState = 0;
  binaryType = "blob";
  sent: string[] = [];
  onopen: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  private listeners = new Map<string, ((e: unknown) => void)[]>();

  constructor(url: string) {
    this.url = url;
    QaWinSocket.made.push(this);
  }

  addEventListener(type: string, fn: (e: unknown) => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
  }

  removeEventListener(type: string, fn: (e: unknown) => void): void {
    this.listeners.set(type, (this.listeners.get(type) ?? []).filter((f) => f !== fn));
  }

  send(data: string): void {
    this.sent.push(data);
  }

  close(): void {
    this.readyState = 3;
  }

  qaWinFire(type: "open" | "message", e: unknown): void {
    const own = (this as unknown as Record<string, ((ev: unknown) => void) | null>)["on" + type];
    if (own) own(e);
    for (const fn of this.listeners.get(type) ?? []) fn(e);
  }

  qaWinOpen(): void {
    this.readyState = 1;
    this.qaWinFire("open", new Event("open"));
  }

  qaWinFrame(frame: QaWinFrameInit): void {
    const data = toJsonString(ServerFrameSchema, create(ServerFrameSchema, { frame }));
    this.qaWinFire("message", { data });
  }

  qaWinCommands(): ClientCommand[] {
    return this.sent.map((s) => fromJsonString(ClientCommandSchema, s));
  }
}

interface QaWinRead {
  path: string;
  auth: string;
  answer: (body: unknown, status?: number) => void;
}

interface QaWinWindow {
  root: HTMLElement;
  session: Session;
  socket: QaWinSocket;
  token: string;
  reads: QaWinRead[];
  answerMe: () => void;
  promote: (role: QaWinRole) => void;
}

const qaWinGlobals = globalThis as unknown as Record<string, unknown>;
const qaWinNativeFetch = qaWinGlobals["fetch"];
const qaWinNativeSocket = qaWinGlobals["WebSocket"];
const qaWinDom = (globalThis as unknown as { happyDOM: { setURL(url: string): void } }).happyDOM;
const qaWinStartURL = location.href;
let qaWinCount = 0;
let qaWinOpenWindows: QaWinWindow[] = [];

function qaWinStorageKeys(): { local: Set<string>; session: Set<string> } {
  const keys = (s: Storage) => new Set(Array.from({ length: s.length }, (_, i) => s.key(i) ?? ""));
  return { local: keys(localStorage), session: keys(sessionStorage) };
}

let qaWinBefore = qaWinStorageKeys();

afterEach(() => {
  for (const w of qaWinOpenWindows) w.session.close();
  qaWinOpenWindows = [];
  qaWinGlobals["fetch"] = qaWinNativeFetch;
  qaWinGlobals["WebSocket"] = qaWinNativeSocket;
  QaWinSocket.made = [];
  const now = qaWinStorageKeys();
  for (const k of now.local) if (!qaWinBefore.local.has(k)) localStorage.removeItem(k);
  for (const k of now.session) if (!qaWinBefore.session.has(k)) sessionStorage.removeItem(k);
  qaWinBefore = qaWinStorageKeys();
  document.body.innerHTML = "";
  qaWinDom.setURL(qaWinStartURL);
});

function qaWinJSON(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function qaWinBoot(role: QaWinRole, participantId: string): QaWinWindow {
  qaWinCount++;
  const token = `qa-win-${qaWinCount}-${role}`;
  const reads: QaWinRead[] = [];
  let meAnswer: (() => void) | null = null;
  let meAsked = false;
  let meReady = false;
  const me: { participantId: string; name: string; role: QaWinRole } = { participantId, name: "Pia", role };
  qaWinGlobals["fetch"] = (input: unknown, init?: RequestInit): Promise<Response> => {
    const url = new URL(String(input));
    const headers = new Headers(init?.headers);
    if (url.pathname === "/api/me") {
      return new Promise<Response>((resolve) => {
        meAsked = true;
        meAnswer = () => resolve(qaWinJSON(me));
        if (meReady) meAnswer();
      });
    }
    if (url.pathname === "/api/ruleset") {
      return new Promise<Response>((resolve) => {
        reads.push({
          path: url.pathname,
          auth: headers.get("Authorization") ?? "",
          answer: (body, status = 200) => resolve(qaWinJSON(body, status)),
        });
      });
    }
    if (url.pathname === "/api/adventures") return Promise.resolve(qaWinJSON([]));
    if (url.pathname === "/api/maps") return Promise.resolve(qaWinJSON({ maps: [], cellPx: 32 }));
    return Promise.resolve(new Response("not here", { status: 404 }));
  };
  qaWinGlobals["WebSocket"] = QaWinSocket;
  qaWinDom.setURL(`http://qa.test/?token=${token}`);
  const root = document.createElement("div");
  document.body.appendChild(root);
  const session = boot(root);
  if (!session) throw new Error("boot drew no table");
  const socket = QaWinSocket.made[QaWinSocket.made.length - 1];
  if (!socket) throw new Error("boot opened no socket");
  const w: QaWinWindow = {
    root, session, socket, token, reads,
    answerMe: () => {
      meReady = true;
      if (meAsked && meAnswer) meAnswer();
    },
    promote: (next) => {
      me.role = next;
    },
  };
  qaWinOpenWindows.push(w);
  return w;
}

async function qaWinFlush(): Promise<void> {
  for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0));
}

function qaWinRuleset(abilities: Ability[]): unknown {
  return { id: "qa-rules", name: "QA Rules", abilities, conditions: [], resources: [], properties: [] };
}

function qaWinSet(actors: [string, string][], controlsNoPartyMember = false): QaWinFrameInit {
  return {
    case: "watchableChanged",
    value: { actors: actors.map(([actorId, name]) => ({ actorId, name })), controlsNoPartyMember },
  };
}

function qaWinViewpoint(actorId: string): QaWinFrameInit {
  return { case: "viewpointChanged", value: { actorId } };
}

function qaWinEvent(seq: number, payload: QaWinPayload): QaWinFrameInit {
  return { case: "event", value: { eventId: `ev-${seq}`, sequence: BigInt(seq), payload } };
}

function qaWinActorAdded(actorId: string, name: string, abilityIds: string[]): QaWinPayload {
  return {
    case: "actorAdded",
    value: { actor: { actorId, name, moduleId: "qa", kind: ActorKind.PARTY_MEMBER, abilityIds } },
  };
}

function qaWinGranted(actorId: string, participantId: string): QaWinPayload {
  return { case: "actorControlGranted", value: { actorId, participantId, kind: ActorKind.PARTY_MEMBER } };
}

function qaWinLogOf(socket: QaWinSocket, actors: [string, string, string[], string[]][]): void {
  let seq = 1;
  socket.qaWinFrame(qaWinEvent(seq++, { case: "sceneCreated", value: { sceneId: "s1", name: "Hall", gridWidth: 8, gridHeight: 8 } }));
  actors.forEach(([id, name, controllers, abilityIds], i) => {
    socket.qaWinFrame(qaWinEvent(seq++, qaWinActorAdded(id, name, abilityIds)));
    for (const p of controllers) socket.qaWinFrame(qaWinEvent(seq++, qaWinGranted(id, p)));
    socket.qaWinFrame(qaWinEvent(seq++, {
      case: "tokenPlaced",
      value: { tokenId: `t-${id}`, sceneId: "s1", actorId: id, position: { x: 1 + i, y: 1 }, name },
    }));
  });
}

function qaWinOpening(socket: QaWinSocket, set: QaWinFrameInit, viewpoint: string | null): void {
  socket.qaWinOpen();
  socket.qaWinFrame({ case: "catchUpHead", value: { headSequence: 0n } });
  socket.qaWinFrame({ case: "presenceSnapshot", value: {} });
  socket.qaWinFrame(set);
  if (viewpoint !== null) socket.qaWinFrame(qaWinViewpoint(viewpoint));
}

function qaWinPanel(root: HTMLElement): HTMLElement | null {
  return root.querySelector<HTMLElement>("section.player");
}

function qaWinOffered(root: HTMLElement): string[] {
  const panel = qaWinPanel(root);
  if (!panel) return [];
  const heading = Array.from(panel.querySelectorAll("h3")).find((h) => h.textContent === "Abilities");
  const row = heading?.nextElementSibling;
  if (!row) return [];
  return Array.from(row.querySelectorAll("button")).map((b) => b.textContent ?? "");
}

async function qaWinPlayerOn(
  actors: [string, string, string[], string[]][],
  set: [string, string][],
  viewpoint: string,
  firstAnswer: Ability[],
): Promise<QaWinWindow> {
  const w = qaWinBoot("player", "p1");
  qaWinOpening(w.socket, qaWinSet(set), viewpoint);
  qaWinLogOf(w.socket, actors);
  w.answerMe();
  await qaWinFlush();
  expect(w.reads.length).toBe(1);
  w.reads[0]!.answer(qaWinRuleset(firstAnswer));
  await qaWinFlush();
  return w;
}

describe("a player's window reads /api/ruleset again when its set's key changes", () => {
  // VTT-696
  test("a grant that adds a party member to the set issues one new read", async () => {
    const w = await qaWinPlayerOn([["a", "Ayla", ["p1"], ["slash"]]], [["a", "Ayla"]], "a", qaWinList("slash"));
    expect(qaWinOffered(w.root)).toEqual(["Slash"]);
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
    expect(w.reads[1]!.path).toBe("/api/ruleset");
    expect(w.reads[1]!.auth).toBe(`Bearer ${w.token}`);
  });

  // VTT-696
  test("a revocation that drops a party member from the set issues one new read", async () => {
    const w = await qaWinPlayerOn(
      [["a", "Ayla", ["p1"], ["slash"]], ["b", "Bo", ["p1"], ["heal"]]],
      [["a", "Ayla"], ["b", "Bo"]], "a", qaWinList("slash", "heal"),
    );
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"]]));
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
  });

  // VTT-696
  test("a set whose flag alone changes issues one new read", async () => {
    const w = await qaWinPlayerOn([["a", "Ayla", ["p1"], ["slash"]]], [["a", "Ayla"]], "a", qaWinList("slash"));
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"]], true));
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"]], false));
    await qaWinFlush();
    expect(w.reads.length).toBe(3);
  });

  // VTT-696
  test("a set changed back before the answer issues another read", async () => {
    const w = await qaWinPlayerOn([["a", "Ayla", ["p1"], ["slash"]]], [["a", "Ayla"]], "a", qaWinList("slash"));
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    await qaWinFlush();
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"]]));
    await qaWinFlush();
    expect(w.reads.length).toBe(3);
  });

  // VTT-696
  test("an equal set, a renamed set, a viewpoint and entries issue no read", async () => {
    const w = await qaWinPlayerOn(
      [["a", "Ayla", ["p1"], ["slash"]], ["b", "Bo", ["p1"], ["heal"]]],
      [["a", "Ayla"], ["b", "Bo"]], "a", qaWinList("slash", "heal"),
    );
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla the Bold"], ["b", "Bo"]]));
    w.socket.qaWinFrame(qaWinViewpoint("a"));
    w.socket.qaWinFrame(qaWinEvent(9, qaWinGranted("a", "p1")));
    w.socket.qaWinFrame({ case: "presenceChanged", value: { participantId: "p9", displayName: "Zed", state: PresenceState.CONNECTED } });
    await qaWinFlush();
    expect(w.reads.length).toBe(1);
  });

  // VTT-696
  test("a set equal to the one the newest read was issued under issues no read while it is pending", async () => {
    const w = await qaWinPlayerOn([["a", "Ayla", ["p1"], ["slash"]]], [["a", "Ayla"]], "a", qaWinList("slash"));
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    await qaWinFlush();
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
  });

  // VTT-696
  test("a redial sent the same set issues no read", async () => {
    const w = await qaWinPlayerOn([["a", "Ayla", ["p1"], ["slash"]]], [["a", "Ayla"]], "a", qaWinList("slash"));
    void w.session.reconnect();
    await qaWinFlush();
    const again = QaWinSocket.made[QaWinSocket.made.length - 1]!;
    expect(again).not.toBe(w.socket);
    qaWinOpening(again, qaWinSet([["a", "Ayla"]]), "a");
    qaWinLogOf(again, [["a", "Ayla", ["p1"], ["slash"]]]);
    await qaWinFlush();
    expect(w.reads.length).toBe(1);
    expect(qaWinOffered(w.root)).toEqual(["Slash"]);
  });

  // VTT-696 VTT-712
  test("a spectator's window promoted to player reads again at the promotion and when its set's flag changes", async () => {
    const w = qaWinBoot("spectator", "p1");
    qaWinOpening(w.socket, qaWinSet([["a", "Ayla"]], true), "");
    w.answerMe();
    await qaWinFlush();
    expect(w.reads.length).toBe(1);
    w.reads[0]!.answer(qaWinRuleset([]));
    await qaWinFlush();
    w.promote("player");
    w.socket.qaWinFrame({ case: "presenceChanged", value: { participantId: "p1", displayName: "Pia", state: PresenceState.CONNECTED } });
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"]]));
    w.socket.qaWinFrame(qaWinViewpoint("a"));
    qaWinLogOf(w.socket, [["a", "Ayla", ["p1"], ["slash"]]]);
    await qaWinFlush();
    expect(w.reads.length).toBe(3);
    w.reads[2]!.answer(qaWinRuleset(qaWinList("slash")));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Slash"]);
  });

  for (const role of ["dm", "agent"] as const) {
    // Keep a DM's and an agent's window on its one start-up read.
    test(`a ${role}'s window never reads again`, async () => {
      const w = qaWinBoot(role, "dm1");
      w.socket.qaWinOpen();
      w.socket.qaWinFrame({ case: "catchUpHead", value: { headSequence: 0n } });
      w.socket.qaWinFrame({ case: "presenceSnapshot", value: {} });
      qaWinLogOf(w.socket, [["a", "Ayla", ["dm1"], ["slash"]]]);
      w.answerMe();
      await qaWinFlush();
      expect(w.reads.length).toBe(1);
      w.reads[0]!.answer(qaWinRuleset(qaWinList("slash", "heal")));
      await qaWinFlush();
      expect(qaWinOffered(w.root)).toEqual(["Slash"]);
      w.socket.qaWinFrame(qaWinEvent(9, qaWinActorAdded("b", "Bo", ["heal"])));
      w.socket.qaWinFrame(qaWinEvent(10, qaWinGranted("b", "dm1")));
      w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
      w.socket.qaWinFrame(qaWinSet([], true));
      w.socket.qaWinFrame(qaWinEvent(11, {
        case: "actorControlRevoked", value: { actorId: "a", participantId: "dm1" },
      }));
      await qaWinFlush();
      expect(w.reads.length).toBe(1);
    });
  }
});

describe("a player's window draws from its newest read's answer", () => {
  // VTT-697
  test("the newest answer wins when the older one arrives after it", async () => {
    const w = await qaWinPlayerOn([["a", "Ayla", ["p1"], ["slash", "bolt"]]], [["a", "Ayla"]], "a", qaWinList("slash", "bolt"));
    expect(qaWinOffered(w.root)).toEqual(["Slash", "Bolt"]);
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    await qaWinFlush();
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"]]));
    await qaWinFlush();
    expect(w.reads.length).toBe(3);
    w.reads[2]!.answer(qaWinRuleset(qaWinList("bolt")));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Bolt"]);
    w.reads[1]!.answer(qaWinRuleset(qaWinList("slash", "bolt", "heal")));
    await qaWinFlush();
    w.socket.qaWinFrame(qaWinViewpoint("a"));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Bolt"]);
  });

  // VTT-697
  test("an older answer arriving while the newest read is pending is dropped", async () => {
    const w = await qaWinPlayerOn([["a", "Ayla", ["p1"], ["slash", "bolt"]]], [["a", "Ayla"]], "a", qaWinList("slash", "bolt"));
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    await qaWinFlush();
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"], ["c", "Cid"]]));
    await qaWinFlush();
    expect(w.reads.length).toBe(3);
    w.reads[1]!.answer(qaWinRuleset(qaWinList("bolt")));
    await qaWinFlush();
    w.socket.qaWinFrame(qaWinViewpoint("a"));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Slash", "Bolt"]);
    w.reads[2]!.answer(qaWinRuleset(qaWinList("slash")));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Slash"]);
  });

  // VTT-697
  test("the start-up read's answer arriving after a re-read's is dropped", async () => {
    const w = qaWinBoot("player", "p1");
    qaWinOpening(w.socket, qaWinSet([["a", "Ayla"]]), "a");
    qaWinLogOf(w.socket, [["a", "Ayla", ["p1"], ["slash", "bolt"]]]);
    w.answerMe();
    await qaWinFlush();
    expect(w.reads.length).toBe(1);
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
    w.reads[1]!.answer(qaWinRuleset(qaWinList("bolt")));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Bolt"]);
    w.reads[0]!.answer(qaWinRuleset(qaWinList("slash", "bolt")));
    await qaWinFlush();
    w.socket.qaWinFrame(qaWinViewpoint("a"));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Bolt"]);
  });
});

describe("a change of control reaches the panel without a reload", () => {
  // VTT-696
  test("a perched player granted a character is offered its abilities once the re-read answers", async () => {
    const w = qaWinBoot("player", "p1");
    qaWinOpening(w.socket, qaWinSet([], true), "");
    w.answerMe();
    await qaWinFlush();
    expect(w.reads.length).toBe(1);
    w.reads[0]!.answer(qaWinRuleset([]));
    await qaWinFlush();
    expect(qaWinPanel(w.root)).toBeNull();
    w.socket.qaWinFrame(qaWinSet([["b", "Bo"]]));
    w.socket.qaWinFrame(qaWinViewpoint("b"));
    qaWinLogOf(w.socket, [["b", "Bo", ["p1"], ["dart", "heal"]]]);
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
    expect(qaWinOffered(w.root)).toEqual([]);
    w.reads[1]!.answer(qaWinRuleset(qaWinList("heal", "dart")));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Heal", "Dart"]);
    expect(QaWinSocket.made.length).toBe(1);
  });

  // VTT-696
  test("a player granted a second character is offered its abilities on switching to it", async () => {
    const w = await qaWinPlayerOn([["a", "Ayla", ["p1"], ["slash"]]], [["a", "Ayla"]], "a", qaWinList("slash"));
    expect(qaWinOffered(w.root)).toEqual(["Slash"]);
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"], ["b", "Bo"]]));
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
    w.reads[1]!.answer(qaWinRuleset(qaWinList("slash", "heal")));
    await qaWinFlush();
    const chip = Array.from(qaWinPanel(w.root)?.querySelectorAll("button") ?? []).find((b) => b.textContent === "Bo");
    expect(chip).toBeDefined();
    chip!.click();
    await qaWinFlush();
    const asked = w.socket.qaWinCommands().filter((c) => c.command.case === "setViewpoint");
    expect(asked.length).toBe(1);
    const req = asked[0]!;
    w.socket.qaWinFrame({ case: "result", value: { requestId: req.requestId, ok: true } });
    w.socket.qaWinFrame(qaWinViewpoint("b"));
    qaWinLogOf(w.socket, [["b", "Bo", ["p1"], ["heal"]]]);
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Heal"]);
    expect(QaWinSocket.made.length).toBe(1);
  });

  // VTT-696
  test("a player whose character is taken back is offered what the re-read answers for the one left", async () => {
    const w = await qaWinPlayerOn(
      [["a", "Ayla", ["p1"], ["slash", "heal"]], ["b", "Bo", ["p1"], ["heal"]]],
      [["a", "Ayla"], ["b", "Bo"]], "b", qaWinList("slash", "heal"),
    );
    expect(qaWinOffered(w.root)).toEqual(["Heal"]);
    w.socket.qaWinFrame(qaWinSet([["a", "Ayla"]]));
    w.socket.qaWinFrame(qaWinViewpoint("a"));
    qaWinLogOf(w.socket, [["a", "Ayla", ["p1"], ["slash", "heal"]]]);
    await qaWinFlush();
    expect(w.reads.length).toBe(2);
    w.reads[1]!.answer(qaWinRuleset(qaWinList("slash")));
    await qaWinFlush();
    expect(qaWinOffered(w.root)).toEqual(["Slash"]);
    expect(QaWinSocket.made.length).toBe(1);
  });
});
