import {
  sentence as describeStep,
  type Names,
  unsetSummary,
  withNewPath,
  withoutPath,
  withPathLabel,
  withSettings,
} from "./step-settings";
import type { Step } from "./workflows";

const names: Names = {
  teams: new Map([["team_1", "Billing"]]),
  members: new Map([["mem_1", "Lea Martin"]]),
  tags: new Map([
    ["tag_1", "VIP"],
    ["tag_2", "Bug"],
  ]),
};

describe("describe", () => {
  it("puts what is typed on the card, or asks for it", () => {
    expect(describeStep({ id: "s", kind: "message", settings: { text: "Hi there" } }, names)).toBe("Hi there");
    expect(describeStep({ id: "s", kind: "message", settings: { text: "  " } }, names)).toBe(unsetSummary);
  });

  it("names teams, teammates and tags rather than their ids", () => {
    expect(describeStep({ id: "s", kind: "assign", settings: { mode: "team", team_id: "team_1" } }, names)).toBe(
      "Assign to Billing",
    );
    expect(
      describeStep({ id: "s", kind: "assign", settings: { mode: "team_round_robin", team_id: "team_1" } }, names),
    ).toBe("Assign to Billing by round robin");
    expect(describeStep({ id: "s", kind: "assign", settings: { mode: "member", member_id: "mem_1" } }, names)).toBe(
      "Assign to Lea Martin",
    );
    expect(describeStep({ id: "s", kind: "tag", settings: { tag_ids: ["tag_1", "tag_2"] } }, names)).toBe(
      "Tag VIP, Bug",
    );
  });

  it("asks for a team or tags until one is picked", () => {
    expect(describeStep({ id: "s", kind: "assign", settings: { mode: "team" } }, names)).toBe(unsetSummary);
    expect(describeStep({ id: "s", kind: "untag", settings: { tag_ids: [] } }, names)).toBe(unsetSummary);
  });

  it("says how long a wait lasts", () => {
    expect(describeStep({ id: "s", kind: "wait", settings: { for: "1d" } }, names)).toBe("Wait for 1 day");
  });

  it("says when a step cannot run yet", () => {
    expect(describeStep({ id: "s", kind: "classify" }, names)).toBe("Sort by topic, not available yet");
  });

  it("says what dimmi is told while it handles the conversation", () => {
    expect(describeStep({ id: "s", kind: "dimmi", settings: { instructions: "Only billing." } }, names)).toBe(
      "dimmi answers, following: Only billing.",
    );
  });
});

describe("withSettings", () => {
  it("changes some settings and drops those set to undefined", () => {
    const step: Step = { id: "s", kind: "assign", settings: { mode: "team", team_id: "team_1" } };

    expect(withSettings(step, { mode: "member", team_id: undefined }).settings).toEqual({ mode: "member" });
    expect(step.settings).toEqual({ mode: "team", team_id: "team_1" });
  });
});

describe("paths", () => {
  const branch: Step = {
    id: "s",
    kind: "branch",
    paths: [
      { id: "p_one00001", label: "One", steps: [] },
      { id: "p_else0001", outcome: "else", steps: [] },
    ],
  };

  it("adds a named path ahead of the fixed ones", () => {
    expect(withNewPath(branch, "Two").paths?.map((path) => path.label ?? path.outcome)).toEqual(["One", "Two", "else"]);
  });

  it("renames and removes a path", () => {
    expect(withPathLabel(branch, "p_one00001", "First").paths?.[0]?.label).toBe("First");
    expect(withoutPath(branch, "p_one00001").paths).toHaveLength(1);
  });
});
