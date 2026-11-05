import { expect, test as it } from "./support/fixtures";

it.describe("the password page of a private deployment", () => {
  it("asks for the shared password and says when it was wrong", async ({ page }) => {
    await page.goto("/__gate?next=%2Fd");

    await expect(page.getByRole("heading", { name: "This site is private" })).toBeVisible();
    await expect(page.getByLabel("Password")).toBeFocused();
    await expect(page.getByRole("button", { name: "Continue" })).toBeVisible();
    await expect(page.getByText("That password is not right.")).toHaveCount(0);

    await page.goto("/__gate?failed=1&next=%2Fd");

    await expect(page.getByText("That password is not right.")).toBeVisible();
  });
});
