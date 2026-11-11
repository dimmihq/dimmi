import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

it.describe("series", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Series ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("builds a series, sets it live, runs a test journey through it, and pauses it", async ({ page, person }) => {
    await api(page, "POST", "/tags", { name: "Welcomed" });
    await api(page, "POST", "/contacts", { name: "Me", email: person.email });

    await page.goto("/d/series");
    await expect(page.getByText("No series yet")).toBeVisible();
    await page.getByRole("button", { name: "New series" }).click();
    const made = page.getByRole("dialog", { name: "New series" });
    await made.getByRole("textbox", { name: "Name" }).fill("Welcome");
    await made.getByRole("radio", { name: /People/ }).check();
    await made.getByRole("radio", { name: /Start from scratch/ }).check();
    await made.getByRole("button", { name: "Create series" }).click();

    await expect(page.getByRole("heading", { name: "Welcome" })).toBeVisible();
    await page.getByRole("button", { name: "Add a step" }).click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Tag", exact: true }).click();
    await expect(page.getByText("Choose the tags.")).toBeVisible();
    await page.getByRole("button", { name: "Tag" }).last().click();
    await page.getByRole("option", { name: "Welcomed" }).click();
    await expect(page.getByText("Saved")).toBeVisible();

    await page.getByRole("button", { name: "Set live" }).click();
    const live = page.getByRole("dialog", { name: "Set this series live" });
    await live.getByRole("button", { name: "Set live" }).click();
    await expect(live).toBeHidden();
    await expect(page.getByText("Live", { exact: true })).toBeVisible();

    await page.getByRole("button", { name: "Try it on me" }).click();
    const trial = page.getByRole("dialog", { name: "Your test journey" });
    await expect(trial.getByText("Ended: Finished.")).toBeVisible({ timeout: 15_000 });
    await expect(trial.getByText("Add 1 tag")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(trial).toBeHidden();

    await page.getByRole("button", { name: "Pause" }).click();
    await expect(page.getByText("Paused", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Resume" }).click();
    await expect(page.getByText("Live", { exact: true })).toBeVisible();

    await page.getByRole("link", { name: "Back to series" }).click();
    await expect(page.getByRole("row", { name: /Welcome/ })).toContainText("Live");
  });

  it("writes a message on its own page and comes back to the same step", async ({ page }) => {
    await page.goto("/d/series");
    await page.getByRole("button", { name: "New series" }).click();
    const made = page.getByRole("dialog", { name: "New series" });
    await made.getByRole("textbox", { name: "Name" }).fill("Check in");
    await made.getByRole("button", { name: "Create series" }).click();

    await page.getByRole("button", { name: "Add a step" }).click();
    await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Chat", exact: true }).click();
    await expect(page.getByText("Nothing written yet")).toBeVisible();
    await page.getByRole("button", { name: "Compose" }).click();

    await expect(page).toHaveURL(/\/compose\//);
    await page.getByRole("textbox", { name: "Message" }).click();
    await page.keyboard.type("How is your first week going?");
    await expect(page.getByText("Saved. Publish the series to send it.")).toBeVisible();
    await page.getByRole("link", { name: "Back to Check in" }).click();

    await expect(page).toHaveURL(/\?step=/);
    const settings = page.getByRole("region", { name: "Step settings" });
    await expect(settings.getByRole("heading", { name: "Chat" })).toBeVisible();
    await expect(settings.getByText("How is your first week going?")).toBeVisible();
  });
});
