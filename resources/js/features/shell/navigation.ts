/*
 * Where the app's pages live and how the sidebar lists them. The app has
 * two sidebars: the workspace's, with the inbox first since answering
 * customers is the job, and the settings', which replaces it while the
 * person is in settings, as Linear and Productlane do. Every page has a
 * title and a line on what it is for, which the page shows until the
 * feature is built.
 */
import {
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
  RiDownload2Line,
  RiEqualizerLine,
  RiFileList3Line,
  RiFingerprintLine,
  RiFlashlightLine,
  RiGitBranchLine,
  RiGithubLine,
  RiGroupLine,
  RiHistoryLine,
  RiInbox2Line,
  RiKey2Line,
  RiLightbulbLine,
  RiListOrdered2,
  RiMailLine,
  RiMegaphoneLine,
  RiNotification3Line,
  RiPlugLine,
  RiPriceTag3Line,
  RiQuestionAnswerLine,
  RiSettings4Line,
  RiShieldCheckLine,
  RiShieldUserLine,
  RiSparkling2Line,
  RiStackLine,
  RiTeamLine,
  RiUserLine,
  RiWebhookLine,
} from "@remixicon/react";

import type { DashboardNavItem, DashboardNavSection } from "@/components/application/dashboard/dashboard-sidebar";

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
    "Dimmi Agent answers customers from your docs and hands over when it should.",
  ),
  workflows: page(
    "workflows",
    "Workflows",
    RiGitBranchLine,
    "/d/workflows",
    "Route conversations and decide when Dimmi takes them.",
  ),
  procedures: page(
    "procedures",
    "Procedures",
    RiListOrdered2,
    "/d/procedures",
    "Step-by-step instructions Dimmi follows for particular requests.",
  ),
  docs: page(
    "docs",
    "Docs",
    RiBookOpenLine,
    "/d/docs",
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
  announcements: page(
    "announcements",
    "Announcements",
    RiMegaphoneLine,
    "/d/announcements",
    "Banners, posts and surveys shown inside your product.",
  ),
  customers: page(
    "customers",
    "Customers",
    RiContactsBook3Line,
    "/d/customers",
    "Everyone who wrote in, with their conversations and companies.",
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
    "Language, time zone, theme and how the inbox behaves for you.",
  ),
  notifications: page(
    "settings-notifications",
    "Notifications",
    RiNotification3Line,
    "/d/settings/notifications",
    "What reaches you in the app, by push, by mail and in Slack.",
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
  inboxes: page(
    "settings-inboxes",
    "Inboxes",
    RiInbox2Line,
    "/d/settings/inboxes",
    "Where conversations land and how they reopen.",
  ),
  tags: page("settings-tags", "Tags", RiPriceTag3Line, "/d/settings/tags", "Labels teammates put on conversations."),
  "saved-replies": page(
    "settings-saved-replies",
    "Saved replies",
    RiChatQuoteLine,
    "/d/settings/saved-replies",
    "Answers your team sends often, with variables.",
  ),
  messenger: page(
    "settings-messenger",
    "Messenger",
    RiChatSmile3Line,
    "/d/settings/messenger",
    "The widget on your site: its look, texts and install.",
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
  agent: page(
    "settings-agent",
    "Dimmi agent",
    RiSparkling2Line,
    "/d/settings/agent",
    "How Dimmi answers customers, when it hands over and what it may spend.",
  ),
  actions: page(
    "settings-actions",
    "Actions",
    RiFlashlightLine,
    "/d/settings/actions",
    "What Dimmi can do in your own systems, such as looking up an order.",
  ),
  "ai-providers": page(
    "settings-ai-providers",
    "AI providers",
    RiCpuLine,
    "/d/settings/ai-providers",
    "The model keys Dimmi and Operator run on.",
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
    "Sites, files and repositories Dimmi learns from.",
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
    "Agents, the CLI and other apps you have let act in Dimmi for you.",
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
  "workspace-security": page(
    "settings-workspace-security",
    "Security",
    RiShieldCheckLine,
    "/d/settings/workspace-security",
    "Rules every member signs in under.",
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
 * appSections lists the workspace's pages in the sidebar, leaving out
 * Unassigned for a member who cannot see conversations nobody holds.
 */
export function appSections({ unassigned = true }: { unassigned?: boolean } = {}): DashboardNavSection[] {
  const views = unassigned
    ? [inboxViews.unassigned, inboxViews.all, inboxViews.mentions]
    : [inboxViews.all, inboxViews.mentions];

  return [
    { key: "you", items: [workspacePages.notifications, inboxViews.mine] },
    { key: "inbox", label: "Inbox", items: views },
    { key: "automation", label: "Automation", items: [workspacePages.workflows, workspacePages.procedures] },
    {
      key: "workspace",
      label: "Workspace",
      items: [
        workspacePages.agent,
        workspacePages.docs,
        workspacePages.feedback,
        workspacePages.changelog,
        workspacePages.announcements,
        workspacePages.customers,
      ],
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
    pages["workspace-security"],
    pages["audit-log"],
    pages.import,
  ];

  return [
    { key: "back", items: [{ key: "back", label: "Back to inbox", icon: RiArrowLeftLine, href: "/d" }] },
    {
      key: "account",
      label: "Account",
      items: [pages.profile, pages.preferences, pages.notifications, pages.security, pages.tokens],
    },
    { key: "inbox", label: "Inbox", items: [pages.inboxes, pages.tags, pages["saved-replies"]] },
    { key: "channels", label: "Channels", items: [pages.messenger, pages.email, pages.connections] },
    { key: "ai", label: "AI", items: [pages.agent, pages.actions, pages["ai-providers"]] },
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
    const href = item.href ?? "";
    const holds = path === href || path.startsWith(`${href}/`);
    if (holds && href.length > best.length) {
      best = { key: item.key, length: href.length };
    }
  }

  return best.key;
}
