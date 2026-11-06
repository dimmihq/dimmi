import type { Locator, Page } from "@playwright/test";

import { ageSessions, verifyEmail } from "./support/database";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { mailedCode } from "./support/mail";

/*
 * signInByMail signs the person in with the code mailed to them. Their
 * address must already be proven, or the code claims the account afresh.
 */
async function signInByMail(page: Page, email: string) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(email);
  await page.getByRole("button", { name: "Email me a sign-in link" }).click();
  await page.getByLabel("Digit 1 of 6").pressSequentially(await mailedCode(email, "sign-in link"));
  await expect(page).toHaveURL(inbox);
}

/*
 * toggle clicks the box drawn for a checkbox, whose input is hidden from
 * sight and kept only for assistive technology.
 */
async function toggle(checkbox: Locator) {
  await checkbox.locator("xpath=ancestor::label[1]").click();
}

/*
 * addAddress adds an entry to the allowlist section through its dialog.
 */
async function addAddress(page: Page, list: Locator, cidr: string, label: string) {
  await list.getByRole("button", { name: "Add address" }).click();
  const dialog = page.getByRole("alertdialog");
  await dialog.getByLabel("Address or range").fill(cidr);
  await dialog.getByLabel("Label").fill(label);
  await dialog.getByRole("button", { name: "Add address" }).click();
}

it.describe("workspace security", () => {
  it("an admin sets a session limit and an old session is asked to sign in again", async ({
    page,
    person,
    request,
  }) => {
    await createAccount(request, person, `Security ${Date.now()}${Math.random().toString(36).slice(2, 6)}`);
    await signIn(page, person);

    await page.goto("/d/settings/workspace-security");
    const length = page.getByRole("region", { name: "Session length" });
    await length.getByRole("button", { name: "Sign in again after" }).click();
    await page.getByRole("option", { name: "1 hour" }).click();
    await expect(length.getByRole("button", { name: "Sign in again after" })).toContainText("1 hour");

    ageSessions(person.email, 90);
    await page.goto("/d/settings/workspace");

    const required = page.getByRole("alert").filter({ hasText: "Sign in again to continue" });
    await expect(required).toBeVisible();
    await required.getByRole("button", { name: "Sign in again" }).click();

    await expect(page.getByRole("heading", { name: "Sign in again" })).toBeVisible();
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();

    await expect(page).toHaveURL(/\/d\/[^/]+\/settings\/workspace$/);
    await expect(page.getByText("Sign in again to continue")).toHaveCount(0);
  });

  it("an admin allows only mailed links and a password session is asked to sign in with one", async ({
    page,
    guest,
    person,
    request,
  }) => {
    await createAccount(request, person, `Security ${Date.now()}${Math.random().toString(36).slice(2, 6)}`);
    verifyEmail(person.email);
    await signIn(guest, person);
    await signInByMail(page, person.email);

    await page.goto("/d/settings/workspace-security");
    const methods = page.getByRole("region", { name: "Sign-in methods" });
    await toggle(methods.getByRole("checkbox", { name: "Allow Password" }));
    await expect(methods.getByRole("checkbox", { name: "Allow Password" })).not.toBeChecked();
    await expect(methods.getByRole("checkbox", { name: "Allow Email link or code" })).toBeChecked();

    await guest.goto("/d/settings/workspace");
    const required = guest.getByRole("alert").filter({ hasText: "Sign in another way" });
    await expect(required).toBeVisible();
    await required.getByRole("button", { name: "Sign in again" }).click();
    await expect(guest.getByRole("heading", { name: "Sign in again" })).toBeVisible();
  });

  it("removing the way the admin signed in explains why it failed", async ({ page, person, request }) => {
    await createAccount(request, person, `Security ${Date.now()}${Math.random().toString(36).slice(2, 6)}`);
    await signIn(page, person);

    await page.goto("/d/settings/workspace-security");
    const methods = page.getByRole("region", { name: "Sign-in methods" });
    await toggle(methods.getByRole("checkbox", { name: "Allow Password" }));

    await expect(methods.getByRole("alert")).toBeVisible();
    await expect(methods.getByRole("checkbox", { name: "Allow Password" })).toBeChecked();
  });

  it("an admin adds their own address and a range", async ({ page, person, request }) => {
    await createAccount(request, person, `Security ${Date.now()}${Math.random().toString(36).slice(2, 6)}`);
    await signIn(page, person);

    await page.goto("/d/settings/workspace-security");
    const list = page.getByRole("region", { name: "Dashboard IP allowlist" });
    await expect(list.getByText("Your address: 127.0.0.1.")).toBeVisible();

    await addAddress(page, list, "127.0.0.1", "This laptop");
    await expect(list.getByText("127.0.0.1/32")).toBeVisible();
    await addAddress(page, list, "203.0.113.0/24", "Office");
    await expect(list.getByText("203.0.113.0/24")).toBeVisible();

    await page.reload();
    await expect(list.getByText("This laptop")).toBeVisible();
    await expect(list.getByText("Office")).toBeVisible();
  });

  it("saving a list without your address explains why it failed", async ({ page, person, request }) => {
    await createAccount(request, person, `Security ${Date.now()}${Math.random().toString(36).slice(2, 6)}`);
    await signIn(page, person);

    await page.goto("/d/settings/workspace-security");
    const list = page.getByRole("region", { name: "Dashboard IP allowlist" });
    await addAddress(page, list, "203.0.113.0/24", "Office");

    await expect(list.getByRole("alert")).toContainText("127.0.0.1");
    await expect(list.getByText("Any address may connect.")).toBeVisible();
  });
});
