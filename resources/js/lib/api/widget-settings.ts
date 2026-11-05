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

/*
 * WidgetAppearance is a messenger's look as its settings store it.
 */
export interface WidgetAppearance {
  primary_color: string;
  header_color: string | null;
  button_color: string | null;
  background: {
    type: "solid" | "gradient" | "image";
    gradient?: { from: string; to: string; angle: number };
    fade: boolean;
  };
  theme: "auto" | "light" | "dark";
  position: "right" | "left";
  padding_x: number;
  padding_y: number;
  font: "system" | "inherit";
  sound: boolean;
  launcher: { style: "bubble" | "tab" | "hidden"; shape: "circle" | "squircle"; label: string; icon: string };
}

/*
 * WidgetSettings is one messenger as its settings page edits it. Texts
 * holds only the words someone changed from the defaults.
 */
export interface WidgetSettings extends WidgetSummary {
  appearance: WidgetAppearance;
  texts: Record<string, string>;
  locale: string;
  reply_time: "auto" | "minutes" | "hours" | "day";
  show_team_avatars: boolean;
  show_availability: boolean;
  allowed_origins: string[];
  allow_anonymous: boolean;
  collect_email: "never" | "offline" | "always";
  require_email_when_offline: boolean;
  attachments_enabled: boolean;
  max_attachment_mb: number;
  native_apps_enabled: boolean;
  help_tab_enabled: boolean;
  changelog_tab_enabled: boolean;
  intercom_shim: boolean;
  show_branding: boolean | null;
  last_boot_at: string | null;
  last_boot_origin: string | null;
}

/*
 * WidgetChange is what a save sends: any of the settings, the look and
 * the words merged over what the messenger has.
 */
export type WidgetChange = Partial<
  Omit<WidgetSettings, "id" | "inbox" | "appearance" | "last_boot_at" | "last_boot_origin">
> & {
  appearance?: Partial<Omit<WidgetAppearance, "launcher" | "background">> & {
    launcher?: Partial<WidgetAppearance["launcher"]>;
    background?: Partial<WidgetAppearance["background"]>;
  };
};

export function widgetQuery(workspace: string, widget: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "widget-settings", widget],
    queryFn: () =>
      request<WidgetSettings>("GET", `/widget-settings/${encodeURIComponent(widget)}`, undefined, { workspace }),
  });
}

/*
 * updateWidget saves a change to the messenger.
 */
export function updateWidget(workspace: string, widget: string, change: WidgetChange): Promise<WidgetSettings> {
  return request<WidgetSettings>("PATCH", `/widget-settings/${encodeURIComponent(widget)}`, change, { workspace });
}

/*
 * previewWidget returns the config the messenger would serve with the
 * change saved, without saving it.
 */
export function previewWidget(
  workspace: string,
  widget: string,
  change: WidgetChange,
  signal?: AbortSignal,
): Promise<Record<string, unknown>> {
  return request<Record<string, unknown>>("POST", `/widget-settings/${encodeURIComponent(widget)}/preview`, change, {
    workspace,
    signal,
  });
}

/*
 * deleteWidget removes the messenger; sites using its snippet stop
 * showing it.
 */
export function deleteWidget(workspace: string, widget: string): Promise<void> {
  return request<void>("DELETE", `/widget-settings/${encodeURIComponent(widget)}`, undefined, { workspace });
}
