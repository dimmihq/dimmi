import { describe, expect, it } from "vitest";

import type { Routine, RoutineTemplate } from "@/lib/api/routines";

import { budgetMicros, draftFromRoutine, draftFromTemplate, inputOf, problems, sameDraft } from "./draft";

const template: RoutineTemplate = {
  key: "tpl_github_pr_docs_sync",
  name: "Pull requests to docs",
  description: "Every morning.",
  prompt: "Check PRs.",
  cadence: "daily",
  cron_expression: "30 8 * * *",
  tool_groups: ["knowledge", "github", "changes"],
  permissions: ["articles:read"],
  notify_policy: "on_findings",
  requires: ["github"],
};

describe("drafts", () => {
  it("fills a new routine from its template, told to its creator", () => {
    const d = draftFromTemplate(template, "Europe/Berlin", "mem_me");
    expect(d.timing).toMatchObject({ kind: "daily", hour: 8, minute: 30, timezone: "Europe/Berlin" });
    expect(d.delivery.email_member_ids).toEqual(["mem_me"]);
    expect(d.github).not.toBeNull();
    expect(problems(d)).toEqual(["Pick the GitHub repositories the routine reads."]);
  });

  it("starts a blank routine from scratch", () => {
    const d = draftFromTemplate({ ...template, key: "tpl_custom", prompt: "" }, "UTC", "mem_me");
    expect(d.templateKey).toBeNull();
    expect(problems(d)).toContain("Give the routine a name.");
  });

  it("writes what the API takes", () => {
    const d = draftFromTemplate(template, "UTC", "mem_me");
    d.github = { installationId: "itg_1", repositories: ["acme/app"], baseBranch: "" };
    d.budgetDollars = "5";
    const input = inputOf(d);
    expect(input).toMatchObject({
      cadence: "daily",
      cron_expression: "30 8 * * *",
      monthly_budget_micros: 5_000_000,
      template_key: "tpl_github_pr_docs_sync",
      scope: { github: { installation_id: "itg_1", repositories: ["acme/app"] } },
    });
    expect(input.clear_budget).toBeUndefined();
  });

  it("reads a saved routine back the same", () => {
    const routine = {
      template_key: null,
      name: "Watcher",
      description: "",
      prompt: "Check.",
      cadence: "monthly",
      cron_expression: "0 9 L * *",
      run_at: null,
      timezone: "UTC",
      tool_groups: ["knowledge"],
      permissions: [],
      inbox_ids: [],
      scope: { help_center_ids: [], github: null },
      model: null,
      monthly_budget_micros: 2_500_000,
      notify_policy: "always",
      delivery: { in_app_member_ids: [], email_member_ids: [], slack_destination_ids: [] },
    } as unknown as Routine;
    const d = draftFromRoutine(routine);
    expect(d.timing).toMatchObject({ kind: "monthly", monthDay: 0 });
    expect(d.budgetDollars).toBe("2.5");
    expect(sameDraft(d, draftFromRoutine(routine))).toBe(true);
  });

  it("reads budgets", () => {
    expect(budgetMicros("$12.5")).toBe(12_500_000);
    expect(budgetMicros("")).toBeNull();
    expect(budgetMicros("-1")).toBeNull();
  });
});
