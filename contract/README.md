# contract/ — the platform's wire constitution

`vtt.v1` protobuf schemas, authored under `contract/vtt/v1/`, are the single
authored source of truth for every boundary in the system. Generated output is
committed under `gen/`; regenerate with `task generate:contract`.

How the contract works — the wire conventions a consumer must know, the
ordering contract, what appends to the log, the evolution rule and the gates
that hold it — is `docs/specifications/007-the-wire-contract.md`. ADR-007 holds
the road to the decision and is frozen.
