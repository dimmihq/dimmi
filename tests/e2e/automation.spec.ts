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

  it("lets dimmi handle a conversation with a path for each way its turn ends", async ({ page }) => {
    await newWorkflow(page, "first message", /Customer sends their first message/);
    const canvas = page.getByRole("region", { name: "Workflow" });
    await canvas.getByRole("button", { name: "Add a step" }).click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Let dimmi handle" }).click();

    const inspector = page.getByRole("region", { name: "Step settings" });
    await expect(inspector.getByRole("region", { name: "dimmi is off" })).toBeVisible();
    for (const path of ["Resolved", "Handed over", "Customer went quiet"]) {
      await expect(canvas.getByText(path, { exact: true })).toBeVisible();
    }

    await inspector.getByRole("textbox", { name: "Extra instructions" }).fill("Only answer billing questions.");
    await expect(canvas.getByRole("button", { name: /following: Only answer billing questions/ })).toBeVisible();
    await expect(page.getByText("Saved as draft")).toBeVisible();
    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByRole("button", { name: "Pause" })).toBeVisible();
  });

  it("waits out a quiet customer until office hours open, and publishes", async ({ page }) => {
    await newWorkflow(page, "stops replying", /Customer stops replying/);
    const inspector = page.getByRole("region", { name: "Step settings" });
    await expect(inspector.getByRole("region", { name: "How long the silence lasts" })).toBeVisible();

    const canvas = page.getByRole("region", { name: "Workflow" });
    await canvas.getByRole("button", { name: "Add a step" }).click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Wait" }).click();
    await inspector.getByRole("button", { name: /Wait until/ }).click();
    await page.getByRole("option", { name: "Office hours to open" }).click();
    await expect(canvas.getByRole("button", { name: /Wait until office hours open/ })).toBeVisible();
    await expect(page.getByText("Saved as draft")).toBeVisible();

    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByRole("button", { name: "Pause" })).toBeVisible();
  });

  it("opens Saturday in office hours and keeps it", async ({ page }) => {
    await page.goto("/d/settings/office-hours");
    await expect(page.getByText(/Open now|Closed now/)).toBeVisible();
    await page.getByRole("switch", { name: "Open on Saturday" }).focus();
    await page.keyboard.press("Space");
    await expect(page.getByRole("switch", { name: "Open on Saturday" })).toBeChecked();
    // The bar stays while the save runs, so wait for the save itself.
    const saved = page.waitForResponse((r) => r.request().method() === "PATCH" && /\/schedules\//.test(r.url()));
    await page.getByRole("button", { name: "Save", exact: true }).click();
    expect((await saved).ok()).toBe(true);
    await expect(page.getByRole("button", { name: "Save", exact: true })).toHaveCount(0);

    await page.reload();
    await expect(page.getByRole("group", { name: "Saturday opens" })).toHaveText(/09\D*00/);
  });

  it("adds a yearly holiday and a one-off one in office hours and keeps both", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1400 });
    await page.goto("/d/settings/office-hours");
    await expect(page.getByText(/Open now|Closed now/)).toBeVisible();

    await addHoliday(page, "25/12/2026", "Christmas");
    await expect(page.getByText("Dec 25, every year")).toBeVisible();

    await addHoliday(page, "03/03/2027", "Offsite", { once: true });
    await expect(page.getByText("Mar 3, 2027")).toBeVisible();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("button", { name: "Save", exact: true })).toHaveCount(0);

    await page.reload();
    await expect(page.getByText("Christmas")).toBeVisible();
    await expect(page.getByText("Dec 25, every year")).toBeVisible();
    await expect(page.getByText("Offsite")).toBeVisible();
    await expect(page.getByText("Mar 3, 2027")).toBeVisible();
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
        const boxes = await frame
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
    await expect(frame.locator('button[aria-pressed="true"]')).toBeInViewport({ ratio: 0.99 });
  });

  it("finds triggers by searching and offers only those that run", async ({ page }) => {
    await page.goto("/d/workflows");
    await page.getByRole("button", { name: "New workflow" }).click();

    const picker = page.getByRole("dialog", { name: "What starts this workflow?" });
    await expect(picker.getByRole("region", { name: "Team" })).toBeVisible();
    await expect(picker.getByRole("region", { name: "dimmi" })).toContainText("dimmi hands over");
    await expect(picker.getByText("Customer opens the messenger")).toBeHidden();
    await expect(picker.getByText("Customer rates the conversation")).toBeHidden();
    await picker.getByRole("textbox", { name: "Search triggers" }).fill("status");
    await expect(picker.getByRole("region", { name: "Start of conversation" })).toBeHidden();
    await picker.getByRole("textbox", { name: "Search triggers" }).press("Enter");

    await expect(page).toHaveURL(/\/d\/[^/]+\/workflows\/wf_[^/]+$/);
    await expect(page.getByRole("region", { name: "When it starts" })).toContainText("Conversation status changes");
  });

  it("writes a procedure, shows what stops it, and publishes it", async ({ page }) => {
    await page.goto("/d/procedures");
    await page.getByRole("button", { name: "New procedure" }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/procedures\/prc_[^/]+$/);

    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByText("Say when dimmi should use this, in 500 characters.")).toBeVisible();

    await page.getByRole("textbox", { name: "When dimmi uses this" }).fill("The customer cannot sign in.");
    await page.getByRole("button", { name: "Instruction", exact: true }).click();
    await page.getByRole("textbox", { name: "Instruction 1" }).fill("Ask which email address they sign in with.");
    await page.getByRole("button", { name: "Condition", exact: true }).click();
    await page.getByRole("textbox", { name: "Condition 2" }).fill("If it is a work address, ask them to check spam.");
    await page.getByRole("button", { name: "Move step 2 up" }).click();
    await expect(page.getByRole("textbox", { name: "Condition 1" })).toBeVisible();
    await expect(page.getByText("Saved as draft")).toBeVisible();

    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByRole("button", { name: "Turn off" })).toBeVisible();
    await page.reload();
    await expect(page.getByRole("textbox", { name: "Condition 1" })).toHaveValue(
      "If it is a work address, ask them to check spam.",
    );
    await page.getByRole("link", { name: "Back to procedures" }).click();
    await expect(page.getByRole("link", { name: /Untitled procedure/ })).toContainText("Live");
  });
});

/*
 * addHoliday picks a day in the holiday date picker, types the date as
 * "DD/MM/YYYY", names it and adds it, yearly unless asked for once.
 */
async function addHoliday(page: Page, date: string, name: string, options: { once?: boolean } = {}) {
  await page.getByRole("button", { name: "Select date" }).click();
  const picker = page.getByRole("dialog", { name: "Holiday date" });
  await picker
    .getByRole("button", { name: /\b15\b/ })
    .first()
    .click();
  await picker.getByRole("textbox", { name: "Date" }).fill(date);
  await picker.getByRole("textbox", { name: "Date" }).press("Enter");
  await picker.getByRole("button", { name: "Apply" }).click();
  await expect(picker).toBeHidden();
  await page.getByRole("textbox", { name: "Holiday name" }).fill(name);
  const yearly = page.getByRole("checkbox", { name: "Repeats every year" });
  await expect(yearly).toBeChecked();
  if (options.once) {
    await page.getByText("Repeats every year").click();
  }
  await expect(yearly).toBeChecked({ checked: !options.once });
  const saved = page.waitForResponse(
    (r) => r.url().includes("/schedules/") && r.request().method() === "PATCH" && r.ok(),
  );
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await saved;
  await expect(page.getByRole("button", { name: "Select date" })).toBeVisible();
}
