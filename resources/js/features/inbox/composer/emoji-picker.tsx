/*
 * Picking an emoji from the add menu: a search over every emoji, most
 * used faces first, and a grid that drops the one clicked into the
 * message.
 */
import { RiSearchLine } from "@remixicon/react";
import { useEffect, useState } from "react";

import { Input } from "@/components/base/input/input";

import { type Emoji, findEmoji, loadEmoji } from "./emoji";

export function EmojiPicker({ onPick }: { onPick: (emoji: string) => void }) {
  const [all, setAll] = useState<Emoji[]>([]);
  const [query, setQuery] = useState("");
  useEffect(() => {
    let live = true;
    void loadEmoji().then((list) => {
      if (live) {
        setAll(list);
      }
    });

    return () => {
      live = false;
    };
  }, []);
  const found = findEmoji(all, query, 64);

  return (
    <div className="flex flex-col gap-2">
      <Input
        aria-label="Search emoji"
        placeholder="Search emoji"
        leadingIcon={RiSearchLine}
        fieldClassName="bg-background-secondary-default"
        value={query}
        onChange={setQuery}
        onKeyDown={(event) => {
          const first = found[0];
          if (event.key === "Enter" && first) {
            onPick(first.unicode);
          }
        }}
        autoFocus
      />
      <div className="grid max-h-56 grid-cols-8 gap-0.5 overflow-y-auto">
        {found.map((emoji) => (
          <button
            key={emoji.unicode}
            type="button"
            aria-label={emoji.label}
            title={emoji.label}
            onClick={() => onPick(emoji.unicode)}
            className="flex size-9 cursor-pointer items-center justify-center rounded-lg text-title-3-semibold outline-none transition-colors hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            {emoji.unicode}
          </button>
        ))}
        {all.length > 0 && found.length === 0 ? (
          <p className="col-span-8 px-2 py-3 text-body-regular text-text-tertiary">No emoji match</p>
        ) : null}
      </div>
    </div>
  );
}
