import { randomUUID } from "node:crypto";

import { createAccount, expect, test as it, signIn } from "./support/fixtures";

it.describe("own password", () => {
  it("changing the password signs out the other browser", async ({ page, guest, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Password ${randomUUID().slice(0, 8)}`);
    await signIn(page, person);
    await signIn(guest, person);

    await page.goto("/d/settings/security");
    await page.getByRole("button", { name: "Change password" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Current password").fill(person.password);
    await dialog.getByLabel("New password", { exact: true }).fill("a-fresh-long-passphrase-1843");
    await dialog.getByLabel("Repeat new password").fill("a-fresh-long-passphrase-1843");
    await dialog.getByRole("button", { name: "Save password" }).click();

    await expect(dialog.getByRole("heading", { name: "Password saved" })).toBeVisible();
    await dialog.getByRole("button", { name: "Done" }).click();

    await page.reload();
    await expect(page.getByRole("heading", { level: 1, name: "Security & access" })).toBeVisible();

    await guest.goto("/d/settings/security");
    await expect(guest).toHaveURL(/\/login/);
  });

  it("a wrong current password is said in the dialog", async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Password ${randomUUID().slice(0, 8)}`);
    await signIn(page, person);

    await page.goto("/d/settings/security");
    await page.getByRole("button", { name: "Change password" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Current password").fill("not-the-password");
    await dialog.getByLabel("New password", { exact: true }).fill("a-fresh-long-passphrase-1843");
    await dialog.getByLabel("Repeat new password").fill("a-fresh-long-passphrase-1843");
    await dialog.getByRole("button", { name: "Save password" }).click();

    await expect(dialog.getByText("The password is incorrect.")).toBeVisible();
  });
});
