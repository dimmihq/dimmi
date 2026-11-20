/*
 * The assign picker, opened with A or from the header: a search box over
 * teammates and teams in one short list. Typing narrows it, the arrow
 * keys move through it and Enter picks. The reader comes first, then
 * whoever holds the conversation. Picking a teammate hands the
 * conversation to them; picking a team moves it to the team and keeps
 * whoever holds it.
 */
import { RiCheckLine, RiSearchLine, RiTeamLine, RiUserAddLine, RiUserUnfollowLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { type KeyboardEvent, type ReactNode, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import {
  Dropdown,
  DropdownGroup,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { avatarColor } from "@/features/shell/avatar-color";
import { teamsQuery } from "@/lib/api/teams";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { useInbox } from "../conversations";

/*
 * A choice is one row of the picker: nobody, a teammate or a team.
 */
interface Choice {
  key: string;
  group: "Teammates" | "Teams";
  name: string;
  icon: ReactNode;
  note?: string;
  current: boolean;
  pick: () => void;
}

/*
 * first puts the reader, then whoever holds the conversation, ahead of
 * everyone else, who stay in the order they came.
 */
function first<T extends { id: string; user: { id: string } }>(members: T[], me: string, assignee: string | null): T[] {
  const rank = (m: T) => (m.user.id === me ? 0 : m.id === assignee ? 1 : 2);

  return [...members].sort((a, b) => rank(a) - rank(b));
}

/*
 * step moves the highlighted row with the arrow keys and wraps at either
 * end; null for any other key.
 */
function step(event: KeyboardEvent, at: number, count: number): number | null {
  if (count === 0 || (event.key !== "ArrowDown" && event.key !== "ArrowUp")) {
    return null;
  }
  const by = event.key === "ArrowDown" ? 1 : -1;

  return (at + by + count) % count;
}

function Choices({ choices, at, onHover }: { choices: Choice[]; at: number; onHover: (index: number) => void }) {
  if (choices.length === 0) {
    return <p className="px-2 py-3 text-body-2-regular text-text-tertiary">Nobody by that name.</p>;
  }

  return (["Teammates", "Teams"] as const).map((group) => {
    const rows = choices.map((choice, index) => ({ choice, index })).filter((row) => row.choice.group === group);
    if (rows.length === 0) {
      return null;
    }

    return (
      <DropdownGroup key={group} label={group}>
        {rows.map(({ choice, index }) => (
          <div key={choice.key} onPointerMove={() => onHover(index)} ref={index === at ? intoView : undefined}>
            <DropdownItem selected={index === at} onSelect={choice.pick} className="px-2 py-1.5">
              {choice.icon}
              <span className="min-w-0 flex-1 truncate text-start text-body-2-regular">
                {choice.name}
                {choice.note ? <span className="text-text-tertiary"> {choice.note}</span> : null}
              </span>
              {choice.current ? (
                <RiCheckLine aria-label="Current" className="size-4 shrink-0 text-foreground-icon-secondary" />
              ) : null}
            </DropdownItem>
          </div>
        ))}
      </DropdownGroup>
    );
  });
}

/*
 * intoView keeps the highlighted row inside the list as the arrow keys
 * move past its edge.
 */
function intoView(row: HTMLDivElement | null) {
  row?.scrollIntoView({ block: "nearest" });
}

export function AssignPicker({
  workspace,
  assignee,
  team,
  isOpen,
  onOpenChange,
  onMember,
  onTeam,
  trigger,
}: {
  workspace: string;
  assignee: string | null;
  team: string | null;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onMember: (id: string | null) => void;
  /* Leave out to list teammates only, where the team has its own field. */
  onTeam?: (id: string) => void;
  /* What opens the picker, when it is not the header's icon button. */
  trigger?: { label: string; className: string; children: ReactNode };
}) {
  const { userId } = useInbox();
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: isOpen });
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: isOpen && Boolean(onTeam) });
  const [text, setText] = useState("");
  const [at, setAt] = useState(0);
  const open = (next: boolean) => {
    setText("");
    setAt(0);
    onOpenChange(next);
  };
  const pick = (run: () => void) => () => {
    run();
    open(false);
  };

  const active = members.filter((m) => m.status === "active");
  const all: Choice[] = [
    ...first(active, userId, assignee).map((m) => ({
      key: m.id,
      group: "Teammates" as const,
      name: m.user.name,
      note: m.user.id === userId ? "(you)" : undefined,
      icon: <Avatar size="xs" color={avatarColor(m.user.name)} initials={m.user.name.charAt(0).toUpperCase()} />,
      current: m.id === assignee,
      pick: pick(() => onMember(m.id)),
    })),
    {
      key: "nobody",
      group: "Teammates" as const,
      name: "Unassigned",
      icon: <RiUserUnfollowLine aria-hidden className="size-4 text-foreground-icon-secondary" />,
      current: false,
      pick: pick(() => onMember(null)),
    },
    ...(onTeam ? teams : []).map((t) => ({
      key: t.id,
      group: "Teams" as const,
      name: t.name,
      icon: <RiTeamLine aria-hidden className="size-4 text-foreground-icon-secondary" />,
      current: t.id === team,
      pick: pick(() => onTeam?.(t.id)),
    })),
  ];
  const wanted = text.trim().toLowerCase();
  const choices = wanted ? all.filter((choice) => choice.name.toLowerCase().includes(wanted)) : all;

  return (
    <Dropdown isOpen={isOpen} onOpenChange={open}>
      {trigger ? (
        <DropdownTrigger aria-label={trigger.label} className={trigger.className}>
          {trigger.children}
        </DropdownTrigger>
      ) : (
        <DropdownTrigger
          aria-label="Assign"
          className={cx(
            "flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-2 text-body-2-medium text-text-secondary",
            "transition-colors duration-150 hover:bg-background-secondary-hover",
          )}
        >
          <RiUserAddLine aria-hidden className="size-4 text-foreground-icon-secondary" />
        </DropdownTrigger>
      )}
      <DropdownPopover
        aria-label="Assign to"
        placement={trigger ? "bottom start" : "bottom end"}
        className="w-64 overflow-hidden"
        dialogClassName="gap-2"
      >
        <Input
          aria-label={onTeam ? "Find a teammate or team" : "Find a teammate"}
          placeholder={onTeam ? "Find a teammate or team" : "Find a teammate"}
          size="small"
          leadingIcon={RiSearchLine}
          fieldClassName="bg-background-secondary-default"
          autoFocus
          value={text}
          onChange={(next) => {
            setText(next);
            setAt(0);
          }}
          onKeyDown={(event) => {
            const next = step(event, at, choices.length);
            if (next !== null) {
              event.preventDefault();
              setAt(next);
            } else if (event.key === "Enter") {
              event.preventDefault();
              choices[at]?.pick();
            }
          }}
        />
        <div className="flex max-h-72 flex-col gap-1 overflow-y-auto">
          <Choices choices={choices} at={at} onHover={setAt} />
        </div>
      </DropdownPopover>
    </Dropdown>
  );
}
