/*
 * Conversation access end to end: an admin narrows what a teammate sees
 * through the API, and the teammate's browser shows only their team's
 * conversations, no Unassigned view, and the same no-access page for a
 * link to anything hidden, whether they opened it just now or had it
 * open when it moved away from them. An inbox restricted from its
 * settings page hides its conversations and itself from everyone not on
 * its list, and adding the teammate shows them again. A messenger added
 * from the inbox's page feeds that inbox.
 */
import type { Page } from "@playwright/test";

import { addTeammate } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, startConversation } from "./support/inbox";

const bea: Customer = {
  name: "Bea Billing",
  email: "bea@northwind.example",
  subject: "Invoice is wrong",
  body: "The VAT number on my invoice is wrong.",
};

const sam: Customer = {
  name: "Sam Support",
  email: "sam@contoso.example",
  subject: "Cannot log in",
  body: "My password is refused.",
};

function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

/*
 * Admin is the signed-in owner's page, calling the API in their
 * workspace.
 */
class Admin {
  private workspace = "";

  constructor(private readonly page: Page) {}

  private async headers() {
    if (!this.workspace) {
      const me = await this.page.request.get("/api/v1/me");
      this.workspace = (await me.json()).data.workspaces[0].id;
    }
    return { "Dimmi-Workspace": this.workspace };
  }

  async team(name: string): Promise<string> {
    const response = await this.page.request.post("/api/v1/teams", {
      headers: await this.headers(),
      data: { name },
    });
    expect(response.status(), await response.text()).toBe(201);
    return (await response.json()).data.id;
  }

  async patch(path: string, data: unknown) {
    const response = await this.page.request.patch(`/api/v1/${path}`, { headers: await this.headers(), data });
    expect(response.status(), await response.text()).toBe(200);
    return (await response.json()).data;
  }

  async post(path: string, data: unknown) {
    const response = await this.page.request.post(`/api/v1/${path}`, { headers: await this.headers(), data });
    expect(response.status(), await response.text()).toBe(201);
    return (await response.json()).data;
  }

  async memberId(email: string): Promise<string> {
    const response = await this.page.request.get("/api/v1/members", { headers: await this.headers() });
    const members = (await response.json()).data as { id: string; user: { email: string } }[];
    return members.find((member) => member.user.email === email)?.id ?? "";
  }
}

/*
 * saveAccess saves the inbox page and waits for the server to hold the
 * new access.
 */
async function saveAccess(page: Page) {
  const saved = page.waitForResponse((r) => r.url().endsWith("/access") && r.request().method() === "PUT");
  await page.getByRole("button", { name: "Save changes" }).click();
  expect((await saved).status()).toBe(200);
}

it.describe("conversation access", () => {
  it("shows a restricted teammate only their team's conversations", async ({ page, person, request, browser }) => {
    const teammate = teammateOf(person);
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Access ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, person);

    const admin = new Admin(page);
    const billing = await admin.team("Billing");
    const support = await admin.team("Support");
    const seen = await startConversation(page, bea);
    const hidden = await startConversation(page, sam);
    await admin.patch(`conversations/${seen}`, { team_id: billing });
    await admin.patch(`conversations/${hidden}`, { team_id: support });
    const member = await admin.memberId(teammate.email);
    await admin.patch(`members/${member}`, { conversation_access: "teams", access_team_ids: [billing] });

    const other = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const restricted = await other.newPage();
    const problems: string[] = [];
    restricted.on("pageerror", (error) => problems.push(error.message));
    restricted.on("console", (message) => {
      const apiRefusal = /status of 4\d\d/.test(message.text());
      if (message.type() === "error" && !apiRefusal) {
        problems.push(message.text());
      }
    });

    await signIn(restricted, teammate);
    await expect(restricted.locator('[data-realtime="connected"]')).toBeAttached();

    await restricted.locator("button[data-view-picker]").click();
    await expect(
      restricted.getByRole("dialog", { name: "Views" }).getByRole("button", { name: /Unassigned/ }),
    ).toHaveCount(0);
    await restricted.keyboard.press("Escape");

    await restricted.goto("/d/inbox/all");
    await expect(restricted.getByRole("link", { name: /Bea Billing/ })).toBeVisible();
    await expect(restricted.getByRole("link", { name: /Sam Support/ })).toHaveCount(0);

    await restricted.goto(`/d/inbox/all/${hidden}`);
    await expect(restricted.getByText("You do not have access to this conversation")).toBeVisible();
    await expect(restricted.getByText("Cannot log in")).toHaveCount(0);
    await restricted.getByRole("link", { name: "Go to my inbox" }).click();
    await expect(restricted).toHaveURL(/\/d\/[^/]+\/inbox\/mine/);

    await restricted.goto(`/d/inbox/all/${seen}`);
    await expect(restricted.getByRole("heading", { name: "Invoice is wrong" })).toBeVisible();

    await admin.patch(`conversations/${seen}`, { team_id: support });
    await expect(restricted.getByText("You do not have access to this conversation")).toBeVisible();
    await expect(restricted.getByRole("heading", { name: "Invoice is wrong" })).toHaveCount(0);

    await admin.patch(`members/${member}`, { conversation_access: "all" });
    await restricted.goto("/d/inbox/all");
    await restricted.locator("button[data-view-picker]").click();
    await expect(
      restricted.getByRole("dialog", { name: "Views" }).getByRole("button", { name: /Unassigned/ }),
    ).toBeVisible();
    await restricted.keyboard.press("Escape");
    await expect(restricted.getByRole("link", { name: /Sam Support/ })).toBeVisible();

    expect(problems).toEqual([]);
    await other.close();
  });

  it("keeps a restricted inbox to the teammates on its list", async ({ page, person, request, browser }) => {
    const teammate = teammateOf(person);
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Desks ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, person);

    const admin = new Admin(page);
    await startConversation(page, bea);
    const it = (await admin.post("inboxes", { name: "IT" })).id;
    const hidden = (
      await admin.post("conversations", {
        inbox_id: it,
        author: "contact",
        subject: sam.subject,
        body: sam.body,
        contact: { email: sam.email, name: sam.name },
      })
    ).id;

    await page.goto("/d/inbox/all");
    await expect(page.getByRole("link", { name: /Sam Support/ })).toContainText("IT");

    await page.goto(`/d/settings/inboxes/${it}`);
    const channels = page.getByRole("region", { name: "Channels" });
    await expect(channels.getByText("API", { exact: true })).toBeVisible();
    await channels.getByRole("button", { name: "Add messenger" }).click();
    await page.getByRole("alertdialog", { name: "Add a messenger" }).getByLabel("Name").fill("IT helpdesk");
    await page.getByRole("button", { name: "Add messenger" }).last().click();
    await expect(channels.getByText("IT helpdesk")).toBeVisible();
    await expect(page.getByRole("alertdialog")).toBeHidden();

    await page.getByRole("button", { name: "Who can open it" }).click();
    await page.getByRole("option", { name: "Chosen teams and teammates" }).click();
    await expect(page.getByText("only owners and admins can open it")).toBeVisible();
    await saveAccess(page);

    const other = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const restricted = await other.newPage();
    await signIn(restricted, teammate);
    await restricted.goto("/d/inbox/all");
    await expect(restricted.getByRole("link", { name: /Bea Billing/ })).toBeVisible();
    await expect(restricted.getByRole("link", { name: /Sam Support/ })).toHaveCount(0);
    await restricted.locator("button[data-view-picker]").click();
    await expect(restricted.getByRole("dialog", { name: "Views" }).getByRole("button", { name: /^IT/ })).toHaveCount(0);
    await restricted.keyboard.press("Escape");
    await restricted.goto(`/d/inbox/all/${hidden}`);
    await expect(restricted.getByText("You do not have access to this conversation")).toBeVisible();

    await page.getByRole("button", { name: "Add", exact: true }).click();
    await page.getByRole("dialog", { name: "Add to access" }).getByRole("button", { name: teammate.name }).click();
    await expect(page.getByRole("button", { name: `Remove ${teammate.name}` })).toBeVisible();
    await saveAccess(page);

    await restricted.goto("/d/inbox/all");
    await expect(restricted.getByRole("link", { name: /Sam Support/ })).toBeVisible();
    await other.close();
  });
});
