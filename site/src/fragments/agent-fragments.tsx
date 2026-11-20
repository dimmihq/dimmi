/*
 * Product fragments for the AI agent page: small pieces of the agent's
 * settings and logs, drawn with the app's own surfaces and type so they
 * read as the product at a glance.
 */
import { RiBookOpenLine, RiCheckLine, RiFileTextLine, RiGlobalLine } from "@remixicon/react";
import type { ReactNode } from "react";

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

const sources = [
  { icon: RiBookOpenLine, name: "Help center", count: "42 articles" },
  { icon: RiGlobalLine, name: "Website", count: "84 pages" },
  { icon: RiFileTextLine, name: "Files", count: "6 documents" },
];

export function KnowledgeFragment() {
  return (
    <Card className="max-w-[300px] self-end rounded-b-none border-b-0">
      <div className="flex items-center justify-between border-b border-separator-border px-4 py-3">
        <span className="text-body-medium text-text-primary">Knowledge</span>
        <span className="inline-flex items-center gap-1.5 rounded-md bg-accent-500/12 px-1.5 py-0.5 text-caption-1-medium text-accent-700 dark:text-accent-400">
          <span className="size-1.5 rounded-full bg-accent-500" />
          Synced
        </span>
      </div>
      <ul className="flex flex-col px-2 py-2">
        {sources.map((source) => (
          <li key={source.name} className="flex items-center gap-2.5 px-2 py-2">
            <source.icon className="size-4 text-foreground-icon-secondary" />
            <span className="text-body-regular text-text-primary">{source.name}</span>
            <span className="ms-auto text-caption-1-regular text-text-tertiary">{source.count}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

const rules = [
  { name: "Answer in the messenger", on: true },
  { name: "Answer email", on: true },
  { name: "Answer in Slack", on: true },
  { name: "Reply to enterprise accounts", on: false },
];

export function RulesFragment() {
  return (
    <Card className="max-w-75 self-end rounded-b-none border-b-0">
      <div className="border-b border-separator-border px-4 py-3 text-body-medium text-text-primary">
        Where it answers
      </div>
      <ul className="flex flex-col px-2 py-2">
        {rules.map((rule) => (
          <li key={rule.name} className="flex items-center justify-between gap-3 px-2 py-2">
            <span className="text-body-regular text-text-primary">{rule.name}</span>
            <span
              className={cx(
                "flex h-4.5 w-8 shrink-0 items-center rounded-full p-0.5",
                rule.on ? "justify-end bg-accent-600" : "justify-start bg-border-button-hover",
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

const log = [
  "Read the whole conversation",
  "Applied your voice and rules",
  "Used 2 knowledge sources",
  "Drafted a reply",
];

export function TestFragment() {
  return (
    <Card className="max-w-75 self-end rounded-b-none border-b-0">
      <div className="flex items-center justify-between border-b border-separator-border px-4 py-3">
        <span className="text-body-medium text-text-primary">Test run</span>
        <span className="text-caption-1-regular text-text-tertiary">Not sent</span>
      </div>
      <ol className="flex flex-col gap-2 px-4 py-3.5 font-mono text-[12px] leading-5 text-text-secondary">
        {log.map((line) => (
          <li key={line} className="flex items-center gap-2">
            <RiCheckLine className="size-3.5 text-accent-600" />
            {line}
          </li>
        ))}
        <li className="mt-1 inline-flex items-center gap-2">
          <span className="rounded bg-accent-500/12 px-1.5 py-0.5 font-medium text-accent-700 dark:text-accent-400">
            ANSWERED
          </span>
          <span className="text-text-tertiary">took 5.2s</span>
        </li>
      </ol>
    </Card>
  );
}

const models = [
  { name: "Claude", by: "Anthropic", picked: true },
  { name: "GPT", by: "OpenAI", picked: false },
  { name: "Gemini", by: "Google", picked: false },
  { name: "Your own endpoint", by: "Any OpenAI-compatible API", picked: false },
];

export function ModelFragment() {
  return (
    <Card className="max-w-105 rounded-b-none border-b-0">
      <div className="flex flex-col gap-1 border-b border-separator-border px-5 py-4">
        <span className="text-headline-semibold text-text-primary">Model</span>
        <span className="text-body-regular text-text-secondary">Runs on your key. You pay your provider.</span>
      </div>
      <ul className="flex flex-col gap-2 px-5 py-5">
        {models.map((model) => (
          <li
            key={model.name}
            className={cx(
              "flex items-center gap-3 rounded-lg border px-3 py-2.5",
              model.picked ? "border-accent-500 bg-accent-500/6" : "border-border-button-default",
            )}
          >
            <span
              className={cx(
                "flex size-4 items-center justify-center rounded-full border",
                model.picked ? "border-accent-600" : "border-border-button-hover",
              )}
            >
              {model.picked ? <span className="size-2 rounded-full bg-accent-600" /> : null}
            </span>
            <span className="text-body-medium text-text-primary">{model.name}</span>
            <span className="ms-auto text-caption-1-regular text-text-tertiary">{model.by}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

/* A two-message exchange for the "connected" step. */
export function ExchangeFragment() {
  return (
    <div className="flex w-full max-w-[320px] flex-col gap-4 self-center">
      <Customer name="Joseph Myers">Stuck on onboarding. Help?</Customer>
      <Agent>Happy to help. Which step are you on?</Agent>
    </div>
  );
}
