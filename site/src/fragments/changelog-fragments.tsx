/*
 * Product fragments for the changelog page: the public changelog as
 * resources/views/changelog draws it, the messenger's changelog tab
 * (resources/widget/views/changelog.tsx), the subscriber email, and the
 * dashboard's publish dialog.
 */
import {
  RiArrowRightLine,
  RiCalendarScheduleLine,
  RiChat3Line,
  RiCheckLine,
  RiHome5Line,
  RiMailLine,
  RiMegaphoneLine,
  RiQuestionLine,
  RiRssLine,
  RiSendPlaneLine,
} from "@remixicon/react";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import { BrowserBar, PortalHeader } from "./help-center-fragments";

function Card({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div
      className={cx(
        "w-full rounded-xl border border-border-button-default bg-background-full shadow-[0_16px_40px_-24px_rgb(0_0_0/0.25)]",
        className,
      )}
    >
      {children}
    </div>
  );
}

/* The labels a changelog starts with, each with its dot. */
const dots = {
  New: "bg-accent-500",
  Improved: "bg-status-blue-text",
  Fixed: "bg-status-orange-text",
} as const;

type LabelName = keyof typeof dots;

function Label({ name }: { name: LabelName }) {
  return (
    <span className="inline-flex h-6 items-center gap-1.5 rounded-full bg-background-secondary-default px-2.5 text-caption-1-medium text-text-secondary">
      <span className={cx("size-2 rounded-full", dots[name])} />
      {name}
    </span>
  );
}

function Version({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex h-6 items-center rounded-md border border-border-button-default px-1.5 font-mono text-[11px] font-medium text-text-secondary">
      {children}
    </span>
  );
}

const reactions = [
  { emoji: "👍", count: 24 },
  { emoji: "🎉", count: 11 },
  { emoji: "🚀", count: 6 },
];

function Reactions() {
  return (
    <span className="flex gap-1.5">
      {reactions.map((r) => (
        <span
          key={r.emoji}
          className="inline-flex h-8 items-center gap-1.5 rounded-full border border-border-button-default bg-background-primary-default px-2.5 text-body-2-medium text-text-secondary tabular-nums"
        >
          <span className="text-[15px] leading-none">{r.emoji}</span>
          {r.count}
        </span>
      ))}
    </span>
  );
}

function Avatars() {
  return (
    <span className="flex -space-x-1.5">
      {["LF", "KM"].map((initials) => (
        <span
          key={initials}
          className="grid size-6 place-items-center rounded-full bg-background-tertiary-default text-[10px] font-medium text-text-secondary ring-2 ring-background-full"
        >
          {initials}
        </span>
      ))}
    </span>
  );
}

/* A drawn cover: a dashboard's tiles in the brand's tint. */
function Cover({ className }: { className?: string }) {
  return (
    <div
      className={cx(
        "grid grid-cols-3 gap-2 overflow-hidden rounded-xl border border-separator-border bg-accent-500/8 p-3",
        className,
      )}
    >
      {[38, 62, 46].map((h) => (
        <div key={h} className="flex items-end rounded-md bg-background-full p-2">
          <div className="w-full rounded-sm bg-accent-500/50" style={{ height: h }} />
        </div>
      ))}
    </div>
  );
}

/* The year as a row of weeks, a brand tick for each week that shipped. */
const shipped = new Set([1, 3, 4, 7, 9, 10, 13, 15, 18, 19, 22, 24, 26, 27]);
const months: Record<number, string> = { 0: "Jul", 5: "Aug", 9: "Sep", 13: "Oct", 18: "Nov", 22: "Dec" };

export function ReleaseStrip() {
  return (
    <ol className="flex items-end">
      {Array.from({ length: 28 }, (_, week) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: weeks are fixed
        <li key={week} className="relative flex h-12 flex-1 items-end justify-center">
          {months[week] ? (
            <span className="absolute start-0 top-0 font-mono text-[10px] leading-3 text-text-tertiary">
              {months[week]}
            </span>
          ) : null}
          {shipped.has(week) ? (
            <span className={cx("block w-1 rounded-full bg-accent-500", week === 26 ? "h-9" : "h-6")} />
          ) : (
            <span className="block h-3 w-px bg-border-button-default" />
          )}
        </li>
      ))}
    </ol>
  );
}

const filters = [
  { name: "All updates", count: 0 },
  { name: "New", count: 12 },
  { name: "Improved", count: 18 },
  { name: "Fixed", count: 9 },
];

function LabelFilter() {
  return (
    <span className="flex flex-wrap justify-center gap-2">
      {filters.map((f, i) => (
        <span
          key={f.name}
          className={cx(
            "inline-flex h-8 items-center gap-2 rounded-full border px-3 text-body-2-medium",
            i === 0
              ? "border-border-button-hover bg-background-secondary-default text-text-primary"
              : "border-border-button-default text-text-secondary",
          )}
        >
          {f.name in dots ? <span className={cx("size-2 rounded-full", dots[f.name as LabelName])} /> : null}
          {f.name}
          {f.count ? <span className="text-text-tertiary tabular-nums">{f.count}</span> : null}
        </span>
      ))}
    </span>
  );
}

/*
 * The public changelog page: its header with the subscribe form, the
 * release strip, the label filter, then the newest entry on the
 * timeline with its date and version in the gutter.
 */
export function ChangelogPageFragment() {
  return (
    <div className="w-full max-w-[780px]" aria-hidden>
      <Card className="h-[580px] overflow-hidden rounded-b-none border-b-0">
        <BrowserBar address="help.lumen.app/changelog" />
        <PortalHeader current="changelog" />
        <div className="mx-auto flex max-w-[640px] flex-col px-6">
          <div className="flex flex-col items-center pt-9 text-center">
            <span className="text-title-1-medium tracking-tight text-text-primary">Changelog</span>
            <span className="mt-1.5 text-body-regular text-text-secondary">
              The latest Lumen releases and improvements.
            </span>
            <div className="mt-5 flex w-full max-w-[340px] gap-2">
              <span className="flex h-9 min-w-0 flex-1 items-center gap-2 rounded-xl border border-border-button-default bg-background-primary-default px-3 text-body-2-regular text-text-placeholder shadow-xs">
                <RiMailLine className="size-4 text-foreground-icon-tertiary" />
                you@company.com
              </span>
              <span className="inline-flex h-9 items-center rounded-xl bg-accent-600 px-3.5 text-body-2-medium text-text-white">
                Subscribe
              </span>
            </div>
            <span className="mt-2.5 inline-flex items-center gap-1.5 text-caption-1-regular text-text-tertiary">
              <RiRssLine className="size-3.5" />
              Subscribe to RSS
            </span>
          </div>
          <div className="mt-8">
            <ReleaseStrip />
          </div>
          <div className="mt-5">
            <LabelFilter />
          </div>
          <div className="mt-9 grid grid-cols-[96px_minmax(0,1fr)] gap-6">
            <div className="flex flex-col items-start gap-2 pt-1">
              <span className="text-body-2-medium text-text-secondary">Nov 12, 2026</span>
              <Version>v4.2</Version>
            </div>
            <div className="relative border-s border-separator-border ps-6">
              <span className="absolute -start-[5px] top-2 size-[9px] rounded-full bg-accent-500 ring-4 ring-background-full" />
              <Label name="New" />
              <p className="mt-2.5 text-title-2-medium tracking-tight text-text-primary">
                Share dashboards with clients
              </p>
              <p className="mt-2 text-body-regular text-text-secondary">
                Send a read-only link to any dashboard. It stays live as your data changes.
              </p>
              <Cover className="mt-4 h-[130px]" />
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
}

/* An update being written: its state, labels, version and authors. */
export function EntryFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <div className="flex items-center justify-between border-b border-separator-border px-4 py-3">
        <span className="text-body-medium text-text-primary">Update</span>
        <Chip variant="caption" color="gray">
          Draft
        </Chip>
      </div>
      <div className="flex flex-col gap-3 px-4 py-3.5">
        <span className="text-headline-semibold text-text-primary">Share dashboards with clients</span>
        <span className="flex flex-wrap items-center gap-1.5">
          <Label name="New" />
          <Label name="Improved" />
          <Version>v4.2</Version>
        </span>
        <span className="flex items-center gap-2 text-caption-1-regular text-text-tertiary">
          <Avatars />
          Lena Fischer and Kofi Mensah
        </span>
      </div>
    </Card>
  );
}

/* Publishing: now or at a time picked ahead, and the subscriber email. */
export function PublishFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <div className="border-b border-separator-border px-4 py-3 text-body-medium text-text-primary">
        Publish update
      </div>
      <div className="flex flex-col gap-2 px-4 py-3.5">
        {[
          { icon: RiSendPlaneLine, title: "Now", detail: "Readers see it right away", picked: false },
          { icon: RiCalendarScheduleLine, title: "Schedule", detail: "Thu, Nov 20 at 9:00", picked: true },
        ].map((option) => (
          <span
            key={option.title}
            className={cx(
              "flex items-center gap-2.5 rounded-lg border px-3 py-2",
              option.picked ? "border-accent-500 bg-accent-500/6" : "border-border-button-default",
            )}
          >
            <option.icon className="size-4 text-foreground-icon-secondary" />
            <span className="flex flex-col">
              <span className="text-body-2-medium text-text-primary">{option.title}</span>
              <span className="text-caption-1-regular text-text-tertiary">{option.detail}</span>
            </span>
          </span>
        ))}
        <span className="mt-1 flex items-center gap-2 text-body-2-regular text-text-primary">
          <span className="grid size-4 place-items-center rounded bg-accent-600 text-text-white">
            <RiCheckLine className="size-3" />
          </span>
          Email 1,284 subscribers
        </span>
      </div>
    </Card>
  );
}

const widgetEntries = [
  {
    tag: "New",
    title: "Share dashboards with clients",
    summary: "Send a read-only link to any dashboard.",
    date: "Nov 12",
  },
  {
    tag: "Improved",
    title: "Faster CSV exports",
    summary: "Large reports now export in seconds.",
    date: "Nov 4",
  },
  {
    tag: "Fixed",
    title: "Time zones in scheduled reports",
    summary: "Reports now go out at your local time.",
    date: "Oct 28",
  },
];

const tabs = [
  { icon: RiHome5Line, name: "Home" },
  { icon: RiChat3Line, name: "Messages" },
  { icon: RiMegaphoneLine, name: "Changelog", current: true },
  { icon: RiQuestionLine, name: "Help" },
];

/*
 * The messenger's changelog tab: a card per update with its label,
 * title, summary and date, over the tab bar.
 */
export function MessengerChangelogFragment({ compact = false }: { compact?: boolean }) {
  const entries = compact ? widgetEntries.slice(0, 2) : widgetEntries;
  return (
    <div
      className={cx(
        "flex w-full flex-col overflow-hidden rounded-t-[22px] border border-b-0 border-border-button-default bg-background-primary-default shadow-[0_16px_40px_-24px_rgb(0_0_0/0.25)]",
        compact ? "max-w-[300px] self-end" : "max-w-[360px]",
      )}
      aria-hidden
    >
      <div className="px-4 py-3.5 text-center text-body-medium text-text-primary">Changelog</div>
      <div className="flex flex-col gap-2.5 px-3">
        {entries.map((entry) => (
          <div
            key={entry.title}
            className="flex gap-3 rounded-2xl border border-separator-border bg-background-full p-2.5 shadow-xs"
          >
            {compact ? null : (
              <span className="grid size-14 shrink-0 place-items-center rounded-xl bg-accent-500/10">
                <span className="grid size-6 place-items-center rounded-md bg-accent-600 text-caption-1-medium text-text-white">
                  L
                </span>
              </span>
            )}
            <span className="flex min-w-0 flex-col gap-0.5">
              <span className="self-start rounded-full bg-accent-500/12 px-1.5 text-[11px] leading-4 font-medium text-accent-700 dark:text-accent-400">
                {entry.tag}
              </span>
              <span className="truncate text-body-2-medium text-text-primary">{entry.title}</span>
              <span className="truncate text-caption-1-regular text-text-secondary">{entry.summary}</span>
              <span className="text-[11px] leading-4 text-text-tertiary">{entry.date}</span>
            </span>
          </div>
        ))}
      </div>
      <div className="mt-3 flex border-t border-separator-border px-2 py-1.5">
        {tabs.map((tab) => (
          <span
            key={tab.name}
            className={cx(
              "relative flex flex-1 flex-col items-center gap-0.5 py-1 text-[11px] leading-4",
              tab.current ? "text-text-primary" : "text-text-tertiary",
            )}
          >
            <tab.icon className="size-[18px]" />
            {tab.name}
            {tab.current ? (
              <span className="absolute top-0.5 end-[calc(50%-14px)] size-2 rounded-full bg-accent-500 ring-2 ring-background-primary-default" />
            ) : null}
          </span>
        ))}
      </div>
    </div>
  );
}

/* The email a subscriber gets: the whole update, in the brand. */
export function EmailFragment() {
  return (
    <Card className="max-w-[420px] rounded-b-none border-b-0">
      <div className="flex flex-col gap-0.5 border-b border-separator-border px-5 py-3.5">
        <span className="text-body-medium text-text-primary">Lumen</span>
        <span className="text-caption-1-regular text-text-tertiary">to nia@pinecrest.com</span>
      </div>
      <div className="flex flex-col gap-3 px-5 py-5">
        <span className="text-caption-1-medium text-text-tertiary">New in Lumen</span>
        <span className="text-title-3-medium text-text-primary">Share dashboards with clients</span>
        <span className="text-body-regular text-text-secondary">
          Send a read-only link to any dashboard. It stays live as your data changes, and you can turn the link off at
          any time.
        </span>
        <Cover className="h-[110px]" />
        <span className="inline-flex items-center gap-1.5 self-start rounded-md bg-accent-600 px-3.5 py-2 text-body-2-medium text-text-white">
          Read the update
          <RiArrowRightLine className="size-4" />
        </span>
        <span className="mt-2 text-caption-1-regular text-text-tertiary">
          You get these because you subscribed to the Lumen changelog. Unsubscribe.
        </span>
      </div>
    </Card>
  );
}

/*
 * An entry's close, as its own page ends: the reactions readers leave,
 * the way to ask about it, and the box to get the next one by email.
 */
export function EntryFooterFragment() {
  return (
    <Card className="max-w-[440px] rounded-b-none border-b-0">
      <div className="flex flex-col gap-2 px-5 pt-5">
        <span className="text-title-3-medium text-text-primary">Share dashboards with clients</span>
        <span className="text-body-regular text-text-secondary">
          Send a read-only link to any dashboard. It stays live as your data changes.
        </span>
        <span className="mt-1 flex items-center gap-2 text-caption-1-regular text-text-tertiary">
          <Avatars />
          Lena Fischer and Kofi Mensah
        </span>
      </div>
      <div className="mt-5 flex items-center gap-3 border-y border-separator-border px-5 py-4">
        <Reactions />
        <span className="ms-auto inline-flex items-center gap-1.5 text-body-2-medium text-text-secondary">
          <RiChat3Line className="size-4" />
          Questions?
        </span>
      </div>
      <div className="m-5 flex flex-col items-center gap-1 rounded-2xl border border-border-button-default bg-background-secondary-default px-5 py-5 text-center">
        <span className="text-body-medium text-text-primary">Get updates in your inbox</span>
        <span className="text-caption-1-regular text-text-secondary">
          One email when we ship something worth knowing about.
        </span>
        <span className="mt-3 flex w-full max-w-[300px] gap-2">
          <span className="flex h-9 min-w-0 flex-1 items-center gap-2 rounded-xl border border-border-button-default bg-background-primary-default px-3 text-body-2-regular text-text-placeholder">
            <RiMailLine className="size-4 text-foreground-icon-tertiary" />
            you@company.com
          </span>
          <span className="inline-flex h-9 items-center rounded-xl bg-accent-600 px-3.5 text-body-2-medium text-text-white">
            Subscribe
          </span>
        </span>
      </div>
    </Card>
  );
}
