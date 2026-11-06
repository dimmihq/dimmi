/*
 * Loading states: with the API held back, every page keeps its shape
 * with skeletons and never claims to be empty before it knows.
 */
import type { Page } from "@playwright/test";

import { createAccount, expect, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

/*
 * holdBack delays every API response matching the pattern until the
 * returned release is called.
 */
async function holdBack(page: Page, pattern: RegExp): Promise<() => void> {
  let release: () => void = () => undefined;
  const gate = new Promise<void>((done) => {
    release = done;
  });
  await page.route(pattern, async (route) => {
    await gate;
    await route.continue();
  });

  return release;
}

it.describe("loading states", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await createAccount(request, person, `Loading ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
  });

  it("shows skeleton rows while the conversation list loads", async ({ page }) => {
    await startConversation(page, {
      name: "Ann Lee",
      email: "ann@example.com",
      subject: "Hello",
      body: "Hi there",
    });
    const release = await holdBack(page, /\/api\/v1\/conversations\?/);

    await page.goto("/d/inbox/all");
    await expect(page.getByRole("list", { name: "Loading conversations" })).toBeVisible();
    await expect(page.getByText("No conversations")).toHaveCount(0);

    release();
    await expect(page.getByRole("link", { name: /Ann Lee/ })).toBeVisible();
    await expect(page.getByRole("list", { name: "Loading conversations" })).toHaveCount(0);
  });

  it("shows skeleton rows while a settings list loads", async ({ page }) => {
    const release = await holdBack(page, /\/api\/v1\/teams(\?|$)/);

    await page.goto("/d/settings/teams");
    const teams = page.getByRole("region", { name: "Teams" });
    await expect(teams.locator("[aria-busy=true]")).toBeVisible();
    await expect(teams.getByText("No teams yet.")).toHaveCount(0);

    release();
    await expect(teams.getByText("No teams yet.")).toBeVisible();
  });
});
