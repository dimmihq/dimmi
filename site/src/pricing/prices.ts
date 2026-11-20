/*
 * dimmi cloud's price book and the bill it produces. Prices are graduated
 * like tax brackets: each band's conversations are charged at that band's
 * price. AI conversations (customer-facing and Operator) and human
 * conversations each run up their own bands. The first 100 conversations of
 * the month are free, whatever their type. Bringing your own model key or
 * ChatGPT plan prices every AI conversation like a human one, because the
 * model is then paid for.
 */

export type Band = { upTo: number; ai: number; human: number };

/* Prices in cents for the conversations up to each band's limit. */
export const BANDS: Band[] = [
  { upTo: 500, ai: 10, human: 5 },
  { upTo: 1000, ai: 8, human: 4 },
  { upTo: 2500, ai: 8, human: 2 },
  { upTo: 5000, ai: 6, human: 1.5 },
  { upTo: 10000, ai: 6, human: 1 },
];

export const FREE_CONVERSATIONS = 100;
export const PUBLISHED_LIMIT = 10000;
export const EMAIL_PER_THOUSAND = 0.5;

/* Dollars for `count` conversations of one type, starting after `skip` free ones. */
function graduated(count: number, skip: number, price: (band: Band) => number): number {
  let cents = 0;
  let from = 0;
  for (const band of BANDS) {
    const inBand = Math.max(0, Math.min(count, band.upTo) - from);
    const free = Math.max(0, Math.min(skip, band.upTo) - from);
    cents += Math.max(0, inBand - free) * price(band);
    from = band.upTo;
  }
  // Past the published bands, the last band's price holds.
  const last = BANDS[BANDS.length - 1];
  cents += Math.max(0, count - Math.max(from, skip)) * price(last);
  return cents / 100;
}

export type Usage = {
  /* Customer conversations in the month. */
  conversations: number;
  /* Share of them the AI agent replies in, 0 to 1. */
  aiShare: number;
  /* Operator threads teammates start in the month. */
  operator: number;
  /* The workspace brings its own model key or ChatGPT plan. */
  ownModel: boolean;
};

export type Bill = {
  ai: number;
  human: number;
  aiCost: number;
  humanCost: number;
  /* What the free 100 saved. */
  waived: number;
  total: number;
  custom: boolean;
};

export function bill(usage: Usage): Bill {
  const customerAi = Math.round(usage.conversations * usage.aiShare);
  const ai = customerAi + usage.operator;
  const human = usage.conversations - customerAi;
  // The free 100 go to AI conversations first, then to human ones.
  const freeAi = Math.min(ai, FREE_CONVERSATIONS);
  const freeHuman = Math.min(human, FREE_CONVERSATIONS - freeAi);
  const aiPrice = (band: Band) => (usage.ownModel ? band.human : band.ai);
  const aiCost = graduated(ai, freeAi, aiPrice);
  const humanCost = graduated(human, freeHuman, (band) => band.human);
  const waived = graduated(ai, 0, aiPrice) + graduated(human, 0, (band) => band.human) - aiCost - humanCost;
  return {
    ai,
    human,
    aiCost,
    humanCost,
    waived,
    total: aiCost + humanCost,
    custom: ai > PUBLISHED_LIMIT || human > PUBLISHED_LIMIT,
  };
}

export function dollars(value: number): string {
  // Rounded to the cent first, so float noise never adds a stray ".00".
  const rounded = Math.round(value * 100) / 100;
  const whole = Number.isInteger(rounded) || rounded >= 1000;
  return rounded.toLocaleString("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: whole ? 0 : 2,
    maximumFractionDigits: whole ? 0 : 2,
  });
}

export function cents(value: number): string {
  return `${value % 1 === 0 ? value : value.toFixed(1)}¢`;
}
