/*
 * The files on a message in the thread. Pictures show themselves at
 * their stored proportions and open full size; other files are a row
 * with their name and size that downloads them. A file the malware scan
 * refused says so and never links; one still being checked waits
 * without a link.
 */
import { RiDownloadLine, RiFileLine, RiShieldCrossLine, RiTimeLine } from "@remixicon/react";

import type { Attachment } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

/*
 * sizeOf writes a file size the way people read it.
 */
function sizeOf(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${Math.round(bytes / 1024)} KB`;
  }

  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function Picture({ file }: { file: Attachment & { url: string } }) {
  return (
    <a
      href={file.url}
      target="_blank"
      rel="noopener noreferrer"
      className="block overflow-hidden rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <img
        src={file.url}
        alt={file.filename}
        width={file.width ?? undefined}
        height={file.height ?? undefined}
        loading="lazy"
        className="block h-auto max-h-72 w-auto max-w-full object-cover"
      />
    </a>
  );
}

function FileRow({ file }: { file: Attachment }) {
  const waiting = !file.url && !file.blocked;
  const Icon = file.blocked ? RiShieldCrossLine : waiting ? RiTimeLine : RiFileLine;
  const detail = file.blocked ? "Blocked by the malware scan" : waiting ? "Being checked…" : sizeOf(file.size_bytes);
  const row = (
    <>
      <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-2-medium text-text-primary">{file.filename}</span>
        <span className="text-caption-1-regular text-text-tertiary">{detail}</span>
      </span>
      {file.url ? <RiDownloadLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" /> : null}
    </>
  );
  const shape = "flex w-64 max-w-full items-center gap-2.5 rounded-xl bg-background-secondary-default px-3 py-2";

  if (!file.url) {
    return <div className={cx(shape, file.blocked && "opacity-70")}>{row}</div>;
  }

  return (
    <a
      href={file.url}
      download={file.filename}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Download ${file.filename}`}
      className={cx(
        shape,
        "outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
      )}
    >
      {row}
    </a>
  );
}

export function MessageFiles({ files, ours }: { files: Attachment[]; ours: boolean }) {
  if (files.length === 0) {
    return null;
  }

  return (
    <ul aria-label="Files" className={cx("mt-1.5 flex flex-col gap-1.5", ours ? "items-end" : "items-start")}>
      {files.map((file) => (
        <li key={file.id} className="max-w-full">
          {file.url && file.content_type.startsWith("image/") && !file.blocked ? (
            <Picture file={{ ...file, url: file.url }} />
          ) : (
            <FileRow file={file} />
          )}
        </li>
      ))}
    </ul>
  );
}
