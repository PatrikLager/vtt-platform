# SPEC-020: A mutation adjudication names the code it excuses

## Status

Accepted. Implemented by `tools/check-mutation.py`, the Go gate over gremlins,
and `tools/check-ts-mutation.py`, the TypeScript gate over Stryker's report,
which `Taskfile.yml`'s `check:mutation` and `check:ts-mutation` steps run
inside `task check`, and by the entries in `tools/mutation-equivalents.txt`
and `tools/ts-mutation-equivalents.txt`; pinned by
`tools/check_mutation_test.py` and `tools/check_ts_mutation_test.py`, and by
QA's `tools/check_mutation_qa_test.py` and `tools/check_ts_mutation_qa_test.py`,
derived from the ticket, the requirement rows and the plan's decisions.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: a gate excuses only what somebody has judged.

## How it works

**An adjudication excuses one surviving mutant.** Each gate fails on a
surviving mutant that no entry claims, and on an entry that claims no
surviving or timed-out mutant (VTT-301). An entry is a header line, a `code:`
line and one reason line. A line whose first character is `#` is a comment. The
reason is required, and the placeholder the gate prints in its place is
refused.

**An entry names the code its mutant sits on, never its line.** The headers,
Go's first:

    <package>  <file>  <MUTATOR>  offset=<n>  nth=<n>  copies=<n>
    <file>  <MUTATOR>  offset=<n>  span=<n>  nth=<n>  copies=<n>  <replacement>

The TypeScript replacement runs to the end of the line.
- `code` is the source line where the mutant starts, with spaces, tabs and
  carriage returns removed from both ends.
- `offset` is the tool's 1-based column less the width of the removed
  indentation: gremlins counts bytes, Stryker UTF-16 code units.
- `copies` is how many lines of the file, trimmed the same way, equal that
  code, and `nth` which of them the entry names, counted from the top.
- In TypeScript, `span` is how many UTF-16 code units the mutant covers from
  Stryker's start to its end, each line's indentation and trailing blanks left
  out and a line break counting one; and the replacement is Stryker's, with
  runs of whitespace collapsed to one space and both ends trimmed.

A survivor is claimed by an entry with the same file (and, in Go, package),
mutator, code, offset and `nth`, and in TypeScript the same replacement and
`span` (VTT-298, VTT-299). A line that moves, or is re-indented, keeps its key.

**A survivor's key is read from the source the run measured** (VTT-303). The
Go gate reads a package's `.go` files, its subdirectories' included and named
by their path relative to the package, immediately before gremlins runs for
that package, or before a stored verdict for it is reused. The TypeScript gate
reads the report's own `source` for each file. A survivor whose line cannot be
read, whose column falls outside the code on its line, or, in TypeScript,
whose end is missing, outside the source or not after its start, fails the
gate (VTT-304).

**Matching counts per key.** Survivors are claimed first, one per entry
(VTT-300); entries left over claim timed-out mutants; entries left after that
are stale. A timed-out mutant never becomes unadjudicated.

**A static check of every entry against the tree runs before the gate matches
any survivor:** in Go before gremlins runs, in TypeScript once Stryker has
written its report and before the gate goes through its mutants. It refuses
an entry whose file cannot be read; whose code stands in the file a number of
times other than `copies` (VTT-302); whose `nth` is above `copies`; that
shares its key with another entry; whose code begins as a comment does (`//`,
`/*`, `*`); whose offset holds nothing its mutator rewrites; or whose mutator
the gate holds no rule for. In Go the offset must hold one of the mutator's
tokens. In TypeScript an operator mutator's code, from its offset, must agree
with the replacement up to an operator of the mutator's kind, and the code
after that operator must begin with the replacement's text after its own; a
StringLiteral's offset must open a string; and the mutators in
`NO_POSITION_RULE` take no column rule. A refused entry fails the gate,
matching still runs, and the entry is not also reported stale.

**An unadjudicated survivor is printed as an entry ready to paste:** the
header in column 1, the `code:` line and a placeholder reason indented four
spaces. With its reason written, the block parses and claims the survivor.

## Consequences

- A line other than a copy of the mutant's code, added or removed above it,
  changes nothing.
- An edit to the mutant's own code changes its key. The static check refuses
  the entry, whose code no longer stands as often as it records, and the
  survivor is printed unadjudicated with its new code; the entry is re-keyed
  only by writing that code line, which is where its reason must be judged
  again. Nothing checks that it was.
- A copy of the code added or removed anywhere in the file changes `copies`,
  and one above the mutant changes `nth` too; the static check refuses the
  entry until it is corrected.
- An entry names no line number, so `CLAUDE.md` rule 8 makes no exception for
  these files. Their reasons carry no coordinate either; over both files this
  prints nothing:

      grep -nE '[A-Za-z0-9_]\.[a-z]{1,5}:[0-9]+|#L[0-9]+|\bline [0-9]|:[0-9]+:[0-9]+'

  Where a reason names another entry it does so by its code, its symbol or
  its place in the file. No gate checks a reason.
- A reason whose premise lies outside the keyed line is not judged again when
  that other code changes. Nothing in the key reaches it.
- A TypeScript mutant that spans lines is keyed by its first line and its
  `span`: an edit to a later line that leaves the span's length unchanged
  leaves its key unchanged.
- The TypeScript operator rule compares text, so a mutant whose replacement
  Stryker reformats, dropping parentheses or carrying a trailing comment,
  cannot be adjudicated as it stands; and a survivor of a mutator in neither
  TypeScript table cannot be adjudicated until the mutator is added to one,
  with its reason.
- The TypeScript gate splits source on line feeds only, so a U+2028, a U+2029
  or a lone carriage return in the client would put Stryker's line numbers
  and the gate's out of step.

## Requirements

VTT-298, VTT-299, VTT-300, VTT-301, VTT-302, VTT-303, VTT-304.
