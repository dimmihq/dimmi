import type { APIRequestContext, Page } from "@playwright/test";

import { verifyEmail } from "./support/database";
import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";
import { mailedLink } from "./support/mail";

/*
 * inviteeOf is a second person for the test, with an address of their
 * own derived from the first person's.
 */
function inviteeOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

/*
 * suffixOf is the unique part of the person's address, so workspaces
 * made at the same moment never race for one name or address.
 */
function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? "";
}

/*
 * ownerOf creates the person's workspace, proves their address, which
 * inviting needs, and signs them in.
 */
async function ownerOf(page: Page, request: APIRequestContext, person: Person, workspace: string) {
  await createAccount(request, person, `${workspace} ${suffixOf(person)}`);
  verifyEmail(person.email);
  await signIn(page, person);
}

/*
 * invite sends one invitation from the members page, with the role.
 */
async function invite(page: Page, email: string, role: "Member" | "Admin" = "Member") {
  await page.goto("/d/settings/members");
  await page.getByRole("button", { name: "Invite", exact: true }).click();

  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Email addresses").fill(email);
  if (role === "Admin") {
    await dialog.getByRole("button", { name: /Role/ }).click();
    await page.getByRole("option", { name: "Admin" }).click();
  }
  await dialog.getByRole("button", { name: "Send invitations" }).click();
  await expect(dialog.getByText(email)).toBeVisible();
  await expect(dialog.getByText("Invitation sent")).toBeVisible();
  await dialog.getByRole("button", { name: "Done" }).click();
  await expect(dialog).toBeHidden();
}

it.describe("invitations", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("signs a new person up from the mailed link, straight into the workspace", async ({
    page,
    guest,
    person,
    request,
  }) => {
    const invitee = inviteeOf(person);
    await ownerOf(page, request, person, "Welcome");
    await invite(page, invitee.email);

    await guest.goto(await mailedLink(invitee.email, "invited you"));
    await expect(guest.getByRole("heading", { name: `Join Welcome ${suffixOf(person)}` })).toBeVisible();
    await expect(guest).not.toHaveURL(/#token=/);
    await guest.getByLabel("Full name").fill(invitee.name);
    await guest.getByRole("textbox", { name: "Email" }).fill(invitee.email);
    await guest.getByLabel("Password").fill(invitee.password);
    await guest.getByRole("button", { name: "Create account" }).click();
    await expect(guest).toHaveURL(inbox);
    await expect(guest.getByRole("button", { name: invitee.name, exact: true })).toBeVisible();

    await page.reload();
    const members = page.getByRole("region", { name: "Members" });
    await expect(members.getByText(invitee.name)).toBeVisible();
    await expect(page.getByText("2 active")).toBeVisible();
  });

  it("lets someone with an account sign in from the invitation and accept it", async ({
    page,
    guest,
    person,
    request,
  }) => {
    const invitee = inviteeOf(person);
    await createAccount(request, invitee);
    await ownerOf(page, request, person, "Returning");
    await invite(page, invitee.email, "Admin");

    await guest.goto(await mailedLink(invitee.email, "invited you"));
    await guest.getByRole("button", { name: "Sign in to accept" }).click();
    await expect(guest).toHaveURL(/\/login\?redirect=/);
    await guest.getByLabel("Email").fill(invitee.email);
    await guest.getByLabel("Password").fill(invitee.password);
    await guest.getByRole("button", { name: "Sign in", exact: true }).click();

    await expect(guest).toHaveURL(/\/invitation$/);
    await expect(guest.getByRole("heading", { name: `Join Returning ${suffixOf(person)}` })).toBeVisible();
    await guest.getByRole("button", { name: "Accept invitation" }).click();
    await expect(guest).toHaveURL(inbox);

    await page.reload();
    const members = page.getByRole("region", { name: "Members" });
    await expect(members.getByText(invitee.name)).toBeVisible();
    await expect(members.getByText("Admin", { exact: true })).toBeVisible();
  });

  it("lists a pending invitation, sends it again and revokes it", async ({ page, guest, person, request }) => {
    const invitee = inviteeOf(person);
    await ownerOf(page, request, person, "Pending");
    await invite(page, invitee.email);

    const pending = page.getByRole("region", { name: "Pending invitations" });
    await expect(pending.getByText(invitee.email)).toBeVisible();
    await expect(pending.getByText(/^Expires /)).toBeVisible();

    const manage = pending.getByRole("button", { name: `Manage invitation for ${invitee.email}` });
    await manage.click();
    const resent = page.waitForResponse((r) => r.url().endsWith("/deliveries") && r.request().method() === "POST");
    const refreshed = page.waitForResponse(
      (r) => r.url().includes("/api/v1/invitations?") && r.request().method() === "GET",
    );
    await page.getByRole("button", { name: "Resend", exact: true }).click();
    expect((await resent).status()).toBe(201);
    await refreshed;
    await expect(pending.getByText(invitee.email)).toBeVisible();
    await expect(page.getByRole("status").filter({ hasText: `Sent again to ${invitee.email}.` })).toBeVisible();

    await expect(page.getByRole("button", { name: "Resend", exact: true })).toBeHidden();
    await manage.click();
    const revoke = page.getByRole("button", { name: "Revoke", exact: true });
    await expect(revoke).toBeVisible();
    await revoke.click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Revoke invitation" }).click();
    await expect(pending).toBeHidden();

    await guest.goto(await mailedLink(invitee.email, "invited you", 2));
    await expect(guest.getByRole("heading", { name: "This invitation was revoked" })).toBeVisible();
  });

  it("puts the invitee in the picked team and hands over a fresh link", async ({ page, guest, person, request }) => {
    const invitee = inviteeOf(person);
    await ownerOf(page, request, person, "Linked");
    await page.goto("/d/settings/teams");
    await page.getByRole("region", { name: "Teams" }).getByRole("button", { name: "New team" }).click();
    await page.getByRole("alertdialog").getByRole("textbox", { name: "Team name" }).fill("Support");
    await page.getByRole("alertdialog").getByRole("button", { name: "Create team" }).click();
    await expect(page.getByRole("heading", { name: "Support", level: 1 })).toBeVisible();

    await page.goto("/d/settings/members");
    await page.getByRole("button", { name: "Invite", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Email addresses").fill(invitee.email);
    await dialog.getByRole("group", { name: "Teams to join" }).getByText("Support").click();
    await dialog.getByRole("button", { name: "Send invitations" }).click();
    await expect(dialog.getByText("Invitation sent")).toBeVisible();
    await dialog.getByRole("button", { name: "Done" }).click();

    const pending = page.getByRole("region", { name: "Pending invitations" });
    await pending.getByRole("button", { name: `Manage invitation for ${invitee.email}` }).click();
    await page.getByRole("button", { name: "Copy a new link", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Make a new link" }).click();
    const link = page.getByRole("textbox", { name: `Invitation link for ${invitee.email}` });
    await expect(link).toHaveValue(/\/invitation#token=/);
    await expect(page.getByText(/the mailed one no longer works/)).toBeVisible();

    await guest.goto(await mailedLink(invitee.email, "invited you"));
    await expect(guest.getByRole("heading", { name: /^This invitation/ })).toBeVisible();
    await expect(guest.getByRole("heading", { name: `Join Linked ${suffixOf(person)}` })).toBeHidden();

    await guest.goto("about:blank");
    await guest.goto(await link.inputValue());
    await expect(guest.getByRole("heading", { name: `Join Linked ${suffixOf(person)}` })).toBeVisible();
    await guest.getByLabel("Full name").fill(invitee.name);
    await guest.getByRole("textbox", { name: "Email" }).fill(invitee.email);
    await guest.getByLabel("Password").fill(invitee.password);
    await guest.getByRole("button", { name: "Create account" }).click();
    await expect(guest).toHaveURL(inbox);

    await page.goto("/d/settings/teams");
    await expect(page.getByRole("region", { name: "Teams" }).getByText("1 member", { exact: true })).toBeVisible();
  });
});
