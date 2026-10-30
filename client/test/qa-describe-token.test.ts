import "./support/dom";
import { afterEach, describe, expect, test } from "bun:test";
import { toJson } from "@bufbuild/protobuf";
import { ClientCommandSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { describeToken } from "../src/commands";
import { newState, type State } from "../src/state";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import { ok } from "./support/answers";

// "Plan" below is
// docs/superpowers/plans/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md.
type Cmd = ReturnType<typeof describeToken>;
type Field = HTMLInputElement | HTMLTextAreaElement;

function stateWith(ids: string[]): State {
  const st = newState();
  for (const [i, id] of ids.entries()) {
    st.Tokens[id] = { ID: id, SceneID: "qadt-scene", ActorID: "qadt-actor", X: i, Y: i };
  }
  return st;
}

function deps(st: State, sent: Cmd[]): DMDeps {
  return {
    st,
    participants: [],
    adventures: [],
    maps: [],
    guideFor: async () => null,
    joinLink: null,
    roster: null,
    origin: "http://qa.test",
    refreshSharing: () => {},
    send: async (c) => {
      sent.push(c);
      return ok();
    },
    rerender: () => {},
    notify: () => {},
    confirm: () => true,
    doorsArmed: false,
    toggleDoors: () => {},
  };
}

function draw(d: DMDeps): HTMLElement {
  const el = renderDMConsole(d);
  document.body.replaceChildren(el);
  return el;
}

function field(root: ParentNode, key: string): Field | null {
  return root.querySelector<Field>(`[data-field="${key}"], [name="${key}"], [id="${key}"]`);
}

function must(root: ParentNode, key: string): Field {
  const f = field(root, key);
  if (!f) throw new Error(`no field ${key}`);
  return f;
}

function rowOf(root: ParentNode, id: string): HTMLElement {
  let el: HTMLElement | null = must(root, `describe-name:${id}`);
  while (el && !el.querySelector('[data-action="describe-token"]')) el = el.parentElement;
  if (!el) throw new Error(`no row with a describe button for ${id}`);
  return el;
}

function typeInto(f: Field, value: string): void {
  f.value = value;
  f.dispatchEvent(new Event("input", { bubbles: true }));
}

function typeRow(root: ParentNode, id: string, v: [string, string, string, string]): void {
  typeInto(must(root, `describe-name:${id}`), v[0]);
  typeInto(must(root, `describe-notes:${id}`), v[1]);
  typeInto(must(root, `describe-gm-name:${id}`), v[2]);
  typeInto(must(root, `describe-gm-notes:${id}`), v[3]);
}

function rowValues(root: ParentNode, id: string): string[] {
  return ["describe-name", "describe-notes", "describe-gm-name", "describe-gm-notes"].map(
    (k) => must(root, `${k}:${id}`).value,
  );
}

async function clickDescribe(root: ParentNode, id: string): Promise<void> {
  const btn = rowOf(root, id).querySelector<HTMLButtonElement>('[data-action="describe-token"]');
  if (!btn) throw new Error(`no describe button for ${id}`);
  btn.click();
  await new Promise((r) => setTimeout(r, 0));
  await new Promise((r) => setTimeout(r, 0));
}

function described(c: Cmd | undefined): { tokenId: string; name: string; notes: string; gmName: string; gmNotes: string } {
  if (!c || c.command.case !== "describeToken") throw new Error(`not a describe_token: ${c?.command.case}`);
  const v = c.command.value;
  return { tokenId: v.tokenId, name: v.name, notes: v.notes, gmName: v.gmName, gmNotes: v.gmNotes };
}

afterEach(() => {
  document.body.replaceChildren();
});

describe("describe_token builder", () => {
  // Plan D16: "The button sends `describeToken(id, name, notes, gmName, gmNotes)` as
  // typed"
  test("qaDT: describeToken builds describe_token from its five arguments in order", () => {
    const c = describeToken("qadt-t", "a hooded figure", "it hums", "Mirela", "a spy");
    expect(described(c)).toEqual({
      tokenId: "qadt-t",
      name: "a hooded figure",
      notes: "it hums",
      gmName: "Mirela",
      gmNotes: "a spy",
    });
  });

  // Plan D2: "An empty string in `DescribeToken` clears that field."
  test("qaDT: describeToken keeps an empty field empty", () => {
    expect(described(describeToken("qadt-t", "", "kept", "", ""))).toEqual({
      tokenId: "qadt-t",
      name: "",
      notes: "kept",
      gmName: "",
      gmNotes: "",
    });
  });

  // SPEC-007 "The event envelope has no `type` field": "The `oneof payload` inlines
  // one key per variant"
  test("qaDT: describeToken goes on the wire under the describeToken key", () => {
    const json = toJson(ClientCommandSchema, describeToken("qadt-t", "n", "no", "g", "gn")) as Record<string, unknown>;
    expect(json["describeToken"]).toEqual({ tokenId: "qadt-t", name: "n", notes: "no", gmName: "g", gmNotes: "gn" });
  });
});

describe("the DM console's describe control", () => {
  // VTT-464
  test("qaDT: one describe control per token the world holds, in id order", () => {
    const ids = ["qadt-c3", "qadt-a1", "qadt-b2"];
    const root = draw(deps(stateWith(ids), []));
    expect(root.querySelectorAll('[data-action="describe-token"]').length).toBe(3);
    const order = Array.from(root.querySelectorAll<Field>('[data-field^="describe-name:"], [name^="describe-name:"], [id^="describe-name:"]')).map(
      (f) => f.getAttribute("data-field") ?? f.getAttribute("name") ?? f.id,
    );
    expect(order).toEqual(["describe-name:qadt-a1", "describe-name:qadt-b2", "describe-name:qadt-c3"]);
    expect(root.textContent ?? "").toContain("Describe tokens");
    for (const id of ids) {
      const row = rowOf(root, id);
      expect(row.querySelectorAll('[data-action="describe-token"]').length).toBe(1);
      expect(row.querySelector('[data-action="describe-token"]')?.textContent?.trim()).toBe("Describe");
      expect(row.textContent ?? "").toContain(id);
      for (const k of ["describe-name", "describe-notes", "describe-gm-name", "describe-gm-notes"]) {
        expect(field(row, `${k}:${id}`)).not.toBeNull();
      }
    }
  });

  // Plan D16: "inputs `describe-name:<id>` and `describe-gm-name:<id>`; textareas
  // `describe-notes:<id>` and `describe-gm-notes:<id>`".
  // VTT-464
  test("qaDT: names are inputs and notes are textareas", () => {
    const root = draw(deps(stateWith(["qadt-kind"]), []));
    expect(must(root, "describe-name:qadt-kind").tagName).toBe("INPUT");
    expect(must(root, "describe-gm-name:qadt-kind").tagName).toBe("INPUT");
    expect(must(root, "describe-notes:qadt-kind").tagName).toBe("TEXTAREA");
    expect(must(root, "describe-gm-notes:qadt-kind").tagName).toBe("TEXTAREA");
  });

  // VTT-464
  test("qaDT: the Describe button sends describe_token for its row's token with the four fields as typed", async () => {
    const sent: Cmd[] = [];
    const root = draw(deps(stateWith(["qadt-send"]), sent));
    typeRow(root, "qadt-send", ["a sleeping creature", "curled by the fire", "Grukk", "wakes at any noise"]);
    await clickDescribe(root, "qadt-send");
    expect(sent.length).toBe(1);
    expect(described(sent[0])).toEqual({
      tokenId: "qadt-send",
      name: "a sleeping creature",
      notes: "curled by the fire",
      gmName: "Grukk",
      gmNotes: "wakes at any noise",
    });
  });

  // Plan D2: "An empty string in `DescribeToken` clears that field".
  // VTT-464
  test("qaDT: an emptied field, and one never typed in, are sent empty", async () => {
    const sent: Cmd[] = [];
    const root = draw(deps(stateWith(["qadt-empty"]), sent));
    typeInto(must(root, "describe-name:qadt-empty"), "first try");
    typeInto(must(root, "describe-name:qadt-empty"), "");
    typeInto(must(root, "describe-notes:qadt-empty"), "kept notes");
    typeInto(must(root, "describe-gm-name:qadt-empty"), "Grukk");
    typeInto(must(root, "describe-gm-name:qadt-empty"), "");
    await clickDescribe(root, "qadt-empty");
    expect(sent.length).toBe(1);
    expect(described(sent[0])).toEqual({
      tokenId: "qadt-empty",
      name: "",
      notes: "kept notes",
      gmName: "",
      gmNotes: "",
    });
  });

  // VTT-464
  test("qaDT: a row's button sends that row's token and fields, never another row's", async () => {
    const sent: Cmd[] = [];
    const root = draw(deps(stateWith(["qadt-r1", "qadt-r2"]), sent));
    typeRow(root, "qadt-r1", ["one", "one notes", "One GM", "one gm notes"]);
    typeRow(root, "qadt-r2", ["two", "two notes", "Two GM", "two gm notes"]);
    await clickDescribe(root, "qadt-r2");
    expect(sent.length).toBe(1);
    expect(described(sent[0])).toEqual({
      tokenId: "qadt-r2",
      name: "two",
      notes: "two notes",
      gmName: "Two GM",
      gmNotes: "two gm notes",
    });
  });

  // Plan D16: "Each field shows its draft."
  test("qaDT: a typed field shows its draft when the console is drawn again", () => {
    const d = deps(stateWith(["qadt-draft"]), []);
    typeRow(draw(d), "qadt-draft", ["n", "no", "g", "gn"]);
    expect(rowValues(draw(d), "qadt-draft")).toEqual(["n", "no", "g", "gn"]);
  });

  // Plan D16: "The button sends `describeToken(id, name, notes, gmName, gmNotes)` as
  // typed, and clears that row's four drafts."
  test("qaDT: sending clears that row's four drafts and leaves another row's", async () => {
    const d = deps(stateWith(["qadt-keep", "qadt-clear"]), []);
    const root = draw(d);
    typeRow(root, "qadt-keep", ["k", "kn", "kg", "kgn"]);
    typeRow(root, "qadt-clear", ["c", "cn", "cg", "cgn"]);
    await clickDescribe(root, "qadt-clear");
    const again = draw(d);
    expect(rowValues(again, "qadt-clear")).toEqual(["", "", "", ""]);
    expect(rowValues(again, "qadt-keep")).toEqual(["k", "kn", "kg", "kgn"]);
  });

  // VTT-464
  test("qaDT: a table with no tokens draws no describe control", () => {
    const root = draw(deps(newState(), []));
    expect(root.querySelectorAll('[data-action="describe-token"]').length).toBe(0);
    expect(root.querySelectorAll('[data-field^="describe-"], [name^="describe-"], [id^="describe-"]').length).toBe(0);
  });
});
