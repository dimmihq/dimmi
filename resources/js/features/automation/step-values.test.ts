import { describe, expect, it } from "vitest";

import type { Action } from "@/lib/api/ai-actions";

import { addAIFields } from "./ai-condition-fields";
import { conditionFields, fieldsFor } from "./conditions";
import { inputsOf, keyFor, sourceOf, storedValue, withNonePath } from "./step-values";
import type { Step } from "./workflows";

describe("step values", () => {
  it("reads and writes where an input comes from", () => {
    expect(sourceOf("{{contact.email}}")).toEqual(["contact.email", ""]);
    expect(sourceOf("{{run.vars.order}}")).toEqual(["run.vars.", "order"]);
    expect(sourceOf("{{contact.custom.plan}}")).toEqual(["contact.custom.", "plan"]);
    expect(sourceOf("{{conversation.ai.intent}}")).toEqual(["conversation.ai.intent", ""]);
    expect(sourceOf("pro")).toEqual(["fixed", "pro"]);

    expect(storedValue("contact.email", "")).toBe("{{contact.email}}");
    expect(storedValue("run.vars.", " order ")).toBe("{{run.vars.order}}");
    expect(storedValue("run.vars.", "")).toBe("");
    expect(storedValue("fixed", "pro")).toBe("pro");
  });

  it("names a value's key from its name", () => {
    expect(keyFor("Order number")).toBe("order_number");
    expect(keyFor("3 Seats!")).toBe("seats");
  });

  it("lists an action's inputs, required first and bound ones left out", () => {
    const action = {
      input_schema: {
        properties: { note: {}, email: {}, account: {} },
        required: ["email"],
      },
      bound_arguments: { account: "acme" },
    } as unknown as Action;

    expect(inputsOf(action)).toEqual([
      { name: "email", required: true },
      { name: "note", required: false },
    ]);
  });

  it("adds and removes the not found path", () => {
    const step: Step = { id: "s_1", kind: "extract", settings: {} };
    const on = withNonePath(step, true);
    expect(on.paths?.map((path) => path.outcome)).toEqual(["none"]);
    expect(withNonePath(on, false).paths).toBeUndefined();
  });

  it("offers AI attributes as condition fields", () => {
    addAIFields([
      {
        id: "aica_1",
        object: "conversation_attribute",
        key: "intent",
        label: "Intent",
        description: "",
        data_type: "choice",
        options: ["refund", "question"],
        option_descriptions: {},
        filled_by: "both",
        required_to_close: false,
        team_ids: [],
        status: "active",
        created_at: "",
        updated_at: "",
      },
    ]);

    expect(conditionFields["conversation.ai.intent"]).toMatchObject({ name: "Intent", type: "choice" });
    expect(fieldsFor("conversation.ai_attribute_changed")).toHaveProperty("ai_attribute.key");
    expect(fieldsFor("message.customer")).not.toHaveProperty("ai_attribute.key");
  });
});
