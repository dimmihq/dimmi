/*
 * A customer type as customers meet it, in the real messenger with the
 * look of the messenger its customers use: the type's form with only the fields
 * customers see, or Messages with a request of this type in the state
 * picked in the type's states, under the label customers read. It
 * follows the editor's draft as it changes. Nothing typed or sent in it
 * reaches anyone.
 */
import { useQuery } from "@tanstack/react-query";

import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { MessengerFrame } from "@/features/settings/messenger/messenger-frame";
import type { TicketField, TicketState } from "@/lib/api/tickets";
import { previewWidget, widgetSettingsQuery } from "@/lib/api/widget-settings";

export type PreviewView = "form" | "messages";

const views: { id: PreviewView; label: string }[] = [
  { id: "form", label: "Form" },
  { id: "messages", label: "Status" },
];

/*
 * widgetFields are the fields the messenger's form shows, shaped as the
 * messenger reads them.
 */
function widgetFields(fields: TicketField[]) {
  return fields
    .filter((f) => !f.archived_at && f.visible_customer)
    .map((f) => ({
      key: f.key,
      label: f.label,
      description: f.description,
      data_type: f.data_type,
      options: f.options,
      required: f.required_create_customer,
      editable: f.editable_customer,
      show_when: f.show_when,
    }));
}

/*
 * useMessengerConfig reads how the messenger customers of this type use
 * looks, so the preview wears it: the one on the type's inbox, else the
 * workspace's first live one.
 */
function useMessengerConfig(workspace: string, inbox: string | null) {
  const { data: widgets = [] } = useQuery(widgetSettingsQuery(workspace));
  const widget = widgets.find((w) => w.enabled && w.inbox.id === inbox) ?? widgets.find((w) => w.enabled) ?? widgets[0];

  return useQuery({
    queryKey: ["workspace", workspace, "widget-settings", widget?.id ?? "", "preview", "", "{}"],
    queryFn: ({ signal }) => previewWidget(workspace, widget?.id ?? "", {}, signal),
    enabled: Boolean(widget),
    staleTime: Number.POSITIVE_INFINITY,
    retry: false,
  }).data;
}

export function MessengerPreview({
  workspace,
  inbox,
  typeId,
  name,
  description,
  icon,
  fields,
  state,
  view,
  onView,
}: {
  workspace: string;
  inbox: string | null;
  typeId: string;
  name: string;
  description: string;
  icon: string;
  fields: TicketField[];
  state: TicketState | undefined;
  view: PreviewView;
  onView: (view: PreviewView) => void;
}) {
  const config = useMessengerConfig(workspace, inbox);
  const tickets = {
    types: [{ id: typeId, name: name || "Untitled type", icon, description, fields: widgetFields(fields) }],
    request: {
      reference: "T-482",
      title: name || "Your request",
      state: { label: state?.customer_label ?? "Submitted", group: state?.group ?? "submitted" },
    },
    show: view,
  };

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between gap-3 px-4 pt-4 pb-3">
        <span className="text-body-2-medium text-text-primary">Customer preview</span>
        <SegmentedControl
          aria-label="What to preview"
          selectedKeys={[view]}
          onSelectionChange={(keys) => {
            const next = [...keys][0];
            if (next) {
              onView(next as PreviewView);
            }
          }}
        >
          {views.map((v) => (
            <SegmentedControlItem key={v.id} id={v.id}>
              {v.label}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      </div>
      <div className="min-h-0 flex-1">
        <MessengerFrame
          title="Customer preview"
          commands={[
            ["configure", config],
            ["tickets", tickets],
          ]}
        />
      </div>
    </div>
  );
}
