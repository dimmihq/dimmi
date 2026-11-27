/*
 * Pure calculation and URL query-parameter helpers for the Intercom cost
 * calculator (`/tools/intercom-cost-calculator`). Keeps numbers in lockstep
 * with `site/src/data/rivals.ts` and `site/src/pricing/prices.ts`.
 */
import { RESOLUTION_RATE } from "../../data/rivals";
import { bill } from "../../pricing/prices";

export type IntercomPlanId = "essential" | "advanced" | "expert";
export type BillingCycle = "annual" | "monthly";

export type IntercomCalcInput = {
  seats: number;
  conversations: number;
  aiShare: number;
  resolutionRate: number;
  plan: IntercomPlanId;
  billing: BillingCycle;
  promo: boolean;
  copilot: boolean;
  proAddOn: boolean;
  proactivePlus: boolean;
};

export const DEFAULT_CALC_INPUT: IntercomCalcInput = {
  seats: 5,
  conversations: 2000,
  aiShare: 0.8,
  resolutionRate: RESOLUTION_RATE,
  plan: "essential",
  billing: "annual",
  promo: false,
  copilot: false,
  proAddOn: false,
  proactivePlus: false,
};

export const INTERCOM_PLAN_RATES: Record<
  IntercomPlanId,
  { name: string; annual: number; monthly: number; annualPromo?: number; summary: string }
> = {
  essential: {
    name: "Essential",
    annual: 29,
    monthly: 39,
    annualPromo: 19,
    summary: "Shared inbox, Messenger, basic automation, public help center.",
  },
  advanced: {
    name: "Advanced",
    annual: 85,
    monthly: 99,
    summary: "Adds workflow builder, round robin, multiple team inboxes, 20 Lite seats.",
  },
  expert: {
    name: "Expert",
    annual: 132,
    monthly: 139,
    summary: "Adds SLAs, SAML SSO, multibrand help center, 50 Lite seats.",
  },
};

export const FIN_OUTCOME_PRICE = 0.99;
export const COPILOT_ANNUAL_PER_SEAT = 29;
export const COPILOT_MONTHLY_PER_SEAT = 35;
export const PRO_ADDON_MONTHLY = 100;
export const PROACTIVE_PLUS_MONTHLY = 99;

export type IntercomCalcResult = {
  seatUnit: number;
  seatCost: number;
  aiAnswered: number;
  finOutcomes: number;
  finCost: number;
  copilotCost: number;
  proCost: number;
  proactiveCost: number;
  addOnsCost: number;
  intercomMonthly: number;
  intercomYearly: number;
  dimmiCloudMonthly: number;
  dimmiCloudYearly: number;
  dimmiByokMonthly: number;
  dimmiByokYearly: number;
  dimmiMoreExpensive: boolean;
  monthlyDiff: number;
};

export function calculateIntercomBill(input: IntercomCalcInput): IntercomCalcResult {
  const seats = Math.max(1, Math.round(input.seats));
  const conversations = Math.max(0, Math.round(input.conversations));
  const aiShare = Math.min(1, Math.max(0, input.aiShare));
  const resolutionRate = Math.min(1, Math.max(0, input.resolutionRate));

  const planSpec = INTERCOM_PLAN_RATES[input.plan];
  const seatUnit =
    input.billing === "annual"
      ? input.promo && planSpec.annualPromo != null
        ? planSpec.annualPromo
        : planSpec.annual
      : planSpec.monthly;

  const seatCost = seats * seatUnit;
  const aiAnswered = Math.round(conversations * aiShare);
  const finOutcomes = aiAnswered * resolutionRate;
  const finCost = finOutcomes * FIN_OUTCOME_PRICE;

  const copilotCost = input.copilot
    ? seats * (input.billing === "annual" ? COPILOT_ANNUAL_PER_SEAT : COPILOT_MONTHLY_PER_SEAT)
    : 0;
  const proCost = input.proAddOn ? PRO_ADDON_MONTHLY : 0;
  const proactiveCost = input.proactivePlus ? PROACTIVE_PLUS_MONTHLY : 0;
  const addOnsCost = copilotCost + proCost + proactiveCost;

  const intercomMonthly = seatCost + finCost + addOnsCost;
  const intercomYearly = intercomMonthly * 12;

  const dimmiCloudMonthly = bill({ conversations, aiShare, operator: 0, ownModel: false }).total;
  const dimmiByokMonthly = bill({ conversations, aiShare, operator: 0, ownModel: true }).total;

  return {
    seatUnit,
    seatCost,
    aiAnswered,
    finOutcomes,
    finCost,
    copilotCost,
    proCost,
    proactiveCost,
    addOnsCost,
    intercomMonthly,
    intercomYearly,
    dimmiCloudMonthly,
    dimmiCloudYearly: dimmiCloudMonthly * 12,
    dimmiByokMonthly,
    dimmiByokYearly: dimmiByokMonthly * 12,
    dimmiMoreExpensive: dimmiCloudMonthly > intercomMonthly,
    monthlyDiff: intercomMonthly - dimmiCloudMonthly,
  };
}

function clampNum(raw: string | null, fallback: number, min: number, max: number): number {
  if (!raw) {
    return fallback;
  }
  const n = Number(raw);
  if (!Number.isFinite(n)) {
    return fallback;
  }
  return Math.min(max, Math.max(min, n));
}

export function parseCalcQuery(search: string): IntercomCalcInput {
  const params = new URLSearchParams(search);
  const rawPlan = params.get("plan");
  const plan: IntercomPlanId =
    rawPlan === "essential" || rawPlan === "advanced" || rawPlan === "expert" ? rawPlan : DEFAULT_CALC_INPUT.plan;
  const billing: BillingCycle = params.get("billing") === "monthly" ? "monthly" : "annual";

  return {
    seats: Math.round(clampNum(params.get("seats"), DEFAULT_CALC_INPUT.seats, 1, 100)),
    conversations: Math.round(clampNum(params.get("conv"), DEFAULT_CALC_INPUT.conversations, 100, 20000)),
    aiShare: clampNum(params.get("ai"), DEFAULT_CALC_INPUT.aiShare, 0, 1),
    resolutionRate: clampNum(params.get("res"), DEFAULT_CALC_INPUT.resolutionRate, 0.2, 0.9),
    plan,
    billing,
    promo: params.get("promo") === "1",
    copilot: params.get("copilot") === "1",
    proAddOn: params.get("pro") === "1",
    proactivePlus: params.get("proactive") === "1",
  };
}

export function buildCalcQuery(input: IntercomCalcInput): string {
  const params = new URLSearchParams();
  params.set("seats", String(input.seats));
  params.set("conv", String(input.conversations));
  params.set("ai", String(Number(input.aiShare.toFixed(2))));
  params.set("res", String(Number(input.resolutionRate.toFixed(2))));
  if (input.plan !== "essential") {
    params.set("plan", input.plan);
  }
  if (input.billing !== "annual") {
    params.set("billing", input.billing);
  }
  if (input.promo) {
    params.set("promo", "1");
  }
  if (input.copilot) {
    params.set("copilot", "1");
  }
  if (input.proAddOn) {
    params.set("pro", "1");
  }
  if (input.proactivePlus) {
    params.set("proactive", "1");
  }
  return `?${params.toString()}`;
}

export function trackCalcChange(field: string): void {
  if (typeof window === "undefined") {
    return;
  }
  const rt = (
    window as unknown as { rustrak?: { trackEvent?: (name: string, props?: Record<string, string>) => void } }
  ).rustrak;
  try {
    rt?.trackEvent?.("pricing_calc_change", { field });
  } catch {
    // Ignore analytics errors.
  }
}
