/*
 * Conversations for the inbox tests, started through the API the way a
 * developer relays what a customer wrote: the signed-in page's own
 * session makes an inbox when the workspace has none, then starts each
 * conversation on its API channel.
 */
import { expect, type Page } from "@playwright/test";

export interface Customer {
  name: string;
  email: string;
  subject: string;
  body: string;
}

/*
 * workspaceOf returns the id of the signed-in person's workspace.
 */
async function workspaceOf(page: Page): Promise<string> {
  const response = await page.request.get("/api/v1/me");
  expect(response.status(), await response.text()).toBe(200);
  const me = (await response.json()) as { data: { workspaces: { id: string }[] } };

  return me.data.workspaces[0]?.id ?? "";
}

/*
 * inboxOf returns the workspace's first inbox, making one when it has
 * none.
 */
async function inboxOf(page: Page, workspace: string): Promise<string> {
  const headers = { "Dimmi-Workspace": workspace };
  const listed = await page.request.get("/api/v1/inboxes", { headers });
  expect(listed.status(), await listed.text()).toBe(200);
  const inboxes = (await listed.json()) as { data: { id: string }[] };
  if (inboxes.data[0]) {
    return inboxes.data[0].id;
  }

  const created = await page.request.post("/api/v1/inboxes", { headers, data: { name: "Support" } });
  expect(created.status(), await created.text()).toBe(201);

  return ((await created.json()) as { data: { id: string } }).data.id;
}

/*
 * startConversation starts a conversation the customer wrote and
 * returns its id.
 */
export async function startConversation(page: Page, customer: Customer): Promise<string> {
  const workspace = await workspaceOf(page);
  const inbox = await inboxOf(page, workspace);
  const response = await page.request.post("/api/v1/conversations", {
    headers: { "Dimmi-Workspace": workspace },
    data: {
      inbox_id: inbox,
      author: "contact",
      subject: customer.subject,
      body: customer.body,
      contact: { email: customer.email, name: customer.name },
    },
  });
  expect(response.status(), await response.text()).toBe(201);

  return ((await response.json()) as { data: { id: string } }).data.id;
}

/*
 * openComposer opens the reply or note composer from its resting bar and
 * waits for the formatting editor, so typing never lands in the plain
 * field it replaces.
 */
export async function openComposer(page: Page, name: RegExp): Promise<void> {
  await page.getByRole("button", { name }).click();
  await expect(page.getByRole("toolbar", { name: "Formatting" })).toBeVisible({ timeout: 15_000 });
}

/*
 * pickView opens the view picker in the list's title and goes to the
 * named view.
 */
export async function pickView(page: Page, name: string | RegExp): Promise<void> {
  await page.locator("button[data-view-picker]").click();
  await page.getByRole("dialog", { name: "Views" }).getByRole("button", { name }).click();
}

/*
 * openAddress opens the page of one of the workspace's email addresses,
 * the first unless one is named.
 */
export async function openAddress(page: Page, name?: string): Promise<void> {
  await page.goto("/d/settings/email");
  const addresses = page.getByRole("region", { name: "Addresses" });
  const link = name ? addresses.getByRole("link", { name }) : addresses.getByRole("link").first();
  await link.click();
  await expect(page.getByTestId("forwarding-address")).toBeVisible();
}

/*
 * openMessenger opens the page of the workspace's first messenger and
 * waits for it to finish arriving. The page rises into place as it
 * arrives, and a click aimed while it moves misses, so Playwright
 * scrolls to try again, and that late scroll closes any popover the
 * click opened.
 */
export async function openMessenger(page: Page): Promise<void> {
  await page.goto("/d/settings/messenger");
  await page.getByRole("region", { name: "Messengers" }).getByRole("link").first().click();
  await expect(page.getByRole("region", { name: "Appearance" })).toBeVisible();
  await page.waitForFunction(() => document.getAnimations().every((a) => a.playState !== "running"));
}
