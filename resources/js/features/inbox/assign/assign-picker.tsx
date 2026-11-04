/*
 * The assign picker, opened with A or from the header: teammates and
 * teams in one list. Picking a teammate hands the conversation to them;
 * picking a team moves it to the team and keeps whoever holds it.
 */
import { RiTeamLine, RiUserAddLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Avatar } from "@/components/base/avatar/avatar";
import {
  Dropdown,
  DropdownGroup,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { avatarColor } from "@/features/shell/avatar-color";
import { teamsQuery } from "@/lib/api/teams";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

export function AssignPicker({
  workspace,
  assignee,
  team,
  isOpen,
  onOpenChange,
  onMember,
  onTeam,
}: {
  workspace: string;
  assignee: string | null;
  team: string | null;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onMember: (id: string | null) => void;
  onTeam: (id: string) => void;
}) {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: isOpen });
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: isOpen });
  const pick = (run: () => void) => {
    run();
    onOpenChange(false);
  };

  return (
    <Dropdown isOpen={isOpen} onOpenChange={onOpenChange}>
      <DropdownTrigger
        aria-label="Assign"
        className={cx(
          "flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-2 text-body-2-medium text-text-secondary",
          "transition-colors duration-150 hover:bg-background-secondary-hover",
        )}
      >
        <RiUserAddLine aria-hidden className="size-4 text-foreground-icon-secondary" />
      </DropdownTrigger>
      <DropdownPopover aria-label="Assign to" placement="bottom end" className="max-h-96 w-64 overflow-y-auto">
        <DropdownGroup label="Teammates">
          <DropdownItem selected={assignee === null} onSelect={() => pick(() => onMember(null))}>
            <span className="text-start">Unassigned</span>
          </DropdownItem>
          {members
            .filter((m) => m.status === "active")
            .map((m) => (
              <DropdownItem key={m.id} selected={m.id === assignee} onSelect={() => pick(() => onMember(m.id))}>
                <Avatar size="xs" color={avatarColor(m.user.name)} initials={m.user.name.charAt(0).toUpperCase()} />
                <span className="min-w-0 flex-1 truncate text-start">{m.user.name}</span>
              </DropdownItem>
            ))}
        </DropdownGroup>
        {teams.length > 0 ? (
          <DropdownGroup label="Teams">
            {teams.map((t) => (
              <DropdownItem key={t.id} selected={t.id === team} onSelect={() => pick(() => onTeam(t.id))}>
                <RiTeamLine aria-hidden className="size-4 text-foreground-icon-secondary" />
                <span className="min-w-0 flex-1 truncate text-start">{t.name}</span>
              </DropdownItem>
            ))}
          </DropdownGroup>
        ) : null}
      </DropdownPopover>
    </Dropdown>
  );
}
