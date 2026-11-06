/*
 * A piece of code to paste into a site, shown in full with a button that
 * copies it and a line saying where it goes. Onboarding and a messenger's
 * settings show the install snippet the same way. The code reads left to
 * right in every language, and the surface it sits on is the caller's,
 * since it follows the card around it.
 */
import { RiCheckLine, RiFileCopyLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import { useCopy } from "./use-copy";

export function Snippet({ code, hint, className }: { code: string; hint: string; className?: string }) {
  const { copied, copy } = useCopy();

  return (
    <div className="flex flex-col gap-3">
      <code
        dir="ltr"
        className={cx(
          "block whitespace-pre-wrap break-all rounded-xl p-3 font-mono text-body-2-regular text-text-primary",
          className,
        )}
      >
        {code}
      </code>
      <div className="flex items-center gap-3">
        <Button
          variant="secondary"
          size="small"
          leadingIcon={copied ? RiCheckLine : RiFileCopyLine}
          onClick={() => void copy(code)}
        >
          {copied ? "Copied" : "Copy snippet"}
        </Button>
        <span className="text-body-2-regular text-text-tertiary">{hint}</span>
      </div>
    </div>
  );
}
