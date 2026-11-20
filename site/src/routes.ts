/*
 * Every page the site prerenders. The prerender step only follows links
 * that appear here, so a link to a page that does not exist yet never
 * produces an empty HTML file.
 */
import type { ComponentType } from "react";

import { AgentPage } from "./pages/agent";
import { BroadcastsPage } from "./pages/broadcasts";
import { ChangelogPage } from "./pages/changelog";
import { FeedbackPage } from "./pages/feedback";
import { HelpCenterPage } from "./pages/help-center";
import { HomePage } from "./pages/home";
import { InboxPage } from "./pages/inbox";
import { OperatorPage } from "./pages/operator";
import { PricingPage } from "./pages/pricing";
import { ProceduresPage } from "./pages/procedures";
import { ReportsPage } from "./pages/reports";
import { TicketsPage } from "./pages/tickets";
import { WorkflowsPage } from "./pages/workflows";

export type Route = {
  path: string;
  title: string;
  description: string;
  page: ComponentType;
};

export const routes: Route[] = [
  {
    path: "/",
    title: "dimmi: customer support you own",
    description:
      "Open-source customer support: shared inbox, AI agent, help center, workflows, broadcasts, changelog and feedback. On our cloud or your servers.",
    page: HomePage,
  },
  {
    path: "/inbox",
    title: "Inbox: every conversation in one place | dimmi",
    description:
      "One inbox for the messenger, email, Slack and the API, with customer context, notes, assignment, snoozes and an AI teammate.",
    page: InboxPage,
  },
  {
    path: "/agent",
    title: "AI agent: answers, acts and hands over | dimmi",
    description:
      "An AI support agent that answers from your knowledge, follows your procedures and hands over cleanly, on your own model key.",
    page: AgentPage,
  },
  {
    path: "/workflows",
    title: "Workflows: automate support work | dimmi",
    description:
      "Build support automations on a visual canvas: trigger from anything, check conditions, then route, reply, escalate or hand to the AI agent.",
    page: WorkflowsPage,
  },
  {
    path: "/pricing",
    title: "Pricing: pay per conversation, never per seat | dimmi",
    description:
      "Unlimited seats, AI included, first 100 conversations free every month. Half price on AI with your own model key. Free to self-host under MIT.",
    page: PricingPage,
  },
  {
    path: "/operator",
    title: "Operator: an AI teammate beside every conversation | dimmi",
    description:
      "Operator briefs your team, answers questions about the customer, drafts replies and proposes changes you apply. Only your team sees it.",
    page: OperatorPage,
  },
  {
    path: "/procedures",
    title: "Procedures: teach the AI agent your team's way | dimmi",
    description:
      "Write how your team handles a request, step by step, in plain words. The agent follows it, calls your systems, and waits for approval where you say.",
    page: ProceduresPage,
  },
  {
    path: "/tickets",
    title: "Tickets: turn conversations into tracked work | dimmi",
    description:
      "Tickets with your own types and states, SLAs, internal hand-offs, and links to Linear, Jira and GitHub. Customers see progress without asking.",
    page: TicketsPage,
  },
  {
    path: "/reports",
    title: "Reports: know how your support is really doing | dimmi",
    description:
      "Volume, response and resolution times, CSAT, team performance and how often the AI agent resolves. Every number opens the conversations behind it.",
    page: ReportsPage,
  },
  {
    path: "/help-center",
    title: "Help center: answers customers find on their own | dimmi",
    description:
      "Write articles, publish them on your own domain, and let the AI agent answer from them. An open-source help center, built into dimmi.",
    page: HelpCenterPage,
  },
  {
    path: "/broadcasts",
    title: "Broadcasts: messages that start conversations | dimmi",
    description:
      "Send broadcasts and series by email, messenger and Slack, from your own inbox. Every reply lands in the inbox as a conversation.",
    page: BroadcastsPage,
  },
  {
    path: "/changelog",
    title: "Changelog: tell customers what you shipped | dimmi",
    description:
      "Publish product updates on a public changelog, in the messenger and by email. Labels, versions, scheduling and reactions included.",
    page: ChangelogPage,
  },
  {
    path: "/feedback",
    title: "Feedback: know what to build next | dimmi",
    description:
      "Feature request boards and a public roadmap on your help center. Customers vote, your team links conversations, and voters hear when it ships.",
    page: FeedbackPage,
  },
];

export function findRoute(path: string): Route | undefined {
  const clean = path.split(/[?#]/)[0].replace(/\/+$/, "") || "/";
  return routes.find((route) => route.path === clean);
}
