import { test, expect } from "bun:test";
import { fetchRuleset, fetchAdventures, fetchRulesetGuide, fetchAdventureGuide, fetchMe } from "../src/metadata";

function fakeAPI(routes: Record<string, { status?: number; body: unknown }>) {
  const seenAuth: string[] = [];
  const server = Bun.serve({
    port: 0,
    fetch(req) {
      seenAuth.push(req.headers.get("Authorization") ?? "");
      const path = new URL(req.url).pathname;
      const r = routes[path];
      if (!r) return new Response("not found", { status: 404 });
      return new Response(JSON.stringify(r.body), {
        status: r.status ?? 200,
        headers: { "content-type": "application/json" },
      });
    },
  });
  return { base: `http://localhost:${server.port}`, seenAuth, stop: () => server.stop(true) };
}

test("the token travels as a Bearer header, never in the URL", async () => {
  // A token in a query string leaks into access logs, Referer and browser
  // history. internal/gateway/metadata.go rejects that posture deliberately;
  // this pins the client half of the same decision.
  const api = fakeAPI({ "/api/ruleset": { body: { id: "rs", name: "RS", abilities: [], conditions: [], resources: [] } } });
  try {
    await fetchRuleset(api.base, "secret-token");
    expect(api.seenAuth[0]).toBe("Bearer secret-token");
  } finally {
    api.stop();
  }
});

test("an empty ruleset is a value, not an error", async () => {
  // The server answers 200 with empty collections when nothing is loaded, so
  // the UI can render an empty picker instead of an error banner.
  const api = fakeAPI({
    "/api/ruleset": { body: { id: "", name: "", abilities: [], conditions: [], resources: [] } },
  });
  try {
    const rs = await fetchRuleset(api.base, "t");
    expect(rs.abilities).toEqual([]);
    expect(rs.resources).toEqual([]);
  } finally {
    api.stop();
  }
});

test("a missing guide returns null rather than throwing", async () => {
  // 404 here means "there is no guide", which is ordinary. Throwing would
  // make every caller wrap this in try/catch to render an empty panel.
  const api = fakeAPI({ "/api/ruleset/guide": { status: 404, body: {} } });
  try {
    expect(await fetchRulesetGuide(api.base, "t")).toBeNull();
  } finally {
    api.stop();
  }
});

test("an adventure guide refused by role is distinguishable from an absent one", async () => {
  // 403 and 404 mean different things to a DM debugging why a panel is empty:
  // "you may not read this" versus "there is nothing to read".
  const api = fakeAPI({ "/api/adventures/x/guide": { status: 403, body: {} } });
  try {
    await expect(fetchAdventureGuide(api.base, "t", "x")).rejects.toThrow(/forbidden/i);
  } finally {
    api.stop();
  }
});

test("adventures decode into id/name pairs", async () => {
  const api = fakeAPI({
    "/api/adventures": { body: { adventures: [{ id: "a", name: "A" }, { id: "b", name: "B" }] } },
  });
  try {
    const advs = await fetchAdventures(api.base, "t");
    expect(advs.map((a) => a.id)).toEqual(["a", "b"]);
  } finally {
    api.stop();
  }
});

test("an unauthorized response is surfaced, not silently empty", async () => {
  // A silently-empty ruleset on a bad token would look identical to a server
  // with nothing loaded, and the user would never learn their token expired.
  const api = fakeAPI({ "/api/ruleset": { status: 401, body: {} } });
  try {
    await expect(fetchRuleset(api.base, "bad")).rejects.toThrow(/unauthorized/i);
  } finally {
    api.stop();
  }
});

test("fetchMe reports the caller's role and controls", async () => {
  const api = fakeAPI({
    "/api/me": { body: { participantId: "p-1", name: "Lera", role: "player", controls: ["a1"] } },
  });
  try {
    const me = await fetchMe(api.base, "t");
    expect(me.role).toBe("player");
    expect(me.participantId).toBe("p-1");
    expect(me.controls).toEqual(["a1"]);
  } finally {
    api.stop();
  }
});
