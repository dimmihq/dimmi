import { RiArrowDownSLine } from "@remixicon/react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import { MethodChip } from "./content";

/*
 * The docs sidebar, in the app sidebar's rows: section labels in the
 * caption tone, 32px rows, the current page on the secondary surface.
 * It renders to static HTML; groups fold with <details>, which open and
 * close on BoardUI's disclosure motion (docs.css). API categories hold
 * resources, which hold endpoints led by their HTTP method.
 */

type Badge = { text: string; variant: string } | undefined;
export type SidebarEntry =
  | { type: "link"; label: string; href: string; isCurrent: boolean; badge: Badge; method?: string }
  | { type: "group"; label: string; entries: SidebarEntry[]; collapsed: boolean; badge: Badge };

type Group = Extract<SidebarEntry, { type: "group" }>;
type Link = Extract<SidebarEntry, { type: "link" }>;

const badgeColor: Record<string, ChipColor> = {
  note: "blue",
  tip: "purple",
  success: "lime",
  caution: "yellow",
  danger: "rose",
  default: "neutral",
};

function hasCurrent(entry: SidebarEntry): boolean {
  return entry.type === "link" ? entry.isCurrent : entry.entries.some(hasCurrent);
}

const isOpen = (group: Group) => !group.collapsed || hasCurrent(group);

export function DocsSidebar({ entries }: { entries: SidebarEntry[] }) {
  return (
    <div className="flex flex-col gap-1">
      {entries.map((entry) =>
        entry.type === "link" ? (
          <SidebarLink key={entry.href} entry={entry} />
        ) : (
          <Section key={entry.label} group={entry} />
        ),
      )}
    </div>
  );
}

function Chevron({ nested = false }: { nested?: boolean }) {
  return (
    <RiArrowDownSLine
      aria-hidden
      className={cx(
        "size-4 shrink-0 -rotate-90 text-foreground-icon-tertiary transition-[rotate] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none",
        nested ? "group-open/nested:rotate-0" : "group-open/section:rotate-0",
      )}
    />
  );
}

function Entries({ entries }: { entries: SidebarEntry[] }) {
  return (
    <ul className="docs-disclosure-body mt-1 flex flex-col gap-0.5">
      {entries.map((child) => (
        <li key={child.type === "link" ? child.href : child.label}>
          {child.type === "link" ? <SidebarLink entry={child} /> : <Nested group={child} />}
        </li>
      ))}
    </ul>
  );
}

// A top-level group: a caption label over its rows.
function Section({ group }: { group: Group }) {
  return (
    <details open={isOpen(group)} className="docs-disclosure group/section open:pb-4">
      <summary className="flex h-8 cursor-pointer list-none items-center justify-between rounded-2lg px-2 text-caption-1-medium text-text-tertiary transition-colors hover:bg-background-secondary-hover hover:text-text-secondary [&::-webkit-details-marker]:hidden">
        {group.label}
        <Chevron />
      </summary>
      <Entries entries={group.entries} />
    </details>
  );
}

// A group inside a group (an API resource): a row that folds open,
// its pages indented on a guide line.
function Nested({ group }: { group: Group }) {
  return (
    <details open={isOpen(group)} className="docs-disclosure group/nested">
      <summary className="docs-side-link list-none justify-between [&::-webkit-details-marker]:hidden">
        <span className="min-w-0 truncate">{group.label}</span>
        <Chevron nested />
      </summary>
      <div className="ms-2.5 border-s border-separator-border ps-1.5">
        <Entries entries={group.entries} />
      </div>
    </details>
  );
}

function SidebarLink({ entry }: { entry: Link }) {
  // .docs-side-link (docs.css) holds the row's styles: the API sidebar
  // repeats it hundreds of times.
  return (
    <a
      href={entry.href}
      aria-current={entry.isCurrent ? "page" : undefined}
      className={entry.method ? "docs-side-link items-start" : "docs-side-link"}
    >
      {entry.method ? <MethodChip method={entry.method} short /> : null}
      <span className="min-w-0 flex-1">{entry.label}</span>
      {entry.badge ? (
        <Chip variant="caption" color={badgeColor[entry.badge.variant] ?? "neutral"}>
          {entry.badge.text}
        </Chip>
      ) : null}
    </a>
  );
}
