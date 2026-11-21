/*
 * Product fragments for the tickets page: the ticket list beside a
 * ticket's panel, the convert and escalate dialogs, the strip customers
 * read in the messenger, a type's states, an SLA policy, a tracker's
 * message to everyone and the engineering work behind a ticket. Each is
 * drawn with the app's own surfaces, labels and pill colours
 * (resources/js/features/tickets) so it reads as the product.
 */
import {
  type RemixiconComponentType,
  RiAlarmLine,
  RiBriefcase4Line,
  RiBugLine,
  RiCheckLine,
  RiExternalLinkLine,
  RiQuestionLine,
  RiQuillPenLine,
  RiRefund2Line,
  RiRouteLine,
  RiSearchLine,
  RiSparkling2Line,
} from "@remixicon/react";
import type { ReactNode } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { ToolMark, ToolTile } from "@/features/tickets/issues/tool-logos";
import { cx } from "@/utils/cx";

type Group = "submitted" | "in_progress" | "waiting_customer" | "waiting_team" | "resolved";

/* The app's pill colour per state group (features/tickets/labels.ts). */
const groupColor: Record<Group, ChipColor> = {
  submitted: "neutral",
  in_progress: "blue",
  waiting_customer: "yellow",
  waiting_team: "purple",
  resolved: "lime",
};

const groupName: Record<Group, string> = {
  submitted: "Submitted",
  in_progress: "In progress",
  waiting_customer: "Waiting on customer",
  waiting_team: "Waiting on team",
  resolved: "Resolved",
};

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
      <span className="text-body-medium text-text-primary">{title}</span>
      {aside}
    </div>
  );
}

function Pill({ group, children }: { group: Group; children: ReactNode }) {
  return (
    <Chip variant="caption" color={groupColor[group]} className="shrink-0">
      {children}
    </Chip>
  );
}

function Initials({ name }: { name: string }) {
  const short = name
    .split(" ")
    .map((w) => w[0])
    .join("")
    .slice(0, 2);
  return (
    <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-background-tertiary-default text-[10px] leading-none font-medium text-text-secondary">
      {short}
    </span>
  );
}

/* A label over its content, as the ticket panel's blocks. */
function Block({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5 border-t border-separator-border pt-3">
      <span className="text-caption-1-medium text-text-tertiary">{label}</span>
      {children}
    </div>
  );
}

/* The AI's mark beside a value it filled. */
function ByAI() {
  return (
    <span className="inline-flex items-center gap-1 text-caption-1-medium text-accent-700 dark:text-accent-400">
      <RiSparkling2Line className="size-3.5" aria-hidden />
      AI
    </span>
  );
}

type Row = {
  icon: RemixiconComponentType;
  reference: string;
  title: string;
  state: string;
  group: Group;
  type: string;
  who: string;
  ago: string;
};

const rows: Row[] = [
  {
    icon: RiBugLine,
    reference: "T-482",
    title: "CSV export drops the last row",
    state: "Investigating",
    group: "in_progress",
    type: "Bug report",
    who: "Kofi Mensah",
    ago: "4m",
  },
  {
    icon: RiRefund2Line,
    reference: "T-481",
    title: "Refund a duplicate charge",
    state: "With billing",
    group: "waiting_team",
    type: "Refund",
    who: "Maya",
    ago: "18m",
  },
  {
    icon: RiQuestionLine,
    reference: "T-479",
    title: "SAML setup for the Acme workspace",
    state: "Need more info",
    group: "waiting_customer",
    type: "Question",
    who: "Lena Fischer",
    ago: "1h",
  },
  {
    icon: RiRouteLine,
    reference: "T-476",
    title: "Webhook deliveries delayed in EU",
    state: "Being worked on",
    group: "in_progress",
    type: "Tracker",
    who: "Kofi Mensah",
    ago: "2h",
  },
  {
    icon: RiBriefcase4Line,
    reference: "T-474",
    title: "Raise the seat limit for Northwind",
    state: "Submitted",
    group: "submitted",
    type: "Account change",
    who: "Maya",
    ago: "3h",
  },
  {
    icon: RiQuestionLine,
    reference: "T-471",
    title: "Change the invoice address",
    state: "Resolved",
    group: "resolved",
    type: "Question",
    who: "Lena Fischer",
    ago: "5h",
  },
];

function TicketRow({ row, open }: { row: Row; open: boolean }) {
  return (
    <li className={cx("flex flex-col gap-1 rounded-xl px-2.5 py-2.5", open && "bg-background-secondary-default")}>
      <span className="flex items-center gap-2">
        <row.icon className="size-4 shrink-0 text-foreground-icon-secondary" aria-hidden />
        <span className="shrink-0 text-caption-1-medium text-text-tertiary">{row.reference}</span>
        <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{row.title}</span>
        <span className="shrink-0 text-caption-1-medium text-text-tertiary">{row.ago}</span>
      </span>
      <span className="flex items-center gap-2 ps-6">
        <Pill group={row.group}>{row.state}</Pill>
        <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-tertiary">{row.type}</span>
        <Initials name={row.who} />
      </span>
    </li>
  );
}

/* The hero: the ticket list beside the open ticket's panel. */
export function TicketsHeroFragment() {
  return (
    <Card className="flex h-[460px] max-w-[920px] overflow-hidden rounded-b-none border-b-0">
      <div className="flex w-full flex-col md:w-[380px] md:shrink-0 md:border-e md:border-separator-border">
        <div className="flex items-center gap-2 px-4 pt-4 pb-3">
          <span className="text-headline-semibold text-text-primary">All tickets</span>
          <span className="text-body-regular text-text-tertiary">24</span>
        </div>
        <div className="px-3 pb-2">
          <div className="flex h-9 items-center gap-2 rounded-lg border border-border-button-default px-2.5 text-body-regular text-text-tertiary">
            <RiSearchLine className="size-4 text-foreground-icon-tertiary" aria-hidden />
            Search tickets or T-482
          </div>
        </div>
        <ul className="flex flex-col gap-1 px-[7px]">
          {rows.map((row, i) => (
            <TicketRow key={row.reference} row={row} open={i === 0} />
          ))}
        </ul>
      </div>
      <div className="hidden min-w-0 flex-1 flex-col gap-3 px-5 py-4 text-start md:flex">
        <div className="flex flex-col gap-1">
          <span className="text-caption-1-medium text-text-tertiary">T-482</span>
          <span className="text-headline-semibold text-text-primary">CSV export drops the last row</span>
        </div>
        <div className="flex items-center gap-2">
          <RiBugLine className="size-4 text-foreground-icon-secondary" aria-hidden />
          <span className="text-body-medium text-text-primary">Bug report</span>
        </div>
        <Block label="State">
          <span className="flex items-center gap-2">
            <Pill group="in_progress">Investigating</Pill>
            <span className="truncate text-body-2-regular text-text-tertiary">Customer sees “We're on it”</span>
          </span>
        </Block>
        <Block label="Fields">
          <dl className="grid grid-cols-[96px_1fr] gap-x-3 gap-y-1.5 text-body-2-regular">
            <dt className="text-text-tertiary">Area</dt>
            <dd className="flex items-center justify-between text-text-primary">
              Exports <ByAI />
            </dd>
            <dt className="text-text-tertiary">Plan</dt>
            <dd className="text-text-primary">Pro</dd>
            <dt className="text-text-tertiary">Steps</dt>
            <dd className="flex items-center justify-between gap-2 text-text-primary">
              <span className="truncate">Export 500 rows to CSV</span> <ByAI />
            </dd>
          </dl>
        </Block>
        <Block label="Issues">
          <span className="flex items-center gap-2 text-body-2-regular">
            <ToolMark provider="linear" />
            <span className="text-body-2-medium text-text-primary">ENG-412</span>
            <span className="min-w-0 flex-1 truncate text-text-secondary">Export truncates final row</span>
            <Chip variant="caption" color="blue">
              In Progress
            </Chip>
          </span>
        </Block>
        <Block label="Service level">
          <span className="flex items-center gap-1.5 text-body-2-medium text-text-primary">
            <RiAlarmLine className="size-4" aria-hidden />
            Next reply due in 1h 20m
          </span>
        </Block>
      </div>
    </Card>
  );
}

/* Step one: the convert dialog, its fields filled by the AI. */
export function ConvertFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <Head title="Convert to ticket" aside={<span className="text-caption-1-regular text-text-tertiary">T</span>} />
      <div className="flex flex-col gap-2 px-4 py-3.5">
        <div className="flex items-center gap-2.5 rounded-lg border border-accent-500 bg-accent-500/6 px-3 py-2">
          <RiBugLine className="size-4 text-foreground-icon-secondary" aria-hidden />
          <span className="text-body-medium text-text-primary">Bug report</span>
          <span className="ms-auto flex size-4 items-center justify-center rounded-full border border-accent-600">
            <span className="size-2 rounded-full bg-accent-600" />
          </span>
        </div>
        {[
          ["Area", "Exports"],
          ["Plan", "Pro"],
        ].map(([label, value]) => (
          <div key={label} className="flex items-center gap-2 px-1 text-body-2-regular">
            <span className="w-12 text-text-tertiary">{label}</span>
            <span className="flex-1 text-text-primary">{value}</span>
            <ByAI />
          </div>
        ))}
      </div>
    </Card>
  );
}

/* Step two: escalating to a back-office team. */
export function EscalateFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <Head title="Escalate T-481" />
      <div className="flex flex-col gap-2.5 px-4 py-3.5">
        <div className="flex items-center gap-2.5 rounded-lg border border-accent-500 bg-accent-500/6 px-3 py-2">
          <RiBriefcase4Line className="size-4 text-foreground-icon-secondary" aria-hidden />
          <span className="text-body-medium text-text-primary">Refund approval</span>
        </div>
        <div className="flex items-center justify-between px-1 text-body-2-regular">
          <span className="text-text-tertiary">Team</span>
          <span className="text-text-primary">Billing</span>
        </div>
        <div className="flex items-center gap-2 border-t border-separator-border px-1 pt-2.5">
          <span className="text-caption-1-regular text-text-tertiary">Customer ticket moves to</span>
          <Pill group="waiting_team">With billing</Pill>
        </div>
      </div>
    </Card>
  );
}

/* Step three: the strip customers read above their request. */
export function CustomerStripFragment() {
  const stops = ["Submitted", "We're on it", "Resolved"];
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0 px-4 pt-4 pb-5">
      <p className="text-[13px] leading-5 text-text-primary">
        <b className="font-semibold">T-482</b> CSV export drops the last row
      </p>
      <ol className="mt-4 grid grid-cols-3">
        {stops.map((stop, i) => (
          <li key={stop} className="flex flex-col gap-2">
            <span className="flex items-center">
              <span
                className={cx(
                  "flex size-4 shrink-0 items-center justify-center rounded-full",
                  i <= 1 ? "bg-accent-500 text-text-white" : "border border-border-button-hover",
                )}
              >
                {i <= 1 ? <RiCheckLine className="size-3" aria-hidden /> : null}
              </span>
              {i < stops.length - 1 ? (
                <span className={cx("h-px flex-1", i < 1 ? "bg-accent-500" : "bg-separator-border")} />
              ) : null}
            </span>
            <span className={cx("text-[12px] leading-4", i === 1 ? "text-text-primary" : "text-text-tertiary")}>
              {stop}
            </span>
          </li>
        ))}
      </ol>
      <p className="mt-3 text-[12px] leading-4 text-text-secondary">A fix is in review</p>
    </Card>
  );
}

const states: { group: Group; name: string; customer: string }[] = [
  { group: "submitted", name: "Submitted", customer: "Submitted" },
  { group: "in_progress", name: "Investigating", customer: "We're on it" },
  { group: "in_progress", name: "Fix in review", customer: "A fix is in review" },
  { group: "waiting_customer", name: "Need more info", customer: "Waiting on you" },
  { group: "waiting_team", name: "With engineering", customer: "We're on it" },
  { group: "resolved", name: "Fixed", customer: "Resolved" },
  { group: "resolved", name: "Won't fix", customer: "Closed" },
];

/* A customer type's states, each with the label customers read. */
export function StatesFragment() {
  return (
    <Card className="max-w-[460px] rounded-b-none border-b-0">
      <div className="flex flex-col gap-1 border-b border-separator-border px-5 py-4">
        <span className="flex items-center gap-2 text-headline-semibold text-text-primary">
          <RiBugLine className="size-4.5 text-foreground-icon-secondary" aria-hidden />
          Bug report
        </span>
        <span className="text-body-regular text-text-secondary">
          Customer ticket. They see its state and can reply.
        </span>
      </div>
      <ul className="flex flex-col px-3 py-3">
        {states.map((state) => (
          <li key={state.name} className="grid grid-cols-[132px_1fr] items-center gap-3 px-2 py-2">
            <span className="flex">
              <Pill group={state.group}>{groupName[state.group]}</Pill>
            </span>
            <span className="flex min-w-0 items-center justify-between gap-3">
              <span className="truncate text-body-medium text-text-primary">{state.name}</span>
              <span className="truncate text-body-2-regular text-text-tertiary">“{state.customer}”</span>
            </span>
          </li>
        ))}
      </ul>
    </Card>
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

const targets = [
  { name: "First reply", value: "30 minutes" },
  { name: "Next reply", value: "2 hours" },
  { name: "Resolution", value: "2 days" },
];

/* An SLA policy: its targets, its hours, and a clock running on a ticket. */
export function SlaFragment() {
  return (
    <Card className="max-w-[440px] rounded-b-none border-b-0">
      <div className="flex flex-col gap-1 border-b border-separator-border px-5 py-4">
        <span className="text-headline-semibold text-text-primary">Enterprise</span>
        <span className="text-body-regular text-text-secondary">Applies to urgent and high priority tickets</span>
      </div>
      <ul className="flex flex-col gap-1 px-5 py-4">
        {targets.map((target) => (
          <li key={target.name} className="flex items-center justify-between py-1.5">
            <span className="text-body-regular text-text-primary">{target.name}</span>
            <span className="rounded-md border border-border-button-default px-2 py-0.5 text-body-2-medium text-text-primary">
              {target.value}
            </span>
          </li>
        ))}
      </ul>
      <ul className="flex flex-col gap-1 border-t border-separator-border px-5 py-4">
        <li className="flex items-center justify-between py-1.5">
          <span className="text-body-regular text-text-primary">Business hours</span>
          <span className="text-body-2-regular text-text-secondary">Weekdays, 9 to 6</span>
        </li>
        <li className="flex items-center justify-between py-1.5">
          <span className="text-body-regular text-text-primary">Pause while waiting on the customer</span>
          <Toggle on />
        </li>
        <li className="flex items-center justify-between py-1.5">
          <span className="text-body-regular text-text-primary">Warn before a breach</span>
          <span className="text-body-2-regular text-text-secondary">15 minutes</span>
        </li>
      </ul>
      <div className="flex items-center gap-2 border-t border-separator-border px-5 py-3.5">
        <span className="text-caption-1-medium text-text-tertiary">T-479</span>
        <span className="flex items-center gap-1.5 text-body-2-medium text-text-error-primary">
          <RiAlarmLine className="size-4" aria-hidden />
          First reply due in 12m
        </span>
      </div>
    </Card>
  );
}

/* A tracker's message to everyone, at the check step. */
export function TrackerFragment() {
  const order = ["Write", "Check", "Sent"];
  return (
    <Card className="max-w-[440px] rounded-b-none border-b-0">
      <div className="flex flex-col gap-1 border-b border-separator-border px-5 py-4">
        <span className="flex items-center gap-2 text-caption-1-medium text-text-tertiary">
          <RiRouteLine className="size-4 text-foreground-icon-secondary" aria-hidden />
          T-476 · Tracker
        </span>
        <span className="text-headline-semibold text-text-primary">Webhook deliveries delayed in EU</span>
        <span className="text-body-2-regular text-text-secondary">38 customers · 21 companies</span>
      </div>
      <div className="flex flex-col gap-4 px-5 py-4">
        <span className="text-body-medium text-text-primary">Message everyone</span>
        <ol className="flex items-center gap-2">
          {order.map((s, i) => (
            <li key={s} className="flex items-center gap-2">
              <span
                className={cx(
                  "flex size-5 items-center justify-center rounded-full text-caption-1-medium",
                  i === 0 && "bg-accent-500 text-text-white",
                  i === 1 && "text-text-primary ring-2 ring-accent-500 ring-inset",
                  i === 2 && "bg-background-tertiary-default text-text-tertiary",
                )}
              >
                {i === 0 ? <RiCheckLine className="size-3.5" aria-hidden /> : i + 1}
              </span>
              <span className={cx("text-caption-1-medium", i === 1 ? "text-text-primary" : "text-text-tertiary")}>
                {s}
              </span>
              {i < order.length - 1 ? <span className="h-px w-6 bg-separator-border" /> : null}
            </li>
          ))}
        </ol>
        <p className="rounded-lg bg-background-secondary-default px-3 py-2.5 text-body-2-regular text-text-primary">
          Deliveries are back to normal. Anything queued during the delay has now been sent.
        </p>
        <div className="flex flex-col gap-1.5">
          <span className="text-body-medium text-text-primary">2 customers held back</span>
          {[
            ["Omar Haddad", "Wrote since it last changed"],
            ["Priya", "A teammate already told them"],
          ].map(([name, why]) => (
            <span key={name} className="flex items-center gap-2 text-body-2-regular">
              <Initials name={name} />
              <span className="text-text-primary">{name}</span>
              <span className="ms-auto text-text-tertiary">{why}</span>
            </span>
          ))}
        </div>
      </div>
      <div className="flex justify-end border-t border-separator-border px-5 py-3">
        <span className="rounded-lg bg-accent-600 px-3 py-1.5 text-body-2-medium text-text-white">
          Send to 36 customers
        </span>
      </div>
    </Card>
  );
}

const issues: { provider: string; tool: string; key: string; title: string; status: string; color: ChipColor }[] = [
  { provider: "linear", tool: "Linear", key: "ENG-412", title: "Export truncates final row", status: "Done", color: "lime" },
  { provider: "jira", tool: "Jira", key: "OPS-88", title: "EU webhook queue backlog", status: "In Progress", color: "blue" },
  { provider: "github", tool: "GitHub", key: "#1904", title: "Flush the last CSV chunk", status: "Closed", color: "lime" },
];

/* The engineering work behind a ticket, and the update it drafted. */
export function IssuesFragment() {
  return (
    <Card className="max-w-[460px] rounded-b-none border-b-0">
      <Head title="Issues" aside={<span className="text-caption-1-medium text-text-tertiary">3 linked</span>} />
      <ul className="flex flex-col">
        {issues.map((issue) => (
          <li key={issue.key} className="flex items-center gap-3 border-b border-separator-border px-4 py-3">
            <span title={issue.tool} className="flex shrink-0 items-center">
              <ToolTile provider={issue.provider} size="sm" />
              <span className="sr-only">{issue.tool}</span>
            </span>
            <span className="shrink-0 text-body-2-medium text-text-primary">{issue.key}</span>
            <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-secondary">{issue.title}</span>
            <RiExternalLinkLine className="size-3.5 shrink-0 text-foreground-icon-tertiary" aria-hidden />
            <Chip variant="caption" color={issue.color}>
              {issue.status}
            </Chip>
          </li>
        ))}
      </ul>
      <div className="flex flex-col gap-2.5 px-4 py-4">
        <div className="flex items-center gap-2">
          <RiQuillPenLine className="size-4 shrink-0 text-foreground-icon-secondary" aria-hidden />
          <span className="min-w-0 flex-1 truncate text-body-2-medium text-text-primary">
            The work behind it is done
          </span>
          <span className="text-caption-1-regular text-text-tertiary">Reaches 12</span>
        </div>
        <p className="rounded-lg border border-border-button-default px-3 py-2.5 text-body-2-regular text-text-primary">
          Good news: CSV exports now include every row. Try your export again and let us know.
        </p>
        <div className="flex justify-end gap-2">
          <span className="rounded-lg border border-border-button-default px-3 py-1.5 text-body-2-medium text-text-primary">
            Discard
          </span>
          <span className="rounded-lg bg-accent-600 px-3 py-1.5 text-body-2-medium text-text-white">Send</span>
        </div>
      </div>
    </Card>
  );
}
