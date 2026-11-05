/*
 * Moving messages in the thread: a teammate edits their own note in
 * place, deletes it after confirming, and the thread keeps a line saying
 * who deleted it; a reply can be deleted too.
 */
import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, openComposer, startConversation } from "./support/inbox";

const emily: Customer = {
  name: "Emily Johnson",
  email: "emily@northwind.example",
  subject: "Can't log in",
  body: "My password manager's password is refused.",
};

function workspaceOf(person: Person): string {
  return `Moves ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

it.describe("message moves", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("edits a note in place, then deletes it to a line naming who did", async ({ page, person }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await openComposer(page, /^Note/);
    await page.getByRole("textbox", { name: "Internal note" }).fill("Card 4242 4242 4242 4242");
    await page.getByRole("button", { name: "Add note" }).click();
    const note = page.getByRole("article", { name: `Internal note from ${person.name}` });
    await expect(note).toContainText("Card 4242");

    await note.hover();
    await note.getByRole("button", { name: "Edit note" }).click();
    const editor = note.getByRole("textbox", { name: "Edit note" });
    await editor.fill("Card on file, ending 4242");
    await note.getByRole("button", { name: "Save" }).click();
    await expect(note).toContainText("Card on file, ending 4242");
    await expect(note).toContainText("Edited");

    await note.hover();
    await note.getByRole("button", { name: "Delete message" }).click();
    const dialog = page.getByRole("alertdialog", { name: "Delete this message?" });
    await expect(dialog).toContainText("Deleted for your team");
    await dialog.getByRole("button", { name: "Delete" }).click();

    await expect(page.getByText(`Message deleted by ${person.name}`)).toBeVisible();
    await expect(page.getByText("Card on file, ending 4242")).toHaveCount(0);
  });

  it("cancels an edit with Escape and deletes a reply", async ({ page, person }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await openComposer(page, /^Reply to /);
    await page.getByRole("textbox", { name: /^Reply to / }).fill("Try a sign-in code instead.");
    await page.getByRole("button", { name: "Send", exact: true }).click();
    const reply = page.getByRole("article", { name: `Message from ${person.name}` });
    await expect(reply).toContainText("Try a sign-in code instead.");
    await expect(reply.getByRole("button", { name: "Edit note" })).toHaveCount(0);

    await reply.hover();
    await reply.getByRole("button", { name: "Delete message" }).click();
    const dialog = page.getByRole("alertdialog", { name: "Delete this message?" });
    await expect(dialog).toContainText("Emily Johnson already received this reply");
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(reply).toContainText("Try a sign-in code instead.");

    await openComposer(page, /^Note/);
    await page.getByRole("textbox", { name: "Internal note" }).fill("Watch this one");
    await page.getByRole("button", { name: "Add note" }).click();
    const note = page.getByRole("article", { name: `Internal note from ${person.name}` });
    await note.hover();
    await note.getByRole("button", { name: "Edit note" }).click();
    await note.getByRole("textbox", { name: "Edit note" }).fill("Changed my mind");
    await page.keyboard.press("Escape");
    await expect(note).toContainText("Watch this one");
  });
});
