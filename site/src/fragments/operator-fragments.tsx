/*
 * Product fragments for the Operator page: the pieces of Operator's
 * thread beside a conversation (a brief, a reply draft, a change to
 * apply, a workspace question) and its settings, drawn after
 * resources/js/features/operator so they read as the product.
 */
import {
  RiArrowDownSLine,
  RiArrowGoBackLine,
  RiArrowUpLine,
  RiBarChartLine,
  RiChat3Line,
  RiCheckLine,
  RiCloseLine,
  RiFileCopyLine,
  RiGitPullRequestLine,
  RiGlobalLine,
  RiLockLine,
  RiReplyLine,
  RiSearchLine,
  RiSparkling2Line,
  RiUserLine,
} from "@remixicon/react";
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

/* A drawn button, not focusable: the fragments are pictures. */
function FakeButton({
  children,
  icon: Icon,
  primary,
}: {
  children: ReactNode;
  icon?: typeof RiCheckLine;
  primary?: boolean;
}) {
  return (
    <span
      className={cx(
        "inline-flex h-7 items-center gap-1 rounded-lg px-2.5 text-body-2-medium",
        primary
          ? "bg-linear-to-b from-accent-500 to-accent-600 text-white"
          : "border border-border-button-default bg-background-full text-text-primary shadow-xs",
      )}
    >
      {Icon ? <Icon className="size-3.5" aria-hidden /> : null}
      {children}
    </span>
  );
}

/* A teammate's question, as Operator's thread draws it. */
function Question({ children }: { children: ReactNode }) {
  return (
    <p className="ms-auto w-fit max-w-[85%] rounded-2xl bg-background-primary-default px-3 py-2 text-body-2-regular text-text-primary shadow-xs">
      {children}
    </p>
  );
}

/* The folded log over a finished answer. */
function Folded({ count }: { count: number }) {
  return (
    <span className="inline-flex items-center gap-1 text-body-2-regular text-text-tertiary">
      Worked through {count} steps
      <RiArrowDownSLine className="size-3.5" aria-hidden />
    </span>
  );
}

const briefLog = [
  { icon: RiChat3Line, text: "Read conversation #4821" },
  { icon: RiUserLine, text: "Looked up Omar Haddad" },
  { icon: RiSearchLine, text: "Found 2 past conversations" },
];

export function BriefFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0 bg-background-secondary-default">
      <div className="flex flex-col gap-2.5 px-4 pt-4 pb-5">
        <Question>Brief me</Question>
        <ul className="flex flex-col gap-1.5">
          {briefLog.map((row) => (
            <li key={row.text} className="flex items-center gap-1.5 text-body-2-regular text-text-secondary">
              <row.icon className="size-3.5 shrink-0 text-foreground-icon-tertiary" aria-hidden />
              {row.text}
            </li>
          ))}
        </ul>
        <div className="flex flex-col gap-1 text-body-2-regular text-text-primary">
          <span>Webhooks stopped after Omar rotated his signing secret on Monday.</span>
          <span className="text-text-secondary">Same issue in March; Kofi fixed it by re-saving the endpoint.</span>
        </div>
      </div>
    </Card>
  );
}

export function DraftFragment() {
  return (
    <Card className="max-w-[300px] self-end overflow-hidden rounded-b-none border-b-0">
      <div className="flex h-10 items-center gap-2 border-b border-separator-border ps-3 pe-2">
        <RiReplyLine className="size-4 shrink-0 text-foreground-icon-secondary" aria-hidden />
        <span className="flex-1 text-body-2-medium text-text-primary">Reply draft</span>
        <span className="text-caption-1-regular text-text-tertiary">Not sent</span>
        <RiFileCopyLine className="size-4 text-foreground-icon-tertiary" aria-hidden />
      </div>
      <p className="px-3 py-2.5 text-body-2-regular text-text-primary">
        Hi Omar, rotating the secret paused delivery. Open Settings, Webhooks and press Save on the endpoint, and events
        start again within a minute.
      </p>
      <div className="px-3 pb-4">
        <FakeButton icon={RiReplyLine}>Use in reply</FakeButton>
      </div>
    </Card>
  );
}

const operations = ["Tag #4821 with webhooks", "Assign #4821 to Kofi Mensah", "Set #4821 to high priority"];

export function ChangeFragment({ status = "pending" }: { status?: "pending" | "applied" }) {
  return (
    <Card className="max-w-[300px] self-end overflow-hidden rounded-b-none border-b-0">
      <div className="flex h-10 items-center gap-2 border-b border-separator-border ps-3 pe-2">
        <RiGitPullRequestLine className="size-4 shrink-0 text-foreground-icon-secondary" aria-hidden />
        <span className="flex-1 truncate text-body-2-medium text-text-primary">Route to engineering</span>
        <Chip variant="caption" color={status === "pending" ? "blue" : "lime"} className="shrink-0 py-0">
          {status === "pending" ? "Waiting for you" : "Applied"}
        </Chip>
      </div>
      <div className="flex flex-col gap-2 px-3 py-2.5">
        <p className="text-body-2-regular text-text-secondary">A webhook bug Kofi owns.</p>
        <ul className="flex flex-col gap-1.5">
          {operations.map((op) => (
            <li key={op} className="flex items-start gap-2 text-body-2-regular text-text-primary">
              <RiCheckLine
                className={cx(
                  "mt-0.5 size-3.5 shrink-0",
                  status === "applied" ? "text-foreground-icon-primary" : "text-foreground-icon-quaternary",
                )}
                aria-hidden
              />
              {op}
            </li>
          ))}
        </ul>
      </div>
      <div className="flex gap-2 px-3 pb-4">
        {status === "pending" ? (
          <>
            <FakeButton primary>Apply</FakeButton>
            <FakeButton icon={RiCloseLine}>Decline</FakeButton>
          </>
        ) : (
          <FakeButton icon={RiArrowGoBackLine}>Undo</FakeButton>
        )}
      </div>
    </Card>
  );
}

/*
 * Operator's panel with a question about the whole workspace: the header,
 * the question, the folded log, the answer, and the composer that names
 * the scope.
 */
export function WorkspaceFragment() {
  return (
    <div className="flex h-[420px] w-full max-w-[420px] flex-col overflow-hidden rounded-t-2xl border border-b-0 border-border-button-default bg-background-secondary-default shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]">
      <div className="flex items-center gap-2 border-b border-separator-border bg-background-full px-4 py-3">
        <RiSparkling2Line className="size-4 text-foreground-icon-secondary" aria-hidden />
        <span className="text-body-medium text-text-primary">Operator</span>
        <span className="rounded-md bg-background-secondary-default px-1.5 py-0.5 text-caption-1-medium text-text-tertiary">
          Internal
        </span>
      </div>
      <div className="flex flex-1 flex-col gap-3 overflow-hidden px-4 py-4">
        <Question>Why did billing questions jump this week?</Question>
        <span className="inline-flex items-center gap-1.5 text-body-2-regular text-text-tertiary">
          <RiBarChartLine className="size-3.5" aria-hidden />
          Pulled the report for the last 7 days
        </span>
        <Folded count={4} />
        <div className="flex flex-col gap-2 text-body-2-regular text-text-primary">
          <span>Billing conversations rose from 38 to 61. Most came after Tuesday's invoice run:</span>
          <ul className="flex list-disc flex-col gap-1 ps-5">
            <li>24 ask about a duplicate charge</li>
            <li>11 ask to change the billing email</li>
            <li>The agent resolved 19 of them on its own</li>
          </ul>
        </div>
      </div>
      <div className="flex flex-col gap-2 px-4 pb-3">
        <div className="flex items-center gap-2 rounded-full bg-background-full py-1.5 ps-4 pe-1.5 shadow-xs">
          <span className="flex-1 text-body-2-regular text-text-tertiary">Ask Operator</span>
          <span className="flex size-7 items-center justify-center rounded-full bg-accent-600 text-white">
            <RiArrowUpLine className="size-4" aria-hidden />
          </span>
        </div>
        <div className="flex items-center justify-between px-1 text-caption-1-medium text-text-secondary">
          <span className="inline-flex items-center gap-1">
            <RiLockLine className="size-3.5" aria-hidden />
            Only your team sees this
          </span>
          <span className="inline-flex items-center gap-1">
            <RiGlobalLine className="size-3.5" aria-hidden />
            Whole workspace
          </span>
        </div>
      </div>
    </div>
  );
}

const policies = [
  { name: "Conversations", detail: "Tags, assignment, priority, status", value: "On its own" },
  { name: "Internal notes", detail: "Notes for the team", value: "On its own" },
  { name: "Customers", detail: "A customer's name", value: "Ask first" },
  { name: "Help articles", detail: "Drafts only. Publishing stays yours.", value: "Ask first" },
  { name: "Procedures", detail: "Working copy only", value: "Ask first" },
  { name: "Workspace actions", detail: "Lookups run at once; changes wait", value: "Ask first" },
];

/* "What Operator may change", from Operator's settings. */
export function PolicyFragment() {
  return (
    <Card className="max-w-[440px] rounded-b-none border-b-0">
      <div className="flex flex-col gap-1 border-b border-separator-border px-5 py-4">
        <span className="text-headline-semibold text-text-primary">What Operator may change</span>
        <span className="text-body-2-regular text-text-secondary">
          Changes it makes on its own still show in the chat, and can be undone.
        </span>
      </div>
      <ul className="flex flex-col px-5 py-2">
        {policies.map((row) => (
          <li
            key={row.name}
            className="flex items-center justify-between gap-4 border-b border-separator-border py-3 last:border-b-0"
          >
            <span className="flex min-w-0 flex-col">
              <span className="text-body-medium text-text-primary">{row.name}</span>
              <span className="truncate text-caption-1-regular text-text-tertiary">{row.detail}</span>
            </span>
            <span className="inline-flex h-7 shrink-0 items-center gap-1 rounded-lg border border-border-button-default px-2.5 text-body-2-medium text-text-primary shadow-xs">
              {row.value}
              <RiArrowDownSLine className="size-3.5 text-foreground-icon-tertiary" aria-hidden />
            </span>
          </li>
        ))}
      </ul>
    </Card>
  );
}
