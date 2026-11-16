import "./support/dom";
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { create, toJsonString } from "@bufbuild/protobuf";
import {
  CatchUpHeadSchema,
  PresenceSnapshotSchema,
  ServerFrameSchema,
  ViewpointChangedSchema,
  WatchableActorSchema,
  WatchableChangedSchema,
  type ServerFrame,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import {
  AbilityGivenSchema,
  AbilityTakenSchema,
  ActorAddedSchema,
  ActorControlGrantedSchema,
  ActorKind,
  ActorSchema,
  EnvelopeSchema,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { boot } from "../src/app";
import { Session, type SheetChange } from "../src/session";
import { sheetNews } from "../src/view/naming";

// ---------------------------------------------------------------------------
// Fakes: a WebSocket the test drives, and a fetch that answers the read
// surface, holding every /api/ruleset read until the test answers it.

type Listener = (e: unknown) => void;

class QAFakeSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static made: QAFakeSocket[] = [];
  readonly CONNECTING = 0;
  readonly OPEN = 1;
  readonly CLOSING = 2;
  readonly CLOSED = 3;
  readyState = 0;
  sent: string[] = [];
  url: string;
  onopen: Listener | null = null;
  onmessage: Listener | null = null;
  onclose: Listener | null = null;
  onerror: Listener | null = null;
  private listeners = new Map<string, Listener[]>();
  constructor(url: string) {
    this.url = url;
    QAFakeSocket.made.push(this);
  }
  addEventListener(type: string, fn: Listener): void {
    const list = this.listeners.get(type) ?? [];
    list.push(fn);
    this.listeners.set(type, list);
  }
  removeEventListener(type: string, fn: Listener): void {
    const list = this.listeners.get(type) ?? [];
    this.listeners.set(
      type,
      list.filter((f) => f !== fn),
    );
  }
  send(data: string): void {
    this.sent.push(data);
  }
  close(): void {
    this.readyState = 3;
  }
  fire(type: string, e: unknown): void {
    const own = (this as unknown as Record<string, unknown>)["on" + type];
    if (typeof own === "function") (own as Listener)(e);
    for (const fn of this.listeners.get(type) ?? []) fn(e);
  }
  open(): void {
    this.readyState = 1;
    this.fire("open", {});
  }
  frame(f: ServerFrame["frame"]): void {
    const data = toJsonString(ServerFrameSchema, create(ServerFrameSchema, { frame: f }));
    this.fire("message", { data });
  }
  drop(): void {
    this.readyState = 3;
    this.fire("close", { code: 1006, reason: "", wasClean: false });
  }
}

interface Held {
  resolve: (r: Response) => void;
  reject: (e: unknown) => void;
}

interface Ability {
  id: string;
  name: string;
}

function rulesetBody(abilities: Ability[]): string {
  return JSON.stringify({
    id: "qa",
    name: "QA rules",
    abilities: abilities.map((a) => ({
      id: a.id,
      name: a.name,
      range: 1,
      maxTargets: 1,
      usage: { kind: "atWill" },
    })),
    conditions: [],
    resources: [],
    properties: [],
  });
}

function json(body: string, status = 200): Response {
  return new Response(body, { status, headers: { "content-type": "application/json" } });
}

class QAServer {
  held: Held[] = [];
  paths: string[] = [];
  constructor(
    readonly participantId: string,
    readonly role: string,
  ) {}
  fetch = (input: unknown, _init?: unknown): Promise<Response> => {
    const u = new URL(String(input), "http://localhost:8080");
    this.paths.push(u.pathname);
    switch (u.pathname) {
      case "/api/me":
        return Promise.resolve(
          json(JSON.stringify({ participantId: this.participantId, name: "Pia", role: this.role })),
        );
      case "/api/ruleset":
        return new Promise<Response>((resolve, reject) => {
          this.held.push({ resolve, reject });
        });
      case "/api/adventures":
        return Promise.resolve(json(JSON.stringify({ adventures: [] })));
      case "/api/maps":
        return Promise.resolve(json(JSON.stringify({ cellPx: 64, maps: [] })));
      default:
        return Promise.resolve(new Response("gateway: not authorized", { status: 403 }));
    }
  };
  rulesetReads(): number {
    return this.paths.filter((p) => p === "/api/ruleset").length;
  }
  answer(i: number, abilities: Ability[]): void {
    const h = this.held[i];
    if (h === undefined) throw new Error(`no ruleset read #${i}; reads: ${this.held.length}`);
    h.resolve(json(rulesetBody(abilities)));
  }
  fail(i: number): void {
    const h = this.held[i];
    if (h === undefined) throw new Error(`no ruleset read #${i}; reads: ${this.held.length}`);
    h.resolve(new Response("gateway: identity unavailable", { status: 503 }));
  }
}

const g = globalThis as unknown as Record<string, unknown>;
const realFetch = g["fetch"];
const realSocket = g["WebSocket"];
let server: QAServer;
const sessions: Session[] = [];

beforeEach(() => {
  QAFakeSocket.made = [];
  g["WebSocket"] = QAFakeSocket;
  localStorage.clear();
  sessionStorage.clear();
  document.body.innerHTML = "";
});

afterEach(() => {
  for (const s of sessions.splice(0)) s.close();
  g["fetch"] = realFetch;
  g["WebSocket"] = realSocket;
  localStorage.clear();
  sessionStorage.clear();
  document.body.innerHTML = "";
  history.replaceState(null, "", "/");
});

async function settle(): Promise<void> {
  for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));
}

async function waitFor(cond: () => boolean, what: string): Promise<void> {
  for (let i = 0; i < 400; i++) {
    if (cond()) return;
    await new Promise((r) => setTimeout(r, 2));
  }
  throw new Error(`timed out waiting for ${what}`);
}

// ---------------------------------------------------------------------------
// Frame builders.

type Row = [actorId: string, name: string, abilityIds: string[]];

function setFrame(none: boolean, ...rows: Row[]): ServerFrame["frame"] {
  return {
    case: "watchableChanged",
    value: create(WatchableChangedSchema, {
      controlsNoPartyMember: none,
      actors: rows.map(([actorId, name, abilityIds]) =>
        create(WatchableActorSchema, { actorId, name, abilityIds }),
      ),
    }),
  };
}

function viewpoint(actorId: string): ServerFrame["frame"] {
  return { case: "viewpointChanged", value: create(ViewpointChangedSchema, { actorId }) };
}

// introduce sends a party member's arrival and its grant to owner, sequences
// 1 and 2 of the log; later entries start at 3.
function introduce(ws: QAFakeSocket, actorId: string, name: string, owner: string, ids: string[]): void {
  ws.frame({
    case: "event",
    value: create(EnvelopeSchema, {
      eventId: "e1",
      sequence: 1n,
      payload: {
        case: "actorAdded",
        value: create(ActorAddedSchema, {
          actor: create(ActorSchema, { actorId, name, kind: ActorKind.PARTY_MEMBER, abilityIds: ids }),
        }),
      },
    }),
  });
  ws.frame({
    case: "event",
    value: create(EnvelopeSchema, {
      eventId: "e2",
      sequence: 2n,
      payload: {
        case: "actorControlGranted",
        value: create(ActorControlGrantedSchema, {
          actorId,
          participantId: owner,
          kind: ActorKind.PARTY_MEMBER,
        }),
      },
    }),
  });
}

function given(seq: bigint, actorId: string, abilityId: string): ServerFrame["frame"] {
  return {
    case: "event",
    value: create(EnvelopeSchema, {
      eventId: `e${seq}`,
      sequence: seq,
      payload: { case: "abilityGiven", value: create(AbilityGivenSchema, { actorId, abilityId }) },
    }),
  };
}

function taken(seq: bigint, actorId: string, abilityId: string): ServerFrame["frame"] {
  return {
    case: "event",
    value: create(EnvelopeSchema, {
      eventId: `e${seq}`,
      sequence: seq,
      payload: { case: "abilityTaken", value: create(AbilityTakenSchema, { actorId, abilityId }) },
    }),
  };
}

function opening(ws: QAFakeSocket): void {
  ws.open();
  ws.frame({ case: "catchUpHead", value: create(CatchUpHeadSchema, { headSequence: 0n }) });
  ws.frame({ case: "presenceSnapshot", value: create(PresenceSnapshotSchema, {}) });
}

// ---------------------------------------------------------------------------
// Window helpers.

function toast(root: HTMLElement): string | null {
  const el = root.querySelector(".toast");
  return el === null ? null : (el.textContent ?? "");
}

function storyText(root: HTMLElement): string {
  return root.querySelector(".feed")?.textContent ?? "";
}

function panelButtons(root: HTMLElement): string[] {
  const panel = root.querySelector(".player");
  if (panel === null) return [];
  return Array.from(panel.querySelectorAll("button")).map((b) => (b.textContent ?? "").trim());
}

function offersAbility(root: HTMLElement, name: string): boolean {
  return panelButtons(root).some((t) => t === name);
}

async function bootWindow(
  participantId: string,
  role: string,
  startup: Ability[],
): Promise<{ root: HTMLElement; ws: QAFakeSocket; session: Session }> {
  server = new QAServer(participantId, role);
  g["fetch"] = server.fetch;
  (globalThis as unknown as { happyDOM: { setURL(url: string): void } }).happyDOM.setURL("http://localhost:8080/");
  history.replaceState(null, "", "/?token=tok-" + participantId);
  const root = document.createElement("main");
  root.id = "app";
  document.body.appendChild(root);
  const session = boot(root);
  if (session === null) throw new Error("boot returned no session");
  sessions.push(session);
  await waitFor(() => server.held.length >= 1, "the start-up /api/ruleset read");
  server.answer(0, startup);
  await waitFor(() => QAFakeSocket.made.length >= 1, "the WebSocket");
  await settle();
  const ws = QAFakeSocket.made[QAFakeSocket.made.length - 1];
  if (ws === undefined) throw new Error("no socket");
  return { root, ws, session };
}

// ===========================================================================
// sheetNews, the words.

describe("qa sheet news: sheetNews writes the words", () => {
  const list: Ability[] = [
    { id: "parry", name: "Parry" },
    { id: "slash", name: "Slash" },
    { id: "heal", name: "Heal" },
    { id: "zeta", name: "Zeta" },
    { id: "alpha", name: "Alpha" },
    { id: "mid", name: "Mid" },
  ];
  const change = (actorId: string, name: string, gave: string[], took: string[]): SheetChange => ({
    actorId,
    name,
    given: gave,
    taken: took,
  });

  // VTT-733
  test("VTT-733 one given ability: '<Name> can now use <A>.'", () => {
    expect(sheetNews([change("a1", "Alda", ["parry"], [])], list)).toBe("Alda can now use Parry.");
  });

  // VTT-733
  test("VTT-733 one taken ability: '<Name> can no longer use <B>.'", () => {
    expect(sheetNews([change("a1", "Alda", [], ["slash"])], list)).toBe("Alda can no longer use Slash.");
  });

  // VTT-733
  test("VTT-733 several given abilities are joined by ' and ', in the change's order", () => {
    expect(sheetNews([change("a1", "Alda", ["zeta", "alpha", "mid"], [])], list)).toBe(
      "Alda can now use Zeta and Alpha and Mid.",
    );
    expect(sheetNews([change("a1", "Alda", [], ["zeta", "alpha"])], list)).toBe(
      "Alda can no longer use Zeta and Alpha.",
    );
  });

  // VTT-733
  test("VTT-733 a give and a take on one actor are two sentences joined by a space", () => {
    expect(sheetNews([change("a1", "Alda", ["parry"], ["slash"])], list)).toBe(
      "Alda can now use Parry. Alda can no longer use Slash.",
    );
  });

  // VTT-733
  test("VTT-733 several actors are told in the changes' order, sentences joined by a space", () => {
    expect(
      sheetNews([change("a2", "Bram", [], ["heal"]), change("a1", "Alda", ["parry", "slash"], [])], list),
    ).toBe("Bram can no longer use Heal. Alda can now use Parry and Slash.");
  });

  // VTT-733
  test("VTT-733 a character the set gives no name is 'An unnamed character'", () => {
    expect(sheetNews([change("a1", "", ["parry"], ["slash"])], list)).toBe(
      "An unnamed character can now use Parry. An unnamed character can no longer use Slash.",
    );
  });

  // VTT-734
  test("VTT-734 an ability the list does not hold is named by its id", () => {
    expect(sheetNews([change("a1", "Alda", ["mystery"], ["slash", "gone"])], list)).toBe(
      "Alda can now use mystery. Alda can no longer use Slash and gone.",
    );
    expect(sheetNews([change("a1", "Alda", ["parry"], [])], [])).toBe("Alda can now use parry.");
  });
});

// ===========================================================================
// Session.onSheetChanged, the hand-over.

async function startSession(): Promise<{ s: Session; ws: QAFakeSocket; handed: SheetChange[][] }> {
  const s = new Session("ws://localhost:8080/ws", "tok-qa", null);
  sessions.push(s);
  const handed: SheetChange[][] = [];
  s.onSheetChanged((c) => handed.push(c.map((x) => ({ ...x, given: [...x.given], taken: [...x.taken] }))));
  void s.start().catch(() => undefined);
  await waitFor(() => QAFakeSocket.made.length >= 1, "the session's socket");
  const ws = QAFakeSocket.made[QAFakeSocket.made.length - 1];
  if (ws === undefined) throw new Error("no socket");
  opening(ws);
  await settle();
  return { s, ws, handed };
}

describe("qa sheet news: Session.onSheetChanged", () => {
  // VTT-733
  test("VTT-733 the page's first set hands over nothing", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash", "dodge"]]));
    await settle();
    expect(handed).toEqual([]);
  });

  // VTT-733
  test("VTT-733 given in the new set's order and taken in the previous set's order, per actor", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["s1", "keep", "s3"]]));
    ws.frame(setFrame(false, ["a1", "Alda", ["n2", "keep", "n1"]]));
    await settle();
    expect(handed).toEqual([[{ actorId: "a1", name: "Alda", given: ["n2", "n1"], taken: ["s1", "s3"] }]]);
  });

  // VTT-733
  test("VTT-733 actors are handed in the new set's order, only actors in both sets", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a2", "Bram", ["heal"]], ["a1", "Alda", ["slash"]], ["a9", "Gone", ["x"]]));
    ws.frame(
      setFrame(false, ["a1", "Alda", ["slash", "parry"]], ["a2", "Bram", []], ["a3", "New", ["fresh"]]),
    );
    await settle();
    expect(handed).toEqual([
      [
        { actorId: "a1", name: "Alda", given: ["parry"], taken: [] },
        { actorId: "a2", name: "Bram", given: [], taken: ["heal"] },
      ],
    ]);
  });

  // VTT-733
  test("VTT-733 each id is handed once, a repeated id included", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["x", "x", "y"]]));
    ws.frame(setFrame(false, ["a1", "Alda", ["y", "z", "z"]]));
    await settle();
    expect(handed).toEqual([[{ actorId: "a1", name: "Alda", given: ["z"], taken: ["x"] }]]);
  });

  // VTT-733
  test("VTT-733 the change is handed before the set is replaced", async () => {
    const { s, ws } = await startSession();
    const seen: string[][] = [];
    s.onSheetChanged(() => seen.push(s.watchable.flatMap((a) => [...a.abilityIds])));
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    ws.frame(setFrame(false, ["a1", "Alda", ["slash", "parry"]]));
    await settle();
    expect(seen).toEqual([["slash"]]);
    expect(s.watchable.flatMap((a) => [...a.abilityIds])).toEqual(["slash", "parry"]);
  });

  // VTT-733
  test("VTT-733 an equal set and a renamed one hand over nothing", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]], ["a2", "Bram", ["heal"]]));
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]], ["a2", "Bram", ["heal"]]));
    ws.frame(setFrame(false, ["a1", "Alda the Bold", ["slash"]], ["a2", "", ["heal"]]));
    await settle();
    expect(handed).toEqual([]);
  });

  // VTT-733
  test("VTT-733 a set after, or beside, a flag saying they control no party member hands over nothing", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    ws.frame(setFrame(true, ["a1", "Alda", ["slash", "parry"]]));
    ws.frame(setFrame(false, ["a1", "Alda", ["dodge"]]));
    ws.frame(setFrame(true, ["a1", "Alda", []], ["a2", "Bram", []]));
    ws.frame(setFrame(true, ["a1", "Alda", ["x"]], ["a2", "Bram", ["y"]]));
    await settle();
    expect(handed).toEqual([]);
  });

  // VTT-733
  test("VTT-733 an actor whose ids did not change is not handed beside one whose did", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]], ["a2", "Bram", ["heal"]]));
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]], ["a2", "Bram", ["heal", "mend"]]));
    await settle();
    expect(handed).toEqual([[{ actorId: "a2", name: "Bram", given: ["mend"], taken: [] }]]);
  });

  // VTT-733
  test("VTT-733 a set that only reorders an actor's ids hands over nothing", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash", "dodge"]]));
    ws.frame(setFrame(false, ["a1", "Alda", ["dodge", "slash"]]));
    await settle();
    expect(handed).toEqual([]);
  });

  // VTT-733
  test("VTT-733 a change on a renamed actor carries the name the new set gives", async () => {
    const { ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    ws.frame(setFrame(false, ["a1", "Alda the Bold", ["slash", "parry"]]));
    await settle();
    expect(handed).toEqual([[{ actorId: "a1", name: "Alda the Bold", given: ["parry"], taken: [] }]]);
  });

  // VTT-733
  test("VTT-733 a redial sent the same set hands over nothing, and one sent a changed set hands the change", async () => {
    const { s, ws, handed } = await startSession();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    ws.frame(viewpoint("a1"));
    await settle();
    ws.drop();
    await settle();
    void s.reconnect().catch(() => undefined);
    await waitFor(() => QAFakeSocket.made.length >= 2, "the redial");
    const ws2 = QAFakeSocket.made[1];
    if (ws2 === undefined) throw new Error("no redial socket");
    opening(ws2);
    ws2.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    await settle();
    expect(handed).toEqual([]);
    ws2.drop();
    await settle();
    void s.reconnect().catch(() => undefined);
    await waitFor(() => QAFakeSocket.made.length >= 3, "the second redial");
    const ws3 = QAFakeSocket.made[2];
    if (ws3 === undefined) throw new Error("no second redial socket");
    opening(ws3);
    ws3.frame(setFrame(false, ["a1", "Alda", ["parry"]]));
    await settle();
    expect(handed).toEqual([[{ actorId: "a1", name: "Alda", given: ["parry"], taken: ["slash"] }]]);
  });
});

// ===========================================================================
// The browser window, end to end through boot().

describe("qa sheet news: a player's window", () => {
  // VTT-732, VTT-733, VTT-734, VTT-695
  test("VTT-732 VTT-733 VTT-734 VTT-695 a give and a take re-read the ruleset, then tell the player and change the panel", async () => {
    const { root, ws } = await bootWindow("p1", "player", [
      { id: "slash", name: "Slash" },
      { id: "dodge", name: "Dodge" },
    ]);
    opening(ws);
    ws.frame(setFrame(false, ["a1", "Alda", ["slash", "dodge"]], ["a2", "Bram", ["ward"]]));
    ws.frame(viewpoint("a1"));
    introduce(ws, "a1", "Alda", "p1", ["slash", "dodge"]);
    await settle();
    // The first set may itself issue a read (VTT-696); answer every read so far.
    for (let i = 1; i < server.held.length; i++) {
      server.answer(i, [
        { id: "dodge", name: "Dodge" },
        { id: "slash", name: "Slash" },
        { id: "ward", name: "Ward" },
      ]);
    }
    await settle();
    expect(toast(root)).toBeNull();
    expect(offersAbility(root, "Slash")).toBe(true);
    expect(offersAbility(root, "Dodge")).toBe(true);
    expect(offersAbility(root, "Parry")).toBe(false);

    // A give: the set comes before the entry, as the gateway sends them.
    let reads = server.rulesetReads();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash", "dodge", "parry"]], ["a2", "Bram", ["ward"]]));
    ws.frame(given(3n, "a1", "parry"));
    await settle();
    expect(server.rulesetReads()).toBe(reads + 1); // VTT-732
    expect(toast(root)).toBeNull(); // kept until the read answers
    server.answer(server.held.length - 1, [
      { id: "dodge", name: "Dodge Roll" },
      { id: "parry", name: "Parry" },
      { id: "slash", name: "Slash" },
      { id: "ward", name: "Ward" },
    ]);
    await settle();
    expect(toast(root)).toBe("Alda can now use Parry."); // VTT-733
    expect(offersAbility(root, "Parry")).toBe(true); // VTT-695
    expect(storyText(root)).not.toContain("can now use"); // VTT-809

    // A take: the answer no longer lists the taken ability; the window still
    // names it by what an earlier answer called it (VTT-734).
    reads = server.rulesetReads();
    ws.frame(setFrame(false, ["a1", "Alda", ["dodge", "parry"]], ["a2", "Bram", ["ward"]]));
    ws.frame(taken(4n, "a1", "slash"));
    await settle();
    expect(server.rulesetReads()).toBe(reads + 1); // VTT-732
    expect(toast(root)).toBe("Alda can now use Parry."); // not yet replaced
    server.answer(server.held.length - 1, [
      { id: "dodge", name: "Dodge Roll" },
      { id: "parry", name: "Parry" },
      { id: "ward", name: "Ward" },
    ]);
    await settle();
    expect(toast(root)).toBe("Alda can no longer use Slash."); // VTT-733, VTT-734
    expect(offersAbility(root, "Slash")).toBe(false); // VTT-695
    expect(offersAbility(root, "Parry")).toBe(true);
    expect(storyText(root)).not.toContain("can no longer use"); // VTT-809

    // A give of an id no answer ever named, whose read fails: shown after the
    // failure, by its id.
    reads = server.rulesetReads();
    ws.frame(setFrame(false, ["a1", "Alda", ["dodge", "parry", "mystery"]], ["a2", "Bram", ["ward"]]));
    ws.frame(given(5n, "a1", "mystery"));
    await settle();
    expect(server.rulesetReads()).toBe(reads + 1);
    expect(toast(root)).toBe("Alda can no longer use Slash.");
    server.fail(server.held.length - 1);
    await settle();
    expect(toast(root)).toBe("Alda can now use mystery."); // VTT-733, VTT-734
    expect(storyText(root)).not.toContain("mystery"); // VTT-809

    // Two actors at once, one with a give and a take: per actor in the set's
    // order, the newest display name any answer gave.
    reads = server.rulesetReads();
    ws.frame(
      setFrame(false, ["a1", "Alda", ["parry", "mystery", "fang", "hook"]], ["a2", "Bram", ["ward", "mend"]]),
    );
    ws.frame(taken(6n, "a1", "dodge"));
    ws.frame(given(7n, "a1", "fang"));
    ws.frame(given(8n, "a1", "hook"));
    await settle();
    expect(server.rulesetReads()).toBe(reads + 1);
    server.answer(server.held.length - 1, [
      { id: "fang", name: "Fang" },
      { id: "hook", name: "Hook" },
      { id: "mend", name: "Mend" },
      { id: "parry", name: "Parry" },
      { id: "ward", name: "Ward" },
    ]);
    await settle();
    expect(toast(root)).toBe(
      "Alda can now use Fang and Hook. Alda can no longer use Dodge Roll. Bram can now use Mend.",
    );
    expect(storyText(root)).not.toContain("Dodge Roll"); // VTT-809
    expect(offersAbility(root, "Fang")).toBe(true);
    expect(offersAbility(root, "Dodge Roll")).toBe(false);
  });

  // VTT-732, VTT-733
  test("VTT-732 VTT-733 an equal set and a renamed one issue no read and tell nothing", async () => {
    const { root, ws } = await bootWindow("p1", "player", [{ id: "slash", name: "Slash" }]);
    opening(ws);
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    ws.frame(viewpoint("a1"));
    introduce(ws, "a1", "Alda", "p1", ["slash"]);
    await settle();
    for (let i = 1; i < server.held.length; i++) server.answer(i, [{ id: "slash", name: "Slash" }]);
    await settle();
    const reads = server.rulesetReads();
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    ws.frame(setFrame(false, ["a1", "Alda the Bold", ["slash"]]));
    await settle();
    for (let i = 1; i < server.held.length; i++) server.answer(i, [{ id: "slash", name: "Slash" }]);
    await settle();
    expect(server.rulesetReads()).toBe(reads);
    expect(toast(root)).toBeNull();
    // A take and a give back of one id between two judgments reorders the
    // list (VTT-715): no id is gained or lost, so nothing more is told.
    ws.frame(setFrame(false, ["a1", "Alda the Bold", ["slash", "dodge"]]));
    ws.frame(given(3n, "a1", "dodge"));
    await settle();
    for (let i = 1; i < server.held.length; i++) {
      server.answer(i, [
        { id: "dodge", name: "Dodge" },
        { id: "slash", name: "Slash" },
      ]);
    }
    await settle();
    expect(toast(root)).toBe("Alda the Bold can now use Dodge.");
    ws.frame(setFrame(false, ["a1", "Alda the Bold", ["dodge", "slash"]]));
    ws.frame(taken(4n, "a1", "slash"));
    ws.frame(given(5n, "a1", "slash"));
    await settle();
    for (let i = 1; i < server.held.length; i++) {
      server.answer(i, [
        { id: "dodge", name: "Dodge" },
        { id: "slash", name: "Slash" },
      ]);
    }
    await settle();
    expect(toast(root)).toBe("Alda the Bold can now use Dodge.");
  });

  // VTT-733
  test("VTT-733 a character the set gives no name is told as 'An unnamed character'", async () => {
    const { root, ws } = await bootWindow("p1", "player", [{ id: "slash", name: "Slash" }]);
    opening(ws);
    ws.frame(setFrame(false, ["a1", "", ["slash"]]));
    ws.frame(viewpoint("a1"));
    introduce(ws, "a1", "Alda", "p1", ["slash"]);
    await settle();
    for (let i = 1; i < server.held.length; i++) server.answer(i, [{ id: "slash", name: "Slash" }]);
    await settle();
    ws.frame(setFrame(false, ["a1", "", ["slash", "parry"]]));
    ws.frame(given(3n, "a1", "parry"));
    await settle();
    server.answer(server.held.length - 1, [
      { id: "parry", name: "Parry" },
      { id: "slash", name: "Slash" },
    ]);
    await settle();
    expect(toast(root)).toBe("An unnamed character can now use Parry.");
  });

  // VTT-733
  test("VTT-733 a set beside a flag saying they control no party member tells no sheet news", async () => {
    const { root, ws } = await bootWindow("p1", "player", []);
    opening(ws);
    ws.frame(setFrame(true, ["a1", "Alda", []]));
    ws.frame(viewpoint(""));
    await settle();
    for (let i = 1; i < server.held.length; i++) server.answer(i, []);
    await settle();
    // Granted: flag true before, so nothing is a sheet change.
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    await settle();
    for (let i = 1; i < server.held.length; i++) server.answer(i, [{ id: "slash", name: "Slash" }]);
    await settle();
    expect(toast(root) ?? "").not.toContain("can now use");
    // Taken back: flag true after, so the left toast alone may speak.
    ws.frame(setFrame(true, ["a1", "Alda", []]));
    await settle();
    for (let i = 1; i < server.held.length; i++) server.answer(i, []);
    await settle();
    expect(toast(root) ?? "").not.toContain("can no longer use");
  });

  // VTT-733 (D8: a redial is sent the new set; the kept set tells what changed)
  test("VTT-733 a redial sent a changed set tells the player after its read", async () => {
    const { root, ws, session } = await bootWindow("p1", "player", [{ id: "slash", name: "Slash" }]);
    opening(ws);
    ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
    ws.frame(viewpoint("a1"));
    introduce(ws, "a1", "Alda", "p1", ["slash"]);
    await settle();
    for (let i = 1; i < server.held.length; i++) server.answer(i, [{ id: "slash", name: "Slash" }]);
    await settle();
    ws.drop();
    await settle();
    void session.reconnect().catch(() => undefined);
    await waitFor(() => QAFakeSocket.made.length >= 2, "the redial");
    const ws2 = QAFakeSocket.made[1];
    if (ws2 === undefined) throw new Error("no redial");
    const reads = server.rulesetReads();
    opening(ws2);
    ws2.frame(setFrame(false, ["a1", "Alda", ["slash", "parry"]]));
    ws2.frame(viewpoint("a1"));
    await settle();
    expect(server.rulesetReads()).toBe(reads + 1);
    server.answer(server.held.length - 1, [
      { id: "parry", name: "Parry" },
      { id: "slash", name: "Slash" },
    ]);
    await settle();
    expect(toast(root)).toBe("Alda can now use Parry.");
  });
});

describe("qa sheet news: windows that are not a player's", () => {
  for (const role of ["dm", "spectator"]) {
    // VTT-733, VTT-732, VTT-752
    test(`VTT-733 VTT-732 a ${role}'s window is told nothing and reads nothing again at a changed set`, async () => {
      const { root, ws } = await bootWindow("x1", role, [{ id: "slash", name: "Slash" }]);
      opening(ws);
      ws.frame(setFrame(false, ["a1", "Alda", ["slash"]]));
      await settle();
      for (let i = 1; i < server.held.length; i++) server.answer(i, [{ id: "slash", name: "Slash" }]);
      await settle();
      const reads = server.rulesetReads();
      ws.frame(setFrame(false, ["a1", "Alda", ["slash", "parry"]]));
      await settle();
      for (let i = 1; i < server.held.length; i++) server.answer(i, [{ id: "slash", name: "Slash" }]);
      await settle();
      expect(server.rulesetReads()).toBe(reads);
      expect(toast(root)).toBeNull();
    });
  }
});
