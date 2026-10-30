// The DM console's Say keeps what was typed in its draft, which lives for the
// whole bun process: a name left in say-as shows in every later file's console.

import { renderDMConsole } from "../../src/view/dm";
import { newState } from "../../src/state";
import { ok } from "./answers";

/** Type "" into each Say box the DM console draws, so the next test finds them empty. */
export function forgetTheSay(): void {
  const node = renderDMConsole({
    st: newState(), participants: [], adventures: [], maps: [],
    guideFor: async () => null, joinLink: null, roster: null, origin: "",
    refreshSharing: () => {}, send: async () => ok(), rerender: () => {},
    notify: () => {}, confirm: () => false, doorsArmed: false, toggleDoors: () => {},
  });
  const boxes = node.querySelectorAll<HTMLInputElement>('[data-field="say-as"], [data-field="say-text"]');
  for (const box of Array.from(boxes)) {
    box.value = "";
    box.dispatchEvent(new Event("input"));
  }
}
