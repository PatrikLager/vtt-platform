# SPEC-008: Requirement ids come from the process's dispenser

## Status

Accepted. Implemented by `docs/requirements.md`, the dispenser `requirement-id`
from the dev-cycle package, and `tools/check-requirements-chain.py`, which
`Taskfile.yml`'s `check:requirements-chain` step runs inside `task check`;
pinned by `tools/check_requirements_chain_test.py` and, derived from this
record alone, `tools/check_requirements_chain_qa_test.py`.

## Principles served

This project has no blueprint (`CLAUDE.md`, "Where the blueprint is"). The
principle this record would serve, that a citation resolves to exactly one
thing, is missing rather than absent; the blueprint is its own ticket.

## How it works

**An id is the tag `VTT` and a number**, `VTT-NNN` counted from one, with no
per-subject segment. The tag is declared once, on the register's `project:`
line.

**Only the dispenser writes an id.** `requirement-id "<the rule, one sentence>"`
run from the repository root appends the row and prints the id. What the
dispenser enforces — never by hand, never reused, never renumbered once cited, a
withdrawn rule keeps its row — and what the evidence cell may hold — test
paths, `**OPEN — no test yet**`, `**READING — <the review>**`, never a blank —
are the dev-cycle package's rules, stated in its `requirements` skill and in the
dispenser's own header; this record does not restate them. The dispenser is
tried on a copy of the register, never on the register itself, because a stray
row, once committed, is a number spent for good.

### How a test cites its requirement

The bare id, in a comment on the line above the check it holds, or above the
section when it holds several. No marker word: the id is unique and greppable,
and a second format would be a second thing to keep true. Only the tag `VTT`
reads as a claim about this register; a fixture that builds registers as test
data carries another tag.

### How the chain is checked

`check:requirements-chain`, a step of `task check`, reads the register's tag
and rows, the test files under the source roots, and the records under
`docs/specifications/`. Which files those are, which directories are skipped,
and which declaration shapes count as a check are the checker's own to say, in
the docstring of `tools/check-requirements-chain.py`. An id with the register's
tag anywhere in one of those files is a citation, comment or not, and only the
register's own tag reads as one.

The gate refuses a citation no row defines; a row whose id is not `VTT-NNN`,
three or more digits and never zero, once markup is stripped; two rows with
one id; a malformed row; a blank evidence cell, or a `READING` that names no
review; and an evidence entry that names no check, a file that does not exist,
a file that does not carry the row's id, or a check the file does not declare.
An evidence entry is `<repo-relative path>#<check name>`; entries are separated
by a comma that begins the next entry, so a comma inside a test's name is the
name's. An empty register passes. A run that scans nothing fails with its own
exit code: no register, a register with no `project:` line or no requirements
header, or no test file under the roots.

### Where the rules come from

Each arc's ticket in `docs/superpowers/specs/` states what done looks like.
Those statements are the candidates; the `requirements` skill's sort refuses
the ones that cannot fail and names, for each survivor, the check that holds it
or one of the skill's three outcomes for a rule no check holds: a gap worth
recording keeps its row, marked `OPEN`; a rule held by a reading is marked
`READING`; a statement nothing could observe is not a rule and gets no row. The
tests are how a rule's check is found, not where the rule comes from.

**A specification names the ids it carries**, under its `Requirements` heading,
copied from the register, per the `specification` skill. A ticket carries no
id: it states its rules in words, and the ids are allocated after its plan is
signed off.

## Consequences

The register holds only ids of the shape `VTT-NNN`.

A citation that resolves to nothing, and an evidence entry that resolves to
nothing, fail the gate rather than a reading. Where a citation sits in its file
is held by a reading; the gate reads that the file carries the id.

## Requirements

None allocated. The register has no rows; the first arc's ticket allocates the
first.
