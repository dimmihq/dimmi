import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("audiences", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Audiences ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("builds an audience, sees who it matches, saves it and tests a person", async ({ page }) => {
    await api(page, "POST", "/contacts", {
      name: "Ana Lima",
      email: "ana@lima.example",
      custom_attributes: { plan: "pro" },
    });
    await api(page, "POST", "/contacts", {
      name: "Bo Chen",
      email: "bo@chen.example",
      custom_attributes: { plan: "free" },
    });

    await page.goto("/d/customers/audiences");
    await expect(page.getByText("No audiences yet")).toBeVisible();
    await page.getByRole("button", { name: "New audience" }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/customers\/audiences\/audn_[^/]+$/);

    const matches = page.getByRole("region", { name: "Right now" });
    await expect(matches).toContainText("2 people");
    await page.getByRole("textbox", { name: "Name" }).fill("Pro customers");
    await page.getByRole("button", { name: "Add condition" }).click();
    await page.getByRole("button", { name: "Field" }).click();
    await page.getByRole("option", { name: "Plan", exact: true }).click();
    await page.getByRole("textbox", { name: "Value" }).fill("PRO");
    await expect(matches).toContainText("1 person");
    await expect(matches.getByRole("link", { name: "Ana Lima" })).toBeVisible();

    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("button", { name: "Save" })).toBeDisabled();
    await expect(page.getByText("Saved", { exact: true })).toBeVisible();

    const tester = page.getByRole("region", { name: "Test a person" });
    await tester.getByRole("button", { name: "Person to test" }).click();
    await page.getByRole("option", { name: "Bo Chen" }).click();
    await tester.getByRole("button", { name: "Test", exact: true }).click();
    await expect(tester.getByText("Does not match")).toBeVisible();
    await expect(tester.getByText("Has free")).toBeVisible();

    await page.getByRole("link", { name: "Back to audiences" }).click();
    await expect(page.getByRole("link", { name: /Pro customers/ })).toContainText("Everyone where Plan is PRO");
  });
});
