import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

it.describe("customer privacy", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Privacy ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("erases a customer, cancels it, then wipes them at once", async ({ page }) => {
    const id = await startConversation(page, {
      name: "Ana Lima",
      email: "ana@lima.example",
      subject: "Delete my data",
      body: "Please remove everything about me.",
    });
    const conversation = await api<{ contact: { id: string } }>(page, "GET", `/conversations/${id}`);
    const contact = conversation.contact.id;

    await page.goto(`/d/customers/${contact}`);
    await page.getByRole("button", { name: "Erase" }).click();
    await page.getByRole("button", { name: "Erase customer" }).click();
    await expect(page).toHaveURL(/\/d\/customers\/erasures$/);
    const erasures = page.getByRole("grid", { name: "Erasures" });
    await expect(erasures.getByText("Waiting")).toBeVisible();
    await expect.poll(async () => (await api<unknown[]>(page, "GET", "/contacts")).length).toBe(0);

    await erasures.getByRole("button", { name: "Cancel erasure" }).click();
    await expect(erasures.getByText("Canceled")).toBeVisible();
    await page.goto(`/d/customers/${contact}`);
    await expect(page.getByRole("heading", { name: "Ana Lima" })).toBeVisible();

    await page.getByRole("button", { name: "Erase" }).click();
    await page.getByText("Wipe now instead of in 7 days").click();
    await page.getByRole("button", { name: "Erase customer" }).click();
    await expect(page).toHaveURL(/\/d\/customers\/erasures$/);
    await expect
      .poll(async () => (await api<{ status: string }[]>(page, "GET", "/contact-erasures"))[0]?.status, {
        timeout: 15_000,
      })
      .toBe("completed");

    await page.goto(`/d/inbox/all/${id}`);
    await expect(page.getByText("Deleted contact").first()).toBeVisible();
    await expect(page.getByText("Please remove everything about me.")).toHaveCount(0);
  });

  it("exports a customer's data as a zip", async ({ page }) => {
    const id = await startConversation(page, {
      name: "Bo Chen",
      email: "bo@chen.example",
      subject: "My data",
      body: "Send me a copy of my data.",
    });
    const conversation = await api<{ contact: { id: string } }>(page, "GET", `/conversations/${id}`);

    await page.goto(`/d/customers/${conversation.contact.id}`);
    await page.getByRole("button", { name: "Export data" }).click();
    const download = page.getByRole("button", { name: "Download export" });
    await expect(download).toBeVisible({ timeout: 15_000 });
    const [file] = await Promise.all([page.waitForEvent("download"), download.click()]);
    expect(file.suggestedFilename()).toMatch(/^exp_.*\.zip$/);
  });
});
