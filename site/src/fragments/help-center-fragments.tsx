/*
 * Product fragments for the help center page: the public help center as
 * readers see it (resources/views/helpcenter), and small pieces of the
 * dashboard where the team writes it, drawn with the app's own surfaces.
 */
import {
  RiArrowRightSLine,
  RiArrowUpLine,
  RiBookOpenLine,
  RiCheckLine,
  RiCodeSSlashLine,
  RiDoubleQuotesL,
  RiFolder3Line,
  RiGlobalLine,
  RiH2,
  RiImageLine,
  RiLightbulbLine,
  RiSparkling2Line,
  RiTable2,
  RiUserFollowLine,
} from "@remixicon/react";
import { AnimatePresence, motion } from "motion/react";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import { Agent, Customer, Source } from "../scenes/agent-pieces";
import { useScene } from "../scenes/use-scene";

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

/* A browser's top bar with the page's address. */
export function BrowserBar({ address }: { address: string }) {
  return (
    <div className="flex items-center gap-3 border-b border-separator-border px-4 py-2.5">
      <span className="flex gap-1.5" aria-hidden>
        <span className="size-2.5 rounded-full bg-background-tertiary-default" />
        <span className="size-2.5 rounded-full bg-background-tertiary-default" />
        <span className="size-2.5 rounded-full bg-background-tertiary-default" />
      </span>
      <span className="mx-auto rounded-md bg-background-secondary-default px-3 py-1 text-caption-1-regular text-text-secondary">
        {address}
      </span>
      <span className="w-[42px]" aria-hidden />
    </div>
  );
}

/* The help center's header: its logo, name and the sections beside help. */
export function PortalHeader({ current }: { current: "help" | "changelog" }) {
  return (
    <div className="flex items-center gap-3 border-b border-separator-border px-5 py-3">
      <span className="flex size-6 items-center justify-center rounded-md bg-accent-600 text-caption-1-medium text-text-white">
        L
      </span>
      <span className="text-body-medium text-text-primary">Lumen</span>
      <span className="ms-auto flex items-center gap-1 text-body-2-medium">
        {(["help", "changelog"] as const).map((key) => (
          <span
            key={key}
            className={cx(
              "rounded-lg px-2.5 py-1",
              current === key ? "bg-background-secondary-default text-text-primary" : "text-text-tertiary",
            )}
          >
            {key === "help" ? "Help center" : "Changelog"}
          </span>
        ))}
      </span>
    </div>
  );
}

const questions = [
  "How do I export a report to CSV?",
  "Can I share a dashboard with a client?",
  "Where do I change my plan?",
];

const groups = [
  {
    title: "Getting started",
    rows: [
      { name: "Set up your workspace", count: 6 },
      { name: "Import your data", count: 4 },
      { name: "Invite your team", count: 3 },
    ],
  },
  {
    title: "Billing and plans",
    rows: [
      { name: "Plans and pricing", count: 5 },
      { name: "Invoices and receipts", count: 3 },
    ],
  },
];

/*
 * The help center's front page, as resources/views/helpcenter/home.templ
 * draws it: the greeting, the one box that asks the AI, suggested
 * articles under it, then the collections. The box types a new question
 * every few seconds.
 */
export function HelpHomeFragment() {
  const { ref, step } = useScene(questions.length, { interval: 2600, hold: 2600 });
  return (
    <div ref={ref} className="w-full max-w-[760px]" aria-hidden>
      <Card className="h-[540px] overflow-hidden rounded-b-none border-b-0">
        <BrowserBar address="help.lumen.app" />
        <PortalHeader current="help" />
        <div className="mx-auto flex max-w-[500px] flex-col px-5 pt-10">
          <h3 className="text-title-1-medium tracking-tight text-text-primary">
            Hi there. <span className="text-text-tertiary">How can we help?</span>
          </h3>
          <div className="mt-5 overflow-hidden rounded-2xl border border-border-button-default bg-background-primary-default shadow-sm">
            <div className="flex h-14 items-center gap-3 ps-4 pe-2.5">
              <RiSparkling2Line className="size-5 shrink-0 text-accent-600 dark:text-accent-400" />
              <span className="relative h-6 min-w-0 flex-1 overflow-hidden">
                <AnimatePresence initial={false}>
                  <motion.span
                    key={step}
                    initial={{ opacity: 0, y: 8 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -8 }}
                    transition={{ duration: 0.35 }}
                    className="absolute inset-0 truncate text-headline-regular text-text-primary"
                  >
                    {questions[step]}
                  </motion.span>
                </AnimatePresence>
              </span>
              <span className="grid size-9 shrink-0 place-items-center rounded-full bg-accent-600 text-text-white">
                <RiArrowUpLine className="size-4" />
              </span>
            </div>
            <div className="flex flex-wrap gap-1 border-t border-separator-border bg-background-secondary-default px-2.5 py-2 text-body-2-regular text-text-secondary">
              <span className="px-1.5 py-1">Invite your team</span>
              <span className="px-1.5 py-1">Share a dashboard</span>
              <span className="px-1.5 py-1">Change your plan</span>
            </div>
          </div>
          <div className="mt-8 flex flex-col gap-6">
            {groups.map((group) => (
              <div key={group.title}>
                <p className="px-1 pb-2 text-caption-1-medium text-text-tertiary">{group.title}</p>
                <div className="overflow-hidden rounded-xl border border-border-button-default bg-background-primary-default">
                  {group.rows.map((row, i) => (
                    <div
                      key={row.name}
                      className={cx(
                        "flex items-center gap-3 px-4 py-2.5 text-body-medium text-text-primary",
                        i > 0 && "border-t border-separator-border",
                      )}
                    >
                      <span className="min-w-0 flex-1 truncate">{row.name}</span>
                      <span className="text-caption-1-medium text-text-tertiary tabular-nums">{row.count}</span>
                      <RiArrowRightSLine className="size-4 text-foreground-icon-tertiary" />
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      </Card>
    </div>
  );
}

const blocks = [
  { icon: RiH2, name: "Heading" },
  { icon: RiLightbulbLine, name: "Callout" },
  { icon: RiImageLine, name: "Image" },
  { icon: RiTable2, name: "Table" },
  { icon: RiCodeSSlashLine, name: "Code" },
  { icon: RiDoubleQuotesL, name: "Quote" },
];

/* The article writer with its slash menu open. */
export function WriterFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <div className="flex flex-col gap-1.5 px-4 pt-4">
        <span className="text-headline-semibold text-text-primary">Export a report</span>
        <span className="text-body-2-regular text-text-secondary">Download any report as CSV or PDF.</span>
        <span className="mt-1 text-body-2-regular text-text-primary">
          /<span className="ms-px inline-block h-4 w-px translate-y-0.5 bg-text-primary" />
        </span>
      </div>
      <ul className="m-2 mt-1.5 grid grid-cols-2 gap-0.5 rounded-lg border border-separator-border p-1">
        {blocks.map((block, i) => (
          <li
            key={block.name}
            className={cx(
              "flex items-center gap-2 rounded-md px-2 py-1.5 text-body-2-regular text-text-primary",
              i === 1 && "bg-background-secondary-default",
            )}
          >
            <block.icon className="size-4 text-foreground-icon-secondary" />
            {block.name}
          </li>
        ))}
      </ul>
    </Card>
  );
}

const tree = [
  { name: "Getting started", count: 13, depth: 0 },
  { name: "Set up your workspace", count: 6, depth: 1 },
  { name: "Billing and plans", count: 8, depth: 0, picked: true },
  { name: "Integrations", count: 12, depth: 0 },
  { name: "Account and security", count: 5, depth: 0 },
];

/* The collections pane: collections as they nest, one picked. */
export function CollectionsFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <div className="border-b border-separator-border px-4 py-3 text-body-medium text-text-primary">Collections</div>
      <ul className="flex flex-col px-2 py-2">
        {tree.map((item) => (
          <li
            key={item.name}
            className={cx(
              "flex items-center gap-2.5 rounded-md px-2 py-1.5",
              item.picked && "bg-background-secondary-default",
            )}
            style={{ paddingInlineStart: 8 + item.depth * 18 }}
          >
            <RiFolder3Line className="size-4 shrink-0 text-foreground-icon-secondary" />
            <span className="truncate text-body-regular text-text-primary">{item.name}</span>
            <span className="ms-auto text-caption-1-regular text-text-tertiary tabular-nums">{item.count}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

const records = [
  { type: "CNAME", purpose: "Points your domain here" },
  { type: "TXT", purpose: "Proves you own it" },
];

/* A help center's own domain, verified. */
export function DomainFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <div className="flex items-center justify-between border-b border-separator-border px-4 py-3">
        <span className="text-body-medium text-text-primary">Your domain</span>
        <Chip variant="caption" color="lime">
          Verified
        </Chip>
      </div>
      <div className="flex flex-col gap-3 px-4 py-3.5">
        <div className="flex items-center gap-2 rounded-lg border border-border-button-default px-3 py-2">
          <RiGlobalLine className="size-4 text-foreground-icon-secondary" />
          <span className="text-body-regular text-text-primary">help.lumen.app</span>
        </div>
        <ul className="flex flex-col gap-2">
          {records.map((record) => (
            <li key={record.type} className="flex items-center gap-2.5">
              <span className="w-12 font-mono text-[11px] leading-4 font-medium text-text-secondary">
                {record.type}
              </span>
              <span className="text-caption-1-regular text-text-secondary">{record.purpose}</span>
              <RiCheckLine className="ms-auto size-4 text-accent-600" />
            </li>
          ))}
        </ul>
        <span className="text-caption-1-regular text-text-tertiary">
          Certificate issued. The old address still works.
        </span>
      </div>
    </Card>
  );
}

/* A line in the thread that says what happened, not who said it. */
function Event({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      className="flex items-center justify-center gap-1.5 text-caption-1-regular text-text-tertiary"
    >
      {icon}
      {children}
    </motion.div>
  );
}

/*
 * A question asked in the help center's box, answered by the agent from
 * an article, then picked up by the team in the same thread. The step
 * the page's numbered list is on chooses how far it has got.
 */
export function AskFlowFragment({ step }: { step: number }) {
  return (
    <div className="flex h-[440px] w-[360px] max-w-full flex-col gap-4" aria-hidden>
      <div className="flex h-12 shrink-0 items-center gap-3 rounded-2xl border border-border-button-default bg-background-full ps-4 pe-2 shadow-sm">
        <RiSparkling2Line className="size-4 shrink-0 text-accent-600 dark:text-accent-400" />
        <span className="min-w-0 flex-1 truncate text-body-regular text-text-primary">
          How do I export a report to CSV?
        </span>
        <span
          className={cx(
            "grid size-8 shrink-0 place-items-center rounded-full text-text-white transition-colors",
            step === 0 ? "bg-accent-600" : "bg-border-button-hover",
          )}
        >
          <RiArrowUpLine className="size-4" />
        </span>
      </div>
      <div className="flex flex-col gap-4">
        <AnimatePresence initial={false}>
          {step >= 1 ? (
            <Customer key="q" name="Nia Lindqvist">
              How do I export a report to CSV?
            </Customer>
          ) : null}
          {step >= 1 ? (
            <Agent
              key="a"
              footer={
                <Source icon={RiBookOpenLine} key="s">
                  Export a report
                </Source>
              }
            >
              Open the report, choose Export, then CSV. The file downloads right away.
            </Agent>
          ) : null}
          {step >= 2 ? (
            <Customer key="q2" name="Nia Lindqvist">
              Can it go to our finance team every Monday?
            </Customer>
          ) : null}
          {step >= 2 ? (
            <Event key="e" icon={<RiUserFollowLine className="size-3.5" />}>
              Kofi Mensah joined, with a summary
            </Event>
          ) : null}
        </AnimatePresence>
      </div>
    </div>
  );
}
