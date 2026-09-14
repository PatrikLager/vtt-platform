#!/usr/bin/env python3
"""Hold the prose a change ADDS to the two checkers nothing was running.

tools/check-citations.py and tools/check-comment-wrap.py both existed, both had
unit tests, and neither was named by Taskfile.yml or .lefthook.yml. They were
tools, not gates. Run over the whole tree each reports findings older than this
file, so wiring either one as-is would either red the gate on its first run or
-- worse -- be wired in a mode that never fails, reading like enforcement while
enforcing nothing.

SCOPED TO ADDED LINES, not to files. Scoping to files would red on somebody
else's older finding the moment a change touched their file: internal/gateway/
authz_test.go carries wrap findings that predate this tool, and editing one
function in it would demand cleaning the rest. Added lines are the unit a change
is actually answerable for.

WHAT EACH HALF ACTUALLY SEES, because the two are not the same set and reading
this file's name as "all prose" would be wrong. check-citations.py harvests
citations out of Go comments and nowhere else -- its own refusal names the set,
"no Go files under <root>". The TypeScript client and the config files it globs
are evidence that a cited name EXISTS; they are never read for citations
themselves. The wrap half is narrow too -- see CHECKED_EXT -- so a Markdown
spec, Taskfile.yml and these tools' own docstrings are held by NEITHER half.
main() prints this whole docstring as its usage message on a bad invocation,
so these are among the first sentences a reader is handed; this paragraph said
the opposite of CHECKED_EXT until 2026-09-14.

WHAT IT DOES NOT DO is clean the findings already in the tree. They stay visible
to anyone who runs either checker directly, and they stop growing. Converting
them is separate work, and CLAUDE.md rule 8 says as much about the citations
already in the tree.

Usage: check-new-prose.py <base-ref>
"""

import os
import pathlib
import re
import subprocess
import sys

CITATION_FINDING = re.compile(r"^check:citations: ([^:]+):(\d+):")
# `(.+?)`, not `(\S+)`: a path containing a space is printed as-is by
# check-comment-wrap.py, and \S+ stops at the space -- discarding a finding the
# checker did report, which reads here as a clean file. The citations half's
# `([^:]+)` was never affected.
WRAP_FINDING = re.compile(r"^(?:LONG|SHORT)\s+(.+?):(\d+) \[")

# PROOF A CHECKER RAN TO THE END, because its exit code is not proof. A Python
# traceback exits 1, and 1 is also how check-citations.py says "I found
# findings" -- so a crashed checker and a working one are the same number. Each
# tool does print a line when it finishes, and check-citations.py prints that
# line only on its clean path; on the findings path the findings themselves are
# the evidence. Either shape is accepted; neither is not.
CITATION_DONE = re.compile(r"^check:citations: \d+ files, ")
WRAP_DONE = re.compile(r"^check-comment-wrap: \d+ file")

# ONE CHECKER'S HATCH MUST NOT TRIP ANOTHER. A hatch is a fixed-format
# annotation, not prose, and check-comment-wrap.py reads an adjudication
# written for a different checker as a SHORT line with more comment after it:
# `citations:ok MapTool name, not ours` is 35 columns of body. Escaping that
# would mean ending the hatch with a period, which nobody would guess. That
# checker skips its OWN hatch (`wrap:ok`); these are the other two, skipped
# here because only this wrapper knows they are annotations, not prose.
HATCHES = ("citations:ok", "doc-owner:ok")

# THE FILES EITHER HALF IS CALIBRATED FOR, and the reason this gate is narrower
# than its name. check-comment-wrap.py finds a comment with `^\s*(//+|\*|#)`,
# which is a comment marker in Go and TypeScript and is something else
# everywhere else: `## A heading` and `**Date:**` in Markdown, `#app {` in CSS,
# `#!/usr/bin/env bash` on a shell script's first line. Measured 2026-09-14 over
# this tree, that regex flagged 179 Markdown lines, none of them a splice.
# Its band was derived from Go comments, so Go and TypeScript are where it means
# what it says. Markdown, shell, CSS and Python prose -- these tools' own
# docstrings included -- are held by NEITHER half, and saying so is better than
# a gate that reds every new spec on its `**Date:**` line with no remedy that
# does not render in the published document.
CHECKED_EXT = (".go", ".ts")

# GENERATED AND VENDORED TREES ARE NOT ANYBODY'S PROSE. Same set
# check-citations.py already skips, and the asymmetry was a live defect: the
# wrap half flags 113 lines in contract/gen (measured 2026-09-14), from TWO
# generators: 47 in protoc-gen-go's "Types that are valid to be assigned to"
# blocks, and 66 in protoc-gen-es's `@generated from` lines in the .ts. Adding
# one event or command -- the only contract change CLAUDE.md rule 3 permits --
# adds a line to such a block in BOTH, and there is no legal remedy: a wrap:ok
# hand-edit in contract/gen is overwritten by `task generate:contract` and then
# fails check:drift, which runs earlier in this gate.
SKIP_DIRS = {"gen", "node_modules", ".git", "contract-spike", "webdist"}


def in_scope(path):
    return (path.endswith(CHECKED_EXT)
            and SKIP_DIRS.isdisjoint(pathlib.PurePosixPath(path).parts))


def resolve_base(base):
    """Resolve the base ref as a LOCAL branch first, then its origin/ spelling.

    The ladder is check:breaking's, and so is the scar it is copied from: that
    gate's own prose records that actions/checkout creates a local branch only
    for the ref it checks out, so on a pull request `main` exists solely as
    refs/remotes/origin/main -- and it hard-failed on this repository's FIRST
    pull request, having passed every run before it. Taking one spelling on
    faith is how that happened, and this gate has never run in CI at all.

    The hazard the ladder accepts: a LOCAL main behind origin/main gives an
    older merge base, so lines the change did not add come into scope. Preferring
    local is check:breaking's choice and staying consistent with it is worth
    more than diverging here; `git fetch` is the fix, and a false FAIL is the
    safe direction for this gate to lean.

    When neither resolves it FAILS LOUDLY. The two outcomes that must not happen
    are a Python traceback (which says nothing about what to do) and a silent
    pass (worse than having no gate, and a failure mode this repository has been
    bitten by).
    """
    stem = base[len("origin/"):] if base.startswith("origin/") else base
    for ref in (stem, "origin/" + stem):
        got = subprocess.run(["git", "rev-parse", "--verify", "--quiet", ref + "^{commit}"],
                             capture_output=True, text=True)
        if got.returncode == 0:
            return ref
    print("check:new-prose: neither %r nor %r resolves to a commit here, so there is no way "
          "to tell which lines this change added. This gate FAILS rather than passing on an "
          "unknown scope. Fetch it (git fetch origin %s) or pass a ref that exists: "
          "check-new-prose.py <base-ref>." % (stem, "origin/" + stem, stem), file=sys.stderr)
    return None


def added_lines(base):
    """Map path -> set of line numbers this change ADDS or rewrites.

    Read from a unified diff rather than `git blame`: a line moved unchanged is
    not new prose, and blame would call it new the moment the file around it
    shifted.

    --merge-base, NOT two-dot. `git diff <base>` compares the two ENDPOINTS, so
    once main advances, a line main DELETED reads as this branch adding it back
    and a line main MODIFIED reads as this branch writing it -- and the branch
    gets failed for a finding on a line it never wrote. That is the
    exact failure this file's scoping exists to avoid, arriving through the ref
    instead of through the scope. It stays invisible while base is an ancestor
    of HEAD, which is every run on a freshly branched tree.

    core.quotePath=false so a non-ASCII path arrives as itself rather than as
    octal escapes. It does NOT cover every path git quotes -- a name containing a
    quote or a control character still arrives as `+++ "b/we\\"ird.go"` -- so a
    `+++` line that is neither /dev/null nor b/-prefixed makes this refuse rather
    than skip. Skipping it reads as "nothing was added", which is the one answer
    a gate must never give for a file it could not look at.
    """
    out = subprocess.run(
        ["git", "-c", "core.quotePath=false", "-c", "diff.noprefix=false",
         "-c", "diff.mnemonicPrefix=false", "diff", "--no-ext-diff",
         "--unified=0", "--merge-base", base, "--"],
        capture_output=True, text=True, check=True).stdout
    added, path, in_header = {}, None, False
    for line in out.splitlines():
        # `+++ ` IS NOT ENOUGH TO IDENTIFY A FILE HEADER. Under --unified=0 an
        # ADDED LINE whose text begins "++ " renders as "+++ ...", and this
        # repository's plans and reports quote diff hunks. Read as a header,
        # such a line reassigns every later hunk in the file to a path that
        # does not exist, and the gate then prints "all clean" for prose it
        # never looked at. `diff --git ` cannot collide the same way: as
        # content it would render "+diff --git".
        if line.startswith("diff --git "):
            in_header, path = True, None
        elif in_header and line.startswith("+++ "):
            # Git appends a TAB to disambiguate a path containing a space, so
            # the header reads `+++ b/my dir/a.go\t`. Left on, it defeats every
            # test below -- the name no longer ends in a checked extension, so
            # the file is dropped AND the refusal that exists for exactly this
            # case never fires.
            target = line[4:].rstrip("\t")
            if target == "/dev/null":
                path = None  # a DELETION adds nothing; leaving `path` at the
                # previous file would file this hunk under an unrelated one
            elif target.startswith("b/"):
                # Scope decided HERE, at the source, so an out-of-scope file
                # records nothing rather than being recorded and then dropped.
                path = target[2:] if in_scope(target[2:]) else None
            elif target.rstrip('"').endswith(CHECKED_EXT):
                print("check:new-prose: git reported a path it had to quote (%s), and this "
                      "gate cannot tell which of its lines are new. It FAILS rather than "
                      "skipping the file, because a skipped file reads here as a clean one. "
                      "Rename it: nothing in this repository needs a quote or a control "
                      "character in its name." % target, file=sys.stderr)
                return None
            else:
                path = None  # a quoted path this gate does not cover anyway
        elif line.startswith("@@"):
            in_header = False
            m = re.search(r"\+(\d+)(?:,(\d+))?", line) if path else None
            if m:
                start, count = int(m.group(1)), int(m.group(2) or 1)
                added.setdefault(path, set()).update(range(start, start + count))
    # AN UNTRACKED FILE IS INVISIBLE TO git diff, and a new file is entirely new
    # prose -- the case this gate most wants to see. Its whole contents count as
    # added. This is how the gate is usually met: `task check` on a dirty tree,
    # before the new file has ever been staged. -z because
    # `git ls-files --others` quotes exactly the paths the diff does, and
    # splitting its output on whitespace also loses any path containing a space.
    untracked = subprocess.run(
        ["git", "ls-files", "-z", "--others", "--exclude-standard"],
        capture_output=True, text=True, check=True).stdout.split("\0")
    for path in filter(in_scope, untracked):
        try:
            with open(path, encoding="utf-8") as fh:
                n = sum(1 for _ in fh)
        except (OSError, UnicodeDecodeError):
            continue  # a binary or unreadable file has no prose to hold
        added.setdefault(path, set()).update(range(1, n + 1))
    # A pure-deletion hunk leaves its file keyed to an EMPTY set. Keeping the key
    # would make this gate claim a scope it does not have and hand the wrap half
    # a path that no longer exists on disk.
    return {p: lines for p, lines in added.items() if lines}


def findings(argv, pattern, ok_codes, done):
    """Run a checker, returning (path, line, text) triples -- or None if it broke.

    A checker that crashes, or that refuses to run, prints nothing this pattern
    matches. Reading that as "no findings" is this gate reporting a clean tree
    precisely when it checked nothing, which is the one result a gate must never
    produce. check-citations.py exits 2 on exactly that refusal, and says so:
    "nothing was scanned, so nothing is proven. A clean run over an empty tree is
    not a pass." Throwing away that signal would discard the thing that tool was
    built to say.

    The exit code alone cannot carry this. A traceback exits 1, and 1 is also
    check-citations.py's "I found findings" -- so a run is believed only when it
    either printed its completion line or named at least one finding.

    That proof is about REACHING THE END, not about coverage: check-comment-wrap
    .py's check() swallows an unreadable file and its summary counts the files it
    was handed, so a run where every file failed to open still prints a
    well-formed line. It catches the crash and the refusal, which are the
    failures that actually happen here; it is not a claim that every file was
    read.
    """
    out = subprocess.run(argv, capture_output=True, text=True)
    stream = (out.stdout + out.stderr).splitlines()
    hits = []
    for line in stream:
        m = pattern.match(line)
        if m:
            hits.append((m.group(1), int(m.group(2)), line))
    if out.returncode not in ok_codes:
        why = "exited %d, which is not a verdict it is allowed to reach here" % out.returncode
    elif not (any(done.match(line) for line in stream) or hits):
        why = ("exited %d but neither finished nor named a finding, so it did not run"
               % out.returncode)
    else:
        return hits
    print("check:new-prose: %s %s. This gate FAILS rather than reading a checker that did "
          "not run as a clean tree.\n%s"
          % (argv[1], why, "\n".join(stream[-8:]).strip()), file=sys.stderr)
    return None


def hatched(path, line):
    """True when the source line's comment BODY opens with a checker's hatch.

    Opens with, not contains: check-doc-owner.py's own hatch check is
    `startswith`, and a substring test would exempt any line that merely
    MENTIONS a hatch in prose. The live adjudication in this tree opens its
    comment -- `tools/toolgen/main_test.go`'s `//citations:ok RemoveTokenRequest
    is a counter-example and must not exist` -- which is the shape both forms
    accept; it is the sentence ABOUT a hatch that separates them. An exemption
    is a clean verdict, so it gets the narrow test rather than the generous
    one. check-citations.py's own block hatch was a substring test until
    2026-09-14, and silenced every citation in a block that mentioned it.
    """
    try:
        with open(path, encoding="utf-8") as fh:
            for n, text in enumerate(fh, 1):
                if n == line:
                    return text.strip().lstrip("/#*").lstrip().startswith(HATCHES)
    except (OSError, UnicodeDecodeError):
        pass
    return False


def main(argv):
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    # The RESOLVED ref, not the one asked for: the ladder is pointless if the
    # spelling that failed is the one handed onward, and `git diff` against a
    # missing ref exits 128 with the traceback resolve_base exists to prevent.
    base = resolve_base(argv[1])
    if base is None:
        return 1
    added = added_lines(base)
    if added is None:
        return 1
    if not added:
        print("check:new-prose: nothing added against %s; no new prose to hold — this "
              "change adds no .go or .ts lines outside the generated trees, or it is "
              "already in the base." % base)
        return 0

    # THE CITATIONS HALF SCANS THE WHOLE TREE, and is then filtered to added
    # lines. check-citations.py takes ONE root -- `root = args[0] if args else
    # "."` -- so handing it a directory per changed file would scan the
    # first one it was handed and silently skip the rest. Scanning from the
    # root is also the only way its `known` set holds the names this very change
    # declares in another directory; a narrower scan reports those as
    # fabrications. It costs ~40s here because that tool runs `git log --all -G`
    # per unresolved name, which is the price of the whole-tree answer.
    #
    # The wrap half takes FILES, so it gets the changed ones and no more.
    files = sorted(p for p in added if os.path.exists(p))
    checks = []
    # NO GO FILE, NO CITATION FINDING IS POSSIBLE, so a docs-only or client-only
    # change does not pay for the whole-tree scan. This is not a sampling
    # shortcut: citations are read from Go comments only, so every finding the
    # scan could report lands at a .go path, and no such path is in scope here.
    if any(p.endswith(".go") for p in added):
        checks.append((["python3", "tools/check-citations.py", "."],
                       CITATION_FINDING, (0, 1), CITATION_DONE, False))
    checks.append((["python3", "tools/check-comment-wrap.py"] + files,
                   WRAP_FINDING, (0,), WRAP_DONE, True))
    guilty = []
    for check_argv, pattern, ok_codes, done, allow_hatch in checks:
        hits = findings(check_argv, pattern, ok_codes, done)
        if hits is None:
            return 1
        for path, line, text in hits:
            if line in added.get(path, ()) and not (allow_hatch and hatched(path, line)):
                guilty.append(text)

    for text in guilty:
        print(text, file=sys.stderr)
    scope = sum(len(v) for v in added.values())
    if guilty:
        print("check:new-prose: %d finding(s) on lines this change added (%d lines across "
              "%d file(s)). The tree's older findings are NOT in scope and are not the "
              "reason this failed." % (len(guilty), scope, len(added)), file=sys.stderr)
        return 1
    print("check:new-prose: %d added line(s) across %d file(s), all clean." % (scope, len(added)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
