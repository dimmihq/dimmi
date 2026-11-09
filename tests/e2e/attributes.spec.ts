import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("attributes", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Attributes ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("lists attributes seen in data, adds a list, and archives and deletes one", async ({ page }) => {
    await page.goto("/d/customers");
    await api(page, "POST", "/contacts", { email: "ana@lima.example", custom_attributes: { seats: 3 } });

    await page.goto("/d/settings/attributes");
    const people = page.getByRole("region", { name: "People" });
    await expect(people).toContainText("seats · Number");

    await page.getByRole("button", { name: "New attribute" }).click();
    const add = page.getByRole("dialog", { name: "New attribute" });
    await add.getByRole("textbox", { name: "Key" }).fill("tier");
    await add.getByRole("button", { name: /Type/ }).click();
    await page.getByRole("option", { name: "List" }).click();
    await add.getByRole("textbox", { name: "Values" }).fill("gold, silver");
    await add.getByRole("button", { name: "Add attribute" }).click();
    await expect(add).toBeHidden();
    await expect(people).toContainText("tier · List · gold, silver");

    await people.getByRole("button", { name: "Archive" }).first().click();
    await expect(people).toContainText("archived");
    await people.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("button", { name: "Delete attribute" }).click();
    await expect(people).not.toContainText("seats · Number");
  });
});
