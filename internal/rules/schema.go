package rules

import "embed"

// Schemas holds the ruleset format's JSON Schema documents — the
// external, documentation-grade contract for ruleset authors (spec §4:
// "All files validated at load against platform-owned JSON Schemas").
// Load itself does not run a JSON Schema validator against these (see
// load.go's package doc / the task-4 controller decision): it hand-decodes
// and hand-validates. schema_test.go cross-checks that these documents'
// claims (required fields, enum values) and the loader's actual behavior
// cannot silently drift apart.
//
//go:embed schema/*.json
var Schemas embed.FS
