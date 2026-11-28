/*
 * The site's frame and type primitives. Every section sits inside the
 * same pair of hairline rails, with a small cross where each section's
 * top edge meets them. Inside the rails the page is open: medium-weight
 * headings at modest sizes, body text in the secondary tone, mono capitals
 * for tags and links, and flat panels that hold one product visual each.
 */
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { RiArrowRightLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { cx } from "@/utils/cx";

export function Container({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cx("mx-auto w-full max-w-[1200px] px-5 sm:px-8", className)}>{children}</div>;
}

function Cross({ side }: { side: "start" | "end" }) {
  return (
    <svg
      viewBox="0 0 11 11"
      aria-hidden
      className={cx(
        "pointer-events-none absolute -top-[5.5px] z-10 size-[11px] text-border-button-hover",
        side === "start" ? "-start-[5.5px]" : "-end-[5.5px]",
      )}
    >
      <path d="M5.5 0v11M0 5.5h11" stroke="currentColor" strokeWidth="1" />
    </svg>
  );
}

export type Tone = "plain" | "brand";

export function Section({
  anchor,
  tone = "plain",
  flush = false,
  className,
  frameClassName,
  children,
}: {
  /* The in-page address the navigation links to, like #agent. */
  anchor?: string;
  tone?: Tone;
  /* No inner padding, for sections that manage their own. */
  flush?: boolean;
  className?: string;
  frameClassName?: string;
  children: ReactNode;
}) {
  return (
    <section
      id={anchor}
      className={cx(
        "relative scroll-mt-16 overflow-x-clip border-t border-separator-border bg-background-full",
        tone === "brand" && "dark site-brand",
        className,
      )}
    >
      <div
        className={cx(
          "relative mx-auto w-full max-w-[1200px] border-x border-separator-border",
          !flush && "px-5 py-20 sm:px-12 md:py-28",
          frameClassName,
        )}
      >
        <Cross side="start" />
        <Cross side="end" />
        {children}
      </div>
    </section>
  );
}

/* Mono capitals: the voice of tags, labels and text links. */
const mono = "font-mono text-[12px] leading-5 font-medium tracking-[0.04em] uppercase";

/* The small label above a hero title: mono capitals after an accent tick. */
export function Eyebrow({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={cx("inline-flex items-center gap-2 text-text-secondary", mono, className)}>
      <span className="size-1.5 bg-accent-500" aria-hidden />
      {children}
    </span>
  );
}

/* An outlined tag that names the product a block is about. */
export function Tag({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={cx(
        "inline-flex self-start rounded-md border border-border-button-default px-2.5 py-1 text-text-tertiary",
        mono,
        className,
      )}
    >
      {children}
    </span>
  );
}

/* A text link in mono capitals with an arrow that nudges on hover. */
export function MonoLink({ href, onClick, children }: { href: string; onClick?: () => void; children: ReactNode }) {
  return (
    <a
      href={href}
      onClick={onClick}
      className={cx("group inline-flex items-center gap-1.5 self-start text-text-primary", mono)}
    >
      {children}
      <RiArrowRightLine className="size-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden />
    </a>
  );
}

/* A section's opening: one centred title and a short line under it. */
export function Intro({ title, body, className }: { title: ReactNode; body?: ReactNode; className?: string }) {
  return (
    <div className={cx("mx-auto flex max-w-[640px] flex-col items-center gap-5 text-center", className)}>
      <h2 className="text-[40px] leading-[1.12] font-medium tracking-[-0.025em] text-balance text-text-primary max-sm:text-[30px]">
        {title}
      </h2>
      {body ? <p className="text-[17px] leading-7 text-pretty text-text-secondary">{body}</p> : null}
    </div>
  );
}

/*
 * A block's heading in two columns: the tag and title on the left, the
 * explanation and a link on the right.
 */
export function SplitHead({
  tag,
  title,
  body,
  link,
}: {
  tag: string;
  title: ReactNode;
  body: ReactNode;
  link?: { href: string; label: string };
}) {
  return (
    <div className="grid grid-cols-1 gap-x-16 gap-y-6 lg:grid-cols-[1fr_minmax(0,460px)] [&>*]:min-w-0">
      <div className="flex flex-col gap-4">
        <Tag>{tag}</Tag>
        <h3 className="max-w-[520px] text-[32px] leading-[1.14] font-medium tracking-[-0.025em] text-balance text-text-primary max-sm:text-[26px]">
          {title}
        </h3>
      </div>
      <div className="flex flex-col gap-4 lg:pt-11">
        <p className="text-[16px] leading-7 text-pretty text-text-primary">{body}</p>
        {link ? <MonoLink href={link.href}>{link.label}</MonoLink> : null}
      </div>
    </div>
  );
}

/* The flat ground one product visual sits on. */
export function Panel({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div className={cx("relative overflow-hidden rounded-xl bg-background-secondary-default", className)}>
      {children}
    </div>
  );
}

/* A short list of points: a title line and a quieter line under it. */
export function Points({ items, className }: { items: { title: string; body: string }[]; className?: string }) {
  return (
    <dl className={cx("flex flex-col gap-7", className)}>
      {items.map((item) => (
        <div key={item.title} className="flex flex-col gap-0.5">
          <dt className="text-[16px] leading-7 text-text-primary">{item.title}</dt>
          <dd className="text-[16px] leading-7 text-text-secondary">{item.body}</dd>
        </div>
      ))}
    </dl>
  );
}

/*
 * A feature's icon: a Phosphor duotone mark in the accent colour, the
 * back tone a light wash of the same green. Every feature icon on the
 * site goes through here, so the set stays one family.
 */
export function FeatureIcon({ icon: Icon }: { icon: PhosphorIcon }) {
  return <Icon weight="duotone" className="size-11 text-accent-600 dark:text-accent-400" aria-hidden />;
}

/* An open grid of features: dotted icon, title, a line of text, a link. */
export function Features({
  items,
  className,
}: {
  items: { icon: PhosphorIcon; title: string; body: string; link?: { href: string; label: string } }[];
  className?: string;
}) {
  return (
    <div className={cx("grid gap-x-6 gap-y-14 sm:grid-cols-2 lg:grid-cols-3 [&>*]:min-w-0", className)}>
      {items.map((item) => (
        <div key={item.title} className="flex flex-col gap-1 lg:pe-10">
          <FeatureIcon icon={item.icon} />
          <h3 className="mt-5 text-[16px] leading-7 text-text-primary">{item.title}</h3>
          <p className="text-[16px] leading-7 text-pretty text-text-secondary">{item.body}</p>
          {item.link ? (
            <span className="mt-3">
              <MonoLink href={item.link.href}>{item.link.label}</MonoLink>
            </span>
          ) : null}
        </div>
      ))}
    </div>
  );
}

/* A hairline between blocks of one section. */
export function Rule({ className }: { className?: string }) {
  return <hr className={cx("border-separator-border", className)} />;
}
