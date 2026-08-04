# at-every-boundary

Every value in this adventure sits EXACTLY on a limit that `load.go` checks,
and every one of them is legal.

| value | limit | check |
|---|---|---|
| `opening_narration` 8192 bytes | `maxTextBytes` | `len(...) > maxTextBytes` |
| note key 128 bytes | `maxNoteKeyBytes` | `len(n.Key) > maxNoteKeyBytes` |
| note title 256 bytes | `maxNoteTitleBytes` | `len(n.Title) > maxNoteTitleBytes` |
| note text 8192 bytes | `maxTextBytes` | `len(n.Text) > maxTextBytes` |
| `grid_width` / `grid_height` = 1 | smallest legal grid | `raw.GridWidth < 1` |
| placement at (0,0) | lowest legal cell | `p.Y < 0` |
| resource `max: 0`, `current: 7` | 0 means unlimited | `rv.Max > 0 && ...` |

Each check is `>` or `<` for a reason: the limit is INCLUSIVE. Loosen any one
of them by a single character — `>` to `>=`, `<` to `<=` — and this adventure
stops loading, which is what makes it a pin rather than a sample. A fixture one
byte under every limit would pass either way and prove nothing.

Nothing here is otherwise interesting; it exists to be exactly legal.
