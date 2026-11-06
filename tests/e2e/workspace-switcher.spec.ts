import { createAccount, expect, finishOnboarding, inbox, test as it, type Person, signIn } from "./support/fixtures";

/*
 * suffixOf is a short part of the person's address that keeps their
 * workspaces' names apart from other tests' running alongside.
 */
function suffixOf(person: Person): string {
  return person.email.split("@")[0]?.slice(-6) ?? "";
}

it.describe("the workspace menu", () => {
  it("starts a new workspace through setup and switches between workspaces", async ({ page, person, request }) => {
    const first = `First ${suffixOf(person)}`;
    const second = `Second ${suffixOf(person)}`;
    await createAccount(request, person, first);
    await signIn(page, person);

    await page.getByRole("button", { name: first, exact: true }).click();
    const menu = page.getByRole("dialog", { name: "Workspaces" });
    await expect(menu.getByText("Owner")).toBeVisible();
    await menu.getByRole("button", { name: "New workspace" }).click();

    await finishOnboarding(page, second);
    await expect(page.getByRole("button", { name: second, exact: true })).toBeVisible();

    await page.getByRole("button", { name: second, exact: true }).click();
    await menu.getByRole("button", { name: new RegExp(first) }).click();
    await expect(page).toHaveURL(inbox);
    await expect(page.getByRole("button", { name: first, exact: true })).toBeVisible();
  });

  it("goes back to the app when a new workspace is cancelled", async ({ page, person, request }) => {
    const first = `First ${suffixOf(person)}`;
    await createAccount(request, person, first);
    await signIn(page, person);

    await page.getByRole("button", { name: first, exact: true }).click();
    await page.getByRole("dialog", { name: "Workspaces" }).getByRole("button", { name: "New workspace" }).click();
    await expect(page.getByRole("heading", { name: "Name your workspace" })).toBeVisible();
    await page.getByRole("button", { name: "Cancel" }).click();

    await expect(page).toHaveURL(inbox);
    await expect(page.getByRole("button", { name: first, exact: true })).toBeVisible();
  });
});
