import { defineConfig } from "@playwright/test";

// e2e is its OWN task, never part of `task check` (client spec §6): it needs
// a browser binary, which would make the default loop undownloadable on a
// fresh clone and slow on every run. It is mandatory at review and for the
// demo instead.
//
// The screenshots are not decoration — they are the remote demo medium
// (spec §6). Patrik reviews from a phone, so "here is what each role sees"
// has to be an artifact, not an instruction to go run something.
export default defineConfig({
  testDir: "./client/e2e",
  outputDir: "./client/e2e/.artifacts",
  // Sequential, but no longer because the specs share a table: each spec file
  // gets its own server and campaign (client/e2e/setup.ts). Kept at one worker
  // because several real `vtt serve` processes plus browsers is a lot of
  // machine for a suite that finishes in 25 seconds.
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"], ["html", { outputFolder: "client/e2e/.report", open: "never" }]],
  timeout: 30_000,
  use: {
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  globalSetup: "./client/e2e/global-setup.ts",
  globalTeardown: "./client/e2e/global-teardown.ts",
});
