/*
 * A block of code in a message. It keeps its own lines and scrolls
 * sideways rather than wrapping, can be copied in one click, and opens
 * full screen for reading a long one. Highlighting comes from the
 * message body, which colours the tokens; this block styles them with
 * the chart colours so they follow the theme.
 */
import { RiCheckLine, RiFileCopyLine, RiFullscreenLine } from "@remixicon/react";
import { type ReactNode, useRef, useState } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";
import { cx } from "@/utils/cx";

const tokens = [
  "[&_.hljs-keyword]:text-chart-5 [&_.hljs-built_in]:text-chart-4 [&_.hljs-type]:text-chart-4",
  "[&_.hljs-string]:text-chart-1 [&_.hljs-number]:text-chart-3 [&_.hljs-literal]:text-chart-3",
  "[&_.hljs-comment]:text-text-tertiary [&_.hljs-title]:text-chart-4 [&_.hljs-attr]:text-chart-3",
].join(" ");

const pre = "overflow-x-auto rounded-xl bg-background-secondary-default p-2 [&_code]:bg-transparent [&_code]:p-0";

export function CodeBlock({ children }: { children?: ReactNode }) {
  const body = useRef<HTMLPreElement>(null);
  const [copied, setCopied] = useState(false);
  const [full, setFull] = useState(false);

  const copy = async () => {
    await navigator.clipboard.writeText(body.current?.textContent ?? "");
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div className="group/code relative">
      <pre ref={body} className={cx(pre, tokens)}>
        {children}
      </pre>
      <div className="absolute end-1 top-1 flex gap-0.5 opacity-0 transition-opacity duration-150 focus-within:opacity-100 group-hover/code:opacity-100">
        <IconButton
          size="small"
          icon={copied ? RiCheckLine : RiFileCopyLine}
          aria-label={copied ? "Copied" : "Copy code"}
          onClick={copy}
        />
        <IconButton
          size="small"
          icon={RiFullscreenLine}
          aria-label="Show code full screen"
          onClick={() => setFull(true)}
        />
      </div>
      <Modal isOpen={full} onOpenChange={setFull} size="xl" aria-label="Code">
        <ModalHeader title="Code" />
        <ModalBody>
          <pre className={cx(pre, tokens, "max-h-[70vh] overflow-y-auto text-body-2-regular")}>{children}</pre>
        </ModalBody>
      </Modal>
    </div>
  );
}
