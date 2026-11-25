import {
  BellSlash,
  BookOpenText,
  CalendarCheck,
  GitPullRequest,
  ListMagnifyingGlass,
  Sliders,
} from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../components/product-page";
import { ShotStill } from "../components/shot-still";
import { FinalCta } from "../sections/final-cta";

const rows = [
  {
    label: "GitHub PR to docs",
    title: "Engineering ships a feature. Your help center draft is waiting by 8:30 AM.",
    body: "Every morning, Operator reads yesterday's merged pull requests, skips internal chores, and leaves exact edits on the help articles that went out of date.",
    visual: (
      <ShotStill
        name="routine-pr-diff"
        alt="Inline Help Center article change request diff drafted by Operator from a merged GitHub pull request"
      />
    ),
  },
  {
    label: "Knowledge gaps & QA",
    title: "Turn unanswered chats and slipping policies into weekly fixes.",
    body: "Spot recurring questions your agents could not answer, catch contradictory articles, and get a Friday support digest paired with concrete procedure fixes.",
    visual: (
      <ShotStill
        name="routine-procedure-diff"
        alt="Inline Procedure change request diff proposed by the weekly QA and procedure tune-up routine"
      />
    ),
  },
  {
    label: "Quiet when fine",
    title: "Zero Slack spam on quiet mornings. Shared threads when something matters.",
    body: "When nothing changed since the last run, Operator stays silent. When it finds real work, it posts to Slack or email and opens a shared thread so teammates can refine the draft.",
    visual: (
      <ShotStill
        name="routine-thread"
        alt="Shared Operator routine thread showing findings, inline changeset cards, and a teammate follow-up reply"
      />
    ),
  },
  {
    label: "Safe by default",
    title: "Human approval on every edit, and zero repeated rejections.",
    body: "Routines never publish an article or change a live procedure on their own. Operator remembers declined suggestions, expires stale ones, and pauses if errors repeat or its owner leaves.",
    visual: (
      <ShotStill
        name="routine-guardrails"
        alt="Routine settings showing cadence, dry run, quiet delivery, permissions, and optional monthly budget cap"
      />
    ),
  },
];

const more = [
  {
    icon: GitPullRequest,
    title: "Pull requests to docs",
    body: "Daily check of merged GitHub pull requests against your live help articles so product changes never break docs.",
  },
  {
    icon: BookOpenText,
    title: "Knowledge gap finder",
    body: "Weekly scan of handed-over chats that drafts missing public articles or agent-only notes with customer details stripped.",
  },
  {
    icon: ListMagnifyingGlass,
    title: "Conflict and staleness audit",
    body: "Rolling review across your help center that catches outdated limits, old screenshots, and contradictory pages.",
  },
  {
    icon: CalendarCheck,
    title: "Weekly QA and procedure tune-up",
    body: "Friday support digest alongside suggested fixes for policies that caused repeated handovers.",
  },
  {
    icon: Sliders,
    title: "Write your own routines",
    body: "Put any recurring ops prompt on a daily, weekly, monthly, or custom schedule with scoped permissions.",
  },
  {
    icon: BellSlash,
    title: "Inline review where you work",
    body: "Approve, decline, or undo proposed edits directly inside your Help Center and Procedure editors, in Slack, or by email.",
  },
];

const questions = [
  {
    question: "Can a scheduled routine publish a help article or edit a live procedure by itself?",
    answer:
      "Every routine only proposes reviewable changesets that wait for a teammate's click. Proposed edits appear inline inside your Help Center and Procedure editors, in the shared thread, and in Slack with Apply, Decline, and Undo.",
  },
  {
    question: "Will this flood our Slack channel with 'nothing to report' messages every day?",
    answer:
      "By default, routines only notify your team when they actually find something or propose an edit. Quiet runs stay logged in run history with zero notifications.",
  },
  {
    question: "What if we decline a suggestion? Will Operator keep proposing it every week?",
    answer:
      "Operator checks recent pending, applied, and declined suggestions before proposing anything new, and old unreviewed suggestions expire automatically.",
  },
  {
    question: "How does Pull requests to docs connect to GitHub?",
    answer:
      "Connect your GitHub integration once, pick the repositories and branch to watch, and set a daily or weekly time. Operator only reads pull requests merged since the last run and skips tests, refactors, and dependency bumps.",
  },
  {
    question: "Can we test a routine before turning it on for the team?",
    answer:
      "Click Dry run to execute the routine against real workspace data without pinging Slack, sending emails, or moving the schedule forward.",
  },
  {
    question: "How are routines billed and capped?",
    answer:
      "Each routine run counts like a standard Operator conversation under your workspace's monthly spend cap, and you can add an optional per-routine monthly budget cap. Every run shows its exact cost in history.",
  },
];

export function RoutinesPage() {
  return (
    <>
      <ProductHero
        eyebrow="Operator Routines"
        title="Keep your docs and policies fresh without remembering to check."
        body="Put Operator on a daily or weekly clock. It reads merged GitHub PRs, unanswered chats, and slipping policies, then leaves ready-to-review edits right inside your editors."
        shot="routines-list"
        shotAlt="Operator Routines page showing Pull requests to docs, Knowledge gap finder, Conflict and staleness audit, and Weekly QA and procedure tune-up"
      />
      <Rows title="The support upkeep nobody has time for, ready before Monday morning." rows={rows} />
      <FeatureGrid title="Built-in routines, plus anything your team repeats." items={more} />
      <Questions body="What support and docs leads ask before putting Operator on a clock." items={questions} />
      <FinalCta />
    </>
  );
}
