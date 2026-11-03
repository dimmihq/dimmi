import { createAccount, expect, test as it, signIn } from "./support/fixtures";
import { mailedLink } from "./support/mail";

it.describe("API tokens", () => {
  it.use({ permissions: ["clipboard-read", "clipboard-write"] });

  it("mints a token, copies its secret once, calls the API with it and revokes it", async ({
    page,
    person,
    request,
  }) => {
    await createAccount(request, person, `Tokens ${person.email.split("@")[0]?.split(".").at(-1)}`);
    await signIn(page, person);
    await page.goto(await mailedLink(person.email, "Verify"));
    await expect(page.getByRole("heading", { name: "Email verified" })).toBeVisible();

    await page.goto("/d/settings/tokens");
    const list = page.getByRole("region", { name: "Your tokens" });
    await expect(list.getByText("No tokens yet")).toBeVisible();

    await list.getByRole("button", { name: "New token" }).click();
    const create = page.getByRole("dialog", { name: "New API token" });
    await create.getByRole("textbox", { name: "Name" }).fill("CI deploy");
    await create.getByText("Members: read", { exact: true }).click();
    await expect(create.getByRole("checkbox", { name: "Members: read" })).toBeChecked();
    await expect(create.getByRole("checkbox", { name: "Billing: read" })).toBeEnabled();
    await create.getByRole("button", { name: "Create token" }).click();

    const secret = page.getByRole("dialog", { name: "Copy your new token" });
    const field = secret.getByRole("textbox", { name: "CI deploy" });
    const token = await field.inputValue();
    expect(token).toMatch(/^dmi_live_[0-9A-Za-z]{38}$/);
    await expect(field).toHaveAttribute("readonly", "");

    await secret.getByRole("button", { name: "Copy token" }).click();
    await expect(secret.getByText("Copied to the clipboard.")).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(token);
    await secret.getByRole("button", { name: "Done" }).click();
    await expect(secret).toBeHidden();

    const row = list.getByText("CI deploy", { exact: true });
    await expect(row).toBeVisible();
    await expect(list.getByText(/Members: read · Never used · Expires/)).toBeVisible();

    const bearer = { Authorization: `Bearer ${token}` };
    const members = await request.get("/api/v1/members", { headers: bearer });
    expect(members.status()).toBe(200);
    const me = await request.get("/api/v1/me", { headers: bearer });
    expect((await me.json()).data.auth.token.name).toBe("CI deploy");
    expect((await request.get("/api/v1/tokens", { headers: bearer })).status()).toBe(403);

    await page.reload();
    await expect(list.getByText(/Last used/)).toBeVisible();

    await list.getByRole("button", { name: "Revoke" }).click();
    const confirm = page.getByRole("alertdialog", { name: "Revoke CI deploy?" });
    await confirm.getByRole("button", { name: "Revoke" }).click();
    await expect(list.getByText("Revoked", { exact: true })).toBeVisible();
    await expect(list.getByRole("button", { name: "Revoke" })).toBeHidden();

    const refused = await request.get("/api/v1/members", { headers: bearer });
    expect(refused.status()).toBe(401);
    expect((await refused.json()).code).toBe("token_revoked");
  });
});
