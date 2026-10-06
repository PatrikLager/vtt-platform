# adventure-id-too-long

`adventure.json` declares an id of 129 `v` characters, one past `maxIDBytes`
(128). Everything else is `scene-id-too-long`'s shape, its scene id restored
to `cellar`, so this is the only fault the loader can meet.
