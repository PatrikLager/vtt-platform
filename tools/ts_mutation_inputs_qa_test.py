"""Independent QA for tools/ts-mutation-inputs.py, derived from VTT-516 and D8.

VTT-516: a change to a tracked file that a client source or test imports, or
that a client test reads by a path written in it, moves the report's input
hash. D8 is "What the hash covers" in
docs/superpowers/plans/2026-10-26-the-client-mutation-gate-runs-when-the-owner-asks.md,
and its ticket's "Done looks like" item 5 names three files.

Black-box: the script is run as a subprocess against roots built in temporary
directories. Neither its source nor its own tests were read. Which files the
client imports or reads outside client/ was found by searching client/src and
client/test, and is written down in IMPORTED_OUTSIDE and READ_DIRS below;
outside_paths() re-derives the files read by a path written in a test.

Every change is made at the same size with the old mtime put back, so a
hasher that looked at size or time alone would not pass.
"""

import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
SCRIPT = os.path.join(HERE, "ts-mutation-inputs.py")
HEX = re.compile(r"\A[0-9a-f]{64}\n?\Z")

PRE_TREES = {"client/src": ".ts", "client/test": ".ts"}
PRE_FILES = ["stryker.conf.json", "package.json", "bun.lock", "client/tsconfig.json"]
NEW_TREES = {
    "contract/gen/ts": ".ts",
    "contract/testdata": ".json",
    "scenarios/goldens": ".json",
    "campaigns/example/art": ".png",
}
NEW_FILES = ["contract/gen/tools/tools.json", "campaigns/example/maps/cellar.json"]

# Imported from outside client/ by client/src and client/test.
IMPORTED_OUTSIDE = [
    "contract/gen/ts/vtt/v1/events_pb.ts",
    "contract/gen/ts/vtt/v1/commands_pb.ts",
    "contract/testdata/join_url_format.json",
]
# Files read by a path written in a client test are found by outside_paths()
# below rather than named here, so a renamed fixture is followed, not lost.
# commands.test.ts's fixture(name) joins a name onto contract/testdata, which
# READ_DIRS covers whole.
# Read as a directory by a client test. The suffixes are the ones those tests
# open: fold-parity opens <scenario>/stream.json and state.json, commands opens
# <name>.json, art-assets opens <id>.png and <id>.json.
READ_DIRS = {
    "contract/testdata": (".json",),
    "scenarios/goldens": (".json",),
    "campaigns/example/art": (".png", ".json"),
}
COPY_PATHS = list(PRE_TREES) + list(NEW_TREES) + PRE_FILES + NEW_FILES
LITERAL = re.compile(r"""["'`](?:\$\{import\.meta\.dir\}/)?((?:\.\./)+[^"'`$\s]*)["'`]""")


def walk(root, rel, suffixes=None):
    """The files under root/rel, as root-relative paths, sorted; the working
    tree, not the index, so an untracked or deleted file is seen as it is."""
    out = []
    for dirpath, _dirs, names in os.walk(os.path.join(root, rel)):
        for n in names:
            if suffixes is None or n.endswith(suffixes):
                out.append(os.path.relpath(os.path.join(dirpath, n), root))
    return sorted(out)


def outside_paths():
    """Every quoted path in client/src and client/test that climbs out of
    client/, resolved against the repository; an import gains its .ts."""
    client = os.path.join(REPO, "client")
    found = set()
    for tree in ("client/src", "client/test"):
        for rel in walk(REPO, tree, (".ts",)):
            with open(os.path.join(REPO, rel), encoding="utf8") as f:
                text = f.read()
            for m in LITERAL.finditer(text):
                target = os.path.normpath(
                    os.path.join(REPO, os.path.dirname(rel), m.group(1))
                )
                if os.path.commonpath([target, client]) == client:
                    continue
                if os.path.isfile(target + ".ts"):
                    target += ".ts"
                found.add(os.path.relpath(target, REPO))
    return found


def run(root):
    return subprocess.run(
        [sys.executable, SCRIPT, root], capture_output=True, text=True, timeout=60
    )


def flip(data):
    if not data:
        return b"x"
    i = len(data) // 2
    return data[:i] + bytes([data[i] ^ 0x01]) + data[i + 1 :]


class Base(unittest.TestCase):
    root = ""

    def hash_of(self, root=None):
        res = run(root or self.root)
        self.assertEqual(res.returncode, 0, res.stderr)
        self.assertRegex(res.stdout, HEX)
        return res.stdout.strip()

    def p(self, rel):
        return os.path.join(self.root, rel)

    def write(self, rel, data):
        os.makedirs(os.path.dirname(self.p(rel)), exist_ok=True)
        with open(self.p(rel), "wb") as f:
            f.write(data)

    def moved_by_edit(self, rel, change=flip):
        """Hash after changing rel in place; the file and its mtime come back."""
        path = self.p(rel)
        st = os.stat(path)
        with open(path, "rb") as f:
            orig = f.read()
        try:
            with open(path, "wb") as f:
                f.write(change(orig))
            os.utime(path, ns=(st.st_atime_ns, st.st_mtime_ns))
            return self.hash_of()
        finally:
            with open(path, "wb") as f:
                f.write(orig)
            os.utime(path, ns=(st.st_atime_ns, st.st_mtime_ns))

    def assert_moves(self, rel, change=flip):
        base = self.hash_of()
        self.assertNotEqual(self.moved_by_edit(rel, change), base, rel)
        self.assertEqual(self.hash_of(), base, "restoring %s did not restore the hash" % rel)

    def assert_added_file_keeps(self, rel, data=b"{}\n"):
        base = self.hash_of()
        self.write(rel, data)
        try:
            self.assertEqual(self.hash_of(), base, rel)
        finally:
            os.remove(self.p(rel))


class TheRepositorysOwnInputs(Base):
    """A temporary root holding a copy of every file the hash could cover."""

    @classmethod
    def setUpClass(cls):
        # The working tree, not `git ls-files`: an untracked client test or a
        # deleted fixture is copied as it stands, so the copy hashes as the
        # repository does whatever is staged.
        cls.root = tempfile.mkdtemp(prefix="ts-inputs-qa-")
        for rel in COPY_PATHS:
            src, dst = os.path.join(REPO, rel), os.path.join(cls.root, rel)
            if os.path.isdir(src):
                shutil.copytree(src, dst)
            elif os.path.isfile(src):
                os.makedirs(os.path.dirname(dst), exist_ok=True)
                shutil.copy2(src, dst)
        goldens = [d for d in sorted(os.listdir(os.path.join(cls.root, "scenarios/goldens")))
                   if all(os.path.isfile(os.path.join(cls.root, "scenarios/goldens", d, f))
                          for f in ("stream.json", "state.json"))]
        cls.golden = "scenarios/goldens/" + goldens[0]
        cls.png = walk(cls.root, "campaigns/example/art", (".png",))[0]
        cls.src_ts = walk(cls.root, "client/src", (".ts",))[0]
        cls.test_ts = walk(cls.root, "client/test", (".ts",))[0]

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.root, ignore_errors=True)

    def test_the_copy_hashes_as_the_repository_does(self):
        # VTT-516 (fixture): the copy stands for the repository, so every probe
        # below is a probe of the repository's own inputs.
        self.assertEqual(self.hash_of(), self.hash_of(REPO))
        self.assertEqual(self.hash_of(), self.hash_of())

    def test_item_5s_three_files_each_move_the_hash(self):
        # VTT-516, ticket item 5: events_pb.ts, join_url_format.json and one
        # golden's state.json, changed in place and by an appended byte.
        for rel in (
            "contract/gen/ts/vtt/v1/events_pb.ts",
            "contract/testdata/join_url_format.json",
            "scenarios/goldens/adventure-night/state.json",
        ):
            with self.subTest(rel=rel):
                self.assert_moves(rel)
                self.assert_moves(rel, lambda d: d + b"\n")

    def test_every_outside_file_the_client_imports_moves_the_hash(self):
        # VTT-516: "a tracked file that a client source or test imports".
        for rel in IMPORTED_OUTSIDE:
            with self.subTest(rel=rel):
                self.assert_moves(rel)

    def test_every_outside_file_a_client_test_reads_by_path_moves_the_hash(self):
        # VTT-516: "a client test reads by a path written in it".
        read = sorted(f for f in outside_paths()
                      if os.path.isfile(os.path.join(REPO, f)) and f not in IMPORTED_OUTSIDE)
        self.assertTrue(set(NEW_FILES) <= set(read), read)
        for rel in read:
            with self.subTest(rel=rel):
                self.assert_moves(rel)

    def test_every_file_in_a_directory_a_client_test_reads_moves_the_hash(self):
        # VTT-516, D8 "Directories, because commands.test.ts reads
        # contract/testdata by a name passed in".
        for d, suffixes in READ_DIRS.items():
            files = walk(self.root, d, suffixes)
            self.assertTrue(files, d)
            for rel in files:
                with self.subTest(rel=rel):
                    self.assert_moves(rel)

    def test_every_outside_path_literal_in_the_client_lands_where_a_change_moves_the_hash(self):
        # VTT-516, re-derived: every quoted path in client/src and client/test
        # that climbs out of client/ names a file, or a directory whose read
        # files, a change to which moves the hash.
        found = outside_paths()
        self.assertTrue(set(IMPORTED_OUTSIDE) <= found, sorted(found))
        self.assertIn("scenarios/goldens", found)
        for rel in sorted(found):
            if os.path.isdir(os.path.join(REPO, rel)):
                files = walk(self.root, rel, (".json", ".png"))
            else:
                files = [rel]
            for f in files:
                with self.subTest(literal=rel, file=f):
                    self.assert_moves(f)

    def test_a_new_scenario_moves_the_hash(self):
        # VTT-516, D8: fold-parity lists scenarios/goldens and folds each one.
        base = self.hash_of()
        self.write("scenarios/goldens/qa-new/stream.json", b"[]\n")
        self.write("scenarios/goldens/qa-new/state.json", b"{}\n")
        try:
            self.assertNotEqual(self.hash_of(), base)
        finally:
            shutil.rmtree(self.p("scenarios/goldens/qa-new"))
        self.assertEqual(self.hash_of(), base)

    def test_a_new_contract_fixture_moves_the_hash(self):
        # VTT-516, D8: contract/testdata is read by a name passed in.
        base = self.hash_of()
        self.write("contract/testdata/qa_new_command.json", b"{}\n")
        try:
            self.assertNotEqual(self.hash_of(), base)
        finally:
            os.remove(self.p("contract/testdata/qa_new_command.json"))

    def test_renaming_a_file_a_test_reads_by_name_moves_the_hash(self):
        # VTT-516: art-assets asks for <id>.png, so a rename changes what it reads.
        base = self.hash_of()
        a = self.p(self.png)
        b = a[: -len(".png")] + "-2.png"
        os.rename(a, b)
        try:
            self.assertNotEqual(self.hash_of(), base)
        finally:
            os.rename(b, a)
        self.assertEqual(self.hash_of(), base)

    def test_swapping_two_read_files_contents_moves_the_hash(self):
        # VTT-516: fold-parity compares stream.json's fold with state.json.
        base = self.hash_of()
        sa, sb = self.p(self.golden + "/stream.json"), self.p(self.golden + "/state.json")
        with open(sa, "rb") as f:
            da = f.read()
        with open(sb, "rb") as f:
            db = f.read()
        try:
            for path, data in ((sa, db), (sb, da)):
                with open(path, "wb") as f:
                    f.write(data)
            self.assertNotEqual(self.hash_of(), base)
        finally:
            for path, data in ((sa, da), (sb, db)):
                with open(path, "wb") as f:
                    f.write(data)

    def test_deleting_a_read_file_never_leaves_the_hash_in_place(self):
        # VTT-516: deletion is a change; either the hash moves or the script
        # refuses, and a refusal prints nothing on stdout.
        base = self.hash_of()
        for rel in NEW_FILES + ["contract/gen/ts/vtt/v1/events_pb.ts",
                                self.golden + "/state.json"]:
            with self.subTest(rel=rel):
                path = self.p(rel)
                with open(path, "rb") as f:
                    orig = f.read()
                os.remove(path)
                try:
                    res = run(self.root)
                    if res.returncode == 0:
                        self.assertRegex(res.stdout, HEX)
                        self.assertNotEqual(res.stdout.strip(), base)
                    else:
                        self.assertEqual(res.returncode, 1)
                        self.assertEqual(res.stdout, "")
                finally:
                    with open(path, "wb") as f:
                        f.write(orig)
        self.assertEqual(self.hash_of(), base)

    def test_the_inputs_hashed_before_the_task_still_move_the_hash(self):
        # D8: TREES and FILES "gain"; nothing hashed before is dropped.
        present = [f for f in PRE_FILES if os.path.isfile(self.p(f))]
        self.assertTrue(present, "none of the files hashed before the task is here")
        for rel in [self.src_ts, self.test_ts] + present:
            with self.subTest(rel=rel):
                self.assert_moves(rel)
        base = self.hash_of()
        self.write("bun.lockb", b"\0")
        try:
            self.assertNotEqual(self.hash_of(), base)
        finally:
            os.remove(self.p("bun.lockb"))

    def test_a_finder_ds_store_does_not_move_the_hash(self):
        # D8: "All files would hash a Finder .DS_Store, which is gitignored".
        for d in NEW_TREES:
            with self.subTest(tree=d):
                self.assert_added_file_keeps(d + "/.DS_Store", b"\0\0\0\1Bud1")

    def test_a_suffix_its_tree_does_not_name_does_not_move_the_hash(self):
        # D8: each tree is hashed for its own suffixes only.
        for rel in (
            "contract/gen/ts/vtt/v1/qa.json",
            "contract/testdata/qa.ts",
            "contract/testdata/qa.txt",
            "scenarios/goldens/qa.png",
            self.golden + "/qa.ts",
            "campaigns/example/art/qa.ts",
            "campaigns/example/art/qa.txt",
        ):
            with self.subTest(rel=rel):
                self.assert_added_file_keeps(rel)
        base = self.hash_of()
        self.assertEqual(self.moved_by_edit("scenarios/goldens/README.md"), base)

    def test_a_sibling_of_a_hashed_file_does_not_move_the_hash(self):
        # D8: FILES gains two files, not their directories.
        for rel in ("campaigns/example/maps/qa.json", "contract/gen/tools/qa.json"):
            with self.subTest(rel=rel):
                self.assert_added_file_keeps(rel)


class AMinimalRoot(Base):
    """A root holding one file in every hashed tree and every hashed file."""

    def setUp(self):
        self.root = tempfile.mkdtemp(prefix="ts-inputs-qa-min-")
        for tree, suffix in {**PRE_TREES, **NEW_TREES}.items():
            self.write(tree + "/a" + suffix, b"a")
        for rel in PRE_FILES + NEW_FILES:
            self.write(rel, b"{}")

    def tearDown(self):
        shutil.rmtree(self.root, ignore_errors=True)

    def test_every_tree_must_exist(self):
        # D8: "Every tree must exist, as today": exit 1, nothing on stdout.
        self.hash_of()
        for tree in list(PRE_TREES) + list(NEW_TREES):
            with self.subTest(tree=tree):
                moved = self.p(tree) + ".away"
                os.rename(self.p(tree), moved)
                try:
                    res = run(self.root)
                    self.assertEqual(res.returncode, 1)
                    self.assertEqual(res.stdout, "")
                finally:
                    os.rename(moved, self.p(tree))
        self.hash_of()

    def test_the_art_hashes_both_its_suffixes(self):
        # D8: campaigns/example/art is hashed for .png and .json.
        base = self.hash_of()
        self.write("campaigns/example/art/a.json", b"{}")
        self.assertNotEqual(self.hash_of(), base)
        self.assert_moves("campaigns/example/art/a.json")
        self.assert_moves("campaigns/example/art/a.png")

    def test_a_nested_file_in_each_new_tree_moves_the_hash(self):
        # VTT-516, D8: the reads go below the tree's top (vtt/v1/, <scenario>/).
        for tree, suffix in NEW_TREES.items():
            with self.subTest(tree=tree):
                rel = tree + "/deep/er/b" + suffix
                base = self.hash_of()
                self.write(rel, b"b")
                self.assertNotEqual(self.hash_of(), base)
                self.assert_moves(rel)

    def test_bytes_moved_across_a_file_boundary_move_the_hash(self):
        # VTT-516: both files changed, though their concatenation did not.
        self.write("contract/testdata/a.json", b"ab")
        self.write("contract/testdata/b.json", b"c")
        before = self.hash_of()
        self.write("contract/testdata/a.json", b"a")
        self.write("contract/testdata/b.json", b"bc")
        self.assertNotEqual(self.hash_of(), before)

    def test_a_file_moved_from_one_tree_to_another_moves_the_hash(self):
        # VTT-516: the same bytes under another path are another input.
        self.write("contract/testdata/x.json", b"{}")
        before = self.hash_of()
        os.remove(self.p("contract/testdata/x.json"))
        self.write("scenarios/goldens/x.json", b"{}")
        self.assertNotEqual(self.hash_of(), before)


if __name__ == "__main__":
    unittest.main()
