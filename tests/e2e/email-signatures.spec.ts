import type { Page } from "@playwright/test";

import { addTeammate } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

/*
 * teammateOf is a second person for the same test.
 */
function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

/*
 * writeSignature types the signature into the named field and waits for
 * its preview to show the expected words.
 */
async function writeSignature(page: Page, field: string, body: string, shown: string) {
  await page.getByRole("textbox", { name: field }).fill(body);
  await expect(page.getByTitle(`${field} preview`)).toHaveAttribute("srcdoc", new RegExp(shown));
}

it.describe("email signatures", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("keeps a teammate's own signature and previews it as a reply shows it", async ({ page, person, request }) => {
    await createAccount(request, person, `Signatures ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await page.goto("/d/settings/profile");

    await writeSignature(page, "Your signature", "Thanks,\n**{{member_name}}**\n{{member_title}}", person.name);
    await page.getByRole("button", { name: "Save signature" }).click();
    await expect(page.getByRole("switch", { name: "Use your signature" })).toBeChecked();

    await page.getByRole("textbox", { name: "Your signature" }).fill("Cheers {{nickname}}");
    await expect(page.getByRole("alert")).toContainText("{{member_name}}");

    await page.reload();
    await expect(page.getByRole("textbox", { name: "Your signature" })).toHaveValue(
      "Thanks,\n**{{member_name}}**\n{{member_title}}",
    );
  });

  it("lets only those who manage everyone's signatures write the team's", async ({
    page,
    person,
    request,
    browser,
  }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, `Team ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, person);
    await page.goto("/d/settings/email");

    const addresses = await page.request.get("/api/v1/email-addresses");
    const first = ((await addresses.json()) as { data: { address: string }[] }).data[0]?.address ?? "";
    const name = `Signature for ${first}`;
    await writeSignature(page, name, "The {{address_name}} team", "team");
    await page.getByRole("button", { name: "Save signature" }).first().click();
    await expect(page.getByRole("switch", { name: `Use ${name.toLowerCase()}` })).toBeChecked();

    const other = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const member = await other.newPage();
    await signIn(member, teammate);
    await member.goto("/d/settings/email");
    await expect(member.getByRole("heading", { level: 1, name: "Email" })).toBeVisible();
    await expect(member.getByRole("region", { name: "Team signatures" })).toHaveCount(0);
    await member.goto("/d/settings/profile");
    await expect(member.getByRole("textbox", { name: "Your signature" })).toBeVisible();
    await other.close();
  });
});
