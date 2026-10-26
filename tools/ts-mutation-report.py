#!/usr/bin/env python3
"""What check:ts-mutation does with the stored client mutation report.

`task check` never starts Stryker (SPEC-023, the owner's rule of 2026-10-26:
the client's full mutation run happens when the owner asks for it). This
script is the step's whole decision, so that tools/ts_mutation_report_test.py
can run it with no Stryker and no container:

    check            the step. A CURRENT report, one whose stamp's input hash
                     is the tree's, is judged by tools/check-ts-mutation.py and
                     the step exits with its verdict. An OUT-OF-DATE one is not
                     judged: the step says the client's mutation was not
                     measured, which commit the report measured and which
                     tickets were added since, and exits 0. With NO report it
                     says the mutation was not measured and that there is no
                     report, and exits 0.
    snapshot         the stamp a full run starting now would write, printed as
                     one JSON line; nothing, and exit 1, when the tree cannot be
                     hashed or is not a git repository.
    stamp SNAPSHOT   writes SNAPSHOT as the stamp, and refuses, writing
                     nothing, unless a report newer than the snapshot exists.

`task check:ts-mutation:full` takes the snapshot before Stryker starts and
stamps after it ends, then judges. It stamps BEFORE it judges, so a full run
that finds a survivor leaves a current report, and the next `task check`
judges it red again rather than calling it out of date and passing.
"""

import argparse
import importlib.util
import json
import math
import os
import subprocess
import sys
import tempfile
import time

REPORT = "reports/mutation/mutation.json"
STAMP = "reports/mutation/ts-mutation-stamp.json"
SPECS = "docs/superpowers/specs"
TICKET_SUFFIX = "-design.md"
NOT_MEASURED = "THE CLIENT'S MUTATION WAS NOT MEASURED"
FULL_RUN = "`task check:ts-mutation:full` measures it when the owner asks for it."
CLEAN_NOTES = {
    True: "from a clean tree",
    False: "with files the hash reads differing from that commit",
    None: "whether the tree was clean was not recorded",
}

_HERE = os.path.dirname(os.path.abspath(__file__))
_spec = importlib.util.spec_from_file_location(
    "tsmi", os.path.join(_HERE, "ts-mutation-inputs.py"))
tsmi = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(tsmi)


def _git(root, *args):
    """git's stdout, or None when it fails or is absent."""
    try:
        done = subprocess.run(["git", "-C", root, *args], capture_output=True,
                              text=True)
    except OSError:
        return None
    return done.stdout if done.returncode == 0 else None


def _hashed(rel):
    """Whether the hasher reads the file at this repository-relative path."""
    rel = rel.replace(os.sep, "/")
    if rel in tsmi.FILES:
        return True
    return any(rel.startswith(tree + "/") and rel.endswith(suffixes)
               for tree, suffixes in tsmi.TREES)


def _clean(root):
    """Whether no file the hasher reads differs from HEAD or is untracked, an
    ignored one included, or None when git cannot say."""
    paths = [tree for tree, _ in tsmi.TREES] + list(tsmi.FILES)
    out = _git(root, "status", "--porcelain=v1", "-z", "--untracked-files=all",
               "--ignored", "--", *paths)
    if out is None:
        return None
    tokens = out.split("\0")
    i = 0
    while i < len(tokens):
        entry = tokens[i]
        i += 1
        if len(entry) < 4:
            continue
        status, rel = entry[:2], entry[3:]
        changed = [rel]
        if ("R" in status or "C" in status) and i < len(tokens):
            changed.append(tokens[i])
            i += 1
        if any(_hashed(p) for p in changed):
            return False
    return True


def snapshot(root):
    """The stamp a full run starting now would write, or None."""
    inputs = tsmi.inputs_hash(root)
    if not inputs:
        return None
    commit = _git(root, "rev-parse", "--verify", "HEAD")
    if commit is None:
        return None
    commit = commit.strip()
    return {"inputs": inputs, "commit": commit, "clean": _clean(root),
            "started": time.time()}


def write_stamp(root, obj):
    """Write the stamp whole or not at all: a temporary file, then a rename."""
    path = os.path.join(root, STAMP)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=os.path.dirname(path), prefix=".stamp-")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            json.dump(obj, fh, sort_keys=True)
            fh.write("\n")
        os.replace(tmp, path)
    except BaseException:
        if os.path.exists(tmp):
            os.unlink(tmp)
        raise


def _fields(obj, need_started=True):
    """A snapshot's or a stamp's fields, each of its kind, or ValueError. A
    stamp may lack `started`: the one written for a report no snapshot
    preceded does."""
    if not isinstance(obj, dict):
        raise ValueError("not a JSON object")
    try:
        inputs, commit, clean = obj["inputs"], obj["commit"], obj["clean"]
    except KeyError as e:
        raise ValueError(f"no {e.args[0]}") from None
    if not (isinstance(inputs, str) and inputs and isinstance(commit, str)
            and commit and (clean is None or isinstance(clean, bool))):
        raise ValueError("inputs, commit or clean is not of its kind")
    fields = {"inputs": inputs, "commit": commit, "clean": clean}
    if "started" in obj or need_started:
        started = obj.get("started")
        if not isinstance(started, (int, float)) or isinstance(started, bool):
            raise ValueError("started is not a number")
        try:
            started = float(started)
        except OverflowError:
            raise ValueError("started is not a finite number") from None
        if not math.isfinite(started):
            raise ValueError("started is not a finite number")
        fields["started"] = started
    return fields


def stamp(root, snap_json, err=sys.stderr):
    """Stamp the report the full run just wrote; 0 when stamped, 1 when not."""
    try:
        fields = _fields(json.loads(snap_json))
    except ValueError as e:
        print(f"ts-mutation-report: not a snapshot ({e}); no stamp written",
              file=err)
        return 1
    started = fields["started"]
    try:
        mtime = os.stat(os.path.join(root, REPORT)).st_mtime
    except OSError:
        print(f"ts-mutation-report: no {REPORT}; no stamp written", file=err)
        return 1
    if mtime <= started:
        print(f"ts-mutation-report: {REPORT} is not newer than this run's start, "
              f"so this run did not write it; no stamp written", file=err)
        return 1
    write_stamp(root, fields)
    return 0


def read_stamp(root):
    """The stamp's fields, or None when it is absent or unreadable."""
    try:
        with open(os.path.join(root, STAMP), encoding="utf-8") as fh:
            return _fields(json.load(fh), need_started=False)
    except (OSError, ValueError):
        return None


def report_state(root):
    """("current" | "missing" | "stale", the stamp or None, why)."""
    stamp_obj = read_stamp(root)
    if not os.path.isfile(os.path.join(root, REPORT)):
        return "missing", stamp_obj, "there is no stored report"
    if stamp_obj is None:
        if os.path.exists(os.path.join(root, STAMP)):
            return "stale", None, f"its stamp, {STAMP}, cannot be read"
        return "stale", None, f"it has no stamp, {STAMP}"
    inputs = tsmi.inputs_hash(root)
    if not inputs:
        return "stale", stamp_obj, "the tree's inputs cannot be hashed"
    if stamp_obj.get("inputs") != inputs:
        return "stale", stamp_obj, "the report's inputs changed since it was made"
    return "current", stamp_obj, "its inputs are the tree's"


def _short(root, commit):
    out = _git(root, "rev-parse", "--short=12", "--verify", f"{commit}^{{commit}}")
    return out.strip() if out else commit[:12]


def tickets_since(root, commit):
    """(the tickets the commit's tree lacks, sorted, or None; a note or None).

    A tree difference, not a history walk: it needs no ancestry, and it counts
    a ticket not yet committed."""
    if not commit:
        return None, "The stamp records no commit, so the tickets cannot be listed"
    listed = _git(root, "ls-tree", "-r", "--name-only", f"{commit}^{{tree}}",
                  "--", SPECS)
    if listed is None:
        return None, (f"The stamp's commit, {commit[:12]}, is not a commit of "
                      f"this repository, so the tickets cannot be listed")
    before = {os.path.basename(p) for p in listed.splitlines()}
    try:
        now = os.listdir(os.path.join(root, SPECS))
    except OSError:
        now = []
    since = sorted(n for n in now if n.endswith(TICKET_SUFFIX) and n not in before)
    note = None
    ancestor = subprocess.run(["git", "-C", root, "merge-base", "--is-ancestor",
                               commit, "HEAD"], capture_output=True)
    if ancestor.returncode == 1:
        note = f"The stamp's commit, {commit[:12]}, is not an ancestor of HEAD"
    return since, note


def _run_checker(cmd, cwd):
    return subprocess.run(cmd, cwd=cwd).returncode


def check(root, run=_run_checker, out=sys.stdout):
    """The step: judge a current report, or say why there is none to judge."""
    state, stamp_obj, why = report_state(root)
    if state == "current":
        commit = stamp_obj.get("commit")
        clean = CLEAN_NOTES.get(stamp_obj.get("clean"), CLEAN_NOTES[None])
        named = _short(root, commit) if commit else "no recorded commit"
        print(f"check:ts-mutation: the stored report is current ({why}); it "
              f"measured {named}, {clean}. Judging it.", file=out, flush=True)
        return run([sys.executable,
                    os.path.join(root, "tools", "check-ts-mutation.py"),
                    os.path.join(root, "tools", "ts-mutation-equivalents.txt")],
                   root)
    lines = [f"check:ts-mutation: {NOT_MEASURED}."]
    if state == "missing":
        lines.append(f"  There is no stored report ({REPORT}).")
    else:
        lines.append(f"  The stored report is out of date: {why}.")
        commit = (stamp_obj or {}).get("commit")
        if commit:
            clean = CLEAN_NOTES.get(stamp_obj.get("clean"), CLEAN_NOTES[None])
            lines.append(f"  It measured {_short(root, commit)}, {clean}.")
        since, note = tickets_since(root, commit)
        if note:
            lines.append(f"  {note}.")
        if since is not None:
            if since:
                lines.append("  Tickets added since:")
                lines.extend(f"    {SPECS}/{name}" for name in since)
            else:
                lines.append("  No ticket was added since.")
    lines.append(f"  {FULL_RUN}")
    print("\n".join(lines), file=out, flush=True)
    return 0


def main(argv):
    parser = argparse.ArgumentParser(prog="ts-mutation-report.py")
    parser.add_argument("--root", default=os.path.dirname(_HERE))
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("check")
    sub.add_parser("snapshot")
    stamp_parser = sub.add_parser("stamp")
    stamp_parser.add_argument("snapshot")
    args = parser.parse_args(argv[1:])
    root = os.path.abspath(args.root)
    if args.cmd == "check":
        return check(root)
    if args.cmd == "snapshot":
        snap = snapshot(root)
        if snap is None:
            print("ts-mutation-report: cannot snapshot the tree: its inputs "
                  "cannot be hashed, or it is not a git repository",
                  file=sys.stderr)
            return 1
        print(json.dumps(snap, sort_keys=True))
        return 0
    return stamp(root, args.snapshot)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
