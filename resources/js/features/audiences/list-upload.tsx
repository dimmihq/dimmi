/*
 * Upload a list: a CSV with an email, external_id or user_id column puts
 * one tag on every customer it names, which audiences, filters and
 * workflows can then use. Customers the workspace does not have are
 * counted and a few shown, never created.
 */
import { RiUploadLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { FileUpload } from "@/components/base/file-upload/file-upload";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { type TaggedList, uploadContactList } from "@/lib/api/audiences";

import { listSummary } from "./tagging";

const maxBytes = 5 * 1024 * 1024;

export function ListUploadButton({ workspace }: { workspace: string }) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [tag, setTag] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const upload = useMutation({
    mutationFn: () => uploadContactList(workspace, tag.trim(), file as File),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace] }),
  });
  const done: TaggedList | undefined = upload.data;

  const close = () => {
    setOpen(false);
    setTag("");
    setFile(null);
    upload.reset();
  };

  return (
    <>
      <Button variant="secondary" size="small" leadingIcon={RiUploadLine} onClick={() => setOpen(true)}>
        Upload list
      </Button>
      <Modal isOpen={open} onOpenChange={(next) => (next ? setOpen(true) : close())}>
        <ModalHeader
          title="Upload a list"
          description="A CSV with an email, external_id or user_id column. Everyone it names gets the tag."
        />
        <ModalBody className="flex flex-col gap-4">
          <Input label="Tag" placeholder="Event 2026" value={tag} onChange={setTag} maxLength={50} />
          {file === null ? (
            <FileUpload allowedExtensions={["csv"]} maxBytes={maxBytes} onUploadComplete={setFile} />
          ) : (
            <p className="truncate text-body-2-medium text-text-secondary">{file.name}</p>
          )}
          {done ? (
            <p role="status" className="text-body-2-regular text-text-primary">
              {listSummary(done)}
            </p>
          ) : null}
          {upload.error ? (
            <p role="alert" className="text-body-2-regular text-text-error-primary">
              {upload.error.message}
            </p>
          ) : null}
        </ModalBody>
        <ModalFooter className="mt-6">
          <Button variant="secondary" onClick={close}>
            {done ? "Done" : "Cancel"}
          </Button>
          {done ? null : (
            <Button
              disabled={file === null || tag.trim() === "" || upload.isPending}
              loading={upload.isPending}
              onClick={() => upload.mutate()}
            >
              Tag customers
            </Button>
          )}
        </ModalFooter>
      </Modal>
    </>
  );
}
