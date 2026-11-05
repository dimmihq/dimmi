/*
 * The messenger on a customer's page, end to end against the release
 * binary: the snippet on a plain page shows the launcher, a visitor writes
 * and a teammate's reply arrives without a reload, the conversation is
 * still there after a reload, replies still arrive by polling when the
 * stream is blocked, and a page with a strict content security policy or
 * a phone-sized window works the same. A wrong app id shows nothing and
 * says why in the console.
 */
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import type { APIRequestContext, Page } from "@playwright/test";

import { createAccount, expect, test as it, type Person } from "./support/fixtures";

const base = `http://localhost:${process.env.E2E_PORT ?? 18090}`;

interface Desk {
  workspace: string;
  app: string;
}

/*
 * openDesk creates the person's account and finds their first widget.
 */
async function openDesk(request: APIRequestContext, person: Person): Promise<Desk> {
  await createAccount(request, person, `Shop ${person.email.split("@")[0]?.split(".").at(-1)}`);
  const me = await request.get("/api/v1/me");
  const workspace: string = (await me.json()).data.workspaces[0].id;
  const widgets = await request.get("/api/v1/widget-settings", { headers: { "Dimmi-Workspace": workspace } });

  return { workspace, app: (await widgets.json()).data[0].id };
}

/*
 * customerSite serves a plain page with the snippet from a site of its
 * own, another origin than the install's, and opens it.
 */
async function customerSite(page: Page, app: string, csp?: string) {
  const server = createServer((_, res) => {
    res.writeHead(200, { "Content-Type": "text/html", ...(csp ? { "Content-Security-Policy": csp } : {}) });
    res.end(`<!doctype html><html><head><title>Shop</title></head><body><h1>Shop</h1>
<script src="${base}/widget/v1/loader.js" data-app-id="${app}" async></script></body></html>`);
  });
  await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
  page.on("close", () => server.close());

  await page.goto(`http://127.0.0.1:${(server.address() as AddressInfo).port}/pricing`);
}

/*
 * reply has the owner answer the visitor's only conversation.
 */
async function reply(request: APIRequestContext, desk: Desk, body: string) {
  const headers = { "Dimmi-Workspace": desk.workspace };
  const list = await request.get("/api/v1/conversations?filter[view]=all", { headers });
  const id: string = (await list.json()).data[0].id;
  const sent = await request.post(`/api/v1/conversations/${id}/replies`, { headers, data: { body } });
  expect(sent.status(), await sent.text()).toBe(201);
}

/*
 * write opens a new conversation and sends the text.
 */
async function write(page: Page, text: string) {
  await page.locator(".launcher").click();
  await page.getByText("Chat with us").click();
  const box = page.locator('[data-view="conversation"] textarea');
  await box.fill(text);
  await box.press("Enter");
}

it.describe("messenger on a customer's page", () => {
  it("sends, hears the team live and keeps the conversation across a reload", async ({ page, person, request }) => {
    const desk = await openDesk(request, person);
    await customerSite(page, desk.app);

    await expect(page.locator(".launcher")).toBeVisible();
    await write(page, "Export fails");
    await expect(page.locator(".thread")).toContainText("Export fails");

    await expect
      .poll(
        async () =>
          (
            await (
              await request.get("/api/v1/conversations?filter[view]=all", {
                headers: { "Dimmi-Workspace": desk.workspace },
              })
            ).json()
          ).data.length,
      )
      .toBe(1);
    await reply(request, desk, "Looking at it now");
    await expect(page.locator(".thread")).toContainText("Looking at it now");

    await page.reload();
    await page.locator(".launcher").click();
    await page.getByRole("tab", { name: "Messages" }).click();
    await expect(page.locator('[data-view="messages"] .loaded .list')).toContainText("#1");
  });

  it("hears the team by polling when the stream is blocked", async ({ page, person, request }) => {
    const desk = await openDesk(request, person);
    await page.route(`${base}/connection/uni_sse**`, (route) =>
      route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        headers: { "Access-Control-Allow-Origin": "*" },
        body: "",
      }),
    );
    await customerSite(page, desk.app);

    await write(page, "Behind a proxy");
    await expect(page.locator(".thread")).toContainText("Behind a proxy");
    await expect
      .poll(
        async () =>
          (
            await (
              await request.get("/api/v1/conversations?filter[view]=all", {
                headers: { "Dimmi-Workspace": desk.workspace },
              })
            ).json()
          ).data.length,
      )
      .toBe(1);
    await reply(request, desk, "Polling works");

    await expect(page.locator(".thread")).toContainText("Polling works", { timeout: 15_000 });
  });

  it("works on a page with a strict content security policy and at phone width", async ({ page, person, request }) => {
    const desk = await openDesk(request, person);
    await page.setViewportSize({ width: 390, height: 844 });
    await customerSite(
      page,
      desk.app,
      `default-src 'none'; script-src ${base}; connect-src ${base}; img-src ${base} data:; style-src 'none'`,
    );

    await write(page, "Strict page");
    await expect(page.locator(".thread")).toContainText("Strict page");
  });

  it("shows nothing for an app id no messenger has, and says why", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("console", (message) => {
      if (message.type() === "error") {
        errors.push(message.text());
      }
    });

    await customerSite(page, "wgt_01m6m7c2h2e4ptzvvpdg41gy3w");

    await expect.poll(() => errors.join(" ")).toContain("app_not_found");
    await expect(page.locator(".launcher")).toHaveCount(0);
    await context.close();
  });
});
