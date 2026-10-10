"""Independent QA tests for the Go mutation gate (tools/check-mutation.py).

Derived from docs/superpowers/specs/2026-10-10-a-mutation-adjudication-is-keyed-by-its-code-design.md,
the requirement rows VTT-298..VTT-304, and the decisions D1-D20 of
docs/superpowers/plans/2026-10-10-a-mutation-adjudication-is-keyed-by-its-code.md.
The gate is loaded by import only; gremlins is replaced by a fake runner that
reports positions in the tree as it stands when the runner is called.
"""

import importlib.util
import io
import os
import re
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True

GATE = os.environ.get(
    "CHECK_MUTATION_PATH",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-mutation.py"),
)
REPO = os.path.dirname(os.path.dirname(os.path.abspath(GATE)))
REAL_EQUIVALENTS = os.path.join(REPO, "tools", "mutation-equivalents.txt")


def _load():
    spec = importlib.util.spec_from_file_location("check_mutation_under_qa", GATE)
    mod = importlib.util.module_from_spec(spec)
    sys.modules["check_mutation_under_qa"] = mod
    spec.loader.exec_module(mod)
    return mod


cm = _load()

CB = "CONDITIONALS_BOUNDARY"
CN = "CONDITIONALS_NEGATION"

# The adjudicated code, and two other lines that put the same mutator's token
# at the same offsets but are not that code.
CODE = "if a > b && c > d {"
OTHER = "if a > b || c > d {"
WIDE = "if a >  b && c > d {"  # CODE with one interior space added
FIRST = CODE.index(">") + 1  # 6: the first `>`
SECOND = CODE.rindex(">") + 1  # 15: the second `>`

REASON = "Equivalent: the branch assigns the value the variable already holds."


def f_go(pkg="p", inner=CODE, outer=None, inner_indent="\t\t", outer_indent="\t",
         pad=0, eol="\n", trail=""):
    """A Go file whose function f holds `inner` at `inner_indent`, after `pad`
    inserted comment lines; function g holds `outer` when given."""
    lines = ["package " + pkg]
    lines += ["// inserted line %d" % i for i in range(pad)]
    lines += [
        "",
        "func f(a, b, c, d int) int {",
        "\tx := 0",
        "\tfor i := 0; i < 3; i++ {",
        inner_indent + inner + trail,
        "\t\t\tx += i",
        "\t\t}",
        "\t}",
        "\treturn x",
        "}",
    ]
    if outer is not None:
        lines += [
            "",
            "func g(a, b, c, d int) bool {",
            outer_indent + outer + trail,
            "\t\treturn true",
            "\t}",
            "\treturn false",
            "}",
        ]
    return (eol.join(lines) + eol).encode("utf-8")


def three_go(pkg="p"):
    """CODE three times, each copy at its own indentation (1, 2 and 9 bytes)."""
    return ("package %s\n\nfunc f(a, b, c, d int) bool {\n"
            "\t%s\n\t\treturn true\n\t}\n"
            "\tfor i := 0; i < 2; i++ {\n\t\t%s\n\t\t\treturn false\n\t\t}\n\t}\n"
            "\tswitch {\n\tdefault:\n\t        %s\n\t\t\treturn true\n\t\t}\n\t}\n"
            "\treturn false\n}\n" % (pkg, CODE, CODE, CODE)).encode("utf-8")


def write(root, rel, data):
    path = os.path.join(root, rel)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "wb") as fh:
        fh.write(data if isinstance(data, bytes) else data.encode("utf-8"))


def entry(pkg, file_, mutator, offset, copies, code, reason=REASON, nth=1):
    return "%s  %s  %s  offset=%d  nth=%d  copies=%d\n    code: %s\n    %s\n" % (
        pkg, file_, mutator, offset, nth, copies, code, reason)


def at(file_, code, offset, copy=1):
    """A locator: copy number `copy` (1 = topmost) of the lines of `file_`
    whose trimmed text is `code`, resolved when the fake gremlins runs."""
    return ("at", file_, code, offset, copy)


def _locate(root, pkg, loc):
    if isinstance(loc, str):
        return loc
    _, file_, code, offset, copy = loc
    pkg_dir = os.path.join(root, pkg.strip("./"))
    with open(os.path.join(pkg_dir, file_), "rb") as fh:
        raw_lines = fh.read().split(b"\n")
    want = code.encode("utf-8")
    seen = 0
    for i, raw in enumerate(raw_lines):
        stripped = raw.strip(b" \t\r")
        if stripped == want:
            seen += 1
            if seen == copy:
                indent = len(raw) - len(raw.lstrip(b" \t\r"))
                return "%s:%d:%d" % (file_, i + 1, indent + offset)
    raise AssertionError("fixture has no copy %d of %r in %s" % (copy, code, file_))


def gremlins_output(lived, timed_out, killed=(), timeouts_first=False):
    t = len(timed_out)
    k = 3 + t + len(killed)
    lines = ["KILLED".rjust(12) + " CONDITIONALS_NEGATION at k%d.go:1:1" % i for i in range(3 + t)]
    lines += ["KILLED".rjust(12) + " %s at %s" % (m, loc) for m, loc in killed]
    lived_lines = ["LIVED".rjust(12) + " %s at %s" % (m, loc) for m, loc in lived]
    to_lines = ["TIMED OUT".rjust(12) + " %s at %s" % (m, loc) for m, loc in timed_out]
    lines += (to_lines + lived_lines) if timeouts_first else (lived_lines + to_lines)
    lines += [
        "Mutation testing completed in 1 seconds",
        "Killed: %d, Lived: %d, Not covered: 0" % (k, len(lived)),
        "Timed out: %d, Not viable: 0, Skipped: 0" % t,
        "Test efficacy: 100.00%",
    ]
    return "\n".join(lines) + "\n"


class Result(object):
    def __init__(self, code, out, err, calls, before_run):
        self.code = code
        self.out = out
        self.err = err
        self.text = out + err
        self.calls = calls
        self.before_run = before_run


def gate(root, equivalents, plan, during=None, timeouts_first=False):
    """Run the gate hermetically. `plan` maps "./p/" to {"lived": [(mutator,
    loc)], "timed_out": [...], "killed": [...]}; `during(pkg)` edits the tree
    after the fake gremlins has read it, as a tree edited while gremlins runs."""
    eq = os.path.join(root, "equivalents.txt")
    write(root, "equivalents.txt", equivalents)
    out, err = io.StringIO(), io.StringIO()
    calls, before_run = [], []

    def runner(pkg, *args, **kwargs):
        if not calls:
            before_run.append(out.getvalue() + err.getvalue())
        calls.append(pkg)
        spec = plan[pkg]
        lived = [(m, _locate(root, pkg, loc)) for m, loc in spec.get("lived", [])]
        tos = [(m, _locate(root, pkg, loc)) for m, loc in spec.get("timed_out", [])]
        killed = [(m, _locate(root, pkg, loc)) for m, loc in spec.get("killed", [])]
        text = gremlins_output(lived, tos, killed, timeouts_first)
        if during is not None:
            during(pkg)
        return text

    code = cm.run(
        eq,
        packages=sorted(plan),
        runner=runner,
        out=out,
        err=err,
        root=root,
        free_bytes=500 * 1024 ** 3,
        cache_bytes=0,
        prepare=lambda: None,
    )
    return Result(code, out.getvalue(), err.getvalue(), calls,
                  before_run[0] if before_run else None)


HEADER = re.compile(
    r"^\s*(\S+)\s+(\S+)\s+([A-Z_]+)\s+offset=(-?\d+)\s+nth=(-?\d+)\s+copies=(-?\d+)\s*$")
CODE_LINE = re.compile(r"^\s*code: (.*)$")


def paste_suggestions(text):
    """Every (header fields, code, index of the header line) the gate printed."""
    lines = text.split("\n")
    found = []
    for i, line in enumerate(lines[:-1]):
        m = HEADER.match(line)
        c = CODE_LINE.match(lines[i + 1])
        if m and c:
            found.append((m.groups(), c.group(1), i))
    return found


def fields(text):
    return [(f, c) for f, c, _ in paste_suggestions(text)]


class Base(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.root = self._tmp.name

    def tearDown(self):
        self._tmp.cleanup()

    def entries(self, text):
        path = os.path.join(self.root, "parse-me.txt")
        write(self.root, "parse-me.txt", text)
        return cm.read_equivalents(path)

    def faulted(self, text):
        es = self.entries(text)
        return [e for e, _ in cm.entry_faults(es, root=self.root)]

    def assertPasses(self, r):
        self.assertEqual(r.code, 0, r.text)
        # D8: a clean run prints the clean-run phrase on stdout and exits 0.
        self.assertIn("zero unadjudicated survivors", r.out, r.text)

    def assertFails(self, r):
        # D8: any failure exits 1.
        self.assertEqual(r.code, 1, r.text)
        self.assertNotIn("zero unadjudicated survivors", r.out, r.text)


# ---------------------------------------------------------------- D5: format


class FormatTest(Base):
    # D5 and D15: the format with `nth` between `offset` and `copies`.
    def test_an_entry_reads_as_its_eight_fields(self):
        es = self.entries("# a comment\n\n" + entry("internal/x", "f.go", CB, FIRST, 3, CODE, nth=2))
        self.assertEqual(len(es), 1)
        e = es[0]
        self.assertEqual(
            (e.package, e.file, e.mutator, e.offset, e.nth, e.copies, e.code),
            ("internal/x", "f.go", CB, FIRST, 2, 3, CODE))
        self.assertIn(REASON, e.reason)

    # D15: "Each entry records `nth`, which copy of its code it names, counted
    # from the top of the file, and `nth` is part of the key."
    def test_a_header_without_nth_is_a_parse_error(self):
        with self.assertRaises(cm.EquivalentsError):
            self.entries("p  f.go  %s  offset=%d  copies=1\n    code: %s\n    %s\n" % (CB, FIRST, CODE, REASON))

    # D5: "An entry without its `code:` line, or without its reason, is a parse error."
    def test_an_entry_without_its_code_line_is_a_parse_error(self):
        with self.assertRaises(cm.EquivalentsError):
            self.entries("p  f.go  %s  offset=%d  nth=1  copies=1\n    %s\n" % (CB, FIRST, REASON))

    # D5: "An entry without its `code:` line, or without its reason, is a parse error."
    def test_an_entry_without_its_reason_is_a_parse_error_mid_file(self):
        text = ("p  f.go  %s  offset=%d  nth=1  copies=1\n    code: %s\n\n" % (CB, FIRST, CODE)
                + entry("p", "f.go", CB, SECOND, 1, CODE))
        with self.assertRaises(cm.EquivalentsError):
            self.entries(text)

    # D5: "An entry without its `code:` line, or without its reason, is a parse error."
    def test_an_entry_without_its_reason_is_a_parse_error_at_end_of_file(self):
        with self.assertRaises(cm.EquivalentsError):
            self.entries("p  f.go  %s  offset=%d  nth=1  copies=1\n    code: %s\n" % (CB, FIRST, CODE))

    # D5: "A comment is a line whose first character is `#`." and a missing reason is a parse error.
    def test_a_hash_in_column_one_is_a_comment_so_it_cannot_be_a_reason(self):
        text = "p  f.go  %s  offset=%d  nth=1  copies=1\n    code: %s\n# not a reason\n" % (CB, FIRST, CODE)
        with self.assertRaises(cm.EquivalentsError):
            self.entries(text)

    # D5: "A `#` anywhere else is text."
    def test_an_indented_hash_line_is_the_reason(self):
        es = self.entries(entry("p", "f.go", CB, FIRST, 1, CODE, reason="# a reason that starts with a hash"))
        self.assertEqual(len(es), 1)
        self.assertIn("# a reason that starts with a hash", es[0].reason)

    # D5: "A `#` anywhere else is text."
    def test_a_hash_inside_the_code_is_kept(self):
        code = 's := "#" + t'
        es = self.entries(entry("p", "f.go", "ARITHMETIC_BASE", code.index("+") + 1, 1, code))
        self.assertEqual(es[0].code, code)

    # D5: "A comment is a line whose first character is `#`."
    def test_comment_lines_between_entries_are_skipped(self):
        text = (entry("p", "f.go", CB, FIRST, 1, CODE) + "\n# between\n#\n\n"
                + entry("p", "f.go", CB, SECOND, 1, CODE))
        es = self.entries(text)
        self.assertEqual([e.offset for e in es], [FIRST, SECOND])

    # D5: "A Go package must be written as survivors carry it (`internal/x`, not
    # `./internal/x/`), or parsing refuses it and names the spelling to use."
    def test_a_package_spelled_as_a_path_is_refused_naming_the_spelling(self):
        for bad in ("./internal/x/", "./internal/x", "internal/x/"):
            with self.subTest(bad=bad):
                with self.assertRaises(cm.EquivalentsError) as ctx:
                    self.entries(entry(bad, "f.go", CB, FIRST, 1, CODE))
                message = str(ctx.exception).replace(bad, "<BAD>")
                self.assertIn("internal/x", message, str(ctx.exception))


# ---------------------------------------------------------------- D1: the key


class KeyTest(Base):
    # D1: "The line terminator is dropped, leading and trailing spaces, tabs and
    # CRs are removed, and the interior is kept byte for byte."
    def test_trimmed_drops_edge_spaces_tabs_and_crs_and_counts_the_indent(self):
        code, width = cm.trimmed(b" \t \tif a >  b {\t \r")
        self.assertEqual((code, width), (b"if a >  b {", 4))

    # D1: "It is the tool's column minus the width of the removed indentation."
    # D15: "`nth` is part of the key."
    def test_a_survivor_resolves_to_its_entrys_key(self):
        write(self.root, "p/f.go", f_go(inner_indent="\t \t"))
        e = self.entries(entry("p", "f.go", CB, SECOND, 1, CODE))[0]
        src = cm.package_source(self.root, "p")
        loc = _locate(self.root, "./p/", at("f.go", CODE, SECOND))
        self.assertEqual(loc, "f.go:6:18")
        self.assertEqual(cm.resolve(src, "p", loc, CB), cm.entry_key(e))

    # D15: "Each entry records `nth`, which copy of its code it names, counted
    # from the top of the file, and `nth` is part of the key."
    def test_each_copy_resolves_to_the_key_of_the_entry_naming_its_nth(self):
        write(self.root, "p/f.go", three_go())
        src = cm.package_source(self.root, "p")
        keys = [cm.resolve(src, "p", _locate(self.root, "./p/", at("f.go", CODE, FIRST, n)), CB)
                for n in (1, 2, 3)]
        self.assertEqual(len(set(keys)), 3, keys)
        for n in (1, 2, 3):
            with self.subTest(nth=n):
                e = self.entries(entry("p", "f.go", CB, FIRST, 3, CODE, nth=n))[0]
                self.assertEqual(keys[n - 1], cm.entry_key(e))

    # D1: "Re-indenting a line leaves its key unchanged, so it is still claimed."
    def test_reindenting_leaves_the_key_unchanged(self):
        keys = []
        for indent in ("\t\t", "        ", " \t   \t"):
            with self.subTest(indent=indent):
                write(self.root, "p/f.go", f_go(inner_indent=indent))
                src = cm.package_source(self.root, "p")
                loc = _locate(self.root, "./p/", at("f.go", CODE, FIRST))
                keys.append(cm.resolve(src, "p", loc, CB))
        self.assertEqual(len(set(keys)), 1, keys)
        self.assertIsNotNone(keys[0])

    # D1: "Changing whitespace inside the line, or editing a trailing comment on
    # it, changes the key and forces a re-judgment."
    def test_interior_whitespace_changes_the_key(self):
        write(self.root, "p/f.go", f_go(inner=CODE))
        k1 = cm.resolve(cm.package_source(self.root, "p"),
                        "p", _locate(self.root, "./p/", at("f.go", CODE, FIRST)), CB)
        write(self.root, "p/f.go", f_go(inner=WIDE))
        k2 = cm.resolve(cm.package_source(self.root, "p"),
                        "p", _locate(self.root, "./p/", at("f.go", WIDE, FIRST)), CB)
        self.assertNotEqual(k1, k2)

    # D4: `copies` is "the number of lines in its file whose code equals its own."
    def test_copies_counts_equal_lines_at_any_indent_and_not_longer_lines(self):
        data = f_go(inner=CODE, outer=CODE) + (
            "\nfunc h(a, b, c, d int) {\n\t" + CODE + " // trailing comment\n\t}\n"
            "\t} else " + CODE + "\n}\n").encode("utf-8")
        write(self.root, "p/f.go", data)
        self.assertEqual(cm.copies_in(cm.package_source(self.root, "p"), "f.go", CODE), 2)

    # D17: "The Go gate reads a package's source recursively, naming each file by
    # its path relative to the package."
    def test_package_source_names_subdirectory_files_by_their_relative_path(self):
        write(self.root, "p/f.go", f_go())
        write(self.root, "p/sub/x.go", "package sub\n\nfunc x(a, b int) bool {\n\t" + CODE + "\n\t}\n}\n")
        write(self.root, "p/sub/deeper/y.go", "package deeper\n\nfunc y() {}\n")
        src = cm.package_source(self.root, "p")
        self.assertEqual(sorted(src), ["f.go", "sub/deeper/y.go", "sub/x.go"])


# ------------------------------------------------- D6 / VTT-302: static check


class StaticCheckTest(Base):
    def setUp(self):
        super(StaticCheckTest, self).setUp()
        write(self.root, "p/f.go", f_go(inner=CODE, outer=CODE, outer_indent="    "))

    # VTT-302
    def test_copies_that_match_the_file_are_not_refused(self):
        self.assertEqual(self.faulted(entry("p", "f.go", CB, FIRST, 2, CODE, nth=1)), [])
        self.assertEqual(self.faulted(entry("p", "f.go", CB, FIRST, 2, CODE, nth=2)), [])

    # VTT-302
    def test_copies_below_the_count_in_the_file_is_refused(self):
        self.assertEqual(len(self.faulted(entry("p", "f.go", CB, FIRST, 1, CODE))), 1)

    # VTT-302
    def test_copies_above_the_count_in_the_file_is_refused(self):
        self.assertEqual(len(self.faulted(entry("p", "f.go", CB, FIRST, 3, CODE))), 1)

    # VTT-302
    def test_code_that_stands_nowhere_in_its_file_is_refused(self):
        self.assertEqual(len(self.faulted(entry("p", "f.go", CB, FIRST, 1, OTHER))), 1)

    # D6: "its file cannot be read"
    def test_an_entry_whose_file_cannot_be_read_is_refused(self):
        self.assertEqual(len(self.faulted(entry("p", "gone.go", CB, FIRST, 1, CODE))), 1)

    # D15: "Two entries now share a key only by duplication, and the static
    # check refuses that"
    def test_a_duplicated_entry_is_refused(self):
        two = "\n".join([entry("p", "f.go", CB, FIRST, 2, CODE, nth=1)] * 2)
        self.assertGreaterEqual(len(self.faulted(two)), 1)

    # D15: "Two entries now share a key only by duplication"
    def test_entries_on_two_copies_are_not_duplicates(self):
        both = entry("p", "f.go", CB, FIRST, 2, CODE, nth=1) + "\n" + entry("p", "f.go", CB, FIRST, 2, CODE, nth=2)
        self.assertEqual(self.faulted(both), [])

    # D15: "It also refuses an `nth` above `copies`."
    def test_an_nth_above_its_copies_is_refused(self):
        self.assertEqual(len(self.faulted(entry("p", "f.go", CB, FIRST, 2, CODE, nth=3))), 1)

    # D6: "its code starts like a comment"
    def test_code_that_starts_like_a_comment_is_refused(self):
        comments = ["// if a > b {", "/* a > b */", "* a > b"]
        body = "package p\n\nfunc k() {\n\t" + comments[0] + "\n\t" + comments[1] + \
               "\n\t/*\n\t " + comments[2] + "\n\t */\n}\n"
        write(self.root, "p/c.go", body)
        for code in comments:
            with self.subTest(code=code):
                self.assertEqual(
                    len(self.faulted(entry("p", "c.go", CB, code.index(">") + 1, 1, code))), 1)

    # D6: "the code at `offset` is not a token its mutator applies to", with
    # D6's table of tokens per mutator.
    def test_every_token_in_the_table_is_accepted_for_its_mutator(self):
        cases = [
            (CB, "_ = a <= b", "<="), (CB, "_ = a >= b", ">="),
            (CB, "_ = a < b", "<"), (CB, "_ = a > b", ">"),
            (CN, "_ = a == b", "=="), (CN, "_ = a != b", "!="),
            (CN, "_ = a <= b", "<="), (CN, "_ = a >= b", ">="),
            (CN, "_ = a < b", "<"), (CN, "_ = a > b", ">"),
            ("ARITHMETIC_BASE", "_ = a + b", "+"), ("ARITHMETIC_BASE", "_ = a - b", "-"),
            ("ARITHMETIC_BASE", "_ = a * b", "*"), ("ARITHMETIC_BASE", "_ = a / b", "/"),
            ("ARITHMETIC_BASE", "_ = a % b", "%"),
            ("INCREMENT_DECREMENT", "i++", "++"), ("INCREMENT_DECREMENT", "i--", "--"),
            ("INVERT_NEGATIVES", "_ = -a", "-"),
            ("INVERT_LOGICAL", "_ = ok && yes", "&&"), ("INVERT_LOGICAL", "_ = ok || yes", "||"),
        ]
        codes = sorted(set(c for _, c, _ in cases))
        write(self.root, "p/t.go",
              "package p\n\nfunc t() {\n" + "".join("\t" + c + "\n" for c in codes) + "}\n")
        for mutator, code, token in cases:
            with self.subTest(mutator=mutator, token=token):
                self.assertEqual(
                    self.faulted(entry("p", "t.go", mutator, code.index(token) + 1, 1, code)), [])

    # D6: "the code at `offset` is not a token its mutator applies to"
    def test_an_offset_that_is_not_its_mutators_token_is_refused(self):
        cases = [
            (CB, "_ = a == b", "=="), (CN, "_ = a + b", "+"),
            ("ARITHMETIC_BASE", "_ = a < b", "<"), ("INCREMENT_DECREMENT", "_ = a + b", "+"),
            ("INVERT_NEGATIVES", "_ = a + b", "+"), ("INVERT_LOGICAL", "_ = a != b", "!="),
            (CB, "_ = a > b", "a"), (CN, "_ = a == b", "_ ="),
        ]
        codes = sorted(set(c for _, c, _ in cases))
        write(self.root, "p/t.go",
              "package p\n\nfunc t() {\n" + "".join("\t" + c + "\n" for c in codes) + "}\n")
        for mutator, code, token in cases:
            with self.subTest(mutator=mutator, code=code, token=token):
                offset = code.index(token) + 1
                if token == "_ =":
                    offset = code.index("=") + 1
                self.assertEqual(len(self.faulted(entry("p", "t.go", mutator, offset, 1, code))), 1)

    # D1: "gremlins counts in bytes", and D6's token rule.
    def test_offset_counts_bytes_not_characters(self):
        code = 'if s != "é" && a > b {'
        write(self.root, "p/u.go", "package p\n\nfunc u() {\n\t" + code + "\n\t}\n}\n")
        byte_offset = code.encode("utf-8").index(b">") + 1
        self.assertEqual(byte_offset, code.index(">") + 2)
        self.assertEqual(self.faulted(entry("p", "u.go", CB, byte_offset, 1, code)), [])

    # D6: "its mutator is unknown (fail closed)."
    def test_an_unknown_mutator_is_refused(self):
        self.assertEqual(len(self.faulted(entry("p", "f.go", "NOT_A_MUTATOR", FIRST, 2, CODE))), 1)

    # D17: an entry names a subdirectory's file by its path relative to the package.
    def test_a_subdirectory_file_is_counted_on_its_own(self):
        write(self.root, "p/x.go", "package p\n\nfunc x(a, b int) bool {\n\tif a > b {\n\t}\n}\n")
        write(self.root, "p/sub/x.go", "package sub\n\nfunc x(a, b int) bool {\n\t\tif a > b {\n\t}\n}\n")
        self.assertEqual(self.faulted(entry("p", "x.go", CB, 6, 1, "if a > b {")), [])
        self.assertEqual(self.faulted(entry("p", "sub/x.go", CB, 6, 1, "if a > b {")), [])

    # VTT-302; D6 and D15, applied to the real file and tree; ticket Done 8:
    # "`task check` whole is green."
    def test_the_real_adjudication_file_passes_the_static_check(self):
        es = cm.read_equivalents(REAL_EQUIVALENTS)
        self.assertGreater(len(es), 0)
        self.assertEqual(cm.entry_faults(es, root=REPO), [])

    # VTT-302, D4, D6 and D15, checked by this test's own reading of the tree.
    def test_the_real_adjudication_file_agrees_with_an_independent_count(self):
        table = {
            CB: ["<=", ">=", "<", ">"], CN: ["==", "!=", "<=", ">=", "<", ">"],
            "ARITHMETIC_BASE": ["+", "-", "*", "/", "%"], "INCREMENT_DECREMENT": ["++", "--"],
            "INVERT_NEGATIVES": ["-"], "INVERT_LOGICAL": ["&&", "||"],
        }
        problems, keys = [], set()
        for e in cm.read_equivalents(REAL_EQUIVALENTS):
            with open(os.path.join(REPO, e.package, e.file), "rb") as fh:
                lines = fh.read().split(b"\n")
            want = e.code.encode("utf-8")
            n = sum(1 for raw in lines if raw.strip(b" \t\r") == want)
            if n != e.copies:
                problems.append((e.package, e.file, e.code, "copies", n, e.copies))
            if not 1 <= e.nth <= e.copies:
                problems.append((e.package, e.file, e.code, "nth", e.nth, e.copies))
            key = (e.package, e.file, e.mutator, e.offset, e.nth, e.code)
            if key in keys:
                problems.append((e.package, e.file, e.code, "duplicate"))
            keys.add(key)
            if e.code.startswith(("//", "/*", "*")):
                problems.append((e.package, e.file, e.code, "comment"))
            rest = want[e.offset - 1:]
            if not any(rest.startswith(t.encode()) for t in table[e.mutator]):
                problems.append((e.package, e.file, e.code, "token", e.offset))
        self.assertEqual(problems, [])


# --------------------------------------------------------- run: static check


class RunStaticTest(Base):
    # VTT-302; D6: "It runs before the mutation run and, like today, it is
    # reported and carried rather than returned on."
    def test_a_copies_refusal_is_reported_before_the_run_and_the_run_goes_ahead(self):
        write(self.root, "p/f.go", f_go(inner=CODE, outer=CODE))
        plan = {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST, 1)), (CB, at("f.go", CODE, FIRST, 2))]}}
        good = gate(self.root, entry("p", "f.go", CB, FIRST, 2, CODE, nth=1) + "\n"
                    + entry("p", "f.go", CB, FIRST, 2, CODE, nth=2), plan)
        self.assertPasses(good)
        bad = gate(self.root, entry("p", "f.go", CB, FIRST, 3, CODE, nth=1) + "\n"
                   + entry("p", "f.go", CB, FIRST, 3, CODE, nth=2), plan)
        self.assertFails(bad)
        self.assertEqual(bad.calls, ["./p/"])
        self.assertNotEqual(bad.before_run, good.before_run)
        self.assertGreater(len(bad.before_run), len(good.before_run))

    # D6: an entry the static check refuses is not also told to remove the entry.
    def test_a_refused_entry_is_not_also_told_to_remove_the_entry(self):
        write(self.root, "p/f.go", f_go(inner=CODE, outer=CODE))
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE), {"./p/": {}})
        self.assertFails(r)
        self.assertNotIn("remove the entry", r.text)

    # D15: the static check refuses a duplicate; D6: the run goes ahead, and a
    # refused entry is not also told to remove the entry.
    def test_a_duplicated_entry_fails_the_run_and_is_not_told_to_remove_itself(self):
        write(self.root, "p/f.go", f_go(inner=CODE, outer=CODE))
        dup = "\n".join([entry("p", "f.go", CB, FIRST, 2, CODE, nth=1)] * 2)
        r = gate(self.root, dup, {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST, 1))]}})
        self.assertFails(r)
        self.assertEqual(r.calls, ["./p/"])
        self.assertNotIn("remove the entry", r.text)

    # D15: "It also refuses an `nth` above `copies`."; D6: the run goes ahead.
    def test_an_nth_above_its_copies_fails_the_run(self):
        write(self.root, "p/f.go", f_go(inner=CODE, outer=CODE))
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 2, CODE, nth=3), {"./p/": {}})
        self.assertFails(r)
        self.assertEqual(r.calls, ["./p/"])

    # D6: "its mutator is unknown (fail closed)."; D6: the run goes ahead; D8: exit 1.
    def test_an_unknown_mutator_fails_the_run(self):
        write(self.root, "p/f.go", f_go())
        r = gate(self.root, entry("p", "f.go", "NOT_A_MUTATOR", FIRST, 1, CODE), {"./p/": {}})
        self.assertFails(r)
        self.assertEqual(r.calls, ["./p/"])

    # D5: "An entry without its `code:` line, or without its reason, is a parse
    # error."; D8: any failure exits 1.
    def test_an_unparseable_file_fails_the_run(self):
        write(self.root, "p/f.go", f_go())
        try:
            r = gate(self.root, "p  f.go  %s  offset=%d  nth=1  copies=1\n    code: %s\n" % (CB, FIRST, CODE),
                     {"./p/": {}})
        except cm.EquivalentsError as exc:
            self.fail("run raised instead of exiting 1: %s" % exc)
        self.assertFails(r)


# ------------------------------------------------------------- run: matching


class ClaimTest(Base):
    # VTT-298; D8: a clean run exits 0.
    def test_a_survivor_on_its_entrys_code_is_claimed(self):
        write(self.root, "p/f.go", f_go())
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST))]}})
        self.assertPasses(r)

    # VTT-298; ticket Done 2: "the gate exits 0 without printing `ADJUDICATION MOVED`"
    def test_a_survivor_moved_down_by_inserted_lines_is_still_claimed(self):
        write(self.root, "p/f.go", f_go(pad=7))
        loc = _locate(self.root, "./p/", at("f.go", CODE, FIRST))
        self.assertEqual(loc, "f.go:13:8")
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE), {"./p/": {"lived": [(CB, loc)]}})
        self.assertPasses(r)
        self.assertNotIn("ADJUDICATION MOVED", r.text)

    # VTT-298
    def test_a_survivor_moved_up_by_removed_lines_is_still_claimed(self):
        data = f_go().replace(b"\tx := 0\n\tfor i := 0; i < 3; i++ {\n", b"")
        write(self.root, "p/f.go", data)
        loc = _locate(self.root, "./p/", at("f.go", CODE, FIRST))
        self.assertEqual(loc, "f.go:4:8")
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE), {"./p/": {"lived": [(CB, loc)]}})
        self.assertPasses(r)

    # VTT-298; D1: "Re-indenting a line leaves its key unchanged, so it is still claimed."
    def test_a_reindented_survivor_is_still_claimed(self):
        write(self.root, "p/f.go", f_go(inner_indent="    \t  ", pad=2))
        loc = _locate(self.root, "./p/", at("f.go", CODE, SECOND))
        self.assertEqual(loc, "f.go:8:22")
        r = gate(self.root, entry("p", "f.go", CB, SECOND, 1, CODE), {"./p/": {"lived": [(CB, loc)]}})
        self.assertPasses(r)

    # D1: "The line terminator is dropped, leading and trailing spaces, tabs and
    # CRs are removed"
    def test_crlf_and_trailing_blanks_are_not_part_of_the_code(self):
        write(self.root, "p/f.go", f_go(eol="\r\n", trail=" \t "))
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST))]}})
        self.assertPasses(r)

    # D1: "gremlins counts in bytes"
    def test_a_survivor_after_a_multibyte_character_is_claimed_at_its_byte_offset(self):
        code = 'if s != "é" && a > b {'
        write(self.root, "p/f.go", f_go(inner=code))
        offset = code.encode("utf-8").index(b">") + 1
        r = gate(self.root, entry("p", "f.go", CB, offset, 1, code),
                 {"./p/": {"lived": [(CB, at("f.go", code, offset))]}})
        self.assertPasses(r)

    # VTT-299; ticket Done 3: "A survivor at an entry's position whose code is
    # not the entry's is not claimed, and the entry is reported."
    def test_a_survivor_on_other_code_at_the_entrys_old_position_is_not_claimed(self):
        write(self.root, "p/f.go", f_go(inner=CODE))
        old = _locate(self.root, "./p/", at("f.go", CODE, FIRST))
        write(self.root, "p/f.go", f_go(inner=OTHER, outer=CODE))
        self.assertEqual(_locate(self.root, "./p/", at("f.go", OTHER, FIRST)), old)
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE), {"./p/": {"lived": [(CB, old)]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertIn("no longer survives", r.text)

    # VTT-299; VTT-302 (the entry is reported); D8: "not adjudicated"
    def test_a_survivor_on_edited_code_at_the_old_position_is_not_claimed(self):
        edited = "if a >= b && c > d {"
        write(self.root, "p/f.go", f_go(inner=edited))
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", edited, FIRST))]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertEqual(len(self.faulted(entry("p", "f.go", CB, FIRST, 1, CODE))), 1)

    # VTT-299; D1: "Changing whitespace inside the line ... changes the key"
    def test_a_survivor_on_code_differing_by_interior_whitespace_is_not_claimed(self):
        write(self.root, "p/f.go", f_go(inner=WIDE, outer=CODE))
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", WIDE, FIRST))]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)

    # VTT-299; ticket Done 4: "a survivor of the entry's mutator on another
    # statement of the same file"
    def test_a_survivor_on_another_statement_of_the_file_is_not_claimed(self):
        write(self.root, "p/f.go", f_go())
        loop = "for i := 0; i < 3; i++ {"
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", loop, loop.index("<") + 1))]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertIn("no longer survives", r.text)

    # VTT-298; ticket Done 4: "a survivor of another mutator at the entry's code"
    def test_a_survivor_of_another_mutator_on_the_entrys_code_is_not_claimed(self):
        write(self.root, "p/f.go", f_go())
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CN, at("f.go", CODE, FIRST))]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertIn("no longer survives", r.text)

    # Ticket Done 4: "two mutants of one mutator on one line, as two claims"
    def test_two_mutants_of_one_mutator_on_one_line_are_two_claims(self):
        write(self.root, "p/f.go", f_go(pad=3))
        eq = entry("p", "f.go", CB, FIRST, 1, CODE) + "\n" + entry("p", "f.go", CB, SECOND, 1, CODE)
        r = gate(self.root, eq, {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST)),
                                                   (CB, at("f.go", CODE, SECOND))]}})
        self.assertPasses(r)

    # VTT-299; D1: the key holds `offset`.
    def test_an_entry_does_not_claim_the_other_mutant_on_its_line(self):
        write(self.root, "p/f.go", f_go())
        r = gate(self.root, entry("p", "f.go", CB, SECOND, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST))]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertIn("no longer survives", r.text)

    # D3: "Matching runs per package (the key holds the package)"
    def test_an_entry_does_not_claim_a_survivor_in_another_package(self):
        write(self.root, "p/f.go", f_go("p"))
        write(self.root, "q/f.go", f_go("q"))
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {}, "./q/": {"lived": [(CB, at("f.go", CODE, FIRST))]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertIn("no longer survives", r.text)

    # VTT-298; D3: "Matching runs per package (the key holds the package)"
    def test_entries_in_two_packages_claim_their_own_survivors(self):
        write(self.root, "p/f.go", f_go("p"))
        write(self.root, "q/f.go", f_go("q", pad=4))
        eq = entry("p", "f.go", CB, FIRST, 1, CODE) + "\n" + entry("q", "f.go", CB, FIRST, 1, CODE)
        r = gate(self.root, eq, {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST))]},
                                 "./q/": {"lived": [(CB, at("f.go", CODE, FIRST))]}})
        self.assertPasses(r)


# ------------------------------------------------ D15: copies and their nth


class CopiesTest(Base):
    def setUp(self):
        super(CopiesTest, self).setUp()
        write(self.root, "p/f.go", f_go(inner=CODE, outer=CODE))
        self.a = at("f.go", CODE, FIRST, 1)
        self.b = at("f.go", CODE, FIRST, 2)

    def e(self, nth):
        return entry("p", "f.go", CB, FIRST, 2, CODE, nth=nth)

    # D15: an entry on one copy never claims a mutant on another copy. The
    # judged copy's mutant is killed and the unjudged copy's survives.
    def test_one_entry_on_two_identical_guards_does_not_claim_the_unjudged_copy(self):
        r = gate(self.root, self.e(1), {"./p/": {"killed": [(CB, self.a)], "lived": [(CB, self.b)]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertIn("no longer survives", r.text)
        self.assertEqual(fields(r.text), [(("p", "f.go", CB, str(FIRST), "2", "2"), CODE)], r.text)

    # D15: an entry on one copy never claims a mutant on another copy.
    def test_an_entry_naming_the_second_copy_does_not_claim_the_first(self):
        r = gate(self.root, self.e(2), {"./p/": {"killed": [(CB, self.b)], "lived": [(CB, self.a)]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertIn("no longer survives", r.text)
        self.assertEqual(fields(r.text), [(("p", "f.go", CB, str(FIRST), "1", "2"), CODE)], r.text)

    # VTT-299; D15; ticket Done 4: "a second copy of an adjudicated line in the
    # same file: one entry claims one survivor, and the other survivor is
    # reported unadjudicated"
    def test_one_entry_claims_its_own_copy_and_the_other_copy_is_unadjudicated(self):
        r = gate(self.root, self.e(1), {"./p/": {"lived": [(CB, self.a), (CB, self.b)]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertNotIn("no longer survives", r.text)
        self.assertEqual(fields(r.text), [(("p", "f.go", CB, str(FIRST), "2", "2"), CODE)], r.text)

    # VTT-301; D15; D3: "Any entries still left over are stale."
    def test_entries_on_two_copies_and_one_survivor_leave_one_stale(self):
        r = gate(self.root, self.e(1) + "\n" + self.e(2), {"./p/": {"lived": [(CB, self.b)]}})
        self.assertFails(r)
        self.assertNotIn("not adjudicated", r.text)
        self.assertEqual(r.text.count("no longer survives"), 1, r.text)

    # VTT-298, VTT-300; D15
    def test_entries_on_two_copies_claim_survivors_on_both(self):
        r = gate(self.root, self.e(1) + "\n" + self.e(2),
                 {"./p/": {"lived": [(CB, self.a), (CB, self.b)]}})
        self.assertPasses(r)

    # D15 counted on three copies at three indentations: nth counts from the top.
    def test_entries_claim_the_copies_their_nth_names_among_three(self):
        write(self.root, "p/f.go", three_go())
        eq = "\n".join(entry("p", "f.go", CB, SECOND, 3, CODE, nth=n) for n in (1, 3))
        ok = gate(self.root, eq, {"./p/": {"lived": [(CB, at("f.go", CODE, SECOND, 1)),
                                                    (CB, at("f.go", CODE, SECOND, 3))]}})
        self.assertPasses(ok)
        r = gate(self.root, eq, {"./p/": {"lived": [(CB, at("f.go", CODE, SECOND, n)) for n in (1, 2, 3)]}})
        self.assertFails(r)
        self.assertNotIn("no longer survives", r.text)
        self.assertEqual(fields(r.text), [(("p", "f.go", CB, str(SECOND), "2", "3"), CODE)], r.text)

    # D3: "Entries left over claim up to that many timed-out mutants."; D15; VTT-301
    def test_an_entry_on_the_other_copy_claims_its_timed_out_mutant(self):
        r = gate(self.root, self.e(1) + "\n" + self.e(2),
                 {"./p/": {"lived": [(CB, self.a)], "timed_out": [(CB, self.b)]}})
        self.assertPasses(r)

    # D3: "Any entries still left over are stale."; D15; VTT-301
    def test_an_entry_left_over_after_the_timeouts_is_stale(self):
        r = gate(self.root, self.e(1) + "\n" + self.e(2), {"./p/": {"timed_out": [(CB, self.a)]}})
        self.assertFails(r)
        self.assertNotIn("not adjudicated", r.text)
        self.assertEqual(r.text.count("no longer survives"), 1, r.text)

    # D15: an entry on one copy never claims a mutant on another copy; VTT-301
    def test_a_timeout_on_the_other_copy_leaves_the_entry_stale(self):
        r = gate(self.root, self.e(1), {"./p/": {"timed_out": [(CB, self.b)]}})
        self.assertFails(r)
        self.assertIn("no longer survives", r.text)
        self.assertNotIn("not adjudicated", r.text)

    # D3: "Survivors are matched before timeouts because a timeout is not an
    # evaluation"; D15: a timeout on the unjudged copy disturbs nothing.
    def test_a_timeout_on_the_unjudged_copy_does_not_disturb_the_entrys_claim(self):
        r = gate(self.root, self.e(2),
                 {"./p/": {"lived": [(CB, self.b)], "timed_out": [(CB, self.a)]}},
                 timeouts_first=True)
        self.assertPasses(r)

    # VTT-301: an adjudication that claims a timed-out mutant does not fail.
    def test_an_entry_whose_only_mutant_timed_out_is_not_stale(self):
        r = gate(self.root, self.e(1), {"./p/": {"timed_out": [(CB, self.a)]}})
        self.assertPasses(r)

    # D3: "a timeout is not an evaluation", so it is never unadjudicated.
    def test_a_timed_out_mutant_without_an_entry_is_not_unadjudicated(self):
        r = gate(self.root, "# no entries\n", {"./p/": {"timed_out": [(CB, self.a)]}})
        self.assertPasses(r)
        self.assertNotIn("not adjudicated", r.text)

    # VTT-301; D8: "no longer survives"
    def test_an_entry_with_no_mutant_on_its_code_fails_as_stale(self):
        r = gate(self.root, self.e(1), {"./p/": {}})
        self.assertFails(r)
        self.assertIn("no longer survives", r.text)

    # VTT-300; D3's counting per key, and D15, at the unit level (claims()
    # shape taken from probing: survivors as [(key, label)]; returns
    # (claimed labels, unadjudicated, stale entries)).
    def test_claims_counts_per_key_with_nth_in_the_key(self):
        src = cm.package_source(self.root, "p")
        ka = cm.resolve(src, "p", _locate(self.root, "./p/", self.a), CB)
        kb = cm.resolve(src, "p", _locate(self.root, "./p/", self.b), CB)
        self.assertNotEqual(ka, kb)
        one = self.entries(self.e(1))
        both = self.entries(self.e(1) + "\n" + self.e(2))
        dup = self.entries(self.e(1) + "\n" + self.e(1))
        claimed, unadj, stale = cm.claims(one, [(ka, "A"), (kb, "B")], [])
        self.assertEqual((claimed, [l for _, l in unadj], len(stale)), (["A"], ["B"], 0))
        claimed, unadj, stale = cm.claims(one, [(kb, "B")], [])
        self.assertEqual((claimed, [l for _, l in unadj], len(stale)), ([], ["B"], 1))
        claimed, unadj, stale = cm.claims(both, [(kb, "B")], [])
        self.assertEqual((claimed, len(unadj), len(stale)), (["B"], 0, 1))
        claimed, unadj, stale = cm.claims(one, [(ka, "S")], [(ka, "T")])
        self.assertEqual((claimed, len(unadj), len(stale)), (["S"], 0, 0))
        claimed, unadj, stale = cm.claims(dup, [(ka, "S")], [(ka, "T")])
        self.assertEqual((len(unadj), len(stale)), (0, 0))
        claimed, unadj, stale = cm.claims(dup, [(ka, "S")], [])
        self.assertEqual((len(unadj), len(stale)), (0, 1))


# ------------------------------------------------- D17: ungated subdirectories


class SubdirectoryTest(Base):
    def setUp(self):
        super(SubdirectoryTest, self).setUp()
        write(self.root, "p/f.go", f_go())
        self.guard = "if a > b {"
        write(self.root, "p/sub/x.go",
              "package sub\n\nfunc x(a, b int) bool {\n\t\t" + self.guard + "\n\t\treturn true\n\t}\n\treturn false\n}\n")
        write(self.root, "p/sub/deeper/y.go",
              "package deeper\n\nfunc y(a, b int) bool {\n\t" + self.guard + "\n\t\treturn true\n\t}\n\treturn false\n}\n")

    # D17: "The Go gate reads a package's source recursively, naming each file by
    # its path relative to the package."
    def test_a_survivor_in_an_ungated_subdirectory_is_claimed_by_an_entry_naming_its_path(self):
        eq = entry("p", "sub/x.go", CB, 6, 1, self.guard) + "\n" + entry("p", "sub/deeper/y.go", CB, 6, 1, self.guard)
        r = gate(self.root, eq, {"./p/": {"lived": [(CB, at("sub/x.go", self.guard, 6)),
                                                   (CB, at("sub/deeper/y.go", self.guard, 6))]}})
        self.assertPasses(r)

    # D17, and VTT-298 (the entry names its file).
    def test_an_entry_naming_the_bare_file_does_not_claim_a_survivor_in_the_subdirectory(self):
        write(self.root, "p/x.go", "package p\n\nfunc x(a, b int) bool {\n\t\t" + self.guard + "\n\t}\n\treturn false\n}\n")
        r = gate(self.root, entry("p", "x.go", CB, 6, 1, self.guard),
                 {"./p/": {"lived": [(CB, at("sub/x.go", self.guard, 6))]}})
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        self.assertIn("no longer survives", r.text)

    # VTT-303 and D17: a subdirectory's file is part of the source the run measured.
    def test_a_subdirectory_file_edited_during_the_run_is_read_as_it_stood_before(self):
        def during(pkg):
            write(self.root, "p/sub/x.go", "package sub\n\n// edited\n// while\n// gremlins\n// ran\n")

        r = gate(self.root, entry("p", "sub/x.go", CB, 6, 1, self.guard),
                 {"./p/": {"lived": [(CB, at("sub/x.go", self.guard, 6))]}}, during=during)
        self.assertPasses(r)

    # D17, D19: the printed entry names the subdirectory file by its path.
    def test_an_unadjudicated_survivor_in_a_subdirectory_is_printed_with_its_path(self):
        r = gate(self.root, "# none\n", {"./p/": {"lived": [(CB, at("sub/x.go", self.guard, 6))]}})
        self.assertFails(r)
        self.assertEqual(fields(r.text), [(("p", "sub/x.go", CB, "6", "1", "1"), self.guard)], r.text)


# ------------------------------------------- D18: a column that names no code


class ColumnTest(Base):
    def setUp(self):
        super(ColumnTest, self).setUp()
        write(self.root, "p/f.go", f_go(trail="  \t"))
        self.line = int(_locate(self.root, "./p/", at("f.go", CODE, FIRST)).split(":")[1])
        self.indent = 2

    def survivor_at(self, col):
        return gate(self.root, "# none\n", {"./p/": {"lived": [(CB, "f.go:%d:%d" % (self.line, col))]}})

    # D18; VTT-304
    def test_a_column_in_the_indentation_is_unreadable(self):
        for col in (1, self.indent):
            with self.subTest(col=col):
                r = self.survivor_at(col)
                self.assertFails(r)
                self.assertEqual(fields(r.text), [], r.text)

    # D18 (boundary): the first byte of the code names code; D8 prints it.
    def test_the_first_column_of_the_code_is_readable(self):
        r = self.survivor_at(self.indent + 1)
        self.assertFails(r)
        self.assertEqual(fields(r.text), [(("p", "f.go", CB, "1", "1", "1"), CODE)], r.text)

    # D18; VTT-304
    def test_a_column_past_the_end_of_the_trimmed_line_is_unreadable(self):
        for col in (self.indent + len(CODE) + 1, self.indent + len(CODE) + 3, 80):
            with self.subTest(col=col):
                r = self.survivor_at(col)
                self.assertFails(r)
                self.assertEqual(fields(r.text), [], r.text)

    # D18 (boundary): the last byte of the code names code; D8 prints it.
    def test_the_last_column_of_the_code_is_readable(self):
        r = self.survivor_at(self.indent + len(CODE))
        self.assertFails(r)
        self.assertEqual(fields(r.text), [(("p", "f.go", CB, str(len(CODE)), "1", "1"), CODE)], r.text)


# ----------------------------------------------------------- D8, D19: messages


class MessageTest(Base):
    # D8: "An unadjudicated survivor is printed with the entry header and
    # `code:` line ready to paste, `copies` included."; plan, also ruled: "the
    # printed entry is ready to paste, with the header in column 1."; D19.
    def test_the_printed_block_pasted_verbatim_with_its_reason_written_claims_the_survivor(self):
        write(self.root, "p/f.go", three_go())
        plan = {"./p/": {"lived": [(CB, at("f.go", CODE, SECOND, 2))]}}
        r = gate(self.root, "# nothing adjudicated\n", plan)
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)
        found = paste_suggestions(r.text)
        self.assertEqual([(f, c) for f, c, _ in found],
                         [(("p", "f.go", CB, str(SECOND), "2", "3"), CODE)], r.text)
        lines = r.text.split("\n")
        i = found[0][2]
        header, code_line, placeholder = lines[i], lines[i + 1], lines[i + 2]
        self.assertEqual(header, header.lstrip(), "header not in column 1: %r" % header)
        self.assertTrue(code_line.startswith("    code: ") and code_line[4] != " ", repr(code_line))
        self.assertTrue(placeholder.startswith("    ") and placeholder[4] not in " \t", repr(placeholder))
        pasted = header + "\n" + code_line + "\n" + placeholder[:4] + REASON + "\n"
        again = gate(self.root, pasted, plan)
        self.assertPasses(again)


# ----------------------------------------------- VTT-303 / D2: source measured


class MeasuredSourceTest(Base):
    # VTT-303
    def test_code_edited_at_the_survivors_line_during_the_run_does_not_unclaim_it(self):
        write(self.root, "p/f.go", f_go(inner=CODE))

        def during(pkg):
            write(self.root, "p/f.go", f_go(inner=OTHER))

        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST))]}}, during=during)
        self.assertPasses(r)

    # VTT-303
    def test_code_edited_into_the_survivors_line_during_the_run_does_not_claim_it(self):
        write(self.root, "p/f.go", f_go(inner=OTHER, outer=CODE))

        def during(pkg):
            write(self.root, "p/f.go", f_go(inner=CODE, outer=OTHER))

        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", OTHER, FIRST))]}}, during=during)
        self.assertFails(r)
        self.assertIn("not adjudicated", r.text)

    # VTT-303
    def test_lines_inserted_above_the_survivor_during_the_run_do_not_move_it(self):
        write(self.root, "p/f.go", f_go())

        def during(pkg):
            write(self.root, "p/f.go", f_go(pad=5))

        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST))]}}, during=during)
        self.assertPasses(r)

    # VTT-303; D2: "Go reads it from the package's `.go` files, read
    # immediately before gremlins is invoked for that package."
    def test_a_file_deleted_during_the_run_is_read_as_it_stood_before(self):
        write(self.root, "p/f.go", f_go())

        def during(pkg):
            os.remove(os.path.join(self.root, "p", "f.go"))

        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST))]}}, during=during)
        self.assertPasses(r)

    # D2: "Go reads it from the package's `.go` files, read immediately before
    # gremlins is invoked for that package."
    def test_each_package_is_read_immediately_before_its_own_run(self):
        write(self.root, "p/f.go", f_go("p"))
        write(self.root, "q/f.go", f_go("q"))
        pads = {"p": 0, "q": 0}

        def during(pkg):
            # gremlins for one package edits the other package's file.
            for other in ("p", "q"):
                if "./%s/" % other != pkg:
                    pads[other] += 6
                    write(self.root, other + "/f.go", f_go(other, pad=pads[other]))

        eq = entry("p", "f.go", CB, FIRST, 1, CODE) + "\n" + entry("q", "f.go", CB, FIRST, 1, CODE)
        r = gate(self.root, eq, {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST))]},
                                 "./q/": {"lived": [(CB, at("f.go", CODE, FIRST))]}},
                 during=during)
        self.assertEqual(sorted(r.calls), ["./p/", "./q/"])
        self.assertPasses(r)


# --------------------------------------------- VTT-304 / D2: unreadable code


class UnreadableTest(Base):
    # VTT-304; D2: "A survivor whose file or line cannot be read fails the gate (fail closed)."
    def test_a_survivor_past_the_end_of_its_file_fails_the_gate(self):
        write(self.root, "p/f.go", f_go())
        r = gate(self.root, "# none\n", {"./p/": {"lived": [(CB, "f.go:99:8")]}})
        self.assertFails(r)

    # VTT-304; D2: "A survivor whose file or line cannot be read fails the gate (fail closed)."
    def test_a_survivor_in_a_file_that_does_not_exist_fails_the_gate(self):
        write(self.root, "p/f.go", f_go())
        r = gate(self.root, "# none\n", {"./p/": {"lived": [(CB, "gone.go:6:8")]}})
        self.assertFails(r)

    # VTT-304
    def test_an_unreadable_survivor_fails_even_beside_a_claimed_one(self):
        write(self.root, "p/f.go", f_go())
        r = gate(self.root, entry("p", "f.go", CB, FIRST, 1, CODE),
                 {"./p/": {"lived": [(CB, at("f.go", CODE, FIRST)), (CB, "gone.go:6:8")]}})
        self.assertFails(r)


if __name__ == "__main__":
    unittest.main()
