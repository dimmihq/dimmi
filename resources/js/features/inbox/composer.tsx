/*
 * The reply and note composer, on BoardUI's composer panel. It rests as a
 * slim bar under the thread so the conversation keeps the room; choosing
 * Reply or Note (or pressing R or N) unfolds it into the full panel, which
 * stays open until the person leaves the conversation. The panel has the
 * BoardUI tab strip behind its top edge saying who will see the message,
 * a field that grows with what is typed with formatting kept as
 * Markdown, and a toolbar with the add menu (help articles, saved replies
 * and more from the workspace, dropped into the text) and a way to ask
 * Operator. Replies cannot carry files yet, so the composer offers none
 * rather than show files that would never be sent. Sending goes to the
 * server; a sent draft empties, a refused one stays with the reason under
 * it. Each draft, reply and note apart, is kept on the server and in the
 * browser, so it survives leaving, reloading and another device. Each
 * time Reply or Note is chosen the field
 * takes the focus, sized to the draft it holds. While a reply is being
 * written the composer says so through onTyping, and says it stopped
 * once the reply is sent or the field is left.
 */
import { RiArrowUpLine, RiChatQuoteLine, RiLock2Line, RiSparkling2Line, RiStickyNoteLine } from "@remixicon/react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useEffect, useState } from "react";

import { Kbd } from "@/components/base/kbd/kbd";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import type { Message } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { AddMenu } from "./add-menu";
import { ComposerField } from "./composer/composer-field";
import { preloadEditor } from "./composer/load-editor";
import { useInbox } from "./conversations";
import { DraftConflict } from "./drafts/draft-conflict";
import { useDraft } from "./drafts/use-draft";
import { type Mode, modes } from "./modes";
import { SendMenu } from "./send-menu";
import { UndoToast } from "./undo/undo-toast";
import { useHeld } from "./undo/use-held";
import type { Report } from "./use-typing-report";

/*
 * Send sends what the composer holds as a reply or a note, and for a
 * reply what then happens to the conversation: close or snooze.
 */
export type Send = (mode: Mode, text: string, then?: "close" | "snooze") => Promise<Message | undefined>;

const surface =
  "rounded-3xl bg-background-primary-default shadow-[0_1px_0.5px_rgba(0,0,0,0.02),0_4px_2px_rgba(0,0,0,0.02)]";
const settle = [0.22, 1, 0.36, 1] as const;

/*
 * ignoreFiles drops files pasted into the field, since a reply cannot
 * carry them yet.
 */
const ignoreFiles = () => undefined;

export function Composer({
  conversation,
  mode,
  focusKey,
  customer,
  channel,
  onSend,
  onOpen,
  onAskOperator,
  onTyping,
}: {
  conversation: string;
  mode: Mode | null;
  focusKey: number;
  customer: string;
  channel: string;
  onSend: Send;
  onOpen: (mode: Mode) => void;
  onAskOperator: () => void;
  onTyping: Report;
}) {
  const still = useReducedMotion();
  useEffect(preloadEditor, []);

  return (
    <div className="mx-auto w-full max-w-3xl">
      <AnimatePresence initial={false} mode="wait">
        {mode ? (
          <motion.div
            key="open"
            initial={{ opacity: 0, height: 52, filter: "blur(2px)" }}
            animate={{ opacity: 1, height: "auto", filter: "blur(0px)" }}
            transition={still ? { duration: 0 } : { duration: 0.3, ease: settle }}
            className="overflow-hidden"
          >
            <Panel
              conversation={conversation}
              mode={mode}
              focusKey={focusKey}
              customer={customer}
              channel={channel}
              onSend={onSend}
              onModeChange={onOpen}
              onAskOperator={onAskOperator}
              onTyping={onTyping}
            />
          </motion.div>
        ) : (
          <motion.div
            key="closed"
            exit={{ opacity: 0, filter: "blur(2px)" }}
            transition={still ? { duration: 0 } : { duration: 0.12, ease: "easeIn" }}
          >
            <Bar customer={customer} onOpen={onOpen} />
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function Bar({ customer, onOpen }: { customer: string; onOpen: (mode: Mode) => void }) {
  return (
    <div className={cx(surface, "flex items-center gap-1 p-2")}>
      <button
        type="button"
        onClick={() => onOpen("reply")}
        className="flex h-9 min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-full ps-2.5 pe-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiChatQuoteLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="min-w-0 flex-1 truncate text-body-regular text-text-tertiary">Reply to {customer}…</span>
        <Kbd>R</Kbd>
      </button>
      <button
        type="button"
        onClick={() => onOpen("note")}
        className="flex h-9 shrink-0 cursor-pointer items-center gap-2 rounded-full ps-2.5 pe-2 outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiStickyNoteLine aria-hidden className="size-4 text-foreground-icon-secondary" />
        <span className="text-body-medium text-text-secondary">Note</span>
        <Kbd>N</Kbd>
      </button>
    </div>
  );
}

function Panel({
  conversation,
  mode,
  focusKey,
  customer,
  channel,
  onSend,
  onModeChange,
  onAskOperator,
  onTyping,
}: {
  conversation: string;
  mode: Mode;
  focusKey: number;
  customer: string;
  channel: string;
  onSend: Send;
  onModeChange: (mode: Mode) => void;
  onAskOperator: () => void;
  onTyping: Report;
}) {
  const { workspace } = useInbox();
  const drafts = { reply: useDraft(workspace, conversation, "reply"), note: useDraft(workspace, conversation, "note") };
  const draft = drafts[mode];
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const note = mode === "note";
  const held = useHeld(workspace, conversation, (body) => {
    drafts.reply.edit(body);
    onModeChange("reply");
  });
  const insert = (text: string) => draft.edit(draft.body ? `${draft.body}\n\n${text}` : text);

  const text = draft.body.trim();
  const send = async (then?: "close" | "snooze") => {
    if (!text || sending) {
      return;
    }
    setSending(true);
    setFailure(null);
    onTyping(false);
    draft.pause();
    try {
      const sent = await onSend(mode, text, note ? undefined : then);
      draft.sent();
      held.hold(sent);
    } catch (error) {
      draft.edit(draft.body);
      setFailure(error instanceof Error ? error.message : "Sending failed. Try again.");
    } finally {
      setSending(false);
    }
  };

  return (
    <div>
      <div className="mx-7 flex h-[34px] items-center justify-between gap-3 rounded-t-2xl bg-composer-panel-tab-background px-1 py-1">
        <SegmentedControl
          aria-label="Write"
          variant="plain"
          selectedKeys={[mode]}
          onSelectionChange={(selected) => {
            const next = [...selected][0];
            if (next === modes.reply || next === modes.note) {
              draft.flush();
              onModeChange(next);
            }
          }}
          className="[&_button]:py-0.5 [&_button]:text-body-2-medium"
        >
          <SegmentedControlItem id={modes.reply}>Reply</SegmentedControlItem>
          <SegmentedControlItem id={modes.note}>Note</SegmentedControlItem>
        </SegmentedControl>
        <span className="flex min-w-0 items-center gap-1 pe-1.5 text-body-2-medium text-text-secondary">
          {note ? <RiLock2Line aria-hidden className="size-3.5 shrink-0" /> : null}
          <span className="truncate">{note ? "Only your team sees this" : `To ${customer} · ${channel}`}</span>
        </span>
      </div>
      <fieldset
        aria-label={note ? "Note composer" : "Reply composer"}
        className={cx(surface, "m-0 flex min-w-0 flex-col gap-5 border-0 p-2.5 pt-4 transition-shadow duration-150")}
      >
        <div className="flex flex-col gap-3 px-1.5">
          <UndoToast held={held} />
          <DraftConflict draft={draft} />
          <ComposerField
            value={draft.body}
            label={note ? "Internal note" : `Reply to ${customer}`}
            placeholder={note ? "Write a note only your team sees…" : `Write your reply to ${customer}…`}
            focusKey={`${mode}-${focusKey}`}
            onChange={(value) => {
              draft.edit(value);
              if (!note) {
                onTyping(value.trim() !== "");
              }
            }}
            onSubmit={(then) => void send(then)}
            onBlur={() => onTyping(false)}
            onFiles={ignoreFiles}
          />
        </div>
        <div className="flex items-center gap-2">
          <AddMenu onInsert={insert} />
          <button
            type="button"
            onClick={onAskOperator}
            className="flex h-[30px] shrink-0 cursor-pointer items-center gap-1 rounded-full ps-2 pe-2.5 text-body-2-medium text-text-secondary outline-none transition-colors duration-150 ease hover:bg-background-secondary-hover hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiSparkling2Line aria-hidden className="size-4" />
            Ask Operator
          </button>
          <span
            role={failure ? "alert" : undefined}
            className={cx(
              "ms-auto truncate text-body-2-regular",
              failure ? "text-text-error-primary" : "hidden text-text-tertiary sm:inline",
            )}
          >
            {failure ?? (note ? "Only your team sees notes" : `Sends on ${channel}`)}
          </span>
          <button
            type="button"
            disabled={!text || sending}
            onClick={() => void send()}
            aria-label={note ? "Add note" : "Send"}
            className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-button-primary p-2 text-text-white outline-none transition-opacity duration-200 ease focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-not-allowed disabled:opacity-40"
          >
            <RiArrowUpLine aria-hidden className="size-5" />
          </button>
          {note ? null : <SendMenu disabled={!text || sending} onSend={(then) => void send(then)} />}
        </div>
      </fieldset>
    </div>
  );
}
