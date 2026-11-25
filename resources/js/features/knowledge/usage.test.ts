import { describe, expect, it } from "vitest";

import type { KnowledgeUsageLine } from "@/lib/api/knowledge-sources";

import { usageDetail } from "./usage";

const line = (changes: Partial<KnowledgeUsageLine>): KnowledgeUsageLine => ({
  source_id: "ks_1",
  source_name: "Docs",
  source_type: "website",
  kind: "embedding",
  input_tokens: 0,
  estimated_tokens: 0,
  browser_ms: 0,
  requests: 1,
  ...changes,
});

describe("knowledge usage", () => {
  it("says what a line used", () => {
    expect(usageDetail(line({ input_tokens: 120_000 }))).toBe("Embedding · 120,000 tokens");
    expect(usageDetail(line({ input_tokens: 40, estimated_tokens: 40 }))).toBe("Embedding · 40 tokens (estimated)");
    expect(usageDetail(line({ kind: "crawl", browser_ms: 570 }))).toBe("Crawling · 1 s of browser time");
    expect(usageDetail(line({ kind: "crawl", browser_ms: 900_000 }))).toBe("Crawling · 15.0 min of browser time");
  });
});
