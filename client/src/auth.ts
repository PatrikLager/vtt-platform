// Token storage.
//
// # Trust posture
//
// The invite token lives in localStorage: browser-local, per-origin, and
// readable by any script running on that origin. That is the SAME posture the
// MCP server's token env var already has (client spec §3) — the token is a
// bearer credential and whoever holds it is the participant.
//
// What that means concretely, so nobody has to infer it:
//   * A token in localStorage survives a tab close. Logging out must remove
//     it, not merely navigate away.
//   * Any XSS on this origin is a full account compromise. There is no
//     httpOnly cookie here to fall back on, because the token also has to
//     reach a WebSocket handshake, which cannot carry custom headers.
//   * Tokens are revocable server-side (identity.Revoke), which is the real
//     mitigation: a leaked token is cut off centrally rather than waited out.
//
// Storage is injected rather than reached for globally so this is testable
// without a DOM, and so a future embedder can supply sessionStorage instead.

export interface TokenStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

const KEY = "vtt.token";

export class Auth {
  constructor(private readonly store: TokenStore) {}

  get(): string | null {
    const t = this.store.getItem(KEY);
    // An empty string is not a usable token, and treating it as one would
    // send `Bearer ` and get a 401 that looks like a server problem.
    return t === null || t === "" ? null : t;
  }

  set(token: string): void {
    if (token === "") {
      throw new Error("auth: refusing to store an empty token");
    }
    this.store.setItem(KEY, token);
  }

  /** Forget the token. Closing the tab does NOT do this. */
  clear(): void {
    this.store.removeItem(KEY);
  }
}

/** In-memory store, for tests and for embedders without a DOM. */
export function memoryStore(): TokenStore {
  const m = new Map<string, string>();
  return {
    getItem: (k) => m.get(k) ?? null,
    setItem: (k, v) => void m.set(k, v),
    removeItem: (k) => void m.delete(k),
  };
}
