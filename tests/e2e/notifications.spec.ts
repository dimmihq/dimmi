import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("notifications", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Alerts ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("keeps a member's choices and delivers a test to the bell", async ({ page }) => {
    await page.goto("/d/settings/notifications");
    await expect(page.getByRole("heading", { level: 1, name: "Notifications" })).toBeVisible();

    const mentionsByEmail = page.getByRole("switch", { name: "Mentions by Email" });
    await expect(mentionsByEmail).toBeChecked();
    const saved = page.waitForResponse(
      (r) => r.url().endsWith("/api/v1/notification-preferences") && r.request().method() === "PATCH",
    );
    await mentionsByEmail.click({ force: true });
    await saved;
    await page.reload();
    await expect(page.getByRole("switch", { name: "Mentions by Email" })).not.toBeChecked();

    await page.getByRole("button", { name: "In app", exact: true }).click();
    await expect(page.getByText("Test sent")).toBeVisible();

    await page.goto("/d/notifications");
    await expect(page.getByText("Notifications work on this device")).toBeVisible();
    await page.getByRole("button", { name: "Mark all read" }).click();
    await expect(page.getByText("No unread notifications")).toBeVisible();
  });
});
