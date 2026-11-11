import type { Page } from "@playwright/test";

import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { mailedText } from "./support/mail";

/*
 * openBoard opens the help center's board to customers from the
 * dashboard and returns the help center's address.
 */
async function openBoard(page: Page): Promise<string> {
  await page.goto("/d/feedback");
  await page.getByText("Open to customers").click();
  await expect(page.getByRole("link", { name: "View the board" })).toBeVisible();
  const centers = await api<{ subdomain: string }[]>(page, "GET", "/help-centers");
  return `/hc/${centers[0]?.subdomain}`;
}

/*
 * readerSignIn signs a customer in to the help center with an emailed
 * code, from the page they are on.
 */
async function readerSignIn(reader: Page, email: string) {
  await reader.getByLabel("Email").fill(email);
  await reader.getByRole("button", { name: "Email me a code" }).click();
  await expect(reader.getByRole("heading", { name: "Check your email" })).toBeVisible();
  const mailed = await mailedText(email, "your code for");
  await reader.getByLabel("Code").fill(mailed.match(/\b\d{6}\b/)?.[0] ?? "");
  await reader.getByRole("button", { name: "Sign in" }).click();
}

it.describe("feature requests", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Ideas ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("customers post and vote on the portal, and the team moves and answers requests", async ({
    page,
    person,
    guest,
  }) => {
    const site = await openBoard(page);
    const customer = `reader.${person.email}`;

    await guest.goto(`${site}/feedback`);
    await expect(guest.getByRole("heading", { name: "Feature requests" })).toBeVisible();
    await guest.getByRole("link", { name: "Submit idea" }).click();
    await readerSignIn(guest, customer);
    await expect(guest).toHaveURL(new RegExp(`${site}/feedback/new$`));
    await guest.getByLabel("Title").fill("Export conversations to CSV");
    await guest.getByLabel("Details").fill("For our monthly reports.");
    await guest.getByRole("button", { name: "Submit idea" }).click();
    await expect(guest.getByRole("heading", { name: "Export conversations to CSV" })).toBeVisible();
    await expect(guest.getByRole("button", { name: /Vote for Export conversations to CSV, 1 votes/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await guest.getByLabel("Add a comment").fill("We would use this every week.");
    await guest.getByRole("button", { name: "Comment" }).click();
    await expect(guest.getByText("We would use this every week.")).toBeVisible();

    await guest.goto(`${site}/feedback/new`);
    await guest.getByLabel("Title").fill("Export conversations as CSV");
    await guest.getByRole("button", { name: "Submit idea" }).click();
    await expect(guest.getByText("Is it one of these? Vote for it instead.")).toBeVisible();

    await page.goto("/d/feedback");
    const row = page
      .getByRole("row")
      .filter({ has: page.getByRole("rowheader", { name: /Export conversations to CSV/ }) });
    await expect(row).toContainText("Under consideration");
    await row.getByRole("rowheader").click();
    await expect(page.getByText("We would use this every week.")).toBeVisible();
    await page.locator("label", { hasText: "Internal note" }).click();
    await page.getByRole("textbox", { name: "Internal note" }).fill("Needs the export service first.");
    await page.getByRole("button", { name: "Add note" }).click();
    await expect(page.getByText("Needs the export service first.")).toBeVisible();

    await page.getByRole("button", { name: /Status/ }).click();
    await page.getByRole("option", { name: "Planned" }).click();
    const move = page.getByRole("dialog", { name: "Move to Planned" });
    await move.getByRole("textbox", { name: "Note" }).fill("Coming next month.");
    await move.getByRole("button", { name: "Move to Planned" }).click();
    await expect(page.getByText("Planned").first()).toBeVisible();
    const told = await mailedText(customer, "is now Planned");
    expect(told).toContain("Coming next month.");

    await guest.goto(`${site}/feedback`);
    const planned = guest.getByRole("region", { name: /Planned/ });
    await expect(planned.getByText("Export conversations to CSV")).toBeVisible();
    await guest.goto(`${site}/feedback/1-export-conversations-to-csv`);
    await expect(guest.getByText("Needs the export service first.")).toHaveCount(0);
  });

  it("holds customers' posts for approval and the team approves them", async ({ page, person, guest }) => {
    const site = await openBoard(page);
    await page.getByText("Approve new posts first").click();
    const customer = `waiting.${person.email}`;

    await guest.goto(`${site}/feedback/new`);
    await readerSignIn(guest, customer);
    await guest.getByLabel("Title").fill("Offline mode");
    await guest.getByRole("button", { name: "Submit idea" }).click();
    await expect(guest.getByText("The team will look at your idea before it appears here.")).toBeVisible();

    await page.goto("/d/feedback?view=pending");
    await page.getByRole("rowheader", { name: /Offline mode/ }).click();
    await page.getByRole("button", { name: "Approve" }).click();
    await expect(page.getByText("Waiting for approval")).toHaveCount(0);

    await guest.goto(`${site}/feedback?sort=new`);
    await expect(guest.getByText("Offline mode")).toBeVisible();
  });

  it("votes for a customer and merges a duplicate from the dashboard", async ({ page }) => {
    await openBoard(page);
    await api(page, "POST", "/contacts", { name: "Cy Diaz", email: "cy.diaz@example.org" });
    for (const title of ["Slack app", "Slack integration"]) {
      await page.getByRole("button", { name: "New request" }).click();
      const dialog = page.getByRole("dialog", { name: "New request" });
      await dialog.getByLabel("Title").fill(title);
      await dialog.getByRole("button", { name: "Post request" }).click();
      await expect(dialog).toHaveCount(0);
      await expect(page.getByRole("textbox", { name: "Title" })).toHaveValue(title);
      await page.goto("/d/feedback");
    }

    await page.getByRole("rowheader", { name: /Slack integration/ }).click();
    await page.getByRole("button", { name: "Vote for a customer" }).click();
    const vote = page.getByRole("dialog", { name: "Vote for a customer" });
    await vote.getByLabel("Customer").fill("cy.diaz");
    await vote.getByRole("button", { name: /Cy Diaz/ }).click();
    await vote.getByText("Must have").click();
    await vote.getByRole("button", { name: "Add vote" }).click();
    await expect(page.getByText("1 vote", { exact: true })).toBeVisible();
    await expect(page.getByText(/Must have/)).toBeVisible();

    await page.getByRole("button", { name: "Merge" }).click();
    const merge = page.getByRole("dialog", { name: "Merge into another request" });
    await merge.getByRole("button", { name: /Slack app/ }).click();
    await merge.getByRole("button", { name: "Merge" }).click();
    await expect(page.getByText("Merged into #1")).toBeVisible();
    await expect(page.getByText("1 vote", { exact: true })).toBeVisible();
  });

  it("reorders statuses from the keyboard and keeps the order after a reload", async ({ page }) => {
    await page.goto("/d/settings/feedback");
    const list = page.getByRole("grid", { name: "Statuses" });
    const names = () => list.getByRole("row").allInnerTexts();
    await expect(list.getByRole("row")).toHaveText([
      "In progress",
      "Planned",
      "Under consideration",
      "Shipped",
      "Closed",
    ]);

    const saved = page.waitForResponse(
      (r) => r.url().endsWith("/feedback-statuses") && r.request().method() === "PATCH",
    );
    await list.getByRole("button", { name: "Move In progress" }).focus();
    await page.keyboard.press("Enter");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Enter");
    expect((await saved).ok()).toBe(true);
    const moved = await names();
    expect(moved[0]).toBe("Planned");
    expect(moved.indexOf("In progress")).toBeGreaterThan(0);

    await page.reload();
    await expect(list.getByRole("row")).toHaveText(moved);

    await page.goto("/d/feedback");
    const views = page.getByRole("navigation", { name: "Feedback views" });
    await expect(views.getByRole("button").nth(1)).toContainText("Planned");
  });
});
