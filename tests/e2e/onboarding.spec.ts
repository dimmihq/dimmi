import { expect, inbox, test as it } from "./support/fixtures";

it.describe("setting up a new workspace", () => {
  it("walks the steps, saves tags and test conversations, and opens the app", async ({ page, person }) => {
    await page.goto("/signup");
    await page.getByLabel("Full name").fill(person.name);
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Create account" }).click();

    await expect(page).toHaveURL(/\/onboarding$/);
    await expect(page.getByText("Step 1 of 7")).toBeVisible();
    await page.getByLabel("Workspace name").fill("Acme");
    await page.getByRole("button", { name: "Save and continue" }).click();

    await expect(page.getByRole("heading", { name: "Add your logo" })).toBeVisible();
    await page.getByRole("button", { name: "Continue without a logo" }).click();

    await expect(page.getByRole("heading", { name: "Connect a channel" })).toBeVisible();
    await expect(page.getByRole("region", { name: "Messenger" })).toContainText("data-app-id=");
    await page.getByRole("button", { name: "Continue", exact: true }).click();

    await page.getByRole("button", { name: "Billing" }).click();
    await page.getByRole("button", { name: "Bug" }).click();
    await page.getByRole("button", { name: "Save and continue" }).click();

    await page.getByRole("button", { name: "Continue without inviting" }).click();
    await page.getByRole("button", { name: "Continue without knowledge" }).click();
    await page.getByRole("button", { name: "Add 5 test conversations" }).click();

    await expect(page.getByRole("heading", { name: "Acme is ready" })).toBeVisible();
    await expect(page.getByText("Billing, Bug")).toBeVisible();
    await page.getByRole("button", { name: "Go to your workspace" }).click();

    await expect(page).toHaveURL(inbox);
    await expect(page.getByText("Emily Johnson").first()).toBeVisible();

    await page.goto("/onboarding");
    await expect(page).toHaveURL(inbox);
  });

  it("resumes on the step it reached after a reload", async ({ page, person }) => {
    await page.goto("/signup");
    await page.getByLabel("Full name").fill(person.name);
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Create account" }).click();
    await page.getByLabel("Workspace name").fill("Acme");
    await page.getByRole("button", { name: "Save and continue" }).click();
    await page.getByRole("button", { name: "Continue without a logo" }).click();
    await expect(page.getByRole("heading", { name: "Connect a channel" })).toBeVisible();

    await page.reload();
    await expect(page.getByRole("heading", { name: "Connect a channel" })).toBeVisible();

    await page.goto("/d/settings/members");
    await expect(page).toHaveURL(/\/onboarding$/);
  });
});
