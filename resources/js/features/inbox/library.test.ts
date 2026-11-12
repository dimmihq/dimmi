import type { ChangelogEntry } from "@/lib/api/changelog";
import type { FeedbackPost } from "@/lib/api/feedback";
import type { Article } from "@/lib/api/help-centers";
import { articleItems, changelogItems, requestItems } from "./library";

const article = (over: Partial<Article>): Article =>
  ({
    id: "art_1",
    title: "Reset your password",
    slug: "reset-your-password",
    description: "Account",
    status: "published",
    visibility: "public",
    ...over,
  }) as Article;

describe("articleItems", () => {
  it("keeps published public articles and links them on the help center", () => {
    const items = articleItems(
      [article({}), article({ id: "art_2", status: "draft" }), article({ id: "art_3", visibility: "agent_only" })],
      "https://acme.dimmi.help/",
      "",
    );
    expect(items.map((item) => item.id)).toEqual(["art_1"]);
    expect(items[0]?.url).toBe("https://acme.dimmi.help/articles/reset-your-password");
  });

  it("matches the title or the description, ignoring case", () => {
    const items = [article({}), article({ id: "art_2", title: "Billing", description: "Invoices" })];
    expect(articleItems(items, "", "invoice").map((item) => item.id)).toEqual(["art_2"]);
    expect(articleItems(items, "", "")[0]?.url).toBeUndefined();
  });
});

describe("requestItems and changelogItems", () => {
  it("link requests by their path and entries by their slug", () => {
    const post = { id: "fbp_1", title: "Dark mode", vote_count: 1, path: "/feedback/dark-mode" } as FeedbackPost;
    const entry = { id: "cle_1", title: "Codes", summary: null, slug: "codes" } as ChangelogEntry;
    expect(requestItems([post], "https://x.test")[0]).toMatchObject({
      detail: "Feature request · 1 vote",
      url: "https://x.test/feedback/dark-mode",
    });
    expect(changelogItems([entry], "https://x.test")[0]).toMatchObject({
      detail: "Changelog",
      url: "https://x.test/changelog/codes",
    });
  });
});
