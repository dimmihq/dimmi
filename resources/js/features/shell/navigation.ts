/*
 * Where the app's pages live and how the sidebar lists them. The app has
 * two sidebars: the workspace's, with the inbox first since answering
 * customers is the job, and the settings', which replaces it while the
 * person is in settings, as Linear and Productlane do. Every page has a
 * title and a line on what it is for, which the page shows until the
 * feature is built.
 */
import {
  RiAlarmLine,
  RiApps2Line,
  RiArrowLeftLine,
  RiAtLine,
  RiBankCardLine,
  RiBookOpenLine,
  RiChatQuoteLine,
  RiChatSmile3Line,
  RiContactsBook3Line,
  RiCpuLine,
  RiDatabase2Line,
  RiDeleteBin6Line,
  RiDownload2Line,
  RiEqualizerLine,
  RiFileList3Line,
  RiFingerprintLine,
  RiFlashlightLine,
  RiFlowChart,
  RiGitBranchLine,
  RiGithubLine,
  RiGroupLine,
  RiHistoryLine,
  RiInbox2Line,
  RiKey2Line,
  RiLightbulbLine,
  RiListOrdered2,
  RiLoginBoxLine,
  RiMailLine,
  RiMegaphoneLine,
  RiNotification3Line,
  RiPlugLine,
  RiPriceTag3Line,
  RiPuzzle2Line,
  RiQuestionAnswerLine,
  RiRobot2Line,
  RiSettings4Line,
  RiShieldCheckLine,
  RiShieldUserLine,
  RiSpam2Line,
  RiSparkling2Line,
  RiStackLine,
  RiTeamLine,
  RiTimeLine,
  RiUserLine,
  RiWebhookLine,
} from "@remixicon/react";

import type {
  DashboardNavItem,
  DashboardNavPanel,
  DashboardNavSection,
} from "@/components/application/dashboard/dashboard-sidebar";

/*
 * A page of the app: its address, its row in a sidebar, and the words it
 * shows about itself.
 */
export interface Page extends DashboardNavItem {
  href: string;
  description: string;
}

function page(key: string, label: string, icon: DashboardNavItem["icon"], href: string, description: string): Page {
  return { key, label, icon, href, description };
}

export const inboxViews = {
  mine: page("inbox-mine", "My inbox", RiInbox2Line, "/d/inbox/mine", "Conversations assigned to you."),
  unassigned: page(
    "inbox-unassigned",
    "Unassigned",
    RiQuestionAnswerLine,
    "/d/inbox/unassigned",
    "Conversations nobody has picked up yet.",
  ),
  all: page("inbox-all", "All", RiStackLine, "/d/inbox/all", "Every open conversation in the workspace."),
  mentions: page(
    "inbox-mentions",
    "Mentions",
    RiAtLine,
    "/d/inbox/mentions",
    "Conversations where a teammate mentioned you.",
  ),
  snoozed: page(
    "inbox-snoozed",
    "Snoozed",
    RiAlarmLine,
    "/d/inbox/snoozed",
    "Your conversations that sleep until their time comes.",
  ),
  spam: page("inbox-spam", "Spam", RiSpam2Line, "/d/inbox/spam", "Conversations marked as spam."),
  deleted: page("inbox-deleted", "Deleted", RiDeleteBin6Line, "/d/inbox/deleted", "Conversations in the trash."),
} as const;

export type InboxView = keyof typeof inboxViews;

export const workspacePages = {
  notifications: page(
    "notifications",
    "Notifications",
    RiNotification3Line,
    "/d/notifications",
    "Replies, mentions and assignments that need you.",
  ),
  agent: page(
    "agent",
    "Agent",
    RiSparkling2Line,
    "/d/agent",
    "dimmi agent answers customers from your docs and hands over when it should.",
  ),
  workflows: page(
    "workflows",
    "Workflows",
    RiGitBranchLine,
    "/d/workflows",
    "Route conversations and decide when dimmi takes them.",
  ),
  procedures: page(
    "procedures",
    "Procedures",
    RiListOrdered2,
    "/d/procedures",
    "Step-by-step instructions dimmi follows for particular requests.",
  ),
  helpCenter: page(
    "help-center",
    "Help center",
    RiBookOpenLine,
    "/d/help-center",
    "Your help center: articles, collections and the review queue.",
  ),
  feedback: page(
    "feedback",
    "Feature requests",
    RiLightbulbLine,
    "/d/feedback",
    "Requests from customers and teammates, voted and grouped into your roadmap.",
  ),
  changelog: page(
    "changelog",
    "Changelog",
    RiHistoryLine,
    "/d/changelog",
    "Tell customers what shipped, by mail and in the widget.",
  ),
  customers: page(
    "customers",
    "Customers",
    RiContactsBook3Line,
    "/d/customers",
    "Everyone who wrote in, with their conversations and companies.",
  ),
  broadcasts: page(
    "broadcasts",
    "Broadcasts",
    RiMegaphoneLine,
    "/d/broadcasts",
    "One-off messages to an audience, by email, in the messenger or inside your product.",
  ),
  series: page(
    "series",
    "Series",
    RiFlowChart,
    "/d/series",
    "Messages sent over days and weeks, each step waiting for what the customer does.",
  ),
} as const;

export const settingsLink = page("settings", "Settings", RiSettings4Line, "/d/settings", "");

export const settingsPages = {
  profile: page("settings-profile", "Profile", RiUserLine, "/d/settings/profile", "Your name, photo and email."),
  preferences: page(
    "settings-preferences",
    "Preferences",
    RiEqualizerLine,
    "/d/settings/preferences",
    "Time zone and theme: how dimmi looks for you.",
  ),
  notifications: page(
    "settings-notifications",
    "Notifications",
    RiNotification3Line,
    "/d/settings/notifications",
    "What reaches you, where, and when to stay quiet.",
  ),
  security: page(
    "settings-security",
    "Security & access",
    RiShieldUserLine,
    "/d/settings/security",
    "Password, two-factor authentication, sign-in methods and sessions.",
  ),
  tokens: page(
    "settings-tokens",
    "API tokens",
    RiKey2Line,
    "/d/settings/tokens",
    "Tokens for scripts, the CLI and coding agents.",
  ),
  "connected-apps": page(
    "settings-connected-apps",
    "Connected apps",
    RiRobot2Line,
    "/d/settings/connected-apps",
    "Apps and AI agents you let work in dimmi as you.",
  ),
  exports: page(
    "settings-exports",
    "Exports",
    RiDownload2Line,
    "/d/settings/exports",
    "Files of conversations and customers you exported.",
  ),
  inboxes: page(
    "settings-inboxes",
    "Inboxes",
    RiInbox2Line,
    "/d/settings/inboxes",
    "Where conversations land and how they reopen.",
  ),
  tags: page("settings-tags", "Tags", RiPriceTag3Line, "/d/settings/tags", "Labels teammates put on conversations."),
  attributes: page(
    "settings-attributes",
    "Attributes",
    RiDatabase2Line,
    "/d/settings/attributes",
    "The fields your contacts and companies carry, and their types.",
  ),
  "office-hours": page(
    "settings-office-hours",
    "Office hours",
    RiTimeLine,
    "/d/settings/office-hours",
    "When your team is around to reply.",
  ),
  "saved-replies": page(
    "settings-saved-replies",
    "Saved replies",
    RiChatQuoteLine,
    "/d/settings/saved-replies",
    "Answers your team sends often, with variables.",
  ),
  "conversation-safety": page(
    "settings-conversation-safety",
    "Conversation safety",
    RiShieldCheckLine,
    "/d/settings/conversation-safety",
    "Keep card numbers and other sensitive text out of conversations.",
  ),
  messenger: page(
    "settings-messenger",
    "Messengers",
    RiChatSmile3Line,
    "/d/settings/messenger",
    "The chat on your site or in your app: its look, texts and install.",
  ),
  email: page(
    "settings-email",
    "Email",
    RiMailLine,
    "/d/settings/email",
    "Support addresses, sending domains and signatures.",
  ),
  connections: page(
    "settings-connections",
    "Connections",
    RiPlugLine,
    "/d/settings/connections",
    "Slack, Discord, Telegram, WhatsApp and the other places customers write from.",
  ),
  integrations: page(
    "settings-integrations",
    "Integrations",
    RiPuzzle2Line,
    "/d/settings/integrations",
    "The tools your team already uses, connected once for every feature.",
  ),
  agent: page(
    "settings-agent",
    "dimmi agent",
    RiSparkling2Line,
    "/d/settings/agent",
    "How the agent answers customers, when it hands over and what it may spend.",
  ),
  operator: page(
    "settings-operator",
    "Operator",
    RiRobot2Line,
    "/d/settings/operator",
    "Your team's own agent: what it may change, what it costs and what it knows.",
  ),
  actions: page(
    "settings-actions",
    "Actions",
    RiFlashlightLine,
    "/d/settings/actions",
    "What the agent can do in your own systems, such as looking up an order.",
  ),
  "ai-providers": page(
    "settings-ai-providers",
    "AI providers",
    RiCpuLine,
    "/d/settings/ai-providers",
    "The models the agent answers with.",
  ),
  "help-centers": page(
    "settings-help-centers",
    "Help centers",
    RiBookOpenLine,
    "/d/settings/help-centers",
    "Your public docs: address, branding and domain.",
  ),
  knowledge: page(
    "settings-knowledge",
    "Knowledge",
    RiDatabase2Line,
    "/d/settings/knowledge",
    "Sites, files and repositories dimmi learns from.",
  ),
  changelog: page(
    "settings-changelog",
    "Changelog",
    RiHistoryLine,
    "/d/settings/changelog",
    "How releases are published and mailed to subscribers.",
  ),
  feedback: page(
    "settings-feedback",
    "Feedback",
    RiLightbulbLine,
    "/d/settings/feedback",
    "The feedback portal, its boards and statuses.",
  ),
  engage: page(
    "settings-engage",
    "Announcements & surveys",
    RiMegaphoneLine,
    "/d/settings/engage",
    "How often customers may be interrupted, and the survey after a conversation.",
  ),
  webhooks: page(
    "settings-webhooks",
    "Webhooks",
    RiWebhookLine,
    "/d/settings/webhooks",
    "Events sent to your own endpoints.",
  ),
  identity: page(
    "settings-identity",
    "Identity verification",
    RiFingerprintLine,
    "/d/settings/identity",
    "Secrets that prove who a signed-in visitor is.",
  ),
  apps: page(
    "settings-apps",
    "Authorized apps",
    RiApps2Line,
    "/d/settings/apps",
    "Agents, the CLI and other apps you have let act in dimmi for you.",
  ),
  github: page(
    "settings-github",
    "GitHub",
    RiGithubLine,
    "/d/settings/github",
    "Repositories for docs sync and release notes.",
  ),
  workspace: page(
    "settings-workspace",
    "Workspace",
    RiSettings4Line,
    "/d/settings/workspace",
    "The workspace's name, address, language and time zone.",
  ),
  members: page(
    "settings-members",
    "Members",
    RiGroupLine,
    "/d/settings/members",
    "Who is in the workspace, their roles and invitations.",
  ),
  teams: page(
    "settings-teams",
    "Teams",
    RiTeamLine,
    "/d/settings/teams",
    "Groups of teammates conversations can be assigned to.",
  ),
  roles: page(
    "settings-roles",
    "Roles",
    RiShieldUserLine,
    "/d/settings/roles",
    "What each person in the workspace may do.",
  ),
  "workspace-security": page(
    "settings-workspace-security",
    "Security",
    RiShieldCheckLine,
    "/d/settings/workspace-security",
    "Rules every member signs in under.",
  ),
  "single-sign-on": page(
    "settings-single-sign-on",
    "Single sign-on",
    RiLoginBoxLine,
    "/d/settings/single-sign-on",
    "Sign in through your company's identity provider.",
  ),
  "audit-log": page(
    "settings-audit-log",
    "Audit log",
    RiFileList3Line,
    "/d/settings/audit-log",
    "Who changed what in the workspace, and when.",
  ),
  import: page(
    "settings-import",
    "Import & export",
    RiDownload2Line,
    "/d/settings/import",
    "Move in from another tool, or take your data with you.",
  ),
  billing: page(
    "settings-billing",
    "Billing",
    RiBankCardLine,
    "/d/settings/billing",
    "Your usage, spend cap and invoices.",
  ),
} as const;
export type SettingsSection = keyof typeof settingsPages;

/*
 * inboxLink is the sidebar's one way into the inbox. The inbox opens on
 * the view the person last looked at, and the list's title switches
 * between views, so the sidebar stays short however many views, teams
 * and teammates the workspace has.
 */
export const inboxLink = page("inbox", "Inbox", RiInbox2Line, "/d/inbox", "Every conversation, in the view you pick.");

/*
 * appSections lists the workspace's pages in the sidebar, the inbox
 * showing how many open conversations are assigned to the person.
 */
export function appSections(
  inboxBadge?: number,
  notificationBadge?: number,
  notifications?: DashboardNavPanel,
): DashboardNavSection[] {
  const unseen = notificationBadge && notificationBadge >= 100 ? "99+" : notificationBadge || undefined;
  return [
    {
      key: "you",
      items: [
        { ...workspacePages.notifications, href: undefined, badge: unseen, panel: notifications },
        { ...inboxLink, badge: inboxBadge || undefined },
      ],
    },
    {
      key: "automation",
      label: "Automation",
      items: [workspacePages.agent, workspacePages.workflows, workspacePages.procedures],
    },
    {
      key: "customers",
      label: "Customers",
      items: [workspacePages.customers, workspacePages.broadcasts, workspacePages.series],
    },
    {
      key: "knowledge",
      label: "Knowledge",
      items: [workspacePages.helpCenter, workspacePages.feedback, workspacePages.changelog],
    },
  ];
}

/*
 * settingsSections lists the settings pages in the sidebar, grouped as
 * the person thinks of them: their own account first, then the inbox and
 * its channels, AI, content, developers, and running the workspace.
 * Billing exists on the cloud only.
 */
export function settingsSections(cloud: boolean): DashboardNavSection[] {
  const pages = settingsPages;
  const admin = [
    pages.workspace,
    pages.members,
    pages.teams,
    pages.roles,
    pages["workspace-security"],
    pages["single-sign-on"],
    pages["audit-log"],
    pages.import,
  ];

  return [
    { key: "back", items: [{ key: "back", label: "Back to inbox", icon: RiArrowLeftLine, href: "/d" }] },
    {
      key: "account",
      label: "Account",
      items: [
        pages.profile,
        pages.preferences,
        pages.notifications,
        pages.security,
        pages.tokens,
        pages["connected-apps"],
        pages.exports,
      ],
    },
    {
      key: "inbox",
      label: "Inbox",
      items: [
        pages.inboxes,
        pages.tags,
        pages.attributes,
        pages["office-hours"],
        pages["saved-replies"],
        pages["conversation-safety"],
      ],
    },
    {
      key: "channels",
      label: "Channels",
      items: [pages.messenger, pages.email, pages.connections, pages.integrations],
    },
    { key: "ai", label: "AI", items: [pages.agent, pages.operator, pages.actions, pages["ai-providers"]] },
    {
      key: "content",
      label: "Content",
      items: [pages["help-centers"], pages.knowledge, pages.changelog, pages.feedback, pages.engage],
    },
    { key: "developers", label: "Developers", items: [pages.webhooks, pages.identity, pages.apps, pages.github] },
    { key: "administration", label: "Administration", items: cloud ? [...admin, pages.billing] : admin },
  ];
}

/*
 * selectedKey returns the key of the row whose page holds the path: the
 * row with the longest address the path starts with.
 */
export function selectedKey(sections: DashboardNavSection[], extra: DashboardNavItem[], path: string): string {
  let best = { key: "", length: -1 };

  for (const item of [...sections.flatMap((section) => section.items), ...extra]) {
    const href = item.href;
    if (!href) {
      continue;
    }
    const holds = path === href || path.startsWith(`${href}/`);
    if (holds && href.length > best.length) {
      best = { key: item.key, length: href.length };
    }
  }

  return best.key;
}
