import { randomUUID } from "node:crypto";

import { addTeammate, markTwoFactorOn } from "./support/database";
import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";
import { mailedLink } from "./support/mail";

/*
 * teammateOf is a second person for the test, with an address of their
 * own derived from the first person's.
 */
function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

it.describe("helping a teammate with two-factor", () => {
  it("an admin starts a reset and the teammate finishes it from the email", async ({
    page,
    guest,
    person,
    request,
  }) => {
    const teammate = teammateOf(person);
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Help ${randomUUID().slice(0, 8)}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    markTwoFactorOn(teammate.email);
    await signIn(page, person);
    await page.goto("/d/settings/members");

    await page
      .getByRole("region", { name: "Members" })
      .getByRole("button", { name: `Manage ${teammate.name}` })
      .click();
    await page.getByRole("button", { name: "Help with two-factor" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("heading", { name: `Help ${teammate.name} with two-factor` })).toBeVisible();
    await dialog.getByRole("button", { name: "Send reset link" }).click();
    await expect(dialog.getByRole("heading", { name: "Link sent" })).toBeVisible();
    await dialog.getByRole("button", { name: "Done" }).click();

    await guest.goto(await mailedLink(teammate.email, "Reset two-factor"));
    await expect(guest.getByRole("heading", { name: "Reset two-factor authentication" })).toBeVisible();
    await guest.getByRole("button", { name: "Turn off two-factor authentication" }).click();
    await guest.getByLabel("Password").fill("not-my-password");
    await guest.getByRole("button", { name: "Turn off two-factor authentication" }).click();
    await expect(guest.getByText("These credentials do not match our records.")).toBeVisible();
    await guest.getByLabel("Password").fill(teammate.password);
    await guest.getByRole("button", { name: "Turn off two-factor authentication" }).click();
    await expect(guest.getByRole("heading", { name: "Two-factor authentication is off" })).toBeVisible();

    await guest.getByRole("link", { name: "Sign in" }).click();
    await guest.getByLabel("Email").fill(teammate.email);
    await guest.getByLabel("Password").fill(teammate.password);
    await guest.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(guest).toHaveURL(inbox);
  });
});
