import type { Page } from "@playwright/test";

import { publish } from "./support/dns";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

/*
 * port is the server's, which names the folder the test DNS server reads.
 */
const port = process.env.E2E_PORT ?? "18090";

/*
 * domainOf is a domain only this person's test uses.
 */
function domainOf(person: Person): string {
  const suffix = person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
  return `acme-${suffix}.test`;
}

interface ApiRecord {
  purpose: string;
  type: string;
  name: string;
  value: string;
}

/*
 * recordsOf reads the records the open domain page shows from the API.
 */
async function recordsOf(page: Page, id: string): Promise<ApiRecord[]> {
  const response = await page.request.get(`/api/v1/email-domains/${id}`);
  expect(response.ok()).toBeTruthy();
  return ((await response.json()) as { data: { records: ApiRecord[] } }).data.records;
}

it.describe("sending domains", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("adds a domain, explains a doubled name and turns green once the records show", async ({
    page,
    person,
    request,
  }) => {
    const domain = domainOf(person);
    publish(port, domain, { NS: ["ada.ns.cloudflare.com"] });
    await createAccount(request, person, `Domains ${domain}`);
    await signIn(page, person);
    await page.goto("/d/settings/email");

    await page.getByRole("textbox", { name: "New sending domain" }).fill(domain);
    await page.getByRole("button", { name: "Add domain" }).click();

    await expect(page.getByRole("heading", { level: 1, name: domain })).toBeVisible();
    await expect(page.getByText("Not checked yet")).toBeVisible();
    await expect(page.getByRole("region", { name: "Set up at Cloudflare" })).toBeVisible();
    await expect(page.getByTestId("domain-record")).toHaveCount(3);
    await expect(page.getByTestId("domain-record").first()).toContainText("_dimmi-verification");

    const id = page.url().split("/").at(-1) ?? "";
    const records = await recordsOf(page, id);
    const ownership = records.find((r) => r.purpose === "ownership");
    const signing = records.find((r) => r.purpose === "dkim");
    if (!ownership || !signing) {
      throw new Error("the domain lacks its ownership or signing record");
    }

    publish(port, `${ownership.name}.${domain}`, { TXT: [ownership.value] });
    await page.getByRole("button", { name: "Check now" }).click();
    await expect(page.getByText(/Your DNS host adds your domain for you/)).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText("Checking", { exact: true })).toBeVisible();

    publish(port, ownership.name, { TXT: [ownership.value] });
    publish(port, signing.name, { TXT: [signing.value] });
    await page.waitForTimeout(11_000);
    await page.getByRole("button", { name: "Check now" }).click();

    await expect(page.getByText("Verified", { exact: true })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText("Found", { exact: true }).first()).toBeVisible();

    await page.getByRole("button", { name: "Email settings" }).click();
    const listed = page.getByRole("region", { name: "Sending domains" });
    await expect(listed.getByRole("link", { name: domain })).toBeVisible();
    await expect(listed.getByText("Verified", { exact: true })).toBeVisible();
  });

  it("removes a domain", async ({ page, person, request }) => {
    const domain = domainOf(person);
    await createAccount(request, person, `Remove ${domain}`);
    await signIn(page, person);
    await page.goto("/d/settings/email");

    await page.getByRole("textbox", { name: "New sending domain" }).fill(domain);
    await page.getByRole("button", { name: "Add domain" }).click();
    await expect(page.getByRole("heading", { level: 1, name: domain })).toBeVisible();

    await page.getByRole("button", { name: "Remove domain" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Remove domain" }).click();

    await expect(page.getByRole("heading", { level: 1, name: "Email" })).toBeVisible();
    await expect(page.getByRole("region", { name: "Sending domains" }).getByRole("link")).toHaveCount(0);
  });
});
