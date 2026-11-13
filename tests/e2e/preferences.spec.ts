import { randomUUID } from "node:crypto";

import type { Page } from "@playwright/test";

import { createAccount, expect, test as it, signIn } from "./support/fixtures";

/*
 * profileSaved waits for the next change to the person's own profile.
 */
function profileSaved(page: Page) {
  return page.waitForResponse((r) => r.url().endsWith("/api/v1/user") && r.request().method() === "PATCH");
}

it.describe("own preferences", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Prefs ${randomUUID().slice(0, 8)}`);
    await signIn(page, person);
    await page.goto("/d/settings/preferences");
  });

  it("every control on the preferences page changes something", async ({ page }) => {
    const picker = page.getByRole("combobox", { name: "Time zone" });
    await expect(picker).toHaveValue(/^Workspace default/);

    const saved = profileSaved(page);
    await picker.fill("Tokyo");
    await page.getByRole("option", { name: "Asia/Tokyo" }).click();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    expect((await saved).status()).toBe(200);

    const dark = await page.evaluate(() => document.documentElement.classList.contains("dark"));
    await page.getByRole("button", { name: /Theme$/ }).click();
    await page.getByRole("option", { name: dark ? "Light" : "Dark" }).click();
    await expect(page.locator("html")).toHaveClass(dark ? /^(?!.*\bdark\b)/ : /\bdark\b/);

    await page.reload();
    await expect(page.getByRole("combobox", { name: "Time zone" })).toHaveValue("Asia/Tokyo");
    await expect(page.locator("html")).toHaveClass(dark ? /^(?!.*\bdark\b)/ : /\bdark\b/);
  });

  it("a person picks a time zone by searching and goes back to the workspace's", async ({ page }) => {
    const saved = profileSaved(page);
    await page.getByRole("combobox", { name: "Time zone" }).fill("Tokyo");
    await page.getByRole("option", { name: "Asia/Tokyo" }).click();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    expect((await saved).status()).toBe(200);
    await page.reload();
    await expect(page.getByRole("combobox", { name: "Time zone" })).toHaveValue("Asia/Tokyo");

    const cleared = profileSaved(page);
    await page.getByRole("combobox", { name: "Time zone" }).fill("Workspace");
    await page.getByRole("option", { name: /^Workspace default/ }).click();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    expect((await cleared).status()).toBe(200);
    await page.reload();
    await expect(page.getByRole("combobox", { name: "Time zone" })).toHaveValue(/^Workspace default/);
  });

  it("lists notification settings beside preferences as a page of their own", async ({ page }) => {
    await page.getByRole("link", { name: "Notifications", exact: true }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/settings\/notifications$/);
    await expect(page.getByRole("heading", { level: 1, name: "Notifications" })).toBeVisible();
    await expect(page.getByRole("combobox", { name: "Time zone" })).toHaveCount(0);
  });
});
