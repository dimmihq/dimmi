import { readFile } from "node:fs/promises";

import { addTeammate } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

/*
 * teammateOf is a second person for the test, with an address of their
 * own derived from the first person's.
 */
function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

/*
 * suffixOf is the unique part of the person's address, so workspaces
 * made at the same moment never race for one name or address.
 */
function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("audit log and security activity", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("shows who changed a role, and narrows the log by event and actor", async ({ page, person, request }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, `Audit ${suffixOf(person)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, person);

    await page.goto("/d/settings/members");
    const members = page.getByRole("region", { name: "Members" });
    await members.getByRole("button", { name: `Manage ${teammate.name}` }).click();
    await page.getByRole("button", { name: "Make admin" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Make admin" }).click();
    await expect(members.getByText("Admin", { exact: true })).toBeVisible();

    await page.goto("/d/settings/audit-log");
    const log = page.getByRole("grid", { name: "Audit log" });
    const change = log.getByRole("row").filter({ hasText: "Changed a role" });
    await expect(change).toContainText(person.name);
    await expect(change).toContainText(teammate.name);
    await expect(change).toContainText("Browser");
    await expect(log.getByRole("row").filter({ hasText: "Created the workspace" })).toBeVisible();

    await page.getByRole("button", { name: "Event" }).click();
    await page.getByRole("option", { name: "Changed a role" }).click();
    await expect(log.getByRole("row").filter({ hasText: "Created the workspace" })).toHaveCount(0);
    await expect(change).toBeVisible();

    await page.getByRole("button", { name: "Event" }).click();
    await page.getByRole("option", { name: "All events" }).click();
    await page.getByRole("button", { name: "Actor" }).click();
    await page.getByRole("option", { name: teammate.name }).click();
    await expect(log.getByText("No events match.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Older" })).toBeDisabled();
  });

  it("an admin filters by date and downloads what they see", async ({ page, person, request }) => {
    await createAccount(request, person, `Export ${suffixOf(person)}`);
    await signIn(page, person);

    await page.goto("/d/settings/audit-log");
    const log = page.getByRole("grid", { name: "Audit log" });
    await expect(log.getByRole("row").filter({ hasText: "Created the workspace" })).toBeVisible();

    await page.getByRole("button", { name: "Any time" }).click();
    const calendar = page.getByRole("dialog", { name: "Time range" });
    await calendar.getByRole("button", { name: "Last 7 days" }).click();
    await calendar.getByRole("button", { name: "Apply" }).click();
    await expect(calendar).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Last 7 days" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Clear dates" })).toBeVisible();
    const created = log.getByRole("row").filter({ hasText: "Created the workspace" });
    await expect(created).toBeVisible();

    await page.getByRole("button", { name: "Event" }).click();
    await page.getByRole("option", { name: "Any members and teams event" }).click();
    await expect(created).toBeVisible();

    await created.click();
    const detail = page.getByRole("dialog");
    await expect(detail.getByText("IP address")).toBeVisible();
    await expect(detail.getByText("workspace.created", { exact: true })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(detail).toHaveCount(0);

    const waiting = page.waitForEvent("download");
    await page.getByRole("button", { name: "Export CSV" }).click();
    const download = await waiting;
    expect(download.suggestedFilename()).toMatch(
      /^dimmi-audit-export-[a-z0-9-]+-\d{4}-\d{2}-\d{2}-\d{4}-\d{2}-\d{2}\.csv$/,
    );
    const file = await download.path();
    const text = await readFile(file, "utf8");
    expect(text.split("\n")[0]).toBe(
      "id,created_at,event,actor_type,actor_id,actor_label,via_kind,via_id,subject_type,subject_id,subject_label,changes,ip,user_agent,request_id",
    );
    expect(text).toContain("workspace.created");
    expect(text).not.toContain("member.signed_in");

    await page.getByRole("button", { name: "Event" }).click();
    await page.getByRole("option", { name: "Exported the audit log" }).click();
    await expect(log.getByRole("row").filter({ hasText: "Exported the audit log" })).toBeVisible();
  });

  it("tells a member the log is for owners and admins", async ({ page, person, request }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, `Closed ${suffixOf(person)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, teammate);

    await page.goto("/d/settings/audit-log");
    await expect(page.getByText("Only owners and admins can read the audit log.")).toBeVisible();
  });

  it("lists the person's own sign-ins on the security page", async ({ page, person, request }) => {
    await createAccount(request, person, `Activity ${suffixOf(person)}`);
    await signIn(page, person);

    await page.goto("/d/settings/security");
    const activity = page.getByRole("region", { name: "Recent security activity" });
    await expect(activity.getByText("Signed in", { exact: true }).first()).toBeVisible();
  });
});
