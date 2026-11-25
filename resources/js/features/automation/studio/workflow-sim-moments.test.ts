import { describe, expect, it } from "vitest";

import type { ChatLine, DryTrace } from "@/lib/api/workflow-simulations";

import { eventText, moments, type Names } from "./workflow-sim-moments";

const names: Names = {
  tag: (id) => ({ tag_pay: "payments" })[id] ?? "a tag",
  team: (id) => ({ team_bill: "Billing" })[id] ?? "a team",
  member: () => "Maya",
};

describe("simulation events", () => {
  it("words each action the way the inbox words the change", () => {
    const at = { step_id: "s", summary: "" };
    expect(eventText({ ...at, kind: "tag", detail: { tag_ids: ["tag_pay"] } }, names)).toBe(
      "Workflow tagged the conversation payments",
    );
    expect(eventText({ ...at, kind: "priority", detail: { priority: "high" } }, names)).toBe(
      "Workflow set the priority to high",
    );
    expect(eventText({ ...at, kind: "assign", detail: { team_id: "team_bill" } }, names)).toBe(
      "Workflow assigned the conversation to Billing",
    );
    expect(eventText({ ...at, kind: "http_request", summary: "Would send a request", mocked: true }, names)).toBe(
      "Workflow: send a request (mocked)",
    );
  });

  it("puts each action in the chat where its step ran", () => {
    const lines: ChatLine[] = [
      { from: "customer", text: "I was charged twice" },
      { from: "workflow", text: "What do you need?", step_id: "s_ask" },
      { from: "customer", text: "Refund", step_id: "s_ask" },
      { from: "workflow", text: "Done, refunded.", step_id: "s_done" },
    ];
    const trace = {
      verdict: { started: true },
      vars: {},
      end: "completed",
      steps: [
        { step_id: "s_sort", kind: "classify", outcome: "matched" },
        { step_id: "s_ask", kind: "buttons", outcome: "button" },
        { step_id: "s_tag", kind: "tag", outcome: "would_run" },
        { step_id: "s_done", kind: "message", outcome: "sent" },
        { step_id: "s_assign", kind: "assign", outcome: "would_run" },
      ],
      actions: [
        { step_id: "s_tag", kind: "tag", summary: "" },
        { step_id: "s_assign", kind: "assign", summary: "" },
      ],
    } as unknown as DryTrace;

    const order = moments(lines, trace).map((m) => (m.kind === "line" ? m.line.text : m.action.step_id));
    expect(order).toEqual([
      "I was charged twice",
      "What do you need?",
      "Refund",
      "s_tag",
      "Done, refunded.",
      "s_assign",
    ]);
  });
});
