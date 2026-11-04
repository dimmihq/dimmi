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
