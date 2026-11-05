/*
 * Realtime end to end: a change one person makes shows on a teammate's
 * open screen without a reload, a reply one teammate sends lands in the
 * conversation another has open, and the centrifuge-js SDK, the one the
 * dashboard and integrations use, speaks to the server from Node:
 * connecting with the session cookie, following the workspace and a
 * conversation, listing who is viewing, and recovering what it missed
 * while disconnected.
 */
import type { APIRequestContext } from "@playwright/test";
import { Centrifuge, type PublicationContext, type SubscribedContext } from "centrifuge";

import { addTeammate, startConversation } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { openComposer, startConversation as relayConversation } from "./support/inbox";

const base = `http://localhost:${process.env.E2E_PORT ?? 18090}`;

function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

/*
 * sessionCookie signs the person in through the API and returns the
 * Cookie header their browser would send.
 */
async function sessionCookie(request: APIRequestContext, person: Person): Promise<string> {
  const response = await request.post("/api/v1/sessions", {
    data: { grant: "password", email: person.email, password: person.password },
  });
  expect(response.status(), await response.text()).toBe(201);

  return response
    .headersArray()
    .filter((header) => header.name.toLowerCase() === "set-cookie")
    .map((header) => header.value.split(";")[0])
    .join("; ");
}

/*
 * socketWithCookie is a WebSocket that sends the cookie with its
 * upgrade, as a browser on the app's own page does.
 */
function socketWithCookie(cookie: string) {
  return class extends WebSocket {
    constructor(url: string | URL, protocols?: string | string[]) {
      super(url, { protocols, headers: { cookie } } as unknown as string[]);
    }
  };
}

it.describe("realtime", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("shows a teammate a renamed workspace without a reload", async ({ page, person, request, browser }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, `Live ${suffixOf(person)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");

    const other = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const watching = await other.newPage();
    const problems: string[] = [];
    watching.on("pageerror", (error) => problems.push(error.message));
    watching.on("console", (message) => {
      const apiRefusal = /status of 4\d\d/.test(message.text()) && message.location().url.includes("/api/v1/");
      if (message.type() === "error" && !apiRefusal) {
        problems.push(message.text());
      }
    });

    await signIn(watching, teammate);
    await expect(watching.locator('[data-realtime="connected"]')).toBeAttached();
    await expect(watching.getByText(`Live ${suffixOf(person)}`).first()).toBeVisible();

    await signIn(page, person);
    const renamed = await page.request.patch("/api/v1/workspace", { data: { name: `Renamed ${suffixOf(person)}` } });
    expect(renamed.status()).toBe(200);

    await expect(watching.getByText(`Renamed ${suffixOf(person)}`).first()).toBeVisible();
    expect(problems).toEqual([]);
    await other.close();
  });

  it("shows a teammate's reply in a conversation another has open", async ({ page, person, request, browser }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, `Desk ${suffixOf(person)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");

    await signIn(page, person);
    const id = await relayConversation(page, {
      name: "Emily Johnson",
      email: "emily@northwind.example",
      subject: "Can't log in",
      body: "My password manager's password is refused.",
    });

    const other = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const watching = await other.newPage();
    const problems: string[] = [];
    watching.on("pageerror", (error) => problems.push(error.message));
    watching.on("console", (message) => {
      const apiRefusal = /status of 4\d\d/.test(message.text()) && message.location().url.includes("/api/v1/");
      if (message.type() === "error" && !apiRefusal) {
        problems.push(message.text());
      }
    });

    await signIn(watching, teammate);
    await watching.goto(`/d/inbox/all/${id}`);
    await expect(watching.getByRole("heading", { name: "Can't log in" })).toBeVisible();
    await expect(watching.locator('[data-realtime="connected"]')).toBeAttached();

    await page.goto(`/d/inbox/all/${id}`);
    await expect(page.getByRole("status", { name: /Ada Lovelace/ })).toBeVisible();
    await openComposer(page, /Reply to Emily Johnson/);
    await page.getByRole("textbox", { name: "Reply to Emily Johnson" }).fill("Try a sign-in code instead.");
    await page.getByRole("button", { name: "Send", exact: true }).click();

    await expect(watching.getByRole("article", { name: `Message from ${person.name}` })).toContainText(
      "Try a sign-in code instead.",
    );
    expect(problems).toEqual([]);
    await other.close();
  });

  it("speaks to centrifuge-js from Node", async ({ person, request }) => {
    await createAccount(request, person, `Node ${suffixOf(person)}`);
    const conversation = startConversation(person.email);
    const cookie = await sessionCookie(request, person);
    const me = await request.get("/api/v1/me");
    const workspace: string = (await me.json()).data.workspaces[0].id;

    const client = new Centrifuge(`${base.replace("http", "ws")}/connection/websocket`, {
      websocket: socketWithCookie(cookie),
      minReconnectDelay: 100,
    });
    const events: string[] = [];
    const subscriptions: SubscribedContext[] = [];

    const lists = client.newSubscription(`workspace:${workspace}`);
    lists.on("publication", (ctx: PublicationContext) => events.push(ctx.data.event));
    lists.on("subscribed", (ctx) => subscriptions.push(ctx));
    const thread = client.newSubscription(`conversation:${conversation}`);
    lists.subscribe();
    thread.subscribe();
    client.connect();

    await lists.ready(5000);
    await thread.ready(5000);
    const presence = await thread.presence();
    expect(Object.values(presence.clients).map((info) => info.connInfo.name)).toEqual([person.name]);

    await request.patch("/api/v1/workspace", { data: { name: `Live ${suffixOf(person)}` } });
    await expect.poll(() => events).toEqual(["workspace.updated"]);

    client.disconnect();
    await request.patch("/api/v1/workspace", { data: { name: `Missed ${suffixOf(person)}` } });
    client.connect();

    await expect.poll(() => events).toEqual(["workspace.updated", "workspace.updated"]);
    await expect.poll(() => subscriptions.at(-1)?.recovered).toBe(true);
    client.disconnect();
  });
});
