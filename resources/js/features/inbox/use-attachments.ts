/*
 * The files a composer holds, uploaded one after another. Uploading is
 * simulated until the inbox stores files: each file's progress climbs to
 * 100 over about a second, then the next one starts. Image previews are
 * local addresses, freed when their file is removed or the composer goes.
 * Things linked from the workspace (an article, an issue) join the same
 * strip already landed, and only once each.
 */
import { useEffect, useRef, useState } from "react";

export interface Attachment {
  id: string;
  name: string;
  kind:
    | "image"
    | "document"
    | "spreadsheet"
    | "presentation"
    | "code"
    | "video"
    | "file"
    | "article"
    | "conversation"
    | "issue"
    | "request"
    | "changelog";
  /* A local preview address for images. */
  src?: string;
  /* 0 to 100 while uploading; absent once the file has landed. */
  progress?: number;
}

const extensions: [RegExp, Attachment["kind"]][] = [
  [/\.(docx?|pdf|txt|md|rtf|pages)$/i, "document"],
  [/\.(xlsx?|csv|numbers)$/i, "spreadsheet"],
  [/\.(pptx?|key)$/i, "presentation"],
  [/\.(tsx?|jsx?|go|py|rb|json|ya?ml|html|css|sh)$/i, "code"],
];

function kindOf(file: File): Attachment["kind"] {
  if (file.type.startsWith("image/")) {
    return "image";
  }
  if (file.type.startsWith("video/")) {
    return "video";
  }

  return extensions.find(([pattern]) => pattern.test(file.name))?.[1] ?? "file";
}

/*
 * useAttachments holds a composer's files and uploads them in turn.
 */
export function useAttachments() {
  const [files, setFiles] = useState<Attachment[]>([]);
  const counter = useRef(0);
  const uploading = files.some((file) => file.progress !== undefined);

  useEffect(() => {
    if (!uploading) {
      return;
    }
    const tick = window.setInterval(() => {
      setFiles((before) => {
        const index = before.findIndex((file) => file.progress !== undefined);
        const current = before[index];
        if (!current || current.progress === undefined) {
          return before;
        }
        const progress = current.progress + 4;
        const next = [...before];
        next[index] = progress >= 100 ? { ...current, progress: undefined } : { ...current, progress };

        return next;
      });
    }, 44);

    return () => window.clearInterval(tick);
  }, [uploading]);

  const latest = useRef(files);
  useEffect(() => {
    latest.current = files;
  }, [files]);
  useEffect(() => () => latest.current.forEach(release), []);

  const add = (picked: Iterable<File>) => {
    const added = [...picked].map((file): Attachment => {
      counter.current += 1;
      const kind = kindOf(file);

      return {
        id: `file-${counter.current}`,
        name: file.name,
        kind,
        src: kind === "image" ? URL.createObjectURL(file) : undefined,
        progress: 0,
      };
    });
    setFiles((before) => [...before, ...added]);
  };

  const link = (item: { id: string; kind: Attachment["kind"]; title: string }) =>
    setFiles((before) =>
      before.some((file) => file.id === item.id)
        ? before
        : [...before, { id: item.id, name: item.title, kind: item.kind }],
    );

  const remove = (id: string) => {
    const gone = files.find((file) => file.id === id);
    if (gone) {
      release(gone);
    }
    setFiles((before) => before.filter((file) => file.id !== id));
  };

  return { files, add, link, remove };
}

/*
 * release frees the memory behind an image preview once its tile goes,
 * or when the composer itself goes.
 */
function release(file: Attachment) {
  if (file.src) {
    URL.revokeObjectURL(file.src);
  }
}
