/*
 * Copying a secret the person must carry somewhere else, such as a setup
 * key or recovery codes. The button says it worked for a moment, then
 * goes back to offering the copy. A browser that refuses the clipboard
 * leaves the button as it was: the text is still on screen to select.
 */
import { useEffect, useState } from "react";

const copiedFor = 2000;

export function useCopy(): { copied: boolean; copy: (text: string) => Promise<void> } {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) {
      return;
    }

    const timer = setTimeout(() => setCopied(false), copiedFor);

    return () => clearTimeout(timer);
  }, [copied]);

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  return { copied, copy };
}
