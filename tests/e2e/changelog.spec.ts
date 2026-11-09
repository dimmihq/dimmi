import { fileURLToPath } from "node:url";

import type { Page } from "@playwright/test";

import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

const picture = fileURLToPath(new URL("./files/screenshot.png", import.meta.url));

/*
 * newUpdate writes an update from the changelog page and opens it.
 */
async function newUpdate(page: Page, title: string) {
  await page.goto("/d/changelog");
  await page.getByRole("button", { name: "New update" }).click();
  const titleField = page.getByRole("textbox", { name: "Title" });
  await expect(titleField).toHaveValue("Untitled update");
  await titleField.fill(title);
}

it.describe("changelog", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Ships ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("writes an update with a cover, labels and authors, publishes it and readers see it", async ({ page, person }) => {
    await page.goto("/d/changelog");
    const views = page.getByRole("navigation", { name: "Changelog views" });
    await expect(views.getByRole("button", { name: "New", exact: true })).toBeVisible();
    await expect(page.getByText("No updates here yet. Write the first one.")).toBeVisible();
    await views.getByRole("button", { name: "New label" }).click();
    await page.getByRole("textbox", { name: "New label name" }).fill("Mobile");
    await page.keyboard.press("Enter");
    await expect(views.getByRole("button", { name: "Mobile", exact: true })).toBeVisible();

    await newUpdate(page, "Shared inboxes for every team");
    await page.getByRole("textbox", { name: "Summary" }).fill("One inbox per team.");
    const body = page.getByRole("textbox", { name: "Update" });
    await body.click();
    await page.keyboard.type("Teams grow, and one inbox stops working.");
    const chooser = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "Add cover" }).click();
    await (await chooser).setFiles(picture);
    await expect(page.getByRole("button", { name: "Change cover" })).toBeAttached();
    await page.getByRole("button", { name: "Labels" }).click();
    await page.getByRole("button", { name: "New", exact: true }).click();
    await page.keyboard.press("Escape");

    await page.getByRole("button", { name: "Authors" }).click();
    const authors = page.getByRole("dialog", { name: "Authors" });
    await authors.getByText("Pick authors").click();
    await authors.getByText(person.name).click();
    await authors.getByRole("button", { name: "Save authors" }).click();
    await expect(authors).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Authors" })).not.toContainText("from edits");

    await page.getByRole("button", { name: "Publish", exact: true }).click();
    const publish = page.getByRole("dialog", { name: "Publish update" });
    await expect(publish.getByText("Email subscribers (nobody has subscribed yet)")).toBeVisible();
    await publish.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByText("Update published")).toBeVisible();
    await expect(page.getByText("Published", { exact: true })).toBeVisible();

    await page.getByRole("link", { name: "Back to changelog" }).click();
    const row = page.getByRole("row", { name: /Shared inboxes for every team/ });
    await expect(row).toContainText("Published");
    await expect(row).toContainText("New");

    const centers = await api<{ subdomain: string }[]>(page, "GET", "/help-centers");
    const site = `/hc/${centers[0]?.subdomain}`;
    await page.goto(site);
    await expect(
      page.getByRole("navigation", { name: "Sections" }).getByRole("link", { name: "Changelog" }),
    ).toBeVisible();
    await page.goto(`${site}/changelog`);
    await expect(page.getByRole("heading", { name: "Shared inboxes for every team" })).toBeVisible();
    await expect(page.locator('img[src^="/hc/"][src*="/help-images/"]').first()).toBeVisible();
    await page.goto(`${site}/changelog/shared-inboxes-for-every-team`);
    await expect(page.getByText(person.name).first()).toBeVisible();
    await page.getByRole("button", { name: /Celebrate/ }).click();
    await expect(page.getByRole("button", { name: /Celebrate/ })).toContainText("1");

    await page.getByRole("textbox", { name: "Email address" }).first().fill("reader@example.org");
    await page.getByRole("button", { name: "Subscribe" }).first().click();
    await expect(page.getByText("Check your inbox to confirm your subscription.")).toBeVisible();

    const feed = await page.request.get(`${site}/changelog/feed.xml`);
    expect(await feed.text()).toContain("Shared inboxes for every team");
  });

  it("schedules an update, unpublishes one, and keeps both off the site", async ({ page }) => {
    await newUpdate(page, "Coming soon");
    await page.getByRole("button", { name: "Publish", exact: true }).click();
    const publish = page.getByRole("dialog", { name: "Publish update" });
    await publish.getByText("Schedule", { exact: true }).click();
    await publish.getByRole("button", { name: "Schedule", exact: true }).click();
    await expect(page.getByText("Update scheduled")).toBeVisible();
    await expect(page.getByText("Scheduled", { exact: true })).toBeVisible();

    await newUpdate(page, "Oops");
    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await page.getByRole("dialog", { name: "Publish update" }).getByRole("button", { name: "Publish" }).click();
    await expect(page.getByText("Update published")).toBeVisible();
    await page.getByRole("button", { name: "Unpublish" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Unpublish" }).click();
    await expect(page.getByText("Draft", { exact: true })).toBeVisible();

    await page.goto("/d/changelog?view=scheduled");
    await expect(page.getByRole("row", { name: /Coming soon/ })).toContainText("Scheduled");
    const centers = await api<{ subdomain: string }[]>(page, "GET", "/help-centers");
    const missing = await page.request.get(`/hc/${centers[0]?.subdomain}/changelog/oops`);
    expect(missing.status()).toBe(404);
  });

  it("lists subscribers and removes one", async ({ page }) => {
    await newUpdate(page, "First news");
    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await page.getByRole("dialog", { name: "Publish update" }).getByRole("button", { name: "Publish" }).click();
    await expect(page.getByText("Update published")).toBeVisible();
    const centers = await api<{ subdomain: string }[]>(page, "GET", "/help-centers");
    await page.goto(`/hc/${centers[0]?.subdomain}/changelog`);
    await page.getByRole("textbox", { name: "Email address" }).first().fill("waiting@example.org");
    await page.getByRole("button", { name: "Subscribe" }).first().click();
    await expect(page.getByText("Check your inbox to confirm your subscription.")).toBeVisible();

    await page.goto("/d/changelog?view=subscribers");
    const row = page.getByRole("row", { name: /waiting@example.org/ });
    await expect(row).toContainText("Waiting to confirm");
    await row.getByRole("button", { name: "Remove waiting@example.org" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Remove" }).click();
    await expect(row).toHaveCount(0);
  });
});
