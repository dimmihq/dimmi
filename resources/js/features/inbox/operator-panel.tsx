/*
 * Operator is the team's own agent: it reads the case and talks it over
 * with the teammate, and nothing said here reaches the customer. Each
 * teammate has their own thread per conversation. Asked for a brief, it
 * reads the conversation and says what the customer needs, what happened
 * and the next step; it does so on its own when the conversation opens
 * only if the workspace turned that on, since every brief spends tokens.
 * Replies it drafts go into the reply box on request, and changes it
 * proposes wait as cards to apply, decline or undo.
 */
import { RiCloseLine, RiSparkling2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { OperatorChat } from "@/features/operator/operator-chat";
import { conversationScope } from "@/features/operator/scope";
import { useOperatorChat } from "@/features/operator/use-operator-chat";
import { Panel } from "@/features/shell/page";
import { Reveal } from "@/features/shell/reveal";
import { briefConversation, conversationThreadQuery, operatorKeys, operatorSettingsQuery } from "@/lib/api/operator";

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
  const key = operatorKeys.conversation(workspace, conversation);
  const { data: thread } = useQuery(conversationThreadQuery(workspace, conversation));
  const { data: settings } = useQuery(operatorSettingsQuery(workspace));
  const chat = useOperatorChat(workspace, thread, key);
  const brief = () => void chat.run(null, (signal) => briefConversation(workspace, conversation, signal));
  const briefed = useRef(false);
  const autoBrief = settings?.enabled && settings.brief_on_open && thread?.messages.length === 0;

  useEffect(() => {
    if (autoBrief && !briefed.current) {
      briefed.current = true;
      void chat.run(null, (signal) => briefConversation(workspace, conversation, signal));
    }
  }, [autoBrief, chat, workspace, conversation]);

  return (
    <Panel label="Operator" className={className}>
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5 pt-[7px]">
        <span className="flex min-w-0 items-center gap-2">
          <RiSparkling2Line aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          <h2 className="truncate text-body-medium text-text-primary">Operator</h2>
          <Chip variant="caption" color="neutral" className="py-0">
            Internal
          </Chip>
        </span>
        <IconButton size="small" icon={RiCloseLine} aria-label="Close Operator" onClick={onClose} />
      </header>
      {settings && !settings.enabled ? (
        <p className="px-4 pt-2 text-body-2-regular text-text-secondary">
          Operator is switched off for this workspace. An admin can turn it on in Settings, Operator.
        </p>
      ) : (
        <OperatorChat
          thread={thread}
          chat={chat}
          empty={<Starting onBrief={brief} onAsk={chat.ask} busy={chat.busy || !thread} />}
          placeholder="Ask Operator about this conversation"
          inputId="operator-composer-input"
          scope={conversationScope}
        />
      )}
    </Panel>
  );
}
