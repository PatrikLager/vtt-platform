# scene-id-too-long

`scenes/cellar.json` declares an id of 200 `s` characters, past `maxIDBytes`
(128). The manifest, the note and all three actors are byte-identical to
`testdata/valid`.

It ships ONE scene where `testdata/valid` and nearly every invalid sibling ship
the same five (cellar, gate, hall, loft, yard), and that departure is
deliberate: those five exist so `Compile`'s file-name scene order is
distinguishable from a map-iteration bug, which is not what this fixture is for.
`cellar` sorts first under `jsonFilesIn`, so any others would never be reached —
the refusal fires before them.

"Nearly every" rather than a number on purpose: `duplicate-scene-id` needs a
sixth scene to have something to duplicate, and `empty-adventure` has no
`scenes/` directory at all.

200 rather than 129 on purpose, and the distance is the point: this fixture
proves the refusal happens, and it CANNOT prove where the edge is, because 200
is refused under `>` and `>=` alike. The edge is pinned from the other side, by
`at-every-boundary`, whose scene id is exactly 128 and must load. Two fixtures,
one comparison — neither is redundant.
