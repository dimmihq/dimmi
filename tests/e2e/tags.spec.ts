import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

const customer = {
  name: "Emily Johnson",
  email: "emily@northwind.example",
  subject: "Refund please",
  body: "I was charged twice.",
};

it.describe("tags", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Tags ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("creates a tag from the keyboard, shows it, and takes it off", async ({ page }) => {
    const id = await startConversation(page, customer);
    await page.goto(`/d/inbox/unassigned/${id}`);
    const details = page.getByRole("region", { name: "Details" });
    await expect(details.getByRole("button", { name: "Add tag", exact: true })).toBeVisible();

    await page.keyboard.press("Shift+T");
    const find = page.getByRole("textbox", { name: "Find or create a tag" });
    await find.fill("Billing");
    await find.press("Enter");
    await page.keyboard.press("Escape");

    await expect(details.getByText("Billing")).toBeVisible();
    await expect(page.getByText(/tagged the conversation Billing/)).toBeVisible();

    await details.getByRole("button", { name: "Remove Billing" }).click();
    await expect(details.getByRole("button", { name: "Remove Billing" })).toBeHidden();
    await expect(page.getByText(/removed the tag Billing/)).toBeVisible();
  });

  it("manages the vocabulary in settings", async ({ page }) => {
    const id = await startConversation(page, customer);
    const me = await page.request.get("/api/v1/me");
    const workspace = ((await me.json()) as { data: { workspaces: { id: string }[] } }).data.workspaces[0]?.id ?? "";
    const tagged = await page.request.post(`/api/v1/conversations/${id}/tags`, {
      headers: { "Dimmi-Workspace": workspace },
      data: { tags: ["Bug"] },
    });
    expect(tagged.status(), await tagged.text()).toBe(200);

    await page.goto("/d/settings/tags");
    await expect(page.getByText("1 conversation", { exact: true })).toBeVisible();

    await page.getByRole("button", { name: "New tag" }).click();
    await page.getByRole("textbox", { name: "Tag name" }).fill("VIP");
    await page.getByRole("button", { name: "Create tag" }).click();
    await expect(page.getByText("VIP")).toBeVisible();

    await page.getByRole("button", { name: "Edit" }).first().click();
    await page.getByRole("textbox", { name: "Tag name" }).fill("Defect");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByText("Defect")).toBeVisible();

    await page.getByRole("button", { name: "Delete" }).first().click();
    await expect(page.getByRole("alertdialog")).toContainText("1 conversations");
    await page.getByRole("button", { name: "Delete tag" }).click();
    await expect(page.getByRole("alertdialog")).toBeHidden();
    await expect(page.getByText("Defect", { exact: true })).toBeHidden();
  });
});
