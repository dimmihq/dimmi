/*
 * Conversation attributes: what the workspace wants known about every
 * conversation, such as the issue type or an order number. Teammates
 * fill them in the conversation's details, the AI fills the ones it may
 * as the customer writes, or both. One may be required before a teammate
 * closes a conversation, and shown only to some teams. Workflows branch
 * and start on them, and inbox views and reports filter by them.
 *
 * The list shows the active attributes, or the archived ones, each with
 * its type and who fills it. New and Edit open a drawer; Done there puts
 * the change in the draft, and nothing is sent until Save in the bar at
 * the bottom is pressed. Discard puts everything back, and leaving with
 * changes not saved asks first. A key and a type are fixed once saved;
 * archiving keeps every value an attribute holds.
 */
import {
  RiAddLine,
  RiAlignLeft,
  RiAttachment2,
  RiBuilding2Line,
  RiCalendarLine,
  RiCheckboxMultipleLine,
  RiDraggable,
  RiHashtag,
  RiListCheck,
  RiMore2Line,
  RiNumbersLine,
  RiText,
  RiTimeLine,
  RiToggleLine,
  RiUser3Line,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { type ComponentType, type DragEvent, type ReactNode, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Drawer, DrawerBody, DrawerFooter, DrawerHeader } from "@/components/base/drawer/drawer";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { MultiPick } from "@/features/automation/pick";
import { keyFor } from "@/features/automation/step-values";
import { usePermission } from "@/features/settings/inboxes";
import {
  aiFillable,
  type ConversationAttribute,
  type ConversationAttributeChange,
  type ConversationAttributeInput,
  type ConversationAttributeType,
  conversationAttributesQuery,
  conversationAttributeTypes,
  createConversationAttribute,
  type FilledBy,
  filledByLabels,
  reorderConversationAttributes,
  updateConversationAttribute,
} from "@/lib/api/conversation-attributes";
import { teamsQuery } from "@/lib/api/teams";
import { cx } from "@/utils/cx";

import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { optionList } from "../options";
import { OptionsEditor } from "../options-editor";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

const typeIcons: Record<ConversationAttributeType, ComponentType<{ className?: string }>> = {
  text: RiText,
  long_text: RiAlignLeft,
  number: RiHashtag,
  decimal: RiNumbersLine,
  choice: RiListCheck,
  multiple_choice: RiCheckboxMultipleLine,
  boolean: RiToggleLine,
  date: RiCalendarLine,
  datetime: RiTimeLine,
  file: RiAttachment2,
  contact: RiUser3Line,
  company: RiBuilding2Line,
};

const typeHints: Record<ConversationAttributeType, string> = {
  text: "One line, such as an order number.",
  long_text: "Several lines, such as steps to reproduce.",
  number: "A whole number, such as seats.",
  decimal: "A number with decimals, such as a refund amount.",
  choice: "One of a list, such as the issue type.",
  multiple_choice: "Any of a list, such as the products involved.",
  boolean: "Yes or no, such as whether it's a bug.",
  date: "A day, such as a renewal date.",
  datetime: "A day and a time, such as a callback.",
  file: "Files a teammate attaches, such as a screenshot.",
  contact: "A link to one of your contacts.",
  company: "A link to one of your companies.",
};

const tabs = { active: "active", archived: "archived" } as const;

const choosable: ConversationAttributeType[] = ["choice", "multiple_choice"];

/*
 * Added is a new attribute in the draft, under an id of its own until
 * it is saved.
 */
interface Added extends ConversationAttributeInput {
  draft: string;
}

/*
 * useDraft holds the changes not saved yet: edits by attribute id, the
 * attributes added, and a new order. Save sends them one by one, then
 * reads the list again; Discard drops them.
 */
function useDraft(workspace: string) {
  const client = useQueryClient();
  const [edits, setEdits] = useState<Record<string, ConversationAttributeChange>>({});
  const [added, setAdded] = useState<Added[]>([]);
  const [order, setOrder] = useState<string[] | null>(null);
  const save = useMutation({
    mutationFn: async () => {
      for (const { draft: _, ...body } of added) {
        await createConversationAttribute(workspace, body);
      }
      for (const [id, change] of Object.entries(edits)) {
        await updateConversationAttribute(workspace, id, change);
      }
      if (order) {
        await reorderConversationAttributes(workspace, order);
      }
    },
    onSuccess: async () => {
      setEdits({});
      setAdded([]);
      setOrder(null);
      await client.invalidateQueries({ queryKey: conversationAttributesQuery(workspace).queryKey });
      toast.success("Conversation attributes saved");
    },
  });

  return {
    edits,
    added,
    order,
    save,
    dirty: Object.keys(edits).length > 0 || added.length > 0 || order !== null,
    reorder: setOrder,
    edit: (id: string, change: ConversationAttributeChange) =>
      setEdits((all) => ({ ...all, [id]: { ...all[id], ...change } })),
    add: (input: ConversationAttributeInput) => setAdded((all) => [...all, { ...input, draft: crypto.randomUUID() }]),
    change: (draft: string, input: ConversationAttributeInput) =>
      setAdded((all) => all.map((item) => (item.draft === draft ? { ...input, draft } : item))),
    drop: (draft: string) => setAdded((all) => all.filter((item) => item.draft !== draft)),
    discard: () => {
      save.reset();
      setEdits({});
      setAdded([]);
      setOrder(null);
    },
  };
}

type Draft = ReturnType<typeof useDraft>;

/*
 * Editing is what the drawer works on: a new attribute, one added in the
 * draft, or one saved.
 */
type Editing = { kind: "new" } | { kind: "added"; item: Added } | { kind: "saved"; attribute: ConversationAttribute };

/*
 * Form is everything the drawer edits.
 */
interface Form {
  label: string;
  key: string;
  dataType: ConversationAttributeType;
  options: string[];
  descriptions: Record<string, string>;
  description: string;
  filledBy: FilledBy;
  required: boolean;
  teams: string[];
}

function formOf(editing: Editing, draft: Draft): Form {
  if (editing.kind === "new") {
    return {
      label: "",
      key: "",
      dataType: "choice",
      options: [],
      descriptions: {},
      description: "",
      filledBy: "both",
      required: false,
      teams: [],
    };
  }
  const base = editing.kind === "added" ? editing.item : { ...editing.attribute, ...draft.edits[editing.attribute.id] };

  return {
    label: base.label,
    key: base.key,
    dataType: base.data_type,
    options: base.options ?? [],
    descriptions: base.option_descriptions ?? {},
    description: base.description ?? "",
    filledBy: base.filled_by ?? "both",
    required: base.required_to_close ?? false,
    teams: base.team_ids ?? [],
  };
}

/*
 * describedOf keeps the descriptions of the options the list has, and
 * drops empty ones.
 */
function describedOf(options: string[], descriptions: Record<string, string>): Record<string, string> {
  return Object.fromEntries(
    options.map((o) => [o, (descriptions[o] ?? "").trim()] as const).filter(([, text]) => text !== ""),
  );
}

/*
 * OptionDescriptions says what each option means, one line each, for
 * the AI to read when it picks one.
 */
function OptionDescriptions({
  options,
  value,
  onChange,
}: {
  options: string[];
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
}) {
  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="pb-0.5 text-body-2-medium text-text-primary">What each option means</legend>
      <p className="pb-1 text-caption-1-regular text-text-tertiary">
        Optional. The AI reads these to pick the right one.
      </p>
      {options.map((option) => (
        <div key={option} className="grid grid-cols-1 gap-1 sm:grid-cols-[8rem_minmax(0,1fr)] sm:items-center sm:gap-3">
          <span className="truncate text-body-2-regular text-text-secondary" title={option}>
            {option}
          </span>
          <Input
            aria-label={`What ${option} means`}
            size="small"
            className="min-w-0"
            value={value[option] ?? ""}
            placeholder={`When to pick ${option}`}
            maxLength={300}
            onChange={(text) => onChange({ ...value, [option]: text })}
          />
        </div>
      ))}
    </fieldset>
  );
}

function AttributeDrawer({ editing, draft, onClose }: { editing: Editing; draft: Draft; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const teams = useQuery(teamsQuery(workspace)).data ?? [];
  const [form, setForm] = useState<Form>(() => formOf(editing, draft));
  const [keyTouched, setKeyTouched] = useState(editing.kind !== "new");
  const saved = editing.kind === "saved";
  const key = keyTouched ? form.key : keyFor(form.label);
  const set = (change: Partial<Form>) => setForm((was) => ({ ...was, ...change }));
  const canAI = aiFillable(form.dataType);
  const filledBy: FilledBy = canAI ? form.filledBy : "teammates";
  const options = optionList(form.options);
  const ready = form.label.trim() !== "" && key !== "" && (!choosable.includes(form.dataType) || options.length > 0);

  const done = () => {
    const input: ConversationAttributeInput = {
      key,
      label: form.label.trim(),
      description: form.description.trim(),
      data_type: form.dataType,
      options: choosable.includes(form.dataType) ? options : [],
      option_descriptions: choosable.includes(form.dataType) ? describedOf(options, form.descriptions) : {},
      filled_by: filledBy,
      required_to_close: form.required,
      team_ids: form.teams,
    };
    if (editing.kind === "new") {
      draft.add(input);
    } else if (editing.kind === "added") {
      draft.change(editing.item.draft, input);
    } else {
      const { key: _, data_type: __, ...change } = input;
      draft.edit(editing.attribute.id, change);
    }
    onClose();
  };

  return (
    <Drawer isOpen onOpenChange={(open) => (open ? undefined : onClose())} size="lg">
      <DrawerHeader
        title={saved ? `Edit ${editing.attribute.label}` : "New conversation attribute"}
        description={
          saved ? "Its key and type stay as they are." : "Teammates and the AI fill it on every conversation."
        }
      />
      <DrawerBody className="flex flex-col gap-5 pb-4">
        <Input
          label="Name"
          value={form.label}
          onChange={(label) => set({ label })}
          placeholder="Issue type"
          maxLength={80}
          isRequired
          autoFocus
        />
        <Input
          label="Key"
          value={key}
          onChange={(next) => {
            setKeyTouched(true);
            set({ key: next });
          }}
          isDisabled={saved}
          maxLength={64}
          hint={`Workflows and views read it as conversation.ai.${key || "key"}.`}
        />
        <div className="flex flex-col gap-2">
          <span className="text-body-2-medium text-text-secondary">Type</span>
          <RadioGroup
            aria-label="Type"
            value={form.dataType}
            isDisabled={saved}
            onChange={(next) => set({ dataType: next as ConversationAttributeType })}
            className="grid grid-cols-1 gap-2 sm:grid-cols-2"
          >
            {(Object.keys(conversationAttributeTypes) as ConversationAttributeType[]).map((type) => (
              <RadioCard
                key={type}
                value={type}
                icon={typeIcons[type]}
                title={conversationAttributeTypes[type]}
                description={typeHints[type]}
              />
            ))}
          </RadioGroup>
        </div>
        {choosable.includes(form.dataType) ? (
          <OptionsEditor label="Options" value={form.options} onChange={(next) => set({ options: next })} />
        ) : null}
        {choosable.includes(form.dataType) && options.length > 0 ? (
          <OptionDescriptions
            options={options}
            value={form.descriptions}
            onChange={(descriptions) => set({ descriptions })}
          />
        ) : null}
        <Textarea
          label="Description"
          value={form.description}
          onChange={(description) => set({ description })}
          placeholder="The main reason the customer wrote in."
          hint="The AI reads this to decide the value, and teammates see it as a hint."
          rows={3}
          maxLength={500}
        />
        <div className="flex flex-col gap-2">
          <span className="text-body-2-medium text-text-secondary">Who fills it</span>
          <RadioGroup
            aria-label="Who fills it"
            value={filledBy}
            onChange={(next) => set({ filledBy: next as FilledBy })}
            className="flex flex-col gap-2"
          >
            <RadioCard
              value="both"
              isDisabled={!canAI}
              title={filledByLabels.both}
              description="The AI fills it as the customer writes. A teammate's value always wins."
            />
            <RadioCard value="teammates" title={filledByLabels.teammates} description="The AI leaves it alone." />
            <RadioCard
              value="ai"
              isDisabled={!canAI}
              title={filledByLabels.ai}
              description="Teammates see the value but can't change it."
            />
          </RadioGroup>
          {canAI ? null : (
            <span className="text-caption-1-regular text-text-tertiary">
              The AI can't fill {conversationAttributeTypes[form.dataType].toLowerCase()} values.
            </span>
          )}
        </div>
        <div className="flex items-start justify-between gap-4">
          <div className="flex min-w-0 flex-col">
            <span className="text-body-2-medium text-text-secondary">Required before closing</span>
            <span className="text-caption-1-regular text-text-tertiary">
              Teammates can't close a conversation while it's empty.
            </span>
          </div>
          <Switch
            aria-label="Required before closing"
            size="sm"
            isSelected={form.required}
            onChange={(required) => set({ required })}
          />
        </div>
        <div className="flex flex-col gap-2">
          <span className="text-body-2-medium text-text-secondary">Visible to</span>
          <MultiPick
            label="Teams"
            choices={teams.map((t) => [t.id, t.name] as const)}
            value={form.teams}
            allLabel="All teams"
            empty="All teams. Add teams to show it to some only."
            onChange={(next) => set({ teams: next })}
          />
          <span className="text-caption-1-regular text-text-tertiary">
            {form.teams.length === 0 ? "Every team sees it." : "Only these teams see it, in the inbox and the API."}
          </span>
        </div>
      </DrawerBody>
      <DrawerFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button disabled={!ready} onClick={done}>
          Done
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}

type Listed = Pick<
  ConversationAttribute,
  "key" | "label" | "data_type" | "filled_by" | "required_to_close" | "team_ids" | "status"
>;

/*
 * Chips are what sets an attribute apart in the list: a change waiting
 * for Save, required to close, and the teams that see it.
 */
function Chips({ attribute, teams, pending }: { attribute: Listed; teams: Map<string, string>; pending?: string }) {
  const teamNames = attribute.team_ids.map((id) => teams.get(id)).filter(Boolean);

  return (
    <>
      {pending ? (
        <Chip variant="caption" color="yellow">
          {pending}
        </Chip>
      ) : null}
      {attribute.required_to_close ? (
        <Chip variant="caption" color="blue">
          Required to close
        </Chip>
      ) : null}
      {teamNames.length > 0 ? (
        <Chip variant="caption" color="neutral" title={teamNames.join(", ")}>
          {teamNames.length === 1 ? teamNames[0] : `${teamNames.length} teams`}
        </Chip>
      ) : null}
    </>
  );
}

/*
 * Menu is what can be done with an attribute: edit it, archive or
 * restore it, or take a new one out of the draft.
 */
function Menu({
  attribute,
  onEdit,
  onStatus,
  onRemove,
}: {
  attribute: Listed;
  onEdit: () => void;
  onStatus?: () => void;
  onRemove?: () => void;
}) {
  const [open, setOpen] = useState(false);
  const label = `Manage ${attribute.label}`;
  const pick = (action?: () => void) => () => {
    setOpen(false);
    action?.();
  };
  const archiving = attribute.status === "active";

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label={label}
        className="flex size-8 items-center justify-center rounded-lg outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiMore2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
      </DropdownTrigger>
      <DropdownPopover aria-label={label} placement="bottom end">
        <DropdownItem onSelect={pick(onEdit)}>
          <span className="text-body-2-regular text-text-primary">Edit</span>
        </DropdownItem>
        {onStatus ? (
          <DropdownItem onSelect={pick(onStatus)}>
            <span className={cx("text-body-2-regular", archiving ? "text-text-error-primary" : "text-text-primary")}>
              {archiving ? "Archive" : "Restore"}
            </span>
          </DropdownItem>
        ) : null}
        {onRemove ? (
          <DropdownItem onSelect={pick(onRemove)}>
            <span className="text-body-2-regular text-text-error-primary">Remove</span>
          </DropdownItem>
        ) : null}
      </DropdownPopover>
    </Dropdown>
  );
}

/*
 * Line is one attribute in the list: its type's mark, its name, its type
 * and who fills it, its chips, and what can be done with it.
 */
function Line({
  attribute,
  teams,
  canManage,
  pending,
  onEdit,
  onStatus,
  onRemove,
}: {
  attribute: Listed;
  teams: Map<string, string>;
  canManage: boolean;
  pending?: string;
  onEdit: () => void;
  onStatus?: () => void;
  onRemove?: () => void;
}) {
  return (
    <Item
      mark={<Glyph icon={typeIcons[attribute.data_type]} />}
      title={attribute.label}
      detail={`${conversationAttributeTypes[attribute.data_type]} · ${filledByLabels[attribute.filled_by]}`}
    >
      <Chips attribute={attribute} teams={teams} pending={pending} />
      {canManage ? <Menu attribute={attribute} onEdit={onEdit} onStatus={onStatus} onRemove={onRemove} /> : null}
    </Item>
  );
}

/*
 * Movable is a row the team drags into a new place by its handle, or
 * moves with the arrow keys on the handle. While a row is dragged, a
 * line follows the pointer to show where it lands: above the row under
 * it when moving up, below it when moving down.
 */
function Movable({
  label,
  movable,
  dragging,
  line,
  onStart,
  onEnd,
  onOver,
  onDrop,
  onStep,
  children,
}: {
  label: string;
  movable: boolean;
  dragging: boolean;
  line: "before" | "after" | null;
  onStart: () => void;
  onEnd: () => void;
  onOver: () => void;
  onDrop: () => void;
  onStep: (by: number) => void;
  children: ReactNode;
}) {
  if (!movable) {
    return <li>{children}</li>;
  }

  return (
    <li
      className={cx("relative flex items-center", dragging && "opacity-50")}
      onDragOver={(event: DragEvent) => {
        event.preventDefault();
        event.dataTransfer.dropEffect = "move";
        onOver();
      }}
      onDrop={(event: DragEvent) => {
        event.preventDefault();
        onDrop();
      }}
    >
      {line ? (
        <span
          aria-hidden
          className={cx(
            "pointer-events-none absolute inset-x-0 z-10 h-0.5 rounded-full bg-accent-500",
            line === "before" ? "-top-px" : "-bottom-px",
          )}
        />
      ) : null}
      <button
        type="button"
        draggable
        aria-label={`Move ${label}`}
        onDragStart={(event) => {
          event.dataTransfer.effectAllowed = "move";
          event.dataTransfer.setData("text/plain", label);
          onStart();
        }}
        onDragEnd={onEnd}
        onKeyDown={(event) => {
          if (event.key === "ArrowUp" || event.key === "ArrowDown") {
            event.preventDefault();
            onStep(event.key === "ArrowUp" ? -1 : 1);
          }
        }}
        className="-ms-2 flex h-8 w-5 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring active:cursor-grabbing"
      >
        <RiDraggable aria-hidden className="size-4" />
      </button>
      <div className="min-w-0 flex-1">{children}</div>
    </li>
  );
}

function ArchiveDialog({
  attribute,
  onArchive,
  onClose,
}: {
  attribute: ConversationAttribute;
  onArchive: () => void;
  onClose: () => void;
}) {
  return (
    <Modal size="sm" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={`Archive ${attribute.label}?`}
        description="It leaves the details panel and nobody fills it any more. Values stay on conversations, so workflows, views and reports keep reading what it last held."
      />
      <ModalBody>{null}</ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="danger" onClick={onArchive}>
          Archive
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * listedOf is how a new attribute in the draft reads in the list.
 */
function listedOf(item: Added): Listed {
  return {
    key: item.key,
    label: item.label,
    data_type: item.data_type,
    filled_by: item.filled_by ?? "both",
    required_to_close: item.required_to_close ?? false,
    team_ids: item.team_ids ?? [],
    status: "active",
  };
}

/*
 * List is the attributes of one tab, the new ones of the draft first on
 * the active tab.
 */
function List({
  attributes,
  tab,
  draft,
  teams,
  canManage,
  onEdit,
  onArchive,
}: {
  attributes: ConversationAttribute[];
  tab: "active" | "archived";
  draft: Draft;
  teams: Map<string, string>;
  canManage: boolean;
  onEdit: (editing: Editing) => void;
  onArchive: (attribute: ConversationAttribute) => void;
}) {
  const [dragging, setDragging] = useState<string | null>(null);
  const [over, setOver] = useState<number | null>(null);
  const ordered = draft.order
    ? [...attributes].sort((a, b) => (draft.order ?? []).indexOf(a.id) - (draft.order ?? []).indexOf(b.id))
    : attributes;
  const listed = ordered
    .map((a) => ({ saved: a, now: { ...a, ...draft.edits[a.id] } }))
    .filter(({ now }) => now.status === tab);
  const added = tab === "active" ? draft.added : [];
  const movable = canManage && tab === "active" && listed.length > 1;
  const from = listed.findIndex(({ saved }) => saved.id === dragging);
  const lineAt = (index: number): "before" | "after" | null => {
    if (dragging === null || over !== index || from === index || from < 0) {
      return null;
    }
    return index < from ? "before" : "after";
  };
  const move = (id: string, to: number) => {
    const ids = ordered.map((a) => a.id);
    const from = ids.indexOf(id);
    const target = ordered.indexOf(listed[Math.max(0, Math.min(listed.length - 1, to))]?.saved);
    if (from < 0 || target < 0 || from === target) {
      return;
    }
    ids.splice(from, 1);
    ids.splice(target, 0, id);
    draft.reorder(ids);
  };

  return (
    <Section title={tab === "active" ? "Attributes" : "Archived"}>
      {added.map((item) => (
        <Line
          key={item.draft}
          attribute={listedOf(item)}
          teams={teams}
          canManage={canManage}
          pending="New"
          onEdit={() => onEdit({ kind: "added", item })}
          onRemove={() => draft.drop(item.draft)}
        />
      ))}
      <ul className="contents">
        {listed.map(({ saved, now }, index) => (
          <Movable
            key={saved.id}
            label={now.label}
            movable={movable}
            dragging={dragging === saved.id}
            line={lineAt(index)}
            onStart={() => setDragging(saved.id)}
            onEnd={() => {
              setDragging(null);
              setOver(null);
            }}
            onOver={() => setOver(index)}
            onDrop={() => {
              if (dragging) {
                move(dragging, index);
              }
              setDragging(null);
              setOver(null);
            }}
            onStep={(by) => move(saved.id, index + by)}
          >
            <Line
              attribute={now}
              teams={teams}
              canManage={canManage}
              pending={draft.edits[saved.id] ? "Edited" : undefined}
              onEdit={() => onEdit({ kind: "saved", attribute: saved })}
              onStatus={() => (now.status === "active" ? onArchive(saved) : draft.edit(saved.id, { status: "active" }))}
            />
          </Movable>
        ))}
      </ul>
      {listed.length === 0 && added.length === 0 ? (
        <Empty>
          {tab === "active" ? "None yet. Add one, such as the issue type or an order number." : "Nothing archived."}
        </Empty>
      ) : null}
    </Section>
  );
}

export function ConversationAttributesSettings() {
  const workspace = useAccount().workspace.id;
  const { data } = useQuery(conversationAttributesQuery(workspace));
  const teams = new Map((useQuery(teamsQuery(workspace)).data ?? []).map((t) => [t.id, t.name]));
  const manages = usePermission(workspace, "attributes:manage");
  const managesAI = usePermission(workspace, "ai_agent:manage");
  const canManage = manages || managesAI;
  const draft = useDraft(workspace);
  const [tab, setTab] = useState<"active" | "archived">("active");
  const [editing, setEditing] = useState<Editing | null>(null);
  const [archiving, setArchiving] = useState<ConversationAttribute | null>(null);
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  const activeCount =
    (data ?? []).filter((a) => ({ ...a, ...draft.edits[a.id] }).status === "active").length + draft.added.length;

  return (
    <SettingsPage
      title="Conversation attributes"
      description="What your team and the AI note on every conversation, for workflows, views and reports."
      actions={
        canManage ? (
          <Button size="small" variant="secondary" leadingIcon={RiAddLine} onClick={() => setEditing({ kind: "new" })}>
            New attribute
          </Button>
        ) : undefined
      }
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error ? failureText(draft.save.error) : null}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
    >
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-3 px-3">
          <SegmentedControl
            aria-label="Which attributes"
            selectedKeys={[tab]}
            onSelectionChange={(keys) => {
              const next = [...keys][0];
              if (next === "active" || next === "archived") {
                setTab(next);
              }
            }}
          >
            <SegmentedControlItem id={tabs.active}>Active</SegmentedControlItem>
            <SegmentedControlItem id={tabs.archived}>Archived</SegmentedControlItem>
          </SegmentedControl>
          <span className="text-body-2-regular text-text-tertiary">{activeCount} of 200 active</span>
        </div>
        {data ? (
          <List
            attributes={data}
            tab={tab}
            draft={draft}
            teams={teams}
            canManage={canManage}
            onEdit={setEditing}
            onArchive={setArchiving}
          />
        ) : (
          <Section title="Attributes" loading rows={4} />
        )}
      </div>
      {editing ? <AttributeDrawer editing={editing} draft={draft} onClose={() => setEditing(null)} /> : null}
      {archiving ? (
        <ArchiveDialog
          attribute={archiving}
          onClose={() => setArchiving(null)}
          onArchive={() => {
            draft.edit(archiving.id, { status: "archived" });
            setArchiving(null);
          }}
        />
      ) : null}
    </SettingsPage>
  );
}
