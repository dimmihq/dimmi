import { createAccount, expect, test as it, type Person, signIn } from "./support/fixtures";
import { openMessenger } from "./support/inbox";

const pages = [
  "Profile",
  "Preferences",
  "Security & access",
  "API tokens",
  "Inboxes",
  "Tags",
  "Saved replies",
  "Messengers",
  "Email",
  "Connections",
  "Dimmi agent",
  "Actions",
  "AI providers",
  "Help centers",
  "Knowledge",
  "Changelog",
  "Feedback",
  "Announcements & surveys",
  "Webhooks",
  "Identity verification",
  "Authorized apps",
  "GitHub",
  "Workspace",
  "Members",
  "Teams",
  "Security",
  "Audit log",
  "Import & export",
  "Billing",
];

/*
 * workspaceOf names the person's workspace after them, so workspaces
 * created at the same moment never race for one address.
 */
function workspaceOf(person: Person): string {
  return `Settings ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

it.describe("settings", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
    await page.goto("/d/settings/profile");
  });

  it("opens every settings page from the settings sidebar", async ({ page }) => {
    const sidebar = page.getByRole("complementary").first();
    for (const label of pages) {
      await sidebar.getByRole("link", { name: label, exact: true }).click();
      await expect(page.getByRole("heading", { level: 1, name: label, exact: true })).toBeVisible();
    }
  });

  it("shows the person and their workspace where they belong", async ({ page, person }) => {
    await expect(page.getByRole("textbox", { name: "Full name" })).toHaveValue(person.name);
    await expect(page.getByRole("region", { name: "You" }).getByText(person.email)).toBeVisible();

    await page.goto("/d/settings/workspace");
    await expect(page.getByRole("textbox", { name: "Workspace name" })).toHaveValue(workspaceOf(person));

    await page.goto("/d/settings/members");
    await expect(page.getByText(`${person.name} (you)`)).toBeVisible();
  });

  it("switches the theme from the preferences page", async ({ page }) => {
    await page.goto("/d/settings/preferences");
    const dark = await page.evaluate(() => document.documentElement.classList.contains("dark"));
    const target = dark ? "Light" : "Dark";

    await page.getByRole("button", { name: /Theme$/ }).click();
    await page.getByRole("option", { name: target }).click();

    await expect(page.locator("html")).toHaveClass(dark ? /^(?!.*\bdark\b)/ : /\bdark\b/);
  });

  it("picks the messenger colour with the colour well", async ({ page }) => {
    await openMessenger(page);
    const appearance = page.getByRole("region", { name: "Appearance" });
    await expect(appearance.getByText("#1F2328", { exact: false })).toBeVisible();

    await appearance.getByRole("button", { name: "Messenger colour" }).click();
    const editor = page.getByRole("dialog", { name: "Messenger colour" });
    await editor.getByRole("textbox").fill("#3b82f6");
    await editor.getByRole("textbox").press("Enter");
    await expect(appearance.getByText("#3B82F6", { exact: false })).toBeVisible();
  });

  it("previews messenger changes live and keeps them only once saved", async ({ page }) => {
    await openMessenger(page);
    const messenger = page.frameLocator('iframe[title="Messenger preview"]').locator("#dimmi-messenger");
    await expect(messenger.getByRole("button", { name: "Close messages" }).first()).toBeVisible();
    const greeting = page.getByRole("textbox", { name: "Greeting" });

    await greeting.fill("Hello from the preview");
    await expect(messenger.getByRole("heading", { name: /Hello from the preview/ })).toBeVisible();
    await expect(page.getByRole("status").filter({ hasText: "Unsaved changes" })).toBeVisible();
    await page.getByRole("button", { name: "Discard" }).click();
    await expect(greeting).toHaveValue("");
    await expect(messenger.getByRole("heading", { name: /Hello from the preview/ })).toHaveCount(0);

    await greeting.fill("Saved for real");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("button", { name: "Discard" })).toHaveCount(0);
    await page.reload();
    await expect(page.getByRole("textbox", { name: "Greeting" })).toHaveValue("Saved for real");
  });

  it("narrows the connections by search", async ({ page }) => {
    await page.goto("/d/settings/connections");
    const apps = page.getByRole("region", { name: "Chat apps" });
    await expect(apps.getByRole("button", { name: "Connect" })).toHaveCount(4);

    await page.getByRole("textbox", { name: "Search connections" }).fill("whats");
    await expect(apps.getByText("WhatsApp")).toBeVisible();
    await expect(apps.getByRole("button", { name: "Connect" })).toHaveCount(1);

    await page.getByRole("textbox", { name: "Search connections" }).fill("teams");
    await expect(apps.getByText("No connection matches")).toBeVisible();
  });
});
