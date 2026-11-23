import { describe, expect, it } from "vitest";

import type { Routine, RoutineTemplate } from "@/lib/api/routines";

import { draftFromRoutine, draftFromTemplate, inputOf, problems, sameDraft } from "./draft";

const template: RoutineTemplate = {
  key: "tpl_github_pr_docs_sync",
  name: "Pull requests to docs",
  description: "Every morning.",
  prompt: "Check PRs.",
  cadence: "daily",
  cron_expression: "30 8 * * *",
  notify_policy: "on_findings",
  requires: ["github"],
};

describe("drafts", () => {
  it("fills a new routine from its template, told to its creator", () => {
    const d = draftFromTemplate(template, "Europe/Berlin", "mem_me");
    expect(d.timing).toMatchObject({ kind: "daily", hour: 8, minute: 30, timezone: "Europe/Berlin" });
    expect(inputOf(d).delivery).toEqual({
      in_app_member_ids: ["mem_me"],
      email_member_ids: ["mem_me"],
      slack_destination_ids: [],
    });
    expect(d.prompt).toBe("Check PRs.");
    expect(problems(d)).toEqual([]);
  });

  it("starts a blank routine from scratch", () => {
    const d = draftFromTemplate({ ...template, key: "tpl_custom", prompt: "" }, "UTC", "mem_me");
    expect(d.templateKey).toBeNull();
    expect(problems(d)).toContain("Give the routine a name.");
  });

  it("writes what the API takes", () => {
    const d = draftFromTemplate(template, "UTC", "mem_me");
    d.github = { installationId: "itg_1", repositories: ["acme/app"], baseBranch: "" };
    const input = inputOf(d);
    expect(input).toMatchObject({
      cadence: "daily",
      cron_expression: "30 8 * * *",
      template_key: "tpl_github_pr_docs_sync",
      scope: { github: { installation_id: "itg_1", repositories: ["acme/app"] } },
    });
    expect(Object.keys(input)).not.toContain("permissions");
  });

  it("reads no repositories until some are picked", () => {
    const d = draftFromTemplate(template, "UTC", "mem_me");
    expect(inputOf(d).scope).toEqual({});
    d.github = { installationId: "itg_1", repositories: [], baseBranch: "" };
    expect(inputOf(d).scope).toEqual({});
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
      inbox_ids: [],
      scope: { help_center_ids: [], github: null },
      notify_policy: "always",
      delivery: { in_app_member_ids: [], email_member_ids: [], slack_destination_ids: [] },
    } as unknown as Routine;
    const d = draftFromRoutine(routine);
    expect(d.timing).toMatchObject({ kind: "monthly", monthDay: 0 });
    expect(sameDraft(d, draftFromRoutine(routine))).toBe(true);
  });

  it("tells the same people on each channel picked", () => {
    const d = draftFromTemplate(template, "UTC", "mem_me");
    d.github = { installationId: "itg_1", repositories: ["acme/app"], baseBranch: "" };
    d.notify = { channels: ["email", "slack"], people: ["mem_me", "mem_ana"], slack: [] };
    expect(problems(d)).toEqual(["Pick the Slack channels to post to."]);
    d.notify.slack = ["dst_1"];
    expect(inputOf(d).delivery).toEqual({
      in_app_member_ids: [],
      email_member_ids: ["mem_me", "mem_ana"],
      slack_destination_ids: ["dst_1"],
    });
    d.notify = { channels: ["in_app"], people: [], slack: [] };
    expect(problems(d)).toEqual(["Pick who hears about each run."]);
  });
});
