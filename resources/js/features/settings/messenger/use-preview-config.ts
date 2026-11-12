/*
 * The config the preview shows: the messenger's settings with the draft
 * applied, as the server would serve them once saved. Typing waits a
 * moment before asking, and the last good config stays on screen while
 * the next one loads, so the preview never blanks. A change the server
 * would refuse comes back as the error, for the page to point at. A
 * language given shows the messenger in it.
 */
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { previewWidget, type WidgetChange } from "@/lib/api/widget-settings";

/*
 * pause is how long typing must rest before the preview asks again.
 */
const pause = 150;

function useSettled<T>(value: T): T {
  const [settled, setSettled] = useState(value);
  useEffect(() => {
    const timer = window.setTimeout(() => setSettled(value), pause);
    return () => window.clearTimeout(timer);
  }, [value]);

  return settled;
}

export function usePreviewConfig(workspace: string, widget: string, draft: WidgetChange, locale?: string) {
  const settled = useSettled(draft);
  const body = JSON.stringify(settled);

  return useQuery({
    queryKey: ["workspace", workspace, "widget-settings", widget, "preview", locale ?? "", body],
    queryFn: ({ signal }) => previewWidget(workspace, widget, settled, signal, locale),
    placeholderData: keepPreviousData,
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
  });
}
