/*
 * Emoji and GIFs in the composer: a colon and a few letters suggest emoji,
 * the add menu has an emoji picker, and GIF search goes to GIPHY, which
 * these tests replace with a stub so no real call is ever made. A sent
 * GIF tells GIPHY so.
 */
import type { Page } from "@playwright/test";

import { createAccount, expect, inbox, test as it, type Person, signIn } from "./support/fixtures";
import { type Customer, openComposer, startConversation } from "./support/inbox";

const lena: Customer = {
  name: "Lena Fischer",
  email: "lena@northwind.example",
  subject: "Thanks!",
  body: "It works now, thank you.",
};

const gifURL = "https://media1.giphy.com/media/stub123/giphy.gif";
const pixel = Buffer.from("R0lGODlhAQABAAAAACw=", "base64");

function workspaceOf(person: Person): string {
  return `Emoji ${person.email.split("@")[0]?.split(".").at(-1) ?? ""}`;
}

/*
 * stubGiphy answers GIPHY's API and media from the test, recording each
 * request, and fails the test on any call that slips past the stub.
 */
async function stubGiphy(page: Page): Promise<string[]> {
  const seen: string[] = [];
  await page.route(/giphy\.com/, async (route) => {
    const url = route.request().url();
    seen.push(url);
    if (url.includes("/v1/randomid")) {
      return route.fulfill({ json: { data: { random_id: "stub-random" } } });
    }
    if (url.includes("/v1/gifs/")) {
      return route.fulfill({
        json: {
          data: [
            {
              id: "stub123",
              title: "Happy dance",
              images: { fixed_width_small: { url: gifURL }, downsized_medium: { url: gifURL } },
              analytics: { onsent: { url: "https://pingback.giphy.com/sent?id=stub123" } },
            },
          ],
        },
      });
    }
    return route.fulfill({ contentType: "image/gif", body: pixel });
  });

  return seen;
}

it.describe("emoji and GIFs", () => {
  it.beforeEach(async ({ page, person, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await createAccount(request, person, workspaceOf(person));
    await signIn(page, person);
    await expect(page).toHaveURL(inbox);
  });

  it("suggests emoji while typing and picks them from the add menu", async ({ page }) => {
    const id = await startConversation(page, lena);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await openComposer(page, /^Note/);
    const field = page.getByRole("textbox", { name: "Internal note" });
    await field.pressSequentially("Great :thumbs");
    await expect(page.getByRole("listbox", { name: "Emoji" })).toContainText("thumbs up");
    await page.keyboard.press("Enter");
    await expect(field).toContainText("Great 👍");

    await page.getByRole("button", { name: "Add to message" }).click();
    await page.getByRole("button", { name: /Add an emoji$/ }).click();
    await page.getByRole("textbox", { name: "Search emoji" }).fill("party popper");
    await page.getByRole("button", { name: "party popper" }).click();
    await expect(field).toContainText("🎉");
  });

  it("searches GIFs through GIPHY and sends the one picked", async ({ page, person }) => {
    const seen = await stubGiphy(page);
    const id = await startConversation(page, lena);
    await page.goto(`/d/inbox/unassigned/${id}`);
    await openComposer(page, /^Note/);

    await page.getByRole("button", { name: "Add to message" }).click();
    await page.getByRole("button", { name: /Search GIPHY$/ }).click();
    await expect(page.getByText("Searching GIFs sends your search to GIPHY.")).toBeVisible();
    await expect(page.getByText("Powered by GIPHY")).toBeVisible();
    await page.getByRole("textbox", { name: "Search GIFs" }).fill("dance");
    await page.getByRole("button", { name: "Happy dance" }).click();
    await page.getByRole("button", { name: "Add note" }).click();

    const note = page.getByRole("article", { name: `Internal note from ${person.name}` });
    await expect(note.getByRole("img", { name: "Happy dance" })).toHaveAttribute("src", gifURL);
    await expect.poll(() => seen.some((url) => url.includes("pingback.giphy.com/sent"))).toBe(true);
    expect(seen.find((url) => url.includes("/gifs/search"))).toContain("rating=g");
    expect(seen.find((url) => url.includes("/gifs/search"))).toContain("random_id=stub-random");
  });
});
