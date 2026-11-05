/*
 * The workspace's messengers, as far as installing one and placing it
 * needs: its id, which the snippet on a site names, its name and the
 * inbox its conversations land in. A new messenger and a move to
 * another inbox go through here too.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface WidgetSummary {
  id: string;
  name: string;
  enabled: boolean;
  inbox: { id: string; name: string };
}

export function widgetSettingsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "widget-settings"],
    queryFn: () => request<WidgetSummary[]>("GET", "/widget-settings", undefined, { workspace }),
  });
}

/*
 * createWidget adds a messenger whose conversations land in the inbox.
 */
export function createWidget(workspace: string, name: string, inbox: string): Promise<WidgetSummary> {
  return request<WidgetSummary>("POST", "/widget-settings", { name, inbox_id: inbox }, { workspace });
}

/*
 * moveWidget sends the messenger's new conversations to another inbox.
 */
export function moveWidget(workspace: string, widget: string, inbox: string): Promise<WidgetSummary> {
  return request<WidgetSummary>(
    "PATCH",
    `/widget-settings/${encodeURIComponent(widget)}`,
    { inbox_id: inbox },
    { workspace },
  );
}

/*
 * installSnippet is the script tag that loads the messenger on a site.
 */
export function installSnippet(origin: string, widget: string): string {
  return `<script async src="${origin}/widget/v1/loader.js" data-app-id="${widget}"></script>`;
}
