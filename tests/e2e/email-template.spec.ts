import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { openAddress } from "./support/inbox";

/*
 * suffixOf is the unique part of the person's address, so workspaces
 * made at the same moment never race for one name.
 */
function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("reply style and default address", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("switches an address to the branded style and previews its footer", async ({ page, person, request }) => {
    await createAccount(request, person, `Style ${suffixOf(person)}`);
    await signIn(page, person);
    await openAddress(page);

    const card = page
      .getByRole("region")
      .filter({ has: page.getByTestId("forwarding-address") })
      .first();
    const preview = card.getByTitle(/^Reply preview for /);
    await expect(preview).toHaveAttribute("srcdoc", /Thanks for getting in touch/);
    await expect(preview).not.toHaveAttribute("srcdoc", /<table/);

    await card.getByRole("radio", { name: "Branded" }).click();
    await expect(preview).toHaveAttribute("srcdoc", /border-top:2px solid #171717/);

    await card.getByRole("textbox", { name: "Footer" }).fill("**Acme** support, every weekday");
    await expect(preview).toHaveAttribute("srcdoc", /<strong>Acme<\/strong> support, every weekday/);
    await page.getByRole("button", { name: "Save", exact: true }).click();

    await expect(async () => {
      const list = await page.request.get("/api/v1/email-addresses");
      const settings = ((await list.json()) as { data: { settings: Record<string, string> }[] }).data[0]?.settings;
      expect(settings?.template).toBe("branded");
      expect(settings?.footer_markdown).toBe("**Acme** support, every weekday");
    }).toPass({ timeout: 10_000 });

    await page.reload();
    await expect(card.getByRole("radio", { name: "Branded" })).toBeChecked();
    await expect(card.getByRole("textbox", { name: "Footer" })).toHaveValue("**Acme** support, every weekday");
  });

  it("makes another address the default", async ({ page, person, request }) => {
    await createAccount(request, person, `Default ${suffixOf(person)}`);
    await signIn(page, person);
    await page.goto("/d/settings/email");

    const added = `sales@acme-${suffixOf(person)}.test`;
    await page.getByRole("textbox", { name: "New email address" }).fill(added);
    await page.getByRole("button", { name: "Add address" }).click();
    await expect(page.getByRole("region", { name: "Addresses" }).getByRole("link", { name: added })).toBeVisible();
    await openAddress(page, added);
    const card = page.getByRole("region", { name: "Setup" });
    await expect(card.getByText("Default", { exact: true })).toHaveCount(0);

    await card.getByRole("button", { name: "Make default" }).click();
    await expect(card.getByText("Default", { exact: true })).toBeVisible();
    await expect(card.getByRole("button", { name: "Make default" })).toHaveCount(0);
    await page.goto("/d/settings/email");
    await expect(page.getByRole("region", { name: "Addresses" }).getByText("Default", { exact: true })).toHaveCount(1);
  });
});
