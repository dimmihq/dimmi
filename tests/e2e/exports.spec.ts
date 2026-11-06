import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

it.describe("data exports", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Exports ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("exports a view's conversations and downloads the file from Exports", async ({ page }) => {
    await startConversation(page, {
      name: "Ana Lima",
      email: "ana@lima.example",
      subject: "Order missing",
      body: "Where is my order?",
    });

    await page.goto("/d/inbox/all");
    await page.getByRole("button", { name: "Export conversations" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("radio", { name: "Spreadsheet" }).click();
    await dialog.getByRole("button", { name: "Export", exact: true }).click();
    await expect(dialog.getByText("Export started")).toBeVisible();
    await dialog.getByRole("link", { name: "Go to Exports" }).click();

    await expect(page).toHaveURL(/\/d\/[^/]+\/settings\/exports$/);
    const download = page.getByRole("button", { name: "Download" });
    await expect(download).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText(/Spreadsheet of conversations, 1 row/)).toBeVisible();
    const [file] = await Promise.all([page.waitForEvent("download"), download.click()]);
    expect(file.suggestedFilename()).toBe("conversations.csv");

    await page.getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(/No exports yet/)).toBeVisible();
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
