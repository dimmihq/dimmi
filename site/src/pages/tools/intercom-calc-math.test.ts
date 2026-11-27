import { describe, expect, it } from "vitest";

import { competitorBill, fullBill, getRival, RESOLUTION_RATE } from "../../data/rivals";
import { bill } from "../../pricing/prices";
import { buildCalcQuery, calculateIntercomBill, DEFAULT_CALC_INPUT, parseCalcQuery } from "./intercom-calc-math";

describe("Intercom cost calculator math", () => {
  it("matches competitorBill, fullBill, and bill for the same inputs", () => {
    const intercom = getRival("intercom").competitor;
    expect(intercom).toBeDefined();
    if (!intercom) {
      return;
    }

    const usage = { conversations: 2000, aiShare: 0.8, operator: 0, ownModel: false, seats: 5 };
    const essentialCalc = calculateIntercomBill({
      ...DEFAULT_CALC_INPUT,
      seats: 5,
      conversations: 2000,
      aiShare: 0.8,
      resolutionRate: RESOLUTION_RATE,
      plan: "essential",
      billing: "annual",
      promo: false,
    });

    expect(essentialCalc.intercomMonthly).toBeCloseTo(competitorBill(intercom, usage), 2);
    expect(essentialCalc.dimmiCloudMonthly).toBeCloseTo(bill(usage).total, 2);
    expect(essentialCalc.dimmiByokMonthly).toBeCloseTo(bill({ ...usage, ownModel: true }).total, 2);

    const expertCalc = calculateIntercomBill({
      ...DEFAULT_CALC_INPUT,
      seats: 5,
      conversations: 2000,
      aiShare: 0.8,
      resolutionRate: RESOLUTION_RATE,
      plan: "expert",
      billing: "annual",
      promo: false,
    });
    expect(expertCalc.intercomMonthly).toBeCloseTo(fullBill(intercom, usage), 2);
  });

  it("applies the $19 Essential promo, monthly rates, and add-ons accurately", () => {
    const promo = calculateIntercomBill({
      ...DEFAULT_CALC_INPUT,
      seats: 5,
      conversations: 0,
      aiShare: 0,
      plan: "essential",
      billing: "annual",
      promo: true,
    });
    expect(promo.seatCost).toBe(95);

    const withAddOns = calculateIntercomBill({
      ...DEFAULT_CALC_INPUT,
      seats: 4,
      conversations: 0,
      aiShare: 0,
      plan: "essential",
      billing: "monthly",
      copilot: true,
      proAddOn: true,
      proactivePlus: true,
    });
    expect(withAddOns.seatCost).toBe(4 * 39);
    expect(withAddOns.copilotCost).toBe(4 * 35);
    expect(withAddOns.proCost).toBe(100);
    expect(withAddOns.proactiveCost).toBe(99);
    expect(withAddOns.intercomMonthly).toBe(4 * 39 + 4 * 35 + 100 + 99);
  });

  it("flags when dimmi Cloud costs more for a small manual-only team", () => {
    const manual = calculateIntercomBill({
      ...DEFAULT_CALC_INPUT,
      seats: 1,
      conversations: 2000,
      aiShare: 0,
      plan: "essential",
      billing: "annual",
      promo: true,
    });
    expect(manual.intercomMonthly).toBe(19);
    expect(manual.dimmiCloudMonthly).toBeGreaterThan(manual.intercomMonthly);
    expect(manual.dimmiMoreExpensive).toBe(true);
  });

  it("round-trips shareable URL query parameters", () => {
    const input = {
      ...DEFAULT_CALC_INPUT,
      seats: 8,
      conversations: 4000,
      aiShare: 0.75,
      resolutionRate: 0.55,
      plan: "advanced" as const,
      billing: "monthly" as const,
      copilot: true,
      proAddOn: true,
    };
    const qs = buildCalcQuery(input);
    expect(parseCalcQuery(qs)).toEqual(input);
  });
});
