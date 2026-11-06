/*
 * The preview messenger in a column of its own beside the settings. It
 * runs in a frame, in preview mode with sample conversations and no
 * server, so it floats in its column the way it floats on a site, keeps
 * its styles and focus to itself, and opens as the page opens. The column
 * is wider than the messenger's phone layout, so it shows as on a
 * desktop. Each config it is given shows on it at once.
 */
import { useEffect, useRef, useState } from "react";

import { Panel } from "@/features/shell/page";

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

export function PreviewPane({ config }: { config: Record<string, unknown> | undefined }) {
  const frame = useRef<HTMLIFrameElement>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (ready && config) {
      messenger(frame.current)?.("configure", config);
    }
  }, [ready, config]);

  return (
    <Panel label="Preview" className="hidden h-full w-[500px] shrink-0 xl:flex">
      <iframe
        ref={frame}
        title="Messenger preview"
        srcDoc={page}
        onLoad={() => {
          messenger(frame.current)?.("open");
          setReady(true);
        }}
        className="size-full border-0 [color-scheme:light]"
      />
    </Panel>
  );
}
