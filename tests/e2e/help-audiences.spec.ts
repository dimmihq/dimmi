import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("article audiences", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Docs ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("keeps an article to an audience and hides it from anonymous readers", async ({ page }) => {
    await page.goto("/d/help-center");
    await api(page, "POST", "/audiences", { name: "Signed-in users", rules: { visitors: "identified" } });
    const centers = await api<{ id: string; subdomain: string }[]>(page, "GET", "/help-centers");
    const center = centers[0] ?? { id: "", subdomain: "" };
    const article = await api<{ id: string }>(page, "POST", `/help-centers/${center.id}/articles`, {
      title: "Admin keys",
      body_markdown: "Rotate them often.",
    });
    await api(page, "POST", `/articles/${article.id}/publications`, {});
    const reader = `/hc/${center.subdomain}/articles/admin-keys`;
    expect((await page.request.get(reader)).status()).toBe(200);

    await page.goto(`/d/help-center/${article.id}`);
    await page.getByRole("button", { name: "Settings" }).click();
    const settings = page.getByRole("dialog", { name: "Article settings" });
    await expect(async () => {
      await settings.getByRole("button", { name: /Audience/ }).click();
      await page.getByRole("option", { name: "Signed-in users" }).click({ timeout: 2000 });
    }).toPass();
    await expect(settings.getByRole("button", { name: /Audience/ })).toContainText("Signed-in users");
    await expect.poll(async () => (await page.request.get(reader)).status()).toBe(404);
  });
});
