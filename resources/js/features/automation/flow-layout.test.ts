import { addSize, cardHeight, cardWidth, layoutWorkflow, pathGap, stepGap } from "./flow-layout";
import type { Step } from "./workflows";

const steps: Step[] = [
  { id: "a", kind: "message", settings: { text: "Hello" } },
  {
    id: "b",
    kind: "branch",
    paths: [
      { id: "p1", label: "Enterprise", steps: [{ id: "c", kind: "tag" }] },
      { id: "p2", outcome: "else", steps: [] },
    ],
  },
];

function at(placed: ReturnType<typeof layoutWorkflow>["placed"], id: string) {
  const found = placed.find((entry) => entry.item.id === id);
  if (!found) {
    throw new Error(`nothing placed for ${id}`);
  }

  return found;
}

describe("layoutWorkflow", () => {
  it("centres the trigger on zero and puts the first step under it", () => {
    const { placed } = layoutWorkflow(steps, new Map());

    expect(at(placed, "trigger")).toMatchObject({ x: -cardWidth / 2, y: 0 });
    expect(at(placed, "a")).toMatchObject({ x: -cardWidth / 2, y: cardHeight + stepGap });
  });

  it("keeps paths in their listed order and ends each with a plus", () => {
    const { placed } = layoutWorkflow(steps, new Map());

    expect(at(placed, "c").x).toBeLessThan(at(placed, "add:p2").x);
    expect(at(placed, "add:p1").item.kind).toBe("add");
    expect(placed.some((entry) => entry.item.id === "add:root")).toBe(false);
  });

  it("labels the first line of each path with its name", () => {
    const { links } = layoutWorkflow(steps, new Map());

    expect(links.find((link) => link.target === "c")?.label).toBe("Enterprise");
    expect(links.find((link) => link.target === "add:p2")?.label).toBe("Everyone else");
    expect(links.find((link) => link.target === "add:p1")?.label).toBeUndefined();
  });

  it("pushes later cards down by the measured height of a tall card", () => {
    const { placed } = layoutWorkflow(steps, new Map([["b", 140]]));
    const branch = at(placed, "b");

    expect(at(placed, "c").y).toBe(branch.y + 140 + pathGap);
    expect(at(placed, "add:p1").y).toBe(at(placed, "c").y + cardHeight + stepGap);
  });

  it("lists cards path by path so the keyboard walks each path in turn", () => {
    const { placed } = layoutWorkflow(steps, new Map());

    expect(placed.map((entry) => entry.item.id)).toEqual(["trigger", "a", "b", "c", "add:p1", "add:p2"]);
  });

  it("keeps sibling columns from overlapping", () => {
    const { placed } = layoutWorkflow(steps, new Map());

    expect(at(placed, "add:p2").x + addSize / 2).toBeGreaterThan(at(placed, "c").x + cardWidth);
  });
});
