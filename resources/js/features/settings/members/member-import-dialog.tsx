/*
 * Changing many members at once from a spreadsheet. The admin picks a
 * CSV file, the server checks it first without changing anything, and
 * only then can the admin apply it, knowing what will happen and which
 * rows have problems. Applying asks for the password again when the
 * last sign-in is a while ago.
 */
import { RiUploadLine } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { FileUpload } from "@/components/base/file-upload/file-upload";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { importMembers, type MemberImport } from "@/lib/api/workspace";

import { useRecentAuth } from "../recent-auth";
import { hasWork, problems, summary } from "./member-import";

const maxBytes = 2 * 1024 * 1024;

function Report({ report }: { report: MemberImport }) {
  const rows = problems(report);

  return (
    <div className="flex flex-col gap-2">
      <p role="status" className="text-body-2-regular text-text-primary">
        {summary(report)}
      </p>
      {rows.length > 0 ? (
        <ul aria-label="Rows with problems" className="flex flex-col gap-1">
          {rows.map((line) => (
            <li key={line} className="text-body-2-regular text-text-error-primary">
              {line}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

export function MemberImportButton({ workspace, hasPassword }: { workspace: string; hasPassword: boolean }) {
  const queryClient = useQueryClient();
  const { run, prompt } = useRecentAuth(hasPassword);
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [report, setReport] = useState<MemberImport | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const close = () => {
    setOpen(false);
    setFile(null);
    setReport(null);
    setFailure(null);
  };

  const send = async (picked: File, dryRun: boolean) => {
    setFailure(null);
    setPending(true);
    try {
      await run(async () => {
        setReport(await importMembers(workspace, picked, dryRun));
        if (!dryRun) {
          await queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
        }
      });
    } catch (error) {
      setFailure(error instanceof Error ? error.message : "Something went wrong. Try again.");
    } finally {
      setPending(false);
    }
  };

  const pick = (picked: File) => {
    setFile(picked);
    void send(picked, true);
  };

  const applied = report !== null && !report.dry_run;

  return (
    <>
      <Button variant="secondary" size="small" leadingIcon={RiUploadLine} onClick={() => setOpen(true)}>
        Import
      </Button>
      <Modal isOpen={open} onOpenChange={(next) => (next ? setOpen(true) : close())}>
        <ModalHeader
          title="Import members"
          description="A CSV file with an email column, and any of role, teams and status. Empty cells change nothing."
        />
        <ModalBody className="flex flex-col gap-4">
          {file === null ? (
            <FileUpload allowedExtensions={["csv"]} maxBytes={maxBytes} onUploadComplete={pick} />
          ) : (
            <p className="truncate text-body-2-medium text-text-secondary">{file.name}</p>
          )}
          {report ? <Report report={report} /> : null}
          {failure ? (
            <p role="alert" className="text-body-2-regular text-text-error-primary">
              {failure}
            </p>
          ) : null}
        </ModalBody>
        <ModalFooter className="mt-6">
          <Button variant="secondary" onClick={close}>
            {applied ? "Done" : "Cancel"}
          </Button>
          {applied ? null : (
            <Button
              disabled={file === null || report === null || !hasWork(report) || pending}
              onClick={() => (file ? void send(file, false) : undefined)}
            >
              Apply changes
            </Button>
          )}
        </ModalFooter>
      </Modal>
      {prompt}
    </>
  );
}
