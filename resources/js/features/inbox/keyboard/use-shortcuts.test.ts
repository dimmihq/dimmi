import { describe, expect, it, vi } from "vitest";

import type { ActionContext, InboxAction } from "../actions/registry";
import { guarded } from "./use-shortcuts";

const ctx: ActionContext = { workspace: "ws", conversation: null, go: () => undefined, open: () => undefined };

function press(target: EventTarget) {
  const event = new KeyboardEvent("keydown", { key: "j", cancelable: true });
  Object.defineProperty(event, "target", { value: target });
  return event;
}

function action(run: () => void, extra: Partial<InboxAction> = {}): InboxAction {
  return { id: "a", label: "A", group: "Help", when: () => true, run, ...extra };
}

describe("guarded", () => {
  it("runs the move from the page and takes the key", () => {
    const run = vi.fn();
    const event = press(document.body);

    guarded(action(run), () => ctx)(event);

    expect(run).toHaveBeenCalledOnce();
    expect(event.defaultPrevented).toBe(true);
  });

  it("leaves a key typed in a field alone", () => {
    const run = vi.fn();
    const input = document.createElement("input");
    const event = press(input);

    guarded(action(run), () => ctx)(event);

    expect(run).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });

  it("runs a move allowed in fields while typing", () => {
    const run = vi.fn();

    guarded(action(run, { allowInInput: true }), () => ctx)(press(document.createElement("textarea")));

    expect(run).toHaveBeenCalledOnce();
  });

  it("does nothing when the move does not apply", () => {
    const run = vi.fn();

    guarded(action(run, { when: () => false }), () => ctx)(press(document.body));

    expect(run).not.toHaveBeenCalled();
  });
});
