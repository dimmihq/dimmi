/*
 * The states of one ticket type: the steps its tickets move through,
 * grouped as a ticket moves (submitted, in progress, waiting on the
 * customer, waiting on another team, resolved), each group in the
 * type's own order. A row shows the state's name, what the customer
 * sees in it, and how many of the type's tickets sit there. New tickets
 * start in the first submitted state and closing the conversation moves
 * the ticket to the first resolved one, so dragging a state to the top
 * of its group is how either is picked. Adding reuses a state another
 * type has or makes a new one. Removing a state that still holds
 * tickets asks where they go; the move happens when the type is saved.
 * Picking a row shows its step in the customer's preview.
 */
import { RiAddLine, RiDeleteBinLine, RiDraggable, RiEditLine } from "@remixicon/react";
import { useState } from "react";
import { Button as AriaButton, DropIndicator, GridList, GridListItem, useDragAndDrop } from "react-aria-components";

import { Chip } from "@/components/base/badges/chip";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Section } from "@/features/settings/layout";
import { moved } from "@/features/settings/status-order";
import { RowMenu } from "@/features/shell/row-menu";
import { type StateGroup, stateGroups, type TicketKind, type TicketState, type TicketType } from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

import { groupHint, groupLabel } from "../labels";
import { requiredGroups, type TypeDraft, type TypeSteps, typeSteps, withSteps } from "./form";
import { AddStateDialog, EditStateDialog, RemoveStateDialog } from "./type-state-dialogs";

type Open =
  | { kind: "add"; group: StateGroup }
  | { kind: "edit"; state: TicketState }
  | { kind: "remove"; state: TicketState }
  | null;

export interface StatesProps {
  workspace: string;
  type: TicketType;
  types: TicketType[];
  states: TicketState[];
  draft: TypeDraft;
  set: (change: Partial<TypeDraft>) => void;
  locked: boolean;
  moves: Record<string, string>;
  setMoves: (moves: Record<string, string>) => void;
  previewId: string | null;
  onPreview: (id: string) => void;
}

/*
 * showsLabels says whether customers ever read the type's state labels.
 */
function showsLabels(kind: TicketKind): boolean {
  return kind === "customer" || kind === "incident";
}

export function TypeStatesSection(props: StatesProps) {
  const { type, types, states, draft, set, moves, setMoves } = props;
  const [open, setOpen] = useState<Open>(null);
  const steps = typeSteps(draft.state_ids, states);
  const change = (next: TypeSteps) => set(withSteps(next));
  const usedBy = (id: string) => types.filter((t) => t.id !== type.id && !t.archived_at && t.state_ids.includes(id));
  const remove = (state: TicketState, target: string | null) => {
    change({ ...steps, [state.group]: steps[state.group].filter((s) => s.id !== state.id) });
    const { [state.id]: _, ...rest } = moves;
    setMoves(target ? { ...rest, [state.id]: target } : rest);
  };

  return (
    <Section
      title="States"
      description={
        showsLabels(type.kind)
          ? "The steps a ticket of this type moves through. Customers see the label on the right."
          : "The steps a ticket of this type moves through."
      }
    >
      {stateGroups.map((group) => (
        <GroupSteps
          key={group}
          group={group}
          steps={steps[group]}
          props={props}
          onAdd={() => setOpen({ kind: "add", group })}
          onReorder={(order) =>
            change({ ...steps, [group]: order.map((id) => steps[group].find((s) => s.id === id) as TicketState) })
          }
          onPick={(kind, state) => setOpen({ kind, state })}
        />
      ))}
      <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">
        New tickets start in the first submitted state. Closing the conversation moves the ticket to the first resolved
        state. Drag a state to the top of its group to change either.
      </p>
      {open?.kind === "add" ? (
        <AddStateDialog
          workspace={props.workspace}
          group={open.group}
          kind={type.kind}
          states={states}
          used={draft.state_ids}
          usedBy={usedBy}
          onClose={() => setOpen(null)}
          onAdd={(state) => {
            change({ ...steps, [state.group]: [...steps[state.group], state] });
            const { [state.id]: _, ...rest } = moves;
            setMoves(rest);
          }}
        />
      ) : null}
      {open?.kind === "edit" ? (
        <EditStateDialog
          workspace={props.workspace}
          state={open.state}
          usedBy={usedBy(open.state.id)}
          onClose={() => setOpen(null)}
        />
      ) : null}
      {open?.kind === "remove" ? (
        <RemoveStateDialog
          state={open.state}
          held={type.tickets_by_state?.[open.state.id] ?? 0}
          options={Object.values(steps)
            .flat()
            .filter((s) => s.id !== open.state.id)}
          onClose={() => setOpen(null)}
          onRemove={(target) => {
            remove(open.state, target);
            setOpen(null);
          }}
        />
      ) : null}
    </Section>
  );
}

const rowActions = [
  { kind: "edit" as const, label: "Edit names", icon: RiEditLine },
  { kind: "remove" as const, label: "Remove from this type", icon: RiDeleteBinLine, danger: true },
];

function GroupSteps({
  group,
  steps,
  props,
  onAdd,
  onReorder,
  onPick,
}: {
  group: StateGroup;
  steps: TicketState[];
  props: StatesProps;
  onAdd: () => void;
  onReorder: (order: string[]) => void;
  onPick: (kind: "edit" | "remove", state: TicketState) => void;
}) {
  const { locked } = props;
  const required = requiredGroups.includes(group);
  const { dragAndDropHooks } = useDragAndDrop({
    isDisabled: locked,
    getItems: (keys) => [...keys].map((key) => ({ "text/plain": String(key) })),
    onReorder: (event) =>
      onReorder(
        moved(
          steps.map((s) => s.id),
          event.keys,
          event.target.key,
          event.target.dropPosition,
        ),
      ),
    renderDropIndicator: (target) => (
      <DropIndicator
        target={target}
        className={({ isDropTarget }) => cx("me-3 h-0.5 rounded-full", isDropTarget ? "bg-accent-500" : "")}
      />
    ),
  });

  return (
    <div className="flex flex-col border-b border-separator-border pb-1 last-of-type:border-b-0">
      <div className="flex items-center gap-3 pt-3 pb-1 pe-3">
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="text-body-2-medium text-text-primary">
            {groupLabel(group)}
            {required ? null : <span className="text-text-tertiary"> · optional</span>}
          </span>
          <span className="text-caption-1-regular text-text-tertiary">{groupHint(group)}</span>
        </div>
        {locked ? null : (
          <LinkButton size="small" variant="secondary" leadingIcon={RiAddLine} onClick={onAdd}>
            Add
          </LinkButton>
        )}
      </div>
      {steps.length === 0 ? (
        <p className="py-2 pe-3 text-body-2-regular text-text-tertiary">None. Tickets skip this step.</p>
      ) : (
        <GridList
          aria-label={`${groupLabel(group)} states`}
          items={steps}
          dependencies={[steps.map((s) => s.id).join(), props.type, locked]}
          dragAndDropHooks={dragAndDropHooks}
          selectionMode="single"
          selectedKeys={props.previewId ? [props.previewId] : []}
          onSelectionChange={(keys) => {
            const id = [...keys][0];
            if (id) {
              props.onPreview(String(id));
            }
          }}
          className="flex flex-col outline-none"
        >
          {(state) => (
            <StepRow
              state={state}
              first={steps[0]?.id === state.id}
              group={group}
              props={props}
              canRemove={!required || steps.length > 1}
              onPick={onPick}
            />
          )}
        </GridList>
      )}
    </div>
  );
}

function StepRow({
  state,
  first,
  group,
  props,
  canRemove,
  onPick,
}: {
  state: TicketState;
  first: boolean;
  group: StateGroup;
  props: StatesProps;
  canRemove: boolean;
  onPick: (kind: "edit" | "remove", state: TicketState) => void;
}) {
  const { type, locked } = props;
  const held = type.tickets_by_state?.[state.id] ?? 0;
  const marker = first && group === "submitted" ? "Start" : first && group === "resolved" ? "On close" : null;

  return (
    <GridListItem
      id={state.id}
      textValue={state.name}
      className={({ isDragging, isFocusVisible, isSelected }) =>
        cx(
          "-ms-1.5 flex min-h-11 cursor-default items-center gap-2 rounded-lg py-1.5 ps-1.5 pe-3 outline-none",
          isSelected ? "bg-background-secondary-hover" : "hover:bg-background-secondary-hover",
          isDragging && "opacity-50",
          isFocusVisible && "ring-2 ring-border-focus-ring",
        )
      }
    >
      {locked ? null : (
        <AriaButton
          slot="drag"
          aria-label={`Move ${state.name}`}
          className="flex size-6 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiDraggable aria-hidden className="size-4" />
        </AriaButton>
      )}
      <span className="flex min-w-0 flex-1 items-baseline gap-2">
        <span className="shrink-0 text-body-regular text-text-primary">{state.name}</span>
        {showsLabels(type.kind) ? (
          <span className="min-w-0 truncate text-body-2-regular text-text-tertiary">“{state.customer_label}”</span>
        ) : null}
      </span>
      {marker ? (
        <Chip variant="caption" color="gray">
          {marker}
        </Chip>
      ) : null}
      {held > 0 ? (
        <span className="shrink-0 text-caption-1-regular text-text-tertiary">
          {held} {held === 1 ? "ticket" : "tickets"}
        </span>
      ) : null}
      {locked ? null : (
        <RowMenu
          quiet
          label={state.name}
          actions={canRemove ? rowActions : rowActions.slice(0, 1)}
          onPick={(kind) => onPick(kind, state)}
        />
      )}
    </GridListItem>
  );
}
