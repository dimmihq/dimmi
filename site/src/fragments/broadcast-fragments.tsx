/*
 * Product fragments for the broadcasts page: the report, the composer's
 * steps, the channel picker, a reply in the inbox and a series on its
 * canvas, drawn with the app's own surfaces, chips and labels
 * (resources/js/features/broadcasts, series and audiences).
 */
import {
  RiChat3Line,
  RiDoorOpenLine,
  RiFileTextLine,
  RiHourglassLine,
  RiLoginCircleLine,
  RiMailLine,
  RiSlackLine,
} from "@remixicon/react";
import type { ComponentType, ReactNode } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { CHIP_COLORS, Chip, type ChipColor } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import { Agent, Customer } from "../scenes/agent-pieces";

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

function Head({ title, aside }: { title: string; aside?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-separator-border px-4 py-3">
      <span className="truncate text-body-medium text-text-primary">{title}</span>
      {aside}
    </div>
  );
}

function Mark({ icon: Icon, tint }: { icon: ComponentType<{ className?: string }>; tint: ChipColor }) {
  return (
    <span className={cx("flex size-6 shrink-0 items-center justify-center rounded-md", CHIP_COLORS[tint])}>
      <Icon className="size-4" />
    </span>
  );
}

function Toggle({ on }: { on: boolean }) {
  return (
    <span
      className={cx(
        "flex h-[18px] w-8 shrink-0 items-center rounded-full p-0.5",
        on ? "justify-end bg-accent-600" : "justify-start bg-border-button-hover",
      )}
    >
      <span className="size-3.5 rounded-full bg-white shadow-xs" />
    </span>
  );
}

/* The report's headline numbers, as the broadcast page's stat row. */
const stats = [
  { label: "Recipients", value: "2,418", caption: "Pro customers" },
  { label: "Delivered", value: "98.6%", caption: "2,384 people" },
  { label: "Opened", value: "58.2%", caption: "Estimated" },
  { label: "Clicked", value: "14.9%", caption: "Of delivered" },
  { label: "Replied", value: "3.4%", caption: "Landed in the inbox" },
];

/* Sent to goal, as the report's funnel: label, note, count and share. */
const funnel = [
  { label: "Sent", note: "Left dimmi", count: "2,418", share: 100 },
  { label: "Delivered", note: "Accepted by their mail server", count: "2,384", share: 98.6 },
  { label: "Opened", note: "Estimated", count: "1,388", share: 57.4 },
  { label: "Clicked", note: "Clicked a link in it", count: "355", share: 14.7 },
  { label: "Reached the goal", note: "Ran an export within 7 days", count: "212", share: 8.8 },
  { label: "Replied", note: "Each reply is a conversation in the inbox", count: "81", share: 3.4 },
];

function Funnel({ rows }: { rows: typeof funnel }) {
  return (
    <div className="flex flex-col gap-3.5">
      {rows.map((row) => (
        <div key={row.label} className="flex flex-col gap-1.5">
          <div className="flex items-baseline justify-between gap-4">
            <span className="flex min-w-0 flex-col">
              <span className="text-body-regular text-text-primary">{row.label}</span>
              <span className="truncate text-body-2-regular text-text-tertiary">{row.note}</span>
            </span>
            <span className="shrink-0 text-body-2-medium text-text-primary tabular-nums">
              {row.count}
              <span className="ms-2 text-text-tertiary">{row.share}%</span>
            </span>
          </div>
          <span className="h-1.5 overflow-hidden rounded-full bg-background-tertiary-default">
            <span className="block h-full rounded-full bg-accent-500" style={{ width: `${row.share}%` }} />
          </span>
        </div>
      ))}
    </div>
  );
}

const replies = [
  { name: "Omar Haddad", initials: "OH", text: "Can the export run every Monday on its own?", at: "4m" },
  { name: "Priya Moreau", initials: "PM", text: "This is great. Does it include archived projects?", at: "18m" },
  { name: "Daniel Okafor", initials: "DO", text: "We need it as JSON too, for our warehouse.", at: "1h" },
  { name: "Sofia Lindqvist", initials: "SL", text: "Thanks! Where do I find it on the Team plan?", at: "2h" },
];

/* The hero: a sent email broadcast's report, with replies arriving beside it. */
export function BroadcastReportFragment() {
  return (
    <Card className="max-w-[1000px] rounded-b-none border-b-0 text-start">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-separator-border px-5 py-4">
        <span className="text-headline-semibold text-text-primary">Bulk export is here</span>
        <Chip color="lime" variant="caption">
          Sent
        </Chip>
        <span className="inline-flex items-center gap-1.5 text-body-2-regular text-text-tertiary">
          <RiMailLine className="size-4" />
          Email from support@lumen.so
        </span>
        <span className="ms-auto text-body-2-regular text-text-tertiary max-sm:hidden">Sent 12 Nov, 10:00</span>
      </div>
      <dl className="grid grid-cols-2 border-b border-separator-border sm:grid-cols-5">
        {stats.map((stat, i) => (
          <div
            key={stat.label}
            className={cx("flex flex-col gap-1 px-5 py-4", i > 0 && "sm:border-s sm:border-separator-border")}
          >
            <dt className="text-body-2-regular text-text-secondary">{stat.label}</dt>
            <dd className="text-[22px] leading-7 font-medium tracking-[-0.01em] text-text-primary tabular-nums">
              {stat.value}
            </dd>
            <dd className="text-caption-1-regular text-text-tertiary">{stat.caption}</dd>
          </div>
        ))}
      </dl>
      <div className="grid grid-cols-1 md:grid-cols-[1.15fr_1fr]">
        <div className="flex flex-col gap-4 px-5 py-5 md:border-e md:border-separator-border">
          <span className="text-body-medium text-text-primary">From sent to the goal</span>
          <Funnel rows={funnel.slice(0, 5)} />
        </div>
        <div className="flex flex-col gap-2 px-3 py-5 max-md:hidden">
          <span className="flex items-center justify-between px-2 text-body-medium text-text-primary">
            Replies
            <span className="text-caption-1-regular text-text-tertiary">Each one is a conversation</span>
          </span>
          <ul className="flex flex-col">
            {replies.map((reply) => (
              <li key={reply.name} className="flex items-start gap-3 rounded-lg px-2 py-2.5">
                <Avatar size="sm" initials={reply.initials} color="neutral" />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="flex items-center justify-between gap-2">
                    <span className="truncate text-body-2-medium text-text-primary">{reply.name}</span>
                    <span className="shrink-0 text-caption-1-regular text-text-tertiary">{reply.at}</span>
                  </span>
                  <span className="truncate text-body-2-regular text-text-secondary">{reply.text}</span>
                </span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Card>
  );
}

/* The audience step: a saved audience's rule and its count. */
export function AudienceFragment() {
  const rule = [
    { join: "Where", field: "Plan", op: "is", value: "Pro" },
    { join: "and", field: "Last seen", op: "less than", value: "30 days ago" },
    { join: "and", field: "Tag", op: "is not", value: "Churn risk" },
  ];

  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <Head title="Active Pro customers" />
      <ul className="flex flex-col gap-2 px-3 py-3">
        {rule.map((row) => (
          <li key={row.field} className="flex items-center gap-1.5 text-caption-1-medium">
            <span className="w-10 shrink-0 text-end text-text-tertiary">{row.join}</span>
            <span className="rounded-md bg-background-secondary-default px-1.5 py-1 text-text-primary">
              {row.field}
            </span>
            <span className="text-text-tertiary">{row.op}</span>
            <span className="truncate rounded-md bg-background-secondary-default px-1.5 py-1 text-text-primary">
              {row.value}
            </span>
          </li>
        ))}
      </ul>
      <div className="flex items-center justify-between border-t border-separator-border px-4 py-3">
        <span className="text-body-2-regular text-text-secondary">Matches now</span>
        <span className="text-body-2-medium text-text-primary tabular-nums">1,284 people</span>
      </div>
    </Card>
  );
}

/* The write step: a subject and a body with a variable chip. */
export function WriteFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <div className="border-b border-separator-border px-4 py-3 text-body-medium text-text-primary">
        Bulk export is here
      </div>
      <div className="flex flex-col gap-2.5 px-4 py-3.5 text-body-regular text-text-primary">
        <p>
          Hi{" "}
          <span className="rounded-md bg-status-blue-background px-1.5 py-0.5 text-body-2-medium text-status-blue-text">
            First name
          </span>
          ,
        </p>
        <p className="text-text-secondary">
          You can now export every project at once, as CSV or JSON. Open Settings, then Exports.
        </p>
        <p className="text-text-secondary">Maya, from Lumen</p>
      </div>
    </Card>
  );
}

/* The delivery step: who it comes from and where replies go. */
export function DeliveryFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <Head title="From" />
      <dl className="flex flex-col gap-3 px-4 py-3.5 text-body-2-regular">
        <div className="flex items-center justify-between gap-3">
          <dt className="text-text-secondary">Sent as</dt>
          <dd className="inline-flex items-center gap-1.5 text-text-primary">
            <Avatar size="xs" initials="M" color="lime" />
            Maya
          </dd>
        </div>
        <div className="flex items-center justify-between gap-3">
          <dt className="text-text-secondary">Send from</dt>
          <dd className="text-text-primary">support@lumen.so</dd>
        </div>
        <div className="flex items-center justify-between gap-3">
          <dt className="text-text-secondary">Replies go to</dt>
          <dd className="text-text-primary">Success team</dd>
        </div>
        <div className="flex items-center justify-between gap-3">
          <dt className="text-text-secondary">The agent answers replies</dt>
          <dd>
            <Toggle on />
          </dd>
        </div>
      </dl>
    </Card>
  );
}

const channelPicks = [
  { icon: RiMailLine, name: "Email", where: "Lands in their inbox, replies come back here.", tint: "blue" as const },
  { icon: RiChat3Line, name: "Chat", where: "A message from a teammate in the messenger.", tint: "lime" as const },
  { icon: RiFileTextLine, name: "Post", where: "An announcement card in the messenger.", tint: "purple" as const },
  {
    icon: RiSlackLine,
    name: "Slack",
    where: "A post in each customer's shared Slack channel.",
    tint: "orange" as const,
  },
];

/* The new broadcast dialog: where it should reach people. */
export function ChannelFragment() {
  return (
    <Card className="max-w-[420px] rounded-b-none border-b-0">
      <div className="flex flex-col gap-1 border-b border-separator-border px-5 py-4">
        <span className="text-headline-semibold text-text-primary">New broadcast</span>
        <span className="text-body-regular text-text-secondary">Where should it reach people?</span>
      </div>
      <ul className="flex flex-col gap-2 px-5 py-5">
        {channelPicks.map((pick, i) => (
          <li
            key={pick.name}
            className={cx(
              "flex items-center gap-3 rounded-lg border px-3 py-2.5",
              i === 0 ? "border-accent-500 bg-accent-500/6" : "border-border-button-default",
            )}
          >
            <Mark icon={pick.icon} tint={pick.tint} />
            <span className="flex min-w-0 flex-col">
              <span className="text-body-medium text-text-primary">{pick.name}</span>
              <span className="truncate text-caption-1-regular text-text-tertiary">{pick.where}</span>
            </span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

/* A reply to a broadcast, open in the inbox, answered by the agent. */
export function ReplyFragment() {
  return (
    <Card className="max-w-[420px] rounded-b-none border-b-0">
      <div className="flex items-center gap-3 border-b border-separator-border px-5 py-3.5">
        <Avatar size="sm" initials="OH" color="neutral" />
        <span className="flex min-w-0 flex-col">
          <span className="text-body-medium text-text-primary">Omar Haddad</span>
          <span className="truncate text-caption-1-regular text-text-tertiary">Re: Bulk export is here</span>
        </span>
        <Chip color="blue" variant="caption" className="ms-auto">
          Email
        </Chip>
      </div>
      <div className="flex flex-col gap-4 px-5 py-5">
        <Customer name="Omar Haddad">Can the export run every Monday on its own?</Customer>
        <Agent>Yes. In Settings, then Exports, pick Weekly and Monday. It lands in your inbox at 9:00.</Agent>
      </div>
    </Card>
  );
}

/* The report's funnel, on its own. */
export function FunnelFragment() {
  return (
    <Card className="max-w-[420px] rounded-b-none border-b-0">
      <Head title="From sent to the goal" />
      <div className="px-5 py-5">
        <Funnel rows={funnel} />
      </div>
    </Card>
  );
}

/*
 * A series on its canvas, top to bottom: who enters, a welcome email, a
 * wait for an event, then two paths. The page's numbered steps light the
 * part in play; nothing moves, so nothing shifts.
 */
function Node({
  icon,
  tint,
  name,
  lit,
  className,
  children,
}: {
  icon: ComponentType<{ className?: string }>;
  tint: ChipColor;
  name: string;
  lit: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={cx(
        "flex flex-col gap-2 rounded-2xl bg-background-primary-default p-3 text-start transition-[box-shadow,opacity] duration-500",
        lit ? "shadow-[0_16px_40px_-24px_rgb(0_0_0/0.3)] ring-2 ring-accent-500" : "opacity-55 shadow-xs",
        className,
      )}
    >
      <span className="flex items-center gap-2">
        <Mark icon={icon} tint={tint} />
        <span className="text-body-2-medium text-text-secondary">{name}</span>
      </span>
      <span className="text-body-regular text-text-primary">{children}</span>
    </div>
  );
}

function Line({ label }: { label?: string }) {
  return (
    <span className="flex h-8 flex-col items-center">
      <span className="w-px flex-1 bg-border-button-hover" />
      {label ? (
        <span className="rounded-full bg-background-tertiary-default px-2 py-0.5 text-caption-1-medium text-text-secondary">
          {label}
        </span>
      ) : null}
    </span>
  );
}

export function SeriesStack({ step }: { step: number }) {
  return (
    <div className="flex flex-col items-center" aria-hidden>
      <Node icon={RiLoginCircleLine} tint="lime" name="Who enters" lit={step === 0} className="w-[264px]">
        Does trial_started
      </Node>
      <Line />
      <Node icon={RiMailLine} tint="blue" name="Email" lit={step === 1} className="w-[264px]">
        Welcome to Lumen
      </Node>
      <Line />
      <Node icon={RiHourglassLine} tint="purple" name="Wait for" lit={step === 1} className="w-[264px]">
        Wait up to 3 days for dashboard_created
      </Node>
      <div className="mt-0 grid w-[400px] grid-cols-2 gap-4 max-sm:w-[300px]">
        <div className="flex flex-col items-center">
          <Line label="It happened" />
          <Node icon={RiDoorOpenLine} tint="gray" name="Leave" lit={step === 2} className="w-full">
            Reached the goal
          </Node>
        </div>
        <div className="flex flex-col items-center">
          <Line label="Time's up" />
          <Node icon={RiChat3Line} tint="blue" name="Chat" lit={step === 2} className="w-full">
            Need a hand with your first dashboard?
          </Node>
        </div>
      </div>
    </div>
  );
}
