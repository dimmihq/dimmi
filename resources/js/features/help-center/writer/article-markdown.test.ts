import { Editor } from "@tiptap/react";
import { afterEach, describe, expect, it } from "vitest";

import { settleUpload } from "./article-image";
import { articleExtensions, articleMarkdown } from "./article-markdown";
import { findCommands, runCommand } from "./slash-commands";

const editors: Editor[] = [];

function open(markdown: string): Editor {
  const editor = new Editor({ extensions: articleExtensions(), content: markdown, contentType: "markdown" });
  editors.push(editor);

  return editor;
}

describe("article markdown", () => {
  afterEach(() => {
    for (const editor of editors.splice(0)) {
      editor.destroy();
    }
  });

  it.each([
    ["heading", "## Getting started"],
    ["small heading", "### Install"],
    ["paragraph", "Hello there"],
    ["bold and italic", "Please **reset** it *now*"],
    ["inline code", "Run `dimmi migrate`"],
    ["code block", "```\nmake test\n```"],
    ["link", "See [the docs](https://example.com/docs)"],
    ["bullet list", "- one\n- two"],
    ["ordered list", "1. one\n2. two"],
    ["quote", "> You said this"],
    ["rule", "Above\n\n---\n\nBelow"],
    ["image", "![chart](https://example.com/chart.png)"],
  ])("keeps a %s as written", (_, markdown) => {
    expect(open(markdown).getMarkdown().trim()).toBe(markdown);
  });

  it("keeps a table", () => {
    const markdown = "| Plan | Seats |\n| --- | --- |\n| Pro | 5 |";
    const out = open(markdown).getMarkdown();
    expect(out).toContain("Plan");
    expect(out).toMatch(/\|\s*Pro\s*\|\s*5\s*\|/);
    expect(out).toMatch(/\|\s*-+\s*\|/);
  });

  it.each([
    ["note", "> [!NOTE]\n> Read this first"],
    ["warning over two paragraphs", "> [!WARNING]\n> Back up first.\n>\n> Then reset."],
    ["tip with a list", "> [!TIP]\n> - one\n> - two"],
  ])("keeps a %s callout as a callout", (_, markdown) => {
    const editor = open(markdown);
    expect(editor.getJSON().content?.[0]?.type).toBe("callout");
    expect(articleMarkdown(editor.getMarkdown()).trim()).toBe(markdown);
  });

  it("keeps a plain quote a quote", () => {
    expect(open("> Just words").getJSON().content?.[0]?.type).toBe("blockquote");
  });

  it("keeps text around a callout", () => {
    const markdown = "Before\n\n> [!CAUTION]\n> Careful\n\nAfter";
    expect(articleMarkdown(open(markdown).getMarkdown()).trim()).toBe(markdown);
  });
});

describe("slash commands", () => {
  afterEach(() => {
    for (const editor of editors.splice(0)) {
      editor.destroy();
    }
  });

  it("finds commands by name, those starting with it first", () => {
    expect(findCommands("not").map((c) => c.id)).toEqual(["note"]);
    expect(findCommands("list").map((c) => c.id)).toEqual(["bullets", "numbers"]);
    expect(findCommands("callout").map((c) => c.id)).toEqual(["note", "tip", "important", "warning", "caution"]);
    expect(findCommands("pic").map((c) => c.id)).toEqual(["image"]);
    expect(findCommands("").length).toBe(15);
  });

  it("writes a callout the help center draws as an aside", () => {
    const editor = open("Intro\n\n/no");
    const note = findCommands("note")[0];
    if (!note) {
      throw new Error("no note command");
    }
    const end = editor.state.doc.content.size - 1;
    editor.commands.setTextSelection(end);
    runCommand(editor, { from: end - 3, to: end }, note, { pickImage: () => undefined });
    editor.commands.insertContent("Links expire after an hour.");
    expect(articleMarkdown(editor.getMarkdown()).trim()).toBe("Intro\n\n> [!NOTE]\n> Links expire after an hour.");
  });
});

describe("pictures", () => {
  it("leaves a picture still uploading out of the Markdown", () => {
    const editor = new Editor({ extensions: articleExtensions(), content: "Intro", contentType: "markdown" });
    editor.commands.insertContentAt(editor.state.doc.content.size, {
      type: "image",
      attrs: { src: "blob:local", alt: "", upload: "u1" },
    });
    expect(editor.getMarkdown()).not.toContain("blob:");
    settleUpload(editor, "u1", "/help-images/a.png");
    expect(editor.getMarkdown()).toContain("![](/help-images/a.png)");
    editor.destroy();
  });

  it("takes out a picture whose upload failed", () => {
    const editor = new Editor({ extensions: articleExtensions(), content: "Intro", contentType: "markdown" });
    editor.commands.insertContentAt(editor.state.doc.content.size, {
      type: "image",
      attrs: { src: "blob:local", alt: "", upload: "u2" },
    });
    settleUpload(editor, "u2", null);
    expect(editor.getJSON().content?.some((n) => n.type === "image")).toBe(false);
    editor.destroy();
  });

  it("keeps a picture's size and place in its address", () => {
    const editor = open("![Plans](/help-images/p.png#width=60&align=center)");
    const image = editor.getJSON().content?.find((n) => n.type === "image");
    expect(image?.attrs).toMatchObject({ src: "/help-images/p.png", width: 60, align: "center" });
    editor.commands.setNodeSelection(0);
    editor.commands.updateAttributes("image", { width: 45, align: "end" });
    expect(editor.getMarkdown().trim()).toBe("![Plans](/help-images/p.png#width=45&align=end)");
  });

  it("puts the old picture back when its replacement fails to upload", () => {
    const editor = open("![Plans](/help-images/p.png#width=60)");
    editor.commands.setNodeSelection(0);
    editor.commands.updateAttributes("image", { src: "blob:new", upload: "u3" });
    settleUpload(editor, "u3", null, "/help-images/p.png");
    expect(editor.getMarkdown().trim()).toBe("![Plans](/help-images/p.png#width=60)");
  });
});
