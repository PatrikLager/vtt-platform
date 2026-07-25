# Smoke Fail

A fixture ruleset whose one ability (`big-move`, costing 5 `focus`) always
exceeds `focus`'s `default_max_expr` (1) — deliberately unresolvable
against `conformance`'s generically generated fixture actor, to prove
`Run`'s smoke-test phase actually catches an unresolvable ability.
