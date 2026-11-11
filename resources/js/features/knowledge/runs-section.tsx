/*
 * A source's latest syncs, newest first: when each started, what started
 * it, how it went and what it did, with the error of one that failed.
 * When and what started a sync share a column, so the table fits
 * without scrolling sideways.
 */
import { useQuery } from "@tanstack/react-query";

import { Chip } from "@/components/base/badges/chip";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { RowsSkeleton, Section } from "@/features/settings/layout";
import { type KnowledgeSource, runsQuery } from "@/lib/api/knowledge-sources";

import { ago, runLabels, runSummary } from "./model";

const triggers: Record<string, string> = {
  create: "First sync",
  manual: "Sync now",
  schedule: "Schedule",
  webhook: "Push",
  retry: "Retry",
  document: "One document",
};

export function RunsSection({ workspace, source }: { workspace: string; source: KnowledgeSource }) {
  const runs = useQuery(runsQuery(workspace, source.id, source.status === "syncing"));

  return (
    <Section title="Sync history" loading={runs.isPending}>
      {runs.isPending ? (
        <RowsSkeleton rows={2} />
      ) : (
        <div className="-ms-3">
          <Table aria-label="Sync history" size="sm" className="[&_th]:border-t-0">
            <TableHeader>
              <TableColumn isRowHeader>Started</TableColumn>
              <TableColumn>Result</TableColumn>
              <TableColumn>What changed</TableColumn>
            </TableHeader>
            <TableBody
              items={runs.data ?? []}
              renderEmptyState={() => <p className="py-2 text-body-2-regular text-text-tertiary">No syncs yet.</p>}
            >
              {(run) => (
                <TableRow id={run.id}>
                  <TableCell>
                    <span className="flex flex-col">
                      <span className="whitespace-nowrap text-body-2-regular text-text-primary">
                        {ago(run.started_at ?? run.created_at)}
                      </span>
                      <span className="text-caption-1-regular text-text-tertiary">
                        {triggers[run.trigger] ?? run.trigger}
                      </span>
                    </span>
                  </TableCell>
                  <TableCell>
                    <Chip variant="caption" color={runLabels[run.status].color}>
                      {runLabels[run.status].label}
                    </Chip>
                  </TableCell>
                  <TableCell>
                    <span className="flex flex-col">
                      <span className="text-body-2-regular text-text-secondary">{runSummary(run)}</span>
                      {run.error ? (
                        <span className="text-caption-1-regular text-text-error-primary">{run.error}</span>
                      ) : null}
                    </span>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      )}
    </Section>
  );
}
