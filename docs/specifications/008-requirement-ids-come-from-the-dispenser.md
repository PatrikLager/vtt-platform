# SPEC-008: Requirement ids come from the process's dispenser

## Status

Accepted, and partly implemented. The register `docs/requirements.md` carries
the tag `VTT` and the `| Id | Requirement | Verified by |` header, and the
dispenser `requirement-id` from the dev-cycle package allocates into it; that
half is in place, exercised by the package's own tests rather than by anything
in this repository. The other half, a gate that checks the chain between the
register, the tests and the specifications, is not implemented: nothing in
`task check` reads a citation. No ticket carries it yet; the first arc's ticket
under the process, written next, is where it lands, because that arc's rows are
the first the gate has to hold.

## Principles served

This project has no blueprint (`CLAUDE.md`, "Where the blueprint is"). The
principle this record would serve, that a citation resolves to exactly one
thing, is missing rather than absent; the blueprint is its own ticket.

## How it works

**An id is the tag `VTT` and a number**, `VTT-001` upward, with no per-subject
segment. The tag is declared once, on the register's `project:` line.

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

Until the chain gate exists, a citation that resolves to nothing is caught by a
reading and by nothing else; `CLAUDE.md`'s record of the register says so in
one sentence, and that sentence changes when the gate lands.

A search of `internal/`, `cmd/`, `client/` and `tools/` for `VTT-` followed by
digits finds no citation; the first arrive with the first arc's ticket.

## Requirements

None allocated. The register has no rows; the first arc's ticket allocates the
first.
