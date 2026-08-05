# resource-default-max-undeclared-ref

`default_max_expr: "10 + @nosuch"` against `attributes: [brawn, grit]`.

Exercises `checkExprRefs` at **load.go:465**, the resource call site. Its
sibling `threshold-when-undeclared-ref` covers the other one (:470); between
them they are the function's ONLY callers.

Inverting its `if e == nil { return nil }` guard makes it skip validation for
every non-nil expression, and before these two fixtures existed the whole suite
still passed. `undeclared-attribute` and `unknown-resource-ref` do not cover
this -- ability expressions are checked on a different path entirely.
