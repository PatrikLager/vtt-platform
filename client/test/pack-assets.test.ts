import { test, expect } from "bun:test";
import {
  packFileURL, packManifestURL, imageRequestsForPack, loadPackImages,
  standardPackFileURL, imageRequestsForStandardPack, loadStandardPackImages,
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

// --- the standard-vocabulary baseline pack (review finding C2, 2026-08-16) -
//
// Every one of internal/mapdef/standard.go's eleven natures needs a picture,
// or a square with NO art override (both shipped adventures, per the
// finding) draws nothing. This pack ships from the client's OWN bundle
// (tools/genmappack/std_pack.go's own header comment has the full why) at a
// fixed path, "/std-pack/...", not the authenticated GET
// /api/packs/{pack}/{file} route — so unlike loadPackImages above, there is
// no pack id and no bearer token anywhere in this section.

test("standardPackFileURL builds under /std-pack, URL-encoded", () => {
  expect(standardPackFileURL("http://x", "std_earth_floor.png")).toBe(
    "http://x/std-pack/std_earth_floor.png",
  );
  // A filename that would otherwise reshape the path must not be able to —
  // mirrors packFileURL's own encoding test above.
  expect(standardPackFileURL("http://x", "a/b.png")).toBe("http://x/std-pack/a%2Fb.png");
});

test("imageRequestsForStandardPack keys by kind/material, NOT by name", () => {
  // scene-plan.ts's tileImage emits `std:${tile.Kind}/${tile.Material}` for
  // an unoverridden square — it never sees this pack's own tile NAMES at
  // all, so the key this file builds has to come from kind/material, unlike
  // imageRequestsForPack's tile: keys above (which key by name because a
  // pack override IS addressed by name).
  const manifest: PackManifestJSON = {
    id: "std", name: "Standard Vocabulary", cell_px: 64,
    tiles: [{ name: "earth", kind: "floor", material: "earth", file: "std_earth_floor.png" }],
    objects: [],
  };
  expect(imageRequestsForStandardPack("http://x", manifest)).toEqual([
    { key: "std:floor/earth", url: standardPackFileURL("http://x", "std_earth_floor.png") },
  ]);
});

test("a standard door's closed picture is the UNMARKED std: key; open gets /open", () => {
  // Same open/closed convention as imageRequestsForPack's door test above,
  // applied to the kind/material-derived key instead of a name-derived one.
  const manifest: PackManifestJSON = {
    id: "std", name: "Standard Vocabulary", cell_px: 64,
    tiles: [{
      name: "wood-door", kind: "door", material: "wood",
      file_closed: "std_wood_door_closed.png", file_open: "std_wood_door_open.png",
    }],
    objects: [],
  };
  expect(imageRequestsForStandardPack("http://x", manifest)).toEqual([
    { key: "std:door/wood", url: standardPackFileURL("http://x", "std_wood_door_closed.png") },
    { key: "std:door/wood/open", url: standardPackFileURL("http://x", "std_wood_door_open.png") },
  ]);
});

test("a tile entry naming no kind or material contributes nothing — defensive, not expected of a real std pack", () => {
  const manifest: PackManifestJSON = {
    id: "std", name: "Standard Vocabulary", cell_px: 64,
    tiles: [{ name: "ghost", file: "ghost.png" }],
    objects: [],
  };
  expect(imageRequestsForStandardPack("http://x", manifest)).toEqual([]);
});

test("all eleven standard entries contribute, in pack.json's own order", () => {
  const manifest: PackManifestJSON = {
    id: "std", name: "Standard Vocabulary", cell_px: 64,
    tiles: [
      { name: "stone-wall", kind: "wall", material: "stone", file: "a.png" },
      { name: "wood-wall", kind: "wall", material: "wood", file: "b.png" },
      { name: "wood-door", kind: "door", material: "wood", file_closed: "c.png", file_open: "d.png" },
      { name: "stone", kind: "floor", material: "stone", file: "e.png" },
      { name: "wood", kind: "floor", material: "wood", file: "f.png" },
      { name: "earth", kind: "floor", material: "earth", file: "g.png" },
      { name: "grass", kind: "floor", material: "grass", file: "h.png" },
      { name: "sand", kind: "floor", material: "sand", file: "i.png" },
      { name: "water", kind: "floor", material: "water", file: "j.png" },
      { name: "metal", kind: "floor", material: "metal", file: "k.png" },
      { name: "ice", kind: "floor", material: "ice", file: "l.png" },
    ],
    objects: [],
  };
  expect(imageRequestsForStandardPack("http://x", manifest).map((r) => r.key)).toEqual([
    "std:wall/stone", "std:wall/wood", "std:door/wood", "std:door/wood/open",
    "std:floor/stone", "std:floor/wood", "std:floor/earth", "std:floor/grass",
    "std:floor/sand", "std:floor/water", "std:floor/metal", "std:floor/ice",
  ]);
});

// --- loadStandardPackImages: the thin fetch/decode orchestration, unauthed -

test("the manifest and every image are fetched from /std-pack, with NO Authorization header", () => {
  const calls: Call[] = [];
  const manifestURL = standardPackFileURL("http://x", "pack.json");
  const imgURL = standardPackFileURL("http://x", "std_earth_floor.png");
  const manifest: PackManifestJSON = {
    id: "std", name: "Standard Vocabulary", cell_px: 64,
    tiles: [{ name: "earth", kind: "floor", material: "earth", file: "std_earth_floor.png" }],
    objects: [],
  };
  const fx = fakeFetch(calls, {
    [manifestURL]: { status: 200, body: JSON.stringify(manifest) },
    [imgURL]: { status: 200, body: "fake-bytes" },
  });
  return loadStandardPackImages("http://x", fx, async (blob) => {
    void blob;
    return "decoded-image" as unknown as CanvasImageSource;
  }).then((images) => {
    expect(images as unknown as Record<string, string>).toEqual({ "std:floor/earth": "decoded-image" });
    expect(calls).toHaveLength(2);
    // Unauthenticated by construction (this file's own header comment / the
    // static-route ruling): every call this loader makes carries no bearer
    // token at all, unlike loadPackImages above.
    for (const c of calls) expect(c.auth).toBeNull();
  });
});

test("a std-pack manifest that 404s resolves to an empty ImageMap, not a thrown error", async () => {
  const fx = fakeFetch([], {});
  const images = await loadStandardPackImages("http://x", fx, async () => "img" as unknown as CanvasImageSource);
  expect(images).toEqual({});
});

test("one bad standard-pack file does not take down the rest of the baseline", async () => {
  const manifestURL = standardPackFileURL("http://x", "pack.json");
  const goodURL = standardPackFileURL("http://x", "good.png");
  const manifest: PackManifestJSON = {
    id: "std", name: "Standard Vocabulary", cell_px: 64,
    tiles: [
      { name: "wood", kind: "floor", material: "wood", file: "good.png" },
      { name: "ice", kind: "floor", material: "ice", file: "missing.png" },
    ],
    objects: [],
  };
  const fx = fakeFetch([], {
    [manifestURL]: { status: 200, body: JSON.stringify(manifest) },
    [goodURL]: { status: 200, body: "bytes" },
  });
  const images = await loadStandardPackImages("http://x", fx, async () => "img" as unknown as CanvasImageSource);
  expect(images as unknown as Record<string, string>).toEqual({ "std:floor/wood": "img" });
});
