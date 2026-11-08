import type { Page } from "@playwright/test";

import { complexWorkflow } from "./support/complex-workflow";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

type Layout = Record<string, string>;

function workspaceOf(person: Person): string {
  return `Layout ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

/*
 * api calls the workspace API from the signed-in page.
 */
async function api(page: Page, method: string, path: string, body?: unknown): Promise<{ id: string }> {
  return page.evaluate(
    async ({ method, path, body }) => {
      const me = await (await fetch("/api/v1/me", { headers: { Accept: "application/json" } })).json();
      const response = await fetch(`/api/v1${path}`, {
        method,
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
          "Dimmi-Workspace": me.data.workspaces[0].id,
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      });

      return (await response.json()).data;
    },
    { method, path, body },
  );
}

/*
 * layout reads where every card and plus sits on the canvas, once
 * every card is measured and shown.
 */
async function layout(page: Page): Promise<Layout> {
  const nodes = page.locator(".react-flow__node");
  await expect(nodes.first()).toBeVisible();
  await expect.poll(() => nodes.evaluateAll((all) => all.every((node) => node.checkVisibility()))).toBe(true);

  return nodes.evaluateAll((all) =>
    Object.fromEntries(
      all.map((node) => [(node as HTMLElement).dataset.id ?? "", (node as HTMLElement).style.transform]),
    ),
  );
}

/*
 * settled waits for two reads a moment apart to agree, so a layout
 * still moving after a change is never taken as the answer.
 */
async function settled(page: Page): Promise<Layout> {
  let before = await layout(page);
  for (;;) {
    await page.waitForTimeout(300);
    const after = await layout(page);
    if (JSON.stringify(after) === JSON.stringify(before)) {
      return after;
    }
    before = after;
  }
}

/*
 * overlaps lists the pairs of cards whose boxes cross.
 */
function overlaps(page: Page): Promise<string[]> {
  return page.locator(".react-flow__node").evaluateAll((all) => {
    const boxes = all.map((node) => ({ id: (node as HTMLElement).dataset.id, box: node.getBoundingClientRect() }));
    const found: string[] = [];
    for (const [index, a] of boxes.entries()) {
      for (const b of boxes.slice(index + 1)) {
        const apart =
          a.box.right <= b.box.left ||
          b.box.right <= a.box.left ||
          a.box.bottom <= b.box.top ||
          b.box.bottom <= a.box.top;
        if (!apart) {
          found.push(`${a.id} × ${b.id}`);
        }
      }
    }

    return found;
  });
}

async function reloaded(page: Page): Promise<Layout> {
  await page.reload();

  return settled(page);
}

it.describe("workflow layout", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
  });

  it("puts every card of a workflow with many branches in the same place across reloads and edits", async ({
    page,
  }) => {
    await page.goto("/d/workflows");
    const team = await api(page, "POST", "/teams", { name: "Billing" });
    const tags = await Promise.all(["VIP", "Bug", "Refund"].map((name) => api(page, "POST", "/tags", { name })));
    const workflow = await api(page, "POST", "/workflows", {
      name: "Complex routing",
      spec: complexWorkflow(
        team.id,
        tags.map((tag) => tag.id),
      ),
    });

    await page.goto(`/d/workflows/${workflow.id}`);
    const first = await settled(page);
    expect(Object.keys(first)).toHaveLength(27);
    expect(await overlaps(page)).toEqual([]);
    expect(await reloaded(page)).toEqual(first);
    expect(await reloaded(page)).toEqual(first);

    const canvas = page.getByRole("region", { name: "Workflow" });
    const inspector = page.getByRole("region", { name: "Step settings" });
    await canvas.getByRole("button", { name: /A sales teammate will reply shortly/ }).click();
    await inspector
      .getByRole("textbox", { name: "Message" })
      .fill("A sales teammate will reply shortly. Meanwhile, tell us your company size and the plan you want.");
    await expect(page.getByText("Saved as draft")).toBeVisible();
    const grown = await settled(page);
    expect(grown).not.toEqual(first);
    expect(await overlaps(page)).toEqual([]);
    expect(await reloaded(page)).toEqual(grown);

    await canvas.getByRole("button", { name: /What can we help with/ }).click();
    const widened = page.waitForResponse((response) => response.request().method() === "PATCH");
    await inspector.getByRole("button", { name: "Add button" }).click();
    await widened;
    await expect(page.getByText("Saved as draft")).toBeVisible();
    const wider = await settled(page);
    expect(Object.keys(wider)).toHaveLength(28);
    expect(await overlaps(page)).toEqual([]);
    expect(await reloaded(page)).toEqual(wider);

    await canvas.getByRole("button", { name: /What can we help with/ }).click();
    const narrowed = page.waitForResponse((response) => response.request().method() === "PATCH");
    await inspector.getByRole("button", { name: "Remove" }).nth(4).click();
    await narrowed;
    await expect(page.getByText("Saved as draft")).toBeVisible();
    expect(await settled(page)).toEqual(grown);
    expect(await reloaded(page)).toEqual(grown);
  });
});
