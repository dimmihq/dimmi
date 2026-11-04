/*
 * The command menu, opened with Cmd+K from anywhere in the dashboard: the
 * moves that apply where the person is, the places to go, and, once they
 * type, what the quick search finds: the conversation a number names,
 * conversations and customers. Every row is reachable by keyboard and
 * shows its shortcut. Moves the person ran recently come first.
 */
import { RiChat3Line, RiSearchLine, RiUserLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useDeferredValue, useEffect, useState } from "react";
import {
  Input as AriaInput,
  Autocomplete,
  Header,
  Menu,
  MenuItem,
  MenuSection,
  SearchField,
  useFilter,
} from "react-aria-components";

import { Kbd } from "@/components/base/kbd/kbd";
import { Modal } from "@/components/base/modal/modal";
import { allActions } from "@/features/inbox/actions/all-actions";
import { type ActionContext, available, type InboxAction } from "@/features/inbox/actions/registry";
import { searchQuery } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { recentFirst, remember } from "./recent";

const searchDelay = 150;

const item = cx(
  "flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 text-body-2-medium text-text-primary outline-none",
  "data-[focused]:bg-background-secondary-hover",
);

function useDebounced(value: string, delay: number): string {
  const [settled, setSettled] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return settled;
}

function ActionRow({ action }: { action: InboxAction }) {
  const Icon = action.icon;

  return (
    <>
      {Icon ? <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" /> : null}
      <span className="min-w-0 flex-1 truncate">{action.label}</span>
      {action.keys ? (
        <span className="flex shrink-0 gap-1">
          {action.keys.map((key) => (
            <Kbd key={key}>{key}</Kbd>
          ))}
        </span>
      ) : null}
    </>
  );
}

export function CommandMenu({
  isOpen,
  onOpenChange,
  context,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  context: ActionContext;
}) {
  const navigate = useNavigate();
  const [text, setText] = useState("");
  const typed = useDebounced(useDeferredValue(text).trim(), searchDelay);
  const { contains } = useFilter({ sensitivity: "base" });
  const { data: found } = useQuery({
    ...searchQuery(context.workspace, typed),
    enabled: isOpen && typed.length > 0 && context.workspace !== "",
  });
  const moves = recentFirst(available(allActions, context)).filter((a) => contains(a.label, text));
  const close = () => {
    setText("");
    onOpenChange(false);
  };
  const run = (key: string) => {
    const [kind, id] = key.split(":", 2);
    const move = allActions.find((a) => a.id === id);
    close();
    if (kind === "action" && move) {
      remember(move.id);
      move.run(context);
    } else if (kind === "conversation" && id) {
      void navigate({ to: "/d/inbox/$view/$conversation", params: { view: "all", conversation: id } });
    } else if (kind === "contact") {
      void navigate({ to: "/d/customers" });
    }
  };
  const hits = found?.conversations ?? [];
  const jump = found?.jump ?? null;

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(next) => (next ? onOpenChange(true) : close())}
      aria-label="Command menu"
      overlayClassName="items-start pt-[12vh]"
      className="p-2"
    >
      <Autocomplete inputValue={text} onInputChange={setText} filter={() => true}>
        <SearchField
          aria-label="Search for a command or conversation"
          autoFocus
          className="flex items-center gap-2 px-2.5 pb-2"
        >
          <RiSearchLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
          <AriaInput
            placeholder="Type a command, #number or words"
            className="h-9 min-w-0 flex-1 bg-transparent text-body-regular text-text-primary outline-none placeholder:text-text-placeholder"
          />
        </SearchField>
        <Menu
          aria-label="Commands"
          onAction={(key) => run(String(key))}
          className="flex max-h-[60vh] flex-col gap-1 overflow-y-auto outline-none"
          renderEmptyState={() => (
            <p className="px-2.5 py-6 text-center text-body-2-regular text-text-tertiary">Nothing found</p>
          )}
        >
          {jump ? (
            <MenuSection>
              <Header className="px-2.5 pt-1 text-caption-1-medium text-text-tertiary">Jump to</Header>
              <MenuItem id={`conversation:${jump.id}`} textValue={`#${jump.number}`} className={item}>
                <RiChat3Line aria-hidden className="size-4 text-foreground-icon-secondary" />
                <span className="truncate">
                  #{jump.number} {jump.subject ?? ""}
                </span>
              </MenuItem>
            </MenuSection>
          ) : null}
          {moves.length > 0 ? (
            <MenuSection>
              <Header className="px-2.5 pt-1 text-caption-1-medium text-text-tertiary">Actions</Header>
              {moves.map((action) => (
                <MenuItem key={action.id} id={`action:${action.id}`} textValue={action.label} className={item}>
                  <ActionRow action={action} />
                </MenuItem>
              ))}
            </MenuSection>
          ) : null}
          {hits.length > 0 ? (
            <MenuSection>
              <Header className="px-2.5 pt-1 text-caption-1-medium text-text-tertiary">Conversations</Header>
              {hits.map((hit) => (
                <MenuItem
                  key={hit.conversation.id}
                  id={`conversation:${hit.conversation.id}`}
                  textValue={hit.conversation.subject ?? `#${hit.conversation.number}`}
                  className={item}
                >
                  <RiChat3Line aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                  <span className="min-w-0 flex-1 truncate">
                    {hit.conversation.subject ?? `#${hit.conversation.number}`}
                  </span>
                  <span className="shrink-0 text-caption-1-medium text-text-tertiary">
                    {hit.contact.name ?? hit.contact.email}
                  </span>
                </MenuItem>
              ))}
            </MenuSection>
          ) : null}
          {(found?.contacts.length ?? 0) > 0 ? (
            <MenuSection>
              <Header className="px-2.5 pt-1 text-caption-1-medium text-text-tertiary">Customers</Header>
              {(found?.contacts ?? []).map((contact) => (
                <MenuItem
                  key={contact.id}
                  id={`contact:${contact.id}`}
                  textValue={contact.name ?? contact.email ?? "Customer"}
                  className={item}
                >
                  <RiUserLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                  <span className="min-w-0 flex-1 truncate">{contact.name ?? contact.email}</span>
                  <span className="shrink-0 text-caption-1-medium text-text-tertiary">{contact.email}</span>
                </MenuItem>
              ))}
            </MenuSection>
          ) : null}
        </Menu>
      </Autocomplete>
    </Modal>
  );
}
