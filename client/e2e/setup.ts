// Boot a real `vtt serve` for the e2e run: build the binary, create a
// campaign, mint one invite per role, and hand the addresses to the specs.
//
// A REAL binary rather than a stub, deliberately. These tests exist to prove
// the thing we ship works — embedded client, wire, HTTP metadata and authz
// all together — and every component seam has already been unit-tested. What
// only a browser can tell us is whether the assembled product actually opens.

import { spawnSync, spawn, type ChildProcess } from "node:child_process";
import { mkdtempSync, writeFileSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

export interface Fixture {
  base: string;
  tokens: Record<string, string>;
  /** Participant ids, needed because ACTOR control is by Actor.controller_id. */
  ids: Record<string, string>;
}

// fileURLToPath, not import.meta.dir: that is a Bun extension and these
// specs run under Playwright's Node runner.
const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = join(HERE, "..", "..");
// The real committed library. It holds adventures for more than one table
// (cellar-rats → tavern-brawl, goblin-ambush → dnd45e-minimal) and
// loadAdventuresDir serves the ones written for the served ruleset, skipping
// the rest — so this boots and offers goblin-ambush. It used to point at a
// symlinked single-ruleset fixture, because the loader refused any mismatch.
const ADVENTURES = join(REPO, "adventures");
const RULESET = join(REPO, "rulesets", "dnd45e-minimal");

const STATE = join(tmpdir(), "vtt-e2e-state.json");

function run(bin: string, args: string[]): string {
  const r = spawnSync(bin, args, { encoding: "utf8", cwd: REPO });
  if (r.status !== 0) {
    throw new Error(`${bin} ${args.join(" ")} failed: ${r.stderr || r.stdout}`);
  }
  return r.stdout.trim();
}

let servers: ChildProcess[] = [];
let binary = "";

async function startTable(): Promise<Fixture> {
  const dir = mkdtempSync(join(tmpdir(), "vtt-e2e-"));
  const campaign = join(dir, "campaign.db");

  if (!binary) {
    // ONCE for the whole run, not once per table: `go build` is the slowest
    // thing here, and the binary does not vary by campaign.
    binary = join(mkdtempSync(join(tmpdir(), "vtt-e2e-bin-")), "vtt");
    run("go", ["build", "-o", binary, "./cmd/vtt"]);
  }
  const bin = binary;

  // One invite per role. The player controls act-lera, which the adventure
  // places on the board — without a controlled actor the player panel has
  // nothing to offer and the move test would be vacuous.
  const mint = (name: string, role: string) => {
    const args = ["invite", "--campaign", campaign, "--name", name, "--role", role];
    const out = run(bin, args);
    const idm = out.match(/^participant id:\s*(\S+)$/m);
    if (!idm) throw new Error(`could not find a participant id in: ${out}`);
    // Parse the TOKEN line specifically. `vtt invite` prints two long
    // alphanumeric strings — the participant id first, then the token — so a
    // generic "find something token-shaped" match silently returns the id and
    // every request 401s with nothing pointing at the cause.
    const m = out.match(/^token[^:]*:\s*(\S+)$/m);
    if (!m) throw new Error(`could not find a token line in: ${out}`);
    return { token: m[1]!, id: idm[1]! };
  };

  // NOTE on control: `--controls` on the invite is identity-side bookkeeping.
  // The gateway authorizes a move by the ACTOR's controller_id (authz.go), and
  // the committed adventure's actors deliberately carry none — they are
  // DM-run until somebody assigns them. So the e2e has the DM create a
  // player-controlled actor through the console, which is both how a real
  // table works and better coverage than pre-seeding it would be.
  const dm = mint("DM", "dm");
  const player = mint("Lera", "player");
  const spectator = mint("Watcher", "spectator");
  const tokens = { dm: dm.token, player: player.token, spectator: spectator.token };
  const ids = { dm: dm.id, player: player.id, spectator: spectator.id };

  // Sequential from a fixed base rather than random: with one server per spec
  // file a collision is no longer a one-in-ninety curiosity, and a predictable
  // port means a failure names an address somebody can go and look at.
  const port = 8900 + servers.length;
  const proc = spawn(
    bin,
    ["serve", "--campaign", campaign, "--addr", `127.0.0.1:${port}`,
     "--ruleset", RULESET, "--adventures-dir", ADVENTURES],
    { cwd: REPO, stdio: "ignore" },
  );
  servers.push(proc);

  const base = `http://127.0.0.1:${port}`;
  let ready = false;
  for (let i = 0; i < 100; i++) {
    try {
      const r = await fetch(`${base}/healthz`);
      if (r.ok) {
        ready = true;
        break;
      }
    } catch {
      // not up yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  // FAIL HERE, LOUDLY. The loop used to fall out identically on success and on
  // a hundred failures, so a server that never bound produced a Fixture
  // pointing at a dead address — and every test then failed on
  // `.conn` never reaching "connected", an error that accuses the CLIENT of
  // something the server did. Ten seconds of waiting followed by silence is
  // the worst possible report.
  if (!ready) {
    throw new Error(
      `e2e: no server answered ${base}/healthz within 10s. If something else is ` +
        `holding that port — a stale \`vtt serve\` from an earlier run, say — kill it: ` +
        `a squatter that answers /healthz is worse than one that does not, because the ` +
        `suite then runs against somebody else's campaign.`,
    );
  }

  return { base, tokens, ids };
}

/**
 * ONE SERVER AND ONE CAMPAIGN PER SPEC FILE.
 *
 * It used to be one for the whole run, and the coupling was not survivable.
 * The campaign is an APPEND-ONLY LOG: a spec can close a session it opened,
 * but it cannot un-create an actor or un-place a token. So handover.spec.ts's
 * Ash and Bram stood on the board while table.spec.ts tried to move a player's
 * token onto ground it had every reason to think was empty, and
 * table.spec.ts's session test found "End session" where it expected "Start".
 *
 * Both failures were real and both were invisible, because `task e2e` is
 * deliberately outside `task check` (it needs a browser binary) and nobody ran
 * it between merges. The DEMO GATE — the one gate whose whole job is catching
 * features no human can reach — had itself been failing since the presence
 * branch landed.
 *
 * The binary is built once and shared; only the campaign and the server are
 * per-file. Ports come from the index rather than at random, so a failure
 * names a predictable address.
 */
export async function startFixtures(names: string[]): Promise<void> {
  const all: Record<string, Fixture> = {};
  for (const n of names) {
    all[n] = await startTable();
  }
  writeFileSync(STATE, JSON.stringify(all));
}

export function stopFixtures(): void {
  for (const s of servers) s.kill();
  servers = [];
}

/**
 * THE CONTRACT EVERY SPEC FILE STILL OWES: leave the table as you found it —
 * no open session.
 *
 * Per-file isolation covers files against EACH OTHER; it does not cover a file
 * against ITSELF. The campaign is append-only and outlives the test run, so a
 * second run of the same file — `--repeat-each`, which is how a new e2e gets
 * checked for flakiness — finds the session the first run opened and waits out
 * its timeout for a Start button that has been replaced by End. Measured:
 * --repeat-each=6 on joining.spec.ts gave 1 pass and 5 failures.
 *
 * So each spec that opens a session closes it in test.afterAll. There is no
 * helper here to do it: it is four lines at the point of use, and a helper
 * would only hide which files actually open one.
 */
export function fixture(name: string): Fixture {
  const all = JSON.parse(readFileSync(STATE, "utf8")) as Record<string, Fixture>;
  const f = all[name];
  if (!f) {
    throw new Error(
      `e2e: no table for ${name}. fixture() takes this spec file's basename, and ` +
        `global-setup starts one server per *.spec.ts it finds. Known: ${Object.keys(all).join(", ")}`,
    );
  }
  return f;
}
