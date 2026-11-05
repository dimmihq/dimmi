/*
 * DraftConflict tells the person their draft changed on another device
 * while they were typing here, and lets them take that version or keep
 * their own, which then wins.
 */
import { RiDeviceLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";

import type { DraftState } from "./use-draft";

export function DraftConflict({ draft }: { draft: DraftState }) {
  if (!draft.conflict) {
    return null;
  }

  return (
    <div
      role="status"
      aria-label="Draft conflict"
      className="flex items-center gap-2 rounded-xl bg-background-secondary-default px-3 py-2"
    >
      <RiDeviceLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="min-w-0 flex-1 text-body-2-regular text-text-secondary">Newer draft on another device</span>
      <Button size="xs" variant="secondary" onClick={draft.takeTheirs}>
        Use
      </Button>
      <Button size="xs" variant="ghost" onClick={draft.keepMine}>
        Keep
      </Button>
    </div>
  );
}
