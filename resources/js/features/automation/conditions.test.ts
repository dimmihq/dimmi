import { changeField, conditionOf, describeCondition, fieldsFor, isSimple } from "./conditions";

describe("conditions", () => {
  it("offers event fields only under the triggers that carry them", () => {
    expect(fieldsFor("message.customer")).toHaveProperty("message.body");
    expect(fieldsFor("note.created")).not.toHaveProperty("message.body");
    expect(fieldsFor("conversation.status_changed")).toHaveProperty("trigger.to");
  });

  it("builds the value in the shape its comparison takes", () => {
    expect(conditionOf("message.body", "contains_any", "refund, money back")).toEqual({
      field: "message.body",
      op: "contains_any",
      value: ["refund", "money back"],
    });
    expect(conditionOf("contact.email", "eq", "a@b.co")).toEqual({ field: "contact.email", op: "eq", value: "a@b.co" });
    expect(conditionOf("contact.email", "exists", "ignored")).toEqual({ field: "contact.email", op: "exists" });
  });

  it("keeps the comparison a new field takes and drops one it does not", () => {
    expect(changeField({ field: "message.body", op: "eq", value: "hi" }, "contact.name")).toEqual({
      field: "contact.name",
      op: "eq",
      value: "hi",
    });
    expect(changeField({ field: "message.body", op: "contains_any", value: ["x"] }, "conversation.priority")).toEqual({
      field: "conversation.priority",
      op: "in",
      value: [],
    });
  });

  it("says a condition in words", () => {
    expect(describeCondition({ field: "conversation.priority", op: "in", value: ["high", "urgent"] })).toBe(
      "Priority is any of High, Urgent",
    );
    expect(describeCondition(undefined)).toBe("No condition yet");
  });

  it("leaves grouped conditions to the API", () => {
    expect(isSimple({ all: [{ field: "contact.name", op: "exists" }] })).toBe(false);
    expect(isSimple({ field: "contact.name", op: "exists" })).toBe(true);
  });
});
