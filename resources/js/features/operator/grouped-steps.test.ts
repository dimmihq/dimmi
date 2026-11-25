import { describe, expect, it } from "vitest";

import type { OperatorStep } from "@/lib/api/operator";

import { grouped, stepCount } from "./grouped-steps";

const step = (tool: string, summary: string, subagent?: number, ok = true): OperatorStep => ({
  tool,
  summary,
  ok,
  subagent,
});

describe("grouped", () => {
  it("gathers each subagent's mixed steps under it, where its first step landed", () => {
    const items = grouped([
      step("list_articles", "Listed 50 articles"),
      step("run_subagents", "Started 2 subagents"),
      step("subagent", "Articles 1 to 25", 1),
      step("subagent", "Articles 26 to 50", 2),
      step("get_article_by_id", "Read A", 2),
      step("get_article_by_id", "Read B", 1),
      step("subagent_report", "Nothing to change.", 1),
      step("subagent_report", "Did not finish.", 2, false),
      step("run_subagents", "2 subagents reported"),
    ]);

    expect(items.map((item) => (item.kind === "step" ? item.step.summary : item.group.title))).toEqual([
      "Listed 50 articles",
      "Started 2 subagents",
      "Articles 1 to 25",
      "Articles 26 to 50",
      "2 subagents reported",
    ]);
    const first = items[2];
    const second = items[3];
    expect(first.kind === "subagent" && first.group.steps.map((s) => s.summary)).toEqual(["Read B"]);
    expect(first.kind === "subagent" && first.group.report).toBe("Nothing to change.");
    expect(second.kind === "subagent" && second.group.ok).toBe(false);
  });

  it("counts real steps only", () => {
    expect(stepCount([step("subagent", "Part", 1), step("get_article_by_id", "Read A", 1), step("note", "x")])).toBe(2);
  });
});
