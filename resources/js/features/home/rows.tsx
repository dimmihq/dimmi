/*
 * The lists and cards on Home. Every row is one link: the whole row
 * answers the click and lights up under the pointer, since a row here
 * only ever opens the page that owns it.
 */
import { Link, type LinkProps } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { cx } from "@/utils/cx";

/*
 * Where is the page a row or a card opens: a route, with what fills its
 * address.
 */
export interface Where {
  to: string;
  params?: Record<string, string>;
  search?: Record<string, string>;
}

/*
 * linkOf hands a Where to the router's link.
 */
function linkOf(where: Where): LinkProps {
  return where as LinkProps;
}

/*
 * Rows is the card a list sits in.
 */
export function Rows({ children }: { children: ReactNode }) {
  return <ul className="flex flex-col overflow-hidden rounded-2xl bg-background-secondary-default">{children}</ul>;
}

/*
 * Row is one line of a list: a mark, a title over a line of detail, and
 * what sits at its end.
 */
export function Row({
  where,
  mark,
  title,
  detail,
  muted = false,
  children,
}: {
  where: Where;
  mark: ReactNode;
  title: ReactNode;
  detail?: ReactNode;
  /* muted draws the title as done with: struck through and faint. */
  muted?: boolean;
  children?: ReactNode;
}) {
  return (
    <li className="border-b border-separator-border last:border-b-0">
      <Link
        {...linkOf(where)}
        className={cx(
          "flex min-h-[52px] items-center gap-3 py-2.5 ps-3 pe-2.5 no-underline outline-none",
          "transition-colors duration-150 hover:bg-background-secondary-hover",
          "focus-visible:ring-2 focus-visible:ring-border-focus-ring focus-visible:ring-inset",
        )}
      >
        <span className="flex shrink-0 items-center">{mark}</span>
        <span className="flex min-w-0 flex-1 flex-col">
          <span
            className={cx(
              "truncate text-body-regular",
              muted ? "text-text-tertiary line-through" : "text-text-primary",
            )}
          >
            {title}
          </span>
          {detail ? <span className="truncate text-body-2-regular text-text-secondary">{detail}</span> : null}
        </span>
        {children ? <span className="flex shrink-0 items-center gap-2">{children}</span> : null}
      </Link>
    </li>
  );
}

/*
 * Blank is the one line a list shows while it has nothing, with the way
 * to fill it beside it.
 */
export function Blank({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <li className="flex min-h-[52px] items-center justify-between gap-4 py-2 ps-3 pe-2.5">
      <span className="text-body-2-regular text-text-tertiary">{children}</span>
      {action}
    </li>
  );
}

/*
 * Card is a block that opens one page wherever it is clicked, yet holds
 * links of its own: the heading's link stretches over the whole card,
 * and the links inside sit above it and keep their own clicks.
 */
export function Card({
  where,
  mark,
  title,
  detail,
  aside,
  children,
}: {
  where: Where;
  mark: ReactNode;
  title: ReactNode;
  detail?: ReactNode;
  /* aside sits at the end of the heading, such as a count. */
  aside?: ReactNode;
  children: ReactNode;
}) {
  return (
    <article
      className={cx(
        "relative flex flex-col gap-3 rounded-2xl bg-background-secondary-default p-4",
        "transition-colors duration-150 hover:bg-background-secondary-hover",
        "has-[a[data-card]:focus-visible]:ring-2 has-[a[data-card]:focus-visible]:ring-border-focus-ring",
      )}
    >
      <header className="flex items-center gap-2">
        {mark}
        <span className="flex min-w-0 flex-1 flex-col">
          <Link
            {...linkOf(where)}
            data-card
            className="truncate text-body-medium text-text-primary no-underline outline-none after:absolute after:inset-0 after:rounded-2xl after:content-['']"
          >
            {title}
          </Link>
          {detail ? <span className="text-body-2-regular text-text-secondary">{detail}</span> : null}
        </span>
        {aside}
      </header>
      <div className="pointer-events-none relative z-10 [&_a]:pointer-events-auto">{children}</div>
    </article>
  );
}
