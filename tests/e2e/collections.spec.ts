import type { Page } from "@playwright/test";

import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

/*
 * tree is the collections pane's tree.
 */
function tree(page: Page) {
  return page.getByRole("region", { name: "Collections" }).getByRole("treegrid", { name: "Collections" });
}

/*
 * row is a collection's row in the collections pane.
 */
function row(page: Page, name: string) {
  return tree(page).getByRole("row", { name: new RegExp(`^${name}`) });
}

/*
 * name types a collection's name into the row being named.
 */
async function name(page: Page, value: string) {
  await page.getByRole("textbox", { name: /collection name/i }).fill(value);
  await page.keyboard.press("Enter");
}

it.describe("collections", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Shelves ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("adds, nests, reorders and files articles in collections from the pane", async ({ page }) => {
    await page.goto("/d/help-center");
    const pane = page.getByRole("region", { name: "Collections" });
    for (const value of ["Basics", "Billing", "Invoices"]) {
      await pane.getByRole("button", { name: "New collection" }).click();
      await name(page, value);
      await expect(row(page, value)).toBeVisible();
      await expect(row(page, value)).toHaveAttribute("aria-selected", "true");
    }
    await expect(page.getByRole("heading", { name: "Invoices", level: 1 })).toBeVisible();

    await row(page, "Billing").hover();
    await row(page, "Billing").getByRole("button", { name: "Add a collection inside Billing" }).click();
    await name(page, "Refunds");
    await expect(row(page, "Refunds")).toHaveAttribute("aria-level", "2");
    await row(page, "Refunds").hover();
    await row(page, "Refunds").getByRole("button", { name: "Add a collection inside Refunds" }).click();
    await name(page, "Cards");
    await expect(row(page, "Cards")).toHaveAttribute("aria-level", "3");
    await row(page, "Cards").hover();
    await expect(row(page, "Cards").getByRole("button", { name: /Add a collection inside/ })).toHaveCount(0);

    await pane.getByRole("button", { name: "New collection" }).click();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("textbox", { name: /collection name/i })).toHaveCount(0);

    await row(page, "Basics").getByRole("button", { name: "Actions for Basics" }).click();
    await page.getByRole("button", { name: "Rename", exact: true }).click();
    await name(page, "Basics and setup");
    await expect(row(page, "Basics and setup")).toBeVisible();

    await row(page, "Invoices").dragTo(row(page, "Billing"));
    await expect(row(page, "Invoices")).toHaveAttribute("aria-level", "2");
    await row(page, "Billing").dragTo(row(page, "Basics and setup"));
    await expect(row(page, "Billing")).toHaveAttribute("aria-level", "1");

    await row(page, "Billing").click();
    await expect(page).toHaveURL(/collection=/);
    await expect(page.getByRole("heading", { name: "Billing", level: 1 })).toBeVisible();
    await expect(row(page, "Billing")).toHaveAttribute("aria-selected", "true");
    await pane.getByRole("button", { name: "All articles" }).click();
    await expect(pane.getByRole("button", { name: "All articles" })).toHaveAttribute("aria-current", "page");

    await row(page, "Basics and setup").getByRole("button", { name: "Actions for Basics and setup" }).click();
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete collection" }).click();
    await expect(row(page, "Basics and setup")).toHaveCount(0);

    await page.setViewportSize({ width: 800, height: 900 });
    const tops = page.getByRole("navigation", { name: "Collections", exact: true });
    await tops.getByRole("button", { name: "Billing" }).click();
    const inside = page.getByRole("navigation", { name: "Collections inside" });
    await expect(inside.getByRole("button", { name: "Refunds" })).toBeVisible();
    await page.getByRole("button", { name: "Collections", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "Collections" }).getByRole("treegrid")).toBeVisible();
    await page.keyboard.press("Escape");
    await page.setViewportSize({ width: 1440, height: 900 });

    const centers = await api<{ id: string }[]>(page, "GET", "/help-centers");
    const article = await api<{ id: string }>(page, "POST", `/help-centers/${centers[0]?.id}/articles`, {
      title: "Card declined",
      body_markdown: "Try another card.",
    });
    await page.goto(`/d/help-center/${article.id}`);
    await page.getByRole("button", { name: "Choose a collection" }).click();
    const settings = page.getByRole("dialog", { name: "Article settings" });
    await settings.getByRole("button", { name: /Collection/ }).click();
    await page.getByRole("option", { name: "Cards" }).click();
    await expect(settings.getByRole("button", { name: /Collection/ })).toContainText("Billing / Refunds / Cards");
    await settings.getByRole("button", { name: "Done" }).click();
    await expect(page.getByRole("button", { name: "Billing / Refunds / Cards" })).toBeVisible();
  });
});
