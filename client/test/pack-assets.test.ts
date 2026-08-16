import { test, expect } from "bun:test";
import {
  packFileURL, packManifestURL, imageRequestsForPack, loadPackImages,
  type PackManifestJSON,
} from "../src/view/pack-assets";

test("packFileURL builds the one route every pack file is served from, URL-encoded", () => {
  expect(packFileURL("http://x", "cellar-basics", "earth_1.png")).toBe(
    "http://x/api/packs/cellar-basics/earth_1.png",
  );
  // A pack id or filename with characters that would otherwise reshape the
  // path (a space, a slash) must not be able to.
  expect(packFileURL("http://x", "a b", "c/d.png")).toBe("http://x/api/packs/a%20b/c%2Fd.png");
});

test("packManifestURL is packFileURL of pack.json — one route, not a second convention", () => {
  expect(packManifestURL("http://x", "cellar-basics")).toBe(
    packFileURL("http://x", "cellar-basics", "pack.json"),
  );
});

// --- imageRequestsForPack: pure, no network -------------------------------

const base = "http://x";
const pack = "p";

test("a tile with a plain file resolves to its unmarked key", () => {
  const manifest: PackManifestJSON = {
    id: "p", name: "P", cell_px: 64,
    tiles: [{ name: "masonry-1", file: "masonry_1.png" }],
    objects: [],
  };
  expect(imageRequestsForPack(base, pack, manifest)).toEqual([
    { key: "tile:masonry-1", url: packFileURL(base, pack, "masonry_1.png") },
  ]);
});

test("a door tile's closed picture is the UNMARKED key; open gets the /open suffix", () => {
  // Matches scene-plan.ts's tileImage exactly: closed is the unmarked
  // default, only "open" gets a marker — a mismatch here would make a real
  // door draw shut while folded state says open, with nothing to catch it.
  const manifest: PackManifestJSON = {
    id: "p", name: "P", cell_px: 64,
    tiles: [{
      name: "cellar-door", kind: "door", material: "wood",
      file_closed: "door_shut.png", file_open: "door_open.png",
    }],
    objects: [],
  };
  expect(imageRequestsForPack(base, pack, manifest)).toEqual([
    { key: "tile:cellar-door", url: packFileURL(base, pack, "door_shut.png") },
    { key: "tile:cellar-door/open", url: packFileURL(base, pack, "door_open.png") },
  ]);
});

test("an object resolves under the SAME 'tile:' key namespace a tile override uses", () => {
  // scene-plan.ts's objectImage returns `tile:${obj.Art}` unconditionally —
  // an object's art and a tile override's art share one ImageMap namespace,
  // even though pack.json keeps them in two separate arrays.
  const manifest: PackManifestJSON = {
    id: "p", name: "P", cell_px: 64,
    tiles: [],
    objects: [{ name: "crate-wood", file: "crate_wood.png" }],
  };
  expect(imageRequestsForPack(base, pack, manifest)).toEqual([
    { key: "tile:crate-wood", url: packFileURL(base, pack, "crate_wood.png") },
  ]);
});

test("a tile entry naming no file at all resolves to nothing, rather than a broken request", () => {
  const manifest: PackManifestJSON = {
    id: "p", name: "P", cell_px: 64,
    tiles: [{ name: "ghost" }],
    objects: [],
  };
  expect(imageRequestsForPack(base, pack, manifest)).toEqual([]);
});

test("tiles and objects both contribute, in their own array order", () => {
  const manifest: PackManifestJSON = {
    id: "p", name: "P", cell_px: 64,
    tiles: [
      { name: "earth-1", file: "earth_1.png" },
      { name: "flagstone-1", file: "flagstone_1.png" },
    ],
    objects: [{ name: "barrel", file: "barrel.png" }],
  };
  expect(imageRequestsForPack(base, pack, manifest).map((r) => r.key)).toEqual([
    "tile:earth-1", "tile:flagstone-1", "tile:barrel",
  ]);
});

// --- loadPackImages: the thin fetch/decode orchestration ------------------

interface Call { url: string; auth: string | null; }

function fakeFetch(
  calls: Call[],
  routes: Record<string, { status: number; body: string }>,
): typeof fetch {
  return (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    calls.push({ url, auth: (init?.headers as Record<string, string> | undefined)?.["Authorization"] ?? null });
    const r = routes[url];
    if (!r) return new Response("", { status: 404 });
    return new Response(r.body, { status: r.status });
  }) as typeof fetch;
}

test("the manifest and every image are fetched with the token as a Bearer header, never a query", () => {
  const calls: Call[] = [];
  const manifestURL = packManifestURL("http://x", "p");
  const imgURL = packFileURL("http://x", "p", "earth_1.png");
  const manifest: PackManifestJSON = {
    id: "p", name: "P", cell_px: 64,
    tiles: [{ name: "earth-1", file: "earth_1.png" }],
    objects: [],
  };
  const fx = fakeFetch(calls, {
    [manifestURL]: { status: 200, body: JSON.stringify(manifest) },
    [imgURL]: { status: 200, body: "fake-bytes" },
  });
  const decoded: unknown[] = [];
  return loadPackImages("http://x", "secret", "p", fx, async (blob) => {
    decoded.push(blob);
    return "decoded-image" as unknown as CanvasImageSource;
  }).then((images) => {
    expect(images as unknown as Record<string, string>).toEqual({ "tile:earth-1": "decoded-image" });
    expect(calls).toHaveLength(2);
    for (const c of calls) expect(c.auth).toBe("Bearer secret");
    expect(calls.map((c) => c.url)).toContain(manifestURL);
    expect(calls.map((c) => c.url)).toContain(imgURL);
  });
});

test("a manifest that 404s resolves to an empty pack, not a thrown error", async () => {
  const fx = fakeFetch([], {});
  const images = await loadPackImages("http://x", "t", "missing", fx, async () => "img" as unknown as CanvasImageSource);
  expect(images).toEqual({});
});

test("one bad file does not take down the rest of the pack", async () => {
  // Boot validation (maps.go's loadMapsDir) is what should have caught a
  // genuinely missing file before a table ever sees it; a client that still
  // hits one anyway must render everything ELSE rather than nothing at all.
  const manifestURL = packManifestURL("http://x", "p");
  const goodURL = packFileURL("http://x", "p", "good.png");
  const manifest: PackManifestJSON = {
    id: "p", name: "P", cell_px: 64,
    tiles: [
      { name: "good", file: "good.png" },
      { name: "bad", file: "missing.png" },
    ],
    objects: [],
  };
  const fx = fakeFetch([], {
    [manifestURL]: { status: 200, body: JSON.stringify(manifest) },
    [goodURL]: { status: 200, body: "bytes" },
    // missingURL deliberately has no route: fakeFetch answers 404.
  });
  const images = await loadPackImages("http://x", "t", "p", fx, async () => "img" as unknown as CanvasImageSource);
  expect(images as unknown as Record<string, string>).toEqual({ "tile:good": "img" });
});

test("a decode failure for one image does not take down the rest of the pack, or reject the promise", async () => {
  const manifestURL = packManifestURL("http://x", "p");
  const aURL = packFileURL("http://x", "p", "a.png");
  const bURL = packFileURL("http://x", "p", "b.png");
  const manifest: PackManifestJSON = {
    id: "p", name: "P", cell_px: 64,
    tiles: [
      { name: "a", file: "a.png" },
      { name: "b", file: "b.png" },
    ],
    objects: [],
  };
  const fx = fakeFetch([], {
    [manifestURL]: { status: 200, body: JSON.stringify(manifest) },
    [aURL]: { status: 200, body: "corrupt" },
    [bURL]: { status: 200, body: "fine" },
  });
  const images = await loadPackImages("http://x", "t", "p", fx, async (blob) => {
    const text = await blob.text();
    if (text === "corrupt") throw new Error("decode boom");
    return `decoded:${text}` as unknown as CanvasImageSource;
  });
  // "a" threw and is absent; "b" still decoded — one bad file, not a bad pack.
  expect(images as unknown as Record<string, string>).toEqual({ "tile:b": "decoded:fine" });
});
