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

NOT A GATE BY ITSELF, and that changed by half on 2026-09-14. Run directly it
still only reports: the band is a convention, not a rule, and this repo has
legitimate long lines (tables, quoted errors, URLs), so the whole tree's
findings are a list to read with judgement rather than a build to fail.

But check:new-prose now promotes this output to a FAILING gate for the lines a
change ADDS, in .go and .ts outside the generated trees. Within that scope a
finding here costs something, which is why HATCH below exists: a line can be
correct and flagged at once, and rewording prose that was right is the wrong
way out.

Usage:  python3 tools/check-comment-wrap.py <file>...
        git diff --name-only main...HEAD | xargs python3 tools/check-comment-wrap.py
        (that three-dot range is the by-hand idiom; the gate scopes to added
        LINES instead, so an older finding in a touched file is not yours)
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

# ADJUDICATION. Same annotation as check-doc-owner.py and check-citations.py --
# `startswith` on the comment body, so prose ABOUT a hatch is not one. (Theirs
# was a substring test over a whole block until the same day this arrived, and
# silenced every citation in a block that merely MENTIONED the hatch.)
#
# SCOPED TO THE LINE, where theirs cover a contiguous BLOCK, and the difference
# is deliberate rather than an oversight: their finding is about what a block
# SAYS, so one reason covers the paragraph, while a finding here is about one
# line's WIDTH and the line beside it is a separate fact. A `wrap:ok` above a
# five-line paragraph therefore adjudicates one line, not five.
# The band is a convention and this tree holds lines that are deliberately off
# it -- a quoted error, a URL, another checker's hatch. Until check:new-prose
# promoted this tool's output to a gate for added lines, a finding here cost
# nothing and needed no escape; now a line can be correct AND flagged, so it
# needs a way to say so in the source rather than by rewording prose that was
# right. Opens the comment body, not merely appears in it: an exemption is a
# clean verdict, so it takes the narrow test.
HATCH = "wrap:ok"


def leading_token(text):
    """The first whitespace-delimited token, which may be unsplittable."""
    parts = text.split()
    return parts[0] if parts else ""


def check(path):
    try:
        with open(path, encoding="utf-8") as fh:
            lines = fh.read().split("\n")
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
        if body.startswith(HATCH):
            continue  # adjudicated in the source, by a reader who looked
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
