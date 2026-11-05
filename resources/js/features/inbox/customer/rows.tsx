/*
 * The rows every panel section is made of: a label beside a value that
 * truncates when long and can be copied whole, and the quiet line a
 * section shows when it has nothing to say.
 */
import { RiFileCopyLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";

export function Row({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="grid min-h-9 grid-cols-[96px_minmax(0,1fr)] items-center gap-2">
      <dt className="flex min-w-0 items-center gap-1 text-body-2-regular text-text-secondary">{label}</dt>
      <dd className="flex min-w-0 items-center justify-end gap-1 text-body-2-medium text-text-primary">{children}</dd>
    </div>
  );
}

export function Nothing({ children }: { children: ReactNode }) {
  return <p className="text-body-2-regular text-text-tertiary">{children}</p>;
}

/*
 * Copyable shows a value cut to the row's width, with a button that puts
 * the whole value on the clipboard. Values such as addresses and ids keep
 * their own left-to-right order in any layout direction.
 */
export function Copyable({ value, label }: { value: string; label: string }) {
  return (
    <>
      <span dir="auto" className="min-w-0 truncate" title={value}>
        {value}
      </span>
      <IconButton
        size="small"
        icon={RiFileCopyLine}
        aria-label={`Copy ${label}`}
        onClick={() => {
          void navigator.clipboard?.writeText(value);
        }}
      />
    </>
  );
}
