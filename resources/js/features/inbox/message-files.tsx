/*
 * The files on a message in the thread. Pictures show themselves at
 * their stored proportions and open full size; voice notes and other
 * audio play in place; other files are a row with their name and size
 * that downloads them. A file the malware scan
 * refused is red, says a virus was found and never links; one still being
 * checked waits without a link. A file nothing checked, because it was too
 * big or the scanner was away, is amber and asks for a second thought
 * before it opens. Rows and players take the surface of the bubbles on
 * their side, so they stand out from the thread the same way.
 */
import { RiDownloadLine, RiErrorWarningLine, RiFileLine, RiShieldCrossLine, RiTimeLine } from "@remixicon/react";
import { type ReactNode, useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
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

/*
 * unchecked reports a file with a link that nothing scanned.
 */
function unchecked(file: Attachment): boolean {
  return Boolean(file.url) && !file.blocked && file.scanned === false;
}

function openFile(url: string) {
  window.open(url, "_blank", "noopener,noreferrer");
}

/*
 * CarefulOpen opens a file nothing scanned only after the teammate has
 * been told to take care.
 */
function CarefulOpen({
  file,
  className,
  label,
  children,
}: {
  file: Attachment & { url: string };
  className: string;
  label: string;
  children: ReactNode;
}) {
  const [asking, setAsking] = useState(false);

  return (
    <>
      <button type="button" aria-label={label} className={cx("text-start", className)} onClick={() => setAsking(true)}>
        {children}
      </button>
      <AlertDialog
        isOpen={asking}
        onOpenChange={setAsking}
        variant="danger"
        title="Open a file that was not scanned?"
        description={`${file.filename} was never checked for viruses. Open it only if you trust the sender, and be very careful.`}
        confirmLabel="Open anyway"
        onConfirm={() => openFile(file.url)}
      />
    </>
  );
}

function Picture({ file }: { file: Attachment & { url: string } }) {
  const frame =
    "block overflow-hidden rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring";
  const picture = (
    <>
      <img
        src={file.url}
        alt={file.filename}
        width={file.width ?? undefined}
        height={file.height ?? undefined}
        loading="lazy"
        className="block h-auto max-h-72 w-auto max-w-full object-cover"
      />
      {unchecked(file) ? (
        <span className="flex items-center gap-1 bg-status-yellow-background px-2 py-1 text-caption-1-regular text-status-yellow-text">
          <RiErrorWarningLine aria-hidden className="size-3.5 shrink-0" />
          Not scanned for viruses
        </span>
      ) : null}
    </>
  );

  if (unchecked(file)) {
    return (
      <CarefulOpen file={file} label={`Open ${file.filename}`} className={frame}>
        {picture}
      </CarefulOpen>
    );
  }

  return (
    <a href={file.url} target="_blank" rel="noopener noreferrer" className={frame}>
      {picture}
    </a>
  );
}

/*
 * surfaces are the backgrounds of file rows and players, matching the
 * bubbles: the team's on the raised surface, the customer's on the
 * tinted one.
 */
const saveStyle =
  "flex items-center gap-1.5 px-1 text-caption-1-regular text-text-tertiary outline-none hover:text-text-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring";

const surfaces = {
  ours: { rest: "bg-background-primary-default shadow-card", hover: "hover:bg-background-primary-hover" },
  theirs: { rest: "bg-background-tertiary-default", hover: "hover:bg-background-tertiary-hover" },
};

type Side = keyof typeof surfaces;

/*
 * Voice is an audio file the team can play in place, with a link to save
 * it beside the player.
 */
function Voice({ file, side }: { file: Attachment & { url: string }; side: Side }) {
  const saveLabel = (
    <>
      <RiDownloadLine aria-hidden className="size-3.5 shrink-0" />
      <span className="truncate">
        {file.filename} · {sizeOf(file.size_bytes)}
        {unchecked(file) ? " · not scanned" : ""}
      </span>
    </>
  );

  return (
    <div className={cx("flex w-72 max-w-full flex-col gap-1.5 rounded-xl p-2", surfaces[side].rest)}>
      <audio controls preload="metadata" src={file.url} aria-label={file.filename} className="h-9 w-full">
        <track kind="captions" />
      </audio>
      {unchecked(file) ? (
        <CarefulOpen file={file} label={`Download ${file.filename}`} className={saveStyle}>
          {saveLabel}
        </CarefulOpen>
      ) : (
        <a href={file.url} download={file.filename} className={saveStyle}>
          {saveLabel}
        </a>
      )}
    </div>
  );
}

type State = "blocked" | "waiting" | "careful" | "clear";

function stateOf(file: Attachment): State {
  if (file.blocked) {
    return "blocked";
  }
  if (!file.url) {
    return "waiting";
  }

  return unchecked(file) ? "careful" : "clear";
}

/*
 * looks is how each state of a file draws its row: the icon, the tint of
 * the icon and of the detail line, and the frame.
 */
const looks: Record<State, { icon: typeof RiFileLine; iconTone: string; tone: string; frame: string }> = {
  blocked: {
    icon: RiShieldCrossLine,
    iconTone: "text-foreground-icon-error",
    tone: "text-text-error-primary",
    frame: "border border-border-error-default bg-background-tertiary-error",
  },
  waiting: { icon: RiTimeLine, iconTone: "text-foreground-icon-secondary", tone: "text-text-tertiary", frame: "" },
  careful: {
    icon: RiErrorWarningLine,
    iconTone: "text-status-yellow-text",
    tone: "text-status-yellow-text",
    frame: "border border-status-yellow-text",
  },
  clear: { icon: RiFileLine, iconTone: "text-foreground-icon-secondary", tone: "text-text-tertiary", frame: "" },
};

function detailOf(file: Attachment, state: State): string {
  const size = sizeOf(file.size_bytes);
  const words: Record<State, string> = {
    blocked: "Virus found. This file cannot be opened.",
    waiting: "Being checked…",
    careful: `Not scanned for viruses · ${size}`,
    clear: size,
  };

  return words[state];
}

function FileRow({ file, side }: { file: Attachment; side: Side }) {
  const state = stateOf(file);
  const look = looks[state];
  const Icon = look.icon;
  const row = (
    <>
      <Icon aria-hidden className={cx("size-4 shrink-0", look.iconTone)} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-2-medium text-text-primary">{file.filename}</span>
        <span className={cx("text-caption-1-regular", look.tone)}>{detailOf(file, state)}</span>
      </span>
      {file.url ? <RiDownloadLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" /> : null}
    </>
  );
  const shape = cx(
    "flex w-fit min-w-48 max-w-full items-center gap-2.5 rounded-xl px-3 py-2",
    look.frame || surfaces[side].rest,
  );
  const interactive = cx(
    shape,
    surfaces[side].hover,
    "outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
  );

  if (!file.url) {
    return (
      <div role={state === "blocked" ? "alert" : undefined} className={shape}>
        {row}
      </div>
    );
  }
  if (state === "careful") {
    return (
      <CarefulOpen file={{ ...file, url: file.url }} label={`Download ${file.filename}`} className={interactive}>
        {row}
      </CarefulOpen>
    );
  }

  return (
    <a
      href={file.url}
      download={file.filename}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Download ${file.filename}`}
      className={interactive}
    >
      {row}
    </a>
  );
}

/*
 * FileView shows a file the way that suits it: a picture, a player for
 * audio, or a row to download.
 */
function FileView({ file, side }: { file: Attachment; side: Side }) {
  if (!file.url || file.blocked) {
    return <FileRow file={file} side={side} />;
  }
  if (file.content_type.startsWith("image/")) {
    return <Picture file={{ ...file, url: file.url }} />;
  }
  if (file.content_type.startsWith("audio/")) {
    return <Voice file={{ ...file, url: file.url }} side={side} />;
  }

  return <FileRow file={file} side={side} />;
}

export function MessageFiles({ files, ours }: { files: Attachment[]; ours: boolean }) {
  if (files.length === 0) {
    return null;
  }

  return (
    <ul aria-label="Files" className={cx("mt-1.5 flex flex-col gap-1.5", ours ? "items-end" : "items-start")}>
      {files.map((file) => (
        <li key={file.id} className="max-w-full">
          <FileView file={file} side={ours ? "ours" : "theirs"} />
        </li>
      ))}
    </ul>
  );
}
