# SPEC-010: A comment in code is a warning or a pointer

## Status

Accepted. Implemented by `tools/check-comments.py`, which `Taskfile.yml`'s
`check:comments` step runs inside `task check`, and by
`tools/comment-ceilings.txt`; pinned by `tools/check_comments_test.py` and,
derived from this record alone, `tools/check_comments_qa_test.py`.

## Principles served

This project has no blueprint (`CLAUDE.md`, "Where the blueprint is"). The
principle this record would serve, that a fact has one home, is missing from
the record rather than absent from the system; the blueprint is owed a ticket
of its own.

## How it works

**A comment in code is one of three things.** Under `internal/`, `cmd/` and
`client/src`, a comment is an imperative warning to whoever edits next; or a
pointer to a record, which is a specification by its number, a requirement by
its id, a test or a symbol by its name, or a report by its path under `docs/`;
or the one-line doc sentence that `go doc` prints for an exported symbol.
History, measurements, arguments and descriptions of what the code does are not
comments: the implementation report holds what happened, and the specification
holds how the system works now.

**This narrows `CLAUDE.md` rule 8 for code comments.** Rule 8 says how to cite
and allows a dated decision and a plan's task among the targets. A code comment
points at none of those: a date outside a `docs/` path is refused, a plan is
not among the records a comment may point at, and neither is a commit hash. An
`[anchor:kebab-name]` marker placed in code is a target for prose elsewhere,
not a comment of any kind, and is allowed as a directive is. The narrowing is
this record's decision, and rule 10 of `CLAUDE.md` says so.

**Added lines are held by `check:comments`**, a step of `task check` that runs
after `check:new-prose` and reads the comment lines a change adds against
`main`, the scope `check:new-prose` uses. It refuses an added line that carries
a banned term, and a comment block longer than the bound when the change added
at least one line to that block, the Go package doc excepted; a citation line,
defined below, is in no block and adds no length to one. The banned list,
the bound, the default ceiling and the band below are the checker's own to
carry, in the docstring of `tools/check-comments.py`, as SPEC-008 leaves its
file sets to its checker.

**Files are held by a ceiling.** `tools/comment-ceilings.txt` holds one row per
file in scope, the file's comment share as a ceiling. A change that adds a
comment line to a file leaves that file at or under its ceiling; a file above
its ceiling to which the change added no comment line is reported and not
refused, since a share also rises when code leaves. A ceiling is lowered with
`tools/check-comments.py --write-ledger`, which never raises a row; the gate
refuses a row raised above the base's, and a row the base does not hold above
the default ceiling unless it is a rename of a row at or above it; a hand edit
that lowers a row is not refused. When the base carries no ledger, the raise
check is skipped and the run says so. A file that has fallen more than the band
under its ceiling is refused until the ledger is lowered, so a share that fell
is recorded by the change that lowered it. A file with no row is held to the
default ceiling; a row whose file no longer exists is refused. A `//` line
carrying only requirement ids of the register's tag, spaces between, is a
citation line: it counts in no comment share, is not an added comment line and
has no place in a block, so adding one moves nothing; a line inside a
`/* ... */` block is a block line whatever it carries, and whether an id
resolves to a row is the chain gate's. The checker's docstring says what such
a line is and what a run with no register does.

**A run proves it ran.** A clean run ends with a completion line naming the
files, added comment lines and ledger rows it read. A run that scans no file in
scope, or cannot establish its base, fails rather than passes.

**What is outside.** Doc comments in the `.proto` files under `contract/` are
SPEC-007's and are warnings already. `tools/*.py` and `Taskfile.yml` are named
undecided by the ticket and are not read. A test's `VTT-NNN` citation is a
pointer (SPEC-008). The directive part of a `//go:` or `//nolint` line is
machine input and is not read; a `// reason` trailing such a line is prose and
is. An `[anchor:...]` marker is not read for its spelling; the rest of its line
is.

## Consequences

A code change that adds a comment line carrying a banned term is refused. A
measurement or an account of the past with no banned term in it is refused by
the reading that holds VTT-051. Either way the sentence belongs in the report.

Correcting a sentence in a comment block longer than the bound means cutting
the block to the bound in the same change.

A change that only removes code and leaves a file above its ceiling is not
refused; the next change that adds a comment line to that file is.

The comment already in the tree is not cleaned by the gate. The ledger records
the shares as they are when it is written, and lowering them is separate work,
each change lowering the ledger for the files it cleans.

There is no hatch. A warning that cannot be written without a banned term is a
reviewed decision under `CLAUDE.md` rule 2, not an annotation.

## Requirements

VTT-050, VTT-051, VTT-052, VTT-053, VTT-054, VTT-055, VTT-056, VTT-057,
VTT-058, VTT-059.
