import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("operator", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Operator ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("keeps spending features off until an admin turns them on, and sets what Operator may change", async ({
    page,
  }) => {
    await page.goto("/d/settings/operator");
    await expect(page.getByRole("switch", { name: "Operator", exact: true })).toBeChecked();
    await expect(page.getByRole("switch", { name: "Brief conversations as they open" })).not.toBeChecked();
    await expect(page.getByRole("switch", { name: "Triage new conversations" })).not.toBeChecked();

    const conversations = page.getByRole("radiogroup", { name: "Conversations changes" });
    await conversations.getByRole("radio", { name: "On its own" }).click();
    await expect(conversations.getByRole("radio", { name: "On its own" })).toBeChecked();
    await expect(
      page.getByRole("radiogroup", { name: "Help articles changes" }).getByRole("radio", { name: "On its own" }),
    ).toHaveCount(0);

    await page.reload();
    await expect(
      page.getByRole("radiogroup", { name: "Conversations changes" }).getByRole("radio", { name: "On its own" }),
    ).toBeChecked();
  });

  it("shows Operator's usage beside dimmi's, and its chats beside a conversation", async ({ page }) => {
    await page.goto("/d/operator");
    await expect(page).toHaveURL(/\/d\/[^/]+\/agent\?tab=operator$/);
    await expect(page.getByRole("tab", { name: "Operator", selected: true })).toBeVisible();
    await expect(page.getByRole("region", { name: "Operator this month" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Operator", exact: true })).toHaveCount(0);

    const inboxes = await api<{ id: string }[]>(page, "GET", "/inboxes");
    const created = await api<{ id: string }>(page, "POST", "/conversations", {
      inbox_id: inboxes[0]?.id,
      author: "contact",
      subject: "Charged twice",
      body: "I was charged twice in March.",
      contact: { email: "ana.lima@example.com", name: "Ana Lima" },
    });
    await page.goto(`/d/inbox/all/${created.id}`);
    const pane = page.getByRole("region", { name: "Operator" });
    await expect(pane.getByRole("button", { name: "Past chats" })).toBeDisabled();
    await pane.getByRole("button", { name: "Brief me" }).click();
    await expect(pane.getByRole("alert")).toHaveText("Operator needs an AI connection. Connect one in Settings, AI.");
  });
});
