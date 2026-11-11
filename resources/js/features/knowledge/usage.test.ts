import { describe, expect, it } from "vitest";

import type { KnowledgeUsageLine } from "@/lib/api/knowledge-sources";

import { formatCost, usageDetail } from "./usage";

const line = (changes: Partial<KnowledgeUsageLine>): KnowledgeUsageLine => ({
  source_id: "ks_1",
  source_name: "Docs",
  source_type: "website",
  kind: "embedding",
  input_tokens: 0,
  estimated_tokens: 0,
  browser_ms: 0,
  requests: 1,
  cost_micros: 0,
  unpriced: false,
  ...changes,
});

describe("knowledge usage", () => {
  it("shows small costs to a tenth of a cent and larger ones to the cent", () => {
    expect(formatCost(0)).toBe("$0.00");
    expect(formatCost(17)).toBe("$0.000017");
    expect(formatCost(1200)).toBe("$0.0012");
    expect(formatCost(2_345_000)).toBe("$2.35");
  });

  it("says what a line used", () => {
    expect(usageDetail(line({ input_tokens: 120_000 }))).toBe("Embedding · 120,000 tokens");
    expect(usageDetail(line({ input_tokens: 40, estimated_tokens: 40 }))).toBe("Embedding · 40 tokens (estimated)");
    expect(usageDetail(line({ kind: "crawl", browser_ms: 570 }))).toBe("Crawling · 1 s of browser time");
    expect(usageDetail(line({ kind: "crawl", browser_ms: 900_000 }))).toBe("Crawling · 15.0 min of browser time");
  });
});
