Invalid v2 fixture (spec §4 key-validity clause): the `dmg` atom's outcome
contribution is conditioned on resolution key "clash" (branch "connect")
but does NOT declare "clash" among its `consumes`. Without that
declaration the provides/consumes DAG gains no edge from the resolution
atom to this outcome, so the outcome's cross-atom ordering silently
degrades to compose-list position — exactly the implicit hand-ordered-list
contract spec §2 guarantee 4 bans. Rejected at load (loadAtoms) naming the
atom file and the offending key/field.
