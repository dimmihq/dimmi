import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("help center messenger", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await createAccount(request, person, `Messenger ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("shows the messenger on the help center, opens it from Contact us, and goes when turned off", async ({
    page,
    guest,
  }) => {
    const centers = await api<{ subdomain: string }[]>(page, "GET", "/help-centers");
    const home = `/hc/${centers[0]?.subdomain}`;

    await guest.setViewportSize({ width: 1440, height: 900 });
    await guest.goto(`${home}/search?q=nothing-matches-this`);
    const launcher = guest.locator(".launcher");
    await expect(launcher).toBeVisible();
    await guest.getByRole("button", { name: "Contact us" }).last().click();
    await expect(launcher).toHaveAttribute("aria-expanded", "true");
    await guest.getByRole("navigation", { name: "Sections" }).getByRole("link", { name: "Help center" }).click();
    await expect(guest).toHaveURL(new RegExp(`${home}/?$`));
    await expect(launcher).toHaveAttribute("aria-expanded", "true");
    expect((await launcher.boundingBox())?.width ?? 0).toBeLessThan(80);

    await page.goto("/d/settings/help-centers");
    const show = page.getByRole("switch", { name: "Show the messenger" });
    await expect(show).toBeChecked();
    const saved = page.waitForResponse((r) => r.request().method() === "PATCH" && r.url().includes("/help-centers/"));
    await show.focus();
    await page.keyboard.press("Space");
    await expect(show).not.toBeChecked();
    expect((await saved).ok()).toBe(true);

    await guest.goto(`${home}/search?q=nothing-matches-this`);
    await expect(guest.getByText("Try fewer or different words.")).toBeVisible();
    await expect(guest.getByRole("button", { name: "Contact us" })).toHaveCount(0);
    await expect(guest.locator(".launcher")).toHaveCount(0);
  });
});
