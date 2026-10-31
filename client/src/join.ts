// Joining a table through the shared link (SPEC-009). Keep 400 the name's
// refusal, its body shown as VTT-556 says, and 403 the door's, one message for
// every cause: a shut door, a wrong secret and a spent budget are answered
// alike (VTT-009), and a held name gets its 400 only past the door, so that 400
// tells its caller the door is open (VTT-013, VTT-546). Send no Bearer header:
// nobody joining has a token yet.

export type JoinOutcome =
  | { ok: true; token: string }
  | { ok: false; reason: "name" | "door" | "unavailable"; message: string };

const DOOR =
  "This link is not letting anyone in right now. Ask your DM to open the table, " +
  "or to send you a fresh link.";

const UNAVAILABLE = "Could not reach the table. Check your connection and try again.";

const UNREADABLE = "The table's answer could not be read. Try again in a moment.";

const NO_CREDENTIAL = "The table let you in but sent no credential. Try again in a moment.";

const NAME_FALLBACK = "That name will not work here. Try a shorter, plainer one.";

const WIRE_PREFIX = "gateway: ";

/** Strip the wire prefix. The person reading this typed into a box; they did
 *  not address a gateway, and "gateway:" tells them nothing they can act on.
 *
 *  A prefix test rather than a regex, deliberately: `/^gateway:\s*​/` has three
 *  near-identical variants that behave the same on every message the server
 *  actually sends, so the difference between them is real but unreachable —
 *  code whose correctness no example can demonstrate. startsWith says the same
 *  thing with one behaviour per branch. */
function humanReadable(body: string): string {
  const trimmed = body.trim();
  const stripped = trimmed.startsWith(WIRE_PREFIX) ? trimmed.slice(WIRE_PREFIX.length) : trimmed;
  return stripped === "" ? NAME_FALLBACK : stripped;
}

/** The reader half of the ?join= link format, and the client's ONLY reader of
 *  that parameter name. Keep it a function taking the params it reads: the
 *  writers, `vtt join-link` and the DM console's share box, are held to
 *  contract/testdata/join_url_format.json by their own tests, and this reader
 *  by join-url-format.test.ts calling it with a URL built from that fixture,
 *  never by reading source text, which Stryker rewrites. */
export function joinSecretFrom(params: URLSearchParams): string | null {
  return params.get("join");
}

/** Exchange the shared secret and a display name for this person's own token. */
export async function requestJoin(
  origin: string,
  secret: string,
  displayName: string,
): Promise<JoinOutcome> {
  let resp: Response;
  try {
    resp = await fetch(`${origin}/join`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ secret, displayName }),
    });
  } catch {
    // An unreachable server is an ANSWER, not an unhandled rejection: there is
    // a form on screen waiting to be told something.
    return { ok: false, reason: "unavailable", message: UNAVAILABLE };
  }

  if (resp.status === 400) {
    return { ok: false, reason: "name", message: humanReadable(await resp.text()) };
  }
  if (resp.status === 403) {
    return { ok: false, reason: "door", message: DOOR };
  }
  if (!resp.ok) {
    return { ok: false, reason: "unavailable", message: UNAVAILABLE };
  }

  // A 200 IS NOT YET A CREDENTIAL. Auth.set throws on an empty token, so a
  // malformed success would take the boot down with an exception rather than
  // put a message where the joiner can see it.
  //
  // The two ways that goes wrong carry DIFFERENT messages, and not merely so a
  // test can tell them apart: "the answer could not be read" and "the answer
  // carried no credential" send whoever debugs this to two different places —
  // a proxy rewriting the body, versus the endpoint itself.
  let body: { token?: unknown };
  try {
    body = (await resp.json()) as { token?: unknown };
  } catch {
    return { ok: false, reason: "unavailable", message: UNREADABLE };
  }
  if (typeof body.token !== "string" || body.token === "") {
    return { ok: false, reason: "unavailable", message: NO_CREDENTIAL };
  }
  return { ok: true, token: body.token };
}
