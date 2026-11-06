/*
 * Finding or creating a tag without leaving the keyboard: type to narrow
 * the workspace's tags, Enter takes the first match, and a name nobody has
 * used becomes a new tag. Shift+T opens it from anywhere on the
 * conversation, unless the person is typing somewhere.
 */
import { RiAddLine, RiCheckLine, RiPriceTag3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { type Tag, tagsQuery } from "@/lib/api/tags";
import { cx } from "@/utils/cx";

import { TagIcon } from "./tag-chip";
import { isNewName, matching } from "./tag-names";

function useOpenKey(open: () => void) {
  useEffect(() => {
    const press = (event: KeyboardEvent) => {
      const target = event.target;
      const typing = target instanceof Element && target.closest("input, textarea, select, [contenteditable='true']");
      if (typing || event.key !== "T" || !event.shiftKey || event.metaKey || event.ctrlKey || event.altKey) {
        return;
      }
      event.preventDefault();
      open();
    };
    document.addEventListener("keydown", press);

    return () => document.removeEventListener("keydown", press);
  }, [open]);
}

function Option({ tag, selected, onToggle }: { tag: Tag; selected: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onToggle}
      className={cx(
        "flex min-h-8 w-full items-center gap-2 rounded-xl px-2 text-start text-body-2-regular text-text-primary",
        "outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
      )}
    >
      <TagIcon tag={tag} />
      <span className="min-w-0 flex-1 truncate">{tag.name}</span>
      {selected ? <RiCheckLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" /> : null}
    </button>
  );
}

export function TagPicker({
  workspace,
  selected,
  onAdd,
  onRemove,
  shortcut = false,
  label,
}: {
  workspace: string;
  selected: Tag[];
  onAdd: (name: string) => void;
  onRemove: (tag: Tag) => void;
  shortcut?: boolean;
  label?: string;
}) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const { data: tags = [] } = useQuery({ ...tagsQuery(workspace), enabled: open });
  const shown = matching(tags, text);
  const isOn = (tag: Tag) => selected.some((other) => other.id === tag.id);
  const toggle = (tag: Tag) => (isOn(tag) ? onRemove(tag) : onAdd(tag.name));
  const create = isNewName(tags, text);
  const openPicker = () => setOpen(true);

  useOpenKey(shortcut ? openPicker : noop);

  const submit = () => {
    const first = shown[0];
    if (create) {
      onAdd(text.trim());
    } else if (first) {
      toggle(first);
    }
    setText("");
  };

  return (
    <PopoverTrigger isOpen={open} onOpenChange={setOpen}>
      <Button variant="secondary" size="small" leadingIcon={RiPriceTag3Line} aria-label={label}>
        Add tag
      </Button>
      <Popover aria-label="Tags" placement="bottom end" className="w-64">
        <div className="flex flex-col gap-1">
          <Input
            aria-label="Find or create a tag"
            size="small"
            autoFocus
            value={text}
            onChange={setText}
            placeholder="Find or create a tag"
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                submit();
              }
            }}
          />
          <div className="flex max-h-64 flex-col overflow-y-auto">
            {shown.map((tag) => (
              <Option key={tag.id} tag={tag} selected={isOn(tag)} onToggle={() => toggle(tag)} />
            ))}
            {create ? (
              <button
                type="button"
                onClick={submit}
                className="flex min-h-8 w-full items-center gap-2 rounded-xl px-2 text-start text-body-2-regular text-text-primary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                <RiAddLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                <span className="truncate">Create "{text.trim()}"</span>
              </button>
            ) : null}
          </div>
        </div>
      </Popover>
    </PopoverTrigger>
  );
}

function noop() {
  return undefined;
}
