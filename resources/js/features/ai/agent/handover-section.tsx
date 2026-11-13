/*
 * The agent's role and where a conversation goes when it hands it over.
 * It either answers and resolves, or only triages and passes everything
 * to the team. Some tags always send a conversation to a person, whatever
 * the role. The handed-over conversation goes to a team the workspace
 * picks, or back to the inbox's own routing, and may jump the queue.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Radio, RadioGroup } from "@/components/base/radio/radio";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { toast } from "@/components/base/toast/toast";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { Row, Section } from "@/features/settings/layout";
import type { AiAgent, AiHandoffMode } from "@/lib/api/ai";
import { createTag, type Tag, tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";

import type { AgentChange } from "../format";
import { refusal } from "../use-agent";
import { StackedRow } from "./fields";
import { handoffModes } from "./options";

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
    <Section title="Handover" description="What it does with a conversation and when it passes it to the team.">
      <StackedRow label="Agent's role">
        <RadioGroup
          aria-label="Agent's role"
          value={agent.handoff_mode}
          isDisabled={!canWrite}
          onChange={(mode) => save({ handoff_mode: mode as AiHandoffMode })}
          className="gap-3"
        >
          {handoffModes.map((mode) => (
            <Radio key={mode.id} value={mode.id}>
              <span className="flex flex-col">
                <span className="text-body-regular text-text-primary">{mode.label}</span>
                <span className="text-body-2-regular text-text-secondary">{mode.line}</span>
              </span>
            </Radio>
          ))}
        </RadioGroup>
      </StackedRow>
      <AlwaysTagsRow workspace={workspace} agent={agent} canWrite={canWrite} save={save} />
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

/*
 * AlwaysTagsRow lists the tags that always send a conversation to the
 * team. A tag deleted since it was chosen no longer shows and drops out
 * on the next save. A name nobody has used yet becomes a new tag first.
 */
function AlwaysTagsRow({
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
  const client = useQueryClient();
  const { data: tags = [] } = useQuery({ ...tagsQuery(workspace), enabled: workspace !== "" });
  const chosen = agent.always_handoff_tag_ids
    .map((id) => tags.find((tag) => tag.id === id))
    .filter((tag): tag is Tag => tag !== undefined);
  const keep = (next: Tag[]) => save({ always_handoff_tag_ids: next.map((tag) => tag.id) });

  const add = async (name: string) => {
    const known = tags.find((tag) => tag.name.toLowerCase() === name.toLowerCase());
    if (known) {
      keep([...chosen, known]);
      return;
    }
    try {
      const created = await createTag(workspace, name);
      await client.invalidateQueries({ queryKey: ["workspace", workspace, "tags"] });
      keep([...chosen, created]);
    } catch (error) {
      toast.error("Tag not created", { description: refusal(error, "Check your connection and try again.") });
    }
  };
  const remove = (tag: Tag) => keep(chosen.filter((other) => other.id !== tag.id));

  return (
    <StackedRow
      label="Always hand over when tagged"
      description="When the agent tags a conversation with one of these, it hands it to your team."
    >
      <div className="flex flex-wrap items-center gap-1.5">
        {chosen.map((tag) => (
          <TagChip key={tag.id} tag={tag} onRemove={canWrite ? () => remove(tag) : undefined} />
        ))}
        {canWrite ? (
          <TagPicker
            workspace={workspace}
            selected={chosen}
            onAdd={add}
            onRemove={remove}
            label="Add a tag that always hands over"
          />
        ) : null}
        {!canWrite && chosen.length === 0 ? (
          <span className="text-body-2-regular text-text-tertiary">No tags yet</span>
        ) : null}
      </div>
    </StackedRow>
  );
}
