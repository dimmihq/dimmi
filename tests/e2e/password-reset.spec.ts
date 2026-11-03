import { createAccount, expect, test as it } from "./support/fixtures";
import { mailedLink } from "./support/mail";

it.describe("resetting a password", () => {
  it("mails a link, sets the new password from it and signs in", async ({ page, person, request }) => {
    await createAccount(request, person);

    await page.goto("/login");
    await page.getByRole("link", { name: "Forgot password?" }).click();
    await page.getByLabel("Email").fill(person.email);
    await page.getByRole("button", { name: "Send reset link" }).click();
    await expect(page.getByRole("heading", { name: "Check your inbox" })).toBeVisible();
    await expect(page.getByRole("status")).toContainText(person.email);

    await page.goto(await mailedLink(person.email, "Reset"));
    await page.getByLabel("New password").fill("difference-engine-1822");
    await page.getByRole("button", { name: "Save password and sign in" }).click();
    await expect(page).toHaveURL(/\/d$/);

    await page.getByRole("button", { name: "Sign out" }).click();
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page.getByText("These credentials do not match our records.")).toBeVisible();
  });

  it("explains a link that lost its secret", async ({ page }) => {
    await page.goto("/reset-password");

    await expect(page.getByRole("heading", { name: "This link is incomplete" })).toBeVisible();
  });
});
