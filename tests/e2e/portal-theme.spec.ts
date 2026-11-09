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
    const menu = guest.getByLabel("Theme", { exact: true });
    const choice = (name: string) => guest.getByRole("button", { name, exact: true });
    await menu.click();
    await expect(choice("System")).toHaveAttribute("aria-pressed", "true");
    await expect(root).not.toHaveClass(/dark/);

    await choice("Dark").click();
    await expect(root).toHaveClass(/dark/);
    await expect(guest.locator("#dimmi-messenger")).toHaveAttribute("data-theme", "dark");
    await expect(choice("Dark")).toBeHidden();
    await guest.reload();
    await expect(root).toHaveClass(/dark/);
    await menu.click();
    await expect(choice("Dark")).toHaveAttribute("aria-pressed", "true");

    await choice("System").click();
    await guest.emulateMedia({ colorScheme: "dark" });
    await expect(root).toHaveClass(/dark/);

    await api(page, "PATCH", `/help-centers/${center?.id}`, { settings: { ...center?.settings, appearance: "light" } });
    await guest.reload();
    await expect(root).not.toHaveClass(/dark/);
    await expect(guest.getByLabel("Theme", { exact: true })).toHaveCount(0);
  });
});
