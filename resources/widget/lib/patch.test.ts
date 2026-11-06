import { describe, expect, it } from "vitest";

import { patch } from "./patch";

function thread(markup: string): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = markup;
  return root;
}

const said = (key: string, bubbles: string[]) =>
  `<div class="said" data-key="said:${key}"><div class="bubbles">${bubbles
    .map((b) => `<div class="bubble" data-key="${b}">${b}</div>`)
    .join("")}</div><time>now</time></div>`;

describe("patch", () => {
  it("keeps the messages already shown and marks only the new one to enter", () => {
    const root = thread(said("a", ["a"]));
    const first = root.querySelector('[data-key="a"]');

    patch(root, said("a", ["a", "b"]), true);

    expect(root.querySelector('[data-key="a"]')).toBe(first);
    expect(first?.classList.contains("enter")).toBe(false);
    expect(root.querySelector('[data-key="b"]')?.classList.contains("enter")).toBe(true);
  });

  it("keeps a new message entering when a second redraw follows at once", () => {
    const root = thread(said("a", ["a"]));

    patch(root, said("a", ["a", "b"]), true);
    patch(root, said("a", ["a", "b"]), true);

    expect(root.querySelector('[data-key="b"]')?.classList.contains("enter")).toBe(true);
  });

  it("updates a message's state in place", () => {
    const root = thread('<div class="bubble" data-key="c:1" data-state="sending">hi</div>');
    const bubble = root.firstElementChild;

    patch(root, '<div class="bubble" data-key="c:1">hi</div>', true);

    expect(root.firstElementChild).toBe(bubble);
    expect(bubble?.hasAttribute("data-state")).toBe(false);
  });

  it("animates nothing when told not to, as when another conversation opens", () => {
    const root = thread("");

    patch(root, said("a", ["a", "b"]), false);

    expect(root.querySelectorAll(".enter")).toHaveLength(0);
  });

  it("removes what the new markup no longer has", () => {
    const root = thread(`${said("a", ["a"])}<div class="said" data-key="typing"></div>`);

    patch(root, said("a", ["a"]), true);

    expect(root.querySelector('[data-key="typing"]')).toBeNull();
  });
});
