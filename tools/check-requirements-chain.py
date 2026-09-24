#!/usr/bin/env python3
"""Fail when the chain between the register, the tests and the specifications is broken.

The register is docs/requirements.md: a `project: <TAG>` line and a table whose
header names Id and Requirement. A requirement id is `<TAG>-NNN`. A test holds a
rule by carrying the bare id, anywhere in the file, comment or not; a row's
evidence cell names the tests that hold it; a specification names the ids it
carries. Each link is walkable
both ways only if every end of it exists, and nothing else checks that.

WHAT IS SCANNED. Test files are `*_test.go`, `*.test.ts` and `*_test.py` under
internal/, cmd/, client/, tools/ and contract/, skipping node_modules, gen,
contract-spike, .git, .superpowers, .stryker-tmp and .tscov. Specifications are
docs/specifications/*.md, read whole. Rows are the table under the register's
`| Id | Requirement |` header; the tag is the register's `project:` line. Only
ids carrying the register's own tag are citations: another project's tag, in a
fixture or anywhere else, is data.

WHAT IS REFUSED, one finding per line on stderr, exit 1:
  - a test file cites an id that no row defines;
  - a specification names an id that no row defines;
  - a row's id is not of the shape `<TAG>-NNN`, three or more digits and never
    zero, once backticks, bold and underscores are stripped, the same markup
    the dispenser reads through;
  - two rows carry one id;
  - an evidence cell is blank, or is `**READING —` with no review named;
  - a row with fewer than three cells;
  - an evidence entry names no check (`path#Check` is the shape; entries are
    separated by a comma that begins the next `path#Check`, so a comma inside
    a check's name is part of the name), names a file that does not exist,
    names a file that does not carry the row's id, or names a check the file
    does not declare — `func Check(` with or without a receiver in a .go file,
    `def Check(` in a .py file, the name as a quoted string in a .ts, .tsx, .js
    or .mjs file, the name as a whole word in a file of any other kind.

Two evidence cells are answers rather than entries and pass by name:
`**OPEN — no test yet**`, which the dispenser writes, and `**READING — <the
review>**` with the review named.

Exit 2, nothing scanned: no register, a register with no `project:` line or no
requirements header, or no test file under the roots. An empty table is a pass:
a register with no rows is a step, not a defect. A run that reaches the end
prints one line on stdout stating the rows, test files and specifications it
read; a run that prints no such line and no finding did not finish.

Run: python3 tools/check-requirements-chain.py [root]
"""

import pathlib
import re
import sys

REGISTER = "docs/requirements.md"
SPEC_DIR = "docs/specifications"
ROOTS = ("internal", "cmd", "client", "tools", "contract")
TEST_GLOBS = ("*_test.go", "*.test.ts", "*_test.py")
SKIP_DIRS = {"node_modules", "gen", "contract-spike", ".git", ".superpowers", ".stryker-tmp", ".tscov"}

TAG_RE = re.compile(r"^\s*project:\s*([A-Z][A-Z0-9]*)\s*$")
HEADER_RE = re.compile(r"^\|\s*Id\s*\|\s*Requirement\s*\|")
SEPARATOR_RE = re.compile(r"^\|\s*:?-+")
CELL_SPLIT_RE = re.compile(r"(?<!\\)\|")
OPEN = "**OPEN — no test yet**"
READING = "**READING —"
PREFIX = "check:requirements-chain:"


def test_files(root):
    """Every test file under the scanned roots, skipping generated and vendored trees."""
    out = []
    for name in ROOTS:
        base = root / name
        if not base.is_dir():
            continue
        for glob in TEST_GLOBS:
            for path in base.rglob(glob):
                if SKIP_DIRS.isdisjoint(path.relative_to(root).parts):
                    out.append(path)
    return sorted(set(out))


def specifications(root):
    base = root / SPEC_DIR
    return sorted(base.glob("*.md")) if base.is_dir() else []


def strip_markup(cell):
    return re.sub(r"[`*_]", "", cell).strip()


def read_register(path):
    """(tag, rows, why_unusable). A row is (raw_id, id, evidence). why_unusable is
    None when the register can be read, else the sentence that says why not."""
    if not path.is_file():
        return None, [], f"no register at {path}"
    lines = path.read_text(encoding="utf-8").split("\n")
    tag = next((m.group(1) for m in map(TAG_RE.match, lines) if m), None)
    if tag is None:
        return None, [], f"{path} declares no project: line, so no id can be bound to it"
    start = next((i for i, ln in enumerate(lines) if HEADER_RE.match(ln)), None)
    if start is None:
        return tag, [], f"{path} has no requirements header (a row naming Id and Requirement)"
    rows = []
    for ln in lines[start + 1:]:
        if not ln.startswith("|"):
            break
        if SEPARATOR_RE.match(ln):
            continue
        cells = [c.strip() for c in CELL_SPLIT_RE.split(ln)]
        # A row reads `| id | requirement | evidence |`: the split yields a
        # leading and a trailing empty cell around the three.
        cells = cells[1:-1] if len(cells) >= 2 else cells
        raw_id = cells[0] if cells else ""
        # Fewer than three cells is a malformed row, not a blank cell: the
        # usual cause is a missing trailing separator.
        evidence = cells[2] if len(cells) >= 3 else None
        rows.append((raw_id, strip_markup(raw_id), evidence))
    return tag, rows, None


def declares(path, check):
    """Whether path declares a check by that name, in the language's own shape."""
    text = path.read_text(encoding="utf-8", errors="replace")
    name = re.escape(check)
    if path.suffix == ".go":
        pattern = rf"^func (?:\([^)]*\) )?{name}\("
    elif path.suffix == ".py":
        pattern = rf"^\s*def {name}\("
    elif path.suffix in (".ts", ".tsx", ".js", ".mjs"):
        pattern = rf"""["'`]{name}["'`]"""
    else:
        pattern = rf"\b{name}\b"
    return re.search(pattern, text, re.M) is not None


def check_evidence(root, tag, row_id, cell):
    """Findings for one row's evidence cell."""
    if cell is None:
        return [f"row {row_id}: the row has fewer than three cells; is its trailing | missing?"]
    if cell == "":
        return [f"row {row_id}: the evidence cell is blank; it holds test entries, {OPEN}, or a named READING"]
    if cell == OPEN:
        return []
    if cell.startswith(READING):
        review = cell[len(READING):].rstrip("*").strip()
        return [] if review else [f"row {row_id}: READING names no review"]
    out = []
    # A comma begins a new entry only when a path#Check follows it; a comma
    # inside a check's name (283 TypeScript test names carry one) is the name's.
    for entry in (e.strip() for e in re.split(r",\s*(?=[^,#]*#)", cell)):
        if "#" not in entry:
            out.append(f"row {row_id}: evidence entry {entry!r} names no check (the shape is path#Check)")
            continue
        rel, check = entry.split("#", 1)
        path = root / rel.strip()
        check = check.strip()
        if not path.is_file():
            out.append(f"row {row_id}: evidence names {rel.strip()}, which does not exist")
            continue
        if not re.search(rf"\b{re.escape(row_id)}\b", path.read_text(encoding="utf-8", errors="replace")):
            out.append(f"row {row_id}: {rel.strip()} does not carry the id, so the link walks one way only")
        if not declares(path, check):
            out.append(f"row {row_id}: {rel.strip()} declares no check named {check}")
    return out


def scan(root):
    """(counts, findings, why_nothing). counts is (rows, test files, specifications)."""
    root = pathlib.Path(root)
    tag, rows, unusable = read_register(root / REGISTER)
    if unusable:
        return None, [], unusable
    tests = test_files(root)
    if not tests:
        return None, [], f"no test files under {', '.join(ROOTS)} in {root}"
    specs = specifications(root)
    id_re = re.compile(rf"\b{tag}-[0-9]{{3,}}\b")
    shape_re = re.compile(rf"^{tag}-[0-9]{{3,}}$")
    findings = []
    seen = set()
    for raw_id, row_id, evidence in rows:
        if not shape_re.match(row_id) or int(row_id.rsplit("-", 1)[1]) == 0:
            findings.append(f"row {raw_id!r}: the id is not of the shape {tag}-NNN counted from one")
            continue
        if row_id in seen:
            findings.append(f"row {row_id}: this id appears twice; an id is allocated once")
            continue
        seen.add(row_id)
        findings.extend(check_evidence(root, tag, row_id, evidence))
    for group, paths in (("test", tests), ("specification", specs)):
        for path in paths:
            text = path.read_text(encoding="utf-8", errors="replace")
            for cited in sorted(set(id_re.findall(text))):
                if cited not in seen:
                    findings.append(
                        f"{path.relative_to(root)} cites {cited} and no row in {REGISTER} defines it"
                        f" ({group} citation)")
    return (len(rows), len(tests), len(specs)), findings, None


def main(argv):
    root = argv[1] if len(argv) > 1 else "."
    counts, findings, why_nothing = scan(root)
    if why_nothing:
        print(f"{PREFIX} {why_nothing} — nothing was scanned, so nothing is proven.", file=sys.stderr)
        return 2
    for finding in findings:
        print(f"{PREFIX} {finding}", file=sys.stderr)
    if findings:
        return 1
    rows, tests, specs = counts
    print(f"{PREFIX} {rows} rows, {tests} test files, {specs} specifications; "
          f"every citation resolves and every row's evidence holds.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
