/*
 * A moment in the thread. It reads as how long ago it was, or as the
 * clock time in the customer's zone while the person reads the thread in
 * the customer's time; hovering or focusing it shows the full date.
 */
import { useContext } from "react";
import { Button } from "react-aria-components";

import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { formatDateTime, formatTime } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { ago } from "../conversations";
import { ThreadZone } from "./thread-zone";

export function When({ iso, prefix = "", className }: { iso: string; prefix?: string; className?: string }) {
  const zone = useContext(ThreadZone);
  const text = zone ? formatTime(iso, { timeZone: zone }) : ago(iso);
  const full = formatDateTime(iso, { timeZone: zone ?? undefined });

  return (
    <TooltipTrigger>
      <Button
        aria-label={`${prefix}${full}`}
        className={cx(
          "cursor-default rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          className,
        )}
      >
        <time dateTime={iso}>
          {prefix}
          {text}
        </time>
      </Button>
      <Tooltip>{full}</Tooltip>
    </TooltipTrigger>
  );
}
