Invalid v2 fixture (spec §4 key-validity clause): a resolution
contribution's `key` ("clash") is not among the owning atom's `provides`
(empty) — the graph key that routes outcomes and the provides list that
drives execution ordering would be disconnected namespaces. Rejected at
load (loadAtoms) naming the atom file and the offending key/field.
