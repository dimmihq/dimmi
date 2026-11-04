import type { Page } from "@playwright/test";

import { expect, inbox, test as it, signIn } from "./support/fixtures";
import { mailedLink } from "./support/mail";

/*
 * ownAddress has the page's requests come from an address of the test's
 * own, since redeeming mailed links is limited per address and these tests
 * redeem several.
 */
async function ownAddress(pages: Page[]) {
  const octet = () => 1 + Math.floor(Math.random() * 250);
  const address = `198.51.${octet()}.${octet()}`;
  for (const page of pages) {
    await page.context().setExtraHTTPHeaders({ "X-Forwarded-For": address });
  }
}

it.describe("signing up", () => {
  it("creates an account, lands in the app and proves the email from the mailed link", async ({ page, person }) => {
    await page.goto("/signup");
    await page.getByLabel("Full name").fill(person.name);
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Create account" }).click();

    await expect(page).toHaveURL(inbox);
    await expect(page.getByRole("button", { name: person.name, exact: true })).toBeVisible();

    await page.goto(await mailedLink(person.email, "Verify"));
    await expect(page.getByRole("heading", { name: "Email verified" })).toBeVisible();

    await page.getByRole("link", { name: "Continue to Dimmi" }).click();
    await expect(page).toHaveURL(inbox);
  });

  it("verifies nothing on another device until the person signs in there", async ({ page, guest, person }) => {
    await ownAddress([page, guest]);
    await page.goto("/signup");
    await page.getByLabel("Full name").fill(person.name);
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page).toHaveURL(inbox);
    const link = await mailedLink(person.email, "Verify");

    await guest.goto(link);
    await expect(guest.getByRole("heading", { name: "Did you create this account?" })).toBeVisible();

    await signIn(guest, person);
    await guest.goto(link);
    await expect(guest.getByRole("heading", { name: "Email verified" })).toBeVisible();

    await page.reload();
    await expect(page).toHaveURL(inbox);
  });

  it("takes the account back from a stranger who signed up with the address", async ({ page, guest, person }) => {
    await ownAddress([page, guest]);
    await page.goto("/signup");
    await page.getByLabel("Full name").fill(person.name);
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page).toHaveURL(inbox);

    await guest.goto(await mailedLink(person.email, "Verify"));
    await guest.getByRole("button", { name: "I did not create this account" }).click();
    await expect(guest.getByRole("heading", { name: "This address is yours" })).toBeVisible();

    await page.reload();
    await expect(page).toHaveURL(/\/login/);
  });

  it("shows why a password is refused under the password, and accepts a better one", async ({ page, person }) => {
    await page.goto("/signup");
    await page.getByLabel("Full name").fill(person.name);
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill("password123");
    await page.getByRole("button", { name: "Create account" }).click();

    await expect(page.getByText("The password is too common. Choose another one.")).toBeVisible();

    await page.getByLabel("Password").fill(person.password);
    await expect(page.getByText("The password is too common. Choose another one.")).toBeHidden();
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page).toHaveURL(inbox);
  });
});
