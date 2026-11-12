import { describe, expect, it } from "vitest";

import { ratedText } from "./conversations";

const actor = { type: "contact", id: "ctc_1", name: "Ada" };

describe("ratedText", () => {
  it("says how the customer rated the conversation", () => {
    expect(ratedText({ name: "conversation.rated", actor, score: 5, label: "Amazing" })).toBe(
      "Ada rated the conversation 😍 Amazing",
    );
  });

  it("adds the comment the customer left", () => {
    expect(ratedText({ name: "conversation.rated", actor, score: 2, label: "Bad", comment: "Slow" })).toBe(
      "Ada rated the conversation 🙁 Bad: “Slow”",
    );
  });

  it("leaves other events alone", () => {
    expect(ratedText({ name: "conversation.closed", actor })).toBeNull();
  });
});
