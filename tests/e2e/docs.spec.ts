import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("docs", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Docs ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("writes and publishes an article readers can open and search", async ({ page }) => {
    await page.goto("/d/docs");
    await expect(page.getByText("No articles yet.")).toBeVisible();

    await page.getByRole("button", { name: "New collection" }).click();
    await page.getByRole("dialog", { name: "New collection" }).getByRole("textbox", { name: "Name" }).fill("Basics");
    await page.getByRole("button", { name: "Create collection" }).click();
    await expect(page.getByRole("button", { name: "Basics" })).toBeVisible();

    await page.getByRole("button", { name: "New article" }).click();
    const dialog = page.getByRole("dialog", { name: "New article" });
    await dialog.getByRole("textbox", { name: "Title" }).fill("Reset your password");
    await dialog.getByRole("button", { name: "Create article" }).click();

    await expect(page.getByText("Draft", { exact: true })).toBeVisible();
    await page.getByRole("textbox", { name: "Article" }).fill("## Steps\n\nOpen **Sign in** and pick Forgot password.");
    await page.getByRole("button", { name: "Publish" }).click();
    await expect(page.getByText("Published", { exact: true })).toBeVisible();

    const view = page.getByRole("link", { name: "View" });
    const href = (await view.getAttribute("href")) ?? "";
    expect(href).toMatch(/\/articles\/reset-your-password$/);
    const url = new URL(href);
    const path = url.pathname.startsWith("/hc/") ? url.pathname : `/hc/${url.hostname.split(".")[0]}${url.pathname}`;
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1, name: "Reset your password" })).toBeVisible();
    await expect(page.getByText("Forgot password")).toBeVisible();

    await page.goto(`${path.replace(/\/articles\/.*$/, "")}/search?q=password`);
    await expect(page.getByRole("link", { name: /Reset your password/ }).first()).toBeVisible();
  });
});
