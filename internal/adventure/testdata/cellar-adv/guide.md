# Cellar Adventure

Fixture adventure for TestBothLoadPathsEmitIdenticalSceneEvents
(internal/adventure/compile_test.go). Its single scene, scenes/cellar.json,
carries the SAME tiles/overrides/objects/placements as
internal/mapdef/testdata/valid/cellar.json, and art/ holds the same pieces
that map's own art directory does — so a standalone mapdef.Load+mapdef.Compile
and this adventure's adventure.Load must compile the identical SceneCreated
event. It shipped a tiles/pack.json until 2026-09-02-art-is-a-flat-library
Task 7, when packs left the platform and Load began refusing a bundle that
still carries one; art/ had already replaced it at Task 3.
