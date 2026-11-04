import type { Page } from "@playwright/test";

import { createAccount, expect, inbox, test as it, type Person, signIn, signOut } from "./support/fixtures";

it.describe("passkeys", () => {
  it("adds a passkey, signs in with it, renames it and removes it", async ({ page, person, request }) => {
    await createAccount(request, person);
    await signIn(page, person);
    await addVirtualAuthenticator(page);

    await page.goto("/d/settings/security");
    const section = page.getByRole("region", { name: "Two-factor authentication" });
    await section.getByRole("button", { name: "Add passkey" }).click();
    const naming = page.getByRole("dialog", { name: "Add a passkey" });
    await naming.getByLabel("Name").fill("Work laptop");
    await naming.getByRole("button", { name: "Continue" }).click();

    const saved = page.getByRole("dialog", { name: "Save your recovery codes" });
    await expect(saved.getByRole("list", { name: "Recovery codes" }).getByRole("listitem")).toHaveCount(10);
    await saved.getByRole("button", { name: "I saved them" }).click();
    await expect(section.getByText("Work laptop")).toBeVisible();
    await expect(section.getByText(/never used/)).toBeVisible();
    await expect(section.getByRole("button", { name: "Turn off" })).toBeVisible();

    await page.goto("/d");
    await signOut(page, person);
    await answerPassword(page, person);
    await expect(page).toHaveURL(/\/login\/two-factor/);
    await expect(page.getByRole("heading", { name: "Use your passkey" })).toBeVisible();
    await page.getByRole("button", { name: "Continue with passkey" }).click();
    await expect(page).toHaveURL(inbox);

    await page.goto("/d/settings/security");
    await expect(section.getByText(/last used/)).toBeVisible();
    await section.getByRole("button", { name: "Rename" }).click();
    const renaming = page.getByRole("dialog", { name: "Rename passkey" });
    await renaming.getByLabel("Name").fill("Home desktop");
    await renaming.getByRole("button", { name: "Save" }).click();
    await expect(section.getByText("Home desktop")).toBeVisible();

    await section.getByRole("button", { name: "Remove" }).click();
    const removal = page.getByRole("alertdialog", { name: "Remove this passkey?" });
    await expect(removal).toContainText("It is your last second factor");
    await removal.getByRole("button", { name: "Remove" }).click();
    await expect(section.getByText("Home desktop")).toBeHidden();
    await expect(section.getByRole("button", { name: "Turn off" })).toBeHidden();
  });
});

/*
 * addVirtualAuthenticator gives the page Chromium's virtual platform
 * authenticator, which makes and signs passkeys without a prompt, as a
 * device with a fingerprint reader would after the person touched it.
 */
async function addVirtualAuthenticator(page: Page): Promise<void> {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: {
      protocol: "ctap2",
      transport: "internal",
      hasResidentKey: true,
      hasUserVerification: true,
      isUserVerified: true,
      automaticPresenceSimulation: true,
    },
  });
}

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
