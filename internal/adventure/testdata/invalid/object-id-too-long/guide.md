# object-id-too-long

`scenes/cellar.json` declares a scene object `id` of 129 `o` characters, one past `maxIDBytes` (128). Everything else is
`placement-token-id-too-long`'s shape, its token id restored to `tok-scout`,
so this is the only fault the loader can meet.
