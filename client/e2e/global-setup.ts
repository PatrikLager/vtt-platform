import { readdirSync } from "node:fs";
import { dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { startFixtures } from "./setup";

// One table per spec file, discovered rather than listed: a new spec that
// forgot to register itself would otherwise get a confusing "no table for ..."
// on its first run, and the fix would be to edit a file nobody thinks to look
// at. See setup.ts's startFixtures for why they are no longer shared.
export default async function globalSetup(): Promise<void> {
  const here = dirname(fileURLToPath(import.meta.url));
  const names = readdirSync(here)
    .filter((f) => f.endsWith(".spec.ts"))
    .map((f) => f.replace(/\.spec\.ts$/, ""))
    .sort();
  await startFixtures(names);
  // eslint-disable-next-line no-console
  console.log(`e2e: ${names.length} table(s): ${names.join(", ")}`);
}
