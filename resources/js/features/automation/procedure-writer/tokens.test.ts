import type { ProcedureStep } from "@/lib/api/procedures";

import { chipsOf, type DocNode, docToText, syncBindings, textToDoc } from "./tokens";

const label = ({ kind, ref }: { kind: string; ref: string }) => `${kind}/${ref}`;

describe("procedure step tokens", () => {
  it("round-trips words, chips and line breaks", () => {
    const text = "Run [[action:aac_lookup]], then check\n[[attribute:contact.attr.plan]] first.";
    const doc = textToDoc(text, label);
    const inline = doc.content?.[0]?.content ?? [];

    expect(inline.map((n) => n.type)).toEqual([
      "text",
      "procedureToken",
      "text",
      "hardBreak",
      "procedureToken",
      "text",
    ]);
    expect(inline[1]?.attrs).toEqual({ kind: "action", ref: "aac_lookup", label: "action/aac_lookup" });
    expect(docToText(doc)).toBe(text);
  });

  it("reads chips in order and ignores what only looks like one", () => {
    expect(chipsOf("[[tag:tag_1]] and [[nope:x]] and [[article:art_2]]")).toEqual([
      { kind: "tag", ref: "tag_1" },
      { kind: "article", ref: "art_2" },
    ]);
  });

  it("keeps the settings of chips that stayed and drops removed ones", () => {
    const step: ProcedureStep = {
      id: "s_1",
      kind: "instruction",
      text: "",
      bindings: [
        { kind: "action", ref: "aac_a", execution: "conditional", on_missing: "continue" },
        { kind: "tag", ref: "tag_gone" },
      ],
    };
    const out = syncBindings(step, "Run [[action:aac_a]] and [[action:aac_b]], twice [[action:aac_a]]", label);

    expect(out).toEqual([
      { kind: "action", ref: "aac_a", execution: "conditional", on_missing: "continue" },
      { kind: "action", ref: "aac_b", label: "action/aac_b", execution: "required", on_missing: "handoff" },
    ]);
    expect(syncBindings({ ...step, kind: "condition" }, "[[action:aac_c]]", label)?.[0]?.execution).toBe("conditional");
    expect(syncBindings({ id: "x", kind: "instruction", text: "" }, "plain", label)).toBeUndefined();
  });

  it("writes an empty document as no words", () => {
    const empty: DocNode = { type: "doc", content: [{ type: "paragraph" }] };
    expect(docToText(empty)).toBe("");
  });
});
