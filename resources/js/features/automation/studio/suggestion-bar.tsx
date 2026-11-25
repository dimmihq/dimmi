import { RiSparkling2Line } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";

/*
 * SuggestionBar sits on a workflow's or series' canvas while Operator's
 * suggestion is under review: what it adds, changes and removes, and the
 * buttons that take it into the working copy or turn it down. The canvas
 * shows the suggested copy, with each added or changed step marked.
 */
export function SuggestionBar({
  added,
  changed,
  removed,
  deciding,
  onAccept,
  onDecline,
}: {
  added: number;
  changed: number;
  removed: number;
  deciding: boolean;
  onAccept: () => void;
  onDecline: () => void;
}) {
  const parts = [
    added ? `${added} added` : "",
    changed ? `${changed} changed` : "",
    removed ? `${removed} removed` : "",
  ].filter(Boolean);

  return (
    <div className="mx-3 mt-2 flex shrink-0 flex-wrap items-center gap-3 rounded-xl bg-background-secondary-default px-3 py-2">
      <RiSparkling2Line aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="min-w-0 flex-1 text-body-2-regular text-text-primary">
        Operator suggests changes{parts.length > 0 ? `: ${parts.join(", ")}` : ""}.
      </span>
      <Button size="small" variant="secondary" disabled={deciding} onClick={onDecline}>
        Decline
      </Button>
      <Button size="small" loading={deciding} onClick={onAccept}>
        Accept
      </Button>
    </div>
  );
}
