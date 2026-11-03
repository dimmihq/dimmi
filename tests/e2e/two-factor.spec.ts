import type { Page } from "@playwright/test";

import { createAccount, expect, inbox, test as it, type Person, signIn, signOut } from "./support/fixtures";
import { currentStep, totpCode } from "./support/totp";

it.describe("two-factor authentication", () => {
  it("turns on with an authenticator app, then asks for a code or a recovery code at sign-in", async ({
    page,
    person,
    request,
  }) => {
    await createAccount(request, person);
    await signIn(page, person);

    await page.goto("/d/settings/security");
    const section = page.getByRole("region", { name: "Two-factor authentication" });
    await section.getByRole("button", { name: "Add authenticator" }).click();

    const setup = page.getByRole("dialog", { name: "Add an authenticator app" });
    await expect(setup.getByRole("img", { name: /QR code/ })).toBeVisible();
    const secret = (await setup.getByTestId("totp-secret").textContent())?.trim() ?? "";
    expect(secret).toMatch(/^[A-Z2-7]+$/);

    const activatedAt = currentStep();
    await setup.getByLabel("Digit 1 of 6").pressSequentially(totpCode(secret, activatedAt));

    const saved = page.getByRole("dialog", { name: "Save your recovery codes" });
    const items = saved.getByRole("list", { name: "Recovery codes" }).getByRole("listitem");
    await expect(items).toHaveCount(10);
    const codes = await items.allTextContents();
    await saved.getByRole("button", { name: "I saved them" }).click();

    await expect(section.getByText("10 of 10 left")).toBeVisible();
    await expect(section.getByRole("button", { name: "Turn off" })).toBeVisible();

    await page.goto("/d");
    await signOut(page, person);
    await answerPassword(page, person);
    await expect(page).toHaveURL(/\/login\/two-factor/);
    const step = Math.max(currentStep(), activatedAt + 1);
    await page.getByLabel("Digit 1 of 6").pressSequentially(totpCode(secret, step));
    await expect(page).toHaveURL(inbox);

    await signOut(page, person);
    await answerPassword(page, person);
    await page.getByRole("button", { name: "Use a recovery code instead" }).click();
    await page.getByLabel("Recovery code").fill(codes[0] ?? "");
    await page.getByRole("button", { name: "Verify" }).click();
    await expect(page).toHaveURL(inbox);

    await page.goto("/d/settings/security");
    await expect(section.getByText("9 of 10 left")).toBeVisible();
  });
});

/*
 * answerPassword passes the first step of signing in, which on an account
 * with two-factor stops at the second.
 */
async function answerPassword(page: Page, person: Person): Promise<void> {
  await page.goto("/login");
  await page.getByLabel("Email").fill(person.email);
  await page.getByLabel("Password").fill(person.password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
}
