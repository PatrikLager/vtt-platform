#!/usr/bin/env python3
"""Independent QA of the client mutation step and its full run.

Derived from VTT-507..VTT-515 and from the plan's decisions D2-D7 and D11
for the ticket docs/superpowers/specs/2026-10-26-the-client-mutation-gate-
runs-when-the-owner-asks-design.md ("Done looks like" items 1 to 4). It
was written without reading tools/ts-mutation-report.py, its own tests,
tools/ts-mutation-inputs.py or Taskfile.yml: it drives the script by its
command line against temporary git repositories, the hasher by its
command line, and the Taskfile through `task --dry` only.

WARNING: no test here may start Stryker, npx, bunx, docker or a real
`task` run. Every script run puts logging stand-ins for them first on
PATH and fails if one is called. The full-run tests execute the block
`task --dry check:ts-mutation:full` prints, with Stryker and the container
replaced by stand-ins.
"""

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import unittest

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TOOLS = os.path.join(REPO, "tools")
SCRIPT = os.path.join(TOOLS, "ts-mutation-report.py")
HASHER = os.path.join(TOOLS, "ts-mutation-inputs.py")
REAL_CHECKER = os.path.join(TOOLS, "check-ts-mutation.py")
PY = sys.executable

GIT = ["git", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false",
       "-c", "user.name=t", "-c", "user.email=t@t"]

NOT_MEASURED = "THE CLIENT'S MUTATION WAS NOT MEASURED"
FULL = "task check:ts-mutation:full"
REPORT = "reports/mutation/mutation.json"
STAMP = "reports/mutation/ts-mutation-stamp.json"
LEGACY = "reports/mutation/ts-inputs.sha256"
SPECS = "docs/superpowers/specs/"

FORBIDDEN = ("docker", "npx", "bunx", "stryker", "task", "bun", "node", "npm")

A_TS = "export const a = true;\n"

# Every tree and file the hasher reads today or after the next task, so
# that a fixture stays hashable when the next task adds trees.
FILES = {
    "client/src/a.ts": A_TS,
    "client/src/style.css": "a { color: red; }\n",
    "client/test/a.test.ts": "import './x';\n",
    "stryker.conf.json": "{}\n",
    "package.json": "{}\n",
    "bun.lock": "lock\n",
    "client/tsconfig.json": "{}\n",
    "contract/gen/ts/vtt/v1/events_pb.ts": "export {};\n",
    "contract/testdata/join_url_format.json": "{}\n",
    "scenarios/goldens/adventure-night/state.json": "{}\n",
    "scenarios/goldens/README.md": "# goldens\n",
    "campaigns/example/art/a.png": "PNG",
    "campaigns/example/art/a.json": "{}\n",
    "contract/gen/tools/tools.json": "{}\n",
    "campaigns/example/maps/cellar.json": "{}\n",
    "tools/ts-mutation-equivalents.txt": "",
    ".gitignore": "reports/\n",
}

STANDIN_CHECKER = '''import json, os, sys
with open({log!r}, "a") as f:
    f.write(json.dumps({{"argv": sys.argv, "cwd": os.getcwd()}}) + "\\n")
if __name__ == "__main__":
    with open({verdict!r}) as f:
        sys.exit(int(f.read().strip() or "0"))
'''

FORBIDDEN_STUB = '''#!/bin/sh
printf '%s\\n' "{name} $*" >> {log}
exit 0
'''

# Runs the script with every process it starts recorded, so a test can
# say which programs a path started (D4).
TRACER = r'''
import json, os, runpy, subprocess, sys, threading
LOG = os.environ["QA_SPAWN_LOG"]
_inside = threading.local()

def _rec(kind, argv):
    argv = [os.fsdecode(a) if isinstance(a, bytes) else str(a)
            for a in argv]
    with open(LOG, "a") as f:
        f.write(json.dumps({"kind": kind, "argv": argv}) + "\n")

_init = subprocess.Popen.__init__

def _popen(self, *a, **k):
    args = a[0] if a else k.get("args")
    if isinstance(args, (str, bytes, os.PathLike)):
        argv = [args]
    else:
        argv = list(args)
    _rec("shell" if k.get("shell") else "popen", argv)
    _inside.on = True
    try:
        return _init(self, *a, **k)
    finally:
        _inside.on = False

subprocess.Popen.__init__ = _popen

def _wrap(name, pick):
    orig = getattr(os, name, None)
    if orig is None:
        return
    def wrapped(*a, **k):
        if not getattr(_inside, "on", False):
            _rec(name, pick(a))
        return orig(*a, **k)
    setattr(os, name, wrapped)

for _n in ("system",):
    _wrap(_n, lambda a: [a[0]])
for _n in ("posix_spawn", "posix_spawnp", "execv", "execve", "execvp",
           "execvpe"):
    _wrap(_n, lambda a: list(a[1]) if len(a) > 1 else [a[0]])
for _n in ("spawnv", "spawnve", "spawnvp", "spawnvpe"):
    _wrap(_n, lambda a: list(a[2]) if len(a) > 2 else [a[1]])
for _n in ("fork", "forkpty"):
    _wrap(_n, lambda a: ["<fork>"])

script = sys.argv[1]
sys.argv = sys.argv[1:]
sys.path[0] = os.path.dirname(os.path.abspath(script))
runpy.run_path(script, run_name="__main__")
'''


def clean_env():
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.pop("VTT_FORCE_TS_MUTATION", None)
    return env


def write(path, content, mode="w"):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, mode) as f:
        f.write(content)


def read(path):
    with open(path) as f:
        return f.read()


def token(text, word):
    """True when word appears not glued to another hex digit."""
    return re.search(r"(?<![0-9a-fA-F])" + re.escape(word) +
                     r"(?![0-9a-fA-F])", text) is not None


class Result(object):
    def __init__(self, rc, out, err, spawns):
        self.rc = rc
        self.out = out
        self.err = err
        self.text = out + err
        self.spawns = spawns

    def programs(self):
        names = []
        for s in self.spawns:
            if s["kind"] in ("shell", "system"):
                names.append("<shell:%s>" % s["argv"][0])
            elif s["kind"] == "fork":
                names.append("<fork>")
            else:
                names.append(os.path.basename(s["argv"][0]))
        return names

    def first_line_with(self, phrase):
        for stream in (self.out, self.err):
            lines = [l for l in stream.splitlines() if l.strip()]
            if lines and phrase in lines[0]:
                return True
        return False


class FixtureCase(unittest.TestCase):
    """A temporary git repository with every hashed tree, a stand-in
    checker at tools/check-ts-mutation.py and logging stand-ins first on
    PATH."""

    def setUp(self):
        self.aux = os.path.realpath(tempfile.mkdtemp(prefix="tsmr-qa-"))
        self.addCleanup(shutil.rmtree, self.aux, ignore_errors=True)
        self.root = os.path.join(self.aux, "repo")
        self.bin = os.path.join(self.aux, "bin")
        self.forbidden_log = os.path.join(self.aux, "forbidden.log")
        self.checker_log = os.path.join(self.aux, "checker.log")
        self.verdict_file = os.path.join(self.aux, "verdict")
        self.spawn_log = os.path.join(self.aux, "spawn.log")
        self.tracer = os.path.join(self.aux, "tracer.py")
        os.makedirs(self.root)
        os.makedirs(self.bin)
        write(self.forbidden_log, "")
        write(self.checker_log, "")
        write(self.verdict_file, "0")
        write(self.tracer, TRACER)
        for name in FORBIDDEN:
            p = os.path.join(self.bin, name)
            write(p, FORBIDDEN_STUB.format(name=name, log=self.forbidden_log))
            os.chmod(p, 0o755)
        self.git("init", "-q")
        self.git("checkout", "-q", "-b", "main")
        for rel, content in FILES.items():
            write(self.path(rel), content)
        self.install_standin_checker()
        write(self.path(SPECS + "2026-01-01-one-design.md"), "one\n")
        write(self.path(SPECS + "2026-01-02-two-design.md"), "two\n")
        self.first = self.commit("first")

    # -- fixture helpers ------------------------------------------------

    def path(self, rel):
        return os.path.join(self.root, rel)

    def git(self, *args, check=True):
        p = subprocess.run(GIT + list(args), cwd=self.root, env=clean_env(),
                           capture_output=True, text=True)
        if check and p.returncode != 0:
            raise AssertionError("git %s failed: %s" % (args, p.stderr))
        return p.stdout.strip()

    def commit(self, msg):
        self.git("add", "-A")
        self.git("commit", "-q", "-m", msg)
        return self.git("rev-parse", "HEAD")

    def head(self):
        return self.git("rev-parse", "HEAD")

    def short(self, commit):
        return self.git("rev-parse", "--short=12", commit)

    def install_standin_checker(self):
        write(self.path("tools/check-ts-mutation.py"),
              STANDIN_CHECKER.format(log=self.checker_log,
                                     verdict=self.verdict_file))

    def set_verdict(self, n):
        write(self.verdict_file, str(n))

    def checker_calls(self):
        return [json.loads(l) for l in read(self.checker_log).splitlines()
                if l.strip()]

    def tree_hash(self, root=None):
        p = subprocess.run([PY, HASHER, root or self.root],
                           capture_output=True, text=True, env=clean_env())
        if p.returncode != 0:
            return None
        return p.stdout.strip()

    def write_report(self, mtime=None, content="{}"):
        write(self.path(REPORT), content)
        if mtime is not None:
            os.utime(self.path(REPORT), (mtime, mtime))

    def write_stamp(self, raw=None, **fields):
        if raw is None:
            stamp = {"inputs": self.tree_hash(), "commit": self.head(),
                     "clean": True, "started": time.time() - 1000}
            stamp.update(fields)
            for k in [k for k, v in fields.items() if v is DROP]:
                del stamp[k]
            raw = json.dumps(stamp)
        write(self.path(STAMP), raw)

    # -- running the script ---------------------------------------------

    def run_script(self, *args, root=None):
        write(self.spawn_log, "")
        env = clean_env()
        env["PATH"] = self.bin + os.pathsep + env.get("PATH", "")
        env["QA_SPAWN_LOG"] = self.spawn_log
        p = subprocess.run(
            [PY, self.tracer, SCRIPT, "--root", root or self.root]
            + list(args),
            cwd=self.aux, env=env, capture_output=True, text=True,
            timeout=120)
        # VTT-507: nothing the step's script starts is Stryker, a
        # package runner, a container or a task run.
        self.assertEqual(read(self.forbidden_log), "",
                         "a forbidden stand-in was called")
        spawns = [json.loads(l) for l in read(self.spawn_log).splitlines()
                  if l.strip()]
        return Result(p.returncode, p.stdout, p.stderr, spawns)

    def check(self, root=None):
        return self.run_script("check", root=root)

    def assert_only_git(self, r):
        # D4: the stale and missing paths start no process but git.
        progs = r.programs()
        self.assertEqual([p for p in progs if p != "git"], [],
                         "processes other than git: %r" % r.spawns)

    def assert_not_judged(self, r):
        # VTT-508 / D4: no part of check-ts-mutation.py ran.
        self.assertEqual(self.checker_calls(), [], r.text)

    def assert_not_measured(self, r):
        self.assertEqual(r.rc, 0, r.text)
        self.assertIn(NOT_MEASURED, r.text)
        self.assertTrue(r.first_line_with(NOT_MEASURED), r.text)


class _Drop(object):
    pass


DROP = _Drop()


# ---------------------------------------------------------------------
# The Taskfile, through `task --dry` only.
# ---------------------------------------------------------------------

def dry(target):
    p = subprocess.run(["task", "--dry", target], cwd=REPO, env=clean_env(),
                       capture_output=True, text=True, timeout=120)
    return p.returncode, p.stdout + p.stderr


def task_lines(output, name):
    """The command text task --dry prints for one task, block by block."""
    prefix = "task: [%s] " % name
    blocks, cur = [], None
    for line in output.splitlines():
        if line.startswith("task: ["):
            if cur is not None:
                blocks.append("\n".join(cur).rstrip())
            cur = [line[len(prefix):]] if line.startswith(prefix) else None
        elif cur is not None:
            cur.append(line)
    if cur is not None:
        blocks.append("\n".join(cur).rstrip())
    return blocks


def code_lines(text):
    return [l for l in text.splitlines()
            if l.strip() and not l.strip().startswith("#")]


@unittest.skipIf(shutil.which("task") is None, "task is not installed")
class TaskfileDryRun(unittest.TestCase):

    def test_task_check_names_no_stryker_and_no_container(self):
        # VTT-507: `task check` starts no Stryker run and no container.
        rc, out = dry("check")
        self.assertEqual(rc, 0, out)
        self.assertIn("task: [check:ts-mutation] ", out)
        bad = [l for l in code_lines(out)
               if re.search(r"\bstryker\b|\bdocker\b", l, re.I)]
        self.assertEqual(bad, [])
        # ticket item 4: the full run is a task that is not a step.
        self.assertNotIn("[check:ts-mutation:full]", out)
        # VTT-507 / D7: task check never snapshots or stamps a report.
        self.assertIsNone(re.search(
            r"ts-mutation-report\.py\s+(snapshot|stamp)", out))

    def test_the_step_runs_the_checkers_tests_then_the_script(self):
        # D4: "The checker's own tests and QA's still run as commands of
        # the step." The step's judgement is the script's `check`.
        rc, out = dry("check")
        self.assertEqual(rc, 0, out)
        cmds = task_lines(out, "check:ts-mutation")
        self.assertTrue(cmds, out)
        self.assertIn("python3 tools/check_ts_mutation_test.py -q", cmds)
        self.assertIn("python3 tools/check_ts_mutation_qa_test.py -q", cmds)
        self.assertEqual(cmds[-1], "python3 tools/ts-mutation-report.py check")

    def test_the_container_task_is_gone(self):
        # D6: "`check:ts-mutation:docker` goes."
        rc, out = dry("check:ts-mutation:docker")
        self.assertNotEqual(rc, 0)
        self.assertIn("does not exist", out)

    def test_the_full_run_is_one_set_e_block_in_order(self):
        # D6: snapshot, Stryker, stamp, checker, under `set -e`.
        rc, out = dry("check:ts-mutation:full")
        self.assertEqual(rc, 0, out)
        blocks = task_lines(out, "check:ts-mutation:full")
        self.assertEqual(len(blocks), 1, out)
        code = code_lines(blocks[0])
        self.assertEqual(code[0].strip(), "set -e")
        text = "\n".join(code)
        i_snap = text.index("tools/ts-mutation-report.py snapshot")
        i_stryker = text.index("stryker run stryker.conf.json")
        i_stamp = text.index("tools/ts-mutation-report.py stamp")
        i_judge = text.index(
            "python3 tools/check-ts-mutation.py tools/ts-mutation-"
            "equivalents.txt")
        self.assertLess(i_snap, i_stryker)
        self.assertLess(i_stryker, i_stamp)
        self.assertLess(i_stamp, i_judge)


# ---------------------------------------------------------------------
# snapshot
# ---------------------------------------------------------------------

class Snapshot(FixtureCase):

    def snap(self):
        r = self.run_script("snapshot")
        self.assertEqual(r.rc, 0, r.text)
        lines = r.out.splitlines()
        self.assertEqual(len(lines), 1, r.out)
        return json.loads(lines[0])

    def test_snapshot_prints_one_json_line_with_the_four_keys(self):
        # D2: inputs, commit, clean; snapshot adds started.
        s = self.snap()
        self.assertEqual(sorted(s), ["clean", "commit", "inputs", "started"])

    def test_snapshot_names_the_commit_checked_out(self):
        # VTT-512, with the checked-out commit not the branch tip.
        write(self.path("client/src/a.ts"), A_TS + "// two\n")
        second = self.commit("second")
        self.assertEqual(self.snap()["commit"], second)
        self.git("checkout", "-q", self.first)
        s = self.snap()
        self.assertEqual(s["commit"], self.first)
        self.assertTrue(re.fullmatch(r"[0-9a-f]{40}", s["commit"]))

    def test_snapshot_inputs_is_the_hash_of_the_tree_now(self):
        # D2: `inputs`, the hash.
        write(self.path("client/src/a.ts"), A_TS + "// edit\n")
        s = self.snap()
        self.assertEqual(s["inputs"], self.tree_hash())
        self.assertTrue(re.fullmatch(r"[0-9a-f]+", s["inputs"]))

    def test_snapshot_started_is_now_in_epoch_seconds(self):
        # D2: `started`, the run's start in epoch seconds.
        before = time.time()
        s = self.snap()
        after = time.time()
        self.assertIsInstance(s["started"], (int, float))
        self.assertNotIsInstance(s["started"], bool)
        self.assertGreaterEqual(s["started"], int(before) - 1)
        self.assertLessEqual(s["started"], after + 1)

    def test_snapshot_writes_no_stamp(self):
        # D2: `stamp` writes the object; snapshot only prints it.
        self.snap()
        self.assertFalse(os.path.exists(self.path(STAMP)))

    def test_clean_is_true_on_a_committed_tree(self):
        # VTT-513
        self.assertIs(self.snap()["clean"], True)

    def test_clean_is_false_exactly_when_an_edit_moves_the_hash(self):
        # VTT-513 and D2: "kept to the files the hasher reads". Whether
        # the hasher reads a file is measured, so this holds when the
        # next task widens what it reads.
        edits = [
            ("edit", "client/src/a.ts"),
            ("edit", "client/test/a.test.ts"),
            ("edit", "stryker.conf.json"),
            ("edit", "package.json"),
            ("edit", "bun.lock"),
            ("edit", "client/tsconfig.json"),
            ("edit", "contract/gen/ts/vtt/v1/events_pb.ts"),
            ("edit", "contract/testdata/join_url_format.json"),
            ("edit", "scenarios/goldens/adventure-night/state.json"),
            ("edit", "campaigns/example/art/a.json"),
            ("edit", "contract/gen/tools/tools.json"),
            ("edit", "campaigns/example/maps/cellar.json"),
            ("edit", "client/src/style.css"),
            ("edit", "scenarios/goldens/README.md"),
            ("edit", SPECS + "2026-01-01-one-design.md"),
            ("edit", "tools/ts-mutation-equivalents.txt"),
            ("delete", "client/src/a.ts"),
            ("delete", "bun.lock"),
            ("add", "client/src/b.ts"),
            ("add", "client/src/deep/c.tsx"),
            ("add", "client/test/b.test.ts"),
            ("add", "client/src/x.mts"),
            ("add", "bun.lockb"),
            ("add", "client/src/notes.txt"),
            ("add", "scenarios/goldens/new/state.json"),
            ("add", "campaigns/example/art/b.png"),
            ("add", "docs/elsewhere.md"),
        ]
        seen = set()
        for kind, rel in edits:
            with self.subTest(kind=kind, file=rel):
                self.git("reset", "-q", "--hard", self.first)
                self.git("clean", "-q", "-fdx", "-e", "tools/")
                before = self.tree_hash()
                if kind == "edit":
                    write(self.path(rel), "changed\n", mode="a")
                elif kind == "delete":
                    os.remove(self.path(rel))
                else:
                    write(self.path(rel), "new\n")
                moved = self.tree_hash() != before
                seen.add(moved)
                self.assertIs(self.snap()["clean"], not moved)
        # Both answers occur, or the matrix says nothing.
        self.assertEqual(seen, {True, False})

    def test_an_unread_file_under_a_hashed_tree_leaves_clean_true(self):
        # D2: client/src/style.css and scenarios/goldens/README.md.
        write(self.path("client/src/style.css"), "b {}\n", mode="a")
        write(self.path("scenarios/goldens/README.md"), "more\n", mode="a")
        self.assertIs(self.snap()["clean"], True)

    def test_an_untracked_file_the_hash_reads_is_not_clean(self):
        # VTT-513: "or was untracked".
        write(self.path("client/src/b.ts"), "export {};\n")
        self.assertIs(self.snap()["clean"], False)

    def test_a_staged_edit_is_not_clean(self):
        # VTT-513: differs from the commit though the index has it.
        write(self.path("client/src/a.ts"), A_TS + "// staged\n")
        self.git("add", "client/src/a.ts")
        self.assertIs(self.snap()["clean"], False)

    def test_an_ignored_file_the_hash_reads_is_not_clean(self):
        # VTT-513: a gitignored file is untracked, and the hash reads it.
        write(self.path(".gitignore"), "client/src/gen.ts\n", mode="a")
        self.commit("ignore a generated file")
        before = self.tree_hash()
        write(self.path("client/src/gen.ts"), "export const g = 1;\n")
        self.assertNotEqual(self.tree_hash(), before,
                            "fixture: the hasher must read the file")
        self.assertIs(self.snap()["clean"], False)

    def test_snapshot_of_an_unhashable_tree_prints_nothing(self):
        # The script's own help: "or nothing and exits 1".
        shutil.rmtree(self.path("client/test"))
        self.assertIsNone(self.tree_hash())
        r = self.run_script("snapshot")
        self.assertEqual(r.rc, 1)
        self.assertEqual(r.out, "")


# ---------------------------------------------------------------------
# stamp
# ---------------------------------------------------------------------

class Stamp(FixtureCase):

    def snapshot(self):
        r = self.run_script("snapshot")
        self.assertEqual(r.rc, 0, r.text)
        return r.out.strip(), json.loads(r.out)

    def report_dir_listing(self):
        return sorted(os.listdir(self.path("reports/mutation")))

    def test_stamp_writes_the_snapshot_as_the_stamp(self):
        # D2: stamp writes the object, started included. VTT-512.
        raw, snap = self.snapshot()
        self.write_report(mtime=snap["started"] + 5)
        r = self.run_script("stamp", raw)
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(json.loads(read(self.path(STAMP))), snap)
        self.assertEqual(snap["commit"], self.first)

    def test_stamp_refuses_without_a_report(self):
        # D7 and VTT-514: no mutation.json, no stamp, exit 1.
        raw, _ = self.snapshot()
        r = self.run_script("stamp", raw)
        self.assertEqual(r.rc, 1, r.text)
        self.assertFalse(os.path.exists(self.path(STAMP)))

    def test_stamp_refuses_a_report_as_old_as_the_snapshot(self):
        # D7: "its mtime is not after `started`" (equal is not after).
        raw, snap = self.snapshot()
        self.write_report(mtime=snap["started"])
        r = self.run_script("stamp", raw)
        self.assertEqual(r.rc, 1, r.text)
        self.assertFalse(os.path.exists(self.path(STAMP)))

    def test_stamp_refuses_a_report_older_than_the_snapshot(self):
        # VTT-514: a run that leaves no newer report writes no stamp.
        raw, snap = self.snapshot()
        self.write_report(mtime=snap["started"] - 3600)
        r = self.run_script("stamp", raw)
        self.assertEqual(r.rc, 1, r.text)
        self.assertFalse(os.path.exists(self.path(STAMP)))
        self.assertEqual(self.report_dir_listing(), ["mutation.json"])

    def test_stamp_accepts_a_report_one_second_newer(self):
        # D7: the positive side of the boundary.
        raw, snap = self.snapshot()
        self.write_report(mtime=snap["started"] + 1)
        r = self.run_script("stamp", raw)
        self.assertEqual(r.rc, 0, r.text)
        self.assertTrue(os.path.exists(self.path(STAMP)))

    def test_a_refused_stamp_leaves_the_previous_stamp_untouched(self):
        # D7: "writes nothing".
        raw, snap = self.snapshot()
        self.write_stamp(commit="1" * 40, inputs="old")
        before = read(self.path(STAMP))
        inode = os.stat(self.path(STAMP)).st_ino
        self.write_report(mtime=snap["started"] - 10)
        r = self.run_script("stamp", raw)
        self.assertEqual(r.rc, 1, r.text)
        self.assertEqual(read(self.path(STAMP)), before)
        self.assertEqual(os.stat(self.path(STAMP)).st_ino, inode)

    def test_stamp_replaces_the_file_rather_than_writing_into_it(self):
        # D2: atomically, a temporary file then os.replace. An in-place
        # write keeps the inode; a replace gives a new one.
        raw, snap = self.snapshot()
        self.write_stamp(commit="1" * 40, inputs="old")
        inode = os.stat(self.path(STAMP)).st_ino
        self.write_report(mtime=snap["started"] + 5)
        r = self.run_script("stamp", raw)
        self.assertEqual(r.rc, 0, r.text)
        self.assertNotEqual(os.stat(self.path(STAMP)).st_ino, inode)
        self.assertEqual(json.loads(read(self.path(STAMP))), snap)
        self.assertEqual(self.report_dir_listing(),
                         ["mutation.json", "ts-mutation-stamp.json"])

    def test_stamp_refuses_a_snapshot_that_is_not_json(self):
        # The script's own help: "or refuses (exit 1, writes nothing)".
        self.write_report(mtime=time.time() + 60)
        for bad in ("", "not json", "[1, 2]"):
            with self.subTest(snapshot=bad):
                r = self.run_script("stamp", bad)
                self.assertEqual(r.rc, 1, r.text)
                self.assertFalse(os.path.exists(self.path(STAMP)))

    def test_stamp_refuses_a_snapshot_without_started(self):
        # D7: newer than started cannot be shown without started.
        raw, snap = self.snapshot()
        del snap["started"]
        self.write_report(mtime=time.time() + 60)
        r = self.run_script("stamp", json.dumps(snap))
        self.assertEqual(r.rc, 1, r.text)
        self.assertFalse(os.path.exists(self.path(STAMP)))

    def test_a_stamped_report_is_current_whatever_the_verdict(self):
        # VTT-515 and D7: "a red verdict must stay red".
        raw, snap = self.snapshot()
        self.write_report(mtime=snap["started"] + 5)
        self.assertEqual(self.run_script("stamp", raw).rc, 0)
        for verdict in (1, 0):
            with self.subTest(verdict=verdict):
                write(self.checker_log, "")
                self.set_verdict(verdict)
                r = self.check()
                self.assertEqual(r.rc, verdict, r.text)
                self.assertNotIn(NOT_MEASURED, r.text)
                self.assertEqual(len(self.checker_calls()), 1)

    def test_a_dirty_tree_is_recorded_not_refused(self):
        # D2: "A dirty tree is recorded, not refused. The hash decides
        # currency."
        write(self.path("client/src/a.ts"), A_TS + "// dirty\n")
        raw, snap = self.snapshot()
        self.assertIs(snap["clean"], False)
        self.write_report(mtime=snap["started"] + 5)
        r = self.run_script("stamp", raw)
        self.assertEqual(r.rc, 0, r.text)
        self.assertIs(json.loads(read(self.path(STAMP)))["clean"], False)
        r = self.check()
        self.assertEqual(r.rc, 0, r.text)
        self.assertNotIn(NOT_MEASURED, r.text)
        self.assertEqual(len(self.checker_calls()), 1)


# ---------------------------------------------------------------------
# check: no stored report
# ---------------------------------------------------------------------

class CheckMissing(FixtureCase):

    def setUp(self):
        super(CheckMissing, self).setUp()
        self.set_verdict(1)

    def test_no_report_passes_and_says_there_is_none(self):
        # VTT-510 and D11: the same first words, and no stored report.
        r = self.check()
        self.assert_not_measured(r)
        self.assertRegex(r.text, r"(?i)\bno (stored )?report\b")
        self.assert_not_judged(r)
        self.assert_only_git(r)

    def test_no_report_with_a_current_stamp_is_still_missing(self):
        # D3: "Missing: no mutation.json", whatever the stamp says.
        self.write_stamp()
        r = self.check()
        self.assert_not_measured(r)
        self.assertRegex(r.text, r"(?i)\bno (stored )?report\b")
        self.assert_not_judged(r)
        self.assert_only_git(r)


# ---------------------------------------------------------------------
# check: a stale stored report
# ---------------------------------------------------------------------

class CheckStale(FixtureCase):
    """The report and its stamp measured the first commit. Since then a
    commit added a ticket, deleted one, edited one and moved the hash,
    and the working tree holds an uncommitted ticket and two files that
    are not tickets. The stand-in checker would fail if it ran."""

    ALPHA = "2026-02-01-alpha-design.md"
    ZETA = "2026-05-01-zeta-design.md"
    GONE = "2026-01-03-gone-design.md"
    EDITED = "2026-01-04-edited-design.md"
    OLD = ("2026-01-01-one-design.md", "2026-01-02-two-design.md")

    def setUp(self):
        super(CheckStale, self).setUp()
        write(self.path(SPECS + self.GONE), "gone\n")
        write(self.path(SPECS + self.EDITED), "edited\n")
        self.measured = self.commit("tickets the report knew")
        self.write_report()
        self.write_stamp()
        os.remove(self.path(SPECS + self.GONE))
        write(self.path(SPECS + self.EDITED), "more\n", mode="a")
        write(self.path(SPECS + self.ZETA), "zeta\n")
        write(self.path("client/src/a.ts"), A_TS + "// later\n")
        self.later = self.commit("later work")
        write(self.path(SPECS + self.ALPHA), "alpha\n")
        write(self.path(SPECS + "notes.md"), "not a ticket\n")
        write(self.path(SPECS + "2026-03-01-draft.md"), "not a ticket\n")
        self.set_verdict(1)

    def test_a_stale_report_passes_without_being_judged(self):
        # VTT-508
        r = self.check()
        self.assertEqual(r.rc, 0, r.text)
        self.assert_not_judged(r)

    def test_a_stale_report_says_not_measured_and_names_its_commit(self):
        # VTT-509 and D11: the commit as rev-parse --short=12 prints it.
        r = self.check()
        self.assert_not_measured(r)
        self.assertNotEqual(self.measured, self.head())
        self.assertTrue(token(r.text, self.short(self.measured)), r.text)

    def test_a_stale_report_lists_the_tickets_its_commits_tree_lacks(self):
        # VTT-509 and D5: a tree difference, uncommitted tickets
        # counted, only *-design.md, sorted by name.
        r = self.check()
        self.assert_not_measured(r)
        self.assertIn(self.ALPHA, r.text)
        self.assertIn(self.ZETA, r.text)
        self.assertLess(r.text.index(self.ALPHA), r.text.index(self.ZETA))
        for absent in self.OLD + (self.GONE, self.EDITED, "notes.md",
                                  "2026-03-01-draft.md"):
            self.assertNotIn(absent, r.text)

    def test_a_stale_report_says_the_full_run_measures_it(self):
        # D11: "that `task check:ts-mutation:full` measures it".
        r = self.check()
        self.assertIn(FULL, r.text)

    def test_the_stale_path_starts_nothing_but_git(self):
        # D4. At least one git process proves the trace sees processes.
        r = self.check()
        self.assert_only_git(r)
        self.assertIn("git", r.programs())

    def test_no_ticket_added_says_none_was(self):
        # D5: "None found prints that none was added."
        os.remove(self.path(SPECS + self.ALPHA))
        self.write_stamp(commit=self.later, inputs="0" * 64)
        r = self.check()
        self.assert_not_measured(r)
        self.assertRegex(r.text, r"(?i)\bno (new )?tickets?\b|\bnone\b")
        self.assertNotIn("-design.md", r.text)

    def test_a_commit_off_the_branch_is_said_not_to_be_an_ancestor(self):
        # D5: the tickets are the tree difference, and the warning says
        # the commit is not an ancestor of HEAD.
        self.git("stash", "-q", "-u")
        self.git("checkout", "-q", "-b", "side", self.measured)
        write(self.path(SPECS + "2026-04-01-side-design.md"), "side\n")
        write(self.path(SPECS + self.ZETA), "zeta on side too\n")
        side = self.commit("side")
        self.git("checkout", "-q", "main")
        self.git("stash", "pop", "-q")
        self.write_stamp(commit=side, inputs="0" * 64)
        r = self.check()
        self.assert_not_measured(r)
        self.assertTrue(token(r.text, self.short(side)), r.text)
        self.assertRegex(r.text, r"(?i)not an ancestor")
        self.assertIn(self.ALPHA, r.text)
        self.assertNotIn(self.ZETA, r.text)
        self.assertNotIn("2026-04-01-side-design.md", r.text)
        self.assert_not_judged(r)

    def test_an_ancestor_commit_draws_no_ancestry_warning(self):
        # D5, the negative side of the warning above.
        r = self.check()
        self.assertNotRegex(r.text, r"(?i)ancestor")

    def test_no_recorded_commit_says_the_tickets_cannot_be_listed(self):
        # D5: "the warning says the tickets cannot be listed, and why".
        for commit in (DROP, None, ""):
            with self.subTest(commit=commit):
                self.write_stamp(commit=commit, inputs="0" * 64)
                r = self.check()
                self.assert_not_measured(r)
                self.assertRegex(r.text, r"(?i)cannot be listed")
                self.assertRegex(r.text, r"(?i)no commit")
                self.assertNotIn(self.ALPHA, r.text)
                self.assert_not_judged(r)

    def test_an_unknown_commit_says_the_tickets_cannot_be_listed(self):
        # D5 (P13): git ls-tree fails, and the warning says why.
        self.write_stamp(commit="deadbeef" * 5, inputs="0" * 64)
        r = self.check()
        self.assert_not_measured(r)
        self.assertRegex(r.text, r"(?i)cannot be listed")
        self.assertRegex(r.text, r"(?i)not a commit|unknown|no such")
        self.assertNotIn(self.ALPHA, r.text)
        self.assert_not_judged(r)

    def test_the_commit_is_printed_as_it_was_recorded(self):
        # D11: the commit as rev-parse prints it, lower-case hex, on
        # every line that names it.
        self.write_stamp(commit="deadbeef" * 5, inputs="0" * 64)
        r = self.check()
        found = re.findall(r"(?i)deadbeefdead", r.text)
        self.assertTrue(found, r.text)
        self.assertEqual(set(found), {"deadbeefdead"}, r.text)

    def test_each_kind_of_stale_says_why(self):
        # D11: "why (inputs changed, stamp unreadable or absent, tree
        # unhashable)", and D3: each of them is stale.
        reasons = {
            "changed": r"(?i)chang",
            "absent": r"(?i)no stamp|stamp[^\n]*(absent|missing)",
            "unreadable": r"(?i)cannot be read|unreadable|malformed",
            "unhashable": r"(?i)cannot be hashed|unhashable|could not hash",
        }
        cases = {
            "changed": lambda: None,
            "absent": lambda: os.remove(self.path(STAMP)),
            "unreadable": lambda: self.write_stamp(raw="7a69{"),
            "unhashable": lambda: shutil.rmtree(self.path("client/test")),
        }
        for kind, make in cases.items():
            with self.subTest(kind=kind):
                self.write_stamp(commit=self.measured, inputs="0" * 64)
                make()
                r = self.check()
                self.assert_not_measured(r)
                self.assert_not_judged(r)
                self.assertRegex(r.text, reasons[kind])
                for other, pattern in reasons.items():
                    if other != kind:
                        self.assertNotRegex(r.text, pattern, other)
                self.git("checkout", "-q", "--", "client/test")

    def test_a_stamp_that_does_not_hold_the_hash_is_stale(self):
        # D3: "anything else ... An unknown never reads current."
        here = self.tree_hash()
        stamps = {
            "array": json.dumps([here]),
            "string": json.dumps(here),
            "bare hash": here + "\n",
            "no inputs": json.dumps({"commit": self.head(), "clean": True,
                                     "started": 1}),
            "null inputs": json.dumps({"inputs": None, "commit": self.head(),
                                       "clean": True, "started": 1}),
        }
        for name, raw in stamps.items():
            with self.subTest(stamp=name):
                self.write_stamp(raw=raw)
                r = self.check()
                self.assert_not_measured(r)
                self.assert_not_judged(r)

    def test_an_unhashable_tree_never_reads_current(self):
        # D3: an unhashable tree is stale, even against an empty stamp.
        here = self.tree_hash()
        shutil.rmtree(self.path("client/test"))
        for inputs in ("", None, here):
            with self.subTest(inputs=inputs):
                self.write_stamp(inputs=inputs, commit=self.later)
                r = self.check()
                self.assert_not_measured(r)
                self.assert_not_judged(r)

    def test_the_old_sha256_file_is_read_by_nothing(self):
        # D2: "reports/mutation/ts-inputs.sha256 is read by nothing
        # afterwards."
        write(self.path(LEGACY), self.tree_hash() + "\n")
        os.remove(self.path(STAMP))
        r = self.check()
        self.assert_not_measured(r)
        self.assert_not_judged(r)
        write(self.path(LEGACY), "0" * 64 + "\n")
        self.write_stamp()
        self.set_verdict(0)
        r = self.check()
        self.assertEqual(r.rc, 0, r.text)
        self.assertNotIn(NOT_MEASURED, r.text)
        self.assertEqual(len(self.checker_calls()), 1)

    def test_how_clean_the_measured_tree_was_is_said(self):
        # D11: "how clean it was"; D2: true, false or null.
        texts = {}
        for clean in (True, False, None):
            self.write_stamp(commit=self.measured, inputs="0" * 64,
                             clean=clean)
            texts[clean] = self.check().text
        self.assertRegex(texts[True], r"(?i)\bclean\b")
        self.assertRegex(texts[False], r"(?i)differ|dirty|not clean")
        self.assertNotRegex(texts[False], r"(?i)from a clean tree")
        self.assertRegex(texts[None], r"(?i)not recorded|unknown")
        self.assertEqual(len(set(texts.values())), 3)

    def test_no_message_carries_a_duration(self):
        # D11: "No duration in a message, since it would rot."
        texts = {"stale": self.check().text}
        self.write_stamp()
        self.set_verdict(0)
        texts["current"] = self.check().text
        os.remove(self.path(REPORT))
        texts["missing"] = self.check().text
        duration = (r"(?i)\b\d+([.,]\d+)?\s*"
                    r"(hours?|minutes?|mins?|seconds?|secs?|h|m|s)\b"
                    r"|\b(an?|one|half an)\s+(hour|minute|second)s?\b")
        for path, text in texts.items():
            with self.subTest(path=path):
                self.assertNotRegex(text, duration)

    def test_a_stale_report_outside_a_git_repository_passes(self):
        # VTT-508 and D5: git cannot answer, the step still passes.
        shutil.rmtree(self.path(".git"))
        r = self.check()
        self.assert_not_measured(r)
        self.assertRegex(r.text, r"(?i)cannot be listed")
        self.assert_not_judged(r)


# ---------------------------------------------------------------------
# check: a current stored report
# ---------------------------------------------------------------------

class CheckCurrent(FixtureCase):

    def setUp(self):
        super(CheckCurrent, self).setUp()
        self.write_report()
        self.write_stamp()

    def test_a_current_report_exits_with_the_checkers_status(self):
        # VTT-511: the checker at <root>/tools, run with Python, in
        # <root>, over the root's equivalents file.
        for verdict in (0, 1, 4):
            with self.subTest(verdict=verdict):
                write(self.checker_log, "")
                self.set_verdict(verdict)
                r = self.check()
                self.assertEqual(r.rc, verdict, r.text)
                calls = self.checker_calls()
                self.assertEqual(len(calls), 1, r.text)
                argv, cwd = calls[0]["argv"], calls[0]["cwd"]
                self.assertEqual(os.path.realpath(cwd), self.root)
                self.assertEqual(
                    os.path.realpath(os.path.join(cwd, argv[0])),
                    self.path("tools/check-ts-mutation.py"))
                self.assertEqual(
                    os.path.realpath(os.path.join(cwd, argv[1])),
                    self.path("tools/ts-mutation-equivalents.txt"))
                runners = [s["argv"][0] for s in r.spawns
                           if any("check-ts-mutation.py" in a
                                  for a in s["argv"])]
                self.assertEqual(len(runners), 1, r.spawns)
                self.assertRegex(os.path.basename(runners[0]), r"^python")

    def test_a_current_report_says_so_and_names_its_commit(self):
        # D11: "current: that the report is current, and its commit".
        # HEAD moves past the stamp's commit without moving the hash.
        write(self.path("docs/elsewhere.md"), "x\n")
        self.assertNotEqual(self.commit("unhashed"), self.first)
        r = self.check()
        self.assertEqual(r.rc, 0, r.text)
        self.assertRegex(r.text, r"(?i)\bcurrent\b")
        self.assertTrue(token(r.text, self.short(self.first)), r.text)
        self.assertNotIn(NOT_MEASURED, r.text)

    def test_check_writes_nothing_under_reports(self):
        # D7: only the full run stamps; the step never writes the stamp
        # or the report, on any path.
        def listing():
            out = {}
            for d, _, names in os.walk(self.path("reports")):
                for n in names:
                    p = os.path.join(d, n)
                    st = os.stat(p)
                    out[p] = (read(p), st.st_ino, st.st_mtime)
            return out
        states = {
            "current": lambda: None,
            "stale": lambda: write(self.path("client/src/a.ts"), "//\n",
                                   mode="a"),
            "no stamp": lambda: os.remove(self.path(STAMP)),
            "missing": lambda: os.remove(self.path(REPORT)),
        }
        for name, make in states.items():
            with self.subTest(state=name):
                make()
                before = listing()
                self.check()
                self.assertEqual(listing(), before)

    def test_only_an_edit_the_hash_reads_makes_the_report_stale(self):
        # D3: currency is the hash, not the commit or the clean flag.
        write(self.path("client/src/style.css"), "b {}\n", mode="a")
        write(self.path("docs/elsewhere.md"), "x\n")
        self.commit("unhashed work")
        r = self.check()
        self.assertNotIn(NOT_MEASURED, r.text)
        self.assertEqual(len(self.checker_calls()), 1)
        write(self.path("client/src/a.ts"), A_TS + "// hashed\n")
        r = self.check()
        self.assert_not_measured(r)
        self.assertEqual(len(self.checker_calls()), 1)

    def test_a_survivor_no_entry_claims_fails_the_step(self):
        # Ticket item 3 and VTT-511, with the real checker.
        shutil.copy(REAL_CHECKER, self.path("tools/check-ts-mutation.py"))
        report = {
            "schemaVersion": "1.0",
            "thresholds": {"high": 80, "low": 60},
            "files": {"client/src/a.ts": {
                "language": "typescript", "source": A_TS,
                "mutants": [{
                    "id": "1", "mutatorName": "BooleanLiteral",
                    "replacement": "false", "status": "Survived",
                    "location": {"start": {"line": 1, "column": 18},
                                 "end": {"line": 1, "column": 22}}}]}}}
        self.write_report(content=json.dumps(report))
        r = self.check()
        self.assertNotEqual(r.rc, 0, r.text)
        self.assertNotIn(NOT_MEASURED, r.text)
        write(self.path("tools/ts-mutation-equivalents.txt"),
              "client/src/a.ts  BooleanLiteral  offset=18  span=4  nth=1"
              "  copies=1  false\n"
              "    code: export const a = true;\n"
              "    no caller reads the constant, so both values look the"
              " same\n")
        r = self.check()
        self.assertEqual(r.rc, 0, r.text)

    def test_a_current_report_whose_checker_cannot_run_fails(self):
        # VTT-511: no verdict is not a pass.
        os.remove(self.path("tools/check-ts-mutation.py"))
        r = self.check()
        self.assertNotEqual(r.rc, 0, r.text)


# ---------------------------------------------------------------------
# The full run, as `task --dry check:ts-mutation:full` prints it.
# ---------------------------------------------------------------------

SIM = r'''
import json, os, subprocess, sys, time
LOG = {log!r}
GIT = {git!r}

def log(argv):
    with open(LOG, "a") as f:
        f.write(json.dumps(argv) + "\n")

def stryker():
    mode = os.environ.get("QA_STRYKER_MODE", "write")
    root = os.getcwd()
    if mode == "fail":
        return 2
    if mode == "nowrite":
        return 0
    path = os.path.join(root, "reports", "mutation", "mutation.json")
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write("{{}}")
    t = time.time() + 30
    os.utime(path, (t, t))
    if mode in ("edit", "commit"):
        with open(os.path.join(root, "client", "src", "a.ts"), "a") as f:
            f.write("// edited while Stryker ran\n")
    if mode == "commit":
        subprocess.run(GIT + ["add", "-A", "client"], check=True)
        subprocess.run(GIT + ["commit", "-q", "-m", "during"], check=True)
    return 0
'''

STUBS = {
    "docker": '''
log(["docker"] + sys.argv[1:])
if sys.argv[1:2] == ["run"]:
    sys.exit(stryker())
sys.exit(0)
''',
    "npx": '''
log(["npx"] + sys.argv[1:])
if sys.argv[1:3] == ["stryker", "run"]:
    sys.exit(stryker())
sys.exit(0)
''',
    "uname": '''
if sys.argv[1:] == ["-s"]:
    print(os.environ["QA_UNAME_S"])
elif sys.argv[1:] == ["-r"]:
    print(os.environ["QA_UNAME_R"])
else:
    print(os.environ["QA_UNAME_S"])
''',
    "python3": '''
log(["python3"] + sys.argv[1:])
os.execv({py!r}, [{py!r}] + sys.argv[1:])
''',
}


@unittest.skipIf(shutil.which("task") is None, "task is not installed")
class FullRun(FixtureCase):
    """Executes the printed block in a fixture, Stryker and the container
    replaced by stand-ins that write a report as Stryker would."""

    @classmethod
    def setUpClass(cls):
        rc, out = dry("check:ts-mutation:full")
        blocks = task_lines(out, "check:ts-mutation:full")
        if rc != 0 or len(blocks) != 1:
            raise AssertionError("cannot read the full run: " + out)
        cls.block = blocks[0]

    def setUp(self):
        super(FullRun, self).setUp()
        shutil.copy(SCRIPT, self.path("tools/ts-mutation-report.py"))
        shutil.copy(HASHER, self.path("tools/ts-mutation-inputs.py"))
        self.bin2 = os.path.join(self.aux, "bin2")
        self.calls_log = os.path.join(self.aux, "calls.log")
        os.makedirs(self.bin2)
        write(self.calls_log, "")
        sim = SIM.format(log=self.calls_log, git=GIT)
        for name, body in STUBS.items():
            p = os.path.join(self.bin2, name)
            write(p, "#!%s\n%s\n%s" % (PY, sim, body.format(py=PY)))
            os.chmod(p, 0o755)
        for name in ("bunx", "stryker", "task", "bun", "node", "npm"):
            p = os.path.join(self.bin2, name)
            write(p, FORBIDDEN_STUB.format(name=name, log=self.forbidden_log))
            os.chmod(p, 0o755)

    def run_block(self, system="Darwin", release="25.2.0", force=None,
                  mode="write", verdict=0):
        self.set_verdict(verdict)
        env = clean_env()
        env["PATH"] = self.bin2 + os.pathsep + env.get("PATH", "")
        env["QA_UNAME_S"] = system
        env["QA_UNAME_R"] = release
        env["QA_STRYKER_MODE"] = mode
        if force is not None:
            env["VTT_FORCE_TS_MUTATION"] = force
        for name in ("docker", "npx", "uname", "python3", "bunx",
                     "stryker", "task"):
            self.assertEqual(shutil.which(name, path=env["PATH"]),
                             os.path.join(self.bin2, name))
        p = subprocess.run(["/bin/sh", "-c", self.block], cwd=self.root,
                           env=env, capture_output=True, text=True,
                           timeout=300)
        self.assertEqual(read(self.forbidden_log), "",
                         "bunx, stryker, task, bun, node or npm ran")
        calls = [json.loads(l) for l in read(self.calls_log).splitlines()
                 if l.strip()]
        return p.returncode, p.stdout + p.stderr, calls

    @staticmethod
    def index(calls, pred):
        for i, c in enumerate(calls):
            if pred(c):
                return i
        return None

    def order(self, calls):
        def py(*words):
            return lambda c: c[0] == "python3" and all(
                any(w in a for a in c) for w in words)
        stryker = self.index(calls, lambda c: (
            c[:2] == ["docker", "run"] or c[:3] == ["npx", "stryker", "run"]))
        return (self.index(calls, py("ts-mutation-report.py", "snapshot")),
                stryker,
                self.index(calls, py("ts-mutation-report.py", "stamp")),
                self.index(calls, py("check-ts-mutation.py")))

    def stamp(self):
        return json.loads(read(self.path(STAMP)))

    def test_darwin_25_runs_stryker_in_the_container_and_judges_natively(self):
        # D6 steps 1 to 4; ticket item 4; VTT-512; VTT-515.
        head = self.head()
        rc, out, calls = self.run_block()
        self.assertEqual(rc, 0, out)
        snap, stryker, stamp, judge = self.order(calls)
        self.assertIsNotNone(snap, calls)
        self.assertIsNotNone(stryker, calls)
        self.assertEqual(calls[stryker][:2], ["docker", "run"])
        self.assertLess(snap, stryker)
        self.assertLess(stryker, stamp)
        self.assertLess(stamp, judge)
        self.assertEqual([c for c in calls if c[0] == "npx"], [])
        inside = " ".join(calls[stryker])
        self.assertIn("npx stryker run stryker.conf.json", inside)
        self.assertNotIn("check-ts-mutation", inside)
        # D6, a preference: the container's apt-get line keeps python3.
        self.assertRegex(inside, r"apt-get install[^\n]*python3")
        self.assertEqual(calls[judge][1:], [
            "tools/check-ts-mutation.py", "tools/ts-mutation-equivalents.txt"])
        self.assertEqual(len(self.checker_calls()), 1)
        self.assertEqual(self.stamp()["commit"], head)
        self.assertIs(self.stamp()["clean"], True)
        self.assertEqual(self.stamp()["inputs"], self.tree_hash())
        # Ticket item 4: a check right after calls the report current
        # and names the commit it measured, with HEAD moved on since.
        write(self.path("docs/elsewhere.md"), "x\n")
        self.assertNotEqual(self.commit("after the run"), head)
        write(self.checker_log, "")
        r = self.check()
        self.assertEqual(r.rc, 0, r.text)
        self.assertRegex(r.text, r"(?i)\bcurrent\b")
        self.assertTrue(token(r.text, self.short(head)), r.text)
        self.assertEqual(len(self.checker_calls()), 1)

    def test_below_darwin_25_stryker_runs_natively(self):
        # D6 step 2: otherwise `npx stryker run stryker.conf.json`.
        for system, release, force in (("Darwin", "24.6.0", None),
                                       ("Darwin", "25.2.0", "1"),
                                       ("Linux", "6.8.0", None)):
            with self.subTest(system=system, release=release, force=force):
                write(self.calls_log, "")
                write(self.checker_log, "")
                rc, out, calls = self.run_block(system, release, force)
                self.assertEqual(rc, 0, out)
                self.assertEqual([c for c in calls if c[0] == "docker"], [])
                runs = [c for c in calls if c[0] == "npx"]
                self.assertEqual(runs, [["npx", "stryker", "run",
                                         "stryker.conf.json"]])
                snap, stryker, stamp, judge = self.order(calls)
                self.assertLess(snap, stryker)
                self.assertLess(stryker, stamp)
                self.assertLess(stamp, judge)
                self.assertTrue(os.path.exists(self.path(STAMP)))

    def test_a_red_verdict_still_leaves_the_report_current(self):
        # VTT-515 and D7: stamped before judged, and red stays red.
        rc, out, calls = self.run_block(verdict=1)
        self.assertNotEqual(rc, 0, out)
        self.assertTrue(os.path.exists(self.path(STAMP)))
        write(self.checker_log, "")
        r = self.check()
        self.assertEqual(r.rc, 1, r.text)
        self.assertNotIn(NOT_MEASURED, r.text)
        self.assertEqual(len(self.checker_calls()), 1)

    def test_a_run_that_leaves_no_newer_report_stamps_and_judges_nothing(self):
        # VTT-514 and D7: an older report is one this run did not write.
        self.write_report(mtime=time.time() - 3600)
        self.write_stamp(commit="1" * 40, inputs="0" * 64)
        before = read(self.path(STAMP))
        rc, out, calls = self.run_block(mode="nowrite")
        self.assertNotEqual(rc, 0, out)
        self.assertEqual(read(self.path(STAMP)), before)
        self.assertEqual(self.checker_calls(), [])
        r = self.check()
        self.assert_not_measured(r)

    def test_a_failed_stryker_stops_the_block(self):
        # D6: under `set -e`.
        rc, out, calls = self.run_block(mode="fail")
        self.assertNotEqual(rc, 0, out)
        self.assertFalse(os.path.exists(self.path(STAMP)))
        self.assertEqual(self.checker_calls(), [])

    def test_an_edit_made_during_the_run_reads_stale_afterwards(self):
        # D7: "The snapshot is taken before Stryker starts".
        before = self.tree_hash()
        rc, out, calls = self.run_block(mode="edit")
        self.assertEqual(rc, 0, out)
        self.assertEqual(self.stamp()["inputs"], before)
        self.assertIs(self.stamp()["clean"], True)
        write(self.checker_log, "")
        r = self.check()
        self.assert_not_measured(r)
        self.assertRegex(r.text, r"(?i)chang")
        self.assert_not_judged(r)

    def test_a_commit_made_during_the_run_does_not_move_the_stamp(self):
        # VTT-512: the commit checked out when the run began.
        began = self.head()
        rc, out, calls = self.run_block(mode="commit")
        self.assertEqual(rc, 0, out)
        self.assertNotEqual(self.head(), began)
        self.assertEqual(self.stamp()["commit"], began)


if __name__ == "__main__":
    unittest.main()
