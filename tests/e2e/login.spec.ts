import { createAccount, expect, inbox, test as it, signIn, signOut } from "./support/fixtures";

it.describe("signing in with a password", () => {
  it("sends a signed-out visitor of the app to sign in, and back where they were going", async ({
    page,
    person,
    request,
  }) => {
    await createAccount(request, person, `${person.name}'s workspace`);

    await page.goto("/d");
    await expect(page).toHaveURL(/\/login\?redirect=%2Fd$/);

    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill(person.password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();

    await expect(page).toHaveURL(inbox);
  });

  it("refuses a wrong password under the email field", async ({ page, person, request }) => {
    await createAccount(request, person, `${person.name}'s workspace`);

    await page.goto("/login");
    await page.getByLabel("Email").fill(person.email);
    await page.getByLabel("Password").fill("not-the-password-1");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();

    await expect(page.getByText("These credentials do not match our records.")).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });

  it("signs out, and the app asks to sign in again", async ({ page, person, request }) => {
    await createAccount(request, person, `${person.name}'s workspace`);
    await signIn(page, person);

    await signOut(page, person);
    await expect(page).toHaveURL(/\/login$/);

    await page.goto("/d");
    await expect(page).toHaveURL(/\/login/);
  });

  it("sends someone already signed in past the sign-in pages", async ({ page, person, request }) => {
    await createAccount(request, person, `${person.name}'s workspace`);
    await signIn(page, person);

    await page.goto("/login");
    await expect(page).toHaveURL(inbox);
    await page.goto("/signup");
    await expect(page).toHaveURL(inbox);
  });

  it("tells the person a provider sign-in was cancelled", async ({ page }) => {
    await page.goto("/login?error=oauth_denied");

    await expect(page.getByRole("alert")).toHaveText("You cancelled the sign-in at the provider.");
  });
});
