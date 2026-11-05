/*
 * Mentions and reactions between two teammates: Grace types @ in a note,
 * picks Ada from the suggestions and adds the note; Ada's Mentions count
 * goes up in her own browser, the conversation is in her Mentions view,
 * and the note shows Ada as a chip. Ada reacts to the note and Grace sees
 * the reaction.
 */
import { addTeammate } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, openComposer, startConversation } from "./support/inbox";

const omar: Customer = {
  name: "Omar Said",
  email: "omar@northwind.example",
  subject: "Refund twice?",
  body: "I think I was refunded twice.",
};

function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

it.describe("mentions", () => {
  it("mentions a teammate in a note, who sees it counted and reacts", async ({ page, person, request, guest }) => {
    const ada = teammateOf(person);
    await page.setViewportSize({ width: 1440, height: 900 });
    await guest.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Mentions ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await createAccount(request, ada);
    addTeammate(person.email, ada.email, "member");
    await signIn(page, person);
    const id = await startConversation(page, omar);

    await signIn(guest, ada);
    await expect(guest.locator('[data-realtime="connected"]')).toBeAttached();
    const mentions = guest.getByRole("link", { name: /^Mentions/ });

    await page.goto(`/d/inbox/unassigned/${id}`);
    await openComposer(page, /^Note/);
    const field = page.getByRole("textbox", { name: "Internal note" });
    await field.pressSequentially("Can you check @Ada");
    const suggestions = page.getByRole("listbox", { name: "Mention" });
    await expect(suggestions).toContainText("@Ada Lovelace");
    await page.keyboard.press("Enter");
    await field.pressSequentially("please");
    await page.getByRole("button", { name: "Add note" }).click();

    const note = page.getByRole("article", { name: `Internal note from ${person.name}` });
    await expect(note).toContainText("Can you check @Ada Lovelace please");

    await expect(mentions).toContainText("1");
    await mentions.click();
    await expect(guest.getByRole("radio", { name: "Unread" })).toBeChecked();
    await guest.getByRole("link", { name: /Omar Said/ }).click();
    const seen = guest.getByRole("article", { name: `Internal note from ${person.name}` });
    await expect(seen).toContainText("@Ada Lovelace");

    await seen.hover();
    await seen.getByRole("button", { name: "Add reaction" }).click();
    await guest.getByRole("button", { name: "React with 🎉" }).click();
    await expect(seen.getByRole("button", { name: "🎉 1" })).toHaveAttribute("aria-pressed", "true");

    await page.reload();
    await expect(note.getByRole("button", { name: "🎉 1" })).toHaveAttribute("aria-pressed", "false");
  });
});
