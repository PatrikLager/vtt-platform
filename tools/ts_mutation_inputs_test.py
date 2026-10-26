#!/usr/bin/env python3
"""Gate the gate: ts-mutation-inputs.py decides whether the stored client
mutation report is current, and so whether `task check` judges it or says the
client's mutation was not measured (SPEC-023).

Its failure modes are asymmetric and that shapes every test here. Hashing too
MUCH calls a report out of date that still describes the client — a warning
nobody needed. Hashing too LITTLE reports a stale report as current, which is a
gate that stopped covering the client while still printing a verdict. So every
test below asks "does this input move the hash", and the ones that must NOT
move it are named explicitly rather than left to inference.
"""

import importlib.util
import os
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    "tsmi", os.path.join(os.path.dirname(__file__), "ts-mutation-inputs.py"))
tsmi = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tsmi)


def tree(**files):
    """A minimal repo root with the shape inputs_hash expects."""
    root = tempfile.mkdtemp()
    base = {
        "client/src/wire.ts": "export const a = 1\n",
        "client/test/wire.test.ts": "test('a', () => {})\n",
        "stryker.conf.json": '{"mutate":["client/src/**/*.ts"]}\n',
        "package.json": '{"devDependencies":{"@stryker-mutator/core":"8.2.6"}}\n',
        "contract/gen/ts/vtt/v1/events_pb.ts": "export const e = 1\n",
        "contract/testdata/join_url_format.json": '{"a": 1}\n',
        "scenarios/goldens/adventure-night/state.json": '{"s": 1}\n',
        "campaigns/example/art/goblin.png": "png\n",
        "contract/gen/tools/tools.json": '{"tools": []}\n',
        "campaigns/example/maps/cellar.json": '{"cells": []}\n',
    }
    base.update(files)
    for rel, content in base.items():
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as fh:
            fh.write(content)
    return root


class InputsHash(unittest.TestCase):
    def test_the_same_tree_hashes_the_same(self):
        # The control. A hash that moved on its own would call every report out
        # of date, and the client's mutation would never read as measured.
        root = tree()
        self.assertEqual(tsmi.inputs_hash(root), tsmi.inputs_hash(root))

    def test_a_mutated_source_moves_it(self):
        a = tsmi.inputs_hash(tree())
        b = tsmi.inputs_hash(tree(**{"client/src/wire.ts": "export const a = 2\n"}))
        self.assertNotEqual(a, b)

    def test_a_TEST_change_moves_it(self):
        # Tests decide whether a mutant lives, so an edited test invalidates the
        # report exactly as much as an edited source does. Easy to forget,
        # because "mutate" names only client/src.
        a = tsmi.inputs_hash(tree())
        b = tsmi.inputs_hash(tree(**{"client/test/wire.test.ts": "test('b', () => {})\n"}))
        self.assertNotEqual(a, b)

    def test_a_new_source_file_moves_it(self):
        # Deletions and additions must count, not just edits to known files.
        a = tsmi.inputs_hash(tree())
        b = tsmi.inputs_hash(tree(**{"client/src/extra.ts": "export const b = 1\n"}))
        self.assertNotEqual(a, b)

    def test_the_stryker_config_moves_it(self):
        a = tsmi.inputs_hash(tree())
        b = tsmi.inputs_hash(tree(**{"stryker.conf.json": '{"mutate":["client/src/**/*.tsx"]}\n'}))
        self.assertNotEqual(a, b)

    def test_a_stryker_version_bump_moves_it(self):
        # A different mutator emits different mutants; the old report describes
        # a population that no longer exists.
        a = tsmi.inputs_hash(tree())
        b = tsmi.inputs_hash(tree(**{
            "package.json": '{"devDependencies":{"@stryker-mutator/core":"9.0.0"}}\n'}))
        self.assertNotEqual(a, b)

    def test_a_file_that_is_not_an_input_does_NOT_move_it(self):
        # The checker and the adjudication file change the VERDICT, which
        # `task check` recomputes from a current report every run; counting
        # them would call that report out of date for nothing.
        a = tsmi.inputs_hash(tree())
        b = tsmi.inputs_hash(tree(**{
            "tools/check-ts-mutation.py": "# edited\n",
            "tools/ts-mutation-equivalents.txt": "client/src/wire.ts 1:1 X y\n    reason\n",
            "README.md": "# edited\n"}))
        self.assertEqual(a, b)

    def test_a_missing_mutated_tree_is_not_UNCHANGED(self):
        # Fail towards out of date: a client/src that is not there is a lookup
        # that went wrong, not evidence that nothing changed.
        root = tree()
        for f in os.listdir(os.path.join(root, "client/src")):
            os.remove(os.path.join(root, "client/src", f))
        os.rmdir(os.path.join(root, "client/src"))
        self.assertIsNone(tsmi.inputs_hash(root))

    def test_an_unreadable_root_is_not_UNCHANGED(self):
        self.assertIsNone(tsmi.inputs_hash("/nonexistent/root/for/this/test"))

    def test_a_package_json_that_is_not_an_object_still_hashes(self):
        for text in ("[]\n", '{"devDependencies": []}\n'):
            with self.subTest(text=text):
                self.assertIsNotNone(tsmi.inputs_hash(tree(**{"package.json": text})))


class OutsideTheClient(unittest.TestCase):
    """What the client imports, or its tests read by a path written in them,
    from outside client/ (VTT-516)."""

    def moved(self, rel, content):
        return tsmi.inputs_hash(tree()) != tsmi.inputs_hash(tree(**{rel: content}))

    def test_the_generated_contract_moves_it(self):
        # VTT-516
        self.assertTrue(self.moved("contract/gen/ts/vtt/v1/events_pb.ts", "export const e = 2\n"))
        self.assertTrue(self.moved("contract/gen/ts/vtt/v1/commands_pb.ts", "export const c = 1\n"))

    def test_the_contract_testdata_moves_it(self):
        # VTT-516
        self.assertTrue(self.moved("contract/testdata/join_url_format.json", '{"a": 2}\n'))

    def test_a_golden_moves_it(self):
        # VTT-516
        self.assertTrue(self.moved("scenarios/goldens/adventure-night/state.json", '{"s": 2}\n'))

    def test_the_example_art_moves_it(self):
        # VTT-516
        self.assertTrue(self.moved("campaigns/example/art/goblin.png", "png2\n"))
        self.assertTrue(self.moved("campaigns/example/art/goblin.json", '{"g": 1}\n'))

    def test_the_tool_definitions_and_the_example_map_move_it(self):
        # VTT-516
        self.assertTrue(self.moved("contract/gen/tools/tools.json", '{"tools": [1]}\n'))
        self.assertTrue(self.moved("campaigns/example/maps/cellar.json", '{"cells": [1]}\n'))

    def test_a_missing_contract_tree_is_not_UNCHANGED(self):
        root = tree()
        os.remove(os.path.join(root, "contract/gen/ts/vtt/v1/events_pb.ts"))
        for d in ("contract/gen/ts/vtt/v1", "contract/gen/ts/vtt", "contract/gen/ts"):
            os.rmdir(os.path.join(root, d))
        self.assertIsNone(tsmi.inputs_hash(root))

    def test_every_path_the_client_writes_outside_client_is_hashed(self):
        # VTT-516. A path built from pieces is not seen; a template literal
        # opening ${import.meta.dir}/ is.
        import re
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        literal = re.compile(r"""["'`](?:\$\{import\.meta\.dir\}/)?(\.\./[^"'`$\s]+)["'`]""")
        outside = set()
        for top in ("client/src", "client/test"):
            for dirpath, _dirs, names in os.walk(os.path.join(repo, top)):
                for n in names:
                    if not n.endswith(".ts"):
                        continue
                    path = os.path.join(dirpath, n)
                    with open(path, encoding="utf-8") as fh:
                        text = fh.read()
                    for lit in literal.findall(text):
                        rel = os.path.relpath(os.path.normpath(
                            os.path.join(os.path.dirname(path), lit)), repo)
                        if not rel.startswith("client" + os.sep):
                            outside.add((rel.replace(os.sep, "/"), os.path.relpath(path, repo)))
        self.assertTrue(outside, "no path leaves client/, so this test sees nothing")

        def hashed(rel):
            for cand in (rel, rel + ".ts", re.sub(r"\.js$", ".ts", rel)):
                full = os.path.join(repo, cand)
                if os.path.isdir(full):
                    return any(cand == t or cand.startswith(t + "/") for t, _ in tsmi.TREES)
                if os.path.isfile(full):
                    return cand in tsmi.FILES or any(
                        cand.startswith(t + "/") and cand.endswith(sfx) for t, sfx in tsmi.TREES)
            return False

        for rel, where in sorted(outside):
            self.assertTrue(hashed(rel), f"{where} names {rel}, which the hash does not read")


class Wiring(unittest.TestCase):
    def test_the_hashed_trees_match_what_stryker_actually_mutates(self):
        # The one claim this tool makes about the CONFIG rather than about
        # itself: that hashing client/src and client/test covers Stryker's
        # `mutate` globs and its commandRunner. If either moves in
        # stryker.conf.json and this list does not, the gate silently starts
        # trusting a stale report.
        # No comment-stripping: stryker.conf.json carries its commentary in
        # "_comment" string keys, not //, so a re.sub here would be a no-op
        # telling the reader the file has a comment style it does not have.
        import json
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        with open(os.path.join(repo, "stryker.conf.json"), encoding="utf-8") as fh:
            conf = json.load(fh)
        hashed = {t for t, _ in tsmi.TREES}
        for glob in conf["mutate"]:
            top = glob.lstrip("!").split("*")[0].rstrip("/")
            # Boundary, not bare prefix: "client/src2" must NOT read as a
            # child of "client/src". gremlins_args documents the same trap
            # ("so internal/rulesets is not read as a child of internal/rules")
            # and this is the one test whose job is catching silent narrowing.
            self.assertTrue(
                any(top == h or top.startswith(h + "/") for h in hashed),
                f"stryker mutates {glob!r} but ts-mutation-inputs.py does not hash it")
        cmd = conf.get("commandRunner", {}).get("command", "")
        for token in cmd.split():
            if "/" in token:
                self.assertIn(token, hashed,
                              f"the test command runs {token!r} but it is not hashed")

    def test_main_prints_a_hash_and_exits_zero(self):
        # stdout captured: this runs inside `task check`, and a bare 64-char
        # hex line in the gate's output is noise nobody can attribute.
        import contextlib
        import io as _io
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        buf = _io.StringIO()
        with contextlib.redirect_stdout(buf):
            self.assertEqual(tsmi.main(["prog", repo]), 0)
        self.assertEqual(len(buf.getvalue().strip()), 64)

    def test_main_refuses_rather_than_printing_nothing(self):
        # The caller reads a non-zero exit as "out of date". Printing an empty
        # hash with status 0 would read as "unchanged" instead.
        self.assertEqual(tsmi.main(["prog", "/nonexistent/root"]), 1)


if __name__ == "__main__":
    unittest.main(verbosity=2)
