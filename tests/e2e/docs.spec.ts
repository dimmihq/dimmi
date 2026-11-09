import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("docs", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Docs ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("writes and publishes an article readers can open and search", async ({ page }) => {
    await page.goto("/d/help-center");
    await expect(page.getByText("No articles here yet.")).toBeVisible();

    const pane = page.getByRole("region", { name: "Collections" });
    await pane.getByRole("button", { name: "New collection" }).click();
    await pane.getByRole("textbox", { name: "New collection name" }).fill("Basics");
    await page.keyboard.press("Enter");
    const tree = page.getByRole("region", { name: "Collections" }).getByRole("treegrid", { name: "Collections" });
    await expect(tree.getByRole("row", { name: /^Basics/ })).toBeVisible();

    await page.getByRole("button", { name: "New article" }).click();
    const dialog = page.getByRole("dialog", { name: "New article" });
    await expect(dialog.getByRole("button", { name: /Collection/ })).toContainText("Basics");
    await dialog.getByRole("textbox", { name: "Title" }).fill("Reset your password");
    await dialog.getByRole("button", { name: "Create article" }).click();

    await expect(page.getByText("Draft", { exact: true })).toBeVisible();
    const body = page.getByRole("textbox", { name: "Article" });
    await body.click();
    await page.keyboard.type("## Steps");
    await page.keyboard.press("Enter");
    await page.keyboard.type("Open **Sign in** and pick Forgot password.");
    await page.keyboard.press("Enter");
    await page.keyboard.type("/note");
    await page.getByRole("listbox", { name: "Blocks" }).getByRole("option", { name: /Note/ }).waitFor();
    await page.keyboard.press("Enter");
    await page.keyboard.type("Links expire after an hour.");
    await expect(body.locator("aside[data-callout].alert-note")).toBeVisible();
    await expect(body).not.toContainText("[!NOTE]");
    await expect(body.getByRole("heading", { name: "Steps" })).toBeVisible();
    await expect(body.locator("strong", { hasText: "Sign in" })).toBeVisible();
    await page.getByRole("button", { name: "Publish" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Publish" }).click();
    await expect(page.getByText("Published", { exact: true })).toBeVisible();

    await page.getByRole("button", { name: "Version history" }).click();
    const history = page.getByRole("region", { name: "Version history" }).or(page.getByLabel("Version history"));
    await expect(page.getByRole("list", { name: "Versions" })).toContainText("Version 1");
    await expect(history.first()).toBeVisible();

    const view = page.getByRole("link", { name: "View" });
    const href = (await view.getAttribute("href")) ?? "";
    expect(href).toMatch(/\/articles\/reset-your-password$/);
    const url = new URL(href);
    const path = url.pathname.startsWith("/hc/") ? url.pathname : `/hc/${url.hostname.split(".")[0]}${url.pathname}`;
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1, name: "Reset your password" })).toBeVisible();
    await expect(page.getByText("Forgot password")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Steps" })).toBeVisible();
    await expect(page.getByText("Links expire after an hour.")).toBeVisible();
    await expect(page.getByText("[!NOTE]")).toHaveCount(0);

    await page.goto(`${path.replace(/\/articles\/.*$/, "")}/search?q=password`);
    await expect(page.getByRole("link", { name: /Reset your password/ }).first()).toBeVisible();
  });
});
