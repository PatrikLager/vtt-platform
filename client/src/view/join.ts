// The join form: the only screen a person sees before they have a credential.
//
// Written without the shared el() helper on purpose. That helper's optional
// `text` argument carries a documented mutation the harness cannot observe
// (happy-dom drops an undefined textContent, a real browser renders the word
// "undefined"), and there is no reason to import that problem into a view this
// small — every node here has text or does not, decided at the point it is
// built.

export interface JoinViewState {
  /** What the person has typed so far. Survives a repaint; see below. */
  name: string;
  /** A join is in flight. Every press mints a participant, so this matters. */
  busy: boolean;
  /** Empty when nothing is wrong — an empty error box reads as a warning. */
  error: string;
}

export interface JoinViewHandlers {
  onName(value: string): void;
  onSubmit(): void;
}

export function renderJoinView(
  root: HTMLElement,
  state: JoinViewState,
  on: JoinViewHandlers,
): void {
  const wrap = document.createElement("section");
  wrap.className = "join";

  const heading = document.createElement("h1");
  heading.textContent = "Join the table";
  wrap.appendChild(heading);

  const label = document.createElement("label");
  label.htmlFor = "join-name";
  label.textContent = "What should everyone call you?";
  wrap.appendChild(label);

  const input = document.createElement("input");
  input.id = "join-name";
  // No `type` is set: an <input> with no type IS a text input, so assigning
  // "text" states nothing the element does not already say, and an assignment
  // whose removal changes nothing is code no test can justify.
  input.dataset["field"] = "join-name";
  // Re-seeded from state on EVERY paint. The repaint that carries an error
  // also rebuilds this element, and losing the value there costs the joiner
  // their typing exactly when they are most likely to try again.
  input.value = state.name;
  input.addEventListener("input", () => on.onName(input.value));
  input.addEventListener("keydown", (ev) => {
    // Nobody reaches for the mouse after typing their own name.
    if (ev.key === "Enter") on.onSubmit();
  });
  wrap.appendChild(input);

  const button = document.createElement("button");
  button.dataset["action"] = "join";
  button.textContent = state.busy ? "Joining…" : "Join";
  // DISABLED IS THE GUARD, and it is the only one. A second press during a
  // slow round trip mints a SECOND participant: the same person at the table
  // twice, holding two credentials, one of which nothing will ever revoke
  // because nobody knows it exists. A guard in the handler as well would be a
  // branch no test could reach, since the browser does not deliver clicks to
  // a disabled button.
  button.disabled = state.busy;
  button.addEventListener("click", on.onSubmit);
  wrap.appendChild(button);

  if (state.error !== "") {
    const err = document.createElement("p");
    err.className = "error";
    err.textContent = state.error;
    wrap.appendChild(err);
  }

  root.replaceChildren(wrap);
}
