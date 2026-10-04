// Independent QA for a move's reason in the client's labels. Derived from
// VTT-263 and the problem paragraph and fourth rule of
// docs/superpowers/specs/2026-10-03-a-moves-reason-reaches-the-log-design.md;
// written without reading spectator.ts's body.

import "./support/dom";
import { describe as group, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  GridPositionSchema,
  TokenMovedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { describe, renderSpectator } from "../src/view/spectator";
import { newState } from "../src/state";

interface MoveOpts {
  seq?: bigint;
  tokenId?: string;
  reason?: string;
  role?: string;
  noTo?: boolean;
}

// From and to differ, and neither equals the token id or any reason, so a
// label that names the wrong field cannot pass by coincidence.
function move(o: MoveOpts = {}): Envelope {
  const seq = o.seq ?? 1n;
  return create(EnvelopeSchema, {
    eventId: `ev-${seq}`,
    sequence: seq,
    actorRole: o.role ?? "dm",
    participantId: "part-1",
    payload: {
      case: "tokenMoved",
      value: create(TokenMovedSchema, {
        tokenId: o.tokenId ?? "tok-a",
        sceneId: "scene-1",
        from: create(GridPositionSchema, { x: 1, y: 2 }),
        ...(o.noTo ? {} : { to: create(GridPositionSchema, { x: 7, y: 5 }) }),
        ...(o.reason !== undefined ? { reason: o.reason } : {}),
      }),
    },
  });
}

const BASE = "tok-a moved to 7,5";

function render(log: Envelope[]): HTMLElement {
  const root = document.createElement("div");
  document.body.appendChild(root);
  renderSpectator(root, newState(), log, "connected");
  return root;
}

function feedBeats(root: HTMLElement): string[] {
  const feed = root.querySelector("section.feed");
  if (!feed) throw new Error("no feed section rendered");
  return Array.from(feed.querySelectorAll(".beat")).map((b) => b.textContent ?? "");
}

// The ticker line for one sequence number, with its "#N" marker removed.
function tickFor(root: HTMLElement, seq: number): string {
  const ticker = root.querySelector("section.ticker");
  if (!ticker) throw new Error("no ticker section rendered");
  for (const t of Array.from(ticker.querySelectorAll(".tick"))) {
    const marker = t.querySelector(".seq")?.textContent ?? "";
    if (marker.trim() === `#${seq}`) {
      return (t.textContent ?? "").slice(marker.length).trim();
    }
  }
  throw new Error(`no ticker line for #${seq}`);
}

function occurrences(hay: string, needle: string): number {
  return hay.split(needle).length - 1;
}

group("describe: a move's label and its reason", () => {
  // VTT-263
  test("a move whose frame carries a reason is labelled with it", () => {
    const label = describe(move({ reason: "toward the window" }));
    expect(label).toContain("toward the window");
  });

  // VTT-263
  test("the reason is shown once, beside the token and destination the label already named", () => {
    const label = describe(move({ reason: "toward the window" }));
    expect(label).toContain(BASE);
    expect(occurrences(label, "toward the window")).toBe(1);
  });

  // The design doc named in this file's header, its problem paragraph: "The
  // client's feed and ticker label a move with `describe` in
  // client/src/view/spectator.ts, `"<token> moved to x,y"`" — a move without
  // a reason keeps exactly that label.
  test("a move with no reason is labelled exactly as before", () => {
    expect(describe(move())).toBe(BASE);
    expect(describe(move({ tokenId: "tok-b" }))).toBe("tok-b moved to 7,5");
  });

  // VTT-263
  test("an empty reason is no reason: proto3 sends nothing, so the label is unchanged", () => {
    expect(describe(move({ reason: "" }))).toBe(BASE);
  });

  // VTT-263
  test.each([
    ["non-latin text", "vers la fenêtre — 窓へ, ✓"],
    ["quotes and backslashes", `he said "go" \\ 'now'`],
    ["replacement patterns", "costs $& and $1 and $$ and ${x}"],
    ["format directives", "%s %d {0} {{name}}"],
    ["html entities", "&amp; &lt;b&gt; &#60;"],
    ["a prototype member name", "__proto__"],
    ["an embedded newline", "line one\nline two"],
  ])("free text with %s is shown verbatim", (_name, reason) => {
    expect(describe(move({ reason }))).toContain(reason);
  });

  // VTT-263
  test("a long reason is shown whole, not cut", () => {
    const reason = "a".repeat(250) + "|middle|" + "z".repeat(250) + "|end";
    expect(describe(move({ reason }))).toContain(reason);
  });

  // VTT-263
  test.each(["dm", "agent", "player"])(
    "the reason is shown whatever role issued the move (%s)",
    (role) => {
      expect(describe(move({ role, reason: "toward the window" }))).toContain(
        "toward the window",
      );
    },
  );

  // VTT-263
  test("a frame carrying a reason but no destination is still labelled with the reason", () => {
    expect(describe(move({ noTo: true, reason: "toward the window" }))).toContain(
      "toward the window",
    );
  });
});

group("renderSpectator: the feed and the ticker", () => {
  // VTT-263
  test("the feed labels a move with its reason", () => {
    const root = render([move({ seq: 1n, reason: "toward the window" })]);
    const beats = feedBeats(root);
    expect(beats.length).toBe(1);
    expect(beats[0]).toContain(BASE);
    expect(beats[0]).toContain("toward the window");
  });

  // VTT-263
  test("the ticker labels a move with its reason", () => {
    const root = render([move({ seq: 1n, reason: "toward the window" })]);
    const tick = tickFor(root, 1);
    expect(tick).toContain(BASE);
    expect(tick).toContain("toward the window");
  });

  // VTT-263
  test("in a mixed log the reason labels only the move whose frame carries it", () => {
    const root = render([
      move({ seq: 1n }),
      move({ seq: 2n, tokenId: "tok-b", reason: "toward the window" }),
      move({ seq: 3n, tokenId: "tok-c" }),
    ]);
    const beats = feedBeats(root);
    expect(beats.filter((b) => b.includes("toward the window"))).toEqual([
      expect.stringContaining("tok-b"),
    ]);
    expect(beats).toContain(BASE);
    expect(beats).toContain("tok-c moved to 7,5");

    expect(tickFor(root, 2)).toContain("toward the window");
    expect(tickFor(root, 1)).toBe(BASE);
    expect(tickFor(root, 3)).toBe("tok-c moved to 7,5");
  });

  // The design doc named in this file's header: `"<token> moved to x,y"` —
  // the feed and ticker label for a move without a reason is unchanged.
  test("a move with no reason renders as before in both places", () => {
    const root = render([move({ seq: 4n })]);
    expect(feedBeats(root)).toEqual([BASE]);
    expect(tickFor(root, 4)).toBe(BASE);
  });

  // VTT-263
  test("a reason that looks like markup is shown as text and builds no elements", () => {
    const reason = `<b id="qa-b">bold</b><img id="qa-img" src="x"><script id="qa-s">1</script><i>it</i>`;
    const root = render([move({ seq: 1n, reason })]);
    expect(root.querySelector("#qa-b, #qa-img, #qa-s, b, img, script, i")).toBeNull();
    expect(feedBeats(root)[0]).toContain(reason);
    expect(tickFor(root, 1)).toContain(reason);
  });

  // VTT-263
  test("html entities in a reason are not decoded on the way into the DOM", () => {
    const reason = "&amp; &lt;b&gt;";
    const root = render([move({ seq: 1n, reason })]);
    expect(feedBeats(root)[0]).toContain(reason);
    expect(tickFor(root, 1)).toContain(reason);
    expect(root.querySelector("b")).toBeNull();
  });

  // VTT-263
  test("an empty reason renders the plain label in the feed and the ticker", () => {
    const root = render([move({ seq: 1n, reason: "" })]);
    expect(feedBeats(root)).toEqual([BASE]);
    expect(tickFor(root, 1)).toBe(BASE);
  });
});
