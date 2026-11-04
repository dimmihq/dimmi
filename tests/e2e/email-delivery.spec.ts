import { resumeMail, stopMail } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { deliverEmail, rawEmail } from "./support/inbound";

/*
 * suffixOf is the unique part of the person's address, so workspaces
 * made at the same moment never race for one name.
 */
function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("a reply that was not delivered", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("says why and sends it again", async ({ page, person, request }) => {
    await createAccount(request, person, `Delivery ${suffixOf(person)}`);
    await signIn(page, person);
    await page.goto("/d/settings/email");
    const address = (await page.getByTestId("forwarding-address").first().textContent()) ?? "";
    const customer = `bob.${suffixOf(person)}@customer.test`;
    await deliverEmail(request, rawEmail(customer, address, "Where is my refund", "Any news?"), address);

    await page.goto("/d/inbox/unassigned");
    const row = page.getByRole("link", { name: /Where is my refund/ });
    await expect(async () => {
      await page.reload();
      await expect(row).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 15_000 });
    await row.click();

    stopMail(person.email, customer);
    await page.getByRole("button", { name: /Reply to Bob Stone/ }).click();
    await page.getByRole("textbox", { name: "Reply to Bob Stone" }).fill("It went out today.");
    await page.getByRole("button", { name: "Send", exact: true }).click();

    const reply = page.getByRole("article", { name: `Message from ${person.name}` });
    await expect(reply).toContainText("Not delivered: The recipient's address bounced or complained before.", {
      timeout: 15_000,
    });

    resumeMail(customer);
    await reply.getByRole("button", { name: "Send again" }).click();
    await expect(reply).not.toContainText("Not delivered", { timeout: 15_000 });
  });
});
