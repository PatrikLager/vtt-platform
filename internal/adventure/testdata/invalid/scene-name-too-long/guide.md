# scene-name-too-long

`scenes/cellar.json` declares a name of 257 `N` characters, one past
`maxNameBytes` (256). Everything else is `scene-id-too-long`'s shape, its
scene id restored to `cellar`, so this is the only fault the loader can meet.
