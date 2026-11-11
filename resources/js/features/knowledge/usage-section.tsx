/*
 * What the workspace's knowledge cost this month: the total, then each
 * source with the tokens it took to embed and the browser time it took
 * to crawl, the costliest first. The workspace's own articles and
 * conversations have their own line. Tokens the provider did not count
 * are marked estimated, and usage without a price says so instead of
 * passing for free.
 */
import { useQuery } from "@tanstack/react-query";

import { Row, RowsSkeleton, Section } from "@/features/settings/layout";
import { type KnowledgeUsageLine, knowledgeUsageQuery } from "@/lib/api/knowledge-sources";

import { formatCost, usageDetail } from "./usage";

export function UsageSection({ workspace }: { workspace: string }) {
  const usage = useQuery(knowledgeUsageQuery(workspace));
  const data = usage.data;
  const summary = data
    ? `${formatCost(data.cost_micros)} for embedding and crawling${data.unpriced ? ", part of it unpriced" : ""}.`
    : undefined;

  return (
    <Section title="This month" description={summary} loading={usage.isPending}>
      {usage.isPending ? <RowsSkeleton rows={2} /> : null}
      {data && data.lines.length === 0 ? (
        <Row label="Nothing yet" description="Embedding and crawling costs show here as sources sync." muted />
      ) : null}
      {(data?.lines ?? []).map((line) => (
        <Row key={keyOf(line)} label={nameOf(line)} description={usageDetail(line)}>
          <span className="text-body-regular text-text-primary tabular-nums">
            {line.unpriced ? "Unpriced" : formatCost(line.cost_micros)}
          </span>
        </Row>
      ))}
    </Section>
  );
}

function keyOf(line: KnowledgeUsageLine): string {
  return `${line.source_id ?? "own"}:${line.kind}`;
}

function nameOf(line: KnowledgeUsageLine): string {
  if (line.source_id === null) {
    return "Help center articles and conversations";
  }

  return line.source_name || "Removed source";
}
