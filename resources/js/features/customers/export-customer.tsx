/*
 * Exporting a customer from their page, to answer a request for a copy
 * of their data. The export is built in the background; the button waits
 * for it and then saves the zip.
 */
import { RiDownload2Line } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { type Contact, downloadExport, exportQuery, requestExport } from "@/lib/api/customers";

import { useWorkspace } from "./names";

const pollEvery = 1000;

function save(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

export function ExportCustomer({ contact }: { contact: Contact }) {
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
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiDownload2Line}
        loading={download.isPending}
        onClick={() => download.mutate()}
      >
        Download export
      </Button>
    );
  }

  const waiting = start.isPending || current?.status === "pending" || current?.status === "running";

  return (
    <Button
      variant="secondary"
      size="small"
      leadingIcon={RiDownload2Line}
      disabled={waiting}
      onClick={() => start.mutate()}
    >
      {waiting ? "Preparing export…" : current?.status === "failed" ? "Export failed, try again" : "Export data"}
    </Button>
  );
}
