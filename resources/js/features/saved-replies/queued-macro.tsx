/*
 * The macro waiting beside the draft: which saved reply it came from and
 * what it will do once the message is sent, with a way to drop it and
 * send the text alone.
 */
import { RiCloseLine, RiFlashlightLine } from "@remixicon/react";

import { macroSummary } from "./macro";
import type { Macro } from "./use-macro";
import { useNames } from "./use-names";

export function QueuedMacro({ workspace, macro }: { workspace: string; macro: Macro }) {
  const names = useNames(workspace, macro.queued !== null);
  if (macro.failure) {
    return (
      <p role="alert" className="px-1.5 text-body-2-regular text-text-error-primary">
        {macro.failure}
      </p>
    );
  }
  if (!macro.queued) {
    return null;
  }

  return (
    <div
      role="status"
      aria-label="Queued macro"
      className="mx-1.5 flex items-center gap-2 rounded-xl bg-background-secondary-default py-1 ps-2.5 pe-1"
    >
      <RiFlashlightLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="min-w-0 flex-1 truncate text-body-2-medium text-text-secondary">
        <span className="text-text-primary">{macro.queued.title}</span>
        <span className="ms-1.5">On send: {macroSummary(macro.queued.actions, names).join(" · ")}</span>
      </span>
      <button
        type="button"
        aria-label="Remove macro actions"
        onClick={macro.clear}
        className="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiCloseLine aria-hidden className="size-4" />
      </button>
    </div>
  );
}
