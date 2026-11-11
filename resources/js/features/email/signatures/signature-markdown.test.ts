import { Editor } from "@tiptap/react";
import { afterEach, describe, expect, it } from "vitest";

import { signatureExtensions, signatureMarkdown } from "./signature-markdown";
import { findSignatureVariables, toWriter } from "./signature-variables";

const editors: Editor[] = [];

function open(body: string): Editor {
  const editor = new Editor({ extensions: signatureExtensions(), content: toWriter(body), contentType: "markdown" });
  editors.push(editor);

  return editor;
}

describe("signature markdown", () => {
  afterEach(() => {
    for (const editor of editors.splice(0)) {
      editor.destroy();
    }
  });

  it.each([
    ["single line", "Thanks"],
    ["lines", "Thanks,\n**{{member_name}}**\n{{member_title}}"],
    ["bold text around a detail", "**{{member_name}}, founder**"],
    ["linked detail", "[*{{company_name}}*](https://example.com)"],
    ["paragraphs", "Thanks,\n{{member_name}}\n\nThe {{address_name}} team at {{company_name}}"],
    ["italic", "*Sent with care*"],
    ["link", "[Our site](https://example.com)\n{{company_name}}"],
    ["email link", "[Write to us](mailto:help@example.com)"],
    ["ampersand", "Smith & Co <support>"],
    ["word with underscores", "[john_doe@example.com](mailto:john_doe@example.com)"],
    ["phone link", "[+1 555 0100](tel:+15550100)"],
  ])("keeps a %s as stored", (_, body) => {
    expect(signatureMarkdown(open(body))).toBe(body);
  });

  it("shows details as chips, not placeholders", () => {
    const editor = open("Thanks,\n{{member_name}}");

    expect(editor.getJSON().content?.[0]?.content?.some((node) => node.type === "signatureVariable")).toBe(true);
  });

  it("writes an inserted detail as the server reads it", () => {
    const editor = open("Thanks,");
    editor.chain().focus("end").insertSignatureVariable("company_name").run();

    expect(signatureMarkdown(editor)).toBe("Thanks,{{company_name}}");
  });

  it("keeps an unknown placeholder as text for the server to reject", () => {
    expect(signatureMarkdown(open("Cheers {{nickname}}"))).toBe("Cheers {{nickname}}");
  });

  it("writes a hard break as a lone line break", () => {
    const editor = open("Thanks,");
    editor.chain().focus("end").setHardBreak().insertContent("Ada").run();

    expect(signatureMarkdown(editor)).toBe("Thanks,\nAda");
  });

  it("finds details by name or key", () => {
    expect(findSignatureVariables("job").map((v) => v.key)).toEqual(["member_title"]);
    expect(findSignatureVariables("company").map((v) => v.key)).toEqual(["company_name"]);
    expect(findSignatureVariables("")).toHaveLength(4);
  });
});
