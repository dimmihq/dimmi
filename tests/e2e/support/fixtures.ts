/*
 * What every end-to-end test gets: a person of its own, so tests never
 * share an account, and a watch on the page that fails the test on any
 * script error, console error or blocked content, such as a content
 * security policy violation. Browsers also log every API call the server
 * refuses; those are the answers tests provoke on purpose, such as a
 * wrong password, so only refusals from the API are let through. The
 * trace recorder also tries to run its snapshot script in every frame,
 * and a frame sandboxed to run no script, such as the one showing an
 * email, refuses it; that refusal is the sandbox working, so it is let
 * through too.
 */
import { type APIRequestContext, test as base, expect, type Page } from "@playwright/test";

import { finishSetup } from "./database";

export interface Person {
  name: string;
  email: string;
  password: string;
}

const refusedByApi = /Failed to load resource: the server responded with a status of 4\d\d/;
const refusedBySandbox =
  /^Blocked script execution in 'about:srcdoc' because the document's frame is sandboxed and the 'allow-scripts' permission is not set\.$/;

/*
 * listen collects every script error, console error and blocked content
 * the page reports, leaving out the API's refusals.
 */
function listen(page: Page): string[] {
  const problems: string[] = [];
  page.on("pageerror", (error) => problems.push(`page error: ${error.message}`));
  page.on("console", (message) => {
    const apiRefusal = refusedByApi.test(message.text()) && message.location().url.includes("/api/v1/");
    if (message.type() === "error" && !apiRefusal && !refusedBySandbox.test(message.text())) {
      problems.push(`console error: ${message.text()}`);
    }
  });

  return problems;
}

export const test = base.extend<{ person: Person; watch: undefined; guest: Page }>({
  person: async ({ browserName }, use, info) => {
    const id = `${browserName}-${info.workerIndex}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;

    await use({ name: "Grace Hopper", email: `grace.${id}@example.com`, password: "analytical-engine-1843" });
  },

  watch: [
    async ({ page }, use) => {
      const problems = listen(page);

      await use(undefined);

      expect(problems, "the page logged errors").toEqual([]);
    },
    { auto: true },
  ],

  /*
   * guest is a page in a browser of its own, sharing no cookies with
   * page, for a second person such as an invitee. It is watched the same
   * way.
   */
  guest: async ({ browser }, use) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    const problems = listen(page);

    await use(page);

    await context.close();
    expect(problems, "the guest page logged errors").toEqual([]);
  },
});

export { expect };

/*
 * inbox matches the address the app opens on: the workspace's
 * unassigned conversations.
 */
export const inbox = /\/d\/[^/]+\/inbox\/unassigned$/;

/*
 * createAccount signs the person up through the API, as a quick way to
 * have an account; it leaves the browser signed out. With a workspace
 * name the account owns that workspace, set up and ready to open; without
 * one it has none, as a teammate added to someone else's workspace. A
 * request outside a browser sends no Origin, which cross-origin
 * protection lets through.
 */
export async function createAccount(request: APIRequestContext, person: Person, workspace?: string) {
  const response = await request.post("/api/v1/users", {
    data: { name: person.name, email: person.email, password: person.password, workspace_name: workspace },
  });
  expect(response.status(), await response.text()).toBe(201);
  if (workspace) {
    finishSetup(person.email);
  }
}

/*
 * finishOnboarding names the first workspace of a person who just signed
 * up and skips every other setup step, which lands them in the inbox.
 */
export async function finishOnboarding(page: Page, workspace: string) {
  await expect(page).toHaveURL(/\/onboarding(\?new=true)?$/);
  await page.getByLabel("Workspace name").fill(workspace);
  await page.getByRole("button", { name: "Save and continue" }).click();
  for (const skip of [
    "Continue without a logo",
    "Continue",
    "Continue without tags",
    "Continue without inviting",
    "Continue without knowledge",
    "Continue without example conversations",
  ]) {
    await page.getByRole("button", { name: skip, exact: true }).click();
  }
  await page.getByRole("button", { name: "Go to your workspace" }).click();
  await expect(page).toHaveURL(inbox);
}

/*
 * signIn signs the person in through the sign-in page.
 */
export async function signIn(page: Page, person: Person) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(person.email);
  await page.getByLabel("Password").fill(person.password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(inbox);
}

/*
 * signOut signs out from the account menu at the foot of the sidebar.
 */
export async function signOut(page: Page, person: Person) {
  await page.getByRole("button", { name: person.name, exact: true }).click();
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login$/);
}
