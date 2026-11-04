import { createAccount, expect, test as it, signIn } from "./support/fixtures";

it.describe("deleting a workspace", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("deletes the workspace once its address is typed", async ({ page, person, request }) => {
    const suffix = person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
    await createAccount(request, person, `Doomed ${suffix}`);
    await signIn(page, person);
    await page.goto("/d/settings/workspace");

    const slug = await page.getByRole("textbox", { name: "Workspace address" }).inputValue();
    await page.getByRole("button", { name: "Delete workspace" }).click();

    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("heading", { name: `Delete Doomed ${suffix}?` })).toBeVisible();
    const confirm = dialog.getByRole("button", { name: "Delete workspace" });
    await expect(confirm).toBeDisabled();

    await dialog.getByLabel("Workspace address to confirm").fill(`${slug}-typo`);
    await expect(confirm).toBeDisabled();
    await dialog.getByLabel("Workspace address to confirm").fill(slug);
    await expect(confirm).toBeEnabled();

    const deleted = page.waitForResponse(
      (r) => r.url().endsWith("/api/v1/workspace") && r.request().method() === "DELETE",
    );
    await confirm.click();
    expect((await deleted).status()).toBe(200);
    await expect(page).not.toHaveURL(/\/settings\/workspace$/);
  });
});
