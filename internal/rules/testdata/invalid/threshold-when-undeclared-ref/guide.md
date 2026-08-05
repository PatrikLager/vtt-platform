# threshold-when-undeclared-ref

A threshold whose `when: "#nosuch_pool"` names a resource that is not declared.

Exercises `checkExprRefs` at **load.go:470**, the threshold call site. Its
sibling `resource-default-max-undeclared-ref` covers the other one (:465);
between them they are the function's ONLY callers.

The case asserts `thresholds[0].when` rather than a bare `when`, which is a
substring of this directory's own name -- and every Load error carries the
fixture path. That is the trap that left `dependency-cycle` asserting nothing
for as long as it did.
