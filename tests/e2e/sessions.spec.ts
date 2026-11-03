import type { Browser, Page } from "@playwright/test";

import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

/*
 * signInElsewhere signs the person in from a browser of its own, as
 * another computer would.
 */
async function signInElsewhere(browser: Browser, person: Person): Promise<Page> {
  const context = await browser.newContext();
  const page = await context.newPage();
  await signIn(page, person);

  return page;
}

/*
 * Signing up through the API signs that request in too, so the person
 * starts with a session of the sign-up besides the browsers here.
 */
it.describe("sessions", () => {
  it("lists where the person is signed in and signs other sessions out", async ({ browser, page, person, request }) => {
    await createAccount(request, person);
    await signIn(page, person);
    const laptop = await signInElsewhere(browser, person);
    const phone = await signInElsewhere(browser, person);

    await page.goto("/d/settings/security");
    const sessions = page.getByRole("region", { name: "Sessions" });
    await expect(sessions.getByText(/^This session/)).toHaveCount(1);
    await expect(sessions.getByRole("button", { name: "Revoke" })).toHaveCount(3);

    await sessions.getByRole("button", { name: "Revoke" }).first().click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Revoke" }).click();
    await expect(sessions.getByRole("button", { name: "Revoke" })).toHaveCount(2);

    await sessions.getByRole("button", { name: "Sign out others" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Sign out others" }).click();
    await expect(sessions.getByRole("button", { name: "Revoke" })).toHaveCount(0);
    await expect(sessions.getByText(/^This session/)).toBeVisible();

    for (const elsewhere of [laptop, phone]) {
      await elsewhere.goto("/d/settings/security");
      await expect(elsewhere).toHaveURL(/\/login/);
      await elsewhere.context().close();
    }
  });
});
