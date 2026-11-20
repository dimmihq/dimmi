/*
 * The docs' navigation: the sidebar groups Starlight builds pages and
 * search from, and the tabs across the header. Each tab shows only its
 * own groups in the sidebar; the API reference tab's sidebar is built
 * from the OpenAPI contract (src/api/spec.ts).
 */

// `label` is the short sidebar name (one to three words; the group gives
// the context). The page's own title, in its frontmatter, is the full one.
type Link = {
  label: string;
  link: string;
  badge?: { text: string; variant?: "note" | "tip" | "caution" | "danger" | "success" | "default" };
};
type Group = { label: string; items: Link[]; collapsed?: boolean };

export const sidebar: Group[] = [
  {
    label: "Get started",
    items: [
      { label: "Introduction", link: "/" },
      { label: "Quickstart", link: "/quickstart/" },
      { label: "Concepts", link: "/concepts/" },
      { label: "Cloud vs self-hosting", link: "/cloud-vs-self-hosted/" },
    ],
  },
  {
    label: "Inbox",
    collapsed: true,
    items: [
      { label: "Overview", link: "/inbox/how-the-inbox-works/" },
      { label: "Views", link: "/inbox/views/" },
      { label: "Saved replies", link: "/inbox/saved-replies/" },
      { label: "Inboxes", link: "/inbox/inboxes-and-assignment/" },
      { label: "Assignment", link: "/inbox/how-assignment-works/" },
      { label: "Office hours", link: "/inbox/office-hours/" },
    ],
  },
  {
    label: "Tickets and SLAs",
    collapsed: true,
    items: [
      { label: "Overview", link: "/tickets/how-tickets-work/" },
      { label: "Ticket types", link: "/tickets/ticket-types/" },
      { label: "Trackers", link: "/tickets/trackers/" },
      { label: "Linked issues", link: "/tickets/linked-issues/" },
      { label: "Linked incidents", link: "/tickets/linked-incidents/" },
      { label: "SLA policies", link: "/tickets/sla-policies/" },
      { label: "SLA timers", link: "/tickets/how-sla-timers-work/" },
    ],
  },
  {
    label: "Channels",
    collapsed: true,
    items: [
      { label: "Messenger", link: "/channels/install-the-messenger/" },
      { label: "Messenger settings", link: "/channels/customize-the-messenger/" },
      { label: "Email", link: "/channels/support-email/" },
      { label: "Sending domain", link: "/channels/sending-domain/" },
      { label: "Signatures", link: "/channels/email-signatures/" },
      { label: "Slack", link: "/channels/slack/" },
      { label: "Slack room modes", link: "/channels/slack-room-modes/" },
      { label: "WhatsApp", link: "/channels/whatsapp/" },
      { label: "Instagram and Messenger", link: "/channels/instagram-messenger/" },
    ],
  },
  {
    label: "dimmi agent",
    collapsed: true,
    items: [
      { label: "Overview", link: "/ai/how-the-agent-works/" },
      { label: "Setup", link: "/ai/set-up-the-agent/" },
      { label: "Knowledge", link: "/ai/knowledge-sources/" },
      { label: "Procedures", link: "/ai/procedures/" },
      { label: "Actions", link: "/ai/actions/" },
      { label: "First action", link: "/ai/first-action/" },
      { label: "Handover", link: "/ai/handover/" },
      { label: "AI providers", link: "/ai/providers/" },
      { label: "Customer data", link: "/ai/customer-data/" },
    ],
  },
  {
    label: "Operator",
    collapsed: true,
    items: [
      { label: "Overview", link: "/operator/work-with-operator/" },
      { label: "Changesets", link: "/operator/changesets/" },
    ],
  },
  {
    label: "Automation",
    collapsed: true,
    items: [
      { label: "Overview", link: "/workflows/how-workflows-work/" },
      { label: "Build a workflow", link: "/workflows/build-a-workflow/" },
      { label: "Triggers and steps", link: "/workflows/triggers-and-steps/" },
    ],
  },
  {
    label: "Customers",
    collapsed: true,
    items: [
      { label: "Overview", link: "/customers/customers-and-companies/" },
      { label: "Import", link: "/customers/import/" },
      { label: "Attributes and events", link: "/customers/attributes-and-events/" },
      { label: "Audiences", link: "/customers/audiences/" },
      { label: "Erase data", link: "/customers/erase-data/" },
    ],
  },
  {
    label: "Outbound",
    collapsed: true,
    items: [
      { label: "Broadcasts", link: "/outbound/broadcasts/" },
      { label: "Series", link: "/outbound/series/" },
      { label: "Topics", link: "/outbound/topics/" },
    ],
  },
  {
    label: "Help center",
    collapsed: true,
    items: [
      { label: "Setup", link: "/help-center/set-up/" },
      { label: "Custom domain", link: "/help-center/custom-domain/" },
      { label: "Restricted articles", link: "/help-center/limit-articles/" },
      { label: "Feature requests", link: "/help-center/feature-requests/" },
      { label: "Changelog", link: "/help-center/changelog/" },
    ],
  },
  {
    label: "Reports",
    collapsed: true,
    items: [
      { label: "Reports", link: "/reports/reports/" },
      { label: "Ratings", link: "/reports/conversation-ratings/" },
    ],
  },
  {
    label: "Workspace and security",
    collapsed: true,
    items: [
      { label: "Roles", link: "/workspace/roles-and-permissions/" },
      { label: "Single sign-on", link: "/workspace/single-sign-on/" },
      { label: "Security", link: "/workspace/security/" },
      { label: "Audit log", link: "/workspace/audit-log/" },
      { label: "Conversation safety", link: "/workspace/conversation-safety/" },
    ],
  },
  {
    label: "Build on dimmi",
    items: [
      { label: "Overview", link: "/developers/" },
      { label: "Install with AI", link: "/developers/ai-coding-agent/" },
    ],
  },
  {
    label: "Messenger",
    collapsed: true,
    items: [
      { label: "Install", link: "/developers/messenger/install/" },
      { label: "Identity verification", link: "/developers/messenger/identity-verification/" },
      { label: "JavaScript API", link: "/developers/messenger/javascript-api/" },
      { label: "Events", link: "/developers/messenger/track-events/" },
      { label: "CSP", link: "/developers/messenger/content-security-policy/" },
      { label: "Messenger API", link: "/developers/messenger/messenger-api/" },
    ],
  },
  {
    label: "REST API",
    collapsed: true,
    items: [
      { label: "Authentication", link: "/developers/api/authentication/" },
      { label: "Abilities", link: "/developers/api/abilities/" },
      { label: "Pagination", link: "/developers/api/pagination/" },
      { label: "Errors", link: "/developers/api/errors/" },
      { label: "Rate limits", link: "/developers/api/rate-limits/" },
      { label: "Idempotency", link: "/developers/api/idempotency/" },
      { label: "Versioning", link: "/developers/api/versioning/" },
    ],
  },
  {
    label: "SDKs",
    collapsed: true,
    items: [
      { label: "TypeScript", link: "/developers/sdks/typescript/" },
      { label: "Go", link: "/developers/sdks/go/" },
      { label: "PHP", link: "/developers/sdks/php/" },
      { label: "Laravel", link: "/developers/sdks/laravel/" },
      { label: "Python", link: "/developers/sdks/python/" },
      { label: "Versions", link: "/developers/sdks/supported-versions/" },
    ],
  },
  {
    label: "Webhooks",
    collapsed: true,
    items: [
      { label: "Receive", link: "/developers/webhooks/receive/" },
      { label: "Signatures", link: "/developers/webhooks/verify-signatures/" },
      { label: "Retries", link: "/developers/webhooks/delivery-and-retries/" },
      { label: "Events", link: "/developers/webhooks/events/" },
    ],
  },
  {
    label: "OAuth and MCP",
    collapsed: true,
    items: [
      { label: "OAuth apps", link: "/developers/oauth-apps/" },
      { label: "MCP", link: "/developers/mcp/" },
    ],
  },
  {
    label: "Your systems",
    collapsed: true,
    items: [
      { label: "Agent actions", link: "/developers/agent-actions-api/" },
      { label: "Workflow requests", link: "/developers/workflow-requests/" },
    ],
  },
  {
    label: "Install",
    items: [
      { label: "Requirements", link: "/self-hosting/" },
      { label: "Install", link: "/self-hosting/install/" },
    ],
  },
  {
    label: "Configure",
    collapsed: true,
    items: [
      { label: "Overview", link: "/self-hosting/configuration/" },
      { label: "Options", link: "/self-hosting/configuration-options/" },
      { label: "Email", link: "/self-hosting/email/" },
      { label: "AI providers", link: "/self-hosting/ai-providers/" },
      { label: "Sign-in providers", link: "/self-hosting/sign-in-providers/" },
      { label: "Integrations", link: "/self-hosting/integrations/" },
      { label: "Domains", link: "/self-hosting/domains/" },
    ],
  },
  {
    label: "Operate",
    collapsed: true,
    items: [
      { label: "Upgrade", link: "/self-hosting/upgrade/" },
      { label: "Backups", link: "/self-hosting/backups/" },
      { label: "Monitoring", link: "/self-hosting/monitoring/" },
      { label: "CLI", link: "/self-hosting/cli/" },
    ],
  },
];

// The header's tabs: each opens its first page and owns the sidebar
// groups listed, or, for the API reference, everything under its path.
export type Section = { key: string; label: string; href: string; groups: string[]; prefix?: string };

export const sections: Section[] = [
  {
    key: "guides",
    label: "Guides",
    href: "/",
    groups: [
      "Get started",
      "Inbox",
      "Tickets and SLAs",
      "Channels",
      "dimmi agent",
      "Operator",
      "Automation",
      "Customers",
      "Outbound",
      "Help center",
      "Reports",
      "Workspace and security",
    ],
  },
  {
    key: "developers",
    label: "Developers",
    href: "/developers/",
    groups: ["Build on dimmi", "Messenger", "REST API", "SDKs", "Webhooks", "OAuth and MCP", "Your systems"],
  },
  { key: "api", label: "API reference", href: "/api-reference/", groups: [], prefix: "/api-reference/" },
  { key: "self-hosting", label: "Self-hosting", href: "/self-hosting/", groups: ["Install", "Configure", "Operate"] },
];

const withSlash = (path: string) => (path.endsWith("/") ? path : `${path}/`);

export function sectionFor(path: string): Section {
  const current = withSlash(path);
  const byPrefix = sections.find((section) => section.prefix && current.startsWith(section.prefix));
  if (byPrefix) {
    return byPrefix;
  }
  const group = sidebar.find((g) => g.items.some((item) => withSlash(item.link) === current));
  return sections.find((section) => group && section.groups.includes(group.label)) ?? sections[0];
}

export const GITHUB_URL = "https://github.com/dimmihq/dimmi";
export const APP_URL = "https://trydimmi.com";
// The marketing site, which the docs are part of (served under /docs).
export const SITE_URL = "/";
