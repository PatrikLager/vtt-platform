import { test, expect } from "bun:test";
import {
  artFileURL,
  artNamesInScene,
  loadArtImages,
  type ArtSidecarJSON,
} from "../src/view/art-assets";
import * as packAssets from "../src/view/pack-assets";
import { fitCamera } from "../src/view/camera";
import { planScene } from "../src/view/scene-plan";
import { readdirSync, readFileSync, lstatSync } from "node:fs";
import { join } from "node:path";
import { newState, type Scene } from "../src/state";

// --- what the wire actually gives this client ------------------------------
//
// A live Scene carries ART IDS and nothing else: TileRef.art and
// SceneObject.art are filename stems in the campaign's flat art/ directory
// (contract/vtt/v1/events.proto), because mapdef.Resolve turns a map's
// overrides into facts at compile time and stops there. So the client's whole
// job is: read the ids off the folded scene, and ask GET /api/art/{file} for
// the files each one implies — the same order artlib.lookupIn resolves in.

function scene(over: Partial<Scene>): Scene {
  return {
    ID: "hall",
    Name: "Hall",
    GridWidth: 2,
    GridHeight: 1,
    Tiles: {},
    Objects: [],
    OpenDoors: {},
    ...over,
  };
}

test("artFileURL is the one route every art file is served from, URL-encoded", () => {
  expect(artFileURL("http://x", "masonry-1.png")).toBe("http://x/api/art/masonry-1.png");
  // Encoded, so a name that somehow contains a "/" 404s on a name that does not
  // exist rather than silently reshaping the path into a directory walk. The
  // server refuses it either way (artlib.IsArtFileName), and this is the client
  // not being the one that asks.
  expect(artFileURL("http://x", "pack-ish/x.png")).toBe("http://x/api/art/pack-ish%2Fx.png");
});

// --- artNamesInScene: pure, no network -------------------------------------

test("every art id a scene names is collected, from tiles AND objects", () => {
  const s = scene({
    Tiles: {
      "0,0": { Kind: "wall", Material: "stone", Art: "masonry-1" },
      "1,0": { Kind: "floor", Material: "earth", Art: "earth-1" },
    },
    Objects: [
      {
        ObjectID: "o1", Kind: "pillar", X: 0, Y: 0, Width: 1, Height: 1,
        RotationDegrees: 0, BlocksSight: true, BlocksMove: true, Art: "pillar-stone",
      },
    ],
  });
  expect(artNamesInScene(s).sort()).toEqual(["earth-1", "masonry-1", "pillar-stone"]);
});

test("an art id named by ninety squares is asked for ONCE", () => {
  // The same deduplication mapdef's warnings make, for the same reason: a name
  // is a property of the piece, not of the square. Ninety fetches of one wall
  // texture is ninety round trips a table pays for on every scene.
  const tiles: Scene["Tiles"] = {};
  for (let i = 0; i < 90; i++) tiles![`${i},0`] = { Kind: "wall", Material: "stone", Art: "masonry-1" };
  expect(artNamesInScene(scene({ GridWidth: 90, Tiles: tiles }))).toEqual(["masonry-1"]);
});

// EVERY "names nothing" ASSERTION BELOW CHECKS THE LENGTH TOO, and that is not
// belt and braces. Measured 2026-09-05: `expect([undefined]).toEqual([])` PASSES
// in bun — toEqual ignores an undefined element rather than counting it — while
// `toHaveLength(0)` fails. A stray undefined in the returned array is exactly
// what a broken `?? []` fallback produces (reading `.Art` off a non-object gives
// undefined, and `undefined !== ""` is true), so toEqual alone is blind to the
// one failure these tests exist to catch. The TS mutation gate found it: the
// ArrayDeclaration mutant on `scene.Objects ?? []` survived the whole suite.

test("a square or an object with no art contributes nothing to fetch", () => {
  // An empty Art is the ordinary degraded state (spec §4) and the ordinary
  // UNOVERRIDDEN state: a tile draws from std:<kind>/<material> and an object
  // draws plain. Asking the server for "" would be one 404 per square.
  const s = scene({
    Tiles: { "0,0": { Kind: "floor", Material: "earth", Art: "" } },
    Objects: [
      {
        ObjectID: "o1", Kind: "pillar", X: 0, Y: 0, Width: 1, Height: 1,
        RotationDegrees: 0, BlocksSight: true, BlocksMove: true, Art: "",
      },
    ],
  });
  expect(artNamesInScene(s)).toEqual([]);
  expect(artNamesInScene(s)).toHaveLength(0);
});

test("a scene with no tiles and no objects at all is legal and names nothing", () => {
  // Tiles/Objects/OpenDoors are OPTIONAL on Scene (state.ts: a terrain-free
  // scene is legal, Patrik's ruling 2026-08-13), so this is the one place that
  // has to know it rather than every lookup needing its own guard.
  const bare = artNamesInScene({ ID: "h", Name: "H", GridWidth: 1, GridHeight: 1 });
  expect(bare).toEqual([]);
  // THE LENGTH IS THE REAL ASSERTION — see the note above this block. A broken
  // `?? []` fallback returns [undefined], which toEqual cannot tell from [].
  expect(bare).toHaveLength(0);
});

// --- loadArtImages: the fetch half -----------------------------------------

/** A fetch double that answers a table of paths and records what was asked. */
function fakeFetch(table: Record<string, Response | (() => Response)>) {
  const asked: { path: string; auth: string | null }[] = [];
  const impl = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = new URL(String(input)).pathname;
    asked.push({
      path,
      auth: (init?.headers as Record<string, string> | undefined)?.["Authorization"] ?? null,
    });
    const hit = table[path];
    if (!hit) return new Response("", { status: 404 });
    return typeof hit === "function" ? hit() : hit.clone();
  }) as typeof fetch;
  return { asked, impl };
}

/** A decoder that tags each image with the bytes it came from, so a test can
 *  say WHICH file reached WHICH key — "something decoded" would pass with two
 *  pictures swapped. */
const tagDecode = async (blob: Blob) => ({ tag: await blob.text() }) as unknown as CanvasImageSource;

function sidecar(body: ArtSidecarJSON): Response {
  return new Response(JSON.stringify(body), { status: 200 });
}

test("a piece with a plain picture resolves to the unmarked key", async () => {
  const { asked, impl } = fakeFetch({
    "/api/art/masonry-1.json": sidecar({ format_version: 1, kind: "wall", material: "stone" }),
    "/api/art/masonry-1.png": new Response("wall-bytes"),
  });
  const images = await loadArtImages("http://x", "tok", ["masonry-1"], impl, tagDecode);
  expect(images).toEqual({ "tile:masonry-1": { tag: "wall-bytes" } as unknown as CanvasImageSource });
  // The sidecar first, then the picture — the SAME order artlib.lookupIn
  // resolves in, which is what keeps the client from having to know in advance
  // whether a piece is a door.
  expect(asked.map((a) => a.path)).toEqual(["/api/art/masonry-1.json", "/api/art/masonry-1.png"]);
  for (const a of asked) expect(a.auth).toBe("Bearer tok");
});

test("a DOOR resolves to the two pictures its sidecar names, closed unmarked and open suffixed", async () => {
  // A door is the one piece whose files are not its own name (artlib.Piece:
  // "File is EMPTY for a door"), so its filenames can only come from the
  // sidecar — which is why this route serves .json at all. Closed is
  // scene-plan.ts's unmarked default, so only open needs a suffix.
  const { asked, impl } = fakeFetch({
    "/api/art/cellar-door.json": sidecar({
      format_version: 1, kind: "door", material: "wood",
      open: "cellar-door-open.png", closed: "cellar-door-closed.png",
    }),
    "/api/art/cellar-door-open.png": new Response("open-bytes"),
    "/api/art/cellar-door-closed.png": new Response("closed-bytes"),
  });
  const images = await loadArtImages("http://x", "tok", ["cellar-door"], impl, tagDecode);
  expect(images).toEqual({
    "tile:cellar-door": { tag: "closed-bytes" } as unknown as CanvasImageSource,
    "tile:cellar-door/open": { tag: "open-bytes" } as unknown as CanvasImageSource,
  });
  // And NOT <id>.png, which does not exist for a door — asking for it would be
  // one guaranteed 404 per door on every scene.
  expect(asked.map((a) => a.path)).not.toContain("/api/art/cellar-door.png");
});

test("object art with NO sidecar resolves from its picture alone", async () => {
  // Design spec §3.4's improvisation case: "a bare pillar-stone.png with no
  // sidecar is complete and usable". The sidecar 404s, and that is an ordinary
  // answer rather than a failure — the client must not treat it as one.
  const { impl } = fakeFetch({ "/api/art/pillar-stone.png": new Response("pillar-bytes") });
  const images = await loadArtImages("http://x", "tok", ["pillar-stone"], impl, tagDecode);
  expect(images).toEqual({
    "tile:pillar-stone": { tag: "pillar-bytes" } as unknown as CanvasImageSource,
  });
});

test("a sidecar declaring only one of a door's two pictures falls back to the plain picture", async () => {
  // The server degrades this piece and warns (mapdef's artCannotBeUsed), so a
  // scene should not normally carry the name at all — but a client that
  // demanded both fields would key nothing and paint a checkerboard on the one
  // path where it does. Half a door is not a door; it is a piece with a
  // picture.
  const { impl } = fakeFetch({
    "/api/art/half-door.json": sidecar({ format_version: 1, kind: "door", closed: "half-door-closed.png" }),
    "/api/art/half-door.png": new Response("plain-bytes"),
  });
  const images = await loadArtImages("http://x", "tok", ["half-door"], impl, tagDecode);
  expect(images).toEqual({ "tile:half-door": { tag: "plain-bytes" } as unknown as CanvasImageSource });
});

test("one piece that does not resolve drops only itself, and the promise never rejects", async () => {
  // The same posture the whole art path takes (spec §4): a campaign that draws
  // one square plain beats a board that does not draw. Three pieces, one of
  // which has no picture anywhere.
  const { impl } = fakeFetch({
    "/api/art/masonry-1.png": new Response("wall-bytes"),
    "/api/art/earth-1.png": new Response("floor-bytes"),
  });
  const images = await loadArtImages(
    "http://x", "tok", ["masonry-1", "gone", "earth-1"], impl, tagDecode,
  );
  expect(Object.keys(images).sort()).toEqual(["tile:earth-1", "tile:masonry-1"]);
});

test("a decode that throws drops only its own key", async () => {
  const { impl } = fakeFetch({
    "/api/art/masonry-1.png": new Response("wall-bytes"),
    "/api/art/broken.png": new Response("not-an-image"),
  });
  const decode = async (blob: Blob) => {
    const bytes = await blob.text();
    if (bytes === "not-an-image") throw new Error("decode failed");
    return { tag: bytes } as unknown as CanvasImageSource;
  };
  const images = await loadArtImages("http://x", "tok", ["masonry-1", "broken"], impl, decode);
  expect(Object.keys(images)).toEqual(["tile:masonry-1"]);
});

test("a fetch that throws outright does not reject the whole load", async () => {
  // Not the same as a 404: no network at all, or a connection cut mid-scene.
  const impl = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/art/masonry-1.png") return new Response("wall-bytes");
    if (path === "/api/art/masonry-1.json") return new Response("", { status: 404 });
    throw new Error("network is down");
  }) as typeof fetch;
  const images = await loadArtImages("http://x", "tok", ["masonry-1", "gone"], impl, tagDecode);
  expect(Object.keys(images)).toEqual(["tile:masonry-1"]);
});

test("a 404 whose body happens to be JSON is still NO sidecar", async () => {
  // The status decides, never the body. Our own server answers a missing file
  // with text/plain (metadata.go's http.Error), but a proxy or a future error
  // envelope may answer 404 with a JSON body — and parsing that as a sidecar
  // would hand a piece two pictures it does not have and ask for both.
  //
  // The fixtures elsewhere in this file 404 with an EMPTY body, where .json()
  // throws and the catch reaches the same answer by accident. That accident is
  // what let the `if (!resp.ok)` mutant survive the whole suite (TS mutation
  // gate, 2026-09-05), so this is the fixture that makes the check load-bearing.
  const { asked, impl } = fakeFetch({
    "/api/art/masonry-1.json": () =>
      new Response(
        JSON.stringify({ format_version: 1, kind: "door", open: "nope-open.png", closed: "nope-closed.png" }),
        { status: 404 },
      ),
    "/api/art/masonry-1.png": new Response("wall-bytes"),
  });
  const images = await loadArtImages("http://x", "tok", ["masonry-1"], impl, tagDecode);
  expect(images).toEqual({ "tile:masonry-1": { tag: "wall-bytes" } as unknown as CanvasImageSource });
  expect(asked.map((a) => a.path)).not.toContain("/api/art/nope-open.png");
});

test("a sidecar that is not JSON is treated as no sidecar, not as a failed piece", async () => {
  // GET /api/art/{file} serves a sidecar as octet-stream with an attachment
  // disposition (metadata.go's closed allowlist), which fetch().json() does not
  // care about — but a truncated file on disk is a real state (spec §4
  // degrades it), and the picture beside it may still be perfectly good.
  const { impl } = fakeFetch({
    "/api/art/masonry-1.json": new Response("{ not json", { status: 200 }),
    "/api/art/masonry-1.png": new Response("wall-bytes"),
  });
  const images = await loadArtImages("http://x", "tok", ["masonry-1"], impl, tagDecode);
  expect(images).toEqual({ "tile:masonry-1": { tag: "wall-bytes" } as unknown as CanvasImageSource });
});

test("the token rides as a Bearer HEADER on every art request, never as a query parameter", async () => {
  // metadata.ts's own header comment: a token in a query string leaks into
  // access logs, Referer headers and browser history. GET /api/art/{file} is
  // authenticated like every other /api route (metadata.go's package doc).
  const { asked, impl } = fakeFetch({
    "/api/art/cellar-door.json": sidecar({
      format_version: 1, kind: "door", open: "cellar-door-open.png", closed: "cellar-door-closed.png",
    }),
    "/api/art/cellar-door-open.png": new Response("open-bytes"),
    "/api/art/cellar-door-closed.png": new Response("closed-bytes"),
  });
  await loadArtImages("http://x", "tok", ["cellar-door"], impl, tagDecode);
  expect(asked.length).toBe(3);
  for (const a of asked) {
    expect(a.auth).toBe("Bearer tok");
    expect(a.path).not.toContain("token");
  }
});

// --- the pack path is gone (art-is-a-flat-library Task 5/6/7) ---------------
//
// ABSENCE TESTS, written the way client/test/command-surface.test.ts writes
// them for create_scene and for retraction: assert the thing is NOT there, so
// the assertion fails before the removal and keeps it removed afterwards.
//
// Task 7's own report noted that the TypeScript half of the pack deletion had
// no absence test at all — GET /api/packs/{pack}/{file} was deleted server-side
// while client/src/view/pack-assets.ts's loadPackImages, packFileURL and
// MapMeta.pack were left standing and INERT, typechecking cleanly and always
// reaching nothing. Inert code that still compiles is exactly what nothing
// shouts about.

test("no client module fetches the pack file route, because no server serves it", () => {
  // A SOURCE SCAN rather than an export check, because the thing being asserted
  // is a URL, and a URL can be rebuilt anywhere — in app.ts, in a view, in a
  // string concatenation that no export name would reveal.
  //
  // COMMENTS ARE STRIPPED FIRST, and that is the difference between this test
  // and a ban on saying the word. This repo names what it deleted, on purpose —
  // internal/gateway/metadata.go's whole ruling section is written about
  // GET /api/packs/{pack}/{file} precisely so the route that replaced it did not
  // have to rediscover it — and an assertion that made those obituaries illegal
  // would be answered by deleting the reasoning, which is the opposite of what
  // it is for. What must not exist is CODE that asks.
  const srcDir = join(import.meta.dir, "../src");
  const walk = (dir: string): string[] =>
    readdirSync(dir).flatMap((name) => {
      const p = join(dir, name);
      if (lstatSync(p).isDirectory()) return walk(p);
      return name.endsWith(".ts") ? [p] : [];
    });
  const strip = (src: string) => src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
  const offenders = walk(srcDir)
    .filter((p) => strip(readFileSync(p, "utf8")).includes("/api/packs"))
    .map((p) => p.slice(srcDir.length + 1));
  expect(offenders).toEqual([]);
});

test("the comment stripper this file relies on actually strips, and only comments", () => {
  // The test above is only as good as its stripper: one that removed nothing
  // would fail on the obituaries, and one that removed everything would pass
  // over a live fetch. Neither failure is visible from the assertion itself, so
  // it is exercised here against both shapes directly.
  const strip = (src: string) => src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
  expect(strip("// GET /api/packs/{pack}/{file} was deleted")).not.toContain("/api/packs");
  expect(strip("/**\n * GET /api/packs/{pack}/{file}\n */")).not.toContain("/api/packs");
  expect(strip('const u = `${base}/api/packs/${id}/x.png`;')).toContain("/api/packs");
});

test("pack-assets.ts exports only the STANDARD baseline pack, and none of the per-pack path", () => {
  // The standard pack is not the same thing and stays: it is FIRST-PARTY,
  // generated by this repo's own tools/genmappack, shipped inside the client
  // bundle and served unauthenticated from "/std-pack/..." — never from a pack
  // id's route. It is genuinely a pack.json on disk, so the file keeps its
  // name; what left is everything that talked to the deleted route.
  const gone = ["loadPackImages", "packFileURL", "packManifestURL", "imageRequestsForPack"];
  expect(Object.keys(packAssets).filter((k) => gone.includes(k))).toEqual([]);
  // And the standard half is still there, so this test cannot pass by the
  // module having been emptied or renamed out from under it.
  expect(typeof packAssets.loadStandardPackImages).toBe("function");
  expect(typeof packAssets.standardPackFileURL).toBe("function");
});

// --- the shipped campaign, end to end through both halves -------------------

/**
 * THE MAGENTA CHECKERBOARD IS THE DEFECT THIS SECTION EXISTS FOR. canvas.ts's
 * paint() draws drawMissingTile over any op whose `image` key is absent from
 * the ImageMap, so the whole art path fails in exactly one way at the table: a
 * key scene-plan.ts emits that art-assets.ts never produced. Every test above
 * checks one half against a hand-written table, and two halves that agree with
 * the same table can still disagree with each other.
 *
 * So this composes the REAL functions over the REAL shipped files:
 * artNamesInScene reads the ids, loadArtImages fetches them out of
 * campaigns/example/art (served here off disk, one Response per file, exactly
 * as GET /api/art/{file} serves them), planScene emits the ops, and every op's
 * key must be one loadArtImages produced.
 *
 * IT IS WHY NO BROWSER SPEC WAS ADDED FOR THE OBJECT PATH. Neither shipped
 * adventure has a single object, so client/e2e — which boots a campaign
 * directory with no maps/ and no art/ and loads an adventure — cannot reach one
 * without a new fixture campaign of its own. What a browser would add over this
 * is that createImageBitmap decodes these particular PNGs and that the 2D
 * context draws them; what it would NOT add is the key agreement, which is the
 * thing that was actually unproven.
 */
const shippedArtDir = join(import.meta.dir, "../../campaigns/example/art");
const shippedMapPath = join(import.meta.dir, "../../campaigns/example/maps/cellar.json");

/** A fetch double serving one real directory, the way GET /api/art/{file} does. */
function diskFetch(dir: string) {
  return (async (input: RequestInfo | URL) => {
    const name = decodeURIComponent(new URL(String(input)).pathname.replace("/api/art/", ""));
    try {
      return new Response(readFileSync(join(dir, name)));
    } catch {
      return new Response("", { status: 404 });
    }
  }) as typeof fetch;
}

/**
 * shippedNatures is the kind and material each standard tile name cellar.json
 * writes in `tiles` carries onto the wire — mapdef.StandardTile's four relevant
 * rows (internal/mapdef/standard.go, and docs/map-format.md §3's table).
 *
 * IT IS NOT A COPY OF THE VOCABULARY, only of the part this one map uses, and
 * `natureOf` throws on anything else — so the day cellar.json gains a fifth
 * nature this fixture stops rather than quietly building a square with an empty
 * Kind. THAT EMPTINESS IS THE DEFECT THIS TABLE EXISTS TO CLOSE: until
 * 2026-09-06 every square here was built `{ Kind: "", Material: "" }` under a
 * comment claiming planScene read them only for the `std:` fallback, and
 * scene-plan.ts's tileImage gates the door's open picture on
 * `tile.Kind === "door"` — so the door branch was unreachable, both halves of
 * the loop below produced the identical plan, and renaming the `/open` key in
 * art-assets.ts left this test passing.
 */
const shippedNatures: Record<string, { Kind: string; Material: string }> = {
  "stone-wall": { Kind: "wall", Material: "stone" },
  "wood-door": { Kind: "door", Material: "wood" },
  stone: { Kind: "floor", Material: "stone" },
  earth: { Kind: "floor", Material: "earth" },
};

function natureOf(name: string, square: string): { Kind: string; Material: string } {
  const std = shippedNatures[name];
  if (!std) throw new Error(`cellar.json names ${name} at ${square}; shippedNatures has no such standard tile`);
  return std;
}

/**
 * shippedCellarScene is campaigns/example/maps/cellar.json as the client would
 * hold it after a load_map.
 *
 * Kind and Material come from `tiles`, and Art from `overrides`, because that is
 * the split the wire actually carries: mapdef.Resolve fills Resolved.Kind and
 * Resolved.Material from StandardTile for EVERY square, overridden or not
 * ("Kind and Material NEVER come from art" — mapdef.Resolved's own doc), and
 * only Art comes from the override. So this is reading the map the way the
 * server does, not inventing data — internal/gateway's
 * TestTheShippedCampaignResolvesItsOwnArt asserts the same `kind=door` at 5,4
 * off the wire.
 *
 * An override that went missing is still the failure worth seeing here: this
 * test loads only campaigns/example/art, never the bundled standard pack, so
 * such a square keys `std:floor/earth`, which loadArtImages never produces and
 * `missing` therefore catches. At the table pack-assets.ts's
 * loadStandardPackImages is what answers those keys.
 */
function shippedCellarScene(openDoors: Record<string, boolean>): Scene {
  const raw = JSON.parse(readFileSync(shippedMapPath, "utf8")) as {
    grid_width: number;
    grid_height: number;
    tiles: Record<string, string>;
    overrides: Record<string, string>;
    objects: { id: string; kind: string; at: [number, number]; size: [number, number];
               rot: number; blocks_sight: boolean; blocks_move: boolean; art: string }[];
  };
  const tiles: Record<string, { Kind: string; Material: string; Art: string }> = {};
  for (const [square, nature] of Object.entries(raw.tiles)) {
    const { Kind, Material } = natureOf(nature, square);
    tiles[square] = { Kind, Material, Art: raw.overrides[square] ?? "" };
  }
  return {
    ID: "cellar",
    Name: "The Sunken Cellar",
    GridWidth: raw.grid_width,
    GridHeight: raw.grid_height,
    Tiles: tiles,
    Objects: raw.objects.map((o) => ({
      ObjectID: o.id, Kind: o.kind, X: o.at[0], Y: o.at[1],
      Width: o.size[0], Height: o.size[1], RotationDegrees: o.rot,
      BlocksSight: o.blocks_sight, BlocksMove: o.blocks_move, Art: o.art,
    })),
    OpenDoors: openDoors,
  };
}

test("every image the shipped campaign's board asks for is one its own art/ answers", async () => {
  // The door's expected key rides along with each state, because "no key is
  // missing" is one-sided: a plan that never asked for the open picture at all
  // satisfies it too, and that is exactly the hole an empty Kind opened here.
  for (const [doors, doorKey] of [
    [{}, "tile:cellar-door"],
    [{ "5,4": true }, "tile:cellar-door/open"],
  ] as const) {
    const sc = shippedCellarScene(doors);
    const names = artNamesInScene(sc);
    expect(names.length).toBeGreaterThan(0);

    const images = await loadArtImages(
      "http://x", "tok", names, diskFetch(shippedArtDir),
      async () => ({}) as unknown as CanvasImageSource,
    );

    // A viewport that holds the whole 10x9 board, so nothing is culled and the
    // op count is the board rather than whatever happened to fit.
    const cell = 44;
    const cam = fitCamera(sc.GridWidth, sc.GridHeight, cell, sc.GridWidth * cell, sc.GridHeight * cell);
    const ops = planScene(
      { ...newState(), Scenes: { cellar: sc } },
      "cellar", cam, cell, sc.GridWidth * cell, sc.GridHeight * cell,
    );
    // 90 squares plus 6 objects: if this drops, the loop below stops covering
    // the board and would pass on an empty plan.
    expect(ops.length).toBe(96);

    // The door square 5,4 is the only one naming cellar-door, so its ONE key
    // pins both directions at once: the open board asks for the open picture,
    // and it stops asking for the closed one.
    const keys = new Set(ops.map((o) => o.image));
    expect([...keys].filter((k) => k.startsWith("tile:cellar-door"))).toEqual([doorKey]);

    const missing = [...keys].filter((k) => k !== "" && !(k in images));
    expect(missing).toEqual([]);
  }
});

test("the shipped door is the piece whose OPEN picture is a different file", async () => {
  // The door is the one shape with two pictures and no <id>.png, and the only
  // one where a wrong key would paint over a square that CHANGES mid-session —
  // so "the plan asks for a key that exists" is not enough: the two states must
  // ask for different pictures, or a door that opens looks shut.
  const images = await loadArtImages(
    "http://x", "tok", ["cellar-door"], diskFetch(shippedArtDir),
    async (blob) => ({ bytes: (await blob.arrayBuffer()).byteLength }) as unknown as CanvasImageSource,
  );
  expect(Object.keys(images).sort()).toEqual(["tile:cellar-door", "tile:cellar-door/open"]);
  expect(images["tile:cellar-door"]).not.toEqual(images["tile:cellar-door/open"]!);
});
