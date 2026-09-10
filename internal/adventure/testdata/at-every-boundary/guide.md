# at-every-boundary

Every value in this adventure sits EXACTLY on a limit that `load.go` checks,
and every one of them is legal.

| value | limit | check |
|---|---|---|
| `opening_narration` 8192 bytes | `maxTextBytes` | `len(...) > maxTextBytes` |
| note key 128 bytes | `maxNoteKeyBytes` | `len(n.Key) > maxNoteKeyBytes` |
| note title 256 bytes | `maxNoteTitleBytes` | `len(n.Title) > maxNoteTitleBytes` |
| note text 8192 bytes | `maxTextBytes` | `len(n.Text) > maxTextBytes` |
| scene `id` 128 bytes | `maxIDBytes` | `len(raw.ID) > maxIDBytes` |
| `grid_width` / `grid_height` = 1 | smallest legal grid | `raw.GridWidth < 1` |
| placement at (0,0) | lowest legal cell | `p.Y < 0` |
| resource `max: 0`, `current: 7` | 0 means unlimited | `rv.Max > 0 && ...` |

Each check is `>` or `<` for a reason: the limit is INCLUSIVE. Loosen any one
of them by a single character — `>` to `>=`, `<` to `<=` — and this adventure
stops loading, which is what makes it a pin rather than a sample. A fixture one
byte under every limit would pass either way and prove nothing.

The scene id is 128 `p`s rather than `pin`, and that is the whole reason this
fixture kills the `>` on `maxIDBytes`. Shorten it and the mutant would live
again, which is why `TestLoadAcceptsValuesExactlyOnEveryLimit` asserts the
length back: the fixture does the work, and the assertion is what stops anyone
undoing it quietly. Without that assertion the id could be shortened with every
test still green.

Nothing here is otherwise interesting; it exists to be exactly legal.
