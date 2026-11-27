/*
 * Regression tests locking `/pricing` numbers after moving competitor pricing
 * data into `site/src/data/rivals.ts`.
 */
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { Calculator } from "./calculator";
import { CompareBars } from "./comparison";
import { COMPETITORS, competitorBill, fullBill, SCENARIOS } from "./competitors";
import { bill } from "./prices";

describe("pricing comparison calculations", () => {
  it("preserves CompareBars scenario bills for dimmi and every competitor", () => {
    const matrix = SCENARIOS.map((scenario) => {
      const usage = {
        conversations: scenario.conversations,
        aiShare: scenario.aiShare,
        operator: 0,
        ownModel: false,
      };
      return {
        conversations: scenario.conversations,
        seats: scenario.seats,
        dimmi: Math.round(bill(usage).total),
        competitors: Object.fromEntries(
          COMPETITORS.map((c) => [
            c.name,
            {
              entry: Math.round(competitorBill(c, { ...usage, seats: scenario.seats })),
              full: Math.round(fullBill(c, { ...usage, seats: scenario.seats })),
            },
          ]),
        ),
      };
    });

    expect(matrix).toEqual([
      {
        conversations: 500,
        seats: 2,
        dimmi: 35,
        competitors: {
          Intercom: { entry: 296, full: 502 },
          Zendesk: { entry: 598, full: 738 },
          "Help Scout": { entry: 230, full: 270 },
          Front: { entry: 206, full: 366 },
          Freshdesk: { entry: 234, full: 306 },
        },
      },
      {
        conversations: 1000,
        seats: 3,
        dimmi: 74,
        competitors: {
          Intercom: { entry: 562, full: 871 },
          Zendesk: { entry: 1137, full: 1347 },
          "Help Scout": { entry: 435, full: 495 },
          Front: { entry: 387, full: 627 },
          Freshdesk: { entry: 449, full: 557 },
        },
      },
      {
        conversations: 2000,
        seats: 4,
        dimmi: 148,
        competitors: {
          Intercom: { entry: 1066, full: 1478 },
          Zendesk: { entry: 2156, full: 2436 },
          "Help Scout": { entry: 820, full: 900 },
          Front: { entry: 724, full: 1044 },
          Freshdesk: { entry: 860, full: 1004 },
        },
      },
      {
        conversations: 5000,
        seats: 6,
        dimmi: 335,
        competitors: {
          Intercom: { entry: 2550, full: 3168 },
          Zendesk: { entry: 5154, full: 5574 },
          "Help Scout": { entry: 1950, full: 2070 },
          Front: { entry: 1710, full: 2190 },
          Freshdesk: { entry: 2074, full: 2290 },
        },
      },
      {
        conversations: 10000,
        seats: 10,
        dimmi: 595,
        competitors: {
          Intercom: { entry: 5042, full: 6072 },
          Zendesk: { entry: 10190, full: 10890 },
          "Help Scout": { entry: 3850, full: 4050 },
          Front: { entry: 3370, full: 4170 },
          Freshdesk: { entry: 4110, full: 4470 },
        },
      },
    ]);
  });

  it("preserves Calculator default dimmi bill with standard AI and BYO keys", () => {
    const standard = bill({ conversations: 1000, aiShare: 0.7, operator: 20, ownModel: false });
    const byok = bill({ conversations: 1000, aiShare: 0.7, operator: 20, ownModel: true });

    expect(Math.round(standard.total)).toBe(73);
    expect(Math.round(byok.total)).toBe(44);
    expect(byok.total).toBeLessThan(standard.total);
  });

  it("renders CompareBars and Calculator with default numbers in static HTML", () => {
    const barsHtml = renderToStaticMarkup(createElement(CompareBars));
    expect(barsHtml).toContain("2,000 conversations, 4 seats");
    expect(barsHtml).toContain("$148");
    expect(barsHtml).toContain("$1,066");
    expect(barsHtml).toContain("$724");

    const calcHtml = renderToStaticMarkup(createElement(Calculator));
    expect(calcHtml).toContain("Intercom");
    expect(calcHtml).toContain("Zendesk");
  });
});
