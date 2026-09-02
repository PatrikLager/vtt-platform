#!/usr/bin/env python3
"""Boundary tests for check-no-create-scene.py.

The gate's whole value is the line it draws between a scene-making COMMAND
identifier (gone, and it must stay gone) and the words in prose (kept on
purpose: dated change-records across the tree explain why each shape changed,
and they are why the next reader does not re-derive the removal from first
principles). A gate that cannot tell those apart is either a gate that reds on
a hundred true sentences or one switched off with file exclusions, so the
distinction is what these tests drive.

THE SECOND LINE IS THIS GATE'S OWN, and the retraction gate had no equivalent:
the EVENT stays. `SceneCreated` is still emitted, still folded and still on the
wire — load_map and load_adventure both produce it — so a gate that matched on
"scene" or on "create" near "scene" would red on the healthy half of the
platform. TheEventIsNotTheCommand below is that boundary, and it is the test
most likely to catch a careless widening of the needle later.

Run: python3 tools/check_no_create_scene_test.py
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
    "check_no_create_scene",
    os.path.join(os.path.dirname(os.path.abspath(__file__)),
                 "check-no-create-scene.py"))
gate = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(gate)


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
        code = gate.main(["check-no-create-scene.py", root])
    return code, out.getvalue(), err.getvalue()


class CodePositionsAreCaught(unittest.TestCase):
    """The defect: the command comes back one helper at a time."""

    def test_a_go_function_named_for_the_removed_command_is_caught(self):
        root = tree(**{"internal/gateway/validate.go": '''package gateway

func validateCreateScene(n int) error { return nil }
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("validate.go:3", err)
            self.assertIn("validateCreateScene", err)
        finally:
            shutil.rmtree(root)

    def test_a_generated_oneof_wrapper_referenced_from_hand_code_is_caught(self):
        """The shape a reintroduced gateway arm actually has.

        `contract/gen` is out of scope on purpose, so the wrapper TYPE is not
        reported where it is generated. The type is still spelled out at every
        hand-written switch arm that routes to it, and that is a code position
        in a scanned file.
        """
        root = tree(**{"internal/gateway/server.go": '''package gateway

func route(c *vttv1.ClientCommand) {
	switch c.GetCommand().(type) {
	case *vttv1.ClientCommand_CreateScene:
	}
}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("ClientCommand_CreateScene", err)
        finally:
            shutil.rmtree(root)

    def test_a_typescript_builder_is_caught(self):
        root = tree(**{"client/src/commands.ts": '''export function createScene() {
  return {};
}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createScene", err)
        finally:
            shutil.rmtree(root)

    def test_a_proto_message_and_its_field_are_both_caught(self):
        """Field 11 is a hole with no `reserved`, so the name can be retaken."""
        root = tree(**{"contract/vtt/v1/commands.proto": '''syntax = "proto3";
message CreateSceneRequest { string scene_id = 1; }
message ClientCommand {
  oneof command { CreateSceneRequest create_scene = 11; }
}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("CreateSceneRequest", err)
            self.assertIn("create_scene", err)
        finally:
            shutil.rmtree(root)

    def test_a_python_helper_is_caught(self):
        root = tree(**{"tools/thing.py": '''def create_scene(x):
    return x
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("create_scene", err)
        finally:
            shutil.rmtree(root)

    def test_all_three_casings_normalize_to_the_same_needle(self):
        """snake, camel and Pascal are one identifier wearing three hats.

        The contract spells it `create_scene`, the TypeScript client
        `createScene` and Go `CreateScene`, and a gate that knew only one of
        them would report clean on the other two. Underscores are DELETED
        rather than treated as separators, which is what makes the generated
        wrapper `ClientCommand_CreateScene` a match as well.
        """
        for spelling in ["create_scene", "createScene", "CreateScene",
                         "CREATE_SCENE", "Create_Scene", "createscene"]:
            root = tree(**{"internal/a/a.go": "package a\n\nvar %s = 1\n" % spelling})
            try:
                code, _, err = run(root)
                self.assertEqual(code, 1, spelling)
                self.assertIn(spelling, err)
            finally:
                shutil.rmtree(root)

    def test_an_identifier_interpolated_into_a_template_literal_is_caught(self):
        """`${...}` is CODE inside a string, and a gate that swallows it is blind there."""
        root = tree(**{"client/src/view/dm.ts":
                       "const s = `row ${createSceneLabel(e)} end`;\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createSceneLabel", err)
        finally:
            shutil.rmtree(root)

    def test_division_is_not_mistaken_for_a_regex_literal(self):
        """A wrong guess would blank the rest of the line and hide the hit."""
        root = tree(**{"client/src/a.ts":
                       "const half = total / 2;\nconst n = createSceneCount / 2;\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createSceneCount", err)
        finally:
            shutil.rmtree(root)

    def test_a_regex_literal_body_is_a_pattern_and_not_a_hit(self):
        """command-surface.test.ts's actual shape, and it is load-bearing.

        `expect(Object.keys(commands).filter((k) => /createScene/i.test(k)))
        .toEqual([])` is the client's absence assertion. If this gate reported
        it, the only way to green would be to exempt the file that enforces the
        absence — the precise self-defeat the word-level design exists to
        avoid.
        """
        body = ("expect(Object.keys(commands).filter((k) => "
                "/createScene/i.test(k))).toEqual([]);\n")
        root = tree(**{"client/test/command-surface.test.ts": body})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)


class TheEventIsNotTheCommand(unittest.TestCase):
    """`SceneCreated` STAYS. This is the boundary a widened needle destroys.

    The command left; the event did not. load_map and load_adventure both emit
    SceneCreated, engine.Apply folds it, the client renders it and the wire
    carries it. Every one of the spellings below is live platform code, and a
    gate that reported any of them would have to be switched off within a day.
    """

    def test_the_event_message_is_not_a_hit(self):
        root = tree(**{"contract/vtt/v1/events.proto": '''syntax = "proto3";
message SceneCreated { string scene_id = 1; }
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_every_live_spelling_of_the_event_is_not_a_hit(self):
        for spelling in ["SceneCreated", "sceneCreated", "scene_created",
                         "SceneCreatedSchema", "Envelope_SceneCreated",
                         "sceneCreatedFrom", "newSceneCreatedEvent"]:
            root = tree(**{"internal/engine/apply.go":
                           "package engine\n\nvar %s = 1\n" % spelling})
            try:
                code, _, err = run(root)
                self.assertEqual(code, 0, "%s: %s" % (spelling, err))
            finally:
                shutil.rmtree(root)

    def test_a_scene_being_created_by_something_else_is_not_a_hit(self):
        """mapdef.Compile and the adventure loader both make scenes.

        The ruling was about a COMMAND that emits a whole scene in one shot,
        not about scenes coming into existence. Compile's own identifiers must
        survive, or the gate forbids the replacement for the thing it removed.
        """
        root = tree(**{"internal/mapdef/compile.go": '''package mapdef

func Compile(m Map) []*vttv1.Envelope { return nil }

func sceneFromMap(m Map) *vttv1.SceneCreated { return nil }
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)


class ProseIsKept(unittest.TestCase):
    """The dated change-records this branch deliberately preserved.

    Measured over the files this gate scans on 2026-09-02: 111 occurrences
    across 42 files, every one of them a comment, a string or a plan-directory
    name inside a citation. Zero code positions.
    """

    def test_a_go_line_comment_recording_the_removal_is_not_a_hit(self):
        root = tree(**{"internal/gateway/server.go": '''package gateway

// create_scene left on 2026-09-01: the kernel serves maps, it does not make
// them. A one-shot command could not edit what it made.
func Serve() {}
'''})
        try:
            code, out, err = run(root)
            self.assertEqual(code, 0, err)
            self.assertIn("clean", out)
        finally:
            shutil.rmtree(root)

    def test_a_citation_naming_the_plan_directory_is_not_a_hit(self):
        """The commonest shape in this tree, by a wide margin.

        Twenty-odd files cite `2026-09-01-create-scene-leaves` by name, which
        is how CLAUDE.md rule 8 says a decision is cited. Hyphens are not
        identifier characters, so this could not be a code position anyway —
        but it is the sentence a careless needle would red on first.
        """
        root = tree(**{"cmd/vtt/maps.go": '''package main

// See 2026-09-01-create-scene-leaves Task 5.
func load() {}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_go_block_comment_recording_the_removal_is_not_a_hit(self):
        root = tree(**{"internal/engine/apply.go": '''package engine

/*
 * This arm no longer handles CreateScene.
 */
func Apply() {}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_go_string_literal_asserting_the_absence_is_not_a_hit(self):
        root = tree(**{"internal/gateway/surface_test.go": '''package gateway

func TestX(t *testing.T) {
	if strings.Contains(lower, "create_scene") {
		t.Fatal("no")
	}
}
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_go_raw_string_is_not_a_hit(self):
        root = tree(**{"internal/store/a.go":
                       'package store\n\nvar s = `{"create_scene": {}}`\n'})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_typescript_comment_and_string_are_not_hits(self):
        root = tree(**{"client/src/session.ts": '''// createScene used to be a builder here
const label = "create_scene";
const other = 'createScene';
const tpl = `a CreateScene b`;
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_python_comment_and_docstring_are_not_hits(self):
        root = tree(**{"tools/check-thing.py":
                       '"""A docstring naming create_scene, as this gate\'s own does."""\n'
                       "\n"
                       "# createScene left 2026-09-01, not corrected\n"
                       "X = 1\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_proto_comment_is_not_a_hit(self):
        root = tree(**{"contract/vtt/v1/commands.proto": '''syntax = "proto3";
// Field 11 is a hole with no reserved: it carried create_scene until
// 2026-09-01, matching 59542e1's convention.
message MoveToken { int64 x = 1; }
'''})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)


class JSONKeysAreCodePositions(unittest.TestCase):
    """In JSON the object KEY is the protobuf field name; the value is data.

    Inherited whole from check-no-retraction.py, which paid a review round to
    learn it: until 2026-09-01 that gate read no `.json` at all, so a command
    step in `scenarios/` reported clean.
    """

    def test_a_command_in_a_scenario_step_is_caught(self):
        root = tree(**{"scenarios/smoke.json": (
            '{\n'
            '  "steps": [\n'
            '    {"by": "dm", "command": {"create_scene": {"width": 3}}}\n'
            '  ]\n'
            '}\n')})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("smoke.json:3", err)
            self.assertIn("create_scene", err)
        finally:
            shutil.rmtree(root)

    def test_a_camel_key_in_a_golden_stream_is_caught(self):
        """protojson's unknown-field strictness refuses a bad COMMAND in a file
        something actually loads. A recorded golden is refused by nothing else."""
        root = tree(**{"scenarios/goldens/g/stream.json": (
            '[\n'
            '  {"sequence": 1, "createScene": {"width": 3}}\n'
            ']\n')})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createScene", err)
        finally:
            shutil.rmtree(root)

    def test_json_values_are_data_and_not_hits(self):
        root = tree(**{
            "scenarios/story.json": (
                '{\n'
                '  "name": "create_scene, and why it left",\n'
                '  "steps": [\n'
                '    {"command": {"addNarration": '
                '{"text": "The DM creates no scenes."}}}\n'
                '  ]\n'
                '}\n'),
            "internal/a/a.go": "package a\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_tool_manifest_name_value_is_a_stated_hole(self):
        """WRITTEN DOWN RATHER THAN HIDDEN, because it is the one place a
        reintroduction could hide from this gate.

        `contract/testdata/expected_tools.json` and `cmd/vtt/tools.json` spell
        a tool's identity as a VALUE — `"name": "load_map"` — not as a key, so
        a `"name": "create_scene"` row is invisible here.

        THE COMPENSATING CONTROL IS REAL BUT IT IS NOT ONE MECHANISM, and the
        difference matters if either half is ever removed. `cmd/vtt/tools.json`
        is a copy that `task generate:contract` rewrites from toolgen's output
        and `check:drift` then diffs, so a hand-added row there is overwritten
        and reported. `contract/testdata/expected_tools.json` is NOT generated
        and NOT in check:drift's pathspec — it is a hand-maintained golden that
        `TestToolsMatchGolden` in tools/toolgen/main_test.go compares against
        `buildTools()`, which reads the compiled descriptors, so a row there
        with no matching oneof arm fails that test. Either way a create_scene
        tool needs a `create_scene` arm in commands.proto, and THAT is a code
        position this gate does see.

        If that ever stops being true, delete this test and teach _mask_json
        about the `name` key. Do not exempt anything.
        """
        root = tree(**{"contract/testdata/expected_tools.json": (
            '[\n'
            '  {"name": "create_scene", "description": "Make a scene."}\n'
            ']\n'),
            "internal/a/a.go": "package a\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_json_file_that_does_not_parse_is_still_scanned(self):
        """The masker is a tokenizer, deliberately, not json.loads: a
        half-written fixture would otherwise go unchecked, silently."""
        root = tree(**{"scenarios/broken.json": '{"steps": [{"create_scene": {'})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("create_scene", err)
        finally:
            shutil.rmtree(root)


class ScopeAndExemptions(unittest.TestCase):
    def test_generated_output_is_out_of_scope(self):
        """contract/gen is regenerated, not written; check:drift owns it."""
        root = tree(**{"contract/gen/go/x.pb.go":
                       "package genpb\n\nfunc createScene() {}\n",
                       "internal/a/a.go": "package a\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_a_gen_directory_that_is_not_the_generated_one_is_still_scanned(self):
        """The skip is anchored to PATHS, not to the name `gen`.

        A bare-name skip applies at every depth, so `internal/foo/gen/` —
        source somebody wrote, in a package this gate covers — would go
        unscanned. Inherited from check-no-retraction.py, where review found it
        on 2026-09-01.
        """
        root = tree(**{"internal/foo/gen/hidden.go":
                       "package gen\n\nfunc createSceneSecret() {}\n",
                       "contract/gen/go/x.pb.go":
                       "package genpb\n\nfunc createSceneThing() {}\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createSceneSecret", err)
            self.assertNotIn("createSceneThing", err)
        finally:
            shutil.rmtree(root)

    def test_every_javascript_and_typescript_spelling_is_scanned(self):
        """Scope must not be "the extension we happen to use today"."""
        for name in ["client/src/a.js", "client/src/b.jsx", "client/src/c.tsx",
                     "client/src/d.mjs", "client/src/e.cjs", "client/src/f.mts",
                     "client/src/g.cts"]:
            root = tree(**{name.replace("/", "__"):
                           "export function createScene() {}\n"})
            try:
                code, _, err = run(root)
                self.assertEqual(code, 1, name)
                self.assertIn("createScene", err)
            finally:
                shutil.rmtree(root)

    def test_a_file_that_cannot_be_read_fails_rather_than_being_skipped(self):
        """A source file this gate cannot read is one it cannot clear."""
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
        root = tree(**{"client/node_modules/p/i.ts": "export function createScene() {}\n",
                       "cmd/vtt/webdist/a.ts": "export function createScene() {}\n",
                       "internal/a/a.go": "package a\n"})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 0, err)
        finally:
            shutil.rmtree(root)

    def test_an_exemption_allows_only_the_words_it_names(self):
        """Per WORD, not per file — and the tree needs none of it today.

        EXEMPT is empty (measured 2026-09-02: zero code positions in the whole
        tree), so this drives the mechanism through `findings`' parameter
        rather than through the module global. The property under test is the
        one that keeps a gate honest: the site allowed to write the identifier
        down is still scanned for every OTHER spelling of it, so the strongest
        enforcement site never becomes the one blind spot.
        """
        allowed = ("const CREATE_SCENE = /createScene/i;\n"
                   "export const R = CREATE_SCENE;\n")
        root = tree(**{"contract/surface.test.ts": allowed})
        exempt = {"contract/surface.test.ts": ({"CREATE_SCENE"}, "the test's own constant")}
        try:
            hits, scanned, unreadable = gate.findings(root, exempt)
            self.assertEqual(hits, [])
            self.assertEqual(scanned, 1)
            self.assertEqual(unreadable, [])
        finally:
            shutil.rmtree(root)

        root = tree(**{"contract/surface.test.ts":
                       allowed + "function createSceneAnyway() {}\n"})
        try:
            hits, _, _ = gate.findings(root, exempt)
            self.assertEqual([w for _, _, w in hits], ["createSceneAnyway"])
        finally:
            shutil.rmtree(root)

    def test_the_shipped_exempt_table_is_empty(self):
        """The measurement, pinned so its erosion is loud.

        Nothing in this tree needs to write the identifier in a code position.
        Adding the first entry is a real decision with prose attached, and this
        assertion is what makes it one rather than a quiet line in a dict.
        """
        self.assertEqual(gate.EXEMPT, {})

    def test_a_tree_with_nothing_to_scan_fails_rather_than_passing(self):
        """Scanning 0 files while exiting 0 is a gate that enforces nothing."""
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


class TheMaskerResumesAfterAClosedConstruct(unittest.TestCase):
    """Every masked construct must END, and nothing else asserted that.

    THE GAP THIS CLOSES. Every "must not fire" test above puts the needle
    INSIDE a comment, a string, a raw string, a docstring or a template
    literal with nothing after it. An over-blanking regression — any one of
    those branches running to end-of-file instead of to its closing delimiter
    — would leave all of them green while blinding the gate across the whole
    rest of the file, silently. That is this repo's own recorded pattern: a
    defensive mechanism tested only by "and nothing bad happens", where the
    mechanism can be switched off entirely and the suite stays green.

    So each case here puts a real code position AFTER a closed construct and
    demands it still be found. Each was proved RED by making its own masker
    branch swallow to end-of-file, one branch at a time, exactly as the
    original fifteen were proved.
    """

    def test_code_after_a_closed_go_raw_string_is_still_scanned(self):
        body = (
            "package store\n"
            "\n"
            "var s = `a create_scene inside a raw string`\n"
            "\n"
            "func createSceneAfterARawString() {}\n"
        )
        root = tree(**{"internal/store/a.go": body})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createSceneAfterARawString", err)
        finally:
            shutil.rmtree(root)

    def test_code_after_a_closed_block_comment_is_still_scanned(self):
        body = (
            "package engine\n"
            "\n"
            "/* create_scene used to be handled here. */\n"
            "\n"
            "func createSceneAfterABlockComment() {}\n"
        )
        root = tree(**{"internal/engine/apply.go": body})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createSceneAfterABlockComment", err)
        finally:
            shutil.rmtree(root)

    def test_code_after_a_closed_python_docstring_is_still_scanned(self):
        body = (
            '"""A docstring mentioning create_scene."""\n'
            "\n"
            "def create_scene_after_a_docstring():\n"
            "    return 1\n"
        )
        root = tree(**{"tools/thing.py": body})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("create_scene_after_a_docstring", err)
        finally:
            shutil.rmtree(root)

    def test_code_after_a_closed_template_literal_is_still_scanned(self):
        body = (
            "const t = `a create_scene inside a template`;\n"
            "export function createSceneAfterATemplate() {}\n"
        )
        root = tree(**{"client/src/view/dm.ts": body})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createSceneAfterATemplate", err)
        finally:
            shutil.rmtree(root)

    def test_code_after_a_closed_regex_literal_is_still_scanned(self):
        """The regex branch GUESSES, so a wrong guess must stay bounded."""
        body = (
            "const r = /createScene/i;\n"
            "export function createSceneAfterARegex() {}\n"
        )
        root = tree(**{"client/test/x.test.ts": body})
        try:
            code, _, err = run(root)
            self.assertEqual(code, 1)
            self.assertIn("createSceneAfterARegex", err)
        finally:
            shutil.rmtree(root)


if __name__ == "__main__":
    unittest.main(verbosity=2)
