import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, openComposer, startConversation } from "./support/inbox";

const emily: Customer = {
  name: "Emily Johnson",
  email: "emily@northwind.example",
  subject: "Can't log in",
  body: "My password manager's password is refused.",
};

const omar: Customer = {
  name: "Omar Haddad",
  email: "omar@contoso.example",
  subject: "Invoice question",
  body: "The VAT number on my invoice is wrong.",
};

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

  it("shows an empty view, then a conversation a customer starts", async ({ page }) => {
    await expect(page.getByText("No conversations")).toBeVisible();

    await startConversation(page, emily);
    await page.reload();

    const row = page.getByRole("link", { name: /Emily Johnson/ });
    await expect(row).toContainText("Can't log in");
    await expect(row).toContainText("Unread");
    await expect(page.getByText("1 open")).toBeVisible();

    await row.click();
    await expect(page).toHaveURL(/\/d\/inbox\/unassigned\/conv_/);
    await expect(page.getByRole("heading", { name: "Can't log in" })).toBeVisible();
    await expect(page.getByRole("article", { name: "Message from Emily Johnson" })).toContainText("password manager");
    await expect(page.getByRole("region", { name: "Details" })).toContainText("emily@northwind.example");
  });

  it("replies to the customer and takes the conversation", async ({ page, person }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);

    await openComposer(page, /Reply to Emily Johnson/);
    await page.getByRole("textbox", { name: "Reply to Emily Johnson" }).fill("Try a sign-in code instead.");
    await page.getByRole("button", { name: "Send", exact: true }).click();

    await expect(page.getByRole("article", { name: `Message from ${person.name}` })).toContainText(
      "Try a sign-in code instead.",
    );
    await expect(page.getByRole("textbox", { name: "Reply to Emily Johnson" })).toHaveText("");
    await expect(page.getByRole("region", { name: "Details" }).getByRole("button", { name: /Assignee/ })).toContainText(
      person.name,
    );
  });

  it("sets a priority from the keyboard and shows it on the row", async ({ page }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await expect(page.getByRole("heading", { name: "Can't log in" })).toBeVisible();

    await page.keyboard.press("p");
    await expect(page.getByRole("dialog", { name: "Set priority" })).toBeVisible();
    await page.keyboard.press("5");

    await expect(page.getByRole("button", { name: "Priority: Urgent" })).toBeVisible();
    await expect(page.getByText("set the priority to urgent")).toBeVisible();

    await page.goto("/d/inbox/unassigned");
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toContainText("Urgent");
  });

  it("adds a note only the team sees", async ({ page, person }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await expect(page.getByRole("button", { name: /Reply to Emily Johnson/ })).toBeVisible();

    await page.keyboard.press("n");
    await expect(page.getByRole("toolbar", { name: "Formatting" })).toBeVisible({ timeout: 15_000 });
    await page.getByRole("textbox", { name: "Internal note" }).fill("Outlook previews may spend the link.");
    await page.getByRole("button", { name: "Add note" }).click();

    await expect(page.getByRole("article", { name: `Internal note from ${person.name}` })).toContainText(
      "Not visible to Emily Johnson",
    );
  });

  it("assigns, closes and reopens the conversation from the details", async ({ page, person }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    const details = page.getByRole("region", { name: "Details" });

    await details.getByRole("button", { name: "Assign to me" }).click();
    await expect(details.getByRole("button", { name: /Assignee/ })).toContainText(person.name);
    await expect(page.getByText(`${person.name} assigned the conversation to ${person.name}`)).toBeVisible();

    await details.getByRole("button", { name: /Assignee/ }).click();
    await page.getByRole("option", { name: "Unassigned" }).click();
    await expect(page.getByText(`${person.name} unassigned the conversation`)).toBeVisible();

    await details.getByRole("button", { name: "Mark as done" }).click();
    await expect(details.getByRole("button", { name: /Status/ })).toContainText("Done");
    await expect(page.getByText(`${person.name} closed the conversation`)).toBeVisible();

    await details.getByRole("button", { name: "Reopen" }).click();
    await expect(details.getByRole("button", { name: /Status/ })).toContainText("Open");
    await expect(page.getByText(`${person.name} reopened the conversation`)).toBeVisible();

    await details.getByRole("button", { name: "Snooze" }).click();
    await expect(details.getByRole("button", { name: /Status/ })).toContainText("Snoozed");
  });

  it("searches the view's conversations", async ({ page }) => {
    await startConversation(page, emily);
    await startConversation(page, omar);
    await page.goto("/d/inbox/all");
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toBeVisible();

    await page.getByRole("textbox", { name: "Search conversations" }).fill("invoice");
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toBeVisible();
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toBeHidden();

    await page.getByRole("textbox", { name: "Search conversations" }).fill("nothing like this");
    await expect(page.getByText("No matches")).toBeVisible();

    await page.getByRole("textbox", { name: "Search conversations" }).fill("haddad");
    await expect(page.getByRole("link", { name: /Omar Haddad/ })).toBeVisible();
    await expect(page.getByRole("link", { name: /Emily Johnson/ })).toBeHidden();
  });

  it("formats a reply by keyboard, keeps it as Markdown and shows it formatted", async ({ page }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await openComposer(page, /Reply to Emily Johnson/);
    const field = page.getByRole("textbox", { name: "Reply to Emily Johnson" });
    await field.click();
    await page.keyboard.type("Use ");
    await page.keyboard.press("ControlOrMeta+b");
    await page.keyboard.type("a sign-in code");
    await page.keyboard.press("ControlOrMeta+b");
    await page.keyboard.press("Enter");
    await page.keyboard.type("- open the app, see docs");
    await field.evaluate((node) => {
      const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
      for (let text = walker.nextNode(); text; text = walker.nextNode()) {
        const at = text.textContent?.indexOf("docs") ?? -1;
        if (at >= 0) {
          const range = document.createRange();
          range.setStart(text, at);
          range.setEnd(text, at + 4);
          window.getSelection()?.removeAllRanges();
          window.getSelection()?.addRange(range);
        }
      }
    });
    await page.keyboard.press("ControlOrMeta+Shift+u");
    await page.getByRole("textbox", { name: "Link address" }).fill("https://help.example/sign-in");
    await page.keyboard.press("Enter");
    const sent = page.waitForRequest((r) => r.url().endsWith("/replies") && r.method() === "POST");
    await page.getByRole("button", { name: "Send", exact: true }).click();

    expect((await sent).postDataJSON().body).toBe(
      "Use **a sign-in code**\n\n- open the app, see [docs](https://help.example/sign-in)",
    );
    const reply = page.getByRole("article", { name: /Message from/ }).last();
    await expect(reply.locator("strong")).toHaveText("a sign-in code");
    await expect(reply.getByRole("listitem")).toHaveCount(1);
    await expect(reply.getByRole("link", { name: "docs" })).toHaveAttribute("target", "_blank");
    await expect(reply).not.toContainText("**");
  });

  it("keeps only what Markdown can carry from pasted pages", async ({ page }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await openComposer(page, /Reply to Emily Johnson/);
    const field = page.getByRole("textbox", { name: "Reply to Emily Johnson" });
    await field.click();
    await field.evaluate((node) => {
      const data = new DataTransfer();
      data.setData("text/html", "<h1>Steps</h1><p><u>Open</u> <b>Settings</b> <span style='color:red'>now</span></p>");
      node.dispatchEvent(new ClipboardEvent("paste", { clipboardData: data, bubbles: true, cancelable: true }));
    });
    const sent = page.waitForRequest((r) => r.url().endsWith("/replies") && r.method() === "POST");
    await page.getByRole("button", { name: "Send", exact: true }).click();

    expect((await sent).postDataJSON().body).toBe("Steps\n\nOpen **Settings** now");
  });

  it("talks the case over with Operator", async ({ page }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    const operator = page.getByRole("region", { name: "Operator" });

    await operator.getByRole("textbox", { name: "Ask Operator about this conversation" }).fill("Which mail app?");
    await operator.getByRole("button", { name: "Send message" }).click();
    await expect(operator).toContainText("Which mail app?");

    await operator.getByRole("button", { name: "Close Operator" }).click();
    await expect(operator).toBeHidden();
    await page.keyboard.press("o");
    await expect(operator).toBeVisible();
  });

  it("toggles the details and remembers the choice", async ({ page }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    const details = page.getByRole("region", { name: "Details" });
    await expect(details).toContainText("Emily Johnson");

    await page.getByRole("button", { name: "Hide details" }).click();
    await expect(details).toBeHidden();

    await page.reload();
    await expect(page.getByRole("button", { name: "Show details" })).toBeVisible();
    await expect(details).toBeHidden();
  });

  it("keeps each draft and drops picked workspace items into the text", async ({ page }) => {
    const id = await startConversation(page, emily);
    await page.goto(`/d/inbox/unassigned/${id}`);
    const reply = page.getByRole("textbox", { name: "Reply to Emily Johnson" });

    await openComposer(page, /Reply to Emily Johnson/);
    await expect(reply).toBeFocused();
    await expect(page.getByRole("button", { name: "Send", exact: true })).toBeDisabled();
    await reply.fill("Hi Emily");

    await page.getByRole("radio", { name: "Note" }).click();
    await expect(page.getByRole("textbox", { name: "Internal note" })).toHaveText("");
    await page.getByRole("radio", { name: "Reply" }).click();
    await expect(reply).toHaveText("Hi Emily");

    await expect(page.getByLabel("Attach files")).toHaveCount(0);
    await page.getByRole("button", { name: "Add to message" }).click();
    const menu = page.getByRole("dialog", { name: "Add to message" });
    await expect(menu).not.toContainText("Files and photos");
    await menu.getByRole("button", { name: /^Help article/ }).click();
    await menu.getByRole("option", { name: /Reset your password/ }).click();
    await expect(reply).toContainText("Hi Emily");
    await expect(reply).toContainText("Reset your password (Help center · Account)");
  });

  it("slides Operator over the thread on a narrow screen", async ({ page }) => {
    const id = await startConversation(page, emily);
    await page.setViewportSize({ width: 900, height: 800 });
    await page.goto(`/d/inbox/unassigned/${id}`);
    await page.getByRole("button", { name: "Operator", exact: true }).click();

    const drawer = page.getByRole("dialog", { name: "Operator" });
    await expect(drawer).toContainText("Operator connects later");
    await drawer.getByRole("button", { name: "Close Operator" }).click();
    await expect(drawer).toBeHidden();
  });

  it("says when a conversation does not exist", async ({ page }) => {
    await page.goto("/d/inbox/unassigned/conv_01jbd0k3a8x9t2m4q5r6s7v8w9");
    await expect(page.getByText("You do not have access to this conversation")).toBeVisible();
  });
});
