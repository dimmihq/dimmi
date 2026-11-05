/*
 * A value the owner carries to their identity provider or DNS, shown in
 * full with a button that copies it.
 */
import { RiFileCopyLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";

import { useCopy } from "../use-copy";

export function CopyField({ label, value }: { label: string; value: string }) {
  const { copied, copy } = useCopy();

  return (
    <div className="flex items-end gap-2">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-body-2-medium text-text-secondary">{label}</span>
        <code className="break-all text-body-2-regular text-text-primary">{value}</code>
      </div>
      <Button variant="secondary" size="xs" leadingIcon={RiFileCopyLine} onClick={() => void copy(value)}>
        {copied ? "Copied" : "Copy"}
      </Button>
    </div>
  );
}
