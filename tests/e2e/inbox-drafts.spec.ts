/*
 * Drafts follow the person: a half written reply survives leaving the
 * conversation, marked on its row, and reloading, shows on another device, stays apart from
 * the note, and is gone once sent.
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
  return `Drafts ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

it.describe("drafts", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("keeps a reply across leaving, reloading and another device, and clears it once sent", async ({
    page,
    person,
    guest,
  }) => {
    const id = await startConversation(page, emily);
    const address = `/d/inbox/unassigned/${id}`;
    const reply = page.getByRole("textbox", { name: "Reply to Emily Johnson" });
    const saved = page.waitForResponse((r) => r.url().includes("/api/v1/drafts") && r.request().method() === "POST");

    await page.goto(address);
    await openComposer(page, /Reply to Emily Johnson/);
    await reply.fill("Hi Emily, resetting it now");
    await saved;

    await page.goto("/d/inbox/all");
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toContainText("Draft");
    await page.goto(address);
    await openComposer(page, /Reply to Emily Johnson/);
    await expect(reply).toHaveText("Hi Emily, resetting it now");

    await page.reload();
    await openComposer(page, /Reply to Emily Johnson/);
    await expect(reply).toHaveText("Hi Emily, resetting it now");

    await signIn(guest, person);
    await guest.goto(address);
    await openComposer(guest, /Reply to Emily Johnson/);
    await expect(guest.getByRole("textbox", { name: "Reply to Emily Johnson" })).toHaveText(
      "Hi Emily, resetting it now",
    );

    await page.getByRole("button", { name: "Send", exact: true }).click();
    await expect(page.getByText("Hi Emily, resetting it now")).toBeVisible();
    await expect(reply).toHaveCount(0);
    await openComposer(page, /Reply to Emily Johnson/);
    await expect(reply).toHaveText("");
    await page.reload();
    await openComposer(page, /Reply to Emily Johnson/);
    await expect(reply).toHaveText("");
  });

  it("keeps the reply and the note apart", async ({ page }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    const saved = page.waitForResponse(
      (r) => r.url().includes("/api/v1/drafts") && r.request().postDataJSON()?.mode === "note",
    );

    await openComposer(page, /Reply to Emily Johnson/);
    await page.getByRole("textbox", { name: "Reply to Emily Johnson" }).fill("For Emily");
    await page.getByRole("radio", { name: "Note" }).click();
    await page.getByRole("textbox", { name: "Internal note" }).fill("For the team");
    await saved;

    await page.reload();
    await openComposer(page, /^Note/);
    await expect(page.getByRole("textbox", { name: "Internal note" })).toHaveText("For the team");
    await page.getByRole("radio", { name: "Reply" }).click();
    await expect(page.getByRole("textbox", { name: "Reply to Emily Johnson" })).toHaveText("For Emily");
  });
});
