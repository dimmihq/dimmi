/*
 * Past imports of one kind, newest first: the file, who ran it and when,
 * what it came to, the rows with problems to download while the file is
 * kept, and an undo for a week after it finishes. Undoing asks first,
 * saying exactly what will be removed and what put back, then runs in
 * the background while the list follows it to what it removed, put back
 * and kept, each kept record named with why.
 */
import { RiArrowGoBackLine, RiDownloadLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Badge } from "@/components/base/badges/badge";
import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { ProgressBar } from "@/components/base/progress/progress";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import {
  downloadImportErrors,
  type ImportKind,
  type ImportSummary,
  importsQuery,
  rollbackImport,
} from "@/lib/api/imports";
import { formatDateTime, formatNumber } from "@/lib/i18n/format";

import { busy, byLine, canUndo, countsLine, keptMore, statusLabel, undoneLine, undoPlan } from "./import-history-text";

const pollEvery = 1000;
const keptShown = 5;

function save(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

function Rollback({ item }: { item: ImportSummary }) {
  const r = item.rollback;
  if (r === null) {
    return null;
  }
  if (r.status === "queued" || r.status === "running") {
    return (
      <ProgressBar
        size="sm"
        label="Undoing"
        value={r.processed}
        maxValue={Math.max(r.total, 1)}
        valueLabel={`${formatNumber(r.processed)} of ${formatNumber(r.total)}`}
      />
    );
  }

  return (
    <div className="flex flex-col gap-1">
      <p className="text-body-2-regular text-text-secondary">{undoneLine(item)}</p>
      {r.error ? <p className="text-body-2-regular text-text-error-primary">{r.error}</p> : null}
      {r.kept_records.length > 0 ? (
        <ul aria-label="Kept records" className="flex flex-col gap-0.5">
          {r.kept_records.slice(0, keptShown).map((k) => (
            <li key={k.id} className="text-body-2-regular text-text-tertiary">
              <span className="text-text-secondary">{k.name}</span> · {k.reason}
            </li>
          ))}
          {keptMore(item, keptShown) ? (
            <li className="text-body-2-regular text-text-tertiary">{keptMore(item, keptShown)}</li>
          ) : null}
        </ul>
      ) : null}
    </div>
  );
}

function Row({
  item,
  mayUndo,
  onUndo,
  onDownload,
}: {
  item: ImportSummary;
  mayUndo: boolean;
  onUndo: () => void;
  onDownload: () => void;
}) {
  const expired = item.file_expired_at !== null;

  return (
    <li className="flex flex-col gap-2 py-3" aria-label={item.file_name}>
      <div className="flex items-center gap-2">
        <p className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{item.file_name}</p>
        <Badge className="shrink-0">{statusLabel(item)}</Badge>
      </div>
      <p className="text-body-2-regular text-text-tertiary">{byLine(item, formatDateTime(item.created_at))}</p>
      <p className="text-body-2-regular text-text-secondary">{countsLine(item)}</p>
      <Rollback item={item} />
      <div className="flex flex-wrap items-center gap-2">
        {item.failed_count > 0 && !expired ? (
          <Button variant="secondary" size="small" leadingIcon={RiDownloadLine} onClick={onDownload}>
            Rows with problems
          </Button>
        ) : null}
        {item.failed_count > 0 && expired ? (
          <p className="text-body-2-regular text-text-tertiary">The file expired 30 days after the import.</p>
        ) : null}
        {mayUndo && canUndo(item, new Date()) ? (
          <Button variant="danger-secondary" size="small" leadingIcon={RiArrowGoBackLine} onClick={onUndo}>
            Undo import
          </Button>
        ) : null}
      </div>
      {mayUndo && canUndo(item, new Date()) && item.rollback_until ? (
        <p className="text-body-2-regular text-text-tertiary">
          Can be undone until {formatDateTime(item.rollback_until)}.
        </p>
      ) : null}
    </li>
  );
}

function UndoDialog({
  item,
  noun,
  onUndo,
  onClose,
}: {
  item: ImportSummary;
  noun: string;
  onUndo: () => Promise<unknown>;
  onClose: () => void;
}) {
  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={`Undo the import of ${item.file_name}?`}
      description="Here is exactly what happens:"
      confirmLabel="Undo import"
      onConfirm={onUndo}
    >
      <ul className="flex list-disc flex-col gap-1.5 ps-5">
        {undoPlan(item, noun).map((line) => (
          <li key={line} className="text-body-2-regular text-text-secondary">
            {line}
          </li>
        ))}
      </ul>
    </AlertDialog>
  );
}

/*
 * ImportHistory lists the past imports of the kind, with their problem
 * rows and an undo for members who may undo imports.
 */
export function ImportHistory({
  workspace,
  kind,
  noun,
  mayUndo,
}: {
  workspace: string;
  kind: ImportKind;
  noun: string;
  mayUndo: boolean;
}) {
  const client = useQueryClient();
  const [undoing, setUndoing] = useState<ImportSummary | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const list = useQuery({
    ...importsQuery(workspace, kind),
    refetchInterval: (query) => ((query.state.data ?? []).some(busy) ? pollEvery : false),
  });
  const undo = useMutation({
    mutationFn: (id: string) => rollbackImport(workspace, id),
    onSuccess: () => client.invalidateQueries({ queryKey: importsQuery(workspace, kind).queryKey }),
    onError: (error) => {
      setFailure(error.message);
      setUndoing(null);
    },
  });
  const items = (list.data ?? []).filter((i) => i.status !== "draft");
  const download = (item: ImportSummary) => {
    setFailure(null);
    downloadImportErrors(workspace, item.id)
      .then((f) => save(f.blob, f.filename))
      .catch((error: unknown) => setFailure(error instanceof Error ? error.message : String(error)));
  };

  if (list.isPending) {
    return <Skeleton className="h-24 w-full" />;
  }

  return (
    <div className="flex flex-col gap-3">
      {items.length === 0 ? (
        <p className="text-body-2-regular text-text-tertiary">No {noun} have been imported yet.</p>
      ) : (
        <ul aria-label="Past imports" className="flex flex-col divide-y divide-separator-border">
          {items.map((item) => (
            <Row
              key={item.id}
              item={item}
              mayUndo={mayUndo}
              onUndo={() => {
                setFailure(null);
                setUndoing(item);
              }}
              onDownload={() => download(item)}
            />
          ))}
        </ul>
      )}
      {failure ? (
        <p role="alert" className="text-body-2-regular text-text-error-primary">
          {failure}
        </p>
      ) : null}
      {undoing ? (
        <UndoDialog
          item={undoing}
          noun={noun}
          onUndo={() => undo.mutateAsync(undoing.id)}
          onClose={() => setUndoing(null)}
        />
      ) : null}
    </div>
  );
}
