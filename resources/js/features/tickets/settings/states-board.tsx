/*
 * The workspace's ticket states, one card per group in the order a
 * ticket moves through them: submitted, in progress, waiting on the
 * customer, waiting on the team and resolved. Each state shows its
 * internal name, the label its customers see, and whether a move into it
 * tells the customer, a switch whose change waits for the page's save
 * bar. A member who may
 * change settings adds a state to a group, renames one, drags one by its
 * handle within its group (the new order saves on drop and goes back,
 * with a note, when the save fails) and deletes one after picking a
 * state of the same group for its tickets. A state's group never
 * changes.
 */
import { RiDeleteBinLine, RiDraggable, RiEditLine, RiInformationLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";
import { Button as AriaButton, DropIndicator, GridList, GridListItem, useDragAndDrop } from "react-aria-components";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { toast } from "@/components/base/toast/toast";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { Action } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import { Empty, Section } from "@/features/settings/layout";
import { moved } from "@/features/settings/status-order";
import { RowMenu } from "@/features/shell/row-menu";
import {
  createTicketState,
  deleteTicketState,
  reorderTicketStates,
  type StateGroup,
  stateGroups,
  type TicketState,
  ticketKeys,
  ticketStatesQuery,
  updateTicketState,
} from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

import { groupHint, groupLabel } from "../labels";
import { byGroup, withGroupOrder } from "./form";
import type { NotifyDraft } from "./notify-draft";

type Open =
  | { kind: "edit"; group: StateGroup; state: TicketState | null }
  | { kind: "delete"; state: TicketState }
  | null;

const actions = [
  { kind: "edit" as const, label: "Edit", icon: RiEditLine },
  { kind: "delete" as const, label: "Delete", icon: RiDeleteBinLine, danger: true },
];

/*
 * useStates reads the states and keeps them current after a change; a
 * new order shows at once and goes back when the save fails.
 */
function useStates(workspace: string) {
  const client = useQueryClient();
  const key = ticketKeys.states(workspace);
  const { data: states = [], isPending } = useQuery(ticketStatesQuery(workspace));
  const refresh = () => client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
  const reorder = useMutation({
    mutationFn: (order: string[]) => reorderTicketStates(workspace, order),
    onMutate: async (order) => {
      await client.cancelQueries({ queryKey: key });
      const previous = client.getQueryData<TicketState[]>(key);
      client.setQueryData(
        key,
        previous?.map((s) => ({ ...s, position: order.indexOf(s.id) === -1 ? s.position : order.indexOf(s.id) })),
      );
      return { previous };
    },
    onError: (_error, _order, context) => {
      client.setQueryData(key, context?.previous);
      toast.error("The new order was not saved.");
    },
    onSettled: refresh,
  });

  return { states, isPending, refresh, reorder };
}

export function StatesBoard({ workspace, draft }: { workspace: string; draft: NotifyDraft }) {
  const { states, isPending, refresh, reorder } = useStates(workspace);
  const canChange = usePermission(workspace, "settings:write");
  const [open, setOpen] = useState<Open>(null);
  const groups = byGroup(states);
  const shown = states.map((s) =>
    s.id in draft.pending ? { ...s, notify_customer: draft.pending[s.id] as boolean } : s,
  );

  return (
    <>
      {stateGroups.map((group) => (
        <Section
          key={group}
          title={`States · ${groupLabel(group)}`}
          description={groupHint(group)}
          loading={isPending}
          rows={2}
          action={
            canChange ? <Action onPress={() => setOpen({ kind: "edit", group, state: null })}>Add state</Action> : null
          }
        >
          <GroupList
            group={group}
            states={byGroup(shown)[group]}
            canChange={canChange}
            onReorder={(order) => reorder.mutate(withGroupOrder(states, group, order))}
            onNotify={(state, on) => {
              const saved = states.find((s) => s.id === state.id);
              if (saved) {
                draft.flip(saved, on);
              }
            }}
            onPick={(kind, state) => setOpen(kind === "edit" ? { kind, group, state } : { kind, state })}
          />
        </Section>
      ))}
      {open?.kind === "edit" ? (
        <StateDialog
          group={open.group}
          state={open.state}
          onClose={() => setOpen(null)}
          onSave={async (change) => {
            try {
              await (open.state
                ? updateTicketState(workspace, open.state.id, change)
                : createTicketState(workspace, { ...change, group: open.group, notify_customer: true }));
              await refresh();
              toast.success(open.state ? "State saved" : "State added");
            } catch (error) {
              toast.error((error as Error).message);
              throw error;
            }
          }}
        />
      ) : null}
      {open?.kind === "delete" ? (
        <DeleteState
          state={open.state}
          options={groups[open.state.group].filter((s) => s.id !== open.state.id)}
          onClose={() => setOpen(null)}
          onConfirm={async (target) => {
            try {
              await deleteTicketState(workspace, open.state.id, target);
              await refresh();
              toast.success("State deleted");
              setOpen(null);
            } catch (error) {
              toast.error((error as Error).message);
              throw error;
            }
          }}
        />
      ) : null}
    </>
  );
}

function GroupList({
  group,
  states,
  canChange,
  onReorder,
  onNotify,
  onPick,
}: {
  group: StateGroup;
  states: TicketState[];
  canChange: boolean;
  onReorder: (order: string[]) => void;
  onNotify: (state: TicketState, on: boolean) => void;
  onPick: (kind: "edit" | "delete", state: TicketState) => void;
}) {
  const { dragAndDropHooks } = useDragAndDrop({
    isDisabled: !canChange,
    getItems: (keys) => [...keys].map((key) => ({ "text/plain": String(key) })),
    onReorder: (event) =>
      onReorder(
        moved(
          states.map((s) => s.id),
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

  if (states.length === 0) {
    return <Empty>No states in this group.</Empty>;
  }

  return (
    <GridList
      aria-label={`${groupLabel(group)} states`}
      items={states}
      dragAndDropHooks={dragAndDropHooks}
      className="flex flex-col outline-none"
    >
      {(state) => (
        <GridListItem
          id={state.id}
          textValue={state.name}
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
              aria-label={`Move ${state.name}`}
              className="flex size-6 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiDraggable aria-hidden className="size-4" />
            </AriaButton>
          ) : null}
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="truncate text-body-regular text-text-primary">{state.name}</span>
            <span className="truncate text-body-2-regular text-text-secondary">
              Customer sees “{state.customer_label}”
            </span>
          </span>
          <span className="flex items-center gap-1">
            <Switch
              size="sm"
              isSelected={state.notify_customer}
              isDisabled={!canChange}
              onChange={(on) => onNotify(state, on)}
            >
              <span className="text-body-2-regular text-text-secondary">Notify</span>
            </Switch>
            <NotifyHelp state={state} />
          </span>
          {canChange ? (
            <RowMenu
              label={state.name}
              actions={states.length > 1 ? actions : actions.slice(0, 1)}
              onPick={(kind) => onPick(kind, state)}
            />
          ) : null}
        </GridListItem>
      )}
    </GridList>
  );
}

/*
 * NotifyHelp explains what Notify does for this state: who hears, how,
 * when, and when nothing is sent.
 */
function NotifyHelp({ state }: { state: TicketState }) {
  return (
    <TooltipTrigger delay={150}>
      <AriaButton
        aria-label={`What Notify does for ${state.name}`}
        className="flex size-6 items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:text-foreground-icon-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiInformationLine aria-hidden className="size-4" />
      </AriaButton>
      <Tooltip size="md" placement="left" className="max-w-[320px]">
        <span className="flex flex-col gap-2 text-body-2-regular">
          <span>
            When a ticket moves into <b className="text-body-2-medium">{state.name}</b>, the customer who asked gets “
            {state.customer_label}” in their conversation, on the channel they wrote from: the messenger, email or
            Slack.
          </span>
          <span className="text-text-secondary">
            dimmi waits two minutes first. Nothing is sent if the ticket moves on, a teammate replies, the customer was
            already told this state, or the customer made the change. Only tickets the customer can see send it.
          </span>
          <span className="text-text-secondary">
            Off: the state changes quietly. Customers still see it in the messenger and the help center.
          </span>
        </span>
      </Tooltip>
    </TooltipTrigger>
  );
}

/*
 * StateDialog adds a state to a group, or changes the one given.
 */
function StateDialog({
  group,
  state,
  onSave,
  onClose,
}: {
  group: StateGroup;
  state: TicketState | null;
  onSave: (change: { name: string; customer_label: string }) => Promise<unknown>;
  onClose: () => void;
}) {
  const formId = useId();
  const [name, setName] = useState(state?.name ?? "");
  const [label, setLabel] = useState(state?.customer_label ?? "");
  const [saving, setSaving] = useState(false);
  const submit = async () => {
    setSaving(true);
    try {
      await onSave({ name: name.trim(), customer_label: label.trim() || name.trim() });
      onClose();
    } catch {
      // The toast already said why.
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={state ? `Edit ${state.name}` : `New ${groupLabel(group).toLowerCase()} state`}
        description="Tickets keep their state when you rename it. Customers only ever see the label."
      />
      <ModalBody className="pb-4">
        <form
          id={formId}
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          className="flex flex-col gap-4"
        >
          <Input label="Name" value={name} onChange={setName} maxLength={60} isRequired placeholder="With billing" />
          <Input
            label="What the customer sees"
            value={label}
            onChange={setLabel}
            maxLength={80}
            placeholder="With our billing team"
            hint="Leave empty to show the name."
          />
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button size="small" type="submit" form={formId} loading={saving} disabled={name.trim() === ""}>
          {state ? "Save state" : "Add state"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * DeleteState removes a state after asking which state of the same group
 * its tickets move to.
 */
function DeleteState({
  state,
  options,
  onConfirm,
  onClose,
}: {
  state: TicketState;
  options: TicketState[];
  onConfirm: (target: string) => Promise<unknown>;
  onClose: () => void;
}) {
  const [target, setTarget] = useState(options[0]?.id ?? "");

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={`Delete ${state.name}?`}
      description={`Its tickets move to the ${groupLabel(state.group).toLowerCase()} state you pick. Customers are not told.`}
      confirmLabel="Delete state"
      isConfirmDisabled={target === ""}
      onConfirm={() => onConfirm(target)}
    >
      <Select label="Move tickets to" selectedKey={target} onSelectionChange={(key) => setTarget(String(key))}>
        {options.map((o) => (
          <SelectItem key={o.id} id={o.id} textValue={o.name}>
            {o.name}
          </SelectItem>
        ))}
      </Select>
    </AlertDialog>
  );
}
