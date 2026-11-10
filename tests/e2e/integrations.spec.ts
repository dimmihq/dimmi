import type { Page, Route } from "@playwright/test";

import { createAccount, expect, test as it, signIn } from "./support/fixtures";

/*
 * Slack is outside the test's reach, so the integration endpoints answer
 * from here: a Slack installation once connected, its two channels (one
 * the app was removed from) and whatever the page sends. The page under
 * test, its sign-in and its permissions are the real server's.
 */
const authorizeURL = "https://slack.com/oauth/v2/authorize?client_id=e2e&state=xyz";

const provider = {
  object: "integration_provider",
  name: "slack",
  label: "Slack",
  installable: true,
  features: [{ name: "channels", bot_scopes: ["chat:write"], user_scopes: ["chat:write"] }],
};

const installation = {
  id: "itg_e2e",
  object: "integration",
  provider: "slack",
  name: "Acme",
  external_account_id: "T0ACME",
  source: "oauth",
  status: "active",
  scopes: ["chat:write"],
  last_error_code: null,
  last_error: null,
  metadata: {},
  features: [
    { name: "channels", status: "ready", missing_scopes: [] },
    { name: "operator", status: "blocked_missing_scope", missing_scopes: ["im:history"] },
  ],
  member_grant_count: 0,
  channel_ids: ["chn_e2e"],
  own_grant: null,
  installed_at: "2026-11-10T09:00:00Z",
  last_checked_at: null,
};

function room(id: string, name: string, status: string) {
  return {
    id,
    object: "channel_room",
    channel_id: "chn_e2e",
    external_id: id.toUpperCase(),
    name,
    status,
    mode: "shared",
    trigger: "author_window",
    is_shared_external: true,
    settings: {},
    last_error: null,
    removed_at: null,
  };
}

/*
 * fakeIntegrations answers the integration endpoints and records what
 * the page sends.
 */
async function fakeIntegrations(page: Page) {
  const sent: { method: string; path: string; body: unknown }[] = [];
  let connected = false;
  const rooms = [room("chr_support", "#acme-support", "active"), room("chr_old", "#old-help", "removed")];

  const answer = (route: Route, data: unknown) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data }) });
  const back = new URL("/d/settings/integrations?connected=itg_e2e", page.url()).href;

  const answers: Record<string, (body: unknown) => unknown> = {
    "/integration-providers": () => [provider],
    "/integrations": () => (connected ? [installation] : []),
    "/integrations/slack/installations": () => {
      connected = true;
      return { object: "integration_authorization", authorize_url: authorizeURL };
    },
    "/integrations/itg_e2e": () => installation,
    "/channels/chn_e2e/rooms": () => rooms,
    "/channels/chn_e2e/rooms/chr_support": (body) => {
      rooms[0] = { ...rooms[0], ...(body as object) } as (typeof rooms)[0];
      return rooms[0];
    },
  };

  await page.route(/\/api\/v1\/(integration|channels\/chn_e2e)/, (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const body = request.postData() ? request.postDataJSON() : null;
    sent.push({ method: request.method(), path, body });

    const reply = answers[path];
    if (!reply) {
      return route.fulfill({ status: 404, contentType: "application/json", body: "{}" });
    }

    return answer(route, reply(body));
  });

  await page
    .context()
    .route("https://slack.com/**", (route) => route.fulfill({ status: 302, headers: { Location: back } }));

  return sent;
}

it.describe("Integrations", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Tools ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
  });

  it("connects Slack, reads its features and channels and changes a channel", async ({ page }) => {
    const sent = await fakeIntegrations(page);

    await page.goto("/d/settings/integrations");
    await expect(page.getByRole("heading", { name: "Integrations" })).toBeVisible();
    await expect(page.getByText("Answer customers in Slack channels", { exact: false })).toBeVisible();

    await page.getByRole("button", { name: "Connect", exact: true }).click();
    await expect(page).toHaveURL(/\/settings\/integrations$/);
    expect(sent.find((call) => call.path === "/integrations/slack/installations")?.body).toEqual({
      return_to: expect.stringMatching(/^\/d\/[^/]+\/settings\/integrations$/),
    });
    await expect(page.getByText("Connected. Every feature can use it now.")).toBeVisible();
    await expect(page.getByText("Needs permissions")).toBeVisible();

    await page.getByRole("button", { name: "Manage Slack" }).click();
    await expect(page.getByRole("heading", { name: "Acme" })).toBeVisible();
    await expect(page.getByText("Needs permission for im:history", { exact: false })).toBeVisible();
    await expect(page.getByRole("button", { name: "Update permissions" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Connect your Slack" })).toBeVisible();
    await expect(page.getByText("Invite @dimmi to the channel again", { exact: false })).toBeVisible();

    await page.getByRole("button", { name: "Settings for #acme-support" }).click();
    const dialog = page.getByRole("dialog", { name: "#acme-support" });
    await dialog.getByRole("button", { name: /Open a conversation for/ }).click();
    await page.getByRole("option", { name: "Requests the AI spots" }).click();
    await expect(page.getByRole("listbox")).toBeHidden();
    await dialog.getByRole("button", { name: /The agent/ }).click();
    await page.getByRole("option", { name: "Draft for review" }).click();
    await expect(page.getByRole("listbox")).toBeHidden();
    await dialog.getByRole("switch", { name: "Automatic reply" }).focus();
    await page.keyboard.press("Space");
    await expect(dialog.getByRole("switch", { name: "Automatic reply" })).toBeChecked();
    await dialog.getByRole("button", { name: "Save" }).click();

    await expect(page.getByText("#acme-support saved.")).toBeVisible();
    expect(sent.find((call) => call.method === "PATCH")?.body).toEqual({
      trigger: "ai",
      settings: { agent_mode: "draft", auto_reply_after_seconds: 300 },
    });
    await expect(page.getByText("Shared with a customer · Requests the AI spots")).toBeVisible();

    await page.getByRole("button", { name: "Disconnect", exact: true }).click();
    await expect(page.getByRole("alertdialog", { name: "Disconnect Slack?" })).toBeVisible();
    await page.getByRole("button", { name: "Cancel" }).click();
  });

  it("explains a connection that was declined", async ({ page }) => {
    await fakeIntegrations(page);

    await page.goto("/d/settings/integrations?error=denied");
    await expect(page.getByText("The connection was declined, so nothing changed.")).toBeVisible();
    await expect(page).toHaveURL(/\/settings\/integrations$/);
  });
});
