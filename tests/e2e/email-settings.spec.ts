import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { deliverEmail, rawEmail } from "./support/inbound";

/*
 * suffixOf is the unique part of the person's address, so workspaces
 * made at the same moment never race for one name.
 */
function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("email settings", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("shows the forwarding address and turns to receiving once mail arrives", async ({ page, person, request }) => {
    await createAccount(request, person, `Mail ${suffixOf(person)}`);
    await signIn(page, person);
    await page.goto("/d/settings/email");

    await expect(page.getByRole("heading", { level: 1, name: "Email" })).toBeVisible();
    const forwarding = page.getByTestId("forwarding-address").first();
    await expect(forwarding).toHaveText(/@inbound\.trydimmi\.com$/);
    await expect(page.getByText("Waiting for first email")).toBeVisible();
    await expect(page.getByRole("button", { name: /^Copy / })).toBeVisible();
    await expect(page.getByText(/Choose Add a forwarding address and enter/)).toBeVisible();

    const address = (await forwarding.textContent()) ?? "";
    await deliverEmail(
      request,
      rawEmail(`bob.${suffixOf(person)}@customer.test`, address, "Refund for order 1042", "Charged twice."),
      address,
    );

    await expect(async () => {
      await page.reload();
      await expect(page.getByText("Receiving", { exact: true })).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 15_000 });
    await expect(page.getByText("Refund for order 1042")).toBeVisible();

    const list = await page.request.get("/api/v1/email-addresses");
    const id = ((await list.json()) as { data: { id: string }[] }).data[0]?.id ?? "";
    const activity = await page.request.get(`/api/v1/email-addresses/${id}/inbound-emails`);
    const emails = (await activity.json()) as { data: { status: string; conversation: { object: string } }[] };
    expect(emails.data[0]?.status).toBe("processed");
    expect(emails.data[0]?.conversation.object).toBe("conversation");
  });

  it("sends a forwarding test and connects another address", async ({ page, person, request }) => {
    await createAccount(request, person, `Test ${suffixOf(person)}`);
    await signIn(page, person);
    await page.goto("/d/settings/email");

    await page.getByRole("button", { name: "Send test email" }).click();
    await expect(page.getByText(/^Sent to .*It shows as receiving once it comes back\.$/)).toBeVisible();

    const added = `sales.${suffixOf(person)}@acme.test`;
    await page.getByRole("textbox", { name: "New email address" }).fill(added);
    await page.getByRole("button", { name: "Add address" }).click();
    await expect(page.getByRole("region", { name: added })).toBeVisible();
    await expect(page.getByRole("region", { name: added }).getByTestId("forwarding-address")).toHaveText(
      /\.sales@inbound\.trydimmi\.com$/,
    );
  });
});
