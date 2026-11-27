/*
 * Single source of truth for competitor plans, verified prices, add-ons,
 * switching triggers, and honest strengths. `site/src/pricing/competitors.ts`
 * re-exports the pricing projections from here so `/pricing` and `/vs/*`
 * always compute identical numbers.
 */
import type { Usage } from "../pricing/prices";

export const CHECKED_ON = "27 November 2026";
export const CHECKED_ON_ISO = "2026-11-27";

/* The share of AI-answered conversations a competitor counts as resolved. */
export const RESOLUTION_RATE = 0.6;

export type RivalAiUnit = "resolution" | "outcome" | "conversation" | "credit" | "session" | "included";

export type RivalPlan = {
  name: string;
  /** List price per seat per month (null for flat workspace plans). */
  seatPrice: number | null;
  /** Optional new-customer or annual promotional seat price. */
  promoSeatPrice?: number | null;
  promoNote?: string;
  /** Flat monthly workspace price when seatPrice is null. */
  flatMonthly?: number | null;
  includedSeats?: number;
  extraSeatPrice?: number;
  includedAi?: number;
  billingNote: string;
  highlights: string[];
  missing?: string[];
};

export type RivalAddOn = {
  name: string;
  price: number;
  unit: "seat/mo" | "workspace/mo" | "per 100 sessions" | "per 1,000 credits" | "flat/mo";
  note: string;
};

export type RivalFact = {
  label: string;
  rival: string;
  dimmi: string;
  source: string;
  checkedOn: string;
};

export type RivalSwitchTrigger = {
  trigger: string;
  rivalReality: string;
  dimmiReality: string;
  sourceLabel: string;
  sourceUrl: string;
};

export type RivalTeamPick = {
  profile: string;
  seats: number;
  conversations: number;
  aiShare: number;
  pick: "dimmi" | "rival";
  reason: string;
  rivalPlanLabel: string;
};

export type Competitor = {
  name: string;
  seat: (seats: number) => number;
  ai: { per: "resolution" | "conversation"; price: number; includedPerSeat?: number };
  basis: string;
  gated: string;
  full: { plan: string; seat: number; adds: string; includedPerSeat?: number };
  source: string;
};

export type Rival = {
  slug: string;
  name: string;
  tone: string;
  pricingUrl: string;
  checkedOn: string;
  checkedOnLabel: string;
  aiUnit: RivalAiUnit;
  aiPrice: number;
  aiNote: string;
  plans: RivalPlan[];
  addOns: RivalAddOn[];
  competitor?: Competitor;
  facts: RivalFact[];
  rivalWins: string[];
  dimmiWins: string[];
  switchTriggers: RivalSwitchTrigger[];
  teams: RivalTeamPick[];
};

export const RIVALS: Record<string, Rival> = {
  intercom: {
    slug: "intercom",
    name: "Intercom",
    tone: "#286efa",
    pricingUrl: "https://www.intercom.com/pricing",
    checkedOn: CHECKED_ON_ISO,
    checkedOnLabel: CHECKED_ON,
    aiUnit: "outcome",
    aiPrice: 0.99,
    aiNote: "Fin AI Agent charges $0.99 per outcome on every plan, including completed procedures and handoffs.",
    plans: [
      {
        name: "Essential",
        seatPrice: 29,
        promoSeatPrice: 19,
        promoNote: "New customers save 35% on annual billing ($19/seat/mo vs $29 list in Intercom FAQ).",
        billingNote: "Billed annually",
        highlights: ["Messenger live chat", "Shared inbox and ticketing", "Public help center", "Basic automation"],
        missing: ["Workflow automation builder", "Multiple team inboxes", "Round robin", "SLAs and SSO"],
      },
      {
        name: "Advanced",
        seatPrice: 85,
        billingNote: "Billed annually",
        highlights: ["Workflow automation builder", "Multiple team inboxes", "Round robin", "20 free Lite seats"],
        missing: ["SLAs", "SAML SSO", "Multibrand help center"],
      },
      {
        name: "Expert",
        seatPrice: 132,
        billingNote: "Billed annually",
        highlights: [
          "SLAs",
          "SSO and identity management",
          "Multibrand Messenger and Help Center",
          "50 free Lite seats",
        ],
      },
    ],
    addOns: [
      {
        name: "Copilot",
        price: 29,
        unit: "seat/mo",
        note: "$29/agent/mo billed annually ($35 monthly) for unlimited usage after 10 free conversations/agent/mo.",
      },
      {
        name: "Pro (includes Operator)",
        price: 100,
        unit: "workspace/mo",
        note: "From $100/workspace/mo; includes 500 conversation analyses and 6,000 credits/mo.",
      },
      {
        name: "Proactive Support Plus",
        price: 99,
        unit: "workspace/mo",
        note: "From $99/workspace/mo; includes 500 messages for Tours, Surveys, Posts, and Series.",
      },
    ],
    competitor: {
      name: "Intercom",
      seat: () => 29,
      ai: { per: "resolution", price: 0.99 },
      basis: "Essential, $29 a seat, plus Fin at $0.99 per resolution.",
      gated:
        "Not on this plan: the workflow builder, round robin, or more than one team inbox (Advanced, $85). SLAs need Expert, $132.",
      full: { plan: "Expert", seat: 132, adds: "workflow builder, round robin, or SLAs" },
      source: "https://www.intercom.com/pricing",
    },
    facts: [
      {
        label: "Entry seat price",
        rival: "$29/seat/mo list ($19/seat/mo new-customer promo)",
        dimmi: "$0 per seat (unlimited teammates)",
        source: "https://www.intercom.com/pricing",
        checkedOn: CHECKED_ON_ISO,
      },
      {
        label: "AI agent billing",
        rival: "$0.99 per Fin outcome on every plan",
        dimmi: "From $0.10/conversation, or $0.05 with your own model key",
        source: "https://www.intercom.com/pricing",
        checkedOn: CHECKED_ON_ISO,
      },
      {
        label: "Workflow builder and SLAs",
        rival: "Workflows on Advanced ($85/seat); SLAs and SSO on Expert ($132/seat)",
        dimmi: "Included on every workspace, including the free tier and self-hosted",
        source: "https://www.intercom.com/pricing",
        checkedOn: CHECKED_ON_ISO,
      },
      {
        label: "Source code and self-hosting",
        rival: "Closed source, Intercom Cloud only",
        dimmi: "MIT license, self-host on one Linux server for $0",
        source: "https://www.intercom.com/pricing",
        checkedOn: CHECKED_ON_ISO,
      },
    ],
    rivalWins: [
      "A long track record and a large customer base; dimmi launched in November 2026.",
      "SOC 2 Type II and HIPAA options ready today; dimmi has no SOC 2 or ISO 27001 certification yet.",
      "A much larger app marketplace and more built-in third-party integrations.",
      "Mature native mobile SDKs for iOS and Android apps.",
    ],
    dimmiWins: [
      "Unlimited free seats so engineers, product managers, and founders can answer bugs without raising the bill.",
      "Bring your own OpenAI or Anthropic key to drop AI conversations to the human rate instead of paying $0.99 per outcome.",
      "MIT open-source code you can self-host on one Linux server with SAML SSO, SLAs, and workflows included.",
      "100 free conversations every month on dimmi Cloud with no credit card and no 14-day countdown.",
    ],
    switchTriggers: [
      {
        trigger: "Fin outcome meter ($0.99/outcome)",
        rivalReality: "Every resolved chat or completed procedure adds $0.99 on top of paid seats.",
        dimmiReality: "First 100 conversations free monthly, then $0.10/AI conversation or $0.05 with your own key.",
        sourceLabel: "Intercom pricing",
        sourceUrl: "https://www.intercom.com/pricing",
      },
      {
        trigger: "Per-seat upgrades for basic ops",
        rivalReality:
          "Workflow builder and round robin require Advanced ($85/seat); SLAs and SSO require Expert ($132/seat).",
        dimmiReality: "Workflows, round robin, multiple inboxes, SLAs, and SAML SSO ship on every plan.",
        sourceLabel: "Intercom pricing",
        sourceUrl: "https://www.intercom.com/pricing",
      },
      {
        trigger: "Add-on creep for Copilot and outbound",
        rivalReality: "Copilot ($29/seat), Pro ($100/mo), and Proactive Support Plus ($99/mo) bill on top of seats.",
        dimmiReality:
          "Operator, broadcasts, series, changelog, and feedback boards live inside one transparent price book.",
        sourceLabel: "Intercom pricing",
        sourceUrl: "https://www.intercom.com/pricing",
      },
      {
        trigger: "Vendor lock-in and data residency",
        rivalReality: "Proprietary cloud platform with no self-hosted deployment option.",
        dimmiReality: "Full MIT codebase you can host on your own Linux box and Postgres database for $0.",
        sourceLabel: "dimmi GitHub",
        sourceUrl: "https://github.com/dimmihq/dimmi",
      },
    ],
    teams: [
      {
        profile: "Solo founder (1 seat, 400 chats/mo)",
        seats: 1,
        conversations: 400,
        aiShare: 0.7,
        pick: "dimmi",
        reason: "Free seats plus 100 free conversations keep the monthly cloud bill tiny, with workflows included.",
        rivalPlanLabel: "Essential ($29) + Fin",
      },
      {
        profile: "5-person SaaS team (2,000 chats/mo)",
        seats: 5,
        conversations: 2000,
        aiShare: 0.8,
        pick: "dimmi",
        reason:
          "At 80% AI share, Fin alone costs nearly $1,000/mo on top of seats. dimmi with BYO keys cuts the bill sharply.",
        rivalPlanLabel: "Essential ($29 x 5) + Fin",
      },
      {
        profile: "25-seat support team (10,000 chats/mo)",
        seats: 25,
        conversations: 10000,
        aiShare: 0.8,
        pick: "dimmi",
        reason: "Free seats eliminate $3,300/mo of Expert seat spend before AI outcomes even start counting.",
        rivalPlanLabel: "Expert ($132 x 25) + Fin",
      },
      {
        profile: "Regulated enterprise needing SOC 2 today",
        seats: 15,
        conversations: 5000,
        aiShare: 0.6,
        pick: "rival",
        reason: "Intercom has SOC 2 Type II and HIPAA compliance ready today. dimmi does not hold SOC 2 yet.",
        rivalPlanLabel: "Expert ($132 x 15) + Fin",
      },
    ],
  },
  zendesk: {
    slug: "zendesk",
    name: "Zendesk",
    tone: "#03363d",
    pricingUrl: "https://www.zendesk.com/pricing/",
    checkedOn: CHECKED_ON_ISO,
    checkedOnLabel: CHECKED_ON,
    aiUnit: "resolution",
    aiPrice: 2,
    aiNote:
      "Automated resolutions beyond plan allowance (5/agent on Suite Team, 10/agent on Suite Professional) cost $2.00 pay-as-you-go ($1.50 committed).",
    plans: [
      {
        name: "Suite Team",
        seatPrice: 69,
        promoSeatPrice: 55,
        promoNote: "$55/agent/mo billed annually ($69/agent/mo monthly).",
        includedAi: 5,
        billingNote: "Monthly list ($55 annual)",
        highlights: [
          "Omnichannel routing",
          "Messaging and live chat",
          "Knowledge base",
          "5 automated resolutions/agent/mo",
        ],
        missing: ["SLAs", "Skills-based routing", "Side conversations", "Multilingual help center"],
      },
      {
        name: "Suite Professional",
        seatPrice: 149,
        promoSeatPrice: 115,
        promoNote: "$115/agent/mo billed annually ($149/agent/mo monthly).",
        includedAi: 10,
        billingNote: "Monthly list ($115 annual)",
        highlights: ["SLAs", "Skills-based routing", "IVR phone tree", "10 automated resolutions/agent/mo"],
      },
    ],
    addOns: [
      { name: "Copilot", price: 50, unit: "seat/mo", note: "$50/agent/mo billed annually." },
      { name: "Workforce Engagement", price: 50, unit: "seat/mo", note: "$50/agent/mo billed annually." },
    ],
    competitor: {
      name: "Zendesk",
      seat: () => 69,
      ai: { per: "resolution", price: 2, includedPerSeat: 5 },
      basis: "Suite Team, $69 a seat monthly, plus $2.00 per automated resolution after 5 a seat.",
      gated:
        "Not on this plan: SLAs, skills-based routing, side conversations, or a second help center language (Suite Professional, $149).",
      full: {
        plan: "Suite Professional",
        seat: 149,
        adds: "SLAs, skills-based routing, or second help center language",
        includedPerSeat: 10,
      },
      source: "https://www.zendesk.com/pricing/",
    },
    facts: [],
    rivalWins: [
      "Deep enterprise ticketing and call-center tooling built for 200+ seat teams.",
      "1,800+ apps in the Zendesk Marketplace.",
      "SOC 2, ISO 27001, and HIPAA compliance certifications ready today; dimmi has none yet.",
      "Broad language and enterprise localization coverage across 80+ languages.",
    ],
    dimmiWins: [
      "Unlimited free seats with SLAs, routing, and multilingual help center unlocked on every plan.",
      "No $1.50 to $2.00 automated resolution fees; plug in your own AI model key.",
      "MIT open source and self-hostable on one Linux server.",
    ],
    switchTriggers: [
      {
        trigger: "Tier jumps for SLAs and skills routing",
        rivalReality:
          "Suite Team ($69/agent monthly, $55 annual) omits SLAs, side conversations, skills routing, and multilingual docs until Suite Professional ($149 monthly, $115 annual).",
        dimmiReality: "SLAs, workflow routing, internal notes, and multilingual help centers ship on every workspace.",
        sourceLabel: "Zendesk pricing",
        sourceUrl: "https://www.zendesk.com/pricing/",
      },
      {
        trigger: "$1.50 to $2.00 automated resolution overages",
        rivalReality:
          "After 5 to 10 automated resolutions per agent/month, extra AI resolutions cost $2.00 pay-as-you-go ($1.50 committed).",
        dimmiReality:
          "Bring your own OpenAI or Anthropic key to drop Cloud AI fees by 50% to 83%, or self-host for $0.",
        sourceLabel: "Zendesk pricing",
        sourceUrl: "https://www.zendesk.com/pricing/",
      },
      {
        trigger: "$50/agent Copilot and WFM add-ons",
        rivalReality:
          "Teammate Copilot ($50/agent/mo) and Workforce Engagement ($50/agent/mo) stack on top of Suite seats.",
        dimmiReality:
          "Operator teammate copilot lives inside the same conversation meter with zero per-seat surcharge.",
        sourceLabel: "Zendesk pricing",
        sourceUrl: "https://www.zendesk.com/pricing/",
      },
      {
        trigger: "Heavy enterprise admin overhead for small teams",
        rivalReality:
          "Closed SaaS tuned for 200+ seat call centers with complex views, triggers, and annual contracts.",
        dimmiReality: "MIT open source in one Go binary, ready in 5 minutes on Cloud or your own Linux server.",
        sourceLabel: "dimmi GitHub",
        sourceUrl: "https://github.com/dimmihq/dimmi",
      },
    ],
    teams: [
      {
        profile: "4-person SaaS support team (2,000 chats/mo)",
        seats: 4,
        conversations: 2000,
        aiShare: 0.75,
        pick: "dimmi",
        reason: "Eliminates $596/mo in Suite Professional seats plus $2.00 resolution overages.",
        rivalPlanLabel: "Suite Team ($69 x 4) + AR",
      },
      {
        profile: "12-person technical team needing SLAs and engineering logins",
        seats: 12,
        conversations: 5000,
        aiShare: 0.7,
        pick: "dimmi",
        reason: "Free seats let every engineer inspect tickets while SLAs and workflows come unlocked.",
        rivalPlanLabel: "Suite Pro ($149 x 12) + AR",
      },
      {
        profile: "300-seat global enterprise call center needing HIPAA and IVR",
        seats: 25,
        conversations: 10000,
        aiShare: 0.5,
        pick: "rival",
        reason: "Zendesk holds SOC 2, ISO 27001, HIPAA, and 1,800+ marketplace integrations dimmi does not have yet.",
        rivalPlanLabel: "Suite Enterprise",
      },
      {
        profile: "Privacy-first EU team self-hosting in its own VPC",
        seats: 8,
        conversations: 3000,
        aiShare: 0.8,
        pick: "dimmi",
        reason: "dimmi runs inside your own Postgres database under MIT for $0 software fees.",
        rivalPlanLabel: "Suite Pro ($149 x 8) + AR",
      },
    ],
  },
  "help-scout": {
    slug: "help-scout",
    name: "Help Scout",
    tone: "#1292ee",
    pricingUrl: "https://www.helpscout.com/pricing/",
    checkedOn: CHECKED_ON_ISO,
    checkedOnLabel: CHECKED_ON,
    aiUnit: "resolution",
    aiPrice: 0.75,
    aiNote: "Envoy AI agent costs $0.75 per resolution after the 3-month trial.",
    plans: [
      {
        name: "Standard",
        seatPrice: 25,
        billingNote: "$25/user/mo annual ($30 monthly), up to 25 users",
        highlights: ["2 Inboxes (+$10/mo annual each)", "2 Docs sites (+$20/mo annual each)", "1 basic SLA policy"],
        missing: ["Round robin routing", "Customer portal", "Internal teams"],
      },
      {
        name: "Plus",
        seatPrice: 45,
        billingNote: "$45/user/mo annual ($54 monthly), up to 50 users",
        highlights: ["5 Inboxes", "Round robin routing", "2 advanced SLA policies", "Customer portal"],
        missing: ["SAML SSO (Pro $75/user/mo)"],
      },
    ],
    addOns: [
      { name: "Extra Inbox", price: 10, unit: "flat/mo", note: "$10/mo annual ($12 monthly) per extra inbox." },
      { name: "Extra Docs site", price: 20, unit: "flat/mo", note: "$20/mo annual ($24 monthly) per extra Docs site." },
    ],
    competitor: {
      name: "Help Scout",
      seat: (seats) => (seats > 25 ? 45 : 25),
      ai: { per: "resolution", price: 0.75 },
      basis: "Standard, $25 a seat (Plus, $45, above 25 seats), plus $0.75 per AI resolution.",
      gated: "Not on this plan: round robin or the customer portal (Plus, $45). Two inboxes included, then $12 each.",
      full: { plan: "Plus", seat: 45, adds: "round robin or customer portal" },
      source: "https://www.helpscout.com/pricing/",
    },
    facts: [],
    rivalWins: [
      "Email-first help desk with a long track record and clean UX.",
      "Strong documentation and mature SOC 2 and HIPAA compliance; dimmi has no SOC 2 yet.",
      "Free plan for up to 5 users with 25 contacts/mo and 1 inbox.",
    ],
    dimmiWins: [
      "Unlimited free seats and unlimited inboxes with no $10 to $12 per-inbox surcharge.",
      "Bring your own AI model keys instead of $0.75 per Envoy resolution.",
      "Open-source MIT codebase with procedures, workflows, and self-hosting.",
    ],
    switchTriggers: [
      {
        trigger: "Per-inbox and per-Docs-site surcharges",
        rivalReality: "Standard includes only 2 inboxes (+$10-$12/mo each) and 2 Docs sites (+$20-$24/mo each).",
        dimmiReality: "Create separate inboxes for support, billing, security, and brands at $0 extra.",
        sourceLabel: "Help Scout pricing",
        sourceUrl: "https://www.helpscout.com/pricing/",
      },
      {
        trigger: "$0.75 per Envoy AI resolution",
        rivalReality:
          "After the introductory trial, Envoy bills $0.75 on every resolved AI thread on top of user seats.",
        dimmiReality: "AI starts at $0.10/conversation on Cloud, or drops to human rates with your own model key.",
        sourceLabel: "Help Scout pricing",
        sourceUrl: "https://www.helpscout.com/pricing/",
      },
      {
        trigger: "Round robin, teams, and SAML SSO tier walls",
        rivalReality: "Round robin needs Plus ($45/user/mo); SAML SSO needs Pro ($75/user/mo with a 10-user minimum).",
        dimmiReality:
          "Round robin, teams, SLAs, and SAML SSO ship on every dimmi workspace, including free and MIT self-hosted.",
        sourceLabel: "Help Scout pricing",
        sourceUrl: "https://www.helpscout.com/pricing/",
      },
      {
        trigger: "Closed SaaS with no self-hosting",
        rivalReality: "Help Scout runs exclusively on its hosted cloud with no open-source code.",
        dimmiReality: "Full MIT Go and React repository you can audit and run on your own Linux box.",
        sourceLabel: "dimmi GitHub",
        sourceUrl: "https://github.com/dimmihq/dimmi",
      },
    ],
    teams: [
      {
        profile: "Multi-brand SaaS running 6 shared inboxes (5 seats, 2,000 chats)",
        seats: 5,
        conversations: 2000,
        aiShare: 0.7,
        pick: "dimmi",
        reason: "Unlimited inboxes and free seats avoid Help Scout Plus ($225/mo) plus $0.75 Envoy resolution fees.",
        rivalPlanLabel: "Standard + 4 inboxes + AI",
      },
      {
        profile: "30-person company inviting engineers into support",
        seats: 30,
        conversations: 3000,
        aiShare: 0.65,
        pick: "dimmi",
        reason: "Crossing 25 users forces Help Scout Plus ($45 x 30 = $1,350/mo) before AI; dimmi seats are $0.",
        rivalPlanLabel: "Plus ($45 x 30) + AI",
      },
      {
        profile: "Tiny 3-person email shop under 25 contacts/month",
        seats: 3,
        conversations: 50,
        aiShare: 0,
        pick: "rival",
        reason: "Help Scout's Free tier and email-first polish fit ultra-low volume email teams that also want SOC 2.",
        rivalPlanLabel: "Help Scout Free ($0)",
      },
      {
        profile: "Engineering team needing SAML SSO without a 10-seat Pro floor",
        seats: 6,
        conversations: 1500,
        aiShare: 0.75,
        pick: "dimmi",
        reason: "Help Scout Pro starts at $750/mo minimum (10 x $75); dimmi includes SAML SSO on every plan.",
        rivalPlanLabel: "Pro (10-seat min $750)",
      },
    ],
  },
  front: {
    slug: "front",
    name: "Front",
    tone: "#635bff",
    pricingUrl: "https://front.com/pricing",
    checkedOn: CHECKED_ON_ISO,
    checkedOnLabel: CHECKED_ON,
    aiUnit: "conversation",
    aiPrice: 0.39,
    aiNote: "Front Autopilot starts at $0.39 per conversation as an add-on on every plan.",
    plans: [
      {
        name: "Starter",
        seatPrice: 25,
        billingNote: "$25/seat/mo annual (up to 10 seats)",
        highlights: ["Single channel type only", "Up to 10 automation rules", "1 workspace"],
        missing: ["Second channel type", "Macros", "SSO and SCIM"],
      },
      {
        name: "Professional",
        seatPrice: 65,
        billingNote: "$65/seat/mo annual (up to 50 seats)",
        highlights: ["Omnichannel (email, SMS, social, WhatsApp)", "Up to 20 rules", "SSO and SCIM"],
        missing: ["Unlimited rules", "Multilingual knowledge base", "Custom reports"],
      },
      {
        name: "Enterprise",
        seatPrice: 105,
        billingNote: "$105/seat/mo annual",
        highlights: ["Unlimited rules and macros", "Multilingual knowledge base", "Custom reports", "Copilot included"],
      },
    ],
    addOns: [{ name: "Copilot", price: 20, unit: "seat/mo", note: "$20/seat/mo on Starter and Professional." }],
    competitor: {
      name: "Front",
      seat: (seats) => (seats > 10 ? 65 : 25),
      ai: { per: "conversation", price: 0.39 },
      basis:
        "Starter, $25 a seat billed yearly (Professional, $65, above 10 seats), plus Autopilot from $0.39 per conversation.",
      gated:
        "Starter covers one channel type and 10 rules. Email plus chat needs Professional, $65. Custom reports need Enterprise, $105.",
      full: { plan: "Enterprise", seat: 105, adds: "second channel type, custom reports, or multilingual help center" },
      source: "https://front.com/pricing",
    },
    facts: [],
    rivalWins: [
      "Strong shared email workspace for sales, account management, and operations teams alongside support.",
      "Mature email routing workflows and SOC 2 compliance; dimmi has no SOC 2 yet.",
      "Deep CRM integrations out of the box.",
    ],
    dimmiWins: [
      "Every channel (web chat, email, Slack, WhatsApp, Instagram, Messenger) included from conversation 1.",
      "Unlimited free seats instead of $25 to $105 per seat.",
      "MIT open source and self-hostable with zero WhatsApp 20% admin markup.",
    ],
    switchTriggers: [
      {
        trigger: "One channel type limit on Starter ($25/seat)",
        rivalReality:
          "Running email plus live chat or WhatsApp forces an immediate jump to Professional ($65/seat/mo).",
        dimmiReality: "Web chat, email, Slack, WhatsApp, Instagram, and Messenger are all unlocked from day one.",
        sourceLabel: "Front pricing",
        sourceUrl: "https://front.com/pricing",
      },
      {
        trigger: "WhatsApp 20% admin fee and $20/seat Copilot",
        rivalReality: "Front adds a 20% admin fee on Meta WhatsApp costs and bills Copilot at $20/seat on Starter/Pro.",
        dimmiReality: "Connect your own Meta account with 0% markup from dimmi, and use Operator without seat fees.",
        sourceLabel: "Front pricing",
        sourceUrl: "https://front.com/pricing",
      },
      {
        trigger: "Autopilot at $0.39/conversation on top of seats",
        rivalReality: "Front Autopilot starts at $0.39 per conversation with no option to bring your own LLM API key.",
        dimmiReality:
          "Cloud AI starts at $0.10/conversation, or drops to human rates when you connect your own API key.",
        sourceLabel: "Front pricing",
        sourceUrl: "https://front.com/pricing",
      },
      {
        trigger: "Enterprise ($105/seat) wall for custom reports and multilingual docs",
        rivalReality:
          "Unlimited rules, multilingual knowledge base, and custom reporting sit on Enterprise ($105/seat).",
        dimmiReality: "Visual workflows, multilingual help centers, and workspace reports ship on every plan.",
        sourceLabel: "Front pricing",
        sourceUrl: "https://front.com/pricing",
      },
    ],
    teams: [
      {
        profile: "6-person product team running email + live chat + WhatsApp",
        seats: 6,
        conversations: 2000,
        aiShare: 0.7,
        pick: "dimmi",
        reason:
          "Adding chat beside email forces Front Professional ($390/mo seats + Autopilot); dimmi includes every channel.",
        rivalPlanLabel: "Front Pro ($65 x 6) + AI",
      },
      {
        profile: "15-person B2B logistics team collaborating across email only",
        seats: 15,
        conversations: 3000,
        aiShare: 0.1,
        pick: "rival",
        reason:
          "Front excels as a shared email client for sales, dispatch, and account managers with mature CRM sync and SOC 2.",
        rivalPlanLabel: "Front Pro ($65 x 15)",
      },
      {
        profile: "12-person SaaS support team with 5,000 monthly chats",
        seats: 12,
        conversations: 5000,
        aiShare: 0.8,
        pick: "dimmi",
        reason:
          "Exceeding 10 seats forces Front Professional ($780/mo) plus $1,560/mo Autopilot; dimmi stays a fraction of that.",
        rivalPlanLabel: "Front Pro + Autopilot",
      },
      {
        profile: "Developer-led startup needing open-source API and MCP access",
        seats: 4,
        conversations: 1000,
        aiShare: 0.75,
        pick: "dimmi",
        reason: "dimmi ships 12 typed SDKs, an MCP server, and MIT self-hosting with no $200 API rate-limit add-on.",
        rivalPlanLabel: "Front Starter + Autopilot",
      },
    ],
  },
  freshdesk: {
    slug: "freshdesk",
    name: "Freshdesk",
    tone: "#25c16f",
    pricingUrl: "https://www.freshworks.com/freshdesk/pricing/",
    checkedOn: CHECKED_ON_ISO,
    checkedOnLabel: CHECKED_ON,
    aiUnit: "session",
    aiPrice: 0.49,
    aiNote: "First 500 Freddy AI Agent sessions included once per account; extra sessions cost $49 per 100 sessions.",
    plans: [
      {
        name: "Growth",
        seatPrice: 19,
        billingNote: "$19/agent/mo billed annually",
        highlights: ["Ticketing and shared inbox", "Knowledge base", "First 500 Freddy sessions once"],
        missing: ["Multilingual helpdesk", "Custom dashboards", "Multiple SLA policies"],
      },
      {
        name: "Pro",
        seatPrice: 55,
        billingNote: "$55/agent/mo billed annually",
        highlights: ["Multilingual helpdesk", "Custom dashboards", "Multiple SLA policies", "Intelligent routing"],
        missing: ["Audit logs", "Skills-based routing", "Sandbox"],
      },
      {
        name: "Enterprise",
        seatPrice: 89,
        billingNote: "$89/agent/mo billed annually",
        highlights: ["Audit logs", "Skills-based routing", "Sandbox", "IP whitelisting"],
      },
    ],
    addOns: [
      {
        name: "Freddy AI Agent sessions",
        price: 49,
        unit: "per 100 sessions",
        note: "$49 per 100 sessions; expire with billing cycle.",
      },
      { name: "Freddy AI Copilot", price: 29, unit: "seat/mo", note: "$29/agent/mo on Pro and Enterprise." },
    ],
    competitor: {
      name: "Freshdesk",
      seat: () => 19,
      ai: { per: "conversation", price: 0.49 },
      basis: "Growth, $19 a seat billed yearly, plus Freddy AI Agent at $49 per 100 sessions.",
      gated: "Not on this plan: custom reports, multiple SLA policies, or a multilingual help desk (Pro, $55).",
      full: { plan: "Pro", seat: 55, adds: "custom reports, multiple SLA policies, or multilingual help desk" },
      source: "https://www.freshworks.com/freshdesk/pricing/",
    },
    facts: [],
    rivalWins: [
      "Low $19/agent/mo entry seat price on annual billing.",
      "Broad Freshworks product suite (CRM, ITSM, phone) from one vendor.",
      "Established SOC 2 and ISO 27001 certifications; dimmi has none yet.",
    ],
    dimmiWins: [
      "Unlimited free seats with multiple SLA policies, custom reports, and audit logs included.",
      "Predictable per-conversation AI pricing with BYO model keys instead of expiring 100-session packs.",
      "Full MIT open source you can inspect and self-host.",
    ],
    switchTriggers: [
      {
        trigger: "Expiring 100-session Freddy AI packs ($49/pack)",
        rivalReality:
          "Freddy AI Agent bills $49 per 100 sessions (with 72-hour email windows) that expire at the end of the billing cycle.",
        dimmiReality:
          "Pay only for real conversations after 100 free monthly chats, or plug in your own OpenAI/Anthropic key.",
        sourceLabel: "Freshdesk pricing",
        sourceUrl: "https://www.freshworks.com/freshdesk/pricing/",
      },
      {
        trigger: "Pro ($55) and Enterprise ($89) tier locks for SLAs and audit logs",
        rivalReality:
          "Multiple SLAs and multilingual docs require Pro ($55/agent); audit logs require Enterprise ($89/agent).",
        dimmiReality: "Multiple SLA policies, multilingual help centers, and full audit logs ship on every plan.",
        sourceLabel: "Freshdesk pricing",
        sourceUrl: "https://www.freshworks.com/freshdesk/pricing/",
      },
      {
        trigger: "Add-on meter for Copilot, MCP actions, and connector tasks",
        rivalReality:
          "Freddy Copilot ($29/agent/mo), MCP actions ($15/1,000), and connector tasks ($80/5,000) bill separately.",
        dimmiReality: "Built-in MCP server costs $0 per call, and Operator needs zero per-seat add-on.",
        sourceLabel: "Freshdesk pricing",
        sourceUrl: "https://www.freshworks.com/freshdesk/pricing/",
      },
      {
        trigger: "Proprietary cloud with no self-hosting",
        rivalReality: "Closed-source SaaS with no way to host support data inside your own database.",
        dimmiReality: "MIT open-source build that deploys onto one Linux server with your own Postgres.",
        sourceLabel: "dimmi GitHub",
        sourceUrl: "https://github.com/dimmihq/dimmi",
      },
    ],
    teams: [
      {
        profile: "5-person SaaS team running AI email + chat (2,000 chats/mo)",
        seats: 5,
        conversations: 2000,
        aiShare: 0.75,
        pick: "dimmi",
        reason: "Avoids 15+ Freddy session packs ($735+) on top of Pro seats ($275/mo) with BYO model keys.",
        rivalPlanLabel: "Growth + Freddy packs",
      },
      {
        profile: "2-person manual-only team on Growth ($19 x 2)",
        seats: 2,
        conversations: 2000,
        aiShare: 0,
        pick: "rival",
        reason:
          "If you use zero AI and only need basic ticketing without multiple SLAs, Freshdesk Growth ($38/mo) beats dimmi Cloud ($60/mo) on paper (though self-hosting dimmi is $0).",
        rivalPlanLabel: "Freshdesk Growth ($19 x 2)",
      },
      {
        profile: "15-person B2B team needing multiple SLAs and audit logs",
        seats: 15,
        conversations: 4000,
        aiShare: 0.7,
        pick: "dimmi",
        reason:
          "Audit logs force Freshdesk Enterprise ($89 x 15 = $1,335/mo) before Freddy AI; dimmi includes them at $0 seats.",
        rivalPlanLabel: "Enterprise ($89 x 15) + AI",
      },
      {
        profile: "Company standardized on Freshsales CRM and Freshcaller phone",
        seats: 10,
        conversations: 2500,
        aiShare: 0.4,
        pick: "rival",
        reason:
          "Keeping CRM, ITSM, and voice under one Freshworks contract with SOC 2 and ISO 27001 is simpler if you already use the suite.",
        rivalPlanLabel: "Freshdesk Pro ($55 x 10)",
      },
    ],
  },
  chatwoot: {
    slug: "chatwoot",
    name: "Chatwoot",
    tone: "#1f93ff",
    pricingUrl: "https://www.chatwoot.com/pricing",
    checkedOn: CHECKED_ON_ISO,
    checkedOnLabel: CHECKED_ON,
    aiUnit: "credit",
    aiPrice: 0.02,
    aiNote:
      "Captain AI credits are bundled on Startups (300/mo), Business (500/mo), and Enterprise (800/mo); extra Captain credits cost $20 per 1,000.",
    plans: [
      {
        name: "Startups",
        seatPrice: 19,
        includedAi: 300,
        billingNote: "$19/agent/mo on Chatwoot Cloud",
        highlights: ["Unlimited chats under fair use", "Help center", "300 Captain AI credits/mo", "1-year retention"],
        missing: ["Teams and round robin", "Automation rules", "SAML SSO and audit logs"],
      },
      {
        name: "Business",
        seatPrice: 39,
        includedAi: 500,
        billingNote: "$39/agent/mo on Chatwoot Cloud",
        highlights: ["Teams and automation rules", "Voice channel", "500 Captain AI credits/mo", "2-year retention"],
        missing: ["SAML SSO", "Audit logs", "Custom roles"],
      },
      {
        name: "Enterprise",
        seatPrice: 99,
        includedAi: 800,
        billingNote: "$99/agent/mo (Cloud or self-hosted Enterprise)",
        highlights: ["SAML SSO", "Audit logs", "Custom roles", "800 Captain AI credits/mo"],
      },
    ],
    addOns: [
      {
        name: "Captain AI extra credits",
        price: 20,
        unit: "per 1,000 credits",
        note: "$20 per 1,000 additional Captain AI credits on Cloud plans.",
      },
    ],
    facts: [
      {
        label: "License model",
        rival: "MIT outside enterprise/; paid Enterprise License required for enterprise/ in production",
        dimmi: "100% MIT across the entire repository, including SAML SSO, audit logs, and AI procedures",
        source: "https://github.com/chatwoot/chatwoot/blob/develop/LICENSE",
        checkedOn: CHECKED_ON_ISO,
      },
      {
        label: "Self-hosted runtime stack",
        rival: "Ruby on Rails app + Sidekiq worker + PostgreSQL (pgvector) + Redis",
        dimmi: "Single Go binary + PostgreSQL (pgvector) + Valkey + Caddy",
        source: "https://github.com/chatwoot/chatwoot/blob/develop/docker-compose.production.yaml",
        checkedOn: CHECKED_ON_ISO,
      },
      {
        label: "Cloud seat pricing",
        rival: "$0 Hacker (2 agents, 500 chats) or $19 to $99/agent/mo",
        dimmi: "$0 per seat (unlimited teammates), 100 free conversations/mo",
        source: "https://www.chatwoot.com/pricing",
        checkedOn: CHECKED_ON_ISO,
      },
    ],
    rivalWins: [
      "Years in production with a large contributor community and roughly 40.7k GitHub stars.",
      "Wider set of social and messaging channels today, including Telegram, LINE, and SMS connectors.",
      "Dedicated open-source mobile apps for iOS and Android teammates.",
      "Established community Helm charts, guides, and third-party deployment templates.",
    ],
    dimmiWins: [
      "Pure MIT codebase from top to bottom with zero enterprise/ paywall folder for SAML SSO, audit logs, or custom roles.",
      "Single compiled Go binary instead of Ruby on Rails plus Sidekiq workers, keeping server memory lean.",
      "Built-in multi-agent studio, policy procedures with simulations, Operator routines, and an MCP server.",
      "Bring your own OpenAI, Anthropic, DeepSeek, Mistral, or OpenRouter keys on both Cloud and self-hosted without Captain credit caps.",
    ],
    switchTriggers: [
      {
        trigger: "enterprise/ license wall on self-hosted",
        rivalReality:
          "SAML SSO, audit logs, custom roles, and Captain Enterprise features live under enterprise/LICENSE and require a paid Enterprise seat license in production.",
        dimmiReality: "SAML SSO, audit logs, roles, SLAs, workflows, and AI procedures ship under MIT for $0.",
        sourceLabel: "Chatwoot enterprise/LICENSE",
        sourceUrl: "https://github.com/chatwoot/chatwoot/blob/develop/enterprise/LICENSE",
      },
      {
        trigger: "Rails + Sidekiq operational footprint",
        rivalReality:
          "Self-hosting runs separate Rails web and Sidekiq worker containers alongside Postgres and Redis.",
        dimmiReality:
          "One Go binary runs web, worker, and scheduler with Postgres, Valkey, and Caddy in one compose file.",
        sourceLabel: "Chatwoot compose file",
        sourceUrl: "https://github.com/chatwoot/chatwoot/blob/develop/docker-compose.production.yaml",
      },
      {
        trigger: "Captain AI credit caps",
        rivalReality: "Cloud tiers bundle 300 to 800 Captain credits/mo and charge $20 per extra 1,000 credits.",
        dimmiReality: "Plug in your own OpenAI or Anthropic API key directly on self-hosted ($0 to dimmi) or Cloud.",
        sourceLabel: "Chatwoot pricing",
        sourceUrl: "https://www.chatwoot.com/pricing",
      },
      {
        trigger: "Per-agent Cloud pricing for occasional teammates",
        rivalReality: "Chatwoot Cloud charges $19 to $99 per agent/month once you grow past 2 agents on Hacker.",
        dimmiReality: "Every teammate seat is free on dimmi Cloud and self-hosted installs.",
        sourceLabel: "Chatwoot pricing",
        sourceUrl: "https://www.chatwoot.com/pricing",
      },
    ],
    teams: [
      {
        profile: "Solo self-hoster (1 seat, 400 chats/mo)",
        seats: 1,
        conversations: 400,
        aiShare: 0.7,
        pick: "dimmi",
        reason: "One Go binary fits comfortably on a $24 VPS with zero Rails tuning and your own AI model key.",
        rivalPlanLabel: "Self-hosted Community + VPS",
      },
      {
        profile: "5-person SaaS team (2,000 chats/mo)",
        seats: 5,
        conversations: 2000,
        aiShare: 0.75,
        pick: "dimmi",
        reason:
          "Unlimited free seats, built-in AI procedures, and BYO keys avoid $195/mo Business seats plus Captain add-ons.",
        rivalPlanLabel: "Chatwoot Business ($39 x 5)",
      },
      {
        profile: "Brand running Telegram, LINE, and broad social DMs today",
        seats: 6,
        conversations: 3000,
        aiShare: 0.4,
        pick: "rival",
        reason: "Chatwoot already ships Telegram and LINE channels alongside mature community connectors.",
        rivalPlanLabel: "Chatwoot Business ($39 x 6)",
      },
      {
        profile: "Regulated engineering team needing self-hosted SAML SSO and audit logs",
        seats: 12,
        conversations: 4000,
        aiShare: 0.65,
        pick: "dimmi",
        reason:
          "SAML SSO and audit logs are MIT in dimmi, whereas Chatwoot puts them under enterprise/LICENSE ($99/agent/mo).",
        rivalPlanLabel: "Chatwoot Enterprise ($99 x 12)",
      },
    ],
  },
};

const COMPETITOR_SLUGS = ["intercom", "zendesk", "help-scout", "front", "freshdesk"] as const;

export const COMPETITORS: Competitor[] = COMPETITOR_SLUGS.flatMap((slug) => {
  const c = RIVALS[slug]?.competitor;
  return c ? [c] : [];
});

export function competitorBill(c: Competitor, usage: Usage & { seats: number }): number {
  const seats = Math.max(1, usage.seats);
  return seats * c.seat(seats) + aiCharge(c, usage, c.ai.includedPerSeat ?? 0);
}

/* The same month on the plan that has what dimmi includes. */
export function fullBill(c: Competitor, usage: Usage & { seats: number }): number {
  const seats = Math.max(1, usage.seats);
  return seats * c.full.seat + aiCharge(c, usage, c.full.includedPerSeat ?? c.ai.includedPerSeat ?? 0);
}

function aiCharge(c: Competitor, usage: Usage & { seats: number }, includedPerSeat: number): number {
  const seats = Math.max(1, usage.seats);
  const answered = Math.round(usage.conversations * usage.aiShare);
  const charged =
    c.ai.per === "resolution" ? Math.max(0, answered * RESOLUTION_RATE - includedPerSeat * seats) : answered;
  return charged * c.ai.price;
}

/* The fixed rows of the comparison table. */
export const SCENARIOS = [
  { conversations: 500, seats: 2, aiShare: 0.8 },
  { conversations: 1000, seats: 3, aiShare: 0.8 },
  { conversations: 2000, seats: 4, aiShare: 0.8 },
  { conversations: 5000, seats: 6, aiShare: 0.8 },
  { conversations: 10000, seats: 10, aiShare: 0.8 },
];

export function getRival(slug: string): Rival {
  const rival = RIVALS[slug];
  if (!rival) {
    throw new Error(`Unknown rival: ${slug}`);
  }
  return rival;
}
