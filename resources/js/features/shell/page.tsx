/*
 * The parts every page of the app is built from: floating panels beside
 * the sidebar, a header with the page's title and actions, a scroll area
 * whose top edge softens once scrolled, and the empty state a page shows
 * when it has nothing to list, or until its feature is built.
 */
import type { ComponentType, ReactNode } from "react";

import { ScrollFade, useScrollFade } from "@/components/foundations/scroll-fade/scroll-fade";
import { cx } from "@/utils/cx";

/*
 * Panel is one floating card of a page, on BoardUI's app geometry: the
 * sidebar's own card, with its light edge and shadow, so every pane
 * beside the sidebar reads as its sibling. A page with several panes
 * puts each in its own Panel.
 *
 * BoardUI's surfaces come in one order: the full ground, secondary cards
 * on it, and primary or tertiary things on those. Controls are drawn for
 * that order (a switch track or a field is tertiary), so a pane that
 * holds cards of controls sits on the ground surface, the way BoardUI's
 * settings modal does, and its cards are secondary. Every other pane is
 * a secondary card itself.
 *
 * A panel clips what overflows it rather than hiding it, since a hidden
 * overflow can still be scrolled by the browser, as when focus lands on
 * something outside it, and the whole pane then slides away from its
 * frame.
 */
export function Panel({
  label,
  ground = false,
  className,
  children,
}: {
  label?: string;
  ground?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <section
      aria-label={label}
      className={cx(
        "flex min-h-0 min-w-0 flex-col overflow-clip rounded-3xl border border-border-button-white shadow-sidebar",
        ground ? "bg-background-full" : "bg-background-secondary-default",
        className,
      )}
    >
      {children}
    </section>
  );
}

/*
 * ScrollArea is the scrolling body of a panel, below its header. Its top
 * edge softens into the panel's surface once scrolled; ground says the
 * panel is on the ground surface. The scrolling body positions what is
 * placed absolutely inside it, such as the hidden input of a switch, so
 * those move with the content and focusing them scrolls only this body.
 */
export function ScrollArea({
  ground = false,
  className,
  children,
}: {
  ground?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const fade = useScrollFade();

  return (
    <div className="relative min-h-0 flex-1">
      <div onScroll={fade.onScroll} className={cx("relative h-full overflow-y-auto [scrollbar-width:none]", className)}>
        {children}
      </div>
      <ScrollFade
        progress={fade.progress}
        surfaceClassName={ground ? "from-background-full" : "from-background-secondary-default"}
      />
    </div>
  );
}

/*
 * PageHeader titles a panel. The page's main panel takes the level 1
 * heading; panels beside it pass level 2.
 */
export function PageHeader({ title, level = 1, children }: { title: ReactNode; level?: 1 | 2; children?: ReactNode }) {
  const Heading = level === 1 ? "h1" : "h2";

  return (
    <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-4 pe-2.5 pt-[7px]">
      <Heading className="min-w-0 truncate text-body-medium text-text-primary">{title}</Heading>
      {children ? <div className="flex items-center gap-2">{children}</div> : null}
    </header>
  );
}

export function EmptyState({
  icon: Icon,
  title,
  description,
  children,
}: {
  icon: ComponentType<{ className?: string }>;
  title: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 py-12 text-center">
      <span
        aria-hidden
        className="flex size-11 items-center justify-center rounded-full bg-background-primary-default shadow-card"
      >
        <Icon className="size-5 text-foreground-icon-secondary" />
      </span>
      <div className="flex max-w-sm flex-col gap-1">
        <p className="text-body-medium text-text-primary">{title}</p>
        {description ? <p className="text-body-regular text-text-secondary">{description}</p> : null}
      </div>
      {children}
    </div>
  );
}

/*
 * FeaturePage is a page whose feature is not built yet: its header, and
 * an empty state saying what will live there.
 */
export function FeaturePage({
  title,
  icon,
  description,
}: {
  title: string;
  icon: ComponentType<{ className?: string }>;
  description: string;
}) {
  return (
    <Panel className="h-full flex-1">
      <PageHeader title={title} />
      <EmptyState icon={icon} title={`${title} is on its way`} description={description} />
    </Panel>
  );
}
