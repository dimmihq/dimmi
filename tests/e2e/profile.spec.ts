import { randomUUID } from "node:crypto";

import { createAccount, expect, test as it, signIn } from "./support/fixtures";
import { mailedLink } from "./support/mail";

const onePixel = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M/wHwAEBgIApD5fRAAAAABJRU5ErkJggg==",
  "base64",
);

it.describe("own profile", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Profile ${randomUUID().slice(0, 8)}`);
    await signIn(page, person);
    await page.goto("/d/settings/profile");
  });

  it("a person changes their name and picture", async ({ page }) => {
    const you = page.getByRole("region", { name: "You" });

    await you.getByRole("textbox", { name: "Full name" }).fill("Ada Lovelace");
    await you.getByRole("button", { name: "Save" }).click();
    await expect(you.getByText("Saved.")).toBeVisible();
    await page.reload();
    await expect(page.getByRole("textbox", { name: "Full name" })).toHaveValue("Ada Lovelace");

    const chooser = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "Upload" }).click();
    await (await chooser).setFiles({
      name: "me.png",
      mimeType: "image/png",
      buffer: onePixel,
    });

    const picture = page.getByRole("img", { name: "Your profile picture" });
    await expect(picture).toBeVisible({ timeout: 20_000 });
    await expect.poll(() => picture.evaluate((image: HTMLImageElement) => image.naturalWidth)).toBe(256);

    await page.getByRole("button", { name: "Remove" }).click();
    await expect(picture).toBeHidden();
  });

  it("a person changes their email and confirms from the new mailbox", async ({ page, person }) => {
    const address = `moved.${randomUUID().slice(0, 8)}@example.com`;

    await page.getByRole("button", { name: "Change", exact: true }).click();
    await page.getByRole("dialog").getByLabel("New email").fill(address);
    await page.getByRole("dialog").getByRole("button", { name: "Send link" }).click();
    await expect(page.getByText(/Open the link we sent to m\*\*\*@example\.com/)).toBeVisible();
    await expect(page.getByRole("region", { name: "You" }).getByText(person.email)).toBeVisible();

    await page.goto(await mailedLink(address, "Confirm your new"));
    await expect(page.getByRole("heading", { name: "Email changed" })).toBeVisible();

    await page.goto("/d/settings/profile");
    await expect(page.getByRole("region", { name: "You" }).getByText(address)).toBeVisible();
  });

  it("a person deletes their account after typing their address", async ({ page, person }) => {
    await page.getByRole("button", { name: "Delete account" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("they are deleted too", { exact: false })).toBeVisible();
    const confirm = dialog.getByRole("button", { name: "Delete account" });
    await expect(confirm).toBeDisabled();

    await dialog.getByLabel("Your email to confirm").fill(person.email);
    await confirm.click();

    await expect(page).toHaveURL(/\/login$/);
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page.getByText("These credentials do not match our records.")).toBeVisible();
  });
});
