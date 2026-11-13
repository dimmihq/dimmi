/*
 * The dialogs of a type's states. Adding picks a state of the group that
 * another type already uses, showing which, or makes a new one with its
 * name and what the customer sees; a new state is made at once and joins
 * the type when the type is saved. Editing renames a state everywhere it
 * is used, saying where first. Removing a state that holds some of the
 * type's tickets asks which of the type's other states they move to.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import {
  createTicketState,
  type StateGroup,
  type TicketState,
  type TicketType,
  ticketKeys,
  updateTicketState,
} from "@/lib/api/tickets";

import { groupLabel } from "../labels";
import { addable } from "./form";

const fresh = "__new";

/*
 * usedByText names the types that use a state.
 */
function usedByText(types: TicketType[]): string {
  if (types.length === 0) {
    return "Not used by another type";
  }
  const names = types.map((t) => t.name);

  return names.length <= 2 ? `Used by ${names.join(" and ")}` : `Used by ${names[0]} and ${names.length - 1} more`;
}

function NameFields({
  name,
  label,
  onName,
  onLabel,
}: {
  name: string;
  label: string;
  onName: (v: string) => void;
  onLabel: (v: string) => void;
}) {
  return (
    <>
      <Input label="Name" value={name} onChange={onName} maxLength={60} isRequired placeholder="With billing" />
      <Input
        label="What the customer sees"
        value={label}
        onChange={onLabel}
        maxLength={80}
        placeholder="With our billing team"
        hint="Leave empty to show the name."
      />
    </>
  );
}

export function AddStateDialog({
  workspace,
  group,
  states,
  used,
  usedBy,
  onAdd,
  onClose,
}: {
  workspace: string;
  group: StateGroup;
  states: TicketState[];
  used: string[];
  usedBy: (id: string) => TicketType[];
  onAdd: (state: TicketState) => void;
  onClose: () => void;
}) {
  const formId = useId();
  const client = useQueryClient();
  const options = addable(states, group, used);
  const [picked, setPicked] = useState(options[0]?.id ?? fresh);
  const [name, setName] = useState("");
  const [label, setLabel] = useState("");
  const create = useMutation({
    mutationFn: () =>
      createTicketState(workspace, {
        group,
        name: name.trim(),
        customer_label: label.trim() || name.trim(),
        notify_customer: true,
      }),
    onSuccess: async (state) => {
      await client.invalidateQueries({ queryKey: ticketKeys.states(workspace) });
      onAdd(state);
      onClose();
    },
    onError: (error) => toast.error(error.message),
  });
  const submit = () => {
    if (picked !== fresh) {
      const state = options.find((s) => s.id === picked);
      if (state) {
        onAdd(state);
      }
      onClose();
      return;
    }
    create.mutate();
  };

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={`Add a state to ${groupLabel(group)}`}
        description={
          options.length > 0
            ? "Reuse a state another type has, or make a new one."
            : "Name it and choose what customers see."
        }
      />
      <ModalBody className="pb-4">
        <form
          id={formId}
          onSubmit={(event) => {
            event.preventDefault();
            submit();
          }}
          className="flex flex-col gap-4"
        >
          {options.length > 0 ? (
            <RadioGroup aria-label="State" value={picked} onChange={setPicked} className="flex flex-col gap-2">
              {options.map((s) => (
                <RadioCard
                  key={s.id}
                  value={s.id}
                  title={s.name}
                  description={`Customers see “${s.customer_label}” · ${usedByText(usedBy(s.id))}`}
                />
              ))}
              <RadioCard value={fresh} title="New state" description="Name it and choose what customers see." />
            </RadioGroup>
          ) : null}
          {picked === fresh ? <NameFields name={name} label={label} onName={setName} onLabel={setLabel} /> : null}
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button
          size="small"
          type="submit"
          form={formId}
          loading={create.isPending}
          disabled={picked === fresh && name.trim() === ""}
        >
          Add state
        </Button>
      </ModalFooter>
    </Modal>
  );
}

export function EditStateDialog({
  workspace,
  state,
  usedBy,
  onClose,
}: {
  workspace: string;
  state: TicketState;
  usedBy: TicketType[];
  onClose: () => void;
}) {
  const formId = useId();
  const client = useQueryClient();
  const [name, setName] = useState(state.name);
  const [label, setLabel] = useState(state.customer_label);
  const save = useMutation({
    mutationFn: () =>
      updateTicketState(workspace, state.id, {
        name: name.trim(),
        customer_label: label.trim() || name.trim(),
      }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
      toast.success("State renamed");
      onClose();
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={`Edit ${state.name}`}
        description={
          usedBy.length > 0
            ? `This also renames it in ${usedBy.map((t) => t.name).join(", ")}. Tickets keep their state.`
            : "Tickets keep their state when you rename it."
        }
      />
      <ModalBody className="pb-4">
        <form
          id={formId}
          onSubmit={(event) => {
            event.preventDefault();
            save.mutate();
          }}
          className="flex flex-col gap-4"
        >
          <NameFields name={name} label={label} onName={setName} onLabel={setLabel} />
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button size="small" type="submit" form={formId} loading={save.isPending} disabled={name.trim() === ""}>
          Save names
        </Button>
      </ModalFooter>
    </Modal>
  );
}

export function RemoveStateDialog({
  state,
  held,
  options,
  onRemove,
  onClose,
}: {
  state: TicketState;
  held: number;
  options: TicketState[];
  onRemove: (target: string | null) => void;
  onClose: () => void;
}) {
  const sorted = [...options].sort((a, b) => Number(b.group === state.group) - Number(a.group === state.group));
  const [target, setTarget] = useState(sorted[0]?.id ?? "");

  if (held === 0) {
    return (
      <AlertDialog
        isOpen
        onOpenChange={(open) => (open ? undefined : onClose())}
        title={`Remove ${state.name}?`}
        description="No tickets of this type are in it. Other types keep it. Nothing changes until you save."
        confirmLabel="Remove"
        onConfirm={() => onRemove(null)}
      />
    );
  }

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={`Remove ${state.name}?`}
      description={`${held} ${held === 1 ? "ticket of this type is" : "tickets of this type are"} in ${state.name}. They move to the state you pick when you save. Customers are not told.`}
      confirmLabel="Remove"
      isConfirmDisabled={target === ""}
      onConfirm={() => onRemove(target)}
    >
      <Select label="Move them to" selectedKey={target} onSelectionChange={(key) => setTarget(String(key))}>
        {sorted.map((o) => (
          <SelectItem key={o.id} id={o.id} textValue={o.name}>
            {o.name}
            <span className="ms-2 text-text-tertiary">{groupLabel(o.group)}</span>
          </SelectItem>
        ))}
      </Select>
    </AlertDialog>
  );
}
