// Joining a table through the shared link (joining-a-table spec §2).
//
// The caller of this has NO credential and no way to get one except here, so
// the failure paths carry more weight than the success. The server draws the
// line for us and it is worth preserving exactly:
//
//   400 — the joiner's own input. Safe to describe, because saying "that name
//         is too long" reveals nothing about the door or the secret.
//   403 — a closed door AND a wrong secret, deliberately identical, so a
//         prober cannot learn which half they got right (spec §5). Collapsing
//         this into the 400 message would leave somebody retyping their name
//         at a door that is simply shut, with no reason to ever stop.
//
// Auth is the point of this call rather than something it carries: unlike
// metadata.ts there is no Bearer header, because nobody has a token yet.

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
