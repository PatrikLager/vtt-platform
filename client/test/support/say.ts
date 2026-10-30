// The DM console's Say keeps what was typed in its draft, which lives for the
// whole bun process: a name left in say-as shows in every later file's console.

import type { ClientCommand, CommandResult } from "../../../contract/gen/ts/vtt/v1/commands_pb";
import { renderDMConsole } from "../../src/view/dm";
import { newState, type State } from "../../src/state";
import { ok } from "./answers";

/** The DM console over `st`, with only its send and its repaint wired. */
export function sayConsole(
  st: State,
  send: (c: ClientCommand) => Promise<CommandResult>,
  rerender: () => void,
): HTMLElement {
  return renderDMConsole({
    st, participants: [], adventures: [], maps: [],
    guideFor: async () => null, joinLink: null, roster: null, origin: "",
    refreshSharing: () => {}, send, rerender,
    notify: () => {}, confirm: () => false, doorsArmed: false, toggleDoors: () => {},
  });
}

/** Type "" into each Say box the DM console draws, so the next test finds them empty. */
export function forgetTheSay(): void {
  const node = sayConsole(newState(), async () => ok(), () => {});
  const boxes = node.querySelectorAll<HTMLInputElement>('[data-field="say-as"], [data-field="say-text"]');
  for (const box of Array.from(boxes)) {
    box.value = "";
    box.dispatchEvent(new Event("input"));
  }
}
