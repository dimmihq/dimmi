import { describe, expect, it } from "vitest";

import { isNewName, matching } from "./tag-names";

const tag = (name: string) => ({ id: name, object: "tag", name, color: null });

describe("matching", () => {
  it("narrows by any part of the name, without regard to case", () => {
    const tags = [tag("Billing"), tag("Refund"), tag("bill-later")];

    expect(matching(tags, " BILL ").map((t) => t.name)).toEqual(["Billing", "bill-later"]);
  });

  it("shows only the first few when nothing is typed", () => {
    const tags = Array.from({ length: 20 }, (_, i) => tag(`tag-${i}`));

    expect(matching(tags, "")).toHaveLength(8);
  });
});

describe("isNewName", () => {
  it("treats a name in another case as the same tag", () => {
    expect(isNewName([tag("Billing")], "billing")).toBe(false);
  });

  it("offers a name nobody has", () => {
    expect(isNewName([tag("Billing")], "Bug")).toBe(true);
  });

  it("offers nothing for blank text", () => {
    expect(isNewName([], "   ")).toBe(false);
  });
});
