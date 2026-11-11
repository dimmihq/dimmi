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
 * insertDetail types a slash and the start of a detail's name in the
 * writer, and takes the first match as a chip.
 */
async function insertDetail(page: Page, query: string) {
  await page.keyboard.type(`/${query}`);
  await expect(page.getByRole("listbox", { name: "Details" })).toBeVisible();
  await page.keyboard.press("Enter");
}

/*
 * storedBodies reads the signatures the API keeps for the filter.
 */
async function storedBodies(page: Page, filter: string): Promise<string[]> {
  const response = await page.request.get(`/api/v1/email-signatures?${filter}`);
  const { data } = (await response.json()) as { data: { body: string }[] };

  return data.map((signature) => signature.body);
}

it.describe("email signatures", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("writes a teammate's own signature on its own page and previews it closing a reply", async ({
    page,
    person,
    request,
  }) => {
    await createAccount(request, person, `Signatures ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await page.goto("/d/settings/profile");

    await page.getByRole("link", { name: "Add your signature" }).click();
    await expect(page).toHaveURL(/\/settings\/signatures\/mine$/);
    const writer = page.getByRole("textbox", { name: "Your signature" });
    await writer.click();
    await page.keyboard.type("Thanks,");
    await page.keyboard.press("Enter");
    await insertDetail(page, "teammate");
    await page.keyboard.press("Enter");
    await page.getByRole("button", { name: "Insert a detail" }).click();
    await page
      .getByRole("dialog", { name: "Details" })
      .getByRole("button", { name: /^Job title/ })
      .click();
    await expect(page.getByRole("dialog", { name: "Details" })).toHaveCount(0);
    await expect(writer.getByRole("button", { name: "Teammate name" })).toBeVisible();
    await expect(writer).not.toContainText("{{");

    const preview = page.getByTitle("Your signature preview");
    await expect(preview).toHaveAttribute("srcdoc", new RegExp(person.name));
    await expect(preview).toHaveAttribute("srcdoc", /refund is on its way/);

    await page.getByRole("button", { name: "Save signature" }).click();
    await expect(page).toHaveURL(/\/settings\/profile#signature$/);
    await expect(page.getByRole("switch", { name: "Use your signature" })).toBeChecked();
    await expect(page.getByTitle("Your signature preview")).toHaveAttribute("srcdoc", new RegExp(person.name));
    expect(await storedBodies(page, "filter[member]=me")).toEqual(["Thanks,\n{{member_name}}\n{{member_title}}"]);

    await page.getByRole("link", { name: "Edit your signature" }).click();
    await writer.click();
    await page.keyboard.press("End");
    await page.keyboard.type(" {{nickname}}");
    await expect(page.getByRole("alert")).toContainText("{{member_name}}");

    await page.getByRole("button", { name: "Back" }).click();
    const leaving = page.getByRole("alertdialog", { name: "Leave without saving?" });
    await leaving.getByRole("button", { name: "Keep writing" }).click();
    await expect(page).toHaveURL(/\/settings\/signatures\/mine$/);
    await page.getByRole("button", { name: "Back" }).click();
    await leaving.getByRole("button", { name: "Leave" }).click();
    await expect(page).toHaveURL(/\/settings\/profile#signature$/);
    expect(await storedBodies(page, "filter[member]=me")).toEqual(["Thanks,\n{{member_name}}\n{{member_title}}"]);
  });

  it("returns to the address or the teammates list a signature was opened from", async ({
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
    const first = ((await addresses.json()) as { data: { id: string }[] }).data[0]?.id ?? "";
    await page.goto(`/d/settings/email/addresses/${first}`);
    await page.getByRole("link", { name: "Add team signature" }).click();
    await expect(page).toHaveURL(new RegExp(`/settings/signatures/address\\?address=${first}$`));
    await page.getByRole("textbox", { name: /^Signature for / }).click();
    await page.keyboard.type("The ");
    await insertDetail(page, "address");
    await page.keyboard.type("team");
    await page.getByRole("button", { name: "Save signature" }).click();
    await expect(page).toHaveURL(new RegExp(`/settings/email/addresses/${first}#signature$`));
    await expect(page.getByRole("switch", { name: "Use team signature" })).toBeChecked();
    expect(await storedBodies(page, "filter[kind]=address")).toEqual(["The {{address_name}} team"]);

    await page.getByRole("link", { name: "Edit team signature" }).click();
    await expect(page.getByRole("textbox", { name: /^Signature for / }).getByRole("button")).toHaveText("Address name");
    await page.getByRole("button", { name: "Back" }).click();
    await expect(page).toHaveURL(new RegExp(`/settings/email/addresses/${first}#signature$`));

    const other = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const member = await other.newPage();
    await signIn(member, teammate);
    await member.goto(`/d/settings/email/addresses/${first}`);
    await expect(member.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(member.getByRole("link", { name: "Add team signature" })).toHaveCount(0);
    await member.goto("/d/settings/profile");
    await member.getByRole("link", { name: "Add your signature" }).click();
    await member.getByRole("textbox", { name: "Your signature" }).click();
    await member.keyboard.type("Ada, ");
    await insertDetail(member, "company");
    await member.getByRole("button", { name: "Save signature" }).click();
    await expect(member.getByRole("switch", { name: "Use your signature" })).toBeChecked();
    await other.close();

    await page.goto("/d/settings/email");
    await page.getByRole("link", { name: `Edit signature of ${teammate.name}` }).click();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(`Signature of ${teammate.name}`);
    const theirs = page.getByRole("textbox", { name: `Signature of ${teammate.name}` });
    await expect(theirs.getByRole("button", { name: "Company name" })).toBeVisible();
    await theirs.click();
    await page.keyboard.press("End");
    await page.keyboard.type(" Founder");
    await page.getByRole("button", { name: "Save signature" }).click();
    await expect(page).toHaveURL(/\/settings\/email#signature-sig_[0-9a-z]+$/);
    await expect(page.getByTitle(`Signature of ${teammate.name} preview`)).toHaveAttribute("srcdoc", /Founder/);
    expect(await storedBodies(page, "filter[kind]=personal")).toContain("Ada, {{company_name}} Founder");
    await expect(page.getByRole("link", { name: `Edit signature of ${person.name}` })).toHaveCount(0);
  });
});
