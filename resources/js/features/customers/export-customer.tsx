/*
 * Exporting a customer from the actions of their row, to answer a
 * request for a copy of their data. The export is built in the
 * background; the dialog waits for it and then saves the zip.
 */
import { RiDownload2Line } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { type Contact, downloadExport, exportQuery, requestExport } from "@/lib/api/customers";

import { personName, useWorkspace } from "./names";

const pollEvery = 1000;

function save(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

function ExportCustomer({ contact }: { contact: Contact }) {
  const workspace = useWorkspace();
  const [id, setId] = useState<string | null>(null);
  const start = useMutation({
    mutationFn: () => requestExport(workspace, contact.id),
    onSuccess: (started) => setId(started.id),
  });
  const { data: current } = useQuery({
    ...exportQuery(workspace, contact.id, id ?? ""),
    enabled: id !== null,
    refetchInterval: (query) =>
      query.state.data?.status === "pending" || query.state.data?.status === "running" ? pollEvery : false,
  });
  const download = useMutation({
    mutationFn: async () => {
      if (!current?.url) {
        return;
      }
      save(await downloadExport(workspace, current.url), `${current.id}.zip`);
    },
  });

  if (current?.status === "ready") {
    return (
      <Button leadingIcon={RiDownload2Line} loading={download.isPending} onClick={() => download.mutate()}>
        Download export
      </Button>
    );
  }

  const waiting = start.isPending || current?.status === "pending" || current?.status === "running";

  return (
    <Button leadingIcon={RiDownload2Line} disabled={waiting} onClick={() => start.mutate()}>
      {waiting ? "Preparing export…" : current?.status === "failed" ? "Export failed, try again" : "Export data"}
    </Button>
  );
}

/*
 * ExportDialog prepares a copy of everything dimmi holds about the
 * customer and saves it once it is ready.
 */
export function ExportDialog({ contact, onClose }: { contact: Contact; onClose: () => void }) {
  return (
    <Modal size="sm" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={`Export ${personName(contact)}'s data`}
        description="A zip of their details, conversations and files, to answer a request for a copy."
      />
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
        <ExportCustomer contact={contact} />
      </ModalFooter>
    </Modal>
  );
}
