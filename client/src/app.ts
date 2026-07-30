// The app shell: read the token, open a session, and report status.
//
// Deliberately thin. This task (T4) owns the WIRE — connect, replay, the
// reconnect cursor, request correlation — and that logic lives in wire.ts and
// session.ts where it is unit-tested against a fake gateway. Rendering the
// board, the story feed and the action panels is T6-T8's job, and the build
// that bundles this into cmd/vtt/webdist is T5's.
//
// What is here is only the composition: which pieces talk to which, and what
// the user sees before any of them have anything to show.

import { Auth } from "./auth";
import { Session } from "./session";
import type { WireStatus } from "./wire";

function gatewayURL(): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}/ws`;
}

export function boot(root: HTMLElement): Session | null {
  const auth = new Auth(localStorage);
  const token = auth.get() ?? new URLSearchParams(location.search).get("token");
  if (!token) {
    root.textContent = "No invite token. Open the link your DM sent you.";
    return null;
  }
  // A token arriving in the URL is stored and then removed from the address
  // bar: leaving it there puts a bearer credential into browser history and
  // into the Referer of every outbound link.
  auth.set(token);
  history.replaceState(null, "", location.pathname);

  const session = new Session(gatewayURL(), token);
  const status = root.querySelector("#status");

  session.onStatus((s: WireStatus) => {
    if (status) status.textContent = s === "open" ? "connected" : s;
  });
  session.onError((e) => {
    // A fold error means the client cannot derive the true board. Say so
    // rather than rendering a plausible-looking wrong one.
    if (status) status.textContent = `cannot derive state: ${e.message}`;
  });

  void session.start();
  return session;
}

// Guarded so importing this module in a test does not require a DOM.
if (typeof document !== "undefined") {
  const root = document.getElementById("app");
  if (root) boot(root);
}
