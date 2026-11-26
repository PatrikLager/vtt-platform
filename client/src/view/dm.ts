// The DM console: run the table. Keep every destructive or structural action
// here rather than in the player panel. Ask first before an action nothing
// undoes, whose damage reaches people not here to be told: New link strands
// every copy of the old one, and Boot removes a participant for good
// (VTT-569). Keep invite management out: it stays `vtt invite`.

import type { State } from "../state";
import type { Participant } from "../session";
import type { Ability, AdventureMeta, CatalogueEntry, ConditionDef, JoinLink, MapMeta, PlaceDef, Property, Roster } from "../metadata";
import { ActorKind, NoteVisibility } from "../../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand, CommandResult } from "../../../contract/gen/ts/vtt/v1/commands_pb";
import {
  startSession, endSession, placeToken, describeToken, removeToken, loadAdventure, loadMap,
  upsertNote, deleteNote, removeCondition, parseActorJSON, addActor, removeActor,
  giveAbility, takeAbility, addReference, changeReference, removeReference,
  grantActorControl, revokeActorControl, addNarration, speakOutOfCharacter, addWhisper,
  setJoinDoor,
  rotateJoinLink,
  promoteParticipant,
  removeParticipant,
} from "../commands";
import { readLine } from "../player";
import { namer } from "./naming";

/** whisperTargets resolves /w's name to party member ids, or says why it cannot (VTT-593, VTT-594). */
export function whisperTargets(name: string, st: State, people: ReadonlyMap<string, { name: string }>): string[] | string {
  if (name === "") return "Say whom to whisper to: /w Name text.";
  const same = (n: string) => n.toLowerCase() === name.toLowerCase();
  const party = Object.values(st.Actors).filter((a) => a.kind === ActorKind.PARTY_MEMBER);
  const named = party.filter((a) => same(a.name));
  if (named.length > 1) return `Two party members are named ${name}; rename one to whisper to it.`;
  if (named.length === 1) return [named[0]!.actorId];
  const who = [...people].find(([, p]) => same(p.name));
  if (who === undefined) return `Nobody at the table is named ${name}.`;
  const theirs = party.filter((a) => a.controllerIds.includes(who[0])).map((a) => a.actorId);
  return theirs.length > 0 ? theirs.sort() : `${who[1].name} controls no party member to whisper to.`;
}

function el(tag: string, cls?: string, text?: string): HTMLElement {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  // Stryker disable next-line ConditionalExpression: only forced true is equivalent
  // and only while n is empty, yet this hides forced false too: drop the directive
  // before n can hold children when this line runs.
  if (text !== undefined) n.textContent = text;
  return n;
}

// What the DM has typed, kept ACROSS RE-RENDERS.
//
// The console is rebuilt whenever an event arrives, and at a live table
// events arrive constantly — every move, every roll, every line of narration.
// Without this, each rebuild replaced the inputs with empty ones and the DM
// lost whatever they were part-way through typing. In practice that made the
// longer forms impossible to complete at all: a note or a pasted actor takes
// longer to type than the gap between two events.
const draft: Record<string, string> = {};

/** Clear a field's remembered text, after the command it fed succeeded. */
function clearDraft(...fields: string[]): void {
  for (const f of fields) delete draft[f];
}

// Keep the locks beside `draft`, outside renderDMConsole: the console is
// rebuilt at every paint, and a lock held inside it would not last (VTT-523).
const unanswered = new Set<string>();

interface Sending {
  command: ClientCommand;
  clears: string[];
  /** Re-read the roster once answered: the command changes control (VTT-890). */
  rereads?: boolean;
}

/**
 * Take `key` only once a command is sent, free it at any answer (VTT-523);
 * clear only at ok (VTT-517, VTT-533).
 */
function sending(d: DMDeps, key: string, press: () => Sending | undefined): () => void {
  return () => {
    if (unanswered.has(key)) return;
    const out = press();
    if (out === undefined) return;
    unanswered.add(key);
    void d.send(out.command)
      .then((res) => {
        if (!res.ok) return;
        clearDraft(...out.clears);
        d.rerender();
        if (out.rereads) d.refreshSharing();
      }, (e: unknown) => console.error(e))
      .finally(() => unanswered.delete(key));
  };
}

/**
 * The wire NAME a kind <select> carries, back to the enum the command needs.
 *
 * Returns null for "nothing chosen", and that is the point rather than an
 * inconvenience: "the DM did not answer" is a THIRD state, distinct from both
 * kinds (visibility spec §5.1). Collapsing it into ACTOR_KIND_UNSPECIFIED and
 * sending it would reproduce on the client exactly the ambiguity the server
 * refuses — an omission indistinguishable from a decision. An unrecognised
 * string falls into the same bucket, which fails closed: nothing is sent at
 * all rather than sent saying something nobody chose.
 *
 * BOTH CALLERS, since actor-kind Task 7 — the Add actor form and the per-actor
 * grant row. This used to say "a grant is not sent", which was the whole truth
 * for one day: creating an actor asks the same question now, and a null here
 * stops that command too.
 */
function actorKindFromWireName(name: string): ActorKind | null {
  switch (name) {
    case "ACTOR_KIND_PARTY_MEMBER":
      return ActorKind.PARTY_MEMBER;
    case "ACTOR_KIND_NON_PARTY":
      return ActorKind.NON_PARTY;
    default:
      return null;
  }
}

const KIND_CHOICES = [
  ["", "what is it?"],
  ["ACTOR_KIND_PARTY_MEMBER", "party member"],
  ["ACTOR_KIND_NON_PARTY", "monster / NPC"],
] as const;

const VISIBILITY_CHOICES = [
  ["", "who may read it?"],
  ["NOTE_VISIBILITY_PUBLIC", "public — every player and spectator"],
  ["NOTE_VISIBILITY_SECRET", "DM only"],
] as const;

function noteVisibilityFromWireName(name: string): NoteVisibility | null {
  switch (name) {
    case "NOTE_VISIBILITY_PUBLIC":
      return NoteVisibility.PUBLIC;
    case "NOTE_VISIBILITY_SECRET":
      return NoteVisibility.SECRET;
    default:
      return null;
  }
}

/**
 * choiceSelect asks one closed question. Keep the blank first option where
 * nothing answers it yet, so "the DM did not answer" stays visible: the server
 * refuses an unstated kind (SPEC-013). `field` is the draft key; `present` is
 * what shows with no draft, as a Describe box shows its token's text.
 */
function choiceSelect(
  cls: string, field: string, choices: readonly (readonly [string, string])[], present?: string,
): HTMLSelectElement {
  const sel = document.createElement("select");
  sel.className = cls;
  for (const [value, label] of choices) {
    const o = document.createElement("option");
    o.value = value;
    o.textContent = label;
    sel.appendChild(o);
    // Select on the option, never by assigning `sel.value` after the loop: a
    // draft no option carries leaves nothing selected in a browser, while
    // happy-dom resets to the first option and hides it. An absent draft with
    // no present matches no option, so the browser's reset shows the blank
    // first one.
    // "a remembered kind selects THAT option, and only that one" holds this.
    o.selected = value === (draft[field] ?? present);
  }
  sel.addEventListener("change", () => {
    draft[field] = sel.value;
  });
  return sel;
}

// Every input carries a stable data-field. Placeholders are prose — they get
// reworded, and they match by substring, so "name" also selects "session
// name". A test that pins a field by its wording breaks on a copy edit and
// silently targets the wrong box when two placeholders overlap.
function input(placeholder: string, field: string, cls = "", presentText = ""): HTMLInputElement {
  const i = document.createElement("input");
  i.placeholder = placeholder;
  i.dataset["field"] = field;
  i.value = draft[field] ?? presentText;
  i.addEventListener("input", () => {
    draft[field] = i.value;
  });
  if (cls) i.className = cls;
  return i;
}

function textarea(placeholder: string, field: string, presentText: string): HTMLTextAreaElement {
  const t = document.createElement("textarea");
  t.placeholder = placeholder;
  t.dataset["field"] = field;
  t.className = "wide";
  t.value = draft[field] ?? presentText;
  t.addEventListener("input", () => {
    draft[field] = t.value;
  });
  return t;
}

const WHOLE = "a whole number from -2147483648 to 2147483647";

// Refuse what a Resource's int32 cannot hold: the server closes on a frame it
// cannot decode (VTT-808).
function int32(text: string): number | undefined {
  const n = Number(text);
  return /^[+-]?\d+$/.test(text) && n >= -2147483648 && n <= 2147483647 ? n : undefined;
}

function button(label: string, onClick: () => void, action?: string): HTMLButtonElement {
  const b = document.createElement("button");
  b.className = "chip";
  b.textContent = label;
  if (action) b.dataset["action"] = action;
  b.addEventListener("click", onClick);
  return b;
}

function group(title: string, ...nodes: HTMLElement[]): HTMLElement {
  const g = el("div", "dmgroup");
  g.appendChild(el("h3", undefined, title));
  const row = el("div", "row");
  row.append(...nodes);
  g.appendChild(row);
  return g;
}

export interface DMDeps {
  st: State;
  /**
   * Who is at the table, for the control panel's grant target.
   *
   * Presence is connection-scoped and control is campaign-scoped (spec §3.1),
   * so this list is a CONVENIENCE for naming people, never the authority on
   * who holds a character. A controller who is offline appears in no entry
   * here and must still be rendered — by id — or a character they hold looks
   * unowned to the DM.
   */
  participants: Participant[];
  adventures: AdventureMeta[];
  /**
   * Every standalone map the server loaded at boot (GET /api/maps), already
   * fetched by app.ts for pack loading. Empty renders no group: a Maps
   * heading with nothing under it tells a DM their maps failed to load, which
   * is a different and false story from "this table has none configured".
   */
  maps: MapMeta[];
  /** The abilities the Abilities group offers to give (VTT-730). */
  abilities: readonly Ability[];
  /** The ruleset's properties, naming a resource a give asks the value of (VTT-806). */
  properties?: readonly Property[] | undefined;
  /** The ruleset's conditions, naming a condition a removal button removes (VTT-864). */
  conditions?: readonly ConditionDef[] | undefined;
  /** The ruleset's catalogue, whose entries the Catalogue group names and offers (VTT-1057). */
  catalogue?: readonly CatalogueEntry[] | undefined;
  /** The ruleset's places, which the Catalogue group names and offers (VTT-1057). */
  places?: readonly PlaceDef[] | undefined;
  guideFor: (id: string) => Promise<string | null>;
  /**
   * The shared join link, or null when there is none to show.
   *
   * null means "do not render a sharing panel at all" — either the fetch
   * failed or this console belongs to somebody the route refuses. An empty
   * panel would be worse than none: a DM might paste a blank link.
   */
  joinLink: JoinLink | null;
  /**
   * Everyone at the table and what they may do, read from the server rather
   * than presence: presence is connection-scoped and carries no role, so a
   * role taken from it would go stale the moment a grant or a revocation
   * moved it (VTT-890).
   */
  roster: Roster[] | null;
  /** Where this table lives, for building the link a DM can actually paste. */
  origin: string;
  /**
   * Re-read the link and roster.
   *
   * The door commands produce NO EVENT, deliberately (spec §4), so nothing
   * re-renders on its own after one. Without this the DM opens the door and
   * the console goes on saying "closed" until something unrelated happens.
   */
  refreshSharing: () => void;
  /**
   * Issue a command, resolving to its CommandResult once answered. Keep the
   * promise: the roster and the door are read over HTTP, and a re-read fired
   * beside the command races it (VTT-904, VTT-933). Keep the result, not void:
   * it carries CommandResult.warnings.
   */
  send: (c: ClientCommand) => Promise<CommandResult>;
  /** Repaint the table, once an accepted command has cleared its form (VTT-533). */
  rerender: () => void;
  notify: (msg: string) => void;
  confirm: (msg: string) => boolean;
  /** Whether the door tool is armed: app.ts's `ui.doorsArmed`, which the board shows too (SPEC-022). */
  doorsArmed: boolean;
  /**
   * Flip doorsArmed and repaint. A LOCAL mode change, not a command — it
   * sends nothing over the wire, so unlike every `d.send(...)` call above
   * there is no server round trip to await; the caller (app.ts) mutates the
   * shared bit and calls paint() itself.
   */
  toggleDoors: () => void;
}

export function renderDMConsole(d: DMDeps): HTMLElement {
  const wrap = el("section", "dm");
  wrap.appendChild(el("h2", undefined, "DM console"));

  // VTT-527 VTT-528 VTT-600 VTT-530 VTT-531 VTT-533 VTT-535 VTT-593 VTT-594
  const sayAs = input("in character as… (optional)", "say-as");
  const sayText = input("say or narrate, /ooc to step out of character, /w Name to whisper", "say-text");
  const say = sending(d, "add-narration", () => {
    const line = readLine(sayText.value);
    if (line.kind === "toDM") return void d.notify("Whisper to a party member with /w Name text.");
    if (line.kind === "whisper") {
      const to = whisperTargets(line.name, d.st, known);
      if (typeof to === "string") return void d.notify(to);
      if (line.text === "") return void d.notify("Say what to whisper: /w Name text.");
      return { command: addWhisper(line.text, to), clears: ["say-text"] };
    }
    if (line.text === "") return;
    const as = sayAs.value.trim() || undefined;
    return {
      command: line.kind === "ooc" ? speakOutOfCharacter(line.text, as) : addNarration(line.text, as),
      clears: ["say-text"],
    };
  });
  sayText.addEventListener("keydown", (e) => {
    if (e.key === "Enter") say();
  });
  wrap.appendChild(group("Say something", sayAs, sayText, button("Send", say, "add-narration")));

  const openSession = d.st.Sessions.find((s) => s.EndSeq === 0);

  // --- session ---
  const sessionName = input("session name", "session-name", "wide");
  wrap.appendChild(
    group(
      openSession ? `Session: ${openSession.Name}` : "Session",
      ...(openSession
        ? [button("End session", () => d.send(endSession()), "end-session")]
        : [
            sessionName,
            button("Start session", sending(d, "start-session", () => {
              const n = sessionName.value.trim();
              if (n === "") return void d.notify("a session needs a name");
              return { command: startSession(n), clears: ["session-name"] };
            }), "start-session"),
          ]),
    ),
  );

  // --- actor: form, and raw paste ---
  //
  // Offer no controller box: who holds an actor is the grant row below, which
  // asks for a kind (SPEC-013). Ask the kind here, blank by default: the server
  // refuses an add_actor that does not state one, and a pre-filled answer
  // hides a DM who never looked.
  const actorId = input("actor id", "actor-id");
  const actorName = input("name", "actor-name");
  const actorKind = choiceSelect("actor-kind", "actor-kind", KIND_CHOICES);
  wrap.appendChild(
    group(
      "Add actor",
      actorId, actorName, actorKind,
      button("Add", sending(d, "add-actor", () => {
        if (actorId.value.trim() === "") return void d.notify("an actor needs an id");
        const kind = actorKindFromWireName(actorKind.value);
        if (kind === null) {
          return void d.notify("Say what it is: a party member, or a monster the party has to find.");
        }
        return {
          command: addActor(actorId.value.trim(), actorName.value.trim(), kind),
          clears: ["actor-id", "actor-name", "actor-kind"],
        };
      }), "add-actor"),
      // Ask inside the press `sending` runs, after the empty check: a press that
      // `sending` holds back, or that the check stops, must ask the DM nothing.
      // VTT-950 VTT-947 VTT-948 VTT-951
      button("Remove", sending(d, "remove-actor", () => {
        const id = actorId.value.trim();
        if (id === "") return void d.notify("name the actor to remove");
        const shown = d.st.Actors[id]?.name || id;
        if (!d.confirm(`Remove ${shown}? Nothing brings it back, and its tokens leave the board.`)) return;
        return { command: removeActor(id), clears: ["actor-id"], rereads: true };
      }), "remove-actor"),
    ),
  );

  const paste = document.createElement("textarea");
  paste.placeholder =
    '{"actorId":"a1","name":"Lera","kind":"ACTOR_KIND_PARTY_MEMBER","attributes":{"brawn":3}}';
  paste.className = "paste";
  paste.dataset["field"] = "actor-json";
  paste.value = draft["actor-json"] ?? "";
  paste.addEventListener("input", () => {
    draft["actor-json"] = paste.value;
  });
  wrap.appendChild(
    group(
      "…or paste actor JSON",
      paste,
      button("Add from JSON", sending(d, "actor-json", () => {
        const result = parseActorJSON(paste.value);
        // Parse errors are shown, never thrown: the DM is mid-edit and
        // blanking the console would lose what they typed.
        if (result instanceof Error) return void d.notify(result.message);
        return { command: result, clears: ["actor-json"] };
      })),
    ),
  );

  // --- place token ---
  const tokId = input("token id", "token-id");
  const tokScene = input("scene id", "token-scene");
  const tokActor = input("actor id", "token-actor");
  const tx = input("x", "token-x");
  const ty = input("y", "token-y");
  tx.className = "tiny";
  ty.className = "tiny";
  wrap.appendChild(
    group(
      "Place token",
      tokId, tokScene, tokActor, tx, ty,
      button("Place", sending(d, "place-token", () => {
        if (!tokId.value.trim() || !tokScene.value.trim() || !tokActor.value.trim()) {
          return void d.notify("a token needs an id, a scene and an actor");
        }
        return {
          command: placeToken(tokId.value.trim(), tokScene.value.trim(), tokActor.value.trim(), {
            x: Number(tx.value) || 0,
            y: Number(ty.value) || 0,
          }),
          clears: ["token-id", "token-scene", "token-actor", "token-x", "token-y"],
        };
      }), "place-token"),
      // Ask inside the press `sending` runs, after the empty check: a press that
      // `sending` holds back, or that the check stops, must ask the DM nothing.
      // VTT-952 VTT-953 VTT-954 VTT-955
      button("Remove", sending(d, "remove-token", () => {
        const id = tokId.value.trim();
        if (id === "") return void d.notify("name the token to remove");
        const tok = d.st.Tokens[id];
        const shown = (tok && (tok.GMName || d.st.Actors[tok.ActorID]?.name || tok.Name)) || id;
        if (!d.confirm(`Remove ${shown} from the board? Its place, names and notes go with it.`)) return;
        return { command: removeToken(id), clears: ["token-id"] };
      }), "remove-token"),
    ),
  );

  // --- describe tokens ---
  const ids = Object.keys(d.st.Tokens).sort();
  if (ids.length > 0) {
    const g = el("div", "dmgroup");
    g.appendChild(el("h3", undefined, "Describe tokens"));
    for (const id of ids) {
      const row = el("div", "row describe-row");
      row.dataset["token"] = id;
      const tok = d.st.Tokens[id]!;
      const name = input("what players see", `describe-name:${id}`, "", tok.Name ?? "");
      const notes = textarea("notes players see", `describe-notes:${id}`, tok.Notes ?? "");
      const gmName = input("GM name", `describe-gm-name:${id}`, "", tok.GMName ?? "");
      const gmNotes = textarea("GM notes", `describe-gm-notes:${id}`, tok.GMNotes ?? "");
      const boxes = [name, notes, gmName, gmNotes];
      const describe = sending(d, `describe-token:${id}`, () => ({
        command: describeToken(id, name.value, notes.value, gmName.value, gmNotes.value),
        clears: boxes.map((b) => b.dataset["field"]!),
      }));
      row.append(el("span", "describe-id", id), ...boxes, button("Describe", describe, "describe-token"));
      g.appendChild(row);
    }
    wrap.appendChild(g);
  }

  // Keep the door toggle offered whatever the scene holds: dm-view.test.ts's
  // "the toggle's label says which state it is in, at render time".
  wrap.appendChild(
    group(
      "Doors",
      button(d.doorsArmed ? "Doors armed" : "Arm doors", d.toggleDoors, "arm-doors"),
    ),
  );

  // --- adventures ---
  if (d.adventures.length > 0) {
    const row: HTMLElement[] = [];
    for (const a of d.adventures) {
      row.push(button(`Load ${a.name || a.id}`, () => d.send(loadAdventure(a.id)), "load-adventure"));
      row.push(
        button("guide", () => {
          void d.guideFor(a.id).then((g) => d.notify(g ?? "no guide for that adventure"));
        }),
      );
    }
    wrap.appendChild(group("Adventures", ...row));
  }

  // --- maps ---
  if (d.maps.length > 0) {
    const row: HTMLElement[] = [];
    for (const m of d.maps) {
      row.push(button(`Load ${m.name || m.id}`, () => void d.send(loadMap(m.id)), "load-map"));
    }
    wrap.appendChild(group("Maps", ...row));
  }

  // --- notes ---
  const noteKey = input("key", "note-key");
  const noteTitle = input("title", "note-title");
  const noteText = input("text", "note-text", "wide");
  const noteVisibility = choiceSelect("note-visibility", "note-visibility", VISIBILITY_CHOICES);
  wrap.appendChild(
    group(
      "Notes",
      noteKey, noteTitle, noteText, noteVisibility,
      button("Save", sending(d, "upsert-note", () => {
        if (!noteKey.value.trim() || !noteText.value.trim()) {
          return void d.notify("a note needs a key and some text");
        }
        const visibility = noteVisibilityFromWireName(noteVisibility.value);
        if (visibility === null) {
          return void d.notify("Say who may read it: every player and spectator, or the DM alone.");
        }
        return {
          command: upsertNote(noteKey.value.trim(), noteTitle.value.trim(), noteText.value.trim(), visibility),
          clears: ["note-visibility"],
        };
      }), "upsert-note"),
      // Ask inside the press `sending` runs, after the empty check (SPEC-022).
      // VTT-956 VTT-957 VTT-958 VTT-959 VTT-960 VTT-961 VTT-962 VTT-963
      button("Delete", sending(d, "delete-note", () => {
        const key = noteKey.value.trim();
        if (key === "") return void d.notify("name the note to delete");
        const shown = d.st.Notes[key]?.Title || key;
        if (!d.confirm(`Delete the note "${shown}"? Its title and text go with it.`)) return;
        return { command: deleteNote(key), clears: [] };
      }), "delete-note"),
    ),
  );

  const names = namer(d.st, [], true, d.abilities, d.properties, d.conditions, d.catalogue, d.places);

  // --- catalogue: what each sheet refers to (VTT-1057, VTT-1058) ---
  const entries = d.catalogue ?? [];
  const places = d.places ?? [];
  const placeKeys = places.map((p) => p.key);
  const placeChoice = (key: string) => [key, names.place(key)] as const;
  const offersAddition = entries.length > 0 && places.length > 0;
  const referenceRows: HTMLElement[] = [];
  for (const actorId of Object.keys(d.st.Actors).sort()) {
    const a = d.st.Actors[actorId]!;
    const ids = Object.keys(a.references).sort();
    if (ids.length === 0 && !offersAddition) continue;
    const row = el("div", "reference-actor");
    row.dataset["actor"] = a.actorId;
    const held = a.name !== "" ? a.name : a.actorId;
    row.appendChild(el("span", "reference-name", held));
    for (const id of ids) {
      const r = a.references[id]!;
      const entry = names.entry(r.entryId);
      const place = names.place(r.place);
      const line = el("div", "reference-row");
      line.dataset["reference"] = id;
      line.appendChild(el("span", "reference-label", `${id}: ${entry} in ${place}${r.state === "" ? "" : ` (${r.state})`}`));
      const placeField = `change-reference-place:${a.actorId}:${id}`;
      const stateField = `change-reference-state:${a.actorId}:${id}`;
      const pick = choiceSelect("change-reference-place", placeField,
        (placeKeys.includes(r.place) ? placeKeys : [...placeKeys, r.place]).map(placeChoice), r.place);
      const state = input("state", stateField, "reference-state", r.state);
      line.append(pick, state, button("Change", sending(d, `change-reference:${a.actorId}:${id}`, () => ({
        command: changeReference(a.actorId, id, pick.value, state.value.trim()), clears: [placeField, stateField],
      })), "change-reference"));
      // Ask inside the press `sending` runs (SPEC-022). VTT-1058
      line.appendChild(button("Remove", sending(d, `remove-reference:${a.actorId}:${id}`, () => {
        if (!d.confirm(`Remove ${id}: ${entry} in ${place} from ${held}? Its id and state go with it.`)) return;
        return { command: removeReference(a.actorId, id), clears: [] };
      }), "remove-reference"));
      row.appendChild(line);
    }
    if (offersAddition) {
      const idField = `add-reference-id:${a.actorId}`;
      const entryField = `add-reference-entry:${a.actorId}`;
      const placeField = `add-reference-place:${a.actorId}`;
      const stateField = `add-reference-state:${a.actorId}`;
      const refId = input("reference id", idField, "reference-id");
      const entry = choiceSelect("add-reference-entry", entryField,
        [["", "choose an entry"], ...entries.map((e) => [e.id, names.entry(e.id)] as const)]);
      const place = choiceSelect("add-reference-place", placeField,
        [["", "choose a place"], ...placeKeys.map(placeChoice)]);
      const state = input("state", stateField, "reference-state");
      row.append(refId, entry, place, state, button("Add", sending(d, `add-reference:${a.actorId}`, () => {
        const id = refId.value.trim();
        if (id === "" || entry.value === "" || place.value === "") {
          return void d.notify("a reference needs an id, an entry and a place");
        }
        return {
          command: addReference(a.actorId, id, entry.value, place.value, state.value.trim()),
          clears: [idField, entryField, placeField, stateField],
        };
      }), "add-reference"));
    }
    referenceRows.push(row);
  }
  if (referenceRows.length > 0) wrap.appendChild(group("Catalogue", ...referenceRows));

  // --- conditions, straight off the actors that have them ---
  const condRow: HTMLElement[] = [];
  for (const [actor, list] of Object.entries(d.st.Conditions)) {
    const held = d.st.Actors[actor]?.name || actor;
    for (const c of list) {
      const condition = names.condition(c.ID);
      // Ask inside the press `sending` runs (SPEC-022).
      // VTT-964 VTT-965 VTT-966 VTT-967 VTT-968 VTT-969
      condRow.push(button(`${held}: ${condition} ✕`, sending(d, `remove-condition:${actor}:${c.ID}`, () => {
        if (!d.confirm(`Remove ${condition} from ${held}? Nothing in the console puts it back.`)) return;
        return { command: removeCondition(actor, c.ID), clears: [] };
      }), "remove-condition"));
    }
  }
  if (condRow.length > 0) wrap.appendChild(group("Remove condition", ...condRow));

  // --- abilities: what each sheet names, given and taken (VTT-730) ---
  const abilityRows: HTMLElement[] = [];
  const named = names.resource;
  for (const a of Object.values(d.st.Actors).sort((x, y) => (x.actorId < y.actorId ? -1 : 1))) {
    const row = el("div", "ability-actor");
    row.dataset["actor"] = a.actorId;
    row.appendChild(el("span", "ability-name", a.name !== "" ? a.name : a.actorId));
    for (const id of new Set(a.abilityIds)) {
      const label = d.abilities.find((ab) => ab.id === id)?.name || id;
      const take = button(`${label} ✕`, sending(d, `take-ability:${a.actorId}:${id}`, () => ({
        command: takeAbility(a.actorId, id), clears: [],
      })), "take-ability");
      take.dataset["ability"] = id;
      row.appendChild(take);
    }
    const offered = d.abilities.filter((ab) => !a.abilityIds.includes(ab.id));
    if (offered.length > 0) {
      const field = `give-ability-${a.actorId}`;
      const pick = choiceSelect("give-ability", field,
        [["", "choose an ability"], ...offered.map((ab) => [ab.id, ab.name || ab.id] as const)]);
      row.appendChild(pick);
      const current = `give-ability-current-${a.actorId}`;
      const max = `give-ability-max-${a.actorId}`;
      const lacking = (id: string) => {
        const usage = d.abilities.find((ab) => ab.id === id)?.usage;
        return usage?.kind === "resource" && usage.resource && a.resources[usage.resource] === undefined
          ? usage.resource : undefined;
      };
      const fields = [input("", current, "give-resource"), input("", max, "give-resource")] as const;
      // Show the fields on a choice without a repaint: a repaint per arrow key
      // takes the select's focus (VTT-788).
      const follow = () => {
        const asked = lacking(pick.value);
        fields.forEach((f, i) => {
          f.hidden = asked === undefined;
          f.placeholder = asked === undefined ? "" : `${named(asked)} ${i === 0 ? "now" : "max"}`;
        });
      };
      follow();
      pick.addEventListener("change", follow);
      row.append(...fields);
      row.appendChild(button("Give", () => {
        const chosen = pick.value;
        if (chosen === "") return void d.notify("Choose an ability to give first.");
        const spent = lacking(chosen);
        const resources: Record<string, { current: number; max: number }> = {};
        if (spent !== undefined) {
          const now = int32((draft[current] ?? "").trim());
          const top = (draft[max] ?? "").trim();
          const ceiling = top === "" ? 0 : int32(top);
          if (now === undefined) return void d.notify(`Give ${named(spent)} a current value that is ${WHOLE}.`);
          if (ceiling === undefined) {
            return void d.notify(`Give ${named(spent)} a max that is ${WHOLE}, or leave it empty for none.`);
          }
          resources[spent] = { current: now, max: ceiling };
        }
        sending(d, `give-ability:${a.actorId}:${chosen}`, () => ({
          command: giveAbility(a.actorId, chosen, resources), clears: [field, current, max],
        }))();
      }, "give-ability"));
    }
    abilityRows.push(row);
  }
  if (abilityRows.length > 0) wrap.appendChild(group("Abilities", ...abilityRows));

  // --- who controls what -------------------------------------------------
  //
  // The DM's half of spec §3.1: a campaign starts with nobody holding
  // anything, and characters are assigned afterwards. Without this the whole
  // control feature is reachable only by an agent over MCP.
  // WHO EXISTS, from BOTH sources, because neither alone is the answer
  // (Patrik's ruling 2026-09-06). Control is durable — it lives in the log as
  // controllerIds and outlives any connection — while presence is
  // connection-scoped and in-memory. Reading people from presence alone is
  // what made a held character render as a raw participant id and made it
  // impossible to hand one to somebody who had not dialled in, which is what
  // session prep IS.
  //
  // The ROSTER is authoritative for names and covers people who are away. But
  // it is a FETCH, so it is null before it lands and stale after somebody joins
  // by share link — and that person is in presence and not yet on the books.
  // Taking the union keeps both cases: the DM can grant to anyone on the
  // roster or anyone they can see. Keyed by participant id, so the two sources
  // agreeing about a person yields one entry, with the roster's name winning
  // because it is the authoritative one.
  const known = new Map<string, { name: string; present: boolean }>();
  for (const r of d.roster ?? []) known.set(r.participantId, { name: r.name, present: false });
  for (const p of d.participants) {
    const had = known.get(p.participantId);
    // `||`, not `??`: an EMPTY roster name must fall through to the presence
    // one. identity.CreateInvite does not refuse a name for being empty, so
    // `--name ""` is reachable, and nullish-coalescing would render a blank.
    known.set(p.participantId, { name: had?.name || p.displayName, present: true });
  }
  // Sorted by NAME, ties broken on id: the console is rebuilt on every event,
  // and a list that reshuffles as people come and go is unreadable and
  // untestable. Same rule session.participants already applies to presence.
  const knownSorted = [...known.entries()].sort(([ai, a], [bi, b]) =>
    a.name === b.name ? (ai < bi ? -1 : 1) : (a.name < b.name ? -1 : 1));

  const controlRows: HTMLElement[] = [];
  for (const a of Object.values(d.st.Actors).sort((x, y) => (x.actorId < y.actorId ? -1 : 1))) {
    const row = el("div", "control-actor");
    row.dataset["actor"] = a.actorId;
    row.appendChild(el("span", "control-name", a.name !== "" ? a.name : a.actorId));

    // Every current controller, named where we can and by id where we cannot.
    // Iterating controllerIds rather than the participant list is what keeps an
    // OFFLINE controller visible: presence is connection-scoped, control is not,
    // so someone can hold a character while away and the DM still needs to see
    // it — otherwise the character reads as unowned and gets handed out twice.
    for (const id of a.controllerIds) {
      // Named from `known`, so a holder who is away still has a NAME. Falling
      // back to the raw id is kept for the one case that earns it: somebody
      // holding a character who is in neither the roster nor presence, where an
      // id is all there is. Before this the fallback fired for anyone merely
      // offline, and a uuid where a name belongs reads as a broken console
      // rather than as a player who stepped away.
      const who = known.get(id);
      const held = el("span", "held");
      // The id is the last resort, and an empty name falls back to it too: a
      // blank where a holder's name belongs is worse than a uuid, because it
      // reads as nobody holding it rather than as somebody unnamed.
      held.appendChild(el("span", "held-who", who?.name || id));
      // PRESENCE KEEPS EXACTLY ONE JOB: here or away. It no longer decides who
      // can hold a character or what they are called.
      // `!who?.present`, so a holder in NEITHER source is marked away too.
      // They are the person most certainly not at the table, and the earlier
      // `who && !who.present` left them the only holder with no marker at all
      // — which reads as "here".
      if (!who?.present) held.appendChild(el("span", "away", "away"));
      const off = document.createElement("button");
      off.className = "chip revoke";
      off.textContent = "Revoke";
      off.dataset["action"] = "revoke-actor-control";
      // The id is captured per BUTTON, not read back from the row: a handler
      // that recomputed it would revoke controllerIds[0] every time, which
      // looks right and takes the wrong character from the wrong person.
      off.addEventListener("click", () => void d.send(revokeActorControl(a.actorId, id)).then(d.refreshSharing));
      held.appendChild(off);
      row.appendChild(held);
    }

    const target = document.createElement("select");
    target.className = "grant-target";
    const field = `grant-${a.actorId}`;
    const blank = document.createElement("option");
    blank.value = "";
    blank.textContent = "choose a participant";
    target.appendChild(blank);
    for (const [id, who] of knownSorted) {
      const o = document.createElement("option");
      o.value = id;
      o.textContent = who.name;
      target.appendChild(o);
    }
    // Remembered ACROSS RE-RENDERS, exactly like the text inputs above and for
    // the same reason: the console is rebuilt on every event, and at a live
    // table events arrive constantly. Without this the DM picks a name, a
    // token moves, and the Grant button then reports "choose a participant
    // first" — a failure that reads as the DM's mistake. Set AFTER the options
    // exist, or the value has nothing to match.
    target.value = draft[field] ?? "";
    target.addEventListener("change", () => {
      draft[field] = target.value;
    });
    row.appendChild(target);

    // WHAT the actor is, asked at the grant (visibility spec §5.1). This is
    // the second question, not a setting: kind describes a character's
    // standing RIGHT NOW, and standing changes — a charmed monster becomes a
    // player's to run and then becomes a monster again — so it is asked every
    // time control moves rather than stamped once.
    //
    // The blank option is FIRST and is what the field starts on. Pre-selecting
    // either value would be a default, and a default is indistinguishable from
    // a DM who never looked — which is the exact reason the server refuses an
    // unstated kind instead of guessing one. Pre-filling from the actor's
    // CURRENT kind was considered and rejected for the same reason: it reads
    // as an answer while being an assumption, and the one actor it would be
    // wrong about is the monster somebody is being handed.
    const kindField = `grant-kind-${a.actorId}`;
    const kindPick = choiceSelect("grant-kind", kindField, KIND_CHOICES);
    row.appendChild(kindPick);

    const give = document.createElement("button");
    give.className = "chip grant";
    give.textContent = "Grant";
    give.dataset["action"] = "grant-actor-control";
    give.addEventListener("click", sending(d, `grant-actor-control:${a.actorId}`, () => {
      // Refused HERE rather than sent and bounced: the server rejects an empty
      // participant (gateway authz, engine controlTarget), and a toast saying
      // what the DM did wrong beats one relaying a protocol error.
      if (target.value === "") {
        d.notify("Grant to nobody: choose a participant first.");
        return;
      }
      // The same reasoning one field over. The server refuses a grant that
      // states no kind (gateway validateGrantActorControl), so without this
      // the DM would read a wire-level refusal for a question the console
      // simply failed to ask.
      const kind = actorKindFromWireName(kindPick.value);
      if (kind === null) {
        d.notify("Say what it is: a party member, or a monster the party has to find.");
        return;
      }
      return { command: grantActorControl(a.actorId, target.value, kind), clears: [field, kindField], rereads: true };
    }));
    row.appendChild(give);
    controlRows.push(row);
  }
  if (controlRows.length > 0) wrap.appendChild(group("Who controls what", ...controlRows));

  // --- who may do what ---
  //
  // Beside "Who controls what" on purpose: a grant there is what makes the
  // player here (VTT-890). Render it whenever the roster was READ, even empty,
  // or a failed fetch looks like an absent feature; build the rows inside the
  // branch, where something can observe them.
  const roster = d.roster;
  if (roster !== null) {
    const rosterRows: HTMLElement[] = [];
    for (const person of roster) {
      const row = el("div", "roster-row");
      row.dataset["participant"] = person.participantId;
      row.appendChild(el("span", "who", person.name));
      row.appendChild(el("span", "role", person.role));

      // Offer no "Make player": a grant makes a player (VTT-905).
      if (person.role === "player") {
        row.appendChild(button("Make spectator", () => {
          const held = Object.values(d.st.Actors)
            .filter((a) => a.controllerIds.includes(person.participantId))
            .map((a) => a.name || a.actorId)
            .sort();
          if (!d.confirm(`Make ${person.name} a spectator? They no longer control ${held.join(" and ")}.`)) {
            return;
          }
          void d.send(promoteParticipant(person.participantId)).then(d.refreshSharing);
        }, `demote-${person.participantId}`));
      }
      if (person.role === "spectator" || person.role === "player") {
        row.appendChild(button("Boot", () => {
          if (!d.confirm(`Boot ${person.name}? Their sign-in stops working for good, and ` +
            "they no longer control any character. They can come back only through the " +
            "share link while it is open.")) {
            return;
          }
          void d.send(removeParticipant(person.participantId)).then(d.refreshSharing);
        }, `boot-${person.participantId}`));
      }
      rosterRows.push(row);
    }
    wrap.appendChild(group("Who may do what", ...rosterRows));
  }

  // --- sharing this table ---
  if (d.joinLink !== null) {
    const link = d.joinLink;
    const nodes: HTMLElement[] = [];

    // The state word first, and rendered even when it is "closed": a sharing
    // panel that only spoke up when open would make "is this live?" a question
    // the DM answers by trying it.
    nodes.push(el("span", "door-state", link.open ? "door: open" : "door: closed"));

    // The whole URL, not the bare secret. A secret alone is not shareable —
    // the DM would have to know to wrap it, and the wrapping they invent is
    // where the ?join= spelling goes wrong. This string and app.ts's reader
    // are the two halves of one format.
    const url = document.createElement("input");
    url.className = "wide";
    url.dataset["field"] = "join-link";
    url.readOnly = true;
    url.value = `${d.origin}/?join=${link.secret}`;
    nodes.push(url);

    nodes.push(
      link.open
        ? button("Close the door", () => void d.send(setJoinDoor(false)).then(d.refreshSharing),
            "close-door")
        : button("Open the door", () => void d.send(setJoinDoor(true)).then(d.refreshSharing),
            "open-door"),
    );

    nodes.push(
      button("New link", () => {
        // ASKED FIRST, unlike opening. Rotating silently stops the link
        // everybody was already sent from working, and there is no undo — the
        // old secret is gone.
        if (!d.confirm("Replace the link? Anyone you already sent the old one to will not get in.")) {
          return;
        }
        void d.send(rotateJoinLink()).then(d.refreshSharing);
      }, "rotate-link"),
    );

    wrap.appendChild(group("Sharing this table", ...nodes));
  }

  return wrap;
}
