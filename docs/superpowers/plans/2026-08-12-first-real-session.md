# First Real Session — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Play one real session of the platform with real people and produce a
ranked, evidenced backlog that decides the next arc.

**Architecture:** This plan runs an experiment, not a build. Tasks 1–2 prove the
machinery works against the same transport the guests will use; Task 3 is the
loop for anything that turns out not to; Task 4 is the session; Task 5 turns
three evidence streams into the deliverable. The expected code diff for Tasks 1,
2, 4 and 5 is zero — Task 3 is where code gets written, and only if the earlier
tasks force it.

**Tech Stack:** the `vtt` CLI (`cmd/vtt`), a cloudflared quick tunnel, an MCP
host (Claude Desktop or Claude Code) running `vtt mcp`, one computer and one
iPad, one browser each.

## Global Constraints

Copied from the spec; every task's requirements implicitly include these.

- **Findings first, then a proper fix.** Anything that must be fixed to make the
  session happen is recorded as a finding BEFORE it is fixed, and then fixed
  properly — full TDD per ADR-009, reviewed, `task check` green, integrated. No
  workarounds, no "temporary" patches. (Spec §3.)
- **The date moves, not the standard.** If a finding needs a real fix, the
  session is rescheduled. Guests are invited on that basis. (Spec §3.)
- **No new subsystems.** No autonomous DM loop, no asset service, no 4.5e
  content work, no client rework. (Spec §2.)
- **Evidence or it does not count.** A backlog item cites a sequence range, a
  transcript line, or a named person's words. Unevidenced observations go in a
  separate suspicions list. (Spec §7.)
- **Session zero runs THROUGH THE TUNNEL**, never localhost. (Spec §4.1, as
  corrected 2026-08-12.)

---

## File Structure

This plan creates no source files. It creates three artifacts:

- Create: `docs/superpowers/notes/2026-08-12-session-zero-findings.md` — the dry
  run's list. Working notes, not a spec.
- Create: `docs/superpowers/notes/2026-08-12-session-one-backlog.md` — the
  deliverable: ranked, evidenced items plus a suspicions list.
- Keep (not committed): the campaign SQLite file and the driver's MCP
  transcript. Both are evidence; neither belongs in git — the campaign holds
  participant tokens' hashes and the transcript is a personal conversation.

`docs/superpowers/notes/` does not exist yet and is created by Task 2.

---

### Task 1: Prove the transport before anything else

The one known risk, and it gates every later task. `internal/gateway/server.go`
calls `websocket.Accept(w, r, nil)`; with nil options coder/websocket applies
its default origin policy (`accept.go:239`): the browser's `Origin` host must
equal the `Host` header the server sees, with no fallback patterns. On localhost
those are always equal. Through a tunnel they are equal only if cloudflared
preserves the original `Host` — which is config-dependent and cannot be settled
by reading code.

**Files:**
- Create: none.
- Modify: none.
- Test: manual, from two devices.

**Interfaces:**
- Consumes: nothing.
- Produces: a verified `wss://<random>.trycloudflare.com` origin, an agent
  token, and a campaign file at `/tmp/vtt-session/session.db` that Tasks 2 and 4
  reuse.

- [ ] **Step 1: Build the binary and make a campaign**

```bash
cd ~/dev/vtt-platform
go build -o /tmp/vtt-session/vtt ./cmd/vtt || (mkdir -p /tmp/vtt-session && go build -o /tmp/vtt-session/vtt ./cmd/vtt)
cd /tmp/vtt-session
./vtt invite --campaign session.db --name "Agent" --role agent
```

Copy the printed token. This is the agent's credential; it is needed in Step 5.

- [ ] **Step 2: Serve the campaign**

In its own terminal, left running:

```bash
cd /tmp/vtt-session
./vtt serve --campaign session.db --addr :8080 \
  --ruleset ~/dev/vtt-platform/rulesets/dnd45e-minimal \
  --adventures-dir ~/dev/vtt-platform/adventures
```

`--ruleset` enables `use_ability` and `remove_condition`; `--adventures-dir`
enables `load_adventure`. Without both, the DM cannot run an encounter.

- [ ] **Step 3: Open the tunnel**

In a second terminal, left running:

```bash
cloudflared tunnel --url http://localhost:8080
```

It prints a `https://<random>.trycloudflare.com` URL. Record it; everything
below uses it. If `cloudflared` is not installed: `brew install cloudflared`.
Nothing is installed on any guest device — that was the reason for choosing it.

- [ ] **Step 4: Verify the upgrade from a genuinely external device**

On the **iPad, using cellular data with wifi turned off** — not the computer,
and not the iPad on the same wifi. Same-network access can succeed for reasons
that will not hold for a guest.

Open `https://<random>.trycloudflare.com/` in Safari.

Expected: the client loads. Then check the WebSocket specifically, since the
page itself is served by a plain file handler and proves nothing about the
upgrade:

```bash
# From the computer, against the tunnel rather than localhost:
curl -sS -o /dev/null -w '%{http_code}\n' https://<random>.trycloudflare.com/healthz
```

Expected: `200`.

- [ ] **Step 5: Verify the WebSocket upgrade itself**

This is the actual risk. Run the MCP server against the tunnel — it dials the
WebSocket immediately on start, so a refused upgrade shows up at once:

```bash
cd /tmp/vtt-session
./vtt mcp --server wss://<random>.trycloudflare.com/ws --token <agent-token>
```

Expected on success: the process starts and stays running (it serves MCP over
stdio and will look idle — that is correct). Press Ctrl-C to stop it.

Expected on failure: it exits with a dial error. A `403` in that error means the
origin check rejected it, which is the risk this task exists to find.

- [ ] **Step 6: Record the result either way**

If Step 5 succeeded, write one line in the findings file Task 2 creates:
"WS upgrade through cloudflared: OK, Origin/Host matched."

If Step 5 failed with a 403, **stop and go to Task 3**. Do not pass
`OriginPatterns` or disable the check to get moving — that is precisely the
workaround the global constraints forbid, and the fix is a real
allowed-origin configuration with its security reasoning written down.

- [ ] **Step 7: Commit nothing**

There is nothing to commit in this task. That is the expected outcome. Proceed
to Task 2.

---

### Task 2: Session zero — walk the whole path

Patrik and the agent, no guests, about an hour, entirely through the tunnel URL
from Task 1. Not a rehearsal of the story; a check that the machinery is real.

**Files:**
- Create: `docs/superpowers/notes/2026-08-12-session-zero-findings.md`
- Modify: none.
- Test: the walk itself. Every step either works or produces a finding.

**Interfaces:**
- Consumes: the tunnel URL, agent token and `session.db` from Task 1.
- Produces: a findings file, and a decision on which adventure session one uses.

- [ ] **Step 1: Create the findings file**

```bash
mkdir -p ~/dev/vtt-platform/docs/superpowers/notes
cat > ~/dev/vtt-platform/docs/superpowers/notes/2026-08-12-session-zero-findings.md <<'EOF'
# Session zero — findings

Dry run for the first real session (spec: 2026-08-12-first-real-session-design.md).
Working notes. Every entry is something that needed a human to notice, explain
or work around — INCLUDING things that worked but were awkward, because those
are the ones memory discards before session one.

Format per entry:
  - WHAT happened
  - WHERE (event sequence, screen, or command)
  - COST (stopped me / cost time / awkward)

## Entries

EOF
```

- [ ] **Step 2: Seat the agent in the MCP host**

Add to the MCP host's config (Claude Desktop: `claude_desktop_config.json`;
Claude Code: `.mcp.json` in the repo):

```json
{
  "mcpServers": {
    "vtt": {
      "command": "/tmp/vtt-session/vtt",
      "args": [
        "mcp",
        "--server", "wss://<random>.trycloudflare.com/ws",
        "--token", "<agent-token>",
        "--ruleset", "/Users/patriklager/dev/vtt-platform/rulesets/dnd45e-minimal",
        "--adventures-dir", "/Users/patriklager/dev/vtt-platform/adventures"
      ]
    }
  }
}
```

Restart the host. Confirm the tools appear.

- [ ] **Step 3: Have the agent load an adventure and open the door**

Prompt the agent (this is a nudge — the whole session runs on them):

> You are the DM. Load the `goblin-ambush` adventure, then open the join door
> for 3 people and tell me the join secret.

Expected: it calls `load_adventure`, then `set_join_door` with
`admitLimit: 3`, then `get_join_link`. Record in the findings file anything it
needed telling that a DM should not have to be told.

- [ ] **Step 4: Join from the iPad as a real guest would**

Build the URL by hand from the secret the agent reported:
`https://<random>.trycloudflare.com/?join=<secret>`

Open it on the iPad. Type a display name. Join.

**Time this.** From opening the link to being a seated participant. Write the
number in the findings file — spec §4.1 asks for it explicitly.

- [ ] **Step 5: Answer the handheld question**

On the iPad, with the join complete, check and record each:

- Can you read the board without pinching?
- Are the buttons hittable with a thumb?
- Does the participant list fit?
- Does anything overflow horizontally?

Every "no" is an entry. This has never been tested on a real handheld.

- [ ] **Step 6: Promote, grant, and act**

Prompt the agent:

> Somebody joined. Find out who, promote them to player, and give them an actor
> from the adventure.

Expected: `get_participants`, `promote_participant`, `grant_actor_control`.
Then, on the iPad, move the granted actor's token.

Record how many nudges this took. Two deliberate DM acts per player is the
design (spec §4.2); what is being measured is what it costs live.

- [ ] **Step 7: Break the connection on purpose**

On the iPad, turn cellular off for ten seconds, then on. Reload if needed.

Expected: the client reconnects manually (reconnection is manual by design —
client spec §3.4) and the board is intact. Record what the experience was: did
it say anything useful, or did it just look broken?

- [ ] **Step 8: Commit the findings file**

```bash
cd ~/dev/vtt-platform
git add docs/superpowers/notes/2026-08-12-session-zero-findings.md
git commit -m "Session zero findings"
```

No gate is required: this file is working notes, not code and not documentation
of code.

- [ ] **Step 9: Decide the walk's verdict**

Session zero is **done when the whole path completed without a workaround**.

- If nothing broke: choose the adventure session one uses (whichever ran
  cleanly) and go to Task 4.
- If anything broke or needed a workaround: go to Task 3, then repeat this task
  from Step 3.

---

### Task 3: Fix what session zero found — properly

Entered only when Task 1 or Task 2 produced something that blocks the session.
This task is a normal development cycle and obeys every ordinary rule.

**Files:**
- Create/Modify: unknown until a finding exists. Whatever the finding names.
- Test: per ADR-009 — a failing test first, behavioural RED where a stub can
  compile.

**Interfaces:**
- Consumes: one entry from the session zero findings file.
- Produces: a merged fix, and a findings entry updated to say what was done.

- [ ] **Step 1: Confirm the finding is recorded BEFORE any fix**

Check the finding is already written in
`docs/superpowers/notes/2026-08-12-session-zero-findings.md`. If it is not,
write it now, before touching code.

This ordering is the point of the constraint: a fix applied first and recorded
afterwards is a fix whose motivation is reconstructed from memory, and the
session then measures a platform that was quietly repaired.

- [ ] **Step 2: Reproduce it as a failing test**

Write the test in the package the finding names. Run it and watch it fail for
the RIGHT reason — read the failure message and confirm it describes the
finding, not a typo in the test.

```bash
cd ~/dev/vtt-platform
go test ./internal/<package>/ -run <TestName> -count=1 -v
```

Expected: FAIL, with a message that would make sense to someone who had not
seen the session.

- [ ] **Step 3: Fix it**

Minimal code to make the test pass. Integrated into the engine — not a flag, not
a special case for the tunnel, not a workaround with a comment promising to do
it properly later.

- [ ] **Step 4: Run the affected package, then the gate**

```bash
go test ./internal/<package>/ -count=1
task check
```

Expected: package green, then `task check` exit 0. Read the exit code from
`task check` directly; do not pipe it through `tail`, which reports the pipe's
status and not the gate's.

- [ ] **Step 5: Review before committing**

Dispatch a code reviewer over the working-tree diff. Apply what it finds. The
repo's pre-commit hook enforces that this happened.

- [ ] **Step 6: Commit, push, merge**

```bash
git checkout -b fix/<short-name>
git add -A
CLAUDE_REVIEW_DONE=1 git commit -F <message-file>
git push -u origin fix/<short-name>
gh pr create --base main --head fix/<short-name> --title "<title>" --body-file <body-file>
gh pr merge --merge --delete-branch
```

- [ ] **Step 7: Update the findings entry and reschedule if needed**

Append to the finding: what the fix was, and its commit. If the fix took long
enough that the session date is now wrong, move the date — the standard does not
move.

Return to Task 2, Step 3, and walk the path again.

---

### Task 4: Session one

The LLM DMs. Patrik and friends play. Ninety minutes, one encounter.

**Files:**
- Create: none during play.
- Modify: none during play.
- Test: the session is the test.

**Interfaces:**
- Consumes: a green session zero, the chosen adventure, the tunnel.
- Produces: the campaign file, the driver's transcript, and debrief answers —
  the three evidence streams Task 5 consumes.

- [ ] **Step 1: Start fresh, an hour before guests arrive**

A new campaign, so the session's event log contains only the session:

```bash
cd /tmp/vtt-session
mv session.db session-zero.db   # keep it; it is session zero's evidence
./vtt invite --campaign session.db --name "Agent" --role agent
./vtt invite --campaign session.db --name "Patrik" --role player
```

**Save both tokens somewhere you will still have after the session.** The agent
token goes in the MCP config; the player token is what Task 5 uses to read the
log back, and there is no way to recover a token from the campaign afterwards —
only its hash is stored.

Then re-run Task 1 Steps 2–3 (serve, tunnel) and Task 2 Step 2 (MCP config)
against the new campaign and the new tunnel URL. **The tunnel URL is different
every run** — the MCP config and the join link both need the new one.

- [ ] **Step 2: Re-verify the transport, briefly**

Repeat Task 1 Step 5 against the new tunnel. Thirty seconds, and it catches the
one failure mode that would waste everybody's evening.

- [ ] **Step 3: Open the door for the real number**

Prompt the agent:

> Load `<the adventure session zero chose>`, open the join door for <N> people,
> and give me the link to send.

Where `<N>` is the number of guests. The admission budget is per opening; if the
door needs reopening later, it gets a fresh budget.

- [ ] **Step 4: Send the link and start the clock**

Send `https://<random>.trycloudflare.com/?join=<secret>` to the guests.

Note the wall-clock time. Note it again when the last player is seated and
holding an actor. That interval is a measurement, not a nuisance.

- [ ] **Step 5: Play, and nudge**

Drive the DM by prompting it between beats. Do not correct the platform mid-game
and do not explain away its gaps to the guests — narrate around them if you must,
but let them happen. The nudges are the deliverable.

**Do not take notes during play.** The event log and the transcript are already
recording. Notes cost attention that the session needs.

- [ ] **Step 6: If it stalls badly, end early**

If the agent stalls so often that the table cannot play, stop the session and
say so plainly. That is a finding, not a failure — and puppeteering it into
looking successful destroys the evidence the evening exists to produce.

- [ ] **Step 7: Debrief at the table, five minutes, before anyone leaves**

Ask, and write down the answers verbatim with names:

1. What confused you?
2. What were you waiting for?
3. What did you try to do that you couldn't?
4. Would you play again?

A form sent tomorrow gets no answers. Question 3 is the sharpest — it surfaces
missing capability rather than defects.

**Weight the guests' answers over your own** (spec §5). You drove the DM, so you
were behind the curtain all evening: you know why it paused, what it was about
to do, and which awkwardness was the platform rather than the story. That
knowledge makes your verdict on the EXPERIENCE unreliable in a way theirs is
not. Your evidence is the nudges; theirs is whether it was any good.

- [ ] **Step 8: Save the evidence before closing anything**

```bash
cd /tmp/vtt-session
cp session.db session-one-$(date +%Y%m%d).db
```

Export or copy the MCP conversation to a file. **Do not close the MCP host
window before doing this.** The transcript is the single most valuable artifact
in the arc and it is the only one not written to disk automatically.

---

### Task 5: The backlog

Turn three evidence streams into the deliverable.

**Files:**
- Create: `docs/superpowers/notes/2026-08-12-session-one-backlog.md`
- Modify: none.
- Test: every item cites evidence, or it is not in the backlog.

**Interfaces:**
- Consumes: the campaign file, the transcript, the debrief answers.
- Produces: the ranked backlog that decides the next arc.

- [ ] **Step 1: Read the event log back**

`state dump` and `events tail` both read **over the wire**, not from the file —
they take `--server` and `--token`, never `--campaign`. So the saved campaign is
re-served locally just to be read. No tunnel is needed; this is only for you.

```bash
cd /tmp/vtt-session
./vtt serve --campaign session-one-<date>.db --addr :8099 \
  --ruleset ~/dev/vtt-platform/rulesets/dnd45e-minimal \
  --adventures-dir ~/dev/vtt-platform/adventures &
SERVE_PID=$!

# Every envelope, in order, as protojson lines. Ctrl-C once it stops printing —
# tail streams and then waits for more.
./vtt events tail --server ws://localhost:8099/ws --token <patrik-token> --after 0 \
  > /tmp/vtt-session/session-one-events.jsonl

# The final board, for context on what the log was building toward.
./vtt state dump --server ws://localhost:8099/ws --token <patrik-token> \
  > /tmp/vtt-session/final-state.json

kill $SERVE_PID
```

`<patrik-token>` is the player token minted in Task 4 Step 1 — which is why that
step says to save it.

Look for the timing evidence in the JSONL: where sequence numbers cluster (fast
play) and where they gap (people waiting). A gap with a wall-clock span is the
citation for a "cost time" item.

- [ ] **Step 2: Read the transcript for nudges**

Every prompt given to the DM is a nudge. Group them: which were storytelling
(a DM's normal job) and which were the agent failing to notice something it
should have? Only the second kind is evidence.

- [ ] **Step 3: Write the backlog**

```bash
cat > ~/dev/vtt-platform/docs/superpowers/notes/2026-08-12-session-one-backlog.md <<'EOF'
# Session one — backlog

Output of the first real session (spec: 2026-08-12-first-real-session-design.md).
Every item cites evidence: a sequence range, a transcript line, or a named
person's words. Ranked by cost to the table.

## Stopped play

## Cost time

## Awkward

## Suspicions — NOT evidenced, NOT actionable

Things believed rather than measured. Recorded so they are not lost, and kept
out of the backlog so they are not acted on. Promote one only when evidence
turns up.

EOF
```

Fill each section from the three streams. An item with no citation goes under
Suspicions, however confident it feels.

- [ ] **Step 4: Rank within each section**

Order by how many people it affected and how long it lasted. A thing that
stopped one player for the whole session outranks a thing that briefly confused
everybody.

- [ ] **Step 5: Commit the backlog**

```bash
cd ~/dev/vtt-platform
git add docs/superpowers/notes/2026-08-12-session-one-backlog.md
git commit -m "Session one backlog: what the first real table found"
git push origin main
```

- [ ] **Step 6: Choose the next arc from the backlog**

The arc is chosen from what the table found, not from the founding spec's list
of candidates (asset service, 4.5e ruleset, voice pipeline). If the backlog
happens to point at one of those, it points at it with evidence — which is the
whole reason this arc came first.

---

## Exit criteria

- Session zero's walk completed without a workaround.
- Session one was played, or abandoned for a recorded reason.
- The backlog exists, every item evidenced, ranked by cost to the table.
- The campaign file and the driver transcript are kept.
- The next arc is chosen from the backlog.
