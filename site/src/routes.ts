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
import { ChannelsIndexPage } from "./pages/channels/index";
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

export const DEFAULT_TITLE = "dimmi: Open-source Intercom alternative for customer support";

export const routes: Route[] = [
  {
    path: "/",
    title: DEFAULT_TITLE,
    description:
      "Open-source (MIT) Intercom alternative: shared inbox, AI agents on your own model key, help center, and workflows. Free seats. Cloud or self-hosted.",
    page: HomePage,
  },
  {
    path: "/inbox",
    title: "Shared inbox with unlimited free seats | dimmi",
    description:
      "Web chat, email, Slack, WhatsApp, Instagram, and Facebook messages in one shared inbox beside customer history, with unlimited free seats.",
    page: InboxPage,
  },
  {
    path: "/agents",
    title: "AI support agents for every queue & brand | dimmi",
    description:
      "Run dedicated AI support agents for frontline, billing, docs, and brands. Replies cite your docs, transfer quietly, and cut AI fees with BYO keys.",
    page: AgentsPage,
  },
  {
    path: "/agent-studio",
    title: "Agent Studio: test & compare AI agents | dimmi",
    description:
      "Write agent guidance with @ chips, run multi-turn simulations, compare Draft against Live side by side, and roll back changes in one click.",
    page: AgentStudioPage,
  },
  {
    path: "/routines",
    title: "Operator Routines: scheduled doc sync & QA | dimmi",
    description:
      "Put Operator on a daily or weekly schedule to sync merged GitHub PRs to help articles, audit knowledge gaps, and review inline diffs.",
    page: RoutinesPage,
  },
  {
    path: "/workflows",
    title: "Support workflows, routing & SLAs | dimmi",
    description:
      "Build visual support workflows included on every plan. Trigger on events, branch on rules, open tickets, or call your own API.",
    page: WorkflowsPage,
  },
  {
    path: "/pricing",
    title: "Pricing: pay per conversation, not per seat | dimmi",
    description:
      "Unlimited free seats, 100 free conversations every month, and 50% to 83% off AI fees with your own model keys. Free to self-host under MIT.",
    page: PricingPage,
  },
  {
    path: "/operator",
    title: "Operator: internal AI teammate for support | dimmi",
    description:
      "Operator sums up long chats, searches customer history, drafts replies, and proposes edits you can undo. Customers never see it.",
    page: OperatorPage,
  },
  {
    path: "/procedures",
    title: "Procedures: AI support policies with limits | dimmi",
    description:
      "Write how your team handles refunds, cancellations, and account changes. Agents follow every step, respect hard limits, and pause for approval.",
    page: ProceduresPage,
  },
  {
    path: "/tickets",
    title: "Support tickets, SLAs & outage trackers | dimmi",
    description:
      "Turn chats into tracked tickets with custom states, SLAs, outage trackers, and links to Linear, Jira, GitHub, and PagerDuty.",
    page: TicketsPage,
  },
  {
    path: "/reports",
    title: "Support reports, CSAT & agent metrics | dimmi",
    description:
      "Track volume, reply times, CSAT, and AI agent results by team or channel. Click any number to inspect the exact conversations behind it.",
    page: ReportsPage,
  },
  {
    path: "/help-center",
    title: "Help center for customers and AI agents | dimmi",
    description:
      "Publish help articles on your own domain. Customers get instant answers, and your support agents read from the same docs.",
    page: HelpCenterPage,
  },
  {
    path: "/broadcasts",
    title: "Customer broadcasts people can reply to | dimmi",
    description:
      "Send broadcasts and series by email, WhatsApp, chat, and Slack from your own domain. Every reply opens in your inbox.",
    page: BroadcastsPage,
  },
  {
    path: "/changelog",
    title: "Product changelog in app and by email | dimmi",
    description:
      "Publish product updates once and reach customers on your changelog, inside the messenger, and by email.",
    page: ChangelogPage,
  },
  {
    path: "/feedback",
    title: "Feature request boards & public roadmap | dimmi",
    description:
      "Collect customer votes on your help center, link live support chats, and email every voter the day you ship.",
    page: FeedbackPage,
  },
  {
    path: "/channels",
    title: "Support channels: chat, email, Slack & Meta | dimmi",
    description:
      "Connect live web messenger, verified support email, Slack Connect, WhatsApp Business, Instagram DMs, Facebook Messenger, and custom API channels.",
    page: ChannelsIndexPage,
  },
  {
    path: "/channels/messenger",
    title: "Web messenger: in-app chat, docs & updates | dimmi",
    description:
      "Embed live chat, help center search, and release notes with one script tag or 6 framework packages. Verified identity and AI agents included.",
    page: MessengerChannelPage,
  },
  {
    path: "/channels/email",
    title: "Shared email inbox for support@ teams | dimmi",
    description:
      "Forward support@, billing@, and security@ into one shared inbox with verified SPF/DKIM/DMARC domains, collision alerts, and AI support agents.",
    page: EmailChannelPage,
  },
  {
    path: "/channels/slack",
    title: "Slack Connect customer support inbox | dimmi",
    description:
      "Turn Slack Connect, community, and internal help desk channels into tracked support threads with two-way sync, SLAs, tickets, and Operator.",
    page: SlackChannelPage,
  },
  {
    path: "/channels/whatsapp",
    title: "WhatsApp Business shared inbox & templates | dimmi",
    description:
      "Connect official WhatsApp Business numbers via Meta, manage approved templates, track 24-hour windows, and answer with zero message markup.",
    page: WhatsAppChannelPage,
  },
  {
    path: "/channels/instagram",
    title: "Instagram DM shared inbox for support | dimmi",
    description:
      "Connect Instagram Business accounts via Meta OAuth, answer DMs and story replies beside order history, and automate returns with AI agents.",
    page: InstagramChannelPage,
  },
  {
    path: "/channels/facebook-messenger",
    title: "Facebook Messenger support inbox | dimmi",
    description:
      "Connect Facebook Pages via Meta OAuth, route multiple brands into dedicated inboxes, and answer Page messages beside email and WhatsApp.",
    page: FacebookMessengerChannelPage,
  },
  {
    path: "/channels/api",
    title: "Headless chat API, webhooks & 11 SDKs | dimmi",
    description:
      "Build custom web or mobile chat on /widget/v1, sync backend threads over /api/v1, subscribe to signed webhooks, and use 11 typed SDKs and MCP.",
    page: ApiChannelPage,
  },
  {
    path: "/terms",
    title: "Terms of Service | dimmi",
    description:
      "dimmi Cloud Terms of Service, Product-Specific Terms, Acceptable Use Policy, and Self-Hosting Notice by Hivy Technologies Limited.",
    page: TermsPage,
  },
  {
    path: "/privacy",
    title: "Privacy Policy & Data Governance | dimmi",
    description:
      "dimmi Cloud Privacy Policy, End-User Notice, Cookie Notice, Subprocessor List, AI Transparency, Security Overview, and Global DPA.",
    page: PrivacyPage,
  },
];

export function findRoute(path: string): Route | undefined {
  const clean = path.split(/[?#]/)[0].replace(/\/+$/, "") || "/";
  return routes.find((route) => route.path === clean);
}
