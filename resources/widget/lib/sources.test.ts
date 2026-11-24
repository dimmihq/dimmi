import { describe, expect, it } from "vitest";

import { Thread } from "../components/thread";
import type { Message } from "../content";
import { toSources } from "./map";

function render(markup: unknown): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = String(markup);
  return root;
}

const answer = (sources: Message["sources"]): Message => ({
  from: "agent",
  body: "Here is how.",
  at: "10:02",
  sources,
});

describe("cited articles", () => {
  it("keeps the excerpt and slug an answer's sources carry", () => {
    expect(
      toSources({
        sources: [
          { title: "Labels", url: "https://help.example/articles/labels", excerpt: "Print one.", slug: "labels" },
          { title: "Notes", url: null, excerpt: null, slug: null },
          { title: "", url: "https://help.example/x" },
        ],
      }),
    ).toEqual([
      { title: "Labels", url: "https://help.example/articles/labels", excerpt: "Print one.", slug: "labels" },
      { title: "Notes" },
    ]);
  });

  it("shows each source as a card under the bubble, not inside it", () => {
    const root = render(
      Thread({
        messages: [
          answer([
            { title: "Labels", url: "https://help.example/articles/labels", excerpt: "Print one.", slug: "labels" },
          ]),
        ],
      }),
    );

    expect(root.querySelector(".bubble .article")).toBeNull();
    const card = root.querySelector<HTMLAnchorElement>(".sources a.article");
    expect(card?.href).toBe("https://help.example/articles/labels");
    expect(card?.dataset.open).toBe("article:labels");
    expect(card?.dataset.action).toBe("click->cite");
    expect(card?.querySelector("strong")?.textContent).toBe("Labels");
    expect(card?.querySelector("small")?.textContent).toBe("Print one.");
  });

  it("folds the sources past the third under a more line", () => {
    const sources = ["A", "B", "C", "D", "E"].map((title) => ({ title, url: `https://help.example/${title}` }));
    const root = render(Thread({ messages: [answer(sources)] }));

    expect(root.querySelectorAll(".sources > .article")).toHaveLength(3);
    expect(root.querySelectorAll(".sources-more .article")).toHaveLength(2);
    expect(root.querySelector(".sources-more summary")?.textContent).toBe("2 more");
  });

  it("opens a workflow's article in place from the same card", () => {
    const root = render(
      Thread({
        messages: [
          {
            from: "team",
            body: "This should help.",
            at: "10:02",
            article: { slug: "labels", title: "Labels", summary: "Print one." },
          },
        ],
      }),
    );

    const card = root.querySelector<HTMLButtonElement>("button.article");
    expect(card?.dataset.open).toBe("article:labels");
    expect(card?.querySelector("small")?.textContent).toBe("Print one.");
  });
});
