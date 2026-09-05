import { test, expect } from "bun:test";
import {
  standardPackFileURL, imageRequestsForStandardPack, loadStandardPackImages,
  type PackManifestJSON,
} from "../src/view/pack-assets";

// pack-assets.test.ts covers the STANDARD BASELINE PACK, and since 2026-09-05
// nothing else.
//
// TWELVE TESTS COVERING THE PER-PACK HALF STOOD ABOVE THIS LINE — packFileURL,
// packManifestURL, imageRequestsForPack and loadPackImages, with their manifest
// walk, their door key convention and their one-bad-file tolerance. Every one of
// those functions talked to GET /api/packs/{pack}/{file}, which
// 2026-09-02-art-is-a-flat-library Task 7 deleted with mapdef.Pack, and Task 6
// of that plan removed the client half.
//
// NOTHING IS UNCOVERED BY THE DELETION. Their replacement is
// client/test/art-assets.test.ts, which covers the same properties against the
// route that exists: the key convention (a door's closed picture unmarked, open
// suffixed), the Bearer header on every request, one bad file dropping only its
// own key, and a decode or a fetch that throws not rejecting the load. What is
// GONE rather than moved is the manifest walk, and it is gone because there is no
// manifest: art/ is flat and a Tile.Art names its own file.
//
// That file also carries the ABSENCE tests for the four deleted exports and for
// the dead route, which is the guard this deletion is owed — Task 5 removed the
// server field two tasks before anything removed the client code, and the
// TypeScript compiled cleanly the whole time.

// --- the standard-vocabulary baseline pack (review finding C2, 2026-08-16) -
//
// Every one of internal/mapdef/standard.go's eleven natures needs a picture,
// or a square with NO art override (both shipped adventures, per the
// finding) draws nothing. This pack ships from the client's OWN bundle
// (tools/genmappack/std_pack.go's own header comment has the full why) at a
// fixed path, "/std-pack/...", not the authenticated art route — so unlike
// art-assets.ts's loadArtImages, there is no id and no bearer token anywhere in
// this section.

test("standardPackFileURL builds under /std-pack, URL-encoded", () => {
  expect(standardPackFileURL("http://x", "std_earth_floor.png")).toBe(
    "http://x/std-pack/std_earth_floor.png",
  );
  // A filename that would otherwise reshape the path must not be able to —
  // the same encoding art-assets.ts's artFileURL applies, and for the same
  // reason: ask for a name that does not exist rather than walk a directory.
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

test("a standard tile naming only ONE of kind and material is skipped too — both halves of the key, or neither", () => {
  // The test above only exercises a tile missing BOTH halves, which any
  // guard shape agrees to skip; these two entries are the ones that tell
  // "either missing" apart from "both missing". Half a pair is worse than
  // no pair, which is why the guard cannot weaken to the both-missing
  // reading: the key is `std:${kind}/${material}`, so a kind-only tile
  // would key as "std:floor/undefined" — a string scene-plan.ts's tileImage
  // can never ask for, since it always builds the key from a folded tile's
  // OWN kind and material, both always set. The picture would be fetched
  // and decoded and then sit in the ImageMap unreachable, so the square it
  // was meant for still draws blank while the load looks entirely healthy.
  const manifest: PackManifestJSON = {
    id: "std", name: "Standard Vocabulary", cell_px: 64,
    tiles: [
      { name: "kind-only", kind: "floor", file: "half_a.png" },
      { name: "material-only", material: "earth", file: "half_b.png" },
    ],
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

// --- loadStandardPackImages: the thin fetch/decode orchestration -----------

interface Call {
  url: string;
  auth: string | null;
}

function fakeFetch(
  calls: Call[],
  routes: Record<string, { status: number; body: string }>,
): typeof fetch {
  return (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    calls.push({
      url,
      auth: (init?.headers as Record<string, string> | undefined)?.["Authorization"] ?? null,
    });
    const r = routes[url];
    if (!r) return new Response("", { status: 404 });
    return new Response(r.body, { status: r.status });
  }) as typeof fetch;
}

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
