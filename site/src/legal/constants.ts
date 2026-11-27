/*
 * Single source of truth for dimmi Cloud's legal documents: entity details,
 * official contact addresses, effective date, subprocessors, security
 * controls, cookies, and retention periods. Sharing these tables keeps
 * /terms and /privacy consistent on every build.
 */

export const LEGAL_ENTITY = {
  name: "Hivy Technologies Limited",
  rcNumber: "RC 9601398",
  incorporationDate: "June 9, 2026",
  incorporationAct: "Companies and Allied Matters Act 2020",
  country: "Federal Republic of Nigeria",
  address: "Unit B1, MIJL Residence and Villa, Ikate, Lagos State, Nigeria",
  productName: "dimmi",
  serviceName: "dimmi Cloud",
  website: "https://trydimmi.com",
} as const;

export const LEGAL_CONTACTS = {
  support: "support@trydimmi.com",
} as const;

export const LEGAL_VERSION = {
  version: "1.0",
  effectiveDate: "December 1, 2026",
} as const;

export type Subprocessor = {
  vendor: string;
  entity: string;
  category: "Infrastructure & email" | "AI & search" | "Billing";
  purpose: string;
  dataReceived: string;
  location: string;
  appliesWhen: string;
  retentionAndTraining: string;
  termsUrl: string;
};

export const SUBPROCESSORS: Subprocessor[] = [
  {
    vendor: "Hetzner",
    entity: "Hetzner Online GmbH (Germany)",
    category: "Infrastructure & email",
    purpose:
      "Dedicated application servers, PostgreSQL database, file storage, cache, malware scanning, load balancing, and database backups.",
    dataReceived: "All workspace data, account data, attachments, and database backups.",
    location: "Falkenstein, Germany (primary servers); Nuremberg, Germany (database backups).",
    appliesWhen: "Always (every dimmi Cloud workspace).",
    retentionAndTraining:
      "No training. Stores data only until deleted by Customer or dimmi; daily database backups roll off after 30 days.",
    termsUrl: "https://www.hetzner.com/legal/terms-and-conditions/",
  },
  {
    vendor: "Amazon Web Services",
    entity: "Amazon Web Services EMEA SARL (Luxembourg)",
    category: "Infrastructure & email",
    purpose:
      "Sending and receiving support email, broadcasts, series, and account notifications (Amazon SES, S3, and SNS).",
    dataReceived:
      "Email headers, sender and recipient addresses, message bodies, attachments, and delivery, bounce, or complaint events.",
    location: "Frankfurt, Germany (AWS Europe eu-central-1).",
    appliesWhen: "Email channel, broadcasts, series, and transactional account emails.",
    retentionAndTraining:
      "No model training. AWS SES scans email in transit for spam and malware. Raw inbound email files in S3 expire after 31 days.",
    termsUrl: "https://aws.amazon.com/agreement/",
  },
  {
    vendor: "Cloudflare",
    entity: "Cloudflare, Inc. (United States)",
    category: "Infrastructure & email",
    purpose: "Public website crawling for knowledge imports; Turnstile bot verification on sign-up and sign-in pages.",
    dataReceived:
      "Public web page content submitted for import (Crawler); team member IP address and browser signals (Turnstile, Account Data only).",
    location: "United States and Cloudflare global edge network.",
    appliesWhen: "Website knowledge imports (Crawler); dashboard sign-up and sign-in (Turnstile).",
    retentionAndTraining:
      "Crawler: public pages only, no generative AI training, crawl results retained up to 14 days. Turnstile: Cloudflare also uses sign-up signals (not Customer Data) as an independent controller to improve its bot detection.",
    termsUrl: "https://www.cloudflare.com/terms/",
  },
  {
    vendor: "Anthropic",
    entity: "Anthropic, PBC (United States)",
    category: "AI & search",
    purpose: "Language model inference for dimmi agent replies, procedures, summaries, and Operator.",
    dataReceived:
      "Conversation messages (excluding internal notes for customer-facing agents), matched knowledge passages, tool/action results, and configured contact fields (first name and locale by default, with PII masking enabled by default).",
    location: "United States (stored in the United States; inference may run in other regions).",
    appliesWhen:
      "AI features enabled using dimmi's built-in models (not used when Customer supplies its own model key for another provider).",
    retentionAndTraining:
      "No training on Customer Data. Requests are retained for up to 30 days under standard commercial API terms, then deleted; content flagged for trust and safety or abuse violations may be retained by Anthropic for up to 2 years.",
    termsUrl: "https://www.anthropic.com/legal/commercial-terms",
  },
  {
    vendor: "OpenAI",
    entity: "OpenAI OpCo, LLC (United States)",
    category: "AI & search",
    purpose: "Language model inference for dimmi agent replies, procedures, summaries, and Operator.",
    dataReceived:
      "Conversation messages (excluding internal notes for customer-facing agents), matched knowledge passages, tool/action results, and configured contact fields (first name and locale by default, with PII masking enabled by default).",
    location: "United States (global inference routing by default).",
    appliesWhen:
      "AI features enabled using dimmi's built-in models (not used when Customer supplies its own model key for another provider).",
    retentionAndTraining:
      "No training on Customer Data. Requests are retained for up to 30 days for abuse monitoring under standard API terms, then deleted.",
    termsUrl: "https://openai.com/policies/services-agreement/",
  },
  {
    vendor: "Voyage AI",
    entity: "Voyage AI Innovations, Inc. (United States)",
    category: "AI & search",
    purpose:
      "Text embeddings and reranking for semantic search across help articles, knowledge sources, and closed conversations.",
    dataReceived:
      "Help articles, uploaded knowledge documents, crawled public pages, closed-conversation text, and search queries.",
    location: "United States.",
    appliesWhen: "AI and semantic search features enabled.",
    retentionAndTraining:
      "Opted out of model training and data retention; submitted inputs are deleted immediately after processing.",
    termsUrl: "https://www.voyageai.com/tos",
  },
  {
    vendor: "Cloudflare Workers AI",
    entity: "Cloudflare, Inc. (United States)",
    category: "AI & search",
    purpose: "Clef decision model: yes-or-no checks on draft replies and procedure steps.",
    dataReceived: "Draft replies, conversation excerpts, and the check questions.",
    location: "United States and Cloudflare global network.",
    appliesWhen: "AI features enabled.",
    retentionAndTraining:
      "Zero data retention and no training on Customer Data, per Cloudflare's Workers AI data terms.",
    termsUrl: "https://developers.cloudflare.com/workers-ai/platform/data-usage/",
  },
  {
    vendor: "Paystack",
    entity: "Paystack Payments Limited (Nigeria)",
    category: "Billing",
    purpose:
      "Payment card processing, recurring billing, invoicing, and payment fraud checks for dimmi Cloud usage charges.",
    dataReceived:
      "Workspace billing contact name, email address, payment method details, transaction references, and billing metadata (Account Data only; never receives End Customer support conversations).",
    location: "Nigeria and PCI-DSS certified payment processing infrastructure.",
    appliesWhen:
      "Paid dimmi Cloud workspaces (conversation 101+ in a month, email broadcasts, or Order Form customers).",
    retentionAndTraining:
      "No AI training. Retained as required for payment processing, chargeback defense, and Nigerian financial and tax regulations.",
    termsUrl: "https://paystack.com/terms",
  },
];

export type ThirdPartyService = {
  name: string;
  entity: string;
  role: string;
  whatItReceives: string;
  appliesWhen: string;
};

export const OTHER_THIRD_PARTY_SERVICES: ThirdPartyService[] = [
  {
    name: "Public Certificate Authority (Let's Encrypt)",
    entity: "Internet Security Research Group (United States)",
    role: "Independent certificate authority issuing TLS certificates for custom help center domains.",
    whatItReceives:
      "Custom domain names and ACME validation requests. Issued domains are recorded in public Certificate Transparency logs.",
    appliesWhen: "Only when a workspace configures a custom domain for its help center.",
  },
  {
    name: "Browser Push Relay Services",
    entity: "Google LLC, Apple Inc., and Mozilla Corporation",
    role: "Independent messaging relays delivering encrypted web push notifications to subscribed browsers.",
    whatItReceives: "Encrypted push payloads and browser subscription endpoints.",
    appliesWhen: "Only when a team member opts into browser push notifications.",
  },
];

export type CustomerPlatform = {
  platform: string;
  purpose: string;
  relationship: string;
};

export const CUSTOMER_CONNECTED_PLATFORMS: CustomerPlatform[] = [
  {
    platform: "Meta (WhatsApp Business Platform, Facebook Messenger, Instagram)",
    purpose: "Receives and sends messages on the Customer's connected business accounts.",
    relationship: "Customer contracts directly with Meta and instructs dimmi to exchange messages through Meta's APIs.",
  },
  {
    platform: "Slack",
    purpose: "Syncs Customer-selected Slack channels, notifications, and Operator interactions.",
    relationship:
      "Customer contracts directly with Slack and authorizes dimmi's Slack app to access selected workspaces and channels.",
  },
  {
    platform: "Google & GitHub Sign-In",
    purpose: "Authenticates team members who choose single sign-in with their Google or GitHub account.",
    relationship:
      "Used solely for authentication; dimmi receives account ID, name, email, and avatar URL and retains no provider access tokens.",
  },
  {
    platform:
      "Knowledge & Issue Integrations (Notion, GitHub, Linear, Jira, Intercom, Zendesk, PagerDuty, incident.io, Rootly)",
    purpose: "Imports help content or links support tickets and outage trackers to Customer's own tools.",
    relationship: "Customer connects its own accounts and warrants it holds the right to import or sync the data.",
  },
  {
    platform: "Customer's Own AI Provider Keys (BYO Key)",
    purpose: "Routes AI agent and Operator inference directly through Customer's own API account.",
    relationship:
      "That AI provider is Customer's direct vendor under Customer's own agreement, not a dimmi subprocessor.",
  },
  {
    platform: "Customer Webhooks, AI Actions, MCP Clients & SAML/OIDC Identity Providers",
    purpose: "Sends event payloads, executes HTTP/MCP calls to Customer endpoints, or authenticates teammates.",
    relationship: "Operated or authorized directly by Customer; Customer controls what data is transmitted.",
  },
];

export type RetentionRow = {
  category: string;
  scope: "Controller" | "Processor";
  period: string;
  notes: string;
};

export const RETENTION_SCHEDULE: RetentionRow[] = [
  {
    category: "Team member account profile (name, email, password hash, security credentials)",
    scope: "Controller",
    period: "While account is active",
    notes: "Deleted when the account or last workspace is deleted, subject to billing and statutory records.",
  },
  {
    category: "Team member login sessions",
    scope: "Controller",
    period: "14 days idle, 30 days maximum",
    notes: "Workspace security rules may enforce shorter session and idle limits.",
  },
  {
    category: "Workspace audit log and security event log",
    scope: "Controller",
    period: "365 days",
    notes: "Records actions, IP address, and user agent for security accountability.",
  },
  {
    category: "Invitations and revoked or expired access tokens",
    scope: "Controller",
    period: "30 days after expiry or revocation",
    notes: "Purged automatically on a scheduled job.",
  },
  {
    category: "Billing records, invoices, and tax documentation",
    scope: "Controller",
    period: "6 years (or statutory minimum)",
    notes: "Retained to satisfy Nigerian tax and accounting obligations under the Nigeria Tax Act 2025 and CAMA 2020.",
  },
  {
    category: "Changelog subscriber email addresses",
    scope: "Controller",
    period: "Until unsubscribed",
    notes: "Collected via double opt-in; deleted or suppressed immediately upon unsubscribe.",
  },
  {
    category: "Dashboard session recordings (signed-in teammates)",
    scope: "Controller",
    period: "30 days",
    notes:
      "Masked in the browser and kept on dimmi's own servers in the EU. Recordings we star to fix a bug stay until unstarred.",
  },
  {
    category: "Server access logs and application traces",
    scope: "Controller",
    period: "14 to 30 days",
    notes: "Operational diagnostics and security monitoring on Hetzner servers.",
  },
  {
    category: "Conversations, messages, contacts, tickets, knowledge, and help articles",
    scope: "Processor",
    period: "Until deleted by Customer or workspace deletion",
    notes: "Customer controls retention periods in workspace settings.",
  },
  {
    category: "Trash and spam conversations",
    scope: "Processor",
    period: "Per Customer's configured trash and spam retention days",
    notes:
      "Conversations, messages, attachments, and linked email records are purged automatically after the configured period.",
  },
  {
    category: "Deleted workspace",
    scope: "Processor",
    period: "30 days restorable, then permanently purged",
    notes: "Purge removes database rows, raw email files, help center images, and knowledge uploads.",
  },
  {
    category: "Erased contact (data subject erasure request)",
    scope: "Processor",
    period: "Hidden immediately, wiped after 7 days",
    notes:
      "Deletes inbound email rows and raw files, blanks outbound recipients and subjects, removes suppressions, and keeps unsubscribe/complaint proofs only as workspace-salted SHA-256 hashes.",
  },
  {
    category: "Full AI provider requests and responses (debugging copies)",
    scope: "Processor",
    period: "30 days",
    notes: "Stored in dimmi's EU database for troubleshooting; separate from AI vendor retention.",
  },
  {
    category: "AI run metadata records",
    scope: "Processor",
    period: "400 days",
    notes: "Operational metrics and run summaries.",
  },
  {
    category: "Raw chat-app webhook payloads (Meta, Slack)",
    scope: "Processor",
    period: "7 days (30 days if processing failed)",
    notes: "Automatically purged once processed or expired.",
  },
  {
    category: "Outbound webhook delivery logs",
    scope: "Processor",
    period: "30 days",
    notes: "Kept for retry tracking and customer debugging.",
  },
  {
    category: "Raw inbound email files (.eml)",
    scope: "Processor",
    period: "31 days in AWS S3; per Customer's raw email setting in dimmi",
    notes: "Unlinked ignored, failed, or quarantined email records are deleted with their file.",
  },
  {
    category: "Anonymous web messenger sessions",
    scope: "Processor",
    period: "180 days",
    notes: "Purged automatically once expired.",
  },
  {
    category: "Help center article views and search terms",
    scope: "Processor",
    period: "400 days (views); 90 days (search terms)",
    notes: "Used for workspace help center analytics.",
  },
  {
    category: "Generated export files",
    scope: "Processor",
    period: "7 days (workspace data exports); 24 hours (contact exports)",
    notes: "Delivered via expiring signed download URLs.",
  },
  {
    category: "PostgreSQL database backups and WAL archives",
    scope: "Processor",
    period: "30 days",
    notes: "Stored in Hetzner Object Storage (Nuremberg, Germany) and purged automatically on a 30-day rolling window.",
  },
];

export type SecurityMeasureGroup = {
  area: string;
  controls: string[];
};

export const SECURITY_MEASURES: SecurityMeasureGroup[] = [
  {
    area: "Transport & credential encryption",
    controls: [
      "TLS encryption enforced on all public endpoints, custom help center domains, APIs, and database connections.",
      "Webhooks signed with HMAC verification using per-workspace secrets.",
      "Sensitive workspace credentials (BYO AI API keys, Meta/Slack/integration OAuth tokens, webhook signing keys, and two-factor authentication secrets) encrypted at the application layer with per-workspace encryption keys.",
      "Kubernetes cluster secrets encrypted at rest (k3s secrets-encryption enabled).",
    ],
  },
  {
    area: "Access control & identity",
    controls: [
      "Passwords hashed with Argon2id; passkeys (WebAuthn) and authenticator-app two-factor authentication (2FA) supported for every account.",
      "Workspace-enforced 2FA blocks dashboard and API token access until teammates enroll.",
      "SAML 2.0 and OIDC single sign-on (SSO), sign-in method restrictions, Google domain restrictions, and configurable session and idle timeouts.",
      "Separate Dashboard IP allowlists and API IP allowlists (up to 100 single IPs or CIDR ranges each).",
      "Three built-in roles plus granular custom roles and scoped API token abilities.",
    ],
  },
  {
    area: "Multi-tenant isolation",
    controls: [
      "Every tenant data row carries a workspace_id and PostgreSQL Row-Level Security (RLS) policies are applied to tenant tables.",
      "Per-workspace encryption keys isolate stored integration credentials and AI keys.",
    ],
  },
  {
    area: "Infrastructure, network & backups",
    controls: [
      "Primary compute, PostgreSQL database, SeaweedFS object storage, Valkey cache, and ClamAV malware scanner run on dedicated Hetzner Online GmbH bare-metal infrastructure in Falkenstein, Germany. Hetzner Online GmbH maintains ISO/IEC 27001 certification for its data centers.",
      "Default-deny host firewall, isolated private VLAN networking, and automated operating system security updates.",
      "Nightly full PostgreSQL base backups plus continuous Write-Ahead Log (WAL) archiving to Hetzner Object Storage in Nuremberg, Germany, retained for 30 days.",
    ],
  },
  {
    area: "Malware, content & abuse controls",
    controls: [
      "Every uploaded attachment is scanned for malware using ClamAV running on our own EU servers (if the scanner fails repeatedly, delivery fails open to preserve availability).",
      "File downloads are served through short-lived, cryptographically signed URLs.",
      "Payment card numbers are automatically masked before storage by default, and workspaces can define custom redaction rules.",
      "Default PII masking (Hide personal data) strips emails, phone numbers, card numbers, bank account numbers, and secrets before prompts are sent to built-in AI models.",
      "Sign-in lockout protection, API rate limiting, sign-up rate limits, and Cloudflare Turnstile bot checks.",
    ],
  },
  {
    area: "Logging & incident response",
    controls: [
      "Workspace audit logs and user security event logs record administrative changes, access rule updates, IP addresses, and user agents, retained for 365 days.",
      "A 72-hour customer notification commitment under the DPA, plus statutory breach notification clocks for the Nigeria Data Protection Commission (NDPC) and national CERT where applicable.",
    ],
  },
];

export type CookieEntry = {
  surface: string;
  name: string;
  mechanism: "HTTP Cookie" | "Local Storage" | "Session Storage";
  purpose: string;
  lifetime: string;
  category: "Strictly necessary" | "Preference" | "Diagnostics";
};

export const COOKIES_AND_STORAGE: CookieEntry[] = [
  {
    surface: "dimmi Cloud dashboard",
    name: "__Host-dimmi_session",
    mechanism: "HTTP Cookie",
    purpose: "Authenticates your signed-in workspace session (first-party, Secure, HttpOnly).",
    lifetime: "14 days idle, up to 30 days maximum (or shorter per workspace security rules)",
    category: "Strictly necessary",
  },
  {
    surface: "dimmi Cloud dashboard",
    name: "__Host-dimmi_device",
    mechanism: "HTTP Cookie",
    purpose: "Recognizes a previously verified device for sign-in security alerts and two-factor flows.",
    lifetime: "365 days",
    category: "Strictly necessary",
  },
  {
    surface: "dimmi Cloud dashboard",
    name: "Short-lived auth flow cookies",
    mechanism: "HTTP Cookie",
    purpose: "Protects CSRF state and OAuth, SAML, magic-link, and passkey sign-in challenges.",
    lifetime: "Approximately 10 minutes",
    category: "Strictly necessary",
  },
  {
    surface: "dimmi Cloud dashboard",
    name: "dimmi_theme & workspace UI state",
    mechanism: "HTTP Cookie",
    purpose:
      "Remembers light/dark theme preference; browser local storage holds unsent message drafts and layout state.",
    lifetime: "Persistent until cleared",
    category: "Preference",
  },
  {
    surface: "dimmi Cloud dashboard",
    name: "kimmi.session & kimmi.halted",
    mechanism: "Session Storage",
    purpose: "Joins a signed-in teammate's masked session recording across page loads.",
    lifetime: "Until the tab closes",
    category: "Diagnostics",
  },
  {
    surface: "dimmi Cloud dashboard",
    name: "kimmi.anonymousId & kimmi.identity",
    mechanism: "Local Storage",
    purpose:
      "Holds the recorder's random browser ID and the signed-in teammate's dimmi user ID, name, email, and workspace.",
    lifetime: "Persistent until cleared",
    category: "Diagnostics",
  },
  {
    surface: "Customer help center",
    name: "dimmi_help_reader",
    mechanism: "HTTP Cookie",
    purpose: "Keeps an authenticated end customer signed into a restricted help center.",
    lifetime: "7 days",
    category: "Strictly necessary",
  },
  {
    surface: "Web messenger (on customer sites)",
    name: "Session token, message outbox, last-seen marker",
    mechanism: "Local Storage",
    purpose:
      "Maintains an end customer's conversation across page loads and retries unsent messages. The web messenger sets no cookies.",
    lifetime: "Up to 180 days (or until browser storage is cleared)",
    category: "Strictly necessary",
  },
  {
    surface: "trydimmi.com website & docs",
    name: "Theme preference",
    mechanism: "Local Storage",
    purpose: "Remembers light or dark appearance across pages. No analytics or advertising cookies are set.",
    lifetime: "Persistent until cleared",
    category: "Preference",
  },
];
