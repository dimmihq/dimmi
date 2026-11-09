/*
 * The row that asks for a link's address, in every formatting editor.
 */
import { useState } from "react";

import { Input } from "@/components/base/input/input";

/*
 * LinkRow asks for the address of a link; Enter applies it, Escape leaves
 * the text as it was, and an empty address removes the link.
 */
export function LinkRow({
  initial,
  onApply,
  onCancel,
}: {
  initial: string;
  onApply: (href: string) => void;
  onCancel: () => void;
}) {
  const [href, setHref] = useState(initial);

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        onApply(href);
      }}
    >
      <Input
        size="small"
        aria-label="Link address"
        inputDir="ltr"
        autoFocus
        value={href}
        onChange={setHref}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            onCancel();
          }
        }}
      />
    </form>
  );
}
