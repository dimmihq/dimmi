/*
 * How knowledge usage reads: a line's tokens or browser time in words.
 */
import type { KnowledgeUsageLine } from "@/lib/api/knowledge-sources";

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
