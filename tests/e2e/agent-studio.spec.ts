import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("agent studio", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Studio ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("makes an agent from a template, publishes it and compares a change with live", async ({ page }) => {
    await page.goto("/d/agents");
    await expect(page.getByRole("heading", { level: 1, name: "Agents" })).toBeVisible();

    await page.getByRole("button", { name: "New agent" }).click();
    await page.getByRole("button", { name: /^Sales/ }).click();
    const details = page.getByRole("dialog", { name: "Sales" });
    await details.getByRole("textbox", { name: "Name" }).fill("Rio");
    await details.getByRole("button", { name: "Create agent" }).click();
    await expect(page).toHaveURL(/\/d\/[^/]+\/agents\/aia_/);

    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByText("Live", { exact: true }).first()).toBeVisible();

    const saved = page.waitForResponse(
      (response) => /\/api\/v1\/ai\/agents\/[^/]+$/.test(response.url()) && response.request().method() === "PATCH",
    );
    const name = page.getByRole("textbox", { name: "Agent name" });
    await name.fill("Rio Sales");
    await name.blur();
    await saved;
    await expect(page.getByRole("button", { name: "Publish changes" })).toBeVisible();

    await page.getByRole("tab", { name: "History" }).click();
    const compare = page.getByRole("region", { name: "Compare" });
    await expect(compare.getByRole("button", { name: "Compare from" })).toContainText("Version 1 (live)");
    await expect(compare.getByText("Name", { exact: true })).toBeVisible();
    await expect(compare.getByText("Rio Sales", { exact: true })).toBeVisible();

    await page.getByRole("tab", { name: "Analytics" }).click();
    await expect(page.getByText("No conversations yet")).toBeVisible();
  });

  it("filters the AI report to one agent", async ({ page }) => {
    await page.goto("/d/reports/ai");
    // The address gains the workspace's slug and the page mounts again: a
    // menu opened before that closes with the first mount.
    await expect(page).toHaveURL(/\/d\/[^/]+\/reports\/ai$/);
    await page.getByRole("button", { name: "Filter" }).click();
    const filters = page.getByRole("dialog", { name: "Filter the report" });
    await filters.getByRole("button", { name: "AI agent" }).click();
    await expect(filters.getByText("dimmi", { exact: true })).toBeVisible();
  });
});
