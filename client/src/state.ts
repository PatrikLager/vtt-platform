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

export interface Scene {
  ID: string;
  Name: string;
  GridWidth: number;
  GridHeight: number;
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
  controllerId: string;
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
