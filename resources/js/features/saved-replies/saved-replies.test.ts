import { describe, expect, it } from "vitest";

import { readReplies } from "./csv";
import { hasActions, macroChanges, macroSummary } from "./macro";
import { renderReply } from "./render";

describe("renderReply", () => {
  it("fills the customer's and sender's details", () => {
    const text = 'Hi {{ contact.first_name | default: "there" }}, {{ sender.first_name }} here.';

    expect(renderReply(text, { contact: { name: "Ada Okafor" }, sender: "Wren Lee" })).toBe("Hi Ada, Wren here.");
  });

  it("takes the fallback for what the conversation does not know", () => {
    const text = 'Hi {{ contact.first_name | default: "there" }} at {{ company.name | default: "your team" }}';

    expect(renderReply(text, { contact: { name: null, email: "a@b.co" } })).toBe("Hi there at your team");
  });
});

describe("macroChanges", () => {
  it("turns actions into the conversation's update", () => {
    expect(
      macroChanges({ status: "snoozed", snooze_preset: "next_week", tag_ids: ["tag_1"], priority: "high" }),
    ).toEqual({ status: "snoozed", snooze_preset: "next_week", add_tags: ["tag_1"], priority: "high" });
  });

  it("leaves the status to a send that closes or snoozes", () => {
    expect(macroChanges({ status: "closed", tag_ids: [] }, true)).toBeNull();
    expect(macroChanges({ status: "closed", assignee_id: "mem_1", tag_ids: [] }, true)).toEqual({
      assignee_id: "mem_1",
    });
  });

  it("knows a plain reply does nothing", () => {
    expect(hasActions({ tag_ids: [] })).toBe(false);
    expect(hasActions({ tag_ids: ["tag_1"] })).toBe(true);
  });

  it("names what a macro does", () => {
    const names = (id: string) => ({ mem_1: "Ada", tag_1: "Refund" })[id];

    expect(macroSummary({ status: "closed", assignee_id: "mem_1", tag_ids: ["tag_1"] }, names)).toEqual([
      "Close",
      "Assign to Ada",
      "Tag Refund",
    ]);
  });
});

describe("readReplies", () => {
  it("reads quoted fields with commas, quotes and line breaks", () => {
    const csv = 'title,body,shared\r\nRefund,"Hi, we ""refunded"" it.\nThanks",yes\nHello,Hi there,no\n';

    expect(readReplies(csv)).toEqual({
      error: null,
      replies: [
        { title: "Refund", body: 'Hi, we "refunded" it.\nThanks', shared: true, shortcut: undefined },
        { title: "Hello", body: "Hi there", shared: false, shortcut: undefined },
      ],
    });
  });

  it("refuses a file without title and body columns", () => {
    expect(readReplies("name,text\nA,B").error).toMatch(/title and a body/);
  });
});
