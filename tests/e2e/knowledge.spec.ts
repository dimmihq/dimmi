import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";

import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

/*
 * pages is a tiny docs site rendered on the server: three pages that
 * link to each other, each with enough words to count as content.
 */
const pages: Record<string, { title: string; body: string }> = {
  "/": {
    title: "Welcome to the handbook",
    body: "This handbook explains how the product works, from your first sign in to your monthly invoice. Read the getting started guide first, then the billing guide when you are ready to pay.",
  },
  "/getting-started": {
    title: "Getting started",
    body: "Create a workspace, invite your teammates and connect your first inbox. Every teammate gets an email with a link that signs them in, and the workspace is ready as soon as they arrive.",
  },
  "/billing": {
    title: "Paying for your plan",
    body: "Plans are billed every month on the day you subscribed. You can change plans at any time; the difference is added to or taken from your next invoice, and receipts are emailed to the workspace owner.",
  },
};

function render(path: string): string {
  const page = pages[path];
  const links = Object.entries(pages)
    .map(([href, p]) => `<li><a href="${href}">${p.title}</a></li>`)
    .join("");

  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>${page?.title} | Handbook</title></head>
<body><nav><ul>${links}</ul></nav><main><article><h1>${page?.title}</h1>
<p>${page?.body}</p><p>${page?.body}</p><p>${page?.body}</p></article></main></body></html>`;
}

/*
 * serveSite starts the docs site on a free port of this machine, with a
 * robots.txt that lets every crawler in and a sitemap of the three pages.
 */
async function serveSite(): Promise<{ server: Server; origin: string }> {
  const server = createServer((req, res) => {
    const path = new URL(req.url ?? "/", "http://site").pathname;
    const origin = `http://${req.headers.host}`;
    if (path === "/robots.txt") {
      res.writeHead(200, { "content-type": "text/plain" });
      res.end(`User-agent: *\nAllow: /\nSitemap: ${origin}/sitemap.xml\n`);
      return;
    }
    if (path === "/sitemap.xml") {
      const urls = Object.keys(pages)
        .map((p) => `<url><loc>${origin}${p}</loc></url>`)
        .join("");
      res.writeHead(200, { "content-type": "application/xml" });
      res.end(
        `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${urls}</urlset>`,
      );
      return;
    }
    if (!pages[path]) {
      res.writeHead(404, { "content-type": "text/plain" });
      res.end("not found");
      return;
    }
    res.writeHead(200, { "content-type": "text/html; charset=utf-8" });
    res.end(render(path));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address() as AddressInfo;

  return { server, origin: `http://127.0.0.1:${port}` };
}

it.describe("knowledge sources", () => {
  let site: { server: Server; origin: string };

  it.beforeAll(async () => {
    site = await serveSite();
  });

  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Knowledge ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it.afterAll(async () => {
    site.server.closeAllConnections();
    await new Promise((resolve) => site.server.close(resolve));
  });

  it("adds a website source and lists its pages as live documents", async ({ page }) => {
    await page.goto("/d/settings/knowledge");
    await page.getByRole("button", { name: "Add source" }).first().click();

    const modal = page.getByRole("dialog");
    await modal.getByRole("button", { name: /^Website/ }).click();
    await modal.getByRole("textbox", { name: "Website address" }).fill(`${site.origin}/`);
    await modal.getByRole("button", { name: "Continue" }).click();

    await expect(modal.getByRole("listitem").filter({ hasText: "Content" })).toHaveAttribute("aria-current", "step");
    await expect(modal.getByRole("treegrid", { name: "Items to read" })).toBeVisible();
    await modal.getByRole("button", { name: "Continue" }).click();
    await expect(modal.getByRole("listitem").filter({ hasText: "Use" })).toHaveAttribute("aria-current", "step");
    await expect(modal.getByRole("checkbox", { name: /Agent/ })).toBeChecked();
    await modal.getByRole("button", { name: "Continue" }).click();
    await expect(modal.getByText("Everything it can read")).toBeVisible();
    await modal.getByRole("button", { name: "Add and sync" }).click();

    await expect(page).toHaveURL(/\/settings\/knowledge\/[^/]+$/);
    const first = page.getByRole("grid", { name: "Sync history" }).getByRole("row").filter({ hasText: "First sync" });
    await expect(first).toContainText("Succeeded", { timeout: 20_000 });
    await expect(first).toContainText("3 new");

    const documents = page.getByRole("grid", { name: "Documents" });
    for (const { title } of Object.values(pages)) {
      await expect(documents.getByRole("row", { name: new RegExp(title) })).toBeVisible({ timeout: 10_000 });
      await expect(documents.getByRole("row", { name: new RegExp(title) })).toContainText("Live");
    }
  });
});
