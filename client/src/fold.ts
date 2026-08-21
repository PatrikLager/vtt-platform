// The TypeScript fold — a mirror of internal/engine/apply.go's Apply, and of
// internal/harness/fold.go's two-pass retraction handling.
//
// "Mirror" is load-bearing: parity includes REJECTING what Go rejects. A fold
// that quietly tolerates a malformed event would diverge from the server's
// own view of history and show a player a state the log does not support.

import type { Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import {
  FoldError,
  newState,
  type Actor,
  type Resource,
  type Scene,
  type SceneObject,
  type State,
  type Tile,
} from "./state";

/**
 * fold applies envelopes in order and returns the derived state.
 *
 * Two passes, matching internal/harness/fold.go: pass 1 collects every
 * sequence covered by an eventsRetracted marker's INCLUSIVE range; pass 2
 * applies everything not in that set, skipping the markers themselves — a
 * marker changes history's shape, not live state.
 */
export function fold(envelopes: Envelope[]): State {
  const retracted = new Set<bigint>();
  for (const env of envelopes) {
    if (env.payload.case === "eventsRetracted") {
      const r = env.payload.value;
      for (let s = r.fromSequence; s <= r.toSequence; s++) retracted.add(s);
    }
  }

  const st = newState();
  for (const env of envelopes) {
    if (retracted.has(env.sequence)) continue;
    if (env.payload.case === "eventsRetracted") continue;
    apply(st, env);
  }
  return st;
}

function apply(st: State, env: Envelope): void {
  const seq = Number(env.sequence);
  const p = env.payload;

  switch (p.case) {
    case "sessionStarted": {
      if (st.Sessions.some((s) => s.EndSeq === 0)) {
        throw new FoldError(`session already open at sequence ${seq}`);
      }
      // ID comes from the ENVELOPE, not the payload.
      st.Sessions.push({ ID: env.sessionId, Name: p.value.name, StartSeq: seq, EndSeq: 0 });
      return;
    }
    case "sessionEnded": {
      const open = st.Sessions.find((s) => s.EndSeq === 0);
      if (!open) throw new FoldError(`session ended with none open at sequence ${seq}`);
      open.EndSeq = seq;
      return;
    }
    case "sceneCreated": {
      const v = p.value;
      if (st.Scenes[v.sceneId]) throw new FoldError(`duplicate scene "${v.sceneId}"`);
      // Translate the wire terrain into engine-shaped Tile/SceneObject,
      // mirroring apply.go's SceneCreated arm. tiles/objects may be empty —
      // a terrain-free scene is legal (Patrik's ruling 2026-08-13) — but
      // protobuf-es always materialises a map field as {} and a repeated
      // field as [] (never undefined; see copyActor's comment below for the
      // same guarantee on a repeated field), so no `?? {}` is needed here.
      const tiles: Record<string, Tile> = {};
      for (const [k, t] of Object.entries(v.tiles)) {
        tiles[k] = { Kind: t.kind, Material: t.material, Art: t.art };
      }
      const objects: SceneObject[] = v.objects.map((o) => ({
        ObjectID: o.objectId,
        Kind: o.kind,
        X: o.at?.x ?? 0,
        Y: o.at?.y ?? 0,
        Width: o.width,
        Height: o.height,
        RotationDegrees: o.rotationDegrees,
        BlocksSight: o.blocksSight,
        BlocksMove: o.blocksMove,
        Art: o.art,
      }));
      st.Scenes[v.sceneId] = {
        ID: v.sceneId,
        Name: v.name,
        GridWidth: v.gridWidth,
        GridHeight: v.gridHeight,
        Tiles: tiles,
        Objects: objects,
        // Doors start CLOSED, mirroring apply.go's SceneCreated arm: a door
        // whose state was never recorded is shut — the fail-closed
        // direction (a door wrongly shut is a puzzle, a door wrongly open
        // is an ambush that does not happen). Initialised as a real empty
        // object, not left absent, so doorOpened/doorClosed below never
        // need their lazy-init guard for a scene built HERE.
        OpenDoors: {},
        // Explored starts empty too, mirroring apply.go's SceneCreated arm
        // (which leaves it nil — Go's zero value for "nothing"). `{}` here
        // rather than nil for the same reason OpenDoors is `{}`: TypeScript
        // has no implicit "absent means empty" read, so sceneSeen below can
        // always write into it without its own lazy-init guard.
        Explored: {},
      };
      return;
    }
    case "doorOpened": {
      const v = p.value;
      const sc = st.Scenes[v.sceneId];
      if (!sc) throw new FoldError(`door opened in unknown scene "${v.sceneId}"`);
      if (!v.at) throw new FoldError(`door opened without position`);
      ensureOpenDoors(sc)[doorKey(v.at.x, v.at.y)] = true;
      return;
    }
    case "doorClosed": {
      const v = p.value;
      const sc = st.Scenes[v.sceneId];
      if (!sc) throw new FoldError(`door closed in unknown scene "${v.sceneId}"`);
      if (!v.at) throw new FoldError(`door closed without position`);
      // delete rather than set-false: restores the "never toggled" state
      // exactly, matching apply.go's DoorClosed arm — a door that is closed
      // either because it was just shut or because it was never touched
      // reads identically.
      delete ensureOpenDoors(sc)[doorKey(v.at.x, v.at.y)];
      return;
    }
    case "actorAdded": {
      const a = p.value.actor;
      if (!a || a.actorId === "") throw new FoldError("actor added with no actor or empty id");
      if (st.Actors[a.actorId]) throw new FoldError(`duplicate actor "${a.actorId}"`);
      st.Actors[a.actorId] = copyActor(a);
      return;
    }
    case "actorControlGranted": {
      const v = p.value;
      const a = requireControlTarget(st, v.actorId, v.participantId, "actor control granted");
      if (a.controllerIds.includes(v.participantId)) return; // idempotent
      Object.assign(a, mirrorControl([...a.controllerIds, v.participantId]));
      return;
    }
    case "actorControlRevoked": {
      const v = p.value;
      const a = requireControlTarget(st, v.actorId, v.participantId, "actor control revoked");
      Object.assign(a, mirrorControl(a.controllerIds.filter((id) => id !== v.participantId)));
      return;
    }
    case "tokenPlaced": {
      const v = p.value;
      // Error ORDER matters: Go checks duplicate, scene, actor, position.
      if (st.Tokens[v.tokenId]) throw new FoldError(`duplicate token "${v.tokenId}"`);
      if (!st.Scenes[v.sceneId]) throw new FoldError(`token placed on unknown scene "${v.sceneId}"`);
      if (!st.Actors[v.actorId]) throw new FoldError(`token placed for unknown actor "${v.actorId}"`);
      if (!v.position) throw new FoldError(`token "${v.tokenId}" placed with no position`);
      st.Tokens[v.tokenId] = {
        ID: v.tokenId,
        SceneID: v.sceneId,
        ActorID: v.actorId,
        X: v.position.x,
        Y: v.position.y,
      };
      return;
    }
    case "tokenMoved": {
      const v = p.value;
      const tok = st.Tokens[v.tokenId];
      if (!tok) throw new FoldError(`unknown token "${v.tokenId}" moved`);
      if (!v.to) throw new FoldError(`token "${v.tokenId}" moved with no destination`);
      // `from` and `sceneId` are ignored entirely, exactly as Go does.
      tok.X = v.to.x;
      tok.Y = v.to.y;
      return;
    }
    case "tokenHidden": {
      // PROJECTION-ONLY (visibility spec §4.2): a viewer is being told a
      // token left their view. Deleting an absent token is deliberately NOT
      // an error — deleting a map key that is already gone is naturally a
      // no-op, and nothing about tokenHidden's own shape (spec §5: a bare
      // token_id) requires refusing a second one. Unlike sceneSeen, whose
      // idempotency spec §5 derives structurally from carrying the whole
      // current visible set every time, tokenHidden's comes from the
      // operation itself: the projection may legitimately re-send a hide —
      // recomputed visibility that already excluded the token, at-least-once
      // delivery — and this arm mirrors apply.go's TokenHidden case so
      // whichever language folds a re-sent hide, the outcome agrees.
      //
      // Verified, not assumed: a thrown FoldError here would not merely fail
      // this one event. session.ts's Session.ingest re-folds the ENTIRE
      // accumulated log on every new event (see its own doc comment on why),
      // and the log is append-only, so a poisoned entry would recur on
      // every future fold call — freezing this viewer's derived state for
      // the rest of the session, not just skipping one redraw. That is the
      // concrete shape of the "worst failure" spec §8 warns a strict fold
      // would produce here.
      delete st.Tokens[p.value.tokenId];
      return;
    }
    case "sceneSeen": {
      const v = p.value;
      const sc = st.Scenes[v.sceneId];
      if (!sc) throw new FoldError(`scene seen for unknown scene "${v.sceneId}"`);
      // sceneCreated above always sets Tiles, Objects and Explored on any
      // Scene built through the fold, but Scene marks all three optional
      // (state.ts) to let bare test literals elsewhere keep compiling — so
      // this arm defaults defensively before writing, the same guard shape
      // ensureOpenDoors below uses for doorOpened/doorClosed.
      if (!sc.Tiles) sc.Tiles = {};
      if (!sc.Explored) sc.Explored = {};
      if (!sc.Objects) sc.Objects = [];
      // REPLACED, not merged, and built fresh before the loop rather than
      // emptied in place — the two halves of this arm pull in opposite
      // directions deliberately. Explored is memory and unions; Visible is the
      // whole current visible set (spec §5) and is therefore whatever THIS
      // message says and nothing else. A sceneSeen carrying no tiles is the
      // projection reporting a scene gone dark, and it must leave an empty
      // object here rather than the `undefined` that means no projection ever
      // arrived (state.ts's Scene.Visible on why those differ). The three
      // Visible lines mirror internal/engine/apply.go's SceneSeen arm exactly.
      sc.Visible = {};
      for (const [key, ref] of Object.entries(v.tiles)) {
        sc.Tiles[key] = { Kind: ref.kind, Material: ref.material, Art: ref.art };
        sc.Explored[key] = true;
        sc.Visible[key] = true;
      }
      for (const o of v.objects) {
        const i = sc.Objects.findIndex((e) => e.ObjectID === o.objectId);
        const got = {
          ObjectID: o.objectId, Kind: o.kind,
          X: o.at?.x ?? 0, Y: o.at?.y ?? 0,
          Width: o.width, Height: o.height,
          RotationDegrees: o.rotationDegrees,
          BlocksSight: o.blocksSight, BlocksMove: o.blocksMove, Art: o.art,
        };
        if (i >= 0) sc.Objects[i] = got; else sc.Objects.push(got);
      }
      return;
    }
    case "conditionApplied": {
      const v = p.value;
      if (!st.Actors[v.actorId]) {
        throw new FoldError(`condition applied to unknown actor "${v.actorId}"`);
      }
      const list = st.Conditions[v.actorId] ?? [];
      if (list.some((c) => c.ID === v.conditionId)) {
        throw new FoldError(`duplicate condition "${v.conditionId}" on actor "${v.actorId}"`);
      }
      // Append order is preserved and never sorted.
      list.push({ ID: v.conditionId, Source: v.source, AppliedSeq: seq });
      st.Conditions[v.actorId] = list;
      return;
    }
    case "conditionRemoved": {
      const v = p.value;
      if (!st.Actors[v.actorId]) {
        throw new FoldError(`condition removed from unknown actor "${v.actorId}"`);
      }
      const list = st.Conditions[v.actorId] ?? [];
      const idx = list.findIndex((c) => c.ID === v.conditionId);
      if (idx < 0) {
        throw new FoldError(`condition "${v.conditionId}" not present on actor "${v.actorId}"`);
      }
      list.splice(idx, 1); // first match only
      // The key is RETAINED even when the slice empties — Go keeps it, and an
      // absent key vs an empty slice is visible in the dump.
      st.Conditions[v.actorId] = list;
      return;
    }
    case "noteUpserted": {
      const v = p.value;
      checkLen("note key", v.key, 1, 128);
      checkLen("note title", v.title, 0, 256);
      checkLen("note text", v.text, 1, 8192);
      st.Notes[v.key] = { Title: v.title, Text: v.text, UpdatedSeq: seq };
      return;
    }
    case "noteDeleted": {
      const v = p.value;
      if (!st.Notes[v.key]) throw new FoldError(`note "${v.key}" deleted but not present`);
      delete st.Notes[v.key];
      return;
    }
    case "resourceChanged": {
      const v = p.value;
      const actor = st.Actors[v.actorId];
      if (!actor) throw new FoldError(`resource changed on unknown actor "${v.actorId}"`);
      const res = actor.resources?.[v.resource];
      if (!res) {
        throw new FoldError(
          `resource changed for unknown resource "${v.resource}" on actor "${v.actorId}"`,
        );
      }
      // Go computes in int64 so int32 cannot wrap before the clamp; JS numbers
      // are exact across this range.
      let computed = res.current + v.delta;
      if (computed < 0) computed = 0;
      if (res.max > 0 && computed > res.max) computed = res.max; // max <= 0 = unlimited
      if (computed !== v.newValue) {
        throw new FoldError(
          `resource "${v.resource}" on actor "${v.actorId}": event new_value ${v.newValue} ` +
            `does not match computed ${computed}`,
        );
      }
      // Max is carried over from state, never taken from the event.
      actor.resources[v.resource] = { current: computed, max: res.max };
      return;
    }
    case "narrationAdded": {
      // Validates but does not mutate: narration lives in the log, and the
      // derived state has no field for it. A malformed one still fails the
      // fold, because the server would never have accepted it.
      const v = p.value;
      checkLen("narration text", v.text, 1, 8192);
      if (v.anchorFromSeq !== 0n || v.anchorToSeq !== 0n) {
        if (v.anchorFromSeq <= 0n || v.anchorToSeq <= 0n) {
          throw new FoldError("narration anchor requires both ends set");
        }
        if (v.anchorFromSeq > v.anchorToSeq) {
          throw new FoldError("narration anchor_from_seq must not exceed anchor_to_seq");
        }
        if (v.anchorToSeq >= env.sequence) {
          throw new FoldError("narration anchor must point backwards");
        }
      }
      return;
    }
    // Recorded on the log, no effect on derived state.
    case "attackRolled":
    case "abilityUsed":
    case "adventureLoaded":
      return;
    default:
      // Unknown variants are SKIPPED, not fatal — the same forward
      // compatibility the server's own replay gives.
      return;
  }
}

/**
 * doorKey matches Go's gridKey: "x,y", column then row, comma-separated
 * (maps-as-geometry spec §4.1) — SceneCreated's Tiles map and OpenDoors are
 * both keyed this way, so this fold and apply.go's have to agree on it.
 */
function doorKey(x: number, y: number): string {
  return `${x},${y}`;
}

/**
 * ensureOpenDoors is the TS analogue of apply.go's fix-round-2 nil-map
 * guard. OpenDoors is optional on Scene ONLY to let bare Scene literals
 * elsewhere in this test suite keep compiling (see state.ts's comment on
 * Scene) — fold's own sceneCreated arm above always sets it, but a Scene
 * built some other way might not have it, and writing straight into a
 * missing property throws instead of erroring cleanly. Go's equivalent bug
 * was a nil-map panic in the fold; this is the same shape of mistake,
 * caught here before it needed its own fix round.
 */
function ensureOpenDoors(sc: Scene): Record<string, boolean> {
  if (!sc.OpenDoors) sc.OpenDoors = {};
  return sc.OpenDoors;
}

/**
 * requireControlTarget resolves the actor a control event names, rejecting an
 * unknown actor and an empty participant — the same two rejections
 * internal/engine's controlTarget makes, for the same reasons: an event naming
 * something absent leaves the log meaning nothing, and "" in the set would make
 * controllerIds non-empty while controllerId mirrors an empty string.
 */
function requireControlTarget(st: State, actorId: string, participantId: string, what: string): Actor {
  if (participantId === "") throw new FoldError(`${what} requires a participant id`);
  const a = st.Actors[actorId];
  if (!a) throw new FoldError(`${what} names unknown actor "${actorId}"`);
  return a;
}

function checkLen(what: string, s: string, min: number, max: number): void {
  const n = new TextEncoder().encode(s).length; // BYTE length, as Go measures
  if (n < min) throw new FoldError(`${what} is shorter than ${min} bytes`);
  if (n > max) throw new FoldError(`${what} exceeds ${max} bytes`);
}

function copyActor(a: {
  actorId: string;
  name: string;
  moduleId: string;
  attributes: Record<string, number>;
  resources: Record<string, { current: number; max: number }>;
  controllerId: string;
  controllerIds: string[];
}): Actor {
  const resources: Record<string, Resource> = {};
  for (const [k, v] of Object.entries(a.resources ?? {})) {
    resources[k] = { current: v.current, max: v.max };
  }
  return {
    actorId: a.actorId,
    name: a.name,
    moduleId: a.moduleId,
    attributes: { ...(a.attributes ?? {}) },
    resources,
    // Empty ids are dropped here, matching internal/engine's fold: the
    // grant/revoke guard does not cover ActorAdded, so a payload carrying
    // controllerIds:[""] would otherwise create a non-empty set whose mirror
    // is the empty string — indistinguishable from an unowned actor, and
    // unremovable, since revoke rejects an empty participant.
    //
    // NOT `?? []`, unlike the two neighbours above. Review flagged the
    // asymmetry and I added the guard; CI's mutation gate then showed it
    // SURVIVING, and the reason is that it is unreachable: every fold path
    // decodes through protobuf-es, which always materialises a repeated field
    // as [], and copyActor is not exported. Nothing can pass undefined, so
    // nothing can test it — and an adjudication for dead code is worse than
    // not writing the dead code. The asymmetry is the honest state: those
    // guards predate this change and carry their own adjudications.
    ...mirrorControl(
      a.controllerIds.length > 0
        ? a.controllerIds.filter((id) => id !== "")
        : a.controllerId !== ""
          ? [a.controllerId]
          : [],
    ),
  };
}

/**
 * mirrorControl derives controllerId from the set, matching internal/engine's
 * fold exactly: controllerIds[0] when non-empty, "" when empty.
 *
 * Both folds must agree byte-for-byte — scenarios/goldens is compared against
 * BOTH (client/test/fold-parity.test.ts and internal/harness's
 * TestFoldGoldenCorpus), which is the keystone this project rests on. A
 * divergence here is not a display bug; it is the two implementations
 * disagreeing about who controls a character.
 */
function mirrorControl(ids: string[]): { controllerId: string; controllerIds: string[] } {
  const first = ids[0];
  return { controllerId: first ?? "", controllerIds: ids };
}

/**
 * foldToDumpJSON renders the folded state exactly as cmd/vtt's writeDump
 * does: the state's fields plus headSequence, two-space indented.
 *
 * Key ORDER is reproduced deliberately rather than left to chance. Go emits
 * the top level from a map (sorted keys) but each struct in its declared
 * field order, and the two differ. Emitting in the wrong order would fail a
 * byte comparison that is otherwise the strongest check available.
 */
export function foldToDumpJSON(envelopes: Envelope[]): string {
  const st = fold(envelopes);
  let head = 0;
  for (const e of envelopes) if (Number(e.sequence) > head) head = Number(e.sequence);

  // Top level: Go marshals a map, so keys are SORTED.
  const out: Record<string, unknown> = {
    Actors: sortedMap(st.Actors, actorJSON),
    Conditions: sortedMap(st.Conditions, (cs) => cs.map((c) => ({ ...c }))),
    Notes: sortedMap(st.Notes, (n) => ({ Title: n.Title, Text: n.Text, UpdatedSeq: n.UpdatedSeq })),
    Scenes: sortedMap(st.Scenes, (s) => {
      // `?? {}` / `?? []` are the same defaulting state.ts's comment on
      // Scene calls for: these fields are optional only to let bare test
      // fixtures compile, but a real fold always sets them, so this is
      // belt-and-braces rather than a path any golden actually exercises.
      const scene: Record<string, unknown> = {
        ID: s.ID,
        Name: s.Name,
        GridWidth: s.GridWidth,
        GridHeight: s.GridHeight,
        Tiles: sortedMap(s.Tiles ?? {}, (t) => ({ Kind: t.Kind, Material: t.Material, Art: t.Art })),
        Objects: (s.Objects ?? []).map((o) => ({
          ObjectID: o.ObjectID,
          Kind: o.Kind,
          X: o.X,
          Y: o.Y,
          Width: o.Width,
          Height: o.Height,
          RotationDegrees: o.RotationDegrees,
          BlocksSight: o.BlocksSight,
          BlocksMove: o.BlocksMove,
          Art: o.Art,
        })),
        OpenDoors: sortedMap(s.OpenDoors ?? {}, (v) => v),
      };
      // Explored mirrors Go's `json:",omitempty"` tag on Scene.Explored
      // (state.go): OMITTED entirely when empty, not serialized as `{}`.
      // Every existing scenarios/goldens/*/state.json was hand-derived from
      // a stream with no SceneSeen, so Explored is empty on all of them —
      // an unconditional key here (even an empty object) fails every one of
      // those byte comparisons (client/test/fold-parity.test.ts), because Go
      // never emits the key at all in that case. This is Correction 1's
      // reasoning carried across the language boundary: the FIELD needed
      // omitempty on the Go side; the DUMP needs the equivalent conditional
      // omission here, since TS has no struct-tag mechanism to do it for us.
      const explored = s.Explored ?? {};
      if (Object.keys(explored).length > 0) {
        scene.Explored = sortedMap(explored, (v) => v);
      }
      // Visible mirrors Go's `json:",omitempty"` on Scene.Visible exactly as
      // Explored above mirrors its own, and the omission collapses the
      // undefined/`{}` distinction the FIELD carries — deliberately, because
      // Go's tag collapses nil and empty the same way. The dump is what the
      // cross-language keystone (spec §4.3) diffs, so what it holds the two
      // folds to is the populated case; the distinction itself is a live-state
      // matter and is pinned separately, in both languages.
      const visible = s.Visible ?? {};
      if (Object.keys(visible).length > 0) {
        scene.Visible = sortedMap(visible, (v) => v);
      }
      return scene;
    }),
    // A Go nil slice marshals as null, not [].
    Sessions: st.Sessions.length === 0 ? null : st.Sessions.map((s) => ({
      ID: s.ID,
      Name: s.Name,
      StartSeq: s.StartSeq,
      EndSeq: s.EndSeq,
    })),
    Tokens: sortedMap(st.Tokens, (t) => ({
      ID: t.ID,
      SceneID: t.SceneID,
      ActorID: t.ActorID,
      X: t.X,
      Y: t.Y,
    })),
    headSequence: head,
  };
  return JSON.stringify(out, null, 2) + "\n";
}

function sortedMap<T, U>(m: Record<string, T>, f: (v: T) => U): Record<string, U> {
  const out: Record<string, U> = {};
  for (const k of Object.keys(m).sort()) out[k] = f(m[k]!);
  return out;
}

/** Actor as Go's encoding/json emits it: snake_case, omitempty, field order. */
function actorJSON(a: Actor): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (a.actorId !== "") out["actor_id"] = a.actorId;
  if (a.name !== "") out["name"] = a.name;
  if (a.moduleId !== "") out["module_id"] = a.moduleId;
  if (Object.keys(a.attributes).length > 0) out["attributes"] = sortedMap(a.attributes, (v) => v);
  if (Object.keys(a.resources).length > 0) {
    out["resources"] = sortedMap(a.resources, (r) => {
      const o: Record<string, number> = {};
      if (r.current !== 0) o["current"] = r.current; // omitempty
      if (r.max !== 0) o["max"] = r.max;
      return o;
    });
  }
  if (a.controllerId !== "") out["controller_id"] = a.controllerId;
  if (a.controllerIds.length > 0) out["controller_ids"] = a.controllerIds;
  return out;
}
