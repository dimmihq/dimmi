/*
 * The messenger's icons, drawn on a 24px grid with one stroke weight so
 * they read as a set, and the Dimmi mark on its own 40px grid. Every icon
 * is decorative: the button or row around it carries the label.
 */
import type { Child } from "widget-jsx/jsx-runtime";

function Icon({ className, children }: { className?: string; children: Child }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" className={className}>
      {children}
    </svg>
  );
}

const book =
  "M4 5.5A1.5 1.5 0 0 1 5.5 4H11v16H5.5A1.5 1.5 0 0 1 4 18.5ZM20 5.5A1.5 1.5 0 0 0 18.5 4H13v16h5.5a1.5 1.5 0 0 0 1.5-1.5Z";

export function Mark() {
  return (
    <svg className="mark" viewBox="4 4 40 40" aria-hidden="true">
      <path d="M4 24a20 20 0 0 1 13.7-19L43 30.3A20 20 0 0 1 24 44H11a7 7 0 0 1-7-7Z" />
      <path d="M19.8 4.5a20 20 0 0 1 4.9-.5L44 23.3a20 20 0 0 1-.5 4.9Z" />
      <path d="M27.6 4.3A20 20 0 0 1 35 7.3l5.7 5.7a20 20 0 0 1 3 7.4Z" />
    </svg>
  );
}

export function CloseIcon() {
  return (
    <Icon>
      <path d="m6.5 6.5 11 11m0-11-11 11" />
    </Icon>
  );
}

export function ExpandIcon() {
  return (
    <Icon className="widen">
      <path d="M14 4h6v6M10 20H4v-6M20 4l-6.5 6.5M4 20l6.5-6.5" />
    </Icon>
  );
}

export function CollapseIcon() {
  return (
    <Icon className="narrow">
      <path d="M20 10h-6V4M4 14h6v6M14 10l6.5-6.5M10 14l-6.5 6.5" />
    </Icon>
  );
}

export function ChevronDownIcon() {
  return (
    <Icon className="chevron">
      <path d="m6 9.5 6 6 6-6" />
    </Icon>
  );
}

export function ChevronRightIcon() {
  return (
    <Icon>
      <path d="m10 7 5 5-5 5" />
    </Icon>
  );
}

export function ArrowRightIcon() {
  return (
    <Icon>
      <path d="M4.5 12h13m-5.5-6 6 6-6 6" />
    </Icon>
  );
}

export function ArrowUpIcon() {
  return (
    <Icon>
      <path d="M12 19V5m-6 6 6-6 6 6" />
    </Icon>
  );
}

export function BackIcon() {
  return (
    <Icon>
      <path d="m14 6-6 6 6 6" />
    </Icon>
  );
}

export function ExternalIcon() {
  return (
    <Icon>
      <path d="M14 5h5v5m0-5-8 8M18 14v4a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h4" />
    </Icon>
  );
}

export function SparkleIcon() {
  return (
    <Icon>
      <path d="M12 3.5c.6 3.9 2.6 5.9 6.5 6.5-3.9.6-5.9 2.6-6.5 6.5-.6-3.9-2.6-5.9-6.5-6.5 3.9-.6 5.9-2.6 6.5-6.5ZM18.5 15.5c.3 1.6 1 2.3 2.5 2.5-1.6.3-2.3 1-2.5 2.5-.3-1.6-1-2.3-2.5-2.5 1.6-.3 2.3-1 2.5-2.5Z" />
    </Icon>
  );
}

export function SearchIcon() {
  return (
    <Icon>
      <circle cx="11" cy="11" r="6.5" />
      <path d="m20 20-4.4-4.4" />
    </Icon>
  );
}

export function BookIcon() {
  return (
    <Icon>
      <path d={book} />
    </Icon>
  );
}

export function CalendarIcon() {
  return (
    <Icon>
      <rect x="4" y="5.5" width="16" height="14.5" rx="2.5" />
      <path d="M4 10h16M8.5 3.5v4m7-4v4" />
    </Icon>
  );
}

export function PaperclipIcon() {
  return (
    <Icon>
      <path d="m19 11.5-6.9 6.9a4.5 4.5 0 0 1-6.4-6.4l7.1-7.1a3 3 0 0 1 4.2 4.2l-7 7.1a1.5 1.5 0 0 1-2.1-2.1l6.4-6.4" />
    </Icon>
  );
}

export function MicIcon() {
  return (
    <Icon>
      <rect x="9" y="3.5" width="6" height="11" rx="3" />
      <path d="M5.5 11.5a6.5 6.5 0 0 0 13 0M12 18v2.5" />
    </Icon>
  );
}

export function SmileIcon() {
  return (
    <Icon>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M8.5 14a4 4 0 0 0 7 0M9.5 9.5v.1M14.5 9.5v.1" />
    </Icon>
  );
}

export function FileIcon() {
  return (
    <Icon>
      <path d="M14 3.5H7a1.5 1.5 0 0 0-1.5 1.5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8Z" />
      <path d="M14 3.5V8h4.5M9 13h6m-6 3h4" />
    </Icon>
  );
}

export function ImageIcon() {
  return (
    <Icon>
      <rect x="3.5" y="5" width="17" height="14" rx="2.5" />
      <circle cx="9" cy="10" r="1.5" />
      <path d="m20.5 16-5-5-8 8" />
    </Icon>
  );
}

export function DownloadIcon() {
  return (
    <Icon>
      <path d="M12 4.5v11m-4.5-4.5 4.5 4.5 4.5-4.5M5 19.5h14" />
    </Icon>
  );
}

export function PlayIcon() {
  return (
    <Icon className="play">
      <path d="M8 5.5v13l10.5-6.5Z" />
    </Icon>
  );
}

export function PauseIcon() {
  return (
    <Icon className="pause">
      <path d="M8.5 5.5v13M15.5 5.5v13" />
    </Icon>
  );
}

export function TrashIcon() {
  return (
    <Icon>
      <path d="M5 7h14M10 4h4M7 7l.8 12a1.5 1.5 0 0 0 1.5 1.5h5.4a1.5 1.5 0 0 0 1.5-1.5L17 7" />
    </Icon>
  );
}

export function HomeIcon() {
  return (
    <Icon>
      <path d="M4 10.2 12 4l8 6.2V19a1 1 0 0 1-1 1h-4.5v-5.5h-5V20H5a1 1 0 0 1-1-1Z" />
    </Icon>
  );
}

export function MessagesIcon() {
  return (
    <Icon>
      <path d="M4 6a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2H9l-3.5 3v-3H6a2 2 0 0 1-2-2Z" />
      <path d="M16 9h2a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-.5v3L14 18h-3a2 2 0 0 1-1.7-1" />
    </Icon>
  );
}

export function ChangelogIcon() {
  return (
    <Icon>
      <rect x="7.5" y="4" width="9" height="16" rx="2" />
      <path d="M4 7v10M20 7v10" />
    </Icon>
  );
}

export function HelpIcon() {
  return (
    <Icon>
      <path d={book} />
      <path d="M15.5 8.5h2m-2 3h2" />
    </Icon>
  );
}
