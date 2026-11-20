/*
 * Product fragments for the feedback page: the portal's roadmap and vote
 * buttons (resources/views/helpcenter/feedback.templ), and the team's
 * side from resources/js/features/feedback: the request list, the link
 * dialog beside a conversation, voters and the status settings.
 */
import { RiArrowUpSLine, RiChat3Line, RiDraggable, RiMailLine, RiSearchLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
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

function Head({ title, aside }: { title: string; aside?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-separator-border px-4 py-3">
      <span className="truncate text-body-medium text-text-primary">{title}</span>
      {aside}
    </div>
  );
}

type Stage = "review" | "planned" | "progress" | "shipped" | "closed";

const stageName: Record<Stage, string> = {
  review: "Under consideration",
  planned: "Planned",
  progress: "In progress",
  shipped: "Shipped",
  closed: "Closed",
};

const stageTone: Record<Stage, string> = {
  review: "text-text-tertiary",
  planned: "text-status-blue-text",
  progress: "text-status-yellow-text",
  shipped: "text-status-lime-text",
  closed: "text-text-tertiary",
};

/*
 * A status as the portal draws it: a ring that fills as a request moves
 * along, open, dashed when planned, half full in progress, full once
 * shipped and crossed out when closed.
 */
function StageIcon({ stage }: { stage: Stage }) {
  return (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      aria-hidden
      className={cx("size-3.5 shrink-0", stageTone[stage])}
    >
      {stage === "planned" ? <circle cx="8" cy="8" r="6.2" strokeDasharray="2.4 2" /> : null}
      {stage === "progress" ? (
        <>
          <circle cx="8" cy="8" r="6.2" />
          <path d="M8 3.6a4.4 4.4 0 0 1 0 8.8Z" fill="currentColor" stroke="none" />
        </>
      ) : null}
      {stage === "shipped" ? (
        <>
          <circle cx="8" cy="8" r="6.2" fill="currentColor" />
          <path d="m5.4 8.2 1.8 1.8 3.4-3.6" className="stroke-white" />
        </>
      ) : null}
      {stage === "closed" ? (
        <>
          <circle cx="8" cy="8" r="6.2" />
          <path d="m5.8 5.8 4.4 4.4M10.2 5.8l-4.4 4.4" />
        </>
      ) : null}
      {stage === "review" ? <circle cx="8" cy="8" r="6.2" /> : null}
    </svg>
  );
}

/* A status chip, as the team's list draws it. */
function StageChip({ stage }: { stage: Stage }) {
  return (
    <span className="inline-flex h-6 shrink-0 items-center gap-1.5 rounded-full bg-background-secondary-default px-2.5 text-caption-1-medium text-text-secondary">
      <StageIcon stage={stage} />
      {stageName[stage]}
    </span>
  );
}

/* The portal's vote button: an arrow over the count, filled once voted. */
function Vote({ count, voted = false }: { count: number; voted?: boolean }) {
  return (
    <span
      className={cx(
        "flex h-12 w-11 shrink-0 flex-col items-center justify-center rounded-xl border text-caption-1-semibold tabular-nums",
        voted
          ? "border-transparent bg-accent-500/12 text-accent-700 dark:text-accent-400"
          : "border-border-button-default bg-background-primary-default text-text-secondary",
      )}
    >
      <RiArrowUpSLine className="size-4" />
      {count}
    </span>
  );
}

function Idea({ title, votes, comments, voted }: { title: string; votes: number; comments?: number; voted?: boolean }) {
  return (
    <li className="flex items-start gap-3 rounded-2xl border border-border-button-default bg-background-primary-default p-3">
      <Vote count={votes} voted={voted} />
      <span className="flex min-w-0 flex-1 flex-col gap-1.5 pt-0.5">
        <span className="line-clamp-2 text-body-2-medium text-text-primary">{title}</span>
        {comments ? (
          <span className="flex items-center gap-1 text-caption-1-medium text-text-tertiary tabular-nums">
            <RiChat3Line className="size-3.5" />
            {comments}
          </span>
        ) : null}
      </span>
    </li>
  );
}

const roadmap: {
  stage: Stage;
  count: number;
  ideas: { title: string; votes: number; comments?: number; voted?: boolean }[];
}[] = [
  {
    stage: "review",
    count: 31,
    ideas: [
      { title: "Dark mode for shared dashboards", votes: 64, comments: 12 },
      { title: "Two-factor login with a security key", votes: 29, comments: 4 },
    ],
  },
  {
    stage: "planned",
    count: 8,
    ideas: [
      { title: "Scheduled exports", votes: 48, comments: 9, voted: true },
      { title: "Comments on charts", votes: 37, comments: 6 },
    ],
  },
  {
    stage: "progress",
    count: 3,
    ideas: [
      { title: "SSO with Okta", votes: 52, comments: 14 },
      { title: "Custom date ranges", votes: 23, comments: 3 },
    ],
  },
  {
    stage: "shipped",
    count: 46,
    ideas: [
      { title: "Bulk export to CSV", votes: 91, comments: 21 },
      { title: "Slack alerts on goals", votes: 40, comments: 7 },
    ],
  },
];

/* The hero: the portal's roadmap, one column per status. */
export function RoadmapFragment() {
  return (
    <Card className="max-w-[1000px] rounded-b-none border-b-0 text-start">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-separator-border px-5 py-4">
        <span className="flex flex-col">
          <span className="text-headline-semibold text-text-primary">Feedback</span>
          <span className="text-body-2-regular text-text-tertiary">Tell us what would make Lumen better.</span>
        </span>
        <span className="flex items-center gap-1 rounded-xl bg-background-secondary-default p-1 text-body-2-medium">
          <span className="rounded-lg bg-background-primary-default px-3 py-1 text-text-primary shadow-xs">
            Roadmap
          </span>
          <span className="px-3 py-1 text-text-secondary">Trending</span>
          <span className="px-3 py-1 text-text-secondary max-sm:hidden">Top</span>
          <span className="px-3 py-1 text-text-secondary max-sm:hidden">New</span>
        </span>
      </div>
      <div className="grid grid-cols-1 gap-3 p-4 sm:grid-cols-2 lg:grid-cols-4">
        {roadmap.map((column, i) => (
          <section
            key={column.stage}
            className={cx(
              "flex flex-col rounded-3xl border border-border-button-default bg-background-secondary-default p-2",
              i > 1 && "max-sm:hidden",
            )}
          >
            <h3 className="flex items-center gap-2 px-2.5 pt-1.5 pb-2.5 text-body-2-semibold text-text-primary">
              <StageIcon stage={column.stage} />
              {stageName[column.stage]}
              <span className="ms-auto text-caption-1-medium text-text-tertiary tabular-nums">{column.count}</span>
            </h3>
            <ul className="flex flex-col gap-2">
              {column.ideas.map((idea) => (
                <Idea key={idea.title} {...idea} />
              ))}
            </ul>
          </section>
        ))}
      </div>
    </Card>
  );
}

/* Customers on the board: the trending list with one vote cast. */
export function VoteFragment() {
  return (
    <ul className="flex w-full max-w-[300px] flex-col gap-2 self-center">
      <Idea title="Dark mode for shared dashboards" votes={64} comments={12} />
      <Idea title="Scheduled exports" votes={48} comments={9} voted />
    </ul>
  );
}

const priorities = ["Nice to have", "Important", "Must have"];

/* Beside a conversation: linking it to a request, with how much it matters. */
export function LinkFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <Head title="Link to feature request" />
      <div className="flex flex-col gap-3 px-4 py-3.5">
        <span className="flex h-9 items-center gap-2 rounded-lg border border-border-button-default px-2.5 text-body-2-regular text-text-primary">
          <RiSearchLine className="size-4 text-foreground-icon-tertiary" />
          export
        </span>
        <span className="flex items-center justify-between gap-2 rounded-lg bg-background-secondary-default px-2.5 py-2">
          <span className="truncate text-body-2-medium text-text-primary">Scheduled exports</span>
          <span className="shrink-0 text-caption-1-regular text-text-tertiary tabular-nums">48 votes</span>
        </span>
        <span className="flex gap-1 rounded-lg bg-background-secondary-default p-1 text-caption-1-medium">
          {priorities.map((priority, i) => (
            <span
              key={priority}
              className={cx(
                "flex-1 rounded-md py-1 text-center whitespace-nowrap",
                i === 2 ? "bg-background-primary-default text-text-primary shadow-xs" : "text-text-secondary",
              )}
            >
              {priority}
            </span>
          ))}
        </span>
      </div>
    </Card>
  );
}

/* The email voters get when a request ships. */
export function ShippedFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <div className="flex items-center gap-2 border-b border-separator-border px-4 py-3">
        <RiMailLine className="size-4 text-foreground-icon-secondary" />
        <span className="truncate text-body-medium text-text-primary">Scheduled exports</span>
      </div>
      <div className="flex flex-col gap-2.5 px-4 py-3.5">
        <StageChip stage="shipped" />
        <p className="text-body-2-regular text-text-secondary">
          You asked for this one. Pick a day and a time in Settings, then Exports.
        </p>
        <span className="text-body-2-medium text-text-primary underline">Read the update</span>
      </div>
    </Card>
  );
}

const requests: { title: string; votes: number; stage: Stage; by: string }[] = [
  { title: "Bulk export to CSV", votes: 91, stage: "shipped", by: "Omar Haddad" },
  { title: "Dark mode for shared dashboards", votes: 64, stage: "review", by: "Priya Moreau" },
  { title: "SSO with Okta", votes: 52, stage: "progress", by: "Lena Fischer" },
  { title: "Scheduled exports", votes: 48, stage: "planned", by: "Omar Haddad" },
  { title: "Comments on charts", votes: 37, stage: "planned", by: "Daniel Okafor" },
];

/* The team's list of requests, most votes first. */
export function RequestsFragment() {
  return (
    <Card className="max-w-[440px] rounded-b-none border-b-0">
      <Head
        title="Feature requests"
        aside={<span className="text-body-2-regular text-text-tertiary">Most votes</span>}
      />
      <ul className="flex flex-col px-2 py-2">
        {requests.map((request) => (
          <li key={request.title} className="flex items-center gap-3 rounded-lg px-2 py-2.5">
            <span className="flex min-w-0 flex-1 flex-col gap-1">
              <span className="truncate text-body-medium text-text-primary">{request.title}</span>
              <span className="truncate text-caption-1-regular text-text-tertiary">Posted by {request.by}</span>
            </span>
            <StageChip stage={request.stage} />
            <span className="w-8 shrink-0 text-end text-body-2-medium text-text-primary tabular-nums">
              {request.votes}
            </span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

const voters = [
  { name: "Omar Haddad", initials: "OH", company: "Northwind", priority: "Must have", tint: "rose" as const },
  { name: "Priya Moreau", initials: "PM", company: "Acme", priority: "Important", tint: "yellow" as const },
  { name: "Daniel Okafor", initials: "DO", company: "Brightline", priority: "Nice to have", tint: "gray" as const },
];

const linked = [
  { number: "#4821", subject: "Can exports run every Monday?", who: "Omar Haddad" },
  { number: "#4790", subject: "Weekly report to our finance team", who: "Priya Moreau" },
];

/* One request's votes and the conversations it came up in. */
export function VotersFragment() {
  return (
    <Card className="max-w-[440px] rounded-b-none border-b-0">
      <div className="flex items-center justify-between gap-3 border-b border-separator-border px-4 py-3">
        <span className="text-body-medium text-text-primary">Scheduled exports</span>
        <StageChip stage="planned" />
      </div>
      <div className="flex flex-col gap-1 px-2 pt-3">
        <span className="px-2 text-caption-1-medium text-text-tertiary">48 votes</span>
        <ul className="flex flex-col">
          {voters.map((voter) => (
            <li key={voter.name} className="flex items-center gap-2.5 px-2 py-2">
              <Avatar size="sm" initials={voter.initials} color="neutral" />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate text-body-2-medium text-text-primary">{voter.name}</span>
                <span className="truncate text-caption-1-regular text-text-tertiary">{voter.company}</span>
              </span>
              <Chip color={voter.tint} variant="caption">
                {voter.priority}
              </Chip>
            </li>
          ))}
        </ul>
      </div>
      <div className="flex flex-col gap-1 border-t border-separator-border px-2 py-3">
        <span className="px-2 text-caption-1-medium text-text-tertiary">12 conversations</span>
        <ul className="flex flex-col">
          {linked.map((conversation) => (
            <li key={conversation.number} className="flex items-center gap-2.5 px-2 py-2">
              <RiChat3Line className="size-4 shrink-0 text-foreground-icon-secondary" />
              <span className="truncate text-body-2-regular text-text-primary">{conversation.subject}</span>
              <span className="ms-auto shrink-0 text-caption-1-regular text-text-tertiary">{conversation.number}</span>
            </li>
          ))}
        </ul>
      </div>
    </Card>
  );
}

const stages: { stage: Stage; category: string; roadmap: boolean }[] = [
  { stage: "review", category: "Open", roadmap: true },
  { stage: "planned", category: "Planned", roadmap: true },
  { stage: "progress", category: "In progress", roadmap: true },
  { stage: "shipped", category: "Done", roadmap: true },
  { stage: "closed", category: "Closed", roadmap: false },
];

/* The workspace's statuses, in roadmap order. */
export function StatusesFragment() {
  return (
    <Card className="max-w-[440px] rounded-b-none border-b-0">
      <div className="flex flex-col gap-1 border-b border-separator-border px-5 py-4">
        <span className="text-headline-semibold text-text-primary">Statuses</span>
        <span className="text-body-regular text-text-secondary">Drag to set the order of the roadmap.</span>
      </div>
      <ul className="flex flex-col px-3 py-3">
        {stages.map((row) => (
          <li key={row.stage} className="flex items-center gap-2.5 px-2 py-2.5">
            <RiDraggable className="size-4 shrink-0 text-foreground-icon-tertiary" />
            <StageIcon stage={row.stage} />
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-body-medium text-text-primary">{stageName[row.stage]}</span>
              <span className="truncate text-caption-1-regular text-text-tertiary">{row.category}</span>
            </span>
            <span className="text-caption-1-regular text-text-tertiary max-sm:hidden">
              {row.roadmap ? "On the roadmap" : "Hidden"}
            </span>
            <span
              className={cx(
                "flex h-[18px] w-8 shrink-0 items-center rounded-full p-0.5",
                row.roadmap ? "justify-end bg-accent-600" : "justify-start bg-border-button-hover",
              )}
            >
              <span className="size-3.5 rounded-full bg-white shadow-xs" />
            </span>
          </li>
        ))}
      </ul>
    </Card>
  );
}
