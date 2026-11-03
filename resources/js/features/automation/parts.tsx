/*
 * Parts the automation pages share: the frame of a list page (a title,
 * a line on what the page is for, the main action, and a centred column
 * of cards on the ground surface, like settings), a row that opens an
 * item, the tinted mark of a step kind, and the chip for a status.
 */
import { Link } from "@tanstack/react-router";
import type { ComponentType, ReactNode } from "react";

import { CHIP_COLORS, Chip, type ChipColor } from "@/components/base/badges/chip";
import { Panel, ScrollArea } from "@/features/shell/page";
import { cx } from "@/utils/cx";

export function ListPage({
  title,
  description,
  action,
  children,
}: {
  title: string;
  description: string;
  action: ReactNode;
  children: ReactNode;
}) {
  return (
    <Panel label={title} ground className="h-full flex-1">
      <ScrollArea ground>
        <div className="mx-auto flex w-full max-w-220 flex-col gap-8 px-4 pt-6 pb-16 sm:gap-10 sm:px-6 sm:pt-10">
          <header className="flex flex-col items-start gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div className="flex flex-col gap-1">
              <h1 className="text-title-2-medium text-text-primary">{title}</h1>
              <p className="text-body-regular text-text-secondary">{description}</p>
            </div>
            {action}
          </header>
          {children}
        </div>
      </ScrollArea>
    </Panel>
  );
}

/*
 * OpenRow is a list row that opens the item it names.
 */
/*
 * Opened is where a list row leads: a workflow's builder or a
 * procedure's editor.
 */
export type Opened =
  | { to: "/d/workflows/$workflow"; params: { workflow: string } }
  | { to: "/d/procedures/$procedure"; params: { procedure: string } };

export function OpenRow({
  target,
  mark,
  title,
  detail,
  children,
}: {
  target: Opened;
  mark: ReactNode;
  title: string;
  detail: string;
  children?: ReactNode;
}) {
  return (
    <Link
      {...target}
      className={cx(
        "-ms-3 flex min-h-[60px] items-center gap-3 border-b border-separator-border py-2.5 ps-3 pe-2.5 outline-none last:border-b-0",
        "transition-colors duration-150 first:rounded-t-2xl last:rounded-b-2xl hover:bg-background-secondary-hover",
        "focus-visible:ring-2 focus-visible:ring-border-focus-ring focus-visible:ring-inset",
      )}
    >
      <span className="flex shrink-0">{mark}</span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-regular text-text-primary">{title}</span>
        <span className="truncate text-body-2-regular text-text-secondary">{detail}</span>
      </span>
      {children ? <span className="flex shrink-0 items-center gap-2">{children}</span> : null}
    </Link>
  );
}

/*
 * KindMark is the tinted square mark of a step or trigger kind.
 */
export function KindMark({
  icon: Icon,
  tint,
  size = "md",
}: {
  icon: ComponentType<{ className?: string }>;
  tint: ChipColor;
  size?: "sm" | "md";
}) {
  return (
    <span
      aria-hidden
      className={cx(
        "flex shrink-0 items-center justify-center rounded-lg",
        size === "md" ? "size-8" : "size-6 rounded-md",
        CHIP_COLORS[tint],
      )}
    >
      <Icon className={size === "md" ? "size-[18px]" : "size-4"} />
    </span>
  );
}

const statusTint = { live: "lime", draft: "neutral", paused: "yellow" } as const;
const statusName = { live: "Live", draft: "Draft", paused: "Paused" } as const;

export function StatusChip({ status }: { status: keyof typeof statusTint }) {
  return (
    <Chip variant="caption" color={statusTint[status]}>
      {statusName[status]}
    </Chip>
  );
}
