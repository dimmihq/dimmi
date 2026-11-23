import { Editor } from "@tiptap/react";
import { afterEach, describe, expect, it } from "vitest";

import { articleExtensions } from "./article-markdown";
import { ChangeRequest, changeRequestKey } from "./change-request";

const editors: Editor[] = [];

/*
 * review opens an article in the writer and sends it a request.
 */
function review(current: string, proposed: string, whole = false): HTMLElement {
  const host = document.createElement("div");
  document.body.appendChild(host);
  const editor = new Editor({
    element: host,
    extensions: articleExtensions([ChangeRequest]),
    content: current,
    contentType: "markdown",
  });
  editors.push(editor);
  const parsed = editor.markdown?.parse(proposed);
  const node = parsed ? editor.state.schema.nodeFromJSON(parsed) : null;
  editor.commands.command(({ tr }) => {
    tr.setMeta(changeRequestKey, { proposed: node, whole });
    return true;
  });
  return editor.view.dom;
}

afterEach(() => {
  for (const editor of editors.splice(0)) {
    editor.destroy();
  }
});

describe("change request", () => {
  it("marks only the reworded item of a list", () => {
    const dom = review("Intro\n\n- one\n- two\n- three", "Intro\n\n- one\n- TWO\n- three");

    expect([...dom.querySelectorAll(".change-removed")].map((n) => n.textContent)).toEqual(["two"]);
    const added = [...dom.querySelectorAll("[data-change=added]")];
    expect(added.map((n) => n.textContent)).toEqual(["TWO"]);
    expect(added[0].tagName).toBe("LI");
  });

  it("draws a requested line again when its wording changes past the first words", () => {
    const long = "Return labels are created on the carrier and service you choose, and shown at the counter.";
    const dom = review("Old line", long);
    const editor = editors[editors.length - 1];
    const parsed = editor.markdown?.parse(long.replace("the counter", "the post office counter"));
    const node = parsed ? editor.state.schema.nodeFromJSON(parsed) : null;
    editor.commands.command(({ tr }) => {
      tr.setMeta(changeRequestKey, { proposed: node, whole: false });
      return true;
    });

    const added = [...dom.querySelectorAll("[data-change=added]")].map((n) => n.textContent);
    expect(added).toEqual([long.replace("the counter", "the post office counter")]);
  });

  it("marks a reworded paragraph and adds a new one after it", () => {
    const dom = review("First\n\nSecond", "First\n\nSecond, reworded\n\nThird");

    expect([...dom.querySelectorAll(".change-removed")].map((n) => n.textContent)).toEqual(["Second"]);
    expect([...dom.querySelectorAll("[data-change=added]")].map((n) => n.textContent)).toEqual([
      "Second, reworded",
      "Third",
    ]);
  });

  it("puts a new paragraph after a list, not inside it", () => {
    const dom = review("- one\n- two", "- one\n- two\n\nAfter");
    const added = dom.querySelector("[data-change=added]");

    expect(added?.parentElement?.tagName).not.toBe("UL");
  });

  it("marks only the changed row of a table", () => {
    const dom = review(
      "| Code | Status |\n| --- | --- |\n| A | 401 |\n| B | 422 |",
      "| Code | Status |\n| --- | --- |\n| A | 401 |",
    );

    expect([...dom.querySelectorAll(".change-removed")].map((n) => n.tagName)).toEqual(["TR"]);
    expect(dom.querySelectorAll("[data-change=added]")).toHaveLength(0);
  });

  it("marks a new article whole", () => {
    const dom = review("One\n\nTwo", "One\n\nTwo", true);

    expect(dom.querySelectorAll(".change-added")).toHaveLength(2);
  });
});
