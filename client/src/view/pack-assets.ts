// pack-assets.ts: turn a pack manifest (design spec §4.2) into the ImageMap
// canvas.ts's paint() draws from — the seam nothing in this arc's first nine
// tasks built (Task 10's own brief: "paint receives an empty ImageMap and
// the canvas draws blank by construction").
//
// The wire never carries a pack reference on a live Scene. SceneCreated
// resolves art names into Tile.Art at COMPILE time and stops there (spec §5:
// "packs are needed to author and to draw, never to fold") — a client
// replaying the log sees "masonry-1" and has no way to ask the log which
// pack that name means anything in. GET /api/maps is therefore the only
// route this client can learn a pack id from at all (metadata.go's
// handleMaps, which already enriches every configured map with its own
// declared pack). app.ts loads EVERY configured map's pack eagerly rather
// than trying to correlate a live scene back to the specific map it came
// from — the wire gives no clean way to do that correlation (two different
// live scenes could share an id with two different maps, and nothing here
// needs to resolve that ambiguity to render the ordinary case of one table
// playing one map at a time).
//
// KNOWN LIMITATION, carried rather than solved here: canvas.ts's ImageMap
// key convention ("tile:<name>") carries no pack id, so two configured packs
// that happen to declare the same tile name would collide in the shared
// ImageMap. That is not a new problem this file introduces — scene-plan.ts's
// tileImage/objectImage (Task 8) already emit an unqualified "tile:<name>"
// key with no pack prefix — so fixing it is a key-format change belonging to
// whichever task first needs more than one pack live in the same table.

import type { ImageMap } from "./canvas";

/**
 * PackManifestJSON is pack.json's raw on-disk shape (design spec §4.2),
 * fetched and parsed AS AUTHORED — handlePackFile serves the file's own
 * bytes verbatim (internal/gateway/metadata.go), never re-marshalled — so
 * the field names here are the snake_case ones a pack author actually
 * writes, matching internal/mapdef/load.go's packJSON/packTileJSON exactly.
 */
export interface PackManifestJSON {
  id: string;
  name: string;
  cell_px: number;
  tiles: PackManifestTileJSON[];
  objects: PackManifestTileJSON[];
}

/** One tiles[] or objects[] entry — the two arrays share this shape
 *  (mapdef.PackTile's own doc comment: "a pack.json entry looks identical
 *  whichever list it sits in"). Every field but `name` is optional: kind and
 *  material are advisory-only (spec §3.2) and a tile has EITHER `file` OR
 *  the `file_open`/`file_closed` pair, never a use for all three. */
export interface PackManifestTileJSON {
  name: string;
  kind?: string;
  material?: string;
  file?: string;
  file_open?: string;
  file_closed?: string;
  desc?: string;
}

/**
 * packFileURL is the ONE route every pack asset — the manifest included —
 * is served from (metadata.go's handlePackFile, GET /api/packs/{pack}/{file}).
 * Both path segments are encoded: an unencoded pack id or filename
 * containing "/" would silently reshape the path instead of 404ing on a
 * name that does not exist.
 */
export function packFileURL(base: string, packId: string, file: string): string {
  return `${base}/api/packs/${encodeURIComponent(packId)}/${encodeURIComponent(file)}`;
}

/** pack.json is just another file at this pack's own route (handlePackFile's
 *  own doc comment: "structured data a client fetches programmatically" —
 *  this function is that fetch's URL). */
export function packManifestURL(base: string, packId: string): string {
  return packFileURL(base, packId, "pack.json");
}

/** One file to fetch, and the ImageMap key canvas.ts's paint() should find
 *  it under once decoded. */
export interface ImageRequest {
  key: string;
  url: string;
}

/**
 * imageRequestsForPack decides WHAT to fetch and WHERE each answer belongs
 * in the ImageMap — pure, so it is fully unit-tested with no network and no
 * image decoder, mirroring scene-plan.ts's own split between deciding
 * (pure, tested) and drawing/fetching (thin, read to verify).
 *
 * Key convention matches scene-plan.ts's tileImage/objectImage EXACTLY:
 * "tile:<name>" for a plain tile or an object (objects have no standard
 * fallback and share this file's own "tile:" namespace with tile overrides
 * — see this file's header comment), with a door's OPEN picture under
 * "tile:<name>/open" and its closed picture under the plain, unmarked
 * "tile:<name>" — closed is scene-plan.ts's unmarked default, so only open
 * needs a suffix at all.
 */
export function imageRequestsForPack(
  base: string,
  packId: string,
  manifest: PackManifestJSON,
): ImageRequest[] {
  const url = (file: string) => packFileURL(base, packId, file);
  const out: ImageRequest[] = [];
  for (const t of manifest.tiles) {
    if (t.file) out.push({ key: `tile:${t.name}`, url: url(t.file) });
    if (t.file_closed) out.push({ key: `tile:${t.name}`, url: url(t.file_closed) });
    if (t.file_open) out.push({ key: `tile:${t.name}/open`, url: url(t.file_open) });
  }
  for (const o of manifest.objects) {
    if (o.file) out.push({ key: `tile:${o.name}`, url: url(o.file) });
  }
  return out;
}

/**
 * loadPackImages fetches one pack's manifest and every image it names, and
 * resolves an ImageMap ready to hand straight to paint().
 *
 * decode defaults to the browser's own createImageBitmap. Neither bun's test
 * runtime nor happy-dom implements it (canvas.ts's own header comment: happy-
 * dom has NO canvas implementation, and the same gap extends to image
 * decoding) — every test in pack-assets.test.ts injects a stand-in; app.ts,
 * the one production call site, never overrides it.
 *
 * A single file that 404s, or a decode that throws, drops JUST that key
 * rather than the whole pack, and the returned promise never rejects on
 * either: boot validation (maps-as-geometry Task 7's loadMapsDir) is what
 * should have caught a genuinely broken pack before a table ever sees it, so
 * a client hitting a bad file anyway should render everything else it can
 * rather than a blank board over one missing texture.
 */
// --- the standard-vocabulary baseline pack (review finding C2, 2026-08-16) -
//
// A picture for every one of internal/mapdef/standard.go's ELEVEN standard
// natures, so a square with no art override draws SOMETHING rather than
// nothing — before this, zero code anywhere produced a std:<kind>/<material>
// image, and both shipped adventures carry no overrides at all (goblin-
// ambush's earth floor, cellar-rats' wood floor), so every square of them
// rendered blank.
//
// Served from a DIFFERENT place than an ordinary pack, deliberately:
// tools/genmappack/std_pack.go's own header comment has the full reasoning,
// summarised here for this file's own readers. GET /api/packs/{pack}/{file}
// (packFileURL/loadPackImages above) exists for OPERATOR-INSTALLED content —
// third-party trust, Bearer-authenticated, served through a closed
// content-type allowlist. The standard pack is first-party: generated by
// this repository's own tool, at this repository's own commit, shipped in
// the SAME build artifact as index.html/index.js. It is served from
// "/std-pack/..." — a plain sibling of the client bundle's own static
// files (client/public/std-pack, copied verbatim by Vite into
// cmd/vtt/webdist/std-pack) — through the gateway's existing unauthenticated
// static route (internal/gateway/server.go's WithStatic: "the browser must
// load the app before it has anywhere to type a token"). No pack id, no
// token: every function below takes neither.
export function standardPackFileURL(base: string, file: string): string {
  return `${base}/std-pack/${encodeURIComponent(file)}`;
}

/**
 * imageRequestsForStandardPack mirrors imageRequestsForPack's shape exactly,
 * with one deliberate difference: the ImageMap key comes from each tile's
 * kind/material, NEVER its name. scene-plan.ts's tileImage resolves an
 * unoverridden square to `std:${tile.Kind}/${tile.Material}` — it has no
 * notion of this pack's own tile NAMES (those are genmappack's own
 * bookkeeping, kept in pack.json only for a human reading the file) — so
 * keying by name here would produce keys tileImage can never ask for.
 *
 * A tile naming no kind or material is skipped rather than guessed at: a
 * genuine std pack.json (tools/genmappack/std_pack.go) always sets both, so
 * this is defensive against a malformed file, not an expected path.
 */
export function imageRequestsForStandardPack(
  base: string,
  manifest: PackManifestJSON,
): ImageRequest[] {
  const out: ImageRequest[] = [];
  for (const t of manifest.tiles) {
    if (!t.kind || !t.material) continue;
    const key = `std:${t.kind}/${t.material}`;
    if (t.file) out.push({ key, url: standardPackFileURL(base, t.file) });
    if (t.file_closed) out.push({ key, url: standardPackFileURL(base, t.file_closed) });
    if (t.file_open) out.push({ key: `${key}/open`, url: standardPackFileURL(base, t.file_open) });
  }
  return out;
}

/**
 * loadStandardPackImages is loadPackImages' unauthenticated twin: same
 * "fetch the manifest, fetch every image, one bad file drops only itself,
 * the returned promise never rejects" shape (see loadPackImages' own doc
 * comment for the full reasoning, which applies here unchanged), but
 * against "/std-pack/..." rather than a pack id's own route, and with no
 * token anywhere — this pack carries nothing that needs gating.
 */
export async function loadStandardPackImages(
  base: string,
  fetchImpl: typeof fetch = fetch,
  decode: (blob: Blob) => Promise<CanvasImageSource> = createImageBitmap,
): Promise<ImageMap> {
  const manifestResp = await fetchImpl(standardPackFileURL(base, "pack.json"));
  if (!manifestResp.ok) return {};
  const manifest = (await manifestResp.json()) as PackManifestJSON;

  const images: ImageMap = {};
  await Promise.all(
    imageRequestsForStandardPack(base, manifest).map(async (req) => {
      try {
        const resp = await fetchImpl(req.url);
        if (!resp.ok) return;
        images[req.key] = await decode(await resp.blob());
      } catch {
        // See loadPackImages' own doc comment: one bad file must not take
        // down the rest, or reject the whole load.
      }
    }),
  );
  return images;
}

export async function loadPackImages(
  base: string,
  token: string,
  packId: string,
  fetchImpl: typeof fetch = fetch,
  decode: (blob: Blob) => Promise<CanvasImageSource> = createImageBitmap,
): Promise<ImageMap> {
  const authed = (url: string) => fetchImpl(url, { headers: { Authorization: `Bearer ${token}` } });

  const manifestResp = await authed(packManifestURL(base, packId));
  if (!manifestResp.ok) return {};
  const manifest = (await manifestResp.json()) as PackManifestJSON;

  const images: ImageMap = {};
  await Promise.all(
    imageRequestsForPack(base, packId, manifest).map(async (req) => {
      try {
        const resp = await authed(req.url);
        if (!resp.ok) return;
        images[req.key] = await decode(await resp.blob());
      } catch {
        // See this function's own doc comment: one bad file must not take
        // down the rest of the pack, or reject the whole load.
      }
    }),
  );
  return images;
}
