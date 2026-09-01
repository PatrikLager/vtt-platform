#!/usr/bin/env python3
"""Boundary tests for check-no-retraction.py.

The gate's whole value is the line it draws between a retraction IDENTIFIER
(gone, and it must stay gone) and the word `retract` in prose (kept on
purpose: ~90 dated change-records across 25 files explain why each shape
changed, and they are why the next reader does not re-derive retraction from
first principles). A gate that cannot tell those apart is either a gate that
reds on 90 true sentences or one switched off with file exclusions, so the
distinction is what these tests drive.

Run: python3 tools/check_no_retraction_test.py
"""

import importlib.util
import io
import os
import pathlib
import shutil
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout

_spec = importlib.util.spec_from_file_location(
    "check_no_retraction",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-no-retraction.py"))
cnr = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(cnr)


def tree(**files):
    """A temp source tree; returns its path. Keys use / for subdirectories."""
    d = tempfile.mkdtemp()
    for name, body in files.items():
        p = pathlib.Path(d) / name.replace("__", "/")
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body, encoding="utf-8")
    return d


def run(root):
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = cnr.main(["check-no-retraction.py", root])
    return code, out.getvalue(), err.getvalue()


class CodePositionsAreCaught(unittest.TestCase):
    """The defect: retraction comes back one helper at a time."""

    def test_a_go_function_named_for_retraction_is_caught(self):
        root = tree(**{"internal/campaign/undo.go": '''package campaign

func retractEvents(n int) error { return nil }
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("undo.go:3", err)
            self.assertIn("retractEvents", err)
        finally:
            shutil.rmtree(root)

    def test_a_typescript_function_named_for_retraction_is_caught(self):
        root = tree(**{"client/src/fold.ts": '''export function retractableRange() {
  return 0;
}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("retractableRange", err)
        finally:
            shutil.rmtree(root)

    def test_a_proto_message_named_for_retraction_is_caught(self):
        root = tree(**{"contract/vtt/v1/events.proto": '''syntax = "proto3";
message EventsRetracted { int64 through = 1; }
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("EventsRetracted", err)
        finally:
            shutil.rmtree(root)

    def test_a_python_helper_named_for_retraction_is_caught(self):
        root = tree(**{"tools/thing.py": '''def retract_entry(x):
    return x
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("retract_entry", err)
        finally:
            shutil.rmtree(root)

    def test_the_match_is_case_insensitive(self):
        """`Rescind` is a synonym no pattern catches; `Retract` is the rename it does."""
        root = tree(**{"internal/campaign/a.go": "package campaign\n\ntype RETRACTOR int\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("RETRACTOR", err)
        finally:
            shutil.rmtree(root)

    def test_an_identifier_interpolated_into_a_template_literal_is_caught(self):
        """`${...}` is CODE inside a string, and a gate that swallows it is blind there."""
        root = tree(**{"client/src/view/feed.ts": "const s = `line ${retractLabel(e)} end`;\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("retractLabel", err)
        finally:
            shutil.rmtree(root)

    def test_division_is_not_mistaken_for_a_regex_literal(self):
        """A wrong guess would blank the rest of the line and hide the hit."""
        root = tree(**{"client/src/a.ts": "const half = total / 2;\nconst n = retractCount / 2;\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("retractCount", err)
        finally:
            shutil.rmtree(root)

    def test_a_regex_literal_body_is_a_pattern_and_not_a_hit(self):
        """dm-view.test.ts's shape: `/retract/i` is how the absence is asserted."""
        body = "expect(labels.filter((t) => /undo|retract/i.test(t))).toEqual([]);\n"
        root = tree(**{"client/test/x.test.ts": body})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)


class ProseIsKept(unittest.TestCase):
    """The 90 dated change-records this branch deliberately preserved."""

    def test_a_go_line_comment_recording_the_removal_is_not_a_hit(self):
        root = tree(**{"internal/campaign/campaign.go": '''package campaign

// Retraction used to rewrite this loop; it left on 2026-08-30 because you
// cannot unring a bell.
func Append() {}
'''})
        try:
            code, out, err = run(root)
            self.assertEqual(code, 0, err)
            self.assertIn("clean", out)
        finally:
            shutil.rmtree(root)

    def test_a_go_block_comment_recording_the_removal_is_not_a_hit(self):
        root = tree(**{"internal/engine/apply.go": '''package engine

/*
 * This arm no longer handles retraction.
 */
func Apply() {}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_go_string_literal_asserting_the_absence_is_not_a_hit(self):
        """forward_only_test.go's shape: the pattern IS the enforcement."""
        root = tree(**{"internal/campaign/forward_only_test.go": '''package campaign

func TestX() {
	if strings.Contains(lower, "retract") {
		panic("no")
	}
}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_go_raw_string_is_not_a_hit(self):
        root = tree(**{"internal/store/a.go": 'package store\n\nvar s = `retract me`\n'})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_typescript_comment_and_string_are_not_hits(self):
        root = tree(**{"client/src/session.ts": '''// retraction used to arrive here as an event
const label = "retracted";
const other = 'retract';
const tpl = `a retract b`;
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_python_comment_and_docstring_are_not_hits(self):
        root = tree(**{"tools/check-thing.py": '''"""A docstring naming retraction, as check-ts-mutation.py's does."""

# retracted 2026-08-05, not corrected
X = 1
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_proto_comment_is_not_a_hit(self):
        root = tree(**{"contract/vtt/v1/commands.proto": '''syntax = "proto3";
// Field 9 is reserved: it carried retraction until 2026-08-30.
message MoveToken { int64 x = 1; }
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)


class JSONKeysAreCodePositions(unittest.TestCase):
    """The spec's exit criterion 1 names `scenarios/`, and .json went unread.

    Until 2026-09-01 `.json` was not in SOURCE_SUFFIXES at all, so a
    `retract_events` step in a scenario file reported clean. In JSON the object
    KEY is the code position — it is the protobuf field name a step, a golden
    stream or a tool manifest is naming — and every value is data.
    """

    def test_a_retraction_command_in_a_scenario_step_is_caught(self):
        root = tree(**{"scenarios/smoke.json": (
            '{\n'
            '  "steps": [\n'
            '    {"by": "dm", "command": {"retract_events": {"fromSequence": 3}}}\n'
            '  ]\n'
            '}\n')})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("smoke.json:3", err)
            self.assertIn("retract_events", err)
        finally:
            shutil.rmtree(root)

    def test_a_retraction_key_in_a_golden_stream_is_caught(self):
        """Not only command positions.

        protojson's unknown-field strictness refuses a bad COMMAND in a file
        something actually loads. A retraction arm in a recorded golden stream
        is refused by nothing else, which is the half this gate has to hold.
        """
        root = tree(**{"scenarios/goldens/g/stream.json": (
            '[\n'
            '  {"sequence": 1, "eventsRetracted": {"fromSequence": 2}}\n'
            ']\n')})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("eventsRetracted", err)
        finally:
            shutil.rmtree(root)

    def test_json_values_are_data_and_not_hits(self):
        """A note, a narration or a scenario name may say what retraction was.

        This is the .json half of the whole gate's premise: the word survives in
        prose on purpose. A value-scanning version of this would red on a
        recorded narration and be switched off with an exclusion within a week.
        """
        root = tree(**{
            "scenarios/story.json": (
                '{\n'
                '  "name": "retraction, and why it left",\n'
                '  "steps": [\n'
                '    {"command": {"addNarration": {"text": "The DM retracts nothing."}}}\n'
                '  ]\n'
                '}\n'),
            "internal/a/a.go": "package a\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_json_file_that_does_not_parse_is_still_scanned(self):
        """The masker is a tokenizer, deliberately, not json.loads.

        A half-written or truncated fixture makes a parser raise, and the file
        would then go unchecked — silently, which is exactly what the
        unreadable-file guard above exists to refuse.
        """
        root = tree(**{"scenarios/broken.json": '{"steps": [{"retract_events": {'})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("retract_events", err)
        finally:
            shutil.rmtree(root)


class ScopeAndExemptions(unittest.TestCase):
    def test_generated_output_is_out_of_scope(self):
        """contract/gen is regenerated, not written; check:drift owns it."""
        root = tree(**{"contract/gen/go/x.pb.go": "package genpb\n\nfunc retractThing() {}\n",
                       "internal/a/a.go": "package a\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_gen_directory_that_is_not_the_generated_one_is_still_scanned(self):
        """The skip is anchored to PATHS, not to the name `gen`.

        A bare-name skip applies at every depth, so `internal/foo/gen/` — source
        somebody wrote, in a package this gate covers — went unscanned and a
        retraction helper there was reported clean. Found in review 2026-09-01.
        """
        root = tree(**{"internal/foo/gen/hidden.go": "package gen\n\nfunc retractSecret() {}\n",
                       "contract/gen/go/x.pb.go": "package genpb\n\nfunc retractThing() {}\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("retractSecret", err)
            # ...and the real generated tree is still out, by its own path.
            self.assertNotIn("retractThing", err)
        finally:
            shutil.rmtree(root)

    def test_every_javascript_and_typescript_spelling_is_scanned(self):
        """Scope must not be "the extension we happen to use today"."""
        for name in ["client/src/a.js", "client/src/b.jsx", "client/src/c.tsx",
                     "client/src/d.mjs", "client/src/e.cjs", "client/src/f.mts"]:
            root = tree(**{name.replace("/", "__"): "export function retractAll() {}\n"})
            try:
                code, _, err = run(root)
                self.assertEqual(code, 1, name)
                self.assertIn("retractAll", err)
            finally:
                shutil.rmtree(root)

    def test_a_file_that_cannot_be_read_fails_rather_than_being_skipped(self):
        """It was skipped silently, and did not even count toward `scanned`."""
        root = tree(**{"internal/a/a.go": "package a\n"})
        try:
            pathlib.Path(root, "internal/a/b.go").write_bytes(b"package a\n// \xff\xfe\n")
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("could not read", err)
            self.assertIn("b.go", err)
        finally:
            shutil.rmtree(root)

    def test_vendored_and_build_output_is_out_of_scope(self):
        root = tree(**{"client/node_modules/p/i.ts": "export function retractAll() {}\n",
                       "cmd/vtt/webdist/a.ts": "export function retractAll() {}\n",
                       "internal/a/a.go": "package a\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_the_exempt_file_may_use_only_the_words_it_is_exempted_for(self):
        """contract/events.test.ts holds the pattern that enforces the absence.

        The exemption is per WORD, not per file: the constant that carries the
        pattern is allowed, and a retraction helper hidden in the same file is
        still caught. A whole-file exclusion would have made the strongest
        enforcement site the one blind spot.
        """
        allowed = 'const RETRACTION = /retract/i;\nexport const R = RETRACTION;\n'
        root = tree(**{"contract/events.test.ts": allowed})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

        root = tree(**{"contract/events.test.ts": allowed + 'function retractEverything() {}\n'})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("retractEverything", err)
        finally:
            shutil.rmtree(root)

    def test_a_tree_with_nothing_to_scan_fails_rather_than_passing(self):
        """The ADR-003 guard's lesson: scanning 0 files while exiting 0 is a
        gate that looks clean and enforces nothing."""
        root = tree(**{"README.md": "no code here\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("scanned no files", err)
        finally:
            shutil.rmtree(root)

    def test_a_clean_tree_reports_what_it_scanned(self):
        root = tree(**{"internal/a/a.go": "package a\n",
                       "client/src/b.ts": "export const b = 1;\n"})
        try:
            code, out, err = run(root)
            self.assertEqual(code, 0, err)
            self.assertIn("2 files", out)
        finally:
            shutil.rmtree(root)


if __name__ == "__main__":
    unittest.main(verbosity=2)
