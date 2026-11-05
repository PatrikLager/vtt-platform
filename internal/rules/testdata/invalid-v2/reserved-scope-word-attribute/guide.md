Invalid v2 fixture (finding F2, direction b): a number property's key is
"caster", an actor scope word. A number property is read after '@'
("@caster.x"), so a key that is a scope word makes such a ref ambiguous.
checkPropertyKey refuses it in loadManifest, naming ruleset.json and the
key.
