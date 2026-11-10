import type { Page, Route } from "@playwright/test";

import { createAccount, expect, test as it, signIn } from "./support/fixtures";

/*
 * Slack is outside the test's reach, so the Slack integration and its
 * notification destinations answer from here: a connected installation,
 * a channel Slack stopped, the channels the bot can post in and the
 * teammate's direct messages once turned on. Sign-in, permissions and
 * the notification preferences are the real server's, with a Slack
 * column added when the server has none yet.
 */
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
  features: [{ name: "notifications", status: "ready", missing_scopes: [] }],
  member_grant_count: 0,
  channel_ids: [],
  own_grant: null,
  installed_at: "2026-11-10T09:00:00Z",
  last_checked_at: null,
};

function destination(id: string, type: string, extra: Record<string, unknown> = {}) {
  return {
    id,
    object: "notification_destination",
    scope: type === "slack_dm" ? "member" : "workspace",
    type,
    name: type === "slack_dm" ? "Slack direct message" : "alerts",
    integration_id: "itg_e2e",
    slack_channel_id: type === "slack_dm" ? null : "C0ALERTS",
    slack_channel_name: type === "slack_dm" ? null : "alerts",
    triggers: ["new_conversation"],
    inbox_ids: null,
    include_ai_handled: false,
    show_previews: true,
    digests: [],
    digest_time: "09:00",
    next_digest_at: null,
    status: "active",
    last_error_code: null,
    last_error: null,
    last_delivered_at: null,
    created_at: "2026-11-10T09:00:00Z",
    ...extra,
  };
}

/*
 * fakeSlack answers the Slack endpoints and records what the page sends.
 */
async function fakeSlack(page: Page) {
  const sent: { method: string; path: string; body: unknown }[] = [];
  const destinations: ReturnType<typeof destination>[] = [
    destination("dst_old", "slack_channel", {
      name: "old-alerts",
      slack_channel_name: "old-alerts",
      status: "error",
      last_error_code: "channel_not_found",
      last_error: "The channel was archived.",
    }),
  ];

  const answer = (route: Route, data: unknown, status = 200) =>
    route.fulfill({ status, contentType: "application/json", body: JSON.stringify(data === null ? {} : { data }) });

  const create = (body: Record<string, unknown>) => {
    const made =
      body.type === "slack_dm"
        ? destination("dst_dm", "slack_dm")
        : destination("dst_new", "slack_channel", { triggers: body.triggers, digests: body.digests });
    destinations.push(made);
    return made;
  };

  const change = (id: string, body: Record<string, unknown>) => {
    const index = destinations.findIndex((d) => d.id === id);
    const current = destinations[index] as ReturnType<typeof destination>;
    const next = { ...current, ...body, ...(body.enabled ? { status: "active", last_error: null } : {}) };
    destinations[index] = next;
    return next;
  };

  const answers: Record<string, (method: string, body: Record<string, unknown>) => unknown> = {
    "/integrations": () => [installation],
    "/integrations/itg_e2e": () => installation,
    "/integrations/itg_e2e/slack-channels": () => [
      { id: "C0ALERTS", object: "slack_channel", name: "alerts", is_private: false },
      { id: "C0LEADS", object: "slack_channel", name: "leads", is_private: true },
    ],
    "/notification-destinations": (method, body) => (method === "POST" ? create(body) : destinations),
    "/notification-destinations/dst_old": (_, body) => change("dst_old", body),
    "/notification-destinations/dst_new/tests": () => ({
      object: "notification_destination_test",
      destination_id: "dst_new",
      status: "sent",
      error_code: null,
      error: null,
    }),
    "/notification-destinations/dst_dm/tests": () => ({
      object: "notification_destination_test",
      destination_id: "dst_dm",
      status: "failed",
      error_code: "user_not_found",
      error: "Slack has no user with your email.",
    }),
  };

  await page.route(/\/api\/v1\/(integration|notification-destinations)/, (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    if (path === "/integration-providers") {
      return route.fallback();
    }
    const body = request.postData() ? request.postDataJSON() : {};
    sent.push({ method: request.method(), path, body });

    const reply = answers[path];
    if (!reply) {
      return route.fulfill({ status: 404, contentType: "application/json", body: "{}" });
    }

    return answer(route, reply(request.method(), body), request.method() === "POST" ? 201 : 200);
  });

  return sent;
}

it.describe("Slack notifications", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Alerts ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
  });

  it("adds, tests and turns back on channel destinations", async ({ page }) => {
    const sent = await fakeSlack(page);

    await page.goto("/d/settings/integrations/itg_e2e");
    const section = page.getByRole("region", { name: "Notifications" });
    await expect(section.getByText("Stopped: The channel was archived.")).toBeVisible();

    await section.getByRole("button", { name: "Turn #old-alerts back on" }).click();
    await expect(page.getByText("Turned back on.")).toBeVisible();
    expect(sent.find((call) => call.method === "PATCH")?.body).toEqual({ enabled: true });
    await expect(section.getByText("Stopped: The channel was archived.")).toBeHidden();

    await section.getByRole("button", { name: "Add channel" }).click();
    const dialog = page.getByRole("dialog", { name: "Post to a Slack channel" });
    await dialog.getByRole("button", { name: /Channel/ }).click();
    await page.getByRole("option", { name: "#leads (private)" }).click();
    await dialog.getByText("Customer replies", { exact: true }).click();
    await dialog.getByText(/^Morning standup/).click();
    await dialog.getByRole("button", { name: "Add channel" }).click();
    await expect(dialog).toBeHidden();

    const created = sent.find((call) => call.method === "POST" && call.path === "/notification-destinations");
    expect(created?.body).toMatchObject({
      type: "slack_channel",
      integration_id: "itg_e2e",
      channel_id: "C0LEADS",
      triggers: ["new_conversation", "ai_handoff_requested", "customer_replied"],
      digests: ["daily_standup"],
      digest_time: "09:00",
      show_previews: true,
    });
    await expect(
      section.getByText("New conversations · Customer replies · Handed over by the agent · Morning standup"),
    ).toBeVisible();

    await section.getByRole("button", { name: "Test #alerts" }).click();
    await expect(page.getByText("Test sent. Check Slack.")).toBeVisible();

    await section.getByRole("button", { name: "Remove #alerts" }).click();
    await expect(page.getByRole("alertdialog", { name: "Stop posting to #alerts?" })).toBeVisible();
    await page.getByRole("button", { name: "Cancel" }).click();
  });

  it("sends notifications to the teammate's own Slack DMs", async ({ page }) => {
    const sent = await fakeSlack(page);
    await page.route("**/api/v1/notification-preferences", async (route) => {
      if (route.request().method() !== "GET") {
        return route.fallback();
      }
      const response = await route.fetch();
      const json = await response.json();
      const prefs = json.data;
      if (!prefs.channels.includes("slack")) {
        prefs.channels.push("slack");
        for (const trigger of prefs.triggers) {
          prefs.cells.push({ trigger: trigger.key, channel: "slack", enabled: true, default: true, locked: false });
        }
      }
      return route.fulfill({ response, json });
    });

    await page.goto("/d/settings/notifications");
    await expect(page.getByRole("columnheader", { name: "Slack" })).toBeVisible();

    await page.getByRole("button", { name: "Send to my Slack DMs" }).click();
    await expect(page.getByText("Slack direct messages are on.")).toBeVisible();
    expect(sent.find((call) => call.method === "POST")?.body).toEqual({ type: "slack_dm", integration_id: "itg_e2e" });

    await page.getByRole("button", { name: "Send a test" }).click();
    await expect(page.getByText("Slack has no user with your email.")).toBeVisible();
  });
});
