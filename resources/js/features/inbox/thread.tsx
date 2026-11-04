/*
 * The conversation itself, laid out like BoardUI's chat: the customer's
 * messages on the left on the tertiary surface, the team's side on the
 * right, Dimmi's replies tagged as the AI agent, and events as centred
 * pills. What Operator adds is a full-width card across the thread, so
 * the team's own agent never reads as part of the exchange with the
 * customer. Teammates' notes sit on the team's side with a dashed edge
 * and a lock. A message the workspace's redaction rules changed says how
 * many items they removed. Every message's Markdown is rendered. Below the thread sits the composer; R and N open it for a
 * reply or a note from anywhere, and O opens or closes Operator.
 */
import { RiLock2Line, RiSparkling2Line } from "@remixicon/react";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { DimmiIconSmall } from "@/features/brand/logo";
import { avatarColor } from "@/features/shell/avatar-color";
import { ScrollArea } from "@/features/shell/page";
import { cx } from "@/utils/cx";

import { Composer, type Send } from "./composer";
import type { ThreadEntry } from "./conversations";
import { LinkPolicyProvider } from "./links/link-policy";
import { MessageBody } from "./message-body";
import type { Mode } from "./modes";
import type { Report } from "./use-typing-report";

type Said = Extract<ThreadEntry, { author: string }>;

const bubble = "w-fit max-w-[min(520px,calc(100%-24px))] rounded-[var(--radius-2-5xl)] p-3";

function Pill({ children }: { children: ReactNode }) {
  return (
    <p className="flex justify-center px-6">
      <span className="rounded-full bg-background-tertiary-default px-[7px] py-1 text-center text-body-2-medium text-text-secondary">
        {children}
      </span>
    </p>
  );
}

function Mark({ entry }: { entry: Said }) {
  if (entry.kind === "dimmi") {
    return <DimmiIconSmall className="size-4 text-text-primary" />;
  }
  if (entry.kind === "note") {
    return <RiLock2Line aria-hidden className="size-4 text-foreground-icon-tertiary" />;
  }

  return <Avatar size="xs" color={avatarColor(entry.author)} initials={entry.author.charAt(0)} />;
}

function Tag({ entry }: { entry: Said }) {
  return (
    <span className="my-1 flex items-center gap-1.5 text-body-2-medium text-text-secondary">
      <Mark entry={entry} />
      {entry.author}
      {entry.kind === "dimmi" ? (
        <Chip variant="caption" color="neutral" className="py-0">
          AI agent
        </Chip>
      ) : null}
      <span className="text-body-2-regular text-text-tertiary">{entry.at}</span>
      {entry.redacted ? (
        <span className="text-body-2-regular text-text-tertiary">
          · {entry.redacted === 1 ? "1 item redacted" : `${entry.redacted} items redacted`}
        </span>
      ) : null}
    </span>
  );
}

function OperatorCard({ entry, customer }: { entry: Said; customer: string }) {
  return (
    <article
      aria-label={`Internal note from ${entry.author}`}
      className="mx-3 flex flex-col gap-2 rounded-[var(--radius-2-5xl)] bg-background-primary-default p-3 shadow-card"
    >
      <header className="flex items-center gap-1.5 text-body-2-medium text-text-primary">
        <RiSparkling2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
        {entry.author}
        <Chip variant="caption" color="soft" className="py-0">
          Internal
        </Chip>
        <span className="ms-auto text-body-2-regular text-text-tertiary">{entry.at}</span>
      </header>
      <MessageBody text={entry.text} className="text-body-regular text-text-primary" />
      <p className="flex items-center gap-1 text-body-2-regular text-text-tertiary">
        <RiLock2Line aria-hidden className="size-3.5" />
        Not visible to {customer}
      </p>
    </article>
  );
}

function Entry({ entry, customer }: { entry: ThreadEntry; customer: string }) {
  if (entry.kind === "event") {
    return <Pill>{entry.text}</Pill>;
  }
  if (entry.kind === "note" && entry.author === "Operator") {
    return <OperatorCard entry={entry} customer={customer} />;
  }

  const ours = entry.kind !== "customer";
  const note = entry.kind === "note";

  return (
    <article
      aria-label={note ? `Internal note from ${entry.author}` : `Message from ${entry.author}`}
      className={cx("flex flex-col px-3", ours ? "items-end" : "items-start")}
    >
      <div
        className={cx(
          bubble,
          "text-body-regular text-text-primary",
          !ours && "bg-background-tertiary-default",
          ours && !note && "bg-background-primary-default shadow-card",
          note && "border border-dashed border-border-button-hover",
        )}
      >
        <MessageBody text={entry.text} fromCustomer={!ours} />
        {note ? <span className="sr-only"> Not visible to {customer}.</span> : null}
      </div>
      <Tag entry={entry} />
      {note ? (
        <span aria-hidden className="text-body-2-regular text-text-tertiary">
          Not visible to {customer}
        </span>
      ) : null}
    </article>
  );
}

/*
 * useComposerKeys switches the composer, or opens Operator, from the
 * keyboard: R, N and O, unless the person is typing somewhere.
 */
function useComposerKeys(keys: Record<string, () => void>) {
  useEffect(() => {
    const press = (event: KeyboardEvent) => {
      const target = event.target;
      const typing = target instanceof Element && target.closest("input, textarea, select, [contenteditable='true']");
      const action = keys[event.key.toLowerCase()];
      if (typing || event.metaKey || event.ctrlKey || event.altKey || !action) {
        return;
      }
      event.preventDefault();
      action();
    };
    document.addEventListener("keydown", press);

    return () => document.removeEventListener("keydown", press);
  }, [keys]);
}

export function Thread({
  entries,
  customer,
  channel,
  channelType,
  onSend,
  onAskOperator,
  onTyping,
  status,
}: {
  entries: ThreadEntry[];
  customer: string;
  channel: string;
  channelType?: string;
  onSend: Send;
  onAskOperator: () => void;
  onTyping: Report;
  status?: ReactNode;
}) {
  const [mode, setMode] = useState<Mode | null>(null);
  const [focusKey, setFocusKey] = useState(0);
  const ask = useRef(onAskOperator);
  useEffect(() => {
    ask.current = onAskOperator;
  }, [onAskOperator]);
  const [keys] = useState(() => {
    const write = (next: Mode) => {
      setMode(next);
      setFocusKey((before) => before + 1);
    };

    return { r: () => write("reply"), n: () => write("note"), o: () => ask.current() };
  });
  useComposerKeys(keys);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <ScrollArea className="pt-2 pb-4">
        <LinkPolicyProvider channelType={channelType}>
          <ol className="mx-auto flex max-w-3xl flex-col gap-3">
            {entries.map((entry) => (
              <li key={entry.id}>
                <Entry entry={entry} customer={customer} />
              </li>
            ))}
          </ol>
        </LinkPolicyProvider>
      </ScrollArea>
      <div className="shrink-0 px-3 pb-3">
        {status ? <div className="mx-auto w-full max-w-3xl px-4 pb-1.5">{status}</div> : null}
        <Composer
          mode={mode}
          focusKey={focusKey}
          customer={customer}
          channel={channel}
          onSend={onSend}
          onOpen={(next) => (next === "reply" ? keys.r() : keys.n())}
          onAskOperator={() => ask.current()}
          onTyping={onTyping}
        />
      </div>
    </div>
  );
}
