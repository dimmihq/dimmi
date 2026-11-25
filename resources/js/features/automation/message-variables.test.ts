import { describe, expect, it } from "vitest";

import { mentionedIn, readableText, variablesFor } from "./message-variables";

describe("message variables", () => {
  it("offers each trigger's own details, and the reply time only where it is filled in", () => {
    const keys = (list: { key: string }[]) => list.map((v) => v.key);
    expect(keys(variablesFor("conversation.first_message"))).toContain("contact.first_name");
    expect(keys(variablesFor("conversation.first_message"))).not.toContain("dimmi.summary");
    expect(keys(variablesFor("dimmi.handed_over"))).toContain("dimmi.summary");
    expect(keys(variablesFor("sla.breached"))).toContain("ticket.reference");
    expect(keys(variablesFor("conversation.first_message", true))[0]).toBe("reply_time");
  });

  it("shows variables by name and mentions as an at sign on a step's card", () => {
    expect(readableText('Hi {{ contact.first_name | default: "there" }}\\! We reply {{reply_time}}.')).toBe(
      "Hi {First name}! We reply {Reply time}.",
    );
    expect(readableText("<@mem_01abc> please look")).toBe("@teammate please look");
    expect(readableText("See [Reset a password](https://help.acme.test/articles/reset) for help.")).toBe(
      "See Reset a password for help.",
    );
    expect(readableText("Open **Settings → Billing**.")).toBe("Open Settings → Billing.");
  });

  it("finds the teammates a note mentions", () => {
    expect(mentionedIn("<@mem_01abc> and <@mem_02def>, then <@mem_01abc>")).toEqual(["mem_01abc", "mem_02def"]);
  });
});
