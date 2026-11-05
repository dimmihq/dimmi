import type { Page } from "@playwright/test";

import { createAccount, expect, inbox, test as it, type Person, signIn, signOut } from "./support/fixtures";
import { currentStep, totpCode } from "./support/totp";

/*
 * Authenticator is what turning two-factor on left the person with: the
 * setup key their app holds, the step its first code used, and the
 * recovery codes shown once.
 */
interface Authenticator {
  secret: string;
  activatedAt: number;
  codes: string[];
}

it.describe("two-factor authentication", () => {
  it.beforeEach(async ({ context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  });

  it("turns on with an authenticator app, then asks for a code or a recovery code at sign-in", async ({
    page,
    person,
    request,
  }) => {
    await createAccount(request, person, `${person.name}'s workspace`);
    await signIn(page, person);

    const { secret, activatedAt, codes } = await turnOn(page);
    const section = page.getByRole("region", { name: "Two-factor authentication" });
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

  it("keeps the setup key behind a link and copies it and the recovery codes", async ({ page, person, request }) => {
    await createAccount(request, person, `${person.name}'s workspace`);
    await signIn(page, person);
    await page.goto("/d/settings/security");
    await page
      .getByRole("region", { name: "Two-factor authentication" })
      .getByRole("button", { name: "Add authenticator" })
      .click();

    const setup = page.getByRole("dialog", { name: "Add an authenticator app" });
    await expect(setup.getByRole("img", { name: /QR code/ })).toBeVisible();
    await expect(setup.getByTestId("totp-secret")).toHaveCount(0);

    await setup.getByRole("button", { name: "Can't scan the code?" }).click();
    const key = setup.getByTestId("totp-secret");
    await expect(key).toHaveAttribute("readonly", "");
    const secret = await key.inputValue();
    expect(secret).toMatch(/^[A-Z2-7]+$/);

    await setup.getByRole("button", { name: "Copy setup key" }).click();
    await expect(setup.getByRole("button", { name: "Copied" })).toBeVisible();
    expect(await clipboard(page)).toBe(secret);
    await expect(setup.getByRole("button", { name: "Copy setup key" })).toBeVisible();

    await setup.getByLabel("Digit 1 of 6").pressSequentially(totpCode(secret, currentStep()));
    const saved = page.getByRole("dialog", { name: "Save your recovery codes" });
    const codes = await recoveryCodes(page);

    await saved.getByRole("button", { name: "Copy codes" }).click();
    await expect(saved.getByRole("button", { name: "Copied" })).toBeVisible();
    expect(await clipboard(page)).toBe(codes.join("\n"));
    await saved.getByRole("button", { name: "I saved them" }).click();
    await expect(saved).toBeHidden();
  });

  it("requires two-factor from everyone in the workspace once the owner has it", async ({ page, person, request }) => {
    await createAccount(request, person, `Secure ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    const rule = page.getByRole("switch", { name: "Require two-factor authentication" });

    await page.goto("/d/settings/workspace");
    await rule.click({ force: true });
    await expect(page.getByRole("alert")).toContainText("Turn on two-factor authentication for your own account");
    await expect(rule).not.toBeChecked();

    await turnOn(page);
    await page.goto("/d/settings/workspace");
    const saved = page.waitForResponse(
      (response) => response.url().endsWith("/api/v1/workspace") && response.request().method() === "PATCH",
    );
    await rule.click({ force: true });
    await expect(rule).toBeChecked();
    expect((await saved).status()).toBe(200);

    await page.reload();
    await expect(rule).toBeChecked();
    await page.goto("/d/settings/security");
    await page
      .getByRole("region", { name: "Two-factor authentication" })
      .getByRole("button", { name: "Turn off" })
      .click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Turn off" }).click();
    await expect(page.getByRole("alert")).toContainText("requires two-factor authentication");
  });
});

/*
 * turnOn adds an authenticator from the security page, scanning nothing:
 * it reads the setup key and answers with the code the app would show.
 */
async function turnOn(page: Page): Promise<Authenticator> {
  await page.goto("/d/settings/security");
  await page
    .getByRole("region", { name: "Two-factor authentication" })
    .getByRole("button", { name: "Add authenticator" })
    .click();

  const setup = page.getByRole("dialog", { name: "Add an authenticator app" });
  await setup.getByRole("button", { name: "Can't scan the code?" }).click();
  const secret = await setup.getByTestId("totp-secret").inputValue();

  const activatedAt = currentStep();
  await setup.getByLabel("Digit 1 of 6").pressSequentially(totpCode(secret, activatedAt));

  const codes = await recoveryCodes(page);
  await page
    .getByRole("dialog", { name: "Save your recovery codes" })
    .getByRole("button", { name: "I saved them" })
    .click();

  return { secret, activatedAt, codes };
}

/*
 * recoveryCodes reads the codes the dialog shows once.
 */
async function recoveryCodes(page: Page): Promise<string[]> {
  const items = page
    .getByRole("dialog", { name: "Save your recovery codes" })
    .getByRole("list", { name: "Recovery codes" })
    .getByRole("listitem");
  await expect(items).toHaveCount(10);

  return items.allTextContents();
}

/*
 * clipboard returns what the page last copied.
 */
function clipboard(page: Page): Promise<string> {
  return page.evaluate(() => navigator.clipboard.readText());
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
