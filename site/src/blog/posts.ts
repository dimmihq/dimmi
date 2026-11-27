/*
 * Loads MDX blog posts from site/content/blog/*.mdx via Vite eager glob.
 * Draft posts (status !== "published") are stripped before routes, index,
 * sitemap, and RSS ever see them.
 */
import type { ComponentType } from "react";

import type { PageMeta, PageRival } from "../routes";

export type BlogMdxComponents = Record<string, ComponentType<Record<string, unknown>>>;

export type BlogPostSummary = {
  slug: string;
  path: string;
  title: string;
  heading: string;
  description: string;
  meta: PageMeta;
};

export type BlogPost = BlogPostSummary & {
  Component: ComponentType<{ components?: BlogMdxComponents }>;
};

export type RawMdxEntry = {
  filePath: string;
  frontmatter: Record<string, unknown>;
  Component: ComponentType<{ components?: BlogMdxComponents }>;
};

function asString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value.trim() : fallback;
}

function asStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const items = value.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
  return items.length > 0 ? items : undefined;
}

function asRivals(value: unknown): PageRival[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const rivals: PageRival[] = [];
  for (const item of value) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const obj = item as Record<string, unknown>;
    const name = asString(obj.name);
    const checkedOn = asString(obj.checkedOn);
    const source = asString(obj.source);
    if (name && checkedOn && source) {
      rivals.push({ name, checkedOn, source });
    }
  }
  return rivals.length > 0 ? rivals : undefined;
}

export function slugFromFilePath(filePath: string): string {
  const base = filePath.split("/").pop() ?? filePath;
  return base.replace(/\.mdx$/i, "");
}

export function parseBlogEntry(entry: RawMdxEntry): BlogPost {
  const fm = entry.frontmatter;
  const slug = slugFromFilePath(entry.filePath);
  const title = asString(fm.title, slug);
  const heading = asString(fm.heading, title);
  const description = asString(fm.description);
  const published = asString(fm.published);
  const updated = asString(fm.updated, published);
  const status = asString(fm.status) === "published" ? "published" : "draft";
  const proof = asStringArray(fm.proof);
  const rivals = asRivals(fm.rivals);

  const meta: PageMeta = {
    type: "blog",
    keyword: asString(fm.keyword),
    answer: asString(fm.answer),
    author: asString(fm.author, "frantz-kati"),
    published,
    updated,
    status,
    reviewedBy: asString(fm.reviewedBy),
    reviewedOn: asString(fm.reviewedOn, updated),
    ...(proof ? { proof } : {}),
    ...(rivals ? { rivals } : {}),
  };

  return {
    slug,
    path: `/blog/${slug}`,
    title,
    heading,
    description,
    meta,
    Component: entry.Component,
  };
}

export function selectPublishedPosts(entries: readonly RawMdxEntry[]): BlogPost[] {
  return entries
    .map(parseBlogEntry)
    .filter((post) => post.meta.status === "published")
    .sort((a, b) => {
      const dateCmp = b.meta.published.localeCompare(a.meta.published);
      return dateCmp !== 0 ? dateCmp : a.slug.localeCompare(b.slug);
    });
}

type MdxModule = {
  frontmatter?: Record<string, unknown>;
  default: ComponentType<{ components?: BlogMdxComponents }>;
};

const rawModules = import.meta.glob<MdxModule>("../../content/blog/*.mdx", { eager: true });

export const publishedBlogPosts: BlogPost[] = selectPublishedPosts(
  Object.entries(rawModules).map(([filePath, mod]) => ({
    filePath,
    frontmatter: mod.frontmatter ?? {},
    Component: mod.default,
  })),
);

export function findBlogPost(slug: string): BlogPost | undefined {
  return publishedBlogPosts.find((post) => post.slug === slug);
}
