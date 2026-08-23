// The DM console (client spec §4): run the table.
//
// Every destructive or structural action lives here rather than in the player
// panel, and the one irreversible-looking action — undo — is gated by both a
// validity check and a confirmation, because a retraction that does nothing
// still writes a marker into the log implying something changed.
//
// Invite management is deliberately absent: it stays CLI (spec §4 non-goal).

import type { State } from "../state";
import type { Participant } from "../session";
import type { AdventureMeta, JoinLink, Roster } from "../metadata";
import { ActorKind, type Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand } from "../../../contract/gen/ts/vtt/v1/commands_pb";
import {
  startSession, endSession, createScene, placeToken, loadAdventure,
  upsertNote, deleteNote, removeCondition, retractEvents, parseActorJSON, addActor,
  grantActorControl, revokeActorControl,
  setJoinDoor,
  rotateJoinLink,
  promoteParticipant,
} from "../commands";
import { lastUndoable, retractableRange } from "../undo";

function el(tag: string, cls?: string, text?: string): HTMLElement {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  // Stryker disable next-line ConditionalExpression: no test can evaluate this
  // one. Removing the guard sets textContent to undefined, which a REAL
  // browser stringifies to the word "undefined" above every container — a
  // genuine defect — while happy-dom coerces it to "" and produces no text
  // node at all (measured, not assumed). So the mutant is observable in
  // production and invisible to the harness. Recorded here rather than in
  // ts-mutation-equivalents.txt, because it is NOT equivalent and claiming so
  // would be the exact wrong-adjudication that file warns about. The e2e in a
  // real browser is where this would surface.
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

/**
 * The wire NAME a kind <select> carries, back to the enum the command needs.
 *
 * Returns null for "nothing chosen", and that is the point rather than an
 * inconvenience: "the DM did not answer" is a THIRD state, distinct from both
 * kinds (visibility spec §5.1). Collapsing it into ACTOR_KIND_UNSPECIFIED and
 * sending it would reproduce on the client exactly the ambiguity the server
 * refuses — an omission indistinguishable from a decision. An unrecognised
 * string falls into the same bucket, which fails closed: a grant is not sent
 * at all rather than sent saying something nobody chose.
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
   * Everyone at the table and what they may do, read from identity rather
   * than presence: presence is connection-scoped and carries no role, so a
   * role taken from it would go stale the moment somebody was promoted
   * without reconnecting (spec §3.2).
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
   * Issue a command, resolving once the server has answered.
   *
   * The promise is load-bearing for the controls below that read state back:
   * a role change and a door change produce NO EVENT, so an HTTP re-read fired
   * beside the command races it on a different transport, and losing that race
   * repaints the panel with exactly the state the command was changing — with
   * nothing left to correct it.
   */
  send: (c: ClientCommand) => Promise<void>;
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
        ? [button("End session", () => d.send(endSession()), "end-session")]
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

  // --- who controls what -------------------------------------------------
  //
  // The DM's half of spec §3.1: a campaign starts with nobody holding
  // anything, and characters are assigned afterwards. Without this the whole
  // control feature is reachable only by an agent over MCP.
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
      const who = d.participants.find((p) => p.participantId === id);
      const held = el("span", "held");
      held.appendChild(el("span", "held-who", who ? who.displayName : id));
      const off = document.createElement("button");
      off.className = "chip revoke";
      off.textContent = "Revoke";
      // The id is captured per BUTTON, not read back from the row: a handler
      // that recomputed it would revoke controllerIds[0] every time, which
      // looks right and takes the wrong character from the wrong person.
      off.addEventListener("click", () => d.send(revokeActorControl(a.actorId, id)));
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
    for (const p of d.participants) {
      const o = document.createElement("option");
      o.value = p.participantId;
      o.textContent = p.displayName;
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
    const kindPick = document.createElement("select");
    kindPick.className = "grant-kind";
    const kindField = `grant-kind-${a.actorId}`;
    for (const [value, label] of [
      ["", "what is it?"],
      ["ACTOR_KIND_PARTY_MEMBER", "party member"],
      ["ACTOR_KIND_NON_PARTY", "monster / NPC"],
    ] as const) {
      const o = document.createElement("option");
      o.value = value;
      o.textContent = label;
      kindPick.appendChild(o);
    }
    kindPick.value = draft[kindField] ?? "";
    kindPick.addEventListener("change", () => {
      draft[kindField] = kindPick.value;
    });
    row.appendChild(kindPick);

    const give = document.createElement("button");
    give.className = "chip grant";
    give.textContent = "Grant";
    give.addEventListener("click", () => {
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
      d.send(grantActorControl(a.actorId, target.value, kind));
      clearDraft(field, kindField);
    });
    row.appendChild(give);
    controlRows.push(row);
  }
  if (controlRows.length > 0) wrap.appendChild(group("Who controls what", ...controlRows));

  // --- who may do what ---
  //
  // Beside "Who controls what" on purpose (spec §4): promoting somebody and
  // handing them a character are one thought, and two screens apart is how a
  // DM promotes a spectator and then wonders why they still cannot act.
  // Rendered whenever the roster was READ, even if it came back empty — which
  // it cannot, since it always holds the caller. A panel that disappeared on
  // an empty list would make a failed fetch look like a deliberately absent
  // feature. Built INSIDE the branch: outside it, a null roster still built
  // rows and discarded them, so nothing could observe what went into them.
  const roster = d.roster;
  if (roster !== null) {
    const rosterRows: HTMLElement[] = [];
    for (const person of roster) {
      const row = el("div", "roster-row");
      row.dataset["participant"] = person.participantId;
      row.appendChild(el("span", "who", person.name));
      row.appendChild(el("span", "role", person.role));

      // ONLY player and spectator are offered, because promote_participant may
      // only ever name those two (spec §3.1a): a control that reached dm or
      // agent would make the shared link a route to full authority in two steps,
      // and the server refuses it — so the button could only ever fail.
      if (person.role === "spectator") {
        row.appendChild(
          button("Make player", () => void d.send(
            promoteParticipant(person.participantId, "player"),
          ).then(d.refreshSharing), `promote-${person.participantId}`),
        );
      } else if (person.role === "player") {
        row.appendChild(
          button("Make spectator", () => void d.send(
            promoteParticipant(person.participantId, "spectator"),
          ).then(d.refreshSharing), `demote-${person.participantId}`),
        );
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
