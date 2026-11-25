/*
 * Every page the site prerenders. The prerender step only follows links
 * that appear here, so a link to a page that does not exist yet never
 * produces an empty HTML file.
 */
import type { ComponentType } from "react";
import { AgentStudioPage } from "./pages/agent-studio";
import { AgentsPage } from "./pages/agents";
import { BroadcastsPage } from "./pages/broadcasts";
import { ChangelogPage } from "./pages/changelog";
import { ApiChannelPage } from "./pages/channels/api";
import { EmailChannelPage } from "./pages/channels/email";
import { FacebookMessengerChannelPage } from "./pages/channels/facebook-messenger";
import { InstagramChannelPage } from "./pages/channels/instagram";
import { MessengerChannelPage } from "./pages/channels/messenger";
import { SlackChannelPage } from "./pages/channels/slack";
import { WhatsAppChannelPage } from "./pages/channels/whatsapp";
import { FeedbackPage } from "./pages/feedback";
import { HelpCenterPage } from "./pages/help-center";
import { HomePage } from "./pages/home";
import { InboxPage } from "./pages/inbox";
import { OperatorPage } from "./pages/operator";
import { PricingPage } from "./pages/pricing";
import { PrivacyPage } from "./pages/privacy";
import { ProceduresPage } from "./pages/procedures";
import { ReportsPage } from "./pages/reports";
import { RoutinesPage } from "./pages/routines";
import { TermsPage } from "./pages/terms";
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
    title: "dimmi: the open customer support platform",
    description:
      "Shared inbox, specialized support agents, tickets, workflows, and help center, all open source under MIT. Run on dimmi Cloud with free seats or self-host for $0.",
    page: HomePage,
  },
  {
    path: "/inbox",
    title: "Inbox: one queue with unlimited free seats | dimmi",
    description:
      "Chat, email, Slack, WhatsApp, SMS, and phone in one inbox beside customer history. Seats are free so your whole team can jump in.",
    page: InboxPage,
  },
  {
    path: "/agents",
    title: "Agents: dedicated support agents for every customer need | dimmi",
    description:
      "Run one frontline support agent or build specialists for billing, technical docs, orders, and brands. Cites your docs and saves 50% to 83% with your own model keys.",
    page: AgentsPage,
  },
  {
    path: "/agent-studio",
    title: "Agent Studio: build, simulate, and compare support agents | dimmi",
    description:
      "Write guidance with @ chips, catch flaky replies with multi-run simulations, compare Draft against Live side by side, and diagnose failures with Operator.",
    page: AgentStudioPage,
  },
  {
    path: "/routines",
    title: "Operator Routines: scheduled doc sync, gap audits, and QA | dimmi",
    description:
      "Put Operator on a daily or weekly schedule to sync merged GitHub PRs to help articles, catch knowledge gaps, and review edits inline.",
    page: RoutinesPage,
  },
  {
    path: "/workflows",
    title: "Workflows: automate routing, SLAs, and follow-ups | dimmi",
    description:
      "Build visual support workflows included on every plan. Trigger on events, branch on rules, open tickets, or call your own API.",
    page: WorkflowsPage,
  },
  {
    path: "/pricing",
    title: "Pricing: pay per conversation, never per seat | dimmi",
    description:
      "Unlimited free seats, 100 free conversations every month, and 50% to 83% off AI fees with your own model keys. Free to self-host under MIT.",
    page: PricingPage,
  },
  {
    path: "/operator",
    title: "Operator: an internal AI teammate in your inbox | dimmi",
    description:
      "Operator sums up long chats, searches customer history, drafts replies, and proposes edits you can undo. Customers never see it.",
    page: OperatorPage,
  },
  {
    path: "/procedures",
    title: "Procedures: support policies your agents cannot break | dimmi",
    description:
      "Write how your team handles refunds, cancellations, and account changes. Agents follow every step, respect hard limits, and pause for approval.",
    page: ProceduresPage,
  },
  {
    path: "/tickets",
    title: "Tickets: stop losing bugs inside long chat threads | dimmi",
    description:
      "Turn chats into tracked tickets with custom states, SLAs, outage trackers, and links to Linear, Jira, GitHub, and more.",
    page: TicketsPage,
  },
  {
    path: "/reports",
    title: "Reports: support metrics you can click into | dimmi",
    description:
      "Track volume, reply times, CSAT, and AI results by team or channel. Click any bar to inspect the conversations behind it.",
    page: ReportsPage,
  },
  {
    path: "/help-center",
    title: "Help center: one source for customers and agents | dimmi",
    description:
      "Publish help articles on your own domain. Customers get instant answers, and your support agents read from the same docs.",
    page: HelpCenterPage,
  },
  {
    path: "/broadcasts",
    title: "Broadcasts: customer updates people can reply to | dimmi",
    description:
      "Send broadcasts and series by email, SMS, WhatsApp, chat, and Slack from your own domain. Every reply opens in your inbox.",
    page: BroadcastsPage,
  },
  {
    path: "/changelog",
    title: "Changelog: release notes customers see | dimmi",
    description:
      "Publish product updates once and reach customers on your changelog, inside the messenger, and by email.",
    page: ChangelogPage,
  },
  {
    path: "/feedback",
    title: "Feedback: feature boards linked to your inbox | dimmi",
    description:
      "Collect customer votes on your help center, link live support chats, and email every voter the day you ship.",
    page: FeedbackPage,
  },
  {
    path: "/channels/messenger",
    title: "Web messenger: live chat, docs, and updates inside your app | dimmi",
    description:
      "Embed live chat, help center search, and release notes with one script tag or 6 framework packages. Verified customer identity and AI support agents included.",
    page: MessengerChannelPage,
  },
  {
    path: "/channels/email",
    title: "Shared email inbox: run support@ on your own domain | dimmi",
    description:
      "Forward support@, billing@, and security@ into one shared inbox with verified SPF/DKIM/DMARC domains, collision alerts, internal notes, and AI support agents.",
    page: EmailChannelPage,
  },
  {
    path: "/channels/slack",
    title: "Slack customer support: sync Slack Connect channels to your inbox | dimmi",
    description:
      "Turn Slack Connect, community, and internal help desk channels into tracked support threads with two-way sync, SLAs, tickets, support agents, and Operator.",
    page: SlackChannelPage,
  },
  {
    path: "/channels/whatsapp",
    title: "WhatsApp Business shared inbox: templates, media, and AI agents | dimmi",
    description:
      "Connect official WhatsApp Business numbers through Meta, manage approved templates, track 24-hour windows, and answer together with zero message markup.",
    page: WhatsAppChannelPage,
  },
  {
    path: "/channels/instagram",
    title: "Instagram DM support: shared inbox for brand accounts | dimmi",
    description:
      "Connect Instagram Business accounts via Meta OAuth, answer DMs and story replies beside order history, track reply windows, and automate returns with support agents.",
    page: InstagramChannelPage,
  },
  {
    path: "/channels/facebook-messenger",
    title: "Facebook Messenger support: manage Page messages in one queue | dimmi",
    description:
      "Connect Facebook Pages via Meta OAuth, route multiple brands into dedicated inboxes, and answer Page messages beside email and WhatsApp without Meta Business Suite.",
    page: FacebookMessengerChannelPage,
  },
  {
    path: "/channels/api",
    title: "Headless chat & API: custom UIs, webhooks, and 11 typed SDKs | dimmi",
    description:
      "Build custom web or mobile chat on /widget/v1, sync backend threads over /api/v1, subscribe to signed webhooks, and ship faster with 11 typed SDKs and MCP.",
    page: ApiChannelPage,
  },
  {
    path: "/terms",
    title: "Terms of Service | dimmi",
    description:
      "Complete dimmi Cloud Terms of Service, Product-Specific Terms, Acceptable Use Policy, and Self-Hosting Notice by Hivy Technologies Limited (RC 9601398).",
    page: TermsPage,
  },
  {
    path: "/privacy",
    title: "Privacy Policy & Data Governance | dimmi",
    description:
      "Complete dimmi Cloud Privacy Policy, End-User Notice, Cookie Notice, Subprocessor List, AI Transparency, Security Overview, and Global DPA.",
    page: PrivacyPage,
  },
];

export function findRoute(path: string): Route | undefined {
  const clean = path.split(/[?#]/)[0].replace(/\/+$/, "") || "/";
  return routes.find((route) => route.path === clean);
}
