/*
 * The actions settings: the empty page explains actions and starts a
 * REST API or MCP server connection in the same two-step dialog as the
 * other pickers, and the action editor checks a draft beside each field
 * before saving, filling the tool name from the title and moving an
 * action that changes data to the customer confirming first. The
 * actions endpoints and their permission are answered in the browser,
 * so these tests cover the pages on their own.
 */
import type { Page } from "@playwright/test";

import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { settled } from "./support/inbox";

const billing = {
  id: "acn_billing",
  object: "ai_action_connection",
  name: "Billing API",
  kind: "http",
  base_url: "https://billing.example.com/api",
  auth_type: "bearer",
  auth_config: {},
  secret_hint: "…9f2a",
  default_headers: {},
  variables: {},
  timeout_ms: 8000,
  status: "active",
  status_reason: null,
  last_success_at: null,
  actions_count: 0,
  created_at: "2026-11-07T09:00:00Z",
  updated_at: "2026-11-07T09:00:00Z",
};

/*
 * answerActions answers the actions endpoints with the connections
 * given and lets the signed-in owner manage actions.
 */
async function answerActions(page: Page, connections: unknown[]) {
  await page.route("**/api/v1/me", async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    const member = body?.data?.member;
    if (member) {
      member.permissions = [...member.permissions, "ai_actions:manage", "ai_action_calls:approve"];
    }
    await route.fulfill({ response, json: body });
  });
  await page.route("**/api/v1/ai/action-connections", (route) => route.fulfill({ json: { data: connections } }));
  await page.route("**/api/v1/ai/actions", (route) => route.fulfill({ json: { data: [] } }));
}

it.describe("agent actions", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Actions ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("explains actions and starts a connection from the empty page", async ({ page }) => {
    await answerActions(page, []);
    await page.goto("/d/settings/actions");
    await expect(page.getByRole("heading", { level: 1, name: "Actions" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Let dimmi act in your systems" })).toBeVisible();
    await expect(page.getByRole("list", { name: "Examples" })).toContainText("Refund a payment");

    await page.getByRole("button", { name: /Connect a REST API/ }).click();
    const form = page.getByRole("dialog", { name: "Connect a REST API" });
    await form.getByRole("button", { name: "Connect REST API" }).click();
    await expect(form.getByText("Give the connection a name.")).toBeVisible();
    await expect(form.getByText("Enter the full address, starting with https://.")).toBeVisible();
    await expect(form.getByText("Paste the secret dimmi signs in with.")).toBeVisible();

    await form.getByRole("button", { name: "All connections" }).click();
    const list = page.getByRole("dialog", { name: "Add a connection" });
    await expect(list.getByRole("button", { name: /REST API/ })).toBeVisible();
    await expect(list.getByRole("region", { name: "How actions stay safe" })).toBeVisible();
    await list.getByRole("button", { name: /MCP server/ }).click();
    const mcp = page.getByRole("dialog", { name: "Connect an MCP server" });
    await expect(mcp.getByRole("textbox", { name: "Server URL" })).toBeVisible();

    await mcp.getByRole("button", { name: "Cancel" }).click();
    await expect(mcp).toBeHidden();
  });

  it("checks a draft action beside each field and defaults writes to the customer confirming", async ({ page }) => {
    await answerActions(page, [billing]);
    await page.goto("/d/settings/actions/new?connection=acn_billing");
    await expect(page.getByRole("heading", { level: 1, name: "New action" })).toBeVisible();
    await expect(page.getByText("Calls through")).toContainText("Billing API");
    await settled(page);

    await page.getByRole("button", { name: "Save draft" }).click();
    await expect(page.getByText("Give the action a name.")).toBeVisible();
    await expect(page.getByText("Give the action a title teammates and customers see.")).toBeVisible();

    await page.getByRole("textbox", { name: "Title" }).fill("Refund an invoice");
    await expect(page.getByRole("textbox", { name: "Tool name" })).toHaveValue("refund_an_invoice");
    await expect(page.getByText("Give the action a name.")).toBeHidden();

    await page.getByRole("radio", { name: /Changes data/ }).check({ force: true });
    await expect(page.getByRole("radio", { name: /Customer confirms first/ })).toBeChecked();
    await expect(page.getByRole("region", { name: "Permissions" })).toContainText(
      "Nothing runs until they press Confirm.",
    );
    await expect(page.getByRole("checkbox", { name: "Email" })).toBeDisabled();

    await page.getByRole("radio", { name: /A teammate approves/ }).check({ force: true });
    await expect(page.getByRole("region", { name: "Permissions" })).toContainText("A teammate approves or declines");
    await expect(page.getByText("Bind the call to the signed-in customer")).toBeVisible();
  });
});
