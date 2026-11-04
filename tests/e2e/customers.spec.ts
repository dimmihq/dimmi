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
    await expect(people.getByRole("link", { name: "Bo Chen" })).toBeVisible();
    await page.getByRole("textbox", { name: "Search people" }).fill("ana");
    await expect(people.getByRole("link", { name: "Bo Chen" })).toBeHidden();
    await people.getByRole("link", { name: "Ana Lima" }).click();

    const details = page.getByRole("region", { name: "Details" });
    await details.getByRole("textbox", { name: "Phone" }).fill("+15550101");
    await details.getByRole("button", { name: "Save" }).click();
    await expect(details.getByRole("button", { name: "Save" })).toBeDisabled();

    await page
      .getByRole("region", { name: "Conversations" })
      .getByRole("link", { name: /Invoice VAT/ })
      .click();
    await expect(page).toHaveURL(/\/d\/inbox\/all\/conv_/);
  });

  it("adds a contact and a company, and keeps blocked people apart", async ({ page }) => {
    await page.goto("/d/customers");
    await page.getByRole("button", { name: "New contact" }).click();
    await page.getByRole("textbox", { name: "Name" }).fill("Cy Diaz");
    await page.getByRole("button", { name: "Add contact" }).click();
    await expect(page.getByRole("heading", { name: "Cy Diaz" })).toBeVisible();

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
    const attributes = page.getByRole("region", { name: "Attributes" });
    await attributes.getByRole("textbox", { name: "Attribute name" }).fill("plan");
    await attributes.getByRole("textbox", { name: "Attribute value" }).fill("pro");
    await attributes.getByRole("button", { name: "Set" }).click();
    await expect(attributes.getByRole("button", { name: "Remove plan" })).toBeVisible();

    await page.goto("/d/customers");
    const people = page.getByRole("grid", { name: "People" });
    await expect(people.getByRole("link", { name: "Bo Chen" })).toBeVisible();
    await page.getByRole("button", { name: "Attribute" }).click();
    await page.getByRole("textbox", { name: "Attribute", exact: true }).fill("plan");
    await page.getByRole("textbox", { name: "Value" }).fill("pro");
    await page.getByRole("button", { name: "Apply" }).click();
    await expect(people.getByRole("link", { name: "Bo Chen" })).toBeHidden();
    await expect(people.getByRole("link", { name: "Ana Lima" })).toBeVisible();

    await page.getByRole("button", { name: "plan is pro" }).click();
    await page.getByRole("button", { name: "Clear" }).click();
    await expect(people.getByRole("link", { name: "Bo Chen" })).toBeVisible();
    await page.getByRole("button", { name: "Company" }).click();
    await page.getByRole("option", { name: "Acme" }).click();
    await expect(people.getByRole("link", { name: "Bo Chen" })).toBeHidden();
    await expect(people.getByRole("link", { name: "Ana Lima" })).toBeVisible();
  });
});
