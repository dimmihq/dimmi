import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { mailedText } from "./support/mail";

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
    await page.getByRole("button", { name: "Save", exact: true }).click();
    expect((await saved).ok()).toBe(true);

    await guest.goto(`${home}/search?q=nothing-matches-this`);
    await expect(guest.getByText("Try fewer or different words.")).toBeVisible();
    await expect(guest.getByRole("button", { name: "Contact us" })).toHaveCount(0);
    await expect(guest.locator(".launcher")).toHaveCount(0);
  });

  it("signs the messenger in as the reader who signs in to the help center, and out with them", async ({
    page,
    person,
    guest,
  }) => {
    const centers = await api<{ subdomain: string }[]>(page, "GET", "/help-centers");
    const apps = await api<{ id: string }[]>(page, "GET", "/widget-settings");
    const home = `/hc/${centers[0]?.subdomain}`;
    const signedIn = () => guest.evaluate((app) => localStorage.getItem(`dimmi:${app}:sub`), apps[0]?.id ?? "");
    const reader = `reader.${person.email}`;

    await guest.goto(home);
    await expect(guest.locator(".launcher")).toBeVisible();
    expect(await signedIn()).toBeNull();

    await guest.goto(`${home}/sso/sign-in?next=${encodeURIComponent(home)}`);
    await guest.getByLabel("Email").fill(reader);
    await guest.getByRole("button", { name: "Email me a code" }).click();
    const mailed = await mailedText(reader, "your code for");
    await guest.getByLabel("Code").fill(mailed.match(/\b\d{6}\b/)?.[0] ?? "");
    await guest.getByRole("button", { name: "Sign in" }).click();
    await expect(guest.getByRole("button", { name: "Sign out" })).toBeVisible();
    await expect.poll(signedIn).not.toBeNull();

    await guest.getByRole("button", { name: "Sign out" }).click();
    await expect(guest.getByRole("button", { name: "Sign out" })).toHaveCount(0);
    await guest.reload();
    await expect.poll(signedIn).toBeNull();
  });
});
