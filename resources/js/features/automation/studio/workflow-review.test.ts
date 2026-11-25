import { describe, expect, it } from "vitest";

import type { OperatorChangeset } from "@/lib/api/operator";
import type { Spec } from "@/lib/api/workflows";

import { requestFor, reviewOf, stableJSON } from "./workflow-review";

const tag = { id: "s_tag", kind: "tag", settings: { tag_ids: ["tag_1"] } };

const spec: Spec = {
  schema: 1,
  trigger: "conversation.first_message",
  schedule: { mode: "any" },
  reentry: { mode: "once_per_conversation" },
  exclusive: false,
  steps: [
    {
      id: "s_ask",
      kind: "buttons",
      settings: { text: "Pick" },
      paths: [
        { id: "p_sales", label: "Sales", steps: [{ id: "s_tag", kind: "tag", settings: { tag_ids: ["tag_1"] } }] },
      ],
    },
    { id: "s_close", kind: "close" },
  ],
};

const changeset = (id: string, created: string, workflow: string): OperatorChangeset => ({
  id,
  thread_id: null,
  message_id: null,
  conversation_id: null,
  proposed_to: null,
  title: "Edit",
  reason: "Why",
  status: "pending",
  auto: false,
  error: null,
  operations: [{ kind: "workflow.edit_draft", label: "Edit", workflow_id: workflow, text: JSON.stringify(spec) }],
  applied_at: null,
  reverted_at: null,
  created_at: created,
});

describe("workflow review", () => {
  it("compares copies whatever their key order", () => {
    expect(stableJSON({ b: 1, a: { d: [1, { f: 2, e: 3 }], c: undefined } })).toBe(
      '{"a":{"d":[1,{"e":3,"f":2}]},"b":1}',
    );
  });

  it("marks the steps a suggestion adds, changes and removes, and a changed trigger", () => {
    const proposed: Spec = {
      ...spec,
      channels: ["email"],
      steps: [
        {
          ...spec.steps[0],
          settings: { text: "Which one?" },
          paths: [{ id: "p_sales", label: "Sales", steps: [tag, { id: "s_new", kind: "note" }] }],
        },
      ],
    };

    const review = reviewOf(spec, proposed);

    expect([...review.added]).toEqual(["s_new"]);
    expect([...review.changed]).toEqual(["s_ask"]);
    expect([...review.removed]).toEqual(["s_close"]);
    expect(review.head).toBe(true);
    expect(reviewOf(spec, spec).head).toBe(false);
  });

  it("picks the oldest suggestion on the workflow", () => {
    const found = requestFor(
      [
        changeset("opc_2", "2026-11-02", "wf_1"),
        changeset("opc_1", "2026-11-01", "wf_1"),
        changeset("opc_3", "2026-10-01", "wf_2"),
      ],
      "wf_1",
    );
    expect(found?.changeset.id).toBe("opc_1");
    expect(requestFor([], "wf_1")).toBeNull();
  });
});
