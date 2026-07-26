Invalid v2 fixture (finding F2, direction b): a condition declares the id
"target" — one of the two reserved actor scope words. Reserving caster/
target against condition ids too (not just manifest names) keeps a
condition-kind param binding from occupying a ref scope position. Rejected
in loadConditions, naming the condition file and the offending id.
