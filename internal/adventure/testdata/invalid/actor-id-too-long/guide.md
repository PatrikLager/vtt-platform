# actor-id-too-long

`actors/vim-fighter.json` declares an `actor_id` of 129 `a` characters, one
past `maxIDBytes` (128). Everything else is `scene-id-too-long`'s shape, its
scene id restored to `cellar`, so this is the only fault the loader can meet.
