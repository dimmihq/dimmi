import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";

import type { Page } from "@playwright/test";

import { createAccount, expect, test as it, signIn, signOut } from "./support/fixtures";

/*
 * pseudo is the pseudo locale's settings text, generated from the English
 * one, which the test server serves beside English.
 */
const pseudo = JSON.parse(readFileSync("resources/js/lang/en-XA/settings.json", "utf8")) as {
  language: { label: string };
};

/*
 * pickLanguage opens the language select labelled with the name and picks
 * the pseudo locale, whose name starts with an accented E.
 */
async function pickLanguage(page: Page, name: RegExp) {
  const saved = page.waitForResponse((r) => r.url().includes("/api/v1/") && r.request().method() === "PATCH");
  await page.getByRole("button", { name }).click();
  await page.getByRole("option", { name: /^\[Éñĝļîšĥ/ }).click();
  expect((await saved).status()).toBe(200);
}

it.describe("interface language", { tag: "@local" }, () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Language ${randomUUID().slice(0, 8)}`);
    await signIn(page, person);
  });

  it("every request names the language the app reads in, and the page says it", async ({ page }) => {
    const asked = page.waitForRequest((r) => r.url().endsWith("/api/v1/auth/config"));
    await page.goto("/d/settings/preferences");

    expect((await asked).headers()["accept-language"]).toBe("en");
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.locator("html")).toHaveAttribute("dir", "ltr");
  });

  it("a person picks a language and the app changes without reloading", async ({ page }) => {
    await page.goto("/d/settings/preferences");
    await page.evaluate(() => {
      document.body.dataset.sameDocument = "yes";
    });

    await pickLanguage(page, /Language$/);

    await expect(page.locator("html")).toHaveAttribute("lang", "en-XA");
    await expect(page.getByText(pseudo.language.label, { exact: true })).toBeVisible();
    await expect(page.locator("body")).toHaveAttribute("data-same-document", "yes");

    const asked = page.waitForRequest((r) => r.url().endsWith("/api/v1/user") && r.method() === "PATCH");
    await page.getByRole("button", { name: pseudo.language.label }).click();
    await page.getByRole("option", { name: "English", exact: true }).click();
    expect((await asked).headers()["accept-language"]).toBe("en-XA");
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
  });

  it("the choice survives a new sign-in", async ({ page, person }) => {
    await page.goto("/d/settings/preferences");
    await pickLanguage(page, /Language$/);
    await expect(page.locator("html")).toHaveAttribute("lang", "en-XA");

    await signOut(page, person);
    await signIn(page, person);

    await expect(page.locator("html")).toHaveAttribute("lang", "en-XA");
  });

  it("a server error reads in that language", async ({ page }) => {
    await page.goto("/d/settings/preferences");
    await pickLanguage(page, /Language$/);
    await expect(page.locator("html")).toHaveAttribute("lang", "en-XA");

    await page.goto("/d/settings/workspace");
    await page.getByRole("textbox", { name: "Workspace address" }).fill("Not A Slug");
    await page.getByRole("button", { name: "Save changes" }).click();

    await expect(page.getByRole("alert")).toHaveText(/^\[Ŧĥé slug ɱåý öñļý/);
  });

  it("a workspace's language reaches a person who picked none", async ({ page }) => {
    await page.goto("/d/settings/workspace");

    await pickLanguage(page, /Workspace language$/);

    await expect(page.locator("html")).toHaveAttribute("lang", "en-XA");
    await page.goto("/d/settings/preferences");
    await expect(page.getByRole("button", { name: /Éñĝļîšĥ/ })).toBeVisible();
  });
});
