// The spectator floor (client spec §4): scene grid, story feed, notes panel,
// session status, event ticker.
//
// Deliberately thin. Everything with a decision in it — which cell a click
// lands on, how narration groups with the events it describes — lives in
// grid.ts and feed.ts, where it is unit-tested without a browser. What is
// here is DOM assembly, which is the part a screenshot verifies better than
// an assertion could.

import type { State } from "../state";
import type { Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";
import { buildFeed, type FeedEntry } from "./feed";
import { tokensOnScene, type Geometry, type TokenDisc } from "./grid";

export const CELL = 44;

function el(tag: string, cls?: string, text?: string): HTMLElement {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined) n.textContent = text;
  return n;
}

/** A short, human label for an event, used by the feed and the ticker. */
export function describe(e: Envelope): string {
  const p = e.payload;
  switch (p.case) {
    case "sessionStarted":
      return `session started — ${p.value.name}`;
    case "sessionEnded":
      return "session ended";
    case "sceneCreated":
      return `scene "${p.value.name}"`;
    case "actorAdded":
      return `actor ${p.value.actor?.name || p.value.actor?.actorId || "?"} joined`;
    case "tokenPlaced":
      return `${p.value.tokenId} placed at ${p.value.position?.x ?? 0},${p.value.position?.y ?? 0}`;
    case "tokenMoved":
      return `${p.value.tokenId} moved to ${p.value.to?.x ?? 0},${p.value.to?.y ?? 0}`;
    case "conditionApplied":
      return `${p.value.actorId} gained ${p.value.conditionId}`;
    case "conditionRemoved":
      return `${p.value.actorId} lost ${p.value.conditionId}`;
    case "resourceChanged":
      return `${p.value.actorId} ${p.value.resource} → ${p.value.newValue}`;
    case "noteUpserted":
      return `note "${p.value.key}" updated`;
    case "noteDeleted":
      return `note "${p.value.key}" deleted`;
    case "abilityUsed":
      return `${p.value.actorId} used ${p.value.abilityId}`;
    case "attackRolled":
      return `roll ${p.value.total}`;
    case "adventureLoaded":
      return `adventure ${p.value.adventureId} loaded`;
    default:
      return p.case ?? "event";
  }
}

function renderGrid(st: State, sceneId: string): HTMLElement {
  const scene = st.Scenes[sceneId];
  const wrap = el("section", "board");
  if (!scene) {
    wrap.appendChild(el("p", "empty", "No scene yet."));
    return wrap;
  }
  wrap.appendChild(el("h2", undefined, scene.Name));

  const geom: Geometry = { cell: CELL, width: scene.GridWidth, height: scene.GridHeight };
  const board = el("div", "grid");
  board.style.width = `${geom.width * CELL}px`;
  board.style.height = `${geom.height * CELL}px`;
  board.style.backgroundSize = `${CELL}px ${CELL}px`;
  board.dataset["sceneId"] = sceneId;

  for (const d of tokensOnScene(st, sceneId)) {
    board.appendChild(renderDisc(d));
  }
  wrap.appendChild(board);
  return wrap;
}

function renderDisc(d: TokenDisc): HTMLElement {
  const t = el("div", "token");
  t.style.left = `${d.x * CELL}px`;
  t.style.top = `${d.y * CELL}px`;
  t.title = d.name || d.actorId;
  t.dataset["tokenId"] = d.tokenId;
  t.appendChild(el("span", "initial", d.initial));

  // ALL resources, ALL conditions — no notion of a "primary" one exists in
  // the ruleset format, and inventing one here would bake in a genre.
  if (d.resources.length > 0) {
    const chips = el("div", "chips");
    for (const r of d.resources) {
      const chip = el("span", "chip", `${r.current}/${r.max}`);
      chip.title = r.name; // the NAME on hover, never abbreviated on the face
      chips.appendChild(chip);
    }
    t.appendChild(chips);
  }
  if (d.conditions.length > 0) {
    const dots = el("div", "dots");
    for (const c of d.conditions) {
      const dot = el("span", "dot");
      dot.title = c.id; // hover names, per the spectator floor
      dots.appendChild(dot);
    }
    t.appendChild(dots);
  }
  return t;
}

function renderFeed(entries: FeedEntry[]): HTMLElement {
  const wrap = el("section", "feed");
  wrap.appendChild(el("h2", undefined, "Story"));
  if (entries.length === 0) {
    wrap.appendChild(el("p", "empty", "Nothing has happened yet."));
    return wrap;
  }
  for (const entry of entries) {
    const item = el("article", "beat");
    for (const n of entry.narrations) {
      const line = el("p", n.as ? "speech" : "narration");
      if (n.as) line.appendChild(el("span", "speaker", `${n.as}: `));
      line.appendChild(document.createTextNode(n.text));
      item.appendChild(line);
    }
    for (const e of entry.events) {
      item.appendChild(el("p", "mechanical", describe(e)));
    }
    wrap.appendChild(item);
  }
  return wrap;
}

function renderNotes(st: State): HTMLElement {
  const wrap = el("section", "notes");
  wrap.appendChild(el("h2", undefined, "Notes"));
  const keys = Object.keys(st.Notes).sort();
  if (keys.length === 0) {
    wrap.appendChild(el("p", "empty", "No notes."));
    return wrap;
  }
  for (const k of keys) {
    const n = st.Notes[k]!;
    const item = el("article", "note");
    item.appendChild(el("h3", undefined, n.Title || k));
    item.appendChild(el("p", undefined, n.Text));
    wrap.appendChild(item);
  }
  return wrap;
}

function renderTicker(log: Envelope[]): HTMLElement {
  const wrap = el("section", "ticker");
  wrap.appendChild(el("h2", undefined, "Events"));
  // Newest first, and bounded: an all-night session is thousands of events and
  // a spectator only ever looks at the recent tail.
  for (const e of log.slice(-40).reverse()) {
    const row = el("p", "tick");
    row.appendChild(el("span", "seq", `#${e.sequence}`));
    row.appendChild(document.createTextNode(` ${describe(e)}`));
    wrap.appendChild(row);
  }
  return wrap;
}

function renderStatus(st: State, status: string): HTMLElement {
  const open = st.Sessions.find((s) => s.EndSeq === 0);
  const wrap = el("header", "status");
  wrap.appendChild(el("span", "conn", status));
  wrap.appendChild(
    el("span", "session", open ? `session: ${open.Name}` : `sessions: ${st.Sessions.length}`),
  );
  return wrap;
}

/** Replace root's contents with the spectator view. */
export function renderSpectator(
  root: HTMLElement,
  st: State,
  log: Envelope[],
  status: string,
): void {
  // The active scene is the most recently created one: there is no
  // "current scene" on the wire yet, and picking the newest matches what a
  // DM just made. A scene selector belongs with the DM console (T8).
  const sceneId = Object.keys(st.Scenes).sort().at(-1) ?? "";

  root.replaceChildren(
    renderStatus(st, status),
    renderGrid(st, sceneId),
    renderFeed(buildFeed(log)),
    renderNotes(st),
    renderTicker(log),
  );
}
