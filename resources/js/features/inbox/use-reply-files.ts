/*
 * Files a teammate adds to a reply. Each one uploads as soon as it is
 * picked, pasted or dropped, so the reply goes out the moment it is
 * sent; one that fails keeps why, and can be taken off.
 */
import { useEffect, useRef, useState } from "react";

import { type Attachment, uploadAttachment } from "@/lib/api/inbox";

export interface Pending {
  key: string;
  file: File;
  preview: string | null;
  attachment?: Attachment;
  failure?: string;
}

/*
 * useReplyFiles keeps the files of one reply: add uploads them, remove
 * drops one, and clear forgets them all once the reply is sent.
 */
export function useReplyFiles(workspace: string) {
  const [files, setFiles] = useState<Pending[]>([]);
  const previews = useRef<string[]>([]);

  useEffect(
    () => () => {
      for (const url of previews.current) {
        URL.revokeObjectURL(url);
      }
    },
    [],
  );

  const update = (key: string, change: Partial<Pending>) =>
    setFiles((current) => current.map((f) => (f.key === key ? { ...f, ...change } : f)));

  const add = (picked: FileList | File[], allowed = true) => {
    if (!allowed) {
      return;
    }
    for (const file of Array.from(picked)) {
      const key = crypto.randomUUID();
      const preview = file.type.startsWith("image/") ? URL.createObjectURL(file) : null;
      if (preview) {
        previews.current.push(preview);
      }
      setFiles((current) => [...current, { key, file, preview }]);
      uploadAttachment(workspace, file)
        .then((attachment) => update(key, { attachment }))
        .catch((error: unknown) =>
          update(key, { failure: error instanceof Error ? error.message : "The file could not be uploaded." }),
        );
    }
  };

  return {
    files,
    add,
    remove: (key: string) => setFiles((current) => current.filter((f) => f.key !== key)),
    clear: () => setFiles([]),
    ids: files.flatMap((f) => (f.attachment ? [f.attachment.id] : [])),
    uploading: files.some((f) => !f.attachment && !f.failure),
    failed: files.some((f) => f.failure),
  };
}

export type ReplyFiles = ReturnType<typeof useReplyFiles>;
