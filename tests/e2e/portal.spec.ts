import { expect, test as it } from "./support/fixtures";

it.describe("the public portal", () => {
  it("moves between pages in place, without reloading the document", async ({ page }) => {
    await page.goto("/hc/dimmi");
    await expect(page.getByRole("heading", { level: 1 })).toContainText("How can we help?");
    await page.evaluate(() => {
      (window as unknown as { kept: boolean }).kept = true;
    });

    await page.getByRole("link", { name: "Getting started" }).first().click();
    await expect(page).toHaveURL(/\/hc\/dimmi\/collections\/getting-started$/);
    await page.getByRole("link", { name: "Invite your team" }).first().click();

    await expect(page.getByRole("heading", { level: 1, name: "Invite your team" })).toBeVisible();
    expect(await page.evaluate(() => (window as unknown as { kept?: boolean }).kept)).toBe(true);
  });

  it("browses from a drawer that pushes the page aside, and marks the heading being read", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/hc/dimmi/articles/write-articles-in-markdown");

    await page.getByLabel("Browse", { exact: true }).click();
    const drawer = page.getByRole("complementary", { name: "Browse articles" });
    await expect(drawer).toBeInViewport();
    await expect(page.locator("[data-page]")).toHaveCSS("translate", "320px");
    await expect(drawer.getByRole("button", { name: "Close" })).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(page.getByLabel("Browse", { exact: true })).toBeFocused();

    const links = page.getByRole("navigation", { name: "In this article" }).getByRole("link");
    await expect(links.first()).toHaveAttribute("aria-current", "location");
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    await expect(links.last()).toHaveAttribute("aria-current", "location");
    await expect(links.first()).not.toHaveAttribute("aria-current", "location");
  });

  it("closes the copy menu on a click outside it or Escape", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/hc/dimmi/articles/write-articles-in-markdown");
    const more = page.getByLabel("More ways to use this page");
    const view = page.getByRole("link", { name: /View as Markdown/ });

    await more.click();
    await expect(view).toBeVisible();
    await page.getByRole("heading", { level: 1 }).click();
    await expect(view).toBeHidden();

    await more.click();
    await expect(view).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(view).toBeHidden();
  });

  it("keeps the brand visible on both themes, whatever its color", async ({ page }) => {
    await page.goto("/hc/dimmi");
    const sample = async (brand: string, dark: boolean) =>
      page.evaluate(
        ([color, on]) => {
          const root = document.documentElement;
          root.style.setProperty("--brand", color as string);
          root.classList.toggle("dark", on as boolean);
          const probe = document.createElement("span");
          probe.className = "button-brand";
          document.body.append(probe);
          const fill = getComputedStyle(probe).backgroundColor;
          probe.remove();
          return fill;
        },
        [brand, dark] as const,
      );
    const lightness = (color: string) => Number(/oklch\(([\d.]+)/.exec(color)?.[1] ?? Number.NaN);

    expect(lightness(await sample("#0a0a0a", true))).toBeGreaterThan(0.9);
    expect(lightness(await sample("#ffffff", false))).toBeLessThan(0.25);
    expect(lightness(await sample("#1e3a8a", true))).toBeGreaterThanOrEqual(0.62);
    expect(lightness(await sample("#f59e0b", true))).toBeCloseTo(0.77, 1);
  });

  it("answers a question from the articles", async ({ page }) => {
    await page.goto("/hc/dimmi");

    await page.getByPlaceholder("Search for articles…").fill("custom domain");
    await page.getByRole("button", { name: "Search", exact: true }).click();

    await expect(page.getByRole("link", { name: /Use a custom domain/ })).toBeVisible();
  });

  it("thanks a reader who says an article helped", async ({ page }) => {
    await page.goto("/hc/dimmi/articles/invite-your-team");

    await page.getByRole("button", { name: "Yes" }).click();

    await expect(page.getByText("Thanks for letting us know.")).toBeVisible();
  });

  it("shows the changelog on a timeline and filters it by label", async ({ page }) => {
    await page.goto("/hc/dimmi/changelog");
    await expect(page.getByRole("navigation", { name: "Releases in 2026" })).toBeVisible();

    await page.getByRole("link", { name: "Fixed" }).first().click();

    await expect(page).toHaveURL(/\/changelog\/labels\/fixed$/);
    await expect(page.getByRole("heading", { name: "Fixes and polish" })).toBeVisible();
  });

  it("signs a visitor in to their conversations and out again", async ({ page }) => {
    await page.goto("/hc/dimmi/conversations");
    await expect(page).toHaveURL(/\/hc\/dimmi\/sign-in$/);

    await page.getByLabel("Email").fill("ada@example.com");
    await page.getByRole("button", { name: "Email me a sign-in link" }).click();
    await expect(page.getByRole("heading", { name: "Your conversations" })).toBeVisible();

    await page.getByRole("link", { name: /Webhook retries after a timeout/ }).click();
    await expect(page.getByText("retried for up to 24 hours")).toBeVisible();

    await page.getByLabel("Browse", { exact: true }).click();
    await page
      .getByRole("complementary", { name: "Your conversations" })
      .getByRole("button", { name: "Sign out" })
      .click();
    await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
  });
});
