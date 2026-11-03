/*
 * Files added to a reply or note, on BoardUI's composer attachment
 * design: a strip of 56px tiles above the text, the thumbnail for images
 * and a file icon with the name for everything else. Files upload one
 * after another; while one does, an accent ring draws around its tile
 * and a small percentage counts up, then the percentage blurs out and the
 * dismiss blurs in on the same spot. Tiles scale in as they arrive and
 * out as they go, while the rest close the gap. Uploading is simulated
 * until the inbox stores files, so the strip already behaves as it will.
 */
import {
  RiArticleLine,
  RiChat3Line,
  RiCloseLine,
  RiFileCodeLine,
  RiFileExcel2Line,
  RiFileLine,
  RiFilePpt2Line,
  RiFileTextLine,
  RiFileVideoLine,
  RiGitPullRequestLine,
  RiHistoryLine,
  RiLightbulbLine,
} from "@remixicon/react";

import { AnimatePresence } from "motion/react";

import { cx } from "@/utils/cx";

import { Pop } from "./motion";
import type { Attachment } from "./use-attachments";

const icons = {
  image: RiFileLine,
  document: RiFileTextLine,
  spreadsheet: RiFileExcel2Line,
  presentation: RiFilePpt2Line,
  code: RiFileCodeLine,
  video: RiFileVideoLine,
  file: RiFileLine,
  article: RiArticleLine,
  conversation: RiChat3Line,
  issue: RiGitPullRequestLine,
  request: RiLightbulbLine,
  changelog: RiHistoryLine,
} as const;

function Ring({ progress }: { progress: number | undefined }) {
  return (
    <svg
      aria-hidden
      viewBox="0 0 56 56"
      className={cx(
        "pointer-events-none absolute -inset-px size-14 transition-opacity duration-300 ease-out",
        progress === undefined ? "opacity-0" : "opacity-100",
      )}
    >
      <rect
        x="1"
        y="1"
        width="54"
        height="54"
        rx="11"
        pathLength={100}
        fill="none"
        strokeWidth="2"
        strokeDasharray={`${progress ?? 100} 100`}
        className="stroke-accent-500 transition-[stroke-dasharray] duration-200 ease-linear"
      />
    </svg>
  );
}

function Tile({ file, onRemove }: { file: Attachment; onRemove: () => void }) {
  const Icon = icons[file.kind];
  const landed = file.progress === undefined;
  const image = file.kind === "image" && file.src;

  return (
    <Pop className="relative size-14 shrink-0 rounded-xl border border-composer-panel-tile-border">
      {image ? (
        <img src={file.src} alt={file.name} className="size-full rounded-[11px] object-cover" />
      ) : (
        <span className="flex size-full flex-col items-center justify-center gap-0.5 px-1">
          <Icon aria-hidden className="size-6 text-foreground-icon-secondary" />
          <span className="w-full truncate text-center text-[9px] leading-3 text-text-secondary">{file.name}</span>
        </span>
      )}
      <Ring progress={file.progress} />
      <span
        aria-hidden
        className={cx(
          "pointer-events-none absolute end-[3px] top-[3px] text-[9px] leading-4 font-medium tabular-nums transition-[opacity,filter] duration-300 ease-out",
          image ? "text-white" : "text-text-secondary",
          landed ? "opacity-0 blur-[3px]" : "opacity-100 blur-0",
        )}
      >
        {file.progress ?? 100}%
      </span>
      <button
        type="button"
        aria-label={`Remove ${file.name}`}
        onClick={onRemove}
        disabled={!landed}
        className={cx(
          "absolute end-[3px] top-[3px] flex size-4 cursor-pointer items-center justify-center rounded-full outline-none",
          "transition-[opacity,filter] duration-300 ease-out focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          image ? "bg-white/50 text-white backdrop-blur-[2px]" : "bg-background-secondary-default text-text-secondary",
          landed ? "opacity-100 blur-0" : "pointer-events-none opacity-0 blur-[3px]",
        )}
      >
        <RiCloseLine aria-hidden className="size-3" />
      </button>
    </Pop>
  );
}

export function AttachmentStrip({ files, onRemove }: { files: Attachment[]; onRemove: (id: string) => void }) {
  return (
    <ul aria-label="Attachments" className={cx("flex flex-wrap gap-2", files.length === 0 && "hidden")}>
      <AnimatePresence initial={false}>
        {files.map((file) => (
          <Tile key={file.id} file={file} onRemove={() => onRemove(file.id)} />
        ))}
      </AnimatePresence>
    </ul>
  );
}
