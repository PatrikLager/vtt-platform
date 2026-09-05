// art-assets.ts: turn the art ids a live scene names into the ImageMap
// canvas.ts's paint() draws from, over GET /api/art/{file}.
//
// THIS FILE REPLACES pack-assets.ts's PER-PACK HALF (2026-09-02-art-is-a-flat-
// library design spec §6). That half fetched a pack MANIFEST, learned every
// picture the pack declared, and fetched them all — because a pack was a
// directory with a table of contents, and GET /api/maps handed the client a
// pack id to fetch it with. Both are gone: Task 5 deleted mapdef.Map.Pack, so
// no map names a container, and Task 7 deleted the route. What is left is
// simpler and has no table of contents in it at all.
//
// THE WIRE CARRIES ART IDS, AND THAT IS THE WHOLE INPUT. TileRef.art and
// SceneObject.art are filename stems in the campaign's flat art/ directory
// (contract/vtt/v1/events.proto), written in at compile time by mapdef.Resolve
// — "art is one art id — a filename stem in the campaign's flat art/
// directory". So there is nothing to correlate and nothing to enumerate: the
// client reads the ids off the folded scene and asks for the files each one
// implies.
//
// AND IT ASKS IN THE SAME ORDER artlib.lookupIn RESOLVES IN — the sidecar
// first, then the picture. That is not an accident of implementation, it is the
// only way the client can find a DOOR's pictures: a door has an open and a
// closed picture and NO <id>.png at all (artlib.Piece: "File is EMPTY for a
// door"), and their filenames are written inside <id>.json. Guessing from the
// tile's own Kind === "door" would have worked for the ordinary case and been
// wrong for the two the server warns about instead of refusing — a door piece
// named on a floor square, and a wall piece named on a door square.
//
// NOTHING HERE IS A SECOND VALIDATOR. The server has already decided what
// resolves and empties the name when it does not: a Tile.Art survives only when
// artlib.Lookup succeeded AND the piece had a sidecar, and a SceneObject.art
// only when Lookup succeeded — a bare picture is complete object art, which is
// spec §3.4's asymmetry and the reason the sidecar fetch below must treat a 404
// as an ordinary answer rather than a failure (mapdef's Resolve and
// ResolveObjectArt). Everything below then treats a missing answer the way spec
// §4 treats one — drop that key, keep the rest, never reject.

import type { Scene } from "../state";
import type { ImageMap } from "./canvas";

/**
 * ArtSidecarJSON is <id>.json's raw on-disk shape, fetched and parsed AS
 * AUTHORED — handleArtFile serves the file's own bytes verbatim
 * (internal/gateway/metadata.go), never re-marshalled — so the field names here
 * are the snake_case ones an art author actually writes, matching
 * internal/artlib's own `sidecar` struct.
 *
 * Every field but format_version is optional, and that is the format rather
 * than laxness here: artlib's decoder requires only format_version, kind and
 * material are advisory, and open/closed exist on a door and nowhere else.
 */
export interface ArtSidecarJSON {
  format_version: number;
  kind?: string;
  material?: string;
  open?: string;
  closed?: string;
}

/**
 * artFileURL is the ONE route every art file is served from — the sidecar
 * included (metadata.go's handleArtFile, GET /api/art/{file}).
 *
 * The name is encoded, so a name that somehow contains a "/" asks for a file
 * that does not exist rather than silently reshaping the path into a directory
 * walk. The server refuses it either way — artlib.IsArtFileName is what keeps
 * art/ flat over this route — and this is the client not being the one that
 * asks.
 */
export function artFileURL(base: string, file: string): string {
  return `${base}/api/art/${encodeURIComponent(file)}`;
}

/**
 * artNamesInScene collects every distinct art id one scene names, tiles and
 * objects alike, in the order first seen.
 *
 * DISTINCT IS THE POINT. An override is per SQUARE and a wall texture is per
 * PIECE, so a room with a stone perimeter names one id ninety times — the same
 * arithmetic mapdef's own warning deduplication is built on (96 warnings over 4
 * names, measured on the shipped cellar.json). Ninety round trips for one
 * texture is a cost a table pays on every scene.
 *
 * AN EMPTY Art CONTRIBUTES NOTHING, and that covers two different ordinary
 * states with one line: a square with no override at all (which draws from
 * std:<kind>/<material>), and a piece whose art did not resolve, which spec §4
 * degrades on purpose. Asking the server for "" would be one guaranteed 404 per
 * square.
 *
 * Tiles and Objects are OPTIONAL on Scene — a terrain-free scene is legal
 * (state.ts, Patrik's ruling 2026-08-13) — so the `?? {}` and `?? []` are here
 * rather than at every call site.
 */
export function artNamesInScene(scene: Scene): string[] {
  const seen = new Set<string>();
  for (const tile of Object.values(scene.Tiles ?? {})) {
    if (tile.Art !== "") seen.add(tile.Art);
  }
  for (const obj of scene.Objects ?? []) {
    if (obj.Art !== "") seen.add(obj.Art);
  }
  return [...seen];
}

/**
 * loadArtImages fetches every named piece and resolves an ImageMap ready to
 * hand straight to paint().
 *
 * Key convention matches scene-plan.ts's tileImage/objectImage EXACTLY:
 * "tile:<id>" for a plain piece or an object, with a door's OPEN picture under
 * "tile:<id>/open" and its closed picture under the plain, unmarked
 * "tile:<id>" — closed is scene-plan.ts's unmarked default, so only open needs
 * a suffix at all.
 *
 * decode defaults to the browser's own createImageBitmap. Neither bun's test
 * runtime nor happy-dom implements it (canvas.ts's own header comment), so
 * every test injects a stand-in; app.ts, the one production call site, never
 * overrides it.
 *
 * A PIECE THAT DOES NOT RESOLVE DROPS JUST ITS OWN KEY, and the returned
 * promise never rejects — on a 404, on a decode that throws, or on a fetch that
 * throws outright. That is spec §4's posture applied at the last layer that can
 * still apply it: a board that draws one square plain beats a board that does
 * not draw. The server has already told the DM what did not resolve, once per
 * name, through the load_map warning channel.
 */
export async function loadArtImages(
  base: string,
  token: string,
  names: string[],
  fetchImpl: typeof fetch = fetch,
  decode: (blob: Blob) => Promise<CanvasImageSource> = createImageBitmap,
): Promise<ImageMap> {
  const authed = (file: string) =>
    fetchImpl(artFileURL(base, file), { headers: { Authorization: `Bearer ${token}` } });

  const images: ImageMap = {};
  // One picture, fetched and decoded under one key. Failures are swallowed
  // HERE, at the smallest unit that can fail, so nothing above needs a second
  // opinion about what counts as fatal.
  const put = async (key: string, file: string) => {
    try {
      const resp = await authed(file);
      if (!resp.ok) return;
      images[key] = await decode(await resp.blob());
    } catch {
      // See this function's own doc comment: one bad file must not take down
      // the rest, or reject the whole load.
    }
  };

  await Promise.all(
    names.map(async (id) => {
      // THE catch IS EMPTY AND THE DEFAULT IS THE DECLARATION, rather than a
      // `catch { return null }` inside readSidecar. Both spell the same
      // behaviour; only this one is checkable. Measured by the TS mutation gate
      // on 2026-09-05: with the null living in the catch, emptying that block
      // survived the whole suite, because it returns undefined instead — and
      // `undefined?.open` and `null?.open` are both undefined, so nothing
      // downstream can tell them apart. An empty block has nothing left to
      // remove. Same shape pack-assets.ts's loadStandardPackImages already uses.
      let sidecar: ArtSidecarJSON | null = null;
      try {
        sidecar = await readSidecar(authed, id);
      } catch {
        // See readSidecar's own doc comment: a sidecar this client cannot
        // reach is the same answer as one that is not there.
      }
      // A door and only a door: BOTH pictures, or this is not one. A sidecar
      // naming half a door is a piece the server degrades and warns about
      // (mapdef's artCannotBeUsed), and demanding both fields without a
      // fallback would key nothing and paint a marker over it.
      if (sidecar?.open && sidecar.closed) {
        await Promise.all([
          put(`tile:${id}`, sidecar.closed),
          put(`tile:${id}/open`, sidecar.open),
        ]);
        return;
      }
      await put(`tile:${id}`, `${id}.png`);
    }),
  );
  return images;
}

/**
 * readSidecar fetches <id>.json, and answers null when the server says there is
 * not one.
 *
 * THREE DIFFERENT FACTS END UP AT THE SAME ANSWER, on purpose, because the
 * client does the same thing about all three and the server has already told the
 * DM which it was: object art legitimately has no sidecar (spec §3.4's
 * improvisation case), a truncated one degrades (spec §4), and a network failure
 * is a network failure. Distinguishing them here would be a second diagnosis
 * competing with the one the DM already has.
 *
 * ONLY THE FIRST IS ANSWERED IN THIS FUNCTION — a 404, returned as null. The
 * other two arrive as a THROW (the parse for a truncated file, the fetch for an
 * unreachable one) and are caught by the caller, which says there why its catch
 * sits there rather than here.
 */
async function readSidecar(
  authed: (file: string) => Promise<Response>,
  id: string,
): Promise<ArtSidecarJSON | null> {
  const resp = await authed(`${id}.json`);
  // THE STATUS DECIDES, NEVER THE BODY. A 404 whose body happens to be JSON —
  // a proxy's error envelope, say — must not be read as a sidecar, or a piece
  // is handed two pictures it does not have.
  if (!resp.ok) return null;
  return (await resp.json()) as ArtSidecarJSON;
}
