/*
 * The actions settings against the real server: the empty page explains
 * actions and starts a REST API connection in the same two-step dialog
 * as the other pickers; a connection to an address that never answers is
 * kept and says its test did not pass; and the action editor checks a
 * draft beside each field, fills the tool name from the title, moves an
 * action that changes data to the customer confirming first, and saves a
 * valid lookup as a draft on its own page.
 */
import type { Page } from "@playwright/test";

import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { settled, workspaceOf } from "./support/inbox";

/*
 * connect creates a REST connection through the API and returns its id.
 */
async function connect(page: Page): Promise<string> {
  const workspace = await workspaceOf(page);
  const response = await page.request.post("/api/v1/ai/action-connections", {
    headers: { "Dimmi-Workspace": workspace },
    data: { name: "Billing API", kind: "http", base_url: "https://billing.invalid/api", auth_type: "none" },
  });
  expect(response.status(), await response.text()).toBe(201);

  return ((await response.json()) as { data: { id: string } }).data.id;
}

it.describe("agent actions", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Actions ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("explains actions and connects a REST API from the empty page", async ({ page }) => {
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
    await expect(list.getByRole("button", { name: /MCP server/ })).toBeVisible();
    await expect(list.getByRole("region", { name: "How actions stay safe" })).toBeVisible();
    await list.getByRole("button", { name: /REST API/ }).click();

    const rest = page.getByRole("dialog", { name: "Connect a REST API" });
    await rest.getByRole("textbox", { name: "Name" }).fill("Billing API");
    await rest.getByRole("textbox", { name: "Base URL" }).fill("https://billing.invalid/api");
    await rest.getByRole("textbox", { name: "Token" }).fill("demo-token");
    await rest.getByRole("button", { name: "Connect REST API" }).click();
    await expect(rest).toBeHidden({ timeout: 20_000 });
    await expect(page.getByText("The connection test did not pass.", { exact: false })).toBeVisible();

    const card = page.getByRole("region", { name: "Billing API" });
    await expect(card).toContainText("billing.invalid");
    await expect(card).toContainText("…oken");
    await expect(card).toContainText("No actions yet.");
  });

  it("checks a draft beside each field and saves a lookup", async ({ page }) => {
    const connection = await connect(page);
    await page.goto(`/d/settings/actions/new?connection=${connection}`);
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

    await page.getByRole("radio", { name: /Only reads data/ }).check({ force: true });
    await page.getByRole("textbox", { name: "Title" }).fill("Check a subscription");
    await page.getByRole("textbox", { name: "Tool name" }).fill("get_subscription");
    await page
      .getByRole("textbox", { name: "Instructions for the agent" })
      .fill("Look up the signed-in customer's plan, seats and renewal date.");
    await page.getByRole("textbox", { name: "Path" }).fill("/customers/{contact.external_id}/subscription");
    await page.getByRole("button", { name: "Save draft" }).click();

    await expect(page).toHaveURL(/\/settings\/actions\/aac_/);
    await expect(page.getByRole("heading", { level: 1, name: "Check a subscription" })).toBeVisible();
    await expect(page.getByText("Draft", { exact: true })).toBeVisible();

    await page.goto("/d/settings/actions");
    await expect(page.getByRole("region", { name: "Billing API" })).toContainText("get_subscription");
  });
});
