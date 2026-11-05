/*
 * The preview messenger, floating on the settings page the way it floats
 * on a customer's site. It runs in preview mode, with sample
 * conversations and no server, and shows the settings as the draft
 * leaves them. The messenger loads once per page; leaving the settings
 * hides it and coming back shows it again, since its script runs only
 * once.
 */
import { useEffect } from "react";

type Queue = ((...command: unknown[]) => void) & { q?: unknown[][] };

interface PreviewWindow extends Window {
  Dimmi?: Queue;
}

const hostId = "dimmi-messenger";

/*
 * load adds the messenger's script in preview mode, once, with the queue
 * the messenger's loader takes over, so commands sent before it arrives
 * run once it does.
 */
function load(win: PreviewWindow, doc: Document) {
  if (doc.querySelector("script[data-dimmi-preview]")) {
    return;
  }
  if (!win.Dimmi) {
    const queue: Queue = (...command) => {
      queue.q = [...(queue.q ?? []), command];
    };
    win.Dimmi = queue;
  }
  const script = doc.createElement("script");
  script.src = "/widget/v1/loader.js";
  script.async = true;
  script.dataset.preview = "";
  script.dataset.latency = "250";
  script.dataset.appId = "preview";
  script.dataset.dimmiPreview = "";
  doc.body.append(script);
}

/*
 * showHost shows or hides the messenger once it is on the page.
 */
function showHost(doc: Document, shown: boolean) {
  const host = doc.getElementById(hostId);
  if (host) {
    host.hidden = !shown;
  }
}

/*
 * useFloatingPreview puts the preview messenger on the page while the
 * settings are open and keeps it showing the config it is given.
 */
export function useFloatingPreview(config: Record<string, unknown> | undefined) {
  useEffect(() => {
    load(window as PreviewWindow, document);
    showHost(document, true);
    const observer = new MutationObserver(() => showHost(document, true));
    observer.observe(document.body, { childList: true });

    return () => {
      observer.disconnect();
      showHost(document, false);
    };
  }, []);

  useEffect(() => {
    if (config) {
      (window as PreviewWindow).Dimmi?.("configure", config);
    }
  }, [config]);
}
