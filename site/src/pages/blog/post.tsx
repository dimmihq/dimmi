/*
 * Individual blog post page (/blog/<slug>).
 * Renders breadcrumb, H1, visible author + published/updated dates, the
 * direct answer block in the first 50 words, MDX body, and Start free + GitHub
 * star CTA buttons.
 */
import { RiArrowLeftLine, RiArrowRightLine, RiGithubFill, RiStarLine } from "@remixicon/react";

import { getAuthor } from "../../authors";
import {
  MdxA,
  MdxBlockquote,
  MdxCode,
  MdxH2,
  MdxH3,
  MdxLi,
  MdxOl,
  MdxP,
  MdxPre,
  MdxProductShot,
  MdxStrong,
  MdxUl,
} from "../../blog/mdx-components";
import { type BlogMdxComponents, findBlogPost } from "../../blog/posts";
import { MonoLink, Section } from "../../components/container";
import { APP_URL, GITHUB_URL } from "../../links";

const BLOG_MDX_COMPONENTS = {
  h2: MdxH2,
  h3: MdxH3,
  p: MdxP,
  ul: MdxUl,
  ol: MdxOl,
  li: MdxLi,
  a: MdxA,
  strong: MdxStrong,
  code: MdxCode,
  pre: MdxPre,
  blockquote: MdxBlockquote,
  ProductShot: MdxProductShot,
} as unknown as BlogMdxComponents;

function formatDate(iso: string): string {
  const parsed = new Date(`${iso}T00:00:00Z`);
  if (Number.isNaN(parsed.getTime())) {
    return iso;
  }
  return parsed.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function BlogPostPage({ path }: { path?: string }) {
  const slug = (path ?? "").replace(/^\/blog\//, "").replace(/\/+$/, "");
  const post = findBlogPost(slug);
  if (!post) {
    return null;
  }

  const author = getAuthor(post.meta.author);
  const Content = post.Component;
  const mdxComponents = BLOG_MDX_COMPONENTS;

  return (
    <article className="flex flex-col">
      <Section frameClassName="flex flex-col gap-6 pt-12 pb-10 sm:pt-18 sm:pb-12">
        <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-[13px] text-text-tertiary">
          <a href="/" className="transition-colors hover:text-text-primary">
            Home
          </a>
          <span aria-hidden>/</span>
          <a href="/blog" className="transition-colors hover:text-text-primary">
            Blog
          </a>
          <span aria-hidden>/</span>
          <span className="text-text-secondary">{post.heading}</span>
        </nav>

        <h1 className="max-w-3xl text-[32px] leading-[1.1] font-medium tracking-[-0.03em] text-text-primary sm:text-[46px]">
          {post.heading}
        </h1>

        <p className="max-w-3xl rounded-xl border border-accent-500/30 bg-accent-500/[0.04] p-5 text-[16px] leading-[1.65] text-text-primary">
          {post.meta.answer}
        </p>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-separator-border pt-4 text-[13px] text-text-secondary">
          <span className="font-medium text-text-primary">
            By {author.name} ({author.role})
          </span>
          <span aria-hidden className="text-text-tertiary">
            ·
          </span>
          <span>
            Published <time dateTime={post.meta.published}>{formatDate(post.meta.published)}</time>
          </span>
          <span aria-hidden className="text-text-tertiary">
            ·
          </span>
          <span>
            Updated <time dateTime={post.meta.updated}>{formatDate(post.meta.updated)}</time>
          </span>
        </div>
      </Section>

      <Section frameClassName="py-10">
        <div className="max-w-3xl">
          <Content components={mdxComponents} />
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-6 py-12">
        <div className="flex flex-col gap-5 rounded-2xl border border-border-button-default bg-background-secondary-default/40 p-6 sm:flex-row sm:items-center sm:justify-between sm:p-8">
          <div className="flex max-w-xl flex-col gap-2">
            <h2 className="text-[20px] font-medium tracking-[-0.02em] text-text-primary">
              Try dimmi free or inspect the MIT source code.
            </h2>
            <p className="text-[14px] leading-6 text-text-secondary">
              Start on dimmi Cloud with 100 free monthly conversations and unlimited free seats, or deploy the open Go
              binary on your own Linux server.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <a
              href={`${APP_URL}/signup`}
              className="inline-flex h-10 items-center gap-1.5 rounded-lg bg-accent-600 px-4 text-[14px] font-medium text-white transition-colors hover:bg-accent-700"
            >
              Start free
              <RiArrowRightLine className="size-4" aria-hidden />
            </a>
            <a
              href={GITHUB_URL}
              className="inline-flex h-10 items-center gap-1.5 rounded-lg border border-border-button-default bg-background-full px-4 text-[14px] font-medium text-text-primary transition-colors hover:border-text-primary/30"
            >
              <RiGithubFill className="size-4" aria-hidden />
              <RiStarLine className="size-3.5 text-text-tertiary" aria-hidden />
              Star on GitHub
            </a>
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-4 pt-2">
          <a
            href="/blog"
            className="inline-flex items-center gap-1.5 text-[14px] font-medium text-text-secondary transition-colors hover:text-text-primary"
          >
            <RiArrowLeftLine className="size-4" aria-hidden />
            Back to all posts
          </a>
          <div className="flex flex-wrap gap-6">
            <MonoLink href="/pricing">Pricing breakdown</MonoLink>
            <MonoLink href="/agents">AI support agents</MonoLink>
            <MonoLink href="/docs">Developer docs</MonoLink>
          </div>
        </div>
      </Section>
    </article>
  );
}
