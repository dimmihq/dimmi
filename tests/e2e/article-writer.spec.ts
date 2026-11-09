import { fileURLToPath } from "node:url";

import type { Page } from "@playwright/test";

import { api } from "./support/customers";
import { createAccount, expect, inbox, test as it, signIn } from "./support/fixtures";

/*
 * lineEnd is the key that moves the caret to the end of its line.
 */
const lineEnd = process.platform === "darwin" ? "Meta+ArrowRight" : "End";

const picture = fileURLToPath(new URL("./files/screenshot.png", import.meta.url));

/*
 * openArticle creates an article through the API and opens it in the writer.
 */
async function openArticle(page: Page, body: string): Promise<{ id: string; subdomain: string }> {
  await page.goto("/d/help-center");
  const centers = await api<{ id: string; subdomain: string }[]>(page, "GET", "/help-centers");
  const center = centers[0] ?? { id: "", subdomain: "" };
  const article = await api<{ id: string }>(page, "POST", `/help-centers/${center.id}/articles`, {
    title: "Reset your password",
    body_markdown: body,
  });
  await page.goto(`/d/help-center/${article.id}`);
  await expect(page.getByRole("textbox", { name: "Article" })).toBeVisible();
  await page.evaluate(() => document.fonts.ready);

  return { id: article.id, subdomain: center.subdomain };
}

/*
 * slash types a slash command on a new line at the end and picks the block.
 */
async function slash(page: Page, query: string, option: RegExp) {
  await page.keyboard.type(`/${query}`);
  const blocks = page.getByRole("listbox", { name: "Blocks" });
  await expect(blocks.getByRole("option", { name: option }).first()).toBeVisible();
  await blocks.getByRole("option", { name: option }).first().click();
}

/*
 * endOfBody puts the caret on a fresh line at the end of the article,
 * through the editor the writer's element carries, so the caret is where
 * the writer thinks it is.
 */
async function endOfBody(page: Page) {
  const body = page.getByRole("textbox", { name: "Article" });
  await body.evaluate((element) => {
    const editor = (element as unknown as { editor: { commands: { focus: (at: string) => boolean } } }).editor;
    editor.commands.focus("end");
  });
  await expect(body).toBeFocused();
  await expect(page.getByRole("toolbar", { name: "Formatting" })).toHaveCount(0);
  await page.keyboard.press("Enter");
}

it.describe("article writer", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, `Writer ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`);
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("hints on empty lines and moves from title to body with Enter", async ({ page }) => {
    await openArticle(page, "");
    const body = page.getByRole("textbox", { name: "Article" });
    await expect(body.locator("p.is-empty")).toHaveAttribute("data-placeholder", "Start writing, or type / for blocks");

    const title = page.getByRole("textbox", { name: "Title" });
    await title.click();
    await page.keyboard.press(lineEnd);
    await page.keyboard.press("Enter");
    await expect(page.getByRole("textbox", { name: "Description" })).toBeFocused();
    await expect(title).toHaveValue("Reset your password");
    await page.keyboard.type("Back in two minutes.");
    await page.keyboard.press("Enter");
    await expect(body).toBeFocused();

    await page.keyboard.type("First line");
    await page.keyboard.press("Enter");
    const empty = body.locator("p.is-empty");
    await expect(empty).toHaveCount(1);
    await expect(empty).toHaveAttribute("data-placeholder", "Type / for blocks");
    await expect(body.locator("p", { hasText: "First line" })).not.toHaveClass(/is-empty/);
  });

  it("opens one block menu from a slash or the plus, and closes it with Escape", async ({ page }) => {
    await openArticle(page, "Intro");
    await endOfBody(page);
    await page.getByRole("button", { name: "Add a block" }).click();
    const blocks = page.getByRole("listbox", { name: "Blocks" });
    await expect(blocks.getByRole("option", { name: /Heading/ }).first()).toBeVisible();
    await page.keyboard.type("callout");
    await expect(blocks.getByRole("option")).toHaveCount(5);
    await page.keyboard.press("Escape");
    await expect(blocks).toHaveCount(0);

    await expect(page.getByRole("textbox", { name: "Article" })).toContainText("/callout");
    await page.keyboard.press(process.platform === "darwin" ? "Meta+Shift+ArrowLeft" : "Shift+Home");
    await page.keyboard.press("Backspace");
    await page.keyboard.type("/head");
    await page.keyboard.press("Enter");
    await page.keyboard.type("Before you start");
    const body = page.getByRole("textbox", { name: "Article" });
    await expect(body.getByRole("heading", { level: 2, name: "Before you start" })).toBeVisible();
    await expect(body).not.toContainText("/");
  });

  it("writes callouts, tables, links and pictures that survive a save and read the same published", async ({
    page,
  }) => {
    const { subdomain } = await openArticle(page, "Intro");
    const body = page.getByRole("textbox", { name: "Article" });

    await endOfBody(page);
    await slash(page, "warn", /Warning/);
    await page.keyboard.type("Back up first.");
    await page.keyboard.press("Enter");
    await page.keyboard.press("Enter");
    await expect(body.locator("aside[data-callout] p")).toHaveCount(1);
    const callout = body.locator("aside[data-callout]");
    await expect(callout).toHaveClass(/alert-warning/);
    await expect(body).not.toContainText("[!WARNING]");
    await callout.getByRole("button", { name: /Callout kind/ }).click();
    await page.getByRole("button", { name: "Tip", exact: true }).click();
    await expect(callout).toHaveClass(/alert-tip/);

    await body.locator(":scope > p").last().click();
    await slash(page, "table", /Table/);
    await expect(body.locator("table")).toBeVisible();

    await body.locator("p", { hasText: "Intro" }).selectText();
    await expect(page.getByRole("toolbar", { name: "Formatting" })).toBeVisible();
    await page.getByRole("toolbar", { name: "Formatting" }).getByRole("button", { name: "Bold" }).click();
    await expect(body.locator("strong", { hasText: "Intro" })).toBeVisible();
    await page.getByRole("toolbar", { name: "Formatting" }).getByRole("button", { name: "Link" }).click();
    await page.getByRole("textbox", { name: "Link address" }).fill("https://example.com/help");
    await page.keyboard.press("Enter");
    await expect(body.locator('a[href="https://example.com/help"]')).toBeVisible();

    await endOfBody(page);
    const chooser = page.waitForEvent("filechooser");
    await slash(page, "image", /Image/);
    await (await chooser).setFiles(picture);
    const image = body.locator('img[src^="/help-images/"]');
    await expect(image).toBeVisible();
    await expect(body.locator("img[data-uploading]")).toHaveCount(0);
    await expect(body.locator("p", { hasText: "Intro" })).toBeVisible();
    await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(64);
    await image.click();
    await page.getByRole("textbox", { name: "Alt text" }).fill("The reset screen");
    await expect(image).toHaveAttribute("alt", "The reset screen");

    const imageBar = page.getByRole("toolbar", { name: "Image" });
    await imageBar.getByRole("button", { name: "Align center" }).click();
    await expect(imageBar.getByRole("button", { name: "Align center" })).toHaveAttribute("aria-pressed", "true");
    const frame = body.locator(".article-image");
    await expect(frame).toHaveClass(/image-align-center/);
    await image.evaluate((img: HTMLImageElement) => {
      img.style.minWidth = "600px";
    });
    const handle = frame.locator('[data-image-handle="right"]');
    const grip = await handle.boundingBox();
    await page.mouse.move((grip?.x ?? 0) + 2, (grip?.y ?? 0) + 10);
    await page.mouse.down();
    await page.mouse.move((grip?.x ?? 0) - 100, (grip?.y ?? 0) + 10, { steps: 5 });
    await page.mouse.up();
    await expect(frame).toHaveClass(/image-width-\d+/);
    const sized = (await frame.getAttribute("class"))?.match(/image-width-(\d+)/)?.[1] ?? "";
    await image.evaluate((img: HTMLImageElement) => {
      img.style.minWidth = "";
    });
    const share = await frame.evaluate(
      (el) => el.getBoundingClientRect().width / (el.closest(".ProseMirror")?.clientWidth ?? 1),
    );
    expect(Math.abs(share * 100 - Number(sized))).toBeLessThan(2);

    const before = await image.getAttribute("src");
    const swap = page.waitForEvent("filechooser");
    await imageBar.getByRole("button", { name: "Replace image" }).click();
    await (await swap).setFiles(picture);
    await expect(body.locator("img[data-uploading]")).toHaveCount(0);
    await expect(body.locator('img[src^="/help-images/"]')).not.toHaveAttribute("src", before ?? "");
    await expect(frame).toHaveClass(new RegExp(`image-width-${sized}`));
    await expect(body.locator('img[alt="The reset screen"]')).toBeVisible();

    await page.getByRole("button", { name: "Save draft" }).click();
    await expect(page.getByText("Draft saved")).toBeVisible();
    await page.reload();
    await expect(body.locator("aside[data-callout].alert-tip")).toContainText("Back up first.");
    await expect(body.locator("table")).toBeVisible();
    await expect(body.locator('img[alt="The reset screen"]')).toBeVisible();

    await page.getByRole("button", { name: "Publish" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Publish" }).click();
    await expect(page.getByText("Published", { exact: true })).toBeVisible();

    await page.goto(`/hc/${subdomain}/articles/reset-your-password`);
    await expect(page.locator("aside.alert-tip")).toContainText("Back up first.");
    await expect(page.locator(".prose table")).toBeVisible();
    const published = page.locator('.prose img[alt="The reset screen"]');
    await expect(published).toHaveClass(/image-align-center/);
    await expect(published).toHaveClass(/image-width-\d+/);
    await expect.poll(() => published.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(64);
    await expect(page.locator('.prose a[href="https://example.com/help"]')).toBeVisible();
  });

  it("restores a kept version from the history", async ({ page }) => {
    const { id } = await openArticle(page, "First words.");
    await api(page, "POST", `/articles/${id}/publications`, {});
    await api(page, "PATCH", `/articles/${id}`, { body_markdown: "Second words." });
    await page.reload();
    const body = page.getByRole("textbox", { name: "Article" });
    await expect(body).toContainText("Second words.");

    await page.getByRole("button", { name: "Version history" }).click();
    const versions = page.getByRole("list", { name: "Versions" });
    await versions.getByText("Version 1").click({ button: "right" });
    await page.getByRole("menuitem", { name: "Restore this version" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Restore" }).click();
    await expect(body).toContainText("First words.");
    await expect(body).not.toContainText("Second words.");
  });
});
