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
  type State,
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
      st.Scenes[v.sceneId] = {
        ID: v.sceneId,
        Name: v.name,
        GridWidth: v.gridWidth,
        GridHeight: v.gridHeight,
      };
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
    // `?? []` matches the `?? {}` its two neighbours already use. protobuf-es
    // always decodes a repeated field to [], so this cannot fire from the wire
    // — it guards the hand-built fixtures that reach copyActor in tests.
    ...mirrorControl(
      (a.controllerIds ?? []).length > 0
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
    Scenes: sortedMap(st.Scenes, (s) => ({
      ID: s.ID,
      Name: s.Name,
      GridWidth: s.GridWidth,
      GridHeight: s.GridHeight,
    })),
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
