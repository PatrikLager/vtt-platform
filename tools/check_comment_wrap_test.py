#!/usr/bin/env python3
"""Tests for check-comment-wrap.py.

IT EXISTS BECAUSE THE CHECKER WAS WRONG TWICE before anyone trusted its output,
and each time it was wrong in the confident direction: it reported a clean sweep
over a file that had drifted onto a `/**` line, and it reported 168 findings over
a repo whose wrapping was fine. This repo's standing rule for enforcement code
applies to advisory code too — a checker that cannot fail is worse than none, so
test it against deliberately broken input before believing it.

Each case below is one of the bugs that actually happened.
"""

import importlib.util
import tempfile
import textwrap
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "ccw", Path(__file__).with_name("check-comment-wrap.py"))
ccw = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ccw)


def flags(src, suffix=".go"):
    with tempfile.NamedTemporaryFile("w", suffix=suffix, delete=False) as f:
        f.write(textwrap.dedent(src))
        path = f.name
    return [(kind.strip(), ln) for _, ln, _, kind, _ in ccw.check(path)]


class CheckCommentWrap(unittest.TestCase):
    def test_short_orphan_is_flagged(self):
        # The shape found at internal/engine/state.go: a splice whose tail was
        # never reflowed, mid-sentence, with more comment after it.
        got = flags("""\
            // aaaa bbbb cccc dddd eeee ffff gggg hhhh iiii jjjj kkkk llll mmmm nn
            // the pair is what the client needs
            // since terrain you remember but cannot currently see is the fog here
            """)
        self.assertIn(("SHORT", 2), got)

    def test_long_splice_is_flagged(self):
        # The inverse shape, found at internal/mapdef/compile.go: new text
        # spliced onto the FRONT of an existing sentence without rewrapping.
        got = flags("""\
            // correction to a number. Past it the frame simply does not arrive, and the way that presents
            // is a connection torn down mid-session
            """)
        self.assertIn(("LONG", 1), got)

    def test_sentence_end_is_not_an_orphan(self):
        got = flags("""\
            // a short closing line is fine.
            // because it ends the sentence it is not a splice artefact at all ok
            """)
        self.assertEqual([], got)

    def test_benign_twin_is_exempt(self):
        # A short line before an unsplittable token is CORRECT wrapping.
        # The name below is long enough that no wrap could have fitted it on
        # the line above, which is the whole condition the exemption tests.
        got = flags("""\
            // the pair panics, which is what
            // TestASeatPerchesOnlyAgainstAWorldItHasSeenAndThenSomeMoreWords catches.
            """)
        self.assertEqual([], got)

    def test_diagram_is_exempt(self):
        # This repo draws grid maps in comments; aligned columns are not prose.
        got = flags("""\
            //  x: 0    1    2    3     4    5    6
            // y=0 wall wall wall wall  wall wall wall
            // y=1 wall flr  flr  door  flr  flr  wall
            """)
        self.assertEqual([], got)

    def test_blank_comment_is_a_paragraph_break(self):
        got = flags("""\
            // a line that does not end a sentence and is short
            //
            // a following paragraph that is long enough not to be flagged here ok
            """)
        self.assertEqual([], got)

    def test_tabs_are_measured_raw_not_expanded(self):
        # Expanding tabs to 8 columns matched no convention this repo follows
        # and flagged 168 correctly-wrapped lines. Raw len() is the metric.
        indented = "\t\t// " + "x" * 70 + "\n"
        with tempfile.NamedTemporaryFile("w", suffix=".go", delete=False) as f:
            f.write(indented)
            path = f.name
        self.assertEqual([], ccw.check(path))

    def test_last_line_of_file_is_not_flagged(self):
        # Nothing follows it, so it cannot be a splice artefact.
        got = flags("// a trailing short comment\n")
        self.assertEqual([], got)


if __name__ == "__main__":
    unittest.main()
