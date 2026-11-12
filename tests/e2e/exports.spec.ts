import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

it.describe("data exports", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Exports ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("exports the people directory", async ({ page }) => {
    await startConversation(page, {
      name: "Bo Chen",
      email: "bo@chen.example",
      subject: "Hello",
      body: "Hi there",
    });

    await page.goto("/d/customers");
    await page.getByRole("button", { name: "Export", exact: true }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Export", exact: true }).click();
    await expect(page.getByRole("dialog").getByText("Export started")).toBeVisible();
    await page.getByRole("dialog").getByRole("button", { name: "Close", exact: true }).last().click();

    await page.goto("/d/settings/exports");
    await expect(page.getByText(/Spreadsheet of customers, 1 row/)).toBeVisible({ timeout: 15_000 });
  });
});
