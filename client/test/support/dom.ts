// A DOM for the view tests, without breaking the tests that use real I/O.
//
// Two hazards, both hit in practice:
//
//   1. bun runs every test file in ONE process, so registering happy-dom
//      anywhere registers it everywhere. Importing this module twice threw
//      "Happy DOM has already been globally registered" and failed the run.
//   2. The registrator replaces global `fetch` and `WebSocket` with its own
//      simulations. metadata.test.ts and wire.test.ts drive a REAL Bun.serve;
//      handing them a simulation swapped the thing under test for a model of
//      it, and they failed with parse errors.
//
// So: register once, then hand the real networking back. The view tests need
// document/HTMLElement, not a fake network.
//
// # Why a list, and why THIS list
//
// GlobalRegistrator.register() copies every own property of a GlobalWindow
// onto globalThis — about 30 names, against an internal ignore list of five.
// A review measured it: the first version of this file restored FIVE and left
// 25 replaced, including AbortController/AbortSignal, Blob, File, FormData
// and URL, every one of which the real-network tests touch. It passed only
// because none of them had yet been handed to Bun's fetch in a way it
// type-checks. That is a latent break waiting on a dependency bump, and it
// would surface as a mysterious failure in a test file that never mentions
// happy-dom.
//
// The split is deliberate and not symmetric:
//
//   RESTORED — networking and data-transfer types. A real Bun.serve must
//   receive Bun's own objects.
//
//   LEFT AS HAPPY-DOM'S — Event, EventTarget, MessageEvent, CloseEvent,
//   navigator and the timers. The view tests dispatch `new Event("input")` at
//   happy-dom elements and rely on its scheduler; restoring those natives
//   would break the DOM this module exists to provide.

import { GlobalRegistrator } from "@happy-dom/global-registrator";

declare global {
  // eslint-disable-next-line no-var
  var __vttDomRegistered: boolean | undefined;
}

/** Globals the real-network tests need to be Bun's, not simulations. */
export const NETWORK_GLOBALS = [
  "fetch", "WebSocket", "Response", "Request", "Headers",
  "AbortController", "AbortSignal", "FormData", "Blob", "File", "URL",
] as const;

const g = globalThis as unknown as Record<string, unknown>;

/** What each name was BEFORE happy-dom touched it. */
const natives = new Map<string, unknown>();

/** Names happy-dom actually replaced. Empty would mean the list is stale. */
const replaced: string[] = [];

if (!globalThis.__vttDomRegistered) {
  for (const name of NETWORK_GLOBALS) natives.set(name, g[name]);

  GlobalRegistrator.register();

  for (const [name, value] of natives) {
    if (g[name] !== value) replaced.push(name);
    g[name] = value;
  }

  globalThis.__vttDomRegistered = true;
}

/** Names in NETWORK_GLOBALS that are NOT the value captured before
 *  registration. Non-empty means a restore was missed. */
export function unrestored(): string[] {
  return NETWORK_GLOBALS.filter((n) => natives.has(n) && g[n] !== natives.get(n));
}

/** Names happy-dom replaced and this module put back. */
export function restored(): string[] {
  return [...replaced];
}
