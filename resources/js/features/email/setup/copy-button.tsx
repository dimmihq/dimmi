import { RiCheckLine, RiFileCopyLine } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";

/*
 * CopyButton copies a value and shows a check for a moment.
 */
export function CopyButton({ value, label }: { value: string; label: string }) {
  const [copied, setCopied] = useState(false);

  return (
    <Button
      variant="secondary"
      size="small"
      aria-label={label}
      leadingIcon={copied ? RiCheckLine : RiFileCopyLine}
      onClick={() => {
        void navigator.clipboard?.writeText(value).catch(() => undefined);
        setCopied(true);
        window.setTimeout(() => setCopied(false), 2000);
      }}
    >
      Copy
    </Button>
  );
}
