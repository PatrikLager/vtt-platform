Invalid v2 fixture (finding F2, direction a): the `bad` atom's resolution
roll template is "1d20 + @{who}.vim" — a name-kind param placeholder in a
ref's SCOPE position (directly between the sigil and '.'). The bound value
would become the actor scope, changing the expression's parse shape rather
than substituting as a name; rejected at load (loadAtoms), naming the atom
file and field.
