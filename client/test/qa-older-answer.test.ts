import "./support/dom";
import { afterEach, describe, expect, test } from "bun:test";
import { create, toJsonString, type MessageInitShape } from "@bufbuild/protobuf";
import { boot } from "../src/app";
import type { Session } from "../src/session";
import { ServerFrameSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";

// SPEC-022: a player's window with two /api/ruleset reads in flight.

type FrameInit = NonNullable<MessageInitShape<typeof ServerFrameSchema>["frame"]>;
type Role = "dm" | "player" | "agent" | "spectator";

class FakeSocket {
  static all: FakeSocket[] = [];
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  readonly CONNECTING = 0;
  readonly OPEN = 1;
  readonly CLOSING = 2;
  readonly CLOSED = 3;
  readyState = 0;
  onopen: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  private listeners = new Map<string, Array<(e: Event) => void>>();
  constructor(readonly url: string) {
    FakeSocket.all.push(this);
    setTimeout(() => {
      this.readyState = 1;
      this.fire("open", new Event("open"));
    }, 0);
  }
  addEventListener(type: string, fn: (e: Event) => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
  }
  removeEventListener(type: string, fn: (e: Event) => void): void {
    this.listeners.set(type, (this.listeners.get(type) ?? []).filter((f) => f !== fn));
  }
  send(_data: string): void {}
  close(): void {
    this.readyState = 3;
  }
  deliver(frame: FrameInit): void {
    const text = toJsonString(ServerFrameSchema, create(ServerFrameSchema, { frame }));
    this.fire("message", new MessageEvent("message", { data: text }));
  }
  private fire(type: string, e: Event): void {
    const h = (this as unknown as Record<string, unknown>)["on" + type];
    if (typeof h === "function") (h as (e: Event) => void)(e);
    for (const fn of this.listeners.get(type) ?? []) fn(e);
  }
}

/** One GET /api/ruleset the window issued, held until the test settles it. */
interface Read {
  settled: boolean;
  answer(abilities: Array<[id: string, name: string]>, resources?: Array<[key: string, name: string]>): void;
  fail(status?: number): void;
}

const realFetch = globalThis.fetch;
const realWS = globalThis.WebSocket;
let session: Session | null = null;

afterEach(() => {
  session?.close();
  session = null;
  globalThis.fetch = realFetch;
  globalThis.WebSocket = realWS;
  FakeSocket.all = [];
  localStorage.clear();
  sessionStorage.clear();
  document.body.innerHTML = "";
});

async function flush(): Promise<void> {
  for (let i = 0; i < 12; i++) await new Promise((r) => setTimeout(r, 0));
}

function rulesetBody(abilities: Array<[string, string]>, resources: Array<[string, string]>): string {
  return JSON.stringify({
    id: "qa-rules",
    name: "QA rules",
    abilities: abilities.map(([id, name]) => ({
      id, name, range: 1, maxTargets: 1, usage: { kind: "atWill" },
    })),
    conditions: [],
    resources: resources.map(([key]) => key),
    properties: resources.map(([key, name]) => ({ key, type: "resource", name, description: "" })),
  });
}

interface PartyMember {
  id: string;
  name: string;
  abilityIds: string[];
}

interface Table {
  root: HTMLElement;
  ws: FakeSocket;
  reads: Read[];
  pending(): Read[];
  give(actorId: string, abilityId: string): void;
  spend(actorId: string, resource: string, newValue: number): void;
  set(actors: PartyMember[], controlsNone?: boolean): void;
  story(): string;
  toast(): string;
  offered(): string[];
}

/**
 * Boot a window whose /api/me answers `role`, open its socket on the first
 * party member's log, send the first set, and answer every read issued so far
 * with `baseline`, in issue order.
 */
async function openTable(
  role: Role,
  party: PartyMember[],
  baseline: Array<[string, string]>,
): Promise<Table> {
  localStorage.setItem("vtt.token", "qa-token");
  const reads: Read[] = [];
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  globalThis.fetch = ((input: RequestInfo | URL) => {
    const u = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const path = u.replace(/^[a-z]+:\/\/[^/]*/, "").replace(/^null/, "").split("?")[0];
    if (path === "/api/me") {
      return Promise.resolve(new Response(JSON.stringify({ participantId: "p1", name: "Pat", role })));
    }
    if (path === "/api/ruleset") {
      return new Promise<Response>((resolve) => {
        const r: Read = {
          settled: false,
          answer(abilities, resources = []) {
            r.settled = true;
            resolve(new Response(rulesetBody(abilities, resources), { headers: { "content-type": "application/json" } }));
          },
          fail(status = 500) {
            r.settled = true;
            resolve(new Response("ruleset unavailable", { status }));
          },
        };
        reads.push(r);
      });
    }
    if (path === "/api/adventures") return Promise.resolve(new Response(JSON.stringify({ adventures: [] })));
    if (path === "/api/maps") return Promise.resolve(new Response(JSON.stringify({ maps: [], cellPx: 64 })));
    return Promise.resolve(new Response("not found", { status: 404 }));
  }) as typeof fetch;

  document.body.innerHTML = '<div id="app"></div>';
  const root = document.getElementById("app") as HTMLElement;
  session = boot(root);
  await flush();
  const ws = FakeSocket.all[0];
  if (!ws) throw new Error("boot opened no socket");

  let seq = 0n;
  const event = (frame: FrameInit): void => ws.deliver(frame);
  const controller = role === "player" ? "p1" : "p2";
  const watched = party[0]?.id ?? "";

  ws.deliver({ case: "catchUpHead", value: { headSequence: 0n } });
  ws.deliver({ case: "presenceSnapshot", value: { present: [] } });
  if (role === "player" || role === "spectator") ws.deliver({ case: "viewpointChanged", value: { actorId: watched } });
  for (const m of party) {
    seq += 1n;
    event({ case: "event", value: { eventId: `add-${m.id}`, sequence: seq, payload: { case: "actorAdded", value: { actor: { actorId: m.id, name: m.name, abilityIds: m.abilityIds, resources: { vigor: { current: 5, max: 5 } } } } } } });
    seq += 1n;
    event({ case: "event", value: { eventId: `grant-${m.id}`, sequence: seq, payload: { case: "actorControlGranted", value: { actorId: m.id, participantId: controller, kind: ActorKind.PARTY_MEMBER } } } });
  }

  const t: Table = {
    root,
    ws,
    reads,
    pending: () => reads.filter((r) => !r.settled),
    give(actorId, abilityId) {
      seq += 1n;
      event({ case: "event", value: { eventId: `give-${actorId}-${abilityId}`, sequence: seq, payload: { case: "abilityGiven", value: { actorId, abilityId } } } });
    },
    spend(actorId, resource, newValue) {
      seq += 1n;
      event({ case: "event", value: { eventId: `spend-${actorId}-${seq}`, sequence: seq, payload: { case: "resourceChanged", value: { actorId, resource, delta: -1, newValue } } } });
    },
    set(actors, controlsNone = false) {
      ws.deliver({
        case: "watchableChanged",
        value: {
          actors: actors.map((a) => ({ actorId: a.id, name: a.name, abilityIds: a.abilityIds })),
          controlsNoPartyMember: controlsNone,
        },
      });
    },
    story: () => root.querySelector("section.feed")?.textContent ?? "",
    toast: () => root.querySelector(".toast")?.textContent ?? "",
    offered() {
      const panel = root.querySelector("section.player");
      if (!panel) return [];
      const heading = Array.from(panel.querySelectorAll("h3")).find((h) => h.textContent === "Abilities");
      const row = heading?.nextElementSibling;
      if (!row) return [];
      return Array.from(row.querySelectorAll("button")).map((b) => b.textContent ?? "");
    },
  };
  t.set(party);
  await flush();
  for (const r of t.pending()) {
    r.answer(baseline);
    await flush();
  }
  return t;
}

const story = (actor: string, ability: string): string => `${actor} can now use ${ability}`;

describe("two /api/ruleset reads in flight in a player's window", () => {
  // VTT-753 VTT-734 VTT-733
  test("an older read's answer arriving after the newest names its ability in the Story, leaves the panel on the newest answer, and tells its own set's change", async () => {
    const t = await openTable("player", [{ id: "a1", name: "Aria", abilityIds: ["strike"] }], [["strike", "Strike"]]);
    expect(t.pending()).toHaveLength(0);
    expect(t.offered()).toEqual(["Strike"]);
    expect(t.toast()).toBe("");

    t.give("a1", "fireball");
    t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] }]);
    await flush();
    expect(t.pending()).toHaveLength(1);
    const older = t.pending()[0]!;
    expect(t.toast()).toBe("");
    expect(t.story()).toContain(story("Aria", "fireball"));

    t.give("a1", "heal");
    t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball", "heal"] }]);
    await flush();
    expect(t.pending()).toHaveLength(2);
    const newest = t.pending()[1]!;

    newest.answer([["strike", "Strike"], ["heal", "Heal"]]);
    await flush();
    expect(t.offered()).toEqual(["Strike", "Heal"]);
    expect(t.toast()).toBe("Aria can now use Heal.");
    expect(t.story()).toContain(story("Aria", "fireball"));
    expect(t.story()).toContain(story("Aria", "Heal"));

    older.answer([["strike", "Strike"], ["fireball", "Fireball"]]);
    await flush();
    expect(t.story()).toContain(story("Aria", "Fireball"));
    expect(t.story()).not.toContain(story("Aria", "fireball"));
    expect(t.offered()).toEqual(["Strike", "Heal"]);
    expect(t.toast()).toBe("Aria can now use Fireball.");
    expect(t.pending()).toHaveLength(0);
  });

  // VTT-753 VTT-734 VTT-733
  test("an older read failing after the newest answered names its ability by its id, keeps the panel on the newest answer, and still tells its set's change", async () => {
    const t = await openTable("player", [{ id: "a1", name: "Aria", abilityIds: ["strike"] }], [["strike", "Strike"]]);

    t.give("a1", "fireball");
    t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] }]);
    await flush();
    const older = t.pending()[0]!;
    t.give("a1", "heal");
    t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball", "heal"] }]);
    await flush();
    expect(t.pending()).toHaveLength(2);
    const newest = t.pending()[1]!;

    newest.answer([["strike", "Strike"], ["heal", "Heal"]]);
    await flush();
    expect(t.toast()).toBe("Aria can now use Heal.");

    older.fail(500);
    await flush();
    expect(t.story()).toContain(story("Aria", "fireball"));
    expect(t.story()).toContain(story("Aria", "Heal"));
    expect(t.offered()).toEqual(["Strike", "Heal"]);
    expect(t.toast()).toBe("Aria can now use fireball.");
    expect(t.reads.filter((r) => !r.settled)).toHaveLength(0);
  });

  // VTT-734 VTT-733 VTT-753
  test("an older read's answer arriving while the newest is pending names its ability and tells its change; the newest answer then sets the panel", async () => {
    const t = await openTable("player", [{ id: "a1", name: "Aria", abilityIds: ["strike"] }], [["strike", "Strike"]]);

    t.give("a1", "fireball");
    t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] }]);
    await flush();
    const older = t.pending()[0]!;
    t.give("a1", "heal");
    t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball", "heal"] }]);
    await flush();
    const newest = t.pending()[1]!;

    older.answer([["strike", "Strike"], ["fireball", "Fireball"]]);
    await flush();
    expect(t.story()).toContain(story("Aria", "Fireball"));
    expect(t.toast()).toBe("Aria can now use Fireball.");

    newest.answer([["strike", "Strike"], ["heal", "Heal"]]);
    await flush();
    expect(t.offered()).toEqual(["Strike", "Heal"]);
    expect(t.story()).toContain(story("Aria", "Fireball"));
    expect(t.toast()).toBe("Aria can now use Heal.");
  });

  // VTT-759 VTT-734
  test("an older read's answer arriving last replaces the newest answer's display name in the Story, and its own message names the ability by it", async () => {
    const t = await openTable("player", [{ id: "a1", name: "Aria", abilityIds: ["strike"] }], [["strike", "Strike"]]);

    t.give("a1", "fireball");
    t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] }]);
    await flush();
    const older = t.pending()[0]!;
    t.give("a1", "heal");
    t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball", "heal"] }]);
    await flush();
    const newest = t.pending()[1]!;

    newest.answer([["strike", "Strike"], ["fireball", "Fire Bolt"], ["heal", "Heal"]]);
    await flush();
    expect(t.story()).toContain(story("Aria", "Fire Bolt"));
    expect(t.toast()).toBe("Aria can now use Heal.");

    older.answer([["strike", "Strike"], ["fireball", "Fireball"]]);
    await flush();
    expect(t.story()).toContain(story("Aria", "Fireball"));
    expect(t.story()).not.toContain(story("Aria", "Fire Bolt"));
    expect(t.toast()).toBe("Aria can now use Fireball.");
    expect(t.offered()).toEqual(["Strike", "Fire Bolt", "Heal"]);
  });

  for (const outcome of ["answers", "fails"] as const) {
    // VTT-753 VTT-733 VTT-758
    test(`an older read for a set that changed no ability ids, which ${outcome} after the newest answered, leaves the newest's message and panel standing`, async () => {
      const t = await openTable(
        "player",
        [
          { id: "a1", name: "Aria", abilityIds: ["strike"] },
          { id: "a2", name: "Bram", abilityIds: ["strike"] },
        ],
        [["strike", "Strike"]],
      );

      t.set([{ id: "a1", name: "Aria", abilityIds: ["strike"] }]);
      await flush();
      expect(t.pending()).toHaveLength(1);
      const older = t.pending()[0]!;
      expect(t.toast()).toBe("You no longer play Bram.");

      t.give("a1", "fireball");
      t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] }]);
      await flush();
      expect(t.pending()).toHaveLength(2);
      const newest = t.pending()[1]!;

      newest.answer([["strike", "Strike"], ["fireball", "Fireball"]]);
      await flush();
      expect(t.toast()).toBe("Aria can now use Fireball.");
      expect(t.offered()).toEqual(["Strike", "Fireball"]);

      if (outcome === "answers") older.answer([["strike", "Strike"]]);
      else older.fail(500);
      await flush();
      expect(t.toast()).toBe("Aria can now use Fireball.");
      expect(t.offered()).toEqual(["Strike", "Fireball"]);
      expect(t.story()).toContain(story("Aria", "Fireball"));
    });
  }

  for (const outcome of ["answers", "fails"] as const) {
    // VTT-605 VTT-733 VTT-753 VTT-758
    test(`the newest read, for a set that changed no ability ids, ${outcome} and leaves "You no longer play" standing; the older read's answer then tells its change`, async () => {
      const t = await openTable(
        "player",
        [
          { id: "a1", name: "Aria", abilityIds: ["strike"] },
          { id: "a2", name: "Bram", abilityIds: ["strike"] },
        ],
        [["strike", "Strike"]],
      );

      t.give("a1", "fireball");
      t.set([
        { id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] },
        { id: "a2", name: "Bram", abilityIds: ["strike"] },
      ]);
      await flush();
      expect(t.pending()).toHaveLength(1);
      const older = t.pending()[0]!;
      expect(t.toast()).toBe("");

      t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] }]);
      await flush();
      expect(t.pending()).toHaveLength(2);
      const newest = t.pending()[1]!;
      expect(t.toast()).toBe("You no longer play Bram.");

      if (outcome === "answers") newest.answer([["strike", "Strike"]]);
      else newest.fail(500);
      await flush();
      expect(t.toast()).toBe("You no longer play Bram.");
      expect(t.offered()).toEqual(["Strike"]);

      older.answer([["strike", "Strike"], ["fireball", "Fireball"]]);
      await flush();
      expect(t.toast()).toBe("Aria can now use Fireball.");
      expect(t.story()).toContain(story("Aria", "Fireball"));
      expect(t.offered()).toEqual(["Strike"]);
    });
  }
});

// VTT-756 VTT-753
test("an older read's answer arriving after the newest does not change how the Story names a resource", async () => {
  const t = await openTable("player", [{ id: "a1", name: "Aria", abilityIds: ["strike"] }], [["strike", "Strike"]]);
  t.spend("a1", "vigor", 4);

  t.give("a1", "fireball");
  t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] }]);
  await flush();
  const older = t.pending()[0]!;
  t.give("a1", "heal");
  t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball", "heal"] }]);
  await flush();
  expect(t.pending()).toHaveLength(2);
  const newest = t.pending()[1]!;

  newest.answer([["strike", "Strike"], ["heal", "Heal"]], [["vigor", "Stamina"]]);
  await flush();
  expect(t.story()).toContain("Stamina");

  older.answer([["strike", "Strike"], ["fireball", "Fireball"]], [["vigor", "Old Vigor"]]);
  await flush();
  expect(t.story()).toContain("Stamina");
  expect(t.story()).not.toContain("Old Vigor");
});

describe("a window that is not a player's", () => {
  for (const role of ["spectator", "dm", "agent"] as const) {
    // VTT-752
    test(`${role === "agent" ? "an" : "a"} ${role}'s window shows no message and issues no read for sets that change a party member's ability ids`, async () => {
      const t = await openTable(role, [{ id: "a1", name: "Aria", abilityIds: ["strike"] }], [["strike", "Strike"]]);
      expect(t.reads).toHaveLength(1);

      t.give("a1", "fireball");
      t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball"] }]);
      await flush();
      t.give("a1", "heal");
      t.set([{ id: "a1", name: "Aria", abilityIds: ["strike", "fireball", "heal"] }]);
      await flush();

      expect(t.reads).toHaveLength(1);
      expect(t.toast()).toBe("");
    });
  }
});
