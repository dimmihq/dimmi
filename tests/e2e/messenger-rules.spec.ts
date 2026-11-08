/*
 * The messenger's rules on a customer's page: a launcher kept to pricing
 * pages shows there, goes away when a single-page app moves elsewhere
 * and comes back, all without a request, and nothing about the rule but
 * its page condition reaches the page.
 */
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import type { Page } from "@playwright/test";

import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

const base = `http://localhost:${process.env.E2E_PORT ?? 18090}`;

/*
 * customerSite serves a page with the snippet from another origin and
 * opens it at the path.
 */
async function customerSite(page: Page, app: string, path: string) {
  const server = createServer((_, res) => {
    res.writeHead(200, { "Content-Type": "text/html" });
    res.end(`<!doctype html><html><head><title>Shop</title></head><body><h1>Shop</h1>
<script src="${base}/widget/v1/loader.js" data-app-id="${app}" async></script></body></html>`);
  });
  await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
  page.on("close", () => server.close());

  await page.goto(`http://127.0.0.1:${(server.address() as AddressInfo).port}${path}`);
}

it.describe("messenger rules", () => {
  it("shows the launcher only on the pages its rule names", async ({ page, person, request }) => {
    await createAccount(request, person, `Rules ${person.email.split("@")[0]?.split(".").at(-1)}`);
    const me = await request.get("/api/v1/me");
    const workspace: string = (await me.json()).data.workspaces[0].id;
    const headers = { "Dimmi-Workspace": workspace };
    const widgets = await request.get("/api/v1/widget-settings", { headers });
    const app: string = (await widgets.json()).data[0].id;
    const saved = await request.patch(`/api/v1/widget-settings/${app}/audiences`, {
      headers,
      data: {
        launcher: {
          rules: {
            any: [
              { field: "contact.custom.secret_plan", op: "eq", value: "platinum" },
              { field: "page.path", op: "starts_with", value: "/pricing" },
            ],
          },
        },
      },
    });
    expect(saved.status(), await saved.text()).toBe(200);

    const config = page.waitForResponse((r) => r.url().endsWith("/config"));
    await customerSite(page, app, "/pricing/teams");
    expect(await (await config).text()).not.toContain("secret_plan");
    await expect(page.locator(".launcher")).toBeVisible();

    await page.evaluate(() => history.pushState({}, "", "/about"));
    await expect(page.locator(".launcher")).toBeHidden();
    await page.evaluate(() => history.pushState({}, "", "/pricing"));
    await expect(page.locator(".launcher")).toBeVisible();
  });

  it("picks who sees the launcher and refuses an untrusted start rule", async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Picks ${person.email.split("@")[0]?.split(".").at(-1)}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
    await api(page, "POST", "/audiences", {
      name: "Pro plan",
      rules: { rules: { field: "contact.custom.plan", op: "eq", value: "pro" } },
    });
    const widgets = await api<{ id: string }[]>(page, "GET", "/widget-settings");
    const app = widgets[0]?.id ?? "";

    await page.goto(`/d/settings/messenger/${app}`);
    const section = page.getByRole("region", { name: "Who sees it" });
    await expect(async () => {
      await section.getByRole("button", { name: "Launcher" }).click();
      await page.getByRole("option", { name: "Pro plan" }).click({ timeout: 2000 });
    }).toPass();
    await expect
      .poll(async () =>
        JSON.stringify((await api<{ audiences: unknown }>(page, "GET", `/widget-settings/${app}`)).audiences),
      )
      .toContain("audience");
    await expect(section.getByRole("button", { name: "Launcher" })).toContainText("Pro plan");

    await expect(async () => {
      await section.getByRole("button", { name: "Starting a conversation" }).click();
      await page.getByRole("option", { name: "Pro plan", selected: false }).click({ timeout: 2000 });
    }).toPass();
    await expect(section).toContainText("cannot decide who may start a conversation");
  });
});
