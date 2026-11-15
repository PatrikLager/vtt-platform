import "./support/dom";
import { afterEach, expect, test } from "bun:test";
import { create, toJsonString, type MessageInitShape } from "@bufbuild/protobuf";
import { PresenceState, ServerFrameSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind, EnvelopeSchema } from "../../contract/gen/ts/vtt/v1/events_pb";
import { boot } from "../src/app";
import type { Ability } from "../src/metadata";
import type { Session } from "../src/session";

type Role = "player" | "dm" | "spectator";
type FrameInit = NonNullable<MessageInitShape<typeof ServerFrameSchema>["frame"]>;
type PayloadInit = NonNullable<MessageInitShape<typeof EnvelopeSchema>["payload"]>;
type Member = [id: string, name: string, abilityIds: string[]];

const ability = (id: string, name: string): Ability =>
  ({ id, name, range: 1, maxTargets: 1, usage: { kind: "atWill" } });
const SLASH = ability("slash", "Slash");
const HEAL = ability("heal", "Heal");
const BOLT = ability("bolt", "Bolt");
const DART = ability("dart", "Dart");

class NewsSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static made: NewsSocket[] = [];
  readonly url: string;
  readyState = 0;
  binaryType = "blob";
  onopen: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  private listeners = new Map<string, ((e: unknown) => void)[]>();

  constructor(url: string) {
    this.url = url;
    NewsSocket.made.push(this);
  }

  addEventListener(type: string, fn: (e: unknown) => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
  }

  removeEventListener(type: string, fn: (e: unknown) => void): void {
    this.listeners.set(type, (this.listeners.get(type) ?? []).filter((f) => f !== fn));
  }

  send(): void {}

  close(): void {
    this.readyState = 3;
  }

  private fire(type: "open" | "message", e: unknown): void {
    const own = (this as unknown as Record<string, ((ev: unknown) => void) | null>)["on" + type];
    if (own) own(e);
    for (const fn of this.listeners.get(type) ?? []) fn(e);
  }

  open(): void {
    this.readyState = 1;
    this.fire("open", new Event("open"));
  }

  frame(frame: FrameInit): void {
    this.fire("message", { data: toJsonString(ServerFrameSchema, create(ServerFrameSchema, { frame })) });
  }
}

interface NewsWindow {
  root: HTMLElement;
  session: Session;
  socket: NewsSocket;
  reads: ((abilities: Ability[], status?: number) => void)[];
  promote: (role: Role) => void;
}

const globals = globalThis as unknown as Record<string, unknown>;
const nativeFetch = globals["fetch"];
const nativeSocket = globals["WebSocket"];
const dom = (globalThis as unknown as { happyDOM: { setURL(url: string): void } }).happyDOM;
const startURL = location.href;
let made = 0;
let windows: NewsWindow[] = [];

afterEach(() => {
  for (const w of windows) w.session.close();
  windows = [];
  globals["fetch"] = nativeFetch;
  globals["WebSocket"] = nativeSocket;
  NewsSocket.made = [];
  localStorage.clear();
  sessionStorage.clear();
  document.body.innerHTML = "";
  dom.setURL(startURL);
});

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function openWindow(role: Role, meAnswered: Promise<void> = Promise.resolve()): NewsWindow {
  made++;
  const reads: NewsWindow["reads"] = [];
  const me = { participantId: "p1", name: "Pia", role };
  globals["fetch"] = (input: unknown): Promise<Response> => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") return meAnswered.then(() => json(me));
    if (path === "/api/ruleset") {
      return new Promise<Response>((resolve) => {
        reads.push((abilities, status = 200) => resolve(json(
          { id: "r", name: "R", abilities, conditions: [], resources: [], properties: [] }, status)));
      });
    }
    if (path === "/api/adventures") return Promise.resolve(json({ adventures: [] }));
    if (path === "/api/maps") return Promise.resolve(json({ maps: [], cellPx: 32 }));
    return Promise.resolve(new Response("not here", { status: 404 }));
  };
  globals["WebSocket"] = NewsSocket;
  dom.setURL(`http://news.test/?token=news-${made}-${role}`);
  const root = document.createElement("div");
  document.body.appendChild(root);
  const session = boot(root);
  if (!session) throw new Error("boot drew no table");
  const w: NewsWindow = {
    root, session, socket: NewsSocket.made[NewsSocket.made.length - 1]!, reads,
    promote: (next) => {
      me.role = next;
    },
  };
  windows.push(w);
  return w;
}

async function flush(): Promise<void> {
  for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0));
}

function set(members: Member[], controlsNoPartyMember = false): FrameInit {
  return {
    case: "watchableChanged",
    value: { actors: members.map(([actorId, name, abilityIds]) => ({ actorId, name, abilityIds })), controlsNoPartyMember },
  };
}

let seq = 0;

function entry(payload: PayloadInit): FrameInit {
  seq++;
  return { case: "event", value: { eventId: `news-${seq}`, sequence: BigInt(seq), payload } };
}

function logOf(socket: NewsSocket, [id, name, abilityIds]: Member): void {
  seq = 0;
  socket.frame(entry({ case: "sceneCreated", value: { sceneId: "s1", name: "Hall", gridWidth: 8, gridHeight: 8 } }));
  socket.frame(entry({
    case: "actorAdded",
    value: { actor: { actorId: id, name, moduleId: "m", kind: ActorKind.PARTY_MEMBER, abilityIds } },
  }));
  socket.frame(entry({ case: "actorControlGranted", value: { actorId: id, participantId: "p1", kind: ActorKind.PARTY_MEMBER } }));
  socket.frame(entry({
    case: "tokenPlaced", value: { tokenId: `t-${id}`, sceneId: "s1", actorId: id, position: { x: 1, y: 1 }, name },
  }));
}

function opening(socket: NewsSocket, members: Member[], watched: Member | null, none = false): void {
  socket.open();
  socket.frame({ case: "catchUpHead", value: { headSequence: 0n } });
  socket.frame({ case: "presenceSnapshot", value: {} });
  socket.frame(set(members, none));
  socket.frame({ case: "viewpointChanged", value: { actorId: watched?.[0] ?? "" } });
  if (watched) logOf(socket, watched);
}

async function playerOn(members: Member[], watched: Member, first: Ability[]): Promise<NewsWindow> {
  const w = openWindow("player");
  opening(w.socket, members, watched);
  await flush();
  expect(w.reads.length).toBe(1);
  w.reads[0]!(first);
  await flush();
  return w;
}

function toast(w: NewsWindow): string | null {
  return w.root.querySelector(".toast")?.textContent ?? null;
}

function offered(w: NewsWindow): string[] {
  const panel = w.root.querySelector<HTMLElement>("section.player");
  const heading = Array.from(panel?.querySelectorAll("h3") ?? []).find((h) => h.textContent === "Abilities");
  return Array.from(heading?.nextElementSibling?.querySelectorAll("button") ?? []).map((b) => b.textContent ?? "");
}

const given = (actorId: string, abilityId: string): FrameInit => entry({ case: "abilityGiven", value: { actorId, abilityId } });
const taken = (actorId: string, abilityId: string): FrameInit => entry({ case: "abilityTaken", value: { actorId, abilityId } });

// VTT-732
test("a player's window reads /api/ruleset again when a set changes a party member's ability ids, and not for an equal set", async () => {
  const w = await playerOn([["a", "Ayla", ["slash"]]], ["a", "Ayla", ["slash"]], [SLASH]);
  w.socket.frame(set([["a", "Ayla", ["slash", "heal"]]]));
  await flush();
  expect(w.reads.length).toBe(2);
  w.socket.frame(set([["a", "Ayla", ["slash", "heal"]]]));
  await flush();
  expect(w.reads.length).toBe(2);
  w.socket.frame(set([["a", "Ayla", ["heal"]]]));
  await flush();
  expect(w.reads.length).toBe(3);
});

// VTT-732 VTT-695
test("a player's panel offers a given ability once the read answers, and stops offering a taken one", async () => {
  const w = await playerOn([["a", "Ayla", ["slash"]]], ["a", "Ayla", ["slash"]], [SLASH]);
  expect(offered(w)).toEqual(["Slash"]);
  w.socket.frame(set([["a", "Ayla", ["slash", "heal"]]]));
  w.socket.frame(given("a", "heal"));
  await flush();
  expect(offered(w)).toEqual(["Slash"]);
  w.reads[1]!([SLASH, HEAL]);
  await flush();
  expect(offered(w)).toEqual(["Slash", "Heal"]);
  w.socket.frame(set([["a", "Ayla", ["heal"]]]));
  w.socket.frame(taken("a", "slash"));
  await flush();
  w.reads[2]!([HEAL]);
  await flush();
  expect(offered(w)).toEqual(["Heal"]);
});

// VTT-733 VTT-734
test("a player is told what a give and a take changed once the read it issued answers, a taken ability named by an earlier answer", async () => {
  const w = await playerOn([["a", "Ayla", ["slash"]]], ["a", "Ayla", ["slash"]], [SLASH]);
  expect(toast(w)).toBeNull();
  w.socket.frame(set([["a", "Ayla", ["slash", "heal"]]]));
  w.socket.frame(given("a", "heal"));
  await flush();
  expect(toast(w)).toBeNull();
  w.reads[1]!([SLASH, HEAL]);
  await flush();
  expect(toast(w)).toBe("Ayla can now use Heal.");
  w.socket.frame(set([["a", "Ayla", ["heal"]]]));
  w.socket.frame(taken("a", "slash"));
  await flush();
  w.reads[2]!([HEAL]);
  await flush();
  expect(toast(w)).toBe("Ayla can no longer use Slash.");
  expect(w.root.querySelector(".feed")?.textContent).toContain("Ayla can no longer use Slash");
});

// VTT-733
test("one set that gives and takes several abilities, and changes a nameless character, is told in one toast", async () => {
  const w = await playerOn(
    [["a", "Ayla", ["slash"]], ["b", "", []]], ["a", "Ayla", ["slash"]], [SLASH],
  );
  w.socket.frame(set([["a", "Ayla", ["heal", "bolt"]], ["b", "", ["dart"]]]));
  await flush();
  w.reads[1]!([HEAL, BOLT, DART]);
  await flush();
  expect(toast(w)).toBe("Ayla can now use Heal and Bolt. Ayla can no longer use Slash. An unnamed character can now use Dart.");
});

// VTT-733
test("a player watching one character is told of a change to another they control", async () => {
  const w = await playerOn([["a", "Ayla", ["slash"]], ["b", "Bo", ["heal"]]], ["a", "Ayla", ["slash"]], [SLASH, HEAL]);
  w.socket.frame(set([["a", "Ayla", ["slash"]], ["b", "Bo", ["heal", "dart"]]]));
  await flush();
  w.reads[1]!([SLASH, HEAL, DART]);
  await flush();
  expect(toast(w)).toBe("Bo can now use Dart.");
});

// VTT-733 VTT-734
test("a read that fails still tells the change, naming an ability no answer has named by its id", async () => {
  const w = await playerOn([["a", "Ayla", ["slash"]]], ["a", "Ayla", ["slash"]], [SLASH]);
  w.socket.frame(set([["a", "Ayla", ["slash", "heal"]]]));
  await flush();
  w.reads[1]!([], 500);
  await flush();
  expect(toast(w)).toBe("Ayla can now use heal.");
});

// VTT-734 VTT-758
test("an older read's answer arriving after the newest names an ability in the Story at once, and leaves the message standing", async () => {
  const ayla: Member = ["a", "Ayla", ["slash"]];
  const w = await playerOn([ayla], ayla, [SLASH]);
  w.socket.frame(set([ayla, ["b", "Bo", []]]));
  await flush();
  w.socket.frame(set([ayla]));
  await flush();
  expect(w.reads.length).toBe(3);
  w.reads[2]!([SLASH]);
  await flush();
  w.socket.frame(given("a", "heal"));
  await flush();
  const story = () => w.root.querySelector(".feed")?.textContent ?? "";
  expect(story()).toContain("Ayla can now use heal");
  expect(toast(w)).toBe("You no longer play Bo.");
  w.reads[1]!([SLASH, HEAL]);
  await flush();
  expect(story()).toContain("Ayla can now use Heal");
  expect(story()).not.toContain("Ayla can now use heal");
  expect(toast(w)).toBe("You no longer play Bo.");
});

// VTT-733
test("a redial sent a set that changed meanwhile tells the change", async () => {
  const w = await playerOn([["a", "Ayla", ["slash"]]], ["a", "Ayla", ["slash"]], [SLASH]);
  void w.session.reconnect();
  await flush();
  const again = NewsSocket.made[NewsSocket.made.length - 1]!;
  expect(again).not.toBe(w.socket);
  opening(again, [["a", "Ayla", ["slash", "heal"]]], ["a", "Ayla", ["slash", "heal"]]);
  await flush();
  w.reads[1]!([SLASH, HEAL]);
  await flush();
  expect(toast(w)).toBe("Ayla can now use Heal.");
});

// VTT-733 VTT-752
test("a DM's and a spectator's window show no toast for a set that changes ability ids", async () => {
  const dm = openWindow("dm");
  dm.socket.open();
  dm.socket.frame({ case: "catchUpHead", value: { headSequence: 0n } });
  dm.socket.frame({ case: "presenceSnapshot", value: {} });
  await flush();
  dm.reads[0]!([SLASH, HEAL]);
  dm.socket.frame(set([["a", "Ayla", ["slash"]]]));
  dm.socket.frame(set([["a", "Ayla", ["slash", "heal"]]]));
  await flush();
  expect(toast(dm)).toBeNull();

  const watcher = openWindow("spectator");
  opening(watcher.socket, [["a", "Ayla", ["slash"]]], null);
  await flush();
  watcher.reads[0]!([]);
  watcher.socket.frame(set([["a", "Ayla", ["slash", "heal"]]]));
  await flush();
  watcher.promote("player");
  watcher.socket.frame({ case: "presenceChanged", value: { participantId: "p1", displayName: "Pia", state: PresenceState.CONNECTED } });
  await flush();
  expect(watcher.reads.length).toBe(2);
  watcher.reads[1]!([SLASH, HEAL]);
  await flush();
  expect(toast(watcher)).toBeNull();
});

// VTT-752
test("sets that change a party member's ability ids before /api/me answers leave the window on the last set, and no toast", async () => {
  let answerMe = (): void => {};
  const w = openWindow("player", new Promise<void>((r) => (answerMe = () => r())));
  opening(w.socket, [["a", "Ayla", ["slash"]]], ["a", "Ayla", ["slash"]]);
  w.socket.frame(set([["a", "Ayla", ["slash", "heal"]]]));
  w.socket.frame(set([["a", "Ayla", ["heal"]]]));
  expect(w.session.watchable.map((a) => a.abilityIds)).toEqual([["heal"]]);
  answerMe();
  await flush();
  expect(w.reads.length).toBe(1);
  w.reads[0]!([HEAL]);
  await flush();
  expect(toast(w)).toBeNull();
});
