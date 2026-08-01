import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  EnvelopeSchema, SessionStartedSchema, SessionEndedSchema, SceneCreatedSchema,
  ActorAddedSchema, TokenPlacedSchema, TokenMovedSchema, NarrationAddedSchema,
  NoteUpsertedSchema, NoteDeletedSchema, ConditionAppliedSchema,
  ConditionRemovedSchema, ResourceChangedSchema, AbilityUsedSchema,
  AttackRolledSchema, AdventureLoadedSchema, EventsRetractedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { renderSpectator, describe as describeEvent, CELL } from "../src/view/spectator";
import { newState, type State } from "../src/state";
import { renderPlayerPanel } from "../src/view/player";
import type { Ability, Me } from "../src/metadata";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";

const env = (seq: number, payload: Envelope["payload"]): Envelope =>
  create(EnvelopeSchema, { eventId: `e${seq}`, sequence: BigInt(seq), payload });

function world(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "The Hall", GridWidth: 6, GridHeight: 4 };
  st.Actors["a1"] = {
    actorId: "a1", name: "Lera", moduleId: "", attributes: {},
    resources: { vigor: { current: 3, max: 10 } }, controllerId: "p-me",
  };
  st.Tokens["t1"] = { ID: "t1", SceneID: "s1", ActorID: "a1", X: 2, Y: 1 };
  st.Conditions["a1"] = [{ ID: "dazed", Source: "dm", AppliedSeq: 4 }];
  st.Notes["k"] = { Title: "A Note", Text: "body text", UpdatedSeq: 5 };
  st.Sessions = [{ ID: "sess-1", Name: "Night One", StartSeq: 1, EndSeq: 0 }];
  return st;
}

function render(st: State, log: Envelope[] = []): HTMLElement {
  const root = document.createElement("div");
  renderSpectator(root, st, log, "connected");
  return root;
}

test("the board draws a disc per token, with its initial", () => {
  const root = render(world());
  const tokens = root.querySelectorAll(".token");
  expect(tokens).toHaveLength(1);
  expect(root.querySelector(".initial")?.textContent).toBe("L");
});

test("every resource shows as a current/max chip, named on hover", () => {
  const chip = render(world()).querySelector(".chip") as HTMLElement;
  expect(chip.textContent).toBe("3/10");
  // The NAME is the tooltip, never abbreviated onto the face.
  expect(chip.title).toBe("vigor");
});

test("every condition shows as a dot carrying its name", () => {
  const dot = render(world()).querySelector(".dot") as HTMLElement;
  expect(dot.title).toBe("dazed");
});

test("a token is positioned by its grid coordinates", () => {
  const tok = render(world()).querySelector(".token") as HTMLElement;
  expect(tok.style.left).toBe("88px"); // x=2 * 44
  expect(tok.style.top).toBe("44px"); // y=1 * 44
});

test("notes render with title and body", () => {
  const notes = render(world()).querySelector(".notes")!;
  expect(notes.textContent).toContain("A Note");
  expect(notes.textContent).toContain("body text");
});

test("an open session is named in the status bar", () => {
  expect(render(world()).querySelector(".session")?.textContent).toContain("Night One");
});

test("empty state reads honestly rather than rendering a blank panel", () => {
  const root = render(newState());
  expect(root.querySelector(".board")?.textContent).toContain("No scene");
  expect(root.querySelector(".feed")?.textContent).toContain("Nothing has happened");
  expect(root.querySelector(".notes")?.textContent).toContain("No notes");
});

test("the ticker shows sequence numbers, newest first", () => {
  const log = [
    env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
    env(2, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "s1", name: "H", gridWidth: 6, gridHeight: 4 }) }),
  ];
  const ticks = render(world(), log).querySelectorAll(".tick");
  expect(ticks[0]!.textContent).toContain("#2");
});

test("the ticker is bounded — an all-night session must not render thousands of rows", () => {
  const log = Array.from({ length: 200 }, (_, i) =>
    env(i + 1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
  );
  const ticks = render(world(), log).querySelectorAll(".tick").length;
  expect(ticks).toBeLessThanOrEqual(40);
  // Paired lower bound: a ticker that rendered nothing at all would satisfy
  // the cap on its own, so the cap alone proves nothing.
  expect(ticks).toBeGreaterThan(0);
});

test("in-character speech is marked with its speaker; table talk is not", () => {
  const log = [
    env(1, { case: "narrationAdded", value: create(NarrationAddedSchema, { text: "Hi", as: "Goblin" }) }),
    env(2, { case: "narrationAdded", value: create(NarrationAddedSchema, { text: "ooc" }) }),
  ];
  const root = render(world(), log);
  expect(root.querySelector(".speaker")?.textContent).toBe("Goblin: ");
  expect(root.querySelectorAll(".narration")).toHaveLength(1);
});

test("describe renders a real label for every event kind, not the fallback", () => {
  // The previous version of this test asserted only `label.length > 0` and
  // `label !== "event"` over six kinds. describe()'s default branch is
  // `return p.case ?? "event"`, and every case name is a non-empty string
  // that is not "event" — so DELETING THE ENTIRE SWITCH left all six
  // assertions passing. It proved that protobuf sets `payload.case`.
  //
  // So: exact strings, and every kind describe() handles. The fallback is
  // separately caught by asserting no label equals its own case name.
  const cases: [Envelope, string][] = [
    [env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
      "session started — S"],
    [env(2, { case: "sessionEnded", value: create(SessionEndedSchema, {}) }),
      "session ended"],
    [env(3, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "s", name: "N", gridWidth: 1, gridHeight: 1 }) }),
      'scene "N"'],
    [env(4, { case: "actorAdded", value: create(ActorAddedSchema, { actor: { actorId: "a", name: "A" } }) }),
      "actor A joined"],
    [env(5, { case: "tokenPlaced", value: create(TokenPlacedSchema, { tokenId: "t", sceneId: "s", actorId: "a", position: { x: 1, y: 2 } }) }),
      "t placed at 1,2"],
    [env(6, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t", to: { x: 3, y: 4 } }) }),
      "t moved to 3,4"],
    [env(7, { case: "conditionApplied", value: create(ConditionAppliedSchema, { actorId: "a", conditionId: "prone" }) }),
      "a gained prone"],
    [env(8, { case: "conditionRemoved", value: create(ConditionRemovedSchema, { actorId: "a", conditionId: "prone" }) }),
      "a lost prone"],
    [env(9, { case: "resourceChanged", value: create(ResourceChangedSchema, { actorId: "a", resource: "hp", newValue: 7 }) }),
      "a hp → 7"],
    [env(10, { case: "noteUpserted", value: create(NoteUpsertedSchema, { key: "k", title: "t", text: "x" }) }),
      'note "k" updated'],
    [env(11, { case: "noteDeleted", value: create(NoteDeletedSchema, { key: "k" }) }),
      'note "k" deleted'],
    [env(12, { case: "abilityUsed", value: create(AbilityUsedSchema, { actorId: "a", abilityId: "cleave" }) }),
      "a used cleave"],
    [env(13, { case: "attackRolled", value: create(AttackRolledSchema, { total: 17 }) }),
      "roll 17"],
    [env(14, { case: "adventureLoaded", value: create(AdventureLoadedSchema, { adventureId: "adv" }) }),
      "adventure adv loaded"],
  ];

  for (const [e, want] of cases) {
    expect(describeEvent(e)).toBe(want);
  }

  // Anything reaching the default branch renders as its own case name, which
  // is developer vocabulary leaking into the story feed.
  for (const [e] of cases) {
    expect(describeEvent(e)).not.toBe(e.payload.case);
  }
});

test("an event kind describe() does not handle degrades to its case name", () => {
  // Pins the fallback itself, so the test above cannot be satisfied BY it.
  const e = env(99, { case: "eventsRetracted", value: create(EventsRetractedSchema, { fromSequence: 1n, toSequence: 2n, reason: "r" }) });
  expect(describeEvent(e)).toBe("eventsRetracted");
});

// --- player panel -----------------------------------------------------------

const me: Me = { participantId: "p-me", name: "Me", role: "player", controls: ["a1"] };
const atWill: Ability = { id: "swing", name: "Swing", range: 2, maxTargets: 1, usage: { kind: "atWill" } };
const costly: Ability = {
  id: "surge", name: "Surge", range: 1, maxTargets: 1,
  usage: { kind: "resource", resource: "vigor", cost: 99 },
};

function panel(
  st: State,
  abilities: Ability[],
  ui = { selectedActorId: "", selectedAbilityId: "" },
  who: Me = me,
) {
  const sent: ClientCommand[] = [];
  // Rerenders are counted: a handler that mutates ui without asking for a
  // repaint leaves the panel showing the old selection, which is the same
  // symptom as not handling the click at all.
  let repaints = 0;
  const node = renderPlayerPanel(st, who, abilities, ui, (c) => sent.push(c), () => { repaints++; });
  return {
    node, sent, ui,
    repaints: () => repaints,
    button: (label: string) =>
      Array.from(node.querySelectorAll("button")).find((b) => b.textContent === label)!,
    input: (cls: string) => node.querySelector(`.${cls}`) as HTMLInputElement,
  };
}

test("a participant controlling nothing is told so, not shown empty controls", () => {
  const st = world();
  st.Actors["a1"]!.controllerId = "someone-else";
  expect(panel(st, [atWill]).node.textContent).toContain("do not control");
});

test("an unaffordable ability is shown but DISABLED, not hidden", () => {
  // A player needs to know the ability exists and why it is unavailable.
  const node = panel(world(), [costly]).node;
  const btn = Array.from(node.querySelectorAll("button")).find((b) => b.textContent === "Surge")!;
  expect(btn.disabled).toBe(true);
  expect(btn.title).toContain("not enough");
});

test("arming an ability lists the targets in range", () => {
  const { node } = panel(world(), [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  expect(node.textContent).toContain("Targets (range 2)");
});

test("with no ability armed the panel invites a move instead", () => {
  expect(panel(world(), [atWill]).node.textContent).toContain("Click the board to move");
});

test("sending narration requires text, and clears the box afterwards", () => {
  const { node, sent } = panel(world(), []);
  const text = node.querySelector(".text") as HTMLInputElement;
  const send = Array.from(node.querySelectorAll("button")).find((b) => b.textContent === "Send")!;

  send.click(); // empty
  expect(sent).toHaveLength(0);

  text.value = "I step forward.";
  send.click();
  expect(sent).toHaveLength(1);
  expect(text.value).toBe("");
});

// ============================================================================
// Mutation-driven suite. view/spectator.ts opened at 71.62%, and the survivors
// were not evenly spread: the board's GEOMETRY and describe()'s crash-safety
// were untested outright, while the labels and classes were merely unasserted.

/** Render with a click handler, and put the board somewhere non-zero. */
function board(st: State, onCell: (c: { x: number; y: number }) => void, at = { left: 100, top: 40 }) {
  const root = document.createElement("div");
  renderSpectator(root, st, [], "connected", { onCell });
  const el = root.querySelector(".grid") as HTMLElement;
  // happy-dom reports a zero rect for everything, which would make the
  // subtraction below a no-op and hide the very bug this pins.
  el.getBoundingClientRect = () =>
    ({ left: at.left, top: at.top, right: 0, bottom: 0, width: 0, height: 0, x: at.left, y: at.top, toJSON() {} }) as DOMRect;
  return { root, el };
}

function clickAt(el: HTMLElement, clientX: number, clientY: number) {
  el.dispatchEvent(new MouseEvent("click", { clientX, clientY, bubbles: true }));
}

test("a board click maps to the cell under the pointer, offset by the board's box", () => {
  // THE one with gameplay consequences. `ev.clientX - r.left` -> `+ r.left`
  // survives every existing test because happy-dom's rect is all zeros, and in
  // a real browser it sends a player's token to the wrong square — silently,
  // plausibly, and only for boards not at the page origin.
  const cells: { x: number; y: number }[] = [];
  const { el } = board(world(), (c) => cells.push(c));

  // CELL is 44. A click 100px right and 40px down from the board's own
  // top-left is cell (2, 0): (190-100)/44 = 2.04, (58-40)/44 = 0.4.
  clickAt(el, 190, 58);
  expect(cells).toEqual([{ x: 2, y: 0 }]);
});

test("the same pixel maps to a DIFFERENT cell when the board moves", () => {
  // Pins that the offset is actually subtracted rather than ignored.
  const a: { x: number; y: number }[] = [];
  const b: { x: number; y: number }[] = [];
  clickAt(board(world(), (c) => a.push(c), { left: 0, top: 0 }).el, 190, 58);
  clickAt(board(world(), (c) => b.push(c), { left: 100, top: 40 }).el, 190, 58);
  expect(a[0]).not.toEqual(b[0]!);
});

test("a click past the last cell is clamped onto the board", () => {
  const cells: { x: number; y: number }[] = [];
  const { el } = board(world(), (c) => cells.push(c));
  clickAt(el, 9999, 9999);
  // The scene is 6x4, so the far corner is (5, 3).
  expect(cells).toEqual([{ x: 5, y: 3 }]);
});

test("the board is sized by its scene, in cells times CELL", () => {
  // `geom.width * CELL` -> `/ CELL` renders a 6-cell board 0.13px wide.
  const root = render(world());
  const grid = root.querySelector(".grid") as HTMLElement;
  expect(grid.style.width).toBe(`${6 * CELL}px`);
  expect(grid.style.height).toBe(`${4 * CELL}px`);
  expect(grid.style.backgroundSize).toBe(`${CELL}px ${CELL}px`);
  expect(grid.dataset["sceneId"]).toBe("s1");
});

test("a board with no click handler is not dressed as clickable", () => {
  const grid = render(world()).querySelector(".grid") as HTMLElement;
  expect(grid.style.cursor).toBe("");
});

test("a board WITH a handler invites the click", () => {
  expect(board(world(), () => {}).el.style.cursor).toBe("crosshair");
});

// --- describe() must not throw on a malformed event -------------------------

test("an event missing the fields describe() reads degrades instead of throwing", () => {
  // The optional chaining in describe() is load-bearing: fold() tolerates
  // these events, so the feed meets them, and a throw here blanks the whole
  // page rather than dropping one row.
  const cases: Envelope[] = [
    env(1, { case: "actorAdded", value: create(ActorAddedSchema, {}) }),
    env(2, { case: "tokenPlaced", value: create(TokenPlacedSchema, { tokenId: "t", sceneId: "s", actorId: "a" }) }),
    env(3, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t" }) }),
  ];
  for (const e of cases) {
    expect(() => describeEvent(e)).not.toThrow();
    expect(describeEvent(e).length).toBeGreaterThan(0);
  }
});

test("an actor with neither name nor id still reads as something", () => {
  const e = env(1, { case: "actorAdded", value: create(ActorAddedSchema, { actor: { actorId: "", name: "" } }) });
  expect(describeEvent(e)).toBe("actor ? joined");
});

test("a position-less placement reads as the origin, not as undefined", () => {
  const e = env(1, { case: "tokenPlaced", value: create(TokenPlacedSchema, { tokenId: "t9", sceneId: "s", actorId: "a" }) });
  expect(describeEvent(e)).toBe("t9 placed at 0,0");
});

test("an envelope with no payload at all reads as a generic event", () => {
  // `p.case ?? "event"` — the last fallback. An empty label is a blank row
  // the reader cannot interpret.
  const bare = create(EnvelopeSchema, { eventId: "bare", sequence: 1n });
  expect(describeEvent(bare)).toBe("event");
});

// --- discs: the fallbacks and the optional sections -------------------------

test("a disc is titled by name, falling back to the actor id", () => {
  const st = world();
  st.Actors["a1"]!.name = "";
  const t = render(st).querySelector(".token") as HTMLElement;
  expect(t.title).toBe("a1");
  expect(t.dataset["tokenId"]).toBe("t1");
});

test("a named actor's disc is titled by the NAME, not the id", () => {
  expect((render(world()).querySelector(".token") as HTMLElement).title).toBe("Lera");
});

test("a token with no resources or conditions renders neither chips nor dots", () => {
  // `d.resources.length > 0` -> `>= 0` renders an empty chip strip on every
  // plain token, which is visual noise on a crowded board.
  const st = world();
  st.Actors["a1"]!.resources = {};
  st.Conditions = {};
  const t = render(st).querySelector(".token")!;
  expect(t.querySelector(".chips")).toBeNull();
  expect(t.querySelector(".dots")).toBeNull();
});

test("a token WITH them renders both containers", () => {
  const t = render(world()).querySelector(".token")!;
  expect(t.querySelector(".chips")).not.toBeNull();
  expect(t.querySelector(".dots")).not.toBeNull();
});

// --- the story feed ---------------------------------------------------------

test("the story panel is headed and says so when empty", () => {
  const root = render(world(), []);
  expect(Array.from(root.querySelectorAll("h2")).some((n) => n.textContent === "Story")).toBe(true);
  expect(root.querySelector(".empty")?.textContent).toBe("Nothing has happened yet.");
});

test("a beat carries the mechanical events it narrates", () => {
  // The for-loop over entry.events was free to be emptied: the narration
  // would render with the mechanics it describes silently missing.
  const log = [
    env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
    env(2, { case: "narrationAdded", value: create(NarrationAddedSchema, { text: "The door groans.", as: "DM" }) }),
  ];
  const root = render(world(), log);
  // An unanchored narration is its own beat, separate from the mechanical
  // event before it (see buildFeed) — so assert across the feed, not within
  // the first article.
  expect(Array.from(root.querySelectorAll(".beat")).length).toBeGreaterThan(0);
  expect(root.querySelector(".speech")).not.toBeNull();
  expect(root.querySelector(".speaker")?.textContent).toBe("DM: ");
  expect(root.querySelector(".speech")?.textContent).toBe("DM: The door groans.");
  expect(Array.from(root.querySelectorAll(".mechanical")).length).toBeGreaterThan(0);
});

test("table talk with no speaker renders as narration, without a speaker span", () => {
  const log = [env(1, { case: "narrationAdded", value: create(NarrationAddedSchema, { text: "ooc: brb" }) })];
  const root = render(world(), log);
  expect(root.querySelector(".narration")).not.toBeNull();
  expect(root.querySelector(".speaker")).toBeNull();
});

// --- notes ------------------------------------------------------------------

test("notes are listed in sorted key order, headed, and say so when empty", () => {
  // `Object.keys(st.Notes).sort()` -> unsorted follows insertion, so the
  // list reorders itself as the DM edits.
  const st = world();
  st.Notes = {
    zeta: { Title: "Z", Text: "z", UpdatedSeq: 1 },
    alpha: { Title: "A", Text: "a", UpdatedSeq: 2 },
    mid: { Title: "M", Text: "m", UpdatedSeq: 3 },
  };
  const titles = Array.from(render(st).querySelectorAll(".note h3")).map((n) => n.textContent);
  expect(titles).toEqual(["A", "M", "Z"]);

  const none = world();
  none.Notes = {};
  const root = render(none);
  expect(Array.from(root.querySelectorAll("h2")).some((n) => n.textContent === "Notes")).toBe(true);
  expect(Array.from(root.querySelectorAll(".empty")).some((n) => n.textContent === "No notes.")).toBe(true);
});

// --- ticker and status ------------------------------------------------------

test("each ticker row carries its sequence number and its description", () => {
  const log = [env(7, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) })];
  const root = render(world(), log);
  expect(Array.from(root.querySelectorAll("h2")).some((n) => n.textContent === "Events")).toBe(true);
  const row = root.querySelector(".tick")!;
  expect(row.querySelector(".seq")?.textContent).toBe("#7");
  expect(row.textContent).toContain("session started");
});

test("the status bar shows the connection state and the session", () => {
  const root = render(world());
  expect(root.querySelector(".status")).not.toBeNull();
  expect(root.querySelector(".conn")?.textContent).toBe("connected");
  expect(root.querySelector(".session")?.textContent).toBe("session: Night One");
});

test("with only a CLOSED session the status does not claim one is open", () => {
  // `find((s) => s.EndSeq === 0)` -> `find(() => true)` names a finished
  // session as the live one.
  const st = world();
  st.Sessions = [{ ID: "s", Name: "Night One", StartSeq: 1, EndSeq: 9 }];
  // Assert the EXACT text, not just the absence of the name: an emptied
  // template also fails to contain "Night One", so `not.toContain` was
  // satisfied by a blank status bar.
  // The closed-session branch reports a COUNT, so the reader can tell "no
  // session open" from "no sessions at all".
  expect(render(st).querySelector(".session")?.textContent).toBe("sessions: 1");
});

// --- which scene is shown ---------------------------------------------------

test("the board shows the LAST scene by sorted id, so a new scene takes over", () => {
  // `Object.keys(st.Scenes).sort().at(-1)` — without the sort this follows
  // insertion order, and the party appears to walk backwards into the room
  // they just left.
  const st = world();
  st.Scenes["s2"] = { ID: "s2", Name: "The Vault", GridWidth: 3, GridHeight: 3 };
  st.Scenes["s0"] = { ID: "s0", Name: "The Steps", GridWidth: 2, GridHeight: 2 };
  expect(render(st).querySelector(".grid")?.getAttribute("data-scene-id")).toBe("s2");
});

test("a state with no scenes at all says so rather than rendering a blank board", () => {
  const st = world();
  st.Scenes = {};
  st.Tokens = {};
  expect(Array.from(render(st).querySelectorAll(".empty")).some((n) => n.textContent === "No scene yet.")).toBe(true);
});

// --- the extras slots -------------------------------------------------------

test("panel, console and toast appear only when supplied", () => {
  const plain = document.createElement("div");
  renderSpectator(plain, world(), [], "connected");
  expect(plain.querySelector(".toast")).toBeNull();

  const full = document.createElement("div");
  const panel = document.createElement("div"); panel.className = "my-panel";
  const console_ = document.createElement("div"); console_.className = "my-console";
  renderSpectator(full, world(), [], "connected", { panel, console: console_, toast: "refused: nope" });
  expect(full.querySelector(".my-panel")).not.toBeNull();
  expect(full.querySelector(".my-console")).not.toBeNull();
  expect(full.querySelector(".toast")?.textContent).toBe("refused: nope");
});

test("the ticker section carries its class, and headings carry none", () => {
  const root = render(world(), [env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) })]);
  expect(root.querySelector(".ticker")).not.toBeNull();
  // `if (cls) n.className = cls` -> always-assign writes undefined, which
  // stringifies to a literal "undefined" class on every unstyled element.
  for (const h of Array.from(root.querySelectorAll("h2"))) expect(h.className).toBe("");
});

test("a scene keyed by the empty string is still rendered", () => {
  // Pins that `.at(-1)` picks the last sorted key even when that key is the
  // empty string — `.at(-1) ?? ""` must not treat a real "" key as "no scene".
  //
  // It does NOT kill the `?? ""` fallback itself, and the comment here used to
  // claim it did: the fallback fires only when there are NO scenes at all, in
  // which case at(-1) is undefined. See ts-mutation-equivalents.txt.
  const st = world();
  st.Scenes = { "": { ID: "", Name: "Nowhere", GridWidth: 2, GridHeight: 2 } };
  st.Tokens = {};
  const root = render(st);
  expect(root.querySelector(".grid")).not.toBeNull();
  expect(Array.from(root.querySelectorAll(".empty")).some((n) => n.textContent === "No scene yet.")).toBe(false);
});

test("omitted extras leave no trace, not the word 'undefined'", () => {
  // `if (extras.panel)` -> always-push appends undefined to the node list,
  // which renders as text next to the board.
  const root = document.createElement("div");
  renderSpectator(root, world(), [], "connected");
  expect(root.textContent).not.toContain("undefined");
  expect(root.querySelector(".my-panel")).toBeNull();
});

// --- the panel's selection state --------------------------------------------

test("the selected actor's chip is marked, and the others are not", () => {
  const st = world();
  st.Actors["a2"] = {
    actorId: "a2", name: "Bran", moduleId: "", attributes: {}, resources: {}, controllerId: "p-me",
  };
  const p = panel(st, [atWill], { selectedActorId: "a2", selectedAbilityId: "" });
  expect(p.button("Bran").className).toBe("chip sel");
  expect(p.button("Lera").className).toBe("chip");
});

test("clicking an actor selects it, drops any armed ability, and repaints", () => {
  const st = world();
  st.Actors["a2"] = {
    actorId: "a2", name: "Bran", moduleId: "", attributes: {}, resources: {}, controllerId: "p-me",
  };
  const p = panel(st, [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  p.button("Bran").click();
  expect(p.ui.selectedActorId).toBe("a2");
  // Switching actor must disarm: the armed ability belonged to the old one.
  expect(p.ui.selectedAbilityId).toBe("");
  expect(p.repaints()).toBe(1);
});

test("an actor with no name falls back to its id on the chip", () => {
  const st = world();
  st.Actors["a1"]!.name = "";
  expect(panel(st, [atWill]).button("a1")).toBeDefined();
});

test("clicking an ability arms it; clicking the armed one disarms", () => {
  // `selectedAbilityId === ab.id ? "" : ab.id` is a TOGGLE, and both arms
  // were free to be replaced by a constant.
  const armed = panel(world(), [atWill], { selectedActorId: "a1", selectedAbilityId: "" });
  armed.button("Swing").click();
  expect(armed.ui.selectedAbilityId).toBe("swing");
  expect(armed.repaints()).toBe(1);

  const disarm = panel(world(), [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  disarm.button("Swing").click();
  expect(disarm.ui.selectedAbilityId).toBe("");
});

test("the armed ability's chip is marked", () => {
  const p = panel(world(), [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  expect(p.button("Swing").className).toBe("chip sel");
});

test("an at-will ability is enabled and says so on hover", () => {
  const p = panel(world(), [atWill]);
  expect(p.button("Swing").disabled).toBe(false);
  expect(p.button("Swing").title).toBe("at will");
});

test("an affordable resource ability names its cost without the shortfall note", () => {
  const cheap: Ability = {
    id: "jab", name: "Jab", range: 1, maxTargets: 1,
    usage: { kind: "resource", resource: "vigor", cost: 1 },
  };
  const p = panel(world(), [cheap]);
  expect(p.button("Jab").disabled).toBe(false);
  expect(p.button("Jab").title).toBe("vigor 1");
});

// --- no token on the board --------------------------------------------------

test("an actor with no token is said so, and every ability is disabled", () => {
  // `disabled = !can || tokenId === ""` — the second arm. An affordable
  // ability with nowhere to act from must not look usable.
  const st = world();
  st.Tokens = {};
  const p = panel(st, [atWill]);
  expect(p.node.textContent).toContain("no token on the board");
  expect(p.button("Swing").disabled).toBe(true);
});

test("with a token present the panel invites a board click instead", () => {
  const p = panel(world(), [atWill]);
  expect(p.node.querySelector(".hint")?.textContent).toBe("Click the board to move.");
  expect(p.node.textContent).not.toContain("no token on the board");
});

test("with no token there is no move hint either", () => {
  const st = world();
  st.Tokens = {};
  expect(panel(st, [atWill]).node.querySelector(".hint")).toBeNull();
});

// --- targets ----------------------------------------------------------------

test("a target button sends the ability at that token, then disarms", () => {
  const st = world();
  st.Actors["a2"] = {
    actorId: "a2", name: "Bran", moduleId: "", attributes: {}, resources: {}, controllerId: "p-them",
  };
  st.Tokens["t2"] = { ID: "t2", SceneID: "s1", ActorID: "a2", X: 3, Y: 1 };
  const p = panel(st, [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  p.button("Bran").click();
  expect(p.sent).toHaveLength(1);
  expect(p.sent[0]!.command.case).toBe("useAbility");
  expect(p.sent[0]!.command.value).toEqual(
    expect.objectContaining({ actorId: "a1", abilityId: "swing", targetIds: ["t2"] }),
  );
  expect(p.ui.selectedAbilityId).toBe("");
  expect(p.repaints()).toBe(1);
});

test("a target with no actor name is labelled by its token id", () => {
  const st = world();
  st.Tokens["t2"] = { ID: "t2", SceneID: "s1", ActorID: "ghost", X: 3, Y: 1 };
  const p = panel(st, [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  expect(p.button("t2")).toBeDefined();
});

test("the targets heading names the ability's range", () => {
  const st = world();
  const reach: Ability = { id: "poke", name: "Poke", range: 3, maxTargets: 1, usage: { kind: "atWill" } };
  const p = panel(st, [reach], { selectedActorId: "a1", selectedAbilityId: "poke" });
  expect(Array.from(p.node.querySelectorAll("h3")).some((n) => n.textContent === "Targets (range 3)")).toBe(true);
});

test("an armed ability always lists at least the actor's own square", () => {
  // Worth stating plainly because it makes the "nothing in range" branch
  // below effectively unreachable: targetableTokens filters by Chebyshev
  // distance from the ACTING token, which is itself at distance 0, so any
  // range >= 0 includes it.
  const p = panel(world(), [{ id: "poke", name: "Poke", range: 0, maxTargets: 1, usage: { kind: "atWill" } }],
                  { selectedActorId: "a1", selectedAbilityId: "poke" });
  expect(p.node.querySelector(".empty")).toBeNull();
  expect(p.button("Lera")).toBeDefined();
});

test("a NEGATIVE range is the only way to reach 'nothing in range'", () => {
  // KNOWN ODDITY, pinned rather than fixed. `if (targets.length === 0)` is
  // dead for every range a sane ruleset declares, because the acting token is
  // always within range 0 of itself. A negative range is the only input that
  // empties the list, and the panel does then say so instead of rendering a
  // bare row. Flagged for the ruleset side: if range is constrained to >= 0
  // there, this branch is unreachable and should be removed rather than left
  // looking like a handled case.
  const nonsense: Ability = { id: "void", name: "Void", range: -1, maxTargets: 1, usage: { kind: "atWill" } };
  const p = panel(world(), [nonsense], { selectedActorId: "a1", selectedAbilityId: "void" });
  expect(Array.from(p.node.querySelectorAll(".empty")).some((n) => n.textContent === "nothing in range")).toBe(true);
});

// --- saying something -------------------------------------------------------

test("Send narrates the trimmed text, clears the box and repaints", () => {
  const p = panel(world(), []);
  p.input("text").value = "  The door groans.  ";
  p.button("Send").click();
  expect(p.sent).toHaveLength(1);
  expect(p.sent[0]!.command.case).toBe("addNarration");
  expect(p.sent[0]!.command.value).toEqual(expect.objectContaining({ text: "The door groans." }));
  expect(p.input("text").value).toBe("");
  expect(p.repaints()).toBe(1);
});

test("an in-character speaker is carried, and a blank one is omitted", () => {
  const named = panel(world(), []);
  named.input("as").value = "  Lera  ";
  named.input("text").value = "Hold.";
  named.button("Send").click();
  expect(named.sent[0]!.command.value).toEqual(expect.objectContaining({ as: "Lera" }));

  const anon = panel(world(), []);
  anon.input("as").value = "   ";
  anon.input("text").value = "ooc: brb";
  anon.button("Send").click();
  expect((anon.sent[0]!.command.value as Record<string, unknown>)["as"] ?? "").toBe("");
});

test("Enter in the text box sends; another key does not", () => {
  // The keydown handler was free to be emptied, and its key check to be
  // forced true — under which every keystroke fires a narration.
  const p = panel(world(), []);
  p.input("text").value = "Go.";
  p.input("text").dispatchEvent(new KeyboardEvent("keydown", { key: "a", bubbles: true }));
  expect(p.sent).toHaveLength(0);
  p.input("text").dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  expect(p.sent).toHaveLength(1);
});

test("empty or whitespace-only text sends nothing", () => {
  for (const v of ["", "   "]) {
    const p = panel(world(), []);
    p.input("text").value = v;
    p.button("Send").click();
    expect(p.sent).toHaveLength(0);
    expect(p.repaints()).toBe(0);
  }
});

test("with no abilities at all the panel omits the abilities section", () => {
  const p = panel(world(), []);
  expect(Array.from(p.node.querySelectorAll("h3")).some((n) => n.textContent === "Abilities")).toBe(false);
  expect(p.node.querySelector(".player")).toBeNull();
});

test("the panel's structure carries the classes the stylesheet targets", () => {
  const p = panel(world(), [atWill]);
  expect(p.node.className).toBe("player");
  expect(p.node.querySelectorAll(".row").length).toBeGreaterThan(0);
  expect(p.input("as").className).toBe("as");
  expect(p.input("text").className).toBe("text");
  // Headings are built without a class; an always-assign writes the string
  // "undefined" into className.
  for (const h of Array.from(p.node.querySelectorAll("h2,h3"))) expect(h.className).toBe("");
});

test("the panel's headings say what each section is", () => {
  const p = panel(world(), [atWill]);
  expect(Array.from(p.node.querySelectorAll("h2")).map((n) => n.textContent)).toEqual(["Your turn"]);
  const h3 = Array.from(p.node.querySelectorAll("h3")).map((n) => n.textContent);
  expect(h3).toContain("Abilities");
  expect(h3).toContain("Say something");
});

test("the empty-state messages are the ones a player can act on", () => {
  const none = world();
  none.Actors["a1"]!.controllerId = "someone-else";
  expect(panel(none, [atWill]).node.querySelector(".empty")?.textContent)
    .toBe("You do not control an actor yet.");

  const noToken = world();
  noToken.Tokens = {};
  expect(panel(noToken, [atWill]).node.querySelector(".empty")?.textContent)
    .toBe("That actor has no token on the board yet.");
});

test("an unarmed ability's chip is NOT marked selected", () => {
  // `selectedAbilityId === ab.id ? "chip sel" : "chip"` -> always-selected
  // marks every ability as armed at once.
  const second: Ability = { id: "jab", name: "Jab", range: 1, maxTargets: 1, usage: { kind: "atWill" } };
  const p = panel(world(), [atWill, second], { selectedActorId: "a1", selectedAbilityId: "swing" });
  expect(p.button("Swing").className).toBe("chip sel");
  expect(p.button("Jab").className).toBe("chip");
});

test("a selection naming an actor that no longer exists renders nothing further", () => {
  // `if (!actor) return wrap` — the actor can vanish between a click and the
  // repaint that follows it. Continuing past this point reads resources off
  // undefined and throws, taking the panel down.
  const st = world();
  const ui = { selectedActorId: "gone", selectedAbilityId: "" };
  let node: HTMLElement | null = null;
  expect(() => { node = renderPlayerPanel(st, me, [atWill], ui, () => {}, () => {}); }).not.toThrow();
  expect(Array.from(node!.querySelectorAll("h3")).some((n) => n.textContent === "Abilities")).toBe(false);
});

test("an armed ability with no token on the board offers no targets", () => {
  // `armed && tokenId !== ""` — the second arm. Offering target buttons for an
  // actor with nowhere to act from sends a command the server will refuse.
  const st = world();
  st.Tokens = {};
  const p = panel(st, [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  expect(Array.from(p.node.querySelectorAll("h3")).some((n) => n.textContent?.startsWith("Targets"))).toBe(false);
});

test("the panel's rows, form and chips carry their classes", () => {
  const st = world();
  st.Tokens["t2"] = { ID: "t2", SceneID: "s1", ActorID: "a1", X: 3, Y: 1 };
  const p = panel(st, [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  // Actor picker, abilities row, targets row.
  expect(p.node.querySelectorAll(".row").length).toBeGreaterThanOrEqual(3);
  expect(p.node.querySelector(".say")).not.toBeNull();
  // Every button in the panel is a chip, including Send and the target ones.
  for (const b of Array.from(p.node.querySelectorAll("button"))) {
    expect(b.className.startsWith("chip")).toBe(true);
  }
  expect(p.button("Send").className).toBe("chip");
});

test("both say-something boxes carry a placeholder", () => {
  // Presence, not wording — the same rule as the DM console: an unlabelled
  // pair of boxes gives no clue which one is the speaker.
  const p = panel(world(), []);
  expect(p.input("as").placeholder.length).toBeGreaterThan(0);
  expect(p.input("text").placeholder.length).toBeGreaterThan(0);
  expect(p.input("as").placeholder).not.toBe(p.input("text").placeholder);
});
