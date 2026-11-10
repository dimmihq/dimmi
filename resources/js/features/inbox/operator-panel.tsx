/*
 * Operator is the team's own agent: it reads the case and talks it over
 * with the teammate, and nothing said here reaches the customer. Each
 * teammate keeps their own threads about a conversation: the latest
 * opens first, past ones are a click away, and a new one starts when the
 * subject changes. Asked for a brief, it reads the conversation and says
 * what the customer needs, what happened and the next step, and keeps
 * the brief on the conversation as a note for the whole team; it briefs
 * on its own when the conversation opens only if the workspace turned
 * that on, since every brief spends tokens, and keeps that one private.
 * Replies it drafts go into the reply box on request, and changes it
 * proposes wait as cards to apply, decline or undo.
 */
import { RiChatNewLine, RiCloseLine, RiHistoryLine, RiSparkling2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { ago } from "@/features/inbox/conversations";
import { OperatorChat } from "@/features/operator/operator-chat";
import { conversationScope } from "@/features/operator/scope";
import { useOperatorChat } from "@/features/operator/use-operator-chat";
import { Panel } from "@/features/shell/page";
import { Reveal } from "@/features/shell/reveal";
import {
  briefConversation,
  conversationThreadQuery,
  conversationThreadsQuery,
  type OperatorThread,
  operatorKeys,
  operatorSettingsQuery,
  operatorThreadQuery,
  startConversationThread,
} from "@/lib/api/operator";

const starters = ["Draft a reply", "Suggest tags and an owner", "Has this customer written before?"];

function Starting({ onBrief, onAsk, busy }: { onBrief: () => void; onAsk: (q: string) => void; busy: boolean }) {
  return (
    <Reveal>
      <div className="flex flex-col items-center gap-4 py-4 text-center">
        <span className="flex size-10 items-center justify-center rounded-full bg-background-primary-default shadow-card">
          <RiSparkling2Line aria-hidden className="size-5 text-foreground-icon-secondary" />
        </span>
        <div className="flex flex-col gap-1">
          <h3 className="text-body-medium text-text-primary">Catch up on this conversation</h3>
          <p className="text-body-2-regular text-text-secondary">
            Operator reads it and briefs you, drafts replies and proposes changes for you to apply.
          </p>
        </div>
        <Button size="small" leadingIcon={RiSparkling2Line} disabled={busy} onClick={onBrief}>
          Brief me
        </Button>
        <div className="flex flex-wrap justify-center gap-1.5">
          {starters.map((starter) => (
            <Button key={starter} size="small" variant="secondary" disabled={busy} onClick={() => onAsk(starter)}>
              {starter}
            </Button>
          ))}
        </div>
      </div>
    </Reveal>
  );
}

/*
 * History lists the teammate's past threads about the conversation, the
 * one shown marked, and opens the one picked.
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

export function OperatorPanel({
  workspace,
  conversation,
  onClose,
  className,
}: {
  workspace: string;
  conversation: string;
  onClose: () => void;
  className?: string;
}) {
  const client = useQueryClient();
  const [picked, setPicked] = useState<string | null>(null);
  const latest = useQuery({ ...conversationThreadQuery(workspace, conversation), enabled: picked === null });
  const chosen = useQuery({ ...operatorThreadQuery(workspace, picked ?? ""), enabled: picked !== null });
  const thread = picked === null ? latest.data : chosen.data;
  const key =
    picked === null ? operatorKeys.conversation(workspace, conversation) : operatorKeys.thread(workspace, picked);
  const { data: past = [] } = useQuery(conversationThreadsQuery(workspace, conversation));
  const { data: settings } = useQuery(operatorSettingsQuery(workspace));
  const chat = useOperatorChat(workspace, thread, key);
  const refreshList = () => client.invalidateQueries({ queryKey: operatorKeys.conversation(workspace, conversation) });
  const brief = (attach: boolean) =>
    void chat
      .run(null, (signal) => briefConversation(workspace, conversation, { thread: thread?.id, attach, signal }))
      .then(refreshList);
  const fresh = useMutation({
    mutationFn: () => startConversationThread(workspace, conversation),
    onSuccess: (started) => {
      client.setQueryData(operatorKeys.thread(workspace, started.id), started);
      setPicked(started.id);
    },
  });
  const briefed = useRef(false);
  const autoBrief = settings?.enabled && settings.brief_on_open && picked === null && thread?.messages.length === 0;

  useEffect(() => {
    if (autoBrief && !briefed.current) {
      briefed.current = true;
      void chat.run(null, (signal) => briefConversation(workspace, conversation, { signal }));
    }
  }, [autoBrief, chat, workspace, conversation]);

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
          <History threads={past} current={thread?.id} onPick={setPicked} />
          <IconButton
            size="small"
            icon={RiChatNewLine}
            aria-label="New chat"
            disabled={chat.busy || fresh.isPending || thread?.messages.length === 0}
            onClick={() => fresh.mutate()}
          />
          <IconButton size="small" icon={RiCloseLine} aria-label="Close Operator" onClick={onClose} />
        </span>
      </header>
      {settings && !settings.enabled ? (
        <p className="px-4 pt-2 text-body-2-regular text-text-secondary">
          Operator is switched off for this workspace. An admin can turn it on in Settings, Operator.
        </p>
      ) : (
        <OperatorChat
          key={thread?.id ?? "opening"}
          thread={thread}
          chat={chat}
          empty={<Starting onBrief={() => brief(true)} onAsk={chat.ask} busy={chat.busy || !thread} />}
          placeholder="Ask Operator about this conversation"
          inputId="operator-composer-input"
          scope={conversationScope}
        />
      )}
    </Panel>
  );
}
