/*
 * The parts every settings page is built from, on BoardUI's settings
 * recipe: the page on the ground surface, and each group a secondary card
 * whose rows divide themselves with hairlines that stop short of the
 * card's left edge. A row names a setting and explains it on the left,
 * with its control on the right; BoardUI's controls are drawn for exactly
 * this card. Lists (members, sessions, tokens) use the same card with an
 * item per row. The page sits in a centred column in its own panel.
 */
import type { ComponentType, ReactNode } from "react";

import { Panel, ScrollArea } from "@/features/shell/page";
import { cx } from "@/utils/cx";

/*
 * rowLine is BoardUI's settings row: at least 52px tall, padded on the
 * right only (the card pads the left), with a hairline under every row
 * but the last.
 */
const rowLine =
  "flex min-h-[52px] items-center justify-between gap-4 border-b border-separator-border py-2.5 pe-2.5 last:border-b-0";

/*
 * stackLine is a setting row that puts its control under the label on
 * phones, where the two do not fit side by side.
 */
const stackLine =
  "flex min-h-[52px] flex-col items-start gap-2 border-b border-separator-border py-2.5 pe-2.5 last:border-b-0 sm:flex-row sm:items-center sm:justify-between sm:gap-4";

export function SettingsPage({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <Panel label={title} ground className="h-full flex-1">
      <ScrollArea ground>
        <div className="mx-auto flex w-full max-w-[680px] flex-col gap-8 px-4 pt-6 pb-16 sm:gap-10 sm:px-6 sm:pt-10">
          <header className="flex flex-col gap-1">
            <h1 className="text-title-2-medium text-text-primary">{title}</h1>
            {description ? <p className="text-body-regular text-text-secondary">{description}</p> : null}
          </header>
          {children}
        </div>
      </ScrollArea>
    </Panel>
  );
}

/*
 * Section groups rows under a heading. action sits beside the heading,
 * for adding to a list.
 */
export function Section({
  title,
  description,
  action,
  children,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <div className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2 px-3">
        <div className="flex min-w-0 flex-col gap-0.5">
          <h2 className="text-body-2-medium text-text-secondary">{title}</h2>
          {description ? <p className="text-body-2-regular text-text-tertiary">{description}</p> : null}
        </div>
        {action ? <div className="shrink-0">{action}</div> : null}
      </div>
      <div className="flex flex-col rounded-2xl bg-background-secondary-default ps-3">{children}</div>
    </section>
  );
}

/*
 * Row is one setting: its name and what it does, and its control. A
 * muted row shows a setting that is not available here, with the reason
 * in place of the control.
 */
export function Row({
  label,
  description,
  muted = false,
  children,
}: {
  label: ReactNode;
  description?: ReactNode;
  muted?: boolean;
  children?: ReactNode;
}) {
  return (
    <div className={stackLine}>
      <div className={cx("flex min-w-0 flex-col", muted && "opacity-60")}>
        <span className="text-body-regular text-text-primary">{label}</span>
        {description ? <span className="text-body-2-regular text-text-secondary">{description}</span> : null}
      </div>
      {children ? <div className="flex max-w-full shrink-0 flex-wrap items-center gap-2">{children}</div> : null}
    </div>
  );
}

/*
 * Item is one entry of a list card: a mark (an icon or an avatar), a
 * title and a line of detail, and what can be done with it.
 */
export function Item({
  mark,
  title,
  detail,
  children,
}: {
  mark: ReactNode;
  title: ReactNode;
  detail?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className={cx(rowLine, "justify-start gap-3")}>
      <span className="flex shrink-0 items-center">{mark}</span>
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-regular text-text-primary">{title}</span>
        {detail ? <span className="truncate text-body-2-regular text-text-secondary">{detail}</span> : null}
      </div>
      {children ? <div className="flex shrink-0 items-center gap-2">{children}</div> : null}
    </div>
  );
}

/*
 * Glyph is the square mark of a list item that has no picture: a
 * device, a key, a channel.
 */
export function Glyph({ icon: Icon }: { icon: ComponentType<{ className?: string }> }) {
  return (
    <span aria-hidden className="flex size-8 items-center justify-center rounded-lg bg-background-tertiary-default">
      <Icon className="size-4 text-foreground-icon-secondary" />
    </span>
  );
}

/*
 * Logo is the mark of a third-party service: its logo in white on its own
 * brand colour, the way BoardUI's social buttons carry each provider's
 * colour, so a list of services is easy to scan. A hairline keeps dark
 * brands such as GitHub's from melting into a dark card.
 */
export function Logo({ icon: Icon, brand }: { icon: ComponentType<{ className?: string }>; brand: string }) {
  return (
    <span
      aria-hidden
      className="flex size-8 items-center justify-center rounded-lg ring-1 ring-border-button-default ring-inset"
      style={{ backgroundColor: brand }}
    >
      <Icon className="size-[18px] text-text-white" />
    </span>
  );
}

/*
 * Empty fills a list card that has nothing in it yet, with the way to
 * add the first one beside it.
 */
export function Empty({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div className={rowLine}>
      <span className="text-body-2-regular text-text-tertiary">{children}</span>
      {action}
    </div>
  );
}
