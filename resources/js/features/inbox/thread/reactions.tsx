/*
 * Emoji reactions on a note. Under the note sit its reactions, each with
 * how many teammates reacted; the reader's own are pressed, and a click
 * adds or takes theirs off. The hover toolbar offers a few common emoji
 * and the full emoji list.
 */
import { RiEmotionLine } from "@remixicon/react";
import { useState } from "react";
import { Button as AriaButton, Dialog, DialogTrigger, Popover } from "react-aria-components";

import { MENU_POPOVER_SURFACE } from "@/components/base/dropdown/menu-styles";
import type { Reaction } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { EmojiPicker } from "../composer/emoji-picker";

const common = ["👍", "❤️", "😄", "🎉", "👀", "✅"];

export function ReactionChips({
  reactions,
  me,
  onToggle,
}: {
  reactions: Reaction[];
  me: string | null;
  onToggle: (emoji: string, remove: boolean) => void;
}) {
  if (reactions.length === 0) {
    return null;
  }

  return (
    <div className="mt-1 flex flex-wrap gap-1">
      {reactions.map((reaction) => {
        const mine = me !== null && reaction.member_ids.includes(me);

        return (
          <button
            key={reaction.emoji}
            type="button"
            aria-pressed={mine}
            aria-label={`${reaction.emoji} ${reaction.count}`}
            onClick={() => onToggle(reaction.emoji, mine)}
            className={cx(
              "flex h-6 cursor-pointer items-center gap-1 rounded-full border px-2 text-body-2-medium text-text-secondary outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
              mine
                ? "border-border-button-active bg-background-primary-active"
                : "border-border-button-default bg-background-primary-default hover:bg-background-secondary-hover",
            )}
          >
            <span aria-hidden>{reaction.emoji}</span>
            {reaction.count}
          </button>
        );
      })}
    </div>
  );
}

export function ReactionPicker({ className, onPick }: { className: string; onPick: (emoji: string) => void }) {
  const [open, setOpen] = useState(false);
  const [all, setAll] = useState(false);
  const pick = (emoji: string) => {
    onPick(emoji);
    setOpen(false);
    setAll(false);
  };

  return (
    <DialogTrigger isOpen={open} onOpenChange={setOpen}>
      <AriaButton aria-label="Add reaction" className={className}>
        <RiEmotionLine aria-hidden className="size-4" />
      </AriaButton>
      <Popover placement="top" offset={6} className={cx(MENU_POPOVER_SURFACE, "w-[320px] p-2")}>
        <Dialog aria-label="Add reaction" className="flex flex-col gap-2 outline-none">
          <div className="flex items-center gap-0.5">
            {common.map((emoji) => (
              <button
                key={emoji}
                type="button"
                aria-label={`React with ${emoji}`}
                onClick={() => pick(emoji)}
                className="flex size-9 cursor-pointer items-center justify-center rounded-lg text-title-3-semibold outline-none transition-colors hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                {emoji}
              </button>
            ))}
            <button
              type="button"
              onClick={() => setAll(!all)}
              className="ms-auto cursor-pointer rounded-lg px-2 py-1 text-body-2-medium text-text-secondary outline-none hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              {all ? "Less" : "More"}
            </button>
          </div>
          {all ? <EmojiPicker onPick={pick} /> : null}
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
