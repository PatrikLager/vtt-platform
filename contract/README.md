# contract/ — the platform's wire constitution

`vtt.v1` protobuf schemas are the single authored source of truth for every
boundary in the system (ADR-007). Generated artifacts are COMMITTED:
`gen/go` (server), `gen/ts` (client), `gen/tools/tools.json` (MCP tool
definitions). Regenerate with `task generate:contract`; CI-equivalent gates:
`task check:drift` (regenerate + diff must be empty) and `task check:breaking`
(`buf breaking` vs main).

## Wire conventions every consumer must know

1. **int64 serializes as a JSON string.** `Envelope.sequence` is `"42"`, not
   `42`, in JSON (protojson convention). Generated consumers handle this;
   hand-rolled ones must.
2. **The event envelope has no `type` field.** The `oneof payload` inlines one
   key per variant: `{"tokenMoved": {...}}` or `{"attackRolled": {...}}`.
   Generated consumers get compiler-checked discrimination (Go type switch;
   TS `payload.case`/`payload.value`); hand-rolled consumers switch on
   presence-of-key. `contract/testdata/envelope.json` is the executable spec.
3. **`Actor.module_data` is an opaque `google.protobuf.Struct`.** The contract
   never inspects it; rule modules own its shape and validate it with their
   own (JSON Schema) content schemas. Go access walks `structpb.Value`; a
   shared helper will live in the engine (sub-project 2), not here.
4. **Optionality annotation:** proto3 `optional` on a field means "optional
   parameter" in derived MCP tools; unannotated fields are required. See
   `tools/toolgen`.

## Evolution rules

Additive changes only: new fields (new numbers), new messages, new oneof
variants. Renames, deletions, number or type changes are breaking —
`task check:breaking` enforces this against main (buf FILE rules). The event
log is forever; the gate is what keeps old campaigns readable.

## Layout note

Files live under `contract/vtt/v1/` (package `vtt.v1`) rather than ADR-007's
literal `contract/v1/` — buf's PACKAGE_DIRECTORY_MATCH lint rule requires the
package path in the directory path, and disabling lint rules in this module
was judged worse than the path refinement.
