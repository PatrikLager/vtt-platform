import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, afterEach } from "bun:test";
import { requestJoin } from "../src/join";
import { renderJoinView, type JoinViewState } from "../src/view/join";

// The shared join link, client side (joining-a-table spec §2, plan J5).
//
// The person on the other end of this has NO credential yet and no way to get
// one except through this form. So the failures matter more than the success:
// a joiner who cannot tell "the DM has not opened the door" from "your name is
// too long" will retype their name until they give up.
//
// The server draws exactly that line for us and it is worth keeping: 400 is
// the joiner's own input and is safe to describe, 403 is deliberately
// identical for a closed door and a wrong secret so a prober learns nothing.

const nativeFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = nativeFetch;
});

interface Seen {
  url: string;
  method: string;
  headers: unknown;
  body: unknown;
}

/** Answer every request with one response, recording what was asked. */
function stubJoin(status: number, body: string, seen: Seen[] = []): Seen[] {
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    seen.push({
      url: String(input),
      method: init?.method ?? "GET",
      headers: init?.headers,
      body: JSON.parse(String(init?.body ?? "null")),
    });
    return new Response(body, { status });
  }) as typeof fetch;
  return seen;
}

test("joining posts the secret and the name, and comes back with a credential", async () => {
  const seen = stubJoin(200, JSON.stringify({ token: "tok-new", participantId: "p-9" }));

  const out = await requestJoin("http://table.example", "s3cret", "Kim");

  expect(out.ok).toBe(true);
  if (out.ok) expect(out.token).toBe("tok-new");
  // The REQUEST, not just the reply. A join that returned the right token
  // while posting the wrong secret would pass a reply-only assertion and fail
  // against a real server.
  expect(seen).toHaveLength(1);
  expect(seen[0]!.url).toBe("http://table.example/join");
  expect(seen[0]!.method).toBe("POST");
  expect(seen[0]!.body).toEqual({ secret: "s3cret", displayName: "Kim" });
  // The content type is declared. Our own handler does not check it, so this
  // costs nothing today — but a body posted without it is at the mercy of
  // whatever proxy or WAF sits between a table and the people joining it.
  expect(seen[0]!.headers).toEqual({ "content-type": "application/json" });
});

test("a refused name is reported as the joiner's own, and without the wire prefix", async () => {
  // 400 is the one refusal the server describes, deliberately: it is the
  // joiner's own input, so saying what is wrong leaks nothing about the door.
  stubJoin(400, "gateway: a display name is required, up to 64 ordinary characters\n");

  const out = await requestJoin("http://table.example", "s3cret", "");

  expect(out.ok).toBe(false);
  if (!out.ok) {
    expect(out.reason).toBe("name");
    // "gateway:" is wire vocabulary. The person reading this typed a name into
    // a box; they did not address a gateway.
    // EXACT, not "contains". The prefix strip, the trim and the fallback all
    // live in one small function, and a containment check cannot see a
    // trailing newline or a half-stripped prefix — which is precisely what
    // the three near-identical regexes it replaced differed by.
    expect(out.message).toBe("a display name is required, up to 64 ordinary characters");
  }

  // A body that carries no wire prefix is passed through UNTOUCHED. Without
  // this, "always strip nine characters" and "strip only when the prefix is
  // there" are the same program.
  stubJoin(400, "that name is already taken at this table");
  const plain = await requestJoin("http://table.example", "s3cret", "Kim");
  expect(plain.ok).toBe(false);
  if (!plain.ok) expect(plain.message).toBe("that name is already taken at this table");

  // A 400 whose body a proxy has eaten still has to say SOMETHING. An empty
  // error box beside a form reads as "this is broken" rather than "fix this".
  stubJoin(400, "");
  const bare = await requestJoin("http://table.example", "s3cret", "");
  expect(bare.ok).toBe(false);
  if (!bare.ok) {
    expect(bare.reason).toBe("name");
    expect(bare.message.length).toBeGreaterThan(0);
  }
});

test("a closed door is never reported as a problem with the name", async () => {
  // The trap this pins: folding 403 into the same "fix your input" message
  // would have somebody retyping their name at a door that is simply shut,
  // with no reason to ever stop.
  stubJoin(403, "gateway: this link is not accepting anyone\n");

  const out = await requestJoin("http://table.example", "wrong", "Kim");

  expect(out.ok).toBe(false);
  if (!out.ok) {
    expect(out.reason).toBe("door");
    // Both halves of the sentence. It has to say what is wrong AND what to do
    // about it: "you cannot come in" with no next step is a dead end for
    // somebody who has no other way to reach this table.
    expect(out.message).toContain("not letting anyone in");
    expect(out.message).toContain("fresh link");
  }
});

test("a success carrying no token is a failure, not a session", async () => {
  // Auth.set THROWS on an empty token, so a 200 with a missing or blank token
  // would take down the boot with an exception instead of showing the joiner
  // anything. The malformed reply has to be caught here, where there is still
  // a form to put a message on.
  // A reply that PARSED but carried nothing usable...
  for (const body of ['{"token":""}', "{}", '{"token":42}']) {
    stubJoin(200, body);
    const out = await requestJoin("http://table.example", "s3cret", "Kim");
    expect(out.ok).toBe(false);
    if (!out.ok) {
      expect(out.reason).toBe("unavailable");
      expect(out.message).toContain("no credential");
    }
  }

  // ...versus one that could not be parsed at all. Two different situations
  // and two different messages: a proxy rewriting the body, or the endpoint
  // itself. Whoever debugs this should not have to guess which.
  stubJoin(200, "not json at all");
  const unreadable = await requestJoin("http://table.example", "s3cret", "Kim");
  expect(unreadable.ok).toBe(false);
  if (!unreadable.ok) {
    expect(unreadable.reason).toBe("unavailable");
    expect(unreadable.message).toContain("could not be read");
  }
});

test("a server error is refused even when it comes with a plausible token", async () => {
  // The status is checked BEFORE the body is trusted. Without that, a 500 or a
  // 502 whose body happens to parse would hand the joiner a credential the
  // server never issued — and every later reload would dial with it and
  // report a connection problem rather than the real failure.
  stubJoin(500, JSON.stringify({ token: "tok-from-an-error-page" }));

  const out = await requestJoin("http://table.example", "s3cret", "Kim");

  expect(out.ok).toBe(false);
  if (!out.ok) {
    expect(out.reason).toBe("unavailable");
    expect(out.message).toContain("Could not reach the table");
  }
});

test("a network failure is an answer, not an unhandled rejection", async () => {
  // `as unknown as` rather than a direct assertion: `typeof fetch` carries
  // `preconnect`, and TS2352 fails the typecheck without the widening step.
  globalThis.fetch = (async () => {
    throw new TypeError("Failed to fetch");
  }) as unknown as typeof fetch;

  const out = await requestJoin("http://table.example", "s3cret", "Kim");

  expect(out.ok).toBe(false);
  if (!out.ok) {
    expect(out.reason).toBe("unavailable");
    expect(out.message).toContain("Check your connection");
  }
});

// --- the view -----------------------------------------------------------

function root(): HTMLElement {
  const el = document.createElement("div");
  document.body.replaceChildren(el);
  return el;
}

function state(over: Partial<JoinViewState> = {}): JoinViewState {
  return { name: "", busy: false, error: "", ...over };
}

test("the form asks for a name and hands back what was typed", () => {
  const r = root();
  let submitted = "";
  let typed = "";
  renderJoinView(r, state(), {
    onName: (v) => {
      typed = v;
    },
    onSubmit: () => {
      submitted = typed;
    },
  });

  const input = r.querySelector<HTMLInputElement>('[data-field="join-name"]')!;
  input.value = "Kim";
  input.dispatchEvent(new Event("input"));
  const btn = r.querySelector<HTMLButtonElement>('[data-action="join"]')!;
  expect(btn.textContent).toBe("Join");
  expect(btn.disabled).toBe(false);
  btn.click();

  expect(submitted).toBe("Kim");
  // The form names itself and names the field. A join screen with no heading
  // and no label is a bare box on a blank page.
  expect(r.querySelector(".join h1")!.textContent).toBe("Join the table");
  const label = r.querySelector<HTMLLabelElement>(".join label")!;
  expect(label.textContent!.length).toBeGreaterThan(0);
  expect(label.htmlFor).toBe(input.id);
});

test("Enter submits, because nobody reaches for the mouse after typing their name", () => {
  const r = root();
  let submits = 0;
  renderJoinView(r, state({ name: "Kim" }), { onName: () => {}, onSubmit: () => submits++ });

  const input = r.querySelector<HTMLInputElement>('[data-field="join-name"]')!;
  input.dispatchEvent(new KeyboardEvent("keydown", { key: "a" }));
  expect(submits).toBe(0); // an ordinary keystroke is not a submit
  input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
  expect(submits).toBe(1);
});

test("while the join is in flight the button cannot be pressed again", () => {
  // Every press mints a PARTICIPANT. A double-click during a slow round trip
  // would put the same person at the table twice, holding two credentials,
  // one of which nothing will ever revoke because nobody knows it exists.
  const r = root();
  let submits = 0;
  renderJoinView(r, state({ busy: true }), { onName: () => {}, onSubmit: () => submits++ });

  const btn = r.querySelector<HTMLButtonElement>('[data-action="join"]')!;
  expect(btn.disabled).toBe(true);
  // And it SAYS so. A button that is merely inert looks like a dead page;
  // this is the only feedback there is while the round trip is in flight.
  expect(btn.textContent).toBe("Joining\u2026");
  btn.click();
  expect(submits).toBe(0);
});

test("a refusal is shown, and the name the person already typed survives it", () => {
  // The repaint that carries the error also rebuilds the input. Losing the
  // value there makes every failed attempt cost the joiner their typing,
  // which is worst exactly when the door is shut and they will try again.
  const r = root();
  renderJoinView(r, state({ name: "Kim", error: "The DM has not opened this table yet." }), {
    onName: () => {},
    onSubmit: () => {},
  });

  expect(r.querySelector(".error")!.textContent).toBe("The DM has not opened this table yet.");
  expect(r.querySelector<HTMLInputElement>('[data-field="join-name"]')!.value).toBe("Kim");
});

test("with nothing wrong there is no error element to read", () => {
  // An empty error node still occupies the page and reads as a blank warning.
  const r = root();
  renderJoinView(r, state(), { onName: () => {}, onSubmit: () => {} });
  expect(r.querySelector(".error")).toBeNull();
});
