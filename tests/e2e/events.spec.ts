import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("events", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Events ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("shows what people did and finds them with an event rule", async ({ page }) => {
    await page.goto("/d/customers");
    const ana = await api<{ id: string }>(page, "POST", "/contacts", { name: "Ana Lima", email: "ana@lima.example" });
    await api(page, "POST", "/contacts", { name: "Bo Chen", email: "bo@chen.example" });
    await api(page, "POST", "/contact-events", {
      events: [
        { name: "report.exported", contact_id: ana.id },
        { name: "report.exported", contact_id: ana.id },
      ],
    });

    await page.goto(`/d/customers/${ana.id}`);
    await expect(page.getByRole("region", { name: "Activity" })).toContainText("2 times, 2 in the last 7 days");

    await page.goto("/d/settings/attributes");
    await expect(page.getByRole("region", { name: "Events" })).toContainText("report.exported · 2 events");

    await page.goto("/d/customers/audiences");
    await page.getByRole("button", { name: "New audience" }).click();
    const matches = page.getByRole("region", { name: "Right now" });
    await expect(matches).toContainText("2 people");
    await page.getByRole("button", { name: "Add condition" }).click();
    await page.getByRole("button", { name: "Field" }).click();
    await page.getByRole("option", { name: "Report exported: times" }).click();
    await page.getByRole("button", { name: "Comparison" }).click();
    await page.getByRole("option", { name: "is at least" }).click();
    await page.getByRole("textbox", { name: "Value" }).fill("2");
    await page.getByRole("textbox", { name: "Days" }).fill("7");
    await expect(matches).toContainText("1 person");
    await expect(matches.getByRole("link", { name: "Ana Lima" })).toBeVisible();
  });
});
