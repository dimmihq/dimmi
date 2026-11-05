/*
 * Undo send: an email reply waits a few seconds before it leaves. Undone
 * in time, no mail goes out and the text is back in the composer; left
 * alone, the mail reaches the customer once the wait is over.
 */
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { deliverEmail, rawEmail } from "./support/inbound";
import { openAddress, openComposer } from "./support/inbox";
import { mailedTexts } from "./support/mail";

function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("undo send", () => {
  it("takes an email reply back within the wait, and sends one left alone", async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Undo ${suffixOf(person)}`);
    await signIn(page, person);
    await openAddress(page);
    const address = (await page.getByTestId("forwarding-address").first().textContent()) ?? "";
    const customer = `bob.${suffixOf(person)}@customer.test`;
    await deliverEmail(request, rawEmail(customer, address, "Refund for order 1042", "Charged twice."), address);

    await expect(async () => {
      await page.goto("/d/inbox/all");
      await expect(page.getByRole("link", { name: /Refund for order 1042/ })).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 15_000 });
    await page.getByRole("link", { name: /Refund for order 1042/ }).click();

    await openComposer(page, /^Reply to /);
    const field = page.getByRole("textbox", { name: /^Reply to / });
    await field.fill("Refunding the second charge");
    await page.getByRole("button", { name: "Send", exact: true }).click();
    const waiting = page.getByRole("status", { name: "Reply waiting to send" });
    await expect(waiting).toContainText("Sending in");
    await expect(page.getByRole("article", { name: /Message from/ }).last()).toContainText("Sending");

    await page.keyboard.press("ControlOrMeta+z");
    await expect(waiting).toHaveCount(0);
    await expect(field).toHaveText("Refunding the second charge");
    await expect(page.getByText("Refunding the second charge")).toHaveCount(1);

    await page.waitForTimeout(7000);
    expect((await mailedTexts(customer)).some((text) => text.includes("Refunding the second charge"))).toBe(false);

    await field.fill("Refunded, it shows in five days");
    await page.getByRole("button", { name: "Send", exact: true }).click();
    await expect(waiting).toContainText("Sending in");
    await expect(async () => {
      expect((await mailedTexts(customer)).some((text) => text.includes("Refunded, it shows in five days"))).toBe(true);
    }).toPass({ timeout: 20_000 });
    await expect(waiting).toHaveCount(0);
  });
});
