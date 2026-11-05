import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { deliverEmail, rawEmail } from "./support/inbound";
import { mailedText } from "./support/mail";

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
    await expect(page.getByText(/^Forward every message sent to .* to .*@inbound\.trydimmi\.com/)).toBeVisible();
    await page.getByRole("button", { name: /Mail service/ }).click();
    await page.getByRole("option", { name: "Gmail" }).click();
    await expect(page.getByText(/choose Add a forwarding address\.$/)).toBeVisible();
    await expect(page.getByText(/Sent folder unless you connect the mailbox/)).toBeVisible();

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

  it("runs a forwarding test and connects another address", async ({ page, person, request }) => {
    await createAccount(request, person, `Test ${suffixOf(person)}`);
    await signIn(page, person);
    await page.goto("/d/settings/email");

    await page.getByRole("button", { name: "Test forwarding" }).click();
    const status = page.getByTestId("forwarding-test-status").first();
    await expect(status).toHaveText(/^Waiting for it to come back/);

    const list = await page.request.get("/api/v1/email-addresses");
    const [address] = ((await list.json()) as { data: { address: string; forwarding_address: string }[] }).data;
    const sent = await mailedText(address?.address ?? "", "dimmi forwarding test");
    const line = /dimmi test t1\.\S+/.exec(sent)?.[0] ?? "";
    expect(line).not.toBe("");
    await deliverEmail(
      request,
      rawEmail(
        `it.${suffixOf(person)}@customer.test`,
        address?.address ?? "",
        "Fwd: forwarding test",
        `Forwarded.\n\n${line}`,
      ),
      address?.forwarding_address ?? "",
    );
    await expect(status).toHaveText(/^Forwarding works, took \d+ s$/, { timeout: 15_000 });

    const added = `sales@acme-${suffixOf(person)}.test`;
    await page.getByRole("textbox", { name: "New email address" }).fill(added);
    await page.getByRole("button", { name: "Add address" }).click();
    await expect(page.getByRole("region", { name: added })).toBeVisible();
    await expect(page.getByRole("region", { name: added }).getByTestId("forwarding-address")).toHaveText(
      /\.sales@inbound\.trydimmi\.com$/,
    );
  });
});
