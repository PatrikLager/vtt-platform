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
