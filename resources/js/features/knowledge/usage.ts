/*
 * How knowledge usage reads: a cost in dollars from millionths of a
 * dollar, down to the millionth when it is tiny, and a line's
 * tokens or browser time in words.
 */
import type { KnowledgeUsageLine } from "@/lib/api/knowledge-sources";

export function formatCost(micros: number): string {
  if (micros === 0) {
    return "$0.00";
  }
  const dollars = micros / 1_000_000;
  if (dollars < 0.0001) {
    return `$${dollars.toFixed(6)}`;
  }
  if (dollars < 0.01) {
    return `$${dollars.toFixed(4)}`;
  }

  return `$${dollars.toFixed(2)}`;
}

export function usageDetail(line: KnowledgeUsageLine): string {
  if (line.kind === "crawl") {
    const minutes = line.browser_ms / 60_000;
    const time = minutes < 1 ? `${Math.max(1, Math.round(line.browser_ms / 1000))} s` : `${minutes.toFixed(1)} min`;

    return `Crawling · ${time} of browser time`;
  }
  const tokens = line.input_tokens.toLocaleString("en-US");
  const estimated = line.estimated_tokens > 0 ? " (estimated)" : "";

  return `Embedding · ${tokens} tokens${estimated}`;
}
