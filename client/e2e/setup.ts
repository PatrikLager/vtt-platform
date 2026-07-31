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
// Point at the SINGLE-RULESET adventures dir, not ./adventures. The committed
// adventures declare different rulesets (cellar-rats → tavern-brawl,
// goblin-ambush → dnd45e-minimal) and loadAdventuresDir rightly refuses the
// mismatch, so `serve --adventures-dir ./adventures` cannot boot at all.
const ADVENTURES = join(REPO, "scenarios", "testdata", "dnd45e-minimal-adventures");
const RULESET = join(REPO, "rulesets", "dnd45e-minimal");

const STATE = join(tmpdir(), "vtt-e2e-state.json");

function run(bin: string, args: string[]): string {
  const r = spawnSync(bin, args, { encoding: "utf8", cwd: REPO });
  if (r.status !== 0) {
    throw new Error(`${bin} ${args.join(" ")} failed: ${r.stderr || r.stdout}`);
  }
  return r.stdout.trim();
}

let server: ChildProcess | undefined;

export async function startFixture(): Promise<Fixture> {
  const dir = mkdtempSync(join(tmpdir(), "vtt-e2e-"));
  const bin = join(dir, "vtt");
  const campaign = join(dir, "campaign.db");

  run("go", ["build", "-o", bin, "./cmd/vtt"]);

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

  const port = 8900 + Math.floor(Math.random() * 90);
  server = spawn(
    bin,
    ["serve", "--campaign", campaign, "--addr", `127.0.0.1:${port}`,
     "--ruleset", RULESET, "--adventures-dir", ADVENTURES],
    { cwd: REPO, stdio: "ignore" },
  );

  const base = `http://127.0.0.1:${port}`;
  for (let i = 0; i < 100; i++) {
    try {
      const r = await fetch(`${base}/healthz`);
      if (r.ok) break;
    } catch {
      // not up yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }

  const fixture: Fixture = { base, tokens, ids };
  writeFileSync(STATE, JSON.stringify(fixture));
  return fixture;
}

export function stopFixture(): void {
  server?.kill();
}

export function fixture(): Fixture {
  return JSON.parse(readFileSync(STATE, "utf8")) as Fixture;
}
