/*
 * A source's documents, filtered to live, failed or excluded ones and by
 * words in their title or path. Each can be synced again now, set aside
 * so no later sync brings it back, or let back in. Why a document was
 * set aside or failed is said in plain words. Excluding asks first and
 * every action says what it did. The table keeps to two columns and a
 * menu, so it never scrolls sideways.
 */
import { RiAddCircleLine, RiIndeterminateCircleLine, RiRefreshLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { toast } from "@/components/base/toast/toast";
import { RowsSkeleton, Section } from "@/features/settings/layout";
import { type RowAction, RowMenu } from "@/features/shell/row-menu";
import { ApiError } from "@/lib/api/client";
import {
  type DocumentStatus,
  documentsQuery,
  excludeDocument,
  includeDocument,
  type KnowledgeDocument,
  type KnowledgeSource,
  resyncDocument,
} from "@/lib/api/knowledge-sources";

import { documentFilters, documentLabels, exclusionReason } from "./model";

export function DocumentsSection({
  workspace,
  source,
  canManage,
  canSync,
}: {
  workspace: string;
  source: KnowledgeSource;
  canManage: boolean;
  canSync: boolean;
}) {
  const [status, setStatus] = useState<DocumentStatus | "">("");
  const [search, setSearch] = useState("");
  const docs = useQuery(documentsQuery(workspace, source.id, status, search.trim(), source.status === "syncing"));

  return (
    <Section title="Documents" description={`${source.document_count} live`}>
      <div className="flex flex-wrap items-center gap-3 py-3 pe-3">
        <SegmentedControl
          aria-label="Show documents"
          selectedKeys={[status || "all"]}
          onSelectionChange={(keys) => {
            const key = [...keys][0];
            if (key !== undefined) {
              setStatus(key === "all" ? "" : (String(key) as DocumentStatus));
            }
          }}
        >
          {documentFilters.map((f) => (
            <SegmentedControlItem key={f.label} id={f.id || "all"}>
              {f.label}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
        <Input
          aria-label="Search documents"
          size="small"
          placeholder="Search"
          value={search}
          onChange={setSearch}
          className="w-48"
        />
      </div>
      {docs.isPending ? (
        <RowsSkeleton rows={3} />
      ) : (
        <DocumentTable
          workspace={workspace}
          source={source}
          documents={docs.data?.items ?? []}
          canManage={canManage}
          canSync={canSync}
        />
      )}
    </Section>
  );
}

function DocumentTable({
  workspace,
  source,
  documents,
  canManage,
  canSync,
}: {
  workspace: string;
  source: KnowledgeSource;
  documents: KnowledgeDocument[];
  canManage: boolean;
  canSync: boolean;
}) {
  const client = useQueryClient();
  const [excluding, setExcluding] = useState<KnowledgeDocument | null>(null);
  const done = () => client.invalidateQueries({ queryKey: ["workspace", workspace, "knowledge"] });
  const act = useMutation({
    mutationFn: ({ doc, action }: { doc: KnowledgeDocument; action: DocumentAction }) => {
      if (action === "exclude") {
        return excludeDocument(workspace, source.id, doc.id);
      }
      return action === "include"
        ? includeDocument(workspace, source.id, doc.id).then(() => undefined)
        : resyncDocument(workspace, source.id, doc.id).then(() => undefined);
    },
    onSuccess: async (_, { doc, action }) => {
      await done();
      toast.success(doneMessages[action](doc.title || doc.path));
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "That did not work. Try again."),
  });

  return (
    <>
      <Table aria-label="Documents" size="sm" className="[&_th]:border-t-0">
        <TableHeader>
          <TableColumn isRowHeader>Document</TableColumn>
          <TableColumn>Status</TableColumn>
          <TableColumn>
            <span className="sr-only">Actions</span>
          </TableColumn>
        </TableHeader>
        <TableBody
          items={documents}
          renderEmptyState={() => <p className="py-2 text-body-2-regular text-text-tertiary">No documents here.</p>}
        >
          {(doc) => (
            <TableRow id={doc.id}>
              <TableCell>
                <DocumentName doc={doc} />
              </TableCell>
              <TableCell>
                <DocumentStatusCell doc={doc} />
              </TableCell>
              <TableCell>
                <DocumentActions
                  doc={doc}
                  canManage={canManage}
                  canSync={canSync}
                  onAct={(action) => (action === "exclude" ? setExcluding(doc) : act.mutate({ doc, action }))}
                />
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
      <AlertDialog
        isOpen={excluding !== null}
        onOpenChange={(open) => (open ? undefined : setExcluding(null))}
        title={excluding ? `Exclude ${excluding.title || excluding.path}?` : "Exclude document?"}
        description="dimmi stops reading it and later syncs leave it out. You can include it again any time."
        confirmLabel="Exclude"
        onConfirm={async () => {
          if (excluding) {
            await act.mutateAsync({ doc: excluding, action: "exclude" });
          }
        }}
      />
    </>
  );
}

const doneMessages: Record<DocumentAction, (title: string) => string> = {
  exclude: (title) => `${title} excluded. dimmi no longer reads it.`,
  include: (title) => `${title} included. The next sync reads it again.`,
  resync: (title) => `Syncing ${title} again.`,
};

function DocumentName({ doc }: { doc: KnowledgeDocument }) {
  const title = doc.title || doc.path;
  const chunks = doc.chunk_count > 0 ? ` · ${doc.chunk_count} ${doc.chunk_count === 1 ? "chunk" : "chunks"}` : "";

  return (
    <span className="flex min-w-0 flex-col">
      {doc.url ? (
        <a
          href={doc.url}
          target="_blank"
          rel="noreferrer"
          className="truncate text-body-2-medium text-text-primary hover:underline"
        >
          {title}
        </a>
      ) : (
        <span className="truncate text-body-2-medium text-text-primary">{title}</span>
      )}
      <span className="truncate text-caption-1-regular text-text-tertiary">
        <bdi dir="ltr">{doc.path}</bdi>
        {doc.locale ? ` · ${doc.locale}` : ""}
        {chunks}
      </span>
    </span>
  );
}

function DocumentStatusCell({ doc }: { doc: KnowledgeDocument }) {
  const why = doc.error ?? (doc.status === "excluded" ? exclusionReason(doc.excluded_by) : "");

  return (
    <span className="flex flex-col items-start gap-0.5">
      <Chip variant="caption" color={documentLabels[doc.status].color}>
        {documentLabels[doc.status].label}
      </Chip>
      {why ? <span className="text-caption-1-regular text-text-tertiary">{why}</span> : null}
    </span>
  );
}

type DocumentAction = "resync" | "exclude" | "include";

function DocumentActions({
  doc,
  canManage,
  canSync,
  onAct,
}: {
  doc: KnowledgeDocument;
  canManage: boolean;
  canSync: boolean;
  onAct: (action: DocumentAction) => void;
}) {
  const excluded = doc.status === "excluded";
  const actions: RowAction<DocumentAction>[] = [
    ...(canSync && !excluded ? [{ kind: "resync" as const, label: "Sync again", icon: RiRefreshLine }] : []),
    ...(canManage && excluded ? [{ kind: "include" as const, label: "Include", icon: RiAddCircleLine }] : []),
    ...(canManage && !excluded
      ? [{ kind: "exclude" as const, label: "Exclude", icon: RiIndeterminateCircleLine, danger: true }]
      : []),
  ];

  if (actions.length === 0) {
    return null;
  }

  return (
    <span className="flex justify-end">
      <RowMenu quiet label={doc.title || doc.path} actions={actions} onPick={onAct} />
    </span>
  );
}
