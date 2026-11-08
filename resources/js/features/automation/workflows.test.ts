import { findStep, newId, pathName, type Step } from "./workflows";

const steps: Step[] = [
  {
    id: "s_ask0001",
    kind: "buttons",
    settings: { text: "What is it about?" },
    paths: [
      { id: "p_billing", label: "Billing", steps: [{ id: "s_tag0001", kind: "tag", settings: { tag_ids: [] } }] },
      { id: "p_typed01", outcome: "typed", steps: [] },
    ],
  },
];

describe("findStep", () => {
  it("finds a step at the top of a workflow", () => {
    expect(findStep(steps, "s_ask0001")?.kind).toBe("buttons");
  });

  it("finds a step down nested paths", () => {
    expect(findStep(steps, "s_tag0001")?.kind).toBe("tag");
  });

  it("returns nothing for an unknown step", () => {
    expect(findStep(steps, "missing")).toBeUndefined();
  });
});

describe("pathName", () => {
  it("names a path by its own label, or a fixed path by its outcome", () => {
    expect(pathName(steps[0]?.paths?.[0] ?? { id: "", steps: [] })).toBe("Billing");
    expect(pathName(steps[0]?.paths?.[1] ?? { id: "", steps: [] })).toBe("Typed an answer");
  });
});

describe("newId", () => {
  it("makes ids in the shape the server accepts", () => {
    expect(newId("s_")).toMatch(/^s_[a-z0-9]{10}$/);
    expect(newId("p_")).toMatch(/^p_[a-z0-9]{10}$/);
  });
});
