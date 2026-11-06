/*
 * Who may open an inbox: everyone, or only the teams and teammates on its
 * list, each on a row of its own with a way to take it off. Owners and
 * admins open every inbox, so a list left empty still leaves them in.
 * The choice is part of the page's draft and saves with everything else.
 */
import { RiAddLine, RiCloseLine, RiTeamLine } from "@remixicon/react";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { IconButton } from "@/components/base/buttons/icon-button";
import {
  Dropdown,
  DropdownGroup,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { avatarColor } from "@/features/shell/avatar-color";
import type { InboxAccess } from "@/lib/api/inboxes";
import type { Team } from "@/lib/api/teams";
import type { Member } from "@/lib/api/workspace";

import { Choice } from "../controls";
import { Empty, Glyph, Item, Row, Section } from "../layout";

const everyone = "Everyone";
const chosen = "Chosen teams and teammates";

function AddPicker({
  teams,
  members,
  onTeam,
  onMember,
}: {
  teams: Team[];
  members: Member[];
  onTeam: (id: string) => void;
  onMember: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const pick = (run: () => void) => {
    run();
    setOpen(false);
  };

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger className="flex h-8 items-center gap-1.5 rounded-lg border border-border-button-default bg-background-primary-default px-2.5 text-body-2-medium text-text-primary transition-colors duration-150 hover:bg-background-primary-hover">
        <RiAddLine aria-hidden className="size-4 text-foreground-icon-secondary" />
        Add
      </DropdownTrigger>
      <DropdownPopover
        aria-label="Add to access"
        placement="bottom end"
        className="max-h-80 w-60 overflow-y-auto p-1.5"
        dialogClassName="gap-0.5"
      >
        {teams.length > 0 ? (
          <DropdownGroup className="gap-0.5 pt-1.5">
            <span className="ps-2 pb-0.5 text-caption-1-medium text-text-tertiary">Teams</span>
            {teams.map((t) => (
              <DropdownItem key={t.id} onSelect={() => pick(() => onTeam(t.id))} className="gap-2 px-2 py-1.5">
                <RiTeamLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                <span className="min-w-0 flex-1 truncate text-start text-body-2-medium">{t.name}</span>
              </DropdownItem>
            ))}
          </DropdownGroup>
        ) : null}
        {members.length > 0 ? (
          <DropdownGroup className="gap-0.5 pt-1.5">
            <span className="ps-2 pb-0.5 text-caption-1-medium text-text-tertiary">Teammates</span>
            {members.map((m) => (
              <DropdownItem key={m.id} onSelect={() => pick(() => onMember(m.id))} className="gap-2 px-2 py-1.5">
                <Avatar size="xs" color={avatarColor(m.user.name)} initials={m.user.name.charAt(0).toUpperCase()} />
                <span className="min-w-0 flex-1 truncate text-start text-body-2-medium">{m.user.name}</span>
              </DropdownItem>
            ))}
          </DropdownGroup>
        ) : null}
        {teams.length + members.length === 0 ? (
          <p className="px-2 py-3 text-caption-1-regular text-text-tertiary">Everyone is on the list.</p>
        ) : null}
      </DropdownPopover>
    </Dropdown>
  );
}

export function AccessSection({
  access,
  teams,
  members,
  locked,
  onChange,
}: {
  access: InboxAccess;
  teams: Team[];
  members: Member[];
  locked: boolean;
  onChange: (access: InboxAccess) => void;
}) {
  const restricted = access.mode === "restricted";
  const listedTeams = teams.filter((t) => access.team_ids.includes(t.id));
  const listedMembers = members.filter((m) => access.member_ids.includes(m.id));
  const active = members.filter((m) => m.status === "active");
  const without = (list: string[], id: string) => list.filter((x) => x !== id);

  return (
    <Section title="Access" description="Who can open this inbox and its conversations. Owners and admins always can.">
      <Row label="Who can open it">
        <Choice
          label="Who can open it"
          options={[everyone, chosen]}
          value={restricted ? chosen : everyone}
          disabled={locked}
          onChange={(option) => onChange({ ...access, mode: option === chosen ? "restricted" : "everyone" })}
        />
      </Row>
      {restricted ? (
        <>
          {listedTeams.map((t) => (
            <Item key={t.id} mark={<Glyph icon={RiTeamLine} />} title={t.name} detail="Everyone in the team">
              {locked ? null : (
                <IconButton
                  size="small"
                  icon={RiCloseLine}
                  aria-label={`Remove ${t.name}`}
                  onClick={() => onChange({ ...access, team_ids: without(access.team_ids, t.id) })}
                />
              )}
            </Item>
          ))}
          {listedMembers.map((m) => (
            <Item
              key={m.id}
              mark={
                <Avatar size="sm" color={avatarColor(m.user.name)} initials={m.user.name.charAt(0).toUpperCase()} />
              }
              title={m.user.name}
              detail={m.user.email}
            >
              {locked ? null : (
                <IconButton
                  size="small"
                  icon={RiCloseLine}
                  aria-label={`Remove ${m.user.name}`}
                  onClick={() => onChange({ ...access, member_ids: without(access.member_ids, m.id) })}
                />
              )}
            </Item>
          ))}
          <Empty
            action={
              locked ? null : (
                <AddPicker
                  teams={teams.filter((t) => !access.team_ids.includes(t.id))}
                  members={active.filter((m) => !access.member_ids.includes(m.id))}
                  onTeam={(id) => onChange({ ...access, team_ids: [...access.team_ids, id] })}
                  onMember={(id) => onChange({ ...access, member_ids: [...access.member_ids, id] })}
                />
              )
            }
          >
            {listedTeams.length + listedMembers.length === 0
              ? "Nobody is on the list yet, so only owners and admins can open it."
              : "Add a team or teammate."}
          </Empty>
        </>
      ) : null}
    </Section>
  );
}
