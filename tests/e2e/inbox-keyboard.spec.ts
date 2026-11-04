/*
 * The inbox by keyboard: J and K walk the list from an open conversation
 * without a spinner, Z snoozes and Cmd+Shift+Y closes, each moving on to
 * the next conversation, ? opens the sheet and the practice page teaches
 * the moves one at a time.
 */
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, startConversation } from "./support/inbox";

const customers: Customer[] = [
  { name: "Emily Johnson", email: "emily@northwind.example", subject: "Can't log in", body: "Refused." },
  { name: "Omar Haddad", email: "omar@contoso.example", subject: "Invoice question", body: "Wrong VAT." },
  { name: "Lena Park", email: "lena@fabrikam.example", subject: "Export fails", body: "CSV is empty." },
];

function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("the inbox by keyboard", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1600, height: 900 });
    await createAccount(request, person, `Keys ${suffixOf(person)}`);
    await signIn(page, person);
    for (const customer of customers) {
      await startConversation(page, customer);
    }
  });

  it("walks the list, snoozes and closes by keys", async ({ page }) => {
    await page.goto("/d/inbox/all");
    await page.getByRole("link", { name: /Lena Park/ }).click();
    await expect(page.getByRole("heading", { name: "Export fails", level: 1 })).toBeVisible();

    await page.keyboard.press("j");
    await expect(page.getByRole("heading", { name: "Invoice question", level: 1 })).toBeVisible();
    await expect(page.getByLabel("Loading the conversation")).toHaveCount(0);
    await page.keyboard.press("k");
    await expect(page.getByRole("heading", { name: "Export fails", level: 1 })).toBeVisible();

    await page.keyboard.press("z");
    await expect(page.getByRole("heading", { name: "Invoice question", level: 1 })).toBeVisible();
    await page.keyboard.press("ControlOrMeta+Shift+Y");
    await expect(page.getByRole("heading", { name: "Can't log in", level: 1 })).toBeVisible();

    await page.getByRole("radio", { name: "All" }).click();
    await expect(page.getByRole("link", { name: /Lena Park/ })).toContainText("Export fails");
    await page.getByRole("radio", { name: "Snoozed" }).click();
    await expect(page.getByRole("link", { name: /Lena Park/ })).toBeVisible();
    await page.getByRole("radio", { name: "Closed" }).click();
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toBeVisible();
  });

  it("opens the sheet with ? and practises the moves", async ({ page }) => {
    await page.goto("/d/inbox/all");
    await page.keyboard.press("Shift+?");
    const sheet = page.getByRole("dialog", { name: "Keyboard shortcuts" });
    await expect(sheet).toContainText("Next conversation");
    await sheet.getByRole("link", { name: "Practise the shortcuts" }).click();

    await expect(page.getByRole("status")).toContainText("Press J");
    await page.keyboard.press("j");
    await expect(page.getByRole("status")).toContainText("Press K");
  });
});
