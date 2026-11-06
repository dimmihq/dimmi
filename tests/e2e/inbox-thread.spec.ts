/*
 * Reading a thread: code in a note is highlighted, copies and opens full
 * screen; every time shows its full date on hover; and Cmd+E folds the
 * events away into a count, kept after a reload.
 */
import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, callAPI, startConversation } from "./support/inbox";

const omar: Customer = {
  name: "Omar Haddad",
  email: "omar@northwind.example",
  subject: "Webhook payload",
  body: "The total field looks wrong.",
};

function workspaceOf(person: Person): string {
  return `Thread ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

it.describe("thread extras", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("highlights code, shows full dates and folds events", async ({ page, person }) => {
    const id = await startConversation(page, omar);
    await callAPI(page, "POST", `/conversations/${id}/notes`, { body: "```js\nconst total = 42;\n```" }, 201);
    await callAPI(page, "PATCH", `/conversations/${id}`, { status: "closed" }, 200);
    await callAPI(page, "PATCH", `/conversations/${id}`, { status: "open" }, 200);
    await page.goto(`/d/inbox/all/${id}`);

    const note = page.getByRole("article", { name: `Internal note from ${person.name}` });
    await expect(note.locator(".hljs-keyword")).toHaveText("const");
    await note.locator("pre").hover();
    await note.getByRole("button", { name: "Show code full screen" }).click();
    const code = page.getByRole("dialog", { name: "Code" });
    await expect(code).toContainText("const total = 42;");
    await page.keyboard.press("Escape");
    await expect(code).toHaveCount(0);

    await note.locator("pre").first().click();
    await note.locator("time").hover();
    await expect(page.getByRole("tooltip")).toContainText(String(new Date().getFullYear()));

    await expect(page.getByText(`${person.name} closed the conversation`)).toBeVisible();
    await page.locator("body").click({ position: { x: 5, y: 5 } });
    const saved = page.waitForResponse(
      (r) => r.url().endsWith("/inbox-preferences") && r.request().method() === "PATCH",
    );
    await page.keyboard.press("ControlOrMeta+e");
    await expect(page.getByText("2 events")).toBeVisible();
    await saved;
    await expect(page.getByText(`${person.name} closed the conversation`)).toHaveCount(0);

    await page.reload();
    await expect(page.getByText("2 events")).toBeVisible();
    await page.keyboard.press("ControlOrMeta+e");
    await expect(page.getByText(`${person.name} reopened the conversation`)).toBeVisible();
  });
});
