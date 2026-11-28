/*
 * Every page the site prerenders. The prerender step only follows links
 * that appear here, so a link to a page that does not exist yet never
 * produces an empty HTML file.
 */
import type { ComponentType } from "react";
import { SDK_COUNT } from "./sdks";

export type Route = {
  path: string;
  title: string;
  description: string;
  /** Loads the page's own chunk, so a visit downloads only the page it shows. */
  load: () => Promise<ComponentType>;
};

export const DEFAULT_TITLE = "dimmi: Open-source Intercom alternative for customer support";

export const routes: Route[] = [
  {
    path: "/",
    title: DEFAULT_TITLE,
    description:
      "Open-source (MIT) Intercom alternative: shared inbox, AI agents on your own model key, help center, and workflows. Free seats. Cloud or self-hosted.",
    load: () => import("./pages/home").then((m) => m.HomePage),
  },
  {
    path: "/inbox",
    title: "Shared inbox with unlimited free seats | dimmi",
    description:
      "Web chat, email, Slack, WhatsApp, Instagram, and Facebook messages in one shared inbox beside customer history, with unlimited free seats.",
    load: () => import("./pages/inbox").then((m) => m.InboxPage),
  },
  {
    path: "/agents",
    title: "AI support agents for every queue & brand | dimmi",
    description:
      "Run dedicated AI support agents for frontline, billing, docs, and brands. Replies cite your docs, transfer quietly, and cut AI fees with BYO keys.",
    load: () => import("./pages/agents").then((m) => m.AgentsPage),
  },
  {
    path: "/agent-studio",
    title: "Agent Studio: test & compare AI agents | dimmi",
    description:
      "Write agent guidance with @ chips, run multi-turn simulations, compare Draft against Live side by side, and roll back changes in one click.",
    load: () => import("./pages/agent-studio").then((m) => m.AgentStudioPage),
  },
  {
    path: "/routines",
    title: "Operator Routines: scheduled doc sync & QA | dimmi",
    description:
      "Put Operator on a daily or weekly schedule to sync merged GitHub PRs to help articles, audit knowledge gaps, and review inline diffs.",
    load: () => import("./pages/routines").then((m) => m.RoutinesPage),
  },
  {
    path: "/workflows",
    title: "Support workflows, routing & SLAs | dimmi",
    description:
      "Build visual support workflows included on every plan. Trigger on events, branch on rules, open tickets, or call your own API.",
    load: () => import("./pages/workflows").then((m) => m.WorkflowsPage),
  },
  {
    path: "/pricing",
    title: "Pricing: pay per conversation, not per seat | dimmi",
    description:
      "Unlimited free seats, 100 free conversations every month, and 50% to 83% off AI fees with your own model keys. Free to self-host under MIT.",
    load: () => import("./pages/pricing").then((m) => m.PricingPage),
  },
  {
    path: "/operator",
    title: "Operator: internal AI teammate for support | dimmi",
    description:
      "Operator sums up long chats, searches customer history, drafts replies, and proposes edits you can undo. Customers never see it.",
    load: () => import("./pages/operator").then((m) => m.OperatorPage),
  },
  {
    path: "/procedures",
    title: "Procedures: AI support policies with limits | dimmi",
    description:
      "Write how your team handles refunds, cancellations, and account changes. Agents follow every step, respect hard limits, and pause for approval.",
    load: () => import("./pages/procedures").then((m) => m.ProceduresPage),
  },
  {
    path: "/tickets",
    title: "Support tickets, SLAs & outage trackers | dimmi",
    description:
      "Turn chats into tracked tickets with custom states, SLAs, outage trackers, and links to Linear, Jira, GitHub, and PagerDuty.",
    load: () => import("./pages/tickets").then((m) => m.TicketsPage),
  },
  {
    path: "/reports",
    title: "Support reports, CSAT & agent metrics | dimmi",
    description:
      "Track volume, reply times, CSAT, and AI agent results by team or channel. Click any number to inspect the exact conversations behind it.",
    load: () => import("./pages/reports").then((m) => m.ReportsPage),
  },
  {
    path: "/help-center",
    title: "Help center for customers and AI agents | dimmi",
    description:
      "Publish help articles on your own domain. Customers get instant answers, and your support agents read from the same docs.",
    load: () => import("./pages/help-center").then((m) => m.HelpCenterPage),
  },
  {
    path: "/broadcasts",
    title: "Customer broadcasts people can reply to | dimmi",
    description:
      "Send broadcasts and series by email, WhatsApp, chat, and Slack from your own domain. Every reply opens in your inbox.",
    load: () => import("./pages/broadcasts").then((m) => m.BroadcastsPage),
  },
  {
    path: "/changelog",
    title: "Product changelog in app and by email | dimmi",
    description:
      "Publish product updates once and reach customers on your changelog, inside the messenger, and by email.",
    load: () => import("./pages/changelog").then((m) => m.ChangelogPage),
  },
  {
    path: "/feedback",
    title: "Feature request boards & public roadmap | dimmi",
    description:
      "Collect customer votes on your help center, link live support chats, and email every voter the day you ship.",
    load: () => import("./pages/feedback").then((m) => m.FeedbackPage),
  },
  {
    path: "/channels",
    title: "Support channels: chat, email, Slack & Meta | dimmi",
    description:
      "Connect live web messenger, verified support email, Slack Connect, WhatsApp Business, Instagram DMs, Facebook Messenger, and custom API channels.",
    load: () => import("./pages/channels/index").then((m) => m.ChannelsIndexPage),
  },
  {
    path: "/channels/messenger",
    title: "Web messenger: in-app chat, docs & updates | dimmi",
    description:
      "Embed live chat, help center search, and release notes with one script tag or 6 framework packages. Verified identity and AI agents included.",
    load: () => import("./pages/channels/messenger").then((m) => m.MessengerChannelPage),
  },
  {
    path: "/channels/email",
    title: "Shared email inbox for support@ teams | dimmi",
    description:
      "Forward support@, billing@, and security@ into one shared inbox with verified SPF/DKIM/DMARC domains, collision alerts, and AI support agents.",
    load: () => import("./pages/channels/email").then((m) => m.EmailChannelPage),
  },
  {
    path: "/channels/slack",
    title: "Slack Connect customer support inbox | dimmi",
    description:
      "Turn Slack Connect, community, and internal help desk channels into tracked support threads with two-way sync, SLAs, tickets, and Operator.",
    load: () => import("./pages/channels/slack").then((m) => m.SlackChannelPage),
  },
  {
    path: "/channels/whatsapp",
    title: "WhatsApp Business shared inbox & templates | dimmi",
    description:
      "Connect official WhatsApp Business numbers via Meta, manage approved templates, track 24-hour windows, and answer with zero message markup.",
    load: () => import("./pages/channels/whatsapp").then((m) => m.WhatsAppChannelPage),
  },
  {
    path: "/channels/instagram",
    title: "Instagram DM shared inbox for support | dimmi",
    description:
      "Connect Instagram Business accounts via Meta OAuth, answer DMs and story replies beside order history, and automate returns with AI agents.",
    load: () => import("./pages/channels/instagram").then((m) => m.InstagramChannelPage),
  },
  {
    path: "/channels/facebook-messenger",
    title: "Facebook Messenger support inbox | dimmi",
    description:
      "Connect Facebook Pages via Meta OAuth, route multiple brands into dedicated inboxes, and answer Page messages beside email and WhatsApp.",
    load: () => import("./pages/channels/facebook-messenger").then((m) => m.FacebookMessengerChannelPage),
  },
  {
    path: "/channels/api",
    title: `Headless chat API, webhooks & ${SDK_COUNT} SDKs | dimmi`,
    description: `Build custom web or mobile chat on /widget/v1, sync backend threads over /api/v1, subscribe to signed webhooks, and use ${SDK_COUNT} typed SDKs and MCP.`,
    load: () => import("./pages/channels/api").then((m) => m.ApiChannelPage),
  },
  {
    path: "/terms",
    title: "Terms of Service | dimmi",
    description:
      "dimmi Cloud Terms of Service, Product-Specific Terms, Acceptable Use Policy, and Self-Hosting Notice by Hivy Technologies Limited.",
    load: () => import("./pages/terms").then((m) => m.TermsPage),
  },
  {
    path: "/privacy",
    title: "Privacy Policy & Data Governance | dimmi",
    description:
      "dimmi Cloud Privacy Policy, End-User Notice, Cookie Notice, Subprocessor List, AI Transparency, Security Overview, and Global DPA.",
    load: () => import("./pages/privacy").then((m) => m.PrivacyPage),
  },
];

const loaded = new Map<string, ComponentType>();

/**
 * The route's page once loadPage has fetched it. The browser loads the
 * first page before it hydrates, so the prerendered HTML always has its
 * page ready.
 */
export function loadedPage(route: Route | undefined): ComponentType | undefined {
  return route ? loaded.get(route.path) : undefined;
}

export async function loadPage(route: Route | undefined): Promise<ComponentType | undefined> {
  if (!route) {
    return undefined;
  }
  const page = loaded.get(route.path) ?? (await route.load());
  loaded.set(route.path, page);
  return page;
}

export function findRoute(path: string): Route | undefined {
  const clean = path.split(/[?#]/)[0].replace(/\/+$/, "") || "/";
  return routes.find((route) => route.path === clean);
}
