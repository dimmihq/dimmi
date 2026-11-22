/*
 * The Operator panel every screen shows beside its work: the header with
 * the thread's title, past chats, a new chat and close; the notice when
 * the workspace switched Operator off; and the chat. An empty chat says
 * what Operator does here and offers suggestion cards, each one a thing
 * to ask with a line on what comes back. A screen brings its own threads
 * and suggestions; how the panel looks is decided here once.
 */
import { RiChatNewLine, RiCloseLine, RiHistoryLine, RiSparkling2Line } from "@remixicon/react";
import type { ComponentProps, ComponentType } from "react";
import { Button as AriaButton } from "react-aria-components";

import { Chip } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { ago } from "@/features/inbox/conversations";
import { Panel } from "@/features/shell/page";
import { Reveal } from "@/features/shell/reveal";
import type { OperatorThread, OperatorThreadView } from "@/lib/api/operator";
import { cx } from "@/utils/cx";

import { OperatorChat } from "./operator-chat";
import type { useOperatorChat } from "./use-operator-chat";

/*
 * Suggestion is one thing to ask Operator from an empty chat.
 */
export interface Suggestion {
  icon: ComponentType<{ className?: string }>;
  title: string;
  /* What Operator does when it is picked, in one line. */
  detail: string;
  onPick: () => void;
}

function Starting({
  heading,
  blurb,
  suggestions,
  busy,
}: {
  heading: string;
  blurb: string;
  suggestions: Suggestion[];
  busy: boolean;
}) {
  return (
    <Reveal>
      <div className="flex flex-col items-center gap-4 py-4 text-center">
        <span className="flex size-10 items-center justify-center rounded-full bg-background-primary-default shadow-card">
          <RiSparkling2Line aria-hidden className="size-5 text-foreground-icon-secondary" />
        </span>
        <div className="flex flex-col gap-1">
          <h3 className="text-body-medium text-text-primary">{heading}</h3>
          <p className="text-body-2-regular text-text-secondary">{blurb}</p>
        </div>
        <div className="flex w-full flex-col gap-1.5">
          {suggestions.map((suggestion) => (
            <AriaButton
              key={suggestion.title}
              isDisabled={busy}
              onPress={suggestion.onPick}
              className={cx(
                "flex w-full cursor-pointer items-start gap-3 rounded-2xl bg-background-primary-default p-3 text-start shadow-card outline-none",
                "transition-colors duration-150 hover:bg-background-primary-hover",
                "focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-default disabled:opacity-60",
              )}
            >
              <suggestion.icon aria-hidden className="mt-0.5 size-4 shrink-0 text-foreground-icon-secondary" />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="text-body-2-medium text-text-primary">{suggestion.title}</span>
                <span className="text-body-2-regular text-text-secondary">{suggestion.detail}</span>
              </span>
            </AriaButton>
          ))}
        </div>
      </div>
    </Reveal>
  );
}

/*
 * History lists the teammate's past threads, the one shown marked, and
 * opens the one picked.
 */
function History({
  threads,
  current,
  onPick,
}: {
  threads: OperatorThread[];
  current: string | undefined;
  onPick: (id: string) => void;
}) {
  return (
    <Dropdown>
      <DropdownTrigger
        aria-label="Past chats"
        isDisabled={threads.length === 0}
        className="flex size-8 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-default disabled:opacity-40"
      >
        <RiHistoryLine aria-hidden className="size-4" />
      </DropdownTrigger>
      <DropdownPopover aria-label="Past chats" placement="bottom end" className="w-72">
        {threads.map((thread) => (
          <DropdownItem key={thread.id} selected={thread.id === current} onSelect={() => onPick(thread.id)}>
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-body-2-medium text-text-primary">{thread.title || "New chat"}</span>
              <span className="text-caption-1-regular text-text-tertiary">
                {ago(thread.last_message_at ?? thread.created_at)}
              </span>
            </span>
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
  );
}

type ChatProps = ComponentProps<typeof OperatorChat>;

export function OperatorSidePanel({
  thread,
  chat,
  past,
  enabled,
  starting,
  heading,
  blurb,
  suggestions,
  placeholder,
  inputId,
  scope,
  onPick,
  onNew,
  onClose,
  className,
}: {
  thread: OperatorThreadView | undefined;
  chat: ReturnType<typeof useOperatorChat>;
  /* The teammate's earlier threads here, for the past chats menu. */
  past: OperatorThread[];
  /* false once the workspace is known to have Operator switched off. */
  enabled: boolean;
  /* A new chat is being started. */
  starting: boolean;
  heading: string;
  blurb: string;
  suggestions: Suggestion[];
  placeholder: string;
  inputId: string;
  scope: ChatProps["scope"];
  onPick: (thread: string) => void;
  onNew: () => void;
  onClose: () => void;
  className?: string;
}) {
  return (
    <Panel label="Operator" className={className}>
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5 pt-[7px]">
        <span className="flex min-w-0 items-center gap-2">
          <RiSparkling2Line aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          <h2 className="truncate text-body-medium text-text-primary">{thread?.title || "Operator"}</h2>
          <Chip variant="caption" color="neutral" className="shrink-0 py-0">
            Internal
          </Chip>
        </span>
        <span className="flex shrink-0 items-center gap-0.5">
          <History threads={past} current={thread?.id} onPick={onPick} />
          <IconButton
            size="small"
            icon={RiChatNewLine}
            aria-label="New chat"
            disabled={chat.busy || starting || thread?.messages.length === 0}
            onClick={onNew}
          />
          <IconButton size="small" icon={RiCloseLine} aria-label="Close Operator" onClick={onClose} />
        </span>
      </header>
      {enabled ? (
        <OperatorChat
          key={thread?.id ?? "opening"}
          thread={thread}
          chat={chat}
          empty={<Starting heading={heading} blurb={blurb} suggestions={suggestions} busy={chat.busy || !thread} />}
          placeholder={placeholder}
          inputId={inputId}
          scope={scope}
        />
      ) : (
        <p className="px-4 pt-2 text-body-2-regular text-text-secondary">
          Operator is switched off for this workspace. An admin can turn it on in Settings, Operator.
        </p>
      )}
    </Panel>
  );
}
