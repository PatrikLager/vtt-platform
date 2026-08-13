# Cellar Adventure

Fixture adventure for TestBothLoadPathsEmitIdenticalSceneEvents
(internal/adventure/compile_test.go). Its single scene, scenes/cellar.json,
carries the SAME tiles/overrides/objects/placements as
internal/mapdef/testdata/valid/cellar.json, and tiles/pack.json is the same
pack (mossy-keep) internal/mapdef/testdata/packs/mossy-keep/pack.json
declares — so a standalone mapdef.Load+LoadPack and this adventure's
adventure.Load must compile the identical SceneCreated event.
