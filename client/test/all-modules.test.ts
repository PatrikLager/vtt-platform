import { test, expect } from "bun:test";
import { readdirSync, lstatSync } from "node:fs";
import { join, relative } from "node:path";

// Bun's coverage reports only files that were IMPORTED during the run. A file
// no test touches does not appear at all — it contributes nothing to the
// denominator, so the headline percentage silently excludes it.
//
// When this was written that hid 660 lines across app.ts, view/dm.ts and
// view/spectator.ts: 30% of the client's source, absent from a "89.78%"
// figure that looked healthy. A coverage gate built on that number would have
// been green while a third of the code was unmeasured — the exact shape this
// repo's gates keep being found to have.
//
// So: import EVERY module. That puts their uncovered lines into the
// denominator where they belong, and fails loudly if a new file cannot even
// be loaded.

const srcDir = join(import.meta.dir, "../src");

// MUST match expected_files() in tools/check-ts-coverage.py — read the note
// there. A file this walk misses is a file no test imports, which bun then
// omits from the coverage report entirely rather than scoring 0.
const SOURCE_SUFFIXES = [".ts", ".tsx", ".mts", ".cts"];

function walk(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    // lstat, not stat: a symlinked directory is skipped, because the Python
    // side's rglob does not descend into one either. Following it here would
    // measure files the gate does not expect.
    const st = lstatSync(p);
    if (st.isSymbolicLink()) continue;
    if (st.isDirectory()) out.push(...walk(p));
    // .d.ts is a declaration file: importing it is meaningless and it has no
    // executable lines to measure.
    else if (SOURCE_SUFFIXES.some((s) => name.endsWith(s)) && !name.endsWith(".d.ts")) {
      out.push(p);
    }
  }
  return out.sort();
}

const modules = walk(srcDir);

test("every source module is importable, and therefore measured", async () => {
  expect(modules.length).toBeGreaterThan(0);
  for (const path of modules) {
    // A module that throws on import is broken for every consumer, and would
    // otherwise only be discovered in a browser.
    await import(path).catch((e) => {
      throw new Error(`${relative(srcDir, path)} failed to import: ${e}`);
    });
  }
});
