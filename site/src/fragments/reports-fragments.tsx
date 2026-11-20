/*
 * Product fragments for the reports page: a report with its list, filter
 * bar, numbers and a chart; the AI report; the spread of ratings; the
 * teammates table; the conversations behind a number; and the email
 * digest. Drawn with the app's own tiles, chips and chart colours
 * (resources/js/features/reports) so they read as the product.
 */
import { RiArrowDownSLine, RiCalendarLine, RiCloseLine, RiDownload2Line, RiMailLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

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

type Tile = { label: string; value: string; delta?: { text: string; good: boolean }; sub?: string };

/* A number at the top of a report, as metric-tiles.tsx draws it. */
function MetricTile({ tile }: { tile: Tile }) {
  return (
    <div className="flex min-w-0 flex-col gap-1 rounded-2xl bg-background-secondary-default p-4 text-start">
      <span className="flex min-w-0 items-center justify-between gap-2">
        <span className="truncate text-body-2-regular text-text-secondary">{tile.label}</span>
        {tile.delta ? (
          <Chip variant="caption" color={tile.delta.good ? "lime" : "rose"}>
            {tile.delta.text}
          </Chip>
        ) : null}
      </span>
      <span className="text-title-2-medium whitespace-nowrap text-text-primary tabular-nums">{tile.value}</span>
      {tile.sub ? (
        <span className="truncate text-caption-1-medium text-text-tertiary tabular-nums">{tile.sub}</span>
      ) : null}
    </div>
  );
}

function Legend({ items }: { items: { label: string; color: string }[] }) {
  return (
    <span className="flex items-center gap-3">
      {items.map((item) => (
        <span key={item.label} className="flex items-center gap-1.5 text-caption-1-medium text-text-secondary">
          <span className="size-2 rounded-full" style={{ background: item.color }} />
          {item.label}
        </span>
      ))}
    </span>
  );
}

function FilterChip({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex h-8 shrink-0 items-center gap-1 rounded-full border border-border-button-default bg-background-secondary-default ps-3 pe-2 text-body-2-medium text-text-primary">
      {children}
      <RiArrowDownSLine className="size-4 text-foreground-icon-tertiary" aria-hidden />
    </span>
  );
}

const reportList = [
  "Overview",
  "Responsiveness",
  "Resolution",
  "Team and teammates",
  "Live workload",
  "Satisfaction",
  "dimmi AI",
  "Help center",
  "Tickets and SLAs",
];

const heroTiles: Tile[] = [
  { label: "New conversations", value: "1,284", delta: { text: "+8%", good: true } },
  { label: "First response", value: "4m 10s", delta: { text: "-22%", good: true }, sub: "p90 38m · avg 12m" },
  { label: "Resolution time", value: "3h 12m", delta: { text: "+6%", good: false }, sub: "p90 1d 4h · avg 9h" },
  { label: "CSAT", value: "92%", delta: { text: "+3%", good: true } },
];

/* Fourteen days of new and closed conversations. */
const volume = [
  [62, 58],
  [71, 66],
  [48, 52],
  [30, 34],
  [26, 22],
  [74, 69],
  [80, 77],
  [69, 72],
  [77, 70],
  [58, 61],
  [33, 30],
  [28, 31],
  [83, 76],
  [88, 84],
];

function VolumeChart() {
  return (
    <div className="flex h-[150px] items-end gap-[6px] border-b border-separator-border">
      {volume.map(([opened, closed], i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: fixed data, one bar pair per day
        <div key={i} className="flex h-full flex-1 items-end gap-[2px]">
          <span className="flex-1 rounded-t-[3px] bg-chart-1" style={{ height: `${opened}%` }} />
          <span className="flex-1 rounded-t-[3px] bg-chart-2" style={{ height: `${closed}%` }} />
        </div>
      ))}
    </div>
  );
}

/* The hero: a report with its list, filters, numbers and a chart. */
export function ReportsHeroFragment() {
  return (
    <Card className="flex h-[500px] max-w-[980px] overflow-hidden rounded-b-none border-b-0 text-start">
      <nav className="hidden w-48 shrink-0 flex-col gap-0.5 border-e border-separator-border px-2 py-4 md:flex">
        {reportList.map((name, i) => (
          <span
            key={name}
            className={cx(
              "rounded-lg px-2.5 py-1.5 text-body-2-medium",
              i === 0 ? "bg-background-secondary-default text-text-primary" : "text-text-secondary",
            )}
          >
            {name}
          </span>
        ))}
      </nav>
      <div className="flex min-w-0 flex-1 flex-col gap-4 p-5">
        <div className="flex items-center justify-between gap-3">
          <span className="text-headline-semibold text-text-primary">Overview</span>
          <span className="hidden items-center gap-2 sm:flex">
            <span className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-border-button-default px-2.5 text-body-2-medium text-text-primary">
              <RiMailLine className="size-4 text-foreground-icon-secondary" aria-hidden />
              Digest
            </span>
            <span className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-border-button-default px-2.5 text-body-2-medium text-text-primary">
              <RiDownload2Line className="size-4 text-foreground-icon-secondary" aria-hidden />
              Export
            </span>
          </span>
        </div>
        <div className="flex flex-nowrap items-center gap-2 overflow-hidden">
          <span className="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg border border-border-button-default px-2.5 text-body-2-medium text-text-primary">
            <RiCalendarLine className="size-4 text-foreground-icon-secondary" aria-hidden />
            Oct 19 – Nov 17
          </span>
          <span className="inline-flex shrink-0 items-center gap-1.5 text-body-2-medium text-text-primary">
            <span className="flex h-[18px] w-8 items-center justify-end rounded-full bg-accent-600 p-0.5">
              <span className="size-3.5 rounded-full bg-white shadow-xs" />
            </span>
            Compare
          </span>
          <FilterChip>Channel</FilterChip>
          <FilterChip>Team</FilterChip>
          <FilterChip>AI</FilterChip>
          <FilterChip>Tag</FilterChip>
        </div>
        <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
          {heroTiles.map((tile) => (
            <MetricTile key={tile.label} tile={tile} />
          ))}
        </div>
        <div className="flex flex-col gap-3 rounded-2xl border border-separator-border p-4">
          <span className="flex items-center justify-between gap-3">
            <span className="text-body-medium text-text-primary">New and closed conversations</span>
            <Legend
              items={[
                { label: "New", color: "var(--color-chart-1)" },
                { label: "Closed", color: "var(--color-chart-2)" },
              ]}
            />
          </span>
          <VolumeChart />
        </div>
      </div>
    </Card>
  );
}

/* A line through points on a 0..100 scale, in a 300 by 120 box. */
function line(points: number[]): string {
  const step = 300 / (points.length - 1);
  return points.map((y, i) => `${i === 0 ? "M" : "L"}${(i * step).toFixed(1)} ${(120 - y * 1.2).toFixed(1)}`).join(" ");
}

const involvement = [62, 64, 63, 66, 68, 67, 70, 69, 71, 72, 70, 73, 74, 75];
const resolution = [44, 47, 46, 49, 50, 52, 51, 54, 55, 54, 57, 58, 57, 60];

/* The AI report: its numbers and the daily lines. */
export function AiFragment() {
  return (
    <Card className="max-w-[460px] rounded-b-none border-b-0 p-4 text-start">
      <span className="text-headline-semibold text-text-primary">dimmi AI</span>
      <div className="mt-3 grid grid-cols-2 gap-2">
        <MetricTile tile={{ label: "AI involvement", value: "71%", delta: { text: "+5%", good: true } }} />
        <MetricTile tile={{ label: "AI resolution rate", value: "58%", delta: { text: "+9%", good: true } }} />
        <MetricTile tile={{ label: "Confirmed resolutions", value: "412" }} />
        <MetricTile tile={{ label: "Hand-off rate", value: "29%", delta: { text: "-4%", good: true } }} />
      </div>
      <div className="mt-3 flex flex-col gap-3 rounded-2xl border border-separator-border p-4">
        <span className="flex items-center justify-between gap-3">
          <span className="text-body-medium text-text-primary">Involvement and resolution</span>
          <Legend
            items={[
              { label: "Involvement", color: "var(--color-chart-1)" },
              { label: "Resolution", color: "var(--color-chart-2)" },
            ]}
          />
        </span>
        <svg viewBox="0 0 300 120" className="h-[120px] w-full overflow-visible" preserveAspectRatio="none" aria-hidden>
          {[30, 60, 90].map((y) => (
            <line
              key={y}
              x1="0"
              x2="300"
              y1={y}
              y2={y}
              stroke="var(--color-chart-track)"
              strokeDasharray="3 3"
              vectorEffect="non-scaling-stroke"
            />
          ))}
          <path
            d={line(involvement)}
            fill="none"
            stroke="var(--color-chart-1)"
            strokeWidth="2"
            vectorEffect="non-scaling-stroke"
          />
          <path
            d={line(resolution)}
            fill="none"
            stroke="var(--color-chart-2)"
            strokeWidth="2"
            vectorEffect="non-scaling-stroke"
          />
        </svg>
      </div>
    </Card>
  );
}

const spread = [
  { score: 5, count: 318 },
  { score: 4, count: 74 },
  { score: 3, count: 19 },
  { score: 2, count: 9 },
  { score: 1, count: 6 },
];

/* The satisfaction report: CSAT and how the ratings spread. */
export function SatisfactionFragment() {
  const most = Math.max(...spread.map((s) => s.count));
  return (
    <Card className="max-w-[440px] rounded-b-none border-b-0 p-4 text-start">
      <span className="text-headline-semibold text-text-primary">Satisfaction</span>
      <div className="mt-3 grid grid-cols-2 gap-2">
        <MetricTile tile={{ label: "CSAT", value: "92%", delta: { text: "+3%", good: true } }} />
        <MetricTile tile={{ label: "Average rating", value: "4.6", sub: "426 ratings" }} />
      </div>
      <div className="mt-3 flex flex-col gap-3 rounded-2xl border border-separator-border p-4">
        <span className="text-body-medium text-text-primary">Ratings</span>
        <ul className="flex flex-col gap-2">
          {spread.map((row) => (
            <li key={row.score} className="flex items-center gap-3">
              <span className="w-3 text-body-2-medium text-text-secondary tabular-nums">{row.score}</span>
              <span className="h-5 flex-1 rounded-[4px] bg-chart-track">
                <span
                  className={cx("block h-full rounded-[4px]", row.score >= 4 ? "bg-chart-2" : "bg-chart-3")}
                  style={{ width: `${(row.count / most) * 100}%` }}
                />
              </span>
              <span className="w-8 text-end text-caption-1-medium text-text-tertiary tabular-nums">{row.count}</span>
            </li>
          ))}
        </ul>
      </div>
      <p className="mt-3 rounded-xl bg-background-secondary-default px-3.5 py-3 text-body-2-regular text-text-secondary">
        <span className="text-text-primary">Omar Haddad</span> rated 5 · “Fixed in one reply, thank you.”
      </p>
    </Card>
  );
}

const teammates = [
  { name: "Maya", replied: 214, first: "3m", csat: "95%" },
  { name: "Kofi Mensah", replied: 188, first: "5m", csat: "93%" },
  { name: "Lena Fischer", replied: 151, first: "4m", csat: "96%" },
  { name: "Sam Ortiz", replied: 97, first: "9m", csat: "88%" },
];

/* The teammates table on the team report. */
export function TeamFragment() {
  return (
    <Card className="max-w-[460px] rounded-b-none border-b-0 text-start">
      <div className="flex flex-col gap-1 border-b border-separator-border px-4 py-3.5">
        <span className="text-headline-semibold text-text-primary">Team and teammates</span>
        <span className="text-body-2-regular text-text-secondary">Support team · Oct 19 – Nov 17</span>
      </div>
      <div className="grid grid-cols-2 gap-2 px-4 pt-4">
        <MetricTile tile={{ label: "Conversations replied to", value: "650" }} />
        <MetricTile tile={{ label: "Reply time", value: "6m", sub: "p90 41m · avg 14m" }} />
      </div>
      <table className="mt-3 w-full text-body-2-regular">
        <thead>
          <tr className="text-caption-1-medium text-text-tertiary">
            <th className="px-4 py-2 text-start font-medium">Teammate</th>
            <th className="px-2 py-2 text-end font-medium">Replied</th>
            <th className="px-2 py-2 text-end font-medium">First response</th>
            <th className="px-4 py-2 text-end font-medium">CSAT</th>
          </tr>
        </thead>
        <tbody>
          {teammates.map((t) => (
            <tr key={t.name} className="border-t border-separator-border">
              <td className="px-4 py-2.5 text-text-primary">{t.name}</td>
              <td className="px-2 py-2.5 text-end text-text-secondary tabular-nums">{t.replied}</td>
              <td className="px-2 py-2.5 text-end text-text-secondary tabular-nums">{t.first}</td>
              <td className="px-4 py-2.5 text-end text-text-secondary tabular-nums">{t.csat}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  );
}

const behind = [
  { number: 4821, who: "Omar Haddad", subject: "Stripe sync stopped overnight", value: "2h 41m" },
  { number: 4817, who: "Priya", subject: "Can't invite a second admin", value: "1h 58m" },
  { number: 4809, who: "Jonas Weber", subject: "Invoice shows the old address", value: "1h 12m" },
  { number: 4802, who: "Ana Lima", subject: "SSO login loops back", value: "54m" },
  { number: 4797, who: "Chen Wei", subject: "Webhook retries", value: "47m" },
];

/* The drawer that lists the conversations behind a number. */
export function DrillFragment() {
  return (
    <Card className="max-w-[460px] rounded-b-none border-b-0 text-start">
      <div className="flex items-center justify-between gap-3 border-b border-separator-border px-4 py-3.5">
        <span className="flex flex-col">
          <span className="text-headline-semibold text-text-primary">First response</span>
          <span className="text-body-2-regular text-text-secondary">312 conversations · slowest first</span>
        </span>
        <RiCloseLine className="size-5 text-foreground-icon-tertiary" aria-hidden />
      </div>
      <ul className="flex flex-col">
        {behind.map((row) => (
          <li
            key={row.number}
            className="flex items-center gap-3 border-b border-separator-border px-4 py-3 last:border-b-0"
          >
            <span className="w-11 shrink-0 text-caption-1-medium text-text-tertiary">#{row.number}</span>
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-body-2-medium text-text-primary">{row.subject}</span>
              <span className="truncate text-caption-1-regular text-text-tertiary">{row.who}</span>
            </span>
            <span className="shrink-0 text-body-2-medium text-text-primary tabular-nums">{row.value}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

const digestLines = [
  { label: "New conversations", value: "312", change: "+6%" },
  { label: "First response", value: "4m", change: "-18%" },
  { label: "Resolution time", value: "3h 4m", change: "-9%" },
  { label: "CSAT", value: "94%", change: "+2%" },
];

/* The email digest: how often, and the mail it sends. */
export function DigestFragment() {
  return (
    <div className="flex w-full max-w-[440px] flex-col gap-4 text-start">
      <Card className="p-4">
        <span className="flex flex-col gap-0.5">
          <span className="text-body-medium text-text-primary">Email digest</span>
          <span className="text-body-2-regular text-text-secondary">The main numbers of your reports, by mail.</span>
        </span>
        <span className="mt-3 grid grid-cols-3 rounded-lg bg-background-secondary-default p-0.5 text-center text-body-2-medium">
          <span className="py-1.5 text-text-tertiary">Off</span>
          <span className="py-1.5 text-text-tertiary">Daily</span>
          <span className="rounded-md bg-background-full py-1.5 text-text-primary shadow-xs">Weekly</span>
        </span>
      </Card>
      <Card className="rounded-b-none border-b-0">
        <div className="flex flex-col gap-0.5 border-b border-separator-border px-4 py-3.5">
          <span className="text-body-medium text-text-primary">Your week in support</span>
          <span className="text-caption-1-regular text-text-tertiary">Monday, 8:00 · against the week before</span>
        </div>
        <ul className="flex flex-col px-4 py-2">
          {digestLines.map((line) => (
            <li key={line.label} className="flex items-center gap-3 py-2">
              <span className="flex-1 text-body-2-regular text-text-secondary">{line.label}</span>
              <span className="text-body-2-medium text-text-primary tabular-nums">{line.value}</span>
              <span className="w-12 text-end text-caption-1-medium text-text-tertiary tabular-nums">{line.change}</span>
            </li>
          ))}
        </ul>
      </Card>
    </div>
  );
}
