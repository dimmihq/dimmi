/*
 * The team a conversation sits with, picked beside the person who holds
 * it in the details. Moving it to a team keeps the person who holds it;
 * a team's view lists it from then on.
 */

import { useQuery } from "@tanstack/react-query";
import type { Key } from "react-aria-components";

import { Select, SelectItem } from "@/components/base/select/select";
import type { Conversation } from "@/lib/api/inbox";
import { teamsQuery } from "@/lib/api/teams";

import type { CaseControls } from "./case";
import { useInbox } from "./conversations";

const noTeam = "none";

export function TeamRow({ conversation, caze }: { conversation: Conversation; caze: CaseControls }) {
  const { workspace } = useInbox();
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: workspace !== "" });

  return (
    <div className="grid min-h-9 grid-cols-[80px_minmax(0,1fr)] items-center gap-2">
      <dt className="text-body-2-regular text-text-secondary">Team</dt>
      <dd className="flex min-w-0 justify-end text-body-2-medium text-text-primary">
        <Select
          aria-label="Team"
          size="sm"
          selectedKey={conversation.team?.id ?? noTeam}
          onSelectionChange={(key: Key | null) => {
            if (key !== null) {
              caze.assignTeam(key === noTeam ? null : String(key));
            }
          }}
        >
          <SelectItem id={noTeam} textValue="No team">
            No team
          </SelectItem>
          {teams.map((team) => (
            <SelectItem key={team.id} id={team.id} textValue={team.name}>
              {team.name}
            </SelectItem>
          ))}
        </Select>
      </dd>
    </div>
  );
}
