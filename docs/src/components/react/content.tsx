import {
  RiAlertLine,
  RiArrowRightLine,
  RiArrowRightUpLine,
  RiCheckboxCircleLine,
  RiErrorWarningLine,
  RiInformationLine,
  RiLightbulbFlashLine,
} from "@remixicon/react";
import type { ReactNode } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import { type IconComponent, icon } from "./icons";

/*
 * The building blocks docs pages are written with, on BoardUI. The .astro
 * wrappers in components/mdx hand them their content; the interactive
 * ones (tabs, accordion, file tree) hydrate, the rest are static HTML.
 */

/* Callout: a tinted aside for notes, tips and warnings. */

export type CalloutType = "note" | "tip" | "success" | "caution" | "danger";

const callouts: Record<CalloutType, { icon: IconComponent; label: string; className: string; iconClass: string }> = {
  note: {
    icon: RiInformationLine,
    label: "Note",
    className: "border-separator-border bg-background-secondary-default",
    iconClass: "text-foreground-icon-secondary",
  },
  tip: {
    icon: RiLightbulbFlashLine,
    label: "Tip",
    className:
      "border-[color-mix(in_oklab,var(--color-accent-500)_28%,var(--color-background-full))] bg-[color-mix(in_oklab,var(--color-accent-500)_9%,var(--color-background-full))]",
    iconClass: "text-accent-600 dark:text-accent-400",
  },
  success: {
    icon: RiCheckboxCircleLine,
    label: "Done",
    className: "border-transparent bg-status-lime-background",
    iconClass: "text-status-lime-text",
  },
  caution: {
    icon: RiAlertLine,
    label: "Caution",
    className: "border-transparent bg-status-yellow-background",
    iconClass: "text-status-yellow-text",
  },
  danger: {
    icon: RiErrorWarningLine,
    label: "Danger",
    className: "border-transparent bg-status-rose-background",
    iconClass: "text-status-rose-text",
  },
};

export function Callout({
  type = "note",
  title,
  children,
}: {
  type?: CalloutType;
  title?: string;
  children?: ReactNode;
}) {
  const style = callouts[type];
  const Icon = style.icon;
  return (
    <aside className={cx("not-prose flex gap-3 rounded-2xl border px-4 py-3.5", style.className)}>
      <Icon className={cx("mt-0.5 size-5 shrink-0", style.iconClass)} aria-hidden />
      <div className="flex min-w-0 flex-col gap-1 text-body-regular text-text-secondary [&_a]:text-text-primary [&_a]:underline [&_code]:font-mono [&_code]:text-[0.8125em] [&_code]:text-text-primary [&_p+p]:mt-2">
        <p className="text-body-medium text-text-primary">{title ?? style.label}</p>
        <div>{children}</div>
      </div>
    </aside>
  );
}

/* Badge: BoardUI's chip, inline in running text or headings. */

export function Badge({ text, color = "neutral" }: { text: string; color?: ChipColor }) {
  return (
    <Chip variant="caption" color={color} className="align-middle">
      {text}
    </Chip>
  );
}

/* Cards: a surface with an icon, a title and a line or two of copy. */

export function Card({
  title,
  icon: iconName,
  href,
  children,
}: {
  title: string;
  icon?: string;
  href?: string;
  children?: ReactNode;
}) {
  const Icon = icon(iconName);
  const body = (
    <>
      <span className="flex items-center justify-between gap-3">
        {Icon ? (
          <span className="grid size-9 place-items-center rounded-2lg border border-separator-border bg-background-secondary-default">
            <Icon className="size-4.5 text-foreground-icon-primary" aria-hidden />
          </span>
        ) : (
          <span />
        )}
        {href ? (
          <RiArrowRightUpLine
            className="size-4.5 text-foreground-icon-tertiary transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5"
            aria-hidden
          />
        ) : null}
      </span>
      <span className="text-headline-medium text-text-primary">{title}</span>
      {children ? <span className="text-body-regular text-text-secondary [&_p]:m-0">{children}</span> : null}
    </>
  );
  const className =
    "group not-prose no-underline flex flex-col gap-3 rounded-2xl border border-border-button-default bg-background-full p-5 shadow-xs";
  return href ? (
    <a href={href} className={cx(className, "transition-colors hover:border-border-button-hover")}>
      {body}
    </a>
  ) : (
    <div className={className}>{body}</div>
  );
}

export function CardGrid({ cols = 2, children }: { cols?: 2 | 3; children?: ReactNode }) {
  return (
    <div
      className={cx(
        "not-prose grid gap-3 [&>astro-slot]:contents",
        cols === 3 ? "sm:grid-cols-2 lg:grid-cols-3" : "sm:grid-cols-2",
      )}
    >
      {children}
    </div>
  );
}

export function LinkCard({ title, description, href }: { title: string; description?: string; href: string }) {
  return (
    <a
      href={href}
      className="group not-prose no-underline flex items-center gap-4 rounded-2xl border border-border-button-default bg-background-full px-5 py-4 shadow-xs transition-colors hover:border-border-button-hover"
    >
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-body-medium text-text-primary">{title}</span>
        {description ? <span className="text-body-regular text-text-secondary">{description}</span> : null}
      </span>
      <RiArrowRightLine
        className="size-5 shrink-0 text-foreground-icon-secondary transition-transform group-hover:translate-x-0.5"
        aria-hidden
      />
    </a>
  );
}

/* API reference pieces: an endpoint line and a parameter row. */

const methodColor: Record<string, ChipColor> = {
  GET: "blue",
  POST: "lime",
  PUT: "orange",
  PATCH: "yellow",
  DELETE: "rose",
};

// An HTTP method as a coloured pill; `short` writes DELETE as DEL to fit the sidebar.
export function MethodChip({ method, short = false }: { method: string; short?: boolean }) {
  const verb = method.toUpperCase();
  const text = short && verb === "DELETE" ? "DEL" : verb;
  return (
    <Chip
      variant="caption"
      color={methodColor[verb] ?? "neutral"}
      className={cx(
        "shrink-0 justify-center font-mono",
        short && "mt-0.5 h-4 w-9 rounded px-0 py-0 text-[0.625rem] leading-none font-semibold",
      )}
    >
      {text}
    </Chip>
  );
}

export function Endpoint({ method, path }: { method: string; path: string }) {
  const verb = method.toUpperCase();
  return (
    <div className="not-prose flex items-center gap-3 overflow-x-auto rounded-2xl border border-separator-border bg-background-secondary-default px-4 py-3">
      <MethodChip method={verb} />
      <code className="border-0 bg-transparent p-0 font-mono text-[0.8125rem] whitespace-nowrap text-text-primary">
        {path}
      </code>
    </div>
  );
}

export function Param({
  name,
  type,
  required = false,
  defaultValue,
  children,
}: {
  name: string;
  type: string;
  required?: boolean;
  defaultValue?: string;
  children?: ReactNode;
}) {
  return (
    <div className="not-prose flex flex-col gap-1.5 border-b border-separator-border py-4 last:border-b-0">
      <div className="flex flex-wrap items-center gap-2">
        <code className="font-mono text-[0.8125rem] font-medium text-text-primary">{name}</code>
        <Chip variant="caption" color="soft" className="font-mono">
          {type}
        </Chip>
        {required ? (
          <Chip variant="caption" color="rose">
            required
          </Chip>
        ) : (
          defaultValue && <span className="text-caption-1-regular text-text-tertiary">default {defaultValue}</span>
        )}
      </div>
      {children ? (
        <div className="text-body-regular text-text-secondary [&_code]:font-mono [&_code]:text-[0.8125em] [&_code]:text-text-primary [&_p]:m-0">
          {children}
        </div>
      ) : null}
    </div>
  );
}
