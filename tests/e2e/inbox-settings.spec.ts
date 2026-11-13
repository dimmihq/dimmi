/*
 * Inbox settings end to end: an admin names teammates in seen receipts,
 * adds an inbox, changes its reopen window and assignment, and deletes
 * it, which moves its conversations to the default inbox. A member sees
 * the same settings without being able to change them.
 */
import type { Page } from "@playwright/test";

import { addTeammate } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

function workspaceOf(person: Person, prefix: string): string {
  return `${prefix} ${person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? ""}`;
}

/*
 * api calls the API as the signed-in page, in its first workspace.
 */
async function api(page: Page) {
  const me = await page.request.get("/api/v1/me");
  const headers = { "Dimmi-Workspace": (await me.json()).data.workspaces[0].id };

  return {
    get: async (path: string) => (await (await page.request.get(`/api/v1/${path}`, { headers })).json()).data,
    post: async (path: string, data: unknown) => {
      const response = await page.request.post(`/api/v1/${path}`, { headers, data });
      expect(response.status(), await response.text()).toBe(201);
      return (await response.json()).data;
    },
  };
}

/*
 * pick chooses the option of a select, opening it again when the page
 * re-rendered under the first press.
 */
async function pick(page: Page, select: string, option: string) {
  await expect(async () => {
    await page.getByRole("button", { name: select }).click();
    await page.getByRole("option", { name: option }).click({ timeout: 1000 });
  }).toPass();
}

it.describe("inbox settings", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("adds an inbox, sets it up and deletes it into the default", async ({ page, person, request }) => {
    await createAccount(request, person, workspaceOf(person, "Inboxes"));
    await signIn(page, person);
    await page.goto("/d/settings/inboxes");

    const inboxes = page.getByRole("region", { name: "Inboxes" });
    await expect(inboxes.getByText("Default")).toBeVisible();

    const names = page.getByRole("switch", { name: "Name in seen receipts" });
    await expect(names).not.toBeChecked();
    const named = page.waitForResponse((r) => r.request().method() === "PATCH" && r.url().endsWith("/workspace"));
    await names.click({ force: true });
    expect((await named).status()).toBe(200);
    await page.reload();
    await expect(names).toBeChecked();

    await inboxes.getByRole("button", { name: "New inbox" }).click();
    await page.getByRole("alertdialog").getByRole("textbox", { name: "Inbox name" }).fill("Billing");
    await page.getByRole("alertdialog").getByRole("button", { name: "Create inbox" }).click();
    await expect(page.getByRole("heading", { name: "Billing", level: 1 })).toBeVisible();

    await pick(page, "Reopen window", "7 days");
    await pick(page, "Assignment mode", "Round robin");
    const saved = page.waitForResponse((r) => r.request().method() === "PATCH" && r.url().includes("/inboxes/"));
    await page.getByRole("button", { name: "Save", exact: true }).click();
    expect((await saved).status()).toBe(200);
    await expect(page.getByRole("button", { name: "Discard" })).toHaveCount(0);

    await page.reload();
    await expect(page.getByRole("button", { name: "Reopen window" })).toContainText("7 days");
    await expect(page.getByRole("button", { name: "Assignment mode" })).toContainText("Round robin");
    await expect(page.getByRole("region", { name: "Assignment status" })).toBeVisible();

    const client = await api(page);
    const billing = (await client.get("inboxes")).find((i: { name: string }) => i.name === "Billing");
    const main = (await client.get("inboxes")).find((i: { default: boolean }) => i.default);
    expect(billing.settings.reopen_window_days).toBe(7);
    expect(billing.settings.assignment.mode).toBe("round_robin");
    const conversation = await client.post("conversations", {
      inbox_id: billing.id,
      author: "contact",
      subject: "Refund",
      body: "Please refund my order.",
      contact: { email: "rae@contoso.example", name: "Rae" },
    });

    await page.getByRole("button", { name: "Delete inbox" }).click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog.getByRole("button", { name: "Move conversations to" })).toContainText(main.name);
    await dialog.getByRole("button", { name: "Delete inbox" }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/settings\/inboxes$/);
    await expect(inboxes.getByText("Billing")).toHaveCount(0);

    expect((await client.get(`conversations/${conversation.id}`)).inbox.id).toBe(main.id);
  });

  it("shows a member the settings without letting them change them", async ({ page, person, request }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, workspaceOf(person, "Readonly"));
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, teammate);
    await page.goto("/d/settings/inboxes");

    const inboxes = page.getByRole("region", { name: "Inboxes" });
    await expect(inboxes.getByRole("button", { name: "New inbox" })).toHaveCount(0);
    await expect(page.getByRole("switch", { name: "Name in seen receipts" })).toBeDisabled();
    await inboxes.getByRole("button", { name: "View" }).first().click();

    await expect(page.getByText("Only admins can change inboxes.")).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Inbox name" })).toBeDisabled();
    await expect(page.getByRole("switch", { name: "Assign to whoever replies" })).toBeDisabled();
    await expect(page.getByRole("button", { name: "Save", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Delete inbox" })).toHaveCount(0);
  });
});
