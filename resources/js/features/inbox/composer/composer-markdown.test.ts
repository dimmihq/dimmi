import { Editor } from "@tiptap/react";
import { afterEach, describe, expect, it } from "vitest";

import { composerExtensions } from "./composer-markdown";

const editors: Editor[] = [];

function open(markdown: string): Editor {
  const editor = new Editor({ extensions: composerExtensions(), content: markdown, contentType: "markdown" });
  editors.push(editor);

  return editor;
}

describe("composer markdown", () => {
  afterEach(() => {
    for (const editor of editors.splice(0)) {
      editor.destroy();
    }
  });

  it.each([
    ["paragraph", "Hello there"],
    ["two paragraphs", "First\n\nSecond"],
    ["bold", "Please **reset** it"],
    ["italic", "Please *reset* it"],
    ["strike", "Please ~~reset~~ it"],
    ["inline code", "Run `dimmi migrate`"],
    ["code block", "```\nmake test\n```"],
    ["link", "See [the docs](https://example.com/docs)"],
    ["bullet list", "- one\n- two"],
    ["ordered list", "1. one\n2. two"],
    ["quote", "> You said this"],
    ["inline image", "Look ![chart](https://example.com/chart.png) here"],
  ])("keeps a %s as written", (_, markdown) => {
    expect(open(markdown).getMarkdown().trim()).toBe(markdown);
  });

  it("keeps a hard line break", () => {
    const out = open("Line one  \nLine two").getMarkdown();

    expect(out).toContain("Line one");
    expect(out).toContain("Line two");
    expect(out.split("\n").length).toBe(2);
  });

  it("drops what Markdown cannot carry", () => {
    const editor = open("");
    editor.commands.setContent("<h1>Title</h1><p><u>under</u> and <span style='color:red'>red</span></p><hr>", {
      contentType: "html",
    });

    expect(editor.getMarkdown().trim()).toBe("Title\n\nunder and red");
  });

  it("strikes through with Cmd+Shift+X and asks for a link with Cmd+Shift+U", () => {
    let asked = 0;
    const editor = new Editor({
      extensions: composerExtensions(() => {
        asked += 1;
      }),
      content: "word",
      contentType: "markdown",
    });
    editors.push(editor);
    editor.commands.selectAll();

    editor.view.someProp("handleKeyDown", (handle) =>
      handle(editor.view, new KeyboardEvent("keydown", { key: "X", keyCode: 88, shiftKey: true, ctrlKey: true })),
    );
    editor.view.someProp("handleKeyDown", (handle) =>
      handle(editor.view, new KeyboardEvent("keydown", { key: "U", keyCode: 85, shiftKey: true, ctrlKey: true })),
    );

    expect(editor.getMarkdown().trim()).toBe("~~word~~");
    expect(asked).toBe(1);
  });
});
