/*
 * The preview composer's add menu, built like the inbox composer's: the
 * round plus turns into a close and a panel rises above it. It lists
 * what a preview can start from, a customer to test as and a real
 * conversation to replay, and each opens a searchable list in the same
 * panel. Typing narrows the list, Enter takes the first match, the down
 * arrow moves into the list, and the back arrow returns to the menu.
 * The panel always reopens on the menu.
 */
import {
  RiAddLine,
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiChat3Line,
  RiSearchLine,
  RiSparkling2Line,
  RiUserLine,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { type ComponentType, type ReactNode, useDeferredValue, useRef, useState } from "react";
import { Button as AriaButton, Dialog, DialogTrigger, ListBox, ListBoxItem, Popover } from "react-aria-components";

import { Avatar } from "@/components/base/avatar/avatar";
import { MENU_POPOVER_SURFACE } from "@/components/base/dropdown/menu-styles";
import { Input } from "@/components/base/input/input";
import { customerName, subjectOf } from "@/features/inbox/conversations";
import { Swap } from "@/features/inbox/motion";
import { avatarColor } from "@/features/shell/avatar-color";
import { type Contact, peoplePageQuery } from "@/lib/api/customers";
import { type Conversation, conversationsQuery } from "@/lib/api/inbox";
import type { SimPersona } from "@/lib/api/procedures";
import { cx } from "@/utils/cx";

import { MadeUpPanel } from "./made-up-panel";

type Finding = "customer" | "made-up" | "conversation";

const row =
  "flex w-full cursor-pointer items-center gap-2 rounded-2lg px-2 py-1.5 text-start outline-none transition-colors hover:bg-background-primary-hover focus-visible:bg-background-primary-hover";

function nameOf(person: Contact): string {
  return person.name || person.email || "Unnamed";
}

function Row({
  icon: Icon,
  label,
  description,
  onPress,
}: {
  icon: ComponentType<{ className?: string }>;
  label: string;
  description: string;
  onPress: () => void;
}) {
  return (
    <button type="button" onClick={onPress} className={cx(row, "gap-3 py-2")}>
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-background-secondary-default">
        <Icon aria-hidden className="size-4 text-foreground-icon-secondary" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-medium text-text-primary">{label}</span>
        <span className="truncate text-body-2-regular text-text-secondary">{description}</span>
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

/*
 * Found is a searchable list inside the panel: a back arrow with the
 * list's name, the search field, and the matches.
 */
function Found<T extends { id: string }>({
  label,
  placeholder,
  items,
  loading,
  empty,
  textOf,
  onSearch,
  onBack,
  onPick,
  children,
}: {
  label: string;
  placeholder: string;
  items: T[];
  loading: boolean;
  empty: string;
  textOf: (item: T) => string;
  onSearch: (query: string) => void;
  onBack: () => void;
  onPick: (item: T) => void;
  children: (item: T) => ReactNode;
}) {
  const [query, setQuery] = useState("");
  const results = useRef<HTMLDivElement>(null);
  const nothing = loading ? "Loading…" : empty;

  return (
    <div className="flex h-full min-h-0 flex-col gap-2">
      <div className="flex shrink-0 items-center gap-1">
        <button
          type="button"
          aria-label="Back"
          onClick={onBack}
          className="flex size-7 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-5" />
        </button>
        <span className="text-body-medium text-text-primary">{label}</span>
      </div>
      <Input
        aria-label={placeholder}
        placeholder={placeholder}
        leadingIcon={RiSearchLine}
        fieldClassName="bg-background-secondary-default"
        value={query}
        onChange={(next) => {
          setQuery(next);
          onSearch(next);
        }}
        onKeyDown={(event) => {
          const first = items[0];
          if (event.key === "Enter" && first) {
            onPick(first);
          }
          if (event.key === "ArrowDown") {
            event.preventDefault();
            results.current?.querySelector<HTMLElement>("[role=option]")?.focus();
          }
        }}
        autoFocus
      />
      <ListBox
        ref={results}
        aria-label={label}
        items={items}
        onAction={(id) => {
          const item = items.find((candidate) => candidate.id === id);
          if (item) {
            onPick(item);
          }
        }}
        renderEmptyState={() => <p className="px-2 py-3 text-body-regular text-text-tertiary">{nothing}</p>}
        className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto outline-none"
      >
        {(item) => (
          <ListBoxItem
            id={item.id}
            textValue={textOf(item)}
            className={cx(row, "data-[focused]:bg-background-primary-hover")}
          >
            {children(item)}
          </ListBoxItem>
        )}
      </ListBox>
    </div>
  );
}

function Customers({
  workspace,
  onBack,
  onPick,
}: {
  workspace: string;
  onBack: () => void;
  onPick: (person: Contact) => void;
}) {
  const [search, setSearch] = useState("");
  const q = useDeferredValue(search).trim();
  const people = useQuery({ ...peoplePageQuery(workspace, { q: q || undefined }, 1), enabled: workspace !== "" });

  return (
    <Found
      label="Test as a customer"
      placeholder="Search customers by name or email"
      items={people.data?.items ?? []}
      loading={people.isLoading}
      empty="Nobody matches"
      textOf={nameOf}
      onSearch={setSearch}
      onBack={onBack}
      onPick={onPick}
    >
      {(person) => (
        <>
          <Avatar size="xs" color={avatarColor(nameOf(person))} initials={nameOf(person).charAt(0)} />
          <span className="flex min-w-0 flex-col">
            <span className="truncate text-body-medium text-text-primary">{nameOf(person)}</span>
            {person.name && person.email ? (
              <span className="truncate text-body-2-regular text-text-secondary">{person.email}</span>
            ) : null}
          </span>
        </>
      )}
    </Found>
  );
}

function Conversations({
  workspace,
  onBack,
  onPick,
}: {
  workspace: string;
  onBack: () => void;
  onPick: (conversation: Conversation) => void;
}) {
  const [search, setSearch] = useState("");
  const q = useDeferredValue(search).trim();
  const found = useQuery({
    ...conversationsQuery(workspace, { view: "all", status: "any", search: q, limit: 20 }),
    enabled: workspace !== "",
  });

  return (
    <Found
      label="Replay a conversation"
      placeholder="Search by words, customer or number"
      items={found.data ?? []}
      loading={found.isLoading}
      empty="No conversation matches"
      textOf={(c) => `#${c.number} ${subjectOf(c)}`}
      onSearch={setSearch}
      onBack={onBack}
      onPick={onPick}
    >
      {(c) => (
        <>
          <span className="w-10 shrink-0 text-body-2-medium text-text-tertiary tabular-nums">#{c.number}</span>
          <span className="flex min-w-0 flex-col">
            <span className="truncate text-body-medium text-text-primary">{subjectOf(c)}</span>
            <span className="truncate text-body-2-regular text-text-secondary">{customerName(c.contact)}</span>
          </span>
        </>
      )}
    </Found>
  );
}

/*
 * PreviewAddMenu is the plus of the preview composer. A conversation
 * can only be replayed before the chat has begun. With onContact, a
 * customer picked from the list is handed over as the contact itself.
 */
export function PreviewAddMenu({
  workspace,
  replayable,
  onCustomer,
  onContact,
  onConversation,
}: {
  workspace: string;
  replayable: boolean;
  onCustomer: (persona: SimPersona) => void;
  onContact?: (person: Contact) => void;
  onConversation: (conversation: Conversation) => void;
}) {
  const [open, setOpen] = useState(false);
  const [finding, setFinding] = useState<Finding | null>(null);

  const toggle = (next: boolean) => {
    setOpen(next);
    if (!next) {
      setFinding(null);
    }
  };

  return (
    <DialogTrigger isOpen={open} onOpenChange={toggle}>
      <AriaButton
        aria-label="Add to the preview"
        className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-composer-panel-add-background p-2 text-foreground-icon-primary outline-none transition-colors duration-150 hover:bg-composer-panel-add-hover-background focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiAddLine
          aria-hidden
          className={cx(
            "size-5 transition-[rotate] duration-200 ease-out motion-reduce:transition-none",
            open && "rotate-45",
          )}
        />
      </AriaButton>
      <Popover
        placement="top start"
        offset={8}
        className={cx(MENU_POPOVER_SURFACE, "h-[420px] max-h-[70vh] w-[361px] overflow-hidden p-2")}
      >
        <Dialog aria-label="Add to the preview" className="relative h-full overflow-hidden outline-none">
          <Swap view={finding ?? "menu"} forward={finding !== null} className="flex h-full flex-col">
            {finding === "customer" ? (
              <Customers
                workspace={workspace}
                onBack={() => setFinding(null)}
                onPick={(person) => {
                  if (onContact) {
                    onContact(person);
                    toggle(false);
                    return;
                  }
                  onCustomer({
                    verified: true,
                    channel: "widget",
                    name: person.name ?? undefined,
                    email: person.email ?? undefined,
                    external_id: person.external_id ?? undefined,
                    attributes: { ...person.custom_attributes },
                  });
                  toggle(false);
                }}
              />
            ) : finding === "made-up" ? (
              <MadeUpPanel
                workspace={workspace}
                onBack={() => setFinding(null)}
                onPick={(persona) => {
                  onCustomer(persona);
                  toggle(false);
                }}
              />
            ) : finding === "conversation" ? (
              <Conversations
                workspace={workspace}
                onBack={() => setFinding(null)}
                onPick={(conversation) => {
                  onConversation(conversation);
                  toggle(false);
                }}
              />
            ) : (
              <div className="flex w-full flex-col gap-1.5 pt-1">
                <span className="ps-2 text-body-medium text-text-secondary">Add</span>
                <div className="flex w-full flex-col gap-1">
                  <Row
                    icon={RiUserLine}
                    label="Customer"
                    description="Test as one of your customers"
                    onPress={() => setFinding("customer")}
                  />
                  <Row
                    icon={RiSparkling2Line}
                    label="Made-up customer"
                    description="Set their plan and other details"
                    onPress={() => setFinding("made-up")}
                  />
                  {replayable ? (
                    <Row
                      icon={RiChat3Line}
                      label="Conversation"
                      description="Replay what a customer wrote"
                      onPress={() => setFinding("conversation")}
                    />
                  ) : null}
                </div>
              </div>
            )}
          </Swap>
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
