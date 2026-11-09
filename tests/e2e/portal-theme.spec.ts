import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("portal theme", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await createAccount(request, person, `Themes ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("lets readers pick light, dark or their system's theme, unless the team fixed one", async ({ page, guest }) => {
    const centers = await api<{ id: string; subdomain: string; settings: Record<string, unknown> }[]>(
      page,
      "GET",
      "/help-centers",
    );
    const center = centers[0];
    const home = `/hc/${center?.subdomain}`;
    const root = guest.locator("html");

    await guest.emulateMedia({ colorScheme: "light" });
    await guest.goto(home);
    const themes = guest.getByRole("group", { name: "Theme" });
    await expect(themes.getByRole("button", { name: "Match my system" })).toHaveAttribute("aria-pressed", "true");
    await expect(root).not.toHaveClass(/dark/);

    await themes.getByRole("button", { name: "Dark theme" }).click();
    await expect(root).toHaveClass(/dark/);
    await guest.reload();
    await expect(root).toHaveClass(/dark/);
    await expect(themes.getByRole("button", { name: "Dark theme" })).toHaveAttribute("aria-pressed", "true");

    await themes.getByRole("button", { name: "Match my system" }).click();
    await guest.emulateMedia({ colorScheme: "dark" });
    await expect(root).toHaveClass(/dark/);

    await api(page, "PATCH", `/help-centers/${center?.id}`, { settings: { ...center?.settings, appearance: "light" } });
    await guest.reload();
    await expect(root).not.toHaveClass(/dark/);
    await expect(guest.getByRole("group", { name: "Theme" })).toHaveCount(0);
  });
});
