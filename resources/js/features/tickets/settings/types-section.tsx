/*
 * The workspace's ticket types, each with its kind and how many tickets
 * it holds, open and in all. A row opens the type's editor. A member who
 * may change settings adds a type, picking its kind on radio cards and
 * naming it, and lands in its editor; archiving a type asks first, and
 * its old tickets keep it. Archived types sit at the end, marked.
 */
import { RiArchiveLine, RiEditLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useId, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { Action } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import { Empty, Item, Section } from "@/features/settings/layout";
import { RowMenu } from "@/features/shell/row-menu";
import {
  archiveTicketType,
  createTicketType,
  type TicketKind,
  type TicketType,
  ticketKeys,
  ticketKinds,
  ticketTypesQuery,
} from "@/lib/api/tickets";

import { kindHint, kindLabel, typeIcon } from "../labels";

const actions = [
  { kind: "edit" as const, label: "Edit", icon: RiEditLine },
  { kind: "archive" as const, label: "Archive", icon: RiArchiveLine, danger: true },
];

function counts(type: TicketType): string {
  return `${kindLabel(type.kind)} · ${type.open_tickets} open · ${type.total_tickets} total`;
}

function TypeMark({ type }: { type: TicketType }) {
  const Icon = typeIcon(type.icon, type.kind);

  return (
    <span aria-hidden className="flex size-8 items-center justify-center rounded-lg bg-background-tertiary-default">
      <Icon className="size-4 text-foreground-icon-secondary" />
    </span>
  );
}

/*
 * ordered puts live types first, in their order, then archived ones.
 */
function ordered(types: TicketType[]): TicketType[] {
  return [...types].sort(
    (a, b) => Number(Boolean(a.archived_at)) - Number(Boolean(b.archived_at)) || a.position - b.position,
  );
}

export function TypesSection({ workspace }: { workspace: string }) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const { data: types = [], isPending } = useQuery(ticketTypesQuery(workspace));
  const canChange = usePermission(workspace, "settings:write");
  const [adding, setAdding] = useState(false);
  const [archiving, setArchiving] = useState<TicketType | null>(null);
  const open = (type: TicketType) => void navigate({ to: "/d/settings/tickets/$type", params: { type: type.id } });

  return (
    <Section
      title="Types"
      description="What a ticket is about. Each type has its own fields and the states it moves through."
      loading={isPending}
      action={canChange ? <Action onPress={() => setAdding(true)}>New type</Action> : null}
    >
      {types.length === 0 ? <Empty>No ticket types yet.</Empty> : null}
      {ordered(types).map((type) => (
        <Item
          key={type.id}
          mark={<TypeMark type={type} />}
          title={
            <button
              type="button"
              onClick={() => open(type)}
              className="text-start outline-none after:absolute after:inset-0 focus-visible:underline"
            >
              {type.name}
            </button>
          }
          detail={counts(type)}
        >
          {type.archived_at ? (
            <Chip variant="caption" color="neutral">
              Archived
            </Chip>
          ) : null}
          {canChange && !type.archived_at ? (
            <RowMenu
              label={type.name}
              actions={actions}
              onPick={(kind) => (kind === "edit" ? open(type) : setArchiving(type))}
            />
          ) : null}
        </Item>
      ))}
      {adding ? (
        <NewTypeDialog
          onClose={() => setAdding(false)}
          onSave={async (kind, name) => {
            const made = await createTicketType(workspace, { kind, name });
            await client.invalidateQueries({ queryKey: ticketKeys.types(workspace) });
            toast.success("Type added");
            open(made);
          }}
        />
      ) : null}
      {archiving ? (
        <AlertDialog
          isOpen
          onOpenChange={(next) => (next ? undefined : setArchiving(null))}
          title={`Archive ${archiving.name}?`}
          description={`New tickets can no longer use it. Its ${archiving.total_tickets} tickets keep it, and so do their fields.`}
          confirmLabel="Archive type"
          onConfirm={async () => {
            try {
              await archiveTicketType(workspace, archiving.id);
              await client.invalidateQueries({ queryKey: ticketKeys.types(workspace) });
              toast.success("Type archived");
              setArchiving(null);
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

/*
 * NewTypeDialog asks for a new type's kind and name. The kind is fixed
 * once the type exists.
 */
function NewTypeDialog({
  onSave,
  onClose,
}: {
  onSave: (kind: TicketKind, name: string) => Promise<unknown>;
  onClose: () => void;
}) {
  const formId = useId();
  const [kind, setKind] = useState<TicketKind>("customer");
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const submit = async () => {
    setSaving(true);
    try {
      await onSave(kind, name.trim());
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title="New ticket type"
        description="Pick what kind of work it tracks. The kind can't change later."
      />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <RadioGroup
          aria-label="Kind"
          value={kind}
          onChange={(next) => setKind(next as TicketKind)}
          className="flex flex-col gap-2"
        >
          {ticketKinds.map((k) => (
            <RadioCard key={k} value={k} title={kindLabel(k)} description={kindHint(k)} />
          ))}
        </RadioGroup>
        <form
          id={formId}
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <Input label="Name" value={name} onChange={setName} maxLength={60} isRequired placeholder="Refund request" />
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button size="small" type="submit" form={formId} loading={saving} disabled={name.trim() === ""}>
          Add type
        </Button>
      </ModalFooter>
    </Modal>
  );
}
