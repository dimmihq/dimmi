import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("home", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Home ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("welcomes a new workspace with its first steps and what each block will hold", async ({ page }) => {
    await page.getByRole("link", { name: "Home", exact: true }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/home$/);
    await expect(page.getByRole("heading", { level: 1, name: "Home" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Home", exact: true })).toHaveAttribute("aria-current", "page");

    await expect(page.getByRole("heading", { name: "Welcome to dimmi." })).toBeVisible();
    const setup = page.getByRole("region", { name: "Get set up" });
    await expect(setup.getByRole("link", { name: /Install the messenger/ })).toBeVisible();
    await expect(setup.getByRole("link", { name: /Connect your email/ })).toBeVisible();

    const queue = page.getByRole("region", { name: "Needs a person now" });
    await expect(queue.getByRole("link", { name: /Unassigned/ })).toContainText("0");
    await expect(queue.getByText("Nobody is waiting.")).toBeVisible();

    const operator = page.getByRole("region", { name: "Put Operator to work" });
    await expect(operator.getByRole("link", { name: /Unhappy customer watch/ })).toBeVisible();
    await expect(page.getByRole("region", { name: "Waiting for your review" })).toHaveCount(0);
  });

  it("opens the first steps beside the sidebar, with why each matters", async ({ page }) => {
    await page.goto("/d/home");
    const trigger = page.getByRole("button", { name: /Get started/ });
    await expect(trigger).toContainText("/5");
    await expect(trigger.getByRole("progressbar", { name: "Setup progress" })).toBeVisible();

    await trigger.click();
    const steps = page.getByRole("dialog", { name: "Get started" });
    await expect(steps.getByText(/of 5 done/)).toBeVisible();
    await expect(steps.getByRole("link", { name: /Install the messenger/ })).toContainText(
      "Customers write to you from your site or app.",
    );

    await steps.getByRole("link", { name: /Add knowledge for the agent/ }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/settings\/knowledge$/);
    await expect(steps).toHaveCount(0);
  });

  it("opens the page a row names", async ({ page }) => {
    await page.goto("/d/home");

    await page
      .getByRole("region", { name: "Get set up" })
      .getByRole("link", { name: /Connect your email/ })
      .click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/settings\/email$/);

    await page.goto("/d/home");
    await page
      .getByRole("region", { name: "Needs a person now" })
      .getByRole("link", { name: /Unassigned/ })
      .click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/inbox\/unassigned/);

    await page.goto("/d/home");
    await page
      .getByRole("region", { name: "Put Operator to work" })
      .getByRole("link", { name: /Knowledge gap finder/ })
      .click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/routines\/new\?template=tpl_knowledge_gap_finder/);
  });
});
