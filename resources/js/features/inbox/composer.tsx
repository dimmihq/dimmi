/*
 * The reply and note composer, on BoardUI's composer panel. It rests as a
 * slim bar under the thread so the conversation keeps the room; choosing
 * Reply or Note (or pressing R or N) unfolds it into the full panel, which
 * stays open until the person leaves the conversation. The panel has the
 * BoardUI tab strip behind its top edge saying who will see the message,
 * a field that grows with what is typed, and a toolbar with the add menu
 * (help articles, saved replies and more from the workspace, dropped into
 * the text) and a way to ask Operator. Replies cannot carry files yet, so
 * the composer offers none rather than show files that would never be
 * sent. Sending goes to the server; a sent draft empties, a refused one
 * stays with the reason under it. Each time Reply or Note is chosen the field
 * takes the focus, sized to the draft it holds. While a reply is being
 * written the composer says so through onTyping, and says it stopped
 * once the reply is sent or the field is left.
 */
import { RiArrowUpLine, RiChatQuoteLine, RiLock2Line, RiSparkling2Line, RiStickyNoteLine } from "@remixicon/react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useCallback, useRef, useState } from "react";

import { Kbd } from "@/components/base/kbd/kbd";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { cx } from "@/utils/cx";

import { AddMenu } from "./add-menu";
import { type Mode, modes } from "./modes";
import type { Report } from "./use-typing-report";

/*
 * Send sends what the composer holds as a reply or a note.
 */
export type Send = (mode: Mode, text: string) => Promise<unknown>;

const surface =
  "rounded-3xl bg-background-primary-default shadow-[0_1px_0.5px_rgba(0,0,0,0.02),0_4px_2px_rgba(0,0,0,0.02)]";
const settle = [0.22, 1, 0.36, 1] as const;

export function Composer({
  mode,
  focusKey,
  customer,
  channel,
  onSend,
  onOpen,
  onAskOperator,
  onTyping,
}: {
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

/*
 * sizeToText sizes the field to its text, between its resting height and a cap
 * past which it scrolls.
 */
function sizeToText(field: HTMLTextAreaElement) {
  field.style.height = "auto";
  field.style.height = `${Math.min(field.scrollHeight, 280)}px`;
}

function Panel({
  mode,
  focusKey,
  customer,
  channel,
  onSend,
  onModeChange,
  onAskOperator,
  onTyping,
}: {
  mode: Mode;
  focusKey: number;
  customer: string;
  channel: string;
  onSend: Send;
  onModeChange: (mode: Mode) => void;
  onAskOperator: () => void;
  onTyping: Report;
}) {
  const [drafts, setDrafts] = useState<Record<Mode, string>>({ reply: "", note: "" });
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const field = useRef<HTMLTextAreaElement | null>(null);
  const note = mode === "note";
  const arrive = useCallback((element: HTMLTextAreaElement | null) => {
    field.current = element;
    if (element) {
      sizeToText(element);
      element.focus();
    }
  }, []);

  const insert = (text: string) => {
    setDrafts((before) => ({ ...before, [mode]: before[mode] ? `${before[mode]}\n\n${text}` : text }));
    window.requestAnimationFrame(() => {
      if (field.current) {
        sizeToText(field.current);
        field.current.focus();
      }
    });
  };

  const text = drafts[mode].trim();
  const send = async () => {
    if (!text || sending) {
      return;
    }
    setSending(true);
    setFailure(null);
    onTyping(false);
    try {
      await onSend(mode, text);
      setDrafts((before) => ({ ...before, [mode]: "" }));
    } catch (error) {
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
        className={cx(surface, "m-0 flex min-w-0 flex-col gap-5 border-0 p-2.5 pt-4")}
      >
        <div className="flex flex-col gap-3 px-1.5">
          <textarea
            key={`${mode}-${focusKey}`}
            ref={arrive}
            value={drafts[mode]}
            onChange={(event) => {
              const value = event.target.value;
              setDrafts((before) => ({ ...before, [mode]: value }));
              if (!note) {
                onTyping(value.trim() !== "");
              }
            }}
            onBlur={() => onTyping(false)}
            onInput={(event) => sizeToText(event.currentTarget)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                void send();
              }
            }}
            rows={4}
            aria-label={note ? "Internal note" : `Reply to ${customer}`}
            placeholder={note ? "Write a note only your team sees…" : `Write your reply to ${customer}…`}
            className="block max-h-[280px] min-h-24 w-full resize-none bg-transparent p-0 text-body-regular text-text-primary caret-accent-500 outline-none placeholder:text-text-tertiary"
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
        </div>
      </fieldset>
    </div>
  );
}
