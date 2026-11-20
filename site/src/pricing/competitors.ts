/*
 * Competitors' public list prices, from each vendor's pricing page. Each
 * one is priced at its cheapest plan that has a shared inbox, live chat
 * and an AI agent, billed monthly where the page shows a monthly price.
 * Their AI agents charge per resolution or per AI conversation on top of
 * seats. Operator threads are left out: competitors sell AI help for
 * teammates as a per-seat add-on instead. `gated` names what that
 * cheapest plan leaves out, as the vendor's own page lists it; the
 * comparison still uses the cheapest plan, so it flatters them.
 */
import type { Usage } from "./prices";

export const CHECKED_ON = "19 November 2026";

/* The share of AI-answered conversations a competitor counts as resolved. */
export const RESOLUTION_RATE = 0.6;

export type Competitor = {
  name: string;
  /* Monthly price per seat for a team of this size. */
  seat: (seats: number) => number;
  /* What the AI agent charges for, and how much. */
  ai: { per: "resolution" | "conversation"; price: number; includedPerSeat?: number };
  basis: string;
  /* What the plan priced here does not include. */
  gated: string;
  /*
   * The cheapest plan with what dimmi includes: its price per seat, the
   * features it adds over the plan above, and any AI resolutions each
   * seat gets with it.
   */
  full: { plan: string; seat: number; adds: string; includedPerSeat?: number };
  source: string;
};

export const COMPETITORS: Competitor[] = [
  {
    name: "Intercom",
    seat: () => 29,
    ai: { per: "resolution", price: 0.99 },
    basis: "Essential, $29 a seat, plus Fin at $0.99 per resolution.",
    gated:
      "Not on this plan: the workflow builder, round robin, or more than one team inbox (Advanced, $85). SLAs need Expert, $132.",
    full: { plan: "Expert", seat: 132, adds: "workflow builder, round robin, or SLAs" },
    source: "https://www.intercom.com/pricing",
  },
  {
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
  {
    name: "Help Scout",
    seat: (seats) => (seats > 25 ? 45 : 25),
    ai: { per: "resolution", price: 0.75 },
    basis: "Standard, $25 a seat (Plus, $45, above 25 seats), plus $0.75 per AI resolution.",
    gated: "Not on this plan: round robin or the customer portal (Plus, $45). Two inboxes included, then $12 each.",
    full: { plan: "Plus", seat: 45, adds: "round robin or customer portal" },
    source: "https://www.helpscout.com/pricing/",
  },
  {
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
  {
    name: "Freshdesk",
    seat: () => 19,
    ai: { per: "conversation", price: 0.49 },
    basis: "Growth, $19 a seat billed yearly, plus Freddy AI Agent at $49 per 100 sessions.",
    gated: "Not on this plan: custom reports, multiple SLA policies, or a multilingual help desk (Pro, $55).",
    full: { plan: "Pro", seat: 55, adds: "custom reports, multiple SLA policies, or multilingual help desk" },
    source: "https://www.freshworks.com/freshdesk/pricing/",
  },
];

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
