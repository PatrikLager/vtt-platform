Invalid v2 fixture (finding F2, direction b): the manifest declares an
attribute literally named "caster" — one of the two reserved actor scope
words. Format v2 exposes attribute/defense values through the '@'-scope
namespace ("@caster.x"), so a name colliding with a scope word makes such a
ref ambiguous and lets a name-kind param binding of "caster"/"target"
silently change a ref's parse shape. Rejected in loadManifest, naming
ruleset.json and the offending name.
