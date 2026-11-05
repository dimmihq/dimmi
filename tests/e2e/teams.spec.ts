import { addTeammate, conversationTeams, handToTeam, startConversation } from "./support/database";
import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";

/*
 * teammateOf is a second person for the test, with an address of their
 * own derived from the first person's.
 */
function teammateOf(person: Person): Person {
  return { name: "Ada Lovelace", email: person.email.replace("grace.", "ada."), password: person.password };
}

/*
 * workspaceOf is a workspace name of the person's own, so workspaces
 * made at the same moment never race for one name or address.
 */
function workspaceOf(person: Person, prefix: string): string {
  return `${prefix} ${person.email.split("@")[0]?.split(".").at(-1)?.toLowerCase() ?? ""}`;
}

it.describe("teams", () => {
  it.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
  });

  it("creates a team, fills it, renames it and deletes it", async ({ page, person, request }) => {
    const teammate = teammateOf(person);
    await createAccount(request, person, workspaceOf(person, "Teams"));
    await createAccount(request, teammate);
    addTeammate(person.email, teammate.email, "member");
    await signIn(page, person);
    await page.goto("/d/settings/teams");

    const teams = page.getByRole("region", { name: "Teams" });
    await expect(teams.getByText("No teams yet.")).toBeVisible();

    await teams.getByRole("button", { name: "New team" }).click();
    const dialog = page.getByRole("alertdialog");
    await dialog.getByRole("textbox", { name: "Team name" }).fill("Support");
    await dialog.getByRole("button", { name: "Create team" }).click();
    await expect(page.getByRole("heading", { name: "Support", level: 1 })).toBeVisible();

    const members = page.getByRole("region", { name: "Members" });
    await expect(members.getByText("Nobody is in this team yet.")).toBeVisible();
    await members.getByRole("button", { name: "Add a member" }).click();
    await page.getByRole("option", { name: teammate.name }).click();
    await expect(members.getByText(teammate.email)).toBeVisible();
    await expect(page.getByText("1 member", { exact: true })).toBeVisible();

    await page.getByRole("textbox", { name: "Team name" }).fill("Customer care");
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(page.getByRole("heading", { name: "Customer care", level: 1 })).toBeVisible();

    await members.getByRole("button", { name: "Remove" }).click();
    await expect(members.getByText("Nobody is in this team yet.")).toBeVisible();

    await page.getByRole("button", { name: "All teams" }).click();
    await expect(teams.getByText("Customer care")).toBeVisible();
    await expect(teams.getByText("0 members")).toBeVisible();

    await teams.getByRole("button", { name: "Manage" }).click();
    await page.getByRole("button", { name: "Delete team" }).click();
    const confirm = page.getByRole("dialog");
    await expect(confirm.getByText("It holds no conversations.")).toBeVisible();
    await confirm.getByRole("button", { name: "Delete team" }).click();
    await expect(page).toHaveURL(/\/d\/settings\/teams$/);
    await expect(teams.getByText("No teams yet.")).toBeVisible();
  });

  it("refuses a name another team already uses", async ({ page, person, request }) => {
    await createAccount(request, person, workspaceOf(person, "Names"));
    await signIn(page, person);
    await page.goto("/d/settings/teams");

    const teams = page.getByRole("region", { name: "Teams" });
    const dialog = page.getByRole("alertdialog");
    const create = async (name: string) => {
      await teams.getByRole("button", { name: "New team" }).click();
      await dialog.getByRole("textbox", { name: "Team name" }).fill(name);
      await dialog.getByRole("button", { name: "Create team" }).click();
    };

    await create("Billing");
    await expect(page.getByRole("heading", { name: "Billing", level: 1 })).toBeVisible();
    await page.getByRole("button", { name: "All teams" }).click();

    await create("billing");
    await expect(dialog.getByText("Another team in this workspace already uses this name.")).toBeVisible();
  });

  it("moves a deleted team's conversations to the team the admin picks", async ({ page, person, request }) => {
    await createAccount(request, person, workspaceOf(person, "Moves"));
    await signIn(page, person);

    const teams = page.getByRole("region", { name: "Teams" });
    for (const name of ["Support", "Billing"]) {
      await page.goto("/d/settings/teams");
      await teams.getByRole("button", { name: "New team" }).click();
      await page.getByRole("alertdialog").getByRole("textbox", { name: "Team name" }).fill(name);
      await page.getByRole("alertdialog").getByRole("button", { name: "Create team" }).click();
      await expect(page.getByRole("heading", { name, level: 1 })).toBeVisible();
    }
    startConversation(person.email);
    startConversation(person.email);
    handToTeam(person.email, "Billing");
    await page.reload();

    await page.getByRole("button", { name: "Delete team" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("It holds 2 open, 0 snoozed and 0 closed conversations.")).toBeVisible();
    const remove = dialog.getByRole("button", { name: "Delete team" });
    await expect(remove).toBeDisabled();

    await dialog.getByRole("radio", { name: "Another team" }).click();
    await expect(remove).toBeDisabled();
    await dialog.getByRole("button", { name: /Team to move them to/ }).click();
    await page.getByRole("option", { name: "Support" }).click();
    await remove.click();

    await expect(page).toHaveURL(/\/d\/settings\/teams$/);
    await expect(teams.getByText("Billing")).toBeHidden();
    expect(conversationTeams(person.email)).toEqual(["Support", "Support"]);
  });
});
