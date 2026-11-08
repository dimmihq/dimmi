/*
 * A reader signs in to a help center through the team's product: the
 * product sends them back to /sso with an identity token in the fragment,
 * the page sends it on, and the article kept to their audience opens.
 * Signed out, the same customer signs in again with only their email and
 * the code mailed to it.
 */
import { createHmac } from "node:crypto";

import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { mailedText } from "./support/mail";

/*
 * hs256 signs claims the way the team's product would.
 */
function hs256(kid: string, secret: string, claims: Record<string, unknown>): string {
  const part = (value: object) => Buffer.from(JSON.stringify(value)).toString("base64url");
  const body = `${part({ alg: "HS256", typ: "JWT", kid })}.${part(claims)}`;

  return `${body}.${createHmac("sha256", secret).update(body).digest("base64url")}`;
}

it.describe("help center sign-in", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await createAccount(request, person, `Readers ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("opens an article kept to the reader's audience after signing in", async ({ page, browser, person }) => {
    await page.goto("/d/docs");
    const secret = await api<{ id: string; secret: string }>(page, "POST", "/identity-secrets", {
      name: "Product",
      algorithm: "HS256",
    });
    const pro = await api<{ id: string }>(page, "POST", "/audiences", {
      name: "Pro",
      rules: { rules: { field: "contact.custom.plan", op: "eq", value: "pro" } },
    });
    const centers = await api<{ id: string; subdomain: string }[]>(page, "GET", "/help-centers");
    const center = centers[0] ?? { id: "", subdomain: "" };
    await api(page, "PATCH", `/help-centers/${center.id}`, { settings: { login_url: "https://login.example/help" } });
    const article = await api<{ id: string }>(page, "POST", `/help-centers/${center.id}/articles`, {
      title: "Priority support",
      body_markdown: "Call us any time.",
    });
    await api(page, "POST", `/articles/${article.id}/publications`, {});
    await api(page, "PATCH", `/articles/${article.id}/audience`, { audience_id: pro.id });

    const reader = await (await browser.newContext({ baseURL: page.url() })).newPage();
    const path = `/hc/${center.subdomain}/articles/priority-support`;
    await reader.goto(path);
    await expect(reader.getByRole("heading", { name: "We couldn't find that page" })).toBeVisible();
    await reader.getByRole("link", { name: "Sign in" }).click();
    await expect(reader.getByRole("heading", { name: "Sign in to see articles for customers" })).toBeVisible();
    await expect(reader.getByRole("link", { name: "Continue with your account" })).toHaveAttribute(
      "href",
      /^https:\/\/login\.example\/help\?return_to=/,
    );

    const now = Math.floor(Date.now() / 1000);
    const token = hs256(secret.id, secret.secret, {
      sub: "user_e2e",
      aud: "dimmi-help",
      iat: now,
      exp: now + 300,
      nonce: `n-${now}`,
      email: `pro.${person.email}`,
      custom_attributes: { plan: "pro" },
    });
    await reader.goto(`/hc/${center.subdomain}/sso?next=${encodeURIComponent(path)}#token=${token}`);
    await expect(reader).toHaveURL(new RegExp(`${path}$`));
    await expect(reader.getByText("Call us any time.")).toBeVisible();
    await reader.getByRole("button", { name: "Sign out" }).click();
    await reader.goto(path);
    await expect(reader.getByRole("heading", { name: "We couldn't find that page" })).toBeVisible();

    await reader.getByRole("link", { name: "Sign in" }).click();
    await reader.getByLabel("Email").fill(`pro.${person.email}`);
    await reader.getByRole("button", { name: "Email me a code" }).click();
    await expect(reader.getByRole("heading", { name: "Check your email" })).toBeVisible();
    const mailed = await mailedText(`pro.${person.email}`, "your code for");
    await reader.getByLabel("Code").fill(mailed.match(/\b\d{6}\b/)?.[0] ?? "");
    await reader.getByRole("button", { name: "Sign in" }).click();
    await expect(reader).toHaveURL(new RegExp(`${path}$`));
    await expect(reader.getByText("Call us any time.")).toBeVisible();
  });
});
