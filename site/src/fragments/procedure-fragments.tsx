/*
 * Product fragments for the Procedures page: the redesigned procedure
 * editor with @ chips, exact Rule branches and simulation pass counts,
 * a conversation where code checks enforce step order, and the approval
 * card a teammate decides in the inbox.
 */
import {
  RiArrowDownSLine,
  RiArrowRightLine,
  RiCheckboxCircleLine,
  RiCheckLine,
  RiCloseCircleLine,
  RiErrorWarningLine,
  RiFlashlightLine,
  RiShieldUserLine,
  RiSplitCellsHorizontal,
  RiTestTubeLine,
  RiTimeLine,
  RiUserReceivedLine,
  RiUserStarLine,
} from "@remixicon/react";
import type { ComponentType, ReactNode } from "react";

import { CHIP_COLORS, Chip, type ChipColor } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import { Agent, Customer, Procedure } from "../scenes/agent-pieces";

type Glyph = ComponentType<{ className?: string }>;

/* The tinted square mark of a step kind, as the editor draws it. */
function KindMark({ icon: Icon, tint }: { icon: Glyph; tint: ChipColor }) {
  return (
    <span className={cx("flex size-6 shrink-0 items-center justify-center rounded-md", CHIP_COLORS[tint])}>
      <Icon className="size-4" />
    </span>
  );
}

/* Inline @ chip pill as StepWriter renders it inside a step. */
function ActionChip({ label }: { label: string }) {
  return (
    <span className="mx-0.5 inline-flex items-center gap-1 rounded-md border border-border-button-default bg-background-full px-1.5 py-px align-baseline text-caption-1-medium text-text-primary">
      <RiFlashlightLine className="size-3 text-status-lime-text" aria-hidden />
      {label}
    </span>
  );
}

/* One part of the editor, lit when it is the one in play. */
function Part({ label, lit, children }: { label: string; lit: boolean; children: ReactNode }) {
  return (
    <div
      className={cx(
        "flex flex-col gap-2 rounded-xl border p-3 transition-[border-color,opacity,box-shadow] duration-500",
        lit
          ? "border-accent-500 bg-background-full opacity-100 shadow-[0_16px_40px_-24px_rgb(0_0_0/0.3)]"
          : "border-transparent opacity-50",
      )}
    >
      <span className="text-body-2-medium text-text-secondary">{label}</span>
      {children}
    </div>
  );
}

/*
 * The redesigned editor, one part lit per HowItWorks step:
 * 0 when it applies, 1 steps with @ chips, 2 exact code Rule branch,
 * 3 simulations passing and publishing.
 */
export function ProcedureEditorFragment({ step }: { step: number }) {
  const live = step === 3;
  return (
    <div
      className="flex h-[510px] w-[440px] max-w-full flex-col gap-1.5 overflow-hidden rounded-2xl border border-border-button-default bg-background-secondary-default p-2 shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]"
      aria-hidden
    >
      <div
        className={cx(
          "flex items-center gap-2 rounded-xl border px-3 py-2 transition-[border-color,opacity] duration-500",
          live ? "border-accent-500 bg-background-full" : "border-transparent",
        )}
      >
        <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">Refund a duplicate charge</span>
        <span className="inline-flex items-center gap-1 rounded-md bg-status-lime-bg px-2 py-0.5 text-caption-1-medium text-status-lime-text tabular-nums">
          <RiTestTubeLine className="size-3" aria-hidden />
          3/3
        </span>
        <span
          className={cx(
            "inline-flex h-7 items-center rounded-lg px-2.5 text-body-2-medium transition-colors duration-500",
            live
              ? "bg-background-tertiary-default text-text-tertiary"
              : "bg-linear-to-b from-accent-500 to-accent-600 text-white",
          )}
        >
          {live ? "Published" : "Publish"}
        </span>
      </div>
      <Part label="When dimmi uses this" lit={step === 0}>
        <p className="rounded-lg bg-background-tertiary-default px-2.5 py-2 text-body-2-regular text-text-primary">
          The customer was charged twice for the same period and asks for one back.
        </p>
      </Part>
      <Part label="Steps" lit={step === 1}>
        <div className="flex items-start gap-2">
          <span className="w-3 shrink-0 pt-0.5 text-end text-caption-1-medium text-text-tertiary tabular-nums">1</span>
          <KindMark icon={RiArrowRightLine} tint="neutral" />
          <p className="min-w-0 flex-1 rounded-md bg-background-tertiary-default px-2 py-1 text-body-2-regular leading-5 text-text-primary">
            Check the month they named with <ActionChip label="Look up charges" />.
          </p>
        </div>
      </Part>
      <Part label="Policy limit" lit={step === 2}>
        <div className="flex items-start gap-2">
          <span className="w-3 shrink-0 pt-0.5 text-end text-caption-1-medium text-text-tertiary tabular-nums">2</span>
          <KindMark icon={RiSplitCellsHorizontal} tint="purple" />
          <div className="flex min-w-0 flex-1 flex-col gap-1.5">
            <span className="rounded-md bg-background-tertiary-default px-2 py-1 text-caption-1-medium text-text-primary">
              Duplicate confirmed · Refund up to $500
            </span>
            <div className="flex flex-col gap-1 border-s-2 border-status-lime-text/60 ps-2.5">
              <span className="text-caption-1-medium text-status-lime-text">Then</span>
              <span className="text-body-2-regular text-text-primary">
                Refund with <ActionChip label="Refund a charge" /> and share the 5 to 10 day timeline.
              </span>
            </div>
            <div className="flex flex-col gap-1 border-s-2 border-border-error-default/60 ps-2.5">
              <span className="text-caption-1-medium text-text-error-primary">Else</span>
              <span className="text-body-2-regular text-text-secondary">
                Explain the charge or hand over to Billing.
              </span>
            </div>
          </div>
        </div>
      </Part>
      <div
        className={cx(
          "mt-auto flex items-center gap-2 px-3 py-1.5 transition-opacity duration-500",
          live ? "opacity-100" : "opacity-0",
        )}
      >
        <Chip variant="caption" color="lime" className="py-0">
          v3 · Live
        </Chip>
        <span className="text-body-2-regular text-text-secondary">3 of 3 test scenarios passed</span>
      </div>
    </div>
  );
}

/* A conversation where code checks enforce step order and exact rules. */
export function FollowFragment() {
  return (
    <div className="flex w-full max-w-[460px] flex-col overflow-hidden rounded-t-2xl border border-b-0 border-border-button-default bg-background-full shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]">
      <div className="flex items-center justify-between border-b border-separator-border px-5 py-3">
        <span className="text-body-medium text-text-primary">Conversation</span>
        <span className="rounded-md bg-background-secondary-default px-2 py-0.5 text-caption-1-medium text-text-secondary">
          Messenger
        </span>
      </div>
      <div className="flex h-[412px] flex-col gap-2.5 overflow-hidden px-5 py-3.5">
        <Customer name="Omar Haddad">I was charged twice for March. Can I get one back?</Customer>
        <div className="me-auto inline-flex items-center gap-1.5 rounded-lg border border-amber-300/70 bg-amber-50 px-2.5 py-1 text-caption-1-medium text-amber-800 dark:border-amber-500/30 dark:bg-amber-950/40 dark:text-amber-300">
          <RiErrorWarningLine className="size-3.5 shrink-0" aria-hidden />
          Held back · Charges must be verified before refunding
        </div>
        <Procedure
          name="Refund a duplicate charge"
          steps={[
            { label: "Look up charges", detail: "Found two $49.00 charges on March 1", state: "done" },
            { label: "Policy limit met", detail: "Duplicate confirmed · $49 (within $500 limit)", state: "done" },
            { label: "Refund a charge", detail: "Waiting for Billing to approve", state: "waiting" },
          ]}
        />
        <Agent>I found both $49.00 charges. I asked our Billing team to approve the refund.</Agent>
      </div>
    </div>
  );
}

const args = [
  { label: "Customer", value: "Omar Haddad" },
  { label: "Charge", value: "$49.00 on March 1" },
  { label: "Reason", value: "Duplicate" },
];

/* The approval card in the inbox thread, waiting for a teammate. */
export function ApprovalFragment() {
  return (
    <div className="flex w-full max-w-[440px] flex-col gap-3 rounded-t-2xl border border-b-0 border-border-button-default bg-background-full p-4 pb-10 shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]">
      <div className="flex items-start gap-2.5">
        <span
          className={cx("flex size-8 shrink-0 items-center justify-center rounded-lg", CHIP_COLORS.orange)}
          aria-hidden
        >
          <RiFlashlightLine className="size-[18px]" />
        </span>
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="text-body-medium text-text-primary">Refund a charge</span>
          <span className="text-body-2-regular text-text-secondary">Requested by dimmi · waiting for approval</span>
        </span>
        <Chip variant="caption" color="orange" className="shrink-0 gap-0.5">
          <RiTimeLine className="size-3" aria-hidden />
          52 min left
        </Chip>
      </div>
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 rounded-xl bg-background-secondary-default p-3">
        {args.map((line) => (
          <div key={line.label} className="contents">
            <dt className="text-body-2-regular text-text-secondary">{line.label}</dt>
            <dd className="text-body-2-medium text-text-primary">{line.value}</dd>
          </div>
        ))}
      </dl>
      <div className="flex flex-wrap items-center gap-2">
        <FakeButton icon={RiCheckLine} primary>
          Approve
        </FakeButton>
        <FakeButton icon={RiCloseCircleLine}>Decline</FakeButton>
        <span className="sm:ms-auto">
          <FakeButton icon={RiUserReceivedLine}>Take over</FakeButton>
        </span>
      </div>
    </div>
  );
}

function FakeButton({ children, icon: Icon, primary }: { children: ReactNode; icon?: Glyph; primary?: boolean }) {
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

const guards = [
  { icon: RiCheckboxCircleLine, title: "No approval", body: "Runs once allowed.", picked: false },
  { icon: RiShieldUserLine, title: "A teammate approves", body: "Anyone who can approve calls.", picked: true },
  { icon: RiUserStarLine, title: "An admin approves", body: "Only people who manage actions.", picked: false },
];

/* The approval choice on a workspace action, as radio cards. */
export function GuardFragment() {
  return (
    <div className="flex w-full max-w-[420px] flex-col rounded-t-2xl border border-b-0 border-border-button-default bg-background-full shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]">
      <div className="flex flex-col gap-1 border-b border-separator-border px-5 py-4">
        <span className="text-headline-semibold text-text-primary">Refund a charge</span>
        <span className="font-mono text-[12px] leading-5 text-text-tertiary">POST /v1/refunds</span>
      </div>
      <div className="flex flex-col gap-2 px-5 pt-4 pb-10">
        <span className="text-body-2-medium text-text-secondary">Approval</span>
        {guards.map((g) => (
          <div
            key={g.title}
            className={cx(
              "flex items-center gap-3 rounded-lg border px-3 py-2.5",
              g.picked ? "border-accent-500 bg-accent-500/6" : "border-border-button-default",
            )}
          >
            <g.icon className="size-4 shrink-0 text-foreground-icon-secondary" aria-hidden />
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="text-body-medium text-text-primary">{g.title}</span>
              <span className="text-caption-1-regular text-text-tertiary">{g.body}</span>
            </span>
            <span
              className={cx(
                "flex size-4 shrink-0 items-center justify-center rounded-full border",
                g.picked ? "border-accent-600" : "border-border-button-hover",
              )}
            >
              {g.picked ? <span className="size-2 rounded-full bg-accent-600" /> : null}
            </span>
          </div>
        ))}
        <div className="mt-2 flex items-center justify-between gap-3">
          <span className="text-body-2-regular text-text-secondary">Time limit</span>
          <span className="inline-flex h-7 items-center gap-1 rounded-lg border border-border-button-default px-2.5 text-body-2-medium text-text-primary shadow-xs">
            1 hour
            <RiArrowDownSLine className="size-3.5 text-foreground-icon-tertiary" aria-hidden />
          </span>
        </div>
      </div>
    </div>
  );
}
