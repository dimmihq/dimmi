import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { deliverEmail, rawEmail } from "./support/inbound";
import { openAddress } from "./support/inbox";

/*
 * suffixOf is the unique part of the person's address, so workspaces
 * made at the same moment never race for one name.
 */
function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

const body = [
  "The <div> tag breaks and 2*3*4 shows wrong.",
  "",
  "On Tue, Nov 3, 2026 at 9:00 AM Support <help@example.com> wrote:",
  "> Which page shows the problem?",
  "",
].join("\r\n");

/*
 * htmlEmail writes an HTML email whose quoted thread its client marked.
 */
function htmlEmail(from: string, to: string): string {
  return [
    `From: Bob Stone <${from}>`,
    `To: ${to}`,
    "Subject: Styled question",
    "Date: Tue, 3 Nov 2026 10:00:00 +0000",
    `Message-ID: <${Date.now()}.html@customer.test>`,
    "MIME-Version: 1.0",
    "Content-Type: text/html; charset=utf-8",
    "",
    '<div style="color:#333">Is the <b>refund</b> on its way?</div>',
    '<div class="gmail_quote">On Mon, Support wrote:<blockquote>We sent it Monday.</blockquote></div>',
    "",
  ].join("\r\n");
}

it.describe("a received email in the thread", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("shows plain text as written and opens the full message", async ({ page, person, request }) => {
    await createAccount(request, person, `Original ${suffixOf(person)}`);
    await signIn(page, person);
    await openAddress(page);
    const address = (await page.getByTestId("forwarding-address").first().textContent()) ?? "";
    await deliverEmail(
      request,
      rawEmail(`bob.${suffixOf(person)}@customer.test`, address, "Broken page", body),
      address,
    );

    await page.goto("/d/inbox/unassigned");
    const row = page.getByRole("link", { name: /Broken page/ });
    await expect(async () => {
      await page.reload();
      await expect(row).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 15_000 });
    await row.click();

    const message = page.getByRole("article", { name: "Message from Bob Stone" });
    await expect(message).toContainText("The <div> tag breaks and 2*3*4 shows wrong.");
    await expect(message).not.toContainText("Which page shows the problem?");

    await message.getByRole("button", { name: "Show full message" }).click();
    const dialog = page.getByRole("dialog", { name: "Full message" });
    await expect(dialog).toContainText("> Which page shows the problem?");

    const download = page.waitForEvent("download");
    await dialog.getByRole("button", { name: "Download original" }).click();
    expect((await download).suggestedFilename()).toMatch(/^msg_.+\.eml$/);

    await dialog.getByRole("button", { name: "Done" }).click();
    await expect(dialog).toBeHidden();
  });

  it("shows an html email as it was formatted in a sandboxed frame", async ({ page, person, request }) => {
    await createAccount(request, person, `Styled ${suffixOf(person)}`);
    await signIn(page, person);
    await openAddress(page);
    const address = (await page.getByTestId("forwarding-address").first().textContent()) ?? "";
    await deliverEmail(request, htmlEmail(`ann.${suffixOf(person)}@customer.test`, address), address);

    await page.goto("/d/inbox/unassigned");
    const row = page.getByRole("link", { name: /Styled question/ });
    await expect(async () => {
      await page.reload();
      await expect(row).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 15_000 });
    await row.click();

    const message = page.getByRole("article", { name: "Message from Bob Stone" });
    await expect(message).toContainText("Is the refund on its way?");
    await expect(message).not.toContainText("We sent it Monday.");

    await message.getByRole("button", { name: "Show full message" }).click();
    const dialog = page.getByRole("dialog", { name: "Full message" });
    const frame = dialog.locator('iframe[title="Email as it was formatted"]');
    await expect(frame).toBeVisible();
    await expect(frame).toHaveAttribute("srcdoc", /We sent it Monday\./);
    await expect(frame).toHaveAttribute("sandbox", "allow-popups allow-popups-to-escape-sandbox");

    await dialog.getByRole("radio", { name: "Plain text" }).click();
    await expect(dialog).toContainText("We sent it Monday.");
  });
});
