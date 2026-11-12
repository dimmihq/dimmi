import { randomUUID } from "node:crypto";

import type { Page } from "@playwright/test";

import { createAccount, expect, inbox, test as it, signIn, signOut } from "./support/fixtures";

/*
 * approve signs in at the local test provider as the address, the way a
 * person clicks through a provider's own page.
 */
async function approve(page: Page, email: string) {
  await expect(page.getByRole("heading", { name: "Sign in to the test provider" })).toBeVisible();
  await page.getByLabel("Provider email").fill(email);
  await page.getByRole("button", { name: "Approve" }).click();
}

it.describe("sign-in methods", { tag: "@local" }, () => {
  it("a person links a provider, signs in with it, and unlinks it", async ({ page, person, request }) => {
    const providerEmail = `linked.${randomUUID().slice(0, 8)}@example.com`;
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Methods ${randomUUID().slice(0, 8)}`);
    await signIn(page, person);
    await page.goto("/d/settings/security");

    const methods = page.getByRole("region", { name: "Sign-in methods" });
    await expect(methods.getByText("You can sign in with your password.")).toBeVisible();
    await expect(methods.getByText("Not linked")).toBeVisible();
    await methods.getByRole("button", { name: "Link" }).click();
    await approve(page, providerEmail);

    await expect(page).toHaveURL(/\/d\/[^/]+\/settings\/security$/);
    await expect(page.getByText("Google linked. You can sign in with it now.")).toBeVisible();
    await expect(methods.getByText(providerEmail)).toBeVisible();

    await page.goto("/d");
    await signOut(page, person);
    await page.getByRole("link", { name: "Continue with Google" }).click();
    await approve(page, providerEmail);
    await expect(page).toHaveURL(inbox);

    await page.goto("/d/settings/security");
    await methods.getByRole("button", { name: "Unlink" }).click();
    await expect(methods.getByText("Not linked")).toBeVisible();
    await expect(methods.getByText(providerEmail)).toBeHidden();
  });
});
