import { addTeammate } from "./support/database";
import { createAccount, expect, test as it, signIn } from "./support/fixtures";
import { startConversation } from "./support/inbox";

it.describe("custom roles", () => {
  it("an admin makes a read only role and a member with it cannot reply", async ({ page, guest, person, request }) => {
    const suffix = person.email.split("@")[0]?.split(".").at(-1) ?? "";
    const teammate = { ...person, name: "Ada Lovelace", email: `ada.${person.email}` };
    await createAccount(request, person, `Roles ${suffix}`);
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await page.setViewportSize({ width: 1440, height: 900 });
    await signIn(page, person);
    const conversation = await startConversation(page, {
      name: "Emily Johnson",
      email: "emily@northwind.example",
      subject: "Can't log in",
      body: "My password is refused.",
    });

    await page.goto("/d/settings/roles");
    const custom = page.getByRole("region", { name: "Custom roles" });
    await custom.getByRole("button", { name: "New role" }).click();
    await page.getByRole("button", { name: "Read only" }).click();
    const editor = page.getByRole("dialog", { name: "New role" });
    await expect(editor.getByRole("switch", { name: /See members/ })).toBeChecked();
    await expect(editor.getByRole("switch", { name: /Reply to conversations/ })).not.toBeChecked();
    await editor.getByRole("button", { name: "Create role" }).click();
    await expect(custom.getByText("Read only")).toBeVisible();

    await page.goto("/d/settings/members");
    await page.getByRole("button", { name: "Manage Ada Lovelace" }).click();
    await page.getByRole("button", { name: "Make Read only" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Make Read only" }).click();
    await expect(page.getByRole("region", { name: "Members" }).getByText("Read only")).toBeVisible();

    await guest.setViewportSize({ width: 1440, height: 900 });
    await signIn(guest, teammate);
    await guest.goto(`/d/inbox/unassigned/${conversation}`);
    await guest.getByRole("button", { name: /Reply to Emily Johnson/ }).click();
    await guest.getByRole("textbox", { name: "Reply to Emily Johnson" }).fill("I should not be able to send this.");
    await guest.getByRole("button", { name: "Send", exact: true }).click();

    await expect(guest.getByText("This action is unauthorized.")).toBeVisible();
    await expect(guest.getByRole("article", { name: "Message from Ada Lovelace" })).toHaveCount(0);
  });
});
