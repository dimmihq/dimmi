import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("article feedback", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await createAccount(request, person, `Notes ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("asks a reader who says no what was missing, and shows the note beside the article", async ({ page, guest }) => {
    const centers = await api<{ id: string; subdomain: string }[]>(page, "GET", "/help-centers");
    const center = centers[0] ?? { id: "", subdomain: "" };
    const article = await api<{ id: string }>(page, "POST", `/help-centers/${center.id}/articles`, {
      title: "Reset your password",
      body_markdown: "Open **Settings** and pick Password.",
    });
    await api(page, "POST", `/articles/${article.id}/publications`, {});

    const articleURL = `/hc/${center.subdomain}/articles/reset-your-password`;
    await guest.goto(articleURL);
    await guest.evaluate(() => {
      (window as unknown as { stayed: boolean }).stayed = true;
    });
    await guest.getByRole("button", { name: "No" }).click();
    await expect(guest.getByLabel("Sorry about that. What were you looking for?")).toBeFocused();
    await guest.getByLabel("Sorry about that. What were you looking for?").fill("How do I reset it with SSO?");
    await guest.getByRole("button", { name: "Send feedback" }).click();
    await expect(guest.getByText("The team reads every note")).toBeVisible();
    await expect(guest).toHaveURL(new RegExp(`${articleURL}$`));
    expect(await guest.evaluate(() => (window as unknown as { stayed?: boolean }).stayed)).toBe(true);

    await page.goto(`/d/help-center/${article.id}`);
    await page.getByRole("button", { name: "Reader feedback" }).click();
    const pane = page.getByRole("region", { name: "Reader feedback" });
    await expect(pane.getByText("How do I reset it with SSO?")).toBeVisible();
    await expect(pane.getByText("Anonymous reader")).toBeVisible();
    await expect(pane.getByText("0 yes · 1 no")).toBeVisible();

    await page.getByRole("button", { name: "Version history" }).click();
    await expect(page.getByRole("region", { name: "Version history" })).toBeVisible();
  });
});
