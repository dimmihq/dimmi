import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";

const example = /\/d\/inbox\/unassigned\/example$/;

/*
 * workspaceOf names the person's workspace after them, so workspaces
 * created at the same moment never race for one address.
 */
function workspaceOf(person: Person): string {
  return `Inbox ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

it.describe("the inbox", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("opens the example conversation with the sidebar folded to a rail", async ({ page }) => {
    await page.getByRole("link", { name: /Emily Johnson/ }).click();

    await expect(page).toHaveURL(example);
    await expect(page.getByRole("heading", { name: "Can't log in" })).toBeVisible();
    await expect(page.getByRole("article", { name: "Internal note from Operator" })).toContainText(
      "Not visible to Emily Johnson",
    );
    await expect(page.getByRole("button", { name: "Expand sidebar" })).toBeVisible();

    await page.getByRole("link", { name: "Back to the list" }).click();
    await expect(page).toHaveURL(inbox);
    await expect(page.getByRole("button", { name: "Collapse sidebar" })).toBeVisible();
  });

  it("toggles the details and remembers the choice", async ({ page }) => {
    await page.goto("/d/inbox/unassigned/example");
    const details = page.getByRole("region", { name: "Details" });
    await expect(details).toContainText("Northwind");

    await page.getByRole("button", { name: "Hide details" }).click();
    await expect(details).toBeHidden();

    await page.reload();
    await expect(page.getByRole("button", { name: "Show details" })).toBeVisible();
    await expect(details).toBeHidden();
  });

  it("assigns the conversation and works the case from the details", async ({ page, person }) => {
    await page.goto("/d/inbox/unassigned/example");
    const details = page.getByRole("region", { name: "Details" });

    await details.getByRole("button", { name: "Assign to me" }).click();
    await expect(details.getByRole("button", { name: /Assignee/ })).toContainText(person.name);
    await expect(page.getByText(`${person.name} assigned the conversation to ${person.name}`)).toBeVisible();

    await details.getByRole("button", { name: /Assignee/ }).click();
    await page.getByRole("option", { name: "Sam Okafor" }).click();
    await expect(page.getByText(`${person.name} assigned the conversation to Sam Okafor`)).toBeVisible();

    await details.getByRole("button", { name: "Mark as done" }).click();
    await expect(details.getByRole("button", { name: /Status/ })).toContainText("Done");
    await expect(details.getByRole("button", { name: "Reopen" })).toBeVisible();
  });

  it("talks the case over with Operator", async ({ page }) => {
    await page.goto("/d/inbox/unassigned/example");
    const operator = page.getByRole("region", { name: "Operator" });
    await expect(operator).toContainText("Emily can't sign in");

    await operator.getByRole("textbox", { name: "Ask Operator about this conversation" }).fill("Which mail app?");
    await operator.getByRole("button", { name: "Send message" }).click();
    await expect(operator).toContainText("Which mail app?");
    await expect(operator).toContainText("Operator connects once the inbox does");

    await operator.getByRole("button", { name: "Close Operator" }).click();
    await expect(operator).toBeHidden();
    await page.keyboard.press("o");
    await expect(operator).toBeVisible();
  });

  it("opens the composer for a reply or a note and keeps each draft", async ({ page }) => {
    await page.goto("/d/inbox/unassigned/example");
    const reply = page.getByRole("textbox", { name: "Reply to Emily Johnson" });
    await expect(reply).toBeHidden();

    await page.getByRole("button", { name: /Reply to Emily Johnson/ }).click();
    await expect(reply).toBeFocused();
    await reply.fill("Hi Emily");
    await expect(page.getByText("To Emily Johnson · Messenger")).toBeVisible();
    await expect(page.getByRole("button", { name: "Send", exact: true })).toBeDisabled();

    await page.getByRole("radio", { name: "Note" }).click();
    const note = page.getByRole("textbox", { name: "Internal note" });
    await expect(note).toBeFocused();
    await expect(note).toHaveValue("");
    await expect(page.getByText("Only your team sees this")).toBeVisible();

    await page.getByRole("radio", { name: "Reply" }).click();
    await expect(reply).toHaveValue("Hi Emily");
  });

  it("opens the note composer from the keyboard", async ({ page }) => {
    await page.goto("/d/inbox/unassigned/example");
    await expect(page.getByRole("button", { name: /Reply to Emily Johnson/ })).toBeVisible();
    await page.keyboard.press("n");

    await expect(page.getByRole("textbox", { name: "Internal note" })).toBeFocused();
    await expect(page.getByRole("button", { name: "Add note" })).toBeDisabled();
  });

  it("attaches files, uploads them in turn and removes them", async ({ page }) => {
    await page.goto("/d/inbox/unassigned/example");
    await expect(page.getByRole("button", { name: /Reply to Emily Johnson/ })).toBeVisible();
    await page.keyboard.press("r");

    await page.getByLabel("Attach files").setInputFiles([
      { name: "refund-policy.pdf", mimeType: "application/pdf", buffer: Buffer.from("policy") },
      { name: "accounts.csv", mimeType: "text/csv", buffer: Buffer.from("a,b") },
    ]);
    const strip = page.getByRole("list", { name: "Attachments" });
    await expect(strip.getByRole("listitem")).toHaveCount(2);
    await expect(strip.getByRole("button", { name: "Remove accounts.csv" })).toBeEnabled();

    await strip.getByRole("button", { name: "Remove refund-policy.pdf" }).click();
    await expect(strip.getByRole("listitem")).toHaveCount(1);
  });

  it("adds help articles, saved replies and issues from the add menu", async ({ page }) => {
    await page.goto("/d/inbox/unassigned/example");
    await expect(page.getByRole("button", { name: /Reply to Emily Johnson/ })).toBeVisible();
    await page.keyboard.press("r");
    const strip = page.getByRole("list", { name: "Attachments" });
    const menu = page.getByRole("dialog", { name: "Add to message" });

    await page.getByRole("button", { name: "Add to message" }).click();
    await menu.getByRole("button", { name: /^Help article/ }).click();
    await menu.getByRole("textbox", { name: "Search articles" }).fill("expire");
    await expect(menu.getByRole("option")).toHaveCount(1);
    await page.keyboard.press("Enter");
    await expect(strip.getByRole("button", { name: "Remove Why reset links expire" })).toBeVisible();

    await page.getByRole("button", { name: "Add to message" }).click();
    await menu.getByRole("button", { name: /^Saved reply/ }).click();
    await menu.getByRole("option", { name: "Send a sign-in code" }).click();
    await expect(page.getByRole("textbox", { name: "Reply to Emily Johnson" })).toHaveValue(/one-time sign-in code/);

    await page.getByRole("button", { name: "Add to message" }).click();
    await menu.getByRole("button", { name: /^Issue/ }).click();
    await menu.getByRole("option", { name: "Mail previews spend single-use links" }).click();
    await expect(strip.getByRole("listitem")).toHaveCount(2);
  });

  it("slides Operator over the thread on a narrow screen", async ({ page }) => {
    await page.setViewportSize({ width: 900, height: 800 });
    await page.goto("/d/inbox/unassigned/example");
    await page.getByRole("button", { name: "Operator", exact: true }).click();

    const drawer = page.getByRole("dialog", { name: "Operator" });
    await expect(drawer).toContainText("Emily can't sign in");
    await drawer.getByRole("button", { name: "Close Operator" }).click();
    await expect(drawer).toBeHidden();
  });

  it("shows no conversations in the views the example does not land in", async ({ page }) => {
    await page.goto("/d/inbox/mine");
    await expect(page.getByText("No conversations")).toBeVisible();
    await page.goto("/d/inbox/unassigned/missing");
    await expect(page.getByRole("heading", { name: "Can't log in" })).toBeHidden();
  });
});
