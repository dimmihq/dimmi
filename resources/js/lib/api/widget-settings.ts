/*
 * The workspace's messengers, as far as installing one needs: its id,
 * which the snippet on a site names, and its name.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface WidgetSummary {
  id: string;
  name: string;
  enabled: boolean;
}

export function widgetSettingsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "widget-settings"],
    queryFn: () => request<WidgetSummary[]>("GET", "/widget-settings", undefined, { workspace }),
  });
}

/*
 * installSnippet is the script tag that loads the messenger on a site.
 */
export function installSnippet(origin: string, widget: string): string {
  return `<script async src="${origin}/widget/v1/loader.js" data-app-id="${widget}"></script>`;
}
