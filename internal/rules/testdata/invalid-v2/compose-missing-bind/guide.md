Invalid v2 fixture: a compose entry omits "bind" entirely for a
param-less atom — must still be rejected (the schema requires "bind";
omission is indistinguishable from {} in Go's map decode, so a
param-less atom's missing bind previously went undetected).
