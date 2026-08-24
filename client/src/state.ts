// TypeScript mirror of internal/engine's State and its JSON shape.
//
// The types exist to make the fold's output byte-identical to Go's
// `vtt state dump`, so the shapes below are dictated by how Go marshals —
// which differs per type and is the main source of subtle mismatch:
//
//   Scene/Token/Session/Note/ActorCondition are plain Go structs with NO json
//   tags, so every field is emitted with its Go name and zero values are NOT
//   omitted. `EndSeq: 0` must appear.
//
//   Actor and Resource are protobuf-generated and DO carry
//   `json:"...,omitempty"` tags, so their keys are snake_case and empty
//   values vanish entirely. `{Current: 0, Max: 0}` marshals as `{}`.
//
//   Sessions is a Go slice: nil marshals as `null`, not `[]`.

import type { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";

/**
 * Tile mirrors internal/engine's Tile: one square's terrain, translated out
 * of the wire vtt.v1.TileRef at sceneCreated fold time. Kind is the closed
 * spatial set "wall"/"floor"/"door"; Material is opaque and must never be
 * branched on (CLAUDE.md rule 5).
 */
export interface Tile {
  Kind: string;
  Material: string;
  Art: string;
}

/**
 * SceneObject mirrors internal/engine's SceneObject: scenery, never an
 * actor. Kind here is an OPEN descriptive label the platform never
 * interprets — only BlocksSight/BlocksMove carry structural effect.
 */
export interface SceneObject {
  ObjectID: string;
  Kind: string;
  X: number;
  Y: number;
  Width: number;
  Height: number;
  RotationDegrees: number;
  BlocksSight: boolean;
  BlocksMove: boolean;
  Art: string;
}

export interface Scene {
  ID: string;
  Name: string;
  GridWidth: number;
  GridHeight: number;
  /**
   * Tiles, Objects and OpenDoors are OPTIONAL here — the TypeScript
   * equivalent of Go's zero-value convenience for engine.Scene, which lets
   * a struct literal omit a map/slice field entirely and read it back as
   * nil (empty). TypeScript has no such affordance: a plain object literal
   * either has the property or doesn't, and there is no implicit "absent
   * means empty" at the type level. fold.ts's sceneCreated arm always sets
   * all three explicitly (mirroring apply.go's SceneCreated arm, which
   * always initialises non-nil maps/slices even for a terrain-free scene —
   * "Tiles may be empty and that is legal", Patrik's ruling 2026-08-13);
   * marking them optional here only accommodates Scene literals built
   * directly elsewhere in this test suite (the TS analogue of Go's
   * `engine.Scene{ID: ..., Name: ..., GridWidth: ..., GridHeight: ...}`
   * fixtures), which predate this field and must keep compiling. Any reader
   * of a Scene that might be one of those has to default explicitly
   * (`?? {}` / `?? []`) rather than relying on Go's implicit nil-read.
   */
  Tiles?: Record<string, Tile>;
  Objects?: SceneObject[];
  OpenDoors?: Record<string, boolean>;
  /**
   * Explored is the squares THIS VIEWER has ever seen, keyed like Tiles.
   * Mirrors internal/engine's Scene.Explored (state.go) so the same fold
   * runs in both languages (visibility spec §6). It only ever grows: terrain
   * is remembered, creatures are not.
   *
   * EMPTY for a scene folded from the real log — nothing in a campaign's log
   * produces sceneSeen, so this is populated only when folding a
   * PROJECTION. Optional for the same reason Tiles/Objects/OpenDoors are:
   * bare Scene literals built directly in other test suites must keep
   * compiling. fold.ts's sceneCreated arm always sets it to `{}` explicitly.
   */
  Explored?: Record<string, boolean>;
  /**
   * Visible is the squares this viewer can see RIGHT NOW, keyed like Tiles,
   * and Explored's opposite number in how it moves: REPLACED wholesale by each
   * sceneSeen rather than unioned, so it shrinks as freely as it grows.
   * Mirrors internal/engine's Scene.Visible (state.go).
   *
   * IT DOES NOT COME FROM Explored'S SOURCE. Visible is folded from
   * sceneSeen's own `visible` field (events.proto field 4) — the server's
   * sight answer, computed over the GRID and owing nothing to terrain —
   * while Explored unions that message's TILE keys. So the two may differ
   * completely: a bare canvas is wholly visible and wholly unexplored, and
   * ground walked out of is explored and not visible. Do not assume they
   * track; both corners are pinned in client/test/visibility.test.ts.
   *
   * UNTIL 2026-08-22 THIS WAS BUILT FROM THE TILE KEYS, which made it mean
   * "visible AND declares terrain" — a lossy proxy for a decision the server
   * had already made, which on a scene with no tiles hid the player's own
   * token. A token is a free object and needs no ground under it.
   *
   * The pair is what the board needs and neither half gives alone —
   * `Explored − Visible` is ground you remember and cannot currently see,
   * which is the fog (visibility spec §6.1).
   *
   * UNDEFINED AND `{}` MEAN DIFFERENT THINGS, which is why sceneCreated
   * leaves this absent while it sets Explored to `{}`. Undefined is "no
   * sceneSeen has ever arrived for this scene" — the DM and the agent, whose
   * streams are the identity projection and contain none. `{}` is "a
   * projection arrived and this seat can see nothing here". Conflating them
   * would blank the DM's board.
   *
   * So every reader that decides what to DRAW must branch on the distinction
   * rather than defaulting with `?? {}` — that is the one place `?? {}` is
   * wrong on a Scene field, and view/scene-plan.ts's planFog and view/grid.ts's
   * tokensOnScene are the two that must not take it. The DUMP is the exception,
   * and it is not a violation: foldToDumpJSON reproduces Go's `omitempty`,
   * which drops nil and empty alike, so `?? {}` there is the correct mirror of
   * a distinction JSON does not carry.
   */
  Visible?: Record<string, boolean>;
}

export interface Token {
  ID: string;
  SceneID: string;
  ActorID: string;
  X: number;
  Y: number;
}

export interface Session {
  ID: string;
  Name: string;
  StartSeq: number;
  EndSeq: number; // 0 = open
}

export interface ActorCondition {
  ID: string;
  Source: string;
  AppliedSeq: number;
}

export interface Note {
  Title: string;
  Text: string;
  UpdatedSeq: number;
}

export interface Resource {
  current: number;
  max: number;
}

/** Mirrors vttv1.Actor as Go's encoding/json emits it (snake_case, omitempty). */
export interface Actor {
  actorId: string;
  name: string;
  moduleId: string;
  attributes: Record<string, number>;
  resources: Record<string, Resource>;
  /**
   * Mirror of controllerIds[0], or "" when nobody controls this actor.
   *
   * Kept because ActorAdded events carrying it exist in real campaign logs and
   * readers predating the set still consult it. NOT blanked when an actor has
   * several controllers: an empty controllerId already means DM/agent-only, so
   * a shared actor would be indistinguishable from an unowned one.
   */
  controllerId: string;
  /** Every participant who may act as this actor. Authoritative. */
  controllerIds: string[];
  /**
   * What this actor IS — the only thing the server's "always known" visibility
   * exception keys on (visibility spec §5.1).
   *
   * REQUIRED, not optional, like every other field here. Go's Actor is
   * protobuf-generated and always has the field, so an optional one would put
   * `undefined` in a mirror position where Go has 0 — a third state the dump
   * comparison cannot express and the fold would have to guess at. UNSPECIFIED
   * (0) is the "the log said nothing" value, and it is a real state rather than
   * an error. What it MEANS belongs to the reader and never to the fold: an
   * absent kind is not a party member, always, with nothing inferred from who
   * controls the actor (§5.1, whose migration rule saying otherwise was deleted
   * 2026-08-24).
   *
   * The client does not enforce visibility — the server projects it, and this
   * client only ever sees what it was sent. Kind is carried so the fold stays a
   * byte-exact mirror of `vtt state dump`, and so a future view can say "your
   * party" without asking who holds whom.
   */
  kind: ActorKind;
}

export interface State {
  Scenes: Record<string, Scene>;
  Actors: Record<string, Actor>;
  Tokens: Record<string, Token>;
  Sessions: Session[];
  Conditions: Record<string, ActorCondition[]>;
  Notes: Record<string, Note>;
}

export function newState(): State {
  return { Scenes: {}, Actors: {}, Tokens: {}, Sessions: [], Conditions: {}, Notes: {} };
}

/** Thrown for any event the Go fold would reject. Parity includes failing. */
export class FoldError extends Error {}
