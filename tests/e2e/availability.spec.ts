/*
 * Availability end to end: a person goes away from the account menu,
 * the dot on their avatar follows, the choice survives a reload, and
 * coming back clears handing conversations back.
 */
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("availability", () => {
  it("goes away and comes back from the account menu", async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Away ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);

    const card = page.getByRole("button", { name: person.name, exact: true });
    const dot = card.locator("[data-status]");
    await expect(dot).toHaveAttribute("data-status", "online");

    await card.click();
    await page.getByRole("button", { name: "Away and reassigning" }).click();
    await expect(dot).toHaveAttribute("data-status", "away");

    await page.reload();
    await expect(dot).toHaveAttribute("data-status", "away");
    await card.click();
    await expect(page.getByRole("button", { name: "Away and reassigning" })).toHaveAttribute("aria-current", "page");

    await page.getByRole("button", { name: "Active" }).click();
    await expect(dot).toHaveAttribute("data-status", "online");

    const me = await page.request.get("/api/v1/me", {
      headers: { "Dimmi-Workspace": (await (await page.request.get("/api/v1/me")).json()).data.workspaces[0].id },
    });
    const member = (await me.json()).data.member;
    expect(member.availability).toBe("online");
    expect(member.away_reassign).toBe(false);
  });
});
