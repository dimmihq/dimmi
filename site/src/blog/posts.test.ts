import { describe, expect, it } from "vitest";

import { findRoute } from "../routes";
import { pageSchemas } from "../seo";
import { publishedBlogPosts, selectPublishedPosts } from "./posts";
import { renderBlogRss } from "./rss";

function DummyPost() {
  return null;
}

describe("blog pipeline", () => {
  it("excludes draft posts and sorts published posts newest first", () => {
    const selected = selectPublishedPosts([
      {
        filePath: "../../content/blog/draft-post.mdx",
        frontmatter: {
          title: "Unfinished draft",
          description: "Should never ship.",
          published: "2026-12-01",
          updated: "2026-12-01",
          author: "frantz-kati",
          reviewedBy: "frantz-kati",
          keyword: "draft test",
          answer: "Draft answer.",
          status: "draft",
        },
        Component: DummyPost,
      },
      {
        filePath: "../../content/blog/older-post.mdx",
        frontmatter: {
          title: "Older post",
          description: "Published earlier.",
          published: "2026-11-10",
          updated: "2026-11-15",
          author: "frantz-kati",
          reviewedBy: "frantz-kati",
          keyword: "older test",
          answer: "Older answer.",
          status: "published",
        },
        Component: DummyPost,
      },
      {
        filePath: "../../content/blog/newer-post.mdx",
        frontmatter: {
          title: "Newer post",
          description: "Published later.",
          published: "2026-11-27",
          updated: "2026-11-27",
          author: "frantz-kati",
          reviewedBy: "frantz-kati",
          keyword: "newer test",
          answer: "Newer answer.",
          status: "published",
        },
        Component: DummyPost,
      },
    ]);

    expect(selected.map((p) => p.slug)).toEqual(["newer-post", "older-post"]);
    expect(selected.some((p) => p.slug === "draft-post")).toBe(false);
  });

  it("renders valid RSS 2.0 XML containing only published posts", () => {
    const rss = renderBlogRss(publishedBlogPosts);
    expect(rss).toContain('<rss version="2.0"');
    expect(rss).toContain("https://trydimmi.com/blog/rss.xml");
    expect(rss).toContain("https://trydimmi.com/blog/why-we-built-dimmi");
    expect(rss).toContain("Why we built dimmi: an open-source Intercom alternative");
    expect(rss).toContain("<pubDate>");
  });

  it("emits BlogPosting with Person author, matching dates, and BreadcrumbList", () => {
    const route = findRoute("/blog/why-we-built-dimmi");
    expect(route).toBeDefined();
    const schemas = pageSchemas(route);
    const types = schemas.map((s) => s["@type"]);
    expect(types).toContain("BlogPosting");
    expect(types).toContain("BreadcrumbList");
    expect(types).not.toContain("FAQPage");

    const posting = schemas.find((s) => s["@type"] === "BlogPosting") as Record<string, unknown>;
    expect(posting.datePublished).toBe(route?.meta?.published);
    expect(posting.dateModified).toBe(route?.meta?.updated);
    expect((posting.author as Record<string, unknown>)["@type"]).toBe("Person");
    expect((posting.author as Record<string, unknown>).name).toBe("Frantz Kati");
  });
});
