import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

const customer = {
  name: "Emily Johnson",
  email: "emily@northwind.example",
  subject: "Refund please",
  body: "I was charged twice.",
};

it.describe("customer panel", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Panel ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("shows who the customer is, edits them in place, and lists their other conversations", async ({ page }) => {
    const older = await startConversation(page, { ...customer, subject: "Older question" });
    const id = await startConversation(page, customer);
    const conversation = await api<{ contact: { id: string } }>(page, "GET", `/conversations/${id}`);
    const company = await api<{ id: string }>(page, "POST", "/companies", {
      name: "Northwind",
      domain: "northwind.example",
    });
    await api(page, "PATCH", `/contacts/${conversation.contact.id}`, { company_id: company.id });

    await page.goto(`/d/inbox/unassigned/${id}`);
    const details = page.getByRole("region", { name: "Details" });
    await expect(details.getByRole("link", { name: "Emily Johnson" })).toBeVisible();
    await expect(details.getByText("emily@northwind.example").first()).toBeVisible();
    await expect(details.getByRole("button", { name: "Copy email" })).toBeVisible();
    await expect(details.getByRole("link", { name: "Northwind" })).toBeVisible();

    await details.getByRole("button", { name: "Edit name" }).click();
    const name = details.getByRole("textbox", { name: "Customer name" });
    await name.fill("Emily J.");
    await name.press("Enter");
    await expect(details.getByRole("link", { name: "Emily J." })).toBeVisible();

    const notes = details.getByRole("textbox", { name: "Notes" });
    await notes.fill("Prefers email");
    await notes.blur();
    await expect
      .poll(
        async () => (await api<{ notes: string | null }>(page, "GET", `/contacts/${conversation.contact.id}`)).notes,
      )
      .toBe("Prefers email");

    await details.getByRole("link", { name: /Older question/ }).click();
    await expect(page).toHaveURL(new RegExp(`/d/[^/]+/inbox/unassigned/${older}$`));
  });

  it("warns of a possible duplicate and opens the customer's page", async ({ page }) => {
    const id = await startConversation(page, customer);
    const conversation = await api<{ contact: { id: string } }>(page, "GET", `/conversations/${id}`);
    await api(page, "PATCH", `/contacts/${conversation.contact.id}`, { phone: "+15550100" });
    await api(page, "POST", "/contacts", { name: "Em Johnson", phone: "+15550100" });

    await page.goto(`/d/inbox/unassigned/${id}`);
    const details = page.getByRole("region", { name: "Details" });
    await expect(details.getByText("Possible duplicate: Em Johnson")).toBeVisible();
    await details.getByRole("link", { name: "Review" }).click();
    await expect(page.getByRole("region", { name: "Possible duplicates" })).toContainText("same phone");
  });

  it("pins an attribute so it shows first, for every customer", async ({ page }) => {
    const id = await startConversation(page, customer);
    const conversation = await api<{ contact: { id: string } }>(page, "GET", `/conversations/${id}`);
    await api(page, "PATCH", `/contacts/${conversation.contact.id}`, {
      custom_attributes: { plan: "pro", seats: 12 },
    });

    await page.goto(`/d/inbox/unassigned/${id}`);
    const details = page.getByRole("region", { name: "Details" });
    await details.getByRole("button", { name: "Pin seats" }).click();
    await expect(details.getByRole("button", { name: "Unpin seats" })).toBeVisible();
    await expect
      .poll(
        async () => (await api<{ pinned_attributes: string[] }>(page, "GET", "/inbox-preferences")).pinned_attributes,
      )
      .toEqual(["seats"]);

    const labels = await details.locator("dt").allInnerTexts();
    expect(labels.indexOf("seats")).toBeLessThan(labels.indexOf("Email"));
    expect(labels.indexOf("plan")).toBeGreaterThan(labels.indexOf("Email"));
  });
});
