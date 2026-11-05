/*
 * Choosing a picture opens the system's file picker straight away, with
 * only the kinds the server accepts offered. The server decides what it
 * keeps; a file over the limit is turned away here first so nobody waits
 * on an upload that cannot succeed.
 */
import { type ReactNode, useRef } from "react";

import { Button } from "@/components/base/buttons/button";

const accepted = "image/png,image/jpeg,image/webp,image/gif";
const maxBytes = 2 * 1024 * 1024;

export function PicturePicker({
  onPick,
  onTooLarge,
  isPending = false,
  children,
}: {
  onPick: (file: File) => void;
  onTooLarge: () => void;
  isPending?: boolean;
  children: ReactNode;
}) {
  const input = useRef<HTMLInputElement>(null);

  return (
    <>
      <input
        ref={input}
        type="file"
        accept={accepted}
        hidden
        onChange={(event) => {
          const file = event.currentTarget.files?.[0];
          event.currentTarget.value = "";
          if (!file) {
            return;
          }
          if (file.size > maxBytes) {
            onTooLarge();
            return;
          }
          onPick(file);
        }}
      />
      <Button variant="secondary" size="small" loading={isPending} onClick={() => input.current?.click()}>
        {children}
      </Button>
    </>
  );
}
