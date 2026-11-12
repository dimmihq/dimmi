/*
 * A ticket type's fields in the order its forms and panel show them.
 * Each row names the field, its data type, and the marks that matter at
 * a glance: required, shown to the customer, conditional, filled by AI.
 * A member who may change settings drags a row by its handle to reorder
 * (saved on drop, put back with a note when the save fails), adds or
 * edits a field in a dialog, and archives one after confirming; old
 * tickets keep the values of an archived field.
 */
import { RiArchiveLine, RiDraggable, RiEditLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button as AriaButton, DropIndicator, GridList, GridListItem, useDragAndDrop } from "react-aria-components";

import { Chip } from "@/components/base/badges/chip";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Action } from "@/features/settings/controls";
import { Empty, Section } from "@/features/settings/layout";
import { moved } from "@/features/settings/status-order";
import { RowMenu } from "@/features/shell/row-menu";
import {
  archiveTicketField,
  createTicketField,
  type FieldChange,
  reorderTicketFields,
  type TicketField,
  type TicketType,
  ticketKeys,
  updateTicketField,
} from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

import { dataTypeLabel } from "../labels";
import { FieldDialog } from "./field-dialog";

type Open = { kind: "edit"; field: TicketField | null } | { kind: "archive"; field: TicketField } | null;

const actions = [
  { kind: "edit" as const, label: "Edit", icon: RiEditLine },
  { kind: "archive" as const, label: "Archive", icon: RiArchiveLine, danger: true },
];

function marks(field: TicketField): string[] {
  const out: string[] = [];
  if (field.required_create || field.required_create_customer) {
    out.push("Required");
  }
  if (field.required_resolve) {
    out.push("To resolve");
  }
  if (field.visible_customer) {
    out.push("Customer");
  }
  if (field.show_when) {
    out.push("Conditional");
  }
  if (field.ai_hint) {
    out.push(field.ai_auto ? "AI fills" : "AI suggests");
  }

  return out;
}

function detailOf(field: TicketField, all: TicketField[]): string {
  const when = field.show_when;
  if (!when) {
    return `${dataTypeLabel(field.data_type)} · ${field.key}`;
  }
  const parent = all.find((f) => f.key === when.field)?.label ?? when.field;
  const values = when.equals?.length ? ` is ${when.equals.join(" or ")}` : " is filled";

  return `${dataTypeLabel(field.data_type)} · shows when ${parent}${values}`;
}

/*
 * useReorder saves a new order of the type's fields, showing it at once.
 */
function useReorder(workspace: string, type: TicketType) {
  const client = useQueryClient();
  const key = ticketKeys.types(workspace);

  return useMutation({
    mutationFn: (order: string[]) => reorderTicketFields(workspace, type.id, order),
    onMutate: async (order) => {
      await client.cancelQueries({ queryKey: key });
      const previous = client.getQueryData<TicketType[]>(key);
      client.setQueryData<TicketType[]>(key, (types) =>
        types?.map((t) =>
          t.id === type.id
            ? { ...t, fields: [...t.fields].sort((a, b) => order.indexOf(a.id) - order.indexOf(b.id)) }
            : t,
        ),
      );
      return { previous };
    },
    onError: (_error, _order, context) => {
      client.setQueryData(key, context?.previous);
      toast.error("The new order was not saved.");
    },
    onSettled: () => client.invalidateQueries({ queryKey: key }),
  });
}

export function FieldBuilder({
  workspace,
  type,
  canChange,
}: {
  workspace: string;
  type: TicketType;
  canChange: boolean;
}) {
  const client = useQueryClient();
  const fields = type.fields.filter((f) => !f.archived_at);
  const reorder = useReorder(workspace, type);
  const [open, setOpen] = useState<Open>(null);
  const refresh = () => client.invalidateQueries({ queryKey: ticketKeys.types(workspace) });
  const { dragAndDropHooks } = useDragAndDrop({
    isDisabled: !canChange,
    getItems: (keys) => [...keys].map((key) => ({ "text/plain": String(key) })),
    onReorder: (event) =>
      reorder.mutate(
        moved(
          fields.map((f) => f.id),
          event.keys,
          event.target.key,
          event.target.dropPosition,
        ),
      ),
    renderDropIndicator: (target) => (
      <DropIndicator
        target={target}
        className={({ isDropTarget }) => cx("me-2.5 h-0.5 rounded-full", isDropTarget ? "bg-accent-500" : "")}
      />
    ),
  });
  const save = async (field: TicketField | null, change: FieldChange) => {
    try {
      await (field ? updateTicketField(workspace, field.id, change) : createTicketField(workspace, type.id, change));
      await refresh();
      toast.success(field ? "Field saved" : "Field added");
    } catch (error) {
      toast.error((error as Error).message);
      throw error;
    }
  };

  return (
    <Section
      title="Fields"
      description="Drag to set the order of the form and the ticket panel."
      action={canChange ? <Action onPress={() => setOpen({ kind: "edit", field: null })}>Add field</Action> : null}
    >
      {fields.length === 0 ? <Empty>No fields yet. A ticket still has a title and description.</Empty> : null}
      {fields.length > 0 ? (
        <GridList
          aria-label="Fields"
          items={fields}
          dragAndDropHooks={dragAndDropHooks}
          className="flex flex-col outline-none"
        >
          {(field) => (
            <GridListItem
              id={field.id}
              textValue={field.label}
              className={({ isDragging, isFocusVisible }) =>
                cx(
                  "flex min-h-[52px] items-center gap-3 border-b border-separator-border py-2.5 pe-2.5 outline-none last:border-b-0",
                  isDragging && "opacity-50",
                  isFocusVisible && "rounded-md ring-2 ring-border-focus-ring",
                )
              }
            >
              {canChange ? (
                <AriaButton
                  slot="drag"
                  aria-label={`Move ${field.label}`}
                  className="flex size-6 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                >
                  <RiDraggable aria-hidden className="size-4" />
                </AriaButton>
              ) : null}
              <span className="flex min-w-0 flex-1 flex-col gap-1">
                <span className="truncate text-body-regular text-text-primary">{field.label}</span>
                <span className="truncate text-body-2-regular text-text-secondary">{detailOf(field, fields)}</span>
                <span className="flex flex-wrap gap-1">
                  {marks(field).map((mark) => (
                    <Chip key={mark} variant="caption" color={mark.startsWith("AI") ? "purple" : "neutral"}>
                      {mark}
                    </Chip>
                  ))}
                </span>
              </span>
              {canChange ? (
                <RowMenu
                  label={field.label}
                  actions={actions}
                  onPick={(kind) => setOpen(kind === "edit" ? { kind, field } : { kind, field })}
                />
              ) : null}
            </GridListItem>
          )}
        </GridList>
      ) : null}
      {open?.kind === "edit" ? (
        <FieldDialog
          field={open.field}
          others={fields.filter((f) => f.id !== open.field?.id)}
          onSave={(change) => save(open.field, change)}
          onClose={() => setOpen(null)}
        />
      ) : null}
      {open?.kind === "archive" ? (
        <AlertDialog
          isOpen
          onOpenChange={(next) => (next ? undefined : setOpen(null))}
          title={`Archive ${open.field.label}?`}
          description="It leaves the form and the panel. Tickets keep the values they already have."
          confirmLabel="Archive field"
          onConfirm={async () => {
            try {
              await archiveTicketField(workspace, open.field.id);
              await refresh();
              toast.success("Field archived");
              setOpen(null);
            } catch (error) {
              toast.error((error as Error).message);
              throw error;
            }
          }}
        />
      ) : null}
    </Section>
  );
}
