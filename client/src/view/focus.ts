type Box = HTMLInputElement | HTMLTextAreaElement;

const BOXES = "input[data-field], textarea[data-field]";

const boxes = (root: HTMLElement): Box[] => Array.from(root.querySelectorAll<Box>(BOXES));

/** replaceKeepingFocus replaces root's children, keeping the box typed in (VTT-491). */
export function replaceKeepingFocus(root: HTMLElement, ...nodes: Node[]): void {
  // activeElement, never :focus: a window in the background matches no :focus.
  const held = boxes(root).find((b) => b === document.activeElement);
  root.replaceChildren(...nodes);
  if (held === undefined) return;
  // Compared, never built into a selector: a field can embed a token id.
  const next = boxes(root).find((b) => b.dataset["field"] === held.dataset["field"]);
  if (next === undefined) return;
  next.focus({ preventScroll: true });
  // setSelectionRange throws on a type with no selection, such as number.
  if (held.selectionStart === null) return;
  next.setSelectionRange(
    held.selectionStart,
    held.selectionEnd,
    held.selectionDirection as "forward" | "backward" | "none",
  );
}
