/*
 * Exporting what a list shows: the conversations of a view or the
 * customers of the directory, with its filters and search. The member
 * picks a format; notes go in only when they may moderate the team's
 * conversations. The file is built in the background and waits on the
 * Exports page, so the dialog only says where to find it. A selection
 * larger than the format allows is refused with the server's reason.
 */
import { RiBracesLine, RiDownload2Line, RiFileTextLine, RiTableLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Focusable } from "react-aria-components";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Switch } from "@/components/base/switch/switch";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { ButtonLink } from "@/features/navigation/links";
import {
  dataExportsKey,
  type ExportFormat,
  type ExportKind,
  type ExportSelection,
  formatsOf,
  requestDataExport,
} from "@/lib/api/data-exports";
import { callerQuery } from "@/lib/api/workspace";

const formatIcons: Record<ExportFormat, typeof RiTableLine> = {
  txt: RiFileTextLine,
  csv: RiTableLine,
  json: RiBracesLine,
};

const permissionOf: Record<ExportKind, string> = {
  conversations: "conversations:export",
  contacts: "contacts:export",
};

/*
 * ExportButton opens the export dialog for the list; compact draws it as
 * an icon, for a list header beside the view's own actions, and "table"
 * as the outlined icon button of a table's action group, with a tooltip.
 */
export function ExportButton({
  workspace,
  kind,
  selection,
  compact = false,
}: {
  workspace: string;
  kind: ExportKind;
  selection: ExportSelection;
  compact?: boolean | "table";
}) {
  const [open, setOpen] = useState(false);
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const permissions = caller?.member?.permissions ?? [];

  if (!permissions.includes(permissionOf[kind])) {
    return null;
  }

  return (
    <>
      {compact === "table" ? (
        <TooltipTrigger delay={200}>
          <Focusable>
            <IconButton icon={RiDownload2Line} aria-label="Export" onClick={() => setOpen(true)} />
          </Focusable>
          <Tooltip size="md">Export</Tooltip>
        </TooltipTrigger>
      ) : compact ? (
        <Button
          iconOnly
          size="xs"
          variant="secondary"
          leadingIcon={RiDownload2Line}
          aria-label={`Export ${kind === "contacts" ? "customers" : "conversations"}`}
          onClick={() => setOpen(true)}
        />
      ) : (
        <Button variant="secondary" size="small" leadingIcon={RiDownload2Line} onClick={() => setOpen(true)}>
          Export
        </Button>
      )}
      {open ? (
        <ExportDialog
          workspace={workspace}
          kind={kind}
          selection={selection}
          notes={kind === "conversations" && permissions.includes("conversations:moderate")}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </>
  );
}

function ExportDialog({
  workspace,
  kind,
  selection,
  notes,
  onClose,
}: {
  workspace: string;
  kind: ExportKind;
  selection: ExportSelection;
  notes: boolean;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const formats = formatsOf(kind);
  const [format, setFormat] = useState<ExportFormat>(formats[0].id);
  const [withNotes, setWithNotes] = useState(false);
  const start = useMutation({
    mutationFn: () => requestDataExport(workspace, { kind, format, include_notes: withNotes, selection }),
    onSuccess: () => client.invalidateQueries({ queryKey: dataExportsKey(workspace) }),
  });
  const what = kind === "contacts" ? "customers" : "conversations";

  return (
    <Modal size="sm" isOpen onOpenChange={(isOpen) => (isOpen ? undefined : onClose())}>
      <ModalHeader
        title={start.isSuccess ? "Export started" : `Export these ${what}`}
        description={
          start.isSuccess
            ? "It is being prepared in the background. Download it from Exports once it is ready."
            : "Everything this list shows that you can see, as one file."
        }
      />
      {start.isSuccess ? null : (
        <ModalBody>
          <div className="flex flex-col gap-3">
            <RadioGroup
              aria-label="Format"
              value={format}
              onChange={(next) => setFormat(next as ExportFormat)}
              className="flex flex-col gap-2"
            >
              {formats.map((f) => (
                <RadioCard key={f.id} value={f.id} icon={formatIcons[f.id]} title={f.label} description={f.hint} />
              ))}
            </RadioGroup>
            {notes && format !== "csv" ? (
              <Switch size="sm" isSelected={withNotes} onChange={setWithNotes}>
                Include internal notes
              </Switch>
            ) : null}
            {start.isError ? (
              <p role="alert" className="text-body-2-regular text-text-error-primary">
                {start.error.message}
              </p>
            ) : null}
          </div>
        </ModalBody>
      )}
      <ModalFooter>
        {start.isSuccess ? (
          <>
            <Button variant="secondary" onClick={onClose}>
              Close
            </Button>
            <ButtonLink to="/d/settings/exports" variant="primary" onClick={onClose}>
              Go to Exports
            </ButtonLink>
          </>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button variant="primary" disabled={start.isPending} onClick={() => start.mutate()}>
              {start.isPending ? "Starting…" : "Export"}
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
