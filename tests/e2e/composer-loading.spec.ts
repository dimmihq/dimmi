import { createAccount, expect, test as it, signIn } from "./support/fixtures";

/*
 * The formatting editor loads after the page, and a plain field stands in
 * until it arrives. Text typed while it arrives must never be lost: the
 * editor waits for a pause in the typing and takes the focus over.
 */
it("keeps what is typed while the formatting editor arrives", async ({ page, person, request }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await createAccount(request, person, `Composer ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
  await signIn(page, person);
  let release: () => void = () => undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(/rich-editor-[^/]*\.js$/, async (route) => {
    await held;
    await route.continue();
  });

  await page.goto("/d/workflows");
  await page.getByRole("button", { name: "New workflow" }).click();
  const picker = page.getByRole("dialog", { name: "What starts this workflow?" });
  await picker.getByRole("textbox", { name: "Search triggers" }).fill("first message");
  await picker.getByRole("button", { name: /Customer sends their first message/ }).click();
  const canvas = page.getByRole("region", { name: "Workflow" });
  await canvas.getByRole("button", { name: "Add a step" }).click();
  await page.getByRole("dialog", { name: "Add a step" }).getByRole("button", { name: "Reply buttons" }).click();
  const inspector = page.getByRole("region", { name: "Step settings" });
  const question = inspector.getByRole("textbox", { name: "Question" });

  await question.focus();
  await page.keyboard.type("What is ");
  release();
  await expect
    .poll(() =>
      page.evaluate(() => performance.getEntriesByType("resource").some((r) => r.name.includes("rich-editor"))),
    )
    .toBe(true);
  await page.keyboard.type("it about?");
  const rich = inspector.locator("[contenteditable][aria-label='Question']");
  await expect(rich).toBeFocused();
  await page.keyboard.type(" Pick one.");
  await expect(rich).toContainText("What is it about? Pick one.");

  await inspector.getByRole("textbox", { name: "Button 1" }).fill("Billing");
  await expect(canvas.getByRole("button", { name: /What is it about\? Pick one\./ })).toBeVisible();
});
