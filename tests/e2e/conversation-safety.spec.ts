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

  it("blocks listed domains and asks before opening a suspicious link", async ({ page }) => {
    await page.goto("/d/settings/conversation-safety");
    await page.getByRole("textbox", { name: "Blocked domains" }).fill("Evil.example");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("button", { name: "Save", exact: true })).toBeHidden();
    await expect(page.getByRole("textbox", { name: "Blocked domains" })).toHaveValue("evil.example");

    const id = await startConversation(page, {
      name: "Bo Chen",
      email: "bo@chen.example",
      subject: "Links",
      body: "[Sign in](https://login.evil.example/x), [bank.example](https://other.example/y) and https://bit.ly/abc",
    });
    await page.goto(`/d/inbox/unassigned/${id}`);
    const message = page.getByRole("article", { name: "Message from Bo Chen" });
    await expect(message.getByLabel("Blocked link to login.evil.example")).toBeVisible();
    await expect(message.getByRole("link", { name: "Sign in" })).toHaveCount(0);

    await message.getByRole("link", { name: /bank\.example/ }).click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toContainText("Opens other.example. Continue?");
    await expect(dialog).toContainText("Its text names a different site.");
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();

    await message.getByRole("link", { name: /bit\.ly/ }).click();
    await expect(page.getByRole("alertdialog")).toContainText("link shortener");
  });

  it("sets retention after saying how many conversations it would move", async ({ page }) => {
    await page.goto("/d/settings/conversation-safety");
    const retention = page.getByRole("region", { name: "Retention" });
    await expect(retention.getByRole("textbox", { name: "Trash" })).toHaveValue("30");

    await retention.getByRole("textbox", { name: "Closed conversations" }).fill("365");
    await retention.getByRole("textbox", { name: "Data exports" }).fill("14");
    await retention.getByRole("button", { name: "Save retention" }).click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toContainText("0 closed conversations are past this period");
    await dialog.getByRole("button", { name: "Save retention" }).click();
    await expect(dialog).toBeHidden();
    await page.reload();
    await expect(page.getByRole("textbox", { name: "Closed conversations" })).toHaveValue("365");
    await expect(page.getByRole("textbox", { name: "Data exports" })).toHaveValue("14");
  });
});
