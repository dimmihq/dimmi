import { addTeammate, verifyEmail } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

/*
 * teammateOf is a second person for the test, with an address of their
 * own derived from the first person's.
 */
function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

/*
 * suffixOf is the unique part of the person's address, so workspaces
 * made at the same moment never race for one name or address.
 */
function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

it.describe("member import", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("checks a file first, then applies it", async ({ page, person, request }) => {
    const teammate = teammateOf(person);
    const newcomer = person.email.replace("grace.", "newcomer.");
    await createAccount(request, person, `Import ${suffixOf(person)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    verifyEmail(person.email);
    await signIn(page, person);
    await page.goto("/d/settings/members");

    const members = page.getByRole("region", { name: "Members" });
    await members.getByRole("button", { name: "Import" }).click();
    const dialog = page.getByRole("dialog", { name: "Import members" });
    const csv = ["email,role", `${teammate.email},admin`, `${newcomer},`, "not-an-address,"].join("\n");
    await dialog.locator('input[type="file"]').setInputFiles({
      name: "members.csv",
      mimeType: "text/csv",
      buffer: Buffer.from(csv),
    });

    await expect(
      dialog.getByText("This file has 1 to invite, 1 to change, 0 unchanged, 1 with problems."),
    ).toBeVisible();
    await expect(dialog.getByText("Row 4, not-an-address: The email must be a valid email address.")).toBeVisible();
    await expect(members.getByText("Admin", { exact: true })).toBeHidden();

    await dialog.getByRole("button", { name: "Apply changes" }).click();
    await expect(dialog.getByText("Done: 1 invited, 1 changed, 0 unchanged, 1 with problems.")).toBeVisible();
    await dialog.getByRole("button", { name: "Done" }).click();

    await expect(members.getByText("Admin", { exact: true })).toBeVisible();
    await expect(page.getByRole("region", { name: "Pending invitations" }).getByText(newcomer)).toBeVisible();
  });
});
