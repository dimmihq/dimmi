import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("ai settings", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `AI ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("adds a provider through the modal and shows what is wrong", async ({ page }) => {
    await page.goto("/d/settings/ai-providers");
    await expect(page.getByRole("heading", { level: 1, name: "AI providers" })).toBeVisible();
    await expect(page.getByText("No providers yet")).toBeVisible();

    await page.getByRole("button", { name: "Add provider" }).click();
    const modal = page.getByRole("dialog", { name: "Add a provider" });
    await expect(modal.getByRole("button", { name: /Anthropic/ })).toBeVisible();
    await expect(modal.getByRole("button", { name: /OpenAI-compatible/ })).toBeVisible();
    await expect(modal.getByRole("button", { name: /Engy/ })).toHaveCount(0);

    await modal.getByRole("button", { name: /Anthropic/ }).click();
    const form = page.getByRole("dialog", { name: "Connect Anthropic" });
    await expect(form.getByRole("link", { name: "Get a key" })).toHaveAttribute(
      "href",
      "https://console.anthropic.com/settings/keys",
    );
    await form.getByRole("button", { name: "Test & save" }).click();
    await expect(form.getByText("Paste the API key.")).toBeVisible();

    await form.getByRole("button", { name: "All providers" }).click();
    await page
      .getByRole("dialog", { name: "Add a provider" })
      .getByRole("button", { name: /OpenAI-compatible/ })
      .click();
    const compatible = page.getByRole("dialog", { name: "Connect OpenAI-compatible" });
    await compatible.getByRole("textbox", { name: "Base URL" }).fill("llm.internal");
    await compatible.getByRole("button", { name: "Test & save" }).click();
    await expect(compatible.getByText("Enter the server's address, starting with https://.")).toBeVisible();

    await compatible.getByRole("button", { name: "Cancel" }).click();
    await expect(compatible).toBeHidden();
    await expect(page.getByText("No providers yet")).toBeVisible();
  });

  it("saves the agent's settings one change at a time", async ({ page }) => {
    await page.goto("/d/settings/agent");
    await expect(page.getByRole("heading", { level: 1, name: "dimmi agent" })).toBeVisible();
    await expect(page.getByRole("list", { name: "Setup" })).toContainText("Connect a provider");
    await expect(page.getByRole("switch", { name: "Answer customers" })).toBeDisabled();

    const saved = () =>
      page.waitForResponse(
        (response) => response.url().endsWith("/api/v1/ai/agent") && response.request().method() === "PATCH",
      );
    const name = page.getByRole("textbox", { name: "Agent name" });
    await name.fill("Ada");
    await Promise.all([saved(), name.press("Enter")]);
    await expect(page.getByText("Saved", { exact: true }).first()).toBeVisible();

    await Promise.all([saved(), page.getByRole("radio", { name: "Concise" }).click()]);
    await Promise.all([saved(), page.locator("label", { has: page.getByRole("checkbox", { name: /Email/ }) }).click()]);

    await page.reload();
    await expect(page.getByRole("textbox", { name: "Agent name" })).toHaveValue("Ada");
    await expect(page.getByRole("radio", { name: "Concise" })).toBeChecked();
    await expect(page.getByRole("checkbox", { name: /Email/ })).toBeChecked();
  });

  it("shows the agent's overview instead of a placeholder", async ({ page }) => {
    await page.goto("/d/agent");
    await expect(page.getByText("Setup not finished")).toBeVisible();
    await expect(page.getByRole("region", { name: "This month" })).toContainText("Answered");
    await expect(page.getByText("Nothing yet. Conversations show here once the agent answers.")).toBeVisible();
    await page.getByRole("button", { name: "Finish setup" }).click();
    await expect(page.getByRole("heading", { level: 1, name: "dimmi agent" })).toBeVisible();
  });
});
