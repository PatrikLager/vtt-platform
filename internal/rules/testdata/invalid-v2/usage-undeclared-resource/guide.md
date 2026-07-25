Invalid v2 fixture (pre-authorized item 1): a composition's
usage.limited.resource ("no_such_pool") names a resource the manifest does
not declare. The compile path must cross-validate it against the ruleset's
declared resources, the same way v1's loader did — rejected at load naming
the ability file and the offending resource.
