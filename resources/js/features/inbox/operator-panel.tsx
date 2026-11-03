/*
 * Operator is the team's own agent: it reads the case and talks it over
 * with teammates, and nothing said here reaches the customer. It opens
 * with what it has gathered about the conversation. Until Operator is
 * connected, a question gets an honest answer saying so.
 */
import { RiCloseLine, RiSparkling2Line } from "@remixicon/react";
import { AnimatePresence } from "motion/react";
import { useState } from "react";

import { AgentMessage } from "@/components/application/agent-chat/agent-chat-message";
import { AgentComposer } from "@/components/application/agent-chat/agent-composer";
import { Chip } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Panel, ScrollArea } from "@/features/shell/page";

import type { Conversation } from "./conversations";
import { Arrive } from "./motion";

interface Said {
  id: number;
  role: "user" | "assistant";
  text: string;
}

const notConnected = "I can't look into this yet: Operator connects once the inbox does.";

export function OperatorPanel({
  conversation,
  onClose,
  className,
}: {
  conversation: Conversation;
  onClose: () => void;
  className?: string;
}) {
  const [said, setSaid] = useState<Said[]>([{ id: 0, role: "assistant", text: conversation.briefing }]);
  const [draft, setDraft] = useState("");

  const ask = () => {
    const question = draft.trim();
    if (!question) {
      return;
    }
    setSaid((before) => [
      ...before,
      { id: before.length, role: "user", text: question },
      { id: before.length + 1, role: "assistant", text: notConnected },
    ]);
    setDraft("");
  };

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
      <ScrollArea>
        <ol aria-label="Operator chat" className="flex flex-col gap-5 px-4 pt-2 pb-6">
          <AnimatePresence initial={false}>
            {said.map((message) => (
              <Arrive key={message.id}>
                <AgentMessage role={message.role} text={message.text} />
              </Arrive>
            ))}
          </AnimatePresence>
        </ol>
      </ScrollArea>
      <div className="shrink-0 px-3 pb-3">
        <AgentComposer
          value={draft}
          onValueChange={setDraft}
          onSubmit={ask}
          onStop={() => undefined}
          busy={false}
          messageCount={said.length}
          placeholder="Ask Operator about this conversation"
          inputId="operator-composer-input"
          showStatus={false}
        />
      </div>
    </Panel>
  );
}
