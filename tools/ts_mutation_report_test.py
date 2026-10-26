#!/usr/bin/env python3
"""Gate the gate: ts-mutation-report.py decides what check:ts-mutation does.

The owner's rule of 2026-10-26 (SPEC-023): `task check` never starts Stryker.
With the stored report current, the step judges it; out of date or missing, it
says the client's mutation was not measured, names the commit the report
measured and the tickets added since, and passes. The full run, `task
check:ts-mutation:full`, is the only thing that writes the stamp.

Every test runs on a temporary git repository, and nothing here may start
Stryker or its container: the command-line tests put stand-ins first on PATH
and assert that none of them was called, and the Taskfile is read with
`task --dry`, which runs none of `check`'s commands.
"""

import importlib.util
import io
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
SCRIPT = os.path.join(HERE, "ts-mutation-report.py")


def _load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


tsmr = _load("tsmr", SCRIPT)
tsmi = _load("tsmi", os.path.join(HERE, "ts-mutation-inputs.py"))

# A fixture commit must run no hook and no signing of this machine.
GIT = ["git", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false",
       "-c", "user.name=t", "-c", "user.email=t@t"]
NOT_MEASURED = "THE CLIENT'S MUTATION WAS NOT MEASURED"
STAND_INS = ("docker", "npx", "bunx", "stryker", "task")


def git(root, *args):
    return subprocess.run(GIT + ["-C", root, *args], capture_output=True,
                          text=True, check=True).stdout.strip()


def write(root, rel, content):
    path = os.path.join(root, rel)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(content)
    return path


def fixture(test):
    """A repository committed at one commit, holding a file in every tree the
    hasher reads, so it hashes whatever TREES and FILES hold."""
    root = tempfile.mkdtemp()
    test.addCleanup(shutil.rmtree, root, True)
    files = {
        "stryker.conf.json": '{"mutate":["client/src/**/*.ts"]}\n',
        "package.json": '{"devDependencies":{"@stryker-mutator/core":"9.6.1"}}\n',
        "client/src/style.css": "a {}\n",
        "docs/superpowers/specs/2026-01-01-old-design.md": "old\n",
        ".gitignore": "reports/\n",
    }
    for tree, suffixes in tsmi.TREES:
        files[f"{tree}/one{suffixes[0]}"] = "1\n"
    for f in tsmi.FILES:
        files.setdefault(f, "{}\n")
    for rel, content in files.items():
        write(root, rel, content)
    git(root, "init", "-q", "-b", "main")
    git(root, "add", "-A")
    git(root, "commit", "-q", "-m", "A")
    return root


def head(root):
    return git(root, "rev-parse", "HEAD")


def report(root, after):
    """A stored report whose mtime is `after` plus a second."""
    path = write(root, tsmr.REPORT, '{"files": {}}\n')
    os.utime(path, (after + 1, after + 1))
    return path


def measured(root):
    """The report and the stamp a full run at HEAD leaves behind."""
    snap = tsmr.snapshot(root)
    report(root, snap["started"])
    if tsmr.stamp(root, json.dumps(snap)) != 0:
        raise AssertionError("the fixture's stamp was refused")
    return snap


def write_stamp(root, obj):
    write(root, tsmr.STAMP, obj if isinstance(obj, str) else json.dumps(obj))


class Runner:
    """Stands in for the checker's process and records each call."""

    def __init__(self, code=0):
        self.code = code
        self.calls = []

    def __call__(self, cmd, cwd):
        self.calls.append((cmd, cwd))
        return self.code


def check(root, runner):
    out = io.StringIO()
    code = tsmr.check(root, run=runner, out=out)
    return code, out.getvalue()


class TheStep(unittest.TestCase):
    """`check`, the step `task check` runs (VTT-507 to VTT-511)."""

    def test_an_out_of_date_report_is_not_judged_and_names_its_commit_and_the_tickets_since(self):
        # VTT-508 VTT-509
        root = fixture(self)
        a = head(root)
        measured(root)
        write(root, "client/src/one.ts", "2\n")
        write(root, "docs/superpowers/specs/2026-10-26-b-design.md", "b\n")
        git(root, "add", "-A")
        git(root, "commit", "-q", "-m", "B")
        write(root, "docs/superpowers/specs/2026-10-27-c-design.md", "c\n")
        runner = Runner()
        code, out = check(root, runner)
        self.assertEqual(code, 0, out)
        self.assertIn(NOT_MEASURED, out)
        self.assertIn(git(root, "rev-parse", "--short=12", a), out)
        self.assertIn("2026-10-26-b-design.md", out)
        self.assertIn("2026-10-27-c-design.md", out)
        self.assertNotIn("2026-01-01-old-design.md", out)
        self.assertEqual(runner.calls, [])

    def test_a_stamp_with_no_commit_or_no_json_says_the_tickets_cannot_be_listed(self):
        # VTT-508
        for stamp in ({"inputs": "0" * 64, "clean": True}, "not json"):
            with self.subTest(stamp=stamp):
                root = fixture(self)
                report(root, 0)
                write_stamp(root, stamp)
                runner = Runner()
                code, out = check(root, runner)
                self.assertEqual(code, 0, out)
                self.assertIn(NOT_MEASURED, out)
                self.assertIn("cannot be listed", out)
                self.assertEqual(runner.calls, [])

    def test_a_stamp_naming_a_commit_the_repository_lacks_says_the_tickets_cannot_be_listed(self):
        # VTT-508
        root = fixture(self)
        report(root, 0)
        write_stamp(root, {"inputs": "0" * 64, "commit": "f" * 40, "clean": True})
        code, out = check(root, Runner())
        self.assertEqual(code, 0, out)
        self.assertIn(NOT_MEASURED, out)
        self.assertIn("cannot be listed", out)

    def test_a_commit_on_another_branch_is_named_as_such_and_its_tickets_are_a_tree_difference(self):
        # VTT-509
        root = fixture(self)
        git(root, "checkout", "-q", "-b", "side")
        write(root, "docs/superpowers/specs/2026-10-20-side-design.md", "s\n")
        git(root, "add", "-A")
        git(root, "commit", "-q", "-m", "X")
        x = head(root)
        measured(root)
        git(root, "checkout", "-q", "main")
        write(root, "docs/superpowers/specs/2026-10-26-b-design.md", "b\n")
        write(root, "client/src/one.ts", "2\n")
        git(root, "add", "-A")
        git(root, "commit", "-q", "-m", "B")
        code, out = check(root, Runner())
        self.assertEqual(code, 0, out)
        self.assertIn(git(root, "rev-parse", "--short=12", x), out)
        self.assertIn("not an ancestor", out)
        self.assertIn("2026-10-26-b-design.md", out)
        self.assertNotIn("2026-10-20-side-design.md", out)

    def test_a_stamp_says_how_clean_the_measured_tree_was(self):
        for clean, note in ((False, tsmr.CLEAN_NOTES[False]), (None, tsmr.CLEAN_NOTES[None])):
            with self.subTest(clean=clean):
                root = fixture(self)
                report(root, 0)
                write_stamp(root, {"inputs": "0" * 64, "commit": head(root), "clean": clean})
                code, out = check(root, Runner())
                self.assertEqual(code, 0, out)
                self.assertIn(note, out)

    def test_no_stored_report_passes_and_says_there_is_none(self):
        # VTT-510
        root = fixture(self)
        runner = Runner()
        code, out = check(root, runner)
        self.assertEqual(code, 0, out)
        self.assertIn(NOT_MEASURED, out)
        self.assertIn("no stored report", out)
        self.assertEqual(runner.calls, [])

    def test_a_current_report_is_judged_and_the_step_exits_with_the_verdict(self):
        # VTT-511
        for verdict in (0, 1):
            with self.subTest(verdict=verdict):
                root = fixture(self)
                measured(root)
                runner = Runner(verdict)
                code, out = check(root, runner)
                self.assertEqual(code, verdict, out)
                self.assertIn("current", out)
                self.assertIn(git(root, "rev-parse", "--short=12", "HEAD"), out)
                self.assertEqual(len(runner.calls), 1)
                cmd, cwd = runner.calls[0]
                self.assertEqual(cmd[-2:], [
                    os.path.join(root, "tools", "check-ts-mutation.py"),
                    os.path.join(root, "tools", "ts-mutation-equivalents.txt"),
                ])
                self.assertEqual(cwd, root)

    def test_a_stamp_whose_fields_are_not_of_their_kind_reads_as_unreadable(self):
        # VTT-508
        for bad in ({"commit": 5}, {"clean": []}, {"inputs": None}):
            with self.subTest(bad=bad):
                root = fixture(self)
                measured(root)
                with open(os.path.join(root, tsmr.STAMP)) as fh:
                    good = json.load(fh)
                write_stamp(root, dict(good, **bad))
                runner = Runner()
                code, out = check(root, runner)
                self.assertEqual(code, 0, out)
                self.assertIn(NOT_MEASURED, out)
                self.assertIn("cannot be read", out)
                self.assertEqual(runner.calls, [])

    def test_a_stamp_written_for_a_report_no_snapshot_preceded_reads_current(self):
        # VTT-511
        root = fixture(self)
        report(root, 0)
        write_stamp(root, {"inputs": tsmi.inputs_hash(root), "commit": head(root),
                           "clean": None})
        runner = Runner()
        code, out = check(root, runner)
        self.assertEqual(code, 0, out)
        self.assertIn(tsmr.CLEAN_NOTES[None], out)
        self.assertEqual(len(runner.calls), 1)

    def test_an_unhashable_tree_is_out_of_date_however_the_stamp_reads(self):
        # VTT-508
        root = fixture(self)
        measured(root)
        shutil.rmtree(os.path.join(root, "client", "src"))
        runner = Runner()
        code, out = check(root, runner)
        self.assertEqual(code, 0, out)
        self.assertIn(NOT_MEASURED, out)
        self.assertEqual(runner.calls, [])


def stand_ins(test):
    """A PATH whose first directory holds a logging stand-in for every program
    that could start Stryker or its container, and the log they write to."""
    bindir = tempfile.mkdtemp()
    test.addCleanup(shutil.rmtree, bindir, True)
    log = os.path.join(bindir, "called.log")
    for name in STAND_INS:
        path = os.path.join(bindir, name)
        with open(path, "w") as fh:
            fh.write(f'#!/bin/sh\necho {name} >> "{log}"\nexit 0\n')
        os.chmod(path, 0o755)
    return bindir + os.pathsep + os.environ.get("PATH", ""), log


def cli(root, *args, path=None):
    env = dict(os.environ)
    if path is not None:
        env["PATH"] = path
    return subprocess.run([sys.executable, SCRIPT, "--root", root, *args],
                          capture_output=True, text=True, env=env, cwd=root)


class TheCommandLine(unittest.TestCase):
    """The script as `task check` runs it (VTT-507)."""

    def test_out_of_date_and_missing_reports_start_nothing(self):
        # VTT-507 VTT-508 VTT-510
        stale = fixture(self)
        measured(stale)
        write(stale, "client/src/one.ts", "2\n")
        missing = fixture(self)
        for root, words in ((stale, "out of date"), (missing, "no stored report")):
            with self.subTest(words=words):
                path, log = stand_ins(self)
                done = cli(root, "check", path=path)
                self.assertEqual(done.returncode, 0, done.stdout + done.stderr)
                self.assertIn(NOT_MEASURED, done.stdout)
                self.assertIn(words, done.stdout)
                self.assertFalse(os.path.exists(log), "a stand-in was called")


class TheFullRunsStamp(unittest.TestCase):
    """`snapshot` and `stamp`, which `check:ts-mutation:full` runs around
    Stryker (VTT-512 to VTT-515)."""

    def test_the_stamp_names_the_commit_checked_out_when_the_run_began(self):
        # VTT-512
        root = fixture(self)
        a = head(root)
        snap = tsmr.snapshot(root)
        write(root, "docs/x.md", "x\n")
        git(root, "add", "-A")
        git(root, "commit", "-q", "-m", "B")
        report(root, snap["started"])
        self.assertEqual(tsmr.stamp(root, json.dumps(snap)), 0)
        with open(os.path.join(root, tsmr.STAMP)) as fh:
            self.assertEqual(json.load(fh)["commit"], a)

    def test_the_stamp_says_whether_a_file_the_hash_reads_differed_from_the_commit(self):
        # VTT-513
        root = fixture(self)
        self.assertIs(tsmr.snapshot(root)["clean"], True)
        write(root, "client/src/style.css", "b {}\n")
        self.assertIs(tsmr.snapshot(root)["clean"], True, "style.css is not hashed")
        write(root, "client/src/new.ts", "3\n")
        self.assertIs(tsmr.snapshot(root)["clean"], False)

    def test_a_hashed_file_renamed_out_of_the_hash_in_the_worktree_is_not_clean(self):
        # VTT-513
        root = fixture(self)
        os.rename(os.path.join(root, "client/src/one.ts"),
                  os.path.join(root, "client/src/one.css"))
        git(root, "add", "-N", "client/src/one.css")
        self.assertIs(tsmr.snapshot(root)["clean"], False)

    def test_no_report_newer_than_the_run_writes_no_stamp(self):
        # VTT-514
        root = fixture(self)
        snap = tsmr.snapshot(root)
        write_stamp(root, {"inputs": "earlier", "commit": "earlier", "clean": True})
        with open(os.path.join(root, tsmr.STAMP)) as fh:
            before = fh.read()
        self.assertEqual(tsmr.stamp(root, json.dumps(snap)), 1)
        report(root, snap["started"] - 10)
        self.assertEqual(tsmr.stamp(root, json.dumps(snap)), 1)
        with open(os.path.join(root, tsmr.STAMP)) as fh:
            self.assertEqual(fh.read(), before)
        report(root, snap["started"])
        self.assertEqual(tsmr.stamp(root, json.dumps(snap)), 0)
        with open(os.path.join(root, tsmr.STAMP)) as fh:
            self.assertEqual(json.load(fh)["commit"], head(root))

    def test_a_snapshot_whose_fields_are_not_of_their_kind_writes_no_stamp(self):
        root = fixture(self)
        snap = tsmr.snapshot(root)
        report(root, snap["started"])
        for bad in ({"clean": "yes"}, {"commit": None}, {"inputs": ""},
                    {"started": float("nan")}, {"started": True},
                    {"started": 10 ** 400}):
            with self.subTest(bad=bad):
                code = tsmr.stamp(root, json.dumps(dict(snap, **bad)), err=io.StringIO())
                self.assertEqual(code, 1)
                self.assertFalse(os.path.exists(os.path.join(root, tsmr.STAMP)))

    def test_a_full_runs_report_is_current_and_judged_whatever_its_verdict(self):
        # VTT-515
        root = fixture(self)
        snap = tsmr.snapshot(root)
        report(root, snap["started"])
        path, log = stand_ins(self)
        self.assertEqual(cli(root, "stamp", json.dumps(snap), path=path).returncode, 0)
        write(root, "tools/check-ts-mutation.py",
              "import sys\nprint('stand-in checker: one survivor')\nsys.exit(1)\n")
        done = cli(root, "check", path=path)
        self.assertEqual(done.returncode, 1, done.stdout + done.stderr)
        self.assertIn("stand-in checker: one survivor", done.stdout)
        self.assertIn(git(root, "rev-parse", "--short=12", "HEAD"), done.stdout)
        self.assertFalse(os.path.exists(log), "a stand-in was called")

    def test_a_snapshot_of_an_unhashable_tree_or_outside_a_repository_prints_nothing(self):
        root = fixture(self)
        shutil.rmtree(os.path.join(root, "client", "src"))
        bare = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, bare, True)
        for rel in ("client/src/one.ts", "client/test/one.ts"):
            write(bare, rel, "1\n")
        for where in (root, bare):
            with self.subTest(where=where):
                path, log = stand_ins(self)
                done = cli(where, "snapshot", path=path)
                self.assertEqual(done.returncode, 1)
                self.assertEqual(done.stdout, "")
                self.assertFalse(os.path.exists(log), "a stand-in was called")


def dry(target):
    """What `task --dry <target>` prints in the repository; it runs nothing."""
    if shutil.which("task") is None:
        raise AssertionError("task is not on PATH; the Taskfile cannot be read")
    done = subprocess.run(["task", "--dry", target], capture_output=True,
                          text=True, cwd=REPO)
    if done.returncode != 0:
        raise AssertionError(f"task --dry {target} failed:\n{done.stderr}")
    return done.stdout + done.stderr


class TheTaskfile(unittest.TestCase):
    """How `Taskfile.yml` wires the two (VTT-507, VTT-515)."""

    def test_task_check_starts_no_stryker_run_and_no_container(self):
        # VTT-507
        out = dry("check")
        for banned in ("stryker run", "docker run", "check:ts-mutation:docker",
                       "check:ts-mutation:full"):
            self.assertNotIn(banned, out)
        step = [l for l in out.splitlines() if "[check:ts-mutation]" in l]
        self.assertTrue(any("ts-mutation-report.py check" in l for l in step), step)

    def test_the_full_run_stamps_after_stryker_and_before_it_judges(self):
        # VTT-515
        out = dry("check:ts-mutation:full")
        snap = out.index("ts-mutation-report.py snapshot")
        stamp = out.index("ts-mutation-report.py stamp")
        judge = out.rindex("check-ts-mutation.py")
        runs = [i for word in ("stryker run", "docker run")
                for i in range(len(out)) if out.startswith(word, i)]
        self.assertTrue(runs, "the full run starts no Stryker")
        self.assertTrue(all(snap < i < stamp for i in runs), out)
        self.assertLess(stamp, judge)


if __name__ == "__main__":
    unittest.main()
