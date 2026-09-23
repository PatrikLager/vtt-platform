# The process is adopted: the tree is committed and the register has one id scheme

## The problem

`docs/requirements.md` declares `project: VTT` and a `| Id | Requirement | Verified by |`
table, and its fourteen rows carry ids of the shape `VTT-VIS-017`, `VTT-LOG-002`,
`VTT-JOIN-001`: a tag, a subject segment and a number. The dispenser the installed
dev-cycle package ships, `requirement-id`, allocates a tag and a number only, reads
rows matching `^VTT-[0-9]+$`, and run against a copy of this register allocates
`VTT-001`, counting none of the fourteen. The file's own rule, that an id is allocated
by `requirement-id` and never chosen by hand, does not describe its contents. Three rows
name files in `internal/perceive`, a package that does not exist in the tree, and one
row's evidence cell is a sentence pointing at a section below the table rather than a
path.

`docs/verification-debt.md`, under `Open debt`, says there is deliberately no register,
states a hand rule for the next number, and holds two commands that check the citation
chain for the `VIS` and `LOG` series only; no task in `Taskfile.yml` runs either.
`docs/superpowers/specs/2026-08-18-visibility-design.md` carries eight
`[VTT-VIS-NNN]` tags, five further inline ids, and four paragraphs ruling that an id is
assigned by whoever first needs to cite it. `docs/adr/011-identity-and-authorization.md`
names `JOIN-001` to `JOIN-004` in its closing paragraph, and
`docs/reports/2026-08-18-visibility.md` names `VIS` ids on two lines. Nothing under
`internal/`, `cmd/`, `client/` or `tools/` cites any id.

`CLAUDE.md` says `docs/verification-debt.md` "holds no such commands" and that the
register's rows "cite real test names". The first is false because the two commands are
there; the second because three rows name a package that is not.
`docs/specifications/007-the-wire-contract.md` says the register carries no rows.

`git status` on `chore/adopt-the-process` lists eleven uncommitted entries:
`.lefthook.yml`, whose `review-gate` command chains the package's `githooks/pre-commit`;
`CLAUDE.md`; the visibility ticket; `docs/verification-debt.md`, intent-to-add;
`tools/mutation-equivalents.txt`; `.claude/`; `docs/ADOPTING-THE-PROCESS.md`; ADR-011;
`docs/reports/`, holding eight arc reports and `DERIVED-contract-experiment.md`;
`docs/requirements.md`; and `docs/specifications/`. A search of the review-record store
for this repository's path finds no record later than 2026-09-17, so the documents dated
2026-09-19 and later have no recorded reading. `docs/ADOPTING-THE-PROCESS.md` describes
itself as a handoff and not a specification; `DERIVED-contract-experiment.md` says it
carries no SPEC number. The branch has no upstream ref.

Two id schemes, a register that contradicts its dispenser, citations that resolve to
nothing, and false sentences in the governing document: every measurement taken against
this tree is ambiguous until it is resolved. The chain between register, tests and
specifications, and the gate that checks it, are the first arc's ticket, not this one:
a gate needs rows to hold, and the rows come from the arcs.

## Done looks like

1. `git status --short` on `chore/adopt-the-process` prints nothing, and
   `git rev-parse --verify origin/chore/adopt-the-process` succeeds. Today: eleven
   entries, and no such ref.
2. `grep -rnE --exclude='2026-09-23-adopting-the-process*' '\bVTT-[A-Z]+-[0-9]{3}\b|(^|[^A-Z-])(VIS|LOG|JOIN)-[0-9]{3}\b' docs CLAUDE.md tools`
   prints nothing; the exclusion keeps this ticket, its plan and its report out, and
   those three name the old forms by shape only. Today: 38 lines carrying the prefixed
   form in four files, and 7 occurrences of the bare form on six lines in two.
3. `docs/requirements.md` holds `project: VTT`, the `| Id | Requirement | Verified by |`
   header and no row, and `requirement-id` run against a copy of the file allocates
   `VTT-001`. Today: fourteen rows of the old shape beside the same allocation.
4. `docs/superpowers/specs/2026-08-18-visibility-design.md` contains no `VTT-` and no
   passage on how ids are assigned, and still contains each of these sentences once:
   "A refusal of sight is not a refusal of the roster", "Exactly one code path introduces
   an actor", "The actor roster is projected too", "One rule, both call sites",
   "Creatures are pure line of sight", "Wall tiles and closed door tiles", "An open door
   blocks nothing", "exhaustive over the". Today: eight tags, five inline ids and four
   paragraphs, and each sentence once.
5. `docs/verification-debt.md` has no passage about a register or a next number and no
   command hardcoding a subject series, and has an entry labelled `test asserts nothing`
   naming `TestAClosedDoorSpendsNothing` and `TestARefusedJoinWritesNothingAtAll` and the
   shut-door refusal's write that neither can see. Today: the passage and the commands are
   there, and that finding sits in the register instead.
6. `grep -c 'holds no such commands' CLAUDE.md` and `grep -c 'fourteen rows' CLAUDE.md`
   both print 0, and the register entry in `CLAUDE.md` names the tag, the header and the
   dispenser. Today: 1 and 1.
7. `docs/ADOPTING-THE-PROCESS.md` and `docs/reports/DERIVED-contract-experiment.md` do
   not exist. Today: both do.
8. Still refused afterwards: with a staged file and no matching review record,
   `git commit` exits non-zero through lefthook's `review-gate`, with and without
   `CLAUDE_REVIEW_DONE=1`. Holds today, observed with
   `lefthook run pre-commit --command review-gate` in a scratch repository with a staged
   file.
9. `docs/specifications/` holds a record of how requirement ids are allocated and cited
   in this repository, whose Status names the chain check as the half not yet
   implemented and says that no ticket carries it yet. Today: no such record.

## Rules this puts on the system

This ticket adds no rule. The three rules of the citation chain and the rule the
commit gate holds travel to the first arc's ticket, together with the gate that will
hold them; a rule allocated here would sit in a register nothing checks.

## What it touches

In the order they must change:

1. `docs/requirements.md`, `docs/verification-debt.md`,
   `docs/superpowers/specs/2026-08-18-visibility-design.md`,
   `docs/adr/011-identity-and-authorization.md`, `docs/reports/2026-08-18-visibility.md`,
   `CLAUDE.md`, `docs/specifications/007-the-wire-contract.md` and
   `contract/README.md`, which points at that record instead of restating it. The
   ids leave together, so that no committed tree cites an id its register does not
   define.
2. `tools/mutation-equivalents.txt`, `.lefthook.yml`, `.claude/settings.json` and the
   seven other files under `docs/reports/`: committed as they are.
3. A new record under `docs/specifications/`; then `docs/ADOPTING-THE-PROCESS.md` and
   `docs/reports/DERIVED-contract-experiment.md` removed.

Twenty existing files and one new one, across the documents and the hooks.

## Specifications this moves

docs/specifications/007-the-wire-contract.md
New: how requirement ids are allocated and cited in this repository

## What could not be established

- Whether the eight arc reports under `docs/reports/` and ADR-011 were read by anyone
  before this ticket. The review-record store holds nothing for this repository later
  than 2026-09-17 and the files are dated 2026-09-19. They are committed as the accounts
  of their periods, unrevised.
- Whether the shut-door refusal in `internal/identity` writes on its refusal path. The two
  tests that aim at it cannot see it: one reads only the effect, the other never reaches
  the guard. The entry this ticket moves into `docs/verification-debt.md` records that;
  closing it is not this ticket's.
- Which tests hold the visibility and joining rules once their ids are gone. The
  sentences stay in their tickets, and each arc's own ticket re-derives its requirements
  from its exit criteria and allocates ids then. This ticket allocates none.
