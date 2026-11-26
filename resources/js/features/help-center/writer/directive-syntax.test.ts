import { describe, expect, it } from "vitest";

import { containerAt, fenceFor, formatDirective, leafAt, parseDirective } from "./directive-syntax";

describe("directive syntax", () => {
  it("reads a leaf with its label and attributes", () => {
    expect(parseDirective('::button[Go \\[now\\]]{href="/x" align=center}')).toEqual({
      fence: 2,
      name: "button",
      label: "Go [now]",
      attrs: { href: "/x", align: "center" },
    });
    expect(leafAt("::embed{src='https://youtu.be/dQw4w9WgXcQ'}\nNext", "embed")?.raw).toBe(
      "::embed{src='https://youtu.be/dQw4w9WgXcQ'}\n",
    );
  });

  it.each(["::embed[open", '::embed{src="x}', "::Embed", "::embed trailing", ":embed"])("refuses %s", (line) => {
    expect(leafAt(line, "embed")).toBeNull();
  });

  it("reads a container up to its closing fence, skipping code", () => {
    const found = containerAt(":::details[Q]\nA\n```\n:::\n```\n:::\nAfter", "details");
    expect(found?.directive.label).toBe("Q");
    expect(found?.inner).toBe("A\n```\n:::\n```\n");
    expect(found?.raw).toBe(":::details[Q]\nA\n```\n:::\n```\n:::\n");
  });

  it("lets an outer container hold an inner one", () => {
    const found = containerAt("::::details[Out]\n:::details[In]\nx\n:::\n::::", "details");
    expect(found?.inner).toBe(":::details[In]\nx\n:::\n");
  });

  it("writes lines the help center reads back", () => {
    const line = formatDirective(2, "button", "A [b]", { href: 'https://x.com/?q="1"', align: "" });
    expect(line).toBe('::button[A \\[b\\]]{href="https://x.com/?q=%221%22"}');
    expect(parseDirective(line)?.label).toBe("A [b]");
    expect(fenceFor("text")).toBe(3);
    expect(fenceFor(":::details[x]\n::::center\n")).toBe(5);
  });
});
