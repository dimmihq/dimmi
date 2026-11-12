/*
 * The views of the inbox: a team's view lists what sits with the team,
 * moving a conversation to a team from its details moves it into that
 * view, the sort a person picks for a view survives a reload, and a
 * teammate sees from the list who has a conversation open.
 */
import { addTeammate } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, pickView, startConversation } from "./support/inbox";

const emily: Customer = {
  name: "Emily Johnson",
  email: "emily@northwind.example",
  subject: "Can't log in",
  body: "My password manager's password is refused.",
};

const omar: Customer = {
  name: "Omar Haddad",
  email: "omar@contoso.example",
  subject: "Invoice question",
  body: "The VAT number on my invoice is wrong.",
};

function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("inbox views", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Views ${suffixOf(person)}`);
  });

  it("moves a conversation into a team's view from its details", async ({ page, person }) => {
    await signIn(page, person);
    const created = await page.request.post("/api/v1/teams", { data: { name: "Billing" } });
    expect(created.status(), await created.text()).toBe(201);
    const team = ((await created.json()) as { data: { id: string } }).data.id;
    const id = await startConversation(page, omar);
    await startConversation(page, emily);

    await page.goto(`/d/inbox/${team}`);
    await expect(page.getByRole("heading", { name: "Billing" })).toBeVisible();
    await expect(page.getByText("No conversations")).toBeVisible();

    await page.goto(`/d/inbox/all/${id}`);
    await page.getByRole("region", { name: "Details" }).getByRole("button", { name: /Team/ }).click();
    await page.getByRole("option", { name: "Billing" }).click();
    await expect(page.getByRole("region", { name: "Details" }).getByRole("button", { name: /Team/ })).toContainText(
      "Billing",
    );

    await pickView(page, /Billing/);
    await expect(page).toHaveURL(new RegExp(`/d/[^/]+/inbox/${team}`));
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toBeVisible();
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toHaveCount(0);
  });

  it("keeps the chosen status and sort of a view after a reload", async ({ page, person }) => {
    await signIn(page, person);
    const closed = await startConversation(page, emily);
    const done = await page.request.patch(`/api/v1/conversations/${closed}`, { data: { status: "closed" } });
    expect(done.status(), await done.text()).toBe(200);
    await startConversation(page, omar);
    await page.goto("/d/inbox/all");
    await expect(page.locator('[data-realtime="connected"]')).toBeAttached();
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toBeVisible();

    await page.getByRole("button", { name: /Sort/ }).click();
    await page.getByRole("option", { name: "Newest started" }).click();
    await page.getByRole("radio", { name: "Closed" }).click();
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toBeVisible();
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toHaveCount(0);

    await expect
      .poll(async () => {
        const saved = await page.request.get("/api/v1/inbox-preferences");
        return ((await saved.json()) as { data: { views: Record<string, { sort?: string }> } }).data.views.all?.sort;
      })
      .toBe("-created_at");
    await page.goto("/d/inbox/all");
    await expect(page.getByRole("button", { name: /Sort/ })).toContainText("Newest started");
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toBeVisible();
  });

  it("shows on the list who has a conversation open", async ({ page, person, request, guest }) => {
    const teammate: Person = { ...person, name: "Ada Lovelace", email: person.email.replace("grace.", "ada.") };
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, person);
    const id = await startConversation(page, emily);

    await signIn(guest, teammate);
    await guest.goto(`/d/inbox/all/${id}`);
    await expect(guest.getByRole("heading", { name: "Can't log in" })).toBeVisible();

    await page.goto("/d/inbox/all");
    await expect(page.locator('[data-realtime="connected"]')).toBeAttached();
    await expect(page.getByRole("img", { name: "Viewing: Ada Lovelace" })).toBeVisible();

    await guest.goto("/d/inbox/all");
    await expect(page.getByRole("img", { name: "Viewing: Ada Lovelace" })).toHaveCount(0);
  });

  it("saves a view from the view picker, lists what matches and counts it once pinned", async ({ page, person }) => {
    await signIn(page, person);
    await startConversation(page, omar);
    await startConversation(page, emily);
    await page.reload();

    await pickView(page, "New view");
    const editor = page.getByRole("dialog", { name: "New view" });
    await editor.getByLabel("Name").fill("Invoices");
    await editor.getByRole("button", { name: /Field/ }).click();
    await page.getByRole("option", { name: "Subject" }).click();
    await editor.getByRole("textbox", { name: "Value" }).fill("Invoice");
    await editor.getByRole("button", { name: "Save view" }).click();

    await expect(page).toHaveURL(/\/d\/[^/]+\/inbox\/cvw_/);
    await expect(page.getByRole("heading", { name: "Invoices" })).toBeVisible();
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toBeVisible();
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toHaveCount(0);

    await page.getByRole("button", { name: "Pin view" }).click();
    await page.locator("button[data-view-picker]").click();
    await expect(page.getByRole("dialog", { name: "Views" }).getByRole("button", { name: /Invoices/ })).toContainText(
      "1",
    );
    await page.keyboard.press("Escape");

    await page.getByRole("button", { name: "Edit view" }).click();
    await page.getByRole("dialog", { name: "Edit view" }).getByLabel("Name").fill("Billing questions");
    await page.getByRole("dialog", { name: "Edit view" }).getByRole("button", { name: "Save view" }).click();
    await expect(page.getByRole("heading", { name: "Billing questions" })).toBeVisible();
  });

  it("applies conditions to the list without saving a view, and clears them", async ({ page, person }) => {
    await signIn(page, person);
    await startConversation(page, omar);
    await startConversation(page, emily);
    await page.goto("/d/inbox/all");

    await page.getByRole("button", { name: "Filter", exact: true }).click();
    const editor = page.getByRole("dialog", { name: "Filter conversations" });
    await editor.getByRole("button", { name: /Field/ }).click();
    await page.getByRole("option", { name: "Subject" }).click();
    await editor.getByRole("textbox", { name: "Value" }).fill("Invoice");
    await editor.getByRole("button", { name: "Apply" }).click();

    await expect(page).toHaveURL(/\/inbox\/all\?where=/);
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toBeVisible();
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toHaveCount(0);

    await page.getByRole("button", { name: "Clear filters" }).click();
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toBeVisible();
  });
});
