import { expect, inbox, test as it } from "./support/fixtures";
import { mailedLink } from "./support/mail";

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
