/*
 * Emoji in the composer: a colon and a few letters suggest emoji, and
 * the add menu has an emoji picker.
 */
import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, openComposer, startConversation } from "./support/inbox";

const lena: Customer = {
  name: "Lena Fischer",
  email: "lena@northwind.example",
  subject: "Thanks!",
  body: "It works now, thank you.",
};

function workspaceOf(person: Person): string {
  return `Emoji ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

it.describe("emoji", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("suggests emoji while typing and picks them from the add menu", async ({ page }) => {
    const id = await startConversation(page, lena);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await openComposer(page, /^Note/);
    const field = page.getByRole("textbox", { name: "Internal note" });
    await field.pressSequentially("Great :thumbs");
    await expect(page.getByRole("listbox", { name: "Emoji" })).toContainText("thumbs up");
    await page.keyboard.press("Enter");
    await expect(field).toContainText("Great 👍");

    await page.getByRole("button", { name: "Add to message" }).click();
    await page.getByRole("button", { name: /Add an emoji$/ }).click();
    await page.getByRole("textbox", { name: "Search emoji" }).fill("party popper");
    await page.getByRole("button", { name: "party popper" }).click();
    await expect(field).toContainText("🎉");
  });
});
