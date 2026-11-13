/*
 * The real messenger in a frame, in preview mode with sample content and
 * no server, so it keeps its styles and focus to itself and opens as the
 * page opens. Each command given runs on it once it loaded, and again
 * when it changes: a config to show, as the messenger settings send, or
 * request types and a request, as the ticket settings send.
 */
import { useEffect, useRef, useState } from "react";

import { cx } from "@/utils/cx";

type Messenger = (...command: unknown[]) => void;

/*
 * page is the frame's document: nothing but the messenger's loader in
 * preview mode. It shares the dashboard's origin, so the dashboard can
 * talk to the messenger through the frame's window. The frame and its
 * page both use the light colour scheme, since a frame whose scheme
 * differs from its page paints an opaque background over the column; the
 * messenger still follows its own theme setting.
 */
const page = `<!doctype html><html><head><meta charset="utf-8"></head><body style="margin:0">
<script src="/widget/v1/loader.js" data-preview data-latency="250" data-app-id="preview"></script>
</body></html>`;

function messenger(frame: HTMLIFrameElement | null): Messenger | undefined {
  return (frame?.contentWindow as (Window & { Dimmi?: Messenger }) | null | undefined)?.Dimmi;
}

export function MessengerFrame({
  title,
  commands,
  className,
}: {
  title: string;
  commands: [string, unknown][];
  className?: string;
}) {
  const frame = useRef<HTMLIFrameElement>(null);
  const [ready, setReady] = useState(false);
  const sent = JSON.stringify(commands);

  // biome-ignore lint/correctness/useExhaustiveDependencies: sent stands for the commands
  useEffect(() => {
    if (!ready) {
      return;
    }
    const dimmi = messenger(frame.current);
    for (const [name, argument] of commands) {
      if (argument !== undefined) {
        dimmi?.(name, argument);
      }
    }
  }, [ready, sent]);

  return (
    <iframe
      ref={frame}
      title={title}
      srcDoc={page}
      onLoad={() => {
        messenger(frame.current)?.("open");
        setReady(true);
      }}
      className={cx("size-full border-0 [color-scheme:light]", className)}
    />
  );
}
