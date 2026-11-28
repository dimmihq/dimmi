import { RiArrowLeftLine, RiArrowRightLine, RiEditLine, RiGithubFill } from "@remixicon/react";
import { type ReactNode, useId } from "react";

import { ButtonLink } from "@/components/base/buttons/button";
import { Divider } from "@/components/base/divider/divider";
import { DimmiLogo } from "@/features/brand/logo";
import { cx } from "@/utils/cx";

import { GITHUB_URL, SITE_URL } from "../../nav";

/* The pieces around a page's content: title, hero, TOC, footer. */

export type TocItem = { depth: number; slug: string; text: string; children: TocItem[] };

export function PageTitle({ title, description, eyebrow }: { title: string; description?: string; eyebrow?: string }) {
  return (
    <header className="flex flex-col gap-3 pb-8">
      {eyebrow ? <p className="text-body-medium text-accent-600 dark:text-accent-400">{eyebrow}</p> : null}
      {/* biome-ignore lint/correctness/useUniqueElementIds: Starlight links "skip to content" and the TOC's top entry to #_top */}
      <h1 id="_top" className="text-[2rem] leading-10 font-semibold tracking-[-0.025em] text-balance text-text-primary">
        {title}
      </h1>
      {description ? <p className="max-w-2xl text-[1.0625rem] leading-7 text-text-secondary">{description}</p> : null}
    </header>
  );
}

type HeroAction = { text: string; link: string; variant?: string; attrs?: Record<string, unknown> };

export function Hero({ title, tagline, actions }: { title: string; tagline?: string; actions: HeroAction[] }) {
  return (
    <header className="site-panel relative mb-10 overflow-hidden rounded-3xl border border-separator-border px-6 py-12 sm:px-10 sm:py-16">
      <div className="relative flex max-w-2xl flex-col gap-5">
        {/* biome-ignore lint/correctness/useUniqueElementIds: Starlight links "skip to content" to #_top */}
        <h1
          id="_top"
          className="text-[2.5rem] leading-[1.1] font-semibold tracking-[-0.03em] text-balance text-text-primary sm:text-5xl"
        >
          {title}
        </h1>
        {tagline ? <p className="text-lg leading-8 text-text-secondary">{tagline}</p> : null}
        {actions.length > 0 && (
          <div className="flex flex-wrap gap-2 pt-2">
            {actions.map((action) => (
              <ButtonLink
                key={action.link}
                href={action.link}
                variant={action.variant === "primary" ? "primary" : "secondary"}
                trailingIcon={action.variant === "primary" ? RiArrowRightLine : undefined}
              >
                {action.text}
              </ButtonLink>
            ))}
          </div>
        )}
      </div>
    </header>
  );
}

// The mobile dropdown names itself in its summary, so it hides the heading.
export function TableOfContents({ items, showTitle = true }: { items: TocItem[]; showTitle?: boolean }) {
  // The page renders this twice (desktop column and mobile dropdown).
  const titleId = useId();
  return (
    <nav
      aria-labelledby={showTitle ? titleId : undefined}
      aria-label={showTitle ? undefined : "On this page"}
      className="flex flex-col gap-3"
    >
      {showTitle ? (
        <h2 id={titleId} className="text-caption-1-medium text-text-tertiary">
          On this page
        </h2>
      ) : null}
      <TocList items={items} />
    </nav>
  );
}

function TocList({ items, nested = false }: { items: TocItem[]; nested?: boolean }) {
  return (
    <ul className={cx("flex flex-col", !nested && "border-s border-separator-border")}>
      {items.map((item) => (
        <li key={item.slug}>
          <a
            href={`#${item.slug}`}
            data-toc-link={item.slug}
            className={cx(
              "-ms-px block border-s border-transparent py-1 text-caption-1-regular text-text-secondary transition-colors hover:text-text-primary",
              nested ? "ps-6" : "ps-3",
            )}
          >
            {item.text}
          </a>
          {item.children.length > 0 && <TocList items={item.children} nested />}
        </li>
      ))}
    </ul>
  );
}

type PageLink = { href: string; label: string } | undefined;

export function EditLink({ href }: { href: string }) {
  return (
    <ButtonLink href={href} size="small" variant="secondary" leadingIcon={RiEditLine}>
      Edit this page
    </ButtonLink>
  );
}

export function LastUpdated({ date, iso }: { date: string; iso: string }) {
  return (
    <p className="text-caption-1-regular text-text-tertiary">
      Last updated <time dateTime={iso}>{date}</time>
    </p>
  );
}

export function Pagination({ prev, next }: { prev: PageLink; next: PageLink }) {
  if (!prev && !next) {
    return null;
  }
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {prev ? <PageCard link={prev} direction="prev" /> : <span className="hidden sm:block" />}
      {next ? <PageCard link={next} direction="next" /> : null}
    </div>
  );
}

export function FooterBar() {
  return (
    <>
      <Divider />
      <div className="flex flex-wrap items-center justify-between gap-4 pb-10 text-caption-1-regular text-text-tertiary">
        <a href={SITE_URL} className="text-text-primary" aria-label="dimmi">
          <DimmiLogo className="h-4.5 w-auto" title="dimmi" />
        </a>
        <span className="flex items-center gap-4">
          <span>© 2026 dimmi. MIT licensed.</span>
          <a
            href={GITHUB_URL}
            aria-label="GitHub"
            data-traffic-event="github_click"
            className="text-foreground-icon-secondary hover:text-foreground-icon-primary"
          >
            <RiGithubFill className="size-4.5" />
          </a>
        </span>
      </div>
    </>
  );
}

function PageCard({ link, direction }: { link: NonNullable<PageLink>; direction: "prev" | "next" }) {
  const Icon = direction === "prev" ? RiArrowLeftLine : RiArrowRightLine;
  return (
    <a
      href={link.href}
      rel={direction}
      className={cx(
        "group flex flex-col gap-1 rounded-2xl border border-border-button-default bg-background-full p-4 shadow-xs transition-colors hover:border-border-button-hover",
        direction === "next" && "items-end text-end sm:col-start-2",
      )}
    >
      <span className="flex items-center gap-1 text-caption-1-regular text-text-tertiary">
        {direction === "prev" && (
          <Icon className="size-3.5 transition-transform group-hover:-translate-x-0.5" aria-hidden />
        )}
        {direction === "prev" ? "Previous" : "Next"}
        {direction === "next" && (
          <Icon className="size-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden />
        )}
      </span>
      <span className="text-body-medium text-text-primary">{link.label}</span>
    </a>
  );
}

export function Notice({ children }: { children: ReactNode }) {
  return (
    <div className="mb-6 rounded-2xl border border-separator-border bg-status-yellow-background px-4 py-3 text-body-regular text-status-yellow-text">
      {children}
    </div>
  );
}
