# second-placeholder-unknown

`1d20 + {edge} + {edge} + {nope}` — the bad placeholder is LAST, behind two
good ones.

Sibling of `atom-unknown-param-placeholder`, which puts the bad placeholder
first. That one cannot distinguish a scan of ALL matches from one that stops
early, because the bad one IS the first. `checkPlaceholders` calls
`FindAllStringSubmatch(raw, -1)`, and `-1` is what means "all"; under any
small positive limit an undeclared name past that point loads clean.

Three placeholders rather than two so a limit of 1 AND a limit of 2 both miss
it — a two-placeholder version killed the `-1 -> 1` mutant but not `-1 -> 2`.
