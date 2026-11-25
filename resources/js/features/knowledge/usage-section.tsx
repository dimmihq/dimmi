/*
 * What the workspace's knowledge took this month: each source with the
 * tokens it took to embed and the browser time it took to crawl. The
 * workspace's own articles and conversations have their own line.
 * Tokens the provider did not count are marked estimated.
 */
import { useQuery } from "@tanstack/react-query";

import { Row, RowsSkeleton, Section } from "@/features/settings/layout";
import { type KnowledgeUsageLine, knowledgeUsageQuery } from "@/lib/api/knowledge-sources";

import { usageDetail } from "./usage";

export function UsageSection({ workspace }: { workspace: string }) {
  const usage = useQuery(knowledgeUsageQuery(workspace));
  const data = usage.data;

  return (
    <Section title="This month" description="What syncing your sources took." loading={usage.isPending}>
      {usage.isPending ? <RowsSkeleton rows={2} /> : null}
      {data && data.lines.length === 0 ? (
        <Row label="Nothing yet" description="Embedding and crawling show here as sources sync." muted />
      ) : null}
      {(data?.lines ?? []).map((line) => (
        <Row key={keyOf(line)} label={nameOf(line)} description={usageDetail(line)} />
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
