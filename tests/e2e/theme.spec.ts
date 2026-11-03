import { expect, test as it } from "./support/fixtures";

it.describe("light and dark", () => {
  it("follows the system theme before anything was chosen", async ({ page, colorScheme }) => {
    await page.goto("/login");

    await expect(page.locator("html")).toHaveClass(colorScheme === "dark" ? "dark" : "");
  });

  it("keeps the chosen theme, rendered by the server from the first byte", async ({ page, context }) => {
    const chosen = async () => (await context.cookies()).find((cookie) => cookie.name === "dimmi_theme")?.value;

    await page.goto("/login");
    await page.getByRole("button", { name: "Use dark mode" }).click();
    await expect(page.locator("html")).toHaveClass(/dark/);
    await expect.poll(chosen).toBe("dark");

    const response = await page.reload();
    expect(await response?.text()).toContain('class="dark"');
    await expect(page.locator("html")).toHaveClass(/dark/);

    await page.getByRole("button", { name: "Use light mode" }).click();
    await expect.poll(chosen).toBe("light");
    const light = await page.reload();
    expect(await light?.text()).toContain('class=""');
    await expect(page.locator("html")).not.toHaveClass(/dark/);
  });
});
