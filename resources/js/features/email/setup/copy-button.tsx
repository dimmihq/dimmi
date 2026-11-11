import { RiCheckLine, RiFileCopyLine } from "@remixicon/react";
import { useState } from "react";
import { Focusable } from "react-aria-components";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";

/*
 * useCopy copies a value and remembers for a moment that it did.
 */
function useCopy(value: string) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    void navigator.clipboard?.writeText(value).catch(() => undefined);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  };

  return [copied, copy] as const;
}

/*
 * CopyIconButton is the compact copy control for rows of values, named
 * by label and explained by a tooltip.
 */
export function CopyIconButton({ value, label }: { value: string; label: string }) {
  const [copied, copy] = useCopy(value);
  const tip = copied ? "Copied" : label;

  return (
    <TooltipTrigger delay={200}>
      <Focusable>
        <IconButton size="small" icon={copied ? RiCheckLine : RiFileCopyLine} aria-label={label} onClick={copy} />
      </Focusable>
      <Tooltip size="md">{tip}</Tooltip>
    </TooltipTrigger>
  );
}

/*
 * CopyButton copies a value and shows a check for a moment.
 */
export function CopyButton({ value, label }: { value: string; label: string }) {
  const [copied, copy] = useCopy(value);

  return (
    <Button
      variant="secondary"
      size="small"
      aria-label={label}
      leadingIcon={copied ? RiCheckLine : RiFileCopyLine}
      onClick={copy}
    >
      Copy
    </Button>
  );
}
