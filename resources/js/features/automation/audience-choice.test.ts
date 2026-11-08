import { describe, expect, it } from "vitest";

import { audienceChoice, audienceFor } from "./audience-choice";

describe("audienceChoice", () => {
  it("reads everyone, one saved audience, or a custom rule", () => {
    expect(audienceChoice(undefined)).toBe("everyone");
    expect(audienceChoice({ visitors: "everyone" })).toBe("everyone");
    expect(audienceChoice(audienceFor("audn_1"))).toBe("audn_1");
    expect(audienceChoice({ visitors: "identified", rules: { field: "audience", op: "in", value: ["audn_1"] } })).toBe(
      "custom",
    );
    expect(audienceChoice({ rules: { field: "contact.kind", op: "eq", value: "user" } })).toBe("custom");
  });

  it("stores nothing for everyone", () => {
    expect(audienceFor("everyone")).toBeUndefined();
  });
});
