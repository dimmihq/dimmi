import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

it.describe("customers", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Customers ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("searches people, edits one, and opens their conversation", async ({ page }) => {
    await startConversation(page, {
      name: "Ana Lima",
      email: "ana@lima.example",
      subject: "Invoice VAT",
      body: "Where is my invoice?",
    });
    await startConversation(page, { name: "Bo Chen", email: "bo@chen.example", subject: "Hi", body: "Hello" });

    await page.goto("/d/customers");
    const people = page.getByRole("grid", { name: "People" });
    await expect(people.getByText("Bo Chen")).toBeVisible();
    await expect(page.getByText("2 people")).toBeVisible();
    await page.getByRole("textbox", { name: "Search people" }).fill("ana");
    await expect(people.getByText("Bo Chen")).toBeHidden();
    await people.getByRole("row", { name: /Ana Lima/ }).click();

    const edit = page.getByRole("dialog", { name: "Edit Ana Lima" });
    await expect(page).toHaveURL(/\/customers\?contact=ctc_/);
    await edit.getByRole("textbox", { name: "Phone" }).fill("+15550101");
    const find = page.getByRole("textbox", { name: "Find or create a tag" });
    await expect(async () => {
      await edit.getByRole("button", { name: "Add tag" }).click();
      await expect(find).toBeVisible({ timeout: 1000 });
    }).toPass();
    await find.fill("vip");
    await expect(page.getByText('Create "vip"')).toBeVisible();
    await find.press("Enter");
    await expect(edit.getByText("vip", { exact: true })).toBeVisible();
    await edit.getByRole("heading", { name: "Edit Ana Lima" }).click();
    await edit.getByRole("button", { name: "Save" }).click();
    await expect(edit).toBeHidden();
    await expect(page).toHaveURL(/\/customers$/);
    await expect(people.getByRole("row", { name: /Ana Lima/ }).getByText("vip")).toBeVisible();
  });

  it("adds a contact and a company, and keeps blocked people apart", async ({ page }) => {
    await page.goto("/d/customers");
    await page.getByRole("button", { name: "New contact" }).click();
    await page.getByRole("textbox", { name: "Name" }).fill("Cy Diaz");
    await page.getByRole("button", { name: "Add contact" }).click();
    await expect(page.getByRole("dialog", { name: "Edit Cy Diaz" })).toBeVisible();

    await page.goto("/d/customers/companies");
    await page.getByRole("button", { name: "New company" }).click();
    await page.getByRole("textbox", { name: "Name" }).fill("Initech");
    await page.getByRole("button", { name: "Add company" }).click();
    await expect(page.getByRole("heading", { name: "Initech" })).toBeVisible();

    await page.goto("/d/customers/blocked");
    await expect(page.getByText("No one is blocked.")).toBeVisible();
    const listed = await api<unknown[]>(page, "GET", "/contacts");
    expect(listed.length).toBe(1);
  });

  it("sets an attribute and narrows people by it and by company", async ({ page }) => {
    await page.goto("/d/customers");
    const ana = await api<{ id: string }>(page, "POST", "/contacts", { name: "Ana Lima", email: "ana@lima.example" });
    await api(page, "POST", "/contacts", { name: "Bo Chen", email: "bo@chen.example" });
    const acme = await api<{ id: string }>(page, "POST", "/companies", { name: "Acme" });
    await api(page, "PATCH", `/contacts/${ana.id}`, { company_id: acme.id });

    await page.goto(`/d/customers/${ana.id}`);
    await expect(page).toHaveURL(new RegExp(`/customers\\?contact=${ana.id}$`));
    const edit = page.getByRole("dialog", { name: "Edit Ana Lima" });
    const attributes = edit.getByRole("region", { name: "Attributes" });
    await attributes.getByRole("button", { name: "Add attribute" }).click();
    await attributes.getByRole("textbox", { name: "Attribute name" }).fill("plan");
    await attributes.getByRole("textbox", { name: "Value of plan" }).fill("pro");
    await edit.getByRole("button", { name: "Save" }).click();
    await expect(edit).toBeHidden();

    await page.goto("/d/customers");
    const people = page.getByRole("grid", { name: "People" });
    await expect(people.getByText("Bo Chen")).toBeVisible();
    const addFilter = async (field: RegExp) => {
      await page.getByRole("button", { name: "Add filter" }).click();
      await page.getByRole("dialog", { name: "Add filter" }).getByRole("button", { name: field }).click();
    };
    const panel = page.getByRole("dialog", { name: "Add filter" });
    await addFilter(/^plan$/i);
    await panel.getByRole("button", { name: "is not" }).click();
    await panel.getByRole("textbox", { name: /value/ }).fill("pro");
    await panel.getByRole("button", { name: "Apply" }).click();
    await expect(page.getByRole("button", { name: /Change filter: plan is not pro/i })).toBeVisible();
    await expect(people.getByText("Ana Lima")).toBeHidden();
    await expect(people.getByText("Bo Chen")).toBeVisible();

    await page.getByRole("button", { name: /Remove filter: plan is not pro/i }).click();
    await expect(people.getByText("Ana Lima")).toBeVisible();
    await addFilter(/^Company$/);
    await panel.getByRole("button", { name: "is any of" }).click();
    await panel.getByText("Acme").click();
    await panel.getByRole("button", { name: "Apply" }).click();
    await expect(page.getByRole("button", { name: "Change filter: Company is any of Acme" })).toBeVisible();
    await expect(people.getByText("Bo Chen")).toBeHidden();
    await expect(people.getByText("Ana Lima")).toBeVisible();
    await page.getByRole("button", { name: "Clear filters" }).click();
    await expect(people.getByText("Bo Chen")).toBeVisible();
  });

  it("tags several people at once with the tag picker", async ({ page }) => {
    await page.goto("/d/customers");
    await api(page, "POST", "/contacts", { name: "Ana Lima", email: "ana@lima.example" });
    await api(page, "POST", "/contacts", { name: "Bo Chen", email: "bo@chen.example" });
    await page.reload();
    const people = page.getByRole("grid", { name: "People" });
    await people
      .locator("label", { has: page.getByRole("checkbox", { name: "Select every row on this page" }) })
      .click();
    await expect(page.getByText("2 selected")).toBeVisible();
    await page.getByRole("button", { name: "Tag the selected people" }).click();
    await page.getByRole("textbox", { name: "Find or create a tag" }).fill("vip");
    await page.keyboard.press("Enter");
    await expect(people.getByText("vip")).toHaveCount(2);
  });

  it("picks the people list's columns and keeps them", async ({ page }) => {
    await page.goto("/d/customers");
    await api(page, "POST", "/contacts", { name: "Ana Lima", email: "ana@lima.example", phone: "+15550101" });
    await page.reload();
    const people = page.getByRole("grid", { name: "People" });
    await expect(people.getByRole("columnheader", { name: "Type" })).toBeVisible();
    await expect(people.getByRole("columnheader", { name: "Phone" })).toBeHidden();

    await page.getByRole("button", { name: "Columns" }).click();
    const chooser = page.getByRole("dialog", { name: "Columns" });
    await chooser.getByText("Phone", { exact: true }).click();
    await expect(chooser.getByRole("checkbox", { name: "Phone" })).toBeChecked();
    await chooser.getByText("Type", { exact: true }).click();
    await page.keyboard.press("Escape");
    await expect(people.getByRole("columnheader", { name: "Phone" })).toBeVisible();
    await expect(people.getByRole("columnheader", { name: "Type" })).toBeHidden();
    await expect(people.getByText("+15550101")).toBeVisible();

    await expect
      .poll(
        async () =>
          (await api<{ views: { people?: { columns?: string[] } } }>(page, "GET", "/inbox-preferences")).views.people
            ?.columns,
      )
      .toEqual(["email", "company", "tags", "phone", "last_seen"]);
    await page.reload();
    await expect(people.getByRole("columnheader", { name: "Phone" })).toBeVisible();
  });
});
