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
import { VsChatwootPage } from "./pages/vs/chatwoot";
import { VsHelpScoutPage } from "./pages/vs/help-scout";
import { VsHubPage } from "./pages/vs/index";
import { VsIntercomPage } from "./pages/vs/intercom";
import { VsZendeskPage } from "./pages/vs/zendesk";
import { WorkflowsPage } from "./pages/workflows";
import { SDK_COUNT } from "./sdks";

export type PageType = "marketing" | "vs" | "alternatives" | "blog" | "self-host" | "tool" | "docs" | "api-reference";

export type PageRival = {
  name: string;
  checkedOn: string;
  source: string;
};

export type PageMeta = {
  type: PageType;
  keyword: string;
  answer: string;
  author: string;
  published: string;
  updated: string;
  status: "draft" | "published";
  reviewedBy: string;
  reviewedOn: string;
  batch?: string;
  rivals?: PageRival[];
  proof?: string[];
};

export type Route = {
  path: string;
  title: string;
  description: string;
  page: ComponentType;
  meta?: PageMeta;
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
    title: `Headless chat API, webhooks & ${SDK_COUNT} SDKs | dimmi`,
    description: `Build custom web or mobile chat on /widget/v1, sync backend threads over /api/v1, subscribe to signed webhooks, and use ${SDK_COUNT} typed SDKs and MCP.`,
    page: ApiChannelPage,
  },
  {
    path: "/vs",
    title: "Compare dimmi against top customer support tools | dimmi",
    description:
      "Honest side-by-side comparisons of dimmi vs Intercom, Chatwoot, Zendesk, and Help Scout: verified pricing, product captures, and MIT self-hosting.",
    page: VsHubPage,
    meta: {
      type: "alternatives",
      status: "published",
      published: "2026-11-27",
      updated: "2026-11-27",
      author: "frantz-kati",
      reviewedBy: "frantz-kati",
      reviewedOn: "2026-11-27",
      keyword: "compare customer support platforms",
      answer: "Every comparison below checks each vendor's live pricing page",
      proof: ["/shots/vs-byo-key-light.webp", "/shots/vs-sso-light.webp"],
    },
  },
  {
    path: "/vs/intercom",
    title: "dimmi vs Intercom: open-source Intercom alternative",
    description:
      "dimmi vs Intercom: $29 seats ($19 promo) and $0.99 Fin outcomes vs free seats, BYO model keys, and MIT self-hosting. Bills at 1, 5, and 25 seats.",
    page: VsIntercomPage,
    meta: {
      type: "vs",
      status: "published",
      published: "2026-11-27",
      updated: "2026-11-27",
      author: "frantz-kati",
      reviewedBy: "frantz-kati",
      reviewedOn: "2026-11-27",
      keyword: "intercom alternative",
      answer: "Pick Intercom if you need a mature platform with SOC 2",
      proof: ["/shots/vs-byo-key-light.webp", "/shots/vs-reports-light.webp"],
      rivals: [{ name: "Intercom", checkedOn: "2026-11-27", source: "https://www.intercom.com/pricing" }],
    },
  },
  {
    path: "/vs/chatwoot",
    title: "dimmi vs Chatwoot: MIT self-hosted Chatwoot alternative",
    description:
      "dimmi vs Chatwoot: pure MIT license with zero enterprise paywall, single Go binary vs Rails and Sidekiq, and BYO AI keys vs Captain AI credit caps.",
    page: VsChatwootPage,
    meta: {
      type: "vs",
      status: "published",
      published: "2026-11-27",
      updated: "2026-11-27",
      author: "frantz-kati",
      reviewedBy: "frantz-kati",
      reviewedOn: "2026-11-27",
      keyword: "chatwoot alternative",
      answer: "Pick Chatwoot if you need Telegram or LINE channels today",
      proof: ["/shots/vs-sso-light.webp"],
      rivals: [{ name: "Chatwoot", checkedOn: "2026-11-27", source: "https://www.chatwoot.com/pricing" }],
    },
  },
  {
    path: "/vs/zendesk",
    title: "dimmi vs Zendesk: open-source Zendesk alternative",
    description:
      "dimmi vs Zendesk: $69 to $149 Suite seats and $2.00 AI resolutions vs unlimited free seats, included SLAs and routing, BYO keys, and MIT self-hosting.",
    page: VsZendeskPage,
    meta: {
      type: "vs",
      status: "published",
      published: "2026-11-27",
      updated: "2026-11-27",
      author: "frantz-kati",
      reviewedBy: "frantz-kati",
      reviewedOn: "2026-11-27",
      keyword: "zendesk alternative",
      answer: "Pick Zendesk if you run a 200+ seat call center needing SOC 2",
      proof: ["/shots/vs-sla-light.webp", "/shots/workflow-canvas-light.webp"],
      rivals: [{ name: "Zendesk", checkedOn: "2026-11-27", source: "https://www.zendesk.com/pricing/" }],
    },
  },
  {
    path: "/vs/help-scout",
    title: "dimmi vs Help Scout: open-source Help Scout alternative",
    description:
      "dimmi vs Help Scout: $25 to $45 seats, $10 extra inbox fees, and $0.75 Envoy resolutions vs unlimited inboxes, free seats, BYO keys, and MIT code.",
    page: VsHelpScoutPage,
    meta: {
      type: "vs",
      status: "published",
      published: "2026-11-27",
      updated: "2026-11-27",
      author: "frantz-kati",
      reviewedBy: "frantz-kati",
      reviewedOn: "2026-11-27",
      keyword: "help scout alternative",
      answer: "Pick Help Scout if you want a veteran email-first help desk",
      proof: ["/shots/help-center-light.webp", "/shots/vs-inboxes-light.webp"],
      rivals: [{ name: "Help Scout", checkedOn: "2026-11-27", source: "https://www.helpscout.com/pricing/" }],
    },
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
