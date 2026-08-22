#!/usr/bin/env python3
"""Flag badly wrapped comment lines in the files a branch has touched.

WHY THIS EXISTS. Orphaned line-wraps were found by eye in three separate rounds
of one task and INTRODUCED by eye in two of them — including once in the very
commit whose job was hunting them, in the file that commit was about. Reading
carefully does not scale to this; a width comparison does. Five lines of logic
settle permanently what four careful reads did not.

TWO SHAPES, which are inverses of each other:

  SHORT  a comment line well under the band, not ending a sentence, with more
         comment after it — text was spliced in and the tail was never reflowed.
  LONG   a comment line well over the band — new text was spliced onto the FRONT
         of an existing sentence without rewrapping.

THE BENIGN TWIN, and it must be exempted or this cries wolf. A short line before
an unsplittable token is CORRECT wrapping: a long test name with no break in it
has to start a line of its own, and the line before it is short for that reason
rather than from a splice. Any line whose successor leads with a token longer
than the slack is therefore allowed to be short.

NOT A GATE. It reports; it does not fail a build. The band is a convention, not
a rule, and this repo has legitimate long lines (tables, quoted errors, URLs).
Run it over a diff, read what it says, and use judgement — the point is that the
judgement is applied to a short list rather than to every comment in the repo.

Usage:  python3 tools/check-comment-wrap.py <file>...
        git diff --name-only main...HEAD | xargs python3 tools/check-comment-wrap.py
"""

import re
import sys

# THE BAND, MEASURED RATHER THAN GUESSED. Over the 15887 comment lines in
# internal/, raw width is 72 at the median, 77 at p90, 80 at p99 and 84 at
# p99.9 — so this repo wraps prose to 80 columns counting a tab as ONE column,
# which is what gofmt-formatted Go looks like in an 8-column editor at these
# indent depths. LONG is therefore 85: past p99.9, catching genuine outliers
# without churning over the tail. A first draft used 88 with tabs expanded to 8
# and flagged 168 lines, nearly all of them correct — the band has to come from
# the corpus, not from a habit.
SHORT, LONG = 55, 85

COMMENT = re.compile(r"^\s*(//+|\*|#)\s?(.*)$")
# A block of aligned columns is a DIAGRAM, not prose — this repo draws grid maps
# in comments — and a diagram may be any width. Runs of interior whitespace are
# the signature, and it is checked over the NEIGHBOURING lines too because a
# diagram comes in blocks: the header row of project_test.go's map has four
# spaces between columns while its data rows have two, so testing each line in
# isolation exempts the header and flags the rows under it.
DIAGRAM = re.compile(r"\S {2,}\S")
# A line that ends a paragraph or a sentence is allowed to be short.
ENDS = (".", ":", ";", ")", "]", "|", "-", ">", "—")


def leading_token(text):
    """The first whitespace-delimited token, which may be unsplittable."""
    parts = text.split()
    return parts[0] if parts else ""


def check(path):
    try:
        lines = open(path, encoding="utf-8").read().split("\n")
    except (OSError, UnicodeDecodeError):
        return []

    out = []
    for i, line in enumerate(lines):
        m = COMMENT.match(line)
        if not m:
            continue
        body = m.group(2).strip()
        if not body:
            continue  # a bare `//` separator is a paragraph break, not an orphan
        near = [m.group(2)]
        for j in (i - 1, i + 1):
            if 0 <= j < len(lines):
                n = COMMENT.match(lines[j])
                if n:
                    near.append(n.group(2))
        if any(DIAGRAM.search(t) for t in near):
            continue
        width = len(line)

        nxt = COMMENT.match(lines[i + 1]) if i + 1 < len(lines) else None
        nxt_body = nxt.group(2).strip() if nxt else ""

        if width > LONG:
            out.append((path, i + 1, width, "LONG ", body[:58]))
            continue
        if width >= SHORT or not nxt_body:
            continue
        if body.endswith(ENDS):
            continue
        # THE BENIGN TWIN: the next line leads with a token that would not have
        # fit in the slack, so wrapping early was correct.
        if width + 1 + len(leading_token(nxt_body)) > LONG:
            continue
        out.append((path, i + 1, width, "SHORT", body[:58]))
    return out


def main(argv):
    findings = []
    for path in argv[1:]:
        findings += check(path)
    for path, ln, width, kind, text in findings:
        print(f"{kind} {path}:{ln} [{width}] {text}")
    print(f"check-comment-wrap: {len(argv) - 1} file(s), {len(findings)} flagged "
          f"(band {SHORT}-{LONG}; SHORT and LONG are the two splice shapes)")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
