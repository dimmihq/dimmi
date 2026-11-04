/*
 * The command menu: Cmd+K from anywhere, find a customer by email, open
 * a conversation by its number, and run a move on the open one.
 */
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("the command menu", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1600, height: 900 });
    await createAccount(request, person, `Menu ${suffixOf(person)}`);
    await signIn(page, person);
    await startConversation(page, {
      name: "Emily Johnson",
      email: "emily@northwind.example",
      subject: "Can't log in",
      body: "My password manager's password is refused.",
    });
  });

  it("finds a customer by email and opens a conversation by number", async ({ page }) => {
    await page.goto("/d/inbox/all");
    await page.keyboard.press("ControlOrMeta+k");
    const menu = page.getByRole("dialog", { name: "Command menu" });
    const search = menu.getByRole("searchbox", { name: "Search for a command or conversation" });

    await search.fill("emily@northwind.example");
    await expect(menu.getByRole("menuitem", { name: /Emily Johnson/ }).first()).toBeVisible();

    await search.fill("#1");
    await menu.getByRole("menuitem", { name: /#1/ }).first().click();
    await expect(page).toHaveURL(/\/d\/inbox\/all\/conv_/);
    await expect(page.getByRole("heading", { name: "Can't log in", level: 1 })).toBeVisible();
  });

  it("runs a move on the open conversation", async ({ page }) => {
    await page.goto("/d/inbox/all");
    await page.getByRole("link", { name: /Emily Johnson/ }).click();
    await expect(page.getByRole("heading", { name: "Can't log in", level: 1 })).toBeVisible();

    await page.keyboard.press("ControlOrMeta+k");
    const menu = page.getByRole("dialog", { name: "Command menu" });
    await menu.getByRole("searchbox").fill("done");
    await menu.getByRole("menuitem", { name: /Mark as done/ }).click();

    await expect(page.getByText(/#1 · Done/)).toBeVisible();
  });
});
