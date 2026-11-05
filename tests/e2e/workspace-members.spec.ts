import { addTeammate, markTwoFactorOn } from "./support/database";
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

it.describe("workspace and members", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("renames the workspace and moves its address", async ({ page, person, request }) => {
    await createAccount(request, person, `Rename ${suffixOf(person)}`);
    await signIn(page, person);
    await page.goto("/d/settings/workspace");

    const name = page.getByRole("textbox", { name: "Workspace name" });
    await expect(name).toHaveValue(`Rename ${suffixOf(person)}`);
    const save = page.getByRole("button", { name: "Save changes" });
    await expect(save).toBeDisabled();

    await name.fill(`Renamed ${suffixOf(person)}`);
    await page.getByRole("textbox", { name: "Workspace address" }).fill(`moved-${suffixOf(person)}`);
    const saved = page.waitForResponse(
      (r) => r.url().endsWith("/api/v1/workspace") && r.request().method() === "PATCH",
    );
    await save.click();
    expect((await saved).status()).toBe(200);
    await expect(save).toBeDisabled();

    await page.reload();
    await expect(page.getByRole("textbox", { name: "Workspace name" })).toHaveValue(`Renamed ${suffixOf(person)}`);
    await expect(page.getByRole("textbox", { name: "Workspace address" })).toHaveValue(`moved-${suffixOf(person)}`);
  });

  it("sets the name customers see and the workspace time zone", async ({ page, person, request }) => {
    await createAccount(request, person, `Facing ${suffixOf(person)}`);
    await signIn(page, person);
    await page.goto("/d/settings/workspace");

    const customerName = page.getByRole("textbox", { name: "Name customers see" });
    await expect(customerName).toHaveAttribute("placeholder", `Facing ${suffixOf(person)}`);
    await customerName.fill("Acme Support");
    const saved = page.waitForResponse(
      (r) => r.url().endsWith("/api/v1/workspace") && r.request().method() === "PATCH",
    );
    await page.getByRole("button", { name: "Save changes" }).click();
    expect((await saved).status()).toBe(200);

    const zoned = page.waitForResponse(
      (r) => r.url().endsWith("/api/v1/workspace") && r.request().method() === "PATCH",
    );
    await page.getByRole("combobox", { name: "Workspace time zone" }).fill("Lagos");
    await page.getByRole("option", { name: "Africa/Lagos" }).click();
    expect((await zoned).status()).toBe(200);

    await page.reload();
    await expect(page.getByRole("textbox", { name: "Name customers see" })).toHaveValue("Acme Support");
    await expect(page.getByRole("combobox", { name: "Workspace time zone" })).toHaveValue("Africa/Lagos");
  });

  it("refuses an address the product keeps for itself", async ({ page, person, request }) => {
    await createAccount(request, person, `Reserved ${suffixOf(person)}`);
    await signIn(page, person);
    await page.goto("/d/settings/workspace");

    await page.getByRole("textbox", { name: "Workspace address" }).fill("admin");
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(page.getByRole("alert")).toHaveText("This address is reserved. Choose another one.");
  });

  it("changes a teammate's role, deactivates and reactivates them", async ({ page, person, request }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, `Team ${suffixOf(person)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, person);
    await page.goto("/d/settings/members");

    const members = page.getByRole("region", { name: "Members" });
    await expect(members.getByText(`${person.name} (you)`)).toBeVisible();
    await expect(members.getByText(teammate.name)).toBeVisible();
    await expect(page.getByText("2 active")).toBeVisible();

    await members.getByRole("button", { name: `Manage ${teammate.name}` }).click();
    await page.getByRole("button", { name: "Make admin" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Make admin" }).click();
    await expect(members.getByText("Admin", { exact: true })).toBeVisible();

    await members.getByRole("button", { name: `Manage ${teammate.name}` }).click();
    await page.getByRole("button", { name: "Deactivate" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Deactivate" }).click();
    await expect(members.getByText(`${teammate.email} · Deactivated`)).toBeVisible();
    await expect(page.getByText("1 active")).toBeVisible();

    await members.getByRole("button", { name: `Manage ${teammate.name}` }).click();
    await page.getByRole("button", { name: "Reactivate" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Reactivate" }).click();
    await expect(page.getByText("2 active")).toBeVisible();
  });

  it("hands ownership over, then leaves the workspace", async ({ page, person, request }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, `Handover ${suffixOf(person)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "admin");
    await signIn(page, person);
    await page.goto("/d/settings/members");

    const members = page.getByRole("region", { name: "Members" });
    await members.getByRole("button", { name: `Manage ${teammate.name}` }).click();
    await page.getByRole("button", { name: "Transfer ownership" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Transfer ownership" }).click();
    await expect(members.getByText("Owner", { exact: true })).toBeVisible();
    await expect(members.getByText("Admin", { exact: true })).toBeVisible();

    await page.getByRole("button", { name: "Leave workspace" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Leave workspace" }).click();
    await expect(page).not.toHaveURL(/\/settings\/members$/);
  });

  it("an admin sees who has no second factor", async ({ page, person, request }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, `Review ${suffixOf(person)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, person);
    markTwoFactorOn(person.email);

    await page.goto("/d/settings/workspace");
    await expect(page.getByText("1 member has no second factor yet.")).toBeVisible();
    await page.getByRole("link", { name: "See who" }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/settings\/members\?two_factor=false$/);

    const members = page.getByRole("region", { name: "Members" });
    await expect(members.getByText(teammate.name)).toBeVisible();
    await expect(members.getByText(`${person.name} (you)`)).toBeHidden();
    await expect(members.getByText("No two-factor")).toBeVisible();

    await members.getByRole("button", { name: /Show/ }).click();
    await page.getByRole("option", { name: "Everyone" }).click();
    await expect(members.getByText(`${person.name} (you)`)).toBeVisible();
    await expect(members.getByText("No two-factor")).toHaveCount(1);

    const download = page.waitForEvent("download");
    await members.getByRole("button", { name: "Export CSV" }).click();
    expect((await download).suggestedFilename()).toBe(`dimmi-members-review-${suffixOf(person)}.csv`);
  });
});
