import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

/*
 * workspaceOf names the person's workspace after them, so workspaces
 * created at the same moment never race for one address.
 */
function workspaceOf(person: Person): string {
  return `Automation ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

it.describe("automation", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
  });

  it("opens a workflow from the list and shows the selected step's settings", async ({ page }) => {
    await page.goto("/d/workflows");
    await page.getByRole("link", { name: /Let Dimmi answer first messages/ }).click();
    await expect(page).toHaveURL(/\/d\/workflows\/first-message$/);
    await expect(page.getByRole("button", { name: "Expand sidebar" })).toBeVisible();

    const inspector = page.getByRole("region", { name: "Step settings" });
    await expect(inspector.getByRole("heading", { name: "Trigger" })).toBeVisible();

    await page.getByRole("button", { name: /Let Dimmi handle/ }).click();
    await expect(inspector.getByRole("heading", { name: "Let Dimmi handle" })).toBeVisible();
    await expect(inspector).toContainText("stops once a teammate replies");
  });

  it("adds a step to a path, removes it, and publishes a draft", async ({ page }) => {
    await page.goto("/d/workflows/vip");
    const canvas = page.getByRole("region", { name: "Workflow" });
    const cards = canvas.locator("button[aria-pressed]");
    await expect(canvas.getByRole("button", { name: /Trigger/ })).toBeVisible();
    const before = await cards.count();

    await canvas.getByRole("button", { name: "Add a step" }).first().click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Wait" }).click();
    await expect(cards).toHaveCount(before + 1);
    await expect(
      page.getByRole("region", { name: "Step settings" }).getByRole("heading", { name: "Wait" }),
    ).toBeVisible();

    await page.getByRole("region", { name: "Step settings" }).getByRole("button", { name: "Remove" }).click();
    await expect(cards).toHaveCount(before);

    await expect(page.getByText("Version 1")).toBeVisible();
    await page.getByRole("button", { name: "Publish" }).click();
    await expect(page.getByText("Version 2")).toBeVisible();
    await expect(page.getByRole("button", { name: "Pause" })).toBeVisible();
  });

  it("fits a wide workflow on screen and keeps a new step in view", async ({ page }) => {
    await page.goto("/d/workflows/first-message");
    const canvas = page.getByRole("region", { name: "Workflow" });
    const frame = canvas.locator(".react-flow");
    await expect(canvas.getByRole("button", { name: /Trigger/ })).toBeVisible();

    await canvas.getByRole("button", { name: "Fit to screen" }).click();
    await expect
      .poll(async () => {
        const bounds = await frame.boundingBox();
        const boxes = await canvas
          .locator("button[aria-pressed]")
          .evaluateAll((cards) => cards.map((card) => card.getBoundingClientRect().toJSON() as DOMRect));
        return boxes.every(
          (box) =>
            bounds !== null &&
            box.left >= bounds.x &&
            box.top >= bounds.y &&
            box.right <= bounds.x + bounds.width &&
            box.bottom <= bounds.y + bounds.height,
        );
      })
      .toBe(true);

    await canvas.getByRole("button", { name: "Zoom in" }).click();
    await canvas.getByRole("button", { name: "Zoom in" }).click();
    await canvas.getByRole("button", { name: "Add a step" }).last().click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Tag" }).click();
    await expect(canvas.locator('button[aria-pressed="true"]')).toBeInViewport({ ratio: 1 });
  });

  it("shows what is typed and picked in the settings on the step's card", async ({ page }) => {
    await page.goto("/d/workflows/first-message");
    const canvas = page.getByRole("region", { name: "Workflow" });
    const inspector = page.getByRole("region", { name: "Step settings" });

    await canvas.getByRole("button", { name: /What can we help you with\?/ }).click();
    await inspector.getByRole("textbox", { name: "Question" }).fill("How can we help today?");
    await inspector.getByRole("textbox", { name: "Button 1" }).fill("Invoices");
    await expect(canvas.getByRole("button", { name: /How can we help today\?/ })).toBeVisible();
    await expect(canvas.getByText("Invoices", { exact: true })).toBeVisible();

    await canvas.getByRole("button", { name: /Assign to the Billing team/ }).click();
    await inspector.getByRole("button", { name: /Assignee/ }).click();
    await page.getByRole("option", { name: "Lea Martin" }).click();
    await expect(canvas.getByRole("button", { name: /Assign to Lea Martin/ })).toBeVisible();
  });

  it("edits a procedure and keeps tests from spending without asking", async ({ page }) => {
    await page.goto("/d/procedures");
    await page.getByRole("link", { name: /Send a sign-in code/ }).click();
    await expect(page).toHaveURL(/\/d\/procedures\/sign-in-code$/);

    const steps = page.getByRole("region", { name: "Steps" });
    await expect(steps.getByRole("textbox")).toHaveCount(5);
    await steps.getByRole("button", { name: "Condition" }).click();
    await expect(steps.getByRole("textbox")).toHaveCount(6);

    const tests = page.getByRole("region", { name: "Tests" });
    await expect(tests.getByText("Failed")).toBeVisible();
    await tests.getByRole("button", { name: "Run tests" }).click();
    await expect(tests).toContainText("Tests run once Dimmi answers customers.");
  });
});
