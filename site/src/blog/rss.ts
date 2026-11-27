/*
 * Pure RSS 2.0 XML builder for /blog/rss.xml.
 * Accepts only published posts and formats RFC-822 pubDate values.
 */
import { getAuthor } from "../authors";
import type { BlogPostSummary } from "./posts";

const SITE_URL = "https://trydimmi.com";

function escapeXml(text: string): string {
  return text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}

function toRfc822(isoDate: string): string {
  const parsed = new Date(`${isoDate}T00:00:00Z`);
  return Number.isNaN(parsed.getTime()) ? new Date(0).toUTCString() : parsed.toUTCString();
}

export function renderBlogRss(posts: readonly BlogPostSummary[]): string {
  const published = posts.filter((post) => post.meta.status === "published");
  const lastBuild = published[0]?.meta.updated ?? "2026-11-27";

  const items = published
    .map((post) => {
      const url = `${SITE_URL}${post.path}`;
      const author = getAuthor(post.meta.author);
      return [
        "    <item>",
        `      <title>${escapeXml(post.title)}</title>`,
        `      <link>${escapeXml(url)}</link>`,
        `      <guid isPermaLink="true">${escapeXml(url)}</guid>`,
        `      <pubDate>${toRfc822(post.meta.published)}</pubDate>`,
        `      <description>${escapeXml(post.description)}</description>`,
        `      <author>${escapeXml(author.name)}</author>`,
        "    </item>",
      ].join("\n");
    })
    .join("\n");

  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">',
    "  <channel>",
    "    <title>dimmi blog</title>",
    `    <link>${SITE_URL}/blog</link>`,
    "    <description>Product notes, open-source architecture, and honest customer support pricing breakdowns from the dimmi team.</description>",
    "    <language>en-us</language>",
    `    <lastBuildDate>${toRfc822(lastBuild)}</lastBuildDate>`,
    `    <atom:link href="${SITE_URL}/blog/rss.xml" rel="self" type="application/rss+xml" />`,
    items,
    "  </channel>",
    "</rss>",
    "",
  ]
    .filter((line) => line !== "")
    .join("\n")
    .concat("\n");
}
