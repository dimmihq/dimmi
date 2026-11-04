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
