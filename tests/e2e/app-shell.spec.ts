import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";

const pages = [
  ["Notifications", "/d/notifications"],
  ["My inbox", "/d/inbox/mine"],
  ["All", "/d/inbox/all"],
  ["Mentions", "/d/inbox/mentions"],
  ["Agent", "/d/agent"],
  ["Docs", "/d/docs"],
  ["Feature requests", "/d/feedback"],
  ["Changelog", "/d/changelog"],
  ["Announcements", "/d/announcements"],
  ["Customers", "/d/customers"],
  ["Unassigned", "/d/inbox/unassigned"],
] as const;

/*
 * workspaceOf names the person's workspace after them. Workspaces created
 * at the same moment with one name race for its address on the server,
 * so every test brings its own.
 */
function workspaceOf(person: Person): string {
  return `Acme ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

it.describe("the app's frame", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
  });

  it("opens on the inbox with the person's workspace in the switcher", async ({ page, person }) => {
    await expect(page).toHaveURL(inbox);
    await expect(page.getByRole("button", { name: workspaceOf(person) })).toBeVisible();
    await expect(page.getByRole("button", { name: person.name, exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Unassigned" })).toHaveAttribute("aria-current", "page");
  });

  it("opens every page from the sidebar without reloading the app", async ({ page }) => {
    await page.evaluate(() => {
      (window as unknown as { stayed: boolean }).stayed = true;
    });

    for (const [label, path] of pages) {
      await page.getByRole("link", { name: label, exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      await expect(page.getByRole("link", { name: label, exact: true })).toHaveAttribute("aria-current", "page");
    }

    expect(await page.evaluate(() => (window as unknown as { stayed?: boolean }).stayed)).toBe(true);
  });

  it("swaps in the settings pages, and back", async ({ page }) => {
    await page.getByRole("link", { name: "Settings" }).click();
    await expect(page).toHaveURL(/\/d\/settings\/profile$/);
    await expect(page.getByRole("heading", { name: "Profile" })).toBeVisible();

    await page.getByRole("link", { name: "Members" }).click();
    await expect(page).toHaveURL(/\/d\/settings\/members$/);
    await expect(page.getByRole("link", { name: "Agent" })).toBeHidden();

    await page.getByRole("link", { name: "Back to inbox" }).click();
    await expect(page).toHaveURL(inbox);
  });

  it("folds the sidebar to a rail and opens it again", async ({ page }) => {
    await page.getByRole("button", { name: "Collapse sidebar" }).click();
    await expect(page.getByRole("button", { name: "Expand sidebar" })).toBeVisible();

    await page.getByRole("link", { name: "Docs" }).click();
    await expect(page).toHaveURL(/\/d\/docs$/);

    await page.getByRole("button", { name: "Expand sidebar" }).click();
    await expect(page.getByRole("button", { name: "Collapse sidebar" })).toBeVisible();
  });

  it("moves the sidebar into a drawer on a phone", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(page.getByRole("link", { name: "Docs" })).toBeHidden();

    await page.getByRole("button", { name: "Open navigation" }).click();
    await page.getByRole("dialog", { name: "Navigation" }).getByRole("link", { name: "Docs" }).click();

    await expect(page).toHaveURL(/\/d\/docs$/);
  });
});
