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
