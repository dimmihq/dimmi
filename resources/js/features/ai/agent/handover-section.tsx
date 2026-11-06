/*
 * Where a conversation goes when the agent hands it over: to a team the
 * workspace picks, or back to the inbox's own routing, and whether it
 * jumps the queue when it arrives.
 */
import { useQuery } from "@tanstack/react-query";

import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Row, Section } from "@/features/settings/layout";
import type { AiAgent } from "@/lib/api/ai";
import { teamsQuery } from "@/lib/api/teams";

import type { AgentChange } from "../format";

/*
 * inboxRouting is the choice that leaves a handed-over conversation to
 * its inbox's assignment rules.
 */
const inboxRouting = "inbox";

export function HandoverSection({
  workspace,
  agent,
  canWrite,
  save,
}: {
  workspace: string;
  agent: AiAgent;
  canWrite: boolean;
  save: (change: AgentChange) => void;
}) {
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: workspace !== "" });
  const options = [{ id: inboxRouting, name: "The inbox's usual routing" }, ...teams];

  return (
    <Section title="Handover" description="What happens when it passes a conversation to the team.">
      <Row label="Hand over to" description="The team that picks up conversations it hands over.">
        <Select
          aria-label="Hand over to"
          size="sm"
          isDisabled={!canWrite}
          selectedKey={agent.handoff_team_id ?? inboxRouting}
          onSelectionChange={(key) => {
            if (key) {
              save({ handoff_team_id: key === inboxRouting ? null : String(key) });
            }
          }}
          popoverClassName="w-64"
        >
          {options.map((option) => (
            <SelectItem key={option.id} id={option.id} textValue={option.name}>
              {option.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Raise priority on handover" description="Conversations it hands over jump the queue.">
        <Switch
          aria-label="Raise priority on handover"
          size="sm"
          isDisabled={!canWrite}
          isSelected={agent.raise_priority_on_handoff}
          onChange={(raise_priority_on_handoff) => save({ raise_priority_on_handoff })}
        />
      </Row>
    </Section>
  );
}
