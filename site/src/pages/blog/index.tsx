/*
 * Blog index page (/blog): newest-first list of published MDX posts with
 * dates, author names, and an RSS feed link. Draft posts are excluded at
 * load time in site/src/blog/posts.ts.
 */
import { RiArrowRightLine, RiRssLine } from "@remixicon/react";

import { getAuthor } from "../../authors";
import { publishedBlogPosts } from "../../blog/posts";
import { MonoLink, Section } from "../../components/container";

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

export function BlogIndexPage() {
  return (
    <div className="flex flex-col">
      <Section frameClassName="flex flex-col gap-6 pt-16 pb-12 sm:pt-24 sm:pb-16">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <span className="inline-flex w-fit items-center rounded-md border border-border-button-default bg-background-full px-2.5 py-1 font-mono text-[11px] font-medium tracking-[0.12em] text-text-secondary uppercase">
            dimmi blog
          </span>
          <a
            href="/blog/rss.xml"
            className="inline-flex items-center gap-1.5 rounded-lg border border-border-button-default bg-background-full px-3 py-1.5 font-mono text-[12px] text-text-secondary transition-colors hover:border-text-primary/30 hover:text-text-primary"
          >
            <RiRssLine className="size-3.5" aria-hidden />
            RSS feed
          </a>
        </div>

        <h1 className="max-w-3xl text-[34px] leading-[1.08] font-medium tracking-[-0.03em] text-text-primary sm:text-[48px]">
          Engineering notes, pricing audits, and open support guides.
        </h1>

        <p className="max-w-2xl text-[17px] leading-[1.6] text-text-secondary">
          First-hand teardowns, real architecture numbers, and setup guides from the team building dimmi. Product
          release notes live on our changelog; this feed stays focused on deep technical and pricing posts.
        </p>
      </Section>

      <Section frameClassName="flex flex-col gap-6 py-12">
        <div className="grid grid-cols-1 gap-6">
          {publishedBlogPosts.map((post) => {
            const author = getAuthor(post.meta.author);
            return (
              <article
                key={post.slug}
                className="group flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 transition-colors hover:border-accent-500/50 sm:p-8"
              >
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-[12px] text-text-tertiary">
                  <time dateTime={post.meta.published}>{formatDate(post.meta.published)}</time>
                  <span aria-hidden>·</span>
                  <span>{author.name}</span>
                  {post.meta.updated !== post.meta.published ? (
                    <>
                      <span aria-hidden>·</span>
                      <span>
                        Updated <time dateTime={post.meta.updated}>{formatDate(post.meta.updated)}</time>
                      </span>
                    </>
                  ) : null}
                </div>

                <h2 className="text-[22px] leading-[1.25] font-medium tracking-[-0.02em] text-text-primary sm:text-[26px]">
                  <a
                    href={post.path}
                    className="transition-colors group-hover:text-accent-600 dark:group-hover:text-accent-400"
                  >
                    {post.heading}
                  </a>
                </h2>

                <p className="max-w-3xl text-[15px] leading-[1.65] text-text-secondary">{post.description}</p>

                <div className="pt-1">
                  <a
                    href={post.path}
                    className="inline-flex items-center gap-1.5 text-[14px] font-medium text-accent-700 transition-colors hover:text-accent-600 dark:text-accent-400"
                  >
                    Read article
                    <RiArrowRightLine className="size-4" aria-hidden />
                  </a>
                </div>
              </article>
            );
          })}
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-5 py-12">
        <h2 className="text-[18px] font-medium text-text-primary">Explore dimmi</h2>
        <div className="flex flex-wrap gap-6">
          <MonoLink href="/pricing">See per-conversation pricing</MonoLink>
          <MonoLink href="/inbox">Tour the shared inbox</MonoLink>
          <MonoLink href="/agents">Explore AI support agents</MonoLink>
          <MonoLink href="/docs">Read developer docs</MonoLink>
        </div>
      </Section>
    </div>
  );
}
