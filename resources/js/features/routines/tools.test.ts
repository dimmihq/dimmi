import { describe, expect, it } from "vitest";

import type { RoutineTool } from "@/lib/api/routines";

import { findTools, toolLabel, toolsIn, withNamedGroups } from "./tools";

const tools: RoutineTool[] = [
  { name: "list_recent_changesets", group: "routine", description: "List recent suggestions. Newest first." },
  { name: "propose_changes", group: "changes", description: "Suggest changes." },
  { name: "workspace_report", group: "insights", description: "Read the report." },
  { name: "github_list_merged_prs", group: "github", description: "List pull requests merged." },
];

describe("routine tools", () => {
  it("names tools the way a teammate reads them", () => {
    expect(toolLabel("list_recent_changesets")).toBe("List recent changesets");
    expect(toolLabel("github_list_merged_prs")).toBe("GitHub list merged PRs");
  });

  it("offers the routine's own tools first, then those that give access", () => {
    const found = findTools([...tools].reverse(), ["changes"], "");
    expect(found.map((o) => o.tool.name)).toEqual([
      "list_recent_changesets",
      "propose_changes",
      "workspace_report",
      "github_list_merged_prs",
    ]);
    expect(found[0]).toMatchObject({ heading: "Every routine", hint: "List recent suggestions", adds: false });
    expect(found[2]).toMatchObject({ heading: "Reports · gives access", adds: true });
  });

  it("finds tools by what was typed", () => {
    expect(findTools(tools, [], "merged").map((o) => o.tool.name)).toEqual(["github_list_merged_prs"]);
    expect(findTools(tools, [], "recent change").map((o) => o.tool.name)).toEqual(["list_recent_changesets"]);
  });

  it("gives a routine the groups of the tools its prompt names", () => {
    const prompt = "Call [[tool:workspace_report]], then [[tool:list_recent_changesets]].";
    expect(toolsIn(prompt)).toEqual(["workspace_report", "list_recent_changesets"]);
    expect(withNamedGroups(["changes"], prompt, tools)).toEqual(["changes", "insights"]);
    const same = ["changes", "insights"] as const;
    expect(withNamedGroups([...same], prompt, tools)).toEqual([...same]);
  });
});
