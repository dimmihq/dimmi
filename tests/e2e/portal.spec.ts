import { expect, test as it } from "./support/fixtures";

it.describe("the public portal", () => {
  it("moves between pages in place, without reloading the document", async ({ page }) => {
    await page.goto("/hc/dimmi");
    await expect(page.getByRole("heading", { level: 1 })).toContainText("How can we help?");
    await page.evaluate(() => {
      (window as unknown as { kept: boolean }).kept = true;
    });

    await page.getByRole("link", { name: "Getting started" }).first().click();
    await expect(page).toHaveURL(/\/hc\/dimmi\/collections\/getting-started$/);
    await page.getByRole("link", { name: "Invite your team" }).first().click();

    await expect(page.getByRole("heading", { level: 1, name: "Invite your team" })).toBeVisible();
    expect(await page.evaluate(() => (window as unknown as { kept?: boolean }).kept)).toBe(true);
  });

  it("answers a question from the articles", async ({ page }) => {
    await page.goto("/hc/dimmi");

    await page.getByPlaceholder("Ask anything…").fill("custom domain");
    await page.getByRole("button", { name: "Ask" }).click();

    await expect(page.getByRole("link", { name: /Use a custom domain/ })).toBeVisible();
  });

  it("thanks a reader who says an article helped", async ({ page }) => {
    await page.goto("/hc/dimmi/articles/invite-your-team");

    await page.getByRole("button", { name: "Yes" }).click();

    await expect(page.getByText("Thanks for letting us know.")).toBeVisible();
  });

  it("shows the changelog on a timeline and filters it by label", async ({ page }) => {
    await page.goto("/hc/dimmi/changelog");
    await expect(page.getByRole("navigation", { name: "Releases in 2026" })).toBeVisible();

    await page.getByRole("link", { name: "Fixed" }).first().click();

    await expect(page).toHaveURL(/\/changelog\/labels\/fixed$/);
    await expect(page.getByRole("heading", { name: "Fixes and polish" })).toBeVisible();
  });

  it("signs a visitor in to their conversations and out again", async ({ page }) => {
    await page.goto("/hc/dimmi/conversations");
    await expect(page).toHaveURL(/\/hc\/dimmi\/sign-in$/);

    await page.getByLabel("Email").fill("ada@example.com");
    await page.getByRole("button", { name: "Email me a sign-in link" }).click();
    await expect(page.getByRole("heading", { name: "Your conversations" })).toBeVisible();

    await page.getByRole("link", { name: /Webhook retries after a timeout/ }).click();
    await expect(page.getByText("retried for up to 24 hours")).toBeVisible();

    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
  });
});
