# Contract Format Spike

This spike explores a unified schema for representing VTT events and operations across multiple prototype implementations. The `fixtures/` directory contains five benchmark JSON files that all prototypes must be able to parse and produce, ensuring a shared contract format across Go, TypeScript, and other targets. Each prototype directory is throwaway evidence for [ADR-007](../docs/adr/ADR-007-contract-format.md), validating the viability of this schema before committing to a production implementation.
