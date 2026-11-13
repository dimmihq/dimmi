import { createHmac } from "node:crypto";

import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

/*
 * sign mints the token a host app's server would, HS256 with the secret
 * as typed.
 */
function sign(secret: string, kid: string, claims: Record<string, unknown>): string {
  const encode = (value: unknown) => Buffer.from(JSON.stringify(value)).toString("base64url");
  const now = Math.floor(Date.now() / 1000);
  const input = `${encode({ alg: "HS256", typ: "JWT", kid })}.${encode({ iat: now, exp: now + 600, ...claims })}`;
  return `${input}.${createHmac("sha256", secret).update(input).digest("base64url")}`;
}

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

  it("switches the sign-in mode and explains pasted tokens", async ({ page }) => {
    await page.goto("/d/settings/identity");
    const mode = page.getByRole("region", { name: "Sign-in mode" });
    await expect(mode.getByText("Signed-in customers are verified; everyone else chats anonymously.")).toBeVisible();
    await expect(mode.getByText("No messenger starts yet.")).toBeVisible();
    await mode.getByRole("button", { name: "Sign-in mode" }).click();
    await page.getByRole("option", { name: "Off" }).click();
    await expect(mode.getByText("Identity tokens are ignored and every visitor chats anonymously.")).toBeVisible();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("button", { name: "Save", exact: true })).toBeHidden();

    await page.getByRole("button", { name: "New secret" }).click();
    await page
      .getByRole("dialog", { name: "New identity secret" })
      .getByRole("button", { name: "Create secret" })
      .click();
    const created = page.getByRole("dialog", { name: "Copy your new secret" });
    const secret = await created.getByRole("textbox", { name: "Secret" }).inputValue();
    const kid = await created.getByRole("textbox", { name: "Key id (kid header)" }).inputValue();
    await created.getByRole("button", { name: "Done" }).click();

    const checker = page.getByRole("region", { name: "Check a token" });
    await checker.getByRole("textbox", { name: "Identity token" }).fill(sign(`${secret}\n`, kid, { sub: "user_1" }));
    await checker.getByRole("button", { name: "Check token" }).click();
    await expect(checker.getByText("Signature does not match.")).toBeVisible();
    await expect(checker.getByText(/trailing newline/)).toBeVisible();

    await checker.getByRole("textbox", { name: "Identity token" }).fill(sign(secret, kid, { sub: "user_1" }));
    await checker.getByRole("button", { name: "Check token" }).click();
    await expect(checker.getByText("Valid token")).toBeVisible();
    await expect(checker.getByText("Would create a new customer on first sign-in.")).toBeVisible();
  });
});
