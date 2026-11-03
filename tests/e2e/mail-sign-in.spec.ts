import { createAccount, expect, test as it } from "./support/fixtures";
import { mailedCode, mailedLink } from "./support/mail";

it.describe("signing in by mail", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await createAccount(request, person);
    await page.goto("/login");
    await page.getByLabel("Email").fill(person.email);
    await page.getByRole("button", { name: "Email me a sign-in link" }).click();
    await expect(page.getByRole("heading", { name: "Check your inbox" })).toBeVisible();
  });

  it("signs in with the mailed code, and shows a wrong one under the boxes", async ({ page, person }) => {
    await page.getByLabel("Digit 1 of 6").pressSequentially("000000");
    await expect(page.getByText("The code is invalid or has expired.")).toBeVisible();
    await expect(page.getByLabel("Digit 1 of 6")).toHaveValue("");

    await page.getByLabel("Digit 1 of 6").click();
    await page.getByLabel("Digit 1 of 6").pressSequentially(await mailedCode(person.email, "sign-in link"));

    await expect(page).toHaveURL(/\/d$/);
  });

  it("signs in with the mailed link once, and explains a second use", async ({ page, person }) => {
    const link = await mailedLink(person.email, "sign-in link");

    await page.goto(link);
    await expect(page).toHaveURL(/\/d$/);

    await page.goto(link);
    await expect(page.getByRole("heading", { name: "This sign-in link no longer works" })).toBeVisible();
  });
});
