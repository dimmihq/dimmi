/*
 * Related links for API reference pages: the docs guide that explains the
 * area, the category index page, and up to 2 sibling operations in the same
 * category. Every guide href maps to a real page under docs/src/content/docs/.
 */
import { type ApiCategory, categoryByLabel, categoryHref, type Operation, operations } from "./spec";

export type RelatedGuide = {
  title: string;
  href: string;
  description: string;
};

export type RelatedCategory = {
  label: string;
  title: string;
  href: string;
  count: number;
};

export type OperationRelated = {
  guide: RelatedGuide;
  category: RelatedCategory;
  siblings: Operation[];
};

const GUIDES = {
  inbox: {
    title: "How the inbox works",
    href: "/inbox/how-the-inbox-works/",
    description: "See how conversations, replies, views, and team assignment fit together.",
  },
  conversationAttributes: {
    title: "Conversation attributes",
    href: "/inbox/conversation-attributes/",
    description: "Track structured fields on conversations filled by teammates, workflows, or AI.",
  },
  inboxes: {
    title: "Set up inboxes and assignment",
    href: "/inbox/inboxes-and-assignment/",
    description: "Route conversations into team inboxes with capacity limits and assignment rules.",
  },
  savedReplies: {
    title: "Use saved replies",
    href: "/inbox/saved-replies/",
    description: "Create reusable reply templates with variables and follow-up actions.",
  },
  officeHours: {
    title: "Set office hours and availability",
    href: "/inbox/office-hours/",
    description: "Set workspace schedules used by SLA timers and the dimmi agent.",
  },
  ratings: {
    title: "Ask for conversation ratings",
    href: "/reports/conversation-ratings/",
    description: "Collect customer satisfaction ratings after a conversation closes.",
  },
  slaPolicies: {
    title: "Set SLA policies",
    href: "/tickets/sla-policies/",
    description: "Set first-reply and resolution targets for conversations and tickets.",
  },
  conversationSafety: {
    title: "Protect conversations",
    href: "/workspace/conversation-safety/",
    description: "Redact sensitive data and protect customer threads automatically.",
  },
  customers: {
    title: "Customers and companies",
    href: "/customers/customers-and-companies/",
    description: "Manage customer profiles, company records, and shared account history.",
  },
  attributesAndEvents: {
    title: "Use attributes and events",
    href: "/customers/attributes-and-events/",
    description: "Store custom customer fields and track product events for targeting.",
  },
  audiences: {
    title: "Build audiences",
    href: "/customers/audiences/",
    description: "Group customers dynamically by attributes, company data, and events.",
  },
  eraseData: {
    title: "Erase a customer's data",
    href: "/customers/erase-data/",
    description: "Delete customer records and conversation history for privacy requests.",
  },
  importCustomers: {
    title: "Import customers from a CSV",
    href: "/customers/import/",
    description: "Bulk import contacts and companies into your dimmi workspace.",
  },
  tickets: {
    title: "How tickets work",
    href: "/tickets/how-tickets-work/",
    description: "Track customer tickets, internal escalations, and shared incident trackers.",
  },
  ticketTypes: {
    title: "Set up ticket types and fields",
    href: "/tickets/ticket-types/",
    description: "Define custom ticket types, fields, and states for your support queue.",
  },
  linkedIssues: {
    title: "Link Linear, Jira, and GitHub issues",
    href: "/tickets/linked-issues/",
    description: "Connect support tickets directly to engineering issues and status updates.",
  },
  aiAgent: {
    title: "How the dimmi agent works",
    href: "/ai/how-the-agent-works/",
    description: "See how the dimmi agent answers from docs, follows procedures, and hands over.",
  },
  aiActions: {
    title: "Connect actions to your systems",
    href: "/ai/actions/",
    description: "Let the dimmi agent call HTTP APIs and MCP tools with approval rules.",
  },
  multipleAgents: {
    title: "Run several AI agents",
    href: "/ai/multiple-agents/",
    description: "Set up specialized AI agents for different channels, brands, or queues.",
  },
  aiProviders: {
    title: "Choose an AI provider and spending cap",
    href: "/ai/providers/",
    description: "Connect your own model keys and set AI provider preferences.",
  },
  knowledgeSources: {
    title: "Add knowledge sources",
    href: "/ai/knowledge-sources/",
    description: "Sync help center articles, websites, files, Notion, and GitHub for AI answers.",
  },
  procedures: {
    title: "Write procedures",
    href: "/ai/procedures/",
    description: "Guide the dimmi agent through requests step by step using rules in plain English.",
  },
  operator: {
    title: "Work with Operator",
    href: "/operator/work-with-operator/",
    description: "Brief teammates, draft replies, and review workspace changesets with Operator.",
  },
  helpCenter: {
    title: "Set up a help center",
    href: "/help-center/set-up/",
    description: "Publish collections, articles, themes, and search on your help center.",
  },
  helpCenterLanguages: {
    title: "Translate your help center",
    href: "/help-center/languages/",
    description: "Manage localized collections and article translations in multiple languages.",
  },
  identityVerification: {
    title: "Verify customer identity",
    href: "/developers/messenger/identity-verification/",
    description: "Sign customer IDs on your server so messenger sessions cannot be spoofed.",
  },
  customizeMessenger: {
    title: "Customize the messenger",
    href: "/channels/customize-the-messenger/",
    description: "Style the launcher, greeting, colors, and behavior of the embedded messenger.",
  },
  installMessenger: {
    title: "Add the messenger to your app",
    href: "/developers/messenger/install/",
    description: "Embed the dimmi messenger in web apps with JavaScript, React, or Next.js.",
  },
  broadcasts: {
    title: "Send a broadcast",
    href: "/outbound/broadcasts/",
    description: "Send targeted outbound messages to an audience and handle replies in the inbox.",
  },
  topics: {
    title: "Manage topics and subscriptions",
    href: "/outbound/topics/",
    description: "Let customers opt in or out of product updates, tips, and announcements.",
  },
  series: {
    title: "Build a series",
    href: "/outbound/series/",
    description: "Automate scheduled onboarding and follow-up messages across channels.",
  },
  supportEmail: {
    title: "Connect a support email address",
    href: "/channels/support-email/",
    description: "Forward support email into dimmi and manage bounce suppressions.",
  },
  sendingDomain: {
    title: "Send email from your own domain",
    href: "/channels/sending-domain/",
    description: "Set up DNS records and DKIM keys to send replies from your custom domain.",
  },
  emailSignatures: {
    title: "Set email signatures and reply style",
    href: "/channels/email-signatures/",
    description: "Control teammate signatures and outbound email formatting.",
  },
  changelog: {
    title: "Publish a changelog",
    href: "/help-center/changelog/",
    description: "Share product release notes in your help center, messenger, and email updates.",
  },
  featureRequests: {
    title: "Run a feature requests board",
    href: "/help-center/feature-requests/",
    description: "Collect customer ideas, votes, statuses, and linked conversations.",
  },
  workflows: {
    title: "How workflows work",
    href: "/workflows/how-workflows-work/",
    description: "Automate routing, tagging, SLA updates, and HTTP requests without code.",
  },
  developers: {
    title: "Ways to build on dimmi",
    href: "/developers/",
    description: "Explore SDKs, webhooks, OAuth apps, MCP, and API integrations.",
  },
  webhooksReceive: {
    title: "Receive webhooks",
    href: "/developers/webhooks/receive/",
    description: "Register webhook endpoints and verify signed payloads from dimmi.",
  },
  webhooksDelivery: {
    title: "Webhook delivery and retries",
    href: "/developers/webhooks/delivery-and-retries/",
    description: "Inspect webhook delivery logs, timeouts, and automatic retry schedules.",
  },
  webhooksEvents: {
    title: "Webhook events",
    href: "/developers/webhooks/events/",
    description: "Browse every webhook event type and payload emitted by dimmi.",
  },
  reports: {
    title: "What each report measures",
    href: "/reports/reports/",
    description: "Track reply times, resolution volume, AI coverage, and SLA performance.",
  },
  auditLog: {
    title: "Audit log",
    href: "/workspace/audit-log/",
    description: "Review a full log of workspace settings and security changes.",
  },
  security: {
    title: "Secure your workspace",
    href: "/workspace/security/",
    description: "Manage workspace security controls, data exports, and retention settings.",
  },
  roles: {
    title: "Roles and permissions",
    href: "/workspace/roles-and-permissions/",
    description: "Manage teammates, teams, invitations, and role permissions.",
  },
  sso: {
    title: "Set up single sign-on",
    href: "/workspace/single-sign-on/",
    description: "Connect SAML or OIDC single sign-on for your workspace domain.",
  },
  authentication: {
    title: "Authenticate API requests",
    href: "/developers/api/authentication/",
    description: "Create API tokens, pass Bearer headers, and check token abilities.",
  },
  migrations: {
    title: "Migrations API",
    href: "/developers/migrations/migrations-api/",
    description: "Import conversations, contacts, companies, and articles from other helpdesks.",
  },
} satisfies Record<string, RelatedGuide>;

const RESOURCE_GUIDES: Record<string, RelatedGuide> = {
  conversations: GUIDES.inbox,
  "conversation-bulk-changes": GUIDES.inbox,
  "conversation-attributes": GUIDES.conversationAttributes,
  "conversation-rating-settings": GUIDES.ratings,
  inboxes: GUIDES.inboxes,
  "inbox-preferences": GUIDES.inboxes,
  "saved-replies": GUIDES.savedReplies,
  "saved-reply-imports": GUIDES.savedReplies,
  schedules: GUIDES.officeHours,
  "sla-policies": GUIDES.slaPolicies,
  "redaction-rules": GUIDES.conversationSafety,
  tags: GUIDES.inbox,
  contacts: GUIDES.customers,
  "contact-lists": GUIDES.customers,
  "contact-notes": GUIDES.customers,
  companies: GUIDES.customers,
  "attribute-definitions": GUIDES.attributesAndEvents,
  "attribute-problems": GUIDES.attributesAndEvents,
  "contact-events": GUIDES.attributesAndEvents,
  "event-definitions": GUIDES.attributesAndEvents,
  "event-settings": GUIDES.attributesAndEvents,
  audiences: GUIDES.audiences,
  "contact-erasures": GUIDES.eraseData,
  imports: GUIDES.importCustomers,
  tickets: GUIDES.tickets,
  "ticket-events": GUIDES.tickets,
  "ticket-fields": GUIDES.ticketTypes,
  "ticket-settings": GUIDES.tickets,
  "ticket-states": GUIDES.ticketTypes,
  "ticket-types": GUIDES.ticketTypes,
  "ticket-issues": GUIDES.linkedIssues,
  "ticket-links": GUIDES.linkedIssues,
  "ai/actions": GUIDES.aiActions,
  "ai/action-calls": GUIDES.aiActions,
  "ai/action-connections": GUIDES.aiActions,
  "ai/agents": GUIDES.multipleAgents,
  "ai/agent-templates": GUIDES.multipleAgents,
  "ai/placements": GUIDES.multipleAgents,
  "ai/conversation-attributes": GUIDES.conversationAttributes,
  "ai/keys": GUIDES.aiProviders,
  "ai/settings": GUIDES.aiAgent,
  "ai/usage": GUIDES.aiAgent,
  knowledge: GUIDES.knowledgeSources,
  procedures: GUIDES.procedures,
  operator: GUIDES.operator,
  articles: GUIDES.helpCenter,
  collections: GUIDES.helpCenter,
  "help-centers": GUIDES.helpCenter,
  "help-center-languages": GUIDES.helpCenterLanguages,
  "identity-secrets": GUIDES.identityVerification,
  "identity-verification": GUIDES.identityVerification,
  "widget-settings": GUIDES.customizeMessenger,
  broadcasts: GUIDES.broadcasts,
  "broadcast-topics": GUIDES.topics,
  series: GUIDES.series,
  "email-addresses": GUIDES.supportEmail,
  "email-suppressions": GUIDES.supportEmail,
  "email-domains": GUIDES.sendingDomain,
  "email-signatures": GUIDES.emailSignatures,
  "changelog-entries": GUIDES.changelog,
  "changelog-labels": GUIDES.changelog,
  "changelog-subscribers": GUIDES.changelog,
  "feedback-boards": GUIDES.featureRequests,
  "feedback-comments": GUIDES.featureRequests,
  "feedback-posts": GUIDES.featureRequests,
  "feedback-statuses": GUIDES.featureRequests,
  workflows: GUIDES.workflows,
  channels: GUIDES.developers,
  integrations: GUIDES.developers,
  "integration-installations": GUIDES.developers,
  "integration-providers": GUIDES.developers,
  "webhook-endpoints": GUIDES.webhooksReceive,
  "webhook-deliveries": GUIDES.webhooksDelivery,
  "webhook-event-types": GUIDES.webhooksEvents,
  reports: GUIDES.reports,
  "audit-logs": GUIDES.auditLog,
  "data-exports": GUIDES.security,
  invitations: GUIDES.roles,
  members: GUIDES.roles,
  permissions: GUIDES.roles,
  roles: GUIDES.roles,
  teams: GUIDES.roles,
  workspace: GUIDES.security,
  tokens: GUIDES.authentication,
  "organization-domains": GUIDES.sso,
  me: GUIDES.authentication,
  migrations: GUIDES.migrations,
};

const CATEGORY_GUIDES: Record<string, RelatedGuide> = {
  Conversations: GUIDES.inbox,
  "Contacts and companies": GUIDES.customers,
  Tickets: GUIDES.tickets,
  "AI agent": GUIDES.aiAgent,
  "Help center": GUIDES.helpCenter,
  Messenger: GUIDES.installMessenger,
  "Broadcasts and series": GUIDES.broadcasts,
  Email: GUIDES.supportEmail,
  "Feedback and changelog": GUIDES.featureRequests,
  "Workflows and integrations": GUIDES.workflows,
  Webhooks: GUIDES.webhooksReceive,
  "Reports and data": GUIDES.reports,
  "Workspace and team": GUIDES.roles,
  Notifications: GUIDES.inbox,
  Account: GUIDES.authentication,
  More: GUIDES.migrations,
};

const operationsByCategory = new Map<string, Operation[]>();
for (const op of operations) {
  const list = operationsByCategory.get(op.category) ?? [];
  list.push(op);
  operationsByCategory.set(op.category, list);
}

export function guideForCategory(category: ApiCategory): RelatedGuide {
  return CATEGORY_GUIDES[category.label] ?? GUIDES.developers;
}

function siblingOperations(op: Operation): Operation[] {
  const group = operationsByCategory.get(op.category) ?? [];
  if (group.length <= 1) {
    return [];
  }
  const index = group.findIndex((item) => item.id === op.id);
  if (index === -1) {
    return group.slice(0, 2);
  }
  if (group.length === 2) {
    return [group[(index + 1) % 2]];
  }
  const firstIndex = (index + 1) % group.length;
  const secondIndex = index > 0 ? index - 1 : (index + 2) % group.length;
  return [group[firstIndex], group[secondIndex]];
}

export function relatedFor(op: Operation): OperationRelated {
  const guide = RESOURCE_GUIDES[op.resource] ?? CATEGORY_GUIDES[op.category] ?? GUIDES.developers;
  const cat = categoryByLabel(op.category);
  const category: RelatedCategory = cat
    ? { label: cat.label, title: cat.title, href: cat.href, count: cat.operations.length }
    : {
        label: op.category,
        title: `${op.category} API`,
        href: categoryHref(op.category),
        count: (operationsByCategory.get(op.category) ?? []).length,
      };
  return {
    guide,
    category,
    siblings: siblingOperations(op),
  };
}
