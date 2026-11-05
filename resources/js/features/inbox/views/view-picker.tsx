/*
 * The view picker: the list's title opens every view of the inbox in one
 * menu, grouped, each with how many conversations wait in it, so the
 * sidebar stays short. It sits inside the list's heading, so the heading
 * reads as the view and how many wait in it. Typing narrows the views by
 * name, V opens it from the keyboard, and a new saved view starts from
 * the view on screen.
 */
import { RiAddLine, RiArrowDownSLine, RiSearchLine } from "@remixicon/react";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import {
  Dropdown,
  DropdownDivider,
  DropdownGroup,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { ScrollFade, useScrollFade } from "@/components/foundations/scroll-fade/scroll-fade";
import { cx } from "@/utils/cx";

import { useViewGroups, type ViewGroup } from "./view-groups";

/*
 * narrowed keeps the views whose name holds the search, dropping groups
 * left empty.
 */
function narrowed(groups: ViewGroup[], search: string): ViewGroup[] {
  const needle = search.trim().toLowerCase();
  if (!needle) {
    return groups;
  }

  return groups
    .map((group) => ({ ...group, rows: group.rows.filter((r) => r.label.toLowerCase().includes(needle)) }))
    .filter((group) => group.rows.length > 0);
}

export function ViewPicker({
  workspace,
  view,
  title,
  count,
}: {
  workspace: string;
  view: string;
  title: string;
  count: number | undefined;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const groups = narrowed(useViewGroups(workspace, open), search);
  const navigate = useNavigate();
  const fade = useScrollFade();
  const toggle = (next: boolean) => {
    setOpen(next);
    setSearch("");
  };
  const go = (to: string, editor?: "new") => {
    toggle(false);
    void navigate({ to: "/d/inbox/$view", params: { view: to }, search: editor ? { editor } : {} });
  };

  return (
    <Dropdown isOpen={open} onOpenChange={toggle}>
      <DropdownTrigger
        data-view-picker
        className="flex h-8 min-w-0 max-w-full items-center gap-1.5 rounded-lg px-2 transition-colors duration-150 hover:bg-background-secondary-hover aria-expanded:bg-background-secondary-hover"
      >
        <span className="min-w-0 truncate text-body-medium text-text-primary">{title}</span>
        <span className="shrink-0 text-body-2-regular text-text-tertiary">
          {count === undefined ? "Open" : `${count} open`}
        </span>
        <RiArrowDownSLine
          aria-hidden
          className={cx(
            "size-4 shrink-0 text-foreground-icon-tertiary transition-transform duration-150",
            open && "rotate-180",
          )}
        />
      </DropdownTrigger>
      <DropdownPopover aria-label="Views" className="w-60 p-1.5" dialogClassName="max-h-[min(28rem,70vh)] gap-0.5">
        <Input
          aria-label="Search views"
          placeholder="Search views"
          leadingIcon={RiSearchLine}
          size="small"
          value={search}
          onChange={setSearch}
          autoFocus
        />
        <div className="relative -mx-1.5 flex min-h-0 flex-1 flex-col">
          <div
            onScroll={fade.onScroll}
            className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-1.5 [scrollbar-width:none]"
          >
            {groups.map((group) => (
              <DropdownGroup key={group.key} className="gap-0.5 pt-1.5">
                <span className="ps-2 pb-0.5 text-caption-1-medium text-text-tertiary">{group.label}</span>
                {group.rows.map((row) => (
                  <DropdownItem
                    key={row.id}
                    selected={row.id === view}
                    onSelect={() => go(row.id)}
                    className="gap-2 px-2 py-1.5"
                  >
                    <row.icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                    <span className="min-w-0 flex-1 truncate text-start text-body-2-medium">{row.label}</span>
                    {row.count ? (
                      <span className="shrink-0 text-caption-1-medium text-text-tertiary">{row.count}</span>
                    ) : null}
                  </DropdownItem>
                ))}
              </DropdownGroup>
            ))}
            {groups.length === 0 ? (
              <p className="px-2 py-3 text-caption-1-regular text-text-tertiary">No views match “{search.trim()}”.</p>
            ) : null}
          </div>
          <ScrollFade progress={fade.progress} />
        </div>
        <DropdownDivider className="-mx-1.5 my-1" />
        <DropdownItem onSelect={() => go(view, "new")} className="gap-2 px-2 py-1.5">
          <RiAddLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          <span className="text-start text-body-2-medium">New view</span>
        </DropdownItem>
      </DropdownPopover>
    </Dropdown>
  );
}
