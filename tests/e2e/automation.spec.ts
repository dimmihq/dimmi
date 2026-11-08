import type { Page } from "@playwright/test";

import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

/*
 * workspaceOf names the person's workspace after them, so workspaces
 * created at the same moment never race for one address.
 */
function workspaceOf(person: Person): string {
  return `Automation ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

/*
 * newWorkflow makes a workflow from the trigger the search finds and
 * waits for its builder.
 */
async function newWorkflow(page: Page, search: string, trigger: RegExp) {
  await page.goto("/d/workflows");
  await page.getByRole("button", { name: "New workflow" }).click();
  const picker = page.getByRole("dialog", { name: "What starts this workflow?" });
  await picker.getByRole("textbox", { name: "Search triggers" }).fill(search);
  await picker.getByRole("button", { name: trigger }).click();
  await expect(page).toHaveURL(/\/d\/[^/]+\/workflows\/wf_[^/]+$/);
  await expect(page.getByRole("region", { name: "Workflow" }).getByRole("button", { name: /Trigger/ })).toBeVisible();
}

it.describe("automation", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
  });

  it("makes a workflow from a trigger, builds it, saves it and publishes it", async ({ page }) => {
    await newWorkflow(page, "first message", /Customer sends their first message/);
    await expect(page.getByRole("heading", { name: "Untitled workflow" })).toBeVisible();
    const inspector = page.getByRole("region", { name: "Step settings" });
    await expect(inspector.getByRole("region", { name: "When it starts" })).toContainText(
      "Customer sends their first message",
    );

    const canvas = page.getByRole("region", { name: "Workflow" });
    await canvas.getByRole("button", { name: "Add a step" }).click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Reply buttons" }).click();
    await expect(inspector.getByRole("heading", { name: "Reply buttons" })).toBeVisible();
    await inspector.getByRole("textbox", { name: "Question" }).fill("What is it about?");
    await inspector.getByRole("textbox", { name: "Button 1" }).fill("Billing");
    await expect(canvas.getByRole("button", { name: /What is it about\?/ })).toBeVisible();
    await expect(canvas.getByText("Billing", { exact: true })).toBeVisible();
    await expect(page.getByText("Saved as draft")).toBeVisible();

    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByRole("button", { name: "Pause" })).toBeVisible();
    await expect(page.getByText("Published", { exact: true })).toBeVisible();

    await page.reload();
    await expect(canvas.getByRole("button", { name: /What is it about\?/ })).toBeVisible();
    await page.getByRole("link", { name: "Back to workflows" }).click();
    await expect(page.getByRole("link", { name: /Untitled workflow/ })).toContainText("Live");
  });

  it("marks what needs fixing and keeps it from being published", async ({ page }) => {
    await newWorkflow(page, "every message", /Customer sends a message/);
    const canvas = page.getByRole("region", { name: "Workflow" });
    await canvas.getByRole("button", { name: "Add a step" }).click();
    const menu = page.getByRole("dialog", { name: "Add a step" });
    await expect(menu.getByRole("button", { name: "Send a message" })).toHaveCount(0);
    await menu.getByRole("button", { name: "Tag", exact: true }).click();

    await page.getByRole("button", { name: "1 to fix" }).click();
    const inspector = page.getByRole("region", { name: "Step settings" });
    await expect(inspector).toContainText("Pick 1 to 20 tags.");
    await expect(page.getByRole("button", { name: "Publish", exact: true })).toBeDisabled();
  });

  it("branches on a condition picked in the inspector and publishes it", async ({ page }) => {
    await newWorkflow(page, "every message", /Customer sends a message/);
    const canvas = page.getByRole("region", { name: "Workflow" });
    await canvas.getByRole("button", { name: "Add a step" }).click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Branch" }).click();
    await expect(page.getByRole("button", { name: "1 to fix" })).toBeVisible();

    const inspector = page.getByRole("region", { name: "Step settings" });
    await inspector.getByRole("button", { name: /Condition field/ }).click();
    await page.getByRole("option", { name: "Message text" }).click();
    await inspector.getByRole("textbox", { name: "Condition value" }).fill("refund, money back");
    await expect(page.getByRole("button", { name: "1 to fix" })).toBeHidden();
    await expect(canvas.getByText("Everyone else", { exact: true })).toBeVisible();

    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByRole("button", { name: "Pause" })).toBeVisible();
  });

  it("fits a workflow on screen and keeps a new step in view", async ({ page }) => {
    await newWorkflow(page, "first message", /Customer sends their first message/);
    const canvas = page.getByRole("region", { name: "Workflow" });
    const frame = canvas.locator(".react-flow");
    await canvas.getByRole("button", { name: "Add a step" }).click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Reply buttons" }).click();

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
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Tag", exact: true }).click();
    await expect(canvas.locator('button[aria-pressed="true"]')).toBeInViewport({ ratio: 0.99 });
  });

  it("finds triggers by searching and offers only those that run", async ({ page }) => {
    await page.goto("/d/workflows");
    await page.getByRole("button", { name: "New workflow" }).click();

    const picker = page.getByRole("dialog", { name: "What starts this workflow?" });
    await expect(picker.getByRole("region", { name: "Team" })).toBeVisible();
    await expect(picker.getByRole("region", { name: "dimmi" })).toBeHidden();
    await picker.getByRole("textbox", { name: "Search triggers" }).fill("status");
    await expect(picker.getByRole("region", { name: "Start of conversation" })).toBeHidden();
    await picker.getByRole("textbox", { name: "Search triggers" }).press("Enter");

    await expect(page).toHaveURL(/\/d\/[^/]+\/workflows\/wf_[^/]+$/);
    await expect(page.getByRole("region", { name: "When it starts" })).toContainText("Conversation status changes");
  });

  it("edits a procedure and keeps tests from spending without asking", async ({ page }) => {
    await page.goto("/d/procedures");
    await page.getByRole("link", { name: /Send a sign-in code/ }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/procedures\/sign-in-code$/);

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
