import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("audiences", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Audiences ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("makes an audience with filters, tests a person, and narrows people by it", async ({ page }) => {
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
    const modal = page.getByRole("dialog", { name: "New audience" });
    await expect(modal.getByText("Matches 2 people")).toBeVisible();
    await modal.getByRole("textbox", { name: "Name" }).fill("Pro customers");
    const group = modal.getByRole("region", { name: "Group 1" });
    await group.getByRole("button", { name: /Field/ }).click();
    await expect(page.getByRole("option", { name: "Plan Text", exact: true })).toBeAttached();
    await page.keyboard.type("Plan");
    await page.keyboard.press("Enter");
    await expect(group.getByRole("button", { name: /^is Comparison/ })).toBeVisible();
    await group.getByRole("textbox", { name: "Value" }).fill("PRO");
    await expect(modal.getByText("Matches 1 person")).toBeVisible();

    await modal.getByRole("button", { name: "Add group" }).click();
    await expect(modal.getByRole("region", { name: "Group 2" })).toBeVisible();
    await modal.getByRole("button", { name: "Remove group 2" }).click();
    await expect(modal.getByRole("region", { name: "Group 2" })).toBeHidden();
    await modal.getByRole("button", { name: "Create audience" }).click();
    await expect(modal).toBeHidden();

    const row = page.getByRole("row", { name: /Pro customers/ });
    await expect(row).toContainText("Everyone where Plan is PRO");
    await expect(row).toContainText("1 person");

    await row.getByRole("button", { name: "Actions for Pro customers" }).click();
    await page.getByRole("menuitem", { name: "Test a person" }).or(page.getByText("Test a person")).first().click();
    const tester = page.getByRole("dialog", { name: "Test Pro customers" });
    await tester.getByRole("textbox", { name: "Person" }).fill("Bo");
    await tester.getByRole("button", { name: "Person to test" }).click();
    await page.getByRole("option", { name: "Bo Chen" }).click();
    await tester.getByRole("button", { name: "Test", exact: true }).click();
    await expect(tester.getByText("Does not match")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(tester).toBeHidden();

    await page.goto("/d/customers");
    await page.getByRole("button", { name: "Add filter" }).click();
    const panel = page.getByRole("dialog", { name: "Add filter" });
    await panel.getByRole("button", { name: /^Audience/ }).click();
    await panel.getByRole("button", { name: "is in any of" }).click();
    await panel.getByText("Pro customers").click();
    await panel.getByRole("button", { name: "Apply" }).click();
    const people = page.getByRole("grid", { name: "People" });
    await expect(people.getByText("Ana Lima")).toBeVisible();
    await expect(people.getByText("Bo Chen")).toBeHidden();
  });
});
