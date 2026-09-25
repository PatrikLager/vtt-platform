#!/usr/bin/env python3
"""Hold SPEC-010: a comment in code is a warning or a pointer.

    check-comments.py <base-ref>    the gate, over the comment lines the change
                                    adds against <base-ref> and over every
                                    file's comment share
    check-comments.py --write-ledger  lower tools/comment-ceilings.txt to the
                                    shares measured now; never raise a row;
                                    move a row a git rename carried; drop rows
                                    whose file is gone
    check-comments.py --report      per-file shares, ceilings, banned lines and
                                    blocks over the bound, for the sweep

Scope: Go and TypeScript files under internal/, cmd/ and client/src, less
generated ones (.pb.go, .d.ts, a gen/ directory). A comment line is one whose
stripped text starts with `//` in Go, or `//`, `*` or `/*` in TypeScript, and
every line of a `/* ... */` block in either. A block is a run of comment lines
that only a code line ends. A file's share is its comment lines over its
non-blank lines, in percent to one decimal. Added lines are the change's
against the base's merge base, an untracked file whole; a rename is one git
detects, so `git mv` or `git add` of both paths. A `//` line carrying only
requirement ids of the register's tag, spaces between and nothing else,
`// VTT-042` or `// VTT-048 VTT-049` (the tag is the `project:` line of
docs/requirements.md, read as tools/check-requirements-chain.py reads it;
whether an id resolves is that checker's), is a citation line: it counts in
no share, is not an added comment line and belongs to no block, as a blank
line counts in none and ends none. A line inside a `/* ... */` block is a
block line whatever it carries. With no register or no `project:` line, no
line is set aside and every run says so.

The gate refuses, and exits 1:
  - an added comment line carrying a banned term (case-insensitive; matched
    after docs/... paths and [anchor:...] markers are removed, and after the
    directive part of a //go:, //nolint, //lint:, //line, //export, //extern,
    //sys or // +build line, so only a trailing // reason on such a line is
    read): a date 20YY-MM-DD, measured, used to, previously, an earlier
    version, turned out, review found, spec §, task N, #NNN
  - a block of more than 6 lines (the bound) to which the change added a
    line, the Go package doc excepted
  - a file above its ceiling to which the change added a comment line (above
    with no added comment line is a notice)
  - a ledger row above the base's row for the same path (renames followed),
    or above 25.0 (the default ceiling) when the base has no row for it
  - a row whose file does not exist, a renamed-away path included: run
    --write-ledger, which moves the row under the new path
  - a file more than 1.0 point (the band) under its ceiling: run
    --write-ledger, which lowers rows to the measured share, never raises one,
    drops rows whose file is gone, and adds a row for a file that has none
    when its share is at or under the default
  - a file with no row above the default ceiling
It exits 2 when it scans no file in scope, cannot establish the base, or finds
no ledger or one it cannot read (a value outside 0.0 to 100.0 with one decimal,
or a path twice); and ends a clean run with a completion line.

Rows: VTT-050 to VTT-059.
"""
import importlib.util
import math
import os
import pathlib
import re
import subprocess
import sys

LEDGER = "tools/comment-ceilings.txt"
REGISTER = "docs/requirements.md"
ROOTS = ("internal/", "cmd/", "client/src/")
SKIP_DIRS = {"gen", "node_modules", ".git", "contract-spike", "webdist"}
BOUND = 6
DEFAULT = 25.0
BAND = 1.0
EPS = 1e-9
BANNED = re.compile(
    r"\b20\d\d-\d\d-\d\d\b|\bmeasured\b|\bused to\b|\bpreviously\b|\ban earlier version\b"
    r"|\bturned out\b|\breview found\b|\bspec §|\btask \d+\b|(?<![\w&])#\d+\b", re.I)
DOCS_PATH = re.compile(r"docs/[\w./-]+")
ANCHOR = re.compile(r"\[anchor:[\w-]+\]")
DIRECTIVE = re.compile(r"^\s*//(go:|nolint|lint:|line |export |extern |sys | \+build)")
VALUE = re.compile(r"^\d{1,3}\.\d$")
TAG_RE = re.compile(r"^\s*project:\s*([A-Z][A-Z0-9]*)\s*$")  # the chain checker's

_prose_spec = importlib.util.spec_from_file_location(
    "check_new_prose", os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-new-prose.py"))
prose = importlib.util.module_from_spec(_prose_spec)
_prose_spec.loader.exec_module(prose)


def in_scope(path):
    p = pathlib.PurePosixPath(path)
    if not path.startswith(ROOTS) or not SKIP_DIRS.isdisjoint(p.parts):
        return False
    if path.endswith(".pb.go") or path.endswith(".d.ts"):
        return False
    return path.endswith((".go", ".ts"))


def prose_of(line):
    """The part of a comment line the banned list is matched against."""
    if DIRECTIVE.match(line):
        body = line.strip()[2:]
        line = body.split(" // ", 1)[1] if " // " in body else ""
    return ANCHOR.sub("", DOCS_PATH.sub("", line))


def read(path):
    try:
        with open(path, encoding="utf-8") as fh:
            return fh.read().split("\n")
    except (OSError, UnicodeDecodeError):
        return None


def register_tag():
    lines = read(REGISTER)
    if lines is None:
        return None
    return next((m.group(1) for m in map(TAG_RE.match, lines) if m), None)


def cite_pattern(tag):
    """A `//` line whose text is ids of `tag`, spaces between, nothing else;
    ASCII digits, as the chain checker's id shape."""
    if tag is None:
        return None
    one = re.escape(tag) + r"-[0-9]{3,}"
    return re.compile(r"^// *%s(?: +%s)* *$" % (one, one))


def measure(lines, ts, cite=None):
    """(nonblank, set of comment line numbers, blocks, set of citation line
    numbers); a block is a list of line numbers paired with whether it is the
    Go package doc. A citation line, one `cite` matches outside a /* */ block,
    is set aside like a blank line: in no count and in no block."""
    nonblank = 0
    comments, cites = set(), set()
    blocks, cur = [], []
    in_block = False
    for i, line in enumerate(lines, 1):
        s = line.strip()
        if not s:
            continue  # a blank line neither counts nor ends a block
        if cite is not None and not in_block and cite.match(s):
            cites.add(i)
            continue
        nonblank += 1
        if in_block:
            is_c, in_block = True, "*/" not in s
        elif s.startswith("/*"):
            is_c, in_block = True, "*/" not in s
        else:
            is_c = s.startswith("//") or (ts and s.startswith("*"))
        if is_c:
            comments.add(i)
            cur.append(i)
        else:
            if cur:
                blocks.append((cur, (not ts) and s.startswith("package ")))
                cur = []
    if cur:
        blocks.append((cur, False))
    return nonblank, comments, blocks, cites


def share_of(nonblank, comment):
    return 100.0 * comment / nonblank if nonblank else 0.0


def ceil1(x):
    return math.ceil(x * 10 - EPS) / 10


def scan_tree():
    files = []
    for root in ROOTS:
        for dirpath, dirnames, filenames in os.walk(root.rstrip("/")):
            dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
            for name in filenames:
                path = os.path.join(dirpath, name).replace(os.sep, "/")
                if in_scope(path):
                    files.append(path)
    return sorted(files)


def parse_ledger(text):
    rows = {}
    for n, line in enumerate(text.split("\n"), 1):
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        parts = s.split()
        if len(parts) != 2 or not VALUE.match(parts[1]) or float(parts[1]) > 100.0:
            raise ValueError("line %d is not '<path>  <share>' with a share 0.0 to 100.0: %r" % (n, line))
        if parts[0] in rows:
            raise ValueError("line %d names %s a second time" % (n, parts[0]))
        rows[parts[0]] = float(parts[1])
    return rows


def write_ledger(rows):
    body = ["# Comment-share ceilings, one row per Go or TypeScript file under",
            "# internal/, cmd/ and client/src: the file's share in percent, at or",
            "# under which check:comments holds it. Written and lowered only by",
            "# `python3 tools/check-comments.py --write-ledger`, never by hand and",
            "# never raised. SPEC-010.", ""]
    body += ["%s  %.1f" % (path, rows[path]) for path in sorted(rows)]
    with open(LEDGER, "w", encoding="utf-8") as fh:
        fh.write("\n".join(body) + "\n")


def renames(base):
    """new path -> old path, from git's rename detection against the base."""
    out = subprocess.run(["git", "-c", "core.quotePath=false", "diff", "--no-ext-diff", "-M",
                          "--name-status", "--merge-base", base, "--"],
                         capture_output=True, text=True, check=True).stdout
    pairs = {}
    for line in out.splitlines():
        parts = line.split("\t")
        if parts[0].startswith("R") and len(parts) == 3:
            pairs[parts[2]] = parts[1]
    return pairs


def base_ledger(base):
    got = subprocess.run(["git", "show", "%s:%s" % (base, LEDGER)], capture_output=True, text=True)
    if got.returncode != 0:
        return None
    return parse_ledger(got.stdout)


def load_ledger():
    """(rows, error): the working tree's ledger, or why it cannot be read."""
    if not os.path.exists(LEDGER):
        return None, "no ledger at %s; nothing is proven. Write one with --write-ledger." % LEDGER
    try:
        return parse_ledger(open(LEDGER, encoding="utf-8").read()), None
    except ValueError as err:
        return None, "%s cannot be read: %s" % (LEDGER, err)


def measure_all(files, cite):
    out = {}
    for path in files:
        lines = read(path)
        if lines is None:
            continue
        out[path] = measure(lines, path.endswith(".ts"), cite) + (lines,)
    return out


def report(files, rows, cite):
    total_nb = total_c = banned = over = aside = 0
    for path, (nonblank, comments, blocks, cites, lines) in measure_all(files, cite).items():
        b = sum(1 for i in comments if BANNED.search(prose_of(lines[i - 1])))
        o = sum(1 for blk, pkg in blocks if len(blk) > BOUND and not pkg)
        ceiling = rows.get(path)
        print("%-60s %5.1f%%  ceiling %s  banned %3d  blocks>%d %3d  cites %3d" % (
            path, share_of(nonblank, len(comments)), "%5.1f" % ceiling if ceiling is not None else "  none", b, BOUND, o, len(cites)))
        total_nb += nonblank; total_c += len(comments); banned += b; over += o; aside += len(cites)
    print("check:comments --report: %d files, %d comment lines of %d (%d%%), %d banned lines, %d blocks over %d, %d citation lines set aside"
          % (len(files), total_c, total_nb, 100 * total_c // total_nb if total_nb else 0, banned, over, BOUND, aside))
    return 0


def do_write_ledger(files, rows, base, cite):
    pairs = renames(base) if base else {}
    measured = measure_all(files, cite)
    new, stranded = {}, []
    for path, (nonblank, comments, _, _, _) in measured.items():
        now = ceil1(share_of(nonblank, len(comments)))
        old = rows.get(path)
        if old is None and path in pairs:
            old = rows.get(pairs[path])
        if old is not None:
            new[path] = min(old, now)
        elif not rows or now <= DEFAULT + EPS:
            new[path] = now  # a first ledger takes every file as it is
        else:
            stranded.append((path, now))
    dropped = [p for p in rows if p not in new and p not in pairs.values()]
    if stranded and dropped:
        for path, now in stranded:
            print("check:comments --write-ledger: %s at %.1f has no row and is above the default %.1f, while %s would be dropped; "
                  "if this is a rename, `git mv` or `git add` both paths first so the row can follow it"
                  % (path, now, DEFAULT, ", ".join(dropped)))
        print("check:comments --write-ledger: nothing written")
        return 1
    write_ledger(new)
    print("check:comments --write-ledger: %d rows written to %s" % (len(new), LEDGER))
    return 0


def gate(base_arg, cite):
    base = prose.resolve_base(base_arg)
    if base is None:
        print("check:comments: no base to measure added lines against (%s); nothing is proven." % base_arg)
        return 2
    files = scan_tree()
    if not files:
        print("check:comments: nothing was scanned, so nothing is proven: no .go or .ts file under %s"
              % ", ".join(r.rstrip("/") for r in ROOTS))
        return 2
    rows, err = load_ledger()
    if err:
        print("check:comments: " + err)
        return 2

    added = prose.added_lines(base)
    if added is None:
        print("check:comments: the added-line scope could not be read (see the line above); nothing is proven.")
        return 2
    added = {p: ls for p, ls in added.items() if in_scope(p)}
    old_of = renames(base)
    base_rows = base_ledger(base)
    measured = measure_all(files, cite)

    findings, notices = [], []
    added_comment_lines = 0
    for path, (nonblank, comments, blocks, _, lines) in measured.items():
        new_lines = added.get(path, set())
        added_here = 0
        for i in sorted(new_lines & comments):
            added_here += 1
            m = BANNED.search(prose_of(lines[i - 1]))
            if m:
                findings.append("%s:%d: an added comment line carries %r; a comment is a warning or a pointer (SPEC-010)"
                                % (path, i, m.group(0)))
        added_comment_lines += added_here
        for blk, pkg in blocks:
            if len(blk) > BOUND and not pkg and any(i in new_lines for i in blk):
                findings.append("%s:%d: a comment block of %d lines is over the bound of %d and this change touched it (SPEC-010)"
                                % (path, blk[0], len(blk), BOUND))
        share = share_of(nonblank, len(comments))
        ceiling = rows.get(path)
        if ceiling is None:
            if share > DEFAULT + EPS:
                findings.append("%s: comment share %.1f with no row in %s is above the default ceiling %.1f (SPEC-010)"
                                % (path, share, LEDGER, DEFAULT))
            continue
        if share > ceiling + EPS:
            if added_here:
                findings.append("%s: comment share %.1f is above its ceiling %.1f and this change added a comment line to it (SPEC-010)"
                                % (path, share, ceiling))
            else:
                notices.append("check:comments: notice: %s is at %.1f above its ceiling %.1f with no comment line added; the next change that adds one brings it under"
                               % (path, share, ceiling))
        elif share < ceiling - BAND - EPS:
            findings.append("%s: comment share %.1f has fallen more than %.1f under its ceiling %.1f; record it: python3 tools/check-comments.py --write-ledger (SPEC-010)"
                            % (path, share, BAND, ceiling))
    for path, ceiling in rows.items():
        if not os.path.exists(path):
            findings.append("%s: stale row for %s, which does not exist; --write-ledger moves it after a rename git can see, or drops it (SPEC-010)"
                            % (LEDGER, path))
        if base_rows is not None:
            was = base_rows.get(path)
            if was is None and path in old_of:
                was = base_rows.get(old_of[path])
            if was is not None and ceiling > was + EPS:
                findings.append("%s: row for %s raised to %.1f above the base's %.1f; a ceiling is never raised (SPEC-010)"
                                % (LEDGER, path, ceiling, was))
            elif was is None and ceiling > DEFAULT + EPS:
                findings.append("%s: row for %s at %.1f has no row at the base and is above the default %.1f (SPEC-010)"
                                % (LEDGER, path, ceiling, DEFAULT))

    for line in notices:
        print(line)
    for line in findings:
        print("check:comments: " + line)
    tail = "; no ledger at %s, raise check skipped" % base_arg if base_rows is None else ""
    if cite is None:
        tail += "; no tag in %s, citation lines counted as comment lines" % REGISTER
    if findings:
        print("check:comments: %d finding(s) on %d files, %d added comment lines, %d ledger rows%s"
              % (len(findings), len(measured), added_comment_lines, len(rows), tail))
        return 1
    print("check:comments: %d files, %d added comment lines, %d ledger rows; clean%s"
          % (len(measured), added_comment_lines, len(rows), tail))
    return 0


def main(argv):
    if len(argv) == 2 and argv[1] in ("--report", "--write-ledger"):
        files = scan_tree()
        if not files:
            print("check:comments: nothing was scanned, so nothing is proven.")
            return 2
        rows = {}
        if os.path.exists(LEDGER):
            rows, err = load_ledger()
            if err:
                print("check:comments: " + err)
                return 2
        cite = cite_pattern(register_tag())
        if cite is None:
            print("check:comments: no tag in %s, citation lines counted as comment lines" % REGISTER)
        if argv[1] == "--report":
            return report(files, rows, cite)
        return do_write_ledger(files, rows, prose.resolve_base("main") if rows else None, cite)
    if len(argv) == 2 and not argv[1].startswith("-"):
        return gate(argv[1], cite_pattern(register_tag()))
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
