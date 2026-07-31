// The DM console (client spec §4): run the table.
//
// Every destructive or structural action lives here rather than in the player
// panel, and the one irreversible-looking action — undo — is gated by both a
// validity check and a confirmation, because a retraction that does nothing
// still writes a marker into the log implying something changed.
//
// Invite management is deliberately absent: it stays CLI (spec §4 non-goal).

import type { State } from "../state";
import type { AdventureMeta } from "../metadata";
import type { Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand } from "../../../contract/gen/ts/vtt/v1/commands_pb";
import {
  startSession, endSession, createScene, placeToken, loadAdventure,
  upsertNote, deleteNote, removeCondition, retractEvents, parseActorJSON, addActor,
} from "../commands";
import { lastUndoable, retractableRange } from "../undo";

function el(tag: string, cls?: string, text?: string): HTMLElement {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
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
//
// Found by the e2e (T9), not by any unit test — the bug only exists when a
// real event stream runs alongside a real form.
const draft: Record<string, string> = {};

/** Clear a field's remembered text, after the command it fed succeeded. */
function clearDraft(...fields: string[]): void {
  for (const f of fields) delete draft[f];
}

// Every input carries a stable data-field. Placeholders are prose — they get
// reworded, and they match by substring, so "name" also selects "session
// name". A test that pins a field by its wording breaks on a copy edit and
// silently targets the wrong box when two placeholders overlap.
function input(placeholder: string, field: string, cls = ""): HTMLInputElement {
  const i = document.createElement("input");
  i.placeholder = placeholder;
  i.dataset["field"] = field;
  i.value = draft[field] ?? "";
  i.addEventListener("input", () => {
    draft[field] = i.value;
  });
  if (cls) i.className = cls;
  return i;
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
  log: Envelope[];
  adventures: AdventureMeta[];
  guideFor: (id: string) => Promise<string | null>;
  send: (c: ClientCommand) => void;
  notify: (msg: string) => void;
  confirm: (msg: string) => boolean;
}

export function renderDMConsole(d: DMDeps): HTMLElement {
  const wrap = el("section", "dm");
  wrap.appendChild(el("h2", undefined, "DM console"));

  const openSession = d.st.Sessions.find((s) => s.EndSeq === 0);

  // --- session ---
  const sessionName = input("session name", "session-name", "wide");
  wrap.appendChild(
    group(
      openSession ? `Session: ${openSession.Name}` : "Session",
      ...(openSession
        ? [button("End session", () => d.send(endSession()))]
        : [
            sessionName,
            button("Start session", () => {
              const n = sessionName.value.trim();
              if (n === "") return d.notify("a session needs a name");
              d.send(startSession(n));
              clearDraft("session-name");
            }, "start-session"),
          ]),
    ),
  );

  // --- scene ---
  const sceneId = input("scene id", "scene-id");
  const sceneName = input("name", "scene-name");
  const w = input("w", "scene-w");
  const h = input("h", "scene-h");
  w.className = "tiny";
  h.className = "tiny";
  wrap.appendChild(
    group(
      "Create scene",
      sceneId, sceneName, w, h,
      button("Create", () => {
        const gw = Number(w.value) || 0;
        const gh = Number(h.value) || 0;
        if (sceneId.value.trim() === "" || gw <= 0 || gh <= 0) {
          return d.notify("scene needs an id and a positive width and height");
        }
        d.send(createScene(sceneId.value.trim(), sceneName.value.trim(), gw, gh));
      }),
    ),
  );

  // --- actor: form, and raw paste ---
  const actorId = input("actor id", "actor-id");
  const actorName = input("name", "actor-name");
  const controller = input("controller participant id (optional)", "actor-controller", "wide");
  wrap.appendChild(
    group(
      "Add actor",
      actorId, actorName, controller,
      button("Add", () => {
        if (actorId.value.trim() === "") return d.notify("an actor needs an id");
        d.send(addActor(actorId.value.trim(), actorName.value.trim(), controller.value.trim() || undefined));
        clearDraft("actor-id", "actor-name", "actor-controller");
      }, "add-actor"),
    ),
  );

  const paste = document.createElement("textarea");
  paste.placeholder = '{"actorId":"a1","name":"Lera","attributes":{"brawn":3}}';
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
      button("Add from JSON", () => {
        const result = parseActorJSON(paste.value);
        // Parse errors are shown, never thrown: the DM is mid-edit and
        // blanking the console would lose what they typed.
        if (result instanceof Error) return d.notify(result.message);
        d.send(result);
        clearDraft("actor-json");
      }),
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
      button("Place", () => {
        if (!tokId.value.trim() || !tokScene.value.trim() || !tokActor.value.trim()) {
          return d.notify("a token needs an id, a scene and an actor");
        }
        d.send(placeToken(tokId.value.trim(), tokScene.value.trim(), tokActor.value.trim(), {
          x: Number(tx.value) || 0,
          y: Number(ty.value) || 0,
        }));
        clearDraft("token-id", "token-scene", "token-actor", "token-x", "token-y");
      }, "place-token"),
    ),
  );

  // --- adventures ---
  if (d.adventures.length > 0) {
    const row: HTMLElement[] = [];
    for (const a of d.adventures) {
      row.push(button(`Load ${a.name || a.id}`, () => d.send(loadAdventure(a.id))));
      row.push(
        button("guide", () => {
          void d.guideFor(a.id).then((g) => d.notify(g ?? "no guide for that adventure"));
        }),
      );
    }
    wrap.appendChild(group("Adventures", ...row));
  }

  // --- notes ---
  const noteKey = input("key", "note-key");
  const noteTitle = input("title", "note-title");
  const noteText = input("text", "note-text", "wide");
  wrap.appendChild(
    group(
      "Notes",
      noteKey, noteTitle, noteText,
      button("Save", () => {
        if (!noteKey.value.trim() || !noteText.value.trim()) {
          return d.notify("a note needs a key and some text");
        }
        d.send(upsertNote(noteKey.value.trim(), noteTitle.value.trim(), noteText.value.trim()));
      }),
      button("Delete", () => {
        if (!noteKey.value.trim()) return d.notify("name the note to delete");
        d.send(deleteNote(noteKey.value.trim()));
      }),
    ),
  );

  // --- conditions, straight off the actors that have them ---
  const condRow: HTMLElement[] = [];
  for (const [actor, list] of Object.entries(d.st.Conditions)) {
    for (const c of list) {
      condRow.push(
        button(`${actor}: ${c.ID} ✕`, () => d.send(removeCondition(actor, c.ID))),
      );
    }
  }
  if (condRow.length > 0) wrap.appendChild(group("Remove condition", ...condRow));

  // --- undo ---
  const last = lastUndoable(d.log);
  const from = input("from", "undo-from");
  const to = input("to", "undo-to");
  from.className = "tiny";
  to.className = "tiny";
  const undoRow: HTMLElement[] = [];

  undoRow.push(
    button(last === null ? "Nothing to undo" : `Undo #${last}`, () => {
      if (last === null) return d.notify("nothing to undo");
      // Confirmation, because a retraction is visible to everyone at the
      // table the moment it lands.
      if (!d.confirm(`Retract event #${last}? Everyone at the table sees this.`)) return;
      d.send(retractEvents(last, last, "undo"));
    }),
  );
  undoRow.push(from, to);
  undoRow.push(
    button("Undo range", () => {
      const f = BigInt(Number(from.value) || 0);
      const t = BigInt(Number(to.value) || 0);
      // Validated BEFORE the dialog, so a pointless undo is never confirmed
      // and never writes a marker implying something changed.
      const why = retractableRange(d.log, f, t);
      if (why !== null) return d.notify(why);
      if (!d.confirm(`Retract events #${f}–#${t}? Everyone at the table sees this.`)) return;
      d.send(retractEvents(f, t, "undo"));
    }),
  );
  wrap.appendChild(group("Undo", ...undoRow));

  return wrap;
}
