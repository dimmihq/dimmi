import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

it.describe("conversation safety", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Safety ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("adds a rule, tries it, and shows what a customer's message lost", async ({ page }) => {
    await page.goto("/d/settings/conversation-safety");
    await expect(page.getByRole("switch", { name: "Card numbers" })).toBeChecked();
    await expect(page.getByRole("switch", { name: "US social security numbers" })).not.toBeChecked();

    await page.getByRole("button", { name: "New rule" }).click();
    await page.getByRole("textbox", { name: "Name" }).fill("Account ids");
    await page.getByRole("textbox", { name: "Pattern" }).fill("(");
    await page.getByRole("button", { name: "Add rule" }).click();
    await expect(page.getByRole("alertdialog")).toContainText("valid expression");
    await page.getByRole("textbox", { name: "Pattern" }).fill("ACCT-\\d+");
    await page.getByRole("button", { name: "Add rule" }).click();
    await expect(page.getByRole("alertdialog")).toBeHidden();
    await expect(page.getByRole("switch", { name: "Use Account ids" })).toBeChecked();

    await page.getByRole("textbox", { name: "Text to try" }).fill("Card 4242 4242 4242 4242 for ACCT-77");
    await page.getByRole("button", { name: "Try", exact: true }).click();
    const stored = page.getByRole("status", { name: "Stored as" });
    await expect(stored).toContainText("2 items redacted");
    await expect(stored).toContainText("Card •••• •••• •••• 4242 for [redacted]");

    const id = await startConversation(page, {
      name: "Ana Lima",
      email: "ana@lima.example",
      subject: "Billing",
      body: "My card is 4242 4242 4242 4242",
    });
    await page.goto(`/d/inbox/unassigned/${id}`);
    const message = page.getByRole("article", { name: "Message from Ana Lima" });
    await expect(message).toContainText("My card is •••• •••• •••• 4242");
    await expect(message).toContainText("1 item redacted");
  });
});
