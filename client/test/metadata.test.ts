import { test, expect } from "bun:test";
import { fetchRuleset, fetchAdventures, fetchRulesetGuide, fetchAdventureGuide, fetchMe } from "../src/metadata";

function fakeAPI(routes: Record<string, { status?: number; body: unknown }>) {
  const seenAuth: string[] = [];
  const seenPaths: string[] = [];
  const server = Bun.serve({
    port: 0,
    fetch(req) {
      seenAuth.push(req.headers.get("Authorization") ?? "");
      const path = new URL(req.url).pathname;
      seenPaths.push(path);
      const r = routes[path];
      if (!r) return new Response("not found", { status: 404 });
      return new Response(JSON.stringify(r.body), {
        status: r.status ?? 200,
        headers: { "content-type": "application/json" },
      });
    },
  });
  return { base: `http://localhost:${server.port}`, seenAuth, seenPaths, stop: () => server.stop(true) };
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

// --- the failure paths ------------------------------------------------------
//
// Everything below pins an error path. The pattern the mutation gate keeps
// exposing across this client is that the happy path is tested and the
// refusals are not — so a blanked message or a deleted guard survives every
// test in the file. These assert the exact wording, because these strings are
// the only explanation a user or a DM ever gets.

test("an unexpected status names the route and the code", async () => {
  // Not 200/404/401/403 — a 500, a 502 from a proxy, anything. The default
  // arm is what turns "the panel is empty" into something reportable, and it
  // has to say WHICH route and WHAT code or it is no better than silence.
  const api = fakeAPI({ "/api/ruleset": { status: 500, body: {} } });
  try {
    await expect(fetchRuleset(api.base, "t")).rejects.toThrow("metadata: /api/ruleset returned 500");
  } finally {
    api.stop();
  }
});

test("a 404 on /api/me is a failure, not an absence", async () => {
  // Unlike a guide, identity has no meaningful "not there" state: a client
  // that cannot learn its own role cannot decide which panels to render, so
  // returning null would push an impossible state into every caller.
  const api = fakeAPI({});
  try {
    await expect(fetchMe(api.base, "t")).rejects.toThrow("metadata: /api/me is unavailable");
  } finally {
    api.stop();
  }
});

test("a 404 on /api/ruleset means the route is missing, and says so", async () => {
  // The server answers 200-with-empty-collections when nothing is loaded, so
  // a 404 cannot mean "no ruleset" — it means the endpoint is not there at
  // all, which is a different problem with a different fix.
  const api = fakeAPI({});
  try {
    await expect(fetchRuleset(api.base, "t")).rejects.toThrow("metadata: /api/ruleset is unavailable");
  } finally {
    api.stop();
  }
});

test("absent adventures are an empty list, not a crash and not a placeholder", async () => {
  // getJSON returns null on 404, so the optional chain and the `?? []` are
  // both load-bearing: without the chain this throws on null, and without the
  // fallback the caller gets undefined and renders nothing at all. The list
  // must also be EMPTY — a non-empty default would put a phantom adventure in
  // the picker.
  const api = fakeAPI({});
  try {
    const advs = await fetchAdventures(api.base, "t");
    expect(advs).toEqual([]);
    expect(advs.length).toBe(0);
  } finally {
    api.stop();
  }
});

test("the ruleset guide is read from /api/ruleset/guide and returned verbatim", async () => {
  // Asserting the TEXT, not just non-null: the path is a bare string literal,
  // and a blanked one still yields a well-formed null via the 404 arm — which
  // the "missing guide returns null" test above accepts. Only a guide that
  // actually comes back distinguishes the right route from no route.
  const api = fakeAPI({ "/api/ruleset/guide": { body: { guide: "# How to play\nRoll high." } } });
  try {
    expect(await fetchRulesetGuide(api.base, "t")).toBe("# How to play\nRoll high.");
    expect(api.seenPaths).toContain("/api/ruleset/guide");
  } finally {
    api.stop();
  }
});

test("an adventure guide is returned verbatim, and its absence is null", async () => {
  // fetchAdventureGuide's own `?? null` was unpinned: every existing test hit
  // its 403 arm. `??` and `&&` differ exactly here — on a guide that EXISTS,
  // `&&` yields null and the panel silently renders empty for an adventure
  // that has a guide.
  const api = fakeAPI({ "/api/adventures/keep/guide": { body: { guide: "The keep is cold." } } });
  try {
    expect(await fetchAdventureGuide(api.base, "t", "keep")).toBe("The keep is cold.");
    expect(await fetchAdventureGuide(api.base, "t", "absent")).toBeNull();
  } finally {
    api.stop();
  }
});

test("an adventure id is URL-encoded on the way into the path", async () => {
  const api = fakeAPI({});
  try {
    await fetchAdventureGuide(api.base, "t", "a b/c");
    expect(api.seenPaths).toContain("/api/adventures/a%20b%2Fc/guide");
  } finally {
    api.stop();
  }
});
