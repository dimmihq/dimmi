/*
 * Where the app's pages live and how the sidebar lists them. The app has
 * two sidebars: the workspace's, with the inbox first since answering
 * customers is the job, and the settings', which replaces it while the
 * person is in settings, as Linear and Productlane do. Every page has a
 * title and a line on what it is for, which the page shows until the
 * feature is built.
 */
import {
  RiArrowLeftLine,
  RiAtLine,
  RiBankCardLine,
  RiBookOpenLine,
  RiChatSmile3Line,
  RiContactsBook3Line,
  RiGroupLine,
  RiHistoryLine,
  RiInbox2Line,
  RiKey2Line,
  RiLightbulbLine,
  RiMegaphoneLine,
  RiNotification3Line,
  RiPlugLine,
  RiQuestionAnswerLine,
  RiSettings4Line,
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
  profile: page(
    "settings-profile",
    "Profile",
    RiUserLine,
    "/d/settings/profile",
    "Your name, photo, language and time zone.",
  ),
  security: page(
    "settings-security",
    "Security",
    RiShieldUserLine,
    "/d/settings/security",
    "Password, two-factor authentication, sessions and sign-in methods.",
  ),
  notifications: page(
    "settings-notifications",
    "Notifications",
    RiNotification3Line,
    "/d/settings/notifications",
    "Which mail and push notifications you get, and when.",
  ),
  workspace: page(
    "settings-workspace",
    "General",
    RiSettings4Line,
    "/d/settings/workspace",
    "The workspace's name, address and time zone.",
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
  channels: page(
    "settings-channels",
    "Channels",
    RiPlugLine,
    "/d/settings/channels",
    "Email, Slack, Discord and the other places customers write from.",
  ),
  messenger: page(
    "settings-messenger",
    "Messenger",
    RiChatSmile3Line,
    "/d/settings/messenger",
    "The widget on your site: its look, its install snippet and identity verification.",
  ),
  tokens: page(
    "settings-tokens",
    "API tokens",
    RiKey2Line,
    "/d/settings/tokens",
    "Tokens for scripts, the CLI and coding agents.",
  ),
  webhooks: page(
    "settings-webhooks",
    "Webhooks",
    RiWebhookLine,
    "/d/settings/webhooks",
    "Events sent to your own endpoints.",
  ),
  billing: page("settings-billing", "Billing", RiBankCardLine, "/d/settings/billing", "Your plan, usage and invoices."),
} as const;

export type SettingsSection = keyof typeof settingsPages;

/*
 * appSections lists the workspace's pages in the sidebar.
 */
export function appSections(): DashboardNavSection[] {
  return [
    { key: "you", items: [workspacePages.notifications, inboxViews.mine] },
    { key: "inbox", label: "Inbox", items: [inboxViews.unassigned, inboxViews.all, inboxViews.mentions] },
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
 * settingsSections lists the settings pages. Billing exists on the cloud
 * only, which a self-hosted install does not have.
 */
export function settingsSections(cloud: boolean): DashboardNavSection[] {
  const workspace = [
    settingsPages.workspace,
    settingsPages.members,
    settingsPages.teams,
    settingsPages.channels,
    settingsPages.messenger,
    settingsPages.tokens,
    settingsPages.webhooks,
  ];

  return [
    { key: "back", items: [{ key: "back", label: "Back to inbox", icon: RiArrowLeftLine, href: "/d" }] },
    {
      key: "account",
      label: "Account",
      items: [settingsPages.profile, settingsPages.security, settingsPages.notifications],
    },
    { key: "workspace", label: "Workspace", items: cloud ? [...workspace, settingsPages.billing] : workspace },
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
