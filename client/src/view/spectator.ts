// The spectator floor (client spec §4): scene grid, story feed, notes panel,
// session status, event ticker.
//
// Deliberately thin. Everything with a decision in it — which cell a click
// lands on, how narration groups with the events it describes — lives in
// grid.ts and feed.ts, where it is unit-tested without a browser. What is
// here is DOM assembly, which is the part a screenshot verifies better than
// an assertion could.

import type { State } from "../state";
import type { Participant } from "../session";
import { ActorKind, type Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";
import { buildFeed, type FeedEntry } from "./feed";
import { cellFromPoint, tokensOnScene, type Geometry, type TokenDisc } from "./grid";
import { fitCamera, worldFromScreen, type Camera } from "./camera";
import { planFog, planGrid, planScene } from "./scene-plan";
import { paint, shadeFog, strokeGrid, type ImageMap } from "./canvas";

export const CELL = 44;

// The pane is a FIXED size, independent of the scene's grid dimensions —
// this is backlog T1/#19 (spec §1.4, §7): the old board was gridWidth*CELL
// px tall (1408 for a 32x32 scene), so the page grew with the map and the
// controls sat ~1450px down it, below every laptop fold. A 200x200 outdoor
// map and a 10x10 room now lay out identically; the camera (fitCamera) is
// what makes the whole scene visible inside whichever of the two binds.
const PANE_W = 640;
const PANE_H = 480;

// The default when app.ts has not (yet, or ever) supplied one: extras.images
// is optional (a spectator view built directly in a test, say, rarely has a
// live pack to load), and an empty map keeps the canvas honestly blank
// rather than inventing a picture — canvas.ts's paint() already skips an
// unresolved key rather than throwing (see its own test). pack-assets.ts
// (Task 10) is what actually populates a real one, over HTTP from
// GET /api/packs/{pack}/{file}; app.ts wires its result in as extras.images.
const NO_IMAGES: ImageMap = {};

/**
 * boardCamera is the ONE fit renderGrid uses -- for the canvas terrain, for
 * token discs, and for click resolution alike. Exported so a test can derive
 * the exact expected transform (via this, not a hand-copied PANE_W/PANE_H
 * pair) rather than duplicating the fit arithmetic and risking silent drift
 * from whatever this function actually computes.
 */
export function boardCamera(width: number, height: number): Camera {
  return fitCamera(width, height, CELL, PANE_W, PANE_H);
}

function el(tag: string, cls?: string, text?: string): HTMLElement {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  // Stryker disable next-line ConditionalExpression: same case as view/dm.ts's
  // el(). Removing the guard sets textContent to undefined, which a real
  // browser stringifies to the word "undefined" above every container, while
  // happy-dom coerces it to "" and creates no text node at all (measured).
  // Observable in production, invisible to the harness — so NOT equivalent,
  // and deliberately not filed in ts-mutation-equivalents.txt. The e2e in a
  // real browser is where it would surface.
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

function renderGrid(
  st: State,
  sceneId: string,
  images: ImageMap,
  // Whether the door tool is armed (Task 4, spec §8) — shown on the board
  // itself, not only in whichever panel armed it, so a DM who arms doors,
  // walks away and comes back can still see why their clicks have stopped
  // moving tokens. NO DEFAULT, deliberately: renderGrid is module-private
  // with exactly one call site (renderSpectator's own, below), which always
  // passes a value, so a default here would be dead code no test could ever
  // reach through it — a `= false` here survived as an unadjudicated
  // Stryker mutant for exactly that reason (fix round 1). Do not re-add
  // one; there is no second caller for it to serve. Placed ahead of the two
  // optional/defaulted parameters below it: TypeScript refuses a required
  // parameter after an optional one.
  doorsArmed: boolean,
  onCell?: (c: { x: number; y: number }) => void,
  // TEST-ONLY SEAM (review finding C4, 2026-08-16): how this function
  // obtains a 2D context. Defaults to the real canvas.getContext, which is
  // what every production call site gets — app.ts never passes
  // extras.getContext, and there is no reason it ever should; a real
  // browser's canvas always answers "2d" with a context. The override
  // exists because happy-dom's canvas.getContext ALWAYS returns null (this
  // file's own comment on `ctx` below), so nothing past that call — the
  // actual paint()/strokeGrid() wiring, including their relative order —
  // was reachable by the suite before this seam existed. Kept minimal
  // deliberately: one optional function parameter, not a restructuring of
  // the drawing layer (spectator.ts's thinness is the design, not an
  // oversight to fix).
  getContext: (canvas: HTMLCanvasElement) => CanvasRenderingContext2D | null = (c) => c.getContext("2d"),
): HTMLElement {
  const scene = st.Scenes[sceneId];
  const wrap = el("section", "board");
  if (!scene) {
    wrap.appendChild(el("p", "empty", "No scene yet."));
    return wrap;
  }
  wrap.appendChild(el("h2", undefined, scene.Name));
  // A LEGIBLE LABEL, beside the class below on the canvas container: the
  // class is what a stylesheet hooks (a border, a cursor), and is not
  // itself something a returning DM reads off the page — the label is the
  // part that actually answers "why did my click just do nothing" (spec §8).
  if (doorsArmed) {
    wrap.appendChild(el("p", "armed-label", "Doors armed — click a door to open or close it."));
  }

  const geom: Geometry = { cell: CELL, width: scene.GridWidth, height: scene.GridHeight };
  // Computed BEFORE anything below reads it (fix round: this used to run
  // after the backgroundSize assignment, which is what let backgroundSize
  // hardcode CELL instead of CELL * cam.scale unnoticed for as long as
  // nothing was drawn on the canvas to visibly disagree with it -- see the
  // backgroundSize assignment's own comment).
  const cam = boardCamera(scene.GridWidth, scene.GridHeight);

  const board = el("div", "grid");
  // NO inline width/height keyed to the scene (T1/#19, see PANE_W/PANE_H
  // above) -- the pane's size comes from style.css and stays fixed.
  //
  // NO CSS BACKGROUND LATTICE EITHER: strokeGrid draws the grid on the canvas
  // below, through the camera. A CSS tiling cannot be made to agree with it,
  // because it tiles the whole pane from the PANE's origin while the canvas
  // grid starts at the camera's OFFSET and spans only the SCENE. Those line up
  // only when the offset is a whole multiple of the step -- true for the 10x9
  // demo map by coincidence, false for a 32x32 one. It also ruled the
  // letterboxed margin, drawing squares outside the map that the server would
  // refuse to move a token onto.
  board.dataset["sceneId"] = sceneId;
  // THE CANVAS CONTAINER carries the class, not `wrap` (the outer section):
  // this div is what directly wraps the `<canvas>` a click lands on, so a
  // stylesheet styling "the board itself" (a border, a cursor change) hooks
  // here rather than on the section that also holds the scene name heading.
  if (doorsArmed) board.classList.add("armed");

  const canvas = document.createElement("canvas");
  canvas.width = PANE_W;
  canvas.height = PANE_H;
  board.appendChild(canvas);

  // Fit the WHOLE scene into the pane (spec §7: "always start seeing the
  // whole map"), turn that into draw instructions (Task 8, pure and fully
  // tested), and hand them to Task 9's thin drawImage loop. ctx is null under
  // happy-dom -- canvas.ts's header comment explains why -- but never in a
  // real browser, which always returns one for "2d". getContext is the
  // TEST-ONLY seam above; production always uses its default, the real
  // canvas.getContext.
  const ctx = getContext(canvas);
  if (ctx) {
    const ops = planScene(st, sceneId, cam, CELL, PANE_W, PANE_H);
    paint(ctx, ops, images);
    // THEN the fog, over the terrain it dims and under the lattice that
    // divides it (spec §6.1's order: terrain, fog, grid). Drawn before the
    // terrain it would be painted over and remembered ground would look lit;
    // drawn after the grid it would dim the lattice too, making a room you
    // remember harder to count for no gain.
    shadeFog(ctx, planFog(st, sceneId, cam, CELL, PANE_W, PANE_H));
    // AFTER the tiles, deliberately: the lattice divides the terrain, so it
    // belongs on top of it. Drawn first, every tile would paint over it and the
    // board would be uncountable again — which is exactly what happened to the
    // old CSS background-size lattice the moment canvas terrain arrived.
    strokeGrid(ctx, planGrid(st, sceneId, cam, CELL, PANE_W, PANE_H));
  }

  if (onCell) {
    board.style.cursor = "crosshair";
    board.addEventListener("click", (ev) => {
      // Offset from the board's own box, so the cell is right regardless of
      // where the board sits in the page or how far it is scrolled -- THEN
      // through worldFromScreen (Task 8, algebraically exact inverse of the
      // same cam the canvas terrain draws through), so a click resolves to
      // the square under the cursor at any scale/offset, not just at the
      // scale-1/offset-0 case where skipping this step is invisible.
      const r = board.getBoundingClientRect();
      const world = worldFromScreen(ev.clientX - r.left, ev.clientY - r.top, cam);
      onCell(cellFromPoint(world.x, world.y, geom));
    });
  }

  // scene.Visible, NOT a set derived here: the seam is grid.ts's, and this
  // call site's only job is to hand it the one this seat was actually sent.
  // Undefined (the DM's stream, which carries no sceneSeen) draws every token;
  // see tokensOnScene on why that is not the same as an empty set.
  for (const d of tokensOnScene(st, sceneId, scene.Visible)) {
    board.appendChild(renderDisc(d, cam));
  }
  wrap.appendChild(board);
  return wrap;
}

/**
 * renderDisc positions and SIZES a token disc through the SAME camera the
 * canvas terrain and click resolution use -- both, because a token scaled in
 * position but not in size would float free of its own square at any scale
 * but 1 (fix round 1 finding: this drew at raw `x * CELL`, invisible only
 * because nothing yet on the canvas gave it anything to visibly disagree
 * with).
 *
 * The disc's INNER content is scaled too (Task 10 cosmetic fix), by the same
 * cam.scale: style.css's 30px/8px/5px figures are the scale-1 defaults, and
 * were the ONLY sizes anything drew at before a camera existed at all. Once
 * fitCamera can shrink a token well below 44px (goblin-ambush's 32x32 fits
 * at ~15px per cell), the 30px initial circle overflowed its own now-smaller
 * box -- invisible until there was real art on the canvas to judge it
 * against, same story as the backgroundSize fix above. Scaling the CHIP and
 * DOT sizes too, not just the initial, for the same reason: nothing about a
 * token's content should be the one part of the board immune to the camera
 * that governs everything else on it.
 */
function renderDisc(d: TokenDisc, cam: Camera): HTMLElement {
  const t = el("div", "token");
  const size = CELL * cam.scale;
  const scale = cam.scale;
  t.style.left = `${d.x * CELL * cam.scale + cam.offsetX}px`;
  t.style.top = `${d.y * CELL * cam.scale + cam.offsetY}px`;
  t.style.width = `${size}px`;
  t.style.height = `${size}px`;
  t.title = d.name || d.actorId;
  t.dataset["tokenId"] = d.tokenId;

  const initial = el("span", "initial", d.initial);
  initial.style.width = `${30 * scale}px`;
  initial.style.height = `${30 * scale}px`;
  initial.style.fontSize = `${16 * scale}px`;
  t.appendChild(initial);

  // ALL resources, ALL conditions — no notion of a "primary" one exists in
  // the ruleset format, and inventing one here would bake in a genre.
  if (d.resources.length > 0) {
    const chips = el("div", "chips");
    for (const r of d.resources) {
      const chip = el("span", "chip", `${r.current}/${r.max}`);
      chip.title = r.name; // the NAME on hover, never abbreviated on the face
      chip.style.fontSize = `${8 * scale}px`;
      chips.appendChild(chip);
    }
    t.appendChild(chips);
  }
  if (d.conditions.length > 0) {
    const dots = el("div", "dots");
    for (const c of d.conditions) {
      const dot = el("span", "dot");
      dot.title = c.id; // hover names, per the spectator floor
      dot.style.width = `${5 * scale}px`;
      dot.style.height = `${5 * scale}px`;
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

/**
 * The spectator's perch: which shoulder they are riding, and the party to
 * choose from (visibility spec §3.1.1).
 *
 * `current` is the shoulder the SERVER has confirmed, never the last one
 * clicked — see app.ts, which only moves it when the command comes back ok.
 * "" is a real value here and means perched on nobody.
 */
export interface PerchControl {
  current: string;
  onPerch: (actorId: string) => void;
}

/**
 * A LIST, NOT A CLICK ON A TOKEN, and that is forced rather than chosen.
 *
 * An unperched spectator has NO EYES, so their board is empty — there is no
 * token on it to click, and direct manipulation cannot bootstrap itself. What
 * makes the list possible is that spec §5's party-member exception is not
 * gated on eyes: internal/gateway/project.go's look() walks st.Actors flat and
 * marks every party member known, so an unperched spectator is told the whole
 * party and nothing else. That is where these names come from. Clicking a
 * token could be added ON TOP of this later; it could never replace it.
 *
 * PARTY MEMBERS ONLY, by KIND (spec §5.1). Not "has a controller" — that was
 * the defect that ruling closed, and it made the Goblin Archer perchable the
 * moment a DM took control of it. The server refuses such a perch anyway
 * (MayPerch); this is the UI not offering the click in the first place, which
 * is a different job and not a substitute for it.
 */
function renderPerch(st: State, perch: PerchControl): HTMLElement {
  const wrap = el("section", "perch");
  wrap.appendChild(el("h2", undefined, "Whose eyes"));

  const party = Object.values(st.Actors).filter((a) => a.kind === ActorKind.PARTY_MEMBER);
  // Sorted by name, ties on the id — the same comparator and the same reasoning
  // as session.ts's participant list: arrival order (here, object key order) is
  // an accident of the log, and a list that reshuffles is hard to read and
  // impossible to test stably. Two arms, not three: actorId is st.Actors' KEY,
  // so no two entries can share one and an "equal" arm would be unreachable.
  party.sort((a, b) =>
    label(a, a.actorId) === label(b, b.actorId)
      ? (a.actorId < b.actorId ? -1 : 1)
      : (label(a, a.actorId) < label(b, b.actorId) ? -1 : 1),
  );

  wrap.appendChild(el("p", "perched-on", `Perched on: ${label(st.Actors[perch.current], perch.current)}`));

  // OFFERED AT ANY SIZE, INCLUDING NONE. A control that vanished when the
  // party list was empty would make "nobody has a character yet" look
  // identical to "the perch is broken", which is the reading a watcher
  // reaches for the moment their board is blank.
  if (party.length === 0) {
    wrap.appendChild(el("p", "empty", "No party members yet."));
  }
  for (const a of party) {
    const btn = document.createElement("button");
    btn.className = a.actorId === perch.current ? "shoulder on" : "shoulder";
    btn.textContent = label(a, a.actorId);
    btn.addEventListener("click", () => perch.onPerch(a.actorId));
    wrap.appendChild(btn);
  }

  // THE EMPTY ID IS AN OPTION, not the absence of one. "Naming no actor is how
  // a bird LEAVES a shoulder without immediately sitting on another"
  // (internal/gateway/viewpoint.go). It sends, like every other choice here.
  const off = document.createElement("button");
  off.className = perch.current === "" ? "unperch on" : "unperch";
  off.textContent = "No shoulder";
  off.addEventListener("click", () => perch.onPerch(""));
  wrap.appendChild(off);

  return wrap;
}

/**
 * How one shoulder reads: its actor's name, its id when the actor has none or
 * is not in this roster, and "nobody" for the empty id.
 *
 * THREE ANSWERS, because there are three states and collapsing any two of them
 * loses something a watcher needs. "" is perched on nobody, which is where
 * every connection starts. An id the roster does not hold is a shoulder this
 * client asked for and can no longer name — a log that does not reach back to
 * that actor's ActorAdded lands here — and showing "nobody" for it would say
 * the board is blank because they chose that, when it is blank because the
 * character is gone.
 */
function label(a: { name: string } | undefined, id: string): string {
  if (id === "") return "nobody";
  return a?.name || id;
}

function renderStatus(st: State, status: string, extras: ViewExtras): HTMLElement {
  const open = st.Sessions.find((s) => s.EndSeq === 0);
  const wrap = el("header", "status");
  wrap.appendChild(el("span", "conn", status));
  wrap.appendChild(
    el("span", "session", open ? `session: ${open.Name}` : `sessions: ${st.Sessions.length}`),
  );

  // Shown at ANY size, including one. Hiding a single-entry list would make
  // "nobody else is here" indistinguishable from "presence is broken", which
  // is the reading a player reaches for the moment they are unexpectedly
  // alone.
  const who = el("span", "present");
  for (const p of extras.participants ?? []) {
    who.appendChild(el("span", "participant", p.displayName));
  }
  wrap.appendChild(who);

  // MANUAL reconnect (spec §3.4). No timer and no backoff: the server cannot
  // know when someone's network came back, so a guess either hammers a dead
  // link or rejoins a session the person has left. Offered ONLY when the
  // connection is actually closed — a button present on a healthy session
  // just drops it for no reason.
  if (status === "closed" && extras.onReconnect) {
    const btn = document.createElement("button");
    btn.className = "reconnect";
    btn.textContent = "Reconnect";
    btn.addEventListener("click", extras.onReconnect);
    wrap.appendChild(btn);
  }
  return wrap;
}

/** Replace root's contents with the spectator view. */
export interface ViewExtras {
  // `| undefined` is explicit rather than relying on `?` alone:
  // exactOptionalPropertyTypes distinguishes "absent" from "present and
  // undefined", and callers build this object with conditional expressions
  // that produce the latter.
  /** Player panel, when the role has one. */
  panel?: HTMLElement | undefined;
  /** DM console, for dm and agent roles only. */
  console?: HTMLElement | undefined;
  /** Board click handler, when the viewer may act. */
  onCell?: ((c: { x: number; y: number }) => void) | undefined;
  /** Transient message from the last command. */
  toast?: string | undefined;
  /** Who is currently at the table (T5). */
  participants?: Participant[] | undefined;
  /** Redial, offered only while the connection is closed. */
  onReconnect?: (() => void) | undefined;
  /**
   * Whose eyes this spectator is looking through, and how to change it.
   *
   * ABSENT FOR EVERY OTHER ROLE, which is the whole of the negative case:
   * MayPerch refuses a player, a DM and an agent outright ("role %q does not
   * perch — a viewpoint is the spectator's"), so offering them the control
   * would be an affordance that can only ever produce a refusal. app.ts is
   * what withholds it; renderSpectator honours the omission rather than
   * defaulting to a control.
   */
  perch?: PerchControl | undefined;
  /**
   * Real pack art, resolved by pack-assets.ts and loaded by app.ts (Task
   * 10) — keyed exactly as canvas.ts's paint() expects. Omitted (or not yet
   * resolved) draws NO_IMAGES, which is not a failure: paint() skips an
   * unresolved key rather than throwing, so a scene renders with whatever
   * art has loaded so far.
   */
  images?: ImageMap | undefined;
  /**
   * TEST-ONLY SEAM (review finding C4, 2026-08-16): overrides how renderGrid
   * obtains its 2D context. Never set by app.ts — see renderGrid's own doc
   * comment on the parameter this threads into for the full reasoning.
   */
  getContext?: ((canvas: HTMLCanvasElement) => CanvasRenderingContext2D | null) | undefined;
  /**
   * Whether the door tool is armed on THIS board (Task 4, spec §8) — the
   * same bit app.ts's `ui.doorsArmed` shares with the DM console and the
   * player panel, threaded here so the board shows it too. Omitted (or
   * false) draws the board exactly as before this task; renderGrid's own
   * comment covers what "armed" adds.
   */
  doorsArmed?: boolean | undefined;
}

export function renderSpectator(
  root: HTMLElement,
  st: State,
  log: Envelope[],
  status: string,
  extras: ViewExtras = {},
): void {
  // The active scene is the most recently created one: there is no
  // "current scene" on the wire yet, and picking the newest matches what a
  // DM just made. A scene selector belongs with the DM console (T8).
  const sceneId = Object.keys(st.Scenes).sort().at(-1) ?? "";

  const nodes: HTMLElement[] = [
    renderStatus(st, status, extras),
    renderGrid(st, sceneId, extras.images ?? NO_IMAGES, extras.doorsArmed ?? false, extras.onCell, extras.getContext),
    renderFeed(buildFeed(log)),
    renderNotes(st),
    renderTicker(log),
  ];
  // Only the spectator is ever given one, and a spectator has neither a panel
  // nor a console, so this never actually competes with them for position.
  if (extras.perch) nodes.push(renderPerch(st, extras.perch));
  if (extras.panel) nodes.push(extras.panel);
  if (extras.console) nodes.push(extras.console);
  if (extras.toast) {
    const t = el("div", "toast", extras.toast);
    nodes.push(t);
  }
  root.replaceChildren(...nodes);
}
