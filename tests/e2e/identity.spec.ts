import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("identity verification", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Identity ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("creates a secret shown once, rotates it and revokes the old one", async ({ page }) => {
    await page.goto("/d/settings/identity");
    await expect(page.getByRole("heading", { level: 1, name: "Identity verification" })).toBeVisible();
    await expect(page.getByText("No secrets yet. Create one to sign in your customers.")).toBeVisible();

    await page.getByRole("button", { name: "New secret" }).click();
    const create = page.getByRole("dialog", { name: "New identity secret" });
    await create.getByRole("button", { name: "Create secret" }).click();

    const created = page.getByRole("dialog", { name: "Copy your new secret" });
    const secret = await created.getByRole("textbox", { name: "Secret" }).inputValue();
    const kid = await created.getByRole("textbox", { name: "Key id (kid header)" }).inputValue();
    expect(secret).toMatch(/^dmi_idsec_[A-Za-z0-9]{43}$/);
    expect(kid).toMatch(/^isk_/);
    await created.getByRole("button", { name: "Done" }).click();

    await expect(page.getByText(`HS256 · …${secret.slice(-4)} · Never used`)).toBeVisible();
    await expect(page.getByText(`kid: "${kid}"`)).toBeVisible();
    await expect(page.getByText(secret)).toHaveCount(0);

    await page.getByRole("button", { name: "Rotate" }).click();
    const rotated = page.getByRole("dialog", { name: "Copy your new secret" });
    await expect(rotated.getByRole("textbox", { name: "Secret" })).not.toHaveValue(secret);
    await rotated.getByRole("button", { name: "Done" }).click();
    await expect(page.getByText(/^Works until /)).toBeVisible();

    await page.getByRole("button", { name: "Revoke" }).last().click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Revoke" }).click();
    await expect(page.getByText("Revoked", { exact: true })).toBeVisible();
  });
});
