import type { Page } from "@playwright/test";

import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

const pages = [
  "/d/workflows",
  "/d/procedures",
  "/d/settings/profile",
  "/d/settings/security",
  "/d/settings/connections",
  "/d/settings/messenger",
  "/d/settings/agent",
  "/d/settings/ai-providers",
  "/d/settings/workspace",
  "/d/settings/members",
  "/d/settings/billing",
];

const sizes = [
  { name: "phone", width: 390, height: 844 },
  { name: "tablet", width: 900, height: 1000 },
];

/*
 * workspaceOf names the person's workspace after them, so workspaces
 * created at the same moment never race for one address.
 */
function workspaceOf(person: Person): string {
  return `Responsive ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

/*
 * spills lists the elements that reach past the right edge of the
 * screen, leaving out the workflow canvas, whose cards pan out of view
 * on purpose.
 */
function spills(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    [...document.querySelectorAll("main *")]
      .filter((element) => {
        const box = element.getBoundingClientRect();
        return box.width > 0 && box.right > window.innerWidth + 1 && !element.closest(".react-flow");
      })
      .slice(0, 3)
      .map((element) => element.textContent?.trim().slice(0, 40) ?? element.tagName),
  );
}

it.describe("small screens", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
  });

  for (const size of sizes) {
    it(`keeps every automation and settings page inside a ${size.name}`, async ({ page }) => {
      await page.setViewportSize({ width: size.width, height: size.height });
      for (const path of pages) {
        await page.goto(path);
        await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
        expect(await spills(page), path).toEqual([]);
      }
    });
  }

  it("opens the workflow inspector in a drawer on a phone", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/d/workflows");
    await page.getByRole("button", { name: "New workflow" }).click();
    const picker = page.getByRole("dialog", { name: "What starts this workflow?" });
    await picker.getByRole("button", { name: /Customer sends their first message/ }).click();
    await expect(page).toHaveURL(/\/workflows\/wf_[^/]+$/);
    await expect(page.getByRole("region", { name: "Step settings" })).toBeHidden();

    await page.getByRole("button", { name: /Trigger/ }).click();
    const drawer = page.getByRole("dialog", { name: "Step settings" });
    await expect(drawer.getByRole("heading", { name: "Trigger" })).toBeVisible();
    await drawer.getByRole("button", { name: "Close step settings" }).click();
    await expect(drawer).toBeHidden();
  });

  it("opens the procedure settings in a drawer on a tablet", async ({ page }) => {
    await page.setViewportSize({ width: 900, height: 1000 });
    await page.goto("/d/procedures");
    await page.getByRole("button", { name: "New procedure" }).click();
    await expect(page).toHaveURL(/\/procedures\/prc_[^/]+$/);
    await page.getByRole("button", { name: "Procedure settings" }).click();
    const drawer = page.getByRole("dialog", { name: "Procedure settings" });
    await expect(drawer.getByText("Delete this procedure")).toBeVisible();
  });
});
